# Backend Architecture Governance Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reduce architectural risk in the Go backend without changing product behavior, so the next product iteration builds on stable service boundaries instead of growing a monolithic control layer.

**Architecture:** Keep the current package layout as the migration shell, but tighten contracts and split responsibilities incrementally. The refactor should first eliminate correctness risks (error contracts, cache/store consistency, partial room loads), then separate realtime transport from meeting application logic, and finally add runtime governance for caches, timers, and background work.

**Tech Stack:** Go 1.22, Gin, Gorilla WebSocket, GORM/MySQL, LangChainGo-compatible LLM client, existing backend integration tests under `backend/internal/tests/**`

---

## Current Risk Summary

- `backend/internal/api/server.go` is acting as HTTP adapter, WebSocket coordinator, access-policy gate, and error-mapping layer at the same time.
- `backend/internal/service/room_service.go` has become a catch-all application service for meetings, agents, knowledge, focus, minutes, and lifecycle rules.
- `backend/internal/room/manager.go` can cache partially loaded rooms when one of several store calls fails.
- `backend/internal/service/agent_service.go` updates in-memory state before persistence succeeds.
- `backend/internal/service/focus_service.go` performs LLM analysis while holding a global write lock.
- `backend/internal/model/types.go` mixes domain entities with transport DTOs and WebSocket envelopes.
- `backend/internal/store/mysql/store.go` concentrates all persistence for unrelated aggregates in one file, which increases change coupling.

## Target Boundaries

### Package Intent

- `backend/internal/api/`
  - HTTP and WebSocket adapters only.
  - Request decoding, response encoding, auth/passcode extraction, protocol translation.
- `backend/internal/app/` or focused `backend/internal/service/*_service.go`
  - Application use cases only.
  - No direct WebSocket coordination, no transport DTOs, no string-based error classification.
- `backend/internal/room/`
  - Runtime meeting aggregate and room-local state transitions only.
  - No transport knowledge except a narrow event sink abstraction.
- `backend/internal/agent/`
  - Agent prompting and orchestration only.
  - Depends on runtime abstractions, not concrete websocket broadcasting.
- `backend/internal/store/`
  - Repository contracts and typed persistence errors.
- `backend/internal/store/mysql/`
  - MySQL repository implementation split by aggregate.
- `backend/internal/model/`
  - Domain entities and value objects only.
- `backend/internal/api/dto/` or `backend/internal/api/contracts/`
  - REST/WebSocket request and response payloads.

### Non-Goals

- Do not rewrite the backend into a full DDD/CQRS architecture.
- Do not change REST or WebSocket behavior visible to the frontend during the first governance pass.
- Do not replace Gin, GORM, or the current LLM client.

## File Map Before Refactor

### Existing hotspots to shrink

- Modify: `backend/internal/api/server.go`
- Modify: `backend/internal/service/room_service.go`
- Modify: `backend/internal/service/agent_service.go`
- Modify: `backend/internal/service/meeting_lifecycle.go`
- Modify: `backend/internal/service/focus_service.go`
- Modify: `backend/internal/room/manager.go`
- Modify: `backend/internal/room/room.go`
- Modify: `backend/internal/model/types.go`
- Modify: `backend/internal/store/store.go`
- Modify: `backend/internal/store/mysql/store.go`

### Planned new files

- Create: `backend/internal/service/errors.go`
- Create: `backend/internal/store/errors.go`
- Create: `backend/internal/service/room_commands.go`
- Create: `backend/internal/service/room_queries.go`
- Create: `backend/internal/service/room_access.go`
- Create: `backend/internal/service/realtime_session.go`
- Create: `backend/internal/room/events.go`
- Create: `backend/internal/api/rest_handlers.go`
- Create: `backend/internal/api/ws_handlers.go`
- Create: `backend/internal/api/access.go`
- Create: `backend/internal/api/errors.go`
- Create: `backend/internal/api/contracts/rooms.go`
- Create: `backend/internal/api/contracts/agents.go`
- Create: `backend/internal/api/contracts/ws.go`
- Create: `backend/internal/store/mysql/agents_repo.go`
- Create: `backend/internal/store/mysql/rooms_repo.go`
- Create: `backend/internal/store/mysql/messages_repo.go`
- Create: `backend/internal/store/mysql/knowledge_repo.go`
- Create: `backend/internal/store/mysql/minutes_repo.go`
- Create: `backend/internal/store/mysql/runs_repo.go`
- Create: `backend/internal/store/mysql/participants_repo.go`
- Create: `backend/internal/tests/service/error_contract_test.go`
- Create: `backend/internal/tests/service/room_snapshot_loading_test.go`
- Create: `backend/internal/tests/service/focus_service_locking_test.go`
- Create: `backend/internal/tests/api/error_mapping_test.go`

## Chunk 1: Normalize Error Contracts

### Task 1: Remove string-based error classification

**Files:**
- Create: `backend/internal/service/errors.go`
- Create: `backend/internal/store/errors.go`
- Modify: `backend/internal/service/room_service.go`
- Modify: `backend/internal/service/agent_service.go`
- Modify: `backend/internal/service/knowledge_service.go`
- Modify: `backend/internal/api/server.go`
- Modify: `backend/internal/store/store.go`
- Modify: `backend/internal/store/mysql/store.go`
- Test: `backend/internal/tests/service/error_contract_test.go`
- Test: `backend/internal/tests/api/error_mapping_test.go`

- [ ] Define sentinel or typed errors for not-found, invalid transition, invalid input, and optimistic no-op conditions.
- [ ] Make store implementations wrap typed errors with `%w` instead of returning human-text-only `fmt.Errorf("... not found")`.
- [ ] Make services translate store errors into service errors using `errors.Is`, not substring checks.
- [ ] Move HTTP error mapping into a dedicated helper and delete all `strings.Contains(err.Error(), ...)` branches from `server.go`.
- [ ] Add regression tests for agent deletion, room knowledge lookup, lifecycle updates, and minutes validation.
- [ ] Run: `go -C backend test ./internal/tests/service ./internal/tests/api`
- [ ] Commit.

## Chunk 2: Fix State Consistency Before Boundary Splits

### Task 2: Make AgentService persistence-first

**Files:**
- Modify: `backend/internal/service/agent_service.go`
- Test: `backend/internal/tests/service/agent_service_test.go`

- [ ] Change update flow so the authoritative write happens before in-memory cache replacement, or add rollback on persistence failure.
- [ ] Re-check create/delete flows for the same cache/store ordering rule.
- [ ] Add a regression test proving failed persistence does not mutate in-memory agent resolution.
- [ ] Run: `go -C backend test ./internal/tests/service -run AgentService`
- [ ] Commit.

### Task 3: Make room loading atomic

**Files:**
- Modify: `backend/internal/room/manager.go`
- Modify: `backend/internal/store/store.go`
- Modify: `backend/internal/store/mysql/store.go`
- Test: `backend/internal/tests/service/room_snapshot_loading_test.go`
- Test: `backend/internal/tests/room/manager_test.go`

- [ ] Replace the multi-call partial load pattern with a single repository-level snapshot load, or fail the load when any required subquery fails.
- [ ] Ensure failed room hydration never enters `Manager.rooms`.
- [ ] Add tests that simulate partial persistence failures and assert no broken cached room survives.
- [ ] Run: `go -C backend test ./internal/tests/room ./internal/tests/service -run Room`
- [ ] Commit.

## Chunk 3: Split Application Use Cases Out of RoomService

### Task 4: Separate command/query/access concerns

**Files:**
- Create: `backend/internal/service/room_commands.go`
- Create: `backend/internal/service/room_queries.go`
- Create: `backend/internal/service/room_access.go`
- Modify: `backend/internal/service/room_service.go`
- Modify: `backend/internal/api/server.go`
- Test: `backend/internal/tests/api/server_room_lifecycle_test.go`
- Test: `backend/internal/tests/api/server_meeting_records_test.go`

- [ ] Move room reads, room writes, and passcode/access policy into dedicated focused services.
- [ ] Keep `RoomService` only as a temporary facade during migration, then shrink it until it can be removed.
- [ ] Make API handlers depend on smaller interfaces instead of the full room super-service.
- [ ] Run: `go -C backend test ./internal/tests/api`
- [ ] Commit.

### Task 5: Pull transport policy out of the HTTP server monolith

**Files:**
- Create: `backend/internal/api/rest_handlers.go`
- Create: `backend/internal/api/ws_handlers.go`
- Create: `backend/internal/api/access.go`
- Create: `backend/internal/api/errors.go`
- Modify: `backend/internal/api/server.go`
- Test: `backend/internal/tests/api/server_security_test.go`
- Test: `backend/internal/tests/api/server_room_ws_test.go`

- [ ] Keep `Server` as a router assembler only.
- [ ] Move REST handlers, WebSocket handlers, room access checks, and HTTP error mapping into separate files.
- [ ] Ensure WebSocket logic no longer owns lifecycle policy decisions directly.
- [ ] Run: `go -C backend test ./internal/tests/api`
- [ ] Commit.

## Chunk 4: Decouple Meeting Domain From WebSocket Transport

### Task 6: Introduce a room event sink abstraction

**Files:**
- Create: `backend/internal/room/events.go`
- Modify: `backend/internal/room/room.go`
- Modify: `backend/internal/agent/runner.go`
- Modify: `backend/internal/service/meeting_lifecycle.go`
- Modify: `backend/internal/api/ws_handlers.go`
- Test: `backend/internal/tests/agent/activity_events_test.go`
- Test: `backend/internal/tests/api/server_room_ws_test.go`

- [ ] Replace direct `Room.Hub().Broadcast*` coupling with a narrower event publisher or sink.
- [ ] Keep room aggregate methods focused on state, IDs, timestamps, and message creation.
- [ ] Make `agent.Runner` publish domain/realtime events through an interface instead of a concrete runtime room with websocket behavior.
- [ ] Run: `go -C backend test ./internal/tests/agent ./internal/tests/api -run \"Activity|WS|Lifecycle\"`
- [ ] Commit.

### Task 7: Move websocket session orchestration out of handlers

**Files:**
- Create: `backend/internal/service/realtime_session.go`
- Modify: `backend/internal/api/ws_handlers.go`
- Modify: `backend/internal/room/hub.go`
- Test: `backend/internal/tests/api/server_room_ws_test.go`

- [ ] Extract join, cleanup, connection lifecycle, and client event dispatch orchestration into a dedicated application component.
- [ ] Keep the API layer limited to protocol upgrade and adapter concerns.
- [ ] Explicitly decide how request context cancellation should propagate instead of defaulting to `context.Background()`.
- [ ] Run: `go -C backend test ./internal/tests/api -run WS`
- [ ] Commit.

## Chunk 5: Add Runtime Governance

### Task 8: Bound caches and background work

**Files:**
- Modify: `backend/internal/room/manager.go`
- Modify: `backend/internal/room/room.go`
- Modify: `backend/internal/service/focus_service.go`
- Modify: `backend/internal/service/room_commands.go`
- Test: `backend/internal/tests/service/focus_service_locking_test.go`
- Test: `backend/internal/tests/service/meeting_lifecycle_test.go`

- [ ] Add eviction or retention strategy for inactive rooms in `Manager.rooms`.
- [ ] Bound in-memory message history retained inside runtime rooms.
- [ ] Rework `FocusService` so it does not call the LLM while holding the global service lock.
- [ ] Add backpressure or worker-pool semantics for asynchronous agent response triggering.
- [ ] Run: `go -C backend test ./internal/tests/service`
- [ ] Commit.

## Chunk 6: Untangle Domain Models From API Contracts

### Task 9: Move transport DTOs out of `model`

**Files:**
- Create: `backend/internal/api/contracts/rooms.go`
- Create: `backend/internal/api/contracts/agents.go`
- Create: `backend/internal/api/contracts/ws.go`
- Modify: `backend/internal/model/types.go`
- Modify: `backend/internal/api/rest_handlers.go`
- Modify: `backend/internal/api/ws_handlers.go`
- Test: `backend/internal/tests/api/server_meeting_records_test.go`
- Test: `backend/internal/tests/api/server_security_test.go`

- [ ] Leave pure domain entities and value objects in `backend/internal/model`.
- [ ] Move REST request/response types and websocket envelope types into API-local contracts.
- [ ] Update handlers and tests to use the new contract package without changing public JSON payload shape.
- [ ] Run: `go -C backend test ./internal/tests/api ./internal/tests/model`
- [ ] Commit.

## Chunk 7: Split MySQL Persistence By Aggregate

### Task 10: Break up the giant MySQL store file

**Files:**
- Modify: `backend/internal/store/mysql/store.go`
- Create: `backend/internal/store/mysql/agents_repo.go`
- Create: `backend/internal/store/mysql/rooms_repo.go`
- Create: `backend/internal/store/mysql/messages_repo.go`
- Create: `backend/internal/store/mysql/knowledge_repo.go`
- Create: `backend/internal/store/mysql/minutes_repo.go`
- Create: `backend/internal/store/mysql/runs_repo.go`
- Create: `backend/internal/store/mysql/participants_repo.go`
- Test: `backend/internal/tests/api/server_meeting_records_test.go`
- Test: `backend/internal/tests/service/meeting_records_test.go`

- [ ] Split methods by aggregate responsibility while preserving the exported `MySQLStore` type.
- [ ] Keep helpers local to the relevant repository file instead of re-growing a new catch-all utility layer.
- [ ] Re-run the full backend test suite after the split.
- [ ] Run: `go -C backend test ./...`
- [ ] Commit.

## Verification Gate

- [ ] `go -C backend test ./...`
- [ ] `go -C backend build ./cmd/server`
- [ ] Review package imports with: `go -C backend list -f '{{.ImportPath}} {{range .Imports}}{{.}} {{end}}' ./internal/...`
- [ ] Confirm `backend/internal/api` no longer imports both transport concerns and broad business orchestration helpers from one monolith file.
- [ ] Confirm `backend/internal/model` no longer contains HTTP request/response DTOs or websocket envelope types.
- [ ] Confirm no production code relies on `strings.Contains(err.Error(), ...)` for control flow.

## Execution Notes

- Preserve current external API and websocket payloads during the first governance pass.
- Prefer extraction and relocation over big-bang rewrites.
- Keep each chunk independently mergeable.
- Every chunk must leave the backend in a releasable state with tests green.
