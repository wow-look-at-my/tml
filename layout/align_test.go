package layout

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml/input"
	"github.com/wow-look-at-my/tml/style"
	"github.com/wow-look-at-my/tml/widget"
)

// A vertical stack with no star child puts its spare height where its valign says.
func TestAStackPlacesItsChildrenByItsValign(t *testing.T) {
	root := layoutTree(t, el("Stack", map[string]string{"valign": "bottom"}, text("a"), text("b")), 10, 10)

	assert.Equal(t, 8, child(root, 0).Rect.Y, "two rows sit at the bottom of ten")
	assert.Equal(t, 9, child(root, 1).Rect.Y)
}

func TestAStackCentresItsChildrenAlongItsAxis(t *testing.T) {
	root := layoutTree(t, el("Stack", map[string]string{"valign": "middle"}, text("a"), text("b")), 10, 6)

	assert.Equal(t, 2, child(root, 0).Rect.Y, "four spare rows split evenly")
}

// A horizontal stack with no star child puts its spare width where its align says.
func TestAHorizontalStackPlacesItsChildrenByItsAlign(t *testing.T) {
	root := layoutTree(t, el("Stack", map[string]string{"orientation": "horizontal", "align": "right"},
		text("ab"), text("cd"),
	), 20, 3)

	assert.Equal(t, 16, child(root, 0).Rect.X, "the pair ends at the right edge")
	assert.Equal(t, 18, child(root, 1).Rect.X)
}

// A star child takes the spare space, so the alignment has nothing left to place.
func TestAStarChildTakesTheSpaceAlignmentWouldUse(t *testing.T) {
	root := layoutTree(t, el("Stack", map[string]string{"valign": "bottom"},
		text("a"), el("Box", map[string]string{"height": "*"}),
	), 10, 10)

	assert.Equal(t, 0, child(root, 0).Rect.Y)
	assert.Equal(t, 9, child(root, 1).Rect.H)
}

// A decorator places its child by align and valign, rather than always at its origin.
func TestADecoratorPlacesItsChildByAlignment(t *testing.T) {
	root := layoutTree(t, el("Box", map[string]string{"align": "center", "valign": "bottom"},
		el("Box", map[string]string{"width": "4", "height": "2"}),
	), 20, 10)

	assert.Equal(t, Rect{X: 8, Y: 8, W: 4, H: 2}, child(root, 0).Rect)
}

func TestADecoratorChildDefaultsToTheOrigin(t *testing.T) {
	root := layoutTree(t, el("Box", nil, el("Box", map[string]string{"width": "4", "height": "2"})), 20, 10)

	assert.Equal(t, Rect{X: 0, Y: 0, W: 4, H: 2}, child(root, 0).Rect)
}

// keyed is a widget that takes keys itself.
type keyed struct{ stub }

func (keyed) Key(input.Key) (widget.Edit, bool) { return widget.Edit{}, false }

// recorder keeps the targets the last frame published.
type recorder struct{ targets []Target }

func (r *recorder) States(targets []Target) []widget.State { return make([]widget.State, len(targets)) }

func (r *recorder) Frame(targets []Target) { r.targets = targets }

func TestATargetCarriesTheKeysOfAKeyedWidget(t *testing.T) {
	sheet, err := style.NewSheet(nil, nil)
	require.NoError(t, err)
	field := keyed{stub{w: 3, h: 1}}
	registry := widget.NewRegistry().Bind("Field", field).Bind("Plain", stub{w: 3, h: 1})
	rec := &recorder{}

	_, err = New(sheet, Options{Widgets: registry, Interaction: rec}).Layout(el("Stack", nil,
		el("Field", map[string]string{"id": "field"}),
		el("Plain", map[string]string{"id": "plain"}),
	), 10, 4)
	require.NoError(t, err)

	require.Len(t, rec.targets, 2)
	assert.Equal(t, "field", rec.targets[0].ID)
	assert.Equal(t, widget.Keyed(field), rec.targets[0].Keys, "the keyed widget hands over its own key handler")
	assert.Nil(t, rec.targets[1].Keys, "a plain widget leaves keys to the focus ring")
}

// A widget with a tooltip answers the pointer with no id, and its target carries the text.
func TestATargetCarriesItsTooltip(t *testing.T) {
	sheet, err := style.NewSheet(nil, nil)
	require.NoError(t, err)
	registry := widget.NewRegistry().Bind("Plain", stub{w: 3, h: 1})
	rec := &recorder{}
	engine := New(sheet, Options{Widgets: registry, Interaction: rec})

	_, err = engine.Layout(el("Stack", nil,
		el("Plain", map[string]string{"tooltip": "Says what this is"}),
		el("Plain", nil),
		el("Plain", map[string]string{"id": "named"}),
	), 10, 4)
	require.NoError(t, err)

	require.Len(t, rec.targets, 2, "the widget with nothing to say to the pointer is not a target")
	assert.Equal(t, "Says what this is", rec.targets[0].Tooltip)
	assert.Empty(t, rec.targets[0].ID)
	assert.Equal(t, Rect{X: 0, Y: 0, W: 3, H: 1}, rec.targets[0].Rect)
	assert.Empty(t, rec.targets[1].Tooltip)

	// The pointer reaches widgets only, so a tooltip on a panel would never show. That is an error, not a blank.
	_, err = engine.Layout(el("Stack", map[string]string{"tooltip": "never seen"}, el("Plain", nil)), 10, 4)
	require.ErrorContains(t, err, "<Stack> has a tooltip, and only a widget answers the pointer")
}

// scrolled is a scrolling composer that keeps what layout told it.
type scrolled struct {
	wrapper
	got *widget.Scroll
}

func (s scrolled) SetScroll(scroll widget.Scroll) { *s.got = scroll }

func TestAScrollingComposerIsToldItsScroll(t *testing.T) {
	var got widget.Scroll
	bindings := map[string]widget.Native{
		"Scroller": scrolled{wrapper: wrapper{free: true, offsetY: 5}, got: &got},
	}
	tall := el("Stack", nil, text("a"), text("b"), text("c"), text("d"))

	layoutWith(t, bindings, el("Scroller", nil, tall), 10, 2)

	assert.Equal(t, widget.Scroll{Y: 2, MaxY: 2}, got, "the offset stops at the last screenful")
}
