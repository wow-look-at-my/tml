package tml_test

import (
	"os"
	"testing"

	"github.com/wow-look-at-my/tml"
)

// TestMain puts this run's sockets somewhere of their own.
//
// Every Load serves, including the several hundred in this package, and without
// this they would land in the socket directory of whoever is running the tests
// and show up in their `tml list`. It moves the sockets; it does not turn
// them off, which is the whole property under test here.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "tml-sockets")
	if err != nil {
		panic(err)
	}
	if err := os.Setenv(tml.DirEnv, dir); err != nil {
		panic(err)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
