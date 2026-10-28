package acptest_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	acp "github.com/maccavelli/acp-go-sdk2"
	"github.com/maccavelli/acp-go-sdk2/acptest"
)

var quiet = acp.WithLogger(slog.New(slog.DiscardHandler))

// askingAgent numbers its sessions, and its Prompt sends three updates and
// asks a permission before it ends its turn.
type askingAgent struct {
	acp.AgentRefusals
	conn     *acp.AgentSideConnection
	sessions atomic.Int64
}

func (a *askingAgent) SetAgentConnection(c *acp.AgentSideConnection) { a.conn = c }

func (*askingAgent) Initialize(context.Context, acp.InitializeRequest) (acp.InitializeResponse, error) {
	return acp.InitializeResponse{ProtocolVersion: acp.ProtocolVersionNumber}, nil
}

func (*askingAgent) Authenticate(context.Context, acp.AuthenticateRequest) (acp.AuthenticateResponse, error) {
	return acp.AuthenticateResponse{}, nil
}

func (a *askingAgent) NewSession(context.Context, acp.NewSessionRequest) (acp.NewSessionResponse, error) {
	return acp.NewSessionResponse{SessionId: acp.SessionId(fmt.Sprintf("session-%d", a.sessions.Add(1)))}, nil
}

func (a *askingAgent) Prompt(ctx context.Context, p acp.PromptRequest) (acp.PromptResponse, error) {
	for _, text := range []string{"one", "two", "three"} {
		if err := a.conn.SessionUpdate(ctx, acp.SessionNotification{SessionId: p.SessionId, Update: acp.UpdateAgentMessageText(text)}); err != nil {
			return acp.PromptResponse{}, err
		}
	}
	if _, err := a.conn.RequestPermission(ctx, acp.RequestPermissionRequest{
		SessionId: p.SessionId,
		ToolCall:  acp.ToolCallUpdate{ToolCallId: "t"},
		Options:   []acp.PermissionOption{{OptionId: "ok", Name: "Allow", Kind: acp.PermissionOptionKindAllowOnce}},
	}); err != nil {
		return acp.PromptResponse{}, err
	}
	return acp.PromptResponse{StopReason: acp.StopReasonEndTurn}, nil
}

func (*askingAgent) Cancel(context.Context, acp.CancelNotification) error { return nil }

func (*askingAgent) SetSessionMode(context.Context, acp.SetSessionModeRequest) (acp.SetSessionModeResponse, error) {
	return acp.SetSessionModeResponse{}, nil
}

func (*askingAgent) SetSessionConfigOption(context.Context, acp.SetSessionConfigOptionRequest) (acp.SetSessionConfigOptionResponse, error) {
	return acp.SetSessionConfigOptionResponse{}, nil
}

// countingClient counts updates and permission requests, and allows each.
type countingClient struct {
	acp.ClientRefusals
	updates, asks atomic.Int64
}

func (c *countingClient) SessionUpdate(context.Context, acp.SessionNotification) error {
	c.updates.Add(1)
	return nil
}

func (c *countingClient) RequestPermission(_ context.Context, p acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
	c.asks.Add(1)
	return acp.RequestPermissionResponse{Outcome: acp.NewRequestPermissionOutcomeSelected(p.Options[0].OptionId)}, nil
}

// session runs a session on c: initialize, two sessions, and a prompt on the
// first, and returns the two session ids and the prompt's stop reason.
func session(t *testing.T, c *acp.ClientSideConnection) ([]acp.SessionId, acp.StopReason) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	if _, err := c.Initialize(ctx, acp.InitializeRequest{ProtocolVersion: acp.ProtocolVersionNumber}); err != nil {
		t.Fatalf("Initialize = %v", err)
	}
	var ids []acp.SessionId
	for range 2 {
		s, err := c.NewSession(ctx, acp.NewSessionRequest{Cwd: acptest.Abs("work"), McpServers: []acp.McpServer{}})
		if err != nil {
			t.Fatalf("NewSession = %v", err)
		}
		ids = append(ids, s.SessionId)
	}
	r, err := c.Prompt(ctx, acp.PromptRequest{SessionId: ids[0], Prompt: []acp.ContentBlock{acp.TextBlock("hi")}})
	if err != nil {
		t.Fatalf("Prompt = %v", err)
	}
	return ids, r.StopReason
}

// recorded is the records of a real session with an askingAgent.
func recorded(t *testing.T) []acptest.Record {
	t.Helper()
	rec := &acptest.Recorder{}
	pair := acptest.ConnectRecorded(t, &askingAgent{}, &countingClient{}, rec, quiet)
	session(t, pair.Client)
	return rec.Records()
}

// serveReplay runs a ReplayAgent of records for client over two pipes,
// recording the client's side, and returns the client's connection, the
// recorder, and end, which closes the connection and returns Serve's result.
func serveReplay(t *testing.T, records []acptest.Record, client acp.Client) (*acp.ClientSideConnection, *acptest.Recorder, func() error) {
	t.Helper()
	agentReads, clientWrites := io.Pipe()
	clientReads, agentWrites := io.Pipe()
	served := make(chan error, 1)
	go func() {
		err := (&acptest.ReplayAgent{Records: records}).Serve(context.Background(), agentReads, agentWrites)
		_ = agentWrites.Close()
		_ = agentReads.Close()
		served <- err
	}()
	rec := &acptest.Recorder{}
	c := acp.NewClientSideConnection(client, rec.Writer(clientWrites), rec.Reader(clientReads), quiet)
	end := sync.OnceValue(func() error {
		_ = c.Close()
		select {
		case err := <-served:
			return err
		case <-time.After(10 * time.Second):
			return errors.New("the replay did not end")
		}
	})
	t.Cleanup(func() { _ = end() })
	return c, rec, end
}

// TestReplayAgentPlays requires a replay of a recorded session to give a
// client that repeats it the same answers, updates and permission request,
// the k-th request of a method meeting the k-th recorded answer, the agent's
// own request under a fresh id, and Serve to return nil
// (0042-MADR-in-process-agent-runner.md, item 4, question 4c).
func TestReplayAgentPlays(t *testing.T) {
	t.Parallel()
	client := &countingClient{}
	c, rec, end := serveReplay(t, recorded(t), client)
	ids, stop := session(t, c)
	if fmt.Sprint(ids) != "[session-1 session-2]" || stop != acp.StopReasonEndTurn {
		t.Errorf("the replayed session gave %v and %q, want [session-1 session-2] and end_turn", ids, stop)
	}
	if client.updates.Load() != 3 || client.asks.Load() != 1 {
		t.Errorf("the client received %d updates and %d permission requests, want 3 and 1", client.updates.Load(), client.asks.Load())
	}
	asks, err := acptest.Decode[json.RawMessage](rec.Records(), acptest.Received, acp.ClientMethodSessionRequestPermission)
	if err != nil || len(asks) != 1 {
		t.Fatalf("the replay sent %d permission requests, %v", len(asks), err)
	}
	for _, r := range rec.Records() {
		if r.Direction == acptest.Received && strings.Contains(string(r.Frame), `"method":"session/request_permission"`) &&
			!strings.Contains(string(r.Frame), `"id":"replay-1"`) {
			t.Errorf("the agent's request went as %s, want the fresh id \"replay-1\"", r.Frame)
		}
	}
	if err := end(); err != nil {
		t.Errorf("Serve = %v, want nil: the client repeated the recording", err)
	}
}

// TestReplayAgentLiveIDs requires a recorded answer to go under the live
// request's id: the recording's ids, 100 on, are not the client's.
func TestReplayAgentLiveIDs(t *testing.T) {
	t.Parallel()
	records := []acptest.Record{
		{Direction: acptest.Sent, Frame: json.RawMessage(`{"jsonrpc":"2.0","id":100,"method":"initialize","params":{"protocolVersion":1,"clientCapabilities":{}}}`)},
		{Direction: acptest.Received, Frame: json.RawMessage(`{"jsonrpc":"2.0","id":100,"result":{"protocolVersion":1}}`)},
	}
	c, _, end := serveReplay(t, records, &countingClient{})
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	if _, err := c.Initialize(ctx, acp.InitializeRequest{ProtocolVersion: acp.ProtocolVersionNumber}); err != nil {
		t.Fatalf("Initialize = %v, want the recorded answer under the live id", err)
	}
	if err := end(); err != nil {
		t.Errorf("Serve = %v, want nil", err)
	}
}

// TestReplayAgentExtraRequest requires a request beyond the recording to be
// answered -32603 naming its method, and Serve to report it.
func TestReplayAgentExtraRequest(t *testing.T) {
	t.Parallel()
	c, _, end := serveReplay(t, recorded(t), &countingClient{})
	session(t, c)
	_, err := c.NewSession(t.Context(), acp.NewSessionRequest{Cwd: acptest.Abs("work"), McpServers: []acp.McpServer{}})
	var re *acp.RequestError
	if !errors.As(err, &re) || re.Code != -32603 || !strings.Contains(re.Error(), "no recorded answer to session/new") {
		t.Errorf("the extra NewSession = %v, want -32603 naming session/new", err)
	}
	if err := end(); err == nil || !strings.Contains(err.Error(), "requests the recording does not answer: session/new") {
		t.Errorf("Serve = %v, want the extra request named", err)
	}
}

// TestReplayAgentStopsEarly requires Serve to name the recorded frames a
// client that stops early left unplayed.
func TestReplayAgentStopsEarly(t *testing.T) {
	t.Parallel()
	c, _, end := serveReplay(t, recorded(t), &countingClient{})
	if _, err := c.Initialize(t.Context(), acp.InitializeRequest{ProtocolVersion: acp.ProtocolVersionNumber}); err != nil {
		t.Fatal(err)
	}
	err := end()
	if err == nil || !strings.Contains(err.Error(), "recorded frames not played: response session/new") {
		t.Errorf("Serve = %v, want the frames not played named, from the first session/new answer", err)
	}
}

// TestReplayAgentUnordered requires a recording whose two ways are out of
// order, every Sent record after every Received one, to replay as well.
func TestReplayAgentUnordered(t *testing.T) {
	t.Parallel()
	var received, sent []acptest.Record
	for _, r := range recorded(t) {
		if r.Direction == acptest.Sent {
			sent = append(sent, r)
		} else {
			received = append(received, r)
		}
	}
	c, _, end := serveReplay(t, append(received, sent...), &countingClient{})
	if ids, stop := session(t, c); fmt.Sprint(ids) != "[session-1 session-2]" || stop != acp.StopReasonEndTurn {
		t.Errorf("the replayed session gave %v and %q", ids, stop)
	}
	if err := end(); err != nil {
		t.Errorf("Serve = %v, want nil", err)
	}
}

// TestConnectReplay requires ConnectReplay to join a client to a replay that
// ends cleanly when the test does.
func TestConnectReplay(t *testing.T) {
	t.Parallel()
	c := acptest.ConnectReplay(t, recorded(t), &countingClient{}, quiet)
	if ids, _ := session(t, c); fmt.Sprint(ids) != "[session-1 session-2]" {
		t.Errorf("the replayed session gave %v", ids)
	}
}
