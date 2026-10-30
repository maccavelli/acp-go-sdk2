package agentproc

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"runtime/pprof"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	acp "github.com/maccavelli/acp-go-sdk2"
	"github.com/maccavelli/acp-go-sdk2/acptest"
)

// hungClient's SessionUpdate ignores its context: it blocks until release
// closes, as a client handler that never returns does. entered closes at its
// first call, and returned is set as it returns.
type hungClient struct {
	acp.Client
	entered  chan struct{}
	release  chan struct{}
	once     sync.Once
	returned atomic.Bool
}

func newHungClient() *hungClient {
	return &hungClient{entered: make(chan struct{}), release: make(chan struct{})}
}

func (c *hungClient) SessionUpdate(context.Context, acp.SessionNotification) error {
	c.once.Do(func() { close(c.entered) })
	<-c.release
	c.returned.Store(true)
	return nil
}

// hang starts an agent with start, c its client, registers c's release to
// run before the agent's Close when the test ends, and returns the agent and
// the release once c's handler is blocked
// (0042-MADR-in-process-agent-runner.md, the amendment of 2026-10-30, I3).
func hang(t *testing.T, c *hungClient, start func(*testing.T, *hungClient) *Process) (*Process, func()) {
	t.Helper()
	p := start(t, c)
	release := sync.OnceFunc(func() { close(c.release) })
	t.Cleanup(release)
	select {
	case <-c.entered:
	case <-time.After(readyWait):
		t.Fatalf("the client's SessionUpdate was not called within %v", readyWait)
	}
	return p, release
}

// hungInProcess runs, in process, a ScriptAgent whose one turn sends a
// session update, served by serve, and prompts it.
func hungInProcess(serve func(context.Context, acp.Agent, io.Reader, io.Writer) error) func(*testing.T, *hungClient) *Process {
	return func(t *testing.T, c *hungClient) *Process {
		t.Helper()
		agent := &acptest.ScriptAgent{Turns: []acptest.Turn{{
			Updates: []acp.SessionUpdate{acp.UpdateAgentMessageText("hi")}, StopReason: acp.StopReasonEndTurn,
		}}}
		p, err := (&InProcess{Grace: testGrace, Serve: func(ctx context.Context, in io.Reader, out io.Writer) error {
			return serve(ctx, agent, in, out)
		}, ConnectionOptions: []acp.ConnectionOption{acp.WithLogger(slog.New(slog.DiscardHandler))}}).Start(c)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = p.Close() })
		ctx := t.Context()
		if _, err := p.Conn().Initialize(ctx, acp.InitializeRequest{ProtocolVersion: acp.ProtocolVersionNumber}); err != nil {
			t.Fatalf("Initialize = %v", err)
		}
		s, err := p.Conn().NewSession(ctx, acp.NewSessionRequest{Cwd: t.TempDir(), McpServers: []acp.McpServer{}})
		if err != nil {
			t.Fatalf("NewSession = %v", err)
		}
		// The prompt's answer may never come: the handler holds the update.
		go func() {
			_, _ = p.Conn().Prompt(ctx, acp.PromptRequest{SessionId: s.SessionId, Prompt: []acp.ContentBlock{acp.TextBlock("go")}})
		}()
		return p
	}
}

// serveAgentNil serves agent with acp.ServeAgent, which returns nil at its
// input's end once its own handlers have returned.
func serveAgentNil(ctx context.Context, agent acp.Agent, in io.Reader, out io.Writer) error {
	return acp.ServeAgent(ctx, agent, in, out)
}

// hungChild runs the test binary's "burst" agent, which writes burstCount
// session updates and exits at once.
func hungChild(t *testing.T, c *hungClient) *Process {
	t.Helper()
	p := start(t, "burst", c)
	if _, err := p.Conn().Initialize(t.Context(), acp.InitializeRequest{}); err != nil {
		t.Fatalf("Initialize = %v", err)
	}
	if err := p.Conn().Connection().SendNotification(t.Context(), "_test/go", map[string]any{}); err != nil {
		t.Fatalf("SendNotification = %v", err)
	}
	return p
}

// isClosed reports whether c is closed.
func isClosed(c <-chan struct{}) bool {
	select {
	case <-c:
		return true
	default:
		return false
	}
}

// TestCloseContextBoundsHandlers requires CloseContext and KillContext, for
// an in-process agent and a child, to return within their bound, and a
// grace, when a client handler ignores its context: their error wraps the
// bound's cause and acp.ErrHandlersRunning, and joins the agent's error, and
// Done is closed. Once the handler is released, Wait returns that agent's
// error, which Close or Kill would have returned
// (0042-MADR-in-process-agent-runner.md, the amendment of 2026-10-30, X1, X2).
func TestCloseContextBoundsHandlers(t *testing.T) {
	t.Parallel()
	for _, agent := range []struct {
		name  string
		start func(*testing.T, *hungClient) *Process
	}{
		{"in process", hungInProcess(serveAgentNil)},
		{"a child", hungChild},
	} {
		for _, m := range []struct {
			name   string
			stop   func(*Process, context.Context) error
			killed bool
		}{
			{"CloseContext", (*Process).CloseContext, false},
			{"KillContext", (*Process).KillContext, true},
		} {
			t.Run(agent.name+", "+m.name, func(t *testing.T) {
				t.Parallel()
				p, release := hang(t, newHungClient(), agent.start)
				const bound = testGrace
				ctx, cancel := context.WithTimeout(context.Background(), bound)
				defer cancel()
				took, err := within(t, bound+testGrace+slack, m.name, func() error { return m.stop(p, ctx) })
				if !errors.Is(err, context.DeadlineExceeded) || !errors.Is(err, acp.ErrHandlersRunning) {
					t.Errorf("%s = %v after %v, want the bound's cause and acp.ErrHandlersRunning", m.name, err, took)
				}
				if !isClosed(p.Done()) {
					t.Errorf("Done is open after %s returned: the agent must have ended", m.name)
				}
				release()
				_, werr := within(t, slack, "Wait", p.Wait)
				// The agent exits 0 of itself at its input's end; killed, it
				// may have exited first, or not.
				if werr != nil && (!m.killed || !errors.Is(werr, ErrKilled)) {
					t.Errorf("Wait after the release = %v, want nil, or ErrKilled after %s", werr, m.name)
				}
				if want := errors.Join(context.DeadlineExceeded, acp.ErrHandlersRunning, werr); err == nil || err.Error() != want.Error() {
					t.Errorf("%s = %v, want %v: the bound's cause, the sentinel, and Wait's error, joined in that order", m.name, err, want)
				}
			})
		}
	}
}

// TestCloseContextJoinsTheAgentsError requires CloseContext's error at its
// context's end to hold the agent's own error, which Wait cannot yet return
// while a handler holds it (0042-MADR-in-process-agent-runner.md, the
// amendment of 2026-10-30, the alternatives for X1).
func TestCloseContextJoinsTheAgentsError(t *testing.T) {
	t.Parallel()
	p, _ := hang(t, newHungClient(), hungInProcess(func(ctx context.Context, agent acp.Agent, in io.Reader, out io.Writer) error {
		_ = acp.ServeAgent(ctx, agent, in, out)
		return errServe
	}))
	ctx, cancel := context.WithTimeout(context.Background(), testGrace)
	defer cancel()
	_, err := within(t, 2*testGrace+slack, "CloseContext", func() error { return p.CloseContext(ctx) })
	want := errors.Join(context.DeadlineExceeded, acp.ErrHandlersRunning, errServe)
	if !errors.Is(err, errServe) || err.Error() != want.Error() {
		t.Errorf("CloseContext = %v, want %v: Serve's own error joined", err, want)
	}
}

// TestCloseContextWaitsForHandlers requires CloseContext to wait for a
// handler that returns within its bound, and then to return Wait's error,
// with nothing joined.
func TestCloseContextWaitsForHandlers(t *testing.T) {
	t.Parallel()
	c := newHungClient()
	p, release := hang(t, c, hungInProcess(serveAgentNil))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	time.AfterFunc(testGrace/2, release)
	took, err := within(t, 5*time.Second+slack, "CloseContext", func() error { return p.CloseContext(ctx) })
	if err != nil {
		t.Errorf("CloseContext = %v after %v, want Wait's, nil", err, took)
	}
	if !c.returned.Load() {
		t.Error("CloseContext returned before the handler had")
	}
}

// TestCloseContextHurriesTheSteps requires a context that ends during Close's
// steps to cut them short, as Kill does: a child that ignores its input's end
// and SIGTERM, under a 5 s grace, is killed without SIGTERM near the
// context's end, not after two graces; and KillContext to kill at once.
func TestCloseContextHurriesTheSteps(t *testing.T) {
	t.Parallel()
	const grace, bound = 5 * time.Second, testGrace
	for _, m := range []struct {
		name  string
		stop  func(*Process, context.Context) error
		floor time.Duration
	}{
		{"CloseContext", (*Process).CloseContext, bound},
		{"KillContext", (*Process).KillContext, 0},
	} {
		t.Run(m.name, func(t *testing.T) {
			t.Parallel()
			p := startTapped(t, "stubborn", nil, grace, nil, nil)
			ctx, cancel := context.WithTimeout(context.Background(), bound)
			defer cancel()
			took, err := within(t, 3*grace+slack, m.name, func() error { return m.stop(p, ctx) })
			if !errors.Is(err, ErrKilled) || errors.Is(err, ErrTerminated) {
				t.Errorf("%s = %v, want ErrKilled, without SIGTERM's ErrTerminated", m.name, err)
			}
			if took < m.floor || took >= bound+slack {
				t.Errorf("%s took %v, want from %v to the bound and the slack, %v, well within one grace, %v",
					m.name, took, m.floor, bound+slack, grace)
			}
		})
	}
}

// TestCloseContextCause requires the cause the caller gives its context to be
// the one CloseContext returns.
func TestCloseContextCause(t *testing.T) {
	t.Parallel()
	p, _ := hang(t, newHungClient(), hungInProcess(serveAgentNil))
	why := errors.New("the test's bound")
	ctx, cancel := context.WithTimeoutCause(context.Background(), testGrace, why)
	defer cancel()
	_, err := within(t, 2*testGrace+slack, "CloseContext", func() error { return p.CloseContext(ctx) })
	if !errors.Is(err, why) || errors.Is(err, context.DeadlineExceeded) || !errors.Is(err, acp.ErrHandlersRunning) {
		t.Errorf("CloseContext = %v, want the cause %q, not context.DeadlineExceeded, and acp.ErrHandlersRunning", err, why)
	}
}

// TestCloseContextNotHandlers requires a context that ends while Wait drains
// a tap, its wait on the connection's goroutines over, to join no
// acp.ErrHandlersRunning: a drain is not a handler, and the grace bounds it.
// The agent has exited 0 of itself, and the connection's goroutines have
// returned, before CloseContext is called.
func TestCloseContextNotHandlers(t *testing.T) {
	t.Parallel()
	tap, release := blockedTap(t)
	p, _ := launchTapped(t, "burst", nil, 2*time.Second, tap, nil)
	t.Cleanup(release)
	if _, err := p.Conn().Initialize(t.Context(), acp.InitializeRequest{}); err != nil {
		t.Fatalf("Initialize = %v", err)
	}
	if err := p.Conn().Connection().SendNotification(t.Context(), "_test/go", map[string]any{}); err != nil {
		t.Fatalf("SendNotification = %v", err)
	}
	ended, cancel := context.WithTimeout(context.Background(), readyWait)
	defer cancel()
	// The connection ends at the agent's output's end, which the watcher
	// closes it at, and its goroutines return.
	if err := p.Conn().Connection().WaitContext(ended); !acp.EndedCleanly(err) {
		t.Fatalf("the connection's WaitContext = %v, want a clean end within %v", err, readyWait)
	}
	select {
	case <-p.Done():
	case <-ended.Done():
		t.Fatalf("the agent had not exited within %v", readyWait)
	}
	const bound = 100 * time.Millisecond
	ctx, cancelBound := context.WithTimeout(context.Background(), bound)
	defer cancelBound()
	took, err := within(t, 2*time.Second+slack, "CloseContext", func() error { return p.CloseContext(ctx) })
	if err == nil || err.Error() != context.DeadlineExceeded.Error() || errors.Is(err, acp.ErrHandlersRunning) {
		t.Errorf("CloseContext = %v after %v, want the bound's cause alone, without acp.ErrHandlersRunning", err, took)
	}
	release()
	if err := p.Wait(); err != nil {
		t.Errorf("Wait after the tap's release = %v, want nil: the agent exited 0", err)
	}
}

// TestCloseContextOneGoroutine requires CloseContext's and KillContext's
// calls, however many, to share WaitContext's one goroutine, and none to be
// left once the handler that held it returns.
func TestCloseContextOneGoroutine(t *testing.T) {
	t.Parallel()
	p, release := hang(t, newHungClient(), hungInProcess(serveAgentNil))
	const label = "TestCloseContextOneGoroutine"
	pprof.Do(context.Background(), pprof.Labels("test", label), func(context.Context) {
		ended, cancel := context.WithCancel(context.Background())
		cancel()
		for range 50 {
			_ = p.CloseContext(ended)
			_ = p.KillContext(ended)
		}
	})
	if n := processWaiters(label); n != 1 {
		t.Errorf("100 calls of CloseContext and KillContext left %d waiting goroutines, want 1", n)
	}
	release()
	deadline := time.Now().Add(slack)
	for processWaiters(label) != 0 {
		if time.Now().After(deadline) {
			t.Fatalf("the waiting goroutine still runs %v after the handler's release", slack)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
