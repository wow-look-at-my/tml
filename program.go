// program.go holds Msg.

package tml

import (
	"errors"
	"io"
	"os"
	"sync"
	"time"

	"github.com/wow-look-at-my/tml/cell"
	"github.com/wow-look-at-my/tml/input"
	"github.com/wow-look-at-my/tml/term"
)

// Msg is anything a program's loop hands its app.
type Msg = any

// The input values a program delivers, under the names a host switches on.
type (
	// Key is one key press.
	Key = input.Key
	// Mouse is one pointer event.
	Mouse = input.Mouse
	// Paste is text the terminal delivered as one paste.
	Paste = input.Paste
	// Resize is the terminal's size. A program sends one before the first frame, and one for each change.
	Resize = input.Resize
)

// RepaintMsg asks the program to draw again. The inspector sends it after an override.
type RepaintMsg struct{}

// ErrInterrupted is what a program reports when an interrupt took it down: SIGINT, SIGTERM.
var ErrInterrupted = errors.New("tml: interrupted")

// App is a host's half of a program. The program owns the terminal, the loop and the view. The app owns the state.
type App interface {
	// Props is what the view draws from. The program reads it before every frame.
	Props() Props
	// Update takes one message, on the program's own goroutine, and reports whether it consumed it.
	Update(p *Program, msg Msg) bool
}

// DefaultInterval is how often a program steps its animations when ProgramOptions.Interval is zero.
const DefaultInterval = 100 * time.Millisecond

// escapeDelay is how long a lone ESC waits for the rest of a sequence before it is the escape key.
const escapeDelay = 50 * time.Millisecond

// ProgramOptions configure a program. The zero value is a full-screen program on the process's own terminal.
type ProgramOptions struct {
	// Input is where keys come from. Nil is os.Stdin.
	Input io.Reader
	// NoInput runs the program without reading anything.
	NoInput bool
	// Output is the terminal the program draws on. Nil is os.Stdout.
	Output io.Writer
	// Inline draws the view in place below what the terminal already shows, as tall as its content.
	Inline bool
	// MaxHeight caps the rows an inline program takes. Zero is the terminal's height.
	MaxHeight int
	// NoMouse leaves the pointer to the terminal, so the user can select text with it.
	NoMouse bool
	// Interval is how often the view's animations step. Zero is DefaultInterval.
	Interval time.Duration
	// Poll repaints on every interval, for an app whose props change with no message to announce it.
	Poll bool
	// Width and Height fix the size of the screen. Zero follows the terminal.
	Width, Height int
	// Env reads the environment that says what the terminal can show. Nil is os.Getenv.
	Env func(string) string
	// HostSignals leaves SIGINT and SIGTERM to the host. The program does not end on them.
	HostSignals bool
	// TooltipDelay is how long the pointer rests on a control before its tooltip shows. Zero is DefaultTooltipDelay.
	TooltipDelay time.Duration
}

// Without a size from the terminal, a program draws at these.
const (
	fallbackWidth  = 80
	fallbackHeight = 24
)

type programState int

const (
	programIdle programState = iota
	programRunning
	programStopped
)

// Program runs a view on a terminal. It reads input, routes it through the app and the focus ring, steps the
// animations, and paints the cells that changed.
type Program struct {
	view *View
	app  App
	opts ProgramOptions
	out  io.Writer

	// queue is the messages waiting for the loop, and signal wakes the loop for them.
	queue  []Msg
	signal chan struct{}
	done   chan struct{}

	screen        *term.Screen
	width, height int
	tty           bool
	dirty         bool
	unchanged     bool
	stopping      bool
	err           error
	// stopTooltip cancels the wait for a tooltip, or is nil when nothing waits.
	stopTooltip func()

	// mu guards the state and the lines waiting to print. Everything else belongs to the loop's goroutine.
	mu      sync.Mutex
	state   programState
	pending []string
	partial string
}

type (
	quitMsg      struct{}
	interruptMsg struct{}
	viewMsg      struct{ view *View }
	copyMsg      struct{ text string }
	titleMsg     struct{ text string }
	// measureMsg asks for a frame, which is when the program reads the terminal's size.
	measureMsg struct{}
	// tooltipMsg says the wait for a tooltip is over. dwell names the wait, as UI.Dwell gave it.
	tooltipMsg struct{ dwell int }
)

// NewProgram prepares a program that shows view and is driven by app. Nothing touches the terminal until Run. The
// error is the reason the inspection socket is not served, as InspectError reports it.
func NewProgram(view *View, app App, opts ...ProgramOptions) (*Program, error) {
	p := &Program{view: view, app: app, signal: make(chan struct{}, 1), done: make(chan struct{})}
	if len(opts) > 0 {
		p.opts = opts[0]
	}
	if p.opts.Interval <= 0 {
		p.opts.Interval = DefaultInterval
	}
	if p.opts.Env == nil {
		p.opts.Env = os.Getenv
	}
	if p.opts.TooltipDelay == 0 {
		p.opts.TooltipDelay = DefaultTooltipDelay
	}
	p.out = p.opts.Output
	if p.out == nil {
		p.out = os.Stdout
	}
	inspection.drive(p)
	return p, InspectError()
}

// Run is NewProgram and then running it, for a host with nothing to say to the program in between.
func Run(view *View, app App, opts ...ProgramOptions) error {
	p, err := NewProgram(view, app, opts...)
	if err != nil {
		return err
	}
	defer inspection.close()
	return p.Run()
}

// Send hands the loop a message from any goroutine, the loop's own included. It never blocks. After the program has
// ended the message is dropped.
func (p *Program) Send(msg Msg) {
	p.mu.Lock()
	if p.state == programStopped {
		p.mu.Unlock()
		return
	}
	p.queue = append(p.queue, msg)
	p.mu.Unlock()
	p.wake()
}

// received removes the messages that are waiting.
func (p *Program) received() []Msg {
	p.mu.Lock()
	defer p.mu.Unlock()
	msgs := p.queue
	p.queue = nil
	return msgs
}

// Quit ends the program after the messages already sent.
func (p *Program) Quit() { p.Send(quitMsg{}) }

// Go runs work on a goroutine of its own and sends what it returns. A nil result sends nothing.
func (p *Program) Go(work func() Msg) {
	go func() {
		if msg := work(); msg != nil {
			p.Send(msg)
		}
	}()
}

// After sends msg once the duration has passed. The function it returns cancels that.
func (p *Program) After(d time.Duration, msg Msg) (cancel func()) {
	timer := time.AfterFunc(d, func() { p.Send(msg) })
	return func() { timer.Stop() }
}

// Every sends msg each time the duration passes, until the program ends or the function it returns is called.
func (p *Program) Every(d time.Duration, msg Msg) (cancel func()) {
	stop := make(chan struct{})
	var once sync.Once
	go func() {
		ticker := time.NewTicker(d)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				p.Send(msg)
			case <-stop:
				return
			case <-p.done:
				return
			}
		}
	}()
	return func() { once.Do(func() { close(stop) }) }
}

// Size is the screen's size in cells.
func (p *Program) Size() (width, height int) { return p.width, p.height }

// View is the view the program shows.
func (p *Program) View() *View { return p.view }

// SetView swaps the view the program shows, which is how a host puts a reloaded document on screen.
func (p *Program) SetView(view *View) { p.Send(viewMsg{view: view}) }

// Copy puts text on the system clipboard through the terminal, with a later OSC sequence.
func (p *Program) Copy(text string) { p.Send(copyMsg{text: text}) }

// SetTitle names the terminal's window. The terminal keeps the name after the program ends.
func (p *Program) SetTitle(text string) { p.Send(titleMsg{text: text}) }

// Done is closed when the program has ended and the terminal is restored.
func (p *Program) Done() <-chan struct{} { return p.done }

// Run takes the terminal, runs the loop until something ends it, and gives the terminal back. It reports why the
// program ended: nil for Quit, ErrInterrupted, or the error a frame failed to draw with. A program runs one time.
func (p *Program) Run() (err error) {
	p.mu.Lock()
	if p.state != programIdle {
		p.mu.Unlock()
		return errors.New("tml: a program runs one time")
	}
	p.state = programRunning
	p.mu.Unlock()

	stops, err := p.enter()
	// The terminal goes back however the loop ends, a panic in the app included.
	defer func() {
		p.leave(stops)
		if err == nil {
			err = p.err
		}
	}()
	if err != nil {
		return err
	}
	p.loop()
	return nil
}

// loop handles messages until one ends the program, and paints after each batch.
func (p *Program) loop() {
	p.dispatch(Resize{Width: p.width, Height: p.height})
	p.dirty = true

	var ticker *time.Ticker
	var ticks <-chan time.Time
	defer func() {
		if ticker != nil {
			ticker.Stop()
		}
	}()

	for !p.stopping {
		if p.dirty {
			if err := p.paint(); err != nil {
				p.err = err
				return
			}
		}
		// The clock runs only while something on screen moves, so an idle program costs nothing.
		if want := p.opts.Poll || p.view.Animating(); want && ticker == nil {
			ticker = time.NewTicker(p.opts.Interval)
			ticks = ticker.C
		} else if !want && ticker != nil {
			ticker.Stop()
			ticker, ticks = nil, nil
		}

		select {
		case <-p.signal:
			// A burst of input is one frame, not a frame for each value.
			for _, msg := range p.received() {
				if p.stopping {
					break
				}
				p.dispatch(msg)
			}
			if p.linesWait() {
				p.dirty = true
			}
		case <-ticks:
			p.view.Advance()
			p.dirty = true
		}
	}
}

// Unchanged says that the message Update is handling changed nothing on screen.
func (p *Program) Unchanged() { p.unchanged = true }

// dispatch routes one message, and asks for a frame unless the app said the message changed nothing.
func (p *Program) dispatch(msg Msg) {
	if !p.route(msg) {
		p.dirty = true
	}
}

// route hands one message on. It reports whether the app took the message and called Unchanged.
func (p *Program) route(msg Msg) bool {
	switch msg := msg.(type) {
	case quitMsg:
		p.stopping = true
	case interruptMsg:
		p.err = ErrInterrupted
		p.stopping = true
	case measureMsg:
	case viewMsg:
		p.view = msg.view
		p.screen.Invalidate()
	case copyMsg:
		if err := p.screen.Clipboard(msg.text); err != nil {
			p.err = err
			p.stopping = true
		}
	case titleMsg:
		if err := p.screen.Title(msg.text); err != nil {
			p.err = err
			p.stopping = true
		}
		// A title is outside the frame.
		return true
	case Resize, RepaintMsg:
		p.app.Update(p, msg)
	case tooltipMsg:
		return !p.view.UI().ShowTooltip(msg.dwell)
	case Key, Mouse, Paste:
		if p.update(msg) {
			// The ring does not see a value the app kept, and a tooltip still goes away on it.
			hid := p.view.UI().Observe(msg)
			p.awaitTooltip()
			return p.unchanged && !hid
		}
		before := p.view.UI().pointerState()
		events := p.view.UI().Update(msg)
		p.awaitTooltip()
		for _, event := range events {
			p.app.Update(p, event)
		}
		// A pointer that moved inside the element it was already on changed no cell, so it asks for no frame.
		if _, pointer := msg.(Mouse); pointer && len(events) == 0 && before == p.view.UI().pointerState() {
			return true
		}
		// A ctrl+c nobody took still ends the program, so a host cannot leave its user without a way out.
		if key, ok := msg.(Key); ok && len(events) == 0 && key.Keystroke() == "ctrl+c" {
			p.err = ErrInterrupted
			p.stopping = true
		}
	default:
		p.update(msg)
		return p.unchanged
	}
	return false
}

// awaitTooltip starts the wait for a tooltip again after an input. The pointer has to rest on a control for the
// whole delay.
func (p *Program) awaitTooltip() {
	if p.stopTooltip != nil {
		p.stopTooltip()
		p.stopTooltip = nil
	}
	dwell, waits := p.view.UI().Dwell()
	if !waits || p.opts.TooltipDelay < 0 {
		return
	}
	p.stopTooltip = p.After(p.opts.TooltipDelay, tooltipMsg{dwell: dwell})
}

// update is one call of the app's Update, with Unchanged not yet said.
func (p *Program) update(msg Msg) bool {
	p.unchanged = false
	return p.app.Update(p, msg)
}

// paint prints the queued lines, lays the view out at the screen's size, and draws the frame.
func (p *Program) paint() error {
	p.dirty = false
	if p.measure() {
		p.screen.Invalidate()
		p.app.Update(p, Resize{Width: p.width, Height: p.height})
	}
	if lines := p.take(false); len(lines) > 0 && p.opts.Inline {
		if err := p.screen.Print(lines); err != nil {
			return err
		}
	} else if len(lines) > 0 {
		// A full-screen program holds its lines until it ends.
		p.mu.Lock()
		p.pending = append(lines, p.pending...)
		p.mu.Unlock()
	}

	var frame *cell.Buffer
	var err error
	if p.opts.Inline {
		limit := p.opts.MaxHeight
		if limit <= 0 {
			limit = p.height
		}
		frame, err = p.view.DrawFit(p.app.Props(), p.width, limit)
	} else {
		frame, err = p.view.Draw(p.app.Props(), p.width, p.height)
	}
	if err != nil {
		return err
	}
	return p.screen.Draw(frame)
}
