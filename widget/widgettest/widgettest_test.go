package widgettest_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml/cell"
	"github.com/wow-look-at-my/tml/sema"
	"github.com/wow-look-at-my/tml/widget"
	"github.com/wow-look-at-my/tml/widget/widgettest"
)

// label measures to a fixed size and keeps the space it was offered.
type label struct {
	text           string
	w, h           int
	offerW, offerH *int
	style          cell.Style
}

func (l label) Measure(maxW, maxH int) (int, int) {
	if l.offerW != nil {
		*l.offerW, *l.offerH = maxW, maxH
	}
	return l.w, l.h
}

func (l label) Draw(c cell.Canvas) { c.Text(0, 0, l.text, l.style) }

func TestDrawTakesTheMeasuredSizeOnAZeroAxis(t *testing.T) {
	var offerW, offerH int
	native := label{text: "abc", w: 3, h: 2, offerW: &offerW, offerH: &offerH}

	buf := widgettest.Draw(native, 0, 0)
	assert.Equal(t, []int{3, 2}, []int{buf.W, buf.H})
	assert.Equal(t, "abc\n   ", buf.Text())

	buf = widgettest.Draw(native, 5, 0)
	assert.Equal(t, []int{5, 2}, []int{buf.W, buf.H}, "a given width is kept")
	assert.Equal(t, []int{5, 0}, []int{offerW, offerH}, "the widget is offered the size asked for")
}

func TestTextAndStyled(t *testing.T) {
	native := label{text: "hi", w: 2, h: 1, style: cell.Style{}.Bold(true)}

	assert.Equal(t, "hi  ", widgettest.Text(native, 4, 1), "rows are padded to the full width")
	assert.Contains(t, widgettest.Styled(native, 0, 0), "\x1b[1mhi")
}

// frame is a composer with a one-cell border on every side.
type frame struct {
	layout  widget.Layout
	scroll  *widget.Scroll
	content cell.Style
	overlay string
}

func (f frame) Measure(int, int) (int, int) { return 0, 0 }

func (f frame) Draw(c cell.Canvas) {
	w, h := c.Size()
	border, _ := cell.BorderNamed("normal")
	c.Box(0, 0, w, h, border, cell.Style{})
}

func (f frame) Insets() sema.Thickness { return sema.Thickness{Top: 1, Right: 1, Bottom: 1, Left: 1} }

// scroller is a frame that scrolls and keeps the scroll it is told.
type scroller struct{ frame }

func (s scroller) Arrange() widget.Layout { return s.layout }

func (s scroller) SetScroll(scroll widget.Scroll) { *s.scroll = scroll }

// styler is a frame whose children inherit a style.
type styler struct{ frame }

func (s styler) ContentStyle() cell.Style { return s.content }

// overlay is a frame that draws over its children.
type overlay struct{ frame }

func (o overlay) Overlay(c cell.Canvas) { c.Text(1, 1, o.overlay, cell.Style{}) }

func TestComposePlacesChildrenInsideTheInsets(t *testing.T) {
	buf := widgettest.Compose(frame{}, []string{"a", "bcdefg"}, 5, 4)

	assert.Equal(t, "┌───┐\n│a  │\n│bcd│\n└───┘", buf.Text(), "children stack inside the border and are cut at its edge")
}

func TestComposeTellsAScrollingWidgetItsScroll(t *testing.T) {
	var got widget.Scroll
	native := scroller{frame{layout: widget.Layout{FreeH: true, OffsetY: 9}, scroll: &got}}

	buf := widgettest.Compose(native, []string{"a", "b", "c", "d"}, 3, 4)

	assert.Equal(t, widget.Scroll{Y: 2, MaxY: 2}, got, "the offset stops at the last screenful")
	assert.Equal(t, "┌─┐\n│c│\n│d│\n└─┘", buf.Text(), "the children shift by the scroll")
}

func TestComposeAppliesTheContentStyle(t *testing.T) {
	native := styler{frame{content: cell.Style{}.Bold(true)}}

	buf := widgettest.Compose(native, []string{"a"}, 3, 3)

	assert.True(t, buf.At(1, 1).Style.Has(cell.Bold), "the child inherits the content style")
	assert.False(t, buf.At(0, 0).Style.Has(cell.Bold), "the widget's own cells do not")
}

func TestComposeDrawsTheOverlayLast(t *testing.T) {
	native := overlay{frame{overlay: "X"}}

	buf := widgettest.Compose(native, []string{"a"}, 3, 3)

	require.Equal(t, 3, buf.H)
	assert.Equal(t, "X", buf.At(1, 1).Text, "the overlay covers the child")
}
