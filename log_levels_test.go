package acp

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"
)

// logRecord is one record of a JSON log handler's output.
type logRecord map[string]any

// jsonLogs is a JSON log handler at Debug writing to a buffer, and the
// records it has written.
type jsonLogs struct{ buf lockedBuffer }

// logger returns a logger writing to l at Debug.
func (l *jsonLogs) logger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(&l.buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

// records returns the records written so far.
func (l *jsonLogs) records(t *testing.T) []logRecord {
	t.Helper()
	var out []logRecord
	for line := range strings.Lines(l.buf.String()) {
		var r logRecord
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatalf("a log line that is not JSON: %q", line)
		}
		out = append(out, r)
	}
	return out
}

// await returns the first record whose message is msg, waiting for it.
func (l *jsonLogs) await(t *testing.T, msg string) logRecord {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		for _, r := range l.records(t) {
			if r["msg"] == msg {
				return r
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("no %q in the log:\n%s", msg, l.buf.String())
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// rawLogged starts a Connection with handler and opts, logging to logs, and
// returns its peer.
func rawLogged(t *testing.T, logs *jsonLogs, handler MethodHandler, opts ...ConnectionOption) wire {
	t.Helper()
	return rawWire(t, func(w io.Writer, r io.Reader) {
		NewConnection(handler, w, r, append(opts, WithLogger(logs.logger()))...)
	})
}

// TestPeerFaultLogLevels requires each fault of the peer's message to log at
// Warn, a notification the dispatch refused too; a notification handler's own
// error and an invalid initialize answer to log at Error; and the end of a
// connection at Debug (0024-PLAN-behaviour-fixes.md, A4).
func TestPeerFaultLogLevels(t *testing.T) {
	t.Parallel()
	refuse := func(_ context.Context, method string, _ json.RawMessage) (any, *RequestError) {
		switch method {
		case "_x/refused":
			return nil, NewInvalidParams(map[string]any{"error": "bad params"})
		case "_x/failed":
			return nil, NewInternalError(map[string]any{"error": "the handler failed"})
		}
		return map[string]any{}, nil
	}
	for _, tc := range []struct {
		name, line, msg, level string
		opts                   []ConnectionOption
	}{
		{"a line over the limit", `{"jsonrpc":"2.0","method":"_x/n","params":{"pad":"` + strings.Repeat("x", 100) + `"}}`,
			"incoming message exceeds the size limit; answered and skipped", "WARN", []ConnectionOption{WithMaxMessageSize(64)}},
		{"not JSON", `{`, "failed to parse incoming message", "WARN", nil},
		{"not an object", `[1]`, "incoming message is not a JSON-RPC object", "WARN", nil},
		{"an invalid request", `{"jsonrpc":"1.0","id":1,"method":"_x/r"}`, "invalid request", "WARN", nil},
		{"an inbound id", `{"jsonrpc":"2.0","id":7e99999,"method":"_x/r"}`, "failed to canonicalize inbound request id", "WARN", nil},
		{"a response without an id", `{"jsonrpc":"2.0","result":{}}`, "received a response without a request id", "WARN", nil},
		{"a malformed response", `{"jsonrpc":"2.0","id":5,"result":{},"error":{"code":1,"message":"x"}}`, "malformed response", "WARN", nil},
		{"a response id", `{"jsonrpc":"2.0","id":7e99999,"result":{}}`, "failed to canonicalize response id", "WARN", nil},
		{"cancel params", `{"jsonrpc":"2.0","method":"$/cancel_request","params":[1]}`, "failed to parse $/cancel_request params", "WARN", nil},
		{"cancel without requestId", `{"jsonrpc":"2.0","method":"$/cancel_request","params":{}}`, "received $/cancel_request without requestId", "WARN", nil},
		{"a cancel requestId", `{"jsonrpc":"2.0","method":"$/cancel_request","params":{"requestId":{"a":1}}}`,
			"failed to canonicalize $/cancel_request requestId", "WARN", nil},
		{"a refused notification", `{"jsonrpc":"2.0","method":"_x/refused","params":{}}`, "failed to handle notification", "WARN", nil},
		{"a notification handler's own error", `{"jsonrpc":"2.0","method":"_x/failed","params":{}}`, "failed to handle notification", "ERROR", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var logs jsonLogs
			p := rawLogged(t, &logs, refuse, tc.opts...)
			p.send(tc.line)
			if r := logs.await(t, tc.msg); r["level"] != tc.level {
				t.Errorf("%q at %v, want %s", tc.msg, r["level"], tc.level)
			}
		})
	}
	t.Run("an invalid initialize answer", func(t *testing.T) {
		t.Parallel()
		var logs jsonLogs
		p := rawWire(t, func(w io.Writer, r io.Reader) {
			NewAgentSideConnection(agentFuncs{InitializeFunc: func(context.Context, InitializeRequest) (InitializeResponse, error) {
				return InitializeResponse{ProtocolVersion: 2}, nil
			}}, w, r, WithLogger(logs.logger()))
		})
		p.send(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":1}}`)
		if r := logs.await(t, "initialize answer not sent"); r["level"] != "ERROR" {
			t.Errorf("the invalid initialize answer at %v, want ERROR", r["level"])
		}
	})
	t.Run("an invalid handler result", func(t *testing.T) {
		t.Parallel()
		var logs jsonLogs
		p := rawWire(t, func(w io.Writer, r io.Reader) {
			asc := NewAgentSideConnection(agentFuncs{ListSessionsFunc: func(context.Context, ListSessionsRequest) (ListSessionsResponse, error) {
				return ListSessionsResponse{Sessions: []SessionInfo{{SessionId: "s", Cwd: "relative"}}}, nil
			}}, w, r, WithLogger(logs.logger()))
			negotiate(t, asc, allAdvertised())
		})
		p.send(`{"jsonrpc":"2.0","id":1,"method":"session/list","params":{}}`)
		if r := logs.await(t, "handler result fails validation; not sent"); r["level"] != "ERROR" {
			t.Errorf("the invalid result at %v, want ERROR", r["level"])
		}
	})
	t.Run("the connection's end", func(t *testing.T) {
		t.Parallel()
		var logs jsonLogs
		r, w := io.Pipe()
		c := NewConnection(nil, io.Discard, r, WithLogger(logs.logger()))
		if err := w.Close(); err != nil {
			t.Fatal(err)
		}
		<-c.Done()
		if rec := logs.await(t, "connection closed"); rec["level"] != "DEBUG" {
			t.Errorf("the connection's end at %v, want DEBUG", rec["level"])
		}
	})
}

// TestUncanonicalIDsLogSize requires the three lines that log an id the
// connection could not canonicalise to log its size, not its bytes: an inbound
// request's numeric id, a response's, and a $/cancel_request's requestId that
// is an object holding a marker. The marker must reach no log line
// (0024-PLAN-behaviour-fixes.md, A3).
func TestUncanonicalIDsLogSize(t *testing.T) {
	t.Parallel()
	const marker = "SECRET-7f3"
	requestID := `{"marker":"` + marker + `"}`
	for _, tc := range []struct {
		line, msg string
		size      int
	}{
		{`{"jsonrpc":"2.0","id":7e99999,"method":"_x/r"}`, "failed to canonicalize inbound request id", len("7e99999")},
		{`{"jsonrpc":"2.0","id":7e99999,"result":{}}`, "failed to canonicalize response id", len("7e99999")},
		{`{"jsonrpc":"2.0","method":"$/cancel_request","params":{"requestId":` + requestID + `}}`,
			"failed to canonicalize $/cancel_request requestId", len(requestID)},
	} {
		var logs jsonLogs
		p := rawLogged(t, &logs, func(context.Context, string, json.RawMessage) (any, *RequestError) { return map[string]any{}, nil })
		p.send(tc.line)
		r := logs.await(t, tc.msg)
		if _, ok := r["id"]; ok {
			t.Errorf("%q logs the id: %v", tc.msg, r)
		}
		if _, ok := r["requestId"]; ok {
			t.Errorf("%q logs the requestId: %v", tc.msg, r)
		}
		if got, ok := r["size"].(float64); !ok || int(got) != tc.size {
			t.Errorf("%q logs size %v, want %d", tc.msg, r["size"], tc.size)
		}
		if strings.Contains(logs.buf.String(), marker) {
			t.Errorf("the marker reached the log:\n%s", logs.buf.String())
		}
	}
}
