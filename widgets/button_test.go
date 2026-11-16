package widgets

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml/cell"
	"github.com/wow-look-at-my/tml/sema"
	"github.com/wow-look-at-my/tml/widget"
	"github.com/wow-look-at-my/tml/widget/widgettest"
)

// face binds a button to a real Border, the way the engine does, puts the button in state, and draws the border around body.
func face(t *testing.T, attrs map[string]string, state widget.State, body string, w int) *cell.Buffer {
	t.Helper()
	button, ok := build(t, "Button", attrs).(widget.Templated)
	require.True(t, ok, "a button is templated")
	element, chrome, _ := button.Template()
	border := build(t, element, chrome)
	require.NoError(t, button.Bind(border))
	button.(widget.Stateful).SetState(state)
	assert.Equal(t, " ", widgettest.Text(button, 1, 1), "the button paints nothing of its own")
	return widgettest.Compose(border.(widget.Composer), []string{body}, w, 3)
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
	assert.Equal(t, "╭──────╮\n│ Save │\n╰──────╯", face(t, map[string]string{"label": "Save"}, widget.State{}, "Save", 8).Text())
}

func TestButtonCentresItsContent(t *testing.T) {
	assert.Equal(t, "╭──────────╮\n│    Go    │\n╰──────────╯", face(t, nil, widget.State{}, "Go", 12).Text())
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

	assert.Equal(t, "╭", resting.At(0, 0).Text)
	assert.Equal(t, "╔", focused.At(0, 0).Text)
	assert.Equal(t, cell.Indexed(13), focused.At(0, 0).Style.Fg, "a focus colour backs up the border")
	assert.True(t, focused.At(2, 1).Style.Has(cell.Bold), "the content is bold while focused")
	assert.False(t, resting.At(2, 1).Style.Has(cell.Bold))
}

func TestPressedAndHoveredButtonsLookDifferent(t *testing.T) {
	base := face(t, nil, widget.State{}, "Go", 6)
	pressed := face(t, nil, widget.State{Pressed: true}, "Go", 6)
	hovered := face(t, nil, widget.State{Hovered: true}, "Go", 6)

	assert.True(t, base.At(2, 1).Style.IsZero(), "a resting button's content is plain")

	// A pressed button inverts its whole inside as one block, the blanks around the content included.
	for x := 1; x < 5; x++ {
		assert.True(t, pressed.At(x, 1).Style.Has(cell.Reverse), "column %d of a pressed button", x)
	}
	assert.False(t, pressed.At(0, 1).Style.Has(cell.Reverse), "the border itself is not inverted")

	assert.True(t, hovered.At(2, 1).Style.Has(cell.Underline), "a hovered button underlines its content")
	assert.False(t, hovered.At(2, 1).Style.Has(cell.Reverse))
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

	assert.Equal(t, "╭", disabled.At(0, 0).Text, "the border stays the resting one")
	assert.Equal(t, cell.Indexed(8), disabled.At(0, 0).Style.Fg)
	assert.True(t, disabled.At(2, 1).Style.Has(cell.Faint), "the content is faint")
	assert.False(t, disabled.At(2, 1).Style.Has(cell.Bold))
}

func TestButtonVariants(t *testing.T) {
	plain := face(t, nil, widget.State{}, "Go", 6)
	danger := face(t, map[string]string{"variant": "danger"}, widget.State{}, "Go", 6)
	assert.False(t, plain.At(0, 0).Style.Fg.IsSet())
	assert.Equal(t, cell.Indexed(9), danger.At(0, 0).Style.Fg, "the variant colours the border")
	assert.Equal(t, cell.Indexed(9), danger.At(2, 1).Style.Fg, "and the label")

	primary := face(t, map[string]string{"variant": "primary"}, widget.State{Focused: true}, "Go", 6)
	assert.Equal(t, cell.Indexed(12), primary.At(0, 0).Style.Fg, "a focused variant keeps its own colour")

	_, err := tryBuild("Button", map[string]string{"variant": "loud"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected one of danger, default, primary")
}
