// tooltip.go holds the tooltip: the text a control shows when the pointer rests on it.

package tml

import (
	"time"

	"github.com/wow-look-at-my/tml/cell"
	"github.com/wow-look-at-my/tml/input"
)

// DefaultTooltipDelay is how long the pointer rests on a control before the control's tooltip shows.
const DefaultTooltipDelay = time.Second

// tooltipWidth is the longest line of a tooltip, in cells. A longer text wraps.
const tooltipWidth = 48

// Tooltip is a tooltip on screen: its text, and the cell the pointer rests on.
type Tooltip struct {
	Text string
	X, Y int
}

// tooltips is what the UI knows about the pointer between frames.
type tooltips struct {
	// x and y are the cell the pointer was last reported on.
	x, y int
	// resting says the last input was the pointer moving with no button held.
	resting bool
	// dwell counts the inputs. A tooltip shows only when no input came during the wait for it.
	dwell int
	// shown is the tooltip on screen, or nil.
	shown *Tooltip
}

// Observe notes one input value for the tooltip. Any key or pointer input takes a tooltip away and starts the wait
// again. Update calls Observe. A host that keeps a value from Update calls Observe with it. It reports whether a
// tooltip went away, which needs a frame.
func (u *UI) Observe(msg any) bool {
	switch msg := msg.(type) {
	case input.Key, input.Paste:
		u.tips.resting = false
	case input.Mouse:
		u.tips.x, u.tips.y = msg.X, msg.Y
		u.tips.resting = msg.Action == input.Motion && msg.Button == input.ButtonNone
	default:
		return false
	}
	u.tips.dwell++
	hid := u.tips.shown != nil
	u.tips.shown = nil
	return hid
}

// Dwell names the wait that the last input started. It reports whether the
// pointer rests on a control that has a tooltip.
func (u *UI) Dwell() (dwell int, waits bool) {
	return u.tips.dwell, u.tips.resting && u.tooltipAt(u.tips.x, u.tips.y) != ""
}

// ShowTooltip shows the tooltip of the control under the pointer, when no input came since Dwell gave the name. It
// reports whether a tooltip came up, which needs a frame.
func (u *UI) ShowTooltip(dwell int) bool {
	if dwell != u.tips.dwell || !u.tips.resting || u.tips.shown != nil {
		return false
	}
	text := u.tooltipAt(u.tips.x, u.tips.y)
	if text == "" {
		return false
	}
	u.tips.shown = &Tooltip{Text: text, X: u.tips.x, Y: u.tips.y}
	return true
}

// Tooltip is the tooltip on screen.
func (u *UI) Tooltip() (Tooltip, bool) {
	if u.tips.shown == nil {
		return Tooltip{}, false
	}
	return *u.tips.shown, true
}

// tooltipAt is the tooltip of the control on top at a cell. A control that covers another hides its tooltip too.
func (u *UI) tooltipAt(x, y int) string {
	index := u.indexOf(u.hit(x, y))
	if index < 0 {
		return ""
	}
	return u.targets[index].Tooltip
}

// settleTooltip drops a tooltip whose control went away or changed its text in the frame laid out.
func (u *UI) settleTooltip() {
	if tip := u.tips.shown; tip != nil && u.tooltipAt(tip.X, tip.Y) != tip.Text {
		u.tips.shown = nil
	}
}

// drawTooltip paints the tooltip over the frame. It is a filled strip with no border, in the theme's tooltip
// colours, so it does not look like a menu. It sits under the pointer, or over it at the bottom of the screen.
func (u *UI) drawTooltip(frame *cell.Buffer, theme cell.Palette, measure cell.Measure) {
	tip := u.tips.shown
	if tip == nil || frame.W < 3 || frame.H < 1 {
		return
	}
	lines := measure.Wrap(tip.Text, min(tooltipWidth, frame.W-2))
	if len(lines) > frame.H {
		lines = lines[:frame.H]
	}
	width := 0
	for _, line := range lines {
		width = max(width, line.Width())
	}
	width += 2
	x := max(0, min(tip.X, frame.W-width))
	y := tip.Y + 1
	if y+len(lines) > frame.H {
		y = tip.Y - len(lines)
	}
	y = max(0, y)

	style := cell.Style{Fg: theme["tooltipText"], Bg: theme["tooltip"]}
	c := frame.Canvas(measure)
	c.Fill(x, y, width, len(lines), style)
	for i, line := range lines {
		c.Line(x+1, y+i, line, style)
	}
}
