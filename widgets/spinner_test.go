package widgets

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml/widget"
)

// spinnerAt builds a Spinner the way the engine does, on the animation frame it is on.
func spinnerAt(t *testing.T, frame int, pairs map[string]string) widget.Native {
	t.Helper()
	factory, ok := Library().Factory("Spinner")
	require.True(t, ok, "the library ships a Spinner")
	native, err := factory.Build(widget.Context{Attrs: attrsOf("Spinner", pairs), Frame: frame})
	require.NoError(t, err)
	return native
}

// The frame is the engine's, so the widget only has to draw the one it was handed.
func TestSpinnerDrawsTheFrameTheEngineIsOn(t *testing.T) {
	assert.Equal(t, "|", spinnerAt(t, 0, map[string]string{"kind": "line"}).Render(1, 1))
	assert.Equal(t, "-", spinnerAt(t, 2, map[string]string{"kind": "line"}).Render(1, 1))
}

// A frame counter only goes up, so the walk through this kind's frames wraps here.
func TestSpinnerWrapsTheFrameCounter(t *testing.T) {
	assert.Equal(t, "/", spinnerAt(t, 5, map[string]string{"kind": "line"}).Render(1, 1))
	assert.Equal(t, "\\", spinnerAt(t, -1, map[string]string{"kind": "line"}).Render(1, 1))
}

// A Spinner is what the engine counts frames for. Animator is how a widget says so.
func TestSpinnerReportsThatItAnimates(t *testing.T) {
	animator, ok := spinnerAt(t, 0, nil).(widget.Animator)
	require.True(t, ok, "the engine animates a widget only when it says it animates")
	assert.True(t, animator.Animating())
}

func TestSpinnerRejectsAnUnknownKind(t *testing.T) {
	_, err := tryBuild("Spinner", map[string]string{"kind": "wobble"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected one of arrow, bar, circle, dot, dots, line")
}
