// Package widgettest draws a widget without a view around it, so a widget's own test asserts on the cells it
// paints. It is for a host that writes widgets as much as for the library's own.
package widgettest

import (
	"github.com/wow-look-at-my/tml/cell"
	"github.com/wow-look-at-my/tml/widget"
)

// Draw paints a widget alone at a size. A size of zero on an axis takes what the widget measures on that axis.
func Draw(native widget.Native, w, h int) *cell.Buffer {
	w, h = sized(native, w, h)
	buf := cell.NewBuffer(w, h)
	native.Draw(buf.Canvas(nil))
	return buf
}

// Text is what Draw painted, without its styles: one line per row, each at the full width.
func Text(native widget.Native, w, h int) string { return Draw(native, w, h).Text() }

// Styled is what Draw painted, as the text a terminal draws.
func Styled(native widget.Native, w, h int) string { return Draw(native, w, h).String() }

func sized(native widget.Native, w, h int) (int, int) {
	measuredW, measuredH := native.Measure(w, h)
	if w <= 0 {
		w = measuredW
	}
	if h <= 0 {
		h = measuredH
	}
	return w, h
}

// Compose paints a composer around children the way the engine does. Each child is a block of text, and the blocks
// stack from the top, inside the widget's insets. A widget that scrolls is told its scroll position first, and a
// widget that draws over its children does so last.
func Compose(native widget.Composer, children []string, w, h int) *cell.Buffer {
	buf := cell.NewBuffer(w, h)
	canvas := buf.Canvas(nil)
	insets := native.Insets()
	viewW, viewH := max(0, w-insets.Horizontal()), max(0, h-insets.Vertical())

	var want widget.Layout
	if arranger, ok := native.(widget.Arranger); ok {
		want = arranger.Arrange()
	}
	total := 0
	for _, child := range children {
		total += cell.Height(child)
	}
	shift := 0
	if want.FreeH {
		if want.ContentH > 0 {
			total = want.ContentH
		}
		scroll := widget.Scroll{MaxY: max(0, total-viewH)}
		scroll.Y = max(0, min(want.OffsetY, scroll.MaxY))
		if scrolling, ok := native.(widget.Scrolling); ok {
			scrolling.SetScroll(scroll)
		}
		// Children that are a window over the content start at the top of it: the host cut them at the offset.
		if want.ContentH == 0 {
			shift = scroll.Y
		}
	}

	native.Draw(canvas)
	inner := canvas.Clip(insets.Left, insets.Top, viewW, viewH)
	if styler, ok := native.(widget.Styler); ok {
		inner = inner.Styled(styler.ContentStyle())
	}
	y := insets.Top - shift
	for _, child := range children {
		x := insets.Left - max(0, want.OffsetX)
		if want.CenterX {
			x += (viewW - cell.Width(child)) / 2
		}
		inner.Text(x, y, child, cell.Style{})
		y += cell.Height(child)
	}
	if overlay, ok := native.(widget.Overlay); ok {
		overlay.Overlay(canvas)
	}
	return buf
}
