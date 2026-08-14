package main

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var update = flag.Bool("update", false, "rewrite the golden files")

// gallery builds the model the binary builds, with the terminal pinned to one
// that can only do colour. Otherwise the image lands on whichever graphics
// protocol the machine running the test happens to advertise, and the golden
// records that machine rather than the layout.
func gallery(t *testing.T) *model {
	t.Helper()
	for _, name := range []string{
		"TERM", "TERM_PROGRAM", "KITTY_WINDOW_ID",
		"GHOSTTY_RESOURCES_DIR", "WEZTERM_EXECUTABLE", "LC_TERMINAL",
	} {
		t.Setenv(name, "")
	}

	m, err := newModel()
	require.NoError(t, err)
	m.width, m.height = 92, 24
	m.query = "sched"
	m.progress = 62
	return m
}

// Every widget the library ships is on one of these three sections, so the
// goldens are the library's own regression net: a widget that stops drawing
// shows up here as a diff rather than as a blank rectangle nobody noticed.
func TestEverySectionRenders(t *testing.T) {
	for _, section := range []struct {
		name  string
		popup bool
	}{
		{name: "controls"},
		{name: "data"},
		{name: "media"},
		{name: "controls", popup: true},
	} {
		file := section.name
		if section.popup {
			file += "-popup"
		}
		t.Run(file, func(t *testing.T) {
			m := gallery(t)
			m.tab, m.confirming = section.name, section.popup

			out := m.frameOf()
			require.NotContains(t, out, "tml: ", "the view failed to render")
			golden(t, file, ansi.Strip(out))
		})
	}
}

// The gallery routes every control through its action string, so an action the
// model has no case for is a button that does nothing when it is pressed. Every
// section's ring is walked, because a control only joins the ring on the frame
// its section is showing.
func TestEveryControlInTheRingIsWiredUp(t *testing.T) {
	m := gallery(t)
	seen := 0

	for _, tab := range []string{"controls", "data", "media"} {
		m.tab, m.confirming = tab, true
		require.NotContains(t, m.frameOf(), "tml: ")

		targets := m.view.UI().Targets()
		require.NotEmpty(t, targets, "%s has no focusable controls", tab)
		for _, target := range targets {
			if target.Action == "" {
				continue
			}
			assert.True(t, m.act(target.Action), "nothing answers action %q", target.Action)
			seen++
		}
	}
	assert.Greater(t, seen, 10, "the walk reached the sections' own controls, not just the tabs")
}

// The check above is only worth anything if act can actually say no.
func TestAnUnknownActionIsRejected(t *testing.T) {
	m := gallery(t)

	assert.False(t, m.act("nonsense"))
	assert.True(t, m.act("save"))
}

func golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")
	if *update {
		require.NoError(t, os.WriteFile(path, []byte(got), 0o644))
		return
	}
	assert.Equal(t, readGolden(t, path, got), got)
}

// readGolden returns the recorded frame. An empty golden is seeded from this run
// and then fails: seeding silently would bless whatever a broken frame happened
// to contain and turn the first run green for the wrong reason.
func readGolden(t *testing.T, path, got string) string {
	t.Helper()
	info, err := os.Stat(path)
	require.NoError(t, err, "missing golden file; rerun with -update")

	if info.Size() > 0 {
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		return string(data)
	}
	require.NoError(t, os.WriteFile(path, []byte(got), 0o644))
	t.Fatalf("golden %s was empty; seeded it from this run -- re-run to verify, and read the diff before trusting it", path)
	return ""
}
