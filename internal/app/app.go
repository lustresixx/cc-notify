package app

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"cc-notify/internal/config"
	"cc-notify/internal/event"
	"cc-notify/internal/notifier"
)

// Options controls runtime dependencies for App.
type Options struct {
	Notifier         notifier.Service
	ApprovalExecutor ApprovalExecutor
	Stdin            io.Reader
	Stdout           io.Writer
	Stderr           io.Writer
	ConfigPath       func() (string, error)
	ClaudeConfigPath func() (string, error)
	SettingsPath     func() (string, error)
	Executable       func() (string, error)
	ReadFile         func(string) ([]byte, error)
	WriteFile        func(string, []byte, fs.FileMode) error
	MkdirAll         func(string, fs.FileMode) error
}

// App is the CLI command dispatcher.
type App struct {
	notifier         notifier.Service
	approvalExecutor ApprovalExecutor
	defaultNotifier  bool
	stdin            io.Reader
	stdout           io.Writer
	stderr           io.Writer
	configPath       func() (string, error)
	claudeConfigPath func() (string, error)
	settingsPath     func() (string, error)
	executable       func() (string, error)
	readFile         func(string) ([]byte, error)
	writeFile        func(string, []byte, fs.FileMode) error
	mkdirAll         func(string, fs.FileMode) error
}

// New builds an App with defaults.
func New(opts Options) *App {
	defaultNotifier := false
	if opts.Notifier == nil {
		defaultNotifier = true
		opts.Notifier = notifier.New()
	}
	if opts.ApprovalExecutor == nil {
		opts.ApprovalExecutor = newDefaultApprovalExecutor()
	}
	if opts.Stdin == nil {
		opts.Stdin = os.Stdin
	}
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	if opts.Stderr == nil {
		opts.Stderr = os.Stderr
	}
	if opts.ConfigPath == nil {
		opts.ConfigPath = config.DefaultPath
	}
	if opts.ClaudeConfigPath == nil {
		opts.ClaudeConfigPath = config.ClaudeDefaultPath
	}
	if opts.SettingsPath == nil {
		opts.SettingsPath = defaultSettingsPath
	}
	if opts.Executable == nil {
		opts.Executable = os.Executable
	}
	if opts.ReadFile == nil {
		opts.ReadFile = os.ReadFile
	}
	if opts.WriteFile == nil {
		opts.WriteFile = os.WriteFile
	}
	if opts.MkdirAll == nil {
		opts.MkdirAll = os.MkdirAll
	}

	return &App{
		notifier:         opts.Notifier,
		approvalExecutor: opts.ApprovalExecutor,
		defaultNotifier:  defaultNotifier,
		stdin:            opts.Stdin,
		stdout:           opts.Stdout,
		stderr:           opts.Stderr,
		configPath:       opts.ConfigPath,
		claudeConfigPath: opts.ClaudeConfigPath,
		settingsPath:     opts.SettingsPath,
		executable:       opts.Executable,
		readFile:         opts.ReadFile,
		writeFile:        opts.WriteFile,
		mkdirAll:         opts.MkdirAll,
	}
}

// Run executes CLI command args and returns an exit code.
func (a *App) Run(args []string) int {
	if len(args) == 0 {
		if err := a.runInteractive(); err != nil {
			fmt.Fprintf(a.stderr, "error: %v\n", err)
			return 1
		}
		return 0
	}

	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(args[0])), "cc-notify://") {
		err := a.runProtocolURI(args[0])
		if err != nil {
			fmt.Fprintf(a.stderr, "error: %v\n", err)
			return 1
		}
		return 0
	}

	var err error
	switch args[0] {
	case "install":
		err = a.runInstall(args[1:])
	case "uninstall":
		err = a.runUninstall(args[1:])
	case "notify":
		err = a.runNotify(args[1:])
	case "respond":
		err = a.runRespond(args[1:])
	case "test-notify":
		err = a.runTestNotify(args[1:])
	case "test-toast":
		err = a.runTestToast(args[1:])
	case "help", "-h", "--help":
		a.printUsage()
		return 0
	default:
		err = fmt.Errorf("unknown command: %s", args[0])
	}

	if err != nil {
		fmt.Fprintf(a.stderr, "error: %v\n", err)
		return 1
	}
	return 0
}

func (a *App) runInstall(args []string) error {
	target := ""
	if len(args) > 0 {
		target = args[0]
	}
	if len(args) > 1 {
		return fmt.Errorf("install accepts at most 1 argument (codex, claude, or empty for both)")
	}

	exePath, err := a.executable()
	if err != nil {
		return err
	}
	if !filepath.IsAbs(exePath) {
		exePath, err = filepath.Abs(exePath)
		if err != nil {
			return fmt.Errorf("resolve executable path: %w", err)
		}
	}

	switch target {
	case "", "all":
		if err := a.installCodex(exePath); err != nil {
			fmt.Fprintf(a.stderr, "  codex install: %v\n", err)
		}
		if err := a.installClaude(exePath); err != nil {
			fmt.Fprintf(a.stderr, "  claude install: %v\n", err)
		}
		return nil
	case "codex":
		return a.installCodex(exePath)
	case "claude":
		return a.installClaude(exePath)
	default:
		return fmt.Errorf("unknown install target: %s (use codex, claude, or leave empty for both)", target)
	}
}

func (a *App) installCodex(exePath string) error {
	cfgPath, err := a.configPath()
	if err != nil {
		return err
	}

	content, err := a.readFile(cfgPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read config: %w", err)
	}

	updated, changed, err := config.UpsertNotify(string(content), []string{exePath, "notify"})
	if err != nil {
		return err
	}
	if !changed {
		fmt.Fprintln(a.stdout, "codex: notify command already configured")
		return nil
	}

	if err := a.mkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	if err := a.writeFile(cfgPath, []byte(updated), 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	fmt.Fprintf(a.stdout, "codex: installed notify command in %s\n", cfgPath)
	return nil
}

func (a *App) installClaude(exePath string) error {
	cfgPath, err := a.claudeConfigPath()
	if err != nil {
		return err
	}

	content, err := a.readFile(cfgPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read claude settings: %w", err)
	}

	updated, changed, err := config.ClaudeUpsertHook(string(content), exePath)
	if err != nil {
		return err
	}
	if !changed {
		fmt.Fprintln(a.stdout, "claude: hook already configured")
		return nil
	}

	if err := a.mkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		return fmt.Errorf("create claude config directory: %w", err)
	}
	if err := a.writeFile(cfgPath, []byte(updated), 0o644); err != nil {
		return fmt.Errorf("write claude settings: %w", err)
	}
	fmt.Fprintf(a.stdout, "claude: installed hook in %s\n", cfgPath)
	return nil
}

func (a *App) runUninstall(args []string) error {
	target := ""
	if len(args) > 0 {
		target = args[0]
	}
	if len(args) > 1 {
		return fmt.Errorf("uninstall accepts at most 1 argument (codex, claude, or empty for both)")
	}

	switch target {
	case "", "all":
		if err := a.uninstallCodex(); err != nil {
			fmt.Fprintf(a.stderr, "  codex uninstall: %v\n", err)
		}
		if err := a.uninstallClaude(); err != nil {
			fmt.Fprintf(a.stderr, "  claude uninstall: %v\n", err)
		}
		return nil
	case "codex":
		return a.uninstallCodex()
	case "claude":
		return a.uninstallClaude()
	default:
		return fmt.Errorf("unknown uninstall target: %s (use codex, claude, or leave empty for both)", target)
	}
}

func (a *App) uninstallCodex() error {
	cfgPath, err := a.configPath()
	if err != nil {
		return err
	}

	content, err := a.readFile(cfgPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintln(a.stdout, "codex: config file not found, nothing to uninstall")
			return nil
		}
		return fmt.Errorf("read config: %w", err)
	}

	updated, changed, err := config.RemoveNotify(string(content))
	if err != nil {
		return err
	}
	if !changed {
		fmt.Fprintln(a.stdout, "codex: notify command not configured")
		return nil
	}

	if err := a.writeFile(cfgPath, []byte(updated), 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	fmt.Fprintf(a.stdout, "codex: removed notify command from %s\n", cfgPath)
	return nil
}

func (a *App) uninstallClaude() error {
	cfgPath, err := a.claudeConfigPath()
	if err != nil {
		return err
	}

	content, err := a.readFile(cfgPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintln(a.stdout, "claude: settings file not found, nothing to uninstall")
			return nil
		}
		return fmt.Errorf("read claude settings: %w", err)
	}

	updated, changed, err := config.ClaudeRemoveHook(string(content))
	if err != nil {
		return err
	}
	if !changed {
		fmt.Fprintln(a.stdout, "claude: hook not configured")
		return nil
	}

	if err := a.writeFile(cfgPath, []byte(updated), 0o644); err != nil {
		return fmt.Errorf("write claude settings: %w", err)
	}
	fmt.Fprintf(a.stdout, "claude: removed hook from %s\n", cfgPath)
	return nil
}

func (a *App) runNotify(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("notify payload argument is required")
	}

	var raw string
	var err error
	source := "codex"

	if args[0] == "--claude" {
		source = "claude"
		raw, err = a.readClaudeHookInput()
	} else {
		raw, err = a.resolveNotifyPayload(args)
	}
	if err != nil {
		return err
	}

	payload, err := event.ParsePayload(raw)
	if err != nil {
		return err
	}

	prefs, _, err := a.loadPreferences()
	if err != nil {
		return err
	}

	enabled, mode, content := prefs.ToolPrefs(source)
	if !enabled {
		fmt.Fprintf(a.stdout, "notifications disabled for %s\n", source)
		return nil
	}

	title, body, ok := event.RenderNotificationWithOptions(payload, event.RenderOptions{
		ContentMode:  event.ContentMode(content),
		IncludeDir:   prefs.IncludeDir,
		IncludeModel: prefs.IncludeModel,
		IncludeEvent: prefs.IncludeEvent,
	})
	if !ok {
		fmt.Fprintf(a.stdout, "ignored event type: %s\n", payload.Type)
		return nil
	}

	service := a.notifier
	if a.defaultNotifier {
		service = notifier.NewWithConfig(notifier.Config{
			Mode:       mode,
			ToastAppID: prefs.ToastAppID,
		})
	}

	if payload.Type == "agent-turn-paused" {
		if actionService, ok := service.(notifier.ActionService); ok {
			pending, createErr := a.createPendingApproval(os.Getppid())
			if createErr != nil {
				return fmt.Errorf("create pending approval: %w", createErr)
			}
			actions := buildPausedActions(payload.Summary, pending.ID)
			if err := actionService.NotifyWithActions(title, body, actions); err != nil {
				_ = a.deletePendingApproval(pending.ID)
				return err
			}
			fmt.Fprintf(a.stdout, "notification sent: %s (%s)\n", payload.Type, source)
			return nil
		}
	}

	if err := service.Notify(title, body); err != nil {
		return err
	}
	fmt.Fprintf(a.stdout, "notification sent: %s (%s)\n", payload.Type, source)
	return nil
}

func (a *App) resolveNotifyPayload(args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("notify payload argument is required")
	}

	switch args[0] {
	case "--file":
		if len(args) < 2 {
			return "", fmt.Errorf("notify --file requires a path argument")
		}
		data, err := a.readFile(args[1])
		if err != nil {
			return "", fmt.Errorf("read notify payload file: %w", err)
		}
		return strings.TrimSpace(string(data)), nil
	case "--b64":
		if len(args) < 2 {
			return "", fmt.Errorf("notify --b64 requires a base64 payload argument")
		}
		decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(args[1]))
		if err != nil {
			return "", fmt.Errorf("decode notify base64 payload: %w", err)
		}
		return strings.TrimSpace(string(decoded)), nil
	default:
		return strings.TrimSpace(strings.Join(args, " ")), nil
	}
}

// readClaudeHookInput reads Claude Code hook input from stdin and converts it
// to a cc-notify event payload JSON string.
// Claude Code "Stop" hook sends JSON via stdin with structure like:
//
//	{
//	  "hook_type": "Stop",
//	  "stop_hook_active": true,
//	  "transcript_path": "...",
//	  "session_id": "...",
//	  ...
//	}
func (a *App) readClaudeHookInput() (string, error) {
	data, err := io.ReadAll(a.stdin)
	if err != nil {
		return "", fmt.Errorf("read claude hook stdin: %w", err)
	}
	raw := strings.TrimSpace(string(data))
	if raw == "" {
		return "", fmt.Errorf("empty claude hook input")
	}

	// Parse the Claude hook JSON
	var claudeInput map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &claudeInput); err != nil {
		return "", fmt.Errorf("parse claude hook input: %w", err)
	}

	// Map Claude hook type to our event type
	hookType, _ := claudeInput["hook_type"].(string)
	eventType := "agent-turn-complete"
	switch hookType {
	case "Stop":
		eventType = "agent-turn-complete"
	default:
		eventType = "agent-turn-complete"
	}

	// Extract useful fields
	summary := ""
	cwd := ""
	model := ""
	transcriptPath := ""

	if v, ok := claudeInput["cwd"].(string); ok {
		cwd = v
	}
	if v, ok := claudeInput["session_id"].(string); ok && summary == "" {
		summary = "Claude Code session " + v + " completed"
	}
	if v, ok := claudeInput["transcript_path"].(string); ok {
		transcriptPath = v
	}
	if v, ok := claudeInput["model"].(string); ok {
		model = v
	}

	// Build our standard payload
	payload := event.Payload{
		Type:           eventType,
		Summary:        summary,
		CWD:            cwd,
		Model:          model,
		TranscriptPath: transcriptPath,
	}

	result, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal converted payload: %w", err)
	}
	return string(result), nil
}

func (a *App) runTestNotify(args []string) error {
	title := "Codex Notification Test"
	body := "cc-notify is ready"
	if len(args) > 0 {
		title = strings.TrimSpace(args[0])
	}
	if len(args) > 1 {
		body = strings.TrimSpace(args[1])
	}
	if len(args) > 2 {
		return fmt.Errorf("test-notify accepts at most 2 arguments")
	}
	if title == "" {
		title = "Codex Notification Test"
	}
	if body == "" {
		body = "cc-notify is ready"
	}
	return a.notifier.Notify(title, body)
}

func (a *App) runTestToast(args []string) error {
	title := "Codex Toast Test"
	body := "toast mode test from cc-notify"
	if len(args) > 0 {
		title = strings.TrimSpace(args[0])
	}
	if len(args) > 1 {
		body = strings.TrimSpace(args[1])
	}
	if len(args) > 2 {
		return fmt.Errorf("test-toast accepts at most 2 arguments")
	}
	if title == "" {
		title = "Codex Toast Test"
	}
	if body == "" {
		body = "toast mode test from cc-notify"
	}

	prefs, _, err := a.loadPreferences()
	if err != nil {
		return err
	}

	service := a.notifier
	if a.defaultNotifier {
		service = notifier.NewWithConfig(notifier.Config{
			Mode:       "toast",
			ToastAppID: prefs.ToastAppID,
		})
	}
	if err := service.Notify(title, body); err != nil {
		return fmt.Errorf("toast test failed (app id: %s): %w", prefs.ToastAppID, err)
	}
	fmt.Fprintf(a.stdout, "toast test sent (app id: %s). If no banner appears, check Notification Center (Win + A).\n", prefs.ToastAppID)
	return nil
}

func (a *App) runRespond(args []string) error {
	id := ""
	decision := approvalDecision("")

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--id":
			if i+1 >= len(args) {
				return fmt.Errorf("respond --id requires a value")
			}
			id = strings.TrimSpace(args[i+1])
			i++
		case "--decision":
			if i+1 >= len(args) {
				return fmt.Errorf("respond --decision requires a value")
			}
			d, err := parseApprovalDecision(args[i+1])
			if err != nil {
				return err
			}
			decision = d
			i++
		case "--approve":
			decision = approvalApprove
		case "--reject":
			decision = approvalReject
		default:
			return fmt.Errorf("unknown respond option: %s", args[i])
		}
	}

	if id == "" {
		return fmt.Errorf("respond requires --id")
	}
	if decision == "" {
		return fmt.Errorf("respond requires --decision (approve|reject) or --approve/--reject")
	}

	pending, err := a.loadPendingApproval(id)
	if err != nil {
		_ = a.notifier.Notify("Codex Approval", "Unable to apply response: request not found or expired.")
		return err
	}
	if time.Now().Unix() > pending.ExpiresAtUnix {
		_ = a.deletePendingApproval(id)
		_ = a.notifier.Notify("Codex Approval", "Unable to apply response: request expired.")
		return fmt.Errorf("approval request expired: %s", id)
	}

	if err := a.approvalExecutor.Deliver(pending.ParentPID, decision); err != nil {
		_ = a.notifier.Notify("Codex Approval", "Unable to apply response automatically. Open terminal and answer manually.")
		return err
	}
	_ = a.deletePendingApproval(id)

	_ = a.notifier.Notify("Codex Approval", "Response sent: "+string(decision))
	fmt.Fprintf(a.stdout, "approval response delivered: %s\n", decision)
	return nil
}

func (a *App) runProtocolURI(raw string) error {
	id, decision, err := parseApprovalProtocolURI(raw)
	if err != nil {
		return err
	}
	return a.runRespond([]string{"--id", id, "--decision", string(decision)})
}

type pendingApproval struct {
	ID            string `json:"id"`
	ParentPID     int    `json:"parent_pid"`
	CreatedAtUnix int64  `json:"created_at_unix"`
	ExpiresAtUnix int64  `json:"expires_at_unix"`
}

func (a *App) createPendingApproval(parentPID int) (pendingApproval, error) {
	id, err := randomApprovalID()
	if err != nil {
		return pendingApproval{}, err
	}
	now := time.Now().Unix()
	item := pendingApproval{
		ID:            id,
		ParentPID:     parentPID,
		CreatedAtUnix: now,
		ExpiresAtUnix: now + int64((15 * time.Minute).Seconds()),
	}
	data, err := json.Marshal(item)
	if err != nil {
		return pendingApproval{}, fmt.Errorf("marshal pending approval: %w", err)
	}
	path, err := a.pendingApprovalPath(id)
	if err != nil {
		return pendingApproval{}, err
	}
	if err := a.mkdirAll(filepath.Dir(path), 0o755); err != nil {
		return pendingApproval{}, fmt.Errorf("create approvals directory: %w", err)
	}
	if err := a.writeFile(path, data, 0o644); err != nil {
		return pendingApproval{}, fmt.Errorf("write pending approval: %w", err)
	}
	return item, nil
}

func (a *App) loadPendingApproval(id string) (pendingApproval, error) {
	path, err := a.pendingApprovalPath(id)
	if err != nil {
		return pendingApproval{}, err
	}
	data, err := a.readFile(path)
	if err != nil {
		return pendingApproval{}, fmt.Errorf("read pending approval: %w", err)
	}
	var item pendingApproval
	if err := json.Unmarshal(data, &item); err != nil {
		return pendingApproval{}, fmt.Errorf("parse pending approval: %w", err)
	}
	if item.ID == "" || item.ParentPID <= 0 {
		return pendingApproval{}, fmt.Errorf("pending approval is invalid: %s", id)
	}
	return item, nil
}

func (a *App) deletePendingApproval(id string) error {
	path, err := a.pendingApprovalPath(id)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (a *App) pendingApprovalPath(id string) (string, error) {
	if !isValidApprovalID(id) {
		return "", fmt.Errorf("invalid approval id")
	}
	settingsPath, err := a.settingsPath()
	if err != nil {
		return "", fmt.Errorf("resolve settings path: %w", err)
	}
	return filepath.Join(filepath.Dir(settingsPath), "approvals", id+".json"), nil
}

func randomApprovalID() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate approval id: %w", err)
	}
	return fmt.Sprintf("%x", buf), nil
}

func isValidApprovalID(id string) bool {
	if len(id) != 16 {
		return false
	}
	for _, r := range id {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

func approvalActionURI(id string, decision approvalDecision) string {
	q := url.Values{}
	q.Set("id", id)
	q.Set("decision", string(decision))
	return "cc-notify://respond?" + q.Encode()
}

func parseApprovalProtocolURI(raw string) (string, approvalDecision, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", "", fmt.Errorf("parse protocol uri: %w", err)
	}
	if strings.ToLower(u.Scheme) != "cc-notify" {
		return "", "", fmt.Errorf("unsupported protocol scheme: %s", u.Scheme)
	}
	target := strings.Trim(strings.ToLower(u.Host+u.Path), "/")
	if target != "respond" {
		return "", "", fmt.Errorf("unsupported protocol action: %s", target)
	}
	id := strings.TrimSpace(u.Query().Get("id"))
	if id == "" {
		return "", "", fmt.Errorf("protocol uri missing id")
	}
	decision, err := parseApprovalDecision(u.Query().Get("decision"))
	if err != nil {
		return "", "", err
	}
	return id, decision, nil
}

func parseApprovalDecision(raw string) (approvalDecision, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "proceed", "once", "1":
		return approvalProceed, nil
	case "proceed-always", "always", "persist", "2":
		return approvalProceedAlways, nil
	case "approve", "yes", "y":
		return approvalProceed, nil
	case "reject", "no", "n":
		return approvalReject, nil
	case "esc", "escape", "3":
		return approvalReject, nil
	default:
		return "", fmt.Errorf("unsupported decision: %s", strconv.Quote(raw))
	}
}

func buildPausedActions(summary, id string) []notifier.Action {
	secondLabel := "Yes, and don't ask again for this command pattern"
	if cmd := firstBacktickValue(summary); cmd != "" {
		secondLabel = "Yes, don't ask again for `" + cmd + "`"
	}
	return []notifier.Action{
		{Label: "Yes, proceed", URI: approvalActionURI(id, approvalProceed)},
		{Label: secondLabel, URI: approvalActionURI(id, approvalProceedAlways)},
		{Label: "No, tell Codex to do differently", URI: approvalActionURI(id, approvalReject)},
	}
}

func firstBacktickValue(input string) string {
	raw := strings.TrimSpace(input)
	start := strings.Index(raw, "`")
	if start < 0 {
		return ""
	}
	end := strings.Index(raw[start+1:], "`")
	if end < 0 {
		return ""
	}
	value := strings.TrimSpace(raw[start+1 : start+1+end])
	if value == "" {
		return ""
	}
	runes := []rune(value)
	if len(runes) > 56 {
		return string(runes[:56]) + "..."
	}
	return value
}

func (a *App) printUsage() {
	fmt.Fprintf(a.stdout, "\n  %s%s⚡ cc-notify%s %s%s%s\n", colorBold, colorCyan, colorReset, colorDim, version, colorReset)
	fmt.Fprintf(a.stdout, "  %sWindows notifications for Codex CLI & Claude Code%s\n\n", colorDim, colorReset)
	fmt.Fprintf(a.stdout, "  %s%sUsage:%s\n", colorBold, colorYellow, colorReset)
	fmt.Fprintf(a.stdout, "    cc-notify                              %sinteractive settings%s\n", colorDim, colorReset)
	fmt.Fprintf(a.stdout, "    cc-notify install [codex|claude]       %sregister hooks (both if omitted)%s\n", colorDim, colorReset)
	fmt.Fprintf(a.stdout, "    cc-notify uninstall [codex|claude]     %sremove hooks (both if omitted)%s\n", colorDim, colorReset)
	fmt.Fprintf(a.stdout, "    cc-notify notify <json>                %shandle Codex event payload%s\n", colorDim, colorReset)
	fmt.Fprintf(a.stdout, "    cc-notify notify --claude              %shandle Claude Code hook (stdin)%s\n", colorDim, colorReset)
	fmt.Fprintf(a.stdout, "    cc-notify notify --file <path>         %sread payload from file%s\n", colorDim, colorReset)
	fmt.Fprintf(a.stdout, "    cc-notify notify --b64 <base64>        %sbase64 encoded payload%s\n", colorDim, colorReset)
	fmt.Fprintf(a.stdout, "    cc-notify respond --id <id> --decision <proceed|proceed-always|reject> %sapply pause response%s\n", colorDim, colorReset)
	fmt.Fprintf(a.stdout, "    cc-notify test-notify [title] [body]   %ssend test notification%s\n", colorDim, colorReset)
	fmt.Fprintf(a.stdout, "    cc-notify test-toast [title] [body]    %stest toast mode%s\n", colorDim, colorReset)
	fmt.Fprintf(a.stdout, "    cc-notify help                         %sshow this help%s\n\n", colorDim, colorReset)
}
