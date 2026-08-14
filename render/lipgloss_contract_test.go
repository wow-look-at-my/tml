package render

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests pin the Lip Gloss v2 behaviour the renderer is built on. They are
// characterization tests: TML delegates measurement, styling and compositing
// rather than reimplementing any of it, so an upstream change has to surface
// here as a failure instead of as garbled frames.

// Measurement is the entire leaf measure pass, so it must ignore ANSI and count
// display cells rather than bytes or runes.
func TestMeasurementIgnoresStylingAndCountsCells(t *testing.T) {
	plain := "hello"
	styled := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#ff0000")).Render(plain)

	require.NotEqual(t, plain, styled, "style must actually decorate the string, else this test proves nothing")
	assert.Equal(t, 5, lipgloss.Width(styled), "width counts cells, not ANSI bytes")
	assert.Equal(t, 1, lipgloss.Height(styled))

	w, h := lipgloss.Size("ab\ncdef\ng")
	assert.Equal(t, 4, w, "a block is as wide as its widest line")
	assert.Equal(t, 3, h)

	assert.Equal(t, 2, lipgloss.Width("世"), "wide runes take two cells; grid tracks are wrong otherwise")
}

// Style.Width wraps rather than truncating, which is what makes it usable as the
// text leaf renderer once layout has assigned a rect.
func TestStyleWidthWrapsToCellWidth(t *testing.T) {
	out := lipgloss.NewStyle().Width(10).Render("the quick brown fox")

	assert.Equal(t, 10, lipgloss.Width(out), "every line is padded out to the set width")
	assert.Greater(t, lipgloss.Height(out), 1, "overflowing text wraps instead of truncating")
}

// Inherit fills unset fields from a parent style but DELIBERATELY excludes padding
// and margin. So <Style extends="..."> cannot be implemented as Inherit alone: TML
// resolves the cascade in its own style model and emits a fully-resolved style here.
// That is required regardless, because layout needs padding and margin at measure
// time, well before any lipgloss.Style exists.
func TestInheritSkipsTheBoxModel(t *testing.T) {
	base := lipgloss.NewStyle().Bold(true).Italic(true).Padding(1, 2).Margin(3)
	derived := lipgloss.NewStyle().Bold(false).Inherit(base)

	assert.False(t, derived.GetBold(), "an explicitly set field survives Inherit")
	assert.True(t, derived.GetItalic(), "an unset text field is inherited")

	top, right, bottom, left := derived.GetPadding()
	assert.Equal(t, []int{0, 0, 0, 0}, []int{top, right, bottom, left}, "padding is never inherited")

	top, right, bottom, left = derived.GetMargin()
	assert.Equal(t, []int{0, 0, 0, 0}, []int{top, right, bottom, left}, "margin is never inherited")
}

// Layer coordinates are relative to the parent layer. This is why arrange can emit
// parent-relative rects and never has to track absolute screen coordinates.
func TestNestedLayerCoordinatesAreParentRelative(t *testing.T) {
	child := lipgloss.NewLayer("X").ID("child").X(1).Y(1)
	parent := lipgloss.NewLayer("....\n....\n....").ID("parent").X(10).Y(5).AddLayers(child)

	comp := lipgloss.NewCompositor(parent)

	hit := comp.Hit(11, 6)
	require.False(t, hit.Empty(), "child sits at parent origin plus its own offset")
	assert.Equal(t, "child", hit.ID())
}

// A layer's size is derived from its content, not set on the layer. TML therefore
// has to render each node to exactly its arranged size, which Style.Width/Height do.
func TestLayerSizeDerivesFromContent(t *testing.T) {
	layer := lipgloss.NewLayer("abc\ndef")

	assert.Equal(t, 3, layer.Width(), "width comes from the content, there is no width setter")
	assert.Equal(t, 2, layer.Height())
}

// Hit testing only considers layers carrying an ID, so TML can opt individual
// elements into mouse routing without giving every node an identity.
func TestHitTestSkipsLayersWithoutAnID(t *testing.T) {
	anonymous := lipgloss.NewLayer("aaaa").X(0).Y(0).Z(0)
	identified := lipgloss.NewLayer("bb").ID("button").X(1).Y(0).Z(1)

	comp := lipgloss.NewCompositor(anonymous, identified)

	assert.Equal(t, "button", comp.Hit(1, 0).ID(), "the topmost identified layer wins")
	assert.True(t, comp.Hit(3, 0).Empty(), "a point covered only by anonymous layers is not a hit")
}

// The compositor sorts by each layer's OWN z value, so a parent's z does not create
// a stacking context for its children: z is global across the whole tree. The
// renderer must therefore allocate z from a single tree-wide counter.
func TestZIndexIsGlobalNotScopedToTheParent(t *testing.T) {
	// A child of a low-z parent still paints above a high-z top-level layer when
	// its own z is higher. Nothing scopes it to the parent's range.
	lowParent := lipgloss.NewLayer("..").ID("low-parent").X(0).Y(0).Z(0).
		AddLayers(lipgloss.NewLayer("C").ID("child-of-low").X(0).Y(0).Z(10))
	highSibling := lipgloss.NewLayer("S").ID("high-sibling").X(0).Y(0).Z(5)

	comp := lipgloss.NewCompositor(lowParent, highSibling)

	assert.Equal(t, "child-of-low", comp.Hit(0, 0).ID(),
		"z is compared globally; a nested layer is not confined to its parent's z range")
}

// Sorting is by z alone and is not stable, so equal-z layers have unspecified paint
// order. The renderer must hand out distinct, increasing z values in document order
// rather than relying on sibling ordering.
func TestOverlappingLayersAreOrderedByDistinctZ(t *testing.T) {
	under := lipgloss.NewLayer("under").ID("under").X(0).Y(0).Z(1)
	over := lipgloss.NewLayer("over").ID("over").X(0).Y(0).Z(2)

	comp := lipgloss.NewCompositor(under, over)

	out := comp.Render()
	t.Logf("composited output: %q", out)

	assert.Equal(t, "over", comp.Hit(0, 0).ID(), "the higher z wins the hit test")
	assert.Contains(t, out, "over", "and wins the painted output")
	assert.NotContains(t, out, "under", "the layer underneath is fully covered")
}

// Style.Render emits ANSI unconditionally for any colour that is set; downsampling
// happens later at the writer. Golden files must therefore either avoid colour or
// compare post-strip, never assume Render produced plain text.
func TestRenderEmitsANSIForColorRegardlessOfTTY(t *testing.T) {
	colored := lipgloss.NewStyle().Foreground(lipgloss.Color("#ff0000")).Render("x")
	plain := lipgloss.NewStyle().Render("x")

	assert.True(t, strings.Contains(colored, "\x1b["), "colour always produces escape sequences, even off a TTY")
	assert.Equal(t, "x", plain, "an unstyled render stays plain, so layout goldens can skip colour entirely")
}
