//go:build !windows

// tty_notwindows.go holds EnableOutput.

package term

// EnableOutput does nothing here: only the Windows console has to be asked to read VT sequences.
func EnableOutput(any) (restore func() error) { return func() error { return nil } }
