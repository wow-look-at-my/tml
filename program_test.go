package tml_test

import _ "embed"

import (
	"io"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml"
	"github.com/wow-look-at-my/tml/input"
	"github.com/wow-look-at-my/tml/internal/vt"
	"github.com/wow-look-at-my/tml/sema"
)

//go:embed testdata/program.xml
var programDoc string

// otherDoc takes the same props as programDoc and draws something else, for a view swap.
//
//go:embed testdata/program-other.xml
var otherDoc string

const (
	programWidth  = 40
	programHeight = 10
)

func loadDoc(t *testing.T, doc string) *tml.View {
	t.Helper()
	view, err := tml.Load(fstest.MapFS{"app.tml": &fstest.MapFile{Data: []byte(doc)}}, "app.tml", tml.Options{})
	require.NoError(t, err)
	return view
}

// recordApp hands every message to the test and consumes none. An empty title leaves the required prop out.
type recordApp struct {
	title string
	got   chan tml.Msg
}

func newRecordApp(title string) recordApp {
	return recordApp{title: title, got: make(chan tml.Msg, 1024)}
}

func (a recordApp) Props() tml.Props {
	if a.title == "" {
		return tml.Props{}
	}
	return tml.Props{"title": sema.StringValue(a.title)}
}

func (a recordApp) Update(_ *tml.Program, msg tml.Msg) bool {
	select {
	case a.got <- msg:
	default:
	}
	return false
}

// typed is a program that reads what the test writes to its pipe and draws on a buffer the test can replay.
type typed struct {
	program *tml.Program
	app     recordApp
	keys    *io.PipeWriter
	out     *syncBuffer
	result  <-chan error
}

func startTyped(t *testing.T, opts tml.ProgramOptions) *typed {
	t.Helper()
	view := loadDoc(t, programDoc)
	reader, writer := io.Pipe()
	t.Cleanup(func() { _ = writer.Close() })
	out := &syncBuffer{}
	opts.Input, opts.Output, opts.Width, opts.Height = reader, out, programWidth, programHeight
	app := newRecordApp("hello")
	p, err := tml.NewProgram(view, app, opts)
	require.NoError(t, err)
	return &typed{program: p, app: app, keys: writer, out: out, result: runProgram(t, p)}
}

func (r *typed) typeBytes(t *testing.T, b string) {
	t.Helper()
	_, err := r.keys.Write([]byte(b))
	require.NoError(t, err)
}

// screen replays what the program wrote so far through a terminal of the program's size.
func screen(written string) []string {
	term := vt.New(programWidth, programHeight)
	term.WriteString(written)
	return term.Lines()
}

func onScreen(written, want string) bool {
	return slices.ContainsFunc(screen(written), func(line string) bool { return strings.Contains(line, want) })
}

// waitResult is what Run returned, or a failure when it does not return.
func waitResult(t *testing.T, result <-chan error) error {
	t.Helper()
	select {
	case err := <-result:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return")
		return nil
	}
}

// none reports whether no message of type T arrives within d.
func none[T any](got <-chan tml.Msg, d time.Duration) bool {
	deadline := time.After(d)
	for {
		select {
		case msg := <-got:
			if _, ok := msg.(T); ok {
				return false
			}
		case <-deadline:
			return true
		}
	}
}

type custom struct{ text string }

type tick struct{}

type once struct{}

type cancelled struct{}

type answer struct{ n int }

// The app hears the size first, then each byte as the key it is, and a key it does not take reaches the ring.
func TestAProgramRoutesInputThroughTheAppAndTheRing(t *testing.T) {
	t.Serial("the program wires its key and click handlers into the process inspector while it reads the pipe")
	exclusive(t)
	run := startTyped(t, tml.ProgramOptions{})

	first := <-run.app.got
	assert.Equal(t, tml.Resize{Width: programWidth, Height: programHeight}, first, "the size comes before anything else")

	require.Eventually(t, func() bool { return onScreen(run.out.String(), "hello") },
		5*time.Second, 5*time.Millisecond, "the view's text is on screen while the program runs")

	run.typeBytes(t, "a")
	assert.Equal(t, input.ParseKey("a"), waitFor[tml.Key](t, run.app.got), "a typed byte is a key")

	run.typeBytes(t, "\r")
	assert.Equal(t, input.ParseKey("enter"), waitFor[tml.Key](t, run.app.got))
	assert.Equal(t, tml.Event{Kind: tml.Activated, ID: "go", Action: "go", X: -1, Y: -1}, waitFor[tml.Event](t, run.app.got),
		"a key the app did not take comes back from the focus ring")

	run.program.Quit()
	assert.NoError(t, waitResult(t, run.result), "Quit is a clean end")
}

// A ctrl+c that nothing took ends the program, so the user always has a way out.
func TestAnUnconsumedCtrlCInterrupts(t *testing.T) {
	t.Serial("this program holds the inspector's driving hooks until the interrupt takes it down")
	exclusive(t)
	run := startTyped(t, tml.ProgramOptions{})

	run.typeBytes(t, "\x03")
	assert.Equal(t, input.ParseKey("ctrl+c"), waitFor[tml.Key](t, run.app.got), "the app still hears it first")
	assert.ErrorIs(t, waitResult(t, run.result), tml.ErrInterrupted)
}

// Messages from the host's own goroutines and clocks all reach Update.
func TestAProgramDeliversTheHostsMessages(t *testing.T) {
	t.Serial("its timers and goroutines feed a program that owns the inspector hooks for the process")
	exclusive(t)
	app := newRecordApp("hello")
	p, err := tml.NewProgram(loadDoc(t, programDoc), app, quietOptions())
	require.NoError(t, err)
	runProgram(t, p)

	p.Send(custom{text: "x"})
	assert.Equal(t, custom{text: "x"}, waitFor[custom](t, app.got))

	p.Go(func() tml.Msg { return answer{n: 42} })
	assert.Equal(t, answer{n: 42}, waitFor[answer](t, app.got))

	p.After(5*time.Millisecond, once{})
	waitFor[once](t, app.got)
	assert.True(t, none[once](app.got, 100*time.Millisecond), "After delivers one time")

	stop := p.After(20*time.Millisecond, cancelled{})
	stop()
	assert.True(t, none[cancelled](app.got, 100*time.Millisecond), "a cancelled After delivers nothing")

	cancel := p.Every(5*time.Millisecond, tick{})
	for range 3 {
		waitFor[tick](t, app.got)
	}
	cancel()
	// A tick already on its way may still land.
	none[tick](app.got, 50*time.Millisecond)
	assert.True(t, none[tick](app.got, 100*time.Millisecond), "cancel stops Every")
}

// A frame that cannot draw ends the program with the reason.
func TestAProgramEndsOnADrawFailure(t *testing.T) {
	t.Serial("a program that fails its first frame still registered itself as the one the inspector drives")
	exclusive(t)
	p, err := tml.NewProgram(loadDoc(t, programDoc), newRecordApp(""), quietOptions())
	require.NoError(t, err)

	result := runProgram(t, p)
	assert.ErrorContains(t, waitResult(t, result), "title", "the error names the property that was missing")
}

// A program runs one time.
func TestAProgramRunsOneTime(t *testing.T) {
	t.Serial("the run under test claims the process-wide inspector driving hooks until Quit releases the loop")
	exclusive(t)
	p, err := tml.NewProgram(loadDoc(t, programDoc), newRecordApp("hello"), quietOptions())
	require.NoError(t, err)

	result := runProgram(t, p)
	p.Quit()
	require.NoError(t, waitResult(t, result))
	<-p.Done()
	assert.Error(t, p.Run(), "a second Run is refused")
}

// SetView puts another view on screen, and a full-screen program holds a printed line until it ends.
func TestAProgramSwapsItsView(t *testing.T) {
	t.Serial("the swapped view is loaded into, and attached to, the single inspector of the process")
	exclusive(t)
	run := startTyped(t, tml.ProgramOptions{})
	require.Eventually(t, func() bool { return onScreen(run.out.String(), "hello") },
		5*time.Second, 5*time.Millisecond)

	other := loadDoc(t, otherDoc)
	run.program.SetView(other)
	require.Eventually(t, func() bool { return onScreen(run.out.String(), "other hello") },
		5*time.Second, 5*time.Millisecond, "the swapped view is drawn")

	run.program.Println("held")
	run.program.Quit()
	require.NoError(t, waitResult(t, run.result))
	assert.Equal(t, other, run.program.View())
	assert.Equal(t, []string{"held"}, screen(run.out.String()),
		"the alternate screen is gone and the held line printed after it")
}

func TestAProgramCopiesToTheClipboard(t *testing.T) {
	t.Serial("the program's view is attached to the single inspector of the process")
	exclusive(t)
	run := startTyped(t, tml.ProgramOptions{})
	require.Eventually(t, func() bool { return onScreen(run.out.String(), "hello") },
		5*time.Second, 5*time.Millisecond)
	run.program.Copy("owner/repo")
	// base64 of "owner/repo", inside a later OSC sequence that sets the clipboard.
	require.Eventually(t, func() bool { return strings.Contains(run.out.String(), "\x1b]52;c;b3duZXIvcmVwbw==\x07") },
		5*time.Second, 5*time.Millisecond, "the program writes the clipboard sequence")
	run.program.Quit()
	require.NoError(t, waitResult(t, run.result))
}

// An inline program prints above its region, and only the printed lines stay once it ends.
func TestAnInlineProgramLeavesOnlyItsLines(t *testing.T) {
	t.Serial("an inline program takes over the inspector's key, click and repaint handlers for the process")
	exclusive(t)
	out := &syncBuffer{}
	p, err := tml.NewProgram(loadDoc(t, programDoc), newRecordApp("hello"), tml.ProgramOptions{
		Inline: true, NoInput: true, Output: out, Width: programWidth, Height: programHeight,
	})
	require.NoError(t, err)
	result := runProgram(t, p)

	require.Eventually(t, func() bool { return onScreen(out.String(), "hello") },
		5*time.Second, 5*time.Millisecond)
	p.Println("one")
	p.Printf("two %d", 2)
	require.Eventually(t, func() bool { return onScreen(out.String(), "two 2") },
		5*time.Second, 5*time.Millisecond)
	lines := screen(out.String())
	assert.Equal(t, []string{"one", "two 2"}, lines[:2], "the lines sit above the region")
	assert.Contains(t, lines[2], "hello", "and the region below them")

	p.Quit()
	require.NoError(t, waitResult(t, result))
	assert.Equal(t, []string{"one", "two 2"}, screen(out.String()), "the region is gone and the lines stay")
}
