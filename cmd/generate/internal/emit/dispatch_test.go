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
	files := map[string]string{"agent_gen.go": out["agent_gen.go"], "client_gen.go": out["client_gen.go"], "interfaces": mustRender(t, f),
		"types_gen.go": out["types_gen.go"]}

	want := map[string][]string{
		"agent_gen.go": {
			"reqCtx, cancel := context.WithCancelCause(ctx)",
			"stored := a.storeTurn(string(p.ConversationId), turn)",
			"if !stored {\n\t\t\tcancel(nil)\n\t\t\treturn nil, NewInternalError(map[string]any{",
			"\"a prompt turn is already running for this session\",",
			"\"conversationId\": p.ConversationId,",
			"resp, err := a.agent.Send(reqCtx, p)",
			"a.removeTurn(string(p.ConversationId), turn)",
			"resp = SendResponse{}",
			"resp.Outcome, err = OutcomeHalted, nil",
			"a.cancelTurns(string(p.ConversationId), s)",
			"resp, err := a.agent.Hello(ctx, p)",
			"if resp.Rev == 0 {\n\t\t\tresp.Rev = ProtocolVersionNumber",
			"Requested: int(p.Rev),",
			"rec, cerr := jsonCopy(Negotiation{",
			"a.negotiation.Store(&rec)",
			"func (a *AgentSideConnection) Negotiated() (Negotiation, bool) {",
			"switch a.conn.promptOverlap {\n\tcase PromptOverlapRefuse:\n\t\tif len(a.sessionCancels[key]) > 0 {\n\t\t\treturn false",
			"case PromptOverlapConcurrent:\n\t\ta.sessionCancels[key] = append(a.sessionCancels[key], turn)",
			"if prev.receipt > turn.receipt {\n\t\t\t\tturn.cancel(errSuperseded)",
			"var errSuperseded = fmt.Errorf(\"%w: superseded by a new prompt\", ErrPromptCancelled)",
			"if turn.receipt != 0 && a.lateCancels[key] > turn.receipt {\n\t\tturn.cancel(ErrPromptCancelled)\n\t} else if supersedes {",
			"if s == 0 || t.receipt < s {\n\t\t\tt.cancel(ErrPromptCancelled)",
			"store, ok := a.agent.(AgentStore)",
			"resp, err := store.Open(ctx, p)",
			"func (a *AgentSideConnection) gate(method string, params any) error {\n\treturn gateMessage(a.negotiation.Load(), method, params)",
			"func (a *AgentSideConnection) resultGate(method string, result any) error {\n\treturn gateResult(a.negotiation.Load(), method, result)",
			"if err := a.conn.checkInbound(ctx, method, &p, a.gate); err != nil {",
			"a.conn.checkResult(method, &resp, a.resultGate)",
			"if err := a.gate(ClientMethodAsk, &params); err != nil {\n\t\treturn AskResponse{}, err\n\t}\n\treturn sendRequest[AskResponse](a.conn, ctx, ClientMethodAsk, &params, a.resultGate)\n}",
			"result, rerr := a.conn.checkResult(method, &resp, func(method string, result any) error {\n\t\t\treturn gateResult(&Negotiation{",
		},
		"client_gen.go": {
			"if sent && ctx.Err() != nil {\n\t\t\tif cerr := c.Stop(context.WithoutCancel(ctx), StopNotification{ConversationId: params.ConversationId}); cerr != nil {\n\t\t\t\tc.conn.loggerOrDefault().Debug(\"failed to send the cancel of a prompt turn\", \"method\", AgentMethodStop, \"err\", cerr)",
			"pctx, done := c.pending.track(ctx, string(p.ConversationId))",
			"if params.Rev == 0 {\n\t\tparams.Rev = ProtocolVersionNumber",
			"return HelloResponse{}, &ProtocolVersionError{Requested: int(params.Rev)}",
			"rec, err := jsonCopy(Negotiation{",
			"c.negotiation.Store(&rec)",
			"func (c *ClientSideConnection) Negotiated() (Negotiation, bool) {",
			"resp, err := answerUnlessCancelled(c.conn, pctx, method, func(ctx context.Context) (AskResponse, error) {",
			"}, AskResponse{Verdict: NewVerdictWithdrawn()})",
			"err := c.conn.SendNotification(ctx, AgentMethodStop, &params)\n\tc.pending.cancel(string(params.ConversationId))",
			"func (c *ClientSideConnection) gate(method string, params any) error {",
			"if err := c.gate(AgentMethodSend, &params); err != nil {\n\t\treturn SendResponse{}, err",
			"resp, sent, err := sendRequestSent[SendResponse](c.conn, ctx, AgentMethodSend, &params, c.resultGate)",
			"sendRequest[HelloResponse](c.conn, ctx, AgentMethodHello, &params, typedGate(func(method string, result *HelloResponse) error {",
			"Response: *result,",
			"if err := c.gate(AgentMethodStop, &params); err != nil {\n\t\treturn err",
			"if err := c.conn.checkInbound(ctx, method, &p, c.gate); err != nil {",
		},
		"types_gen.go": {
			"func (n Negotiation) Allows(method string) error {\n\treturn nil\n}",
			"if method == AgentMethodHello {\n\t\treturn nil\n\t}",
			"func gateResult(n *Negotiation, method string, result any) error {",
			"if g, ok := params.(gater); ok {\n\t\treturn g.gates(n, method)",
			"type gater interface {\n\tgates(n *Negotiation, method string) error",
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
			"RequestPermission", "RequestPermissionOutcome", "InitializeRequest", "params.ProtocolVersion", "resp.ProtocolVersion",
			"AgentMethodInitialize"} {
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
			"// AgentSideConnection represents the agent's view of a connection to a client.\n//\n// Its methods are safe for concurrent use. Messages sent from several\n// goroutines are written whole, one at a time, in the order they reach the\n// connection's writer (0024-PLAN-documentation.md, A5).\ntype AgentSideConnection struct {",
			"func NewAgentSideConnection(agent Agent, peerInput io.Writer, peerOutput io.Reader, opts ...ConnectionOption) *AgentSideConnection {\n" +
				"\tif agent == nil {\n\t\tpanic(\"acp: NewAgentSideConnection: nil agent\")\n\t}\n\tif peerInput == nil {\n\t\tpanic(\"acp: NewAgentSideConnection: nil peerInput\")\n\t}\n\tif peerOutput == nil {\n\t\tpanic(\"acp: NewAgentSideConnection: nil peerOutput\")\n\t}\n",
			"asc.conn = buildConnection(asc.handleWithExtensions, peerInput, peerOutput, opts...)\n" +
				"\tif aware, ok := agent.(AgentConnAware); ok {\n\t\taware.SetAgentConnection(asc)\n\t}\n" +
				"\tasc.conn.start()\n\treturn asc\n}\n",
			"func (a *AgentSideConnection) DroppedNotifications() uint64 {",
			"func (a *AgentSideConnection) Done() <-chan struct{} {",
			"func (a *AgentSideConnection) SetLogger(l *slog.Logger) {",
			"// It is for requests the typed methods cannot make: a method the client\n",
			"func (a *AgentSideConnection) Connection() *Connection {",
		},
		"client_gen.go": {
			"// ClientSideConnection provides the client's view of the connection and implements Agent calls.\n//\n// Its methods are safe for concurrent use. Messages sent from several\n// goroutines are written whole, one at a time, in the order they reach the\n// connection's writer (0024-PLAN-documentation.md, A5).\ntype ClientSideConnection struct {",
			"func NewClientSideConnection(client Client, peerInput io.Writer, peerOutput io.Reader, opts ...ConnectionOption) *ClientSideConnection {\n" +
				"\tif client == nil {\n\t\tpanic(\"acp: NewClientSideConnection: nil client\")\n\t}\n\tif peerInput == nil {\n\t\tpanic(\"acp: NewClientSideConnection: nil peerInput\")\n\t}\n\tif peerOutput == nil {\n\t\tpanic(\"acp: NewClientSideConnection: nil peerOutput\")\n\t}\n",
			"csc.conn = buildConnection(csc.handleWithExtensions, peerInput, peerOutput, opts...)\n" +
				"\tif aware, ok := client.(ClientConnAware); ok {\n\t\taware.SetClientConnection(csc)\n\t}\n" +
				"\tcsc.conn.start()\n\treturn csc\n}\n",
			"// It is for requests the typed methods cannot make: a method the agent\n",
			"func (c *ClientSideConnection) Connection() *Connection {",
		},
	}
	turnState := []string{"\tmu             sync.Mutex\n", "\tsessionCancels map[string][]*promptTurn\n",
		"asc.sessionCancels = make(map[string][]*promptTurn)", "type promptTurn struct {", "var promptReturned func()",
		"func (a *AgentSideConnection) storeTurn(key string, turn *promptTurn) bool {",
		"func (a *AgentSideConnection) removeTurn(key string, turn *promptTurn) {",
		"func (a *AgentSideConnection) cancelTurns(key string, s uint64) {",
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
		gated := name == "with prompt turns"
		for file, w := range map[string]string{
			"agent_gen.go":  "func (a *AgentSideConnection) gate(",
			"client_gen.go": "func (c *ClientSideConnection) resultGate(",
			"types_gen.go":  "func gateMessage(",
		} {
			if got := strings.Contains(files[file], w); got != gated {
				t.Errorf("%s: %s holds %q: %v, want %v", name, file, w, got, gated)
			}
		}
		for file, w := range map[string]string{
			"agent_gen.go":  "checkInbound(ctx, method, &p, nil)",
			"client_gen.go": "sendRequest[OpenResponse](c.conn, ctx, AgentMethodOpen, &params, nil)",
		} {
			if got := strings.Contains(files[file], w); got == gated {
				t.Errorf("%s: %s holds %q: %v, want %v", name, file, w, got, !gated)
			}
		}
	}
}

// TestGatesFollowTheModel requires Negotiation.Allows to test each method's
// capability path from its side's capabilities in the record: a bool read as
// it is, each optional step checked for nil, methods of one capability in one
// case; and every typed wrapper but initialize's to call the gate first
// (0017-PLAN-remediate-acp-v1-conformance-violations.md, step 9).
func TestGatesFollowTheModel(t *testing.T) {
	t.Parallel()
	var schema load.Schema
	if err := json.Unmarshal([]byte(`{"$defs":{
		"Rev":{"type":"integer","format":"uint16"},
		"Leaf":{"type":"object"},
		"Nested":{"type":"object","properties":{"leaf":{"anyOf":[{"$ref":"#/$defs/Leaf"},{"type":"null"}]}}},
		"AgentCapabilities":{"type":"object","properties":{"flag":{"type":"boolean"},
			"nested":{"anyOf":[{"$ref":"#/$defs/Nested"},{"type":"null"}]}}},
		"ClientCapabilities":{"type":"object","properties":{"flag2":{"type":"boolean"}}},
		"InitRequest":{"type":"object","x-side":"agent","x-method":"chat/init","required":["rev","mine"],
			"properties":{"rev":{"$ref":"#/$defs/Rev"},"mine":{"$ref":"#/$defs/ClientCapabilities"}}},
		"InitResponse":{"type":"object","x-side":"agent","x-method":"chat/init","required":["rev","theirs"],
			"properties":{"rev":{"$ref":"#/$defs/Rev"},"theirs":{"$ref":"#/$defs/AgentCapabilities"}}},
		"ARequest":{"type":"object","x-side":"agent","x-method":"chat/a"},
		"AResponse":{"type":"object","x-side":"agent","x-method":"chat/a"},
		"BRequest":{"type":"object","x-side":"agent","x-method":"chat/b"},
		"BResponse":{"type":"object","x-side":"agent","x-method":"chat/b"},
		"CRequest":{"type":"object","x-side":"agent","x-method":"chat/c"},
		"CResponse":{"type":"object","x-side":"agent","x-method":"chat/c"},
		"DNotification":{"type":"object","x-side":"client","x-method":"chat/d"}}}`), &schema); err != nil {
		t.Fatal(err)
	}
	m, err := model.Build(model.Input{
		Meta: &load.Meta{AgentMethods: map[string]string{"init": "chat/init", "a": "chat/a", "b": "chat/b", "c": "chat/c"},
			ClientMethods: map[string]string{"d": "chat/d"}},
		Schema: &schema,
		Protocol: &model.Protocol{Methods: []model.MethodRule{
			{Side: "agent", Wire: "chat/init", Initializes: true, VersionKey: "rev", RequestCapabilities: "mine", ResponseCapabilities: "theirs"},
			{Side: "agent", Wire: "chat/a", Capability: "flag"},
			{Side: "agent", Wire: "chat/b", Capability: "nested.leaf"},
			{Side: "agent", Wire: "chat/c", Capability: "flag"},
			{Side: "client", Wire: "chat/d", Capability: "flag2"},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	out := rendered(t, m)
	for file, lines := range map[string][]string{
		"types_gen.go": {
			"case AgentMethodA, AgentMethodC:\n\t\tif !n.Response.Theirs.Flag {\n\t\t\treturn &CapabilityError{\n\t\t\t\tMethod:     method,\n\t\t\t\tCapability: \"flag\",",
			"case AgentMethodB:\n\t\tif n.Response.Theirs.Nested == nil || n.Response.Theirs.Nested.Leaf == nil {",
			"case ClientMethodD:\n\t\tif !n.Request.Mine.Flag2 {",
			"if method == AgentMethodInit {\n\t\treturn nil\n\t}",
		},
		"client_gen.go": {
			"if err := c.gate(AgentMethodA, &params); err != nil {\n\t\treturn AResponse{}, err",
			"if err := c.gate(AgentMethodB, &params); err != nil {",
		},
		"agent_gen.go": {"if err := a.gate(ClientMethodD, &params); err != nil {\n\t\treturn err"},
	} {
		for _, w := range lines {
			if !strings.Contains(out[file], w) {
				t.Errorf("%s lacks %q:\n%s", file, w, out[file])
			}
		}
	}
	if strings.Contains(out["client_gen.go"], "c.gate(AgentMethodInit") {
		t.Error("client_gen.go gates initialize")
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
		// Without the protocol method, its definition is left out: a definition
		// linked to a method no meta file lists is refused
		// (0019-PLAN-generator-hardening.md, G18).
		defs := `{"$defs":{}}`
		if tt.protocol != nil {
			defs = `{"$defs":{"CancelRequestNotification":{"type":"object","x-side":"protocol","x-method":"$/cancel_request"}}}`
		}
		var schema load.Schema
		if err := json.Unmarshal([]byte(defs), &schema); err != nil {
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
