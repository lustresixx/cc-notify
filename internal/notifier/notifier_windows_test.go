//go:build windows

package notifier

import (
	"errors"
	"strings"
	"testing"
)

type captureRunner struct {
	name string
	args []string
	err  error
	errs []error
	call int
}

func (r *captureRunner) Run(name string, args ...string) error {
	r.name = name
	r.args = append([]string{}, args...)
	if len(r.errs) > 0 {
		var err error
		if r.call < len(r.errs) {
			err = r.errs[r.call]
		} else {
			err = r.errs[len(r.errs)-1]
		}
		r.call++
		return err
	}
	r.call++
	return r.err
}

func TestWindowsNotifierNotify_BuildsPowerShellCommand(t *testing.T) {
	runner := &captureRunner{}
	n := &windowsNotifier{
		shell:  "powershell.exe",
		runner: runner,
		mode:   modeAuto,
		appID:  "Windows PowerShell",
	}

	if err := n.Notify("title", "body"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if runner.name != "powershell.exe" {
		t.Fatalf("unexpected command: %q", runner.name)
	}
	joined := strings.Join(runner.args, " ")
	if !strings.Contains(joined, "-EncodedCommand") {
		t.Fatalf("expected -EncodedCommand arg, got %q", joined)
	}
}

func TestWindowsNotifierNotify_WrapsRunnerError(t *testing.T) {
	runner := &captureRunner{errs: []error{errors.New("toast boom"), errors.New("popup boom")}}
	n := &windowsNotifier{
		shell:  "powershell.exe",
		runner: runner,
		mode:   modeAuto,
		appID:  "Windows PowerShell",
	}

	err := n.Notify("title", "body")
	if err == nil {
		t.Fatalf("expected wrapped error")
	}
	if !strings.Contains(err.Error(), "popup fallback failed") {
		t.Fatalf("unexpected error message: %v", err)
	}
	if runner.call != 2 {
		t.Fatalf("expected both toast and popup attempts, got %d", runner.call)
	}
}

func TestWindowsNotifierNotify_FallbackToPopup(t *testing.T) {
	runner := &captureRunner{errs: []error{errors.New("toast denied"), nil}}
	n := &windowsNotifier{
		shell:  "powershell.exe",
		runner: runner,
		mode:   modeAuto,
		appID:  "Windows PowerShell",
	}

	if err := n.Notify("title", "body"); err != nil {
		t.Fatalf("expected popup fallback to succeed, got %v", err)
	}
	if runner.call != 2 {
		t.Fatalf("expected 2 calls (toast + popup), got %d", runner.call)
	}
}

func TestWindowsNotifierNotify_ToastModeNoFallback(t *testing.T) {
	runner := &captureRunner{err: errors.New("toast denied")}
	n := &windowsNotifier{
		shell:  "powershell.exe",
		runner: runner,
		mode:   modeToast,
		appID:  "Windows PowerShell",
	}

	err := n.Notify("title", "body")
	if err == nil {
		t.Fatalf("expected error in toast-only mode")
	}
	if runner.call != 1 {
		t.Fatalf("expected one call in toast-only mode, got %d", runner.call)
	}
}

func TestParseNotifyMode(t *testing.T) {
	if got := parseNotifyMode("toast"); got != modeToast {
		t.Fatalf("expected toast mode, got %v", got)
	}
	if got := parseNotifyMode("popup"); got != modePopup {
		t.Fatalf("expected popup mode, got %v", got)
	}
	if got := parseNotifyMode("invalid"); got != modeAuto {
		t.Fatalf("expected auto mode, got %v", got)
	}
}

func TestNewWithConfig_UsesCodexToastAppIDByDefault(t *testing.T) {
	n := NewWithConfig(Config{})
	wn, ok := n.(*windowsNotifier)
	if !ok {
		t.Fatalf("expected windowsNotifier type")
	}
	if wn.appID != "cc-notify.desktop" {
		t.Fatalf("expected default app id cc-notify.desktop, got %q", wn.appID)
	}
}

func TestNewWithConfig_MigratesLegacyToastAppID(t *testing.T) {
	n := NewWithConfig(Config{ToastAppID: "Windows PowerShell"})
	wn, ok := n.(*windowsNotifier)
	if !ok {
		t.Fatalf("expected windowsNotifier type")
	}
	if wn.appID != "cc-notify.desktop" {
		t.Fatalf("expected migrated app id cc-notify.desktop, got %q", wn.appID)
	}
}
