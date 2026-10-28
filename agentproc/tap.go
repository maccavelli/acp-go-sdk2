package agentproc

import (
	"bytes"
	"io"
	"sync"
)

// tapQueueBytes bounds what a tap's queue holds: a copy that would pass it
// detaches the tap (0030-MADR-record-initialize-on-arrival.md, the amendment
// of 2026-10-21, questions 9 and 10).
const tapQueueBytes = 32 << 20

// tap copies a stream to the caller's writer on a goroutine of its own, so
// that the stream never waits for the writer
// (0030-MADR-record-initialize-on-arrival.md, the amendment of 2026-10-21,
// H1-async). What the writer receives is the stream's beginning, in order:
// once a copy would pass tapQueueBytes, the tap is detached, and every byte
// it does not receive is counted in dropped.
type tap struct {
	w io.Writer

	mu        sync.Mutex
	queue     [][]byte
	queued    uint64 // bytes queued, the one being written included
	inflight  uint64 // bytes of the copy being written
	ended     bool   // the stream has ended: nothing more is offered
	detached  bool   // nothing more is queued
	abandoned bool   // nothing more is written
	dropped   uint64

	wake chan struct{} // wakes run; holds one signal
	done chan struct{} // closed when run returns
}

// newTap starts a tap's goroutine, writing to w.
func newTap(w io.Writer) *tap {
	t := &tap{w: w, wake: make(chan struct{}, 1), done: make(chan struct{})}
	go t.run()
	return t
}

// offer queues a copy of p, unless the tap is detached or the copy would
// pass tapQueueBytes, which detaches it; then p is counted as dropped. It
// never waits.
func (t *tap) offer(p []byte) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.detached || t.ended || t.queued+uint64(len(p)) > tapQueueBytes {
		t.detached = true
		t.dropped += uint64(len(p))
		return
	}
	t.queue = append(t.queue, bytes.Clone(p))
	t.queued += uint64(len(p))
	t.signal()
}

// end marks the stream ended: run returns once it has written what is queued.
func (t *tap) end() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ended = true
	t.signal()
}

// abandon detaches the tap and counts what it has not written as dropped:
// run writes nothing after the Write in progress, if any.
func (t *tap) abandon() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.abandoned {
		return
	}
	t.abandoned, t.detached, t.ended = true, true, true
	t.dropped += t.queued - t.inflight
	t.queue = nil
	t.signal()
}

// droppedBytes reports the bytes the writer has not received and will not.
func (t *tap) droppedBytes() uint64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.dropped
}

// signal wakes run, under t.mu.
func (t *tap) signal() {
	select {
	case t.wake <- struct{}{}:
	default:
	}
}

// run writes the queued copies in order, one Write each, ignoring the
// writer's error, which is the caller's to see; it returns once the stream
// has ended and the queue is empty, or the tap is abandoned.
func (t *tap) run() {
	defer close(t.done)
	for {
		t.mu.Lock()
		if t.abandoned || (t.ended && len(t.queue) == 0) {
			t.mu.Unlock()
			return
		}
		if len(t.queue) == 0 {
			t.mu.Unlock()
			<-t.wake
			continue
		}
		b := t.queue[0]
		t.queue[0] = nil
		t.queue = t.queue[1:]
		t.inflight = uint64(len(b))
		t.mu.Unlock()
		_, _ = t.w.Write(b) //nolint:errcheck // the writer is the caller's, and its error the caller's to see.
		t.mu.Lock()
		t.queued -= uint64(len(b))
		t.inflight = 0
		t.mu.Unlock()
	}
}

// tapReader is the agent's output as the connection reads it, with a copy of
// each read offered to t. Its Close closes the pipe, so that the
// connection's Close still ends a read (0027-MADR-agent-process-helper.md;
// 0030-MADR-record-initialize-on-arrival.md, the amendment of 2026-10-21, T2
// and T3). The pipe is any that closes, so that an agent of either kind has
// its taps (0042-MADR-in-process-agent-runner.md, item 1, A3).
type tapReader struct {
	f    io.ReadCloser
	t    *tap
	hook func() // Command.readHook
}

// Read reads the pipe, offers what it read, and returns the pipe's result.
func (r *tapReader) Read(p []byte) (int, error) {
	if r.hook != nil {
		r.hook()
	}
	n, err := r.f.Read(p)
	if n > 0 {
		r.t.offer(p[:n])
	}
	if err != nil {
		r.t.end()
	}
	return n, err
}

// Close ends the tap's stream and closes the pipe.
func (r *tapReader) Close() error {
	r.t.end()
	return r.f.Close()
}

// tapWriter is the agent's input as the connection writes it, with a copy of
// each write offered to t.
type tapWriter struct {
	w io.WriteCloser
	t *tap
}

// Write writes the agent's input, offers what was written, and returns the
// write's result.
func (w *tapWriter) Write(p []byte) (int, error) {
	n, err := w.w.Write(p)
	if n > 0 {
		w.t.offer(p[:n])
	}
	return n, err
}

// Close ends the tap's stream and closes the agent's input, once with
// Close's own close of it (a child's sharedClose).
func (w *tapWriter) Close() error {
	w.t.end()
	return w.w.Close()
}

// streams returns what the connection writes to and reads from: the agent's
// input and output as they are when no tap is set, or wrapped, each with its
// tap, which it starts; the output's with readHook.
func streams(stdin io.WriteCloser, outR io.ReadCloser, inputTap, outputTap io.Writer, readHook func()) (in io.Writer, out io.Reader, inTap, outTap *tap) {
	in, out = stdin, outR
	if outputTap != nil {
		outTap = newTap(outputTap)
		out = &tapReader{f: outR, t: outTap, hook: readHook}
	}
	if inputTap != nil {
		inTap = newTap(inputTap)
		in = &tapWriter{w: stdin, t: inTap}
	}
	return in, out, inTap, outTap
}
