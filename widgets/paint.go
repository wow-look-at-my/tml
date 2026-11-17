// paint.go holds the paint code.

package widgets

import (
	"github.com/wow-look-at-my/tml/cell"
	"github.com/wow-look-at-my/tml/widget"
)

// paint builds a style from an optional colour, so a widget can colour a single part of itself.
func paint(color cell.Color) cell.Style { return cell.Style{Fg: color} }

// colorAttr reads an optional colour attribute. A value that is not a colour fails the build of the widget.
func colorAttr(attrs widget.Attrs, name string) (cell.Color, error) {
	raw := attrs.String(name, "")
	if raw == "" {
		return cell.Color{}, nil
	}
	color, err := cell.ParseColor(raw)
	if err != nil {
		return cell.Color{}, attrs.Errorf(name, "%v", err)
	}
	return color, nil
}
