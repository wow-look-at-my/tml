package tml_test

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"

	"github.com/wow-look-at-my/tml"
)

func TestKeyMsgSpellsTheProtocolsVocabulary(t *testing.T) {
	cases := map[string]tea.KeyPressMsg{
		"a":            {Code: 'a', Text: "a"},
		"enter":        {Code: tea.KeyEnter},
		"esc":          {Code: tea.KeyEscape},
		"escape":       {Code: tea.KeyEscape},
		"space":        {Code: tea.KeySpace},
		"ctrl+c":       {Code: 'c', Mod: tea.ModCtrl},
		"ctrl+enter":   {Code: tea.KeyEnter, Mod: tea.ModCtrl},
		"alt+left":     {Code: tea.KeyLeft, Mod: tea.ModAlt},
		"ctrl+shift+a": {Code: 'a', Mod: tea.ModCtrl | tea.ModShift},
	}
	for name, want := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, want, tml.KeyMsg(name))
		})
	}
}

// A bare modifier is a key press with no code rather than a panic on the first
// rune of an empty string.
func TestKeyMsgTakesABareModifier(t *testing.T) {
	assert.Equal(t, tea.KeyPressMsg{Mod: tea.ModCtrl}, tml.KeyMsg("ctrl+"))
}
