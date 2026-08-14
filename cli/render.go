package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	var (
		dark          bool
		width, height int
		props         []string
	)

	render := &cobra.Command{
		Use:   "render <file.tml>",
		Short: "Render a view to stdout",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			view, parsed, err := loadView(args[0], dark, props)
			if err != nil {
				return err
			}
			out, err := view.Render(parsed, width, height)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), out)
			return nil
		},
	}
	render.Flags().BoolVar(&dark, "dark", false, "resolve adaptive theme tokens to their dark value")
	render.Flags().IntVar(&width, "width", 80, "viewport width in cells")
	render.Flags().IntVar(&height, "height", 24, "viewport height in cells")
	render.Flags().StringArrayVar(&props, "prop", nil, "set a property as name=value (repeatable)")
	root.AddCommand(render)
}
