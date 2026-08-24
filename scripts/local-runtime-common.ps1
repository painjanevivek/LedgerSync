Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$script:LedgerSyncRepositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$script:LedgerSyncComposeFile = Join-Path $script:LedgerSyncRepositoryRoot "deploy\compose\docker-compose.yml"
$script:LedgerSyncRuntimeStateDirectory = Join-Path $script:LedgerSyncRepositoryRoot "data\local-runtime"
$script:LedgerSyncRuntimeEnvironmentFile = Join-Path $script:LedgerSyncRuntimeStateDirectory "runtime.env"
$script:LedgerSyncWebUrl = "http://127.0.0.1:3000"
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

function New-LedgerSyncLocalSecret {
    $bytes = New-Object byte[] 32
    [Security.Cryptography.RandomNumberGenerator]::Fill($bytes)
    return -join ($bytes | ForEach-Object { $_.ToString("x2") })
}

function Test-LedgerSyncRuntimeEnvironmentFile {
    param([Parameter(Mandatory = $true)][string]$Path)

    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) {
        return $false
    }
    $required = @(
        "POSTGRES_PASSWORD",
        "LEDGERSYNC_SESSION_SECRET",
        "LEDGERSYNC_CONSISTENCY_SIGNING_KEY",
        "LEDGERSYNC_BFF_ASSERTION_SECRET",
        "LEDGERSYNC_WEB_SESSION_SECRET",
        "LEDGERSYNC_DEVELOPMENT_API_TOKEN"
    )
    $values = @{}
    foreach ($line in Get-Content -LiteralPath $Path) {
        if ($line -match '^([A-Z0-9_]+)=([a-f0-9]{64})$') {
            $values[$Matches[1]] = $Matches[2]
        }
    }
    return @($required | Where-Object { -not $values.ContainsKey($_) }).Count -eq 0
}

function Get-LedgerSyncRuntimeEnvironmentValue {
    param(
        [Parameter(Mandatory = $true)][string]$Path,
        [Parameter(Mandatory = $true)][string]$Name
    )
    foreach ($line in Get-Content -LiteralPath $Path) {
        if ($line.StartsWith("$Name=", [StringComparison]::Ordinal)) {
            return $line.Substring($Name.Length + 1)
        }
    }
    throw "Local runtime secret state is incomplete. Delete only data/local-runtime/runtime.env, then run scripts/start-local.ps1 again."
}

function Protect-LedgerSyncLocalSecretFile {
    param([Parameter(Mandatory = $true)][string]$Path)

    if ($env:OS -eq "Windows_NT") {
        $principal = [Security.Principal.WindowsIdentity]::GetCurrent().Name
        & icacls.exe $Path /inheritance:r /grant:r "${principal}:(F)" *> $null
        if ($LASTEXITCODE -ne 0) {
            throw "Could not restrict the local runtime secret file to the current Windows user."
        }
    }
}

function Initialize-LedgerSyncLocalSecrets {
    if (Test-LedgerSyncRuntimeEnvironmentFile -Path $script:LedgerSyncRuntimeEnvironmentFile) {
        return
    }

    New-Item -ItemType Directory -Force -Path $script:LedgerSyncRuntimeStateDirectory | Out-Null
    $pendingPath = "$($script:LedgerSyncRuntimeEnvironmentFile).pending"
    if (-not (Test-LedgerSyncRuntimeEnvironmentFile -Path $pendingPath)) {
        $lines = @(
            "POSTGRES_PASSWORD=$(New-LedgerSyncLocalSecret)",
            "LEDGERSYNC_SESSION_SECRET=$(New-LedgerSyncLocalSecret)",
            "LEDGERSYNC_CONSISTENCY_SIGNING_KEY=$(New-LedgerSyncLocalSecret)",
            "LEDGERSYNC_BFF_ASSERTION_SECRET=$(New-LedgerSyncLocalSecret)",
            "LEDGERSYNC_WEB_SESSION_SECRET=$(New-LedgerSyncLocalSecret)",
            "LEDGERSYNC_DEVELOPMENT_API_TOKEN=$(New-LedgerSyncLocalSecret)"
        )
        [IO.File]::WriteAllLines($pendingPath, $lines, [Text.UTF8Encoding]::new($false))
        Protect-LedgerSyncLocalSecretFile -Path $pendingPath
    }

    # Upgrade an existing pre-Phase-6 database without resetting its named
    # volume. The new secret travels only over stdin to the local container.
    $postgresContainer = @(& docker ps -a `
        --filter "label=com.docker.compose.project=$script:LedgerSyncComposeProject" `
        --filter "label=com.docker.compose.service=postgres" `
        --format '{{.ID}}' 2>$null | Select-Object -First 1)
    if ($postgresContainer.Count -eq 1 -and -not [string]::IsNullOrWhiteSpace([string]$postgresContainer[0])) {
        $containerID = [string]$postgresContainer[0]
        $running = (& docker inspect --format '{{.State.Running}}' $containerID 2>$null) -eq "true"
        if (-not $running) {
            & docker start $containerID *> $null
            if ($LASTEXITCODE -ne 0) { throw "Could not start the existing PostgreSQL container for local credential rotation." }
        }
        $ready = $false
        for ($attempt = 1; $attempt -le 30; $attempt++) {
            & docker exec $containerID pg_isready -U ledgersync -d ledgersync *> $null
            if ($LASTEXITCODE -eq 0) { $ready = $true; break }
            Start-Sleep -Seconds 1
        }
        if (-not $ready) { throw "Existing PostgreSQL did not become ready for local credential rotation." }
        $postgresPassword = Get-LedgerSyncRuntimeEnvironmentValue -Path $pendingPath -Name "POSTGRES_PASSWORD"
        "ALTER ROLE ledgersync PASSWORD '$postgresPassword';" | & docker exec -i $containerID psql -v ON_ERROR_STOP=1 -U ledgersync -d ledgersync *> $null
        if ($LASTEXITCODE -ne 0) { throw "Existing PostgreSQL rejected the local credential rotation; no runtime secret file was activated." }
    } else {
        $volumeName = "$($script:LedgerSyncComposeProject)_postgres-data"
        & docker volume inspect $volumeName *> $null
        if ($LASTEXITCODE -eq 0) {
            throw "A preserved PostgreSQL volume exists without its container, so automatic password rotation cannot prove safety. Restore the matching postgres container or use the explicit reset-local workflow."
        }
    }

    Move-Item -LiteralPath $pendingPath -Destination $script:LedgerSyncRuntimeEnvironmentFile -Force
    Protect-LedgerSyncLocalSecretFile -Path $script:LedgerSyncRuntimeEnvironmentFile
}

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
        "--env-file", $script:LedgerSyncRuntimeEnvironmentFile,
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
    $sql = "SELECT json_build_object('migration_version',COALESCE((SELECT max(version) FROM schema_migrations),'none'),'migration_count',(SELECT count(*) FROM schema_migrations),'outbox_pending',(SELECT count(*) FROM outbox_events WHERE published_at IS NULL AND dead_at IS NULL),'outbox_dead',(SELECT count(*) FROM outbox_events WHERE dead_at IS NOT NULL),'reconciliation_status',COALESCE((SELECT status FROM reconciliation_runs ORDER BY started_at DESC LIMIT 1),'not_run'),'reconciliation_mismatches',COALESCE((SELECT mismatch_count FROM reconciliation_runs ORDER BY started_at DESC LIMIT 1),0));"
    $result = @(Invoke-LedgerSyncCompose -ComposeArguments @("exec", "-T", "postgres", "psql", "-U", "ledgersync", "-d", "ledgersync", "-Atc", $sql) -CaptureOutput)
    $json = @($result | Where-Object { ([string]$_).TrimStart().StartsWith("{") } | Select-Object -Last 1)
    if ($json.Count -ne 1) {
        throw "Operational summary did not return a bounded result."
    }
    return ([string]$json[0] | ConvertFrom-Json)
}

function Get-LedgerSyncFinancialFingerprint {
    $sql = @"
SELECT json_build_object(
  'migration_version', COALESCE((SELECT max(version) FROM schema_migrations), 'none'),
  'migration_count', (SELECT count(*) FROM schema_migrations),
  'accounts', (SELECT count(*) FROM accounts),
  'transfers', (SELECT count(*) FROM transfers),
  'ledger_postings', (SELECT count(*) FROM ledger_postings),
  'balance_fingerprint', COALESCE((
    SELECT md5(string_agg(account_id::text || ':' || available_minor::text || ':' || ledger_minor::text || ':' || balance_version::text, ',' ORDER BY account_id))
    FROM account_balance_projections
  ), md5('')),
  'transfer_fingerprint', COALESCE((
    SELECT md5(string_agg(id::text || ':' || status || ':' || amount_minor::text || ':' || currency, ',' ORDER BY id))
    FROM transfers
  ), md5('')),
  'posting_fingerprint', COALESCE((
    SELECT md5(string_agg(id::text || ':' || journal_transaction_id::text || ':' || account_id::text || ':' || direction || ':' || amount_minor::text || ':' || currency, ',' ORDER BY id))
    FROM ledger_postings
  ), md5(''))
)::text;
"@
    $result = @(Invoke-LedgerSyncCompose -ComposeArguments @(
        "exec", "-T", "postgres", "psql", "-U", "ledgersync", "-d", "ledgersync", "-Atc", $sql
    ) -CaptureOutput)
    $json = @($result | Where-Object { ([string]$_).TrimStart().StartsWith("{") } | Select-Object -Last 1)
    if ($json.Count -ne 1) {
        throw "Financial fingerprint did not return one bounded result."
    }
    return ([string]$json[0] | ConvertFrom-Json)
}

function Compare-LedgerSyncFinancialFingerprint {
    param(
        [Parameter(Mandatory = $true)][object]$Before,
        [Parameter(Mandatory = $true)][object]$After
    )

    foreach ($field in @(
        "migration_version", "migration_count", "accounts", "transfers", "ledger_postings",
        "balance_fingerprint", "transfer_fingerprint", "posting_fingerprint"
    )) {
        if ([string]$Before.$field -cne [string]$After.$field) {
            throw "Authoritative financial fingerprint changed at '$field'."
        }
    }
}

function ConvertTo-LedgerSyncRedactedLogLine {
    param([Parameter(Mandatory = $true)][string]$Line)

    $redacted = $Line -replace '(?i)(authorization:\s*bearer\s+)[^\s]+', '$1[REDACTED]'
    $redacted = $redacted -replace '(?i)(postgres(?:ql)?://[^:\s]+:)[^@\s]+(@)', '$1[REDACTED]$2'
    $redacted = $redacted -replace '(?i)(password|secret|token|signing_key|authorization|cookie|session|csrf|consistency|credential|database_url|connection_string|dsn|private_key|api_key|access_key|balance|amount|email|phone|address|ip_address)(?:_[a-z0-9]+)*(["''=: ]+)([^,\s}"'']+)', '$1$2[REDACTED]'
    return $redacted
}
