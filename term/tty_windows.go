//go:build windows

// tty_windows.go holds makeRaw.

package term

import (
	"io"
	"os"

	"golang.org/x/sys/windows"
)

func isTerminal(fd uintptr) bool {
	var mode uint32
	return windows.GetConsoleMode(windows.Handle(fd), &mode) == nil
}

func size(fd uintptr) (width, height int, err error) {
	var info windows.ConsoleScreenBufferInfo
	if err := windows.GetConsoleScreenBufferInfo(windows.Handle(fd), &info); err != nil {
		return 0, 0, err
	}
	return int(info.Window.Right - info.Window.Left + 1), int(info.Window.Bottom - info.Window.Top + 1), nil
}

// makeRaw turns off the console's line input and echo, and asks it to send keys as VT sequences.
func makeRaw(fd uintptr) (func() error, error) {
	handle := windows.Handle(fd)
	var old uint32
	if err := windows.GetConsoleMode(handle, &old); err != nil {
		return nil, err
	}
	raw := old &^ (windows.ENABLE_ECHO_INPUT | windows.ENABLE_PROCESSED_INPUT | windows.ENABLE_LINE_INPUT)
	raw |= windows.ENABLE_VIRTUAL_TERMINAL_INPUT
	if err := windows.SetConsoleMode(handle, raw); err != nil {
		return nil, err
	}
	return func() error { return windows.SetConsoleMode(handle, old) }, nil
}

// EnableOutput asks the console to read VT sequences on its output. The function it returns undoes that.
func EnableOutput(stream any) (restore func() error) {
	file, ok := stream.(File)
	if !ok {
		return func() error { return nil }
	}
	handle := windows.Handle(file.Fd())
	var old uint32
	if err := windows.GetConsoleMode(handle, &old); err != nil {
		return func() error { return nil }
	}
	_ = windows.SetConsoleMode(handle, old|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING)
	return func() error { return windows.SetConsoleMode(handle, old) }
}

// NotifyResize does nothing on Windows: the console sends no signal for a new size. A program polls Size.
func NotifyResize(chan<- os.Signal) (stop func()) { return func() {} }

// NewReader reads the stream as it is. Close cannot interrupt a console read.
func NewReader(stream io.Reader) (Reader, error) { return plainReader{stream}, nil }

type plainReader struct{ io.Reader }

func (plainReader) Close() error { return nil }
