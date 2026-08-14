package tml_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/tml"
)

const watchHeader = "<?xml version=\"1.1\" encoding=\"UTF-8\"?>\n"

func component(body string) string {
	return watchHeader + `<Component xmlns="urn:tml:v1" name="App"><Template>` + body + `</Template></Component>`
}

// reloads collects what the watcher delivers, so a test can wait for the next
// result rather than sleeping for a fixed time.
type reloads struct {
	views chan *tml.View
	errs  chan error
}

func newReloads() *reloads {
	return &reloads{views: make(chan *tml.View, 8), errs: make(chan error, 8)}
}

func (r *reloads) onChange(view *tml.View, err error) {
	if err != nil {
		r.errs <- err
		return
	}
	r.views <- view
}

// awaitView keeps applying the edit until a reload arrives.
//
// The watcher takes its baseline fingerprint when its goroutine first runs, and
// a test cannot know when that happens. A single write can therefore land before
// the baseline and be absorbed into it. Re-applying the edit is what makes the
// test independent of that scheduling, without sleeping for a guessed interval.
func (r *reloads) awaitView(t *testing.T, edit func()) *tml.View {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		edit()
		select {
		case view := <-r.views:
			return view
		case err := <-r.errs:
			t.Fatalf("wanted a reloaded view, got error: %v", err)
		case <-time.After(300 * time.Millisecond):
		}
	}
	t.Fatal("timed out waiting for a reload")
	return nil
}

func (r *reloads) awaitError(t *testing.T, edit func()) error {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		edit()
		select {
		case err := <-r.errs:
			return err
		case <-r.views:
			t.Fatal("wanted an error, got a successful reload")
		case <-time.After(300 * time.Millisecond):
		}
	}
	t.Fatal("timed out waiting for an error")
	return nil
}

func writeApp(t *testing.T, dir, body string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "app.tml"), []byte(component(body)), 0o644))
}

func TestWatchReloadsOnChange(t *testing.T) {
	dir := t.TempDir()
	writeApp(t, dir, `<Text>before</Text>`)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	got := newReloads()
	go tml.Watch(ctx, dir, "app.tml", tml.Options{}, got.onChange)

	view := got.awaitView(t, func() { writeApp(t, dir, `<Text>after</Text>`) })

	out, err := view.Render(nil, 20, 3)
	require.NoError(t, err)
	assert.Contains(t, out, "after", "the reloaded view reflects the edit")
}

// A reload that fails must reach the caller. Keeping the last good view quietly
// would hide the very typo hot reload exists to surface.
func TestWatchDeliversReloadFailures(t *testing.T) {
	dir := t.TempDir()
	writeApp(t, dir, `<Text>fine</Text>`)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	got := newReloads()
	go tml.Watch(ctx, dir, "app.tml", tml.Options{}, got.onChange)

	err := got.awaitError(t, func() { writeApp(t, dir, `<Nonsense/>`) })
	assert.Contains(t, err.Error(), "unknown element <Nonsense>")
}

func TestWatchStopsWhenTheContextIsCancelled(t *testing.T) {
	dir := t.TempDir()
	writeApp(t, dir, `<Text>x</Text>`)

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		tml.Watch(ctx, dir, "app.tml", tml.Options{}, func(*tml.View, error) {})
		close(done)
	}()

	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("watch did not stop on cancellation")
	}
}
