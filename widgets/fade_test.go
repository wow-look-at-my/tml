package widgets

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wow-look-at-my/tml/cell"
)

// The band steps down from the content: the row nearest it keeps most of its color, the row at the edge is mostly
// background.
func TestFadePercentStepsWithTheBand(t *testing.T) {
	assert.Equal(t, 55, fadePercent(1, 1))
	assert.Equal(t, 35, fadePercent(1, 2))
	assert.Equal(t, 70, fadePercent(2, 2))
}

// fadedCell draws one glyph in a style, fades its row once, and reads the cell back.
func fadedCell(glyph string, style cell.Style, depth, band int, dark bool) cell.Cell {
	buf := cell.NewBuffer(1, 1)
	c := buf.Canvas(nil)
	c.Text(0, 0, glyph, style)
	palette := theme(dark)
	box := &scrollbox{fadeTo: palette["fade"], fadeText: palette["fadeText"]}
	box.fadeRow(c, 0, 1, depth, band)
	return buf.At(0, 0)
}

// Every spelling of a foreground is resolved to channels first. The fade adjusts a 256-colour or a named colour the
// same way it adjusts an RGB one.
func TestFadeRowResolvesEveryForegroundSpelling(t *testing.T) {
	tests := []struct {
		name  string
		fg    cell.Color
		faded cell.Color
	}{
		{"rgb", cell.RGB(10, 20, 30), cell.RGB(145, 149, 154)},
		{"256 cube", cell.Indexed(196), cell.RGB(255, 140, 140)},
		{"256 grey", cell.Indexed(236), cell.RGB(162, 162, 162)},
		{"named", cell.Indexed(1), cell.RGB(233, 140, 140)},
		{"bright named", cell.Indexed(9), cell.RGB(255, 140, 140)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.faded, fadedCell("x", cell.Style{Fg: tc.fg}, 1, 1, false).Style.Fg)
		})
	}
}

// A glyph that names no colour is drawn in the terminal's own, so the fade starts from an approximation of it.
func TestFadeRowBlendsTowardTheBackground(t *testing.T) {
	assert.Equal(t, cell.RGB(90, 90, 90), fadedCell("x", cell.Style{}, 1, 1, true).Style.Fg)
	assert.Equal(t, cell.RGB(167, 167, 167), fadedCell("x", cell.Style{}, 1, 1, false).Style.Fg)
	assert.Equal(t, theme(true)["fade"], cell.RGB(0, 0, 0))
	assert.Equal(t, theme(false)["fade"], cell.RGB(255, 255, 255))
	assert.Equal(t, theme(true)["fadeText"].Blend(theme(true)["fade"], 55), fadedCell("x", cell.Style{}, 1, 1, true).Style.Fg)
}

// The fade moves the foreground and nothing else: a background and the attributes survive it.
func TestFadeRowKeepsWhatIsNotForeground(t *testing.T) {
	style := cell.Style{Bg: cell.Indexed(4)}.Bold(true).Underline(true)

	got := fadedCell("x", style, 1, 1, false)
	assert.Equal(t, "x", got.Text)
	assert.Equal(t, cell.Indexed(4), got.Style.Bg, "a background survives the fade")
	assert.True(t, got.Style.Has(cell.Bold|cell.Underline))
	assert.Equal(t, cell.RGB(167, 167, 167), got.Style.Fg)
}

// A blank has no glyph to fade, so it is left exactly as it was.
func TestFadeRowLeavesABlankAlone(t *testing.T) {
	assert.True(t, fadedCell(" ", cell.Style{}, 2, 2, true).Style.IsZero())
}

// An attribute that draws on a blank makes the blank visible, so the fade takes it too.
func TestFadeRowFadesAVisibleBlank(t *testing.T) {
	for _, style := range []cell.Style{
		cell.Style{}.Underline(true), cell.Style{}.Reverse(true), cell.Style{}.Strikethrough(true),
	} {
		assert.Equal(t, cell.RGB(167, 167, 167), fadedCell(" ", style, 1, 1, false).Style.Fg, "%v", style.Attrs())
	}
}

func TestFadeRowsNeverMeet(t *testing.T) {
	assert.Equal(t, 0, fadeRows(1), "a single row has no edge band at all")
	assert.Equal(t, 0, fadeRows(2))
	assert.Equal(t, 1, fadeRows(3))
	assert.Equal(t, 1, fadeRows(7))
	assert.Equal(t, 2, fadeRows(8), "a tall region fades a band")
	assert.Equal(t, 2, fadeRows(40))
}
