package acptest

import (
	"bytes"
	"context"
	"errors"
	"io"
	"runtime/pprof"
	"strings"
	"testing"
	"time"

	acp "github.com/maccavelli/acp-go-sdk2"
)

// Pair is an agent-side and a client-side connection joined to each other.
type Pair struct {
	Agent  *acp.AgentSideConnection
	Client *acp.ClientSideConnection
}

// cleanupWait bounds how long Connect's cleanup waits for the pair's
// goroutines once it has closed the pair
// (0041-MADR-close-the-gaps-the-consumers-wait-on.md, item 6).
const cleanupWait = 10 * time.Second

// Connect joins agent and client over two pipes, each side through its
// public constructor with opts, and returns the pair. When the test ends, it
// closes both and waits for them, failing the test if either ended for a
// reason other than a close, so no goroutine of theirs outlives the test
// (0039-MADR-five-adopter-additions-on-schema-1-25-0.md, item 5). It waits up
// to 10 seconds: a handler that ignores its context keeps its side's
// goroutines running, and the test then fails with the goroutines still
// running, rather than hanging
// (0041-MADR-close-the-gaps-the-consumers-wait-on.md, item 6).
func Connect(tb testing.TB, agent acp.Agent, client acp.Client, opts ...acp.ConnectionOption) Pair {
	tb.Helper()
	return connectWithin(tb, cleanupWait, agent, client, nil, opts...)
}

// ConnectRecorded is Connect with the client side's streams recorded into
// rec: a Sent record is a frame the client sent the agent, a Received one a
// frame the agent sent the client, in the order the client wrote and read
// them. ValidateGolden reads records so
// (0041-MADR-close-the-gaps-the-consumers-wait-on.md, item 8).
func ConnectRecorded(tb testing.TB, agent acp.Agent, client acp.Client, rec *Recorder, opts ...acp.ConnectionOption) Pair {
	tb.Helper()
	return connectWithin(tb, cleanupWait, agent, client, rec, opts...)
}

// connectWithin is Connect, its cleanup waiting up to bound, and the client
// side's streams recorded into rec unless it is nil.
func connectWithin(tb testing.TB, bound time.Duration, agent acp.Agent, client acp.Client, rec *Recorder,
	opts ...acp.ConnectionOption) Pair {
	tb.Helper()
	agentReads, clientWrites := io.Pipe()
	clientReads, agentWrites := io.Pipe()
	var w io.Writer = clientWrites
	var r io.Reader = clientReads
	if rec != nil {
		w, r = rec.Writer(clientWrites), rec.Reader(clientReads)
	}
	p := Pair{
		Agent:  acp.NewAgentSideConnection(agent, agentWrites, agentReads, opts...),
		Client: acp.NewClientSideConnection(client, w, r, opts...),
	}
	tb.Cleanup(func() {
		if err := p.Close(); err != nil {
			tb.Errorf("acptest: closing the pair: %v", err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), bound)
		defer cancel()
		sides := []struct {
			name string
			wait func(context.Context) error
		}{{"agent", p.Agent.Connection().WaitContext}, {"client", p.Client.Connection().WaitContext}}
		for _, s := range sides {
			switch err := s.wait(ctx); {
			case errors.Is(err, context.DeadlineExceeded):
				tb.Errorf("acptest: the %s side's goroutines still run %v after the pair was closed: a handler that ignores its context holds them. Running:\n%s",
					s.name, bound, running())
			case !errors.Is(err, acp.ErrConnectionClosed) && !errors.Is(err, acp.ErrPeerDisconnected):
				tb.Errorf("acptest: the %s side ended: %v", s.name, err)
			}
		}
	})
	return p
}

// running returns the stacks of the goroutines that run this module's code,
// grouped with their counts.
func running() string {
	var b bytes.Buffer
	_ = pprof.Lookup("goroutine").WriteTo(&b, 1) //nolint:errcheck // a bytes.Buffer does not fail.
	var out []string
	for group := range strings.SplitSeq(b.String(), "\n\n") {
		if strings.Contains(group, "github.com/maccavelli/acp-go-sdk2") && !strings.Contains(group, "acptest.running") {
			out = append(out, group)
		}
	}
	return strings.Join(out, "\n\n")
}

// Close closes both connections, and their pipes with them, and returns
// their errors joined.
func (p Pair) Close() error { return errors.Join(p.Agent.Close(), p.Client.Close()) }

// Wait blocks until both connections have ended and every goroutine they
// started has returned, and returns why each ended, joined, as
// acp.Connection.Err reports it. It does not end them: Close does.
func (p Pair) Wait() error { return errors.Join(p.Agent.Wait(), p.Client.Wait()) }
