// progress_bar.go holds the progress bar code.

package widgets

import (
	"fmt"

	"github.com/wow-look-at-my/tml/cell"
	"github.com/wow-look-at-my/tml/widget"
)

var progressAttrs = []string{"value", "max", "filled", "empty", "color", "trackColor", "percent"}

// progressBar fills in proportion to value.
type progressBar struct {
	value, limit  float64
	filled, empty rune
	color, track  cell.Color
	percent       bool
	measure       widget.Measurer
}

func newProgressBar(ctx widget.Context) (widget.Native, error) {
	bar := &progressBar{measure: ctx.Measure}
	var err error
	if bar.value, err = ctx.Attrs.Float("value", 0); err != nil {
		return nil, err
	}
	if bar.limit, err = ctx.Attrs.Float("max", 1); err != nil {
		return nil, err
	}
	if bar.limit <= 0 {
		return nil, fmt.Errorf("<ProgressBar> max must be greater than zero, got %v", bar.limit)
	}
	if bar.filled, err = ctx.Attrs.Rune("filled", '█'); err != nil {
		return nil, err
	}
	if bar.empty, err = ctx.Attrs.Rune("empty", '░'); err != nil {
		return nil, err
	}
	if bar.percent, err = ctx.Attrs.Bool("percent", false); err != nil {
		return nil, err
	}
	if bar.color, err = ctx.Color("color"); err != nil {
		return nil, err
	}
	if bar.track, err = ctx.Color("trackColor"); err != nil {
		return nil, err
	}
	return bar, nil
}

// ratio is the fill fraction, clamped: a value outside the range is the host's arithmetic being off, and a bar longer
func (b *progressBar) ratio() float64 {
	return min(1, max(0, b.value/b.limit))
}

func (b *progressBar) label() string {
	if !b.percent {
		return ""
	}
	return fmt.Sprintf(" %3d%%", int(b.ratio()*100+0.5))
}

func (b *progressBar) Measure(maxW, _ int) (int, int) {
	// A bar has no natural length. It is as long as it is allowed to be, with enough room for the label when a label exists.
	width := maxW
	if width <= 0 {
		width = 20 + b.measure.Width(b.label())
	}
	return width, 1
}

func (b *progressBar) Draw(c cell.Canvas) {
	w, _ := c.Size()
	label := b.label()
	track := max(0, w-b.measure.Width(label))
	full := int(float64(track)*b.ratio() + 0.5)
	c.Repeat(0, 0, full, 1, string(b.filled), paint(b.color))
	c.Repeat(full, 0, track-full, 1, string(b.empty), paint(b.track))
	c.Text(track, 0, label, cell.Style{})
}
