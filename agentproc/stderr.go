package agentproc

import (
	"bytes"
	"io"
	"strings"
	"sync"
)

// StderrError is the end of the agent's standard error, which Wait's error
// has joined under Command.JoinStderrTail, so that a caller reporting the
// error reports what the agent said; errors.As reads it
// (0042-MADR-in-process-agent-runner.md, item 3).
type StderrError struct {
	// Tail is what the Command's caps kept, as Process.StderrTail returns it.
	Tail string
}

// Error returns the tail, its last newline left out.
func (e *StderrError) Error() string {
	return "agentproc: the agent's standard error ended: " + strings.TrimSuffix(e.Tail, "\n")
}

// tailWriter keeps the end of the agent's standard error: at most bytes
// bytes and at most lines lines, a cap of zero being no cap, from a line's
// beginning, unless the last line alone is longer than the byte cap, which
// keeps its end. It forwards every byte to next, the caller's Cmd.Stderr,
// when that is set, and returns next's result, so that os/exec sees the
// caller's writer as it would without the tail.
type tailWriter struct {
	next  io.Writer
	bytes int
	lines int

	mu  sync.Mutex
	buf []byte
}

// Write keeps p's end in the tail and forwards p.
func (w *tailWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	// One byte more than the cap, so that trim sees whether its cut falls
	// mid-line.
	keep := p
	if w.bytes > 0 && len(keep) > w.bytes+1 {
		keep = keep[len(keep)-w.bytes-1:]
	}
	w.buf = append(w.buf, keep...)
	w.trim()
	w.mu.Unlock()
	if w.next == nil {
		return len(p), nil
	}
	return w.next.Write(p)
}

// trim applies the caps to buf, under mu.
func (w *tailWriter) trim() {
	if w.lines > 0 {
		count := bytes.Count(w.buf, []byte{'\n'})
		if len(w.buf) > 0 && w.buf[len(w.buf)-1] != '\n' {
			count++ // an unended last line
		}
		for ; count > w.lines; count-- {
			i := bytes.IndexByte(w.buf, '\n')
			w.buf = w.buf[i+1:]
		}
	}
	if w.bytes > 0 && len(w.buf) > w.bytes {
		cut := len(w.buf) - w.bytes
		midLine := w.buf[cut-1] != '\n'
		w.buf = w.buf[cut:]
		// From the next line's beginning, unless the last line is all there is.
		if i := bytes.IndexByte(w.buf, '\n'); midLine && i >= 0 && i < len(w.buf)-1 {
			w.buf = w.buf[i+1:]
		}
	}
	if cap(w.buf) > 2*max(len(w.buf), 4096) {
		w.buf = bytes.Clone(w.buf) // let go of what was cut
	}
}

// tail returns what is kept.
func (w *tailWriter) tail() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return string(w.buf)
}
