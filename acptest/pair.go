package acptest

import (
	"errors"
	"io"
	"testing"

	acp "github.com/maccavelli/acp-go-sdk2"
)

// Pair is an agent-side and a client-side connection joined to each other.
type Pair struct {
	Agent  *acp.AgentSideConnection
	Client *acp.ClientSideConnection
}

// Connect joins agent and client over two pipes, each side through its
// public constructor with opts, and returns the pair. When the test ends, it
// closes both and waits for them, failing the test if either ended for a
// reason other than a close, so no goroutine of theirs outlives the test
// (0039-MADR-five-adopter-additions-on-schema-1-25-0.md, item 5).
func Connect(tb testing.TB, agent acp.Agent, client acp.Client, opts ...acp.ConnectionOption) Pair {
	tb.Helper()
	agentReads, clientWrites := io.Pipe()
	clientReads, agentWrites := io.Pipe()
	p := Pair{
		Agent:  acp.NewAgentSideConnection(agent, agentWrites, agentReads, opts...),
		Client: acp.NewClientSideConnection(client, clientWrites, clientReads, opts...),
	}
	tb.Cleanup(func() {
		if err := p.Close(); err != nil {
			tb.Errorf("acptest: closing the pair: %v", err)
		}
		for side, err := range map[string]error{"agent": p.Agent.Wait(), "client": p.Client.Wait()} {
			if !errors.Is(err, acp.ErrConnectionClosed) && !errors.Is(err, acp.ErrPeerDisconnected) {
				tb.Errorf("acptest: the %s side ended: %v", side, err)
			}
		}
	})
	return p
}

// Close closes both connections, and their pipes with them, and returns
// their errors joined.
func (p Pair) Close() error { return errors.Join(p.Agent.Close(), p.Client.Close()) }

// Wait blocks until both connections have ended and every goroutine they
// started has returned, and returns why each ended, joined, as
// acp.Connection.Err reports it. It does not end them: Close does.
func (p Pair) Wait() error { return errors.Join(p.Agent.Wait(), p.Client.Wait()) }
