package widgets

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml/cell"
	"github.com/wow-look-at-my/tml/widget/widgettest"
)

func TestSparklinePlotsAgainstItsLargestValue(t *testing.T) {
	line := build(t, "Sparkline", map[string]string{"values": "0,2,4,8"})

	w, h := line.Measure(0, 0)
	assert.Equal(t, 4, w)
	assert.Equal(t, 1, h)
	assert.Equal(t, "▁▂▄█", widgettest.Text(line, 4, 1))
}

func TestSparklineScalesAgainstADeclaredMaximum(t *testing.T) {
	line := build(t, "Sparkline", map[string]string{"values": "0,4", "max": "8"})

	assert.Equal(t, "▁▄", widgettest.Text(line, 2, 1))
}

// A series longer than the space keeps its most recent points, because that is the end of a series anybody is reading.
func TestSparklineKeepsTheMostRecentPoints(t *testing.T) {
	line := build(t, "Sparkline", map[string]string{"values": "8,8,0,8"})

	assert.Equal(t, "▁█", widgettest.Text(line, 2, 1))
}

// A graph given more than one row fills whole cells from the bottom, and the partial block is the top of a bar.
func TestSparklineFillsEveryRowItIsGiven(t *testing.T) {
	line := build(t, "Sparkline", map[string]string{"values": "0,5,9,15", "max": "15"})

	assert.Equal(t, "  ▂█\n▁▆██", widgettest.Text(line, 4, 2))
	assert.Equal(t, "   █\n  ▆█\n▁███", widgettest.Text(line, 4, 3))
	assert.Equal(t, "▁▃▅█", widgettest.Text(line, 4, 1), "one row draws as it always did")
}

func TestSparklineTakesAColour(t *testing.T) {
	line := build(t, "Sparkline", map[string]string{"values": "1,2", "color": ref("accent")})

	assert.Equal(t, cell.RGB(255, 0, 0), widgettest.Draw(line, 0, 0).At(1, 0).Style.Fg)
}

func TestSparklineRejectsAValueThatIsNotANumber(t *testing.T) {
	_, err := tryBuild("Sparkline", map[string]string{"values": "1,two"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"two" is not a number`)
}
