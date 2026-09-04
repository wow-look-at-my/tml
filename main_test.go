package tml_test

import (
	"flag"
	"os"
	"testing"

	"github.com/wow-look-at-my/tml"
)

// TestMain puts this run's sockets somewhere of their own, and runs this package's tests serially.
//
// The inspector is process-wide on purpose: every Load adopts into it, and opening the socket is a latch the leading
// Load trips. Tests in flight together therefore share an inspector, and the later Load attaches its own view and takes
// the socket the earlier test is still asserting about. Serialising here covers every test in the package, including
// the ones that only load a view and never mention the inspector.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "tml-sockets")
	if err != nil {
		panic(err)
	}
	if err := os.Setenv(tml.DirEnv, dir); err != nil {
		panic(err)
	}
	// Parsing here rather than leaving it to m.Run is what makes the value stick: m.Run parses only if nobody has.
	flag.Parse()
	if err := flag.Set("test.parallel", "1"); err != nil {
		panic(err)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
