//go:build !windows

package app

import "io"

func enableRawInput(_ io.Reader, _ io.Writer) (func(), bool) {
	return func() {}, false
}
