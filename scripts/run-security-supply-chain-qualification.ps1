[CmdletBinding()]
param(
    [ValidateSet('Validate', 'Source', 'Images', 'All')]
    [string]$Mode = 'Validate',

    [ValidatePattern('^[0-9a-f]{40}$')]
    [string]$ExpectedCommit = '',

    [ValidatePattern('^[a-z0-9][a-z0-9._-]{0,63}$')]
    [string]$ImagePrefix = 'ledgersync-phase10',

    [string]$OutputPath = '',

    [string]$GitleaksImage = 'zricethezav/gitleaks@sha256:691af3c7c5a48b16f187ce3446d5f194838f91238f27270ed36eef6359a574d9',

    [string]$TrivyImage = 'aquasec/trivy@sha256:bcc376de8d77cfe086a917230e818dc9f8528e3c852f7b1aff648949b6258d1c'
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$securityRepository = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$securityHead = (& git -C $securityRepository rev-parse HEAD).Trim()
if ($LASTEXITCODE -ne 0 -or $securityHead -notmatch '^[0-9a-f]{40}$') { throw 'Unable to resolve the source commit.' }
$securityIsShallow = ((& git -C $securityRepository rev-parse --is-shallow-repository).Trim() -eq 'true')

function Assert-SecurityPinnedImage {
    param([Parameter(Mandatory)][string]$Reference)
    if ($Reference -notmatch '^[a-z0-9./_-]+@sha256:[0-9a-f]{64}$') {
        throw "Scanner image is not digest-pinned: $Reference"
    }
    & docker image inspect $Reference *> $null
    if ($LASTEXITCODE -ne 0) {
        throw "Required pinned scanner image is not already available locally: $Reference"
    }
}

function Invoke-SecurityNativeStep {
    param(
        [Parameter(Mandatory)][string]$Name,
        [Parameter(Mandatory)][string]$Executable,
        [Parameter(Mandatory)][string[]]$Arguments,
        [Parameter(Mandatory)][string]$WorkingDirectory
    )
    $securityStarted = (Get-Date).ToUniversalTime()
    Push-Location $WorkingDirectory
    try {
        & $Executable @Arguments
        $securityExitCode = $LASTEXITCODE
    }
    finally {
        Pop-Location
    }
    if ($securityExitCode -ne 0) { throw "$Name failed with exit code $securityExitCode." }
    $script:securitySteps.Add([ordered]@{
        name = $Name
        status = 'passed'
        command = "$Executable $($Arguments -join ' ')"
        started_at = $securityStarted.ToString('O')
        completed_at = (Get-Date).ToUniversalTime().ToString('O')
    })
}

foreach ($securityPinnedReference in @($GitleaksImage, $TrivyImage)) {
    if ($securityPinnedReference -notmatch '^[a-z0-9./_-]+@sha256:[0-9a-f]{64}$') {
        throw "Scanner image is not digest-pinned: $securityPinnedReference"
    }
}

$securityPlan = [ordered]@{
    schema_version = 1
    mode = $Mode
    source_commit = $securityHead
    local_history_scope = if ($securityIsShallow) { 'available_history_plus_ci_full_history' } else { 'full_history' }
    clean_commit_required_for_execution = $true
    pinned_tools = [ordered]@{
        govulncheck = 'golang.org/x/vuln/cmd/govulncheck@v1.7.0'
        gitleaks_image = $GitleaksImage
        trivy_image = $TrivyImage
    }
    source_checks = @(
        'go test ./... -count=1',
        'go vet ./...',
        'go test -run=^$ -fuzz=FuzzParseExactMoney -fuzztime=10s ./tests/unit',
        'go test -race ./cmd/... ./internal/... ./tests/unit ./tests/contract (when local CGO race support is available; otherwise pinned Linux CI is mandatory)',
        'go run golang.org/x/vuln/cmd/govulncheck@v1.7.0 ./...',
        'npm audit --omit=dev --audit-level=high',
        'digest-pinned gitleaks git --redact over full history',
        'digest-pinned Trivy config scan over deploy/'
    )
    image_checks = @(
        'build API, worker, and web images tagged with the exact source commit',
        'digest-pinned Trivy HIGH/CRITICAL image scan from bounded saved archives',
        'SPDX JSON SBOM per image',
        'local commit/image/SBOM hash provenance record; signed CI attestation remains authoritative'
    )
}

if ($Mode -eq 'Validate') {
    $securityPlan | ConvertTo-Json -Depth 5 -Compress
    return
}

Assert-SecurityPinnedImage -Reference $GitleaksImage
Assert-SecurityPinnedImage -Reference $TrivyImage

if ([string]::IsNullOrWhiteSpace($ExpectedCommit) -or $ExpectedCommit -ne $securityHead) {
    throw 'Execution requires -ExpectedCommit equal to the exact current 40-character HEAD.'
}
$securityDirty = @(& git -C $securityRepository status --porcelain=v1 --untracked-files=all)
if ($LASTEXITCODE -ne 0 -or $securityDirty.Count -ne 0) {
    throw 'Security qualification executes only from a clean commit, including no untracked files.'
}

$securityEvidenceRoot = [System.IO.Path]::GetFullPath((Join-Path $securityRepository ".tmp/security-phase10/$securityHead"))
if ([string]::IsNullOrWhiteSpace($OutputPath)) { $OutputPath = Join-Path $securityEvidenceRoot 'qualification.json' }
$securityOutput = [System.IO.Path]::GetFullPath($OutputPath)
$securityPrefix = $securityEvidenceRoot.TrimEnd('\') + '\'
if (-not $securityOutput.StartsWith($securityPrefix, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw 'Security evidence output must remain under .tmp/security-phase10/<commit>.'
}
New-Item -ItemType Directory -Force -Path $securityEvidenceRoot | Out-Null
$securitySteps = [System.Collections.Generic.List[object]]::new()
$securityRaceDisposition = 'not_requested'

if ($Mode -eq 'Source' -or $Mode -eq 'All') {
    Invoke-SecurityNativeStep -Name 'full Go tests' -Executable 'go' -Arguments @('test', './...', '-count=1') -WorkingDirectory $securityRepository
    Invoke-SecurityNativeStep -Name 'Go vet' -Executable 'go' -Arguments @('vet', './...') -WorkingDirectory $securityRepository
    $securityCoveragePath = (Join-Path $securityEvidenceRoot 'critical-coverage.out')
    Invoke-SecurityNativeStep -Name 'critical financial coverage' -Executable 'go' -Arguments @('test', '-count=1', '-covermode=atomic', '-coverpkg=./internal/domain/...,./internal/application/transfers/...', "-coverprofile=$securityCoveragePath", './internal/domain/...', './internal/application/transfers/...', './tests/unit', './tests/contract') -WorkingDirectory $securityRepository
    $securityCoverageSummary = @(& go tool cover "-func=$securityCoveragePath")
    if ($LASTEXITCODE -ne 0) { throw 'Unable to evaluate critical financial coverage.' }
    $securityCoverageSummary | Write-Output
    $securityCoverageTotal = @($securityCoverageSummary | Where-Object { [string]$_ -match '^total:\s+\(statements\)\s+([0-9]+(?:\.[0-9]+)?)%$' } | Select-Object -Last 1)
    if ($securityCoverageTotal.Count -ne 1 -or [string]$securityCoverageTotal[0] -notmatch '^total:\s+\(statements\)\s+([0-9]+(?:\.[0-9]+)?)%$' -or [double]$Matches[1] -lt 60) {
        throw 'Critical financial coverage is below the required 60 percent floor or could not be parsed.'
    }
    Invoke-SecurityNativeStep -Name 'exact-money fuzz' -Executable 'go' -Arguments @('test', '-run=^$', '-fuzz=FuzzParseExactMoney', '-fuzztime=10s', './tests/unit') -WorkingDirectory $securityRepository

    $securityCGO = (& go env CGO_ENABLED).Trim()
    if ($LASTEXITCODE -eq 0 -and $securityCGO -eq '1') {
        Invoke-SecurityNativeStep -Name 'critical race test' -Executable 'go' -Arguments @('test', '-race', './cmd/...', './internal/...', './tests/unit', './tests/contract') -WorkingDirectory $securityRepository
        $securityRaceDisposition = 'passed_locally'
    }
    else {
        $securityWorkflow = Get-Content -LiteralPath (Join-Path $securityRepository '.github/workflows/quality.yml') -Raw
        if ($securityWorkflow -notmatch [regex]::Escape('go test -race ./cmd/... ./internal/... ./tests/unit ./tests/contract')) {
            throw 'Local race support is unavailable and the pinned Linux CI race gate is missing.'
        }
        $securityRaceDisposition = 'delegated_to_pinned_linux_ci_local_cgo_unavailable'
    }

    Invoke-SecurityNativeStep -Name 'Go vulnerability scan' -Executable 'go' -Arguments @('run', 'golang.org/x/vuln/cmd/govulncheck@v1.7.0', './...') -WorkingDirectory $securityRepository
    Invoke-SecurityNativeStep -Name 'npm production dependency audit' -Executable 'npm' -Arguments @('audit', '--omit=dev', '--audit-level=high') -WorkingDirectory (Join-Path $securityRepository 'web')

    if ($securityIsShallow) {
        $securityWorkflow = Get-Content -LiteralPath (Join-Path $securityRepository '.github/workflows/security.yml') -Raw
        if ($securityWorkflow -notmatch 'fetch-depth:\s*0') {
            throw 'Local history is shallow and the authoritative CI secret scan does not fetch full history.'
        }
    }
    $securityDockerRepository = $securityRepository.Replace('\', '/')
    Invoke-SecurityNativeStep -Name 'history-aware secret scan' -Executable 'docker' -Arguments @(
        'run', '--rm', '--mount', "type=bind,source=$securityDockerRepository,destination=/repo,readonly", '-w', '/repo',
        $GitleaksImage, 'git', '--redact', '--no-banner', '--timeout', '300', '--exit-code', '1', '/repo'
    ) -WorkingDirectory $securityRepository
    Invoke-SecurityNativeStep -Name 'infrastructure configuration scan' -Executable 'docker' -Arguments @(
        'run', '--rm', '--mount', "type=bind,source=$securityDockerRepository,destination=/workspace,readonly", '-w', '/workspace',
        $TrivyImage, 'config', '--cache-dir', '/tmp/trivy-cache', '--severity', 'HIGH,CRITICAL', '--exit-code', '1', '/workspace/deploy'
    ) -WorkingDirectory $securityRepository
}

$securityImages = [ordered]@{}
if ($Mode -eq 'Images' -or $Mode -eq 'All') {
    $securityImageDefinitions = @(
        [ordered]@{ name = 'api'; dockerfile = 'deploy/docker/api.Dockerfile' },
        [ordered]@{ name = 'worker'; dockerfile = 'deploy/docker/outbox-worker.Dockerfile' },
        [ordered]@{ name = 'web'; dockerfile = 'deploy/docker/web.Dockerfile' }
    )
    $securityDockerEvidence = $securityEvidenceRoot.Replace('\', '/')
    foreach ($securityImageDefinition in $securityImageDefinitions) {
        $securityName = $securityImageDefinition.name
        $securityTag = "$ImagePrefix-$securityName`:$securityHead"
        $securityDockerfilePath = Join-Path $securityRepository $securityImageDefinition.dockerfile
        $securityUnpinnedBase = @(Get-Content -LiteralPath $securityDockerfilePath | Where-Object { $_ -match '^FROM\s+' -and $_ -notmatch '@sha256:[0-9a-f]{64}(\s+AS\s+\S+)?$' })
        if ($securityUnpinnedBase.Count -ne 0) { throw "$($securityImageDefinition.dockerfile) contains an unpinned base image." }
        Invoke-SecurityNativeStep -Name "build $securityName image" -Executable 'docker' -Arguments @('build', '--pull=false', '--file', $securityImageDefinition.dockerfile, '--tag', $securityTag, '.') -WorkingDirectory $securityRepository
        $securityArchive = Join-Path $securityEvidenceRoot "$securityName-image.tar"
        Invoke-SecurityNativeStep -Name "save $securityName image" -Executable 'docker' -Arguments @('save', '--output', $securityArchive, $securityTag) -WorkingDirectory $securityRepository
        $securitySBOM = Join-Path $securityEvidenceRoot "$securityName.spdx.json"
        Invoke-SecurityNativeStep -Name "generate $securityName SBOM" -Executable 'docker' -Arguments @(
            'run', '--rm', '--mount', "type=bind,source=$securityDockerEvidence,destination=/evidence", $TrivyImage,
            'image', '--input', "/evidence/$securityName-image.tar", '--format', 'spdx-json', '--output', "/evidence/$securityName.spdx.json"
        ) -WorkingDirectory $securityRepository
        $securityScan = Join-Path $securityEvidenceRoot "$securityName-vulnerabilities.json"
        Invoke-SecurityNativeStep -Name "scan $securityName image" -Executable 'docker' -Arguments @(
            'run', '--rm', '--mount', "type=bind,source=$securityDockerEvidence,destination=/evidence", $TrivyImage,
            'image', '--input', "/evidence/$securityName-image.tar", '--cache-dir', '/tmp/trivy-cache', '--severity', 'HIGH,CRITICAL', '--exit-code', '1', '--format', 'json', '--output', "/evidence/$securityName-vulnerabilities.json"
        ) -WorkingDirectory $securityRepository
        $securityImageID = (& docker image inspect --format '{{.Id}}' $securityTag).Trim()
        if ($LASTEXITCODE -ne 0 -or $securityImageID -notmatch '^sha256:[0-9a-f]{64}$') { throw "Unable to resolve immutable image ID for $securityTag." }
        $securityImages[$securityName] = [ordered]@{
            tag = $securityTag
            image_id = $securityImageID
            dockerfile_sha256 = (Get-FileHash -LiteralPath (Join-Path $securityRepository $securityImageDefinition.dockerfile) -Algorithm SHA256).Hash.ToLowerInvariant()
            archive_sha256 = (Get-FileHash -LiteralPath $securityArchive -Algorithm SHA256).Hash.ToLowerInvariant()
            sbom_sha256 = (Get-FileHash -LiteralPath $securitySBOM -Algorithm SHA256).Hash.ToLowerInvariant()
            vulnerability_report_sha256 = (Get-FileHash -LiteralPath $securityScan -Algorithm SHA256).Hash.ToLowerInvariant()
        }
    }
}

$securityEvidence = [ordered]@{
    schema_version = 1
    decision = 'pass'
    source_commit = $securityHead
    local_history_scope = if ($securityIsShallow) { 'available_history_plus_ci_full_history' } else { 'full_history' }
    generated_at = (Get-Date).ToUniversalTime().ToString('O')
    mode = $Mode
    race_disposition = $securityRaceDisposition
    scanners = $securityPlan.pinned_tools
    steps = @($securitySteps)
    images = $securityImages
    provenance = [ordered]@{
        kind = 'local-commit-bound-build-record'
        signed = $false
        authoritative_signed_attestation = '.github/workflows/security.yml'
    }
}
$securityEvidence | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $securityOutput -Encoding utf8NoBOM
$securityEvidence | ConvertTo-Json -Depth 5 -Compress
