package acp

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

// TestAllowsValue requires Negotiation.AllowsValue to return, without
// sending, the error a typed send returns: for each capability rule of
// valueGates with a record that does not advertise it, the send's
// *CapabilityError, for the value and for a pointer to it, and nil with one
// that does; for each method gate of gatedMethods, Allows' error; a
// result's, as a received result gets it; and the terminal auth method's.
// The cases are the tables', so a rule added to them is covered
// (0041-MADR-close-the-gaps-the-consumers-wait-on.md, item 4).
func TestAllowsValue(t *testing.T) {
	t.Parallel()
	for _, g := range valueGates(t) {
		name := g.capability + " " + g.variant + g.field
		without, with := allAdvertised(), allAdvertised()
		g.advertise(without, false)
		w := &countingWriter{}
		asc, csc := sides(t, w, without)
		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
		sent := g.send(ctx, asc, csc, g.params)
		cancel()
		if _, ok := errors.AsType[*CapabilityError](sent); !ok {
			t.Fatalf("%s: the send = %v, want a *CapabilityError: the table's case does not hold", name, sent)
		}
		if got := without.AllowsValue(g.wire, g.params); !reflect.DeepEqual(got, sent) {
			t.Errorf("%s: AllowsValue = %v, want the send's error, %v", name, got, sent)
		}
		p := reflect.New(reflect.TypeOf(g.params))
		p.Elem().Set(reflect.ValueOf(g.params))
		if got := without.AllowsValue(g.wire, p.Interface()); !reflect.DeepEqual(got, sent) {
			t.Errorf("%s: AllowsValue of a pointer = %v, want the send's error, %v", name, got, sent)
		}
		if err := with.AllowsValue(g.wire, g.params); err != nil {
			t.Errorf("%s: AllowsValue with the capability = %v, want nil", name, err)
		}
	}
	for _, m := range gatedMethods() {
		without, only := records(m)
		if got, want := without.AllowsValue(m.wire, m.params), without.Allows(m.wire); want == nil || !reflect.DeepEqual(got, want) {
			t.Errorf("%s: AllowsValue without its capability = %v, want Allows', %v", m.wire, got, want)
		}
		if err := only.AllowsValue(m.wire, m.params); err != nil {
			t.Errorf("%s: AllowsValue with only %s = %v, want nil", m.wire, m.capability, err)
		}
	}
	n := allAdvertised()
	n.Request.ClientCapabilities.Session.ConfigOptions.Boolean = nil
	result := decoded[NewSessionResponse](t, `{"sessionId":"s","configOptions":[{"type":"boolean","id":"b","name":"B","currentValue":true}]}`)
	want := gateResult(n, AgentMethodSessionNew, &result)
	if got := n.AllowsValue(AgentMethodSessionNew, result); want == nil || !reflect.DeepEqual(got, want) {
		t.Errorf("AllowsValue of a session/new result with a boolean option = %v, want the result gate's, %v", got, want)
	}
	n = allAdvertised()
	n.Response.AuthMethods = []AuthMethod{NewAuthMethodAgent("a", "A"), NewAuthMethodTerminal("t", "T")}
	if err := n.AllowsValue(AgentMethodAuthenticate, AuthenticateRequest{MethodId: "t"}); !errors.Is(err, ErrTerminalAuthMethod) {
		t.Errorf("AllowsValue of authenticate naming a terminal method = %v, want ErrTerminalAuthMethod", err)
	}
	if err := n.AllowsValue(AgentMethodAuthenticate, AuthenticateRequest{MethodId: "a"}); err != nil {
		t.Errorf("AllowsValue of authenticate naming an agent method = %v, want nil", err)
	}
	if err := versionOnly().AllowsValue(AgentMethodSessionNew, nil); err != nil {
		t.Errorf("AllowsValue of no params = %v, want nil", err)
	}
}
