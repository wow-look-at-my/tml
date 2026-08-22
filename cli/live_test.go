package cli

import (
	"bytes"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml/inspect"
)

// runCmd executes the shared cobra root with args and returns stdout. Flags
// live on that command, so leftover values from a previous test can leak —
// which is why these tests are not the CLI's real coverage.
func runCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), err
}

// serveInspect binds a unix socket that answers one inspect.Request with the
// given response, then closes. The path is what --socket wants.
func serveInspect(t *testing.T, handle func(inspect.Request) inspect.Response) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "inspect.sock")
	ln, err := net.Listen("unix", path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		var req inspect.Request
		if json.NewDecoder(conn).Decode(&req) != nil {
			return
		}
		_ = json.NewEncoder(conn).Encode(handle(req))
	}()
	return path
}

func TestRootListsFileAndLiveCommands(t *testing.T) {
	out, err := runCmd(t, "--help")
	require.NoError(t, err)
	for _, name := range []string{
		"check", "tree", "render", "inspect", "query", "elements",
		"ids", "at", "frame", "input", "restyle", "serve",
	} {
		assert.Contains(t, out, name)
	}
	assert.NotContains(t, out, "tml-test")
	assert.Contains(t, out, "Usage:")
	assert.Contains(t, out, "tml")
}

func TestLiveCommandsFailWhenNoSocketIsGiven(t *testing.T) {
	t.Setenv("TML_INSPECT_SOCKET", "")
	_, err := runCmd(t, "ids")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--socket")
	assert.Contains(t, err.Error(), "TML_INSPECT_SOCKET")

	t.Setenv("TML_INSPECT_SOCKET", "")
	_, err = runCmd(t, "query", "--id", "composer")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--socket")
	assert.Contains(t, err.Error(), "TML_INSPECT_SOCKET")
}

func TestIDsPrintsWhatTheSocketAnswers(t *testing.T) {
	path := serveInspect(t, func(inspect.Request) inspect.Response {
		return inspect.Response{IDs: []string{"composer", "status"}}
	})
	out, err := runCmd(t, "ids", "--socket", path)
	require.NoError(t, err)
	assert.Contains(t, out, "composer")
	assert.Contains(t, out, "status")
}

func TestQueryPrintsOneFieldFromTheSocket(t *testing.T) {
	path := serveInspect(t, func(inspect.Request) inspect.Response {
		return inspect.Response{Element: &inspect.Element{ID: "prompt", Text: "ask for a change"}}
	})
	out, err := runCmd(t, "query", "--socket", path, "--id", "prompt", "--field", "text")
	require.NoError(t, err)
	assert.Contains(t, out, "ask for a change")
}

func TestQueryRequiresID(t *testing.T) {
	_, err := runCmd(t, "query")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--id")
}

func TestTreeWithAFilePrintsTheExpandedDocument(t *testing.T) {
	out, err := runCmd(t, "tree", "../testdata/inspect/app.tml", "--prop", "title=STATUS")
	require.NoError(t, err)
	assert.Contains(t, out, "header")
	assert.Contains(t, out, "STATUS")
}

func TestTreeWithoutAFilePrintsTheLiveFrame(t *testing.T) {
	path := serveInspect(t, func(inspect.Request) inspect.Response {
		return inspect.Response{Tree: &inspect.Node{
			Element: "Stack",
			ID:      "app",
			Rect:    inspect.Rect{W: 80, H: 24},
			Children: []inspect.Node{{
				Element: "Text",
				ID:      "status",
				Rect:    inspect.Rect{X: 2, Y: 3, W: 10, H: 1},
				Text:    "ready",
			}},
		}}
	})
	out, err := runCmd(t, "tree", "--socket", path)
	require.NoError(t, err)
	assert.Contains(t, out, "<Stack> #app")
	assert.Contains(t, out, "<Text> #status")
	assert.Contains(t, out, "ready")
}

func TestAtReportsTheHitFromTheSocket(t *testing.T) {
	path := serveInspect(t, func(inspect.Request) inspect.Response {
		return inspect.Response{Hit: "composer", Found: true}
	})
	out, err := runCmd(t, "at", "--socket", path, "--x", "2", "--y", "3")
	require.NoError(t, err)
	assert.Contains(t, out, "composer")
}

func TestInputSendsTheKeyTheSocketAskedFor(t *testing.T) {
	var got inspect.Request
	path := serveInspect(t, func(req inspect.Request) inspect.Response {
		got = req
		return inspect.Response{OK: true}
	})
	out, err := runCmd(t, "input", "--socket", path, "--key", "enter")
	require.NoError(t, err)
	assert.Contains(t, out, "ok")
	assert.Equal(t, "key", got.Op)
	assert.Equal(t, "enter", got.Key)
}

func TestElementsAndFramePrintJSONFromTheSocket(t *testing.T) {
	path := serveInspect(t, func(inspect.Request) inspect.Response {
		return inspect.Response{Elements: []inspect.Element{{ID: "header", Element: "Text"}}}
	})
	out, err := runCmd(t, "elements", "--socket", path)
	require.NoError(t, err)
	assert.Contains(t, out, `"id": "header"`)

	path = serveInspect(t, func(inspect.Request) inspect.Response {
		return inspect.Response{Frame: &inspect.FrameInfo{Seq: 3, Width: 80, Height: 24, Text: "ready"}}
	})
	out, err = runCmd(t, "frame", "--socket", path)
	require.NoError(t, err)
	assert.Contains(t, out, `"seq": 3`)
	assert.Contains(t, out, "ready")
}

func TestRestyleForwardsAttributesAndClearResets(t *testing.T) {
	var got inspect.Request
	path := serveInspect(t, func(req inspect.Request) inspect.Response {
		got = req
		return inspect.Response{OK: true}
	})
	out, err := runCmd(t, "restyle", "--socket", path, "--id", "header", "--set", "foreground=red")
	require.NoError(t, err)
	assert.Contains(t, out, "ok")
	assert.Equal(t, "restyle", got.Op)
	assert.Equal(t, "header", got.ID)
	assert.Equal(t, "red", got.Attrs["foreground"])

	path = serveInspect(t, func(req inspect.Request) inspect.Response {
		got = req
		return inspect.Response{OK: true}
	})
	out, err = runCmd(t, "restyle", "--socket", path, "--clear")
	require.NoError(t, err)
	assert.Contains(t, out, "overrides dropped")
	assert.Equal(t, "reset", got.Op)
}

func init() {
	// Live commands must not inherit a leftover socket from the process
	// environment; tests that want one pass --socket.
	_ = os.Unsetenv("TML_INSPECT_SOCKET")
}
