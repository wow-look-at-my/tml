package sema

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// transcript is the shape this whole mechanism exists for: a scrolling region
// whose items are widgets, not lines somebody else already drew.
const transcript = `<Component xmlns="urn:tml:v1" name="App">
	<Property name="messages" type="record[]" default=""/>

	<Component name="MessageHeader">
		<Property name="role" type="string"/>
		<Property name="model" type="string" default=""/>
		<Template>
			<Text>{role} {model}</Text>
		</Template>
	</Component>

	<DataTemplate name="Message">
		<Property name="role" type="string" required="true"/>
		<Property name="model" type="string" default=""/>
		<Property name="body" type="string[]" default=""/>
		<Template>
			<Stack>
				<MessageHeader role="{role}" model="{model}"/>
				<For each="{body}" as="line">
					<Text>{line}</Text>
				</For>
			</Stack>
		</Template>
	</DataTemplate>

	<Template>
		<Scrollbox itemsSource="{messages}" itemTemplate="Message"/>
	</Template>
</Component>`

func messages() Value {
	return RecordListValue([]map[string]Value{
		{"role": StringValue("user"), "model": StringValue(""), "body": ListValue([]string{"do the thing"})},
		{"role": StringValue("assistant"), "model": StringValue("opus"), "body": ListValue([]string{"done", "here is how"})},
	})
}

func TestAnItemsControlDrawsOneTemplatePerItem(t *testing.T) {
	got, err := expand(t, map[string]string{"app.tml": transcript}, map[string]Value{"messages": messages()})
	require.NoError(t, err)

	// Two messages, each a Stack of a header and its body lines -- a tree, not
	// two pre-rendered strings.
	assert.Equal(t, strings.TrimSpace(`
App
  Scrollbox
    Stack
      Text "user "
      Text "do the thing"
    Stack
      Text "assistant opus"
      Text "done"
      Text "here is how"`), strings.TrimSpace(got))
}

func TestTheItemsControlAttributesNeverReachTheWidget(t *testing.T) {
	got, err := expand(t, map[string]string{"app.tml": transcript}, map[string]Value{"messages": messages()})
	require.NoError(t, err)

	// A widget handed `itemsSource` would be holding a list it has no idea what
	// to do with: they say what the element CONTAINS, not what it is.
	assert.NotContains(t, got, "itemsSource")
	assert.NotContains(t, got, "itemTemplate")
}

func TestAnEmptyListDrawsNothingRatherThanFailing(t *testing.T) {
	got, err := expand(t, map[string]string{"app.tml": transcript},
		map[string]Value{"messages": RecordListValue(nil)})
	require.NoError(t, err)
	assert.Equal(t, strings.TrimSpace("App\n  Scrollbox"), strings.TrimSpace(got))
}

// TestAFieldNobodyDeclaredIsAnError is the failure XAML answers with a blank
// cell. A message whose cost is called `spend` on one side and `cost` on the
// other must not render an empty column forever.
func TestAFieldNobodyDeclaredIsAnError(t *testing.T) {
	_, err := expand(t, map[string]string{"app.tml": transcript}, map[string]Value{
		"messages": RecordListValue([]map[string]Value{
			{"role": StringValue("user"), "spend": StringValue("$1")},
		}),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"spend"`)
	assert.Contains(t, err.Error(), "Message")
}

// TestAnItemMissingARequiredPropertyIsAnError: absent is not empty.
func TestAnItemMissingARequiredPropertyIsAnError(t *testing.T) {
	_, err := expand(t, map[string]string{"app.tml": transcript}, map[string]Value{
		"messages": RecordListValue([]map[string]Value{{"model": StringValue("opus")}}),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "role")
}

func TestAnItemTemplateMustBeADataTemplate(t *testing.T) {
	_, err := expand(t, map[string]string{
		"app.tml": `<Component xmlns="urn:tml:v1" name="App">
	<Property name="rows" type="record[]" default=""/>
	<Component name="Row">
		<Property name="text" type="string" default=""/>
		<Template><Text>{text}</Text></Template>
	</Component>
	<Template>
		<Stack itemsSource="{rows}" itemTemplate="Row"/>
	</Template>
</Component>`,
	}, map[string]Value{"rows": RecordListValue([]map[string]Value{{"text": StringValue("x")}})})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "<DataTemplate>")
}

func TestAnUnknownItemTemplateIsNamed(t *testing.T) {
	_, err := expand(t, map[string]string{
		"app.tml": `<Component xmlns="urn:tml:v1" name="App">
	<Property name="rows" type="record[]" default=""/>
	<Template>
		<Stack itemsSource="{rows}" itemTemplate="Nope"/>
	</Template>
</Component>`,
	}, map[string]Value{"rows": RecordListValue(nil)})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"Nope"`)
}

func TestAnItemsSourceMustBeAList(t *testing.T) {
	_, err := expand(t, map[string]string{
		"app.tml": `<Component xmlns="urn:tml:v1" name="App">
	<Property name="one" type="string" default=""/>
	<DataTemplate name="Row">
		<Property name="value" type="string" default=""/>
		<Template><Text>{value}</Text></Template>
	</DataTemplate>
	<Template>
		<Stack itemsSource="{one}" itemTemplate="Row"/>
	</Template>
</Component>`,
	}, map[string]Value{"one": StringValue("not a list")})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "needs a list")
}

// TestAListOfStringsStillWorks: the case that is genuinely just text should not
// have to be wrapped in a record to be drawn.
func TestAListOfStringsStillWorks(t *testing.T) {
	got, err := expand(t, map[string]string{
		"app.tml": `<Component xmlns="urn:tml:v1" name="App">
	<Property name="lines" type="string[]" default=""/>
	<DataTemplate name="Line">
		<Property name="value" type="string" default=""/>
		<Template><Text>{value}</Text></Template>
	</DataTemplate>
	<Template>
		<Stack itemsSource="{lines}" itemTemplate="Line"/>
	</Template>
</Component>`,
	}, map[string]Value{"lines": ListValue([]string{"one", "two"})})
	require.NoError(t, err)
	assert.Contains(t, got, "Text\n      \"one\"")
	assert.Contains(t, got, "Text\n      \"two\"")
}

// TestATemplateThatAsksForItsPositionGetsIt, and one that does not is not
// handed a property it never declared.
func TestATemplateThatAsksForItsPositionGetsIt(t *testing.T) {
	got, err := expand(t, map[string]string{
		"app.tml": `<Component xmlns="urn:tml:v1" name="App">
	<Property name="lines" type="string[]" default=""/>
	<DataTemplate name="Numbered">
		<Property name="value" type="string" default=""/>
		<Property name="index" type="int" default="0"/>
		<Template><Text>{index}: {value}</Text></Template>
	</DataTemplate>
	<Template>
		<Stack itemsSource="{lines}" itemTemplate="Numbered"/>
	</Template>
</Component>`,
	}, map[string]Value{"lines": ListValue([]string{"one", "two"})})
	require.NoError(t, err)
	assert.Contains(t, got, "Text\n      \"0: one\"")
	assert.Contains(t, got, "Text\n      \"1: two\"")
}
