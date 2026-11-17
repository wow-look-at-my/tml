package input

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseKeySpellsTheProtocolsVocabulary(t *testing.T) {
	cases := map[string]Key{
		"a":            {Code: 'a', Text: "a"},
		"enter":        {Code: KeyEnter},
		"esc":          {Code: KeyEscape},
		"escape":       {Code: KeyEscape},
		"space":        {Code: KeySpace, Text: " "},
		"ctrl+c":       {Code: 'c', Mod: Ctrl},
		"ctrl+enter":   {Code: KeyEnter, Mod: Ctrl},
		"ctrl+space":   {Code: KeySpace, Mod: Ctrl},
		"alt+left":     {Code: KeyLeft, Mod: Alt},
		"ctrl+shift+a": {Code: 'a', Mod: Ctrl | Shift},
		"shift+a":      {Code: 'a', Mod: Shift, Text: "A"},
		"shift+tab":    {Code: KeyTab, Mod: Shift},
		"f12":          {Code: KeyF12},
	}
	for name, want := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, want, ParseKey(name))
		})
	}
}

// A bare modifier is a key press with no code rather than a panic on the leading rune of an empty string.
func TestParseKeyTakesABareModifier(t *testing.T) {
	assert.Equal(t, Key{Mod: Ctrl}, ParseKey("ctrl+"))
}

// An empty name is the zero key.
func TestParseKeyOfNothingIsTheZeroKey(t *testing.T) {
	assert.Equal(t, Key{}, ParseKey(""))
}

// A keystroke reads back into the name it was parsed from, with the modifiers in their canonical order.
func TestKeystrokeNamesThePress(t *testing.T) {
	cases := map[string]string{
		"a":            "a",
		"enter":        "enter",
		"esc":          "esc",
		"escape":       "esc",
		"space":        "space",
		"ctrl+c":       "ctrl+c",
		"shift+tab":    "shift+tab",
		"alt+left":     "alt+left",
		"shift+ctrl+a": "ctrl+shift+a",
		"meta+alt+x":   "alt+meta+x",
		"pgdown":       "pgdown",
		"f1":           "f1",
	}
	for name, want := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, want, ParseKey(name).Keystroke())
		})
	}
}

// String is the typed text when the press types any, and the keystroke otherwise.
func TestStringIsTheTextOrTheKeystroke(t *testing.T) {
	cases := []struct {
		key  Key
		want string
	}{
		{ParseKey("a"), "a"},
		{ParseKey("shift+a"), "A"},
		{ParseKey("space"), "space"},
		{ParseKey("enter"), "enter"},
		{ParseKey("ctrl+c"), "ctrl+c"},
		{Key{Code: 'é', Text: "é"}, "é"},
		{Key{}, ""},
	}
	for _, c := range cases {
		t.Run(c.want, func(t *testing.T) {
			assert.Equal(t, c.want, c.key.String())
		})
	}
}
