// sparkline.go holds the sparkline code.

package widgets

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/wow-look-at-my/tml/cell"
	"github.com/wow-look-at-my/tml/widget"
)

var sparklineAttrs = []string{"values", "max", "color"}

// sparkBars are the partial-height blocks, low to high.
var sparkBars = []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

// sparkline plots a series in a single row of cells.
type sparkline struct {
	values []float64
	limit  float64
	color  cell.Color
}

func newSparkline(ctx widget.Context) (widget.Native, error) {
	raw := ctx.Attrs.List("values")
	color, err := ctx.Color("color")
	if err != nil {
		return nil, err
	}
	line := &sparkline{color: color}
	for _, field := range raw {
		n, err := strconv.ParseFloat(strings.TrimSpace(field), 64)
		if err != nil {
			return nil, fmt.Errorf("<Sparkline> values: %q is not a number", field)
		}
		line.values = append(line.values, n)
	}
	limit, err := ctx.Attrs.Float("max", 0)
	if err != nil {
		return nil, err
	}
	line.limit = limit
	return line, nil
}

// scale is the value the tallest bar stands for: the declared maximum, or the largest value when none was declared.
func (s *sparkline) scale() float64 {
	if s.limit > 0 {
		return s.limit
	}
	scale := 0.0
	for _, v := range s.values {
		scale = max(scale, v)
	}
	if scale <= 0 {
		return 1
	}
	return scale
}

func (s *sparkline) Measure(maxW, _ int) (int, int) {
	width := len(s.values)
	if maxW > 0 {
		width = min(width, maxW)
	}
	return width, 1
}

// Draw plots the series over every row it is given. A height on the element makes the graph that tall. A bar
// fills whole cells from the bottom row up, and the partial block is its top.
func (s *sparkline) Draw(c cell.Canvas) {
	w, h := c.Size()
	values := s.values
	// Too many points for the space: keep the most recent, because a series scrolls off the left in every chart anybody
	if w > 0 && len(values) > w {
		values = values[len(values)-w:]
	}
	scale := s.scale()

	h = max(h, 1)
	steps := len(sparkBars)
	rows := make([]strings.Builder, h)
	for _, v := range values {
		// A bar is a count of eighths of a cell. The lowest bar is still one eighth, so a zero shows.
		level := int(min(1, max(0, v/scale)) * float64(h*steps-1))
		for row := range h {
			switch above := level - row*steps; {
			case above >= steps:
				rows[row].WriteRune(sparkBars[steps-1])
			case above >= 0:
				rows[row].WriteRune(sparkBars[above])
			default:
				rows[row].WriteRune(' ')
			}
		}
	}
	for row := range h {
		c.Text(0, h-1-row, rows[row].String(), paint(s.color))
	}
}
