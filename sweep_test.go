package tml_test

import (
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml"
)

// staleSocket leaves a socket file that nothing listens on, the way a process that exits without Close does.
func staleSocket(t *testing.T, path string) {
	t.Helper()
	ln, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	require.NoError(t, err)
	ln.SetUnlinkOnClose(false)
	require.NoError(t, ln.Close())
}

// A command-line tool loads a view on every run, so each run that exits leaves a socket. The next one to serve
// removes them. A live program's socket, the caller's own and any other file stay.
func TestServingSweepsTheSocketsOfProgramsThatExited(t *testing.T) {
	dir, err := os.MkdirTemp("", "sweep")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(dir) })

	stale := filepath.Join(dir, "101.sock")
	staleSocket(t, stale)
	own := filepath.Join(dir, "102.sock")
	staleSocket(t, own)

	live := filepath.Join(dir, "103.sock")
	ln, err := net.Listen("unix", live)
	require.NoError(t, err)
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	other := filepath.Join(dir, "agent.sock")
	staleSocket(t, other)
	note := filepath.Join(dir, "9.sock.txt")
	require.NoError(t, os.WriteFile(note, nil, 0o600))

	tml.SweepStale(dir, own)

	assert.NoFileExists(t, stale, "a socket nothing listens on goes")
	for _, kept := range []string{own, live, other, note} {
		_, err := os.Lstat(kept)
		assert.NoError(t, err, "%s stays", filepath.Base(kept))
	}
}
