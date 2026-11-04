package remote

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"

	acp "github.com/maccavelli/acp-go-sdk2"
)

// errLineTooLong is what a lineSplitter's Write returns for a line longer
// than its limit, which it drops.
var errLineTooLong = errors.New("remote: message exceeds the size limit")

// overlongAnswer is the line a side answers a message over limit bytes with,
// as a connection answers an over-long line on stdio: -32600 with "id": null,
// and the connection's own text (0049-PLAN-subpackage-fixes.md, phase 5, P6
// and P9).
func overlongAnswer(limit int) []byte {
	b, err := json.Marshal(struct {
		JSONRPC string            `json:"jsonrpc"`
		ID      *int              `json:"id"`
		Error   *acp.RequestError `json:"error"`
	}{"2.0", nil, acp.NewInvalidRequest(map[string]any{"error": fmt.Sprintf("message exceeds %d bytes", limit)})})
	if err != nil {
		panic(err) // a fixed shape, which always encodes
	}
	return b
}

// answerer writes the answers to over-long messages on a goroutine of its
// own, so the reader or the pump that found one never waits on the side it
// answers. While one answer waits, a later one is not written, so at most one
// goroutine is held; the answer's write ends at its own bound or at the
// stream's end.
type answerer struct {
	busy atomic.Bool
}

// answer starts write unless an answer is still being written.
func (a *answerer) answer(write func()) {
	if !a.busy.CompareAndSwap(false, true) {
		return
	}
	go func() {
		defer a.busy.Store(false)
		write()
	}()
}

// lineSplitter is the writing half of the seam: the bytes a connection or a
// serve writes are split at '\n', a line split across writes is joined, as
// agentproc's LineTap joins it, and each complete line, without its newline,
// goes to send as one message. What is held when the stream ends is not a
// line, and Close drops it. A line longer than maxBytes is dropped, and Write
// returns errLineTooLong; the lines after it are sent. It is for one
// goroutine.
type lineSplitter struct {
	send     func(line []byte) error
	maxBytes int
	held     []byte
	skipping bool // dropping the rest of an over-long line
}

// newLineSplitter returns a lineSplitter that gives each line to send and
// refuses one longer than maxBytes.
func newLineSplitter(send func(line []byte) error, maxBytes int) *lineSplitter {
	return &lineSplitter{send: send, maxBytes: maxBytes}
}

// Write sends each line p ends and holds what follows the last newline. It
// returns the first error: send's, which stops it, or errLineTooLong, which
// does not.
func (s *lineSplitter) Write(p []byte) (int, error) {
	var tooLong bool
	consumed := 0
	for {
		i := bytes.IndexByte(p, '\n')
		if i < 0 {
			if !s.skipping {
				s.held = append(s.held, p...)
				if len(s.held) > s.maxBytes {
					s.held, s.skipping, tooLong = nil, true, true
				}
			}
			break
		}
		if s.skipping {
			s.skipping = false
		} else {
			line := make([]byte, 0, len(s.held)+i)
			line = append(append(line, s.held...), p[:i]...)
			s.held = nil
			if len(line) > s.maxBytes {
				tooLong = true
			} else if err := s.send(line); err != nil {
				return consumed, err
			}
		}
		consumed += i + 1
		p = p[i+1:]
	}
	if tooLong {
		return consumed + len(p), fmt.Errorf("%w: more than %d bytes", errLineTooLong, s.maxBytes)
	}
	return consumed + len(p), nil
}

// Close drops the line not yet ended and returns nil.
func (s *lineSplitter) Close() error {
	s.held, s.skipping = nil, false
	return nil
}

// toLine makes msg, one message as a transport received it, one line of the
// seam, with its newline. JSON is compacted, so it holds no newline, and its
// id keeps its bytes. Anything else has its CR and LF bytes replaced by
// spaces, so the connection answers it as it answers an unparsable line on
// stdio (docs/decisions/0043-MADR-remote-http-and-websocket-transport.md,
// "Amended by the PLAN, 2026-10-29", 6).
func toLine(msg []byte) []byte {
	var b bytes.Buffer
	b.Grow(len(msg) + 1)
	if json.Valid(msg) && json.Compact(&b, msg) == nil {
		b.WriteByte('\n')
		return b.Bytes()
	}
	b.Reset()
	for _, c := range msg {
		if c == '\r' || c == '\n' {
			c = ' '
		}
		b.WriteByte(c)
	}
	b.WriteByte('\n')
	return b.Bytes()
}

// lineQueue is the reading half of the seam: the lines a transport received,
// in the order pushed, read as one byte stream. push waits while the bytes it
// holds and the line's would pass its limit, so a reader that falls behind
// slows the transport rather than growing the queue; below the limit the
// transport goes on reading, and so answers a WebSocket's pings
// (0049-PLAN-subpackage-fixes.md, phase 1, P1). A line longer than the limit
// is taken once the queue is empty. After end, Read returns the lines already
// pushed, then end's error; a push after end, or waiting at end, returns
// false.
type lineQueue struct {
	mu    sync.Mutex
	cond  *sync.Cond
	lines [][]byte
	cur   []byte // the part of the first line not yet read
	held  int    // the bytes of lines and cur not yet read
	limit int
	err   error // set by end
}

// newLineQueue returns an empty lineQueue that holds up to limit bytes.
func newLineQueue(limit int) *lineQueue {
	q := &lineQueue{limit: limit}
	q.cond = sync.NewCond(&q.mu)
	return q
}

// push queues line, which must end in '\n', and reports whether it was
// queued.
func (q *lineQueue) push(line []byte) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	for q.err == nil && q.held > 0 && q.held+len(line) > q.limit {
		q.cond.Wait()
	}
	if q.err != nil {
		return false
	}
	q.lines = append(q.lines, line)
	q.held += len(line)
	q.cond.Broadcast()
	return true
}

// end ends the queue with err, or io.EOF when err is nil. The first end wins.
func (q *lineQueue) end(err error) {
	if err == nil {
		err = io.EOF
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.err == nil {
		q.err = err
		q.cond.Broadcast()
	}
}

// Read reads the queued lines as one stream. It waits for a line, and returns
// end's error once every line pushed before it has been read.
func (q *lineQueue) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	for len(q.cur) == 0 {
		if len(q.lines) > 0 {
			q.cur, q.lines = q.lines[0], q.lines[1:]
			continue
		}
		if q.err != nil {
			return 0, q.err
		}
		q.cond.Wait()
	}
	n := copy(p, q.cur)
	q.cur = q.cur[n:]
	q.held -= n
	q.cond.Broadcast()
	return n, nil
}
