package layout

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml/sema"
	"github.com/wow-look-at-my/tml/style"
	"github.com/wow-look-at-my/tml/widget"
)

// stub is a widget that reports a fixed size, so a placement test is about the placement rather than about measuring.
type stub struct{ w, h int }

func (s stub) Measure(_, _ int) (int, int) { return s.w, s.h }

func (s stub) Render(_, _ int) string { return "" }

// wrapper is a composer that keeps a fixed inset, for testing that children are laid out inside what a widget leaves
type wrapper struct {
	stub
	insetW, insetH   int
	free, freeW      bool
	offsetX, offsetY int
}

func (w wrapper) Inset() (int, int) { return w.insetW, w.insetH }

func (w wrapper) Compose(widget.Slots, int, int) string { return "" }

func (w wrapper) Arrange() widget.Layout {
	return widget.Layout{FreeH: w.free, FreeW: w.freeW, OffsetX: w.offsetX, OffsetY: w.offsetY}
}

// dialog is a widget with an opinion about where it belongs.
type dialog struct{ wrapper }

func (d dialog) DefaultAnchor() string { return "center" }

func widgetEngine(t *testing.T, bindings map[string]widget.Native) *Engine {
	t.Helper()
	sheet, err := style.NewSheet(nil, nil)
	require.NoError(t, err)
	registry := widget.NewRegistry()
	for name, native := range bindings {
		registry.Bind(name, native)
	}
	return New(sheet, Options{Widgets: registry})
}

func layoutWith(t *testing.T, bindings map[string]widget.Native, node *sema.Node, w, h int) *Box {
	t.Helper()
	box, err := widgetEngine(t, bindings).Layout(node, w, h)
	require.NoError(t, err)
	return box
}

func child(box *Box, i int) *Box { return box.Children[i] }

func TestCanvasPlacesChildrenAtCoordinates(t *testing.T) {
	root := layoutTree(t, el("Canvas", nil,
		el("Box", map[string]string{"width": "4", "height": "2", "Canvas.x": "3", "Canvas.y": "1"}),
	), 20, 10)

	assert.Equal(t, Rect{X: 3, Y: 1, W: 4, H: 2}, child(root, 0).Rect)
}

// An anchor picks which edges the coordinates are measured from, so a child pinned to a corner stays there when the
func TestCanvasAnchorsToEveryCorner(t *testing.T) {
	tests := []struct {
		anchor string
		want   Rect
	}{
		{"topLeft", Rect{X: 1, Y: 1, W: 4, H: 2}},
		{"topRight", Rect{X: 15, Y: 1, W: 4, H: 2}},
		{"bottomLeft", Rect{X: 1, Y: 7, W: 4, H: 2}},
		{"bottomRight", Rect{X: 15, Y: 7, W: 4, H: 2}},
		{"center", Rect{X: 9, Y: 5, W: 4, H: 2}},
	}
	for _, tc := range tests {
		t.Run(tc.anchor, func(t *testing.T) {
			root := layoutTree(t, el("Canvas", nil,
				el("Box", map[string]string{
					"width": "4", "height": "2",
					"Canvas.x": "1", "Canvas.y": "1", "Canvas.anchor": tc.anchor,
				}),
			), 20, 10)
			assert.Equal(t, tc.want, child(root, 0).Rect)
		})
	}
}

// A canvas takes everything on offer. A single that shrank to its content would leave a child pinned to the bottom-right
func TestCanvasFillsItsSpace(t *testing.T) {
	root := layoutTree(t, el("Canvas", nil, el("Box", map[string]string{"width": "2", "height": "1"})), 20, 10)

	assert.Equal(t, Size{W: 20, H: 10}, root.Content)
}

func TestCanvasChildDefaultsToTheOrigin(t *testing.T) {
	root := layoutTree(t, el("Canvas", nil, el("Box", map[string]string{"width": "4", "height": "2"})), 20, 10)

	assert.Equal(t, Rect{X: 0, Y: 0, W: 4, H: 2}, child(root, 0).Rect)
}

// A dialog says where it belongs, so it lands in the middle without the author positioning it. Saying so explicitly
func TestCanvasAsksTheWidgetWhereItBelongs(t *testing.T) {
	bindings := map[string]widget.Native{"Dialog": dialog{wrapper{stub: stub{w: 4, h: 2}}}}

	centred := layoutWith(t, bindings, el("Canvas", nil, el("Dialog", nil)), 20, 10)
	assert.Equal(t, Rect{X: 8, Y: 4, W: 4, H: 2}, child(centred, 0).Rect)

	pinned := layoutWith(t, bindings, el("Canvas", nil,
		el("Dialog", map[string]string{"Canvas.anchor": "topLeft"}),
	), 20, 10)
	assert.Equal(t, Rect{X: 0, Y: 0, W: 4, H: 2}, child(pinned, 0).Rect)
}

func TestCanvasRejectsNonsensePlacement(t *testing.T) {
	tests := []struct {
		name    string
		attrs   map[string]string
		wantErr string
	}{
		{"unknown anchor", map[string]string{"Canvas.anchor": "middle"}, "expected one of topLeft"},
		{"unknown property", map[string]string{"Canvas.z": "1"}, "takes Canvas.x, Canvas.y and Canvas.anchor"},
		{"non-numeric x", map[string]string{"Canvas.x": "left"}, "Canvas.x"},
		{"another panel's property", map[string]string{"Grid.row": "1"}, "only applies to a child of <Grid>"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := engine(t).Layout(el("Canvas", nil, el("Box", tc.attrs)), 20, 10)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

// A composer's children live inside whatever space it kept for itself.
func TestComposerChildrenGoInsideItsInset(t *testing.T) {
	bindings := map[string]widget.Native{"Frame": wrapper{insetW: 4, insetH: 2}}

	// Inside a stack rather than at the root, which fills the viewport by design and would say nothing about what the
	root := layoutWith(t, bindings, el("Stack", nil, el("Frame", nil, text("hello"))), 20, 10)
	frame := child(root, 0)

	assert.Equal(t, Size{W: 9, H: 3}, frame.Content, "five cells of text plus the frame")
	assert.Equal(t, Rect{X: 0, Y: 0, W: 5, H: 1}, child(frame, 0).Rect)
}

// A scrolling region measures its children against unlimited height -- that is what lets the content be taller than
func TestUnboundedComposerLetsChildrenOverflow(t *testing.T) {
	bindings := map[string]widget.Native{"Scroller": wrapper{insetW: 1, free: true}}
	tall := el("Stack", nil, text("a"), text("b"), text("c"), text("d"), text("e"))

	root := layoutWith(t, bindings, el("Scroller", nil, tall), 10, 3)

	assert.Equal(t, 3, root.Content.H, "the viewport is the space on offer")
	assert.Equal(t, 5, child(root, 0).Rect.H, "the content keeps its full height")
}

// A control scrolled halfway off the top is clicked where it is drawn, so the offset has to reach the geometry rather
func TestScrolledComposerShiftsItsChildren(t *testing.T) {
	bindings := map[string]widget.Native{"Scroller": wrapper{free: true, offsetY: 2}}
	tall := el("Stack", nil, text("a"), text("b"), text("c"), text("d"))

	root := layoutWith(t, bindings, el("Scroller", nil, tall), 10, 2)

	assert.Equal(t, -2, child(root, 0).Rect.Y)
}

// Scrolling stops at the content on both axes. The widget draws the last screenful rather than blank space, so the
func TestAScrolledComposerStopsAtItsContent(t *testing.T) {
	bindings := map[string]widget.Native{
		"Scroller": wrapper{free: true, freeW: true, offsetX: 99, offsetY: 99},
	}
	tall := el("Stack", nil, text("abcdefgh"), text("b"), text("c"), text("d"))

	root := layoutWith(t, bindings, el("Scroller", nil, tall), 3, 2)

	assert.Equal(t, -2, child(root, 0).Rect.Y, "four lines seen through two")
	assert.Equal(t, -5, child(root, 0).Rect.X, "eight cells seen through three")
}
