package cli

import (
	"testing"
	"testing/fstest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml"
	"github.com/wow-look-at-my/tml/inspect"
	"github.com/wow-look-at-my/tml/sema"
)

const awaitView = `<?xml version="1.1" encoding="UTF-8"?>
<Component xmlns="urn:tml:v1" name="Awaited">
	<Property name="body" type="string" default="waiting"/>
	<Template><Stack id="app" width="20"><Text id="status">{body}</Text></Stack></Template>
</Component>`

// renderAwait paints a single frame of awaitView through the same path a program uses, so the inspector answers about a
// real frame. The inspector belongs to the process, so this takes the serial barrier: any test that paints at the same
// time replaces the frame this test waits on.
func renderAwait(t *testing.T, body string) *tml.View {
	t.Helper()
	t.Serial()
	view, err := tml.Load(fstest.MapFS{"app.tml": &fstest.MapFile{Data: []byte(awaitView)}},
		"app.tml", tml.Options{})
	require.NoError(t, err)
	_, err = view.Render(tml.Props{"body": sema.StringValue(body)}, 20, 3)
	require.NoError(t, err)
	return view
}

// An await returns as soon as the element draws the pattern, and it returns the element rather than a bare bool, so
func TestAwaitFieldReturnsWhenTheElementDrawsIt(t *testing.T) {
	view := renderAwait(t, "waiting")

	go func() {
		time.Sleep(150 * time.Millisecond)
		_, _ = view.Render(tml.Props{"body": sema.StringValue("turn 1")}, 20, 3)
	}()

	el, err := awaitField("status", false, "text", "turn [0-9]+", false, 10*time.Second)
	require.NoError(t, err)
	assert.Contains(t, el.Text, "turn 1")
}

// --await-gone is the same question inverted, for something that has to leave the screen.
func TestAwaitFieldGoneReturnsWhenTheTextLeaves(t *testing.T) {
	view := renderAwait(t, "working")

	go func() {
		time.Sleep(150 * time.Millisecond)
		_, _ = view.Render(tml.Props{"body": sema.StringValue("done")}, 20, 3)
	}()

	el, err := awaitField("status", false, "text", "working", true, 10*time.Second)
	require.NoError(t, err)
	assert.NotContains(t, el.Text, "working")
}

// A timeout names what the element actually drew. "expected output to contain X" says nothing about what was on
func TestAwaitFieldTimeoutReportsWhatItDrew(t *testing.T) {
	renderAwait(t, "still here")

	_, err := awaitField("status", false, "text", "never appears", false, 200*time.Millisecond)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "never matched")
	assert.Contains(t, err.Error(), "still here")
}

// An id the frame does not declare is not a match that has not happened yet, so it fails with the timeout and says the
func TestAwaitFieldReportsAnIDTheFrameDoesNotDeclare(t *testing.T) {
	renderAwait(t, "here")

	_, err := awaitField("no-such-id", false, "text", "anything", false, 200*time.Millisecond)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no-such-id")
}

// A bad pattern is the caller's mistake, and it is reported before anything waits: a regular expression that cannot
func TestAwaitFieldRejectsAPatternThatCannotCompile(t *testing.T) {
	_, err := awaitField("status", false, "text", "(unclosed", false, time.Second)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a regular expression")
}

func TestFieldValueNamesTheFieldsItKnows(t *testing.T) {
	el := inspect.Element{
		ID: "status", Text: "hello", Lines: []string{"hello", "there"},
		Rect: inspect.Rect{X: 1, Y: 2, W: 3, H: 4}, Focus: true,
		Action: "submit", Element: "Text",
	}
	for field, want := range map[string]string{
		"":        "hello",
		"text":    "hello",
		"lines":   "hello\nthere",
		"x":       "1",
		"y":       "2",
		"w":       "3",
		"h":       "4",
		"focus":   "true",
		"action":  "submit",
		"element": "Text",
	} {
		got, err := fieldValue(el, field)
		require.NoError(t, err, "field %q", field)
		assert.Equal(t, want, got, "field %q", field)
	}

	_, err := fieldValue(el, "colour")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown field")
}

// The width is the only a terminal draws in, so a box rule measures its cells rather than its bytes and a wide glyph
func TestWidestLineMeasuresDisplayCells(t *testing.T) {
	assert.Equal(t, 5, widestLine("abc\nabcde\nab"))
	assert.Equal(t, 4, widestLine("────"))
	assert.Equal(t, 4, widestLine("日本"))
	assert.Equal(t, 0, widestLine(""))
}
