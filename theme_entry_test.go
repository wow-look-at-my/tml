package tml_test

import (
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml"
)

const themeXMLHeader = "<?xml version=\"1.1\" encoding=\"UTF-8\"?>\n"

// A theme has no properties and nothing to instantiate. Loading a theme on its own -- rather than only when some component
func TestAThemeEntryLoadsOnItsOwn(t *testing.T) {
	fsys := fstest.MapFS{"theme.tml": &fstest.MapFile{Data: []byte(themeXMLHeader + `
<Theme xmlns="urn:tml:v1" name="slh">
	<Token name="accent" light="#000" dark="#fff"/>
	<Style name="card" fg="{theme.accent}"/>
</Theme>`)}}
	_, err := tml.Load(fsys, "theme.tml", tml.Options{})
	assert.NoError(t, err)
}

// An extends chain that cycles is a defect in the theme itself, and a theme entry is exactly where it should surface
func TestAThemeEntryStillCatchesACyclicExtends(t *testing.T) {
	fsys := fstest.MapFS{"theme.tml": &fstest.MapFile{Data: []byte(themeXMLHeader + `
<Theme xmlns="urn:tml:v1" name="slh">
	<Style name="a" extends="b"/>
	<Style name="b" extends="a"/>
</Theme>`)}}
	_, err := tml.Load(fsys, "theme.tml", tml.Options{})
	require.Error(t, err)
	assert.ErrorContains(t, err, "extends itself")
}

// A theme entry declares no component, so there is no root to instantiate: the refusal has to name that plainly rather
func TestAThemeEntryRefusesToExpand(t *testing.T) {
	fsys := fstest.MapFS{"theme.tml": &fstest.MapFile{Data: []byte(themeXMLHeader + `
<Theme xmlns="urn:tml:v1" name="slh">
	<Token name="accent" value="1"/>
</Theme>`)}}
	view, err := tml.Load(fsys, "theme.tml", tml.Options{})
	require.NoError(t, err)
	_, err = view.Expand(nil)
	assert.ErrorContains(t, err, "declares a theme, not a component")
}
