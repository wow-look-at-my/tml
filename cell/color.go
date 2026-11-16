// Package cell is TML's drawing model: a grid of styled cells, the canvas a widget draws on, and the text measurement
// both share. Nothing here knows about layout or a terminal's wire format beyond SGR.
package cell

import (
	"fmt"
	"strconv"
	"strings"
)

type colorKind uint8

const (
	colorNone colorKind = iota
	colorIndexed
	colorRGB
)

// Color is a foreground or background colour. The zero value is unset: the cell keeps what is already there.
type Color struct {
	kind    colorKind
	r, g, b uint8
}

func Indexed(n uint8) Color { return Color{kind: colorIndexed, r: n} }

// RGB is a 24-bit colour.
func RGB(r, g, b uint8) Color { return Color{kind: colorRGB, r: r, g: g, b: b} }

// IsSet reports whether the colour names anything.
func (c Color) IsSet() bool { return c.kind != colorNone }

// Index is the palette index, and false for an RGB or unset colour.
func (c Color) Index() (uint8, bool) { return c.r, c.kind == colorIndexed }

// RGB is the colour's channels. A palette colour answers with the xterm default for that index.
func (c Color) RGB() (r, g, b uint8) {
	if c.kind == colorIndexed {
		p := palette(c.r)
		return p[0], p[1], p[2]
	}
	return c.r, c.g, c.b
}

// String spells the colour the way ParseColor reads it.
func (c Color) String() string {
	switch c.kind {
	case colorIndexed:
		return strconv.Itoa(int(c.r))
	case colorRGB:
		return fmt.Sprintf("#%02x%02x%02x", c.r, c.g, c.b)
	default:
		return ""
	}
}

// names are the terminal colours, in palette order.
var names = []string{
	"black", "red", "green", "yellow", "blue", "magenta", "cyan", "white",
	"brightBlack", "brightRed", "brightGreen", "brightYellow", "brightBlue", "brightMagenta", "brightCyan", "brightWhite",
}

// ParseColor reads a palette index (`12`), a hex colour (`#rgb` or `#rrggbb`), or one of the terminal names.
func ParseColor(s string) (Color, error) {
	raw := strings.TrimSpace(s)
	if raw == "" {
		return Color{}, fmt.Errorf("a colour cannot be empty")
	}
	if strings.HasPrefix(raw, "#") {
		return parseHex(raw)
	}
	if n, err := strconv.Atoi(raw); err == nil {
		if n < 0 || n > 255 {
			return Color{}, fmt.Errorf("palette index %d is not between 0 and 255", n)
		}
		return Indexed(uint8(n)), nil
	}
	for i, name := range names {
		if name == raw {
			return Indexed(uint8(i)), nil
		}
	}
	return Color{}, fmt.Errorf("%q is not a colour; want a palette index, #rgb, #rrggbb or a terminal colour name", s)
}

func parseHex(raw string) (Color, error) {
	hex := raw[1:]
	if len(hex) == 3 {
		hex = string([]byte{hex[0], hex[0], hex[1], hex[1], hex[2], hex[2]})
	}
	if len(hex) != 6 {
		return Color{}, fmt.Errorf("%q is not #rgb or #rrggbb", raw)
	}
	n, err := strconv.ParseUint(hex, 16, 32)
	if err != nil {
		return Color{}, fmt.Errorf("%q is not #rgb or #rrggbb", raw)
	}
	return RGB(uint8(n>>16), uint8(n>>8), uint8(n)), nil
}

// MustColor is ParseColor for a literal the caller wrote.
func MustColor(s string) Color {
	c, err := ParseColor(s)
	if err != nil {
		panic(err)
	}
	return c
}

// Blend moves the colour part of the way to another, percent being how much of the other it takes.
func (c Color) Blend(to Color, percent int) Color {
	r, g, b := c.RGB()
	tr, tg, tb := to.RGB()
	mix := func(from, to uint8) uint8 {
		return uint8((int(from)*(100-percent) + int(to)*percent + 50) / 100)
	}
	return RGB(mix(r, tr), mix(g, tg), mix(b, tb))
}

// base16 is xterm's default for the colours a terminal is free to redefine.
var base16 = [16][3]uint8{
	{0, 0, 0}, {205, 0, 0}, {0, 205, 0}, {205, 205, 0},
	{0, 0, 238}, {205, 0, 205}, {0, 205, 205}, {229, 229, 229},
	{127, 127, 127}, {255, 0, 0}, {0, 255, 0}, {255, 255, 0},
	{92, 92, 255}, {255, 0, 255}, {0, 255, 255}, {255, 255, 255},
}

var cubeSteps = [6]uint8{0, 95, 135, 175, 215, 255}

func palette(n uint8) [3]uint8 {
	switch {
	case n < 16:
		return base16[n]
	case n < 232:
		i := int(n) - 16
		return [3]uint8{cubeSteps[i/36], cubeSteps[(i%36)/6], cubeSteps[i%6]}
	default:
		g := uint8(8 + 10*(int(n)-232))
		return [3]uint8{g, g, g}
	}
}
