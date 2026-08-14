package main

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml"
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
	m.width, m.height = 92, 26
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

// Clicking a tab is the whole loop: the click lands on a rect the last frame
// published, the ring turns it into an action, and the model switches section.
func TestClickingATabSwitchesSection(t *testing.T) {
	m := gallery(t)
	require.NotContains(t, m.frameOf(), "tml: ")

	target := control(t, m, "tab-media")
	press(m, target.Rect.X+1, target.Rect.Y+1)

	assert.Equal(t, "media", m.tab)
	assert.Contains(t, ansi.Strip(m.frameOf()), "Media ─", "the media section is the one drawn")
}

// Typing belongs to the field while the field has focus, so the ring's own keys
// and the quit key both have to stand aside there.
func TestTypingGoesToTheFocusedField(t *testing.T) {
	m := gallery(t)
	require.NotContains(t, m.frameOf(), "tml: ")
	require.True(t, m.view.UI().Focus("search"))

	for _, r := range "qq" {
		m.Update(tea.KeyPressMsg{Code: r})
	}
	assert.Equal(t, "schedqq", m.query)

	m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	assert.Equal(t, "schedq", m.query)
}

func TestQuitKeysOnlyBindOutsideAField(t *testing.T) {
	m := gallery(t)
	require.NotContains(t, m.frameOf(), "tml: ")
	require.True(t, m.view.UI().Focus("save"))

	_, cmd := m.Update(tea.KeyPressMsg{Code: 'q'})
	assert.NotNil(t, cmd, "q quits when the keyboard is not in a field")
}

// Escape backs out of the popup before it backs out of the program: a modal that
// quits the whole thing on the key everyone presses to dismiss it is a trap.
func TestEscapeClosesThePopupFirst(t *testing.T) {
	m := gallery(t)
	m.confirming = true

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	assert.False(t, m.confirming)
	assert.Nil(t, cmd)

	_, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	assert.NotNil(t, cmd, "with nothing left to dismiss it quits")
}

// Activating Quit and then Yes leaves through the popup, which is the one path
// where an action has to reach the program rather than just the model.
func TestConfirmingQuitEndsTheProgram(t *testing.T) {
	m := gallery(t)
	require.NotContains(t, m.frameOf(), "tml: ")

	quit := control(t, m, "quit")
	press(m, quit.Rect.X+1, quit.Rect.Y+1)
	require.True(t, m.confirming)
	require.NotContains(t, m.frameOf(), "tml: ")

	yes := control(t, m, "yes")
	_, cmd := press(m, yes.Rect.X+1, yes.Rect.Y+1)
	assert.NotNil(t, cmd)
}

// The log is a scrolling region, not a tab stop, so the wheel over it is the
// only way to reach it -- which is the whole reason the pointer resolves against
// more than the focus ring.
func TestTheWheelScrollsTheLog(t *testing.T) {
	m := gallery(t)
	m.tab = "data"
	require.NotContains(t, m.frameOf(), "tml: ")

	log := control(t, m, "log")
	m.Update(tea.MouseWheelMsg{X: log.Rect.X + 1, Y: log.Rect.Y + 1, Button: tea.MouseWheelDown})

	assert.Equal(t, 1, m.offset)
	assert.NotContains(t, ansi.Strip(m.frameOf()), "step 1 finished", "the first line has scrolled off")
}

// A list is one control to the ring, so clicking a row is the host reading the
// event's own coordinates. This is the case the coordinates exist for.
func TestClickingAListRowSelectsIt(t *testing.T) {
	m := gallery(t)
	m.tab = "data"
	require.NotContains(t, m.frameOf(), "tml: ")

	list := control(t, m, "services")
	press(m, list.Rect.X+2, list.Rect.Y+3)

	assert.Equal(t, 3, m.selected)
	assert.Contains(t, ansi.Strip(m.frameOf()), "> scheduler", "the cursor moved to the row that was clicked")
}

// press clicks and releases over one point, which is what actually activates a
// control: a press alone is only the start of a click that can still be taken
// back by releasing somewhere else.
func press(m *model, x, y int) (tea.Model, tea.Cmd) {
	m.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
	return m.Update(tea.MouseReleaseMsg{X: x, Y: y, Button: tea.MouseLeft})
}

func control(t *testing.T, m *model, id string) tml.Target {
	t.Helper()
	for _, target := range m.view.UI().Targets() {
		if target.ID == id {
			return target
		}
	}
	t.Fatalf("no control with id %q on this frame", id)
	return tml.Target{}
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
