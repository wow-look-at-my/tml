// profile.go holds Profile.

package cell

import (
	"strconv"
	"strings"
)

// Profile is how much of a style a terminal can show.
type Profile uint8

const (
	// TrueColor shows every colour as written.
	TrueColor Profile = iota
	// ANSI256 maps an RGB colour to the nearest palette colour.
	ANSI256
	// ANSI maps every colour to the terminal's own base colours.
	ANSI
	// NoColor keeps the attributes and drops the colours.
	NoColor
	// Plain writes no escape sequence at all.
	Plain
)

// DetectProfile reads the environment for what the terminal behind a stream can show. tty says whether the stream is a
// terminal at all. A stream that is not one gets plain text, unless CLICOLOR_FORCE says otherwise.
func DetectProfile(env func(string) string, tty bool) Profile {
	if force := env("CLICOLOR_FORCE"); force != "" && force != "0" {
		tty = true
	}
	term := env("TERM")
	if !tty || term == "dumb" {
		return Plain
	}
	if env("NO_COLOR") != "" {
		return NoColor
	}
	switch env("COLORTERM") {
	case "truecolor", "24bit":
		return TrueColor
	}
	switch env("TERM_PROGRAM") {
	case "iTerm.app", "WezTerm", "ghostty", "vscode", "Hyper":
		return TrueColor
	}
	if env("WT_SESSION") != "" || env("KITTY_WINDOW_ID") != "" {
		return TrueColor
	}
	switch {
	case strings.Contains(term, "direct"), strings.Contains(term, "truecolor"):
		return TrueColor
	case term == "xterm-kitty", term == "xterm-ghostty", term == "alacritty", term == "wezterm", strings.HasPrefix(term, "foot"):
		return TrueColor
	case strings.Contains(term, "256color"):
		return ANSI256
	default:
		return ANSI
	}
}

// writeTransition writes what turns a terminal drawing in from into one drawing in to.
func writeTransition(out *strings.Builder, from, to Style, profile Profile) {
	if profile == Plain {
		return
	}
	fromLook, toLook := from, to
	fromLook.Link, toLook.Link = "", ""
	if fromLook != toLook {
		out.WriteString("\x1b[")
		params := sgrParams(to, profile)
		if !fromLook.IsZero() && params != "" {
			out.WriteString("0;")
		}
		out.WriteString(params)
		out.WriteByte('m')
	}
	if from.Link != to.Link {
		out.WriteString("\x1b]8;;")
		out.WriteString(to.Link)
		out.WriteString("\x1b\\")
	}
}

// sgrParams spells a style as SGR parameters. A style that states nothing is the empty string, which is a reset.
func sgrParams(s Style, profile Profile) string {
	var params []string
	for _, attr := range []struct {
		bit  Attr
		code string
	}{
		{Bold, "1"}, {Faint, "2"}, {Italic, "3"}, {Underline, "4"}, {Blink, "5"}, {Reverse, "7"}, {Strikethrough, "9"},
	} {
		if s.on&attr.bit != 0 {
			params = append(params, attr.code)
		}
	}
	if profile != NoColor {
		if s.Fg.IsSet() {
			params = append(params, colorParams(s.Fg, profile, false))
		}
		if s.Bg.IsSet() {
			params = append(params, colorParams(s.Bg, profile, true))
		}
	}
	return strings.Join(params, ";")
}

func colorParams(c Color, profile Profile, background bool) string {
	base, extended := 30, "38"
	if background {
		base, extended = 40, "48"
	}
	if c.kind == colorRGB {
		switch profile {
		case TrueColor:
			return extended + ";2;" + strconv.Itoa(int(c.r)) + ";" + strconv.Itoa(int(c.g)) + ";" + strconv.Itoa(int(c.b))
		case ANSI256:
			c = Indexed(nearest256(c.r, c.g, c.b))
		default:
			c = Indexed(nearest16(c.r, c.g, c.b))
		}
	}
	index := int(c.r)
	if profile == ANSI && index >= 16 {
		p := palette(c.r)
		index = int(nearest16(p[0], p[1], p[2]))
	}
	switch {
	case index < 8:
		return strconv.Itoa(base + index)
	case index < 16:
		return strconv.Itoa(base + 60 + index - 8)
	default:
		return extended + ";5;" + strconv.Itoa(index)
	}
}

func distance(r1, g1, b1, r2, g2, b2 uint8) int {
	dr, dg, db := int(r1)-int(r2), int(g1)-int(g2), int(b1)-int(b2)
	return dr*dr + dg*dg + db*db
}

// nearest256 is the palette colour closest to an RGB colour, out of the colour cube and the grey ramp.
func nearest256(r, g, b uint8) uint8 {
	best, bestDistance := uint8(16), 1<<30
	for n := 16; n < 256; n++ {
		p := palette(uint8(n))
		if d := distance(r, g, b, p[0], p[1], p[2]); d < bestDistance {
			best, bestDistance = uint8(n), d
		}
	}
	return best
}

// nearest16 is the base colour closest to an RGB colour.
func nearest16(r, g, b uint8) uint8 {
	best, bestDistance := uint8(0), 1<<30
	for n := range base16 {
		p := base16[n]
		if d := distance(r, g, b, p[0], p[1], p[2]); d < bestDistance {
			best, bestDistance = uint8(n), d
		}
	}
	return best
}
