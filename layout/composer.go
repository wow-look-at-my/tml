package layout

import (
	"github.com/wow-look-at-my/tml/sema"
	"github.com/wow-look-at-my/tml/widget"
)

// composer is the widget behind this box if it wraps children, and nil
// otherwise. A composer with nothing inside it is left to measure and draw
// itself, so an empty <Border/> is still a frame.
func (b *Box) composer() widget.Composer {
	if b.Native == nil {
		return nil
	}
	composer, ok := b.Native.(widget.Composer)
	if !ok {
		return nil
	}
	return composer
}

// Composer exposes the wrapping widget to the renderer, which hands it the
// children it drew.
func (b *Box) Composer() widget.Composer { return b.composer() }

// Slot is which of the parent widget's slots this box was written into.
func (b *Box) Slot() string { return b.slot }

// measureComposer sizes a wrapping widget: its children measured against what is
// left after its own inset, stacked down the page, plus the inset back on.
//
// Stacking is the only sensible default. A widget that wants its children
// arranged some other way is describing a layout panel, and those solve
// constraints rather than draw.
func (e *Engine) measureComposer(box *Box, inner Constraints) Size {
	insetW, insetH := box.composer().Inset()
	freeW, freeH := unbounded(box)

	child := Constraints{MaxW: max(0, inner.MaxW-insetW), MaxH: max(0, inner.MaxH-insetH)}
	if freeW {
		child.MaxW = 0
	}
	if freeH {
		child.MaxH = 0
	}

	var content Size
	for _, kid := range box.Children {
		size := e.measure(kid, child)
		content.W = max(content.W, size.W)
		content.H += size.H
	}
	box.scrolled = content

	// An unbounded axis measured the content at its full extent, which is not
	// what the widget occupies: it takes the space on offer and shows part of it.
	if freeW && inner.MaxW > 0 {
		content.W = max(0, inner.MaxW-insetW)
	}
	if freeH && inner.MaxH > 0 {
		content.H = min(content.H, max(0, inner.MaxH-insetH))
	}
	return Size{W: content.W + insetW, H: content.H + insetH}
}

func (e *Engine) arrangeComposer(box *Box, composer widget.Composer) {
	insetW, insetH := composer.Inset()
	width := max(0, box.Content.W-insetW)
	height := max(0, box.Content.H-insetH)

	offsetX, offsetY := childOffset(box)
	y := -offsetY
	for _, child := range box.Children {
		w, h := child.desired.W, child.desired.H
		if child.width.Kind == sema.LengthStar {
			w = width
		}
		if child.height.Kind == sema.LengthStar {
			h = height
		}
		e.arrange(child, Rect{X: -offsetX, Y: y, W: min(w, width), H: h})
		y += h
	}
}

// childOffset is how far a widget has shifted its children inside itself.
func childOffset(box *Box) (x, y int) {
	scrolled, ok := box.Native.(widget.Scrolled)
	if !ok {
		return 0, 0
	}
	return scrolled.ChildOffset()
}

// unbounded asks a widget whether it measures its children against unlimited
// space, which is what a scrolling region does.
func unbounded(box *Box) (horizontal, vertical bool) {
	free, ok := box.Native.(widget.Unbounded)
	if !ok {
		return false, false
	}
	return free.Unbounded()
}
