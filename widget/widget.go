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

// Composer is a widget that wraps the children written inside it rather than
// replacing them: a frame, a dialog, a labelled group.
//
// The division of labour is that widgets draw and panels solve constraints. A
// composer is handed its children already rendered, at the size layout worked
// out from Inset, and returns the block to put on the screen. Everything the
// built-in <Border> and <Popup> do is done through this interface, so a widget
// written outside the language is not a second-class one.
//
// A composer's Measure and Render are still called when the element has no
// children, which is what makes an empty <Border/> draw an empty frame instead
// of nothing.
type Composer interface {
	Native
	// Inset is the space the widget keeps for itself around its children, so
	// layout can measure them against what is actually left.
	Inset() (horizontal, vertical int)
	// Compose draws the widget at the size it was given, around its children.
	Compose(slots Slots, w, h int) string
}

// Layout is how a composer wants its children measured and placed, for the
// cases the default does not cover. The default -- shrink to the children,
// stacked down the page, starting at the top-left -- is the zero value, so a
// widget only says what differs.
type Layout struct {
	// FillW and FillH take all the space on offer rather than shrinking to fit
	// the children. A viewport does: one that shrank to its content would not be
	// a viewport, and its scrollbar would end up somewhere in the middle.
	FillW, FillH bool
	// FreeW and FreeH measure the children against unlimited space, so they are
	// allowed to be bigger than the widget showing them. This is what makes a
	// scrolling region possible at all.
	FreeW, FreeH bool
	// OffsetX and OffsetY shift the children inside the widget. Layout needs to
	// know because a control scrolled halfway off the top is clicked where it is
	// drawn, not where it would otherwise have been.
	OffsetX, OffsetY int
	// ContentW and ContentH are the size of the whole content when the children
	// are only a window over it, and zero when they are all of it.
	//
	// A host with more rows than a frame can afford to lay out hands over the
	// visible ones and says how many there are in total. Without that the extent
	// is whatever arrived, so the scrollbar would measure the window rather than
	// the content and the maximum offset reported back would be nearly zero. The
	// children are placed from the top in this mode, because the host sliced at
	// the offset already; Offset stays the position to report.
	ContentW, ContentH int
}

// Arranger is the optional half of a composer whose children need measuring or
// placing differently from the default.
type Arranger interface {
	Arrange() Layout
}

// Anchored is a widget with an opinion about where it belongs on a canvas. A
// dialog says center, so a popup lands in the middle without the author having
// to position it. An explicit Canvas.anchor still wins.
type Anchored interface {
	DefaultAnchor() string
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
	// Measure is how wide a string is, in cells; nil means lipgloss.Width. A
	// widget measures with this rather than with lipgloss.Width directly, or it
	// sizes itself by one rule inside a layout solved by another.
	Measure Measurer
}

// Measurer reports the width of a string in display cells.
//
// A terminal draws a ZWJ sequence like a family emoji in 2 cells if it agreed to
// mode 2027 and in 6 if it did not, so the width of one string depends on a
// negotiation only the host took part in. lipgloss.Width always answers 2. A
// host that got the other answer and cannot say so measures its own screen one
// way and gets this view laid out the other, four columns apart on that string:
// a row it sized to fit wraps anyway, and a click lands on the wrong element.
type Measurer func(string) int

// Width measures s, falling back to lipgloss when the host had no opinion.
func (m Measurer) Width(s string) int {
	if m == nil {
		return lipgloss.Width(s)
	}
	return m(s)
}

// Slots are a composer's children, already drawn, grouped by the slot they were
// written into. The default slot -- anything written directly inside the
// element -- is the empty name.
type Slots map[string][]string

// Get is the content of one slot.
func (s Slots) Get(name string) []string { return s[name] }

// Default is the content written directly inside the element.
func (s Slots) Default() []string { return s[""] }

// Slotted is the optional half of a factory that accepts named content. The
// names are checked when the view loads, so a misspelt slot is a diagnostic
// rather than content that silently goes nowhere.
type Slotted interface {
	Slots() []string
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

// NewFactory pairs the attribute names a widget reads with the function that
// makes one, which is all a Factory is. The language's own library is built
// through it, so a host binding its own widget writes the same line.
func NewFactory(attrs []string, build func(Context) (Native, error)) Factory {
	return declared{attrs: attrs, build: build}
}

// NewSlottedFactory is NewFactory for a widget that takes named content, so a
// misspelt slot is rejected when the view loads rather than going nowhere.
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

// SlotNames maps each bound widget to the slots it accepts, for the analyzer to
// check property elements against.
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

// BubbleMeasured is Bubble with the host's own width method, for a host that
// negotiated one with its terminal.
func BubbleMeasured(v Viewer, m Measurer) Native { return bubble{v: v, m: m} }

type bubble struct {
	v Viewer
	m Measurer
}

func (b bubble) Measure(maxW, _ int) (int, int) {
	b.resize(maxW)
	out := b.v.View()
	return b.m.Width(out), lipgloss.Height(out)
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
