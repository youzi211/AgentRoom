## Context

AgentRoom now has a pluggable `AgentRuntime` path, including a DeepAgent runtime that can return downloadable Markdown artifacts. The mention fanout path persists these artifacts, but guided dialogue currently flattens runtime responses to plain text. The redesigned entry page also shows recent rooms, but it calls the admin room listing API and reads fields that the existing `RoomSummary` contract does not provide.

The DeepAgent runtime shells out to `uv run deepagent-research`. This keeps the Python prototype isolated, but the Go side must treat subprocess execution as an operational boundary: user questions must not be parsed as CLI flags, and multiple agent responses must not create unbounded local subprocess fan-out.

## Goals / Non-Goals

**Goals:**
- Preserve message artifacts from every runtime-supported agent reply path.
- Let the entry page display recent active rooms without requiring admin credentials.
- Keep admin room management separate from public room discovery.
- Make DeepAgent CLI invocation safe for user-controlled question text.
- Add a configurable or conservative in-process concurrency limit for DeepAgent subprocesses.
- Cover each regression with focused tests.

**Non-Goals:**
- Replacing the DeepAgent subprocess adapter with an embedded Python service.
- Redesigning room permissions, passcode behavior, or admin authentication.
- Backfilling legacy messages that were persisted before artifact metadata existed.
- Adding a general distributed job queue.

## Decisions

### Preserve full runtime responses in guided dialogue

Change guided dialogue helpers to carry `AgentRuntimeResponse` instead of only `Content`. The final message creation step should mirror mention fanout by assigning `KnowledgeSources` and `Artifacts` before persistence.

Alternative considered: keep returning a string and re-run artifact lookup later. That would duplicate runtime-specific knowledge and risks reconstructing artifacts from side effects instead of using the runtime contract.

### Add a public recent-room summary path

Introduce a safe listing API for recent active rooms, or split the existing service method into admin and public projections. The public projection should only expose fields the entry page needs to render and join: room ID, name, status, created time, dialogue mode, agent count, and whether a passcode exists. It must not expose admin-only lifecycle operations or sensitive internal fields.

Alternative considered: make the entry page silently use the admin key if present and hide the panel otherwise. That avoids a backend change but leaves the normal user entry page degraded and does not fix the field mismatch.

### Keep admin listing admin-only

The existing `GET /api/rooms` route is used for administration and should remain protected when `ADMIN_API_KEY` is configured. Public discovery should use a distinct route or an explicit non-admin handler so future admin fields do not leak by accident.

### Separate CLI options from question text

Append `--` before the DeepAgent question argument when building the command. The Python CLI already accepts a positional `question`; `--` is the standard argv boundary that prevents question text beginning with `--` from being parsed as an option.

Alternative considered: reject questions beginning with `-`. That would be user-hostile and still leaves the command boundary less explicit.

### Limit DeepAgent subprocess concurrency in the runtime adapter

Add a small concurrency gate around `cmd.Run()`, defaulting to one or another conservative value. The gate should be local to the runtime registry/config created by the backend process. If the project already has a config env pattern, expose the limit through that path; otherwise keep the default internal and testable.

Alternative considered: rely only on dialogue turn limits. That does not protect concurrent rooms or multiple simultaneous HTTP/WebSocket triggers.

## Risks / Trade-offs

- Public recent-room discovery may reveal active room IDs → expose only active rooms intentionally shown on the entry page, keep passcode protection for joining and reading room details, and avoid participant/message metadata.
- A low DeepAgent concurrency limit can delay responses → prefer predictable queuing over local process exhaustion, and keep the limit configurable if config plumbing is already available.
- Guided dialogue response type changes can touch tests and duplicate mention fanout logic → factor the message-building behavior narrowly, without a broad runner refactor.
- Existing legacy artifacts remain unavailable → document as out of scope for this bugfix and keep the new path correct going forward.

## Migration Plan

1. Add tests that reproduce the four defects.
2. Implement backend runtime/dialogue/API changes.
3. Update frontend entry page API usage and render fields to match the new public summary contract.
4. Run Go tests, `go vet`, frontend targeted tests, and `npm --prefix frontend run build`.
5. Rollback is a code revert; no schema migration should be required unless the chosen summary query needs existing room metadata already present in storage.
