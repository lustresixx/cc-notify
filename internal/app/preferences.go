package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	defaultNotifyMode  = "auto"
	defaultContentMode = "summary"
	defaultToastAppID  = "cc-notify.desktop"
	legacyToastAppID   = "Windows PowerShell"
	legacyToastAppID2  = "codex-notified.desktop"
)

// Preferences stores user-facing behavior controls for notifications.
type Preferences struct {
	Enabled          bool   `json:"enabled"`
	Persist          bool   `json:"persist"`
	Mode             string `json:"mode"`
	Content          string `json:"content"`
	IncludeDir       bool   `json:"include_dir"`
	IncludeModel     bool   `json:"include_model"`
	IncludeEvent     bool   `json:"include_event"`
	FieldsConfigured bool   `json:"fields_configured"`
	ToastAppID       string `json:"toast_app_id"`
	SetupDone        bool   `json:"setup_done"`

	// Per-tool overrides. Empty string means "use global default".
	CodexEnabled  *bool  `json:"codex_enabled,omitempty"`
	CodexMode     string `json:"codex_mode,omitempty"`
	CodexContent  string `json:"codex_content,omitempty"`
	ClaudeEnabled *bool  `json:"claude_enabled,omitempty"`
	ClaudeMode    string `json:"claude_mode,omitempty"`
	ClaudeContent string `json:"claude_content,omitempty"`
}

// ToolPrefs returns the effective mode/content/enabled for the given source.
// source is "codex" or "claude". Falls back to global defaults.
func (p Preferences) ToolPrefs(source string) (enabled bool, mode string, content string) {
	enabled = p.Enabled
	mode = p.Mode
	content = p.Content

	switch source {
	case "codex":
		if p.CodexEnabled != nil {
			enabled = *p.CodexEnabled
		}
		if p.CodexMode != "" {
			mode = p.CodexMode
		}
		if p.CodexContent != "" {
			content = p.CodexContent
		}
	case "claude":
		if p.ClaudeEnabled != nil {
			enabled = *p.ClaudeEnabled
		}
		if p.ClaudeMode != "" {
			mode = p.ClaudeMode
		}
		if p.ClaudeContent != "" {
			content = p.ClaudeContent
		}
	}
	return
}

func DefaultPreferences() Preferences {
	return Preferences{
		Enabled:          true,
		Persist:          true,
		Mode:             defaultNotifyMode,
		Content:          defaultContentMode,
		IncludeDir:       true,
		IncludeModel:     false,
		IncludeEvent:     false,
		FieldsConfigured: true,
		ToastAppID:       defaultToastAppID,
		SetupDone:        false,
	}
}

func normalizePreferences(p Preferences) Preferences {
	def := DefaultPreferences()
	if strings.TrimSpace(p.Mode) == "" {
		p.Mode = def.Mode
	}
	if strings.TrimSpace(p.Content) == "" {
		p.Content = def.Content
	}
	if strings.TrimSpace(p.ToastAppID) == "" {
		p.ToastAppID = def.ToastAppID
	}
	if p.ToastAppID == legacyToastAppID || p.ToastAppID == legacyToastAppID2 {
		p.ToastAppID = def.ToastAppID
	}
	if p.Mode != "auto" && p.Mode != "toast" && p.Mode != "popup" {
		p.Mode = def.Mode
	}
	switch p.Content {
	case "complete", "summary", "full":
	default:
		p.Content = def.Content
	}
	if !p.FieldsConfigured {
		p.IncludeDir = def.IncludeDir
		p.IncludeModel = def.IncludeModel
		p.IncludeEvent = def.IncludeEvent
		p.FieldsConfigured = true
	}
	return p
}

func defaultSettingsPath() (string, error) {
	if localAppData := strings.TrimSpace(os.Getenv("LOCALAPPDATA")); localAppData != "" {
		return filepath.Join(localAppData, "cc-notify", "settings.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve settings path: %w", err)
	}
	return filepath.Join(home, ".cc-notify", "settings.json"), nil
}

func (a *App) loadPreferences() (Preferences, bool, error) {
	path, err := a.settingsPath()
	if err != nil {
		return Preferences{}, false, err
	}

	raw, err := a.readFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultPreferences(), false, nil
		}
		return Preferences{}, false, fmt.Errorf("read preferences: %w", err)
	}

	var p Preferences
	raw = stripUTF8BOM(raw)
	if err := json.Unmarshal(raw, &p); err != nil {
		return Preferences{}, false, fmt.Errorf("parse preferences: %w", err)
	}
	return normalizePreferences(p), true, nil
}

func stripUTF8BOM(raw []byte) []byte {
	return bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF})
}

func (a *App) savePreferences(p Preferences) error {
	path, err := a.settingsPath()
	if err != nil {
		return err
	}
	p = normalizePreferences(p)

	if err := a.mkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create settings directory: %w", err)
	}
	raw, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("encode preferences: %w", err)
	}
	raw = append(raw, '\n')
	if err := a.writeFile(path, raw, 0o644); err != nil {
		return fmt.Errorf("write preferences: %w", err)
	}
	return nil
}
