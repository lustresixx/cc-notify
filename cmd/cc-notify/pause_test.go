package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestShouldPause(t *testing.T) {
	cases := []struct {
		name    string
		goos    string
		args    []string
		noPause string
		isTTY   bool
		want    bool
	}{
		{
			name:  "windows no args tty",
			goos:  "windows",
			args:  nil,
			isTTY: true,
			want:  true,
		},
		{
			name:  "windows with args",
			goos:  "windows",
			args:  []string{"install"},
			isTTY: true,
			want:  false,
		},
		{
			name:    "disabled by env",
			goos:    "windows",
			args:    nil,
			noPause: "1",
			isTTY:   true,
			want:    false,
		},
		{
			name:  "non windows",
			goos:  "linux",
			args:  nil,
			isTTY: true,
			want:  false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldPause(tc.goos, tc.args, tc.noPause, tc.isTTY); got != tc.want {
				t.Fatalf("shouldPause() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestMaybePause_WritesPrompt(t *testing.T) {
	var out bytes.Buffer
	maybePause(
		[]string{},
		strings.NewReader("\n"),
		&out,
		func(string) string { return "" },
		func() bool { return true },
		"windows",
	)
	if !strings.Contains(out.String(), "Press Enter to exit") {
		t.Fatalf("expected pause prompt, got %q", out.String())
	}
}
