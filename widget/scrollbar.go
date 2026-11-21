// scrollbar.go holds the geometry and the drawing of a vertical scrollbar.

package widget

import "github.com/wow-look-at-my/tml/cell"

// Scrollbar is a vertical scrollbar beside a view of View rows. The content
// is scrolled Y rows down, of the MaxY rows it can be.
type Scrollbar struct {
	View, Y, MaxY int
}

// Thumb is the row the thumb starts on, and how many rows it covers. ok is false when the content fits the view:
// there is then nothing to scroll and no thumb.
func (s Scrollbar) Thumb() (start, size int, ok bool) {
	if s.View <= 0 || s.MaxY <= 0 {
		return 0, 0, false
	}
	size = max(1, s.View*s.View/(s.MaxY+s.View))
	span := s.View - size
	return min(span, max(0, s.Y*span/s.MaxY)), size, true
}

// Draw paints the track down column x of a canvas, and the thumb on it when there is one.
func (s Scrollbar) Draw(c cell.Canvas, x int, style cell.Style) {
	if s.View <= 0 {
		return
	}
	c.Repeat(x, 0, 1, s.View, "│", style)
	if start, size, ok := s.Thumb(); ok {
		c.Repeat(x, start, 1, min(size, s.View-start), "█", style)
	}
}

// At is the position that puts a row of the thumb under a row of the bar. grab is the row of the thumb, counted
// from its first. It is what a drag of the thumb asks for, and where a press on the track sends it.
func (s Scrollbar) At(row, grab int) int {
	_, size, ok := s.Thumb()
	span := s.View - size
	if !ok || span <= 0 {
		return max(0, min(s.Y, s.MaxY))
	}
	start := min(span, max(0, row-grab))
	return (start*s.MaxY + span/2) / span
}
