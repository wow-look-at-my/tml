package cli

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/wow-look-at-my/tml"
)

// program is a single live program found in the socket directory.
type program struct {
	Path string
	PID  string
}

// resolveSocket answers which program to talk to. A path given by hand wins, then TML_INSPECT_SOCKET, and everything
func resolveSocket(flag string) (string, error) {
	if flag != "" {
		return flag, nil
	}
	if env := os.Getenv(tml.SocketEnv); env != "" {
		return env, nil
	}
	dir := tml.SocketDir()
	live, err := livePrograms(dir)
	if err != nil {
		return "", err
	}
	switch len(live) {
	case 0:
		return "", fmt.Errorf("no TML program is running: nothing answers in %s\nStart one, or pass --socket if it serves somewhere else (%s)", dir, tml.SocketEnv)
	case 1:
		return live[0].Path, nil
	default:
		return "", fmt.Errorf("%d TML programs are running; name one with --socket:\n%s", len(live), listing(live))
	}
}

// livePrograms is every socket in dir that a program is still behind. Each is dialled rather than trusted: a
func livePrograms(dir string) ([]program, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %w", dir, err)
	}
	var live []program
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".sock" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		probe, err := net.DialTimeout("unix", path, 200*time.Millisecond)
		if err != nil {
			continue
		}
		probe.Close()
		live = append(live, program{Path: path, PID: strings.TrimSuffix(entry.Name(), ".sock")})
	}
	sort.Slice(live, func(a, b int) bool { return live[a].Path < live[b].Path })
	return live, nil
}

func listing(live []program) string {
	var b strings.Builder
	for _, p := range live {
		fmt.Fprintf(&b, "  pid %s  --socket %s\n", p.PID, p.Path)
	}
	return strings.TrimRight(b.String(), "\n")
}

func init() { root.AddCommand(newListCmd()) }

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List the TML programs running as this user",
		Long: "Every program built on TML serves the inspection protocol, so this is\n" +
			"every one of them that is running now. A socket whose program has gone\n" +
			"is not listed: each is dialled before it is reported.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			dir := tml.SocketDir()
			live, err := livePrograms(dir)
			if err != nil {
				return err
			}
			if len(live) == 0 {
				return fmt.Errorf("no TML program is running: nothing answers in %s", dir)
			}
			fmt.Fprintln(cmd.OutOrStdout(), listing(live))
			return nil
		},
	}
}
