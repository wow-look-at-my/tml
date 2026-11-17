// textbox.go holds the textbox code.

package widgets

import (
	"strings"
	"unicode"

	"github.com/wow-look-at-my/tml/cell"
	"github.com/wow-look-at-my/tml/input"
	"github.com/wow-look-at-my/tml/widget"
)

var textboxAttrs = []string{"value", "placeholder", "cursor", "disabled", "password"}

// textbox is a text field. It draws a value; it does not edit the value. Editing is keys, history, selection and a cursor,
type textbox struct {
	value       string
	placeholder string
	cursor      int
	disabled    bool
	password    bool
	state       widget.State
	measure     widget.Measurer
}

func newTextbox(ctx widget.Context) (widget.Native, error) {
	box := &textbox{
		value:       ctx.Attrs.String("value", ""),
		placeholder: ctx.Attrs.String("placeholder", ""),
		measure:     ctx.Measure,
	}
	var err error
	if box.cursor, err = ctx.Attrs.Int("cursor", -1); err != nil {
		return nil, err
	}
	if box.disabled, err = ctx.Attrs.Bool("disabled", false); err != nil {
		return nil, err
	}
	if box.password, err = ctx.Attrs.Bool("password", false); err != nil {
		return nil, err
	}
	return box, nil
}

func (t *textbox) AcceptsFocus() bool { return !t.disabled }

func (t *textbox) SetState(state widget.State) { t.state = state }

func (t *textbox) Measure(maxW, _ int) (int, int) {
	// A field has no natural width: it is as wide as it is allowed to be. A default width applies when nothing constrains
	if maxW <= 0 {
		return 20, 1
	}
	return maxW, 1
}

func (t *textbox) Draw(c cell.Canvas) {
	w, _ := c.Size()
	shown, empty := t.text()

	// The whole field is underlined, not the text in it: an empty field with nothing under it looks like no field.
	style := cell.Style{}.Underline(true)
	if t.disabled {
		style = style.Faint(true).Underline(false)
	} else if empty {
		style = style.Faint(true)
	}

	// The window follows the cursor, so a value longer than the field still shows the part being typed.
	body, start := t.window(shown, w)
	c.Fill(0, 0, w, 1, style)
	c.Line(0, 0, body, style)
	if !t.state.Focused || t.disabled {
		return
	}
	// The caret is one cell of the field reversed.
	c.Restyle(t.column(shown)-start, 0, 1, 1, func(drawn cell.Cell) cell.Style { return drawn.Style.Reverse(true) })
}

// glyphs is the value as the clusters an edit moves over.
func (t *textbox) glyphs() cell.Line { return t.measure.Lines(t.value)[0] }

// index is the caret's position in the value, counted in glyphs.
func (t *textbox) index(glyphs cell.Line) int {
	if t.cursor < 0 {
		return len(glyphs)
	}
	col := 0
	for i, g := range glyphs {
		width := g.Width
		if t.password {
			width = 1
		}
		if col+width > t.cursor {
			return i
		}
		col += width
	}
	return len(glyphs)
}

// edit is a value and a caret position, as the Edit a host stores. The cursor is a column of what the field shows.
func (t *textbox) edit(glyphs cell.Line, index int) widget.Edit {
	col := 0
	for _, g := range glyphs[:index] {
		if t.password {
			col++
		} else {
			col += g.Width
		}
	}
	return widget.Edit{Value: glyphs.String(), Cursor: col}
}

// Key edits the value. The field holds no state: it answers with the value and the caret the host must hand back.
// Enter, tab and the vertical arrows are not the field's, so they reach the focus ring.
func (t *textbox) Key(key input.Key) (widget.Edit, bool) {
	if t.disabled {
		return widget.Edit{}, false
	}
	glyphs := t.glyphs()
	at := t.index(glyphs)
	without := func(from, to int) cell.Line {
		return append(append(cell.Line{}, glyphs[:from]...), glyphs[to:]...)
	}

	switch key.Keystroke() {
	case "left", "ctrl+b":
		return t.edit(glyphs, max(0, at-1)), true
	case "right", "ctrl+f":
		return t.edit(glyphs, min(len(glyphs), at+1)), true
	case "home", "ctrl+a":
		return t.edit(glyphs, 0), true
	case "end", "ctrl+e":
		return t.edit(glyphs, len(glyphs)), true
	case "backspace":
		if at == 0 {
			return t.edit(glyphs, 0), true
		}
		return t.edit(without(at-1, at), at-1), true
	case "delete", "ctrl+d":
		if at == len(glyphs) {
			return t.edit(glyphs, at), true
		}
		return t.edit(without(at, at+1), at), true
	case "ctrl+u":
		return t.edit(without(0, at), 0), true
	case "ctrl+k":
		return t.edit(without(at, len(glyphs)), at), true
	case "ctrl+w", "alt+backspace":
		from := wordStart(glyphs, at)
		return t.edit(without(from, at), from), true
	}

	if key.Text == "" || key.Mod&(input.Ctrl|input.Alt|input.Meta) != 0 {
		return widget.Edit{}, false
	}
	typed := t.measure.Lines(strings.ReplaceAll(key.Text, "\n", " "))[0]
	if len(typed) == 0 {
		return widget.Edit{}, false
	}
	joined := append(append(append(cell.Line{}, glyphs[:at]...), typed...), glyphs[at:]...)
	return t.edit(joined, at+len(typed)), true
}

// wordStart is where the word before the caret begins: past the blanks before the caret, then back over the word.
func wordStart(glyphs cell.Line, at int) int {
	blank := func(i int) bool { return unicode.IsSpace([]rune(glyphs[i].Text)[0]) }
	for at > 0 && blank(at-1) {
		at--
	}
	for at > 0 && !blank(at-1) {
		at--
	}
	return at
}

// column is where the caret sits in the value. An unset cursor means the end, which is where a host that is only
func (t *textbox) column(shown string) int {
	if t.value == "" {
		return 0
	}
	if t.cursor < 0 {
		return t.measure.Width(shown)
	}
	return min(t.cursor, t.measure.Width(shown))
}

// text is what to draw and whether it is the placeholder rather than a value.
func (t *textbox) text() (string, bool) {
	if t.value == "" {
		return t.placeholder, true
	}
	if t.password {
		return strings.Repeat("•", len([]rune(t.value))), false
	}
	return t.value, false
}

// window is the slice of the text the field shows, and the column of the text that slice starts at.
func (t *textbox) window(s string, w int) (cell.Line, int) {
	if w <= 0 {
		return nil, 0
	}
	line := t.measure.Lines(s)[0]
	width := line.Width()
	if width <= w {
		return line, 0
	}
	end := width
	if t.cursor >= 0 {
		end = min(width, max(w, t.cursor+1))
	}
	return line.Cut(end-w, end), end - w
}
