package style

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml/syntax"
)

func theme(styles ...*syntax.Style) []*syntax.Theme {
	return []*syntax.Theme{{Name: "t", Styles: styles}}
}

func namedStyle(name, extends string, attrs map[string]string) *syntax.Style {
	style := &syntax.Style{Name: name, Extends: extends}
	for key, value := range attrs {
		style.Attrs = append(style.Attrs, syntax.Attr{Name: key, Value: value})
	}
	return style
}

// extends is resolved in TML's own model precisely because lipgloss Inherit drops padding. A style that extends another
func TestExtendsCarriesTheBoxModel(t *testing.T) {
	sheet, err := NewSheet(theme(
		namedStyle("card", "", map[string]string{"padding": "1 2", "border": "rounded"}),
		namedStyle("card.title", "card", map[string]string{"bold": "true"}),
	), nil)
	require.NoError(t, err)

	resolved, err := sheet.Resolve("card.title", nil)
	require.NoError(t, err)

	assert.True(t, resolved.Style.GetBold(), "the child's own attribute applies")
	top, right, bottom, left := resolved.Style.GetPadding()
	assert.Equal(t, []int{1, 2, 1, 2}, []int{top, right, bottom, left},
		"padding survives extends, which Inherit alone would have dropped")
}

func TestChildOverridesParent(t *testing.T) {
	sheet, err := NewSheet(theme(
		namedStyle("base", "", map[string]string{"padding": "4"}),
		namedStyle("tight", "base", map[string]string{"padding": "0"}),
	), nil)
	require.NoError(t, err)

	resolved, err := sheet.Resolve("tight", nil)
	require.NoError(t, err)
	assert.Equal(t, 0, resolved.Style.GetHorizontalPadding())
}

func TestInlineAttributesBeatTheNamedStyle(t *testing.T) {
	sheet, err := NewSheet(theme(namedStyle("card", "", map[string]string{"padding": "2"})), nil)
	require.NoError(t, err)

	resolved, err := sheet.Resolve("card", map[string]string{"padding": "0"})
	require.NoError(t, err)
	assert.Equal(t, 0, resolved.Style.GetHorizontalPadding())
}

func TestStyleAttributesResolveThemeTokens(t *testing.T) {
	sheet, err := NewSheet(
		theme(namedStyle("card", "", map[string]string{"padding": "{theme.gutter}"})),
		map[string]string{"gutter": "3"},
	)
	require.NoError(t, err)

	resolved, err := sheet.Resolve("card", nil)
	require.NoError(t, err)
	assert.Equal(t, 6, resolved.Style.GetHorizontalPadding(), "3 cells each side")
}

// Margin is deliberately kept off the lipgloss style: lipgloss treats the width set on a style as the border box and
func TestMarginIsKeptOutOfTheLipglossStyle(t *testing.T) {
	sheet, err := NewSheet(nil, nil)
	require.NoError(t, err)

	resolved, err := sheet.Resolve("", map[string]string{"margin": "1 2"})
	require.NoError(t, err)

	assert.Equal(t, 2, resolved.Margin.Left)
	assert.Equal(t, 0, resolved.Style.GetHorizontalMargins(), "the style itself carries no margin")
}

func TestFrameCountsPaddingAndBorder(t *testing.T) {
	sheet, err := NewSheet(nil, nil)
	require.NoError(t, err)

	resolved, err := sheet.Resolve("", map[string]string{"padding": "1 2", "border": "normal"})
	require.NoError(t, err)

	horizontal, vertical := resolved.Frame()
	assert.Equal(t, 6, horizontal, "4 cells of padding plus 2 of border")
	assert.Equal(t, 4, vertical, "2 cells of padding plus 2 of border")
}

func TestSheetRejects(t *testing.T) {
	t.Run("a style that extends itself", func(t *testing.T) {
		_, err := NewSheet(theme(namedStyle("a", "b", nil), namedStyle("b", "a", nil)), nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "extends itself")
	})

	t.Run("a duplicate style name", func(t *testing.T) {
		_, err := NewSheet(theme(namedStyle("a", "", nil), namedStyle("a", "", nil)), nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already declared")
	})

	t.Run("an unknown token", func(t *testing.T) {
		_, err := NewSheet(theme(namedStyle("a", "", map[string]string{"fg": "{theme.nope}"})), nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `unknown theme token "nope"`)
	})

	t.Run("a style referencing something other than a token", func(t *testing.T) {
		_, err := NewSheet(theme(namedStyle("a", "", map[string]string{"fg": "{accent}"})), nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "can only reference theme tokens")
	})
}

func TestResolveRejects(t *testing.T) {
	sheet, err := NewSheet(nil, nil)
	require.NoError(t, err)

	tests := []struct {
		name    string
		named   string
		inline  map[string]string
		wantErr string
	}{
		{name: "unknown named style", named: "nope", wantErr: `unknown style "nope"`},
		{name: "unknown attribute", inline: map[string]string{"colour": "red"}, wantErr: "unknown style attribute"},
		{name: "unknown border", inline: map[string]string{"border": "wiggly"}, wantErr: "unknown border"},
		{name: "non-boolean flag", inline: map[string]string{"bold": "yes"}, wantErr: "expected true or false"},
		{name: "bad alignment", inline: map[string]string{"align": "middle"}, wantErr: "align must be"},
		{name: "bad padding", inline: map[string]string{"padding": "1 2 3"}, wantErr: "takes 1, 2 or 4 values"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := sheet.Resolve(tc.named, tc.inline)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}
