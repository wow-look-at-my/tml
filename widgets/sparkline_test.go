package widgets

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSparklinePlotsAgainstItsLargestValue(t *testing.T) {
	line := build(t, "Sparkline", map[string]string{"values": "0,2,4,8"})

	w, h := line.Measure(0, 0)
	assert.Equal(t, 4, w)
	assert.Equal(t, 1, h)
	assert.Equal(t, "▁▂▄█", line.Render(4, 1))
}

func TestSparklineScalesAgainstADeclaredMaximum(t *testing.T) {
	line := build(t, "Sparkline", map[string]string{"values": "0,4", "max": "8"})

	assert.Equal(t, "▁▄", line.Render(2, 1))
}

// A series longer than the space keeps its most recent points, because that is the end of a series anybody is reading.
func TestSparklineKeepsTheMostRecentPoints(t *testing.T) {
	line := build(t, "Sparkline", map[string]string{"values": "8,8,0,8"})

	assert.Equal(t, "▁█", line.Render(2, 1))
}

func TestSparklineRejectsAValueThatIsNotANumber(t *testing.T) {
	_, err := tryBuild("Sparkline", map[string]string{"values": "1,two"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"two" is not a number`)
}
