#!/usr/bin/env sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
tool_bin="$repo_root/.tools/bin"

mkdir -p "$tool_bin" "$repo_root/deepagent/.uv-cache"
export GOBIN="$tool_bin"
export GOCACHE="$repo_root/.tools/go-cache"
export UV_CACHE_DIR="$repo_root/deepagent/.uv-cache"
export PATH="$tool_bin:$PATH"

go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.6
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1

uv run --project "$repo_root/deepagent" python -m grpc_tools.protoc \
  --proto_path "$repo_root/proto" \
  --python_out "$repo_root/deepagent/src" \
  --pyi_out "$repo_root/deepagent/src" \
  --grpc_python_out "$repo_root/deepagent/src" \
  --go_out "$repo_root/backend" \
  --go_opt module=agentroom/backend \
  --go-grpc_out "$repo_root/backend" \
  --go-grpc_opt module=agentroom/backend \
  "$repo_root/proto/agent_runtime/v1/agent_runtime.proto"
