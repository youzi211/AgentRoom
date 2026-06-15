# AgentRoom Backend

The backend is the Go service behind AgentRoom. It exposes the HTTP API, WebSocket room transport, MySQL persistence, Markdown knowledge upload, focus extraction, and OpenAI-compatible agent execution.

Rooms support two dialogue modes:

- `mention_fanout`: only directly mentioned agents reply, once each.
- `guided_dialogue`: a bounded multi-turn exchange where mentioned agents reply first and may hand off to other agents through explicit mentions.

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
- `GET /api/rooms/:roomID`
- `GET /api/rooms/:roomID/messages`
- `POST /api/rooms/:roomID/minutes`
- `GET /api/rooms/:roomID/minutes.md`
- `GET /api/rooms/:roomID/knowledge`
- `POST /api/rooms/:roomID/knowledge`
- `DELETE /api/knowledge/:documentID`
- `GET /api/rooms/:roomID/ws?name=Alice`

Legacy non-`/api` routes are still registered for compatibility.

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
