package cell

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseColorReadsEverySpelling(t *testing.T) {
	for raw, want := range map[string]Color{
		"12":          Indexed(12),
		"#ff0000":     RGB(255, 0, 0),
		"#fff":        RGB(255, 255, 255),
		"red":         Indexed(1),
		"brightBlue":  Indexed(12),
		" #10101a ":   RGB(0x10, 0x10, 0x1a),
		"brightWhite": Indexed(15),
	} {
		got, err := ParseColor(raw)
		require.NoError(t, err, raw)
		assert.Equal(t, want, got, raw)
	}
}

// A colour nobody can read is refused by name, so a typo in a theme fails the load instead of drawing in the default.
func TestParseColorRefusesWhatIsNotAColour(t *testing.T) {
	for _, raw := range []string{"", "nope", "#12", "#gggggg", "256", "-1"} {
		_, err := ParseColor(raw)
		assert.Error(t, err, raw)
	}
	assert.Panics(t, func() { MustColor("nope") })
}

func TestColorReportsItsChannels(t *testing.T) {
	r, g, b := RGB(1, 2, 3).RGB()
	assert.Equal(t, []uint8{1, 2, 3}, []uint8{r, g, b})

	r, g, b = Indexed(9).RGB()
	assert.Equal(t, []uint8{255, 0, 0}, []uint8{r, g, b}, "a palette colour answers with the xterm default")

	r, g, b = Indexed(232).RGB()
	assert.Equal(t, []uint8{8, 8, 8}, []uint8{r, g, b}, "the grey ramp")

	r, g, b = Indexed(196).RGB()
	assert.Equal(t, []uint8{255, 0, 0}, []uint8{r, g, b}, "the colour cube")

	index, ok := Indexed(7).Index()
	assert.True(t, ok)
	assert.Equal(t, uint8(7), index)
	_, ok = RGB(0, 0, 0).Index()
	assert.False(t, ok)

	assert.False(t, Color{}.IsSet())
	assert.Equal(t, "", Color{}.String())
	assert.Equal(t, "12", Indexed(12).String())
	assert.Equal(t, "#ff8000", RGB(255, 128, 0).String())
}

func TestBlendMovesTowardTheOtherColour(t *testing.T) {
	assert.Equal(t, RGB(100, 100, 100), RGB(200, 200, 200).Blend(RGB(0, 0, 0), 50))
	assert.Equal(t, RGB(200, 200, 200), RGB(200, 200, 200).Blend(RGB(0, 0, 0), 0))
	assert.Equal(t, RGB(0, 0, 0), RGB(200, 200, 200).Blend(RGB(0, 0, 0), 100))
}

// Over is the cascade: what the upper style states wins, and everything it leaves unset shows the style beneath.
func TestStyleOverInheritsWhatIsUnset(t *testing.T) {
	base := Style{Fg: Indexed(1), Bg: Indexed(2)}.Bold(true).Underline(true)
	upper := Style{Fg: Indexed(3)}.Underline(false).Italic(true)

	merged := upper.Over(base)
	assert.Equal(t, Indexed(3), merged.Fg, "a stated colour wins")
	assert.Equal(t, Indexed(2), merged.Bg, "an unset colour inherits")
	assert.True(t, merged.Has(Bold), "an attribute nobody cleared inherits")
	assert.False(t, merged.Has(Underline), "a cleared attribute goes")
	assert.True(t, merged.Has(Italic))
	assert.True(t, merged.Has(Bold|Italic))
	assert.Equal(t, Bold|Italic, merged.Attrs())

	assert.Equal(t, "x", Style{}.Hyperlink("x").Over(Style{Link: "y"}).Link)
	assert.Equal(t, "y", Style{}.Over(Style{Link: "y"}).Link)
	assert.True(t, Style{}.IsZero())
	assert.False(t, Style{}.Faint(true).IsZero())
	assert.False(t, base.Text().Bg.IsSet(), "the text half carries no fill")
	assert.True(t, Style{}.Reverse(true).Strikethrough(true).Foreground(Indexed(1)).Background(Indexed(2)).Has(Reverse|Strikethrough))
}

func TestWidthCountsCellsNotBytes(t *testing.T) {
	assert.Equal(t, 5, Width("hello"))
	assert.Equal(t, 2, Width("世"), "a wide rune takes a pair of cells")
	assert.Equal(t, 5, Width("\x1b[1;31mhello\x1b[m"), "an escape sequence takes no cells")
	assert.Equal(t, 4, Width("ab\ncdef\ng"), "a block is as wide as its widest line")
	assert.Equal(t, 4, Width("\t"), "a tab is its expansion")
	assert.Equal(t, 3, Height("ab\ncdef\ng"))
	assert.Equal(t, 1, Height(""))
}

// A host that asked its terminal how wide a cluster draws supplies the answer, and it governs every measurement.
func TestMeasureIsAskedPerCluster(t *testing.T) {
	var asked []string
	double := Measure(func(s string) int {
		asked = append(asked, s)
		return 2
	})
	assert.Equal(t, 6, double.Width("abc"))
	assert.Equal(t, []string{"a", "b", "c"}, asked)
	assert.Equal(t, 4, double.Width("ab\ncd\ne"))
}

func TestLinesReadSGRIntoGlyphStyles(t *testing.T) {
	lines := Lines("a\x1b[1;38;2;1;2;3mb\x1b[0mc\x1b[38;5;200;48;5;17md\x1b[39;49me\nf")
	require.Len(t, lines, 2)
	first := lines[0]
	require.Len(t, first, 5)
	assert.True(t, first[0].Style.IsZero())
	assert.True(t, first[1].Style.Has(Bold))
	assert.Equal(t, RGB(1, 2, 3), first[1].Style.Fg)
	assert.True(t, first[2].Style.IsZero(), "a reset clears the style")
	assert.Equal(t, Indexed(200), first[3].Style.Fg)
	assert.Equal(t, Indexed(17), first[3].Style.Bg)
	assert.False(t, first[4].Style.Fg.IsSet())
	assert.Equal(t, "f", lines[1].String())
}

func TestLinesReadEveryAttributeAndItsOff(t *testing.T) {
	on := Lines("\x1b[2;3;4;5;7;9;31;42;91;102mx")[0][0].Style
	assert.True(t, on.Has(Faint|Italic|Underline|Blink|Reverse|Strikethrough))
	assert.Equal(t, Indexed(9), on.Fg)
	assert.Equal(t, Indexed(10), on.Bg)

	off := Lines("\x1b[1;2;3;4;5;7;9m\x1b[22;23;24;25;27;29mx")[0][0].Style
	assert.Zero(t, off.Attrs())

	plain := Lines("\x1b[1m\x1b[mx")[0][0].Style
	assert.Zero(t, plain.Attrs(), "an empty SGR is a reset")

	assert.Equal(t, "x", Lines("\x1b[38mx")[0].String(), "a colour with no form is dropped")
	assert.Equal(t, "x", Lines("\x1b[?1mx")[0].String(), "parameters that are not numbers are dropped")
}

func TestLinesDropWhatIsNotText(t *testing.T) {
	line := Lines("a\x1b[2Kb\x1b]0;title\x07c\x1b_Gimage\x1b\\d\re\x00f\x1bMg")[0]
	assert.Equal(t, "abcdefg", line.String())

	linked := Lines("\x1b]8;;http://x\x1b\\go\x1b]8;;\x1b\\ on")[0]
	assert.Equal(t, "http://x", linked[0].Style.Link)
	assert.Equal(t, "", linked[2].Style.Link, "the link ends where the sequence says")

	assert.Equal(t, "ab", Lines("ab\x1b")[0].String(), "a cut-off escape is dropped")
	assert.Equal(t, "ab", Lines("ab\x1b[12")[0].String())
	assert.Equal(t, "ab", Lines("ab\x1b]never ends")[0].String())
}

func TestStripKeepsOnlyTheText(t *testing.T) {
	assert.Equal(t, "plain", Strip("plain"))
	assert.Equal(t, "red\nrow", Strip("\x1b[31mred\x1b[m\nrow"))
}

func TestTextBuildsALineInOneStyle(t *testing.T) {
	line := Measure(nil).Text("a\nb", Style{}.Bold(true))
	assert.Equal(t, "ab", line.String())
	assert.True(t, line[0].Style.Has(Bold))
	assert.Equal(t, 2, line.Width())

	styled := Lines("a\x1b[3mb")[0].Styled(Style{Fg: Indexed(4)})
	assert.Equal(t, Indexed(4), styled[1].Style.Fg)
	assert.True(t, styled[1].Style.Has(Italic), "a glyph's own style lies over the one laid under it")
}

func texts(lines []Line) []string {
	out := make([]string, len(lines))
	for i, line := range lines {
		out[i] = line.String()
	}
	return out
}

func TestWrapBreaksAtSpacesAndDropsThem(t *testing.T) {
	assert.Equal(t, []string{"the quick", "brown fox"}, texts(Lines("the quick brown fox")[0].Wrap(10)))
	assert.Equal(t, []string{"one"}, texts(Lines("one")[0].Wrap(10)), "a line that fits is left alone")
	assert.Equal(t, []string{"a b c"}, texts(Lines("a b c")[0].Wrap(0)), "no limit is no wrap")
}

func TestWrapBreaksAWordOnlyWhenItIsWiderThanARow(t *testing.T) {
	assert.Equal(t, []string{"abcde", "fghij", "k"}, texts(Lines("abcdefghijk")[0].Wrap(5)))
	assert.Equal(t, []string{"ab", "cdefg", "h"}, texts(Lines("ab cdefgh")[0].Wrap(5)))
}

func TestWrapBreaksAfterAHyphen(t *testing.T) {
	assert.Equal(t, []string{"well-", "known"}, texts(Lines("well-known")[0].Wrap(6)))
}

func TestWrapNeverSplitsAWideGlyph(t *testing.T) {
	rows := Lines("世界世界")[0].Wrap(3)
	assert.Equal(t, []string{"世", "界", "世", "界"}, texts(rows))
	for _, row := range rows {
		assert.LessOrEqual(t, row.Width(), 3)
	}
}

func TestWrapKeepsTrailingBlanksThatFit(t *testing.T) {
	assert.Equal(t, []string{"ab  "}, texts(Lines("ab  ")[0].Wrap(4)))
	assert.Equal(t, []string{"abcd", "ef"}, texts(Lines("abcd ef   ")[0].Wrap(4)), "blanks that do not fit are dropped")
	assert.Equal(t, []string{"", "abcd"}, texts(Lines("   abcd")[0].Wrap(4)), "an indent that leaves no room goes")
}

func TestMeasureWrapHandlesEveryLine(t *testing.T) {
	assert.Equal(t, []string{"aa", "bb", "", "cc"}, texts(Measure(nil).Wrap("aa bb\n\ncc", 2)))
}

func TestTruncatePathCutsBeforeTheLastElement(t *testing.T) {
	line := Lines("src/videos/2026/holiday-4k.mov")[0]
	assert.Equal(t, "src/videos/2026/holiday-4k.mov", line.TruncatePath(40, "…").String(), "a path that fits is left alone")
	assert.Equal(t, "src/vide…/holiday-4k.mov", line.TruncatePath(24, "…").String(), "the start stays as long as it fits")
	assert.Equal(t, "…/holiday-4k.mov", line.TruncatePath(16, "…").String())
	assert.Equal(t, "…holiday-4k.mov", line.TruncatePath(15, "…").String(), "the name keeps the mark before the separator")
	assert.Equal(t, "holiday-…", line.TruncatePath(9, "…").String(), "a name wider than the row loses its own end")
	assert.Equal(t, `C:\Users\…\notes.txt`, Lines(`C:\Users\me\Documents\notes.txt`)[0].TruncatePath(20, "…").String())
	assert.Equal(t, "plainna…", Lines("plainname.txt")[0].TruncatePath(8, "…").String(), "no separator is a plain cut")
	for width := 0; width < line.Width(); width++ {
		assert.LessOrEqual(t, line.TruncatePath(width, "…").Width(), width, "width %d", width)
	}
}

func TestTruncateMarksTheCut(t *testing.T) {
	line := Lines("hello world")[0]
	assert.Equal(t, "hello w…", line.Truncate(8, "…").String())
	assert.Equal(t, "hello wo", line.Truncate(8, "").String())
	assert.Equal(t, "hello world", line.Truncate(20, "…").String(), "a line that fits is left alone")
	assert.Equal(t, "", line.Truncate(0, "…").String(), "no room for the mark is no mark")
	assert.Equal(t, "世…", Lines("世界世界")[0].Truncate(4, "…").String(), "a wide glyph is never cut in half")

	styled := Lines("\x1b[1mhello world")[0].Truncate(6, "…")
	assert.True(t, styled[len(styled)-1].Style.Has(Bold), "the mark draws in the style of the text it ends")
}

func TestCutTakesColumns(t *testing.T) {
	line := Lines("abcdef")[0]
	assert.Equal(t, "cd", line.Cut(2, 4).String())
	assert.Equal(t, "", line.Cut(9, 12).String())
	assert.Equal(t, " b ", Lines("世b界")[0].Cut(1, 4).String(), "half a wide glyph becomes a blank")
}

func TestBufferStartsBlankAndReadsBlankOutside(t *testing.T) {
	buf := NewBuffer(3, 2)
	assert.Equal(t, "   \n   ", buf.Text())
	assert.Equal(t, []string{"   ", "   "}, buf.Lines())
	assert.Equal(t, " ", buf.At(9, 9).Text)
	assert.Equal(t, " ", buf.At(-1, 0).Text)
	assert.Equal(t, []string{}, NewBuffer(4, 0).Lines())
	assert.Equal(t, 0, NewBuffer(-1, -1).W)
}

func TestCanvasDrawsTextAndClips(t *testing.T) {
	buf := NewBuffer(6, 2)
	c := buf.Canvas(nil)
	end := c.Text(1, 0, "hello world", Style{}.Bold(true))
	assert.Equal(t, 12, end, "the column after the last glyph, clipped or not")
	assert.Equal(t, " hello\n      ", buf.Text())
	assert.True(t, buf.At(1, 0).Style.Has(Bold))

	c.Text(0, 1, "a\nb", Style{})
	assert.Equal(t, "a", buf.At(0, 1).Text, "a newline moves to the next row, and a row outside is dropped")
	w, h := c.Size()
	assert.Equal(t, []int{6, 2}, []int{w, h})
	assert.Nil(t, c.Measure())
}

func TestCanvasSubTranslatesAndClips(t *testing.T) {
	buf := NewBuffer(6, 3)
	sub := buf.Canvas(nil).Sub(2, 1, 3, 1)
	sub.Text(0, 0, "abcdef", Style{})
	sub.Text(0, 1, "below", Style{})
	sub.Text(-1, 0, "X", Style{})
	assert.Equal(t, "      \n  abc \n      ", buf.Text())
	assert.Equal(t, "a", sub.At(0, 0).Text)
	assert.Equal(t, Rect{X: 0, Y: 0, W: 3, H: 1}, sub.Visible())

	clipped := buf.Canvas(nil).Clip(0, 0, 2, 1)
	clipped.Fill(0, 0, 6, 3, Style{Bg: Indexed(1)})
	assert.True(t, buf.At(1, 0).Style.Bg.IsSet())
	assert.False(t, buf.At(2, 0).Style.Bg.IsSet(), "a clip narrows what draws and leaves the origin alone")
}

// A glyph drawn with no fill of its own keeps the fill already on the cell, which is how text sits on a background.
func TestCanvasKeepsTheFillUnderAGlyph(t *testing.T) {
	buf := NewBuffer(3, 1)
	c := buf.Canvas(nil)
	c.Fill(0, 0, 3, 1, Style{Bg: Indexed(4)})
	c.Text(0, 0, "a", Style{Fg: Indexed(1)})
	assert.Equal(t, Indexed(4), buf.At(0, 0).Style.Bg)
	assert.Equal(t, Indexed(1), buf.At(0, 0).Style.Fg)
}

func TestCanvasStyledIsInherited(t *testing.T) {
	buf := NewBuffer(2, 1)
	c := buf.Canvas(nil).Styled(Style{Fg: Indexed(2)}.Bold(true))
	assert.True(t, c.Base().Has(Bold))
	c.Text(0, 0, "a", Style{})
	c.Text(1, 0, "b", Style{Fg: Indexed(3)}.Bold(false))
	assert.Equal(t, Indexed(2), buf.At(0, 0).Style.Fg)
	assert.True(t, buf.At(0, 0).Style.Has(Bold))
	assert.Equal(t, Indexed(3), buf.At(1, 0).Style.Fg)
	assert.False(t, buf.At(1, 0).Style.Has(Bold), "a draw can clear what it inherited")
}

func TestWideGlyphsOwnTwoCellsAndAreRepairedWhenCut(t *testing.T) {
	buf := NewBuffer(4, 1)
	c := buf.Canvas(nil)
	c.Text(0, 0, "世界", Style{})
	assert.Equal(t, uint8(2), buf.At(0, 0).Width)
	assert.Equal(t, uint8(0), buf.At(1, 0).Width)
	assert.Equal(t, "世界", buf.Text())

	c.Text(1, 0, "x", Style{})
	assert.Equal(t, " x界", buf.Text(), "a write on the second half blanks the first")

	c.Text(2, 0, "y", Style{})
	assert.Equal(t, " xy ", buf.Text(), "a write on the first half blanks the second")

	edge := NewBuffer(3, 1)
	edge.Canvas(nil).Text(2, 0, "世", Style{})
	assert.Equal(t, "   ", edge.Text(), "a wide glyph the clip cuts in half becomes a blank")
}

func TestRestyleChangesStylesAndKeepsGlyphs(t *testing.T) {
	buf := NewBuffer(3, 1)
	c := buf.Canvas(nil)
	c.Text(0, 0, "abc", Style{})
	c.Restyle(1, 0, 5, 1, func(drawn Cell) Style { return drawn.Style.Reverse(true) })
	assert.False(t, buf.At(0, 0).Style.Has(Reverse))
	assert.True(t, buf.At(1, 0).Style.Has(Reverse))
	assert.Equal(t, "abc", buf.Text())
}

func TestEraseBlanksBackToTheGround(t *testing.T) {
	buf := NewBuffer(3, 1)
	c := buf.Canvas(nil)
	c.Fill(0, 0, 3, 1, Style{Bg: Indexed(1)})
	c.Text(0, 0, "abc", Style{})

	c.Grounded(Indexed(5)).Erase(1, 0, 1, 1)
	assert.Equal(t, "a c", buf.Text())
	assert.Equal(t, Indexed(5), buf.At(1, 0).Style.Bg, "the erased cell takes the ground, not the fill it had")
	assert.Equal(t, Indexed(1), buf.At(0, 0).Style.Bg)
}

func TestBoxDrawsABorder(t *testing.T) {
	buf := NewBuffer(4, 3)
	border, ok := BorderNamed("rounded")
	require.True(t, ok)
	buf.Canvas(nil).Box(0, 0, 4, 3, border, Style{Fg: Indexed(2)})
	assert.Equal(t, "╭──╮\n│  │\n╰──╯", buf.Text())
	assert.Equal(t, Indexed(2), buf.At(0, 0).Style.Fg)

	buf.Canvas(nil).Box(0, 0, 0, 3, border, Style{})
	_, ok = BorderNamed("nope")
	assert.False(t, ok)
	assert.Equal(t, []string{"ascii", "block", "double", "hidden", "normal", "rounded", "thick"}, BorderNames())
}

func TestPlaceRecordsAnEscapeOverARegion(t *testing.T) {
	buf := NewBuffer(4, 2)
	c := buf.Canvas(nil)
	c.Place(1, 0, 2, 2, "IMG", "CLR")
	require.Len(t, buf.Placements, 1)
	assert.Equal(t, Placement{Rect: Rect{X: 1, Y: 0, W: 2, H: 2}, Seq: "IMG", Clear: "CLR"}, buf.Placements[0])
	assert.Equal(t, " IMG   \n    ", buf.String(), "the escape is written where its region starts")

	c.Place(3, 0, 2, 1, "CUT", "")
	c.Place(0, 0, 0, 1, "EMPTY", "")
	assert.Len(t, buf.Placements, 1, "a region the clip cuts into is skipped")
}

func TestStringStylesRunsAndResetsEachRow(t *testing.T) {
	buf := NewBuffer(4, 2)
	c := buf.Canvas(nil)
	c.Text(0, 0, "ab", Style{Fg: RGB(255, 0, 0)}.Bold(true))
	c.Text(2, 0, "c", Style{Fg: Indexed(2)})
	c.Text(0, 1, "d", Style{Bg: Indexed(12)}.Underline(true))

	assert.Equal(t, "\x1b[1;38;2;255;0;0mab\x1b[0;32mc\x1b[m \n\x1b[4;104md\x1b[m   ", buf.String())
}

func TestEncodeFollowsTheProfile(t *testing.T) {
	buf := NewBuffer(1, 1)
	buf.Canvas(nil).Text(0, 0, "x", Style{Fg: RGB(255, 0, 0), Bg: Indexed(200)}.Bold(true))

	assert.Equal(t, "\x1b[1;38;2;255;0;0;48;5;200mx\x1b[m", buf.Encode(TrueColor))
	assert.Equal(t, "\x1b[1;38;5;196;48;5;200mx\x1b[m", buf.Encode(ANSI256))
	assert.Equal(t, "\x1b[1;91;105mx\x1b[m", buf.Encode(ANSI), "every colour maps to a base colour")
	assert.Equal(t, "\x1b[1mx\x1b[m", buf.Encode(NoColor), "the attributes stay and the colours go")
	assert.Equal(t, "x", buf.Encode(Plain))
}

func TestEncodeWritesHyperlinks(t *testing.T) {
	buf := NewBuffer(3, 1)
	buf.Canvas(nil).Text(0, 0, "go", Style{}.Hyperlink("http://x"))
	assert.Equal(t, "\x1b]8;;http://x\x1b\\go\x1b]8;;\x1b\\ ", buf.String())
	assert.Equal(t, "go ", buf.Encode(Plain))
}

func TestUsedIgnoresBlanksThatShowNothing(t *testing.T) {
	buf := NewBuffer(6, 4)
	c := buf.Canvas(nil)
	c.Text(0, 0, "ab", Style{})
	c.Fill(0, 1, 3, 1, Style{}.Bold(true))
	c.Fill(0, 2, 3, 1, Style{Bg: Indexed(1)})
	assert.Equal(t, 2, buf.Used(0))
	assert.Equal(t, 0, buf.Used(1), "a bold blank shows nothing")
	assert.Equal(t, 3, buf.Used(2), "a filled blank shows its fill")
	assert.Equal(t, 0, buf.Used(3))

	var out strings.Builder
	buf.WriteRow(&out, 0, 1, 99, Plain)
	assert.Equal(t, "b    ", out.String())
}

func TestDetectProfileReadsTheEnvironment(t *testing.T) {
	env := func(pairs ...string) func(string) string {
		values := map[string]string{}
		for i := 0; i < len(pairs); i += 2 {
			values[pairs[i]] = pairs[i+1]
		}
		return func(name string) string { return values[name] }
	}

	assert.Equal(t, Plain, DetectProfile(env("TERM", "xterm-256color", "COLORTERM", "truecolor"), false), "a pipe gets plain text")
	assert.Equal(t, TrueColor, DetectProfile(env("TERM", "xterm", "COLORTERM", "truecolor", "CLICOLOR_FORCE", "1"), false))
	assert.Equal(t, Plain, DetectProfile(env("TERM", "dumb"), true))
	assert.Equal(t, NoColor, DetectProfile(env("TERM", "xterm", "NO_COLOR", "1"), true))
	assert.Equal(t, TrueColor, DetectProfile(env("TERM", "xterm", "COLORTERM", "24bit"), true))
	assert.Equal(t, TrueColor, DetectProfile(env("TERM", "xterm", "TERM_PROGRAM", "WezTerm"), true))
	assert.Equal(t, TrueColor, DetectProfile(env("TERM", "xterm", "WT_SESSION", "x"), true))
	assert.Equal(t, TrueColor, DetectProfile(env("TERM", "xterm-kitty"), true))
	assert.Equal(t, TrueColor, DetectProfile(env("TERM", "xterm-direct"), true))
	assert.Equal(t, ANSI256, DetectProfile(env("TERM", "screen-256color"), true))
	assert.Equal(t, ANSI, DetectProfile(env("TERM", "vt100"), true))
}

func TestRectIntersects(t *testing.T) {
	assert.Equal(t, Rect{X: 2, Y: 2, W: 2, H: 1}, Rect{W: 4, H: 3}.Intersect(Rect{X: 2, Y: 2, W: 5, H: 5}))
	assert.Equal(t, 0, Rect{W: 2, H: 2}.Intersect(Rect{X: 5, Y: 5, W: 1, H: 1}).W)
	assert.True(t, Rect{W: 2, H: 2}.Contains(1, 1))
	assert.False(t, Rect{W: 2, H: 2}.Contains(2, 1))
}
