package widgets

import (
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml/cell"
	"github.com/wow-look-at-my/tml/input"
	"github.com/wow-look-at-my/tml/widget"
	"github.com/wow-look-at-my/tml/widget/widgettest"
)

// pem is a key whose line structure must survive a paste.
const pem = "-----BEGIN KEY-----\nMIIBOgIBAAJBAKj34GkxFhD9\n0EJ/9zRP+Gh5fMr==\n-----END KEY-----"

// area builds a text area over value with the caret at cursor.
func area(t *testing.T, value string, cursor int, pairs map[string]string) widget.Native {
	t.Helper()
	attrs := map[string]string{"value": value, "cursor": strconv.Itoa(cursor)}
	for name, raw := range pairs {
		attrs[name] = raw
	}
	return build(t, "TextArea", attrs)
}

// press offers a key to a text area and reports the edit.
func press(t *testing.T, native widget.Native, key input.Key) (widget.Edit, bool) {
	t.Helper()
	return native.(widget.Keyed).Key(key)
}

// caretAt lists the cells of a buffer that carry the caret.
func caretAt(buf *cell.Buffer) [][2]int {
	var cells [][2]int
	for y := range buf.H {
		for x := range buf.W {
			if buf.At(x, y).Style.Has(cell.Reverse) {
				cells = append(cells, [2]int{x, y})
			}
		}
	}
	return cells
}

func TestATextAreaKeepsTheNewlinesOfAPaste(t *testing.T) {
	edit, taken := press(t, area(t, "", -1, nil), input.Key{Text: pem})
	require.True(t, taken)
	assert.Equal(t, pem, edit.Value)
	assert.Equal(t, len([]rune(pem)), edit.Cursor)

	crlf := strings.ReplaceAll(pem, "\n", "\r\n")
	edit, _ = press(t, area(t, "", -1, nil), input.Key{Text: crlf})
	assert.Equal(t, pem, edit.Value, "a carriage return and a line feed are one newline")
	edit, _ = press(t, area(t, "", -1, nil), input.Key{Text: "a\rb"})
	assert.Equal(t, "a\nb", edit.Value, "a lone carriage return is a newline")

	edit, _ = press(t, area(t, "xy", 1, nil), input.Key{Text: "1\n2"})
	assert.Equal(t, "x1\n2y", edit.Value, "the paste lands at the caret")
	assert.Equal(t, 4, edit.Cursor)
}

// Consider the caret moves by a rune across line. Those moves ends, keeps its column on the line above or below,
// and goes to the edges of its line.
func TestTheCaretMovesAcrossLines(t *testing.T) {
	const value = "abcd\nef\nghij"
	cursor := func(at int, stroke string) (int, bool) {
		edit, taken := press(t, area(t, value, at, nil), input.ParseKey(stroke))
		if taken {
			assert.Equal(t, value, edit.Value, "%s moves the caret and keeps the value", stroke)
		}
		return edit.Cursor, taken
	}
	expect := func(at int, stroke string, want int) {
		t.Helper()
		got, taken := cursor(at, stroke)
		require.True(t, taken, "%s at %d", stroke, at)
		assert.Equal(t, want, got, "%s at %d", stroke, at)
	}

	expect(4, "right", 5)
	expect(5, "left", 4)
	expect(0, "left", 0)
	expect(len(value), "right", len(value))

	expect(3, "down", 7)
	expect(7, "down", 10)
	expect(11, "up", 7)
	expect(6, "up", 1)
	expect(9, "up", 6)
	expect(8, "home", 8)
	expect(9, "end", 12)
	expect(6, "home", 5)

	_, taken := cursor(2, "up")
	assert.False(t, taken, "up on the first line goes to the ring")
	_, taken = cursor(10, "down")
	assert.False(t, taken, "down on the last line goes to the ring")
	_, taken = cursor(2, "tab")
	assert.False(t, taken, "tab moves the focus")
}

func TestEnterBackspaceAndDeleteWorkAtLineEdges(t *testing.T) {
	edit, taken := press(t, area(t, "abcd", 2, nil), input.ParseKey("enter"))
	require.True(t, taken)
	assert.Equal(t, widget.Edit{Value: "ab\ncd", Cursor: 3}, edit)

	edit, _ = press(t, area(t, "ab\ncd", 3, nil), input.ParseKey("backspace"))
	assert.Equal(t, widget.Edit{Value: "abcd", Cursor: 2}, edit, "backspace at a line start joins the lines")

	edit, _ = press(t, area(t, "ab\ncd", 2, nil), input.ParseKey("delete"))
	assert.Equal(t, widget.Edit{Value: "abcd", Cursor: 2}, edit, "delete at a line end joins the lines")

	edit, taken = press(t, area(t, "ab", 0, nil), input.ParseKey("backspace"))
	require.True(t, taken)
	assert.Equal(t, widget.Edit{Value: "ab", Cursor: 0}, edit)
	edit, taken = press(t, area(t, "ab", 2, nil), input.ParseKey("delete"))
	require.True(t, taken)
	assert.Equal(t, widget.Edit{Value: "ab", Cursor: 2}, edit)

	edit, _ = press(t, area(t, "ab", 1, nil), input.ParseKey("x"))
	assert.Equal(t, widget.Edit{Value: "axb", Cursor: 2}, edit)
	_, taken = press(t, area(t, "ab", 1, nil), input.ParseKey("ctrl+x"))
	assert.False(t, taken)
}

// A password draws a dot for each rune and keeps the lines. The edit still carries the plain text.
func TestATextAreaMasksAPassword(t *testing.T) {
	secret := area(t, "hunter\nab", -1, map[string]string{"password": "true"})
	out := widgettest.Text(secret, 10, 0)
	assert.Equal(t, []string{"••••••", "••", ""}, trimmed(strings.Split(out, "\n")))
	assert.NotContains(t, out, "hunter")

	edit, _ := press(t, secret, input.ParseKey("c"))
	assert.Equal(t, "hunter\nabc", edit.Value)
}

func TestATextAreaGrowsWithItsLines(t *testing.T) {
	_, h := area(t, "a", -1, nil).Measure(10, 0)
	assert.Equal(t, 3, h, "a floor of a few lines")
	_, h = area(t, "a\nb\nc\nd\ne", -1, nil).Measure(10, 0)
	assert.Equal(t, 5, h)
	_, h = area(t, "a\nb\nc\nd\ne", -1, nil).Measure(10, 2)
	assert.Equal(t, 2, h)
	w, _ := area(t, "a", -1, nil).Measure(0, 0)
	assert.Equal(t, 20, w)
}

// The rows on screen follow the caret down, and the columns follow it right.
func TestTheWindowFollowsTheCaret(t *testing.T) {
	const value = "one\ntwo\nthree\nfour\nfive"
	focused := func(at int, w, h int) *cell.Buffer {
		native := area(t, value, at, nil)
		native.(widget.Stateful).SetState(widget.State{Focused: true})
		return widgettest.Draw(native, w, h)
	}

	top := focused(0, 6, 2)
	assert.Equal(t, []string{"one", "two"}, trimmed(top.Lines()))
	assert.Equal(t, [][2]int{{0, 0}}, caretAt(top))

	bottom := focused(len(value), 6, 2)
	assert.Equal(t, []string{"four", "five"}, trimmed(bottom.Lines()), "the caret is on the last line")
	assert.Equal(t, [][2]int{{4, 1}}, caretAt(bottom))

	wide := focused(strings.Index(value, "three")+5, 3, 2)
	assert.Equal(t, []string{"ee"}, trimmed(wide.Lines())[1:], "the columns scroll to keep the caret at the edge")
	assert.Equal(t, [][2]int{{2, 1}}, caretAt(wide))
}

func TestATextAreaShowsItsPlaceholder(t *testing.T) {
	native := build(t, "TextArea", map[string]string{"placeholder": "paste a key"})
	buf := widgettest.Draw(native, 12, 0)
	assert.Equal(t, "paste a key", strings.TrimRight(buf.Lines()[0], " "))
	assert.True(t, buf.At(0, 0).Style.Has(cell.Faint))
}

func TestADisabledTextAreaTakesNothing(t *testing.T) {
	native := area(t, "ab", 1, map[string]string{"disabled": "true"})
	assert.False(t, native.(widget.Focusable).AcceptsFocus())
	_, taken := press(t, native, input.ParseKey("x"))
	assert.False(t, taken)

	native.(widget.Stateful).SetState(widget.State{Focused: true})
	assert.Empty(t, caretAt(widgettest.Draw(native, 5, 0)), "no caret")
	assert.True(t, area(t, "", -1, nil).(widget.Focusable).AcceptsFocus())
}
