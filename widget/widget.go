// Package widget lets a host plug its own elements into a TML view.
//
// The host keeps its state and its Update loop. TML only measures a widget,
// places it, and asks it to render into the space it was given, so adopting TML
// never means handing over control of a Bubble Tea program.
package widget

import (
	"sort"

	"charm.land/lipgloss/v2"
)

// Native is an element supplied by the host rather than by the language.
type Native interface {
	// Measure reports the size the widget wants within the space on offer. A
	// zero maximum means unconstrained.
	Measure(maxW, maxH int) (w, h int)
	// Render draws the widget into the size layout settled on.
	Render(w, h int) string
}

// Registry maps element names to the widgets behind them.
//
// Names are resolved by the analyzer, so a template referring to a widget the
// host never bound fails when the view loads rather than rendering a blank.
type Registry struct {
	natives map[string]Native
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{natives: map[string]Native{}}
}

// Bind makes a widget available under an element name. It returns the registry
// so bindings can be chained.
func (r *Registry) Bind(name string, native Native) *Registry {
	r.natives[name] = native
	return r
}

// Lookup resolves an element name.
func (r *Registry) Lookup(name string) (Native, bool) {
	if r == nil {
		return nil, false
	}
	native, ok := r.natives[name]
	return native, ok
}

// Names lists the bound element names, sorted so diagnostics are stable.
func (r *Registry) Names() []string {
	if r == nil {
		return nil
	}
	names := make([]string, 0, len(r.natives))
	for name := range r.natives {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Viewer is the part of a Bubble Tea component TML needs: something that can
// draw itself. Every bubbles component satisfies it.
type Viewer interface {
	View() string
}

// Sizer is the optional half: a component that can be told how wide it is.
// bubbles components that support it expose SetWidth, and TML uses it so a
// widget fills the space layout gave it.
type Sizer interface {
	SetWidth(int)
}

// Bubble adapts a Bubble Tea component into a native element.
//
// Pass a pointer, as in Bubble(&m.input): SetWidth has a pointer receiver, and
// without one the width TML computes would be set on a copy and thrown away.
func Bubble(v Viewer) Native { return bubble{v: v} }

type bubble struct{ v Viewer }

func (b bubble) Measure(maxW, _ int) (int, int) {
	b.resize(maxW)
	out := b.v.View()
	return lipgloss.Width(out), lipgloss.Height(out)
}

func (b bubble) Render(w, _ int) string {
	b.resize(w)
	return b.v.View()
}

func (b bubble) resize(w int) {
	if sizer, ok := b.v.(Sizer); ok && w > 0 {
		sizer.SetWidth(w)
	}
}
