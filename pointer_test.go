package tml_test

import _ "embed"

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml"
	"github.com/wow-look-at-my/tml/input"
)

//go:embed testdata/pointer-table.xml
var pointerTable string

func mouse(action input.Action, button input.Button, x, y int) input.Mouse {
	return input.Mouse{Action: action, Button: button, X: x, Y: y}
}

// A right click asks for a context menu, and says which part of the table it landed on.
func TestARightClickAsksForAMenuAndNamesThePart(t *testing.T) {
	loaded := interactive(t, pointerTable, 20, 4)
	grid := targetOf(t, loaded, "grid")

	events := loaded.UI().Update(mouse(input.Press, input.ButtonRight, grid.Box.X, grid.Box.Y+3))
	require.Len(t, events, 1)
	assert.Equal(t, tml.Menu, events[0].Kind)
	assert.Equal(t, "grid", events[0].ID)
	assert.Equal(t, "row:1", events[0].Part, "line 3 is under the header and its rule")
}

// A left click names its part too, so a host can tell a heading from a row.
func TestAClickOnAHeadingNamesTheColumn(t *testing.T) {
	loaded := interactive(t, pointerTable, 20, 4)
	grid := targetOf(t, loaded, "grid")
	last := grid.Box.X + grid.Box.W - 1

	loaded.UI().Update(mouse(input.Press, input.ButtonLeft, last, grid.Box.Y))
	events := loaded.UI().Update(mouse(input.Release, input.ButtonLeft, last, grid.Box.Y))
	require.Len(t, events, 1)
	assert.Equal(t, tml.Activated, events[0].Kind)
	assert.Equal(t, "header:1", events[0].Part)
}

// The modifier keys held with a click reach the host, so it can extend a selection.
func TestAClickCarriesItsModifiers(t *testing.T) {
	loaded := interactive(t, pointerTable, 20, 4)
	grid := targetOf(t, loaded, "grid")

	click := mouse(input.Press, input.ButtonLeft, grid.Box.X, grid.Box.Y+2)
	click.Mod = input.Ctrl
	loaded.UI().Update(click)
	click.Action = input.Release
	events := loaded.UI().Update(click)
	require.Len(t, events, 1)
	assert.Equal(t, input.Ctrl, events[0].Mod)
	assert.Equal(t, "row:0", events[0].Part)
}

// Dragging the thumb scrolls by the rows the content has to move, as Scrolled events, and releasing ends the drag
// without a click.
func TestDraggingTheScrollbarScrolls(t *testing.T) {
	loaded := interactive(t, pointerTable, 20, 4)
	list := targetOf(t, loaded, "list")
	require.Equal(t, 8, list.Scroll.MaxY, "twelve lines seen through four")
	bar := list.Box.X + list.Box.W - 1

	events := loaded.UI().Update(mouse(input.Press, input.ButtonLeft, bar, list.Box.Y))
	assert.Empty(t, events, "grabbing the thumb where it sits moves nothing")

	events = loaded.UI().Update(mouse(input.Motion, input.ButtonLeft, bar, list.Box.Y+3))
	require.Len(t, events, 1)
	assert.Equal(t, tml.Scrolled, events[0].Kind)
	assert.Equal(t, "list", events[0].ID)
	assert.Equal(t, 8, events[0].Delta, "the thumb at the bottom is the end of the content")

	events = loaded.UI().Update(mouse(input.Motion, input.ButtonLeft, bar+5, list.Box.Y+3))
	assert.Empty(t, events, "the pointer leaving the bar sideways moves nothing")

	events = loaded.UI().Update(mouse(input.Motion, input.ButtonLeft, bar, list.Box.Y))
	require.Len(t, events, 1)
	assert.Equal(t, -8, events[0].Delta, "each move is relative to the last one the drag asked for")

	events = loaded.UI().Update(mouse(input.Release, input.ButtonLeft, bar, list.Box.Y))
	assert.Empty(t, events)
	events = loaded.UI().Update(mouse(input.Motion, input.ButtonNone, bar, list.Box.Y+3))
	assert.Empty(t, events, "the drag ends with the button")
}

// A press on the track jumps the thumb under the pointer.
func TestAPressOnTheTrackJumps(t *testing.T) {
	loaded := interactive(t, pointerTable, 20, 4)
	list := targetOf(t, loaded, "list")
	bar := list.Box.X + list.Box.W - 1

	events := loaded.UI().Update(mouse(input.Press, input.ButtonLeft, bar, list.Box.Y+3))
	require.Len(t, events, 1)
	assert.Equal(t, tml.Scrolled, events[0].Kind)
	assert.Positive(t, events[0].Delta)
}

func TestTheMenuKindIsNamed(t *testing.T) {
	assert.Equal(t, "menu", tml.Menu.String())
}
