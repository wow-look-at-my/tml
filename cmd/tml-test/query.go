package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/spf13/cobra"

	"github.com/wow-look-at-my/tml/inspect"
)

func init() {
	root.AddCommand(newQueryCmd(), newElementsCmd(), newIDsCmd(), newAtCmd(), newFrameCmd(), newKeyCmd(), newRestyleCmd())
}

// newQueryCmd reports one element by id.
func newQueryCmd() *cobra.Command {
	var (
		id        string
		keepANSI  bool
		field     string
		await     string
		awaitGone string
		timeout   time.Duration
	)
	cmd := &cobra.Command{
		Use:   "query",
		Short: "Report one element by id",
		Long: "Prints the element as JSON: what it is, where it landed, the space it\n" +
			"was given, its clip, its scroll position, whether it has focus, and the\n" +
			"lines it drew.\n" +
			"\n" +
			"--field prints one value on its own, which is what a shell assertion\n" +
			"wants: text, lines, x, y, w, h, focus, action, element.\n" +
			"\n" +
			"--await blocks until that value matches a regular expression, and\n" +
			"--await-gone until it stops matching. A screen a test asserts about is\n" +
			"still changing, so the question is always \"has it happened yet\", and a\n" +
			"sleep answers it by guessing at how fast the machine is. A timeout\n" +
			"exits non-zero naming what the element last drew.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if id == "" {
				return fmt.Errorf("--id is required; run `tml-test ids` to see what the frame declares")
			}
			if await != "" && awaitGone != "" {
				return fmt.Errorf("give --await or --await-gone, not both")
			}
			var res inspect.Response
			var err error
			if await != "" || awaitGone != "" {
				pattern, gone := await, false
				if awaitGone != "" {
					pattern, gone = awaitGone, true
				}
				res.Element, err = awaitField(id, keepANSI, field, pattern, gone, timeout)
			} else {
				res, err = ask(inspect.Request{Op: "query", ID: id, ANSI: keepANSI})
			}
			if err != nil {
				return err
			}
			if field != "" {
				return printField(cmd.OutOrStdout(), *res.Element, field)
			}
			return encode(cmd.OutOrStdout(), res.Element)
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "id of the element to report")
	cmd.Flags().BoolVar(&keepANSI, "ansi", false, "include the styled text as well as the plain text")
	cmd.Flags().StringVar(&field, "field", "", "print one field instead of the whole element")
	cmd.Flags().StringVar(&await, "await", "", "block until --field matches this regular expression")
	cmd.Flags().StringVar(&awaitGone, "await-gone", "", "block until --field stops matching this regular expression")
	cmd.Flags().DurationVar(&timeout, "timeout", 20*time.Second, "how long an await waits before failing")
	return cmd
}

func newElementsCmd() *cobra.Command {
	var keepANSI bool
	cmd := &cobra.Command{
		Use:   "elements",
		Short: "Report every id-bearing element, in document order",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			res, err := ask(inspect.Request{Op: "elements", ANSI: keepANSI})
			if err != nil {
				return err
			}
			return encode(cmd.OutOrStdout(), res.Elements)
		},
	}
	cmd.Flags().BoolVar(&keepANSI, "ansi", false, "include the styled text as well as the plain text")
	return cmd
}

func newIDsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ids",
		Short: "List the ids the current frame declares",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			res, err := ask(inspect.Request{Op: "ids"})
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), strings.Join(res.IDs, "\n"))
			return err
		},
	}
}

func newAtCmd() *cobra.Command {
	var x, y int
	cmd := &cobra.Command{
		Use:   "at",
		Short: "Report which element covers a cell",
		Long: "Prints the id of the innermost element covering the cell, and exits 1\n" +
			"when nothing does. It is the pointer's own question, asked from a shell.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			res, err := ask(inspect.Request{Op: "at", X: x, Y: y})
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
	cmd.Flags().IntVar(&x, "x", 0, "column, in cells")
	cmd.Flags().IntVar(&y, "y", 0, "row, in cells")
	return cmd
}

func newFrameCmd() *cobra.Command {
	var (
		keepANSI bool
		since    uint64
		maxWidth bool
	)
	cmd := &cobra.Command{
		Use:   "frame",
		Short: "Report the frame the program has on screen",
		Long: "With --since it waits for a frame newer than that sequence number, so a\n" +
			"test can say \"after the next paint\" instead of sleeping.\n" +
			"\n" +
			"--max-width prints the widest line of the frame in DISPLAY CELLS, which\n" +
			"is what catches a region that fits its rectangle and paints past its own\n" +
			"edge. Counting bytes or runes is a different number: a box-drawing rule\n" +
			"is three bytes a cell, and a wide glyph is one rune in two cells.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			res, err := ask(inspect.Request{Op: "frame", ANSI: keepANSI, Since: since})
			if err != nil {
				return err
			}
			if maxWidth {
				_, err := fmt.Fprintln(cmd.OutOrStdout(), widestLine(res.Frame.Text))
				return err
			}
			return encode(cmd.OutOrStdout(), res.Frame)
		},
	}
	cmd.Flags().BoolVar(&keepANSI, "ansi", false, "include the styled text as well as the plain text")
	cmd.Flags().Uint64Var(&since, "since", 0, "wait for a frame newer than this sequence number")
	cmd.Flags().BoolVar(&maxWidth, "max-width", false, "print the widest line of the frame, in display cells")
	return cmd
}

// widestLine measures in the cells a terminal draws, through the same function
// the engine lays out with.
func widestLine(text string) int {
	widest := 0
	for _, line := range strings.Split(text, "\n") {
		if w := lipgloss.Width(line); w > widest {
			widest = w
		}
	}
	return widest
}

func newKeyCmd() *cobra.Command {
	var (
		key  string
		x, y int
		clk  bool
	)
	cmd := &cobra.Command{
		Use:   "input",
		Short: "Send a keystroke or a click to the program",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			req := inspect.Request{Op: "key", Key: key}
			if clk {
				req = inspect.Request{Op: "click", X: x, Y: y}
			} else if key == "" {
				return fmt.Errorf("give --key, or --click with --x and --y")
			}
			if _, err := ask(req); err != nil {
				return err
			}
			_, err := fmt.Fprintln(cmd.OutOrStdout(), "ok")
			return err
		},
	}
	cmd.Flags().StringVar(&key, "key", "", "key name, such as enter or ctrl+c")
	cmd.Flags().BoolVar(&clk, "click", false, "click at --x,--y instead of sending a key")
	cmd.Flags().IntVar(&x, "x", 0, "column to click, in cells")
	cmd.Flags().IntVar(&y, "y", 0, "row to click, in cells")
	return cmd
}

func newRestyleCmd() *cobra.Command {
	var (
		id    string
		set   []string
		clear bool
	)
	cmd := &cobra.Command{
		Use:   "restyle",
		Short: "Override an element's attributes, or drop every override",
		Long: "Overrides land on the next frame the program paints. Any attribute the\n" +
			"document could carry works, layout and style alike: width, height,\n" +
			"margin-left, background, foreground.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if clear {
				if _, err := ask(inspect.Request{Op: "reset"}); err != nil {
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
			if _, err := ask(inspect.Request{Op: "restyle", ID: id, Attrs: attrs}); err != nil {
				return err
			}
			_, err := fmt.Fprintln(cmd.OutOrStdout(), "ok")
			return err
		},
	}
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
