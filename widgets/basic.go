package widgets

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/wow-look-at-my/tml/widget"
)

// paint builds a style from an optional colour, so a widget can colour one part
// of itself without the element's own style bleeding onto the rest.
func paint(color string) lipgloss.Style {
	style := lipgloss.NewStyle()
	if color == "" {
		return style
	}
	return style.Foreground(lipgloss.Color(color))
}

// repeat draws n copies of r, guarding the negative counts that arithmetic on a
// too-small box produces.
func repeat(r rune, n int) string {
	if n <= 0 {
		return ""
	}
	return strings.Repeat(string(r), n)
}

// ---------------------------------------------------------------- Rule

var ruleAttrs = []string{"orientation", "char", "title", "color"}

// rule is a divider: a line across the space it is given, optionally broken by a
// title.
type rule struct {
	vertical bool
	char     rune
	title    string
	color    string
	measure  widget.Measurer
}

func newRule(ctx widget.Context) (widget.Native, error) {
	orientation, err := ctx.Attrs.Enum("orientation", "horizontal", "horizontal", "vertical")
	if err != nil {
		return nil, err
	}
	r := &rule{
		vertical: orientation == "vertical",
		title:    ctx.Attrs.String("title", ""),
		measure:  ctx.Measure,
	}
	deflt := '─'
	if r.vertical {
		deflt = '│'
	}
	if r.char, err = ctx.Attrs.Rune("char", deflt); err != nil {
		return nil, err
	}
	r.color = ctx.Attrs.String("color", "")
	return r, nil
}

func (r *rule) Measure(maxW, maxH int) (int, int) {
	if r.vertical {
		return 1, max(1, maxH)
	}
	return max(r.measure.Width(r.title), maxW), 1
}

func (r *rule) Render(w, h int) string {
	style := paint(r.color)
	if r.vertical {
		lines := make([]string, max(1, h))
		for i := range lines {
			lines[i] = style.Render(string(r.char))
		}
		return strings.Join(lines, "\n")
	}
	if r.title == "" {
		return style.Render(repeat(r.char, w))
	}
	// The title sits one cell in from the left with a space either side, which
	// is what keeps it from touching the line it interrupts.
	label := " " + r.title + " "
	lead := min(1, max(0, w-r.measure.Width(label)))
	trail := max(0, w-lead-r.measure.Width(label))
	return style.Render(repeat(r.char, lead)) + label + style.Render(repeat(r.char, trail))
}

// ---------------------------------------------------------------- ProgressBar

var progressAttrs = []string{"value", "max", "filled", "empty", "color", "trackColor", "percent"}

// progressBar fills in proportion to value.
type progressBar struct {
	value, limit  float64
	filled, empty rune
	color, track  string
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
	bar.color = ctx.Attrs.String("color", "")
	bar.track = ctx.Attrs.String("trackColor", "")
	return bar, nil
}

// ratio is the fill fraction, clamped: a value outside the range is the host's
// arithmetic being off, and a bar longer than its own track would corrupt the
// layout rather than report it.
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
	// A bar has no natural length: it is as long as it is allowed to be, with
	// enough room for the label when there is one.
	width := maxW
	if width <= 0 {
		width = 20 + b.measure.Width(b.label())
	}
	return width, 1
}

func (b *progressBar) Render(w, _ int) string {
	label := b.label()
	track := max(0, w-b.measure.Width(label))
	full := int(float64(track)*b.ratio() + 0.5)
	return paint(b.color).Render(repeat(b.filled, full)) +
		paint(b.track).Render(repeat(b.empty, track-full)) + label
}

// ---------------------------------------------------------------- Spinner

var spinnerAttrs = []string{"frame", "kind", "color"}

// spinners are the frame sets, keyed by the name a template asks for.
var spinners = map[string][]string{
	"dots":   {"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
	"line":   {"|", "/", "-", "\\"},
	"bar":    {"▁", "▃", "▄", "▅", "▆", "▇", "▆", "▅", "▄", "▃"},
	"circle": {"◐", "◓", "◑", "◒"},
	"arrow":  {"←", "↖", "↑", "↗", "→", "↘", "↓", "↙"},
	"dot":    {"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"},
}

// spinnerKinds is sorted so the diagnostic listing them is stable.
var spinnerKinds = []string{"arrow", "bar", "circle", "dot", "dots", "line"}

// spinner shows one frame of an animation. Which frame is the host's business:
// TML has no clock, and a widget that animated itself would have to own one.
type spinner struct {
	frames  []string
	frame   int
	color   string
	measure widget.Measurer
}

func newSpinner(ctx widget.Context) (widget.Native, error) {
	kind, err := ctx.Attrs.Enum("kind", "dots", spinnerKinds...)
	if err != nil {
		return nil, err
	}
	frame, err := ctx.Attrs.Int("frame", 0)
	if err != nil {
		return nil, err
	}
	frames := spinners[kind]
	// A frame counter is normally a tick count that only goes up, so it wraps
	// here rather than making every caller do the modulo.
	frame = ((frame % len(frames)) + len(frames)) % len(frames)
	return &spinner{
		frames:  frames,
		frame:   frame,
		color:   ctx.Attrs.String("color", ""),
		measure: ctx.Measure,
	}, nil
}

func (s *spinner) Measure(_, _ int) (int, int) {
	return s.measure.Width(s.frames[s.frame]), 1
}

func (s *spinner) Render(_, _ int) string {
	return paint(s.color).Render(s.frames[s.frame])
}

// ---------------------------------------------------------------- Sparkline

var sparklineAttrs = []string{"values", "max", "color"}

// sparkBars are the eighth-height blocks, low to high.
var sparkBars = []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

// sparkline plots a series in one row of cells.
type sparkline struct {
	values []float64
	limit  float64
	color  string
}

func newSparkline(ctx widget.Context) (widget.Native, error) {
	raw := ctx.Attrs.List("values")
	line := &sparkline{color: ctx.Attrs.String("color", "")}
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

// scale is the value the tallest bar stands for: the declared maximum, or the
// largest value when none was declared.
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

func (s *sparkline) Render(w, _ int) string {
	values := s.values
	// Too many points for the space: keep the most recent, because a series
	// scrolls off the left in every chart anybody reads.
	if w > 0 && len(values) > w {
		values = values[len(values)-w:]
	}
	scale := s.scale()

	var b strings.Builder
	for _, v := range values {
		index := int(min(1, max(0, v/scale)) * float64(len(sparkBars)-1))
		b.WriteRune(sparkBars[index])
	}
	return paint(s.color).Render(b.String())
}

// ---------------------------------------------------------------- Badge

var badgeAttrs = []string{"label"}

// badge is a short label with breathing room, for a status chip or a count. Its
// colours come from the element's own style attributes, so a badge is themed the
// same way anything else is.
type badge struct {
	label   string
	measure widget.Measurer
}

func newBadge(ctx widget.Context) (widget.Native, error) {
	return &badge{label: " " + ctx.Attrs.String("label", "") + " ", measure: ctx.Measure}, nil
}

func (b *badge) Measure(_, _ int) (int, int) { return b.measure.Width(b.label), 1 }

func (b *badge) Render(_, _ int) string { return b.label }
