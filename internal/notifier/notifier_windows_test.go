//go:build windows

package notifier

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"unicode/utf16"
)

type captureRunner struct {
	name string
	args []string
	all  [][]string
	err  error
	errs []error
	call int
}

func (r *captureRunner) Run(name string, args ...string) error {
	r.name = name
	r.args = append([]string{}, args...)
	r.all = append(r.all, append([]string{}, args...))
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

func decodeEncodedCommand(args []string) string {
	for i := 0; i < len(args)-1; i++ {
		if args[i] != "-EncodedCommand" {
			continue
		}
		raw := args[i+1]
		decoded, err := base64.StdEncoding.DecodeString(raw)
		if err != nil {
			return ""
		}
		if len(decoded)%2 != 0 {
			return ""
		}
		u16 := make([]uint16, 0, len(decoded)/2)
		for j := 0; j < len(decoded); j += 2 {
			u16 = append(u16, uint16(decoded[j])|uint16(decoded[j+1])<<8)
		}
		return string(utf16.Decode(u16))
	}
	return ""
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
	runner := &captureRunner{errs: []error{errors.New("toast boom"), errors.New("legacy toast boom"), errors.New("popup boom")}}
	n := &windowsNotifier{
		shell:  "powershell.exe",
		runner: runner,
		mode:   modeAuto,
		appID:  "cc-notify.desktop",
	}

	err := n.Notify("title", "body")
	if err == nil {
		t.Fatalf("expected wrapped error")
	}
	if !strings.Contains(err.Error(), "popup fallback failed") {
		t.Fatalf("unexpected error message: %v", err)
	}
	if runner.call != 3 {
		t.Fatalf("expected toast + legacy toast + popup attempts, got %d", runner.call)
	}
}

func TestWindowsNotifierNotify_FallbackToLegacyToast(t *testing.T) {
	runner := &captureRunner{errs: []error{errors.New("toast denied"), nil}}
	n := &windowsNotifier{
		shell:  "powershell.exe",
		runner: runner,
		mode:   modeAuto,
		appID:  "cc-notify.desktop",
	}

	if err := n.Notify("title", "body"); err != nil {
		t.Fatalf("expected legacy toast fallback to succeed, got %v", err)
	}
	if runner.call != 2 {
		t.Fatalf("expected 2 calls (toast + legacy toast), got %d", runner.call)
	}
	script := decodeEncodedCommand(runner.all[1])
	if !strings.Contains(script, "V2luZG93cyBQb3dlclNoZWxs") {
		t.Fatalf("expected second toast attempt to use legacy app id, got script=%q", script)
	}
}

func TestWindowsNotifierNotify_ToastModeNoFallback(t *testing.T) {
	runner := &captureRunner{errs: []error{errors.New("toast denied"), nil}}
	n := &windowsNotifier{
		shell:  "powershell.exe",
		runner: runner,
		mode:   modeToast,
		appID:  "cc-notify.desktop",
	}

	if err := n.Notify("title", "body"); err != nil {
		t.Fatalf("expected legacy fallback success in toast-only mode, got %v", err)
	}
	if runner.call != 2 {
		t.Fatalf("expected two toast attempts in toast-only mode, got %d", runner.call)
	}
}

func TestWindowsNotifierNotify_ToastModeAccessDeniedFallsBackToPopup(t *testing.T) {
	denied := errors.New("Exception from HRESULT: 0x80070005 (E_ACCESSDENIED)")
	runner := &captureRunner{errs: []error{denied, denied, nil}}
	n := &windowsNotifier{
		shell:  "powershell.exe",
		runner: runner,
		mode:   modeToast,
		appID:  "cc-notify.desktop",
	}

	if err := n.Notify("title", "body"); err != nil {
		t.Fatalf("expected popup fallback after toast access denied, got %v", err)
	}
	if runner.call != 3 {
		t.Fatalf("expected toast + legacy toast + popup attempts, got %d", runner.call)
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
