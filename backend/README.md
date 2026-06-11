# AgentRoom Backend

This service provides the AgentRoom MVP backend: room management, participant tracking, WebSocket chat, explicit agent mention triggering, in-memory agent configuration, and OpenAI-compatible LLM integration.

## Prerequisites

- Go 1.22+

## Configuration

The server loads the project-root `.env` file first, then reads environment variables. Existing shell environment variables take precedence over `.env` values.

From the repository root:

```bash
copy .env.example .env
```

Then fill in:

| Name | Required | Default | Description |
| --- | --- | --- | --- |
| `PORT` | No | `8080` | HTTP listen port |
| `LLM_BASE_URL` | No | `https://api.openai.com` | Base URL for the chat completion API |
| `LLM_API_KEY` | No | _empty_ | API key used for agent responses |
| `LLM_MODEL` | No | `gpt-4o-mini` | Model name sent to `/v1/chat/completions` |

If `LLM_API_KEY` is missing, the service still starts. Human chat continues to work, and agent invocations produce system messages describing the failure.

Do not commit `.env`; it is ignored by git.

## Run the server

From the repository root:

```bash
go -C backend run ./cmd/server
```

Or from inside `backend/`:

```bash
go run ./cmd/server
```

## Test and dependency maintenance

```bash
go -C backend test ./...
go -C backend mod tidy
```

## API endpoints

API routes are exposed under `/api` for frontend and production deployments.
Legacy non-`/api` routes are still registered for compatibility.

### `GET /api/health`

Returns:

```json
{
  "ok": true
}
```

### `GET /api/agents`

Returns the configured agent roster, including editable system prompts for the management page.

### `PUT /api/agents/:agentID`

Updates one predefined agent in memory.

Request:

```json
{
  "name": "产品负责人",
  "role": "Product Lead",
  "description": "负责收敛范围和确认优先级。",
  "systemPrompt": "你是产品负责人。",
  "enabled": true
}
```

When the display name changes, the mention string changes with it, for example `产品负责人` becomes `@产品负责人`.

### `POST /api/rooms`

Creates a room using the currently enabled agents.

Request:

```json
{
  "name": "Demo Room"
}
```

### `GET /api/rooms/:roomID`

Returns room metadata, participants, and enabled agents. Room responses hide system prompts.

### `GET /api/rooms/:roomID/messages`

Returns recent room messages.

### `GET /api/rooms/:roomID/ws?name=Alice`

Upgrades to WebSocket, joins the participant to the room, sends a room snapshot, and then streams room events.

## WebSocket events

Client to server:

```json
{
  "type": "message",
  "content": "@前端工程师 这个页面怎么布局？"
}
```

Server to client event types:

- `room_snapshot`
- `message`
- `participant_joined`
- `participant_left`
- `error`

## Agent behavior

- Agents only respond when their exact mention string appears in a human message.
- Normal human messages do not trigger agents.
- One message can trigger multiple agents.
- Agent and system messages do not trigger follow-up agents.
- Disabled agents are excluded from room agent rosters and mention detection.
- LLM failures are surfaced as room-visible system messages.

Default predefined agents:

- `@产品经理`
- `@前端工程师`
- `@后端工程师`
- `@测试工程师`
- `@会议秘书`

## Package overview

- `cmd/server`: process entrypoint
- `internal/api`: Gin routes, HTTP handlers, and WebSocket handling
- `internal/agent`: predefined agents, mention detection, and response generation
- `internal/llm`: minimal OpenAI-compatible client
- `internal/model`: shared API and room event types
- `internal/room`: in-memory room state, clients, agent configuration, and broadcast hub
