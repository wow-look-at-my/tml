package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml/inspect"
)

// The document renders, and a failure to render is a failing test.
//
// frame() turns a render error into the string it draws, which is right for a
// user staring at a terminal and invisible to everything else: a document that
// stopped analysing shipped past a green run once already. This is the check
// that stops it happening again.
func TestTheDashboardRenders(t *testing.T) {
	m, err := newModel()
	require.NoError(t, err)

	out := m.frame()
	require.False(t, strings.HasPrefix(out, "tml: "), "the document did not render: %s", out)
	assert.Contains(t, out, "Deployments")
	assert.Contains(t, out, "Filter")
}

// Every region names itself, so the inspector can be asked about one rather
// than handed the whole tree. An element that stops naming itself is a
// question the debugger stops being able to answer.
func TestTheDashboardNamesItsElements(t *testing.T) {
	m, err := newModel()
	require.NoError(t, err)

	// The box tree, which is what the inspector reads. UI().Target answers a
	// narrower question -- the focus and pointer ring -- so a Stack or a Text
	// is absent from it and present on screen.
	box, err := m.view.Layout(m.props(), m.width, m.height)
	require.NoError(t, err)

	for _, id := range []string{"app", "title", "filter", "search", "services", "hint"} {
		assert.NotNil(t, inspect.Find(box, id), "the frame declares %q", id)
	}
}
