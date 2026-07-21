# Agent Runtime v1 contract

`AgentRuntimeService.ExecuteAgent` executes exactly one Agent turn. Go owns
Agent selection, dialogue continuation, MySQL state, and WebSocket broadcasts;
Python owns prompt rendering, model/tool execution, and artifact production.

## Version and compatibility

- `protocol_version` is required and is currently `v1`.
- An unsupported version is rejected before `accepted` with gRPC
  `UNIMPLEMENTED`.
- Existing field numbers are never reused. Removed fields must be declared
  `reserved` in the containing message.
- New fields must be optional under proto3 semantics. Clients ignore unknown
  fields and continue processing known fields.
- A stream contains exactly one `run_id`, strictly increasing non-zero
  `sequence` values, an initial `accepted`, and exactly one terminal
  `completed` or `failed` event.

## Transport status mapping

Errors before the run is accepted use canonical gRPC status codes:

| Condition | Status |
| --- | --- |
| Missing or malformed fields | `INVALID_ARGUMENT` |
| Unsupported `protocol_version` | `UNIMPLEMENTED` |
| Missing service authentication | `UNAUTHENTICATED` |
| Authenticated caller not permitted | `PERMISSION_DENIED` |
| Hard capacity or payload limit | `RESOURCE_EXHAUSTED` |
| Client cancellation | `CANCELLED` |
| Deadline expired | `DEADLINE_EXCEEDED` |
| Runtime unavailable before acceptance | `UNAVAILABLE` |
| Unexpected transport/service failure | `INTERNAL` |

Model, tool, and output-validation failures after acceptance use a terminal
`failed` event with `RunErrorCode`. Go must not parse free-form Python exception
text to decide run state.

## Size limits

Both peers configure explicit gRPC and application limits. Initial defaults:

- serialized `ExecuteAgentRequest`: 8 MiB;
- serialized `AgentEvent`: 4 MiB;
- one inline artifact: 2 MiB;
- output text: 1 MiB.

Exceeding a limit returns `RESOURCE_EXHAUSTED`; content is never silently
truncated. External artifact storage is not part of v1, so `external_uri`
remains empty until that capability is specified.

## Sensitive fields

`ModelConnection.api_key` is sensitive. Request bodies, gRPC metadata,
credentials, full prompts, and raw provider errors must not be logged. Go sends
only the model connection needed for this run; Python keeps it in the run-local
context and releases it on success, failure, cancellation, or timeout. Plaintext
transport is allowed only in explicitly enabled local development and must not
listen on an uncontrolled external interface.
