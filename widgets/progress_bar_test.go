package widgets

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml/cell"
	"github.com/wow-look-at-my/tml/widget/widgettest"
)

func TestProgressBarFillsInProportion(t *testing.T) {
	bar := build(t, "ProgressBar", map[string]string{"value": "0.25"})

	assert.Equal(t, "██░░░░░░", widgettest.Text(bar, 8, 1))
}

func TestProgressBarTakesAMaximumAndAPercentLabel(t *testing.T) {
	bar := build(t, "ProgressBar", map[string]string{"value": "5", "max": "10", "percent": "true"})

	w, _ := bar.Measure(13, 1)
	assert.Equal(t, 13, w)
	assert.Equal(t, "████░░░░  50%", widgettest.Text(bar, 13, 1))
}

// A value outside the range is the host's arithmetic being wrong.
func TestProgressBarClampsOutOfRangeValues(t *testing.T) {
	over := build(t, "ProgressBar", map[string]string{"value": "9"})
	assert.Equal(t, "████", widgettest.Text(over, 4, 1))

	under := build(t, "ProgressBar", map[string]string{"value": "-3"})
	assert.Equal(t, "░░░░", widgettest.Text(under, 4, 1))
}

func TestProgressBarColoursItsFillAndItsTrack(t *testing.T) {
	bar := build(t, "ProgressBar", map[string]string{"value": "0.5", "color": "2", "trackColor": "#102030"})
	buf := widgettest.Draw(bar, 4, 1)

	assert.Equal(t, cell.Indexed(2), buf.At(0, 0).Style.Fg)
	assert.Equal(t, cell.RGB(0x10, 0x20, 0x30), buf.At(3, 0).Style.Fg)
}

// A colour that does not parse is a mistake in the document, so the build fails at the attribute.
func TestProgressBarRejectsAColourThatIsNotAColour(t *testing.T) {
	_, err := tryBuild("ProgressBar", map[string]string{"color": "nope"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nope")

	_, err = tryBuild("ProgressBar", map[string]string{"trackColor": "#12"})
	require.Error(t, err)
}

func TestProgressBarRejectsAnImpossibleMaximum(t *testing.T) {
	_, err := tryBuild("ProgressBar", map[string]string{"max": "0"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "max must be greater than zero")
}
