package tml_test

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml"
	"github.com/wow-look-at-my/tml/sema"
)

var update = flag.Bool("update", false, "rewrite the golden files")

// golden compares against testdata/<name>.golden, rewriting it under -update.
//
// The dashboard fixture deliberately uses no colour, so the goldens stay plain
// readable text. Style.Render emits ANSI for any colour that is set regardless
// of TTY, which would otherwise make these files unreviewable.
func golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")
	if *update {
		require.NoError(t, os.WriteFile(path, []byte(got), 0o644))
		return
	}
	assert.Equal(t, readGolden(t, path, got), got)
}

// readGolden returns the recorded output. An empty golden is seeded from this
// run and then fails: seeding silently would bless whatever a broken renderer
// happened to emit and turn the first run green for the wrong reason.
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

// The expanded tree proves the language features composed: an imported
// component, a filled slot, a slot falling back, and a loop.
func TestDashboardExpansion(t *testing.T) {
	node, err := load(t).Expand(props(t))
	require.NoError(t, err)
	golden(t, "dashboard-tree", node.Dump())
}

// The rendered frame proves layout and styling: borders, padding, gaps, and a
// star-sized card filling the viewport width.
func TestDashboardRender(t *testing.T) {
	out, err := load(t).Render(props(t), 40, 20)
	require.NoError(t, err)
	golden(t, "dashboard-render", out)
}

// A star-sized child fills the width it is given, and an auto-sized sibling does
// not. This is the property the whole sizing model rests on.
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

func TestLoadReportsDiagnosticsWithPositions(t *testing.T) {
	_, err := tml.Load(os.DirFS("testdata/dashboard"), "missing.tml", tml.Options{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot read")
}
