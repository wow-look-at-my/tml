package widgets

import (
	"slices"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"

	"github.com/wow-look-at-my/tml/widget"
)

var tableAttrs = []string{"columns", "rows", "separator", "border", "rule", "spacing", "selected", "disabled", "wrap", "collapse"}

// dataTable is a grid of text with headers.
type dataTable struct {
	columns  []string
	rows     [][]string
	bordered bool
	rule     bool
	// spacing is the blank cells between a pair of columns of a borderless table.
	spacing   int
	separator string
	// selected is the row the host has marked, counted from the leading data row: the header is not a row anyone selects.
	selected int
	disabled bool
	wrap     bool
	// collapse is the order the columns give way in when the table is wider than its space, as a Grid's collapse is.
	collapse []int
	state    widget.State
}

func newTable(ctx widget.Context) (widget.Native, error) {
	separator := ctx.Attrs.String("separator", "|")
	bordered, err := ctx.Attrs.Bool("border", true)
	if err != nil {
		return nil, err
	}

	t := &dataTable{columns: ctx.Attrs.List("columns"), bordered: bordered, separator: separator}
	if t.selected, err = ctx.Attrs.Int("selected", -1); err != nil {
		return nil, err
	}
	if t.disabled, err = ctx.Attrs.Bool("disabled", false); err != nil {
		return nil, err
	}
	if t.wrap, err = ctx.Attrs.Bool("wrap", true); err != nil {
		return nil, err
	}
	// A borderless table keeps the rule under its header unless told otherwise. Without it, the header is one line of the listing.
	if t.rule, err = ctx.Attrs.Bool("rule", true); err != nil {
		return nil, err
	}
	if t.spacing, err = ctx.Attrs.Int("spacing", 1); err != nil {
		return nil, err
	}
	if t.spacing < 0 {
		return nil, ctx.Attrs.Errorf("spacing", "expected zero or more cells, got %d", t.spacing)
	}
	for _, raw := range ctx.Attrs.List("collapse") {
		col, convErr := strconv.Atoi(raw)
		if convErr != nil || col < 0 || col >= len(t.columns) {
			return nil, ctx.Attrs.Errorf("collapse", "%q is not a column of this table (0 to %d)", raw, len(t.columns)-1)
		}
		if slices.Contains(t.collapse, col) {
			return nil, ctx.Attrs.Errorf("collapse", "column %d is named twice", col)
		}
		t.collapse = append(t.collapse, col)
	}
	for _, raw := range ctx.Attrs.List("rows") {
		cells := strings.Split(raw, separator)
		for i, cell := range cells {
			cells[i] = strings.TrimSpace(cell)
		}
		t.rows = append(t.rows, cells)
	}
	return t, nil
}

// A table with no rows has nothing to land on, so tab passes over it.
func (t *dataTable) AcceptsFocus() bool { return !t.disabled && len(t.rows) > 0 }

func (t *dataTable) SetState(state widget.State) { t.state = state }

// cell styles the marked row, and pads a borderless table's columns apart. Padding rather than a rule keeps the gap
// part of the row, so the mark reads as an unbroken bar. The mark does not wait for focus; focus adds the bold.
func (t *dataTable) cell(row, col int) lipgloss.Style {
	style := lipgloss.NewStyle()
	if !t.bordered {
		style = style.PaddingRight(t.spacing)
	}
	if t.selected < 0 || row != t.selected || t.disabled {
		return style
	}
	style = style.Reverse(true)
	if t.state.Focused {
		style = style.Bold(true)
	}
	return style
}

// build lays the table out at w with the columns in hidden left out. A w of zero is the table's natural width.
func (t *dataTable) build(w int, hidden []int) *table.Table {
	columns, rows := t.columns, t.rows
	if len(hidden) > 0 {
		columns = dropCells(t.columns, hidden)
		rows = make([][]string, len(t.rows))
		for i, row := range t.rows {
			rows[i] = dropCells(row, hidden)
		}
	}
	built := table.New().Headers(columns...).Rows(rows...).StyleFunc(t.styleFor(len(columns))).Wrap(t.wrap)
	if w > 0 {
		built = built.Width(w)
	}
	if !t.bordered {
		// Every rule goes, the column rule included: cell padding is what now keeps full cells apart.
		built = built.Border(lipgloss.HiddenBorder()).BorderTop(false).BorderBottom(false).
			BorderLeft(false).BorderRight(false).BorderColumn(false).BorderHeader(t.rule)
	}
	return built
}

// styleFor is cell for a table that shows count columns. The padding rule asks which column is last on screen.
func (t *dataTable) styleFor(count int) func(row, col int) lipgloss.Style {
	return func(row, col int) lipgloss.Style {
		style := t.cell(row, col)
		if !t.bordered && col == count-1 {
			style = style.PaddingRight(0)
		}
		return style
	}
}

// dropCells is cells without the indices in hidden. A short row has fewer cells than the header, and keeps what it has.
func dropCells(cells []string, hidden []int) []string {
	kept := make([]string, 0, len(cells))
	for i, cell := range cells {
		if !slices.Contains(hidden, i) {
			kept = append(kept, cell)
		}
	}
	return kept
}

// hiddenAt is the columns that give way so the table fits in w. It stops when the table fits or the list runs out.
func (t *dataTable) hiddenAt(w int) []int {
	if w <= 0 {
		return nil
	}
	var hidden []int
	for _, col := range t.collapse {
		if natural, _ := lipgloss.Size(t.build(0, hidden).String()); natural <= w {
			break
		}
		hidden = append(hidden, col)
	}
	return hidden
}

func (t *dataTable) Measure(maxW, _ int) (int, int) {
	hidden := t.hiddenAt(maxW)
	w, _ := lipgloss.Size(t.build(0, hidden).String())
	if maxW > 0 {
		w = min(w, maxW)
	}
	// Height at the width it will draw at, not at the width it wants: a wrapped cell that measured short is clipped.
	_, h := lipgloss.Size(t.build(w, hidden).String())
	return w, h
}

func (t *dataTable) Render(w, _ int) string {
	out := t.build(w, t.hiddenAt(w)).String()
	if t.disabled {
		return lipgloss.NewStyle().Faint(true).Render(out)
	}
	return out
}
