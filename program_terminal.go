// program_terminal.go holds enter.

package tml

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/wow-look-at-my/tml/cell"
	"github.com/wow-look-at-my/tml/term"
)

// enter takes the terminal. It returns what leave must undo, in order.
func (p *Program) enter() (stops []func(), err error) {
	p.tty = term.IsTerminal(p.out)
	p.measure()

	hasInput := !p.opts.NoInput
	if hasInput {
		in := p.opts.Input
		if in == nil {
			in = os.Stdin
		}
		if term.IsTerminal(in) {
			restore, rawErr := term.Raw(in)
			if rawErr != nil {
				return stops, fmt.Errorf("tml: cannot put the terminal in raw mode: %w", rawErr)
			}
			stops = append(stops, func() { _ = restore() })
		}
		reader, readErr := term.NewReader(in)
		if readErr != nil {
			return stops, fmt.Errorf("tml: cannot read the terminal: %w", readErr)
		}
		stops = append(stops, func() { _ = reader.Close() })
		go p.read(reader)
	}

	restoreOutput := term.EnableOutput(p.out)
	stops = append(stops, func() { _ = restoreOutput() })

	p.screen = term.NewScreen(p.out, term.Options{
		Inline:  p.opts.Inline,
		Mouse:   hasInput && !p.opts.NoMouse && !p.opts.Inline,
		Paste:   hasInput,
		Profile: cell.DetectProfile(p.opts.Env, p.tty),
	})
	if err := p.screen.Enter(); err != nil {
		return stops, err
	}

	signals := make(chan os.Signal, 4)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	resizes := make(chan os.Signal, 1)
	stopResize := term.NotifyResize(resizes)
	quit := make(chan struct{})
	go func() {
		for {
			select {
			case <-signals:
				p.Send(interruptMsg{})
			case <-resizes:
				p.wake()
			case <-quit:
				return
			}
		}
	}()
	stops = append(stops, func() {
		signal.Stop(signals)
		stopResize()
		close(quit)
	})
	return stops, nil
}

// leave gives the terminal back: the queued lines print, the screen goes, and what enter took is undone.
func (p *Program) leave(stops []func()) {
	lines := p.take(true)
	if p.screen != nil {
		if p.opts.Inline {
			_ = p.screen.Print(lines)
			lines = nil
		}
		_ = p.screen.Leave()
	}
	for i := len(stops) - 1; i >= 0; i-- {
		stops[i]()
	}
	p.mu.Lock()
	p.state = programStopped
	p.mu.Unlock()
	// A line queued while the screen was leaving still belongs to the terminal.
	p.writeLines(append(lines, p.take(true)...))
	close(p.done)
}

// measure reads the screen's size, and reports whether it changed.
func (p *Program) measure() bool {
	width, height := p.opts.Width, p.opts.Height
	if width <= 0 || height <= 0 {
		w, h, err := term.Size(p.out)
		if err != nil || w <= 0 || h <= 0 {
			w, h = fallbackWidth, fallbackHeight
		}
		if width <= 0 {
			width = w
		}
		if height <= 0 {
			height = h
		}
	}
	changed := width != p.width || height != p.height
	p.width, p.height = width, height
	return changed
}

// read decodes the terminal's input and sends each value to the loop.
func (p *Program) read(reader term.Reader) {
	var (
		mu      sync.Mutex
		decoder term.Decoder
		round   int
	)
	send := func(values []any) {
		for _, value := range values {
			p.Send(value)
		}
	}
	buf := make([]byte, 4096)
	for {
		n, err := reader.Read(buf)
		mu.Lock()
		round++
		values := decoder.Feed(buf[:n])
		if decoder.Pending() {
			// Bytes that end inside a sequence wait a moment for the rest. A lone ESC that stays alone is the key.
			mine := round
			time.AfterFunc(escapeDelay, func() {
				mu.Lock()
				var late []any
				if round == mine {
					late = decoder.Flush()
				}
				mu.Unlock()
				send(late)
			})
		}
		mu.Unlock()
		send(values)
		if err != nil {
			return
		}
	}
}
