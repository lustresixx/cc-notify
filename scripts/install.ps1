param(
  [string]$ExePath = "",
  [string]$RunTest = "1",
  [string]$LaunchInteractive = "1",
  [string]$ForceReinstall = "0"
)

$ErrorActionPreference = "Stop"

$projectRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$targetDir = Join-Path $env:LOCALAPPDATA "cc-notify"
$targetExe = Join-Path $targetDir "cc-notify.exe"
$configPath = Join-Path $env:USERPROFILE ".codex\config.toml"
$settingsPath = Join-Path $targetDir "settings.json"
$toastAppID = "cc-notify.desktop"
$shortcutPath = Join-Path $env:APPDATA "Microsoft\Windows\Start Menu\Programs\cc-notify.lnk"
$protocolKey = "HKCU:\Software\Classes\cc-notify"

function Get-ExpectedNotifyLine([string]$path) {
  $escaped = $path.Replace('\', '\\')
  return "notify = [""$escaped"", ""notify""]"
}

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

$runTestBool = To-Bool $RunTest $true
$launchInteractiveBool = To-Bool $LaunchInteractive $true
$forceReinstallBool = To-Bool $ForceReinstall $false

function Test-IsInstalled([string]$targetPath, [string]$cfgPath) {
  if (-not (Test-Path $targetPath)) {
    return $false
  }
  if (-not (Test-Path $cfgPath)) {
    return $false
  }

  $expected = Get-ExpectedNotifyLine $targetPath
  $content = Get-Content -Raw $cfgPath
  return $content.Contains($expected)
}

function Resolve-SourceExe([string]$explicitPath) {
  if ($explicitPath) {
    if (-not (Test-Path $explicitPath)) {
      throw "Specified -ExePath does not exist: $explicitPath"
    }
    return $explicitPath
  }

  $isArm64 = $env:PROCESSOR_ARCHITECTURE -eq "ARM64" -or $env:PROCESSOR_ARCHITEW6432 -eq "ARM64"
  $archFirst = if ($isArm64) { "arm64" } else { "amd64" }
  $archSecond = if ($isArm64) { "amd64" } else { "arm64" }

  $candidatePaths = @(
    (Join-Path $projectRoot "dist/cc-notify-windows-$archFirst.exe"),
    (Join-Path $projectRoot "dist/cc-notify-windows-$archSecond.exe"),
    (Join-Path $projectRoot "dist/cc-notify-windows-amd64.exe"),
    (Join-Path $projectRoot "dist/cc-notify-windows-arm64.exe"),
    (Join-Path $projectRoot "dist/cc-notify.exe"),
    (Join-Path $projectRoot "cc-notify-windows-$archFirst.exe"),
    (Join-Path $projectRoot "cc-notify-windows-$archSecond.exe"),
    (Join-Path $projectRoot "cc-notify.exe"),
    $targetExe
  )

  $resolved = $candidatePaths | Where-Object { Test-Path $_ } | Select-Object -First 1
  if (-not $resolved) {
    throw "Cannot find cc-notify executable. Build first or pass -ExePath explicitly."
  }
  return $resolved
}

function Resolve-PathSafe([string]$path) {
  if (-not (Test-Path $path)) {
    return ""
  }
  return (Resolve-Path $path).Path
}

function Should-CopyExecutable([string]$sourcePath, [string]$destPath) {
  if (-not (Test-Path $sourcePath)) {
    return $false
  }
  if (-not (Test-Path $destPath)) {
    return $true
  }
  $srcResolved = Resolve-PathSafe $sourcePath
  $dstResolved = Resolve-PathSafe $destPath
  if ($srcResolved -eq $dstResolved) {
    return $false
  }
  $srcHash = (Get-FileHash -Algorithm SHA256 $sourcePath).Hash
  $dstHash = (Get-FileHash -Algorithm SHA256 $destPath).Hash
  return $srcHash -ne $dstHash
}

function Ensure-ToastShortcut([string]$exePath, [string]$linkPath, [string]$appID) {
  $linkDir = Split-Path -Parent $linkPath
  New-Item -ItemType Directory -Force $linkDir | Out-Null

  $shell = New-Object -ComObject WScript.Shell
  $shortcut = $shell.CreateShortcut($linkPath)
  $shortcut.TargetPath = $exePath
  $shortcut.Arguments = ""
  $shortcut.WorkingDirectory = Split-Path -Parent $exePath
  $shortcut.IconLocation = "$exePath,0"
  $shortcut.Description = "cc-notify"
  $shortcut.Save()

  if (-not ("ShortcutAppId" -as [type])) {
    Add-Type -TypeDefinition @"
using System;
using System.Runtime.InteropServices;

public static class ShortcutAppId {
  [StructLayout(LayoutKind.Sequential, Pack = 4)]
  public struct PROPERTYKEY {
    public Guid fmtid;
    public uint pid;
  }

  [StructLayout(LayoutKind.Explicit)]
  public struct PROPVARIANT {
    [FieldOffset(0)] public ushort vt;
    [FieldOffset(8)] public IntPtr pointerValue;
  }

  [ComImport, Guid("886D8EEB-8CF2-4446-8D02-CDBA1DBDCF99"), InterfaceType(ComInterfaceType.InterfaceIsIUnknown)]
  interface IPropertyStore {
    uint GetCount(out uint cProps);
    uint GetAt(uint iProp, out PROPERTYKEY pkey);
    uint GetValue(ref PROPERTYKEY key, out PROPVARIANT pv);
    uint SetValue(ref PROPERTYKEY key, ref PROPVARIANT pv);
    uint Commit();
  }

  [DllImport("shell32.dll", CharSet = CharSet.Unicode, PreserveSig = false)]
  static extern void SHGetPropertyStoreFromParsingName(
    string pszPath,
    IntPtr zero,
    uint flags,
    ref Guid iid,
    out IPropertyStore propertyStore
  );

  [DllImport("ole32.dll", PreserveSig = false)]
  static extern void PropVariantClear(ref PROPVARIANT pvar);

  public static void SetShortcutAppId(string shortcutPath, string appId) {
    Guid iid = new Guid("886D8EEB-8CF2-4446-8D02-CDBA1DBDCF99");
    IPropertyStore store;
    SHGetPropertyStoreFromParsingName(shortcutPath, IntPtr.Zero, 0x2, ref iid, out store);
    PROPERTYKEY key = new PROPERTYKEY { fmtid = new Guid("9F4C2855-9F79-4B39-A8D0-E1D42DE1D5F3"), pid = 5 };
    PROPVARIANT pv = new PROPVARIANT();
    pv.vt = 31;
    pv.pointerValue = Marshal.StringToCoTaskMemUni(appId);
    try {
      store.SetValue(ref key, ref pv);
      store.Commit();
    } finally {
      PropVariantClear(ref pv);
      Marshal.ReleaseComObject(store);
    }
  }
}
"@
  }

  [ShortcutAppId]::SetShortcutAppId($linkPath, $appID)
}

function Update-SettingsToastAppId([string]$path, [string]$appID) {
  if (-not (Test-Path $path)) {
    return
  }
  $raw = Get-Content -Raw $path
  if (-not $raw) {
    return
  }
  $obj = $raw | ConvertFrom-Json
  if (-not $obj) {
    return
  }
  $current = ""
  if ($obj.PSObject.Properties.Name -contains "toast_app_id") {
    $current = [string]$obj.toast_app_id
  }
  if ([string]::IsNullOrWhiteSpace($current) -or $current -eq "Windows PowerShell") {
    $obj.toast_app_id = $appID
    $obj | ConvertTo-Json -Depth 8 | Set-Content -Encoding UTF8 $path
    Write-Host "Updated settings toast_app_id to $appID"
  }
}

function Ensure-UriProtocol([string]$exePath) {
  New-Item -Path $protocolKey -Force | Out-Null
  Set-Item -Path $protocolKey -Value "URL:cc-notify Protocol"
  New-ItemProperty -Path $protocolKey -Name "URL Protocol" -Value "" -PropertyType String -Force | Out-Null

  $iconKey = Join-Path $protocolKey "DefaultIcon"
  New-Item -Path $iconKey -Force | Out-Null
  Set-Item -Path $iconKey -Value "`"$exePath`",0"

  $commandKey = Join-Path $protocolKey "shell\open\command"
  New-Item -Path $commandKey -Force | Out-Null
  Set-Item -Path $commandKey -Value "`"$exePath`" `"%1`""
}

$installed = $false
if (-not $forceReinstallBool) {
  $installed = Test-IsInstalled $targetExe $configPath
}

$sourceExe = Resolve-SourceExe $ExePath

if ($installed) {
  if (Should-CopyExecutable $sourceExe $targetExe) {
    New-Item -ItemType Directory -Force $targetDir | Out-Null
    Copy-Item -Force $sourceExe $targetExe
    Write-Host "Updated installed executable to latest version."
    & $targetExe install
  } else {
    Write-Host "cc-notify is already installed and up to date."
  }
} else {
  New-Item -ItemType Directory -Force $targetDir | Out-Null

  if (Should-CopyExecutable $sourceExe $targetExe) {
    Copy-Item -Force $sourceExe $targetExe
  }

  Write-Host "Installed executable to: $targetExe"
  & $targetExe install

  if ($runTestBool) {
    Write-Host "Running notification self-test..."
    & $targetExe test-notify
  }
}

Ensure-ToastShortcut $targetExe $shortcutPath $toastAppID
Update-SettingsToastAppId $settingsPath $toastAppID
Ensure-UriProtocol $targetExe

if ($launchInteractiveBool) {
  Write-Host "Launching interactive control center..."
  & $targetExe
}

Write-Host "Done."
