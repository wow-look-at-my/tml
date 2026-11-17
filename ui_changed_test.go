package tml_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml"
	"github.com/wow-look-at-my/tml/input"
)

// field is a text field ahead of a button, the field first in the ring, with the caret between its letters.
const field = `<Stack><Textbox id="name" action="rename" value="ab" cursor="1"/><Button id="ok" action="ok" label="OK"/></Stack>`

func TestAFocusedTextboxEditsItself(t *testing.T) {
	loaded := interactive(t, field, 30, 6)
	id, _ := loaded.UI().Focused()
	require.Equal(t, "name", id)

	events := loaded.UI().Update(input.ParseKey("x"))
	require.Len(t, events, 1)
	assert.Equal(t, tml.Event{Kind: tml.Changed, ID: "name", Action: "rename", X: -1, Y: -1, Value: "axb", Cursor: 2}, events[0],
		"a typed key comes back as the value and caret the host stores")

	events = loaded.UI().Update(input.ParseKey("backspace"))
	require.Len(t, events, 1)
	assert.Equal(t, "b", events[0].Value, "the field edits the value it was drawn with")
	assert.Equal(t, 0, events[0].Cursor)
}

func TestAPasteInsertsIntoTheFocusedTextbox(t *testing.T) {
	loaded := interactive(t, field, 30, 6)

	events := loaded.UI().Update(input.Paste{Text: "hi"})
	require.Len(t, events, 1)
	assert.Equal(t, tml.Changed, events[0].Kind)
	assert.Equal(t, "ahib", events[0].Value)
	assert.Equal(t, 3, events[0].Cursor)
}

// Enter and tab are not the field's, so the ring still activates and moves.
func TestATextboxLeavesEnterAndTabToTheRing(t *testing.T) {
	loaded := interactive(t, field, 30, 6)

	events := key(t, loaded, "enter", 30, 6)
	require.Len(t, events, 1)
	assert.Equal(t, tml.Event{Kind: tml.Activated, ID: "name", Action: "rename", X: -1, Y: -1}, events[0])

	events = key(t, loaded, "tab", 30, 6)
	require.Len(t, events, 1)
	assert.Equal(t, tml.Event{Kind: tml.FocusMoved, ID: "ok", Action: "ok", X: -1, Y: -1}, events[0])

	assert.Empty(t, key(t, loaded, "x", 30, 6), "a button takes no typing")
	assert.Empty(t, loaded.UI().Update(input.Paste{Text: "hi"}), "nor a paste")
}

// The frame says which controls take keys themselves.
func TestTheFrameMarksAControlThatTakesKeys(t *testing.T) {
	loaded := interactive(t, field, 30, 6)

	name, ok := loaded.UI().Target("name")
	require.True(t, ok)
	assert.NotNil(t, name.Keys)

	ok2, ok := loaded.UI().Target("ok")
	require.True(t, ok)
	assert.Nil(t, ok2.Keys)
}
