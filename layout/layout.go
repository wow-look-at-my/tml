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
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"

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
	// Rect is the box's outer rect, margin included.
	Rect Rect
	// Content is the size available inside margin, border and padding. It is
	// what a bound widget is told to render into.
	Content Size
	Style   style.Resolved
	Text    string
	// Native is the host widget behind this element, if any. Layout measures it
	// and the renderer asks it to draw; TML never touches its state.
	Native   widget.Native
	Children []*Box
	Pos      syntax.Pos

	attrs   map[string]string
	desired Size
	width   sema.Length
	height  sema.Length
}

// Engine lays out expanded trees against a stylesheet.
type Engine struct {
	sheet    *style.Sheet
	registry *widget.Registry
}

// New returns an engine that resolves named styles through sheet and host
// widgets through registry. A nil registry means the view uses no widgets.
func New(sheet *style.Sheet, registry *widget.Registry) *Engine {
	return &Engine{sheet: sheet, registry: registry}
}

// layoutAttrs are consumed by the engine; every other attribute is styling.
var layoutAttrs = map[string]bool{
	"width": true, "height": true, "orientation": true, "gap": true, "style": true,
}

// Layout measures and arranges the tree inside a viewport.
func (e *Engine) Layout(node *sema.Node, width, height int) (*Box, error) {
	box, err := e.build(node)
	if err != nil {
		return nil, err
	}
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
	return box, nil
}

func (e *Engine) build(node *sema.Node) (*Box, error) {
	if node.Kind == syntax.TextNode {
		return &Box{Name: "#text", Text: node.Text, Pos: node.Pos, attrs: map[string]string{}}, nil
	}

	attrs := make(map[string]string, len(node.Attrs))
	inline := map[string]string{}
	for name, value := range node.Attrs {
		attrs[name] = value.String()
		if !layoutAttrs[name] {
			inline[name] = value.String()
		}
	}
	resolved, err := e.sheet.Resolve(attrs["style"], inline)
	if err != nil {
		return nil, &syntax.Error{Pos: node.Pos, Message: fmt.Sprintf("<%s>: %v", node.Name, err)}
	}

	box := &Box{Name: node.Name, Style: resolved, Pos: node.Pos, attrs: attrs}
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
	if native, ok := e.registry.Lookup(node.Name); ok {
		box.Native = native
		return box, nil
	}
	for _, child := range node.Children {
		built, err := e.build(child)
		if err != nil {
			return nil, err
		}
		box.Children = append(box.Children, built)
	}
	return box, nil
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

// measure computes each box's desired outer size within the given constraints.
func (e *Engine) measure(box *Box, c Constraints) Size {
	outerW, outerH := box.outer()
	inner := Constraints{MaxW: max(0, c.MaxW-outerW), MaxH: max(0, c.MaxH-outerH)}

	var content Size
	switch {
	case box.Native != nil:
		// A host widget measures itself. TML supplies the space and takes the
		// answer, so a bubbles component keeps deciding its own shape.
		w, h := box.Native.Measure(inner.MaxW, inner.MaxH)
		content = Size{W: w, H: h}
	default:
		content = e.measureContent(box, inner)
	}

	// An explicit size overrides what the content asked for. A star size cannot
	// be known until the parent shares out its leftover space, so it measures as
	// zero here and is filled in during arrange.
	switch box.width.Kind {
	case sema.LengthCells:
		content.W = max(0, box.width.Cells-outerW)
	case sema.LengthStar:
		content.W = 0
	}
	switch box.height.Kind {
	case sema.LengthCells:
		content.H = max(0, box.height.Cells-outerH)
	case sema.LengthStar:
		content.H = 0
	}

	box.desired = Size{W: content.W + outerW, H: content.H + outerH}
	return box.desired
}

func (e *Engine) measureContent(box *Box, inner Constraints) Size {
	var content Size
	switch box.Name {
	case "#text":
		content = Size{W: lipgloss.Width(box.Text), H: lipgloss.Height(box.Text)}
	case "Text":
		content = measureText(box, inner)
	case "Spacer":
		content = Size{}
	case "Stack":
		content = e.measureStack(box, inner)
	default:
		content = e.measureChildren(box, inner)
	}
	return content
}

// measureText reports the height the text needs once wrapped, because lipgloss
// wraps rather than truncates and the parent has to budget for the extra lines.
func measureText(box *Box, inner Constraints) Size {
	if box.Text == "" {
		return Size{}
	}
	natural := lipgloss.Width(box.Text)
	if inner.MaxW <= 0 || natural <= inner.MaxW {
		return Size{W: natural, H: lipgloss.Height(box.Text)}
	}
	wrapped := lipgloss.NewStyle().Width(inner.MaxW).Render(box.Text)
	return Size{W: inner.MaxW, H: lipgloss.Height(wrapped)}
}

func (e *Engine) measureChildren(box *Box, inner Constraints) Size {
	var content Size
	for _, child := range box.Children {
		size := e.measure(child, inner)
		content.W = max(content.W, size.W)
		content.H = max(content.H, size.H)
	}
	return fillForStars(box, content, inner)
}

// fillForStars propagates a star size up the tree.
//
// A star child fills the space its parent gives it, so a parent that only ever
// shrank to its content would leave that child nothing to fill and the star
// would collapse. A container holding a star child therefore asks for all the
// space available on that axis. Without this, `width="*"` inside an auto-sized
// ancestor silently does nothing.
func fillForStars(box *Box, content Size, inner Constraints) Size {
	for _, child := range box.Children {
		if child.width.Kind == sema.LengthStar {
			content.W = max(content.W, inner.MaxW)
		}
		if child.height.Kind == sema.LengthStar {
			content.H = max(content.H, inner.MaxH)
		}
	}
	return content
}

func (e *Engine) measureStack(box *Box, inner Constraints) Size {
	vertical := box.vertical()
	gap := box.gap()

	var content Size
	for i, child := range box.Children {
		size := e.measure(child, inner)
		if vertical {
			content.H += size.H
			content.W = max(content.W, size.W)
			if i > 0 {
				content.H += gap
			}
		} else {
			content.W += size.W
			content.H = max(content.H, size.H)
			if i > 0 {
				content.W += gap
			}
		}
	}
	return fillForStars(box, content, inner)
}

// arrange assigns a final rect to a box and places its children inside it.
func (e *Engine) arrange(box *Box, rect Rect) {
	box.Rect = rect

	outerW, outerH := box.outer()
	box.Content = Size{W: max(0, rect.W-outerW), H: max(0, rect.H-outerH)}

	if box.Native != nil {
		return
	}
	switch box.Name {
	case "Text", "Spacer", "#text":
		return
	case "Stack":
		e.arrangeStack(box)
	default:
		// A decorator gives each child the whole content box, clamped to what
		// the child asked for unless it is star-sized and wants to fill.
		for _, child := range box.Children {
			w, h := child.desired.W, child.desired.H
			if child.width.Kind == sema.LengthStar {
				w = box.Content.W
			}
			if child.height.Kind == sema.LengthStar {
				h = box.Content.H
			}
			e.arrange(child, Rect{W: min(w, box.Content.W), H: min(h, box.Content.H)})
		}
	}
}

func (e *Engine) arrangeStack(box *Box) {
	if len(box.Children) == 0 {
		return
	}
	vertical := box.vertical()
	gap := box.gap()

	available := box.Content.W
	if vertical {
		available = box.Content.H
	}
	available = max(0, available-gap*(len(box.Children)-1))

	// Fixed and auto children keep what they asked for; the remainder is shared
	// out by star weight.
	used, weight := 0, 0
	for _, child := range box.Children {
		length, size := child.width, child.desired.W
		if vertical {
			length, size = child.height, child.desired.H
		}
		if length.Kind == sema.LengthStar {
			weight += length.Weight
			continue
		}
		used += size
	}
	leftover := max(0, available-used)

	offset := 0
	remaining := leftover
	remainingWeight := weight

	for i, child := range box.Children {
		mainLength, crossLength := child.width, child.height
		main := child.desired.W
		crossAvailable := box.Content.H
		cross := child.desired.H
		if vertical {
			mainLength, crossLength = child.height, child.width
			main = child.desired.H
			crossAvailable = box.Content.W
			cross = child.desired.W
		}

		if mainLength.Kind == sema.LengthStar {
			// The last star child takes the rounding remainder so the row or
			// column always fills exactly.
			if remainingWeight == mainLength.Weight {
				main = remaining
			} else {
				main = leftover * mainLength.Weight / weight
			}
			remaining -= main
			remainingWeight -= mainLength.Weight
		}
		if crossLength.Kind == sema.LengthStar {
			cross = crossAvailable
		}
		cross = min(cross, crossAvailable)

		if vertical {
			e.arrange(child, Rect{X: 0, Y: offset, W: cross, H: main})
		} else {
			e.arrange(child, Rect{X: offset, Y: 0, W: main, H: cross})
		}

		offset += main
		if i < len(box.Children)-1 {
			offset += gap
		}
	}
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
