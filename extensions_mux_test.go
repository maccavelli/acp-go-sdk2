package acp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strings"
	"sync"
	"testing"

	acp "github.com/maccavelli/acp-go-sdk2"
	"github.com/maccavelli/acp-go-sdk2/acptest"
)

type echoParams struct {
	Text string `json:"text"`
}

type echoResult struct {
	Echo string `json:"echo"`
}

// muxAgent is a ScriptAgent whose extension methods an ExtensionMux serves.
// It keeps each answer the mux gave, what its handlers ran with, and whether
// IsNotification held in its typed Cancel and NewSession.
type muxAgent struct {
	*acptest.ScriptAgent
	mux acp.ExtensionMux

	mu      sync.Mutex
	answers map[muxCall]muxAnswer
	echoed  []string
	pinged  []string
	kinds   map[string]bool
}

// muxCall is a method and whether it was received as a notification.
type muxCall struct {
	method       string
	notification bool
}

type muxAnswer struct {
	result any
	err    error
}

func newMuxAgent() *muxAgent {
	a := &muxAgent{ScriptAgent: &acptest.ScriptAgent{}, answers: map[muxCall]muxAnswer{}, kinds: map[string]bool{}}
	acp.Handle(&a.mux, "_test/echo", func(_ context.Context, p echoParams) (echoResult, error) {
		a.mu.Lock()
		a.echoed = append(a.echoed, p.Text)
		a.mu.Unlock()
		if p.Text == "refuse" {
			return echoResult{}, &acp.RequestError{Code: -32050, Message: "vendor refused"}
		}
		return echoResult{Echo: p.Text}, nil
	})
	acp.Notify(&a.mux, "_test/ping", func(_ context.Context, p echoParams) error {
		a.mu.Lock()
		a.pinged = append(a.pinged, p.Text)
		a.mu.Unlock()
		return nil
	})
	return a
}

func (a *muxAgent) HandleExtensionMethod(ctx context.Context, method string, params json.RawMessage) (any, error) {
	result, err := a.mux.HandleExtensionMethod(ctx, method, params)
	a.mu.Lock()
	a.answers[muxCall{method, acp.IsNotification(ctx)}] = muxAnswer{result: result, err: err}
	a.mu.Unlock()
	return result, err
}

func (a *muxAgent) Cancel(ctx context.Context, p acp.CancelNotification) error {
	a.mark("session/cancel", acp.IsNotification(ctx))
	return a.ScriptAgent.Cancel(ctx, p)
}

func (a *muxAgent) NewSession(ctx context.Context, p acp.NewSessionRequest) (acp.NewSessionResponse, error) {
	a.mark("session/new", acp.IsNotification(ctx))
	return a.ScriptAgent.NewSession(ctx, p)
}

func (a *muxAgent) mark(method string, notification bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.kinds[method] = notification
}

// state returns copies of what the agent kept.
func (a *muxAgent) state() (answers map[muxCall]muxAnswer, echoed, pinged []string, kinds map[string]bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return maps.Clone(a.answers), slices.Clone(a.echoed), slices.Clone(a.pinged), maps.Clone(a.kinds)
}

// muxLog is a log destination safe for the connection's goroutines.
type muxLog struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (l *muxLog) Write(b []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(b)
}

func (l *muxLog) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.String()
}

// muxPair connects a muxAgent and a MemoryClient, logging at Warn and above
// to the returned log, and initializes them.
func muxPair(t *testing.T) (acptest.Pair, *muxAgent, *muxLog) {
	t.Helper()
	agent := newMuxAgent()
	log := &muxLog{}
	logger := slog.New(slog.NewTextHandler(log, &slog.HandlerOptions{Level: slog.LevelWarn}))
	pair := acptest.Connect(t, agent, &acptest.MemoryClient{}, acp.WithLogger(logger))
	if _, err := pair.Client.Initialize(t.Context(), acp.InitializeRequest{ProtocolVersion: acp.ProtocolVersionNumber}); err != nil {
		t.Fatal(err)
	}
	return pair, agent, log
}

// echo calls _test/echo with text and requires its answer. The answer comes
// after every notification sent before it has been handled.
func echo(t *testing.T, pair acptest.Pair, text string) {
	t.Helper()
	raw, err := pair.Client.CallExtension(t.Context(), "_test/echo", echoParams{Text: text})
	if err != nil {
		t.Fatalf("_test/echo: %v", err)
	}
	if want := fmt.Sprintf(`{"echo":%q}`, text); string(raw) != want {
		t.Fatalf("_test/echo answered %s, want %s", raw, want)
	}
}

// requestCode returns the code of err, a *RequestError, failing the test when
// it is not one.
func requestCode(t *testing.T, err error) int {
	t.Helper()
	re, ok := errors.AsType[*acp.RequestError](err)
	if !ok {
		t.Fatalf("got %v (%T), want a *RequestError", err, err)
	}
	return re.Code
}

// TestExtensionMuxRequests decodes a typed request and answers its result,
// answers a params mismatch -32602 and an unregistered request -32601, and
// passes a handler's *RequestError through.
func TestExtensionMuxRequests(t *testing.T) {
	t.Parallel()
	pair, agent, log := muxPair(t)
	ctx := t.Context()

	echo(t, pair, "hello")

	_, err := pair.Client.CallExtension(ctx, "_test/echo", map[string]any{"text": 5})
	if code := requestCode(t, err); code != -32602 {
		t.Errorf("a params mismatch answered %d, want -32602", code)
	}
	_, err = pair.Client.CallExtension(ctx, "_test/echo", echoParams{Text: "refuse"})
	if code := requestCode(t, err); code != -32050 {
		t.Errorf("a handler's *RequestError answered %d, want -32050", code)
	}
	_, err = pair.Client.CallExtension(ctx, "_test/unknown", map[string]any{})
	if code := requestCode(t, err); code != -32601 {
		t.Errorf("an unregistered request answered %d, want -32601", code)
	}

	_, echoed, _, _ := agent.state()
	if want := []string{"hello", "refuse"}; !slices.Equal(echoed, want) {
		t.Errorf("the echo handler ran with %q, want %q", echoed, want)
	}
	if s := log.String(); s != "" {
		t.Errorf("requests logged %q, want nothing", s)
	}
}

// TestExtensionMuxNotifications runs a Notify handler for a notification and
// not for a request of its name, and ignores an unregistered notification and
// a notification of a Handle name: the mux returns nothing for either, and
// nothing is logged.
func TestExtensionMuxNotifications(t *testing.T) {
	t.Parallel()
	pair, agent, log := muxPair(t)
	ctx := t.Context()

	for _, method := range []string{"_test/ping", "_test/unknown", "_test/echo"} {
		if err := pair.Client.NotifyExtension(ctx, method, echoParams{Text: "note " + method}); err != nil {
			t.Fatal(err)
		}
	}
	echo(t, pair, "sync")
	answers, echoed, _, _ := agent.state()
	for _, method := range []string{"_test/unknown", "_test/echo"} {
		if a, ok := answers[muxCall{method, true}]; !ok || a.result != nil || a.err != nil {
			t.Errorf("the mux answered the notification %s with (%v, %v), received %v, want nothing", method, a.result, a.err, ok)
		}
	}
	if want := []string{"sync"}; !slices.Equal(echoed, want) {
		t.Errorf("the echo handler ran with %q, want only %q: a notification of a Handle name ran it", echoed, want)
	}

	_, err := pair.Client.CallExtension(ctx, "_test/ping", echoParams{Text: "request"})
	if code := requestCode(t, err); code != -32601 {
		t.Errorf("a request of a Notify name answered %d, want -32601", code)
	}
	_, _, pinged, _ := agent.state()
	if want := []string{"note _test/ping"}; !slices.Equal(pinged, want) {
		t.Errorf("the ping handler ran with %q, want %q", pinged, want)
	}
	if s := log.String(); s != "" {
		t.Errorf("notifications logged %q, want nothing", s)
	}
}

// TestIsNotification is true in a typed notification handler's context and
// false in a typed request handler's and in one the connection did not make.
func TestIsNotification(t *testing.T) {
	t.Parallel()
	pair, agent, _ := muxPair(t)
	ctx := t.Context()

	if err := pair.Client.Cancel(ctx, acp.CancelNotification{SessionId: "s"}); err != nil {
		t.Fatal(err)
	}
	if _, err := pair.Client.NewSession(ctx, acp.NewSessionRequest{Cwd: acptest.Abs("/work"), McpServers: []acp.McpServer{}}); err != nil {
		t.Fatal(err)
	}
	_, _, _, kinds := agent.state()
	if want := map[string]bool{"session/cancel": true, "session/new": false}; !maps.Equal(kinds, want) {
		t.Errorf("IsNotification in the typed handlers was %v, want %v", kinds, want)
	}
	if acp.IsNotification(ctx) {
		t.Error("IsNotification is true for a context the connection did not make")
	}
}

// TestExtensionMuxDirect takes a call it did not receive through a
// connection for a request, and the zero mux is ready.
func TestExtensionMuxDirect(t *testing.T) {
	t.Parallel()
	var m acp.ExtensionMux
	if got := m.Methods(); len(got) != 0 {
		t.Errorf("the zero mux's Methods is %q, want none", got)
	}
	_, err := m.HandleExtensionMethod(t.Context(), "_test/unknown", nil)
	if code := requestCode(t, err); code != -32601 {
		t.Errorf("a direct call of an unregistered name answered %d, want -32601", code)
	}
	acp.Handle(&m, "_test/echo", func(_ context.Context, p echoParams) (echoResult, error) { return echoResult{Echo: p.Text}, nil })
	got, err := m.HandleExtensionMethod(t.Context(), "_test/echo", json.RawMessage(`{"text":"direct"}`))
	if err != nil || got != (echoResult{Echo: "direct"}) {
		t.Errorf("a direct call answered (%v, %v), want ({direct}, nil)", got, err)
	}
}

// panicOf returns what f panics with, as a string, or "" when it does not.
func panicOf(f func()) (msg string) {
	defer func() {
		if r := recover(); r != nil {
			msg = fmt.Sprint(r)
		}
	}()
	f()
	return ""
}

// TestExtensionMuxRegistration panics on a name without "_", a name already
// registered and a nil function, and Methods lists the names sorted.
func TestExtensionMuxRegistration(t *testing.T) {
	t.Parallel()
	nop := func(context.Context, echoParams) (echoResult, error) { return echoResult{}, nil }
	note := func(context.Context, echoParams) error { return nil }
	var m acp.ExtensionMux
	acp.Handle(&m, "_b", nop)
	acp.Notify(&m, "_a", note)
	acp.Handle(&m, "_c", nop)
	if got, want := m.Methods(), []string{"_a", "_b", "_c"}; !slices.Equal(got, want) {
		t.Errorf("Methods is %q, want %q", got, want)
	}

	for _, tc := range []struct {
		name     string
		register func()
		want     string
	}{
		{"Handle without _", func() { acp.Handle(&m, "test/echo", nop) }, `must start with '_' (got "test/echo")`},
		{"Notify without _", func() { acp.Notify(&m, "ping", note) }, `must start with '_' (got "ping")`},
		{"empty name", func() { acp.Handle(&m, "", nop) }, "must be non-empty"},
		{"Handle of a Handle name", func() { acp.Handle(&m, "_b", nop) }, `"_b" is already registered`},
		{"Notify of a Handle name", func() { acp.Notify(&m, "_c", note) }, `"_c" is already registered`},
		{"Handle of a Notify name", func() { acp.Handle(&m, "_a", nop) }, `"_a" is already registered`},
		{"nil Handle function", func() { acp.Handle[echoParams, echoResult](&m, "_d", nil) }, `nil handler for extension method "_d"`},
		{"nil Notify function", func() { acp.Notify[echoParams](&m, "_e", nil) }, `nil handler for extension method "_e"`},
	} {
		if got := panicOf(tc.register); !strings.HasPrefix(got, "acp: ") || !strings.Contains(got, tc.want) {
			t.Errorf("%s: panicked with %q, want an acp: panic containing %q", tc.name, got, tc.want)
		}
	}
	if got, want := m.Methods(), []string{"_a", "_b", "_c"}; !slices.Equal(got, want) {
		t.Errorf("after the refused registrations, Methods is %q, want %q", got, want)
	}
}

// TestTraceMeta sets only the three reserved keys, leaving out empty ones.
func TestTraceMeta(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		parent, state, baggage string
		want                   map[string]any
	}{
		{"00-trace-span-01", "vendor=1", "user=a", map[string]any{"traceparent": "00-trace-span-01", "tracestate": "vendor=1", "baggage": "user=a"}},
		{"00-trace-span-01", "", "user=a", map[string]any{"traceparent": "00-trace-span-01", "baggage": "user=a"}},
		{"", "", "", map[string]any{}},
	} {
		got := acp.TraceMeta(tc.parent, tc.state, tc.baggage)
		if got == nil || !maps.Equal(got, tc.want) {
			t.Errorf("TraceMeta(%q, %q, %q) = %v, want %v", tc.parent, tc.state, tc.baggage, got, tc.want)
		}
	}
}

// TestAdvertiseMeta sets the namespace on a copy, leaving its input as it
// was, and its result is a capability object's _meta.
func TestAdvertiseMeta(t *testing.T) {
	t.Parallel()
	features := map[string]any{"workspace": true, "fileNotifications": true}
	in := map[string]any{"other": 1}
	caps := acp.AgentCapabilities{Meta: acp.AdvertiseMeta(in, "zed.dev", features)}
	if want := map[string]any{"other": 1}; !maps.Equal(in, want) {
		t.Errorf("AdvertiseMeta changed its input to %v, want %v", in, want)
	}
	b, err := json.Marshal(caps)
	if err != nil {
		t.Fatal(err)
	}
	if want := `"_meta":{"other":1,"zed.dev":{"fileNotifications":true,"workspace":true}}`; !strings.Contains(string(b), want) {
		t.Errorf("the capabilities encode as %s, want them to hold %s", b, want)
	}

	out := acp.AdvertiseMeta(in, "", features)
	out["added"] = true
	if want := map[string]any{"other": 1}; !maps.Equal(in, want) {
		t.Errorf("with no namespace, AdvertiseMeta returned its input, not a copy: it is now %v", in)
	}
	if got := acp.AdvertiseMeta(nil, "zed.dev", features); len(got) != 1 || got["zed.dev"] == nil {
		t.Errorf("AdvertiseMeta(nil, ...) = %v, want only zed.dev", got)
	}
}
