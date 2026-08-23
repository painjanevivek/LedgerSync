[CmdletBinding()]
param(
    [string]$ComposeProject = "ledgersync-system",
    [string]$TenantId = "00000000-0000-4000-8000-000000000001"
)

$ErrorActionPreference = "Stop"
$PSNativeCommandUseErrorActionPreference = $true

$suffix = (Get-Date).ToUniversalTime().ToString("yyyyMMddHHmmss")
$sourcePostgres = "$ComposeProject-postgres-1"
$restorePostgres = "ledgersync-restore-pg-$suffix"
$restoreRedis = "ledgersync-restore-redis-$suffix"
$restoreNetwork = "ledgersync-restore-net-$suffix"
$restoreVolume = "ledgersync_restore_pg_$suffix"
$apiImage = "$ComposeProject-api"
$temporaryRoot = Join-Path ([IO.Path]::GetTempPath()) "ledgersync-restore-$suffix"
$dumpPath = Join-Path $temporaryRoot "ledgersync.dump"
$startedAt = [DateTimeOffset]::UtcNow

function Test-DockerObject {
    param(
        [Parameter(Mandatory)]
        [ValidateSet("container", "network", "volume")]
        [string]$Kind,
        [Parameter(Mandatory)]
        [string]$Name
    )

    docker $Kind inspect $Name *> $null
    return $LASTEXITCODE -eq 0
}

try {
    if (Test-Path -LiteralPath $temporaryRoot) {
        throw "Refusing to reuse existing temporary path: $temporaryRoot"
    }

    New-Item -ItemType Directory -Path $temporaryRoot | Out-Null
    docker exec $sourcePostgres pg_dump -U ledgersync -d ledgersync -Fc -f /tmp/ledgersync-restore.dump
    docker cp "${sourcePostgres}:/tmp/ledgersync-restore.dump" $dumpPath | Out-Null

    docker network create --internal $restoreNetwork | Out-Null
    docker volume create $restoreVolume | Out-Null
    docker run -d --name $restorePostgres --network $restoreNetwork `
        -v "${restoreVolume}:/var/lib/postgresql/data" `
        -e POSTGRES_DB=ledgersync `
        -e POSTGRES_USER=ledgersync `
        -e POSTGRES_PASSWORD=restore-drill-only `
        postgres:16-alpine | Out-Null
    docker run -d --name $restoreRedis --network $restoreNetwork `
        redis:7.4-alpine redis-server --appendonly yes | Out-Null

    $ready = $false
    for ($attempt = 0; $attempt -lt 30; $attempt++) {
        $PSNativeCommandUseErrorActionPreference = $false
        docker exec $restorePostgres pg_isready -U ledgersync -d ledgersync *> $null
        $readinessExitCode = $LASTEXITCODE
        $PSNativeCommandUseErrorActionPreference = $true
        if ($readinessExitCode -eq 0) {
            $ready = $true
            break
        }
        Start-Sleep -Seconds 1
    }
    if (-not $ready) {
        throw "Restore PostgreSQL did not become ready within 30 seconds."
    }

    docker cp $dumpPath "${restorePostgres}:/tmp/ledgersync.dump" | Out-Null
    docker exec $restorePostgres pg_restore -U ledgersync -d ledgersync `
        --no-owner --exit-on-error /tmp/ledgersync.dump

    $databaseUrl = "postgres://ledgersync:restore-drill-only@${restorePostgres}:5432/ledgersync?sslmode=disable"
    docker run --rm --network $restoreNetwork `
        -e LEDGERSYNC_ENV=development `
        -e "LEDGERSYNC_DATABASE_URL=$databaseUrl" `
        --entrypoint /usr/local/bin/migrate $apiImage

    $reconciliationOutput = docker run --rm --network $restoreNetwork `
        -e LEDGERSYNC_ENV=development `
        -e "LEDGERSYNC_DATABASE_URL=$databaseUrl" `
        -e "LEDGERSYNC_REDIS_ADDR=${restoreRedis}:6379" `
        --entrypoint /usr/local/bin/reconcile $apiImage `
        --run --rebuild-cache --tenant-id $TenantId 2>&1

    $databaseEvidence = docker exec $restorePostgres psql -U ledgersync -d ledgersync `
        -At -F "|" -c "SELECT (SELECT count(*) FROM schema_migrations),(SELECT count(*) FROM accounts),(SELECT count(*) FROM transfers WHERE status='posted'),(SELECT status FROM reconciliation_runs ORDER BY started_at DESC LIMIT 1),(SELECT mismatch_count FROM reconciliation_runs ORDER BY started_at DESC LIMIT 1),(SELECT id FROM reconciliation_runs ORDER BY started_at DESC LIMIT 1);"
    $redisKeyCount = docker exec $restoreRedis redis-cli DBSIZE
    $elapsedSeconds = [Math]::Round(([DateTimeOffset]::UtcNow - $startedAt).TotalSeconds, 2)
    $dumpBytes = (Get-Item -LiteralPath $dumpPath).Length

    Write-Output "RESTORE_DRILL=PASS"
    Write-Output "DUMP_BYTES=$dumpBytes"
    Write-Output "DB_EVIDENCE=$databaseEvidence"
    Write-Output "REDIS_DBSIZE=$redisKeyCount"
    Write-Output "LOCAL_RTO_SECONDS=$elapsedSeconds"
    Write-Output "RECONCILE=$($reconciliationOutput -join ' ')"
}
finally {
    $PSNativeCommandUseErrorActionPreference = $false
    if (Test-DockerObject -Kind container -Name $restorePostgres) {
        docker container rm --force $restorePostgres | Out-Null
    }
    if (Test-DockerObject -Kind container -Name $restoreRedis) {
        docker container rm --force $restoreRedis | Out-Null
    }
    if (Test-DockerObject -Kind volume -Name $restoreVolume) {
        docker volume rm $restoreVolume | Out-Null
    }
    if (Test-DockerObject -Kind network -Name $restoreNetwork) {
        docker network rm $restoreNetwork | Out-Null
    }

    $resolvedTemporaryRoot = [IO.Path]::GetFullPath($temporaryRoot)
    $expectedTemporaryRoot = [IO.Path]::GetFullPath(
        (Join-Path ([IO.Path]::GetTempPath()) "ledgersync-restore-$suffix")
    )
    if ((Test-Path -LiteralPath $resolvedTemporaryRoot) -and
        $resolvedTemporaryRoot -eq $expectedTemporaryRoot) {
        Remove-Item -LiteralPath $resolvedTemporaryRoot -Recurse -Force
    }
    Write-Output "CLEANUP=COMPLETE"
}
