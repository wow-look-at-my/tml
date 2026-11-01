package syntax

import _ "embed"

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// colOf is the column of needle on the given line, counted from its start.
func colOf(t *testing.T, src string, line int, needle string) int {
	t.Helper()
	lines := strings.Split(src, "\n")
	require.GreaterOrEqual(t, len(lines), line, "source has fewer than %d lines", line)
	idx := strings.Index(lines[line-1], needle)
	require.GreaterOrEqual(t, idx, 0, "%q is not on line %d", needle, line)
	return idx + 1
}

// requirePos asserts a diagnostic lands on the attribute rather than the element. Both checks matter: the element's
func requirePos(t *testing.T, err error, src string, line int, attr, element string) {
	t.Helper()
	var syntaxErr *Error
	require.ErrorAs(t, err, &syntaxErr)
	assert.Equal(t, line, syntaxErr.Pos.Line)
	assert.Equal(t, colOf(t, src, line, attr), syntaxErr.Pos.Col, "points at %s", attr)
	assert.NotEqual(t, colOf(t, src, line, element), syntaxErr.Pos.Col, "not at the owning element")
}

func TestUnknownAttributeIsReportedAtTheAttribute(t *testing.T) {
	src := header + testUnknownAttributeIsReportedAtTheAttributeXml

	_, err := Parse("test.tml", []byte(src))
	require.Error(t, err)
	assert.Contains(t, err.Error(), `<Property> has no attribute "bogus"`)
	requirePos(t, err, src, 3, `bogus=`, `<Property`)
}

func TestBadBooleanIsReportedAtTheAttribute(t *testing.T) {
	src := header + testBadBooleanIsReportedAtTheAttributeXml

	_, err := Parse("test.tml", []byte(src))
	require.Error(t, err)
	assert.Contains(t, err.Error(), `attribute "required" must be true or false`)
	requirePos(t, err, src, 3, `required=`, `<Property`)
}

// Template and style attributes keep their own positions on the way through, so the later passes that report against
func TestParsedAttributesCarryTheirOwnPositions(t *testing.T) {
	src := header + testParsedAttributesCarryTheirOwnPositionsXml

	file, err := Parse("test.tml", []byte(src))
	require.NoError(t, err)

	text := file.Component.Template.Elements()[0]
	require.Len(t, text.Attrs, 2)
	for _, attr := range text.Attrs {
		assert.Equal(t, 4, attr.Pos.Line)
		assert.Equal(t, colOf(t, src, 4, attr.Name+`=`), attr.Pos.Col, "attribute %q", attr.Name)
	}
	assert.NotEqual(t, text.Attrs[0].Pos.Col, text.Attrs[1].Pos.Col,
		"two attributes on one element are two different places")
}

func TestStyleAttributesCarryTheirOwnPositions(t *testing.T) {
	src := header + testStyleAttributesCarryTheirOwnPositionsXml

	file, err := Parse("test.tml", []byte(src))
	require.NoError(t, err)

	style := file.Theme.Styles[0]
	require.Len(t, style.Attrs, 2)
	for _, attr := range style.Attrs {
		assert.Equal(t, 3, attr.Pos.Line)
		assert.Equal(t, colOf(t, src, 3, attr.Name+`=`), attr.Pos.Col, "attribute %q", attr.Name)
	}
}

//go:embed testdata/test-unknown-attribute-is-reported-at-the-attribute.xml
var testUnknownAttributeIsReportedAtTheAttributeXml string

//go:embed testdata/test-bad-boolean-is-reported-at-the-attribute.xml
var testBadBooleanIsReportedAtTheAttributeXml string

//go:embed testdata/test-parsed-attributes-carry-their-own-positions.xml
var testParsedAttributesCarryTheirOwnPositionsXml string

//go:embed testdata/test-style-attributes-carry-their-own-positions.xml
var testStyleAttributesCarryTheirOwnPositionsXml string
