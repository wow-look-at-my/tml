// paint.go holds the paint code.

package widgets

import (
	"github.com/wow-look-at-my/tml/cell"
	"github.com/wow-look-at-my/tml/widget"
)

// paint builds a style from an optional colour, so a widget can colour a single part of itself.
func paint(color cell.Color) cell.Style { return cell.Style{Fg: color} }

// themed reads the theme colours a widget draws with, in the order of the names.
func themed(ctx widget.Context, names ...string) ([]cell.Color, error) {
	colors := make([]cell.Color, len(names))
	for i, name := range names {
		color, err := ctx.Theme.Named(name)
		if err != nil {
			return nil, err
		}
		colors[i] = color
	}
	return colors, nil
}
