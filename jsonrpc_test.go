package acp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"
)

// wire is a raw peer of a connection: it writes lines to it and reads the
// lines it writes back.
type wire struct {
	t     *testing.T
	w     io.Writer
	lines chan string
}

// send writes s and a newline.
func (p wire) send(s string) {
	p.t.Helper()
	if _, err := io.WriteString(p.w, s+"\n"); err != nil {
		p.t.Fatal(err)
	}
}

// answer is a received JSON-RPC response, its id raw.
type answer struct {
	ID     json.RawMessage `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *RequestError   `json:"error"`
}

// next returns the next line the connection wrote, decoded.
func (p wire) next(what string) answer {
	p.t.Helper()
	line := within(p.t, p.lines, what)
	var a answer
	if err := json.Unmarshal([]byte(line), &a); err != nil {
		p.t.Fatalf("%s: %q: %v", what, line, err)
	}
	return a
}

// rawWire starts a connection from start, reading from and writing to a raw
// peer.
func rawWire(t *testing.T, start func(w io.Writer, r io.Reader)) wire {
	t.Helper()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	t.Cleanup(func() {
		_ = inW.Close()
		_ = outW.Close()
	})
	start(outW, inR)
	lines := make(chan string, 16)
	go func() {
		sc := bufio.NewScanner(outR)
		sc.Buffer(nil, 1<<20)
		for sc.Scan() {
			lines <- sc.Text()
		}
	}()
	return wire{t: t, w: inW, lines: lines}
}

// agentWire is a raw peer of an agent whose session/new answers "s".
func agentWire(t *testing.T) wire {
	t.Helper()
	return rawWire(t, func(w io.Writer, r io.Reader) {
		negotiate(t, NewAgentSideConnection(agentFuncs{NewSessionFunc: func(context.Context, NewSessionRequest) (NewSessionResponse, error) {
			return NewSessionResponse{SessionId: "s"}, nil
		}}, w, r), allAdvertised())
	})
}

// probe is a request for a method no connection has; its answer, -32601 with
// id "probe", shows that the lines before it were not answered.
const probe = `{"jsonrpc":"2.0","id":"probe","method":"nope/x"}`

// expectNoAnswer requires the next answer to be the probe's.
func expectNoAnswer(t *testing.T, p wire, line string) {
	t.Helper()
	p.send(line)
	p.send(probe)
	if a := p.next("the probe's answer"); string(a.ID) != `"probe"` || a.Error == nil || a.Error.Code != -32601 {
		t.Errorf("%s was answered: got %s %+v before the probe's answer", line, a.ID, a.Error)
	}
}

// TestJSONRPCErrorAnswers requires each line that is not a valid Request
// object to be answered as JSON-RPC 2.0 requires, with its id when it has a
// valid one and null otherwise, a request with a null id to be answered with
// null, and a valid notification not to be answered
// (0017-PLAN-remediate-acp-v1-conformance-violations.md, step 7).
func TestJSONRPCErrorAnswers(t *testing.T) {
	t.Parallel()
	cwd, err := json.Marshal(testAbsPath())
	if err != nil {
		t.Fatal(err)
	}
	newSession := `"method":"session/new","params":{"cwd":` + string(cwd) + `,"mcpServers":[]}`
	p := agentWire(t)
	for _, tt := range []struct {
		line, id string
		code     int // 0: a result
	}{
		{`not json`, "null", -32700},
		{`{"jsonrpc":"2.0","method":5,"id":7}`, "7", -32600},
		{`{"jsonrpc":"1.0","id":8,` + newSession + `}`, "8", -32600},
		{`{"jsonrpc":"2.0","id":null,` + newSession + `}`, "null", 0},
		{`{"jsonrpc":"2.0"}`, "null", -32600},
		{`[]`, "null", -32600},
		{`{"jsonrpc":"2.0","id":9,"method":"session/new","params":5}`, "9", -32600},
		{`{"jsonrpc": "2.0", "method": 1, "params": "bar"}`, "null", -32600},
		{`{"jsonrpc":"2.0","id":{"a":1},"method":"x"}`, "null", -32600},
	} {
		p.send(tt.line)
		a := p.next(tt.line)
		switch {
		case string(a.ID) != tt.id:
			t.Errorf("%s: answered with id %s, want %s", tt.line, a.ID, tt.id)
		case tt.code == 0 && (a.Error != nil || !strings.Contains(string(a.Result), `"sessionId":"s"`)):
			t.Errorf("%s: answered %s %+v, want the session", tt.line, a.Result, a.Error)
		case tt.code != 0 && (a.Error == nil || a.Error.Code != tt.code):
			t.Errorf("%s: answered %+v, want %d", tt.line, a.Error, tt.code)
		}
	}
	expectNoAnswer(t, p, `{"jsonrpc":"2.0","method":"nope/x"}`)
}

// TestJSONRPCNoReplyToResponses requires an error response without an id,
// and a result that answers no request, to go unanswered
// (0017-PLAN-remediate-acp-v1-conformance-violations.md, step 7).
func TestJSONRPCNoReplyToResponses(t *testing.T) {
	t.Parallel()
	p := agentWire(t)
	expectNoAnswer(t, p, `{"jsonrpc":"2.0","id":null,"error":{"code":-32700,"message":"Parse error"}}`)
	expectNoAnswer(t, p, `{"jsonrpc":"2.0","id":99,"result":{}}`)
}

// TestMalformedResponseFailsCall requires a response with neither or both of
// result and error, another jsonrpc, or an error that is not an object, to
// fail the call it answers (0017-PLAN-remediate-acp-v1-conformance-violations.md, step 7).
func TestMalformedResponseFailsCall(t *testing.T) {
	t.Parallel()
	for _, reply := range []string{
		`{"jsonrpc":"2.0","id":%s}`,
		`{"jsonrpc":"2.0","id":%s,"result":{"sessionId":"s"},"error":{"code":1,"message":"x"}}`,
		`{"jsonrpc":"1.0","id":%s,"result":{"sessionId":"s"}}`,
		`{"jsonrpc":"2.0","id":%s,"error":"boom"}`,
	} {
		t.Run(reply, func(t *testing.T) {
			t.Parallel()
			var c *ClientSideConnection
			p := rawWire(t, func(w io.Writer, r io.Reader) { c = NewClientSideConnection(&clientFuncs{}, w, r) })
			negotiate(t, c, allAdvertised())
			done := make(chan error, 1)
			go func() {
				_, err := c.NewSession(context.Background(), NewSessionRequest{Cwd: testAbsPath()})
				done <- err
			}()
			req := p.next("the request")
			p.send(fmt.Sprintf(reply, req.ID))
			if err := within(t, done, "NewSession"); err == nil || !strings.Contains(err.Error(), "malformed response") {
				t.Errorf("NewSession = %v, want a malformed-response error", err)
			}
		})
	}
}

// lineOf returns a request for a method no connection has, padded to n bytes.
func lineOf(t *testing.T, id, n int) string {
	t.Helper()
	head, tail := fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"nope/x","params":{"p":"`, id), `"}}`
	if pad := n - len(head) - len(tail); pad >= 0 {
		return head + strings.Repeat("a", pad) + tail
	}
	t.Fatalf("a line of %d bytes is too short", n)
	return ""
}

// TestMaxMessageSize requires a line over the limit to be answered -32600
// with id null and reading to go on, and a line at the limit to be read
// (0017-PLAN-remediate-acp-v1-conformance-violations.md, step 7).
func TestMaxMessageSize(t *testing.T) {
	t.Parallel()
	notFound := func(_ context.Context, method string, _ json.RawMessage) (any, *RequestError) {
		return nil, NewMethodNotFound(method)
	}
	for _, tt := range []struct {
		name  string
		limit int
		opts  []ConnectionOption
	}{
		{"WithMaxMessageSize(1024)", 1024, []ConnectionOption{WithMaxMessageSize(1024)}},
		{"the default", defaultMaxMessageSize, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := rawWire(t, func(w io.Writer, r io.Reader) { NewConnection(notFound, w, r, tt.opts...) })
			p.send(lineOf(t, 1, tt.limit+1))
			if a := p.next("the overlong line's answer"); string(a.ID) != "null" || a.Error == nil || a.Error.Code != -32600 ||
				!strings.Contains(fmt.Sprint(a.Error.Data), fmt.Sprintf("exceeds %d bytes", tt.limit)) {
				t.Errorf("overlong line answered %s %+v, want -32600 with id null naming the limit", a.ID, a.Error)
			}
			p.send(lineOf(t, 2, tt.limit))
			if a := p.next("the line at the limit"); string(a.ID) != "2" || a.Error == nil || a.Error.Code != -32601 {
				t.Errorf("line at the limit answered %s %+v, want -32601 with id 2", a.ID, a.Error)
			}
		})
	}
}
