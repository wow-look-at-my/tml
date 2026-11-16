package widgets

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml/sema"
	"github.com/wow-look-at-my/tml/widget"
	"github.com/wow-look-at-my/tml/widget/widgettest"
)

// build makes a single widget the way the engine does, from the element's name and its attributes.
func build(t *testing.T, element string, pairs map[string]string) widget.Native {
	t.Helper()
	native, err := tryBuild(element, pairs)
	require.NoError(t, err)
	return native
}

func tryBuild(element string, pairs map[string]string) (widget.Native, error) {
	factory, ok := Library().Factory(element)
	if !ok {
		return nil, assertUnknown(element)
	}
	return factory.Build(widget.Context{Attrs: attrsOf(element, pairs)})
}

// attrsOf builds the attribute set the engine would hand a factory.
func attrsOf(element string, pairs map[string]string) widget.Attrs {
	values := map[string]sema.Value{}
	order := make([]string, 0, len(pairs))
	for name, raw := range pairs {
		values[name] = sema.StringValue(raw)
		order = append(order, name)
	}
	return widget.NewAttrs(element, values, order)
}

// text is what a widget paints at a size, without styles, each row cut of its trailing blanks.
func text(native widget.Native, w, h int) string {
	return strings.Join(trimmedLines(widgettest.Draw(native, w, h).Lines()), "\n")
}

// rows is what a widget paints at a size, without styles, one row per entry at the full width.
func rows(native widget.Native, w, h int) []string {
	return widgettest.Draw(native, w, h).Lines()
}

// trimmedLines cuts the trailing blanks off every line.
func trimmedLines(lines []string) []string {
	out := make([]string, len(lines))
	for i, line := range lines {
		out[i] = strings.TrimRight(line, " ")
	}
	return out
}

type unknownElement string

func (u unknownElement) Error() string { return "no widget named " + string(u) }

func assertUnknown(element string) error { return unknownElement(element) }

func TestLibraryBindsEveryDocumentedWidget(t *testing.T) {
	assert.Equal(t, []string{
		"Badge", "Border", "Button", "Checkbox", "Image", "List", "Popup", "ProgressBar",
		"Radio", "Rule", "Scrollbox", "Sparkline", "Spinner", "Table", "Textbox",
	}, Names())
}
