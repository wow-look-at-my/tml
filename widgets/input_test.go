package widgets

import (
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml/widget"
)

func TestTextboxDrawsItsValuePaddedToTheField(t *testing.T) {
	box := build(t, "Textbox", map[string]string{"value": "hi"})

	assert.Equal(t, "hi        ", ansi.Strip(box.Render(10, 1)))
}

// The field is drawn whether or not there is anything in it, because an empty
// box with no rule under it reads as a gap rather than as somewhere to type.
func TestTextboxUnderlinesTheWholeField(t *testing.T) {
	box := build(t, "Textbox", nil)

	assert.Contains(t, box.Render(6, 1), "\x1b[4m", "the field is underlined")
	assert.Equal(t, "      ", ansi.Strip(box.Render(6, 1)))
}

func TestTextboxFallsBackToItsPlaceholder(t *testing.T) {
	box := build(t, "Textbox", map[string]string{"placeholder": "Search"})

	assert.Contains(t, ansi.Strip(box.Render(10, 1)), "Search")
}

func TestTextboxMasksAPassword(t *testing.T) {
	box := build(t, "Textbox", map[string]string{"value": "hunter2", "password": "true"})

	out := ansi.Strip(box.Render(10, 1))
	assert.Contains(t, out, "•••••••")
	assert.NotContains(t, out, "hunter2")
}

// A value longer than the field shows the end, and the cursor drags the window
// with it, so what is being typed stays on screen.
func TestTextboxWindowsAroundTheCursor(t *testing.T) {
	end := build(t, "Textbox", map[string]string{"value": "abcdefghij"})
	assert.Equal(t, "fghij", ansi.Strip(end.Render(5, 1)))

	early := build(t, "Textbox", map[string]string{"value": "abcdefghij", "cursor": "2"})
	assert.Equal(t, "abcde", ansi.Strip(early.Render(5, 1)))
}

// The caret is the only thing that says where typing lands, and it has to point
// into the window rather than into the whole value, or it drifts off the field
// the moment the text is longer than the box.
func TestFocusedTextboxShowsACaretInsideItsWindow(t *testing.T) {
	box := build(t, "Textbox", map[string]string{"value": "abcdefghij", "cursor": "9"})
	box.(widget.Stateful).SetState(widget.State{Focused: true})

	out := box.Render(5, 1)
	assert.Equal(t, "fghij", ansi.Strip(out))
	assert.Contains(t, out, "\x1b[7mj", "the caret sits on the last character, not past it")
}

func TestAnEmptyFocusedTextboxPutsTheCaretFirst(t *testing.T) {
	box := build(t, "Textbox", map[string]string{"placeholder": "search"})
	box.(widget.Stateful).SetState(widget.State{Focused: true})

	assert.Contains(t, box.Render(8, 1), "\x1b[7ms", "the caret is where the first character will go")
}

// A field nobody can type into shows no caret, however the ring got there.
func TestDisabledTextboxShowsNoCaret(t *testing.T) {
	box := build(t, "Textbox", map[string]string{"value": "hi", "disabled": "true"})
	box.(widget.Stateful).SetState(widget.State{Focused: true})

	assert.NotContains(t, box.Render(6, 1), "\x1b[7m")
}

func TestTextboxFillsTheWidthItIsGiven(t *testing.T) {
	box := build(t, "Textbox", nil)

	w, h := box.Measure(30, 1)
	assert.Equal(t, 30, w)
	assert.Equal(t, 1, h)

	w, _ = box.Measure(0, 0)
	assert.Equal(t, 20, w, "a field with nothing to go on is about a search box wide")
}

func TestDisabledTextboxRefusesFocus(t *testing.T) {
	assert.True(t, build(t, "Textbox", nil).(widget.Focusable).AcceptsFocus())
	assert.False(t, build(t, "Textbox", map[string]string{"disabled": "true"}).(widget.Focusable).AcceptsFocus())
}

func TestFocusedTextboxLooksDifferent(t *testing.T) {
	box := build(t, "Textbox", map[string]string{"value": "hi"})
	resting := box.Render(6, 1)

	box.(widget.Stateful).SetState(widget.State{Focused: true})
	assert.NotEqual(t, resting, box.Render(6, 1))
}

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
