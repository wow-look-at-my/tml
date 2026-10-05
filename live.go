package tml

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"
)

// DefaultLiveInterval is how often a Live region repaints when LiveOptions.Interval is zero.
const DefaultLiveInterval = 100 * time.Millisecond

// ErrInterrupted is what Stop and Wait report when SIGINT took the region down.
var ErrInterrupted = tea.ErrInterrupted

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
	// Program goes to the Bubble Tea program after Live's own options, so it can override them.
	Program []tea.ProgramOption
}

// Live draws a view inline, pinned below everything else the process prints,
// and repaints it in place.
type Live struct {
	view  *View
	props func() Props
	opts  LiveOptions
	out   io.Writer

	mu      sync.Mutex
	state   liveState
	pending []string
	partial string
	program *tea.Program
	done    chan struct{}
	err     error
}

type liveState int

const (
	liveIdle liveState = iota
	liveRunning
	liveStopped
)

// NewLive prepares a region that renders view from props. Nothing draws until Start.
func NewLive(view *View, props func() Props, opts LiveOptions) *Live {
	out := opts.Output
	if out == nil {
		out = os.Stdout
	}
	if opts.Interval <= 0 {
		opts.Interval = DefaultLiveInterval
	}
	return &Live{view: view, props: props, opts: opts, out: out, done: make(chan struct{})}
}

// Start shows the region and begins to repaint it. A region starts one time.
func (l *Live) Start() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.state != liveIdle {
		return errors.New("tml: a Live region starts one time")
	}
	// The region takes no input. A child process can then own stdin, and ctrl+c stays a signal.
	options := []tea.ProgramOption{tea.WithInput(nil), tea.WithOutput(l.out), tea.WithFilter(liveFilter)}
	program, err := NewProgram(&liveModel{live: l}, append(options, l.opts.Program...)...)
	if err != nil {
		return err
	}
	l.program = program
	l.state = liveRunning
	go l.run()
	return nil
}

func (l *Live) run() {
	final, err := l.program.Run()
	if model, ok := final.(*liveModel); ok && err == nil {
		err = model.result()
	}
	l.mu.Lock()
	l.state = liveStopped
	l.err = err
	// A program that died before its last flush still owes these lines to the terminal.
	leftover := l.take(true)
	l.mu.Unlock()
	l.writeLines(leftover)
	close(l.done)
	if errors.Is(err, ErrInterrupted) && l.opts.OnInterrupt != nil {
		l.opts.OnInterrupt()
	}
}

// Stop prints what is still pending, removes the region and gives the cursor back. It reports why the region ended: nil,
// ErrInterrupted, or the error a frame failed to render with.
func (l *Live) Stop() error {
	l.mu.Lock()
	switch l.state {
	case liveIdle:
		l.state = liveStopped
		lines := l.take(true)
		close(l.done)
		l.mu.Unlock()
		l.writeLines(lines)
		return nil
	case liveStopped:
		l.mu.Unlock()
		return l.Wait()
	}
	program := l.program
	l.mu.Unlock()
	program.Send(liveClose{})
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
func (l *Live) Refresh() {
	l.mu.Lock()
	program, running := l.program, l.state == liveRunning
	l.mu.Unlock()
	if running {
		program.Send(RepaintMsg{})
	}
}

// Println queues a line to print above the region.
func (l *Live) Println(a ...any) { l.line(fmt.Sprint(a...)) }

// Printf queues a formatted line to print above the region. A trailing newline is not needed.
func (l *Live) Printf(format string, a ...any) { l.line(fmt.Sprintf(format, a...)) }

// Write takes output such as a child's stderr. A partial line waits for its newline, so a line never arrives in pieces.
func (l *Live) Write(p []byte) (int, error) {
	l.mu.Lock()
	if l.state == liveStopped {
		l.mu.Unlock()
		return l.out.Write(p)
	}
	defer l.mu.Unlock()
	l.partial += strings.ReplaceAll(string(p), "\r", "")
	for {
		i := strings.IndexByte(l.partial, '\n')
		if i < 0 {
			break
		}
		l.pending = append(l.pending, l.partial[:i])
		l.partial = l.partial[i+1:]
	}
	return len(p), nil
}

func (l *Live) line(s string) {
	l.mu.Lock()
	if l.state == liveStopped {
		l.mu.Unlock()
		l.writeLines([]string{s})
		return
	}
	defer l.mu.Unlock()
	l.pending = append(l.pending, strings.Split(strings.TrimSuffix(s, "\n"), "\n")...)
}

// take removes the queued lines. The partial line comes too when the region is about to end. Callers hold l.mu.
func (l *Live) take(partial bool) []string {
	lines := l.pending
	l.pending = nil
	if partial && l.partial != "" {
		lines = append(lines, l.partial)
		l.partial = ""
	}
	return lines
}

func (l *Live) writeLines(lines []string) {
	for _, line := range lines {
		_, _ = fmt.Fprintln(l.out, line)
	}
}

// liveFilter turns SIGINT into an orderly close, so the pending lines print and the region comes off the screen first.
func liveFilter(_ tea.Model, msg tea.Msg) tea.Msg {
	if _, ok := msg.(tea.InterruptMsg); ok {
		return liveClose{interrupted: true}
	}
	return msg
}

type (
	liveTick    struct{}
	livePrinted struct{}
	liveClose   struct{ interrupted bool }
)

// Without a size from the terminal, a region draws at these.
const (
	liveFallbackWidth  = 80
	liveFallbackHeight = 24
)

// liveModel is the program behind a Live region. At most one print is in flight, which keeps the lines in order.
type liveModel struct {
	live          *Live
	width, height int
	frame         string
	printing      bool
	closing       bool
	interrupted   bool
	err           error
}

// Init waits for the first WindowSizeMsg to paint, so the first frame is already the terminal's width.
func (m *liveModel) Init() tea.Cmd { return m.tick() }

func (m *liveModel) tick() tea.Cmd {
	return tea.Tick(m.live.opts.Interval, func(time.Time) tea.Msg { return liveTick{} })
}

func (m *liveModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, m.repaint()
	case RepaintMsg:
		return m, m.repaint()
	case liveTick:
		if m.closing {
			return m, nil
		}
		// The region's interval is its clock, so a step of every animation rides on it: a widget that animates itself is
		// drawn on a frame that has moved, and the host passes no frame counter of its own.
		m.live.view.Advance()
		return m, tea.Batch(m.tick(), m.repaint(), m.flush())
	case livePrinted:
		m.printing = false
		return m, m.flush()
	case liveClose:
		m.interrupted = m.interrupted || msg.interrupted
		return m, m.close()
	}
	return m, nil
}

// repaint reads the host's props and lays the view out again. A frame that fails to render ends the region with that
// error, because a region that keeps its last good frame looks like progress that stopped.
func (m *liveModel) repaint() tea.Cmd {
	if m.closing {
		return nil
	}
	width, height := m.width, m.live.opts.MaxHeight
	if width <= 0 {
		width = liveFallbackWidth
	}
	if height <= 0 {
		height = m.height
	}
	if height <= 0 {
		height = liveFallbackHeight
	}
	frame, err := m.live.view.RenderFit(m.live.props(), width, height)
	if err != nil {
		m.err = err
		return m.close()
	}
	m.frame = frame
	return nil
}

// close empties the frame, which takes the region off the screen, and quits after the last line prints.
func (m *liveModel) close() tea.Cmd {
	m.closing = true
	m.frame = ""
	return m.flush()
}

// flush prints the queued lines above the region. The done message follows
// the print through Sequence, so the next batch cannot overtake this.
func (m *liveModel) flush() tea.Cmd {
	if m.printing {
		return nil
	}
	m.live.mu.Lock()
	lines := m.live.take(m.closing)
	m.live.mu.Unlock()
	if len(lines) == 0 {
		if m.closing {
			return tea.Quit
		}
		return nil
	}
	m.printing = true
	body := strings.Join(lines, "\n")
	if body == "" {
		// The renderer drops an empty print, and a blank line is still a line.
		body = " "
	}
	return tea.Sequence(tea.Println(body), func() tea.Msg { return livePrinted{} })
}

// result is why the region ended, once the program has returned cleanly.
func (m *liveModel) result() error {
	if m.err != nil {
		return m.err
	}
	if m.interrupted {
		return ErrInterrupted
	}
	return nil
}

func (m *liveModel) View() tea.View {
	view := tea.NewView(m.frame)
	view.DisableBracketedPasteMode = true
	return view
}
