package tml_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml"
	"github.com/wow-look-at-my/tml/layout"
)

// boxByID is the laid-out element carrying id, anywhere under root.
func boxByID(root *layout.Box, id string) *layout.Box {
	if root.ID == id {
		return root
	}
	for _, child := range root.Children {
		if found := boxByID(child, id); found != nil {
			return found
		}
	}
	return nil
}

func names(boxes []*layout.Box) []string {
	out := make([]string, 0, len(boxes))
	for _, box := range boxes {
		out = append(out, box.Name)
	}
	return out
}

// A button is a Border element around its content, in the tree: nothing about the frame is private to the button.
func TestButtonIsABorderAroundItsContent(t *testing.T) {
	for _, c := range []struct {
		name, template string
		content        string
	}{
		{"label", `<Button id="go" label="Go"/>`, "Text"},
		{"content slot", `<Button id="go"><Button.Content><Stack><Text>Save</Text><Badge label="2"/></Stack></Button.Content></Button>`, "Stack"},
		{"default slot", `<Button id="go"><Badge label="new"/></Button>`, "Badge"},
	} {
		t.Run(c.name, func(t *testing.T) {
			loaded, err := view(t, app(c.template), tml.Options{})
			require.NoError(t, err)
			root, err := loaded.Layout(nil, 30, 6)
			require.NoError(t, err)

			button := boxByID(root, "go")
			require.NotNil(t, button)
			require.Equal(t, []string{"Border"}, names(button.Children), "the button's only child is its border")
			assert.Equal(t, []string{c.content}, names(button.Children[0].Children), "what was written in the button is inside the border")
			assert.Equal(t, button.Rect.W, button.Children[0].Rect.W, "the border is the button's whole face")
		})
	}
}

// The label and what is written inside are the same thing, so anything written inside wins.
func TestButtonContentReplacesItsLabel(t *testing.T) {
	out := draw(t, app(`<Button label="Save"><Button.Content><Text>[3]</Text></Button.Content></Button>`), 12, 3)

	assert.Contains(t, out, "[3]")
	assert.NotContains(t, out, "Save")
}

func TestButtonIsAsBigAsItsLabelPlusItsBorder(t *testing.T) {
	loaded := interactive(t, `<Stack><Button id="go" label="Save"/></Stack>`, 30, 6)
	button, ok := loaded.UI().Target("go")
	require.True(t, ok)
	assert.Equal(t, 8, button.Rect.W, "four cells of label, a border and a space either side")
	assert.Equal(t, 3, button.Rect.H)

	narrow := interactive(t, `<Stack><Button id="go" label="Save"/></Stack>`, 5, 6)
	button, ok = narrow.UI().Target("go")
	require.True(t, ok)
	assert.LessOrEqual(t, button.Rect.W, 5, "a button never takes more than it was offered")
}

// Content inside a button is published where it is drawn, because its geometry is the border's.
func TestButtonContentIsPublishedWhereItIsDrawn(t *testing.T) {
	loaded := interactive(t, `<Stack><Text>head</Text><Button id="go"><Badge id="count" label="7"/></Button></Stack>`, 30, 8)
	frame, err := loaded.Draw(nil, 30, 8)
	require.NoError(t, err)

	count, ok := loaded.UI().Target("count")
	require.True(t, ok)
	x, y := find(t, frame, "7")
	assert.Equal(t, count.Rect.Y, y, "the badge is drawn on the row it was published on")
	assert.GreaterOrEqual(t, x, count.Rect.X)
	assert.Less(t, x, count.Rect.X+count.Rect.W)

	button, ok := loaded.UI().Target("go")
	require.True(t, ok)
	assert.Contains(t, "╭╔", cellAt(t, frame, button.Rect.X, button.Rect.Y), "the button's rect starts on its border")
}
