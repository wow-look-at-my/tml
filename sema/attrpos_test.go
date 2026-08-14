package sema

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml/syntax"
)

// The faulty attribute is never the first one on its element, so a diagnostic
// that still pointed at the element would land in a visibly different column and
// fail these tests. Asserting only the message would not.
const attrPosApp = `<Component xmlns="urn:tml:v1" name="App">
	<Property name="count" type="int" default="1"/>
	<Template>
		<Text style="a" width="%s">hi</Text>
	</Template>
</Component>`

const attrPosTextLine = 5

func colOf(t *testing.T, src string, line int, needle string) int {
	t.Helper()
	lines := strings.Split(src, "\n")
	require.GreaterOrEqual(t, len(lines), line, "source has fewer than %d lines", line)
	idx := strings.Index(lines[line-1], needle)
	require.GreaterOrEqual(t, idx, 0, "%q is not on line %d", needle, line)
	return idx + 1
}

func requireAttrPos(t *testing.T, err error, src string, line int, attr, element string) {
	t.Helper()
	var syntaxErr *syntax.Error
	require.ErrorAs(t, err, &syntaxErr)
	assert.Equal(t, line, syntaxErr.Pos.Line)
	assert.Equal(t, colOf(t, src, line, attr), syntaxErr.Pos.Col, "points at %s", attr)
	assert.NotEqual(t, colOf(t, src, line, element), syntaxErr.Pos.Col, "not at the owning element")
}

func TestExpressionParseFailureIsReportedAtTheAttribute(t *testing.T) {
	body := fmt.Sprintf(attrPosApp, "{a b}")
	_, err := build(t, map[string]string{"app.tml": body})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `attribute "width"`)
	requireAttrPos(t, err, xmlHeader+body, attrPosTextLine, `width=`, `<Text`)
}

func TestUnknownReferenceIsReportedAtTheAttribute(t *testing.T) {
	body := fmt.Sprintf(attrPosApp, "{nope}")
	_, err := build(t, map[string]string{"app.tml": body})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `unknown reference "nope"`)
	requireAttrPos(t, err, xmlHeader+body, attrPosTextLine, `width=`, `<Text`)
}

// `not` on an int survives analysis -- the name is in scope and the type is only
// known once a value arrives -- so this is the evaluation-time path.
func TestEvalFailureOnNativeIsReportedAtTheAttribute(t *testing.T) {
	body := fmt.Sprintf(attrPosApp, "{not count}")
	_, err := expand(t, map[string]string{"app.tml": body}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `attribute "width": cannot negate {not count}`)
	requireAttrPos(t, err, xmlHeader+body, attrPosTextLine, `width=`, `<Text`)
}

func TestEvalFailureOnComponentInstanceIsReportedAtTheAttribute(t *testing.T) {
	app := `<Component xmlns="urn:tml:v1" name="App">
	<Import src="./card.tml"/>
	<Property name="count" type="int" default="1"/>
	<Template>
		<Card tone="x" label="{not count}"/>
	</Template>
</Component>`
	card := `<Component xmlns="urn:tml:v1" name="Card">
	<Property name="tone" type="string" default=""/>
	<Property name="label" type="string" default=""/>
	<Template>
		<Text>{label}</Text>
	</Template>
</Component>`

	_, err := expand(t, map[string]string{"app.tml": app, "card.tml": card}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `attribute "label": cannot negate {not count}`)
	requireAttrPos(t, err, xmlHeader+app, 6, `label=`, `<Card`)
}

// An element-scoped diagnostic must not drift onto an attribute: an unknown
// element is a problem with the element itself.
func TestUnknownElementStaysOnTheElement(t *testing.T) {
	body := `<Component xmlns="urn:tml:v1" name="App">
	<Template>
		<Dock side="top"/>
	</Template>
</Component>`
	_, err := build(t, map[string]string{"app.tml": body})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown element <Dock>")

	var syntaxErr *syntax.Error
	require.ErrorAs(t, err, &syntaxErr)
	src := xmlHeader + body
	assert.Equal(t, 4, syntaxErr.Pos.Line)
	assert.Equal(t, colOf(t, src, 4, `<Dock`), syntaxErr.Pos.Col)
}
