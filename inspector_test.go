package tml_test

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml"
	"github.com/wow-look-at-my/tml/inspect"
)

// inspectorView is a small real view: two ids, one inside the other, rendered
// through the same path a program uses.
const inspectorView = `<?xml version="1.1" encoding="UTF-8"?>
<Component xmlns="urn:tml:v1" name="Inspected">
	<Template>
		<Stack id="app" orientation="vertical" width="20">
			<Text id="title">hello</Text>
			<Text id="body">world</Text>
		</Stack>
	</Template>
</Component>`

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
	tml.ResetInspection()
	view := loadInspectorView(t)
	insp := tml.Inspect()

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
// inspector reports back is the geometry the terminal got. Restyle drives the
// repaint itself, which is what makes an edit visible on an idle program.
func TestRestyleChangesTheNextFramesLayout(t *testing.T) {
	tml.ResetInspection()
	view := loadInspectorView(t)
	insp := tml.Inspect()

	insp.OnRepaint(func() error {
		_, err := view.Render(nil, 40, 10)
		return err
	})

	_, err := view.Render(nil, 40, 10)
	require.NoError(t, err)
	before, ok := insp.Frame()
	require.True(t, ok)
	assert.Equal(t, 20, inspect.Find(before.Box, "app").Screen.W)

	require.NoError(t, insp.Restyle("app", map[string]string{"width": "8"}))

	after, ok := insp.Frame()
	require.True(t, ok)
	assert.Equal(t, 8, inspect.Find(after.Box, "app").Screen.W,
		"the override replaced the width the document wrote")
	assert.Greater(t, after.Seq, before.Seq, "the restyle painted, rather than waiting for something else to")

	require.NoError(t, insp.Reset())
	restored, ok := insp.Frame()
	require.True(t, ok)
	assert.Equal(t, 20, inspect.Find(restored.Box, "app").Screen.W, "reset gives the document back")
}

// A host that never wired a repaint says so, rather than reporting an override
// that stays off the screen.
func TestRestyleWithNoRepaintWiringSaysSo(t *testing.T) {
	tml.ResetInspection()
	view := loadInspectorView(t)
	insp := tml.Inspect()

	_, err := view.Render(nil, 40, 10)
	require.NoError(t, err)

	assert.ErrorContains(t, insp.Restyle("app", map[string]string{"width": "8"}), "tml.NewProgram")
	assert.ErrorContains(t, insp.Reset(), "tml.NewProgram")
}

// A host that wires no input says so, rather than accepting a keystroke and
// dropping it.
func TestAnInspectorWithNoInputWiringRefusesToDrive(t *testing.T) {
	tml.ResetInspection()
	loadInspectorView(t)
	insp := tml.Inspect()

	assert.ErrorContains(t, insp.Key("enter"), "tml.NewProgram")
	assert.ErrorContains(t, insp.Click(1, 1), "tml.NewProgram")

	var got string
	insp.OnKey(func(key string) error { got = key; return nil })
	require.NoError(t, insp.Key("ctrl+c"))
	assert.Equal(t, "ctrl+c", got)
}

// OnFrame is what wakes a watcher: the hook fires on the paint, not on a
// timer.
func TestOnFrameFiresPerPaint(t *testing.T) {
	tml.ResetInspection()
	view := loadInspectorView(t)

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
	tml.ResetInspection()
	view := loadInspectorView(t)
	insp := tml.Inspect()

	_, err := view.Render(nil, 40, 10)
	require.NoError(t, err)

	path := filepath.Join(t.TempDir(), "app.sock")
	require.NoError(t, insp.ListenSocket(path))

	res := inspect.NewServer(insp).Handle(inspect.Request{Op: "query", ID: "title"})
	require.Empty(t, res.Error)
	require.NotNil(t, res.Element)
	assert.Equal(t, "hello", res.Element.Text)
}

// A host that recompiles its document renders through a new View, and the only
// thing it does about that is load. The inspector follows on its own, and keeps
// numbering frames upward so a caller waiting for a newer one is not answered
// by the new View's first paint.
func TestLoadingAgainMovesTheInspectorToTheNewView(t *testing.T) {
	tml.ResetInspection()
	first := loadInspectorView(t)
	insp := tml.Inspect()

	_, err := first.Render(nil, 40, 10)
	require.NoError(t, err)
	before, ok := insp.Frame()
	require.True(t, ok)

	second := loadInspectorView(t)

	_, err = second.Render(nil, 60, 12)
	require.NoError(t, err)

	after, ok := insp.Frame()
	require.True(t, ok, "the inspector answers from the view the host now paints")
	assert.Equal(t, 60, after.Width, "the frame is the new view's, not the old one's")
	assert.Greater(t, after.Seq, before.Seq, "frame numbers continue across the swap")

	// The old view stops recording, so a stray paint on it cannot come back as
	// the current frame.
	_, err = first.Render(nil, 11, 11)
	require.NoError(t, err)
	still, ok := insp.Frame()
	require.True(t, ok)
	assert.Equal(t, 60, still.Width, "the detached view no longer feeds the inspector")
}

// Overrides are the inspector's, not the view's, so they survive a reload and
// the new view lays them out.
func TestOverridesSurviveAReload(t *testing.T) {
	tml.ResetInspection()
	loadInspectorView(t)
	insp := tml.Inspect()

	second := loadInspectorView(t)
	insp.OnRepaint(func() error {
		_, err := second.Render(nil, 40, 10)
		return err
	})

	require.NoError(t, insp.Restyle("title", map[string]string{"width": "7"}))

	frame, ok := insp.Frame()
	require.True(t, ok)
	assert.Equal(t, 7, inspect.Find(frame.Box, "title").Screen.W,
		"the new view lays out the override the old one was given")
}

// The socket is Load's doing, not a host's. Nothing below wires an inspector,
// serves anything, or knows one exists: it sets the variable and loads.
func TestLoadServesTheSocketWithNoHostWiring(t *testing.T) {
	tml.ResetInspection()
	path := filepath.Join(t.TempDir(), "auto.sock")
	t.Setenv(tml.SocketEnv, path)

	view := loadInspectorView(t)
	require.NoError(t, tml.InspectError())
	_, err := view.Render(nil, 40, 10)
	require.NoError(t, err)

	conn, err := net.Dial("unix", path)
	require.NoError(t, err, "Load opened the socket named by %s", tml.SocketEnv)
	t.Cleanup(func() { assert.NoError(t, conn.Close()) })

	_, err = conn.Write([]byte(`{"op":"query","id":"title"}` + "\n"))
	require.NoError(t, err)
	var res inspect.Response
	require.NoError(t, json.NewDecoder(conn).Decode(&res))
	require.Empty(t, res.Error)
	require.NotNil(t, res.Element)
	assert.Equal(t, "hello", res.Element.Text)
}

// A view that cannot be served is not returned at all.
//
// Reporting it and handing back a working View would leave a program running
// that nothing can reach -- which is the state this whole mechanism exists to
// make unreachable. Load is where it has to fail, because Load is the last
// point at which the host has not started drawing.
func TestAViewThatCannotBeServedIsNotReturned(t *testing.T) {
	tml.ResetInspection()
	// A regular file where the socket's directory has to be. ListenSocket
	// creates a missing directory on purpose, so an absent path is servable
	// and this is what genuinely is not.
	blocker := filepath.Join(t.TempDir(), "not-a-dir")
	require.NoError(t, os.WriteFile(blocker, []byte("x"), 0o600))
	t.Setenv(tml.SocketEnv, filepath.Join(blocker, "x.sock"))

	loaded, err := tml.Load(fstest.MapFS{"app.tml": &fstest.MapFile{Data: []byte(inspectorView)}},
		"app.tml", tml.Options{})
	require.Error(t, err)
	assert.Nil(t, loaded)
	assert.ErrorContains(t, err, "serve the inspection protocol")
	assert.ErrorContains(t, tml.InspectError(), "serve the inspection protocol")
}

// The socket is not conditional on the environment: a program that sets nothing
// at all still serves, and the path it serves on is the one tml looks in.
func TestAProgramServesWithNothingSet(t *testing.T) {
	tml.ResetInspection()
	t.Setenv(tml.SocketEnv, "")

	loadInspectorView(t)
	require.NoError(t, tml.InspectError())

	path := tml.SocketPath()
	assert.Equal(t, tml.SocketDir(), filepath.Dir(path))
	conn, err := net.DialTimeout("unix", path, 2*time.Second)
	require.NoError(t, err, "nothing is listening on the path a program serves on by default")
	require.NoError(t, conn.Close())
}

// The directory carries the right to drive every program in it, so it is this
// user's and nobody else's.
func TestTheSocketDirectoryIsPrivateToThisUser(t *testing.T) {
	tml.ResetInspection()
	t.Setenv(tml.SocketEnv, "")

	loadInspectorView(t)
	info, err := os.Stat(tml.SocketDir())
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), info.Mode().Perm())
}
