package layout

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml/sema"
	"github.com/wow-look-at-my/tml/syntax"
)

func cell(row, column string, extra map[string]string) map[string]string {
	attrs := map[string]string{"Grid.row": row, "Grid.column": column}
	for key, value := range extra {
		attrs[key] = value
	}
	return attrs
}

// textCell is a Text element already placed on the grid.
func textCell(content, row, column string) *sema.Node {
	node := el("Text", cell(row, column, nil))
	node.Children = append(node.Children, &sema.Node{Kind: syntax.TextNode, Text: content})
	return node
}

// Track solving runs fixed, then auto, then star: a star track only ever divides
// what the other two left behind.
func TestTrackSolvingOrder(t *testing.T) {
	grid := el("Grid", map[string]string{"columns": "6,auto,*"},
		el("Box", cell("0", "0", nil)),
		textCell("abc", "0", "1"), // the auto column sizes to this
		el("Box", cell("0", "2", nil)),
	)

	box := layoutTree(t, grid, 20, 5)

	assert.Equal(t, 6, box.Children[0].Rect.W, "the fixed track keeps its 6 cells")
	assert.Equal(t, 3, box.Children[1].Rect.W, "the auto track sizes to its content")
	assert.Equal(t, 11, box.Children[2].Rect.W, "the star track takes what is left")
}

func TestStarTracksDivideByWeight(t *testing.T) {
	grid := el("Grid", map[string]string{"columns": "1*,3*"},
		el("Box", cell("0", "0", nil)),
		el("Box", cell("0", "1", nil)),
	)
	box := layoutTree(t, grid, 20, 5)

	assert.Equal(t, 5, box.Children[0].Rect.W)
	assert.Equal(t, 15, box.Children[1].Rect.W)
}

func TestGapSitsBetweenTracksOnly(t *testing.T) {
	grid := el("Grid", map[string]string{"columns": "*,*", "gap": "2"},
		el("Box", cell("0", "0", nil)),
		el("Box", cell("0", "1", nil)),
	)
	box := layoutTree(t, grid, 12, 5)

	assert.Equal(t, 5, box.Children[0].Rect.W, "12 cells less one 2-cell gap, halved")
	assert.Equal(t, 0, box.Children[0].Rect.X)
	assert.Equal(t, 7, box.Children[1].Rect.X, "the second column starts after the gap")
}

// A span covers its tracks and swallows the gaps between them, so a spanning
// child lines up with the columns underneath it.
func TestSpanCoversTracksAndTheGapsBetween(t *testing.T) {
	grid := el("Grid", map[string]string{"columns": "4,4,4", "gap": "1"},
		el("Box", cell("0", "0", map[string]string{"Grid.columnSpan": "2"})),
	)
	box := layoutTree(t, grid, 20, 5)

	assert.Equal(t, 9, box.Children[0].Rect.W, "two 4-cell columns plus the 1-cell gap between them")
}

func TestRowsPlaceChildrenVertically(t *testing.T) {
	grid := el("Grid", map[string]string{"rows": "2,3"},
		el("Box", cell("0", "0", nil)),
		el("Box", cell("1", "0", nil)),
	)
	box := layoutTree(t, grid, 10, 10)

	assert.Equal(t, 0, box.Children[0].Rect.Y)
	assert.Equal(t, 2, box.Children[1].Rect.Y, "the second row starts below the first")
	assert.Equal(t, 3, box.Children[1].Rect.H)
}

// Declaring fewer tracks than a child uses widens the grid rather than dropping
// the child off it.
func TestGridWidensForAChildBeyondTheDeclaredTracks(t *testing.T) {
	grid := el("Grid", map[string]string{"columns": "auto"},
		el("Box", cell("0", "0", nil)),
		textCell("x", "0", "3"),
	)

	box := layoutTree(t, grid, 20, 5)
	assert.Len(t, box.cols, 4, "the grid grew to hold column 3")
}

func TestGridRejects(t *testing.T) {
	tests := []struct {
		name    string
		node    func() *sema.Node
		wantErr string
	}{
		{
			name:    "an unknown attached property",
			node:    func() *sema.Node { return el("Grid", nil, el("Box", map[string]string{"Grid.depth": "1"})) },
			wantErr: `unknown attached property "Grid.depth"`,
		},
		{
			name:    "a non-numeric placement",
			node:    func() *sema.Node { return el("Grid", nil, el("Box", map[string]string{"Grid.row": "first"})) },
			wantErr: "must be a whole number",
		},
		{
			name:    "a bad track list",
			node:    func() *sema.Node { return el("Grid", map[string]string{"columns": "auto,wide"}) },
			wantErr: "columns:",
		},
		{
			name:    "an attached property outside a Grid",
			node:    func() *sema.Node { return el("Stack", nil, el("Box", map[string]string{"Grid.row": "1"})) },
			wantErr: "only applies to a child of <Grid>",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := engine(t).Layout(tc.node(), 20, 5)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}
