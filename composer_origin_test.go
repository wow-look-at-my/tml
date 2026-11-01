package tml_test

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml"
)

// cellAt is the character drawn at column x of row y.
func cellAt(frame string, x, y int) string {
	rows := strings.Split(ansi.Strip(frame), "\n")
	if y < 0 || y >= len(rows) {
		return ""
	}
	cells := []rune(rows[y])
	if x < 0 || x >= len(cells) {
		return ""
	}
	return string(cells[x])
}

// A control inside a frame is published where it is drawn: the frame's border and padding come before it.
func TestComposerChildrenArePublishedWhereTheyAreDrawn(t *testing.T) {
	cases := []struct {
		name     string
		template string
	}{
		{"border", `<Border><Button id="go" label="Go"/></Border>`},
		{"border with padding", `<Border pad="2"><Button id="go" label="Go"/></Border>`},
		{"nested borders", `<Border><Stack><Text>head</Text><Border pad="0"><Button id="go" label="Go"/></Border></Stack></Border>`},
		{"popup", `<Canvas><Popup title="Ask"><Button id="go" label="Go"/></Popup></Canvas>`},
		{"scrollbox in a border", `<Border><Scrollbox height="4"><Stack><Text>a</Text><Button id="go" label="Go"/></Stack></Scrollbox></Border>`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			loaded, err := view(t, app(c.template), tml.Options{})
			require.NoError(t, err)
			frame, err := loaded.Render(nil, 30, 10)
			require.NoError(t, err)

			target, ok := loaded.UI().Target("go")
			require.True(t, ok)
			corner := cellAt(frame, target.Rect.X, target.Rect.Y)
			assert.Contains(t, "╭╔", corner, "the published rect %+v starts at %q, not at the button's corner:\n%s",
				target.Rect, corner, ansi.Strip(frame))
		})
	}
}
