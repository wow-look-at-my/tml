package widgets

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml/widget"
)

func stateful(t *testing.T, native widget.Native, state widget.State) widget.Composer {
	t.Helper()
	set, ok := native.(widget.Stateful)
	require.True(t, ok)
	set.SetState(state)
	wrapper, ok := native.(widget.Composer)
	require.True(t, ok)
	return wrapper
}

func TestButtonLabelIsSugarForItsContent(t *testing.T) {
	button := composer(t, "Button", map[string]string{"label": "Save"})

	assert.Equal(t, "╭──────╮\n│ Save │\n╰──────╯", button.Compose(nil, 8, 3))
}

// The label and the slot are the same thing seen twice, so anything written
// inside wins: that is what makes rich content possible at all.
func TestButtonContentSlotBeatsTheLabel(t *testing.T) {
	button := composer(t, "Button", map[string]string{"label": "Save"})

	out := button.Compose(widget.Slots{"Content": {"[3]"}}, 8, 3)
	assert.Contains(t, out, "[3]")
	assert.NotContains(t, out, "Save")
}

func TestButtonTakesDefaultSlotContentToo(t *testing.T) {
	button := composer(t, "Button", nil)

	assert.Contains(t, button.Compose(content("Quit"), 8, 3), "Quit")
}

func TestButtonDeclaresItsSlot(t *testing.T) {
	factory, ok := Library().Factory("Button")
	require.True(t, ok)
	slotted, ok := factory.(widget.Slotted)
	require.True(t, ok)
	assert.Equal(t, []string{"Content"}, slotted.Slots())
}

// The focused control has to be obvious without relying on colour alone, so the
// border thickens rather than only changing hue.
func TestFocusedButtonChangesItsBorder(t *testing.T) {
	resting := composer(t, "Button", map[string]string{"label": "Go"}).Compose(nil, 6, 3)
	focused := stateful(t, build(t, "Button", map[string]string{"label": "Go"}),
		widget.State{Focused: true}).Compose(nil, 6, 3)

	assert.Contains(t, resting, "╭")
	assert.Contains(t, focused, "╔")
}

func TestPressedAndHoveredButtonsLookDifferent(t *testing.T) {
	base := composer(t, "Button", map[string]string{"label": "Go"}).Compose(nil, 6, 3)

	pressed := stateful(t, build(t, "Button", map[string]string{"label": "Go"}),
		widget.State{Pressed: true}).Compose(nil, 6, 3)
	hovered := stateful(t, build(t, "Button", map[string]string{"label": "Go"}),
		widget.State{Hovered: true}).Compose(nil, 6, 3)

	assert.NotEqual(t, base, pressed)
	assert.NotEqual(t, base, hovered)
	assert.NotEqual(t, pressed, hovered)
}

// Tab must not stop on a control that would do nothing.
func TestDisabledButtonRefusesFocus(t *testing.T) {
	enabled := build(t, "Button", map[string]string{"label": "Go"})
	disabled := build(t, "Button", map[string]string{"label": "Go", "disabled": "true"})

	assert.True(t, enabled.(widget.Focusable).AcceptsFocus())
	assert.False(t, disabled.(widget.Focusable).AcceptsFocus())
}

// A disabled control still looks disabled when focus happens to be reported on
// it, which is what a stale frame would do.
func TestDisabledButtonIgnoresFocusStyling(t *testing.T) {
	disabled := stateful(t, build(t, "Button", map[string]string{"label": "Go", "disabled": "true"}),
		widget.State{Focused: true}).Compose(nil, 6, 3)

	assert.Contains(t, disabled, "╭", "the border stays the resting one")
}

func TestButtonVariants(t *testing.T) {
	plain := composer(t, "Button", map[string]string{"label": "Go"}).Compose(nil, 6, 3)
	danger := composer(t, "Button", map[string]string{"label": "Go", "variant": "danger"}).Compose(nil, 6, 3)
	assert.NotEqual(t, plain, danger)

	_, err := tryBuild("Button", map[string]string{"variant": "loud"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected one of danger, default, primary")
}

func TestButtonMeasuresItsLabelPlusItsFrame(t *testing.T) {
	button := build(t, "Button", map[string]string{"label": "Save"})

	w, h := button.Measure(0, 0)
	assert.Equal(t, 8, w, "four cells of label plus a border and a space either side")
	assert.Equal(t, 3, h)

	w, _ = button.Measure(5, 0)
	assert.Equal(t, 5, w, "a button never asks for more than it was offered")
}

func TestButtonRendersWithoutChildren(t *testing.T) {
	button := build(t, "Button", map[string]string{"label": "Ok"})

	assert.Equal(t, 3, len(strings.Split(button.Render(6, 3), "\n")))
}
