package tml

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml/cell"
	"github.com/wow-look-at-my/tml/input"
	"github.com/wow-look-at-my/tml/layout"
)

// tipped is a control with a tooltip.
func tipped(id, tooltip string, x, y, w, h int) layout.Target {
	found := target(id, id, x, y, w, h)
	found.Tooltip = tooltip
	return found
}

func rest(u *UI, x, y int) { u.Update(input.Mouse{X: x, Y: y, Action: input.Motion}) }

func TestATooltipShowsOnceThePointerHasRested(t *testing.T) {
	u := NewUI()
	ring(u, tipped("save", "Writes the file", 0, 0, 8, 1), target("quit", "quit", 0, 1, 8, 1))

	rest(u, 3, 0)
	dwell, waits := u.Dwell()
	assert.True(t, waits, "the pointer rests on a control with a tooltip")
	_, shown := u.Tooltip()
	assert.False(t, shown, "nothing shows before the wait is over")

	assert.True(t, u.ShowTooltip(dwell))
	tip, shown := u.Tooltip()
	require.True(t, shown)
	assert.Equal(t, Tooltip{Text: "Writes the file", X: 3, Y: 0}, tip)
	assert.False(t, u.ShowTooltip(dwell), "a tooltip on screen does not come up a second time")
}

func TestAMoveDuringTheWaitStartsTheWaitAgain(t *testing.T) {
	u := NewUI()
	ring(u, tipped("save", "Writes the file", 0, 0, 8, 1))

	rest(u, 3, 0)
	stale, _ := u.Dwell()
	rest(u, 4, 0)
	assert.False(t, u.ShowTooltip(stale), "the pointer moved during the wait")

	fresh, waits := u.Dwell()
	assert.True(t, waits)
	assert.True(t, u.ShowTooltip(fresh))
}

func TestEveryInputTakesATooltipAway(t *testing.T) {
	inputs := map[string]any{
		"a move":  input.Mouse{X: 4, Y: 0, Action: input.Motion},
		"a press": input.Mouse{X: 3, Y: 0, Action: input.Press, Button: input.ButtonLeft},
		"a wheel": input.Mouse{X: 3, Y: 0, Action: input.Wheel, Button: input.WheelDown},
		"a key":   input.ParseKey("a"),
		"a paste": input.Paste{Text: "x"},
	}
	for name, msg := range inputs {
		t.Run(name, func(t *testing.T) {
			u := NewUI()
			ring(u, tipped("save", "Writes the file", 0, 0, 8, 1))
			rest(u, 3, 0)
			dwell, _ := u.Dwell()
			require.True(t, u.ShowTooltip(dwell))

			assert.True(t, u.Observe(msg), "the tooltip went away, which needs a frame")
			_, shown := u.Tooltip()
			assert.False(t, shown)
			assert.False(t, u.Observe(msg), "with no tooltip on screen there is nothing to repaint")
		})
	}
	assert.False(t, NewUI().Observe(Resize{}), "a value that is not an input is not counted")
}

func TestATooltipNeedsAPointerAtRestOnAControlThatHasOne(t *testing.T) {
	u := NewUI()
	ring(u, tipped("save", "Writes the file", 0, 0, 8, 1), target("quit", "quit", 0, 1, 8, 1))

	rest(u, 3, 1)
	dwell, waits := u.Dwell()
	assert.False(t, waits, "this control has no tooltip")
	assert.False(t, u.ShowTooltip(dwell))

	rest(u, 30, 9)
	_, waits = u.Dwell()
	assert.False(t, waits, "the pointer is on nothing")

	u.Update(input.Mouse{X: 3, Y: 0, Action: input.Motion, Button: input.ButtonLeft})
	dwell, waits = u.Dwell()
	assert.False(t, waits, "a drag is not a rest")
	assert.False(t, u.ShowTooltip(dwell))

	rest(u, 3, 0)
	u.Update(input.ParseKey("a"))
	dwell, waits = u.Dwell()
	assert.False(t, waits, "a key after the move ends the rest")
	assert.False(t, u.ShowTooltip(dwell))
}

// A control on top hides the tooltip of the control under it, as it hides the control.
func TestTheControlOnTopDecidesTheTooltip(t *testing.T) {
	u := NewUI()
	ring(u, tipped("save", "Writes the file", 0, 0, 8, 1), pointerOnly("menu", "", 0, 0, 20, 5))

	rest(u, 3, 0)
	_, waits := u.Dwell()
	assert.False(t, waits)
}

func TestATooltipGoesWithItsControl(t *testing.T) {
	u := NewUI()
	ring(u, tipped("save", "Writes the file", 0, 0, 8, 1))
	rest(u, 3, 0)
	dwell, _ := u.Dwell()
	require.True(t, u.ShowTooltip(dwell))

	ring(u, tipped("save", "Writes the file", 0, 0, 8, 1))
	_, shown := u.Tooltip()
	assert.True(t, shown, "a frame that keeps the control keeps the tooltip")

	ring(u, tipped("save", "Writes a copy", 0, 0, 8, 1))
	_, shown = u.Tooltip()
	assert.False(t, shown, "a new text is a new tooltip, and it waits again")

	rest(u, 3, 0)
	dwell, _ = u.Dwell()
	require.True(t, u.ShowTooltip(dwell))
	ring(u)
	_, shown = u.Tooltip()
	assert.False(t, shown, "the control went away")
}

// shownAt puts a tooltip on screen with the pointer on a cell, and draws it on an empty frame.
func shownAt(t *testing.T, text string, x, y, w, h int) *cell.Buffer {
	t.Helper()
	u := NewUI()
	ring(u, tipped("all", text, 0, 0, w, h))
	rest(u, x, y)
	dwell, _ := u.Dwell()
	require.True(t, u.ShowTooltip(dwell))

	frame := cell.NewBuffer(w, h)
	u.drawTooltip(frame, cell.Palette{"tooltip": cell.RGB(1, 2, 3), "tooltipText": cell.RGB(4, 5, 6)}, nil)
	return frame
}

func TestATooltipIsAFilledStripUnderThePointer(t *testing.T) {
	frame := shownAt(t, "Save", 2, 1, 20, 5)

	assert.Equal(t, []string{"", "", "   Save", "", ""}, rowsOf(frame))
	for x := 2; x < 8; x++ {
		style := frame.At(x, 2).Style
		assert.Equal(t, cell.RGB(1, 2, 3), style.Bg, "cell %d has the tooltip's surface", x)
		assert.Equal(t, cell.RGB(4, 5, 6), style.Fg)
	}
	assert.False(t, frame.At(1, 2).Style.Bg.IsSet(), "the strip is as wide as the text and its padding")
	assert.False(t, frame.At(8, 2).Style.Bg.IsSet())
	assert.NotContains(t, frame.Text(), "╭", "a tooltip has no border, so it does not look like a menu")
}

func TestATooltipStaysOnTheScreen(t *testing.T) {
	right := shownAt(t, "Save", 19, 1, 20, 5)
	assert.Equal(t, "               Save", rowsOf(right)[2], "it moves left at the right edge")

	bottom := shownAt(t, "Save", 2, 4, 20, 5)
	assert.Equal(t, "   Save", rowsOf(bottom)[3], "it goes over the pointer at the bottom edge")

	long := shownAt(t, strings.Repeat("word ", 20), 0, 0, 30, 8)
	lines := rowsOf(long)
	assert.Equal(t, "", lines[0], "the pointer's own row stays clear")
	assert.Equal(t, " word word word word word", lines[1], "a long text wraps to the screen")
	assert.Equal(t, " word word word word word", lines[4])

	none := NewUI()
	blank := cell.NewBuffer(4, 2)
	none.drawTooltip(blank, cell.Palette{}, nil)
	assert.Equal(t, []string{"", ""}, rowsOf(blank), "with no tooltip nothing is drawn")
}

// rowsOf is the rows of a frame without the blanks at their ends.
func rowsOf(frame *cell.Buffer) []string {
	lines := frame.Lines()
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " ")
	}
	return lines
}
