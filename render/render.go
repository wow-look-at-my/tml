package render

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/wow-look-at-my/tml/layout"
	"github.com/wow-look-at-my/tml/sema"
	"github.com/wow-look-at-my/tml/widget"
)

// marginZero is the absent margin; setting a nothing margin on a style is harmless but pointless, so it is skipped.
var marginZero sema.Thickness

// Render composes a laid-out tree into terminal output. Composition is bottom-up.
func Render(box *layout.Box) string {
	if box.Visibility == layout.Collapsed {
		// A collapsed element gave its slot back at measure time, so it draws nothing at all: not its text, not a border.
		return ""
	}
	if box.Name == "#text" {
		return box.Text
	}
	if box.Visibility == layout.Hidden {
		// A hidden element keeps the space it was given and paints none of it, border and fill included.
		return blank(box.Rect.W, box.Rect.H)
	}

	content := box.Text
	switch {
	case box.Name == "Text" && box.Overflow != "" && box.Overflow != layout.OverflowWrap:
		// Cut each line here, not with MaxWidth: MaxWidth cuts without marking the cut.
		content = clip(box.Text, box.Content.W, box.Overflow)
	case box.Native != nil && len(box.Children) == 0:
		// The widget draws itself into the space layout settled on. A widget with children is a composer and goes the other
		content = box.Native.Render(box.Content.W, box.Content.H)
	case box.Name != "Text":
		content = renderChildren(box)
	}

	st := box.Style.Style
	margin := box.Style.Margin
	if margin != marginZero {
		st = st.Margin(margin.Top, margin.Right, margin.Bottom, margin.Left)
	}

	// Width and Height are the border box: what is left of the rect as soon as margin is removed.
	w := box.Rect.W - margin.Horizontal()
	h := box.Rect.H - margin.Vertical()
	if w > 0 {
		st = st.Width(w)
	}
	if h > 0 {
		st = st.Height(h)
	}

	// A container's content is already composed, so overflow has to be clipped.
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

// clip cuts each line of a Text to width, and an ellipsis costs the line its last cell.
func clip(text string, width int, overflow layout.Overflow) string {
	if width <= 0 {
		return text
	}
	tail := ""
	if overflow == layout.OverflowEllipsis {
		tail = "…"
	}
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = ansi.Truncate(line, width, tail)
	}
	return strings.Join(lines, "\n")
}

func renderChildren(box *layout.Box) string {
	if len(box.Children) == 0 {
		return ""
	}

	// A collapsed child contributes no area, so the joins below drop it while parts stay aligned with box.Children.
	parts := make([]string, 0, len(box.Children))
	for _, child := range box.Children {
		parts = append(parts, Render(child))
	}

	if composer := box.Composer(); composer != nil {
		// The widget draws itself around its children, which reach it grouped by the slot they were written into.
		slots := widget.Slots{}
		for i, child := range box.Children {
			if child.Visibility == layout.Collapsed {
				continue
			}
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
		// A decorator holds a single child; a collapsed one would otherwise join in as a blank line of its own.
		held := parts[:0:0]
		for i, child := range box.Children {
			if child.Visibility == layout.Collapsed {
				continue
			}
			held = append(held, parts[i])
		}
		return lipgloss.JoinVertical(lipgloss.Left, held...)
	}
}

// compose places children at the coordinates arrange gave them.
func compose(box *layout.Box, parts []string) string {
	layers := make([]*lipgloss.Layer, 0, len(parts))
	for i, child := range box.Children {
		// A child with no area is in a collapsed column, or in a track sized to nothing. It draws nothing.
		if child.Rect.W == 0 || child.Rect.H == 0 {
			continue
		}
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

	// A child measured to nothing on the stacking axis (an items control with no items, or a component whose own body drew
	kept := parts[:0:0]
	for i, child := range box.Children {
		size := child.Rect.W
		if vertical {
			size = child.Rect.H
		}
		if size == 0 {
			continue
		}
		kept = append(kept, parts[i])
	}
	parts = kept

	if len(parts) == 0 {
		// Every child measured to nothing: the stack has nothing to draw and nothing to put a gap between.
		return ""
	}

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

// gapFiller is the blank run between a pair of stacked children: blank lines down the page, blank columns across it.
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
