package main

import (
	"flag"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml"
	"github.com/wow-look-at-my/tml/cell"
	"github.com/wow-look-at-my/tml/input"
)

var update = flag.Bool("update", false, "rewrite the golden files")

// gallery builds the model the binary builds, with the terminal pinned to a profile that can only do colour.
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
	m.query, m.cursor = "sched", len("sched")
	m.progress = 62
	return m
}

// program is a program around the model. A test steps the loop itself with send, and runs the program only to see
// whether a quit is queued.
func program(t *testing.T, m *model) *tml.Program {
	t.Helper()
	p, err := tml.NewProgram(m.view, m, tml.ProgramOptions{NoInput: true, Output: io.Discard, Width: m.width, Height: m.height})
	require.NoError(t, err)
	return p
}

// send does what the program's loop does with one message. The model sees it first, the focus ring gets what the
// model leaves, and the frame is painted again. It reports whether the model consumed the message.
func send(t *testing.T, m *model, p *tml.Program, msg tml.Msg) bool {
	t.Helper()
	consumed := m.Update(p, msg)
	if !consumed {
		for _, event := range m.view.UI().Update(msg) {
			m.Update(p, event)
		}
	}
	require.NotContains(t, m.frameOf(), "tml: ", "the view failed to render")
	return consumed
}

// quits runs the program and reports whether it ends by itself, which it does only when a quit is already queued. A
// program that keeps running is told to quit, so the test does not leak it.
func quits(t *testing.T, p *tml.Program) bool {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- p.Run() }()
	select {
	case err := <-done:
		require.NoError(t, err)
		return true
	case <-time.After(time.Second):
		p.Quit()
		require.NoError(t, <-done)
		return false
	}
}

// Every widget the library ships is in some of these sections, so the goldens are the library's own regression
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
			golden(t, file, cell.Strip(out))
		})
	}
}

// The gallery routes every control through its action string.
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

// The check above is only worth anything if act can say no.
func TestAnUnknownActionIsRejected(t *testing.T) {
	m := gallery(t)

	assert.False(t, m.act("nonsense"))
	assert.True(t, m.act("save"))
}

// Clicking a tab is the whole loop: the click lands on a rect the last frame published.
func TestClickingATabSwitchesSection(t *testing.T) {
	m := gallery(t)
	require.NotContains(t, m.frameOf(), "tml: ")

	target := control(t, m, "tab-media")
	press(t, m, program(t, m), target.Rect.X+1, target.Rect.Y+1)

	assert.Equal(t, "media", m.tab)
	assert.Contains(t, cell.Strip(m.frameOf()), "Media ─", "the media section is the one drawn")
}

// Typing belongs to the field while the field has focus, so the ring's own keys and the quit key both have to stand
func TestTypingGoesToTheFocusedField(t *testing.T) {
	m := gallery(t)
	require.NotContains(t, m.frameOf(), "tml: ")
	require.True(t, m.view.UI().Focus("search"))
	p := program(t, m)

	for _, r := range "qq" {
		assert.False(t, send(t, m, p, input.ParseKey(string(r))), "q is a letter while the field has the keyboard")
	}
	assert.Equal(t, "schedqq", m.query)
	assert.Equal(t, len("schedqq"), m.cursor)

	send(t, m, p, input.ParseKey("backspace"))
	assert.Equal(t, "schedq", m.query)
	assert.False(t, quits(t, p), "typing q into the field does not quit")
}

func TestQuitKeysOnlyBindOutsideAField(t *testing.T) {
	m := gallery(t)
	require.NotContains(t, m.frameOf(), "tml: ")
	require.True(t, m.view.UI().Focus("save"))

	p := program(t, m)
	assert.True(t, send(t, m, p, input.ParseKey("q")))
	assert.True(t, quits(t, p), "q quits when the keyboard is not in a field")
}

// Escape backs out of the popup before it backs out of the program: a modal that quits the whole thing on the key
func TestEscapeClosesThePopupFirst(t *testing.T) {
	m := gallery(t)
	m.confirming = true
	require.NotContains(t, m.frameOf(), "tml: ")

	p := program(t, m)
	assert.True(t, send(t, m, p, input.ParseKey("esc")))
	assert.False(t, m.confirming)
	assert.False(t, quits(t, p), "closing the popup does not quit")

	p = program(t, m)
	assert.True(t, send(t, m, p, input.ParseKey("esc")))
	assert.True(t, quits(t, p), "with nothing left to dismiss it quits")
}

// Activating Quit and then Yes leaves through the popup.
func TestConfirmingQuitEndsTheProgram(t *testing.T) {
	m := gallery(t)
	require.NotContains(t, m.frameOf(), "tml: ")

	p := program(t, m)
	quit := control(t, m, "quit")
	press(t, m, p, quit.Rect.X+1, quit.Rect.Y+1)
	require.True(t, m.confirming)
	assert.False(t, m.quitting, "Quit only asks")

	yes := control(t, m, "yes")
	press(t, m, p, yes.Rect.X+1, yes.Rect.Y+1)
	assert.True(t, m.quitting)
	assert.True(t, quits(t, p))
}

// The log is a scrolling region, not a tab stop.
func TestTheWheelScrollsTheLog(t *testing.T) {
	m := gallery(t)
	m.tab = "data"
	require.NotContains(t, m.frameOf(), "tml: ")

	log := control(t, m, "log")
	send(t, m, program(t, m), input.Mouse{X: log.Rect.X + 1, Y: log.Rect.Y + 1, Button: input.WheelDown, Action: input.Wheel})

	assert.Equal(t, 1, m.offset)
	assert.NotContains(t, cell.Strip(m.frameOf()), "step 1 finished", "the first line has scrolled off")
}

// A list is a single control to the ring, so clicking a row is the host reading the event's own coordinates.
func TestClickingAListRowSelectsIt(t *testing.T) {
	m := gallery(t)
	m.tab = "data"
	require.NotContains(t, m.frameOf(), "tml: ")

	list := control(t, m, "services")
	press(t, m, program(t, m), list.Rect.X+2, list.Rect.Y+3)

	assert.Equal(t, 3, m.selected)
	assert.Contains(t, cell.Strip(m.frameOf()), "> scheduler", "the cursor moved to the row that was clicked")
}

// press clicks and releases over a single point, which is what activates a
// control.
func press(t *testing.T, m *model, p *tml.Program, x, y int) {
	t.Helper()
	send(t, m, p, input.Mouse{X: x, Y: y, Button: input.ButtonLeft, Action: input.Press})
	send(t, m, p, input.Mouse{X: x, Y: y, Button: input.ButtonLeft, Action: input.Release})
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

// readGolden returns the recorded frame. An empty golden is seeded from this run and then fails: seeding silently
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
