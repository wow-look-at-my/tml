package widgets

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBadgePadsItsLabel(t *testing.T) {
	badge := build(t, "Badge", map[string]string{"label": "new"})

	w, h := badge.Measure(0, 0)
	assert.Equal(t, 5, w)
	assert.Equal(t, 1, h)
	assert.Equal(t, " new ", badge.Render(5, 1))
}
