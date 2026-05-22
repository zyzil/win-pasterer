$ErrorActionPreference = "Stop"

Add-Type -AssemblyName System.Drawing

$repoRoot = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
$winresDir = Join-Path $repoRoot "winres"
$sourceIcon = Join-Path $repoRoot "icon.ico"
New-Item -ItemType Directory -Path $winresDir -Force | Out-Null

if (-not (Test-Path $sourceIcon)) {
    throw "Missing source icon: $sourceIcon"
}

function Export-IconPng {
    param(
        [int]$Size,
        [string]$Path
    )

    $icon = New-Object System.Drawing.Icon($sourceIcon, $Size, $Size)
    $src = $icon.ToBitmap()
    $bmp = New-Object System.Drawing.Bitmap $Size, $Size, ([System.Drawing.Imaging.PixelFormat]::Format32bppArgb)
    $g = [System.Drawing.Graphics]::FromImage($bmp)
    $g.Clear([System.Drawing.Color]::Transparent)
    $g.InterpolationMode = [System.Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
    $g.SmoothingMode = [System.Drawing.Drawing2D.SmoothingMode]::HighQuality
    $g.PixelOffsetMode = [System.Drawing.Drawing2D.PixelOffsetMode]::HighQuality
    $g.DrawImage($src, 0, 0, $Size, $Size)
    $bmp.Save($Path, [System.Drawing.Imaging.ImageFormat]::Png)

    $g.Dispose()
    $bmp.Dispose()
    $src.Dispose()
    $icon.Dispose()
}

$targets = @(16, 32, 48, 64, 128, 256)
foreach ($size in $targets) {
    $path = Join-Path $winresDir ("icon_{0}.png" -f $size)
    Export-IconPng -Size $size -Path $path
    Write-Host "Generated $path"
}
