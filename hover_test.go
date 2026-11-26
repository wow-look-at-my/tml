package tml_test

import _ "embed"

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml"
	"github.com/wow-look-at-my/tml/cell"
	"github.com/wow-look-at-my/tml/input"
	"github.com/wow-look-at-my/tml/sema"
)

//go:embed testdata/hover-view.xml
var hoverView string

// hoverFrame paints the view with the pointer at a cell, and returns the styled frame.
func hoverFrame(t *testing.T, props tml.Props, x, y int) string {
	t.Helper()
	body := `<Component xmlns="urn:tml:v1" name="App"><Property name="dimmed" type="bool" default="false"/><Template>` +
		hoverView + `</Template></Component>`
	loaded, err := view(t, body, tml.Options{})
	require.NoError(t, err)
	_, err = loaded.Render(props, 20, 3)
	require.NoError(t, err)
	loaded.UI().Update(input.Mouse{X: x, Y: y, Action: input.Motion})
	out, err := loaded.Render(props, 20, 3)
	require.NoError(t, err)
	return out
}

// The detail is darkened until the pointer is on its row, anywhere on the row: the scrim has no id. It takes the
// hover of the area around it.
func TestAScrimInsideAnAreaLiftsWhenTheAreaIsHovered(t *testing.T) {
	away := hoverFrame(t, nil, 0, 2)
	on := hoverFrame(t, nil, 0, 0)

	assert.Equal(t, cell.Strip(away), cell.Strip(on), "a scrim changes colours and moves nothing")
	assert.NotEqual(t, away, on)
	assert.Contains(t, on, "name detail", "hovered, the row is drawn with no colour change between its parts")
	assert.NotContains(t, away, "name detail", "away from the row, the detail takes a colour of its own")
}

// A scrim that is off is the frame without it.
func TestAScrimTurnsOnFromAProperty(t *testing.T) {
	off := hoverFrame(t, tml.Props{"dimmed": sema.BoolValue(false)}, 0, 0)
	on := hoverFrame(t, tml.Props{"dimmed": sema.BoolValue(true)}, 0, 0)

	assert.Equal(t, cell.Strip(off), cell.Strip(on))
	assert.Contains(t, off, "below")
	assert.NotEqual(t, off, on, "on, the text under it is darkened")
}
