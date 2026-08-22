package cli

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/spf13/cobra"

	"github.com/wow-look-at-my/tml/inspect"
)

// client is one connection to a program's inspection socket.
type client struct {
	conn net.Conn
	dec  *json.Decoder
	enc  *json.Encoder
}

// socketFlag adds --socket to a live-program command. The flag is empty by
// default so discovery (and TML_INSPECT_SOCKET) run at call time rather than
// when the command is constructed.
func socketFlag(cmd *cobra.Command, dest *string) {
	cmd.Flags().StringVar(dest, "socket", "",
		"path of the program's inspection socket (default: discover, or $TML_INSPECT_SOCKET)")
}

// dial connects, or says why it could not in terms of what the caller can do
// about it. A missing socket file and a program that died holding one are
// different problems with different fixes.
func dial(path string) (*client, error) {
	if path == "" {
		return nil, errors.New("no socket resolved")
	}
	conn, err := net.DialTimeout("unix", path, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("cannot reach a program at %s: %w\nIs it running, and was it started with TML_INSPECT_SOCKET set to this path?", path, err)
	}
	reader := bufio.NewReaderSize(conn, 64*1024)
	return &client{conn: conn, dec: json.NewDecoder(reader), enc: json.NewEncoder(conn)}, nil
}

func (c *client) Close() error { return c.conn.Close() }

// do sends one request and returns its answer. A protocol error is returned as
// an error, so every caller reports it the same way.
func (c *client) do(req inspect.Request) (inspect.Response, error) {
	if err := c.enc.Encode(req); err != nil {
		return inspect.Response{}, fmt.Errorf("cannot send %s: %w", req.Op, err)
	}
	var res inspect.Response
	if err := c.dec.Decode(&res); err != nil {
		return inspect.Response{}, fmt.Errorf("cannot read the answer to %s: %w", req.Op, err)
	}
	if res.Error != "" {
		return res, errors.New(res.Error)
	}
	return res, nil
}

// ask opens a connection, asks one question and closes it. Every one-shot
// subcommand is this plus a printer. flag is the command's --socket value;
// resolveSocket turns it into a path, discovering the live program when it is
// empty.
func ask(flag string, req inspect.Request) (inspect.Response, error) {
	path, err := resolveSocket(flag)
	if err != nil {
		return inspect.Response{}, err
	}
	c, err := dial(path)
	if err != nil {
		return inspect.Response{}, err
	}
	defer c.Close()
	return c.do(req)
}
