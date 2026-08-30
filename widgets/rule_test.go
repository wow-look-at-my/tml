package widgets

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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

// A title wider than the space must not produce a negative run of characters, which is what a naive subtraction does
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
