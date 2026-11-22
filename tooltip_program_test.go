package tml_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml"
	"github.com/wow-look-at-my/tml/input"
	"github.com/wow-look-at-my/tml/sema"
)

// rest is the bytes a terminal sends when the pointer moves onto a cell with no button held. The cell counts from one.
const restOnButton = "\x1b[<35;3;3M"

func tooltipShows(run *typed) func() bool {
	return func() bool { return onScreen(run.out.String(), " Starts the run ") }
}

// A view draws the tooltip of the control the pointer rests on, with no program around it.
func TestAViewDrawsTheTooltipOfItsControl(t *testing.T) {
	view := loadDoc(t, programDoc)
	props := tml.Props{"title": sema.StringValue("hello")}
	_, err := view.Draw(props, programWidth, programHeight)
	require.NoError(t, err)

	ui := view.UI()
	button, ok := ui.Target("go")
	require.True(t, ok)
	assert.Equal(t, "Starts the run", button.Tooltip)
	x, y := button.Rect.X+2, button.Rect.Y+1

	ui.Update(tml.Mouse{X: x, Y: y, Action: input.Motion})
	dwell, waits := ui.Dwell()
	require.True(t, waits, "the pointer rests on the button at %d,%d of %+v", x, y, button.Rect)
	require.True(t, ui.ShowTooltip(dwell))

	frame, err := view.Draw(props, programWidth, programHeight)
	require.NoError(t, err)
	_, shown := ui.Tooltip()
	require.True(t, shown, "the frame kept the tooltip")
	assert.Contains(t, frame.Lines()[y+1], " Starts the run ", "the tooltip is on the row under the pointer")
	assert.Equal(t, view.Theme()["tooltip"], frame.At(x, y+1).Style.Bg, "in the theme's tooltip colour")
	assert.True(t, view.Theme()["tooltip"].IsSet())
}

// The pointer rests on the button, and the button's tooltip comes up after the delay. A key takes it away.
func TestAProgramShowsATooltipOnceThePointerRests(t *testing.T) {
	t.Serial("the process inspector takes this program's pointer handlers, and another test's program would replace them")
	exclusive(t)
	run := startTyped(t, tml.ProgramOptions{TooltipDelay: 20 * time.Millisecond})
	require.Eventually(t, func() bool { return onScreen(run.out.String(), "Go") }, 5*time.Second, 5*time.Millisecond)
	assert.False(t, tooltipShows(run)(), "nothing shows before the pointer comes")

	run.typeBytes(t, restOnButton)
	require.Eventually(t, tooltipShows(run), 5*time.Second, 5*time.Millisecond, "the tooltip shows after the rest")
	tip, shown := run.program.View().UI().Tooltip()
	require.True(t, shown)
	assert.Equal(t, tml.Tooltip{Text: "Starts the run", X: 2, Y: 2}, tip)

	run.typeBytes(t, "a")
	require.Eventually(t, func() bool { return !tooltipShows(run)() }, 5*time.Second, 5*time.Millisecond,
		"a key takes the tooltip away")

	run.program.Quit()
	assert.NoError(t, waitResult(t, run.result))
}

// keeper is an app that keeps every pointer value, so the focus ring sees none of them.
type keeper struct{ recordApp }

func (k keeper) Update(p *tml.Program, msg tml.Msg) bool {
	k.recordApp.Update(p, msg)
	_, pointer := msg.(tml.Mouse)
	if pointer {
		p.Unchanged()
	}
	return pointer
}

// An app that keeps the pointer for itself still gets tooltips, and a move still takes one away.
func TestATooltipDoesNotNeedTheRingToSeeThePointer(t *testing.T) {
	t.Serial("exclusive resets the process inspector and the drivable guard, which every running program shares")
	exclusive(t)
	run := startKept(t, tml.ProgramOptions{TooltipDelay: 20 * time.Millisecond})
	require.Eventually(t, func() bool { return onScreen(run.out.String(), "Go") }, 5*time.Second, 5*time.Millisecond)

	run.typeBytes(t, restOnButton)
	require.Eventually(t, tooltipShows(run), 5*time.Second, 5*time.Millisecond)

	run.typeBytes(t, "\x1b[<35;30;9M")
	require.Eventually(t, func() bool { return !tooltipShows(run)() }, 5*time.Second, 5*time.Millisecond,
		"the pointer left, and the app said the move changed nothing")
}

// A negative delay turns tooltips off.
func TestANegativeDelayShowsNoTooltip(t *testing.T) {
	t.Serial("a program adopted by the process inspector outlives a parallel test's reset of that inspector")
	exclusive(t)
	run := startTyped(t, tml.ProgramOptions{TooltipDelay: -1})
	require.Eventually(t, func() bool { return onScreen(run.out.String(), "Go") }, 5*time.Second, 5*time.Millisecond)

	run.typeBytes(t, restOnButton)
	waitFor[tml.Mouse](t, run.app.got)
	assert.Never(t, tooltipShows(run), 150*time.Millisecond, 10*time.Millisecond)
}
