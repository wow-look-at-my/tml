package widgets

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml/widget"
)

// composer builds a wrapping widget the way the engine does.
func composer(t *testing.T, element string, pairs map[string]string) widget.Composer {
	t.Helper()
	native := build(t, element, pairs)
	wrapper, ok := native.(widget.Composer)
	require.True(t, ok, "<%s> should wrap its children", element)
	return wrapper
}

func content(lines ...string) widget.Slots { return widget.Slots{"": lines} }

func TestBorderDrawsAFrameAroundItsChildren(t *testing.T) {
	border := composer(t, "Border", nil)

	assert.Equal(t, "┌────┐\n│ hi │\n└────┘", border.Compose(content("hi"), 6, 3))
}

func TestBorderInsetIsTheFrameItDraws(t *testing.T) {
	w, h := composer(t, "Border", nil).Inset()
	assert.Equal(t, 4, w, "a border either side plus the default padding")
	assert.Equal(t, 2, h)

	w, _ = composer(t, "Border", map[string]string{"pad": "2"}).Inset()
	assert.Equal(t, 6, w, "padding is on both sides, inside the border")

	w, _ = composer(t, "Border", map[string]string{"pad": "0"}).Inset()
	assert.Equal(t, 2, w, "pad=0 is the tight frame")
}

func TestBorderTakesATitleInItsTopEdge(t *testing.T) {
	border := composer(t, "Border", map[string]string{"title": "Logs"})

	assert.Equal(t, "┌ Logs ─────┐\n│ x         │\n└───────────┘", border.Compose(content("x"), 13, 3))
}

func TestBorderTitleAligns(t *testing.T) {
	right := composer(t, "Border", map[string]string{"title": "Logs", "titleAlign": "right"})
	assert.Equal(t, "┌───── Logs ┐", firstLine(right.Compose(content("x"), 13, 3)))

	middle := composer(t, "Border", map[string]string{"title": "Logs", "titleAlign": "center"})
	assert.Equal(t, "┌── Logs ───┐", firstLine(middle.Compose(content("x"), 13, 3)))
}

// A title with nowhere to go is left off rather than overrunning the corner it
// would otherwise write over.
func TestBorderDropsATitleThatCannotFit(t *testing.T) {
	border := composer(t, "Border", map[string]string{"title": "A very long title"})

	assert.Equal(t, "┌────┐", firstLine(border.Compose(content("x"), 6, 3)))
}

func TestBorderKindsAreNamed(t *testing.T) {
	assert.Equal(t, "╭────╮", firstLine(composer(t, "Border", map[string]string{"kind": "rounded"}).Compose(content("hi"), 6, 3)))
	assert.Equal(t, "+----+", firstLine(composer(t, "Border", map[string]string{"kind": "ascii"}).Compose(content("hi"), 6, 3)))

	_, err := tryBuild("Border", map[string]string{"kind": "squiggly"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected one of ascii, block, double, hidden, normal, rounded, thick")
}

// An empty frame is still a frame: it measures and draws itself, because layout
// only takes over the children when there are some.
func TestEmptyBorderStillDrawsItself(t *testing.T) {
	border := build(t, "Border", nil)

	w, h := border.Measure(0, 0)
	assert.Equal(t, 4, w)
	assert.Equal(t, 2, h)
	// Three rows, not two: a bordered block always has at least one content line.
	assert.Equal(t, "┌──┐\n│  │\n└──┘", border.Render(4, 3))
}

// A popup is the same frame with a dialog's defaults, and it says where it
// belongs so a canvas puts it in the middle without being told.
func TestPopupIsADialogShapedFrame(t *testing.T) {
	popup := composer(t, "Popup", map[string]string{"title": "Confirm"})

	assert.Equal(t, "╭ Confirm ───╮", firstLine(popup.Compose(content("Quit?"), 14, 3)))

	anchored, ok := popup.(widget.Anchored)
	require.True(t, ok)
	assert.Equal(t, "center", anchored.DefaultAnchor())

	w, _ := popup.Inset()
	assert.Equal(t, 4, w, "a dialog is padded inside its border")
}

func TestBorderStaysWhereItIsPut(t *testing.T) {
	anchored, ok := composer(t, "Border", nil).(widget.Anchored)
	require.True(t, ok)
	assert.Empty(t, anchored.DefaultAnchor())
}

func TestFrameRejectsNegativePadding(t *testing.T) {
	_, err := tryBuild("Border", map[string]string{"pad": "-1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pad must not be negative")
}

func firstLine(s string) string {
	for i, r := range s {
		if r == '\n' {
			return s[:i]
		}
	}
	return s
}
