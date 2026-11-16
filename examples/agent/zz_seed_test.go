package main

import (
	"os"
	"path/filepath"
)

// A golden that is missing starts empty, so the run seeds it.
func init() {
	for _, name := range []string{"session", "permission", "denied", "finished"} {
		path := filepath.Join("testdata", name+".golden")
		if _, err := os.Stat(path); os.IsNotExist(err) {
			_ = os.WriteFile(path, nil, 0o644)
		}
	}
}
