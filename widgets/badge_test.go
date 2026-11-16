package widgets

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wow-look-at-my/tml/widget/widgettest"
)

func TestBadgePadsItsLabel(t *testing.T) {
	badge := build(t, "Badge", map[string]string{"label": "new"})

	w, h := badge.Measure(0, 0)
	assert.Equal(t, 5, w)
	assert.Equal(t, 1, h)
	assert.Equal(t, " new ", widgettest.Text(badge, 5, 1))
}

// Its colours are the element's own, so the badge paints its label plain.
func TestBadgeDrawsItsLabelUnstyled(t *testing.T) {
	buf := widgettest.Draw(build(t, "Badge", map[string]string{"label": "ok"}), 0, 0)

	for x := range buf.W {
		assert.True(t, buf.At(x, 0).Style.IsZero(), "cell %d", x)
	}
}
