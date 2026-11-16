package widgets

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"math/rand"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml/cell"
	"github.com/wow-look-at-my/tml/widget"
	"github.com/wow-look-at-my/tml/widget/widgettest"
)

// encode writes an image as a PNG.
func encode(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

// checkerboard is a picture with known colours in known places.
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

// picture builds an Image widget over a filesystem holding a single file.
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

	buf := widgettest.Draw(img, 4, 2)
	assert.Equal(t, "▀▀▀▀\n▀▀▀▀", buf.Text(), "each cell carries two pixels in one glyph")
	assert.Empty(t, buf.Placements, "a mosaic is cells, not an escape")
	for y := range 2 {
		for x := range 4 {
			style := buf.At(x, y).Style
			assert.True(t, style.Fg.IsSet() && style.Bg.IsSet(), "the pixels are colour, cell %d,%d", x, y)
		}
	}
}

// The glyph's colour is the upper pixel and its fill is the lower one.
func TestMosaicPaintsTheUpperPixelOverTheLower(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 0xff, A: 0xff})
	img.Set(1, 0, color.RGBA{G: 0xff, A: 0xff})
	img.Set(0, 1, color.RGBA{B: 0xff, A: 0xff})
	img.Set(1, 1, color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff})
	pic := picture(t, "four.png", encode(t, img), map[string]string{"protocol": "mosaic"}, true)

	buf := widgettest.Draw(pic, 2, 1)
	assert.Equal(t, cell.Style{Fg: cell.RGB(255, 0, 0), Bg: cell.RGB(0, 0, 255)}, buf.At(0, 0).Style)
	assert.Equal(t, cell.Style{Fg: cell.RGB(0, 255, 0), Bg: cell.RGB(255, 255, 255)}, buf.At(1, 0).Style)
}

// The shape survives: a wide image is not stretched into a square.
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

// The Kitty and iTerm2 escapes are instructions to the terminal, not text. They are a placement over the region, and
// the cells under it stay blank.
func TestGraphicsProtocolsReserveTheirCells(t *testing.T) {
	data := checkerboard(t, 8, 8)

	for _, protocol := range []string{"kitty", "iterm"} {
		t.Run(protocol, func(t *testing.T) {
			img := picture(t, "logo.png", data, map[string]string{"protocol": protocol}, true)

			buf := widgettest.Draw(img, 6, 3)
			assert.Equal(t, "      \n      \n      ", buf.Text(), "there is no text, only the reservation")
			require.Len(t, buf.Placements, 1, "and the escape really was emitted")
			assert.Equal(t, cell.Rect{W: 6, H: 3}, buf.Placements[0].Rect, "over the whole region")
		})
	}
}

func TestKittyAndITermEmitTheirOwnProtocols(t *testing.T) {
	data := checkerboard(t, 4, 4)

	kitty := widgettest.Draw(picture(t, "logo.png", data, map[string]string{"protocol": "kitty"}, true), 4, 2)
	require.Len(t, kitty.Placements, 1)
	seq := kitty.Placements[0].Seq
	assert.True(t, strings.HasPrefix(seq, "\x1b_Ga=T,f=100,t=d,c=4,r=2,C=1,q=2,m=0;"),
		"transmit and display in one, sized in cells so the terminal does the scaling: %q", seq[:min(len(seq), 60)])
	assert.True(t, strings.HasSuffix(seq, "\x1b\\"))
	assert.Equal(t, kittyClear, kitty.Placements[0].Clear, "a Kitty image stays on screen until it is deleted")

	iterm := widgettest.Draw(picture(t, "logo.png", data, map[string]string{"protocol": "iterm"}, true), 4, 2)
	require.Len(t, iterm.Placements, 1)
	seq = iterm.Placements[0].Seq
	name := base64.StdEncoding.EncodeToString([]byte("logo.png"))
	assert.Regexp(t, regexp.MustCompile(`^\x1b\]1337;File=name=`+regexp.QuoteMeta(name)+
		`;size=\d+;width=4;height=2;preserveAspectRatio=0;inline=1;doNotMoveCursor=1:[A-Za-z0-9+/=]+\a$`), seq)
	assert.Empty(t, iterm.Placements[0].Clear, "text drawn over an iTerm image is enough to remove it")
}

// The protocol limits what one graphics escape carries, so a large picture goes in chunks. Only the first carries
// the keys, and every chunk but the last says more follows.
func TestKittySendsALargePictureInChunks(t *testing.T) {
	noise := image.NewRGBA(image.Rect(0, 0, 64, 64))
	random := rand.New(rand.NewSource(1))
	random.Read(noise.Pix)
	for i := 3; i < len(noise.Pix); i += 4 {
		noise.Pix[i] = 0xff
	}
	data := encode(t, noise)
	pic := picture(t, "noise.png", data, map[string]string{"protocol": "kitty"}, true)

	buf := widgettest.Draw(pic, 8, 4)
	require.Len(t, buf.Placements, 1)
	chunks := strings.SplitAfter(buf.Placements[0].Seq, "\x1b\\")
	chunks = chunks[:len(chunks)-1]
	require.Greater(t, len(chunks), 1, "this picture does not fit in one escape")

	var payload strings.Builder
	for i, chunk := range chunks {
		keys, body, ok := strings.Cut(strings.TrimSuffix(strings.TrimPrefix(chunk, "\x1b_G"), "\x1b\\"), ";")
		require.True(t, ok, "chunk %d", i)
		assert.LessOrEqual(t, len(body), kittyChunk, "chunk %d", i)
		more := "m=1"
		if i == len(chunks)-1 {
			more = "m=0"
		}
		if i == 0 {
			assert.Equal(t, "a=T,f=100,t=d,c=8,r=4,C=1,q=2,"+more, keys)
		} else {
			assert.Equal(t, more, keys, "chunk %d carries no keys", i)
		}
		payload.WriteString(body)
	}
	decoded, err := base64.StdEncoding.DecodeString(payload.String())
	require.NoError(t, err)
	got, err := png.Decode(bytes.NewReader(decoded))
	require.NoError(t, err)
	assert.Equal(t, noise.Bounds(), got.Bounds(), "the chunks join back into the picture")
}

// A terminal cannot draw part of an inline image, so a placement the clip cuts into is left off.
func TestAClippedImageIsLeftOff(t *testing.T) {
	pic := picture(t, "logo.png", checkerboard(t, 4, 4), map[string]string{"protocol": "kitty"}, true)

	buf := cell.NewBuffer(2, 1)
	pic.Draw(buf.Canvas(nil).Sub(0, 0, 4, 2))
	assert.Empty(t, buf.Placements)
}

// The last resort needs no pixels at all, which is what makes it always available: it does not even read the file.
func TestLinkFallbackWritesAHyperlink(t *testing.T) {
	factory, ok := Library().Factory("Image")
	require.True(t, ok)
	native, err := factory.Build(widget.Context{
		Attrs: attrsOf("Image", map[string]string{
			"src": "https://example.invalid/logo.png", "protocol": "link", "alt": "the logo",
		}),
	})
	require.NoError(t, err)

	buf := widgettest.Draw(native, 20, 1)
	assert.Equal(t, "the logo", strings.TrimRight(buf.Text(), " "))
	for x := range len("the logo") {
		assert.Equal(t, "https://example.invalid/logo.png", buf.At(x, 0).Style.Link, "column %d", x)
	}
	assert.Empty(t, buf.At(9, 0).Style.Link, "the link ends with the text")
	assert.Empty(t, buf.Placements)
}

func TestLinkFallbackNamesALocalFile(t *testing.T) {
	factory, _ := Library().Factory("Image")
	native, err := factory.Build(widget.Context{
		Attrs: attrsOf("Image", map[string]string{"src": "/tmp/logo.png", "protocol": "link"}),
	})
	require.NoError(t, err)

	buf := widgettest.Draw(native, 20, 1)
	assert.Equal(t, "file:///tmp/logo.png", buf.At(0, 0).Style.Link)
	assert.Equal(t, "logo.png", strings.TrimRight(buf.Text(), " "), "the file's name stands in for missing alt text")

	w, h := native.Measure(0, 0)
	assert.Equal(t, 8, w, "a link is as wide as its text")
	assert.Equal(t, 1, h)
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

// A transparent pixel is blended onto the theme's background, so a logo with a clear surround does not come out ringed
func TestTransparencyBlendsOntoTheTheme(t *testing.T) {
	transparent := encode(t, image.NewRGBA(image.Rect(0, 0, 2, 2)))

	dark := widgettest.Draw(picture(t, "clear.png", transparent, map[string]string{"protocol": "mosaic"}, true), 1, 1)
	light := widgettest.Draw(picture(t, "clear.png", transparent, map[string]string{"protocol": "mosaic"}, false), 1, 1)

	black, white := cell.RGB(0, 0, 0), cell.RGB(255, 255, 255)
	assert.Equal(t, cell.Style{Fg: black, Bg: black}, dark.At(0, 0).Style, "a clear pixel is the dark theme's own background")
	assert.Equal(t, cell.Style{Fg: white, Bg: white}, light.At(0, 0).Style)
}

// A picture that cannot be read or cannot be decoded is a mistake in the template. Drawing a blank rectangle instead
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
	for _, protocol := range []string{"kitty", "iterm", "mosaic"} {
		img := picture(t, "logo.png", checkerboard(t, 4, 4), map[string]string{"protocol": protocol}, true)

		for _, size := range [][2]int{{0, 2}, {4, 0}} {
			buf := cell.NewBuffer(4, 2)
			img.Draw(buf.Canvas(nil).Sub(0, 0, size[0], size[1]))
			assert.Empty(t, buf.Placements, "%s at %v", protocol, size)
			assert.Equal(t, "    \n    ", buf.Text(), "%s at %v", protocol, size)
		}
	}
}
