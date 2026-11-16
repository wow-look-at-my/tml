package widget

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml/cell"
)

// fakeInput stands in for a bubbles component: it draws itself and can be told how wide it is.
type fakeInput struct {
	value string
	width int
}

func (f *fakeInput) View() string {
	if f.width <= 0 {
		return f.value
	}
	if len(f.value) >= f.width {
		return f.value[:f.width]
	}
	return f.value + strings.Repeat(".", f.width-len(f.value))
}

func (f *fakeInput) SetWidth(w int) { f.width = w }

// viewOnly has no SetWidth, which many components do not.
type viewOnly struct{ text string }

func (v viewOnly) View() string { return v.text }

// draw paints a widget on a buffer of the given size. widgettest imports this package, so a test here cannot use it.
func draw(native Native, w, h int) *cell.Buffer {
	buf := cell.NewBuffer(w, h)
	native.Draw(buf.Canvas(nil))
	return buf
}

func TestFromViewerMeasuresWhatTheComponentDraws(t *testing.T) {
	native := FromViewer(viewOnly{text: "ab\ncd"}, nil)

	w, h := native.Measure(0, 0)
	assert.Equal(t, 2, w)
	assert.Equal(t, 2, h)
}

// A styled view takes the cells its text shows, not the bytes of its escapes.
func TestFromViewerMeasuresStyledTextByItsCells(t *testing.T) {
	native := FromViewer(viewOnly{text: "\x1b[1mbold\x1b[m"}, nil)

	w, h := native.Measure(0, 0)
	assert.Equal(t, 4, w)
	assert.Equal(t, 1, h)

	buf := draw(native, 4, 1)
	assert.Equal(t, "bold", buf.Text())
	assert.True(t, buf.At(0, 0).Style.Has(cell.Bold), "the SGR in the view styles the cells")
}

// The measurer decides how wide a grapheme cluster is.
func TestFromViewerMeasuresWithTheMeasurer(t *testing.T) {
	wide := func(string) int { return 2 }
	w, _ := FromViewer(viewOnly{text: "abc"}, wide).Measure(0, 0)
	assert.Equal(t, 6, w)
}

// A component that accepts a width is told the space layout gave it, which is how a widget fills a star-sized cell.
func TestFromViewerPassesTheWidthThrough(t *testing.T) {
	input := &fakeInput{value: "hi"}
	native := FromViewer(input, nil)

	assert.Equal(t, "hi........", draw(native, 10, 1).Text())
	assert.Equal(t, 10, input.width, "the component was told its width")
}

// A component without SetWidth is left alone rather than being forced.
func TestFromViewerToleratesAComponentThatCannotResize(t *testing.T) {
	native := FromViewer(viewOnly{text: "fixed"}, nil)
	assert.Equal(t, "fixed", strings.TrimRight(draw(native, 40, 1).Text(), " "))
}

func TestRegistryBindingAndNames(t *testing.T) {
	registry := NewRegistry().
		Bind("Search", FromViewer(&fakeInput{value: "q"}, nil)).
		Bind("Alpha", FromViewer(viewOnly{text: "a"}, nil))

	assert.Equal(t, []string{"Alpha", "Search"}, registry.Names(), "names are sorted for stable diagnostics")

	native, ok := registry.Lookup("Search")
	require.True(t, ok)
	assert.NotNil(t, native)

	_, ok = registry.Lookup("Missing")
	assert.False(t, ok)
}

// A view with no widgets passes a nil registry, so lookups must be safe on nil.
func TestNilRegistryIsUsable(t *testing.T) {
	var registry *Registry

	_, ok := registry.Lookup("anything")
	assert.False(t, ok)
	_, ok = registry.Factory("anything")
	assert.False(t, ok)
	assert.False(t, registry.Bound("anything"))
	assert.Empty(t, registry.Names())
	assert.Empty(t, registry.Merge(nil).Names())
}

// stub is a factory that builds nothing, for testing the registry itself.
type stub struct{ attrs []string }

func (s stub) Attributes() []string { return s.attrs }

func (s stub) Build(Context) (Native, error) { return FromViewer(viewOnly{text: "stub"}, nil), nil }

func TestRegistryHoldsBothKindsOfBinding(t *testing.T) {
	registry := NewRegistry().
		Bind("Search", FromViewer(&fakeInput{value: "q"}, nil)).
		BindFactory("Rule", stub{attrs: []string{"char"}})

	assert.Equal(t, []string{"Rule", "Search"}, registry.Names())
	assert.True(t, registry.Bound("Search"))
	assert.True(t, registry.Bound("Rule"))

	factory, ok := registry.Factory("Rule")
	require.True(t, ok)
	assert.Equal(t, []string{"char"}, factory.Attributes())

	_, ok = registry.Lookup("Rule")
	assert.False(t, ok, "a factory is not a bound instance")
}

// Rebinding a name has to drop the other kind of binding, or the stale binding wins every lookup and the new binding
func TestRebindingReplacesTheOtherKind(t *testing.T) {
	registry := NewRegistry().BindFactory("Button", stub{}).Bind("Button", FromViewer(viewOnly{text: "host"}, nil))
	_, ok := registry.Factory("Button")
	assert.False(t, ok)

	registry.BindFactory("Button", stub{})
	_, ok = registry.Lookup("Button")
	assert.False(t, ok)
}

// A host binding its own <Button> keeps the rest of the library rather than having to shadow every name to get its own
func TestMergeLetsTheHostWin(t *testing.T) {
	library := NewRegistry().BindFactory("Button", stub{attrs: []string{"label"}}).BindFactory("Rule", stub{})
	host := NewRegistry().Bind("Button", FromViewer(viewOnly{text: "host"}, nil))

	merged := host.Merge(library)
	assert.Equal(t, []string{"Button", "Rule"}, merged.Names())

	native, ok := merged.Lookup("Button")
	require.True(t, ok, "the host's binding survived")
	assert.Equal(t, "host", draw(native, 4, 1).Text())

	_, ok = merged.Factory("Rule")
	assert.True(t, ok, "the library's other widgets came along")

	_, ok = library.Lookup("Button")
	assert.False(t, ok, "merging left the inputs alone")
}
