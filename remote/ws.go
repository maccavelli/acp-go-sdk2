package remote

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// wsClient is a Conn's WebSocket profile: a reader goroutine pushes each text
// frame, made a line, to in; each line written goes out as one text frame.
type wsClient struct {
	c      *websocket.Conn
	ctx    context.Context // the connection's life: every read and write runs under it
	cancel context.CancelFunc
	in     *lineQueue

	mu  sync.Mutex // serializes Write
	out *lineSplitter

	readDone  chan struct{}
	closeOnce sync.Once
	closeErr  error
}

// dialWebSocket runs the handshake under ctx, and starts the reader.
func dialWebSocket(ctx context.Context, rawURL string, o DialOptions) (*wsClient, error) {
	client := *o.HTTPClient
	client.Transport = http1Only(client.Transport)
	c, _, err := websocket.Dial(ctx, rawURL, &websocket.DialOptions{HTTPClient: &client, HTTPHeader: o.Header}) //nolint:bodyclose // Dial owns the handshake's body: "You never need to close resp.Body yourself."
	if err != nil {
		return nil, fmt.Errorf("remote: WebSocket handshake: %w", err)
	}
	c.SetReadLimit(int64(o.MaxMessageSize))
	lctx, cancel := context.WithCancel(context.Background())
	w := &wsClient{c: c, ctx: lctx, cancel: cancel, in: newLineQueue(), readDone: make(chan struct{})}
	w.out = newLineSplitter(func(line []byte) error {
		return w.c.Write(w.ctx, websocket.MessageText, line)
	}, o.MaxMessageSize)
	go w.readLoop()
	return w, nil
}

// http1Only returns rt limited to HTTP/1.1 when it is an *http.Transport, a
// copy, and rt itself otherwise. A transport that would speak HTTP/2 alone
// fails the upgrade: "http2: invalid Upgrade request header"
// (0043-PLAN-remote-http-and-websocket-transport.md, phase 2, P1b).
func http1Only(rt http.RoundTripper) http.RoundTripper {
	if rt == nil {
		rt = http.DefaultTransport
	}
	t, ok := rt.(*http.Transport)
	if !ok {
		return rt
	}
	t = t.Clone()
	var p http.Protocols
	p.SetHTTP1(true)
	t.Protocols = &p
	return t
}

// readLoop pushes each text frame to in until the socket ends, and ends in
// with the socket's end: io.EOF after a normal closure. A binary frame is
// ignored, as the RFD says.
func (w *wsClient) readLoop() {
	defer close(w.readDone)
	for {
		typ, b, err := w.c.Read(w.ctx)
		if err != nil {
			w.in.end(socketEnd(err))
			return
		}
		if typ == websocket.MessageText && !w.in.push(toLine(b)) {
			return
		}
	}
}

// socketEnd is the error a reader ends with when a socket's read failed with
// err: nil, for io.EOF, after a normal closure, and err otherwise.
func socketEnd(err error) error {
	if websocket.CloseStatus(err) == websocket.StatusNormalClosure {
		return nil
	}
	return fmt.Errorf("remote: WebSocket: %w", err)
}

func (w *wsClient) Read(p []byte) (int, error) { return w.in.Read(p) }

func (w *wsClient) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.out.Write(p)
}

// Close runs the close handshake, once, and waits for the reader.
func (w *wsClient) Close() error {
	w.closeOnce.Do(func() {
		w.in.end(nil)
		err := w.c.Close(websocket.StatusNormalClosure, "")
		w.cancel()
		<-w.readDone
		if err != nil && websocket.CloseStatus(err) == -1 && !errors.Is(err, context.Canceled) {
			w.closeErr = fmt.Errorf("remote: WebSocket close: %w", err)
		}
	})
	return w.closeErr
}

// runWebSocket serves sc over c until the socket ends and sc's ending is
// done: the reader runs here, the pump and the keepalive beside it.
func (sc *serverConn) runWebSocket(c *websocket.Conn) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pumped := make(chan struct{})
	go func() {
		defer close(pumped)
		sc.pumpWebSocket(ctx, c)
	}()
	var keepAlive sync.WaitGroup
	keepAlive.Go(func() { sc.keepAlive(ctx, c) })
	sc.readWebSocket(ctx, c)
	sc.end(ErrDisconnected)
	<-sc.ended
	<-pumped
	cancel()
	_ = c.CloseNow() //nolint:errcheck // The socket may be closed already; nothing is left to report.
	keepAlive.Wait()
}

// readWebSocket writes each text frame, made a line, to the agent's input,
// until the socket ends. A binary frame is ignored and logged, once. Once the
// agent's input is closed, frames are read and dropped, so the close
// handshake is still read.
func (sc *serverConn) readWebSocket(ctx context.Context, c *websocket.Conn) {
	warned := false
	for {
		typ, b, err := c.Read(ctx)
		if err != nil {
			return
		}
		if typ != websocket.MessageText {
			if !warned {
				warned = true
				sc.h.log.Warn("remote: binary WebSocket frame ignored", "bytes", len(b))
			}
			continue
		}
		_, _ = sc.inW.Write(toLine(b)) //nolint:errcheck // A closed input drops the frame; the ending says why.
	}
}

// pumpWebSocket sends each line the agent writes as one text frame, each
// under a bound of Grace. At the end of the agent's output it closes the
// socket normally, after its last line; after any other end, at once. A line
// longer than MaxMessageSize is dropped and logged.
func (sc *serverConn) pumpWebSocket(ctx context.Context, c *websocket.Conn) {
	grace := sc.h.opts.Grace
	s := newLineSplitter(func(line []byte) error {
		wctx, cancel := context.WithTimeout(ctx, grace)
		defer cancel()
		return c.Write(wctx, websocket.MessageText, line)
	}, sc.h.opts.MaxMessageSize)
	clean := sc.pump(s)
	if clean {
		_ = c.Close(websocket.StatusNormalClosure, "") //nolint:errcheck // The client may have gone; the reader ends either way.
		return
	}
	_ = c.CloseNow() //nolint:errcheck // The socket may be closed already.
}

// keepAlive pings the client every KeepAlive, and ends sc with
// ErrDisconnected when a ping is not answered within KeepAlive.
func (sc *serverConn) keepAlive(ctx context.Context, c *websocket.Conn) {
	every := sc.h.opts.KeepAlive
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		pctx, cancel := context.WithTimeout(ctx, every)
		err := c.Ping(pctx)
		cancel()
		if err != nil {
			if ctx.Err() == nil {
				sc.h.log.Info("remote: WebSocket ping not answered")
				sc.end(ErrDisconnected)
				_ = c.CloseNow() //nolint:errcheck // The socket is being dropped.
			}
			return
		}
	}
}

// pump copies the agent's output to s, and reports whether it ended at the
// output's clean end. When s fails, it closes the output, so the agent's
// writes fail rather than wait.
func (sc *serverConn) pump(s *lineSplitter) bool {
	buf := make([]byte, 32*1024)
	for {
		n, err := sc.outR.Read(buf)
		if n > 0 {
			if _, werr := s.Write(buf[:n]); werr != nil {
				if !errors.Is(werr, errLineTooLong) {
					_ = sc.outR.CloseWithError(werr)
					return false
				}
				sc.h.log.Warn("remote: a message from the agent exceeds MaxMessageSize and was dropped", "limit", sc.h.opts.MaxMessageSize)
			}
		}
		if err != nil {
			return errors.Is(err, io.EOF)
		}
	}
}
