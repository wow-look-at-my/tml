package widgets

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wow-look-at-my/tml/widget"
	"github.com/wow-look-at-my/tml/widget/widgettest"
)

// A bordered table names each heading by its column and each data row by its index. Borders, rules and gaps name
// nothing.
func TestABorderedTableNamesItsParts(t *testing.T) {
	table := build(t, "Table", map[string]string{"columns": "A,Bb", "rows": "1|2,333|4"})
	widgettest.Draw(table, 0, 0)
	parted := table.(widget.Parted)

	assert.Empty(t, parted.Part(1, 0), "the top border")
	assert.Equal(t, "header:0", parted.Part(1, 1))
	assert.Equal(t, "header:1", parted.Part(5, 1))
	assert.Empty(t, parted.Part(4, 1), "the rule between the headings")
	assert.Empty(t, parted.Part(1, 2), "the rule under the header")
	assert.Equal(t, "row:0", parted.Part(1, 3))
	assert.Equal(t, "row:1", parted.Part(6, 4))
	assert.Empty(t, parted.Part(1, 5), "the bottom border")
}

// A collapsed column is skipped, so a heading names the column the table declared, not its place on screen.
func TestAPartNamesTheDeclaredColumn(t *testing.T) {
	table := build(t, "Table", map[string]string{
		"columns": "Name,Wide column,Size", "rows": "a|bbbbbbbbbbbbbb|1",
		"border": "false", "collapse": "1",
	})
	widgettest.Draw(table, 9, 0)
	parted := table.(widget.Parted)

	assert.Equal(t, "header:0", parted.Part(0, 0))
	assert.Equal(t, "header:2", parted.Part(8, 0))
	assert.Equal(t, "row:0", parted.Part(0, 2))
}
