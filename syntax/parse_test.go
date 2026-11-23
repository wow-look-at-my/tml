package syntax

import _ "embed"

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const header = "<?xml version=\"1.1\" encoding=\"UTF-8\"?>\n"

func parseSource(t *testing.T, body string) (*File, error) {
	t.Helper()
	return Parse("test.tml", []byte(header+body))
}

func mustParse(t *testing.T, body string) *File {
	t.Helper()
	file, err := parseSource(t, body)
	require.NoError(t, err)
	return file
}

func TestParseComponentCapturesPropertiesAndTemplate(t *testing.T) {
	file := mustParse(t, testParseComponentCapturesPropertiesAndTemplateXml)

	component := file.Component
	require.NotNil(t, component)
	assert.Equal(t, "Card", component.Name)

	require.Len(t, component.Imports, 1)
	assert.Equal(t, "./Badge.tml", component.Imports[0].Src)

	require.Len(t, component.Properties, 2)
	assert.Equal(t, "title", component.Properties[0].Name)
	assert.True(t, component.Properties[0].Required)
	assert.False(t, component.Properties[0].HasDefault)
	assert.True(t, component.Properties[1].HasDefault, "an empty default is still a default")

	body := component.Template.Elements()
	require.Len(t, body, 1)
	assert.Equal(t, "Stack", body[0].Name)

	orientation, ok := body[0].Attr("orientation")
	require.True(t, ok)
	assert.Equal(t, "vertical", orientation)
}

// Dotted names carry the property-element slot syntax and attached layout properties. Syntax must pass them through
func TestParsePreservesDottedNames(t *testing.T) {
	file := mustParse(t, testParsePreservesDottedNamesXml)

	card := file.Component.Template.Elements()[0]
	slot := card.Elements()[0]
	assert.Equal(t, "Card.actions", slot.Name, "the property-element form survives parsing")

	button := slot.Elements()[0]
	row, ok := button.Attr("Grid.row")
	require.True(t, ok, "an attached property keeps its dotted attribute name")
	assert.Equal(t, "1", row)
}

// A file defines exactly a single component.
func TestParseRefusesANestedComponent(t *testing.T) {
	_, err := parseSource(t, testParseRefusesANestedComponentXml)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "test.tml")
	assert.Contains(t, err.Error(), `"App"`)
	assert.Contains(t, err.Error(), `"Row"`)
	assert.Contains(t, err.Error(), "one component per file")
}

func TestParseTheme(t *testing.T) {
	file := mustParse(t, testParseThemeXml)

	theme := file.Theme
	require.NotNil(t, theme)
	require.Len(t, theme.Tokens, 2)
	assert.Equal(t, "rounded", theme.Tokens[0].Dark)
	assert.Equal(t, "1", theme.Tokens[1].Value)

	require.Len(t, theme.Colors, 2)
	accent, ink := theme.Colors[0], theme.Colors[1]
	assert.Equal(t, "what stands out", accent.For)
	assert.Equal(t, "#5f5fd7", accent.In(false))
	assert.Equal(t, "#e0af68", accent.In(true))
	assert.Equal(t, "0", ink.In(false))
	assert.Equal(t, "0", ink.In(true), "a colour with one value has it in both modes")

	require.Len(t, theme.Styles, 2)
	assert.Equal(t, "card", theme.Styles[1].Extends)
	assert.Len(t, theme.Styles[1].Attrs, 1, "name and extends are not style attributes")
	assert.Equal(t, "bold", theme.Styles[1].Attrs[0].Name)
}

// Indentation must not become content, or every panel gains phantom text children and every wrapped sentence keeps its
func TestTextNormalization(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
		keep bool
	}{
		{name: "indentation between elements is dropped", raw: "\n\t\t", keep: false},
		{name: "inline spacing is deliberate", raw: " ", want: " ", keep: true},
		{name: "edges of a wrapped node are trimmed", raw: "\n\tHello\n", want: "Hello", keep: true},
		{name: "an internal line break becomes one space", raw: "Hello\n\t\tthere", want: "Hello there", keep: true},
		{name: "single-line runs are preserved", raw: "a  b", want: "a  b", keep: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, keep := normalizeText(tc.raw)
			assert.Equal(t, tc.keep, keep)
			if tc.keep {
				assert.Equal(t, tc.want, got)
			}
		})
	}
}

func TestParseTextContentIsNormalizedInTemplates(t *testing.T) {
	file := mustParse(t, testParseTextContentIsNormalizedInTemplatesXml)

	body := file.Component.Template.Elements()
	require.Len(t, body, 1, "indentation must not appear as extra children")

	text := body[0].Children
	require.Len(t, text, 1)
	assert.Equal(t, TextNode, text[0].Kind)
	assert.Equal(t, "Hello {name}", text[0].Text)
}

func TestParseRejects(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr string
	}{
		{
			name:    "missing namespace",
			body:    `<Component name="A"><Template/></Component>`,
			wantErr: "has no namespace",
		},
		{
			name:    "wrong root element",
			body:    `<Widget xmlns="urn:tml:v1"/>`,
			wantErr: "must be <Component> or <Theme>",
		},
		{
			name:    "component without a template",
			body:    `<Component xmlns="urn:tml:v1" name="A"/>`,
			wantErr: "has no <Template>",
		},
		{
			name:    "unknown attribute is a typo, not something to ignore",
			body:    `<Component xmlns="urn:tml:v1" name="A" nmae="B"><Template/></Component>`,
			wantErr: `has no attribute "nmae"`,
		},
		{
			name:    "unknown child directive",
			body:    `<Component xmlns="urn:tml:v1" name="A"><Slots/><Template/></Component>`,
			wantErr: "unexpected <Slots>",
		},
		{
			name:    "property without a type",
			body:    `<Component xmlns="urn:tml:v1" name="A"><Property name="x"/><Template/></Component>`,
			wantErr: "requires a type attribute",
		},
		{
			name:    "required and defaulted at once",
			body:    `<Component xmlns="urn:tml:v1" name="A"><Property name="x" type="string" required="true" default="y"/><Template/></Component>`,
			wantErr: "required and also has a default",
		},
		{
			name:    "non-boolean required",
			body:    `<Component xmlns="urn:tml:v1" name="A"><Property name="x" type="string" required="yes"/><Template/></Component>`,
			wantErr: `must be true or false, got "yes"`,
		},
		{
			name:    "two templates",
			body:    `<Component xmlns="urn:tml:v1" name="A"><Template/><Template/></Component>`,
			wantErr: "more than one <Template>",
		},
		{
			// A single definition per file, per CLAUDE.md: a file's root is exactly a single <Component> or <Theme>.
			name:    "a second root component",
			body:    `<Component xmlns="urn:tml:v1" name="A"><Template/></Component><Component xmlns="urn:tml:v1" name="B"><Template/></Component>`,
			wantErr: "unexpected content after root element",
		},
		{
			name:    "half an adaptive token",
			body:    `<Theme xmlns="urn:tml:v1" name="t"><Token name="c" light="#fff"/></Theme>`,
			wantErr: "only one of light and dark",
		},
		{
			name:    "a colour that does not say what it is for",
			body:    `<Theme xmlns="urn:tml:v1" name="t"><Color name="c" value="#fff"/></Theme>`,
			wantErr: `<Color> "c" requires a for attribute that says what the colour is used for`,
		},
		{
			name:    "a colour whose use is blank",
			body:    `<Theme xmlns="urn:tml:v1" name="t"><Color name="c" for="  " value="#fff"/></Theme>`,
			wantErr: "requires a for attribute",
		},
		{
			name:    "a colour with no name",
			body:    `<Theme xmlns="urn:tml:v1" name="t"><Color for="the page" value="#fff"/></Theme>`,
			wantErr: "<Color> requires a name attribute",
		},
		{
			name:    "half an adaptive colour",
			body:    `<Theme xmlns="urn:tml:v1" name="t"><Color name="c" for="the page" dark="#fff"/></Theme>`,
			wantErr: "only one of light and dark",
		},
		{
			name:    "a colour with a value and a pair",
			body:    `<Theme xmlns="urn:tml:v1" name="t"><Color name="c" for="the page" value="1" light="2" dark="3"/></Theme>`,
			wantErr: "sets both value and a light/dark pair",
		},
		{
			name:    "a colour with no value at all",
			body:    `<Theme xmlns="urn:tml:v1" name="t"><Color name="c" for="the page"/></Theme>`,
			wantErr: "needs either a value or a light/dark pair",
		},
		{
			name:    "token with no value at all",
			body:    `<Theme xmlns="urn:tml:v1" name="t"><Token name="c"/></Theme>`,
			wantErr: "needs either a value or a light/dark pair",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseSource(t, tc.body)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

// The strict XML rules come from xml-validator, and their diagnostics must reach the user with the file name attached
func TestXMLFailuresArePositionedAndAttributed(t *testing.T) {
	_, err := Parse("card.tml", []byte(`<Component xmlns="urn:tml:v1" name="A"><Template/></Component>`))
	require.Error(t, err, "a missing XML 1.1 declaration is rejected")
	assert.Contains(t, err.Error(), "card.tml:", "the diagnostic names the file it came from")

	_, err = parseSource(t, `<Component xmlns="urn:tml:v1" name="A"><Template></Component>`)
	require.Error(t, err, "a mismatched tag is rejected")

	var syntaxErr *Error
	require.ErrorAs(t, err, &syntaxErr)
	assert.Positive(t, syntaxErr.Pos.Line, "the position points into the source")
}

//go:embed testdata/test-parse-component-captures-properties-and-template.xml
var testParseComponentCapturesPropertiesAndTemplateXml string

//go:embed testdata/test-parse-preserves-dotted-names.xml
var testParsePreservesDottedNamesXml string

//go:embed testdata/test-parse-refuses-a-nested-component.xml
var testParseRefusesANestedComponentXml string

//go:embed testdata/test-parse-theme.xml
var testParseThemeXml string

//go:embed testdata/test-parse-text-content-is-normalized-in-templates.xml
var testParseTextContentIsNormalizedInTemplatesXml string
