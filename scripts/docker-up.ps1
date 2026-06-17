[CmdletBinding()]
param(
  [switch]$NoBuild,
  [int]$BackendTimeoutSeconds = 120,
  [int]$FrontendTimeoutSeconds = 120
)

$ErrorActionPreference = 'Stop'

function Write-Step {
  param([string]$Message)
  Write-Host "==> $Message" -ForegroundColor Cyan
}

function Write-Note {
  param([string]$Message)
  Write-Host $Message -ForegroundColor Yellow
}

function New-RandomToken {
  param([int]$Length = 48)

  $alphabet = 'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789'
  $bytes = New-Object byte[] $Length
  $rng = [System.Security.Cryptography.RandomNumberGenerator]::Create()
  try {
    $rng.GetBytes($bytes)
  } finally {
    if ($null -ne $rng) {
      $rng.Dispose()
    }
  }

  $builder = New-Object System.Text.StringBuilder
  foreach ($byte in $bytes) {
    [void]$builder.Append($alphabet[$byte % $alphabet.Length])
  }

  return $builder.ToString()
}

function Get-EnvValue {
  param(
    [string[]]$Lines,
    [string]$Key
  )

  $pattern = "^\s*$([regex]::Escape($Key))=(.*)$"
  foreach ($line in $Lines) {
    if ($line -match $pattern) {
      return $Matches[1]
    }
  }

  return ''
}

function Set-EnvValue {
  param(
    [string[]]$Lines,
    [string]$Key,
    [string]$Value
  )

  $pattern = "^\s*$([regex]::Escape($Key))="
  $updated = @()
  $replaced = $false

  foreach ($line in $Lines) {
    if ($line -match $pattern) {
      $updated += "${Key}=${Value}"
      $replaced = $true
    } else {
      $updated += $line
    }
  }

  if (-not $replaced) {
    $updated += "${Key}=${Value}"
  }

  return ,$updated
}

function Needs-Randomization {
  param(
    [string]$Value,
    [string[]]$Placeholders
  )

  if ([string]::IsNullOrWhiteSpace($Value)) {
    return $true
  }

  foreach ($placeholder in $Placeholders) {
    if ($Value -eq $placeholder) {
      return $true
    }
  }

  return $false
}

function Append-CsvUnique {
  param(
    [string]$Csv,
    [string]$Item
  )

  $safeItem = if ($null -eq $Item) { '' } else { $Item }
  $safeCsv = if ($null -eq $Csv) { '' } else { $Csv }
  $trimmedItem = $safeItem.Trim()
  if ([string]::IsNullOrWhiteSpace($trimmedItem)) {
    return $safeCsv.Trim()
  }

  $result = New-Object System.Collections.Generic.List[string]
  $found = $false

  foreach ($entry in ($safeCsv -split ',')) {
    $trimmedEntry = $entry.Trim()
    if ([string]::IsNullOrWhiteSpace($trimmedEntry)) {
      continue
    }
    if ($trimmedEntry -eq $trimmedItem) {
      $found = $true
    }
    $result.Add($trimmedEntry)
  }

  if (-not $found) {
    $result.Add($trimmedItem)
  }

  return ($result -join ',')
}

function Append-CsvListUnique {
  param(
    [string]$Csv,
    [string]$List
  )

  $result = if ($null -eq $Csv) { '' } else { $Csv.Trim() }
  $listValue = if ($null -eq $List) { '' } else { $List }
  foreach ($entry in ($listValue -split ',')) {
    $result = Append-CsvUnique -Csv $result -Item $entry
  }

  return $result
}

function Wait-ForBackendHealth {
  param(
    [int]$TimeoutSeconds,
    [string]$HealthUrl
  )

  $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
  while ((Get-Date) -lt $deadline) {
    try {
      $response = Invoke-RestMethod -Uri $HealthUrl -Method Get -TimeoutSec 5
      if ($response.ok -eq $true -and $response.database.ok -eq $true) {
        return
      }
    } catch {
    }

    Start-Sleep -Seconds 2
  }

  throw "Backend health check never passed at $HealthUrl within $TimeoutSeconds seconds."
}

function Wait-ForFrontend {
  param(
    [int]$TimeoutSeconds,
    [string]$FrontendUrl
  )

  $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
  while ((Get-Date) -lt $deadline) {
    try {
      $response = Invoke-WebRequest -Uri $FrontendUrl -TimeoutSec 5 -UseBasicParsing
      if ($response.StatusCode -ge 200 -and $response.StatusCode -lt 500) {
        return
      }
    } catch {
    }

    Start-Sleep -Seconds 2
  }

  throw "Frontend did not answer at $FrontendUrl within $TimeoutSeconds seconds."
}

function Invoke-CheckedCommand {
  param(
    [string]$FailureMessage,
    [string[]]$Arguments,
    [switch]$Quiet
  )

  $stdoutFile = [System.IO.Path]::GetTempFileName()
  $stderrFile = [System.IO.Path]::GetTempFileName()

  try {
    $process = Start-Process -FilePath 'docker' -ArgumentList $Arguments -NoNewWindow -Wait -PassThru -RedirectStandardOutput $stdoutFile -RedirectStandardError $stderrFile
    $stdout = Get-Content $stdoutFile -ErrorAction SilentlyContinue
    $stderr = Get-Content $stderrFile -ErrorAction SilentlyContinue

    if (-not $Quiet -or $process.ExitCode -ne 0) {
      if ($stdout) {
        $stdout | ForEach-Object { Write-Host $_ }
      }
      if ($stderr) {
        $stderr | ForEach-Object { Write-Host $_ }
      }
    }

    if ($process.ExitCode -ne 0) {
      throw $FailureMessage
    }
  } finally {
    Remove-Item $stdoutFile, $stderrFile -ErrorAction SilentlyContinue
  }
}

function Invoke-CapturedCommand {
  param(
    [string]$FailureMessage,
    [string[]]$Arguments
  )

  $stdoutFile = [System.IO.Path]::GetTempFileName()
  $stderrFile = [System.IO.Path]::GetTempFileName()

  try {
    $process = Start-Process -FilePath 'docker' -ArgumentList $Arguments -NoNewWindow -Wait -PassThru -RedirectStandardOutput $stdoutFile -RedirectStandardError $stderrFile
    $stdout = Get-Content $stdoutFile -ErrorAction SilentlyContinue
    $stderr = Get-Content $stderrFile -ErrorAction SilentlyContinue

    if ($process.ExitCode -ne 0) {
      if ($stdout) {
        $stdout | ForEach-Object { Write-Host $_ }
      }
      if ($stderr) {
        $stderr | ForEach-Object { Write-Host $_ }
      }
      throw $FailureMessage
    }

    $stdoutLines = if ($null -eq $stdout) { @() } else { @($stdout) }
    return ($stdoutLines -join [Environment]::NewLine).Trim()
  } finally {
    Remove-Item $stdoutFile, $stderrFile -ErrorAction SilentlyContinue
  }
}

function Parse-PublishedPort {
  param(
    [string]$Mapping,
    [string]$Service,
    [string]$ContainerPort
  )

  $mapping = (($Mapping -split "`r?`n") | Where-Object { -not [string]::IsNullOrWhiteSpace($_) } | Select-Object -First 1).Trim()
  if ([string]::IsNullOrWhiteSpace($mapping)) {
    throw "Could not determine the published port for ${Service}:${ContainerPort}."
  }

  $lastColonIndex = $mapping.LastIndexOf(':')
  if ($lastColonIndex -lt 0) {
    throw "Could not parse the published port for ${Service}:${ContainerPort} from '$mapping'."
  }

  return $mapping.Substring($lastColonIndex + 1)
}

function Get-BackendPublishedPort {
  $mapping = Invoke-CapturedCommand -FailureMessage 'Could not determine the published port for backend:8080.' -Arguments @('compose', 'port', 'backend', '8080')
  return Parse-PublishedPort -Mapping $mapping -Service 'backend' -ContainerPort '8080'
}

function Get-FrontendPublishedPort {
  $mapping = Invoke-CapturedCommand -FailureMessage 'Could not determine the published port for frontend:80.' -Arguments @('compose', 'port', 'frontend', '80')
  return Parse-PublishedPort -Mapping $mapping -Service 'frontend' -ContainerPort '80'
}

try {
  $scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
  $repoRoot = (Resolve-Path (Join-Path $scriptDir '..')).Path
  $envPath = Join-Path $repoRoot '.env'
  $envExamplePath = Join-Path $repoRoot '.env.example'

  Set-Location $repoRoot

  if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
    throw 'Docker CLI is not installed or is not on PATH.'
  }

  Write-Step 'Checking Docker daemon'
  Invoke-CheckedCommand -FailureMessage 'Docker daemon is unavailable. Start Docker Desktop and rerun this script.' -Arguments @('info') -Quiet

  if (-not (Test-Path $envPath)) {
    Write-Step 'Bootstrapping .env from .env.example'
    Copy-Item $envExamplePath $envPath
  }

  $envLines = Get-Content $envPath

  $adminApiKey = Get-EnvValue -Lines $envLines -Key 'ADMIN_API_KEY'
  if (Needs-Randomization -Value $adminApiKey -Placeholders @('change_me_admin_key')) {
    $adminApiKey = New-RandomToken
  }
  $envLines = Set-EnvValue -Lines $envLines -Key 'ADMIN_API_KEY' -Value $adminApiKey
  $envLines = Set-EnvValue -Lines $envLines -Key 'VITE_ADMIN_API_KEY' -Value $adminApiKey

  $mysqlPassword = Get-EnvValue -Lines $envLines -Key 'MYSQL_PASSWORD'
  if (Needs-Randomization -Value $mysqlPassword -Placeholders @('agentroom_password')) {
    $mysqlPassword = New-RandomToken
  }
  $envLines = Set-EnvValue -Lines $envLines -Key 'MYSQL_PASSWORD' -Value $mysqlPassword

  $mysqlRootPassword = Get-EnvValue -Lines $envLines -Key 'MYSQL_ROOT_PASSWORD'
  if (Needs-Randomization -Value $mysqlRootPassword -Placeholders @('change_me_root_password')) {
    $mysqlRootPassword = New-RandomToken
  }
  $envLines = Set-EnvValue -Lines $envLines -Key 'MYSQL_ROOT_PASSWORD' -Value $mysqlRootPassword

  $llmApiKey = Get-EnvValue -Lines $envLines -Key 'LLM_API_KEY'
  if ($llmApiKey -eq 'your-api-key-here') {
    $envLines = Set-EnvValue -Lines $envLines -Key 'LLM_API_KEY' -Value ''
  }

  $publicOrigins = if ([string]::IsNullOrWhiteSpace($env:PUBLIC_ORIGIN)) {
    Get-EnvValue -Lines $envLines -Key 'PUBLIC_ORIGIN'
  } else {
    $env:PUBLIC_ORIGIN
  }
  if ($null -eq $publicOrigins) {
    $publicOrigins = ''
  }
  $publicOrigins = $publicOrigins.Trim()
  if (-not [string]::IsNullOrWhiteSpace($publicOrigins)) {
    $envLines = Set-EnvValue -Lines $envLines -Key 'PUBLIC_ORIGIN' -Value $publicOrigins
  }

  $allowedOrigins = Get-EnvValue -Lines $envLines -Key 'ALLOWED_ORIGINS'
  $allowedOrigins = Append-CsvUnique -Csv $allowedOrigins -Item 'http://localhost:5173'
  $allowedOrigins = Append-CsvUnique -Csv $allowedOrigins -Item 'http://127.0.0.1:5173'
  $allowedOrigins = Append-CsvListUnique -Csv $allowedOrigins -List $publicOrigins
  $envLines = Set-EnvValue -Lines $envLines -Key 'ALLOWED_ORIGINS' -Value $allowedOrigins

  $utf8NoBom = New-Object System.Text.UTF8Encoding $false
  [System.IO.File]::WriteAllLines($envPath, $envLines, $utf8NoBom)

  Write-Step 'Validating docker compose configuration'
  Invoke-CheckedCommand -FailureMessage 'docker compose config failed. Fix the compose file or .env values and rerun.' -Arguments @('compose', 'config') -Quiet

  $composeArgs = @('compose', 'up', '-d')
  if (-not $NoBuild) {
    $composeArgs += '--build'
  }

  Write-Step 'Starting AgentRoom stack'
  Invoke-CheckedCommand -FailureMessage 'docker compose up failed. Check docker compose output above for details.' -Arguments $composeArgs

  $backendPort = Get-BackendPublishedPort
  $frontendPort = Get-FrontendPublishedPort
  $frontendUrl = "http://127.0.0.1:$frontendPort"
  $backendHealthUrl = "http://127.0.0.1:$backendPort/api/health"

  $currentAllowedOrigins = (Get-EnvValue -Lines $envLines -Key 'ALLOWED_ORIGINS').Trim()
  $updatedAllowedOrigins = Append-CsvUnique -Csv $currentAllowedOrigins -Item "http://localhost:$frontendPort"
  $updatedAllowedOrigins = Append-CsvUnique -Csv $updatedAllowedOrigins -Item "http://127.0.0.1:$frontendPort"
  if ($updatedAllowedOrigins -ne $currentAllowedOrigins) {
    Write-Step 'Refreshing backend origin allowlist'
    $envLines = Set-EnvValue -Lines $envLines -Key 'ALLOWED_ORIGINS' -Value $updatedAllowedOrigins
    [System.IO.File]::WriteAllLines($envPath, $envLines, $utf8NoBom)
    Invoke-CheckedCommand -FailureMessage 'docker compose could not refresh the backend with the updated origin allowlist.' -Arguments @('compose', 'up', '-d', '--force-recreate', '--no-deps', 'backend')
  }

  Write-Step 'Waiting for backend health'
  Wait-ForBackendHealth -TimeoutSeconds $BackendTimeoutSeconds -HealthUrl $backendHealthUrl

  Write-Step 'Waiting for frontend'
  Wait-ForFrontend -TimeoutSeconds $FrontendTimeoutSeconds -FrontendUrl $frontendUrl

  Write-Step 'Container status'
  Invoke-CheckedCommand -FailureMessage 'docker compose ps failed after startup.' -Arguments @('compose', 'ps')

  $finalLlmApiKey = Get-EnvValue -Lines (Get-Content $envPath) -Key 'LLM_API_KEY'
  if ([string]::IsNullOrWhiteSpace($finalLlmApiKey)) {
    Write-Note 'LLM_API_KEY is blank. Human chat will work, but agent replies stay disabled until you set a real key and rerun this script.'
  }

  Write-Host ''
  Write-Host 'AgentRoom is ready.' -ForegroundColor Green
  Write-Host "Frontend: http://localhost:$frontendPort"
  Write-Host "Backend health: http://localhost:$backendPort/api/health"
} catch {
  Write-Host $_.Exception.Message -ForegroundColor Red
  exit 1
}
