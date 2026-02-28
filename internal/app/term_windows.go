//go:build windows

package app

import (
	"io"
	"os"
	"syscall"
	"unsafe"
)

const (
	enableEchoInput                 = 0x0004
	enableLineInput                 = 0x0002
	enableVirtualTerminalInput      = 0x0200
	enableVirtualTerminalProcessing = 0x0004
)

var (
	kernel32           = syscall.NewLazyDLL("kernel32.dll")
	procGetConsoleMode = kernel32.NewProc("GetConsoleMode")
	procSetConsoleMode = kernel32.NewProc("SetConsoleMode")
)

func enableRawInput(in io.Reader, out io.Writer) (func(), bool) {
	inFile, ok := in.(*os.File)
	if !ok {
		return func() {}, false
	}
	outFile, ok := out.(*os.File)
	if !ok {
		return func() {}, false
	}

	inMode, err := getConsoleMode(syscall.Handle(inFile.Fd()))
	if err != nil {
		return func() {}, false
	}
	outMode, outErr := getConsoleMode(syscall.Handle(outFile.Fd()))
	if outErr != nil {
		outMode = 0
	}

	rawInMode := inMode &^ (enableEchoInput | enableLineInput)
	rawInMode |= enableVirtualTerminalInput
	if err := setConsoleMode(syscall.Handle(inFile.Fd()), rawInMode); err != nil {
		return func() {}, false
	}

	if outErr == nil {
		_ = setConsoleMode(syscall.Handle(outFile.Fd()), outMode|enableVirtualTerminalProcessing)
	}

	restore := func() {
		_ = setConsoleMode(syscall.Handle(inFile.Fd()), inMode)
		if outErr == nil {
			_ = setConsoleMode(syscall.Handle(outFile.Fd()), outMode)
		}
	}
	return restore, true
}

func getConsoleMode(handle syscall.Handle) (uint32, error) {
	var mode uint32
	r1, _, e1 := procGetConsoleMode.Call(uintptr(handle), uintptr(unsafe.Pointer(&mode)))
	if r1 == 0 {
		return 0, e1
	}
	return mode, nil
}

func setConsoleMode(handle syscall.Handle, mode uint32) error {
	r1, _, e1 := procSetConsoleMode.Call(uintptr(handle), uintptr(mode))
	if r1 == 0 {
		return e1
	}
	return nil
}
