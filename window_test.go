package tml_test

import (
	"fmt"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml"
	"github.com/wow-look-at-my/tml/sema"
)

// heights is a list whose item heights are known up front.
type heights []int

func (h heights) Len() int         { return len(h) }
func (h heights) Height(i int) int { return h[i] }

func TestAWindowFollowsTheEndOfItsList(t *testing.T) {
	w := tml.NewWindow()
	s := w.Slice(heights{3, 3, 3}, 4)

	assert.Equal(t, tml.Slice{First: 1, Last: 2, Offset: 2, Above: true}, s, "the newest row is the bottom row")
	assert.True(t, w.Following())
}

// Scrolling stops at the first screenful, as it stops at the last. A window that ended on the oldest row would draw it
// alone above blank rows.
func TestAWindowScrolledToTheTopIsFull(t *testing.T) {
	list := heights{3, 3, 3}
	w := tml.NewWindow()
	w.Scroll(-100, list, 4)

	assert.Equal(t, tml.Slice{First: 0, Last: 1, Offset: 0, Below: true}, w.Slice(list, 4))
	assert.False(t, w.Following())

	w.Scroll(1, list, 4)
	assert.Equal(t, 1, w.Slice(list, 4).Offset, "the first scroll down moves the view")

	w.Scroll(100, list, 4)
	assert.True(t, w.Following(), "the newest row follows the list again")
}

// An item that draws nothing is stepped over. A position left on one would have nowhere to draw from.
func TestAWindowStepsOverItemsThatDrawNothing(t *testing.T) {
	list := heights{3, 0, 0, 3}
	w := tml.NewWindow()
	w.Scroll(-100, list, 2)
	for range 10 {
		s := w.Slice(list, 2)
		assert.NotContains(t, []int{1, 2}, s.Last, "the bottom row is never on an empty item")
		w.Scroll(1, list, 2)
	}
	assert.True(t, w.Following(), "single-row steps reach the newest row")
}

// A list shorter than the window is shown whole from the top, and there is nowhere to scroll.
func TestAShortListIsShownWhole(t *testing.T) {
	list := heights{1, 1}
	w := tml.NewWindow()
	w.Scroll(-5, list, 10)

	assert.Equal(t, tml.Slice{First: 0, Last: 1}, w.Slice(list, 10))
	assert.True(t, w.Following())
}

func TestAWindowScrolledAwayKeepsItsPlaceAsItemsArrive(t *testing.T) {
	w := tml.NewWindow()
	w.Scroll(-3, heights{3, 3, 3}, 4)
	before := w.Slice(heights{3, 3, 3}, 4)

	assert.Equal(t, before, w.Slice(heights{3, 3, 3, 3}, 4), "an arriving item does not move a reader")
	assert.True(t, w.Slice(heights{3, 3, 3, 3}, 4).Below)
}

func TestAnEmptyListHasNothingOnScreen(t *testing.T) {
	w := tml.NewWindow()
	w.Scroll(-1, heights{0, 0}, 4)
	assert.Equal(t, tml.Slice{First: -1, Last: -1}, w.Slice(heights{0, 0}, 4))
}

const windowedList = `<?xml version="1.1" encoding="UTF-8"?>
<Component xmlns="urn:tml:v1" name="List">
	<DataTemplate name="Row">
		<Property name="title" type="string" default=""/>
		<Property name="lines" type="string[]" default=""/>
		<Template>
			<Stack width="*">
				<Text>{title}</Text>
				<Scrollbox scrollbar="never">
					<Stack width="*">
						<For each="{lines}" as="line">
							<Text>{line}</Text>
						</For>
					</Stack>
				</Scrollbox>
			</Stack>
		</Template>
	</DataTemplate>

	<Property name="rows" type="record[]" default=""/>
	<Property name="offset" type="int" default="0"/>
	<Template>
		<Scrollbox scrollbar="never" offset="{offset}">
			<Stack itemsSource="{rows}" itemTemplate="Row" width="*"/>
		</Scrollbox>
	</Template>
</Component>
`

func row(i int) map[string]sema.Value {
	return map[string]sema.Value{
		"title": sema.StringValue(fmt.Sprintf("title %d", i)),
		"lines": sema.ListValue([]string{fmt.Sprintf("first %d", i), "", fmt.Sprintf("last %d", i)}),
	}
}

// The height of an item is the rows the engine draws for it: a blank line is a row, and a nested scrolling region is as
// tall as its content.
func TestItemHeightCountsTheRowsTheEngineDraws(t *testing.T) {
	view, err := tml.Load(fstest.MapFS{"list.tml": &fstest.MapFile{Data: []byte(windowedList)}}, "list.tml", tml.Options{})
	require.NoError(t, err)

	h, err := view.ItemHeight("Row", sema.RecordValue(row(0)), 20)
	require.NoError(t, err)
	assert.Equal(t, 4, h, "a title and three lines, one of them blank")

	_, err = view.ItemHeight("Nope", sema.RecordValue(row(0)), 20)
	assert.ErrorContains(t, err, `no data template is named "Nope"`)
}

// measured is a list whose heights are the engine's own.
type measured struct {
	t    *testing.T
	view *tml.View
	rows []map[string]sema.Value
}

func (m measured) Len() int { return len(m.rows) }
func (m measured) Height(i int) int {
	h, err := m.view.ItemHeight("Row", sema.RecordValue(m.rows[i]), 20)
	require.NoError(m.t, err)
	return h
}

// drawWindow renders the items a window shows, as a host does.
func drawWindow(t *testing.T, list measured, s tml.Slice, height int) []string {
	t.Helper()
	out, err := list.view.Render(tml.Props{
		"rows":   sema.RecordListValue(list.rows[s.First : s.Last+1]),
		"offset": sema.StringValue(fmt.Sprint(s.Offset)),
	}, 20, height)
	require.NoError(t, err)
	return strings.Split(ansi.Strip(out), "\n")
}

// A window and a Scrollbox agree on every row: the newest line sits on the bottom row, and the top of the list fills
// the screen.
func TestAWindowDrawsFullScreensAtBothEnds(t *testing.T) {
	view, err := tml.Load(fstest.MapFS{"list.tml": &fstest.MapFile{Data: []byte(windowedList)}}, "list.tml", tml.Options{})
	require.NoError(t, err)
	list := measured{t: t, view: view}
	for i := range 10 {
		list.rows = append(list.rows, row(i))
	}

	w := tml.NewWindow()
	screen := drawWindow(t, list, w.Slice(list, 6), 6)
	assert.Equal(t, "last 9", strings.TrimSpace(screen[5]), "the newest line is on the bottom row:\n%s", strings.Join(screen, "\n"))

	w.Scroll(-1000, list, 6)
	screen = drawWindow(t, list, w.Slice(list, 6), 6)
	assert.Equal(t, "title 0", strings.TrimSpace(screen[0]))
	assert.Equal(t, "title 1", strings.TrimSpace(screen[4]), "the screen is full at the top:\n%s", strings.Join(screen, "\n"))
	assert.Equal(t, "first 1", strings.TrimSpace(screen[5]))
}
