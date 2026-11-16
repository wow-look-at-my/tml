package widgets

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml/cell"
	"github.com/wow-look-at-my/tml/input"
	"github.com/wow-look-at-my/tml/widget"
	"github.com/wow-look-at-my/tml/widget/widgettest"
)

// reversed lists the columns of row zero that carry the caret.
func reversed(buf *cell.Buffer) []int {
	var cols []int
	for x := range buf.W {
		if buf.At(x, 0).Style.Has(cell.Reverse) {
			cols = append(cols, x)
		}
	}
	return cols
}

func TestTextboxDrawsItsValuePaddedToTheField(t *testing.T) {
	box := build(t, "Textbox", map[string]string{"value": "hi"})

	assert.Equal(t, "hi        ", widgettest.Text(box, 10, 1))
}

// The field is drawn whether there is anything in it, because an empty box
// with no rule under it reads as a gap
func TestTextboxUnderlinesTheWholeField(t *testing.T) {
	box := build(t, "Textbox", nil)
	buf := widgettest.Draw(box, 6, 1)

	assert.Equal(t, "      ", buf.Text())
	for x := range buf.W {
		assert.True(t, buf.At(x, 0).Style.Has(cell.Underline), "cell %d is underlined", x)
	}
}

func TestTextboxFallsBackToItsPlaceholder(t *testing.T) {
	box := build(t, "Textbox", map[string]string{"placeholder": "Search"})
	buf := widgettest.Draw(box, 10, 1)

	assert.Contains(t, buf.Text(), "Search")
	assert.True(t, buf.At(0, 0).Style.Has(cell.Faint), "a placeholder is faint")
}

func TestTextboxMasksAPassword(t *testing.T) {
	box := build(t, "Textbox", map[string]string{"value": "hunter2", "password": "true"})

	out := widgettest.Text(box, 10, 1)
	assert.Contains(t, out, "•••••••")
	assert.NotContains(t, out, "hunter2")
}

// A value longer than the field shows the end, and the cursor drags the window with it, so what is being typed stays
func TestTextboxWindowsAroundTheCursor(t *testing.T) {
	end := build(t, "Textbox", map[string]string{"value": "abcdefghij"})
	assert.Equal(t, "fghij", widgettest.Text(end, 5, 1))

	early := build(t, "Textbox", map[string]string{"value": "abcdefghij", "cursor": "2"})
	assert.Equal(t, "abcde", widgettest.Text(early, 5, 1))
}

// The caret is the only thing that says where typing lands.
func TestFocusedTextboxShowsACaretInsideItsWindow(t *testing.T) {
	box := build(t, "Textbox", map[string]string{"value": "abcdefghij", "cursor": "9"})
	box.(widget.Stateful).SetState(widget.State{Focused: true})

	buf := widgettest.Draw(box, 5, 1)
	assert.Equal(t, "fghij", buf.Text())
	assert.Equal(t, []int{4}, reversed(buf), "the caret sits on the last character, not past it")
	assert.Equal(t, "j", buf.At(4, 0).Text)
	assert.True(t, buf.At(4, 0).Style.Has(cell.Underline), "the caret lies on top of the field's underline")
}

func TestAnEmptyFocusedTextboxPutsTheCaretFirst(t *testing.T) {
	box := build(t, "Textbox", map[string]string{"placeholder": "search"})
	box.(widget.Stateful).SetState(widget.State{Focused: true})

	buf := widgettest.Draw(box, 8, 1)
	assert.Equal(t, []int{0}, reversed(buf), "the caret is where the first character will go")
	assert.Equal(t, "s", buf.At(0, 0).Text)
}

// A field nobody can type into shows no caret, however the ring got there.
func TestDisabledTextboxShowsNoCaret(t *testing.T) {
	box := build(t, "Textbox", map[string]string{"value": "hi", "disabled": "true"})
	box.(widget.Stateful).SetState(widget.State{Focused: true})

	buf := widgettest.Draw(box, 6, 1)
	assert.Empty(t, reversed(buf))
	assert.True(t, buf.At(0, 0).Style.Has(cell.Faint))
	assert.False(t, buf.At(0, 0).Style.Has(cell.Underline), "a disabled field has no rule under it")
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
	assert.Empty(t, reversed(widgettest.Draw(box, 6, 1)))

	box.(widget.Stateful).SetState(widget.State{Focused: true})
	assert.Equal(t, []int{2}, reversed(widgettest.Draw(box, 6, 1)), "the caret sits past the value")
}

// key offers one key to a field built from attrs.
func key(t *testing.T, attrs map[string]string, name string) (widget.Edit, bool) {
	t.Helper()
	box := build(t, "Textbox", attrs)
	keyed, ok := box.(widget.Keyed)
	require.True(t, ok, "a text field takes keys itself")
	return keyed.Key(input.ParseKey(name))
}

func TestTextboxTypesAtTheCaret(t *testing.T) {
	edit, ok := key(t, map[string]string{"value": "ac", "cursor": "1"}, "b")
	assert.True(t, ok)
	assert.Equal(t, widget.Edit{Value: "abc", Cursor: 2}, edit)

	edit, ok = key(t, map[string]string{"value": "ab"}, "C")
	assert.True(t, ok)
	assert.Equal(t, widget.Edit{Value: "abC", Cursor: 3}, edit, "an unset cursor is the end")

	edit, ok = key(t, nil, "space")
	assert.True(t, ok)
	assert.Equal(t, widget.Edit{Value: " ", Cursor: 1}, edit)
}

func TestTextboxTypesPastedText(t *testing.T) {
	box := build(t, "Textbox", map[string]string{"value": "a"})

	edit, ok := box.(widget.Keyed).Key(input.Key{Text: "b\nc"})
	assert.True(t, ok)
	assert.Equal(t, widget.Edit{Value: "ab c", Cursor: 4}, edit, "a newline in a one-line field is a blank")
}

func TestTextboxBackspaceAndDelete(t *testing.T) {
	attrs := map[string]string{"value": "abc", "cursor": "1"}

	edit, ok := key(t, attrs, "backspace")
	assert.True(t, ok)
	assert.Equal(t, widget.Edit{Value: "bc", Cursor: 0}, edit)

	edit, ok = key(t, attrs, "delete")
	assert.True(t, ok)
	assert.Equal(t, widget.Edit{Value: "ac", Cursor: 1}, edit)

	edit, ok = key(t, map[string]string{"value": "abc", "cursor": "0"}, "backspace")
	assert.True(t, ok, "backspace at the start is still the field's key")
	assert.Equal(t, widget.Edit{Value: "abc", Cursor: 0}, edit)

	edit, ok = key(t, map[string]string{"value": "abc"}, "delete")
	assert.True(t, ok, "delete at the end is still the field's key")
	assert.Equal(t, widget.Edit{Value: "abc", Cursor: 3}, edit)
}

func TestTextboxMovesItsCursor(t *testing.T) {
	attrs := map[string]string{"value": "abc", "cursor": "1"}
	for name, cursor := range map[string]int{
		"left": 0, "right": 2, "home": 0, "end": 3, "ctrl+a": 0, "ctrl+e": 3, "ctrl+b": 0, "ctrl+f": 2,
	} {
		edit, ok := key(t, attrs, name)
		assert.True(t, ok, name)
		assert.Equal(t, widget.Edit{Value: "abc", Cursor: cursor}, edit, name)
	}

	edit, _ := key(t, map[string]string{"value": "abc", "cursor": "0"}, "left")
	assert.Equal(t, 0, edit.Cursor, "left stops at the start")
	edit, _ = key(t, map[string]string{"value": "abc"}, "right")
	assert.Equal(t, 3, edit.Cursor, "right stops at the end")
}

func TestTextboxCutsWithControlKeys(t *testing.T) {
	attrs := map[string]string{"value": "abc def", "cursor": "4"}

	edit, _ := key(t, attrs, "ctrl+u")
	assert.Equal(t, widget.Edit{Value: "def", Cursor: 0}, edit, "ctrl+u cuts to the start")

	edit, _ = key(t, attrs, "ctrl+k")
	assert.Equal(t, widget.Edit{Value: "abc ", Cursor: 4}, edit, "ctrl+k cuts to the end")

	edit, _ = key(t, map[string]string{"value": "abc def  "}, "ctrl+w")
	assert.Equal(t, widget.Edit{Value: "abc ", Cursor: 4}, edit, "ctrl+w cuts the word before the caret and its blanks")
}

// The cursor is a column of what the field shows, so a wide glyph moves it by its width in cells.
func TestTextboxCountsTheCursorInCells(t *testing.T) {
	edit, _ := key(t, map[string]string{"value": "世a"}, "left")
	assert.Equal(t, widget.Edit{Value: "世a", Cursor: 2}, edit)

	edit, _ = key(t, map[string]string{"value": "世a", "cursor": "2"}, "left")
	assert.Equal(t, widget.Edit{Value: "世a", Cursor: 0}, edit)

	edit, _ = key(t, map[string]string{"value": "世a", "cursor": "2"}, "界")
	assert.Equal(t, widget.Edit{Value: "世界a", Cursor: 4}, edit)

	edit, _ = key(t, map[string]string{"value": "世a", "cursor": "2"}, "backspace")
	assert.Equal(t, widget.Edit{Value: "a", Cursor: 0}, edit)

	edit, _ = key(t, map[string]string{"value": "a世b", "cursor": "1"}, "delete")
	assert.Equal(t, widget.Edit{Value: "ab", Cursor: 1}, edit, "delete takes the whole wide glyph")
}

// A password field shows one dot per glyph, so its cursor counts one column per glyph whatever the glyph's width.
func TestPasswordTextboxCountsOneColumnPerGlyph(t *testing.T) {
	edit, ok := key(t, map[string]string{"value": "ab", "password": "true"}, "世")
	assert.True(t, ok)
	assert.Equal(t, widget.Edit{Value: "ab世", Cursor: 3}, edit)

	edit, _ = key(t, map[string]string{"value": "世世", "password": "true", "cursor": "1"}, "backspace")
	assert.Equal(t, widget.Edit{Value: "世", Cursor: 0}, edit)
}

func TestDisabledTextboxRefusesKeys(t *testing.T) {
	_, ok := key(t, map[string]string{"value": "a", "disabled": "true"}, "b")
	assert.False(t, ok)
	_, ok = key(t, map[string]string{"value": "a", "disabled": "true"}, "backspace")
	assert.False(t, ok)
}

// Enter, tab and the vertical arrows belong to the focus ring, and a control key the field has no use for is not its.
func TestTextboxLeavesTheFocusRingItsKeys(t *testing.T) {
	for _, name := range []string{"enter", "tab", "shift+tab", "up", "down", "ctrl+x", "esc"} {
		_, ok := key(t, map[string]string{"value": "a"}, name)
		assert.False(t, ok, name)
	}
}
