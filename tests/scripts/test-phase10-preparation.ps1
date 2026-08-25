[CmdletBinding()]
param()

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$phase10Repository = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$phase10CapacityScript = Join-Path $phase10Repository 'scripts/run-capacity-qualification.ps1'
$phase10SecurityScript = Join-Path $phase10Repository 'scripts/run-security-supply-chain-qualification.ps1'

foreach ($phase10Script in @($phase10CapacityScript, $phase10SecurityScript)) {
    $phase10ParseErrors = $null
    [System.Management.Automation.Language.Parser]::ParseFile($phase10Script, [ref]$null, [ref]$phase10ParseErrors) | Out-Null
    if ($phase10ParseErrors.Count -ne 0) {
        throw "PowerShell parse failed for $phase10Script`: $($phase10ParseErrors -join '; ')"
    }
}

$phase10Capacity = & $phase10CapacityScript -ValidateOnly | ConvertFrom-Json
if ($phase10Capacity.executes_load -ne $false -or
    $phase10Capacity.offered_tps -ne 25 -or
    $phase10Capacity.duration_seconds -ne 300 -or
    $phase10Capacity.expected_transfer_iterations -ne 7500 -or
    $phase10Capacity.expected_control_iterations -ne 5) {
    throw "The default capacity contract drifted: $($phase10Capacity | ConvertTo-Json -Compress)"
}
if ($phase10Capacity.k6_image -notmatch '@sha256:[0-9a-f]{64}$') {
    throw 'The k6 qualification image is not digest-pinned.'
}

$phase10Hot = & $phase10CapacityScript -ValidateOnly -WorkloadShape hot -TransactionsPerSecond 7 -Duration 12s | ConvertFrom-Json
if ($phase10Hot.expected_transfer_iterations -ne 84 -or $phase10Hot.expected_control_iterations -ne 0) {
    throw 'Capacity duration/count calculation is not deterministic for non-default workloads.'
}

$phase10Security = & $phase10SecurityScript -Mode Validate | ConvertFrom-Json
if ($phase10Security.clean_commit_required_for_execution -ne $true -or $phase10Security.source_checks.Count -lt 8 -or $phase10Security.image_checks.Count -lt 4) {
    throw 'The security qualification plan is incomplete or no longer clean-commit-bound.'
}
foreach ($phase10Scanner in @($phase10Security.pinned_tools.gitleaks_image, $phase10Security.pinned_tools.trivy_image)) {
    if ($phase10Scanner -notmatch '@sha256:[0-9a-f]{64}$') { throw "Scanner is not digest-pinned: $phase10Scanner" }
}
if ($phase10Security.pinned_tools.govulncheck -ne 'golang.org/x/vuln/cmd/govulncheck@v1.7.0') {
    throw 'govulncheck version drifted.'
}
if ($phase10Security.source_checks -notcontains 'npm audit --omit=dev --audit-level=high') {
    throw 'The local npm gate must remain scoped to production dependencies and match CI.'
}

$phase10K6Source = Get-Content -LiteralPath (Join-Path $phase10Repository 'tests/performance/k6/transfers.js') -Raw
foreach ($phase10Marker in @(
    'exec: "transferTraffic"',
    'exec: "lowRateControls"',
    'executor: "per-vu-iterations"',
    'iterations: durationSeconds',
    'controlIntervalSeconds',
    'ledgersync_transfer_iterations',
    'ledgersync_control_iterations',
    '/api/local/diagnostics',
    '/api/events?limit=10',
    '/api/reconciliation/runs',
    'available_minor === "0"',
    'Idempotent-Replay'
)) {
    if (-not $phase10K6Source.Contains($phase10Marker)) { throw "k6 mixed-workload control is missing: $phase10Marker" }
}

$phase10CapacitySource = Get-Content -LiteralPath $phase10CapacityScript -Raw
foreach ($phase10Marker in @(
    'expected_transfer_iterations',
    'expected_control_iterations',
    'durable_movement_delta',
    'projection_ledger_drift',
    'new_reconciliation_mismatches',
    '--network host',
    'LEDGERSYNC_PERF_BFF_URL=http://127.0.0.1:3000',
    'PostgreSQL reported deadlocks',
    'outbox delivery was not fully drained'
)) {
    if (-not $phase10CapacitySource.Contains($phase10Marker)) { throw "capacity evidence gate is missing: $phase10Marker" }
}

$phase10SecurityWorkflow = Get-Content -LiteralPath (Join-Path $phase10Repository '.github/workflows/security.yml') -Raw
foreach ($phase10Marker in @(
    'image: ledgersync-api:${{ github.sha }}',
    'image: ledgersync-worker:${{ github.sha }}',
    'image: ledgersync-web:${{ github.sha }}',
    'subject-path: api.spdx.json',
    'subject-path: worker.spdx.json',
    'subject-path: web.spdx.json'
)) {
    if (-not $phase10SecurityWorkflow.Contains($phase10Marker)) { throw "per-image SBOM/provenance workflow is missing: $phase10Marker" }
}

[ordered]@{
    status = 'passed'
    capacity_default = [ordered]@{ tps = 25; seconds = 300; transfers = 7500; controls = 5 }
    security_source_checks = $phase10Security.source_checks.Count
    security_image_checks = $phase10Security.image_checks.Count
} | ConvertTo-Json -Compress
