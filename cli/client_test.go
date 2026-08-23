package cli

import (
	"encoding/json"
	"net"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml/inspect"
)

func TestDialNamesTheUnresolvedSocket(t *testing.T) {
	_, err := dial("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no socket resolved")
}

func TestAskReadsOneJSONResponse(t *testing.T) {
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
		_ = json.NewEncoder(conn).Encode(inspect.Response{IDs: []string{"composer", "status"}})
	}()

	res, err := ask(path, inspect.Request{Op: "ids"})
	require.NoError(t, err)
	assert.Equal(t, []string{"composer", "status"}, res.IDs)
}

func TestPrintFieldWritesTheBareValue(t *testing.T) {
	var b strings.Builder
	el := inspect.Element{Text: "hello", Rect: inspect.Rect{X: 3}, Focus: true, Action: "send", Element: "Button"}
	require.NoError(t, printField(&b, el, "text"))
	assert.Equal(t, "hello\n", b.String())

	err := printField(&b, el, "nope")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown field")
}

func TestPrintNodeShowsGeometryAndID(t *testing.T) {
	var b strings.Builder
	err := printNode(&b, inspect.Node{
		Element: "Stack",
		ID:      "app",
		Rect:    inspect.Rect{W: 80, H: 24},
		Children: []inspect.Node{{
			Element: "Text",
			ID:      "status",
			Rect:    inspect.Rect{X: 2, Y: 3, W: 10, H: 1},
			Text:    "ready",
		}},
	}, "", false)
	require.NoError(t, err)
	assert.Contains(t, b.String(), "<Stack> #app")
	assert.Contains(t, b.String(), "<Text> #status")
	assert.Contains(t, b.String(), "ready")
}
