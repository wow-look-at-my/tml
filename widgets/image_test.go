package widgets

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
	"testing/fstest"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml/widget"
)

// checkerboard is a picture with known colours in known places, so a test can
// say where a colour ended up rather than only that the output is non-empty.
func checkerboard(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			shade := color.RGBA{R: 0, G: 0, B: 0, A: 0xff}
			if (x+y)%2 == 0 {
				shade = color.RGBA{R: 0xff, G: 0, B: 0, A: 0xff}
			}
			img.Set(x, y, shade)
		}
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

// picture builds an Image widget over a filesystem holding one file.
func picture(t *testing.T, name string, data []byte, attrs map[string]string, dark bool) widget.Native {
	t.Helper()
	native, err := pictureOrError(t, name, data, attrs, dark)
	require.NoError(t, err)
	return native
}

func pictureOrError(t *testing.T, name string, data []byte, attrs map[string]string, dark bool) (widget.Native, error) {
	t.Helper()
	factory, ok := Library().Factory("Image")
	require.True(t, ok)

	values := map[string]string{"src": name}
	for k, v := range attrs {
		values[k] = v
	}
	return factory.Build(widget.Context{
		Attrs: attrsOf("Image", values),
		FS:    fstest.MapFS{name: &fstest.MapFile{Data: data}},
		Dark:  dark,
	})
}

func TestImageDrawsAMosaicOfHalfBlocks(t *testing.T) {
	img := picture(t, "logo.png", checkerboard(t, 8, 8), map[string]string{"protocol": "mosaic"}, true)

	out := img.Render(4, 2)
	assert.Equal(t, 4, lipgloss.Width(out), "one cell per column")
	assert.Equal(t, 2, lipgloss.Height(out))
	assert.Equal(t, 8, strings.Count(out, "▀"), "each cell carries two pixels in one glyph")
	assert.Contains(t, out, "\x1b[", "the pixels are colour, so there is styling")
}

// The shape survives: a wide image is not stretched into a square, allowing for
// a terminal cell being about twice as tall as it is wide.
func TestImageKeepsItsShape(t *testing.T) {
	wide := picture(t, "wide.png", checkerboard(t, 40, 10), nil, true)

	w, h := wide.Measure(20, 0)
	assert.Equal(t, 20, w)
	assert.Equal(t, 2, h, "ten rows of pixels over forty, halved again for the cell's shape")
}

func TestImageFitsAHeightLimit(t *testing.T) {
	tall := picture(t, "tall.png", checkerboard(t, 10, 40), nil, true)

	w, h := tall.Measure(20, 4)
	assert.Equal(t, 4, h, "the limit wins")
	assert.LessOrEqual(t, w, 2, "and the width comes down with it rather than stretching")
}

// The Kitty and iTerm2 escapes are instructions to the terminal, not text, so
// they measure zero. The blanks after them are what reserve the cells the image
// will cover -- without which everything downstream would compose around a
// footprint of nothing.
func TestGraphicsProtocolsReserveTheirCells(t *testing.T) {
	data := checkerboard(t, 8, 8)

	for _, protocol := range []string{"kitty", "iterm"} {
		t.Run(protocol, func(t *testing.T) {
			img := picture(t, "logo.png", data, map[string]string{"protocol": protocol}, true)

			out := img.Render(6, 3)
			assert.Equal(t, 6, lipgloss.Width(out), "the escape is zero cells wide")
			assert.Equal(t, 3, lipgloss.Height(out))
			assert.Empty(t, strings.TrimSpace(ansi.Strip(out)), "there is no text, only the reservation")
			assert.Contains(t, out, "\x1b", "and the escape really was emitted")
		})
	}
}

func TestKittyAndITermEmitTheirOwnProtocols(t *testing.T) {
	data := checkerboard(t, 4, 4)

	kitty := picture(t, "logo.png", data, map[string]string{"protocol": "kitty"}, true).Render(4, 2)
	assert.Contains(t, kitty, "\x1b_G", "the Kitty graphics introducer")
	assert.Contains(t, kitty, "a=T", "transmit and display")
	assert.Contains(t, kitty, "c=4", "sized in cells, so the terminal does the scaling")
	assert.Contains(t, kitty, "r=2")

	iterm := picture(t, "logo.png", data, map[string]string{"protocol": "iterm"}, true).Render(4, 2)
	assert.Contains(t, iterm, "\x1b]1337;File=", "the iTerm2 file introducer")
	assert.Contains(t, iterm, "inline=1")
	assert.Contains(t, iterm, "width=4")
	assert.Contains(t, iterm, "height=2")
}

// The last resort needs no pixels at all, which is what makes it always
// available: it does not even read the file.
func TestLinkFallbackWritesAHyperlink(t *testing.T) {
	factory, ok := Library().Factory("Image")
	require.True(t, ok)
	native, err := factory.Build(widget.Context{
		Attrs: attrsOf("Image", map[string]string{
			"src": "https://example.invalid/logo.png", "protocol": "link", "alt": "the logo",
		}),
	})
	require.NoError(t, err)

	out := native.Render(20, 1)
	assert.Contains(t, out, "the logo")
	assert.Contains(t, out, "\x1b]8;;https://example.invalid/logo.png")
	assert.Equal(t, "the logo", ansi.Strip(out))
}

func TestLinkFallbackNamesALocalFile(t *testing.T) {
	factory, _ := Library().Factory("Image")
	native, err := factory.Build(widget.Context{
		Attrs: attrsOf("Image", map[string]string{"src": "/tmp/logo.png", "protocol": "link"}),
	})
	require.NoError(t, err)

	out := native.Render(20, 1)
	assert.Contains(t, out, "file:///tmp/logo.png")
	assert.Equal(t, "logo.png", ansi.Strip(out), "the file's name stands in for missing alt text")
}

func TestProtocolDetection(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want string
	}{
		{"kitty by TERM", map[string]string{"TERM": "xterm-kitty"}, "kitty"},
		{"kitty by window id", map[string]string{"KITTY_WINDOW_ID": "1"}, "kitty"},
		{"ghostty", map[string]string{"TERM_PROGRAM": "ghostty"}, "kitty"},
		{"wezterm", map[string]string{"TERM_PROGRAM": "WezTerm"}, "kitty"},
		{"iterm2", map[string]string{"TERM_PROGRAM": "iTerm.app"}, "iterm"},
		{"iterm2 over ssh", map[string]string{"LC_TERMINAL": "iTerm2"}, "iterm"},
		{"anything else", map[string]string{"TERM": "xterm-256color"}, "mosaic"},
		{"nothing at all", nil, "mosaic"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, detectProtocol(func(key string) string { return tc.env[key] }))
		})
	}
}

// A transparent pixel is blended onto the theme's background, so a logo with a
// clear surround does not come out ringed in black on a light terminal.
func TestTransparencyBlendsOntoTheTheme(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))

	dark := picture(t, "clear.png", buf.Bytes(), map[string]string{"protocol": "mosaic"}, true).Render(1, 1)
	light := picture(t, "clear.png", buf.Bytes(), map[string]string{"protocol": "mosaic"}, false).Render(1, 1)

	assert.Contains(t, dark, "0;0;0", "a clear pixel is the dark theme's own background")
	assert.Contains(t, light, "255;255;255")
}

// A picture that cannot be read or cannot be decoded is a mistake in the
// template. Drawing a blank rectangle instead would leave the author hunting for
// why nothing appeared.
func TestImageFailuresAreReported(t *testing.T) {
	_, err := pictureOrError(t, "logo.png", checkerboard(t, 2, 2), map[string]string{"src": "missing.png"}, true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing.png")

	_, err = pictureOrError(t, "broken.png", []byte("not a picture"), nil, true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "broken.png")

	_, err = pictureOrError(t, "logo.png", checkerboard(t, 2, 2), map[string]string{"src": "../secrets.png"}, true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "escapes the view's filesystem")
}

func TestImageRequiresASource(t *testing.T) {
	factory, _ := Library().Factory("Image")
	_, err := factory.Build(widget.Context{Attrs: attrsOf("Image", nil)})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires a src attribute")
}

func TestImageRejectsAnUnknownProtocol(t *testing.T) {
	_, err := pictureOrError(t, "logo.png", checkerboard(t, 2, 2), map[string]string{"protocol": "sixel"}, true)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected one of auto, iterm, kitty, link, mosaic")
}

func TestImageDrawsNothingInNoSpace(t *testing.T) {
	img := picture(t, "logo.png", checkerboard(t, 4, 4), nil, true)

	assert.Empty(t, img.Render(0, 2))
	assert.Empty(t, img.Render(4, 0))
}
