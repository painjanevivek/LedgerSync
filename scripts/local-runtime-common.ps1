Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$script:LedgerSyncRepositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$script:LedgerSyncComposeFile = Join-Path $script:LedgerSyncRepositoryRoot "deploy\compose\docker-compose.yml"
$script:LedgerSyncWebUrl = "http://localhost:3000"
$script:LedgerSyncLongRunningServices = @("postgres", "redis", "api", "outbox-worker", "web")
$script:LedgerSyncOneShotServices = @("migrate", "demo-seed")

$requestedProject = [Environment]::GetEnvironmentVariable("LEDGERSYNC_LOCAL_COMPOSE_PROJECT")
if ([string]::IsNullOrWhiteSpace($requestedProject)) {
    $requestedProject = "compose"
}
if ($requestedProject -cnotmatch '^[a-z0-9][a-z0-9_-]{0,62}$') {
    throw "LEDGERSYNC_LOCAL_COMPOSE_PROJECT must contain only lowercase letters, digits, underscores, or hyphens."
}
$script:LedgerSyncComposeProject = $requestedProject

function Assert-LedgerSyncDockerAvailable {
    if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
        throw "Docker was not found. Install or start Docker Desktop, then try again."
    }

    & docker info --format '{{json .ServerVersion}}' *> $null
    if ($LASTEXITCODE -ne 0) {
        throw "Docker Desktop is not ready. Start it and wait until the engine reports healthy."
    }
}

function Invoke-LedgerSyncCompose {
    param(
        [Parameter(Mandatory = $true)]
        [string[]]$ComposeArguments,
        [switch]$CaptureOutput
    )

    $baseArguments = @(
        "compose",
        "-p", $script:LedgerSyncComposeProject,
        "-f", $script:LedgerSyncComposeFile
    )

    if ($CaptureOutput) {
        $result = & docker @baseArguments @ComposeArguments 2>&1
        if ($LASTEXITCODE -ne 0) {
            throw "Docker Compose command failed: docker $($baseArguments -join ' ') $($ComposeArguments -join ' ')`n$($result -join [Environment]::NewLine)"
        }
        return $result
    }

    & docker @baseArguments @ComposeArguments
    if ($LASTEXITCODE -ne 0) {
        throw "Docker Compose command failed: docker $($baseArguments -join ' ') $($ComposeArguments -join ' ')"
    }
}

function Get-LedgerSyncComposeRows {
    $raw = @(Invoke-LedgerSyncCompose -ComposeArguments @("ps", "--all", "--format", "json") -CaptureOutput)
    $rows = @()
    foreach ($line in $raw) {
        if (-not [string]::IsNullOrWhiteSpace([string]$line)) {
            $rows += ([string]$line | ConvertFrom-Json)
        }
    }
    return $rows
}

function Get-LedgerSyncServiceRow {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Service,
        [object[]]$Rows = (Get-LedgerSyncComposeRows)
    )

    return @($Rows | Where-Object { $_.Service -eq $Service } | Select-Object -First 1)
}

function Assert-LedgerSyncOneShotServicesCompleted {
    $rows = @(Get-LedgerSyncComposeRows)
    foreach ($service in $script:LedgerSyncOneShotServices) {
        $row = @(Get-LedgerSyncServiceRow -Service $service -Rows $rows)
        if ($row.Count -ne 1 -or $row[0].State -ne "exited" -or [int]$row[0].ExitCode -ne 0) {
            throw "The $service setup step did not complete successfully. Run scripts/logs-local.ps1 -Service $service for bounded diagnostics."
        }
    }
}

function Assert-LedgerSyncLongRunningServicesHealthy {
    $rows = @(Get-LedgerSyncComposeRows)
    foreach ($service in $script:LedgerSyncLongRunningServices) {
        $row = @(Get-LedgerSyncServiceRow -Service $service -Rows $rows)
        if ($row.Count -ne 1 -or $row[0].State -ne "running") {
            throw "The $service service is not running. Run scripts/status-local.ps1 and bounded service logs for details."
        }
        if ($row[0].Health -and $row[0].Health -ne "healthy") {
            throw "The $service service is running but not healthy (health=$($row[0].Health))."
        }
    }
}

function Test-LedgerSyncPortAvailableOrOwned {
    $listening = $false
    $client = [System.Net.Sockets.TcpClient]::new()
    try {
        $connect = $client.ConnectAsync("127.0.0.1", 3000)
        $listening = $connect.Wait(500) -and $client.Connected
    }
    catch {
        $listening = $false
    }
    finally {
        $client.Dispose()
    }

    if (-not $listening) {
        return
    }

    $owners = @(& docker ps --filter "publish=3000" --format '{{.Label "com.docker.compose.project"}}|{{.Label "com.docker.compose.service"}}' 2>$null)
    $expected = "$($script:LedgerSyncComposeProject)|web"
    if ($LASTEXITCODE -ne 0 -or $owners -notcontains $expected) {
        throw "Port 3000 is already in use by another process. LedgerSync will not stop or replace that process."
    }
}

function Invoke-LedgerSyncWebSmoke {
    param([int]$TimeoutSeconds = 15)

    $session = [Microsoft.PowerShell.Commands.WebRequestSession]::new()
    $targets = @(
        "/api/session",
        "/api/me/accounts?limit=1",
        "/api/reconciliation/runs?limit=1"
    )

    foreach ($target in $targets) {
        $response = Invoke-WebRequest -UseBasicParsing -WebSession $session -Uri "$($script:LedgerSyncWebUrl)$target" -TimeoutSec $TimeoutSeconds
        if ([int]$response.StatusCode -ne 200) {
            throw "Local smoke request failed for $target with HTTP $($response.StatusCode)."
        }
    }
}

function Get-LedgerSyncOperationalSummary {
    $sql = "SELECT json_build_object('outbox_pending',(SELECT count(*) FROM outbox_events WHERE published_at IS NULL AND dead_at IS NULL),'outbox_dead',(SELECT count(*) FROM outbox_events WHERE dead_at IS NOT NULL),'reconciliation_status',COALESCE((SELECT status FROM reconciliation_runs ORDER BY started_at DESC LIMIT 1),'not_run'),'reconciliation_mismatches',COALESCE((SELECT mismatch_count FROM reconciliation_runs ORDER BY started_at DESC LIMIT 1),0));"
    $result = @(Invoke-LedgerSyncCompose -ComposeArguments @("exec", "-T", "postgres", "psql", "-U", "ledgersync", "-d", "ledgersync", "-Atc", $sql) -CaptureOutput)
    $json = @($result | Where-Object { ([string]$_).TrimStart().StartsWith("{") } | Select-Object -Last 1)
    if ($json.Count -ne 1) {
        throw "Operational summary did not return a bounded result."
    }
    return ([string]$json[0] | ConvertFrom-Json)
}

function ConvertTo-LedgerSyncRedactedLogLine {
    param([Parameter(Mandatory = $true)][string]$Line)

    $redacted = $Line -replace '(?i)(authorization:\s*bearer\s+)[^\s]+', '$1[REDACTED]'
    $redacted = $redacted -replace '(?i)(postgres(?:ql)?://[^:\s]+:)[^@\s]+(@)', '$1[REDACTED]$2'
    $redacted = $redacted -replace '(?i)(password|secret|token|signing_key)(["''=: ]+)([^,\s}"'']+)', '$1$2[REDACTED]'
    return $redacted
}
