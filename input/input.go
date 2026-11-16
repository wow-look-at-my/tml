// Package input is what a terminal tells a program: keys, the pointer, pasted text, focus and size. The types are
// plain values, so a test or the inspector builds one the same way the terminal decoder does.
package input

import (
	"strings"
	"unicode"
)

// Mod is the set of modifier keys held with a key or a pointer event.
type Mod uint8

const (
	Shift Mod = 1 << iota
	Alt
	Ctrl
	Meta
)

// The keys that are not a character. Each is a code past the last Unicode code point, so none collides with text.
const (
	KeyUp rune = unicode.MaxRune + 1 + iota
	KeyDown
	KeyLeft
	KeyRight
	KeyHome
	KeyEnd
	KeyPgUp
	KeyPgDown
	KeyInsert
	KeyDelete
	KeyF1
	KeyF2
	KeyF3
	KeyF4
	KeyF5
	KeyF6
	KeyF7
	KeyF8
	KeyF9
	KeyF10
	KeyF11
	KeyF12
)

// The keys that are also a control character.
const (
	KeyEnter     rune = '\r'
	KeyTab       rune = '\t'
	KeyEscape    rune = 0x1b
	KeyBackspace rune = 0x7f
	KeySpace     rune = ' '
)

// Key is one key press.
type Key struct {
	// Code is the key: a character in its unshifted form, or one of the Key constants.
	Code rune
	Mod  Mod
	// Text is what the press types. It is empty for a key that types nothing, and for a key held with ctrl or alt.
	Text string
}

// names are the keys a keystroke spells by name, in the order a lookup by code prefers.
var names = []struct {
	name string
	code rune
}{
	{"enter", KeyEnter}, {"tab", KeyTab}, {"esc", KeyEscape}, {"escape", KeyEscape},
	{"space", KeySpace}, {"backspace", KeyBackspace}, {"delete", KeyDelete}, {"insert", KeyInsert},
	{"up", KeyUp}, {"down", KeyDown}, {"left", KeyLeft}, {"right", KeyRight},
	{"home", KeyHome}, {"end", KeyEnd}, {"pgup", KeyPgUp}, {"pgdown", KeyPgDown},
	{"f1", KeyF1}, {"f2", KeyF2}, {"f3", KeyF3}, {"f4", KeyF4}, {"f5", KeyF5}, {"f6", KeyF6},
	{"f7", KeyF7}, {"f8", KeyF8}, {"f9", KeyF9}, {"f10", KeyF10}, {"f11", KeyF11}, {"f12", KeyF12},
}

// mods are the modifier prefixes, in the order Keystroke writes them.
var mods = []struct {
	prefix string
	mod    Mod
}{
	{"ctrl+", Ctrl}, {"alt+", Alt}, {"shift+", Shift}, {"meta+", Meta},
}

// Keystroke names the press by its key and modifiers.
func (k Key) Keystroke() string {
	var b strings.Builder
	for _, m := range mods {
		if k.Mod&m.mod != 0 {
			b.WriteString(m.prefix)
		}
	}
	for _, n := range names {
		if n.code == k.Code {
			b.WriteString(n.name)
			return b.String()
		}
	}
	if k.Code != 0 {
		b.WriteRune(k.Code)
	}
	return b.String()
}

// String is the text the press types when it types any, and its keystroke otherwise. A space is spelled `space`.
func (k Key) String() string {
	if k.Text != "" && k.Text != " " {
		return k.Text
	}
	return k.Keystroke()
}

// ParseKey reads a keystroke name back into the press a terminal delivers for it. A name that is not a named key is a
// single character.
func ParseKey(name string) Key {
	var key Key
	for cut := true; cut; {
		cut = false
		for _, m := range mods {
			if rest, ok := strings.CutPrefix(name, m.prefix); ok && rest != "" {
				key.Mod |= m.mod
				name = rest
				cut = true
			}
		}
	}
	for _, n := range names {
		if n.name == name {
			key.Code = n.code
			if key.Code == KeySpace && key.Mod == 0 {
				key.Text = " "
			}
			return key
		}
	}
	if name == "" {
		return key
	}
	key.Code = []rune(name)[0]
	// A modified key types nothing: a terminal sends ctrl+c as a code and a modifier, never as the letter.
	if key.Mod&^Shift == 0 {
		key.Text = string(key.Code)
		if key.Mod&Shift != 0 {
			key.Text = strings.ToUpper(key.Text)
		}
	}
	return key
}

// Button is a pointer button.
type Button uint8

const (
	ButtonNone Button = iota
	ButtonLeft
	ButtonMiddle
	ButtonRight
	WheelUp
	WheelDown
	WheelLeft
	WheelRight
)

// Action is what the pointer did.
type Action uint8

const (
	// Press is a button going down.
	Press Action = iota
	// Release is a button coming up.
	Release
	// Motion is the pointer moving, with or without a button held.
	Motion
	// Wheel is one notch of the wheel. Button says which way.
	Wheel
)

// Mouse is one pointer event, at a cell of the screen.
type Mouse struct {
	X, Y   int
	Button Button
	Action Action
	Mod    Mod
}

// Paste is text the terminal delivered as one paste.
type Paste struct{ Text string }

// Focus is the terminal window gaining or losing the focus.
type Focus struct{ Gained bool }

// Resize is the terminal's size in cells.
type Resize struct{ Width, Height int }
