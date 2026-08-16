package widgets

import (
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"

	"github.com/wow-look-at-my/tml/widget"
)

var tableAttrs = []string{"columns", "rows", "separator", "border"}

// dataTable is a grid of text with headers.
//
// Rows arrive as a list of delimited strings because that is the shape the host
// already has: it built the values, and it can join them. The alternative --
// repeating elements per row -- is what <For> over a <Grid> is for, and this
// widget exists for the case where the data is a table rather than a layout.
type dataTable struct {
	columns   []string
	rows      [][]string
	bordered  bool
	separator string
}

func newTable(ctx widget.Context) (widget.Native, error) {
	separator := ctx.Attrs.String("separator", "|")
	bordered, err := ctx.Attrs.Bool("border", true)
	if err != nil {
		return nil, err
	}

	t := &dataTable{columns: ctx.Attrs.List("columns"), bordered: bordered, separator: separator}
	for _, raw := range ctx.Attrs.List("rows") {
		cells := strings.Split(raw, separator)
		for i, cell := range cells {
			cells[i] = strings.TrimSpace(cell)
		}
		t.rows = append(t.rows, cells)
	}
	return t, nil
}

func (t *dataTable) build(w int) *table.Table {
	built := table.New().Headers(t.columns...).Rows(t.rows...)
	if w > 0 {
		built = built.Width(w)
	}
	if !t.bordered {
		// The column rule stays on: hidden, it is the space that keeps two full
		// cells from running together into one word.
		built = built.Border(lipgloss.HiddenBorder()).BorderTop(false).BorderBottom(false).
			BorderLeft(false).BorderRight(false)
	}
	return built
}

func (t *dataTable) Measure(maxW, _ int) (int, int) {
	out := t.build(0).String()
	w, h := lipgloss.Size(out)
	if maxW > 0 {
		w = min(w, maxW)
	}
	return w, h
}

func (t *dataTable) Render(w, _ int) string { return t.build(w).String() }
