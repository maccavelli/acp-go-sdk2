package acp

import (
	"context"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// handOffInput holds one waiting line, then nothing until the test ends. Its
// first Read reports whether the hand-off had finished.
type handOffInput struct {
	line   io.Reader
	idle   io.Reader
	handed *atomic.Bool
	first  chan bool
	once   sync.Once
}

func newHandOffInput(t *testing.T, line string, handed *atomic.Bool) *handOffInput {
	t.Helper()
	return &handOffInput{line: strings.NewReader(line + "\n"), idle: idleReader(t), handed: handed, first: make(chan bool, 1)}
}

func (r *handOffInput) Read(p []byte) (int, error) {
	r.once.Do(func() { r.first <- r.handed.Load() })
	n, err := r.line.Read(p)
	if err == io.EOF {
		return r.idle.Read(p)
	}
	return n, err
}

// handOffPause is how long a setter takes: a read begun meanwhile finds the
// hand-off unfinished.
const handOffPause = 10 * time.Millisecond

type awareAgent struct {
	agentFuncs
	handed  atomic.Bool
	conn    *AgentSideConnection
	hadConn bool
	sawConn chan bool
}

func (a *awareAgent) SetAgentConnection(conn *AgentSideConnection) {
	time.Sleep(handOffPause)
	a.conn, a.hadConn = conn, conn != nil && conn.Connection() != nil
	a.handed.Store(true)
}

func (a *awareAgent) Initialize(context.Context, InitializeRequest) (InitializeResponse, error) {
	a.sawConn <- a.conn != nil
	return InitializeResponse{}, nil
}

type awareClient struct {
	clientFuncs
	handed  atomic.Bool
	conn    *ClientSideConnection
	hadConn bool
}

func (c *awareClient) SetClientConnection(conn *ClientSideConnection) {
	time.Sleep(handOffPause)
	c.conn, c.hadConn = conn, conn != nil && conn.Connection() != nil
	c.handed.Store(true)
}

// TestConnAwareCalledBeforeReading requires each side's constructor to hand
// a value that implements its ConnAware interface the connection it returns,
// built, before reading starts, with input already waiting, so a handler
// finds it set; run under -race (0019-PLAN-api-decisions.md, B2).
func TestConnAwareCalledBeforeReading(t *testing.T) {
	t.Parallel()
	t.Run("agent", func(t *testing.T) {
		t.Parallel()
		ag := &awareAgent{sawConn: make(chan bool, 1)}
		in := newHandOffInput(t, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":1}}`, &ag.handed)
		asc := NewAgentSideConnection(ag, io.Discard, in)
		if !within(t, in.first, "the first read") {
			t.Error("reading started before SetAgentConnection returned")
		}
		if ag.conn != asc || !ag.hadConn {
			t.Errorf("SetAgentConnection got %p (built: %v), want the returned %p, built", ag.conn, ag.hadConn, asc)
		}
		if !within(t, ag.sawConn, "the initialize handler") {
			t.Error("the initialize handler found the connection unset")
		}
	})
	t.Run("client", func(t *testing.T) {
		t.Parallel()
		cl := &awareClient{}
		in := newHandOffInput(t, `{"jsonrpc":"2.0","method":"_x/n","params":{}}`, &cl.handed)
		csc := NewClientSideConnection(cl, io.Discard, in)
		if !within(t, in.first, "the first read") {
			t.Error("reading started before SetClientConnection returned")
		}
		if cl.conn != csc || !cl.hadConn {
			t.Errorf("SetClientConnection got %p (built: %v), want the returned %p, built", cl.conn, cl.hadConn, csc)
		}
	})
}
