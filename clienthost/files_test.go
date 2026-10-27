package clienthost

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	acp "github.com/maccavelli/acp-go-sdk2"
	"github.com/maccavelli/acp-go-sdk2/acptest"
)

// TestReadTextFileLines requires ReadTextFile to return the lines line and
// limit select, line 1-based, each with its end as the file has it, "\r\n"
// kept; a line past the end to give no content, and line 0 to be refused
// as invalid params (0041-PLAN-close-the-gaps-the-consumers-wait-on.md,
// phase 9).
func TestReadTextFileLines(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(path, []byte("one\ntwo\r\nthree\nfour"), 0o600); err != nil {
		t.Fatal(err)
	}
	ended := filepath.Join(dir, "ended.txt")
	if err := os.WriteFile(ended, []byte("a\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	empty := filepath.Join(dir, "empty.txt")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name        string
		path        string
		line, limit *uint32
		want        string
	}{
		{"neither", path, nil, nil, "one\ntwo\r\nthree\nfour"},
		{"line 1", path, new(uint32(1)), nil, "one\ntwo\r\nthree\nfour"},
		{"line 2", path, new(uint32(2)), nil, "two\r\nthree\nfour"},
		{"the last line, not ended", path, new(uint32(4)), nil, "four"},
		{"past the end", path, new(uint32(5)), nil, ""},
		{"line 2, limit 1", path, new(uint32(2)), new(uint32(1)), "two\r\n"},
		{"limit 2", path, nil, new(uint32(2)), "one\ntwo\r\n"},
		{"limit 0", path, nil, new(uint32(0)), ""},
		{"a limit past the end", path, new(uint32(3)), new(uint32(10)), "three\nfour"},
		{"past an ended last line", ended, new(uint32(2)), nil, ""},
		{"an ended last line", ended, new(uint32(1)), new(uint32(1)), "a\n"},
		{"an empty file", empty, new(uint32(1)), nil, ""},
	} {
		resp, err := Files{}.ReadTextFile(t.Context(), acp.ReadTextFileRequest{SessionId: "s", Path: c.path, Line: c.line, Limit: c.limit})
		if err != nil || resp.Content != c.want {
			t.Errorf("%s: ReadTextFile = %q, %v; want %q", c.name, resp.Content, err, c.want)
		}
	}
	if _, err := (Files{}).ReadTextFile(t.Context(), acp.ReadTextFileRequest{SessionId: "s", Path: path, Line: new(uint32(0))}); code(err) != -32602 {
		t.Errorf("ReadTextFile at line 0 = %v, want invalid params (-32602)", err)
	}
}

// TestReadTextFileRefused requires a missing file to be resource-not-found,
// a relative path invalid params, and a directory refused otherwise.
func TestReadTextFileRefused(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	ctx := t.Context()
	_, err := Files{}.ReadTextFile(ctx, acp.ReadTextFileRequest{SessionId: "s", Path: filepath.Join(dir, "missing.txt")})
	if code(err) != -32002 || !acp.IsResourceNotFound(err) {
		t.Errorf("ReadTextFile of a missing file = %v, want resource-not-found (-32002)", err)
	}
	if _, err := (Files{}).ReadTextFile(ctx, acp.ReadTextFileRequest{SessionId: "s", Path: "a.txt"}); code(err) != -32602 {
		t.Errorf("ReadTextFile of a relative path = %v, want invalid params (-32602)", err)
	}
	if _, err := (Files{}).ReadTextFile(ctx, acp.ReadTextFileRequest{SessionId: "s", Path: dir}); err == nil || code(err) == -32002 {
		t.Errorf("ReadTextFile of a directory = %v, want a refusal that is not resource-not-found", err)
	}
}

// TestWriteTextFile requires WriteTextFile to create a missing file, to
// replace an existing one's content and keep its mode, to answer a missing
// directory with resource-not-found unless MkdirAll, which creates it, and to
// refuse a relative path and a directory.
func TestWriteTextFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	ctx := t.Context()
	read := func(path string) string {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	path := filepath.Join(dir, "new.txt")
	if _, err := (Files{}).WriteTextFile(ctx, acp.WriteTextFileRequest{SessionId: "s", Path: path, Content: "first\r\n"}); err != nil ||
		read(path) != "first\r\n" {
		t.Errorf("WriteTextFile of a new file = %v, content %q", err, read(path))
	}
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := (Files{}).WriteTextFile(ctx, acp.WriteTextFileRequest{SessionId: "s", Path: path, Content: "second"}); err != nil ||
		read(path) != "second" {
		t.Errorf("WriteTextFile over a file = %v, content %q; want it replaced", err, read(path))
	}
	if fi, err := os.Stat(path); runtime.GOOS != "windows" && (err != nil || fi.Mode().Perm() != 0o600) {
		t.Errorf("the file's mode after a write = %v, %v; want 0600 kept", fi.Mode(), err)
	}
	deep := filepath.Join(dir, "a", "b", "c.txt")
	_, err := Files{}.WriteTextFile(ctx, acp.WriteTextFileRequest{SessionId: "s", Path: deep, Content: "x"})
	if code(err) != -32002 {
		t.Errorf("WriteTextFile under a missing directory = %v, want resource-not-found (-32002)", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "a")); err == nil {
		t.Error("WriteTextFile without MkdirAll created the missing directory")
	}
	if _, err := (Files{MkdirAll: true}).WriteTextFile(ctx, acp.WriteTextFileRequest{SessionId: "s", Path: deep, Content: "x"}); err != nil ||
		read(deep) != "x" {
		t.Errorf("WriteTextFile with MkdirAll = %v", err)
	}
	if _, err := (Files{}).WriteTextFile(ctx, acp.WriteTextFileRequest{SessionId: "s", Path: "a.txt", Content: "x"}); code(err) != -32602 {
		t.Errorf("WriteTextFile of a relative path = %v, want invalid params (-32602)", err)
	}
	if _, err := (Files{}).WriteTextFile(ctx, acp.WriteTextFileRequest{SessionId: "s", Path: dir, Content: "x"}); err == nil {
		t.Error("WriteTextFile of a directory = nil, want a refusal")
	}
}

// TestFilesRefuseDevice requires both methods to refuse a path that is not a
// regular file, such as a device, which a read could otherwise never finish
// and a write would not keep: /dev/null, on Unix.
func TestFilesRefuseDevice(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" || runtime.GOOS == "plan9" {
		t.Skip("no /dev/null here")
	}
	if _, err := (Files{}).ReadTextFile(t.Context(), acp.ReadTextFileRequest{SessionId: "s", Path: os.DevNull}); err == nil {
		t.Errorf("ReadTextFile of %s = nil, want a refusal: it is not a regular file", os.DevNull)
	}
	if _, err := (Files{}).WriteTextFile(t.Context(), acp.WriteTextFileRequest{SessionId: "s", Path: os.DevNull, Content: "x"}); err == nil {
		t.Errorf("WriteTextFile of %s = nil, want a refusal: it is not a regular file", os.DevNull)
	}
}

// fileClient serves the file-system methods with Files, with MkdirAll, and
// the terminal methods with Terminals, beside Refusals, as the package doc
// shows.
type fileClient struct {
	Terminals
	Files
	Refusals
}

func (*fileClient) SessionUpdate(context.Context, acp.SessionNotification) error { return nil }

func (*fileClient) RequestPermission(context.Context, acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
	return acp.RequestPermissionResponse{}, nil
}

// TestFilesOverConnection requires a client embedding Terminals, Files and
// Refusals to serve the file-system methods over a connection, a missing
// file reaching the agent as resource-not-found, which IsResourceNotFound
// reads.
func TestFilesOverConnection(t *testing.T) {
	t.Parallel()
	client := &fileClient{Terminals: Terminals{Command: testCommand}, Files: Files{MkdirAll: true}}
	t.Cleanup(func() { _ = client.Close() })
	pair := acptest.Connect(t, &acptest.ScriptAgent{}, client)
	ctx := t.Context()
	if _, err := pair.Client.Initialize(ctx, acp.InitializeRequest{ProtocolVersion: acp.ProtocolVersionNumber,
		ClientCapabilities: acp.ClientCapabilities{Terminal: true, Fs: acp.FileSystemCapabilities{ReadTextFile: true, WriteTextFile: true}}}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "dir", "f.txt")
	if _, err := pair.Agent.WriteTextFile(ctx, acp.WriteTextFileRequest{SessionId: "s", Path: path, Content: "l1\nl2\n"}); err != nil {
		t.Fatalf("fs/write_text_file = %v", err)
	}
	got, err := pair.Agent.ReadTextFile(ctx, acp.ReadTextFileRequest{SessionId: "s", Path: path, Line: new(uint32(2))})
	if err != nil || got.Content != "l2\n" {
		t.Errorf("fs/read_text_file = %+v, %v; want %q", got, err, "l2\n")
	}
	_, err = pair.Agent.ReadTextFile(ctx, acp.ReadTextFileRequest{SessionId: "s", Path: filepath.Join(filepath.Dir(path), "missing.txt")})
	if !acp.IsResourceNotFound(err) {
		t.Errorf("fs/read_text_file of a missing file = %v, want resource-not-found", err)
	}
	created, err := pair.Agent.CreateTerminal(ctx, acp.CreateTerminalRequest{SessionId: "s", Command: "write"})
	if err != nil {
		t.Fatalf("terminal/create beside Files = %v", err)
	}
	if _, err := pair.Agent.ReleaseTerminal(ctx, acp.ReleaseTerminalRequest{SessionId: "s", TerminalId: created.TerminalId}); err != nil {
		t.Errorf("terminal/release = %v", err)
	}
}
