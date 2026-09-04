package cli

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/wow-look-at-my/tml/inspect"
	"github.com/wow-look-at-my/tml/render"
)

func init() { root.AddCommand(newCaptureCmd()) }

// newCaptureCmd writes the inspector over a single frame, as a self-contained HTML file.
func newCaptureCmd() *cobra.Command {
	var (
		dark          bool
		width, height int
		props         []string
		sock          string
		out           string
	)
	cmd := &cobra.Command{
		Use:   "capture [file.tml]",
		Short: "Write one frame as a self-contained inspector page",
		Long: "Writes the browser inspector over a single frame as one HTML file: the\n" +
			"terminal as it was painted, the element tree, and every element's\n" +
			"geometry, clip, scroll and drawn text. The file fetches nothing when it\n" +
			"opens, so it travels through a comment box or an email.\n" +
			"\n" +
			"With a file, lays the view out at the given size and captures that. With\n" +
			"no file, captures the frame a running program has on screen, over\n" +
			"--socket, $TML_INSPECT_SOCKET, or by discovering the live program.\n" +
			"\n" +
			"A capture is read-only by construction: driving a program and overriding\n" +
			"a style both need the program, so the page leaves those controls out.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			capture, err := takeCapture(args, dark, width, height, props, sock)
			if err != nil {
				return err
			}
			return writeCapture(cmd, capture, out)
		},
	}
	cmd.Flags().BoolVar(&dark, "dark", false, "resolve adaptive theme tokens to their dark value")
	cmd.Flags().IntVar(&width, "width", 80, "viewport width in cells, when capturing a file")
	cmd.Flags().IntVar(&height, "height", 24, "viewport height in cells, when capturing a file")
	cmd.Flags().StringArrayVar(&props, "prop", nil, "set a property as name=value (repeatable)")
	socketFlag(cmd, &sock)
	cmd.Flags().StringVarP(&out, "out", "o", "", "write to this path instead of stdout")
	return cmd
}

func takeCapture(args []string, dark bool, width, height int, props []string, sock string) (inspect.Capture, error) {
	if len(args) == 0 {
		path, err := resolveSocket(sock)
		if err != nil {
			return inspect.Capture{}, err
		}
		return inspect.Snapshot(&proxy{path: path})
	}
	view, parsed, err := loadView(args[0], dark, props)
	if err != nil {
		return inspect.Capture{}, err
	}
	box, err := view.Layout(parsed, width, height)
	if err != nil {
		return inspect.Capture{}, err
	}
	return inspect.SnapshotFrame(inspect.Frame{
		Seq: 1, At: time.Now(), Width: width, Height: height,
		Box: box, State: inspect.StateOf(view.UI().Targets()), ANSI: render.Render(box),
	})
}

// writeCapture sends the page to stdout, or to a file and its path to stdout. The path is what a caller opens next, and
// a page written down a pipe with a note above it is not an HTML file any more.
func writeCapture(cmd *cobra.Command, capture inspect.Capture, out string) error {
	if out == "" {
		return inspect.WriteCapture(cmd.OutOrStdout(), capture)
	}
	file, err := os.Create(out)
	if err != nil {
		return fmt.Errorf("cannot write %s: %w", out, err)
	}
	if err := writeAndClose(file, capture); err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), out)
	return nil
}

func writeAndClose(file io.WriteCloser, capture inspect.Capture) error {
	if err := inspect.WriteCapture(file, capture); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}
