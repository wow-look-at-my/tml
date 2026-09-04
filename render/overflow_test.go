package render_test

import (
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml"
	"github.com/wow-look-at-my/tml/sema"
)

// A card holding a log is the case overflow exists for: the box is as tall as
// the log has lines, and a long line neither wraps nor pushes the border down.
const overflowDoc = `<?xml version="1.1" encoding="UTF-8"?>
<Component xmlns="urn:tml:v1" name="Card">
	<Property name="mode" type="string" default="wrap"/>
	<Template>
		<Box width="20">
			<Stack orientation="vertical">
				<Text overflow="{mode}">a line that is far too long for this box</Text>
				<Text>after</Text>
			</Stack>
		</Box>
	</Template>
</Component>
`

func renderMode(t *testing.T, mode string) string {
	t.Helper()
	view, err := tml.Load(docFS(t, "card.tml", overflowDoc), "card.tml", tml.Options{})
	require.NoError(t, err)
	out, err := view.Render(tml.Props{"mode": stringValue(mode)}, 40, 10)
	require.NoError(t, err)
	return out
}

func TestOverflow_WrapKeepsEveryCell(t *testing.T) {
	out := renderMode(t, "wrap")
	assert.Contains(t, out, "too long")
	// Wrapping puts the rest of the line on the next row, so the box is taller.
	assert.GreaterOrEqual(t, len(nonEmptyLines(out)), 3)
}

func TestOverflow_ClipCutsTheLineAndKeepsTheHeight(t *testing.T) {
	clipped := nonEmptyLines(renderMode(t, "clip"))
	wrapped := nonEmptyLines(renderMode(t, "wrap"))
	assert.Less(t, len(clipped), len(wrapped), "clipping must not cost a row")
	assert.NotContains(t, strings.Join(clipped, "\n"), "…")
	assert.Contains(t, strings.Join(clipped, "\n"), "after")
	for _, line := range clipped {
		assert.LessOrEqual(t, len([]rune(strings.TrimRight(line, " "))), 40)
	}
}

func TestOverflow_EllipsisMarksTheCut(t *testing.T) {
	out := renderMode(t, "ellipsis")
	assert.Contains(t, out, "…")
	assert.NotContains(t, out, "too long for this box")
}

func TestOverflow_RejectsAnUnknownMode(t *testing.T) {
	view, err := tml.Load(docFS(t, "card.tml", overflowDoc), "card.tml", tml.Options{})
	require.NoError(t, err)
	_, err = view.Render(tml.Props{"mode": stringValue("scroll")}, 40, 10)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not wrap, clip or ellipsis")
}

func docFS(t *testing.T, name, body string) fs.FS {
	t.Helper()
	return fstest.MapFS{name: &fstest.MapFile{Data: []byte(body)}}
}

func stringValue(s string) sema.Value { return sema.StringValue(s) }

func nonEmptyLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if strings.TrimSpace(line) != "" {
			out = append(out, line)
		}
	}
	return out
}
