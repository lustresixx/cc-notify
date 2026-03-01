param(
  [string]$KeepSettings = "0"
)

$ErrorActionPreference = "Stop"

$targetDir = Join-Path $env:LOCALAPPDATA "cc-notify"
$targetExe = Join-Path $targetDir "cc-notify.exe"
$configPath = Join-Path $env:USERPROFILE ".codex\config.toml"
$shortcutPath = Join-Path $env:APPDATA "Microsoft\Windows\Start Menu\Programs\cc-notify.lnk"
$protocolKey = "HKCU:\Software\Classes\cc-notify"

function To-Bool([string]$value, [bool]$defaultValue) {
  switch ($value.Trim().ToLowerInvariant()) {
    "1" { return $true }
    "true" { return $true }
    "yes" { return $true }
    "on" { return $true }
    "0" { return $false }
    "false" { return $false }
    "no" { return $false }
    "off" { return $false }
    "" { return $defaultValue }
    default { return $defaultValue }
  }
}

function Remove-NotifyFallback([string]$path) {
  if (-not (Test-Path $path)) {
    return
  }
  $raw = Get-Content -Raw $path
  if (-not $raw) {
    return
  }

  $pattern = "(?m)^\s*notify\s*=\s*\[[^\r\n]*cc-notify[^\r\n]*\]\s*\r?\n?"
  $updated = [regex]::Replace($raw, $pattern, "")
  if ($updated -ne $raw) {
    Set-Content -Encoding UTF8 $path $updated
    Write-Host "Removed notify hook from $path (fallback mode)."
  } else {
    Write-Host "No cc-notify hook found in $path."
  }
}

$keepSettingsBool = To-Bool $KeepSettings $false

if (Test-Path $targetExe) {
  Write-Host "Removing Codex hook..."
  & $targetExe uninstall
} else {
  Write-Host "Installed executable not found, using config fallback cleanup..."
  Remove-NotifyFallback $configPath
}

if (Test-Path $shortcutPath) {
  Remove-Item -Force $shortcutPath
  Write-Host "Removed Start Menu shortcut."
}

if (Test-Path $protocolKey) {
  Remove-Item -Recurse -Force $protocolKey
  Write-Host "Removed cc-notify:// protocol handler."
}

if (Test-Path $targetDir) {
  if ($keepSettingsBool) {
    $exePath = Join-Path $targetDir "cc-notify.exe"
    if (Test-Path $exePath) {
      try {
        Remove-Item -Force $exePath -ErrorAction Stop
        Write-Host "Removed executable."
      } catch {
        Write-Host "Executable is locked, scheduling deferred cleanup..."
        Start-Process cmd.exe -ArgumentList "/c", "ping 127.0.0.1 -n 3 >nul & del /f /q `"$exePath`"" -WindowStyle Hidden
        Write-Host "A background process will remove the executable in a few seconds."
      }
    }
  } else {
    try {
      Remove-Item -Recurse -Force $targetDir -ErrorAction Stop
      Write-Host "Removed data directory: $targetDir"
    } catch {
      Write-Host "Directory is locked, scheduling deferred cleanup..."
      Start-Process cmd.exe -ArgumentList "/c", "ping 127.0.0.1 -n 3 >nul & rmdir /s /q `"$targetDir`"" -WindowStyle Hidden
      Write-Host "A background process will remove $targetDir in a few seconds."
    }
  }
}

Write-Host "Uninstall complete."
