# AgentRoom MVP

AgentRoom is a real-time text meeting room where humans collaborate with predefined role-based AI agents. Agents stay silent unless a human explicitly mentions them with strings like `@产品经理` or `@前端工程师`.

The project has two parts:

- `backend/`: Go service that owns rooms, participants, WebSocket fanout, agent triggering, and LLM calls
- `frontend/`: React + Vite client for creating or joining a room and chatting in real time

For the implementation handoff and product scope, see [ARCHITECTURE_GO_MVP.md](./ARCHITECTURE_GO_MVP.md).

## Prerequisites

- Go 1.22+
- Node.js 18+
- npm 9+

## Repository layout

```text
agentRoom_test/
├── backend/
├── frontend/
└── ARCHITECTURE_GO_MVP.md
```

## Backend environment

The backend reads these environment variables:

| Name | Required | Default | Description |
| --- | --- | --- | --- |
| `PORT` | No | `8080` | HTTP server port |
| `LLM_BASE_URL` | No | `https://api.openai.com` | OpenAI-compatible API base URL |
| `LLM_API_KEY` | No | _empty_ | API key for agent responses |
| `LLM_MODEL` | No | `gpt-4o-mini` | Chat completion model |

If `LLM_API_KEY` is not set, the backend still starts. Mentioned agents will fail gracefully and emit visible system messages in the room instead of crashing the process.

## Quick start

### 1. Start the backend

From the repository root:

```bash
go -C backend run ./cmd/server
```

The backend listens on `http://localhost:8080` by default.

### 2. Install frontend dependencies

```bash
npm --prefix frontend install
```

### 3. Start the frontend dev server

```bash
npm --prefix frontend run dev
```

The Vite dev server runs on `http://localhost:5173` and proxies `/health`, `/agents`, and `/rooms` to the backend.

### 4. Open the app

Open the Vite URL in your browser, enter a display name, then either:

- leave the room ID blank to create a new room, or
- enter an existing room ID to join one

## Typical meeting flow

1. Send a normal message like `我们先做一个文字会议室`.
2. Mention a specific agent, for example `@前端工程师 这个页面怎么布局？`.
3. Mention multiple agents in one message, for example `@产品经理 @测试工程师 第一版验收标准怎么定？`.
4. Ask the secretary to summarize with `@会议秘书 总结一下目前结论`.

## Development commands

### Backend

```bash
go -C backend test ./...
go -C backend mod tidy
```

### Frontend

```bash
npm --prefix frontend run build
npm --prefix frontend run preview
```

## Manual verification checklist

1. Start backend and frontend.
2. Open two browser windows.
3. Create a room in one window and join it from the other with a different display name.
4. Send a normal message and confirm no agent responds.
5. Send `@前端工程师 这个页面怎么布局？` and confirm only that agent responds.
6. Send `@产品经理 @测试工程师 第一版验收标准怎么定？` and confirm both respond.
7. Send `@会议秘书 总结一下目前结论` and confirm the structured summary appears.
8. Close one window and confirm the participant list updates.
9. If the LLM is unavailable, confirm backend-generated system messages are visible in the chat.

## HTTP surface

The backend exposes these MVP endpoints:

- `GET /health`
- `GET /agents`
- `POST /rooms`
- `GET /rooms/:roomID`
- `GET /rooms/:roomID/messages`
- `GET /rooms/:roomID/ws?name=Alice`

See [`backend/README.md`](./backend/README.md) for backend-specific details.