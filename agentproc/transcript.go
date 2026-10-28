package agentproc

import (
	"encoding/json"
	"errors"
	"io"
	"sync"
)

// Transcript writes the frames of a connection's two streams to one writer,
// one JSON object a line, {"direction":…,"frame":…}: "sent" for a frame the
// client sent the agent, through Input, and "received" for one the agent
// sent, through Output. It is the form acptest.Record reads, so what a client
// records in use is what its tests read with acptest.ReadRecords, check with
// acptest.ValidateGolden and decode with acptest.Decode
// (0042-MADR-in-process-agent-runner.md, item 4).
//
// A frame that is JSON is written compact; a line that is not is written as
// a JSON string, so that the file stays one object a line. Its writes are
// serialized. The two streams have no order between them, as the taps'
// have none, so a response can be written before the request it answers. A
// redaction is the caller's: it wraps w. Input and Output are each for one
// stream, written from one goroutine, as a tap is; the two may be written at
// once.
type Transcript struct {
	in, out *lineTap // each recording one direction

	mu  sync.Mutex
	w   io.Writer
	err error // the first write's error; nothing is written after it
}

// NewTranscript returns a Transcript writing to w.
func NewTranscript(w io.Writer) *Transcript {
	t := &Transcript{w: w}
	t.in = &lineTap{fn: func(frame []byte) { t.write("sent", frame) }}
	t.out = &lineTap{fn: func(frame []byte) { t.write("received", frame) }}
	return t
}

// Input is the writer for Command.InputTap or InProcess.InputTap: what the
// client sends the agent, recorded "sent". Lines are joined across writes,
// as LineTap joins them.
func (t *Transcript) Input() io.Writer { return t.in }

// Output is the writer for Command.OutputTap or InProcess.OutputTap: what
// the agent sends the client, recorded "received".
func (t *Transcript) Output() io.Writer { return t.out }

// Close drops a line either stream has not ended, which is not a frame, and
// returns the first error a write to w returned. It does not close w.
func (t *Transcript) Close() error {
	_ = t.in.Close()  //nolint:errcheck // a LineTap's Close returns nil.
	_ = t.out.Close() //nolint:errcheck // a LineTap's Close returns nil.
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.err
}

// write writes one record of frame, the way dir names.
func (t *Transcript) write(dir string, frame []byte) {
	raw := json.RawMessage(frame)
	if !json.Valid(frame) {
		s, err := json.Marshal(string(frame))
		if err != nil {
			return
		}
		raw = s
	}
	line, err := json.Marshal(struct {
		Direction string          `json:"direction"`
		Frame     json.RawMessage `json:"frame"`
	}{dir, raw})
	if err != nil {
		return // raw is valid JSON, so it does not happen
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.err != nil {
		return
	}
	if _, err := t.w.Write(append(line, '\n')); err != nil {
		t.err = errors.Join(errors.New("agentproc: writing the transcript"), err)
	}
}
