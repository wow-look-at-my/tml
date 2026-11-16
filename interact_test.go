package tml_test

import _ "embed"

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml"
	"github.com/wow-look-at-my/tml/cell"
	"github.com/wow-look-at-my/tml/input"
)

// buttons is a row of several controls, the middle control disabled.
//
//go:embed testdata/buttons.xml
var buttons string

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
	events := loaded.UI().Update(input.ParseKey(stroke))
	_, err := loaded.Render(nil, w, h)
	require.NoError(t, err)
	return events
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
	assert.Contains(t, cell.Strip(first), "║ Save ║", "the focused button wears the doubled border")

	key(t, loaded, "tab", 40, 3)
	second, err := loaded.Render(nil, 40, 3)
	require.NoError(t, err)
	assert.Contains(t, cell.Strip(second), "║ Quit ║")
	assert.NotContains(t, cell.Strip(second), "║ Save ║")
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

	loaded.UI().Update(input.Mouse{X: inside, Y: quit.Rect.Y + 1, Button: input.ButtonLeft, Action: input.Press})
	events := loaded.UI().Update(input.Mouse{X: inside, Y: quit.Rect.Y + 1, Button: input.ButtonLeft, Action: input.Release})

	require.Len(t, events, 1)
	assert.Equal(t, tml.Activated, events[0].Kind)
	assert.Equal(t, "quit", events[0].ID)
}

// Clicking where a disabled control is drawn does nothing: it never entered the ring.
func TestClickingADisabledControlDoesNothing(t *testing.T) {
	loaded := interactive(t, buttons, 40, 3)

	// The locked button sits between both live ones.
	save := targetOf(t, loaded, "save")
	quit := targetOf(t, loaded, "quit")
	between := (save.Rect.X + save.Rect.W + quit.Rect.X) / 2

	loaded.UI().Update(input.Mouse{X: between, Y: 1, Button: input.ButtonLeft, Action: input.Press})
	assert.Empty(t, loaded.UI().Update(input.Mouse{X: between, Y: 1, Button: input.ButtonLeft, Action: input.Release}))
}

// Hovering shows through to the frame, so a pointer over a control is visible rather than only recorded.
func TestHoverReachesTheFrame(t *testing.T) {
	loaded := interactive(t, buttons, 40, 3)

	// Hover the next control, so the leading's focus styling is not what changes the output.
	quit := targetOf(t, loaded, "quit")
	before, err := loaded.Render(nil, 40, 3)
	require.NoError(t, err)

	loaded.UI().Update(input.Mouse{X: quit.Rect.X + 1, Y: quit.Rect.Y + 1, Action: input.Motion})
	after, err := loaded.Render(nil, 40, 3)
	require.NoError(t, err)

	assert.NotEqual(t, before, after)
}

// A control scrolled out of its viewport is not on the screen, so clicking
// where it.
func TestScrolledOutControlsCannotBeClicked(t *testing.T) {
	loaded := interactive(t, testScrolledOutControlsCannotBeClickedXml, 20, 2)

	for _, target := range loaded.UI().Targets() {
		assert.NotEqual(t, "top", target.ID, "the first button has been scrolled away")
	}
}

// Where a scrolling region stopped is a number only the frame knows.
func TestTheFrameReportsWhereAScrollingRegionLanded(t *testing.T) {
	loaded := interactive(t, testTheFrameReportsWhereAScrollingRegionLandedXml, 20, 2)

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

// A view with nothing focusable is not broken, it has no ring.
func TestAViewWithNoControlsHasNoRing(t *testing.T) {
	loaded := interactive(t, `<Text>just words</Text>`, 20, 1)

	assert.Empty(t, loaded.UI().Targets())
	id, _ := loaded.UI().Focused()
	assert.Empty(t, id)
	assert.Empty(t, key(t, loaded, "tab", 20, 1))
}

// A pair of controls answering to the same id would make focus ambiguous from a single frame to the next.
func TestDuplicateIDsAreRejected(t *testing.T) {
	loaded, err := view(t, app(testDuplicateIDsAreRejectedXml), tml.Options{})
	require.NoError(t, err)

	_, err = loaded.Render(nil, 20, 6)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `id "go" is already used`)
}

// Geometry and output have to agree, or a click lands on nothing.
func TestPublishedRectsMatchWhereControlsArePainted(t *testing.T) {
	for _, panel := range []struct {
		name     string
		template string
	}{
		{"grid", testPublishedRectsMatchWhereControlsArePaintedXml},
		{"centered stack", testPublishedRectsMatchWhereControlsArePaintedXml2},
		{"horizontal stack", testPublishedRectsMatchWhereControlsArePaintedXml3},
		{"border", `<Border><Stack><Button id="a" label="Aaa"/></Stack></Border>`},
		{"padded border", `<Border pad="3"><Button id="a" label="Aaa"/></Border>`},
		{"nested borders", testPublishedRectsMatchWhereControlsArePaintedXml4},
		{"popup", `<Canvas><Popup title="Ask"><Button id="a" label="Aaa"/></Popup></Canvas>`},
		{"scrollbox in a border", testPublishedRectsMatchWhereControlsArePaintedXml5},
	} {
		t.Run(panel.name, func(t *testing.T) {
			loaded := interactive(t, panel.template, 40, 12)
			frame, err := loaded.Draw(nil, 40, 12)
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
func cellAt(t *testing.T, frame *cell.Buffer, x, y int) string {
	t.Helper()
	require.Less(t, y, frame.H, "row %d is off the frame", y)
	require.Less(t, x, frame.W, "column %d is off the frame", x)
	return frame.At(x, y).Text
}

// find is where a string of single-cell characters was painted in a frame, in cells.
func find(t *testing.T, frame *cell.Buffer, needle string) (x, y int) {
	t.Helper()
	chars := strings.Split(needle, "")
	for y := range frame.H {
		for x := 0; x+len(chars) <= frame.W; x++ {
			matched := true
			for i, char := range chars {
				if frame.At(x+i, y).Text != char {
					matched = false
					break
				}
			}
			if matched {
				return x, y
			}
		}
	}
	t.Fatalf("%q is not in the frame:\n%s", needle, frame.Text())
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

//go:embed testdata/test-scrolled-out-controls-cannot-be-clicked.xml
var testScrolledOutControlsCannotBeClickedXml string

//go:embed testdata/test-the-frame-reports-where-a-scrolling-region-landed.xml
var testTheFrameReportsWhereAScrollingRegionLandedXml string

//go:embed testdata/test-duplicate-i-ds-are-rejected.xml
var testDuplicateIDsAreRejectedXml string

//go:embed testdata/test-published-rects-match-where-controls-are-painted.xml
var testPublishedRectsMatchWhereControlsArePaintedXml string

//go:embed testdata/test-published-rects-match-where-controls-are-painted-2.xml
var testPublishedRectsMatchWhereControlsArePaintedXml2 string

//go:embed testdata/test-published-rects-match-where-controls-are-painted-3.xml
var testPublishedRectsMatchWhereControlsArePaintedXml3 string

//go:embed testdata/test-published-rects-match-where-controls-are-painted-4.xml
var testPublishedRectsMatchWhereControlsArePaintedXml4 string

//go:embed testdata/test-published-rects-match-where-controls-are-painted-5.xml
var testPublishedRectsMatchWhereControlsArePaintedXml5 string
