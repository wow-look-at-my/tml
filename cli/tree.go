package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	var (
		dark  bool
		props []string
	)

	tree := &cobra.Command{
		Use:   "tree <file.tml>",
		Short: "Print the expanded element tree",
		Long: "Instantiates the view and prints the tree with components, slots and " +
			"control flow resolved away, which is what layout and rendering see.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
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
		},
	}
	tree.Flags().BoolVar(&dark, "dark", false, "resolve adaptive theme tokens to their dark value")
	tree.Flags().StringArrayVar(&props, "prop", nil, "set a property as name=value (repeatable)")
	root.AddCommand(tree)
}
