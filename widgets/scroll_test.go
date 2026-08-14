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

// Scrolling past the end shows blanks rather than snapping back, so a host with
// its arithmetic wrong sees that it is wrong.
func TestScrollboxPastTheEndGoesBlank(t *testing.T) {
	box := composer(t, "Scrollbox", map[string]string{"offset": "9", "scrollbar": "never"})

	assert.Equal(t, " \n \n ", box.Compose(content(lines(3)...), 1, 3))
}

func TestScrollboxScrollsSideways(t *testing.T) {
	box := composer(t, "Scrollbox", map[string]string{"offsetX": "2", "scrollbar": "never"})

	assert.Equal(t, "cde", box.Compose(content("abcdefgh"), 3, 1))
}

// A short line still fills the viewport, or the rows below would be ragged and
// the box's background would show through in stripes.
func TestScrollboxPadsShortLines(t *testing.T) {
	box := composer(t, "Scrollbox", map[string]string{"scrollbar": "never"})

	assert.Equal(t, "ab   ", box.Compose(content("ab"), 5, 1))
}

func TestScrollbarTracksTheOffset(t *testing.T) {
	// Four rows showing eight means a thumb half the height of the track.
	top := composer(t, "Scrollbox", map[string]string{"offset": "0"})
	assert.Equal(t, []string{"a█", "b█", "c│", "d│"}, split(top.Compose(content(lines(8)...), 2, 4)))

	bottom := composer(t, "Scrollbox", map[string]string{"offset": "4"})
	assert.Equal(t, []string{"e│", "f│", "g█", "h█"}, split(bottom.Compose(content(lines(8)...), 2, 4)))
}

// Content that fits needs no thumb. Auto leaves the column blank rather than
// dropping it, because reflowing the moment the content grew would shift
// everything sideways.
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

// The content is measured against unlimited height -- that is the whole point --
// and layout is told how far it has been shifted so a control scrolled halfway
// off is clicked where it is drawn.
func TestScrollboxDeclaresItsUnboundedAxisAndOffset(t *testing.T) {
	box := composer(t, "Scrollbox", map[string]string{"offset": "3", "offsetX": "2"})

	free, ok := box.(widget.Unbounded)
	require.True(t, ok)
	horizontal, vertical := free.Unbounded()
	assert.False(t, horizontal)
	assert.True(t, vertical)

	scrolled, ok := box.(widget.Scrolled)
	require.True(t, ok)
	x, y := scrolled.ChildOffset()
	assert.Equal(t, 2, x)
	assert.Equal(t, 3, y)
}

// A negative offset is meaningless, and letting it through would put the content
// below the top of its own viewport.
func TestScrollboxClampsNegativeOffsets(t *testing.T) {
	box := build(t, "Scrollbox", map[string]string{"offset": "-4", "offsetX": "-2"})

	scrolled := box.(widget.Scrolled)
	x, y := scrolled.ChildOffset()
	assert.Equal(t, 0, x)
	assert.Equal(t, 0, y)
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
