package widgets

import (
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"

	"github.com/wow-look-at-my/tml/widget"
)

var tableAttrs = []string{"columns", "rows", "separator", "border", "selected", "disabled"}

// dataTable is a grid of text with headers. Rows arrive as a list of delimited strings because that is the shape the
type dataTable struct {
	columns   []string
	rows      [][]string
	bordered  bool
	separator string
	// selected is the row the host has marked, counted from the leading data row: the header is not a row anyone selects.
	selected int
	disabled bool
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

// cell styles the marked row, and pads a borderless table's columns apart. The gap is padding rather than a rule so
// that it belongs to the row: a marked row then highlights as an unbroken bar, not as chunks with holes between them.
func (t *dataTable) cell(row, col int) lipgloss.Style {
	style := lipgloss.NewStyle()
	if !t.bordered && col < len(t.columns)-1 {
		style = style.PaddingRight(1)
	}
	if t.selected < 0 || row != t.selected || t.disabled {
		return style
	}
	style = style.Bold(true)
	if t.state.Focused {
		style = style.Reverse(true)
	}
	return style
}

func (t *dataTable) build(w int) *table.Table {
	built := table.New().Headers(t.columns...).Rows(t.rows...).StyleFunc(t.cell)
	if w > 0 {
		built = built.Width(w)
	}
	if !t.bordered {
		// Every rule goes, the column rule included: cell padding is what now keeps full cells apart.
		built = built.Border(lipgloss.HiddenBorder()).BorderTop(false).BorderBottom(false).
			BorderLeft(false).BorderRight(false).BorderColumn(false)
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

func (t *dataTable) Render(w, _ int) string {
	out := t.build(w).String()
	if t.disabled {
		return lipgloss.NewStyle().Faint(true).Render(out)
	}
	return out
}
