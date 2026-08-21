package tml

import (
	"flag"
	"fmt"
	"os"
	"sync"
	"time"
)

// DriveGrace is how long a program may paint before it has to be drivable.
//
// A program is readable the moment it loads, so the window is only about the
// other half: the inspector delivers a keystroke by sending a message to a
// running *tea.Program, and that program does not exist until the host builds
// it. tml.NewProgram hands it over. A host that starts within this window is
// never noticed.
const DriveGrace = 5 * time.Second

// drivable is the guard on the one hole this library cannot close by
// construction.
//
// tea.NewProgram builds a program tml cannot reach, and Go offers no way to
// intercept another package's constructor -- so a host can build one, render
// through a View, and leave a program running that the inspector can read and
// cannot drive. That is a program the debugger only half works against, and
// half is the state this whole mechanism exists to remove.
//
// So it is not tolerated. A view still painting after the grace window with
// nothing able to drive it takes the program down, naming the one identifier
// that fixes it. Under Bubble Tea the panic unwinds through its own recover,
// which restores the terminal and returns the error to the host -- the same
// exit any other fatal misconfiguration gets.
type drivable struct {
	mu    sync.Mutex
	first time.Time
	seen  int
}

var drives drivable

// painted is called once per frame, by the View that painted it.
func (d *drivable) painted(driven bool) {
	if driven || underTest() {
		return
	}
	d.check()
}

// check is painted without its two exemptions, so a test can reach the failure
// without building a program and waiting out the window.
func (d *drivable) check() {
	d.mu.Lock()
	if d.first.IsZero() {
		d.first = time.Now()
	}
	d.seen++
	seen, since := d.seen, time.Since(d.first)
	d.mu.Unlock()

	// One frame is a one-shot render -- `tml render`, a golden, a screenshot --
	// and it is over before anything could have driven it. What this catches is
	// the other shape: a view still painting minutes later with no way in.
	if seen < 2 || since < DriveGrace {
		return
	}
	fmt.Fprintf(os.Stderr, "\ntml: this program has been drawing for %s and the inspector cannot drive it.\n"+
		"Build it with tml.NewProgram (or tml.Run) instead of tea.NewProgram:\n"+
		"\n    program, err := tml.NewProgram(model)\n\n"+
		"tea.NewProgram returns a program this library never sees, so op=key, op=click\n"+
		"and op=restyle have nothing to send to. Reading works either way; driving does\n"+
		"not, and a program the debugger only half works against is not one this library\n"+
		"will keep running.\n", since.Truncate(time.Second))
	panic("tml: no way to drive this program; build it with tml.NewProgram")
}

// reset forgets what has been painted. Test-only, and it exists because the
// guard is process-wide for the same reason the inspector is.
func (d *drivable) reset() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.first = time.Time{}
	d.seen = 0
}

// underTest reports whether this binary was built by `go test`.
//
// It is not an off switch: the flag exists only in a test binary, which nothing
// a user ships is. It is here because a test renders a view directly, with no
// program to drive it and no intention of having one, and taking the test
// binary down over that would say nothing about any program anybody runs.
func underTest() bool { return flag.Lookup("test.v") != nil }
