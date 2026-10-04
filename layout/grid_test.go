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

// Track solving runs fixed, then auto, then star: a star track only ever divides what the other a pair of left behind.
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

// A span covers its tracks and swallows the gaps between them, so a spanning child lines up with the columns
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

// Declaring fewer tracks than a child uses widens the grid rather than dropping the child off it.
func TestGridWidensForAChildBeyondTheDeclaredTracks(t *testing.T) {
	grid := el("Grid", map[string]string{"columns": "auto"},
		el("Box", cell("0", "0", nil)),
		textCell("x", "0", "3"),
	)

	box := layoutTree(t, grid, 20, 5)
	assert.Len(t, box.cols, 4, "the grid grew to hold column 3")
}

// A star track with a range keeps inside it. What a capped track cannot take goes to the other star tracks.
func TestStarTrackRange(t *testing.T) {
	grid := el("Grid", map[string]string{"columns": "*[..5],*"},
		el("Box", cell("0", "0", nil)),
		el("Box", cell("0", "1", nil)),
	)
	box := layoutTree(t, grid, 20, 5)
	assert.Equal(t, 5, box.Children[0].Rect.W, "the capped track stops at its maximum")
	assert.Equal(t, 15, box.Children[1].Rect.W, "the other track takes what the cap left")

	grid = el("Grid", map[string]string{"columns": "*[8..],12"},
		el("Box", cell("0", "0", nil)),
		el("Box", cell("0", "1", nil)),
	)
	box = layoutTree(t, grid, 16, 5)
	assert.Equal(t, 8, box.Children[0].Rect.W, "a track keeps its minimum even when the grid runs out")
}

// A lone star track with a maximum leaves the rest of the row empty rather than stretching past it.
func TestCappedStarLeavesTheRestEmpty(t *testing.T) {
	grid := el("Grid", map[string]string{"columns": "*[4..10],6", "gap": "1"},
		el("Box", cell("0", "0", nil)),
		el("Box", cell("0", "1", nil)),
	)
	box := layoutTree(t, grid, 40, 5)
	assert.Equal(t, 10, box.Children[0].Rect.W)
	assert.Equal(t, 11, box.Children[1].Rect.X, "the next column starts right after the capped one")
}

// An auto track with a range clamps what its content measured.
func TestAutoTrackRange(t *testing.T) {
	grid := el("Grid", map[string]string{"columns": "auto[..3],auto[5..]"},
		textCell("abcdef", "0", "0"),
		textCell("x", "0", "1"),
	)
	box := layoutTree(t, grid, 40, 5)
	assert.Equal(t, 3, box.Children[0].Rect.W)
	assert.Equal(t, 5, box.Children[1].Rect.W)
}

// collapse names the order columns give way in. A column goes only while the rest do not fit, and the gap goes with it.
func TestCollapseDropsColumnsInOrderUntilTheRestFit(t *testing.T) {
	grid := func() *sema.Node {
		return el("Grid", map[string]string{"columns": "*[6..],4,5,3", "gap": "1", "collapse": "2,3"},
			textCell("label", "0", "0"),
			textCell("aaaa", "0", "1"),
			textCell("bbbbb", "0", "2"),
			textCell("ccc", "0", "3"),
		)
	}

	wide := layoutTree(t, grid(), 30, 1)
	for i, want := range []int{15, 4, 5, 3} {
		assert.Equal(t, want, wide.Children[i].Rect.W, "column %d at full width", i)
	}

	narrow := layoutTree(t, grid(), 20, 1)
	assert.Equal(t, 0, narrow.Children[2].Rect.W, "column 2 collapsed")
	assert.Equal(t, 3, narrow.Children[3].Rect.W, "column 3 still fits")
	assert.Equal(t, 11, narrow.Children[0].Rect.W, "the star column takes what the collapse freed")
	assert.Equal(t, 17, narrow.Children[3].Rect.X, "no gap is left where column 2 was")

	narrower := layoutTree(t, grid(), 11, 1)
	assert.Equal(t, 0, narrower.Children[3].Rect.W, "column 3 collapsed next")
	assert.Equal(t, 6, narrower.Children[0].Rect.W)

	// A column the list does not name never goes. The grid overflows instead, and the star keeps its minimum.
	tiny := layoutTree(t, grid(), 5, 1)
	assert.Equal(t, 4, tiny.Children[1].Rect.W)
	assert.Equal(t, 6, tiny.Children[0].Rect.W)
}

// A child that spans a collapsed column keeps the shown part of its span.
func TestSpanSkipsACollapsedColumn(t *testing.T) {
	grid := el("Grid", map[string]string{"columns": "4,4,4", "gap": "1", "collapse": "1"},
		el("Box", cell("0", "0", map[string]string{"Grid.columnSpan": "3"})),
	)
	box := layoutTree(t, grid, 10, 1)
	assert.Equal(t, 9, box.Children[0].Rect.W, "two shown columns and the one gap between them")
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
			name:    "a range on a fixed track",
			node:    func() *sema.Node { return el("Grid", map[string]string{"columns": "4[1..2]"}) },
			wantErr: "a fixed track has no range",
		},
		{
			name:    "a range without its dots",
			node:    func() *sema.Node { return el("Grid", map[string]string{"columns": "*[4]"}) },
			wantErr: "a range is written [min..max]",
		},
		{
			name:    "a range that is upside down",
			node:    func() *sema.Node { return el("Grid", map[string]string{"columns": "*[9..3]"}) },
			wantErr: "the minimum is larger than the maximum",
		},
		{
			name:    "a range end that is not a number",
			node:    func() *sema.Node { return el("Grid", map[string]string{"columns": "*[a..3]"}) },
			wantErr: "a range end must be a whole number",
		},
		{
			name:    "a collapse index past the last column",
			node:    func() *sema.Node { return el("Grid", map[string]string{"columns": "1,1", "collapse": "2"}) },
			wantErr: "is not a column index from 0 to 1",
		},
		{
			name:    "a column named twice in collapse",
			node:    func() *sema.Node { return el("Grid", map[string]string{"columns": "1,1", "collapse": "1,1"}) },
			wantErr: "column 1 is named twice",
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
