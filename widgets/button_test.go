package widgets

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml/sema"
	"github.com/wow-look-at-my/tml/widget"
)

// face binds a button to a real Border, the way the engine does, puts the button in state, and draws the border around body.
func face(t *testing.T, attrs map[string]string, state widget.State, body string, w int) string {
	t.Helper()
	button, ok := build(t, "Button", attrs).(widget.Templated)
	require.True(t, ok, "a button is templated")
	element, chrome, _ := button.Template()
	border := build(t, element, chrome)
	require.NoError(t, button.Bind(border))
	button.(widget.Stateful).SetState(state)
	return border.(widget.Composer).Compose(content(body), w, 3)
}

func TestButtonIsDrawnAsABorder(t *testing.T) {
	button := build(t, "Button", map[string]string{"label": "Save"}).(widget.Templated)

	element, attrs, fallback := button.Template()
	assert.Equal(t, "Border", element)
	assert.Equal(t, map[string]string{"kind": "rounded", "pad": "1", "contentAlign": "center"}, attrs)
	assert.Equal(t, "Save", fallback, "the label is the content when nothing was written inside")
	assert.Equal(t, sema.Thickness{}, button.Insets(), "the border keeps the insets, not the button")
}

func TestButtonLabelIsSugarForItsContent(t *testing.T) {
	assert.Equal(t, "╭──────╮\n│ Save │\n╰──────╯", face(t, map[string]string{"label": "Save"}, widget.State{}, "Save", 8))
}

func TestButtonCentresItsContent(t *testing.T) {
	assert.Equal(t, "╭──────────╮\n│    Go    │\n╰──────────╯", face(t, nil, widget.State{}, "Go", 12))
}

func TestButtonBindsOnlyTheLibraryBorder(t *testing.T) {
	button := build(t, "Button", nil).(widget.Templated)

	err := button.Bind(build(t, "Badge", map[string]string{"label": "x"}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "a button is drawn as the library's Border")
}

func TestButtonDeclaresItsSlot(t *testing.T) {
	factory, ok := Library().Factory("Button")
	require.True(t, ok)
	slotted, ok := factory.(widget.Slotted)
	require.True(t, ok)
	assert.Equal(t, []string{"Content"}, slotted.Slots())
}

// The focused control has to be obvious without relying on colour alone, so the border thickens rather than only
func TestFocusedButtonChangesItsBorder(t *testing.T) {
	resting := face(t, nil, widget.State{}, "Go", 6)
	focused := face(t, nil, widget.State{Focused: true}, "Go", 6)

	assert.Contains(t, resting, "╭")
	assert.Contains(t, focused, "╔")
}

func TestPressedAndHoveredButtonsLookDifferent(t *testing.T) {
	base := face(t, nil, widget.State{}, "Go", 6)
	pressed := face(t, nil, widget.State{Pressed: true}, "Go", 6)
	hovered := face(t, nil, widget.State{Hovered: true}, "Go", 6)

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

// A disabled control still looks disabled when focus happens to be reported on it, which is what a stale frame would
func TestDisabledButtonIgnoresFocusStyling(t *testing.T) {
	disabled := face(t, map[string]string{"disabled": "true"}, widget.State{Focused: true}, "Go", 6)

	assert.Contains(t, disabled, "╭", "the border stays the resting one")
}

func TestButtonVariants(t *testing.T) {
	plain := face(t, nil, widget.State{}, "Go", 6)
	danger := face(t, map[string]string{"variant": "danger"}, widget.State{}, "Go", 6)
	assert.NotEqual(t, plain, danger)

	_, err := tryBuild("Button", map[string]string{"variant": "loud"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected one of danger, default, primary")
}
