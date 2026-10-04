package widgets

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/wow-look-at-my/tml/widget"
)

var scrollboxAttrs = []string{"offset", "offsetX", "scrollbar", "contentHeight", "fade", "moreAbove", "moreBelow"}

// scrollbox shows part of content that is taller than the space it has, with a scrollbar beside it when there is more
type scrollbox struct {
	offsetX, offsetY int
	contentHeight    int
	bar              string
	fade             bool
	moreAbove        bool
	moreBelow        bool
	dark             bool
	measure          widget.Measurer
}

func newScrollbox(ctx widget.Context) (widget.Native, error) {
	bar, err := ctx.Attrs.Enum("scrollbar", "auto", "auto", "always", "never")
	if err != nil {
		return nil, err
	}
	box := &scrollbox{bar: bar, dark: ctx.Dark, measure: ctx.Measure}
	if box.fade, err = ctx.Attrs.Bool("fade", false); err != nil {
		return nil, err
	}
	if box.moreAbove, err = ctx.Attrs.Bool("moreAbove", false); err != nil {
		return nil, err
	}
	if box.moreBelow, err = ctx.Attrs.Bool("moreBelow", false); err != nil {
		return nil, err
	}
	if box.offsetY, err = ctx.Attrs.Int("offset", 0); err != nil {
		return nil, err
	}
	if box.offsetX, err = ctx.Attrs.Int("offsetX", 0); err != nil {
		return nil, err
	}
	if box.contentHeight, err = ctx.Attrs.Int("contentHeight", 0); err != nil {
		return nil, err
	}
	box.offsetX = max(0, box.offsetX)
	box.offsetY = max(0, box.offsetY)
	box.contentHeight = max(0, box.contentHeight)
	return box, nil
}

// Arrange is the whole point of a scrolling region: the content is measured against unlimited height.
func (s *scrollbox) Arrange() widget.Layout {
	return widget.Layout{
		FreeH:    true,
		FillW:    true,
		OffsetX:  s.offsetX,
		OffsetY:  s.offsetY,
		ContentH: s.contentHeight,
	}
}

// Inset reserves the scrollbar column.
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

	// The offset is clamped to the content, so scrolling stops at the last screenful instead of running off into blank
	total := len(all)
	offset := clamp(s.offsetY, 0, max(0, total-h))
	from := offset
	if s.contentHeight > 0 {
		// The children ARE the window: the host cut it at the offset.
		total = s.contentHeight
		offset = clamp(s.offsetY, 0, max(0, total-h))
		from = 0
	}

	lines := window(all, from, h)
	for i, line := range lines {
		lines[i] = columns(line, s.offsetX, viewW, s.measure)
	}
	if s.fade {
		fadeEdges(lines, s.moreAbove || offset > 0, s.moreBelow || offset+h < total, h, s.dark)
	}
	if gutter == 0 {
		return strings.Join(lines, "\n")
	}

	bar := s.scrollbar(offset, total, h)
	for i := range lines {
		if i < len(bar) {
			lines[i] += bar[i]
		}
	}
	return strings.Join(lines, "\n")
}

// scrollbar is the gutter column: a track with a thumb sized and placed in proportion to what is on screen.
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

// fadeTall is the viewport height at which an edge fade widens to a couple of rows. One row is enough to show a short region
const fadeTall = 8

// fadeRows is how many rows at each edge a viewport of the given height fades. Both bands never meet, so a window
func fadeRows(height int) int {
	rows := 1
	if height >= fadeTall {
		rows = 2
	}
	return min(rows, max(0, (height-1)/2))
}

// fadeEdges blends the rows at the ends of the window toward the background when there is content beyond them.
func fadeEdges(lines []string, above, below bool, height int, dark bool) {
	rows := fadeRows(height)
	beyond := min(rows, len(lines))
	// Depth counts inward from the edge, so the row at the edge is the faintest of the band.
	depth := make([]int, len(lines))
	if above {
		for i := range beyond {
			depth[i] = rows - i
		}
	}
	if below {
		for i := range beyond {
			j := len(depth) - 1 - i
			depth[j] = max(depth[j], rows-i)
		}
	}
	for i, d := range depth {
		if d > 0 {
			lines[i] = fadeRow(lines[i], d, rows, dark)
		}
	}
}

// window takes height rows starting at offset, padding with blanks when the content is shorter than the space.
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

// columns takes width cells starting at offset, measured in display cells so an escape sequence is carried along
func columns(line string, offset, width int, measure widget.Measurer) string {
	if width <= 0 {
		return ""
	}
	cut := ansi.Cut(line, offset, offset+width)
	if gap := width - measure.Width(cut); gap > 0 {
		cut += strings.Repeat(" ", gap)
	}
	return cut
}

// clamp keeps n inside [low, high].
func clamp(n, low, high int) int { return max(low, min(n, high)) }
