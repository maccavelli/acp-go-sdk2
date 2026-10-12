package emit

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/load"
	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/model"
)

// rendered returns the files Render builds from m, by name.
func rendered(t *testing.T, m *model.Model) map[string]string {
	t.Helper()
	files, err := Render(m)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, f := range files {
		out[f.Name] = string(f.Src)
	}
	return out
}

// renamedProtocol is a protocol whose prompt turn, cancel, session key, and
// optional interface are named unlike ACP's.
func renamedProtocol(t *testing.T) *model.Model {
	t.Helper()
	var schema load.Schema
	if err := json.Unmarshal([]byte(`{"$defs":{
		"ConversationId":{"type":"string"},
		"SendRequest":{"type":"object","x-side":"agent","x-method":"chat/send","required":["conversationId"],
			"properties":{"conversationId":{"allOf":[{"$ref":"#/$defs/ConversationId"}]}}},
		"Outcome":{"type":"string","enum":["finished","halted"]},
		"SendResponse":{"type":"object","x-side":"agent","x-method":"chat/send","required":["outcome"],
			"properties":{"outcome":{"$ref":"#/$defs/Outcome"}}},
		"StopNotification":{"type":"object","x-side":"agent","x-method":"chat/stop","required":["conversationId"],
			"properties":{"conversationId":{"allOf":[{"$ref":"#/$defs/ConversationId"}]}}},
		"OpenRequest":{"type":"object","x-side":"agent","x-method":"chat/open"},
		"OpenResponse":{"type":"object","x-side":"agent","x-method":"chat/open"},
		"Rev":{"type":"integer","format":"uint16"},
		"Caps":{"type":"object","properties":{"x":{"type":"boolean"}}},
		"HelloRequest":{"type":"object","x-side":"agent","x-method":"chat/hello","required":["rev"],
			"properties":{"rev":{"$ref":"#/$defs/Rev"},"mine":{"$ref":"#/$defs/Caps"}}},
		"HelloResponse":{"type":"object","x-side":"agent","x-method":"chat/hello","required":["rev"],
			"properties":{"rev":{"$ref":"#/$defs/Rev"},"theirs":{"$ref":"#/$defs/Caps"}}},
		"AskRequest":{"type":"object","x-side":"client","x-method":"chat/ask","required":["conversationId"],
			"properties":{"conversationId":{"allOf":[{"$ref":"#/$defs/ConversationId"}]}}},
		"AskResponse":{"type":"object","x-side":"client","x-method":"chat/ask","required":["verdict"],
			"properties":{"verdict":{"$ref":"#/$defs/Verdict"}}},
		"Verdict":{"oneOf":[{"type":"object","required":["verdict"],"properties":{"verdict":{"const":"withdrawn","type":"string"}}},
			{"type":"object","required":["verdict","choice"],"properties":{"verdict":{"const":"chosen","type":"string"},"choice":{"type":"string"}}}],
			"discriminator":{"propertyName":"verdict"}}}}`), &schema); err != nil {
		t.Fatal(err)
	}
	m, err := model.Build(model.Input{
		Meta: &load.Meta{AgentMethods: map[string]string{"send": "chat/send", "stop": "chat/stop", "open": "chat/open", "hello": "chat/hello"},
			ClientMethods: map[string]string{"ask": "chat/ask"}},
		Schema: &schema,
		Protocol: &model.Protocol{
			Interfaces: []model.Interface{{Binding: model.BindAgentOptional, Name: "AgentStore", Var: "store", Doc: []string{"AgentStore stores."}}},
			Methods: []model.MethodRule{
				{Side: "agent", Wire: "chat/open", Binding: model.BindAgentOptional},
				{Side: "agent", Wire: "chat/send", PromptTurn: true, SessionKey: "conversationId", StopKey: "outcome", StopCancelled: "halted"},
				{Side: "agent", Wire: "chat/stop", CancelsPromptTurn: true, SessionKey: "conversationId"},
				{Side: "agent", Wire: "chat/hello", Initializes: true, VersionKey: "rev", RequestCapabilities: "mine", ResponseCapabilities: "theirs"},
				{Side: "client", Wire: "chat/ask", CancelledByPromptCancel: true, SessionKey: "conversationId",
					CancelledKey: "verdict", CancelledVariant: "withdrawn"},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// TestDispatchFollowsTheModel requires the dispatch code and the interfaces
// to take every protocol name from the model, none from ACP.
func TestDispatchFollowsTheModel(t *testing.T) {
	t.Parallel()
	m := renamedProtocol(t)
	out := rendered(t, m)
	f := newFile()
	emitRPCInterfaces(f, m)
	files := map[string]string{"agent_gen.go": out["agent_gen.go"], "client_gen.go": out["client_gen.go"], "interfaces": mustRender(t, f)}

	want := map[string][]string{
		"agent_gen.go": {
			"reqCtx, cancel := context.WithCancelCause(ctx)",
			"a.sessionCancels[string(p.ConversationId)] = turn",
			"resp, err := a.agent.Send(reqCtx, p)",
			"if a.sessionCancels[string(p.ConversationId)] == turn {",
			"resp = SendResponse{}",
			"resp.Outcome, err = OutcomeHalted, nil",
			"cn.cancel(ErrPromptCancelled)",
			"resp, err := a.agent.Hello(ctx, p)",
			"if resp.Rev == 0 {\n\t\t\tresp.Rev = ProtocolVersionNumber",
			"Requested: int(p.Rev),",
			"a.negotiation.Store(&Negotiation{",
			"func (c *AgentSideConnection) Negotiated() (Negotiation, bool) {",
			"if prev, ok := a.sessionCancels[string(p.ConversationId)]; ok && prev.receipt > r {",
			"if r != 0 && a.lateCancels[string(p.ConversationId)] > r {",
			"if cn, ok := a.sessionCancels[string(p.ConversationId)]; ok && (s == 0 || cn.receipt < s) {",
			"store, ok := a.agent.(AgentStore)",
			"resp, err := store.Open(ctx, p)",
		},
		"client_gen.go": {
			"_ = c.Stop(context.Background(), StopNotification{ConversationId: params.ConversationId})",
			"pctx, done := c.pending.track(ctx, string(p.ConversationId))",
			"if params.Rev == 0 {\n\t\tparams.Rev = ProtocolVersionNumber",
			"return HelloResponse{}, &ProtocolVersionError{Requested: int(params.Rev)}",
			"c.negotiation.Store(&Negotiation{",
			"func (c *ClientSideConnection) Negotiated() (Negotiation, bool) {",
			"resp, err := answerUnlessCancelled(c.conn, pctx, method, func(ctx context.Context) (AskResponse, error) {",
			"}, AskResponse{Verdict: NewVerdictWithdrawn()})",
			"err := c.conn.SendNotification(ctx, AgentMethodStop, params)\n\tc.pending.cancel(string(params.ConversationId))",
		},
		"interfaces": {"// AgentStore stores.\ntype AgentStore interface {", "Open(ctx context.Context, params OpenRequest) (OpenResponse, error)"},
	}
	for name, lines := range want {
		for _, w := range lines {
			if !strings.Contains(files[name], w) {
				t.Errorf("%s lacks %q:\n%s", name, w, files[name])
			}
		}
	}
	for name, src := range files {
		for _, bad := range []string{"session/prompt", "SessionId", "CancelNotification", "AgentLoader", "loader", "c.Cancel(", "StopReason",
			"RequestPermission", "RequestPermissionOutcome", "InitializeRequest", "params.ProtocolVersion", "resp.ProtocolVersion"} {
			if strings.Contains(src, bad) {
				t.Errorf("%s contains %q", name, bad)
			}
		}
	}
}

// negotiationField and pendingField match the two fields as gofmt aligns them.
var (
	negotiationField = regexp.MustCompile(`\tnegotiation +atomic\.Pointer\[Negotiation\]\n`)
	pendingField     = regexp.MustCompile(`\tpending +pendingRequests\n`)
)

// TestSideConnectionsFollowTheModel requires each side connection to be
// generated with its constructor, forwarding methods and doc comments, and the
// agent's prompt-turn state only when a prompt-turn rule applies
// (0017-PLAN-remediate-acp-v1-conformance-violations.md, step 4).
func TestSideConnectionsFollowTheModel(t *testing.T) {
	t.Parallel()
	withTurns := rendered(t, renamedProtocol(t))
	var schema load.Schema
	if err := json.Unmarshal([]byte(`{"$defs":{
		"OpenRequest":{"type":"object","x-side":"agent","x-method":"chat/open"},
		"OpenResponse":{"type":"object","x-side":"agent","x-method":"chat/open"}}}`), &schema); err != nil {
		t.Fatal(err)
	}
	m, err := model.Build(model.Input{Meta: &load.Meta{AgentMethods: map[string]string{"open": "chat/open"}}, Schema: &schema,
		Protocol: &model.Protocol{}})
	if err != nil {
		t.Fatal(err)
	}
	without := rendered(t, m)

	common := map[string][]string{
		"agent_gen.go": {
			"// AgentSideConnection represents the agent's view of a connection to a client.\ntype AgentSideConnection struct {",
			"func NewAgentSideConnection(agent Agent, peerInput io.Writer, peerOutput io.Reader, opts ...ConnectionOption) *AgentSideConnection {",
			"asc.conn = NewConnection(asc.handleWithExtensions, peerInput, peerOutput, opts...)\n\treturn asc\n}\n",
			"func (c *AgentSideConnection) DroppedNotifications() uint64 {",
			"func (c *AgentSideConnection) Done() <-chan struct{} {",
			"func (c *AgentSideConnection) SetLogger(l *slog.Logger) {",
			"// It is for requests the typed methods cannot make: a method the client\n",
			"func (c *AgentSideConnection) Connection() *Connection {",
		},
		"client_gen.go": {
			"// ClientSideConnection provides the client's view of the connection and implements Agent calls.\ntype ClientSideConnection struct {",
			"func NewClientSideConnection(client Client, peerInput io.Writer, peerOutput io.Reader, opts ...ConnectionOption) *ClientSideConnection {",
			"// It is for requests the typed methods cannot make: a method the agent\n",
			"func (c *ClientSideConnection) Connection() *Connection {",
		},
	}
	turnState := []string{"\tmu             sync.Mutex\n", "\tsessionCancels map[string]*promptTurn\n",
		"asc.sessionCancels = make(map[string]*promptTurn)", "type promptTurn struct {", "var promptReturned func()",
		"\tlateCancels    map[string]uint64\n", "asc.lateCancels = make(map[string]uint64)"}
	for name, files := range map[string]map[string]string{"with prompt turns": withTurns, "without": without} {
		for file, lines := range common {
			for _, w := range lines {
				if !strings.Contains(files[file], w) {
					t.Errorf("%s: %s lacks %q", name, file, w)
				}
			}
		}
		for _, w := range turnState {
			if got, want := strings.Contains(files["agent_gen.go"], w), name == "with prompt turns"; got != want {
				t.Errorf("%s: agent_gen.go holds %q: %v, want %v", name, w, got, want)
			}
		}
		if strings.Contains(files["client_gen.go"], "sessionCancels") {
			t.Errorf("%s: client_gen.go holds prompt-turn state", name)
		}
		for _, file := range []string{"agent_gen.go", "client_gen.go"} {
			if got, want := negotiationField.MatchString(files[file]), name == "with prompt turns"; got != want {
				t.Errorf("%s: %s holds the negotiation: %v, want %v", name, file, got, want)
			}
		}
		if got, want := pendingField.MatchString(files["client_gen.go"]), name == "with prompt turns"; got != want {
			t.Errorf("%s: client_gen.go holds the pending requests: %v, want %v", name, got, want)
		}
	}
}

// TestProtocolMethodConstants requires a constant for each protocol method of
// the meta files, and no block when there is none.
func TestProtocolMethodConstants(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name     string
		protocol map[string]string
		want     string
		present  bool
	}{
		{"one", map[string]string{"cancel_request": "$/cancel_request"}, "// Protocol method names\nconst (\n\tProtocolMethodCancelRequest = \"$/cancel_request\"\n)\n", true},
		{"none", nil, "Protocol method names", false},
	} {
		var schema load.Schema
		if err := json.Unmarshal([]byte(`{"$defs":{"CancelRequestNotification":{"type":"object","x-side":"protocol","x-method":"$/cancel_request"}}}`), &schema); err != nil {
			t.Fatal(err)
		}
		m, err := model.Build(model.Input{Meta: &load.Meta{Version: 1, ProtocolMethods: tt.protocol}, Schema: &schema})
		if err != nil {
			t.Fatal(err)
		}
		src := rendered(t, m)["constants_gen.go"]
		if got := strings.Contains(src, tt.want); got != tt.present {
			t.Errorf("%s: contains %q = %v, want %v:\n%s", tt.name, tt.want, got, tt.present, src)
		}
	}
}
