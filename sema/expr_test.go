package sema

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testScope is a fixed set of names, standing in for a component's properties and its theme.
type testScope struct {
	props  map[string]Value
	tokens map[string]Value
}

func (s testScope) Lookup(name string) (Value, bool) { v, ok := s.props[name]; return v, ok }
func (s testScope) Token(name string) (Value, bool)  { v, ok := s.tokens[name]; return v, ok }

func TestParseExprSegments(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		wantRefs []string
		wantLit  bool
	}{
		{name: "plain text", raw: "hello", wantLit: true},
		{name: "sole reference", raw: "{title}", wantRefs: []string{"{title}"}},
		{name: "interpolation", raw: "Hello {name}!", wantRefs: []string{"{name}"}},
		{name: "several references", raw: "{a}-{b}", wantRefs: []string{"{a}", "{b}"}},
		{name: "theme token", raw: "{theme.accent}", wantRefs: []string{"{theme.accent}"}},
		{name: "negation", raw: "{not ready}", wantRefs: []string{"{not ready}"}},
		{name: "escaped braces are literal", raw: "{{title}}", wantLit: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			expr, err := ParseExpr(tc.raw)
			require.NoError(t, err)

			assert.Equal(t, tc.wantLit, expr.IsLiteral())

			got := make([]string, 0, len(expr.Refs()))
			for _, ref := range expr.Refs() {
				got = append(got, ref.String())
			}
			assert.Equal(t, tc.wantRefs, nilIfEmpty(got))
		})
	}
}

func nilIfEmpty(s []string) []string {
	if len(s) == 0 {
		return nil
	}
	return s
}

func TestEscapedBracesProduceLiteralText(t *testing.T) {
	expr, err := ParseExpr("{{title}}")
	require.NoError(t, err)

	value, err := expr.Eval(testScope{})
	require.NoError(t, err)
	assert.Equal(t, "{title}", value.String(), "{{ and }} are the only escaping the language has")
}

// A lone reference keeps its type, so padding="{gutter}" is still a thickness. Mixing a reference with text produces a
func TestSoleReferenceKeepsItsTypeButInterpolationDoesNot(t *testing.T) {
	thickness, err := ParseType("thickness")
	require.NoError(t, err)
	gutter, err := ParseValue(thickness, "1 2")
	require.NoError(t, err)

	scope := testScope{props: map[string]Value{"gutter": gutter}}

	sole, err := ParseExpr("{gutter}")
	require.NoError(t, err)
	value, err := sole.Eval(scope)
	require.NoError(t, err)
	assert.Equal(t, KindThickness, value.Type().Kind, "a lone reference passes the value through untouched")

	mixed, err := ParseExpr("pad:{gutter}")
	require.NoError(t, err)
	value, err = mixed.Eval(scope)
	require.NoError(t, err)
	assert.Equal(t, KindString, value.Type().Kind, "anything mixed with text becomes a string")
	assert.Equal(t, "pad:1 2 1 2", value.String())
}

func TestEvalResolvesPropertiesTokensAndNegation(t *testing.T) {
	scope := testScope{
		props:  map[string]Value{"name": StringValue("world"), "ready": BoolValue(false)},
		tokens: map[string]Value{"accent": StringValue("#e0af68")},
	}

	cases := []struct {
		raw  string
		want string
	}{
		{raw: "Hello {name}!", want: "Hello world!"},
		{raw: "{theme.accent}", want: "#e0af68"},
		{raw: "{not ready}", want: "true"},
	}
	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			expr, err := ParseExpr(tc.raw)
			require.NoError(t, err)
			value, err := expr.Eval(scope)
			require.NoError(t, err)
			assert.Equal(t, tc.want, value.String())
		})
	}
}

func TestParseExprRejects(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantErr string
	}{
		{name: "unclosed brace", raw: "{title", wantErr: "unclosed {"},
		{name: "stray closing brace", raw: "title}", wantErr: "unmatched }"},
		{name: "empty interpolation", raw: "{}", wantErr: "empty interpolation"},
		{name: "arithmetic is not in the grammar", raw: "{a + b}", wantErr: "cannot parse interpolation"},
		{name: "a call is not in the grammar", raw: "{upper(a)}", wantErr: "cannot parse interpolation"},
		{name: "only theme uses a dot", raw: "{user.name}", wantErr: "only theme.<token> uses a dot"},
		{name: "not with nothing to negate", raw: "{not }", wantErr: "names nothing"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseExpr(tc.raw)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

func TestEvalRejectsUnknownNamesAndUnnegatableValues(t *testing.T) {
	expr, err := ParseExpr("{missing}")
	require.NoError(t, err)
	_, err = expr.Eval(testScope{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown reference {missing}")

	intType, err := ParseType("int")
	require.NoError(t, err)
	count, err := ParseValue(intType, "3")
	require.NoError(t, err)

	expr, err = ParseExpr("{not count}")
	require.NoError(t, err)
	_, err = expr.Eval(testScope{props: map[string]Value{"count": count}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot negate")
}
