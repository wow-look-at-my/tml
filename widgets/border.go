// border.go holds frame.

package widgets

import (
	"fmt"

	"github.com/wow-look-at-my/tml/cell"
	"github.com/wow-look-at-my/tml/sema"
	"github.com/wow-look-at-my/tml/widget"
)

var frameAttrs = []string{"kind", "title", "titleAlign", "color", "pad", "contentAlign"}

// frame draws a box around whatever is written inside it, with an optional title let into the top edge. Border and
// Popup are both this widget.
type frame struct {
	border     cell.Border
	title      string
	titleAlign string
	color      cell.Color
	pad        int
	// center puts each child in the middle of the width.
	center bool
	// face is the look a templated widget gives the frame's content: bold while focused, reversed while pressed.
	face cell.Style
	// dialog marks the popup spelling, which sits in the middle of a canvas rather than in its corner.
	dialog  bool
	measure widget.Measurer
}

func newFrame(dialog bool) builder {
	return func(ctx widget.Context) (widget.Native, error) {
		// A frame with its content against the edge reads badly, so both spellings pad by default.
		deflt, pad := "normal", 1
		if dialog {
			deflt = "rounded"
		}

		kind, err := ctx.Attrs.Enum("kind", deflt, cell.BorderNames()...)
		if err != nil {
			return nil, err
		}
		align, err := ctx.Attrs.Enum("titleAlign", "left", "left", "center", "right")
		if err != nil {
			return nil, err
		}
		if pad, err = ctx.Attrs.Int("pad", pad); err != nil {
			return nil, err
		}
		if pad < 0 {
			return nil, fmt.Errorf("<%s> pad must not be negative, got %d", ctx.Attrs.Element(), pad)
		}
		body, err := ctx.Attrs.Enum("contentAlign", "left", "left", "center")
		if err != nil {
			return nil, err
		}
		color, err := colorAttr(ctx.Attrs, "color")
		if err != nil {
			return nil, err
		}
		border, _ := cell.BorderNamed(kind)
		return &frame{
			border:     border,
			title:      ctx.Attrs.String("title", ""),
			titleAlign: align,
			color:      color,
			pad:        pad,
			center:     body == "center",
			dialog:     dialog,
			measure:    ctx.Measure,
		}, nil
	}
}

// Arrange tells layout where the children go, so a centred child is published where it is drawn.
func (f *frame) Arrange() widget.Layout { return widget.Layout{CenterX: f.center} }

// restyle is how a templated widget that wraps itself in this frame steers it: the border it draws. Its colour, and
// the face its content takes.
func (f *frame) restyle(border cell.Border, color cell.Color, face cell.Style) {
	f.border, f.color, f.face = border, color, face
}

// Insets is the border on every side, plus the padding on the left and the right.
func (f *frame) Insets() sema.Thickness {
	return sema.Thickness{Top: 1, Bottom: 1, Left: 1 + f.pad, Right: 1 + f.pad}
}

// ContentStyle hands the face to whatever is written inside the frame.
func (f *frame) ContentStyle() cell.Style { return f.face }

// DefaultAnchor puts a dialog in the middle of a canvas. A frame is a decoration and stays where it was put.
func (f *frame) DefaultAnchor() string {
	if f.dialog {
		return "center"
	}
	return ""
}

// Measure is only reached when the element is empty, since layout measures a composer's children itself.
func (f *frame) Measure(_, _ int) (int, int) {
	insets := f.Insets()
	insetW, insetH := insets.Horizontal(), insets.Vertical()
	if f.title != "" {
		insetW = max(insetW, f.measure.Width(f.title)+4)
	}
	return insetW, insetH
}

func (f *frame) Draw(c cell.Canvas) {
	w, h := c.Size()
	// A reversed face covers the whole inside of the frame, so a pressed button inverts as one block.
	if f.face.Has(cell.Reverse) {
		c.Fill(1, 1, w-2, h-2, f.face)
	}
	c.Box(0, 0, w, h, f.border, paint(f.color))
	f.drawTitle(c, w)
}

// drawTitle writes the label into the top edge.
func (f *frame) drawTitle(c cell.Canvas, width int) {
	edgeTitle(c, f.measure, f.title, f.titleAlign, width)
}

// edgeTitle writes a label into the top edge of a box. The corners are not edge, so the label only goes between
// them, and a label with no room is left out.
func edgeTitle(c cell.Canvas, measure widget.Measurer, title, align string, width int) {
	if title == "" {
		return
	}
	text := " " + title + " "
	room := width - 2
	size := measure.Width(text)
	if room <= 0 || size > room {
		return
	}
	offset := 1
	switch align {
	case "center":
		offset = 1 + (room-size)/2
	case "right":
		offset = 1 + room - size
	}
	c.Text(offset, 0, text, cell.Style{})
}
