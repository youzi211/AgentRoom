# AgentRoom 系统架构文档

## 1. 项目定位

AgentRoom 是一个实时文本会议室应用。人类用户和可配置的 AI Agent 在同一个房间内协作，Agent 默认保持沉默，只有在被明确 `@AgentName` 提及时才响应。

核心目标：

- 支持多人实时文本会议。
- 支持管理员维护全局 Agent 配置。
- 支持创建房间时选择参与会议的 Agent。
- 支持房间级 Agent 快照，保证会议期间 Agent 配置稳定。
- 支持消息、参与者、Agent 执行记录持久化。
- 支持 OpenAI 兼容模型接口。
- 支持房间级会议文件和 Agent 级知识库，第一版支持 Markdown 文档。

## 2. 技术栈

| 层级 | 技术 |
| --- | --- |
| 后端 | Go 1.22+ / Gin / gorilla/websocket |
| 持久化 | MySQL 8.0+ / GORM |
| LLM | `langchaingo` + OpenAI-compatible chat completions |
| 日志 | Go 标准库 `log/slog` |
| 前端 | React 18 / Vite |
| 路由 | History API pathname-based routing |

当前技术栈保持轻量，适合从单体应用起步。后续如果要支持多实例部署或上百人会议，应补充 Redis Pub/Sub、任务队列、连接心跳和限流设计。

## 3. 后端分层

```text
backend/
├── cmd/server/
│   └── main.go                    # 应用入口与依赖装配
└── internal/
    ├── api/
    │   └── server.go              # HTTP/WS 路由和协议适配器
    ├── agent/
    │   ├── registry.go            # 预定义 Agent
    │   ├── runner.go              # Agent 响应生成，依赖 RuntimeRoom 接口
    │   ├── dialogue.go            # Guided Dialogue 调度、停机条件与重复抑制
    │   ├── mention.go             # @ 提及检测
    │   └── sanitize.go            # 去除 <think>/<thinking> 私有推理内容
    ├── config/
    │   └── env.go                 # .env 与环境变量加载
    ├── llm/
    │   └── client.go              # LLMClient 接口与 langchaingo OpenAI-compatible 实现
    ├── logging/
    │   ├── logging.go             # slog 初始化
    │   └── middleware.go          # Gin 请求日志与 panic recovery
    ├── model/
    │   ├── types.go               # 领域/API 数据结构
    │   └── id.go                  # ID 生成
    ├── service/
    │   ├── agent_service.go       # 全局 Agent 配置管理
    │   ├── knowledge_service.go   # Markdown 知识文档解析、切片和检索入口
    │   └── room_service.go        # 房间、参与者、消息和 Agent 编排
    ├── room/
    │   ├── manager.go             # 房间生命周期与内存缓存
    │   ├── room.go                # 房间运行时状态
    │   └── hub.go                 # WebSocket 广播中心
    ├── store/
    │   ├── store.go               # 持久化接口
    │   └── mysql/                 # GORM MySQL 实现、模型、迁移
    └── tests/
        ├── agent/                 # Agent 外部行为测试
        ├── room/                  # Room/Manager 外部行为测试
        ├── service/               # Service 外部行为测试
        └── teststore/             # 测试用 Store 替身
```

### 3.1 API 层

`internal/api` 是协议适配层，只负责：

- 注册 HTTP 和 WebSocket 路由。
- 解析请求参数和 JSON body。
- 将 HTTP/WebSocket 错误转换为用户可理解的响应。
- 将事件写入 WebSocket。

API 层不直接持有 `store.Store`、`agent.Runner` 或 `room.Manager`，统一通过 `service.RoomService` 执行业务用例。

### 3.2 Service 层

`AgentService` 管理全局 Agent 配置：

- `Agents()` 返回可编辑的全局 Agent 配置。
- `CreateAgent()` 创建 Agent，并自动派生 `Mention = "@" + Name`。
- `UpdateAgent()` 更新 Agent，并检查 mention 冲突。
- `DeleteAgent()` 删除全局 Agent。
- `ResolveForRoom(agentIDs)` 根据创建房间请求选择要快照进房间的 Agent。

`RoomService` 编排房间业务流程：

- `CreateRoom()` 创建房间，并把 Agent 快照持久化到 `room_agents`。
- `JoinParticipant()` 创建参与者并持久化。
- `LeaveParticipant()` 标记参与者离开。
- `ListMessages()` 优先从数据库读取消息，失败时回退到房间内存缓存。
- `HandleHumanMessage()` 先持久化人类消息，再追加内存并异步触发 Agent Runner。
- `UploadRoomKnowledge()` 上传房间级 Markdown 会议文件，所有房间 Agent 都可以参考。
- `UploadAgentKnowledge()` 上传 Agent 级 Markdown 知识文件，仅对应 Agent 发言时参考。

`KnowledgeService` 管理知识文档能力：

- 第一版只接收 `.md` / `.markdown` 文件，单文件限制 1MB。
- 使用 `goldmark` 解析 Markdown AST，提取可进入 LLM 上下文的纯文本。
- 将文档拆成 `knowledge_chunks`，后续可以替换为 embedding 检索或向量库。
- 对外暴露 room scope 与 agent scope 两类知识查询，不把知识逻辑散落在 Runner 或 API 层。

### 3.3 Room 层

`room.Manager` 只负责房间生命周期：

- 新建房间。
- 从数据库按需加载房间快照。
- 缓存活跃房间。

它不再负责全局 Agent 的增删改查。创建房间时，它通过注入的 `resolveAgents` 函数获取当前可用 Agent 列表。

`room.Room` 是单个房间的运行时状态，包含参与者、房间 Agent 快照、消息缓存和 WebSocket Hub。所有可变状态由 `sync.RWMutex` 保护。

Room 元数据还持有房间级 `DialoguePolicy`，用于区分默认的 `mention_fanout` 和受控多轮的 `guided_dialogue`。

### 3.4 Agent 层

`agent.Runner` 负责：

- 只响应人类消息中的显式 `@AgentName`。
- 在 `guided_dialogue` 模式下按房间策略调度后续 Agent 发言。
- 根据房间最近消息构造 LLM 上下文。
- 查询房间级与 Agent 级知识片段，并拼入“可参考知识库片段”。
- 调用 `llm.Client`。
- 清洗 `<think>` / `<thinking>` 私有推理内容。
- 持久化并广播 Agent 或系统消息。
- 记录 `agent_runs` 执行状态。
- 记录 `dialogue_runs`，并把 `dialogue_run_id` / `turn_index` / `parent_message_id` 写入生成消息。
- 在 Agent run 与 dialogue run 开始/结束时广播 `agent_activity` 事件，供前端实时展示运行状态。

Runner 依赖 `RuntimeRoom` 接口，而不是具体 `*room.Room`：

```go
type RuntimeRoom interface {
    Info() model.RoomMeta
    Agents() []model.Agent
    AgentsWithPrompts() []model.Agent
    RecentMessages(limit int) []model.Message
    NewSystemMessage(content string) model.Message
    NewAgentMessage(agent model.Agent, content string) model.Message
    AppendMessage(message model.Message)
    Broadcast(message model.Message)
    BroadcastEvent(event model.ServerEvent)
}
```

这个边界让 Agent 执行逻辑只依赖最小运行时能力，未来可以替换为队列任务、跨实例执行上下文或测试替身。

### 3.5 Store 层

`internal/store.Store` 是唯一持久化接口。业务层依赖接口，不依赖 MySQL/GORM 实现。

主要数据能力：

- Agent 配置：seed/list/create/update/delete。
- 房间：create/get/list room agents。
- 参与者：join/leave/list active。
- 消息：add/list。
- Agent 执行记录：create/finish/list run。
- Dialogue 执行记录：create/finish/list run。

MySQL 实现在 `internal/store/mysql` 中，GORM model 和 domain model 分离，通过转换函数连接。

## 4. 数据模型

核心表：

| 表 | 用途 |
| --- | --- |
| `agents` | 全局 Agent 配置模板 |
| `rooms` | 房间元数据 |
| `room_agents` | 房间创建时的 Agent 快照 |
| `participants` | 参与者与在线状态 |
| `messages` | 会议消息 |
| `agent_runs` | Agent 执行审计记录 |
| `dialogue_runs` | 多轮 Agent 对话链路与停止状态 |
| `knowledge_documents` | 知识文档元数据，支持 `room` 与 `agent` 两种作用域 |
| `knowledge_chunks` | Markdown 解析后的文本片段，用于 Agent 发言上下文 |
| `schema_migrations` | 数据库迁移记录 |

关键设计：房间创建时会把选中的全局 Agent 配置复制到 `room_agents`。之后全局 Agent 的更新不会影响已有房间。

## 5. 请求与事件流

### 5.1 创建房间

```text
POST /api/rooms
  -> api.Server 解析请求
  -> RoomService.CreateRoom
  -> room.Manager.CreateRoom
  -> AgentService.ResolveForRoom
  -> store.CreateRoom 持久化 rooms + room_agents
  -> 返回 RoomMeta
```

### 5.2 WebSocket 发送消息

```text
client message
  -> api.Server 读取 WebSocket 事件
  -> RoomService.HandleHumanMessage
  -> store.AddMessage 先持久化人类消息
  -> room.AppendMessage 更新内存缓存
  -> room.Hub.Broadcast 广播人类消息
  -> goroutine 异步调用 agent.Runner.HandleHumanMessage
  -> Runner 检测房间 DialoguePolicy
  -> mention_fanout: 被 @ 的 Agent 各回复一次
  -> guided_dialogue: 创建 dialogue_run，按策略选择首个与后续发言者
  -> RuntimeRoom.BroadcastEvent 广播 agent_activity started 事件
  -> llm.Client 调用模型
  -> agent.StripThinkBlocks 清洗响应
  -> store.AddMessage 持久化 Agent 消息
  -> RuntimeRoom.Broadcast 广播 Agent 消息
  -> 达到轮次上限 / 无可选发言者 / 重复内容 / provider error 时停止 dialogue_run
  -> RuntimeRoom.BroadcastEvent 广播 agent_activity finished 事件
```

## 6. HTTP API

基础路径：`/api`

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/health` | 健康检查 |
| GET | `/agents` | 获取全局 Agent 配置 |
| POST | `/agents` | 创建 Agent |
| PUT | `/agents/:agentID` | 更新 Agent |
| DELETE | `/agents/:agentID` | 删除 Agent |
| GET | `/agents/:agentID/knowledge` | 获取某个 Agent 的知识文档 |
| POST | `/agents/:agentID/knowledge` | 为某个 Agent 上传 Markdown 知识文档 |
| POST | `/rooms` | 创建房间 |
| GET | `/rooms/:roomID` | 获取房间快照 |
| GET | `/rooms/:roomID/messages` | 获取消息列表 |
| GET | `/rooms/:roomID/activity` | 获取最近 Agent run 与 dialogue run 活动 |
| GET | `/rooms/:roomID/knowledge` | 获取会议室文件 |
| POST | `/rooms/:roomID/knowledge` | 上传会议室 Markdown 文件 |
| DELETE | `/knowledge/:documentID` | 删除知识文档 |
| GET | `/rooms/:roomID/ws?name=Alice` | 加入 WebSocket 房间 |

当前仍保留无 `/api` 前缀的 legacy routes，用于迁移期兼容。前端应优先使用 `/api/*`。

`POST /api/rooms` 支持可选的 `dialoguePolicy` 字段，至少可以通过 `mode = guided_dialogue` 开启受控多轮对话。

WebSocket 连接除消息、房间快照和焦点更新外，也会收到 `type = "agent_activity"` 的事件。事件体包含 `activity.kind`（`agent_run` 或 `dialogue_run`）、`activity.phase`（`started` 或 `finished`）、运行状态、创建/完成时间，以及可用的 Agent 名称或 dialogue 轮次。

## 7. 前端结构

```text
frontend/src/
├── App.jsx                    # 顶层路由状态与页面切换
├── main.jsx                   # 入口
├── routing.js                 # History API pathname-based 路由
├── api/roomClient.js          # HTTP + WebSocket 客户端封装
├── components/
│   ├── JoinScreen.jsx
│   ├── RoomEntry.jsx
│   ├── ChatRoom.jsx
│   ├── AgentActivityPanel.jsx
│   ├── MessageList.jsx
│   ├── MessageComposer.jsx
│   ├── ParticipantList.jsx
│   ├── AgentRoster.jsx
│   ├── AgentAdmin.jsx
│   └── NotFound.jsx
├── styles.css
└── chat-room.css
```

前端路由：

| 路径 | 页面 |
| --- | --- |
| `/` | 创建/加入房间首页 |
| `/agents` | Agent 管理页 |
| `/rooms/:roomID` | 会议室或加入确认页 |

## 8. 配置

| 变量 | 必需 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `PORT` | 否 | `8080` | HTTP 端口 |
| `MYSQL_DSN` | 是 | - | MySQL DSN |
| `DB_AUTO_MIGRATE` | 否 | `false` | 是否自动迁移数据库 |
| `LLM_BASE_URL` | 否 | `https://api.openai.com/v1` | OpenAI 兼容接口地址 |
| `LLM_API_KEY` | 否 | - | LLM API Key，未配置时 Agent 调用失败 |
| `LLM_MODEL` | 否 | `gpt-4o-mini` | 模型名称 |
| `LOG_LEVEL` | 否 | `info` | 日志级别 |
| `LOG_FORMAT` | 否 | `text` | `text` 或 `json` |
| `LOG_ADD_SOURCE` | 否 | `false` | 是否输出源码位置 |

## 9. 安全与生产风险

当前仍需在上线前补齐：

- WebSocket `CheckOrigin` 目前允许所有来源，生产环境必须限制来源。
- Agent 管理 API 暂无认证与管理员权限控制。
- 房间加入仍以 query name 区分用户，没有正式用户体系。
- 缺少消息发送失败重试与幂等机制。
- 单实例内存 Hub 不能支持多后端实例广播，需要 Redis Pub/Sub 或消息总线。
- Agent 执行在 goroutine 中直接跑，后续应考虑任务队列、限流、超时和重试策略。

## 10. 测试组织

测试集中放在 `backend/internal/tests`：

- `tests/agent`：Agent mention 和响应清洗行为。
- `tests/agent`：Agent mention、知识检索、guided dialogue 调度与停机行为。
- `tests/service`：AgentService 选择、冲突检测等服务行为。
- `tests/room`：房间创建和 Agent 快照选择行为。
- `tests/teststore`：测试用 Store 替身。

生产包目录只保留生产代码。测试通过包外调用验证公开边界，避免依赖内部实现细节。
