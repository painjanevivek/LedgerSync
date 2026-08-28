Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$script:LedgerSyncRepositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$script:LedgerSyncComposeFile = Join-Path $script:LedgerSyncRepositoryRoot "deploy\compose\docker-compose.yml"
$script:LedgerSyncWebUrl = "http://127.0.0.1:3000"
$script:LedgerSyncLongRunningServices = @("postgres", "redis", "api", "outbox-worker", "web")
$script:LedgerSyncOneShotServices = @("migrate", "demo-seed")
$script:LedgerSyncMinimumComposeVersion = [Version]::new(2, 20, 0)
$script:LedgerSyncMinimumFreeDiskBytes = 5GB

$requestedProject = [Environment]::GetEnvironmentVariable("LEDGERSYNC_LOCAL_COMPOSE_PROJECT")
if ([string]::IsNullOrWhiteSpace($requestedProject)) {
    $requestedProject = "compose"
}
if ($requestedProject -cnotmatch '^[a-z0-9][a-z0-9_-]{0,62}$') {
    throw "LEDGERSYNC_LOCAL_COMPOSE_PROJECT must contain only lowercase letters, digits, underscores, or hyphens."
}
$script:LedgerSyncComposeProject = $requestedProject

$script:LedgerSyncRuntimeStateDirectory = Join-Path $script:LedgerSyncRepositoryRoot "data\local-runtime"
$requestedStateDirectory = [Environment]::GetEnvironmentVariable("LEDGERSYNC_LOCAL_STATE_DIRECTORY")
if (-not [string]::IsNullOrWhiteSpace($requestedStateDirectory)) {
    if ($script:LedgerSyncComposeProject -cnotmatch '^ledgersync-acceptance-\d{14}-[0-9a-f]{8}$') {
        throw "An isolated state directory is permitted only for an exact LedgerSync acceptance project."
    }
    $acceptanceRoot = [IO.Path]::GetFullPath((Join-Path $script:LedgerSyncRepositoryRoot "data\local-acceptance"))
    $expectedStateDirectory = [IO.Path]::GetFullPath((Join-Path $acceptanceRoot $script:LedgerSyncComposeProject))
    $resolvedStateDirectory = if ([IO.Path]::IsPathRooted($requestedStateDirectory)) {
        [IO.Path]::GetFullPath($requestedStateDirectory)
    } else {
        [IO.Path]::GetFullPath((Join-Path $script:LedgerSyncRepositoryRoot $requestedStateDirectory))
    }
    if (-not $resolvedStateDirectory.Equals($expectedStateDirectory, [StringComparison]::OrdinalIgnoreCase)) {
        throw "The isolated state directory must exactly match data/local-acceptance/<acceptance-project>."
    }
    $script:LedgerSyncRuntimeStateDirectory = $resolvedStateDirectory
}
$script:LedgerSyncRuntimeEnvironmentFile = Join-Path $script:LedgerSyncRuntimeStateDirectory "runtime.env"
$script:LedgerSyncRuntimeSecretNames = @(
    "POSTGRES_PASSWORD",
    "LEDGERSYNC_API_DATABASE_PASSWORD",
    "LEDGERSYNC_WORKER_DATABASE_PASSWORD",
    "LEDGERSYNC_SESSION_SECRET",
    "LEDGERSYNC_CONSISTENCY_SIGNING_KEY",
    "LEDGERSYNC_BFF_ASSERTION_SECRET",
    "LEDGERSYNC_WEB_SESSION_SECRET",
    "LEDGERSYNC_DEVELOPMENT_API_TOKEN"
)
$script:LedgerSyncLegacyRuntimeSecretNames = @(
    "POSTGRES_PASSWORD",
    "LEDGERSYNC_SESSION_SECRET",
    "LEDGERSYNC_CONSISTENCY_SIGNING_KEY",
    "LEDGERSYNC_BFF_ASSERTION_SECRET",
    "LEDGERSYNC_WEB_SESSION_SECRET",
    "LEDGERSYNC_DEVELOPMENT_API_TOKEN"
)

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
    $values = @{}
    foreach ($line in Get-Content -LiteralPath $Path) {
        if ($line -cnotmatch '^([A-Z0-9_]+)=([a-f0-9]{64})$' -or $values.ContainsKey([string]$Matches[1])) {
            return $false
        }
        $values[[string]$Matches[1]] = [string]$Matches[2]
    }
    return @($script:LedgerSyncRuntimeSecretNames | Where-Object { -not $values.ContainsKey($_) }).Count -eq 0
}

function New-LedgerSyncRuntimeEnvironmentLines {
    param([string]$ExistingPath)

    $lines = [Collections.Generic.List[string]]::new()
    $values = @{}
    if (-not [string]::IsNullOrWhiteSpace($ExistingPath) -and (Test-Path -LiteralPath $ExistingPath -PathType Leaf)) {
        foreach ($line in Get-Content -LiteralPath $ExistingPath) {
            if ($line -cnotmatch '^([A-Z0-9_]+)=([a-f0-9]{64})$') {
                throw "Existing local runtime secret state is malformed; it was not replaced."
            }
            $name = [string]$Matches[1]
            if ($values.ContainsKey($name)) {
                throw "Existing local runtime secret state contains a duplicate entry; it was not replaced."
            }
            $values[$name] = [string]$Matches[2]
            $lines.Add([string]$line)
        }
        if (@($script:LedgerSyncLegacyRuntimeSecretNames | Where-Object { -not $values.ContainsKey($_) }).Count -ne 0) {
            throw "Existing local runtime secret state is incomplete; it was not replaced."
        }
    }

    foreach ($name in $script:LedgerSyncRuntimeSecretNames) {
        if (-not $values.ContainsKey($name)) {
            $value = New-LedgerSyncLocalSecret
            $values[$name] = $value
            $lines.Add("$name=$value")
        }
    }
    return @($lines)
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
        $upgradeSource = if (Test-Path -LiteralPath $script:LedgerSyncRuntimeEnvironmentFile -PathType Leaf) {
            $script:LedgerSyncRuntimeEnvironmentFile
        } elseif (Test-Path -LiteralPath $pendingPath -PathType Leaf) {
            $pendingPath
        } else {
            $null
        }
        $lines = @(New-LedgerSyncRuntimeEnvironmentLines -ExistingPath $upgradeSource)
        [IO.File]::WriteAllLines($pendingPath, $lines, [Text.UTF8Encoding]::new($false))
        Protect-LedgerSyncLocalSecretFile -Path $pendingPath
    }

    # Activate new or upgraded protected state without resetting the named
    # volume. The owner secret travels only over stdin to the local container.
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

function Resolve-LedgerSyncDockerFailure {
    param([AllowEmptyString()][string]$Details)

    if ($Details -match '(?i)(permission denied|access is denied|unauthorized|forbidden)') {
        return "Docker is installed, but this user cannot access the engine. Open Docker Desktop with your normal user, verify Docker permissions, then run scripts/doctor-local.ps1 again."
    }
    if ($Details -match '(?i)(cannot connect|failed to connect|is the docker daemon running|docker_engine|connection refused|the system cannot find the file specified)') {
        return "Docker is installed, but its engine is stopped or still starting. Start Docker Desktop, wait for Engine running, then run scripts/doctor-local.ps1 again."
    }
    return "Docker is installed, but the engine health check failed. Run 'docker info', resolve the reported engine error, then run scripts/doctor-local.ps1 again."
}

function Assert-LedgerSyncDockerAvailable {
    if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
        throw "Docker is not installed or is not on PATH. Install Docker Desktop, open a new PowerShell window, then run scripts/doctor-local.ps1."
    }

    $nativePreference = $PSNativeCommandUseErrorActionPreference
    try {
        $PSNativeCommandUseErrorActionPreference = $false
        $details = @(& docker info --format '{{json .ServerVersion}}' 2>&1)
        if ($LASTEXITCODE -ne 0) {
            throw (Resolve-LedgerSyncDockerFailure -Details ($details -join " "))
        }
    }
    finally {
        $PSNativeCommandUseErrorActionPreference = $nativePreference
    }
}

function Get-LedgerSyncComposeVersion {
    $nativePreference = $PSNativeCommandUseErrorActionPreference
    try {
        $PSNativeCommandUseErrorActionPreference = $false
        $output = @(& docker compose version --short 2>&1)
        if ($LASTEXITCODE -ne 0) {
            throw "Docker Compose is unavailable. Update Docker Desktop so the Compose v2 plugin is installed, then run scripts/doctor-local.ps1 again."
        }
    }
    finally {
        $PSNativeCommandUseErrorActionPreference = $nativePreference
    }
    $text = ($output -join " ").Trim()
    if ($text -notmatch '(?i)v?(\d+)\.(\d+)\.(\d+)') {
        throw "Docker Compose returned an unreadable version. Update Docker Desktop, then run scripts/doctor-local.ps1 again."
    }
    $version = [Version]::new([int]$Matches[1], [int]$Matches[2], [int]$Matches[3])
    if ($version -lt $script:LedgerSyncMinimumComposeVersion) {
        throw "Docker Compose $version is too old. Install Compose $($script:LedgerSyncMinimumComposeVersion) or newer, then run scripts/doctor-local.ps1 again."
    }
    return $version
}

function Get-LedgerSyncRepositoryFreeBytes {
    $root = [IO.Path]::GetPathRoot([IO.Path]::GetFullPath($script:LedgerSyncRepositoryRoot))
    return [int64]([IO.DriveInfo]::new($root).AvailableFreeSpace)
}

function Assert-LedgerSyncLocalPrerequisites {
    if ($PSVersionTable.PSVersion -lt [Version]::new(7, 2, 0)) {
        throw "PowerShell $($PSVersionTable.PSVersion) is too old. Install PowerShell 7.2 or newer, then run scripts/doctor-local.ps1 again."
    }
    if (-not (Get-Command git -ErrorAction SilentlyContinue)) {
        throw "Git is not installed or is not on PATH. Install Git, open a new PowerShell window, then run scripts/doctor-local.ps1 again."
    }
    Assert-LedgerSyncDockerAvailable
    Get-LedgerSyncComposeVersion | Out-Null
    $freeBytes = Get-LedgerSyncRepositoryFreeBytes
    if ($freeBytes -lt $script:LedgerSyncMinimumFreeDiskBytes) {
        $freeGiB = [Math]::Round($freeBytes / 1GB, 1)
        throw "Only $freeGiB GiB is free on the repository drive. Free at least 5 GiB for images, volumes, backups, and build layers, then run scripts/doctor-local.ps1 again."
    }
    Test-LedgerSyncPortAvailableOrOwned
}

function Get-LedgerSyncServiceRecoveryGuidance {
    param(
        [Parameter(Mandatory = $true)][ValidateSet("postgres", "redis", "api", "outbox-worker", "web", "migrate", "demo-seed")][string]$Service,
        [AllowEmptyString()][string]$State,
        [AllowEmptyString()][string]$Health,
        [AllowNull()][Nullable[int]]$ExitCode
    )

    $impact = switch ($Service) {
        "postgres" { "Authoritative ledger reads and writes are unavailable." }
        "redis" { "Cached reads and event delivery are unavailable; PostgreSQL ledger data remains authoritative." }
        "api" { "LedgerSync reads and commands are unavailable." }
        "outbox-worker" { "New ledger commits remain authoritative, but downstream event delivery is paused." }
        "web" { "The browser console is unavailable; private services remain isolated." }
        "migrate" { "The schema was not proven compatible, so dependent services must not start." }
        "demo-seed" { "Demo initialization did not complete; no ready result may be trusted." }
    }
    $action = switch ($Service) {
        "postgres" { "Run scripts/logs-local.ps1 -Service postgres. If storage is suspect, stop commands and follow docs/runbooks/restore.md." }
        "redis" { "Run scripts/logs-local.ps1 -Service redis, then scripts/start-local.ps1 -SkipBuild. Do not edit PostgreSQL to repair a cache symptom." }
        "api" { "Run scripts/logs-local.ps1 -Service api, then scripts/start-local.ps1 -SkipBuild after PostgreSQL and Redis are healthy." }
        "outbox-worker" { "Run scripts/logs-local.ps1 -Service outbox-worker, then scripts/start-local.ps1 -SkipBuild. Do not replay events manually." }
        "web" { "Run scripts/logs-local.ps1 -Service web, verify port 3000 with scripts/doctor-local.ps1, then scripts/start-local.ps1 -SkipBuild." }
        "migrate" { "Run scripts/logs-local.ps1 -Service migrate and repair the migration error. Never mark or skip a failed migration." }
        "demo-seed" { "Run scripts/logs-local.ps1 -Service demo-seed. If it reports incompatible demo evidence, back up and use the explicit reset workflow." }
    }
    $condition = if ($Service -in $script:LedgerSyncOneShotServices) {
        "state=$State; exit=$ExitCode"
    } else {
        "state=$State; health=$Health"
    }
    return [pscustomobject]@{ Service = $Service; Condition = $condition; Impact = $impact; NextAction = $action }
}

function Get-LedgerSyncLocalDoctorChecks {
    $checks = [Collections.Generic.List[object]]::new()
    $add = {
        param([string]$Name, [string]$Status, [string]$Detail, [string]$NextAction)
        $checks.Add([pscustomobject]@{ Check = $Name; Status = $Status; Detail = $Detail; NextAction = $NextAction })
    }

    if ($PSVersionTable.PSVersion -lt [Version]::new(7, 2, 0)) {
        & $add "PowerShell" "fail" "$($PSVersionTable.PSVersion)" "Install PowerShell 7.2 or newer and rerun this command."
    } else {
        & $add "PowerShell" "pass" "$($PSVersionTable.PSVersion)" "None"
    }
    foreach ($binary in @("git", "docker")) {
        if (Get-Command $binary -ErrorAction SilentlyContinue) {
            & $add $binary "pass" "Available on PATH" "None"
        } else {
            & $add $binary "fail" "Not found on PATH" "Install $binary, open a new PowerShell window, and rerun this command."
        }
    }
    $engineAvailable = $false
    if (Get-Command docker -ErrorAction SilentlyContinue) {
        try {
            Assert-LedgerSyncDockerAvailable
            $engineAvailable = $true
            & $add "Docker engine" "pass" "Engine responded" "None"
        }
        catch { & $add "Docker engine" "fail" $_.Exception.Message "Follow the stated Docker recovery action." }
    } else {
        & $add "Docker engine" "blocked" "Docker executable is unavailable" "Install Docker Desktop first."
    }
    if ($engineAvailable) {
        try { $composeVersion = Get-LedgerSyncComposeVersion; & $add "Docker Compose" "pass" "$composeVersion" "None" }
        catch { & $add "Docker Compose" "fail" $_.Exception.Message "Update Docker Desktop and rerun this command." }
    } else {
        & $add "Docker Compose" "blocked" "Docker engine prerequisite is unavailable" "Restore Docker access, then rerun this command."
    }
    try {
        $freeBytes = Get-LedgerSyncRepositoryFreeBytes
        $freeGiB = [Math]::Round($freeBytes / 1GB, 1)
        if ($freeBytes -lt $script:LedgerSyncMinimumFreeDiskBytes) {
            & $add "Disk space" "fail" "$freeGiB GiB free" "Free at least 5 GiB on the repository drive."
        } else { & $add "Disk space" "pass" "$freeGiB GiB free" "None" }
    } catch { & $add "Disk space" "fail" "Could not measure repository drive" "Verify the repository drive is mounted and writable." }
    if ($engineAvailable) {
        try { Test-LedgerSyncPortAvailableOrOwned; & $add "Loopback port 3000" "pass" "Free or owned by this LedgerSync web container" "None" }
        catch { & $add "Loopback port 3000" "fail" $_.Exception.Message "Stop the conflicting process yourself; LedgerSync will not terminate it." }
    } else {
        & $add "Loopback port 3000" "blocked" "Container ownership cannot be proven without Docker" "Restore Docker access, then rerun this command."
    }

    if (-not (Test-Path -LiteralPath $script:LedgerSyncComposeFile -PathType Leaf)) {
        & $add "Compose file" "fail" "Missing deploy/compose/docker-compose.yml" "Restore the repository checkout before starting LedgerSync."
    } else { & $add "Compose file" "pass" "Present" "None" }
    if (-not (Test-Path -LiteralPath $script:LedgerSyncRuntimeEnvironmentFile -PathType Leaf)) {
        & $add "Runtime environment" "info" "Not initialized" "scripts/start-local.ps1 will create a protected local secret file."
    } elseif (Test-LedgerSyncRuntimeEnvironmentFile -Path $script:LedgerSyncRuntimeEnvironmentFile) {
        & $add "Runtime environment" "pass" "Protected secret schema is complete" "None"
    } else {
        & $add "Runtime environment" "fail" "Existing file is malformed or incomplete" "Preserve it for investigation; do not replace it until the matching PostgreSQL credential state is understood."
    }
    if ($engineAvailable) {
        foreach ($volume in @("postgres-data", "redis-data")) {
            $volumeName = "$($script:LedgerSyncComposeProject)_$volume"
            $nativePreference = $PSNativeCommandUseErrorActionPreference
            try {
                $PSNativeCommandUseErrorActionPreference = $false
                & docker volume inspect $volumeName *> $null
                $state = if ($LASTEXITCODE -eq 0) { "present and preserved" } else { "absent; first start will create it" }
            }
            finally {
                $PSNativeCommandUseErrorActionPreference = $nativePreference
            }
            & $add "Volume $volume" "info" $state "Normal stop/start preserves this volume; only reset-local deletes it."
        }
    } else {
        & $add "Volume state" "blocked" "Docker engine prerequisite is unavailable" "Restore Docker access; the doctor will then inspect exact project volumes read-only."
    }
    return @($checks)
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
            $state = if ($row.Count -eq 1) { [string]$row[0].State } else { "missing" }
            $exitCode = if ($row.Count -eq 1) { [int]$row[0].ExitCode } else { $null }
            $guidance = Get-LedgerSyncServiceRecoveryGuidance -Service $service -State $state -ExitCode $exitCode
            throw "The $service setup step did not complete successfully ($($guidance.Condition)). $($guidance.Impact) Next action: $($guidance.NextAction)"
        }
    }
}

function Assert-LedgerSyncLongRunningServicesHealthy {
    $rows = @(Get-LedgerSyncComposeRows)
    foreach ($service in $script:LedgerSyncLongRunningServices) {
        $row = @(Get-LedgerSyncServiceRow -Service $service -Rows $rows)
        if ($row.Count -ne 1 -or $row[0].State -ne "running") {
            $state = if ($row.Count -eq 1) { [string]$row[0].State } else { "missing" }
            $health = if ($row.Count -eq 1) { [string]$row[0].Health } else { "unknown" }
            $guidance = Get-LedgerSyncServiceRecoveryGuidance -Service $service -State $state -Health $health
            throw "The $service service is not running ($($guidance.Condition)). $($guidance.Impact) Next action: $($guidance.NextAction)"
        }
        if ($row[0].Health -and $row[0].Health -ne "healthy") {
            $guidance = Get-LedgerSyncServiceRecoveryGuidance -Service $service -State ([string]$row[0].State) -Health ([string]$row[0].Health)
            throw "The $service service is running but not healthy ($($guidance.Condition)). $($guidance.Impact) Next action: $($guidance.NextAction)"
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

    $ownerIDs = @(& docker ps --filter "publish=3000" --format '{{.ID}}' 2>$null)
    if ($LASTEXITCODE -ne 0) {
        throw "Port 3000 is listening, but Docker ownership could not be verified."
    }
    $owners = @($ownerIDs | ForEach-Object {
        $container = & docker inspect ([string]$_) 2>$null | ConvertFrom-Json
        if ($LASTEXITCODE -ne 0 -or $null -eq $container) {
            throw "Port 3000 is listening, but its Docker container could not be inspected."
        }
        $labels = @($container)[0].Config.Labels
        "$($labels.'com.docker.compose.project')|$($labels.'com.docker.compose.service')"
    })
    $expected = "$($script:LedgerSyncComposeProject)|web"
    if ($owners -notcontains $expected) {
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
