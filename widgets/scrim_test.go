package widgets

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml/cell"
	"github.com/wow-look-at-my/tml/widget"
	"github.com/wow-look-at-my/tml/widget/widgettest"
)

// scrimmed draws one child inside a scrim and reads its first cell back.
func scrimmed(t *testing.T, child string, attrs map[string]string, dark bool) cell.Cell {
	t.Helper()
	factory, ok := Library().Factory("Scrim")
	require.True(t, ok)
	native, err := factory.Build(widget.Context{Attrs: attrsOf("Scrim", attrs), Theme: theme(dark)})
	require.NoError(t, err)
	return widgettest.Compose(native.(widget.Composer), []string{child}, 4, 1).At(0, 0)
}

// overlaid draws one cell in a style, lays a scrim over it, and reads the cell back.
func overlaid(glyph string, style cell.Style, s *scrim) cell.Style {
	buf := cell.NewBuffer(1, 1)
	c := buf.Canvas(nil)
	c.Text(0, 0, glyph, style)
	s.Overlay(c)
	return buf.At(0, 0).Style
}

// A glyph with no colour of its own is drawn in the terminal's, so the scrim starts from the theme's guess at it.
func TestScrimDarkensTextTowardTheBackground(t *testing.T) {
	palette := theme(true)
	want := palette["fadeText"].Blend(palette["fade"], scrimPercent)

	assert.Equal(t, want, scrimmed(t, "x", nil, true).Style.Fg)
}

// A coloured glyph keeps its hue and loses its weight, which is what keeps a darkened screen readable.
func TestScrimDarkensAColouredGlyph(t *testing.T) {
	got := overlaid("x", cell.Style{Fg: cell.RGB(200, 0, 0)}, &scrim{on: true, percent: 50, to: cell.RGB(0, 0, 0)})

	assert.Equal(t, cell.RGB(100, 0, 0), got.Fg)
}

// A fill is darkened with the text over it, or a highlighted row would stay the brightest thing on screen.
func TestScrimDarkensAFill(t *testing.T) {
	got := overlaid(" ", cell.Style{Bg: cell.RGB(200, 100, 0)}, &scrim{on: true, percent: 50, to: cell.RGB(0, 0, 0)})

	assert.Equal(t, cell.RGB(100, 50, 0), got.Bg)
	assert.False(t, got.Fg.IsSet(), "a blank has no glyph, so it takes no foreground")
}

// Off is the same frame as no scrim at all: the element stays in the tree, so turning it on moves nothing.
func TestScrimOffLeavesTheContentAlone(t *testing.T) {
	assert.True(t, scrimmed(t, "x", map[string]string{"on": "false"}, true).Style.IsZero())
	assert.True(t, scrimmed(t, "x", map[string]string{"percent": "0"}, true).Style.IsZero())
}

// The attributes survive: a darkened caret is still a caret.
func TestScrimKeepsAttributes(t *testing.T) {
	got := overlaid(" ", cell.Style{}.Reverse(true),
		&scrim{on: true, percent: 50, to: cell.RGB(0, 0, 0), text: cell.RGB(200, 200, 200)})

	assert.True(t, got.Has(cell.Reverse))
	assert.Equal(t, cell.RGB(100, 100, 100), got.Fg, "a reversed blank is drawn in the foreground, so it darkens")
}

func TestScrimRejectsABadAttribute(t *testing.T) {
	_, err := tryBuild("Scrim", map[string]string{"on": "maybe"})
	require.Error(t, err)
	_, err = tryBuild("Scrim", map[string]string{"percent": "half"})
	require.Error(t, err)
}
