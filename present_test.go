package acp_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	acp "github.com/maccavelli/acp-go-sdk2"
	"github.com/maccavelli/acp-go-sdk2/acptest"
)

// presentPair connects a ScriptAgent and a MemoryClient and initializes them,
// the client advertising session.notices and session.compaction as asked.
func presentPair(t *testing.T, notices, compaction bool) (acptest.Pair, *acptest.MemoryClient) {
	t.Helper()
	client := &acptest.MemoryClient{}
	pair := acptest.Connect(t, &acptest.ScriptAgent{}, client)
	var session *acp.ClientSessionCapabilities
	if notices || compaction {
		session = &acp.ClientSessionCapabilities{}
		if notices {
			session.Notices = &acp.NoticeCapabilities{}
		}
		if compaction {
			session.Compaction = &acp.CompactionCapabilities{}
		}
	}
	req := acp.InitializeRequest{ProtocolVersion: acp.ProtocolVersionNumber, ClientCapabilities: acp.ClientCapabilities{Session: session}}
	if _, err := pair.Client.Initialize(t.Context(), req); err != nil {
		t.Fatal(err)
	}
	return pair, client
}

// received sends a final agent message and waits until the client has it, so
// every update sent before it has arrived, in order; it returns those, each
// as its JSON.
func received(t *testing.T, pair acptest.Pair, client *acptest.MemoryClient) []string {
	t.Helper()
	if err := pair.Agent.SessionUpdate(t.Context(), acp.SessionNotification{SessionId: "s", Update: acp.UpdateAgentMessageText("end")}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		updates := client.Updates()
		if n := len(updates); n > 0 && updates[n-1].Update.AgentMessageChunk != nil {
			if b, err := json.Marshal(updates[n-1].Update); err == nil && strings.Contains(string(b), `"end"`) {
				var out []string
				for _, u := range updates[:n-1] {
					b, err := json.Marshal(u.Update)
					if err != nil {
						t.Fatal(err)
					}
					out = append(out, string(b))
				}
				return out
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("the client did not receive the final message; it has %d updates", len(updates))
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// TestPresentNotice sends the notice variant to a client that advertised
// session.notices, and the agent message of its title and description to one
// that did not, where UpdateNotice sent directly still fails.
func TestPresentNotice(t *testing.T) {
	t.Parallel()
	n := acp.Notice{Severity: acp.NoticeSeverityWarning, Title: "Quota low", Description: new("10% left")}

	pair, client := presentPair(t, true, false)
	if err := acp.PresentNotice(t.Context(), pair.Agent, "s", n); err != nil {
		t.Fatal(err)
	}
	if got, want := received(t, pair, client), []string{
		`{"description":"10% left","sessionUpdate":"notice","severity":"warning","title":"Quota low"}`,
	}; strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("with session.notices: %q, want %q", got, want)
	}

	pair, client = presentPair(t, false, true)
	if err := acp.PresentNotice(t.Context(), pair.Agent, "s", n); err != nil {
		t.Fatal(err)
	}
	if err := acp.PresentNotice(t.Context(), pair.Agent, "s", acp.Notice{Severity: acp.NoticeSeverityInfo, Title: "Plain"}); err != nil {
		t.Fatal(err)
	}
	if got, want := received(t, pair, client), []string{
		`{"content":{"text":"Quota low\n10% left","type":"text"},"sessionUpdate":"agent_message_chunk"}`,
		`{"content":{"text":"Plain","type":"text"},"sessionUpdate":"agent_message_chunk"}`,
	}; strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("without session.notices: %q, want %q", got, want)
	}
	err := pair.Agent.SessionUpdate(t.Context(), acp.SessionNotification{SessionId: "s", Update: acp.UpdateNotice(acp.NoticeSeverityInfo, "x")})
	if _, ok := errors.AsType[*acp.CapabilityError](err); !ok {
		t.Errorf("UpdateNotice without session.notices: %v, want a *CapabilityError", err)
	}
}

// TestPresentCompaction sends each status as the compaction variant to a
// client that advertised session.compaction, and "Compaction <status>", with
// the error, to one that did not; a summary chunk goes as its variant, or
// not at all.
func TestPresentCompaction(t *testing.T) {
	t.Parallel()
	statuses := []acp.CompactionStatus{acp.CompactionStatusCompleted, acp.CompactionStatusFailed, acp.CompactionStatusCancelled}
	chunk := acp.CompactionSummaryChunk{CompactionId: "c", Content: acp.TextBlock("summary")}

	pair, client := presentPair(t, false, true)
	var want []string
	for _, st := range statuses {
		u := acp.CompactionUpdate{CompactionId: "c", Status: st}
		if st == acp.CompactionStatusFailed {
			u.Error = acp.PatchValue("out of memory")
		}
		if err := acp.PresentCompaction(t.Context(), pair.Agent, "s", u); err != nil {
			t.Fatal(err)
		}
		v := `{"compactionId":"c",`
		if st == acp.CompactionStatusFailed {
			v += `"error":"out of memory",`
		}
		want = append(want, v+`"sessionUpdate":"compaction_update","status":"`+string(st)+`"}`)
	}
	if err := acp.PresentCompactionSummary(t.Context(), pair.Agent, "s", chunk); err != nil {
		t.Fatal(err)
	}
	want = append(want, `{"compactionId":"c","content":{"text":"summary","type":"text"},"sessionUpdate":"compaction_summary_chunk"}`)
	if got := received(t, pair, client); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("with session.compaction:\n got %q\nwant %q", got, want)
	}

	pair, client = presentPair(t, true, false)
	want = nil
	for _, st := range statuses {
		u := acp.CompactionUpdate{CompactionId: "c", Status: st}
		line := "Compaction " + string(st)
		if st == acp.CompactionStatusFailed {
			u.Error = acp.PatchValue("out of memory")
			line += `\nout of memory`
		}
		if err := acp.PresentCompaction(t.Context(), pair.Agent, "s", u); err != nil {
			t.Fatal(err)
		}
		want = append(want, `{"content":{"text":"`+line+`","type":"text"},"sessionUpdate":"agent_message_chunk"}`)
	}
	if err := acp.PresentCompactionSummary(t.Context(), pair.Agent, "s", chunk); err != nil {
		t.Fatalf("a summary without session.compaction: %v, want nil", err)
	}
	if got := received(t, pair, client); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("without session.compaction:\n got %q\nwant %q", got, want)
	}
}

// TestPresentBeforeInitialize requires each function, before initialize, to
// return the error SessionUpdate does and to write nothing, and to refuse a
// nil connection.
func TestPresentBeforeInitialize(t *testing.T) {
	t.Parallel()
	var rec acptest.Recorder
	agentReads, clientWrites := io.Pipe()
	clientReads, agentWrites := io.Pipe()
	pair := acptest.Pair{
		Agent:  acp.NewAgentSideConnection(&acptest.ScriptAgent{}, rec.Writer(agentWrites), agentReads),
		Client: acp.NewClientSideConnection(&acptest.MemoryClient{}, clientWrites, clientReads),
	}
	t.Cleanup(func() {
		if err := pair.Close(); err != nil {
			t.Error(err)
		}
		if err := pair.Wait(); !errors.Is(err, acp.ErrConnectionClosed) {
			t.Error(err)
		}
	})
	ctx := t.Context()
	for name, call := range map[string]func(*acp.AgentSideConnection) error{
		"PresentNotice": func(a *acp.AgentSideConnection) error {
			return acp.PresentNotice(ctx, a, "s", acp.Notice{Severity: acp.NoticeSeverityInfo, Title: "t"})
		},
		"PresentCompaction": func(a *acp.AgentSideConnection) error {
			return acp.PresentCompaction(ctx, a, "s", acp.CompactionUpdate{CompactionId: "c", Status: acp.CompactionStatusCompleted})
		},
		"PresentCompactionSummary": func(a *acp.AgentSideConnection) error {
			return acp.PresentCompactionSummary(ctx, a, "s", acp.CompactionSummaryChunk{CompactionId: "c", Content: acp.TextBlock("x")})
		},
	} {
		if err := call(pair.Agent); !errors.Is(err, acp.ErrNotInitialized) {
			t.Errorf("%s before initialize: %v, want ErrNotInitialized", name, err)
		}
		if err := call(nil); err == nil {
			t.Errorf("%s with a nil connection: no error", name)
		}
	}
	if frames := rec.Frames(); len(frames) != 0 {
		t.Errorf("the agent wrote %d frames before initialize: %s", len(frames), frames[0])
	}
}

// With a client that did not advertise session.notices, PresentNotice sends
// the notice as an agent message, the description on a line after the title.
func ExamplePresentNotice() {
	agentReads, clientWrites := io.Pipe()
	clientReads, agentWrites := io.Pipe()
	client := &acptest.MemoryClient{}
	pair := acptest.Pair{
		Agent:  acp.NewAgentSideConnection(&acptest.ScriptAgent{}, agentWrites, agentReads),
		Client: acp.NewClientSideConnection(client, clientWrites, clientReads),
	}
	ctx := context.Background()
	if _, err := pair.Client.Initialize(ctx, acp.InitializeRequest{ProtocolVersion: acp.ProtocolVersionNumber}); err != nil {
		fmt.Println(err)
	}
	desc := "Requests are slower than usual."
	n := acp.Notice{Severity: acp.NoticeSeverityWarning, Title: "Degraded service", Description: &desc}
	if err := acp.PresentNotice(ctx, pair.Agent, "s", n); err != nil {
		fmt.Println(err)
	}
	for len(client.Updates()) == 0 {
		time.Sleep(time.Millisecond)
	}
	b, err := json.Marshal(client.Updates()[0].Update)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(string(b))
	if err := pair.Close(); err != nil {
		fmt.Println(err)
	}
	fmt.Println("closed:", errors.Is(pair.Wait(), acp.ErrConnectionClosed))
	// Output:
	// {"content":{"text":"Degraded service\nRequests are slower than usual.","type":"text"},"sessionUpdate":"agent_message_chunk"}
	// closed: true
}
