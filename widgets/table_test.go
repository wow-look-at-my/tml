package widgets

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
