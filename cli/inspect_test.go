package cli

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// run executes a fresh inspect command and returns its stdout and error, so a test asserts on what a caller would see
func run(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	cmd := newInspectCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SilenceUsage = true
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func TestInspectReportsOneElementByID(t *testing.T) {
	out, err := run(t, "../testdata/inspect/app.tml",
		"--width", "40", "--height", "8",
		"--prop", "title=STATUS", "--prop", "rows=one,two,three", "--id", "header")
	require.NoError(t, err)

	var el element
	require.NoError(t, json.Unmarshal([]byte(out), &el))
	assert.Equal(t, "header", el.ID)
	assert.Equal(t, "Text", el.Element)
	assert.Equal(t, rect{X: 0, Y: 0, W: 6, H: 1}, el.Rect)
	require.Len(t, el.Lines, 1)
	assert.Contains(t, el.Lines[0], "STATUS")
}

func TestInspectReportsWhereAScrollingRegionLanded(t *testing.T) {
	out, err := run(t, "../testdata/inspect/app.tml",
		"--width", "40", "--height", "6",
		"--prop", "rows=one,two,three,four,five,six", "--prop", "offset=1", "--id", "body")
	require.NoError(t, err)

	var el element
	require.NoError(t, json.Unmarshal([]byte(out), &el))
	assert.Equal(t, "Scrollbox", el.Element)
	assert.Equal(t, 1, el.Scroll.Y, "the offset the host asked for")
	assert.Positive(t, el.Scroll.MaxY, "six rows do not fit, so there is somewhere to scroll")
	assert.Contains(t, el.Lines[0], "two", "an offset of one starts at the second row")
}

func TestInspectListsEveryIDBearingElementInDocumentOrder(t *testing.T) {
	out, err := run(t, "../testdata/inspect/app.tml",
		"--width", "40", "--height", "8", "--prop", "rows=one")
	require.NoError(t, err)

	var all []element
	require.NoError(t, json.Unmarshal([]byte(out), &all))
	ids := make([]string, 0, len(all))
	for _, el := range all {
		ids = append(ids, el.ID)
	}
	assert.Equal(t, []string{"header", "body", "footer", "draft"}, ids)
}

func TestInspectFailsOnAnIDTheViewDoesNotDeclare(t *testing.T) {
	_, err := run(t, "../testdata/inspect/app.tml", "--id", "nope")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `no element has id "nope"`)
	assert.Contains(t, err.Error(), "body, draft, footer, header",
		"naming the ids that do exist is what makes the failure actionable")
}

func TestInspectStripsANSIUnlessAsked(t *testing.T) {
	plain, err := run(t, "../testdata/inspect/app.tml",
		"--width", "20", "--height", "4", "--prop", "title=hi", "--id", "header")
	require.NoError(t, err)
	assert.NotContains(t, plain, "\\u001b")

	withANSI, err := run(t, "../testdata/inspect/app.tml",
		"--width", "20", "--height", "4", "--prop", "title=hi", "--id", "header", "--ansi")
	require.NoError(t, err)
	assert.Contains(t, withANSI, "hi", "the text survives either way")
}
