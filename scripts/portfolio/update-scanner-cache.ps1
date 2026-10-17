[CmdletBinding()]
param([switch]$TLSCompatibility)
$ErrorActionPreference='Stop'
$taskRepo=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '../..'))
$taskCache=Join-Path $taskRepo 'data/portfolio/r13-20261015/scanner-cache'
$taskStamp=Get-Date -AsUTC -Format 'yyyyMMddHHmmss'
$taskSuffix=[guid]::NewGuid().ToString('N').Substring(0,8)
$taskPhase=Join-Path $taskRepo ('data/portfolio/r17-'+(Get-Date -Format 'yyyyMMdd'))
$taskOutput=Join-Path $taskPhase ("scanner-update-$taskStamp-$taskSuffix")
$taskImage='aquasec/trivy@sha256:be1190afcb28352bfddc4ddeb71470835d16462af68d310f9f4bca710961a41e'
$taskContext=& docker context show
if($LASTEXITCODE -ne 0 -or $taskContext.Trim() -cne 'desktop-linux'){throw 'A running local desktop-linux Docker context is required. This command does not start Docker.'}
foreach($taskPath in @((Join-Path $taskRepo 'data'),(Join-Path $taskRepo 'data/portfolio'),(Split-Path $taskCache -Parent),$taskCache,$taskPhase)){
 if((Test-Path -LiteralPath $taskPath) -and ((Get-Item -LiteralPath $taskPath).Attributes -band [IO.FileAttributes]::ReparsePoint)){throw 'Scanner state cannot use a reparse point'}
}
if([IO.DriveInfo]::new([IO.Path]::GetPathRoot($taskRepo)).AvailableFreeSpace -lt 4GB){throw 'At least 4 GiB of free local disk is required for scanner staging'}
if(Test-Path -LiteralPath $taskOutput){throw 'Scanner update output already exists'}
New-Item -ItemType Directory -Path $taskCache -Force|Out-Null
New-Item -ItemType Directory -Path $taskOutput|Out-Null
$taskScratch=Join-Path $taskOutput 'scratch'
New-Item -ItemType Directory -Path $taskScratch|Out-Null
$taskMetadata=Join-Path $taskCache 'db/metadata.json'
if(Test-Path -LiteralPath $taskMetadata){Copy-Item -LiteralPath $taskMetadata -Destination (Join-Path $taskOutput 'metadata-before.json')}
# Keep public artifact extraction on disk. Do not allocate its archives in RAM.
$taskArguments=@('run','--rm','--name',"ledgersync-scanner-update-$taskStamp-$taskSuffix",'--label','ledgersync.purpose=portfolio-scanner-cache','--read-only','--cpus','1','--memory','1g','--pids-limit','128','--cap-drop','ALL','--security-opt','no-new-privileges','--env','TMPDIR=/scratch','--mount',"type=bind,source=$taskScratch,target=/scratch",'--mount',"type=bind,source=$taskCache,target=/cache",'--tmpfs','/tmp:rw,nosuid,size=32m')
# This option changes only public-download negotiation. Certificate checks stay on.
if($TLSCompatibility){$taskArguments+=@('--env','GODEBUG=tlsmlkem=0')}
$taskArguments+=@($taskImage,'image','--cache-dir','/cache','--download-db-only')
& docker @taskArguments *> (Join-Path $taskOutput 'download.txt')
if($LASTEXITCODE -ne 0){throw "Public advisory update failed. Retain evidence: $taskOutput"}
$taskAfter=Get-Content -LiteralPath $taskMetadata -Raw|ConvertFrom-Json
if($taskAfter.Version -ne 2 -or [datetimeoffset]$taskAfter.NextUpdate -le [datetimeoffset]::UtcNow){throw 'The advisory cache is absent, incompatible or stale'}
Copy-Item -LiteralPath $taskMetadata -Destination (Join-Path $taskOutput 'metadata-after.json')
[ordered]@{status='verified_local';purpose='public_advisory_cache';scanner=$taskImage;database_updated=$taskAfter.UpdatedAt;valid_until=$taskAfter.NextUpdate;memory_limit_mib=1024;cpu_limit=1;staging='private_local_disk';tls_compatibility=[bool]$TLSCompatibility;certificate_verification=$true;application_started=$false;cloud_execution=$false}|ConvertTo-Json|Set-Content -LiteralPath (Join-Path $taskOutput 'result.json')
Write-Output "Public advisory cache is current. Local evidence: $taskOutput"
