package tml_test

import (
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml"
	"github.com/wow-look-at-my/tml/input"
)

// driveApp is the smallest host: it forwards every message it gets and consumes none of them.
type driveApp struct{ got chan tml.Msg }

func (a driveApp) Props() tml.Props { return nil }

func (a driveApp) Update(_ *tml.Program, msg tml.Msg) bool {
	if a.got != nil {
		select {
		case a.got <- msg:
		default:
		}
	}
	return false
}

// quietOptions is a program that reads nothing and draws on nothing.
func quietOptions() tml.ProgramOptions {
	return tml.ProgramOptions{NoInput: true, Output: io.Discard, Width: 40, Height: 10}
}

// runProgram runs p on a goroutine, and stops it when the test ends. The channel carries what Run returned.
func runProgram(t *testing.T, p *tml.Program) <-chan error {
	t.Helper()
	result := make(chan error, 1)
	go func() { result <- p.Run() }()
	t.Cleanup(func() {
		p.Quit()
		select {
		case <-p.Done():
		case <-time.After(5 * time.Second):
			t.Error("the program did not stop")
		}
	})
	return result
}

// A program built through NewProgram can be typed into; a program built any other way refuses by name.
func TestNewProgramIsWhatMakesAProgramDrivable(t *testing.T) {
	t.Serial("TestNewProgramIsWhatMakesAProgramDrivable resets the process's inspector, which every Load and NewProgram in this package uses")
	exclusive(t)
	view := loadInspectorView(t)
	insp := tml.Inspect()

	require.ErrorContains(t, insp.Key("a"), "tml.NewProgram",
		"a program this library did not build refuses input rather than dropping it")
	require.ErrorContains(t, insp.Click(1, 1), "tml.NewProgram")

	app := driveApp{got: make(chan tml.Msg, 64)}
	p, err := tml.NewProgram(view, app, quietOptions())
	require.NoError(t, err)
	runProgram(t, p)

	require.NoError(t, insp.Key("a"))
	assert.Equal(t, input.ParseKey("a"), waitFor[tml.Key](t, app.got),
		"the keystroke reached the app as the value a terminal would have sent")

	require.NoError(t, insp.Click(3, 4))
	press := waitFor[tml.Mouse](t, app.got)
	assert.Equal(t, tml.Mouse{X: 3, Y: 4, Button: input.ButtonLeft, Action: input.Press}, press)
	release := waitFor[tml.Mouse](t, app.got)
	assert.Equal(t, tml.Mouse{X: 3, Y: 4, Button: input.ButtonLeft, Action: input.Release}, release,
		"a click is a press and a release on one cell")
}

// waitFor reads messages until a message is of the wanted type, so an unrelated startup message does not decide what a test
func waitFor[T any](t *testing.T, got <-chan tml.Msg) T {
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
