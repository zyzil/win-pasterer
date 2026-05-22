param(
    [switch]$IncludeIntegration
)

$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
Set-Location $repoRoot

Write-Host "Running gofmt check..."
$gofmtOutput = gofmt -l .
if ($gofmtOutput) {
    Write-Error "gofmt required for:`n$gofmtOutput"
}

Write-Host "Running unit tests..."
go test ./...

Write-Host "Running short tests..."
go test -short ./...

if ($IncludeIntegration) {
    Write-Host "Running integration tests..."
    go test -tags integration ./...
}

Write-Host "Building app..."
go build ./cmd/win-pasterer

Write-Host "Verification complete."
