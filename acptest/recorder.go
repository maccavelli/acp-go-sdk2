package acptest

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"sync"
)

// Recorder records the frames that pass through the readers and writers it
// wraps: each newline-terminated line, in the order the lines end. A frame
// that is JSON is kept in canonical form, as encoding/json v1 writes the
// value it decodes to: keys sorted, HTML characters escaped, numbers as they
// were. Any other line is kept as it was. A line not yet ended is not a
// frame. The zero Recorder is ready; it is safe for concurrent use.
type Recorder struct {
	mu     sync.Mutex
	frames []json.RawMessage
}

// Writer returns a writer that records what it is given and then writes it
// to w, so frames are recorded in the order they are sent, a frame before
// the peer's answer to it; a frame whose write fails is recorded too.
// Closing it closes w when w is an io.Closer, so a connection's Close still
// ends what it writes to.
func (r *Recorder) Writer(w io.Writer) io.WriteCloser { return &recordingWriter{r: r, w: w} }

// Reader returns a reader that reads from rd and records what it reads.
// Closing it closes rd when rd is an io.Closer, so a connection's Close still
// ends a read in progress.
func (r *Recorder) Reader(rd io.Reader) io.ReadCloser { return &recordingReader{r: r, rd: rd} }

// Frames returns a copy of the frames recorded so far, each its own copy.
func (r *Recorder) Frames() []json.RawMessage {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]json.RawMessage, len(r.frames))
	for i, f := range r.frames {
		out[i] = bytes.Clone(f)
	}
	return out
}

// add records each line of b that ends a line started in *partial.
func (r *Recorder) add(partial *[]byte, b []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	*partial = append(*partial, b...)
	for {
		i := bytes.IndexByte(*partial, '\n')
		if i < 0 {
			return
		}
		r.frames = append(r.frames, canonical((*partial)[:i]))
		*partial = append((*partial)[:0], (*partial)[i+1:]...)
	}
}

// canonical is line re-encoded by encoding/json v1 when it is one JSON value,
// and a copy of line otherwise.
func canonical(line []byte) json.RawMessage {
	d := json.NewDecoder(bytes.NewReader(line))
	d.UseNumber()
	var v any
	if err := d.Decode(&v); err == nil {
		// One value, and nothing after it.
		if _, err := d.Token(); errors.Is(err, io.EOF) {
			if b, err := json.Marshal(v); err == nil {
				return b
			}
		}
	}
	return bytes.Clone(line)
}

type recordingWriter struct {
	r       *Recorder
	w       io.Writer
	partial []byte
}

// Write records b, then writes it. Recording first keeps the frames in the
// order they were sent: a write to a pipe returns once the peer has read
// it, and by then the peer may have answered and its writer recorded the
// answer. A frame is so recorded even if the write then fails.
func (w *recordingWriter) Write(b []byte) (int, error) {
	w.r.add(&w.partial, b)
	return w.w.Write(b)
}

func (w *recordingWriter) Close() error {
	if c, ok := w.w.(io.Closer); ok {
		return c.Close()
	}
	return nil
}

type recordingReader struct {
	r       *Recorder
	rd      io.Reader
	partial []byte
}

func (r *recordingReader) Read(b []byte) (int, error) {
	n, err := r.rd.Read(b)
	r.r.add(&r.partial, b[:n])
	return n, err
}

func (r *recordingReader) Close() error {
	if c, ok := r.rd.(io.Closer); ok {
		return c.Close()
	}
	return nil
}
