package acp

import (
	"encoding/json"
	"testing"
)

// The tests here guard the regression of 0012-MADR-schema-fidelity-framework.md,
// amendment of 2026-10-11: a raw value that is absent or null is empty, as ACP
// v1 defines "Omission and null are equivalent for rawInput and rawOutput".
// An empty raw value has len 0 and string(v) == "". They do not test what fmt
// prints for it: under Go 1.27's default build json.RawMessage is
// jsontext.Value, whose String method gives "null" for an empty value
// (golang/go issue 81050), which this SDK does not change.

// rawKeys decodes b into its keys and their raw values.
func rawKeys(t *testing.T, b []byte) map[string]json.RawMessage {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("%s: %v", b, err)
	}
	return m
}

// TestRawToolInputNullIsAbsent is the reported case: a tool call and a tool
// call update whose rawInput and rawOutput are absent or null decode to an
// empty value and are not sent; any other value is kept and sent byte for
// byte, integers beyond 2^53 included.
func TestRawToolInputNullIsAbsent(t *testing.T) {
	t.Parallel()
	type raw struct{ in, out json.RawMessage }
	decoders := []struct {
		name, head string
		decode     func(t *testing.T, b []byte) (raw, []byte)
	}{
		{"ToolCall", `{"toolCallId":"1","title":"t"`, func(t *testing.T, b []byte) (raw, []byte) {
			t.Helper()
			var v ToolCall
			if err := json.Unmarshal(b, &v); err != nil {
				t.Fatal(err)
			}
			out, err := json.Marshal(v)
			if err != nil {
				t.Fatal(err)
			}
			return raw{v.RawInput, v.RawOutput}, out
		}},
		{"ToolCallUpdate", `{"toolCallId":"1"`, func(t *testing.T, b []byte) (raw, []byte) {
			t.Helper()
			var v ToolCallUpdate
			if err := json.Unmarshal(b, &v); err != nil {
				t.Fatal(err)
			}
			out, err := json.Marshal(v)
			if err != nil {
				t.Fatal(err)
			}
			return raw{v.RawInput, v.RawOutput}, out
		}},
		{"tool_call", `{"sessionUpdate":"tool_call","toolCallId":"1","title":"t"`, func(t *testing.T, b []byte) (raw, []byte) {
			t.Helper()
			var u SessionUpdate
			if err := json.Unmarshal(b, &u); err != nil {
				t.Fatal(err)
			}
			out, err := json.Marshal(u)
			if err != nil {
				t.Fatal(err)
			}
			return raw{u.ToolCall.RawInput, u.ToolCall.RawOutput}, out
		}},
		{"tool_call_update", `{"sessionUpdate":"tool_call_update","toolCallId":"1"`, func(t *testing.T, b []byte) (raw, []byte) {
			t.Helper()
			var u SessionUpdate
			if err := json.Unmarshal(b, &u); err != nil {
				t.Fatal(err)
			}
			out, err := json.Marshal(u)
			if err != nil {
				t.Fatal(err)
			}
			return raw{u.ToolCallUpdate.RawInput, u.ToolCallUpdate.RawOutput}, out
		}},
	}
	values := []struct {
		name, wire string // wire is the value sent, "" for absent
		want       string // the decoded bytes, "" for empty
	}{
		{"absent", "", ""},
		{"null", "null", ""},
		{"empty object", "{}", "{}"},
		{"integer beyond 2^53", `{"n":9007199254740993}`, `{"n":9007199254740993}`},
	}
	for _, d := range decoders {
		for _, v := range values {
			t.Run(d.name+"/"+v.name, func(t *testing.T) {
				t.Parallel()
				in := d.head
				if v.wire != "" {
					in += `,"rawInput":` + v.wire + `,"rawOutput":` + v.wire
				}
				got, out := d.decode(t, []byte(in+"}"))
				for field, r := range map[string]json.RawMessage{"rawInput": got.in, "rawOutput": got.out} {
					if string(r) != v.want || (v.want == "" && len(r) != 0) {
						t.Errorf("%s decoded as %q (len %d), want %q", field, string(r), len(r), v.want)
					}
					sent, ok := rawKeys(t, out)[field]
					if v.want == "" && ok {
						t.Errorf("%s sent as %s, want it omitted: %s", field, sent, out)
					}
					if v.want != "" && string(sent) != v.want {
						t.Errorf("%s sent as %s, want %s", field, sent, v.want)
					}
				}
			})
		}
	}
}

// TestOptionalRawFieldsNullIsAbsent covers the other optional raw fields: a
// null decodes to an empty value, and a value is kept.
func TestOptionalRawFieldsNullIsAbsent(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, head string
		get        func(t *testing.T, b []byte) json.RawMessage
	}{
		{"Error.data", `{"code":1,"message":"m","data":`, func(t *testing.T, b []byte) json.RawMessage {
			t.Helper()
			var v Error
			mustDecode(t, b, &v)
			return v.Data
		}},
		{"AgentNotification.params", `{"method":"m","params":`, func(t *testing.T, b []byte) json.RawMessage {
			t.Helper()
			var v AgentNotification
			mustDecode(t, b, &v)
			return v.Params
		}},
		{"AgentRequest.params", `{"id":1,"method":"m","params":`, func(t *testing.T, b []byte) json.RawMessage {
			t.Helper()
			var v AgentRequest
			mustDecode(t, b, &v)
			return v.Params
		}},
		{"ClientNotification.params", `{"method":"m","params":`, func(t *testing.T, b []byte) json.RawMessage {
			t.Helper()
			var v ClientNotification
			mustDecode(t, b, &v)
			return v.Params
		}},
		{"ClientRequest.params", `{"id":1,"method":"m","params":`, func(t *testing.T, b []byte) json.RawMessage {
			t.Helper()
			var v ClientRequest
			mustDecode(t, b, &v)
			return v.Params
		}},
		{"IdleStateUpdateNone.stopReason", `{"stopReason":`, func(t *testing.T, b []byte) json.RawMessage {
			t.Helper()
			var v IdleStateUpdateNone
			mustDecode(t, b, &v)
			return v.StopReason
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.get(t, []byte(tt.head+"null}")); len(got) != 0 || string(got) != "" {
				t.Errorf("null decoded as %q, want empty", string(got))
			}
			if got := tt.get(t, []byte(tt.head+`{"n":9007199254740993}}`)); string(got) != `{"n":9007199254740993}` {
				t.Errorf("a value decoded as %q", string(got))
			}
		})
	}
}

// TestUnstableMcpErrorKeepsExtraAndNull covers McpError, whose schema keeps
// the keys it does not declare and an explicit null data apart from an omitted
// one (0019-PLAN-generator-hardening.md, G2 and G3): each is read and sent back
// as received.
func TestUnstableMcpErrorKeepsExtraAndNull(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, in, data string
		extra          map[string]string
	}{
		{"absent", `{"code":1,"message":"m"}`, "", nil},
		{"null", `{"code":1,"message":"m","data":null}`, "null", nil},
		{"value", `{"code":1,"message":"m","data":{"n":9007199254740993}}`, `{"n":9007199254740993}`, nil},
		{"extra keys", `{"code":1,"message":"m","vendor":"x","n":[1,2]}`, "", map[string]string{"vendor": `"x"`, "n": `[1,2]`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var v UnstableMcpError
			mustDecode(t, []byte(tt.in), &v)
			if string(v.Data) != tt.data || (tt.data == "" && v.Data != nil) {
				t.Errorf("data decoded as %q, want %q", string(v.Data), tt.data)
			}
			if len(v.Extra) != len(tt.extra) {
				t.Errorf("Extra %v, want %v", v.Extra, tt.extra)
			}
			for k, want := range tt.extra {
				if string(v.Extra[k]) != want {
					t.Errorf("Extra[%q] = %s, want %s", k, v.Extra[k], want)
				}
			}
			out, err := json.Marshal(v)
			if err != nil {
				t.Fatal(err)
			}
			sent := rawKeys(t, out)
			if got, ok := sent["data"]; (tt.data == "" && ok) || string(got) != tt.data {
				t.Errorf("data sent as %s, want %q: %s", got, tt.data, out)
			}
			for k, want := range tt.extra {
				if string(sent[k]) != want {
					t.Errorf("%s sent as %s, want %s: %s", k, sent[k], want, out)
				}
			}
		})
	}
	t.Run("a declared key wins", func(t *testing.T) {
		t.Parallel()
		v := UnstableMcpError{Code: 1, Message: "m", Extra: map[string]json.RawMessage{"message": json.RawMessage(`"x"`)}}
		out, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		if got := string(rawKeys(t, out)["message"]); got != `"m"` {
			t.Errorf("message sent as %s, want \"m\": %s", got, out)
		}
	})
}

// TestRequiredRawResultKeepsNull requires a required raw field to keep null:
// it is the value the field carries.
func TestRequiredRawResultKeepsNull(t *testing.T) {
	t.Parallel()
	var v AgentResult
	mustDecode(t, []byte(`{"id":1,"result":null}`), &v)
	if string(v.Result) != "null" {
		t.Errorf("result decoded as %q, want null", string(v.Result))
	}
	out, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if sent := rawKeys(t, out)["result"]; string(sent) != "null" {
		t.Errorf("result sent as %s, want null: %s", sent, out)
	}
}

// TestStartLocationsAfterNullInput requires WithStartLocations to add the
// path to a raw input that is null, decoded or given.
func TestStartLocationsAfterNullInput(t *testing.T) {
	t.Parallel()
	loc := WithStartLocations([]ToolCallLocation{{Path: "/p"}})
	var u SessionUpdate
	mustDecode(t, []byte(`{"sessionUpdate":"tool_call","toolCallId":"1","title":"t","rawInput":null}`), &u)
	loc(u.ToolCall)
	if got := string(u.ToolCall.RawInput); got != `{"path":"/p"}` {
		t.Errorf("decoded null input: rawInput = %q, want {\"path\":\"/p\"}", got)
	}
	given := StartToolCall("1", "t", WithStartRawInput(json.RawMessage("null")), loc)
	if got := string(given.ToolCall.RawInput); got != `{"path":"/p"}` {
		t.Errorf("given null input: rawInput = %q, want {\"path\":\"/p\"}", got)
	}
}

func mustDecode(t *testing.T, b []byte, v any) {
	t.Helper()
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatalf("%s: %v", b, err)
	}
}
