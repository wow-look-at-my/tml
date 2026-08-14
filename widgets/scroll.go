package widgets

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/wow-look-at-my/tml/widget"
)

var scrollboxAttrs = []string{"offset", "offsetX", "scrollbar"}

// scrollbox shows part of content that is taller than the space it has, with a
// scrollbar beside it when there is more than fits.
//
// The offset is a property, not state: the host owns how far it has scrolled, the
// same way it owns everything else a view is rendered from. Wheel events come
// back through the UI as Scrolled events for the host to act on.
type scrollbox struct {
	offsetX, offsetY int
	bar              string
}

func newScrollbox(ctx widget.Context) (widget.Native, error) {
	bar, err := ctx.Attrs.Enum("scrollbar", "auto", "auto", "always", "never")
	if err != nil {
		return nil, err
	}
	box := &scrollbox{bar: bar}
	if box.offsetY, err = ctx.Attrs.Int("offset", 0); err != nil {
		return nil, err
	}
	if box.offsetX, err = ctx.Attrs.Int("offsetX", 0); err != nil {
		return nil, err
	}
	box.offsetX = max(0, box.offsetX)
	box.offsetY = max(0, box.offsetY)
	return box, nil
}

// Arrange is the whole point of a scrolling region: the content is measured
// against unlimited height, so it is free to be taller than the hole it is seen
// through, while the region itself takes the full width on offer -- one that
// shrank to its content would put its scrollbar somewhere in the middle.
//
// The offset goes back to layout as well, so a control scrolled halfway off the
// top is clicked where it is drawn.
func (s *scrollbox) Arrange() widget.Layout {
	return widget.Layout{FreeH: true, FillW: true, OffsetX: s.offsetX, OffsetY: s.offsetY}
}

// Inset reserves the scrollbar column. Auto cannot know yet whether there is
// anything to scroll -- that needs the drawn content -- so it reserves the
// column and draws a blank one when there is not. Reflowing the content the
// moment it grew past the edge would be worse: everything would shift sideways.
func (s *scrollbox) Inset() (int, int) {
	if s.bar == "never" {
		return 0, 0
	}
	return 1, 0
}

func (s *scrollbox) Measure(maxW, maxH int) (int, int) { return maxW, maxH }

func (s *scrollbox) Render(w, h int) string { return s.Compose(nil, w, h) }

func (s *scrollbox) Compose(slots widget.Slots, w, h int) string {
	gutter, _ := s.Inset()
	viewW := max(0, w-gutter)

	content := lipgloss.JoinVertical(lipgloss.Left, slots.Default()...)
	all := strings.Split(content, "\n")

	// The offset is clamped to the content, so scrolling stops at the last
	// screenful instead of running off into blank space -- and a host that wants
	// the end of a growing transcript can just ask for a big number. Layout
	// clamps its copy the same way, or a click would land a line off.
	offset := clamp(s.offsetY, 0, max(0, len(all)-h))

	lines := window(all, offset, h)
	for i, line := range lines {
		lines[i] = columns(line, s.offsetX, viewW)
	}
	if gutter == 0 {
		return strings.Join(lines, "\n")
	}

	bar := s.scrollbar(offset, len(all), h)
	for i := range lines {
		if i < len(bar) {
			lines[i] += bar[i]
		}
	}
	return strings.Join(lines, "\n")
}

// scrollbar is the gutter column: a track with a thumb sized and placed in
// proportion to what is on screen.
func (s *scrollbox) scrollbar(offset, total, view int) []string {
	bar := make([]string, max(0, view))
	blank := " "
	if s.bar == "always" {
		blank = "│"
	}
	for i := range bar {
		bar[i] = blank
	}
	if view <= 0 || total <= view {
		return bar
	}

	for i := range bar {
		bar[i] = "│"
	}
	thumb := max(1, view*view/total)
	span := view - thumb
	reach := total - view
	start := 0
	if reach > 0 {
		start = min(span, max(0, offset*span/reach))
	}
	for i := start; i < start+thumb && i < view; i++ {
		bar[i] = "█"
	}
	return bar
}

// window takes height rows starting at offset, padding with blanks when the
// content is shorter than the space. The offset is already clamped by the
// caller: how many lines the content wraps to depends on the width the widget
// was given, so the end is a number only this side can work out.
func window(lines []string, offset, height int) []string {
	if height <= 0 {
		return nil
	}
	out := make([]string, 0, height)
	for i := offset; i < offset+height; i++ {
		if i >= 0 && i < len(lines) {
			out = append(out, lines[i])
			continue
		}
		out = append(out, "")
	}
	return out
}

// columns takes width cells starting at offset, measured in display cells so an
// escape sequence is carried along rather than counted.
func columns(line string, offset, width int) string {
	if width <= 0 {
		return ""
	}
	cut := ansi.Cut(line, offset, offset+width)
	if gap := width - lipgloss.Width(cut); gap > 0 {
		cut += strings.Repeat(" ", gap)
	}
	return cut
}

// clamp keeps n inside [low, high].
func clamp(n, low, high int) int { return max(low, min(n, high)) }
