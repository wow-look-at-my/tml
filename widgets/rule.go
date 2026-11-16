// rule.go holds the rule code.

package widgets

import (
	"github.com/wow-look-at-my/tml/cell"
	"github.com/wow-look-at-my/tml/widget"
)

var ruleAttrs = []string{"orientation", "char", "title", "color"}

// rule is a divider: a line across the space it is given, optionally broken by a title.
type rule struct {
	vertical bool
	char     rune
	title    string
	color    cell.Color
	measure  widget.Measurer
}

func newRule(ctx widget.Context) (widget.Native, error) {
	orientation, err := ctx.Attrs.Enum("orientation", "horizontal", "horizontal", "vertical")
	if err != nil {
		return nil, err
	}
	r := &rule{
		vertical: orientation == "vertical",
		title:    ctx.Attrs.String("title", ""),
		measure:  ctx.Measure,
	}
	deflt := '─'
	if r.vertical {
		deflt = '│'
	}
	if r.char, err = ctx.Attrs.Rune("char", deflt); err != nil {
		return nil, err
	}
	if r.color, err = colorAttr(ctx.Attrs, "color"); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *rule) Measure(maxW, maxH int) (int, int) {
	if r.vertical {
		return 1, max(1, maxH)
	}
	return max(r.measure.Width(r.title), maxW), 1
}

func (r *rule) Draw(c cell.Canvas) {
	w, h := c.Size()
	style := paint(r.color)
	if r.vertical {
		c.Repeat(0, 0, 1, max(1, h), string(r.char), style)
		return
	}
	c.Repeat(0, 0, w, 1, string(r.char), style)
	if r.title == "" {
		return
	}
	// The title sits one cell in from the left with a space either side, so it never touches the line it breaks.
	label := " " + r.title + " "
	lead := min(1, max(0, w-r.measure.Width(label)))
	c.Text(lead, 0, label, cell.Style{})
}
