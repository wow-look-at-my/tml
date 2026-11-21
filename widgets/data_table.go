// data_table.go holds dataTable.

package widgets

import (
	"slices"
	"strconv"
	"strings"

	"github.com/wow-look-at-my/tml/cell"
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
	spacing int
	// selected is the row the host has marked, counted from the first data row: the header is not a row anyone selects.
	selected int
	disabled bool
	wrap     bool
	// collapse is the order the columns give way in when the table is wider than its space, as a Grid's collapse is.
	collapse []int
	state    widget.State
	measure  widget.Measurer
	// drawn is the width the table last drew at, which is the shape Part reads a point against.
	drawn int
}

func newTable(ctx widget.Context) (widget.Native, error) {
	separator := ctx.Attrs.String("separator", "|")
	bordered, err := ctx.Attrs.Bool("border", true)
	if err != nil {
		return nil, err
	}

	t := &dataTable{columns: ctx.Attrs.List("columns"), bordered: bordered, measure: ctx.Measure}
	if t.selected, err = ctx.Attrs.Int("selected", -1); err != nil {
		return nil, err
	}
	if t.disabled, err = ctx.Attrs.Bool("disabled", false); err != nil {
		return nil, err
	}
	if t.wrap, err = ctx.Attrs.Bool("wrap", true); err != nil {
		return nil, err
	}
	// A borderless table keeps the rule under its header unless told otherwise. Without it, the header is one line of
	// the listing.
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

// Part names a point of the table. "header:N" over the heading of column N, counted among every column the table
// declares, and "row:N" over data row N. A border, a rule or the gap between a pair of headings is no part.
func (t *dataTable) Part(x, y int) string {
	s := t.shapeAt(t.drawn)
	top := 0
	if t.bordered {
		top = 1
	}
	if y < top {
		return ""
	}
	header := t.rowHeight(t.columns, s)
	if y < top+header {
		if col, ok := t.columnAt(x, s); ok {
			return "header:" + strconv.Itoa(col)
		}
		return ""
	}
	at := top + header
	if t.headerRule() {
		at++
	}
	for i, row := range t.rows {
		height := t.rowHeight(row, s)
		if y >= at && y < at+height {
			return "row:" + strconv.Itoa(i)
		}
		at += height
	}
	return ""
}

// columnAt is the declared column whose cells span x.
func (t *dataTable) columnAt(x int, s shape) (int, bool) {
	start := t.edges() / 2
	for i, width := range s.widths {
		if x >= start && x < start+width {
			return s.shown[i], true
		}
		start += width + t.gap()
	}
	return 0, false
}

// A table with no rows has nothing to land on, so tab passes over it.
func (t *dataTable) AcceptsFocus() bool { return !t.disabled && len(t.rows) > 0 }

func (t *dataTable) SetState(state widget.State) { t.state = state }

// gap is the cells between a pair of columns: a border glyph, or the spacing of a borderless table.
func (t *dataTable) gap() int {
	if t.bordered {
		return 1
	}
	return t.spacing
}

// edges is the cells the left and right borders take together.
func (t *dataTable) edges() int {
	if t.bordered {
		return 2
	}
	return 0
}

// cellText is the text of a row's cell. A short row has fewer cells than the header, and the rest read as empty.
func cellText(row []string, col int) string {
	if col < len(row) {
		return row[col]
	}
	return ""
}

// natural is the widest cell of each shown column.
func (t *dataTable) natural(shown []int) []int {
	widths := make([]int, len(shown))
	for i, col := range shown {
		widths[i] = t.measure.Width(t.columns[col])
		for _, row := range t.rows {
			widths[i] = max(widths[i], t.measure.Width(cellText(row, col)))
		}
	}
	return widths
}

// total is the width a table of these columns draws to.
func (t *dataTable) total(widths []int) int {
	sum := t.edges() + t.gap()*max(0, len(widths)-1)
	for _, w := range widths {
		sum += w
	}
	return sum
}

// shownAt is the columns that stay when the table has to fit in w. A column gives way only while the table is too
// wide, in the order collapse names.
func (t *dataTable) shownAt(w int) []int {
	shown := make([]int, len(t.columns))
	for i := range shown {
		shown[i] = i
	}
	if w <= 0 {
		return shown
	}
	for _, col := range t.collapse {
		if t.total(t.natural(shown)) <= w {
			break
		}
		shown = slices.DeleteFunc(shown, func(i int) bool { return i == col })
	}
	return shown
}

// shapeAt lays the table out at w. A w of zero is the table's natural width. A table too wide for w takes cells from
// its widest column first, and a table narrower than w shares the room out evenly.
func (t *dataTable) shapeAt(w int) shape {
	shown := t.shownAt(w)
	widths := t.natural(shown)
	if w <= 0 || len(widths) == 0 {
		return shape{shown: shown, widths: widths}
	}
	for over := t.total(widths) - w; over > 0; over-- {
		widest := 0
		for i, width := range widths {
			if width > widths[widest] {
				widest = i
			}
		}
		if widths[widest] <= 1 {
			break
		}
		widths[widest]--
	}
	for under, i := w-t.total(widths), 0; under > 0; under, i = under-1, i+1 {
		widths[i%len(widths)]++
	}
	return shape{shown: shown, widths: widths}
}

// lines is one cell's text at its column width: wrapped, or cut to one line.
func (t *dataTable) lines(text string, width int) []cell.Line {
	if !t.wrap {
		return []cell.Line{t.measure.Text(text, cell.Style{}).Truncate(width, "…")}
	}
	return t.measure.Wrap(text, width)
}

// rowHeight is the lines the tallest cell of a row takes.
func (t *dataTable) rowHeight(row []string, s shape) int {
	height := 1
	for i, col := range s.shown {
		height = max(height, len(t.lines(cellText(row, col), s.widths[i])))
	}
	return height
}

// headerRule reports whether a line sits between the header and the rows.
func (t *dataTable) headerRule() bool { return t.bordered || t.rule }

func (t *dataTable) height(s shape) int {
	height := t.rowHeight(t.columns, s)
	if t.headerRule() {
		height++
	}
	if t.bordered {
		height += 2
	}
	for _, row := range t.rows {
		height += t.rowHeight(row, s)
	}
	return height
}

func (t *dataTable) Measure(maxW, _ int) (int, int) {
	w := t.total(t.natural(t.shownAt(maxW)))
	if maxW > 0 {
		w = min(w, maxW)
	}
	// Height at the width it draws at, not at the width it wants: a wrapped cell that measured short is clipped.
	return w, t.height(t.shapeAt(w))
}

func (t *dataTable) Draw(c cell.Canvas) {
	w, _ := c.Size()
	t.drawn = w
	if t.disabled {
		c = c.Styled(cell.Style{}.Faint(true))
	}
	s := t.shapeAt(w)

	y := 0
	if t.bordered {
		t.drawRule(c, y, s, "┌", "─", "┬", "┐")
		y++
	}
	y += t.drawRow(c, y, t.columns, s, cell.Style{})
	if t.bordered {
		t.drawRule(c, y, s, "├", "─", "┼", "┤")
		y++
	} else if t.rule {
		c.Repeat(0, y, t.total(s.widths), 1, "─", cell.Style{}.Faint(true))
		y++
	}
	for i, row := range t.rows {
		// The mark does not wait for focus. Focus adds the bold.
		var style cell.Style
		if i == t.selected && !t.disabled {
			style = style.Reverse(true)
			if t.state.Focused {
				style = style.Bold(true)
			}
		}
		y += t.drawRow(c, y, row, s, style)
	}
	if t.bordered {
		t.drawRule(c, y, s, "└", "─", "┴", "┘")
	}
}

// drawRule draws one horizontal border line, with a joint where each column rule meets it.
func (t *dataTable) drawRule(c cell.Canvas, y int, s shape, left, line, joint, right string) {
	x := 0
	c.Text(x, y, left, cell.Style{})
	x++
	for i, width := range s.widths {
		if i > 0 {
			c.Text(x, y, joint, cell.Style{})
			x++
		}
		c.Repeat(x, y, width, 1, line, cell.Style{})
		x += width
	}
	c.Text(x, y, right, cell.Style{})
}

// drawRow draws one row of cells in style, and reports how many lines it took. A borderless row carries the style
// across the gaps between its columns, so a marked row reads as one unbroken bar.
func (t *dataTable) drawRow(c cell.Canvas, y int, row []string, s shape, style cell.Style) int {
	height := t.rowHeight(row, s)
	x := 0
	if t.bordered {
		c.Repeat(x, y, 1, height, "│", cell.Style{})
		x++
	} else if !style.IsZero() {
		c.Fill(0, y, t.total(s.widths), height, style)
	}
	for i, col := range s.shown {
		if i > 0 {
			if t.bordered {
				c.Repeat(x, y, 1, height, "│", cell.Style{})
			}
			x += t.gap()
		}
		if t.bordered && !style.IsZero() {
			c.Fill(x, y, s.widths[i], height, style)
		}
		for n, line := range t.lines(cellText(row, col), s.widths[i]) {
			c.Line(x, y+n, line, style)
		}
		x += s.widths[i]
	}
	if t.bordered {
		c.Repeat(x, y, 1, height, "│", cell.Style{})
	}
	return height
}
