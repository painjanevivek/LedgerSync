[CmdletBinding()]
param()

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$repositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$commonPath = Join-Path $repositoryRoot 'scripts/local-acceptance-common.ps1'
$harnessPath = Join-Path $repositoryRoot 'scripts/test-local-acceptance.ps1'
. $commonPath

function Assert-AcceptanceTooling {
    param([bool]$Condition,[Parameter(Mandatory = $true)][string]$Message)
    if (-not $Condition) { throw $Message }
}

function Assert-AcceptanceRejected {
    param([Parameter(Mandatory = $true)][scriptblock]$Action,[Parameter(Mandatory = $true)][string]$Message)
    $rejected = $false
    try { & $Action } catch { $rejected = $true }
    Assert-AcceptanceTooling $rejected $Message
}

$temporaryParent = [IO.Path]::GetFullPath([IO.Path]::GetTempPath())
$testRoot = Join-Path $temporaryParent "ledgersync-acceptance-tooling-$([Guid]::NewGuid().ToString('N'))"
$junctionPath = $null
try {
    foreach ($path in @($commonPath,$harnessPath)) {
        $tokens = $null; $errors = $null
        [Management.Automation.Language.Parser]::ParseFile($path,[ref]$tokens,[ref]$errors) | Out-Null
        Assert-AcceptanceTooling ($errors.Count -eq 0) "PowerShell parser rejected $path."
    }

    New-Item -ItemType Directory -Path $testRoot | Out-Null
    $stateRoot = Join-Path $testRoot 'data/local-acceptance'
    New-Item -ItemType Directory -Path $stateRoot -Force | Out-Null
    $project = 'ledgersync-acceptance-20260825112233-a1b2c3d4'
    $state = Join-Path $stateRoot $project
    Assert-LedgerSyncAcceptanceProjectIdentity -Project $project -StateRoot $stateRoot -StatePath $state
    Assert-AcceptanceRejected { Assert-LedgerSyncAcceptanceProjectIdentity -Project 'compose' -StateRoot $stateRoot -StatePath (Join-Path $stateRoot 'compose') } 'Normal Compose project was accepted as disposable.'
    Assert-AcceptanceRejected { Assert-LedgerSyncAcceptanceProjectIdentity -Project $project -StateRoot $stateRoot -StatePath (Join-Path $stateRoot "$project/../other") } 'Traversal state path was accepted.'
    Assert-AcceptanceRejected { Assert-LedgerSyncAcceptanceProjectIdentity -Project $project -StateRoot $stateRoot -StatePath (Join-Path $testRoot $project) } 'State outside the canonical acceptance root was accepted.'

    $junctionTarget = Join-Path $testRoot 'junction-target'
    New-Item -ItemType Directory -Path $junctionTarget | Out-Null
    $junctionPath = Join-Path $stateRoot $project
    if ($env:OS -eq 'Windows_NT') { New-Item -ItemType Junction -Path $junctionPath -Target $junctionTarget | Out-Null }
    else { New-Item -ItemType SymbolicLink -Path $junctionPath -Target $junctionTarget | Out-Null }
    Assert-AcceptanceRejected { Assert-LedgerSyncAcceptanceProjectIdentity -Project $project -StateRoot $stateRoot -StatePath $junctionPath } 'Reparse-point acceptance state was accepted.'
    Remove-Item -LiteralPath $junctionPath -Force
    $junctionPath = $null

    $header = @('schema_version','transfer_id','amount_minor','currency')
    $validCSV = '"schema_version","transfer_id","amount_minor","currency"' + "`r`n" + '"1","00000000-0000-4000-8000-000000000001","300","INR"' + "`r`n"
    $validRows = @(ConvertFrom-LedgerSyncAcceptanceCSV -Content $validCSV -ExpectedHeader $header)
    Assert-AcceptanceTooling ($validRows.Count -eq 1 -and [string]$validRows[0].amount_minor -ceq '300') 'Exact quoted CSV fixture was not preserved.'
    Assert-AcceptanceRejected { ConvertFrom-LedgerSyncAcceptanceCSV -Content ("schema_version,transfer_id,amount_minor,currency`n1,id,300,INR`n") -ExpectedHeader $header | Out-Null } 'Unquoted CSV fixture was accepted.'
    Assert-AcceptanceRejected { ConvertFrom-LedgerSyncAcceptanceCSV -Content $validCSV -ExpectedHeader @('schema_version','amount_minor','transfer_id','currency') | Out-Null } 'CSV header reordering was accepted.'
    $wrongSchemaCSV = $validCSV.Replace('"1"','"2"')
    Assert-AcceptanceRejected { ConvertFrom-LedgerSyncAcceptanceCSV -Content $wrongSchemaCSV -ExpectedHeader $header | Out-Null } 'Wrong CSV schema version was accepted.'

    $source = Get-Content -LiteralPath $harnessPath -Raw
    foreach ($marker in @(
        'status --porcelain --untracked-files=no','Get-LedgerSyncAcceptanceNormalState','Assert-LedgerSyncAcceptanceRuntimeHardening',
        '/api/session','Test-LedgerSyncAcceptanceOrientation','Test-LedgerSyncAcceptanceAccountCreationBoundary','Test-LedgerSyncAcceptanceLostTransferBoundary',
        'account_not_zero','account_inactive','Test-LedgerSyncAcceptanceReconciliationBoundary','Test-LedgerSyncAcceptanceOperationalEvidence',
        'Test-LedgerSyncAcceptanceExports','/api/recovery/manifests',"-Service redis","-Service outbox-worker","-Service api","-Service web","-Service postgres",
        'backup-local.ps1','local-restore-drill.ps1','Compare-LedgerSyncAcceptanceNormalState','Assert-LedgerSyncAcceptanceResourcesAbsent',
        'test:e2e:real-stack','LEDGERSYNC_SYSTEM_ISOLATED_PROJECT','REAL_STACK_BROWSER=PASS',
        'IncludeCapacity','run-capacity-qualification.ps1','CAPACITY=PASS','CAPACITY=SEPARATE_GATE','SECURITY_SCAN=SEPARATE_GATE'
    )) { Assert-AcceptanceTooling ($source.Contains($marker)) "Acceptance harness omitted required marker: $marker" }
    foreach ($forbidden in @('run-security-supply-chain-qualification.ps1','docker system prune','docker volume prune','docker network prune','TRANSFERS=','RECOVERY_EVIDENCE_JSON=','POSTGRES_PASSWORD=')) {
        Assert-AcceptanceTooling (-not $source.Contains($forbidden)) "Acceptance harness contains a forbidden broad/raw action: $forbidden"
    }
    Assert-AcceptanceTooling ($source -notmatch '(?m)^\s*param\([^)]*(Project|State|Backup)') 'Acceptance harness accepts caller-controlled project/state/backup targets.'
    $commonSource = Get-Content -LiteralPath $commonPath -Raw
    Assert-AcceptanceTooling ($commonSource -match 'client/harness boundary' -and $commonSource -match 'response-boundary') 'Acceptance helpers do not declare their safe boundary-loss simulations.'
    foreach ($marker in @('Idempotent-Replay','idempotency_conflict','exactly once','account.created.v1','no_delivery_attempts','unquoted field','ReadonlyRootfs','no-new-privileges:true','PortBindings','Internal')) {
        Assert-AcceptanceTooling ($commonSource.Contains($marker)) "Acceptance helper omitted required invariant marker: $marker"
    }
    [ordered]@{ status='passed'; parser_files=2; identity_cases=5; csv_cases=4; docker_executed=$false } | ConvertTo-Json -Compress
}
finally {
    if ($junctionPath -and (Test-Path -LiteralPath $junctionPath)) { Remove-Item -LiteralPath $junctionPath -Force }
    if (Test-Path -LiteralPath $testRoot) {
        $canonicalTemp = [IO.Path]::GetFullPath($temporaryParent).TrimEnd([IO.Path]::DirectorySeparatorChar) + [IO.Path]::DirectorySeparatorChar
        $canonicalTest = [IO.Path]::GetFullPath($testRoot)
        if (-not $canonicalTest.StartsWith($canonicalTemp,[StringComparison]::OrdinalIgnoreCase) -or (Split-Path -Leaf $canonicalTest) -cnotmatch '^ledgersync-acceptance-tooling-[0-9a-f]{32}$') { throw 'Test cleanup refused an unexpected path.' }
        Remove-Item -LiteralPath $canonicalTest -Recurse -Force
    }
}
