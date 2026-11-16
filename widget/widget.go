// Package widget lets a host plug its own elements into a TML view. The host keeps its state and its Update loop. TML
package widget

import (
	"io/fs"
	"sort"

	"github.com/wow-look-at-my/tml/cell"
	"github.com/wow-look-at-my/tml/input"
	"github.com/wow-look-at-my/tml/sema"
)

// Native is an element supplied by something other than the language itself.
type Native interface {
	// Measure reports the size the widget wants within the space on offer. A nothing maximum means unconstrained.
	Measure(maxW, maxH int) (w, h int)
	// Draw paints the widget on a canvas of the size layout settled on.
	Draw(c cell.Canvas)
}

// State is what the view knows about an element's interaction at render time. A widget that draws itself differently
type State struct {
	// Focused marks the element the keyboard is on.
	Focused bool
	// Hovered marks the element the pointer is over.
	Hovered bool
	// Pressed marks an element held down by the pointer.
	Pressed bool
}

// Stateful is the optional half of a widget that reacts to focus or the pointer. The engine calls SetState before
type Stateful interface {
	SetState(State)
}

// Focusable marks a widget that takes part in the focus ring: tab lands on it and Enter activates it.
type Focusable interface {
	// AcceptsFocus reports whether this instance is focusable. A disabled control returns false, which is what
	AcceptsFocus() bool
}

// Animator is the optional half of a widget that changes without input from the host.
type Animator interface {
	// Animating reports whether the widget still changes on its own.
	Animating() bool
}

// Composer is a widget that wraps the children written inside it rather than replacing them: a frame, a dialog.
type Composer interface {
	Native
	// Insets is the space the widget keeps for itself on each side of its children.
	Insets() sema.Thickness
}

// Overlay is the optional half of a composer that draws over its children
// once they are painted: a scrollbar, a fade.
type Overlay interface {
	Overlay(c cell.Canvas)
}

// Scroll is how far a scrolling region's content is shifted, and how far it could be.
type Scroll struct{ X, Y, MaxX, MaxY int }

// Scrolling is the optional half of a composer that scrolls.
type Scrolling interface {
	SetScroll(Scroll)
}

// Styler is the optional half of a composer whose children inherit a style from it.
type Styler interface {
	ContentStyle() cell.Style
}

// Edit is what a key did to a widget's value.
type Edit struct {
	Value  string
	Cursor int
}

// Keyed is the optional half of a focusable widget that takes keys itself while it holds the focus. A text field does.
type Keyed interface {
	// Key offers the widget one key. It reports the edit the key makes, and false for a key it has no use for.
	Key(key input.Key) (Edit, bool)
}

// Layout is how a composer wants its children measured and placed, for the cases the default does not cover.
type Layout struct {
	// FillW and FillH take all the space on offer rather than shrinking to fit the children. A viewport does, and so does anything that scrolls.
	FillW, FillH bool
	// FreeW and FreeH measure the children against unlimited space, so they are allowed to be bigger than the widget
	FreeW, FreeH bool
	// OffsetX and OffsetY shift the children inside the widget. Layout needs to know because a control scrolled halfway
	OffsetX, OffsetY int
	// ContentW and ContentH are the size of the whole content when the children are only a window over it.
	ContentW, ContentH int
	// CenterX puts each child in the middle of the width rather than at the left. The widget must draw it there too.
	CenterX bool
}

// Templated is a composer whose look is another element wrapped around its
// children, the way a WPF button is a Border around its content.
type Templated interface {
	Composer
	// Template names the element the engine wraps the children in, with its attributes.
	Template() (element string, attrs map[string]string, fallback string)
	// Bind hands the widget the element the engine built, so its own state can steer how that element draws.
	Bind(chrome Native) error
}

// Arranger is the optional half of a composer whose children need measuring or placing differently from the default.
type Arranger interface {
	Arrange() Layout
}

// Anchored is a widget with an opinion about where it belongs on a canvas. A dialog says center.
type Anchored interface {
	DefaultAnchor() string
}

// Context is what a factory gets to build a single element's widget.
type Context struct {
	// Attrs are the element's evaluated attributes.
	Attrs Attrs
	// FS is the filesystem the view was loaded from.
	FS fs.FS
	// Dir is the directory of the .tml file the element was written in.
	Dir string
	// Dark reports whether the view is rendering against a dark theme.
	Dark bool
	// Measure is how wide a grapheme cluster is, in cells. Nil is the Unicode default.
	Measure Measurer
	// Frame is the animation step the engine is on; a widget that animates draws it, others ignore it.
	Frame int
}

// Measurer reports the width of a grapheme cluster in display cells. See cell.Measure for why a host supplies one.
type Measurer = cell.Measure

// Slotted is the optional half of a factory that accepts named content. The names are checked when the view loads.
type Slotted interface {
	Slots() []string
}

// Factory builds a widget per element, from that element's attributes. This is the seam the language's own widgets
type Factory interface {
	// Attributes lists the attribute names this widget consumes. Everything else written on the element is styling.
	Attributes() []string
	// Build makes the widget for a single element. A failure here is reported at the element's position, so a bad attribute
	Build(Context) (Native, error)
}

// NewFactory pairs the attribute names a widget reads with the function that makes the widget, which is all a Factory is.
func NewFactory(attrs []string, build func(Context) (Native, error)) Factory {
	return declared{attrs: attrs, build: build}
}

// NewSlottedFactory is NewFactory for a widget that takes named content, so a misspelt slot is rejected when the view
func NewSlottedFactory(attrs, slots []string, build func(Context) (Native, error)) Factory {
	return declared{attrs: attrs, slots: slots, build: build}
}

type declared struct {
	attrs []string
	slots []string
	build func(Context) (Native, error)
}

func (d declared) Attributes() []string { return d.attrs }

func (d declared) Slots() []string { return d.slots }

func (d declared) Build(ctx Context) (Native, error) { return d.build(ctx) }

// Registry maps element names to the widgets behind them. Names are resolved by the analyzer, so a template referring
type Registry struct {
	natives   map[string]Native
	factories map[string]Factory
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{natives: map[string]Native{}, factories: map[string]Factory{}}
}

// Bind makes a single widget instance available under an element name. It returns the registry so bindings can be
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

// SlotNames maps each bound widget to the slots it accepts, for the analyzer to check property elements against.
func (r *Registry) SlotNames() map[string][]string {
	if r == nil {
		return nil
	}
	slots := map[string][]string{}
	for name, factory := range r.factories {
		if slotted, ok := factory.(Slotted); ok {
			slots[name] = slotted.Slots()
		}
	}
	return slots
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

// Merge layers other's bindings under this registry's own, and returns the result without touching either input.
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

// Viewer is a component that draws itself as a string, styled with SGR or not. It is the seam for code that was not
// written against a canvas.
type Viewer interface {
	View() string
}

// Sizer is the optional half of a Viewer that can be told how wide it is.
type Sizer interface {
	SetWidth(int)
}

// FromViewer adapts a Viewer into a native element. Pass a pointer, as in FromViewer(&m.table, nil).
func FromViewer(v Viewer, m Measurer) Native { return viewer{v: v, m: m} }

type viewer struct {
	v Viewer
	m Measurer
}

func (b viewer) Measure(maxW, _ int) (int, int) {
	b.resize(maxW)
	out := b.v.View()
	return b.m.Width(out), cell.Height(out)
}

func (b viewer) Draw(c cell.Canvas) {
	w, _ := c.Size()
	b.resize(w)
	c.Text(0, 0, b.v.View(), cell.Style{})
}

func (b viewer) resize(w int) {
	if sizer, ok := b.v.(Sizer); ok && w > 0 {
		sizer.SetWidth(w)
	}
}
