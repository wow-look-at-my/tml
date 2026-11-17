package vt

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTextWrapsAndScrollsIntoHistory(t *testing.T) {
	term := New(4, 2)
	term.WriteString("abcdef\r\nxy\r\nz")
	assert.Equal(t, []string{"abcd", "ef"}, term.History())
	assert.Equal(t, []string{"xy", "z"}, term.Screen())
	assert.Equal(t, []string{"abcd", "ef", "xy", "z"}, term.Lines())
	x, y := term.Cursor()
	assert.Equal(t, []int{1, 1}, []int{x, y})
}

func TestCursorMovesAndErases(t *testing.T) {
	term := New(6, 3)
	term.WriteString("aaaaaa\r\nbbbbbb\r\ncccccc")
	term.WriteString("\x1b[2;3H\x1b[K")
	assert.Equal(t, []string{"aaaaaa", "bb", "cccccc"}, term.Screen())

	term.WriteString("\x1b[1A\x1b[2DX\x1b[1B\x1b[1CY\x1b[5GZ\x1b[2K")
	assert.Equal(t, []string{"Xaaaaa", "", "cccccc"}, term.Screen())

	term.WriteString("\x1b[1;1H\x1b[J")
	assert.Empty(t, term.Lines(), "an erase from the first cell clears the screen")

	term.WriteString("one\x1b7\r\ntwo\x1b8!\x1b[2J")
	assert.Empty(t, term.Lines())
	term.WriteString("\x1b[1;1Hab\bc\a")
	assert.Equal(t, []string{"ac"}, term.Lines())
}

func TestWideGlyphsAndStylesTakeTheirCells(t *testing.T) {
	term := New(5, 2)
	term.WriteString("\x1b[1;31m世界\x1b[mab")
	assert.Equal(t, []string{"世界a", "b"}, term.Lines(), "a styled wide glyph takes a pair of cells")
}

func TestTheAlternateScreenLeavesTheMainOneAlone(t *testing.T) {
	term := New(6, 2)
	term.WriteString("main")
	term.WriteString("\x1b[?1049h\x1b[Halt\r\nmore\r\nrows")
	assert.Equal(t, []string{"more", "rows"}, term.Screen())
	assert.Empty(t, term.History(), "the alternate screen keeps no history")

	term.WriteString("\x1b[?1049l")
	assert.Equal(t, []string{"main"}, term.Lines())
}

func TestASequenceCutOffWaitsForItsRest(t *testing.T) {
	term := New(6, 1)
	_, _ = term.Write([]byte("a\x1b[1"))
	_, _ = term.Write([]byte("mb\x1b]0;title"))
	_, _ = term.Write([]byte("\x07c\x1b_Gx\x1b\\d\x1b"))
	_, _ = term.Write([]byte("[?25le"))
	assert.Equal(t, []string{"abcde"}, term.Lines())
}
