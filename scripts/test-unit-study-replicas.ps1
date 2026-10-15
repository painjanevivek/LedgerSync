. (Join-Path $PSScriptRoot 'unit-study-common.ps1')
$probe = @'
const dns = require('node:dns/promises');
(async () => {
  const addresses = await dns.lookup('api', {all:true});
  const results = [];
  for (const {address} of addresses) {
    const r = await fetch(`http://${address}:8080/readyz`, {signal:AbortSignal.timeout(3000)});
    if (!r.ok) throw new Error('API instance is not ready');
    const node = r.headers.get('x-ledgersync-node');
    if (!node) throw new Error('API identity is unavailable');
    results.push({node, status:r.status});
  }
  if (new Set(results.map(r=>r.node)).size !== 2) throw new Error('Expected two unique ready APIs');
  console.log(JSON.stringify({captured_at:new Date().toISOString(), replicas:results}));
})().catch(()=>{console.error('Replica readiness or identity check failed');process.exit(1)});
'@
$result = @(Invoke-LedgerSyncCompose -ComposeArguments @('exec','-T','web','node','-e',$probe) -CaptureOutput)
$result | Set-Content -LiteralPath (Join-Path $studyRoot 'docs/release-evidence/unit-3-unit-4/replica-readiness.json')
Invoke-LedgerSyncWebSmoke
$session = [Microsoft.PowerShell.Commands.WebRequestSession]::new()
Initialize-LedgerSyncLocalWebSession -Session $session
$identity = Invoke-LedgerSyncAcceptanceGET -Session $session -Path '/api/session'
Test-LedgerSyncAcceptanceJourney -Session $session -CSRFToken $identity.csrf_token -AmountMinor '100' -VerifyReplay | Out-Null
Invoke-LedgerSyncAcceptanceReconciliation -TenantID '00000000-0000-4000-8000-000000000001' | Out-Null
Write-Host 'Two unique API instances are ready. Session, transfer replay, and reconciliation checks passed.'
