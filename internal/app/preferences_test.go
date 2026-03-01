package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultPreferences_UsesCodexToastAppID(t *testing.T) {
	p := DefaultPreferences()
	if p.ToastAppID != "cc-notify.desktop" {
		t.Fatalf("expected default toast app id cc-notify.desktop, got %q", p.ToastAppID)
	}
	if p.Mode != "toast" {
		t.Fatalf("expected default mode toast, got %q", p.Mode)
	}
	if p.PausePrompt != "toast" {
		t.Fatalf("expected default pause prompt toast, got %q", p.PausePrompt)
	}
}

func TestNormalizePreferences_MigratesLegacyToastAppID(t *testing.T) {
	p := normalizePreferences(Preferences{
		Enabled:    true,
		Persist:    true,
		Mode:       "toast",
		Content:    "summary",
		ToastAppID: "Windows PowerShell",
	})
	if p.ToastAppID != "cc-notify.desktop" {
		t.Fatalf("expected migrated toast app id cc-notify.desktop, got %q", p.ToastAppID)
	}
}

func TestLoadPreferences_AcceptsUTF8BOM(t *testing.T) {
	temp := t.TempDir()
	settingsPath := filepath.Join(temp, "settings.json")
	content := append([]byte{0xEF, 0xBB, 0xBF}, []byte(`{"enabled":true,"persist":true,"mode":"toast","content":"summary","toast_app_id":"cc-notify.desktop"}`)...)
	if err := os.WriteFile(settingsPath, content, 0o644); err != nil {
		t.Fatalf("write settings: %v", err)
	}

	tool := New(Options{
		SettingsPath: func() (string, error) { return settingsPath, nil },
	})

	prefs, exists, err := tool.loadPreferences()
	if err != nil {
		t.Fatalf("load preferences: %v", err)
	}
	if !exists {
		t.Fatalf("expected settings file to exist")
	}
	if prefs.Mode != "toast" {
		t.Fatalf("expected mode toast, got %q", prefs.Mode)
	}
}
