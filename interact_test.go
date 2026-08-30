package tml_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml"
)

// buttons is a row of several controls, the middle control disabled.
const buttons = `<Stack orientation="horizontal" gap="1">
	<Button id="save" action="save" label="Save"/>
	<Button id="locked" label="Locked" disabled="true"/>
	<Button id="quit" action="quit" label="Quit"/>
</Stack>`

// interactive loads a view and renders a single frame, which is what publishes the geometry the UI resolves clicks against.
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

// Focus starts on the earliest control and tab walks the row, skipping the disabled control: tab must never stop somewhere
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

// The focused control looks focused in the frame, not merely in the UI's own bookkeeping.
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

// A click resolves against the geometry of the frame the user is looking at, which is the whole reason layout
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

// Clicking where a disabled control is drawn does nothing: it never entered the ring, so there is nothing under the
func TestClickingADisabledControlDoesNothing(t *testing.T) {
	loaded := interactive(t, buttons, 40, 3)

	// The locked button sits between both live ones.
	save := targetOf(t, loaded, "save")
	quit := targetOf(t, loaded, "quit")
	between := (save.Rect.X + save.Rect.W + quit.Rect.X) / 2

	loaded.UI().Update(tea.MouseClickMsg{X: between, Y: 1, Button: tea.MouseLeft})
	assert.Empty(t, loaded.UI().Update(tea.MouseReleaseMsg{X: between, Y: 1, Button: tea.MouseLeft}))
}

// Hovering shows through to the frame, so a pointer over a control is visible rather than only recorded.
func TestHoverReachesTheFrame(t *testing.T) {
	loaded := interactive(t, buttons, 40, 3)

	// Hover the next control, so the leading's focus styling is not what changes the output.
	quit := targetOf(t, loaded, "quit")
	before, err := loaded.Render(nil, 40, 3)
	require.NoError(t, err)

	loaded.UI().Update(tea.MouseMotionMsg{X: quit.Rect.X + 1, Y: quit.Rect.Y + 1})
	after, err := loaded.Render(nil, 40, 3)
	require.NoError(t, err)

	assert.NotEqual(t, before, after)
}

// A control scrolled out of its viewport is not on the screen, so clicking where it would have been must not reach it.
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

// Where a scrolling region actually stopped is a number only the frame knows: how far the content runs depends on the
func TestTheFrameReportsWhereAScrollingRegionLanded(t *testing.T) {
	loaded := interactive(t, `<Scrollbox id="log" offset="9999" height="2" width="20">
		<Stack>
			<Text>one</Text>
			<Text>two</Text>
			<Text>three</Text>
			<Text>four</Text>
		</Stack>
	</Scrollbox>`, 20, 2)

	target, ok := loaded.UI().Target("log")
	require.True(t, ok)
	assert.Equal(t, 2, target.Scroll.MaxY, "four lines seen through two")
	assert.Equal(t, 2, target.Scroll.Y, "asking for further than there is stops at the end")
}

// Anything that does not scroll reports no scrolling, rather than a position a host would then try to move.
func TestANonScrollingControlReportsNoScroll(t *testing.T) {
	loaded := interactive(t, `<Button id="go" action="go" label="Go"/>`, 20, 3)

	target, ok := loaded.UI().Target("go")
	require.True(t, ok)
	assert.Equal(t, tml.Scroll{}, target.Scroll)

	_, ok = loaded.UI().Target("missing")
	assert.False(t, ok)
}

// A view with nothing focusable is not broken, it simply has no ring.
func TestAViewWithNoControlsHasNoRing(t *testing.T) {
	loaded := interactive(t, `<Text>just words</Text>`, 20, 1)

	assert.Empty(t, loaded.UI().Targets())
	id, _ := loaded.UI().Focused()
	assert.Empty(t, id)
	assert.Empty(t, key(t, loaded, "tab", 20, 1))
}

// A pair of controls answering to the same id would make focus ambiguous from a single frame to the next, so it is rejected where
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

// Geometry and output have to agree, or a click lands on nothing. These walk the panels that place children at
func TestPublishedRectsMatchWhereControlsArePainted(t *testing.T) {
	for _, panel := range []struct {
		name     string
		template string
	}{
		{"grid", `<Grid columns="auto,1*" rows="auto,auto" gap="1">
			<Button id="a" label="Aaa" Grid.row="0" Grid.column="0"/>
			<Button id="b" label="Bbb" Grid.row="0" Grid.column="1"/>
			<Button id="c" label="Ccc" Grid.row="1" Grid.column="1"/>
		</Grid>`},
		{"centered stack", `<Stack align="center" gap="1" width="40">
			<Button id="a" label="Aaa"/>
			<Button id="b" label="Bbbbbbbb"/>
		</Stack>`},
		{"horizontal stack", `<Stack orientation="horizontal" gap="3">
			<Button id="a" label="Aaa"/>
			<Button id="b" label="Bbb"/>
		</Stack>`},
	} {
		t.Run(panel.name, func(t *testing.T) {
			loaded := interactive(t, panel.template, 40, 12)
			frame, err := loaded.Render(nil, 40, 12)
			require.NoError(t, err)

			for _, target := range loaded.UI().Targets() {
				rect := target.Rect
				assert.Contains(t, "╭╔", cellAt(t, frame, rect.X, rect.Y),
					"%s: no button corner where its rect starts", target.ID)

				// A stretched button centres its label, so where the text lands is only pinned to the rect it has to sit inside.
				label := strings.ToUpper(target.ID) + strings.Repeat(strings.ToLower(target.ID), 2)
				x, y := find(t, frame, label)
				assert.Equal(t, rect.Y+1, y, "%s is drawn on a different row from its rect", target.ID)
				assert.GreaterOrEqual(t, x, rect.X, "%s is drawn left of its rect", target.ID)
				assert.Less(t, x, rect.X+rect.W, "%s is drawn past its rect", target.ID)
			}
		})
	}
}

// cellAt is the character painted at a single cell of a frame.
func cellAt(t *testing.T, frame string, x, y int) string {
	t.Helper()
	lines := strings.Split(ansi.Strip(frame), "\n")
	require.Less(t, y, len(lines), "row %d is off the frame", y)
	return ansi.Cut(lines[y], x, x+1)
}

// find is where a string was painted in a frame, in cells.
func find(t *testing.T, frame, needle string) (x, y int) {
	t.Helper()
	for row, line := range strings.Split(ansi.Strip(frame), "\n") {
		if column := strings.Index(line, needle); column >= 0 {
			return lipgloss.Width(line[:column]), row
		}
	}
	t.Fatalf("%q is not in the frame:\n%s", needle, ansi.Strip(frame))
	return 0, 0
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
