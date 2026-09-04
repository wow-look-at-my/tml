package tml_test

import (
	"os"
	"testing"

	"github.com/wow-look-at-my/tml"
)

// TestMain puts this run's sockets somewhere of their own. Every Load serves, including the several many in this
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
