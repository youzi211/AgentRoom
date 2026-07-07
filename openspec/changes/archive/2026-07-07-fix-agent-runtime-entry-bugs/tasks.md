## 1. Regression Coverage

- [x] 1.1 Add a backend agent-runner test proving guided dialogue persists runtime artifacts from an `AgentRuntimeResponse`.
- [x] 1.2 Add a backend runtime test proving DeepAgent argv includes `--` before question text and handles option-like questions as positional input.
- [x] 1.3 Add a backend runtime concurrency test proving calls beyond the limit wait and canceled waiters do not start a subprocess.
- [x] 1.4 Add API/service tests for a public recent active room summary route and for keeping the admin room listing protected.
- [x] 1.5 Add or update frontend tests proving the entry page calls the public recent-room endpoint and renders fields present in that response.

## 2. Runtime And Dialogue Fixes

- [x] 2.1 Change guided dialogue response plumbing to preserve `AgentRuntimeResponse` instead of returning only response content.
- [x] 2.2 Apply `messageArtifactsFromRuntime` when creating guided dialogue agent messages.
- [x] 2.3 Insert `--` before the DeepAgent question argument in the Go subprocess argv.
- [x] 2.4 Add a bounded concurrency gate to `DeepAgentRuntime` that observes context cancellation while waiting.
- [x] 2.5 Wire the DeepAgent concurrency limit through existing runtime configuration if a suitable config path exists; otherwise keep a conservative default.

## 3. Recent Room Discovery

- [x] 3.1 Define a public recent room summary contract with only safe entry-page fields.
- [x] 3.2 Implement a non-admin backend route for recent active room summaries while leaving `GET /api/rooms` admin-gated.
- [x] 3.3 Populate dialogue mode, agent count, status, room ID, name, created time, and passcode status in the public summary.
- [x] 3.4 Update `roomClient.js` to call the new public route for entry-page recent rooms.
- [x] 3.5 Update `JoinScreen.jsx` to render only fields provided by the public summary contract.

## 4. Verification

- [x] 4.1 Run `go -C backend test ./...`.
- [x] 4.2 Run `go -C backend vet ./...`.
- [x] 4.3 Run relevant frontend node tests, including entry/recent-room and artifact rendering tests.
- [x] 4.4 Run `npm --prefix frontend run build`.
- [x] 4.5 Run `openspec validate fix-agent-runtime-entry-bugs` or the equivalent validation command for this OpenSpec checkout.
