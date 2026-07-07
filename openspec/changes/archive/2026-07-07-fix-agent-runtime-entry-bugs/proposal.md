## Why

Recent DeepAgent integration work introduced regressions in two user-facing paths: guided dialogue can lose downloadable research artifacts, and the entry page can call an admin-only room listing endpoint for ordinary users. The same runtime path also has avoidable operational risks around CLI argument parsing and unbounded subprocess fan-out.

## What Changes

- Preserve runtime artifacts for agent replies in both mention fanout and guided dialogue modes.
- Add a safe non-admin recent active room listing path, or otherwise adjust the entry page so ordinary users do not call admin-gated APIs.
- Align recent room summary fields between frontend expectations and backend response shape.
- Pass DeepAgent questions to the Python CLI in a way that cannot be parsed as CLI options.
- Limit concurrent DeepAgent subprocess executions to prevent uncontrolled local process fan-out.
- Add regression coverage for the fixed paths.

## Capabilities

### New Capabilities
- `agent-runtime-entry-reliability`: Reliable DeepAgent runtime execution, artifact persistence across dialogue modes, and safe entry-page room discovery.

### Modified Capabilities

## Impact

- Backend agent runtime and dialogue orchestration under `backend/internal/agent/`.
- Backend room listing API, contracts, services, and tests under `backend/internal/api/`, `backend/internal/service/`, and `backend/internal/tests/`.
- Frontend entry page and API helper behavior under `frontend/src/components/JoinScreen.jsx` and `frontend/src/api/roomClient.js`.
- DeepAgent CLI invocation boundary between Go and `deepagent/src/agentroom_deepagent/cli.py`.
- Verification should include Go tests, frontend targeted tests, frontend build, and `go vet`.
