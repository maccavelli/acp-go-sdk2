package remote

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	acp "github.com/maccavelli/acp-go-sdk2"
	"github.com/maccavelli/acp-go-sdk2/acptest"
)

// waitBound is how long a test waits for an event it requires.
const waitBound = 10 * time.Second

// serveFunc is NewHandler's serve.
type serveFunc = func(ctx context.Context, in io.Reader, out io.Writer) error

// serveAgents serves a new agent from newAgent on each connection, through
// acp.ServeAgent.
func serveAgents(newAgent func() acp.Agent) serveFunc {
	return func(ctx context.Context, in io.Reader, out io.Writer) error {
		return acp.ServeAgent(ctx, newAgent(), in, out)
	}
}

// testServer serves h over loopback until the test ends, closing h first, as
// an http.Server's host must.
func testServer(t *testing.T, h *Handler) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(h)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := h.Close(ctx); err != nil && !errors.Is(err, ErrAbandoned) {
			t.Errorf("Handler.Close = %v", err)
		}
		ts.Close()
	})
	return ts
}

// wsURL is ts's WebSocket URL.
func wsURL(ts *httptest.Server) string {
	return "ws" + strings.TrimPrefix(ts.URL, "http")
}

// dialClient dials url and joins client to it, closing both when the test
// ends.
func dialClient(t *testing.T, url string, client acp.Client, opts *DialOptions) (*acp.ClientSideConnection, *Conn) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), waitBound)
	defer cancel()
	c, err := Dial(ctx, url, opts)
	if err != nil {
		t.Fatalf("Dial(%s) = %v", url, err)
	}
	conn := acp.NewClientSideConnection(client, c, c)
	t.Cleanup(func() {
		_ = c.Close()
		select {
		case <-conn.Done():
		case <-time.After(waitBound):
			t.Errorf("the client's connection did not end within %v of Conn.Close", waitBound)
		}
	})
	return conn, c
}

// startSession runs initialize and session/new on conn.
func startSession(t *testing.T, conn *acp.ClientSideConnection) acp.SessionId {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), waitBound)
	defer cancel()
	if _, err := conn.Initialize(ctx, acp.InitializeRequest{ProtocolVersion: acp.ProtocolVersionNumber}); err != nil {
		t.Fatalf("Initialize = %v", err)
	}
	sess, err := conn.NewSession(ctx, acp.NewSessionRequest{Cwd: acptest.Abs("/work")})
	if err != nil {
		t.Fatalf("NewSession = %v", err)
	}
	return sess.SessionId
}

// waitFor polls cond until it holds, or fails the test after waitBound.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(waitBound)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("%s: not within %v", what, waitBound)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// logBuffer is a Logger's output, for tests that count what was logged.
type logBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (l *logBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(p)
}

func (l *logBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.String()
}

// logger returns a Logger that writes to l, every level.
func (l *logBuffer) logger() *slog.Logger {
	return slog.New(slog.NewTextHandler(l, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

// lineRecorder keeps the lines written to it, one stream's, for a tap.
type lineRecorder struct {
	mu    sync.Mutex
	lines []string
	held  []byte
}

func (r *lineRecorder) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.held = append(r.held, p...)
	for {
		i := bytes.IndexByte(r.held, '\n')
		if i < 0 {
			return len(p), nil
		}
		r.lines = append(r.lines, string(r.held[:i]))
		r.held = r.held[i+1:]
	}
}

// all returns the lines recorded so far.
func (r *lineRecorder) all() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.lines...)
}

// find returns the first line holding every one of subs.
func (r *lineRecorder) find(subs ...string) (string, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
next:
	for _, l := range r.lines {
		for _, s := range subs {
			if !strings.Contains(l, s) {
				continue next
			}
		}
		return l, true
	}
	return "", false
}

// permissionAgent is a ScriptAgent whose Prompt asks the client's permission
// first, which ScriptAgent does not, and answers end_turn when it is granted.
type permissionAgent struct {
	acptest.ScriptAgent
	conn *acp.AgentSideConnection
}

func (a *permissionAgent) SetAgentConnection(c *acp.AgentSideConnection) {
	a.conn = c
	a.ScriptAgent.SetAgentConnection(c)
}

func (a *permissionAgent) Prompt(ctx context.Context, p acp.PromptRequest) (acp.PromptResponse, error) {
	resp, err := a.conn.RequestPermission(ctx, acp.RequestPermissionRequest{
		SessionId: p.SessionId,
		ToolCall:  acp.ToolCallUpdate{ToolCallId: "call-1"},
		Options:   []acp.PermissionOption{{Kind: acp.PermissionOptionKindAllowOnce, Name: "Allow", OptionId: "allow"}},
	})
	if err != nil {
		return acp.PromptResponse{}, err
	}
	if resp.Outcome.Selected == nil || resp.Outcome.Selected.OptionId != "allow" {
		return acp.PromptResponse{}, errors.New("permission not granted")
	}
	return acp.PromptResponse{StopReason: acp.StopReasonEndTurn}, nil
}

// modeAgent is a ScriptAgent whose SetSessionMode waits for its context's end
// and returns its error, which the connection answers -32800 when the client
// sent $/cancel_request.
type modeAgent struct {
	acptest.ScriptAgent
	started chan struct{}
}

func (a *modeAgent) SetSessionMode(ctx context.Context, _ acp.SetSessionModeRequest) (acp.SetSessionModeResponse, error) {
	close(a.started)
	<-ctx.Done()
	return acp.SetSessionModeResponse{}, ctx.Err()
}
