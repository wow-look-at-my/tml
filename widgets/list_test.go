package widgets

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wow-look-at-my/tml/cell"
	"github.com/wow-look-at-my/tml/widget"
	"github.com/wow-look-at-my/tml/widget/widgettest"
)

func TestListMarksTheSelectedItem(t *testing.T) {
	l := build(t, "List", map[string]string{"items": "one,two,three", "selected": "1"})

	assert.Equal(t, []string{"  one  ", "> two  ", "  three"}, rows(l, 7, 3))
	buf := widgettest.Draw(l, 7, 3)
	assert.True(t, buf.At(6, 1).Style.Has(cell.Bold), "the mark covers the row's whole width")
	assert.False(t, buf.At(0, 0).Style.Has(cell.Bold))
}

// The gutter is kept on every row.
func TestListKeepsItsGutterOnEveryRow(t *testing.T) {
	l := build(t, "List", map[string]string{"items": "a,b"})

	assert.Equal(t, []string{"  a", "  b"}, rows(l, 3, 2))
}

func TestListMeasuresItsWidestItem(t *testing.T) {
	l := build(t, "List", map[string]string{"items": "a,bbbb,cc"})

	w, h := l.Measure(0, 0)
	assert.Equal(t, 6, w, "four cells plus the cursor gutter")
	assert.Equal(t, 3, h)
}

func TestListTakesACustomCursor(t *testing.T) {
	l := build(t, "List", map[string]string{"items": "a,b", "selected": "0", "cursor": "*"})

	assert.Equal(t, "* a", rows(l, 3, 2)[0])
}

// An empty list has nothing to land on, so tab passes over it.
func TestEmptyListRefusesFocus(t *testing.T) {
	assert.False(t, build(t, "List", nil).(widget.Focusable).AcceptsFocus())
	assert.True(t, build(t, "List", map[string]string{"items": "a"}).(widget.Focusable).AcceptsFocus())
	assert.False(t, build(t, "List", map[string]string{"items": "a", "disabled": "true"}).(widget.Focusable).AcceptsFocus())
}

func TestFocusedListHighlightsItsSelection(t *testing.T) {
	l := build(t, "List", map[string]string{"items": "a,b", "selected": "0"})
	assert.False(t, widgettest.Draw(l, 5, 2).At(0, 0).Style.Has(cell.Reverse))

	l.(widget.Stateful).SetState(widget.State{Focused: true})
	buf := widgettest.Draw(l, 5, 2)
	assert.True(t, buf.At(0, 0).Style.Has(cell.Reverse), "the selected row is reversed")
	assert.True(t, buf.At(4, 0).Style.Has(cell.Reverse), "across the whole width")
	assert.False(t, buf.At(0, 1).Style.Has(cell.Reverse), "and no other row is")
}

func TestDisabledListIsFaint(t *testing.T) {
	l := build(t, "List", map[string]string{"items": "a", "selected": "0", "disabled": "true"})
	l.(widget.Stateful).SetState(widget.State{Focused: true})

	style := widgettest.Draw(l, 3, 1).At(2, 0).Style
	assert.True(t, style.Has(cell.Faint))
	assert.False(t, style.Has(cell.Reverse), "a disabled list shows no focus")
}

// A row wider than the space is cut, not wrapped: wrapping would push every row below it down a line, and the list's
func TestListCutsRowsTooWideForIt(t *testing.T) {
	l := build(t, "List", map[string]string{"items": "cmd/report.go,report/table.go", "selected": "1"})

	assert.Equal(t, []string{"  cmd/repor…", "> report/ta…"}, rows(l, 12, 2))
}
