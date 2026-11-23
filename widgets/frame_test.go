package widgets

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml/cell"
	"github.com/wow-look-at-my/tml/sema"
	"github.com/wow-look-at-my/tml/widget"
	"github.com/wow-look-at-my/tml/widget/widgettest"
)

// composer builds a wrapping widget the way the engine does.
func composer(t *testing.T, element string, pairs map[string]string) widget.Composer {
	t.Helper()
	native := build(t, element, pairs)
	wrapper, ok := native.(widget.Composer)
	require.True(t, ok, "<%s> should wrap its children", element)
	return wrapper
}

// compose draws a composer around children the way the engine does, as plain text.
func compose(c widget.Composer, w, h int, children ...string) string {
	return widgettest.Compose(c, children, w, h).Text()
}

func TestBorderDrawsAFrameAroundItsChildren(t *testing.T) {
	border := composer(t, "Border", nil)

	assert.Equal(t, "┌────┐\n│ hi │\n└────┘", compose(border, 6, 3, "hi"))
}

func TestBorderColoursItsEdgeButNotItsContent(t *testing.T) {
	buf := widgettest.Compose(composer(t, "Border", map[string]string{"color": ref("line")}), []string{"hi"}, 6, 3)

	for _, at := range [][2]int{{0, 0}, {3, 0}, {5, 1}, {0, 2}} {
		assert.Equal(t, cell.Indexed(5), buf.At(at[0], at[1]).Style.Fg, "edge cell %v", at)
	}
	assert.False(t, buf.At(2, 1).Style.Fg.IsSet(), "the content keeps its own colour")
}

func TestBorderRejectsAColourThatIsNotAColour(t *testing.T) {
	_, err := tryBuild("Border", map[string]string{"color": "chartreuse-ish"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "color")
}

func TestBorderInsetIsTheFrameItDraws(t *testing.T) {
	assert.Equal(t, sema.Thickness{Top: 1, Right: 2, Bottom: 1, Left: 2}, composer(t, "Border", nil).Insets(),
		"a border on every side, plus the default padding either side")
	assert.Equal(t, sema.Thickness{Top: 1, Right: 3, Bottom: 1, Left: 3},
		composer(t, "Border", map[string]string{"pad": "2"}).Insets(), "padding is on both sides, inside the border")
	assert.Equal(t, sema.Thickness{Top: 1, Right: 1, Bottom: 1, Left: 1},
		composer(t, "Border", map[string]string{"pad": "0"}).Insets(), "pad=0 is the tight frame")
}

func TestBorderTakesATitleInItsTopEdge(t *testing.T) {
	border := composer(t, "Border", map[string]string{"title": "Logs"})

	assert.Equal(t, "┌ Logs ─────┐\n│ x         │\n└───────────┘", compose(border, 13, 3, "x"))
}

func TestBorderTitleAligns(t *testing.T) {
	right := composer(t, "Border", map[string]string{"title": "Logs", "titleAlign": "right"})
	assert.Equal(t, "┌───── Logs ┐", firstLine(compose(right, 13, 3, "x")))

	middle := composer(t, "Border", map[string]string{"title": "Logs", "titleAlign": "center"})
	assert.Equal(t, "┌── Logs ───┐", firstLine(compose(middle, 13, 3, "x")))
}

// A title with nowhere to go is left off rather than overrunning the corner it would otherwise write over.
func TestBorderDropsATitleThatCannotFit(t *testing.T) {
	border := composer(t, "Border", map[string]string{"title": "A very long title"})

	assert.Equal(t, "┌────┐", firstLine(compose(border, 6, 3, "x")))
}

func TestBorderKindsAreNamed(t *testing.T) {
	assert.Equal(t, "╭────╮", firstLine(compose(composer(t, "Border", map[string]string{"kind": "rounded"}), 6, 3, "hi")))
	assert.Equal(t, "+----+", firstLine(compose(composer(t, "Border", map[string]string{"kind": "ascii"}), 6, 3, "hi")))

	_, err := tryBuild("Border", map[string]string{"kind": "squiggly"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected one of ascii, block, double, hidden, normal, rounded, thick")
}

// An empty frame is still a frame: it measures and draws itself.
func TestEmptyBorderStillDrawsItself(t *testing.T) {
	border := build(t, "Border", nil)

	w, h := border.Measure(0, 0)
	assert.Equal(t, 4, w)
	assert.Equal(t, 2, h)
	// Several rows, not a pair of: a bordered block always has at least a single content line.
	assert.Equal(t, "┌──┐\n│  │\n└──┘", widgettest.Text(border, 4, 3))
}

// A popup is the same frame with a dialog's defaults, and it says where it belongs so a canvas puts it in the middle
func TestPopupIsADialogShapedFrame(t *testing.T) {
	popup := composer(t, "Popup", map[string]string{"title": "Confirm"})

	assert.Equal(t, "╭ Confirm ───╮", firstLine(compose(popup, 14, 3, "Quit?")))

	anchored, ok := popup.(widget.Anchored)
	require.True(t, ok)
	assert.Equal(t, "center", anchored.DefaultAnchor())

	assert.Equal(t, 4, popup.Insets().Horizontal(), "a dialog is padded inside its border")
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
