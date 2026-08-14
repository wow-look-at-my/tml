package widgets

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml/widget"
)

// plain drops the styling, for a test about what the text says rather than about
// how it is decorated.
func plain(s string) []string {
	return strings.Split(ansi.Strip(s), "\n")
}

func TestListMarksTheSelectedItem(t *testing.T) {
	l := build(t, "List", map[string]string{"items": "one,two,three", "selected": "1"})

	assert.Equal(t, []string{"  one  ", "> two  ", "  three"}, plain(l.Render(7, 3)))
}

// The gutter is kept on every row, or the text shifts sideways as the selection
// moves and the whole list appears to jitter.
func TestListKeepsItsGutterOnEveryRow(t *testing.T) {
	l := build(t, "List", map[string]string{"items": "a,b"})

	assert.Equal(t, []string{"  a", "  b"}, split(l.Render(3, 2)))
}

func TestListMeasuresItsWidestItem(t *testing.T) {
	l := build(t, "List", map[string]string{"items": "a,bbbb,cc"})

	w, h := l.Measure(0, 0)
	assert.Equal(t, 6, w, "four cells plus the cursor gutter")
	assert.Equal(t, 3, h)
}

func TestListTakesACustomCursor(t *testing.T) {
	l := build(t, "List", map[string]string{"items": "a,b", "selected": "0", "cursor": "*"})

	assert.Equal(t, "* a", plain(l.Render(3, 2))[0])
}

// An empty list has nothing to land on, so tab passes over it.
func TestEmptyListRefusesFocus(t *testing.T) {
	assert.False(t, build(t, "List", nil).(widget.Focusable).AcceptsFocus())
	assert.True(t, build(t, "List", map[string]string{"items": "a"}).(widget.Focusable).AcceptsFocus())
	assert.False(t, build(t, "List", map[string]string{"items": "a", "disabled": "true"}).(widget.Focusable).AcceptsFocus())
}

func TestFocusedListHighlightsItsSelection(t *testing.T) {
	l := build(t, "List", map[string]string{"items": "a,b", "selected": "0"})
	resting := l.Render(5, 2)

	l.(widget.Stateful).SetState(widget.State{Focused: true})
	assert.NotEqual(t, resting, l.Render(5, 2))
}

func TestTableDrawsHeadersAndRows(t *testing.T) {
	table := build(t, "Table", map[string]string{
		"columns": "Name,Size",
		"rows":    "go.mod|1.2K,go.sum|48K",
	})

	out := table.Render(0, 0)
	for _, want := range []string{"Name", "Size", "go.mod", "1.2K", "go.sum", "48K"} {
		assert.Contains(t, out, want)
	}
	assert.Contains(t, out, "┌", "a table is bordered unless told otherwise")
}

func TestTableCanDropItsBorder(t *testing.T) {
	table := build(t, "Table", map[string]string{
		"columns": "Name",
		"rows":    "go.mod",
		"border":  "false",
	})

	assert.NotContains(t, table.Render(0, 0), "┌")
}

// Dropping the border drops the rules, not the gap between the columns: two
// cells that fill their columns would otherwise run together into one word.
func TestABorderlessTableStillSeparatesItsColumns(t *testing.T) {
	table := build(t, "Table", map[string]string{
		"columns": "tests,result",
		"rows":    "12|ok",
		"border":  "false",
	})

	assert.NotContains(t, table.Render(0, 0), "testsresult")
}

// The separator is the host's choice, because the host is what joined the cells
// and its data may well contain a pipe.
func TestTableTakesACustomSeparator(t *testing.T) {
	table := build(t, "Table", map[string]string{
		"columns":   "A,B",
		"rows":      "one;two",
		"separator": ";",
	})

	out := table.Render(0, 0)
	assert.Contains(t, out, "one")
	assert.Contains(t, out, "two")
}

func TestTableMeasuresWhatItDraws(t *testing.T) {
	table := build(t, "Table", map[string]string{"columns": "Name,Size", "rows": "go.mod|1.2K"})

	w, h := table.Measure(0, 0)
	assert.Positive(t, w)
	assert.Positive(t, h)

	narrow, _ := table.Measure(8, 0)
	assert.Equal(t, 8, narrow, "a table never asks for more than it was offered")
}

func TestTableRejectsANonBooleanBorder(t *testing.T) {
	_, err := tryBuild("Table", map[string]string{"border": "yes"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected true or false")
}

// A row wider than the space is cut, not wrapped: wrapping would push every row
// below it down a line, and the list's geometry would stop matching the screen.
func TestListCutsRowsTooWideForIt(t *testing.T) {
	l := build(t, "List", map[string]string{"items": "cmd/report.go,report/table.go", "selected": "1"})

	assert.Equal(t, []string{"  cmd/repor…", "> report/ta…"}, plain(l.Render(12, 2)))
}
