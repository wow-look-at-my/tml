// scrim.go holds scrim.

package widgets

import (
	"github.com/wow-look-at-my/tml/cell"
	"github.com/wow-look-at-my/tml/sema"
	"github.com/wow-look-at-my/tml/widget"
)

var scrimAttrs = []string{"on", "percent", "lift"}

// scrimPercent is how much of the background a darkened cell takes by default.
const scrimPercent = 60

// scrim darkens everything written inside it, so the part of the screen that has the user's attention stands out.
type scrim struct {
	on      bool
	percent int
	// liftOnHover takes the scrim off while the pointer is over it, or over the element around it.
	liftOnHover bool
	// to is the colour a darkened cell moves toward. text is the colour of a glyph that names none.
	to, text cell.Color
	state    widget.State
}

func (s *scrim) SetState(state widget.State) { s.state = state }

func newScrim(ctx widget.Context) (widget.Native, error) {
	s := &scrim{}
	var err error
	if s.on, err = ctx.Attrs.Bool("on", true); err != nil {
		return nil, err
	}
	if s.percent, err = ctx.Attrs.Int("percent", scrimPercent); err != nil {
		return nil, err
	}
	s.percent = min(100, max(0, s.percent))
	lift, err := ctx.Attrs.Enum("lift", "never", "never", "hover")
	if err != nil {
		return nil, err
	}
	s.liftOnHover = lift == "hover"
	colors, err := themed(ctx, "fade", "fadeText")
	if err != nil {
		return nil, err
	}
	s.to, s.text = colors[0], colors[1]
	return s, nil
}

// Insets is nothing: a scrim takes no cell of its own, so turning it on moves nothing.
func (s *scrim) Insets() sema.Thickness { return sema.Thickness{} }

// Measure is only reached when the element is empty, and an empty scrim is no cells.
func (s *scrim) Measure(_, _ int) (int, int) { return 0, 0 }

// Draw paints nothing behind the content.
func (s *scrim) Draw(cell.Canvas) {}

// Overlay moves every colour the children drew toward the background.
func (s *scrim) Overlay(c cell.Canvas) {
	if !s.on || s.percent == 0 || (s.liftOnHover && s.state.Hovered) {
		return
	}
	w, h := c.Size()
	c.Restyle(0, 0, w, h, func(drawn cell.Cell) cell.Style {
		style := drawn.Style
		if style.Bg.IsSet() {
			style.Bg = style.Bg.Blend(s.to, s.percent)
		}
		// A blank has no glyph to darken, unless an attribute makes the blank itself visible.
		if drawn.Text == " " && !style.Has(cell.Underline) && !style.Has(cell.Reverse) && !style.Has(cell.Strikethrough) {
			return style
		}
		from := style.Fg
		if !from.IsSet() {
			from = s.text
		}
		style.Fg = from.Blend(s.to, s.percent)
		return style
	})
}
