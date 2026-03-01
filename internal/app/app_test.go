package app

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cc-notify/internal/notifier"
)

type fakeNotifier struct {
	count int
	title string
	body  string
}

func (f *fakeNotifier) Notify(title, body string) error {
	f.count++
	f.title = title
	f.body = body
	return nil
}

type fakeActionNotifier struct {
	fakeNotifier
	actionCount int
	actions     []notifier.Action
}

func (f *fakeActionNotifier) NotifyWithActions(title, body string, actions []notifier.Action) error {
	f.actionCount++
	f.title = title
	f.body = body
	f.actions = append([]notifier.Action{}, actions...)
	return nil
}

type fakeApprovalExecutor struct {
	calls []approvalInput
	err   error
}

type approvalInput struct {
	parentPID int
	decision  approvalDecision
}

func (f *fakeApprovalExecutor) Deliver(parentPID int, decision approvalDecision) error {
	if f.err != nil {
		return f.err
	}
	f.calls = append(f.calls, approvalInput{parentPID: parentPID, decision: decision})
	return nil
}

func TestRun_UnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	tool := New(Options{
		Stdout: &stdout,
		Stderr: &stderr,
	})

	code := tool.Run([]string{"badcmd"})
	if code == 0 {
		t.Fatalf("expected non-zero exit code")
	}
	if !strings.Contains(stderr.String(), "unknown command") {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
}

func TestRun_NotifyMissingPayload(t *testing.T) {
	var stdout, stderr bytes.Buffer
	tool := New(Options{
		Stdout: &stdout,
		Stderr: &stderr,
	})

	code := tool.Run([]string{"notify"})
	if code == 0 {
		t.Fatalf("expected non-zero exit code")
	}
	if !strings.Contains(stderr.String(), "notify payload") {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
}

func TestRun_NotifyTriggersNotifierForCompletion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	notifier := &fakeNotifier{}
	tool := New(Options{
		Notifier: notifier,
		Stdout:   &stdout,
		Stderr:   &stderr,
	})

	payload := `{"type":"agent-turn-complete","summary":"done","cwd":"C:\\code\\demo"}`
	code := tool.Run([]string{"notify", payload})
	if code != 0 {
		t.Fatalf("expected zero exit code, got %d, stderr=%q", code, stderr.String())
	}
	if notifier.count != 1 {
		t.Fatalf("expected notifier called once, got %d", notifier.count)
	}
	if notifier.title != "Codex Task Complete" {
		t.Fatalf("unexpected title: %q", notifier.title)
	}
}

func TestRun_NotifyIgnoresUnknownType(t *testing.T) {
	var stdout, stderr bytes.Buffer
	notifier := &fakeNotifier{}
	tool := New(Options{
		Notifier: notifier,
		Stdout:   &stdout,
		Stderr:   &stderr,
	})

	code := tool.Run([]string{"notify", `{"type":"unknown-event"}`})
	if code != 0 {
		t.Fatalf("expected zero exit code, got %d", code)
	}
	if notifier.count != 0 {
		t.Fatalf("expected notifier not called")
	}
}

func TestRun_NotifyFromFile(t *testing.T) {
	temp := t.TempDir()
	payloadPath := filepath.Join(temp, "payload.json")
	payload := `{"type":"agent-turn-complete","summary":"from-file","cwd":"C:\\code\\demo"}`
	if err := os.WriteFile(payloadPath, []byte(payload), 0o644); err != nil {
		t.Fatalf("write payload: %v", err)
	}

	var stdout, stderr bytes.Buffer
	notifier := &fakeNotifier{}
	tool := New(Options{
		Notifier: notifier,
		Stdout:   &stdout,
		Stderr:   &stderr,
	})

	code := tool.Run([]string{"notify", "--file", payloadPath})
	if code != 0 {
		t.Fatalf("expected zero exit code, got %d, stderr=%q", code, stderr.String())
	}
	if notifier.count != 1 {
		t.Fatalf("expected notifier called once, got %d", notifier.count)
	}
	if notifier.body != "from-file\nDir: demo" {
		t.Fatalf("unexpected body: %q", notifier.body)
	}
}

func TestRun_NotifyFromBase64(t *testing.T) {
	raw := `{"type":"agent-turn-complete","summary":"from-b64","cwd":"C:\\code\\demo"}`
	encoded := base64.StdEncoding.EncodeToString([]byte(raw))

	var stdout, stderr bytes.Buffer
	notifier := &fakeNotifier{}
	tool := New(Options{
		Notifier: notifier,
		Stdout:   &stdout,
		Stderr:   &stderr,
	})

	code := tool.Run([]string{"notify", "--b64", encoded})
	if code != 0 {
		t.Fatalf("expected zero exit code, got %d, stderr=%q", code, stderr.String())
	}
	if notifier.count != 1 {
		t.Fatalf("expected notifier called once, got %d", notifier.count)
	}
}

func TestRun_InstallAndUninstall(t *testing.T) {
	temp := t.TempDir()
	configPath := filepath.Join(temp, ".codex", "config.toml")
	claudeConfigPath := filepath.Join(temp, ".claude", "settings.json")
	exePath := `C:\tools\cc-notify.exe`

	var stdout, stderr bytes.Buffer
	notifier := &fakeNotifier{}
	tool := New(Options{
		Notifier:         notifier,
		Stdout:           &stdout,
		Stderr:           &stderr,
		ConfigPath:       func() (string, error) { return configPath, nil },
		ClaudeConfigPath: func() (string, error) { return claudeConfigPath, nil },
		Executable:       func() (string, error) { return exePath, nil },
	})

	code := tool.Run([]string{"install", "codex"})
	if code != 0 {
		t.Fatalf("install codex failed: %q", stderr.String())
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config after install: %v", err)
	}
	if !strings.Contains(string(data), `notify = ["C:\\tools\\cc-notify.exe", "notify"]`) {
		t.Fatalf("notify config was not written: %q", string(data))
	}

	stderr.Reset()
	code = tool.Run([]string{"uninstall", "codex"})
	if code != 0 {
		t.Fatalf("uninstall codex failed: %q", stderr.String())
	}
	data, err = os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config after uninstall: %v", err)
	}
	if strings.Contains(string(data), "notify =") {
		t.Fatalf("notify config should be removed: %q", string(data))
	}
}

func TestRun_InstallAndUninstallClaude(t *testing.T) {
	temp := t.TempDir()
	claudeConfigPath := filepath.Join(temp, ".claude", "settings.json")
	exePath := `C:\tools\cc-notify.exe`

	var stdout, stderr bytes.Buffer
	notifier := &fakeNotifier{}
	tool := New(Options{
		Notifier:         notifier,
		Stdout:           &stdout,
		Stderr:           &stderr,
		ConfigPath:       func() (string, error) { return filepath.Join(temp, ".codex", "config.toml"), nil },
		ClaudeConfigPath: func() (string, error) { return claudeConfigPath, nil },
		Executable:       func() (string, error) { return exePath, nil },
	})

	code := tool.Run([]string{"install", "claude"})
	if code != 0 {
		t.Fatalf("install claude failed: %q", stderr.String())
	}
	data, err := os.ReadFile(claudeConfigPath)
	if err != nil {
		t.Fatalf("read claude config after install: %v", err)
	}
	if !strings.Contains(string(data), "cc-notify") {
		t.Fatalf("claude hook was not written: %q", string(data))
	}
	if !strings.Contains(string(data), "Stop") {
		t.Fatalf("expected Stop hook: %q", string(data))
	}

	stderr.Reset()
	code = tool.Run([]string{"uninstall", "claude"})
	if code != 0 {
		t.Fatalf("uninstall claude failed: %q", stderr.String())
	}
	data, err = os.ReadFile(claudeConfigPath)
	if err != nil {
		t.Fatalf("read claude config after uninstall: %v", err)
	}
	if strings.Contains(string(data), "cc-notify") {
		t.Fatalf("claude hook should be removed: %q", string(data))
	}
}

func TestRun_UninstallHandlesConfigUTF8BOM(t *testing.T) {
	temp := t.TempDir()
	configPath := filepath.Join(temp, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	content := append([]byte{0xEF, 0xBB, 0xBF}, []byte("notify = [\"C:\\\\tools\\\\cc-notify.exe\", \"notify\"]\n[foo]\nbar=1\n")...)
	if err := os.WriteFile(configPath, content, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var stdout, stderr bytes.Buffer
	tool := New(Options{
		Stdout:           &stdout,
		Stderr:           &stderr,
		ConfigPath:       func() (string, error) { return configPath, nil },
		ClaudeConfigPath: func() (string, error) { return filepath.Join(temp, ".claude", "settings.json"), nil },
	})

	code := tool.Run([]string{"uninstall", "codex"})
	if code != 0 {
		t.Fatalf("uninstall failed: %q", stderr.String())
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config after uninstall: %v", err)
	}
	if strings.Contains(string(data), "notify =") {
		t.Fatalf("notify config should be removed with BOM present: %q", string(data))
	}
}

func TestRun_TestNotifyDefaultsAndValidation(t *testing.T) {
	var stdout, stderr bytes.Buffer
	notifier := &fakeNotifier{}
	tool := New(Options{
		Notifier: notifier,
		Stdout:   &stdout,
		Stderr:   &stderr,
	})

	code := tool.Run([]string{"test-notify"})
	if code != 0 {
		t.Fatalf("test-notify should succeed, stderr=%q", stderr.String())
	}
	if notifier.count != 1 {
		t.Fatalf("expected notifier to be called once")
	}
	if notifier.title != "Codex Notification Test" {
		t.Fatalf("unexpected default title: %q", notifier.title)
	}
	if notifier.body != "cc-notify is ready" {
		t.Fatalf("unexpected default body: %q", notifier.body)
	}

	code = tool.Run([]string{"test-notify", " ", " "})
	if code != 0 {
		t.Fatalf("test-notify with blank args should succeed")
	}
	if notifier.count != 2 {
		t.Fatalf("expected notifier to be called twice")
	}
	if notifier.title != "Codex Notification Test" || notifier.body != "cc-notify is ready" {
		t.Fatalf("expected blank args fallback to defaults, got title=%q body=%q", notifier.title, notifier.body)
	}

	stderr.Reset()
	code = tool.Run([]string{"test-notify", "a", "b", "c"})
	if code == 0 {
		t.Fatalf("expected non-zero exit code for too many args")
	}
	if !strings.Contains(stderr.String(), "at most 2 arguments") {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
}

func TestRun_TestToastDefaultsAndValidation(t *testing.T) {
	var stdout, stderr bytes.Buffer
	notifier := &fakeNotifier{}
	tool := New(Options{
		Notifier: notifier,
		Stdout:   &stdout,
		Stderr:   &stderr,
	})

	code := tool.Run([]string{"test-toast"})
	if code != 0 {
		t.Fatalf("test-toast should succeed, stderr=%q", stderr.String())
	}
	if notifier.count != 1 {
		t.Fatalf("expected notifier to be called once")
	}
	if notifier.title != "Codex Toast Test" {
		t.Fatalf("unexpected default title: %q", notifier.title)
	}
	if notifier.body != "toast mode test from cc-notify" {
		t.Fatalf("unexpected default body: %q", notifier.body)
	}

	code = tool.Run([]string{"test-toast", " ", " "})
	if code != 0 {
		t.Fatalf("test-toast with blank args should succeed")
	}
	if notifier.count != 2 {
		t.Fatalf("expected notifier to be called twice")
	}
	if notifier.title != "Codex Toast Test" || notifier.body != "toast mode test from cc-notify" {
		t.Fatalf("expected blank args fallback to defaults, got title=%q body=%q", notifier.title, notifier.body)
	}

	stderr.Reset()
	code = tool.Run([]string{"test-toast", "a", "b", "c"})
	if code == 0 {
		t.Fatalf("expected non-zero exit code for too many args")
	}
	if !strings.Contains(stderr.String(), "at most 2 arguments") {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
}

func TestRun_NotifyRespectsDisabledSetting(t *testing.T) {
	temp := t.TempDir()
	settingsPath := filepath.Join(temp, "settings.json")
	settings := Preferences{
		Enabled:    false,
		Persist:    true,
		Mode:       "auto",
		Content:    "summary",
		ToastAppID: "Windows PowerShell",
	}
	raw, err := json.Marshal(settings)
	if err != nil {
		t.Fatalf("marshal settings: %v", err)
	}
	if err := os.WriteFile(settingsPath, raw, 0o644); err != nil {
		t.Fatalf("write settings: %v", err)
	}

	var stdout, stderr bytes.Buffer
	notifier := &fakeNotifier{}
	tool := New(Options{
		Notifier:     notifier,
		Stdout:       &stdout,
		Stderr:       &stderr,
		SettingsPath: func() (string, error) { return settingsPath, nil },
	})

	code := tool.Run([]string{"notify", `{"type":"agent-turn-complete","summary":"done"}`})
	if code != 0 {
		t.Fatalf("expected zero exit code, stderr=%q", stderr.String())
	}
	if notifier.count != 0 {
		t.Fatalf("expected notification skipped when disabled")
	}
}

func TestRun_NotifyRespectsContentModeComplete(t *testing.T) {
	temp := t.TempDir()
	settingsPath := filepath.Join(temp, "settings.json")
	settings := Preferences{
		Enabled:    true,
		Persist:    true,
		Mode:       "auto",
		Content:    "complete",
		ToastAppID: "Windows PowerShell",
	}
	raw, err := json.Marshal(settings)
	if err != nil {
		t.Fatalf("marshal settings: %v", err)
	}
	if err := os.WriteFile(settingsPath, raw, 0o644); err != nil {
		t.Fatalf("write settings: %v", err)
	}

	var stdout, stderr bytes.Buffer
	notifier := &fakeNotifier{}
	tool := New(Options{
		Notifier:     notifier,
		Stdout:       &stdout,
		Stderr:       &stderr,
		SettingsPath: func() (string, error) { return settingsPath, nil },
	})

	code := tool.Run([]string{"notify", `{"type":"agent-turn-complete","summary":"done","last-assistant-message":"full text"}`})
	if code != 0 {
		t.Fatalf("expected zero exit code, stderr=%q", stderr.String())
	}
	if notifier.count != 1 {
		t.Fatalf("expected notifier to be called once")
	}
	if !strings.HasPrefix(notifier.body, "complete") {
		t.Fatalf("expected complete body mode, got %q", notifier.body)
	}
}

func TestRun_NoArgsOpensInteractiveAndAutoInstalls(t *testing.T) {
	temp := t.TempDir()
	settingsPath := filepath.Join(temp, "settings.json")
	configPath := filepath.Join(temp, ".codex", "config.toml")
	claudeConfigPath := filepath.Join(temp, ".claude", "settings.json")

	var stdout, stderr bytes.Buffer
	tool := New(Options{
		Stdout:           &stdout,
		Stderr:           &stderr,
		Stdin:            strings.NewReader("0\n"),
		SettingsPath:     func() (string, error) { return settingsPath, nil },
		ConfigPath:       func() (string, error) { return configPath, nil },
		ClaudeConfigPath: func() (string, error) { return claudeConfigPath, nil },
		Executable:       func() (string, error) { return `C:\tool\cc-notify.exe`, nil },
	})

	code := tool.Run(nil)
	if code != 0 {
		t.Fatalf("expected zero exit code, stderr=%q", stderr.String())
	}
	if !strings.Contains(stdout.String(), "First launch detected") {
		t.Fatalf("expected first-launch message, got %q", stdout.String())
	}
	if _, err := os.Stat(settingsPath); err != nil {
		t.Fatalf("expected settings file to be created: %v", err)
	}
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("expected codex config to be auto-installed: %v", err)
	}
}

func TestRun_NoArgsWithBOMSettings(t *testing.T) {
	temp := t.TempDir()
	settingsPath := filepath.Join(temp, "settings.json")
	content := append([]byte{0xEF, 0xBB, 0xBF}, []byte(`{"enabled":true,"persist":true,"mode":"toast","content":"summary","toast_app_id":"cc-notify.desktop","setup_done":true}`)...)
	if err := os.WriteFile(settingsPath, content, 0o644); err != nil {
		t.Fatalf("write settings: %v", err)
	}

	var stdout, stderr bytes.Buffer
	tool := New(Options{
		Stdout:       &stdout,
		Stderr:       &stderr,
		Stdin:        strings.NewReader("0\n"),
		SettingsPath: func() (string, error) { return settingsPath, nil },
	})

	code := tool.Run(nil)
	if code != 0 {
		t.Fatalf("expected zero exit code, stderr=%q", stderr.String())
	}
}

func TestRun_NotifyPausedUsesActionableNotification(t *testing.T) {
	temp := t.TempDir()
	settingsPath := filepath.Join(temp, "settings.json")

	var stdout, stderr bytes.Buffer
	actionNotifier := &fakeActionNotifier{}
	tool := New(Options{
		Notifier:     actionNotifier,
		Stdout:       &stdout,
		Stderr:       &stderr,
		SettingsPath: func() (string, error) { return settingsPath, nil },
	})

	code := tool.Run([]string{"notify", `{"type":"agent-turn-paused","summary":"need approval"}`})
	if code != 0 {
		t.Fatalf("expected zero exit code, stderr=%q", stderr.String())
	}
	if actionNotifier.actionCount != 1 {
		t.Fatalf("expected actionable notification, got actionCount=%d", actionNotifier.actionCount)
	}
	if len(actionNotifier.actions) != 3 {
		t.Fatalf("expected 3 actions, got %d", len(actionNotifier.actions))
	}
	if actionNotifier.actions[0].Label != "Yes, proceed" {
		t.Fatalf("unexpected first action label: %q", actionNotifier.actions[0].Label)
	}
	if !strings.Contains(actionNotifier.actions[1].Label, "don't ask again") {
		t.Fatalf("unexpected second action label: %q", actionNotifier.actions[1].Label)
	}
	if actionNotifier.actions[2].Label != "No, tell Codex to do differently" {
		t.Fatalf("unexpected action labels: %+v", actionNotifier.actions)
	}

	uri, err := url.Parse(actionNotifier.actions[0].URI)
	if err != nil {
		t.Fatalf("parse action uri: %v", err)
	}
	if uri.Scheme != "cc-notify" {
		t.Fatalf("unexpected action scheme: %q", uri.Scheme)
	}
	if uri.Host != "respond" {
		t.Fatalf("unexpected action host: %q", uri.Host)
	}
	if got := uri.Query().Get("decision"); got != string(approvalProceed) {
		t.Fatalf("unexpected decision in uri: %q", got)
	}
}

func TestRun_RespondDeliversPendingApproval(t *testing.T) {
	temp := t.TempDir()
	settingsPath := filepath.Join(temp, "settings.json")

	var stdout, stderr bytes.Buffer
	actionNotifier := &fakeActionNotifier{}
	executor := &fakeApprovalExecutor{}
	tool := New(Options{
		Notifier:         actionNotifier,
		Stdout:           &stdout,
		Stderr:           &stderr,
		SettingsPath:     func() (string, error) { return settingsPath, nil },
		ApprovalExecutor: executor,
	})

	code := tool.Run([]string{"notify", `{"type":"agent-turn-paused","summary":"need approval"}`})
	if code != 0 {
		t.Fatalf("notify paused failed: stderr=%q", stderr.String())
	}
	if len(actionNotifier.actions) == 0 {
		t.Fatalf("expected action uri to be generated")
	}
	uri, err := url.Parse(actionNotifier.actions[0].URI)
	if err != nil {
		t.Fatalf("parse action uri: %v", err)
	}
	id := uri.Query().Get("id")
	if id == "" {
		t.Fatalf("expected approval id in action uri")
	}

	stderr.Reset()
	code = tool.Run([]string{"respond", "--id", id, "--decision", "approve"})
	if code != 0 {
		t.Fatalf("respond failed: stderr=%q", stderr.String())
	}
	if len(executor.calls) != 1 {
		t.Fatalf("expected one executor call, got %d", len(executor.calls))
	}
	if executor.calls[0].decision != approvalProceed {
		t.Fatalf("unexpected decision: %q", executor.calls[0].decision)
	}
}

func TestRun_ProtocolURIRespond(t *testing.T) {
	temp := t.TempDir()
	settingsPath := filepath.Join(temp, "settings.json")

	var stdout, stderr bytes.Buffer
	actionNotifier := &fakeActionNotifier{}
	executor := &fakeApprovalExecutor{}
	tool := New(Options{
		Notifier:         actionNotifier,
		Stdout:           &stdout,
		Stderr:           &stderr,
		SettingsPath:     func() (string, error) { return settingsPath, nil },
		ApprovalExecutor: executor,
	})

	code := tool.Run([]string{"notify", `{"type":"agent-turn-paused","summary":"need approval"}`})
	if code != 0 {
		t.Fatalf("notify paused failed: stderr=%q", stderr.String())
	}
	uri := actionNotifier.actions[0].URI

	stderr.Reset()
	code = tool.Run([]string{uri})
	if code != 0 {
		t.Fatalf("protocol respond failed: stderr=%q", stderr.String())
	}
	if len(executor.calls) != 1 {
		t.Fatalf("expected one executor call, got %d", len(executor.calls))
	}
}
