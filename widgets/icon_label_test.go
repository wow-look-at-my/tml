package widgets

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml/cell"
	"github.com/wow-look-at-my/tml/widget"
	"github.com/wow-look-at-my/tml/widget/widgettest"
)

// iconLabelOrError builds an IconLabel over a filesystem holding one icon file.
func iconLabelOrError(t *testing.T, attrs map[string]string) (widget.Native, error) {
	t.Helper()
	factory, ok := Library().Factory("IconLabel")
	require.True(t, ok)
	return factory.Build(widget.Context{
		Attrs: attrsOf("IconLabel", attrs),
		FS:    fstest.MapFS{"mark.png": &fstest.MapFile{Data: checkerboard(t, 8, 8)}},
		Dark:  true,
	})
}

func iconLabelOf(t *testing.T, attrs map[string]string) widget.Native {
	t.Helper()
	native, err := iconLabelOrError(t, attrs)
	require.NoError(t, err)
	return native
}

func TestIconLabelWithNoIconIsItsLabel(t *testing.T) {
	label := iconLabelOf(t, map[string]string{"label": "Anthropic"})

	w, h := label.Measure(0, 0)
	assert.Equal(t, 9, w)
	assert.Equal(t, 1, h)
	assert.Equal(t, "Anthropic", widgettest.Text(label, 9, 1))
}

func TestIconLabelPutsTheIconBeforeTheLabel(t *testing.T) {
	label := iconLabelOf(t, map[string]string{"icon": "mark.png", "label": "Anthropic", "protocol": "mosaic"})

	w, h := label.Measure(0, 0)
	assert.Equal(t, 12, w, "two cells of icon, one of gap, then the label")
	assert.Equal(t, 1, h)

	buf := widgettest.Draw(label, 12, 1)
	assert.Equal(t, "▀▀ Anthropic", buf.Text())
	assert.True(t, buf.At(0, 0).Style.Fg.IsSet(), "the icon cells carry the picture's colour")
	assert.True(t, buf.At(3, 0).Style.IsZero(), "the label's colour is the element's own")
}

// A graphics protocol draws the icon as a placement over the cells the layout reserved for it.
func TestIconLabelPlacesAPixelIconOverItsCells(t *testing.T) {
	for _, protocol := range []string{"kitty", "iterm"} {
		t.Run(protocol, func(t *testing.T) {
			label := iconLabelOf(t, map[string]string{"icon": "mark.png", "label": "Groq", "protocol": protocol})

			buf := widgettest.Draw(label, 7, 1)
			assert.Equal(t, "   Groq", buf.Text())
			require.Len(t, buf.Placements, 1)
			assert.Equal(t, cell.Rect{W: 2, H: 1}, buf.Placements[0].Rect)
		})
	}
}

func TestIconLabelTakesItsIconWidthAndGap(t *testing.T) {
	label := iconLabelOf(t, map[string]string{
		"icon": "mark.png", "label": "xAI", "protocol": "mosaic", "iconWidth": "4", "gap": "2",
	})

	w, _ := label.Measure(0, 0)
	assert.Equal(t, 9, w)
	assert.Equal(t, "▀▀▀▀  xAI", widgettest.Text(label, 9, 1))
}

// A row too narrow for both keeps the name, because the name is what the reader needs.
func TestIconLabelDropsTheIconWhenTheRowIsTooNarrow(t *testing.T) {
	label := iconLabelOf(t, map[string]string{"icon": "mark.png", "label": "Anthropic", "protocol": "kitty"})

	buf := widgettest.Draw(label, 10, 1)
	assert.Equal(t, "Anthropic", strings.TrimRight(buf.Text(), " "))
	assert.Empty(t, buf.Placements)
}

func TestIconLabelWithNoLabelIsItsIcon(t *testing.T) {
	label := iconLabelOf(t, map[string]string{"icon": "mark.png", "protocol": "mosaic"})

	w, _ := label.Measure(0, 0)
	assert.Equal(t, 2, w, "no gap follows an icon with nothing after it")
}

// The label is already the text a link shows, so the link protocol reads no file and draws no icon.
func TestIconLabelDrawsNoIconAsALink(t *testing.T) {
	label := iconLabelOf(t, map[string]string{"icon": "absent.png", "label": "Mistral", "protocol": "link"})

	w, _ := label.Measure(0, 0)
	assert.Equal(t, 7, w)
	assert.Equal(t, "Mistral", widgettest.Text(label, 7, 1))
}

func TestIconLabelRejectsBadAttributes(t *testing.T) {
	_, err := iconLabelOrError(t, map[string]string{"icon": "absent.png", "label": "x", "protocol": "mosaic"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `<IconLabel> icon "absent.png"`)

	_, err = iconLabelOrError(t, map[string]string{"label": "x", "iconWidth": "0"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "iconWidth")

	_, err = iconLabelOrError(t, map[string]string{"label": "x", "gap": "-1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "gap")

	_, err = iconLabelOrError(t, map[string]string{"icon": "mark.png", "label": "x", "protocol": "sixel"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "protocol")
}
