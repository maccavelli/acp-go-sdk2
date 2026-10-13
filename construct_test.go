package acp

import (
	"bytes"
	"encoding/json"
	"io"
	"sync"
	"testing"
	"time"
)

// firstLine is a peer's input that hands over the first line written to it.
type firstLine struct {
	mu   sync.Mutex
	buf  bytes.Buffer
	sent bool
	line chan string
}

func newFirstLine() *firstLine { return &firstLine{line: make(chan string, 1)} }

func (f *firstLine) Write(p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.buf.Write(p)
	if i := bytes.IndexByte(f.buf.Bytes(), '\n'); i >= 0 && !f.sent {
		f.sent = true
		f.line <- string(f.buf.Bytes()[:i])
	}
	return len(p), nil
}

// TestConstructorAssignsBeforeReading builds each side with a message
// already waiting, so its handler runs as soon as reading starts. Run with
// -race, it fails when a constructor assigns its Connection after reading
// starts (0018-MADR-start-side-connections-after-assignment.md).
func TestConstructorAssignsBeforeReading(t *testing.T) {
	const runs = 100
	// The path is absolute on the host, so the request passes validation
	// and reaches the gate that answers a request before initialize.
	path, err := json.Marshal(testAbsPath("x"))
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name  string
		line  string
		build func(w io.Writer, r io.Reader) <-chan struct{}
		check func(t *testing.T, resp map[string]json.RawMessage)
	}{
		{
			name: "agent",
			line: `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":1}}`,
			build: func(w io.Writer, r io.Reader) <-chan struct{} {
				return NewAgentSideConnection(agentFuncs{}, w, r).Done()
			},
			check: func(t *testing.T, resp map[string]json.RawMessage) {
				t.Helper()
				if _, ok := resp["result"]; !ok {
					t.Fatalf("initialize answered %v, want a result", resp)
				}
			},
		},
		{
			name: "client",
			line: `{"jsonrpc":"2.0","id":1,"method":"fs/read_text_file","params":{"sessionId":"s","path":` + string(path) + `}}`,
			build: func(w io.Writer, r io.Reader) <-chan struct{} {
				return NewClientSideConnection(&clientFuncs{}, w, r).Done()
			},
			check: func(t *testing.T, resp map[string]json.RawMessage) {
				t.Helper()
				var e struct{ Code int }
				if err := json.Unmarshal(resp["error"], &e); err != nil || e.Code != -32601 {
					t.Fatalf("a request before initialize answered %v, want -32601", resp)
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for range runs {
				r, w := io.Pipe()
				// The write blocks until the connection reads it, so the
				// message is waiting when reading starts.
				go func() { _, _ = io.WriteString(w, tc.line+"\n") }()
				out := newFirstLine()
				done := tc.build(out, r)
				var line string
				select {
				case line = <-out.line:
				case <-time.After(5 * time.Second):
					t.Fatal("no answer")
				}
				var resp map[string]json.RawMessage
				if err := json.Unmarshal([]byte(line), &resp); err != nil {
					t.Fatalf("answer %q: %v", line, err)
				}
				tc.check(t, resp)
				_ = w.Close()
				<-done
			}
		})
	}
}
