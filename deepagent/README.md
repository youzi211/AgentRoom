# AgentRoom DeepAgent Prototype

This `uv` project provides AgentRoom's long-running Python Agent Runtime. It
hosts ordinary LLM and DeepAgent Executors behind a server-streaming gRPC
service. The research CLI remains available for standalone smoke tests and
migration rollback; remote Go turns do not launch it per request.

The first prototype focuses on one capability:

- take a research question;
- search the public web with Tavily;
- run a DeepAgents research agent;
- save the final Markdown report and event log under `runs/`.

Model connections are normally managed as encrypted Profiles on AgentRoom's
Models admin page. DeepAgent-specific search settings, Tavily credentials,
output paths, and limits remain local runtime concerns.

## Setup

```powershell
cd deepagent
uv sync
Copy-Item .env.example .env
```

For a standalone live run, put the search credential and model configuration in
`.env`:

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
Non-empty process environment variables override `.env` and TOML, while empty
process variables do not hide file values. This precedence applies to the
standalone CLI only. Managed gRPC turns receive a request-scoped model
connection from Go and do not write it into process environment files.

`deepagent.toml` is now for non-secret defaults such as search limits and output
directory. Legacy `CUSTOM_*` and provider-specific keys like `OPENAI_API_KEY`
still work for standalone/migration use, but `MODEL_*` is the preferred
environment contract.

## AgentRoom-managed model configuration

Create Profiles in the main application's Models admin page under either the
`go` or `deepagent` runtime scope. A DeepAgent can bind a compatible Profile or
inherit the DeepAgent default. Model resolution follows this order:

1. the concrete Profile ID snapshotted into the room Agent;
2. the enabled database default for the `deepagent` scope when no snapshot ID
   exists;
3. root `MODEL_BASE_URL`, `MODEL_NAME`, and `MODEL_API_KEY` only as a migration
   fallback when no database default exists;
4. a model-not-configured error.

An explicit Profile that is missing, disabled, incompatible, or undecryptable
fails without falling through to another Profile or environment credential.
Environment fallbacks are not imported into MySQL or copied into this
project's files.

For a managed run, the Go backend decrypts the selected Profile in memory and
sends only these values plus safe audit identifiers in the protected gRPC
request:

```text
MODEL_PROTOCOL=openai
MODEL_BASE_URL=...
MODEL_NAME=...
MODEL_API_KEY=...
```

The values are not passed in command-line arguments and are not written to
`.env`, `deepagent.toml`, `runs/*/report.md`, or `runs/*/events.jsonl`. API
responses expose only masked key state. The service-side
`MODEL_CONFIG_ENCRYPTION_KEY` never enters the Python process; it remains in the
Go backend and must be backed up separately. Losing or changing that master key
makes API keys already encrypted in MySQL undecryptable, requiring operators to
enter replacement Profile keys.

`TAVILY_API_KEY` is not part of a model Profile. Live web research still needs
it in the DeepAgent runtime environment (or, for standalone development, the
local `.env`). Keep it out of reports, events, and source control.

## Run the gRPC service

Local plaintext must be explicit:

```powershell
$env:AGENT_RUNTIME_INSECURE='true'
uv run agent-runtime
```

The service defaults to `127.0.0.1:50051`. Non-local deployments should set
certificate/key/CA paths, keep insecure mode disabled, and configure the Go
client's CA and server name. Standard `grpc.health.v1.Health` reports readiness.

## Docker and production

`deepagent/Dockerfile` builds the dedicated non-root Runtime image from locked
dependencies. Compose configures an internal-only service and does not publish
port 50051 to the host:

```text
AGENT_RUNTIME_HOST=0.0.0.0
AGENT_RUNTIME_PORT=50051
AGENT_RUNTIME_INSECURE=true
AGENT_RUNTIME_WORK_DIR=/app/runs
```

Temporary run workspaces use the `agent-runtime-runs` volume and are cleaned at
the end of each call after inline artifacts are read. A live research run still
requires `TAVILY_API_KEY`; model credentials arrive only in request memory.

For a non-Docker host deployment, install Python 3.11+ and `uv`, run `uv sync`
in this directory, make the configured work directory writable by the backend
service account, and set `DEEPAGENT_WORKDIR` to its absolute path.

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

The automated tests do not call a real LLM or Tavily. They cover the gRPC
contract, health, TLS configuration, LLM/DeepAgent execution, capacity,
cancellation, redaction, CLI compatibility, and report persistence.
