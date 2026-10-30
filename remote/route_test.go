package remote

import (
	"bufio"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	acp "github.com/maccavelli/acp-go-sdk2"
	"github.com/maccavelli/acp-go-sdk2/acptest"
)

// TestClassify requires each kind of line to be read with its id key, method
// and sessions (0043-PLAN-remote-http-and-websocket-transport.md, phase 3).
func TestClassify(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		line string
		want message
	}{
		{`{"jsonrpc":"2.0","id":1,"method":"session/prompt","params":{"sessionId":"s1","prompt":[]}}`,
			message{kind: kindRequest, idKey: "1", method: "session/prompt", sessionID: "s1"}},
		{`{"jsonrpc":"2.0","id":"a b","method":"initialize","params":{}}`,
			message{kind: kindRequest, idKey: `"a b"`, method: "initialize"}},
		{`{"jsonrpc":"2.0","id":2.50,"result":{"sessionId":"s2"}}`,
			message{kind: kindResponse, idKey: "2.50", resultSessionID: "s2"}},
		{`{"jsonrpc":"2.0","id":3,"error":{"code":-32603,"message":"x"}}`,
			message{kind: kindResponse, idKey: "3"}},
		{`{"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"s1","update":{}}}`,
			message{kind: kindNotification, method: "session/update", sessionID: "s1"}},
		{`{"jsonrpc":"2.0","method":"$/cancel_request","params":{"requestId":4,"sessionId":"s1"}}`,
			message{kind: kindNotification, method: "$/cancel_request", sessionID: "s1"}},
		{`{"jsonrpc":"2.0","id":5,"method":"x","params":[1,2]}`,
			message{kind: kindRequest, idKey: "5", method: "x"}},
		{`{"jsonrpc":"2.0","id":6,"method":"x","params":{"sessionId":7}}`,
			message{kind: kindRequest, idKey: "6", method: "x"}},
		{`[{"jsonrpc":"2.0","method":"x"}]`, message{}},
		{`{"jsonrpc":"2.0","method":null}`, message{}},
		{`not json`, message{}},
	} {
		if got := classify([]byte(tc.line)); got != tc.want {
			t.Errorf("classify(%s) = %+v, want %+v", tc.line, got, tc.want)
		}
	}
	if s := classify([]byte(`{"jsonrpc":"2.0","method":"$/cancel_request","params":{"sessionId":"s1"}}`)).session(); s != "" {
		t.Errorf("a $/ notification's session = %q, want none: it is connection-scoped", s)
	}
	if s := classify([]byte(`{"jsonrpc":"2.0","method":"session/cancel","params":{"sessionId":"s1"}}`)).session(); s != "s1" {
		t.Errorf("session/cancel's session = %q, want s1", s)
	}
}

// TestSessionScoped requires the session methods of the reference SDKs' lists
// to be session-scoped, and the methods that make, list or precede a session
// not to be.
func TestSessionScoped(t *testing.T) {
	t.Parallel()
	for _, m := range []string{
		acp.AgentMethodSessionPrompt, acp.AgentMethodSessionCancel, acp.AgentMethodSessionClose, acp.AgentMethodSessionDelete,
		"session/fork", acp.AgentMethodSessionLoad, acp.AgentMethodSessionResume, acp.AgentMethodSessionSetConfigOption,
		acp.AgentMethodSessionSetMode,
	} {
		if !sessionScoped(m) {
			t.Errorf("sessionScoped(%q) = false, want true", m)
		}
	}
	for _, m := range []string{acp.AgentMethodInitialize, acp.AgentMethodSessionNew, acp.AgentMethodSessionList, acp.AgentMethodAuthenticate, "$/cancel_request"} {
		if sessionScoped(m) {
			t.Errorf("sessionScoped(%q) = true, want false", m)
		}
	}
}

// TestIDKeyBytesExact requires an id key to be the id's bytes, not its value:
// 1.0 and 1 are the same JSON number and different keys. The connection
// answers an id with the bytes it received, which TestIDKeyRoundTrip shows
// for 1.0, so a key that normalized numbers could pair an answer with a
// request it does not answer
// (0043-PLAN-remote-http-and-websocket-transport.md, "Deviation, 2026-10-29,
// phase 3's id-key break").
func TestIDKeyBytesExact(t *testing.T) {
	t.Parallel()
	req := classify([]byte(`{"jsonrpc":"2.0","id":1.0,"method":"x"}`))
	if same := classify([]byte(`{"jsonrpc":"2.0","id":1.0,"result":{}}`)); same.idKey != req.idKey {
		t.Errorf("the answer to id 1.0 keys %q, the request %q: want the same key", same.idKey, req.idKey)
	}
	if other := classify([]byte(`{"jsonrpc":"2.0","id":1,"result":{}}`)); other.idKey == req.idKey {
		t.Errorf("the answer to id 1 keys %q, as the request with id 1.0 does: want byte-exact keys, which differ", other.idKey)
	}
}

// TestIDKeyRoundTrip requires a request received, made a line by toLine, and
// the agent connection's answer to it, to have the same id key: the router
// finds an answer's route by it
// (0043-PLAN-remote-http-and-websocket-transport.md, phase 3).
func TestIDKeyRoundTrip(t *testing.T) {
	t.Parallel()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	served := make(chan error, 1)
	go func() {
		served <- acp.ServeAgent(context.Background(), &acptest.ScriptAgent{}, inR, outW)
		_ = outW.Close()
	}()
	answers := bufio.NewReader(outR)
	for _, req := range []string{
		"{\n  \"jsonrpc\": \"2.0\",\n  \"id\": 7,\n  \"method\": \"initialize\",\n  \"params\": {\"protocolVersion\": 1}\n}",
		`{"jsonrpc":"2.0","id":"a b","method":"session/new","params":{"cwd":"/","mcpServers":[]}}`,
		`{"jsonrpc":"2.0","id":  123456789012345678901234567890 ,"method":"session/prompt","params":{"sessionId":"session-1","prompt":[]}}`,
		`{"jsonrpc":"2.0","id":1.0,"method":"session/new","params":{"cwd":"/","mcpServers":[]}}`,
	} {
		line := toLine([]byte(req))
		if _, err := inW.Write(line); err != nil {
			t.Fatal(err)
		}
		answer, err := answers.ReadBytes('\n')
		if err != nil {
			t.Fatal(err)
		}
		sent, got := classify(line), classify(answer)
		if got.kind != kindResponse || got.idKey != sent.idKey {
			t.Errorf("request id key %q, answer %s: kind %d, id key %q; want a response keyed %q", sent.idKey, answer, got.kind, got.idKey, sent.idKey)
		}
	}
	_ = inW.Close()
	select {
	case err := <-served:
		if err != nil && !errors.Is(err, io.EOF) {
			t.Errorf("ServeAgent = %v, want nil at the input's end", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("ServeAgent did not return within 10s of its input's end")
	}
}
