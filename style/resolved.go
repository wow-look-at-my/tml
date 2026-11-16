// resolved.go holds Align.

package style

import (
	"fmt"
	"strings"

	"github.com/wow-look-at-my/tml/cell"
	"github.com/wow-look-at-my/tml/sema"
)

// Align is where content sits in space it does not fill: at the start, in the middle, or at the end.
type Align uint8

const (
	AlignStart Align = iota
	AlignCenter
	AlignEnd
)

// Offset is where content of the given size starts in the space available. The middle rounds toward the end.
func (a Align) Offset(size, available int) int {
	slack := max(0, available-size)
	switch a {
	case AlignCenter:
		return (slack + 1) / 2
	case AlignEnd:
		return slack
	default:
		return 0
	}
}

// Resolved is a single element's finished styling.
type Resolved struct {
	// Paint is the colours and text attributes. The element's descendants inherit the text half of it.
	Paint   cell.Style
	Padding sema.Thickness
	Margin  sema.Thickness
	// Border is the edge drawn inside the margin, or nil for none. BorderPaint is its colour.
	Border      *cell.Border
	BorderPaint cell.Style
	Align       Align
	VAlign      Align
}

// edge is the cells the border takes on each side.
func (r Resolved) edge() int {
	if r.Border == nil {
		return 0
	}
	return 1
}

// Frame is the space padding and borders take from the content box.
func (r Resolved) Frame() (horizontal, vertical int) {
	return r.Padding.Horizontal() + 2*r.edge(), r.Padding.Vertical() + 2*r.edge()
}

// ContentOffset is the distance from a box's outer top-left corner to the leading cell of its content.
func (r Resolved) ContentOffset() (x, y int) {
	x = r.Margin.Left + r.Padding.Left + r.edge()
	y = r.Margin.Top + r.Padding.Top + r.edge()
	return x, y
}

// colorAttrs are the attributes that take a colour.
var colorAttrs = map[string]bool{"fg": true, "bg": true, "borderColor": true}

// build turns a flattened attribute map into a resolved style.
func build(attrs map[string]string) (Resolved, error) {
	var resolved Resolved

	for name, raw := range attrs {
		var err error
		switch name {
		case "fg":
			resolved.Paint.Fg, err = cell.ParseColor(raw)
		case "bg":
			resolved.Paint.Bg, err = cell.ParseColor(raw)
		case "bold", "italic", "underline", "strikethrough", "faint", "reverse":
			err = applyTextFlag(&resolved, name, raw)
		case "padding":
			resolved.Padding, err = sema.ParseThickness(raw)
		case "margin":
			resolved.Margin, err = sema.ParseThickness(raw)
		case "border":
			err = applyBorder(&resolved, raw)
		case "borderColor":
			resolved.BorderPaint.Fg, err = cell.ParseColor(raw)
		case "align":
			resolved.Align, err = parseHorizontal(raw)
		case "valign":
			resolved.VAlign, err = parseVertical(raw)
		default:
			err = fmt.Errorf("unknown style attribute %q", name)
		}
		if err != nil {
			if colorAttrs[name] {
				err = fmt.Errorf("%s: %w", name, err)
			}
			return Resolved{}, err
		}
	}
	return resolved, nil
}

// textFlags are the attributes a style turns on or off by name.
var textFlags = map[string]cell.Attr{
	"bold":          cell.Bold,
	"italic":        cell.Italic,
	"underline":     cell.Underline,
	"strikethrough": cell.Strikethrough,
	"faint":         cell.Faint,
	"reverse":       cell.Reverse,
}

func applyTextFlag(resolved *Resolved, name, raw string) error {
	on, err := parseBool(raw)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	resolved.Paint = resolved.Paint.With(textFlags[name], on)
	return nil
}

func applyBorder(resolved *Resolved, raw string) error {
	if raw == "none" {
		return nil
	}
	border, ok := cell.BorderNamed(raw)
	if !ok {
		return fmt.Errorf("unknown border %q; want none, %s", raw, strings.Join(cell.BorderNames(), ", "))
	}
	resolved.Border = &border
	return nil
}

func parseHorizontal(raw string) (Align, error) {
	switch raw {
	case "left":
		return AlignStart, nil
	case "center":
		return AlignCenter, nil
	case "right":
		return AlignEnd, nil
	default:
		return 0, fmt.Errorf("align must be left, center or right, got %q", raw)
	}
}

func parseVertical(raw string) (Align, error) {
	switch raw {
	case "top":
		return AlignStart, nil
	case "middle":
		return AlignCenter, nil
	case "bottom":
		return AlignEnd, nil
	default:
		return 0, fmt.Errorf("valign must be top, middle or bottom, got %q", raw)
	}
}

func parseBool(raw string) (bool, error) {
	switch raw {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, fmt.Errorf("expected true or false, got %q", raw)
	}
}
