# AgentRoom 架构索引

> 当前 Agent 执行拓扑已演进为 Go 控制面 + Python `agent-runtime` gRPC 执行面。Compose 包含 `frontend`、`backend`、`agent-runtime`、`mysql` 四个服务；旧的 Go LLM Agent Runtime 和 DeepAgent 子进程 Adapter 仅作为迁移期 `local` 回退保留。详细边界见 [Agent Runtime 与模型](agent-runtime-and-models.md) 和 [灰度与回滚](agent-runtime-rollout.md)。

本文档目录描述 AgentRoom **当前工作树已经实现的架构**，供项目维护者、Claude Code 和 Codex 在开发前快速建立上下文。它不是路线图，也不表示 OpenSpec 中尚未验收的行为已经通过端到端验证。

## 事实来源与阅读顺序

出现不一致时，按以下优先级判断当前行为：

1. `backend/cmd/server/main.go`、各生产包和前端生产代码；
2. `backend/internal/store/mysql` 的 GORM 模型、Repository 与迁移参考；
3. 本目录中的架构说明；
4. 根 `README.md`、`CLAUDE.md` 和 `AGENTS.md` 中的摘要；
5. `openspec/` 与历史设计文档——它们记录需求、变更和决策过程，不自动代表已落地现状。

代码变更导致本文档失真时，应在同一变更中更新相关章节。代码引用使用仓库相对路径和符号名，避免依赖易漂移的行号。

## 系统概览

AgentRoom 是一个 Go 模块化单体后端、React 单页前端和 MySQL 数据库组成的实时 AI 会议工作区。Go 后端同时承载 HTTP、WebSocket、房间运行时状态和 Agent 编排；DeepAgent 不是独立服务，而是由 Go 后端按次启动的 Python 子进程。

```text
                           Browser
                 React 18 + Mantine SPA
                    REST + WebSocket
                           |
                           v
                frontend / nginx container
                 static files + /api proxy
                           |
                           v
                backend container / process
        Go + Gin + gorilla/websocket + room Hub
                 |                       |
                 |                       +--> OpenAI-compatible model API
                 |                       |
                 |                       +--> Python DeepAgent child
                 |                              |--> model API
                 |                              |--> Tavily
                 |                              `--> runs/<runID>/
                 v
                    MySQL 8 / GORM
```

Docker Compose 由三个服务组成：

- `frontend`：nginx 托管 Vite 构建，并代理 `/api` 与 WebSocket；
- `backend`：Go 服务，以及镜像内置的 Python、`uv` 和 DeepAgent；
- `mysql`：房间、消息、配置和运行审计的持久化事实来源。

## 文档地图

| 文档 | 主要内容 | 修改这些代码前先读 |
| --- | --- | --- |
| [后端架构](backend.md) | 组合根、包依赖、API/Service/Room/Agent/Store 边界、后台任务 | `backend/cmd`、`backend/internal/api`、`service`、`room`、`store` |
| [Agent Runtime 与模型](agent-runtime-and-models.md) | mention、对话策略、Runner、Go/DeepAgent Runtime、Model Profile、密钥 | `backend/internal/agent`、`llm`、模型 Profile、`deepagent/` |
| [数据、实时与会议生命周期](data-realtime-lifecycle.md) | MySQL 与内存状态、建房快照、消息一致性、WebSocket、房间状态机 | schema、Repository、消息、房间、参与者、会议生命周期 |
| [前端与部署](frontend-and-deployment.md) | React 路由、页面和 API Client、Docker 拓扑、部署与浏览器安全边界 | `frontend/`、Dockerfile、Compose、nginx、部署配置 |

## 仓库模块地图

```text
agentRoom_test/
├── backend/
│   ├── cmd/server/                 # 唯一应用组合根
│   └── internal/
│       ├── api/                    # Gin HTTP/WS 协议适配
│       ├── service/                # 应用用例与协调
│       ├── room/                   # 内存房间、Manager、Hub
│       ├── agent/                  # Runner、对话策略、Runtime
│       ├── llm/                    # OpenAI-compatible 客户端
│       ├── store/                  # 持久化端口
│       ├── store/mysql/            # GORM/MySQL 实现
│       ├── model/                  # 共享领域类型
│       ├── realtime/               # 实时事件类型
│       ├── config/                 # 环境配置
│       ├── logging/                # slog 与 Gin 中间件
│       └── tests/                  # 集中的后端测试
├── frontend/                       # React/Vite/Mantine SPA 与 nginx
├── deepagent/                      # Python/uv DeepAgent 项目
├── docs/                           # 当前架构、需求和历史设计
├── openspec/                       # 规格和变更工件
└── scripts/                        # Docker 启动辅助脚本
```

## 核心架构原则

### 1. 组合根明确

`backend/cmd/server/main.go` 是唯一基础设施装配入口。MySQL、模型解析器、Runtime、Room Manager、Runner 和 API Server 都在此构造和注入。

### 2. 持久化事实与实时状态分离

MySQL 保存可恢复业务事实；当前进程保存 WebSocket 连接、Hub、活跃房间缓存和 Focus 状态。后端重启后可从 MySQL 恢复房间，但不会恢复连接、Hub 或 Focus Timeline。

### 3. 全局 Agent 与房间 Agent 分离

`agents` 是全局可编辑配置；创建房间时，Agent 的 Prompt、Runtime 和顺序被复制到 `room_agents`。如果 Agent 已显式绑定 Profile，或当时存在对应 Runtime 的数据库默认 Profile，还会复制具体 Profile ID；否则该字段保持为空。之后修改全局 Agent 只影响新房间。

### 4. 有具体 Profile 快照时选择固定，连接内容动态

房间快照中非空的 Profile ID 固定模型选择，而 Profile 的 Base URL、模型名和 API Key 在每次调用时重新读取。因此切换默认值不改变已有具体 Profile 快照，但更新同一 Profile 的连接内容会在旧房间下一次调用时生效。没有 Profile ID 的旧房间会在每次调用时解析当前数据库默认值，并在默认值不存在时使用环境迁移兜底。

### 5. Agent Runtime 可替换

Runner 通过 Runtime Registry 将 `llm` Agent 交给 Go LLM Runtime，将 `deepagent` Agent 交给 Python 子进程适配器。两者共享后端的 Model Resolver 与审计链路。

### 6. `/api/*` 是规范协议面

后端仍注册无 `/api` 前缀的兼容路由，但新开发只应面向 `/api/*`。前端使用相对 `/api`，开发环境由 Vite 代理，容器环境由 nginx 代理。

## 当前范围与扩展边界

当前架构主要面向单实例、可信内网或单组织部署：

- WebSocket Hub 和房间实时广播完全位于单个 Go 进程；
- 没有 Redis Pub/Sub、跨实例房间所有权或分布式任务队列；
- 没有正式用户、租户和角色体系；管理员和房间访问仍是共享 bearer secret；
- 知识检索是 Markdown 文本解析和分片，不是向量检索；
- Agent 响应由进程内有界队列处理；
- DeepAgent 子进程不是安全沙箱，必须按高权限可信代码看待。

如果要支持多实例或公网多租户，优先重新设计实时事件总线、任务执行、身份授权、配额限流和 DeepAgent 隔离，而不是直接复制 backend 实例。

## 按开发任务导航

- 新增 HTTP/WS 用例：先读 [后端架构](backend.md) 的 API 与应用端口部分，再读 [数据、实时与会议生命周期](data-realtime-lifecycle.md)。
- 修改 Agent 行为、Prompt 或对话策略：读 [Agent Runtime 与模型](agent-runtime-and-models.md)。
- 修改模型 Profile、密钥或 Provider 连接：读 [Agent Runtime 与模型](agent-runtime-and-models.md) 的解析与安全不变量。
- 修改表、GORM 模型或 Repository：读 [数据、实时与会议生命周期](data-realtime-lifecycle.md)，并同步根 README 和相关运维文档。
- 修改房间 UI、管理台或 API Client：读 [前端与部署](frontend-and-deployment.md)。
- 修改 Docker 或环境变量：读 [前端与部署](frontend-and-deployment.md)，以 `.env.example` 和根 README 为完整配置清单。

## 相关文档

- [`../ARCHITECTURE.md`](../ARCHITECTURE.md)：原集中式架构说明，仍包含 API 和配置摘要；本目录是新的分册入口。
- [`../agentroom-requirements.md`](../agentroom-requirements.md)：产品能力和业务范围。
- [`../data-persistence-design.md`](../data-persistence-design.md)：持久化设计背景和历史取舍。
- [`../../deepagent/README.md`](../../deepagent/README.md)：DeepAgent 独立开发、配置和运行方式。
- [`../../README.md`](../../README.md)：安装、启动、完整环境变量和运维说明。
- [`../../CLAUDE.md`](../../CLAUDE.md) 与 [`../../AGENTS.md`](../../AGENTS.md)：代理开发、测试和提交约定。
