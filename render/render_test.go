package render

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml/layout"
	"github.com/wow-look-at-my/tml/sema"
	"github.com/wow-look-at-my/tml/style"
	"github.com/wow-look-at-my/tml/syntax"
)

func el(name string, attrs map[string]string, children ...*sema.Node) *sema.Node {
	node := &sema.Node{Kind: syntax.ElementNode, Name: name, Attrs: map[string]sema.Value{}, Children: children}
	for key, value := range attrs {
		node.Attrs[key] = sema.StringValue(value)
	}
	return node
}

func text(content string) *sema.Node {
	return el("Text", nil, &sema.Node{Kind: syntax.TextNode, Text: content})
}

func renderTree(t *testing.T, node *sema.Node, w, h int) string {
	t.Helper()
	sheet, err := style.NewSheet(nil, nil)
	require.NoError(t, err)
	box, err := layout.New(sheet, layout.Options{}).Layout(node, w, h)
	require.NoError(t, err)
	return Render(box)
}

// fitTree renders a tree as tall as its content, the way RenderFit does. A
// viewport-filling layout hides the difference a child that takes no space
// makes, so the state tests need this.
func fitTree(t *testing.T, node *sema.Node, w, h int) string {
	t.Helper()
	sheet, err := style.NewSheet(nil, nil)
	require.NoError(t, err)
	box, err := layout.New(sheet, layout.Options{}).Fit(node, w, h)
	require.NoError(t, err)
	return Render(box)
}

// Regression: a container whose composed content is wider than its box must be CLIPPED. Setting only Width makes
func TestOverflowingContentIsClippedNotWrapped(t *testing.T) {
	row := el("Stack", map[string]string{"orientation": "horizontal", "gap": "1"},
		text("aaaaaaaaaa"),
		text("bbbbbbbbbb"),
	)
	out := renderTree(t, el("Box", map[string]string{"width": "12"}, row), 40, 6)

	for _, line := range strings.Split(out, "\n") {
		assert.LessOrEqual(t, lipgloss.Width(line), 12, "no line may exceed the box it was given")
	}
	assert.NotContains(t, out, "bbbbbbbbbb", "the overflow is cut off, not folded onto another line")
}

func TestVerticalGapProducesBlankLines(t *testing.T) {
	stack := el("Stack", map[string]string{"gap": "1"}, text("one"), text("two"))
	out := renderTree(t, stack, 10, 6)

	lines := strings.Split(out, "\n")
	require.GreaterOrEqual(t, len(lines), 3)
	assert.Contains(t, lines[0], "one")
	assert.Empty(t, strings.TrimSpace(lines[1]), "one blank line between the two children")
	assert.Contains(t, lines[2], "two")
}

// Regression: a Stack that measures to nothing on the stacking axis -- an empty items control, or a component whose own
func TestAZeroHeightChildAddsNoRow(t *testing.T) {
	empty := el("Stack", nil)
	stack := el("Stack", nil, text("one"), empty, text("two"))
	out := renderTree(t, stack, 10, 6)

	lines := strings.Split(out, "\n")
	require.GreaterOrEqual(t, len(lines), 2)
	assert.Contains(t, lines[0], "one")
	assert.Contains(t, lines[1], "two", "the empty child contributed no row between them")
}

// The horizontal counterpart: a nothing-width child must not open a gap between the columns on either side of it.
func TestAZeroWidthChildAddsNoColumn(t *testing.T) {
	empty := el("Stack", nil)
	stack := el("Stack", map[string]string{"orientation": "horizontal"}, text("ab"), empty, text("cd"))
	out := renderTree(t, stack, 20, 3)

	assert.Contains(t, strings.Split(out, "\n")[0], "abcd", "the empty child contributed no column of its own")
}

func TestHorizontalGapProducesBlankColumns(t *testing.T) {
	stack := el("Stack", map[string]string{"orientation": "horizontal", "gap": "3"}, text("ab"), text("cd"))
	out := renderTree(t, stack, 20, 3)

	assert.Contains(t, strings.Split(out, "\n")[0], "ab   cd", "three blank columns separate the children")
}

func TestTextStillWrapsWithinItsOwnBox(t *testing.T) {
	stack := el("Stack", nil, text("the quick brown fox"))
	out := renderTree(t, stack, 9, 6)

	assert.Greater(t, lipgloss.Height(out), 1, "text wraps rather than being clipped")
	for _, line := range strings.Split(out, "\n") {
		assert.LessOrEqual(t, lipgloss.Width(line), 9)
	}
}

// A grid cannot be produced by joining: its children sit at coordinates, so the renderer composites them through
func TestGridComposesChildrenAtTheirCoordinates(t *testing.T) {
	grid := el("Grid", map[string]string{"columns": "6,6", "rows": "1,1"},
		gridCell("aa", "0", "0"),
		gridCell("bb", "0", "1"),
		gridCell("cc", "1", "0"),
		gridCell("dd", "1", "1"),
	)

	out := renderTree(t, grid, 12, 2)
	lines := strings.Split(out, "\n")
	require.Len(t, lines, 2)

	assert.Equal(t, "aa    bb", strings.TrimRight(lines[0], " "), "columns are placed side by side")
	assert.Equal(t, "cc    dd", strings.TrimRight(lines[1], " "), "the second row lands underneath")
}

func gridCell(content, row, column string) *sema.Node {
	node := el("Text", map[string]string{"Grid.row": row, "Grid.column": column})
	node.Children = append(node.Children, &sema.Node{Kind: syntax.TextNode, Text: content})
	return node
}

func TestBorderAndPaddingSurroundTheContent(t *testing.T) {
	box := el("Box", map[string]string{"border": "normal", "padding": "0 1", "width": "6"}, text("hi"))
	out := renderTree(t, box, 20, 5)

	lines := strings.Split(out, "\n")
	require.GreaterOrEqual(t, len(lines), 3)
	assert.Contains(t, lines[0], "┌", "the border is drawn around the content")
	assert.Contains(t, lines[1], "│ hi │", "padding sits inside the border")
}

// A hidden element keeps its row and draws nothing in it; a collapsed one leaves no row behind at all. The geometry half is
// layout's test.
func TestHiddenKeepsItsRowAndCollapsedDropsIt(t *testing.T) {
	stack := func(state string) *sema.Node {
		attrs := map[string]string{}
		if state != "" {
			attrs["state"] = state
		}
		return el("Stack", nil,
			text("one"),
			el("Text", attrs, &sema.Node{Kind: syntax.TextNode, Text: "two"}),
			text("three"),
		)
	}

	visible := strings.Split(fitTree(t, stack(""), 20, 8), "\n")
	hidden := strings.Split(fitTree(t, stack("hidden"), 20, 8), "\n")
	collapsed := strings.Split(fitTree(t, stack("collapsed"), 20, 8), "\n")

	require.Len(t, visible, 3)
	assert.Equal(t, []string{"one", "two", "three"}, trimmed(visible))

	require.Len(t, hidden, 3, "a hidden element keeps its row")
	assert.Equal(t, []string{"one", "", "three"}, trimmed(hidden), "and draws nothing in it")

	require.Len(t, collapsed, 2, "a collapsed element leaves no row behind")
	assert.Equal(t, []string{"one", "three"}, trimmed(collapsed))
}

// A Stack whose children all measured to nothing still has a gap to put nowhere: it draws nothing rather than panicking.
func TestAStackWithOnlyEmptyChildrenDrawsNothing(t *testing.T) {
	empty := el("Stack", nil)
	stack := el("Stack", map[string]string{"gap": "1"}, empty, el("Text", map[string]string{"state": "collapsed"}, &sema.Node{Kind: syntax.TextNode, Text: "gone"}))

	assert.Equal(t, "", strings.TrimRight(fitTree(t, stack, 10, 4), " \n"))
}

// trimmed is a frame's lines with their padding removed, so a blank row reads as an empty string.
func trimmed(lines []string) []string {
	out := make([]string, len(lines))
	for i, line := range lines {
		out[i] = strings.TrimRight(line, " ")
	}
	return out
}
