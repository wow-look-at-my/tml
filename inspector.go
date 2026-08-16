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
	styles map[string]map[string]string
}

// NewInspector returns an inspector reading v's frames.
func NewInspector(v *View) *Inspector {
	i := &Inspector{view: v, styles: map[string]map[string]string{}}
	v.recordFrames(true)
	v.engine.SetOverride(i.override)
	i.srv = inspect.NewServer(i)
	return i
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

// ListenSocket serves the line protocol at path.
func (i *Inspector) ListenSocket(path string) error { return i.srv.ListenSocket(path) }

// ListenHTTP serves the browser inspector and returns its URL.
func (i *Inspector) ListenHTTP(addr string) (string, error) { return i.srv.ListenHTTP(addr) }

// Close stops serving.
func (i *Inspector) Close() error { return i.srv.Close() }

// Frame implements inspect.Source.
func (i *Inspector) Frame() (inspect.Frame, bool) { return i.view.lastFrame() }

// Key implements inspect.Controller.
func (i *Inspector) Key(key string) error {
	i.mu.Lock()
	fn := i.onKey
	i.mu.Unlock()
	if fn == nil {
		return fmt.Errorf("this program accepts no input: its host wired no key handler")
	}
	return fn(key)
}

// Click implements inspect.Controller.
func (i *Inspector) Click(x, y int) error {
	i.mu.Lock()
	fn := i.onClk
	i.mu.Unlock()
	if fn == nil {
		return fmt.Errorf("this program accepts no pointer input: its host wired no click handler")
	}
	return fn(x, y)
}

// Restyle implements inspect.Controller. The override lands on the next frame
// the program paints, because the frame on screen is already painted.
func (i *Inspector) Restyle(id string, attrs map[string]string) error {
	i.mu.Lock()
	current := i.styles[id]
	if current == nil {
		current = map[string]string{}
		i.styles[id] = current
	}
	maps.Copy(current, attrs)
	i.mu.Unlock()
	return nil
}

// Reset implements inspect.Controller.
func (i *Inspector) Reset() error {
	i.mu.Lock()
	i.styles = map[string]map[string]string{}
	i.mu.Unlock()
	return nil
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
