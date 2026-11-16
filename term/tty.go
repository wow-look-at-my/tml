// Package term is the terminal itself. Raw mode and size. The decoder that turns the bytes a terminal sends into
// input values, and the screen writer that turns frames into the bytes a terminal draws.
package term

import (
	"io"
	"os"
)

// File is a stream with a file descriptor, which is what a terminal is.
type File interface {
	Fd() uintptr
}

// IsTerminal reports whether the stream is a terminal.
func IsTerminal(stream any) bool {
	file, ok := stream.(File)
	return ok && isTerminal(file.Fd())
}

// Size is the terminal's size in cells. It fails for a stream that is not a terminal.
func Size(stream any) (width, height int, err error) {
	file, ok := stream.(File)
	if !ok {
		return 0, 0, os.ErrInvalid
	}
	return size(file.Fd())
}

// Raw puts the terminal behind the stream into raw mode: no echo, no line buffer, and no signal keys. The function
// it returns puts the terminal back.
func Raw(stream any) (restore func() error, err error) {
	file, ok := stream.(File)
	if !ok {
		return nil, os.ErrInvalid
	}
	return makeRaw(file.Fd())
}

// Reader reads a terminal's input in a way Close can interrupt, so a program that ends gives the stream back with
// nothing swallowed.
type Reader interface {
	io.Reader
	// Close makes a blocked Read return io.EOF.
	Close() error
}
