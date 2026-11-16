// buffer.go holds Cell.

package cell

import (
	"strings"
)

// Cell is one position of the grid. A wide glyph owns its cell and the cell after it, and that second cell has a zero
// width and no text.
type Cell struct {
	Text  string
	Width uint8
	Style Style
}

// blank is what an untouched cell holds.
var blank = Cell{Text: " ", Width: 1}

// Rect is a region of the grid.
type Rect struct{ X, Y, W, H int }

// Intersect is the overlap of a pair of regions. A pair that does not touch gives an empty region.
func (r Rect) Intersect(o Rect) Rect {
	x, y := max(r.X, o.X), max(r.Y, o.Y)
	return Rect{X: x, Y: y, W: max(0, min(r.X+r.W, o.X+o.W)-x), H: max(0, min(r.Y+r.H, o.Y+o.H)-y)}
}

// Contains reports whether the cell at x, y is inside the region.
func (r Rect) Contains(x, y int) bool {
	return x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H
}

// Placement is an escape sequence that draws over a region of cells: an inline image. The cells under it stay blank.
type Placement struct {
	Rect
	// Seq draws it, with the cursor at the region's top-left cell.
	Seq string
	// Clear removes it from the screen. Empty means that text drawn over the region is enough.
	Clear string
}

// Buffer is a frame: a grid of cells, plus the placements drawn over it.
type Buffer struct {
	W, H       int
	cells      []Cell
	Placements []Placement
}

// NewBuffer returns a blank grid.
func NewBuffer(w, h int) *Buffer {
	w, h = max(0, w), max(0, h)
	b := &Buffer{W: w, H: h, cells: make([]Cell, w*h)}
	for i := range b.cells {
		b.cells[i] = blank
	}
	return b
}

// At reads one cell. A position outside the grid reads as blank.
func (b *Buffer) At(x, y int) Cell {
	if x < 0 || y < 0 || x >= b.W || y >= b.H {
		return blank
	}
	return b.cells[y*b.W+x]
}

// set writes one cell, and repairs a wide glyph the write cuts in half.
func (b *Buffer) set(x, y int, c Cell) {
	if x < 0 || y < 0 || x >= b.W || y >= b.H {
		return
	}
	i := y*b.W + x
	old := b.cells[i]
	if old.Width == 0 && x > 0 {
		// The write lands on the second half of a wide glyph, so its first half goes blank.
		b.cells[i-1] = Cell{Text: " ", Width: 1, Style: b.cells[i-1].Style}
	}
	if old.Width == 2 && x+1 < b.W {
		b.cells[i+1] = Cell{Text: " ", Width: 1, Style: b.cells[i+1].Style}
	}
	b.cells[i] = c
}

// Canvas is the whole grid as a drawing surface.
func (b *Buffer) Canvas(m Measure) Canvas {
	return Canvas{buf: b, w: b.W, h: b.H, clip: Rect{W: b.W, H: b.H}, measure: m}
}

// Text is the grid's glyphs, one line per row, without styles.
func (b *Buffer) Text() string {
	var out strings.Builder
	for y := range b.H {
		if y > 0 {
			out.WriteByte('\n')
		}
		for x := range b.W {
			out.WriteString(b.cells[y*b.W+x].Text)
		}
	}
	return out.String()
}

// Lines is Text split into its rows.
func (b *Buffer) Lines() []string {
	if b.H == 0 {
		return []string{}
	}
	return strings.Split(b.Text(), "\n")
}

// String is the grid as text a terminal draws: every row at full width, styled with SGR, in full colour.
func (b *Buffer) String() string { return b.Encode(TrueColor) }

// Encode is String for a terminal with the given colour profile.
func (b *Buffer) Encode(profile Profile) string {
	var out strings.Builder
	for y := range b.H {
		if y > 0 {
			out.WriteByte('\n')
		}
		b.encodeRow(&out, y, 0, b.W, profile, true)
	}
	return out.String()
}

// encodeRow writes the cells of row y from column from up to column to. It leaves the terminal's style reset.
func (b *Buffer) encodeRow(out *strings.Builder, y, from, to int, profile Profile, placements bool) {
	current := Style{}
	for x := from; x < to; x++ {
		if placements {
			for _, p := range b.Placements {
				if p.X == x && p.Y == y {
					out.WriteString(p.Seq)
				}
			}
		}
		c := b.cells[y*b.W+x]
		if c.Width == 0 {
			continue
		}
		style := c.Style.drawn()
		if style != current {
			writeTransition(out, current, style, profile)
			current = style
		}
		out.WriteString(c.Text)
	}
	if !current.IsZero() {
		writeTransition(out, current, Style{}, profile)
	}
}
