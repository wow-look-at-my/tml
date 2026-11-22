package tml_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml"
)

// rest is the bytes a terminal sends when the pointer moves onto a cell with no button held. The cell counts from one.
const restOnButton = "\x1b[<35;3;3M"

func tooltipShows(run *typed) func() bool {
	return func() bool { return onScreen(run.out.String(), " Starts the run ") }
}

// The pointer rests on the button, and the button's tooltip comes up after the delay. A key takes it away.
func TestAProgramShowsATooltipOnceThePointerRests(t *testing.T) {
	t.Serial("the program wires its key and click handlers into the process inspector while it reads the pipe")
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
	t.Serial("the program wires its key and click handlers into the process inspector while it reads the pipe")
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
	t.Serial("the program wires its key and click handlers into the process inspector while it reads the pipe")
	exclusive(t)
	run := startTyped(t, tml.ProgramOptions{TooltipDelay: -1})
	require.Eventually(t, func() bool { return onScreen(run.out.String(), "Go") }, 5*time.Second, 5*time.Millisecond)

	run.typeBytes(t, restOnButton)
	waitFor[tml.Mouse](t, run.app.got)
	assert.Never(t, tooltipShows(run), 150*time.Millisecond, 10*time.Millisecond)
}
