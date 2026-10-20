package model

import (
	"slices"
	"strings"
	"testing"
)

// interfaceDoc returns the doc of m's interface for binding b, joined.
func interfaceDoc(m *Model, b Binding) string {
	for _, in := range m.Interfaces {
		if in.Binding == b {
			return strings.Join(in.Doc, " ")
		}
	}
	return ""
}

// TestAnsweredCancelledNotes requires the docs of the agent's and the client's
// interfaces to end with the exception to their paragraph on a handler's
// error, naming the method from the model: the prompt turn's method, and the
// request answered with its prompt turn. A table without those rules adds no
// such sentence, and no line is longer than 76 characters
// (0024-PLAN-behaviour-fixes.md, step 3 deviation).
func TestAnsweredCancelledNotes(t *testing.T) {
	t.Parallel()
	m, err := Build(Input{Meta: protocolMeta(), Schema: schemaOf(t, protocolDefs), Protocol: testProtocol()})
	if err != nil {
		t.Fatal(err)
	}
	prompt, ask := methodByWire(m, "s/prompt"), methodByWire(m, "s/ask")
	for _, tc := range []struct {
		b    Binding
		want string
	}{
		{BindAgent, prompt.GoName + " is the exception: once its turn is cancelled, it is answered with " + prompt.StopConst +
			" as its " + prompt.StopField + ", whatever it returns."},
		{BindClient, ask.GoName + " is the exception: once its prompt turn is cancelled, it is answered with " +
			ask.CancelledHelper + "() as its " + ask.CancelledField + ", whatever it returns."},
	} {
		if doc := interfaceDoc(m, tc.b); !strings.HasSuffix(doc, tc.want) {
			t.Errorf("binding %d: doc ends %q, want %q", tc.b, doc[max(0, len(doc)-len(tc.want)):], tc.want)
		}
	}
	for _, in := range m.Interfaces {
		for _, line := range in.Doc {
			if len(line) > 76 && strings.Contains(line, " is the exception") {
				t.Errorf("%s: line of %d characters: %q", in.Name, len(line), line)
			}
		}
	}

	bare, err := Build(Input{Meta: protocolMeta(), Schema: schemaOf(t, protocolDefs), Protocol: &Protocol{}})
	if err != nil {
		t.Fatal(err)
	}
	for _, in := range bare.Interfaces {
		if slices.ContainsFunc(in.Doc, func(l string) bool { return strings.Contains(l, "is the exception") }) {
			t.Errorf("%s: an exception without a prompt-turn rule: %q", in.Name, in.Doc)
		}
	}
	// The shared table names no method of any protocol.
	for _, in := range sdkInterfaces {
		if doc := strings.Join(in.Doc, " "); strings.Contains(doc, "is the exception") {
			t.Errorf("sdkInterfaces' %s names an exception: %q", in.Name, doc)
		}
	}
}

// checkCommittedAnsweredCancelled requires ACP's Agent and Client docs to name
// Prompt and RequestPermission as the exceptions.
func checkCommittedAnsweredCancelled(t *testing.T, m *Model) {
	t.Helper()
	for b, want := range map[Binding]string{
		BindAgent:  "Prompt is the exception: once its turn is cancelled, it is answered with StopReasonCancelled as its StopReason, whatever it returns.",
		BindClient: "RequestPermission is the exception: once its prompt turn is cancelled, it is answered with NewRequestPermissionOutcomeCancelled() as its Outcome, whatever it returns.",
	} {
		if doc := interfaceDoc(m, b); !strings.HasSuffix(doc, want) {
			t.Errorf("binding %d: the committed doc does not end %q", b, want)
		}
	}
}
