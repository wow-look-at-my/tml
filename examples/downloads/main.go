// Command downloads is a mock download manager whose progress display is a tml.Live region: a block pinned below the
// log, repainted in place, with finished transfers printed above it.
package main

import (
	"embed"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"math/rand/v2"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wow-look-at-my/tml"
	"github.com/wow-look-at-my/tml/sema"
)

//go:embed ui
var uiFS embed.FS

type state int32

const (
	queued state = iota
	active
	done
	failed
)

// transfer is one download. Workers write it and the painter reads it, so every field that changes is atomic.
type transfer struct {
	name  string
	total int64
	got   atomic.Int64
	state atomic.Int32
	start atomic.Int64
}

func (t *transfer) is(s state) bool { return state(t.state.Load()) == s }

func loadView() (*tml.View, error) {
	ui, err := fs.Sub(uiFS, "ui")
	if err != nil {
		return nil, err
	}
	return tml.Load(ui, "Downloads.tml", tml.Options{Dark: true})
}

// props is one frame of the block: the counts, the step status, one row per active transfer and the TOTAL row.
func props(items []*transfer, status string, now time.Time) tml.Props {
	var nActive, nQueued, nDone, nFailed int
	var got, total int64
	var first time.Time
	var rows []map[string]sema.Value
	for _, t := range items {
		switch state(t.state.Load()) {
		case queued:
			nQueued++
			continue
		case active:
			nActive++
		case done:
			nDone++
		case failed:
			nFailed++
		}
		started := time.Unix(0, t.start.Load())
		if first.IsZero() || started.Before(first) {
			first = started
		}
		got += t.got.Load()
		total += t.total
		if t.is(active) {
			rows = append(rows, row(t.name, "active", t.got.Load(), t.total, now.Sub(started)))
		}
	}
	rows = append(rows, row("TOTAL", "total", got, total, now.Sub(first)))

	head := fmt.Sprintf("downloads: %d active, %d queued, %d done", nActive, nQueued, nDone)
	headStyle := "head"
	if nFailed > 0 {
		head += fmt.Sprintf(", %d failed", nFailed)
		headStyle = "head.failed"
	}
	return tml.Props{
		"head":      sema.StringValue(head),
		"headStyle": sema.StringValue(headStyle),
		"status":    sema.StringValue(status),
		"rows":      sema.RecordListValue(rows),
	}
}

// row is the record the Transfer template draws. A total of zero is an unknown length: no bar and no percentage.
func row(label, st string, got, total int64, elapsed time.Duration) map[string]sema.Value {
	r := map[string]sema.Value{
		"label": sema.StringValue(label),
		"state": sema.StringValue(st),
		"known": sema.BoolValue(total > 0),
		"sizes": sema.StringValue(fmt.Sprintf("%10s / %s", humanBytes(got), sizeOrUnknown(total))),
	}
	if total > 0 {
		fraction := float64(got) / float64(total)
		r["value"] = sema.StringValue(fmt.Sprintf("%.4f", fraction))
		r["percent"] = sema.StringValue(fmt.Sprintf("%3.0f%%", fraction*100))
	}
	if elapsed > 0 && got > 0 {
		speed := float64(got) / elapsed.Seconds()
		r["speed"] = sema.StringValue(humanBytes(int64(speed)) + "/s")
		if total > got {
			r["eta"] = sema.StringValue("ETA " + clock(time.Duration(float64(total-got)/speed*float64(time.Second))))
		}
	}
	return r
}

func sizeOrUnknown(n int64) string {
	if n <= 0 {
		return "?"
	}
	return humanBytes(n)
}

func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for x := n / unit; x >= unit && exp < 4; x /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTP"[exp])
}

func clock(d time.Duration) string {
	total := int(max(0, d.Seconds()))
	if h := total / 3600; h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, (total/60)%60, total%60)
	}
	return fmt.Sprintf("%02d:%02d", total/60, total%60)
}

// sample is a fixed moment of a run, for -frame and the goldens.
func sample(now time.Time) []*transfer {
	specs := []struct {
		name      string
		total     int64
		got       int64
		st        state
		startedAt time.Duration
	}{
		{"ubuntu-24.04-desktop-amd64.iso", 6 << 30, 2 << 30, active, 40 * time.Second},
		{"日本語のファイル名.tar.zst", 300 << 20, 120 << 20, active, 12 * time.Second},
		{"stream.log", 0, 5 << 20, active, 3 * time.Second},
		{"checksums.txt", 4 << 10, 4 << 10, done, 50 * time.Second},
		{"mirror-list.json", 20 << 10, 0, failed, 45 * time.Second},
		{"release-notes.md", 64 << 10, 0, queued, 0},
	}
	items := make([]*transfer, 0, len(specs))
	for _, s := range specs {
		t := &transfer{name: s.name, total: s.total}
		t.got.Store(s.got)
		t.state.Store(int32(s.st))
		t.start.Store(now.Add(-s.startedAt).UnixNano())
		items = append(items, t)
	}
	return items
}

// run moves a queue of fake transfers through workers while the region shows them.
func run() error {
	view, err := loadView()
	if err != nil {
		return err
	}
	items := make([]*transfer, 12)
	for i := range items {
		items[i] = &transfer{name: fmt.Sprintf("part-%02d.bin", i), total: int64(rand.IntN(40)+8) << 20}
	}
	var status atomic.Value
	status.Store("")
	live := tml.NewLive(view, func() tml.Props { return props(items, status.Load().(string), time.Now()) }, tml.LiveOptions{
		OnInterrupt: func() {
			fmt.Println("interrupted")
			os.Exit(130)
		},
	})
	if err := live.Start(); err != nil {
		return err
	}

	queue := make(chan *transfer, len(items))
	for _, t := range items {
		queue <- t
	}
	close(queue)
	var wg sync.WaitGroup
	for w := range 3 {
		wg.Go(func() {
			for t := range queue {
				fetch(live, t, w)
				status.Store(fmt.Sprintf("worker %d finished %s", w, t.name))
			}
		})
	}
	wg.Wait()
	return live.Stop()
}

// fetch pretends to download one file. Every seventh file fails, so the failed state has something to draw.
func fetch(live *tml.Live, t *transfer, worker int) {
	t.start.Store(time.Now().UnixNano())
	t.state.Store(int32(active))
	for t.got.Load() < t.total {
		time.Sleep(30 * time.Millisecond)
		t.got.Store(min(t.total, t.got.Load()+int64(rand.IntN(2<<20))))
		if t.name == "part-06.bin" && t.got.Load() > t.total/2 {
			t.state.Store(int32(failed))
			live.Printf("failed %s: connection reset (worker %d)", t.name, worker)
			return
		}
	}
	t.state.Store(int32(done))
	live.Printf("done   %s  %s", t.name, humanBytes(t.total))
}

func main() {
	frame := flag.Bool("frame", false, "render one frame of a fixed sample to stdout and exit")
	width := flag.Int("width", 100, "width for -frame")
	flag.Parse()

	if *frame {
		view, err := loadView()
		if err == nil {
			var out string
			now := time.Unix(1_800_000_000, 0)
			out, err = view.RenderFit(props(sample(now), "jobs 07/20 JOB-000123  attempt 004/030  status=pending", now), *width, 40)
			fmt.Println(out)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		return
	}
	if err := run(); err != nil && !errors.Is(err, tml.ErrInterrupted) {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
