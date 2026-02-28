package notifier

import (
	"encoding/base64"
	"strings"
	"testing"
	"unicode/utf16"
)

func TestBuildToastScript_EmbedsBase64Payload(t *testing.T) {
	title := `$env:PATH $(Get-Process)`
	body := "line1 & line2"
	appID := "Windows PowerShell"
	script := buildToastScript(title, body, appID)

	titleB64 := base64.StdEncoding.EncodeToString([]byte(title))
	bodyB64 := base64.StdEncoding.EncodeToString([]byte(body))
	appIDB64 := base64.StdEncoding.EncodeToString([]byte(appID))

	if !strings.Contains(script, "FromBase64String('"+titleB64+"')") {
		t.Fatalf("title base64 payload missing in script: %q", script)
	}
	if !strings.Contains(script, "FromBase64String('"+bodyB64+"')") {
		t.Fatalf("body base64 payload missing in script: %q", script)
	}
	if !strings.Contains(script, "FromBase64String('"+appIDB64+"')") {
		t.Fatalf("appID base64 payload missing in script: %q", script)
	}
	if strings.Contains(script, title) {
		t.Fatalf("raw title should not be embedded directly: %q", script)
	}
}

func TestEncodePowerShellCommand_UTF16LEBase64(t *testing.T) {
	encoded := encodePowerShellCommand("abc")
	got, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("decode base64: %v", err)
	}
	if len(got)%2 != 0 {
		t.Fatalf("expected utf16-le bytes length to be even")
	}

	u16 := make([]uint16, 0, len(got)/2)
	for i := 0; i < len(got); i += 2 {
		u16 = append(u16, uint16(got[i])|uint16(got[i+1])<<8)
	}
	if decoded := string(utf16.Decode(u16)); decoded != "abc" {
		t.Fatalf("unexpected decoded command: %q", decoded)
	}
}

func TestBuildPopupScript_EmbedsBase64Payload(t *testing.T) {
	title := `$env:PATH`
	body := `$(Get-Process)`
	script := buildPopupScript(title, body)

	titleB64 := base64.StdEncoding.EncodeToString([]byte(title))
	bodyB64 := base64.StdEncoding.EncodeToString([]byte(body))

	if !strings.Contains(script, "FromBase64String('"+titleB64+"')") {
		t.Fatalf("title base64 payload missing in popup script: %q", script)
	}
	if !strings.Contains(script, "FromBase64String('"+bodyB64+"')") {
		t.Fatalf("body base64 payload missing in popup script: %q", script)
	}
	if strings.Contains(script, title) || strings.Contains(script, body) {
		t.Fatalf("raw payload should not be embedded directly: %q", script)
	}
}
