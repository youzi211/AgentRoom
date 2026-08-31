# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What This Is

AgentRoom is a real-time AI meeting workspace: humans create a room, invite role-based agents via `@mentions`, attach Markdown knowledge, and the conversation (human + agent turns, focus timeline, minutes) is persisted in MySQL. Go backend + React/Vite frontend + Python Agent Runtime.

## Commands

All commands assume the repository root.

```powershell
# Backend
go -C backend run ./cmd/server        # start the API/WebSocket server (loads ../.env)
go -C backend test ./...              # run all Go tests
go -C backend vet ./...               # static checks
go -C backend build ./cmd/server      # verify it builds

# Frontend
npm --prefix frontend install
npm --prefix frontend run dev         # Vite dev server on :5173, proxies /api -> :8080
npm --prefix frontend run build       # production build
node --test frontend/src/**/*.test.mjs  # frontend tests

# Python Runtime
uv --directory deepagent run pytest   # run all Python tests
uv --directory deepagent run agent-runtime  # start gRPC runtime service

# Full stack
docker compose up --build             # mysql + backend + frontend + agent-runtime
docker compose config                 # validate compose
```

Local dev requires a reachable MySQL 8 and `MYSQL_DSN` (with `parseTime=true`) plus `DB_AUTO_MIGRATE=true` in `.env`. Copy `.env.example` to `.env` first. Start the local MySQL80 Windows service if needed.

## Architecture Principles — 六条铁律

These rules govern all Go backend decisions. Violating them requires explicit justification.

### ① 职责清晰

每个包只有一个变化的理由。包名即职责：

| 包 | 职责 | 不该做的事 |
|---|---|---|
| `api` | HTTP/WS 协议适配、路由、请求校验 | 业务逻辑、直接访问 Store |
| `service` | 用例协调、房间生命周期、Agent 调度 | HTTP 细节、gRPC 传输 |
| `room` | 内存房间状态、WebSocket Hub | 持久化、模型调用 |
| `agent` | Agent 编排、对话策略、Prompt 组装 | 房间状态管理 |
| `collaboration` | 多 Agent 协作运行时（gRPC 合约、协调器、事件验证、快照、turn 治理） | 业务消息持久化、WebSocket 广播 |
| `store` | 持久化接口 + MySQL 实现 | 业务规则、HTTP |
| `model` | 共享领域类型 | 任何逻辑 |
| `llm` | OpenAI-compatible 模型客户端 | 对话策略 |
| `config` | 环境配置加载 | 业务逻辑 |
| `logging` | slog 设置 | 业务逻辑 |
| `realtime` | 实时事件类型定义 | 事件处理逻辑 |

### ② 依赖单向

依赖只能从外向内流动，不可逆行：

```
cmd/server (组合根)
  ↓
api → service → room / agent / collaboration → store → model
                                        ↓
                              llm / config / logging / realtime
```

- `model` 不 import 任何其他内部包。
- `store` 不 import `service` 或 `api`。
- `api` 不直接访问 `store`，只通过 `service` 暴露的接口。
- `collaboration` 不 import `api` 或 `service`；`service` 通过接口调用 `collaboration`。
- Python Runtime 不访问 AgentRoom MySQL，只接收 Go 提供的不可变快照。

### ③ 核心业务不依赖具体技术

- `service` 和 `room` 通过接口（`roomStore`、`collaborationExecutor`、`AgentRuntime`）依赖 Store 和 Runtime，不直接依赖 GORM、gRPC 或 langchaingo。
- `collaboration` 包的 `types.go`/`events.go`/`runtime.go` 只依赖 `context` 和 `time`，不 import gRPC、protobuf 或任何框架。
- 模型调用通过 `llm.Client` 接口，不散落在业务代码中。
- Protobuf 生成代码隔离在 `collaboration/proto/v1` 子包，不污染业务类型。

### ④ 接口小而精

- 每个接口只声明调用方真正需要的方法。`roomStore` 不暴露全量 Store 方法，只暴露 service 用到的那些。
- 不要为一个调用方创建包含 20 个方法的"上帝接口"。
- 接口定义在消费方包中（Go 惯例），不放在实现方。例如 `service` 包定义 `collaborationExecutor`，`collaboration` 包不需要知道它的存在。

### ⑤ 并发、资源、错误都有边界

- 每个并发 goroutine 必须有明确的 cancel 传播路径和超时。
- 容量限制（`MaxConcurrent`、`MaxPending`）必须有界，溢出时返回稳定错误而非创建无界任务。
- 错误必须分类（`ErrCapacity`、`ErrDuplicateRun`、`ErrProtocol`），调用方可以 `errors.Is` 判断。
- 资源（gRPC 连接、MySQL 连接、临时工作目录）必须有 `Close()`/`cleanup()` 并在 `defer` 中调用。
- 协作运行中取消旧 run 时，已提交的消息不可回滚，未提交的候选内容不可进入聊天历史。

### ⑥ 架构服务于变化，而不是炫技

- 不为"将来可能需要"提前创建抽象。单次使用的代码不需要接口。
- 不引入新依赖解决能用标准库解决的问题。
- 重构的目的是让某个真实变化更容易，不是为了"更干净"。
- 合并包是因为 8 个 1-3 文件的包确实造成了导航负担和 import 冗余；不是因为"包应该大"。

## 代码实现准则

- **不要重复造轮子**：标准库能做的不自己写，已有工具函数不重新实现。
- **不要过度设计**：YAGNI（You Aren't Gonna Need It）。不为假想需求建抽象层。
- **改一行能解决的就不改五行**：每行改动都应能追溯到需求。
- **测试是行为契约**：测试验证公开行为，不验证实现细节。改实现不改测试，除非行为本身变了。

## Architecture Overview

Start with [`docs/architecture/README.md`](docs/architecture/README.md) for the full map. The fastest way in is `backend/cmd/server/main.go` — it wires every component and shows the dependency direction:

```
Browser → api.Server → service.RoomService → room.Manager / agent.Runner / collaboration.Coordinator
                                              ↓                                    ↓
                                         store.Store (MySQL)              Python Agent Runtime (gRPC)
                                         llm.Client
```

### Backend packages (`backend/internal/`)

- **`api`** — Gin HTTP + Gorilla WebSocket. Routes under `/api/*`. Admin write routes gated by `X-Admin-Key`. Handlers depend only on `service` interfaces.
- **`service`** — use-case hub. `room_service.go` wires reads/writes; `collaboration_scheduler.go` bridges room messages to the remote Collaboration Runtime; `meeting_lifecycle.go` handles close/archive/reopen.
- **`room`** — in-memory room state, WebSocket hub, participant/message buffers.
- **`agent`** — single-agent orchestration: mention detection, dialogue policies (`mention_fanout` / `guided_dialogue`), prompt composition, LLM/deepagent runtime dispatch.
- **`collaboration`** — multi-agent collaboration runtime: gRPC contract, coordinator (per-room serial runs, global capacity), event validator, lifecycle audit, snapshot builder, turn handler. Proto generated code in `collaboration/proto/v1`.
- **`store`** — `Store` interface + `store/mysql` GORM implementation. Embedded SQL migrations under `store/mysql/migrations/`.
- **`llm`** — OpenAI-compatible client on `langchaingo`.
- **`model`** / **`config`** / **`logging`** / **`realtime`** — shared types, env config, slog, event type definitions.
- **`agentproto`** — generated protobuf for single-agent runtime.

### Collaboration Runtime

When `COLLABORATION_RUNTIME_MODE=remote`, Go delegates multi-agent collaboration to the Python Runtime:

```
Human message persisted & broadcast
  → CollaborationScheduler creates collaboration run
  → CollaborationCoordinator.Execute (per-room serial, global capacity)
  → gRPC ExecuteConversation stream → Python Native/AutoGen Engine
  ← Events: speaker_selected, agent_turn_started, model_started, agent_message_completed, terminal
  → Go validates events, commits Agent messages in idempotent transactions, broadcasts to WebSocket
```

Go owns: room access, message persistence, run audit, WebSocket, cancellation, deadline.
Python owns: speaker selection, Agent turn execution, engine state, checkpoint.

### Python Runtime (`deepagent/`)

- **`agent_runtime/`** — single-agent gRPC service: `LLMExecutor`, `DeepAgentExecutor`, `FakeExecutor`.
- **`collaboration_runtime/`** — collaboration engine registry, `NativeCollaborationEngine`, `AutoGenCollaborationEngine`, model gateway, capacity limiter.
- **`agentroom_deepagent/`** — DeepAgent research agent (Tavily search, report generation).

### Frontend (`frontend/src/`)

React + Vite + Mantine. `components/` holds UI, `api/roomClient.js` is the backend client, `collaborationRuntime.js` maps collaboration events to UI state, `routing.js` is shared routing.

## Conventions

- Go: `gofmt`-clean; exported `PascalCase`, internal `camelCase`. Prefer small package-local changes.
- Frontend: 2-space indent, single quotes, no semicolons. Components `PascalCase.jsx`, utilities `camelCase`.
- Tests consolidated under `backend/internal/tests/**` mirroring the package. Frontend tests `*.test.mjs` next to what they cover.
- When changing schema/migrations/env/config, update `README.md` and relevant `docs/`.
- Commit messages in Chinese, Conventional Commits format: `<type>: <中文描述>`.
- Branch prefix `codex/`, e.g. `codex/feat/user-registration`. Do not develop on `main`.
- Do not commit `.mcp.json` or `.env`.
