package event

import (
	"strings"
	"testing"
)

func TestParsePayload_Success(t *testing.T) {
	raw := `{"type":"agent-turn-complete","summary":"done","cwd":"C:\\work","model":"gpt-5"}`
	got, err := ParsePayload(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Type != "agent-turn-complete" {
		t.Fatalf("unexpected type: %q", got.Type)
	}
	if got.Summary != "done" {
		t.Fatalf("unexpected summary: %q", got.Summary)
	}
}

func TestParsePayload_InvalidJSON(t *testing.T) {
	_, err := ParsePayload("{")
	if err == nil {
		t.Fatalf("expected parse error")
	}
}

func TestParsePayload_SupportsUTF8BOM(t *testing.T) {
	raw := "\uFEFF" + `{"type":"agent-turn-complete","summary":"done"}`
	got, err := ParsePayload(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Type != "agent-turn-complete" {
		t.Fatalf("unexpected type: %q", got.Type)
	}
}

func TestRenderNotification_UsesSummaryFirst(t *testing.T) {
	payload := Payload{
		Type:                 "agent-turn-complete",
		Summary:              "refactor completed",
		LastAssistantMessage: "fallback",
		CWD:                  `C:\code\project`,
	}
	title, body, ok := RenderNotification(payload)
	if !ok {
		t.Fatalf("expected supported event type")
	}
	if title != "Codex Task Complete" {
		t.Fatalf("unexpected title: %q", title)
	}
	if body != "refactor completed\nDir: project" {
		t.Fatalf("unexpected body: %q", body)
	}
}

func TestRenderNotificationWithOptions_CompleteOnly(t *testing.T) {
	payload := Payload{
		Type:                 "agent-turn-complete",
		Summary:              "summary text",
		LastAssistantMessage: "full text",
		CWD:                  `C:\code\project`,
	}
	_, body, ok := RenderNotificationWithOptions(payload, RenderOptions{
		ContentMode: ContentModeComplete,
		IncludeDir:  false,
	})
	if !ok {
		t.Fatalf("expected supported event type")
	}
	if body != "complete" {
		t.Fatalf("unexpected body: %q", body)
	}
}

func TestRenderNotificationWithOptions_FullMessage(t *testing.T) {
	payload := Payload{
		Type:                 "agent-turn-complete",
		Summary:              "summary text",
		LastAssistantMessage: "full answer body",
	}
	_, body, ok := RenderNotificationWithOptions(payload, RenderOptions{
		ContentMode: ContentModeFull,
		IncludeDir:  false,
	})
	if !ok {
		t.Fatalf("expected supported event type")
	}
	if body != "full answer body" {
		t.Fatalf("unexpected body: %q", body)
	}
}

func TestRenderNotificationWithOptions_ExtraFields(t *testing.T) {
	payload := Payload{
		Type:    "agent-turn-complete",
		Summary: "done",
		Model:   "gpt-5",
		CWD:     `C:\code\project`,
	}
	_, body, ok := RenderNotificationWithOptions(payload, RenderOptions{
		ContentMode:  ContentModeSummary,
		IncludeDir:   true,
		IncludeModel: true,
		IncludeEvent: true,
	})
	if !ok {
		t.Fatalf("expected supported event type")
	}
	if !strings.Contains(body, "Dir: project") {
		t.Fatalf("expected dir in body: %q", body)
	}
	if !strings.Contains(body, "Model: gpt-5") {
		t.Fatalf("expected model in body: %q", body)
	}
	if !strings.Contains(body, "Event: agent-turn-complete") {
		t.Fatalf("expected event type in body: %q", body)
	}
}

func TestRenderNotification_UnknownTypeIgnored(t *testing.T) {
	payload := Payload{Type: "something-else"}
	_, _, ok := RenderNotification(payload)
	if ok {
		t.Fatalf("expected unknown event to be ignored")
	}
}

func TestRenderNotification_PausedEvent(t *testing.T) {
	payload := Payload{
		Type:    "agent-turn-paused",
		Summary: "needs permission to run `rm -rf /tmp/build`",
		CWD:     `C:\code\project`,
	}
	title, body, ok := RenderNotification(payload)
	if !ok {
		t.Fatalf("expected paused event to be supported")
	}
	if title != "Codex Needs Input" {
		t.Fatalf("unexpected title: %q", title)
	}
	if !strings.Contains(body, "needs permission") {
		t.Fatalf("expected summary in body: %q", body)
	}
}

func TestRenderNotification_PausedEventNoSummary(t *testing.T) {
	payload := Payload{
		Type: "agent-turn-paused",
	}
	title, body, ok := RenderNotification(payload)
	if !ok {
		t.Fatalf("expected paused event to be supported")
	}
	if title != "Codex Needs Input" {
		t.Fatalf("unexpected title: %q", title)
	}
	if body != "Waiting for your approval" {
		t.Fatalf("unexpected default body: %q", body)
	}
}

func TestRenderNotificationWithOptions_PausedCompleteMode(t *testing.T) {
	payload := Payload{
		Type:    "agent-turn-paused",
		Summary: "wants to run something",
	}
	_, body, ok := RenderNotificationWithOptions(payload, RenderOptions{
		ContentMode: ContentModeComplete,
		IncludeDir:  false,
	})
	if !ok {
		t.Fatalf("expected paused event to be supported")
	}
	if body != "waiting for approval" {
		t.Fatalf("unexpected body: %q", body)
	}
}

func TestRenderNotification_TruncatesByRunesNotBytes(t *testing.T) {
	payload := Payload{
		Type:    "agent-turn-complete",
		Summary: strings.Repeat("你", 310),
	}
	_, body, ok := RenderNotification(payload)
	if !ok {
		t.Fatalf("expected supported event")
	}

	firstLine := strings.Split(body, "\n")[0]
	if !strings.HasSuffix(firstLine, "...") {
		t.Fatalf("expected ellipsis suffix, got %q", firstLine)
	}
	if strings.Contains(firstLine, "\uFFFD") {
		t.Fatalf("expected no utf8 replacement char: %q", firstLine)
	}
	if runeCount := len([]rune(firstLine)); runeCount != 300 {
		t.Fatalf("expected 300 runes after truncation, got %d", runeCount)
	}
}
