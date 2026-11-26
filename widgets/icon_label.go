// icon_label.go holds the icon label code.

package widgets

import (
	"os"

	"github.com/wow-look-at-my/tml/cell"
	"github.com/wow-look-at-my/tml/widget"
)

var iconLabelAttrs = []string{"icon", "label", "protocol", "iconWidth", "gap"}

// iconLabelWidth is the cells an icon takes when the element does not say.
const iconLabelWidth = cellAspect

// iconLabel is a small picture and the text it names, on one row: a provider's mark beside the provider's name.
type iconLabel struct {
	icon    *imageWidget
	label   string
	iconW   int
	gap     int
	measure widget.Measurer
}

func newIconLabel(ctx widget.Context) (widget.Native, error) {
	iconW, err := ctx.Attrs.Int("iconWidth", iconLabelWidth)
	if err != nil {
		return nil, err
	}
	gap, err := ctx.Attrs.Int("gap", 1)
	if err != nil {
		return nil, err
	}
	if iconW < 1 {
		return nil, ctx.Attrs.Errorf("iconWidth", "expected at least 1, got %d", iconW)
	}
	if gap < 0 {
		return nil, ctx.Attrs.Errorf("gap", "expected 0 or more, got %d", gap)
	}
	l := &iconLabel{label: ctx.Attrs.String("label", ""), iconW: iconW, gap: gap, measure: ctx.Measure}

	src := ctx.Attrs.String("icon", "")
	if src == "" {
		return l, nil
	}
	protocol, err := ctx.Attrs.Enum("protocol", protoAuto, protocolNames...)
	if err != nil {
		return nil, err
	}
	if protocol == protoAuto {
		protocol = detectProtocol(os.Getenv)
	}
	// The label is already the text a link shows, so a link draws no icon.
	if protocol == protoLink {
		return l, nil
	}
	icon, err := loadImage(ctx, "IconLabel", "icon", src, l.label, protocol)
	if err != nil {
		return nil, err
	}
	l.icon = icon
	return l, nil
}

// lead is the cells before the label: the icon and the gap after it.
func (l *iconLabel) lead() int {
	if l.icon == nil {
		return 0
	}
	if l.label == "" {
		return l.iconW
	}
	return l.iconW + l.gap
}

func (l *iconLabel) Measure(_, _ int) (int, int) { return l.lead() + l.measure.Width(l.label), 1 }

// Draw gives the row to the label first. An icon the row has no room for is left out: a cut name reads worse than a
// missing mark.
func (l *iconLabel) Draw(c cell.Canvas) {
	w, h := c.Size()
	if w <= 0 || h <= 0 {
		return
	}
	lead := l.lead()
	if lead+l.measure.Width(l.label) > w {
		lead = 0
	}
	if lead > 0 {
		l.icon.Draw(c.Sub(0, 0, l.iconW, 1))
	}
	c.Text(lead, 0, l.label, cell.Style{})
}
