package layout

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml/style"
	"github.com/wow-look-at-my/tml/widget"
)

// hovering reports the element with the given id as the one under the pointer.
type hovering struct{ id string }

func (h hovering) States(targets []Target) []widget.State {
	states := make([]widget.State, len(targets))
	for i, target := range targets {
		states[i].Hovered = target.ID == h.id
	}
	return states
}

func (hovering) Frame([]Target) {}

// told is a widget that keeps the state layout gave it.
type told struct {
	wrapper
	got *widget.State
}

func (w told) SetState(state widget.State) { *w.got = state }

// hoverTree is a region around a part, and a second region around a part that has its own id.
func hoverTree(t *testing.T, hovered string) (part, named, outside widget.State) {
	t.Helper()
	sheet, err := style.NewSheet(nil, nil)
	require.NoError(t, err)
	registry := widget.NewRegistry().
		Bind("Region", wrapper{}).
		Bind("Part", told{got: &part}).
		Bind("Named", told{got: &named}).
		Bind("Outside", told{got: &outside})

	_, err = New(sheet, Options{Widgets: registry, Interaction: hovering{id: hovered}}).Layout(el("Stack", nil,
		el("Region", map[string]string{"id": "region"},
			el("Stack", nil, el("Part", nil)),
			el("Named", map[string]string{"id": "named"}),
		),
		el("Outside", nil),
	), 10, 4)
	require.NoError(t, err)
	return part, named, outside
}

// A widget the pointer cannot reach takes the hover of the element around it, so a part of a hovered region can draw
// itself differently.
func TestAPartOfAHoveredElementIsHovered(t *testing.T) {
	part, named, outside := hoverTree(t, "region")

	assert.True(t, part.Hovered, "the part has no id, so the region's hover is its own")
	assert.False(t, named.Hovered, "an element with an id answers the pointer itself")
	assert.False(t, outside.Hovered, "nothing around this widget is hovered")
}

func TestAPartOfAnElementThatIsNotHoveredIsNot(t *testing.T) {
	part, named, _ := hoverTree(t, "named")

	assert.False(t, part.Hovered)
	assert.True(t, named.Hovered)
}
