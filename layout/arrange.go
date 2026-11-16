// arrange.go holds the arrange code.

package layout

import (
	"github.com/wow-look-at-my/tml/sema"
)

// arrange assigns a final rect to a box and places its children inside it.
func (e *Engine) arrange(box *Box, rect Rect) {
	// A collapsed box was measured to nothing, so its parent handed it no space. Nothing is placed inside it either.
	if box.Visibility == Collapsed {
		box.Rect, box.Content = Rect{}, Size{}
		return
	}
	box.Rect = rect
	// A box measured against one width and given another is measured again, so
	// what is inside it wraps where it is drawn.
	if stale(box, rect) {
		e.measure(box, Constraints{MaxW: rect.W, MaxH: box.offered.MaxH})
	}

	outerW, outerH := box.outer()
	box.Content = Size{W: max(0, rect.W-outerW), H: max(0, rect.H-outerH)}

	if composer := box.composer(); composer != nil && len(box.Children) > 0 {
		e.arrangeComposer(box, composer)
		return
	}
	if box.Native != nil {
		return
	}
	switch box.Name {
	case "Text", "Spacer", "#text":
		return
	case "Stack":
		e.arrangeStack(box)
	case "Grid":
		e.arrangeGrid(box)
	case "Canvas":
		e.arrangeCanvas(box)
	default:
		// A decorator gives each child the whole content box, clamped to what the child asked for unless it is star-sized
		for _, child := range box.Children {
			w, h := child.desired.W, child.desired.H
			if child.width.Kind == sema.LengthStar {
				w = box.Content.W
			}
			if child.height.Kind == sema.LengthStar {
				h = box.Content.H
			}
			w, h = min(w, box.Content.W), min(h, box.Content.H)
			e.arrange(child, Rect{
				X: box.Style.Align.Offset(w, box.Content.W),
				Y: box.Style.VAlign.Offset(h, box.Content.H),
				W: w, H: h,
			})
		}
	}
}

// stale reports whether a box's measurements belong to a width it did not
// get. A star-sized box is measured before its share is known.
func stale(box *Box, rect Rect) bool {
	if rect.W <= 0 || rect.W == box.offered.MaxW {
		return false
	}
	if box.width.Kind == sema.LengthStar {
		return true
	}
	return rect.W < box.desired.W
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
	// Only placed children get a gap: a collapsed element leaves no blank cell where the gap beside it would be.
	placed := 0
	for _, child := range box.Children {
		if stackPlaces(child, vertical) {
			placed++
		}
	}
	available = max(0, available-gap*max(0, placed-1))

	// Fixed and auto children keep what they asked for; the remainder is shared out by star weight.
	used, weight := 0, 0
	for _, child := range box.Children {
		if !stackPlaces(child, vertical) {
			continue
		}
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

	// Space no star child takes is shared out by the stack's own alignment along its axis.
	offset := 0
	if weight == 0 {
		along := box.Style.VAlign
		if !vertical {
			along = box.Style.Align
		}
		offset = along.Offset(used, available)
	}
	remaining := leftover
	remainingWeight := weight
	first := true

	for _, child := range box.Children {
		if !stackPlaces(child, vertical) {
			continue
		}
		if !first {
			offset += gap
		}
		first = false
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
			// The last star child takes the rounding remainder so the row or column always fills exactly.
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

		align := box.Style.Align
		if !vertical {
			align = box.Style.VAlign
		}
		crossOffset := align.Offset(cross, crossAvailable)

		if vertical {
			e.arrange(child, Rect{X: crossOffset, Y: offset, W: cross, H: main})
		} else {
			e.arrange(child, Rect{X: offset, Y: crossOffset, W: main, H: cross})
		}

		offset += main
	}
}
