param(
  [string]$OutputDir = "dist"
)

$ErrorActionPreference = "Stop"

$projectRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$fullOutputDir = Join-Path $projectRoot $OutputDir
New-Item -ItemType Directory -Force $fullOutputDir | Out-Null

$targets = @(
  @{ GOOS = "windows"; GOARCH = "amd64"; Name = "cc-notify-windows-amd64.exe" },
  @{ GOOS = "windows"; GOARCH = "arm64"; Name = "cc-notify-windows-arm64.exe" }
)

foreach ($target in $targets) {
  $outputPath = Join-Path $fullOutputDir $target.Name
  Write-Host "Building $($target.GOOS)/$($target.GOARCH) -> $outputPath"

  $env:GOOS = $target.GOOS
  $env:GOARCH = $target.GOARCH
  go build -trimpath -ldflags "-s -w" -o $outputPath ./cmd/cc-notify
}

Remove-Item Env:GOOS -ErrorAction SilentlyContinue
Remove-Item Env:GOARCH -ErrorAction SilentlyContinue

Write-Host "Build complete."
