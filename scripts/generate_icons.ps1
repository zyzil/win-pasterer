$ErrorActionPreference = "Stop"

Add-Type -AssemblyName System.Drawing

$repoRoot = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
$winresDir = Join-Path $repoRoot "winres"
New-Item -ItemType Directory -Path $winresDir -Force | Out-Null

function New-PlaceholderIcon {
    param(
        [int]$Size,
        [string]$Path
    )

    $bmp = New-Object System.Drawing.Bitmap $Size, $Size
    $g = [System.Drawing.Graphics]::FromImage($bmp)
    $g.SmoothingMode = [System.Drawing.Drawing2D.SmoothingMode]::AntiAlias
    $g.Clear([System.Drawing.Color]::FromArgb(28, 46, 74))

    $brush = New-Object System.Drawing.SolidBrush([System.Drawing.Color]::FromArgb(91, 192, 222))
    $margin = [Math]::Max([int]($Size * 0.16), 2)
    $diameter = $Size - ($margin * 2)
    $g.FillEllipse($brush, $margin, $margin, $diameter, $diameter)

    $fontSize = [Math]::Max([single]($Size * 0.36), 6)
    $font = New-Object System.Drawing.Font("Segoe UI", $fontSize, [System.Drawing.FontStyle]::Bold, [System.Drawing.GraphicsUnit]::Pixel)
    $textBrush = New-Object System.Drawing.SolidBrush([System.Drawing.Color]::White)
    $format = New-Object System.Drawing.StringFormat
    $format.Alignment = [System.Drawing.StringAlignment]::Center
    $format.LineAlignment = [System.Drawing.StringAlignment]::Center

    $rect = New-Object System.Drawing.RectangleF(0, 0, $Size, $Size)
    $g.DrawString("P", $font, $textBrush, $rect, $format)

    $bmp.Save($Path, [System.Drawing.Imaging.ImageFormat]::Png)

    $format.Dispose()
    $textBrush.Dispose()
    $font.Dispose()
    $brush.Dispose()
    $g.Dispose()
    $bmp.Dispose()
}

$targets = @(16, 32, 48, 64)
foreach ($size in $targets) {
    $path = Join-Path $winresDir ("icon_{0}.png" -f $size)
    New-PlaceholderIcon -Size $size -Path $path
    Write-Host "Generated $path"
}
