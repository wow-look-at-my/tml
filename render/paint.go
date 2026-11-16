// paint.go holds Frame.

package render

import (
	"github.com/wow-look-at-my/tml/cell"
	"github.com/wow-look-at-my/tml/layout"
	"github.com/wow-look-at-my/tml/style"
	"github.com/wow-look-at-my/tml/widget"
)

// Frame paints a laid-out tree into a new buffer the size of the viewport.
func Frame(box *layout.Box, width, height int, measure cell.Measure) *cell.Buffer {
	buf := cell.NewBuffer(width, height)
	Paint(box, buf.Canvas(measure))
	return buf
}

// Alone paints one box and what is inside it into a buffer of the box's own size, as if nothing were around it.
func Alone(box *layout.Box, measure cell.Measure) *cell.Buffer {
	buf := cell.NewBuffer(box.Rect.W, box.Rect.H)
	if box.Visibility == layout.Collapsed {
		return cell.NewBuffer(0, 0)
	}
	Paint(box, buf.Canvas(measure).Sub(-box.Rect.X, -box.Rect.Y, box.Rect.X+box.Rect.W, box.Rect.Y+box.Rect.H))
	return buf
}

// Render is Alone as the text a terminal draws.
func Render(box *layout.Box) string {
	buf := Alone(box, nil)
	if buf.H == 0 {
		return ""
	}
	return buf.String()
}

// Paint draws box on the canvas of its parent's content. Every box lands on the rect arrange gave it, so what the
// screen shows and what layout published cannot disagree. Later siblings cover earlier ones.
func Paint(box *layout.Box, parent cell.Canvas) {
	// A collapsed element gave its slot back. A hidden one keeps its slot and paints none of it.
	if box.Visibility != layout.Visible {
		return
	}
	rect := box.Rect
	if box.Name == "#text" {
		parent.Sub(rect.X, rect.Y, rect.W, rect.H).Text(0, 0, box.Text, cell.Style{})
		return
	}

	look := box.Style
	margin := look.Margin
	width, height := rect.W-margin.Horizontal(), rect.H-margin.Vertical()
	if width <= 0 || height <= 0 {
		return
	}
	outer := parent.Sub(rect.X+margin.Left, rect.Y+margin.Top, width, height)

	edge := 0
	if look.Border != nil {
		edge = 1
		outer.Box(0, 0, width, height, *look.Border, look.BorderPaint)
	}
	// A fill covers the padding and the content. A reversed element fills too, because a reversed blank is visible.
	if look.Paint.Bg.IsSet() || look.Paint.Has(cell.Reverse) {
		outer.Fill(edge, edge, width-2*edge, height-2*edge, look.Paint)
	}
	if look.Paint.Bg.IsSet() {
		outer = outer.Grounded(look.Paint.Bg)
	}
	// Descendants inherit the text half of the style. The fill is already on the cells beneath them.
	content := outer.Styled(look.Paint.Text()).
		Sub(edge+look.Padding.Left, edge+look.Padding.Top, box.Content.W, box.Content.H)

	switch {
	case box.Name == "Text":
		paintText(box, content)
	case box.Native != nil:
		paintNative(box, content)
	default:
		// On a surface that places children at coordinates, a child covers what is beneath it, blank cells included.
		covers := box.Name == "Canvas" || box.Name == "Grid"
		for _, child := range box.Children {
			if covers && child.Visibility != layout.Collapsed {
				content.Erase(child.Rect.X, child.Rect.Y, child.Rect.W, child.Rect.H)
			}
			Paint(child, content)
		}
	}
}

// paintNative draws a widget, and the children it wraps when it is a composer.
func paintNative(box *layout.Box, content cell.Canvas) {
	box.Native.Draw(content)
	composer := box.Composer()
	if composer == nil || len(box.Children) == 0 {
		return
	}
	// Children stay inside the widget's insets, which is also what confines scrolled content to its viewport.
	insets := composer.Insets()
	inner := content.Clip(insets.Left, insets.Top, box.Content.W-insets.Horizontal(), box.Content.H-insets.Vertical())
	if styler, ok := box.Native.(widget.Styler); ok {
		inner = inner.Styled(styler.ContentStyle())
	}
	for _, child := range box.Children {
		Paint(child, inner)
	}
	if overlay, ok := box.Native.(widget.Overlay); ok {
		overlay.Overlay(content)
	}
}

// paintText draws a Text: wrapped to the width, or with each line cut at it.
func paintText(box *layout.Box, c cell.Canvas) {
	width, height := c.Size()
	measure := c.Measure()

	var lines []cell.Line
	switch box.Overflow {
	case layout.OverflowClip, layout.OverflowEllipsis:
		tail := ""
		if box.Overflow == layout.OverflowEllipsis {
			tail = "…"
		}
		for _, line := range measure.Lines(box.Text) {
			lines = append(lines, line.Truncate(width, tail))
		}
	default:
		lines = measure.Wrap(box.Text, width)
	}

	top := textTop(box.Style.VAlign, len(lines), height)
	for i, line := range lines {
		c.Line(textLeft(box.Style.Align, line.Width(), width), top+i, line, cell.Style{})
	}
}

// textLeft is the column a line starts at. The middle rounds toward the start, so a centred word leans left.
func textLeft(align style.Align, size, available int) int {
	slack := max(0, available-size)
	switch align {
	case style.AlignCenter:
		return slack / 2
	case style.AlignEnd:
		return slack
	default:
		return 0
	}
}

// textTop is the row the first line starts at.
func textTop(align style.Align, lines, available int) int {
	return align.Offset(lines, available)
}
