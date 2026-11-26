// ui.go holds the ui code.

package tml

import (
	"strconv"

	"github.com/wow-look-at-my/tml/input"
	"github.com/wow-look-at-my/tml/layout"
	"github.com/wow-look-at-my/tml/widget"
)

// Target is a single focusable control in the last rendered frame: what it is called, what it does, and where on the screen
type Target = layout.Target

// Scroll is how far a scrolling region's content is shifted, and how far it could be.
type Scroll = layout.Scroll

// EventKind is what happened to an interactive element.
type EventKind int

const (
	// Activated is a control being pressed: enter or space on the focused control, or a click released over it.
	Activated EventKind = iota
	// FocusMoved is the keyboard landing on a different control.
	FocusMoved
	// Scrolled is a wheel turn over a control, with Delta in notches: negative up, positive down.
	Scrolled
	// Changed is a key or a paste editing the focused control. Value and Cursor are what the control holds now.
	Changed
	// Menu is the right button pressed over a control: the request for a context menu at that point.
	Menu
)

func (k EventKind) String() string {
	switch k {
	case Activated:
		return "activated"
	case FocusMoved:
		return "focus-moved"
	case Scrolled:
		return "scrolled"
	case Changed:
		return "changed"
	case Menu:
		return "menu"
	default:
		return "invalid"
	}
}

// Event is a single interaction, reported back to the host. Action is the element's action attribute.
type Event struct {
	Kind   EventKind
	ID     string
	Action string
	Delta  int
	// X and Y are where the pointer was inside the control, in cells from its top-left. A widget that draws several
	X, Y int
	// Part is what the control drew under the pointer, for a control that names its parts.
	Part string
	// Mod is the modifier keys held with a pointer event.
	Mod input.Mod
	// Value and Cursor are a Changed control's new value and caret column.
	Value  string
	Cursor int
}

// KeyMap is which keys drive the focus ring.
type KeyMap struct {
	Next     []string
	Prev     []string
	Activate []string
}

// DefaultKeyMap moves with tab and the arrows and fires on enter or space.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Next:     []string{"tab", "down", "right"},
		Prev:     []string{"shift+tab", "up", "left"},
		Activate: []string{"enter", "space"},
	}
}

// UI is a view's interaction state. The cycle is a single frame long: laying out asks the UI how each control should draw,
type UI struct {
	keys    KeyMap
	focus   string
	hover   string
	press   string
	targets []layout.Target
	// drag is the scrolling region whose scrollbar the left button holds, or "" when no bar is held.
	drag string
	// dragGrab is the row of the thumb the pointer holds, and dragAt the offset the drag last asked for.
	dragGrab, dragAt int
	tips             tooltips
}

// pointerState is what a frame draws of the pointer: the element under it,
// the element it holds.
type pointerState struct {
	hover, press, drag string
	tip                bool
}

func (u *UI) pointerState() pointerState {
	return pointerState{hover: u.hover, press: u.press, drag: u.drag, tip: u.tips.shown != nil}
}

// NewUI returns interaction state with the default key map.
func NewUI() *UI { return &UI{keys: DefaultKeyMap()} }

// SetKeyMap replaces the bindings.
func (u *UI) SetKeyMap(keys KeyMap) { u.keys = keys }

// States implements layout.Interaction.
func (u *UI) States(targets []layout.Target) []widget.State {
	// Nothing has been focused yet, so the keyboard starts on the frame's leading control rather than nowhere: a view whose
	focus := u.focus
	if focus == "" {
		focus = firstFocusable(targets)
	}

	states := make([]widget.State, 0, len(targets))
	for i, target := range targets {
		key := targetKey(i, target.ID)
		states = append(states, widget.State{
			Focused: target.Focus && focus == key,
			Hovered: u.hover == key,
			Pressed: u.press == key,
		})
	}
	return states
}

// Frame implements layout.Interaction.
func (u *UI) Frame(targets []layout.Target) {
	u.targets = targets
	// A control can disappear between frames -- a popup closes, an `if` flips. Falling back to the leading focusable keeps
	if index := u.indexOf(u.focus); index < 0 || !targets[index].Focus {
		u.focus = firstFocusable(targets)
	}
	u.settleTooltip()
}

func firstFocusable(targets []layout.Target) string {
	for i, target := range targets {
		if target.Focus {
			return targetKey(i, target.ID)
		}
	}
	return ""
}

// Update feeds one input value through the focus ring and reports what the user did. A value that means nothing to
// the ring reports nothing. A Program calls this for the host. A host with a loop of its own calls it.
func (u *UI) Update(msg any) []Event {
	u.Observe(msg)
	switch msg := msg.(type) {
	case input.Key:
		if events, taken := u.edit(msg); taken {
			return events
		}
		return u.key(msg.Keystroke())
	case input.Paste:
		events, _ := u.edit(input.Key{Text: msg.Text})
		return events
	case input.Mouse:
		return u.pointer(msg)
	}
	return nil
}

// edit offers a key to the focused control when that control takes keys itself.
func (u *UI) edit(key input.Key) ([]Event, bool) {
	index := u.indexOf(u.focus)
	if index < 0 || u.targets[index].Keys == nil {
		return nil, false
	}
	change, taken := u.targets[index].Keys.Key(key)
	if !taken {
		return nil, false
	}
	target := u.targets[index]
	return []Event{{
		Kind: Changed, ID: target.ID, Action: target.Action,
		X: -1, Y: -1, Value: change.Value, Cursor: change.Cursor,
	}}, true
}

func (u *UI) pointer(msg input.Mouse) []Event {
	switch msg.Action {
	case input.Motion:
		if u.drag != "" && msg.Button == input.ButtonLeft {
			return u.dragTo(msg.Y)
		}
		u.hover = u.hit(msg.X, msg.Y)
	case input.Press:
		target := u.hit(msg.X, msg.Y)
		u.hover = target
		if msg.Button == input.ButtonRight {
			if target == "" {
				return nil
			}
			return u.at(target, Menu, 0, msg)
		}
		if msg.Button != input.ButtonLeft {
			return nil
		}
		if u.onBar(target, msg.X, msg.Y) {
			return u.grab(target, msg.Y)
		}
		u.press = target
		if target == "" {
			return nil
		}
		return u.focusKey(target)
	case input.Release:
		if u.drag != "" {
			u.drag = ""
			return nil
		}
		pressed := u.press
		u.press = ""
		target := u.hit(msg.X, msg.Y)
		u.hover = target
		// Releasing somewhere else cancels.
		if pressed == "" || target != pressed {
			return nil
		}
		return u.at(target, Activated, 0, msg)
	case input.Wheel:
		target := u.hit(msg.X, msg.Y)
		if target == "" {
			return nil
		}
		switch msg.Button {
		case input.WheelUp:
			return u.at(target, Scrolled, -1, msg)
		case input.WheelDown:
			return u.at(target, Scrolled, 1, msg)
		}
	}
	return nil
}

// bar is the scrollbar a scrolling target draws in its last column, and the
// row of the screen it starts on.
type bar struct {
	top int
	widget.Scrollbar
}

// barOf is the bar of a target that has a thumb with room to move.
func (u *UI) barOf(key string) (bar, bool) {
	index := u.indexOf(key)
	if index < 0 {
		return bar{}, false
	}
	target := u.targets[index]
	b := bar{top: target.Box.Y, Scrollbar: widget.Scrollbar{
		View: target.Box.H, Y: target.Scroll.Y, MaxY: target.Scroll.MaxY,
	}}
	_, size, ok := b.Thumb()
	return b, ok && size < b.View
}

// onBar reports whether a point is on the scrollbar of the target under it.
func (u *UI) onBar(key string, x, y int) bool {
	index := u.indexOf(key)
	if index < 0 {
		return false
	}
	box := u.targets[index].Box
	_, ok := u.barOf(key)
	return ok && x == box.X+box.W-1 && y >= box.Y && y < box.Y+box.H
}

// grab takes hold of a scrollbar. A press on the thumb holds it where it was pressed. A press on the track holds the
// thumb by its middle, so it jumps under the pointer.
func (u *UI) grab(key string, y int) []Event {
	b, _ := u.barOf(key)
	start, size, _ := b.Thumb()
	row := y - b.top
	u.drag, u.dragGrab = key, size/2
	if row >= start && row < start+size {
		u.dragGrab = row - start
	}
	u.dragAt = b.Y
	return u.dragTo(y)
}

// dragTo moves the held thumb under the pointer. It reports the move as a Scrolled event whose Delta is the rows
// the content has to shift. A host that already scrolls on the wheel scrolls on the bar with no new code.
func (u *UI) dragTo(y int) []Event {
	b, ok := u.barOf(u.drag)
	if !ok {
		u.drag = ""
		return nil
	}
	want := b.At(y-b.top, u.dragGrab)
	delta := want - u.dragAt
	if delta == 0 {
		return nil
	}
	u.dragAt = want
	return u.event(u.drag, Scrolled, delta, -1, -1)
}

func (u *UI) key(stroke string) []Event {
	switch {
	case contains(u.keys.Next, stroke):
		return u.move(1)
	case contains(u.keys.Prev, stroke):
		return u.move(-1)
	case contains(u.keys.Activate, stroke):
		if u.focus == "" {
			return nil
		}
		return u.activate(u.focus)
	}
	return nil
}

// move steps the ring, wrapping at both ends so tab never dead-ends. Elements that only answer the pointer are stepped
func (u *UI) move(step int) []Event {
	ring := u.ring()
	if len(ring) == 0 {
		return nil
	}
	at := 0
	for i, index := range ring {
		if targetKey(index, u.targets[index].ID) == u.focus {
			at = (i + step + len(ring)) % len(ring)
			break
		}
	}
	index := ring[at]
	return u.focusKey(targetKey(index, u.targets[index].ID))
}

// ring is the indices of the targets the keyboard can reach, in document order.
func (u *UI) ring() []int {
	var ring []int
	for i, target := range u.targets {
		if target.Focus {
			ring = append(ring, i)
		}
	}
	return ring
}

// Focus moves the keyboard to the control with the given id. It reports whether such a control took focus in the last
func (u *UI) Focus(id string) bool {
	for i, target := range u.targets {
		if target.ID == id && id != "" && target.Focus {
			u.focus = targetKey(i, target.ID)
			return true
		}
	}
	return false
}

// Focused is the id and action of the control the keyboard is on.
func (u *UI) Focused() (id, action string) {
	index := u.indexOf(u.focus)
	if index < 0 {
		return "", ""
	}
	return u.targets[index].ID, u.targets[index].Action
}

// Targets is every interactive element in the last frame, in document order: the focus ring plus anything the pointer
func (u *UI) Targets() []layout.Target { return u.targets }

// Target is the element with the given id in the last frame. A host reads it to find out where something landed -- how
func (u *UI) Target(id string) (layout.Target, bool) {
	for _, target := range u.targets {
		if target.ID == id && id != "" {
			return target, true
		}
	}
	return layout.Target{}, false
}

func (u *UI) focusKey(key string) []Event {
	index := u.indexOf(key)
	if u.focus == key || index < 0 || !u.targets[index].Focus {
		return nil
	}
	u.focus = key
	id, action := u.Focused()
	return []Event{{Kind: FocusMoved, ID: id, Action: action, X: -1, Y: -1}}
}

// activate is the keyboard firing a control, so there is no pointer to report.
func (u *UI) activate(key string) []Event {
	return u.event(key, Activated, 0, -1, -1)
}

// at is a pointer event. That event is with the screen point turned into an offset. This is inside the control it
// landed on, and the part of the control it landed on.
func (u *UI) at(key string, kind EventKind, delta int, msg input.Mouse) []Event {
	index := u.indexOf(key)
	if index < 0 {
		return nil
	}
	target := u.targets[index]
	events := u.event(key, kind, delta, msg.X-target.Rect.X, msg.Y-target.Rect.Y)
	events[0].Mod = msg.Mod
	if target.Part != nil {
		events[0].Part = target.Part.Part(msg.X-target.Box.X, msg.Y-target.Box.Y)
	}
	return events
}

func (u *UI) event(key string, kind EventKind, delta, x, y int) []Event {
	index := u.indexOf(key)
	if index < 0 {
		return nil
	}
	return []Event{{
		Kind:   kind,
		ID:     u.targets[index].ID,
		Action: u.targets[index].Action,
		Delta:  delta,
		X:      x,
		Y:      y,
	}}
}

// hit finds the control under a point. Later targets win.
func (u *UI) hit(x, y int) string {
	found := ""
	for i, target := range u.targets {
		r := target.Rect
		if x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H {
			found = targetKey(i, target.ID)
		}
	}
	return found
}

func (u *UI) indexOf(key string) int {
	if key == "" {
		return -1
	}
	for i, target := range u.targets {
		if targetKey(i, target.ID) == key {
			return i
		}
	}
	return -1
}

// targetKey identifies a control across frames. An id survives the control moving; without an id, position in the ring
func targetKey(index int, id string) string {
	if id != "" {
		return "#" + id
	}
	return "@" + strconv.Itoa(index)
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
