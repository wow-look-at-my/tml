package tml

import (
	"fmt"
	"os"
	"sync"

	tea "charm.land/bubbletea/v2"
)

// SocketEnv names the unix socket a program serves the inspection protocol on.
//
// It is read by Load rather than by a host, because a host that has to
// remember to serve is a host that forgets: every program built on this
// library would be inspectable in principle and not in fact, which is how a
// test suite ends up reading pane captures instead.
const SocketEnv = "TML_INSPECT_SOCKET"

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
	err     error
}

// adopt takes over a freshly loaded View, and opens the socket the first time
// there is a frame to answer about.
//
// A host that loads again -- a theme flip, a width method the terminal
// answered late -- gets a new View and this follows it. Nothing about that is
// the host's to arrange.
func (s *session) adopt(v *View) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.insp == nil {
		s.insp = newInspector(v)
	} else {
		s.insp.Attach(v)
	}
	if s.serving {
		return
	}
	socket := os.Getenv(SocketEnv)
	if socket == "" {
		return
	}
	s.serving = true
	if err := s.insp.ListenSocket(socket); err != nil {
		// Recorded rather than returned: Load's error is about the document,
		// and a caller cannot tell one from the other by reading a string.
		// Run reports this, and a program that never calls Run has one waiting
		// in InspectError.
		s.err = fmt.Errorf("tml: serve the inspection protocol on %s (%s): %w", socket, SocketEnv, err)
	}
}

// drive gives the protocol the three capabilities that need a running program.
func (s *session) drive(p *tea.Program) {
	s.mu.Lock()
	defer s.mu.Unlock()
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

// Inspect is the process's inspector: the one every Load adopts its View into.
//
// It is a reader and a driver, not a thing to construct. A host that could
// build its own would be a host that can build a second, and two inspectors on
// one View overwrite each other's engine override and frame hook -- so the
// answer to "which one is serving" would depend on construction order.
//
// It is nil until something has been loaded, because until then there is no
// frame to be asked about.
func Inspect() *Inspector {
	inspection.mu.Lock()
	defer inspection.mu.Unlock()
	return inspection.insp
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
// It exists so that wiring is not a thing to remember. tea.NewProgram builds a
// program this library cannot reach, so a program started that way answers
// every op=key with "its host wired no key handler" -- true, unhelpful, and
// entirely avoidable by there being one way to start.
func Run(m tea.Model, opts ...tea.ProgramOption) (tea.Model, error) {
	p := tea.NewProgram(m, opts...)
	inspection.drive(p)
	defer inspection.close()
	if err := InspectError(); err != nil {
		return nil, err
	}
	return p.Run()
}
