package widgets

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml/sema"
	"github.com/wow-look-at-my/tml/widget"
)

// build makes one widget the way the engine does, from the element's name and
// its attributes.
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

type unknownElement string

func (u unknownElement) Error() string { return "no widget named " + string(u) }

func assertUnknown(element string) error { return unknownElement(element) }

func TestLibraryBindsEveryDocumentedWidget(t *testing.T) {
	assert.Equal(t, []string{
		"Badge", "Border", "Button", "Checkbox", "Image", "List", "Popup", "ProgressBar",
		"Radio", "Rule", "Scrollbox", "Sparkline", "Spinner", "Table", "Textbox",
	}, Names())
}
