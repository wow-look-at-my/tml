package widgets

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml/cell"
	"github.com/wow-look-at-my/tml/sema"
	"github.com/wow-look-at-my/tml/widget"
	"github.com/wow-look-at-my/tml/widget/widgettest"
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

	assert.Equal(t, "c\nd\ne", compose(box, 1, 3, lines(8)...))
}

// Layout tells the box where the content landed, clamped, and the box draws its scrollbar and fade from that.
func TestScrollboxLearnsItsScrollFromLayout(t *testing.T) {
	box := composer(t, "Scrollbox", map[string]string{"offset": "9999"})
	widgettest.Compose(box, lines(8), 2, 3)

	assert.Equal(t, widget.Scroll{Y: 5, MaxY: 5}, box.(*scrollbox).scroll)
}

// Scrolling stops at the last screenful.
func TestScrollboxStopsAtTheEnd(t *testing.T) {
	box := composer(t, "Scrollbox", map[string]string{"offset": "9999", "scrollbar": "never"})

	assert.Equal(t, "f\ng\nh", compose(box, 1, 3, lines(8)...))
}

// Content that fits needs no scrolling, however far a host asks to go.
func TestScrollboxWithNothingToScrollStaysPut(t *testing.T) {
	box := composer(t, "Scrollbox", map[string]string{"offset": "4", "scrollbar": "never"})

	assert.Equal(t, "a\nb\nc", compose(box, 1, 3, lines(3)...))
}

func TestScrollboxScrollsSideways(t *testing.T) {
	box := composer(t, "Scrollbox", map[string]string{"offsetX": "2", "scrollbar": "never"})

	assert.Equal(t, "cde", compose(box, 3, 1, "abcdefgh"))
}

// A short line still fills the viewport, or the rows below would be ragged and the box's background would show through
func TestScrollboxPadsShortLines(t *testing.T) {
	box := composer(t, "Scrollbox", map[string]string{"scrollbar": "never"})

	assert.Equal(t, "ab   ", compose(box, 5, 1, "ab"))
}

func TestScrollbarTracksTheOffset(t *testing.T) {
	top := composer(t, "Scrollbox", map[string]string{"offset": "0"})
	assert.Equal(t, []string{"a█", "b█", "c│", "d│"}, split(compose(top, 2, 4, lines(8)...)))

	bottom := composer(t, "Scrollbox", map[string]string{"offset": "4"})
	assert.Equal(t, []string{"e│", "f│", "g█", "h█"}, split(compose(bottom, 2, 4, lines(8)...)))
}

// Content that fits needs no thumb. Auto leaves the column blank rather than dropping it, because reflowing the moment
func TestScrollbarIsBlankWhenEverythingFits(t *testing.T) {
	auto := composer(t, "Scrollbox", nil)
	assert.Equal(t, []string{"a ", "b "}, split(compose(auto, 2, 2, lines(2)...)))

	always := composer(t, "Scrollbox", map[string]string{"scrollbar": "always"})
	assert.Equal(t, []string{"a│", "b│"}, split(compose(always, 2, 2, lines(2)...)))
}

func TestScrollboxGutter(t *testing.T) {
	assert.Equal(t, sema.Thickness{Right: 1}, composer(t, "Scrollbox", nil).Insets(), "the gutter is on the right only")
	assert.Equal(t, sema.Thickness{}, composer(t, "Scrollbox", map[string]string{"scrollbar": "never"}).Insets())
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

// faded is a row the edge fade drew in one flat color, which is what a row naming no colors of its own becomes.
func faded(channel int, line string) string {
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm%s%s", channel, channel, channel, line, resetSGR)
}

// darkScrollbox is the widget as a dark-themed host builds it, where the fade blends toward black rather than white.
func darkScrollbox(t *testing.T, pairs map[string]string) widget.Composer {
	t.Helper()
	factory, ok := Library().Factory("Scrollbox")
	require.True(t, ok)
	native, err := factory.Build(widget.Context{Dark: true, Attrs: attrsOf("Scrollbox", pairs)})
	require.NoError(t, err)
	box, ok := native.(widget.Composer)
	require.True(t, ok, "<Scrollbox> should wrap its children")
	return box
}

// An edge with content beyond it is faded, which is how a region shows it continues off-screen.
func TestScrollboxFadesTheEdgeWithContentBeyondIt(t *testing.T) {
	box := composer(t, "Scrollbox", map[string]string{"offset": "5", "scrollbar": "never", "fade": "true"})

	assert.Equal(t, faded(167, "f")+"\ng\nh", box.Compose(content(lines(8)...), 1, 3))
}

// The bottom edge fades the same way once the content below it runs past the window.
func TestScrollboxFadesTheBottomEdge(t *testing.T) {
	box := composer(t, "Scrollbox", map[string]string{"scrollbar": "never", "fade": "true"})

	assert.Equal(t, "a\nb\n"+faded(167, "c"), box.Compose(content(lines(8)...), 1, 3))
}

func TestScrollboxFadesBothEdges(t *testing.T) {
	box := composer(t, "Scrollbox", map[string]string{"offset": "2", "scrollbar": "never", "fade": "true"})

	assert.Equal(t, faded(167, "c")+"\nd\n"+faded(167, "e"), box.Compose(content(lines(8)...), 1, 3))
}

// A tall region has room for a band deeper than one row. Its rows are faded by different amounts: the edge is a
// gradient rather than one flat line.
func TestScrollboxStepsTheBandOnATallRegion(t *testing.T) {
	box := composer(t, "Scrollbox", map[string]string{"offset": "4", "scrollbar": "never", "fade": "true"})

	rows := split(box.Compose(content(lines(20)...), 1, 8))
	assert.Equal(t, faded(197, "e"), rows[0], "the row at the edge is mostly background")
	assert.Equal(t, faded(128, "f"), rows[1], "the row nearest the content keeps most of its color")
	assert.Equal(t, []string{"g", "h", "i", "j"}, rows[2:6], "the middle is untouched")
	assert.Equal(t, faded(128, "k"), rows[6], "the step into the bottom band is the shallower one")
	assert.Equal(t, faded(197, "l"), rows[7])
}

// Nothing is faded when the whole content is on screen -- the fade marks what is hidden, not what is displayed.
func TestScrollboxFadesNothingWhenTheEndIsInView(t *testing.T) {
	box := composer(t, "Scrollbox", map[string]string{"offset": "9999", "scrollbar": "never", "fade": "true"})

	assert.Equal(t, "a\nb\nc", box.Compose(content(lines(3)...), 1, 3))
}

// A host that hands over a window it cut itself knows which edges have content beyond them. It says so, rather than
// handing over a total it never laid out.
func TestScrollboxFadesTheEdgeAHostNames(t *testing.T) {
	above := composer(t, "Scrollbox", map[string]string{"scrollbar": "never", "fade": "true", "moreAbove": "true"})
	assert.Equal(t, faded(167, "a")+"\nb\nc", above.Compose(content(lines(3)...), 1, 3))

	below := composer(t, "Scrollbox", map[string]string{"scrollbar": "never", "fade": "true", "moreBelow": "true"})
	assert.Equal(t, "a\nb\n"+faded(167, "c"), below.Compose(content(lines(3)...), 1, 3))
}

// On a dark theme the background is black, so the same band lands on different colors.
func TestScrollboxFadesTowardTheBackgroundOfTheTheme(t *testing.T) {
	box := darkScrollbox(t, map[string]string{"scrollbar": "never", "fade": "true"})

	assert.Equal(t, "a\nb\n"+faded(90, "c"), box.Compose(content(lines(8)...), 1, 3))
}

func TestScrollboxLeavesTheFadeOffUnlessAsked(t *testing.T) {
	box := composer(t, "Scrollbox", map[string]string{"scrollbar": "never"})

	assert.Equal(t, "a\nb\nc", box.Compose(content(lines(8)...), 1, 3))
}

func TestScrollboxRejectsAnUnknownFadeValue(t *testing.T) {
	_, err := tryBuild("Scrollbox", map[string]string{"fade": "sometimes"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected true or false")
}

// A child's own styling ends with a reset, and the fade has to carry on past it, in the color of that child's own text.
func TestScrollboxFadesPastAChildsReset(t *testing.T) {
	box := composer(t, "Scrollbox", map[string]string{
		"offset":    "5",
		"scrollbar": "never",
		"fade":      "true",
	})
	rows := lines(8)
	rows[5] = "\x1b[31mf\x1b[0m tail"

	pad := strings.Repeat(" ", 5)
	assert.Equal(t, faded(167, "\x1b[38;2;233;140;140mf\x1b[0;38;2;167;167;167m tail")+"\ng"+pad+"\nh"+pad,
		box.Compose(content(rows...), 6, 3))
}

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

// contentHeight is what the maximum offset is derived from.
func TestScrollboxClampsToTheContentItWasToldAbout(t *testing.T) {
	box := composer(t, "Scrollbox", map[string]string{
		"offset":        "9999",
		"contentHeight": "1000",
		"scrollbar":     "always",
	})

	out := box.Compose(content(lines(3)...), 2, 3)
	assert.Equal(t, "a│\nb│\nc█", out, "clamped to 997, which is the bottom of 1000 in a 3-row view")
}
