package tml_test

import _ "embed"

import (
	"bytes"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml"
	"github.com/wow-look-at-my/tml/cell"
	"github.com/wow-look-at-my/tml/sema"
)

//go:embed testdata/listing-doc.xml
var listingDoc string

//go:embed testdata/listing-theme.xml
var listingTheme string

func loadListing(t *testing.T) *tml.View {
	t.Helper()
	view, err := tml.Load(fstest.MapFS{
		"Listing.tml": {Data: []byte(listingDoc)},
		"theme.tml":   {Data: []byte(listingTheme)},
	}, "Listing.tml", tml.Options{})
	require.NoError(t, err)
	return view
}

func listingProps(rows ...string) tml.Props {
	return tml.Props{"title": sema.StringValue("Runs"), "rows": sema.ListValue(rows)}
}

// A pipe gets the text and nothing else: no escapes, no padding to a width, no blank lines after the content.
func TestPrintToAPipeIsPlainAndCompact(t *testing.T) {
	var out bytes.Buffer
	require.NoError(t, loadListing(t).Print(&out, listingProps("✅\t101\tbuild", "❌\t7\tlint"), 0))

	assert.Equal(t, "Runs\n    ID   NAME\n✅  101  build\n❌  7    lint\n", out.String())
}

// Width zero on a stream that is not a terminal is no limit, so a long cell keeps its whole line. An agent greps it.
func TestPrintToAPipeNeverWrapsALongCell(t *testing.T) {
	long := strings.Repeat("x", 300)
	var out bytes.Buffer
	require.NoError(t, loadListing(t).Print(&out, listingProps("✅\t1\t"+long), 0))

	assert.Contains(t, out.String(), "\n✅  1   "+long+"\n")
}

// An explicit width is the space the document has. A table with no collapse list cuts or wraps inside it.
func TestPrintKeepsInsideAGivenWidth(t *testing.T) {
	var out bytes.Buffer
	require.NoError(t, loadListing(t).Print(&out, listingProps("✅\t1\t"+strings.Repeat("y", 60)), 30))

	for _, line := range strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n") {
		assert.LessOrEqual(t, cell.Width(line), 30, "line %q", line)
	}
}

// CLICOLOR_FORCE makes the writer treat the buffer as a terminal. The colour then reaches it, and NO_COLOR takes the
// colour away and leaves the bold.
func TestPrintFollowsTheTerminalAndNoColor(t *testing.T) {
	t.Serial("it sets TERM, COLORTERM, NO_COLOR and CLICOLOR_FORCE, which every Print in the process reads")
	view := loadListing(t)
	t.Setenv("CLICOLOR_FORCE", "1")
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("COLORTERM", "truecolor")
	t.Setenv("NO_COLOR", "")

	var colour bytes.Buffer
	require.NoError(t, view.Print(&colour, listingProps("✅\t1\tbuild"), 80))
	assert.Contains(t, colour.String(), "38;2;255;0;0", "a truecolor terminal gets the colour")

	t.Setenv("NO_COLOR", "1")
	var bare bytes.Buffer
	require.NoError(t, view.Print(&bare, listingProps("✅\t1\tbuild"), 80))
	assert.NotContains(t, bare.String(), "38;2", "NO_COLOR drops the colour")
	assert.Contains(t, bare.String(), "\x1b[1m", "NO_COLOR keeps the bold")
	assert.Equal(t, "Runs\n    ID  NAME\n✅  1   build\n", cell.Strip(bare.String()))

	t.Setenv("NO_COLOR", "")
	t.Setenv("TERM", "dumb")
	t.Setenv("COLORTERM", "")
	var dumb bytes.Buffer
	require.NoError(t, view.Print(&dumb, listingProps("✅\t1\tbuild"), 80))
	assert.Equal(t, cell.Strip(dumb.String()), dumb.String(), "TERM=dumb gets no escapes at all")
}

// A document with nothing in it prints nothing, not a blank line.
func TestPrintOfNothingWritesNothing(t *testing.T) {
	const doc = `<?xml version="1.1" encoding="UTF-8"?>
<Component xmlns="urn:tml:v1" name="Empty">
	<Property name="rows" type="string[]" default=""/>
	<Template><Stack><For each="{rows}" as="row"><Text>{row}</Text></For></Stack></Template>
</Component>
`
	view, err := tml.Load(fstest.MapFS{"Empty.tml": {Data: []byte(doc)}}, "Empty.tml", tml.Options{})
	require.NoError(t, err)
	var out bytes.Buffer
	require.NoError(t, view.Print(&out, tml.Props{}, 0))
	assert.Empty(t, out.String())
}

func TestPrintWidthOfAStreamThatIsNotATerminal(t *testing.T) {
	assert.Equal(t, tml.UnboundedWidth, tml.PrintWidth(&bytes.Buffer{}))
}
