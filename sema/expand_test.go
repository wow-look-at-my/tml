package sema

import _ "embed"

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml/syntax"
)

const xmlHeader = "<?xml version=\"1.1\" encoding=\"UTF-8\"?>\n"

func build(t *testing.T, files map[string]string) (*Program, error) {
	t.Helper()
	fsys := fstest.MapFS{}
	for name, body := range files {
		fsys[name] = &fstest.MapFile{Data: []byte(xmlHeader + body)}
	}
	unit, err := syntax.Load(fsys, "app.tml")
	require.NoError(t, err)
	return Analyze(unit, Options{})
}

func expand(t *testing.T, files map[string]string, args map[string]Value) (string, error) {
	t.Helper()
	program, err := build(t, files)
	if err != nil {
		return "", err
	}
	node, err := program.Expand(args, ExpandOptions{})
	if err != nil {
		return "", err
	}
	return node.Dump(), nil
}

func TestExpandSubstitutesPropertiesAndDefaults(t *testing.T) {
	got, err := expand(t, map[string]string{
		"app.tml": testExpandSubstitutesPropertiesAndDefaultsXml,
	}, map[string]Value{"title": StringValue("Deploy")})
	require.NoError(t, err)

	assert.Equal(t, strings.TrimSpace(`
App
  Stack gap="2"
    Text
      "Deploy"`), strings.TrimSpace(got))
}

// Slot content is a closure over the call site: it sees the caller's properties, never the component it is passed into.
func TestSlotContentEvaluatesInTheCallersScope(t *testing.T) {
	got, err := expand(t, map[string]string{
		"app.tml":  testSlotContentEvaluatesInTheCallersScopeXml,
		"Card.tml": testSlotContentEvaluatesInTheCallersScopeXml2,
	}, nil)
	require.NoError(t, err)

	assert.Equal(t, strings.TrimSpace(`
App
  Box
    Text
      "outer"
    Text
      "hello world"
    Text
      "ok"`), strings.TrimSpace(got))
}

func TestSlotFallbackIsUsedWhenNothingIsPassed(t *testing.T) {
	files := map[string]string{
		"app.tml":  testSlotFallbackIsUsedWhenNothingIsPassedXml,
		"Card.tml": testSlotFallbackIsUsedWhenNothingIsPassedXml2,
	}
	got, err := expand(t, files, nil)
	require.NoError(t, err)
	assert.Contains(t, got, `"none"`, "an unfilled slot falls back to its own children")
}

func TestConditionalAndLoop(t *testing.T) {
	got, err := expand(t, map[string]string{
		"app.tml": testConditionalAndLoopXml,
	}, map[string]Value{"tags": mustValue(t, "string[]", "api,web")})
	require.NoError(t, err)

	assert.Equal(t, strings.TrimSpace(`
App
  Stack
    Text
      "no note"
    Text
      "0:api"
    Text
      "1:web"`), strings.TrimSpace(got))
}

func mustValue(t *testing.T, typeSrc, raw string) Value {
	t.Helper()
	typ, err := ParseType(typeSrc)
	require.NoError(t, err)
	value, err := ParseValue(typ, raw)
	require.NoError(t, err)
	return value
}

func TestThemeTokensResolveAndFollowDarkMode(t *testing.T) {
	files := map[string]string{
		"app.tml":   testThemeTokensResolveAndFollowDarkModeXml,
		"theme.tml": testThemeTokensResolveAndFollowDarkModeXml2,
	}

	program, err := build(t, files)
	require.NoError(t, err)

	light, err := program.Expand(nil, ExpandOptions{})
	require.NoError(t, err)
	assert.Contains(t, light.Dump(), `fg="#5f5fd7"`)

	dark, err := program.Expand(nil, ExpandOptions{Dark: true})
	require.NoError(t, err)
	assert.Contains(t, dark.Dump(), `fg="#e0af68"`, "an adaptive token follows the mode")
}

// A theme token is text until it reaches a typed property, where it is re-read as that type.
func TestTokenIsCheckedAgainstTheTargetType(t *testing.T) {
	_, err := expand(t, map[string]string{
		"app.tml":   testTokenIsCheckedAgainstTheTargetTypeXml,
		"theme.tml": testTokenIsCheckedAgainstTheTargetTypeXml2,
	}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "hex triplet")
}

func TestAnalyzeRejects(t *testing.T) {
	tests := []struct {
		name    string
		files   map[string]string
		wantErr string
	}{
		{
			name:    "unknown element",
			files:   map[string]string{"app.tml": testAnalyzeRejectsXml},
			wantErr: "unknown element <Stak>",
		},
		{
			name:    "unknown reference is caught without rendering",
			files:   map[string]string{"app.tml": testAnalyzeRejectsXml2},
			wantErr: `unknown reference "titel"`,
		},
		{
			name:    "unknown reference inside a branch never taken",
			files:   map[string]string{"app.tml": testAnalyzeRejectsXml3},
			wantErr: `unknown reference "missing"`,
		},
		{
			name:    "unknown theme token",
			files:   map[string]string{"app.tml": testAnalyzeRejectsXml4},
			wantErr: `unknown theme token "nope"`,
		},
		{
			name:    "duplicate property",
			files:   map[string]string{"app.tml": testAnalyzeRejectsXml5},
			wantErr: `duplicate property "a"`,
		},
		{
			name:    "default must fit its declared type",
			files:   map[string]string{"app.tml": testAnalyzeRejectsXml6},
			wantErr: "expected a whole number",
		},
		{
			name:    "unknown type",
			files:   map[string]string{"app.tml": testAnalyzeRejectsXml7},
			wantErr: `unknown type "number"`,
		},
		{
			name:    "a component cannot instantiate itself",
			files:   map[string]string{"app.tml": testAnalyzeRejectsXml8},
			wantErr: "instantiates itself",
		},
		{
			name:    "a loop variable is not visible outside its loop",
			files:   map[string]string{"app.tml": testAnalyzeRejectsXml9},
			wantErr: `unknown reference "tag"`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := build(t, tc.files)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

func TestExpandRejects(t *testing.T) {
	cardFiles := func(template string) map[string]string {
		return map[string]string{
			"app.tml": `<Component xmlns="urn:tml:v1" name="App">
	<Import src="./Card.tml"/>
	<Template>` + template + `</Template>
</Component>`,
			"Card.tml": testExpandRejectsXml,
		}
	}

	tests := []struct {
		name     string
		template string
		wantErr  string
	}{
		{
			name:     "missing required property",
			template: `<Card><Card.actions><Text>x</Text></Card.actions></Card>`,
			wantErr:  `requires property "label"`,
		},
		{
			name:     "unknown property",
			template: `<Card label="a" colour="red"><Card.actions><Text>x</Text></Card.actions></Card>`,
			wantErr:  `has no property "colour"`,
		},
		{
			name:     "unfilled required slot",
			template: `<Card label="a"/>`,
			wantErr:  `slot "actions" is required but was not filled`,
		},
		{
			name:     "slot filled twice",
			template: `<Card label="a"><Card.actions><Text>x</Text></Card.actions><Card.actions><Text>y</Text></Card.actions></Card>`,
			wantErr:  `slot "actions" is filled twice`,
		},
		{
			name:     "property element under the wrong parent",
			template: `<Stack><Card.actions><Text>x</Text></Card.actions></Stack>`,
			wantErr:  `<Card.actions> names a slot on "Card" but is inside <Stack>`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := expand(t, cardFiles(tc.template), nil)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

//go:embed testdata/test-expand-substitutes-properties-and-defaults.xml
var testExpandSubstitutesPropertiesAndDefaultsXml string

//go:embed testdata/test-slot-content-evaluates-in-the-callers-scope.xml
var testSlotContentEvaluatesInTheCallersScopeXml string

//go:embed testdata/test-slot-content-evaluates-in-the-callers-scope-2.xml
var testSlotContentEvaluatesInTheCallersScopeXml2 string

//go:embed testdata/test-slot-fallback-is-used-when-nothing-is-passed.xml
var testSlotFallbackIsUsedWhenNothingIsPassedXml string

//go:embed testdata/test-slot-fallback-is-used-when-nothing-is-passed-2.xml
var testSlotFallbackIsUsedWhenNothingIsPassedXml2 string

//go:embed testdata/test-conditional-and-loop.xml
var testConditionalAndLoopXml string

//go:embed testdata/test-theme-tokens-resolve-and-follow-dark-mode.xml
var testThemeTokensResolveAndFollowDarkModeXml string

//go:embed testdata/test-theme-tokens-resolve-and-follow-dark-mode-2.xml
var testThemeTokensResolveAndFollowDarkModeXml2 string

//go:embed testdata/test-token-is-checked-against-the-target-type.xml
var testTokenIsCheckedAgainstTheTargetTypeXml string

//go:embed testdata/test-token-is-checked-against-the-target-type-2.xml
var testTokenIsCheckedAgainstTheTargetTypeXml2 string

//go:embed testdata/test-analyze-rejects.xml
var testAnalyzeRejectsXml string

//go:embed testdata/test-analyze-rejects-2.xml
var testAnalyzeRejectsXml2 string

//go:embed testdata/test-analyze-rejects-3.xml
var testAnalyzeRejectsXml3 string

//go:embed testdata/test-analyze-rejects-4.xml
var testAnalyzeRejectsXml4 string

//go:embed testdata/test-analyze-rejects-5.xml
var testAnalyzeRejectsXml5 string

//go:embed testdata/test-analyze-rejects-6.xml
var testAnalyzeRejectsXml6 string

//go:embed testdata/test-analyze-rejects-7.xml
var testAnalyzeRejectsXml7 string

//go:embed testdata/test-analyze-rejects-8.xml
var testAnalyzeRejectsXml8 string

//go:embed testdata/test-analyze-rejects-9.xml
var testAnalyzeRejectsXml9 string

//go:embed testdata/test-expand-rejects.xml
var testExpandRejectsXml string
