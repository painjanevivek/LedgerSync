[CmdletBinding()]
param([switch]$Reveal)

. (Join-Path $PSScriptRoot "local-runtime-common.ps1")
. (Join-Path $PSScriptRoot "local-api-credential-common.ps1")

try {
    $result = Get-LedgerSyncPrivateAPICredentialOutput `
        -Path $script:LedgerSyncRuntimeEnvironmentFile `
        -Reveal:$Reveal
    if ($Reveal) {
        Write-Output ([string]$result)
    } else {
        $result | Format-List Name, Bytes, Fingerprint, Protected, LastWriteTimeUtc
    }
}
catch {
    Write-Error "Local private API credential inspection failed. No credential was disclosed."
    exit 1
}
