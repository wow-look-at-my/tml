package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/wow-look-at-my/tml/inspect"
)

func init() {
	root.AddCommand(newQueryCmd(), newElementsCmd(), newIDsCmd(), newAtCmd(), newFrameCmd(), newKeyCmd(), newRestyleCmd())
}

// newQueryCmd reports one element by id from a running program.
func newQueryCmd() *cobra.Command {
	var (
		sock     string
		id       string
		keepANSI bool
		field    string
	)
	cmd := &cobra.Command{
		Use:   "query",
		Short: "Report one element by id from a running program",
		Long: "Prints the element as JSON: what it is, where it landed, the space it\n" +
			"was given, its clip, its scroll position, whether it has focus, and the\n" +
			"lines it drew.\n" +
			"\n" +
			"--field prints one value on its own, which is what a shell assertion\n" +
			"wants: text, lines, x, y, w, h, focus, action, element.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if id == "" {
				return fmt.Errorf("--id is required; run `tml ids` to see what the frame declares")
			}
			res, err := ask(socketPath(sock), inspect.Request{Op: "query", ID: id, ANSI: keepANSI})
			if err != nil {
				return err
			}
			if field != "" {
				return printField(cmd.OutOrStdout(), *res.Element, field)
			}
			return encode(cmd.OutOrStdout(), res.Element)
		},
	}
	socketFlag(cmd, &sock)
	cmd.Flags().StringVar(&id, "id", "", "id of the element to report")
	cmd.Flags().BoolVar(&keepANSI, "ansi", false, "include the styled text as well as the plain text")
	cmd.Flags().StringVar(&field, "field", "", "print one field instead of the whole element")
	return cmd
}

// printField writes one value bare, so a test can compare it without a JSON
// parser in the way.
func printField(w io.Writer, el inspect.Element, field string) error {
	switch field {
	case "text":
		_, err := fmt.Fprintln(w, el.Text)
		return err
	case "lines":
		for _, line := range el.Lines {
			if _, err := fmt.Fprintln(w, line); err != nil {
				return err
			}
		}
		return nil
	case "x":
		return printInt(w, el.Rect.X)
	case "y":
		return printInt(w, el.Rect.Y)
	case "w":
		return printInt(w, el.Rect.W)
	case "h":
		return printInt(w, el.Rect.H)
	case "focus":
		_, err := fmt.Fprintln(w, el.Focus)
		return err
	case "action":
		_, err := fmt.Fprintln(w, el.Action)
		return err
	case "element":
		_, err := fmt.Fprintln(w, el.Element)
		return err
	default:
		return fmt.Errorf("unknown field %q; want one of text, lines, x, y, w, h, focus, action, element", field)
	}
}

func printInt(w io.Writer, n int) error {
	_, err := fmt.Fprintln(w, n)
	return err
}

func newElementsCmd() *cobra.Command {
	var (
		sock     string
		keepANSI bool
	)
	cmd := &cobra.Command{
		Use:   "elements",
		Short: "Report every id-bearing element of a running program, in document order",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			res, err := ask(socketPath(sock), inspect.Request{Op: "elements", ANSI: keepANSI})
			if err != nil {
				return err
			}
			return encode(cmd.OutOrStdout(), res.Elements)
		},
	}
	socketFlag(cmd, &sock)
	cmd.Flags().BoolVar(&keepANSI, "ansi", false, "include the styled text as well as the plain text")
	return cmd
}

func newIDsCmd() *cobra.Command {
	var sock string
	cmd := &cobra.Command{
		Use:   "ids",
		Short: "List the ids the current frame of a running program declares",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			res, err := ask(socketPath(sock), inspect.Request{Op: "ids"})
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), strings.Join(res.IDs, "\n"))
			return err
		},
	}
	socketFlag(cmd, &sock)
	return cmd
}

func newAtCmd() *cobra.Command {
	var sock string
	var x, y int
	cmd := &cobra.Command{
		Use:   "at",
		Short: "Report which element of a running program covers a cell",
		Long: "Prints the id of the innermost element covering the cell, and exits 1\n" +
			"when nothing does. It is the pointer's own question, asked from a shell.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			res, err := ask(socketPath(sock), inspect.Request{Op: "at", X: x, Y: y})
			if err != nil {
				return err
			}
			if !res.Found {
				return fmt.Errorf("no element covers cell %d,%d", x, y)
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), res.Hit)
			return err
		},
	}
	socketFlag(cmd, &sock)
	cmd.Flags().IntVar(&x, "x", 0, "column, in cells")
	cmd.Flags().IntVar(&y, "y", 0, "row, in cells")
	return cmd
}

func newFrameCmd() *cobra.Command {
	var (
		sock     string
		keepANSI bool
		since    uint64
	)
	cmd := &cobra.Command{
		Use:   "frame",
		Short: "Report the frame a running program has on screen",
		Long: "With --since it waits for a frame newer than that sequence number, so a\n" +
			"test can say \"after the next paint\" instead of sleeping.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			res, err := ask(socketPath(sock), inspect.Request{Op: "frame", ANSI: keepANSI, Since: since})
			if err != nil {
				return err
			}
			return encode(cmd.OutOrStdout(), res.Frame)
		},
	}
	socketFlag(cmd, &sock)
	cmd.Flags().BoolVar(&keepANSI, "ansi", false, "include the styled text as well as the plain text")
	cmd.Flags().Uint64Var(&since, "since", 0, "wait for a frame newer than this sequence number")
	return cmd
}

func newKeyCmd() *cobra.Command {
	var (
		sock string
		key  string
		x, y int
		clk  bool
	)
	cmd := &cobra.Command{
		Use:   "input",
		Short: "Send a keystroke or a click to a running program",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			req := inspect.Request{Op: "key", Key: key}
			if clk {
				req = inspect.Request{Op: "click", X: x, Y: y}
			} else if key == "" {
				return fmt.Errorf("give --key, or --click with --x and --y")
			}
			if _, err := ask(socketPath(sock), req); err != nil {
				return err
			}
			_, err := fmt.Fprintln(cmd.OutOrStdout(), "ok")
			return err
		},
	}
	socketFlag(cmd, &sock)
	cmd.Flags().StringVar(&key, "key", "", "key name, such as enter or ctrl+c")
	cmd.Flags().BoolVar(&clk, "click", false, "click at --x,--y instead of sending a key")
	cmd.Flags().IntVar(&x, "x", 0, "column to click, in cells")
	cmd.Flags().IntVar(&y, "y", 0, "row to click, in cells")
	return cmd
}

func newRestyleCmd() *cobra.Command {
	var (
		sock  string
		id    string
		set   []string
		clear bool
	)
	cmd := &cobra.Command{
		Use:   "restyle",
		Short: "Override a running program's element attributes, or drop every override",
		Long: "Overrides land on the next frame the program paints. Any attribute the\n" +
			"document could carry works, layout and style alike: width, height,\n" +
			"margin-left, background, foreground.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			path := socketPath(sock)
			if clear {
				if _, err := ask(path, inspect.Request{Op: "reset"}); err != nil {
					return err
				}
				_, err := fmt.Fprintln(cmd.OutOrStdout(), "overrides dropped")
				return err
			}
			if id == "" || len(set) == 0 {
				return fmt.Errorf("give --id and at least one --set name=value, or --clear")
			}
			attrs := map[string]string{}
			for _, pair := range set {
				name, value, ok := strings.Cut(pair, "=")
				if !ok {
					return fmt.Errorf("--set wants name=value, got %q", pair)
				}
				attrs[name] = value
			}
			if _, err := ask(path, inspect.Request{Op: "restyle", ID: id, Attrs: attrs}); err != nil {
				return err
			}
			_, err := fmt.Fprintln(cmd.OutOrStdout(), "ok")
			return err
		},
	}
	socketFlag(cmd, &sock)
	cmd.Flags().StringVar(&id, "id", "", "id of the element to override")
	cmd.Flags().StringArrayVar(&set, "set", nil, "attribute as name=value (repeatable)")
	cmd.Flags().BoolVar(&clear, "clear", false, "drop every override")
	return cmd
}

func encode(w io.Writer, body any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(body)
}
