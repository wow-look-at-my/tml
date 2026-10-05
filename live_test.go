package tml_test

import (
	"bytes"
	"io"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/fstest"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/vt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml"
	"github.com/wow-look-at-my/tml/sema"
)

const liveBlock = `<?xml version="1.1" encoding="UTF-8"?>
<Component xmlns="urn:tml:v1" name="Block">
	<Property name="head" type="string" required="true"/>
	<Property name="rows" type="string[]" default=""/>
	<Template>
		<Stack>
			<Text fg="#ff0000" bold="true">{head}</Text>
			<For each="{rows}" as="row"><Text>  {row}</Text></For>
		</Stack>
	</Template>
</Component>
`

// syncBuffer is the terminal a region writes to. The program and the region's own goroutine both write to it.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

const (
	liveWidth  = 40
	liveHeight = 12
)

// startLive runs a region over the test block on a terminal the test can replay. The env decides the colour profile.
func startLive(t *testing.T, props func() tml.Props, env []string, onInterrupt func()) (*tml.Live, *syncBuffer) {
	t.Helper()
	view, err := tml.Load(fstest.MapFS{"block.tml": &fstest.MapFile{Data: []byte(liveBlock)}}, "block.tml", tml.Options{})
	require.NoError(t, err)
	out := &syncBuffer{}
	live := tml.NewLive(view, props, tml.LiveOptions{
		Interval:    5 * time.Millisecond,
		Output:      out,
		OnInterrupt: onInterrupt,
		Program:     []tea.ProgramOption{tea.WithWindowSize(liveWidth, liveHeight), tea.WithEnvironment(env)},
	})
	require.NoError(t, live.Start())
	t.Cleanup(func() { _ = live.Stop() })
	return live, out
}

// colourEnv is a truecolor terminal. TTY_FORCE makes the test's buffer count as one.
var colourEnv = []string{"TERM=xterm-256color", "COLORTERM=truecolor", "TTY_FORCE=1"}

// blockProps is a header and one row per call count, so a later frame differs from an earlier one.
func blockProps() func() tml.Props {
	var calls atomic.Int64
	return func() tml.Props {
		n := calls.Add(1)
		return tml.Props{
			"head": sema.StringValue("downloads: " + strconv.FormatInt(n, 10)),
			"rows": sema.ListValue([]string{"alpha", "beta"}),
		}
	}
}

// screenOf replays what the region wrote through a terminal emulator. It returns the lines a user scrolls back through,
// then the lines on screen, blank lines at the end dropped.
func screenOf(t *testing.T, written string) []string {
	t.Helper()
	term := vt.NewEmulator(liveWidth, liveHeight)
	// The emulator answers the program's queries on a pipe. Nobody reads that pipe unless a goroutine does.
	go func() { _, _ = io.Copy(io.Discard, term) }()
	_, err := term.WriteString(written)
	require.NoError(t, err)
	var lines []string
	for _, line := range term.Scrollback().Lines() {
		lines = append(lines, strings.TrimRight(line.String(), " "))
	}
	for _, line := range strings.Split(term.String(), "\n") {
		lines = append(lines, strings.TrimRight(line, " "))
	}
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	require.NoError(t, term.Close())
	return lines
}

// waitDrawn blocks until the region has painted a frame that holds want.
func waitDrawn(t *testing.T, out *syncBuffer, want string) {
	t.Helper()
	require.Eventually(t, func() bool { return strings.Contains(ansi.Strip(out.String()), want) },
		5*time.Second, 5*time.Millisecond, "the region never drew %q", want)
}

// The region is live while it runs. Every line printed goes above it one time and in order. Stop takes the region off
// the screen, so what is left is the log alone.
func TestLivePrintsAboveTheRegionAndLeavesOnlyTheLog(t *testing.T) {
	t.Serial()
	exclusive(t)
	live, out := startLive(t, blockProps(), colourEnv, nil)

	waitDrawn(t, out, "downloads: ")
	waitDrawn(t, out, "  alpha")
	live.Println("one")
	live.Printf("two %d", 2)
	_, _ = live.Write([]byte("thr"))
	_, _ = live.Write([]byte("ee\nfour\r\n"))
	waitDrawn(t, out, "four")
	_, _ = live.Write([]byte("five, with no newline"))
	live.Println("")
	live.Println("six")

	require.NoError(t, live.Stop())
	assert.Equal(t, []string{"one", "two 2", "three", "four", "", "six", "five, with no newline"}, screenOf(t, out.String()),
		"the log is on the screen in order, and the region is gone")
}

// Many lines from many goroutines all arrive, and each goroutine's lines keep their order.
func TestLiveKeepsEveryLineFromConcurrentWriters(t *testing.T) {
	t.Serial()
	exclusive(t)
	live, out := startLive(t, blockProps(), colourEnv, nil)

	var wg sync.WaitGroup
	for w := range 4 {
		wg.Go(func() {
			for i := range 25 {
				live.Printf("w%d-%02d", w, i)
			}
		})
	}
	wg.Wait()
	require.NoError(t, live.Stop())

	lines := strings.Split(strings.TrimSpace(ansi.Strip(out.String())), "\n")
	next := map[string]int{}
	seen := 0
	for _, line := range lines {
		match := regexp.MustCompile(`w(\d)-(\d\d)`).FindStringSubmatch(line)
		if match == nil {
			continue
		}
		n, _ := strconv.Atoi(match[2])
		assert.Equal(t, next[match[1]], n, "writer %s printed out of order", match[1])
		next[match[1]] = n + 1
		seen++
	}
	assert.Equal(t, 100, seen, "every line printed exactly one time")
}

// Colour comes from the theme and the terminal decides how much of it survives. NO_COLOR removes all of it.
func TestLiveColourFollowsTheTerminal(t *testing.T) {
	t.Serial()
	exclusive(t)

	coloured, out := startLive(t, blockProps(), colourEnv, nil)
	waitDrawn(t, out, "alpha")
	require.NoError(t, coloured.Stop())
	assert.True(t, hasColour(out.String()), "a colour terminal gets the header's colour")

	plain, out := startLive(t, blockProps(), append(colourEnv, "NO_COLOR=1"), nil)
	waitDrawn(t, out, "alpha")
	require.NoError(t, plain.Stop())
	assert.False(t, hasColour(out.String()), "NO_COLOR removes every colour")
	assert.Contains(t, ansi.Strip(out.String()), "downloads: ", "the text itself still draws")
}

// hasColour reports whether any SGR sequence sets a foreground or background colour.
func hasColour(s string) bool {
	for _, match := range regexp.MustCompile(`\x1b\[([0-9;:]*)m`).FindAllStringSubmatch(s, -1) {
		for _, param := range strings.FieldsFunc(match[1], func(r rune) bool { return r == ';' || r == ':' }) {
			n, _ := strconv.Atoi(param)
			if (n >= 30 && n <= 48) || (n >= 90 && n <= 107) {
				return true
			}
		}
	}
	return false
}

// An interrupt is an orderly end: the pending lines print, the region comes off the screen, and then the host hears.
func TestLiveInterruptPrintsWhatIsPendingThenTellsTheHost(t *testing.T) {
	t.Serial()
	exclusive(t)
	heard := make(chan struct{})
	live, out := startLive(t, blockProps(), colourEnv, func() { close(heard) })
	waitDrawn(t, out, "alpha")

	live.Println("last words")
	tml.InterruptLive(live)

	select {
	case <-heard:
	case <-time.After(5 * time.Second):
		t.Fatal("OnInterrupt never ran")
	}
	require.ErrorIs(t, live.Wait(), tml.ErrInterrupted)
	require.ErrorIs(t, live.Stop(), tml.ErrInterrupted, "Stop after the end reports the same end")
	assert.Equal(t, []string{"last words"}, screenOf(t, out.String()))
}

// A frame that cannot render ends the region with the reason, rather than leave the last good frame up as if it were live.
func TestLiveEndsOnARenderFailure(t *testing.T) {
	t.Serial()
	exclusive(t)
	live, _ := startLive(t, func() tml.Props { return tml.Props{} }, colourEnv, nil)

	select {
	case <-live.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("a region that cannot render kept running")
	}
	assert.ErrorContains(t, live.Wait(), "head", "the error names the property that was missing")
}

// A region that never started still owes its lines to the terminal, and a stopped region writes straight through.
func TestLiveLinesOutsideTheRegion(t *testing.T) {
	t.Serial()
	exclusive(t)
	view, err := tml.Load(fstest.MapFS{"block.tml": &fstest.MapFile{Data: []byte(liveBlock)}}, "block.tml", tml.Options{})
	require.NoError(t, err)
	out := &syncBuffer{}
	live := tml.NewLive(view, blockProps(), tml.LiveOptions{Output: out})

	live.Println("before")
	_, _ = live.Write([]byte("partial"))
	require.NoError(t, live.Stop())
	live.Println("after")
	_, _ = live.Write([]byte("raw\n"))

	assert.Equal(t, "before\npartial\nafter\nraw\n", out.String())
	assert.Error(t, live.Start(), "a stopped region does not start again")
}

// A region starts one time.
func TestLiveStartsOneTime(t *testing.T) {
	t.Serial()
	exclusive(t)
	live, _ := startLive(t, blockProps(), colourEnv, nil)
	assert.Error(t, live.Start())
	live.Refresh()
	require.NoError(t, live.Stop())
	require.NoError(t, live.Stop(), "a second Stop is harmless")
}

// A fitted frame is as tall as its content, not as tall as the viewport it may use.
func TestRenderFitTakesOnlyTheRowsItUses(t *testing.T) {
	view, err := tml.Load(fstest.MapFS{"block.tml": &fstest.MapFile{Data: []byte(liveBlock)}}, "block.tml", tml.Options{})
	require.NoError(t, err)
	props := tml.Props{"head": sema.StringValue("h"), "rows": sema.ListValue([]string{"a", "b"})}

	out, err := view.RenderFit(props, 20, 50)
	require.NoError(t, err)
	assert.Equal(t, 3, strings.Count(out, "\n")+1, "a header and two rows")

	out, err = view.RenderFit(props, 20, 2)
	require.NoError(t, err)
	assert.Equal(t, 2, strings.Count(out, "\n")+1, "capped at the height it was allowed")

	full, err := view.Render(props, 20, 10)
	require.NoError(t, err)
	assert.Equal(t, 10, strings.Count(full, "\n")+1, "Render still fills its viewport")
}
