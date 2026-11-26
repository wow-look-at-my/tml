//go:build linux || darwin || dragonfly || freebsd || netbsd || openbsd

// tty_unix.go holds NotifyResize.

package term

import (
	"errors"
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"golang.org/x/sys/unix"
)

func isTerminal(fd uintptr) bool {
	_, err := unix.IoctlGetTermios(int(fd), ioctlReadTermios)
	return err == nil
}

func size(fd uintptr) (width, height int, err error) {
	ws, err := unix.IoctlGetWinsize(int(fd), unix.TIOCGWINSZ)
	if err != nil {
		return 0, 0, err
	}
	return int(ws.Col), int(ws.Row), nil
}

func makeRaw(fd uintptr) (func() error, error) {
	old, err := unix.IoctlGetTermios(int(fd), ioctlReadTermios)
	if err != nil {
		return nil, err
	}
	raw := *old
	raw.Iflag &^= unix.IGNBRK | unix.BRKINT | unix.PARMRK | unix.ISTRIP | unix.INLCR | unix.IGNCR | unix.ICRNL | unix.IXON
	raw.Lflag &^= unix.ECHO | unix.ECHONL | unix.ICANON | unix.ISIG | unix.IEXTEN
	raw.Cflag &^= unix.CSIZE | unix.PARENB
	raw.Cflag |= unix.CS8
	raw.Cc[unix.VMIN] = 1
	raw.Cc[unix.VTIME] = 0
	if err := unix.IoctlSetTermios(int(fd), ioctlWriteTermios, &raw); err != nil {
		return nil, err
	}
	return func() error { return unix.IoctlSetTermios(int(fd), ioctlWriteTermios, old) }, nil
}

// NotifyResize sends on the channel each time the terminal's size changes. The function it returns stops that.
func NotifyResize(ch chan<- os.Signal) (stop func()) {
	signal.Notify(ch, syscall.SIGWINCH)
	return func() { signal.Stop(ch) }
}

// pollReader reads a descriptor through poll, with a pipe that wakes the poll when the reader closes.
type pollReader struct {
	fd         int
	wake, woke *os.File
	once       sync.Once
	// closed is closed by Close, which ends a read that poll cannot wake.
	closed chan struct{}
	// blocking is set when poll does not work here, and the reader falls back to a plain read.
	blocking bool
}

// NewReader wraps a terminal's input. A stream with no descriptor is read as it is, and Close cannot interrupt it.
func NewReader(stream io.Reader) (Reader, error) {
	file, ok := stream.(File)
	if !ok {
		return plainReader{stream}, nil
	}
	woke, wake, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	return &pollReader{fd: int(file.Fd()), wake: wake, woke: woke, closed: make(chan struct{})}, nil
}

func (r *pollReader) Read(p []byte) (int, error) {
	if r.blocking {
		return r.readBlocking(p)
	}
	fds := []unix.PollFd{
		{Fd: int32(r.fd), Events: unix.POLLIN},
		{Fd: int32(r.woke.Fd()), Events: unix.POLLIN},
	}
	for {
		_, err := unix.Poll(fds, -1)
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			// A host with no poll for this descriptor still has a keyboard. The read then blocks, and Close cannot interrupt it.
			r.blocking = true
			return r.readBlocking(p)
		}
		if fds[1].Revents != 0 {
			return 0, io.EOF
		}
		if fds[0].Revents&(unix.POLLIN|unix.POLLHUP|unix.POLLERR) != 0 {
			n, err := unix.Read(r.fd, p)
			if errors.Is(err, unix.EINTR) || errors.Is(err, unix.EAGAIN) {
				continue
			}
			if n == 0 && err == nil {
				return 0, io.EOF
			}
			return max(n, 0), err
		}
	}
}

// readBlocking reads when poll cannot watch the descriptor. The read runs in a goroutine whose buffer the goroutine
// owns. Close can end the read without a later write landing in a caller that has moved on.
func (r *pollReader) readBlocking(p []byte) (int, error) {
	select {
	case <-r.closed:
		return 0, io.EOF
	default:
	}
	buf := make([]byte, len(p))
	type result struct {
		n   int
		err error
	}
	done := make(chan result, 1)
	go func() {
		for {
			n, err := unix.Read(r.fd, buf)
			if errors.Is(err, unix.EINTR) {
				continue
			}
			done <- result{n, err}
			return
		}
	}()
	select {
	case res := <-done:
		if res.n == 0 && res.err == nil {
			return 0, io.EOF
		}
		n := max(res.n, 0)
		copy(p, buf[:n])
		return n, res.err
	case <-r.closed:
		return 0, io.EOF
	}
}

func (r *pollReader) Close() error {
	r.once.Do(func() {
		close(r.closed)
		_, _ = r.wake.Write([]byte{0})
		_ = r.wake.Close()
	})
	return nil
}

type plainReader struct{ io.Reader }

func (plainReader) Close() error { return nil }
