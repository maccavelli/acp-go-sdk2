package acp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
	"unicode/utf8"
)

// streamClient records the tool-call updates a client receives, in order,
// and applies them to a ToolCallTimeline.
type streamClient struct {
	mu       sync.Mutex
	updates  []SessionUpdate
	timeline ToolCallTimeline
}

func (r *streamClient) handle(_ context.Context, n SessionNotification) error {
	if n.Update.ToolCall == nil && n.Update.ToolCallUpdate == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.updates = append(r.updates, n.Update)
	r.timeline.Apply(n.Update)
	return nil
}

// entity is the client's state of call id.
func (r *streamClient) entity(id ToolCallId) (ToolCallEntity, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.timeline.Get(id)
}

// count is how many tool_call_update notifications the client has received.
func (r *streamClient) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, u := range r.updates {
		if u.ToolCallUpdate != nil {
			n++
		}
	}
	return n
}

// streamPair connects an agent built from a, with agentOpts, and a client
// that records into rec, over pipes, both sides negotiated with everything
// advertised.
func streamPair(t *testing.T, a agentFuncs, rec *streamClient, agentOpts ...ConnectionOption) (*AgentSideConnection, *ClientSideConnection) {
	t.Helper()
	c2aR, c2aW := io.Pipe()
	a2cR, a2cW := io.Pipe()
	t.Cleanup(func() {
		_ = c2aW.Close()
		_ = a2cW.Close()
	})
	asc := NewAgentSideConnection(a, a2cW, c2aR, agentOpts...)
	csc := NewClientSideConnection(&clientFuncs{SessionUpdateFunc: rec.handle}, c2aW, a2cR)
	negotiate(t, asc, allAdvertised())
	negotiate(t, csc, allAdvertised())
	return asc, csc
}

// holdUpdates is a sending middleware that holds every tool_call_update until
// hold is closed, signalling entered as one waits, and counts them in sent.
func holdUpdates(hold <-chan struct{}, entered chan<- struct{}, sent *atomic.Int32) Middleware {
	return func(next MethodHandler) MethodHandler {
		return func(ctx context.Context, method string, params json.RawMessage) (any, *RequestError) {
			if method == ClientMethodSessionUpdate && bytes.Contains(params, []byte(`"sessionUpdate":"tool_call_update"`)) {
				sent.Add(1)
				select {
				case entered <- struct{}{}:
				default:
				}
				<-hold
			}
			return next(ctx, method, params)
		}
	}
}

// normalized is e with empty collections as nil, so a state the agent holds
// and one a client rebuilt compare equal, as JSON.
func normalized(t *testing.T, e ToolCallEntity) string {
	t.Helper()
	if len(e.Content) == 0 {
		e.Content = nil
	}
	if len(e.Locations) == 0 {
		e.Locations = nil
	}
	b, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// waitUntil waits, in real time, up to 10 seconds for cond.
func waitUntil(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("%s did not happen", what)
		}
		time.Sleep(time.Millisecond)
	}
}

// textOf is the text of e's last content item, or "".
func textOf(e ToolCallEntity) string {
	if len(e.Content) == 0 {
		return ""
	}
	last := e.Content[len(e.Content)-1]
	if last.Content == nil || last.Content.Content.Text == nil {
		return ""
	}
	return last.Content.Content.Text.Text
}

func startStream(t *testing.T, asc *AgentSideConnection, id ToolCallId, opts ...ToolCallStreamOption) *ToolCallStream {
	t.Helper()
	s, err := StartToolCallStream(t.Context(), asc, "s1", StartToolCall(id, "Run", WithStartKind(ToolKindExecute),
		WithStartStatus(ToolCallStatusPending), WithStartRawInput(json.RawMessage(`{"cmd":"ls"}`))), opts...)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// TestToolCallStreamRoundTrip requires the state a client's ToolCallTimeline
// rebuilds from a stream's updates to be the stream's own, once finished
// (0052-MADR-agent-tool-call-snapshot-stream-and-message-size-default.md,
// Confirmation, round trip).
func TestToolCallStreamRoundTrip(t *testing.T) {
	t.Parallel()
	rec := &streamClient{}
	asc, _ := streamPair(t, agentFuncs{}, rec)
	s := startStream(t, asc, "c1", WithToolCallStreamInterval(time.Millisecond))
	steps := []func() error{
		func() error { return s.SetStatus(ToolCallStatusInProgress) },
		func() error { _, err := s.WriteString("line 1\n"); return err },
		func() error {
			return s.Update(func(e *ToolCallEntity) {
				e.Title = "Run ls"
				e.Locations = []ToolCallLocation{{Path: absPath("x")}}
			})
		},
		func() error { return s.Flush(t.Context()) },
		func() error { _, err := s.Write([]byte("line 2 é\n")); return err },
		func() error {
			return s.Update(func(e *ToolCallEntity) {
				e.Content = []ToolCallContent{ToolContent(TextBlock("summary"))}
				e.RawOutput = json.RawMessage(`{"ok":true}`)
			})
		},
		func() error { return s.Finish(t.Context(), ToolCallStatusCompleted) },
	}
	for i, step := range steps {
		if err := step(); err != nil {
			t.Fatalf("step %d: %v", i, err)
		}
	}
	waitUntil(t, "the final update", func() bool {
		e, _ := rec.entity("c1")
		return e.Status == ToolCallStatusCompleted
	})
	got, _ := rec.entity("c1")
	if g, w := normalized(t, got), normalized(t, s.Entity()); g != w {
		t.Errorf("the client's state is\n%s\nwant the stream's\n%s", g, w)
	}
	if textOf(got) != "line 1\nline 2 é\n" {
		t.Errorf("the client's live text is %q", textOf(got))
	}
}

// absPath is an absolute path on this host.
func absPath(name string) string {
	return string(os.PathSeparator) + name
}

// FuzzToolCallStream drives a stream with a sequence of changes read from the
// input and requires the round trip of TestToolCallStreamRoundTrip.
func FuzzToolCallStream(f *testing.F) {
	f.Add([]byte{0, 1, 2, 3, 4})
	f.Add([]byte{0, 0, 0, 3, 0, 3, 2, 2, 4})
	f.Add([]byte{4, 4, 1, 1, 0xf8, 0x09})
	f.Fuzz(func(t *testing.T, ops []byte) {
		if len(ops) > 64 {
			ops = ops[:64]
		}
		rec := &streamClient{}
		asc, _ := streamPair(t, agentFuncs{}, rec)
		s := startStream(t, asc, "f1", WithToolCallStreamInterval(time.Millisecond), WithToolCallStreamMaxText(64))
		for i, op := range ops {
			var err error
			switch op % 5 {
			case 0:
				_, err = s.WriteString(strings.Repeat(string(rune('a'+i%26)), int(op>>3)) + "€")
			case 1:
				err = s.SetStatus([]ToolCallStatus{ToolCallStatusPending, ToolCallStatusInProgress}[i%2])
			case 2:
				err = s.Update(func(e *ToolCallEntity) { e.Title = fmt.Sprint("t", i) })
			case 3:
				err = s.Flush(t.Context())
			case 4:
				err = s.Update(func(e *ToolCallEntity) { e.Locations = []ToolCallLocation{{Path: absPath(strconv.Itoa(i))}} })
			}
			if err != nil {
				t.Fatalf("op %d (%d): %v", i, op, err)
			}
		}
		if err := s.Finish(t.Context(), ToolCallStatusFailed); err != nil {
			t.Fatal(err)
		}
		waitUntil(t, "the final update", func() bool {
			e, _ := rec.entity("f1")
			return e.Status == ToolCallStatusFailed
		})
		got, _ := rec.entity("f1")
		if g, w := normalized(t, got), normalized(t, s.Entity()); g != w {
			t.Errorf("ops %v: the client's state is\n%s\nwant\n%s", ops, g, w)
		}
	})
}

// TestToolCallStreamHelperProcess is not a test: TestToolCallStreamExecWriter
// runs the test binary with it as a command that prints three lines.
func TestToolCallStreamHelperProcess(t *testing.T) {
	if os.Getenv("ACP_TOOLSTREAM_HELPER") != "1" {
		t.Skip("a helper process for TestToolCallStreamExecWriter")
	}
	fmt.Print("one\ntwo\nthree\n")
	os.Exit(0)
}

// TestToolCallStreamExecWriter requires a stream to take a command's output
// as its Stdout, the io.Writer it is
// (0052-MADR-agent-tool-call-snapshot-stream-and-message-size-default.md, D1).
func TestToolCallStreamExecWriter(t *testing.T) {
	t.Parallel()
	rec := &streamClient{}
	asc, _ := streamPair(t, agentFuncs{}, rec)
	s := startStream(t, asc, "x1")
	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestToolCallStreamHelperProcess$")
	cmd.Env = append(os.Environ(), "ACP_TOOLSTREAM_HELPER=1")
	cmd.Stdout = s
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	if err := s.Finish(t.Context(), ToolCallStatusCompleted); err != nil {
		t.Fatal(err)
	}
	waitUntil(t, "the final update", func() bool {
		e, _ := rec.entity("x1")
		return e.Status == ToolCallStatusCompleted
	})
	if got, _ := rec.entity("x1"); textOf(got) != "one\ntwo\nthree\n" {
		t.Errorf("the client's text is %q, want the command's three lines", textOf(got))
	}
}

// TestToolCallStreamCoalesces requires changes made while a write is held by
// back-pressure to be written together, the latest state, once, and not
// queued: one held write, then at most one more before Finish's
// (0052-MADR-agent-tool-call-snapshot-stream-and-message-size-default.md, D2).
func TestToolCallStreamCoalesces(t *testing.T) {
	t.Parallel()
	hold, entered := make(chan struct{}), make(chan struct{}, 1)
	var sent atomic.Int32
	rec := &streamClient{}
	asc, _ := streamPair(t, agentFuncs{}, rec, WithSendingMiddleware(holdUpdates(hold, entered, &sent)))
	s := startStream(t, asc, "k1", WithToolCallStreamInterval(time.Millisecond))
	_, _ = s.WriteString("a")
	select {
	case <-entered:
	case <-time.After(10 * time.Second):
		t.Fatal("the first write did not reach the middleware")
	}
	for range 1000 {
		_, _ = s.WriteString("x")
	}
	close(hold)
	if err := s.Flush(t.Context()); err != nil {
		t.Fatal(err)
	}
	if n := sent.Load(); n > 2 {
		t.Errorf("%d writes for 1,001 changes held by back-pressure, want at most 2", n)
	}
	waitUntil(t, "the coalesced text", func() bool {
		e, _ := rec.entity("k1")
		return textOf(e) == "a"+strings.Repeat("x", 1000)
	})
}

// TestToolCallStreamInterval requires changes within one interval to be
// written at most twice, the first at once after a quiet interval and the
// rest together at the interval's end
// (0052-MADR-agent-tool-call-snapshot-stream-and-message-size-default.md, D2).
func TestToolCallStreamInterval(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		var mu sync.Mutex
		var at []time.Time
		count := func(next MethodHandler) MethodHandler {
			return func(ctx context.Context, method string, params json.RawMessage) (any, *RequestError) {
				if bytes.Contains(params, []byte(`"sessionUpdate":"tool_call_update"`)) {
					mu.Lock()
					at = append(at, time.Now())
					mu.Unlock()
				}
				return next(ctx, method, params)
			}
		}
		rec := &streamClient{}
		asc, _ := streamPair(t, agentFuncs{}, rec, WithSendingMiddleware(count))
		s := startStream(t, asc, "i1")
		time.Sleep(time.Second) // a quiet interval
		burst := time.Now()
		for range 100 {
			_, _ = s.WriteString("x")
			time.Sleep(time.Millisecond)
		}
		time.Sleep(time.Second)
		synctest.Wait()
		mu.Lock()
		defer mu.Unlock()
		if len(at) != 2 {
			t.Fatalf("%d writes for a burst within one interval, want 2", len(at))
		}
		if !at[0].Equal(burst) {
			t.Errorf("the first write came %v after the quiet interval, want at once", at[0].Sub(burst))
		}
		if d := at[1].Sub(at[0]); d != defaultToolCallStreamInterval {
			t.Errorf("the second write came %v after the first, want the interval, %v", d, defaultToolCallStreamInterval)
		}
	})
}

// runningNow reports whether s has a sender.
func (s *ToolCallStream) runningNow() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// TestToolCallStreamIdle requires a stream with nothing pending to hold no
// sender once its last write is done, and a later change to start one
// (0052-MADR-agent-tool-call-snapshot-stream-and-message-size-default.md, D2).
func TestToolCallStreamIdle(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		rec := &streamClient{}
		asc, _ := streamPair(t, agentFuncs{}, rec)
		s := startStream(t, asc, "d1")
		_, _ = s.WriteString("one")
		if !s.runningNow() {
			t.Error("no sender after a change")
		}
		time.Sleep(time.Second)
		synctest.Wait()
		if s.runningNow() {
			t.Error("a sender still runs with nothing pending")
		}
		_, _ = s.WriteString(" two")
		if !s.runningNow() {
			t.Error("no sender after a change made while idle")
		}
		time.Sleep(time.Second)
		synctest.Wait()
		if e, _ := rec.entity("d1"); textOf(e) != "one two" || s.runningNow() {
			t.Errorf("after the second change: text %q, sender running %v; want %q, false", textOf(e), s.runningNow(), "one two")
		}
	})
}

// TestToolCallStreamNoLostChange requires a change made just after the
// sender's write, as it looks for more, to be written: the sender clears its
// mark under the lock it checks pending under
// (0052-MADR-agent-tool-call-snapshot-stream-and-message-size-default.md, D2).
func TestToolCallStreamNoLostChange(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		rec := &streamClient{}
		asc, _ := streamPair(t, agentFuncs{}, rec)
		s := startStream(t, asc, "n1")
		want := ""
		for round := range 3 {
			time.Sleep(time.Second) // quiet: the next change is written at once
			b := fmt.Sprint("b", round)
			_, _ = s.WriteString(b)
			time.Sleep(500 * time.Microsecond) // just after that write
			c := fmt.Sprint("c", round)
			_, _ = s.WriteString(c)
			want += b + c
			time.Sleep(time.Second)
			synctest.Wait()
			if e, _ := rec.entity("n1"); textOf(e) != want {
				t.Fatalf("round %d: the client's text is %q, want %q: a change was lost", round, textOf(e), want)
			}
		}
	})
}

// TestToolCallStreamFinishAfterCancel requires Finish on the context of an
// agent's Prompt, after session/cancel cancelled it, to write the final
// update, which the client receives before the cancelled answer
// (0052-MADR-agent-tool-call-snapshot-stream-and-message-size-default.md, D1).
func TestToolCallStreamFinishAfterCancel(t *testing.T) {
	t.Parallel()
	started, finished := make(chan struct{}), make(chan error, 1)
	rec := &streamClient{}
	var asc *AgentSideConnection
	a := agentFuncs{PromptFunc: func(ctx context.Context, p PromptRequest) (PromptResponse, error) {
		s, err := StartToolCallStream(ctx, asc, p.SessionId, StartToolCall("q1", "Run", WithStartStatus(ToolCallStatusInProgress)))
		if err != nil {
			finished <- err
			return PromptResponse{}, err
		}
		_, _ = s.WriteString("partial")
		close(started)
		<-ctx.Done()
		finished <- s.Finish(ctx, ToolCallStatusFailed)
		return PromptResponse{}, ctx.Err()
	}}
	asc, csc := streamPair(t, a, rec)
	answered := prompt(t, csc, "s1")
	select {
	case <-started:
	case <-time.After(10 * time.Second):
		t.Fatal("the prompt did not start its stream")
	}
	if err := csc.Cancel(t.Context(), CancelNotification{SessionId: "s1"}); err != nil {
		t.Fatal(err)
	}
	resp := within(t, answered, "the cancelled answer")
	if err := within(t, finished, "Finish's result"); err != nil {
		t.Fatalf("Finish after session/cancel: %v", err)
	}
	if resp.StopReason != StopReasonCancelled {
		t.Errorf("stop reason %q, want cancelled", resp.StopReason)
	}
	if e, _ := rec.entity("q1"); e.Status != ToolCallStatusFailed || textOf(e) != "partial" {
		t.Errorf("when the answer arrived the client had status %q and text %q, want failed and %q", e.Status, textOf(e), "partial")
	}
}

// TestToolCallStreamFinishDeadline requires Finish to honour its context's
// deadline, to leave the stream unfinished when it does, and to write when
// called again
// (0052-MADR-agent-tool-call-snapshot-stream-and-message-size-default.md, D1).
func TestToolCallStreamFinishDeadline(t *testing.T) {
	t.Parallel()
	rec := &streamClient{}
	asc, _ := streamPair(t, agentFuncs{}, rec)
	s := startStream(t, asc, "e1")
	s.slot <- struct{}{} // a write in flight, held
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- s.Finish(ctx, ToolCallStatusCompleted) }()
	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("Finish past its deadline = %v, want context.DeadlineExceeded", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Finish ignored its deadline")
	}
	<-s.slot
	if e := s.Entity(); e.Status != ToolCallStatusPending {
		t.Errorf("after a failed Finish the status is %q, want pending", e.Status)
	}
	if err := s.Finish(t.Context(), ToolCallStatusCompleted); err != nil {
		t.Fatalf("Finish again: %v", err)
	}
	waitUntil(t, "the final update", func() bool {
		e, _ := rec.entity("e1")
		return e.Status == ToolCallStatusCompleted
	})
}

// TestWrapUp requires the wrap-up context to stay live when its parent ends
// for the prompt turn's own end, and to end with the parent's cause
// otherwise (0052-MADR-agent-tool-call-snapshot-stream-and-message-size-default.md,
// D1).
func TestWrapUp(t *testing.T) {
	t.Parallel()
	errOther := errors.New("other")
	for _, tc := range []struct {
		cause error
		live  bool
	}{
		{ErrPromptCancelled, true},
		{errSuperseded, true},
		{ErrShutdown, true},
		{context.Canceled, false},
		{errOther, false},
	} {
		for _, before := range []bool{false, true} {
			synctest.Test(t, func(t *testing.T) {
				parent, cancel := context.WithCancelCause(context.Background())
				if before {
					cancel(tc.cause)
				}
				out, release := wrapUp(parent)
				defer release()
				cancel(tc.cause)
				synctest.Wait()
				if live := out.Err() == nil; live != tc.live {
					t.Errorf("cause %v, cancelled before %v: live %v, want %v", tc.cause, before, live, tc.live)
				}
				if !tc.live && !errors.Is(context.Cause(out), tc.cause) {
					t.Errorf("cause %v: the wrap-up context ended with %v", tc.cause, context.Cause(out))
				}
			})
		}
	}
	synctest.Test(t, func(t *testing.T) {
		parent, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		out, release := wrapUp(parent)
		defer release()
		time.Sleep(2 * time.Second)
		synctest.Wait()
		if !errors.Is(context.Cause(out), context.DeadlineExceeded) {
			t.Errorf("past the parent's deadline the wrap-up context's cause is %v, want context.DeadlineExceeded", context.Cause(out))
		}
	})
}

// TestToolCallStreamOwnedFields requires Update to undo a change of Id or
// Status, keep its other changes and return ErrToolCallStreamField naming the
// field, and SetStatus to refuse a final status
// (0052-MADR-agent-tool-call-snapshot-stream-and-message-size-default.md, D1).
func TestToolCallStreamOwnedFields(t *testing.T) {
	t.Parallel()
	rec := &streamClient{}
	asc, _ := streamPair(t, agentFuncs{}, rec)
	s := startStream(t, asc, "o1")
	err := s.Update(func(e *ToolCallEntity) {
		e.Id = "other"
		e.Title = "kept"
	})
	if !errors.Is(err, ErrToolCallStreamField) || !strings.Contains(err.Error(), "Id") {
		t.Errorf("Update changing Id = %v, want ErrToolCallStreamField naming Id", err)
	}
	err = s.Update(func(e *ToolCallEntity) { e.Status = ToolCallStatusCompleted })
	if !errors.Is(err, ErrToolCallStreamField) || !strings.Contains(err.Error(), "Status") {
		t.Errorf("Update changing Status = %v, want ErrToolCallStreamField naming Status", err)
	}
	if e := s.Entity(); e.Id != "o1" || e.Status != ToolCallStatusPending || e.Title != "kept" {
		t.Errorf("after the refused changes: id %q, status %q, title %q; want o1, pending, kept", e.Id, e.Status, e.Title)
	}
	if err := s.SetStatus(ToolCallStatusCompleted); !errors.Is(err, ErrToolCallStreamField) {
		t.Errorf("SetStatus(completed) = %v, want ErrToolCallStreamField", err)
	}
}

// TestToolCallStreamBound requires the live text past its bound to keep a
// valid UTF-8 tail of at most the bound, Dropped to count what was cut, and a
// write that ends inside a character to be held back until it completes
// (0052-MADR-agent-tool-call-snapshot-stream-and-message-size-default.md, D4).
func TestToolCallStreamBound(t *testing.T) {
	t.Parallel()
	rec := &streamClient{}
	asc, _ := streamPair(t, agentFuncs{}, rec)
	s := startStream(t, asc, "b1", WithToolCallStreamMaxText(101))
	written := 0
	for range 1000 {
		n, _ := s.WriteString("😀")
		written += n
	}
	text := textOf(s.Entity())
	if !utf8.ValidString(text) || len(text) > 101 || len(text) < 101-(utf8.UTFMax-1) {
		t.Errorf("kept %d bytes, valid %v; want valid UTF-8 of at most 101 and at least %d", len(text), utf8.ValidString(text), 101-(utf8.UTFMax-1))
	}
	if d := s.Dropped(); d != int64(written-len(text)) {
		t.Errorf("Dropped() = %d, want %d", d, written-len(text))
	}
	euro := []byte("€")
	_, _ = s.Write(euro[:2])
	if text := textOf(s.Entity()); !utf8.ValidString(text) || strings.HasSuffix(text, "€") {
		t.Errorf("a character not finished: kept text valid %v, ends with it %v; want it held back", utf8.ValidString(text),
			strings.HasSuffix(text, "€"))
	}
	_, _ = s.Write(euro[2:])
	if text := textOf(s.Entity()); !strings.HasSuffix(text, "€") {
		t.Error("a character completed by a later write is not in the text")
	}
}

// TestToolCallStreamConcurrent requires 16 streams, each written by 4
// goroutines at once, to deliver every byte of each
// (0052-MADR-agent-tool-call-snapshot-stream-and-message-size-default.md, D2).
func TestToolCallStreamConcurrent(t *testing.T) {
	t.Parallel()
	rec := &streamClient{}
	asc, _ := streamPair(t, agentFuncs{}, rec)
	var streams sync.WaitGroup
	for i := range 16 {
		id := ToolCallId(fmt.Sprint("p", i))
		s := startStream(t, asc, id, WithToolCallStreamInterval(time.Millisecond))
		streams.Go(func() {
			var writers sync.WaitGroup
			for range 4 {
				writers.Go(func() {
					for range 100 {
						_, _ = s.WriteString("0123456789")
					}
				})
			}
			writers.Wait()
			if err := s.Finish(t.Context(), ToolCallStatusCompleted); err != nil {
				t.Errorf("%s: %v", id, err)
			}
		})
	}
	streams.Wait()
	for i := range 16 {
		id := ToolCallId(fmt.Sprint("p", i))
		waitUntil(t, string(id)+"'s final update", func() bool {
			e, _ := rec.entity(id)
			return e.Status == ToolCallStatusCompleted
		})
		if e, _ := rec.entity(id); len(textOf(e)) != 4000 {
			t.Errorf("%s: the client has %d bytes of text, want 4,000", id, len(textOf(e)))
		}
	}
}

// TestToolCallStreamLifetime requires Wait to return after Close with streams
// open and their senders waiting, no sender to be left, and a later change to
// leave ErrConnectionClosed in Err
// (0052-MADR-agent-tool-call-snapshot-stream-and-message-size-default.md, D2).
func TestToolCallStreamLifetime(t *testing.T) {
	t.Parallel()
	rec := &streamClient{}
	asc, _ := streamPair(t, agentFuncs{}, rec)
	var streams []*ToolCallStream
	for i := range 4 {
		s := startStream(t, asc, ToolCallId(fmt.Sprint("l", i)), WithToolCallStreamInterval(time.Hour))
		_, _ = s.WriteString("waiting an hour")
		streams = append(streams, s)
	}
	_ = asc.Connection().Close()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	if err := asc.Connection().WaitContext(ctx); errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("Wait did not return after Close: a sender outlived its connection")
	}
	for i, s := range streams {
		if s.runningNow() {
			t.Errorf("stream %d: a sender still runs after Wait", i)
		}
	}
	_, _ = streams[0].WriteString("after the end")
	if err := streams[0].Err(); !errors.Is(err, ErrConnectionClosed) {
		t.Errorf("Err after a change on an ended connection = %v, want ErrConnectionClosed", err)
	}
}

// TestToolCallStreamRefusals requires the stream's refusals
// (0052-MADR-agent-tool-call-snapshot-stream-and-message-size-default.md, D1).
func TestToolCallStreamRefusals(t *testing.T) {
	t.Parallel()
	rec := &streamClient{}
	asc, _ := streamPair(t, agentFuncs{}, rec)
	var verr *ValidationError
	if _, err := StartToolCallStream(t.Context(), asc, "s1", UpdateToolCall("u1")); !errors.As(err, &verr) {
		t.Errorf("a start that is not a tool_call = %v, want a *ValidationError", err)
	}
	c2aR, _ := io.Pipe()
	bare := NewAgentSideConnection(agentFuncs{}, io.Discard, c2aR)
	t.Cleanup(func() { _ = bare.Connection().Close() })
	if _, err := StartToolCallStream(t.Context(), bare, "s1", StartToolCall("u2", "Run")); !errors.Is(err, ErrNotInitialized) {
		t.Errorf("a start before initialize = %v, want ErrNotInitialized", err)
	}
	s := startStream(t, asc, "r1")
	if err := s.Finish(t.Context(), ToolCallStatusPending); !errors.Is(err, ErrToolCallStreamField) {
		t.Errorf("Finish(pending) = %v, want ErrToolCallStreamField", err)
	}
	if err := s.Finish(t.Context(), ToolCallStatusCompleted); err != nil {
		t.Fatal(err)
	}
	for name, call := range map[string]func() error{
		"Finish":    func() error { return s.Finish(t.Context(), ToolCallStatusFailed) },
		"Flush":     func() error { return s.Flush(t.Context()) },
		"Write":     func() error { _, err := s.WriteString("late"); return err },
		"SetStatus": func() error { return s.SetStatus(ToolCallStatusInProgress) },
		"Update":    func() error { return s.Update(func(e *ToolCallEntity) { e.Title = "late" }) },
	} {
		if err := call(); !errors.Is(err, ErrToolCallStreamFinished) {
			t.Errorf("%s after Finish = %v, want ErrToolCallStreamFinished", name, err)
		}
	}
}

// TestToolCallStreamSendError requires a refused state to be reported: by
// Flush, which wrote it, and by Err for a background write; and a later
// change that corrects it to be written
// (0052-MADR-agent-tool-call-snapshot-stream-and-message-size-default.md, D1).
func TestToolCallStreamSendError(t *testing.T) {
	t.Parallel()
	rec := &streamClient{}
	asc, _ := streamPair(t, agentFuncs{}, rec)
	s := startStream(t, asc, "v1", WithToolCallStreamInterval(time.Millisecond))
	relative := func(e *ToolCallEntity) { e.Locations = []ToolCallLocation{{Path: "relative/path"}} }
	if err := s.Update(relative); err != nil {
		t.Fatal(err)
	}
	var verr *ValidationError
	if err := s.Flush(t.Context()); err != nil && !errors.As(err, &verr) {
		t.Errorf("Flush of a relative location = %v, want a *ValidationError", err)
	}
	if err := s.Update(func(e *ToolCallEntity) { e.Title = "again"; relative(e) }); err != nil {
		t.Fatal(err)
	}
	waitUntil(t, "the background write's error", func() bool { return errors.As(s.Err(), &verr) })
	if err := s.Update(func(e *ToolCallEntity) { e.Locations = []ToolCallLocation{{Path: absPath("ok")}} }); err != nil {
		t.Fatal(err)
	}
	if err := s.Flush(t.Context()); err != nil {
		t.Fatalf("Flush of a corrected location: %v", err)
	}
	waitUntil(t, "the corrected location", func() bool {
		e, _ := rec.entity("v1")
		return len(e.Locations) == 1 && e.Locations[0].Path == absPath("ok")
	})
}

// TestToolCallStreamMiddleware requires every snapshot to pass the sending
// middleware as a session/update, as any notification does
// (0052-MADR-agent-tool-call-snapshot-stream-and-message-size-default.md, D1).
func TestToolCallStreamMiddleware(t *testing.T) {
	t.Parallel()
	hold := make(chan struct{})
	close(hold)
	var sent atomic.Int32
	rec := &streamClient{}
	asc, _ := streamPair(t, agentFuncs{}, rec, WithSendingMiddleware(holdUpdates(hold, make(chan struct{}, 1), &sent)))
	s := startStream(t, asc, "m1")
	for i := range 3 {
		_, _ = s.WriteString(strconv.Itoa(i))
		if err := s.Flush(t.Context()); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Finish(t.Context(), ToolCallStatusCompleted); err != nil {
		t.Fatal(err)
	}
	waitUntil(t, "the final update", func() bool { return rec.count() == int(sent.Load()) && rec.count() >= 4 })
	if n := sent.Load(); n != 4 {
		t.Errorf("the middleware saw %d tool_call_update sends, want 4: three flushes and Finish", n)
	}
}

// holdCall is a sending middleware that holds every tool_call_update whose
// params name the call id until hold is closed, signalling entered as one
// waits.
func holdCall(id string, hold <-chan struct{}, entered chan<- struct{}) Middleware {
	needle := []byte(`"toolCallId":"` + id + `"`)
	return func(next MethodHandler) MethodHandler {
		return func(ctx context.Context, method string, params json.RawMessage) (any, *RequestError) {
			if method == ClientMethodSessionUpdate && bytes.Contains(params, []byte(`"sessionUpdate":"tool_call_update"`)) &&
				bytes.Contains(params, needle) {
				select {
				case entered <- struct{}{}:
				default:
				}
				<-hold
			}
			return next(ctx, method, params)
		}
	}
}

// TestToolCallStreamFlushedBeforeAnswer requires a stream that Prompt left
// unfinished, whose own sender waits an hour, to be written before the turn
// is answered, its status as it was
// (0052-MADR-agent-tool-call-snapshot-stream-and-message-size-default.md, D3).
func TestToolCallStreamFlushedBeforeAnswer(t *testing.T) {
	t.Parallel()
	rec := &streamClient{}
	var asc *AgentSideConnection
	a := agentFuncs{PromptFunc: func(ctx context.Context, p PromptRequest) (PromptResponse, error) {
		s, err := StartToolCallStream(ctx, asc, p.SessionId, StartToolCall("t1", "Run", WithStartStatus(ToolCallStatusInProgress)),
			WithToolCallStreamInterval(time.Hour))
		if err != nil {
			return PromptResponse{}, err
		}
		_, _ = s.WriteString("late text")
		return PromptResponse{StopReason: StopReasonEndTurn}, nil
	}}
	asc, csc := streamPair(t, a, rec)
	within(t, prompt(t, csc, "s1"), "the answer")
	if e, _ := rec.entity("t1"); textOf(e) != "late text" || e.Status != ToolCallStatusInProgress {
		t.Errorf("when the answer arrived the client had text %q and status %q, want %q and in_progress", textOf(e), e.Status,
			"late text")
	}
}

// TestToolCallStreamTurnsSeparate requires, under PromptOverlapConcurrent,
// one turn's answer not to wait for another turn's stream, whose write is
// held (0052-MADR-agent-tool-call-snapshot-stream-and-message-size-default.md,
// D3).
func TestToolCallStreamTurnsSeparate(t *testing.T) {
	t.Parallel()
	hold, entered := make(chan struct{}), make(chan struct{}, 1)
	t.Cleanup(func() {
		select {
		case <-hold:
		default:
			close(hold)
		}
	})
	rec := &streamClient{}
	var asc *AgentSideConnection
	a := agentFuncs{PromptFunc: func(ctx context.Context, p PromptRequest) (PromptResponse, error) {
		id := ToolCallId("t" + p.Prompt[0].Text.Text)
		s, err := StartToolCallStream(ctx, asc, p.SessionId, StartToolCall(id, "Run"), WithToolCallStreamInterval(time.Hour))
		if err != nil {
			return PromptResponse{}, err
		}
		_, _ = s.WriteString("text")
		return PromptResponse{StopReason: StopReasonEndTurn}, nil
	}}
	asc, csc := streamPair(t, a, rec, WithPromptOverlapPolicy(PromptOverlapConcurrent), WithSendingMiddleware(holdCall("tB", hold, entered)))
	answer := func(turn string) <-chan error {
		done := make(chan error, 1)
		go func() {
			_, err := csc.Prompt(t.Context(), PromptRequest{SessionId: "s1", Prompt: []ContentBlock{TextBlock(turn)}})
			done <- err
		}()
		return done
	}
	b := answer("B")
	within(t, entered, "turn B's flush held")
	if err := within(t, answer("A"), "turn A's answer while turn B's flush is held"); err != nil {
		t.Fatal(err)
	}
	close(hold)
	if err := within(t, b, "turn B's answer"); err != nil {
		t.Fatal(err)
	}
}

// TestToolCallStreamCancelDuringFlush requires a session/cancel received
// while the turn's flush waits to reach the agent: the flush holds no lock of
// the side connection
// (0052-MADR-agent-tool-call-snapshot-stream-and-message-size-default.md, D3).
func TestToolCallStreamCancelDuringFlush(t *testing.T) {
	t.Parallel()
	hold, entered := make(chan struct{}), make(chan struct{}, 1)
	t.Cleanup(func() {
		select {
		case <-hold:
		default:
			close(hold)
		}
	})
	cancelled := make(chan struct{}, 1)
	rec := &streamClient{}
	var asc *AgentSideConnection
	a := agentFuncs{
		PromptFunc: func(ctx context.Context, p PromptRequest) (PromptResponse, error) {
			s, err := StartToolCallStream(ctx, asc, p.SessionId, StartToolCall("c1", "Run"), WithToolCallStreamInterval(time.Hour))
			if err != nil {
				return PromptResponse{}, err
			}
			_, _ = s.WriteString("text")
			return PromptResponse{StopReason: StopReasonEndTurn}, nil
		},
		CancelFunc: func(context.Context, CancelNotification) error {
			cancelled <- struct{}{}
			return nil
		},
	}
	asc, csc := streamPair(t, a, rec, WithSendingMiddleware(holdCall("c1", hold, entered)))
	answered := prompt(t, csc, "s1")
	within(t, entered, "the turn's flush held")
	if err := csc.Cancel(t.Context(), CancelNotification{SessionId: "s1"}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-cancelled:
	case <-time.After(5 * time.Second):
		t.Error("session/cancel did not reach the agent while the turn's flush was held")
	}
	close(hold)
	within(t, answered, "the answer")
}

// TestToolCallStreamNoTurn requires a stream started on a context of no turn
// to be flushed by no answer, and to work
// (0052-MADR-agent-tool-call-snapshot-stream-and-message-size-default.md, D3).
func TestToolCallStreamNoTurn(t *testing.T) {
	t.Parallel()
	rec := &streamClient{}
	var asc *AgentSideConnection
	streams := make(chan *ToolCallStream, 1)
	a := agentFuncs{PromptFunc: func(_ context.Context, p PromptRequest) (PromptResponse, error) {
		s, err := StartToolCallStream(context.Background(), asc, p.SessionId, StartToolCall("n1", "Run"),
			WithToolCallStreamInterval(time.Hour))
		if err != nil {
			return PromptResponse{}, err
		}
		_, _ = s.WriteString("text")
		streams <- s
		return PromptResponse{StopReason: StopReasonEndTurn}, nil
	}}
	asc, csc := streamPair(t, a, rec)
	within(t, prompt(t, csc, "s1"), "the answer")
	s := within(t, streams, "the stream")
	if e, _ := rec.entity("n1"); textOf(e) != "" {
		t.Errorf("a stream of no turn was flushed by the answer: the client has %q", textOf(e))
	}
	if err := s.Flush(t.Context()); err != nil {
		t.Fatal(err)
	}
	waitUntil(t, "the flushed text", func() bool {
		e, _ := rec.entity("n1")
		return textOf(e) == "text"
	})
}

// TestToolCallStreamFinishedLeavesTurn requires Finish to remove the stream
// from its turn, which then holds nothing for the answer to flush
// (0052-MADR-agent-tool-call-snapshot-stream-and-message-size-default.md, D3).
func TestToolCallStreamFinishedLeavesTurn(t *testing.T) {
	t.Parallel()
	rec := &streamClient{}
	var asc *AgentSideConnection
	held := make(chan [2]int, 1)
	a := agentFuncs{PromptFunc: func(ctx context.Context, p PromptRequest) (PromptResponse, error) {
		s, err := StartToolCallStream(ctx, asc, p.SessionId, StartToolCall("f1", "Run"))
		if err != nil {
			return PromptResponse{}, err
		}
		before := turnStreamsOf(ctx).size()
		if err := s.Finish(ctx, ToolCallStatusCompleted); err != nil {
			return PromptResponse{}, err
		}
		held <- [2]int{before, turnStreamsOf(ctx).size()}
		return PromptResponse{StopReason: StopReasonEndTurn}, nil
	}}
	asc, csc := streamPair(t, a, rec)
	within(t, prompt(t, csc, "s1"), "the answer")
	if got := within(t, held, "the turn's counts"); got != [2]int{1, 0} {
		t.Errorf("the turn held %d streams after the start and %d after Finish, want 1 and 0", got[0], got[1])
	}
}

// BenchmarkToolCallStream writes 1,000 chunks of 100 bytes to one stream,
// then to 16 at once, and finishes them.
func BenchmarkToolCallStream(b *testing.B) {
	chunk := strings.Repeat("x", 100)
	for _, n := range []int{1, 16} {
		b.Run(fmt.Sprint(n, " streams"), func(b *testing.B) {
			rec := &streamClient{}
			c2aR, c2aW := io.Pipe()
			a2cR, a2cW := io.Pipe()
			b.Cleanup(func() {
				_ = c2aW.Close()
				_ = a2cW.Close()
			})
			asc := NewAgentSideConnection(agentFuncs{}, a2cW, c2aR)
			csc := NewClientSideConnection(&clientFuncs{SessionUpdateFunc: rec.handle}, c2aW, a2cR)
			asc.negotiation.Store(allAdvertised())
			csc.negotiation.Store(allAdvertised())
			i := 0
			for b.Loop() {
				var all sync.WaitGroup
				for range n {
					i++
					s, err := StartToolCallStream(b.Context(), asc, "s1", StartToolCall(ToolCallId(fmt.Sprint("b", i)), "Run"))
					if err != nil {
						b.Fatal(err)
					}
					all.Go(func() {
						for range 1000 {
							_, _ = s.WriteString(chunk)
						}
						if err := s.Finish(b.Context(), ToolCallStatusCompleted); err != nil {
							b.Error(err)
						}
					})
				}
				all.Wait()
			}
		})
	}
}
