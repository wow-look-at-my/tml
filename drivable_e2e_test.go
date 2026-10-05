package tml_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml"
)

// goCommand is the go command that built this test. A toolchain that runs from one packed binary reports a GOROOT
// with no bin/go inside it. Then the go on PATH is the one to use.
func goCommand(t *testing.T) string {
	t.Helper()
	if bin := filepath.Join(runtime.GOROOT(), "bin", "go"); isFile(bin) {
		return bin
	}
	bin, err := exec.LookPath("go")
	require.NoError(t, err, "no go command under GOROOT %s or on PATH", runtime.GOROOT())
	return bin
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

// buildUndrivable builds the fixture program. The child keeps only the module flags of the build that runs this
// test. This is because the org pins in go.mod are placeholders that only those flags resolve. It never prompts
// for git credentials, and a build that does not finish fails the test with its output instead of hanging the
// run.
func buildUndrivable(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "undrivable")
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	defer cancel()
	build := exec.CommandContext(ctx, goCommand(t), "build", "-o", bin, "./testdata/undrivable")
	var flags []string
	for _, flag := range strings.Fields(os.Getenv("GOFLAGS")) {
		if strings.HasPrefix(flag, "-mod=") || strings.HasPrefix(flag, "-modfile=") {
			flags = append(flags, flag)
		}
	}
	build.Env = append(os.Environ(), "GOFLAGS="+strings.Join(flags, " "), "GIT_TERMINAL_PROMPT=0")
	out, err := build.CombinedOutput()
	require.NoError(t, err, "building the fixture with GOFLAGS=%q: %s", strings.Join(flags, " "), out)
	return bin
}

// A real program, built the way this library cannot reach, does not stay up. The unit tests drive the guard's decision
func TestARealUndrivableProgramDoesNotStayUp(t *testing.T) {
	bin := buildUndrivable(t)

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

	// Any crash would satisfy the exit code. What has to be true is that it died of THIS.
	said := stderr.String()
	assert.Contains(t, said, "the inspector cannot drive it", "the program said why it stopped")
	assert.Contains(t, said, "tml.NewProgram", "the program named the fix")
	assert.Contains(t, said, "tea.NewProgram", "the program named what it did instead")
}

// The same program is readable for the whole of its short life. Reading is not what the guard is about, and a guard
func TestAnUndrivableProgramIsReadableWhileItLives(t *testing.T) {
	bin := buildUndrivable(t)

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
