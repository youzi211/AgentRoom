# Repository Guidelines

## Project Structure & Module Organization
- `backend/cmd/server/`: Go service entrypoint.
- `backend/internal/`: backend runtime code split by concern: `api`, `agent`, `llm`, `room`, `service`, `store`, and `config`.
- `backend/internal/tests/`: consolidated Go test suites. Add new backend tests under the matching area, for example `backend/internal/tests/service/`.
- `backend/internal/tests/teststore/`: shared in-memory test doubles for backend tests.
- `frontend/src/`: React + Vite app. UI lives in `components/`, API helpers in `api/`, shared routing in `routing.js`.
- `docs/`: architecture and product notes. Update these when behavior or deployment assumptions change.

## Build, Test, and Development Commands
- `go -C backend run ./cmd/server`: start the backend locally.
- `go -C backend test ./...`: run all Go tests.
- `go -C backend vet ./...`: run static checks on backend packages.
- `go -C backend build ./cmd/server`: verify the backend builds cleanly.
- `npm --prefix frontend install`: install frontend dependencies.
- `npm --prefix frontend run dev`: start the Vite dev server.
- `npm --prefix frontend run build`: produce a production frontend build.
- Example targeted frontend test: `node --test frontend/src/components/meetingMinutes.test.mjs`

## Coding Style & Naming Conventions
- Go code should stay `gofmt`-clean and follow standard Go naming: exported `PascalCase`, internal helpers `camelCase`.
- React components use `PascalCase` filenames such as `ChatRoom.jsx`; utility modules use `camelCase`.
- Match the existing frontend style: 2-space indentation, single quotes, and no semicolons unless required.
- Prefer small, package-local changes over new abstractions or dependencies.

## Testing Guidelines
- Backend tests use Go's `testing` package and should live under `backend/internal/tests/**` with `*_test.go` names.
- Favor black-box tests through exported APIs rather than exposing internals just for test access.
- Frontend tests use Node's built-in test runner with `*.test.mjs` files near the component or helper they cover.
- For changes touching both layers, run backend tests plus the relevant frontend tests and `npm --prefix frontend run build`.

## Commit & Pull Request Guidelines
- Recent history uses short, intent-first subjects, for example `Capture the next dialogue layer...` and `Decouple AgentRoom from the raw OpenAI SDK...`.
- Keep commit titles concise and explain why the change exists, not just what moved.
- PRs should include: scope summary, affected paths, verification commands run, and screenshots for UI changes.
- Call out schema, env, or deployment changes explicitly so reviewers can test safely.

## Security & Configuration Tips
- Copy `.env.example` to `.env` for local setup and never commit secrets.
- In non-local environments, set `ADMIN_API_KEY`, `VITE_ADMIN_API_KEY`, and `ALLOWED_ORIGINS`.
- If you change MySQL, migrations, or LLM configuration, update `README.md` and any affected docs in `docs/`.
