// Command gallery is an interactive tour of the widget library.
package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/wow-look-at-my/tml"
	"github.com/wow-look-at-my/tml/sema"
)

//go:embed ui
var uiFS embed.FS

// sections are the tabs, in the order the strip draws them.
var sections = []string{"controls", "data", "media"}

var services = []string{"api", "web", "worker", "scheduler", "database", "cache"}

// steps are table rows: cells joined by the separator <Table> splits on.
var steps = []string{
	"pipeline|09:05|running",
	"build|09:05|done",
	"test|09:12|done",
	"deploy|09:20|running",
}

type model struct {
	view *tml.View

	tab      string
	status   string
	query    string
	cursor   int
	notes    string
	// notesCursor is the caret in notes, in runes. A negative one is the end.
	notesCursor int
	notify      bool
	size     string
	progress int
	frame    int
	load     []int

	selected   int
	step       int
	folded     bool
	offset     int
	log        []string
	confirming bool

	width, height int
	quitting      bool
}

// tick drives the progress bar and the sparkline. The spinner needs none: the program steps a view's animations.
type tick struct{}

const tickEvery = 120 * time.Millisecond

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
		notes:  "first line\nsecond line",
		size:   "medium",
		// The caret starts at the end of the notes.
		notesCursor: -1,
		load:   []int{2, 5, 3, 8, 6, 9, 4, 7, 5, 9, 6, 3},
		width:  90,
		height: 30,
	}
	for i := 1; i <= 24; i++ {
		m.log = append(m.log, fmt.Sprintf("%02d:%02d deploy step %d finished", 9+i/12, i*5%60, i))
	}
	return m, nil
}

// Update takes a message from the program. A key it does not consume goes to the focus ring, which owns the
// geometry a click resolves against, and comes back as an event.
func (m *model) Update(p *tml.Program, msg tml.Msg) bool {
	switch msg := msg.(type) {
	case tick:
		m.progress = (m.progress + 3) % 101
		m.frame++
		m.load = append(m.load[1:], 1+(m.frame*7)%9)
	case tml.Key:
		return m.hotkey(p, msg)
	case tml.Event:
		m.event(msg)
		if m.quitting {
			p.Quit()
		}
	}
	return false
}

func (m *model) event(event tml.Event) {
	switch event.Kind {
	case tml.Changed:
		// The search box edits itself and holds nothing: the value and the caret come back here.
		if event.ID == "search" {
			m.query, m.cursor = event.Value, event.Cursor
		}
		// The notes keep their newlines, and the caret is an offset in runes.
		if event.ID == "notes" {
			m.notes, m.notesCursor = event.Value, event.Cursor
		}
		// The arrows on the tab strip report the index of the new tab.
		if event.ID == "tabs" {
			m.show(event.Cursor)
		}
	case tml.Activated:
		// A click on the tab strip names the tab it landed on.
		if event.ID == "tabs" {
			if index, ok := strings.CutPrefix(event.Part, "tab:"); ok {
				n, _ := strconv.Atoi(index)
				m.show(n)
			}
			return
		}
		// A list is a single control to the ring.
		if event.ID == "services" && event.Y >= 0 {
			m.selected = clamp(event.Y, 0, len(services)-1)
			return
		}
		// A table names the row a click landed on, and the disclosure glyph of a row in its tree.
		if event.ID == "steps" {
			if _, ok := strings.CutPrefix(event.Part, "toggle:"); ok {
				m.folded = !m.folded
				m.step = 0
			} else if row, ok := strings.CutPrefix(event.Part, "row:"); ok {
				n, _ := strconv.Atoi(row)
				m.step = clamp(n, 0, len(m.shownSteps())-1)
			}
			return
		}
		if event.Action != "" && !m.act(event.Action) {
			// A control the model has no answer for is a bug in this file, so it says so on screen.
			m.status = "unhandled: " + event.Action
		}
	case tml.Scrolled:
		m.scroll(event.ID, event.Delta)
	}
}

// hotkey handles the keys the host keeps for itself. A letter belongs to a text field while that field has the focus.
func (m *model) hotkey(p *tml.Program, key tml.Key) bool {
	focused, _ := m.view.UI().Focused()
	switch key.String() {
	case "ctrl+c":
		p.Quit()
		return true
	case "esc":
		if m.confirming {
			m.confirming = false
			return true
		}
		p.Quit()
		return true
	case "q":
		if focused != "search" && focused != "notes" {
			p.Quit()
			return true
		}
	}
	return false
}

// act runs a single control's action and reports whether it was a single this model knows.
func (m *model) act(action string) bool {
	kind, arg, _ := strings.Cut(action, ":")
	switch kind {
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

// show switches to the section at index n of sections.
func (m *model) show(n int) {
	m.tab = sections[clamp(n, 0, len(sections)-1)]
	m.status = m.tab
}

// section is the index of the section on screen, which is the tab the strip marks.
func (m *model) section() int {
	return max(0, slices.Index(sections, m.tab))
}

func (m *model) scroll(id string, delta int) {
	switch id {
	case "log":
		m.offset = clamp(m.offset+delta, 0, len(m.log)-1)
	case "services":
		m.selected = clamp(m.selected+delta, 0, len(services)-1)
	case "steps":
		m.step = clamp(m.step+delta, 0, len(m.shownSteps())-1)
	}
}

// shownSteps is the rows of the table: the run alone while it is folded, and the run with its steps otherwise.
func (m *model) shownSteps() []string {
	if m.folded {
		return steps[:1]
	}
	return steps
}

// tree is the place of each shown row in the table's tree.
func (m *model) tree() []string {
	if m.folded {
		return []string{"0:closed"}
	}
	out := []string{"0:open"}
	for range steps[1:] {
		out = append(out, "1:leaf")
	}
	return out
}

func clamp(n, low, high int) int { return max(low, min(n, high)) }

// frameOf renders a single frame as text, which is how -frame and the golden tests reach the whole UI without a
// terminal.
func (m *model) frameOf() string {
	out, err := m.view.Render(m.Props(), m.width, m.height)
	if err != nil {
		// A render failure is shown rather than swallowed: a blank frame looks like an empty gallery.
		return "tml: " + err.Error()
	}
	return out
}

// Props is what the document is drawn from.
func (m *model) Props() tml.Props {
	return tml.Props{
		"status":      sema.StringValue(m.status),
		"controlsTab": sema.BoolValue(m.tab == "controls"),
		"dataTab":     sema.BoolValue(m.tab == "data"),
		"mediaTab":    sema.BoolValue(m.tab == "media"),

		"section":    sema.StringValue(strconv.Itoa(m.section())),
		"query":      sema.StringValue(m.query),
		"cursor":      sema.StringValue(strconv.Itoa(m.cursor)),
		"notes":       sema.StringValue(m.notes),
		"notesCursor": sema.StringValue(strconv.Itoa(m.notesCursor)),
		"notify":     sema.BoolValue(m.notify),
		"small":      sema.BoolValue(m.size == "small"),
		"medium":     sema.BoolValue(m.size == "medium"),
		"large":      sema.BoolValue(m.size == "large"),
		"progress":   sema.StringValue(strconv.Itoa(m.progress)),
		"load":       sema.StringValue(join(m.load)),
		"services":   sema.StringValue(strings.Join(services, ",")),
		"selected":   sema.StringValue(strconv.Itoa(m.selected)),
		"offset":     sema.StringValue(strconv.Itoa(m.offset)),
		"log":        sema.StringValue(strings.Join(m.log, ",")),
		"steps":      sema.StringValue(strings.Join(m.shownSteps(), ",")),
		"tree":       sema.StringValue(strings.Join(m.tree(), ",")),
		"step":       sema.StringValue(strconv.Itoa(m.step)),
		"confirming": sema.BoolValue(m.confirming),
	}
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

	// The flags set the starting state either way round: a still frame and a terminal session are the same program.
	m.tab, m.confirming = *tab, *popup
	// An animation shown at its start reads as an animation that does not work, so both begin partway through.
	m.query, m.cursor, m.progress, m.frame = "sched", len("sched"), 62, 3
	if *frame {
		m.width, m.height = *width, *height
	}
	if *focus != "" {
		// Focus resolves against a frame's controls, so there has to be a frame before there is anything to name.
		m.frameOf()
		if !m.view.UI().Focus(*focus) {
			fmt.Fprintf(os.Stderr, "error: no control with id %q on this frame\n", *focus)
			os.Exit(1)
		}
	}
	if *frame {
		fmt.Println(m.frameOf())
		return
	}
	// Nothing here wires an inspector. Load adopted the view and opened the socket, and the program is what drives it.
	program, err := tml.NewProgram(m.view, m)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	program.Every(tickEvery, tick{})
	if err := program.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
