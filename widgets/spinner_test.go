package widgets

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSpinnerShowsTheRequestedFrame(t *testing.T) {
	assert.Equal(t, "|", build(t, "Spinner", map[string]string{"kind": "line", "frame": "0"}).Render(1, 1))
	assert.Equal(t, "-", build(t, "Spinner", map[string]string{"kind": "line", "frame": "2"}).Render(1, 1))
}

// A frame counter is a tick count that only goes up, so it has to wrap here rather than making every caller remember
func TestSpinnerWrapsTheFrameCounter(t *testing.T) {
	assert.Equal(t, "/", build(t, "Spinner", map[string]string{"kind": "line", "frame": "5"}).Render(1, 1))
	assert.Equal(t, "\\", build(t, "Spinner", map[string]string{"kind": "line", "frame": "-1"}).Render(1, 1))
}

func TestSpinnerRejectsAnUnknownKind(t *testing.T) {
	_, err := tryBuild("Spinner", map[string]string{"kind": "wobble"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected one of arrow, bar, circle, dot, dots, line")
}
