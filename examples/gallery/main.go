// Command gallery is an interactive tour of the widget library.
//
// Every control on screen is declared in ui/app.tml and nothing here knows
// where any of them ended up: the model matches on the action strings the
// template wrote, which is the whole point of routing interaction through the
// view rather than through the layout.
package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"

	tea "charm.land/bubbletea/v2"

	"github.com/wow-look-at-my/tml"
	"github.com/wow-look-at-my/tml/sema"
)

//go:embed ui
var uiFS embed.FS

var services = []string{"api", "web", "worker", "scheduler", "database", "cache"}

// steps are table rows: cells joined by the separator <Table> splits on.
var steps = []string{
	"build|09:05|done",
	"test|09:12|done",
	"deploy|09:20|running",
}

type model struct {
	view *tml.View

	tab      string
	status   string
	query    string
	notify   bool
	size     string
	progress int
	frame    int
	load     []int

	selected   int
	offset     int
	log        []string
	confirming bool

	width, height int
	quitting      bool
}

// tick drives the spinner, the progress bar and the sparkline. A declarative
// widget draws the frame it is handed, so animation is the host's clock.
type tick time.Time

func ticker() tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(t time.Time) tea.Msg { return tick(t) })
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

	m := &model{
		view:   view,
		tab:    "controls",
		status: "ready",
		notify: true,
		size:   "medium",
		load:   []int{2, 5, 3, 8, 6, 9, 4, 7, 5, 9, 6, 3},
		width:  90,
		height: 30,
	}
	for i := 1; i <= 24; i++ {
		m.log = append(m.log, fmt.Sprintf("%02d:%02d deploy step %d finished", 9+i/12, i*5%60, i))
	}
	return m, nil
}

func (m *model) Init() tea.Cmd { return ticker() }

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tick:
		m.frame++
		m.progress = (m.progress + 3) % 101
		m.load = append(m.load[1:], 1+(m.frame*7)%9)
		return m, ticker()
	case tea.KeyPressMsg:
		if cmd, handled := m.hotkey(msg); handled {
			return m, cmd
		}
	}

	// Everything else goes to the view, which owns the focus ring and the
	// geometry a click resolves against.
	for _, event := range m.view.UI().Update(msg) {
		switch event.Kind {
		case tml.Activated:
			if !m.act(event.Action) {
				// A control the model has no answer for is a bug in this file,
				// and a button that does nothing when pressed looks exactly like
				// one that is broken. Say so on screen rather than swallowing it.
				m.status = "unhandled: " + event.Action
			}
		case tml.Scrolled:
			m.scroll(event.ID, event.Delta)
		}
	}
	if m.quitting {
		return m, tea.Quit
	}
	return m, nil
}

// hotkey handles the keys the host keeps for itself. Typing goes to the search
// box while it has focus, so the ring's own keys have to give way there.
func (m *model) hotkey(msg tea.KeyPressMsg) (tea.Cmd, bool) {
	focused, _ := m.view.UI().Focused()
	switch msg.String() {
	case "ctrl+c":
		return tea.Quit, true
	case "esc":
		if m.confirming {
			m.confirming = false
			return nil, true
		}
		return tea.Quit, true
	}
	if focused != "search" {
		if msg.String() == "q" {
			return tea.Quit, true
		}
		return nil, false
	}
	switch msg.String() {
	case "backspace":
		if m.query != "" {
			m.query = m.query[:len(m.query)-1]
		}
		return nil, true
	case "tab", "shift+tab", "enter", "up", "down":
		return nil, false
	}
	if r := msg.Code; unicode.IsPrint(r) && msg.Mod == 0 {
		m.query += string(r)
		return nil, true
	}
	return nil, false
}

// act runs one control's action and reports whether it was one this model knows.
func (m *model) act(action string) bool {
	kind, arg, _ := strings.Cut(action, ":")
	switch kind {
	case "tab":
		m.tab = arg
		m.status = arg
	case "toggle":
		m.notify = !m.notify
	case "size":
		m.size = arg
	case "focus":
		// The ring already moved; the field only had to be reachable.
	case "save":
		m.status = "saved"
	case "confirm-quit":
		m.confirming = true
	case "cancel":
		m.confirming = false
	case "quit":
		m.quitting = true
	default:
		return false
	}
	return true
}

func (m *model) scroll(id string, delta int) {
	switch id {
	case "log":
		m.offset = clamp(m.offset+delta, 0, len(m.log)-1)
	case "services":
		m.selected = clamp(m.selected+delta, 0, len(services)-1)
	}
}

func clamp(n, low, high int) int { return max(low, min(n, high)) }

func (m *model) View() tea.View {
	view := tea.NewView(m.frameOf())
	// All motion, not cell motion: cell motion only reports movement while a
	// button is held, so a button would never light up under an idle pointer.
	view.MouseMode = tea.MouseModeAllMotion
	return view
}

// frameOf renders one frame. Keeping it out of View is what lets -frame and any
// golden test reach the whole UI without a terminal.
func (m *model) frameOf() string {
	out, err := m.view.Render(tml.Props{
		"status":      sema.StringValue(m.status),
		"frame":       sema.StringValue(strconv.Itoa(m.frame)),
		"controlsTab": sema.BoolValue(m.tab == "controls"),
		"dataTab":     sema.BoolValue(m.tab == "data"),
		"mediaTab":    sema.BoolValue(m.tab == "media"),
		"query":       sema.StringValue(m.query),
		"cursor":      sema.StringValue(strconv.Itoa(len(m.query))),
		"notify":      sema.BoolValue(m.notify),
		"small":       sema.BoolValue(m.size == "small"),
		"medium":      sema.BoolValue(m.size == "medium"),
		"large":       sema.BoolValue(m.size == "large"),
		"progress":    sema.StringValue(strconv.Itoa(m.progress)),
		"load":        sema.StringValue(join(m.load)),
		"services":    sema.StringValue(strings.Join(services, ",")),
		"selected":    sema.StringValue(strconv.Itoa(m.selected)),
		"offset":      sema.StringValue(strconv.Itoa(m.offset)),
		"log":         sema.StringValue(strings.Join(m.log, ",")),
		"steps":       sema.StringValue(strings.Join(steps, ",")),
		"confirming":  sema.BoolValue(m.confirming),
	}, m.width, m.height)
	if err != nil {
		// A render failure is shown rather than swallowed: a blank frame looks
		// like an empty gallery instead of a broken one.
		return "tml: " + err.Error()
	}
	return out
}

func join(values []int) string {
	parts := make([]string, 0, len(values))
	for _, v := range values {
		parts = append(parts, strconv.Itoa(v))
	}
	return strings.Join(parts, ",")
}

func main() {
	frame := flag.Bool("frame", false, "render one frame to stdout and exit")
	width := flag.Int("width", 90, "viewport width for -frame")
	height := flag.Int("height", 26, "viewport height for -frame")
	tab := flag.String("tab", "controls", "section to show for -frame: controls, data or media")
	popup := flag.Bool("popup", false, "show the confirmation popup for -frame")
	focus := flag.String("focus", "", "id of the control to put the keyboard on for -frame")
	flag.Parse()

	m, err := newModel()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	if *frame {
		m.width, m.height = *width, *height
		m.tab, m.confirming = *tab, *popup
		// A still frame of an animation has to be taken somewhere other than at
		// the start, or the progress bar in it reads as a bar that does not work.
		m.query, m.progress, m.frame = "sched", 62, 3
		if *focus != "" {
			// Focus resolves against a frame's controls, so there has to be one
			// before there is anything to name.
			m.frameOf()
			if !m.view.UI().Focus(*focus) {
				fmt.Fprintf(os.Stderr, "error: no control with id %q on this frame\n", *focus)
				os.Exit(1)
			}
		}
		fmt.Println(m.frameOf())
		return
	}
	if _, err := tea.NewProgram(m).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
