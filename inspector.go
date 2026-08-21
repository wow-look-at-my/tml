package tml

import (
	"fmt"
	"maps"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wow-look-at-my/tml/inspect"
	"github.com/wow-look-at-my/tml/layout"
)

// Inspector exposes a running program's frames.
//
// A program that renders through a View gets one with two lines: make it, then
// let it serve. The View records every frame it paints, so nothing has to be
// re-rendered to answer a question and the answer is about the frame that is
// actually on screen.
type Inspector struct {
	view *View
	srv  *inspect.Server

	// keys and clicks are how the inspector drives the program. A host wires
	// them to whatever delivers input -- for Bubble Tea that is Program.Send.
	// Leaving them nil makes the program read-only, and the protocol says so
	// rather than silently doing nothing.
	mu     sync.Mutex
	onKey  func(string) error
	onClk  func(x, y int) error
	onPnt  func() error
	styles map[string]map[string]string
}

// newInspector returns an inspector reading v's frames. The process has one,
// built by the first Load; a second would fight it for the engine override and
// the frame hook, which is why nothing outside this package can make one.
func newInspector(v *View) *Inspector {
	i := &Inspector{styles: map[string]map[string]string{}}
	i.srv = inspect.NewServer(i)
	i.Attach(v)
	return i
}

// Attach points the inspector at another View, and is how a host survives
// recompiling its own document.
//
// Load bakes in which half of every theme token resolves and how a width is
// measured, so a host that learns either late -- a terminal answering OSC 11,
// or mode 2027 -- loads again and renders through a NEW View. The old one stops
// painting. An inspector still reading it answers about a frame nothing is
// drawing, which reads as a program that froze rather than one that changed
// theme.
//
// The new View continues the old one's frame numbers. A caller waiting for a
// frame newer than the one it read must not be answered by a fresh View's
// first frame, which would otherwise number 1.
func (i *Inspector) Attach(v *View) {
	i.mu.Lock()
	old := i.view
	i.view = v
	i.mu.Unlock()
	if old == v {
		return
	}

	v.recordFrames(true)
	if old != nil && old.frames != nil {
		v.frames.seq.Store(old.frames.seq.Load())
		old.frames.on.Store(false)
	}
	v.engine.SetOverride(i.override)
	v.OnFrame(i.Publish)
	i.srv.Publish()
}

// currentView reads the attached View. Every path that touches it goes through
// here, because Attach can replace it while a request is being answered.
func (i *Inspector) currentView() *View {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.view
}

// OnKey and OnClick wire the driving half. A host calls them once, before
// serving.
func (i *Inspector) OnKey(fn func(key string) error) {
	i.mu.Lock()
	i.onKey = fn
	i.mu.Unlock()
}

func (i *Inspector) OnClick(fn func(x, y int) error) {
	i.mu.Lock()
	i.onClk = fn
	i.mu.Unlock()
}

// OnRepaint wires the way this host is asked to draw again.
//
// An override only reaches the screen on the next paint, and an idle program
// does not paint. Without this the terminal keeps the old geometry until some
// other event arrives, so a restyle looks like it did nothing. A host wires it
// to whatever wakes its loop: for Bubble Tea that is Program.Send.
func (i *Inspector) OnRepaint(fn func() error) {
	i.mu.Lock()
	i.onPnt = fn
	i.mu.Unlock()
}

// repaint asks the host to draw again and waits for the frame that answers.
//
// It waits so a caller that restyles and then reads gets the new geometry
// rather than the geometry it was trying to change. A host that wired no
// repaint says so: a silent return would report a restyle that never reached
// the terminal.
func (i *Inspector) repaint() error {
	i.mu.Lock()
	fn := i.onPnt
	i.mu.Unlock()
	if fn == nil {
		return fmt.Errorf("this program cannot be asked to redraw: build it with tml.NewProgram or tml.Run rather than tea.NewProgram")
	}
	before, _ := i.currentView().lastFrame()
	if err := fn(); err != nil {
		return err
	}
	deadline := time.After(2 * time.Second)
	for {
		if now, ok := i.currentView().lastFrame(); ok && now.Seq > before.Seq {
			return nil
		}
		select {
		case <-time.After(2 * time.Millisecond):
		case <-deadline:
			return fmt.Errorf("the program did not repaint within 2s of being asked")
		}
	}
}

// ListenSocket serves the line protocol at path.
func (i *Inspector) ListenSocket(path string) error { return i.srv.ListenSocket(path) }

// ListenHTTP serves the browser inspector and returns its URL.
func (i *Inspector) ListenHTTP(addr string) (string, error) { return i.srv.ListenHTTP(addr) }

// Close stops serving.
func (i *Inspector) Close() error { return i.srv.Close() }

// Frame implements inspect.Source.
func (i *Inspector) Frame() (inspect.Frame, bool) { return i.currentView().lastFrame() }

// Key implements inspect.Controller.
func (i *Inspector) Key(key string) error {
	i.mu.Lock()
	fn := i.onKey
	i.mu.Unlock()
	if fn == nil {
		return fmt.Errorf("this program can be read and not driven: build it with tml.NewProgram or tml.Run rather than tea.NewProgram")
	}
	return fn(key)
}

// Click implements inspect.Controller.
func (i *Inspector) Click(x, y int) error {
	i.mu.Lock()
	fn := i.onClk
	i.mu.Unlock()
	if fn == nil {
		return fmt.Errorf("this program can be read and not driven: build it with tml.NewProgram or tml.Run rather than tea.NewProgram")
	}
	return fn(x, y)
}

// Restyle implements inspect.Controller. It returns once the program has
// painted with the override, so what the caller reads next is what the
// terminal shows.
func (i *Inspector) Restyle(id string, attrs map[string]string) error {
	i.mu.Lock()
	current := i.styles[id]
	if current == nil {
		current = map[string]string{}
		i.styles[id] = current
	}
	maps.Copy(current, attrs)
	i.mu.Unlock()
	return i.repaint()
}

// Reset implements inspect.Controller.
func (i *Inspector) Reset() error {
	i.mu.Lock()
	i.styles = map[string]map[string]string{}
	i.mu.Unlock()
	return i.repaint()
}

// override is what the layout engine asks per element.
func (i *Inspector) override(id string) map[string]string {
	if id == "" {
		return nil
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.styles[id]
}

// Publish wakes anything waiting for a newer frame. The View calls it after
// each paint.
func (i *Inspector) Publish() { i.srv.Publish() }

// --- the View's side of the recording ---

// frameRecord is the last frame a View painted.
type frameRecord struct {
	mu    sync.Mutex
	on    atomic.Bool
	seq   atomic.Uint64
	frame inspect.Frame
	hook  func()
}

func (v *View) recordFrames(on bool) {
	if v.frames == nil {
		v.frames = &frameRecord{}
	}
	v.frames.on.Store(on)
}

// record stores the frame just painted. It is called from Render, so the cost
// on a program with no inspector attached is one atomic load.
func (v *View) record(box *layout.Box, ansi string, width, height int) {
	if v.frames == nil || !v.frames.on.Load() {
		return
	}
	seq := v.frames.seq.Add(1)
	v.frames.mu.Lock()
	v.frames.frame = inspect.Frame{
		Seq: seq, At: time.Now(), Width: width, Height: height,
		Box: box, State: inspect.StateOf(v.ui.Targets()), ANSI: ansi,
	}
	hook := v.frames.hook
	v.frames.mu.Unlock()
	if hook != nil {
		hook()
	}
}

func (v *View) lastFrame() (inspect.Frame, bool) {
	if v.frames == nil {
		return inspect.Frame{}, false
	}
	v.frames.mu.Lock()
	defer v.frames.mu.Unlock()
	if v.frames.frame.Box == nil {
		return inspect.Frame{}, false
	}
	return v.frames.frame, true
}

// OnFrame registers a function called after each recorded frame. The inspector
// uses it to wake watchers the moment the program paints.
func (v *View) OnFrame(fn func()) {
	if v.frames == nil {
		v.frames = &frameRecord{}
	}
	v.frames.mu.Lock()
	v.frames.hook = fn
	v.frames.mu.Unlock()
}
