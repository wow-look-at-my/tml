package tml_test

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml"
	"github.com/wow-look-at-my/tml/sema"
	"github.com/wow-look-at-my/tml/widget"
)

const header = "<?xml version=\"1.1\" encoding=\"UTF-8\"?>\n"

// view loads a-file view the way a host does, with the widget library on.
func view(t *testing.T, body string, opts tml.Options) (*tml.View, error) {
	t.Helper()
	fsys := fstest.MapFS{"app.tml": &fstest.MapFile{Data: []byte(header + body)}}
	return tml.Load(fsys, "app.tml", opts)
}

func draw(t *testing.T, body string, w, h int) string {
	t.Helper()
	loaded, err := view(t, body, tml.Options{})
	require.NoError(t, err)
	out, err := loaded.Render(nil, w, h)
	require.NoError(t, err)
	return ansi.Strip(out)
}

func app(template string) string {
	return `<Component xmlns="urn:tml:v1" name="App"><Template>` + template + `</Template></Component>`
}

// The library is on by default: a template uses <Button> without the host binding anything, or the widgets would exist
func TestLibraryIsAvailableWithoutAnySetup(t *testing.T) {
	out := draw(t, app(`<Button label="Save"/>`), 20, 4)

	assert.Contains(t, out, "Save")
	assert.Contains(t, out, "║", "the sole button holds focus, so it wears the focused border")
}

func TestBareDropsTheLibrary(t *testing.T) {
	_, err := view(t, app(`<Button label="Save"/>`), tml.Options{Bare: true})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown element <Button>")
}

// A host binding its own <Button> keeps the rest of the library rather than having to shadow every name to replace
func TestHostBindingsBeatTheLibrary(t *testing.T) {
	host := widget.NewRegistry().Bind("Button", widget.Bubble(fixedView("HOST")))
	loaded, err := view(t, app(`<Stack><Button/><Badge label="new"/></Stack>`),
		tml.Options{Widgets: host})
	require.NoError(t, err)

	out, err := loaded.Render(nil, 20, 4)
	require.NoError(t, err)
	assert.Contains(t, out, "HOST")
	assert.Contains(t, out, "new", "the rest of the library came along")
}

// A component may legitimately be called Table. Resolving its expanded root as the Table WIDGET would silently replace
func TestAComponentMayShareAWidgetsName(t *testing.T) {
	out := draw(t, `<Component xmlns="urn:tml:v1" name="Table"><Template>
		<Text>not a widget</Text>
	</Template></Component>`, 20, 2)

	assert.Contains(t, out, "not a widget")
}

// Slot content reaches a widget, and the widget decides what to do with it.
func TestButtonContentSlotThroughTheWholePipeline(t *testing.T) {
	out := draw(t, app(`<Button label="ignored"><Button.Content><Text>Rich</Text></Button.Content></Button>`), 20, 4)

	assert.Contains(t, out, "Rich")
	assert.NotContains(t, out, "ignored")
}

func TestWidgetSlotsAreCheckedWhenTheViewLoads(t *testing.T) {
	_, err := view(t, app(`<Button><Button.Contnt><Text>x</Text></Button.Contnt></Button>`), tml.Options{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `<Button> has no slot "Contnt"; it has Content`)

	_, err = view(t, app(`<Badge><Badge.Label><Text>x</Text></Badge.Label></Badge>`), tml.Options{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "<Badge> takes no slot content")
}

// A bad attribute on a widget is reported where it was written, like any other diagnostic, rather than rendering
func TestWidgetAttributeErrorsArePositioned(t *testing.T) {
	loaded, err := view(t, app(`<Spinner kind="wobble"/>`), tml.Options{})
	require.NoError(t, err)

	_, err = loaded.Render(nil, 20, 4)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "app.tml:")
	assert.Contains(t, err.Error(), "expected one of arrow, bar, circle, dot, dots, line")
}

// A widget's own attributes are its own; everything else on the element is styling. Both have to work on the same
func TestWidgetAttributesAndStylingCoexist(t *testing.T) {
	loaded, err := view(t, app(`<Badge label="new" bold="true"/>`), tml.Options{})
	require.NoError(t, err)

	out, err := loaded.Render(nil, 10, 1)
	require.NoError(t, err)
	assert.Contains(t, ansi.Strip(out), "new")
	assert.NotEqual(t, ansi.Strip(out), out, "the style attribute reached lipgloss")
}

// Anything a widget does not claim is styling.
func TestUnknownWidgetAttributeIsRejectedAsStyling(t *testing.T) {
	loaded, err := view(t, app(`<Badge label="new" colour="red"/>`), tml.Options{})
	require.NoError(t, err)

	_, err = loaded.Render(nil, 20, 4)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `unknown style attribute "colour"`)
}

// A popup written last on a canvas covers the view it interrupts, which is the whole reason a canvas exists.
func TestPopupCoversTheViewOnACanvas(t *testing.T) {
	out := draw(t, app(`<Canvas>
		<Stack><Text>aaaaaaaaaaaaaaaaaaaa</Text><Text>bbbbbbbbbbbbbbbbbbbb</Text><Text>cccccccccccccccccccc</Text></Stack>
		<Popup title="Hi"><Text>over</Text></Popup>
	</Canvas>`), 20, 5)

	assert.Contains(t, out, "over")
	assert.Contains(t, out, "a", "the view underneath is still there around the edges")

	middle := strings.Split(out, "\n")[2]
	assert.Contains(t, middle, "over", "the dialog sits in the middle by default")
}

// fixedView is a widget that always draws the same thing, for testing which binding won rather than what it drew.
type fixedView string

func (f fixedView) View() string { return string(f) }

// A Spinner's frame belongs to the engine, so the host passes none: showing the element is what animates it, and hiding it
// is what stops the repainting.
func TestASpinnerAnimatesOnlyWhileItIsVisible(t *testing.T) {
	loaded, err := view(t, `<Component xmlns="urn:tml:v1" name="App">
		<Property name="state" type="string" default="visible"/>
		<Template><Stack><Spinner kind="line" state="{state}"/></Stack></Template>
	</Component>`, tml.Options{})
	require.NoError(t, err)

	hidden := tml.Props{"state": sema.StringValue("hidden")}
	shown := tml.Props{"state": sema.StringValue("visible")}

	// The spinner is the only thing on the frame, so a row of blanks is the whole of "it drew nothing".
	blank := strings.Repeat(" ", 10)
	firstRow := func(p tml.Props) string {
		t.Helper()
		out, err := loaded.Render(p, 10, 3)
		require.NoError(t, err)
		return strings.Split(ansi.Strip(out), "\n")[0]
	}

	assert.Equal(t, blank, firstRow(hidden), "a hidden spinner draws nothing")
	assert.False(t, loaded.Animating(), "and leaves the host nothing to keep painting")

	loaded.Advance()
	assert.Equal(t, blank, firstRow(hidden), "a step taken while it is hidden changes nothing on screen")

	before := firstRow(shown)
	assert.NotEqual(t, blank, before, "showing it again draws the frame the engine is on")
	assert.True(t, loaded.Animating(), "and the host has something to paint for again")

	loaded.Advance()
	after := firstRow(shown)
	assert.NotEqual(t, before, after, "a step moves the spinner on with no frame passed in from the host")

	loaded.Advance()
	assert.NotContains(t, firstRow(shown), after, "and it keeps moving as long as it is shown")
	assert.Equal(t, blank, firstRow(hidden), "hiding it again passes no frame at all")
	assert.False(t, loaded.Animating())
}
