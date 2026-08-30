package widgets

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml/widget"
)

func lines(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = string(rune('a' + i))
	}
	return out
}

func TestScrollboxShowsTheWindowTheOffsetSelects(t *testing.T) {
	box := composer(t, "Scrollbox", map[string]string{"offset": "2", "scrollbar": "never"})

	assert.Equal(t, "c\nd\ne", box.Compose(content(lines(8)...), 1, 3))
}

// Scrolling stops at the last screenful. How many lines the content wraps to depends on the width the widget was
func TestScrollboxStopsAtTheEnd(t *testing.T) {
	box := composer(t, "Scrollbox", map[string]string{"offset": "9999", "scrollbar": "never"})

	assert.Equal(t, "f\ng\nh", box.Compose(content(lines(8)...), 1, 3))
}

// Content that fits needs no scrolling, however far a host asks to go.
func TestScrollboxWithNothingToScrollStaysPut(t *testing.T) {
	box := composer(t, "Scrollbox", map[string]string{"offset": "4", "scrollbar": "never"})

	assert.Equal(t, "a\nb\nc", box.Compose(content(lines(3)...), 1, 3))
}

func TestScrollboxScrollsSideways(t *testing.T) {
	box := composer(t, "Scrollbox", map[string]string{"offsetX": "2", "scrollbar": "never"})

	assert.Equal(t, "cde", box.Compose(content("abcdefgh"), 3, 1))
}

// A short line still fills the viewport, or the rows below would be ragged and the box's background would show through
func TestScrollboxPadsShortLines(t *testing.T) {
	box := composer(t, "Scrollbox", map[string]string{"scrollbar": "never"})

	assert.Equal(t, "ab   ", box.Compose(content("ab"), 5, 1))
}

func TestScrollbarTracksTheOffset(t *testing.T) {
	top := composer(t, "Scrollbox", map[string]string{"offset": "0"})
	assert.Equal(t, []string{"a█", "b█", "c│", "d│"}, split(top.Compose(content(lines(8)...), 2, 4)))

	bottom := composer(t, "Scrollbox", map[string]string{"offset": "4"})
	assert.Equal(t, []string{"e│", "f│", "g█", "h█"}, split(bottom.Compose(content(lines(8)...), 2, 4)))
}

// Content that fits needs no thumb. Auto leaves the column blank rather than dropping it, because reflowing the moment
func TestScrollbarIsBlankWhenEverythingFits(t *testing.T) {
	auto := composer(t, "Scrollbox", nil)
	assert.Equal(t, []string{"a ", "b "}, split(auto.Compose(content(lines(2)...), 2, 2)))

	always := composer(t, "Scrollbox", map[string]string{"scrollbar": "always"})
	assert.Equal(t, []string{"a│", "b│"}, split(always.Compose(content(lines(2)...), 2, 2)))
}

func TestScrollboxGutter(t *testing.T) {
	w, h := composer(t, "Scrollbox", nil).Inset()
	assert.Equal(t, 1, w)
	assert.Equal(t, 0, h)

	w, _ = composer(t, "Scrollbox", map[string]string{"scrollbar": "never"}).Inset()
	assert.Equal(t, 0, w)
}

// The content is measured against unlimited height -- that is the whole point -- the region itself takes the full
func TestScrollboxDeclaresHowItsChildrenAreLaidOut(t *testing.T) {
	box := composer(t, "Scrollbox", map[string]string{"offset": "3", "offsetX": "2"})

	arranger, ok := box.(widget.Arranger)
	require.True(t, ok)
	assert.Equal(t, widget.Layout{FreeH: true, FillW: true, OffsetX: 2, OffsetY: 3}, arranger.Arrange())
}

// A negative offset is meaningless, and letting it through would put the content below the top of its own viewport.
func TestScrollboxClampsNegativeOffsets(t *testing.T) {
	box := build(t, "Scrollbox", map[string]string{"offset": "-4", "offsetX": "-2"})

	want := box.(widget.Arranger).Arrange()
	assert.Equal(t, 0, want.OffsetX)
	assert.Equal(t, 0, want.OffsetY)
}

func TestScrollboxRejectsAnUnknownScrollbarMode(t *testing.T) {
	_, err := tryBuild("Scrollbox", map[string]string{"scrollbar": "sometimes"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected one of auto, always, never")
}

func TestEmptyScrollboxFillsItsSpace(t *testing.T) {
	box := build(t, "Scrollbox", nil)

	w, h := box.Measure(10, 4)
	assert.Equal(t, 10, w)
	assert.Equal(t, 4, h)
	assert.Equal(t, "  \n  ", box.Render(2, 2))
}

func split(s string) []string { return strings.Split(s, "\n") }

// A host with more content than it can afford to lay out every frame hands over the rows on screen and says how many
func TestScrollboxDrawsTheWindowItWasHanded(t *testing.T) {
	box := composer(t, "Scrollbox", map[string]string{
		"offset":        "500",
		"contentHeight": "1000",
		"scrollbar":     "never",
	})

	assert.Equal(t, "a\nb\nc", box.Compose(content(lines(3)...), 1, 3))
}

// The bar describes the whole content, which is the only reason to be told its size: a thumb sized against the window
func TestScrollboxBarMeasuresTheContentNotTheWindow(t *testing.T) {
	windowed := composer(t, "Scrollbox", map[string]string{
		"offset":        "0",
		"contentHeight": "100",
		"scrollbar":     "always",
	})
	whole := composer(t, "Scrollbox", map[string]string{"offset": "0", "scrollbar": "always"})

	thumb := strings.Count(windowed.Compose(content(lines(4)...), 2, 4), "█")
	assert.Equal(t, 1, thumb, "4 rows of 100 is the smallest thumb there is")
	assert.Equal(t, 0, strings.Count(whole.Compose(content(lines(4)...), 2, 4), "█"),
		"4 rows of 4 does not scroll, so there is no thumb at all")
}

// contentHeight is what the maximum offset is derived from, so a host can ask for the end of the content rather than
func TestScrollboxClampsToTheContentItWasToldAbout(t *testing.T) {
	box := composer(t, "Scrollbox", map[string]string{
		"offset":        "9999",
		"contentHeight": "1000",
		"scrollbar":     "always",
	})

	out := box.Compose(content(lines(3)...), 2, 3)
	assert.Equal(t, "a│\nb│\nc█", out, "clamped to 997, which is the bottom of 1000 in a 3-row view")
}
