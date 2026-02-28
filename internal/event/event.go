package event

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

// Payload is the Codex notify JSON payload.
type Payload struct {
	Type                 string `json:"type"`
	Summary              string `json:"summary"`
	LastAssistantMessage string `json:"last-assistant-message"`
	CWD                  string `json:"cwd"`
	Model                string `json:"model"`
	TranscriptPath       string `json:"transcript-path"`
}

// ContentMode controls which payload content is used for notification body.
type ContentMode string

const (
	ContentModeSummary  ContentMode = "summary"
	ContentModeComplete ContentMode = "complete"
	ContentModeFull     ContentMode = "full"
)

// RenderOptions controls notification rendering behavior.
type RenderOptions struct {
	ContentMode  ContentMode
	IncludeDir   bool
	IncludeModel bool
	IncludeEvent bool
}

// ParsePayload parses a Codex notify payload from JSON.
func ParsePayload(raw string) (Payload, error) {
	raw = strings.TrimSpace(stripBOM(raw))
	var payload Payload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return Payload{}, fmt.Errorf("parse notify payload: %w", err)
	}
	if strings.TrimSpace(payload.Type) == "" {
		return Payload{}, fmt.Errorf("parse notify payload: missing type")
	}
	return payload, nil
}

func stripBOM(value string) string {
	return strings.TrimPrefix(value, "\uFEFF")
}

// RenderNotification converts payload into notification title/body.
// ok is false when event type is unsupported and should be ignored.
func RenderNotification(payload Payload) (title string, body string, ok bool) {
	return RenderNotificationWithOptions(payload, RenderOptions{
		ContentMode:  ContentModeSummary,
		IncludeDir:   true,
		IncludeModel: false,
		IncludeEvent: false,
	})
}

// RenderNotificationWithOptions converts payload into notification title/body using user-selected options.
// ok is false when event type is unsupported and should be ignored.
func RenderNotificationWithOptions(payload Payload, opts RenderOptions) (title string, body string, ok bool) {
	switch payload.Type {
	case "agent-turn-complete":
		title = "Codex Task Complete"
	case "agent-turn-paused":
		title = "Codex Needs Input"
	default:
		return "", "", false
	}

	mode := normalizeContentMode(opts.ContentMode)
	switch mode {
	case ContentModeComplete:
		body = "complete"
		if payload.Type == "agent-turn-paused" {
			body = "waiting for approval"
		}
	case ContentModeFull:
		body = firstNonEmpty(payload.LastAssistantMessage, payload.Summary, defaultBodyForType(payload.Type))
	default:
		body = firstNonEmpty(payload.Summary, payload.LastAssistantMessage, defaultBodyForType(payload.Type))
	}
	body = cleanText(body)

	if opts.IncludeDir {
		dirName := strings.TrimSpace(filepath.Base(strings.TrimSpace(payload.CWD)))
		if dirName != "" && dirName != "." && dirName != string(filepath.Separator) {
			body += "\nDir: " + dirName
		}
	}
	if opts.IncludeModel {
		model := strings.TrimSpace(payload.Model)
		if model != "" {
			body += "\nModel: " + model
		}
	}
	if opts.IncludeEvent {
		body += "\nEvent: " + payload.Type
	}
	return title, body, true
}

func normalizeContentMode(mode ContentMode) ContentMode {
	switch mode {
	case ContentModeComplete, ContentModeFull:
		return mode
	default:
		return ContentModeSummary
	}
}

func defaultBodyForType(eventType string) string {
	switch eventType {
	case "agent-turn-paused":
		return "Waiting for your approval"
	default:
		return "Task completed"
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func cleanText(value string) string {
	lines := strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n")
	for i, line := range lines {
		lines[i] = strings.Join(strings.Fields(line), " ")
	}
	result := strings.TrimSpace(strings.Join(lines, "\n"))
	runes := []rune(result)
	if len(runes) > 300 {
		result = string(runes[:297]) + "..."
	}
	if result == "" {
		return "Task completed"
	}
	return result
}
