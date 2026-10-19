package tml

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"syscall"
	"time"

	tea "charm.land/bubbletea/v2"
)

// pidSocket is the name SocketPath gives a socket in the shared directory.
var pidSocket = regexp.MustCompile(`^[0-9]+\.sock$`)

// sweepStale removes the sockets in dir that no process listens on. A process that exits without Close leaves its
// socket behind, and a command-line tool that loads a view. On every run would fill the directory. Only a socket
// that refuses a connection goes. A socket that is slow to answer belongs to a live program and stays.
func sweepStale(dir, own string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		if path == own || !pidSocket.MatchString(entry.Name()) || entry.Type()&os.ModeSocket == 0 {
			continue
		}
		conn, err := net.DialTimeout("unix", path, 50*time.Millisecond)
		if err == nil {
			conn.Close()
			continue
		}
		if errors.Is(err, syscall.ECONNREFUSED) {
			_ = os.Remove(path)
		}
	}
}

// inspection is the process's own inspector. Every View that Load builds is adopted by it, so recording is not a mode
var inspection = &session{}

type session struct {
	mu      sync.Mutex
	insp    *Inspector
	serving bool
	driven  bool
	err     error
}

// isDriven reports whether something handed this process's program over.
func (s *session) isDriven() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.driven
}

// adopt takes over a freshly loaded View, and opens the socket the leading time there is a frame to answer about.
func (s *session) adopt(v *View) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.insp == nil {
		s.insp = newInspector(v)
	} else {
		s.insp.Attach(v)
	}
	if s.serving {
		return nil
	}
	s.serving = true
	socket := SocketPath()
	err := prepareSocketDir(socket)
	if err == nil {
		err = s.insp.ListenSocket(socket)
	}
	if err == nil {
		if os.Getenv(SocketEnv) == "" {
			sweepStale(filepath.Dir(socket), socket)
		}
		return nil
	}
	s.err = fmt.Errorf("tml: serve the inspection protocol on %s: %w", socket, err)
	return s.err
}

// drive gives the protocol the several capabilities that need a running program.
func (s *session) drive(p *tea.Program) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.driven = true
	if s.insp == nil {
		return
	}
	s.insp.OnKey(func(key string) error {
		p.Send(KeyMsg(key))
		return nil
	})
	s.insp.OnClick(func(x, y int) error {
		p.Send(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
		return nil
	})
	s.insp.OnRepaint(func() error {
		p.Send(RepaintMsg{})
		return nil
	})
}

func (s *session) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.insp != nil {
		_ = s.insp.Close()
	}
	s.serving = false
}

// InspectError reports why the inspection socket is not being served, or nil.
func InspectError() error {
	inspection.mu.Lock()
	defer inspection.mu.Unlock()
	return inspection.err
}

// RepaintMsg asks the program to draw again. The inspector sends it after an override.
type RepaintMsg struct{}

// Run runs a Bubble Tea program with the inspection protocol wired.
func NewProgram(m tea.Model, opts ...tea.ProgramOption) (*tea.Program, error) {
	p := tea.NewProgram(m, opts...)
	inspection.drive(p)
	return p, InspectError()
}

// Run is NewProgram plus running it, for a host with nothing to say to the program in between.
func Run(m tea.Model, opts ...tea.ProgramOption) (tea.Model, error) {
	p, err := NewProgram(m, opts...)
	if err != nil {
		return nil, err
	}
	defer inspection.close()
	return p.Run()
}
