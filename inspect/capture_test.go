package inspect

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSnapshotHoldsTheFrameTheTreeAndTheElements(t *testing.T) {
	got, err := Snapshot(NewServer(newFake()))
	require.NoError(t, err)

	assert.Equal(t, uint64(7), got.Frame.Seq)
	assert.Equal(t, 80, got.Frame.Width)
	assert.Equal(t, "\x1b[31mready\x1b[0m", got.Frame.ANSI, "the styled frame is what the preview draws")
	require.NotNil(t, got.Tree)
	assert.Equal(t, "app", got.Tree.ID)

	ids := make([]string, 0, len(got.Elements))
	for _, el := range got.Elements {
		ids = append(ids, el.ID)
	}
	assert.Equal(t, []string{"app", "status"}, ids)
}

// A program that has not painted has nothing to capture, and the reason it gives is the program's own.
func TestSnapshotFailsWhenNothingHasBeenPainted(t *testing.T) {
	_, err := Snapshot(NewServer(&fake{}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "has not painted a frame yet")
}

func TestSnapshotFrameTakesAFrameInHand(t *testing.T) {
	got, err := SnapshotFrame(newFake().frame)
	require.NoError(t, err)
	assert.Equal(t, uint64(7), got.Frame.Seq)
	require.NotNil(t, got.Tree)
}

// The page has to open on its own: a capture that fetches an asset is a blank window everywhere it travels to.
func TestWriteCaptureInlinesEveryAsset(t *testing.T) {
	page := capturePage(t)

	assert.NotContains(t, page, styleTag, "the stylesheet link is replaced by the stylesheet")
	assert.NotContains(t, page, scriptTag, "the script tag is replaced by the script")
	assert.NotContains(t, page, importMark, "an import cannot resolve from a file")
	assert.NotContains(t, page, exportMark, "an export outside a module file is a syntax error")
	assert.Contains(t, page, "--highlight", "the stylesheet's tokens are in the page")
	assert.Contains(t, page, "function toHTML", "the ANSI converter is in the page")
	assert.Contains(t, page, "answerFromCapture", "the reads the page makes are answered from the document")

	for _, ref := range regexp.MustCompile(`(?:href|src)="([^"]+)"`).FindAllStringSubmatch(page, -1) {
		assert.Fail(t, "the page still loads something", "it references %s", ref[1])
	}
}

// The page reads its answers back out of the document, so what it holds has to be the capture itself.
func TestWriteCaptureHoldsTheAnswersAsJSON(t *testing.T) {
	page := capturePage(t)

	held := between(t, page, `<script type="application/json" id="capture">`, "</script>")
	var back Capture
	require.NoError(t, json.Unmarshal([]byte(held), &back))
	assert.Equal(t, uint64(7), back.Frame.Seq)
	require.NotNil(t, back.Tree)
	assert.Equal(t, "app", back.Tree.ID)
	require.Len(t, back.Elements, 2)
	assert.Equal(t, "status", back.Elements[1].ID)
}

// The held JSON sits inside a script element, so a frame that drew a closing tag must not end that element early.
func TestWriteCaptureCannotCloseItsOwnScript(t *testing.T) {
	f := newFake()
	f.frame.ANSI = "</script><script>alert(1)</script>"
	c, err := SnapshotFrame(f.frame)
	require.NoError(t, err)

	var page strings.Builder
	require.NoError(t, WriteCapture(&page, c))
	assert.NotContains(t, page.String(), "alert(1)</script>")
	assert.Contains(t, page.String(), "\\u003c/script\\u003e", "the escaped form is what the document holds")
}

// A renamed asset or a rewritten module seam fails the write. A capture that opens blank reports nothing at all.
func TestWriteCaptureFailsOnAMarkerItCannotFind(t *testing.T) {
	_, err := replace("a page with no marker in it", styleTag, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no longer contains")
}

func capturePage(t *testing.T) string {
	t.Helper()
	c, err := Snapshot(NewServer(newFake()))
	require.NoError(t, err)
	var page strings.Builder
	require.NoError(t, WriteCapture(&page, c))
	return page.String()
}

func between(t *testing.T, text, opens, closes string) string {
	t.Helper()
	_, rest, ok := strings.Cut(text, opens)
	require.True(t, ok, "the page contains %s", opens)
	held, _, ok := strings.Cut(rest, closes)
	require.True(t, ok, "the element is closed")
	return held
}
