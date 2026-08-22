package tml

import (
	"fmt"
	"os"
	"path/filepath"
)

// SocketEnv overrides the path this process serves the inspection protocol on.
//
// It is an override and not a switch. A program built on this library serves
// whether or not the variable is set, because a socket somebody has to ask for
// is a socket nobody has: every program would be inspectable in principle and
// not in fact, which is how a test suite ends up reading pane captures.
const SocketEnv = "TML_INSPECT_SOCKET"

// DirEnv overrides the directory the default path is built in. It is what a
// test sets so its programs do not appear in the user's own listing.
const DirEnv = "TML_INSPECT_DIR"

// SocketDir is where a program with no path of its own serves.
//
// One socket per process, named by pid, rather than one fixed name: a fixed
// name is a name the second program on the machine cannot have, and a program
// that cannot open the socket is a program nothing can reach -- the hole this
// exists to close, wearing a different shape.
//
// XDG_RUNTIME_DIR when there is one, because it is per-user, already 0700, and
// cleared when the user logs out. The fallback carries the uid in its name so
// two users on one machine do not land in the same directory.
func SocketDir() string {
	if dir := os.Getenv(DirEnv); dir != "" {
		return dir
	}
	if run := os.Getenv("XDG_RUNTIME_DIR"); run != "" {
		return filepath.Join(run, "tml")
	}
	return filepath.Join(os.TempDir(), fmt.Sprintf("tml-%d", os.Getuid()))
}

// SocketPath is the path this process serves on.
func SocketPath() string {
	if path := os.Getenv(SocketEnv); path != "" {
		return path
	}
	return filepath.Join(SocketDir(), fmt.Sprintf("%d.sock", os.Getpid()))
}

// prepareSocketDir makes the directory readable by this user and nobody else.
//
// The socket carries the right to drive the program, so its directory is the
// boundary. 0700 makes that the same boundary as the user's own shell, which
// is the one they already trust with the terminal the program draws on.
func prepareSocketDir(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("cannot create %s: %w", dir, err)
	}
	// MkdirAll leaves an existing directory's mode alone, and the fallback
	// under TMPDIR is a path anybody can have created first.
	if err := os.Chmod(dir, 0o700); err != nil {
		return fmt.Errorf("cannot restrict %s to this user: %w", dir, err)
	}
	return nil
}
