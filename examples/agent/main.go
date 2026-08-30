// Command agent is a mock coding agent: the shape of an AI harness, with a script where the model would be. It exists
package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"time"
	"unicode"

	tea "charm.land/bubbletea/v2"

	"github.com/wow-look-at-my/tml"
	"github.com/wow-look-at-my/tml/sema"
	"github.com/wow-look-at-my/tml/widget"
)

//go:embed ui
var uiFS embed.FS

// beat is a single step of the script. The model is a list.
type beat struct {
	role, text string
	status     string
	working    bool
	tokens     int
	file       int
	patch      bool
	results    bool
	// ask stops the script until the user answers. Everything after it happens only if they allowed it.
	ask string
}

var script = []beat{
	{role: "you", text: "add a --json flag to the report command", status: "thinking", working: true, tokens: 180, file: -1},
	{role: "agent", text: "Reading the command first so the flag lands next to the ones already there.", status: "reading", working: true, tokens: 640, file: 1},
	{role: "tool", text: "read_file cmd/report.go -- 128 lines", status: "reading", working: true, tokens: 1480, file: 1},
	{
		role: "agent", status: "editing", working: true, tokens: 2210, file: 1,
		text: "It writes the table straight to stdout, so the flag has to pick the encoder rather than post-process the text.",
	},
	{role: "tool", text: "edit_file cmd/report.go", status: "editing", working: true, tokens: 3050, file: 1, patch: true},
	{
		ask:    "Run `go test ./...` in the working tree?",
		status: "waiting for you", file: -1, patch: true,
	},
	{role: "tool", text: "bash go test ./...", status: "running tests", working: true, tokens: 3900, file: 2, patch: true, results: true},
	{
		role: "agent", status: "ready", tokens: 4260, file: 2, patch: true, results: true,
		text: "Tests pass. --json prints the same rows through encoding/json, and the table stays the default.",
	},
}

var files = []string{"cmd/report.go", "report/table.go", "report/json.go", "report/report_test.go"}

var patch = []string{
	"@@ -18,6 +18,10 @@ func newReportCmd() *cobra.Command {",
	" \tcmd.Flags().StringVar(&out, \"out\", \"\", \"write to a file\")",
	"+\tcmd.Flags().BoolVar(&asJSON, \"json\", false, \"write the report as JSON\")",
	" ",
	" \treturn cmd",
	"@@ -41,7 +45,11 @@ func run(cmd *cobra.Command, args []string) error {",
	"-\treturn table.Write(os.Stdout, rows)",
	"+\tif asJSON {",
	"+\t\treturn json.NewEncoder(os.Stdout).Encode(rows)",
	"+\t}",
	"+\treturn table.Write(os.Stdout, rows)",
}

var results = []string{
	"report|12|ok",
	"report/table|8|ok",
	"report/json|4|ok",
}

type model struct {
	view *tml.View

	at      int
	entries []string
	state   beat
	tokens  int
	frame   int
	offset  int
	prompt  string

	// patchAt and resultsAt are how many turns had been said when each card arrived, which is what puts the cards back in
	patchAt, resultsAt int

	asking  bool
	ask     string
	denied  bool
	waiting bool

	width, height int
	quitting      bool
}

type tick time.Time

func ticker() tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(t time.Time) tea.Msg { return tick(t) })
}

func newModel() (*model, error) {
	ui, err := fs.Sub(uiFS, "ui")
	if err != nil {
		return nil, err
	}

	// The transcript and the diff are this program's own widgets, bound by name and used in the template exactly like a
	widgets := widget.NewRegistry().
		BindFactory("Transcript", widget.NewFactory([]string{"entries"}, newTranscript)).
		BindFactory("Diff", widget.NewFactory([]string{"lines"}, newDiff))

	view, err := tml.Load(ui, "app.tml", tml.Options{Widgets: widgets, Dark: true})
	if err != nil {
		return nil, err
	}
	return &model{
		view:      view,
		state:     beat{status: "ready", file: 0},
		entries:   []string{"note|a scripted session: nothing here talks to a model"},
		patchAt:   -1,
		resultsAt: -1,
		width:     96,
		height:    30,
	}, nil
}

func (m *model) Init() tea.Cmd { return ticker() }

// step runs the next beat. A beat that asks for permission stops the script where it is until the answer arrives.
func (m *model) step() {
	if m.waiting || m.at >= len(script) {
		return
	}
	next := script[m.at]
	m.at++

	if next.ask != "" {
		m.asking, m.waiting, m.ask = true, true, next.ask
		m.state.status, m.state.working = next.status, false
		return
	}
	m.apply(next)
}

func (m *model) apply(next beat) {
	if next.role != "" {
		m.entries = append(m.entries, next.role+"|"+next.text)
	}
	// A card belongs where it happened, so the turn count when it leading appeared is the point the transcript is cut at.
	if next.patch && m.patchAt < 0 {
		m.patchAt = len(m.entries)
	}
	if next.results && m.resultsAt < 0 {
		m.resultsAt = len(m.entries)
	}
	m.tokens = next.tokens
	m.state = next
	m.follow()
}

// split cuts the transcript where the cards sit in it. A card that has not arrived yet takes no cut, so everything
func (m *model) split() (before, between, after []string) {
	patchAt, resultsAt := len(m.entries), len(m.entries)
	if m.patchAt >= 0 {
		patchAt = min(m.patchAt, len(m.entries))
	}
	if m.resultsAt >= 0 {
		resultsAt = max(patchAt, min(m.resultsAt, len(m.entries)))
	}
	return m.entries[:patchAt], m.entries[patchAt:resultsAt], m.entries[resultsAt:]
}

// tail is further down than any transcript reaches. How far the bottom actually is depends on how the text wrapped at
const tail = 1 << 20

// follow pins the session to the bottom, which is where a transcript anyone is reading actually is.
func (m *model) follow() { m.offset = tail }

// scroll moves the session by delta. While the host is following the tail its own offset is a number past the end, so
func (m *model) scroll(delta int) int {
	from, limit := m.offset, tail
	if target, ok := m.view.UI().Target("session"); ok {
		limit = target.Scroll.MaxY
		if m.offset >= limit {
			from = target.Scroll.Y
		}
	}
	return max(0, min(from+delta, limit))
}

func (m *model) answer(allowed bool) {
	m.asking, m.waiting = false, false
	if allowed {
		m.entries = append(m.entries, "note|allowed: go test ./...")
		m.step()
		return
	}
	m.denied = true
	m.entries = append(m.entries, "note|denied -- the run was skipped and the session stopped here")
	m.state.status, m.state.working = "stopped", false
	m.at = len(script)
	m.follow()
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tick:
		m.frame++
		return m, ticker()
	case tea.KeyPressMsg:
		if cmd, handled := m.hotkey(msg); handled {
			return m, cmd
		}
	}

	for _, event := range m.view.UI().Update(msg) {
		switch event.Kind {
		case tml.Activated:
			m.act(event.Action, event.ID, event.Y)
		case tml.Scrolled:
			if event.ID == "session" {
				m.offset = m.scroll(event.Delta)
			}
		}
	}
	if m.quitting {
		return m, tea.Quit
	}
	return m, nil
}

func (m *model) act(action, id string, row int) {
	switch {
	case action == "send":
		m.send()
	case action == "allow":
		m.answer(true)
	case action == "deny":
		m.answer(false)
	case id == "files" && row >= 0:
		m.state.file = min(row, len(files)-1)
	}
}

// send is the prompt being submitted. The script is what answers, because there is nothing else here to answer.
func (m *model) send() {
	if m.prompt != "" {
		m.entries = append(m.entries, "you|"+m.prompt)
		m.prompt = ""
		m.entries = append(m.entries, "note|the script is what answers here, not a model")
		m.follow()
		return
	}
	m.step()
}

func (m *model) hotkey(msg tea.KeyPressMsg) (tea.Cmd, bool) {
	focused, _ := m.view.UI().Focused()
	switch msg.String() {
	case "ctrl+c":
		return tea.Quit, true
	case "esc":
		if m.asking {
			m.answer(false)
			return nil, true
		}
		return tea.Quit, true
	}
	if m.asking {
		switch msg.String() {
		case "y":
			m.answer(true)
			return nil, true
		case "n":
			m.answer(false)
			return nil, true
		}
	}
	if focused == "prompt" {
		return m.typing(msg)
	}
	switch msg.String() {
	case "q":
		return tea.Quit, true
	case " ", "space":
		m.step()
		return nil, true
	}
	return nil, false
}

func (m *model) typing(msg tea.KeyPressMsg) (tea.Cmd, bool) {
	switch msg.String() {
	case "backspace":
		if m.prompt != "" {
			m.prompt = m.prompt[:len(m.prompt)-1]
		}
		return nil, true
	case "enter":
		m.send()
		return nil, true
	case "tab", "shift+tab", "up", "down":
		return nil, false
	}
	if r := msg.Code; unicode.IsPrint(r) && msg.Mod == 0 {
		m.prompt += string(r)
		return nil, true
	}
	return nil, false
}

func (m *model) View() tea.View {
	view := tea.NewView(m.render())
	view.MouseMode = tea.MouseModeAllMotion
	return view
}

func (m *model) render() string {
	before, between, after := m.split()
	out, err := m.view.Render(tml.Props{
		"status":      sema.StringValue(m.state.status),
		"working":     sema.BoolValue(m.state.working),
		"frame":       sema.StringValue(strconv.Itoa(m.frame)),
		"tokens":      sema.StringValue(strconv.Itoa(m.tokens)),
		"cost":        sema.StringValue(cost(m.tokens)),
		"files":       sema.ListValue(files),
		"file":        sema.StringValue(strconv.Itoa(max(0, m.state.file))),
		"before":      sema.ListValue(before),
		"between":     sema.ListValue(between),
		"after":       sema.ListValue(after),
		"showBetween": sema.BoolValue(len(between) > 0),
		"showAfter":   sema.BoolValue(len(after) > 0),
		"offset":      sema.StringValue(strconv.Itoa(m.offset)),
		"patch":       sema.ListValue(patch),
		"patchFile":   sema.StringValue("cmd/report.go"),
		"showPatch":   sema.BoolValue(m.state.patch),
		"results":     sema.ListValue(results),
		"showResults": sema.BoolValue(m.state.results),
		"prompt":      sema.StringValue(m.prompt),
		"cursor":      sema.StringValue(strconv.Itoa(len(m.prompt))),
		"sendVariant": sema.StringValue(sendVariant(m.waiting)),
		"asking":      sema.BoolValue(m.asking),
		"ask":         sema.StringValue(m.ask),
	}, m.width, m.height)
	if err != nil {
		return "tml: " + err.Error()
	}
	return out
}

// cost is the going rate for a model that does not exist, so the meter has something to show.
func cost(tokens int) string {
	return fmt.Sprintf("$%.2f", float64(tokens)*0.000003)
}

func sendVariant(waiting bool) string {
	if waiting {
		return "default"
	}
	return "primary"
}

func main() {
	frame := flag.Bool("frame", false, "render one frame to stdout and exit")
	width := flag.Int("width", 96, "viewport width for -frame")
	height := flag.Int("height", 30, "viewport height for -frame")
	steps := flag.Int("steps", 0, "run this many script beats before starting")
	focus := flag.String("focus", "", "id of the control to put the keyboard on")
	answer := flag.String("answer", "", "allow or deny the permission the script stopped on")
	flag.Parse()

	m, err := newModel()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	// The flags set the starting state either way round: a still frame and a terminal session are the same program, and a
	if *frame {
		m.width, m.height = *width, *height
	}
	for range *steps {
		m.step()
	}
	switch *answer {
	case "allow":
		m.answer(true)
	case "deny":
		m.answer(false)
	case "":
	default:
		fmt.Fprintf(os.Stderr, "error: -answer takes allow or deny, not %q\n", *answer)
		os.Exit(1)
	}
	if *focus != "" {
		// Focus resolves against a frame's controls, so there has to be a frame before there is anything to name.
		m.render()
		if !m.view.UI().Focus(*focus) {
			fmt.Fprintf(os.Stderr, "error: no control with id %q on this frame\n", *focus)
			os.Exit(1)
		}
	}
	if *frame {
		fmt.Println(m.render())
		return
	}
	// Nothing here wires an inspector. Load adopted the view and opened the socket, and Run gave the protocol a program
	if _, err := tml.Run(m); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
