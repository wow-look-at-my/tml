package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/wow-look-at-my/tml/inspect"
)

func init() { root.AddCommand(newTreeCmd()) }

// newTreeCmd prints the frame's whole box tree.
//
// Elements answers "what can a test address"; this answers "what is actually
// there". A layout usually goes wrong in a wrapper nobody gave an id to, and
// that box has to be visible for the mistake to be findable.
func newTreeCmd() *cobra.Command {
	var (
		asJSON bool
		ids    bool
	)
	cmd := &cobra.Command{
		Use:   "tree",
		Short: "Print the frame's element tree",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			res, err := ask(inspect.Request{Op: "tree"})
			if err != nil {
				return err
			}
			if asJSON {
				return encode(cmd.OutOrStdout(), res.Tree)
			}
			return printNode(cmd.OutOrStdout(), *res.Tree, "", ids)
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print the tree as JSON instead of an outline")
	cmd.Flags().BoolVar(&ids, "ids-only", false, "print only the nodes that carry an id")
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
