package agentproc

import (
	"bytes"
	"io"
)

// LineTap returns a writer, for Command.OutputTap or InputTap, that calls fn
// with each line written to it, without its newline: a frame of the wire, as
// the connection reads or writes it. Each frame is a copy, which fn may keep
// and change. A line split across writes is joined, and a line not yet ended
// is held, whole, until its newline arrives; a "\r" before the newline is
// kept. What is held when the stream ends is not a line, and fn never sees
// it. The writer is also an io.Closer, whose Close drops it.
//
// The writer is for one stream, written from one goroutine, as a tap is: set
// as both taps, it would be written from two, so each tap takes a LineTap of
// its own. fn runs on the tap's goroutine, and the connection does not wait
// for it. LineTap panics if fn is nil
// (0041-MADR-close-the-gaps-the-consumers-wait-on.md, item 5).
func LineTap(fn func(frame []byte)) io.Writer {
	if fn == nil {
		panic("agentproc: LineTap's fn is nil")
	}
	return &lineTap{fn: fn}
}

// lineTap is the writer LineTap returns.
type lineTap struct {
	fn   func(frame []byte)
	held []byte // the line not yet ended
}

// Write calls fn with each line p ends, and holds what follows the last
// newline. It never fails.
func (l *lineTap) Write(p []byte) (int, error) {
	n := len(p)
	for {
		i := bytes.IndexByte(p, '\n')
		if i < 0 {
			l.held = append(l.held, p...)
			return n, nil
		}
		frame := make([]byte, 0, len(l.held)+i)
		frame = append(append(frame, l.held...), p[:i]...)
		l.held = nil
		l.fn(frame)
		p = p[i+1:]
	}
}

// Close drops the line not yet ended, which is not a frame, and returns nil.
func (l *lineTap) Close() error {
	l.held = nil
	return nil
}
