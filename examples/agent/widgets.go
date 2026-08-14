package main

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/wow-look-at-my/tml/widget"
)

// The two widgets this example binds are the point of it: neither belongs in
// the language's own library, both are drawn entirely through the public seam,
// and the template uses them exactly like a built-in.

// roles are the speakers a transcript line can have, and how each is marked.
var roles = map[string]struct {
	label  string
	colour string
}{
	"you":   {label: "you", colour: "12"},
	"agent": {label: "tml", colour: "10"},
	"tool":  {label: " ⚙ ", colour: "11"},
	"note":  {label: "   ", colour: "8"},
}

var roleOrder = []string{"you", "agent", "tool", "note"}

// transcript draws a conversation: a role gutter, then the turn's text wrapped
// to whatever width is left.
//
// TML cannot express this on its own, and that is worth saying rather than
// working around: a template can repeat over a list of strings, but it has no
// way to switch on what a list item IS. A heterogeneous transcript is either a
// widget or a language feature, and a widget is the honest one today.
type transcript struct {
	entries []entry
}

type entry struct {
	role string
	text string
}

func newTranscript(ctx widget.Context) (widget.Native, error) {
	t := &transcript{}
	for _, raw := range ctx.Attrs.List("entries") {
		role, text, _ := strings.Cut(raw, "|")
		if _, known := roles[role]; !known {
			return nil, ctx.Attrs.Errorf("entries", "unknown role %q, expected one of %s",
				role, strings.Join(roleOrder, ", "))
		}
		t.entries = append(t.entries, entry{role: role, text: text})
	}
	return t, nil
}

// gutter is the width of the role column, label plus a space either side.
const gutter = 5

func (t *transcript) Measure(maxW, _ int) (int, int) {
	if maxW <= 0 {
		maxW = 60
	}
	return maxW, lipgloss.Height(t.body(maxW))
}

func (t *transcript) Render(w, h int) string {
	body := t.body(w)
	if h > 0 && lipgloss.Height(body) > h {
		body = strings.Join(strings.Split(body, "\n")[:h], "\n")
	}
	return body
}

// body lays every turn out at the given width. A blank line between turns is
// what makes a wall of text readable as a conversation.
func (t *transcript) body(w int) string {
	var lines []string
	for i, e := range t.entries {
		if i > 0 {
			lines = append(lines, "")
		}
		role := roles[e.role]
		wrapped := lipgloss.NewStyle().Width(max(1, w-gutter)).Render(e.text)
		for n, line := range strings.Split(wrapped, "\n") {
			mark := strings.Repeat(" ", gutter)
			if n == 0 {
				mark = " " + lipgloss.NewStyle().
					Foreground(lipgloss.Color(role.colour)).
					Bold(true).
					Render(pad(role.label)) + " "
			}
			lines = append(lines, mark+line)
		}
	}
	return strings.Join(lines, "\n")
}

func pad(label string) string {
	for lipgloss.Width(label) < gutter-2 {
		label += " "
	}
	return label
}

// diff draws a unified diff: a marker column, then the line, coloured by what
// happened to it.
type diff struct {
	lines []string
}

func newDiff(ctx widget.Context) (widget.Native, error) {
	return &diff{lines: ctx.Attrs.List("lines")}, nil
}

// Measure reports the height the diff actually draws to, wrapping included. A
// long line becomes two rows, and a card sized for the unwrapped count loses its
// bottom border to the clip.
func (d *diff) Measure(maxW, _ int) (int, int) {
	width := 0
	for _, line := range d.lines {
		width = max(width, lipgloss.Width(line))
	}
	if maxW > 0 {
		width = min(width, maxW)
	}
	return width, lipgloss.Height(d.body(width))
}

func (d *diff) Render(w, h int) string {
	body := d.body(w)
	if h > 0 && lipgloss.Height(body) > h {
		body = strings.Join(strings.Split(body, "\n")[:h], "\n")
	}
	return body
}

func (d *diff) body(w int) string {
	out := make([]string, 0, len(d.lines))
	for _, line := range d.lines {
		out = append(out, d.style(line).Width(max(0, w)).Render(line))
	}
	return strings.Join(out, "\n")
}

// style colours a line by its marker. A hunk header is dimmed rather than
// coloured, because it is not a change -- it says where the changes are.
func (d *diff) style(line string) lipgloss.Style {
	style := lipgloss.NewStyle()
	switch {
	case strings.HasPrefix(line, "+"):
		return style.Foreground(lipgloss.Color("10"))
	case strings.HasPrefix(line, "-"):
		return style.Foreground(lipgloss.Color("9"))
	case strings.HasPrefix(line, "@@"):
		return style.Foreground(lipgloss.Color("13"))
	default:
		return style.Faint(true)
	}
}
