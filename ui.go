package tml

import (
	"strconv"

	tea "charm.land/bubbletea/v2"

	"github.com/wow-look-at-my/tml/layout"
	"github.com/wow-look-at-my/tml/widget"
)

// EventKind is what happened to an interactive element.
type EventKind int

const (
	// Activated is a control being pressed: enter or space on the focused one,
	// or a click released over it.
	Activated EventKind = iota
	// FocusMoved is the keyboard landing on a different control.
	FocusMoved
	// Scrolled is a wheel turn over a control, with Delta in notches: negative
	// up, positive down.
	Scrolled
)

func (k EventKind) String() string {
	switch k {
	case Activated:
		return "activated"
	case FocusMoved:
		return "focus-moved"
	case Scrolled:
		return "scrolled"
	default:
		return "invalid"
	}
}

// Event is one interaction, reported back to the host.
//
// Action is the element's action attribute: the string the template author
// chose to name what this control does. Matching on it keeps the host's Update
// free of any knowledge of where the control sits in the layout.
type Event struct {
	Kind   EventKind
	ID     string
	Action string
	Delta  int
}

// KeyMap is which keys drive the focus ring. Replace it when the host wants a
// key for itself -- a list that owns the arrows, say.
type KeyMap struct {
	Next     []string
	Prev     []string
	Activate []string
}

// DefaultKeyMap moves with tab and the arrows and fires on enter or space.
//
// The arrows are included because a row of buttons should respond to them
// without any setup. A host that needs them takes them back through KeyMap, or
// by not forwarding the message at all.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Next:     []string{"tab", "down", "right"},
		Prev:     []string{"shift+tab", "up", "left"},
		Activate: []string{"enter", "space"},
	}
}

// UI is a view's interaction state.
//
// The cycle is one frame long: laying out asks the UI how each control should
// draw, then hands back where every control landed, so the next key press or
// click resolves against the frame the user is actually looking at.
type UI struct {
	keys    KeyMap
	focus   string
	hover   string
	press   string
	targets []layout.Target
}

// NewUI returns interaction state with the default key map.
func NewUI() *UI { return &UI{keys: DefaultKeyMap()} }

// SetKeyMap replaces the bindings.
func (u *UI) SetKeyMap(keys KeyMap) { u.keys = keys }

// State implements layout.Interaction.
func (u *UI) State(index int, id, _ string) widget.State {
	key := targetKey(index, id)
	focused := u.focus == key
	if u.focus == "" {
		// Nothing has been focused yet, so the ring starts at its first control
		// rather than nowhere. A view whose buttons are all unfocused on the
		// first frame looks broken.
		focused = index == 0
	}
	return widget.State{Focused: focused, Hovered: u.hover == key, Pressed: u.press == key}
}

// Frame implements layout.Interaction.
func (u *UI) Frame(targets []layout.Target) {
	u.targets = targets
	if len(targets) == 0 {
		u.focus = ""
		return
	}
	// A control can disappear between frames -- a popup closes, an `if` flips.
	// Falling back to the first keeps the ring usable instead of stranding focus
	// on something no longer drawn.
	if u.indexOf(u.focus) < 0 {
		u.focus = targetKey(0, targets[0].ID)
	}
}

// Update feeds a Bubble Tea message through the focus ring and reports what the
// user did. Messages that mean nothing here are ignored, so forwarding
// everything is safe.
func (u *UI) Update(msg tea.Msg) []Event {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return u.key(msg.Keystroke())
	case tea.MouseMotionMsg:
		u.hover = u.hit(msg.X, msg.Y)
		return nil
	case tea.MouseClickMsg:
		if msg.Button != tea.MouseLeft {
			return nil
		}
		target := u.hit(msg.X, msg.Y)
		u.press = target
		u.hover = target
		if target == "" {
			return nil
		}
		return u.focusKey(target)
	case tea.MouseReleaseMsg:
		pressed := u.press
		u.press = ""
		target := u.hit(msg.X, msg.Y)
		u.hover = target
		// Releasing somewhere else cancels, which is what every other button in
		// every other toolkit does and what lets a misclick be taken back.
		if pressed == "" || target != pressed {
			return nil
		}
		return u.activate(target)
	case tea.MouseWheelMsg:
		target := u.hit(msg.X, msg.Y)
		if target == "" {
			return nil
		}
		switch msg.Button {
		case tea.MouseWheelUp:
			return u.event(target, Scrolled, -1)
		case tea.MouseWheelDown:
			return u.event(target, Scrolled, 1)
		}
	}
	return nil
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

// move steps the ring, wrapping at both ends so tab never dead-ends.
func (u *UI) move(step int) []Event {
	if len(u.targets) == 0 {
		return nil
	}
	index := u.indexOf(u.focus)
	if index < 0 {
		index = 0
	} else {
		index = (index + step + len(u.targets)) % len(u.targets)
	}
	return u.focusKey(targetKey(index, u.targets[index].ID))
}

// Focus moves the keyboard to the control with the given id. It reports whether
// such a control was in the last frame.
func (u *UI) Focus(id string) bool {
	for i, target := range u.targets {
		if target.ID == id && id != "" {
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

// Targets is the last frame's focus ring, in document order.
func (u *UI) Targets() []layout.Target { return u.targets }

func (u *UI) focusKey(key string) []Event {
	if u.focus == key {
		return nil
	}
	u.focus = key
	id, action := u.Focused()
	return []Event{{Kind: FocusMoved, ID: id, Action: action}}
}

func (u *UI) activate(key string) []Event {
	return u.event(key, Activated, 0)
}

func (u *UI) event(key string, kind EventKind, delta int) []Event {
	index := u.indexOf(key)
	if index < 0 {
		return nil
	}
	return []Event{{Kind: kind, ID: u.targets[index].ID, Action: u.targets[index].Action, Delta: delta}}
}

// hit finds the control under a point. Later targets win, because that is the
// order they are painted in and so the order they overlap in.
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

// targetKey identifies a control across frames. An id survives the control
// moving; without one, position in the ring is all there is to go on.
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
