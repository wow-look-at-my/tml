package tml

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// DefaultWatchInterval is how often Watch looks for changes. Terminal layout is
// iterated on by saving and looking, so the delay wants to be below noticing.
const DefaultWatchInterval = 250 * time.Millisecond

// Watch reloads a view whenever any .tml file under dir changes, calling
// onChange with the result. It runs until ctx is cancelled.
//
// Changes are detected by polling modification times rather than by subscribing
// to filesystem events. Editors overwhelmingly save by writing a temporary file
// and renaming it over the original, which replaces the inode an event-based
// watcher is holding and drops exactly the change that mattered. Polling a
// directory has no such blind spot, and at this interval the cost is a stat per
// file per quarter second.
//
// **A reload failure is delivered, not swallowed.** onChange receives either a
// new view or an error, and the previous view is left untouched so the caller
// can keep rendering it. Callers should show the error -- a hot-reloading view
// that silently keeps the last good version hides the very typo it exists to
// surface.
func Watch(ctx context.Context, dir, entry string, opts Options, onChange func(*View, error)) {
	WatchInterval(ctx, dir, entry, opts, DefaultWatchInterval, onChange)
}

// WatchInterval is Watch at a rate the caller picks. How fast a save should show
// up is the caller's judgement, not ours: a tree being iterated on wants the
// default, and a program that keeps the watcher armed all session wants seconds
// rather than four directory walks of it.
func WatchInterval(ctx context.Context, dir, entry string, opts Options, interval time.Duration, onChange func(*View, error)) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	previous, err := fingerprint(dir)
	if err != nil {
		onChange(nil, err)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			current, err := fingerprint(dir)
			if err != nil {
				onChange(nil, err)
				continue
			}
			if current == previous {
				continue
			}
			previous = current
			onChange(Load(os.DirFS(dir), entry, opts))
		}
	}
}

// fingerprint summarises every .tml file under dir by name, size and
// modification time. Any add, delete or edit changes it.
func fingerprint(dir string) (string, error) {
	var b strings.Builder
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".tml") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		b.WriteString(path)
		b.WriteByte(0)
		b.WriteString(info.ModTime().String())
		b.WriteByte(0)
		// Size is included because two saves inside one modification-time tick
		// would otherwise look identical.
		b.WriteString(strconv.FormatInt(info.Size(), 10))
		b.WriteByte('\n')
		return nil
	})
	return b.String(), err
}
