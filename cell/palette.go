// palette.go holds the colours of a theme.

package cell

import (
	"fmt"
	"sort"
	"strings"
)

// Palette is the colours a theme declares, by name, for the mode a view draws in.
type Palette map[string]Color

const refPrefix = "theme:"

// Ref is the text a `{theme.name}` reference to a theme colour evaluates to.
func Ref(name string) string { return refPrefix + name }

// RefName is the name of the theme colour a reference points at.
func RefName(raw string) (string, bool) {
	name, ok := strings.CutPrefix(raw, refPrefix)
	return name, ok && name != ""
}

// Named is the theme colour with a name. A widget written in Go takes its colours through this.
func (p Palette) Named(name string) (Color, error) {
	color, ok := p[name]
	if !ok {
		return Color{}, fmt.Errorf("the theme declares no colour %q; it has %s", name, strings.Join(p.Names(), ", "))
	}
	return color, nil
}

// Resolve reads the value of a colour attribute. Only a reference to a theme colour is a value: a colour written
// in place is an error.
func (p Palette) Resolve(raw string) (Color, error) {
	name, ok := RefName(strings.TrimSpace(raw))
	if !ok {
		return Color{}, fmt.Errorf(
			"%q is a colour written in place; declare <Color name=\"…\" for=\"…\"> in a theme and write {theme.name}", raw)
	}
	return p.Named(name)
}

// Names lists the colours of the palette in order.
func (p Palette) Names() []string {
	names := make([]string, 0, len(p))
	for name := range p {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
