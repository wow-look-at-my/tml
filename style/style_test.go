package style

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml/cell"
	"github.com/wow-look-at-my/tml/sema"
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

// A style that extends another carries its box model along, padding and border included.
func TestExtendsCarriesTheBoxModel(t *testing.T) {
	sheet, err := NewSheet(theme(
		namedStyle("card", "", map[string]string{"padding": "1 2", "border": "rounded"}),
		namedStyle("card.title", "card", map[string]string{"bold": "true"}),
	), nil)
	require.NoError(t, err)

	resolved, err := sheet.Resolve("card.title", nil)
	require.NoError(t, err)

	assert.True(t, resolved.Paint.Has(cell.Bold), "the child's own attribute applies")
	assert.Equal(t, sema.Thickness{Top: 1, Right: 2, Bottom: 1, Left: 2}, resolved.Padding, "padding survives extends")
	assert.NotNil(t, resolved.Border, "so does the border")
}

func TestChildOverridesParent(t *testing.T) {
	sheet, err := NewSheet(theme(
		namedStyle("base", "", map[string]string{"padding": "4"}),
		namedStyle("tight", "base", map[string]string{"padding": "0"}),
	), nil)
	require.NoError(t, err)

	resolved, err := sheet.Resolve("tight", nil)
	require.NoError(t, err)
	assert.Equal(t, 0, resolved.Padding.Horizontal())
}

func TestInlineAttributesBeatTheNamedStyle(t *testing.T) {
	sheet, err := NewSheet(theme(namedStyle("card", "", map[string]string{"padding": "2"})), nil)
	require.NoError(t, err)

	resolved, err := sheet.Resolve("card", map[string]string{"padding": "0"})
	require.NoError(t, err)
	assert.Equal(t, 0, resolved.Padding.Horizontal())
}

func TestStyleAttributesResolveThemeTokens(t *testing.T) {
	sheet, err := NewSheet(
		theme(namedStyle("card", "", map[string]string{"padding": "{theme.gutter}"})),
		map[string]string{"gutter": "3"},
	)
	require.NoError(t, err)

	resolved, err := sheet.Resolve("card", nil)
	require.NoError(t, err)
	assert.Equal(t, 6, resolved.Padding.Horizontal(), "3 cells each side")
}

// Margin sits outside the frame, so it neither counts as padding nor moves the frame.
func TestMarginIsKeptOutOfTheFrame(t *testing.T) {
	sheet, err := NewSheet(nil, nil)
	require.NoError(t, err)

	resolved, err := sheet.Resolve("", map[string]string{"margin": "1 2"})
	require.NoError(t, err)

	assert.Equal(t, 2, resolved.Margin.Left)
	assert.Equal(t, 0, resolved.Padding.Horizontal(), "margin is not padding")
	horizontal, _ := resolved.Frame()
	assert.Equal(t, 0, horizontal, "the frame carries no margin")
	x, y := resolved.ContentOffset()
	assert.Equal(t, []int{2, 1}, []int{x, y}, "the content starts past the margin")
}

func TestColoursAndTextAttributesLandOnThePaint(t *testing.T) {
	sheet, err := NewSheet(nil, nil)
	require.NoError(t, err)

	resolved, err := sheet.Resolve("", map[string]string{
		"fg": "#ff0000", "bg": "12", "borderColor": "red", "border": "rounded",
		"bold": "true", "italic": "true", "underline": "false",
	})
	require.NoError(t, err)

	assert.Equal(t, cell.MustColor("#ff0000"), resolved.Paint.Fg)
	assert.Equal(t, cell.Indexed(12), resolved.Paint.Bg)
	assert.True(t, resolved.Paint.Has(cell.Bold|cell.Italic))
	assert.False(t, resolved.Paint.Has(cell.Underline))
	require.NotNil(t, resolved.Border)
	rounded, _ := cell.BorderNamed("rounded")
	assert.Equal(t, rounded, *resolved.Border)
	assert.Equal(t, cell.MustColor("red"), resolved.BorderPaint.Fg, "the border colour paints the border only")
}

func TestBorderNoneDrawsNoBorder(t *testing.T) {
	sheet, err := NewSheet(nil, nil)
	require.NoError(t, err)

	resolved, err := sheet.Resolve("", map[string]string{"border": "none"})
	require.NoError(t, err)
	assert.Nil(t, resolved.Border)
}

func TestAlignmentNamesMapToAlign(t *testing.T) {
	sheet, err := NewSheet(nil, nil)
	require.NoError(t, err)

	tests := []struct {
		align, valign string
		want, wantV   Align
	}{
		{"left", "top", AlignStart, AlignStart},
		{"center", "middle", AlignCenter, AlignCenter},
		{"right", "bottom", AlignEnd, AlignEnd},
	}
	for _, tc := range tests {
		resolved, err := sheet.Resolve("", map[string]string{"align": tc.align, "valign": tc.valign})
		require.NoError(t, err)
		assert.Equal(t, tc.want, resolved.Align, tc.align)
		assert.Equal(t, tc.wantV, resolved.VAlign, tc.valign)
	}
}

func TestAlignOffset(t *testing.T) {
	assert.Equal(t, 0, AlignStart.Offset(4, 10))
	assert.Equal(t, 3, AlignCenter.Offset(4, 10))
	assert.Equal(t, 4, AlignCenter.Offset(3, 10), "the middle rounds toward the end")
	assert.Equal(t, 6, AlignEnd.Offset(4, 10))
	assert.Equal(t, 0, AlignEnd.Offset(12, 10), "content larger than the space starts at the start")
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
		{name: "bad vertical alignment", inline: map[string]string{"valign": "center"}, wantErr: "valign must be"},
		{name: "bad foreground", inline: map[string]string{"fg": "nope"}, wantErr: "fg: "},
		{name: "bad background", inline: map[string]string{"bg": "nope"}, wantErr: "bg: "},
		{name: "bad border colour", inline: map[string]string{"borderColor": "nope"}, wantErr: "borderColor: "},
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
