//go:build windows

package app

import (
	"encoding/base64"
	"fmt"
	"os/exec"
	"strings"
	"unicode/utf16"
)

type windowsApprovalExecutor struct{}

func newDefaultApprovalExecutor() ApprovalExecutor {
	return windowsApprovalExecutor{}
}

func (windowsApprovalExecutor) Deliver(parentPID int, decision approvalDecision) error {
	if parentPID <= 0 {
		return fmt.Errorf("cannot deliver approval: invalid parent process id")
	}

	keys := "n{ENTER}"
	if decision == approvalApprove {
		keys = "y{ENTER}"
	}

	script := fmt.Sprintf(
		`$ErrorActionPreference = 'Stop'
$seed = %d
$keys = '%s'
$wshell = New-Object -ComObject WScript.Shell

function Get-ParentPid([int]$pid) {
  try {
    $proc = Get-CimInstance Win32_Process -Filter ("ProcessId = " + $pid)
    if ($null -eq $proc) { return 0 }
    return [int]$proc.ParentProcessId
  } catch {
    return 0
  }
}

$pids = @()
$current = [int]$seed
for ($i = 0; $i -lt 8 -and $current -gt 0; $i++) {
  $pids += $current
  $current = Get-ParentPid $current
}

foreach ($p in $pids) {
  if ($wshell.AppActivate([int]$p)) {
    Start-Sleep -Milliseconds 120
    $wshell.SendKeys($keys)
    exit 0
  }
}

throw 'no interactive terminal session found for pending approval'
`,
		parentPID,
		keys,
	)

	cmd := exec.Command("powershell.exe",
		"-NoProfile",
		"-NonInteractive",
		"-ExecutionPolicy", "Bypass",
		"-EncodedCommand", encodePowerShellCommand(script),
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(output))
		if msg == "" {
			return fmt.Errorf("deliver approval decision: %w", err)
		}
		return fmt.Errorf("deliver approval decision: %w: %s", err, msg)
	}
	return nil
}

func encodePowerShellCommand(command string) string {
	utf16Text := utf16.Encode([]rune(command))
	utf16LEBytes := make([]byte, len(utf16Text)*2)
	for i, code := range utf16Text {
		utf16LEBytes[i*2] = byte(code)
		utf16LEBytes[i*2+1] = byte(code >> 8)
	}
	return base64.StdEncoding.EncodeToString(utf16LEBytes)
}
