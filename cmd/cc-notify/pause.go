package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
)

func maybePause(
	args []string,
	in io.Reader,
	out io.Writer,
	getenv func(string) string,
	isTTY func() bool,
	goos string,
) {
	if !shouldPause(goos, args, getenv("CC_NOTIFY_NO_PAUSE"), isTTY()) {
		return
	}

	fmt.Fprintln(out)
	fmt.Fprintln(out, "No command provided. Press Enter to exit...")
	_, _ = bufio.NewReader(in).ReadString('\n')
}

func shouldPause(goos string, args []string, noPause string, isTTY bool) bool {
	if goos != "windows" {
		return false
	}
	if len(args) != 0 {
		return false
	}
	if strings.TrimSpace(noPause) == "1" {
		return false
	}
	return isTTY
}

func runtimeGOOS() string {
	return runtime.GOOS
}

func stdinIsCharDevice() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
