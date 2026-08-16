package tml_test

import (
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml"
	"github.com/wow-look-at-my/tml/inspect"
)

// inspectorView is a small real view: two ids, one inside the other, rendered
// through the same path a program uses.
const inspectorView = `<Root>
	<Stack id="app" width="20">
		<Text id="title">hello</Text>
		<Text id="body">world</Text>
	</Stack>
</Root>`

func loadInspectorView(t *testing.T) *tml.View {
	t.Helper()
	view, err := tml.Load(fstest.MapFS{"app.tml": &fstest.MapFile{Data: []byte(inspectorView)}},
		"app.tml", tml.Options{})
	require.NoError(t, err)
	return view
}

// The frame the inspector answers from is the frame Render painted, so an
// assertion is about what the program actually put on screen.
func TestInspectorAnswersFromTheFrameTheViewPainted(t *testing.T) {
	view := loadInspectorView(t)
	insp := tml.NewInspector(view)
	t.Cleanup(func() { require.NoError(t, insp.Close()) })

	_, ok := insp.Frame()
	require.False(t, ok, "nothing has been painted yet")

	_, err := view.Render(nil, 40, 10)
	require.NoError(t, err)

	frame, ok := insp.Frame()
	require.True(t, ok)
	assert.Equal(t, 40, frame.Width)
	assert.Equal(t, 10, frame.Height)
	assert.Equal(t, uint64(1), frame.Seq)
	require.NotNil(t, frame.Box)

	body := inspect.Find(frame.Box, "body")
	require.NotNil(t, body, "the view declares a body element")
	assert.Equal(t, "world", inspect.Describe(body, frame.State, inspect.Options{}).Text)
}

// A restyle is a real override: the next layout uses it, so the geometry the
// inspector reports back is the geometry the terminal got.
func TestRestyleChangesTheNextFramesLayout(t *testing.T) {
	view := loadInspectorView(t)
	insp := tml.NewInspector(view)
	t.Cleanup(func() { require.NoError(t, insp.Close()) })

	_, err := view.Render(nil, 40, 10)
	require.NoError(t, err)
	before, ok := insp.Frame()
	require.True(t, ok)
	assert.Equal(t, 20, inspect.Find(before.Box, "app").Screen.W)

	require.NoError(t, insp.Restyle("app", map[string]string{"width": "8"}))
	_, err = view.Render(nil, 40, 10)
	require.NoError(t, err)

	after, ok := insp.Frame()
	require.True(t, ok)
	assert.Equal(t, 8, inspect.Find(after.Box, "app").Screen.W,
		"the override replaced the width the document wrote")
	assert.Equal(t, uint64(2), after.Seq)

	require.NoError(t, insp.Reset())
	_, err = view.Render(nil, 40, 10)
	require.NoError(t, err)
	restored, ok := insp.Frame()
	require.True(t, ok)
	assert.Equal(t, 20, inspect.Find(restored.Box, "app").Screen.W, "reset gives the document back")
}

// A host that wires no input says so, rather than accepting a keystroke and
// dropping it.
func TestAnInspectorWithNoInputWiringRefusesToDrive(t *testing.T) {
	view := loadInspectorView(t)
	insp := tml.NewInspector(view)
	t.Cleanup(func() { require.NoError(t, insp.Close()) })

	assert.ErrorContains(t, insp.Key("enter"), "no key handler")
	assert.ErrorContains(t, insp.Click(1, 1), "no click handler")

	var got string
	insp.OnKey(func(key string) error { got = key; return nil })
	require.NoError(t, insp.Key("ctrl+c"))
	assert.Equal(t, "ctrl+c", got)
}

// OnFrame is what wakes a watcher: the hook fires on the paint, not on a
// timer.
func TestOnFrameFiresPerPaint(t *testing.T) {
	view := loadInspectorView(t)
	insp := tml.NewInspector(view)
	t.Cleanup(func() { require.NoError(t, insp.Close()) })

	painted := 0
	view.OnFrame(func() { painted++ })
	for range 3 {
		_, err := view.Render(nil, 40, 10)
		require.NoError(t, err)
	}
	assert.Equal(t, 3, painted)
}

// The socket is the CLI's whole contract, so it is exercised here against a
// real view rather than only against a hand-built frame.
func TestSocketServesARealViewsFrame(t *testing.T) {
	view := loadInspectorView(t)
	insp := tml.NewInspector(view)
	t.Cleanup(func() { require.NoError(t, insp.Close()) })
	_, err := view.Render(nil, 40, 10)
	require.NoError(t, err)

	path := filepath.Join(t.TempDir(), "app.sock")
	require.NoError(t, insp.ListenSocket(path))

	res := inspect.NewServer(insp).Handle(inspect.Request{Op: "query", ID: "title"})
	require.Empty(t, res.Error)
	require.NotNil(t, res.Element)
	assert.Equal(t, "hello", res.Element.Text)
}
