// Command dashboard is a Bubble Tea program whose entire view is declared in
// TML.
//
// The point of the example is the division of labour: the model still owns the
// text input and still runs its own Update, while app.tml owns the layout, the
// theme and the component structure. Binding the input as a <Search> element is
// the only place the two meet.
package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"strings"

	textinput "charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/wow-look-at-my/tml"
	"github.com/wow-look-at-my/tml/sema"
	"github.com/wow-look-at-my/tml/widget"
)

//go:embed ui
var uiFS embed.FS

var services = []string{"api", "web", "worker", "scheduler", "database"}

type model struct {
	view          *tml.View
	input         *textinput.Model
	width, height int
}

func newModel() (*model, error) {
	input := textinput.New()
	input.Placeholder = "type to filter"
	input.Focus()

	// The model keeps the component; TML is only given a reference to it.
	m := &model{input: &input, width: 80, height: 24}

	ui, err := fs.Sub(uiFS, "ui")
	if err != nil {
		return nil, err
	}
	widgets := widget.NewRegistry().Bind("Search", widget.Bubble(m.input))

	view, err := tml.Load(ui, "app.tml", tml.Options{Widgets: widgets, Dark: true})
	if err != nil {
		return nil, err
	}
	m.view = view
	return m, nil
}

func (m *model) Init() tea.Cmd { return nil }

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		}
	}

	updated, cmd := m.input.Update(msg)
	*m.input = updated
	return m, cmd
}

func (m *model) View() tea.View { return tea.NewView(m.frame()) }

// frame renders the view to a string. Splitting it out of View keeps the whole
// UI reachable without a terminal, which is what -frame and any future golden
// test need.
func (m *model) frame() string {
	out, err := m.view.Render(tml.Props{
		"title":    sema.StringValue("Deployments"),
		"query":    sema.StringValue(m.input.Value()),
		"services": sema.StringValue(strings.Join(m.matches(), ",")),
	}, m.width, m.height)
	if err != nil {
		// A render failure is shown, never swallowed: a blank frame would look
		// like an empty dashboard rather than a broken one.
		return "tml: " + err.Error()
	}
	return out
}

func (m *model) matches() []string {
	query := strings.TrimSpace(m.input.Value())
	if query == "" {
		return services
	}
	var out []string
	for _, service := range services {
		if strings.Contains(service, query) {
			out = append(out, service)
		}
	}
	return out
}

func main() {
	// -frame renders a single frame and exits, so the example can be smoke
	// tested without a terminal. Bubble Tea needs a TTY; the view does not.
	frame := flag.Bool("frame", false, "render one frame to stdout and exit")
	width := flag.Int("width", 60, "viewport width for -frame")
	height := flag.Int("height", 18, "viewport height for -frame")
	flag.Parse()

	m, err := newModel()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	if *frame {
		m.width, m.height = *width, *height
		fmt.Println(m.frame())
		return
	}
	if _, err := tea.NewProgram(m).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
