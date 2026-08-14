package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml/sema"
)

func attrs(values map[string]sema.Value) Attrs {
	order := make([]string, 0, len(values))
	for name := range values {
		order = append(order, name)
	}
	return NewAttrs("Widget", values, order)
}

func strs(pairs map[string]string) Attrs {
	values := map[string]sema.Value{}
	for name, raw := range pairs {
		values[name] = sema.StringValue(raw)
	}
	return attrs(values)
}

func TestAttrsFallBackWhenAbsent(t *testing.T) {
	a := strs(nil)

	assert.Equal(t, "fallback", a.String("missing", "fallback"))
	assert.False(t, a.Has("missing"))

	n, err := a.Int("missing", 7)
	require.NoError(t, err)
	assert.Equal(t, 7, n)

	f, err := a.Float("missing", 1.5)
	require.NoError(t, err)
	assert.InDelta(t, 1.5, f, 0)

	on, err := a.Bool("missing", true)
	require.NoError(t, err)
	assert.True(t, on)

	r, err := a.Rune("missing", 'x')
	require.NoError(t, err)
	assert.Equal(t, 'x', r)

	assert.Nil(t, a.List("missing"))
}

// An attribute nobody can parse is a mistake in the template. Reporting it beats
// rendering something plausible, which would leave the author hunting for why
// the number they wrote had no effect.
func TestAttrsRejectUnparseableValues(t *testing.T) {
	a := strs(map[string]string{
		"count": "many", "ratio": "half", "on": "yes", "glyph": "ab",
	})

	_, err := a.Int("count", 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `<Widget> count: expected a whole number, got "many"`)

	_, err = a.Float("ratio", 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected a number")

	_, err = a.Bool("on", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected true or false")

	_, err = a.Rune("glyph", ' ')
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected a single character")

	_, err = a.Enum("count", "", "one", "two")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected one of one, two")
}

func TestAttrsReadTypedValues(t *testing.T) {
	list, err := sema.ParseValue(sema.Type{Kind: sema.KindInt, IsList: true}, "3, 1, 2")
	require.NoError(t, err)
	a := attrs(map[string]sema.Value{
		"count":  mustValue(t, sema.Type{Kind: sema.KindInt}, "42"),
		"on":     sema.BoolValue(true),
		"values": list,
	})

	n, err := a.Int("count", 0)
	require.NoError(t, err)
	assert.Equal(t, 42, n)

	f, err := a.Float("count", 0)
	require.NoError(t, err)
	assert.InDelta(t, 42.0, f, 0, "an int reads as a number too")

	on, err := a.Bool("on", false)
	require.NoError(t, err)
	assert.True(t, on)

	assert.Equal(t, []string{"3", "1", "2"}, a.List("values"))
	assert.Equal(t, []string{"count", "on", "values"}, sorted(a.Names()))
}

// A native element's attributes are untyped text, so a list written there has to
// split the same way a typed list does.
func TestAttrsSplitAnUntypedList(t *testing.T) {
	a := strs(map[string]string{"values": "3, 1 ,2", "blank": "  "})

	assert.Equal(t, []string{"3", "1", "2"}, a.List("values"))
	assert.Nil(t, a.List("blank"))
}

func TestAttrsExposeTheRawValue(t *testing.T) {
	a := strs(map[string]string{"label": "Save"})

	value, ok := a.Value("label")
	require.True(t, ok)
	assert.Equal(t, "Save", value.String())
	assert.Equal(t, "Widget", a.Element())

	_, ok = a.Value("missing")
	assert.False(t, ok)
}

func mustValue(t *testing.T, typ sema.Type, raw string) sema.Value {
	t.Helper()
	value, err := sema.ParseValue(typ, raw)
	require.NoError(t, err)
	return value
}

func sorted(in []string) []string {
	out := append([]string{}, in...)
	for i := range out {
		for j := i + 1; j < len(out); j++ {
			if out[j] < out[i] {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}
