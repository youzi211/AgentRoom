# AgentRoom v0.2 Deployment Preview

AgentRoom is a real-time AI meeting workspace. Humans create a room, invite role-based agents with explicit `@mentions`, attach Markdown knowledge, track meeting focus, and preserve the meeting history in MySQL.

This repository contains:

- `backend/`: Go HTTP/WebSocket service, MySQL persistence, agent orchestration, knowledge upload, focus extraction, and OpenAI-compatible LLM calls.
- `frontend/`: React + Vite client for room entry, live chat, focus timeline, knowledge panels, and agent administration.

The v0.2 target is an online-ready internal deployment: persistent MySQL storage, container startup, explicit admin protection, WebSocket origin restrictions, optional room passcodes, and meeting minutes export. This README documents the deployment path and calls out the runtime knobs that are already present in the current backend.

## Current Status

Implemented in the current tree:

- MySQL-backed agents, rooms, participants, messages, agent runs, and Markdown knowledge metadata/chunks.
- Automatic schema migration when `DB_AUTO_MIGRATE=true`.
- OpenAI-compatible agent responses through `LLM_BASE_URL`, `LLM_API_KEY`, and `LLM_MODEL`.
- Room and agent Markdown knowledge upload APIs.
- WebSocket live room updates under `/api/rooms/:roomID/ws`.
- Agent activity history under `/api/rooms/:roomID/activity` plus live `agent_activity` WebSocket events.
- Admin key protection for agent and knowledge write APIs when `ADMIN_API_KEY` is configured.
- WebSocket origin allowlist through `ALLOWED_ORIGINS`.
- Optional room passcodes for room metadata, message history, minutes, and WebSocket join.
- Meeting minutes generation, persistence with versioning, admin editing, and Markdown export.
- Admin console (gated by `ADMIN_API_KEY`) with meeting/room management (list, archive, restore) and Agent configuration.
- Health endpoint with database status at `/api/health`.

Recommended for deployment:

- Keep `ADMIN_API_KEY` and `ALLOWED_ORIGINS` set in non-local environments.
- The admin console at `/admin` prompts for the admin key and stores it in the browser's `localStorage` (sent as `X-Admin-Key`). `VITE_ADMIN_API_KEY` remains an optional build-time fallback for internal deployments.
- Treat the admin key as an internal deployment convenience, not a substitute for a real user auth system.

## Prerequisites

For local development:

- Go 1.22+
- Node.js 18+
- npm 9+
- MySQL 8+

For container deployment:

- Docker
- Docker Compose v2

## Quick Start With Docker Compose

Create a local environment file:

```powershell
Copy-Item .env.example .env
```

Edit `.env` and set at least:

- `LLM_API_KEY`
- `MYSQL_ROOT_PASSWORD`
- `MYSQL_PASSWORD`

The shipped `.env.example` already includes aligned local defaults for:

- `ADMIN_API_KEY`
- `VITE_ADMIN_API_KEY`
- `ALLOWED_ORIGINS=http://localhost:5173,http://127.0.0.1:5173`

Start the stack:

```powershell
docker compose up --build
```

Open:

- Frontend: `http://localhost:5173`
- Backend health: `http://localhost:8080/api/health`

Compose starts three services:

- `mysql`: MySQL 8 database with a persistent Docker volume.
- `backend`: Go API server on port `8080`.
- `frontend`: nginx serving the built Vite app and proxying `/api` plus WebSocket traffic to the backend.

The backend container receives a container-network DSN:

```text
agentroom:${MYSQL_PASSWORD}@tcp(mysql:3306)/agentroom?parseTime=true&charset=utf8mb4&loc=UTC
```

Keep the root `.env` file out of git. It is already ignored.

## Local Development

If you prefer to run services directly, start a MySQL database first and set `MYSQL_DSN` in `.env`:

```text
MYSQL_DSN=agentroom:agentroom_password@tcp(127.0.0.1:3306)/agentroom?parseTime=true&charset=utf8mb4&loc=UTC
DB_AUTO_MIGRATE=true
```

Run the backend:

```powershell
go -C backend run ./cmd/server
```

Run the frontend dev server:

```powershell
npm --prefix frontend install
npm --prefix frontend run dev
```

The Vite dev server listens on `http://localhost:5173` and proxies `/api` to `http://127.0.0.1:8080` by default. Override with `VITE_API_PROXY_TARGET` if needed.

## Environment Variables

The backend loads `../.env` when it starts from `backend/`, then reads process environment variables. Values already present in the process environment take precedence over `.env`.

| Name | Required | Default | Description |
| --- | --- | --- | --- |
| `PORT` | No | `8080` | Backend HTTP port. |
| `DB_DRIVER` | No | `mysql` | Database driver label. Current backend uses MySQL. |
| `MYSQL_DSN` | Yes | _none_ | MySQL DSN. Must include `parseTime=true`; `charset=utf8mb4` is recommended. |
| `DB_AUTO_MIGRATE` | No | `false` | Runs embedded schema migrations at startup when `true`. |
| `LLM_BASE_URL` | No | `https://api.openai.com` | OpenAI-compatible API base URL. |
| `LLM_API_KEY` | No | _empty_ | API key for agent responses. If empty, human chat still works and agent failures are shown as room system messages. |
| `LLM_MODEL` | No | `gpt-4o-mini` | Chat-completions model name. |
| `LOG_LEVEL` | No | `info` | `debug`, `info`, `warn`, or `error`. |
| `LOG_FORMAT` | No | `text` | Use `json` for production log collectors. |
| `LOG_ADD_SOURCE` | No | `false` | Include source file information in logs when `true`. |
| `ADMIN_API_KEY` | No | _empty_ | Protects agent and knowledge write APIs when set. |
| `ALLOWED_ORIGINS` | No | _empty_ | Comma-separated origins allowed to open WebSocket connections. Empty means allow all. |

Compose-only database bootstrap variables:

| Name | Description |
| --- | --- |
| `MYSQL_DATABASE` | Database created by the MySQL image. |
| `MYSQL_USER` | Application database user created by the MySQL image. |
| `MYSQL_PASSWORD` | Application database password. |
| `MYSQL_ROOT_PASSWORD` | MySQL root password. |
| `VITE_ADMIN_API_KEY` | Frontend build-time admin key used by the internal management UI. |

## v0.2 Security And Product Checks

Before exposing AgentRoom outside a trusted internal network, verify these items against the running build:

- Admin API writes require the configured `X-Admin-Key`: creating/updating/deleting agents and uploading/deleting knowledge should not be anonymous.
- WebSocket origin checks allow only the configured production frontend origins.
- Rooms that are created with a passcode require that passcode for metadata, history, meeting minutes, and WebSocket join paths (a valid admin key bypasses the passcode for admin operations).
- Meeting minutes can be generated from persisted room messages, are stored as versioned records, can be edited by an admin, and exported as Markdown.
- The admin console can list rooms, archive a room (making it read-only), and restore it.
- `/api/health` reports `"database": {"ok": true}`.

## HTTP Surface

Primary API routes are under `/api`:

- `GET /api/health`
- `GET /api/admin/verify` (admin) — validates `X-Admin-Key`, used by the admin console gate
- `GET /api/agents`
- `POST /api/agents`
- `PUT /api/agents/:agentID`
- `DELETE /api/agents/:agentID`
- `GET /api/agents/:agentID/knowledge`
- `POST /api/agents/:agentID/knowledge`
- `POST /api/rooms`
- `GET /api/rooms` (admin) — list rooms for the admin console (`?status=active|archived`, `?limit`, `?offset`)
- `GET /api/rooms/:roomID`
- `POST /api/rooms/:roomID/archive` (admin) — archive a room (becomes read-only)
- `POST /api/rooms/:roomID/restore` (admin) — restore an archived room
- `GET /api/rooms/:roomID/messages`
- `GET /api/rooms/:roomID/activity`
- `POST /api/rooms/:roomID/minutes` — generate and persist a new AI minutes version
- `PUT /api/rooms/:roomID/minutes` (admin) — save an edited minutes body as a new manual version
- `GET /api/rooms/:roomID/minutes/history` (admin) — list persisted minutes versions
- `GET /api/rooms/:roomID/minutes.md` — download the latest persisted minutes as Markdown
- `GET /api/rooms/:roomID/knowledge`
- `POST /api/rooms/:roomID/knowledge`
- `DELETE /api/knowledge/:documentID`
- `GET /api/rooms/:roomID/ws?name=Alice`

Routes marked **(admin)** require the `X-Admin-Key` header when `ADMIN_API_KEY` is configured. A valid admin key also bypasses a room's passcode for minutes/generation routes, so an operator can manage any room without knowing each passcode. Archived rooms reject new human messages over WebSocket and stop triggering agent turns.

Legacy non-`/api` backend routes are still registered for compatibility. New frontend and deployment traffic should use `/api/*`.

## Verification Commands

Backend:

```powershell
go -C backend test ./...
```

Frontend:

```powershell
npm --prefix frontend run build
```

Compose configuration:

```powershell
docker compose config
```

## Repository Layout

```text
agentRoom_test/
|-- backend/
|-- frontend/
|-- docs/
|-- docker-compose.yml
|-- .env.example
`-- README.md
```
