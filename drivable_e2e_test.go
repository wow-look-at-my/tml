package tml_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml"
)

// A real program, built the way this library cannot reach, does not stay up. The unit tests drive the guard's decision
func TestARealUndrivableProgramDoesNotStayUp(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "undrivable")
	build := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"),
		"build", "-o", bin, "./testdata/undrivable")
	build.Env = append(os.Environ(), "GOFLAGS=")
	out, err := build.CombinedOutput()
	require.NoError(t, err, "building the fixture: %s", out)

	// Its socket goes in a directory of this run's own, the same as every other program these tests start.
	run := exec.Command(bin)
	run.Env = append(os.Environ(), tml.DirEnv+"="+t.TempDir())
	var stderr bytes.Buffer
	run.Stderr = &stderr

	done := make(chan error, 1)
	require.NoError(t, run.Start())
	go func() { done <- run.Wait() }()

	select {
	case err := <-done:
		require.Error(t, err, "a program nothing can drive exited 0")
	case <-time.After(tml.DriveGrace + 20*time.Second):
		_ = run.Process.Kill()
		t.Fatal("a program nothing can drive was still running well past the grace window")
	}

	// Any crash would satisfy the exit code. What has to be true is that it died of THIS, and that whoever reads the
	said := stderr.String()
	assert.Contains(t, said, "the inspector cannot drive it", "the program said why it stopped")
	assert.Contains(t, said, "tml.NewProgram", "the program named the fix")
	assert.Contains(t, said, "tea.NewProgram", "the program named what it did instead")
}

// The same program is readable for the whole of its short life. Reading is not what the guard is about, and a guard
func TestAnUndrivableProgramIsReadableWhileItLives(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "undrivable")
	build := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"),
		"build", "-o", bin, "./testdata/undrivable")
	build.Env = append(os.Environ(), "GOFLAGS=")
	out, err := build.CombinedOutput()
	require.NoError(t, err, "building the fixture: %s", out)

	dir := t.TempDir()
	run := exec.Command(bin)
	run.Env = append(os.Environ(), tml.DirEnv+"="+dir)
	require.NoError(t, run.Start())
	t.Cleanup(func() { _ = run.Process.Kill() })

	// The socket appears without the program being asked for it.
	var socket string
	require.Eventually(t, func() bool {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return false
		}
		for _, entry := range entries {
			if filepath.Ext(entry.Name()) == ".sock" {
				socket = filepath.Join(dir, entry.Name())
				return true
			}
		}
		return false
	}, 10*time.Second, 50*time.Millisecond, "a program that set nothing still served a socket")

	assert.Equal(t, dir, filepath.Dir(socket))
}
