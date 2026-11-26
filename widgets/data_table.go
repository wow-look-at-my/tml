// data_table.go holds dataTable.

package widgets

import (
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/wow-look-at-my/tml/cell"
	"github.com/wow-look-at-my/tml/widget"
)

var tableAttrs = []string{
	"columns", "rows", "separator", "border", "rule", "spacing", "selected", "disabled", "wrap", "collapse", "weights", "fit", "tree",
}

// dataTable is a grid of text with headers. It takes every row up front. It sizes its columns from all of them, and
// draws only the rows a scrolling region shows.
type dataTable struct {
	columns  []string
	rows     *widget.Rows
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
	// weights is the share of spare room each column takes. fit is the columns.
	weights []int
	fit     []int
	// tree is each row's place in the tree the first column draws, or nil for a table with no tree.
	tree      []treeNode
	treeWidth int
	state     widget.State
	measure widget.Measurer
	// drawn is the width the table last drew at, and top the first of its lines on screen.
	drawn, top int
}

func newTable(ctx widget.Context) (widget.Native, error) {
	bordered, err := ctx.Attrs.Bool("border", true)
	if err != nil {
		return nil, err
	}

	t := &dataTable{columns: ctx.Attrs.List("columns"), bordered: bordered, measure: ctx.Measure, treeWidth: -1}
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
	if t.collapse, err = columnList(ctx, "collapse", len(t.columns)); err != nil {
		return nil, err
	}
	if t.fit, err = columnList(ctx, "fit", len(t.columns)); err != nil {
		return nil, err
	}
	if t.weights, err = weightList(ctx, len(t.columns)); err != nil {
		return nil, err
	}
	if t.rows, err = tableRows(ctx); err != nil {
		return nil, err
	}
	if t.tree, err = treeNodes(ctx, t.rows.Len()); err != nil {
		return nil, err
	}
	return t, nil
}

// columnList reads an attribute that names columns by index, each once.
func columnList(ctx widget.Context, name string, columns int) ([]int, error) {
	var out []int
	for _, raw := range ctx.Attrs.List(name) {
		col, err := strconv.Atoi(raw)
		if err != nil || col < 0 || col >= columns {
			return nil, ctx.Attrs.Errorf(name, "%q is not a column of this table (0 to %d)", raw, columns-1)
		}
		if slices.Contains(out, col) {
			return nil, ctx.Attrs.Errorf(name, "column %d is named twice", col)
		}
		out = append(out, col)
	}
	return out, nil
}

// weightList reads the weights, one for each column. A column the list does not reach weighs one.
func weightList(ctx widget.Context, columns int) ([]int, error) {
	raw := ctx.Attrs.List("weights")
	if len(raw) > columns {
		return nil, ctx.Attrs.Errorf("weights", "names %d weights for %d columns", len(raw), columns)
	}
	weights := make([]int, columns)
	for i := range weights {
		weights[i] = 1
	}
	for i, text := range raw {
		w, err := strconv.Atoi(strings.TrimSpace(text))
		if err != nil || w < 0 {
			return nil, ctx.Attrs.Errorf("weights", "%q is not a weight: expected zero or more", text)
		}
		weights[i] = w
	}
	return weights, nil
}

// tableRows reads the rows: a widget.Rows the host keeps, or a list of delimited strings.
func tableRows(ctx widget.Context) (*widget.Rows, error) {
	if src := ctx.Attrs.Source("rows"); src != nil {
		rows, ok := src.(*widget.Rows)
		if !ok {
			return nil, ctx.Attrs.Errorf("rows", "expected a widget.Rows, got %T", src)
		}
		return rows, nil
	}
	separator := ctx.Attrs.String("separator", "|")
	var cells [][]string
	for _, raw := range ctx.Attrs.List("rows") {
		row := strings.Split(raw, separator)
		for i, text := range row {
			row[i] = strings.TrimSpace(text)
		}
		cells = append(cells, row)
	}
	return widget.NewRows(cells), nil
}

// Part names a point of the table. "header:N" over the heading of column N, counted among every column the table
// declares, and "row:N" over data row N. A border, a rule or the gap between a pair of headings is no part.
func (t *dataTable) Part(x, y int) string {
	s := t.shapeAt(t.drawn)
	head := t.headLines(s)
	// The header is drawn at the top of what is on screen, over the rows a scroll moved under it.
	if line := y - t.top; line >= 0 && line < head {
		border := t.edges() / 2
		if line >= border && line < border+t.rowHeight(t.columns, s) {
			if col, ok := t.columnAt(x, s); ok {
				return "header:" + strconv.Itoa(col)
			}
		}
		return ""
	}
	if i := t.rowAt(s, y-head); i >= 0 && i < t.rows.Len() {
		return "row:" + strconv.Itoa(i)
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
func (t *dataTable) AcceptsFocus() bool { return !t.disabled && t.rows.Len() > 0 }

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

// natural is the widest cell of each shown column, over every row, not only the rows on screen.
func (t *dataTable) natural(shown []int) []int {
	all := t.rows.Widths(len(t.columns), t.measure)
	widths := make([]int, len(shown))
	for i, col := range shown {
		widths[i] = max(t.measure.Width(t.columns[col]), all[col])
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
// the columns fit does not name first, and from the fit columns last. A table narrower than w shares the room out by
// weight.
func (t *dataTable) shapeAt(w int) shape {
	shown := t.shownAt(w)
	widths := t.natural(shown)
	if w <= 0 || len(widths) == 0 {
		return shape{shown: shown, widths: widths}
	}
	if over := t.total(widths) - w; over > 0 {
		t.shrink(shown, widths, over)
	} else if over < 0 {
		t.grow(shown, widths, -over)
	}
	return shape{shown: shown, widths: widths}
}

// shrink takes over cells from the columns, widest first within each group. The columns fit does not name are the
// first group. Each fit column is a group of its own, the least important first.
func (t *dataTable) shrink(shown, widths []int, over int) {
	var rest []int
	for i, col := range shown {
		if !slices.Contains(t.fit, col) {
			rest = append(rest, i)
		}
	}
	groups := [][]int{rest}
	for _, col := range slices.Backward(t.fit) {
		if i := slices.Index(shown, col); i >= 0 {
			groups = append(groups, []int{i})
		}
	}
	for _, group := range groups {
		for over > 0 {
			widest := -1
			for _, i := range group {
				if widths[i] > 1 && (widest < 0 || widths[i] > widths[widest]) {
					widest = i
				}
			}
			if widest < 0 {
				break
			}
			widths[widest]--
			over--
		}
	}
}

// grow shares under spare cells among the columns by weight. What the shares leave over goes one cell at a time to the
// weighted columns, from the left.
func (t *dataTable) grow(shown, widths []int, under int) {
	sum := 0
	for _, col := range shown {
		sum += t.weights[col]
	}
	if sum == 0 {
		return
	}
	given := 0
	for i, col := range shown {
		share := under * t.weights[col] / sum
		widths[i] += share
		given += share
	}
	for i := 0; given < under; i = (i + 1) % len(shown) {
		if t.weights[shown[i]] > 0 {
			widths[i]++
			given++
		}
	}
}

// lines is one cell's text at its column width: wrapped, or cut to one line.
func (t *dataTable) lines(text string, width int) []cell.Line {
	if !t.wrap {
		return []cell.Line{t.measure.Text(text, cell.Style{}).Truncate(width, "…")}
	}
	return t.measure.Wrap(text, width)
}

// rowHeight is the lines the tallest cell of a row takes. A table that does not wrap gives every row one line.
func (t *dataTable) rowHeight(row []string, s shape) int {
	if !t.wrap {
		return 1
	}
	height := 1
	for i, col := range s.shown {
		height = max(height, len(t.lines(cellText(row, col), s.widths[i])))
	}
	return height
}

// starts is where each row begins, counted in lines from the first, with the line after the last at the end. A table
// that does not wrap needs none: row i starts at line i.
func (t *dataTable) starts(s shape) []int {
	if !t.wrap {
		return nil
	}
	return t.rows.Memo(fmt.Sprint(s.shown, s.widths), func() []int {
		out := make([]int, t.rows.Len()+1)
		for i := range t.rows.Len() {
			out[i+1] = out[i] + t.rowHeight(t.rows.Row(i), s)
		}
		return out
	})
}

// rowsHeight is the lines every row takes together.
func (t *dataTable) rowsHeight(s shape) int {
	if starts := t.starts(s); starts != nil {
		return starts[len(starts)-1]
	}
	return t.rows.Len()
}

func (t *dataTable) rowAt(s shape, y int) int {
	if y < 0 {
		return -1
	}
	starts := t.starts(s)
	if starts == nil {
		return y
	}
	return sort.SearchInts(starts, y+1) - 1
}

// rowStart is the line row i starts on, counted from the first row.
func (t *dataTable) rowStart(s shape, i int) int {
	if starts := t.starts(s); starts != nil {
		return starts[i]
	}
	return i
}

// headerRule reports whether a line sits between the header and the rows.
func (t *dataTable) headerRule() bool { return t.bordered || t.rule }

// headLines is the lines above the first row: the top border, the header and the rule under it.
func (t *dataTable) headLines(s shape) int {
	lines := t.edges()/2 + t.rowHeight(t.columns, s)
	if t.headerRule() {
		lines++
	}
	return lines
}

func (t *dataTable) height(s shape) int {
	return t.headLines(s) + t.rowsHeight(s) + t.edges()/2
}

func (t *dataTable) Measure(maxW, _ int) (int, int) {
	w := t.total(t.natural(t.shownAt(maxW)))
	if maxW > 0 {
		w = min(w, maxW)
	}
	// Height at the width it draws at, not at the width it wants: a wrapped cell that measured short is clipped.
	return w, t.height(t.shapeAt(w))
}

// Draw paints the rows that are on screen and no others, so a table of any length costs a screenful. A scrolling
// region that moved the header off the top gets it back there, drawn over the rows beneath it.
func (t *dataTable) Draw(c cell.Canvas) {
	w, _ := c.Size()
	visible := c.Visible()
	t.drawn, t.top = w, max(0, visible.Y)
	if t.disabled {
		c = c.Styled(cell.Style{}.Faint(true))
	}
	s := t.shapeAt(w)
	head := t.headLines(s)
	end := visible.Y + visible.H

	body := c.Clip(0, t.top+head, w, max(0, end-t.top-head))
	first := max(0, t.rowAt(s, t.top))
	y := head + t.rowStart(s, min(first, t.rows.Len()))
	for i := first; i < t.rows.Len() && y < end; i++ {
		// The mark does not wait for focus. Focus adds the bold.
		var style cell.Style
		if i == t.selected && !t.disabled {
			style = style.Reverse(true)
			if t.state.Focused {
				style = style.Bold(true)
			}
		}
		y += t.drawRow(body, y, t.rows.Row(i), s, style)
	}
	if t.bordered && y < end {
		t.drawRule(body, y, s, "└", "─", "┴", "┘")
	}
	t.drawHead(c, t.top, s)
}

// drawHead draws the top border, the header and the rule under it, from line y.
func (t *dataTable) drawHead(c cell.Canvas, y int, s shape) {
	if t.bordered {
		t.drawRule(c, y, s, "┌", "─", "┬", "┐")
		y++
	}
	y += t.drawRow(c, y, t.columns, s, cell.Style{})
	if t.bordered {
		t.drawRule(c, y, s, "├", "─", "┼", "┤")
	} else if t.rule {
		c.Repeat(0, y, t.total(s.widths), 1, "─", cell.Style{}.Faint(true))
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
