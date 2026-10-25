package acp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"slices"
	"testing"
	"time"
)

// refusingClient implements the two Client methods ClientRefusals leaves out.
type refusingClient struct{ ClientRefusals }

func (refusingClient) SessionUpdate(context.Context, SessionNotification) error { return nil }

func (refusingClient) RequestPermission(context.Context, RequestPermissionRequest) (RequestPermissionResponse, error) {
	return RequestPermissionResponse{Outcome: NewRequestPermissionOutcomeCancelled()}, nil
}

var _ Client = refusingClient{}

// TestClientRefusalsAnswers requires each refused method to answer -32601
// naming its own method, and CompleteElicitation to return nil
// (0036-PLAN-session-update-constructors-kind-and-client-refusals.md, step 4).
func TestClientRefusalsAnswers(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	var r ClientRefusals
	for _, c := range []struct {
		name   string
		method string
		call   func() error
	}{
		{"CreateElicitation", ClientMethodElicitationCreate, func() error {
			_, err := r.CreateElicitation(ctx, CreateElicitationRequest{})
			return err
		}},
		{"ReadTextFile", ClientMethodFsReadTextFile, func() error { _, err := r.ReadTextFile(ctx, ReadTextFileRequest{}); return err }},
		{"WriteTextFile", ClientMethodFsWriteTextFile, func() error { _, err := r.WriteTextFile(ctx, WriteTextFileRequest{}); return err }},
		{"CreateTerminal", ClientMethodTerminalCreate, func() error { _, err := r.CreateTerminal(ctx, CreateTerminalRequest{}); return err }},
		{"KillTerminal", ClientMethodTerminalKill, func() error { _, err := r.KillTerminal(ctx, KillTerminalRequest{}); return err }},
		{"TerminalOutput", ClientMethodTerminalOutput, func() error { _, err := r.TerminalOutput(ctx, TerminalOutputRequest{}); return err }},
		{"ReleaseTerminal", ClientMethodTerminalRelease, func() error {
			_, err := r.ReleaseTerminal(ctx, ReleaseTerminalRequest{})
			return err
		}},
		{"WaitForTerminalExit", ClientMethodTerminalWaitForExit, func() error {
			_, err := r.WaitForTerminalExit(ctx, WaitForTerminalExitRequest{})
			return err
		}},
	} {
		err := c.call()
		re, ok := errors.AsType[*RequestError](err)
		if !ok || re.Code != -32601 || !reflect.DeepEqual(re.Data, map[string]any{"method": c.method}) {
			t.Errorf("%s: %v, want -32601 naming %s", c.name, err, c.method)
		}
	}
	if err := r.CompleteElicitation(ctx, CompleteElicitationNotification{}); err != nil {
		t.Errorf("CompleteElicitation: %v, want nil", err)
	}
}

// TestClientRefusalsMethodSet requires ClientRefusals to provide exactly the
// nine methods a client may leave unadvertised, so that it is no Client alone.
func TestClientRefusalsMethodSet(t *testing.T) {
	t.Parallel()
	var got []string
	for m := range reflect.TypeFor[ClientRefusals]().Methods() {
		got = append(got, m.Name)
	}
	want := []string{
		"CompleteElicitation", "CreateElicitation", "CreateTerminal", "KillTerminal", "ReadTextFile",
		"ReleaseTerminal", "TerminalOutput", "WaitForTerminalExit", "WriteTextFile",
	}
	if !slices.Equal(got, want) {
		t.Errorf("methods = %v, want %v", got, want)
	}
	if reflect.TypeFor[ClientRefusals]().Implements(reflect.TypeFor[Client]()) {
		t.Error("ClientRefusals implements Client by itself")
	}
}

// TestClientRefusalsOverTheWire requires an advertised method a client
// leaves to ClientRefusals to reach the agent as -32601 naming the method.
func TestClientRefusalsOverTheWire(t *testing.T) {
	t.Parallel()
	c2aR, c2aW := io.Pipe()
	a2cR, a2cW := io.Pipe()
	csc := NewClientSideConnection(refusingClient{}, c2aW, a2cR)
	asc := NewAgentSideConnection(agentFuncs{}, a2cW, c2aR)
	t.Cleanup(func() { _ = csc.Close(); _ = asc.Close() })
	n := versionOnly()
	n.Request.ClientCapabilities.Fs.ReadTextFile = true
	negotiate(t, csc, n)
	negotiate(t, asc, n)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	_, err := asc.ReadTextFile(ctx, ReadTextFileRequest{Path: testAbsPath("notes.txt"), SessionId: "s"})
	re, ok := errors.AsType[*RequestError](err)
	if !ok || re.Code != -32601 {
		t.Fatalf("ReadTextFile: %v, want a -32601 *RequestError", err)
	}
	data, err := json.Marshal(re.Data)
	if err != nil || string(data) != `{"method":"fs/read_text_file"}` {
		t.Errorf("ReadTextFile: data %s (%v), want {\"method\":\"fs/read_text_file\"}", data, err)
	}
}

// refusingAgent implements the seven ungated Agent methods AgentRefusals
// leaves out.
type refusingAgent struct{ AgentRefusals }

func (refusingAgent) Authenticate(context.Context, AuthenticateRequest) (AuthenticateResponse, error) {
	return AuthenticateResponse{}, nil
}

func (refusingAgent) Initialize(context.Context, InitializeRequest) (InitializeResponse, error) {
	return InitializeResponse{ProtocolVersion: ProtocolVersionNumber}, nil
}

func (refusingAgent) Cancel(context.Context, CancelNotification) error { return nil }

func (refusingAgent) NewSession(context.Context, NewSessionRequest) (NewSessionResponse, error) {
	return NewSessionResponse{SessionId: "s"}, nil
}

func (refusingAgent) Prompt(context.Context, PromptRequest) (PromptResponse, error) {
	return PromptResponse{StopReason: StopReasonEndTurn}, nil
}

func (refusingAgent) SetSessionConfigOption(context.Context, SetSessionConfigOptionRequest) (SetSessionConfigOptionResponse, error) {
	return SetSessionConfigOptionResponse{}, nil
}

func (refusingAgent) SetSessionMode(context.Context, SetSessionModeRequest) (SetSessionModeResponse, error) {
	return SetSessionModeResponse{}, nil
}

var _ Agent = refusingAgent{}

// TestAgentRefusalsAnswers requires each of the five to answer -32601 naming
// its own method (0038-PLAN-consumer-helpers-kind-and-agent-refusals.md,
// step 4).
func TestAgentRefusalsAnswers(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	var r AgentRefusals
	for _, c := range []struct {
		name   string
		method string
		call   func() error
	}{
		{"CloseSession", AgentMethodSessionClose, func() error { _, err := r.CloseSession(ctx, CloseSessionRequest{}); return err }},
		{"DeleteSession", AgentMethodSessionDelete, func() error { _, err := r.DeleteSession(ctx, DeleteSessionRequest{}); return err }},
		{"ListSessions", AgentMethodSessionList, func() error { _, err := r.ListSessions(ctx, ListSessionsRequest{}); return err }},
		{"ResumeSession", AgentMethodSessionResume, func() error { _, err := r.ResumeSession(ctx, ResumeSessionRequest{}); return err }},
		{"Logout", AgentMethodLogout, func() error { _, err := r.Logout(ctx, LogoutRequest{}); return err }},
	} {
		err := c.call()
		re, ok := errors.AsType[*RequestError](err)
		if !ok || re.Code != -32601 || !reflect.DeepEqual(re.Data, map[string]any{"method": c.method}) {
			t.Errorf("%s: %v, want -32601 naming %s", c.name, err, c.method)
		}
	}
}

// TestAgentRefusalsMethodSet requires AgentRefusals to provide exactly the
// five gated Agent methods, so that it is no Agent alone.
func TestAgentRefusalsMethodSet(t *testing.T) {
	t.Parallel()
	var got []string
	for m := range reflect.TypeFor[AgentRefusals]().Methods() {
		got = append(got, m.Name)
	}
	want := []string{"CloseSession", "DeleteSession", "ListSessions", "Logout", "ResumeSession"}
	if !slices.Equal(got, want) {
		t.Errorf("methods = %v, want %v", got, want)
	}
	if reflect.TypeFor[AgentRefusals]().Implements(reflect.TypeFor[Agent]()) {
		t.Error("AgentRefusals implements Agent by itself")
	}
}

// TestAgentRefusalsOverTheWire requires an advertised method an agent
// leaves to AgentRefusals to reach the client as -32601 naming the method.
func TestAgentRefusalsOverTheWire(t *testing.T) {
	t.Parallel()
	c2aR, c2aW := io.Pipe()
	a2cR, a2cW := io.Pipe()
	asc := NewAgentSideConnection(refusingAgent{}, a2cW, c2aR)
	csc := NewClientSideConnection(&clientFuncs{}, c2aW, a2cR)
	t.Cleanup(func() { _ = csc.Close(); _ = asc.Close() })
	n := versionOnly()
	n.Response.AgentCapabilities.SessionCapabilities.List = &SessionListCapabilities{}
	negotiate(t, csc, n)
	negotiate(t, asc, n)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	_, err := csc.ListSessions(ctx, ListSessionsRequest{})
	re, ok := errors.AsType[*RequestError](err)
	if !ok || re.Code != -32601 {
		t.Fatalf("ListSessions: %v, want a -32601 *RequestError", err)
	}
	data, err := json.Marshal(re.Data)
	if err != nil || string(data) != `{"method":"session/list"}` {
		t.Errorf("ListSessions: data %s (%v), want {\"method\":\"session/list\"}", data, err)
	}
}
