package widgets

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wow-look-at-my/tml/cell"
	"github.com/wow-look-at-my/tml/widget/widgettest"
)

func TestRuleFillsItsSpace(t *testing.T) {
	rule := build(t, "Rule", nil)

	w, h := rule.Measure(10, 1)
	assert.Equal(t, 10, w)
	assert.Equal(t, 1, h)
	assert.Equal(t, "──────────", widgettest.Text(rule, 10, 1))
}

func TestRuleBreaksForItsTitle(t *testing.T) {
	rule := build(t, "Rule", map[string]string{"title": "Logs", "char": "="})

	assert.Equal(t, "= Logs =====", widgettest.Text(rule, 12, 1))
}

// A title wider than the space must not produce a negative run of characters, which is what a naive subtraction does.
// The title then starts at the left edge and the canvas cuts it.
func TestRuleSurvivesTooLittleSpace(t *testing.T) {
	rule := build(t, "Rule", map[string]string{"title": "Logs"})

	assert.Equal(t, " Lo", widgettest.Text(rule, 3, 1))
	w, _ := rule.Measure(3, 1)
	assert.Equal(t, 6, w, "the rule asks for room for its title")
}

func TestRuleColoursItsLineButNotItsTitle(t *testing.T) {
	buf := widgettest.Draw(build(t, "Rule", map[string]string{"title": "A", "color": "red"}), 6, 1)

	assert.Equal(t, cell.MustColor("red"), buf.At(0, 0).Style.Fg)
	assert.False(t, buf.At(2, 0).Style.Fg.IsSet(), "the title is drawn plain")
	assert.Equal(t, cell.MustColor("red"), buf.At(5, 0).Style.Fg)
}

func TestVerticalRuleIsOneColumn(t *testing.T) {
	rule := build(t, "Rule", map[string]string{"orientation": "vertical"})

	w, h := rule.Measure(10, 3)
	assert.Equal(t, 1, w)
	assert.Equal(t, 3, h)
	assert.Equal(t, "│\n│\n│", widgettest.Text(rule, 1, 3))
}
