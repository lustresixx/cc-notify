param(
  [string]$OutputDir = "dist"
)

$ErrorActionPreference = "Stop"

$projectRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$fullOutputDir = Join-Path $projectRoot $OutputDir
New-Item -ItemType Directory -Force $fullOutputDir | Out-Null

$localGoCache = Join-Path $projectRoot ".gocache"
$localGoTmp = Join-Path $projectRoot ".gotmp"
$localGoTelemetry = Join-Path $projectRoot ".gotelemetry"
New-Item -ItemType Directory -Force $localGoCache | Out-Null
New-Item -ItemType Directory -Force $localGoTmp | Out-Null
New-Item -ItemType Directory -Force $localGoTelemetry | Out-Null

$useLocalGoCache = [string]::IsNullOrWhiteSpace($env:GOCACHE)
$useLocalGoTmp = [string]::IsNullOrWhiteSpace($env:GOTMPDIR)
$hadGoTelemetry = -not [string]::IsNullOrWhiteSpace($env:GOTELEMETRY)
$hadGoTelemetryDir = -not [string]::IsNullOrWhiteSpace($env:GOTELEMETRYDIR)
$oldGoTelemetry = $env:GOTELEMETRY
$oldGoTelemetryDir = $env:GOTELEMETRYDIR
if ($useLocalGoCache) {
  $env:GOCACHE = $localGoCache
}
if ($useLocalGoTmp) {
  $env:GOTMPDIR = $localGoTmp
}

# Keep builds quiet and deterministic in restricted environments.
$env:GOTELEMETRY = "off"
$env:GOTELEMETRYDIR = $localGoTelemetry

function Invoke-GoCommand([string[]]$GoArgs) {
  $output = & go @GoArgs 2>&1
  $exitCode = $LASTEXITCODE

  foreach ($line in @($output)) {
    $text = [string]$line
    if ($text -match '^error acquiring upload token:') { continue }
    Write-Host $text
  }

  if ($exitCode -ne 0) {
    throw "go $($GoArgs -join ' ') failed with exit code $exitCode"
  }
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
    Invoke-GoCommand @("build", "-trimpath", "-ldflags", "-s -w", "-o", $outputPath, "./cmd/cc-notify")
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
  if ($hadGoTelemetry) {
    $env:GOTELEMETRY = $oldGoTelemetry
  } else {
    Remove-Item Env:GOTELEMETRY -ErrorAction SilentlyContinue
  }
  if ($hadGoTelemetryDir) {
    $env:GOTELEMETRYDIR = $oldGoTelemetryDir
  } else {
    Remove-Item Env:GOTELEMETRYDIR -ErrorAction SilentlyContinue
  }
}

Write-Host "Build complete."
