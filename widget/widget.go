// Package widget lets a host plug its own elements into a TML view.
//
// The host keeps its state and its Update loop. TML only measures a widget,
// places it, and asks it to render into the space it was given, so adopting TML
// never means handing over control of a Bubble Tea program.
package widget

import (
	"io/fs"
	"sort"

	"charm.land/lipgloss/v2"
)

// Native is an element supplied by something other than the language itself.
type Native interface {
	// Measure reports the size the widget wants within the space on offer. A
	// zero maximum means unconstrained.
	Measure(maxW, maxH int) (w, h int)
	// Render draws the widget into the size layout settled on.
	Render(w, h int) string
}

// State is what the view knows about an element's interaction at render time.
// A widget that draws itself differently under the cursor takes it through
// [Stateful].
type State struct {
	// Focused marks the element the keyboard is on.
	Focused bool
	// Hovered marks the element the pointer is over.
	Hovered bool
	// Pressed marks an element held down by the pointer.
	Pressed bool
}

// Stateful is the optional half of a widget that reacts to focus or the
// pointer. The engine calls SetState before measuring and rendering, so a
// widget renders the state of the frame it is in rather than the previous one.
type Stateful interface {
	SetState(State)
}

// Focusable marks a widget that takes part in the focus ring: tab lands on it
// and Enter activates it. A widget that only displays says nothing and is
// skipped.
type Focusable interface {
	// AcceptsFocus reports whether this instance is currently focusable. A
	// disabled control returns false, which is what keeps tab from stopping on
	// something that cannot be used.
	AcceptsFocus() bool
}

// Context is what a factory gets to build one element's widget.
type Context struct {
	// Attrs are the element's evaluated attributes.
	Attrs Attrs
	// FS is the filesystem the view was loaded from, so a widget that reads a
	// file reads it from the same place the templates came from.
	FS fs.FS
	// Dir is the directory of the .tml file the element was written in, which is
	// what a relative path in an attribute is relative to.
	Dir string
	// Dark reports whether the view is rendering against a dark theme.
	Dark bool
}

// Factory builds a widget per element, from that element's attributes.
//
// This is the seam the language's own widgets use. A host widget that owns
// state across frames is bound with [Registry.Bind] instead: one instance, kept
// by the host, told to draw.
type Factory interface {
	// Attributes lists the attribute names this widget consumes. Everything else
	// written on the element is styling, so the two never have to be told apart
	// by guessing.
	Attributes() []string
	// Build makes the widget for one element. A failure here is reported at the
	// element's position, so a bad attribute reads like any other diagnostic.
	Build(Context) (Native, error)
}

// Registry maps element names to the widgets behind them.
//
// Names are resolved by the analyzer, so a template referring to a widget the
// host never bound fails when the view loads rather than rendering a blank.
type Registry struct {
	natives   map[string]Native
	factories map[string]Factory
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{natives: map[string]Native{}, factories: map[string]Factory{}}
}

// Bind makes a single widget instance available under an element name. It
// returns the registry so bindings can be chained.
func (r *Registry) Bind(name string, native Native) *Registry {
	r.natives[name] = native
	delete(r.factories, name)
	return r
}

// BindFactory makes a per-element widget available under an element name.
func (r *Registry) BindFactory(name string, factory Factory) *Registry {
	r.factories[name] = factory
	delete(r.natives, name)
	return r
}

// Lookup resolves an element name to a single bound instance.
func (r *Registry) Lookup(name string) (Native, bool) {
	if r == nil {
		return nil, false
	}
	native, ok := r.natives[name]
	return native, ok
}

// Factory resolves an element name to a per-element widget.
func (r *Registry) Factory(name string) (Factory, bool) {
	if r == nil {
		return nil, false
	}
	factory, ok := r.factories[name]
	return factory, ok
}

// Bound reports whether the name resolves to a widget of either kind.
func (r *Registry) Bound(name string) bool {
	if r == nil {
		return false
	}
	if _, ok := r.natives[name]; ok {
		return true
	}
	_, ok := r.factories[name]
	return ok
}

// Names lists the bound element names, sorted so diagnostics are stable.
func (r *Registry) Names() []string {
	if r == nil {
		return nil
	}
	names := make([]string, 0, len(r.natives)+len(r.factories))
	for name := range r.natives {
		names = append(names, name)
	}
	for name := range r.factories {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Merge layers other's bindings under this registry's own, and returns the
// result without touching either input. The receiver wins every collision, so a
// host that binds its own <Button> gets that one and keeps the rest of the
// library.
func (r *Registry) Merge(other *Registry) *Registry {
	merged := NewRegistry()
	for _, source := range []*Registry{other, r} {
		if source == nil {
			continue
		}
		for name, native := range source.natives {
			merged.Bind(name, native)
		}
		for name, factory := range source.factories {
			merged.BindFactory(name, factory)
		}
	}
	return merged
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
