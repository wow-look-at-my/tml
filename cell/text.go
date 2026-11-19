// text.go holds Measure.

package cell

import (
	"strconv"
	"strings"
	"unicode"

	"github.com/clipperhouse/displaywidth"
)

// Measure reports the width of a grapheme cluster in display cells.
type Measure func(string) int

// Glyph is one grapheme cluster and the style it was written in.
type Glyph struct {
	Text  string
	Width int
	Style Style
}

// Line is one row of text, as glyphs.
type Line []Glyph

// tabWidth is the cells a tab expands to.
const tabWidth = 4

// nbsp is the space that does not break a line.
const nbsp = ' '

// Width is the widest line of s in cells. Escape sequences take no cells.
func (m Measure) Width(s string) int {
	if !strings.ContainsAny(s, "\x1b\n\t\r") && m == nil {
		return displaywidth.String(s)
	}
	widest := 0
	for _, line := range m.Lines(s) {
		widest = max(widest, line.Width())
	}
	return widest
}

// Width is the widest line of s in cells, by the Unicode default.
func Width(s string) int { return Measure(nil).Width(s) }

// Strip is s without its escape sequences and control characters. The newlines stay.
func Strip(s string) string {
	if !strings.ContainsAny(s, "\x1b\t\r") {
		return s
	}
	lines := Lines(s)
	plain := make([]string, len(lines))
	for i, line := range lines {
		plain[i] = line.String()
	}
	return strings.Join(plain, "\n")
}

// Height is how many lines s has. An empty string is one line.
func Height(s string) int { return strings.Count(s, "\n") + 1 }

// Lines splits s at newlines and reads each line into glyphs. An SGR sequence styles the glyphs after it and an OSC
// hyperlink links them. Every other control sequence is dropped.
func (m Measure) Lines(s string) []Line {
	lines := []Line{nil}
	var style Style
	add := func(text string, width int) {
		lines[len(lines)-1] = append(lines[len(lines)-1], Glyph{Text: text, Width: width, Style: style})
	}
	for len(s) > 0 {
		if s[0] == 0x1b {
			s = readEscape(s, &style)
			continue
		}
		end := strings.IndexByte(s, 0x1b)
		if end < 0 {
			end = len(s)
		}
		graphemes := displaywidth.StringGraphemes(s[:end])
		for graphemes.Next() {
			text := graphemes.Value()
			switch {
			case text == "\n" || text == "\r\n":
				lines = append(lines, nil)
			case text == "\t":
				for range tabWidth {
					add(" ", 1)
				}
			case len(text) == 1 && (text[0] < 0x20 || text[0] == 0x7f):
				// A control character has no cell.
			default:
				width := graphemes.Width()
				if m != nil {
					width = m(text)
				}
				if width > 0 {
					add(text, min(width, 2))
				}
			}
		}
		s = s[end:]
	}
	return lines
}

// Lines reads s by the Unicode default widths.
func Lines(s string) []Line { return Measure(nil).Lines(s) }

// readEscape consumes one escape sequence from the front of s and applies it to style when it is SGR or a hyperlink.
func readEscape(s string, style *Style) string {
	if len(s) < 2 {
		return ""
	}
	switch s[1] {
	case '[':
		i := 2
		for i < len(s) && (s[i] < 0x40 || s[i] > 0x7e) {
			i++
		}
		if i >= len(s) {
			return ""
		}
		if s[i] == 'm' {
			applySGR(s[2:i], style)
		}
		return s[i+1:]
	case ']':
		body, rest := untilTerminator(s[2:])
		if link, ok := strings.CutPrefix(body, "8;"); ok {
			if _, url, found := strings.Cut(link, ";"); found {
				style.Link = url
			}
		}
		return rest
	case 'P', '_', '^', 'X':
		_, rest := untilTerminator(s[2:])
		return rest
	default:
		return s[2:]
	}
}

// untilTerminator splits a string sequence at its terminator: BEL, or ESC and a backslash.
func untilTerminator(s string) (body, rest string) {
	for i := 0; i < len(s); i++ {
		switch {
		case s[i] == 0x07:
			return s[:i], s[i+1:]
		case s[i] == 0x1b && i+1 < len(s) && s[i+1] == '\\':
			return s[:i], s[i+2:]
		}
	}
	return s, ""
}

// applySGR reads the parameters of one SGR sequence into style.
func applySGR(params string, style *Style) {
	if params == "" {
		*style = Style{Link: style.Link}
		return
	}
	fields := strings.FieldsFunc(params, func(r rune) bool { return r == ';' || r == ':' })
	nums := make([]int, 0, len(fields))
	for _, field := range fields {
		n, err := strconv.Atoi(field)
		if err != nil {
			return
		}
		nums = append(nums, n)
	}
	for i := 0; i < len(nums); i++ {
		switch n := nums[i]; {
		case n == 0:
			*style = Style{Link: style.Link}
		case n == 1:
			*style = style.Bold(true)
		case n == 2:
			*style = style.Faint(true)
		case n == 3:
			*style = style.Italic(true)
		case n == 4:
			*style = style.Underline(true)
		case n == 5:
			*style = style.With(Blink, true)
		case n == 7:
			*style = style.Reverse(true)
		case n == 9:
			*style = style.Strikethrough(true)
		case n == 22:
			*style = style.With(Bold|Faint, false)
		case n == 23:
			*style = style.Italic(false)
		case n == 24:
			*style = style.Underline(false)
		case n == 25:
			*style = style.With(Blink, false)
		case n == 27:
			*style = style.Reverse(false)
		case n == 29:
			*style = style.Strikethrough(false)
		case n >= 30 && n <= 37:
			style.Fg = Indexed(uint8(n - 30))
		case n == 39:
			style.Fg = Color{}
		case n >= 40 && n <= 47:
			style.Bg = Indexed(uint8(n - 40))
		case n == 49:
			style.Bg = Color{}
		case n >= 90 && n <= 97:
			style.Fg = Indexed(uint8(n - 90 + 8))
		case n >= 100 && n <= 107:
			style.Bg = Indexed(uint8(n - 100 + 8))
		case n == 38 || n == 48:
			c, used := extendedColor(nums[i+1:])
			if n == 38 {
				style.Fg = c
			} else {
				style.Bg = c
			}
			i += used
		}
	}
}

func extendedColor(rest []int) (Color, int) {
	switch {
	case len(rest) >= 2 && rest[0] == 5:
		return Indexed(uint8(rest[1])), 2
	case len(rest) >= 4 && rest[0] == 2:
		return RGB(uint8(rest[1]), uint8(rest[2]), uint8(rest[3])), 4
	default:
		return Color{}, len(rest)
	}
}

// Text builds a line of plain glyphs in one style.
func (m Measure) Text(s string, style Style) Line {
	lines := m.Lines(s)
	line := lines[0]
	for _, more := range lines[1:] {
		line = append(line, more...)
	}
	for i := range line {
		line[i].Style = style.Over(line[i].Style)
	}
	return line
}

// Width is the line's width in cells.
func (l Line) Width() int {
	width := 0
	for _, g := range l {
		width += g.Width
	}
	return width
}

// String is the line's text without its styles.
func (l Line) String() string {
	var b strings.Builder
	for _, g := range l {
		b.WriteString(g.Text)
	}
	return b.String()
}

// Styled returns the line with style laid under every glyph's own.
func (l Line) Styled(style Style) Line {
	out := make(Line, len(l))
	for i, g := range l {
		g.Style = g.Style.Over(style)
		out[i] = g
	}
	return out
}

// Cut is the cells of the line from column from up to column to. A wide glyph that straddles an end becomes a blank.
func (l Line) Cut(from, to int) Line {
	var out Line
	col := 0
	for _, g := range l {
		start, end := col, col+g.Width
		col = end
		if end <= from || start >= to {
			continue
		}
		if start < from || end > to {
			for range min(end, to) - max(start, from) {
				out = append(out, Glyph{Text: " ", Width: 1, Style: g.Style})
			}
			continue
		}
		out = append(out, g)
	}
	return out
}

// Truncate cuts the line to width. A line that loses cells ends in tail, which costs the line that many cells more.
func (l Line) Truncate(width int, tail string) Line {
	if l.Width() <= width {
		return l
	}
	mark := Lines(tail)[0]
	room := max(0, width-mark.Width())
	var out Line
	used := 0
	for _, g := range l {
		if used+g.Width > room {
			break
		}
		out = append(out, g)
		used += g.Width
	}
	if mark.Width() <= width {
		style := Style{}
		if len(out) > 0 {
			style = out[len(out)-1].Style
		}
		out = append(out, mark.Styled(style)...)
	}
	return out
}

// TruncatePath cuts a path to width in its middle. The start of the path stays as long as it fits, the last element
// always stays. The mark takes the place of what was cut before the last separator. A last element wider than width
// loses its own end instead.
func (l Line) TruncatePath(width int, mark string) Line {
	if l.Width() <= width {
		return l
	}
	sep := -1
	for i, g := range l {
		if g.Text == "/" || g.Text == `\` {
			sep = i
		}
	}
	if sep < 0 {
		return l.Truncate(width, mark)
	}
	dots := Lines(mark)[0]
	tail := l[sep:]
	style := l[sep].Style
	if room := width - dots.Width() - tail.Width(); room >= 0 {
		head := l[:sep].Cut(0, room)
		out := append(Line{}, head...)
		out = append(out, dots.Styled(style)...)
		return append(out, tail...)
	}
	base := l[sep+1:]
	if base.Width()+dots.Width() <= width {
		return append(append(Line{}, dots.Styled(style)...), base...)
	}
	return base.Truncate(width, mark)
}

func (g Glyph) space() bool {
	r := []rune(g.Text)[0]
	return unicode.IsSpace(r) && r != nbsp
}

// Wrap breaks the line into rows no wider than limit. It breaks at spaces and after hyphens, and inside a word only
// when the word is wider than a row. The spaces at a break are dropped.
func (l Line) Wrap(limit int) []Line {
	if limit < 1 || l.Width() <= limit {
		return []Line{l}
	}
	var (
		rows         []Line
		row, spaces  Line
		rowW, spaceW int
	)
	newline := func() {
		rows = append(rows, row)
		row, rowW = nil, 0
		spaces, spaceW = nil, 0
	}
	place := func(word Line) {
		if len(word) == 0 {
			return
		}
		width := word.Width()
		if (rowW > 0 || spaceW > 0) && rowW+spaceW+width > limit {
			newline()
		}
		row = append(row, spaces...)
		rowW += spaceW
		spaces, spaceW = nil, 0
		for _, g := range word {
			if rowW+g.Width > limit && rowW > 0 {
				newline()
			}
			row = append(row, g)
			rowW += g.Width
		}
	}

	var word Line
	for _, g := range l {
		switch {
		case g.space():
			place(word)
			word = nil
			spaces = append(spaces, g)
			spaceW += g.Width
		case g.Text == "-":
			word = append(word, g)
			place(word)
			word = nil
		default:
			word = append(word, g)
		}
	}
	place(word)
	if rowW+spaceW <= limit {
		row = append(row, spaces...)
	}
	return append(rows, row)
}

// Wrap reads s and wraps every line of it to width. A width under one leaves the lines as they are.
func (m Measure) Wrap(s string, width int) []Line {
	var rows []Line
	for _, line := range m.Lines(s) {
		rows = append(rows, line.Wrap(width)...)
	}
	return rows
}
