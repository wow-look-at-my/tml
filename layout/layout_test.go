package layout

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

func engine(t *testing.T) *Engine {
	t.Helper()
	sheet, err := style.NewSheet(nil, nil)
	require.NoError(t, err)
	return New(sheet, nil)
}

func layoutTree(t *testing.T, node *sema.Node, w, h int) *Box {
	t.Helper()
	box, err := engine(t).Layout(node, w, h)
	require.NoError(t, err)
	return box
}

// Star children share whatever is left after the fixed ones, in proportion to
// their weights.
func TestStarSharesLeftoverByWeight(t *testing.T) {
	root := el("Stack", map[string]string{"orientation": "horizontal"},
		text("ab"), // auto: 2 cells
		el("Box", map[string]string{"width": "1*"}),
		el("Box", map[string]string{"width": "3*"}),
	)

	box := layoutTree(t, root, 22, 5)
	children := box.Children

	assert.Equal(t, 2, children[0].Rect.W, "an auto child keeps its content width")
	assert.Equal(t, 5, children[1].Rect.W, "1 of 4 shares of the remaining 20")
	assert.Equal(t, 15, children[2].Rect.W, "3 of 4 shares")

	total := children[0].Rect.W + children[1].Rect.W + children[2].Rect.W
	assert.Equal(t, 22, total, "the row fills exactly, with no cell lost to rounding")
}

// The remainder from an uneven division goes to the last star child rather than
// being dropped, so a row always adds up.
func TestStarRoundingLeavesNoGap(t *testing.T) {
	root := el("Stack", map[string]string{"orientation": "horizontal"},
		el("Box", map[string]string{"width": "*"}),
		el("Box", map[string]string{"width": "*"}),
		el("Box", map[string]string{"width": "*"}),
	)

	box := layoutTree(t, root, 10, 3)

	total := 0
	for _, child := range box.Children {
		total += child.Rect.W
	}
	assert.Equal(t, 10, total, "10 cells across 3 equal shares still fills 10")
}

func TestGapConsumesSpaceBetweenChildrenOnly(t *testing.T) {
	root := el("Stack", map[string]string{"orientation": "horizontal", "gap": "2"},
		el("Box", map[string]string{"width": "*"}),
		el("Box", map[string]string{"width": "*"}),
	)

	box := layoutTree(t, root, 12, 3)

	assert.Equal(t, 5, box.Children[0].Rect.W, "12 cells minus one 2-cell gap, halved")
	assert.Equal(t, 5, box.Children[1].Rect.W)
	assert.Equal(t, 7, box.Children[1].Rect.X, "the second child starts after the gap")
}

func TestVerticalIsTheDefaultOrientation(t *testing.T) {
	root := el("Stack", nil, text("a"), text("b"))
	box := layoutTree(t, root, 10, 10)

	assert.Equal(t, 0, box.Children[0].Rect.Y)
	assert.Equal(t, 1, box.Children[1].Rect.Y, "children stack downward by default")
}

// Wrapping changes how much vertical space a text needs, so measure has to
// account for it or the parent under-budgets the row.
func TestTextMeasuresItsWrappedHeight(t *testing.T) {
	root := el("Stack", nil, text("the quick brown fox jumps"))

	narrow := layoutTree(t, root, 10, 10)
	wide := layoutTree(t, root, 40, 10)

	assert.Greater(t, narrow.Children[0].Rect.H, 1, "narrow text wraps onto more lines")
	assert.Equal(t, 1, wide.Children[0].Rect.H, "given room, it stays on one line")
}

// Padding and borders come out of the content box, so a box's content area is
// smaller than its rect by exactly the frame.
func TestFrameReducesTheContentBox(t *testing.T) {
	root := el("Box", map[string]string{"width": "20", "height": "5", "padding": "1 2", "border": "normal"})
	box := layoutTree(t, root, 40, 10)

	assert.Equal(t, 20, box.Rect.W)
	// 2 cells of border plus 4 of horizontal padding.
	assert.Equal(t, 14, box.Content.W)
	// 2 cells of border plus 2 of vertical padding.
	assert.Equal(t, 1, box.Content.H)
}

// Margin is TML's, not lipgloss's, so it must come out of the rect too.
func TestMarginReducesTheContentBox(t *testing.T) {
	root := el("Box", map[string]string{"width": "20", "height": "5", "margin": "1"})
	box := layoutTree(t, root, 40, 10)

	assert.Equal(t, 18, box.Content.W)
	assert.Equal(t, 3, box.Content.H)
}

func TestExplicitSizeOverridesContent(t *testing.T) {
	root := el("Stack", nil, el("Text", map[string]string{"width": "3"},
		&sema.Node{Kind: syntax.TextNode, Text: "much longer than three"}))

	box := layoutTree(t, root, 40, 10)
	assert.Equal(t, 3, box.Children[0].Rect.W)
}

func TestLayoutRejectsABadLength(t *testing.T) {
	_, err := engine(t).Layout(el("Box", map[string]string{"width": "wide"}), 10, 10)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "length must be auto")
}

func TestLayoutRejectsAnUnknownStyleAttribute(t *testing.T) {
	_, err := engine(t).Layout(el("Box", map[string]string{"colour": "red"}), 10, 10)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `unknown style attribute "colour"`)
}
