# AgentRoom Backend

The backend is the Go service behind AgentRoom. It exposes the HTTP API, WebSocket room transport, MySQL persistence, Markdown knowledge upload, focus extraction, and OpenAI-compatible agent execution.

Rooms support two dialogue modes:

- `mention_fanout`: only directly mentioned agents reply, once each.
- `guided_dialogue`: a bounded multi-turn exchange where mentioned agents reply first and may hand off to other agents through explicit mentions.

Rooms also persist three lifecycle states:

- `active`: live meeting, ordinary users can join WebSocket and speak.
- `closed`: read-only meeting history, ordinary users can still open the room link to view messages and minutes.
- `archived`: admin-only history, no ordinary reads or live participation.

The current live human owner is the only participant who can close a meeting or transfer ownership to another online human. When the last human leaves, the backend starts a 30-second auto-close grace window.

The LLM integration layer is implemented with `langchaingo` and still uses the existing `LLM_*` environment variables, so OpenAI-compatible providers continue to work without changing the runtime configuration contract.

## Run Locally

From the repository root, create `.env` first:

```powershell
Copy-Item .env.example .env
```

Set `MYSQL_DSN` to a reachable MySQL 8 database:

```text
MYSQL_DSN=agentroom:agentroom_password@tcp(127.0.0.1:3306)/agentroom?parseTime=true&charset=utf8mb4&loc=UTC
DB_AUTO_MIGRATE=true
```

Then start the service:

```powershell
go -C backend run ./cmd/server
```

The default listen address is `http://localhost:8080`.

## Configuration

The server loads `../.env` when started from `backend/`, then reads environment variables. Existing process environment values take precedence over `.env`.

| Name | Required | Default | Description |
| --- | --- | --- | --- |
| `PORT` | No | `8080` | HTTP listen port. |
| `DB_DRIVER` | No | `mysql` | Database driver label. |
| `MYSQL_DSN` | Yes | _none_ | MySQL DSN. Include `parseTime=true`; use `utf8mb4` for multilingual room content. |
| `DB_AUTO_MIGRATE` | No | `false` | Runs embedded migrations on startup when `true`. |
| `LLM_BASE_URL` | No | `https://api.openai.com` | Base URL for the `langchaingo` OpenAI-compatible chat API client. |
| `LLM_API_KEY` | No | _empty_ | API key for agent responses. If empty, human chat still works and agent calls return room-visible system messages. |
| `LLM_MODEL` | No | `gpt-4o-mini` | Chat-completions model name. |
| `LOG_LEVEL` | No | `info` | `debug`, `info`, `warn`, or `error`. |
| `LOG_FORMAT` | No | `text` | Use `json` for container logs. |
| `LOG_ADD_SOURCE` | No | `false` | Include source file information in logs when `true`. |
| `ADMIN_API_KEY` | No | _empty_ | When set, protects agent and knowledge write APIs. |
| `ALLOWED_ORIGINS` | No | _empty_ | Comma-separated origins allowed to open WebSocket connections. Empty means allow all. |

## Docker

The root `docker-compose.yml` builds `backend/Dockerfile`, starts MySQL, waits for database health, injects a container-network `MYSQL_DSN`, and forwards the v0.2 security env vars (`ADMIN_API_KEY` and `ALLOWED_ORIGINS`) into the backend container.

```powershell
docker compose up --build
```

## API Surface

Primary routes are exposed under `/api`:

- `GET /api/health`
- `GET /api/agents`
- `POST /api/agents`
- `PUT /api/agents/:agentID`
- `DELETE /api/agents/:agentID`
- `GET /api/agents/:agentID/knowledge`
- `POST /api/agents/:agentID/knowledge`
- `POST /api/rooms`
- `GET /api/rooms` (admin)
- `GET /api/rooms/:roomID`
- `POST /api/rooms/:roomID/archive` (admin)
- `POST /api/rooms/:roomID/reopen` (admin)
- `POST /api/rooms/:roomID/restore` (admin)
- `GET /api/rooms/:roomID/messages`
- `GET /api/rooms/:roomID/activity`
- `POST /api/rooms/:roomID/minutes`
- `PUT /api/rooms/:roomID/minutes` (admin)
- `GET /api/rooms/:roomID/minutes/history` (admin)
- `GET /api/rooms/:roomID/minutes.md`
- `GET /api/rooms/:roomID/knowledge`
- `POST /api/rooms/:roomID/knowledge`
- `DELETE /api/knowledge/:documentID`
- `GET /api/rooms/:roomID/ws?name=Alice`

Legacy non-`/api` routes are still registered for compatibility.

`GET /api/rooms/:roomID/activity` returns recent `agentRuns` and `dialogueRuns` for the room. The endpoint uses the same room passcode checks as metadata and history reads, and accepts an optional `limit` query parameter capped by the API layer.

`GET /api/rooms/:roomID/messages` is cursor-paginated. It accepts `limit` plus an optional `before` cursor and returns `{ messages, hasMore, nextBefore }`.

`GET /api/rooms/:roomID/minutes.md` is now a pure read endpoint. It downloads the latest persisted minutes and returns `404` when no saved minutes exist.

Room lifecycle semantics:

- Closed rooms allow ordinary `GET /api/rooms/:roomID`, `GET /messages`, and `GET /minutes.md`.
- Closed rooms reject ordinary WebSocket joins and ordinary `POST /minutes`.
- Archived rooms reject ordinary room reads entirely and remain available only through admin-gated endpoints.
- `POST /api/rooms/:roomID/reopen` moves `closed -> active`.
- `POST /api/rooms/:roomID/restore` moves `archived -> closed`.

Live-room WebSocket control events now include:

- `{"type":"close_room"}`
- `{"type":"transfer_owner","participantID":"participant_123"}`

`POST /api/rooms` accepts an optional `dialoguePolicy` object. For example:

```json
{
  "name": "Planning",
  "agentIds": ["pm", "architect"],
  "dialoguePolicy": {
    "mode": "guided_dialogue"
  }
}
```

## Persistence

The backend persists:

- Agent definitions and prompts.
- Room metadata and per-room agent snapshots.
- Participants and message history.
- Agent run records.
- Dialogue run records plus per-message dialogue metadata (`dialogueRunID`, `turnIndex`, `parentMessageID`).
- Markdown knowledge documents and chunks.

Schema migrations live under `internal/store/mysql/migrations/` and run at startup when `DB_AUTO_MIGRATE=true`.

## Verification

```powershell
go -C backend test ./...
```
