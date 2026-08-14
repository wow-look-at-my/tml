package widget

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeInput stands in for a bubbles component: it draws itself and can be told
// how wide it is.
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

// A component that accepts a width is told the space layout gave it, which is
// how a widget fills a star-sized cell.
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

// A view with no widgets passes a nil registry, so lookups must be safe on one.
func TestNilRegistryIsUsable(t *testing.T) {
	var registry *Registry

	_, ok := registry.Lookup("anything")
	assert.False(t, ok)
	assert.Empty(t, registry.Names())
}
