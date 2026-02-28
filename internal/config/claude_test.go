package config

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestClaudeUpsertHook_EmptySettings(t *testing.T) {
	out, changed, err := ClaudeUpsertHook("", `C:\tools\cc-notify.exe`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !changed {
		t.Fatal("expected changed")
	}
	if !strings.Contains(out, "cc-notify") {
		t.Fatalf("expected cc-notify in output: %q", out)
	}
	if !strings.Contains(out, "Stop") {
		t.Fatalf("expected Stop hook: %q", out)
	}

	// Verify valid JSON
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
}

func TestClaudeUpsertHook_ExistingSettings(t *testing.T) {
	existing := `{"permissions":{"allow":["Bash(git *)"]}}` + "\n"
	out, changed, err := ClaudeUpsertHook(existing, `C:\tools\cc-notify.exe`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !changed {
		t.Fatal("expected changed")
	}
	if !strings.Contains(out, "cc-notify") {
		t.Fatalf("expected hook in output: %q", out)
	}
	if !strings.Contains(out, "permissions") {
		t.Fatalf("expected existing settings preserved: %q", out)
	}
}

func TestClaudeUpsertHook_ReplaceExisting(t *testing.T) {
	existing := `{
  "hooks": {
    "Stop": [
      {
        "matcher": "",
        "hooks": [
          {"type": "command", "command": "old-cc-notify notify --claude"}
        ]
      }
    ]
  }
}`
	out, changed, err := ClaudeUpsertHook(existing, `C:\tools\cc-notify.exe`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !changed {
		t.Fatal("expected changed")
	}
	if strings.Contains(out, "old-cc-notify") {
		t.Fatalf("expected old hook to be replaced: %q", out)
	}
	if !strings.Contains(out, `C:\\tools\\cc-notify.exe`) {
		t.Fatalf("expected new path: %q", out)
	}
}

func TestClaudeRemoveHook_RemovesOurHook(t *testing.T) {
	existing := `{
  "hooks": {
    "Stop": [
      {
        "matcher": "",
        "hooks": [
          {"type": "command", "command": "C:\\tools\\cc-notify.exe notify --claude"}
        ]
      }
    ]
  }
}`
	out, changed, err := ClaudeRemoveHook(existing)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !changed {
		t.Fatal("expected changed")
	}
	if strings.Contains(out, "cc-notify") {
		t.Fatalf("expected hook to be removed: %q", out)
	}
}

func TestClaudeRemoveHook_PreservesOtherHooks(t *testing.T) {
	existing := `{
  "hooks": {
    "Stop": [
      {
        "matcher": "",
        "hooks": [
          {"type": "command", "command": "some-other-tool"},
          {"type": "command", "command": "C:\\tools\\cc-notify.exe notify --claude"}
        ]
      }
    ]
  }
}`
	out, changed, err := ClaudeRemoveHook(existing)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !changed {
		t.Fatal("expected changed")
	}
	if strings.Contains(out, "cc-notify") {
		t.Fatalf("expected our hook removed: %q", out)
	}
	if !strings.Contains(out, "some-other-tool") {
		t.Fatalf("expected other hooks preserved: %q", out)
	}
}

func TestClaudeRemoveHook_NoHook(t *testing.T) {
	existing := `{"permissions":{"allow":["Bash(git *)"]}}`
	_, changed, err := ClaudeRemoveHook(existing)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if changed {
		t.Fatal("expected no change")
	}
}
