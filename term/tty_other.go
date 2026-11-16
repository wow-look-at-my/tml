//go:build !(linux || darwin || dragonfly || freebsd || netbsd || openbsd || windows)

// tty_other.go holds errUnsupported.

package term

import (
	"errors"
	"io"
	"os"
)

// errUnsupported is what every terminal call answers on a system this package has no terminal code for.
var errUnsupported = errors.New("tml: no terminal support on this operating system")

func isTerminal(uintptr) bool { return false }

func size(uintptr) (width, height int, err error) { return 0, 0, errUnsupported }

func makeRaw(uintptr) (func() error, error) { return nil, errUnsupported }

// NotifyResize does nothing here.
func NotifyResize(chan<- os.Signal) (stop func()) { return func() {} }

// NewReader reads the stream as it is.
func NewReader(stream io.Reader) (Reader, error) { return plainReader{stream}, nil }

type plainReader struct{ io.Reader }

func (plainReader) Close() error { return nil }
