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

// A list of records has no spelling as name=value. A file of typed values draws the view a host would.
func TestPropsComeFromAFileOfTypedValues(t *testing.T) {
	drawn := func(args ...string) string {
		t.Helper()
		page, err := runCapture(t, append([]string{"../testdata/records/app.tml", "--width", "30", "--height", "6"}, args...)...)
		require.NoError(t, err)
		var held inspect.Capture
		require.NoError(t, json.Unmarshal([]byte(betweenTags(t, page)), &held))
		return held.Frame.Text
	}

	out := drawn("--prop", "@../testdata/records/props.json")
	assert.Contains(t, out, "from the file")
	assert.Contains(t, out, "alpha #one #two", "a record holds a list of its own")
	assert.Contains(t, out, "beta")
	assert.Contains(t, out, "second note")
	assert.Contains(t, out, "shown", "true is a flag, not the text true")

	// A value named after the file wins, so one file serves several renders.
	out = drawn("--prop", "@../testdata/records/props.json", "--prop", "title=from the flag")
	assert.Contains(t, out, "from the flag")
	assert.NotContains(t, out, "from the file")
}

func TestABadPropsFileSaysWhatIsWrongWithIt(t *testing.T) {
	dir := t.TempDir()
	for name, tc := range map[string]struct{ body, want string }{
		"array.json":  {`[1, 2]`, "expected a JSON object of properties"},
		"nested.json": {`{"rows": {"a": 1}}`, "rows: a property is text"},
		"mixed.json":  {`{"rows": [{"a": "b"}, "c"]}`, "this holds both"},
		"deep.json":   {`{"rows": [[1]]}`, "a list holds text or records"},
		"null.json":   {`{"rows": [{"a": null}]}`, "a: a property is text"},
	} {
		path := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(path, []byte(tc.body), 0o600))
		_, err := runCapture(t, "../testdata/records/app.tml", "--prop", "@"+path)
		require.Error(t, err, name)
		assert.Contains(t, err.Error(), tc.want, name)
	}

	_, err := runCapture(t, "../testdata/records/app.tml", "--prop", "@"+filepath.Join(dir, "missing.json"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing.json")
}

// A capture of a document is the same bytes twice when the build names its moment. A committed capture can be
// checked against a fresh one.
func TestACaptureOfAFileIsReproducible(t *testing.T) {
	t.Setenv("SOURCE_DATE_EPOCH", "86400")
	first, err := runCapture(t, "../testdata/inspect/app.tml", "--width", "40", "--height", "8")
	require.NoError(t, err)
	second, err := runCapture(t, "../testdata/inspect/app.tml", "--width", "40", "--height", "8")
	require.NoError(t, err)

	assert.Equal(t, first, second)
	assert.Contains(t, first, "1970-01-02T00:00:00Z")
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
