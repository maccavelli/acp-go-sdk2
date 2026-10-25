package acp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"slices"
	"sync/atomic"
	"testing"
	"time"
)

// listingPair pairs a client with an agent that answers session/list from
// pages, by the request's cursor ("" for none), and counts the calls.
func listingPair(t *testing.T, pages map[string]ListSessionsResponse, fail map[string]error) (*ClientSideConnection, *atomic.Int32) {
	t.Helper()
	calls := new(atomic.Int32)
	c2aR, c2aW := io.Pipe()
	a2cR, a2cW := io.Pipe()
	agent := agentFuncs{ListSessionsFunc: func(_ context.Context, req ListSessionsRequest) (ListSessionsResponse, error) {
		calls.Add(1)
		key := ""
		if req.Cursor != nil {
			key = *req.Cursor
		}
		if err := fail[key]; err != nil {
			return ListSessionsResponse{}, err
		}
		return pages[key], nil
	}}
	asc := NewAgentSideConnection(agent, a2cW, c2aR)
	csc := NewClientSideConnection(&clientFuncs{}, c2aW, a2cR)
	t.Cleanup(func() { _ = csc.Close(); _ = asc.Close() })
	negotiate(t, asc, allAdvertised())
	negotiate(t, csc, allAdvertised())
	return csc, calls
}

func sessionsPage(next string, ids ...SessionId) ListSessionsResponse {
	r := ListSessionsResponse{Sessions: []SessionInfo{}}
	for _, id := range ids {
		r.Sessions = append(r.Sessions, SessionInfo{SessionId: id, Cwd: testAbsPath("w")})
	}
	if next != "" {
		r.NextCursor = &next
	}
	return r
}

// collect runs seq to its end, returning the session ids and the errors it
// yielded.
func collect(seq func(func(SessionInfo, error) bool)) ([]SessionId, []error) {
	var ids []SessionId
	var errs []error
	for s, err := range seq {
		if err != nil {
			errs = append(errs, err)
			continue
		}
		ids = append(ids, s.SessionId)
	}
	return ids, errs
}

// TestAllSessions requires AllSessions to follow the cursor to the end, stop
// on a repeated cursor or an error, and send nothing after a break
// (0038-PLAN-consumer-helpers-kind-and-agent-refusals.md, step 2).
func TestAllSessions(t *testing.T) {
	t.Parallel()
	t.Run("two pages", testAllSessionsTwoPages)
	t.Run("repeating cursor", testAllSessionsRepeat)
	t.Run("error on second page", testAllSessionsError)
	t.Run("break", testAllSessionsBreak)
	t.Run("empty next cursor", testAllSessionsEmptyCursor)
	t.Run("not advertised", testAllSessionsNotAdvertised)
	t.Run("nil connection", testAllSessionsNil)
}

func testAllSessionsNil(t *testing.T) {
	t.Parallel()
	ids, errs := collect(AllSessions(listCtx(t), nil, ListSessionsRequest{}))
	if len(ids) != 0 || len(errs) != 1 || !errors.Is(errs[0], errNilConnection) {
		t.Errorf("ids %v, errors %v; want none and one errNilConnection", ids, errs)
	}
}

func listCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func testAllSessionsTwoPages(t *testing.T) {
	t.Parallel()
	c, calls := listingPair(t, map[string]ListSessionsResponse{"": sessionsPage("p2", "a", "b"), "p2": sessionsPage("", "c")}, nil)
	ids, errs := collect(AllSessions(listCtx(t), c, ListSessionsRequest{}))
	if !slices.Equal(ids, []SessionId{"a", "b", "c"}) || len(errs) != 0 || calls.Load() != 2 {
		t.Errorf("ids %v, errors %v, %d calls; want [a b c], none, 2", ids, errs, calls.Load())
	}
}

func testAllSessionsRepeat(t *testing.T) {
	t.Parallel()
	c, calls := listingPair(t, map[string]ListSessionsResponse{"": sessionsPage("p2", "a"), "p2": sessionsPage("p2", "b")}, nil)
	ids, errs := collect(AllSessions(listCtx(t), c, ListSessionsRequest{}))
	if !slices.Equal(ids, []SessionId{"a", "b"}) || len(errs) != 1 || !errors.Is(errs[0], ErrRepeatedCursor) || calls.Load() != 2 {
		t.Errorf("ids %v, errors %v, %d calls; want [a b], ErrRepeatedCursor, 2", ids, errs, calls.Load())
	}
}

func testAllSessionsError(t *testing.T) {
	t.Parallel()
	c, calls := listingPair(t, map[string]ListSessionsResponse{"": sessionsPage("p2", "a")},
		map[string]error{"p2": NewInternalError(map[string]any{"error": "boom"})})
	ids, errs := collect(AllSessions(listCtx(t), c, ListSessionsRequest{}))
	var re *RequestError
	if !slices.Equal(ids, []SessionId{"a"}) || len(errs) != 1 || !errors.As(errs[0], &re) || re.Code != -32603 || calls.Load() != 2 {
		t.Errorf("ids %v, errors %v, %d calls; want [a], one -32603, 2", ids, errs, calls.Load())
	}
}

func testAllSessionsBreak(t *testing.T) {
	t.Parallel()
	c, calls := listingPair(t, map[string]ListSessionsResponse{"": sessionsPage("p2", "a", "b"), "p2": sessionsPage("", "c")}, nil)
	for s, err := range AllSessions(listCtx(t), c, ListSessionsRequest{}) {
		if err != nil || s.SessionId != "a" {
			t.Errorf("first yield %v, %v; want a, nil", s.SessionId, err)
		}
		break
	}
	if calls.Load() != 1 {
		t.Errorf("%d calls after a break on the first session, want 1", calls.Load())
	}
}

func testAllSessionsEmptyCursor(t *testing.T) {
	t.Parallel()
	page := sessionsPage("", "a")
	page.NextCursor = new(string)
	c, calls := listingPair(t, map[string]ListSessionsResponse{"": page}, nil)
	ids, errs := collect(AllSessions(listCtx(t), c, ListSessionsRequest{}))
	if !slices.Equal(ids, []SessionId{"a"}) || len(errs) != 0 || calls.Load() != 1 {
		t.Errorf("ids %v, errors %v, %d calls; want [a], none, 1", ids, errs, calls.Load())
	}
}

func testAllSessionsNotAdvertised(t *testing.T) {
	t.Parallel()
	w := &countingWriter{}
	_, csc := sides(t, w, versionOnly())
	ids, errs := collect(AllSessions(listCtx(t), csc, ListSessionsRequest{}))
	ce, ok := errors.AsType[*CapabilityError](errOf(errs))
	if len(ids) != 0 || len(errs) != 1 || !ok || ce.Capability != "sessionCapabilities.list" || w.count() != 0 {
		t.Errorf("ids %v, errors %v, %d bytes; want none, one *CapabilityError on sessionCapabilities.list, 0", ids, errs, w.count())
	}
}

func errOf(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	return errs[0]
}

// TestSessionInfoUpdated requires Updated to read RFC 3339, fractional
// seconds and offsets included, and to refuse anything else.
func TestSessionInfoUpdated(t *testing.T) {
	t.Parallel()
	want := time.Date(2026, 10, 25, 13, 4, 5, 0, time.UTC)
	for _, c := range []struct {
		in   *string
		want time.Time
		ok   bool
	}{
		{new("2026-10-25T13:04:05Z"), want, true},
		{new("2026-10-25T15:04:05.250+02:00"), want.Add(250 * time.Millisecond), true},
		{nil, time.Time{}, false},
		{new("yesterday"), time.Time{}, false},
	} {
		got, ok := SessionInfo{UpdatedAt: c.in}.Updated()
		if ok != c.ok || !got.Equal(c.want) {
			t.Errorf("Updated(%v) = %v, %v; want %v, %v", c.in, got, ok, c.want, c.ok)
		}
	}
}

// TestPickPermissionOption requires the choice to follow the kinds asked,
// in order, and never the option's name.
func TestPickPermissionOption(t *testing.T) {
	t.Parallel()
	dont := PermissionOption{OptionId: "no", Name: "Don't allow", Kind: PermissionOptionKindRejectOnce}
	always := PermissionOption{OptionId: "always", Name: "Always", Kind: PermissionOptionKindAllowAlways}
	once := PermissionOption{OptionId: "once", Name: "Once", Kind: PermissionOptionKindAllowOnce}
	named := PermissionOption{OptionId: "trap", Name: "Allow", Kind: PermissionOptionKindRejectOnce}
	allow := []PermissionOptionKind{PermissionOptionKindAllowOnce, PermissionOptionKindAllowAlways}
	for _, c := range []struct {
		name  string
		opts  []PermissionOption
		kinds []PermissionOptionKind
		want  PermissionOptionId
		ok    bool
	}{
		{"don't allow is not an allow", []PermissionOption{dont, always}, allow, "always", true},
		{"kinds in order", []PermissionOption{always, once}, allow, "once", true},
		{"no match", []PermissionOption{dont}, allow, "", false},
		{"no kinds", []PermissionOption{once}, nil, "", false},
		{"name never read", []PermissionOption{named}, allow, "", false},
	} {
		got, ok := PickPermissionOption(c.opts, c.kinds...)
		if ok != c.ok || got.OptionId != c.want {
			t.Errorf("%s: %q, %v; want %q, %v", c.name, got.OptionId, ok, c.want, c.ok)
		}
	}
}

// TestChosen requires the selected option, and false otherwise.
func TestChosen(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		r    RequestPermissionResponse
		want PermissionOptionId
		ok   bool
	}{
		{"selected", RequestPermissionResponse{Outcome: NewRequestPermissionOutcomeSelected("o")}, "o", true},
		{"cancelled", RequestPermissionResponse{Outcome: NewRequestPermissionOutcomeCancelled()}, "", false},
		{"unknown", RequestPermissionResponse{Outcome: RequestPermissionOutcome{Unknown: []byte(`{"outcome":"later"}`)}}, "", false},
		{"zero", RequestPermissionResponse{}, "", false},
	} {
		if got, ok := c.r.Chosen(); ok != c.ok || got != c.want {
			t.Errorf("%s: %q, %v; want %q, %v", c.name, got, ok, c.want, c.ok)
		}
	}
}

// TestMetaObject requires a namespaced object, and nil for anything else.
func TestMetaObject(t *testing.T) {
	t.Parallel()
	meta := map[string]any{"vendor.dev": map[string]any{"x": 1.0}, "s": "text"}
	if got := MetaObject(meta, "vendor.dev"); got["x"] != 1.0 {
		t.Errorf("object: %v", got)
	}
	for _, key := range []string{"s", "absent"} {
		if got := MetaObject(meta, key); got != nil {
			t.Errorf("%s: %v, want nil", key, got)
		}
	}
	if got := MetaObject(nil, "vendor.dev"); got != nil {
		t.Errorf("nil map: %v, want nil", got)
	}
}

// TestDecodeMeta requires a typed decode, an error on a mismatch, and false
// with no error when the key is absent.
func TestDecodeMeta(t *testing.T) {
	t.Parallel()
	type feature struct {
		Enabled bool `json:"enabled"`
		Level   int  `json:"level"`
	}
	meta := map[string]any{"vendor.dev": map[string]any{"enabled": true, "level": 2.0}, "bad": "text"}
	if got, ok, err := DecodeMeta[feature](meta, "vendor.dev"); !ok || err != nil || got != (feature{true, 2}) {
		t.Errorf("struct: %+v, %v, %v", got, ok, err)
	}
	if _, ok, err := DecodeMeta[feature](meta, "bad"); !ok || err == nil {
		t.Errorf("mismatch: %v, %v; want true and an error", ok, err)
	}
	if _, ok, err := DecodeMeta[feature](meta, "absent"); ok || err != nil {
		t.Errorf("absent: %v, %v; want false, nil", ok, err)
	}
}

// TestConfigOptionByCategory requires the first select option of the
// category, past a boolean and a select without one.
func TestConfigOptionByCategory(t *testing.T) {
	t.Parallel()
	model := SessionConfigOptionCategoryModel
	uncategorised := NewSessionConfigOptionSelect("plain", "Plain", "a", NewSessionConfigSelectOptionsUngrouped(nil))
	byModel := NewSessionConfigOptionSelect("model", "Model", "a", NewSessionConfigSelectOptionsUngrouped(nil))
	byModel.Select.Category = &model
	opts := []SessionConfigOption{NewSessionConfigOptionBoolean("b", "B", true), uncategorised, byModel}
	if got, ok := ConfigOptionByCategory(opts, model); !ok || got.Id != "model" {
		t.Errorf("model: %+v, %v", got, ok)
	}
	if got, ok := ConfigOptionByCategory(opts, SessionConfigOptionCategoryThoughtLevel); ok || got != nil {
		t.Errorf("thought level: %+v, %v; want nil, false", got, ok)
	}
}

// TestSelectOptionsAll requires ungrouped values as they are, grouped ones
// in order, and nil for neither.
func TestSelectOptionsAll(t *testing.T) {
	t.Parallel()
	opt := func(v SessionConfigValueId) SessionConfigSelectOption {
		return SessionConfigSelectOption{Value: v, Name: string(v)}
	}
	values := func(o []SessionConfigSelectOption) []SessionConfigValueId {
		var out []SessionConfigValueId
		for _, x := range o {
			out = append(out, x.Value)
		}
		return out
	}
	ungrouped := NewSessionConfigSelectOptionsUngrouped([]SessionConfigSelectOption{opt("a"), opt("b")})
	grouped := NewSessionConfigSelectOptionsGrouped([]SessionConfigSelectGroup{
		{Group: "g1", Name: "G1", Options: []SessionConfigSelectOption{opt("a"), opt("b")}},
		{Group: "g2", Name: "G2", Options: []SessionConfigSelectOption{opt("c")}},
	})
	if got := values(ungrouped.All()); !slices.Equal(got, []SessionConfigValueId{"a", "b"}) {
		t.Errorf("ungrouped: %v", got)
	}
	if got := values(grouped.All()); !slices.Equal(got, []SessionConfigValueId{"a", "b", "c"}) {
		t.Errorf("grouped: %v", got)
	}
	if got := (SessionConfigSelectOptions{}).All(); got != nil {
		t.Errorf("neither: %v, want nil", got)
	}
}

// fakeLookPath resolves "npx" to an absolute path, "rel" to a relative one,
// and fails for anything else, counting its calls.
func fakeLookPath(calls *atomic.Int32) func(string) (string, error) {
	return func(name string) (string, error) {
		calls.Add(1)
		switch name {
		case "npx":
			return testAbsPath("bin", "npx"), nil
		case "rel":
			return "bin/rel", nil
		}
		return "", errors.New("not found")
	}
}

// TestResolveStdioCommands requires each of the four requests' stdio
// commands to be resolved when bare, and left when absolute, unresolved or
// not stdio (0038-PLAN-consumer-helpers-kind-and-agent-refusals.md, step 4).
func TestResolveStdioCommands(t *testing.T) {
	t.Parallel()
	npx := testAbsPath("bin", "npx")
	abs := testAbsPath("opt", "tool")
	stable := func() []McpServer {
		return []McpServer{NewMcpServerStdio("a", "npx", nil, nil), NewMcpServerStdio("b", abs, nil, nil),
			NewMcpServerStdio("c", "rel", nil, nil), NewMcpServerStdio("d", "missing", nil, nil),
			NewMcpServerHttp("e", "https://example.invalid", nil)}
	}
	newReq := &NewSessionRequest{Cwd: testAbsPath("w"), McpServers: stable()}
	loadReq := &LoadSessionRequest{Cwd: testAbsPath("w"), SessionId: "s", McpServers: stable()}
	resumeReq := &ResumeSessionRequest{Cwd: testAbsPath("w"), SessionId: "s", McpServers: stable()}
	forkReq := &UnstableForkSessionRequest{Cwd: testAbsPath("w"), SessionId: "s", McpServers: []UnstableMcpServer{
		NewUnstableMcpServerStdio("a", "npx", nil, nil), NewUnstableMcpServerStdio("b", abs, nil, nil)}}
	calls := new(atomic.Int32)
	hook := ResolveStdioCommands(fakeLookPath(calls))
	for method, params := range map[string]any{AgentMethodSessionNew: newReq, AgentMethodSessionLoad: loadReq,
		AgentMethodSessionResume: resumeReq, "session/fork": forkReq} {
		if err := hook(t.Context(), method, params); err != nil {
			t.Errorf("%s: hook returned %v", method, err)
		}
	}
	want := []string{npx, abs, "rel", "missing"}
	for name, servers := range map[string][]McpServer{"new": newReq.McpServers, "load": loadReq.McpServers, "resume": resumeReq.McpServers} {
		for i, w := range want {
			if got := servers[i].Stdio.Command; got != w {
				t.Errorf("%s server %d: command %q, want %q", name, i, got, w)
			}
		}
		if servers[4].Http == nil || servers[4].Http.Url != "https://example.invalid" {
			t.Errorf("%s: the http server changed: %+v", name, servers[4])
		}
	}
	if got := forkReq.McpServers[0].Stdio.Command; got != npx {
		t.Errorf("fork: command %q, want %q", got, npx)
	}
	// Three bare commands in each of three requests, one in fork; never an absolute one.
	if got := calls.Load(); got != 10 {
		t.Errorf("lookPath called %d times, want 10, never for an absolute command", got)
	}
	other := &PromptRequest{SessionId: "s"}
	if err := hook(t.Context(), AgentMethodSessionPrompt, other); err != nil || calls.Load() != 10 {
		t.Errorf("another method's params: %v, %d calls", err, calls.Load())
	}
}

// TestResolveStdioCommandsOverTheWire requires an agent with the hook to see
// a client's bare command resolved, and to refuse one it cannot resolve.
func TestResolveStdioCommandsOverTheWire(t *testing.T) {
	t.Parallel()
	seen := make(chan string, 1)
	c2aR, c2aW := io.Pipe()
	a2cR, a2cW := io.Pipe()
	agent := agentFuncs{NewSessionFunc: func(_ context.Context, req NewSessionRequest) (NewSessionResponse, error) {
		seen <- req.McpServers[0].Stdio.Command
		return NewSessionResponse{SessionId: "s"}, nil
	}}
	asc := NewAgentSideConnection(agent, a2cW, c2aR, WithInboundRewrite(ResolveStdioCommands(fakeLookPath(new(atomic.Int32)))))
	csc := NewClientSideConnection(&clientFuncs{}, c2aW, a2cR)
	t.Cleanup(func() { _ = csc.Close(); _ = asc.Close() })
	negotiate(t, asc, allAdvertised())
	negotiate(t, csc, allAdvertised())
	ctx := listCtx(t)
	// A client in another language sends "npx" as it is; this SDK's own client
	// would refuse to, so the request goes out untyped.
	send := func(command string) error {
		_, err := SendRequest[json.RawMessage](csc.Connection(), ctx, AgentMethodSessionNew, map[string]any{
			"cwd": testAbsPath("w"), "mcpServers": []any{map[string]any{"name": "s", "command": command, "args": []any{}, "env": []any{}}},
		})
		return err
	}
	if err := send("npx"); err != nil {
		t.Fatalf("session/new with npx: %v", err)
	}
	if got := <-seen; got != testAbsPath("bin", "npx") {
		t.Errorf("the handler saw %q, want the resolved path", got)
	}
	re, ok := errors.AsType[*RequestError](send("missing"))
	if !ok || re.Code != -32602 {
		t.Errorf("session/new with an unresolvable command: %v, want -32602", re)
	}
}

// TestChainRewrites requires the hooks in order, the first error stopping
// the rest, and no hooks to do nothing.
func TestChainRewrites(t *testing.T) {
	t.Parallel()
	var order []int
	hook := func(n int, err error) func(context.Context, string, any) error {
		return func(context.Context, string, any) error {
			order = append(order, n)
			return err
		}
	}
	stop := errors.New("stop")
	if err := ChainRewrites(hook(1, nil), hook(2, nil))(t.Context(), "m", nil); err != nil || !slices.Equal(order, []int{1, 2}) {
		t.Errorf("in order: %v, %v; want nil, [1 2]", err, order)
	}
	order = nil
	if err := ChainRewrites(hook(1, stop), hook(2, nil))(t.Context(), "m", nil); !errors.Is(err, stop) || !slices.Equal(order, []int{1}) {
		t.Errorf("first error: %v, %v; want stop, [1]", err, order)
	}
	if err := ChainRewrites()(t.Context(), "m", nil); err != nil {
		t.Errorf("no hooks: %v", err)
	}
}

// TestOfferedAuthMethods requires terminal methods dropped without
// auth.terminal and kept with it, the others kept in order, the input
// unchanged, and the filtered list accepted by a client the unfiltered one
// is refused by.
func TestOfferedAuthMethods(t *testing.T) {
	t.Parallel()
	all := []AuthMethod{NewAuthMethodTerminal("term", "Terminal"), NewAuthMethodAgent("agent", "Agent"),
		{Unknown: json.RawMessage(`{"type":"future","id":"f","name":"F"}`)}}
	ids := func(ms []AuthMethod) []string {
		var out []string
		for _, m := range ms {
			switch {
			case m.Terminal != nil:
				out = append(out, "terminal")
			case m.Agent != nil:
				out = append(out, "agent")
			default:
				out = append(out, "unknown")
			}
		}
		return out
	}
	if got := ids(OfferedAuthMethods(ClientCapabilities{}, all)); !slices.Equal(got, []string{"agent", "unknown"}) {
		t.Errorf("without auth.terminal: %v", got)
	}
	if got := ids(OfferedAuthMethods(ClientCapabilities{Auth: AuthCapabilities{Terminal: true}}, all)); !slices.Equal(got, []string{"terminal", "agent", "unknown"}) {
		t.Errorf("with auth.terminal: %v", got)
	}
	if got := ids(all); !slices.Equal(got, []string{"terminal", "agent", "unknown"}) {
		t.Errorf("the input changed: %v", got)
	}
	known := all[:2]
	if _, err := initializeOffering(t, known, false); err == nil {
		t.Error("the unfiltered list was accepted by a client without auth.terminal; the test proves nothing")
	}
	resp, err := initializeOffering(t, known, true)
	if err != nil || !slices.Equal(ids(resp.AuthMethods), []string{"agent"}) {
		t.Errorf("filtered: %v, %v; want [agent], nil", ids(resp.AuthMethods), err)
	}
}

// initializeOffering runs initialize from a client without auth.terminal
// against an agent offering methods, through OfferedAuthMethods when filter
// is set.
func initializeOffering(t *testing.T, methods []AuthMethod, filter bool) (InitializeResponse, error) {
	t.Helper()
	c2aR, c2aW := io.Pipe()
	a2cR, a2cW := io.Pipe()
	agent := agentFuncs{InitializeFunc: func(_ context.Context, req InitializeRequest) (InitializeResponse, error) {
		offered := methods
		if filter {
			offered = OfferedAuthMethods(req.ClientCapabilities, methods)
		}
		return InitializeResponse{ProtocolVersion: ProtocolVersionNumber, AuthMethods: offered}, nil
	}}
	asc := NewAgentSideConnection(agent, a2cW, c2aR)
	csc := NewClientSideConnection(&clientFuncs{}, c2aW, a2cR)
	t.Cleanup(func() { _ = csc.Close(); _ = asc.Close() })
	return csc.Initialize(listCtx(t), InitializeRequest{ProtocolVersion: ProtocolVersionNumber})
}
