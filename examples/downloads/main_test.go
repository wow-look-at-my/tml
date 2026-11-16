package main

import (
	"flag"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml/cell"
)

var update = flag.Bool("update", false, "rewrite the golden files")

const status = "jobs 07/20 JOB-000123  attempt 004/030  status=pending"

func frameAt(t *testing.T, width int) string {
	t.Helper()
	view, err := loadView()
	require.NoError(t, err)
	now := time.Unix(1_800_000_000, 0)
	out, err := view.RenderFit(props(sample(now), status, now), width, 40)
	require.NoError(t, err)
	return out
}

// The block at widths. A wide terminal shows every column. Narrower ones drop the bar first, then the speed, the ETA
// and the sizes. Every row drops the same ones, so the columns stay in line.
func TestTheBlockDropsColumnsAsTheTerminalNarrows(t *testing.T) {
	for _, width := range []int{110, 72, 40} {
		t.Run(strconv.Itoa(width), func(t *testing.T) {
			out := cell.Strip(frameAt(t, width))
			golden(t, "block-"+strconv.Itoa(width), out)
			for _, line := range strings.Split(out, "\n") {
				assert.LessOrEqual(t, cell.Width(line), width, "no line runs past the terminal: %q", line)
			}
		})
	}
}

// The header, the status line and the rows each carry their theme colour, and the failed count turns the header red.
func TestTheBlockIsColoured(t *testing.T) {
	out := frameAt(t, 110)
	assert.Contains(t, out, "\x1b[", "the frame carries styling")
	assert.Contains(t, cell.Strip(out), "1 failed")
}

// A header, a status line, active rows and the TOTAL row, and not one blank row more.
func TestTheBlockIsAsTallAsItsContent(t *testing.T) {
	assert.Len(t, strings.Split(frameAt(t, 110), "\n"), 6)
}

func golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")
	if *update {
		require.NoError(t, os.WriteFile(path, []byte(got), 0o644))
		return
	}
	assert.Equal(t, readGolden(t, path, got), got)
}

// readGolden returns the recorded frame. An empty golden is seeded from this run and then fails, so a broken frame
// cannot bless itself.
func readGolden(t *testing.T, path, got string) string {
	t.Helper()
	info, err := os.Stat(path)
	require.NoError(t, err, "missing golden file; rerun with -update")

	if info.Size() > 0 {
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		return string(data)
	}
	require.NoError(t, os.WriteFile(path, []byte(got), 0o644))
	t.Fatalf("golden %s was empty; seeded it from this run -- re-run to verify, and read the diff before trusting it", path)
	return ""
}
