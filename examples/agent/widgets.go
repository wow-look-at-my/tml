// widgets.go holds the widgets code.

package main

import (
	"strings"

	"github.com/wow-look-at-my/tml/cell"
	"github.com/wow-look-at-my/tml/widget"
)

// Both widgets this example binds are the point of it: neither belongs in the language's own library.
// A role names the theme colour of its label.
var roles = map[string]struct {
	label  string
	colour string
}{
	"you":   {label: "you", colour: "accent"},
	"agent": {label: "tml", colour: "ok"},
	"tool":  {label: " ⚙ ", colour: "warn"},
	"note":  {label: "   ", colour: "dim"},
}

var roleOrder = []string{"you", "agent", "tool", "note"}

// transcript draws a conversation: a role gutter, then the turn's text wrapped to whatever width is left. TML cannot
type transcript struct {
	entries []entry
	measure widget.Measurer
	theme   cell.Palette
}

type entry struct {
	role string
	text string
}

func newTranscript(ctx widget.Context) (widget.Native, error) {
	t := &transcript{measure: ctx.Measure, theme: ctx.Theme}
	for _, name := range roleOrder {
		if _, err := ctx.Theme.Named(roles[name].colour); err != nil {
			return nil, err
		}
	}
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
	height := 0
	for i, e := range t.entries {
		if i > 0 {
			height++
		}
		height += len(t.wrap(e.text, maxW))
	}
	return maxW, height
}

// wrap is one turn's text at the width the gutter leaves.
func (t *transcript) wrap(text string, w int) []cell.Line {
	return t.measure.Wrap(text, max(1, w-gutter))
}

// Draw lays every turn out at the canvas's width: the role in the gutter of its first line, then the text.
func (t *transcript) Draw(c cell.Canvas) {
	w, _ := c.Size()
	y := 0
	for i, e := range t.entries {
		if i > 0 {
			y++
		}
		role := roles[e.role]
		c.Text(1, y, role.label, cell.Style{Fg: t.theme[role.colour]}.Bold(true))
		for _, line := range t.wrap(e.text, w) {
			c.Line(gutter, y, line, cell.Style{})
			y++
		}
	}
}

// diff draws a unified diff: a marker column, then the line, coloured by what happened to it.
type diff struct {
	lines   []string
	measure widget.Measurer
	// added, removed and hunk are the theme colours of the lines a diff marks.
	added, removed, hunk cell.Color
}

func newDiff(ctx widget.Context) (widget.Native, error) {
	d := &diff{lines: ctx.Attrs.List("lines"), measure: ctx.Measure}
	for name, into := range map[string]*cell.Color{"ok": &d.added, "danger": &d.removed, "note": &d.hunk} {
		color, err := ctx.Theme.Named(name)
		if err != nil {
			return nil, err
		}
		*into = color
	}
	return d, nil
}

// Measure reports the height the diff draws to, wrapping included. A long line takes more than one row, and a card
// measured without that would cut the diff short.
func (d *diff) Measure(maxW, _ int) (int, int) {
	width := 0
	for _, line := range d.lines {
		width = max(width, d.measure.Width(line))
	}
	if maxW > 0 {
		width = min(width, maxW)
	}
	height := 0
	for _, line := range d.lines {
		height += len(d.measure.Wrap(line, width))
	}
	return width, height
}

func (d *diff) Draw(c cell.Canvas) {
	w, _ := c.Size()
	y := 0
	for _, line := range d.lines {
		style := d.style(line)
		for _, row := range d.measure.Wrap(line, w) {
			c.Line(0, y, row, style)
			y++
		}
	}
}

// style colours a line by its marker. A hunk header is not a change, so it gets a colour of its own, and context is
// dimmed.
func (d *diff) style(line string) cell.Style {
	switch {
	case strings.HasPrefix(line, "+"):
		return cell.Style{Fg: d.added}
	case strings.HasPrefix(line, "-"):
		return cell.Style{Fg: d.removed}
	case strings.HasPrefix(line, "@@"):
		return cell.Style{Fg: d.hunk}
	default:
		return cell.Style{}.Faint(true)
	}
}
