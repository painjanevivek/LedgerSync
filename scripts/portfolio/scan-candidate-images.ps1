[CmdletBinding()]
param([Parameter(Mandatory=$true)][string]$CandidateRoot)
$ErrorActionPreference='Stop'
$taskRepo=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '../..'))
$taskCandidate=[IO.Path]::GetFullPath($CandidateRoot)
$taskAllowed=Join-Path $taskRepo 'data/portfolio'
$taskRelative=[IO.Path]::GetRelativePath($taskAllowed,$taskCandidate).Replace('\','/')
if($taskRelative -notmatch '^r1[567]-\d{8}/candidate-\d{14}-[a-f0-9]{8}$'){throw 'Exact current-phase private candidate path required'}
$taskCandidateResult=Get-Content -LiteralPath (Join-Path $taskCandidate 'result.json') -Raw|ConvertFrom-Json
if($taskCandidateResult.status -cne 'verified_local' -or !$taskCandidateResult.source_preserved -or !$taskCandidateResult.normal_preserved){throw 'Verified preserved candidate required'}
$taskStamp=Get-Date -AsUTC -Format 'yyyyMMddHHmmss'
$taskScanRoot=Join-Path (Split-Path $taskCandidate -Parent) ('scans-'+(Split-Path $taskCandidate -Leaf).Substring(10)+'-'+$taskStamp)
if(Test-Path -LiteralPath $taskScanRoot){throw 'Scan output already exists'}
New-Item -ItemType Directory -Path $taskScanRoot|Out-Null
# Large public advisory/image archives use disk staging, not scarce host RAM.
$taskScratch=Join-Path $taskScanRoot 'scratch'
New-Item -ItemType Directory -Path $taskScratch|Out-Null
$taskCache=Join-Path $taskRepo 'data/portfolio/r13-20261015/scanner-cache'
$taskMetadata=Get-Content -LiteralPath (Join-Path $taskCache 'db/metadata.json') -Raw|ConvertFrom-Json
if([datetimeoffset]$taskMetadata.NextUpdate -lt [datetimeoffset]::UtcNow){throw 'Scanner database is stale. Update the pinned scanner cache before qualification.'}
$taskScanner='aquasec/trivy@sha256:be1190afcb28352bfddc4ddeb71470835d16462af68d310f9f4bca710961a41e'
$taskResults=@()
foreach($taskService in @('api','outbox-worker','migrate','web')){
 $taskImage=$taskCandidateResult.images.$taskService
 $taskInspectRaw=& docker image inspect $taskImage.id
 if($LASTEXITCODE -ne 0){throw 'Candidate image inspection failed'}
 $taskInspect=($taskInspectRaw|ConvertFrom-Json)[0]
 if($taskInspect.Id -cne $taskImage.id){throw 'Candidate image identity differs'}
 if($taskInspect.Size -le 0 -or $taskInspect.Size -gt 2GB){throw 'Image exceeds the bounded local scan size'}
 $taskFree=[IO.DriveInfo]::new([IO.Path]::GetPathRoot($taskScanRoot)).AvailableFreeSpace
 if($taskFree -lt [math]::Max(4GB,3*[long]$taskInspect.Size+1GB)){throw 'Insufficient free disk for isolated scanner staging'}
 $taskArchive=Join-Path $taskScanRoot "$taskService.tar"
 & docker save --output $taskArchive $taskImage.id
 if($LASTEXITCODE -ne 0){throw 'Image archive failed'}
 $taskArguments=@('run','--rm','--network','none','--read-only','--cpus','1','--memory','1g','--env','TMPDIR=/scratch','--mount',"type=bind,source=$taskScratch,target=/scratch",'--mount',"type=bind,source=$taskCache,target=/cache",'--mount',"type=bind,source=$taskScanRoot,target=/evidence",'--tmpfs','/tmp:rw,nosuid,size=128m',$taskScanner,'image','--cache-dir','/cache','--input',"/evidence/$taskService.tar",'--skip-db-update','--offline-scan')
 & docker @taskArguments --scanners vuln --severity HIGH,CRITICAL --format json --output "/evidence/$taskService-vulnerabilities.json" --exit-code 1 *> (Join-Path $taskScanRoot "$taskService-scan.txt")
 $taskVulnerabilityExit=$LASTEXITCODE
 & docker @taskArguments --format cyclonedx --output "/evidence/$taskService-sbom.cdx.json" *> (Join-Path $taskScanRoot "$taskService-sbom.txt")
 $taskSBOMExit=$LASTEXITCODE
 $taskReport=Get-Content -LiteralPath (Join-Path $taskScanRoot "$taskService-vulnerabilities.json") -Raw|ConvertFrom-Json
 $taskSBOM=Get-Content -LiteralPath (Join-Path $taskScanRoot "$taskService-sbom.cdx.json") -Raw|ConvertFrom-Json
 $taskDiffs=@($taskReport.Metadata.DiffIDs)
 if(($taskDiffs -join ',') -cne (@($taskInspect.RootFS.Layers) -join ',')){throw 'Scanned layers differ from the candidate image'}
 $taskSBOMImage=@($taskSBOM.metadata.component.properties|Where-Object name -CEQ 'aquasecurity:trivy:ImageID')
 if($taskSBOMImage.Count -ne 1 -or $taskSBOMImage[0].value -cne $taskReport.Metadata.ImageID){throw 'SBOM and vulnerability image configuration differ'}
 $taskResults+=[pscustomobject]@{service=$taskService;image_id=$taskImage.id;image_config_id=$taskReport.Metadata.ImageID;archive_sha256=(Get-FileHash -LiteralPath $taskArchive -Algorithm SHA256).Hash.ToLowerInvariant();vulnerability_exit=$taskVulnerabilityExit;sbom_exit=$taskSBOMExit;layers_verified=$true;scanner=$taskScanner;scanner_database_updated=$taskMetadata.UpdatedAt}
}
$taskResults|ConvertTo-Json -Depth 8|Set-Content -LiteralPath (Join-Path $taskScanRoot 'results.json')
if(@($taskResults|Where-Object {$_.vulnerability_exit -ne 0 -or $_.sbom_exit -ne 0}).Count){throw 'Image checks require review. Keep all results.'}
Write-Output "Verified four candidate images serially. Evidence: $taskScanRoot"
