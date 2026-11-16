package main

import (
	"testing"

	"github.com/wow-look-at-my/tml/cell"
)

func TestProbe(t *testing.T) {
	for _, name := range []string{"TERM", "TERM_PROGRAM", "KITTY_WINDOW_ID", "GHOSTTY_RESOURCES_DIR", "WEZTERM_EXECUTABLE", "LC_TERMINAL"} {
		t.Setenv(name, "")
	}
	out := ""
	for _, tab := range []string{"controls", "data", "media"} {
		m, err := newModel()
		if err != nil {
			t.Fatal(err)
		}
		m.width, m.height = 92, 26
		m.query, m.cursor, m.progress = "sched", 5, 62
		m.tab = tab
		out += "\n=== " + tab + "\n" + cell.Strip(m.frameOf())
	}
	m, _ := newModel()
	m.width, m.height = 92, 26
	m.confirming = true
	out += "\n=== popup\n" + cell.Strip(m.frameOf())
	t.Fatal(out)
}
