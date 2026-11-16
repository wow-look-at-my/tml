// decode.go holds Decoder.

package term

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/wow-look-at-my/tml/input"
)

// Decoder turns the bytes a terminal sends into input values.
type Decoder struct {
	pending []byte
	// paste holds the text of a bracketed paste while it is still arriving.
	paste   strings.Builder
	pasting bool
}

const (
	esc        = 0x1b
	pasteStart = "\x1b[200~"
	pasteEnd   = "\x1b[201~"
)

// Feed takes the next bytes and returns every value they complete: input.Key,
// input.Mouse, input.Paste or input.Focus.
func (d *Decoder) Feed(p []byte) []any {
	d.pending = append(d.pending, p...)
	return d.drain(false)
}

// Pending reports whether the decoder holds bytes it could not read yet.
func (d *Decoder) Pending() bool { return len(d.pending) > 0 }

// Flush reads what is left as it stands. A lone ESC is then the escape key.
func (d *Decoder) Flush() []any { return d.drain(true) }

func (d *Decoder) drain(final bool) []any {
	var out []any
	for len(d.pending) > 0 {
		if d.pasting {
			if !d.readPaste(&out) {
				break
			}
			continue
		}
		value, used := d.next(d.pending, final)
		if used == 0 {
			break
		}
		d.pending = d.pending[used:]
		if value != nil {
			out = append(out, value)
		}
	}
	if len(d.pending) == 0 {
		d.pending = nil
	}
	return out
}

// readPaste moves pasted bytes into the paste, and ends it at the closing sequence. It reports false when the bytes
// held so far could still be the start of that sequence.
func (d *Decoder) readPaste(out *[]any) bool {
	text := string(d.pending)
	if end := strings.Index(text, pasteEnd); end >= 0 {
		d.paste.WriteString(text[:end])
		d.pending = d.pending[end+len(pasteEnd):]
		*out = append(*out, input.Paste{Text: d.paste.String()})
		d.paste.Reset()
		d.pasting = false
		return true
	}
	// Keep back a tail that could be the first bytes of the closing sequence.
	keep := 0
	for n := min(len(pasteEnd)-1, len(text)); n > 0; n-- {
		if strings.HasPrefix(pasteEnd, text[len(text)-n:]) {
			keep = n
			break
		}
	}
	d.paste.WriteString(text[:len(text)-keep])
	d.pending = d.pending[len(text)-keep:]
	return false
}

// next reads one value from the front of p. It reports how many bytes the value took, and zero when p ends inside
// a sequence. A nil value with bytes used is a sequence that means nothing here.
func (d *Decoder) next(p []byte, final bool) (any, int) {
	b := p[0]
	switch {
	case b == esc:
		return d.escape(p, final)
	case b == 0:
		return input.Key{Code: input.KeySpace, Mod: input.Ctrl}, 1
	case b == '\r':
		return input.Key{Code: input.KeyEnter}, 1
	case b == '\t':
		return input.Key{Code: input.KeyTab}, 1
	case b == 0x7f:
		return input.Key{Code: input.KeyBackspace}, 1
	case b < 0x1b:
		return input.Key{Code: rune('a' + b - 1), Mod: input.Ctrl}, 1
	case b < 0x20:
		return input.Key{Code: rune('\\' + b - 0x1c), Mod: input.Ctrl}, 1
	}
	if !utf8.FullRune(p) && !final {
		return nil, 0
	}
	r, size := utf8.DecodeRune(p)
	if r == utf8.RuneError && size <= 1 {
		return nil, 1
	}
	return typed(r), size
}

// typed is the key for a character the terminal sent as text.
func typed(r rune) input.Key {
	key := input.Key{Code: r, Text: string(r)}
	if unicode.IsUpper(r) {
		key.Code = unicode.ToLower(r)
		key.Mod = input.Shift
	}
	return key
}

func (d *Decoder) escape(p []byte, final bool) (any, int) {
	if len(p) == 1 {
		if final {
			return input.Key{Code: input.KeyEscape}, 1
		}
		return nil, 0
	}
	switch p[1] {
	case '[':
		return d.csi(p, final)
	case 'O':
		if len(p) < 3 {
			if final {
				return input.Key{Code: 'o', Mod: input.Alt | input.Shift}, 2
			}
			return nil, 0
		}
		if key, ok := finalKeys[p[2]]; ok {
			return input.Key{Code: key}, 3
		}
		return nil, 3
	case ']', 'P', '_', '^', 'X':
		// A string the terminal sent in answer to a query. Nothing here asked for it.
		for i := 2; i < len(p); i++ {
			if p[i] == 0x07 {
				return nil, i + 1
			}
			if p[i] == esc && i+1 < len(p) && p[i+1] == '\\' {
				return nil, i + 2
			}
		}
		if final {
			return nil, len(p)
		}
		return nil, 0
	case esc:
		return input.Key{Code: input.KeyEscape}, 1
	}
	// ESC and then a key is that key with alt held.
	value, used := d.next(p[1:], final)
	if used == 0 {
		return nil, 0
	}
	if key, ok := value.(input.Key); ok {
		key.Mod |= input.Alt
		key.Text = ""
		return key, used + 1
	}
	return value, used + 1
}

// finalKeys are the keys a CSI or SS3 sequence names by its last byte.
var finalKeys = map[byte]rune{
	'A': input.KeyUp, 'B': input.KeyDown, 'C': input.KeyRight, 'D': input.KeyLeft,
	'H': input.KeyHome, 'F': input.KeyEnd,
	'P': input.KeyF1, 'Q': input.KeyF2, 'R': input.KeyF3, 'S': input.KeyF4,
}

// tildeKeys are the keys a CSI sequence that ends in a tilde names by its first number.
var tildeKeys = map[int]rune{
	1: input.KeyHome, 2: input.KeyInsert, 3: input.KeyDelete, 4: input.KeyEnd,
	5: input.KeyPgUp, 6: input.KeyPgDown, 7: input.KeyHome, 8: input.KeyEnd,
	11: input.KeyF1, 12: input.KeyF2, 13: input.KeyF3, 14: input.KeyF4, 15: input.KeyF5,
	17: input.KeyF6, 18: input.KeyF7, 19: input.KeyF8, 20: input.KeyF9, 21: input.KeyF10,
	23: input.KeyF11, 24: input.KeyF12,
}

func (d *Decoder) csi(p []byte, final bool) (any, int) {
	end := 2
	for end < len(p) && (p[end] < 0x40 || p[end] > 0x7e) {
		end++
	}
	if end >= len(p) {
		if final {
			return nil, len(p)
		}
		return nil, 0
	}
	body, last, used := string(p[2:end]), p[end], end+1

	if strings.HasPrefix(body, "<") && (last == 'M' || last == 'm') {
		return mouse(body[1:], last == 'm'), used
	}
	if strings.ContainsAny(body, "?>=$") {
		// A report the terminal sent in answer to a query.
		return nil, used
	}
	params := numbers(body)
	param := func(i, deflt int) int {
		if i < len(params) && params[i] > 0 {
			return params[i]
		}
		return deflt
	}
	mod := modifiers(param(1, 1))

	switch last {
	case '~':
		switch param(0, 0) {
		case 200:
			d.pasting = true
			return nil, used
		case 27:
			// The xterm form for a modified key: the modifiers, then the character.
			return character(rune(param(2, 0)), mod), used
		}
		if key, ok := tildeKeys[param(0, 0)]; ok {
			return input.Key{Code: key, Mod: mod}, used
		}
	case 'u':
		// The kitty form: the character, then the modifiers.
		return character(rune(param(0, 0)), mod), used
	case 'Z':
		return input.Key{Code: input.KeyTab, Mod: input.Shift}, used
	case 'I':
		return input.Focus{Gained: true}, used
	case 'O':
		return input.Focus{Gained: false}, used
	default:
		// F3 shares its last byte with a cursor position report, which always carries a column.
		if last == 'R' && len(params) > 1 && params[0] != 1 {
			return nil, used
		}
		if key, ok := finalKeys[last]; ok {
			return input.Key{Code: key, Mod: mod}, used
		}
	}
	return nil, used
}

// character is a key the terminal named by code point, with its modifiers. It types its character when the
// modifiers leave it a character.
func character(r rune, mod input.Mod) any {
	if r == 0 {
		return nil
	}
	key := input.Key{Code: r, Mod: mod}
	switch r {
	case 13:
		key.Code = input.KeyEnter
	case 9:
		key.Code = input.KeyTab
	case 27:
		key.Code = input.KeyEscape
	case 127:
		key.Code = input.KeyBackspace
	default:
		if mod&^input.Shift == 0 && unicode.IsPrint(r) {
			key.Text = string(r)
			if mod&input.Shift != 0 {
				key.Text = strings.ToUpper(key.Text)
			}
		}
	}
	return key
}

// modifiers reads the modifier parameter of a sequence, which is one more than a bit set.
func modifiers(param int) input.Mod {
	bits := param - 1
	var mod input.Mod
	if bits&1 != 0 {
		mod |= input.Shift
	}
	if bits&2 != 0 {
		mod |= input.Alt
	}
	if bits&4 != 0 {
		mod |= input.Ctrl
	}
	if bits&8 != 0 {
		mod |= input.Meta
	}
	return mod
}

// numbers reads the parameters of a sequence. A sub-parameter after a colon is dropped.
func numbers(body string) []int {
	if body == "" {
		return nil
	}
	fields := strings.Split(body, ";")
	out := make([]int, len(fields))
	for i, field := range fields {
		field, _, _ = strings.Cut(field, ":")
		out[i], _ = strconv.Atoi(field)
	}
	return out
}

// mouse reads an SGR pointer report: the button bits, then the cell, counted from one.
func mouse(body string, release bool) any {
	params := numbers(body)
	if len(params) < 3 {
		return nil
	}
	bits := params[0]
	event := input.Mouse{X: params[1] - 1, Y: params[2] - 1}
	if bits&4 != 0 {
		event.Mod |= input.Shift
	}
	if bits&8 != 0 {
		event.Mod |= input.Alt
	}
	if bits&16 != 0 {
		event.Mod |= input.Ctrl
	}
	button := bits & 3
	switch {
	case bits&64 != 0:
		event.Action = input.Wheel
		event.Button = []input.Button{input.WheelUp, input.WheelDown, input.WheelLeft, input.WheelRight}[button]
		return event
	case bits&32 != 0:
		event.Action = input.Motion
	case release:
		event.Action = input.Release
	default:
		event.Action = input.Press
	}
	event.Button = []input.Button{input.ButtonLeft, input.ButtonMiddle, input.ButtonRight, input.ButtonNone}[button]
	return event
}
