[CmdletBinding()]
param([string]$Pattern = '.', [string[]]$Packages = @('./tests/integration'), [ValidatePattern('^[a-z0-9-]+$')][string]$Label = 'integration')
. (Join-Path $PSScriptRoot 'unit-study-common.ps1')
Initialize-LedgerSyncLocalSecrets
Invoke-LedgerSyncCompose -ComposeArguments @('--profile','test-dependencies','up','-d','--wait','test-postgres','test-redis')
# These dependencies have separate volumes. Integration tests reset their schema.
# Never pass the running study ledger or normal database to that harness.
$password = Get-LedgerSyncRuntimeEnvironmentValue -Path $script:LedgerSyncRuntimeEnvironmentFile -Name 'POSTGRES_PASSWORD'
$env:LEDGERSYNC_TEST_DATABASE_URL = "postgres://ledgersync:$password@127.0.0.1:35432/ledgersync_test?sslmode=disable"
$env:LEDGERSYNC_TEST_REDIS_ADDR = '127.0.0.1:36379'
try {
    & go run ./cmd/study-test-setup
    if ($LASTEXITCODE -ne 0) { throw 'Isolated test setup failed.' }
    $evidencePath = Join-Path $studyRoot "docs/release-evidence/unit-3-unit-4/$Label-$([DateTime]::UtcNow.ToString('yyyyMMddTHHmmssZ')).txt"
    & go test -p 1 -count=1 -v @Packages -run $Pattern 2>&1 | Tee-Object -FilePath $evidencePath
    if ($LASTEXITCODE -ne 0) { throw "Go study checks failed: $Label" }
} finally {
    Remove-Item Env:LEDGERSYNC_TEST_DATABASE_URL -ErrorAction SilentlyContinue
    Remove-Item Env:LEDGERSYNC_TEST_REDIS_ADDR -ErrorAction SilentlyContinue
}
