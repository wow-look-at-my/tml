// textarea.go holds the text area: a text field of many lines.

package widgets

import (
	"strings"

	"github.com/wow-look-at-my/tml/cell"
	"github.com/wow-look-at-my/tml/input"
	"github.com/wow-look-at-my/tml/widget"
)

var textareaAttrs = []string{"value", "placeholder", "cursor", "disabled", "password"}

// textareaLines is the fewest lines a text area asks for. textareaWidth is its width when nothing constrains it.
const (
	textareaLines = 3
	textareaWidth = 20
)

// textarea is a text field of many lines.
type textarea struct {
	value       []rune
	placeholder string
	cursor      int
	disabled    bool
	password    bool
	state       widget.State
	measure     widget.Measurer
}

func newTextArea(ctx widget.Context) (widget.Native, error) {
	area := &textarea{
		value:       []rune(ctx.Attrs.String("value", "")),
		placeholder: ctx.Attrs.String("placeholder", ""),
		measure:     ctx.Measure,
	}
	var err error
	if area.cursor, err = ctx.Attrs.Int("cursor", -1); err != nil {
		return nil, err
	}
	if area.cursor < 0 || area.cursor > len(area.value) {
		area.cursor = len(area.value)
	}
	if area.disabled, err = ctx.Attrs.Bool("disabled", false); err != nil {
		return nil, err
	}
	if area.password, err = ctx.Attrs.Bool("password", false); err != nil {
		return nil, err
	}
	return area, nil
}

func (a *textarea) AcceptsFocus() bool { return !a.disabled }

func (a *textarea) SetState(state widget.State) { a.state = state }

// lines is the value split at its newlines. An empty value is one empty line.
func (a *textarea) lines() [][]rune {
	out := [][]rune{nil}
	for _, r := range a.value {
		if r == '\n' {
			out = append(out, nil)
			continue
		}
		out[len(out)-1] = append(out[len(out)-1], r)
	}
	return out
}

// runePosition is the line the rune offset at is on, and its column in runes on that line.
func runePosition(lines [][]rune, at int) (line, col int) {
	for line < len(lines)-1 && at > len(lines[line]) {
		at -= len(lines[line]) + 1
		line++
	}
	return line, min(at, len(lines[line]))
}

// runeOffset is the rune offset of a line and a column on it.
func runeOffset(lines [][]rune, line, col int) int {
	at := col
	for _, l := range lines[:line] {
		at += len(l) + 1
	}
	return at
}

// Measure fills the width on offer. The height is the line count, with a floor of textareaLines.
func (a *textarea) Measure(maxW, maxH int) (int, int) {
	w := maxW
	if w <= 0 {
		w = textareaWidth
	}
	h := max(len(a.lines()), textareaLines)
	if maxH > 0 {
		h = min(h, maxH)
	}
	return w, h
}

func (a *textarea) Draw(c cell.Canvas) {
	w, h := c.Size()
	style := cell.Style{}.Underline(true)
	if a.disabled {
		style = style.Faint(true).Underline(false)
	}
	empty := len(a.value) == 0
	shown := a.shown()
	if empty {
		shown = strings.Split(a.placeholder, "\n")
		if !a.disabled {
			style = style.Faint(true)
		}
	}

	// The window keeps the caret in view.
	line, col := runePosition(a.lines(), a.cursor)
	caretX := 0
	if !empty {
		caretX = a.measure.Width(string([]rune(shown[line])[:col]))
	}
	top := max(0, line-h+1)
	left := max(0, caretX+1-w)
	for row := range h {
		c.Fill(0, row, w, 1, style)
		if i := top + row; i < len(shown) {
			c.Line(0, row, a.measure.Lines(shown[i])[0].Cut(left, left+w), style)
		}
	}
	if !a.state.Focused || a.disabled {
		return
	}
	c.Restyle(caretX-left, line-top, 1, 1, func(drawn cell.Cell) cell.Style { return drawn.Style.Reverse(true) })
}

// shown is each line as the area draws it. A password draws a dot for each rune and keeps its newlines.
func (a *textarea) shown() []string {
	lines := a.lines()
	out := make([]string, len(lines))
	for i, l := range lines {
		if a.password {
			out[i] = strings.Repeat("•", len(l))
		} else {
			out[i] = string(l)
		}
	}
	return out
}

// runeEdit is a new value and caret, as the Edit a host stores.
func runeEdit(value []rune, at int) widget.Edit {
	return widget.Edit{Value: string(value), Cursor: at}
}

// Key edits the value. Tab is not taken, so the focus ring moves on it. Up on the first line and down on the last are
// not taken either.
func (a *textarea) Key(key input.Key) (widget.Edit, bool) {
	if a.disabled {
		return widget.Edit{}, false
	}
	lines := a.lines()
	line, col := runePosition(lines, a.cursor)
	at := a.cursor
	insert := func(text []rune) widget.Edit {
		joined := append(append(append([]rune{}, a.value[:at]...), text...), a.value[at:]...)
		return runeEdit(joined, at+len(text))
	}
	without := func(from int) widget.Edit {
		return runeEdit(append(append([]rune{}, a.value[:from]...), a.value[from+1:]...), from)
	}

	switch key.Keystroke() {
	case "tab", "shift+tab":
		return widget.Edit{}, false
	case "left", "ctrl+b":
		return runeEdit(a.value, max(0, at-1)), true
	case "right", "ctrl+f":
		return runeEdit(a.value, min(len(a.value), at+1)), true
	case "up":
		if line == 0 {
			return widget.Edit{}, false
		}
		return runeEdit(a.value, runeOffset(lines, line-1, min(col, len(lines[line-1])))), true
	case "down":
		if line == len(lines)-1 {
			return widget.Edit{}, false
		}
		return runeEdit(a.value, runeOffset(lines, line+1, min(col, len(lines[line+1])))), true
	case "home", "ctrl+a":
		return runeEdit(a.value, runeOffset(lines, line, 0)), true
	case "end", "ctrl+e":
		return runeEdit(a.value, runeOffset(lines, line, len(lines[line]))), true
	case "enter":
		return insert([]rune{'\n'}), true
	case "backspace":
		if at == 0 {
			return runeEdit(a.value, 0), true
		}
		return without(at - 1), true
	case "delete", "ctrl+d":
		if at == len(a.value) {
			return runeEdit(a.value, at), true
		}
		return without(at), true
	}

	if key.Text == "" || key.Mod&(input.Ctrl|input.Alt|input.Meta) != 0 {
		return widget.Edit{}, false
	}
	text := strings.ReplaceAll(strings.ReplaceAll(key.Text, "\r\n", "\n"), "\r", "\n")
	return insert([]rune(text)), true
}
