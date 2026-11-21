// unchanged_test.go covers Program.Unchanged.

package tml_test

import (
	"io"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml"
	"github.com/wow-look-at-my/tml/sema"
)

type (
	still struct{}
	moved struct{}
)

// quietApp says that a still message and the key "q" changed nothing. It
// keeps "q" and "k", and leaves every other key to the focus ring.
type quietApp struct {
	frames *atomic.Int64
	got    chan tml.Msg
}

func (a quietApp) Props() tml.Props {
	a.frames.Add(1)
	return tml.Props{"title": sema.StringValue("hello")}
}

func (a quietApp) Update(p *tml.Program, msg tml.Msg) bool {
	a.got <- msg
	switch msg := msg.(type) {
	case still:
		p.Unchanged()
	case tml.Key:
		switch msg.String() {
		case "q":
			p.Unchanged()
			return true
		case "k":
			return true
		case "r":
			// The app does not keep this key, so what it says here does not count.
			p.Unchanged()
		}
	}
	return false
}

// quiet is a running program with a quietApp, and the pipe its keys come from.
type quiet struct {
	app     quietApp
	program *tml.Program
	keys    *io.PipeWriter
}

func startQuiet(t *testing.T) *quiet {
	t.Helper()
	reader, writer := io.Pipe()
	t.Cleanup(func() { _ = writer.Close() })
	app := quietApp{frames: &atomic.Int64{}, got: make(chan tml.Msg, 64)}
	p, err := tml.NewProgram(loadDoc(t, programDoc), app, tml.ProgramOptions{
		Input: reader, Output: io.Discard, Width: programWidth, Height: programHeight,
	})
	require.NoError(t, err)
	runProgram(t, p)
	waitFor[tml.Resize](t, app.got)
	require.Eventually(t, func() bool { return app.frames.Load() > 0 }, 5*time.Second, time.Millisecond,
		"the program painted no first frame")
	return &quiet{app: app, program: p, keys: writer}
}

// paints runs a step and waits for the frame that follows it.
func (q *quiet) paints(t *testing.T, step func(), what string) {
	t.Helper()
	before := q.app.frames.Load()
	step()
	require.Eventually(t, func() bool { return q.app.frames.Load() > before }, 5*time.Second, time.Millisecond,
		"%s painted no frame", what)
}

// rests runs a step and reports that no frame follows it.
func (q *quiet) rests(t *testing.T, step func(), what string) {
	t.Helper()
	before := q.app.frames.Load()
	step()
	assert.Never(t, func() bool { return q.app.frames.Load() != before }, 100*time.Millisecond, 5*time.Millisecond,
		"%s painted a frame", what)
}

func (q *quiet) key(t *testing.T, name string) func() {
	return func() {
		t.Helper()
		_, err := q.keys.Write([]byte(name))
		require.NoError(t, err)
		waitFor[tml.Key](t, q.app.got)
	}
}

func (q *quiet) send(t *testing.T, msg tml.Msg) func() {
	return func() {
		t.Helper()
		q.program.Send(msg)
		waitFor[tml.Msg](t, q.app.got)
	}
}

// A message the app calls unchanged paints no frame. The next message paints again.
func TestAMessageThatChangedNothingPaintsNoFrame(t *testing.T) {
	t.Serial("the program under test holds the inspector's driving hooks of the process while it runs")
	exclusive(t)
	q := startQuiet(t)

	q.rests(t, q.send(t, still{}), "a message the app called unchanged")
	q.paints(t, q.send(t, moved{}), "a message the app said nothing about")
	q.rests(t, q.send(t, still{}), "Unchanged speaks for one call, and a second unchanged message")
}

// A key the app keeps and calls unchanged paints no frame. A key that goes on to the focus ring always paints.
func TestAnUnchangedKeyPaintsNoFrameUnlessTheRingGetsIt(t *testing.T) {
	t.Serial("the program under test holds the inspector's driving hooks of the process while it reads its pipe")
	exclusive(t)
	q := startQuiet(t)

	q.rests(t, q.key(t, "q"), "a key the app kept and called unchanged")
	q.paints(t, q.key(t, "k"), "a key the app kept")
	q.paints(t, q.key(t, "r"), "a key the app called unchanged and left to the ring")
	q.paints(t, q.key(t, "x"), "a key for the ring")
}
