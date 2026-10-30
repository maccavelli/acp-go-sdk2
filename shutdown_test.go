package acp

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"
)

// shutdownAgent answers the handshake; its Prompt calls prompt, when set, and
// otherwise ends its turn.
type shutdownAgent struct {
	AgentRefusals
	conn   *AgentSideConnection
	prompt func(ctx context.Context, a *shutdownAgent) (PromptResponse, error)
}

func (a *shutdownAgent) SetAgentConnection(c *AgentSideConnection) { a.conn = c }

func (*shutdownAgent) Initialize(context.Context, InitializeRequest) (InitializeResponse, error) {
	return InitializeResponse{ProtocolVersion: ProtocolVersionNumber}, nil
}

func (*shutdownAgent) Authenticate(context.Context, AuthenticateRequest) (AuthenticateResponse, error) {
	return AuthenticateResponse{}, nil
}

func (*shutdownAgent) NewSession(context.Context, NewSessionRequest) (NewSessionResponse, error) {
	return NewSessionResponse{SessionId: "s"}, nil
}

func (a *shutdownAgent) Prompt(ctx context.Context, _ PromptRequest) (PromptResponse, error) {
	if a.prompt != nil {
		return a.prompt(ctx, a)
	}
	return PromptResponse{StopReason: StopReasonEndTurn}, nil
}

func (*shutdownAgent) Cancel(context.Context, CancelNotification) error { return nil }

func (*shutdownAgent) SetSessionMode(context.Context, SetSessionModeRequest) (SetSessionModeResponse, error) {
	return SetSessionModeResponse{}, nil
}

func (*shutdownAgent) SetSessionConfigOption(context.Context, SetSessionConfigOptionRequest) (SetSessionConfigOptionResponse, error) {
	return SetSessionConfigOptionResponse{}, nil
}

// shutdownClient answers a permission request with the first option, and
// counts the requests.
type shutdownClient struct {
	ClientRefusals
	mu    sync.Mutex
	asked int
}

func (*shutdownClient) SessionUpdate(context.Context, SessionNotification) error { return nil }

func (c *shutdownClient) RequestPermission(_ context.Context, p RequestPermissionRequest) (RequestPermissionResponse, error) {
	c.mu.Lock()
	c.asked++
	c.mu.Unlock()
	return RequestPermissionResponse{Outcome: NewRequestPermissionOutcomeSelected(p.Options[0].OptionId)}, nil
}

// shutdownPipes is a client's connection and the two pipes it shares with
// an agent: the agent reads agentIn and writes agentOut.
type shutdownPipes struct {
	agentIn  *io.PipeReader
	inW      *io.PipeWriter
	agentOut *io.PipeWriter
	client   *ClientSideConnection
}

// newShutdownPipes connects a shutdownClient to pipes for an agent, and
// closes the pipes and the client when the test ends.
func newShutdownPipes(t *testing.T, client Client) shutdownPipes {
	t.Helper()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	p := shutdownPipes{agentIn: inR, inW: inW, agentOut: outW,
		client: NewClientSideConnection(client, inW, outR, WithLogger(slog.New(slog.DiscardHandler)))}
	t.Cleanup(func() {
		_ = p.client.Close()
		_ = inR.Close()
		_ = outW.Close()
	})
	return p
}

// handshake initializes the client and opens session "s".
func handshake(t *testing.T, c *ClientSideConnection) {
	t.Helper()
	if _, err := c.Initialize(t.Context(), InitializeRequest{ProtocolVersion: ProtocolVersionNumber}); err != nil {
		t.Fatalf("Initialize = %v", err)
	}
	if _, err := c.NewSession(t.Context(), NewSessionRequest{Cwd: testAbsPath(), McpServers: []McpServer{}}); err != nil {
		t.Fatalf("NewSession = %v", err)
	}
}

// promptResult is what a client's Prompt returned.
type promptResult struct {
	r   PromptResponse
	err error
}

// startPrompt sends a prompt on its own goroutine.
func startPrompt(c *ClientSideConnection) <-chan promptResult {
	res := make(chan promptResult, 1)
	go func() {
		r, err := c.Prompt(context.Background(), PromptRequest{SessionId: "s", Prompt: []ContentBlock{TextBlock("hi")}})
		res <- promptResult{r, err}
	}()
	return res
}

// await returns what ch holds, failing the test after five seconds.
func await[T any](t *testing.T, ch <-chan T, what string) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(5 * time.Second):
		t.Fatalf("%s: nothing within 5 s", what)
	}
	var zero T
	return zero
}

// TestShutdownAnswersRunningPrompt requires Shutdown to let a prompt whose
// handler honours its context be answered cancelled, the handler's cause
// being ErrShutdown, and then to close the connection, which ended cleanly
// (0042-MADR-in-process-agent-runner.md, item 2).
func TestShutdownAnswersRunningPrompt(t *testing.T) {
	t.Parallel()
	started, cause := make(chan struct{}), make(chan error, 1)
	agent := &shutdownAgent{prompt: func(ctx context.Context, _ *shutdownAgent) (PromptResponse, error) {
		close(started)
		<-ctx.Done()
		cause <- context.Cause(ctx)
		return PromptResponse{StopReason: StopReasonCancelled}, nil
	}}
	p := newShutdownPipes(t, &shutdownClient{})
	as := NewAgentSideConnection(agent, p.agentOut, p.agentIn, WithLogger(slog.New(slog.DiscardHandler)))
	handshake(t, p.client)
	res := startPrompt(p.client)
	await(t, started, "the prompt's handler")
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	if err := as.Connection().Shutdown(ctx); err != nil {
		t.Errorf("Shutdown = %v, want nil", err)
	}
	got := await(t, res, "the client's Prompt")
	if got.err != nil || got.r.StopReason != StopReasonCancelled {
		t.Errorf("Prompt = %q, %v; want cancelled, nil", got.r.StopReason, got.err)
	}
	if c := await(t, cause, "the handler's cause"); !errors.Is(c, ErrShutdown) {
		t.Errorf("the handler's cause = %v, want ErrShutdown", c)
	}
	end := as.Connection().Err()
	if !errors.Is(end, ErrShutdown) || !errors.Is(end, ErrConnectionClosed) || !EndedCleanly(end) {
		t.Errorf("the agent's Err = %v, want ErrConnectionClosed wrapping ErrShutdown, a clean end", end)
	}
}

// TestShutdownRefusesLateRequest requires a request received during Shutdown
// to be answered -32603 at once, and the answer to the agent's own request,
// made after its handler's context ended, still to arrive.
func TestShutdownRefusesLateRequest(t *testing.T) {
	t.Parallel()
	started, late := make(chan struct{}), make(chan struct{})
	asked := make(chan error, 1)
	agent := &shutdownAgent{prompt: func(ctx context.Context, a *shutdownAgent) (PromptResponse, error) {
		close(started)
		<-ctx.Done()
		<-late
		_, err := a.conn.RequestPermission(context.Background(), RequestPermissionRequest{
			SessionId: "s",
			ToolCall:  ToolCallUpdate{ToolCallId: "t"},
			Options:   []PermissionOption{{OptionId: "ok", Name: "Allow", Kind: PermissionOptionKindAllowOnce}},
		})
		asked <- err
		return PromptResponse{StopReason: StopReasonCancelled}, nil
	}}
	client := &shutdownClient{}
	p := newShutdownPipes(t, client)
	as := NewAgentSideConnection(agent, p.agentOut, p.agentIn, WithLogger(slog.New(slog.DiscardHandler)))
	handshake(t, p.client)
	res := startPrompt(p.client)
	await(t, started, "the prompt's handler")
	shut := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		shut <- as.Connection().Shutdown(ctx)
	}()
	var lateErr error
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(5 * time.Millisecond) {
		_, lateErr = p.client.NewSession(t.Context(), NewSessionRequest{Cwd: testAbsPath(), McpServers: []McpServer{}})
		if lateErr != nil {
			break
		}
	}
	var re *RequestError
	if !errors.As(lateErr, &re) || re.Code != -32603 || !strings.Contains(re.Error(), "shutting down") {
		t.Errorf("the late request = %v, want -32603 saying the connection is shutting down", lateErr)
	}
	close(late)
	if err := await(t, asked, "the agent's permission request"); err != nil {
		t.Errorf("the agent's RequestPermission during Shutdown = %v, want its answer", err)
	}
	if got := await(t, res, "the client's Prompt"); got.err != nil || got.r.StopReason != StopReasonCancelled {
		t.Errorf("Prompt = %q, %v; want cancelled, nil", got.r.StopReason, got.err)
	}
	if err := await(t, shut, "Shutdown"); err != nil {
		t.Errorf("Shutdown = %v, want nil", err)
	}
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.asked != 1 {
		t.Errorf("the client was asked %d times, want 1", client.asked)
	}
}

// TestShutdownBound requires Shutdown, when a handler ignores its context,
// to close the connection once its own context ends, and to return that
// context's cause.
func TestShutdownBound(t *testing.T) {
	t.Parallel()
	started, release := make(chan struct{}), make(chan struct{})
	agent := &shutdownAgent{prompt: func(context.Context, *shutdownAgent) (PromptResponse, error) {
		close(started)
		<-release
		return PromptResponse{StopReason: StopReasonEndTurn}, nil
	}}
	t.Cleanup(sync.OnceFunc(func() { close(release) }))
	p := newShutdownPipes(t, &shutdownClient{})
	as := NewAgentSideConnection(agent, p.agentOut, p.agentIn, WithLogger(slog.New(slog.DiscardHandler)))
	handshake(t, p.client)
	_ = startPrompt(p.client)
	await(t, started, "the prompt's handler")
	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer cancel()
	begin := time.Now()
	err := as.Connection().Shutdown(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Shutdown = %v, want its context's cause, context.DeadlineExceeded", err)
	}
	if d := time.Since(begin); d > 3*time.Second {
		t.Errorf("Shutdown took %v, past its context's 200 ms", d)
	}
	select {
	case <-as.Done():
	default:
		t.Error("the connection is open after Shutdown returned")
	}
}

// serveAgentAsync runs ServeAgent over p's pipes on its own goroutine.
func serveAgentAsync(ctx context.Context, agent Agent, p shutdownPipes) <-chan error {
	done := make(chan error, 1)
	go func() {
		done <- ServeAgent(ctx, agent, p.agentIn, p.agentOut, WithLogger(slog.New(slog.DiscardHandler)))
	}()
	return done
}

// TestServeAgentInputEnd requires ServeAgent to return nil at a clean end of
// its input, the request it was running answered first.
func TestServeAgentInputEnd(t *testing.T) {
	t.Parallel()
	started, finish := make(chan struct{}), make(chan struct{})
	agent := &shutdownAgent{prompt: func(context.Context, *shutdownAgent) (PromptResponse, error) {
		close(started)
		<-finish
		return PromptResponse{StopReason: StopReasonEndTurn}, nil
	}}
	p := newShutdownPipes(t, &shutdownClient{})
	served := serveAgentAsync(t.Context(), agent, p)
	handshake(t, p.client)
	res := startPrompt(p.client)
	await(t, started, "the prompt's handler")
	_ = p.inW.Close() // the end of the agent's input
	close(finish)
	if got := await(t, res, "the client's Prompt"); got.err != nil || got.r.StopReason != StopReasonEndTurn {
		t.Errorf("Prompt = %q, %v; want end_turn, nil", got.r.StopReason, got.err)
	}
	if err := await(t, served, "ServeAgent"); err != nil {
		t.Errorf("ServeAgent = %v, want nil at a clean end of input", err)
	}
}

// TestServeAgentContextEnd requires ServeAgent, when its context ends during
// a prompt, to have the prompt answered cancelled and to return the
// context's cause: the lost answer of the PLAN's phase 2, now given.
func TestServeAgentContextEnd(t *testing.T) {
	t.Parallel()
	started := make(chan struct{})
	agent := &shutdownAgent{prompt: func(ctx context.Context, _ *shutdownAgent) (PromptResponse, error) {
		close(started)
		<-ctx.Done()
		return PromptResponse{StopReason: StopReasonCancelled}, nil
	}}
	p := newShutdownPipes(t, &shutdownClient{})
	stop := errors.New("the test's signal")
	ctx, cancel := context.WithCancelCause(t.Context())
	served := serveAgentAsync(ctx, agent, p)
	handshake(t, p.client)
	res := startPrompt(p.client)
	await(t, started, "the prompt's handler")
	cancel(stop)
	if got := await(t, res, "the client's Prompt"); got.err != nil || got.r.StopReason != StopReasonCancelled {
		t.Errorf("Prompt = %q, %v; want cancelled, nil", got.r.StopReason, got.err)
	}
	if err := await(t, served, "ServeAgent"); !errors.Is(err, stop) {
		t.Errorf("ServeAgent = %v, want the context's cause", err)
	}
}

// TestServeAgentHandlersStillRun requires ServeAgent's error to wrap
// ErrHandlersRunning when its 5 s bound passes with a prompt handler that
// ignores its context still running: at its input's end, with the text it
// has always had; and at its context's end, beside the context's cause. Each
// runs in a synctest bubble, so the bounds pass on the bubble's clock, and
// releases its handler before the bubble ends
// (0042-MADR-in-process-agent-runner.md, the amendment of 2026-10-30, X2).
func TestServeAgentHandlersStillRun(t *testing.T) {
	t.Parallel()
	// serve starts ServeAgent under ctx with a prompt that runs until
	// released, and returns once the prompt's handler runs.
	serve := func(t *testing.T, ctx context.Context, release <-chan struct{}) (shutdownPipes, <-chan error) {
		t.Helper()
		started := make(chan struct{})
		agent := &shutdownAgent{prompt: func(context.Context, *shutdownAgent) (PromptResponse, error) {
			close(started)
			<-release
			return PromptResponse{StopReason: StopReasonEndTurn}, nil
		}}
		p := newShutdownPipes(t, &shutdownClient{})
		served := serveAgentAsync(ctx, agent, p)
		handshake(t, p.client)
		startPrompt(p.client)
		<-started
		return p, served
	}
	t.Run("the input's end", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			release := make(chan struct{})
			p, served := serve(t, t.Context(), release)
			_ = p.inW.Close() // the end of the agent's input
			err := <-served
			close(release)
			const want = "acp: the connection's handlers still run 5s after its end"
			if !errors.Is(err, ErrHandlersRunning) || err.Error() != want {
				t.Errorf("ServeAgent = %v, want %q, wrapping ErrHandlersRunning", err, want)
			}
		})
	})
	t.Run("the context's end", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			release := make(chan struct{})
			stop := errors.New("the test's signal")
			ctx, cancel := context.WithCancelCause(t.Context())
			_, served := serve(t, ctx, release)
			cancel(stop)
			err := <-served
			close(release)
			if !errors.Is(err, stop) || !errors.Is(err, ErrHandlersRunning) || !errors.Is(err, context.DeadlineExceeded) ||
				!strings.Contains(err.Error(), "acp: the connection's handlers still run 5s after the context's end") {
				t.Errorf("ServeAgent = %v, want the context's cause and ErrHandlersRunning, the 5 s bound passed", err)
			}
		})
	})
}
