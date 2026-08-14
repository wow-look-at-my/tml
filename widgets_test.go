package tml_test

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml"
	"github.com/wow-look-at-my/tml/widget"
)

const header = "<?xml version=\"1.1\" encoding=\"UTF-8\"?>\n"

// view loads a one-file view the way a host does, with the widget library on.
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

// The library is on by default: a template uses <Button> without the host
// binding anything, or the widgets would exist only for whoever found the
// registry.
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

// A host binding its own <Button> keeps the rest of the library rather than
// having to shadow every name to replace one.
//
// The host's is bound as a single instance, which claims no attributes -- one
// instance serves every element, so there is nothing per-element for them to
// configure. A widget that needs attributes is bound as a factory instead.
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

// A component may legitimately be called Table. Resolving its expanded root as
// the Table WIDGET would silently replace the whole view with an empty grid.
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

// A bad attribute on a widget is reported where it was written, like any other
// diagnostic, rather than rendering something plausible instead.
//
// It surfaces when the view is rendered rather than when it is loaded: a widget
// is built from evaluated attributes, and an attribute may hold an expression
// whose value is not known until the caller passes its properties in.
func TestWidgetAttributeErrorsArePositioned(t *testing.T) {
	loaded, err := view(t, app(`<Spinner kind="wobble"/>`), tml.Options{})
	require.NoError(t, err)

	_, err = loaded.Render(nil, 20, 4)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "app.tml:")
	assert.Contains(t, err.Error(), "expected one of arrow, bar, circle, dot, dots, line")
}

// A widget's own attributes are its own; everything else on the element is
// styling. Both have to work on the same element at once.
func TestWidgetAttributesAndStylingCoexist(t *testing.T) {
	loaded, err := view(t, app(`<Badge label="new" bold="true"/>`), tml.Options{})
	require.NoError(t, err)

	out, err := loaded.Render(nil, 10, 1)
	require.NoError(t, err)
	assert.Contains(t, ansi.Strip(out), "new")
	assert.NotEqual(t, ansi.Strip(out), out, "the style attribute reached lipgloss")
}

// Anything a widget does not claim is styling, so a typo lands there and is
// rejected as a style attribute rather than being quietly dropped.
func TestUnknownWidgetAttributeIsRejectedAsStyling(t *testing.T) {
	loaded, err := view(t, app(`<Badge label="new" colour="red"/>`), tml.Options{})
	require.NoError(t, err)

	_, err = loaded.Render(nil, 20, 4)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `unknown style attribute "colour"`)
}

// A popup written last on a canvas covers the view it interrupts, which is the
// whole reason a canvas exists.
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

// fixedView is a widget that always draws the same thing, for testing which
// binding won rather than what it drew.
type fixedView string

func (f fixedView) View() string { return string(f) }
