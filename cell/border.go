// border.go holds Border.

package cell

import "sort"

// Border is the glyphs of a box's edges and corners. Each is one cell wide.
type Border struct {
	Top, Bottom, Left, Right                   string
	TopLeft, TopRight, BottomLeft, BottomRight string
}

// borders are named, so a document never spells out box drawing characters.
var borders = map[string]Border{
	"normal":  {"─", "─", "│", "│", "┌", "┐", "└", "┘"},
	"rounded": {"─", "─", "│", "│", "╭", "╮", "╰", "╯"},
	"thick":   {"━", "━", "┃", "┃", "┏", "┓", "┗", "┛"},
	"double":  {"═", "═", "║", "║", "╔", "╗", "╚", "╝"},
	"hidden":  {" ", " ", " ", " ", " ", " ", " ", " "},
	"block":   {"█", "█", "█", "█", "█", "█", "█", "█"},
	"ascii":   {"-", "-", "|", "|", "+", "+", "+", "+"},
}

// BorderNamed looks a border up by name.
func BorderNamed(name string) (Border, bool) {
	border, ok := borders[name]
	return border, ok
}

// BorderNames lists the borders, sorted so a diagnostic that names them is stable.
func BorderNames() []string {
	names := make([]string, 0, len(borders))
	for name := range borders {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
