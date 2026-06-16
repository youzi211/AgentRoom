# Meeting Lifecycle And Read-Only History Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement a three-state meeting lifecycle with live owner controls, 30-second last-human auto-close, closed-room read-only history, and admin room-detail message inspection.

**Architecture:** Keep `RoomService` as the public facade, but move lifecycle policy into a new `meeting_lifecycle.go` coordinator that owns owner selection, persisted lifecycle transitions, grace-window timers, and live-room termination. Split API access by intent (`read`, `live`, `admin`) and add a frontend room gateway so `ChatRoom` stays live-only while `RoomReadOnly` handles closed-room history and `MeetingAdmin` gains a dedicated room detail/messages surface.

**Tech Stack:** Go, Gin, GORM/MySQL, existing `room.Hub` WebSocket transport, React 18, Vite, Node built-in test runner.

---

## Requirements Summary

- Preserve three persisted room states: `active`, `closed`, `archived`.
- Let the current online human owner close the room or transfer ownership only to another online human.
- Auto-close an `active` room 30 seconds after the last human leaves unless someone rejoins in time.
- Let ordinary users open `closed` rooms in read-only mode without a display name.
- Reject ordinary access to `archived` rooms while keeping admin read/manage access.
- Keep owner actions on the active-room WebSocket session instead of adding participant-authenticated HTTP routes.
- Make `GET /api/rooms/:roomID/messages` cursor-paginated and strict about invalid cursors.
- Make `GET /api/rooms/:roomID/minutes.md` a pure read endpoint with no generation side effect.
- Let admins inspect full room metadata, messages, and minutes from the meeting management UI.
- Avoid new dependencies and keep diffs small, test-first, and reversible.

## Acceptance Criteria

- `RoomMeta`, `RoomSummary`, runtime room state, and MySQL room rows all preserve `active`, `closed`, and `archived` distinctly.
- First successful live human join claims owner for an ownerless active room.
- Owner transfer succeeds only to another currently online human participant in the same active room.
- When the owner leaves and humans remain, ownership reassigns to the earliest joined online human.
- When the last human leaves, the room stores `auto_close_deadline_at = now + 30s`; rejoin cancels it; expiry closes the room with `closed_reason = last_human_left`.
- Owner manual close sets `closed_reason = manual`, clears owner/deadline, ejects connected clients, and closes live sockets.
- `POST /api/rooms/:roomID/reopen` is admin-only and valid only for `closed`.
- `POST /api/rooms/:roomID/restore` is admin-only and valid only for `archived`, always returning the room to `closed`.
- Ordinary `GET /api/rooms/:roomID`, `GET /messages`, and `GET /minutes.md` succeed for `closed` rooms and fail for `archived` rooms.
- Ordinary `POST /api/rooms/:roomID/minutes` fails for `closed` and `archived`; admins can still generate/save minutes in all states.
- `GET /api/rooms/:roomID/messages` returns `{ messages, hasMore, nextBefore }`, newest page by default, oldest-to-newest order per page, and `400` for malformed or wrong-room cursors.
- Closed-room frontend access never creates a WebSocket connection, never shows a composer, and never requires a display name.
- `MeetingAdmin` can filter `active`, `closed`, `archived`, `all` and open a room detail surface that shows overview, paginated messages, and minutes actions/history.
- Verification passes:
  - `go -C backend test ./...`
  - `go -C backend vet ./...`
  - `go -C backend build ./cmd/server`
  - `node --test frontend/src/api/roomClient.test.mjs`
  - `node --test frontend/src/components/roomAccess.test.mjs`
  - `node --test frontend/src/components/meetingAdminDetail.test.mjs`
  - `node --test frontend/src/chat-room-layout.test.mjs`
  - `npm --prefix frontend run build`

## File Structure

- Modify: `backend/internal/model/types.go`
  - Add `RoomStatusClosed`, close/owner/deadline metadata, lifecycle-specific event types, paginated messages response, and structured client-event payload support for `transfer_owner`.
- Modify: `backend/internal/store/store.go`
  - Replace the narrow `SetRoomStatus` contract with a lifecycle update input, add strict paginated message reads, and extend room-list filters to include `closed`.
- Modify: `backend/internal/store/mysql/models.go`
  - Extend `RoomModel` with owner/closed/deadline fields and keep domain conversion helpers aligned.
- Modify: `backend/internal/store/mysql/store.go`
  - Implement `UpdateRoomLifecycle`, strict message cursor validation, `closed` filtering, and lifecycle-aware room summaries.
- Create: `backend/internal/store/mysql/migrations/003_room_lifecycle.sql`
  - Document the schema additions even though runtime migration is driven by GORM AutoMigrate.
- Modify: `backend/internal/room/room.go`
  - Preserve `closed` status, store owner/closure/deadline fields in memory, add helpers for lifecycle snapshots, online-human ordering, and room termination cleanup.
- Modify: `backend/internal/room/hub.go`
  - Add a way to snapshot/drop active clients so close/archive can eject everyone deterministically.
- Modify: `backend/internal/room/manager.go`
  - Keep lazy room loading, but return rooms with full lifecycle metadata so service-side reconciliation can re-arm/consume grace timers.
- Create: `backend/internal/service/meeting_lifecycle.go`
  - Centralize owner assignment, transfer validation, grace timer scheduling/cancellation, manual close, reopen, archive, restore, and lazy-load reconciliation.
- Modify: `backend/internal/service/room_service.go`
  - Delegate lifecycle work to the coordinator, expose paginated message reads, keep `minutes.md` read-only, and enforce `active`-only ordinary writes.
- Modify: `backend/internal/api/server.go`
  - Split read/live/admin room resolution, add `/reopen`, tighten `/restore`, parse structured WebSocket control events, and return lifecycle-specific errors.
- Modify: `backend/internal/tests/teststore/store.go`
  - Mirror new lifecycle fields, cursor validation, paginated message reads, and deterministic lifecycle updates for tests.
- Create: `backend/internal/tests/service/meeting_lifecycle_test.go`
  - Lock owner assignment, transfer, last-human grace logic, manual close, reopen, and restore semantics.
- Create: `backend/internal/tests/api/server_room_lifecycle_test.go`
  - Cover read-vs-live-vs-admin access, minutes read/write gates, room filters, and message pagination/cursor errors.
- Create: `backend/internal/tests/api/server_room_ws_test.go`
  - Cover `close_room`, `transfer_owner`, closed-room WebSocket rejection, and live ejection on close/archive.
- Modify: `frontend/src/api/roomClient.js`
  - Add paginated room-history helpers, admin-capable read headers, `reopenRoom`, and structured room-control send helpers.
- Modify: `frontend/src/api/roomClient.test.mjs`
  - Cover paginated messages query construction, admin-capable read headers, and `reopenRoom`.
- Create: `frontend/src/components/roomAccess.js`
  - Hold pure routing decisions such as `active -> ChatRoom`, `closed -> RoomReadOnly`, `archived -> denial`, plus live-termination navigation rules.
- Create: `frontend/src/components/roomAccess.test.mjs`
  - Unit-test the gateway decision logic without needing a browser runner.
- Create: `frontend/src/components/RoomGateway.jsx`
  - Fetch room metadata, choose between entry/live/read-only/denial surfaces, and clear stale participant session on lifecycle changes.
- Create: `frontend/src/components/RoomReadOnly.jsx`
  - Render closed-room metadata, paginated history, latest minutes preview/download, and explicit read-only copy without a WebSocket or composer.
- Modify: `frontend/src/components/ChatRoom.jsx`
  - Handle new lifecycle events, expose owner controls, send structured WebSocket control events, and hand off to the gateway on close/archive.
- Create: `frontend/src/components/MeetingRoomDetail.jsx`
  - Show admin overview, paginated messages, and minutes actions/history for the selected room.
- Modify: `frontend/src/components/MeetingAdmin.jsx`
  - Add `closed` filter/state labels, open the detail surface, and wire `archive`, `reopen`, and `restore` correctly per status.
- Modify: `frontend/src/components/MinutesHistory.jsx`
  - Respect admin-only generation/editing for archived/closed rooms and reuse the new room detail flow.
- Modify: `frontend/src/App.jsx`
  - Route `/rooms/:roomID` through `RoomGateway` instead of deciding only on `participantName`.
- Modify: `frontend/src/routing.js`
  - Add a way to clear stored room session separately from keeping the room link, which is required when live users are ejected into read-only mode.
- Modify: `frontend/src/styles.css`, `frontend/src/chat-room.css`
  - Add styles for closed-room history and admin room detail.
- Create: `frontend/src/components/meetingAdminDetail.test.mjs`
  - Lock the admin surface wiring, `closed` filter, and message-history affordances with source/pure-helper tests.
- Modify: `README.md`, `backend/README.md`, `docs/data-persistence-design.md`
  - Update public behavior, route semantics, and schema docs.

---

## Chunk 1: Backend Lifecycle Data Model

### Task 1: Lock the new lifecycle contract with failing backend tests

**Files:**
- Create: `backend/internal/tests/service/meeting_lifecycle_test.go`
- Create: `backend/internal/tests/api/server_room_lifecycle_test.go`
- Modify: `backend/internal/tests/teststore/store.go`

- [ ] **Step 1: Add service tests for owner and close semantics**

Create tests that cover:
- first live join claims owner in an ownerless active room
- owner transfer rejects offline targets
- owner leave reassigns to earliest joined remaining human
- last human leave stores a 30-second deadline instead of closing immediately
- grace expiry closes with `closed_reason = last_human_left`
- manual close clears owner and deadline, then marks the room `closed`
- reopen clears close metadata and leaves owner empty
- restore/unarchive always ends at `closed`

Suggested test names:

```go
func TestMeetingLifecycleAssignsInitialOwnerOnFirstJoin(t *testing.T) {}
func TestMeetingLifecycleTransfersOwnerOnlyToOnlineParticipant(t *testing.T) {}
func TestMeetingLifecycleClosesAfterGraceWindowWhenLastHumanLeaves(t *testing.T) {}
func TestMeetingLifecycleManualCloseClearsOwnerAndDeadline(t *testing.T) {}
func TestMeetingLifecycleRestoreReturnsArchivedRoomToClosed(t *testing.T) {}
```

- [ ] **Step 2: Add failing API tests for access split and minutes behavior**

Cover:
- ordinary `GET /rooms/:roomID` and `GET /messages` succeed for `closed`
- ordinary `GET` for `archived` returns `403`
- ordinary `POST /minutes` on `closed` returns `403`
- admin `POST /minutes` on `closed` still returns `200`
- `GET /minutes.md` returns `404` when no persisted minutes exist instead of generating them
- `/rooms?status=closed` returns only closed rooms

- [ ] **Step 3: Add failing API tests for message pagination and cursor errors**

Cover:
- newest page by default
- oldest-to-newest ordering inside a page
- `hasMore` and `nextBefore`
- malformed cursor returns `400`
- cursor for a different room returns `400`

Expected response shape:

```json
{
  "messages": [
    {
      "id": "msg_100",
      "roomID": "room_1",
      "senderID": "participant_1",
      "senderName": "Alice",
      "senderType": "human",
      "content": "Let's close on Friday.",
      "createdAt": "2026-06-16T09:00:00Z"
    }
  ],
  "hasMore": true,
  "nextBefore": "msg_100"
}
```

- [ ] **Step 4: Run targeted tests to verify RED**

Run:

```powershell
go -C backend test ./internal/tests/service -run "TestMeetingLifecycle"
go -C backend test ./internal/tests/api -run "TestRoomLifecycle|TestRoomMessagesPagination"
```

Expected: FAIL because the lifecycle fields, access helpers, and pagination contract do not exist yet.

### Task 2: Extend the persisted room model and strict history reads

**Files:**
- Modify: `backend/internal/model/types.go`
- Modify: `backend/internal/store/store.go`
- Modify: `backend/internal/store/mysql/models.go`
- Modify: `backend/internal/store/mysql/store.go`
- Create: `backend/internal/store/mysql/migrations/003_room_lifecycle.sql`
- Modify: `backend/internal/tests/teststore/store.go`

- [ ] **Step 1: Extend room metadata and event constants**

In `backend/internal/model/types.go`, add:

```go
const (
	RoomStatusActive   = "active"
	RoomStatusClosed   = "closed"
	RoomStatusArchived = "archived"

	EventTypeRoomClosed   = "room_closed"
	EventTypeRoomArchived = "room_archived"
)

type RoomMeta struct {
	...
	Status              string     `json:"status"`
	OwnerParticipantID  string     `json:"ownerParticipantID,omitempty"`
	ClosedAt            *time.Time `json:"closedAt,omitempty"`
	ClosedReason        string     `json:"closedReason,omitempty"`
	AutoCloseDeadlineAt *time.Time `json:"autoCloseDeadlineAt,omitempty"`
	ArchivedAt          *time.Time `json:"archivedAt,omitempty"`
}
```

Add a paginated messages response and structured control payload support:

```go
type GetMessagesResponse struct {
	Messages   []Message `json:"messages"`
	HasMore    bool      `json:"hasMore"`
	NextBefore string    `json:"nextBefore,omitempty"`
}

type ClientEvent struct {
	Type          string `json:"type"`
	Content       string `json:"content,omitempty"`
	ParticipantID string `json:"participantID,omitempty"`
}
```

- [ ] **Step 2: Replace the narrow room-status store write**

In `backend/internal/store/store.go`, replace `SetRoomStatus` with a lifecycle update input:

```go
type UpdateRoomLifecycleInput struct {
	RoomID               string
	Status               string
	OwnerParticipantID   string
	ClosedAt             *time.Time
	ClosedReason         string
	AutoCloseDeadlineAt  *time.Time
	ArchivedAt           *time.Time
}

type MessagePage struct {
	Messages   []model.Message
	HasMore    bool
	NextBefore string
}

var ErrInvalidMessageCursor = errors.New("invalid message cursor")

UpdateRoomLifecycle(ctx context.Context, input UpdateRoomLifecycleInput) error
ListMessagesPage(ctx context.Context, query ListMessagesQuery) (MessagePage, error)
```

Keep the existing plain `ListMessages` method for internal callers like minutes generation and room bootstrap.

- [ ] **Step 3: Extend MySQL room columns and conversion helpers**

Update `RoomModel` to include:

```go
OwnerParticipantID  *string    `gorm:"column:owner_participant_id;size:64"`
ClosedAt            *time.Time `gorm:"column:closed_at"`
ClosedReason        string     `gorm:"column:closed_reason;size:32;not null;default:''"`
AutoCloseDeadlineAt *time.Time `gorm:"column:auto_close_deadline_at"`
```

Update `toDomain()` / `roomToModel()` so `RoomMeta` and `RoomSummary` round-trip the new fields.

- [ ] **Step 4: Implement strict paginated message reads**

In `backend/internal/store/mysql/store.go`, add `ListMessagesPage` with this shape:

```go
func (s *MySQLStore) ListMessagesPage(ctx context.Context, query store.ListMessagesQuery) (store.MessagePage, error) {
	limit := normalizedMessageLimit(query.Limit)
	base := s.db.WithContext(ctx).Where("room_id = ?", query.RoomID)

	if query.Before != "" {
		var cursor MessageModel
		err := s.db.WithContext(ctx).
			Select("id, room_id, created_at").
			Where("id = ?", query.Before).
			First(&cursor).Error
		if err != nil || cursor.RoomID != query.RoomID {
			return store.MessagePage{}, store.ErrInvalidMessageCursor
		}
		base = base.Where("(created_at, id) < (?, ?)", cursor.CreatedAt, cursor.ID)
	}

	var models []MessageModel
	if err := base.Order("created_at DESC, id DESC").Limit(limit + 1).Find(&models).Error; err != nil {
		return store.MessagePage{}, fmt.Errorf("list messages page: %w", err)
	}
	...
}
```

Reverse the selected slice before returning so each page stays oldest-to-newest.

- [ ] **Step 5: Document the schema change**

Add `backend/internal/store/mysql/migrations/003_room_lifecycle.sql` with:
- new room columns
- `idx_rooms_status_created` or equivalent status/index support if helpful
- comments that this file is a schema reference for AutoMigrate-backed runtime migration

- [ ] **Step 6: Update the in-memory test store**

Mirror the same lifecycle fields and cursor validation in `backend/internal/tests/teststore/store.go`.

Add helpers like:

```go
func (s *Store) UpdateRoomLifecycle(_ context.Context, input store.UpdateRoomLifecycleInput) error
func (s *Store) ListMessagesPage(_ context.Context, query store.ListMessagesQuery) (store.MessagePage, error)
```

- [ ] **Step 7: Run focused backend tests**

Run:

```powershell
go -C backend test ./internal/tests/service -run "TestMeetingLifecycle"
go -C backend test ./internal/tests/api -run "TestRoomLifecycle|TestRoomMessagesPagination"
```

Expected: still partly RED until lifecycle policy and handlers are wired, but model/store code should compile and strict cursor failures should now be reachable.

---

## Chunk 2: Runtime Lifecycle Coordinator

### Task 3: Introduce a focused lifecycle coordinator with fakeable timers

**Files:**
- Create: `backend/internal/service/meeting_lifecycle.go`
- Modify: `backend/internal/service/room_service.go`
- Modify: `backend/internal/room/room.go`
- Modify: `backend/internal/room/hub.go`
- Modify: `backend/internal/tests/service/meeting_lifecycle_test.go`

- [ ] **Step 1: Add failing coordinator-focused tests for the grace timer**

Use an injected clock/scheduler so tests can deterministically fire the deadline without sleeping 30 real seconds.

Target behavior:
- leaving the last human schedules one timer
- rejoin cancels that timer
- manual close cancels any timer
- archive cancels any timer
- lazy load with a past deadline closes immediately

- [ ] **Step 2: Create `meeting_lifecycle.go`**

Recommended structure:

```go
type timerHandle interface {
	Stop() bool
}

type scheduleFunc func(delay time.Duration, fn func()) timerHandle

type MeetingLifecycle struct {
	store    store.Store
	now      func() time.Time
	schedule scheduleFunc
	logger   *slog.Logger

	mu     sync.Mutex
	timers map[string]timerHandle
}
```

Expose methods such as:

```go
func (l *MeetingLifecycle) OnParticipantJoined(ctx context.Context, currentRoom *room.Room, participant model.Participant) error
func (l *MeetingLifecycle) OnParticipantLeft(ctx context.Context, currentRoom *room.Room, participantID string) error
func (l *MeetingLifecycle) TransferOwner(ctx context.Context, currentRoom *room.Room, callerParticipantID string, targetParticipantID string) error
func (l *MeetingLifecycle) CloseByOwner(ctx context.Context, currentRoom *room.Room, callerParticipantID string) error
func (l *MeetingLifecycle) Archive(ctx context.Context, currentRoom *room.Room) error
func (l *MeetingLifecycle) Reopen(ctx context.Context, currentRoom *room.Room) error
func (l *MeetingLifecycle) Restore(ctx context.Context, currentRoom *room.Room) error
func (l *MeetingLifecycle) ReconcileLoadedRoom(ctx context.Context, currentRoom *room.Room) error
```

- [ ] **Step 3: Expand runtime room state**

In `backend/internal/room/room.go`:
- store owner/close/deadline metadata alongside `status`
- stop collapsing non-archived statuses to `active`
- add helpers to list online humans in join order
- add lifecycle mutators that apply an entire snapshot in one lock
- add a termination helper that clears runtime participants before sockets are dropped, so manual close/archive does not accidentally schedule another grace timer during cleanup

Example shape:

```go
type LifecycleState struct {
	Status              string
	OwnerParticipantID  string
	ClosedAt            *time.Time
	ClosedReason        string
	AutoCloseDeadlineAt *time.Time
	ArchivedAt          *time.Time
}
```

- [ ] **Step 4: Give the hub deterministic eject support**

In `backend/internal/room/hub.go`, add a helper that collects and drops active clients in one operation so lifecycle transitions can:
- broadcast `room_closed` or `room_archived`
- then close client channels/connections cleanly

One acceptable shape:

```go
func (h *Hub) BroadcastAndClose(event model.ServerEvent) {
	...
}
```

- [ ] **Step 5: Wire the coordinator into `RoomService`**

`RoomService` should:
- own one `MeetingLifecycle`
- call `ReconcileLoadedRoom` from `GetRoom`
- call `OnParticipantJoined` / `OnParticipantLeft` from join/leave
- use lifecycle-aware errors for human writes (`closed` vs `archived`)
- expose thin facade methods used by the API layer instead of embedding policy in handlers

- [ ] **Step 6: Run focused service tests and keep them GREEN**

Run:

```powershell
go -C backend test ./internal/tests/service -run "TestMeetingLifecycle"
go -C backend test ./internal/tests/room
```

Expected: PASS.

### Task 4: Preserve explicit minutes semantics and room list filters

**Files:**
- Modify: `backend/internal/service/room_service.go`
- Modify: `backend/internal/model/types.go`
- Modify: `backend/internal/store/store.go`
- Modify: `backend/internal/store/mysql/store.go`
- Modify: `backend/internal/tests/service/meeting_records_test.go`
- Modify: `backend/internal/tests/api/server_room_lifecycle_test.go`

- [ ] **Step 1: Add failing tests for pure-read `minutes.md`**

Replace the current download expectation with:
- no persisted minutes => `404`
- persisted minutes => `200 text/markdown`
- closed ordinary read still allowed
- archived ordinary read rejected

- [ ] **Step 2: Add a pure latest-minutes read path**

Replace `LatestMinutesMarkdown` with a pure read helper, for example:

```go
func (s *RoomService) LatestPersistedMinutesMarkdown(ctx context.Context, currentRoom *room.Room) (string, bool, error)
```

`handleDownloadMinutes` should map `ok == false` to `404`.

- [ ] **Step 3: Tighten room filters to include `closed`**

Update `ListRoomsQuery` documentation and `ListRooms` implementations so:
- `?status=active`
- `?status=closed`
- `?status=archived`
- empty or `all`

all work consistently in MySQL and the test store.

- [ ] **Step 4: Keep write permissions aligned with room status**

In `RoomService`, ordinary message and minutes generation paths should reject non-`active` rooms explicitly:

```go
var (
	ErrRoomClosed   = errors.New("room is closed")
	ErrRoomArchived = errors.New("room is archived")
)
```

- [ ] **Step 5: Run targeted tests**

Run:

```powershell
go -C backend test ./internal/tests/service -run "TestArchivedRoomRejectsHumanMessage|TestGenerateMinutes"
go -C backend test ./internal/tests/api -run "TestRoomLifecycle|TestMeetingMinutes"
```

Expected: PASS.

---

## Chunk 3: API Access Split And WebSocket Owner Controls

### Task 5: Separate read/live/admin room resolution in the API layer

**Files:**
- Modify: `backend/internal/api/server.go`
- Modify: `backend/internal/tests/api/server_room_lifecycle_test.go`

- [ ] **Step 1: Add failing API tests for the access matrix**

Cover:
- `GET /api/rooms/:roomID` ordinary read allowed for `active` and `closed`
- `GET /api/rooms/:roomID/ws` rejected for `closed` and `archived`
- `POST /api/rooms/:roomID/reopen` allowed only for admins on `closed`
- `POST /api/rooms/:roomID/archive` valid on `active` and `closed`
- `POST /api/rooms/:roomID/restore` valid only on `archived` and returns `closed`

- [ ] **Step 2: Replace `getRoomFromRequest` with intent-specific helpers**

Recommended helpers:

```go
func (s *Server) getRoomForRead(c *gin.Context) (*room.Room, bool)
func (s *Server) getRoomForLive(c *gin.Context) (*room.Room, bool)
func (s *Server) getRoomForMinutesWrite(c *gin.Context) (*room.Room, bool, bool) // room, ok, isAdmin
func (s *Server) getRoomForAdmin(c *gin.Context) (*room.Room, bool)
```

Rules:
- admins bypass passcode for all management reads
- ordinary read permits `active` and `closed`
- ordinary live permits only `active`
- ordinary minutes write permits only `active`

- [ ] **Step 3: Add `/reopen` and tighten `/restore`**

Register:

```go
routes.POST("/rooms/:roomID/reopen", s.requireAdmin, s.handleReopenRoom)
```

Keep `/restore` for unarchive-only semantics.

- [ ] **Step 4: Return lifecycle-specific HTTP errors**

Map:
- invalid transition => `409`
- archived ordinary read => `403`
- closed live join => `409` with “meeting is closed; read-only only”
- invalid cursor => `400`
- missing latest minutes => `404`

- [ ] **Step 5: Run route-level tests**

Run:

```powershell
go -C backend test ./internal/tests/api -run "TestRoomLifecycle"
```

Expected: PASS.

### Task 6: Expand WebSocket client events beyond message-only payloads

**Files:**
- Modify: `backend/internal/api/server.go`
- Modify: `backend/internal/model/types.go`
- Create: `backend/internal/tests/api/server_room_ws_test.go`

- [ ] **Step 1: Add failing WebSocket tests for owner actions**

Cover:
- non-owner `close_room` gets a room-scoped error event
- owner `transfer_owner` succeeds only for another online participant
- `transfer_owner` to offline/wrong participant returns an error event
- owner manual close broadcasts `room_closed` and then disconnects clients
- admin archive of an active room broadcasts `room_archived` and disconnects clients

- [ ] **Step 2: Parse structured client events**

Keep `message` working, but extend the switch to support:

```go
case "close_room":
case "transfer_owner":
```

This is the spec-required payload expansion note: current `ClientEvent` only models message content, so this step must happen before owner transfer can be implemented safely.

- [ ] **Step 3: Route owner actions through `RoomService` / `MeetingLifecycle`**

`handleClientEvent` should not update room ownership directly. It should call facade methods like:

```go
err := s.rooms.CloseRoomByOwner(ctx, currentRoom, participant.ID)
err := s.rooms.TransferRoomOwner(ctx, currentRoom, participant.ID, event.ParticipantID)
```

On success:
- `transfer_owner` broadcasts a fresh `room_snapshot`
- `close_room` broadcasts `room_closed`, clears live participants, and closes sockets

- [ ] **Step 4: Keep join/leave wired through lifecycle hooks**

`handleRoomWebSocket` should:
- use `getRoomForLive`
- join participant through the service
- on cleanup, delegate leave handling to the service so grace-timer logic stays centralized

- [ ] **Step 5: Run focused WebSocket tests**

Run:

```powershell
go -C backend test ./internal/tests/api -run "TestRoomWebSocket"
```

Expected: PASS.

---

## Chunk 4: Frontend Room Gateway And Closed Read-Only Flow

### Task 7: Add a room gateway that chooses live, read-only, or denial

**Files:**
- Create: `frontend/src/components/roomAccess.js`
- Create: `frontend/src/components/roomAccess.test.mjs`
- Create: `frontend/src/components/RoomGateway.jsx`
- Modify: `frontend/src/App.jsx`
- Modify: `frontend/src/routing.js`

- [ ] **Step 1: Add failing pure tests for route decision logic**

Cover:
- `active + no participantName => entry`
- `active + participantName => live`
- `closed => read-only` even without participantName
- `archived + ordinary user => denied`
- `room_closed` should clear stored participant session but keep the room link
- `room_archived` should clear session and send ordinary users away from the room surface

- [ ] **Step 2: Implement `roomAccess.js`**

Recommended exports:

```js
export function resolveRoomSurface({ roomStatus, participantName, hasRoomData })
export function nextRouteAfterLiveTermination({ status, roomId, passcode })
```

- [ ] **Step 3: Add `RoomGateway.jsx`**

Responsibilities:
- fetch `getRoom(roomId, passcode)` on route entry
- if `active`, show `RoomEntry` or `ChatRoom`
- if `closed`, render `RoomReadOnly`
- if `archived`, render an explicit denial state
- avoid duplicating room fetch logic in `App.jsx`

- [ ] **Step 4: Add a session-clearing helper in routing**

In `frontend/src/routing.js`, add:

```js
export function clearRoomSession(roomId) {}
```

This is needed so live users who are ejected by `room_closed` can stay on `/rooms/:roomID` and re-enter via read-only mode without stale `participantName` forcing `ChatRoom`.

- [ ] **Step 5: Route `/rooms/:roomID` through the gateway**

Simplify `App.jsx` so it no longer assumes “missing display name means entry screen”.

- [ ] **Step 6: Run the targeted frontend tests**

Run:

```powershell
node --test frontend/src/components/roomAccess.test.mjs
```

Expected: PASS.

### Task 8: Build the closed-room read-only experience

**Files:**
- Create: `frontend/src/components/RoomReadOnly.jsx`
- Modify: `frontend/src/api/roomClient.js`
- Modify: `frontend/src/api/roomClient.test.mjs`
- Modify: `frontend/src/styles.css`
- Modify: `frontend/src/chat-room-layout.test.mjs`

- [ ] **Step 1: Add failing client tests for paginated room history and admin-capable reads**

Cover:
- `getMessages(roomId, passcode, { before, limit })` builds the correct query string
- `getRoom`, `getMessages`, and `exportRoomMinutesMarkdown` include the admin key when one is available
- `reopenRoom(roomId)` calls `POST /api/rooms/:roomID/reopen`

- [ ] **Step 2: Extend `roomClient.js`**

Recommended shapes:

```js
export async function getRoom(roomId, passcode = '') {
  const response = await fetch(`${API_BASE_PATH}/rooms/${encodedRoomId}`, {
    headers: withAdminKey(withRoomPasscode({}, passcode)),
  })
  return parseResponse(response)
}

export async function getMessages(roomId, passcode = '', { before = '', limit } = {}) {}
export async function reopenRoom(roomId) {}
```

- [ ] **Step 3: Create `RoomReadOnly.jsx`**

Responsibilities:
- show a clear “会议已关闭，仅可只读查看” banner
- load latest room metadata and latest history page
- page older messages via `nextBefore`
- preview latest persisted minutes if available
- offer a download action for latest minutes markdown
- never call `createRoomSocket`
- never render `MessageComposer`

- [ ] **Step 4: Add regression assertions to keep read-only truly read-only**

In `frontend/src/chat-room-layout.test.mjs`, assert:
- `RoomReadOnly` exists
- it does not import `createRoomSocket`
- `App.jsx` or `RoomGateway.jsx` imports it for `closed` rooms

- [ ] **Step 5: Add styles for the closed history surface**

Add layout rules for:
- `.room-readonly`
- `.room-readonly-banner`
- `.room-history-list`
- `.room-history-load-more`
- `.room-minutes-preview`

- [ ] **Step 6: Run frontend checks**

Run:

```powershell
node --test frontend/src/api/roomClient.test.mjs
node --test frontend/src/components/roomAccess.test.mjs
node --test frontend/src/chat-room-layout.test.mjs
```

Expected: PASS.

---

## Chunk 5: Live Owner Controls And Admin Message Inspection

### Task 9: Add live-room owner controls without mixing in read-only behavior

**Files:**
- Modify: `frontend/src/components/ChatRoom.jsx`
- Modify: `frontend/src/chat-room.css`
- Modify: `frontend/src/components/roomAccess.js`
- Modify: `frontend/src/chat-room-layout.test.mjs`

- [ ] **Step 1: Add failing UI regressions for owner controls and lifecycle exits**

Cover:
- `ChatRoom` sends `close_room` and `transfer_owner` payloads through the existing socket
- `room_closed` clears the live session and hands control back to the gateway/read-only flow
- `room_archived` clears the session and exits the live surface

- [ ] **Step 2: Add owner-only controls to `ChatRoom`**

Use `room.ownerParticipantID` plus the joined participant ID from the initial snapshot to decide whether the current user is owner.

Suggested payloads:

```js
socket.send(JSON.stringify({ type: 'close_room' }))
socket.send(JSON.stringify({ type: 'transfer_owner', participantID: nextOwnerId }))
```

- [ ] **Step 3: Handle lifecycle events explicitly**

Add new event constants:

```js
const ROOM_CLOSED_EVENT = 'room_closed'
const ROOM_ARCHIVED_EVENT = 'room_archived'
```

On `room_closed`:
- close the socket
- clear the stored participant session
- navigate to the same room route so `RoomGateway` resolves to `RoomReadOnly`

On `room_archived`:
- close the socket
- clear the stored session
- show an archived/denied message or bounce to home/admin as appropriate

- [ ] **Step 4: Style the owner controls carefully**

Add compact styles in `frontend/src/chat-room.css` for:
- `.meeting-owner-panel`
- `.meeting-owner-actions`
- `.meeting-owner-badge`

Keep this additive; do not overload `MessageComposer`.

- [ ] **Step 5: Run targeted frontend regressions**

Run:

```powershell
node --test frontend/src/chat-room-layout.test.mjs
```

Expected: PASS.

### Task 10: Extend the admin meeting surface with room detail and paginated messages

**Files:**
- Create: `frontend/src/components/MeetingRoomDetail.jsx`
- Create: `frontend/src/components/meetingAdminDetail.test.mjs`
- Modify: `frontend/src/components/MeetingAdmin.jsx`
- Modify: `frontend/src/components/MinutesHistory.jsx`
- Modify: `frontend/src/styles.css`

- [ ] **Step 1: Add failing tests for admin room detail wiring**

Cover:
- `MeetingAdmin` status filter list contains `closed`
- list rows show `active`, `closed`, `archived` labels distinctly
- a detail surface is available for each room
- the detail surface loads paginated messages and minutes actions
- action labels reflect the new semantics: `archive`, `reopen`, `restore`

- [ ] **Step 2: Create `MeetingRoomDetail.jsx`**

Sections:
- **Overview**: room name, ID, status, created time, closed time/reason, owner if active
- **Messages**: paginated read-only history using `getMessages`
- **Minutes**: reuse `MinutesHistory` launch plus generate/save/export affordances for admins

- [ ] **Step 3: Update `MeetingAdmin.jsx`**

Add filters:

```js
const STATUS_FILTERS = [
  { value: '', label: '全部' },
  { value: 'active', label: '进行中' },
  { value: 'closed', label: '已关闭' },
  { value: 'archived', label: '已归档' },
]
```

Action matrix:
- `active`: detail, archive
- `closed`: detail, reopen, archive
- `archived`: detail, restore

- [ ] **Step 4: Keep `MinutesHistory` aligned with admin-only generation/editing**

Ensure the admin modal can still:
- generate minutes in `closed` and `archived`
- save manual edits in all states
- show a clear empty state when no persisted version exists yet

- [ ] **Step 5: Style the detail surface**

Add admin-panel rules in `frontend/src/styles.css` for:
- `.meeting-detail`
- `.meeting-detail-grid`
- `.meeting-detail-messages`
- `.meeting-detail-actions`

- [ ] **Step 6: Run frontend tests**

Run:

```powershell
node --test frontend/src/components/meetingAdminDetail.test.mjs
node --test frontend/src/api/roomClient.test.mjs
npm --prefix frontend run build
```

Expected: PASS.

---

## Chunk 6: Final Verification And Docs

### Task 11: Update docs and run the full verification loop

**Files:**
- Modify: `README.md`
- Modify: `backend/README.md`
- Modify: `docs/data-persistence-design.md`

- [ ] **Step 1: Update public behavior docs**

Document:
- `active`, `closed`, `archived`
- owner close vs admin archive
- closed read-only access
- `POST /api/rooms/:roomID/reopen`
- `POST /api/rooms/:roomID/restore` now means unarchive to `closed`
- `GET /api/rooms/:roomID/messages` pagination semantics
- `GET /api/rooms/:roomID/minutes.md` pure read behavior

- [ ] **Step 2: Update schema docs**

In `docs/data-persistence-design.md`, add the new `rooms` columns and explain:
- `owner_participant_id`
- `closed_at`
- `closed_reason`
- `auto_close_deadline_at`

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
node --test frontend/src/components/roomAccess.test.mjs
node --test frontend/src/components/meetingAdminDetail.test.mjs
node --test frontend/src/components/copyRegression.test.mjs
node --test frontend/src/components/meetingMinutes.test.mjs
node --test frontend/src/chat-room-layout.test.mjs
npm --prefix frontend run build
```

Expected: PASS.

- [ ] **Step 5: Manual smoke test the lifecycle**

Verify in a local run:
- create an active room, join as Alice, confirm Alice becomes owner
- join as Bob, transfer owner from Alice to Bob
- Bob closes the room and both live clients drop into read-only / exit correctly
- closed room link opens without asking for display name and shows history/minutes
- archived room link is denied for ordinary users
- admin detail view shows messages, can archive/reopen/restore correctly
- last human leave schedules close after 30 seconds and a rejoin before expiry keeps the room active

- [ ] **Step 6: Commit with a Lore-format message**

Example intent line:

```text
厘清会议结束与归档边界，补上只读历史与房主控制
```

Include trailers for:
- `Constraint:` lightweight no-account ownership model
- `Rejected:` owner HTTP mutations | ownership is bound to the live websocket session
- `Directive:` do not collapse `closed` into `active`
- `Tested:` commands actually run
- `Not-tested:` any remaining manual gaps

## Risks And Mitigations

- **Risk: Manual close/archive can race with socket cleanup and accidentally reschedule auto-close.**
  - Mitigation: clear runtime participants and cancel timers before dropping clients; keep leave side effects centralized in the lifecycle coordinator.

- **Risk: `closed` logic leaks into live-only UI and makes `ChatRoom` harder to reason about.**
  - Mitigation: route through `RoomGateway` and keep `RoomReadOnly` as a separate component.

- **Risk: Admin archived-room reads break because client helpers currently omit the admin key on read endpoints.**
  - Mitigation: update `getRoom`, `getMessages`, and `exportRoomMinutesMarkdown` to send `withAdminKey(...)`.

- **Risk: Message pagination regressions affect existing chat bootstrap.**
  - Mitigation: keep a plain internal `ListMessages` path for live bootstrap and minutes generation; add a separate paginated API read path with dedicated tests.

- **Risk: Timer behavior becomes flaky in tests.**
  - Mitigation: inject `now` and `schedule` into `meeting_lifecycle.go`; never use `time.Sleep(30 * time.Second)` in tests.

## Execution Notes

- Do not add a separate participant-authenticated HTTP endpoint for owner actions. `close_room` and `transfer_owner` stay on WebSocket by design.
- Do not keep `normalizeRoomStatus` collapsing unknown statuses to `active`; `closed` must remain first-class from storage through UI.
- Do not auto-generate minutes from `GET /minutes.md`; generation must stay explicit through `POST /minutes`.
- Prefer additive helpers over large rewrites in `App.jsx` and `MeetingAdmin.jsx`.
- Keep Chinese UI copy aligned with the new semantics: `进行中`, `已关闭`, `已归档`, `只读查看`, `恢复会议`, `取消归档`.
