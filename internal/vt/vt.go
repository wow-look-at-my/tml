// Package vt is a terminal small enough to read: it replays the bytes a screen writer produced and answers what a
// user would see. It knows the sequences this module writes and drops the rest. Tests use it.
package vt

import (
	"strconv"
	"strings"

	"github.com/wow-look-at-my/tml/cell"
)

// Terminal is a screen of cells with a cursor, and the lines that scrolled off the top of it.
type Terminal struct {
	w, h int
	rows [][]string
	x, y int
	// wrap is set when a glyph landed on the last column: the next glyph starts the next row.
	wrap           bool
	history        []string
	savedX, savedY int
	// main holds the main screen while the alternate screen is up.
	main    [][]string
	pending string
}

// New returns a blank terminal.
func New(width, height int) *Terminal {
	t := &Terminal{w: width, h: height}
	t.rows = t.blank()
	return t
}

func (t *Terminal) blank() [][]string {
	rows := make([][]string, t.h)
	for i := range rows {
		rows[i] = t.blankRow()
	}
	return rows
}

func (t *Terminal) blankRow() []string {
	row := make([]string, t.w)
	for i := range row {
		row[i] = " "
	}
	return row
}

// Write replays output. A sequence cut off at the end of p waits for the next write.
func (t *Terminal) Write(p []byte) (int, error) {
	s := t.pending + string(p)
	t.pending = ""
	for len(s) > 0 {
		if s[0] != 0x1b {
			end := strings.IndexByte(s, 0x1b)
			if end < 0 {
				end = len(s)
			}
			t.text(s[:end])
			s = s[end:]
			continue
		}
		used := t.escape(s)
		if used == 0 {
			t.pending = s
			break
		}
		s = s[used:]
	}
	return len(p), nil
}

// WriteString is Write for a string.
func (t *Terminal) WriteString(s string) { _, _ = t.Write([]byte(s)) }

func (t *Terminal) text(s string) {
	for len(s) > 0 {
		switch s[0] {
		case '\r':
			t.x, t.wrap = 0, false
			s = s[1:]
		case '\n':
			t.lineFeed()
			s = s[1:]
		case '\b':
			t.x, t.wrap = max(0, t.x-1), false
			s = s[1:]
		case 0x07:
			s = s[1:]
		default:
			end := strings.IndexAny(s, "\r\n\b\a")
			if end < 0 {
				end = len(s)
			}
			for _, g := range cell.Lines(s[:end])[0] {
				t.put(g.Text, g.Width)
			}
			s = s[end:]
		}
	}
}

func (t *Terminal) lineFeed() {
	t.wrap = false
	if t.y < t.h-1 {
		t.y++
		return
	}
	if t.main == nil {
		t.history = append(t.history, strings.Join(t.rows[0], ""))
	}
	t.rows = append(t.rows[1:], t.blankRow())
}

func (t *Terminal) put(text string, width int) {
	if t.wrap || t.x+width > t.w {
		t.x = 0
		t.lineFeed()
	}
	t.rows[t.y][t.x] = text
	for i := 1; i < width; i++ {
		t.rows[t.y][t.x+i] = ""
	}
	t.x += width
	if t.x >= t.w {
		t.x = t.w - 1
		t.wrap = true
	}
}

// escape reads one sequence from the front of s and reports its length, or zero when s ends inside it.
func (t *Terminal) escape(s string) int {
	if len(s) < 2 {
		return 0
	}
	switch s[1] {
	case '[':
		end := 2
		for end < len(s) && (s[end] < 0x40 || s[end] > 0x7e) {
			end++
		}
		if end >= len(s) {
			return 0
		}
		t.csi(s[2:end], s[end])
		return end + 1
	case ']', 'P', '_', '^', 'X':
		for i := 2; i < len(s); i++ {
			if s[i] == 0x07 {
				return i + 1
			}
			if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '\\' {
				return i + 2
			}
		}
		return 0
	case '7':
		t.savedX, t.savedY = t.x, t.y
	case '8':
		t.x, t.y, t.wrap = t.savedX, t.savedY, false
	}
	return 2
}

func (t *Terminal) csi(body string, final byte) {
	private := strings.HasPrefix(body, "?")
	fields := strings.Split(strings.TrimPrefix(body, "?"), ";")
	param := func(i, deflt int) int {
		if i < len(fields) {
			if n, err := strconv.Atoi(fields[i]); err == nil && n > 0 {
				return n
			}
		}
		return deflt
	}
	if private {
		if param(0, 0) == 1049 {
			t.alternate(final == 'h')
		}
		return
	}
	switch final {
	case 'A':
		t.y, t.wrap = max(0, t.y-param(0, 1)), false
	case 'B':
		t.y, t.wrap = min(t.h-1, t.y+param(0, 1)), false
	case 'C':
		t.x, t.wrap = min(t.w-1, t.x+param(0, 1)), false
	case 'D':
		t.x, t.wrap = max(0, t.x-param(0, 1)), false
	case 'G':
		t.x, t.wrap = min(t.w-1, param(0, 1)-1), false
	case 'H', 'f':
		t.y, t.x, t.wrap = min(t.h-1, param(0, 1)-1), min(t.w-1, param(1, 1)-1), false
	case 'J':
		t.eraseScreen(body)
	case 'K':
		from := t.x
		if body == "2" {
			from = 0
		}
		for x := from; x < t.w; x++ {
			t.rows[t.y][x] = " "
		}
	}
}

func (t *Terminal) eraseScreen(body string) {
	if body == "2" || body == "3" {
		t.rows = t.blank()
		return
	}
	for x := t.x; x < t.w; x++ {
		t.rows[t.y][x] = " "
	}
	for y := t.y + 1; y < t.h; y++ {
		t.rows[y] = t.blankRow()
	}
}

func (t *Terminal) alternate(on bool) {
	switch {
	case on && t.main == nil:
		t.main = t.rows
		t.rows = t.blank()
	case !on && t.main != nil:
		t.rows = t.main
		t.main = nil
	}
}

// Screen is the rows on screen, each without the blanks at its end.
func (t *Terminal) Screen() []string {
	lines := make([]string, len(t.rows))
	for i, row := range t.rows {
		lines[i] = strings.TrimRight(strings.Join(row, ""), " ")
	}
	return lines
}

// History is the lines that scrolled off the top, oldest first, each without the blanks at its end.
func (t *Terminal) History() []string {
	lines := make([]string, len(t.history))
	for i, line := range t.history {
		lines[i] = strings.TrimRight(line, " ")
	}
	return lines
}

// Lines is everything a user scrolls back through and then sees, with the blank lines at the end dropped.
func (t *Terminal) Lines() []string {
	lines := append(t.History(), t.Screen()...)
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// Cursor is where the cursor rests.
func (t *Terminal) Cursor() (x, y int) { return t.x, t.y }
