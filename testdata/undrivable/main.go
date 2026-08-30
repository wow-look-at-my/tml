// Command undrivable is the program the drivable guard exists to stop. It loads a document and then builds its Bubble
package main

import (
	"fmt"
	"io"
	"os"
	"testing/fstest"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/wow-look-at-my/tml"
)

const doc = `<?xml version="1.1" encoding="UTF-8"?>
<Component xmlns="urn:tml:v1" name="App">
	<Template><Stack id="app" width="20"><Text id="hello">hello</Text></Stack></Template>
</Component>`

type model struct{ view *tml.View }

// It paints as soon as and then waits, which is what a terminal program does between keystrokes and the harder case for the
func (m model) Init() tea.Cmd {
	return tea.Tick(time.Hour, func(time.Time) tea.Msg { return struct{}{} })
}

func (m model) Update(tea.Msg) (tea.Model, tea.Cmd) { return m, nil }

func (m model) View() tea.View {
	out, err := m.view.Render(nil, 40, 10)
	if err != nil {
		panic(err)
	}
	return tea.NewView(out)
}

func main() {
	view, err := tml.Load(fstest.MapFS{"app.tml": &fstest.MapFile{Data: []byte(doc)}},
		"app.tml", tml.Options{})
	if err != nil {
		fmt.Fprintln(os.Stderr, "load:", err)
		os.Exit(1)
	}
	// The whole point of this program: tea.NewProgram, not tml.NewProgram.
	if _, err := tea.NewProgram(model{view: view},
		tea.WithInput(nil), tea.WithOutput(io.Discard)).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "run:", err)
		os.Exit(1)
	}
}
