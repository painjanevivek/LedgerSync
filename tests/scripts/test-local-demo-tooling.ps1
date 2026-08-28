[CmdletBinding()]
param()

$ErrorActionPreference = "Stop"
$repositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
. (Join-Path $repositoryRoot "scripts\local-runtime-common.ps1")
. (Join-Path $repositoryRoot "scripts\local-backup-common.ps1")
. (Join-Path $repositoryRoot "scripts\local-initialization-common.ps1")
. (Join-Path $repositoryRoot "scripts\local-retry-lab-common.ps1")

$temporaryParent = [IO.Path]::GetFullPath([IO.Path]::GetTempPath())
$testRoot = Join-Path $temporaryParent "ledgersync-demo-tooling-$([Guid]::NewGuid().ToString('N'))"
$originalRepositoryRoot = $script:LedgerSyncRepositoryRoot
$originalStateDirectory = $script:LedgerSyncRuntimeStateDirectory
$originalProject = $script:LedgerSyncComposeProject
$retryJunctionPath = $null

function Assert-DemoTooling {
    param([bool]$Condition, [string]$Message)
    if (-not $Condition) { throw $Message }
}

function Assert-DemoRejected {
    param([scriptblock]$Action, [string]$Message)
    $rejected = $false
    try { & $Action }
    catch { $rejected = $true }
    Assert-DemoTooling $rejected $Message
}

try {
    New-Item -ItemType Directory -Path $testRoot | Out-Null
    Assert-DemoTooling ((Resolve-LedgerSyncInitializationModeDecision -PostgresVolumeExists $false) -ceq "empty") "Fresh default initialization was not empty."
    Assert-DemoTooling ((Resolve-LedgerSyncInitializationModeDecision -RequestedMode "empty" -PostgresVolumeExists $false) -ceq "empty") "Fresh empty initialization was not accepted."
    Assert-DemoTooling ((Resolve-LedgerSyncInitializationModeDecision -ExistingMode "demo" -PostgresVolumeExists $true) -ceq "demo") "Existing demo mode was not retained."
    Assert-DemoTooling ((Resolve-LedgerSyncInitializationModeDecision -ExistingMode "demo" -RequestedMode "empty" -PostgresVolumeExists $false) -ceq "empty") "Mode did not change after the PostgreSQL volume was absent."
    Assert-DemoRejected {
        Resolve-LedgerSyncInitializationModeDecision -ExistingMode "demo" -RequestedMode "empty" -PostgresVolumeExists $true | Out-Null
    } "Mode switched from demo to empty over an existing PostgreSQL volume."
    Assert-DemoRejected {
        Resolve-LedgerSyncInitializationModeDecision -RequestedMode "empty" -PostgresVolumeExists $true | Out-Null
    } "Empty mode was adopted over an existing unmarked PostgreSQL volume."
    Assert-DemoRejected {
        Resolve-LedgerSyncInitializationModeDecision -RequestedMode "hostile" -PostgresVolumeExists $false | Out-Null
    } "An unsupported initialization mode was accepted."

    $validRetryProject = "ledgersync-acceptance-20260825010203-1234abcd"
    $validRetryState = Join-Path $repositoryRoot "data\local-acceptance\$validRetryProject"
    Assert-LedgerSyncRetryLabIdentity -RepositoryRoot $repositoryRoot `
        -Project $validRetryProject -StateDirectory $validRetryState | Out-Null
    Assert-DemoRejected {
        Assert-LedgerSyncRetryLabIdentity -RepositoryRoot $repositoryRoot `
            -Project "compose" -StateDirectory (Join-Path $repositoryRoot "data\local-acceptance\compose") | Out-Null
    } "Retry lab accepted the normal Compose project."
    Assert-DemoRejected {
        Assert-LedgerSyncRetryLabIdentity -RepositoryRoot $repositoryRoot `
            -Project $validRetryProject -StateDirectory (Join-Path $repositoryRoot "data\local-runtime") | Out-Null
    } "Retry lab accepted state outside its exact generated project path."
    Assert-DemoRejected {
        Assert-LedgerSyncRetryLabIdentity -RepositoryRoot $repositoryRoot `
            -Project $validRetryProject -StateDirectory (Join-Path $repositoryRoot "data\local-acceptance\$validRetryProject\..\other") | Out-Null
    } "Retry lab accepted a traversal state path."
    $retryIdentityRepository = Join-Path $testRoot "retry-identity-repository"
    $retryIdentityRoot = Join-Path $retryIdentityRepository "data\local-acceptance"
    $retryIdentityTarget = Join-Path $testRoot "retry-identity-target"
    New-Item -ItemType Directory -Path $retryIdentityRoot -Force | Out-Null
    New-Item -ItemType Directory -Path $retryIdentityTarget | Out-Null
    $retryJunctionPath = Join-Path $retryIdentityRoot $validRetryProject
    if ($env:OS -eq "Windows_NT") {
        New-Item -ItemType Junction -Path $retryJunctionPath -Target $retryIdentityTarget | Out-Null
    } else {
        New-Item -ItemType SymbolicLink -Path $retryJunctionPath -Target $retryIdentityTarget | Out-Null
    }
    Assert-DemoRejected {
        Assert-LedgerSyncRetryLabIdentity -RepositoryRoot $retryIdentityRepository `
            -Project $validRetryProject -StateDirectory $retryJunctionPath | Out-Null
    } "Retry lab accepted a reparse-point state directory."
    Remove-Item -LiteralPath $retryJunctionPath -Force
    $retryJunctionPath = $null

    $script:LedgerSyncRepositoryRoot = $testRoot
    $script:LedgerSyncRuntimeStateDirectory = Join-Path $testRoot "state"
    $script:LedgerSyncComposeProject = "ledgersync-acceptance-20260825010101-abcdef12"
    $marker = Write-LedgerSyncInitializationModeMarker -Mode "demo"
    $markerPath = Get-LedgerSyncInitializationModePath
    Assert-DemoTooling ($marker.mode -ceq "demo" -and $marker.compose_project -ceq $script:LedgerSyncComposeProject) "Protected initialization marker was not bound to its project."
    if ($env:OS -eq "Windows_NT") {
        Assert-DemoTooling ((Get-Acl -LiteralPath $markerPath).AreAccessRulesProtected) "Initialization marker ACL is not protected."
    }
    $script:LedgerSyncComposeProject = "ledgersync-acceptance-20260825010102-abcdef13"
    Assert-DemoRejected {
        Read-LedgerSyncInitializationModeMarker | Out-Null
    } "A marker belonging to another exact Compose project was accepted."
    $script:LedgerSyncComposeProject = "ledgersync-acceptance-20260825010101-abcdef12"
    [IO.File]::WriteAllText($markerPath, "{malformed", [Text.UTF8Encoding]::new($false))
    Protect-LedgerSyncRecoveryFile -Path $markerPath
    Assert-DemoRejected {
        Read-LedgerSyncInitializationModeMarker | Out-Null
    } "A malformed initialization marker was accepted or repaired."
    Assert-DemoTooling ((Get-Content -LiteralPath $markerPath -Raw) -ceq "{malformed") "Malformed initialization evidence was overwritten."

    $retryScript = Get-Content -LiteralPath (Join-Path $repositoryRoot "scripts\run-local-retry-lab.ps1") -Raw
    Assert-DemoTooling ($retryScript -match 'ConfirmIsolatedRetryLab' -and
        $retryScript -match '\^ledgersync-acceptance-' -and
        $retryScript -notmatch '(?i)(toxiproxy|fault[_-]?toggle|kill\s+api|restart.+api)') "Retry lab lacks its explicit isolated boundary or adds a runtime fault toggle."
    Assert-DemoTooling ($retryScript -notmatch '(?m)^\s*\[string\]\$AcceptanceProject') "Retry lab accepts a caller-selected Compose project."
    $noFlagOutput = @(& pwsh -NoProfile -File (Join-Path $repositoryRoot "scripts\run-local-retry-lab.ps1") *>&1)
    Assert-DemoTooling ($LASTEXITCODE -ne 0 -and ($noFlagOutput -join "`n") -match 'ConfirmIsolatedRetryLab') "Retry lab did not reject an omitted confirmation flag before Docker access."

    $composeSource = Get-Content -LiteralPath (Join-Path $repositoryRoot "deploy\compose\docker-compose.yml") -Raw
    Assert-DemoTooling ($composeSource -match 'LEDGERSYNC_INITIALIZATION_MODE' -and
        $composeSource -match 'local-bootstrap.sql' -and $composeSource -match 'demo\) exec psql' -and
        $composeSource -match 'empty\).*Fresh workspace initialized without sample financial records' -and
        $composeSource -match 'Unsupported LedgerSync initialization mode') "Compose does not distinguish strict demo/empty fresh initialization."
    $startSource = Get-Content -LiteralPath (Join-Path $repositoryRoot "scripts\start-local.ps1") -Raw
    $resetSource = Get-Content -LiteralPath (Join-Path $repositoryRoot "scripts\reset-local.ps1") -Raw
    Assert-DemoTooling ($startSource -match 'Initialize-LedgerSyncInitializationMode' -and
        $resetSource -match 'Set-LedgerSyncFreshInitializationMode') "Start/reset scripts do not enforce the host-owned mode marker."

    $apiRoutes = @(Get-ChildItem -LiteralPath (Join-Path $repositoryRoot "web\src\app\api") -Recurse -Filter "route.ts" -File)
    foreach ($route in $apiRoutes) {
        $source = Get-Content -LiteralPath $route.FullName -Raw
        Assert-DemoTooling ($source -notmatch '(?i)(reset-local|demo-seed|initialization-mode|docker\s+compose|child_process|execFile|spawn\s*\()') "A browser route gained reset, reseed, initialization, Docker, or shell authority."
    }
    Assert-DemoTooling (@($apiRoutes | Where-Object { $_.FullName -match '(?i)(reset|reseed|initialization)' }).Count -eq 0) "A browser-visible reset/reseed route exists."

    Write-Output "LOCAL_DEMO_TOOLING_TESTS=PASS"
    Write-Output "MODE_SWITCH_OVER_EXISTING_LEDGER=REJECTED"
    Write-Output "MALFORMED_MARKER=REJECTED_UNCHANGED"
    Write-Output "RETRY_LAB_NO_FLAG_AND_IDENTITY=PASS"
    Write-Output "BROWSER_RESET_RESEED_AUTHORITY=ABSENT"
}
finally {
    $script:LedgerSyncRepositoryRoot = $originalRepositoryRoot
    $script:LedgerSyncRuntimeStateDirectory = $originalStateDirectory
    $script:LedgerSyncComposeProject = $originalProject
    if ($retryJunctionPath -and (Test-Path -LiteralPath $retryJunctionPath)) {
        Remove-Item -LiteralPath $retryJunctionPath -Force
    }
    if (Test-Path -LiteralPath $testRoot) {
        Remove-LedgerSyncValidatedDirectory -Parent $temporaryParent -Directory $testRoot `
            -AllowedLeafPattern '^ledgersync-demo-tooling-[0-9a-f]{32}$'
    }
}
