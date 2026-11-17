// check.go holds the check code.

package widgets

import (
	"github.com/wow-look-at-my/tml/cell"
	"github.com/wow-look-at-my/tml/widget"
)

var checkAttrs = []string{"label", "checked", "disabled"}

// marks are the glyph pairs for each spelling: unchecked then checked.
var marks = map[string][2]string{
	"Checkbox": {"[ ]", "[x]"},
	"Radio":    {"( )", "(•)"},
}

// check is a labelled on/off control. Checkbox and Radio differ only in their glyphs.
type check struct {
	kind     string
	label    string
	checked  bool
	disabled bool
	state    widget.State
	measure  widget.Measurer
}

func newCheck(kind string) builder {
	return func(ctx widget.Context) (widget.Native, error) {
		c := &check{kind: kind, label: ctx.Attrs.String("label", ""), measure: ctx.Measure}
		var err error
		if c.checked, err = ctx.Attrs.Bool("checked", false); err != nil {
			return nil, err
		}
		if c.disabled, err = ctx.Attrs.Bool("disabled", false); err != nil {
			return nil, err
		}
		return c, nil
	}
}

func (c *check) AcceptsFocus() bool { return !c.disabled }

func (c *check) SetState(state widget.State) { c.state = state }

func (c *check) mark() string {
	glyphs := marks[c.kind]
	if c.checked {
		return glyphs[1]
	}
	return glyphs[0]
}

func (c *check) body() string {
	if c.label == "" {
		return c.mark()
	}
	return c.mark() + " " + c.label
}

func (c *check) Measure(_, _ int) (int, int) { return c.measure.Width(c.body()), 1 }

func (c *check) Draw(canvas cell.Canvas) {
	var style cell.Style
	switch {
	case c.disabled:
		style = style.Faint(true)
	case c.state.Pressed:
		style = style.Reverse(true)
	case c.state.Focused:
		style = style.Bold(true).Underline(true)
	case c.state.Hovered:
		style = style.Underline(true)
	}
	canvas.Text(0, 0, c.body(), style)
}
