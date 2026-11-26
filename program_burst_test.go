// program_burst_test.go covers the inputs of one burst, each resolved against the state the one before it left.

package tml_test

import _ "embed"

import (
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml"
	"github.com/wow-look-at-my/tml/input"
	"github.com/wow-look-at-my/tml/sema"
)

//go:embed testdata/program-burst.xml
var burstDoc string

const (
	burstWidth  = 40
	burstHeight = 12
	// syncOn opens the synchronized update that holds each frame the screen writes.
	syncOn = "\x1b[?2026h"
)

type (
	// burstEnd marks the end of a burst. The app says it changed nothing.
	burstEnd struct{}
	// burstQuiet is a message the app says changed nothing.
	burstQuiet struct{}
)

// burstApp keeps the field's value and caret from Changed events and scrolls
// the log from where it last landed.
type burstApp struct {
	value, cursor string
	offset        int
	lines         []string
	activated     []string
	props         atomic.Int64
	ended         chan struct{}
}

func newBurstApp() *burstApp {
	lines := make([]string, 20)
	for i := range lines {
		lines[i] = "line " + strconv.Itoa(i)
	}
	return &burstApp{cursor: "0", lines: lines, ended: make(chan struct{})}
}

func (a *burstApp) Props() tml.Props {
	a.props.Add(1)
	return tml.Props{
		"value":  sema.StringValue(a.value),
		"cursor": sema.StringValue(a.cursor),
		"offset": sema.StringValue(strconv.Itoa(a.offset)),
		"lines":  sema.ListValue(a.lines),
	}
}

func (a *burstApp) Update(p *tml.Program, msg tml.Msg) bool {
	switch msg := msg.(type) {
	case burstEnd:
		p.Unchanged()
		close(a.ended)
	case burstQuiet:
		p.Unchanged()
	case tml.Key:
		if msg.String() == "q" {
			p.Unchanged()
			return true
		}
	case tml.Event:
		a.event(p, msg)
	}
	return false
}

func (a *burstApp) event(p *tml.Program, event tml.Event) {
	switch event.Kind {
	case tml.Changed:
		a.value, a.cursor = event.Value, strconv.Itoa(event.Cursor)
	case tml.Scrolled:
		target, _ := p.View().UI().Target("log")
		a.offset = target.Scroll.Y + event.Delta
	case tml.Activated:
		a.activated = append(a.activated, event.ID)
	}
}

// runBurst queues every message before the program starts, so the loop takes them all as one batch. It returns the
// app once the program has ended, and what the program wrote.
func runBurst(t *testing.T, msgs ...tml.Msg) (*burstApp, string) {
	t.Helper()
	app := newBurstApp()
	out := &syncBuffer{}
	p, err := tml.NewProgram(loadDoc(t, burstDoc), app, tml.ProgramOptions{
		NoInput: true, Output: out, Width: burstWidth, Height: burstHeight,
	})
	require.NoError(t, err)
	for _, msg := range msgs {
		p.Send(msg)
	}
	p.Send(burstEnd{})
	result := runProgram(t, p)
	select {
	case <-app.ended:
	case <-time.After(5 * time.Second):
		t.Fatal("the burst never reached its end")
	}
	p.Quit()
	require.NoError(t, waitResult(t, result))
	return app, out.String()
}

func keys(strokes ...string) []tml.Msg {
	msgs := make([]tml.Msg, 0, len(strokes))
	for _, stroke := range strokes {
		msgs = append(msgs, input.ParseKey(stroke))
	}
	return msgs
}

// sequential is the value and caret the field holds when every key gets a frame of its own.
func sequential(t *testing.T, strokes ...string) (string, int) {
	t.Helper()
	view := loadDoc(t, burstDoc)
	app := newBurstApp()
	for _, stroke := range strokes {
		_, err := view.Layout(app.Props(), burstWidth, burstHeight)
		require.NoError(t, err)
		for _, event := range view.UI().Update(input.ParseKey(stroke)) {
			if event.Kind == tml.Changed {
				app.value, app.cursor = event.Value, strconv.Itoa(event.Cursor)
			}
		}
	}
	cursor, err := strconv.Atoi(app.cursor)
	require.NoError(t, err)
	return app.value, cursor
}

// landed is where an element lands when the view is laid out with a field that holds value.
func landed(t *testing.T, value, id string) tml.Target {
	t.Helper()
	view := loadDoc(t, burstDoc)
	app := newBurstApp()
	app.value = value
	_, err := view.Layout(app.Props(), burstWidth, burstHeight)
	require.NoError(t, err)
	target, ok := view.UI().Target(id)
	require.True(t, ok, "the view has no %q", id)
	return target
}

// Keys typed in one burst all reach the field, and the burst is one frame on the terminal.
func TestABurstOfKeysTypesEveryKey(t *testing.T) {
	t.Serial("the program under test holds the inspector's driving hooks of the process while it runs")
	exclusive(t)

	app, written := runBurst(t, keys("l", "e", "a", "s", "e")...)
	assert.Equal(t, "lease", app.value, "every key of the burst edits what the key before it left")
	assert.Equal(t, "5", app.cursor, "the caret ends after the text")
	assert.Equal(t, 2, strings.Count(written, syncOn), "the first frame and one frame for the whole burst")
}

// A burst that types, moves the caret and deletes ends where key-by-key editing ends.
func TestABurstOfEditsMatchesSequentialEditing(t *testing.T) {
	t.Serial("the program under test holds the inspector's driving hooks of the process while it runs")
	exclusive(t)

	strokes := []string{"a", "b", "c", "left", "left", "backspace", "x", "y"}
	wantValue, wantCursor := sequential(t, strokes...)
	require.NotEmpty(t, wantValue)

	app, _ := runBurst(t, keys(strokes...)...)
	assert.Equal(t, wantValue, app.value)
	assert.Equal(t, strconv.Itoa(wantCursor), app.cursor)
}

// Each wheel notch of a burst scrolls on from where the one before it left the log.
func TestABurstOfWheelNotchesScrollsByAllOfThem(t *testing.T) {
	t.Serial("the program under test holds the inspector's driving hooks of the process while it runs")
	exclusive(t)

	log := landed(t, "", "log")
	notch := input.Mouse{X: log.Rect.X + 1, Y: log.Rect.Y + 1, Button: input.WheelDown, Action: input.Wheel}

	app, written := runBurst(t, notch, notch, notch, notch)
	assert.Equal(t, 4, app.offset, "four notches scroll four rows")
	assert.Equal(t, 2, strings.Count(written, syncOn), "the burst is one frame")
}

// A click after a key in the same burst lands on the layout the key produced.
func TestAClickAfterAKeyResolvesAgainstTheKeysLayout(t *testing.T) {
	t.Serial("the program under test holds the inspector's driving hooks of the process while it runs")
	exclusive(t)

	before, after := landed(t, "", "go"), landed(t, "x", "go")
	require.Greater(t, after.Rect.Y, before.Rect.Y, "the typed key pushes the button down")
	x, y := after.Rect.X+1, after.Rect.Y+after.Rect.H-1

	app, _ := runBurst(t,
		input.ParseKey("x"),
		input.Mouse{X: x, Y: y, Button: input.ButtonLeft, Action: input.Press},
		input.Mouse{X: x, Y: y, Button: input.ButtonLeft, Action: input.Release},
	)
	assert.Equal(t, []string{"go"}, app.activated, "the click lands on the button where the key moved it")
}

// Messages the app says changed nothing cost no layout, and an input after them costs none of its own.
func TestUnchangedMessagesInABurstCostNoLayout(t *testing.T) {
	t.Serial("the program under test holds the inspector's driving hooks of the process while it runs")
	exclusive(t)

	app, written := runBurst(t, burstQuiet{}, input.ParseKey("q"), burstQuiet{}, input.ParseKey("q"), input.ParseKey("z"))
	assert.Equal(t, "z", app.value)
	assert.Equal(t, int64(2), app.props.Load(), "the first frame and the burst's frame are the only layouts")
	assert.Equal(t, 2, strings.Count(written, syncOn))
}

// A burst the app says changed nothing paints no frame and lays nothing out.
func TestAnUnchangedBurstLaysNothingOut(t *testing.T) {
	t.Serial("the program under test holds the inspector's driving hooks of the process while it runs")
	exclusive(t)

	app, written := runBurst(t, burstQuiet{}, input.ParseKey("q"), burstQuiet{})
	assert.Equal(t, int64(1), app.props.Load(), "only the first frame laid the view out")
	assert.Equal(t, 1, strings.Count(written, syncOn))
}
