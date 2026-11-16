// Command agent is a mock coding agent: the shape of an AI harness, with a script where the model would be. It exists
package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"strconv"

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
	offset  int
	prompt  string
	cursor  int

	// patchAt and resultsAt are how many turns had been said when each card arrived.
	patchAt, resultsAt int

	asking  bool
	ask     string
	denied  bool
	waiting bool

	width, height int
	quitting      bool
}

func newModel() (*model, error) {
	ui, err := fs.Sub(uiFS, "ui")
	if err != nil {
		return nil, err
	}

	// The transcript and the diff are this program's own widgets.
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

// tail is further down than any transcript reaches.
const tail = 1 << 20

// follow pins the session to the bottom, which is where a transcript anyone is reading is.
func (m *model) follow() { m.offset = tail }

// scroll moves the session by delta. While the host is following the tail its own offset is a number past the end.
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

// Update takes a message from the program. A key it does not consume goes to the focus ring and comes back as an
// event. The program steps the spinner by itself, so there is no clock here.
func (m *model) Update(p *tml.Program, msg tml.Msg) bool {
	switch msg := msg.(type) {
	case tml.Key:
		handled := m.hotkey(msg)
		if m.quitting {
			p.Quit()
		}
		return handled
	case tml.Event:
		switch msg.Kind {
		case tml.Changed:
			// The prompt edits itself and holds nothing: the value and the caret come back here.
			if msg.ID == "prompt" {
				m.prompt, m.cursor = msg.Value, msg.Cursor
			}
		case tml.Activated:
			m.act(msg.Action, msg.ID, msg.Y)
		case tml.Scrolled:
			if msg.ID == "session" {
				m.offset = m.scroll(msg.Delta)
			}
		}
	}
	return false
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
		m.prompt, m.cursor = "", 0
		m.entries = append(m.entries, "note|the script is what answers here, not a model")
		m.follow()
		return
	}
	m.step()
}

// hotkey handles the keys the host keeps for itself, and reports whether it consumed the key. Typing is the prompt's
// own while it has the focus.
func (m *model) hotkey(key tml.Key) bool {
	focused, _ := m.view.UI().Focused()
	switch key.String() {
	case "ctrl+c":
		m.quitting = true
		return true
	case "esc":
		if m.asking {
			m.answer(false)
			return true
		}
		m.quitting = true
		return true
	}
	if m.asking {
		switch key.String() {
		case "y":
			m.answer(true)
			return true
		case "n":
			m.answer(false)
			return true
		}
	}
	if focused == "prompt" {
		if key.String() == "enter" {
			m.send()
			return true
		}
		return false
	}
	switch key.String() {
	case "q":
		m.quitting = true
		return true
	case "space":
		m.step()
		return true
	}
	return false
}

// render draws one frame as text, which is how -frame and the golden tests reach the UI without a terminal.
func (m *model) render() string {
	out, err := m.view.Render(m.Props(), m.width, m.height)
	if err != nil {
		return "tml: " + err.Error()
	}
	return out
}

// Props is what the document is drawn from.
func (m *model) Props() tml.Props {
	before, between, after := m.split()
	return tml.Props{
		"status":      sema.StringValue(m.state.status),
		"working":     sema.BoolValue(m.state.working),
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
		"cursor":      sema.StringValue(strconv.Itoa(m.cursor)),
		"sendVariant": sema.StringValue(sendVariant(m.waiting)),
		"asking":      sema.BoolValue(m.asking),
		"ask":         sema.StringValue(m.ask),
	}
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

	// The flags set the starting state either way round: a still frame and a terminal session are the same program.
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
	// Nothing here wires an inspector. Load adopted the view and opened the socket, and Run gave the protocol a program.
	if err := tml.Run(m.view, m); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
