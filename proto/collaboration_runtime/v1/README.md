# Collaboration Runtime v1 contract

`CollaborationRuntimeService.ExecuteConversation` executes one immutable,
multi-turn room collaboration. Go owns room authorization, MySQL state,
business-message commits, and WebSocket broadcasts. Python owns run-local
speaker selection, Agent execution, handoff planning, and termination.

## Version and stream rules

- `protocol_version` is required and is currently `v1`.
- Unsupported versions are rejected before `accepted` with `UNIMPLEMENTED`.
- Existing field numbers are never reused. Removed fields are reserved.
- Clients ignore unknown binary fields and continue processing known fields.
- Every event contains one run ID and a strictly increasing non-zero sequence.
- The first event is `accepted`; exactly one of `completed`, `stopped`,
  `cancelled`, or `failed` ends the stream.
- Turn-scoped events contain both `turn_id` and `agent_id`. Run-scoped and
  terminal events contain neither. `speaker_selected` identifies the turn it
  selects.

## Transport status mapping

Errors before acceptance use canonical gRPC status codes:

| Condition | Status |
| --- | --- |
| Missing or malformed fields | `INVALID_ARGUMENT` |
| Unsupported `protocol_version` | `UNIMPLEMENTED` |
| Missing service authentication | `UNAUTHENTICATED` |
| Authenticated caller not permitted | `PERMISSION_DENIED` |
| Request, queue, or hard capacity limit | `RESOURCE_EXHAUSTED` |
| Duplicate active run ID | `ALREADY_EXISTS` |
| Same room already active | `FAILED_PRECONDITION` |
| Client cancellation | `CANCELLED` |
| Deadline expired | `DEADLINE_EXCEEDED` |
| Runtime or requested Engine unavailable | `UNAVAILABLE` |
| Unexpected transport or service failure | `INTERNAL` |

Engine, model, tool, output-validation, and protocol failures after acceptance
use the unique terminal event and stable `CollaborationErrorCode`. Neither peer
parses free-form exception text to decide run state. Cancellation and deadlines
remain active during queueing, selection, model/tool work, event delivery, and
checkpoint generation.

## Size limits

Both peers apply the lower of deployment limits and request execution limits.
Initial defaults are:

- serialized `ExecuteConversationRequest`: 8 MiB;
- serialized `CollaborationEvent`: 4 MiB;
- one inline artifact: 2 MiB;
- one opaque checkpoint payload: 1 MiB.

Limits are checked before model execution where possible. Content is never
silently truncated.

## Checkpoints and sensitive data

Checkpoints are optional opaque bytes with an engine, engine version, format
version, SHA-256 digest, and exact payload size. A missing, oversized, damaged,
or incompatible checkpoint is discarded and the Engine rebuilds from the
authoritative request snapshot.

Model references contain no credentials or Provider client configuration.
Logs, events, errors, and checkpoints must not expose API keys, Authorization
metadata, full prompts, raw Provider responses, or framework-internal state.
