package remote

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"sync"
	"time"

	acp "github.com/maccavelli/acp-go-sdk2"
)

// The Streamable HTTP headers and values (the RFD's "Identity Model").
const (
	sessionIDHeader       = "Acp-Session-Id"
	protocolVersionHeader = "Acp-Protocol-Version"
	eventStreamType       = "text/event-stream"
	jsonType              = "application/json"
)

// deleteBound bounds Close's DELETE.
const deleteBound = 5 * time.Second

// errorExcerpt is how much of a refused answer's body an error quotes.
const errorExcerpt = 512

// errNotInitializeFirst is a Write's error when the connection's first message
// is not the initialize request, which the RFD makes the one POST answered in
// its body.
var errNotInitializeFirst = errors.New("remote: the first message on a Streamable HTTP connection must be the initialize request")

// httpClient is a Conn's Streamable HTTP profile
// (0043-PLAN-remote-http-and-websocket-transport.md, phase 5): each line
// written is POSTed, one at a time, in order; the connection-scoped stream and
// each session's stream feed one lineQueue.
type httpClient struct {
	url      string
	client   *http.Client
	header   http.Header
	maxBytes int

	ctx    context.Context // the connection's life: every request runs under it
	cancel context.CancelFunc
	in     *lineQueue

	mu          sync.Mutex // serializes Write, so the POSTs go in order
	out         *lineSplitter
	initialized bool

	smu        sync.Mutex
	connID     string
	streams    map[string]bool   // sessions whose stream is open or opening
	serverReqs map[string]string // the server's requests, by id key: their session
	reopen     map[string]string // load and resume requests, by id key, whose stream was refused: their session
	closing    bool

	streamsDone sync.WaitGroup
	closeOnce   sync.Once
	closeErr    error
}

// newHTTPClient returns the Streamable HTTP profile for rawURL. It sends
// nothing until the first Write.
func newHTTPClient(rawURL string, o DialOptions) *httpClient {
	ctx, cancel := context.WithCancel(context.Background())
	c := &httpClient{
		url: rawURL, client: o.HTTPClient, header: o.Header, maxBytes: o.MaxMessageSize,
		ctx: ctx, cancel: cancel, in: newLineQueue(),
		streams: map[string]bool{}, serverReqs: map[string]string{}, reopen: map[string]string{},
	}
	c.out = newLineSplitter(c.post, o.MaxMessageSize)
	return c
}

func (c *httpClient) Read(p []byte) (int, error) { return c.in.Read(p) }

func (c *httpClient) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.out.Write(p)
}

// Close sends DELETE, under deleteBound, ends every stream, and ends the
// reader with io.EOF.
func (c *httpClient) Close() error {
	c.closeOnce.Do(func() {
		c.smu.Lock()
		c.closing = true
		id := c.connID
		c.smu.Unlock()
		c.in.end(nil)
		if id != "" {
			ctx, cancel := context.WithTimeout(context.Background(), deleteBound)
			resp, err := c.do(ctx, http.MethodDelete, nil, id, "")
			cancel()
			if err != nil {
				c.closeErr = fmt.Errorf("remote: DELETE: %w", err)
			} else {
				drain(resp)
			}
		}
		c.cancel()
		c.streamsDone.Wait()
	})
	return c.closeErr
}

// fail ends the connection with err: the reader returns it after the lines
// already queued, and the streams end.
func (c *httpClient) fail(err error) {
	c.in.end(err)
	c.cancel()
}

// post sends line, one message, as the RFD's steps say.
func (c *httpClient) post(line []byte) error {
	m := classify(line)
	if !c.initialized {
		if m.kind != kindRequest || m.method != acp.AgentMethodInitialize {
			c.fail(errNotInitializeFirst)
			return errNotInitializeFirst
		}
		return c.initialize(line, m)
	}
	c.smu.Lock()
	id := c.connID
	c.smu.Unlock()
	session := c.sessionOf(m)
	if m.kind == kindRequest && (m.method == acp.AgentMethodSessionLoad || m.method == acp.AgentMethodSessionResume) && session != "" {
		if status := c.openStream(session); status == http.StatusNotFound {
			c.smu.Lock()
			c.reopen[m.idKey] = session
			c.smu.Unlock()
		}
	}
	resp, err := c.do(c.ctx, http.MethodPost, line, id, session)
	if err == nil && resp.StatusCode == http.StatusNotFound && session != "" {
		// Python's answer to a session it has not seen: sent again without the
		// header (0043-MADR-remote-http-and-websocket-transport.md, "Amended by
		// the PLAN, 2026-10-29", 5).
		drain(resp)
		resp, err = c.do(c.ctx, http.MethodPost, line, id, "") //nolint:bodyclose // refused or drain closes it below.
	}
	if err != nil {
		err = fmt.Errorf("remote: POST: %w", err)
		c.fail(err)
		return err
	}
	if resp.StatusCode != http.StatusAccepted {
		err := refused("POST", resp)
		c.fail(err)
		return err
	}
	drain(resp)
	return nil
}

// sessionOf is the session a message's POST names in Acp-Session-Id: a
// request's or a notification's, when its method is session-scoped or its
// params name one, unless its method is a "$/" one; a response's, when the
// request it answers named one.
func (c *httpClient) sessionOf(m message) string {
	switch m.kind {
	case kindRequest, kindNotification:
		return m.session()
	case kindResponse:
		c.smu.Lock()
		defer c.smu.Unlock()
		s := c.serverReqs[m.idKey]
		delete(c.serverReqs, m.idKey)
		return s
	}
	return ""
}

// initialize POSTs the initialize request, with no connection header, and
// reads its answer from the POST's body.
func (c *httpClient) initialize(line []byte, m message) error {
	resp, err := c.do(c.ctx, http.MethodPost, line, "", "")
	if err != nil {
		err = fmt.Errorf("remote: POST initialize: %w", err)
		c.fail(err)
		return err
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, int64(c.maxBytes)+1))
	drain(resp)
	if err != nil {
		err = fmt.Errorf("remote: reading initialize's answer: %w", err)
		c.fail(err)
		return err
	}
	answer := classify(toLine(body))
	isAnswer := len(body) <= c.maxBytes && answer.kind == kindResponse && answer.idKey == m.idKey
	id := resp.Header.Get(connectionIDHeader)
	switch {
	case resp.StatusCode == http.StatusOK && id != "" && isAnswer && isType(resp, jsonType):
		c.smu.Lock()
		c.connID = id
		c.smu.Unlock()
		c.initialized = true
		c.in.push(toLine(body))
		if status := c.openStream(""); status != http.StatusOK {
			err := fmt.Errorf("remote: the connection's stream was answered %d", status)
			c.fail(err)
			return err
		}
		return nil
	case isAnswer && id == "":
		// The agent's error, sent with no connection: the caller's Initialize
		// returns it, and then the connection ends.
		c.in.push(toLine(body))
		c.fail(fmt.Errorf("remote: initialize answered %d with no connection", resp.StatusCode))
		return nil
	default:
		err := refusedText("POST initialize", resp.StatusCode, body)
		c.fail(err)
		return err
	}
}

// openStream opens session's stream, or the connection's when session is "",
// unless it is open, and returns the GET's status: 200 when it is open, 0 when
// it was open already or the request failed. A stream's events go to the
// reader on a goroutine of their own.
func (c *httpClient) openStream(session string) int {
	c.smu.Lock()
	if session != "" && c.streams[session] {
		c.smu.Unlock()
		return 0
	}
	c.streams[session] = true
	id := c.connID
	c.smu.Unlock()
	resp, err := c.do(c.ctx, http.MethodGet, nil, id, session)
	if err != nil || resp.StatusCode != http.StatusOK {
		c.smu.Lock()
		delete(c.streams, session)
		c.smu.Unlock()
		if err != nil {
			return 0
		}
		drain(resp)
		return resp.StatusCode
	}
	c.streamsDone.Add(1)
	go c.readStream(session, resp)
	return http.StatusOK
}

// readStream gives each event of a stream to the reader, after opening the
// streams its answers name, until the stream ends. The connection's stream's
// end ends the connection; a session's ends that stream only.
func (c *httpClient) readStream(session string, resp *http.Response) {
	defer c.streamsDone.Done()
	defer drain(resp)
	events := newSSEReader(resp.Body, c.maxBytes)
	for {
		data, err := events.next()
		if err != nil {
			c.smu.Lock()
			closing := c.closing
			delete(c.streams, session)
			c.smu.Unlock()
			if session == "" && !closing {
				c.fail(fmt.Errorf("remote: the connection's stream ended: %w", err))
			}
			return
		}
		line := toLine(data)
		m := classify(line)
		switch m.kind {
		case kindRequest:
			if s := m.session(); s != "" {
				c.smu.Lock()
				c.serverReqs[m.idKey] = s
				c.smu.Unlock()
			}
		case kindResponse:
			c.smu.Lock()
			retry, ok := c.reopen[m.idKey]
			delete(c.reopen, m.idKey)
			c.smu.Unlock()
			if m.resultSessionID != "" {
				c.openStream(m.resultSessionID)
			}
			if ok {
				c.openStream(retry)
			}
		}
		if !c.in.push(line) {
			return
		}
	}
}

// do sends one request of the connection: body, if any, as JSON; the
// connection's id and the protocol version once there is an id; the session,
// if any; and the caller's headers.
func (c *httpClient) do(ctx context.Context, method string, body []byte, connID, session string) (*http.Response, error) {
	var r io.Reader
	if body != nil {
		r = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.url, r)
	if err != nil {
		return nil, err
	}
	for k, vs := range c.header {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	switch method {
	case http.MethodPost:
		req.Header.Set("Content-Type", jsonType)
		req.Header.Set("Accept", jsonType)
	case http.MethodGet:
		req.Header.Set("Accept", eventStreamType)
	}
	if connID != "" {
		req.Header.Set(connectionIDHeader, connID)
		req.Header.Set(protocolVersionHeader, "1")
	}
	if session != "" {
		req.Header.Set(sessionIDHeader, session)
	}
	return c.client.Do(req)
}

// refused is the error for an answer of resp's status, quoting at most
// errorExcerpt bytes of its body, which it drains and closes.
func refused(what string, resp *http.Response) error {
	b, _ := io.ReadAll(io.LimitReader(resp.Body, errorExcerpt)) //nolint:errcheck // The excerpt is best effort.
	drain(resp)
	return refusedText(what, resp.StatusCode, b)
}

// refusedText is the error for an answer of status whose body is body,
// quoting at most errorExcerpt bytes of it.
func refusedText(what string, status int, body []byte) error {
	if len(body) > errorExcerpt {
		body = body[:errorExcerpt]
	}
	return fmt.Errorf("remote: %s answered %d: %s", what, status, bytes.TrimSpace(body))
}

// drain reads what is left of resp's body, up to a bound, and closes it, so
// its connection can be kept.
func drain(resp *http.Response) {
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64*1024)) //nolint:errcheck // Draining is best effort.
	_ = resp.Body.Close()                                          //nolint:errcheck // Nothing is left to report.
}

// isType reports whether resp's Content-Type is media, whatever its
// parameters.
func isType(resp *http.Response, media string) bool {
	mt, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	return err == nil && mt == media
}
