// program.go holds Msg.

package tml

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
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

	msgs chan Msg
	done chan struct{}

	screen        *term.Screen
	width, height int
	tty           bool
	dirty         bool
	stopping      bool
	err           error

	// mu guards the state and the lines waiting to print. Everything else belongs to the loop's goroutine.
	mu      sync.Mutex
	state   programState
	pending []string
	partial string
}

type (
	quitMsg      struct{}
	interruptMsg struct{}
	wakeMsg      struct{}
	viewMsg      struct{ view *View }
)

// NewProgram prepares a program that shows view and is driven by app. Nothing touches the terminal until Run. The
// error is the reason the inspection socket is not served, as InspectError reports it.
func NewProgram(view *View, app App, opts ...ProgramOptions) (*Program, error) {
	p := &Program{view: view, app: app, msgs: make(chan Msg, 256), done: make(chan struct{})}
	if len(opts) > 0 {
		p.opts = opts[0]
	}
	if p.opts.Interval <= 0 {
		p.opts.Interval = DefaultInterval
	}
	if p.opts.Env == nil {
		p.opts.Env = os.Getenv
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

// Send hands the loop a message from any goroutine. After the program has ended the message is dropped.
func (p *Program) Send(msg Msg) {
	select {
	case p.msgs <- msg:
	case <-p.done:
	}
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

// Done is closed when the program has ended and the terminal is restored.
func (p *Program) Done() <-chan struct{} { return p.done }

// Println queues a line to print above an inline program.
func (p *Program) Println(a ...any) { p.line(fmt.Sprint(a...)) }

// Printf queues a formatted line. A trailing newline is not needed.
func (p *Program) Printf(format string, a ...any) { p.line(fmt.Sprintf(format, a...)) }

// Write takes output such as a child's stderr. A partial line waits for its newline, so a line never arrives in
// pieces.
func (p *Program) Write(b []byte) (int, error) {
	p.mu.Lock()
	if p.state == programStopped {
		p.mu.Unlock()
		return p.out.Write(b)
	}
	p.partial += strings.ReplaceAll(string(b), "\r", "")
	for {
		i := strings.IndexByte(p.partial, '\n')
		if i < 0 {
			break
		}
		p.pending = append(p.pending, p.partial[:i])
		p.partial = p.partial[i+1:]
	}
	p.mu.Unlock()
	p.wake()
	return len(b), nil
}

func (p *Program) line(s string) {
	p.mu.Lock()
	if p.state == programStopped {
		p.mu.Unlock()
		p.writeLines([]string{s})
		return
	}
	p.pending = append(p.pending, strings.Split(strings.TrimSuffix(s, "\n"), "\n")...)
	p.mu.Unlock()
	p.wake()
}

// wake asks the loop to look at the queue. A loop that is already behind will get to it without being asked.
func (p *Program) wake() {
	select {
	case p.msgs <- wakeMsg{}:
	default:
	}
}

// take removes the queued lines. The partial line comes too when the program is about to end.
func (p *Program) take(partial bool) []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	lines := p.pending
	p.pending = nil
	if partial && p.partial != "" {
		lines = append(lines, p.partial)
		p.partial = ""
	}
	return lines
}

func (p *Program) writeLines(lines []string) {
	for _, line := range lines {
		_, _ = fmt.Fprintln(p.out, line)
	}
}

// flush is Stop for a program that never ran: the queued lines go straight to the output.
func (p *Program) flush() bool {
	p.mu.Lock()
	if p.state != programIdle {
		p.mu.Unlock()
		return false
	}
	p.state = programStopped
	p.mu.Unlock()
	p.writeLines(p.take(true))
	close(p.done)
	return true
}

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

// enter takes the terminal. It returns what leave must undo, in order.
func (p *Program) enter() (stops []func(), err error) {
	p.tty = term.IsTerminal(p.out)
	p.measure()

	hasInput := !p.opts.NoInput
	if hasInput {
		in := p.opts.Input
		if in == nil {
			in = os.Stdin
		}
		if term.IsTerminal(in) {
			restore, rawErr := term.Raw(in)
			if rawErr != nil {
				return stops, fmt.Errorf("tml: cannot put the terminal in raw mode: %w", rawErr)
			}
			stops = append(stops, func() { _ = restore() })
		}
		reader, readErr := term.NewReader(in)
		if readErr != nil {
			return stops, fmt.Errorf("tml: cannot read the terminal: %w", readErr)
		}
		stops = append(stops, func() { _ = reader.Close() })
		go p.read(reader)
	}

	restoreOutput := term.EnableOutput(p.out)
	stops = append(stops, func() { _ = restoreOutput() })

	p.screen = term.NewScreen(p.out, term.Options{
		Inline:  p.opts.Inline,
		Mouse:   hasInput && !p.opts.NoMouse && !p.opts.Inline,
		Paste:   hasInput,
		Profile: cell.DetectProfile(p.opts.Env, p.tty),
	})
	if err := p.screen.Enter(); err != nil {
		return stops, err
	}

	signals := make(chan os.Signal, 4)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	resizes := make(chan os.Signal, 1)
	stopResize := term.NotifyResize(resizes)
	quit := make(chan struct{})
	go func() {
		for {
			select {
			case <-signals:
				p.Send(interruptMsg{})
			case <-resizes:
				p.Send(wakeMsg{})
			case <-quit:
				return
			}
		}
	}()
	stops = append(stops, func() {
		signal.Stop(signals)
		stopResize()
		close(quit)
	})
	return stops, nil
}

// leave gives the terminal back: the queued lines print, the screen goes, and what enter took is undone.
func (p *Program) leave(stops []func()) {
	lines := p.take(true)
	if p.screen != nil {
		if p.opts.Inline {
			_ = p.screen.Print(lines)
			lines = nil
		}
		_ = p.screen.Leave()
	}
	for i := len(stops) - 1; i >= 0; i-- {
		stops[i]()
	}
	p.mu.Lock()
	p.state = programStopped
	p.mu.Unlock()
	// A line queued while the screen was leaving still belongs to the terminal.
	p.writeLines(append(lines, p.take(true)...))
	close(p.done)
}

// measure reads the screen's size, and reports whether it changed.
func (p *Program) measure() bool {
	width, height := p.opts.Width, p.opts.Height
	if width <= 0 || height <= 0 {
		w, h, err := term.Size(p.out)
		if err != nil || w <= 0 || h <= 0 {
			w, h = fallbackWidth, fallbackHeight
		}
		if width <= 0 {
			width = w
		}
		if height <= 0 {
			height = h
		}
	}
	changed := width != p.width || height != p.height
	p.width, p.height = width, height
	return changed
}

// read decodes the terminal's input and sends each value to the loop.
func (p *Program) read(reader term.Reader) {
	var (
		mu      sync.Mutex
		decoder term.Decoder
		round   int
	)
	send := func(values []any) {
		for _, value := range values {
			p.Send(value)
		}
	}
	buf := make([]byte, 4096)
	for {
		n, err := reader.Read(buf)
		mu.Lock()
		round++
		values := decoder.Feed(buf[:n])
		if decoder.Pending() {
			// Bytes that end inside a sequence wait a moment for the rest. A lone ESC that stays alone is the key.
			mine := round
			time.AfterFunc(escapeDelay, func() {
				mu.Lock()
				var late []any
				if round == mine {
					late = decoder.Flush()
				}
				mu.Unlock()
				send(late)
			})
		}
		mu.Unlock()
		send(values)
		if err != nil {
			return
		}
	}
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
		case msg := <-p.msgs:
			p.dispatch(msg)
			// A burst of input is one frame, not a frame for each value.
			for more := true; more && !p.stopping; {
				select {
				case msg := <-p.msgs:
					p.dispatch(msg)
				default:
					more = false
				}
			}
		case <-ticks:
			p.view.Advance()
			p.dirty = true
		}
	}
}

// dispatch routes one message.
func (p *Program) dispatch(msg Msg) {
	p.dirty = true
	switch msg := msg.(type) {
	case quitMsg:
		p.stopping = true
	case interruptMsg:
		p.err = ErrInterrupted
		p.stopping = true
	case wakeMsg:
	case viewMsg:
		p.view = msg.view
		p.screen.Invalidate()
	case Resize:
		p.app.Update(p, msg)
	case Key, Mouse, Paste:
		if p.app.Update(p, msg) {
			return
		}
		events := p.view.UI().Update(msg)
		for _, event := range events {
			p.app.Update(p, event)
		}
		// A ctrl+c nobody took still ends the program, so a host cannot leave its user without a way out.
		if key, ok := msg.(Key); ok && len(events) == 0 && key.Keystroke() == "ctrl+c" {
			p.err = ErrInterrupted
			p.stopping = true
		}
	default:
		p.app.Update(p, msg)
	}
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
