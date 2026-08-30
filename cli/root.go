// Package cli is the tml command line. Each subcommand lives in its own file and registers itself, so adding a command never
package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/wow-look-at-my/tml"
	"github.com/wow-look-at-my/tml/sema"
)

var root = &cobra.Command{
	Use:   "tml",
	Short: "Terminal Markup Language: declarative, reusable terminal components",
	// A diagnostic is the product here, so cobra must not add usage noise underneath a diagnostic. Execute prints the error
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the CLI, exiting non-nothing on any diagnostic.
func Execute() {
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// loadView opens a .tml file by path. The file's directory becomes the root imports resolve against, so a view is
func loadView(path string, dark bool, props []string) (*tml.View, tml.Props, error) {
	dir, file := filepath.Split(path)
	if dir == "" {
		dir = "."
	}
	view, err := tml.Load(os.DirFS(dir), filepath.Clean(file), tml.Options{Dark: dark})
	if err != nil {
		return nil, nil, err
	}
	parsed, err := parseProps(props)
	if err != nil {
		return nil, nil, err
	}
	return view, parsed, nil
}

// parseProps reads --prop name=value pairs. Values arrive untyped and are coerced against the component's declaration
func parseProps(pairs []string) (tml.Props, error) {
	props := tml.Props{}
	for _, pair := range pairs {
		name, value, ok := strings.Cut(pair, "=")
		if !ok {
			return nil, fmt.Errorf("--prop expects name=value, got %q", pair)
		}
		props[name] = sema.StringValue(value)
	}
	return props, nil
}
