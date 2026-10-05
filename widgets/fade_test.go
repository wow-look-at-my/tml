package widgets

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The band steps down from the content: the row nearest it keeps most of its color, the row at the edge is mostly
// background.
func TestFadePercentStepsWithTheBand(t *testing.T) {
	assert.Equal(t, 55, fadePercent(1, 1))
	assert.Equal(t, 35, fadePercent(1, 2))
	assert.Equal(t, 70, fadePercent(2, 2))
}

// Every spelling of a foreground is resolved to channels first. The fade adjusts a 256-color or a named color the
// same way it adjusts an RGB one.
func TestFadeParamsResolvesEveryForegroundSpelling(t *testing.T) {
	tests := []struct {
		name   string
		params []int
		color  shade
	}{
		{"shade", []int{38, 2, 10, 20, 30}, shade{10, 20, 30}},
		{"256 cube", []int{38, 5, 196}, shade{255, 0, 0}},
		{"256 grey", []int{38, 5, 236}, shade{48, 48, 48}},
		{"named", []int{31}, shade{205, 0, 0}},
		{"bright named", []int{91}, shade{255, 0, 0}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			kept, c, stated := fadeParams(tc.params, true)
			assert.True(t, stated)
			assert.Equal(t, tc.color, c)
			assert.Empty(t, kept)
		})
	}
}

// A parameter list that turns on an attribute and names no color leaves the color in effect alone.
func TestFadeParamsKeepsWhatIsNotForeground(t *testing.T) {
	kept, _, stated := fadeParams([]int{1, 4}, true)
	assert.Equal(t, []int{1, 4}, kept)
	assert.False(t, stated)

	kept, _, stated = fadeParams([]int{41, 38, 2, 1, 2, 3}, true)
	assert.Equal(t, []int{41}, kept, "a background survives the fade")
	assert.True(t, stated)
}

// A reset turns the foreground off, so the fade restates the approximated default in the same sequence.
func TestFadeParamsRestatesTheForegroundAfterAReset(t *testing.T) {
	kept, c, stated := fadeParams([]int{0}, true)
	assert.Equal(t, []int{0}, kept)
	assert.Equal(t, defaultForeground(true), c)
	assert.True(t, stated)
}

// The fade is written with ANSI RGB, so a row keeps its hue and loses only brightness.
func TestFadeRowBlendsTowardTheBackground(t *testing.T) {
	assert.Equal(t, "\x1b[38;2;90;90;90mx"+resetSGR, fadeRow("x", 1, 1, true))
	assert.Equal(t, "\x1b[38;2;167;167;167mx"+resetSGR, fadeRow("x", 1, 1, false))
	assert.Equal(t, "\x1b[38;2;167;167;167m\x1b[38;2;233;140;140mred"+resetSGR, fadeRow("\x1b[31mred", 1, 1, false))
}

// A row with nothing on it is left exactly as it was, because its escapes would be the whole row.
func TestFadeRowLeavesABlankRowAlone(t *testing.T) {
	assert.Equal(t, "   ", fadeRow("   ", 2, 2, true))
}
