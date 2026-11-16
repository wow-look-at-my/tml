// button.go holds the button code.

package widgets

import (
	"fmt"

	"github.com/wow-look-at-my/tml/cell"
	"github.com/wow-look-at-my/tml/sema"
	"github.com/wow-look-at-my/tml/widget"
)

var buttonAttrs = []string{"label", "disabled", "variant"}

// buttonSlots is the region a button's content goes in.
var buttonSlots = []string{"Content"}

// buttonChrome is the element a button is drawn as. The label, or whatever is written inside, is its content.
var buttonChrome = map[string]string{"kind": "rounded", "pad": "1", "contentAlign": "center"}

// variants are the looks a button comes in, as a foreground colour for its border and label.
var variants = map[string]cell.Color{
	"default": {},
	"primary": cell.Indexed(12),
	"danger":  cell.Indexed(9),
}

var variantNames = []string{"danger", "default", "primary"}

// button is a control the keyboard and the pointer can reach.
type button struct {
	label    string
	disabled bool
	accent   cell.Color
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

// Measure is only reached by a button with no border, which the engine never builds.
func (b *button) Measure(_, _ int) (int, int) { return 0, 0 }

// Draw paints nothing: the border is the button's whole face, and the engine draws it as the button's child.
func (b *button) Draw(cell.Canvas) {}

// steer sets the border's look from the button's state. A focused button wears a double border, so the focus is
// obvious without colour.
func (b *button) steer() {
	if b.chrome == nil {
		return
	}
	border, _ := cell.BorderNamed("rounded")
	var color cell.Color
	var face cell.Style
	switch {
	case b.disabled:
		color = cell.Indexed(8)
		face = face.Faint(true)
	case b.state.Pressed:
		face = face.Reverse(true)
	case b.state.Focused:
		border, _ = cell.BorderNamed("double")
		color = b.focusColor()
		face = face.Bold(true)
	case b.state.Hovered:
		face = face.Underline(true)
	case b.accent.IsSet():
		color = b.accent
		face = face.Foreground(b.accent)
	}
	b.chrome.restyle(border, color, face)
}

func (b *button) focusColor() cell.Color {
	if b.accent.IsSet() {
		return b.accent
	}
	return cell.Indexed(13)
}
