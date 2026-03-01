package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ClaudeDefaultPath returns the Claude Code settings path.
func ClaudeDefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home: %w", err)
	}
	return filepath.Join(home, ".claude", "settings.json"), nil
}

// claudeSettings is the top-level Claude Code settings.json structure.
type claudeSettings struct {
	raw map[string]json.RawMessage
}

// claudeHookEntry represents a single hook command entry.
type claudeHookEntry struct {
	Type    string `json:"type"`
	Command string `json:"command"`
}

// claudeHookMatcher represents a matcher group containing hooks.
type claudeHookMatcher struct {
	Matcher string            `json:"matcher"`
	Hooks   []claudeHookEntry `json:"hooks"`
}

const claudeHookMarker = "cc-notify"

func parseClaudeSettings(content string) (claudeSettings, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return claudeSettings{raw: make(map[string]json.RawMessage)}, nil
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(content), &raw); err != nil {
		return claudeSettings{}, fmt.Errorf("parse claude settings: %w", err)
	}
	if raw == nil {
		raw = make(map[string]json.RawMessage)
	}
	return claudeSettings{raw: raw}, nil
}

func (s claudeSettings) serialize() (string, error) {
	data, err := json.MarshalIndent(s.raw, "", "  ")
	if err != nil {
		return "", fmt.Errorf("serialize claude settings: %w", err)
	}
	return string(data) + "\n", nil
}

func (s claudeSettings) getHooks() (map[string]json.RawMessage, error) {
	hooksRaw, ok := s.raw["hooks"]
	if !ok {
		return make(map[string]json.RawMessage), nil
	}
	var hooks map[string]json.RawMessage
	if err := json.Unmarshal(hooksRaw, &hooks); err != nil {
		return nil, fmt.Errorf("parse hooks: %w", err)
	}
	return hooks, nil
}

func (s *claudeSettings) setHooks(hooks map[string]json.RawMessage) error {
	data, err := json.Marshal(hooks)
	if err != nil {
		return fmt.Errorf("marshal hooks: %w", err)
	}
	s.raw["hooks"] = json.RawMessage(data)
	return nil
}

func getMatcherList(hooks map[string]json.RawMessage, event string) ([]claudeHookMatcher, error) {
	raw, ok := hooks[event]
	if !ok {
		return nil, nil
	}
	var matchers []claudeHookMatcher
	if err := json.Unmarshal(raw, &matchers); err != nil {
		return nil, fmt.Errorf("parse hook event %s: %w", event, err)
	}
	return matchers, nil
}

func setMatcherList(hooks map[string]json.RawMessage, event string, matchers []claudeHookMatcher) error {
	data, err := json.Marshal(matchers)
	if err != nil {
		return fmt.Errorf("marshal hook event %s: %w", event, err)
	}
	hooks[event] = json.RawMessage(data)
	return nil
}

func containsOurHook(matchers []claudeHookMatcher) bool {
	for _, m := range matchers {
		for _, h := range m.Hooks {
			if strings.Contains(h.Command, claudeHookMarker) {
				return true
			}
		}
	}
	return false
}

func removeOurHook(matchers []claudeHookMatcher) ([]claudeHookMatcher, bool) {
	changed := false
	var result []claudeHookMatcher
	for _, m := range matchers {
		var filtered []claudeHookEntry
		for _, h := range m.Hooks {
			if strings.Contains(h.Command, claudeHookMarker) {
				changed = true
			} else {
				filtered = append(filtered, h)
			}
		}
		if len(filtered) > 0 {
			m.Hooks = filtered
			result = append(result, m)
		} else if len(m.Hooks) > 0 {
			// All hooks were ours, drop the entire matcher group
		} else {
			result = append(result, m)
		}
	}
	return result, changed
}

func buildNotifyCommand(exePath string) string {
	// Claude Code passes hook context via stdin as JSON.
	// We use the exe path to call notify with --stdin for Claude mode.
	return fmt.Sprintf("%s notify --claude", exePath)
}

// ClaudeUpsertHook inserts or updates the cc-notify hook in Claude Code settings.
// It installs a "Stop" hook so we get notified when Claude finishes.
func ClaudeUpsertHook(content string, exePath string) (string, bool, error) {
	settings, err := parseClaudeSettings(content)
	if err != nil {
		return "", false, err
	}

	hooks, err := settings.getHooks()
	if err != nil {
		return "", false, err
	}

	cmd := buildNotifyCommand(exePath)
	anyChanged := false

	// Install on "Stop" (task complete) and "Notification" (permission prompts).
	for _, event := range []string{"Stop", "Notification"} {
		matchers, err := getMatcherList(hooks, event)
		if err != nil {
			return "", false, err
		}

		// Remove existing to avoid duplicates
		matchers, removed := removeOurHook(matchers)
		if removed {
			anyChanged = true
		}

		newMatcher := claudeHookMatcher{
			Matcher: "",
			Hooks: []claudeHookEntry{
				{Type: "command", Command: cmd},
			},
		}
		matchers = append(matchers, newMatcher)
		anyChanged = true

		if err := setMatcherList(hooks, event, matchers); err != nil {
			return "", false, err
		}
	}

	if err := settings.setHooks(hooks); err != nil {
		return "", false, err
	}

	result, err := settings.serialize()
	if err != nil {
		return "", false, err
	}
	return result, anyChanged, nil
}

// ClaudeRemoveHook removes the cc-notify hook from Claude Code settings.
func ClaudeRemoveHook(content string) (string, bool, error) {
	settings, err := parseClaudeSettings(content)
	if err != nil {
		return "", false, err
	}

	hooks, err := settings.getHooks()
	if err != nil {
		return "", false, err
	}

	anyChanged := false
	for _, event := range []string{"Stop", "Notification"} {
		matchers, err := getMatcherList(hooks, event)
		if err != nil {
			return "", false, err
		}

		matchers, removed := removeOurHook(matchers)
		if removed {
			anyChanged = true
			if len(matchers) == 0 {
				delete(hooks, event)
			} else {
				if err := setMatcherList(hooks, event, matchers); err != nil {
					return "", false, err
				}
			}
		}
	}

	if !anyChanged {
		return content, false, nil
	}

	if len(hooks) == 0 {
		delete(settings.raw, "hooks")
	} else {
		if err := settings.setHooks(hooks); err != nil {
			return "", false, err
		}
	}

	result, err := settings.serialize()
	if err != nil {
		return "", false, err
	}
	return result, true, nil
}
