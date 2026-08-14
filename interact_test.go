package tml_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml"
)

// buttons is a row of three controls, the middle one disabled.
const buttons = `<Stack orientation="horizontal" gap="1">
	<Button id="save" action="save" label="Save"/>
	<Button id="locked" label="Locked" disabled="true"/>
	<Button id="quit" action="quit" label="Quit"/>
</Stack>`

// interactive loads a view and renders one frame, which is what publishes the
// geometry the UI resolves clicks against.
func interactive(t *testing.T, template string, w, h int) *tml.View {
	t.Helper()
	loaded, err := view(t, app(template), tml.Options{})
	require.NoError(t, err)
	_, err = loaded.Render(nil, w, h)
	require.NoError(t, err)
	return loaded
}

func key(t *testing.T, loaded *tml.View, stroke string, w, h int) []tml.Event {
	t.Helper()
	events := loaded.UI().Update(tea.KeyPressMsg{Code: strokeCode(stroke), Mod: strokeMod(stroke)})
	_, err := loaded.Render(nil, w, h)
	require.NoError(t, err)
	return events
}

func strokeCode(stroke string) rune {
	switch strings.TrimPrefix(stroke, "shift+") {
	case "tab":
		return tea.KeyTab
	case "enter":
		return tea.KeyEnter
	default:
		return ' '
	}
}

func strokeMod(stroke string) tea.KeyMod {
	if strings.HasPrefix(stroke, "shift+") {
		return tea.ModShift
	}
	return 0
}

// Focus starts on the first control and tab walks the row, skipping the
// disabled one: tab must never stop somewhere that would do nothing.
func TestTabWalksTheRealFocusRing(t *testing.T) {
	loaded := interactive(t, buttons, 40, 3)

	id, _ := loaded.UI().Focused()
	assert.Equal(t, "save", id)
	require.Len(t, loaded.UI().Targets(), 2, "the disabled button is not in the ring")

	key(t, loaded, "tab", 40, 3)
	id, action := loaded.UI().Focused()
	assert.Equal(t, "quit", id)
	assert.Equal(t, "quit", action)

	key(t, loaded, "tab", 40, 3)
	id, _ = loaded.UI().Focused()
	assert.Equal(t, "save", id, "the ring wraps")
}

// The focused control looks focused in the frame, not merely in the UI's own
// bookkeeping.
func TestFocusIsVisibleInTheRenderedFrame(t *testing.T) {
	loaded := interactive(t, buttons, 40, 3)

	first, err := loaded.Render(nil, 40, 3)
	require.NoError(t, err)
	assert.Contains(t, ansi.Strip(first), "║ Save ║", "the focused button wears the doubled border")

	key(t, loaded, "tab", 40, 3)
	second, err := loaded.Render(nil, 40, 3)
	require.NoError(t, err)
	assert.Contains(t, ansi.Strip(second), "║ Quit ║")
	assert.NotContains(t, ansi.Strip(second), "║ Save ║")
}

func TestEnterActivatesTheFocusedControl(t *testing.T) {
	loaded := interactive(t, buttons, 40, 3)

	events := key(t, loaded, "enter", 40, 3)
	require.Len(t, events, 1)
	assert.Equal(t, tml.Activated, events[0].Kind)
	assert.Equal(t, "save", events[0].Action)
}

// A click resolves against the geometry of the frame the user is looking at,
// which is the whole reason layout publishes it.
func TestClickingLandsOnTheControlUnderThePointer(t *testing.T) {
	loaded := interactive(t, buttons, 40, 3)

	quit := targetOf(t, loaded, "quit")
	inside := quit.Rect.X + 1

	loaded.UI().Update(tea.MouseClickMsg{X: inside, Y: quit.Rect.Y + 1, Button: tea.MouseLeft})
	events := loaded.UI().Update(tea.MouseReleaseMsg{X: inside, Y: quit.Rect.Y + 1, Button: tea.MouseLeft})

	require.Len(t, events, 1)
	assert.Equal(t, tml.Activated, events[0].Kind)
	assert.Equal(t, "quit", events[0].ID)
}

// Clicking where a disabled control is drawn does nothing: it never entered the
// ring, so there is nothing under the pointer to hit.
func TestClickingADisabledControlDoesNothing(t *testing.T) {
	loaded := interactive(t, buttons, 40, 3)

	// The locked button sits between the two live ones.
	save := targetOf(t, loaded, "save")
	quit := targetOf(t, loaded, "quit")
	between := (save.Rect.X + save.Rect.W + quit.Rect.X) / 2

	loaded.UI().Update(tea.MouseClickMsg{X: between, Y: 1, Button: tea.MouseLeft})
	assert.Empty(t, loaded.UI().Update(tea.MouseReleaseMsg{X: between, Y: 1, Button: tea.MouseLeft}))
}

// Hovering shows through to the frame, so a pointer over a control is visible
// rather than only recorded.
func TestHoverReachesTheFrame(t *testing.T) {
	loaded := interactive(t, buttons, 40, 3)

	// Hover the second control, so the first one's focus styling is not what
	// changes the output.
	quit := targetOf(t, loaded, "quit")
	before, err := loaded.Render(nil, 40, 3)
	require.NoError(t, err)

	loaded.UI().Update(tea.MouseMotionMsg{X: quit.Rect.X + 1, Y: quit.Rect.Y + 1})
	after, err := loaded.Render(nil, 40, 3)
	require.NoError(t, err)

	assert.NotEqual(t, before, after)
}

// A control scrolled out of its viewport is not on the screen, so clicking
// where it would have been must not reach it.
func TestScrolledOutControlsCannotBeClicked(t *testing.T) {
	loaded := interactive(t, `<Scrollbox offset="4" height="2" width="20">
		<Stack>
			<Button id="top" action="top" label="Top"/>
			<Button id="bottom" action="bottom" label="Bottom"/>
		</Stack>
	</Scrollbox>`, 20, 2)

	for _, target := range loaded.UI().Targets() {
		assert.NotEqual(t, "top", target.ID, "the first button has been scrolled away")
	}
}

// A view with nothing focusable is not broken, it simply has no ring.
func TestAViewWithNoControlsHasNoRing(t *testing.T) {
	loaded := interactive(t, `<Text>just words</Text>`, 20, 1)

	assert.Empty(t, loaded.UI().Targets())
	id, _ := loaded.UI().Focused()
	assert.Empty(t, id)
	assert.Empty(t, key(t, loaded, "tab", 20, 1))
}

// Two controls answering to the same id would make focus ambiguous from one
// frame to the next, so it is rejected where it is written.
func TestDuplicateIDsAreRejected(t *testing.T) {
	loaded, err := view(t, app(`<Stack>
		<Button id="go" label="One"/>
		<Button id="go" label="Two"/>
	</Stack>`), tml.Options{})
	require.NoError(t, err)

	_, err = loaded.Render(nil, 20, 6)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `id "go" is already used`)
}

func targetOf(t *testing.T, loaded *tml.View, id string) tml.Target {
	t.Helper()
	for _, target := range loaded.UI().Targets() {
		if target.ID == id {
			return target
		}
	}
	t.Fatalf("no control with id %q in the ring", id)
	return tml.Target{}
}
