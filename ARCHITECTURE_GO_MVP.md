# AgentRoom Go MVP Architecture

This document is the implementation handoff for rebuilding AgentRoom from scratch with a Go backend and a React frontend.

## Product Goal

AgentRoom is a real-time text meeting room where humans and predefined role-based AI agents collaborate in the same conversation.

The first version must prove one core experience:

> Humans chat normally. Agents stay silent by default. When a human explicitly mentions an agent with `@AgentName`, only that agent responds using its role prompt and the recent meeting context.

This is not a video meeting product, not a generic chatbot, and not a full enterprise collaboration suite. The MVP should feel like a focused meeting workspace with role agents available on demand.

## MVP Scope

Build the smallest usable version that supports:

- One or more text meeting rooms.
- Human participants joining by display name.
- Real-time chat over WebSocket.
- A fixed set of predefined agents.
- Explicit `@AgentName` triggering.
- Agent replies generated through an OpenAI-compatible chat completion API.
- A special meeting secretary agent that can summarize the current meeting.
- A clean React UI with a join screen, message list, participant list, agent roster, and message composer.

Do not implement these in the MVP:

- Login or user accounts.
- Permissions or organizations.
- Video or voice.
- File upload.
- Local knowledge base or RAG.
- SQLite/Postgres persistence.
- Multi-tenant administration.
- Agent proactive interruption.
- CrewAI, AutoGen, LangChain, or another agent framework.

The MVP should use in-memory state. Process restart may lose rooms and messages.

## Recommended Stack

Backend:

- Go 1.22+
- `github.com/gin-gonic/gin` for HTTP API routing and middleware
- `github.com/gorilla/websocket` for WebSocket
- Standard `net/http`, `encoding/json`, `context`, `sync`, `time`

Frontend:

- React + Vite
- Plain CSS or CSS modules
- No heavy UI framework for the MVP

LLM:

- OpenAI-compatible `/v1/chat/completions` HTTP API
- Configurable by environment variables:
  - `LLM_BASE_URL`
  - `LLM_API_KEY`
  - `LLM_MODEL`

## Repository Layout

Use this structure:

```text
agentroom/
+-- backend/
|   +-- cmd/
|   |   +-- server/
|   |       +-- main.go
|   +-- internal/
|   |   +-- api/
|   |   |   +-- handlers.go
|   |   |   +-- router.go
|   |   |   +-- websocket.go
|   |   +-- agent/
|   |   |   +-- registry.go
|   |   |   +-- runner.go
|   |   |   +-- trigger.go
|   |   +-- llm/
|   |   |   +-- client.go
|   |   +-- model/
|   |   |   +-- types.go
|   |   +-- room/
|   |       +-- manager.go
|   |       +-- room.go
|   |       +-- hub.go
|   +-- go.mod
|   +-- README.md
|
+-- frontend/
|   +-- index.html
|   +-- package.json
|   +-- vite.config.js
|   +-- src/
|       +-- App.jsx
|       +-- main.jsx
|       +-- api/
|       |   +-- client.js
|       +-- hooks/
|       |   +-- useRoomSocket.js
|       +-- components/
|           +-- JoinScreen.jsx
|           +-- ChatRoom.jsx
|           +-- MessageList.jsx
|           +-- MessageComposer.jsx
|           +-- AgentRoster.jsx
|           +-- ParticipantList.jsx
|
+-- ARCHITECTURE_GO_MVP.md
+-- README.md
```

## Backend Architecture

The Go backend owns room state, WebSocket connections, message broadcast, agent triggering, and LLM calls.

Use Gin for the HTTP surface so the implementation is familiar, compact, and easy to hand to another coding agent. Use `gorilla/websocket` for the WebSocket upgrade inside the Gin handler.

### Core Modules

`internal/model`

- Defines shared types: `Room`, `Participant`, `Agent`, `Message`, API requests, API responses, and WebSocket events.

`internal/room`

- Owns in-memory room state.
- Tracks participants, connected WebSocket clients, messages, and active agents.
- Broadcasts events to all clients in a room.
- Protects mutable state with `sync.RWMutex`.

`internal/agent`

- Provides the predefined agent registry.
- Detects explicit mentions such as `@产品经理` or `@frontend`.
- Builds prompts for each agent.
- Calls the LLM client and returns an agent message.

`internal/llm`

- Implements a minimal OpenAI-compatible chat completion client.
- Takes system prompt, recent context messages, and user message.
- Returns plain text content.

`internal/api`

- Provides Gin router setup, HTTP handlers, and WebSocket endpoint.
- Translates HTTP/WS payloads into room and agent operations.

## Data Model

Use IDs as strings. UUIDs are fine, but simple generated IDs are acceptable for the MVP.

```go
type Room struct {
    ID           string
    Name         string
    Participants map[string]*Participant
    Agents       map[string]*Agent
    Messages     []Message
    CreatedAt    time.Time
}

type Participant struct {
    ID       string
    Name     string
    JoinedAt time.Time
}

type Agent struct {
    ID           string
    Name         string
    Mention      string
    Role         string
    Description  string
    SystemPrompt string
}

type Message struct {
    ID         string
    RoomID     string
    SenderID   string
    SenderName string
    SenderType string // "human", "agent", or "system"
    Content    string
    CreatedAt  time.Time
}
```

## Predefined Agents

Create these agents in `internal/agent/registry.go`.

### 产品经理

Mention: `@产品经理`

Role:

- Clarify requirements.
- Control MVP scope.
- Identify user value and product tradeoffs.
- Avoid overbuilding.

### 前端工程师

Mention: `@前端工程师`

Role:

- Propose UI layout.
- Discuss interaction patterns.
- Point out frontend state and usability issues.
- Keep the interface practical and simple.

### 后端工程师

Mention: `@后端工程师`

Role:

- Discuss APIs, data models, room state, WebSocket behavior, concurrency, and failure modes.
- Keep implementation boundaries clear.

### 测试工程师

Mention: `@测试工程师`

Role:

- Convert discussion into acceptance criteria.
- Identify edge cases and risks.
- Suggest manual and automated tests.

### 会议秘书

Mention: `@会议秘书`

Role:

- Summarize the meeting.
- Extract decisions, action items, risks, and open questions.
- Keep output structured and concise.

## Agent Trigger Rules

MVP trigger behavior must be conservative:

- Agents never respond to normal messages.
- Agents respond only when the message contains their exact `Mention` string.
- If one message mentions multiple agents, each mentioned agent may respond.
- Agent messages should not trigger other agents.
- System messages should not trigger agents.
- If LLM generation fails, broadcast a system message or agent error message instead of crashing the room.

Examples:

```text
我们先做一个文字会议室
=> no agent response

@前端工程师 这个页面第一版怎么布局？
=> only 前端工程师 responds

@产品经理 @测试工程师 第一版验收标准怎么定？
=> 产品经理 responds, then 测试工程师 responds

@会议秘书 总结一下目前结论
=> 会议秘书 responds with structured summary
```

## Prompt Design

Each agent response should include:

- The agent's role system prompt.
- The latest room context, limited to recent messages.
- The user message that triggered the agent.

Keep context bounded. Use the latest 30 messages for the MVP.

Generic agent system prompt pattern:

```text
你是 AgentRoom 中的一个职能型 AI Agent。
你的角色是：{role}

行为规则：
- 你只在被明确 @ 提及时发言。
- 你要基于当前会议上下文回答。
- 你要保持角色边界，不要假装自己是其他角色。
- 回答要适合会议协作，简洁、具体、可执行。
- 如果问题超出你的角色范围，请指出并给出有限建议。
```

Meeting secretary prompt pattern:

```text
你是 AgentRoom 的会议秘书。
请基于当前会议上下文输出结构化会议记录。

输出格式：
1. 已达成结论
2. 待办事项
3. 风险
4. 未决问题

要求：
- 不要编造没有出现的信息。
- 待办事项要尽量包含负责人；如果没有负责人，写“待定”。
- 保持简洁。
```

## HTTP API

Use JSON for all HTTP responses.

### `GET /health`

Returns:

```json
{
  "ok": true
}
```

### `GET /agents`

Returns all predefined agents.

```json
{
  "agents": [
    {
      "id": "pm",
      "name": "产品经理",
      "mention": "@产品经理",
      "role": "Product Manager",
      "description": "澄清需求、控制范围、判断用户价值"
    }
  ]
}
```

Do not expose full system prompts to the frontend unless needed.

### `POST /rooms`

Request:

```json
{
  "name": "Demo Room"
}
```

Response:

```json
{
  "room": {
    "id": "room_abc",
    "name": "Demo Room"
  }
}
```

### `GET /rooms/{roomID}`

Returns room metadata, participants, and agents.

### `GET /rooms/{roomID}/messages`

Returns recent messages.

## WebSocket API

Endpoint:

```text
GET /rooms/{roomID}/ws?name=Alice
```

When the connection opens:

- Create or attach a participant.
- Broadcast a `participant_joined` event.
- Send current room snapshot to the newly connected client.

Client-to-server message:

```json
{
  "type": "message",
  "content": "@前端工程师 这个页面怎么布局？"
}
```

Server-to-client events:

```json
{
  "type": "message",
  "message": {
    "id": "msg_123",
    "roomID": "room_abc",
    "senderID": "p_123",
    "senderName": "Alice",
    "senderType": "human",
    "content": "@前端工程师 这个页面怎么布局？",
    "createdAt": "2026-06-11T10:00:00Z"
  }
}
```

```json
{
  "type": "room_snapshot",
  "room": {},
  "participants": [],
  "agents": [],
  "messages": []
}
```

```json
{
  "type": "participant_joined",
  "participant": {}
}
```

```json
{
  "type": "participant_left",
  "participantID": "p_123"
}
```

## Message Flow

```text
1. Browser sends a chat message over WebSocket.
2. Backend validates non-empty content.
3. Backend creates a human Message.
4. Room stores the message.
5. Room broadcasts the message to all connected clients.
6. Agent trigger scans the human message.
7. If no agent is mentioned, stop.
8. If agents are mentioned, generate one response per mentioned agent.
9. Each agent response is stored as a Message.
10. Each agent response is broadcast to all connected clients.
```

Agent generation may run asynchronously so the human message appears immediately. Preserve response order as much as practical.

## Frontend Architecture

The frontend should present the actual meeting room as the first experience after joining. Do not build a marketing landing page.

### Screens

`JoinScreen`

- Display app name.
- Ask for display name.
- Optional room name for creating a room.
- Button to create/join room.

`ChatRoom`

- Main meeting workspace.
- Left sidebar: participants and agents.
- Main area: messages.
- Bottom: message composer.

### Components

`ParticipantList`

- Shows currently connected humans.

`AgentRoster`

- Shows predefined agents and their mention names.
- Makes it easy for users to copy or insert mentions.

`MessageList`

- Renders human, agent, and system messages differently.
- Agent messages should visibly show the agent role.

`MessageComposer`

- Text input.
- Send button.
- Enter to send.
- Shift+Enter for newline if multiline input is used.

## Frontend UX Requirements

The MVP should feel like a work tool:

- Dense but readable layout.
- No decorative landing page.
- No oversized hero section.
- No nested cards.
- Keep sidebars and message area stable.
- Make agent mention names visible.
- Use clear empty states.

Recommended layout:

```text
+--------------------------------------------------+
| AgentRoom                         Room: Demo     |
+------------------+-------------------------------+
| Participants     | Message List                  |
| - Alice          |                               |
| - Bob            | Alice: 我们先讨论第一版       |
|                  | Bob: @产品经理 范围怎么收？   |
| Agents           | 产品经理: 建议先做...         |
| - @产品经理      |                               |
| - @前端工程师    |                               |
| - @后端工程师    |                               |
| - @测试工程师    |                               |
| - @会议秘书      |                               |
+------------------+-------------------------------+
|                  | [输入消息...] [发送]          |
+------------------+-------------------------------+
```

## Environment Configuration

Backend environment variables:

```text
PORT=8080
LLM_BASE_URL=https://api.openai.com
LLM_API_KEY=...
LLM_MODEL=gpt-4o-mini
```

If `LLM_API_KEY` is missing, the backend should still start. Agent calls should return a clear error message in the room instead of crashing.

## Error Handling

Implement these behaviors:

- Invalid room ID: return `404`.
- Empty message: ignore or send a validation error event.
- WebSocket read error: disconnect participant and broadcast leave event.
- LLM error: create a system message such as `Agent 前端工程师 failed to respond: <short reason>`.
- Panic prevention: one bad WebSocket or LLM call must not bring down the process.

## Testing Strategy

Add lightweight backend tests where practical:

- Agent mention detection.
- Multiple mention detection.
- No trigger for normal messages.
- No trigger for agent messages.
- Room stores and returns messages in order.

Manual verification is required for the MVP:

1. Start backend.
2. Start frontend.
3. Open two browser windows.
4. Join with two different names.
5. Send a normal message and confirm no agent responds.
6. Send `@前端工程师 这个页面怎么布局？` and confirm only that agent responds.
7. Send `@产品经理 @测试工程师 第一版验收标准怎么定？` and confirm both respond.
8. Send `@会议秘书 总结一下目前结论` and confirm structured summary appears.
9. Disconnect one window and confirm participant list updates.

## Implementation Order

1. Create Go backend skeleton and health endpoint.
2. Add room manager and in-memory room model.
3. Add predefined agent registry.
4. Add HTTP endpoints for agents, rooms, and messages.
5. Add WebSocket join, message receive, and broadcast.
6. Add mention detection.
7. Add OpenAI-compatible LLM client.
8. Add agent runner and broadcast agent replies.
9. Create React frontend skeleton.
10. Add join screen.
11. Add WebSocket hook.
12. Add chat room layout.
13. Add participants, agents, message list, and composer.
14. Run backend tests.
15. Run frontend build.
16. Perform manual two-window verification.

## Acceptance Criteria

The MVP is done when:

- `GET /health` works.
- The frontend can create or join a room.
- Two browser windows can join the same room as different participants.
- Human messages appear in real time for all connected clients.
- Predefined agents are visible in the UI.
- Normal messages do not trigger agents.
- `@AgentName` triggers only the matching agent.
- `@会议秘书` can produce a structured meeting summary.
- LLM errors are visible but do not crash the backend.
- Backend tests for mention detection pass.
- Frontend production build passes.

## Design Principles

- Keep the first version small.
- Prefer explicit behavior over clever automation.
- Agents are meeting participants, not background magic.
- Default silence is a product feature.
- A working meeting loop matters more than advanced agent orchestration.
- Avoid framework lock-in until the core interaction is proven.
