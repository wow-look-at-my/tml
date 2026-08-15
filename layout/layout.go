// Package layout turns an expanded TML tree into sized, positioned boxes.
//
// Two passes, as in XAML: measure asks every node how big it wants to be within
// the space on offer, then arrange hands each node the space it actually gets.
// Sizes are terminal cells throughout.
//
// A child's rect is relative to its parent's CONTENT origin -- the first cell
// inside the parent's margin, border and padding. That is the same origin
// lipgloss composes into when it renders a styled block, so the rects and the
// rendered output cannot drift apart.
package layout

import (
	"fmt"
	"io/fs"
	"path"
	"strconv"
	"strings"

	"github.com/wow-look-at-my/tml/sema"
	"github.com/wow-look-at-my/tml/style"
	"github.com/wow-look-at-my/tml/syntax"
	"github.com/wow-look-at-my/tml/widget"
)

// Size is a measurement in terminal cells.
type Size struct{ W, H int }

// Rect is a position and size relative to the parent's content origin.
type Rect struct{ X, Y, W, H int }

// Constraints is the space available to a node.
type Constraints struct{ MaxW, MaxH int }

// Box is a laid-out node.
type Box struct {
	Name string
	// Rect is the box's outer rect, margin included, relative to the parent's
	// content origin.
	Rect Rect
	// Screen is the same box in viewport coordinates, with margin excluded so it
	// covers the cells the box actually paints. This is what a pointer is tested
	// against.
	Screen Rect
	// Clip is the region an ancestor still shows. It is the viewport for
	// anything outside a Scrollbox, and the visible part of one inside.
	Clip Rect
	// Content is the size available inside margin, border and padding. It is
	// what a bound widget is told to render into.
	Content Size
	Style   style.Resolved
	Text    string
	// Native is the widget behind this element, if any. Layout measures it and
	// the renderer asks it to draw; TML never touches its state.
	Native   widget.Native
	Children []*Box
	Pos      syntax.Pos

	// ID and Action identify an interactive element to the host: ID names it,
	// Action is what it reports when activated.
	ID     string
	Action string
	// State is how this element renders in this frame -- focused, hovered, held.
	State widget.State

	// focus reports whether the keyboard can land here. An element with an id
	// but no focus still answers the pointer.
	focus   bool
	attrs   map[string]string
	slot    string
	desired Size
	width   sema.Length
	height  sema.Length

	// Grid state: the track definitions, the sizes auto tracks measured, and
	// this box's own placement within its parent grid.
	cols, rows              []sema.Length
	autoWidths, autoHeights []int
	place                   placement

	// canvas is this box's placement within a parent Canvas; scroll is where a
	// scrolling region's content ended up, which is what the frame reports back
	// to the host.
	canvas canvasPlacement
	scroll Scroll
}

// Options configure an engine.
type Options struct {
	// Widgets resolves element names to widgets.
	Widgets *widget.Registry
	// FS is the filesystem the view was loaded from, handed to any widget that
	// reads a file.
	FS fs.FS
	// Dark reports whether the view renders against a dark theme.
	Dark bool
	// Interaction carries focus and pointer state across frames. A nil one means
	// nothing is focusable, which is what a static render wants.
	Interaction Interaction
	// Measure is how wide a string is, in cells. Nil means lipgloss.Width.
	Measure widget.Measurer
}

// Engine lays out expanded trees against a stylesheet.
type Engine struct {
	sheet *style.Sheet
	opts  Options
}

// New returns an engine that resolves named styles through sheet and widgets
// through opts.
func New(sheet *style.Sheet, opts Options) *Engine {
	return &Engine{sheet: sheet, opts: opts}
}

// layoutAttrs are consumed by the engine; every other attribute is styling,
// unless a widget claims it. Attached properties are recognised by their dot and
// handled separately.
var layoutAttrs = map[string]bool{
	"width": true, "height": true, "orientation": true, "gap": true, "style": true,
	"columns": true, "rows": true, "id": true, "action": true,
	"offset": true, "offsetX": true, "scrollbar": true,
	"title": true, "titleAlign": true,
}

func isLayoutAttr(name string) bool {
	return layoutAttrs[name] || strings.Contains(name, ".")
}

// Layout measures and arranges the tree inside a viewport.
func (e *Engine) Layout(node *sema.Node, width, height int) (*Box, error) {
	p := &pass{e: e}
	box, err := p.build(node)
	if err != nil {
		return nil, err
	}
	p.syncState()

	e.measure(box, Constraints{MaxW: width, MaxH: height})

	// The root fills the viewport unless it asked for a specific size. Filling
	// is what a view wants by default, and it is also what gives a star-sized
	// descendant something to fill.
	rect := Rect{W: width, H: height}
	if box.width.Kind == sema.LengthCells {
		rect.W = min(box.desired.W, width)
	}
	if box.height.Kind == sema.LengthCells {
		rect.H = min(box.desired.H, height)
	}
	e.arrange(box, rect)
	setScreen(box, 0, 0, Rect{W: width, H: height})
	p.publish()
	return box, nil
}

func (p *pass) build(node *sema.Node) (*Box, error) {
	e := p.e
	if node.Kind == syntax.TextNode {
		return &Box{Name: "#text", Text: node.Text, Pos: node.Pos, attrs: map[string]string{}}, nil
	}

	// A node carrying a component's name is never a widget, however the component
	// happens to be called.
	factory, hasFactory := e.opts.Widgets.Factory(node.Name)
	hasFactory = hasFactory && !node.Component
	claimed := map[string]bool{}
	if hasFactory {
		for _, name := range factory.Attributes() {
			claimed[name] = true
		}
	}

	attrs := make(map[string]string, len(node.Attrs))
	inline := map[string]string{}
	for name, value := range node.Attrs {
		attrs[name] = value.String()
		if !isLayoutAttr(name) && !claimed[name] {
			inline[name] = value.String()
		}
	}
	resolved, err := e.sheet.Resolve(attrs["style"], inline)
	if err != nil {
		return nil, &syntax.Error{Pos: node.Pos, Message: fmt.Sprintf("<%s>: %v", node.Name, err)}
	}

	box := &Box{
		Name:   node.Name,
		Style:  resolved,
		Pos:    node.Pos,
		attrs:  attrs,
		slot:   node.Slot,
		ID:     attrs["id"],
		Action: attrs["action"],
	}
	if box.width, err = lengthAttr(node, "width"); err != nil {
		return nil, err
	}
	if box.height, err = lengthAttr(node, "height"); err != nil {
		return nil, err
	}

	// Text collapses its character data into one string; every other element
	// keeps its children as boxes.
	if node.Name == "Text" {
		box.Text = textOf(node)
		return box, nil
	}
	if native, ok := e.opts.Widgets.Lookup(node.Name); ok && !node.Component {
		box.Native = native
	} else if hasFactory {
		native, err := factory.Build(widget.Context{
			Attrs:   widget.NewAttrs(node.Name, node.Attrs, node.Order),
			FS:      e.opts.FS,
			Dir:     path.Dir(node.Pos.File),
			Dark:    e.opts.Dark,
			Measure: e.opts.Measure,
		})
		if err != nil {
			return nil, &syntax.Error{Pos: node.Pos, Message: err.Error()}
		}
		box.Native = native
	}
	if box.Native != nil {
		if err := p.track(box); err != nil {
			return nil, err
		}
		// A composer keeps its children: they are laid out inside whatever space
		// it insets for itself, and handed back to it already drawn. Anything else
		// draws itself, so whatever was written inside it is not layout's to place.
		if _, wraps := box.Native.(widget.Composer); !wraps {
			return box, nil
		}
	}
	for _, child := range node.Children {
		built, err := p.build(child)
		if err != nil {
			return nil, err
		}
		box.Children = append(box.Children, built)
	}

	switch box.Name {
	case "Grid":
		if err := initGrid(box); err != nil {
			return nil, err
		}
	case "Canvas":
		if err := initCanvas(box); err != nil {
			return nil, err
		}
	default:
		if err := rejectAttachedProperties(box); err != nil {
			return nil, err
		}
	}
	return box, nil
}

// rejectAttachedProperties catches Grid.row and friends written on a child of
// something that is not a Grid. Ignoring them would leave the author staring at
// a layout that quietly disregards what they wrote.
func rejectAttachedProperties(box *Box) error {
	for _, child := range box.Children {
		for name := range child.attrs {
			owner, _, dotted := strings.Cut(name, ".")
			if !dotted {
				continue
			}
			return &syntax.Error{Pos: child.Pos, Message: fmt.Sprintf(
				"attached property %q only applies to a child of <%s>, but this is inside <%s>", name, owner, box.Name)}
		}
	}
	return nil
}

func textOf(node *sema.Node) string {
	var b strings.Builder
	for _, child := range node.Children {
		if child.Kind == syntax.TextNode {
			b.WriteString(child.Text)
		}
	}
	return b.String()
}

func lengthAttr(node *sema.Node, name string) (sema.Length, error) {
	value, ok := node.Attr(name)
	if !ok {
		return sema.Length{Kind: sema.LengthAuto}, nil
	}
	if value.Type().Kind == sema.KindLength {
		return value.Length(), nil
	}
	length, err := sema.ParseLength(value.String())
	if err != nil {
		return sema.Length{}, &syntax.Error{Pos: node.Pos, Message: fmt.Sprintf("<%s> %s: %v", node.Name, name, err)}
	}
	return length, nil
}

// outer is the space a box consumes beyond its content: margin plus the frame
// lipgloss draws (padding and border).
func (b *Box) outer() (w, h int) {
	frameW, frameH := b.Style.Frame()
	return frameW + b.Style.Margin.Horizontal(), frameH + b.Style.Margin.Vertical()
}

// Vertical reports the stack direction. Vertical is the default because a
// column is the common case in a terminal.
func (b *Box) Vertical() bool {
	if value, ok := b.attrs["orientation"]; ok {
		return value != "horizontal"
	}
	return true
}

// Gap is the blank run between stacked children.
func (b *Box) Gap() int { return b.gap() }

func (b *Box) vertical() bool { return b.Vertical() }

func (b *Box) gap() int {
	value, ok := b.attrs["gap"]
	if !ok {
		return 0
	}
	n, err := strconv.Atoi(value)
	if err != nil || n < 0 {
		return 0
	}
	return n
}
