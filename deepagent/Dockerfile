FROM python:3.12-slim

COPY --from=ghcr.io/astral-sh/uv:0.11.27 /uv /uvx /bin/

ENV UV_COMPILE_BYTECODE=1 \
    UV_LINK_MODE=copy \
    UV_NO_SYNC=1 \
    UV_PYTHON_DOWNLOADS=0 \
    AGENT_RUNTIME_HOST=0.0.0.0 \
    AGENT_RUNTIME_PORT=50051 \
    AGENT_RUNTIME_WORK_DIR=/app/runs

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/* \
    && useradd --system --uid 10001 --no-create-home --home-dir /nonexistent agentroom

WORKDIR /app/deepagent
COPY pyproject.toml uv.lock ./
RUN uv sync --locked --no-dev --no-install-project
COPY . ./
RUN uv sync --locked --no-dev --no-editable \
    && mkdir -p /app/runs \
    && chown -R agentroom:agentroom /app/deepagent /app/runs

USER agentroom
EXPOSE 50051

HEALTHCHECK --interval=10s --timeout=5s --retries=12 CMD ["uv", "run", "python", "-m", "agent_runtime.healthcheck"]
ENTRYPOINT ["uv", "run", "agent-runtime"]
