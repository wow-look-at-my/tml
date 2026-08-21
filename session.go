package tml

import (
	"fmt"
	"sync"

	tea "charm.land/bubbletea/v2"
)

// inspection is the process's one inspector.
//
// Every View that Load builds is adopted by it, so recording is not a mode a
// program opts into. A View records into a fixed-size record and the cost on a
// program nobody is inspecting is one atomic load per render, which is not a
// price worth making anybody decide about.
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

// adopt takes over a freshly loaded View, and opens the socket the first time
// there is a frame to answer about.
//
// The socket is not conditional on anything. A program that loads a document
// is a program the inspector can reach, from that moment until it exits, with
// nothing set and nothing passed -- because a switch is a switch whichever way
// it points, and a library whose inspector has to be asked for is a library
// whose programs are inspectable in principle and not in fact.
//
// A host that loads again -- a theme flip, a width method the terminal
// answered late -- gets a new View and this follows it. Nothing about that is
// the host's to arrange.
// It returns what went wrong THIS time. The session also keeps the failure, so
// a host holding only a program can still read it, but Load must not fail on a
// failure some earlier Load already reported: one unservable path would then be
// the last word for the life of the process.
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
		return nil
	}
	s.err = fmt.Errorf("tml: serve the inspection protocol on %s: %w", socket, err)
	return s.err
}

// drive gives the protocol the three capabilities that need a running program.
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
//
// A program that asked for a socket by setting the environment variable and
// silently did not get one would be a program every test times out against
// with nothing to read. Run returns this; a host that runs its own program
// checks it.
func InspectError() error {
	inspection.mu.Lock()
	defer inspection.mu.Unlock()
	return inspection.err
}

// RepaintMsg asks the program to draw again. The inspector sends it after an
// override, and it is the one message a host has to act on.
//
// A host that caches its frame must invalidate that cache here. Bubble Tea
// redraws after every update, but a model that answers View with the string it
// answered last time paints the old geometry -- so the override lands in the
// layout and never on the screen. The protocol catches that rather than
// letting it pass: a restyle waits for a frame that is genuinely new and fails
// by name when none arrives.
type RepaintMsg struct{}

// Run runs a Bubble Tea program with the inspection protocol wired.
//
// It exists so that wiring is not a thing to remember, and it is the ONE thing
// a host does that this library cannot do for it. Reading is automatic: Load
// adopts the View and opens the socket. Driving needs the *tea.Program, which
// does not exist when this package initialises, and Go offers no way to
// intercept another package's constructor -- so the handle has to come from
// whoever builds it. tea.NewProgram still works and produces a program this
// library cannot reach: it answers every drive op by naming this function.
func NewProgram(m tea.Model, opts ...tea.ProgramOption) (*tea.Program, error) {
	p := tea.NewProgram(m, opts...)
	inspection.drive(p)
	return p, InspectError()
}

// Run is NewProgram plus running it, for a host with nothing to say to the
// program in between. A host that needs the handle -- to kill the program from
// a worker, or to send to it -- calls NewProgram and runs it itself.
func Run(m tea.Model, opts ...tea.ProgramOption) (tea.Model, error) {
	p, err := NewProgram(m, opts...)
	if err != nil {
		return nil, err
	}
	defer inspection.close()
	return p.Run()
}
