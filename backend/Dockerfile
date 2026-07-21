FROM golang:1.23-alpine AS build

WORKDIR /src/backend

ENV GOPROXY=https://goproxy.cn,direct
ENV GOSUMDB=sum.golang.google.cn

COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/agentroom ./cmd/server

FROM python:3.12-slim

COPY --from=ghcr.io/astral-sh/uv:0.11.27 /uv /uvx /bin/

ENV UV_COMPILE_BYTECODE=1 \
    UV_LINK_MODE=copy \
    UV_NO_SYNC=1 \
    UV_PYTHON_DOWNLOADS=0

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates wget \
    && rm -rf /var/lib/apt/lists/* \
    && useradd --system --uid 10001 --no-create-home --home-dir /nonexistent agentroom

WORKDIR /app/deepagent
COPY deepagent/pyproject.toml deepagent/uv.lock ./
RUN uv sync --locked --no-dev --no-install-project
COPY deepagent/ ./
RUN uv sync --locked --no-dev --no-editable \
    && mkdir -p /app/deepagent/runs \
    && chown -R agentroom:agentroom /app/deepagent

WORKDIR /app
COPY --from=build /out/agentroom /app/agentroom

USER agentroom
EXPOSE 8080

ENTRYPOINT ["/app/agentroom"]
