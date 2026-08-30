package main

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var update = flag.Bool("update", false, "rewrite the golden files")

func agent(t *testing.T) *model {
	t.Helper()
	m, err := newModel()
	require.NoError(t, err)
	m.width, m.height = 96, 30
	return m
}

// run steps the script the way pressing space does.
func run(t *testing.T, m *model, steps int) {
	t.Helper()
	for range steps {
		m.step()
	}
	require.NotContains(t, m.render(), "tml: ", "the view failed to render")
}

// The several frames worth pinning: a session mid-flight, the permission prompt that interrupts it, and what is left
func TestTheSessionRenders(t *testing.T) {
	for _, state := range []struct {
		name  string
		steps int
		after func(*model)
	}{
		{name: "session", steps: 5},
		{name: "permission", steps: 6},
		{name: "denied", steps: 6, after: func(m *model) { m.answer(false) }},
		{name: "finished", steps: 6, after: func(m *model) { m.answer(true); m.step() }},
	} {
		t.Run(state.name, func(t *testing.T) {
			m := agent(t)
			run(t, m, state.steps)
			if state.after != nil {
				state.after(m)
			}
			golden(t, state.name, ansi.Strip(m.render()))
		})
	}
}

// The script is the model here, so the permission beat has to actually stop it: a harness that ran the command and
func TestThePermissionBeatStopsTheScript(t *testing.T) {
	m := agent(t)
	run(t, m, 6)

	require.True(t, m.asking)
	at := m.at
	m.step()
	assert.Equal(t, at, m.at, "the script does not move while it is waiting for an answer")
	assert.NotContains(t, ansi.Strip(m.render()), "bash go test", "no test run before the answer")

	m.answer(true)
	assert.False(t, m.asking)
	assert.Contains(t, ansi.Strip(m.render()), "bash go test ./...")
}

// Denying stops the session rather than carrying on without the thing that was refused, and says so where the user is
func TestDenyingStopsTheSession(t *testing.T) {
	m := agent(t)
	run(t, m, 6)
	m.answer(false)

	frame := ansi.Strip(m.render())
	assert.Contains(t, frame, "denied")
	assert.Contains(t, frame, "stopped")
	assert.NotContains(t, frame, "package")
	assert.Equal(t, len(script), m.at, "there is nothing left to step")
}

// The permission popup is a modal, so escape has to dismiss it rather than quitting the whole program out from under
func TestEscapeAnswersTheQuestionRatherThanQuitting(t *testing.T) {
	m := agent(t)
	run(t, m, 6)

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	assert.Nil(t, cmd)
	assert.False(t, m.asking)
	assert.True(t, m.denied)
}

// Typing at the prompt reaches the prompt, and sending it puts the text in the transcript -- with a note, because
func TestTypingReachesThePromptAndSending(t *testing.T) {
	m := agent(t)
	require.NotContains(t, m.render(), "tml: ")
	require.True(t, m.view.UI().Focus("prompt"))

	for _, r := range "hi" {
		m.Update(tea.KeyPressMsg{Code: r})
	}
	assert.Equal(t, "hi", m.prompt)

	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.Empty(t, m.prompt)
	assert.Contains(t, ansi.Strip(m.render()), "hi")
}

// Space steps the script, which is the whole interaction when the keyboard is not in the prompt.
func TestSpaceStepsTheScript(t *testing.T) {
	m := agent(t)
	require.NotContains(t, m.render(), "tml: ")
	require.True(t, m.view.UI().Focus("files"))

	m.Update(tea.KeyPressMsg{Code: ' '})
	assert.Equal(t, 1, m.at)
	assert.Contains(t, ansi.Strip(m.render()), "add a --json flag")
}

// The session follows the tail by asking to scroll further than there is, so the wheel has to move from where the
func TestScrollingUpLeavesTheTail(t *testing.T) {
	m := agent(t)
	run(t, m, 5)
	tailed := m.render()

	session, ok := m.view.UI().Target("session")
	require.True(t, ok)
	require.Positive(t, session.Scroll.MaxY, "there is more session than viewport")
	require.Equal(t, session.Scroll.MaxY, session.Scroll.Y, "following pins it to the end")

	for range 3 {
		m.Update(tea.MouseWheelMsg{X: session.Rect.X + 2, Y: session.Rect.Y + 2, Button: tea.MouseWheelUp})
	}
	assert.Equal(t, session.Scroll.MaxY-3, m.offset, "three notches back from where the frame stopped")
	assert.NotEqual(t, tailed, m.render())
}

// Clicking a context file selects it, which is the pointer coordinate inside a multi-row widget doing its job.
func TestClickingAContextFileSelectsIt(t *testing.T) {
	m := agent(t)
	require.NotContains(t, m.render(), "tml: ")

	var rect struct{ x, y int }
	for _, target := range m.view.UI().Targets() {
		if target.ID == "files" {
			rect.x, rect.y = target.Rect.X, target.Rect.Y
		}
	}
	m.Update(tea.MouseClickMsg{X: rect.x + 3, Y: rect.y + 2, Button: tea.MouseLeft})
	m.Update(tea.MouseReleaseMsg{X: rect.x + 3, Y: rect.y + 2, Button: tea.MouseLeft})

	assert.Equal(t, 2, m.state.file)
	assert.Contains(t, ansi.Strip(m.render()), "> report/json")
}

// Every role the script uses has to be a role the transcript knows, or the whole view is an error string instead of a
func TestEveryScriptedRoleIsDrawable(t *testing.T) {
	m := agent(t)
	for range len(script) {
		m.step()
		if m.asking {
			m.answer(true)
		}
	}
	frame := m.render()
	require.NotContains(t, frame, "tml: ")

	for _, beat := range script {
		if beat.text == "" {
			continue
		}
		first, _, _ := strings.Cut(beat.text, " ")
		assert.Contains(t, ansi.Strip(frame)+strings.Join(m.entries, "\n"), first)
	}
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
