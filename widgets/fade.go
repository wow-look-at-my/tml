// fade.go holds the fade code.

package widgets

import (
	"strings"

	ansiopt "github.com/wow-look-at-my/ansi-optimizer"
)

// resetSGR ends every styled span a child drew.
const resetSGR = "\x1b[0m"

// emitter spells a parameter list back out the way a terminal reads it.
var emitter = ansiopt.NewEmitter()

// sgr is one SGR sequence built from a parameter list.
func sgr(params []int) string {
	return emitter.Emit([]ansiopt.Token{{Type: ansiopt.TokenSGR, SGRParams: params}})
}

// shade is a color resolved to its channels. A row's own colors arrive as whatever spelling the theme chose.
type shade struct{ r, g, b int }

// fadeTarget is what a faded row is blended toward: the background, which is black on a dark theme and white on a light
func fadeTarget(dark bool) shade {
	if dark {
		return shade{0, 0, 0}
	}
	return shade{255, 255, 255}
}

// defaultForeground approximates the terminal's own foreground, the color a row that names none is drawn in.
func defaultForeground(dark bool) shade {
	if dark {
		return shade{200, 200, 200}
	}
	return shade{60, 60, 60}
}

// fadePercent is how much of the background a row this deep into a band takes, so an edge reads as a gradient.
func fadePercent(depth, band int) int {
	if band < 2 {
		return 55
	}
	return 35 + 35*(depth-1)/(band-1)
}

// blend moves one channel part of the way to the background, rounded to the nearest color.
func blend(from, to, percent int) int { return (from*(100-percent) + to*percent + 50) / 100 }

// shaded is the color a row at depth d of a band of n rows draws a given foreground in.
func shaded(c shade, depth, band int, dark bool) shade {
	percent := fadePercent(depth, band)
	to := fadeTarget(dark)
	return shade{blend(c.r, to.r, percent), blend(c.g, to.g, percent), blend(c.b, to.b, percent)}
}

// fadeRow re-colors the foreground of every span in a drawn row toward the background, so it reads as an edge.
func fadeRow(line string, depth, band int, dark bool) string {
	if strings.TrimSpace(line) == "" {
		return line
	}
	base := shaded(defaultForeground(dark), depth, band, dark)
	var out strings.Builder
	out.WriteString(sgr(channels(base)))
	for _, token := range ansiopt.NewParser(line).Parse() {
		if token.Type != ansiopt.TokenSGR {
			out.WriteString(token.Raw)
			continue
		}
		kept, c, stated := fadeParams(token.SGRParams, dark)
		if stated {
			blended := shaded(c, depth, band, dark)
			kept = append(kept, channels(blended)...)
		}
		if len(kept) > 0 {
			out.WriteString(sgr(kept))
		}
	}
	out.WriteString(resetSGR)
	return out.String()
}

// fadeParams splits one SGR parameter list into what a fade keeps and the foreground it replaces.
func fadeParams(params []int, dark bool) (kept []int, c shade, stated bool) {
	for i := 0; i < len(params); i++ {
		switch p := params[i]; {
		case p == 0:
			kept = append(kept, p)
			c, stated = defaultForeground(dark), true
		case p == 38 && i+4 < len(params) && params[i+1] == 2:
			c, stated = shade{params[i+2], params[i+3], params[i+4]}, true
			i += 4
		case p == 38 && i+2 < len(params) && params[i+1] == 5:
			c, stated = palette256(params[i+2]), true
			i += 2
		case p == 39:
			c, stated = defaultForeground(dark), true
		case p >= 30 && p <= 37:
			c, stated = palette16[p-30], true
		case p >= 90 && p <= 97:
			c, stated = palette16[p-90+8], true
		default:
			kept = append(kept, p)
		}
	}
	return kept, c, stated
}

// channels spells a color as the parameters a terminal reads as a foreground.
func channels(c shade) []int { return []int{38, 2, c.r, c.g, c.b} }

// palette16 is the base palette a terminal names by index: the colors, then their bright halves.
var palette16 = [16]shade{
	{0, 0, 0}, {205, 0, 0}, {0, 205, 0}, {205, 205, 0},
	{0, 0, 238}, {205, 0, 205}, {0, 205, 205}, {229, 229, 229},
	{127, 127, 127}, {255, 0, 0}, {0, 255, 0}, {255, 255, 0},
	{92, 92, 255}, {255, 0, 255}, {0, 255, 255}, {255, 255, 255},
}

func palette256(n int) shade {
	switch {
	case n < 0:
		return shade{}
	case n < 16:
		return palette16[n]
	case n < 232:
		steps := [6]int{0, 95, 135, 175, 215, 255}
		i := n - 16
		return shade{steps[i/36], steps[(i%36)/6], steps[i%6]}
	default:
		g := 8 + 10*(min(n, 255)-232)
		return shade{g, g, g}
	}
}
