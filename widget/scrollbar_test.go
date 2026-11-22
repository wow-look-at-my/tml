package widget_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wow-look-at-my/tml/cell"
	"github.com/wow-look-at-my/tml/widget"
)

// column is one column of a canvas the bar was drawn on, as a string of its glyphs.
func column(bar widget.Scrollbar, height int) string {
	buf := cell.NewBuffer(1, height)
	bar.Draw(buf.Canvas(nil), 0, cell.Style{})
	out := ""
	for _, line := range buf.Lines() {
		out += line
	}
	return out
}

func TestAScrollbarSizesAndPlacesItsThumb(t *testing.T) {
	bar := widget.Scrollbar{View: 10, MaxY: 30}
	start, size, ok := bar.Thumb()
	assert.True(t, ok)
	assert.Equal(t, [2]int{0, 2}, [2]int{start, size}, "a quarter of the content shows, so the thumb is a quarter of the bar")
	assert.Equal(t, "██││││││││", column(bar, 10))

	bar.Y = 30
	assert.Equal(t, "││││││││██", column(bar, 10), "at the end of the content the thumb is at the end of the bar")
	bar.Y = 15
	assert.Equal(t, "││││██││││", column(bar, 10))

	long := widget.Scrollbar{View: 5, Y: 0, MaxY: 5000}
	_, size, _ = long.Thumb()
	assert.Equal(t, 1, size, "the thumb never shrinks to nothing")
}

func TestAScrollbarWithNothingToScrollHasNoThumb(t *testing.T) {
	bar := widget.Scrollbar{View: 4}
	_, _, ok := bar.Thumb()
	assert.False(t, ok)
	assert.Equal(t, "││││", column(bar, 4), "the track alone")
	assert.Equal(t, "", column(widget.Scrollbar{}, 0))
	assert.Zero(t, bar.At(3, 0))
}

func TestAScrollbarTurnsAPointerRowIntoAPosition(t *testing.T) {
	bar := widget.Scrollbar{View: 10, MaxY: 30}
	assert.Equal(t, 0, bar.At(0, 0))
	assert.Equal(t, 30, bar.At(9, 1), "the thumb's last row under the bar's last row is the end")
	assert.Equal(t, 30, bar.At(40, 0), "a pointer past the bar stops at the end")
	assert.Equal(t, 0, bar.At(-5, 0))
	middle := bar.At(5, 1)
	assert.Equal(t, 15, middle)
	bar.Y = middle
	start, _, _ := bar.Thumb()
	assert.Equal(t, 4, start, "the thumb then starts where the pointer holds it")
}
