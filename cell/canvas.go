// canvas.go holds Canvas.

package cell

// Canvas is a region of a Buffer that something draws on. Its coordinates
// start at its own top-left corner.
type Canvas struct {
	buf *Buffer
	// x and y are the canvas origin in buffer coordinates. The origin can be outside the clip: scrolled content is.
	x, y    int
	w, h    int
	clip    Rect
	base    Style
	measure Measure
}

// Size is the canvas's width and height in cells.
func (c Canvas) Size() (w, h int) { return c.w, c.h }

// Measure is how this canvas counts the width of text.
func (c Canvas) Measure() Measure { return c.measure }

// Base is the style every draw on this canvas lies over.
func (c Canvas) Base() Style { return c.base }

// Styled returns the canvas with style laid over its base, so everything drawn on it inherits that style.
func (c Canvas) Styled(style Style) Canvas {
	c.base = style.Over(c.base)
	return c
}

// Sub is a region of this canvas as a canvas of its own. Draws on it are clipped to the region.
func (c Canvas) Sub(x, y, w, h int) Canvas {
	w, h = max(0, w), max(0, h)
	c.x += x
	c.y += y
	c.w, c.h = w, h
	c.clip = c.clip.Intersect(Rect{X: c.x, Y: c.y, W: w, H: h})
	return c
}

// Clip narrows what the canvas shows to a region of it, and leaves its origin and size alone.
func (c Canvas) Clip(x, y, w, h int) Canvas {
	c.clip = c.clip.Intersect(Rect{X: c.x + x, Y: c.y + y, W: max(0, w), H: max(0, h)})
	return c
}

// Visible is the part of the canvas that draws, in the canvas's own coordinates.
func (c Canvas) Visible() Rect {
	return Rect{X: c.clip.X - c.x, Y: c.clip.Y - c.y, W: c.clip.W, H: c.clip.H}
}

// At reads the cell at x, y.
func (c Canvas) At(x, y int) Cell { return c.buf.At(c.x+x, c.y+y) }

// put writes one glyph, composited over what the cell already holds.
func (c Canvas) put(x, y int, text string, width int, style Style) {
	bx, by := c.x+x, c.y+y
	if !c.clip.Contains(bx, by) {
		return
	}
	style = style.Over(c.base)
	if width == 2 && !c.clip.Contains(bx+1, by) {
		// The second half of a wide glyph is clipped away, so a blank stands in for the first.
		text, width = " ", 1
	}
	if !style.Bg.IsSet() {
		style.Bg = c.buf.At(bx, by).Style.Bg
	}
	c.buf.set(bx, by, Cell{Text: text, Width: uint8(width), Style: style.drawn()})
	if width == 2 {
		c.buf.set(bx+1, by, Cell{Width: 0, Style: style.drawn()})
		// set repaired the first half as a cut glyph, so it is written again.
		c.buf.cells[by*c.buf.W+bx] = Cell{Text: text, Width: 2, Style: style.drawn()}
	}
}

// Line draws glyphs along row y from column x. Each glyph's own style lies over style. It returns the column after
// the last glyph.
func (c Canvas) Line(x, y int, line Line, style Style) int {
	for _, g := range line {
		c.put(x, y, g.Text, g.Width, g.Style.Over(style))
		x += g.Width
	}
	return x
}

// Text draws s from x, y. A newline moves to the next row, back at column x.
// SGR sequences in s style what follows them.
func (c Canvas) Text(x, y int, s string, style Style) int {
	end := x
	for i, line := range c.measure.Lines(s) {
		end = c.Line(x, y+i, line, style)
	}
	return end
}

// Fill paints a region with blanks in style.
func (c Canvas) Fill(x, y, w, h int, style Style) { c.Repeat(x, y, w, h, " ", style) }

// Repeat paints a region with one narrow glyph.
func (c Canvas) Repeat(x, y, w, h int, glyph string, style Style) {
	visible := c.Visible().Intersect(Rect{X: x, Y: y, W: w, H: h})
	for row := visible.Y; row < visible.Y+visible.H; row++ {
		for col := visible.X; col < visible.X+visible.W; col++ {
			c.put(col, row, glyph, 1, style)
		}
	}
}

// Restyle changes the style of the cells already drawn in a region, and leaves their glyphs alone.
func (c Canvas) Restyle(x, y, w, h int, change func(Style) Style) {
	visible := c.Visible().Intersect(Rect{X: x, Y: y, W: w, H: h})
	for row := visible.Y; row < visible.Y+visible.H; row++ {
		for col := visible.X; col < visible.X+visible.W; col++ {
			i := (c.y+row)*c.buf.W + c.x + col
			c.buf.cells[i].Style = change(c.buf.cells[i].Style).drawn()
		}
	}
}

// Place draws an escape sequence over a region, which is how an inline image reaches the screen. A region the clip cuts
// into is skipped, because a terminal cannot draw part of one.
func (c Canvas) Place(x, y, w, h int, seq, clear string) {
	region := Rect{X: c.x + x, Y: c.y + y, W: w, H: h}
	if w <= 0 || h <= 0 || c.clip.Intersect(region) != region {
		return
	}
	c.buf.Placements = append(c.buf.Placements, Placement{Rect: region, Seq: seq, Clear: clear})
}

// Box draws a border around a region.
func (c Canvas) Box(x, y, w, h int, border Border, style Style) {
	if w <= 0 || h <= 0 {
		return
	}
	right, bottom := x+w-1, y+h-1
	c.Repeat(x+1, y, w-2, 1, border.Top, style)
	c.Repeat(x+1, bottom, w-2, 1, border.Bottom, style)
	c.Repeat(x, y+1, 1, h-2, border.Left, style)
	c.Repeat(right, y+1, 1, h-2, border.Right, style)
	c.put(x, y, border.TopLeft, 1, style)
	c.put(right, y, border.TopRight, 1, style)
	c.put(x, bottom, border.BottomLeft, 1, style)
	c.put(right, bottom, border.BottomRight, 1, style)
}
