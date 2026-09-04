package widgets

import (
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml/widget"
)

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

// Dropping the border drops the rules, not the gap between the columns: a pair of cells that fill their columns would
func TestABorderlessTableStillSeparatesItsColumns(t *testing.T) {
	table := build(t, "Table", map[string]string{
		"columns": "tests,result",
		"rows":    "12|ok",
		"border":  "false",
	})

	assert.NotContains(t, table.Render(0, 0), "testsresult")
}

// The separator is the host's choice, because the host is what joined the cells and its data may well contain a pipe.
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

// A cell too wide for its column wraps, so the height a table reports has to be the height at the width it was
// offered. Measured at its natural width instead, the wrapped lines are clipped off by whatever placed it, and rows
// disappear off the bottom of a table that had room for them.
func TestTableMeasuresTheHeightItWrapsTo(t *testing.T) {
	table := build(t, "Table", map[string]string{
		"columns": "path",
		"rows":    "/a/long/path/that/will/not/fit,/another/long/path/that/will/not/fit",
		"border":  "false",
	})

	const narrow = 14
	_, measured := table.Measure(narrow, 0)
	assert.Equal(t, len(split(table.Render(narrow, 0))), measured,
		"the measured height is the height it draws")
	assert.Contains(t, table.Render(narrow, 0), "another", "both rows are drawn")
}

func TestTableRejectsANonBooleanBorder(t *testing.T) {
	_, err := tryBuild("Table", map[string]string{"border": "yes"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected true or false")
}

// The marked row is drawn differently from the rest, which is what makes a table something to pick a row out of rather
// than only something to read.
func TestTableMarksTheSelectedRow(t *testing.T) {
	rows := map[string]string{"columns": "Name", "rows": "a,b,c", "border": "false"}
	plainTable := build(t, "Table", rows)

	rows["selected"] = "1"
	marked := build(t, "Table", rows)

	assert.NotEqual(t, plainTable.Render(10, 0), marked.Render(10, 0))
	assert.Equal(t, plain(plainTable.Render(10, 0)), plain(marked.Render(10, 0)),
		"marking a row decorates it rather than changing what it says")
}

// A table nobody has selected a row of draws undecorated, headers included. A lipgloss table gives its header row the
// same negative index an unselected table carries, so the header is what a careless mark lands on.
func TestAnUnselectedTableMarksNothing(t *testing.T) {
	unselected := build(t, "Table", map[string]string{"columns": "Name", "rows": "a,b", "border": "false"})

	assert.Equal(t, ansi.Strip(unselected.Render(10, 0)), unselected.Render(10, 0))
}

// A table with no rows has nothing to land on, so tab passes over it rather than stopping on an empty control.
func TestEmptyTableRefusesFocus(t *testing.T) {
	empty := map[string]string{"columns": "Name"}
	assert.False(t, build(t, "Table", empty).(widget.Focusable).AcceptsFocus())

	filled := map[string]string{"columns": "Name", "rows": "a"}
	assert.True(t, build(t, "Table", filled).(widget.Focusable).AcceptsFocus())

	filled["disabled"] = "true"
	assert.False(t, build(t, "Table", filled).(widget.Focusable).AcceptsFocus())
}

func TestFocusedTableHighlightsItsSelection(t *testing.T) {
	table := build(t, "Table", map[string]string{"columns": "Name", "rows": "a,b", "selected": "0", "border": "false"})
	resting := table.Render(10, 0)

	table.(widget.Stateful).SetState(widget.State{Focused: true})
	assert.NotEqual(t, resting, table.Render(10, 0))
}
