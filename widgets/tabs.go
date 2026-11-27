// tabs.go holds the tab strip: a row of labels with one of them selected.

package widgets

import (
	"strconv"

	"github.com/wow-look-at-my/tml/cell"
	"github.com/wow-look-at-my/tml/input"
	"github.com/wow-look-at-my/tml/widget"
)

var tabsAttrs = []string{"items", "selected", "disabled"}

// tabPad is the blank cells on each side of a label. tabGap is the blank cells between a pair of labels.
const (
	tabPad = 1
	tabGap = 1
)

// tabs is a strip of labels. The host owns which is selected, as with a List.
type tabs struct {
	items    []string
	selected int
	disabled bool
	// primary is the colour of the selected label. off is the colour of a disabled strip.
	primary, off cell.Color
	state        widget.State
	measure      widget.Measurer
	// drawn is the width the strip last drew at. Zero means it has not drawn yet.
	drawn int
}

func newTabs(ctx widget.Context) (widget.Native, error) {
	t := &tabs{items: ctx.Attrs.List("items"), measure: ctx.Measure}
	var err error
	if t.selected, err = ctx.Attrs.Int("selected", 0); err != nil {
		return nil, err
	}
	if t.disabled, err = ctx.Attrs.Bool("disabled", false); err != nil {
		return nil, err
	}
	colors, err := themed(ctx, "primary", "disabled")
	if err != nil {
		return nil, err
	}
	t.primary, t.off = colors[0], colors[1]
	return t, nil
}

// AcceptsFocus is false while disabled or empty.
func (t *tabs) AcceptsFocus() bool { return !t.disabled && len(t.items) > 0 }

func (t *tabs) SetState(state widget.State) { t.state = state }

// spans lays the labels out in w cells. A w of zero is unlimited. A label that does not fit is cut, and the labels
// after it are not drawn.
func (t *tabs) spans(w int) []tabSpan {
	var out []tabSpan
	x := 0
	for i, item := range t.items {
		if i > 0 {
			x += tabGap
		}
		text := t.measure.Text(item, cell.Style{})
		width := text.Width() + 2*tabPad
		if w > 0 && x+width > w {
			room := w - x - 2*tabPad
			if room <= 0 {
				break
			}
			out = append(out, tabSpan{x: x, w: room + 2*tabPad, text: text.Truncate(room, "…")})
			break
		}
		out = append(out, tabSpan{x: x, w: width, text: text})
		x += width
	}
	return out
}

func (t *tabs) Measure(maxW, _ int) (int, int) {
	width := 0
	if spans := t.spans(0); len(spans) > 0 {
		last := spans[len(spans)-1]
		width = last.x + last.w
	}
	if maxW > 0 {
		width = min(width, maxW)
	}
	return width, 1
}

func (t *tabs) Draw(c cell.Canvas) {
	w, _ := c.Size()
	t.drawn = w
	if t.disabled {
		c = c.Styled(cell.Style{}.Faint(true).Foreground(t.off))
	}
	for i, span := range t.spans(w) {
		var style cell.Style
		if i == t.selected && !t.disabled {
			style = style.Bold(true).Underline(true).Foreground(t.primary)
			if t.state.Focused {
				style = style.Reverse(true)
			}
		}
		c.Fill(span.x, 0, span.w, 1, style)
		c.Line(span.x+tabPad, 0, span.text, style)
	}
}

// Part names the label under x as "tab:N". The gaps between labels are no part, and a disabled strip has none.
func (t *tabs) Part(x, y int) string {
	if t.disabled || y != 0 {
		return ""
	}
	for i, span := range t.spans(t.drawn) {
		if x >= span.x && x < span.x+span.w {
			return "tab:" + strconv.Itoa(i)
		}
	}
	return ""
}

// Key moves the selection with left and right. The edit's value is the index of the new tab. Left at the first tab
// and right at the last are not taken, so the focus ring gets them.
func (t *tabs) Key(key input.Key) (widget.Edit, bool) {
	if t.disabled || len(t.items) == 0 {
		return widget.Edit{}, false
	}
	at := min(max(t.selected, 0), len(t.items)-1)
	switch key.Keystroke() {
	case "left":
		at--
	case "right":
		at++
	default:
		return widget.Edit{}, false
	}
	if at < 0 || at >= len(t.items) || at == t.selected {
		return widget.Edit{}, false
	}
	return widget.Edit{Value: strconv.Itoa(at), Cursor: at}, true
}
