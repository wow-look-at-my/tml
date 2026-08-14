package widgets

import (
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/charmbracelet/x/ansi"

	"github.com/wow-look-at-my/tml/widget"
)

// ---------------------------------------------------------------- List

var listAttrs = []string{"items", "selected", "cursor", "disabled"}

// list is a column of items with one of them marked.
//
// Which item is selected is a property, not state: the host owns the selection
// the same way it owns the items. Moving the selection is a key press the host
// already handles, and a list that tracked it privately would disagree with
// whatever the host does with the value afterwards.
type list struct {
	items    []string
	selected int
	cursor   rune
	disabled bool
	state    widget.State
}

func newList(ctx widget.Context) (widget.Native, error) {
	l := &list{items: ctx.Attrs.List("items")}
	var err error
	if l.selected, err = ctx.Attrs.Int("selected", -1); err != nil {
		return nil, err
	}
	if l.cursor, err = ctx.Attrs.Rune("cursor", '>'); err != nil {
		return nil, err
	}
	if l.disabled, err = ctx.Attrs.Bool("disabled", false); err != nil {
		return nil, err
	}
	return l, nil
}

func (l *list) AcceptsFocus() bool { return !l.disabled && len(l.items) > 0 }

func (l *list) SetState(state widget.State) { l.state = state }

// gutter is the cursor column plus its trailing space, kept even on unselected
// rows so the text does not shift sideways as the selection moves.
const gutter = 2

func (l *list) Measure(maxW, _ int) (int, int) {
	width := 0
	for _, item := range l.items {
		width = max(width, lipgloss.Width(item)+gutter)
	}
	if maxW > 0 {
		width = min(width, maxW)
	}
	return width, len(l.items)
}

func (l *list) Render(w, _ int) string {
	rows := make([]string, 0, len(l.items))
	for i, item := range l.items {
		mark := strings.Repeat(" ", gutter)
		style := lipgloss.NewStyle()
		if i == l.selected {
			mark = string(l.cursor) + " "
			style = style.Bold(true)
			if l.state.Focused && !l.disabled {
				style = style.Reverse(true)
			}
		}
		// A row too wide for the space is cut, never wrapped: a wrapped row would
		// take two lines, push every row below it down, and stop the list's own
		// geometry from matching what is on the screen.
		row := ansi.Truncate(mark+item, max(0, w), "…")
		if gap := w - lipgloss.Width(row); gap > 0 {
			row += strings.Repeat(" ", gap)
		}
		rows = append(rows, style.Render(row))
	}
	if l.disabled {
		return lipgloss.NewStyle().Faint(true).Render(strings.Join(rows, "\n"))
	}
	return strings.Join(rows, "\n")
}

// ---------------------------------------------------------------- Table

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
