param(
  [string]$OutputDir = "dist"
)

$ErrorActionPreference = "Stop"

$projectRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$fullOutputDir = Join-Path $projectRoot $OutputDir
New-Item -ItemType Directory -Force $fullOutputDir | Out-Null

$localGoCache = Join-Path $projectRoot ".gocache"
$localGoTmp = Join-Path $projectRoot ".gotmp"
New-Item -ItemType Directory -Force $localGoCache | Out-Null
New-Item -ItemType Directory -Force $localGoTmp | Out-Null

$useLocalGoCache = [string]::IsNullOrWhiteSpace($env:GOCACHE)
$useLocalGoTmp = [string]::IsNullOrWhiteSpace($env:GOTMPDIR)
if ($useLocalGoCache) {
  $env:GOCACHE = $localGoCache
}
if ($useLocalGoTmp) {
  $env:GOTMPDIR = $localGoTmp
}

$targets = @(
  @{ GOOS = "windows"; GOARCH = "amd64"; Name = "cc-notify-windows-amd64.exe" },
  @{ GOOS = "windows"; GOARCH = "arm64"; Name = "cc-notify-windows-arm64.exe" }
)

try {
  foreach ($target in $targets) {
    $outputPath = Join-Path $fullOutputDir $target.Name
    Write-Host "Building $($target.GOOS)/$($target.GOARCH) -> $outputPath"

    $env:GOOS = $target.GOOS
    $env:GOARCH = $target.GOARCH
    go build -trimpath -ldflags "-s -w" -o $outputPath ./cmd/cc-notify
  }
}
finally {
  Remove-Item Env:GOOS -ErrorAction SilentlyContinue
  Remove-Item Env:GOARCH -ErrorAction SilentlyContinue
  if ($useLocalGoCache) {
    Remove-Item Env:GOCACHE -ErrorAction SilentlyContinue
  }
  if ($useLocalGoTmp) {
    Remove-Item Env:GOTMPDIR -ErrorAction SilentlyContinue
  }
}

Write-Host "Build complete."
