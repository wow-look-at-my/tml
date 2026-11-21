// list.go holds the list code.

package widgets

import (
	"strconv"
	"strings"

	"github.com/wow-look-at-my/tml/cell"
	"github.com/wow-look-at-my/tml/widget"
)

var listAttrs = []string{"items", "selected", "cursor", "disabled"}

// list is a column of items with a single item marked. Which item is selected is a property, not state: the host owns
type list struct {
	items    []string
	selected int
	cursor   rune
	disabled bool
	state    widget.State
	measure  widget.Measurer
}

func newList(ctx widget.Context) (widget.Native, error) {
	l := &list{items: ctx.Attrs.List("items"), measure: ctx.Measure}
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

// Part names the item on line y as "item:N". Every item takes one line.
func (l *list) Part(_, y int) string {
	if y < 0 || y >= len(l.items) {
		return ""
	}
	return "item:" + strconv.Itoa(y)
}

func (l *list) SetState(state widget.State) { l.state = state }

// gutter is the cursor column plus its trailing space, kept even on unselected rows so the text does not shift
const gutter = 2

func (l *list) Measure(maxW, _ int) (int, int) {
	width := 0
	for _, item := range l.items {
		width = max(width, l.measure.Width(item)+gutter)
	}
	if maxW > 0 {
		width = min(width, maxW)
	}
	return width, len(l.items)
}

func (l *list) Draw(c cell.Canvas) {
	w, _ := c.Size()
	if l.disabled {
		c = c.Styled(cell.Style{}.Faint(true))
	}
	for i, item := range l.items {
		mark := strings.Repeat(" ", gutter)
		var style cell.Style
		if i == l.selected {
			mark = string(l.cursor) + " "
			style = style.Bold(true)
			if l.state.Focused && !l.disabled {
				style = style.Reverse(true)
			}
		}
		// The mark covers the row's whole width.
		c.Fill(0, i, w, 1, style)
		c.Line(0, i, l.measure.Text(mark+item, cell.Style{}).Truncate(max(0, w), "…"), style)
	}
}
