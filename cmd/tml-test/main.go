// Command tml-test inspects a running TML program.
//
// The program serves its frames on a unix socket; this asks the questions. It
// is the assertion side of a TML test: every answer describes one laid-out
// element, taken from the frame the program has on screen, so a test can say
// what an element contained and where it was at a moment rather than grepping
// a screenshot.
//
//	tml-test query --socket /tmp/app.sock --id composer
//	tml-test tree  --socket /tmp/app.sock
//	tml-test serve --socket /tmp/app.sock
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var root = &cobra.Command{
	Use:   "tml-test",
	Short: "Inspect a running TML program",
	Long: "Connects to a program serving the TML inspection protocol and asks it\n" +
		"about the frame it has on screen: one element by id, every element, the\n" +
		"whole tree, or the frame itself. `serve` opens a browser inspector.",
	SilenceUsage: true,
}

// socket is the connection every subcommand needs, so it is registered once on
// the root rather than repeated per command.
var socket string

func init() {
	root.PersistentFlags().StringVar(&socket, "socket", os.Getenv("TML_INSPECT_SOCKET"),
		"path of the program's inspection socket (default $TML_INSPECT_SOCKET)")
}

func main() {
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "tml-test:", err)
		os.Exit(1)
	}
}
