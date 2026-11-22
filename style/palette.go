// palette.go builds the colours of a view from its themes.

package style

import (
	_ "embed"
	"fmt"
	"sync"

	"github.com/wow-look-at-my/tml/cell"
	"github.com/wow-look-at-my/tml/syntax"
)

//go:embed base.tml
var baseSource []byte

// Base is the theme every view starts from. It declares the colours the library itself draws with. A theme of the
// host replaces a colour when it declares the same name.
var Base = sync.OnceValue(func() *syntax.Theme {
	file, err := syntax.Parse("tml:base.tml", baseSource)
	if err != nil {
		panic(fmt.Sprintf("the base theme does not parse: %v", err))
	}
	return file.Theme
})

// Palette reads the colours of the themes for a mode. The base theme comes first, and a later theme wins.
func Palette(themes []*syntax.Theme, dark bool) (cell.Palette, error) {
	palette := cell.Palette{}
	for _, theme := range append([]*syntax.Theme{Base()}, themes...) {
		for _, declared := range theme.Colors {
			color, err := cell.ParseColor(declared.In(dark))
			if err != nil {
				return nil, &syntax.Error{Pos: declared.Pos, Message: fmt.Sprintf(
					"theme colour %q: %v", declared.Name, err)}
			}
			palette[declared.Name] = color
		}
	}
	return palette, nil
}
