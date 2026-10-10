package acp

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// These tests cover how the generated types follow the schema's keywords: see
// docs/decisions/0012-PLAN-schema-fidelity-framework.md.

// TestIntegerFormatsBoundDecoding requires an integer field to reject a value
// outside the range of its declared format.
func TestIntegerFormatsBoundDecoding(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, in string
		into     any
	}{
		{"uint64 outputByteLimit, negative", `{"sessionId":"s","command":"c","outputByteLimit":-1}`, &CreateTerminalRequest{}},
		{"uint32 line, 2^32", `{"sessionId":"s","path":"p","line":4294967296}`, &ReadTextFileRequest{}},
		{"uint16 protocolVersion, 65536", `{"protocolVersion":65536}`, &InitializeRequest{}},
		{"int64 size, 2^63", `{"name":"n","uri":"u","size":9223372036854775808}`, &ResourceLink{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := json.Unmarshal([]byte(tt.in), tt.into)
			if err == nil || !strings.Contains(err.Error(), "cannot unmarshal number") {
				t.Errorf("decoded %s: %v", tt.in, err)
			}
		})
	}
	var r ResourceLink
	if err := json.Unmarshal([]byte(`{"name":"n","uri":"u","size":9007199254740993}`), &r); err != nil || r.Size == nil || *r.Size != 9007199254740993 {
		t.Errorf("size 2^53+1: %v %v", err, r.Size)
	}
}

// TestTypedAdditionalProperties requires an object whose additionalProperties
// names a schema to be a map of that type.
func TestTypedAdditionalProperties(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name  string
		field any
		want  string
	}{
		{"AuthMethodTerminalInline.Env", AuthMethodTerminalInline{}.Env, "map[string]string"},
		{"AuthMethodTerminal.Env", AuthMethodTerminal{}.Env, "map[string]string"},
		{"CreateElicitationAccept.Content", CreateElicitationAccept{}.Content, "map[string]acp.ElicitationContentValue"},
		{"ElicitationAcceptAction.Content", ElicitationAcceptAction{}.Content, "map[string]acp.ElicitationContentValue"},
		{"ElicitationSchema.Properties", ElicitationSchema{}.Properties, "map[string]acp.ElicitationPropertySchema"},
		{"UnstableSetProviderRequest.Headers", UnstableSetProviderRequest{}.Headers, "map[string]string"},
	} {
		if got := reflect.TypeOf(tt.field).String(); got != tt.want {
			t.Errorf("%s is %s, want %s", tt.name, got, tt.want)
		}
	}
	var ok AuthMethodTerminal
	if err := json.Unmarshal([]byte(`{"id":"i","name":"n","env":{"A":"1"}}`), &ok); err != nil {
		t.Fatal(err)
	}
	var bad AuthMethodTerminal
	if err := json.Unmarshal([]byte(`{"id":"i","name":"n","env":{"A":1}}`), &bad); err == nil {
		t.Error("a number in env decoded")
	}
}

// TestRawJSONKeepsDigits requires a value the schema gives no Go type to be
// carried byte for byte, so an integer above 2^53 keeps every digit.
func TestRawJSONKeepsDigits(t *testing.T) {
	t.Parallel()
	const digits = "9007199254740993"
	for _, tt := range []struct {
		name, in string
		into     any
	}{
		{"SessionUpdate rawInput", `{"sessionUpdate":"tool_call","toolCallId":"t","title":"x","rawInput":{"n":` + digits + `}}`, &SessionUpdate{}},
		{"Error data", `{"code":-32000,"message":"m","data":{"n":` + digits + `}}`, &Error{}},
		{"AgentRequest params", `{"id":1,"method":"x","params":{"n":` + digits + `}}`, &AgentRequest{}},
	} {
		if err := json.Unmarshal([]byte(tt.in), tt.into); err != nil {
			t.Fatalf("%s: %v", tt.name, err)
		}
		b, err := json.Marshal(reflect.ValueOf(tt.into).Elem().Interface())
		if err != nil || !strings.Contains(string(b), `{"n":`+digits+`}`) {
			t.Errorf("%s: encoded %s, %v", tt.name, b, err)
		}
	}
}

// TestStartLocationsMergesRawInput requires WithStartLocations to add path to
// a raw object input without touching its other keys, and to leave a raw input
// that is not an object alone.
func TestStartLocationsMergesRawInput(t *testing.T) {
	t.Parallel()
	loc := WithStartLocations([]ToolCallLocation{{Path: "/p"}})
	for _, tt := range []struct {
		name string
		opts []ToolCallStartOpt
		want string
	}{
		{"no input", []ToolCallStartOpt{loc}, `{"path":"/p"}`},
		{"object input", []ToolCallStartOpt{WithStartRawInput(json.RawMessage(`{"n":9007199254740993}`)), loc}, `{"n":9007199254740993,"path":"/p"}`},
		{"object with a path", []ToolCallStartOpt{WithStartRawInput(json.RawMessage(`{"path":"/q"}`)), loc}, `{"path":"/q"}`},
		{"array input", []ToolCallStartOpt{WithStartRawInput(json.RawMessage(`[1]`)), loc}, `[1]`},
	} {
		u := StartToolCall("t", "x", tt.opts...)
		b, err := json.Marshal(u.ToolCall.RawInput)
		if err != nil || string(b) != tt.want {
			t.Errorf("%s: rawInput %s, %v; want %s", tt.name, b, err, tt.want)
		}
	}
}

// TestStartEditToolCallRawInput requires the edit tool call's raw input to be
// {"content": content, "path": path} byte for byte, content as given.
func TestStartEditToolCallRawInput(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		content json.RawMessage
		want    string
	}{
		{json.RawMessage(`"x = 1"`), `{"content":"x = 1","path":"/a.py"}`},
		{json.RawMessage(`{"n":9007199254740993}`), `{"content":{"n":9007199254740993},"path":"/a.py"}`},
		{nil, `{"content":null,"path":"/a.py"}`},
	} {
		u := StartEditToolCall("t", "edit", "/a.py", tt.content)
		if got := string(u.ToolCall.RawInput); got != tt.want {
			t.Errorf("rawInput %s, want %s", got, tt.want)
		}
	}
}

// TestStructValuesHaveNoOmitempty requires the struct-valued capability fields
// to say in their tags what the encoder does: always written.
func TestStructValuesHaveNoOmitempty(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		v     any
		field string
	}{
		{AgentCapabilities{}, "Auth"}, {AgentCapabilities{}, "McpCapabilities"}, {AgentCapabilities{}, "PromptCapabilities"},
		{AgentCapabilities{}, "SessionCapabilities"}, {ClientCapabilities{}, "Auth"}, {ClientCapabilities{}, "Fs"},
		{InitializeRequest{}, "ClientCapabilities"}, {InitializeResponse{}, "AgentCapabilities"},
	} {
		f, ok := reflect.TypeOf(tt.v).FieldByName(tt.field)
		if !ok {
			t.Fatalf("%T has no %s", tt.v, tt.field)
		}
		if tag := f.Tag.Get("json"); strings.Contains(tag, "omitempty") {
			t.Errorf("%T.%s tag %q", tt.v, tt.field, tag)
		}
	}
	b, err := json.Marshal(InitializeResponse{})
	if err != nil || !strings.Contains(string(b), `"agentCapabilities":{`) {
		t.Errorf("InitializeResponse{} encodes %s, %v", b, err)
	}
}
