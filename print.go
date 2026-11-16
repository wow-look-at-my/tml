// print.go holds the print code.

package tml

import (
	"io"
	"os"
	"strings"

	"github.com/wow-look-at-my/tml/cell"
	"github.com/wow-look-at-my/tml/render"
	"github.com/wow-look-at-my/tml/term"
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
	frame := render.Alone(box, v.measure)
	v.record(box, frame, frame.W, frame.H)
	text := trimmed(frame, cell.DetectProfile(os.Getenv, term.IsTerminal(w)))
	if text == "" {
		return nil
	}
	_, err = io.WriteString(w, text+"\n")
	return err
}

// PrintWidth is the width Print uses for w when it is given none: the terminal's columns, or UnboundedWidth.
func PrintWidth(w io.Writer) int {
	if cols, _, err := term.Size(w); err == nil && cols > 0 {
		return cols
	}
	return UnboundedWidth
}

// trimmed is the frame as text without the blanks at the end of each line, or the blank lines at the end of the
// document. The layout pads every line to the document's width, which on a pipe is only noise.
func trimmed(frame *cell.Buffer, profile cell.Profile) string {
	last := frame.H
	for last > 0 && frame.Used(last-1) == 0 {
		last--
	}
	var out strings.Builder
	for y := range last {
		if y > 0 {
			out.WriteByte('\n')
		}
		frame.WriteRow(&out, y, 0, frame.Used(y), profile)
	}
	return out.String()
}
