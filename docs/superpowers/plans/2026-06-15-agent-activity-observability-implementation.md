# Agent Activity Observability Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a room-level Agent activity timeline so users and developers can see which Agent/dialogue run started, finished, failed, or stopped, and why.

**Architecture:** Reuse the existing `agent_runs`, `dialogue_runs`, and per-message dialogue metadata as the source of truth. Add read APIs for historical room activity, emit lightweight WebSocket activity events from the runner, and render a compact activity panel in `ChatRoom` without changing the core dialogue scheduling policy.

**Tech Stack:** Go, Gin, GORM/MySQL, existing `room.Hub` WebSocket events, React 18, Vite, Node built-in test runner.

---

## Requirements Summary

- Show recent Agent execution and guided dialogue activity for a room.
- Preserve current room passcode access rules for activity reads.
- Keep activity data derived from existing `agent_runs`, `dialogue_runs`, and generated message metadata.
- Emit realtime activity events so the UI can show "Agent is thinking" and "dialogue stopped because X" without waiting for a page reload.
- Avoid new dependencies and avoid changing LLM/provider contracts.
- Keep `mention_fanout` and `guided_dialogue` behavior unchanged.

## Acceptance Criteria

- `GET /api/rooms/:roomID/activity` returns recent agent runs and dialogue runs for an accessible room.
- Passcode-protected rooms reject activity requests without the correct passcode.
- Agent run activity includes `id`, `roomID`, `agentID`, `agentName`, `triggerMessageID`, `status`, `errorText`, `createdAt`, and `completedAt`.
- Dialogue run activity includes `id`, `roomID`, `triggerMessageID`, `mode`, `turnCount`, `status`, `createdAt`, and `completedAt`.
- WebSocket clients receive activity events when an agent run starts/finishes and when a dialogue run starts/finishes.
- `ChatRoom` displays a compact "Agent 活动" panel with active/running items and recent completed items.
- Existing message, focus, minutes, knowledge, and dialogue tests continue to pass.
- Verification commands pass:
  - `go -C backend test ./...`
  - `go -C backend vet ./...`
  - `go -C backend build ./cmd/server`
  - `node --test frontend/src/api/roomClient.test.mjs`
  - `node --test frontend/src/components/agentActivity.test.mjs`
  - `npm --prefix frontend run build`

## File Structure

- Modify: `backend/internal/model/types.go`
  - Add API response/event DTOs and `EventTypeAgentActivity`.
- Modify: `backend/internal/store/store.go`
  - Add list query types and store interface methods for agent/dialogue runs.
- Modify: `backend/internal/store/mysql/store.go`
  - Implement MySQL list queries ordered by newest run first.
- Modify: `backend/internal/store/mysql/models.go`
  - Add conversion helpers from GORM run models back to store/domain run structs if needed.
- Modify: `backend/internal/tests/teststore/store.go`
  - Store created runs in memory and support list methods for service/API tests.
- Modify: `backend/internal/service/room_service.go`
  - Add `ListRoomActivity(ctx, currentRoom, limit)`.
- Modify: `backend/internal/api/server.go`
  - Register `GET /api/rooms/:roomID/activity`.
  - Enforce room passcode consistently with room metadata/messages.
- Modify: `backend/internal/agent/runner.go`
  - Emit agent run start/finish activity events.
  - Extend `RuntimeRoom` with an event broadcast method, or introduce a narrow optional activity notifier.
- Modify: `backend/internal/agent/dialogue.go`
  - Emit dialogue run start/finish activity events.
- Modify: `backend/internal/room/room.go`
  - Add a room method for broadcasting non-message events, if `RuntimeRoom` is extended.
- Test: `backend/internal/tests/api/server_activity_test.go`
  - Cover HTTP activity endpoint, passcode behavior, and response shape.
- Test: `backend/internal/tests/agent/activity_events_test.go`
  - Cover realtime event emission from agent/dialogue runs using runtime-room test doubles.
- Modify: `frontend/src/api/roomClient.js`
  - Add `getRoomActivity(roomId, passcode)`.
- Modify: `frontend/src/api/roomClient.test.mjs`
  - Cover activity request URL/passcode header through fetch stubbing or extracted helpers.
- Create: `frontend/src/components/AgentActivityPanel.jsx`
  - Render activity timeline and active/running indicators.
- Create: `frontend/src/components/agentActivity.js`
  - Pure helpers for labels, sorting, merging realtime events, and status descriptions.
- Create: `frontend/src/components/agentActivity.test.mjs`
  - Unit-test pure helpers without a browser test runner.
- Modify: `frontend/src/components/ChatRoom.jsx`
  - Load initial activity, process WebSocket activity events, and render the panel.
- Modify: `frontend/src/chat-room.css`
  - Add compact panel styles.
- Modify: `README.md`, `backend/README.md`, `docs/ARCHITECTURE.md`
  - Document the activity endpoint and UI behavior.

---

## Chunk 1: Backend Activity Read Model

### Task 1: Lock the activity API contract with failing tests

**Files:**
- Create: `backend/internal/tests/api/server_activity_test.go`
- Modify: `backend/internal/tests/teststore/store.go`

- [ ] **Step 1: Add a failing API test for open room activity**

Create a test that:
- creates a room through `POST /api/rooms`
- inserts representative agent/dialogue runs through the test store helper fields
- calls `GET /api/rooms/:roomID/activity`
- expects HTTP 200 and both `agentRuns` and `dialogueRuns` arrays.

Expected response shape:

```json
{
  "agentRuns": [
    {
      "id": "run_1",
      "roomID": "room_1",
      "agentID": "pm",
      "agentName": "PM",
      "triggerMessageID": "msg_1",
      "status": "succeeded",
      "errorText": "",
      "createdAt": "2026-06-15T00:00:00Z",
      "completedAt": "2026-06-15T00:00:01Z"
    }
  ],
  "dialogueRuns": [
    {
      "id": "dialogue_1",
      "roomID": "room_1",
      "triggerMessageID": "msg_1",
      "mode": "guided_dialogue",
      "turnCount": 2,
      "status": "stopped_limit",
      "createdAt": "2026-06-15T00:00:00Z",
      "completedAt": "2026-06-15T00:00:02Z"
    }
  ]
}
```

- [ ] **Step 2: Add a failing passcode test**

Create a passcode-protected room and assert:
- missing passcode returns 403
- wrong passcode returns 403
- correct `?passcode=` or `X-Room-Passcode` returns 200

Follow the existing passcode patterns in `backend/internal/tests/api/server_security_test.go`.

- [ ] **Step 3: Run the targeted API tests and verify RED**

Run:

```powershell
go -C backend test ./internal/tests/api -run "TestRoomActivity"
```

Expected: FAIL because the route and store methods do not exist yet.

### Task 2: Add activity DTOs and store queries

**Files:**
- Modify: `backend/internal/model/types.go`
- Modify: `backend/internal/store/store.go`
- Modify: `backend/internal/store/mysql/models.go`
- Modify: `backend/internal/store/mysql/store.go`
- Modify: `backend/internal/tests/teststore/store.go`

- [ ] **Step 1: Add model response structs**

In `backend/internal/model/types.go`, add:

```go
const EventTypeAgentActivity = "agent_activity"

type RoomActivityResponse struct {
	AgentRuns    []AgentRunActivity    `json:"agentRuns"`
	DialogueRuns []DialogueRunActivity `json:"dialogueRuns"`
}

type AgentRunActivity struct {
	ID               string     `json:"id"`
	RoomID           string     `json:"roomID"`
	AgentID          string     `json:"agentID"`
	AgentName        string     `json:"agentName"`
	TriggerMessageID string     `json:"triggerMessageID"`
	Status           string     `json:"status"`
	ErrorText        string     `json:"errorText,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
	CompletedAt      *time.Time `json:"completedAt,omitempty"`
}

type DialogueRunActivity struct {
	ID               string     `json:"id"`
	RoomID           string     `json:"roomID"`
	TriggerMessageID string     `json:"triggerMessageID"`
	Mode             string     `json:"mode"`
	TurnCount        int        `json:"turnCount"`
	Status           string     `json:"status"`
	CreatedAt        time.Time  `json:"createdAt"`
	CompletedAt      *time.Time `json:"completedAt,omitempty"`
}

type AgentActivityEvent struct {
	Kind             string     `json:"kind"`
	Phase            string     `json:"phase"`
	ID               string     `json:"id"`
	RoomID           string     `json:"roomID"`
	AgentID          string     `json:"agentID,omitempty"`
	AgentName        string     `json:"agentName,omitempty"`
	TriggerMessageID string     `json:"triggerMessageID,omitempty"`
	Status           string     `json:"status,omitempty"`
	ErrorText        string     `json:"errorText,omitempty"`
	TurnCount        int        `json:"turnCount,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
	CompletedAt      *time.Time `json:"completedAt,omitempty"`
}
```

Extend `ServerEvent` with:

```go
Activity *AgentActivityEvent `json:"activity,omitempty"`
```

- [ ] **Step 2: Add store query methods**

In `backend/internal/store/store.go`, add:

```go
ListAgentRuns(ctx context.Context, query ListRunsQuery) ([]AgentRun, error)
ListDialogueRuns(ctx context.Context, query ListRunsQuery) ([]DialogueRun, error)

type ListRunsQuery struct {
	RoomID string
	Limit  int
}
```

- [ ] **Step 3: Implement MySQL queries**

In `backend/internal/store/mysql/store.go`, query by `room_id`, order `created_at DESC`, and default to a conservative limit, for example 50.

Implementation shape:

```go
func (s *MySQLStore) ListAgentRuns(ctx context.Context, query store.ListRunsQuery) ([]store.AgentRun, error) {
	limit := query.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	var models []AgentRunModel
	err := s.db.WithContext(ctx).
		Where("room_id = ?", query.RoomID).
		Order("created_at DESC").
		Limit(limit).
		Find(&models).Error
	if err != nil {
		return nil, fmt.Errorf("list agent runs: %w", err)
	}
	runs := make([]store.AgentRun, 0, len(models))
	for _, model := range models {
		runs = append(runs, model.toStore())
	}
	return runs, nil
}
```

Mirror this for dialogue runs.

- [ ] **Step 4: Update test store**

Keep in-memory slices:

```go
AgentRuns    []store.AgentRun
DialogueRuns []store.DialogueRun
```

`Create*Run` appends. `Finish*Run` updates matching records. `List*Runs` filters by room ID, newest first.

- [ ] **Step 5: Run focused backend tests**

Run:

```powershell
go -C backend test ./internal/tests/api -run "TestRoomActivity"
go -C backend test ./internal/tests/model ./internal/tests/service
```

Expected: API tests may still fail until the service and route exist; model/service tests should compile.

### Task 3: Add RoomService and API route

**Files:**
- Modify: `backend/internal/service/room_service.go`
- Modify: `backend/internal/api/server.go`
- Modify: `backend/README.md`
- Modify: `docs/ARCHITECTURE.md`

- [ ] **Step 1: Add `ListRoomActivity` to `RoomService`**

Use `currentRoom.Info().ID`, call both store list methods, map store records into `model.RoomActivityResponse`.

- [ ] **Step 2: Register route**

In `backend/internal/api/server.go`, add near the other room routes:

```go
routes.GET("/rooms/:roomID/activity", s.handleGetRoomActivity)
```

- [ ] **Step 3: Implement handler**

Handler behavior:
- load room via existing room lookup path
- enforce `s.rooms.CanAccessRoom(currentRoom, roomPasscodeFromRequest(c))`
- parse optional `limit`, cap at 100
- return `model.RoomActivityResponse`

- [ ] **Step 4: Update docs**

Document:

```text
GET /api/rooms/:roomID/activity
```

Mention passcode behavior and the two returned arrays.

- [ ] **Step 5: Verify GREEN**

Run:

```powershell
go -C backend test ./internal/tests/api -run "TestRoomActivity"
go -C backend test ./internal/tests/api -run "TestCreateRoomPersistsDialoguePolicy|TestRoomActivity"
```

Expected: PASS.

---

## Chunk 2: Realtime Activity Events

### Task 4: Add runner event broadcast boundary

**Files:**
- Modify: `backend/internal/agent/runner.go`
- Modify: `backend/internal/agent/dialogue.go`
- Modify: `backend/internal/room/room.go`
- Modify: `backend/internal/tests/agent/dialogue_phase2_test.go`
- Modify: `backend/internal/tests/agent/runner_knowledge_test.go`
- Create: `backend/internal/tests/agent/activity_events_test.go`

- [ ] **Step 1: Add failing event tests**

Create tests that use a runtime room double to capture server events:
- mention fanout emits `agent_activity` started and finished around one agent response
- provider failure emits finished event with failed/timeout status
- guided dialogue emits dialogue started and dialogue finished with final stop status

Run:

```powershell
go -C backend test ./internal/tests/agent -run "TestAgentActivity"
```

Expected: FAIL because no event broadcast exists yet.

- [ ] **Step 2: Extend runtime room event boundary**

Preferred small change:

```go
type RuntimeRoom interface {
	...
	Broadcast(message model.Message)
	BroadcastEvent(event model.ServerEvent)
}
```

In `backend/internal/room/room.go`:

```go
func (r *Room) BroadcastEvent(event model.ServerEvent) {
	r.Hub().Broadcast(event)
}
```

Update test doubles with no-op or capture implementations.

- [ ] **Step 3: Emit agent run events**

In `handleAgentResponse`:
- after `CreateAgentRun`, broadcast started event
- after `FinishAgentRun`, broadcast finished event
- include `runID`, room ID, agent ID/name, trigger message ID, status, error text, and timestamps

Keep this best-effort: event broadcast must not block or fail the run.

- [ ] **Step 4: Emit dialogue run events**

In `handleGuidedDialogue`:
- after `CreateDialogueRun`, broadcast started event
- after `FinishDialogueRun`, broadcast finished event with `turnCount` and final status

- [ ] **Step 5: Verify event tests**

Run:

```powershell
go -C backend test ./internal/tests/agent -run "TestAgentActivity|TestGuidedDialogue"
```

Expected: PASS.

### Task 5: Preserve event compatibility

**Files:**
- Modify: `frontend/src/components/ChatRoom.jsx`
- Modify: `backend/internal/tests/api/server_security_test.go`

- [ ] **Step 1: Confirm unknown events remain harmless**

Existing clients should ignore unknown event types. In `ChatRoom.jsx`, handle `agent_activity` explicitly and keep default behavior as no-op.

- [ ] **Step 2: Add or update API/WS regression if practical**

If existing WebSocket tests are not ergonomic, keep this covered by agent unit tests and frontend event reducer tests in Chunk 3.

- [ ] **Step 3: Run backend tests**

Run:

```powershell
go -C backend test ./internal/tests/agent ./internal/tests/api
```

Expected: PASS.

---

## Chunk 3: Frontend Activity Panel

### Task 6: Add frontend activity helpers and API client

**Files:**
- Modify: `frontend/src/api/roomClient.js`
- Modify: `frontend/src/api/roomClient.test.mjs`
- Create: `frontend/src/components/agentActivity.js`
- Create: `frontend/src/components/agentActivity.test.mjs`

- [ ] **Step 1: Add failing helper tests**

Cover:
- status label mapping for `running`, `succeeded`, `failed`, `timeout`, `stopped_limit`, `stopped_duplicate`, `stopped_empty`
- merge logic updates an existing run by ID instead of duplicating it
- sorting keeps running items first, then newest completed items

Run:

```powershell
node --test frontend/src/components/agentActivity.test.mjs
```

Expected: FAIL until helpers exist.

- [ ] **Step 2: Implement `agentActivity.js` helpers**

Export:

```js
export function labelForActivityStatus(status) {}
export function descriptionForActivity(activity) {}
export function mergeActivityEvent(current, eventActivity) {}
export function sortActivityItems(items) {}
export function normalizeActivityPayload(payload) {}
```

- [ ] **Step 3: Add `getRoomActivity`**

In `frontend/src/api/roomClient.js`:

```js
export async function getRoomActivity(roomId, passcode = '') {
  const encodedRoomId = encodeURIComponent(roomId)
  const response = await fetch(`${API_BASE_PATH}/rooms/${encodedRoomId}/activity`, {
    headers: withRoomPasscode({}, passcode),
  })
  return parseResponse(response)
}
```

- [ ] **Step 4: Test client/helper behavior**

Run:

```powershell
node --test frontend/src/api/roomClient.test.mjs
node --test frontend/src/components/agentActivity.test.mjs
```

Expected: PASS.

### Task 7: Render the Agent Activity panel

**Files:**
- Create: `frontend/src/components/AgentActivityPanel.jsx`
- Modify: `frontend/src/components/ChatRoom.jsx`
- Modify: `frontend/src/chat-room.css`
- Modify: `frontend/src/chat-room-layout.test.mjs`

- [ ] **Step 1: Add panel component**

`AgentActivityPanel` props:

```js
function AgentActivityPanel({ activities = [], isLoading = false, errorMessage = '' }) {}
```

Render:
- heading: `Agent 活动`
- loading state
- empty state: `暂无 Agent 活动`
- active/running rows
- recent completed rows

- [ ] **Step 2: Load historical activity in `ChatRoom`**

In `ChatRoom.jsx`, load `getRoomActivity(roomId, roomPasscode)` with room and messages. Store normalized activity items.

If activity load fails, show a compact panel notice and keep the room usable.

- [ ] **Step 3: Process WebSocket activity events**

When event type is `agent_activity`, merge `event.activity` into the activity list.

Do not append these events to the message list.

- [ ] **Step 4: Render in the sidebar**

Place the panel near `ParticipantList`, `MeetingMinutesPanel`, and `KnowledgePanel`, so users see activity alongside room context.

- [ ] **Step 5: Add CSS**

Add compact styles:
- `.agent-activity-panel`
- `.agent-activity-list`
- `.agent-activity-item`
- `.agent-activity-item--running`
- `.agent-activity-status`
- `.agent-activity-meta`

Keep card radius consistent with existing panels and avoid nested card styling.

- [ ] **Step 6: Add layout regression**

Update `frontend/src/chat-room-layout.test.mjs` to assert `AgentActivityPanel` is imported/rendered by `ChatRoom`.

- [ ] **Step 7: Verify frontend**

Run:

```powershell
node --test frontend/src/components/agentActivity.test.mjs
node --test frontend/src/chat-room-layout.test.mjs
npm --prefix frontend run build
```

Expected: PASS.

---

## Chunk 4: Full Verification And Documentation

### Task 8: Update documentation and run full verification

**Files:**
- Modify: `README.md`
- Modify: `backend/README.md`
- Modify: `docs/ARCHITECTURE.md`

- [ ] **Step 1: Document the endpoint**

Add `GET /api/rooms/:roomID/activity` to API lists.

- [ ] **Step 2: Document realtime activity**

In architecture docs, note:
- Agent runner emits activity events for run lifecycle.
- Historical activity remains queryable from persisted runs.
- Activity events are UI observability only; message persistence remains the source of room transcript truth.

- [ ] **Step 3: Run all backend verification**

Run:

```powershell
go -C backend test ./...
go -C backend vet ./...
go -C backend build ./cmd/server
```

Expected: PASS.

- [ ] **Step 4: Run all frontend verification**

Run:

```powershell
node --test frontend/src/api/roomClient.test.mjs
node --test frontend/src/components/agentActivity.test.mjs
node --test frontend/src/components/copyRegression.test.mjs
node --test frontend/src/components/meetingMinutes.test.mjs
node --test frontend/src/chat-room-layout.test.mjs
npm --prefix frontend run build
```

Expected: PASS.

- [ ] **Step 5: Manual smoke check**

Run the app and verify:
- create `mention_fanout` room, `@Agent`, see running then succeeded activity
- create `guided_dialogue` room, trigger multiple turns, see dialogue run status/turn count
- provider error produces failed activity and room-visible system message
- passcode room activity is not visible without passcode

- [ ] **Step 6: Commit**

Use a Chinese Lore-protocol commit message. Example intent line:

```text
让多 Agent 对话过程可追踪
```

Include `Tested:` trailers with the commands actually run.

---

## Risks And Mitigations

- **Risk: Activity events duplicate historical data.**
  - Mitigation: Treat WebSocket events as UI freshness hints. Persisted run tables remain the source of truth. Frontend merge by `kind + id`.

- **Risk: Runner event broadcasting couples agent execution to WebSocket details.**
  - Mitigation: Broadcast through `RuntimeRoom.BroadcastEvent(model.ServerEvent)` only; do not import API/websocket packages into `agent`.

- **Risk: Activity endpoint leaks passcode-protected room metadata.**
  - Mitigation: Reuse the same room lookup and passcode check pattern as room metadata/messages/minutes.

- **Risk: Failed store writes make activity incomplete.**
  - Mitigation: Preserve existing behavior: room-visible system messages still report agent failures. Activity panel can show partial data but must not break chat.

- **Risk: UI becomes noisy in guided dialogue.**
  - Mitigation: Panel shows active items plus a capped recent list; message timeline remains focused on actual room-visible messages.

## Execution Notes

- Do not add a database migration unless the existing `agent_runs` and `dialogue_runs` columns cannot support the API response. Current schema appears sufficient.
- Do not add polling in the first pass. Use one initial fetch plus WebSocket events.
- Do not expose prompt text, hidden reasoning, provider payloads, or API keys through activity responses.
- Keep all activity labels in Chinese on the frontend.
- Keep changes small and commit only after full verification.
