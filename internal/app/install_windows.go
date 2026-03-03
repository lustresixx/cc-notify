//go:build windows

package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ensureToastShortcut creates a Start Menu shortcut with the correct
// AppUserModelID so that Windows toast notifications work. This replicates
// the Ensure-ToastShortcut function from scripts/install.ps1.
func ensureToastShortcut(exePath, appID string) error {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return fmt.Errorf("APPDATA environment variable is not set")
	}
	shortcutPath := filepath.Join(appData, "Microsoft", "Windows", "Start Menu", "Programs", "cc-notify.lnk")

	script := fmt.Sprintf(
		`$ErrorActionPreference = 'Stop'
$exePath = '%s'
$linkPath = '%s'
$appID = '%s'
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
`,
		escapePSString(exePath),
		escapePSString(shortcutPath),
		escapePSString(appID),
	)

	return runPowerShellScript(script)
}

// ensureURIProtocol registers the cc-notify:// protocol handler in the
// current user's registry so that toast notification action buttons can
// call back into cc-notify. Replicates Ensure-UriProtocol from install.ps1.
func ensureURIProtocol(exePath string) error {
	script := fmt.Sprintf(
		`$ErrorActionPreference = 'Stop'
$exePath = '%s'
$protocolKey = 'HKCU:\Software\Classes\cc-notify'

New-Item -Path $protocolKey -Force | Out-Null
Set-Item -Path $protocolKey -Value 'URL:cc-notify Protocol'
New-ItemProperty -Path $protocolKey -Name 'URL Protocol' -Value '' -PropertyType String -Force | Out-Null

$iconKey = Join-Path $protocolKey 'DefaultIcon'
New-Item -Path $iconKey -Force | Out-Null
Set-Item -Path $iconKey -Value ('"{0}",0' -f $exePath)

$commandKey = Join-Path $protocolKey 'shell\open\command'
New-Item -Path $commandKey -Force | Out-Null
Set-Item -Path $commandKey -Value ('"{0}" "%%1"' -f $exePath)
`,
		escapePSString(exePath),
	)

	return runPowerShellScript(script)
}

func runPowerShellScript(script string) error {
	cmd := exec.Command("powershell.exe",
		"-NoProfile",
		"-NonInteractive",
		"-ExecutionPolicy", "Bypass",
		"-Command", script,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		text := strings.TrimSpace(string(output))
		if text == "" {
			return err
		}
		return fmt.Errorf("%w: %s", err, text)
	}
	return nil
}

func escapePSString(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}
