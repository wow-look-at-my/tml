package layout

import (
	"fmt"

	"github.com/wow-look-at-my/tml/syntax"
	"github.com/wow-look-at-my/tml/widget"
)

// Target is one focusable element in a rendered frame: what to call it, what it
// does, and where it ended up on screen.
type Target struct {
	ID     string
	Action string
	Rect   Rect
}

// Interaction is the focus and pointer state a frame renders against.
//
// Layout asks it how each focusable element should draw before measuring, then
// hands back the frame's geometry so the next key press or click has something
// to resolve against. Keeping it an interface is what stops the layout engine
// from depending on Bubble Tea.
type Interaction interface {
	// State reports how the focusable at index in the ring should render.
	State(index int, id, action string) widget.State
	// Frame publishes the ring for the frame just laid out, in document order.
	Frame(targets []Target)
}

// pass is the mutable state of one layout call. Keeping it off the engine means
// two goroutines can lay out against the same engine without sharing a ring.
type pass struct {
	e       *Engine
	targets []*Box
	ids     map[string]syntax.Pos
}

// syncState tells every focusable widget how it is being interacted with, before
// anything is measured. A control that grows when focused has to know first, or
// the frame would be sized for the state it was in last time.
func (p *pass) syncState() {
	if p.e.opts.Interaction == nil {
		return
	}
	for i, box := range p.targets {
		box.State = p.e.opts.Interaction.State(i, box.ID, box.Action)
		if stateful, ok := box.Native.(widget.Stateful); ok {
			stateful.SetState(box.State)
		}
	}
}

// publish hands the frame's focus ring back with the geometry it landed on.
func (p *pass) publish() {
	if p.e.opts.Interaction == nil {
		return
	}
	targets := make([]Target, 0, len(p.targets))
	for _, box := range p.targets {
		targets = append(targets, Target{ID: box.ID, Action: box.Action, Rect: box.Screen})
	}
	p.e.opts.Interaction.Frame(targets)
}

// track adds a widget to the focus ring if it takes focus right now. A disabled
// control says no, which is what keeps tab from stopping somewhere unusable.
//
// An id is how focus survives from one frame to the next, so two controls
// answering to the same one is rejected rather than left to send focus
// somewhere arbitrary.
func (p *pass) track(box *Box) error {
	if box.ID != "" {
		if first, dup := p.ids[box.ID]; dup {
			return &syntax.Error{Pos: box.Pos, Message: fmt.Sprintf(
				"id %q is already used at %s; an id has to name one element", box.ID, first)}
		}
		if p.ids == nil {
			p.ids = map[string]syntax.Pos{}
		}
		p.ids[box.ID] = box.Pos
	}
	focusable, ok := box.Native.(widget.Focusable)
	if !ok || !focusable.AcceptsFocus() {
		return nil
	}
	p.targets = append(p.targets, box)
	return nil
}

// setScreen converts the tree's parent-relative rects into viewport coordinates.
//
// A child's rect is relative to its parent's content origin, so the walk carries
// that origin down. Margin is dropped on the way out: the screen rect is the
// cells the box paints, which is what a click has to land in.
func setScreen(box *Box, originX, originY int) {
	margin := box.Style.Margin
	box.Screen = Rect{
		X: originX + box.Rect.X + margin.Left,
		Y: originY + box.Rect.Y + margin.Top,
		W: max(0, box.Rect.W-margin.Horizontal()),
		H: max(0, box.Rect.H-margin.Vertical()),
	}
	offsetX, offsetY := box.Style.ContentOffset()
	for _, child := range box.Children {
		setScreen(child, originX+box.Rect.X+offsetX, originY+box.Rect.Y+offsetY)
	}
}
