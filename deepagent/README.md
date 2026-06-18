# AgentRoom DeepAgent Prototype

This is an isolated `uv` project for testing DeepAgents before integrating them
into the main AgentRoom platform.

The first prototype focuses on one capability:

- take a research question;
- search the public web with Tavily;
- run a DeepAgents research agent;
- save the final Markdown report and event log under `runs/`.

It intentionally does not touch the existing Go backend, React frontend, MySQL
schema, or AgentRoom runtime.

## Setup

```powershell
cd deepagent
uv sync
Copy-Item .env.example .env
```

Put the important live-run configuration in `.env`:

```text
TAVILY_API_KEY=...
MODEL_PROTOCOL=openai
MODEL_BASE_URL=https://your-proxy.com/v1
MODEL_NAME=your-model
MODEL_API_KEY=sk-...
```

For a provider-native model, leave `MODEL_BASE_URL` empty and use a prefixed
model name:

```text
TAVILY_API_KEY=...
MODEL_BASE_URL=
MODEL_NAME=openai:gpt-5.5
MODEL_API_KEY=sk-...
```

Secrets are read from the `.env` file next to the resolved `deepagent.toml`.
Non-empty process environment variables can override `.env`, but empty process
environment variables do not hide `.env` values.

`deepagent.toml` is now for non-secret defaults such as search limits and output
directory. Legacy `CUSTOM_*` and provider-specific keys like `OPENAI_API_KEY`
still work, but `MODEL_*` is the preferred path.

## Run

```powershell
uv run deepagent-research "Research whether DeepAgents fits AgentRoom as a research runtime"
```

Output is written to:

```text
runs/<run-id>/report.md
runs/<run-id>/events.jsonl
```

## Offline Smoke Test

When provider or Tavily credentials are not available yet, verify the local
project wiring without calling DeepAgents or the public web:

```powershell
uv run deepagent-research --run-id smoke-test --offline-smoke "Research whether DeepAgents fits AgentRoom as a web research runtime"
```

This writes `runs/smoke-test/report.md` and `runs/smoke-test/events.jsonl`.
The report is explicitly labeled as an offline smoke report, not a real
research result.

## Test

```powershell
uv run pytest
```

The automated tests do not call the LLM or Tavily. They cover configuration,
CLI behavior, and local report/event persistence.
