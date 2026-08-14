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

// Regression: a container whose composed content is wider than its box must be
// CLIPPED. Setting only Width makes lipgloss wrap the block, which folds a
// too-wide row onto the next line and interleaves the pieces -- a row of buttons
// came out as alternating fragments before this was fixed.
func TestOverflowingContentIsClippedNotWrapped(t *testing.T) {
	row := el("Stack", map[string]string{"orientation": "horizontal", "gap": "1"},
		text("aaaaaaaaaa"),
		text("bbbbbbbbbb"),
	)
	// The row wants 21 cells and is given 12.
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

// A grid cannot be produced by joining: its children sit at coordinates, so the
// renderer composites them through lipgloss layers instead.
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
	// 2 cells of border, 2 of padding, 2 of content.
	box := el("Box", map[string]string{"border": "normal", "padding": "0 1", "width": "6"}, text("hi"))
	out := renderTree(t, box, 20, 5)

	lines := strings.Split(out, "\n")
	require.GreaterOrEqual(t, len(lines), 3)
	assert.Contains(t, lines[0], "┌", "the border is drawn around the content")
	assert.Contains(t, lines[1], "│ hi │", "padding sits inside the border")
}
