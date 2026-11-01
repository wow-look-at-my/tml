// button.go holds the button code.

package widgets

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/wow-look-at-my/tml/sema"
	"github.com/wow-look-at-my/tml/widget"
)

var buttonAttrs = []string{"label", "disabled", "variant"}

// buttonSlots is the region a button's content goes in.
var buttonSlots = []string{"Content"}

// buttonChrome is the element a button is drawn as. The label, or whatever is written inside, is its content.
var buttonChrome = map[string]string{"kind": "rounded", "pad": "1", "contentAlign": "center"}

// variants are the looks a button comes in, as a foreground colour for its border and label.
var variants = map[string]string{
	"default": "",
	"primary": "12",
	"danger":  "9",
}

var variantNames = []string{"danger", "default", "primary"}

// button is a control the keyboard and the pointer can reach.
type button struct {
	label    string
	disabled bool
	accent   string
	state    widget.State
	chrome   *frame
}

func newButton(ctx widget.Context) (widget.Native, error) {
	variant, err := ctx.Attrs.Enum("variant", "default", variantNames...)
	if err != nil {
		return nil, err
	}
	disabled, err := ctx.Attrs.Bool("disabled", false)
	if err != nil {
		return nil, err
	}
	return &button{
		label:    ctx.Attrs.String("label", ""),
		disabled: disabled,
		accent:   variants[variant],
	}, nil
}

// AcceptsFocus is false while disabled, which is what keeps tab from stopping on a control that would do nothing.
func (b *button) AcceptsFocus() bool { return !b.disabled }

func (b *button) SetState(state widget.State) {
	b.state = state
	b.steer()
}

func (b *button) Template() (string, map[string]string, string) {
	return "Border", buttonChrome, b.label
}

func (b *button) Bind(chrome widget.Native) error {
	border, ok := chrome.(*frame)
	if !ok {
		return fmt.Errorf("a button is drawn as the library's Border, got %T", chrome)
	}
	b.chrome = border
	b.steer()
	return nil
}

// Insets is nothing: the border is its own element, with its own insets.
func (b *button) Insets() sema.Thickness { return sema.Thickness{} }

func (b *button) Slots() []string { return buttonSlots }

// Measure and Render are only reached by a button with no border, which the engine never builds.
func (b *button) Measure(_, _ int) (int, int) { return 0, 0 }

func (b *button) Render(_, _ int) string { return "" }

// Compose hands back what the border drew.
func (b *button) Compose(slots widget.Slots, _, _ int) string {
	return strings.Join(slots.Default(), "\n")
}

// steer sets the border's look from the button's state. A focused button wears a double border, so the focus is
// obvious without colour.
func (b *button) steer() {
	if b.chrome == nil {
		return
	}
	border := lipgloss.RoundedBorder()
	color := ""
	face := lipgloss.NewStyle()
	switch {
	case b.disabled:
		color = "8"
		face = face.Faint(true)
	case b.state.Pressed:
		face = face.Reverse(true)
	case b.state.Focused:
		border = lipgloss.DoubleBorder()
		color = b.focusColor()
		face = face.Bold(true)
	case b.state.Hovered:
		face = face.Underline(true)
	case b.accent != "":
		color = b.accent
		face = face.Foreground(lipgloss.Color(b.accent))
	}
	b.chrome.restyle(border, color, face)
}

func (b *button) focusColor() string {
	if b.accent != "" {
		return b.accent
	}
	return "13"
}
