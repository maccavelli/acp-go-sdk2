package acp

import (
	"errors"
	"slices"
	"testing"
)

// TestAuthMethodID requires ID to return each variant's id, an entry of a
// type this SDK does not know its "id" when that is a JSON string, the key
// folded as its decoder folds it, and "" otherwise and for the zero value
// (0050-MADR-auth-method-ids-and-the-prompt-result-for-issue-14.md, D3).
func TestAuthMethodID(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		a    AuthMethod
		want AuthMethodId
	}{
		{"agent", NewAuthMethodAgent("openai", "OpenAI"), "openai"},
		{"terminal", NewAuthMethodTerminal("grok-login", "Grok"), "grok-login"},
		{"unknown with id", decoded[AuthMethod](t, `{"type":"future","id":"f","name":"F"}`), "f"},
		{"unknown with ID", decoded[AuthMethod](t, `{"type":"future","ID":"g","name":"F"}`), "g"},
		{"unknown with an empty id", decoded[AuthMethod](t, `{"type":"future","id":"","name":"F"}`), ""},
		{"unknown without id", decoded[AuthMethod](t, `{"type":"future","name":"F"}`), ""},
		{"unknown with a null id", decoded[AuthMethod](t, `{"type":"future","id":null,"name":"F"}`), ""},
		{"unknown with a number id", decoded[AuthMethod](t, `{"type":"future","id":7,"name":"F"}`), ""},
		{"zero", AuthMethod{}, ""},
	} {
		if got := tc.a.ID(); got != tc.want {
			t.Errorf("%s: ID() = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// TestAuthenticateMethodIDsIsTheGate requires every id AuthenticateMethodIDs
// returns to pass authenticate's gate, and every other id of the answer, and
// "unknown" and "stranger", to be refused, as terminal or as unadvertised; and
// the list to keep the answer's order, each id once
// (0050-MADR-auth-method-ids-and-the-prompt-result-for-issue-14.md, D2 and D3).
func TestAuthenticateMethodIDsIsTheGate(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		methods []AuthMethod
		want    []AuthMethodId
	}{
		{"agents and a terminal", []AuthMethod{NewAuthMethodAgent("openai", "OpenAI"), NewAuthMethodAgent("grok", "Grok"),
			NewAuthMethodTerminal("grok-login", "Grok login")}, []AuthMethodId{"openai", "grok"}},
		{"an id both terminal and agent", []AuthMethod{NewAuthMethodAgent("a", "A"), NewAuthMethodTerminal("t", "T"),
			NewAuthMethodAgent("t", "T, agent")}, []AuthMethodId{"a"}},
		{"unknown entries", []AuthMethod{decoded[AuthMethod](t, `{"type":"future","name":"F"}`),
			decoded[AuthMethod](t, `{"type":"future","id":"f","name":"F"}`), NewAuthMethodAgent("a", "A")},
			[]AuthMethodId{"f", "a"}},
		{"a duplicate agent id", []AuthMethod{NewAuthMethodAgent("b", "B"), NewAuthMethodAgent("a", "A"),
			NewAuthMethodAgent("b", "B again")}, []AuthMethodId{"b", "a"}},
		{"none", nil, nil},
	} {
		got := AuthenticateMethodIDs(tc.methods)
		if !slices.Equal(got, tc.want) {
			t.Errorf("%s: AuthenticateMethodIDs = %q, want %q", tc.name, got, tc.want)
		}
		n := allAdvertised()
		n.Response.AuthMethods = tc.methods
		others := []AuthMethodId{"unknown", "stranger"}
		for _, m := range tc.methods {
			others = append(others, m.ID())
		}
		for _, id := range got {
			if err := n.AllowsValue(AgentMethodAuthenticate, AuthenticateRequest{MethodId: id}); err != nil {
				t.Errorf("%s: %q is listed, and AllowsValue refuses it: %v", tc.name, id, err)
			}
		}
		for _, id := range others {
			if slices.Contains(got, id) {
				continue
			}
			err := n.AllowsValue(AgentMethodAuthenticate, AuthenticateRequest{MethodId: id})
			if !errors.Is(err, ErrTerminalAuthMethod) && !errors.Is(err, ErrUnadvertisedAuthMethod) {
				t.Errorf("%s: %q is not listed, and AllowsValue = %v, want it refused", tc.name, id, err)
			}
		}
	}
}
