package tml_test

import (
	"io"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml"
)

// The failure names the fix. A guard that takes a program down and leaves the reader to guess why has traded a real fix for a puzzle
func TestTheRefusalNamesTheFix(t *testing.T) {
	exclusive(t)
	assert.PanicsWithValue(t,
		"tml: no way to drive this program; build it with tml.NewProgram",
		func() { tml.CheckDrivable() })
}

// A program tml built is driven from the frame it starts on, so the guard has nothing to say about it however long it
func TestADrivenProgramIsNeverTouched(t *testing.T) {
	exclusive(t)

	view := loadInspectorView(t)
	_, err := tml.NewProgram(driveModel{}, tea.WithInput(strings.NewReader("")), tea.WithOutput(io.Discard))
	require.NoError(t, err)

	assert.NotPanics(t, func() {
		for range 3 {
			_, err := view.Render(nil, 40, 10)
			require.NoError(t, err)
		}
	})
}
