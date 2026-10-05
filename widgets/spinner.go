package widgets

import (
	"github.com/wow-look-at-my/tml/widget"
)

var spinnerAttrs = []string{"kind", "color"}

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

// spinner draws the frame its animation is on: the engine counts frames and decides what is on screen.
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
	frames := spinners[kind]
	// A frame counter only goes up, so the walk through this kind's frames has to wrap here.
	frame := ((ctx.Frame % len(frames)) + len(frames)) % len(frames)
	return &spinner{
		frames:  frames,
		frame:   frame,
		color:   ctx.Attrs.String("color", ""),
		measure: ctx.Measure,
	}, nil
}

// Animating reports that a spinner keeps moving for as long as it is visible.
func (s *spinner) Animating() bool { return true }

func (s *spinner) Measure(_, _ int) (int, int) {
	return s.measure.Width(s.frames[s.frame]), 1
}

func (s *spinner) Render(_, _ int) string {
	return paint(s.color).Render(s.frames[s.frame])
}
