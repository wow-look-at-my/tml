package term

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml/cell"
	"github.com/wow-look-at-my/tml/input"
	"github.com/wow-look-at-my/tml/internal/vt"
)

// decode feeds every byte at once and flushes, as a read that ended with nothing after it does.
func decode(s string) []any {
	var d Decoder
	return append(d.Feed([]byte(s)), d.Flush()...)
}

func keystrokes(values []any) []string {
	var out []string
	for _, value := range values {
		if key, ok := value.(input.Key); ok {
			out = append(out, key.Keystroke())
		}
	}
	return out
}

func TestDecoderReadsTypedText(t *testing.T) {
	values := decode("aB世")
	require.Len(t, values, 3)
	assert.Equal(t, input.Key{Code: 'a', Text: "a"}, values[0])
	assert.Equal(t, input.Key{Code: 'b', Mod: input.Shift, Text: "B"}, values[1], "an upper-case letter is its key with shift")
	assert.Equal(t, input.Key{Code: '世', Text: "世"}, values[2])
}

func TestDecoderReadsControlKeys(t *testing.T) {
	assert.Equal(t, []string{"enter", "tab", "backspace", "ctrl+c", "ctrl+a", "ctrl+space", "ctrl+j", "ctrl+]"},
		keystrokes(decode("\r\t\x7f\x03\x01\x00\n\x1d")))
}

func TestDecoderReadsNamedKeys(t *testing.T) {
	for seq, want := range map[string]string{
		"\x1b[A": "up", "\x1b[B": "down", "\x1b[C": "right", "\x1b[D": "left",
		"\x1b[H": "home", "\x1b[F": "end", "\x1b[Z": "shift+tab",
		"\x1b[2~": "insert", "\x1b[3~": "delete", "\x1b[5~": "pgup", "\x1b[6~": "pgdown",
		"\x1b[1~": "home", "\x1b[4~": "end", "\x1b[15~": "f5", "\x1b[24~": "f12",
		"\x1bOA": "up", "\x1bOP": "f1", "\x1bOS": "f4",
		"\x1b[1;5A": "ctrl+up", "\x1b[1;2D": "shift+left", "\x1b[3;3~": "alt+delete", "\x1b[1;8H": "ctrl+alt+shift+home",
		"\x1b[97;5u": "ctrl+a", "\x1b[13;2u": "shift+enter", "\x1b[27;5;13~": "ctrl+enter",
		"\x1ba": "alt+a", "\x1b\x7f": "alt+backspace", "\x1b": "esc",
	} {
		assert.Equal(t, []string{want}, keystrokes(decode(seq)), "%q", seq)
	}
}

func TestDecoderReadsCodePointKeysAsText(t *testing.T) {
	values := decode("\x1b[97u\x1b[97;2u\x1b[9u\x1b[27u\x1b[127u")
	require.Len(t, values, 5)
	assert.Equal(t, "a", values[0].(input.Key).Text)
	assert.Equal(t, "A", values[1].(input.Key).Text)
	assert.Equal(t, []string{"a", "shift+a", "tab", "esc", "backspace"}, keystrokes(values))
	assert.Empty(t, decode("\x1b[0u"), "a key with no code point is nothing")
}

// A lone ESC is the escape key only once nothing followed it, so the decoder holds it until it is flushed.
func TestDecoderHoldsALoneEscapeUntilFlushed(t *testing.T) {
	var d Decoder
	assert.Empty(t, d.Feed([]byte{0x1b}))
	assert.True(t, d.Pending())
	assert.Equal(t, []string{"esc"}, keystrokes(d.Flush()))
	assert.False(t, d.Pending())
}

func TestDecoderJoinsASequenceThatArrivesInPieces(t *testing.T) {
	var d Decoder
	assert.Empty(t, d.Feed([]byte("\x1b[1;")))
	assert.True(t, d.Pending())
	assert.Equal(t, []string{"ctrl+right"}, keystrokes(d.Feed([]byte("5C"))))

	assert.Empty(t, d.Feed([]byte{0xe4, 0xb8}), "a rune cut in half waits for its last byte")
	assert.Equal(t, []string{"世"}, keystrokes(d.Feed([]byte{0x96})))

	assert.Empty(t, d.Feed([]byte("\x1bO")))
	assert.Equal(t, []string{"down"}, keystrokes(d.Feed([]byte("B"))))
}

func TestDecoderReadsThePointer(t *testing.T) {
	values := decode("\x1b[<0;5;3M\x1b[<0;5;3m\x1b[<35;6;4M\x1b[<64;1;1M\x1b[<65;1;1M\x1b[<2;2;2M\x1b[<20;9;9M\x1b[<8;1;1M")
	require.Len(t, values, 8)
	assert.Equal(t, input.Mouse{X: 4, Y: 2, Button: input.ButtonLeft, Action: input.Press}, values[0])
	assert.Equal(t, input.Mouse{X: 4, Y: 2, Button: input.ButtonLeft, Action: input.Release}, values[1])
	assert.Equal(t, input.Mouse{X: 5, Y: 3, Button: input.ButtonNone, Action: input.Motion}, values[2])
	assert.Equal(t, input.Mouse{Button: input.WheelUp, Action: input.Wheel}, values[3])
	assert.Equal(t, input.Mouse{Button: input.WheelDown, Action: input.Wheel}, values[4])
	assert.Equal(t, input.ButtonRight, values[5].(input.Mouse).Button)
	assert.Equal(t, input.Ctrl|input.Shift, values[6].(input.Mouse).Mod)
	assert.Equal(t, input.Alt, values[7].(input.Mouse).Mod)
	assert.Empty(t, decode("\x1b[<0;5M"), "a report with no row is dropped")
}

// A paste arrives as one value, however the terminal cut it up, and a key sequence inside it is text.
func TestDecoderReadsAPasteAsOneValue(t *testing.T) {
	assert.Equal(t, []any{input.Paste{Text: "hi\x1b[Athere"}, input.Key{Code: 'x', Text: "x"}},
		decode("\x1b[200~hi\x1b[Athere\x1b[201~x"))

	var d Decoder
	assert.Empty(t, d.Feed([]byte("\x1b[200~par")))
	assert.Empty(t, d.Feed([]byte("t\x1b[20")), "bytes that could open the closing sequence wait")
	assert.Equal(t, []any{input.Paste{Text: "part"}}, d.Feed([]byte("1~")))
}

func TestDecoderReadsFocusAndDropsReports(t *testing.T) {
	assert.Equal(t, []any{input.Focus{Gained: true}, input.Focus{Gained: false}}, decode("\x1b[I\x1b[O"))
	assert.Empty(t, decode("\x1b[?62;4c\x1b[12;40R\x1b]11;rgb:0/0/0\x07\x1b_Gi=1;OK\x1b\\\x1bP1$r\x1b\\\x1b[?2026;2$y"),
		"an answer to a query nobody asked is dropped")
	assert.Equal(t, []string{"f3"}, keystrokes(decode("\x1b[1;1R")), "F3 is told apart from a cursor report")
	assert.Empty(t, decode("\x1b[99~\x1b[x"), "a sequence that names nothing is dropped")
	assert.Empty(t, decode("\x1b[12"), "a cut-off sequence is dropped at a flush")
	assert.Empty(t, decode("\x1b]never ends"))
	assert.Empty(t, decode("\xff"), "a byte that is no rune is dropped")
	assert.Equal(t, []string{"alt+shift+o"}, keystrokes(decode("\x1bO")))
	assert.Equal(t, []string{"esc", "esc"}, keystrokes(decode("\x1b\x1b")))
}

func row(text string, style cell.Style, width int) *cell.Buffer {
	lines := strings.Split(text, "\n")
	buf := cell.NewBuffer(width, len(lines))
	for y, line := range lines {
		buf.Canvas(nil).Text(0, y, line, style)
	}
	return buf
}

func TestFullScreenDrawsAndLeaves(t *testing.T) {
	var out bytes.Buffer
	screen := NewScreen(&out, Options{Mouse: true, Paste: true, Focus: true, Profile: cell.TrueColor})
	require.NoError(t, screen.Enter())
	require.NoError(t, screen.Draw(row("hello\nworld", cell.Style{}, 10)))

	term := vt.New(10, 4)
	term.WriteString(out.String())
	assert.Equal(t, []string{"hello", "world"}, term.Lines())
	assert.Contains(t, out.String(), "\x1b[?1049h", "the alternate screen")
	assert.Contains(t, out.String(), "\x1b[?25l", "the cursor is hidden")
	assert.Contains(t, out.String(), "\x1b[?1003h\x1b[?1006h", "the pointer reports")
	assert.Contains(t, out.String(), "\x1b[?2004h")
	assert.Contains(t, out.String(), "\x1b[?1004h")

	out.Reset()
	require.NoError(t, screen.Leave())
	assert.Equal(t, "\x1b[?1000l\x1b[?1003l\x1b[?1006l\x1b[?2004l\x1b[?1004l\x1b[?25h\x1b[?1049l", out.String())
}

// The second frame writes only the cells that changed, which is the whole point of keeping the first.
func TestFullScreenWritesOnlyWhatChanged(t *testing.T) {
	var out bytes.Buffer
	screen := NewScreen(&out, Options{Profile: cell.Plain})
	require.NoError(t, screen.Draw(row("hello world one\nsame row", cell.Style{}, 20)))

	out.Reset()
	require.NoError(t, screen.Draw(row("hello WORLD one\nsame row", cell.Style{}, 20)))
	assert.Equal(t, "\x1b[?2026h\x1b[1;7HWORLD\x1b[?2026l", out.String())

	out.Reset()
	require.NoError(t, screen.Draw(row("hello WORLD one\nsame row", cell.Style{}, 20)))
	assert.Equal(t, "\x1b[?2026h\x1b[?2026l", out.String(), "an unchanged frame writes no cell")

	out.Reset()
	require.NoError(t, screen.Draw(row("Xello WORLD onY\nsame row", cell.Style{}, 20)))
	assert.Equal(t, "\x1b[?2026h\x1b[1;1HX\x1b[1;15HY\x1b[?2026l", out.String(), "changes far apart are separate writes")
}

func TestFullScreenRepaintsAfterAResizeOrInvalidate(t *testing.T) {
	var out bytes.Buffer
	screen := NewScreen(&out, Options{Profile: cell.Plain})
	require.NoError(t, screen.Draw(row("ab", cell.Style{}, 4)))

	out.Reset()
	require.NoError(t, screen.Draw(row("ab", cell.Style{}, 6)))
	assert.Contains(t, out.String(), "\x1b[2J\x1b[1;1Hab", "a new size repaints")

	out.Reset()
	screen.Invalidate()
	require.NoError(t, screen.Draw(row("ab", cell.Style{}, 6)))
	assert.Contains(t, out.String(), "\x1b[2J\x1b[1;1Hab")
}

func TestFullScreenRewritesAWholeWideGlyph(t *testing.T) {
	var out bytes.Buffer
	screen := NewScreen(&out, Options{Profile: cell.Plain})
	require.NoError(t, screen.Draw(row("a世b", cell.Style{}, 6)))
	term := vt.New(6, 1)
	term.WriteString(out.String())

	out.Reset()
	require.NoError(t, screen.Draw(row("a界b", cell.Style{}, 6)))
	term.WriteString(out.String())
	assert.Equal(t, []string{"a界b"}, term.Lines())
}

func TestFullScreenDrawsImagesAndClearsThem(t *testing.T) {
	var out bytes.Buffer
	screen := NewScreen(&out, Options{Profile: cell.Plain})
	with := cell.NewBuffer(6, 2)
	with.Canvas(nil).Place(2, 1, 2, 1, "<IMG>", "<CLR>")
	require.NoError(t, screen.Draw(with))
	assert.Contains(t, out.String(), "\x1b[2;3H<IMG>", "the image is written at its region")

	out.Reset()
	require.NoError(t, screen.Draw(with))
	assert.NotContains(t, out.String(), "<IMG>", "an image that did not change is not sent again")

	out.Reset()
	require.NoError(t, screen.Draw(cell.NewBuffer(6, 2)))
	assert.Contains(t, out.String(), "<CLR>\x1b[2J", "a frame without the image takes it off the screen")

	require.NoError(t, screen.Draw(with))
	out.Reset()
	require.NoError(t, screen.Leave())
	assert.Contains(t, out.String(), "<CLR>")
}

func TestStylesFollowTheProfile(t *testing.T) {
	var out bytes.Buffer
	screen := NewScreen(&out, Options{Profile: cell.ANSI})
	require.NoError(t, screen.Draw(row("x", cell.Style{Fg: cell.RGB(255, 0, 0)}, 2)))
	assert.Contains(t, out.String(), "\x1b[91mx\x1b[m")
}

// An inline region draws below what the terminal shows, and what is printed above it scrolls into history.
func TestInlineRegionRedrawsInPlace(t *testing.T) {
	term := vt.New(12, 6)
	screen := NewScreen(term, Options{Inline: true, Profile: cell.Plain})
	term.WriteString("$ run\r\n")
	require.NoError(t, screen.Enter())

	require.NoError(t, screen.Draw(row("one\ntwo", cell.Style{}, 12)))
	assert.Equal(t, []string{"$ run", "one", "two"}, term.Lines())
	_, y := term.Cursor()
	assert.Equal(t, 1, y, "the cursor rests on the region's first row")

	require.NoError(t, screen.Draw(row("one\nTWO\nthree", cell.Style{}, 12)))
	assert.Equal(t, []string{"$ run", "one", "TWO", "three"}, term.Lines(), "a taller frame grows downward")

	require.NoError(t, screen.Draw(row("only", cell.Style{}, 12)))
	assert.Equal(t, []string{"$ run", "only"}, term.Lines(), "a shorter frame blanks the rows it gave back")

	require.NoError(t, screen.Print([]string{"log a", "log b"}))
	assert.Equal(t, []string{"$ run", "log a", "log b", "only"}, term.Lines(), "printed lines go above the region")
	require.NoError(t, screen.Print(nil))

	require.NoError(t, screen.Leave())
	assert.Equal(t, []string{"$ run", "log a", "log b"}, term.Lines(), "leaving takes the region off and keeps the log")
}

func TestInlineRegionScrollsTheTerminalWhenItRunsOutOfRows(t *testing.T) {
	term := vt.New(8, 3)
	screen := NewScreen(term, Options{Inline: true, Profile: cell.Plain})
	term.WriteString("a\r\nb\r\n")
	require.NoError(t, screen.Draw(row("r1\nr2", cell.Style{}, 8)))
	assert.Equal(t, []string{"a", "b", "r1", "r2"}, term.Lines())

	require.NoError(t, screen.Print([]string{"x", "y", "z"}))
	assert.Equal(t, []string{"a", "b", "x", "y", "z", "r1", "r2"}, term.Lines())

	require.NoError(t, screen.Draw(row("r1\nR2", cell.Style{}, 8)))
	assert.Equal(t, []string{"a", "b", "x", "y", "z", "r1", "R2"}, term.Lines())
}

func TestInlineRegionWritesOnlyChangedRows(t *testing.T) {
	var out bytes.Buffer
	screen := NewScreen(&out, Options{Inline: true, Profile: cell.Plain})
	require.NoError(t, screen.Draw(row("one\ntwo\nthree", cell.Style{}, 8)))

	out.Reset()
	require.NoError(t, screen.Draw(row("one\nTWO\nthree", cell.Style{}, 8)))
	assert.NotContains(t, out.String(), "one")
	assert.NotContains(t, out.String(), "three")
	assert.Contains(t, out.String(), "TWO\x1b[K")

	out.Reset()
	require.NoError(t, screen.Draw(row("12345678", cell.Style{}, 8)))
	assert.NotContains(t, out.String(), "12345678\x1b[K", "a row that fills the width is not erased after")
}

func TestInlineRegionPlacesImages(t *testing.T) {
	var out bytes.Buffer
	screen := NewScreen(&out, Options{Inline: true, Profile: cell.Plain})
	frame := cell.NewBuffer(6, 2)
	frame.Canvas(nil).Place(1, 1, 2, 1, "<IMG>", "<CLR>")
	require.NoError(t, screen.Draw(frame))
	assert.Contains(t, out.String(), "\x1b7\x1b[1B\x1b[2G<IMG>\x1b8")

	out.Reset()
	require.NoError(t, screen.Leave())
	assert.Contains(t, out.String(), "<CLR>")
}

func TestAPrintedLineCannotColourTheRegion(t *testing.T) {
	var out bytes.Buffer
	screen := NewScreen(&out, Options{Inline: true, Profile: cell.TrueColor})
	require.NoError(t, screen.Print([]string{"\x1b[31mred"}))
	assert.Contains(t, out.String(), "\x1b[31mred\x1b[m\r\n")
}

func TestAStreamThatIsNotATerminal(t *testing.T) {
	assert.False(t, IsTerminal(&bytes.Buffer{}))
	_, _, err := Size(&bytes.Buffer{})
	assert.Error(t, err)
	_, err = Raw(&bytes.Buffer{})
	assert.Error(t, err)

	file, err := os.CreateTemp(t.TempDir(), "plain")
	require.NoError(t, err)
	defer file.Close()
	assert.False(t, IsTerminal(file), "a file is not a terminal")
	_, _, err = Size(file)
	assert.Error(t, err)
	_, err = Raw(file)
	assert.Error(t, err)
	assert.NoError(t, EnableOutput(file)())
}

// Close has to end a read that is already waiting, or a program that ends would keep eating the user's keys.
func TestReaderCloseInterruptsABlockedRead(t *testing.T) {
	source, sink, err := os.Pipe()
	require.NoError(t, err)
	defer source.Close()
	defer sink.Close()

	reader, err := NewReader(source)
	require.NoError(t, err)

	_, err = sink.Write([]byte("hi"))
	require.NoError(t, err)
	buf := make([]byte, 8)
	n, err := reader.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, "hi", string(buf[:n]))

	done := make(chan error, 1)
	go func() {
		_, err := reader.Read(buf)
		done <- err
	}()
	require.NoError(t, reader.Close())
	require.NoError(t, reader.Close(), "closing again is harmless")
	assert.ErrorIs(t, <-done, io.EOF)
}

// Close has to end a blocked read when poll does not work, which is the path this reader falls back to there.
func TestReaderCloseInterruptsAReadWithoutPoll(t *testing.T) {
	source, sink, err := os.Pipe()
	require.NoError(t, err)
	defer source.Close()
	defer sink.Close()

	reader, err := NewReader(source)
	require.NoError(t, err)
	reader.(*pollReader).blocking = true

	_, err = sink.Write([]byte("hi"))
	require.NoError(t, err)
	buf := make([]byte, 8)
	n, err := reader.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, "hi", string(buf[:n]))

	done := make(chan error, 1)
	go func() {
		_, err := reader.Read(buf)
		done <- err
	}()
	require.NoError(t, reader.Close())
	assert.ErrorIs(t, <-done, io.EOF)
}

func TestReaderOverAPlainStream(t *testing.T) {
	reader, err := NewReader(strings.NewReader("abc"))
	require.NoError(t, err)
	buf := make([]byte, 8)
	n, err := reader.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, "abc", string(buf[:n]))
	assert.NoError(t, reader.Close())
}

func TestResizeNotificationsStop(t *testing.T) {
	stop := NotifyResize(make(chan os.Signal, 1))
	stop()
}
