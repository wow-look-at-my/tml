package tml

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml/layout"
	"github.com/wow-look-at-my/tml/widget"
)

// ring publishes a frame the way layout does, so the UI has geometry to resolve
// against.
func ring(u *UI, targets ...layout.Target) {
	u.Frame(targets)
}

func target(id, action string, x, y, w, h int) layout.Target {
	return layout.Target{ID: id, Action: action, Focus: true, Rect: layout.Rect{X: x, Y: y, W: w, H: h}}
}

// pointerOnly is an element the pointer can reach and the keyboard cannot, which
// is what a scrolling region is.
func pointerOnly(id, action string, x, y, w, h int) layout.Target {
	found := target(id, action, x, y, w, h)
	found.Focus = false
	return found
}

// state is how the element at index draws in the frame the UI has published,
// which is what layout asks for before it measures anything.
func state(u *UI, index int) widget.State {
	return u.States(u.Targets())[index]
}

func press(u *UI, stroke string) []Event {
	return u.key(stroke)
}

// Nothing has been focused on the first frame, so the ring starts at its first
// control. A view whose buttons are all unfocused to begin with looks broken.
func TestFocusStartsOnTheFirstControl(t *testing.T) {
	u := NewUI()

	states := u.States([]layout.Target{
		pointerOnly("log", "", 0, 0, 8, 4),
		target("save", "save", 0, 5, 8, 1),
		target("quit", "quit", 0, 6, 8, 1),
	})
	assert.False(t, states[0].Focused, "the keyboard never starts somewhere it cannot go")
	assert.True(t, states[1].Focused)
	assert.False(t, states[2].Focused)
}

func TestTabMovesAndWraps(t *testing.T) {
	u := NewUI()
	ring(u, target("save", "save", 0, 0, 8, 1), target("quit", "quit", 0, 1, 8, 1))

	events := press(u, "tab")
	require.Len(t, events, 1)
	assert.Equal(t, Event{Kind: FocusMoved, ID: "quit", Action: "quit"}, events[0])
	assert.True(t, state(u, 1).Focused)

	press(u, "tab")
	id, _ := u.Focused()
	assert.Equal(t, "save", id, "the ring wraps rather than dead-ending")

	press(u, "shift+tab")
	id, _ = u.Focused()
	assert.Equal(t, "quit", id)
}

func TestEnterAndSpaceActivateTheFocusedControl(t *testing.T) {
	u := NewUI()
	ring(u, target("save", "save-doc", 0, 0, 8, 1))

	for _, stroke := range []string{"enter", "space"} {
		events := press(u, stroke)
		require.Len(t, events, 1, stroke)
		assert.Equal(t, Event{Kind: Activated, ID: "save", Action: "save-doc"}, events[0])
	}
}

func TestArrowsMoveTheRingToo(t *testing.T) {
	u := NewUI()
	ring(u, target("a", "", 0, 0, 4, 1), target("b", "", 5, 0, 4, 1))

	press(u, "right")
	id, _ := u.Focused()
	assert.Equal(t, "b", id)

	press(u, "up")
	id, _ = u.Focused()
	assert.Equal(t, "a", id)
}

func TestUnboundKeysAreIgnored(t *testing.T) {
	u := NewUI()
	ring(u, target("a", "", 0, 0, 4, 1))

	assert.Empty(t, press(u, "ctrl+c"))
}

// A host that needs the arrows takes them back, and the ring must stop using
// them the moment it does.
func TestKeyMapIsReplaceable(t *testing.T) {
	u := NewUI()
	u.SetKeyMap(KeyMap{Next: []string{"n"}, Prev: []string{"p"}, Activate: []string{"x"}})
	ring(u, target("a", "", 0, 0, 4, 1), target("b", "", 0, 1, 4, 1))

	assert.Empty(t, press(u, "tab"), "tab is no longer bound")
	press(u, "n")
	id, _ := u.Focused()
	assert.Equal(t, "b", id)
}

func TestPointerHoversAndActivatesOnRelease(t *testing.T) {
	u := NewUI()
	ring(u, target("save", "save", 0, 0, 8, 1), target("quit", "quit", 0, 2, 8, 1))

	u.Update(tea.MouseMotionMsg{X: 3, Y: 2})
	assert.True(t, state(u, 1).Hovered)
	assert.False(t, state(u, 0).Hovered)

	u.Update(tea.MouseClickMsg{X: 3, Y: 2, Button: tea.MouseLeft})
	assert.True(t, state(u, 1).Pressed)
	id, _ := u.Focused()
	assert.Equal(t, "quit", id, "clicking takes focus as well")

	events := u.Update(tea.MouseReleaseMsg{X: 3, Y: 2, Button: tea.MouseLeft})
	require.Len(t, events, 1)
	assert.Equal(t, Activated, events[0].Kind)
	assert.Equal(t, "quit", events[0].ID)
	assert.False(t, state(u, 1).Pressed)
}

// Releasing somewhere else takes the click back, which is what every other
// button in every other toolkit does.
func TestReleasingElsewhereCancels(t *testing.T) {
	u := NewUI()
	ring(u, target("save", "save", 0, 0, 8, 1), target("quit", "quit", 0, 2, 8, 1))

	u.Update(tea.MouseClickMsg{X: 1, Y: 0, Button: tea.MouseLeft})
	assert.Empty(t, u.Update(tea.MouseReleaseMsg{X: 1, Y: 2, Button: tea.MouseLeft}))
	assert.Empty(t, u.Update(tea.MouseReleaseMsg{X: 40, Y: 40, Button: tea.MouseLeft}))
}

func TestClickingNothingDoesNothing(t *testing.T) {
	u := NewUI()
	ring(u, target("save", "save", 0, 0, 8, 1))

	assert.Empty(t, u.Update(tea.MouseClickMsg{X: 30, Y: 30, Button: tea.MouseLeft}))
	assert.Empty(t, u.Update(tea.MouseClickMsg{X: 1, Y: 0, Button: tea.MouseRight}))
}

// Overlapping controls are painted in document order, so the later one is the
// one the user can see and therefore the one they clicked.
func TestTheTopmostControlWinsAClick(t *testing.T) {
	u := NewUI()
	ring(u, target("under", "", 0, 0, 10, 3), target("over", "", 2, 1, 4, 1))

	u.Update(tea.MouseClickMsg{X: 3, Y: 1, Button: tea.MouseLeft})
	id, _ := u.Focused()
	assert.Equal(t, "over", id)
}

func TestWheelReportsScrollOverAControl(t *testing.T) {
	u := NewUI()
	ring(u, target("log", "scroll-log", 0, 0, 10, 5))

	events := u.Update(tea.MouseWheelMsg{X: 1, Y: 1, Button: tea.MouseWheelDown})
	require.Len(t, events, 1)
	assert.Equal(t, Event{Kind: Scrolled, ID: "log", Action: "scroll-log", Delta: 1}, events[0])

	events = u.Update(tea.MouseWheelMsg{X: 1, Y: 1, Button: tea.MouseWheelUp})
	require.Len(t, events, 1)
	assert.Equal(t, -1, events[0].Delta)

	assert.Empty(t, u.Update(tea.MouseWheelMsg{X: 40, Y: 40, Button: tea.MouseWheelUp}))
}

// A control can vanish between frames -- a popup closes, an `if` flips. Focus
// falls back to the first rather than being stranded on something not drawn.
func TestFocusRecoversWhenItsControlDisappears(t *testing.T) {
	u := NewUI()
	ring(u, target("save", "", 0, 0, 8, 1), target("quit", "", 0, 1, 8, 1))
	press(u, "tab")

	ring(u, target("save", "", 0, 0, 8, 1))
	id, _ := u.Focused()
	assert.Equal(t, "save", id)

	ring(u)
	id, _ = u.Focused()
	assert.Empty(t, id)
	assert.Empty(t, press(u, "tab"))
}

// Without an id a control is known by where it sits in the ring, which is all
// there is to go on.
func TestControlsWithoutIdsStillFocus(t *testing.T) {
	u := NewUI()
	ring(u, target("", "first", 0, 0, 4, 1), target("", "second", 0, 1, 4, 1))

	press(u, "tab")
	_, action := u.Focused()
	assert.Equal(t, "second", action)
}

func TestFocusByIDReportsWhetherItLanded(t *testing.T) {
	u := NewUI()
	ring(u, target("save", "", 0, 0, 8, 1), target("quit", "", 0, 1, 8, 1))

	assert.True(t, u.Focus("quit"))
	id, _ := u.Focused()
	assert.Equal(t, "quit", id)

	assert.False(t, u.Focus("nope"))
	assert.False(t, u.Focus(""))
	assert.Len(t, u.Targets(), 2)
}

// A scrolling region answers the wheel without ever being a tab stop: the
// pointer reaches everything the frame published, the keyboard only the part of
// it that can do something with a key.
func TestThePointerReachesWhatTheKeyboardSkips(t *testing.T) {
	u := NewUI()
	ring(u, target("save", "save", 0, 0, 8, 1), pointerOnly("log", "scroll-log", 0, 2, 10, 5))

	assert.Empty(t, press(u, "tab"), "there is nowhere else for the keyboard to go")
	id, _ := u.Focused()
	assert.Equal(t, "save", id)

	events := u.Update(tea.MouseWheelMsg{X: 2, Y: 3, Button: tea.MouseWheelDown})
	require.Len(t, events, 1)
	assert.Equal(t, Event{Kind: Scrolled, ID: "log", Action: "scroll-log", Delta: 1}, events[0])
}

// Clicking one leaves the keyboard where it was rather than sending focus
// somewhere it cannot come back from.
func TestClickingAPointerOnlyElementDoesNotMoveFocus(t *testing.T) {
	u := NewUI()
	ring(u, target("save", "save", 0, 0, 8, 1), pointerOnly("log", "", 0, 2, 10, 5))

	assert.Empty(t, u.Update(tea.MouseClickMsg{X: 2, Y: 3, Button: tea.MouseLeft}))
	id, _ := u.Focused()
	assert.Equal(t, "save", id)
	assert.False(t, u.Focus("log"), "it never takes focus by name either")
}

func TestEventKindsAreNamed(t *testing.T) {
	assert.Equal(t, "activated", Activated.String())
	assert.Equal(t, "focus-moved", FocusMoved.String())
	assert.Equal(t, "scrolled", Scrolled.String())
	assert.Equal(t, "invalid", EventKind(9).String())
}
