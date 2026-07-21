$ErrorActionPreference = 'Stop'

$repoRoot = Split-Path -Parent $PSScriptRoot
$toolBin = Join-Path $repoRoot '.tools\bin'
$goCache = Join-Path $repoRoot '.tools\go-cache'
$uvCache = Join-Path $repoRoot 'deepagent\.uv-cache'

New-Item -ItemType Directory -Force -Path $toolBin | Out-Null
New-Item -ItemType Directory -Force -Path $goCache | Out-Null
New-Item -ItemType Directory -Force -Path $uvCache | Out-Null

$env:GOBIN = $toolBin
$env:GOCACHE = $goCache
$env:UV_CACHE_DIR = $uvCache
$env:PATH = "$toolBin;$env:PATH"

go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.6
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1

uv run --project (Join-Path $repoRoot 'deepagent') python -m grpc_tools.protoc `
  --proto_path (Join-Path $repoRoot 'proto') `
  --python_out (Join-Path $repoRoot 'deepagent\src') `
  --pyi_out (Join-Path $repoRoot 'deepagent\src') `
  --grpc_python_out (Join-Path $repoRoot 'deepagent\src') `
  --go_out (Join-Path $repoRoot 'backend') `
  --go_opt module=agentroom/backend `
  --go-grpc_out (Join-Path $repoRoot 'backend') `
  --go-grpc_opt module=agentroom/backend `
  (Join-Path $repoRoot 'proto\agent_runtime\v1\agent_runtime.proto')
