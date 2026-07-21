# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What This Is

AgentRoom is a real-time AI meeting workspace: humans create a room, invite role-based agents via `@mentions`, attach Markdown knowledge, and the conversation (human + agent turns, focus timeline, minutes) is persisted in MySQL. Go backend + React/Vite frontend.

## Commands

All commands assume the repository root. The backend is driven with `go -C backend`; the frontend with `npm --prefix frontend`.

```powershell
# Backend
go -C backend run ./cmd/server        # start the API/WebSocket server (loads ../.env)
go -C backend test ./...              # run all Go tests
go -C backend vet ./...               # static checks
go -C backend build ./cmd/server      # verify it builds

# Run a single Go test
go -C backend test ./internal/tests/agent -run TestName

# Frontend
npm --prefix frontend install
npm --prefix frontend run dev         # Vite dev server on :5173, proxies /api -> :8080
npm --prefix frontend run build       # production build (also the frontend CI gate)

# Frontend tests use Node's built-in runner (no test script in package.json)
node --test frontend/src/components/meetingMinutes.test.mjs

# Full stack
docker compose up --build             # mysql + backend(:8080) + frontend nginx(:5173)
docker compose config                 # validate compose
```

Local dev requires a reachable MySQL 8 and `MYSQL_DSN` (with `parseTime=true`) plus `DB_AUTO_MIGRATE=true` in `.env`. Copy `.env.example` to `.env` first. Without `LLM_API_KEY`, human chat still works but agent turns surface failures as room system messages.

## Architecture

Start with [`docs/architecture/README.md`](docs/architecture/README.md) for the current architecture map, then open the linked backend, runtime/model, data/realtime, or frontend/deployment guide for the area you are changing.

The backend is layered; understanding the wiring in `backend/cmd/server/main.go` is the fastest way in — it constructs every component and shows the dependency direction:

```
api.Server  ->  service.RoomService  ->  room.Manager / agent.Runner  ->  store.Store (MySQL)
                                          llm.Client (langchaingo)
```

- **`internal/api`** (`server.go`) — Gin HTTP + Gorilla WebSocket. Routes are registered **twice**: under `/api/*` (canonical) and under `""` (legacy compatibility). New work targets `/api/*`. Admin write routes are gated by `requireAdmin` (the `X-Admin-Key` header vs `ADMIN_API_KEY`). WebSocket origin is checked against `ALLOWED_ORIGINS`. Handlers depend only on `service.RoomService` — they do not touch the store directly.
- **`internal/service`** — use-case coordination (`room_service.go` is the hub; plus `agent_service`, `knowledge_service`, `focus_service`, `minutes_service`, `passcode`). Optional dependencies are attached fluently (e.g. `NewRoomService(...).WithMinutes(...)`).
- **`internal/room`** — live in-memory room state and the WebSocket hub (`manager.go`, `room.go`, `hub.go`). Holds participants, recent messages, and broadcasts.
- **`internal/agent`** — agent orchestration. `runner.go` turns a human (or agent) message into LLM-backed replies; `dialogue.go`, `mention.go` implement the two dialogue policies; `prompt_composer.go` + `prompt_context.go` build the prompt; `registry.go` holds `PredefinedAgents()` seeded at startup; `sanitize.go`'s `StripThinkBlocks` removes model `<think>` output before persisting.
- **`internal/store`** — `Store` interface + `store/mysql` implementation. Schema migrations are embedded SQL under `store/mysql/migrations/`, applied at startup when `DB_AUTO_MIGRATE=true`.
- **`internal/llm`** — OpenAI-compatible client built on `langchaingo`, configured from the `LLM_*` env vars. `NewClientFromEnv()`.
- **`internal/model`**, **`internal/config`**, **`internal/logging`** — shared types, env loading, and structured `slog` setup.

### Dialogue policies (the core agent behavior)

A room carries a `DialoguePolicy` (see `model/dialogue.go`, defaulted via `WithDefaults()`). `Runner.HandleHumanMessage` branches on it:

- **`mention_fanout`** — only directly `@mentioned` agents reply, once each.
- **`guided_dialogue`** — a bounded multi-turn exchange; mentioned agents reply first and may hand off to other agents via explicit mentions. Bounded by `MaxTurnsPerAgent`, `MaxAutonomousTurns`, `AllowAgentToAgentMentions`, and `AllowSelfFollowup`.

Agent runs are recorded in the store (`AgentRun` rows: running → succeeded/failed/timeout); guided dialogue also persists `dialogueRunID`, `turnIndex`, `parentMessageID` per message.

### Frontend

`frontend/src/` — React + Vite. `components/` holds the UI (`ChatRoom.jsx`, `AgentAdmin.jsx`, `JoinScreen.jsx`, focus/knowledge/minutes panels), `api/roomClient.js` is the backend client, `routing.js` is shared routing. The admin key is build-time `VITE_ADMIN_API_KEY` — an internal-deployment convenience, not real auth.

## Conventions

- Go: keep `gofmt`-clean; exported `PascalCase`, internal `camelCase`. Prefer small package-local changes over new abstractions or dependencies.
- Frontend: 2-space indent, single quotes, no semicolons unless required; components are `PascalCase.jsx`, utilities `camelCase`.
- **Tests are consolidated under `backend/internal/tests/**`** mirroring the package being tested (e.g. `tests/service/`, `tests/agent/`), not co-located with source. Favor black-box tests through exported APIs; shared in-memory doubles live in `tests/teststore/`. Frontend tests are `*.test.mjs` next to what they cover.
- When changing MySQL schema/migrations, env vars, or LLM config, update `README.md` and the relevant `docs/`.
- Commit subjects are short and intent-first (explain *why*, not just what moved).
- Commit messages must be written in Chinese.
- I must not develop directly on `main`; I must create a short-lived task branch and merge it only after review and verification.
- Branch names should start with the change type. When I create a branch, I must prefix it with `claude/`, for example `claude/feat/user-registration` or `claude/fix/payment-callback`.
- Commit messages must follow `<type>: <Chinese description>` and remain short and intent-first. Use `feat`, `fix`, `refactor`, `docs`, or `chore`, for example `feat: 添加用户注册` and `fix: 修复支付回调错误`.
