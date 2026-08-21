package tml

// ResetInspection drops the process's inspector and stops serving.
//
// Test-only, and it exists because the inspector is deliberately process-wide:
// without it one test's key handler, override set and frame numbering would
// decide the next test's, and the order they run in would be part of the
// contract.
func ResetInspection() {
	inspection.close()
	inspection.mu.Lock()
	defer inspection.mu.Unlock()
	inspection.insp = nil
	inspection.err = nil
}

// Inspect is the process's inspector, for the tests that read and drive one.
//
// Test-only, and deliberately: the inspector is the debugger's, and the
// debugger reaches it over the protocol. A host that could fetch it in-process
// could drive its own controls through it, open the browser inspector on an
// address nobody asked for, and hold an override the socket cannot clear --
// none of which is what a rendering library is for.
func Inspect() *Inspector {
	inspection.mu.Lock()
	defer inspection.mu.Unlock()
	return inspection.insp
}

// ResetDrivable forgets what the drivable guard has seen. Test-only, and it is
// how a test drives the guard's own clock rather than waiting on it.
func ResetDrivable() { drives.reset() }

// CheckDrivable is the guard's failure, reached directly. Test-only: it is what
// lets a test see the panic and its wording without building a program and
// waiting out the grace window.
func CheckDrivable() { drives.check() }
