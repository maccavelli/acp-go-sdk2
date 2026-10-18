package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/maccavelli/acp-go-sdk2"
)

// TestWriteTextFileStaysInCwd pins that the client writes inside its
// session's working directory and refuses a path outside it, by name or
// through a symbolic link (0019-PLAN-examples-docs-tests-tooling.md, E7).
func TestWriteTextFileStaysInCwd(t *testing.T) {
	cwd := t.TempDir()
	c := &exampleClient{cwd: cwd}
	inside := filepath.Join(cwd, "sub", "a.txt")
	if _, err := c.WriteTextFile(t.Context(), acp.WriteTextFileRequest{Path: inside, Content: "x"}); err != nil {
		t.Fatalf("a write inside the working directory: %v", err)
	}
	if b, err := os.ReadFile(inside); err != nil || string(b) != "x" {
		t.Fatalf("the file written inside: %q, %v", b, err)
	}

	outside := t.TempDir()
	for _, p := range []string{filepath.Join(outside, "b.txt"), filepath.Join(cwd, "..", "c.txt")} {
		_, err := c.WriteTextFile(t.Context(), acp.WriteTextFileRequest{Path: p, Content: "x"})
		if re, ok := errors.AsType[*acp.RequestError](err); !ok || re.Code != int(acp.ErrorCodeInvalidParams) {
			t.Errorf("a write to %s: %v, want invalid params", p, err)
		}
	}

	link := filepath.Join(cwd, "link")
	if err := os.Symlink(outside, link); err != nil {
		t.Logf("no symbolic link on this host, so its escape is not checked: %v", err)
	} else if _, err := c.WriteTextFile(t.Context(), acp.WriteTextFileRequest{Path: filepath.Join(link, "d.txt"), Content: "x"}); err == nil {
		t.Errorf("a write through a link out of the working directory succeeded")
	}
	if entries, err := os.ReadDir(outside); err != nil || len(entries) != 0 {
		t.Errorf("the directory outside holds %v (%v), want nothing", entries, err)
	}
}

// TestTerminalMethodsNotFound pins that the client, which does not advertise
// the terminal capability, answers its methods method-not-found (E7).
func TestTerminalMethodsNotFound(t *testing.T) {
	c := &exampleClient{}
	_, err := c.CreateTerminal(t.Context(), acp.CreateTerminalRequest{})
	if re, ok := errors.AsType[*acp.RequestError](err); !ok || re.Code != int(acp.ErrorCodeMethodNotFound) {
		t.Fatalf("CreateTerminal: %v, want method not found", err)
	}
}

// TestToolCallUpdateLine requires a tool call update's status printed as its
// value, not its pointer, and left out when the update carries none
// (0019-PLAN-issue-fixes.md, E18).
func TestToolCallUpdateLine(t *testing.T) {
	for _, tt := range []struct {
		u    acp.SessionToolCallUpdate
		want string
	}{
		{acp.SessionToolCallUpdate{ToolCallId: "c1", Status: new(acp.ToolCallStatusCompleted)}, "🔧 Tool call `c1` updated: completed"},
		{acp.SessionToolCallUpdate{ToolCallId: "c2"}, "🔧 Tool call `c2` updated"},
	} {
		if got := toolCallUpdateLine(&tt.u); got != tt.want {
			t.Errorf("toolCallUpdateLine(%s) = %q, want %q", tt.u.ToolCallId, got, tt.want)
		}
	}
}
