package sema

import _ "embed"

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml/syntax"
)

// The faulty attribute is never the leading on its element, so a diagnostic that still pointed at the element would
//
//go:embed testdata/attr-pos-app.xml
var attrPosApp string

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

// `not` on an int survives analysis -- the name is in scope and the type is only known as soon as a value arrives -- so this
func TestEvalFailureOnNativeIsReportedAtTheAttribute(t *testing.T) {
	body := fmt.Sprintf(attrPosApp, "{not count}")
	_, err := expand(t, map[string]string{"app.tml": body}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `attribute "width": cannot negate {not count}`)
	requireAttrPos(t, err, xmlHeader+body, attrPosTextLine, `width=`, `<Text`)
}

func TestEvalFailureOnComponentInstanceIsReportedAtTheAttribute(t *testing.T) {
	app := testEvalFailureOnComponentInstanceIsReportedAtTheAttributeXml
	card := testEvalFailureOnComponentInstanceIsReportedAtTheAttributeXml2

	_, err := expand(t, map[string]string{"app.tml": app, "card.tml": card}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `attribute "label": cannot negate {not count}`)
	requireAttrPos(t, err, xmlHeader+app, 6, `label=`, `<Card`)
}

// An element-scoped diagnostic must not drift onto an attribute: an unknown element is a problem with the element
func TestUnknownElementStaysOnTheElement(t *testing.T) {
	body := testUnknownElementStaysOnTheElementXml
	_, err := build(t, map[string]string{"app.tml": body})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown element <Dock>")

	var syntaxErr *syntax.Error
	require.ErrorAs(t, err, &syntaxErr)
	src := xmlHeader + body
	assert.Equal(t, 4, syntaxErr.Pos.Line)
	assert.Equal(t, colOf(t, src, 4, `<Dock`), syntaxErr.Pos.Col)
}

//go:embed testdata/test-eval-failure-on-component-instance-is-reported-at-the-attribute.xml
var testEvalFailureOnComponentInstanceIsReportedAtTheAttributeXml string

//go:embed testdata/test-eval-failure-on-component-instance-is-reported-at-the-attribute-2.xml
var testEvalFailureOnComponentInstanceIsReportedAtTheAttributeXml2 string

//go:embed testdata/test-unknown-element-stays-on-the-element.xml
var testUnknownElementStaysOnTheElementXml string
