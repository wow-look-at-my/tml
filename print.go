package tml

import (
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/term"

	"github.com/wow-look-at-my/tml/render"
)

// UnboundedWidth is the width Print lays a document out at when w is not a terminal.
const UnboundedWidth = 1 << 14

// printHeight caps the rows of a printed document. A printed document scrolls, so the cap only guards the arithmetic.
const printHeight = 1 << 20

// Print writes the view to w as a document: as wide and as tall as its content, with no blank cells after a line.
//
// A width of zero is the terminal's width when w is a terminal, and UnboundedWidth otherwise. The escapes follow w
// and the environment. A pipe, a file or TERM=dumb gets plain text, NO_COLOR keeps bold and drops colour, and a
// terminal gets the colours it supports.
func (v *View) Print(w io.Writer, props Props, width int) error {
	if width <= 0 {
		width = PrintWidth(w)
	}
	node, err := v.Expand(props)
	if err != nil {
		return err
	}
	box, err := v.engine.Shrink(node, width, printHeight)
	if err != nil {
		return err
	}
	out := render.Render(box)
	v.record(box, out, box.Rect.W, box.Rect.H)
	text := trimLines(out)
	if text == "" {
		return nil
	}
	_, err = io.WriteString(colorprofile.NewWriter(w, os.Environ()), text+"\n")
	return err
}

// PrintWidth is the width Print uses for w when it is given none: the terminal's columns, or UnboundedWidth.
func PrintWidth(w io.Writer) int {
	if f, ok := w.(term.File); ok && term.IsTerminal(f.Fd()) {
		if cols, _, err := term.GetSize(f.Fd()); err == nil && cols > 0 {
			return cols
		}
	}
	return UnboundedWidth
}

// trimLines drops the blanks at the end of each line, and the blank lines at the end of the document. The layout
// pads every line to the document's width, which on a pipe is only noise.
func trimLines(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = trimTrailingBlanks(line)
	}
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return strings.Join(lines, "\n")
}

// trimTrailingBlanks drops the spaces after the last visible cell. A space inside an escape run is part of the escape
// run, so the line keeps its escapes and loses only the blanks.
func trimTrailingBlanks(line string) string {
	end := len(line)
	for end > 0 {
		switch {
		case line[end-1] == ' ':
			end--
		case strings.HasSuffix(line[:end], "\x1b[m"):
			end -= len("\x1b[m")
		case strings.HasSuffix(line[:end], "\x1b[0m"):
			end -= len("\x1b[0m")
		default:
			trimmed := line[:end]
			if end < len(line) && strings.Contains(line[end:], "\x1b[") {
				// A reset after the last cell keeps the next line from inheriting a style.
				trimmed += "\x1b[m"
			}
			return trimmed
		}
	}
	return ""
}
