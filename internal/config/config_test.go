package config

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestUpsertNotify_EmptyConfig(t *testing.T) {
	out, changed, err := UpsertNotify("", []string{`C:\tools\cc-notify.exe`, "notify"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !changed {
		t.Fatalf("expected changed to be true")
	}
	want := "notify = [\"C:\\\\tools\\\\cc-notify.exe\", \"notify\"]\n"
	if out != want {
		t.Fatalf("unexpected output:\nwant: %q\ngot:  %q", want, out)
	}
}

func TestUpsertNotify_InsertsBeforeFirstTable(t *testing.T) {
	in := "[model_providers.openai]\nname = \"OpenAI\"\n"
	out, changed, err := UpsertNotify(in, []string{`C:\bin\cc-notify.exe`, "notify"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !changed {
		t.Fatalf("expected changed to be true")
	}

	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 lines, got %d", len(lines))
	}
	if lines[0] != "notify = [\"C:\\\\bin\\\\cc-notify.exe\", \"notify\"]" {
		t.Fatalf("unexpected notify line: %q", lines[0])
	}
	if lines[2] != "[model_providers.openai]" {
		t.Fatalf("expected table after blank line, got %q", lines[2])
	}
}

func TestUpsertNotify_ReplacesExistingTopLevelNotify(t *testing.T) {
	in := "notify = [\"old.exe\", \"notify\"]\n[sandbox_workspace_write]\nnetwork_access = true\n"
	out, changed, err := UpsertNotify(in, []string{`C:\new\cc-notify.exe`, "notify"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !changed {
		t.Fatalf("expected changed to be true")
	}
	if strings.Contains(out, "old.exe") {
		t.Fatalf("expected old command to be replaced: %q", out)
	}
	if !strings.Contains(out, `notify = ["C:\\new\\cc-notify.exe", "notify"]`) {
		t.Fatalf("expected new notify line: %q", out)
	}
}

func TestRemoveNotify_RemovesMultilineAssignment(t *testing.T) {
	in := "notify = [\n  \"C:\\\\tool.exe\",\n  \"notify\",\n]\n[projects.\"C:\\\\code\"]\ntrust_level = \"trusted\"\n"
	out, changed, err := RemoveNotify(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !changed {
		t.Fatalf("expected changed to be true")
	}
	if strings.Contains(out, "notify =") {
		t.Fatalf("expected notify assignment to be removed: %q", out)
	}
	if !strings.Contains(out, "[projects.\"C:\\\\code\"]") {
		t.Fatalf("expected table content to remain")
	}
}

func TestDefaultPath(t *testing.T) {
	got, err := DefaultPath()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if filepath.Base(got) != "config.toml" {
		t.Fatalf("expected config.toml, got %q", got)
	}
	if !strings.Contains(strings.ToLower(got), strings.ToLower(filepath.Join(".codex"))) {
		t.Fatalf("expected .codex directory in path, got %q", got)
	}
}
