package widgets

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml/sema"
	"github.com/wow-look-at-my/tml/widget"
)

// build makes one widget the way the engine does, from the element's name and
// its attributes.
func build(t *testing.T, element string, pairs map[string]string) widget.Native {
	t.Helper()
	native, err := tryBuild(element, pairs)
	require.NoError(t, err)
	return native
}

func tryBuild(element string, pairs map[string]string) (widget.Native, error) {
	factory, ok := Library().Factory(element)
	if !ok {
		return nil, assertUnknown(element)
	}
	values := map[string]sema.Value{}
	order := make([]string, 0, len(pairs))
	for name, raw := range pairs {
		values[name] = sema.StringValue(raw)
		order = append(order, name)
	}
	return factory.Build(widget.Context{Attrs: widget.NewAttrs(element, values, order)})
}

type unknownElement string

func (u unknownElement) Error() string { return "no widget named " + string(u) }

func assertUnknown(element string) error { return unknownElement(element) }

func TestLibraryBindsEveryDocumentedWidget(t *testing.T) {
	assert.Equal(t, []string{
		"Badge", "Border", "Button", "Popup", "ProgressBar",
		"Rule", "Scrollbox", "Sparkline", "Spinner",
	}, Names())
}

func TestRuleFillsItsSpace(t *testing.T) {
	rule := build(t, "Rule", nil)

	w, h := rule.Measure(10, 1)
	assert.Equal(t, 10, w)
	assert.Equal(t, 1, h)
	assert.Equal(t, "──────────", rule.Render(10, 1))
}

func TestRuleBreaksForItsTitle(t *testing.T) {
	rule := build(t, "Rule", map[string]string{"title": "Logs", "char": "="})

	assert.Equal(t, "= Logs =====", rule.Render(12, 1))
}

// A title wider than the space must not produce a negative run of characters,
// which is what a naive subtraction does at the moment the box gets small.
func TestRuleSurvivesTooLittleSpace(t *testing.T) {
	rule := build(t, "Rule", map[string]string{"title": "Logs"})

	assert.Equal(t, " Logs ", rule.Render(3, 1))
}

func TestVerticalRuleIsOneColumn(t *testing.T) {
	rule := build(t, "Rule", map[string]string{"orientation": "vertical"})

	w, h := rule.Measure(10, 3)
	assert.Equal(t, 1, w)
	assert.Equal(t, 3, h)
	assert.Equal(t, "│\n│\n│", rule.Render(1, 3))
}

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

func TestSpinnerShowsTheRequestedFrame(t *testing.T) {
	assert.Equal(t, "|", build(t, "Spinner", map[string]string{"kind": "line", "frame": "0"}).Render(1, 1))
	assert.Equal(t, "-", build(t, "Spinner", map[string]string{"kind": "line", "frame": "2"}).Render(1, 1))
}

// A frame counter is a tick count that only goes up, so it has to wrap here
// rather than making every caller remember the modulo.
func TestSpinnerWrapsTheFrameCounter(t *testing.T) {
	assert.Equal(t, "/", build(t, "Spinner", map[string]string{"kind": "line", "frame": "5"}).Render(1, 1))
	assert.Equal(t, "\\", build(t, "Spinner", map[string]string{"kind": "line", "frame": "-1"}).Render(1, 1))
}

func TestSpinnerRejectsAnUnknownKind(t *testing.T) {
	_, err := tryBuild("Spinner", map[string]string{"kind": "wobble"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected one of arrow, bar, circle, dot, dots, line")
}

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

// A series longer than the space keeps its most recent points, because that is
// the end of a series anybody is reading.
func TestSparklineKeepsTheMostRecentPoints(t *testing.T) {
	line := build(t, "Sparkline", map[string]string{"values": "8,8,0,8"})

	assert.Equal(t, "▁█", line.Render(2, 1))
}

func TestSparklineRejectsAValueThatIsNotANumber(t *testing.T) {
	_, err := tryBuild("Sparkline", map[string]string{"values": "1,two"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"two" is not a number`)
}

func TestBadgePadsItsLabel(t *testing.T) {
	badge := build(t, "Badge", map[string]string{"label": "new"})

	w, h := badge.Measure(0, 0)
	assert.Equal(t, 5, w)
	assert.Equal(t, 1, h)
	assert.Equal(t, " new ", badge.Render(5, 1))
}
