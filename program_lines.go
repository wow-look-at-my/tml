// program_lines.go holds Println.

package tml

import (
	"fmt"
	"strings"
)

// Println queues a line to print above an inline program.
func (p *Program) Println(a ...any) { p.line(fmt.Sprint(a...)) }

// Printf queues a formatted line. A trailing newline is not needed.
func (p *Program) Printf(format string, a ...any) { p.line(fmt.Sprintf(format, a...)) }

// Write takes output such as a child's stderr. A partial line waits for its newline, so a line never arrives in
// pieces.
func (p *Program) Write(b []byte) (int, error) {
	p.mu.Lock()
	if p.state == programStopped {
		p.mu.Unlock()
		return p.out.Write(b)
	}
	p.partial += strings.ReplaceAll(string(b), "\r", "")
	for {
		i := strings.IndexByte(p.partial, '\n')
		if i < 0 {
			break
		}
		p.pending = append(p.pending, p.partial[:i])
		p.partial = p.partial[i+1:]
	}
	p.mu.Unlock()
	p.wake()
	return len(b), nil
}

func (p *Program) line(s string) {
	p.mu.Lock()
	if p.state == programStopped {
		p.mu.Unlock()
		p.writeLines([]string{s})
		return
	}
	p.pending = append(p.pending, strings.Split(strings.TrimSuffix(s, "\n"), "\n")...)
	p.mu.Unlock()
	p.wake()
}

// wake asks the loop to look at its queues. A loop already asked needs no second asking.
func (p *Program) wake() {
	select {
	case p.signal <- struct{}{}:
	default:
	}
}

// take removes the queued lines. The partial line comes too when the program is about to end.
func (p *Program) take(partial bool) []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	lines := p.pending
	p.pending = nil
	if partial && p.partial != "" {
		lines = append(lines, p.partial)
		p.partial = ""
	}
	return lines
}

func (p *Program) writeLines(lines []string) {
	for _, line := range lines {
		_, _ = fmt.Fprintln(p.out, line)
	}
}

// flush is Stop for a program that never ran: the queued lines go straight to the output.
func (p *Program) flush() bool {
	p.mu.Lock()
	if p.state != programIdle {
		p.mu.Unlock()
		return false
	}
	p.state = programStopped
	p.mu.Unlock()
	p.writeLines(p.take(true))
	close(p.done)
	return true
}
