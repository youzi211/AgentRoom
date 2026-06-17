# AgentRoom 数据持久化设计

更新时间：2026-06-17

## 1. 背景

AgentRoom 已经从内存原型演进为 Go 后端、React 前端和 MySQL 持久化的会议工作台。当前持久化层需要支撑房间、参与者、消息、Agent、知识文档、Agent 执行记录、对话链路和会议纪要。

本文档描述当前数据底座的设计约束，方便后续改动继续沿用现有架构，而不是重复建设平行的数据系统。

## 2. 设计原则

1. **MySQL 是主要事实来源**
   房间、消息、Agent 配置、房间 Agent 快照、参与者、知识文档和会议纪要都应落库。

2. **业务层只依赖 Store 接口**
   API、Room、Agent Runner 和 Service 层不直接写 SQL。MySQL 细节集中在 `backend/internal/store/mysql`。

3. **运行时状态和持久化状态分离**
   WebSocket Hub、在线参与者连接和活跃房间缓存仍是运行时内存组件；历史记录和可恢复状态进入数据库。

4. **全局 Agent 与房间 Agent 快照分离**
   管理端修改全局 Agent 后，只影响新会议。已有会议使用 `room_agents` 快照，保证历史会议行为边界稳定。

5. **先复用文本知识检索**
   Markdown 文档先进入 `knowledge_documents` 和 `knowledge_chunks`。当前不引入向量数据库、后台索引器或新的检索服务。

6. **增量字段保持向后兼容**
   新增消息来源等能力时优先使用可选字段和 nullable/text JSON 列，避免破坏旧消息和已有 API 消费者。

## 3. 当前持久化范围

当前应持久化：

- Agent 全局配置。
- Room 元数据和生命周期状态。
- Room 内 Agent 快照。
- Participant 加入、离开和在线状态。
- Message 历史，包括 dialogue 元数据和知识来源摘要。
- Agent run 与 dialogue run。
- Knowledge document 和 chunk。
- Meeting minutes 多版本。

当前不做：

- 正式账号体系。
- 组织、租户和成员权限表。
- Redis Pub/Sub 多实例实时同步。
- 向量索引、embedding 表或外部检索服务。
- 文件对象存储；Markdown 原始内容仍按当前实现处理。

## 4. Store 接口边界

`backend/internal/store.Store` 是业务层唯一持久化接口。新增能力应优先扩展该接口，测试替身位于 `backend/internal/tests/teststore`。

典型调用关系：

```text
api.Server
  -> service.RoomService / service.AgentService / service.KnowledgeService
  -> store.Store
  -> store/mysql
```

Agent Runner 通过 Store 写入 Agent run、dialogue run 和 Agent 消息，但不直接接触 MySQL 模型。

## 5. 核心表

| 表 | 说明 |
| --- | --- |
| `agents` | 全局 Agent 配置 |
| `rooms` | 会议房间元数据、状态、口令哈希和生命周期时间 |
| `room_agents` | 房间创建时的 Agent 快照 |
| `participants` | 参与者加入、离开和在线记录 |
| `messages` | 人类、Agent 和系统消息 |
| `agent_runs` | 单次 Agent 执行审计 |
| `dialogue_runs` | 多轮 Agent 对话链路 |
| `knowledge_documents` | Markdown 知识文档元数据 |
| `knowledge_chunks` | 知识文档切分后的文本片段 |
| `meeting_minutes` | 会议纪要版本 |
| `schema_migrations` | 内置迁移执行记录 |

## 6. 消息模型

`messages` 是会议时间线的主要持久化表。当前领域模型 `model.Message` 包含：

- 基本消息字段：ID、房间 ID、发送者、内容、创建时间。
- Dialogue 元数据：`dialogueRunID`、`turnIndex`、`parentMessageID`。
- 知识来源摘要：`knowledgeSources`。

`knowledgeSources` 由 Agent Runner 根据实际检索到的知识片段确定性生成，并以 JSON 文本保存在 `knowledge_sources_json`。它不是模型自行声称的引用，因此可以反映真实检索来源。

## 7. 知识文档模型

知识文档分为两种 scope：

- `room`：会议级知识，会议内所有 Agent 可参考。
- `agent`：Agent 级知识，仅对应 Agent 发言时参考。

上传流程：

```text
UploadRoomKnowledge / UploadAgentKnowledge
  -> KnowledgeService 解析 Markdown
  -> SaveKnowledgeDocument
  -> SaveKnowledgeChunks
```

检索流程：

```text
Agent Runner
  -> KnowledgeService.SearchForAgent
  -> SearchKnowledgeChunks(room scope)
  -> SearchKnowledgeChunks(agent scope)
  -> prompt context + Message.knowledgeSources
```

`KnowledgeChunk` 会保留 `DocumentID` 和 `DocumentName`。MySQL 搜索通过 `knowledge_chunks` join `knowledge_documents` 获取文件名，不需要修改 chunk 表结构。

## 8. 迁移策略

迁移文件位于：

```text
backend/internal/store/mysql/migrations/
```

当前包括：

- `001_initial_schema.sql`
- `002_meeting_minutes.sql`
- `003_room_lifecycle.sql`
- `004_message_sources.sql`

`004_message_sources.sql` 为 `messages` 增加 `knowledge_sources_json`，用于保存 Agent 消息的确定性知识来源摘要。

开发环境可通过 `DB_AUTO_MIGRATE=true` 自动执行迁移。生产环境应保留显式迁移窗口和备份策略。

## 9. 环境变量

关键配置：

```env
DB_DRIVER=mysql
MYSQL_DSN=agentroom:agentroom_password@tcp(127.0.0.1:3306)/agentroom?parseTime=true&charset=utf8mb4&loc=UTC
DB_AUTO_MIGRATE=true
```

要求：

- `MYSQL_DSN` 必须包含 `parseTime=true`。
- 推荐使用 `charset=utf8mb4`，保证中文消息和文件名完整存储。
- 推荐使用 `loc=UTC`，统一时间语义。
- 生产环境不应默认打开自动迁移，除非部署流程已明确验证。

## 10. 后续演进建议

- 为 Agent 增加可选 `templateID` 字段，让角色模板和角色组能稳定关联持久化 Agent。
- 为知识来源芯片增加文档预览读取路径，但仍复用现有知识文档表。
- 在检索质量成为真实瓶颈后，再评估 BM25、hybrid search 或向量库。
- 引入正式账号体系前，先设计会议权限矩阵和管理员边界。
- 多实例部署前，补齐实时事件广播、任务队列和连接治理设计。
