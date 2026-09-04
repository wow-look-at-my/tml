package tml_test

import (
	"io"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml"
)

// driveModel is the smallest host: it forwards what the inspector sent it.
type driveModel struct{ got chan tea.Msg }

func (m driveModel) Init() tea.Cmd { return nil }

func (m driveModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, quit := msg.(tea.QuitMsg); quit {
		return m, tea.Quit
	}
	select {
	case m.got <- msg:
	default:
	}
	return m, nil
}

func (m driveModel) View() tea.View { return tea.NewView("") }

// A program built through NewProgram can be typed into; a program built any other way refuses by name. That difference is
func TestNewProgramIsWhatMakesAProgramDrivable(t *testing.T) {
	exclusive(t)
	loadInspectorView(t)
	insp := tml.Inspect()

	require.ErrorContains(t, insp.Key("a"), "tml.NewProgram",
		"a program this library did not build refuses input rather than dropping it")
	require.ErrorContains(t, insp.Click(1, 1), "tml.NewProgram")

	m := driveModel{got: make(chan tea.Msg, 16)}
	p, err := tml.NewProgram(m, tea.WithInput(strings.NewReader("")), tea.WithOutput(io.Discard))
	require.NoError(t, err)

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = p.Run()
	}()
	t.Cleanup(func() {
		p.Quit()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("the program did not stop")
		}
	})

	require.NoError(t, insp.Key("a"))
	assert.Equal(t, tml.KeyMsg("a"), waitFor[tea.KeyPressMsg](t, m.got),
		"the keystroke reached the model as the message a terminal would have sent")

	require.NoError(t, insp.Click(3, 4))
	click := waitFor[tea.MouseClickMsg](t, m.got)
	assert.Equal(t, 3, click.X)
	assert.Equal(t, 4, click.Y)
}

// waitFor reads messages until a message is of the wanted type, so an unrelated startup message does not decide what a test
func waitFor[T tea.Msg](t *testing.T, got <-chan tea.Msg) T {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case msg := <-got:
			if want, ok := msg.(T); ok {
				return want
			}
		case <-deadline:
			var zero T
			t.Fatalf("no %T arrived within 5s", zero)
			return zero
		}
	}
}
