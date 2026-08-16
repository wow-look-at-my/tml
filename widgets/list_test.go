package widgets

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"

	"github.com/wow-look-at-my/tml/widget"
)

// plain drops the styling, for a test about what the text says rather than about
// how it is decorated.
func plain(s string) []string {
	return strings.Split(ansi.Strip(s), "\n")
}

func TestListMarksTheSelectedItem(t *testing.T) {
	l := build(t, "List", map[string]string{"items": "one,two,three", "selected": "1"})

	assert.Equal(t, []string{"  one  ", "> two  ", "  three"}, plain(l.Render(7, 3)))
}

// The gutter is kept on every row, or the text shifts sideways as the selection
// moves and the whole list appears to jitter.
func TestListKeepsItsGutterOnEveryRow(t *testing.T) {
	l := build(t, "List", map[string]string{"items": "a,b"})

	assert.Equal(t, []string{"  a", "  b"}, split(l.Render(3, 2)))
}

func TestListMeasuresItsWidestItem(t *testing.T) {
	l := build(t, "List", map[string]string{"items": "a,bbbb,cc"})

	w, h := l.Measure(0, 0)
	assert.Equal(t, 6, w, "four cells plus the cursor gutter")
	assert.Equal(t, 3, h)
}

func TestListTakesACustomCursor(t *testing.T) {
	l := build(t, "List", map[string]string{"items": "a,b", "selected": "0", "cursor": "*"})

	assert.Equal(t, "* a", plain(l.Render(3, 2))[0])
}

// An empty list has nothing to land on, so tab passes over it.
func TestEmptyListRefusesFocus(t *testing.T) {
	assert.False(t, build(t, "List", nil).(widget.Focusable).AcceptsFocus())
	assert.True(t, build(t, "List", map[string]string{"items": "a"}).(widget.Focusable).AcceptsFocus())
	assert.False(t, build(t, "List", map[string]string{"items": "a", "disabled": "true"}).(widget.Focusable).AcceptsFocus())
}

func TestFocusedListHighlightsItsSelection(t *testing.T) {
	l := build(t, "List", map[string]string{"items": "a,b", "selected": "0"})
	resting := l.Render(5, 2)

	l.(widget.Stateful).SetState(widget.State{Focused: true})
	assert.NotEqual(t, resting, l.Render(5, 2))
}

// A row wider than the space is cut, not wrapped: wrapping would push every row
// below it down a line, and the list's geometry would stop matching the screen.
func TestListCutsRowsTooWideForIt(t *testing.T) {
	l := build(t, "List", map[string]string{"items": "cmd/report.go,report/table.go", "selected": "1"})

	assert.Equal(t, []string{"  cmd/repor…", "> report/ta…"}, plain(l.Render(12, 2)))
}
