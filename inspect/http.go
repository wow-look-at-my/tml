package inspect

import (
	"embed"
	"encoding/json"
	"io/fs"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"
)

//go:embed ui
var uiFS embed.FS

// httpHandler serves the browser inspector: the page, its assets, the same RPC
// the socket speaks, and a stream that pushes each new frame.
func (s *Server) httpHandler() http.Handler {
	mux := http.NewServeMux()

	assets, err := fs.Sub(uiFS, "ui")
	if err != nil {
		// The assets are embedded at build time, so a failure here is a build
		// that shipped without them rather than anything a user can fix.
		panic("inspect: embedded ui assets are unreadable: " + err.Error())
	}
	mux.Handle("/", http.FileServer(http.FS(assets)))
	mux.HandleFunc("/rpc", s.serveRPC)
	mux.HandleFunc("/events", s.serveEvents)
	return mux
}

func (s *Server) serveRPC(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, Response{Error: "rpc takes POST"})
		return
	}
	var req Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, Response{Error: fmt.Sprintf("request is not JSON: %v", err)})
		return
	}
	writeJSON(w, http.StatusOK, s.Handle(req))
}

// serveEvents pushes one message per painted frame, so the preview follows the
// program without the page polling it.
//
// The stream also sends a heartbeat, because a program that paints once and
// then sits still is indistinguishable from a dead connection otherwise, and a
// preview that quietly stopped updating is the worst thing this page could do.
func (s *Server) serveEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, Response{Error: "this server cannot stream"})
		return
	}
	h := w.Header()
	h.Set("content-type", "text/event-stream")
	h.Set("cache-control", "no-cache")
	h.Set("connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	var last uint64
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	for {
		f, ok := s.source.Frame()
		if ok && f.Seq != last {
			last = f.Seq
			payload, err := json.Marshal(FrameInfo{
				Seq: f.Seq, At: f.At.UTC().Format(time.RFC3339Nano),
				Width: f.Width, Height: f.Height,
				Text: stripANSI(f.ANSI), ANSI: f.ANSI,
			})
			if err != nil {
				return
			}
			fmt.Fprintf(w, "event: frame\ndata: %s\n\n", payload)
			flusher.Flush()
		}

		wake := s.wait()
		select {
		case <-wake:
		case <-heartbeat.C:
			fmt.Fprint(w, ": still here\n\n")
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(code)
	encoded, err := json.Marshal(body)
	if err != nil {
		return
	}
	_, _ = w.Write(encoded)
}

func stripANSI(s string) string { return ansi.Strip(s) }

func join(items []string) string { return strings.Join(items, ", ") }
