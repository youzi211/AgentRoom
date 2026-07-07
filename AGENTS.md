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



## 1. Think Before Coding

**Don't assume. Don't hide confusion. Surface tradeoffs.**

Before implementing:
- State your assumptions explicitly. If uncertain, ask.
- If multiple interpretations exist, present them - don't pick silently.
- If a simpler approach exists, say so. Push back when warranted.
- If something is unclear, stop. Name what's confusing. Ask.

## 2. Simplicity First

**Minimum code that solves the problem. Nothing speculative.**

- No features beyond what was asked.
- No abstractions for single-use code.
- No "flexibility" or "configurability" that wasn't requested.
- No error handling for impossible scenarios.
- If you write 200 lines and it could be 50, rewrite it.

Ask yourself: "Would a senior engineer say this is overcomplicated?" If yes, simplify.

## 3. Surgical Changes

**Touch only what you must. Clean up only your own mess.**

When editing existing code:
- Don't "improve" adjacent code, comments, or formatting.
- Don't refactor things that aren't broken.
- Match existing style, even if you'd do it differently.
- If you notice unrelated dead code, mention it - don't delete it.

When your changes create orphans:
- Remove imports/variables/functions that YOUR changes made unused.
- Don't remove pre-existing dead code unless asked.

The test: Every changed line should trace directly to the user's request.

## 4. Goal-Driven Execution

**Define success criteria. Loop until verified.**

Transform tasks into verifiable goals:
- "Add validation" → "Write tests for invalid inputs, then make them pass"
- "Fix the bug" → "Write a test that reproduces it, then make it pass"
- "Refactor X" → "Ensure tests pass before and after"

For multi-step tasks, state a brief plan:
```
1. [Step] → verify: [check]
2. [Step] → verify: [check]
3. [Step] → verify: [check]
```

Strong success criteria let you loop independently. Weak criteria ("make it work") require constant clarification.

---

**These guidelines are working if:** fewer unnecessary changes in diffs, fewer rewrites due to overcomplication, and clarifying questions come before implementation rather than after mistakes.
