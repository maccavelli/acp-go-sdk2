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
	"slices"
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

// receiveRoutes says how each case of pathCases is received: the method a
// request or notification arrives as, at the agent or the client, or
// "response" for a session/list result.
var receiveRoutes = map[string]struct{ side, method string }{
	"session/new cwd":                           {"agent", AgentMethodSessionNew},
	"session/new additionalDirectories":         {"agent", AgentMethodSessionNew},
	"session/load cwd":                          {"agent", AgentMethodSessionLoad},
	"session/load additionalDirectories":        {"agent", AgentMethodSessionLoad},
	"session/resume cwd":                        {"agent", AgentMethodSessionResume},
	"session/resume additionalDirectories":      {"agent", AgentMethodSessionResume},
	"session/fork cwd":                          {"agent", AgentMethodSessionFork},
	"session/fork additionalDirectories":        {"agent", AgentMethodSessionFork},
	"session/list cwd":                          {"agent", AgentMethodSessionList},
	"session/list result cwd":                   {"response", AgentMethodSessionList},
	"session/list result additionalDirectories": {"response", AgentMethodSessionList},
	"MCP server command":                        {"agent", AgentMethodSessionNew},
	"fs/read_text_file path":                    {"client", ClientMethodFsReadTextFile},
	"fs/write_text_file path":                   {"client", ClientMethodFsWriteTextFile},
	"terminal/create cwd":                       {"client", ClientMethodTerminalCreate},
	"tool call location":                        {"client", ClientMethodSessionUpdate},
	"diff path":                                 {"client", ClientMethodSessionUpdate},
}

// idleReader is a peer that never sends, closed when the test ends.
func idleReader(t *testing.T) io.Reader {
	t.Helper()
	pr, pw := io.Pipe()
	t.Cleanup(func() { _ = pw.Close() })
	return pr
}

// rawPeer starts a client connection whose agent answers every request with
// result, as raw JSON that is not validated on its way out.
func rawPeer(t *testing.T, result string, opts ...ConnectionOption) *ClientSideConnection {
	t.Helper()
	c2aR, c2aW := io.Pipe()
	a2cR, a2cW := io.Pipe()
	t.Cleanup(func() {
		_ = c2aW.Close()
		_ = a2cW.Close()
	})
	NewConnection(func(context.Context, string, json.RawMessage) (any, *RequestError) {
		return json.RawMessage(result), nil
	}, a2cW, c2aR)
	return NewClientSideConnection(&clientFuncs{}, c2aW, a2cR, opts...)
}

// TestReceiveRefusesRelativePaths requires each of the 17 properties, received
// empty or relative, to be refused: a request with -32602 naming the type and
// property, before the handler runs; a notification likewise, so it is
// dropped and not delivered; a response with an error that wraps the
// *ValidationError (0015-PLAN-validate-absolute-protocol-paths.md, step 6).
func TestReceiveRefusesRelativePaths(t *testing.T) {
	t.Parallel()
	delivered := 0
	asc := NewAgentSideConnection(agentFuncs{}, io.Discard, idleReader(t))
	csc := NewClientSideConnection(&clientFuncs{SessionUpdateFunc: func(context.Context, SessionNotification) error {
		delivered++
		return nil
	}}, io.Discard, idleReader(t))
	for _, tc := range pathCases() {
		route, ok := receiveRoutes[tc.name]
		if !ok {
			t.Fatalf("%s: no receive route", tc.name)
		}
		for _, bad := range []string{"", "relative/dir"} {
			raw, err := json.Marshal(tc.build(bad))
			if err != nil {
				t.Fatal(err)
			}
			want := tc.typ + ": " + tc.field + ": "
			switch route.side {
			case "response":
				_, err := rawPeer(t, string(raw)).ListSessions(t.Context(), ListSessionsRequest{})
				var ve *ValidationError
				if !errors.As(err, &ve) || ve.Type != tc.typ || ve.Field != tc.field {
					t.Errorf("%s = %q: ListSessions() = %v, want an error wrapping the absolute-path error", tc.name, bad, err)
				}
			default:
				var rerr *RequestError
				if route.side == "agent" {
					_, rerr = asc.handle(t.Context(), route.method, raw)
				} else {
					_, rerr = csc.handle(t.Context(), route.method, raw)
				}
				if rerr == nil || rerr.Code != -32602 || !strings.Contains(fmt.Sprint(rerr.Data), want) {
					t.Errorf("%s = %q: %s answered %v, want -32602 naming %s", tc.name, bad, route.method, rerr, want)
				}
			}
		}
	}
	if delivered != 0 {
		t.Errorf("%d refused notifications were delivered", delivered)
	}
}

// bareCommand is a session/new whose MCP server command is a bare name, as a
// client that forwards its configuration may send it.
func bareCommand(t *testing.T) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(NewSessionRequest{Cwd: testAbsPath(), McpServers: []McpServer{{Stdio: &McpServerStdio{Name: "m", Command: "npx"}}}})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// TestInboundHooks requires the receiving order of the MADR's H1: the rewrite
// hook changes the decoded message, the SDK validates what it leaves, and
// the validation hook decides with that result; a message that does not
// decode reaches neither (0015-PLAN-validate-absolute-protocol-paths.md, step 6).
func TestInboundHooks(t *testing.T) {
	t.Parallel()
	resolved := filepath.Join(testAbsPath("bin"), "npx")
	newSession := func(got *string) agentFuncs {
		return agentFuncs{NewSessionFunc: func(_ context.Context, p NewSessionRequest) (NewSessionResponse, error) {
			*got = p.McpServers[0].Stdio.Command
			return NewSessionResponse{SessionId: "s"}, nil
		}}
	}
	t.Run("rewrite resolves a bare command", func(t *testing.T) {
		var got string
		asc := NewAgentSideConnection(newSession(&got), io.Discard, idleReader(t), WithInboundRewrite(func(_ context.Context, _ string, params any) error {
			if r, ok := params.(*NewSessionRequest); ok && r.McpServers[0].Stdio.Command == "npx" {
				r.McpServers[0].Stdio.Command = resolved
			}
			return nil
		}))
		if _, rerr := asc.handle(t.Context(), AgentMethodSessionNew, bareCommand(t)); rerr != nil || got != resolved {
			t.Errorf("handle = %v, the handler saw %q, want %q", rerr, got, resolved)
		}
	})
	t.Run("rewrite that leaves the command relative", func(t *testing.T) {
		var got string
		asc := NewAgentSideConnection(newSession(&got), io.Discard, idleReader(t),
			WithInboundRewrite(func(context.Context, string, any) error { return nil }))
		_, rerr := asc.handle(t.Context(), AgentMethodSessionNew, bareCommand(t))
		if rerr == nil || rerr.Code != -32602 || !strings.Contains(fmt.Sprint(rerr.Data), "McpServerStdio: command") || got != "" {
			t.Errorf("handle = %v, the handler saw %q, want -32602 naming the command", rerr, got)
		}
	})
	t.Run("rewrite that fails", func(t *testing.T) {
		asc := NewAgentSideConnection(agentFuncs{}, io.Discard, idleReader(t),
			WithInboundRewrite(func(context.Context, string, any) error { return errors.New("no npx on this host") }))
		_, rerr := asc.handle(t.Context(), AgentMethodSessionNew, bareCommand(t))
		if rerr == nil || rerr.Code != -32602 || !strings.Contains(fmt.Sprint(rerr.Data), "no npx on this host") {
			t.Errorf("handle = %v, want -32602 with the hook's error", rerr)
		}
	})
	t.Run("validation hook accepts a violation", func(t *testing.T) {
		var got string
		asc := NewAgentSideConnection(newSession(&got), io.Discard, idleReader(t),
			WithInboundValidation(func(_ context.Context, _ string, _ any, err error) error {
				var ve *ValidationError
				if errors.As(err, &ve) && ve.Type == "McpServerStdio" && ve.Field == "command" {
					return nil
				}
				return err
			}))
		if _, rerr := asc.handle(t.Context(), AgentMethodSessionNew, bareCommand(t)); rerr != nil || got != "npx" {
			t.Errorf("handle = %v, the handler saw %q, want the accepted npx", rerr, got)
		}
	})
	t.Run("validation hook refuses a valid message", func(t *testing.T) {
		raw, _ := json.Marshal(NewSessionRequest{Cwd: testAbsPath()})
		for _, tt := range []struct {
			hookErr  error
			wantCode int
			wantText string
		}{
			{errors.New("not in this workspace"), -32602, "not in this workspace"},
			{&RequestError{Code: -32001, Message: "denied"}, -32001, "denied"},
		} {
			asc := NewAgentSideConnection(agentFuncs{}, io.Discard, idleReader(t),
				WithInboundValidation(func(_ context.Context, _ string, _ any, err error) error {
					if err != nil {
						return err
					}
					return tt.hookErr
				}))
			_, rerr := asc.handle(t.Context(), AgentMethodSessionNew, raw)
			if rerr == nil || rerr.Code != tt.wantCode || !strings.Contains(rerr.Error(), tt.wantText) {
				t.Errorf("handle = %v, want code %d with %q", rerr, tt.wantCode, tt.wantText)
			}
		}
	})
	t.Run("order on notifications and responses", func(t *testing.T) {
		var mu sync.Mutex
		var order []string
		hooks := []ConnectionOption{
			WithInboundRewrite(func(_ context.Context, method string, params any) error {
				mu.Lock()
				defer mu.Unlock()
				order = append(order, fmt.Sprintf("rewrite %s %T", method, params))
				return nil
			}),
			WithInboundValidation(func(_ context.Context, method string, _ any, err error) error {
				mu.Lock()
				defer mu.Unlock()
				order = append(order, fmt.Sprintf("validate %s %v", method, err))
				return err
			}),
		}
		csc := NewClientSideConnection(&clientFuncs{}, io.Discard, idleReader(t), hooks...)
		note, _ := json.Marshal(SessionNotification{SessionId: "s", Update: StartToolCall("t", "read",
			WithStartLocations([]ToolCallLocation{{Path: testAbsPath("a")}}))})
		if _, rerr := csc.handle(t.Context(), ClientMethodSessionUpdate, note); rerr != nil {
			t.Fatal(rerr)
		}
		list, _ := json.Marshal(ListSessionsResponse{Sessions: []SessionInfo{{SessionId: "s", Cwd: testAbsPath()}}})
		if _, err := rawPeer(t, string(list), hooks...).ListSessions(t.Context(), ListSessionsRequest{}); err != nil {
			t.Fatal(err)
		}
		mu.Lock()
		defer mu.Unlock()
		want := []string{
			"rewrite session/update *acp.SessionNotification", "validate session/update <nil>",
			"rewrite session/list *acp.ListSessionsResponse", "validate session/list <nil>",
		}
		if !slices.Equal(order, want) {
			t.Errorf("hooks ran %q, want %q", order, want)
		}
	})
	t.Run("a message that does not decode reaches no hook", func(t *testing.T) {
		calls := 0
		count := func(context.Context, string, any) error { calls++; return nil }
		asc := NewAgentSideConnection(agentFuncs{}, io.Discard, idleReader(t), WithInboundRewrite(count),
			WithInboundValidation(func(context.Context, string, any, error) error { calls++; return nil }))
		_, rerr := asc.handle(t.Context(), AgentMethodSessionNew, json.RawMessage(`{}`))
		if rerr == nil || rerr.Code != -32602 || calls != 0 {
			t.Errorf("handle = %v with %d hook calls, want -32602 and none", rerr, calls)
		}
	})
}

// TestRoots requires each Roots method to return cwd, then each additional
// directory in order, in a slice that does not share the request's
// (0015-PLAN-validate-absolute-protocol-paths.md, step 7).
func TestRoots(t *testing.T) {
	t.Parallel()
	cwd, a, b := testAbsPath("cwd"), testAbsPath("a"), testAbsPath("b")
	// extra leaves room past its length, so a Roots that appended to it would
	// write into the request's own array.
	extra := func() []string {
		s := make([]string, 2, 4)
		s[0], s[1] = a, b
		return s
	}
	for _, tc := range []struct {
		name  string
		roots func(dirs []string) []string
	}{
		{"NewSessionRequest", func(d []string) []string {
			return (&NewSessionRequest{Cwd: cwd, AdditionalDirectories: d}).Roots()
		}},
		{"LoadSessionRequest", func(d []string) []string {
			return (&LoadSessionRequest{Cwd: cwd, AdditionalDirectories: d}).Roots()
		}},
		{"ResumeSessionRequest", func(d []string) []string {
			return (&ResumeSessionRequest{Cwd: cwd, AdditionalDirectories: d}).Roots()
		}},
		{"UnstableForkSessionRequest", func(d []string) []string {
			return (&UnstableForkSessionRequest{Cwd: cwd, AdditionalDirectories: d}).Roots()
		}},
		{"SessionInfo", func(d []string) []string {
			return (&SessionInfo{Cwd: cwd, AdditionalDirectories: d}).Roots()
		}},
	} {
		if got := tc.roots(nil); !slices.Equal(got, []string{cwd}) {
			t.Errorf("%s with no additional directories: Roots() = %q, want %q", tc.name, got, []string{cwd})
		}
		dirs := extra()
		got := tc.roots(dirs)
		if want := []string{cwd, a, b}; !slices.Equal(got, want) {
			t.Errorf("%s: Roots() = %q, want %q", tc.name, got, want)
			continue
		}
		got[1], got[2] = "changed", "changed"
		if full := dirs[:cap(dirs)]; full[0] != a || full[1] != b || full[2] != "" {
			t.Errorf("%s: Roots() shares or changed the request's slice: %q", tc.name, full)
		}
	}
}
