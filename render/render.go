package render

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/wow-look-at-my/tml/layout"
	"github.com/wow-look-at-my/tml/sema"
	"github.com/wow-look-at-my/tml/widget"
)

// marginZero is the absent margin; setting a zero margin on a style is harmless
// but pointless, so it is skipped.
var marginZero sema.Thickness

// Render composes a laid-out tree into terminal output.
//
// Composition is bottom-up: every box renders its children to strings and then
// hands the result to its own lipgloss.Style, which adds padding, border and
// alignment. Because layout already sized each box to exactly the cells it was
// given, joining the pieces reproduces the arranged geometry.
//
// Margin goes back onto the style here and nowhere else. lipgloss treats the
// width set on a style as the border box and adds margin outside it, so the two
// only compose unambiguously at render time; layout keeps margin separate so it
// can reason about sizes.
func Render(box *layout.Box) string {
	if box.Name == "#text" {
		return box.Text
	}

	content := box.Text
	switch {
	case box.Native != nil && len(box.Children) == 0:
		// The widget draws itself into the space layout settled on. A widget with
		// children is a composer and goes the other way, so its children are drawn
		// before it wraps them.
		content = box.Native.Render(box.Content.W, box.Content.H)
	case box.Name != "Text":
		content = renderChildren(box)
	}

	st := box.Style.Style
	margin := box.Style.Margin
	if margin != marginZero {
		st = st.Margin(margin.Top, margin.Right, margin.Bottom, margin.Left)
	}

	// Width and Height are the border box: what is left of the rect once margin
	// is removed.
	w := box.Rect.W - margin.Horizontal()
	h := box.Rect.H - margin.Vertical()
	if w > 0 {
		st = st.Width(w)
	}
	if h > 0 {
		st = st.Height(h)
	}

	// A container's content is already composed, so overflow has to be clipped.
	// Width alone would make lipgloss WRAP the block, which folds a too-wide row
	// onto the next line and shreds the arranged geometry. Text is the one place
	// wrapping is wanted, so it keeps Width on its own.
	if box.Name != "Text" || box.Native != nil {
		if w > 0 {
			st = st.MaxWidth(w)
		}
		if h > 0 {
			st = st.MaxHeight(h)
		}
	}
	return st.Render(content)
}

func renderChildren(box *layout.Box) string {
	if len(box.Children) == 0 {
		return ""
	}

	parts := make([]string, 0, len(box.Children))
	for _, child := range box.Children {
		parts = append(parts, Render(child))
	}

	if composer := box.Composer(); composer != nil {
		// The widget draws itself around its children, which reach it grouped by
		// the slot they were written into.
		slots := widget.Slots{}
		for i, child := range box.Children {
			slots[child.Slot()] = append(slots[child.Slot()], parts[i])
		}
		return composer.Compose(slots, box.Content.W, box.Content.H)
	}

	switch box.Name {
	case "Stack":
		return joinStack(box, parts)
	case "Grid":
		return compose(box, parts)
	case "Canvas":
		return canvas(box, parts)
	default:
		// A decorator holds a single child in practice; stacking any extras
		// vertically keeps the output well-formed rather than silently dropping
		// them.
		return lipgloss.JoinVertical(lipgloss.Left, parts...)
	}
}

// compose places children at the coordinates arrange gave them, which joining
// cannot express once children sit on a grid rather than in a line.
//
// Every layer gets a distinct, increasing z in document order. The compositor
// sorts on each layer's own z with an unstable sort, so equal values would leave
// the paint order of overlapping cells undefined. See docs/lipgloss-contract.md.
func compose(box *layout.Box, parts []string) string {
	layers := make([]*lipgloss.Layer, 0, len(parts))
	for i, child := range box.Children {
		layers = append(layers, lipgloss.NewLayer(parts[i]).
			X(child.Rect.X).
			Y(child.Rect.Y).
			Z(i+1))
	}
	return lipgloss.NewCompositor(layers...).Render()
}

func joinStack(box *layout.Box, parts []string) string {
	gap := box.Gap()
	vertical := box.Vertical()

	if gap > 0 {
		spaced := make([]string, 0, len(parts)*2-1)
		for i, part := range parts {
			if i > 0 {
				spaced = append(spaced, gapFiller(box, gap, vertical))
			}
			spaced = append(spaced, part)
		}
		parts = spaced
	}

	if vertical {
		return lipgloss.JoinVertical(box.Style.Align, parts...)
	}
	return lipgloss.JoinHorizontal(box.Style.VAlign, parts...)
}

// gapFiller is the blank run between two stacked children: blank lines down the
// page, blank columns across it.
func gapFiller(box *layout.Box, gap int, vertical bool) string {
	if vertical {
		return strings.Repeat("\n", gap-1)
	}
	column := strings.Repeat(" ", gap)
	height := max(1, box.Content.H)
	lines := make([]string, height)
	for i := range lines {
		lines[i] = column
	}
	return strings.Join(lines, "\n")
}
