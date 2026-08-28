[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$InputRoot,

    [Parameter(Mandatory = $true)]
    [string]$OutputDirectory,

    [Parameter(Mandatory = $true)]
    [string]$Prefix,

    [Parameter(Mandatory = $true)]
    [ValidateRange(1, 500)]
    [int]$ExpectedCount,

    [string]$Filter = "*.png"
)

$ErrorActionPreference = "Stop"
Add-Type -AssemblyName System.Drawing

$inputPath = (Resolve-Path -LiteralPath $InputRoot).Path
$files = @(Get-ChildItem -LiteralPath $inputPath -Recurse -File -Filter $Filter | Sort-Object Name, FullName)
if ($files.Count -ne $ExpectedCount) {
    throw "Expected $ExpectedCount screenshots under $inputPath, found $($files.Count)."
}

New-Item -ItemType Directory -Path $OutputDirectory -Force | Out-Null
$outputPath = (Resolve-Path -LiteralPath $OutputDirectory).Path
$tileWidth = 400
$tileHeight = 340
$labelHeight = 48
$columns = 3
$perSheet = 8

for ($offset = 0; $offset -lt $files.Count; $offset += $perSheet) {
    $last = [Math]::Min($offset + $perSheet - 1, $files.Count - 1)
    $batch = @($files[$offset..$last])
    $rows = [Math]::Ceiling($batch.Count / $columns)
    $sheet = [System.Drawing.Bitmap]::new($tileWidth * $columns, $tileHeight * $rows)
    $graphics = [System.Drawing.Graphics]::FromImage($sheet)
    $font = [System.Drawing.Font]::new("Segoe UI", 9)

    try {
        $graphics.Clear([System.Drawing.Color]::White)
        $graphics.InterpolationMode = [System.Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
        for ($index = 0; $index -lt $batch.Count; $index++) {
            $file = $batch[$index]
            $x = ($index % $columns) * $tileWidth
            $y = [Math]::Floor($index / $columns) * $tileHeight
            $source = [System.Drawing.Image]::FromFile($file.FullName)
            try {
                $availableWidth = $tileWidth - 16
                $availableHeight = $tileHeight - $labelHeight - 12
                $scale = [Math]::Min($availableWidth / $source.Width, $availableHeight / $source.Height)
                $drawWidth = [Math]::Max(1, [int]($source.Width * $scale))
                $drawHeight = [Math]::Max(1, [int]($source.Height * $scale))
                $drawX = $x + [int](($tileWidth - $drawWidth) / 2)
                $drawY = $y + $labelHeight
                $graphics.DrawImage($source, $drawX, $drawY, $drawWidth, $drawHeight)
                $label = $file.Name -replace "-actual\.png$", ""
                $labelRectangle = [System.Drawing.RectangleF]::new($x + 6, $y + 5, $tileWidth - 12, $labelHeight - 6)
                $graphics.DrawString($label, $font, [System.Drawing.Brushes]::Black, $labelRectangle)
                $graphics.DrawRectangle([System.Drawing.Pens]::LightGray, $x, $y, $tileWidth - 1, $tileHeight - 1)
            }
            finally {
                $source.Dispose()
            }
        }

        $sheetNumber = [int]($offset / $perSheet) + 1
        $destination = Join-Path $outputPath "$Prefix-$sheetNumber.png"
        $sheet.Save($destination, [System.Drawing.Imaging.ImageFormat]::Png)
        Write-Output $destination
    }
    finally {
        $font.Dispose()
        $graphics.Dispose()
        $sheet.Dispose()
    }
}
