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
