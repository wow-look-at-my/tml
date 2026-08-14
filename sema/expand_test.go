package sema

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
		"app.tml": `<Component xmlns="urn:tml:v1" name="App">
	<Property name="title" type="string" required="true"/>
	<Property name="gap" type="int" default="2"/>
	<Template>
		<Stack gap="{gap}">
			<Text>{title}</Text>
		</Stack>
	</Template>
</Component>`,
	}, map[string]Value{"title": StringValue("Deploy")})
	require.NoError(t, err)

	assert.Equal(t, strings.TrimSpace(`
App
  Stack gap="2"
    Text
      "Deploy"`), strings.TrimSpace(got))
}

// Slot content is a closure over the call site: it sees the caller's properties,
// never the component it is passed into.
func TestSlotContentEvaluatesInTheCallersScope(t *testing.T) {
	got, err := expand(t, map[string]string{
		"app.tml": `<Component xmlns="urn:tml:v1" name="App">
	<Import src="./Card.tml"/>
	<Property name="who" type="string" default="world"/>
	<Template>
		<Card label="outer">
			<Text>hello {who}</Text>
			<Card.actions>
				<Text>ok</Text>
			</Card.actions>
		</Card>
	</Template>
</Component>`,
		"Card.tml": `<Component xmlns="urn:tml:v1" name="Card">
	<Property name="label" type="string" default=""/>
	<Template>
		<Box>
			<Text>{label}</Text>
			<Slot/>
			<Slot name="actions"/>
		</Box>
	</Template>
</Component>`,
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
		"app.tml": `<Component xmlns="urn:tml:v1" name="App">
	<Import src="./Card.tml"/>
	<Template><Card/></Template>
</Component>`,
		"Card.tml": `<Component xmlns="urn:tml:v1" name="Card">
	<Template>
		<Slot name="actions">
			<Text>none</Text>
		</Slot>
	</Template>
</Component>`,
	}
	got, err := expand(t, files, nil)
	require.NoError(t, err)
	assert.Contains(t, got, `"none"`, "an unfilled slot falls back to its own children")
}

func TestConditionalAndLoop(t *testing.T) {
	got, err := expand(t, map[string]string{
		"app.tml": `<Component xmlns="urn:tml:v1" name="App">
	<Property name="tags" type="string[]" default=""/>
	<Property name="note" type="string" default=""/>
	<Template>
		<Stack>
			<Text if="{note}">{note}</Text>
			<Text if="{not note}">no note</Text>
			<For each="{tags}" as="tag" index="i">
				<Text>{i}:{tag}</Text>
			</For>
		</Stack>
	</Template>
</Component>`,
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
		"app.tml": `<Component xmlns="urn:tml:v1" name="App">
	<Import src="./theme.tml"/>
	<Property name="accent" type="color" default="{theme.accent}"/>
	<Template><Box fg="{accent}"/></Template>
</Component>`,
		"theme.tml": `<Theme xmlns="urn:tml:v1" name="default">
	<Token name="accent" light="#5f5fd7" dark="#e0af68"/>
</Theme>`,
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

// A theme token is text until it reaches a typed property, where it is re-read
// as that type. A token that is not a valid colour must fail there.
func TestTokenIsCheckedAgainstTheTargetType(t *testing.T) {
	_, err := expand(t, map[string]string{
		"app.tml": `<Component xmlns="urn:tml:v1" name="App">
	<Import src="./theme.tml"/>
	<Property name="accent" type="color" default="{theme.spacing}"/>
	<Template><Box/></Template>
</Component>`,
		"theme.tml": `<Theme xmlns="urn:tml:v1" name="default">
	<Token name="spacing" value="wide"/>
</Theme>`,
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
			name: "unknown element",
			files: map[string]string{"app.tml": `<Component xmlns="urn:tml:v1" name="App">
	<Template><Stak/></Template>
</Component>`},
			wantErr: "unknown element <Stak>",
		},
		{
			name: "unknown reference is caught without rendering",
			files: map[string]string{"app.tml": `<Component xmlns="urn:tml:v1" name="App">
	<Property name="title" type="string" default=""/>
	<Template><Text>{titel}</Text></Template>
</Component>`},
			wantErr: `unknown reference "titel"`,
		},
		{
			name: "unknown reference inside a branch never taken",
			files: map[string]string{"app.tml": `<Component xmlns="urn:tml:v1" name="App">
	<Property name="on" type="bool" default="false"/>
	<Template><Text if="{on}">{missing}</Text></Template>
</Component>`},
			wantErr: `unknown reference "missing"`,
		},
		{
			name: "unknown theme token",
			files: map[string]string{"app.tml": `<Component xmlns="urn:tml:v1" name="App">
	<Template><Box fg="{theme.nope}"/></Template>
</Component>`},
			wantErr: `unknown theme token "nope"`,
		},
		{
			name: "duplicate property",
			files: map[string]string{"app.tml": `<Component xmlns="urn:tml:v1" name="App">
	<Property name="a" type="string" default=""/>
	<Property name="a" type="int" default="0"/>
	<Template><Box/></Template>
</Component>`},
			wantErr: `duplicate property "a"`,
		},
		{
			name: "default must fit its declared type",
			files: map[string]string{"app.tml": `<Component xmlns="urn:tml:v1" name="App">
	<Property name="n" type="int" default="lots"/>
	<Template><Box/></Template>
</Component>`},
			wantErr: "expected a whole number",
		},
		{
			name: "unknown type",
			files: map[string]string{"app.tml": `<Component xmlns="urn:tml:v1" name="App">
	<Property name="n" type="number" default="1"/>
	<Template><Box/></Template>
</Component>`},
			wantErr: `unknown type "number"`,
		},
		{
			name: "a component cannot instantiate itself",
			files: map[string]string{"app.tml": `<Component xmlns="urn:tml:v1" name="App">
	<Template><App/></Template>
</Component>`},
			wantErr: "instantiates itself",
		},
		{
			name: "a loop variable is not visible outside its loop",
			files: map[string]string{"app.tml": `<Component xmlns="urn:tml:v1" name="App">
	<Property name="tags" type="string[]" default=""/>
	<Template>
		<Stack>
			<For each="{tags}" as="tag"><Text>{tag}</Text></For>
			<Text>{tag}</Text>
		</Stack>
	</Template>
</Component>`},
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
			"Card.tml": `<Component xmlns="urn:tml:v1" name="Card">
	<Property name="label" type="string" required="true"/>
	<Template><Box><Slot name="actions" required="true"/></Box></Template>
</Component>`,
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
