//go:build windows

package notifier

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type commandRunner interface {
	Run(name string, args ...string) error
}

type execRunner struct{}

func (execRunner) Run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
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

type windowsNotifier struct {
	shell  string
	runner commandRunner
	mode   notifyMode
	appID  string
}

type notifyMode int

const (
	modeAuto notifyMode = iota
	modeToast
	modePopup
)

const (
	defaultToastAppID = "cc-notify.desktop"
	legacyToastAppID  = "Windows PowerShell"
)

// New creates a Windows notifier backed by PowerShell.
func New() Service {
	return NewWithConfig(Config{
		Mode:       os.Getenv("CC_NOTIFY_MODE"),
		ToastAppID: os.Getenv("CC_NOTIFY_TOAST_APP_ID"),
	})
}

// NewWithConfig creates a Windows notifier with explicit config values.
func NewWithConfig(cfg Config) Service {
	appID := strings.TrimSpace(cfg.ToastAppID)
	if appID == "" || appID == legacyToastAppID || appID == "codex-notified.desktop" {
		appID = defaultToastAppID
	}
	return &windowsNotifier{
		shell:  "powershell.exe",
		runner: execRunner{},
		mode:   parseNotifyMode(cfg.Mode),
		appID:  appID,
	}
}

func (n *windowsNotifier) Notify(title, body string) error {
	switch n.mode {
	case modeToast:
		if err := n.runPowerShell(buildToastScript(title, body, n.appID)); err != nil {
			return fmt.Errorf("send windows notification (toast): %w", err)
		}
		return nil
	case modePopup:
		if err := n.runPowerShell(buildPopupScript(title, body)); err != nil {
			return fmt.Errorf("send windows notification (popup): %w", err)
		}
		return nil
	default:
		if err := n.runPowerShell(buildToastScript(title, body, n.appID)); err != nil {
			fallbackErr := n.runPowerShell(buildPopupScript(title, body))
			if fallbackErr != nil {
				return fmt.Errorf("send windows notification: toast failed: %v; popup fallback failed: %w", err, fallbackErr)
			}
		}
		return nil
	}
}

func parseNotifyMode(raw string) notifyMode {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "toast":
		return modeToast
	case "popup":
		return modePopup
	default:
		return modeAuto
	}
}

func (n *windowsNotifier) runPowerShell(script string) error {
	encoded := encodePowerShellCommand(script)
	args := []string{
		"-NoProfile",
		"-NonInteractive",
		"-ExecutionPolicy", "Bypass",
		"-EncodedCommand", encoded,
	}
	return n.runner.Run(n.shell, args...)
}
