package tml_test

import (
	"io"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml"
)

// A one-shot render is not a program. `tml render`, a golden and a screenshot
// all paint once and exit, and none of them was ever going to be driven.
func TestOneFrameIsNotAnUndrivableProgram(t *testing.T) {
	tml.ResetDrivable()
	assert.NotPanics(t, func() {
		tml.PaintedUndrivable(time.Now().Add(-time.Hour), 1)
	}, "one frame is a render, not a running program")
}

// Inside the grace window nothing is said: a host builds its program after it
// loads its document, and this is the gap between the two.
func TestAProgramThatHasJustStartedIsLeftAlone(t *testing.T) {
	tml.ResetDrivable()
	assert.NotPanics(t, func() {
		tml.PaintedUndrivable(time.Now(), 40)
	})
}

// The hole this closes: a host that built its program with tea.NewProgram keeps
// painting, and the inspector can read every frame and drive none of them. The
// program goes down rather than staying up in that state, and the message names
// the identifier that fixes it.
func TestAProgramNothingCanDriveDoesNotKeepRunning(t *testing.T) {
	tml.ResetDrivable()
	assert.PanicsWithValue(t,
		"tml: no way to drive this program; build it with tml.NewProgram",
		func() { tml.PaintedUndrivable(time.Now().Add(-tml.DriveGrace-time.Second), 2) })
}

// A program tml built is driven from the frame it starts on, so the guard has
// nothing to say about it however long it runs. Rendering is exercised through
// the real path rather than the seam: what matters is that a driven session is
// not a case the guard can reach at all.
func TestADrivenProgramIsNeverTouched(t *testing.T) {
	tml.ResetInspection()
	tml.ResetDrivable()

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
