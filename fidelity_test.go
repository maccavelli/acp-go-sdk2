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
	// size carries x-deserialize-default-on-error: a value outside int64 falls
	// back to its zero value instead of failing the link.
	var over ResourceLink
	if err := json.Unmarshal([]byte(`{"name":"n","uri":"u","size":9223372036854775808}`), &over); err != nil || over.Size != nil {
		t.Errorf("size 2^63: %v %v, want no size", err, over.Size)
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

// TestRequiredKeysOnDecode requires decoding to fail, naming the type and the
// property, when a required key is absent, and dispatch to answer -32602.
func TestRequiredKeysOnDecode(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name, in string
		into     any
		want     string
	}{
		{"PromptRequest without sessionId", `{"prompt":[]}`, &PromptRequest{}, `PromptRequest: missing required property "sessionId"`},
		{"InitializeRequest without protocolVersion", `{}`, &InitializeRequest{}, `InitializeRequest: missing required property "protocolVersion"`},
		{"image block without data", `{"type":"image","mimeType":"image/png"}`, &ContentBlock{}, `missing required property "data"`},
		{"null for a struct with a required key", `null`, &PromptRequest{}, `PromptRequest: missing required property "prompt"`},
	} {
		err := json.Unmarshal([]byte(tt.in), tt.into)
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s: %v, want an error containing %q", tt.name, err, tt.want)
		}
	}
	var c ContentBlock
	if err := json.Unmarshal([]byte(`{"text":"hi"}`), &c); err != nil || c.Text == nil {
		t.Errorf("text block without its discriminator: %v", err)
	}
	// Dispatch must refuse the request before it reaches the (absent) agent.
	reqErr, panicked := func() (e *RequestError, p any) {
		defer func() { p = recover() }()
		a := &AgentSideConnection{sessionCancels: map[string]context.CancelFunc{}}
		_, e = a.handle(context.Background(), AgentMethodSessionPrompt, json.RawMessage(`{"prompt":[]}`))
		return e, nil
	}()
	data, _ := reqErr.dataMap()
	if panicked != nil || reqErr == nil || reqErr.Code != -32602 || !strings.Contains(fmt.Sprint(data["error"]), `missing required property "sessionId"`) {
		t.Errorf("dispatch: %+v, panic %v", reqErr, panicked)
	}
}

// dataMap returns e.Data as a map, or nil.
func (e *RequestError) dataMap() (map[string]any, bool) {
	if e == nil {
		return nil, false
	}
	m, ok := e.Data.(map[string]any)
	return m, ok
}

// TestLenientFields requires x-deserialize-default-on-error to replace a
// malformed value with its schema default, or the zero value, and
// x-deserialize-skip-invalid-items to drop the array items that fail.
func TestLenientFields(t *testing.T) {
	t.Parallel()
	var garbage, absent InitializeResponse
	if err := json.Unmarshal([]byte(`{"protocolVersion":1,"agentCapabilities":"garbage","authMethods":"x"}`), &garbage); err != nil {
		t.Errorf("malformed lenient fields: %v", err)
	}
	if err := json.Unmarshal([]byte(`{"protocolVersion":1}`), &absent); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(garbage, absent) {
		t.Errorf("malformed fields decode as %+v, want the defaults %+v", garbage, absent)
	}
	var schema ElicitationSchema
	if err := json.Unmarshal([]byte(`{"type":5,"properties":{}}`), &schema); err != nil || schema.Type != "object" {
		t.Errorf("ElicitationSchema type 5: %v, Type %q, want the default object", err, schema.Type)
	}
	var plan Plan
	if err := json.Unmarshal([]byte(`{"entries":[{"content":"x","priority":"high","status":"pending"},5,{"content":"y"}]}`), &plan); err != nil || len(plan.Entries) != 1 {
		t.Errorf("Plan with invalid entries: %v, %d entries, want 1", err, len(plan.Entries))
	}
	var notArray Plan
	if err := json.Unmarshal([]byte(`{"entries":"x"}`), &notArray); err != nil || notArray.Entries != nil {
		t.Errorf("Plan entries not an array: %v, %v", err, notArray.Entries)
	}
	if err := json.Unmarshal([]byte(`{}`), &Plan{}); err == nil || !strings.Contains(err.Error(), `Plan: missing required property "entries"`) {
		t.Errorf("Plan without entries: %v", err)
	}
	var tc ToolCall
	if err := json.Unmarshal([]byte(`{"toolCallId":"t","title":"x","content":[{"type":"zz_future"}]}`), &tc); err != nil ||
		len(tc.Content) != 1 || tc.Content[0].Unknown == nil {
		t.Errorf("ToolCall content with an unknown variant: %v, %+v", err, tc.Content)
	}
}

// TestValidationErrorRules requires every kind of generated check to return a
// *ValidationError naming the type, the key, the value and the rule,
// reachable with errors.As through a parent's wrapping, with the message
// TestValidateSchemaConstraints pins; minimum and maximum, which no committed
// type has, are checked by the engine cases accept-validate-minimum and
// accept-validate-maximum (0015-PLAN-validate-absolute-protocol-paths.md,
// step 3).
func TestValidationErrorRules(t *testing.T) {
	t.Parallel()
	type validator interface{ Validate() error }
	for _, tt := range []struct {
		name        string
		v           validator
		typ, field  string
		value       any
		rule        ValidationRule
		wantMessage string
	}{
		{"minLength", &Notice{}, "Notice", "title", "", RuleMinLength, "Notice: title: shorter than minLength 1"},
		{"one variant", &ContentBlock{}, "ContentBlock", "", nil, RuleOneVariant, "ContentBlock must have exactly one variant set"},
		{"one variant, nested", &PromptRequest{SessionId: "s", Prompt: []ContentBlock{{}}}, "ContentBlock", "", nil, RuleOneVariant,
			"PromptRequest: prompt[0]: ContentBlock must have exactly one variant set"},
		{"one member", &CreateElicitationForm{Mode: "form"}, "CreateElicitationForm", "", nil, RuleOneMember,
			"CreateElicitationForm must have exactly one member set"},
		{"excluded value", &ElicitationPropertySchema{Other: &ElicitationPropertySchemaOther{Type: "string"}}, "ElicitationPropertySchema",
			"type", "string", RuleExcludedValue, `ElicitationPropertySchema: variant Other: type "string" names another variant`},
	} {
		err := tt.v.Validate()
		var ve *ValidationError
		if !errors.As(err, &ve) {
			t.Errorf("%s: Validate() = %v, not a *ValidationError", tt.name, err)
			continue
		}
		if ve.Type != tt.typ || ve.Field != tt.field || ve.Value != tt.value || ve.Rule != tt.rule {
			t.Errorf("%s: %+v, want Type %q Field %q Value %v Rule %q", tt.name, *ve, tt.typ, tt.field, tt.value, tt.rule)
		}
		if err.Error() != tt.wantMessage {
			t.Errorf("%s: message %q, want %q", tt.name, err.Error(), tt.wantMessage)
		}
	}
	// A ValidationError a consumer builds, with no message, still reads.
	if got := (&ValidationError{Type: "T", Field: "f", Rule: RuleAbsolutePath}).Error(); got != "T: f: absolutePath" {
		t.Errorf("a ValidationError without a message reads %q", got)
	}
	if got := (&ValidationError{Type: "T", Rule: RuleOneVariant}).Error(); got != "T: oneVariant" {
		t.Errorf("a ValidationError without a field reads %q", got)
	}
}

// TestValidateSchemaConstraints requires Validate to check what the schema
// states, at every depth, and nothing else: a union has exactly one variant
// set, a string meets its minLength, and a nested value, slice element, map
// value, or pointer target validates; a present empty string passes.
func TestValidateSchemaConstraints(t *testing.T) {
	t.Parallel()
	type validator interface{ Validate() error }
	for _, tt := range []struct {
		name string
		v    any
		want string // empty: Validate passes
	}{
		{"empty block in a prompt", &PromptRequest{SessionId: "s", Prompt: []ContentBlock{{}}}, "PromptRequest: prompt[0]: ContentBlock must have exactly one variant set"},
		{"empty required string", &ReadTextFileRequest{SessionId: "s"}, ""},
		{"title below minLength", &Notice{}, "Notice: title: shorter than minLength 1"},
		{"map value", &ElicitationSchema{Properties: map[string]ElicitationPropertySchema{"a": {}}},
			`ElicitationSchema: properties["a"]: ElicitationPropertySchema must have exactly one variant set`},
		{"pointer target", &AvailableCommand{Name: "n", Description: "d", Input: &AvailableCommandInput{}},
			"AvailableCommand: input: AvailableCommandInput must have exactly one variant set"},
		{"no variant", &CreateElicitationRequest{}, "CreateElicitationRequest must have exactly one variant set"},
		{"form with an empty message", &CreateElicitationRequest{Form: &CreateElicitationForm{Mode: "form",
			Session: &ElicitationSessionScope{SessionId: "s"}}}, ""},
		{"valid prompt", &PromptRequest{SessionId: "s", Prompt: []ContentBlock{TextBlock("x")}}, ""},
		{"set variant validates", &SessionUpdate{UserMessageChunk: &SessionUpdateUserMessageChunk{}},
			"SessionUpdate: variant UserMessageChunk: SessionUpdateUserMessageChunk: content: ContentBlock must have exactly one variant set"},
	} {
		v, ok := tt.v.(validator)
		if !ok {
			t.Errorf("%s: %T has no Validate", tt.name, tt.v)
			continue
		}
		err := v.Validate()
		switch {
		case tt.want == "" && err != nil:
			t.Errorf("%s: %v, want it to pass", tt.name, err)
		case tt.want != "" && (err == nil || !strings.Contains(err.Error(), tt.want)):
			t.Errorf("%s: %v, want %q", tt.name, err, tt.want)
		}
	}
}
