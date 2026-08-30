package tml_test

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml"
	"github.com/wow-look-at-my/tml/sema"
	"github.com/wow-look-at-my/tml/widget"
)

var update = flag.Bool("update", false, "rewrite the golden files")

// golden compares against testdata/<name>.golden, rewriting it under -update. The dashboard fixture deliberately uses
func golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")
	if *update {
		require.NoError(t, os.WriteFile(path, []byte(got), 0o644))
		return
	}
	assert.Equal(t, readGolden(t, path, got), got)
}

// readGolden returns the recorded output. An empty golden is seeded from this run and then fails: seeding silently
func readGolden(t *testing.T, path, got string) string {
	t.Helper()
	info, err := os.Stat(path)
	require.NoError(t, err, "missing golden file; rerun with -update")

	if info.Size() > 0 {
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		return string(data)
	}
	require.NoError(t, os.WriteFile(path, []byte(got), 0o644))
	t.Fatalf("golden %s was empty; seeded it from this run -- re-run to verify, and read the diff before trusting it", path)
	return ""
}

func load(t *testing.T) *tml.View {
	t.Helper()
	view, err := tml.Load(os.DirFS("testdata/dashboard"), "app.tml", tml.Options{})
	require.NoError(t, err)
	return view
}

func props(t *testing.T) tml.Props {
	t.Helper()
	tags, err := sema.ParseType("string[]")
	require.NoError(t, err)
	value, err := sema.ParseValue(tags, "api,web")
	require.NoError(t, err)
	return tml.Props{"title": sema.StringValue("Deployments"), "tags": value}
}

// The expanded tree proves the language features composed: an imported component, a filled slot, a slot falling back,
func TestDashboardExpansion(t *testing.T) {
	node, err := load(t).Expand(props(t))
	require.NoError(t, err)
	golden(t, "dashboard-tree", node.Dump())
}

// The rendered frame proves layout and styling: borders, padding, gaps, and a star-sized card filling the viewport
func TestDashboardRender(t *testing.T) {
	out, err := load(t).Render(props(t), 40, 20)
	require.NoError(t, err)
	golden(t, "dashboard-render", out)
}

// A star-sized child fills the width it is given, and an auto-sized sibling does not. This is the property the whole
func TestStarSizingFillsTheViewport(t *testing.T) {
	view := load(t)

	for _, width := range []int{30, 60} {
		box, err := view.Layout(props(t), width, 20)
		require.NoError(t, err)

		stack := box.Children[0]
		filled := stack.Children[1]
		natural := stack.Children[2]

		assert.Equal(t, width, filled.Rect.W, "the star-sized card takes the full width")
		assert.Less(t, natural.Rect.W, width, "the auto-sized card takes only what it needs")
	}
}

// The grid fixture proves the whole pipeline handles attached properties: parse, analyse, expand, solve tracks, and
func TestGridRendersThroughTheWholePipeline(t *testing.T) {
	view, err := tml.Load(os.DirFS("testdata/grid"), "app.tml", tml.Options{})
	require.NoError(t, err)

	out, err := view.Render(nil, 40, 7)
	require.NoError(t, err)
	golden(t, "grid-render", out)
}

// fakeInput stands in for a bubbles component: it draws itself and accepts a width, exactly both things TML asks of
type fakeInput struct{ width int }

func (f *fakeInput) View() string {
	if f.width <= 0 {
		return "[]"
	}
	return "[" + strings.Repeat("_", f.width-2) + "]"
}

func (f *fakeInput) SetWidth(w int) { f.width = w }

func widgetFS() fstest.MapFS {
	const header = "<?xml version=\"1.1\" encoding=\"UTF-8\"?>\n"
	return fstest.MapFS{
		"app.tml": {Data: []byte(header + `<Component xmlns="urn:tml:v1" name="App">
	<Template>
		<Stack orientation="horizontal" gap="1">
			<Text>find</Text>
			<Search width="*"/>
		</Stack>
	</Template>
</Component>`)},
	}
}

// A bound widget is laid out like any other element: it is told the width the star share gave it and renders into
func TestBoundWidgetIsSizedByLayout(t *testing.T) {
	input := &fakeInput{}
	widgets := widget.NewRegistry().Bind("Search", widget.Bubble(input))

	view, err := tml.Load(widgetFS(), "app.tml", tml.Options{Widgets: widgets})
	require.NoError(t, err)

	out, err := view.Render(nil, 20, 3)
	require.NoError(t, err)

	assert.Equal(t, 15, input.width, "the widget was told the width layout computed")
	assert.Contains(t, out, "find [_____________]")
}

// A template naming a widget the host never bound must fail when the view loads, not render a silent blank.
func TestUnboundWidgetIsRejectedAtLoad(t *testing.T) {
	_, err := tml.Load(widgetFS(), "app.tml", tml.Options{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown element <Search>")
}

func TestLoadReportsDiagnosticsWithPositions(t *testing.T) {
	_, err := tml.Load(os.DirFS("testdata/dashboard"), "missing.tml", tml.Options{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot read")
}

// measuredFS is a single Text and a single Badge, so the assertion covers both the language's own measurement and a widget's.
func measuredFS() fstest.MapFS {
	const header = "<?xml version=\"1.1\" encoding=\"UTF-8\"?>\n"
	return fstest.MapFS{
		"app.tml": {Data: []byte(header + `<Component xmlns="urn:tml:v1" name="App">
	<Template>
		<Stack>
			<Text>abcd</Text>
			<Badge label="abcd"/>
		</Stack>
	</Template>
</Component>`)},
	}
}

// Which width method is right depends on what the terminal agreed to, and only the host had that conversation. A view
func TestOptionsMeasureGovernsGeometry(t *testing.T) {
	layoutWith := func(m widget.Measurer) (text, badge int) {
		view, err := tml.Load(measuredFS(), "app.tml", tml.Options{Measure: m})
		require.NoError(t, err)
		box, err := view.Layout(nil, 40, 6)
		require.NoError(t, err)
		stack := box.Children[0]
		return stack.Children[0].Rect.W, stack.Children[1].Rect.W
	}

	text, badge := layoutWith(nil)
	assert.Equal(t, 4, text, "lipgloss is the default and counts four cells")
	assert.Equal(t, 6, badge, "the badge pads its label with a space either side")

	wideText, wideBadge := layoutWith(func(s string) int { return len([]rune(s)) * 2 })
	assert.Equal(t, 8, wideText, "the host's measurer reaches the language's own text")
	assert.Equal(t, 12, wideBadge, "and reaches a widget, through its Context")
}
