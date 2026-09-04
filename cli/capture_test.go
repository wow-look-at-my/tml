package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml/inspect"
)

// runCapture executes a fresh capture command and returns its stdout.
func runCapture(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	cmd := newCaptureCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SilenceUsage = true
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

// A capture of a file is the whole page: it opens with nothing running and nothing to fetch.
func TestCaptureOfAFileIsSelfContained(t *testing.T) {
	page, err := runCapture(t, "../testdata/inspect/app.tml",
		"--width", "40", "--height", "8", "--prop", "title=STATUS", "--prop", "rows=one,two")
	require.NoError(t, err)

	assert.Contains(t, page, "<!doctype html>")
	assert.NotContains(t, page, `src="inspector.js"`)
	assert.NotContains(t, page, `href="inspector.css"`)

	var held inspect.Capture
	require.NoError(t, json.Unmarshal([]byte(betweenTags(t, page)), &held))
	assert.Equal(t, 40, held.Frame.Width)
	assert.Equal(t, 8, held.Frame.Height)
	assert.Contains(t, held.Frame.Text, "STATUS", "the capture holds the frame the view painted")

	ids := make([]string, 0, len(held.Elements))
	for _, el := range held.Elements {
		ids = append(ids, el.ID)
	}
	assert.Equal(t, []string{"header", "body", "footer", "draft"}, ids)
}

// The geometry in a capture is the geometry inspect reports, because both read the same laid-out frame.
func TestCaptureAgreesWithInspect(t *testing.T) {
	page, err := runCapture(t, "../testdata/inspect/app.tml",
		"--width", "40", "--height", "8", "--prop", "title=STATUS", "--prop", "rows=one,two,three")
	require.NoError(t, err)
	var held inspect.Capture
	require.NoError(t, json.Unmarshal([]byte(betweenTags(t, page)), &held))

	out, err := run(t, "../testdata/inspect/app.tml",
		"--width", "40", "--height", "8", "--prop", "title=STATUS", "--prop", "rows=one,two,three", "--id", "header")
	require.NoError(t, err)
	var el element
	require.NoError(t, json.Unmarshal([]byte(out), &el))

	captured := held.Elements[0]
	require.Equal(t, "header", captured.ID)
	assert.Equal(t, el.Rect.X, captured.Rect.X)
	assert.Equal(t, el.Rect.Y, captured.Rect.Y)
	assert.Equal(t, el.Rect.W, captured.Rect.W)
	assert.Equal(t, el.Rect.H, captured.Rect.H)
}

func TestCaptureWritesTheFileItNames(t *testing.T) {
	path := filepath.Join(t.TempDir(), "frame.html")

	out, err := runCapture(t, "../testdata/inspect/app.tml", "--prop", "rows=one", "-o", path)
	require.NoError(t, err)
	assert.Equal(t, path, strings.TrimSpace(out), "the path is what a caller opens next")

	page, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(page), `id="capture"`)
}

// With no file, a capture needs a program, and says so rather than writing an empty page.
func TestCaptureWithNoProgramSaysSo(t *testing.T) {
	t.Serial()
	t.Setenv("TML_INSPECT_SOCKET", "")
	t.Setenv("TML_INSPECT_DIR", t.TempDir())

	_, err := runCapture(t)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no TML program is running")
}

func betweenTags(t *testing.T, page string) string {
	t.Helper()
	_, rest, ok := strings.Cut(page, `<script type="application/json" id="capture">`)
	require.True(t, ok, "the page holds the capture")
	held, _, ok := strings.Cut(rest, "</script>")
	require.True(t, ok, "the element is closed")
	return held
}
