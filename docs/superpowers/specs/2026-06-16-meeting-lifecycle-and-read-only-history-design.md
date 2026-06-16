# Meeting Lifecycle, Ownership, And Read-Only History Design

Date: 2026-06-16
Status: Approved for implementation planning

## Background

AgentRoom currently treats room lifecycle as a two-state model: `active` and `archived`. This collapses two different meanings into one flag:

- a meeting has naturally ended or has been explicitly closed by meeting participants
- an administrator has archived the room for long-term management purposes

That model creates the wrong authority boundary. Meeting closure should belong to the meeting owner inside the room, while archiving remains an administrator-only management action.

At the same time, the current system has no persisted owner concept, no owner handoff workflow, no automatic close when the last human leaves, and no admin-facing way to inspect the full meeting message history from the meeting management surface.

## Goals

- Separate participant-driven meeting closure from administrator-driven archival.
- Introduce an explicit room owner model without adding a full account system.
- Allow the room owner to close the meeting and transfer ownership to another online human participant.
- Automatically close the meeting when the last human leaves, with a 30-second grace period.
- Allow ordinary participants to access a closed room in read-only mode for history review, not for live participation.
- Keep archived rooms as an admin-only management state.
- Expose the full room message history from the admin meeting management UI.
- Preserve the existing lightweight room architecture and avoid introducing new dependencies.

## Non-Goals

- No formal user account, login, or durable identity system in this phase.
- No resumable owner token or owner recovery across refreshes/rejoins.
- No participant-level edit/delete permissions for historical messages.
- No admin-triggered "close meeting" action; admin controls recovery and archival, not in-room ownership.
- No per-message audit trail for owner transfers beyond room metadata and live events.
- No change to Agent dialogue policy semantics beyond respecting the new room lifecycle state.

## Decision Summary

### 1. Three distinct room lifecycle states

Rooms move between three persisted states:

- `active`: live meeting; humans can join via WebSocket and send messages.
- `closed`: meeting ended; ordinary users may open the room in read-only mode, but cannot join live, send messages, transfer ownership, or reopen the meeting.
- `archived`: admin-managed long-term state; ordinary users cannot access the room by link, while admins can inspect it from the management console.

`closed` and `archived` are intentionally different:

- `closed` means "the meeting has ended"
- `archived` means "management has sealed this room"

### 2. Owner is an online human participant instance

The room owner is modeled as the current online human participant instance, referenced by `owner_participant_id`.

This is the intentionally limited phase-1 ownership model:

- ownership is tied to the current joined participant instance
- ownership can be handed off only to another currently online human
- ownership does not survive a leave/rejoin cycle for the same person

This matches the chosen trade-off: keep implementation lightweight now, and accept that durable owner identity is a future enhancement.

### 3. First successful human join claims the initial owner role

Room creation currently persists room metadata before the creator becomes a persisted participant. To stay inside the selected ownership model, the initial owner is assigned on the first successful human live join for an ownerless active room.

In the standard create flow, this first join is the creator entering the room immediately after creation, so the result still behaves as "creator becomes owner" without adding a new identity mechanism.

Accepted limitation:

- if a room is created but the creator never successfully joins, the first other human who joins the live room becomes the initial owner

### 4. Closed rooms become read-only, not re-openable by ordinary users

An ordinary participant may still open a closed room link and review:

- historical messages
- latest meeting minutes
- basic room metadata

They may not:

- establish a live room WebSocket session
- appear as an online participant
- send messages
- transfer ownership
- reopen the meeting

Only an administrator may reopen a closed room.

### 5. Archived rooms remain admin-only

Archived rooms keep a stronger access boundary than closed rooms:

- ordinary users cannot access archived rooms by link
- admins can inspect archived rooms from the management UI
- unarchiving returns the room to `closed`, not directly to `active`

This preserves a clean semantic distinction between "ended but reviewable" and "management-sealed".

### 6. Meeting management must expose complete message history

The admin meeting management experience expands from list/archive/minutes into a room detail view that also exposes the full message history for the selected meeting.

This is a read-only inspection surface:

- no admin editing of messages
- no admin deletion of messages
- no separate admin-only message model

The admin UI reuses the room message history as the source of truth.

## Lifecycle State Machine

### Canonical transitions

- `active -> closed`
  - owner manually closes the meeting
  - last human leaves, grace period expires, and no human has returned
- `closed -> active`
  - administrator explicitly reopens the meeting
- `active -> archived`
  - administrator archives the room
- `closed -> archived`
  - administrator archives the room
- `archived -> closed`
  - administrator removes the archive state

### Invalid transitions

- ordinary participant `closed -> active`
- ordinary participant `archived -> active`
- administrator direct `archived -> active`
- any live join against `closed` or `archived`
- owner handoff in `closed` or `archived`
- owner manual close in `closed` or `archived`

### State metadata rules

- `active`
  - `owner_participant_id` may be set or empty
  - `closed_at` is null
  - `closed_reason` is empty
  - `auto_close_deadline_at` may be set only while waiting to determine auto-close after the last human left
- `closed`
  - `owner_participant_id` is empty
  - `closed_at` is set
  - `closed_reason` is `manual`, `last_human_left`, or `admin_unarchive`
  - `auto_close_deadline_at` is null
- `archived`
  - `owner_participant_id` is empty
  - `auto_close_deadline_at` is null
  - `closed_at` / `closed_reason` preserve the prior closure semantics if the room was already closed before archiving

### Transition effects table

| Transition | Persisted fields | Timer handling | Live participant / socket effect |
| --- | --- | --- | --- |
| `active -> closed` (owner manual close) | set `status = closed`; set `closed_at = now`; set `closed_reason = manual`; clear `owner_participant_id`; clear `auto_close_deadline_at` | cancel pending auto-close timer | broadcast `room_closed`; eject all connected clients; close room WebSockets |
| `active -> closed` (last human left timeout) | set `status = closed`; set `closed_at = deadline fire time`; set `closed_reason = last_human_left`; clear `owner_participant_id`; clear `auto_close_deadline_at` | consume and clear auto-close timer | no eject needed because no humans remain connected |
| `active -> archived` | set `status = archived`; set `archived_at = now`; clear `owner_participant_id`; clear `auto_close_deadline_at`; keep `closed_at` and `closed_reason` empty | cancel pending auto-close timer | broadcast `room_archived`; eject all connected clients; close room WebSockets |
| `closed -> active` (admin reopen) | set `status = active`; clear `closed_at`; clear `closed_reason`; clear `owner_participant_id`; clear `auto_close_deadline_at`; clear `archived_at` | no timer until a future last-human-left event | no clients are connected; first later human join claims owner |
| `closed -> archived` | set `status = archived`; set `archived_at = now`; preserve existing `closed_at` and `closed_reason`; clear `owner_participant_id`; clear `auto_close_deadline_at` | no timer remains | no live sockets should exist |
| `archived -> closed` when archived room was previously closed | set `status = closed`; clear `archived_at`; preserve prior `closed_at` and `closed_reason`; keep `owner_participant_id` empty; keep `auto_close_deadline_at` null | no timer | no live sockets should exist |
| `archived -> closed` when archived room came from `active` | set `status = closed`; clear `archived_at`; set `closed_at = now`; set `closed_reason = admin_unarchive`; keep `owner_participant_id` empty; keep `auto_close_deadline_at` null | no timer | no live sockets should exist |

## Ownership And Permission Rules

### Owner capabilities

The current room owner may:

- close the active meeting
- transfer ownership to another online human participant

The current room owner may not:

- archive the room
- unarchive the room
- reopen a closed room

### Admin capabilities

An administrator may:

- inspect all room states from the management UI
- reopen a closed room
- archive an active or closed room
- unarchive an archived room back to `closed`
- inspect full message history, minutes history, and room metadata

An administrator does not implicitly become the room owner.

### Ordinary participant capabilities

Ordinary participants may:

- join an active room live
- read a closed room in read-only mode

They may not:

- reopen a closed room
- enter an archived room
- close the room unless they are the current owner
- transfer ownership unless they are the current owner

## Runtime Behavior

### 1. Human joins an active room

When a human successfully joins an active room via WebSocket:

- the participant is persisted and added to runtime state
- if the room has no owner, that participant becomes owner
- any pending auto-close deadline is cancelled
- live clients receive an updated room snapshot reflecting participant and owner state

### 2. Owner handoff

Owner handoff is permitted only when:

- the room is `active`
- the caller is the current owner
- the target is a currently online human participant in the same room

On success:

- `owner_participant_id` is updated atomically
- live clients receive an updated room snapshot

No persisted system message is required in phase 1. Ownership changes remain a room-state concern rather than a transcript concern.

### 3. Owner leaves while other humans remain

If the current owner leaves an active room and other online humans still remain:

- ownership transfers automatically to the earliest joined currently online human participant
- no grace timer is scheduled
- live clients receive an updated room snapshot

### 4. Last human leaves an active room

If the last online human leaves an active room:

- clear `owner_participant_id`
- set `auto_close_deadline_at = now + 30s`
- keep room status as `active` during the grace window
- schedule an in-memory timer to evaluate closure at the deadline

If a human rejoins before the deadline:

- cancel the timer
- clear `auto_close_deadline_at`
- keep the room `active`
- assign the owner to the earliest joined currently online human

If no human returns by the deadline:

- transition the room to `closed`
- set `closed_at`
- set `closed_reason = last_human_left`
- clear `owner_participant_id`
- clear `auto_close_deadline_at`

Because there are no humans online at that moment, no forced client eject is needed for the auto-close path.

### 5. Owner manually closes the room

When the owner manually closes an active room:

- set room status to `closed`
- set `closed_at`
- set `closed_reason = manual`
- clear `owner_participant_id`
- clear any pending `auto_close_deadline_at`
- broadcast a dedicated live event telling clients the meeting has been closed
- force all currently connected clients back to the meeting entry surface
- close their room WebSocket connections on the server side

This path is intentionally disruptive because the selected product behavior is "close now, everyone leaves now."

### 6. Service restart during an auto-close grace window

Because auto-close uses an in-memory timer but must remain correct across process restarts:

- the deadline is also persisted in `rooms.auto_close_deadline_at`
- when a room is lazily reloaded from storage:
  - if status is `active`, deadline is set, and no online humans are loaded
  - if the deadline has already passed, close the room immediately
  - if the deadline is still in the future, re-arm the timer for the remaining duration

This keeps the runtime timer lightweight while preserving correctness.

## Access Model By Route Type

The existing `getRoomFromRequest` helper is too coarse for the new lifecycle. The backend should distinguish room access by intent:

- **read access**
  - ordinary users: `active` and `closed` (with valid passcode if required)
  - ordinary users denied for `archived`
  - admins may read all states
- **live access**
  - ordinary users and admins: only `active`
- **admin management access**
  - admin key only, regardless of passcode

This split avoids mixing read-only closed access with live room participation.

## HTTP And WebSocket Contract

### Read APIs

`GET /rooms/:roomID`

- `active`: returns live-capable room metadata
- `closed`: returns read-only room metadata, including closure reason/timestamps
- `archived`: ordinary users rejected; admins allowed

`GET /rooms/:roomID/messages`

- allowed for ordinary users in `active` and `closed`
- denied for ordinary users in `archived`
- allowed for admins in all states
- upgraded to cursor-aware pagination metadata:
  - `messages`
  - `hasMore`
  - `nextBefore`
- without `before`, return the latest page of messages
- within each response page, `messages` are ordered chronologically from oldest to newest
- `before` is an exclusive cursor meaning "return messages strictly older than this message ID"
- `nextBefore` is the ID of the oldest returned message in the current page when older messages remain; otherwise it is empty or null
- invalid `before` values are a client error:
  - if the cursor is malformed, return `400 Bad Request`
  - if the cursor does not identify a message in the same room, return `400 Bad Request`

Example response shape:

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

`GET /rooms/:roomID/minutes.md`

- allowed for ordinary users in `active` and `closed`
- denied for ordinary users in `archived`
- allowed for admins in all states
- pure download/read endpoint only; it must never generate or persist new minutes as a side effect
- if no persisted minutes exist yet, return a not-found-style response and let the UI show "no minutes generated yet"

`GET /rooms/:roomID/minutes/history`

- remains admin-only
- ordinary users only see the latest minutes view, not version history

`POST /rooms/:roomID/minutes`

- ordinary users allowed only for `active`
- ordinary users denied for `closed` and `archived`
- admins allowed for `active`, `closed`, and `archived`
- this preserves read-only semantics for closed rooms while keeping the admin management minutes workflow available

### Admin mutation APIs

`POST /rooms/:roomID/reopen`

- admin-only
- valid only for `closed`
- clears closure metadata and returns room to `active`
- owner remains empty until the first new human live join
- no request body
- success response returns the updated room metadata

### Existing admin mutation APIs with clarified semantics

`POST /rooms/:roomID/archive`

- admin-only
- valid for `active` and `closed`
- transitions room to `archived`
- no request body
- success response returns the updated room metadata
- if the room was `active`, archiving is a terminating live action: connected clients are ejected and room WebSockets are closed

`POST /rooms/:roomID/restore`

- keep the route for backward compatibility, but change semantics to "unarchive"
- admin-only
- valid only for `archived`
- always returns the room to `closed`, never directly to `active`
- no request body
- success response returns the updated room metadata
- if the archived room had no prior closure metadata because it was archived from `active`, the restore sets `closed_at = now` and `closed_reason = admin_unarchive`

### WebSocket

`GET /rooms/:roomID/ws`

- allowed only for `active`
- rejected for `closed` with a lifecycle-specific error
- rejected for `archived` for all callers

Owner-only room-control actions are intentionally handled inside the active room WebSocket session rather than via standalone ordinary HTTP endpoints. This avoids inventing a second caller-auth model for participant-bound ownership, because the server already knows which participant is bound to each WebSocket connection.

New client events:

- `close_room`
  - no extra payload
  - valid only for the current owner in an `active` room
  - on success triggers the manual-close transition and broadcasts `room_closed`
- `transfer_owner`
  - payload:

```json
{
  "type": "transfer_owner",
  "participantID": "participant_123"
}
```

  - valid only for the current owner in an `active` room
  - target must be a currently online human participant
  - on success updates ownership and broadcasts a fresh `room_snapshot`

New live event:

- `room_closed`
  - broadcast when the owner manually closes the meeting
  - tells connected clients to leave the live surface immediately

For owner transfer and auto-close scheduling changes, a fresh `room_snapshot` is sufficient after `RoomMeta` is expanded to include owner and closure fields.

Additional live termination event:

- `room_archived`
  - broadcast when an active room is archived by an administrator
  - tells connected clients to leave the live surface immediately because the room is no longer available to ordinary users

## Data Model And Persistence

### Room metadata

Extend `model.RoomMeta`, the in-memory `room.Room`, and the `rooms` table with:

- `status`: `active | closed | archived`
- `ownerParticipantID`
- `closedAt`
- `closedReason`
- `autoCloseDeadlineAt`

`archivedAt` remains as today.

### Participant model

No participant schema change is required. The current `participants` table already supports:

- online membership via `left_at IS NULL`
- stable ordering by `joined_at`

That is sufficient for:

- picking the earliest joined online human for automatic owner reassignment
- determining whether the last human has left

### Message model

No message schema change is required for this feature. Full admin inspection uses existing persisted messages.

### Store interface shape

The current `SetRoomStatus` method is too narrow for this lifecycle model. Introduce one lifecycle-focused persistence method instead of many small point updates.

Recommended shape:

- `UpdateRoomLifecycle(ctx, input)`

Where the input atomically carries:

- `status`
- `ownerParticipantID`
- `closedAt`
- `closedReason`
- `autoCloseDeadlineAt`
- `archivedAt`

This avoids stitching multi-step state transitions together from loosely related store calls.

## Current Behavior Changes Required

The implementation plan must explicitly change the following existing behaviors:

- `normalizeRoomStatus` in runtime room state currently collapses non-archived statuses to `active`; it must preserve `closed` as a first-class state.
- `GET /rooms/:roomID/minutes.md` currently falls back to generating minutes when none exist; it must become a pure read/download endpoint with no write side effect.
- `POST /rooms/:roomID/minutes` is currently reachable through ordinary room access; it must be blocked for ordinary users when the room is `closed` or `archived`.
- archiving an active room must no longer leave existing WebSocket clients connected until their next send attempt; archive becomes an immediate terminating action for the live session.
- owner-only room control currently has no secure caller-binding outside the WebSocket session; owner close and owner transfer must be implemented as WebSocket control events rather than unauthenticated ordinary HTTP mutations.

## Backend Module Boundaries

### RoomService remains the facade

HTTP and WebSocket handlers should still call `RoomService`, not reach through to lifecycle-specific internals.

### Add a focused meeting lifecycle coordinator

To keep `room_service.go` from absorbing all ownership and timer logic, add a dedicated service module such as:

- `backend/internal/service/meeting_lifecycle.go`

Responsibilities:

- owner assignment
- owner transfer validation
- auto-close scheduling and cancellation
- active/closed/archived transitions
- lifecycle-specific persistence
- runtime room snapshot refreshes and close-event broadcast

`RoomService` delegates:

- participant join/leave side effects
- owner/admin room mutations
- startup reconciliation for pending auto-close windows

### Runtime room state remains a state container

`backend/internal/room/room.go` should hold mutable lifecycle fields and expose focused getters/setters, but it should not own lifecycle policy or timers.

Timers remain outside `room.Room` so that:

- state storage stays simple
- policy remains testable
- scheduler behavior can be injected and mocked

## Frontend Module Boundaries

### Split live and read-only room surfaces

Do not overload `ChatRoom` with read-only behavior. Introduce a separate component:

- `RoomReadOnly`

Responsibilities:

- fetch closed-room metadata
- fetch paginated historical messages
- show latest minutes export or preview affordances
- show a clear "meeting closed" banner
- never create a WebSocket connection
- never show a message composer

`ChatRoom` remains live-only.

Closed-room read-only access must not require a display name. The route should render `RoomReadOnly` directly after room/passcode validation, without routing through the current display-name entry flow that exists for live participation.

### Add a route-level room gateway

The room route needs a small decision layer that determines whether `/rooms/:roomID` resolves to:

- live room flow
- closed-room read-only flow
- archived-room denial
- passcode prompt when needed

This keeps room access decisions out of both `ChatRoom` and `RoomReadOnly`.

### Extend meeting management

Add a dedicated room detail surface from `MeetingAdmin`, such as:

- drawer
- modal
- split-pane detail view

Recommended sections:

- **Overview**
  - name, ID, status, timestamps, close reason, owner when applicable
- **Messages**
  - full read-only paginated message history
- **Minutes**
  - reuse existing minutes history/editor/export surface

Meeting list filters become:

- all
- active
- closed
- archived

## Error Handling Rules

- non-owner `close_room` event -> room-scoped authorization error surfaced through the WebSocket error channel
- non-owner `transfer_owner` event -> room-scoped authorization error surfaced through the WebSocket error channel
- owner transfer target offline or not human -> room-scoped conflict/error response surfaced through the WebSocket error channel
- ordinary reopen request -> `403 Forbidden`
- invalid lifecycle transition -> `409 Conflict`
- closed-room WebSocket request -> lifecycle-specific `409` or `403` response, surfaced as "meeting is closed; read-only only"
- archived ordinary read request -> `403 Forbidden`
- passcode mismatch for active/closed ordinary read -> `403 Forbidden`

These errors should be explicit in both backend JSON responses and frontend UI copy so users understand whether the room is closed, archived, or simply access-protected.

## Testing Strategy

### Backend service tests

Add tests for:

- first successful human live join claims owner when owner is empty
- owner transfer succeeds only to a currently online human
- owner leaving with other humans online automatically reassigns ownership
- last human leaving sets a 30-second auto-close deadline
- rejoin inside the grace window cancels auto-close and restores an owner
- grace expiry closes the room with `closed_reason = last_human_left`
- manual owner close sets `closed_reason = manual`
- reopen clears closure metadata and leaves owner empty
- unarchive always returns to `closed`

### API tests

Add tests for:

- ordinary read access to closed rooms succeeds
- ordinary live join to closed rooms is rejected
- ordinary access to archived rooms is rejected
- admin access to archived rooms succeeds
- owner-only WebSocket control events reject non-owners
- reopen rejects non-admin callers
- restore/unarchive never returns directly to `active`
- message history pagination metadata is correct
- invalid message cursors return `400 Bad Request`

### Frontend tests

Add tests for:

- room route chooses `RoomReadOnly` for closed rooms
- closed read-only room access does not require a display name
- `RoomReadOnly` renders messages and latest minutes without a composer
- closed room never opens a WebSocket connection
- manual close event ejects live users back to the entry page
- meeting admin shows `closed` status and room detail history
- admin room detail can page through historical messages

## Rollout Plan

### Phase 1: Persistence and room metadata

- extend room status enum and metadata fields
- migrate `rooms` table
- extend storage and runtime models

### Phase 2: Lifecycle coordinator

- implement owner assignment, transfer, close, reopen, archive, unarchive
- add scheduler injection for auto-close logic
- reconcile pending auto-close deadlines on room load

### Phase 3: Access split

- separate room read access from live access in the API layer
- add closed-room read-only flow
- reject closed-room WebSocket joins

### Phase 4: Frontend room surfaces

- add route gateway
- add `RoomReadOnly`
- add owner controls to the live room UI

### Phase 5: Admin meeting management

- add closed-state filters
- add room detail view
- add full message-history inspection
- wire reopen and clarified unarchive actions

## Acceptance Criteria

This design is ready for implementation planning when the following are true:

- `closed` and `archived` have distinct semantics and authority boundaries
- meeting ownership belongs to an online human participant instance
- the owner can transfer ownership only to another online human
- the last human leaving closes the meeting after a 30-second grace period
- manual owner close immediately ejects live users
- ordinary participants can review closed meetings in read-only mode
- ordinary participants cannot reopen closed rooms
- archived rooms remain admin-only
- admins can inspect complete historical room messages from the meeting management UI
- the design stays within the current lightweight architecture and adds no new dependency
