package acp

import "context"

// ClientRefusals answers method-not-found (-32601) for the Client methods a
// client may leave unadvertised: the file-system, terminal and elicitation
// methods. Embed it in a client that implements only SessionUpdate and
// RequestPermission, which it does not provide, so the compiler still
// requires them.
//
// The SDK refuses a method whose capability the client did not advertise
// before the client's method runs, so these answer only for a capability the
// client did advertise. A method the embedding type declares itself takes
// precedence over the one promoted from ClientRefusals.
//
// A client that implements some of these methods for real should implement
// all of them itself rather than embed ClientRefusals: a real method renamed
// or removed would otherwise be refused without a compile error.
type ClientRefusals struct{}

// CompleteElicitation ignores the notification and returns nil: a
// notification has no answer to refuse with.
func (ClientRefusals) CompleteElicitation(context.Context, CompleteElicitationNotification) error {
	return nil
}

// CreateElicitation refuses elicitation/create with method-not-found.
func (ClientRefusals) CreateElicitation(context.Context, CreateElicitationRequest) (CreateElicitationResponse, error) {
	return CreateElicitationResponse{}, NewMethodNotFound(ClientMethodElicitationCreate)
}

// ReadTextFile refuses fs/read_text_file with method-not-found.
func (ClientRefusals) ReadTextFile(context.Context, ReadTextFileRequest) (ReadTextFileResponse, error) {
	return ReadTextFileResponse{}, NewMethodNotFound(ClientMethodFsReadTextFile)
}

// WriteTextFile refuses fs/write_text_file with method-not-found.
func (ClientRefusals) WriteTextFile(context.Context, WriteTextFileRequest) (WriteTextFileResponse, error) {
	return WriteTextFileResponse{}, NewMethodNotFound(ClientMethodFsWriteTextFile)
}

// CreateTerminal refuses terminal/create with method-not-found.
func (ClientRefusals) CreateTerminal(context.Context, CreateTerminalRequest) (CreateTerminalResponse, error) {
	return CreateTerminalResponse{}, NewMethodNotFound(ClientMethodTerminalCreate)
}

// KillTerminal refuses terminal/kill with method-not-found.
func (ClientRefusals) KillTerminal(context.Context, KillTerminalRequest) (KillTerminalResponse, error) {
	return KillTerminalResponse{}, NewMethodNotFound(ClientMethodTerminalKill)
}

// TerminalOutput refuses terminal/output with method-not-found.
func (ClientRefusals) TerminalOutput(context.Context, TerminalOutputRequest) (TerminalOutputResponse, error) {
	return TerminalOutputResponse{}, NewMethodNotFound(ClientMethodTerminalOutput)
}

// ReleaseTerminal refuses terminal/release with method-not-found.
func (ClientRefusals) ReleaseTerminal(context.Context, ReleaseTerminalRequest) (ReleaseTerminalResponse, error) {
	return ReleaseTerminalResponse{}, NewMethodNotFound(ClientMethodTerminalRelease)
}

// WaitForTerminalExit refuses terminal/wait_for_exit with method-not-found.
func (ClientRefusals) WaitForTerminalExit(context.Context, WaitForTerminalExitRequest) (WaitForTerminalExitResponse, error) {
	return WaitForTerminalExitResponse{}, NewMethodNotFound(ClientMethodTerminalWaitForExit)
}

// AgentRefusals answers method-not-found (-32601) for the five Agent methods
// an agent may leave unadvertised, each gated on one of its own capabilities:
// CloseSession, DeleteSession, ListSessions, ResumeSession and Logout. Embed
// it in an agent that offers none of them; the seven ungated methods,
// Authenticate, Initialize, Cancel, NewSession, Prompt, SetSessionConfigOption
// and SetSessionMode, it does not provide, so the compiler still requires
// them.
//
// The SDK refuses a method whose capability the agent did not advertise
// before the agent's method runs, so these answer only for a capability the
// agent did advertise. A method the embedding type declares itself takes
// precedence over the one promoted from AgentRefusals.
//
// An agent that implements some of these methods for real should implement
// all of them itself rather than embed AgentRefusals: a real method renamed
// or removed would otherwise be refused without a compile error
// (0038-MADR-consumer-helpers-kind-and-agent-refusals.md).
type AgentRefusals struct{}

// CloseSession refuses session/close with method-not-found.
func (AgentRefusals) CloseSession(context.Context, CloseSessionRequest) (CloseSessionResponse, error) {
	return CloseSessionResponse{}, NewMethodNotFound(AgentMethodSessionClose)
}

// DeleteSession refuses session/delete with method-not-found.
func (AgentRefusals) DeleteSession(context.Context, DeleteSessionRequest) (DeleteSessionResponse, error) {
	return DeleteSessionResponse{}, NewMethodNotFound(AgentMethodSessionDelete)
}

// ListSessions refuses session/list with method-not-found.
func (AgentRefusals) ListSessions(context.Context, ListSessionsRequest) (ListSessionsResponse, error) {
	return ListSessionsResponse{}, NewMethodNotFound(AgentMethodSessionList)
}

// ResumeSession refuses session/resume with method-not-found.
func (AgentRefusals) ResumeSession(context.Context, ResumeSessionRequest) (ResumeSessionResponse, error) {
	return ResumeSessionResponse{}, NewMethodNotFound(AgentMethodSessionResume)
}

// Logout refuses logout with method-not-found.
func (AgentRefusals) Logout(context.Context, LogoutRequest) (LogoutResponse, error) {
	return LogoutResponse{}, NewMethodNotFound(AgentMethodLogout)
}
