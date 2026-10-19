package acp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"
	"testing/synctest"
	"time"
)

// failingWriter fails every write with err.
type failingWriter struct{ err error }

// Write returns w.err.
func (w failingWriter) Write([]byte) (int, error) { return 0, w.err }

// wantRequestErrorWrapping requires err to be a *RequestError that wraps
// target.
func wantRequestErrorWrapping(t *testing.T, what string, err, target error) {
	t.Helper()
	if _, ok := errors.AsType[*RequestError](err); !ok || !errors.Is(err, target) {
		t.Errorf("%s = %v, want a *RequestError wrapping %v", what, err, target)
	}
}

// bounded is t's context, ended after five seconds: a call that a broken
// build sends instead of refusing fails then rather than waiting for the
// test's deadline.
func bounded(t *testing.T) context.Context {
	t.Helper()
	ctx, stop := context.WithTimeout(t.Context(), 5*time.Second)
	t.Cleanup(stop)
	return ctx
}

// TestMethodErrorsAsDocumented makes, through the typed and the extension
// methods of a side connection, the claims of their errors paragraphs that no
// other test makes through them: a context's own cause, ended before the send
// or while waiting; a write's error; a response that does not decode; params
// that break the schema; the connection's end, closed here or by the peer;
// the peer's error answer; and an extension's params that do not encode, that
// encode as neither an object nor an array, or that are of a type of this
// package and break the schema (0024-PLAN-documentation.md, A6).
func TestMethodErrorsAsDocumented(t *testing.T) {
	t.Parallel()
	newSession := NewSessionRequest{Cwd: testAbsPath(), McpServers: []McpServer{}}
	cancel := CancelNotification{SessionId: "s"}
	ext := map[string]any{"n": 1}
	t.Run("a context's cause, before the send", func(t *testing.T) {
		t.Parallel()
		cause := errors.New("the caller gave up")
		ctx, stop := context.WithCancelCause(t.Context())
		stop(cause)
		csc := NewClientSideConnection(&clientFuncs{}, io.Discard, idleReader(t))
		negotiate(t, csc, versionOnly())
		_, err := csc.NewSession(ctx, newSession)
		wantRequestErrorWrapping(t, "NewSession", err, cause)
		wantRequestErrorWrapping(t, "Cancel", csc.Cancel(ctx, cancel), cause)
		_, err = csc.CallExtension(ctx, "_x/call", ext)
		wantRequestErrorWrapping(t, "CallExtension", err, cause)
		wantRequestErrorWrapping(t, "NotifyExtension", csc.NotifyExtension(ctx, "_x/notify", ext), cause)
	})
	// It runs in a synctest bubble, so the call has seen its write done and
	// waits for the answer when its context ends; ended earlier, the context
	// would end the wait for the write instead.
	t.Run("a context's cause, while waiting", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			var csc *ClientSideConnection
			p := rawWire(t, func(w io.Writer, r io.Reader) {
				csc = NewClientSideConnection(&clientFuncs{}, w, r)
				negotiate(t, csc, versionOnly())
			})
			for _, call := range []struct {
				name string
				send func(context.Context) error
			}{
				{"NewSession", func(ctx context.Context) error { _, err := csc.NewSession(ctx, newSession); return err }},
				{"CallExtension", func(ctx context.Context) error { _, err := csc.CallExtension(ctx, "_x/call", ext); return err }},
			} {
				cause := errors.New(call.name + ": the caller gave up")
				ctx, stop := context.WithCancelCause(t.Context())
				done := make(chan error, 1)
				go func() { done <- call.send(ctx) }()
				within(t, p.lines, call.name+"'s request")
				synctest.Wait() // the call waits for its answer
				stop(cause)
				wantRequestErrorWrapping(t, call.name, within(t, done, call.name+"'s return"), cause)
				within(t, p.lines, call.name+"'s cancel request")
			}
		})
	})
	t.Run("a write's error", func(t *testing.T) {
		t.Parallel()
		writeErr := errors.New("the disk is full")
		csc := NewClientSideConnection(&clientFuncs{}, failingWriter{writeErr}, idleReader(t))
		negotiate(t, csc, versionOnly())
		_, err := csc.NewSession(t.Context(), newSession)
		wantRequestErrorWrapping(t, "NewSession", err, writeErr)
		wantRequestErrorWrapping(t, "Cancel", csc.Cancel(t.Context(), cancel), writeErr)
		_, err = csc.CallExtension(t.Context(), "_x/call", ext)
		wantRequestErrorWrapping(t, "CallExtension", err, writeErr)
		wantRequestErrorWrapping(t, "NotifyExtension", csc.NotifyExtension(t.Context(), "_x/notify", ext), writeErr)
	})
	t.Run("a response that does not decode", func(t *testing.T) {
		t.Parallel()
		var csc *ClientSideConnection
		p := rawWire(t, func(w io.Writer, r io.Reader) {
			csc = NewClientSideConnection(&clientFuncs{}, w, r)
			negotiate(t, csc, versionOnly())
		})
		done := make(chan error, 1)
		go func() { _, err := csc.NewSession(t.Context(), newSession); done <- err }()
		var req struct{ ID json.RawMessage }
		if err := json.Unmarshal([]byte(within(t, p.lines, "the request")), &req); err != nil {
			t.Fatal(err)
		}
		p.send(`{"jsonrpc":"2.0","id":` + string(req.ID) + `,"result":{"sessionId":42}}`)
		err := within(t, done, "NewSession's return")
		if _, ok := errors.AsType[*RequestError](err); !ok {
			t.Errorf("NewSession = %v, want a *RequestError", err)
		}
	})
	t.Run("params that break the schema", func(t *testing.T) {
		t.Parallel()
		csc := NewClientSideConnection(&clientFuncs{}, io.Discard, idleReader(t))
		negotiate(t, csc, versionOnly())
		_, err := csc.NewSession(bounded(t), NewSessionRequest{Cwd: "relative", McpServers: []McpServer{}})
		if _, ok := errors.AsType[*ValidationError](err); !ok {
			t.Errorf("NewSession = %v, want a *ValidationError", err)
		}
	})
	t.Run("the connection's end", func(t *testing.T) {
		t.Parallel()
		closed := NewClientSideConnection(&clientFuncs{}, io.Discard, idleReader(t))
		negotiate(t, closed, versionOnly())
		if err := closed.Close(); err != nil {
			t.Fatal(err)
		}
		pr, pw := io.Pipe()
		ended := NewClientSideConnection(&clientFuncs{}, io.Discard, pr)
		negotiate(t, ended, versionOnly())
		if err := pw.Close(); err != nil {
			t.Fatal(err)
		}
		within(t, ended.Done(), "the peer's end")
		for _, side := range []struct {
			csc  *ClientSideConnection
			want error
		}{{closed, ErrConnectionClosed}, {ended, ErrPeerDisconnected}} {
			_, err := side.csc.NewSession(bounded(t), newSession)
			wantRequestErrorWrapping(t, "NewSession", err, side.want)
			wantRequestErrorWrapping(t, "Cancel", side.csc.Cancel(t.Context(), cancel), side.want)
			_, err = side.csc.CallExtension(bounded(t), "_x/call", ext)
			wantRequestErrorWrapping(t, "CallExtension", err, side.want)
			wantRequestErrorWrapping(t, "NotifyExtension", side.csc.NotifyExtension(t.Context(), "_x/notify", ext), side.want)
		}
	})
	t.Run("the peer's error answer", func(t *testing.T) {
		t.Parallel()
		var csc *ClientSideConnection
		p := rawWire(t, func(w io.Writer, r io.Reader) {
			csc = NewClientSideConnection(&clientFuncs{}, w, r)
			negotiate(t, csc, versionOnly())
		})
		for _, call := range []struct {
			name string
			send func() error
		}{
			{"NewSession", func() error { _, err := csc.NewSession(t.Context(), newSession); return err }},
			{"CallExtension", func() error { _, err := csc.CallExtension(t.Context(), "_x/call", ext); return err }},
		} {
			done := make(chan error, 1)
			go func() { done <- call.send() }()
			var req struct{ ID json.RawMessage }
			if err := json.Unmarshal([]byte(within(t, p.lines, call.name+"'s request")), &req); err != nil {
				t.Fatal(err)
			}
			p.send(`{"jsonrpc":"2.0","id":` + string(req.ID) + `,"error":{"code":-32042,"message":"no"}}`)
			err := within(t, done, call.name+"'s return")
			if re, ok := errors.AsType[*RequestError](err); !ok || re.Code != -32042 {
				t.Errorf("%s = %v, want the peer's *RequestError, -32042", call.name, err)
			}
		}
	})
	t.Run("an extension's params that do not encode, or encode as neither an object nor an array", func(t *testing.T) {
		t.Parallel()
		csc := NewClientSideConnection(&clientFuncs{}, io.Discard, idleReader(t))
		for _, params := range []any{"bar", 42, map[string]any{"f": func() {}}} {
			_, err := csc.CallExtension(bounded(t), "_x/call", params)
			if re, ok := errors.AsType[*RequestError](err); !ok || re.Code != -32602 {
				t.Errorf("CallExtension(%T) = %v, want a *RequestError, -32602", params, err)
			}
			err = csc.NotifyExtension(t.Context(), "_x/notify", params)
			if re, ok := errors.AsType[*RequestError](err); !ok || re.Code != -32602 {
				t.Errorf("NotifyExtension(%T) = %v, want a *RequestError, -32602", params, err)
			}
		}
	})
	t.Run("an extension's params that break the schema", func(t *testing.T) {
		t.Parallel()
		csc := NewClientSideConnection(&clientFuncs{}, io.Discard, idleReader(t))
		bad := NewSessionRequest{Cwd: "relative", McpServers: []McpServer{}}
		_, err := csc.CallExtension(bounded(t), "_x/call", bad)
		if _, ok := errors.AsType[*ValidationError](err); !ok {
			t.Errorf("CallExtension = %v, want a *ValidationError", err)
		}
		if err := csc.NotifyExtension(t.Context(), "_x/notify", bad); err == nil {
			t.Error("NotifyExtension = nil, want a *ValidationError")
		} else if _, ok := errors.AsType[*ValidationError](err); !ok {
			t.Errorf("NotifyExtension = %v, want a *ValidationError", err)
		}
	})
}
