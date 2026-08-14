package sema

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseType(t *testing.T) {
	tests := []struct {
		src     string
		want    string
		wantErr string
	}{
		{src: "string", want: "string"},
		{src: "int", want: "int"},
		{src: "bool", want: "bool"},
		{src: "color", want: "color"},
		{src: "length", want: "length"},
		{src: "thickness", want: "thickness"},
		{src: "string[]", want: "string[]"},
		{src: "enum(left|center|right)", want: "enum(left|center|right)"},
		{src: "enum(a|b)[]", want: "enum(a|b)[]"},
		{src: "list<string>", wantErr: "unknown type"},
		{src: "enum(only)", wantErr: "at least two members"},
		{src: "enum(a||b)", wantErr: "empty member"},
	}
	for _, tc := range tests {
		t.Run(tc.src, func(t *testing.T) {
			got, err := ParseType(tc.src)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got.String(), "a type must round-trip through its own spelling")
		})
	}
}

func TestParseLength(t *testing.T) {
	tests := []struct {
		src     string
		want    Length
		wantErr bool
	}{
		{src: "auto", want: Length{Kind: LengthAuto}},
		{src: "0", want: Length{Kind: LengthCells}},
		{src: "12", want: Length{Kind: LengthCells, Cells: 12}},
		{src: "*", want: Length{Kind: LengthStar, Weight: 1}},
		{src: "3*", want: Length{Kind: LengthStar, Weight: 3}},
		{src: "-1", wantErr: true},
		{src: "0*", wantErr: true},
		{src: "wide", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.src, func(t *testing.T) {
			got, err := ParseLength(tc.src)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestParseThickness(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		want    Thickness
		wantErr bool
	}{
		{name: "one value covers every side", src: "1", want: Thickness{1, 1, 1, 1}},
		{name: "two values are vertical then horizontal", src: "1 2", want: Thickness{1, 2, 1, 2}},
		{name: "four values run clockwise from the top", src: "1 2 3 4", want: Thickness{1, 2, 3, 4}},
		{name: "three values are not a shorthand", src: "1 2 3", wantErr: true},
		{name: "negative padding is meaningless", src: "-1", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseThickness(tc.src)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
			assert.Equal(t, tc.want.Left+tc.want.Right, got.Horizontal())
			assert.Equal(t, tc.want.Top+tc.want.Bottom, got.Vertical())
		})
	}
}

func TestParseValue(t *testing.T) {
	tests := []struct {
		name    string
		typ     string
		raw     string
		want    string
		wantErr string
	}{
		{name: "int", typ: "int", raw: "42", want: "42"},
		{name: "int rejects text", typ: "int", raw: "many", wantErr: "expected a whole number"},
		{name: "bool", typ: "bool", raw: "true", want: "true"},
		{name: "bool is strict", typ: "bool", raw: "yes", wantErr: "expected true or false"},
		{name: "hex colour", typ: "color", raw: "#e0af68", want: "#e0af68"},
		{name: "short hex colour", typ: "color", raw: "#fff", want: "#fff"},
		{name: "ansi index", typ: "color", raw: "205", want: "205"},
		{name: "colour rejects names", typ: "color", raw: "red", wantErr: "hex triplet"},
		{name: "colour rejects bad hex", typ: "color", raw: "#gggggg", wantErr: "non-hex digit"},
		{name: "colour rejects out of range index", typ: "color", raw: "300", wantErr: "0-255"},
		{name: "enum member", typ: "enum(a|b)", raw: "b", want: "b"},
		{name: "enum rejects a non-member", typ: "enum(a|b)", raw: "c", wantErr: "expected one of a, b"},
		{name: "list", typ: "string[]", raw: "api, web", want: "api,web"},
		{name: "empty list", typ: "string[]", raw: "", want: ""},
		{name: "list checks its element type", typ: "int[]", raw: "1,two", wantErr: "expected a whole number"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			typ, err := ParseType(tc.typ)
			require.NoError(t, err)

			got, err := ParseValue(typ, tc.raw)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got.String())
		})
	}
}

// Truthiness is defined only where it is unambiguous. A number has no obvious
// truth, so `if="{count}"` is a hard error rather than a silent "non-zero".
func TestTruthyIsDefinedOnlyWhereItIsUnambiguous(t *testing.T) {
	listType, err := ParseType("string[]")
	require.NoError(t, err)
	intType, err := ParseType("int")
	require.NoError(t, err)

	cases := []struct {
		name  string
		value Value
		want  bool
	}{
		{name: "true", value: BoolValue(true), want: true},
		{name: "false", value: BoolValue(false), want: false},
		{name: "non-empty string", value: StringValue("x"), want: true},
		{name: "empty string", value: StringValue(""), want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.value.Truthy()
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}

	empty, err := ParseValue(listType, "")
	require.NoError(t, err)
	got, err := empty.Truthy()
	require.NoError(t, err)
	assert.False(t, got, "an empty list is false")

	filled, err := ParseValue(listType, "a,b")
	require.NoError(t, err)
	got, err = filled.Truthy()
	require.NoError(t, err)
	assert.True(t, got)

	number, err := ParseValue(intType, "0")
	require.NoError(t, err)
	_, err = number.Truthy()
	require.Error(t, err, "a number has no defined truth")
	assert.Contains(t, err.Error(), "no truth value")
}

// A host's list has to survive its own contents. Joining the items into one
// string leaves them to be split on commas again, which is how one sentence
// becomes two entries.
func TestListValueKeepsItemsWithCommasInThem(t *testing.T) {
	value := ListValue([]string{"one, with a comma", "two"})

	require.True(t, value.Type().IsList)
	require.Len(t, value.Items(), 2)
	assert.Equal(t, "one, with a comma", value.Items()[0].String())

	truthy, err := value.Truthy()
	require.NoError(t, err)
	assert.True(t, truthy)

	empty, err := ListValue(nil).Truthy()
	require.NoError(t, err)
	assert.False(t, empty)
}
