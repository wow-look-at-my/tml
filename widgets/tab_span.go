// tab_span.go holds tabSpan.

package widgets

import "github.com/wow-look-at-my/tml/cell"

// tabSpan is where one label lands: its first column, its width with the padding, and the text it shows.
type tabSpan struct {
	x, w int
	text cell.Line
}
