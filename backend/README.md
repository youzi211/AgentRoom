# AgentRoom Backend

This service provides the AgentRoom MVP backend: room management, participant tracking, WebSocket chat, explicit agent mention triggering, and OpenAI-compatible LLM integration.

## Prerequisites

- Go 1.22+

## Configuration

The server reads configuration from environment variables.

| Name | Required | Default | Description |
| --- | --- | --- | --- |
| `PORT` | No | `8080` | HTTP listen port |
| `LLM_BASE_URL` | No | `https://api.openai.com` | Base URL for the chat completion API |
| `LLM_API_KEY` | No | _empty_ | API key used for agent responses |
| `LLM_MODEL` | No | `gpt-4o-mini` | Model name sent to `/v1/chat/completions` |

If `LLM_API_KEY` is missing, the service still starts. Human chat continues to work, and agent invocations produce system messages describing the failure.

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

### `GET /health`

Returns:

```json
{
  "ok": true
}
```

### `GET /agents`

Returns the predefined agent roster.

### `POST /rooms`

Creates a room.

Request:

```json
{
  "name": "Demo Room"
}
```

### `GET /rooms/:roomID`

Returns room metadata, participants, and agents.

### `GET /rooms/:roomID/messages`

Returns recent room messages.

### `GET /rooms/:roomID/ws?name=Alice`

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
- `internal/room`: in-memory room state, clients, and broadcast hub