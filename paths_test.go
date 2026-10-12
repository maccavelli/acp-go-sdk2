package acp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// testAbsPath is a path under the host's temporary directory, absolute on
// every host, for tests that send a path the protocol requires to be
// absolute; a POSIX literal such as "/" is not absolute on Windows
// (0015-PLAN-validate-absolute-protocol-paths.md, step 4).
func testAbsPath(elem ...string) string {
	return filepath.Join(append([]string{os.TempDir()}, elem...)...)
}

// TestIsPortableAbs pins the rule for a path the SDK sends: absolute in POSIX
// or Windows form, on every host (0015-MADR-validate-absolute-protocol-paths.md,
// A4).
func TestIsPortableAbs(t *testing.T) {
	t.Parallel()
	for _, p := range []string{"/a", "/", `C:\a`, "C:/a", `c:\`, `\\server\share`, `\\?\C:\a`, "//server/share"} {
		if !isPortableAbs(p) {
			t.Errorf("isPortableAbs(%q) = false, want true", p)
		}
	}
	for _, p := range []string{"", "a", "./a", "../a", `\a`, "C:a", "C:", "1:/a", ":/a", "~/a"} {
		if isPortableAbs(p) {
			t.Errorf("isPortableAbs(%q) = true, want false", p)
		}
	}
	if p := testAbsPath("x"); !filepath.IsAbs(p) || !isPortableAbs(p) {
		t.Errorf("testAbsPath(%q) is not absolute on this host", p)
	}
}

// countingWriter counts the bytes written to it.
type countingWriter struct {
	mu sync.Mutex
	n  int
}

func (w *countingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.n += len(p)
	return len(p), nil
}

func (w *countingWriter) count() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.n
}

// pathCase builds a message that carries one of the 17 properties as p, and
// names the type and field a failure reports.
type pathCase struct {
	name, typ, field string
	build            func(p string) any
}

// pathCases are the 17 properties the protocol requires to be absolute, each
// in a message that carries it, nested ones through their parents.
func pathCases() []pathCase {
	abs := testAbsPath()
	return []pathCase{
		{"session/new cwd", "NewSessionRequest", "cwd", func(p string) any { return NewSessionRequest{Cwd: p} }},
		{"session/new additionalDirectories", "NewSessionRequest", "additionalDirectories[0]",
			func(p string) any { return NewSessionRequest{Cwd: abs, AdditionalDirectories: []string{p}} }},
		{"session/load cwd", "LoadSessionRequest", "cwd", func(p string) any { return LoadSessionRequest{SessionId: "s", Cwd: p} }},
		{"session/load additionalDirectories", "LoadSessionRequest", "additionalDirectories[0]",
			func(p string) any {
				return LoadSessionRequest{SessionId: "s", Cwd: abs, AdditionalDirectories: []string{p}}
			}},
		{"session/resume cwd", "ResumeSessionRequest", "cwd", func(p string) any { return ResumeSessionRequest{SessionId: "s", Cwd: p} }},
		{"session/resume additionalDirectories", "ResumeSessionRequest", "additionalDirectories[0]",
			func(p string) any {
				return ResumeSessionRequest{SessionId: "s", Cwd: abs, AdditionalDirectories: []string{p}}
			}},
		{"session/fork cwd", "UnstableForkSessionRequest", "cwd",
			func(p string) any { return UnstableForkSessionRequest{SessionId: "s", Cwd: p} }},
		{"session/fork additionalDirectories", "UnstableForkSessionRequest", "additionalDirectories[0]",
			func(p string) any {
				return UnstableForkSessionRequest{SessionId: "s", Cwd: abs, AdditionalDirectories: []string{p}}
			}},
		{"session/list cwd", "ListSessionsRequest", "cwd", func(p string) any { return ListSessionsRequest{Cwd: &p} }},
		{"session/list result cwd", "SessionInfo", "cwd",
			func(p string) any { return ListSessionsResponse{Sessions: []SessionInfo{{SessionId: "s", Cwd: p}}} }},
		{"session/list result additionalDirectories", "SessionInfo", "additionalDirectories[0]",
			func(p string) any {
				return ListSessionsResponse{Sessions: []SessionInfo{{SessionId: "s", Cwd: abs, AdditionalDirectories: []string{p}}}}
			}},
		{"MCP server command", "McpServerStdio", "command",
			func(p string) any {
				return NewSessionRequest{Cwd: abs, McpServers: []McpServer{{Stdio: &McpServerStdio{Name: "m", Command: p}}}}
			}},
		{"fs/read_text_file path", "ReadTextFileRequest", "path", func(p string) any { return ReadTextFileRequest{SessionId: "s", Path: p} }},
		{"fs/write_text_file path", "WriteTextFileRequest", "path", func(p string) any { return WriteTextFileRequest{SessionId: "s", Path: p} }},
		{"terminal/create cwd", "CreateTerminalRequest", "cwd",
			func(p string) any { return CreateTerminalRequest{SessionId: "s", Command: "c", Cwd: &p} }},
		{"tool call location", "ToolCallLocation", "path", func(p string) any {
			return SessionNotification{SessionId: "s", Update: StartToolCall("t", "read", WithStartLocations([]ToolCallLocation{{Path: p}}))}
		}},
		{"diff path", "ToolCallContentDiff", "path", func(p string) any {
			return SessionNotification{SessionId: "s", Update: StartToolCall("t", "edit", WithStartContent([]ToolCallContent{ToolDiffContent(p, "x")}))}
		}},
	}
}

// TestSendRefusesRelativePaths requires sending to refuse each of the 17
// properties when it is empty or relative, writing nothing, and to send it
// when it is absolute in POSIX or Windows form, whatever the host
// (0015-PLAN-validate-absolute-protocol-paths.md, step 5).
func TestSendRefusesRelativePaths(t *testing.T) {
	t.Parallel()
	pr, pw := io.Pipe()
	t.Cleanup(func() { _ = pw.Close() })
	w := &countingWriter{}
	c := NewConnection(func(context.Context, string, json.RawMessage) (any, *RequestError) { return nil, nil }, w, pr)
	cases := pathCases()
	if len(cases) != 17 {
		t.Fatalf("%d cases, want the 17 properties", len(cases))
	}
	for _, tc := range cases {
		for _, bad := range []string{"", "relative/dir"} {
			before := w.count()
			err := c.SendNotification(t.Context(), "x/send", tc.build(bad))
			var ve *ValidationError
			if !errors.As(err, &ve) || ve.Rule != RuleAbsolutePath || ve.Type != tc.typ || ve.Field != tc.field {
				t.Errorf("%s = %q: SendNotification() = %v, want an absolute-path error on %s %s", tc.name, bad, err, tc.typ, tc.field)
			}
			if w.count() != before {
				t.Errorf("%s = %q: %d bytes were written", tc.name, bad, w.count()-before)
			}
		}
		for _, good := range []string{"/a", `C:\a`} {
			before := w.count()
			if err := c.SendNotification(t.Context(), "x/send", tc.build(good)); err != nil || w.count() == before {
				t.Errorf("%s = %q: SendNotification() = %v, %d bytes written", tc.name, good, err, w.count()-before)
			}
		}
	}
	// A request is checked before anything is written, too; the timeout makes
	// a request that is sent anyway fail rather than wait for a reply.
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	before := w.count()
	_, err := SendRequest[json.RawMessage](c, ctx, "x/request", &ReadTextFileRequest{SessionId: "s", Path: "rel"})
	var ve *ValidationError
	if !errors.As(err, &ve) || ve.Field != "path" || w.count() != before {
		t.Errorf("SendRequest() = %v, %d bytes written, want an absolute-path error and nothing written", err, w.count()-before)
	}
}

// syncBuffer is a bytes.Buffer safe for the connection's goroutines.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.String()
}

// TestHandlerResultValidated requires a handler's result that would send a
// relative path to be answered -32603 naming the type and property, and to
// be logged (0015-PLAN-validate-absolute-protocol-paths.md, step 5).
func TestHandlerResultValidated(t *testing.T) {
	t.Parallel()
	c2aR, c2aW := io.Pipe()
	a2cR, a2cW := io.Pipe()
	t.Cleanup(func() {
		_ = c2aW.Close()
		_ = a2cW.Close()
	})
	var logs syncBuffer
	NewAgentSideConnection(agentFuncs{ListSessionsFunc: func(context.Context, ListSessionsRequest) (ListSessionsResponse, error) {
		return ListSessionsResponse{Sessions: []SessionInfo{{SessionId: "s", Cwd: "rel"}}}, nil
	}}, a2cW, c2aR, WithLogger(slog.New(slog.NewTextHandler(&logs, nil))))
	cs := NewClientSideConnection(&clientFuncs{}, c2aW, a2cR)
	_, err := cs.ListSessions(t.Context(), ListSessionsRequest{})
	var re *RequestError
	if !errors.As(err, &re) || re.Code != -32603 || !strings.Contains(fmt.Sprint(re.Data), `SessionInfo: cwd: "rel" is not an absolute path`) {
		t.Fatalf("ListSessions() = %v, want -32603 naming SessionInfo cwd", err)
	}
	if !strings.Contains(logs.String(), "handler result fails validation") {
		t.Errorf("the agent did not log the refused result: %q", logs.String())
	}
}
