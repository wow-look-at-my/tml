// Package cli is the tml command line. Each subcommand lives in its own file and registers itself, so adding a command never
package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
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

// loadView opens a .tml file by path. The file's directory becomes the root imports resolve against.
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

// propHelp is what --prop says about itself on every command that takes it.
const propHelp = "set a property as name=value, or as @file.json for a file of typed values (repeatable)"

// parseProps reads --prop name=value pairs. Values arrive untyped and are coerced against the component's declaration
func parseProps(pairs []string) (tml.Props, error) {
	props := tml.Props{}
	for _, pair := range pairs {
		// A list of records has no spelling as name=value, so a file of typed values stands in for a host.
		if file, ok := strings.CutPrefix(pair, "@"); ok {
			if err := readProps(file, props); err != nil {
				return nil, err
			}
			continue
		}
		name, value, ok := strings.Cut(pair, "=")
		if !ok {
			return nil, fmt.Errorf("--prop expects name=value or @file.json, got %q", pair)
		}
		props[name] = sema.StringValue(value)
	}
	return props, nil
}

// readProps adds the properties a JSON file holds: an object whose members are the property values.
func readProps(file string, props tml.Props) error {
	raw, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("--prop @%s: %w", file, err)
	}
	var members map[string]any
	if err := json.Unmarshal(raw, &members); err != nil {
		return fmt.Errorf("--prop @%s: expected a JSON object of properties: %w", file, err)
	}
	for name, member := range members {
		value, err := propValue(member)
		if err != nil {
			return fmt.Errorf("--prop @%s: %s: %w", file, name, err)
		}
		props[name] = value
	}
	return nil
}

// propValue turns one JSON value into the value a host would pass. A number is its text, as every scalar is at this
// boundary. An array of objects is a list of records, and an array of anything else is a list of strings.
func propValue(member any) (sema.Value, error) {
	switch v := member.(type) {
	case string:
		return sema.StringValue(v), nil
	case bool:
		return sema.BoolValue(v), nil
	case float64:
		return sema.StringValue(strconv.FormatFloat(v, 'f', -1, 64)), nil
	case []any:
		return propList(v)
	default:
		return sema.Value{}, fmt.Errorf("a property is text, a number, true or false, or a list, and this is %T", member)
	}
}

func propList(items []any) (sema.Value, error) {
	records := make([]map[string]sema.Value, 0, len(items))
	texts := make([]string, 0, len(items))
	for _, item := range items {
		switch v := item.(type) {
		case map[string]any:
			record := make(map[string]sema.Value, len(v))
			for field, member := range v {
				value, err := propValue(member)
				if err != nil {
					return sema.Value{}, fmt.Errorf("%s: %w", field, err)
				}
				record[field] = value
			}
			records = append(records, record)
		case string:
			texts = append(texts, v)
		case float64:
			texts = append(texts, strconv.FormatFloat(v, 'f', -1, 64))
		default:
			return sema.Value{}, fmt.Errorf("a list holds text or records, and this holds %T", item)
		}
	}
	if len(records) > 0 && len(texts) > 0 {
		return sema.Value{}, errors.New("a list holds text or records, and this holds both")
	}
	if len(records) > 0 {
		return sema.RecordListValue(records), nil
	}
	return sema.ListValue(texts), nil
}
