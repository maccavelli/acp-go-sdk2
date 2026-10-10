package model

import (
	"strings"
	"testing"

	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/load"
)

const methodDefs = `{
	"NewSessionRequest":{"type":"object","x-side":"agent","x-method":"session/new","description":"Creates a session."},
	"NewSessionResponse":{"type":"object","x-side":"agent","x-method":"session/new"},
	"LoadSessionRequest":{"type":"object","x-side":"agent","x-method":"session/load"},
	"LoadSessionResponse":{"type":"object","x-side":"agent","x-method":"session/load"},
	"CancelNotification":{"type":"object","x-side":"agent","x-method":"session/cancel","description":"Cancel."},
	"FooRequest":{"type":"object","x-side":"agent","x-method":"unstable/foo"},
	"FooResponse":{"type":"object","x-side":"agent","x-method":"unstable/foo","description":"Foo result."},
	"SessionNotification":{"type":"object","x-side":"client","x-method":"session/update"},
	"WriteRequest":{"type":"object","x-side":"client","x-method":"fs/write","description":"Write."},
	"WriteResponse":{"type":"null","x-side":"client","x-method":"fs/write","x-docs-ignore":true},
	"CancelRequestNotification":{"type":"object","x-side":"protocol","x-method":"$/cancel_request"}}`

// methodProtocol is the part of ACP that methodDefs exercises: session/load on
// AgentLoader, and session/update named SessionUpdate.
func methodProtocol() *Protocol {
	return &Protocol{
		Interfaces: []Interface{{Binding: BindAgentOptional, Name: "AgentLoader", Var: "loader"}},
		Methods: []MethodRule{
			{Side: "agent", Wire: "session/load", Binding: BindAgentOptional},
			{Side: "client", Wire: "session/update", GoName: "SessionUpdate"},
		},
	}
}

func methodMeta() *load.Meta {
	return &load.Meta{
		AgentMethods: map[string]string{"session_new": "session/new", "session_load": "session/load",
			"session_cancel": "session/cancel", "foo": "unstable/foo"},
		ClientMethods:   map[string]string{"session_update": "session/update", "fs_write": "fs/write"},
		ProtocolMethods: map[string]string{"cancel_request": "$/cancel_request"},
	}
}

func TestBuildMethods(t *testing.T) {
	t.Parallel()
	prov := &load.Provenance{UnstableWires: map[string]bool{"unstable/foo": true}}
	m, err := Build(Input{Meta: methodMeta(), Schema: schemaOf(t, methodDefs), Provenance: prov, Protocol: methodProtocol()})
	if err != nil {
		t.Fatal(err)
	}
	type row struct {
		key, goName, constName, req, resp, notif string
		result                                   bool
		binding                                  Binding
		doc                                      string
	}
	check := func(side string, got []*Method, want []row) {
		t.Helper()
		if len(got) != len(want) {
			t.Fatalf("%s: %d methods, want %d", side, len(got), len(want))
		}
		for i, w := range want {
			g := got[i]
			r := row{g.Key, g.GoName, g.Const, g.Req, g.Resp, g.Notif, g.HasResult, g.Binding, strings.Join(g.Doc, "|")}
			if r != w {
				t.Errorf("%s method %d:\n got %+v\nwant %+v", side, i, r, w)
			}
		}
	}
	check("agent", m.AgentMethods, []row{
		{"foo", "Foo", "AgentMethodFoo", "FooRequest", "FooResponse", "", true, BindAgentExperimental,
			"**UNSTABLE**||This capability is not part of the spec yet, and may be removed or changed at any point.||Foo result."},
		{"session_cancel", "Cancel", "AgentMethodSessionCancel", "", "", "CancelNotification", false, BindAgent, "Cancel."},
		{"session_load", "LoadSession", "AgentMethodSessionLoad", "LoadSessionRequest", "LoadSessionResponse", "", true, BindAgentOptional, ""},
		{"session_new", "NewSession", "AgentMethodSessionNew", "NewSessionRequest", "NewSessionResponse", "", true, BindAgent, "Creates a session."},
	})
	check("client", m.ClientMethods, []row{
		{"fs_write", "Write", "ClientMethodFsWrite", "WriteRequest", "WriteResponse", "", false, BindClient, ""},
		{"session_update", "SessionUpdate", "ClientMethodSessionUpdate", "", "", "SessionNotification", false, BindClient, ""},
	})
	if !m.ClientMethods[0].DocsIgnored || m.AgentMethods[3].DocsIgnored {
		t.Error("DocsIgnored is wrong")
	}
	if !m.AgentMethods[0].Unstable || m.AgentMethods[3].Unstable {
		t.Error("Unstable is wrong")
	}
	if len(m.Protocol) != 1 || m.Protocol[0].Wire != "$/cancel_request" || m.Protocol[0].Binding != BindUnknown || m.Protocol[0].Const != "ProtocolMethodCancelRequest" {
		t.Errorf("protocol methods: %+v", m.Protocol)
	}

	// Without provenance nothing is unstable, so foo binds to Agent.
	plain, err := Build(Input{Meta: methodMeta(), Schema: schemaOf(t, methodDefs), Protocol: methodProtocol()})
	if err != nil {
		t.Fatal(err)
	}
	if b := plain.AgentMethods[0].Binding; b != BindAgent {
		t.Errorf("foo without provenance: binding %v, want BindAgent", b)
	}
}

func TestBuildMethodsRefuses(t *testing.T) {
	t.Parallel()
	agent := func(methods map[string]string) *load.Meta { return &load.Meta{AgentMethods: methods} }
	tests := []struct {
		name string
		meta *load.Meta
		defs string
		want []string
	}{
		{
			name: "meta method with no linked types",
			meta: agent(map[string]string{"x": "a/x"}),
			defs: `{}`,
			want: []string{`agent method "a/x"`, "no request or notification definition is linked"},
		},
		{
			name: "two definitions in one role",
			meta: agent(map[string]string{"x": "a/x"}),
			defs: `{"ARequest":{"type":"object","x-side":"agent","x-method":"a/x"},"BRequest":{"type":"object","x-side":"agent","x-method":"a/x"}}`,
			want: []string{"definition BRequest", "already has ARequest in that role"},
		},
		{
			name: "request without a response",
			meta: agent(map[string]string{"x": "a/x"}),
			defs: `{"XRequest":{"type":"object","x-side":"agent","x-method":"a/x"}}`,
			want: []string{`agent method "a/x"`, "request XRequest has no linked response definition"},
		},
		{
			name: "request and notification",
			meta: agent(map[string]string{"x": "a/x"}),
			defs: `{"XRequest":{"type":"object","x-side":"agent","x-method":"a/x"},"XResponse":{"type":"object","x-side":"agent","x-method":"a/x"},
				"XNotification":{"type":"object","x-side":"agent","x-method":"a/x"}}`,
			want: []string{"both XRequest and XNotification are linked"},
		},
		{
			name: "linked definition without a role suffix",
			meta: agent(map[string]string{}),
			defs: `{"Thing":{"type":"object","x-side":"agent","x-method":"a/x"}}`,
			want: []string{"definition Thing", "ends in none of Request, Response, Notification"},
		},
		{
			name: "interface methods collide",
			meta: agent(map[string]string{"x": "a/x", "y": "a/y"}),
			defs: `{"FooRequest":{"type":"object","x-side":"agent","x-method":"a/x"},"FooResponse":{"type":"object","x-side":"agent","x-method":"a/x"},
				"FooNotification":{"type":"object","x-side":"agent","x-method":"a/y"}}`,
			want: []string{"interface Agent", `"a/x" and "a/y" both become Foo`},
		},
		{
			name: "definition named AgentLoader with a loader method",
			meta: agent(map[string]string{"session_load": "session/load"}),
			defs: `{"AgentLoader":{"type":"string"},"LoadSessionRequest":{"type":"object","x-side":"agent","x-method":"session/load"},
				"LoadSessionResponse":{"type":"object","x-side":"agent","x-method":"session/load"}}`,
			want: []string{"AgentLoader is already declared by definition AgentLoader"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := Build(Input{Meta: tt.meta, Schema: schemaOf(t, tt.defs), Protocol: methodProtocol()})
			if err == nil {
				t.Fatal("Build accepted the schema")
			}
			for _, w := range tt.want {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("error %q does not contain %q", err, w)
				}
			}
		})
	}
}
