package tml

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

// KeyMsg turns a key name from the inspection protocol into the message a
// terminal would have delivered.
//
// It lives here because the vocabulary is the protocol's, not each host's. A
// host that spells `ctrl+c` its own way answers `op=key` differently from the
// program next to it, and a driver has no way to tell. Named keys are the ones
// a driver sends; anything else is its first rune, which is right for every
// printable key.
func KeyMsg(key string) tea.KeyPressMsg {
	var mod tea.KeyMod
	name := key
	for {
		rest, ok := cutMod(name)
		if !ok {
			break
		}
		mod |= rest.mod
		name = rest.name
	}
	if code, ok := namedKeys[name]; ok {
		return tea.KeyPressMsg{Code: code, Mod: mod}
	}
	if name == "" {
		return tea.KeyPressMsg{Mod: mod}
	}
	code := []rune(name)[0]
	if mod != 0 {
		// A modified key produces no text: a terminal sends ctrl+c as a code
		// and a modifier, never as the letter c. Filling Text anyway hands a
		// host that reads it an input no terminal delivers -- which is how a
		// driver types a literal "c" into a composer by asking to cancel.
		return tea.KeyPressMsg{Code: code, Mod: mod}
	}
	return tea.KeyPressMsg{Code: code, Text: string(code)}
}

// namedKeys is every key a driver can ask for by name. A name that is not here
// is a single character, so the map is the whole list of keys that are not.
var namedKeys = map[string]rune{
	"enter": tea.KeyEnter, "tab": tea.KeyTab, "esc": tea.KeyEscape,
	"escape": tea.KeyEscape, "space": tea.KeySpace, "backspace": tea.KeyBackspace,
	"delete": tea.KeyDelete, "insert": tea.KeyInsert,
	"up": tea.KeyUp, "down": tea.KeyDown, "left": tea.KeyLeft, "right": tea.KeyRight,
	"home": tea.KeyHome, "end": tea.KeyEnd,
	"pgup": tea.KeyPgUp, "pgdown": tea.KeyPgDown,
}

type modPrefix struct {
	mod  tea.KeyMod
	name string
}

// cutMod takes one modifier off the front. They compose, so `ctrl+shift+a` is
// two passes rather than a name nobody thought to add to a table.
func cutMod(name string) (modPrefix, bool) {
	for prefix, mod := range map[string]tea.KeyMod{
		"ctrl+": tea.ModCtrl, "alt+": tea.ModAlt,
		"shift+": tea.ModShift, "meta+": tea.ModMeta,
	} {
		if rest, ok := strings.CutPrefix(name, prefix); ok {
			return modPrefix{mod: mod, name: rest}, true
		}
	}
	return modPrefix{}, false
}
