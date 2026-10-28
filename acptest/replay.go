package acptest

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	acp "github.com/maccavelli/acp-go-sdk2"
)

// ReplayAgent plays the agent's side of a recorded session to a client under
// test, from records as ConnectRecorded or agentproc.Transcript records a
// client's streams: Sent frames the client sent, Received frames the agent
// sent. Its Serve is a serve function, for agentproc.InProcess or
// ConnectReplay (0042-MADR-in-process-agent-runner.md, item 4, question 4c).
//
// The Received records are played in their order:
//
//   - a notification, or a request of the agent's own, is sent when its turn
//     comes; the agent's request takes a fresh id, "replay-1" and on, and its
//     turn ends when the client answers it, an answer checked as JSON-RPC
//     only;
//   - a response waits for the client's request it stands for: the k-th
//     request of a method is answered with the recorded answer to the k-th
//     recorded request of that method, under the live request's id. Which
//     recorded request a response answered is read from the Sent records, by
//     id, so the records need both ways, but not an order between them.
//
// A client request beyond the recorded ones of its method is answered at once
// with an internal error, -32603, naming the method. Serve returns at its
// input's end: nil when every Received record was played and no request went
// unrecorded, and otherwise an error listing how the client departed from the
// recording. It returns context.Cause(ctx) when ctx ends first; its reader
// then ends when in does. Records it cannot play, a Received record that is
// not a JSON object or a response to an id no Sent record asks, make it
// return an error at once, before it reads or writes.
//
// A replay does not reproduce timing, nor the agent's choices: a client that
// branches on what it receives meets the recorded content whatever it asked.
type ReplayAgent struct {
	Records []Record
}

// replayStep is one Received record to play.
type replayStep struct {
	frame  map[string]json.RawMessage
	kind   string // "notify", "request" (the agent's own) or "response"
	method string // for a response: the method of the client request it answers
	nth    int    // for a response: which request of that method, from 0
}

// planReplay reads the steps of records, and how many recorded answers each
// method of the client's has.
func planReplay(records []Record) ([]replayStep, map[string]int, error) {
	type ask struct {
		method string
		nth    int
	}
	asked := map[string]ask{} // a recorded client request, by id
	count := map[string]int{}
	for _, r := range records {
		if r.Direction != Sent {
			continue
		}
		var m struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		if json.Unmarshal(r.Frame, &m) != nil || m.Method == "" || m.ID == nil {
			continue
		}
		asked[string(bytes.TrimSpace(m.ID))] = ask{m.Method, count[m.Method]}
		count[m.Method]++
	}
	var steps []replayStep
	answers := map[string]int{}
	for i, r := range records {
		if r.Direction != Received {
			continue
		}
		var frame map[string]json.RawMessage
		if err := json.Unmarshal(r.Frame, &frame); err != nil {
			return nil, nil, fmt.Errorf("acptest: replay: record %d is not a JSON object: %w", i, err)
		}
		_, hasID := frame["id"]
		_, hasMethod := frame["method"]
		switch {
		case hasMethod && hasID:
			steps = append(steps, replayStep{frame: frame, kind: "request"})
		case hasMethod:
			steps = append(steps, replayStep{frame: frame, kind: "notify"})
		case hasID:
			a, ok := asked[string(bytes.TrimSpace(frame["id"]))]
			if !ok {
				return nil, nil, fmt.Errorf("acptest: replay: record %d answers id %s, which no Sent record asks", i, frame["id"])
			}
			steps = append(steps, replayStep{frame: frame, kind: "response", method: a.method, nth: a.nth})
			answers[a.method]++
		default:
			return nil, nil, fmt.Errorf("acptest: replay: record %d is neither a request, a notification nor a response", i)
		}
	}
	return steps, answers, nil
}

// replay is one run of a ReplayAgent's Serve.
type replay struct {
	answers map[string]int // the recorded answers each client method has

	writeMu sync.Mutex
	out     io.Writer

	mu      sync.Mutex
	changed chan struct{}                // closed and replaced at each change below
	live    map[string][]json.RawMessage // the client's live request ids, by method, in order
	extra   []string                     // the methods of the requests beyond the recorded
	pending map[string]bool              // the agent's requests not yet answered, by fresh id
	bad     []string                     // the agent's requests answered not as JSON-RPC
	ended   bool                         // the input has ended
}

// Serve plays the records to the client at the other end of in and out.
func (r *ReplayAgent) Serve(ctx context.Context, in io.Reader, out io.Writer) error {
	steps, answers, err := planReplay(r.Records)
	if err != nil {
		return err
	}
	s := &replay{answers: answers, out: out, changed: make(chan struct{}),
		live: map[string][]json.RawMessage{}, pending: map[string]bool{}}
	read := make(chan struct{})
	go func() {
		defer close(read)
		s.read(in)
	}()
	played := s.play(ctx, steps)
	select {
	case <-read:
	case <-ctx.Done():
		return context.Cause(ctx)
	}
	return s.drift(steps[played:])
}

// play plays steps until they are done, the input ends or ctx does, and
// returns how many it played.
func (s *replay) play(ctx context.Context, steps []replayStep) int {
	fresh := 0
	for i, st := range steps {
		frame := st.frame
		switch st.kind {
		case "request":
			fresh++
			id := json.RawMessage(strconv.Quote("replay-" + strconv.Itoa(fresh)))
			s.mu.Lock()
			s.pending[string(id)] = true
			s.mu.Unlock()
			frame = withID(frame, id)
			if !s.write(frame) || !s.await(ctx, func() bool { return !s.pending[string(id)] }) {
				return i
			}
			continue
		case "response":
			var id json.RawMessage
			if !s.await(ctx, func() bool {
				if q := s.live[st.method]; len(q) > st.nth {
					id = q[st.nth]
					return true
				}
				return false
			}) {
				return i
			}
			frame = withID(frame, id)
		}
		if !s.write(frame) {
			return i
		}
	}
	return len(steps)
}

// await waits until done, called under mu, reports true; it reports false
// when the input ends or ctx does first.
func (s *replay) await(ctx context.Context, done func() bool) bool {
	for {
		s.mu.Lock()
		ok, ended, changed := done(), s.ended, s.changed
		s.mu.Unlock()
		switch {
		case ok:
			return true
		case ended:
			return false
		}
		select {
		case <-changed:
		case <-ctx.Done():
			return false
		}
	}
}

// signal wakes every await, under mu.
func (s *replay) signal() {
	close(s.changed)
	s.changed = make(chan struct{})
}

// read reads the client's frames until in ends: it queues each request by
// its method, answers one beyond the recorded at once, and marks the agent's
// requests answered.
func (s *replay) read(in io.Reader) {
	br := bufio.NewReader(in)
	for {
		line, err := br.ReadBytes('\n')
		if len(bytes.TrimSpace(line)) > 0 {
			s.take(line)
		}
		if err != nil {
			s.mu.Lock()
			s.ended = true
			s.signal()
			s.mu.Unlock()
			return
		}
	}
}

// take handles one frame of the client's.
func (s *replay) take(line []byte) {
	var m struct {
		ID     json.RawMessage `json:"id"`
		Method string          `json:"method"`
		Result json.RawMessage `json:"result"`
		Error  json.RawMessage `json:"error"`
	}
	if json.Unmarshal(line, &m) != nil {
		return
	}
	if m.Method != "" && m.ID != nil && s.unrecorded(m.Method, m.ID) {
		// Written outside mu, as play writes.
		s.write(map[string]json.RawMessage{"jsonrpc": json.RawMessage(`"2.0"`), "id": m.ID,
			"error": mustJSON(acp.NewInternalError(map[string]any{"error": "the replay has no recorded answer to " + m.Method}))})
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case m.Method != "" && m.ID != nil:
		s.live[m.Method] = append(s.live[m.Method], m.ID)
		s.signal()
	case m.Method == "" && m.ID != nil:
		id := string(bytes.TrimSpace(m.ID))
		if s.pending[id] {
			if m.Result == nil && m.Error == nil {
				s.bad = append(s.bad, id)
			}
			delete(s.pending, id)
			s.signal()
		}
	}
}

// unrecorded reports whether a request of method, id id, is beyond the
// recorded ones of its method, and records it so when it is. Only read
// calls it, so no other request of method comes between it and take.
func (s *replay) unrecorded(method string, id json.RawMessage) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.live[method]) < s.answers[method] {
		return false
	}
	s.extra = append(s.extra, method+" (id "+string(bytes.TrimSpace(id))+")")
	s.signal()
	return true
}

// write writes frame as one line, and reports whether it was written.
func (s *replay) write(frame map[string]json.RawMessage) bool {
	b, err := json.Marshal(frame)
	if err != nil {
		return false
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err = s.out.Write(append(b, '\n'))
	return err == nil
}

// drift is the error naming how the client departed from the recording, or
// nil when it did not.
func (s *replay) drift(unplayed []replayStep) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var parts []string
	if len(s.extra) > 0 {
		parts = append(parts, "requests the recording does not answer: "+strings.Join(s.extra, ", "))
	}
	if len(s.bad) > 0 {
		parts = append(parts, "answers to the agent's requests that are not JSON-RPC responses: "+strings.Join(s.bad, ", "))
	}
	if len(unplayed) > 0 {
		var names []string
		for _, st := range unplayed {
			name := st.method
			if name == "" {
				_ = json.Unmarshal(st.frame["method"], &name) //nolint:errcheck // a frame without a string method is named by its kind.
			}
			names = append(names, st.kind+" "+name)
		}
		parts = append(parts, fmt.Sprintf("%d recorded frames not played: %s", len(unplayed), strings.Join(names, ", ")))
	}
	if len(parts) == 0 {
		return nil
	}
	return errors.New("acptest: the client departed from the recording: " + strings.Join(parts, "; "))
}

// withID is frame with its id replaced by id.
func withID(frame map[string]json.RawMessage, id json.RawMessage) map[string]json.RawMessage {
	out := maps.Clone(frame)
	out["id"] = id
	return out
}

// mustJSON is v encoded, which cannot fail for a *acp.RequestError.
func mustJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

// ConnectReplay joins client to a ReplayAgent of records over two pipes, and
// returns the client's side. When the test ends it closes the connection,
// which ends the replay's input, and fails the test if the replay reports
// that the client departed from the recording, or does not return within
// 10 seconds (0042-MADR-in-process-agent-runner.md, item 4).
func ConnectReplay(tb testing.TB, records []Record, client acp.Client, opts ...acp.ConnectionOption) *acp.ClientSideConnection {
	tb.Helper()
	agentReads, clientWrites := io.Pipe()
	clientReads, agentWrites := io.Pipe()
	served := make(chan error, 1)
	go func() {
		err := (&ReplayAgent{Records: records}).Serve(context.Background(), agentReads, agentWrites)
		_ = agentWrites.Close() //nolint:errcheck // a pipe's Close returns nil.
		_ = agentReads.Close()  //nolint:errcheck // a pipe's Close returns nil.
		served <- err
	}()
	c := acp.NewClientSideConnection(client, clientWrites, clientReads, opts...)
	tb.Cleanup(func() {
		_ = c.Close() //nolint:errcheck // the replay's end reports what matters.
		select {
		case err := <-served:
			if err != nil {
				tb.Errorf("%v", err)
			}
		case <-time.After(cleanupWait):
			tb.Errorf("acptest: the replay did not end within %v of the connection's close", cleanupWait)
		}
	})
	return c
}
