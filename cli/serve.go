package cli

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/wow-look-at-my/tml/inspect"
)

func init() { root.AddCommand(newServeCmd()) }

// proxy answers inspection requests by forwarding them over the program's
// socket. One connection per request keeps a long-blocking frame wait from
// holding up an ordinary query, which matters because the page streams frames
// and answers clicks at the same time.
type proxy struct {
	path string
	// mu guards nothing but the error the last dial produced, so the page can
	// be told why it went quiet.
	mu   sync.Mutex
	last string
}

func (p *proxy) Handle(req inspect.Request) inspect.Response {
	c, err := dial(p.path)
	if err != nil {
		p.note(err.Error())
		return inspect.Response{Error: err.Error()}
	}
	defer c.Close()
	res, err := c.do(req)
	if err != nil && res.Error == "" {
		p.note(err.Error())
		return inspect.Response{Error: err.Error()}
	}
	return res
}

func (p *proxy) note(msg string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if msg != p.last {
		fmt.Fprintln(os.Stderr, "tml:", msg)
		p.last = msg
	}
}

// newServeCmd runs the browser inspector.
func newServeCmd() *cobra.Command {
	var (
		sock string
		addr string
		open bool
	)
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Serve the interactive inspector in a browser",
		Long: "Opens a local HTTP server showing the program's terminal live, its\n" +
			"element tree, and the element you pick. Click the preview to select,\n" +
			"drag to move an element, shift-drag to resize, and edit any attribute\n" +
			"from the panel. Every edit is an override the program lays out for\n" +
			"real, so what the browser shows is what the terminal shows.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			path, err := resolveSocket(sock)
			if err != nil {
				return err
			}
			p := &proxy{path: path}
			// Fail here rather than after printing a URL that answers nothing:
			// a page that loads and then says "connection lost" is a worse
			// error message than the dial's own.
			probe, err := dial(path)
			if err != nil {
				return err
			}
			probe.Close()

			ln, err := net.Listen("tcp", addr)
			if err != nil {
				return fmt.Errorf("cannot listen on %s: %w", addr, err)
			}
			url := "http://" + ln.Addr().String()
			fmt.Fprintf(cmd.OutOrStdout(), "inspector on %s (program: %s)\n", url, path)
			if open {
				fmt.Fprintln(cmd.OutOrStdout(), "open that URL in a browser")
			}

			srv := &http.Server{Handler: inspect.HTTPHandler(p), ReadHeaderTimeout: 10 * time.Second}
			stop := make(chan os.Signal, 1)
			signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
			go func() {
				<-stop
				_ = srv.Close()
			}()
			if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
				return err
			}
			return nil
		},
	}
	socketFlag(cmd, &sock)
	cmd.Flags().StringVar(&addr, "addr", "127.0.0.1:0", "address to serve the inspector on")
	cmd.Flags().BoolVar(&open, "print-open-hint", true, "print a line telling you to open the URL")
	return cmd
}
