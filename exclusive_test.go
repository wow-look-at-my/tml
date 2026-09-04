package tml_test

import (
	"testing"

	"github.com/wow-look-at-my/tml"
)

// exclusive gives a test a clean inspector and a clean drivable guard. Both are process-wide by design -- every Load
// adopts into the same inspector -- so a test that asserts on either starts by putting it back to nothing.
func exclusive(t *testing.T) {
	t.Helper()
	tml.ResetInspection()
	tml.ResetDrivable()
}
