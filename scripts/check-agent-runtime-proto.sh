#!/usr/bin/env sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
before=$(mktemp)
after=$(mktemp)
trap 'rm -f "$before" "$after"' EXIT

find "$repo_root/backend/internal/agentproto/v1" "$repo_root/deepagent/src/agent_runtime/v1" \
  -type f ! -name '__init__.py' \( -name '*.pb.go' -o -name '*.py' -o -name '*.pyi' \) \
  -exec sha256sum {} \; | sort > "$before"

"$repo_root/scripts/generate-agent-runtime-proto.sh"

find "$repo_root/backend/internal/agentproto/v1" "$repo_root/deepagent/src/agent_runtime/v1" \
  -type f ! -name '__init__.py' \( -name '*.pb.go' -o -name '*.py' -o -name '*.pyi' \) \
  -exec sha256sum {} \; | sort > "$after"

diff -u "$before" "$after"
