package acptest

import (
	"context"
	"strings"
	"sync"

	acp "github.com/maccavelli/acp-go-sdk2"
)

// MemoryClient is an acp.Client over memory. ReadTextFile and WriteTextFile
// serve files by path: a read takes a 1-based line and a limit, as the
// protocol defines them, and a missing path is answered
// acp.NewResourceNotFound. RequestPermission answers the outcome set for the
// request's tool call, and cancelled for one with none. SessionUpdate keeps
// each notification. The other methods, those acp.ClientRefusals covers,
// answer method-not-found. The zero MemoryClient is ready; it is safe for
// concurrent use (0039-MADR-five-adopter-additions-on-schema-1-25-0.md,
// item 5).
type MemoryClient struct {
	acp.ClientRefusals

	mu          sync.Mutex
	files       map[string]string
	permissions map[acp.ToolCallId]acp.RequestPermissionOutcome
	updates     []acp.SessionNotification
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
