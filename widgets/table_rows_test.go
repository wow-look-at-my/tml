package widgets

import (
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml/cell"
	"github.com/wow-look-at-my/tml/sema"
	"github.com/wow-look-at-my/tml/widget"
)

// sourceTable builds a borderless table over rows the host keeps, with the attributes given.
func sourceTable(t *testing.T, rows *widget.Rows, pairs map[string]string) widget.Native {
	t.Helper()
	values := map[string]sema.Value{"rows": rows.Value(), "border": sema.StringValue("false")}
	order := []string{"rows", "border"}
	for name, raw := range pairs {
		values[name] = sema.StringValue(raw)
		order = append(order, name)
	}
	factory, ok := Library().Factory("Table")
	require.True(t, ok)
	native, err := factory.Build(widget.Context{Attrs: widget.NewAttrs("Table", values, order), Theme: theme(false)})
	require.NoError(t, err)
	return native
}

// numbered is n rows of a name and a number.
func numbered(n int) *widget.Rows {
	cells := make([][]string, n)
	for i := range cells {
		name := "r"
		if i == 0 {
			name = "a-much-longer-name"
		}
		cells[i] = []string{name, fmt.Sprint(i)}
	}
	return widget.NewRows(cells)
}

// scrolled draws a table the way a Scrollbox does. The table at its full height, moved up by offset, and seen through
// a viewport view lines tall.
func scrolled(native widget.Native, w, view, offset int) []string {
	_, full := native.Measure(w, 0)
	buf := cell.NewBuffer(w, view)
	native.Draw(buf.Canvas(nil).Sub(0, -offset, w, full))
	return trimmed(buf.Lines())
}

// Scrolled down, the table draws the rows under the viewport, keeps its header at the top, and keeps the column width
// the long row. This is above gave it.
func TestATableScrollsUnderAHeaderThatStays(t *testing.T) {
	table := sourceTable(t, numbered(100), map[string]string{"columns": "NAME,N", "wrap": "false"})

	top := scrolled(table, 30, 5, 0)
	assert.Equal(t, "NAME               N", top[0])
	assert.Equal(t, "a-much-longer-name 0", top[2])

	down := scrolled(table, 30, 5, 50)
	assert.Equal(t, top[0], down[0], "the header stayed, and in the same place")
	assert.Equal(t, []string{"r                  50", "r                  51", "r                  52"}, down[2:])
}

// A point names the row under it however far the table scrolled, and a point on the header still names a heading.
func TestATableNamesTheRowUnderAPointWhileScrolled(t *testing.T) {
	table := sourceTable(t, numbered(100), map[string]string{"columns": "NAME,N", "wrap": "false"})
	scrolled(table, 30, 5, 50)
	parted := table.(widget.Parted)

	assert.Equal(t, "header:0", parted.Part(0, 50), "the header drawn over the rows")
	assert.Equal(t, "row:50", parted.Part(0, 52))
	assert.Equal(t, "row:52", parted.Part(0, 54))
}

// A long table costs the rows on screen.
func TestATableDrawsOnlyTheRowsOnScreen(t *testing.T) {
	cost := func(n int) uint64 {
		table := sourceTable(t, numbered(n), map[string]string{"columns": "NAME,N", "wrap": "false"})
		scrolled(table, 40, 20, n/2)
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		scrolled(table, 40, 20, n/2)
		runtime.ReadMemStats(&after)
		return after.TotalAlloc - before.TotalAlloc
	}
	small, large := cost(200), cost(200000)
	assert.Less(t, large, 2*small, "200000 rows allocated %d bytes a frame, and 200 rows %d", large, small)
}

// A wrapped table scrolls by line, and a row that wraps keeps the rows after it in step.
func TestAWrappedTableScrollsByLine(t *testing.T) {
	rows := widget.NewRows([][]string{{"one two three"}, {"b"}, {"c"}})
	table := sourceTable(t, rows, map[string]string{"columns": "X", "rule": "false"})

	_, full := table.Measure(5, 0)
	assert.Equal(t, 1+3+1+1, full, "the header and five lines of rows")
	assert.Equal(t, []string{"X", "b"}, scrolled(table, 5, 2, 3))
	assert.Equal(t, "row:0", table.(widget.Parted).Part(0, 3))
	assert.Equal(t, "row:1", table.(widget.Parted).Part(0, 4))
}

// Spare room goes to the columns in proportion to their weights. A weight of zero takes none.
func TestSpareRoomGoesByWeight(t *testing.T) {
	table := build(t, "Table", map[string]string{
		"columns": "A,B,C", "rows": "1|2|3", "border": "false", "rule": "false", "weights": "0,1,3",
	})
	// Natural width: a column each and a pair of gaps, so cells.
	line := rows(table, 20, 0)[0]
	assert.Equal(t, 0, strings.Index(line, "A"))
	assert.Equal(t, 2, strings.Index(line, "B"), "A took none of the room")
	// B's quarter is cells, and the cell the shares leave over goes to the first weighted column.
	assert.Equal(t, 2+5+1, strings.Index(line, "C"), "B took a quarter of the room, and C the rest")
}

// Consider a table too narrow for its columns. That table takes the room from the columns fit does not name, and
// from the fit columns last, the least important first.
func TestFitColumnsKeepTheirWidthLongest(t *testing.T) {
	attrs := map[string]string{
		"columns": "NAME,PATH,SIZE", "rows": "report.pdf|/a/very/long/directory|12K", "border": "false", "rule": "false",
		"wrap": "false", "fit": "0,2",
	}
	table := build(t, "Table", attrs)
	// Too narrow by multiple cells: the path gives them all, and the name and the size keep their whole width.
	wide := rows(table, 23, 0)[1]
	assert.True(t, strings.HasPrefix(wide, "report.pdf /a/"), wide)
	assert.Contains(t, wide, "… 12K", "the path gave way and the size kept its width")

	// Too narrow by twenty-six: the path goes down to one cell, then the size to one, and the name gives the rest.
	narrow := strings.TrimRight(rows(table, 12, 0)[1], " ")
	assert.Equal(t, "report.… … …", narrow, "past the path, SIZE gives way before NAME")

	plain := build(t, "Table", map[string]string{
		"columns": "NAME,PATH,SIZE", "rows": "report.pdf|/a/very/long/directory|12K", "border": "false", "rule": "false",
		"wrap": "false",
	})
	assert.NotEqual(t, rows(table, 23, 0)[1], rows(plain, 23, 0)[1], "without fit the widest column is not the only one to give")
}

func TestTableRejectsBadWeightsAndFit(t *testing.T) {
	for attrs, want := range map[[2]string]string{
		{"weights", "1,2,3"}: "for 2 columns",
		{"weights", "-1"}:    "not a weight",
		{"fit", "2"}:         "is not a column",
		{"fit", "0,0"}:       "named twice",
	} {
		_, err := tryBuild("Table", map[string]string{"columns": "A,B", attrs[0]: attrs[1]})
		require.Error(t, err, attrs)
		assert.Contains(t, err.Error(), want)
	}
}

// A host's rows measure their widths once: a second table over the same rows reads the widths already measured.
func TestRowsMeasureTheirWidthsOnce(t *testing.T) {
	calls := 0
	measure := cell.Measure(func(s string) int { calls++; return len(s) })
	rows := widget.NewRows([][]string{{"ab", "c"}, {"d", "efg"}})

	assert.Equal(t, []int{2, 3}, rows.Widths(2, measure))
	first := calls
	assert.Equal(t, []int{2, 3}, rows.Widths(2, measure))
	assert.Equal(t, first, calls, "the second read measured again")
	assert.Equal(t, 2, rows.Len())
	assert.Equal(t, 0, (*widget.Rows)(nil).Len())
}

// Something other than widget.Rows in a table's rows is an error, not a blank table.
func TestATableRefusesASourceItCannotRead(t *testing.T) {
	values := map[string]sema.Value{"rows": sema.SourceValue(otherSource{}), "columns": sema.StringValue("A")}
	factory, _ := Library().Factory("Table")
	_, err := factory.Build(widget.Context{Attrs: widget.NewAttrs("Table", values, []string{"rows", "columns"})})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected a widget.Rows")
}

type otherSource struct{}

func (otherSource) Len() int { return 1 }
