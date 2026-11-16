package widgets

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml/cell"
	"github.com/wow-look-at-my/tml/widget"
	"github.com/wow-look-at-my/tml/widget/widgettest"
)

func TestTableDrawsHeadersAndRows(t *testing.T) {
	table := build(t, "Table", map[string]string{
		"columns": "Name,Size",
		"rows":    "go.mod|1.2K,go.sum|48K",
	})

	out := text(table, 0, 0)
	for _, want := range []string{"Name", "Size", "go.mod", "1.2K", "go.sum", "48K"} {
		assert.Contains(t, out, want)
	}
	assert.Contains(t, out, "┌", "a table is bordered unless told otherwise")
}

// A bordered table rules every column and the header off, and pads no cell.
func TestABorderedTableRulesItsCells(t *testing.T) {
	table := build(t, "Table", map[string]string{"columns": "A,Bb", "rows": "1|2,333|4"})

	assert.Equal(t, []string{
		"┌───┬──┐",
		"│A  │Bb│",
		"├───┼──┤",
		"│1  │2 │",
		"│333│4 │",
		"└───┴──┘",
	}, rows(table, 0, 0))
}

// A borderless table keeps a blank line under its header, where the rule would be.
func TestABorderlessTableLeavesABlankLineUnderItsHeader(t *testing.T) {
	table := build(t, "Table", map[string]string{"columns": "A,B", "rows": "1|2", "border": "false"})

	assert.Equal(t, []string{"A B", "", "1 2"}, trimmed(rows(table, 0, 0)))
}

// A table offered more room than it needs shares the room out between its columns.
func TestTableSharesSpareRoomBetweenItsColumns(t *testing.T) {
	table := build(t, "Table", map[string]string{"columns": "A,B", "rows": "1|2", "border": "false", "rule": "false"})

	assert.Equal(t, []string{"A   B  ", "1   2  "}, rows(table, 7, 0))
}

func TestTableCanDropItsBorder(t *testing.T) {
	table := build(t, "Table", map[string]string{
		"columns": "Name",
		"rows":    "go.mod",
		"border":  "false",
	})

	assert.NotContains(t, text(table, 0, 0), "┌")
}

// Dropping the border drops the rules, not the gap between the columns: a pair of cells that fill their columns would
func TestABorderlessTableStillSeparatesItsColumns(t *testing.T) {
	table := build(t, "Table", map[string]string{
		"columns": "tests,result",
		"rows":    "12|ok",
		"border":  "false",
	})

	assert.NotContains(t, text(table, 0, 0), "testsresult")
}

// The separator is the host's choice, because the host is what joined the cells and its data may well contain a pipe.
func TestTableTakesACustomSeparator(t *testing.T) {
	table := build(t, "Table", map[string]string{
		"columns":   "A,B",
		"rows":      "one;two",
		"separator": ";",
	})

	out := text(table, 0, 0)
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
	drawn := rows(table, narrow, 0)
	assert.Equal(t, len(drawn), measured, "the measured height is the height it draws")
	assert.Contains(t, text(table, narrow, 0), "another", "both rows are drawn")
}

// wrap="false" keeps a row to a single line, which is what makes a row index a line index. A host that maps a click or
// a scroll offset onto a row needs that arithmetic to hold. It breaks once a long cell takes another line too.
func TestATableCanKeepEveryRowToOneLine(t *testing.T) {
	attrs := map[string]string{
		"columns": "path",
		"rows":    "/a/long/path/that/will/not/fit,/another/long/path/that/will/not/fit",
		"border":  "false",
	}
	wrapped := build(t, "Table", attrs)

	attrs["wrap"] = "false"
	cut := build(t, "Table", attrs)

	const narrow = 14
	assert.Greater(t, len(rows(wrapped, narrow, 0)), len(rows(cut, narrow, 0)))
	assert.Equal(t, []string{"path", "", "/a/long/path/…", "/another/long…"}, trimmed(rows(cut, narrow, 0)),
		"a header, the rule under it, and a line for each row, each cut cell saying it was cut")
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
	assert.Contains(t, text(table, wide, 0), "BRANCH", "a table that fits keeps every column")

	fits, _ := table.Measure(wide-1, 0)
	out := text(table, fits, 0)
	assert.NotContains(t, out, "BRANCH", "the first column in the list goes first")
	assert.Contains(t, out, "AGE", "a column goes only while the table does not fit")

	narrow, _ := table.Measure(20, 0)
	out = text(table, narrow, 0)
	assert.NotContains(t, out, "AGE")
	assert.Contains(t, out, "WORKFLOW", "a column the list does not name stays")
	assert.Equal(t, []string{"ID  STATE   WORKFLOW", "101 success CI"}, split(out),
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
	attrs := map[string]string{"columns": "A,B", "rows": "1|2", "border": "false"}
	ruled := build(t, "Table", attrs)
	attrs["rule"] = "false"
	bare := build(t, "Table", attrs)

	assert.Len(t, rows(ruled, 0, 0), 3)
	assert.Len(t, rows(bare, 0, 0), 2)
}

// spacing is the gap between a pair of columns. The last column carries none, so a row ends at its last cell.
func TestABorderlessTableTakesItsColumnSpacing(t *testing.T) {
	table := build(t, "Table", map[string]string{
		"columns": "A,B", "rows": "1|2", "border": "false", "rule": "false", "spacing": "3",
	})

	assert.Equal(t, []string{"A   B", "1   2"}, rows(table, 0, 0))

	_, err := tryBuild("Table", map[string]string{"columns": "A", "spacing": "-1"})
	require.Error(t, err)
}

// An emoji takes cells, and a column of them lines up with the column beside it.
func TestTableAlignsWideRunes(t *testing.T) {
	table := build(t, "Table", map[string]string{
		"columns": ",ID", "rows": "✅|1,x|22", "border": "false", "rule": "false", "spacing": "2",
	})

	assert.Equal(t, []string{"    ID", "✅  1", "x   22"}, trimmed(rows(table, 0, 0)))
	assert.Equal(t, uint8(2), widgettest.Draw(table, 0, 0).At(0, 1).Width, "the emoji owns a pair of cells")
}

func TestTableRejectsANonBooleanBorder(t *testing.T) {
	_, err := tryBuild("Table", map[string]string{"border": "yes"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected true or false")
}

// The marked row is drawn differently from the rest. That is what makes a table something to pick a row out of, rather
// than only something to read.
func TestTableMarksTheSelectedRow(t *testing.T) {
	attrs := map[string]string{"columns": "Name", "rows": "a,b,c", "border": "false"}
	plainTable := build(t, "Table", attrs)

	attrs["selected"] = "1"
	marked := build(t, "Table", attrs)

	assert.Equal(t, rows(plainTable, 10, 0), rows(marked, 10, 0),
		"marking a row decorates it rather than changing what it says")

	// The header and its blank line come first, so the row marked one is the fourth line.
	buf := widgettest.Draw(marked, 10, 0)
	for x := range buf.W {
		assert.True(t, buf.At(x, 3).Style.Has(cell.Reverse), "the marked row is one unbroken bar, column %d", x)
	}
	for _, y := range []int{0, 2, 4} {
		assert.False(t, buf.At(0, y).Style.Has(cell.Reverse), "line %d is not marked", y)
	}
	assert.False(t, buf.At(0, 3).Style.Has(cell.Bold), "bold waits for focus")
}

// A bordered table marks the cells of a row, and leaves the rules between them alone.
func TestABorderedTableMarksCellsNotRules(t *testing.T) {
	table := build(t, "Table", map[string]string{"columns": "A,B", "rows": "1|2", "selected": "0"})

	buf := widgettest.Draw(table, 0, 0)
	assert.True(t, buf.At(1, 3).Style.Has(cell.Reverse))
	assert.True(t, buf.At(3, 3).Style.Has(cell.Reverse))
	assert.False(t, buf.At(0, 3).Style.Has(cell.Reverse), "the left rule")
	assert.False(t, buf.At(2, 3).Style.Has(cell.Reverse), "the rule between the columns")
}

// A table nobody has selected a row of draws undecorated, headers included. A negative selection must not land on the
// header, which is the row before the first.
func TestAnUnselectedTableMarksNothing(t *testing.T) {
	unselected := build(t, "Table", map[string]string{"columns": "Name", "rows": "a,b", "border": "false"})

	buf := widgettest.Draw(unselected, 10, 0)
	for y := range buf.H {
		for x := range buf.W {
			assert.True(t, buf.At(x, y).Style.IsZero(), "cell %d,%d", x, y)
		}
	}
}

func TestDisabledTableIsFaintAndMarksNothing(t *testing.T) {
	table := build(t, "Table", map[string]string{"columns": "N", "rows": "a", "selected": "0", "disabled": "true", "border": "false"})

	buf := widgettest.Draw(table, 0, 0)
	assert.True(t, buf.At(0, 2).Style.Has(cell.Faint))
	assert.False(t, buf.At(0, 2).Style.Has(cell.Reverse))
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
	assert.False(t, widgettest.Draw(table, 10, 0).At(0, 2).Style.Has(cell.Bold))

	table.(widget.Stateful).SetState(widget.State{Focused: true})
	style := widgettest.Draw(table, 10, 0).At(0, 2).Style
	assert.True(t, style.Has(cell.Bold|cell.Reverse), "focus adds the bold to the mark")
}
