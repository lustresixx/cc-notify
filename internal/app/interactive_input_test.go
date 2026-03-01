package app

import (
	"bytes"
	"strings"
	"testing"
)

func TestPromptLine_AcceptsCarriageReturnTerminator(t *testing.T) {
	tool := New(Options{
		Stdin:  strings.NewReader("cc-notify.desktop\r"),
		Stdout: &bytes.Buffer{},
	})

	got, err := tool.promptLine("Toast AppId: ")
	if err != nil {
		t.Fatalf("promptLine returned error for carriage return input: %v", err)
	}
	if got != "cc-notify.desktop" {
		t.Fatalf("expected parsed app id, got %q", got)
	}
}

func TestPromptLine_AcceptsLineFeedTerminator(t *testing.T) {
	tool := New(Options{
		Stdin:  strings.NewReader("cc-notify.desktop\n"),
		Stdout: &bytes.Buffer{},
	})

	got, err := tool.promptLine("Toast AppId: ")
	if err != nil {
		t.Fatalf("promptLine returned error for line feed input: %v", err)
	}
	if got != "cc-notify.desktop" {
		t.Fatalf("expected parsed app id, got %q", got)
	}
}
