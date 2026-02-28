package main

import (
	"os"

	"cc-notify/internal/app"
)

func main() {
	args := os.Args[1:]
	tool := app.New(app.Options{})
	code := tool.Run(args)
	if len(args) > 0 {
		maybePause(args, os.Stdin, os.Stdout, os.Getenv, stdinIsCharDevice, runtimeGOOS())
	}
	os.Exit(code)
}
