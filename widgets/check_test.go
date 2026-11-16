package widgets

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml/cell"
	"github.com/wow-look-at-my/tml/widget"
	"github.com/wow-look-at-my/tml/widget/widgettest"
)

func TestCheckboxAndRadioUseTheirOwnGlyphs(t *testing.T) {
	assert.Equal(t, "[ ] Dark mode", text(build(t, "Checkbox", map[string]string{"label": "Dark mode"}), 20, 1))
	assert.Equal(t, "[x] Dark mode",
		text(build(t, "Checkbox", map[string]string{"label": "Dark mode", "checked": "true"}), 20, 1))

	assert.Equal(t, "( ) Small", text(build(t, "Radio", map[string]string{"label": "Small"}), 20, 1))
	assert.Equal(t, "(•) Small",
		text(build(t, "Radio", map[string]string{"label": "Small", "checked": "true"}), 20, 1))
}

func TestCheckWithoutALabelIsJustItsMark(t *testing.T) {
	box := build(t, "Checkbox", nil)

	w, h := box.Measure(0, 0)
	assert.Equal(t, 3, w)
	assert.Equal(t, 1, h)
	assert.Equal(t, "[ ]", widgettest.Text(box, 3, 1))
}

func TestDisabledCheckRefusesFocus(t *testing.T) {
	assert.True(t, build(t, "Checkbox", nil).(widget.Focusable).AcceptsFocus())
	assert.False(t, build(t, "Radio", map[string]string{"disabled": "true"}).(widget.Focusable).AcceptsFocus())
}

func TestCheckShowsItsState(t *testing.T) {
	box := build(t, "Checkbox", map[string]string{"label": "On"})
	assert.True(t, widgettest.Draw(box, 10, 1).At(0, 0).Style.IsZero(), "a resting check is plain")

	for state, attr := range map[widget.State]cell.Attr{
		{Focused: true}: cell.Underline | cell.Bold,
		{Hovered: true}: cell.Underline,
		{Pressed: true}: cell.Reverse,
	} {
		box.(widget.Stateful).SetState(state)
		assert.True(t, widgettest.Draw(box, 10, 1).At(0, 0).Style.Has(attr), "%+v", state)
	}
}

func TestDisabledCheckIsFaint(t *testing.T) {
	box := build(t, "Checkbox", map[string]string{"label": "On", "disabled": "true"})

	assert.True(t, widgettest.Draw(box, 0, 0).At(0, 0).Style.Has(cell.Faint))
}

func TestCheckRejectsANonBooleanChecked(t *testing.T) {
	_, err := tryBuild("Checkbox", map[string]string{"checked": "yes"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected true or false")
}
