package render

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml/cell"
	"github.com/wow-look-at-my/tml/layout"
	"github.com/wow-look-at-my/tml/sema"
	"github.com/wow-look-at-my/tml/style"
	"github.com/wow-look-at-my/tml/syntax"
)

// paintTree lays a tree out at a viewport and paints the whole frame.
func paintTree(t *testing.T, node *sema.Node, w, h int) *cell.Buffer {
	t.Helper()
	sheet, err := style.NewSheet(nil, nil)
	require.NoError(t, err)
	box, err := layout.New(sheet, layout.Options{}).Layout(node, w, h)
	require.NoError(t, err)
	return Frame(box, w, h, nil)
}

func styledText(attrs map[string]string, content string) *sema.Node {
	return el("Text", attrs, &sema.Node{Kind: syntax.TextNode, Text: content})
}

func row(buf *cell.Buffer, y int) string { return buf.Lines()[y] }

func TestADescendantInheritsTheTextStyle(t *testing.T) {
	buf := paintTree(t, el("Box", map[string]string{"fg": "#ff0000", "bold": "true"},
		el("Stack", nil, text("hi")),
	), 6, 2)

	got := buf.At(0, 0)
	assert.Equal(t, "h", got.Text)
	assert.Equal(t, cell.MustColor("#ff0000"), got.Style.Fg, "the colour reaches a grandchild")
	assert.True(t, got.Style.Has(cell.Bold), "so does bold")
}

func TestAChildStyleOverridesTheInheritedOne(t *testing.T) {
	buf := paintTree(t, el("Box", map[string]string{"fg": "#ff0000", "bold": "true"},
		styledText(map[string]string{"fg": "#00ff00", "bold": "false"}, "hi"),
	), 6, 1)

	got := buf.At(0, 0)
	assert.Equal(t, cell.MustColor("#00ff00"), got.Style.Fg)
	assert.False(t, got.Style.Has(cell.Bold), "a child that turns bold off draws plain")
}

func TestBackgroundFillsThePaddingAndStaysUnderTheGlyphs(t *testing.T) {
	blue := cell.MustColor("#0000ff")
	buf := paintTree(t, el("Box", map[string]string{"bg": "#0000ff", "padding": "1", "width": "5", "height": "3"},
		text("a"),
	), 8, 4)

	assert.Equal(t, blue, buf.At(0, 0).Style.Bg, "the padding is filled")
	assert.Equal(t, blue, buf.At(4, 2).Style.Bg, "to the far corner")
	assert.Equal(t, "a", buf.At(1, 1).Text)
	assert.Equal(t, blue, buf.At(1, 1).Style.Bg, "a child's glyph keeps the fill behind it")
	assert.False(t, buf.At(5, 0).Style.Bg.IsSet(), "nothing past the box is filled")
	assert.False(t, buf.At(0, 3).Style.Bg.IsSet())
}

func TestACanvasChildCoversWhatIsBeneathIt(t *testing.T) {
	buf := paintTree(t, el("Canvas", nil,
		text("XXXXXX"),
		el("Box", map[string]string{"width": "3", "height": "1", "Canvas.x": "1"}),
	), 6, 1)

	assert.Equal(t, "X   XX", row(buf, 0), "the empty box erases the cells of its own rect only")
}

func TestAGridChildCoversWhatIsBeneathIt(t *testing.T) {
	spanning := gridCell("XXXXXX", "0", "0")
	spanning.Attrs["Grid.columnSpan"] = sema.StringValue("2")
	buf := paintTree(t, el("Grid", map[string]string{"columns": "2,4", "rows": "1"},
		spanning,
		el("Box", map[string]string{"Grid.row": "0", "Grid.column": "0"}),
	), 6, 1)

	assert.Equal(t, "  XXXX", row(buf, 0), "the empty box erases its own cell of the grid")
}

func TestTextAlignment(t *testing.T) {
	tests := []struct {
		name  string
		attrs map[string]string
		x, y  int
	}{
		{"start", nil, 0, 0},
		{"center leans left", map[string]string{"align": "center"}, 3, 0},
		{"right", map[string]string{"align": "right"}, 7, 0},
		{"middle leans down", map[string]string{"valign": "middle"}, 0, 2},
		{"bottom", map[string]string{"valign": "bottom"}, 0, 3},
		{"bottom right", map[string]string{"align": "right", "valign": "bottom"}, 7, 3},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			buf := paintTree(t, styledText(tc.attrs, "abc"), 10, 4)

			assert.Equal(t, "a", buf.At(tc.x, tc.y).Text, "the text starts at %d,%d", tc.x, tc.y)
			assert.Equal(t, "c", buf.At(tc.x+2, tc.y).Text)
		})
	}
}

func TestTextOverflow(t *testing.T) {
	tests := []struct {
		overflow string
		want     []string
	}{
		{"wrap", []string{"abcde", "fghij"}},
		{"clip", []string{"abcde", "     "}},
		{"ellipsis", []string{"abcd…", "     "}},
	}
	for _, tc := range tests {
		t.Run(tc.overflow, func(t *testing.T) {
			buf := paintTree(t, styledText(map[string]string{"overflow": tc.overflow}, "abcdefghij"), 5, 2)
			assert.Equal(t, tc.want, buf.Lines())
		})
	}
}

func TestAHiddenElementPaintsNothingAndKeepsItsSpace(t *testing.T) {
	buf := paintTree(t, el("Stack", nil,
		el("Box", map[string]string{"state": "hidden", "bg": "#ff0000", "border": "normal", "width": "4", "height": "3"}),
		text("x"),
	), 6, 5)

	for y := range 3 {
		for x := range 6 {
			got := buf.At(x, y)
			assert.Equal(t, " ", got.Text, "cell %d,%d", x, y)
			assert.False(t, got.Style.Bg.IsSet(), "cell %d,%d", x, y)
		}
	}
	assert.Equal(t, "x", buf.At(0, 3).Text, "the next child still starts below the kept slot")
}

func TestABorderIsDrawnInItsColour(t *testing.T) {
	green := cell.MustColor("#00ff00")
	buf := paintTree(t, el("Box", map[string]string{
		"border": "rounded", "borderColor": "#00ff00", "width": "4", "height": "3",
	}, text("ab")), 6, 4)

	assert.Equal(t, []string{"╭──╮  ", "│ab│  ", "╰──╯  ", "      "}, buf.Lines())
	for _, at := range [][2]int{{0, 0}, {3, 0}, {0, 1}, {3, 1}, {1, 2}} {
		assert.Equal(t, green, buf.At(at[0], at[1]).Style.Fg, "border cell %v", at)
	}
	assert.False(t, buf.At(1, 1).Style.Fg.IsSet(), "the border colour does not reach the content")
}

func TestAMarginKeepsTheBorderAndFillInside(t *testing.T) {
	buf := paintTree(t, el("Box", map[string]string{
		"margin": "1", "border": "normal", "bg": "#0000ff", "width": "6", "height": "5",
	}), 7, 5)

	assert.Equal(t, " ", buf.At(0, 0).Text, "the margin stays blank")
	assert.False(t, buf.At(0, 0).Style.Bg.IsSet())
	assert.Equal(t, "┌", buf.At(1, 1).Text)
	assert.Equal(t, cell.MustColor("#0000ff"), buf.At(2, 2).Style.Bg, "the fill sits inside the border")
}

func TestRenderPadsEveryRowToTheBoxWidth(t *testing.T) {
	out := renderTree(t, el("Stack", nil, text("a"), text("bcd")), 4, 2)
	assert.Equal(t, "a   \nbcd ", out)
}
