package acp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// The closed enumerations (0049-MADR-remediate-the-2026-11-01-debugging-pass.md,
// S6, and its Q4 a): what the SDK sends and Validate refuse a value a closed
// set does not list; what it receives keeps one, unless the schema lets the
// field default when malformed, where an unknown value reads as absent.

// closedPosition is a closed enumeration's field in a definition: a JSON
// template for the definition with %s where the value goes, and a value the
// set lists.
type closedPosition struct {
	name     string
	value    func() validatable
	template string
	known    string
}

// validatable is a generated type with Validate.
type validatable interface{ Validate() error }

func closedPositions() []closedPosition {
	return []closedPosition{
		{"PromptResponse.stopReason", func() validatable { return new(PromptResponse) }, `{"stopReason":%s}`, `"end_turn"`},
		{"PermissionOption.kind", func() validatable { return new(PermissionOption) }, `{"optionId":"o","name":"n","kind":%s}`, `"allow_once"`},
		{"PlanEntry.priority", func() validatable { return new(PlanEntry) }, `{"content":"c","priority":%s,"status":"pending"}`, `"high"`},
		{"PlanEntry.status", func() validatable { return new(PlanEntry) }, `{"content":"c","priority":"high","status":%s}`, `"pending"`},
		{"StringPropertySchema.format", func() validatable { return new(StringPropertySchema) }, `{"type":"string","format":%s}`, `"email"`},
		{"UnstableNesDiagnostic.severity", func() validatable { return new(UnstableNesDiagnostic) },
			`{"uri":"file:///a","range":{"start":{"line":0,"character":0},"end":{"line":0,"character":1}},"severity":%s,"message":"m"}`, `"error"`},
	}
}

// TestClosedEnumsRefusedOnSend requires Validate to refuse "" and an unknown
// value in a closed enumeration's field, with RuleEnum and a message that
// quotes the value, and to accept a value the set lists; and an agent whose
// Prompt returns PromptResponse{}, an empty stopReason, to answer -32603, so
// the client's Prompt returns that error.
func TestClosedEnumsRefusedOnSend(t *testing.T) {
	t.Parallel()
	for _, p := range closedPositions() {
		for _, raw := range []string{`""`, `"bogus"`, p.known} {
			v := p.value()
			if err := json.Unmarshal(fmt.Appendf(nil, p.template, raw), v); err != nil {
				t.Fatalf("%s %s: decode: %v", p.name, raw, err)
			}
			err := v.Validate()
			if raw == p.known {
				if err != nil {
					t.Errorf("%s %s: Validate = %v, want nil", p.name, raw, err)
				}
				continue
			}
			ve, ok := errors.AsType[*ValidationError](err)
			if !ok || ve.Rule != RuleEnum || !strings.Contains(err.Error(), raw) {
				t.Errorf("%s %s: Validate = %v, want a RuleEnum ValidationError quoting %s", p.name, raw, err, raw)
			}
		}
	}

	csc := promptPair(t, agentFuncs{PromptFunc: func(context.Context, PromptRequest) (PromptResponse, error) {
		return PromptResponse{}, nil
	}})
	_, err := csc.Prompt(t.Context(), PromptRequest{SessionId: "s", Prompt: []ContentBlock{TextBlock("hi")}})
	if re, ok := errors.AsType[*RequestError](err); !ok || re.Code != -32603 || !strings.Contains(fmt.Sprint(re.Data), "is not a StopReason") {
		t.Errorf("Prompt answered with PromptResponse{} = %v, want -32603 naming the stop reason", err)
	}
}

// TestClosedEnumsReadableOnReceive requires a client that receives a stop
// reason its closed set does not list to return it, as received, with no
// error, as Q4 (a) chose.
func TestClosedEnumsReadableOnReceive(t *testing.T) {
	t.Parallel()
	c, p, _ := initializeClient(t)
	if _, err := initializeAgainst(t.Context(), t, c, p, ""); err != nil {
		t.Fatal(err)
	}
	type result struct {
		r   PromptResponse
		err error
	}
	done := make(chan result, 1)
	go func() {
		r, err := c.Prompt(t.Context(), PromptRequest{SessionId: "s", Prompt: []ContentBlock{TextBlock("hi")}})
		done <- result{r, err}
	}()
	req := p.next("the prompt")
	p.send(fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"result":{"stopReason":"bogus"}}`, req.ID))
	if got := within(t, done, "Prompt"); got.err != nil || got.r.StopReason != "bogus" {
		t.Errorf("Prompt = %q, %v; want \"bogus\", nil", got.r.StopReason, got.err)
	}
}

// TestLenientClosedEnumsReadAbsent requires a closed enumeration's unknown
// value, in a field the schema lets default when malformed, to read as absent:
// a value field as "" or its schema default, a pointer as nil, and an item of
// a skip-invalid-items array as dropped (0012-MADR-schema-fidelity-framework.md,
// treatments 5 and 6). A listed value is kept.
func TestLenientClosedEnumsReadAbsent(t *testing.T) {
	t.Parallel()
	decode := func(t *testing.T, raw string, v any) {
		t.Helper()
		if err := json.Unmarshal([]byte(raw), v); err != nil {
			t.Fatalf("%s: %v", raw, err)
		}
	}
	var call, known SessionUpdate
	decode(t, `{"sessionUpdate":"tool_call","toolCallId":"t","title":"T","kind":"browse","status":"queued"}`, &call)
	decode(t, `{"sessionUpdate":"tool_call","toolCallId":"t","title":"T","kind":"read","status":"pending"}`, &known)
	if tc := call.ToolCall; tc.Kind != "" || tc.Status != "" {
		t.Errorf("tool_call kind %q status %q, want both absent", tc.Kind, tc.Status)
	}
	if tc := known.ToolCall; tc.Kind != ToolKindRead || tc.Status != ToolCallStatusPending {
		t.Errorf("tool_call kind %q status %q, want read and pending kept", tc.Kind, tc.Status)
	}
	var update SessionUpdate
	decode(t, `{"sessionUpdate":"tool_call_update","toolCallId":"t","kind":"browse","status":"queued"}`, &update)
	if u := update.ToolCallUpdate; u.Kind != nil || u.Status != nil {
		t.Errorf("tool_call_update kind %v status %v, want both nil", u.Kind, u.Status)
	}
	var a Annotations
	decode(t, `{"audience":["user","system","assistant"]}`, &a)
	if !reflect.DeepEqual(a.Audience, []Role{RoleUser, RoleAssistant}) {
		t.Errorf("audience %q, want [user assistant]", a.Audience)
	}
	var s ElicitationSchema
	decode(t, `{"type":"array","properties":{}}`, &s)
	if s.Type != "object" {
		t.Errorf("ElicitationSchema type %q, want its default, object", s.Type)
	}
	var caps AgentCapabilities
	decode(t, `{"positionEncoding":"utf-7"}`, &caps)
	if caps.PositionEncoding != nil {
		t.Errorf("positionEncoding %q, want nil", *caps.PositionEncoding)
	}
}
