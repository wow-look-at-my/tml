package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	var dark bool

	check := &cobra.Command{
		Use:   "check <file.tml>",
		Short: "Parse and check a view without rendering it",
		Long: "Reports every diagnostic that does not depend on runtime arguments: " +
			"malformed XML, unknown elements, bad property types, unresolved references.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, _, err := loadView(args[0], dark, nil); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "ok: %s\n", args[0])
			return nil
		},
	}
	check.Flags().BoolVar(&dark, "dark", false, "resolve adaptive theme tokens to their dark value")
	root.AddCommand(check)
}
