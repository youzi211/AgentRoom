# AgentRoom 系统架构文档

> 当前架构已按主题拆分到 [`architecture/README.md`](architecture/README.md)。本文件保留为兼容入口和集中式摘要；开发前请优先阅读新索引及对应分册。

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
| DeepAgent | Python 3.11+ / `uv` / DeepAgents，由 Go 后端按次启动子进程 |
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

API Server 主要通过聚焦的 Query、Command、Access 和 Model Profile 端口执行业务用例，不直接依赖 MySQL/GORM 实现。当前仍有少量过渡边界：Handler 直接读取 `*room.Room`、调用静态 Agent 模板，并识别部分 `store` typed errors。

### 3.2 Service 层

`AgentService` 管理全局 Agent 配置：

- `Agents()` 返回可编辑的全局 Agent 配置。
- `CreateAgent()` 创建 Agent，并自动派生 `Mention = "@" + Name`。
- `UpdateAgent()` 更新 Agent，并检查 mention 冲突。
- `DeleteAgent()` 删除全局 Agent。
- `ResolveForRoom(agentIDs)` 根据创建房间请求选择要快照进房间的 Agent。

`ModelProfileService` 统一管理 `go` 与 `deepagent` 两个 runtime scope 的 OpenAI-compatible Profile，负责字段校验、每个 scope 的默认值、连接测试、引用约束和 API Key 脱敏。`ModelResolver` 在每次调用时解析房间快照、数据库默认值或迁移期环境兜底；它不缓存解密后的 API Key。

角色模板和角色组定义在 `agent/templates.go` 中。它们是静态产品默认值，用于预填 Agent 创建表单和提供会议创建时的选择快捷方式，不作为独立数据库实体，也不绕过 `AgentService` 的全局 Agent 管理。

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
- 检索结果会携带文档名称，供提示词来源标签和消息来源展示使用。
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
- 根据实际检索到的知识片段生成确定性的 `Message.KnowledgeSources`，避免依赖模型自行声明引用。
- 记录 `agent_runs` 执行状态。
- 记录 `dialogue_runs`，并把 `dialogue_run_id` / `turn_index` / `parent_message_id` 写入生成消息。
- 在 Agent run 与 dialogue run 开始/结束时广播 `agent_activity` 事件，供前端实时展示运行状态。
- 按 Agent Runtime 把调用交给 Go LLM 或 DeepAgent 适配器，并记录实际 Profile、来源和模型名。

Go LLM Runtime 每次调用都通过 `ModelResolver` 获取当前 Profile 内容，再创建 OpenAI-compatible 客户端。DeepAgent Runtime 解析 `deepagent` scope Profile 后启动一次 Python 子进程，并向继承自 backend 的进程环境追加本次 `MODEL_PROTOCOL`、`MODEL_BASE_URL`、`MODEL_NAME` 与 `MODEL_API_KEY`。模型凭据不会写入 argv、DeepAgent 配置文件、报告或事件日志；但由于当前以 `os.Environ()` 为基础，Python 子进程实际与 backend 处于同一高权限信任域，并能看到其他 backend 环境变量。

Runner 依赖 `RuntimeRoom` 接口，而不是具体 `*room.Room`：

```go
type RuntimeRoom interface {
    Info() model.RoomMeta
    Participants() []model.Participant
    Agents() []model.Agent
    AgentsWithPrompts() []model.Agent
    RecentMessages(limit int) []model.Message
    NewSystemMessage(content string) model.Message
    NewAgentMessage(agent model.Agent, content string) model.Message
    AppendMessage(message model.Message)
    Broadcaster() room.MessageBroadcaster
}
```

这个边界让 Agent 执行逻辑只依赖最小运行时能力，未来可以替换为队列任务、跨实例执行上下文或测试替身。

### 3.5 Store 层

`internal/store.Store` 是较宽的聚合持久化接口；业务组件也按消费者需要定义窄 Store 接口。它们都由 MySQL Store 实现，业务层不直接依赖 MySQL/GORM 具体实现。

主要数据能力：

- Agent 配置：seed/list/create/update/delete。
- 房间：create/get/list room agents。
- 参与者：join/leave/list active。
- 消息：add/list。
- Agent 执行记录：create/finish/list run。
- Dialogue 执行记录：create/finish/list run。
- 模型 Profile：列表、按 ID/默认值查询、创建、更新、事务切换默认值、引用计数与受约束删除。

MySQL 实现在 `internal/store/mysql` 中，GORM model 和 domain model 分离，通过转换函数连接。

## 4. 数据模型

核心表：

| 表 | 用途 |
| --- | --- |
| `agents` | 全局 Agent 配置模板 |
| `model_profiles` | Go/DeepAgent 模型连接 Profile；API Key 仅保存认证加密密文与不可逆提示 |
| `rooms` | 房间元数据 |
| `room_agents` | 房间创建时的 Agent 快照 |
| `participants` | 参与者与在线状态 |
| `messages` | 会议消息 |
| `agent_runs` | Agent 执行审计记录 |
| `dialogue_runs` | 多轮 Agent 对话链路与停止状态 |
| `knowledge_documents` | 知识文档元数据，支持 `room` 与 `agent` 两种作用域 |
| `knowledge_chunks` | Markdown 解析后的文本片段，用于 Agent 发言上下文 |
| `schema_migrations` | 为迁移治理预留的兼容结构；当前生产代码不读写版本记录 |

关键设计：房间创建时会把选中的全局 Agent 配置复制到 `room_agents`。之后全局 Agent 的更新不会影响已有房间。

若 Agent 已显式绑定 Profile，或建房时存在对应 runtime 的数据库默认 Profile，模型选择会快照为非空的 `room_agents.model_profile_id`；这类房间不受后续全局 Agent 改绑或默认值切换影响。若建房时没有可用数据库 Profile，该字段保持空值，旧房间会在每次调用时解析当前数据库默认值或环境兜底。被具体 ID 引用的 Profile 连接内容（Base URL、模型名、API Key）仍在每次调用时读取当前值，因此密钥轮换无需重建房间。`agent_runs` 记录实际 Profile ID、配置来源和模型名，不记录 API Key。

`messages.knowledge_sources_json` 保存 Agent 回复关联的知识来源摘要。该字段从检索结果派生，属于可选元数据；旧消息没有该字段时仍可正常展示。

`knowledge_chunks` 不重复保存文件名。检索时通过 `knowledge_documents` join 补齐 `KnowledgeChunk.DocumentName`，让来源信息留在既有知识文档模型里。

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
  -> RoomCommands.PostRealtimeMessage
  -> RoomService.HandleHumanMessage
  -> store.AddMessage 先持久化人类消息
  -> room.AppendMessage 更新内存缓存
  -> Room.Broadcaster().BroadcastMessage 广播人类消息
  -> TriggerAgentResponses 投入容量 64、4 worker 的进程内有界队列
  -> worker 调用 agent.Runner.HandleHumanMessage
  -> Runner 检测房间 DialoguePolicy
  -> mention_fanout: 被 @ 的 Agent 各回复一次
  -> guided_dialogue: 创建 dialogue_run，按策略选择首个与后续发言者
  -> Room.Broadcaster().BroadcastEvent 广播 agent_activity started 事件
  -> ModelResolver 按快照 Profile / 数据库 runtime 默认值 / 环境迁移兜底解析
  -> Go llm.Client 或 DeepAgent Python 子进程调用模型
  -> agent.StripThinkBlocks 清洗响应
  -> store.AddMessage 尝试持久化 Agent 消息
  -> Room.Broadcaster().BroadcastMessage 广播 Agent 消息
  -> 达到轮次上限 / 无可选发言者 / 重复内容 / provider error 时停止 dialogue_run
  -> Room.Broadcaster().BroadcastEvent 广播 agent_activity finished 事件
```

## 6. HTTP API

基础路径：`/api`

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/health` | 健康检查 |
| GET | `/agents` | 获取全局 Agent 配置 |
| GET | `/agent-templates` | 获取静态 Agent 角色模板 |
| GET | `/agent-role-sets` | 获取创建会议时的推荐角色组 |
| POST | `/agents` | 创建 Agent |
| PUT | `/agents/:agentID` | 更新 Agent |
| DELETE | `/agents/:agentID` | 删除 Agent |
| GET/POST | `/model-profiles` | 列出或创建模型 Profile（管理员） |
| PUT/DELETE | `/model-profiles/:profileID` | 更新或受约束删除模型 Profile（管理员） |
| POST | `/model-profiles/:profileID/default` | 设为对应 runtime 默认 Profile（管理员） |
| POST | `/model-profiles/:profileID/test` | 对已保存 Profile 发起最小连接测试（管理员） |
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
| `/models` | Go / DeepAgent 模型 Profile 管理页 |
| `/rooms/:roomID` | 会议室或加入确认页 |

## 8. 配置

| 变量 | 必需 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `PORT` | 否 | `8080` | HTTP 端口 |
| `MYSQL_DSN` | 是 | - | MySQL DSN |
| `DB_AUTO_MIGRATE` | 否 | `false` | 是否自动迁移数据库 |
| `MODEL_CONFIG_ENCRYPTION_KEY` | 保存模型密钥时必需 | - | base64 编码的 32 字节主密钥；必须备份并在所有后端实例保持一致 |
| `LLM_BASE_URL` / `LLM_API_KEY` / `LLM_MODEL` | 否 | - | 仅在无数据库 Go 默认 Profile 时使用的迁移兜底，不自动导入数据库 |
| `MODEL_BASE_URL` / `MODEL_API_KEY` / `MODEL_NAME` | 否 | - | 仅在无数据库 DeepAgent 默认 Profile 时使用的迁移兜底，不写回 DeepAgent 文件 |
| `TAVILY_API_KEY` | DeepAgent 联网研究时必需 | - | DeepAgent 搜索凭据，不属于模型 Profile |
| `DEEPAGENT_WORKDIR` | 否 | `../deepagent` | Python 项目目录；Compose 固定为 `/app/deepagent` |
| `LOG_LEVEL` | 否 | `info` | 日志级别 |
| `LOG_FORMAT` | 否 | `text` | `text` 或 `json` |
| `LOG_ADD_SOURCE` | 否 | `false` | 是否输出源码位置 |

## 9. 安全与生产风险

### 9.1 模型解析优先级

模型配置按确定顺序解析：

1. 房间 Agent 快照中的具体 Profile ID；
2. 对旧房间空绑定及 runtime 默认调用，使用对应 scope 的数据库默认 Profile；
3. 数据库没有可用默认值时，Go 使用 `LLM_*`、DeepAgent 使用 `MODEL_*` 迁移兜底；
4. 所有来源均不可用时返回“模型未配置”。

显式 Profile 缺失、停用、scope 不匹配或不可解密时立即失败，不得降级到默认值或环境变量。环境变量不自动导入数据库；管理页面只显示非敏感的兜底状态，不读取或回显环境 API Key。

### 9.2 密钥边界

`MODEL_CONFIG_ENCRYPTION_KEY` 是服务端主密钥，必须是 base64 编码的 32 字节随机值。Profile API Key 使用 AES-256-GCM 加密后写入 MySQL，管理 API 永不返回明文或可识别密文。主密钥应放在部署 Secret Store 中并纳入加密备份；丢失或误换后，现有 Profile 密文无法恢复，只能为受影响 Profile 重新录入 API Key。

解密明文只在单次后端调用内存中短暂存在。DeepAgent 模型凭据通过该次 Python 子进程环境传递，不进入 argv、`.env`、TOML、报告或事件。当前子进程同时继承 backend 的完整进程环境，所以它不是凭据隔离的沙箱，必须作为与 backend 同等级的高权限主体管理。`TAVILY_API_KEY`、输出目录和搜索限制仍按独立 DeepAgent 运维配置管理。

### 9.3 Docker DeepAgent 可用性

生产 backend 镜像基于 Python 3.12 slim，包含固定版本 `uv`、按 `deepagent/uv.lock` 安装的生产依赖、DeepAgent 源码、registry 和非密钥 TOML 默认值。Compose 显式设置 `/app/deepagent` 工作目录，并把 `runs/` 持久化到 `deepagent-runs` volume。因此容器内存在真实 Python 执行器；运行模型连接由 Go 后端按次注入，联网研究还需要单独提供 `TAVILY_API_KEY`。

其他仍需在上线前评估的风险：

- WebSocket `CheckOrigin` 仅在 `ALLOWED_ORIGINS` 非空时执行 allowlist；生产必须配置实际前端 Origin。
- Agent 和模型管理 API 在 `ADMIN_API_KEY` 非空时受保护；空值会放行，只适合明确的本地开发环境。
- 房间加入仍以 query name 区分用户，没有正式用户体系。
- 缺少消息发送失败重试与幂等机制。
- 单实例内存 Hub 不能支持多后端实例广播，需要 Redis Pub/Sub 或消息总线。
- Agent 执行已使用容量 64、4 worker 的进程内有界队列，但任务不持久化，后续仍需评估 durable queue、配额、重试和跨实例执行。
- 前端 build-time `VITE_ADMIN_API_KEY` 会进入浏览器 bundle，不能承担公网生产鉴权。

## 10. 测试组织

大多数测试集中放在 `backend/internal/tests`：

- `backend/internal/tests/agent`：Agent mention、响应清洗、知识检索、Runtime 和 guided dialogue 行为。
- `backend/internal/tests/service`：AgentService、房间用例、队列、生命周期和模型 Profile 行为。
- `backend/internal/tests/room`：房间创建、快照加载和运行时治理。
- `backend/internal/tests/api`：HTTP、WebSocket、鉴权和响应合约。
- `backend/internal/tests/teststore`：测试用 Store 替身。

大多数测试通过包外调用验证公开边界；少量必须访问 MySQL 包内部迁移细节的测试与实现共置，例如 `backend/internal/store/mysql/model_profiles_migration_test.go`。
