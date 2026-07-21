$ErrorActionPreference = 'Stop'

$repoRoot = Split-Path -Parent $PSScriptRoot
$generatedRoots = @(
  (Join-Path $repoRoot 'backend\internal\agentproto\v1'),
  (Join-Path $repoRoot 'deepagent\src\agent_runtime\v1')
)

function Get-GeneratedHashes {
  $result = @{}
  foreach ($root in $generatedRoots) {
    Get-ChildItem -LiteralPath $root -File | Where-Object { $_.Name -match '\.(pb\.go|py|pyi)$' -and $_.Name -ne '__init__.py' } | ForEach-Object {
      $result[$_.FullName] = (Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256).Hash
    }
  }
  return $result
}

$before = Get-GeneratedHashes
& (Join-Path $PSScriptRoot 'generate-agent-runtime-proto.ps1')
$after = Get-GeneratedHashes

if ($before.Count -ne $after.Count) {
  throw 'Agent Runtime generated file set changed. Commit regenerated files.'
}
foreach ($path in $before.Keys) {
  if (-not $after.ContainsKey($path) -or $before[$path] -ne $after[$path]) {
    throw "Agent Runtime generated code is stale: $path"
  }
}
