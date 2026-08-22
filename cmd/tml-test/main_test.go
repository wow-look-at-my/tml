package main

import (
	"os"
	"testing"

	"github.com/wow-look-at-my/tml"
)

// TestMain puts this run's sockets somewhere of their own.
//
// These tests ask the inspector about a view this process loaded, so the socket
// they talk over is this process's own. Without this it would land in the
// socket directory of whoever is running the tests and show up in their
// `tml-test list`.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "tml-test-cli-sockets")
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
