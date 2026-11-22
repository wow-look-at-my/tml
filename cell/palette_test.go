package cell

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPaletteResolvesAReferenceAndNothingElse(t *testing.T) {
	palette := Palette{"accent": RGB(1, 2, 3), "dim": Indexed(8)}

	color, err := palette.Resolve(Ref("accent"))
	require.NoError(t, err)
	assert.Equal(t, RGB(1, 2, 3), color)

	color, err = palette.Named("dim")
	require.NoError(t, err)
	assert.Equal(t, Indexed(8), color)

	for _, literal := range []string{"#010203", "8", "red", "accent", "theme:"} {
		_, err := palette.Resolve(literal)
		require.ErrorContains(t, err, "written in place", literal)
	}

	_, err = palette.Resolve(Ref("nope"))
	require.ErrorContains(t, err, `the theme declares no colour "nope"; it has accent, dim`)
	assert.Equal(t, []string{"accent", "dim"}, palette.Names())
}

func TestRefNameReadsBackWhatRefWrote(t *testing.T) {
	name, ok := RefName(Ref("accent"))
	assert.True(t, ok)
	assert.Equal(t, "accent", name)

	_, ok = RefName("#ff0000")
	assert.False(t, ok)
}
