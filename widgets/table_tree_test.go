package widgets

import (
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml/widget"
)

// treeTable is a borderless table of a name and a number, with a tree in its first column.
func treeTable(t *testing.T, rows, tree string) widget.Native {
	t.Helper()
	return build(t, "Table", map[string]string{
		"columns": "NAME,N", "rows": rows, "tree": tree, "border": "false", "wrap": "false",
	})
}

func TestATreeIndentsAndMarksEachRow(t *testing.T) {
	table := treeTable(t, "root|1,child|2,deep|3,other|4", "0:open,1:closed,2:leaf,0:leaf")
	assert.Equal(t, []string{
		"NAME       N",
		"────────────",
		"▾ root     1",
		"  ▸ child  2",
		"      deep 3",
		"  other    4",
	}, trimmed(rows(table, 0, 0)))
}

// The first column is as wide as its widest cell with its prefix. The deepest row wins here, though its text is the shortest.
func TestATreeColumnIsSizedFromItsDeepestPrefix(t *testing.T) {
	table := treeTable(t, "abcdef|1,x|2", "0:open,5:leaf")
	w, _ := table.Measure(0, 0)
	assert.Equal(t, 5*2+2+1+1+1, w, "the deep row's prefix and its text, a gap, and the number")

	plain := build(t, "Table", map[string]string{"columns": "NAME,N", "rows": "abcdef|1,x|2", "border": "false"})
	w, _ = plain.Measure(0, 0)
	assert.Equal(t, 6+1+1, w)
}

// A point on the glyph of a row that opens or closes names toggle:N. Anywhere else on the row, and anywhere on a leaf,
// names the row.
func TestATreeNamesItsToggles(t *testing.T) {
	table := treeTable(t, "root|1,child|2,deep|3", "0:open,1:closed,2:leaf")
	rows(table, 0, 0)
	parted := table.(widget.Parted)

	assert.Equal(t, "toggle:0", parted.Part(0, 2))
	assert.Equal(t, "row:0", parted.Part(1, 2), "the space after the glyph")
	assert.Equal(t, "row:0", parted.Part(3, 2), "the text")
	assert.Equal(t, "row:1", parted.Part(0, 3), "the indent")
	assert.Equal(t, "toggle:1", parted.Part(2, 3))
	assert.Equal(t, "row:2", parted.Part(4, 4), "a leaf has no toggle")
	assert.Equal(t, "header:0", parted.Part(0, 0))
}

// The toggle names the row by its index in the whole listing, however far the table scrolled.
func TestATreeNamesItsTogglesWhileScrolled(t *testing.T) {
	n := 100
	tree := make([]string, n)
	for i := range tree {
		tree[i] = fmt.Sprintf("%d:closed", i%3)
	}
	values := map[string]string{"columns": "NAME,N", "wrap": "false", "tree": strings.Join(tree, ",")}
	table := sourceTable(t, numbered(n), values)
	scrolled(table, 40, 5, 50)
	parted := table.(widget.Parted)

	// The header and its rule take the first a couple of lines.
	assert.Equal(t, "toggle:50", parted.Part(4, 52))
	assert.Equal(t, "row:50", parted.Part(0, 52))
	assert.Equal(t, "toggle:51", parted.Part(0, 53))
	assert.Equal(t, "toggle:52", parted.Part(2, 54))
}

// A wrapped tree row keeps its prefix on every line, and only its first line has a toggle.
func TestAWrappedTreeRowIndentsEveryLine(t *testing.T) {
	table := build(t, "Table", map[string]string{
		"columns": "X", "rows": "one two", "tree": "1:open", "border": "false", "rule": "false",
	})
	assert.Equal(t, []string{"X", "  ▾ one", "    two"}, trimmed(rows(table, 7, 0)))
	parted := table.(widget.Parted)
	assert.Equal(t, "toggle:0", parted.Part(2, 1))
	assert.Equal(t, "row:0", parted.Part(2, 2))
}

func TestATreeRejectsAMalformedEntry(t *testing.T) {
	for tree, want := range map[string]string{
		"0:open,x:leaf":   `entry "x:leaf" of row 1`,
		"0:open,-1:leaf":  `entry "-1:leaf" of row 1`,
		"0:shut,0:leaf":   `entry "0:shut" of row 0`,
		"0:open,1":        `entry "1" of row 1`,
		"0:open":          "names 1 entries for 2 rows",
		"0:open,0:open,0": "names 3 entries for 2 rows",
	} {
		_, err := tryBuild("Table", map[string]string{"columns": "A", "rows": "a,b", "tree": tree})
		require.Error(t, err, tree)
		assert.Contains(t, err.Error(), want, tree)
	}
}

// Without a tree the table draws no prefix and names no toggle.
func TestATableWithoutATreeIsUnchanged(t *testing.T) {
	table := build(t, "Table", map[string]string{
		"columns": "NAME,N", "rows": "root|1,child|2", "border": "false", "wrap": "false",
	})
	assert.Equal(t, []string{"NAME  N", "───────", "root  1", "child 2"}, trimmed(rows(table, 0, 0)))
	assert.Equal(t, "row:0", table.(widget.Parted).Part(0, 2))
}

// A long tree costs the rows on screen, as a long table does.
func TestATreeDrawsOnlyTheRowsOnScreen(t *testing.T) {
	cost := func(n int) uint64 {
		tree := make([]string, n)
		for i := range tree {
			tree[i] = "1:leaf"
		}
		table := sourceTable(t, numbered(n), map[string]string{"columns": "NAME,N", "wrap": "false", "tree": strings.Join(tree, ",")})
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
