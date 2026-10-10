package emit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/load"
	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/model"
)

// renamedProtocol is a protocol whose prompt turn, cancel, session key, and
// optional interface are named unlike ACP's.
func renamedProtocol(t *testing.T) *model.Model {
	t.Helper()
	var schema load.Schema
	if err := json.Unmarshal([]byte(`{"$defs":{
		"ConversationId":{"type":"string"},
		"SendRequest":{"type":"object","x-side":"agent","x-method":"chat/send","required":["conversationId"],
			"properties":{"conversationId":{"allOf":[{"$ref":"#/$defs/ConversationId"}]}}},
		"SendResponse":{"type":"object","x-side":"agent","x-method":"chat/send"},
		"StopNotification":{"type":"object","x-side":"agent","x-method":"chat/stop","required":["conversationId"],
			"properties":{"conversationId":{"allOf":[{"$ref":"#/$defs/ConversationId"}]}}},
		"OpenRequest":{"type":"object","x-side":"agent","x-method":"chat/open"},
		"OpenResponse":{"type":"object","x-side":"agent","x-method":"chat/open"}}}`), &schema); err != nil {
		t.Fatal(err)
	}
	m, err := model.Build(model.Input{
		Meta:   &load.Meta{AgentMethods: map[string]string{"send": "chat/send", "stop": "chat/stop", "open": "chat/open"}},
		Schema: &schema,
		Protocol: &model.Protocol{
			Interfaces: []model.Interface{{Binding: model.BindAgentOptional, Name: "AgentStore", Var: "store", Doc: []string{"AgentStore stores."}}},
			Methods: []model.MethodRule{
				{Side: "agent", Wire: "chat/open", Binding: model.BindAgentOptional},
				{Side: "agent", Wire: "chat/send", PromptTurn: true, SessionKey: "conversationId"},
				{Side: "agent", Wire: "chat/stop", CancelsPromptTurn: true, SessionKey: "conversationId"},
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
	dir := t.TempDir()
	if err := WriteDispatch(dir, m); err != nil {
		t.Fatal(err)
	}
	read := func(name string) string {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	f := newFile()
	emitRPCInterfaces(f, m)
	files := map[string]string{"agent_gen.go": read("agent_gen.go"), "client_gen.go": read("client_gen.go"), "interfaces": render(t, f)}

	want := map[string][]string{
		"agent_gen.go": {
			"reqCtx, cancel = context.WithCancel(ctx)",
			"a.sessionCancels[string(p.ConversationId)] = cancel",
			"resp, err := a.agent.Send(reqCtx, p)",
			"if cn, ok := a.sessionCancels[string(p.ConversationId)]; ok {",
			"store, ok := a.agent.(AgentStore)",
			"resp, err := store.Open(ctx, p)",
		},
		"client_gen.go": {"_ = c.Stop(context.Background(), StopNotification{ConversationId: params.ConversationId})"},
		"interfaces":    {"// AgentStore stores.\ntype AgentStore interface {", "Open(ctx context.Context, params OpenRequest) (OpenResponse, error)"},
	}
	for name, lines := range want {
		for _, w := range lines {
			if !strings.Contains(files[name], w) {
				t.Errorf("%s lacks %q:\n%s", name, w, files[name])
			}
		}
	}
	for name, src := range files {
		for _, bad := range []string{"session/prompt", "SessionId", "CancelNotification", "AgentLoader", "loader", "c.Cancel("} {
			if strings.Contains(src, bad) {
				t.Errorf("%s contains %q", name, bad)
			}
		}
	}
}
