package clienthost_test

import (
	"context"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	acp "github.com/maccavelli/acp-go-sdk2"
	"github.com/maccavelli/acp-go-sdk2/clienthost"
)

// client serves the terminal methods with Terminals and the file-system
// methods with Files, and refuses the other optional client methods with
// Refusals.
type client struct {
	clienthost.Terminals
	clienthost.Files
	clienthost.Refusals
}

func (*client) SessionUpdate(context.Context, acp.SessionNotification) error { return nil }

// RequestPermission of a real client asks its user.
func (*client) RequestPermission(context.Context, acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
	return acp.RequestPermissionResponse{Outcome: acp.NewRequestPermissionOutcomeCancelled()}, nil
}

// A client that runs the agent's terminal commands, its policy in Command:
// it refuses a command whose cwd is outside the project, and runs the rest
// directly. Close ends every terminal when the client is done.
func ExampleTerminals() {
	const project = "/work/project"
	c := &client{}
	c.Command = func(ctx context.Context, req acp.CreateTerminalRequest) (*exec.Cmd, error) {
		if req.Cwd == nil {
			dir := project
			req.Cwd = &dir
		}
		if rel, err := filepath.Rel(project, *req.Cwd); err != nil || !filepath.IsLocal(rel) {
			return nil, acp.NewInvalidParams(map[string]any{"cwd": "outside the project"})
		}
		return clienthost.DirectCommand(ctx, req)
	}
	defer func() {
		if err := c.Close(); err != nil {
			log.Printf("terminals: %v", err)
		}
	}()
	conn := acp.NewClientSideConnection(c, os.Stdout, os.Stdin)
	if _, err := conn.Initialize(context.Background(), acp.InitializeRequest{ProtocolVersion: acp.ProtocolVersionNumber,
		ClientCapabilities: acp.ClientCapabilities{Terminal: true}}); err != nil {
		log.Printf("initialize: %v", err)
	}
}

// rooted is a client whose file-system methods keep to its project: its own
// ReadTextFile and WriteTextFile win over the embedded Files', check the
// path, and call Files' for a path they allow.
type rooted struct {
	client
	project string
}

// inside refuses a path outside the project.
func (r *rooted) inside(path string) error {
	if rel, err := filepath.Rel(r.project, path); err != nil || !filepath.IsLocal(rel) {
		return acp.NewInvalidParams(map[string]any{"path": "outside the project"})
	}
	return nil
}

func (r *rooted) ReadTextFile(ctx context.Context, req acp.ReadTextFileRequest) (acp.ReadTextFileResponse, error) {
	if err := r.inside(req.Path); err != nil {
		return acp.ReadTextFileResponse{}, err
	}
	return r.Files.ReadTextFile(ctx, req)
}

func (r *rooted) WriteTextFile(ctx context.Context, req acp.WriteTextFileRequest) (acp.WriteTextFileResponse, error) {
	if err := r.inside(req.Path); err != nil {
		return acp.WriteTextFileResponse{}, err
	}
	return r.Files.WriteTextFile(ctx, req)
}

// A client that serves the agent's file reads and writes inside its project,
// creating a written file's missing directories.
func ExampleFiles() {
	r := &rooted{project: "/work/project"}
	r.MkdirAll = true
	conn := acp.NewClientSideConnection(r, os.Stdout, os.Stdin)
	if _, err := conn.Initialize(context.Background(), acp.InitializeRequest{ProtocolVersion: acp.ProtocolVersionNumber,
		ClientCapabilities: acp.ClientCapabilities{Fs: acp.FileSystemCapabilities{ReadTextFile: true, WriteTextFile: true}}}); err != nil {
		log.Printf("initialize: %v", err)
	}
}
