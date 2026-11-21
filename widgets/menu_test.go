package widgets

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml/cell"
	"github.com/wow-look-at-my/tml/widget"
	"github.com/wow-look-at-my/tml/widget/widgettest"
)

// drawn is a menu painted at the size it asks for.
func drawn(t *testing.T, pairs map[string]string, maxW int) (lines []string, buf *cell.Buffer) {
	t.Helper()
	m := build(t, "Menu", pairs)
	w, h := m.Measure(maxW, 0)
	buf = widgettest.Draw(m, w, h)
	return buf.Lines(), buf
}

func TestAMenuIsAsWideAsItsLongestLine(t *testing.T) {
	lines, _ := drawn(t, map[string]string{"title": "pane", "items": "Zoom,Even out panes,Close"}, 0)
	assert.Equal(t, []string{
		"╭ pane ──────────╮",
		"│ Zoom           │",
		"│ Even out panes │",
		"│ Close          │",
		"╰────────────────╯",
	}, lines)

	lines, _ = drawn(t, map[string]string{"title": "a title longer than the item", "items": "Zoom"}, 0)
	assert.Equal(t, "╭ a title longer than the item ╮", lines[0], "a longer title sets the width")

	lines, _ = drawn(t, map[string]string{"items": "one"}, 0)
	assert.Equal(t, []string{"╭─────╮", "│ one │", "╰─────╯"}, lines, "a menu needs no title")
}

func TestAMenuStopsAtItsCapAndCutsALongerChoice(t *testing.T) {
	long := strings.Repeat("w", 90)
	m := build(t, "Menu", map[string]string{"items": long})
	w, _ := m.Measure(0, 0)
	assert.Equal(t, menuMaxWidth, w, "the cap when the element names none")

	lines, _ := drawn(t, map[string]string{"items": long, "maxWidth": "12"}, 0)
	assert.Equal(t, "│ wwwwwww… │", lines[1])

	lines, _ = drawn(t, map[string]string{"items": long}, 10)
	assert.Len(t, []rune(lines[0]), 10, "the room on offer is a limit too")

	_, err := tryBuild("Menu", map[string]string{"items": "a", "maxWidth": "4"})
	require.ErrorContains(t, err, "maxWidth")
}

func TestAMenuMarksTheSelectedChoiceFromBorderToBorder(t *testing.T) {
	pairs := map[string]string{"items": "one,two", "selected": "1"}
	_, buf := drawn(t, pairs, 0)
	assert.True(t, buf.At(1, 2).Style.Has(cell.Reverse), "the padding is marked")
	assert.True(t, buf.At(5, 2).Style.Has(cell.Reverse))
	assert.False(t, buf.At(0, 2).Style.Has(cell.Reverse), "the border is not")
	assert.False(t, buf.At(2, 1).Style.Has(cell.Reverse), "and no other choice is")

	pairs["accent"], pairs["accentText"] = "#ff0000", "#ffffff"
	_, buf = drawn(t, pairs, 0)
	marked := buf.At(2, 2).Style
	assert.False(t, marked.Has(cell.Reverse), "the accent colours take the place of the reversal")
	assert.Equal(t, cell.RGB(255, 0, 0), marked.Bg)
	assert.Equal(t, cell.RGB(255, 255, 255), marked.Fg)
}

func TestAMenuNamesTheChoiceUnderAPoint(t *testing.T) {
	m := build(t, "Menu", map[string]string{"items": "one,two"}).(widget.Parted)
	assert.Equal(t, "", m.Part(3, 0), "the top border is no choice")
	assert.Equal(t, "item:0", m.Part(3, 1))
	assert.Equal(t, "item:1", m.Part(3, 2))
	assert.Equal(t, "", m.Part(3, 3))
}

// A canvas moves a menu back inside. The layout tests cover the move.
func TestAMenuAsksItsCanvasToKeepItWhole(t *testing.T) {
	assert.True(t, build(t, "Menu", nil).(widget.Contained).Contained())
}
