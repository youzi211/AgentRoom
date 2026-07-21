# 数据、实时与会议生命周期

## Agent 成功消息与 Run 终态

远程或本地 Agent 成功后，Go 通过 `CommitAgentRunSuccess` 在同一 MySQL 事务中锁定 `running` Run、插入带唯一可空 `messages.agent_run_id` 的最终消息、保存非敏感模型审计，并把 Run 条件更新为 `succeeded`。只有事务提交后，消息才进入 Room 内存并通过 WebSocket 广播；提交失败时不会展示不可恢复的成功消息。

失败、取消、超时和中断同样使用只允许非终态进入一个终态的条件更新。后端启动会把没有本地活动调用对应的遗留 `running` 记录收敛为 `interrupted`，不会自动重新调用 Python。`agent_activity` 的模型/工具/进度事件仍是瞬时实时观测，不作为普通聊天消息持久化。

[返回架构索引](README.md)

本文描述 MySQL 持久化、进程内房间状态、消息一致性、WebSocket 会话和会议生命周期。核心原则是：**MySQL 保存可恢复事实，Go 内存保存当前实例的实时运行状态。**

## 1. 状态边界

| 状态 | Go 内存 | MySQL | DeepAgent 文件 | 恢复语义 |
| --- | ---: | ---: | ---: | --- |
| 全局 Agent 配置 | 是 | 是 | 否 | 启动加载；本进程管理 API 更新时同步 |
| 房间元数据与策略 | 是 | 是 | 否 | cache miss 时恢复 |
| 房间 Agent 快照 | 是 | 是 | 否 | cache miss 时恢复 |
| Model Profile | 不长期缓存 DB 行 | 是，密文 | 否 | 每次模型调用重新读取 |
| 环境模型兜底 | 启动时快照 | 否 | `.env` 仅为输入 | 重启后重读 |
| 在线参与者 | 是 | 是 | 否 | 启动时旧在线记录统一离线 |
| 人类消息 | 最近窗口 | 是 | 否 | 严格先持久化后可见 |
| Agent/System 消息 | 最近窗口 | best-effort | 否 | DB 失败时可能仅实时可见 |
| Agent run | 无长期缓存 | 是 | 否 | durable audit |
| Guided dialogue run | 无长期缓存 | 是 | 否 | durable audit |
| Focus Timeline | 是 | 否 | 否 | 重启丢失 |
| WebSocket Hub/Client | 是 | 否 | 否 | 断线或重启丢失 |
| DeepAgent 原始报告 | 短暂 | artifact 副本 | 是 | 文件和 MySQL 双份 |
| 会议纪要 | 可读取 | 是 | 可导出 | 版本化持久化 |

## 2. 数据模型

当前核心表：

| 表 | 用途 |
| --- | --- |
| `agents` | 全局 Agent 配置 |
| `model_profiles` | Go/DeepAgent 模型连接，API Key 为认证加密密文 |
| `rooms` | 房间元数据、Dialogue Policy 和生命周期 |
| `room_agents` | 建房时的 Agent 与 Profile ID 快照 |
| `participants` | 参与者、加入/离开和在线状态 |
| `messages` | Human、Agent、System 消息及来源/artifact JSON |
| `agent_runs` | 每次 Agent turn 的状态和模型审计 |
| `dialogue_runs` | Guided Dialogue 整段运行和停止状态 |
| `knowledge_documents` | room/agent scope 知识文档元数据 |
| `knowledge_chunks` | Markdown 解析后的文本分片 |
| `meeting_minutes` | 会议纪要版本和生成来源 |
| `schema_migrations` | 为迁移治理预留的兼容结构；当前生产代码不读写版本记录 |

参考 SQL 位于 `backend/internal/store/mysql/migrations/`：

```text
001_initial_schema.sql
002_meeting_minutes.sql
003_room_lifecycle.sql
004_message_sources.sql
005_model_profiles.sql
```

当前应用实际由 `MySQLStore.Migrate` 调用 GORM `AutoMigrate`；这些 SQL 文件主要提供可审阅的结构参考，不是一个按编号执行的独立迁移引擎。

## 3. 创建房间与 Agent 快照

```text
POST /api/rooms
  -> resolve DialoguePolicy defaults
  -> AgentService.ResolveForRoom(agentIDs)
       |-- choose enabled Agents
       |-- preserve explicit compatible Profile ID
       `-- otherwise snapshot current runtime DB default ID when available
  -> Manager.CreateRoom
  -> MySQL transaction
       |-- INSERT rooms
       `-- INSERT room_agents for every selected Agent
  -> construct Room in memory
  -> cache in Manager.rooms
```

事务边界保证 `rooms` 与 `room_agents` 一起成功或失败。只有数据库提交成功后，Manager 才把 Room 放入内存缓存。

`room_agents` 固化：

- Agent ID、名称、mention；
- 角色、描述和 system prompt；
- enabled、顺序和 Runtime；
- 具体 `model_profile_id`（存在数据库绑定/默认时）。

它不会复制 Profile 的 Base URL、模型名或密文。模型连接内容在每次调用时由 Profile ID 动态读取。

事实来源：

- `backend/internal/service/agent_service.go`
- `backend/internal/room/manager.go`
- `backend/internal/store/mysql/rooms_repo.go`
- `backend/internal/store/mysql/models.go`

## 4. 房间缓存与冷加载

`room.Manager` 保存当前进程的房间 map。查询未命中时，从 Store 加载完整快照：

```text
rooms
+ room_agents
+ messages (bounded)
+ active participants
    -> Room snapshot
    -> NewFromSnapshot
    -> cache only after complete load succeeds
```

不能把部分加载成功的 Room 放入缓存；否则后续请求会长期看到缺少 Agent、消息或参与者的不完整状态。

当前内存 Room 最多保留最近的一个有界消息窗口，MySQL 保留完整历史。需要注意，冷加载使用的某些 bounded message 查询当前按 `created_at ASC LIMIT` 读取，可能取得最早的一段而非最新一段；REST 分页采用独立游标逻辑。修改这部分时应明确“初始上下文需要最新消息”这一不变量。

主要代码：

- `backend/internal/room/manager.go`
- `backend/internal/room/room.go`
- `backend/internal/store/mysql/rooms_repo.go`
- `backend/internal/store/mysql/messages_repo.go`

## 5. WebSocket 会话生命周期

### 5.1 建立连接

浏览器连接：

```text
ws[s]://<current-host>/api/rooms/<roomID>/ws
  ?name=<participantName>
  &passcode=<optionalRoomPasscode>
```

服务端顺序：

```text
resolve room
  -> validate admin bypass or room passcode
  -> require active room
  -> validate participant name
  -> upgrade connection (CheckOrigin runs inside Upgrade)
  -> persist participant
  -> add participant to Room memory
  -> lifecycle hook may broadcast an updated room_snapshot to existing clients
  -> register the new Client in Hub
  -> broadcast participant_joined to existing clients
  -> send initial room_snapshot to the new Client
  -> broadcast the final updated snapshot to existing clients
```

主要代码：

- `frontend/src/api/roomClient.js`
- `backend/internal/api/access.go`
- `backend/internal/api/ws_handlers.go`
- `backend/internal/service/realtime_session.go`

### 5.2 读写循环

WebSocket transport 分成：

- read loop：读取 Client event，转交 `RoomCommandService`；
- write pump：从 Client 的发送 channel 写出 Server event；
- cleanup：注销 Hub client、标记 participant 离开并广播状态。

当前有消息大小限制，但没有完整的 ping/pong heartbeat、read deadline、自动重连退避或消息级速率限制。浏览器断线后的重连行为不能被误认为已有可靠 session 恢复协议。

### 5.3 Hub 背压

每个 Client 有小型发送 buffer。广播时：

- channel 可写：排队给 write pump；
- channel 满：Hub 移除该 Client 并关闭发送 channel。

这是单房间内避免慢消费者阻塞的策略，不是消息可靠投递。客户端丢失事件后应通过 REST 历史/房间 snapshot 重新同步。

## 6. 人类消息一致性

人类消息遵循严格写序：

```text
ClientEvent(message)
  -> validate non-empty and room active
  -> INSERT messages in MySQL
  -> append to Room memory
  -> update FocusService
  -> broadcast human message
  -> optionally broadcast focus_update
  -> enqueue Agent response job
```

不变量：

- MySQL 写入失败时，不进入内存；
- 不广播未持久化的人类消息；
- 不为未持久化的人类消息触发 Agent；
- Agent 不能在人类消息对调用者可见之前开始生成回复。

主要代码：

- `backend/internal/service/room_commands.go`
- `backend/internal/service/realtime_session.go`
- `backend/internal/tests/service/room_service_order_test.go`

Focus 分析目前在广播前同步执行；命中分析阈值时可能增加人类消息可见延迟。Focus 状态只在内存，后端重启后丢失。

## 7. Agent/System 消息一致性

Runner 的消息写入策略与人类消息不同：

```text
try INSERT message in MySQL
  |-- success -> use persisted message
  `-- failure -> log error and continue with in-memory message

append to Room memory
broadcast to WebSocket clients
```

这是“实时可见性优先”的 best-effort 策略。数据库暂时故障时，在线用户可能看到 Agent/System 消息，但刷新、REST 历史或进程重启后无法恢复该消息。

若未来把消息历史定义为严格审计记录，应改为 durable outbox、重试队列或“持久化成功后广播”，并重新评估用户体验和模型调用幂等性。

主要代码：`backend/internal/agent/runner.go`。

## 8. Agent 与 Dialogue 审计

### 8.1 Agent run

每次 Agent turn 创建一行 `agent_runs`：

```text
running
  -> execute runtime
  -> update actual model profile/source/name
  -> succeeded | failed | timeout
```

`agent_activity` 是瞬时 WebSocket 事件；`agent_runs` 是持久化审计。数据库写失败时，活动事件仍可能被广播，所以两者不是事务一致的事件日志。

### 8.2 Dialogue run

只有 `guided_dialogue` 创建 `dialogue_runs`。它记录整段对话：

- root human trigger；
- mode；
- turn count；
- `running`、`succeeded`、`stopped_*` 或失败状态；
- created/completed timestamps。

每个实际 turn 仍对应一条 `agent_runs`，后续 turn 的 trigger 可指向上一条 Agent message。

## 9. 实时事件模型

Server event 的权威结构位于：

- `backend/internal/model` 的消息和房间 DTO；
- `backend/internal/realtime/events.go` 的活动事件；
- `backend/internal/api/contracts` 的外部响应转换。

事件类型包括：

- room snapshot；
- message；
- participant joined/left；
- owner/lifecycle 变化；
- focus update；
- agent activity；
- error。

不要在文档中复制全部 JSON 字段作为独立 schema。修改事件时应同步检查前端 `ChatRoom` 的 switch/merge 逻辑和相关 `*.test.mjs`。

## 10. 会议生命周期

房间主要状态：

```text
active   --owner close / last-human timeout--> closed
active   --admin archive---------------------> archived
closed   --admin archive---------------------> archived
archived --admin restore---------------------> closed
closed   --admin reopen----------------------> active
```

管理员可以直接把 `active` 或 `closed` 房间归档；只有当前房主可以执行手工 close。Restore 将 `archived` 恢复为 `closed`，Reopen 再将 `closed` 恢复为 `active`。

语义概览：

| 状态 | 普通读取 | WebSocket 加入/发言 | 管理操作 |
| --- | --- | --- | --- |
| `active` | 允许，受口令保护 | 允许，受口令和 Origin 保护 | 房主可关闭；管理员可归档 |
| `closed` | 可只读查看历史和纪要 | 拒绝 live join 和发言 | 管理员可 reopen/archive |
| `archived` | 普通用户拒绝 | 拒绝 | 管理员可查看并 restore |

房主可以关闭活跃会议或转移所有权；最后一位人类离开后会启动 grace window，并按当前生命周期策略关闭会议。

主要代码：

- `backend/internal/service/meeting_lifecycle.go`
- `backend/internal/service/realtime_session.go`
- `backend/internal/api/access.go`
- `backend/internal/store/mysql/rooms_repo.go`

## 11. 访问凭据边界

### 11.1 管理员密钥

受保护管理路由使用 `X-Admin-Key` 与 `ADMIN_API_KEY` 比较。管理员密钥也可绕过房间口令执行管理读取。

当 `ADMIN_API_KEY` 为空时，当前 `requireAdmin` 行为会放行；这适合显式的本地开发模式，但生产必须配置非空值并使用 TLS。

### 11.2 房间口令

房间口令是共享 bearer secret：

- REST 通过 `X-Room-Passcode`；
- WebSocket 通过 query 参数；
- MySQL 保存 SHA-256 hash。

它不是用户身份。口令进入 URL 时可能出现在浏览器历史、复制链接和代理访问日志；SHA-256 也不适合抵抗低熵短口令的数据库离线字典攻击。

### 11.3 Origin

Origin allowlist 仅保护浏览器 WebSocket Upgrade：

- 配置 `ALLOWED_ORIGINS` 时精确匹配；
- 配置为空时允许所有；
- 空 Origin 的非浏览器客户端被允许。

Origin 不是认证，不能替代管理员或房间访问控制。

## 12. 单实例限制

实时层没有跨实例同步：

```text
backend A: Room + Hub + Focus + response queue
            X no shared bus X
backend B: Room + Hub + Focus + response queue
```

横向扩展前至少要决定：

- 房间是否固定到一个 backend 实例；
- WebSocket sticky routing；
- Redis Pub/Sub、Streams 或其他事件总线；
- Agent 任务是否进入 durable queue；
- participant presence 的权威存储；
- 跨实例幂等和分布式锁；
- Focus 是否持久化。

## 13. Schema/状态修改检查表

- GORM model、domain model、Store contract 和转换函数是否同步；
- `backend/internal/tests/teststore` 是否同步；
- 建房与快照恢复是否保留新字段；
- MySQL 事务边界是否仍正确；
- 人类消息的持久化先行顺序是否被破坏；
- WebSocket snapshot 与 REST DTO 是否需要新字段；
- 生命周期访问矩阵是否更新；
- `.env.example`、README 和架构文档是否同步；
- 是否需要真实 MySQL 集成测试，而不仅是内存 Store 测试。

## 14. 相关架构

- [后端架构](backend.md)
- [Agent Runtime 与模型](agent-runtime-and-models.md)
- [前端与部署](frontend-and-deployment.md)
