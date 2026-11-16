// screen.go holds Options.

package term

import (
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/wow-look-at-my/tml/cell"
)

// Options say how a Screen uses the terminal.
type Options struct {
	// Inline draws the frame in place, below what the terminal already shows, and leaves the scrollback alone.
	Inline bool
	// Mouse asks the terminal to report the pointer: buttons, the wheel and motion.
	Mouse bool
	// Paste asks for pasted text as one value rather than as keys.
	Paste bool
	// Focus asks to be told when the terminal window gains or loses the focus.
	Focus bool
	// Profile is how much colour the terminal shows.
	Profile cell.Profile
}

// Screen draws frames on a terminal. It keeps the last frame, and writes only the cells that changed.
type Screen struct {
	out  io.Writer
	opts Options
	// prev is the frame on screen, or nil when the next draw must repaint everything.
	prev *cell.Buffer
	// last is the newest frame handed to Draw, kept so an inline print can draw the region again.
	last *cell.Buffer
	// drawn is how many rows an inline region takes on screen.
	drawn int
	b     strings.Builder
}

// NewScreen returns a screen that writes to out. Nothing is written until Enter.
func NewScreen(out io.Writer, opts Options) *Screen {
	return &Screen{out: out, opts: opts}
}

const (
	syncOn  = "\x1b[?2026h"
	syncOff = "\x1b[?2026l"
	// gapLimit is how many unchanged cells a rewrite runs over before it stops and moves the cursor instead.
	gapLimit = 6
)

// modes is the sequence that turns the optional reports on or off.
func (s *Screen) modes(set string) string {
	var b strings.Builder
	if s.opts.Mouse {
		b.WriteString("\x1b[?1000" + set + "\x1b[?1003" + set + "\x1b[?1006" + set)
	}
	if s.opts.Paste {
		b.WriteString("\x1b[?2004" + set)
	}
	if s.opts.Focus {
		b.WriteString("\x1b[?1004" + set)
	}
	return b.String()
}

// Enter takes the terminal: the alternate screen unless inline, a hidden cursor, and the reports the options ask for.
func (s *Screen) Enter() error {
	seq := "\x1b[?25l" + s.modes("h")
	if !s.opts.Inline {
		seq = "\x1b[?1049h\x1b[H\x1b[2J" + seq
	}
	_, err := io.WriteString(s.out, seq)
	return err
}

// Leave gives the terminal back. An inline region comes off the screen, and what was printed above it stays.
func (s *Screen) Leave() error {
	seq := s.modes("l") + "\x1b[?25h"
	if s.opts.Inline {
		seq = "\r\x1b[J" + s.clears() + seq
	} else {
		seq = s.clears() + seq + "\x1b[?1049l"
	}
	s.prev, s.last, s.drawn = nil, nil, 0
	_, err := io.WriteString(s.out, seq)
	return err
}

// Invalidate makes the next Draw repaint every cell. A resize needs it: the terminal moved what it showed.
func (s *Screen) Invalidate() { s.prev = nil }

// clears is what removes the images of the frame on screen.
func (s *Screen) clears() string {
	if s.prev == nil {
		return ""
	}
	var seqs []string
	for _, p := range s.prev.Placements {
		if p.Clear != "" && !slices.Contains(seqs, p.Clear) {
			seqs = append(seqs, p.Clear)
		}
	}
	return strings.Join(seqs, "")
}

// Draw puts a frame on screen.
func (s *Screen) Draw(frame *cell.Buffer) error {
	s.b.Reset()
	s.b.WriteString(syncOn)
	s.last = frame
	if s.opts.Inline {
		s.inline(frame)
	} else {
		s.full(frame)
	}
	s.b.WriteString(syncOff)
	_, err := io.WriteString(s.out, s.b.String())
	return err
}

func (s *Screen) moveTo(x, y int) {
	s.b.WriteString("\x1b[")
	s.b.WriteString(strconv.Itoa(y + 1))
	s.b.WriteByte(';')
	s.b.WriteString(strconv.Itoa(x + 1))
	s.b.WriteByte('H')
}

// full draws on the alternate screen, where every cell has a fixed address.
func (s *Screen) full(frame *cell.Buffer) {
	repaint := s.prev == nil || s.prev.W != frame.W || s.prev.H != frame.H ||
		!slices.Equal(s.prev.Placements, frame.Placements)
	if repaint {
		// An image is not a cell, so a change to the images redraws the whole screen under them.
		s.b.WriteString(s.clears())
		s.b.WriteString("\x1b[2J")
	}
	for y := range frame.H {
		if repaint {
			if used := frame.Used(y); used > 0 {
				s.moveTo(0, y)
				frame.WriteRow(&s.b, y, 0, used, s.opts.Profile)
			}
			continue
		}
		s.changed(frame, y)
	}
	if repaint {
		for _, p := range frame.Placements {
			s.moveTo(p.X, p.Y)
			s.b.WriteString(p.Seq)
		}
	}
	s.prev = frame
}

// changed rewrites the cells of row y that differ from the frame on screen.
func (s *Screen) changed(frame *cell.Buffer, y int) {
	for x := 0; x < frame.W; x++ {
		if frame.At(x, y) == s.prev.At(x, y) {
			continue
		}
		start, end, same := x, x+1, 0
		for end < frame.W && same < gapLimit {
			if frame.At(end, y) == s.prev.At(end, y) {
				same++
			} else {
				same = 0
			}
			end++
		}
		end -= same
		// A rewrite never starts on the second half of a wide glyph.
		for start > 0 && frame.At(start, y).Width == 0 {
			start--
		}
		s.moveTo(start, y)
		frame.WriteRow(&s.b, y, start, end, s.opts.Profile)
		x = end
	}
}

// inline draws a region in place. The cursor rests on the region's first row between draws, at the left edge.
func (s *Screen) inline(frame *cell.Buffer) {
	same := s.prev != nil && s.prev.W == frame.W
	for y := range frame.H {
		if y > 0 {
			if y < s.drawn {
				s.b.WriteString("\x1b[1B\r")
			} else {
				// A new line at the bottom of the terminal scrolls, which is how the region makes room for itself.
				s.b.WriteString("\r\n")
			}
		}
		if same && y < s.drawn && y < s.prev.H && s.rowSame(frame, y) {
			continue
		}
		used := frame.Used(y)
		frame.WriteRow(&s.b, y, 0, used, s.opts.Profile)
		// A row that fills the width leaves nothing to erase, and an erase there would take its last cell.
		if used < frame.W {
			s.b.WriteString("\x1b[K")
		}
	}
	if s.drawn > frame.H {
		// The region got shorter, so the rows it no longer takes go blank.
		if frame.H > 0 {
			s.b.WriteString("\x1b[1B\r\x1b[J\x1b[1A")
		} else {
			s.b.WriteString("\r\x1b[J")
		}
	}
	if frame.H > 1 {
		s.b.WriteString("\x1b[" + strconv.Itoa(frame.H-1) + "A")
	}
	s.b.WriteByte('\r')
	if s.prev == nil || !slices.Equal(s.prev.Placements, frame.Placements) {
		s.b.WriteString(s.clears())
		for _, p := range frame.Placements {
			s.b.WriteString("\x1b7")
			if p.Y > 0 {
				s.b.WriteString("\x1b[" + strconv.Itoa(p.Y) + "B")
			}
			s.b.WriteString("\x1b[" + strconv.Itoa(p.X+1) + "G" + p.Seq + "\x1b8")
		}
	}
	s.drawn = frame.H
	s.prev = frame
}

func (s *Screen) rowSame(frame *cell.Buffer, y int) bool {
	for x := range frame.W {
		if frame.At(x, y) != s.prev.At(x, y) {
			return false
		}
	}
	return true
}

// Print writes lines above an inline region: it takes the region off, writes the lines where it was, and draws the
// region again below them. The lines scroll into the terminal's own history like any other output.
func (s *Screen) Print(lines []string) error {
	if len(lines) == 0 {
		return nil
	}
	s.b.Reset()
	s.b.WriteString(syncOn)
	s.b.WriteString("\r\x1b[J")
	for _, line := range lines {
		s.b.WriteString(line)
		// A line that left a style on must not colour the region below it.
		if s.opts.Profile != cell.Plain {
			s.b.WriteString("\x1b[m")
		}
		s.b.WriteString("\r\n")
	}
	s.prev, s.drawn = nil, 0
	if s.last != nil {
		s.inline(s.last)
	}
	s.b.WriteString(syncOff)
	_, err := io.WriteString(s.out, s.b.String())
	return err
}
