package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/wow-look-at-my/tml/inspect"
)

func init() { root.AddCommand(newTreeCmd()) }

// newTreeCmd prints either a file's expanded document or a running program's
// laid-out frame. The two share a name because both are "the tree", and they
// are told apart by the argument: a path is a file; no path talks to the
// socket. Dropping either would leave a hole the other cannot fill — expansion
// is what layout sees, the live tree is what the terminal is showing.
func newTreeCmd() *cobra.Command {
	var (
		dark   bool
		props  []string
		sock   string
		asJSON bool
		ids    bool
	)
	cmd := &cobra.Command{
		Use:   "tree [file.tml]",
		Short: "Print an expanded document, or a running program's element tree",
		Long: "With a file, instantiates the view and prints the tree with components,\n" +
			"slots and control flow resolved away, which is what layout and rendering\n" +
			"see.\n" +
			"\n" +
			"With no file, asks a running program for the frame on screen, including\n" +
			"boxes nobody gave an id to — a layout usually goes wrong in a wrapper,\n" +
			"and that box has to be visible for the mistake to be findable. Connects\n" +
			"via --socket or TML_INSPECT_SOCKET.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				view, parsed, err := loadView(args[0], dark, props)
				if err != nil {
					return err
				}
				node, err := view.Expand(parsed)
				if err != nil {
					return err
				}
				fmt.Fprint(cmd.OutOrStdout(), node.Dump())
				return nil
			}
			res, err := ask(socketPath(sock), inspect.Request{Op: "tree"})
			if err != nil {
				return err
			}
			if asJSON {
				return encode(cmd.OutOrStdout(), res.Tree)
			}
			return printNode(cmd.OutOrStdout(), *res.Tree, "", ids)
		},
	}
	cmd.Flags().BoolVar(&dark, "dark", false, "resolve adaptive theme tokens to their dark value")
	cmd.Flags().StringArrayVar(&props, "prop", nil, "set a property as name=value (repeatable)")
	socketFlag(cmd, &sock)
	cmd.Flags().BoolVar(&asJSON, "json", false, "print the live tree as JSON instead of an outline")
	cmd.Flags().BoolVar(&ids, "ids-only", false, "print only the live-tree nodes that carry an id")
	return cmd
}

// printNode writes one node and its children as an indented outline. Geometry
// rides on the same line, because the question asked of a tree is nearly
// always "which box is the wrong size".
func printNode(w io.Writer, node inspect.Node, indent string, idsOnly bool) error {
	show := !idsOnly || node.ID != ""
	next := indent
	if show {
		label := "<" + node.Element + ">"
		if node.ID != "" {
			label += " #" + node.ID
		}
		if node.Action != "" {
			label += " action=" + node.Action
		}
		if node.Focus {
			label += " *focus*"
		}
		line := fmt.Sprintf("%s%-34s %3dx%-3d @%3d,%-3d", indent, label, node.Rect.W, node.Rect.H, node.Rect.X, node.Rect.Y)
		if node.Text != "" {
			line += "  " + strings.TrimSpace(node.Text)
		}
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
		next = indent + "  "
	}
	for _, child := range node.Children {
		if err := printNode(w, child, next, idsOnly); err != nil {
			return err
		}
	}
	return nil
}
