// scrollbox.go holds scrollbox.

package widgets

import (
	"github.com/wow-look-at-my/tml/cell"
	"github.com/wow-look-at-my/tml/sema"
	"github.com/wow-look-at-my/tml/widget"
)

var scrollboxAttrs = []string{"offset", "offsetX", "scrollbar", "contentHeight", "fade", "moreAbove", "moreBelow"}

// scrollbox shows part of content that is taller than the space it has, with a scrollbar beside it when there is more
// than fits.
type scrollbox struct {
	offsetX, offsetY int
	contentHeight    int
	bar              string
	fade             bool
	moreAbove        bool
	moreBelow        bool
	dark             bool
	// scroll is where layout put the content. The scrollbar and the fade are drawn from it.
	scroll widget.Scroll
}

func newScrollbox(ctx widget.Context) (widget.Native, error) {
	bar, err := ctx.Attrs.Enum("scrollbar", "auto", "auto", "always", "never")
	if err != nil {
		return nil, err
	}
	box := &scrollbox{bar: bar, dark: ctx.Dark}
	if box.fade, err = ctx.Attrs.Bool("fade", false); err != nil {
		return nil, err
	}
	if box.moreAbove, err = ctx.Attrs.Bool("moreAbove", false); err != nil {
		return nil, err
	}
	if box.moreBelow, err = ctx.Attrs.Bool("moreBelow", false); err != nil {
		return nil, err
	}
	if box.offsetY, err = ctx.Attrs.Int("offset", 0); err != nil {
		return nil, err
	}
	if box.offsetX, err = ctx.Attrs.Int("offsetX", 0); err != nil {
		return nil, err
	}
	if box.contentHeight, err = ctx.Attrs.Int("contentHeight", 0); err != nil {
		return nil, err
	}
	box.offsetX = max(0, box.offsetX)
	box.offsetY = max(0, box.offsetY)
	box.contentHeight = max(0, box.contentHeight)
	return box, nil
}

// Arrange is the whole point of a scrolling region: the content is measured against unlimited height.
func (s *scrollbox) Arrange() widget.Layout {
	return widget.Layout{
		FreeH:    true,
		FillW:    true,
		OffsetX:  s.offsetX,
		OffsetY:  s.offsetY,
		ContentH: s.contentHeight,
	}
}

// SetScroll takes the offset layout clamped to the content, and how far the content runs.
func (s *scrollbox) SetScroll(scroll widget.Scroll) { s.scroll = scroll }

// Insets reserves the scrollbar column, on the right.
func (s *scrollbox) Insets() sema.Thickness {
	if s.bar == "never" {
		return sema.Thickness{}
	}
	return sema.Thickness{Right: 1}
}

func (s *scrollbox) Measure(maxW, maxH int) (int, int) { return maxW, maxH }

// Draw paints nothing behind the content: the engine draws the children, clipped to the viewport.
func (s *scrollbox) Draw(cell.Canvas) {}

// Overlay draws what lies over the content: the fade at the edges, and the scrollbar in its gutter.
func (s *scrollbox) Overlay(c cell.Canvas) {
	w, h := c.Size()
	gutter := s.Insets().Right
	if s.fade {
		s.fadeEdges(c, w-gutter, h)
	}
	if gutter > 0 {
		s.drawBar(c, w-1, h)
	}
}

// drawBar is the gutter column: a track with a thumb sized and placed in proportion to what is on screen.
func (s *scrollbox) drawBar(c cell.Canvas, x, view int) {
	total := s.scroll.MaxY + view
	if view <= 0 || total <= view {
		if s.bar == "always" {
			c.Repeat(x, 0, 1, view, "│", cell.Style{})
		}
		return
	}
	c.Repeat(x, 0, 1, view, "│", cell.Style{})
	thumb := max(1, view*view/total)
	span := view - thumb
	start := min(span, max(0, s.scroll.Y*span/s.scroll.MaxY))
	c.Repeat(x, start, 1, min(thumb, view-start), "█", cell.Style{})
}

// fadeTall is the viewport height at which an edge fade widens from one row to a band.
const fadeTall = 8

// fadeRows is how many rows at each edge a viewport of the given height fades. The bands never meet.
func fadeRows(height int) int {
	rows := 1
	if height >= fadeTall {
		rows = 2
	}
	return min(rows, max(0, (height-1)/2))
}

// fadeEdges blends the rows at the ends of the viewport toward the background when there is content beyond them.
func (s *scrollbox) fadeEdges(c cell.Canvas, width, height int) {
	rows := fadeRows(height)
	above := s.moreAbove || s.scroll.Y > 0
	below := s.moreBelow || s.scroll.Y < s.scroll.MaxY
	// Depth counts inward from the edge, so the row at the edge is the faintest of the band.
	for i := range rows {
		if above {
			fadeRow(c, i, width, rows-i, rows, s.dark)
		}
		if below {
			fadeRow(c, height-1-i, width, rows-i, rows, s.dark)
		}
	}
}

// fadeTarget is what a faded row is blended toward: the background, which is black on a dark theme and white on a
// light one.
func fadeTarget(dark bool) cell.Color {
	if dark {
		return cell.RGB(0, 0, 0)
	}
	return cell.RGB(255, 255, 255)
}

// defaultForeground approximates the terminal's own foreground, the colour a cell that names none is drawn in.
func defaultForeground(dark bool) cell.Color {
	if dark {
		return cell.RGB(200, 200, 200)
	}
	return cell.RGB(60, 60, 60)
}

// fadePercent is how much of the background a row this deep into a band takes, so an edge reads as a gradient.
func fadePercent(depth, band int) int {
	if band < 2 {
		return 55
	}
	return 35 + 35*(depth-1)/(band-1)
}

// fadeRow moves the foreground of every glyph in a drawn row toward the background.
func fadeRow(c cell.Canvas, row, width, depth, band int, dark bool) {
	percent := fadePercent(depth, band)
	c.Restyle(0, row, width, 1, func(drawn cell.Cell) cell.Style {
		style := drawn.Style
		// A blank has no glyph to fade, unless an attribute makes the blank itself visible.
		if drawn.Text == " " && !style.Has(cell.Underline) && !style.Has(cell.Reverse) && !style.Has(cell.Strikethrough) {
			return style
		}
		from := style.Fg
		if !from.IsSet() {
			from = defaultForeground(dark)
		}
		style.Fg = from.Blend(fadeTarget(dark), percent)
		return style
	})
}
