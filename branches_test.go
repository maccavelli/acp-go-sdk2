package acp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"math"
	"reflect"
	"strings"
	"testing"
	"testing/synctest"
)

// The tests in this file run the documented branches the coverage profile
// showed unrun (0019-PLAN-examples-docs-tests-tooling.md, E9).

// TestSendRequestNoResultAnswers requires SendRequestNoResult, its request
// written, to return nil for a result, the peer's error for an error, and the
// connection's end when the peer goes away before answering.
func TestSendRequestNoResultAnswers(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name, reply string // reply "": the peer closes its end
		code        int    // 0: nil
	}{
		{"a result", `"result":{}`, 0},
		{"an error", `"error":{"code":-32602,"message":"Invalid params"}`, -32602},
		{"the connection's end", "", -32603},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var c *Connection
			p := rawWire(t, func(w io.Writer, r io.Reader) { c = NewConnection(nil, w, r) })
			returned := make(chan error, 1)
			go func() { returned <- c.SendRequestNoResult(t.Context(), "_x/r", map[string]any{}) }()
			var req struct {
				ID json.RawMessage `json:"id"`
			}
			if err := json.Unmarshal([]byte(within(t, p.lines, "the request")), &req); err != nil {
				t.Fatal(err)
			}
			if tt.reply == "" {
				if err := p.w.(io.Closer).Close(); err != nil {
					t.Fatal(err)
				}
			} else {
				p.send(`{"jsonrpc":"2.0","id":` + string(req.ID) + `,` + tt.reply + `}`)
			}
			err := within(t, returned, "SendRequestNoResult's return")
			if tt.code == 0 {
				if err != nil {
					t.Fatalf("SendRequestNoResult = %v, want nil", err)
				}
				return
			}
			if re, ok := errors.AsType[*RequestError](err); !ok || re.Code != tt.code {
				t.Fatalf("SendRequestNoResult = %v, want code %d", err, tt.code)
			}
			if tt.reply == "" && !errors.Is(err, ErrPeerDisconnected) {
				t.Errorf("SendRequestNoResult = %v, want it to wrap ErrPeerDisconnected", err)
			}
		})
	}
}

// TestNotifyExtensionNames requires each side's NotifyExtension to refuse a
// name that does not start with "_" and write nothing, and the agent's to send
// a notification with a valid one.
func TestNotifyExtensionNames(t *testing.T) {
	t.Parallel()
	var asc *AgentSideConnection
	p := rawWire(t, func(w io.Writer, r io.Reader) { asc = NewAgentSideConnection(agentFuncs{}, w, r) })
	csc := NewClientSideConnection(&clientFuncs{}, io.Discard, idleReader(t))
	for _, name := range []string{"", "x/ping"} {
		if err := asc.NotifyExtension(t.Context(), name, nil); err == nil {
			t.Errorf("the agent's NotifyExtension(%q) = nil, want an error", name)
		}
		if err := csc.NotifyExtension(t.Context(), name, nil); err == nil {
			t.Errorf("the client's NotifyExtension(%q) = nil, want an error", name)
		}
	}
	if err := asc.NotifyExtension(t.Context(), "_x/ping", map[string]any{"n": 1}); err != nil {
		t.Fatalf("NotifyExtension(_x/ping) = %v", err)
	}
	// The refused names wrote nothing, so the first line is the valid one.
	var got map[string]any
	if err := json.Unmarshal([]byte(within(t, p.lines, "the notification")), &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"jsonrpc": "2.0", "method": "_x/ping", "params": map[string]any{"n": float64(1)}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("wrote %v, want %v", got, want)
	}
}

// promptCancelledLater is a context whose Done channel is withheld, so a
// handler's result arrives before its end is seen while its cause is already
// set: the race of a session/cancel with a handler's return.
type promptCancelledLater struct{ context.Context }

func (promptCancelledLater) Done() <-chan struct{} { return nil }

// TestAnswerUnlessCancelledHandlerFirst requires answerUnlessCancelled to
// answer cancelled when the handler returns first under a context whose cause
// is already ErrPromptCancelled, and to discard the handler's result.
func TestAnswerUnlessCancelledHandlerFirst(t *testing.T) {
	t.Parallel()
	c := NewConnection(nil, io.Discard, idleReader(t))
	ctx, cancel := context.WithCancelCause(t.Context())
	got, err := answerUnlessCancelled(c, promptCancelledLater{ctx}, "m", func(context.Context) (string, error) {
		cancel(ErrPromptCancelled)
		return "the handler's result", nil
	}, "cancelled")
	if got != "cancelled" || err != nil {
		t.Errorf("answerUnlessCancelled = %q, %v; want cancelled, nil", got, err)
	}
}

// TestPermissionCancelRequestWaitsForHandler requires a permission request
// cancelled by $/cancel_request, a cause other than ErrPromptCancelled, to be
// answered with what its handler returns once it does, not cancelled at once.
// It runs in a synctest bubble, so the request is cancelled while the answer
// waits on both the handler and the context.
func TestPermissionCancelRequestWaitsForHandler(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		causes := make(chan error, 1)
		p := rawWire(t, func(w io.Writer, r io.Reader) {
			csc := NewClientSideConnection(&clientFuncs{RequestPermissionFunc: func(ctx context.Context, _ RequestPermissionRequest) (RequestPermissionResponse, error) {
				<-ctx.Done()
				causes <- context.Cause(ctx)
				return RequestPermissionResponse{Outcome: NewRequestPermissionOutcomeSelected("allow")}, nil
			}}, w, r)
			negotiate(t, csc, allAdvertised())
		})
		p.send(`{"jsonrpc":"2.0","id":4,"method":"session/request_permission","params":{"sessionId":"s","toolCall":{"toolCallId":"t"},"options":[]}}`)
		synctest.Wait()
		p.send(`{"jsonrpc":"2.0","method":"$/cancel_request","params":{"requestId":4}}`)
		if cause := within(t, causes, "the handler's cancel"); !errors.Is(cause, context.Canceled) || errors.Is(cause, ErrPromptCancelled) {
			t.Errorf("context.Cause = %v, want context.Canceled", cause)
		}
		a := p.next("the permission request's answer")
		var resp RequestPermissionResponse
		if err := json.Unmarshal(a.Result, &resp); err != nil || resp.Outcome.Selected == nil || resp.Outcome.Selected.OptionId != "allow" {
			t.Errorf("answered %s %+v (%v), want the handler's selected outcome", a.Result, a.Error, err)
		}
	})
}

// blockedNotification starts a connection whose handler blocks on the
// notification _x/n until the test ends, sends it, and returns the
// connection once the handler runs, with the peer's end.
func blockedNotification(t *testing.T) (*Connection, wire) {
	t.Helper()
	started, release := make(chan struct{}), make(chan struct{})
	t.Cleanup(func() { close(release) })
	var c *Connection
	p := rawWire(t, func(w io.Writer, r io.Reader) {
		c = NewConnection(func(_ context.Context, method string, _ json.RawMessage) (any, *RequestError) {
			if method == "_x/n" {
				close(started)
				<-release
			}
			return nil, nil
		}, w, r)
	})
	p.send(`{"jsonrpc":"2.0","method":"_x/n"}`)
	within(t, started, "the notification's handler")
	return c, p
}

// TestSendRequestNoResultHeldBehindNotification requires SendRequestNoResult,
// answered while a notification received before the answer still runs, not
// to return until that notification's handler does, and to return its
// context's end when that ends first. It runs in a synctest bubble, so the
// context ends once the call waits.
func TestSendRequestNoResultHeldBehindNotification(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		c, p := blockedNotification(t)
		ctx, cancel := context.WithCancel(t.Context())
		returned := make(chan error, 1)
		go func() { returned <- c.SendRequestNoResult(ctx, "_x/r", map[string]any{}) }()
		var req struct {
			ID json.RawMessage `json:"id"`
		}
		if err := json.Unmarshal([]byte(within(t, p.lines, "the request")), &req); err != nil {
			t.Fatal(err)
		}
		p.send(`{"jsonrpc":"2.0","id":` + string(req.ID) + `,"result":{}}`)
		synctest.Wait()
		select {
		case err := <-returned:
			t.Fatalf("SendRequestNoResult returned %v while the notification's handler ran", err)
		default:
		}
		cancel()
		err := within(t, returned, "SendRequestNoResult's return")
		if re, ok := errors.AsType[*RequestError](err); !ok || re.Code != -32800 || !errors.Is(err, context.Canceled) {
			t.Errorf("SendRequestNoResult = %v, want -32800 wrapping context.Canceled", err)
		}
	})
}

// TestWaitNotificationsEnds requires the wait a response makes for the
// notifications received before it to return the caller's cause when its
// context ends, and the connection's end when the connection ends, while a
// notification's handler still runs.
func TestWaitNotificationsEnds(t *testing.T) {
	t.Parallel()
	t.Run("context", func(t *testing.T) {
		t.Parallel()
		c, _ := blockedNotification(t)
		ctx, cancel := context.WithCancel(t.Context())
		returned := make(chan error, 1)
		go func() { returned <- c.waitNotificationsUpTo(ctx, 1) }()
		cancel()
		err := within(t, returned, "the wait's return")
		if re, ok := errors.AsType[*RequestError](err); !ok || re.Code != -32800 || !errors.Is(err, context.Canceled) {
			t.Errorf("waitNotificationsUpTo = %v, want -32800 wrapping context.Canceled", err)
		}
	})
	t.Run("connection", func(t *testing.T) {
		t.Parallel()
		c, p := blockedNotification(t)
		returned := make(chan error, 1)
		go func() { returned <- c.waitNotificationsUpTo(t.Context(), 1) }()
		if err := p.w.(io.Closer).Close(); err != nil {
			t.Fatal(err)
		}
		err := within(t, returned, "the wait's return")
		if re, ok := errors.AsType[*RequestError](err); !ok || re.Code != -32603 || !errors.Is(err, ErrPeerDisconnected) {
			t.Errorf("waitNotificationsUpTo = %v, want -32603 wrapping ErrPeerDisconnected", err)
		}
	})
}

// TestCancelRequestFaults requires $/cancel_request with params that do not
// decode, without a requestId, or naming no request in flight to be logged
// or ignored and cancel nothing, and one naming an id that does not
// canonicalise to cancel the request keyed by that id's bytes.
func TestCancelRequestFaults(t *testing.T) {
	t.Parallel()
	var logs syncBuffer
	started := make(chan context.Context, 1)
	p := rawWire(t, func(w io.Writer, r io.Reader) {
		NewConnection(func(ctx context.Context, method string, _ json.RawMessage) (any, *RequestError) {
			if method != "_x/wait" {
				return nil, NewMethodNotFound(method)
			}
			started <- ctx
			<-ctx.Done()
			return nil, toReqErr(context.Cause(ctx))
		}, w, r, WithLogger(slog.New(slog.NewTextHandler(&logs, nil))))
	})
	p.send(`{"jsonrpc":"2.0","id":1e5000,"method":"_x/wait"}`)
	ctx := within(t, started, "the request's handler")
	for _, params := range []string{`[1]`, `{}`, `{"requestId":1e4999}`, `{"requestId":2}`} {
		expectNoAnswer(t, p, `{"jsonrpc":"2.0","method":"$/cancel_request","params":`+params+`}`)
		if ctx.Err() != nil {
			t.Fatalf("$/cancel_request with params %s cancelled request 1e5000", params)
		}
	}
	p.send(`{"jsonrpc":"2.0","method":"$/cancel_request","params":{"requestId":1e5000}}`)
	if a := p.next("the cancelled request's answer"); string(a.ID) != "1e5000" || a.Error == nil || a.Error.Code != -32800 {
		t.Errorf("answered id %s %+v, want id 1e5000 cancelled -32800", a.ID, a.Error)
	}
	out := logs.String()
	for _, want := range []string{
		"failed to parse $/cancel_request params",
		"received $/cancel_request without requestId",
		"failed to canonicalize inbound request id",
		"failed to canonicalize $/cancel_request requestId",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the log has no %q: %s", want, out)
		}
	}
}

// TestNilHandlerAnswers requires a connection without a handler to answer a
// request method-not-found and leave a notification unanswered.
func TestNilHandlerAnswers(t *testing.T) {
	t.Parallel()
	p := rawWire(t, func(w io.Writer, r io.Reader) { NewConnection(nil, w, r) })
	p.send(`{"jsonrpc":"2.0","id":3,"method":"_x/r"}`)
	if a := p.next("the request's answer"); string(a.ID) != "3" || a.Error == nil || a.Error.Code != -32601 {
		t.Errorf("answered id %s %+v, want id 3 -32601", a.ID, a.Error)
	}
	expectNoAnswer(t, p, `{"jsonrpc":"2.0","method":"_x/n"}`)
}

// TestUnmarshalableResultAnswered requires a handler's result that does not
// encode to be answered -32603 naming the encoding error.
func TestUnmarshalableResultAnswered(t *testing.T) {
	t.Parallel()
	p := rawWire(t, func(w io.Writer, r io.Reader) {
		NewConnection(func(context.Context, string, json.RawMessage) (any, *RequestError) {
			return map[string]any{"c": make(chan int)}, nil
		}, w, r)
	})
	p.send(`{"jsonrpc":"2.0","id":5,"method":"_x/r"}`)
	a := p.next("the request's answer")
	if string(a.ID) != "5" || a.Error == nil || a.Error.Code != -32603 {
		t.Fatalf("answered id %s %+v, want id 5 -32603", a.ID, a.Error)
	}
	if data, _ := a.Error.Data.(map[string]any); !strings.Contains(data["error"].(string), "unsupported type") {
		t.Errorf("error data %v, want the encoding error", a.Error.Data)
	}
}

// TestUncanonicalIDs requires a response whose id does not canonicalise to
// be logged and dropped, and an object with such an id and no method, result
// or error to be answered as an invalid request with that id.
func TestUncanonicalIDs(t *testing.T) {
	t.Parallel()
	var logs syncBuffer
	p := rawWire(t, func(w io.Writer, r io.Reader) {
		NewConnection(nil, w, r, WithLogger(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))))
	})
	expectNoAnswer(t, p, `{"jsonrpc":"2.0","id":1e5000,"result":1}`)
	if out := logs.String(); !strings.Contains(out, "failed to canonicalize response id") || !strings.Contains(out, "received a response to no pending request") {
		t.Errorf("the response was not logged: %s", out)
	}
	p.send(`{"jsonrpc":"2.0","id":1e5000}`)
	if a := p.next("the invalid request's answer"); string(a.ID) != "1e5000" || a.Error == nil || a.Error.Code != -32600 {
		t.Errorf("answered id %s %+v, want id 1e5000 -32600", a.ID, a.Error)
	}
}

// TestSmallConstructors requires NewAuthRequired, Ptr and WithStartRawOutput
// to build what their docs say.
func TestSmallConstructors(t *testing.T) {
	t.Parallel()
	if e := NewAuthRequired("d"); e.Code != -32000 || e.Message != "Authentication required" || e.Data != "d" {
		t.Errorf("NewAuthRequired = %+v", e)
	}
	v := 7
	if p := Ptr(v); *p != 7 || p == &v { //nolint:govet // inline: Ptr's own test calls it; callers write new(v).
		t.Errorf("Ptr(7) = %v pointing at %d, want a new pointer to 7", p, *p)
	}
	raw := json.RawMessage(`{"ok":true}`)
	if u := StartToolCall("c", "t", WithStartRawOutput(raw)); u.ToolCall == nil || string(u.ToolCall.RawOutput) != string(raw) {
		t.Errorf("WithStartRawOutput: %+v, want rawOutput %s", u.ToolCall, raw)
	}
}

// TestRequestErrorNilAndFallback requires a nil *RequestError's Error and
// Unwrap to be "<nil>" and nil, and Error to fall back to plain text when
// the data does not encode.
func TestRequestErrorNilAndFallback(t *testing.T) {
	t.Parallel()
	var e *RequestError
	if got := e.Error(); got != "<nil>" {
		t.Errorf("nil Error() = %q, want <nil>", got)
	}
	if got := e.Unwrap(); got != nil {
		t.Errorf("nil Unwrap() = %v, want nil", got)
	}
	if got, want := NewInternalError(math.Inf(1)).Error(), "code -32603: Internal error (data: +Inf)"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}
