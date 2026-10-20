package acp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"testing/synctest"
	"time"
)

// The tests in this file pin what a clean end of input does: the requests
// already received are answered before the connection ends
// (0019-PLAN-issue-fixes.md, B23).

// TestRequestAnsweredAtEndOfInput requires an agent whose input holds a
// request and then ends to answer it, its handler running under a live
// context, before Done is closed.
func TestRequestAnsweredAtEndOfInput(t *testing.T) {
	t.Parallel()
	const req = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":1,"clientCapabilities":{}}}` + "\n"
	for range 20 {
		out := &syncBuffer{}
		live := make(chan bool, 1)
		asc := NewAgentSideConnection(agentFuncs{InitializeFunc: func(ctx context.Context, _ InitializeRequest) (InitializeResponse, error) {
			live <- ctx.Err() == nil
			return InitializeResponse{ProtocolVersion: ProtocolVersionNumber}, nil
		}}, out, strings.NewReader(req))
		select {
		case <-asc.Done():
		case <-time.After(5 * time.Second):
			t.Fatal("the connection did not end")
		}
		if !<-live {
			t.Fatal("the handler ran under an ended context")
		}
		if !strings.Contains(out.String(), `"id":1`) {
			t.Fatalf("the request was not answered before Done: %q", out.String())
		}
	}
}

// endOfInputError requires err to be the error of a request whose answer
// cannot arrive because the input ended.
func endOfInputError(t *testing.T, err error) {
	t.Helper()
	re, ok := errors.AsType[*RequestError](err)
	if !ok || re.Code != -32603 || !errors.Is(err, ErrPeerDisconnected) {
		t.Fatalf("error = %v, want -32603 wrapping ErrPeerDisconnected", err)
	}
	if data, _ := re.Data.(map[string]any); data["error"] != "input ended before a response" {
		t.Errorf("error data %v, want the end of input", re.Data)
	}
}

// TestPendingRequestAtEndOfInput requires a request waiting for its answer
// to fail at once when the input ends, and one whose answer was read before
// the end to return it.
func TestPendingRequestAtEndOfInput(t *testing.T) {
	t.Parallel()
	for _, answered := range []bool{false, true} {
		var c *Connection
		p := rawWire(t, func(w io.Writer, r io.Reader) { c = NewConnection(nil, w, r) })
		returned := make(chan error, 1)
		go func() {
			_, err := SendRequest[json.RawMessage](c, t.Context(), "_x/r", map[string]any{})
			returned <- err
		}()
		var req struct {
			ID json.RawMessage `json:"id"`
		}
		if err := json.Unmarshal([]byte(within(t, p.lines, "the request")), &req); err != nil {
			t.Fatal(err)
		}
		if answered {
			p.send(`{"jsonrpc":"2.0","id":` + string(req.ID) + `,"result":{}}`)
		}
		if err := p.w.(io.Closer).Close(); err != nil {
			t.Fatal(err)
		}
		err := within(t, returned, "the request's return")
		if answered {
			if err != nil {
				t.Errorf("answered before the end of input: %v, want the answer", err)
			}
			continue
		}
		endOfInputError(t, err)
	}
}

// TestRequestAfterEndOfInputNotWritten requires a request that a handler
// still running sends after the end of input to fail at once and write
// nothing, and the handler's own answer to be written. It runs in a synctest
// bubble, so the request is sent once the drain waits for the handler.
func TestRequestAfterEndOfInputNotWritten(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		release := make(chan struct{})
		outErr := make(chan error, 2)
		var c *Connection
		p := rawWire(t, func(w io.Writer, r io.Reader) {
			c = NewConnection(func(ctx context.Context, _ string, _ json.RawMessage) (any, *RequestError) {
				<-release
				_, err := SendRequest[json.RawMessage](c, ctx, "_x/out", map[string]any{})
				outErr <- err
				outErr <- c.SendRequestNoResult(ctx, "_x/out", map[string]any{})
				return map[string]any{}, nil
			}, w, r)
		})
		p.send(`{"jsonrpc":"2.0","id":9,"method":"_x/r"}`)
		if err := p.w.(io.Closer).Close(); err != nil {
			t.Fatal(err)
		}
		synctest.Wait() // the drain waits for the handler
		close(release)
		endOfInputError(t, <-outErr)
		endOfInputError(t, <-outErr)
		if a := p.next("the handler's answer"); string(a.ID) != "9" || a.Error != nil {
			t.Fatalf("first line written: id %s %+v, want the answer to 9", a.ID, a.Error)
		}
		<-c.Done()
		synctest.Wait()
		if len(p.lines) != 0 {
			t.Errorf("%q written after the answer, want nothing", <-p.lines)
		}
	})
}

// TestEndOfInputDrainIsBounded requires a handler that does not return to
// hold the connection open for notificationQueueDrainTimeout only: its
// context is then cancelled, as the connection ends, Done is closed, and its
// answer is not written. It runs in a synctest bubble, so the bound passes in
// no wall time.
func TestEndOfInputDrainIsBounded(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		causes := make(chan error, 1)
		var c *Connection
		p := rawWire(t, func(w io.Writer, r io.Reader) {
			c = NewConnection(func(ctx context.Context, _ string, _ json.RawMessage) (any, *RequestError) {
				<-ctx.Done()
				causes <- context.Cause(ctx)
				return map[string]any{}, nil
			}, w, r)
		})
		start := time.Now()
		p.send(`{"jsonrpc":"2.0","id":3,"method":"_x/stuck"}`)
		if err := p.w.(io.Closer).Close(); err != nil {
			t.Fatal(err)
		}
		<-c.Done()
		if waited := time.Since(start); waited < notificationQueueDrainTimeout {
			t.Errorf("Done closed after %v, want the drain's %v", waited, notificationQueueDrainTimeout)
		}
		if cause := <-causes; cause == nil {
			t.Error("the handler's context ended without a cause")
		}
		synctest.Wait()
		if len(p.lines) != 0 {
			t.Errorf("%q written, want the stuck handler's answer dropped", <-p.lines)
		}
	})
}

// TestEndOfInputNotificationDrainIsBounded requires a notification handler
// that does not return to hold the end of the input for
// notificationQueueDrainTimeout only: the drain's timeout wakes its wait, and
// the handler's context is then cancelled with a cause. It runs in a synctest
// bubble, so the bound passes in no wall time, and a drain that is never woken
// leaves the bubble deadlocked (0019-PLAN-idioms-performance.md, D8).
func TestEndOfInputNotificationDrainIsBounded(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		causes := make(chan error, 1)
		p := rawWire(t, func(w io.Writer, r io.Reader) {
			NewConnection(func(ctx context.Context, _ string, _ json.RawMessage) (any, *RequestError) {
				<-ctx.Done()
				causes <- context.Cause(ctx)
				return nil, nil
			}, w, r)
		})
		start := time.Now()
		p.send(`{"jsonrpc":"2.0","method":"_x/stuck"}`)
		synctest.Wait() // the notification's handler runs
		if err := p.w.(io.Closer).Close(); err != nil {
			t.Fatal(err)
		}
		cause := <-causes
		if waited := time.Since(start); waited < notificationQueueDrainTimeout {
			t.Errorf("the handler's context ended after %v, want the drain's %v", waited, notificationQueueDrainTimeout)
		}
		if cause == nil {
			t.Error("the handler's context ended without a cause")
		}
	})
}

// TestCloseEndsEndOfInputDrain requires Close during the drain to end the
// connection at once, its shutdown included, even with a handler that ignores
// its context. It runs in a synctest bubble, so "at once" is no time.
func TestCloseEndsEndOfInputDrain(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		release := make(chan struct{})
		t.Cleanup(func() { close(release) })
		var logs syncBuffer
		var c *Connection
		p := rawWire(t, func(w io.Writer, r io.Reader) {
			c = NewConnection(func(context.Context, string, json.RawMessage) (any, *RequestError) {
				<-release // ignores its context
				return nil, nil
			}, w, r, WithLogger(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))))
		})
		start := time.Now()
		p.send(`{"jsonrpc":"2.0","id":4,"method":"_x/stuck"}`)
		if err := p.w.(io.Closer).Close(); err != nil {
			t.Fatal(err)
		}
		synctest.Wait() // the drain waits for the handler
		if err := c.Close(); err != nil {
			t.Fatal(err)
		}
		<-c.Done()
		synctest.Wait() // the reader shuts down, unless it still waits out the drain
		if waited := time.Since(start); waited != 0 {
			t.Errorf("the connection ended after %v, want at once", waited)
		}
		if !strings.Contains(logs.String(), "connection closed") {
			t.Errorf("the reader had not shut down after Close: %s", logs.String())
		}
	})
}
