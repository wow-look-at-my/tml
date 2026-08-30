package widget

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestBubbleMeasuresWhatTheComponentDraws(t *testing.T) {
	native := Bubble(viewOnly{text: "ab\ncd"})

	w, h := native.Measure(0, 0)
	assert.Equal(t, 2, w)
	assert.Equal(t, 2, h)
}

// A component that accepts a width is told the space layout gave it, which is how a widget fills a star-sized cell.
func TestBubblePassesTheWidthThrough(t *testing.T) {
	input := &fakeInput{value: "hi"}
	native := Bubble(input)

	assert.Equal(t, "hi........", native.Render(10, 1))
	assert.Equal(t, 10, input.width, "the component was told its width")
}

// A component without SetWidth is left alone rather than being forced.
func TestBubbleToleratesAComponentThatCannotResize(t *testing.T) {
	native := Bubble(viewOnly{text: "fixed"})
	assert.Equal(t, "fixed", native.Render(40, 1))
}

func TestRegistryBindingAndNames(t *testing.T) {
	registry := NewRegistry().
		Bind("Search", Bubble(&fakeInput{value: "q"})).
		Bind("Alpha", Bubble(viewOnly{text: "a"}))

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

func (s stub) Build(Context) (Native, error) { return Bubble(viewOnly{text: "stub"}), nil }

func TestRegistryHoldsBothKindsOfBinding(t *testing.T) {
	registry := NewRegistry().
		Bind("Search", Bubble(&fakeInput{value: "q"})).
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
	registry := NewRegistry().BindFactory("Button", stub{}).Bind("Button", Bubble(viewOnly{text: "host"}))
	_, ok := registry.Factory("Button")
	assert.False(t, ok)

	registry.BindFactory("Button", stub{})
	_, ok = registry.Lookup("Button")
	assert.False(t, ok)
}

// A host binding its own <Button> keeps the rest of the library rather than having to shadow every name to get its own
func TestMergeLetsTheHostWin(t *testing.T) {
	library := NewRegistry().BindFactory("Button", stub{attrs: []string{"label"}}).BindFactory("Rule", stub{})
	host := NewRegistry().Bind("Button", Bubble(viewOnly{text: "host"}))

	merged := host.Merge(library)
	assert.Equal(t, []string{"Button", "Rule"}, merged.Names())

	native, ok := merged.Lookup("Button")
	require.True(t, ok, "the host's binding survived")
	assert.Equal(t, "host", native.Render(0, 0))

	_, ok = merged.Factory("Rule")
	assert.True(t, ok, "the library's other widgets came along")

	_, ok = library.Lookup("Button")
	assert.False(t, ok, "merging left the inputs alone")
}
