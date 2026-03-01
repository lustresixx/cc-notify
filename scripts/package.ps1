param(
  [string]$Version = "",
  [string]$InputDir = "dist",
  [string]$OutputDir = "release"
)

$ErrorActionPreference = "Stop"

$projectRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$fullInputDir = Join-Path $projectRoot $InputDir
$fullOutputDir = Join-Path $projectRoot $OutputDir

if ([string]::IsNullOrWhiteSpace($Version)) {
  throw "Version is required. Use semantic version format like -Version v0.4.1."
}
if ($Version -notmatch '^v\d+\.\d+\.\d+$') {
  throw "Invalid version format '$Version'. Expected: v<major>.<minor>.<patch> (example: v0.4.1)."
}

if (-not (Test-Path $fullInputDir)) {
  throw "Input directory not found: $fullInputDir. Run scripts/build.ps1 first."
}

$executables = Get-ChildItem -Path $fullInputDir -Filter "cc-notify-*.exe" -File
if (-not $executables) {
  throw "No executables found in $fullInputDir. Run scripts/build.ps1 first."
}

New-Item -ItemType Directory -Force $fullOutputDir | Out-Null

$packageRoot = Join-Path $fullOutputDir "cc-notify-$Version"
if (Test-Path $packageRoot) {
  Remove-Item -Recurse -Force $packageRoot
}
New-Item -ItemType Directory -Force $packageRoot | Out-Null

foreach ($exe in $executables) {
  Copy-Item -Path $exe.FullName -Destination (Join-Path $packageRoot $exe.Name)
}
Copy-Item -Path (Join-Path $projectRoot "README.md") -Destination (Join-Path $packageRoot "README.md")
Copy-Item -Path (Join-Path $projectRoot "README_zh.md") -Destination (Join-Path $packageRoot "README_zh.md")
Copy-Item -Path (Join-Path $projectRoot "LICENSE") -Destination (Join-Path $packageRoot "LICENSE")
Copy-Item -Path (Join-Path $projectRoot "install.cmd") -Destination (Join-Path $packageRoot "install.cmd")
Copy-Item -Path (Join-Path $projectRoot "uninstall.cmd") -Destination (Join-Path $packageRoot "uninstall.cmd")

$scriptDir = Join-Path $packageRoot "scripts"
New-Item -ItemType Directory -Force $scriptDir | Out-Null
Copy-Item -Path (Join-Path $projectRoot "scripts/install.ps1") -Destination (Join-Path $scriptDir "install.ps1")
Copy-Item -Path (Join-Path $projectRoot "scripts/install.cmd") -Destination (Join-Path $scriptDir "install.cmd")
Copy-Item -Path (Join-Path $projectRoot "scripts/uninstall.ps1") -Destination (Join-Path $scriptDir "uninstall.ps1")
Copy-Item -Path (Join-Path $projectRoot "scripts/uninstall.cmd") -Destination (Join-Path $scriptDir "uninstall.cmd")

$zipPath = Join-Path $fullOutputDir "cc-notify-$Version.zip"
if (Test-Path $zipPath) {
  Remove-Item -Force $zipPath
}
Compress-Archive -Path (Join-Path $packageRoot "*") -DestinationPath $zipPath

Write-Host "Package created: $zipPath"
