# AgentRoom 数据持久化设计

> 目标读者：后续负责实现的 Claude / Codex / 人类工程师。
>
> 当前阶段目标：把 AgentRoom 从内存态原型推进到可上线演进的服务端数据底座。本文是实现规格，不是头脑风暴记录。

## 1. 背景

当前 AgentRoom 已具备：

- Go 后端：房间、成员、消息、Agent 触发、WebSocket fanout、LLM 调用。
- React 前端：会议入口、Agent 管理页、会议室页、URL 路由。
- 后端 API 命名空间：`/api/*`。
- `.env` 配置加载。
- OpenAI 官方 Go SDK 调用 OpenAI-compatible Chat Completions。

当前主要问题：

- 房间存在于 `room.Manager` 的内存 map 中，后端重启后丢失。
- 消息存在于 `room.Room.messages` 内存切片中，后端重启后丢失。
- Agent 配置存在于 `room.Manager.agents` 内存切片中，后端重启后恢复默认值。
- `/rooms/:roomId` 已经是可分享 URL，但没有持久化后，分享价值有限。
- 会议参与者身份仍是浏览器本地临时态，后端没有稳定访客身份记录。

下一步应先补数据底座，再继续做知识库、会议纪要、权限、多实例扩展。

## 2. 数据库选择

主数据库选择：**MySQL**。

不把 SQLite 作为主线，原因：

- 项目目标已从 demo 升级为需要上线的应用。
- 会议消息、成员状态、Agent 响应会产生持续写入。
- 后续可能需要多后端实例、云数据库、备份、监控、权限管理。
- MySQL 更适合承载房间、消息、Agent 配置、会议纪要等关系型数据。

允许保留测试或本地 fake store，但第一阶段不要实现 SQLite 分支，避免分散交付。

## 3. 设计原则

1. **MySQL 是持久化事实来源**  
   房间、消息、Agent 配置、房间 Agent 快照都应落库。

2. **WebSocket Hub 仍然是运行时内存组件**  
   当前阶段不做 Redis pub/sub，不做多实例实时同步。持久化层先解决重启不丢数据。

3. **业务代码依赖 Store 接口，不直接依赖 MySQL**  
   API、Room、Agent Runner 不应到处写 SQL。

4. **Agent 全局配置和房间内 Agent 快照分离**  
   管理页修改全局 Agent 后，默认只影响新房间。已创建房间使用 `room_agents` 快照，避免历史会议的 Agent 行为边界被后台配置悄悄改变。

5. **先用 `database/sql`，不引入 ORM**  
   推荐依赖 `github.com/go-sql-driver/mysql`。迁移可以先用项目内置小型 migration runner，避免为了第一阶段引入复杂迁移框架。

6. **事务边界清晰**  
   创建房间、写消息、创建参与者、创建 Agent 运行记录等必须有明确事务或幂等策略。

## 4. 目标范围

本阶段必须完成：

- Agent 配置持久化。
- 房间持久化。
- 房间 Agent 快照持久化。
- 参与者加入/离开记录持久化。
- 消息持久化。
- 后端重启后，可以通过 `/api/rooms/:roomID` 找回房间。
- 后端重启后，可以通过 `/api/rooms/:roomID/messages` 找回历史消息。
- Agent 管理页保存后，配置进入 MySQL。
- 启动时如果 `agents` 表为空，使用 `agent.PredefinedAgents()` 初始化默认 Agent。
- `.env.example` 增加 MySQL 配置。
- 加回最小测试集，覆盖 store 和关键 API。

本阶段不做：

- 正式登录系统。
- 组织/租户体系。
- Redis pub/sub 多实例实时广播。
- 向量知识库。
- 会议纪要持久化 UI。
- 完整权限系统。
- 数据库分库分表。

## 5. 建议目录结构

```text
backend/internal/store/
  store.go                 # Store 接口和查询参数
  mysql/
    store.go               # MySQL 实现
    migrations.go          # 内置迁移执行器
    migrations/
      001_initial_schema.sql
  memory/
    store.go               # 测试用 fake store，可选
```

如果时间有限，`memory` 可以先不做，但测试仍需覆盖 MySQL store 或 API 层。

## 6. 环境变量

新增配置：

```env
DB_DRIVER=mysql
MYSQL_DSN=agentroom:agentroom_password@tcp(127.0.0.1:3306)/agentroom?parseTime=true&charset=utf8mb4&loc=UTC
DB_AUTO_MIGRATE=true
```

说明：

- `DB_DRIVER` 第一阶段只支持 `mysql`。
- `MYSQL_DSN` 必须包含 `parseTime=true`，否则 Go 扫描 `time.Time` 会出问题。
- `charset=utf8mb4` 是必须项，Agent 名称、中文消息、emoji 都需要完整 UTF-8 支持。
- `loc=UTC` 推荐统一存储 UTC 时间。
- `DB_AUTO_MIGRATE=true` 允许开发环境启动时自动建表；生产环境可改为手动执行迁移。

## 7. 数据模型

### 7.1 agents

全局 Agent 配置表。管理页读写这张表。

```sql
CREATE TABLE agents (
  id VARCHAR(64) PRIMARY KEY,
  name VARCHAR(128) NOT NULL,
  mention VARCHAR(128) NOT NULL,
  role VARCHAR(128) NOT NULL,
  description TEXT NOT NULL,
  system_prompt TEXT NOT NULL,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  sort_order INT NOT NULL DEFAULT 0,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  UNIQUE KEY uk_agents_mention (mention)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
```

实现规则：

- `mention` 由 `name` 派生：`@` + `name`。
- 管理页修改 `name` 时必须同步更新 `mention`。
- `sort_order` 用于稳定展示顺序，不要依赖 MySQL 默认行顺序。

### 7.2 rooms

会议房间表。

```sql
CREATE TABLE rooms (
  id VARCHAR(64) PRIMARY KEY,
  name VARCHAR(255) NOT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'active',
  owner_participant_id VARCHAR(64) NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  closed_at DATETIME(6) NULL,
  closed_reason VARCHAR(32) NOT NULL DEFAULT '',
  auto_close_deadline_at DATETIME(6) NULL,
  archived_at DATETIME(6) NULL,
  KEY idx_rooms_status_created_at (status, created_at),
  KEY idx_rooms_auto_close_deadline (auto_close_deadline_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
```

`status` 预留：

- `active`
- `archived`
- `deleted`

第一阶段只使用 `active`。

2026-06 生命周期补充：
- `rooms.status` 现在正式区分 `active`、`closed`、`archived`。
- `owner_participant_id` 记录当前在线人类房主；只有房主可以关闭会议或把房主转交给另一位在线人类。
- `closed_at` / `closed_reason` 记录会议结束语义；`closed_reason` 至少覆盖 `manual`、`last_human_left`、`admin_unarchive`。
- `auto_close_deadline_at` 用于“最后一位人类离开后 30 秒自动关闭”的宽限期。
- `archived` 恢复后应进入 `closed`，只有 `reopen` 才能回到 `active`。

### 7.3 room_agents

房间内 Agent 快照表。房间创建时，把当时启用的全局 Agent 复制一份到这里。

```sql
CREATE TABLE room_agents (
  room_id VARCHAR(64) NOT NULL,
  agent_id VARCHAR(64) NOT NULL,
  name VARCHAR(128) NOT NULL,
  mention VARCHAR(128) NOT NULL,
  role VARCHAR(128) NOT NULL,
  description TEXT NOT NULL,
  system_prompt TEXT NOT NULL,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  sort_order INT NOT NULL DEFAULT 0,
  created_at DATETIME(6) NOT NULL,
  PRIMARY KEY (room_id, agent_id),
  KEY idx_room_agents_mention (room_id, mention),
  CONSTRAINT fk_room_agents_room FOREIGN KEY (room_id) REFERENCES rooms(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
```

关键约束：

- `room_agents.system_prompt` 可以被 Agent Runner 使用，但不能通过普通房间 API 暴露给前端会议室。
- `/api/rooms/:roomID` 返回的 Agent 仍应走 `model.Agent.Public()` 风格，隐藏系统提示词。
- 管理页修改 `agents` 后，不自动修改已创建房间的 `room_agents`。

### 7.4 participants

参与者表。第一阶段不做正式账号，只记录会议参与者实例和可选访客 key。

```sql
CREATE TABLE participants (
  id VARCHAR(64) PRIMARY KEY,
  room_id VARCHAR(64) NOT NULL,
  display_name VARCHAR(128) NOT NULL,
  guest_key VARCHAR(128) NULL,
  joined_at DATETIME(6) NOT NULL,
  last_seen_at DATETIME(6) NOT NULL,
  left_at DATETIME(6) NULL,
  KEY idx_participants_room_active (room_id, left_at),
  KEY idx_participants_guest (guest_key),
  CONSTRAINT fk_participants_room FOREIGN KEY (room_id) REFERENCES rooms(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
```

第一阶段规则：

- WebSocket 连接成功时创建 participant。
- WebSocket 断开时设置 `left_at`。
- `Participants()` 默认只返回 `left_at IS NULL` 的在线参与者。
- `guest_key` 可先允许为空，为下一阶段访客身份做铺垫。

### 7.5 messages

消息表。

```sql
CREATE TABLE messages (
  id VARCHAR(64) PRIMARY KEY,
  room_id VARCHAR(64) NOT NULL,
  sender_id VARCHAR(64) NOT NULL,
  sender_name VARCHAR(128) NOT NULL,
  sender_type VARCHAR(32) NOT NULL,
  content TEXT NOT NULL,
  created_at DATETIME(6) NOT NULL,
  KEY idx_messages_room_created (room_id, created_at, id),
  CONSTRAINT fk_messages_room FOREIGN KEY (room_id) REFERENCES rooms(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
```

`sender_type` 允许值：

- `human`
- `agent`
- `system`

实现规则：

- 写消息必须先落库，再进入内存消息列表和 WebSocket 广播。
- `GET /api/rooms/:roomID/messages` 第一阶段可继续返回最近消息，但要为分页参数预留。
- 建议支持查询参数：
  - `limit`，默认 100，最大 500。
  - `before`，可选，使用消息 `created_at/id` 游标后续实现。

2026-06 消息历史补充：
- `GET /api/rooms/:roomID/messages` 使用 cursor 分页，返回 `{ messages, hasMore, nextBefore }`。
- 默认返回最新一页，但页内顺序保持“从旧到新”。
- `before` 指向上一页最早消息的 id；非法或跨房间 cursor 应返回 `400`。
- `closed` 房间允许普通用户只读查看消息历史；`archived` 房间仅管理员可读。

### 7.6 agent_runs

Agent 响应运行记录表。第一阶段建议实现，哪怕 UI 暂时不展示。

```sql
CREATE TABLE agent_runs (
  id VARCHAR(64) PRIMARY KEY,
  room_id VARCHAR(64) NOT NULL,
  agent_id VARCHAR(64) NOT NULL,
  trigger_message_id VARCHAR(64) NOT NULL,
  status VARCHAR(32) NOT NULL,
  error TEXT NULL,
  started_at DATETIME(6) NOT NULL,
  completed_at DATETIME(6) NULL,
  KEY idx_agent_runs_room (room_id, started_at),
  KEY idx_agent_runs_trigger (trigger_message_id),
  CONSTRAINT fk_agent_runs_room FOREIGN KEY (room_id) REFERENCES rooms(id),
  CONSTRAINT fk_agent_runs_trigger FOREIGN KEY (trigger_message_id) REFERENCES messages(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
```

`status` 允许值：

- `running`
- `succeeded`
- `failed`
- `timeout`

用途：

- 排查 Agent 为什么没有响应。
- 后续支持“Agent 正在思考”的 UI。
- 后续支持重试和异步任务队列。

## 8. Store 接口设计

建议先定义一个聚合接口，后续可以拆分：

```go
package store

import (
    "context"
    "time"

    "agentroom/backend/internal/model"
)

type Store interface {
    Ping(ctx context.Context) error
    Close() error

    SeedAgents(ctx context.Context, agents []model.Agent) error
    ListAgents(ctx context.Context) ([]model.Agent, error)
    UpdateAgent(ctx context.Context, agent model.Agent) (model.Agent, error)

    CreateRoom(ctx context.Context, input CreateRoomInput) (model.RoomMeta, []model.Agent, error)
    GetRoom(ctx context.Context, roomID string) (model.RoomMeta, error)
    ListRoomAgents(ctx context.Context, roomID string) ([]model.Agent, error)

    AddParticipant(ctx context.Context, input AddParticipantInput) (model.Participant, error)
    MarkParticipantLeft(ctx context.Context, participantID string, leftAt time.Time) error
    ListActiveParticipants(ctx context.Context, roomID string) ([]model.Participant, error)

    AddMessage(ctx context.Context, message model.Message) (model.Message, error)
    ListMessages(ctx context.Context, query ListMessagesQuery) ([]model.Message, error)

    CreateAgentRun(ctx context.Context, run AgentRun) error
    FinishAgentRun(ctx context.Context, runID string, status string, errText string, completedAt time.Time) error
}
```

建议配套输入类型：

```go
type CreateRoomInput struct {
    ID        string
    Name      string
    Agents    []model.Agent
    CreatedAt time.Time
}

type AddParticipantInput struct {
    ID          string
    RoomID      string
    DisplayName string
    GuestKey    string
    JoinedAt    time.Time
}

type ListMessagesQuery struct {
    RoomID string
    Limit  int
    Before string
}
```

## 9. Manager / Room 改造方案

### 9.1 当前职责

当前 `room.Manager` 同时承担：

- 全局 Agent 配置。
- 房间创建。
- 房间内存索引。
- Agent 配置更新后同步房间。

当前 `room.Room` 承担：

- 房间元信息。
- 在线参与者。
- 房间 Agent 列表。
- 消息列表。
- Hub。

### 9.2 改造后职责

`store.Store`：

- 负责持久化事实。
- 不负责 WebSocket Hub。
- 不负责 LLM 调用。

`room.Manager`：

- 持有 `store.Store`。
- 持有 active rooms map，用于 WebSocket Hub 和运行时对象复用。
- `GetRoom(roomID)` 如果内存不存在，应从 Store 加载 room meta、room agents、近期消息并构造运行时 Room。

`room.Room`：

- 仍持有 Hub。
- 仍持有在线内存状态。
- 增加对 Store 的写入路径，或者由 Manager/API 调用 Store 后再更新 Room。

推荐较小改法：

- `Manager` 注入 Store。
- `Room` 不直接依赖 Store，避免 Room 变重。
- API 层和 Manager 负责持久化，然后调用 Room 的内存更新方法。

### 9.3 创建房间流程

```text
POST /api/rooms
  -> Manager.CreateRoom(name)
  -> Store.ListAgents(enabled=true)
  -> 生成 roomID
  -> Store.CreateRoom(tx: rooms + room_agents snapshots)
  -> room.NewFromState(...)
  -> Manager.rooms[roomID] = room
  -> 返回 RoomMeta
```

### 9.4 获取房间流程

```text
GET /api/rooms/:roomID
  -> Manager.GetRoom(roomID)
  -> 如果内存有：返回内存 Room
  -> 如果内存没有：
       Store.GetRoom
       Store.ListRoomAgents
       Store.ListMessages(limit=100)
       Store.ListActiveParticipants
       room.NewFromSnapshot(...)
       放入 Manager.rooms
  -> 返回 RoomState
```

注意：

- 后端重启后，没有 WebSocket 在线连接时，`participants` 应为空或只有未 left 的记录。
- 更推荐启动后把旧 active participants 标记为离线，避免重启后显示幽灵在线用户。

### 9.5 WebSocket 加入流程

```text
GET /api/rooms/:roomID/ws?name=Alice
  -> Manager.GetRoom(roomID)
  -> Store.AddParticipant
  -> Room.AddParticipantFromStore(participant)
  -> Hub.Register
  -> Broadcast participant_joined
  -> Send room_snapshot
```

断开时：

```text
cleanup
  -> Hub.Unregister
  -> Store.MarkParticipantLeft(participantID)
  -> Room.RemoveParticipant(participantID)
  -> Broadcast participant_left
```

### 9.6 发消息流程

```text
Client message
  -> validate content
  -> 构造 model.Message
  -> Store.AddMessage
  -> Room.AppendMessage(message)
  -> Hub.Broadcast(message)
  -> agent.Runner.HandleHumanMessage(...)
```

Agent 消息、System 消息同理：必须先落库，再广播。

## 10. Agent 配置行为

### 10.1 全局 Agent 管理

`GET /api/agents`：

- 从 `agents` 表读取。
- 返回包含 `systemPrompt` 的 `AgentConfig`，其中该字段语义为 Agent 角色模板，仅供管理页。

`PUT /api/agents/:agentID`：

- 更新 `agents` 表。
- `name` 修改时同步 `mention`。
- 不自动更新既有房间的 `room_agents`。

### 10.2 房间 Agent 快照

房间创建时：

- 查询全局 enabled agents。
- 复制到 `room_agents`。

会议中提及检测：

- 使用 `room_agents` 快照。
- 禁用全局 Agent 不影响已创建房间。

需要同步 UI 文案：

- 当前 Agent 管理页文案如果写了“已存在房间也会同步更新”，应改为“新建房间使用最新配置；已存在房间保留创建时的 Agent 快照”。

## 11. API 影响

当前 API path 不变：

- `GET /api/agents`
- `PUT /api/agents/:agentID`
- `POST /api/rooms`
- `GET /api/rooms/:roomID`
- `GET /api/rooms/:roomID/messages`
- `GET /api/rooms/:roomID/ws?name=Alice`

建议扩展：

### 11.1 消息分页

`GET /api/rooms/:roomID/messages?limit=100&before=<cursor>`

第一阶段可以只实现 `limit`，但接口类型要预留 `before`。

### 11.2 健康检查增强

`GET /api/health` 当前只返回：

```json
{ "ok": true }
```

建议改为：

```json
{
  "ok": true,
  "database": {
    "ok": true
  },
  "llm": {
    "configured": true
  }
}
```

第一阶段至少后端启动时应 `Ping` MySQL；如果数据库不可用，应启动失败，而不是运行成半残状态。

## 12. 迁移策略

建议实现内置迁移：

```text
backend/internal/store/mysql/migrations/
  001_initial_schema.sql
```

再增加迁移记录表：

```sql
CREATE TABLE schema_migrations (
  version VARCHAR(64) PRIMARY KEY,
  applied_at DATETIME(6) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
```

启动逻辑：

```text
main.go
  -> LoadDotEnv
  -> Load DB config
  -> mysqlstore.Open
  -> store.Ping
  -> if DB_AUTO_MIGRATE=true: store.Migrate
  -> store.SeedAgents(agent.PredefinedAgents())
  -> room.NewManager(store)
```

迁移要求：

- 每个 migration 在事务中执行。
- 已执行 migration 不能重复执行。
- migration 失败时后端启动失败。

## 13. 测试计划

之前测试文件按用户要求删除了。现在进入上线标准后，必须恢复最小测试集。

### 13.1 Store 测试

覆盖：

- `SeedAgents` 首次写入。
- `SeedAgents` 重复执行不覆盖用户修改。
- `UpdateAgent` 保存 name/mention/role/description/system_prompt/enabled。
- `CreateRoom` 同时写入 `rooms` 和 `room_agents` 快照。
- 修改全局 Agent 后，旧房间 `room_agents` 不变。
- `AddMessage` 后按创建时间读取。
- `AddParticipant` 和 `MarkParticipantLeft` 后在线列表变化。

### 13.2 API 测试

覆盖：

- `POST /api/rooms` 返回房间并落库。
- 重建 Manager 后 `GET /api/rooms/:roomID` 仍能返回房间。
- `GET /api/rooms/:roomID/messages` 重启后仍能返回消息。
- `PUT /api/agents/:agentID` 重启后仍保留配置。

### 13.3 WebSocket smoke test

覆盖：

- 创建房间。
- 建立 WebSocket。
- 发送 human message。
- REST 拉取 messages 能看到同一条消息。

### 13.4 LLM 不可用测试

覆盖：

- `LLM_API_KEY` 为空或 LLM 返回错误时，system message 落库。
- 系统消息通过 REST 能查到。

## 14. 实施顺序

按以下顺序实现，避免大面积返工：

1. 增加 DB 配置读取和 `.env.example`。
2. 新建 `backend/internal/store` 接口。
3. 实现 MySQL 连接、Ping、Close。
4. 实现 migration runner 和 `001_initial_schema.sql`。
5. 实现 `SeedAgents`、`ListAgents`、`UpdateAgent`。
6. 改造 Agent 管理 API 使用 Store。
7. 实现 `CreateRoom`、`GetRoom`、`ListRoomAgents`。
8. 改造 `room.Manager.CreateRoom/GetRoom`。
9. 实现 participants 持久化。
10. 实现 messages 持久化。
11. 改造 human/agent/system message 写入路径。
12. 实现 agent_runs 记录。
13. 更新 Agent 管理页文案，明确既有房间保留 Agent 快照。
14. 恢复最小测试集。
15. 更新 README 和 backend README。

## 15. Claude 实现注意事项

请严格遵守：

- 不要把 API 路径从 `/api/*` 改回裸 `/agents` 或 `/rooms`。
- 不要提交 `.env`。
- 不要在日志或错误里打印完整 `MYSQL_DSN`、`LLM_API_KEY`。
- 不要把 `room_agents.system_prompt` 暴露给普通房间详情接口。
- 不要让全局 Agent 更新自动覆盖既有房间快照。
- 不要一口气引入 ORM、Redis、登录系统、知识库。
- 不要把 WebSocket Hub 持久化；Hub 是运行时连接管理，MySQL 是事实存储。
- 每个落库写路径要考虑失败时的用户可见错误，不要静默失败。

## 16. 验收标准

实现完成后，至少满足：

- 后端连接 MySQL 成功后才能启动。
- 启动后 `agents` 表为空时自动种子初始化。
- 管理页修改 Agent，重启后配置仍存在。
- 创建房间，重启后 `/rooms/:roomId` 仍可访问。
- 发送消息，重启后历史消息仍可读取。
- 创建房间时使用 Agent 快照，后续修改全局 Agent 不影响旧房间。
- `go test ./...` 通过。
- `npm --prefix frontend run build` 通过。
- README 描述 MySQL 配置、迁移和运行方式。

## 17. 后续阶段

本阶段完成后，下一阶段建议继续：

1. 访客身份：`guest_key` 正式接入 localStorage 和后端识别。
2. 消息分页：支持 `before` 游标。
3. Agent 响应状态：基于 `agent_runs` 显示“正在思考 / 失败 / 重试”。
4. 会议纪要：新增 `meeting_summaries` 表和秘书总结保存。
5. 多实例实时：Redis pub/sub 或消息队列。
6. 权限模型：房间邀请链接、主持人、只读访问。
## 18. 2026-06 会议生命周期补丁

本轮实现后，接口与权限语义需要以以下内容为准：

- 会议状态固定为 `active`、`closed`、`archived` 三态，不再把“已关闭”和“已归档”混为同一状态。
- 在线人类房主可以通过 WebSocket 发送 `close_room` 或 `transfer_owner`；不新增独立的参与者鉴权 HTTP 变更接口。
- 当最后一位人类离开时，后端写入 `auto_close_deadline_at = now + 30s`；若 30 秒内无人重返，房间自动转为 `closed`。
- `GET /api/rooms/:roomID`、`GET /messages`、`GET /minutes.md`：普通用户在 `closed` 状态下仍可访问，但 `archived` 仅管理员可读。
- `GET /api/rooms/:roomID/ws?name=Alice`：只允许 `active`；`closed` 必须走只读查看，不能加入实时连接。
- `POST /api/rooms/:roomID/minutes`：普通用户只允许在 `active` 生成；管理员在三种状态下都可以生成和保存纪要。
- `POST /api/rooms/:roomID/reopen`：仅管理员可用，只支持 `closed -> active`。
- `POST /api/rooms/:roomID/restore`：仅管理员可用，只支持 `archived -> closed`。
- `GET /api/rooms/:roomID/minutes.md` 是纯读取接口；如果没有已持久化纪要，返回 `404`，不再隐式生成。
