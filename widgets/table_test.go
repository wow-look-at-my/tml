package widgets

import (
	"strings"
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
// disappear off the bottom of a table. That table had room for them.
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

// wrap="false" keeps a row to a single line, which is what makes a row index a line index. A host that maps a click or
// a scroll offset onto a row needs that arithmetic to hold. It breaks once a long cell takes another line too.
func TestATableCanKeepEveryRowToOneLine(t *testing.T) {
	rows := map[string]string{
		"columns": "path",
		"rows":    "/a/long/path/that/will/not/fit,/another/long/path/that/will/not/fit",
		"border":  "false",
	}
	wrapped := build(t, "Table", rows)

	rows["wrap"] = "false"
	cut := build(t, "Table", rows)

	const narrow = 14
	assert.Greater(t, len(split(wrapped.Render(narrow, 0))), len(split(cut.Render(narrow, 0))))
	assert.Len(t, split(cut.Render(narrow, 0)), 4, "a header, the rule under it, and a line for each row")
	assert.Contains(t, cut.Render(narrow, 0), "…", "a cut cell says it was cut")
}

// A listing that does not fit drops whole columns in the order collapse names, rather than wrapping every cell.
func TestTableCollapsesColumnsInOrderWhenNarrow(t *testing.T) {
	table := build(t, "Table", map[string]string{
		"columns":  "ID,STATE,WORKFLOW,BRANCH,AGE",
		"rows":     "101|success|CI|master|2m ago",
		"border":   "false",
		"rule":     "false",
		"collapse": "3,4",
	})

	wide, _ := table.Measure(0, 0)
	assert.Contains(t, ansi.Strip(table.Render(wide, 0)), "BRANCH", "a table that fits keeps every column")

	fits, _ := table.Measure(wide-1, 0)
	out := ansi.Strip(table.Render(fits, 0))
	assert.NotContains(t, out, "BRANCH", "the first column in the list goes first")
	assert.Contains(t, out, "AGE", "a column goes only while the table does not fit")

	narrow, _ := table.Measure(20, 0)
	out = ansi.Strip(table.Render(narrow, 0))
	assert.NotContains(t, out, "AGE")
	assert.Contains(t, out, "WORKFLOW", "a column the list does not name stays")
	assert.Equal(t, []string{"ID  STATE   WORKFLOW", "101 success CI"}, trimmed(split(out)),
		"the row loses the same cells as the header")
}

func TestTableRejectsACollapseColumnItDoesNotHave(t *testing.T) {
	_, err := tryBuild("Table", map[string]string{"columns": "A,B", "collapse": "2"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "is not a column")

	_, err = tryBuild("Table", map[string]string{"columns": "A,B", "collapse": "1,1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "named twice")
}

// rule="false" makes the header one more line of the listing. A tool whose output an agent reads wants no blank line under it.
func TestABorderlessTableCanDropTheRuleUnderItsHeader(t *testing.T) {
	rows := map[string]string{"columns": "A,B", "rows": "1|2", "border": "false"}
	ruled := build(t, "Table", rows)
	rows["rule"] = "false"
	bare := build(t, "Table", rows)

	assert.Len(t, split(ruled.Render(0, 0)), 3)
	assert.Len(t, split(bare.Render(0, 0)), 2)
}

// spacing is the gap between a pair of columns. The last column carries none, so a row ends at its last cell.
func TestABorderlessTableTakesItsColumnSpacing(t *testing.T) {
	table := build(t, "Table", map[string]string{
		"columns": "A,B", "rows": "1|2", "border": "false", "rule": "false", "spacing": "3",
	})

	assert.Equal(t, []string{"A   B", "1   2"}, split(ansi.Strip(table.Render(0, 0))))

	_, err := tryBuild("Table", map[string]string{"columns": "A", "spacing": "-1"})
	require.Error(t, err)
}

// An emoji takes cells, and a column of them lines up with the column beside it.
func TestTableAlignsWideRunes(t *testing.T) {
	table := build(t, "Table", map[string]string{
		"columns": ",ID", "rows": "✅|1,x|22", "border": "false", "rule": "false", "spacing": "2",
	})

	lines := trimmed(split(ansi.Strip(table.Render(0, 0))))
	assert.Equal(t, []string{"    ID", "✅  1", "x   22"}, lines)
}

// trimmed drops the blanks a column pads its last cell with.
func trimmed(lines []string) []string {
	out := make([]string, len(lines))
	for i, line := range lines {
		out[i] = strings.TrimRight(line, " ")
	}
	return out
}

func TestTableRejectsANonBooleanBorder(t *testing.T) {
	_, err := tryBuild("Table", map[string]string{"border": "yes"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected true or false")
}

// The marked row is drawn differently from the rest. That is what makes a table something to pick a row out of, rather
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
