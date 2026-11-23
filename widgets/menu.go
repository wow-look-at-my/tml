// menu.go holds the menu: a framed column of choices, opened at a place on a canvas.

package widgets

import (
	"strconv"

	"github.com/wow-look-at-my/tml/cell"
	"github.com/wow-look-at-my/tml/widget"
)

var menuAttrs = []string{"items", "title", "selected", "maxWidth", "color", "accent", "accentText"}

// menuMaxWidth is how wide a menu gets when the element sets no maxWidth.
const menuMaxWidth = 40

// menuChrome is the cells a menu keeps beside each choice: the border and one cell of padding, on both sides.
const menuChrome = 4

// menu is a framed column of choices. It is as wide as its longest line, up
// to a cap, and a longer choice is cut. A canvas keeps it whole on screen.
type menu struct {
	items              []string
	title              string
	selected           int
	maxWidth           int
	color              cell.Color
	accent, accentText cell.Color
	measure            widget.Measurer
}

func newMenu(ctx widget.Context) (widget.Native, error) {
	m := &menu{items: ctx.Attrs.List("items"), title: ctx.Attrs.String("title", ""), measure: ctx.Measure}
	var err error
	if m.selected, err = ctx.Attrs.Int("selected", -1); err != nil {
		return nil, err
	}
	if m.maxWidth, err = ctx.Attrs.Int("maxWidth", menuMaxWidth); err != nil {
		return nil, err
	}
	if m.maxWidth <= menuChrome {
		return nil, ctx.Attrs.Errorf("maxWidth", "a menu needs more than %d cells, got %d", menuChrome, m.maxWidth)
	}
	if m.color, err = ctx.Color("color"); err != nil {
		return nil, err
	}
	if m.accent, err = ctx.Color("accent"); err != nil {
		return nil, err
	}
	if m.accentText, err = ctx.Color("accentText"); err != nil {
		return nil, err
	}
	return m, nil
}

// Contained keeps the menu whole on its canvas: opened near an edge, it moves back inside.
func (m *menu) Contained() bool { return true }

// Part names the choice on row y as "item:N". The first and the last row are the border.
func (m *menu) Part(_, y int) string {
	if y < 1 || y > len(m.items) {
		return ""
	}
	return "item:" + strconv.Itoa(y-1)
}

// Measure is the longest line with its chrome, held to the cap and to the room on offer.
func (m *menu) Measure(maxW, _ int) (int, int) {
	width := menuChrome
	if m.title != "" {
		width = m.measure.Width(m.title) + menuChrome
	}
	for _, item := range m.items {
		width = max(width, m.measure.Width(item)+menuChrome)
	}
	width = min(width, m.maxWidth)
	if maxW > 0 {
		width = min(width, maxW)
	}
	return width, len(m.items) + 2
}

func (m *menu) Draw(c cell.Canvas) {
	w, h := c.Size()
	border, _ := cell.BorderNamed("rounded")
	c.Box(0, 0, w, h, border, paint(m.color))
	edgeTitle(c, m.measure, m.title, "left", w)
	for i, item := range m.items {
		var style cell.Style
		if i == m.selected {
			style = m.marked()
		}
		// The mark covers the row from border to border.
		c.Fill(1, 1+i, w-2, 1, style)
		c.Line(2, 1+i, m.measure.Text(item, cell.Style{}).Truncate(max(0, w-menuChrome), "…"), style)
	}
}

// marked is the look of the selected choice: the accent colours when the element gave any, else reversed.
func (m *menu) marked() cell.Style {
	if m.accent == (cell.Color{}) && m.accentText == (cell.Color{}) {
		return cell.Style{}.Reverse(true)
	}
	return cell.Style{Fg: m.accentText, Bg: m.accent}
}
