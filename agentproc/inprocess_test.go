package agentproc

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	acp "github.com/maccavelli/acp-go-sdk2"
)

// serveAgent is an agent for an InProcess's Serve: it answers the methods an
// acp.Agent must have, and AgentRefusals refuses the rest.
type serveAgent struct{ acp.AgentRefusals }

func (serveAgent) Initialize(context.Context, acp.InitializeRequest) (acp.InitializeResponse, error) {
	return acp.InitializeResponse{ProtocolVersion: acp.ProtocolVersionNumber}, nil
}

func (serveAgent) Authenticate(context.Context, acp.AuthenticateRequest) (acp.AuthenticateResponse, error) {
	return acp.AuthenticateResponse{}, nil
}

func (serveAgent) NewSession(context.Context, acp.NewSessionRequest) (acp.NewSessionResponse, error) {
	return acp.NewSessionResponse{SessionId: "s"}, nil
}

func (serveAgent) Prompt(context.Context, acp.PromptRequest) (acp.PromptResponse, error) {
	return acp.PromptResponse{StopReason: acp.StopReasonEndTurn}, nil
}

func (serveAgent) Cancel(context.Context, acp.CancelNotification) error { return nil }

func (serveAgent) SetSessionMode(context.Context, acp.SetSessionModeRequest) (acp.SetSessionModeResponse, error) {
	return acp.SetSessionModeResponse{}, nil
}

func (serveAgent) SetSessionConfigOption(context.Context, acp.SetSessionConfigOptionRequest) (acp.SetSessionConfigOptionResponse, error) {
	return acp.SetSessionConfigOptionResponse{}, nil
}

// errServe is what a test's Serve returns of itself.
var errServe = errors.New("serve's own error")

// startInProcess starts a, under testGrace unless its Grace is set, with a
// connection that logs nothing, and closes it when the test ends.
func startInProcess(t *testing.T, a *InProcess) *Process {
	t.Helper()
	if a.Grace == 0 {
		a.Grace = testGrace
	}
	a.ConnectionOptions = append(a.ConnectionOptions, acp.WithLogger(slog.New(slog.DiscardHandler)))
	p, err := a.Start(&testClient{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	return p
}

// readUntilFailure reads in until a read fails with other than io.EOF,
// ignoring the input's end and its context, and returns that failure.
func readUntilFailure(_ context.Context, in io.Reader, _ io.Writer) error {
	buf := make([]byte, 512)
	for {
		switch _, err := in.Read(buf); {
		case err == nil:
		case errors.Is(err, io.EOF):
			time.Sleep(time.Millisecond)
		default:
			return err
		}
	}
}

// TestInProcessEndsAtInputEnd requires an agent that serves until its input
// ends to be stopped by Close's first step, at once, and Wait's error to be
// Serve's (0042-MADR-in-process-agent-runner.md, item 1).
func TestInProcessEndsAtInputEnd(t *testing.T) {
	t.Parallel()
	p := startInProcess(t, &InProcess{Grace: 5 * time.Second, Serve: func(_ context.Context, in io.Reader, out io.Writer) error {
		<-acp.NewAgentSideConnection(serveAgent{}, out, in).Done()
		return errServe
	}})
	if _, err := p.Conn().Initialize(t.Context(), acp.InitializeRequest{ProtocolVersion: acp.ProtocolVersionNumber}); err != nil {
		t.Fatalf("Initialize = %v", err)
	}
	d, err := within(t, slack, "Close", p.Close)
	if !errors.Is(err, errServe) || errors.Is(err, ErrTerminated) || errors.Is(err, ErrKilled) {
		t.Errorf("Close = %v, want Serve's own error alone", err)
	}
	if d >= time.Second {
		t.Errorf("Close took %v: Serve returns once its input ends, in the first step", d)
	}
}

// TestInProcessTerminate requires a Serve that ignores its input's end to be
// stopped by the second step, its context cancelled with ErrTerminated.
func TestInProcessTerminate(t *testing.T) {
	t.Parallel()
	cause := make(chan error, 1)
	p := startInProcess(t, &InProcess{Serve: func(ctx context.Context, _ io.Reader, _ io.Writer) error {
		<-ctx.Done()
		cause <- context.Cause(ctx)
		return errServe
	}})
	d, err := within(t, 2*testGrace+slack, "Close", p.Close)
	if !errors.Is(err, ErrTerminated) || !errors.Is(err, errServe) || errors.Is(err, ErrKilled) {
		t.Errorf("Close = %v, want ErrTerminated joined to Serve's error", err)
	}
	if got := <-cause; !errors.Is(got, ErrTerminated) {
		t.Errorf("Serve's context's cause = %v, want ErrTerminated", got)
	}
	if d < testGrace {
		t.Errorf("Close took %v: the first step waits the grace, %v", d, testGrace)
	}
}

// TestInProcessKill requires a Serve that ignores its input's end and its
// context, and returns only when a read fails, to be stopped by the third
// step, which closes its pipes, and not abandoned.
func TestInProcessKill(t *testing.T) {
	t.Parallel()
	p := startInProcess(t, &InProcess{Serve: readUntilFailure})
	_, err := within(t, 3*testGrace+slack, "Close", p.Close)
	if !errors.Is(err, ErrKilled) || !errors.Is(err, io.ErrClosedPipe) || errors.Is(err, ErrAbandoned) {
		t.Errorf("Close = %v, want ErrKilled joined to Serve's io.ErrClosedPipe, and not ErrAbandoned", err)
	}
}

// TestInProcessAbandon requires a Serve that ignores every step to be
// abandoned within the third step's grace: Close returns, Done is closed,
// and Wait's error has ErrKilled and ErrAbandoned. The test then releases it,
// so that no goroutine outlives the package's tests.
func TestInProcessAbandon(t *testing.T) {
	t.Parallel()
	release, returned := make(chan struct{}), make(chan struct{})
	free := sync.OnceFunc(func() { close(release) })
	p := startInProcess(t, &InProcess{Serve: func(context.Context, io.Reader, io.Writer) error {
		defer close(returned)
		<-release
		return nil
	}})
	// Registered after startInProcess's Close, so it runs first: should
	// Close hang, the test still reports it, and ends.
	t.Cleanup(free)
	d, err := within(t, 3*testGrace+slack, "Close", p.Close)
	if !errors.Is(err, ErrKilled) || !errors.Is(err, ErrAbandoned) {
		t.Errorf("Close = %v, want ErrKilled and ErrAbandoned", err)
	}
	if d < 3*testGrace {
		t.Errorf("Close took %v: it waits three graces, %v each, before it abandons Serve", d, testGrace)
	}
	select {
	case <-p.Done():
	default:
		t.Error("Done is open after an abandonment")
	}
	free()
	select {
	case <-returned:
	case <-time.After(slack):
		t.Fatal("Serve did not return once released")
	}
}

// TestInProcessKillAtOnce requires Kill to skip the first two steps' waits:
// with a grace of 5 s, it returns at once for a Serve that returns when its
// read fails.
func TestInProcessKillAtOnce(t *testing.T) {
	t.Parallel()
	p := startInProcess(t, &InProcess{Grace: 5 * time.Second, Serve: readUntilFailure})
	d, err := within(t, slack, "Kill", p.Kill)
	if !errors.Is(err, ErrKilled) || errors.Is(err, ErrAbandoned) {
		t.Errorf("Kill = %v, want ErrKilled and not ErrAbandoned", err)
	}
	if d >= time.Second {
		t.Errorf("Kill took %v: it waits for no step", d)
	}
}

// TestInProcessReturnsFirst requires ExitedFirst to report whether Serve
// returned while the connection was open.
func TestInProcessReturnsFirst(t *testing.T) {
	t.Parallel()
	t.Run("Serve first", func(t *testing.T) {
		t.Parallel()
		p := startInProcess(t, &InProcess{Serve: func(context.Context, io.Reader, io.Writer) error { return nil }})
		if _, err := within(t, testGrace+slack, "Wait", p.Wait); err != nil {
			t.Errorf("Wait = %v, want nil", err)
		}
		if !p.ExitedFirst() {
			t.Error("ExitedFirst = false, want true: Serve returned while the connection was open")
		}
	})
	t.Run("connection first", func(t *testing.T) {
		t.Parallel()
		p := startInProcess(t, &InProcess{Serve: func(_ context.Context, in io.Reader, _ io.Writer) error {
			_, err := io.Copy(io.Discard, in)
			return err
		}})
		if err := p.Conn().Close(); err != nil {
			t.Fatalf("closing the connection: %v", err)
		}
		if _, err := within(t, testGrace+slack, "Wait", p.Wait); err != nil {
			t.Errorf("Wait = %v, want nil", err)
		}
		if p.ExitedFirst() {
			t.Error("ExitedFirst = true, want false: the connection ended first")
		}
	})
}

// TestInProcessClientWriteAfterReturn requires a request the client sends
// after Serve has returned to fail, not to wait: Serve's input is closed when
// it returns, and an io.Pipe write with no reader would wait for ever
// (0042-PLAN-in-process-agent-runner.md, phase 2, P1).
func TestInProcessClientWriteAfterReturn(t *testing.T) {
	t.Parallel()
	p := startInProcess(t, &InProcess{Serve: func(context.Context, io.Reader, io.Writer) error { return nil }})
	select {
	case <-p.Done():
	case <-time.After(slack):
		t.Fatal("Done is open after Serve returned")
	}
	_, err := within(t, slack, "Initialize", func() error {
		_, err := p.Conn().Initialize(t.Context(), acp.InitializeRequest{ProtocolVersion: acp.ProtocolVersionNumber})
		return err
	})
	if err == nil {
		t.Error("Initialize = nil, want an error: the agent is gone")
	}
}

// TestInProcessEndClosesInput requires the agent's end of its input to be
// closed once Serve has returned, so that a write of the client's fails
// rather than waits for a reader. The connection's own close, which the
// output's end brings at once, closes the client's end too, and a write
// cannot tell the two apart; a read of the agent's end can: io.ErrClosedPipe
// when that end was closed, io.EOF when only the client's was.
func TestInProcessEndClosesInput(t *testing.T) {
	t.Parallel()
	p := startInProcess(t, &InProcess{Serve: func(context.Context, io.Reader, io.Writer) error { return nil }})
	if _, err := within(t, testGrace+slack, "Wait", p.Wait); err != nil {
		t.Fatalf("Wait = %v, want nil", err)
	}
	ag, ok := p.agent.(*inProcessAgent)
	if !ok {
		t.Fatalf("the agent is a %T, want *inProcessAgent", p.agent)
	}
	_, err := within(t, slack, "a read of the agent's input", func() error {
		_, err := ag.inR.Read(make([]byte, 1))
		return err
	})
	if !errors.Is(err, io.ErrClosedPipe) {
		t.Errorf("the read = %v, want io.ErrClosedPipe: the agent's end was not closed", err)
	}
}

// TestInProcessTaps requires the output tap of an in-process agent to
// receive all it wrote before Serve returned, with nothing dropped, while
// the connection's reader is held back, 1 ms before each read, as
// TestCloseReadsTheAgentsLastOutput holds a child's.
func TestInProcessTaps(t *testing.T) {
	t.Parallel()
	var reads atomic.Int64
	tap := &capture{}
	p := startInProcess(t, &InProcess{
		Grace:     5 * time.Second,
		OutputTap: tap,
		Serve: func(_ context.Context, _ io.Reader, out io.Writer) error {
			_, err := out.Write(bulk64())
			return err
		},
		readHook: func() {
			reads.Add(1)
			time.Sleep(time.Millisecond)
		},
	})
	if err := p.Wait(); err != nil {
		t.Fatalf("Wait = %v, want nil", err)
	}
	got, want := tap.bytes(), bulk64()
	if out, _ := p.DroppedTapBytes(); !bytes.Equal(got, want) || out != 0 {
		t.Errorf("the tap received %d bytes and dropped %d of the agent's %d", len(got), out, len(want))
	}
	if reads.Load() == 0 {
		t.Error("the read hook never ran: the tap was not on the connection's reader")
	}
}

// promptAgent is serveAgent whose Prompt says it has started, then waits for
// its context's end and answers cancelled, as a handler that honours it does.
type promptAgent struct {
	serveAgent
	started chan struct{}
}

func (a promptAgent) Prompt(ctx context.Context, _ acp.PromptRequest) (acp.PromptResponse, error) {
	close(a.started)
	<-ctx.Done()
	return acp.PromptResponse{StopReason: acp.StopReasonCancelled}, nil
}

// TestInProcessServeAgentStep2 requires an InProcess agent served by
// acp.ServeAgent, stopped by Close's second step during a prompt, to answer
// the prompt cancelled: the step cancels Serve's context, and ServeAgent
// shuts its connection down gracefully
// (0042-MADR-in-process-agent-runner.md, item 2, S4).
func TestInProcessServeAgentStep2(t *testing.T) {
	t.Parallel()
	agent := promptAgent{started: make(chan struct{})}
	p := startInProcess(t, &InProcess{Serve: func(ctx context.Context, in io.Reader, out io.Writer) error {
		return acp.ServeAgent(ctx, agent, in, out, acp.WithLogger(slog.New(slog.DiscardHandler)))
	}})
	if _, err := p.Conn().Initialize(t.Context(), acp.InitializeRequest{ProtocolVersion: acp.ProtocolVersionNumber}); err != nil {
		t.Fatalf("Initialize = %v", err)
	}
	if _, err := p.Conn().NewSession(t.Context(), acp.NewSessionRequest{Cwd: rootDir(t), McpServers: []acp.McpServer{}}); err != nil {
		t.Fatalf("NewSession = %v", err)
	}
	type result struct {
		r   acp.PromptResponse
		err error
	}
	res := make(chan result, 1)
	go func() {
		r, err := p.Conn().Prompt(context.Background(), acp.PromptRequest{SessionId: "s", Prompt: []acp.ContentBlock{acp.TextBlock("hi")}})
		res <- result{r, err}
	}()
	select {
	case <-agent.started:
	case <-time.After(slack):
		t.Fatal("the prompt's handler did not start")
	}
	_, err := within(t, 3*testGrace+slack, "Close", p.Close)
	if !errors.Is(err, ErrTerminated) || errors.Is(err, ErrKilled) {
		t.Errorf("Close = %v, want ErrTerminated: ServeAgent returns in the second step", err)
	}
	select {
	case got := <-res:
		if got.err != nil || got.r.StopReason != acp.StopReasonCancelled {
			t.Errorf("Prompt = %q, %v; want cancelled, nil", got.r.StopReason, got.err)
		}
	case <-time.After(slack):
		t.Fatal("the client's Prompt did not return")
	}
}

// rootDir is an absolute directory on the host, for a session's cwd.
func rootDir(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

// TestInProcessDetachedTapIsPrefix is TestDetachedTapIsPrefix for an
// in-process agent: its tap blocks until it is detached, is released once the
// connection has read Serve's first 56 MiB, while Serve still has 8 MiB to
// write, and what it received is the stream's beginning, the rest counted as
// dropped (0042-MADR-in-process-agent-runner.md, "Confirmation"). As the
// child test waits for its agent's ready, this waits for the first Write to
// return, so that Close's grace covers the last 8 MiB alone
// (0042-PLAN-in-process-agent-runner.md, phase 11's deviation).
func TestInProcessDetachedTapIsPrefix(t *testing.T) {
	t.Parallel()
	tap, release := blockedTap(t)
	more, wrote := make(chan struct{}), make(chan struct{})
	stream := bulk64()
	const first = 56 << 20
	p := startInProcess(t, &InProcess{Grace: 5 * time.Second, OutputTap: tap, Serve: func(ctx context.Context, _ io.Reader, out io.Writer) error {
		if _, err := out.Write(stream[:first]); err != nil {
			return err
		}
		close(wrote)
		select {
		case <-more:
		case <-ctx.Done():
			return context.Cause(ctx)
		}
		_, err := out.Write(stream[first:])
		return err
	}})
	t.Cleanup(release)
	// An io.Pipe's Write returns once its reader has read all of it.
	select {
	case <-wrote:
	case <-time.After(readyWait):
		t.Fatalf("the connection did not read Serve's first 56 MiB within %v", readyWait)
	}
	deadline := time.Now().Add(readyWait)
	for out, _ := p.DroppedTapBytes(); out == 0; out, _ = p.DroppedTapBytes() {
		if time.Now().After(deadline) {
			t.Fatalf("the tap, blocked through 56 MiB, was not detached within %v", readyWait)
		}
		time.Sleep(10 * time.Millisecond)
	}
	release()
	close(more)
	if err := p.Close(); err != nil {
		t.Fatalf("Close = %v", err)
	}
	got := tap.bytes()
	if !bytes.HasPrefix(stream, got) {
		t.Errorf("the detached tap received %d bytes that are not the stream's beginning", len(got))
	}
	out, _ := p.DroppedTapBytes()
	if total := uint64(len(got)) + out; total != uint64(len(stream)) {
		t.Errorf("the tap received %d bytes and dropped %d, %d in all; Serve wrote %d", len(got), out, total, len(stream))
	}
}

// TestInProcessStartRefuses requires Start to refuse, starting nothing, a nil
// Serve or client.
func TestInProcessStartRefuses(t *testing.T) {
	t.Parallel()
	serve := func(context.Context, io.Reader, io.Writer) error { return nil }
	for _, tc := range []struct {
		name   string
		a      *InProcess
		client acp.Client
		want   string
	}{
		{"nil Serve", &InProcess{}, &testClient{}, "agentproc: InProcess.Serve is nil"},
		{"nil client", &InProcess{Serve: serve}, nil, "agentproc: the client is nil"},
	} {
		p, err := tc.a.Start(tc.client)
		if p != nil || err == nil || err.Error() != tc.want {
			t.Errorf("%s: Start = %v, %v; want nil, %q", tc.name, p, err, tc.want)
		}
	}
}
