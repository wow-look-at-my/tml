package sema

import _ "embed"

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// transcript is the shape this whole mechanism exists for: a scrolling region whose items are widgets, not lines
//
//go:embed testdata/transcript.xml
var transcript string

//go:embed testdata/message-header.xml
var messageHeader string

// transcriptFiles is the message fixture as files the loader sees.
func transcriptFiles() map[string]string {
	return map[string]string{"app.tml": transcript, "MessageHeader.tml": messageHeader}
}

func messages() Value {
	return RecordListValue([]map[string]Value{
		{"role": StringValue("user"), "model": StringValue(""), "body": ListValue([]string{"do the thing"})},
		{"role": StringValue("assistant"), "model": StringValue("opus"), "body": ListValue([]string{"done", "here is how"})},
	})
}

func TestAnItemsControlDrawsOneTemplatePerItem(t *testing.T) {
	got, err := expand(t, transcriptFiles(), map[string]Value{"messages": messages()})
	require.NoError(t, err)

	// A pair of messages, each a Stack of a header and its body lines -- a tree, not a pair of pre-rendered strings.
	assert.Equal(t, strings.TrimSpace(`
App
  Stack
    Stack
      Text
        "user "
      Text
        "do the thing"
    Stack
      Text
        "assistant opus"
      Text
        "done"
      Text
        "here is how"`), strings.TrimSpace(got))
}

func TestTheItemsControlAttributesNeverReachTheWidget(t *testing.T) {
	got, err := expand(t, transcriptFiles(), map[string]Value{"messages": messages()})
	require.NoError(t, err)

	// A widget handed `itemsSource` would be holding a list it has no idea what to do with: they say what the element
	assert.NotContains(t, got, "itemsSource")
	assert.NotContains(t, got, "itemTemplate")
}

func TestAnEmptyListDrawsNothingRatherThanFailing(t *testing.T) {
	got, err := expand(t, transcriptFiles(),
		map[string]Value{"messages": RecordListValue(nil)})
	require.NoError(t, err)
	assert.Equal(t, strings.TrimSpace("App\n  Stack"), strings.TrimSpace(got))
}

// TestAFieldNobodyDeclaredIsAnError is the failure XAML answers with a blank cell. A message whose cost is called
func TestAFieldNobodyDeclaredIsAnError(t *testing.T) {
	_, err := expand(t, transcriptFiles(), map[string]Value{
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
	_, err := expand(t, transcriptFiles(), map[string]Value{
		"messages": RecordListValue([]map[string]Value{{"model": StringValue("opus")}}),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "role")
}

func TestAnItemTemplateMustBeADataTemplate(t *testing.T) {
	// A component is a single per file, so "Row" lives in its own file. Using a component's name as an itemTemplate is still
	_, err := expand(t, map[string]string{
		"app.tml": testAnItemTemplateMustBeADataTemplateXml,
		"Row.tml": testAnItemTemplateMustBeADataTemplateXml2,
	}, map[string]Value{"rows": RecordListValue([]map[string]Value{{"text": StringValue("x")}})})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "<DataTemplate>")
}

func TestAnUnknownItemTemplateIsNamed(t *testing.T) {
	_, err := expand(t, map[string]string{
		"app.tml": testAnUnknownItemTemplateIsNamedXml,
	}, map[string]Value{"rows": RecordListValue(nil)})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"Nope"`)
}

func TestAnItemsSourceMustBeAList(t *testing.T) {
	_, err := expand(t, map[string]string{
		"app.tml": testAnItemsSourceMustBeAListXml,
	}, map[string]Value{"one": StringValue("not a list")})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "needs a list")
}

// TestAListOfStringsStillWorks: the case that is genuinely text should not
// have to be wrapped in a record to be
func TestAListOfStringsStillWorks(t *testing.T) {
	got, err := expand(t, map[string]string{
		"app.tml": testAListOfStringsStillWorksXml,
	}, map[string]Value{"lines": ListValue([]string{"one", "two"})})
	require.NoError(t, err)
	assert.Contains(t, got, "Text\n      \"one\"")
	assert.Contains(t, got, "Text\n      \"two\"")
}

// TestATemplateThatAsksForItsPositionGetsIt, and a template that does not is not handed a property it never declared.
func TestATemplateThatAsksForItsPositionGetsIt(t *testing.T) {
	got, err := expand(t, map[string]string{
		"app.tml": testATemplateThatAsksForItsPositionGetsItXml,
	}, map[string]Value{"lines": ListValue([]string{"one", "two"})})
	require.NoError(t, err)
	assert.Contains(t, got, "Text\n      \"0: one\"")
	assert.Contains(t, got, "Text\n      \"1: two\"")
}

//go:embed testdata/test-an-item-template-must-be-a-data-template.xml
var testAnItemTemplateMustBeADataTemplateXml string

//go:embed testdata/test-an-item-template-must-be-a-data-template-2.xml
var testAnItemTemplateMustBeADataTemplateXml2 string

//go:embed testdata/test-an-unknown-item-template-is-named.xml
var testAnUnknownItemTemplateIsNamedXml string

//go:embed testdata/test-an-items-source-must-be-a-list.xml
var testAnItemsSourceMustBeAListXml string

//go:embed testdata/test-a-list-of-strings-still-works.xml
var testAListOfStringsStillWorksXml string

//go:embed testdata/test-a-template-that-asks-for-its-position-gets-it.xml
var testATemplateThatAsksForItsPositionGetsItXml string
