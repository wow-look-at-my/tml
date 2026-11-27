package widgets

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml/cell"
	"github.com/wow-look-at-my/tml/input"
	"github.com/wow-look-at-my/tml/widget"
	"github.com/wow-look-at-my/tml/widget/widgettest"
)

func threeTabs(t *testing.T, pairs map[string]string) widget.Native {
	t.Helper()
	attrs := map[string]string{"items": "One,Two,Three"}
	for name, value := range pairs {
		attrs[name] = value
	}
	return build(t, "Tabs", attrs)
}

// Each label is padded by a cell on each side, and a cell of gap sits between a pair of labels.
func TestTabsDrawTheirLabelsInARow(t *testing.T) {
	strip := threeTabs(t, nil)
	w, h := strip.Measure(0, 0)
	assert.Equal(t, 5+1+5+1+7, w)
	assert.Equal(t, 1, h)
	assert.Equal(t, " One   Two   Three ", widgettest.Text(strip, 0, 0))
}

func TestTabsNameTheLabelUnderThePointer(t *testing.T) {
	strip := threeTabs(t, nil)
	widgettest.Draw(strip, 0, 0)
	parted := strip.(widget.Parted)

	for x, want := range map[int]string{
		0: "tab:0", 4: "tab:0", 5: "", 6: "tab:1", 10: "tab:1", 11: "", 12: "tab:2", 18: "tab:2", 19: "",
	} {
		assert.Equal(t, want, parted.Part(x, 0), "x=%d", x)
	}
	assert.Empty(t, parted.Part(0, 1), "below the strip")
}

// The selected label carries the theme's primary colour. Focus reverses it as well.
func TestTabsMarkTheSelectedLabel(t *testing.T) {
	strip := threeTabs(t, map[string]string{"selected": "1"})
	buf := widgettest.Draw(strip, 0, 0)
	primary, err := theme(false).Named("primary")
	require.NoError(t, err)

	assert.Equal(t, primary, buf.At(7, 0).Style.Fg, "the selected label")
	assert.True(t, buf.At(7, 0).Style.Has(cell.Bold))
	assert.False(t, buf.At(1, 0).Style.Fg.IsSet(), "a label that is not selected")
	assert.False(t, buf.At(7, 0).Style.Has(cell.Reverse), "no focus, no reverse")

	strip.(widget.Stateful).SetState(widget.State{Focused: true})
	buf = widgettest.Draw(strip, 0, 0)
	assert.True(t, buf.At(7, 0).Style.Has(cell.Reverse), "focus reverses the selected label")
	assert.False(t, buf.At(1, 0).Style.Has(cell.Reverse))
}

func TestDisabledTabsRefuseFocusPartsAndKeys(t *testing.T) {
	strip := threeTabs(t, map[string]string{"disabled": "true"})
	assert.False(t, strip.(widget.Focusable).AcceptsFocus())
	widgettest.Draw(strip, 0, 0)
	assert.Empty(t, strip.(widget.Parted).Part(1, 0))
	_, taken := strip.(widget.Keyed).Key(input.ParseKey("right"))
	assert.False(t, taken)

	assert.True(t, threeTabs(t, nil).(widget.Focusable).AcceptsFocus())
	assert.False(t, build(t, "Tabs", nil).(widget.Focusable).AcceptsFocus(), "a strip with no labels")
}

// A strip narrower than its labels cuts the label that crosses the edge and drops the rest.
func TestNarrowTabsCutRatherThanOverflow(t *testing.T) {
	strip := threeTabs(t, nil)
	assert.Equal(t, " One   T… ", widgettest.Text(strip, 10, 1))
	parted := strip.(widget.Parted)
	assert.Equal(t, "tab:1", parted.Part(9, 0))

	assert.Equal(t, " One  ", widgettest.Text(strip, 6, 1), "no room for a second label")
	assert.Empty(t, parted.Part(5, 0))
	w, _ := strip.Measure(6, 0)
	assert.Equal(t, 6, w)
}

// Left and right report the index of the new tab. At either end the key goes to the focus ring.
func TestTabsMoveTheSelectionWithTheArrows(t *testing.T) {
	key := func(selected, stroke string) (widget.Edit, bool) {
		return threeTabs(t, map[string]string{"selected": selected}).(widget.Keyed).Key(input.ParseKey(stroke))
	}

	edit, taken := key("1", "right")
	require.True(t, taken)
	assert.Equal(t, widget.Edit{Value: "2", Cursor: 2}, edit)

	edit, taken = key("1", "left")
	require.True(t, taken)
	assert.Equal(t, widget.Edit{Value: "0", Cursor: 0}, edit)

	_, taken = key("0", "left")
	assert.False(t, taken, "left at the first tab")
	_, taken = key("2", "right")
	assert.False(t, taken, "right at the last tab")
	_, taken = key("1", "x")
	assert.False(t, taken, "a key the strip has no use for")
}
