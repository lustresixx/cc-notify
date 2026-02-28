package app

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"cc-notify/internal/config"
	"cc-notify/internal/event"
	"cc-notify/internal/notifier"
)

// Options controls runtime dependencies for App.
type Options struct {
	Notifier         notifier.Service
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

	var err error
	switch args[0] {
	case "install":
		err = a.runInstall(args[1:])
	case "uninstall":
		err = a.runUninstall(args[1:])
	case "notify":
		err = a.runNotify(args[1:])
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
	fmt.Fprintf(a.stdout, "    cc-notify test-notify [title] [body]   %ssend test notification%s\n", colorDim, colorReset)
	fmt.Fprintf(a.stdout, "    cc-notify test-toast [title] [body]    %stest toast mode%s\n", colorDim, colorReset)
	fmt.Fprintf(a.stdout, "    cc-notify help                         %sshow this help%s\n\n", colorDim, colorReset)
}
