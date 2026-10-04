package tml_test

import (
	"testing"

	"github.com/wow-look-at-my/tml"
)

// exclusive gives a test a clean inspector and a clean drivable guard.
func exclusive(t *testing.T) {
	t.Helper()
	tml.ResetInspection()
	tml.ResetDrivable()
}
