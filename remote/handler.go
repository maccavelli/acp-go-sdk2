package remote

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
)

// connectionIDHeader names the connection in every response that starts one,
// and in every later request of Streamable HTTP.
const connectionIDHeader = "Acp-Connection-Id"

// HandlerOptions configure NewHandler. The zero value, or nil, is ready to
// use.
type HandlerOptions struct {
	// Grace is how long each step of a connection's ending waits for its serve
	// to return, and the bound on each WebSocket message written to a client.
	// Zero or less is 5 seconds.
	Grace time.Duration

	// KeepAlive is how often a connection is pinged, whether messages flow
	// or not: a WebSocket ping, or a comment on each event stream. It is also
	// how long a WebSocket ping waits for its answer.
	// Zero or less is 15 seconds, the official servers' interval
	// (0043-MADR-remote-http-and-websocket-transport.md, "Amended by the PLAN,
	// 2026-10-29", 8).
	KeepAlive time.Duration

	// MaxMessageSize is the largest message, in bytes, a connection reads or
	// writes. Zero or less is 10 MiB, the connection's own default.
	MaxMessageSize int

	// MaxBuffered is how many bytes of messages a Streamable HTTP connection
	// holds for streams no client is receiving before it ends with
	// ErrOverflow. Zero or less is 64 MiB.
	MaxBuffered int

	// OriginPatterns are the host patterns of the browser origins allowed
	// besides the request's own host, as websocket.AcceptOptions'
	// OriginPatterns are matched. A request without an Origin header, as a
	// program sends, is always allowed.
	OriginPatterns []string

	// Logger receives what the handler cannot return: a refused request, a
	// serve's error, an abandonment. It logs byte counts and fixed labels,
	// never a message, an id or a session id. Nil is slog.Default().
	Logger *slog.Logger
}

// Handler serves ACP's remote transport: an http.Handler for the RFD's single
// endpoint, which runs a serve function for each connection a client opens.
type Handler struct {
	serve func(ctx context.Context, in io.Reader, out io.Writer) error
	opts  HandlerOptions
	log   *slog.Logger

	mu        sync.Mutex
	closed    bool
	conns     map[*serverConn]struct{}
	httpConns map[string]*httpConn // the Streamable HTTP connections, by Acp-Connection-Id
	active    sync.WaitGroup       // each connection, from its start to its ending's end

	abandoned atomic.Int64

	// initBound and idleBound are the Streamable HTTP bounds: how long
	// initialize waits for its answer, and how long a connection lives with no
	// connection-scoped stream (0043-MADR-remote-http-and-websocket-transport.md,
	// "Amended by the PLAN, 2026-10-29", 3 and 9). Only this package's tests
	// change them.
	initBound, idleBound time.Duration
	// inboxLimit is how many lines a Streamable HTTP connection queues for its
	// agent's input before a POST waits for room
	// (0043-PLAN-remote-http-and-websocket-transport.md, "Deviation,
	// 2026-10-30, phase 7's 202 after the agent's answer"). Only this package's
	// tests change it.
	inboxLimit int
}

// NewHandler returns a Handler that runs serve once for each connection, on
// a goroutine of its own, as a program's main serves os.Stdin and os.Stdout:
// in carries the client's messages, one line each, and out the agent's.
// acp.ServeAgent is the usual body. serve's ctx ends, with a cause
// context.Cause reports, when the connection ends and serve has not returned
// at its input's end. NewHandler panics if serve is nil.
func NewHandler(serve func(ctx context.Context, in io.Reader, out io.Writer) error, opts *HandlerOptions) *Handler {
	if serve == nil {
		panic("remote: NewHandler: nil serve")
	}
	o := resolveHandlerOptions(opts)
	return &Handler{
		serve: serve, opts: o, log: o.Logger, conns: make(map[*serverConn]struct{}), httpConns: make(map[string]*httpConn),
		initBound: 30 * time.Second, idleBound: 60 * time.Second, inboxLimit: queueLimit,
	}
}

// resolveHandlerOptions returns opts with its defaults set.
func resolveHandlerOptions(opts *HandlerOptions) HandlerOptions {
	var o HandlerOptions
	if opts != nil {
		o = *opts
	}
	if o.Grace <= 0 {
		o.Grace = 5 * time.Second
	}
	if o.KeepAlive <= 0 {
		o.KeepAlive = 15 * time.Second
	}
	if o.MaxMessageSize <= 0 {
		o.MaxMessageSize = defaultMaxMessageSize
	}
	if o.MaxBuffered <= 0 {
		o.MaxBuffered = 64 * 1024 * 1024
	}
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	return o
}

// ServeHTTP serves one request to the endpoint, as the RFD routes it: a GET
// with "Upgrade: websocket" opens a WebSocket connection; a POST sends a
// message, initialize opening a Streamable HTTP connection; a GET opens one of
// its event streams; a DELETE ends it. A POST is answered 202 once its
// message is queued for the connection's serve, before serve reads it, so the
// answer cannot arrive after the agent's reply; while 256 messages wait, a
// POST waits for room. Any other method is answered 405, a browser's CORS
// preflight included, which is the host's middleware. After Close, every
// request is answered 503.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.isClosed() {
		h.refuse(w, http.StatusServiceUnavailable, "remote: the handler is closed")
		return
	}
	if r.Method == http.MethodGet && strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		h.serveWebSocket(w, r)
		return
	}
	switch r.Method {
	case http.MethodPost, http.MethodGet, http.MethodDelete:
	default:
		w.Header().Set("Allow", "GET, POST, DELETE")
		h.refuse(w, http.StatusMethodNotAllowed, "remote: the method is not one the endpoint serves")
		return
	}
	if !originAllowed(r, h.opts.OriginPatterns) {
		h.refuse(w, http.StatusForbidden, "remote: the request's origin is not allowed")
		return
	}
	switch r.Method {
	case http.MethodPost:
		h.servePost(w, r)
	case http.MethodGet:
		h.serveStream(w, r)
	default:
		h.serveDelete(w, r)
	}
}

// originAllowed applies the WebSocket upgrade's origin rule to a request
// (0043-MADR-remote-http-and-websocket-transport.md, "Amended by the PLAN,
// 2026-10-29", 7): a request with no Origin passes, as a program sends none;
// so does one whose Origin's host is the request's Host, or matches one of
// patterns, against the Origin's scheme and host when the pattern holds
// "://", as websocket.Accept matches them.
func originAllowed(r *http.Request, patterns []string) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	if strings.EqualFold(r.Host, u.Host) {
		return true
	}
	for _, p := range patterns {
		target := u.Host
		if strings.Contains(p, "://") {
			target = u.Scheme + "://" + u.Host
		}
		if ok, err := path.Match(strings.ToLower(p), strings.ToLower(target)); err == nil && ok {
			return true
		}
	}
	return false
}

// refuse answers a request with status and text, and logs it.
func (h *Handler) refuse(w http.ResponseWriter, status int, text string) {
	h.log.Info("remote: request refused", "status", status)
	http.Error(w, text, status)
}

// serveWebSocket upgrades the request and serves the connection until it
// ends.
func (h *Handler) serveWebSocket(w http.ResponseWriter, r *http.Request) {
	w.Header().Set(connectionIDHeader, rand.Text())
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: h.opts.OriginPatterns})
	if err != nil {
		// Accept has answered the request.
		h.log.Info("remote: WebSocket upgrade refused")
		return
	}
	c.SetReadLimit(int64(h.opts.MaxMessageSize))
	sc, ok := h.start()
	if !ok {
		_ = c.Close(websocket.StatusGoingAway, "the handler is closed") //nolint:errcheck // The client is turned away; nothing is left to report.
		return
	}
	defer h.finish(sc)
	sc.runWebSocket(c)
}

// Close ends every connection, as at its client's end: each serve's input
// is closed, then its context cancelled with ErrHandlerClosed, then its pipes
// closed, each step waiting up to Grace. A request after Close is answered
// 503. A WebSocket's close handshake, after its serve has returned, waits up
// to 5 seconds, websocket.Conn.Close's own bound, for a client that does not
// answer it. Close waits for every connection to end, or for ctx; it returns
// context.Cause(ctx) when ctx ends first, and an error wrapping ErrAbandoned
// when a serve had to be abandoned. A second Close waits as the first does.
// Call it before http.Server.Shutdown, which waits for every open connection.
func (h *Handler) Close(ctx context.Context) error {
	h.mu.Lock()
	if !h.closed {
		h.closed = true
		for sc := range h.conns {
			sc.end(ErrHandlerClosed)
		}
	}
	h.mu.Unlock()
	done := make(chan struct{})
	go func() {
		h.active.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		return context.Cause(ctx)
	}
	if n := h.abandoned.Load(); n > 0 {
		return fmt.Errorf("%w: %d serve functions still running", ErrAbandoned, n)
	}
	return nil
}

// isClosed reports whether Close has been called.
func (h *Handler) isClosed() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.closed
}

// start starts a serve for a new connection, unless the handler is closed.
func (h *Handler) start() (*serverConn, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return nil, false
	}
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	ctx, cancel := context.WithCancelCause(context.Background())
	sc := &serverConn{
		h: h, cancel: cancel, inR: inR, inW: inW, outR: outR, outW: outW,
		returned: make(chan struct{}), ending: make(chan struct{}), ended: make(chan struct{}),
	}
	h.conns[sc] = struct{}{}
	h.active.Add(1)
	// Not in active: Close would wait for an abandoned serve for ever.
	go sc.run(ctx)
	return sc, true
}

// finish removes sc, whose ending is done.
func (h *Handler) finish(sc *serverConn) {
	h.mu.Lock()
	delete(h.conns, sc)
	h.mu.Unlock()
	h.active.Done()
}

// serverConn is one connection's serve: its goroutine and its ends of the two
// pipes.
type serverConn struct {
	h      *Handler
	cancel context.CancelCauseFunc // ends serve's context
	inR    *io.PipeReader          // the agent's input, which serve reads
	inW    *io.PipeWriter          // the transport's end of it
	outR   *io.PipeReader          // the transport's end of the agent's output
	outW   *io.PipeWriter          // the agent's output, which serve writes

	returned chan struct{} // closed when serve has returned, err set
	err      error         // what serve returned

	endOnce sync.Once
	ending  chan struct{} // closed when the ending starts
	ended   chan struct{} // closed when the ending's last step is done
}

// run runs serve, then closes its output, so the transport reads its end,
// and its input, so a later message fails rather than waits.
func (sc *serverConn) run(ctx context.Context) {
	sc.err = sc.h.serve(ctx, sc.inR, sc.outW)
	_ = sc.outW.Close() //nolint:errcheck // Close returns nil.
	_ = sc.inR.Close()  //nolint:errcheck // Close returns nil.
	close(sc.returned)
}

// end starts the connection's ending with cause, once; later calls do
// nothing. ending closes as it starts, and ended when it is done.
func (sc *serverConn) end(cause error) {
	sc.endOnce.Do(func() {
		close(sc.ending)
		go sc.ladder(cause)
	})
}

// ladder is the ending, the same for every cause, as agentproc.InProcess's
// Close (0042-MADR-in-process-agent-runner.md, item 1): close serve's input;
// then cancel its context with cause; then close its pipes with cause; each
// step waiting up to Grace for serve to return. A serve still running after
// the last is abandoned: counted, logged, and left.
func (sc *serverConn) ladder(cause error) {
	defer close(sc.ended)
	defer sc.cancel(cause)
	grace := sc.h.opts.Grace
	_ = sc.inW.Close() //nolint:errcheck // Close returns nil.
	if sc.waitReturned(grace) {
		return
	}
	sc.cancel(cause)
	if sc.waitReturned(grace) {
		return
	}
	_ = sc.inR.CloseWithError(cause)
	_ = sc.outW.CloseWithError(cause)
	if sc.waitReturned(grace) {
		return
	}
	sc.h.abandoned.Add(1)
	sc.h.log.Warn("remote: serve abandoned: it had not returned a grace after its pipes were closed", "grace", grace)
}

// waitReturned waits up to d for serve to return, reports whether it did,
// and logs its error once it has.
func (sc *serverConn) waitReturned(d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-sc.returned:
		if sc.err != nil && !errors.Is(sc.err, context.Canceled) {
			sc.h.log.Info("remote: serve returned an error", "type", fmt.Sprintf("%T", sc.err))
		}
		return true
	case <-t.C:
		return false
	}
}
