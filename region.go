// region.go holds DefaultLiveInterval.

package tml

import (
	"errors"
	"io"
	"sync"
	"time"
)

// DefaultLiveInterval is how often a Live region repaints when LiveOptions.Interval is zero.
const DefaultLiveInterval = 100 * time.Millisecond

// LiveOptions configure a Live region.
type LiveOptions struct {
	// Interval is how often the region reads its props and repaints. Zero is DefaultLiveInterval.
	Interval time.Duration
	// Output is the terminal the region draws on. Nil is os.Stdout.
	Output io.Writer
	// MaxHeight caps the rows the region takes. Zero is the terminal's height.
	MaxHeight int
	// OnInterrupt runs after SIGINT took the region down and the terminal is restored.
	OnInterrupt func()
	// Width and Height fix the size the region draws at. Zero follows the terminal.
	Width, Height int
	// Env reads the environment that says what the terminal can show. Nil is os.Getenv.
	Env func(string) string
}

// Live draws a view inline, pinned below everything else the process prints,
// and repaints it in place.
type Live struct {
	program *Program
	opts    LiveOptions
	// inspect is why the inspection socket is not served, which Start reports.
	inspect error

	mu      sync.Mutex
	started bool
	done    chan struct{}
	err     error
}

// liveApp is the app behind a region: props from the host's function, and no input to handle.
type liveApp struct{ props func() Props }

func (a liveApp) Props() Props { return a.props() }

func (a liveApp) Update(*Program, Msg) bool { return false }

// NewLive prepares a region that renders view from props. Nothing draws until Start. A line printed before Start
// waits for it.
func NewLive(view *View, props func() Props, opts LiveOptions) *Live {
	if opts.Interval <= 0 {
		opts.Interval = DefaultLiveInterval
	}
	program, err := NewProgram(view, liveApp{props: props}, ProgramOptions{
		NoInput:   true,
		Output:    opts.Output,
		Inline:    true,
		MaxHeight: opts.MaxHeight,
		Interval:  opts.Interval,
		// The props change with no message to say so, and the interval is the region's clock.
		Poll:   true,
		Width:  opts.Width,
		Height: opts.Height,
		Env:    opts.Env,
	})
	return &Live{program: program, opts: opts, inspect: err, done: make(chan struct{})}
}

// Start shows the region and begins to repaint it. A region starts one time.
func (l *Live) Start() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.started {
		return errors.New("tml: a Live region starts one time")
	}
	if l.inspect != nil {
		return l.inspect
	}
	l.started = true
	go l.run()
	return nil
}

func (l *Live) run() {
	err := l.program.Run()
	l.mu.Lock()
	l.err = err
	l.mu.Unlock()
	close(l.done)
	if errors.Is(err, ErrInterrupted) && l.opts.OnInterrupt != nil {
		l.opts.OnInterrupt()
	}
}

// Stop prints what is still pending, removes the region and gives the cursor back. It reports why the region ended:
// nil, ErrInterrupted, or the error a frame failed to render with.
func (l *Live) Stop() error {
	l.mu.Lock()
	if !l.started {
		// A region that never started has only its queued lines to deliver.
		l.started = true
		l.mu.Unlock()
		l.program.flush()
		close(l.done)
		return nil
	}
	l.mu.Unlock()
	l.program.Quit()
	return l.Wait()
}

// Wait blocks until the region is gone, and reports what Stop reports.
func (l *Live) Wait() error {
	<-l.done
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.err
}

// Done is closed when the region is gone, for a host that selects on it.
func (l *Live) Done() <-chan struct{} { return l.done }

// Refresh repaints now instead of at the next interval.
func (l *Live) Refresh() { l.program.Send(RepaintMsg{}) }

// Println queues a line to print above the region.
func (l *Live) Println(a ...any) { l.program.Println(a...) }

// Printf queues a formatted line to print above the region. A trailing newline is not needed.
func (l *Live) Printf(format string, a ...any) { l.program.Printf(format, a...) }

// Write takes output such as a child's stderr. A partial line waits for its newline, so a line never arrives in pieces.
func (l *Live) Write(p []byte) (int, error) { return l.program.Write(p) }
