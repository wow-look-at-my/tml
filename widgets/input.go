package widgets

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/wow-look-at-my/tml/widget"
)

// ---------------------------------------------------------------- Textbox

var textboxAttrs = []string{"value", "placeholder", "cursor", "disabled", "password"}

// textbox is a text field.
//
// It draws a value; it does not edit one. Editing is keys, history, selection
// and a cursor, all of which are state, and state lives in the host's model --
// which is also where the value it is showing came from. A host that wants a
// real editor binds bubbles' textinput as its own element; this is the
// declarative half, for a field whose value the host already has.
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
	// A field has no natural width: it is as wide as it is allowed to be. Twenty
	// is the fallback when nothing constrains it, which is about the width of a
	// search box.
	if maxW <= 0 {
		return 20, 1
	}
	return maxW, 1
}

func (t *textbox) Render(w, _ int) string {
	shown, empty := t.text()

	// The whole field is underlined, not just the text in it: an empty box with
	// nothing under it is indistinguishable from a gap, and a field nobody can
	// see is a field nobody clicks.
	style := lipgloss.NewStyle().Underline(true)
	if t.disabled {
		style = style.Faint(true).Underline(false)
	} else if empty {
		style = style.Faint(true)
	}

	// The window follows the cursor, so a value longer than the field still shows
	// the part being typed rather than its beginning.
	body, start := t.window(shown, w)
	if gap := w - t.measure.Width(body); gap > 0 {
		body += strings.Repeat(" ", gap)
	}
	if !t.state.Focused || t.disabled {
		return style.Render(body)
	}
	return caret(style, body, t.column(shown)-start, t.measure)
}

// column is where the caret sits in the value. An unset cursor means the end,
// which is where a host that is only appending characters wants it.
func (t *textbox) column(shown string) int {
	if t.value == "" {
		return 0
	}
	if t.cursor < 0 {
		return t.measure.Width(shown)
	}
	return min(t.cursor, t.measure.Width(shown))
}

// caret draws one cell of the field reversed. The host owns the text, so this is
// the only thing left that says where the next character will land.
//
// The caret cell is styled from scratch rather than from the field: lipgloss
// drops Reverse from a whitespace-only render that also carries Underline, and
// the caret sits on a blank cell whenever it is at the end of the value.
// render/lipgloss_contract_test.go pins that.
func caret(style lipgloss.Style, body string, col int, measure widget.Measurer) string {
	width := measure.Width(body)
	if col < 0 || col >= width {
		return style.Render(body)
	}
	return style.Render(ansi.Cut(body, 0, col)) +
		lipgloss.NewStyle().Reverse(true).Render(ansi.Cut(body, col, col+1)) +
		style.Render(ansi.Cut(body, col+1, width))
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

// window is the slice of the text the field shows, and the column of the text
// that slice starts at.
func (t *textbox) window(s string, w int) (string, int) {
	if w <= 0 {
		return "", 0
	}
	width := t.measure.Width(s)
	if width <= w {
		return s, 0
	}
	end := width
	if t.cursor >= 0 {
		end = min(width, max(w, t.cursor+1))
	}
	return ansi.Cut(s, end-w, end), end - w
}

// ---------------------------------------------------------------- Checkbox and Radio

var checkAttrs = []string{"label", "checked", "disabled"}

// marks are the glyph pairs for each spelling: unchecked then checked.
var marks = map[string][2]string{
	"Checkbox": {"[ ]", "[x]"},
	"Radio":    {"( )", "(•)"},
}

// check is a labelled on/off control. Checkbox and Radio differ only in their
// glyphs: which of several is exclusive is the host's business, since the host
// owns the value they are drawn from.
type check struct {
	kind     string
	label    string
	checked  bool
	disabled bool
	state    widget.State
	measure  widget.Measurer
}

func newCheck(kind string) builder {
	return func(ctx widget.Context) (widget.Native, error) {
		c := &check{kind: kind, label: ctx.Attrs.String("label", ""), measure: ctx.Measure}
		var err error
		if c.checked, err = ctx.Attrs.Bool("checked", false); err != nil {
			return nil, err
		}
		if c.disabled, err = ctx.Attrs.Bool("disabled", false); err != nil {
			return nil, err
		}
		return c, nil
	}
}

func (c *check) AcceptsFocus() bool { return !c.disabled }

func (c *check) SetState(state widget.State) { c.state = state }

func (c *check) mark() string {
	glyphs := marks[c.kind]
	if c.checked {
		return glyphs[1]
	}
	return glyphs[0]
}

func (c *check) body() string {
	if c.label == "" {
		return c.mark()
	}
	return c.mark() + " " + c.label
}

func (c *check) Measure(_, _ int) (int, int) { return c.measure.Width(c.body()), 1 }

func (c *check) Render(_, _ int) string {
	style := lipgloss.NewStyle()
	switch {
	case c.disabled:
		style = style.Faint(true)
	case c.state.Pressed:
		style = style.Reverse(true)
	case c.state.Focused:
		style = style.Bold(true).Underline(true)
	case c.state.Hovered:
		style = style.Underline(true)
	}
	return style.Render(c.body())
}
