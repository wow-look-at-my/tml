// style.go holds Attr.

package cell

// Attr is a set of text attributes.
type Attr uint8

const (
	Bold Attr = 1 << iota
	Faint
	Italic
	Underline
	Reverse
	Strikethrough
	Blink
)

// Style is how a cell draws. Every field can be unset, which is what lets one style lie over another: an unset field
// shows what is beneath it.
type Style struct {
	Fg, Bg Color
	// on and off are the attributes this style states. An attribute in neither is inherited.
	on, off Attr
	Link    string
}

// Foreground returns the style with a text colour.
func (s Style) Foreground(c Color) Style { s.Fg = c; return s }

// Background returns the style with a fill colour.
func (s Style) Background(c Color) Style { s.Bg = c; return s }

// With returns the style with the attributes stated: set when on is true, cleared otherwise.
func (s Style) With(attrs Attr, on bool) Style {
	if on {
		s.on |= attrs
		s.off &^= attrs
	} else {
		s.off |= attrs
		s.on &^= attrs
	}
	return s
}

func (s Style) Bold(on bool) Style          { return s.With(Bold, on) }
func (s Style) Faint(on bool) Style         { return s.With(Faint, on) }
func (s Style) Italic(on bool) Style        { return s.With(Italic, on) }
func (s Style) Underline(on bool) Style     { return s.With(Underline, on) }
func (s Style) Reverse(on bool) Style       { return s.With(Reverse, on) }
func (s Style) Strikethrough(on bool) Style { return s.With(Strikethrough, on) }

// Hyperlink returns the style with a link target.
func (s Style) Hyperlink(url string) Style { s.Link = url; return s }

// Has reports whether every attribute in attrs is on.
func (s Style) Has(attrs Attr) bool { return s.on&attrs == attrs }

// Attrs is the set of attributes that are on.
func (s Style) Attrs() Attr { return s.on }

// IsZero reports whether the style states nothing.
func (s Style) IsZero() bool { return s == Style{} }

// Over lays the style on base: whatever s states wins, and the rest comes from base.
func (s Style) Over(base Style) Style {
	out := base
	if s.Fg.IsSet() {
		out.Fg = s.Fg
	}
	if s.Bg.IsSet() {
		out.Bg = s.Bg
	}
	out.on = (base.on &^ s.off) | s.on
	out.off = (base.off &^ s.on) | s.off
	if s.Link != "" {
		out.Link = s.Link
	}
	return out
}

// Text is the half of the style that belongs to a glyph, without the fill behind it.
func (s Style) Text() Style {
	s.Bg = Color{}
	return s
}

// drawn is the style as a terminal sees it: only what is on.
func (s Style) drawn() Style {
	s.off = 0
	return s
}
