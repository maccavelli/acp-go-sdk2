package agentproc

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	acp "github.com/maccavelli/acp-go-sdk2"
	"github.com/maccavelli/acp-go-sdk2/acptest"
)

// lockedBuffer is a bytes.Buffer safe for concurrent use.
type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

// TestTranscriptLines requires a Transcript to write one record a frame, its
// direction by the writer it came through, a line split across writes as one
// frame, a line that is not JSON as a JSON string, and an unended line as
// nothing (0042-MADR-in-process-agent-runner.md, item 4).
func TestTranscriptLines(t *testing.T) {
	t.Parallel()
	var b lockedBuffer
	tr := NewTranscript(&b)
	for _, w := range []struct {
		to io.Writer
		s  string
	}{
		{tr.Input(), `{"jsonrpc":"2.0","id":1,`},
		{tr.Input(), `"method":"_x/a"}` + "\n"},
		{tr.Output(), "not json\n"},
		{tr.Output(), `{"unended":`},
	} {
		if _, err := w.to.Write([]byte(w.s)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tr.Close(); err != nil {
		t.Fatalf("Close = %v", err)
	}
	got, err := acptest.ReadRecords(strings.NewReader(b.String()))
	if err != nil {
		t.Fatal(err)
	}
	want := []acptest.Record{
		{Direction: acptest.Sent, Frame: json.RawMessage(`{"jsonrpc":"2.0","id":1,"method":"_x/a"}`)},
		{Direction: acptest.Received, Frame: json.RawMessage(`"not json"`)},
	}
	if len(got) != len(want) {
		t.Fatalf("the transcript has %d records, want %d:\n%s", len(got), len(want), b.String())
	}
	for i := range want {
		if got[i].Direction != want[i].Direction || !bytes.Equal(got[i].Frame, want[i].Frame) {
			t.Errorf("record %d = %s %s, want %s %s", i, got[i].Direction, got[i].Frame, want[i].Direction, want[i].Frame)
		}
	}
}

// TestTranscriptWriteError requires Close to return the first error a write
// to the Transcript's writer returned.
func TestTranscriptWriteError(t *testing.T) {
	t.Parallel()
	tr := NewTranscript(failing{})
	if _, err := tr.Output().Write([]byte("{}\n")); err != nil {
		t.Fatal(err)
	}
	if err := tr.Close(); err == nil || !strings.Contains(err.Error(), "the tap fails") {
		t.Errorf("Close = %v, want the writer's error", err)
	}
}

// transcriptAgent is serveAgent whose Prompt sends three updates and asks a
// permission, then ends its turn.
type transcriptAgent struct {
	serveAgent
	conn *acp.AgentSideConnection
}

func (a *transcriptAgent) SetAgentConnection(c *acp.AgentSideConnection) { a.conn = c }

func (a *transcriptAgent) Prompt(ctx context.Context, p acp.PromptRequest) (acp.PromptResponse, error) {
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

// transcriptClient takes updates and allows a permission.
type transcriptClient struct{ acp.ClientRefusals }

func (transcriptClient) SessionUpdate(context.Context, acp.SessionNotification) error { return nil }

func (transcriptClient) RequestPermission(_ context.Context, p acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
	return acp.RequestPermissionResponse{Outcome: acp.NewRequestPermissionOutcomeSelected(p.Options[0].OptionId)}, nil
}

// TestTranscriptOfSession requires a Transcript set as both taps of a real
// session, an in-process agent served by acp.ServeAgent, to read back with
// acptest.ReadRecords, to validate with acptest.ValidateGolden, and to decode
// its session/updates with acptest.Decode.
func TestTranscriptOfSession(t *testing.T) {
	t.Parallel()
	var b lockedBuffer
	tr := NewTranscript(&b)
	a := &InProcess{
		Grace:     testGrace,
		InputTap:  tr.Input(),
		OutputTap: tr.Output(),
		Serve: func(ctx context.Context, in io.Reader, out io.Writer) error {
			return acp.ServeAgent(ctx, &transcriptAgent{}, in, out, acp.WithLogger(slog.New(slog.DiscardHandler)))
		},
		ConnectionOptions: []acp.ConnectionOption{acp.WithLogger(slog.New(slog.DiscardHandler))},
	}
	p, err := a.Start(transcriptClient{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Conn().Initialize(t.Context(), acp.InitializeRequest{ProtocolVersion: acp.ProtocolVersionNumber}); err != nil {
		t.Fatalf("Initialize = %v", err)
	}
	if _, err := p.Conn().NewSession(t.Context(), acp.NewSessionRequest{Cwd: rootDir(t), McpServers: []acp.McpServer{}}); err != nil {
		t.Fatalf("NewSession = %v", err)
	}
	if _, err := p.Conn().Prompt(t.Context(), acp.PromptRequest{SessionId: "s", Prompt: []acp.ContentBlock{acp.TextBlock("hi")}}); err != nil {
		t.Fatalf("Prompt = %v", err)
	}
	if err := p.Close(); err != nil {
		t.Fatalf("Close = %v", err)
	}
	if err := tr.Close(); err != nil {
		t.Fatalf("the transcript's Close = %v", err)
	}
	records, err := acptest.ReadRecords(strings.NewReader(b.String()))
	if err != nil {
		t.Fatal(err)
	}
	acptest.ValidateGolden(t, records)
	updates, err := acptest.Decode[acp.SessionNotification](records, acptest.Received, acp.ClientMethodSessionUpdate)
	if err != nil {
		t.Fatal(err)
	}
	if len(updates) != 3 {
		t.Errorf("the transcript holds %d updates, want 3:\n%s", len(updates), b.String())
	}
	asks, err := acptest.Decode[acp.RequestPermissionRequest](records, acptest.Received, acp.ClientMethodSessionRequestPermission)
	if err != nil || len(asks) != 1 {
		t.Errorf("the transcript holds %d permission requests, %v; want 1", len(asks), err)
	}
	if _, err := acptest.Decode[acp.PromptRequest](records, acptest.Sent, acp.AgentMethodSessionPrompt); err != nil {
		t.Errorf("Decode of the prompt = %v", err)
	}
}

// blockingWriter holds each Write until release is closed. It signals entered
// at its first Write, and counts the Writes that returned.
type blockingWriter struct {
	release, entered chan struct{}
	once             sync.Once

	mu sync.Mutex
	n  int
}

func (w *blockingWriter) Write(p []byte) (int, error) {
	w.once.Do(func() { close(w.entered) })
	<-w.release
	w.mu.Lock()
	defer w.mu.Unlock()
	w.n++
	return len(p), nil
}

func (w *blockingWriter) writes() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.n
}

// TestTranscriptCloseAfterAbandon requires Transcript.Close, called while a
// tap that Wait abandoned is still inside the Transcript's writer, to race
// with nothing and to end the record: nothing reaches w once Close has
// returned (0049-PLAN-subpackage-fixes.md, phase 3, P7). The agent writes
// 5 000 notifications of a method the client does not know, which it does
// not answer, in one write, which the tap is writing when Wait's grace
// passes, and one more that is still queued, so the tap is abandoned:
// DroppedTapBytes is above 0, the precondition the pass's probe could not
// check. Once the writer is released, the tap goes on through the burst while
// Close waits for the lock; the burst outlasts a mutex's 1 ms before it hands
// the lock to a waiter, so Close takes it within the burst, and a frame
// written after Close returned is seen.
func TestTranscriptCloseAfterAbandon(t *testing.T) {
	t.Parallel()
	w := &blockingWriter{release: make(chan struct{}), entered: make(chan struct{})}
	var released sync.Once
	release := func() { released.Do(func() { close(w.release) }) }
	t.Cleanup(release)
	tr := NewTranscript(w)
	p := startInProcess(t, &InProcess{Grace: 100 * time.Millisecond, OutputTap: tr.Output(),
		Serve: func(ctx context.Context, _ io.Reader, out io.Writer) error {
			if _, err := io.WriteString(out, strings.Repeat(`{"jsonrpc":"2.0","method":"x"}`+"\n", 5000)); err != nil {
				return err
			}
			select {
			case <-w.entered: // the tap is writing the first frame
			case <-ctx.Done():
				return context.Cause(ctx)
			}
			_, err := io.WriteString(out, `{"jsonrpc":"2.0","method":"y"}`+"\n")
			return err
		}})
	_ = p.Wait()
	if output, _ := p.DroppedTapBytes(); output == 0 {
		t.Fatal("the output tap was not abandoned: DroppedTapBytes is 0, so the test proves nothing")
	}
	go func() {
		time.Sleep(20 * time.Millisecond)
		release()
	}()
	if err := tr.Close(); err != nil {
		t.Errorf("Close = %v", err)
	}
	at := w.writes()
	time.Sleep(100 * time.Millisecond) // the abandoned tap goes on through the burst
	if got := w.writes(); at < 1 || got != at {
		t.Errorf("the writer had taken %d records when Close returned, and %d records 100ms later; want none after Close", at, got)
	}
}
