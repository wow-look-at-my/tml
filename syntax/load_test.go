package syntax

import (
	"os"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func component(name, body string) string {
	return header + `<Component xmlns="urn:tml:v1" name="` + name + `">` + body + `<Template/></Component>`
}

func TestLoadResolvesImportsRelativeToTheImporter(t *testing.T) {
	fsys := fstest.MapFS{
		"ui/app.tml":           {Data: []byte(component("App", `<Import src="./widgets/Card.tml"/>`))},
		"ui/widgets/Card.tml":  {Data: []byte(component("Card", `<Import src="./Badge.tml"/>`))},
		"ui/widgets/Badge.tml": {Data: []byte(component("Badge", ``))},
	}

	unit, err := Load(fsys, "ui/app.tml")
	require.NoError(t, err)

	assert.Len(t, unit.Files, 3, "every reachable file is loaded")

	_, ok := unit.Lookup("ui/app.tml", "Card")
	assert.True(t, ok, "an imported component is in scope")

	_, ok = unit.Lookup("ui/app.tml", "Badge")
	assert.False(t, ok, "imports are not transitive; App never imported Badge")

	_, ok = unit.Lookup("ui/widgets/Card.tml", "Badge")
	assert.True(t, ok, "but Card imported it directly")
}

func TestLoadPutsHelpersAndSelfInScope(t *testing.T) {
	fsys := fstest.MapFS{
		"app.tml": {Data: []byte(header + `<Component xmlns="urn:tml:v1" name="App">
	<DataTemplate name="Row"><Template/></DataTemplate>
	<Template/>
</Component>`)},
	}

	unit, err := Load(fsys, "app.tml")
	require.NoError(t, err)

	assert.Equal(t, []string{"App", "Row"}, unit.InScope("app.tml"),
		"a component sees itself and its data templates")
}

// A file defines exactly a single component. A component nested under the root is another definition, and it is refused by
func TestLoadRefusesTwoComponentsInOneFile(t *testing.T) {
	src, err := os.ReadFile("testdata/two-components.tml")
	require.NoError(t, err)
	fsys := fstest.MapFS{
		"app.tml": {Data: src},
	}

	_, err = Load(fsys, "app.tml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "app.tml")
	assert.Contains(t, err.Error(), `"App"`)
	assert.Contains(t, err.Error(), `"Row"`)
	assert.Contains(t, err.Error(), "one component per file")
}

// An import cycle only makes names mutually visible, so loading must terminate rather than fail. Self-instantiation is
func TestLoadTerminatesOnAnImportCycle(t *testing.T) {
	fsys := fstest.MapFS{
		"a.tml": {Data: []byte(component("A", `<Import src="./b.tml"/>`))},
		"b.tml": {Data: []byte(component("B", `<Import src="./a.tml"/>`))},
	}

	unit, err := Load(fsys, "a.tml")
	require.NoError(t, err)
	assert.Len(t, unit.Files, 2)

	_, ok := unit.Lookup("a.tml", "B")
	assert.True(t, ok)
}

func TestLoadCollectsThemes(t *testing.T) {
	fsys := fstest.MapFS{
		"app.tml": {Data: []byte(component("App", `<Import src="./theme.tml"/>`))},
		"theme.tml": {Data: []byte(header + `<Theme xmlns="urn:tml:v1" name="default">
	<Token name="accent" value="#f00"/>
</Theme>`)},
	}

	unit, err := Load(fsys, "app.tml")
	require.NoError(t, err)

	require.Len(t, unit.Themes, 1)
	assert.Equal(t, "default", unit.Themes[0].Name)
	assert.Equal(t, []string{"App"}, unit.InScope("app.tml"),
		"a theme import contributes no component name, so only App itself is in scope")
}

func TestLoadRejects(t *testing.T) {
	t.Run("a missing import names the importer's position", func(t *testing.T) {
		fsys := fstest.MapFS{
			"app.tml": {Data: []byte(component("App", `<Import src="./nope.tml"/>`))},
		}
		_, err := Load(fsys, "app.tml")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot read nope.tml")
		assert.Contains(t, err.Error(), "app.tml:", "the diagnostic points at the import, not the missing file")
	})

	t.Run("an import cannot escape the project root", func(t *testing.T) {
		fsys := fstest.MapFS{
			"app.tml": {Data: []byte(component("App", `<Import src="../../etc/passwd.tml"/>`))},
		}
		_, err := Load(fsys, "app.tml")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot escape")
	})

	t.Run("two imported components cannot share a name in one scope", func(t *testing.T) {
		// The old shape for this clash was a nested definition, which the only-component rule now refuses earlier. Duplicate
		fsys := fstest.MapFS{
			"app.tml":        {Data: []byte(component("App", `<Import src="./card.tml"/><Import src="./other/card.tml"/>`))},
			"card.tml":       {Data: []byte(component("Card", ``))},
			"other/card.tml": {Data: []byte(component("Card", ``))},
		}
		_, err := Load(fsys, "app.tml")
		require.Error(t, err)
		assert.Contains(t, err.Error(), `component "Card" is already in scope`)
	})
}
