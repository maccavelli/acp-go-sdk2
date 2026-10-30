// Command remote serves an ACP agent over the network, or dials one, through
// package remote: Streamable HTTP for http:// and https:// URLs, WebSocket for
// ws:// and wss://.
//
//	remote serve -addr 127.0.0.1:7331
//	remote dial http://127.0.0.1:7331/acp
//	remote dial ws://127.0.0.1:7331/acp
//
// serve answers each prompt with its text echoed, and keeps every session's
// updates in memory, shared by every connection, so a later connection can
// load the session. dial runs initialize, session/new and a prompt of
// "hello", printing each agent message chunk's text and then the stop reason.
// When the agent advertises loadSession, dial connects again and loads the
// session, printing how many updates the agent replayed.
package main

import (
	"context"
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/maccavelli/acp-go-sdk2"
	"github.com/maccavelli/acp-go-sdk2/remote"
)

// exchangeBound bounds all of dial's exchanges, the load included.
const exchangeBound = 30 * time.Second

// replayBound is how long dial waits, after session/load is answered, for the
// replay it expects. Over Streamable HTTP the replay comes on the session's
// stream and the answer on the connection's, with no order between them, so
// the answer may be read first.
const replayBound = 5 * time.Second

// shutdownBound bounds serve's Handler.Close and http.Server.Shutdown.
const shutdownBound = 10 * time.Second

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "remote:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: remote serve -addr ADDR | remote dial URL")
	}
	switch args[0] {
	case "serve":
		fs := flag.NewFlagSet("serve", flag.ContinueOnError)
		addr := fs.String("addr", "127.0.0.1:7331", "the address to listen on")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		ln, err := net.Listen("tcp", *addr)
		if err != nil {
			return err
		}
		return serve(ctx, ln, stdout)
	case "dial":
		if len(args) != 2 {
			return errors.New("usage: remote dial URL")
		}
		return dial(ctx, args[1], stdout)
	default:
		return fmt.Errorf("unknown mode %q: want serve or dial", args[0])
	}
}

// serve serves the echo agent at /acp on ln until ctx ends. It then closes
// the handler, which ends every ACP connection and its streams, before
// shutting the server down: Shutdown waits for active connections, and an
// open event stream is one until the handler ends it.
func serve(ctx context.Context, ln net.Listener, stdout io.Writer) error {
	store := &store{sessions: map[acp.SessionId]*session{}}
	h := remote.NewHandler(func(ctx context.Context, in io.Reader, out io.Writer) error {
		return acp.ServeAgent(ctx, &echoAgent{store: store}, in, out)
	}, nil)
	mux := http.NewServeMux()
	mux.Handle("/acp", h)
	// Unencrypted HTTP/2 as well as HTTP/1.1, so a client that speaks
	// HTTP/2 with prior knowledge over http:// is served.
	var protocols http.Protocols
	protocols.SetHTTP1(true)
	protocols.SetUnencryptedHTTP2(true)
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second, Protocols: &protocols}
	served := make(chan error, 1)
	go func() { served <- srv.Serve(ln) }()
	if _, err := fmt.Fprintf(stdout, "serving ACP at http://%s/acp and ws://%s/acp\n", ln.Addr(), ln.Addr()); err != nil {
		_ = srv.Close() //nolint:errcheck // The write's error is the one returned.
		return err
	}
	select {
	case err := <-served:
		return err
	case <-ctx.Done():
	}
	bound, cancel := context.WithTimeout(context.Background(), shutdownBound)
	defer cancel()
	return errors.Join(h.Close(bound), srv.Shutdown(bound))
}

// store keeps every session's updates, shared by every connection.
type store struct {
	mu       sync.Mutex
	sessions map[acp.SessionId]*session
}

type session struct {
	history []acp.SessionNotification
}

// echoAgent is one connection's agent.
type echoAgent struct {
	acp.AgentRefusals
	conn  *acp.AgentSideConnection
	store *store
}

var (
	_ acp.Agent       = (*echoAgent)(nil)
	_ acp.AgentLoader = (*echoAgent)(nil)
)

// SetAgentConnection implements acp.AgentConnAware.
func (a *echoAgent) SetAgentConnection(conn *acp.AgentSideConnection) { a.conn = conn }

// Initialize advertises loadSession.
func (a *echoAgent) Initialize(context.Context, acp.InitializeRequest) (acp.InitializeResponse, error) {
	return acp.InitializeResponse{
		ProtocolVersion:   acp.ProtocolVersionNumber,
		AgentCapabilities: acp.AgentCapabilities{LoadSession: true},
	}, nil
}

// Authenticate accepts every client: authentication is the host's, before
// the handler.
func (a *echoAgent) Authenticate(context.Context, acp.AuthenticateRequest) (acp.AuthenticateResponse, error) {
	return acp.AuthenticateResponse{}, nil
}

// NewSession makes a session with no history.
func (a *echoAgent) NewSession(context.Context, acp.NewSessionRequest) (acp.NewSessionResponse, error) {
	id := acp.SessionId(rand.Text())
	a.store.mu.Lock()
	a.store.sessions[id] = &session{}
	a.store.mu.Unlock()
	return acp.NewSessionResponse{SessionId: id}, nil
}

// LoadSession replays the session's updates, then answers.
func (a *echoAgent) LoadSession(ctx context.Context, params acp.LoadSessionRequest) (acp.LoadSessionResponse, error) {
	a.store.mu.Lock()
	s, ok := a.store.sessions[params.SessionId]
	var history []acp.SessionNotification
	if ok {
		history = append(history, s.history...)
	}
	a.store.mu.Unlock()
	if !ok {
		return acp.LoadSessionResponse{}, acp.NewInvalidParams(map[string]any{"error": "unknown session"})
	}
	for _, n := range history {
		if err := a.conn.SessionUpdate(ctx, n); err != nil {
			return acp.LoadSessionResponse{}, err
		}
	}
	return acp.LoadSessionResponse{}, nil
}

// Prompt sends each text block back as an agent message chunk, "echo: "
// before it, and keeps each in the session's history.
func (a *echoAgent) Prompt(ctx context.Context, params acp.PromptRequest) (acp.PromptResponse, error) {
	a.store.mu.Lock()
	s, ok := a.store.sessions[params.SessionId]
	a.store.mu.Unlock()
	if !ok {
		return acp.PromptResponse{}, acp.NewInvalidParams(map[string]any{"error": "unknown session"})
	}
	for _, b := range params.Prompt {
		if b.Text == nil {
			continue
		}
		n := acp.SessionNotification{SessionId: params.SessionId, Update: acp.UpdateAgentMessageText("echo: " + b.Text.Text)}
		a.store.mu.Lock()
		s.history = append(s.history, n)
		a.store.mu.Unlock()
		if err := a.conn.SessionUpdate(ctx, n); err != nil {
			return acp.PromptResponse{}, err
		}
	}
	return acp.PromptResponse{StopReason: acp.StopReasonEndTurn}, nil
}

// Cancel has nothing to stop: a turn here never waits.
func (a *echoAgent) Cancel(context.Context, acp.CancelNotification) error { return nil }

// SetSessionMode refuses session/set_mode: the agent offers no modes.
func (a *echoAgent) SetSessionMode(context.Context, acp.SetSessionModeRequest) (acp.SetSessionModeResponse, error) {
	return acp.SetSessionModeResponse{}, acp.NewMethodNotFound(acp.AgentMethodSessionSetMode)
}

// SetSessionConfigOption refuses session/set_config_option: the agent offers
// no options.
func (a *echoAgent) SetSessionConfigOption(context.Context, acp.SetSessionConfigOptionRequest) (acp.SetSessionConfigOptionResponse, error) {
	return acp.SetSessionConfigOptionResponse{}, acp.NewMethodNotFound(acp.AgentMethodSessionSetConfigOption)
}

// dial runs the exchange against the agent at url, and loads the session on
// a second connection when the agent advertises loadSession.
func dial(ctx context.Context, url string, stdout io.Writer) error {
	ctx, cancel := context.WithTimeout(ctx, exchangeBound)
	defer cancel()
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	c := newDialClient(stdout, true)
	conn, closeConn, err := connect(ctx, url, c)
	if err != nil {
		return err
	}
	init, err := conn.Initialize(ctx, acp.InitializeRequest{ProtocolVersion: acp.ProtocolVersionNumber})
	if err != nil {
		closeConn()
		return fmt.Errorf("initialize: %w", err)
	}
	sess, err := conn.NewSession(ctx, acp.NewSessionRequest{Cwd: cwd})
	if err != nil {
		closeConn()
		return fmt.Errorf("session/new: %w", err)
	}
	res, err := conn.Prompt(ctx, acp.PromptRequest{SessionId: sess.SessionId, Prompt: []acp.ContentBlock{acp.TextBlock("hello")}})
	closeConn()
	if err != nil {
		return fmt.Errorf("session/prompt: %w", err)
	}
	if _, err := fmt.Fprintf(stdout, "stop reason: %s\n", res.StopReason); err != nil {
		return err
	}
	if res.StopReason != acp.StopReasonEndTurn {
		return fmt.Errorf("the turn ended %s, want end_turn", res.StopReason)
	}
	if !init.AgentCapabilities.LoadSession {
		return nil
	}
	return load(ctx, url, cwd, sess.SessionId, c.count(), stdout)
}

// load loads id on a new connection and prints how many updates the agent
// replayed, waiting up to replayBound for the want it saw on the first.
func load(ctx context.Context, url, cwd string, id acp.SessionId, want int, stdout io.Writer) error {
	c := newDialClient(stdout, false)
	conn, closeConn, err := connect(ctx, url, c)
	if err != nil {
		return err
	}
	defer closeConn()
	if _, err := conn.Initialize(ctx, acp.InitializeRequest{ProtocolVersion: acp.ProtocolVersionNumber}); err != nil {
		return fmt.Errorf("initialize: %w", err)
	}
	if _, err := conn.LoadSession(ctx, acp.LoadSessionRequest{SessionId: id, Cwd: cwd}); err != nil {
		return fmt.Errorf("session/load: %w", err)
	}
	n := c.wait(want, replayBound)
	if _, err := fmt.Fprintf(stdout, "loaded: %d updates\n", n); err != nil {
		return err
	}
	if n == 0 {
		return errors.New("the load replayed no update")
	}
	return nil
}

// connect dials url and joins c to it. closeConn closes the Conn and waits
// for the connection to end.
func connect(ctx context.Context, url string, c acp.Client) (*acp.ClientSideConnection, func(), error) {
	rc, err := remote.Dial(ctx, url, nil)
	if err != nil {
		return nil, nil, err
	}
	conn := acp.NewClientSideConnection(c, rc, rc)
	return conn, func() {
		_ = rc.Close() //nolint:errcheck // The exchange is over; a failed DELETE leaves the server's idle bound to end it.
		<-conn.Done()
	}, nil
}

// dialClient prints agent message chunks when show is set, counts every
// update, and answers a permission request with its first option.
type dialClient struct {
	acp.ClientRefusals
	out  io.Writer
	show bool

	mu      sync.Mutex
	n       int
	changed chan struct{}
}

func newDialClient(out io.Writer, show bool) *dialClient {
	return &dialClient{out: out, show: show, changed: make(chan struct{})}
}

// SessionUpdate implements acp.Client.
func (c *dialClient) SessionUpdate(_ context.Context, n acp.SessionNotification) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.n++
	close(c.changed)
	c.changed = make(chan struct{})
	if chunk := n.Update.AgentMessageChunk; c.show && chunk != nil && chunk.Content.Text != nil {
		_, err := fmt.Fprintln(c.out, chunk.Content.Text.Text)
		return err
	}
	return nil
}

// RequestPermission implements acp.Client.
func (c *dialClient) RequestPermission(_ context.Context, p acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
	if len(p.Options) == 0 {
		return acp.RequestPermissionResponse{Outcome: acp.NewRequestPermissionOutcomeCancelled()}, nil
	}
	return acp.RequestPermissionResponse{Outcome: acp.NewRequestPermissionOutcomeSelected(p.Options[0].OptionId)}, nil
}

func (c *dialClient) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n
}

// wait returns the count once it reaches want, or when bound passes.
func (c *dialClient) wait(want int, bound time.Duration) int {
	t := time.NewTimer(bound)
	defer t.Stop()
	for {
		c.mu.Lock()
		n, changed := c.n, c.changed
		c.mu.Unlock()
		if n >= want {
			return n
		}
		select {
		case <-changed:
		case <-t.C:
			return n
		}
	}
}
