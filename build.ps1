param(
    [string]$Output = "win-pasterer.exe",
    [switch]$Clean
)

$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $repoRoot

if ($Clean) {
    Remove-Item -Path "cmd/win-pasterer/rsrc_windows_*.syso" -Force -ErrorAction SilentlyContinue
}

$goBin = Join-Path (go env GOPATH) "bin"
$goWinres = Join-Path $goBin "go-winres.exe"
if (-not (Test-Path $goWinres)) {
    Write-Host "Installing go-winres..."
    go install github.com/tc-hib/go-winres@latest
}

if (-not (Test-Path "winres/icon_16.png")) {
    throw "Missing placeholder icons. Run scripts/generate_icons.ps1 or add your own files in winres/."
}

Write-Host "Generating Windows resources..."
& $goWinres make --in winres/winres.json --out cmd/win-pasterer/rsrc

Write-Host "Building $Output ..."
go build -trimpath -ldflags "-s -w" -o $Output ./cmd/win-pasterer

Write-Host "Build complete: $Output"
