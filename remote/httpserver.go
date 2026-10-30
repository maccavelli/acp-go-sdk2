package remote

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
	"sync"
	"time"

	acp "github.com/maccavelli/acp-go-sdk2"
)

// maxIDLength bounds an id key and a session id a connection keeps track of,
// as TypeScript's DEFAULT_MAX_ID_LENGTH does
// (0043-MADR-remote-http-and-websocket-transport.md, "Amended by the PLAN,
// 2026-10-29", 2).
const maxIDLength = 1024

// httpConn is one Streamable HTTP connection: its serve, the router that
// reads the agent's output, and the streams that carry it to the client
// (0043-PLAN-remote-http-and-websocket-transport.md, phase 6).
type httpConn struct {
	h  *Handler
	sc *serverConn
	id string

	initID  string      // initialize's id key, until its answer arrives
	initAns chan []byte // the answer to initialize

	mu           sync.Mutex
	streams      map[string]*sseStream // "" is the connection's
	held         int                   // bytes held across streams
	routes       map[string]string     // the client's requests, by id key: their answer's stream
	attaching    map[string]string     // session/load and session/resume requests, by id key: their session
	agentReqs    map[string]string     // the agent's requests, by id key: their session
	named        map[string]bool       // sessions a result or a load names
	connAttached bool                  // the connection stream has a receiver
	idleSince    time.Time             // when the connection stream last had none
	overflowed   bool

	routed chan struct{} // closed when the router has read the agent's output to its end
	done   chan struct{} // closed when the connection has ended: the streams send what is held and close
}

// sseStream is one stream of a connection: the lines held for it, and its
// receiver, if any.
type sseStream struct {
	held        [][]byte
	notify      chan struct{} // a line or a stop for the receiver
	attached    bool
	gen         int           // the receiver's generation, so a late detach leaves a newer receiver
	stop        chan struct{} // closed to end the receiver of a closed provisional stream
	provisional bool          // opened by a GET for a session nothing has named yet
	created     time.Time
}

// newSSEStream returns an empty stream.
func newSSEStream() *sseStream {
	return &sseStream{notify: make(chan struct{}, 1), stop: make(chan struct{})}
}

// wake tells the stream's receiver to look again.
func (st *sseStream) wake() {
	select {
	case st.notify <- struct{}{}:
	default:
	}
}

// servePost serves a POST: the RFD's routing tree, with the official
// servers' answers where it is silent.
func (h *Handler) servePost(w http.ResponseWriter, r *http.Request) {
	if mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mt != jsonType {
		h.refuse(w, http.StatusUnsupportedMediaType, "remote: a POST's Content-Type must be application/json")
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, int64(h.opts.MaxMessageSize)))
	if err != nil {
		if _, tooBig := errors.AsType[*http.MaxBytesError](err); tooBig {
			h.refuse(w, http.StatusRequestEntityTooLarge, "remote: the message exceeds MaxMessageSize")
			return
		}
		h.refuse(w, http.StatusBadRequest, "remote: reading the body failed")
		return
	}
	trimmed := bytes.TrimSpace(body)
	switch {
	case !json.Valid(trimmed):
		h.refuse(w, http.StatusBadRequest, "remote: the body is not JSON")
		return
	case trimmed[0] == '[':
		h.refuse(w, http.StatusNotImplemented, "remote: batches are not supported")
		return
	}
	m := classify(trimmed)
	header := r.Header.Get(sessionIDHeader)
	switch {
	case m.kind == kindOther:
		h.refuse(w, http.StatusBadRequest, "remote: the body is not a JSON-RPC request, notification or response")
		return
	case len(m.idKey) > maxIDLength || len(m.sessionID) > maxIDLength || len(header) > maxIDLength:
		h.refuse(w, http.StatusBadRequest, "remote: an id or a session id is too long")
		return
	}
	isInit := m.kind == kindRequest && m.method == acp.AgentMethodInitialize
	connID := r.Header.Get(connectionIDHeader)
	switch {
	case connID == "" && isInit:
		h.serveInitialize(w, r, trimmed, m)
		return
	case connID == "":
		h.refuse(w, http.StatusBadRequest, "remote: Acp-Connection-Id is required after initialize")
		return
	case isInit:
		h.refuse(w, http.StatusBadRequest, "remote: initialize on an existing connection")
		return
	}
	hc := h.httpConn(connID)
	if hc == nil {
		h.refuse(w, http.StatusNotFound, "remote: unknown connection")
		return
	}
	if text := hc.admit(m, header); text != "" {
		h.refuse(w, http.StatusBadRequest, text)
		return
	}
	if _, err := hc.sc.inW.Write(toLine(trimmed)); err != nil {
		h.refuse(w, http.StatusNotFound, "remote: the connection has ended")
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

// admit checks a message's session, Rust's rule: a request or notification
// takes it from Acp-Session-Id or params.sessionId, and both, differing, are
// refused; a session-scoped method needs one; a response may omit it, and
// names its request's when it gives one. It records a request's answer's
// route. It returns a refusal's text, or "".
func (hc *httpConn) admit(m message, header string) string {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	if m.kind == kindResponse {
		if header != "" && header != hc.agentReqs[m.idKey] {
			return "remote: Acp-Session-Id is not the session of the request this answers"
		}
		delete(hc.agentReqs, m.idKey)
		return ""
	}
	session := m.session()
	if connectionScoped(m.method) {
		header = ""
	}
	switch {
	case header != "" && session != "" && header != session:
		return "remote: Acp-Session-Id and params.sessionId differ"
	case session == "":
		session = header
	}
	if sessionScoped(m.method) && session == "" {
		return "remote: a session-scoped method needs Acp-Session-Id or params.sessionId"
	}
	if m.kind != kindRequest {
		return ""
	}
	switch m.method {
	case acp.AgentMethodSessionNew, "session/fork":
		hc.routes[m.idKey] = ""
	case acp.AgentMethodSessionLoad, acp.AgentMethodSessionResume:
		hc.routes[m.idKey] = ""
		hc.attaching[m.idKey] = session
		hc.named[session] = true
	default:
		hc.routes[m.idKey] = session
	}
	return ""
}

// httpConn returns the connection named id, or nil.
func (h *Handler) httpConn(id string) *httpConn {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.httpConns[id]
}

// serveInitialize opens a connection: it starts its serve, forwards
// initialize, and answers with the agent's answer, or ends the connection
// (0043-MADR-remote-http-and-websocket-transport.md, "Amended by the PLAN,
// 2026-10-29", 9).
func (h *Handler) serveInitialize(w http.ResponseWriter, r *http.Request, body []byte, m message) {
	sc, ok := h.start()
	if !ok {
		h.refuse(w, http.StatusServiceUnavailable, "remote: the handler is closed")
		return
	}
	hc := &httpConn{
		h: h, sc: sc, id: rand.Text(), initID: m.idKey, initAns: make(chan []byte, 1),
		streams: map[string]*sseStream{}, routes: map[string]string{}, attaching: map[string]string{},
		agentReqs: map[string]string{}, named: map[string]bool{}, idleSince: time.Now(),
		routed: make(chan struct{}), done: make(chan struct{}),
	}
	go hc.run()
	if _, err := sc.inW.Write(toLine(body)); err != nil {
		hc.fail(w, m)
		return
	}
	t := time.NewTimer(h.initBound)
	defer t.Stop()
	select {
	case ans := <-hc.initAns:
		var env struct {
			Error json.RawMessage `json:"error"`
		}
		_ = json.Unmarshal(ans, &env) //nolint:errcheck // The router read it as a response already.
		w.Header().Set("Content-Type", jsonType)
		if env.Error != nil {
			sc.end(ErrDisconnected)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(ans) //nolint:errcheck // The client may have gone; the connection is ended either way.
			return
		}
		h.mu.Lock()
		closed := h.closed
		if !closed {
			h.httpConns[hc.id] = hc
		}
		h.mu.Unlock()
		if closed {
			// Close ended the connection meanwhile.
			h.refuse(w, http.StatusServiceUnavailable, "remote: the handler is closed")
			return
		}
		w.Header().Set(connectionIDHeader, hc.id)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(ans) //nolint:errcheck // The client may have gone; its stream's absence ends the connection.
	case <-sc.returned:
		hc.fail(w, m)
	case <-t.C:
		sc.end(ErrDisconnected)
		h.refuse(w, http.StatusGatewayTimeout, "remote: the agent did not answer initialize in time")
	case <-r.Context().Done():
		sc.end(ErrDisconnected)
	}
}

// fail answers initialize 500, with a JSON-RPC -32603 for its id, as
// TypeScript's server does, and ends the connection.
func (hc *httpConn) fail(w http.ResponseWriter, m message) {
	hc.sc.end(ErrDisconnected)
	hc.h.log.Info("remote: request refused", "status", http.StatusInternalServerError)
	w.Header().Set("Content-Type", jsonType)
	w.WriteHeader(http.StatusInternalServerError)
	_, _ = fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%s,"error":{"code":-32603,"message":"Internal error","data":{"error":"the agent ended before it answered initialize"}}}`, m.idKey) //nolint:errcheck // The client may have gone.
}

// run owns the connection: the router reads the agent's output; the idle
// bound and the provisional streams are checked on a ticker; and when the
// ending is done and the router has read to the end, the streams send what is
// held and close.
func (hc *httpConn) run() {
	h, sc := hc.h, hc.sc
	go hc.route()
	tick := min(h.opts.Grace, h.idleBound) / 4
	tick = max(tick, 10*time.Millisecond)
	t := time.NewTicker(tick)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			hc.check()
			continue
		case <-sc.ended:
		}
		break
	}
	<-hc.routed
	h.mu.Lock()
	if h.httpConns[hc.id] == hc {
		delete(h.httpConns, hc.id)
	}
	h.mu.Unlock()
	close(hc.done)
	h.finish(sc)
}

// check ends the connection when its connection stream has had no receiver
// for the idle bound, and closes the provisional streams nothing has named
// within Grace.
func (hc *httpConn) check() {
	now := time.Now()
	hc.mu.Lock()
	idle := !hc.connAttached && now.Sub(hc.idleSince) >= hc.h.idleBound
	for key, st := range hc.streams {
		if st.provisional && !hc.named[key] && now.Sub(st.created) >= hc.h.opts.Grace {
			hc.closeStreamLocked(key, st)
		}
	}
	hc.mu.Unlock()
	if idle {
		hc.h.log.Info("remote: a Streamable HTTP connection had no stream for the idle bound", "bound", hc.h.idleBound)
		hc.sc.end(ErrDisconnected)
	}
}

// closeStreamLocked ends key's stream and drops what it held; hc.mu is held.
func (hc *httpConn) closeStreamLocked(key string, st *sseStream) {
	for _, l := range st.held {
		hc.held -= len(l)
	}
	close(st.stop)
	delete(hc.streams, key)
}

// route reads the agent's output, one line each, and holds each line for its
// stream (0043-PLAN-remote-http-and-websocket-transport.md, phase 6, "Outbound
// router"). At the output's end it ends the connection.
func (hc *httpConn) route() {
	defer close(hc.routed)
	s := newLineSplitter(func(line []byte) error {
		hc.dispatch(line)
		return nil
	}, hc.h.opts.MaxMessageSize)
	hc.sc.pump(s)
	hc.sc.end(ErrDisconnected)
}

// dispatch holds line for its stream: initialize's answer for its POST; a
// request or notification for its session's stream; an answer for its
// request's route; anything else for the connection's.
func (hc *httpConn) dispatch(line []byte) {
	m := classify(line)
	if m.kind == kindResponse && hc.initID != "" && m.idKey == hc.initID {
		hc.initID = ""
		hc.initAns <- line
		return
	}
	hc.mu.Lock()
	defer hc.mu.Unlock()
	if hc.overflowed {
		return
	}
	key := ""
	switch m.kind {
	case kindResponse:
		key = hc.routes[m.idKey]
		delete(hc.routes, m.idKey)
		if s, ok := hc.attaching[m.idKey]; ok {
			delete(hc.attaching, m.idKey)
			hc.settleAttach(s, isErrorAnswer(line))
		}
		if m.resultSessionID != "" {
			hc.named[m.resultSessionID] = true
			if st := hc.streams[m.resultSessionID]; st != nil {
				st.provisional = false
			}
		}
	case kindRequest, kindNotification:
		key = m.session()
		if m.kind == kindRequest {
			hc.agentReqs[m.idKey] = m.session()
		}
	}
	st := hc.streams[key]
	if st == nil {
		st = newSSEStream()
		hc.streams[key] = st
	}
	st.held = append(st.held, line)
	hc.held += len(line)
	if hc.held > hc.h.opts.MaxBuffered {
		hc.overflowed = true
		hc.h.log.Warn("remote: messages held for streams no client receives exceed MaxBuffered", "limit", hc.h.opts.MaxBuffered)
		hc.sc.end(ErrOverflow)
		return
	}
	st.wake()
}

// isErrorAnswer reports whether line, a response, carries an error.
func isErrorAnswer(line []byte) bool {
	var env struct {
		Error json.RawMessage `json:"error"`
	}
	return json.Unmarshal(line, &env) == nil && env.Error != nil && string(env.Error) != "null"
}

// settleAttach ends a session/load's or session/resume's hold on its
// session's stream: confirmed by a result, or, after an error, the stream
// closed if a GET had opened it for that load alone, as Python closes it;
// hc.mu is held.
func (hc *httpConn) settleAttach(session string, failed bool) {
	st := hc.streams[session]
	if !failed {
		if st != nil {
			st.provisional = false
		}
		return
	}
	delete(hc.named, session)
	if st != nil && st.provisional {
		hc.closeStreamLocked(session, st)
	}
}

// serveStream serves a GET: one stream's events, until the stream, the
// connection or the request ends.
func (h *Handler) serveStream(w http.ResponseWriter, r *http.Request) {
	if !strings.Contains(r.Header.Get("Accept"), eventStreamType) {
		h.refuse(w, http.StatusNotAcceptable, "remote: a GET must accept text/event-stream")
		return
	}
	connID := r.Header.Get(connectionIDHeader)
	if connID == "" {
		h.refuse(w, http.StatusBadRequest, "remote: Acp-Connection-Id is required")
		return
	}
	key := r.Header.Get(sessionIDHeader)
	if len(key) > maxIDLength {
		h.refuse(w, http.StatusBadRequest, "remote: the session id is too long")
		return
	}
	hc := h.httpConn(connID)
	if hc == nil {
		h.refuse(w, http.StatusNotFound, "remote: unknown connection")
		return
	}
	st, gen, ok := hc.attach(key)
	if !ok {
		h.refuse(w, http.StatusConflict, "remote: the stream already has a receiver")
		return
	}
	defer hc.detach(key, st, gen)
	w.Header().Set("Content-Type", eventStreamType)
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	rc := http.NewResponseController(w)
	// A long-lived stream outlives the server's WriteTimeout (phase 2, P5);
	// a writer that cannot clear it, as httptest's recorder, keeps it.
	_ = rc.SetWriteDeadline(time.Time{}) //nolint:errcheck // http.ErrNotSupported leaves the stream as the server bounds it.
	if rc.Flush() != nil {
		return
	}
	hc.stream(w, rc, r, st)
}

// attach puts a receiver on key's stream, made if new, and returns the stream
// and the receiver's generation; it reports false when the stream has one.
func (hc *httpConn) attach(key string) (*sseStream, int, bool) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	st := hc.streams[key]
	if st == nil {
		st = newSSEStream()
		if key != "" && !hc.named[key] {
			st.provisional, st.created = true, time.Now()
		}
		hc.streams[key] = st
	}
	if st.attached {
		return nil, 0, false
	}
	st.attached = true
	st.gen++
	if key == "" {
		hc.connAttached = true
	}
	// What was held before the receiver came is sent at once.
	st.wake()
	return st, st.gen, true
}

// detach takes the receiver of generation gen off key's stream.
func (hc *httpConn) detach(key string, st *sseStream, gen int) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	if st.gen != gen {
		return
	}
	st.attached = false
	if key == "" {
		hc.connAttached = false
		hc.idleSince = time.Now()
	}
}

// stream sends st's lines as events, and a comment every KeepAlive, until the
// request ends, the stream is closed, or the connection ends, after which it
// sends what is held. A line taken for a receiver that has gone is lost:
// v1 replays nothing.
func (hc *httpConn) stream(w io.Writer, rc *http.ResponseController, r *http.Request, st *sseStream) {
	t := time.NewTicker(hc.h.opts.KeepAlive)
	defer t.Stop()
	for {
		ended := false
		select {
		case <-r.Context().Done():
			return
		case <-st.stop:
			return
		case <-hc.done:
			ended = true
		case <-st.notify:
		case <-t.C:
			if _, err := io.WriteString(w, keepAliveComment); err != nil || rc.Flush() != nil {
				return
			}
			continue
		}
		hc.mu.Lock()
		lines := st.held
		st.held = nil
		for _, l := range lines {
			hc.held -= len(l)
		}
		hc.mu.Unlock()
		for _, l := range lines {
			if writeEvent(w, l) != nil {
				return
			}
		}
		if rc.Flush() != nil || ended {
			return
		}
	}
}

// serveDelete serves a DELETE: the connection ends with ErrDeleted.
func (h *Handler) serveDelete(w http.ResponseWriter, r *http.Request) {
	connID := r.Header.Get(connectionIDHeader)
	if connID == "" {
		h.refuse(w, http.StatusBadRequest, "remote: Acp-Connection-Id is required")
		return
	}
	h.mu.Lock()
	hc := h.httpConns[connID]
	delete(h.httpConns, connID)
	h.mu.Unlock()
	if hc == nil {
		h.refuse(w, http.StatusNotFound, "remote: unknown connection")
		return
	}
	hc.sc.end(ErrDeleted)
	w.WriteHeader(http.StatusAccepted)
}
