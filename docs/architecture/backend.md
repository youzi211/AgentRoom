# 后端架构

[返回架构索引](README.md)

本文描述当前 Go 后端的装配、模块职责和依赖边界。当前实现最准确的定义是：**由显式组合根装配的模块化单体**。它借鉴了端口与适配器思想，但不是严格的 Clean Architecture。

## 1. 组合根与启动顺序

`backend/cmd/server/main.go` 是唯一应用组合根。理解后端时应先从这里查看真实对象图，不要只依据包名推断依赖。

启动顺序如下：

```text
load root .env
  -> initialize slog
  -> load DB / security / DeepAgent config
  -> open and ping MySQL
  -> optionally AutoMigrate
  -> load built-in + DeepAgent Agent definitions
  -> seed and load global Agents
  -> construct SecretCipher / ModelProfileService / ModelResolver
  -> construct KnowledgeService / room.Manager
  -> construct Go resolving LLM client
  -> construct agent.Runner and RuntimeRegistry
  -> construct FocusService / MinutesService / RoomService
  -> inject focused API ports into api.Server
  -> mark stale online participants offline
  -> start HTTP server
  -> lazily start response workers on the first Agent response job
```

主要事实来源：

- `backend/cmd/server/main.go`
- `backend/internal/config/env.go`
- `backend/internal/store/mysql/store.go`

## 2. 依赖方向

```text
                         cmd/server
                     constructs and injects
                              |
        +---------------------+----------------------+
        |                     |                      |
        v                     v                      v
      api                  service                 config
 HTTP / WS adapter      use-case coordination     logging
        |                     |
        |              +------+-------+
        |              |              |
        v              v              v
      room <--------- agent          llm
 runtime state       execution     model adapter
        |              |              |
        +--------------+--------------+
                       |
                    store ports
                       |
                       v
                   store/mysql
                    GORM/MySQL

model and realtime are shared types used across the graph.
```

主要包依赖为：

```text
api     -> service ports, room, agent templates, store errors, model, realtime
service -> room, agent, llm, store ports, model, realtime
agent   -> room runtime interface, llm, store ports, model, realtime
room    -> store ports, model, realtime
llm     -> model and a narrow model-resolver interface
mysql   -> store contracts, model
```

禁止业务包直接导入 `internal/store/mysql` 或 GORM。MySQL 具体实现只应在组合根被构造。

## 3. API 适配层

`backend/internal/api` 负责：

- Gin 路由和 middleware；
- HTTP 请求、响应和 contracts；
- WebSocket Upgrade、读写循环；
- 管理员密钥、房间口令和 Origin 检查；
- 将应用错误映射为 HTTP 状态；
- 把实时 client event 转交给应用端口。

关键文件：

```text
backend/internal/api/
├── server.go                    # Server、端口接口、路由注册
├── rest_handlers.go             # 通用 REST handlers
├── model_profile_handlers.go    # 模型 Profile handlers
├── ws_handlers.go               # WebSocket transport loop
├── access.go                    # admin/passcode/origin 策略
└── contracts/                   # API DTO 和领域类型转换
```

### 3.1 API 依赖的端口

API 不再只依赖一个宽泛接口，而是注入以下聚焦能力：

- `RoomQueryService`
- `RoomCommandService`
- `RoomAccessService`
- `ModelProfileService`

这些接口在 `backend/internal/api/server.go` 中按消费者需要定义，实际由 Service façade 提供。

### 3.2 路由约定

路由同时注册在：

- `/api/*`：规范路径；
- `/*`：迁移期兼容路径。

新前端和新集成只应使用 `/api/*`。

### 3.3 当前边界并非完全纯净

开发时需要知道以下现状，而不要误以为层次已经完全隔离：

- API 的部分错误映射直接识别 `store` typed errors；
- 部分 Query 接口仍返回具体 `*room.Room`，Handler 会读取其 Info、Participants 和 Agents；
- 个别新增能力通过运行时接口断言取得，而非完整写入正式端口。

新增功能应优先扩大明确的消费者端口和应用 DTO，不要继续增加隐藏的类型断言或让 Handler 接触持久化实现。

## 4. 应用服务层

`backend/internal/service` 是主要用例协调层。

### 4.1 RoomService façade

`RoomService` 聚合：

- `room.Manager`
- `AgentService`
- `agent.Runner`
- Store 端口
- Knowledge、Focus、Minutes、Meeting Lifecycle 等能力
- 有界 Agent 响应队列

它通过 `Queries()`、`Commands()`、`Access()` 暴露聚焦端口。当前仍是迁移期 façade，不应在文档或新代码中假设它已经消失，也不应把所有新逻辑继续堆回单个文件。

关键文件：

- `backend/internal/service/room_service.go`
- `backend/internal/service/room_ports.go`
- `backend/internal/service/room_queries.go`
- `backend/internal/service/room_commands.go`
- `backend/internal/service/realtime_session.go`
- `backend/internal/service/meeting_lifecycle.go`

### 4.2 领域服务

| 服务 | 职责 |
| --- | --- |
| `AgentService` | 全局 Agent CRUD、Runtime/Profile 兼容校验、建房 Agent 快照解析 |
| `KnowledgeService` | Markdown 校验、解析、分片、room/agent scope 检索 |
| `FocusService` | 从房间消息生成进程内 Focus Timeline |
| `MinutesService` | 使用 Go 默认模型生成并持久化会议纪要 |
| `ModelProfileService` | Profile CRUD、默认规则、引用约束、加密、脱敏、连接测试 |
| `ModelResolver` | 运行时解析具体数据库 Profile 或环境迁移兜底 |

模型相关细节见 [Agent Runtime 与模型](agent-runtime-and-models.md)。

### 4.3 新用例放置规则

- HTTP/WS 解析和序列化放在 `api`；
- 跨组件业务顺序、权限后的用例执行放在 `service`；
- 单个房间的锁保护状态操作放在 `room.Room`；
- Agent 执行和对话策略放在 `agent`；
- Provider 协议适配放在 `llm` 或对应 Runtime；
- 数据查询与事务放在 Repository；
- 共享状态类型放在 `model`，协议事件放在 `realtime`。

避免把新业务规则放进 Handler、GORM hook 或 React Client。

## 5. Room 运行时子系统

`backend/internal/room` 同时承担领域运行时与实时基础设施职责，不是纯实体层。

### 5.1 Room

`room.Room` 是带 `sync.RWMutex` 的内存聚合，持有：

- `RoomMeta` 与 Dialogue Policy；
- 房间 Agent 快照；
- 在线参与者；
- 最近消息；
- WebSocket Hub。

内存消息有容量上限；完整消息历史仍以 MySQL 为准。

### 5.2 Manager

`room.Manager`：

- 创建房间；
- 调用注入的 Agent resolver 生成快照；
- 在数据库成功后创建内存 Room；
- 缓存当前进程加载过的房间；
- cache miss 时从 MySQL 快照恢复。

由于 Manager 直接依赖窄 Store 端口，它更适合被理解为“房间运行时子系统”，而不是无基础设施依赖的纯领域服务。

### 5.3 Hub

`room.Hub` 管理当前进程的 WebSocket Client 和非阻塞广播。Client 的发送缓冲区满时会被移除，避免慢连接阻塞整个房间。

Hub 没有跨进程总线。部署多个 backend 实例之前，必须增加 sticky routing 和共享实时事件机制，或明确房间单实例所有权。

## 6. Agent 执行子系统

`backend/internal/agent` 中的 Runner 是另一个应用级协调组件。它会：

- 解析 mention 和 Dialogue Policy；
- 检索知识、组合 Prompt；
- 创建和完成运行审计；
- 调用 Runtime；
- 持久化 Agent/System 消息；
- 广播消息和 `agent_activity`；
- 在引导对话中维护轮次和父子消息关系。

Runner 通过 `RuntimeRoom` 一类最小接口使用房间能力，而不是要求整个 Service。但它同时依赖 Store 和实时广播，因此不是纯文本生成函数。

详见 [Agent Runtime 与模型](agent-runtime-and-models.md)。

## 7. LLM 适配层

`backend/internal/llm` 是较薄的 OpenAI-compatible Chat Completions 适配层：

- 定义统一 `Client` / `JSONClient`；
- 包装 LangChainGo OpenAI model；
- 标准化 API Base URL；
- 转换消息角色；
- 统一未配置和调用错误；
- `ResolvingClient` 在每次系统调用前解析当前 Go 默认 Profile。

Profile CRUD、密钥和解析策略不属于 `llm` 包，而属于 Service 层。`llm` 通过窄 Resolver 接口取得运行配置。

## 8. Store 与 MySQL

`backend/internal/store` 保存持久化契约、typed errors 和 persistence-oriented 参数。当前同时存在：

- 一个较宽的 `store.Store` 聚合接口；
- 各消费者在本包定义的窄 Store 接口。

MySQL 实现在 `backend/internal/store/mysql`，Repository 已按聚合拆分，例如 rooms、messages、agents、runs、model profiles。

```text
service / room / agent
        |
        v
consumer-defined narrow interfaces
        |
        v
mysql.MySQLStore
        |
        v
GORM models and transactions
```

GORM model 与领域 model 通过转换函数隔离。新增字段时必须同时检查：

1. `model` / `store` 合约；
2. GORM model；
3. domain <-> GORM 转换；
4. 写入与读取 Repository；
5. `backend/internal/tests/teststore`；
6. API contract 是否应暴露；
7. 迁移参考和文档。

当前程序迁移入口主要是 GORM `AutoMigrate`；`backend/internal/store/mysql/migrations/*.sql` 是可审阅的参考迁移，不应误写成启动时按文件顺序执行的迁移引擎。

## 9. 后台任务与上下文

人类消息广播后，Agent 工作进入 `RoomService` 内部的有界队列：

```text
responseJobs capacity = 64
worker count = 4
```

这样 WebSocket read loop 不直接承担长时间模型调用。任务上下文通过 `context.WithoutCancel` 脱离单个连接生命周期，所以客户端断开不会自动取消已经接受的人类消息所触发的 Agent 工作。

Go LLM Runtime 有调用超时；DeepAgent Runtime 有独立总超时和并发 semaphore。增加新的后台工作时应明确：

- 是否需要 durable queue；
- 上下文由谁取消；
- 是否允许进程重启丢失；
- 并发和背压位置；
- 运行状态如何落库；
- 是否可能重复执行。

## 10. 当前架构债务

以下是现状说明，不表示应在无关变更中顺手重构：

1. `api` 仍直接认识少量 `store` 错误和具体 Room；
2. Service、Room 和 Agent 都承担一定应用编排职责；
3. 个别 Profile 能力通过运行时类型断言取得，编译期契约不完整；
4. `ModelProfileService` 同时承担策略、密码学和 HTTP 连接测试，职责偏重；
5. `store.Store` 较宽，但消费者窄接口尚未成为统一端口层；
6. 实时 Hub 和任务队列仅支持单进程；
7. Agent/System 消息采用实时优先的 best-effort 持久化，可能与 MySQL 历史短暂不一致。

## 11. 测试入口

大多数后端行为测试集中在 `backend/internal/tests/**`：

- `backend/internal/tests/api`：HTTP、鉴权和 contracts；
- `backend/internal/tests/service`：应用用例、队列和 Profile；
- `backend/internal/tests/agent`：mention、对话、Runtime 和审计；
- `backend/internal/tests/room`：房间与 Manager；
- `backend/internal/tests/teststore`：共享内存替身。

少量必须访问 MySQL 包内部迁移细节的测试与实现共置，例如 `backend/internal/store/mysql/model_profiles_migration_test.go`。

常用验证：

```powershell
go -C backend test ./...
go -C backend vet ./...
go -C backend build ./cmd/server
```

## 12. 相关架构

- [Agent Runtime 与模型](agent-runtime-and-models.md)
- [数据、实时与会议生命周期](data-realtime-lifecycle.md)
- [前端与部署](frontend-and-deployment.md)
