package tml

// ResetInspection drops the process's inspector and stops serving. Test-only, and it exists because the inspector is
func ResetInspection() {
	inspection.close()
	inspection.mu.Lock()
	defer inspection.mu.Unlock()
	inspection.insp = nil
	inspection.err = nil
}

// Inspect is the process's inspector, for the tests that read and drive it. Test-only, and deliberately: the
func Inspect() *Inspector {
	inspection.mu.Lock()
	defer inspection.mu.Unlock()
	return inspection.insp
}

// ResetDrivable forgets what the drivable guard has seen. Test-only, and it is how a test drives the guard's own clock
func ResetDrivable() { drives.reset() }

// CheckDrivable is the guard's failure, reached directly. Test-only: it is what lets a test see the panic and its
func CheckDrivable() { drives.check() }
