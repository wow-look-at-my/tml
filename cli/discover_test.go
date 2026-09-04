package cli

import (
	"bytes"
	"flag"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml"
)

// TestMain gives this run its own socket directory, and runs the package's tests serially. An await asks the
// process-wide inspector what is on screen, so a concurrent test that loads its own view answers the waiting test's
// question about a frame it never painted.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "tml-cli-sockets")
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

func listenSock(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	ln, err := net.Listen("unix", path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	return path
}

func TestResolveSocketPrefersTheFlag(t *testing.T) {
	path := filepath.Join(t.TempDir(), "given.sock")
	got, err := resolveSocket(path)
	require.NoError(t, err)
	assert.Equal(t, path, got)
}

func TestResolveSocketPrefersTheEnv(t *testing.T) {
	path := filepath.Join(t.TempDir(), "env.sock")
	t.Setenv(tml.SocketEnv, path)
	got, err := resolveSocket("")
	require.NoError(t, err)
	assert.Equal(t, path, got)
}

func TestResolveSocketWithNoProgram(t *testing.T) {
	t.Setenv(tml.SocketEnv, "")
	t.Setenv(tml.DirEnv, t.TempDir())
	_, err := resolveSocket("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no TML program is running")
	assert.Contains(t, err.Error(), "--socket")
	assert.Contains(t, err.Error(), tml.SocketEnv)
}

func TestResolveSocketFindsTheOneLiveProgram(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(tml.SocketEnv, "")
	t.Setenv(tml.DirEnv, dir)
	path := listenSock(t, dir, "42.sock")

	got, err := resolveSocket("")
	require.NoError(t, err)
	assert.Equal(t, path, got)
}

func TestResolveSocketRefusesWhenSeveralProgramsAreRunning(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(tml.SocketEnv, "")
	t.Setenv(tml.DirEnv, dir)
	listenSock(t, dir, "1.sock")
	listenSock(t, dir, "2.sock")

	_, err := resolveSocket("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "2 TML programs")
	assert.Contains(t, err.Error(), "--socket")
	assert.Contains(t, err.Error(), "1.sock")
	assert.Contains(t, err.Error(), "2.sock")
}

func TestLiveProgramsIgnoresAStaleSocket(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "9.sock"), []byte("no"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0o600))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "other"), 0o700))
	live, err := livePrograms(dir)
	require.NoError(t, err)
	assert.Empty(t, live)
}

func TestLiveProgramsSkipsAMissingDirectory(t *testing.T) {
	live, err := livePrograms(filepath.Join(t.TempDir(), "nope"))
	require.NoError(t, err)
	assert.Empty(t, live)
}

func TestLiveProgramsNamesADirectoryItCannotRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "not-a-dir")
	require.NoError(t, os.WriteFile(path, []byte("x"), 0o600))
	_, err := livePrograms(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot read")
}

func TestListReportsLivePrograms(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(tml.DirEnv, dir)
	listenSock(t, dir, "7.sock")

	var out bytes.Buffer
	cmd := newListCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SilenceUsage = true
	require.NoError(t, cmd.Execute())
	assert.Contains(t, out.String(), "pid 7")
	assert.Contains(t, out.String(), "7.sock")
}

func TestListFailsWhenNothingIsRunning(t *testing.T) {
	t.Setenv(tml.DirEnv, t.TempDir())

	var out bytes.Buffer
	cmd := newListCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SilenceUsage = true
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no TML program is running")
}
