package remote

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	acp "github.com/maccavelli/acp-go-sdk2"
	"github.com/maccavelli/acp-go-sdk2/acptest"
)

// fakeMode is the official server a fakeServer plays, one column of the
// MADR's K3 second table (0043-PLAN-remote-http-and-websocket-transport.md,
// phase 5).
type fakeMode int

const (
	// tsMode: Acp-Session-Id required whenever params.sessionId is present;
	// session/load's answer on the connection stream, its replay on the
	// session's; a response to the agent's session request must name it.
	tsMode fakeMode = iota
	// rustMode: the header or params.sessionId; session/load's answer and
	// replay on the session stream.
	rustMode
	// pyMode: 404 for a header naming a session the connection has not seen,
	// on POST and on GET; session/load's answer and replay on the connection
	// stream.
	pyMode
)

func (m fakeMode) String() string { return [...]string{"ts", "rust", "py"}[m] }

// pySessionScoped is the Python server's list: the header is required for
// these five alone.
var pySessionScoped = map[string]bool{
	acp.AgentMethodSessionPrompt: true, acp.AgentMethodSessionCancel: true, acp.AgentMethodSessionClose: true,
	acp.AgentMethodSessionSetMode: true, acp.AgentMethodSessionSetConfigOption: true,
}

// fakeRequest is one request the fake server received.
type fakeRequest struct {
	method, proto                 string
	connID, session, protoVersion string
	cookie, custom                string
	body                          string
	status                        int
}

// fakeServer plays the RFD's server side, in one fakeMode, for a new agent
// on each connection, and records every request.
type fakeServer struct {
	t        *testing.T
	mode     fakeMode
	newAgent func() acp.Agent
	ts       *httptest.Server

	mu        sync.Mutex
	conns     map[string]*fakeConn
	requests  []fakeRequest
	next      int
	initFail  int           // 200, 500 or 504: initialize refused so
	failNext  int           // a status for the next POST that is not initialize
	setCookie bool          // a cookie on initialize's answer
	getDelay  time.Duration // how long a session stream's GET waits before its 200
}

// fakeConn is one connection of the fake server.
type fakeConn struct {
	id      string
	inW     *io.PipeWriter
	served  chan struct{} // closed when the agent's serve and the router are done
	initID  string
	initAns chan []byte

	mu          sync.Mutex
	cond        *sync.Cond
	streams     map[string]*fakeStream // "" is the connection's
	known       map[string]bool
	routes      map[string]string // the client's requests, by id key: the stream of their answer
	loads       map[string]bool   // pyMode: sessions whose load is pending, replayed on the connection stream
	loadIDs     map[string]string // pyMode: the pending loads, by request id key: their session
	agentReqs   map[string]string // the agent's requests, by id key: their session
	agentClosed bool
}

// fakeStream is what is held for one stream, and whether a receiver is on it.
type fakeStream struct {
	held     [][]byte
	attached bool
	closed   bool
}

// newFakeServer serves a fake in mode until the test ends.
func newFakeServer(t *testing.T, mode fakeMode, newAgent func() acp.Agent) *fakeServer {
	t.Helper()
	f := &fakeServer{t: t, mode: mode, newAgent: newAgent, conns: map[string]*fakeConn{}}
	f.ts = httptest.NewUnstartedServer(f)
	return f
}

// start starts the fake's server, plain or with TLS and HTTP/2, and stops it,
// its connections first, when the test ends.
func (f *fakeServer) start(tls bool) *fakeServer {
	if tls {
		f.ts.EnableHTTP2 = true
		f.ts.StartTLS()
	} else {
		f.ts.Start()
	}
	f.t.Cleanup(func() {
		f.mu.Lock()
		conns := make([]*fakeConn, 0, len(f.conns))
		for _, c := range f.conns {
			conns = append(conns, c)
		}
		f.mu.Unlock()
		for _, c := range conns {
			c.end()
		}
		f.ts.Close()
	})
	return f
}

// all returns the requests received so far.
func (f *fakeServer) all() []fakeRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]fakeRequest(nil), f.requests...)
}

// conn returns the fake's one connection.
func (f *fakeServer) conn() *fakeConn {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, c := range f.conns {
		return c
	}
	return nil
}

func (f *fakeServer) record(r *http.Request, body string, status int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	ck := ""
	if c, err := r.Cookie("affinity"); err == nil {
		ck = c.Value
	}
	f.requests = append(f.requests, fakeRequest{
		method: r.Method, proto: r.Proto, connID: r.Header.Get(connectionIDHeader), session: r.Header.Get(sessionIDHeader),
		protoVersion: r.Header.Get(protocolVersionHeader), cookie: ck, custom: r.Header.Get("X-Test"), body: body, status: status,
	})
}

func (f *fakeServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		f.post(w, r)
	case http.MethodGet:
		f.get(w, r)
	case http.MethodDelete:
		c := f.lookup(r.Header.Get(connectionIDHeader))
		status := http.StatusAccepted
		if c == nil {
			status = http.StatusNotFound
		} else {
			f.mu.Lock()
			delete(f.conns, c.id)
			f.mu.Unlock()
			c.end()
		}
		f.record(r, "", status)
		w.WriteHeader(status)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (f *fakeServer) lookup(id string) *fakeConn {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.conns[id]
}

// forget drops a connection from the registry, as a server that lost it,
// leaving its agent running until the test ends.
func (f *fakeServer) forget(c *fakeConn) {
	f.mu.Lock()
	delete(f.conns, c.id)
	f.mu.Unlock()
	f.t.Cleanup(c.end)
}

func (f *fakeServer) post(w http.ResponseWriter, r *http.Request) {
	b, _ := io.ReadAll(r.Body)
	m := classify(b)
	id := r.Header.Get(connectionIDHeader)
	answer := func(status int) {
		f.record(r, string(b), status)
		w.WriteHeader(status)
	}
	if id == "" {
		if m.kind != kindRequest || m.method != acp.AgentMethodInitialize {
			answer(http.StatusBadRequest)
			return
		}
		f.initialize(w, r, b, m)
		return
	}
	c := f.lookup(id)
	if c == nil {
		answer(http.StatusNotFound)
		return
	}
	f.mu.Lock()
	fail := f.failNext
	f.failNext = 0
	if m.method == "_test/seq" && f.mode == tsMode {
		// Jitter, so that POSTs sent at once would arrive out of order.
		time.Sleep(time.Duration(len(f.requests)%3) * time.Millisecond)
	}
	f.mu.Unlock()
	if fail != 0 {
		answer(fail)
		return
	}
	header := r.Header.Get(sessionIDHeader)
	if status := c.admit(f.mode, m, header); status != http.StatusAccepted {
		answer(status)
		return
	}
	if _, err := c.inW.Write(toLine(b)); err != nil {
		answer(http.StatusGone)
		return
	}
	answer(http.StatusAccepted)
}

// admit checks a POST's session as the mode's server does, and records the
// route of a request's answer.
func (c *fakeConn) admit(mode fakeMode, m message, header string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	switch m.kind {
	case kindResponse:
		if want := c.agentReqs[m.idKey]; mode == tsMode && want != "" && header != want {
			return http.StatusBadRequest
		}
		return http.StatusAccepted
	case kindRequest, kindNotification:
	default:
		return http.StatusBadRequest
	}
	session := m.session()
	switch mode {
	case tsMode:
		if (sessionScoped(m.method) || session != "") && !connectionScoped(m.method) && header == "" {
			return http.StatusBadRequest
		}
		if header != "" && session != "" && header != session {
			return http.StatusBadRequest
		}
	case rustMode:
		if sessionScoped(m.method) && header == "" && session == "" {
			return http.StatusBadRequest
		}
		if header != "" && session != "" && header != session {
			return http.StatusBadRequest
		}
	case pyMode:
		if pySessionScoped[m.method] && header == "" {
			return http.StatusBadRequest
		}
		if header != "" && !c.known[header] {
			return http.StatusNotFound
		}
	}
	if session == "" {
		session = header
	}
	attach := m.method == acp.AgentMethodSessionLoad || m.method == acp.AgentMethodSessionResume
	if attach {
		c.known[session] = true
		if mode == pyMode && m.kind == kindRequest {
			c.loads[session] = true
			c.loadIDs[m.idKey] = session
		}
	}
	if m.kind == kindRequest {
		route := session
		if m.method == acp.AgentMethodSessionNew || m.method == "session/fork" || (attach && mode != rustMode) {
			route = ""
		}
		c.routes[m.idKey] = route
	}
	return http.StatusAccepted
}

// initialize starts a connection and answers initialize from its agent, or
// refuses it as initFail says.
func (f *fakeServer) initialize(w http.ResponseWriter, r *http.Request, b []byte, m message) {
	f.mu.Lock()
	initFail := f.initFail
	f.next++
	id := fmt.Sprintf("conn-%d", f.next)
	f.mu.Unlock()
	switch initFail {
	case http.StatusOK, http.StatusInternalServerError:
		f.record(r, string(b), initFail)
		w.Header().Set("Content-Type", jsonType)
		w.WriteHeader(initFail)
		_, _ = fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%s,"error":{"code":-32603,"message":"Internal error","data":{"error":"refused by the fake"}}}`, m.idKey)
		return
	case http.StatusGatewayTimeout:
		f.record(r, string(b), initFail)
		http.Error(w, "initialize timed out", initFail)
		return
	}
	c := f.startConn(id, m.idKey)
	if _, err := c.inW.Write(toLine(b)); err != nil {
		f.record(r, string(b), http.StatusInternalServerError)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	select {
	case ans := <-c.initAns:
		f.record(r, string(b), http.StatusOK)
		w.Header().Set(connectionIDHeader, id)
		w.Header().Set("Content-Type", jsonType)
		f.mu.Lock()
		setCookie := f.setCookie
		f.mu.Unlock()
		if setCookie {
			http.SetCookie(w, &http.Cookie{Name: "affinity", Value: "node-7", Path: "/"})
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(ans)
	case <-time.After(waitBound):
		f.record(r, string(b), http.StatusGatewayTimeout)
		w.WriteHeader(http.StatusGatewayTimeout)
	}
}

// startConn starts an agent for a new connection, with its router.
func (f *fakeServer) startConn(id, initID string) *fakeConn {
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	c := &fakeConn{
		id: id, inW: inW, served: make(chan struct{}), initID: initID, initAns: make(chan []byte, 1),
		streams: map[string]*fakeStream{}, known: map[string]bool{}, routes: map[string]string{},
		loads: map[string]bool{}, loadIDs: map[string]string{}, agentReqs: map[string]string{},
	}
	c.cond = sync.NewCond(&c.mu)
	f.mu.Lock()
	f.conns[id] = c
	f.mu.Unlock()
	agent := f.newAgent()
	served := make(chan struct{})
	go func() {
		defer close(served)
		_ = acp.ServeAgent(context.Background(), agent, inR, outW)
		_ = outW.Close()
	}()
	go func() {
		defer close(c.served)
		f.route(c, outR)
		<-served
		c.mu.Lock()
		c.agentClosed = true
		c.cond.Broadcast()
		c.mu.Unlock()
	}()
	return c
}

// end ends the connection's agent and waits for it.
func (c *fakeConn) end() {
	_ = c.inW.Close()
	<-c.served
}

// route reads the agent's output and holds each line for its stream, as the
// mode's server routes it.
func (f *fakeServer) route(c *fakeConn, out io.Reader) {
	sc := bufio.NewScanner(out)
	sc.Buffer(make([]byte, 0, 64*1024), defaultMaxMessageSize+1)
	for sc.Scan() {
		line := append([]byte(nil), sc.Bytes()...)
		m := classify(line)
		c.mu.Lock()
		if c.initID != "" && m.kind == kindResponse && m.idKey == c.initID {
			c.initID = ""
			c.mu.Unlock()
			c.initAns <- line
			continue
		}
		key := ""
		switch m.kind {
		case kindResponse:
			key = c.routes[m.idKey]
			delete(c.routes, m.idKey)
			if m.resultSessionID != "" {
				c.known[m.resultSessionID] = true
			}
			if s, ok := c.loadIDs[m.idKey]; ok {
				delete(c.loadIDs, m.idKey)
				delete(c.loads, s)
			}
		case kindRequest, kindNotification:
			key = m.session()
			if f.mode == pyMode && c.loads[key] {
				key = ""
			}
			if m.kind == kindRequest {
				c.agentReqs[m.idKey] = m.session()
			}
		}
		st := c.stream(key)
		st.held = append(st.held, line)
		c.cond.Broadcast()
		c.mu.Unlock()
	}
}

// stream returns key's stream, made if new; c.mu is held.
func (c *fakeConn) stream(key string) *fakeStream {
	st := c.streams[key]
	if st == nil {
		st = &fakeStream{}
		c.streams[key] = st
	}
	return st
}

// attached reports whether key's stream has a receiver.
func (c *fakeConn) attached(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	st := c.streams[key]
	return st != nil && st.attached
}

// closeStream ends key's stream's response, as a server that drops it.
func (c *fakeConn) closeStream(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if st := c.streams[key]; st != nil {
		st.closed = true
		c.cond.Broadcast()
	}
}

func (f *fakeServer) get(w http.ResponseWriter, r *http.Request) {
	if !strings.Contains(r.Header.Get("Accept"), eventStreamType) {
		f.record(r, "", http.StatusNotAcceptable)
		w.WriteHeader(http.StatusNotAcceptable)
		return
	}
	c := f.lookup(r.Header.Get(connectionIDHeader))
	if c == nil {
		f.record(r, "", http.StatusNotFound)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	key := r.Header.Get(sessionIDHeader)
	c.mu.Lock()
	if f.mode == pyMode && key != "" && !c.known[key] {
		c.mu.Unlock()
		f.record(r, "", http.StatusNotFound)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	st := c.stream(key)
	if st.attached {
		c.mu.Unlock()
		f.record(r, "", http.StatusConflict)
		w.WriteHeader(http.StatusConflict)
		return
	}
	st.attached = true
	st.closed = false
	c.mu.Unlock()
	f.record(r, "", http.StatusOK)
	f.mu.Lock()
	delay := f.getDelay
	f.mu.Unlock()
	if key != "" && delay > 0 {
		time.Sleep(delay)
	}
	w.Header().Set("Content-Type", eventStreamType)
	w.WriteHeader(http.StatusOK)
	rc := http.NewResponseController(w)
	_ = rc.Flush()
	stop := context.AfterFunc(r.Context(), func() {
		c.mu.Lock()
		c.cond.Broadcast()
		c.mu.Unlock()
	})
	defer stop()
	defer func() {
		c.mu.Lock()
		st.attached = false
		c.mu.Unlock()
	}()
	for {
		c.mu.Lock()
		for len(st.held) == 0 && !st.closed && !c.agentClosed && r.Context().Err() == nil {
			c.cond.Wait()
		}
		held := st.held
		st.held = nil
		done := st.closed || r.Context().Err() != nil || (c.agentClosed && len(held) == 0)
		c.mu.Unlock()
		for _, line := range held {
			if _, err := fmt.Fprintf(w, "data: %s\n\n", line); err != nil {
				return
			}
		}
		_ = rc.Flush()
		if done {
			return
		}
	}
}

// loadAgent is a ScriptAgent that advertises loadSession and replays one
// update when a session is loaded.
type loadAgent struct {
	acptest.ScriptAgent
	conn *acp.AgentSideConnection
}

func newLoadAgent() acp.Agent {
	a := &loadAgent{}
	a.Capabilities = acp.AgentCapabilities{LoadSession: true}
	a.Turns = []acptest.Turn{{Updates: []acp.SessionUpdate{acp.UpdateAgentMessageText("after load")}, StopReason: acp.StopReasonEndTurn}}
	return a
}

func (a *loadAgent) SetAgentConnection(c *acp.AgentSideConnection) {
	a.conn = c
	a.ScriptAgent.SetAgentConnection(c)
}

func (a *loadAgent) LoadSession(ctx context.Context, p acp.LoadSessionRequest) (acp.LoadSessionResponse, error) {
	err := a.conn.SessionUpdate(ctx, acp.SessionNotification{SessionId: p.SessionId, Update: acp.UpdateAgentMessageText("replayed")})
	return acp.LoadSessionResponse{}, err
}

// jsonLine is v as one line, for raw writes to a Conn.
func jsonLine(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return append(b, '\n')
}
