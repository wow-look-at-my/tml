package term

import (
	"fmt"
	"os"
	"runtime"
	"testing"
)

func TestTagProbe(t *testing.T) {
	reader, _ := NewReader(os.Stdin)
	t.Fatalf("GOOS=%s GOARCH=%s linuxTag=%v reader=%T stdinTTY=%v", runtime.GOOS, runtime.GOARCH, tagLinux, reader, IsTerminal(os.Stdin))
	_ = fmt.Sprint()
}
