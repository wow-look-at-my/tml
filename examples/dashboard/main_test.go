package main

import (
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml"
	"github.com/wow-look-at-my/tml/cell"
	"github.com/wow-look-at-my/tml/input"
	"github.com/wow-look-at-my/tml/inspect"
)

// The size the binary's -frame draws at.
const width, height = 60, 18

// The document renders, and a failure to render is a failing test. frame() turns a render error into the string it
func TestTheDashboardRenders(t *testing.T) {
	m, err := newModel()
	require.NoError(t, err)

	out := m.frame(width, height)
	require.False(t, strings.HasPrefix(out, "tml: "), "the document did not render: %s", out)
	assert.Contains(t, out, "Deployments")
	assert.Contains(t, out, "Filter")
}

// Typing into the search box filters the services, and escape quits.
func TestTypingFiltersTheServices(t *testing.T) {
	m, err := newModel()
	require.NoError(t, err)
	require.NotContains(t, m.frame(width, height), "tml: ")
	require.True(t, m.view.UI().Focus("search"))

	p, err := tml.NewProgram(m.view, m, tml.ProgramOptions{NoInput: true, Output: io.Discard, Width: width, Height: height})
	require.NoError(t, err)
	send := func(msg tml.Msg) bool {
		consumed := m.Update(p, msg)
		if !consumed {
			for _, event := range m.view.UI().Update(msg) {
				m.Update(p, event)
			}
		}
		require.NotContains(t, m.frame(width, height), "tml: ")
		return consumed
	}

	for _, r := range "wor" {
		assert.False(t, send(input.ParseKey(string(r))))
	}
	assert.Equal(t, "wor", m.query)
	assert.Equal(t, 3, m.cursor)

	out := cell.Strip(m.frame(width, height))
	assert.Contains(t, out, "- worker")
	assert.NotContains(t, out, "- web")

	assert.True(t, send(input.ParseKey("esc")), "escape is the dashboard's own key")
}

// Every region names itself, so the inspector can be asked about a region rather than handed the whole tree. An element
func TestTheDashboardNamesItsElements(t *testing.T) {
	m, err := newModel()
	require.NoError(t, err)

	// The box tree, which is what the inspector reads. UI().Target answers a narrower question -- the focus and pointer
	box, err := m.view.Layout(m.Props(), width, height)
	require.NoError(t, err)

	for _, id := range []string{"app", "title", "filter", "search", "services", "hint"} {
		assert.NotNil(t, inspect.Find(box, id), "the frame declares %q", id)
	}
}
