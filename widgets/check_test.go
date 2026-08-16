package widgets

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml/widget"
)

func TestCheckboxAndRadioUseTheirOwnGlyphs(t *testing.T) {
	assert.Equal(t, "[ ] Dark mode", build(t, "Checkbox", map[string]string{"label": "Dark mode"}).Render(20, 1))
	assert.Equal(t, "[x] Dark mode",
		build(t, "Checkbox", map[string]string{"label": "Dark mode", "checked": "true"}).Render(20, 1))

	assert.Equal(t, "( ) Small", build(t, "Radio", map[string]string{"label": "Small"}).Render(20, 1))
	assert.Equal(t, "(•) Small",
		build(t, "Radio", map[string]string{"label": "Small", "checked": "true"}).Render(20, 1))
}

func TestCheckWithoutALabelIsJustItsMark(t *testing.T) {
	box := build(t, "Checkbox", nil)

	w, h := box.Measure(0, 0)
	assert.Equal(t, 3, w)
	assert.Equal(t, 1, h)
	assert.Equal(t, "[ ]", box.Render(3, 1))
}

func TestDisabledCheckRefusesFocus(t *testing.T) {
	assert.True(t, build(t, "Checkbox", nil).(widget.Focusable).AcceptsFocus())
	assert.False(t, build(t, "Radio", map[string]string{"disabled": "true"}).(widget.Focusable).AcceptsFocus())
}

func TestCheckShowsItsState(t *testing.T) {
	box := build(t, "Checkbox", map[string]string{"label": "On"})
	resting := box.Render(10, 1)

	for _, state := range []widget.State{{Focused: true}, {Hovered: true}, {Pressed: true}} {
		box.(widget.Stateful).SetState(state)
		assert.NotEqual(t, resting, box.Render(10, 1))
	}
}

func TestCheckRejectsANonBooleanChecked(t *testing.T) {
	_, err := tryBuild("Checkbox", map[string]string{"checked": "yes"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected true or false")
}
