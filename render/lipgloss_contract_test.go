package render

import (
	"testing"

	"github.com/charmbracelet/lipgloss/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests pin the Lip Gloss behaviour the renderer is built on. They are
// characterization tests: if an upstream release changes any of it, the break
// surfaces here rather than as garbled frames.

// Measurement is the whole leaf measure pass, so it must ignore ANSI and count
// display cells, not bytes or runes.
func TestMeasurementIgnoresStylingAndCountsCells(t *testing.T) {
	plain := "hello"
	styled := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#ff0000")).Render(plain)

	require.NotEqual(t, plain, styled, "style must actually decorate the string, else this test proves nothing")
	assert.Equal(t, 5, lipgloss.Width(styled), "width must count cells, not ANSI bytes")
	assert.Equal(t, 1, lipgloss.Height(styled))

	w, h := lipgloss.Size("ab\ncdef\ng")
	assert.Equal(t, 4, w, "width of a block is its widest line")
	assert.Equal(t, 3, h)

	// Wide runes occupy two cells. Grid track solving is wrong if this is off.
	assert.Equal(t, 2, lipgloss.Width("世"))
}

// Style.Width wraps rather than truncates, which is what makes it usable as the
// text leaf renderer once layout has assigned a rect.
func TestStyleWidthWrapsToCellWidth(t *testing.T) {
	out := lipgloss.NewStyle().Width(10).Render("the quick brown fox")

	assert.Equal(t, 10, lipgloss.Width(out), "every line is padded to the set width")
	assert.Greater(t, lipgloss.Height(out), 1, "overflowing text wraps instead of truncating")
}

// Inherit is the cascade primitive behind <Style extends="...">: unset fields
// are filled from the parent, set fields win.
func TestInheritFillsOnlyUnsetFields(t *testing.T) {
	base := lipgloss.NewStyle().Bold(true).Padding(1, 2)
	derived := lipgloss.NewStyle().Bold(false).Inherit(base)

	assert.False(t, derived.GetBold(), "an explicitly set field must survive Inherit")
	top, right, bottom, left := derived.GetPadding()
	assert.Equal(t, []int{1, 2, 1, 2}, []int{top, right, bottom, left}, "unset fields come from the parent")
}

// The compositor is how TML places absolutely-positioned rects. Layers carry
// x/y/z, so the renderer never pads or joins strings by hand.
func TestCanvasPlacesLayersAtCoordinates(t *testing.T) {
	canvas := lipgloss.NewCanvas(
		lipgloss.NewLayer("base\nbase").X(0).Y(0).Z(0),
		lipgloss.NewLayer("TOP").X(2).Y(1).Z(1),
	)

	out := canvas.Render()
	t.Logf("canvas render:\n%q", out)

	assert.Equal(t, 2, lipgloss.Height(out))
	assert.Contains(t, out, "TOP", "the higher-z layer is composited in")
}

// Hit testing is what lets mouse events route back to a TML element, so layers
// must be addressable by id.
func TestCanvasHitTestResolvesLayerByID(t *testing.T) {
	canvas := lipgloss.NewCanvas(
		lipgloss.NewLayer("aaaa").ID("bg").X(0).Y(0).Z(0),
		lipgloss.NewLayer("bb").ID("button").X(1).Y(0).Z(1),
	)
	canvas.Render()

	hit := canvas.Hit(1, 0)
	require.NotNil(t, hit, "a point covered by a layer must resolve")
	assert.Equal(t, "button", hit.GetID(), "the topmost layer at the point wins")
}
