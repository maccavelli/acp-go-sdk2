package acptest

import (
	"context"
	"fmt"
	"strings"
	"sync"

	acp "github.com/maccavelli/acp-go-sdk2"
)

// MemoryClient is an acp.Client over memory. ReadTextFile and WriteTextFile
// serve files by path: a read takes a 1-based line and a limit, as the
// protocol defines them, and a missing path is answered
// acp.NewResourceNotFound. RequestPermission answers the outcome set for the
// request's tool call, and cancelled for one with none. SessionUpdate keeps
// each notification. CreateElicitation answers from the script SetElicitation
// sets, and the terminal methods are Terminals', when each is set. The other
// methods, those acp.ClientRefusals covers, answer method-not-found. The zero
// MemoryClient is ready; it is safe for concurrent use
// (0039-MADR-five-adopter-additions-on-schema-1-25-0.md, item 5;
// 0041-MADR-close-the-gaps-the-consumers-wait-on.md, item 8).
type MemoryClient struct {
	acp.ClientRefusals

	// Terminals, when set, serves the five terminal methods: a
	// *clienthost.Terminals whose Command builds the scripted commands. Nil
	// answers method-not-found. Set it before connecting the client.
	Terminals TerminalHost

	mu          sync.Mutex
	files       map[string]string
	permissions map[acp.ToolCallId]acp.RequestPermissionOutcome
	updates     []acp.SessionNotification
	elicit      func(acp.CreateElicitationRequest) acp.CreateElicitationResponse
}

// TerminalHost serves the five terminal methods for a MemoryClient, as
// *clienthost.Terminals does. It is an interface here so that this package
// does not import clienthost, whose tests use this package.
type TerminalHost interface {
	CreateTerminal(ctx context.Context, params acp.CreateTerminalRequest) (acp.CreateTerminalResponse, error)
	TerminalOutput(ctx context.Context, params acp.TerminalOutputRequest) (acp.TerminalOutputResponse, error)
	WaitForTerminalExit(ctx context.Context, params acp.WaitForTerminalExitRequest) (acp.WaitForTerminalExitResponse, error)
	KillTerminal(ctx context.Context, params acp.KillTerminalRequest) (acp.KillTerminalResponse, error)
	ReleaseTerminal(ctx context.Context, params acp.ReleaseTerminalRequest) (acp.ReleaseTerminalResponse, error)
}

// SetElicitation sets the script CreateElicitation answers from: answer is
// called with each request, and what it returns is checked as
// acp.WithElicitationCheck checks it, an accept of a form against the form,
// before it is sent.
func (c *MemoryClient) SetElicitation(answer func(acp.CreateElicitationRequest) acp.CreateElicitationResponse) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.elicit = answer
}

// CreateElicitation answers params from the script SetElicitation set. An
// accept of a form that its form refuses is not sent: it is an error, which
// the agent sees as an internal error, so a script that answers wrong fails
// the test that runs it. With no script, it answers method-not-found.
func (c *MemoryClient) CreateElicitation(ctx context.Context, params acp.CreateElicitationRequest) (acp.CreateElicitationResponse, error) {
	c.mu.Lock()
	answer := c.elicit
	c.mu.Unlock()
	if answer == nil {
		return c.ClientRefusals.CreateElicitation(ctx, params)
	}
	resp := answer(params)
	if params.Form != nil && resp.Accept != nil {
		if err := params.Form.RequestedSchema.Check(resp.Accept.Content); err != nil {
			return acp.CreateElicitationResponse{}, fmt.Errorf("acptest: the script's answer fails its form: %w", err)
		}
	}
	return resp, nil
}

// CreateTerminal is Terminals', or method-not-found without them.
func (c *MemoryClient) CreateTerminal(ctx context.Context, params acp.CreateTerminalRequest) (acp.CreateTerminalResponse, error) {
	if c.Terminals == nil {
		return c.ClientRefusals.CreateTerminal(ctx, params)
	}
	return c.Terminals.CreateTerminal(ctx, params)
}

// TerminalOutput is Terminals', or method-not-found without them.
func (c *MemoryClient) TerminalOutput(ctx context.Context, params acp.TerminalOutputRequest) (acp.TerminalOutputResponse, error) {
	if c.Terminals == nil {
		return c.ClientRefusals.TerminalOutput(ctx, params)
	}
	return c.Terminals.TerminalOutput(ctx, params)
}

// WaitForTerminalExit is Terminals', or method-not-found without them.
func (c *MemoryClient) WaitForTerminalExit(ctx context.Context, params acp.WaitForTerminalExitRequest) (acp.WaitForTerminalExitResponse, error) {
	if c.Terminals == nil {
		return c.ClientRefusals.WaitForTerminalExit(ctx, params)
	}
	return c.Terminals.WaitForTerminalExit(ctx, params)
}

// KillTerminal is Terminals', or method-not-found without them.
func (c *MemoryClient) KillTerminal(ctx context.Context, params acp.KillTerminalRequest) (acp.KillTerminalResponse, error) {
	if c.Terminals == nil {
		return c.ClientRefusals.KillTerminal(ctx, params)
	}
	return c.Terminals.KillTerminal(ctx, params)
}

// ReleaseTerminal is Terminals', or method-not-found without them.
func (c *MemoryClient) ReleaseTerminal(ctx context.Context, params acp.ReleaseTerminalRequest) (acp.ReleaseTerminalResponse, error) {
	if c.Terminals == nil {
		return c.ClientRefusals.ReleaseTerminal(ctx, params)
	}
	return c.Terminals.ReleaseTerminal(ctx, params)
}

var _ acp.Client = (*MemoryClient)(nil)

// SetFile stores content at path.
func (c *MemoryClient) SetFile(path, content string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.files == nil {
		c.files = map[string]string{}
	}
	c.files[path] = content
}

// File returns the content stored at path, and whether there is any.
func (c *MemoryClient) File(path string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	content, ok := c.files[path]
	return content, ok
}

// SetPermission sets the outcome RequestPermission answers for a request
// about the tool call id.
func (c *MemoryClient) SetPermission(id acp.ToolCallId, outcome acp.RequestPermissionOutcome) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.permissions == nil {
		c.permissions = map[acp.ToolCallId]acp.RequestPermissionOutcome{}
	}
	c.permissions[id] = outcome
}

// Updates returns a copy of the session notifications received so far, in
// the order they arrived.
func (c *MemoryClient) Updates() []acp.SessionNotification {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]acp.SessionNotification(nil), c.updates...)
}

// ReadTextFile answers the file at params.Path, from its 1-based Line, the
// first when unset, for at most Limit lines, all when unset. A line past the
// last answers no content; Line 0, which names no line, is invalid params.
func (c *MemoryClient) ReadTextFile(_ context.Context, params acp.ReadTextFileRequest) (acp.ReadTextFileResponse, error) {
	content, ok := c.File(params.Path)
	if !ok {
		return acp.ReadTextFileResponse{}, acp.NewResourceNotFound(params.Path)
	}
	if params.Line == nil && params.Limit == nil {
		return acp.ReadTextFileResponse{Content: content}, nil
	}
	first := 1
	if params.Line != nil {
		if *params.Line == 0 {
			return acp.ReadTextFileResponse{}, acp.NewInvalidParams(map[string]any{"line": 0, "reason": "lines are 1-based"})
		}
		first = int(*params.Line)
	}
	lines := strings.SplitAfter(content, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if first > len(lines) {
		return acp.ReadTextFileResponse{}, nil
	}
	lines = lines[first-1:]
	if params.Limit != nil && int(*params.Limit) < len(lines) {
		lines = lines[:*params.Limit]
	}
	return acp.ReadTextFileResponse{Content: strings.Join(lines, "")}, nil
}

// WriteTextFile stores params.Content at params.Path.
func (c *MemoryClient) WriteTextFile(_ context.Context, params acp.WriteTextFileRequest) (acp.WriteTextFileResponse, error) {
	c.SetFile(params.Path, params.Content)
	return acp.WriteTextFileResponse{}, nil
}

// RequestPermission answers the outcome set for the tool call, cancelled when
// none is.
func (c *MemoryClient) RequestPermission(_ context.Context, params acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
	c.mu.Lock()
	outcome, ok := c.permissions[params.ToolCall.ToolCallId]
	c.mu.Unlock()
	if !ok {
		outcome = acp.NewRequestPermissionOutcomeCancelled()
	}
	return acp.RequestPermissionResponse{Outcome: outcome}, nil
}

// SessionUpdate keeps params.
func (c *MemoryClient) SessionUpdate(_ context.Context, params acp.SessionNotification) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.updates = append(c.updates, params)
	return nil
}
