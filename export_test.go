package tml

import tea "charm.land/bubbletea/v2"

// ResetInspection drops the process's inspector and stops serving. Test-only.
func ResetInspection() {
	inspection.close()
	inspection.mu.Lock()
	defer inspection.mu.Unlock()
	inspection.insp = nil
	inspection.err = nil
}

// Inspect is the process's inspector, for the tests that read and drive it. Test-only, and deliberately.
func Inspect() *Inspector {
	inspection.mu.Lock()
	defer inspection.mu.Unlock()
	return inspection.insp
}

// SweepStale is the stale-socket sweep, reached directly. Test-only.
func SweepStale(dir, own string) { sweepStale(dir, own) }

// ResetDrivable forgets what the drivable guard has seen. Test-only, and it is how a test drives the guard's own clock
func ResetDrivable() { drives.reset() }

// InterruptLive delivers what SIGINT delivers to a running region, without a signal the test process would also get.
func InterruptLive(l *Live) {
	l.mu.Lock()
	program := l.program
	l.mu.Unlock()
	program.Send(tea.InterruptMsg{})
}

// CheckDrivable is the guard's failure, reached directly. Test-only: it is what lets a test see the panic and its
func CheckDrivable() { drives.check() }
