// Command dashboard is a program whose entire view is declared in TML, and whose host is a few lines of state.
package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"strings"

	"github.com/wow-look-at-my/tml"
	"github.com/wow-look-at-my/tml/sema"
)

//go:embed ui
var uiFS embed.FS

var services = []string{"api", "web", "worker", "scheduler", "database"}

// model is everything the dashboard remembers: what is typed in the filter, and where the caret is.
type model struct {
	view   *tml.View
	query  string
	cursor int
}

func newModel() (*model, error) {
	ui, err := fs.Sub(uiFS, "ui")
	if err != nil {
		return nil, err
	}
	view, err := tml.Load(ui, "app.tml", tml.Options{Dark: true})
	if err != nil {
		return nil, err
	}
	return &model{view: view}, nil
}

// Update takes a message from the program. The filter box edits itself, and what it holds comes back as an event.
func (m *model) Update(p *tml.Program, msg tml.Msg) bool {
	switch msg := msg.(type) {
	case tml.Key:
		if msg.String() == "esc" {
			p.Quit()
			return true
		}
	case tml.Event:
		if msg.Kind == tml.Changed && msg.ID == "search" {
			m.query, m.cursor = msg.Value, msg.Cursor
		}
	}
	return false
}

// Props is what the document is drawn from.
func (m *model) Props() tml.Props {
	return tml.Props{
		"title":    sema.StringValue("Deployments"),
		"query":    sema.StringValue(m.query),
		"cursor":   sema.StringValue(strconv.Itoa(m.cursor)),
		"services": sema.StringValue(strings.Join(m.matches(), ",")),
	}
}

func (m *model) matches() []string {
	query := strings.TrimSpace(m.query)
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

// frame renders one frame as text, which keeps the whole UI reachable without a terminal.
func (m *model) frame(width, height int) string {
	out, err := m.view.Render(m.Props(), width, height)
	if err != nil {
		// A render failure is shown, never swallowed: a blank frame looks like an empty dashboard.
		return "tml: " + err.Error()
	}
	return out
}

func main() {
	// -frame renders a single frame and exits, so the example is checked without a terminal.
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
		fmt.Println(m.frame(*width, *height))
		return
	}
	if err := tml.Run(m.view, m); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
