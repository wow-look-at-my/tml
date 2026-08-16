package widgets

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProgressBarFillsInProportion(t *testing.T) {
	bar := build(t, "ProgressBar", map[string]string{"value": "0.25"})

	assert.Equal(t, "██░░░░░░", bar.Render(8, 1))
}

func TestProgressBarTakesAMaximumAndAPercentLabel(t *testing.T) {
	bar := build(t, "ProgressBar", map[string]string{"value": "5", "max": "10", "percent": "true"})

	w, _ := bar.Measure(13, 1)
	assert.Equal(t, 13, w)
	assert.Equal(t, "████░░░░  50%", bar.Render(13, 1))
}

// A value outside the range is the host's arithmetic being wrong. Clamping keeps
// the bar inside its own track rather than letting it corrupt the row it sits in.
func TestProgressBarClampsOutOfRangeValues(t *testing.T) {
	over := build(t, "ProgressBar", map[string]string{"value": "9"})
	assert.Equal(t, "████", over.Render(4, 1))

	under := build(t, "ProgressBar", map[string]string{"value": "-3"})
	assert.Equal(t, "░░░░", under.Render(4, 1))
}

func TestProgressBarRejectsAnImpossibleMaximum(t *testing.T) {
	_, err := tryBuild("ProgressBar", map[string]string{"max": "0"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "max must be greater than zero")
}
