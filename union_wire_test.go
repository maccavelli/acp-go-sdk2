package acp

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// These tests cover how the generated unions read and write the wire: see
// docs/decisions/0011-PLAN-wire-fixes.md.

// TestUnionDecodeErrorsNameTheirPlace requires a union decode failure to name
// the union and, for a variant, the variant and the underlying cause.
func TestUnionDecodeErrorsNameTheirPlace(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   string
		into any
		want []string
	}{
		{"known variant with a bad payload", `{"type":"text","text":5}`, &ContentBlock{}, []string{"ContentBlock", `type "text"`, "cannot unmarshal number"}},
		{"variant found by its keys, with a bad payload", `{"uri":"u","text":5}`, &EmbeddedResourceResource{},
			[]string{"EmbeddedResourceResource: variant TextResourceContents", "cannot unmarshal number"}},
		{"no variant matches", `true`, &RequestId{}, []string{"RequestId: no variant matches"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := json.Unmarshal([]byte(tt.in), tt.into)
			if err == nil {
				t.Fatalf("decoded %s", tt.in)
			}
			for _, w := range tt.want {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("error %q does not contain %q", err, w)
				}
			}
		})
	}
}

// TestUnionKeepsIntegerDigits requires an integer above 2^53 to keep every
// digit through a union, alone and nested in another union.
func TestUnionKeepsIntegerDigits(t *testing.T) {
	t.Parallel()
	const digits = "9007199254740993"
	size := 9007199254740993
	block := ContentBlock{ResourceLink: &ContentBlockResourceLink{Name: "n", Uri: "u", Size: &size}}
	for name, v := range map[string]any{
		"ContentBlock":  block,
		"SessionUpdate": SessionUpdate{AgentMessageChunk: &SessionUpdateAgentMessageChunk{Content: block}},
	} {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(b), `"size":`+digits) {
			t.Errorf("%s: %s does not carry size %s", name, b, digits)
		}
	}
	b, err := json.Marshal(block)
	if err != nil {
		t.Fatal(err)
	}
	var back ContentBlock
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if back.ResourceLink == nil || back.ResourceLink.Size == nil || *back.ResourceLink.Size != size {
		t.Errorf("decoded %s, want size %d", b, size)
	}
}

// unknownUnions are the discriminated unions without a schema catch-all,
// with their discriminator: 7 closed and 4 defaulted.
var unknownUnions = []struct {
	name   string
	newPtr func() any
	key    string
	closed bool
}{
	{"ContentBlock", func() any { return &ContentBlock{} }, "type", true},
	{"PlanUpdateContent", func() any { return &PlanUpdateContent{} }, "type", true},
	{"RequestPermissionOutcome", func() any { return &RequestPermissionOutcome{} }, "outcome", true},
	{"SessionConfigOption", func() any { return &SessionConfigOption{} }, "type", true},
	{"SessionUpdate", func() any { return &SessionUpdate{} }, "sessionUpdate", true},
	{"ToolCallContent", func() any { return &ToolCallContent{} }, "type", true},
	{"UnstableNesSuggestion", func() any { return &UnstableNesSuggestion{} }, "kind", true},
	{"AuthMethod", func() any { return &AuthMethod{} }, "type", false},
	{"McpServer", func() any { return &McpServer{} }, "type", false},
	{"SetSessionConfigOptionRequest", func() any { return &SetSessionConfigOptionRequest{} }, "type", false},
	{"UnstableMcpServer", func() any { return &UnstableMcpServer{} }, "type", false},
}

// setVariants returns the names of a union's non-nil pointer fields.
func setVariants(u any) []string {
	rv := reflect.ValueOf(u).Elem()
	var out []string
	for i := range rv.NumField() {
		if f := rv.Field(i); f.Kind() == reflect.Pointer && !f.IsNil() {
			out = append(out, rv.Type().Field(i).Name)
		}
	}
	return out
}

// TestUnknownValueKept requires a discriminator value no variant knows to be
// kept in Unknown, without error, and encoded back unchanged.
func TestUnknownValueKept(t *testing.T) {
	t.Parallel()
	for _, tt := range unknownUnions {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			in := `{"` + tt.key + `":"zz_future","extra":1}`
			u := tt.newPtr()
			buf := []byte(in)
			if err := json.Unmarshal(buf, u); err != nil {
				t.Fatalf("decode: %v", err)
			}
			// Unknown must be a copy: overwriting the input must not change it.
			for i := range buf {
				buf[i] = 'X'
			}
			unknown := reflect.ValueOf(u).Elem().FieldByName("Unknown")
			if !unknown.IsValid() {
				t.Fatalf("%s has no Unknown field; variants set: %v", tt.name, setVariants(u))
			}
			if got := setVariants(u); len(got) != 0 {
				t.Errorf("variants set: %v", got)
			}
			if got := string(unknown.Bytes()); got != in {
				t.Errorf("Unknown = %s, want %s", got, in)
			}
			b, err := json.Marshal(reflect.ValueOf(u).Elem().Interface())
			if err != nil || string(b) != in {
				t.Errorf("encoded %s, %v; want %s", b, err, in)
			}
			if !tt.closed {
				return
			}
			validate := func() error { return u.(interface{ Validate() error }).Validate() }
			if err := validate(); err != nil {
				t.Errorf("Validate with only Unknown: %v", err)
			}
			first := reflect.ValueOf(u).Elem().Field(0)
			first.Set(reflect.New(first.Type().Elem()))
			if err := validate(); err == nil {
				t.Error("Validate passed with Unknown and a variant set")
			}
		})
	}
}

// catchAllUnions are the unions with a schema catch-all, with their
// discriminator, its Go field, and the other keys the catch-all requires.
var catchAllUnions = []struct {
	name   string
	newPtr func() any
	key    string
	field  string
	more   string
}{
	{"CreateElicitationRequest", func() any { return &CreateElicitationRequest{} }, "mode", "Mode", `,"message":"m"`},
	{"CreateElicitationResponse", func() any { return &CreateElicitationResponse{} }, "action", "Action", ""},
	{"ElicitationPropertySchema", func() any { return &ElicitationPropertySchema{} }, "type", "Type", ""},
	{"IdleStateUpdate", func() any { return &IdleStateUpdate{} }, "stopReason", "StopReason", ""},
	{"MultiSelectItems", func() any { return &MultiSelectItems{} }, "type", "Type", ""},
	{"StateUpdate", func() any { return &StateUpdate{} }, "state", "State", ""},
}

// sameJSON reports whether a and b encode the same JSON value.
func sameJSON(t *testing.T, a, b []byte) bool {
	t.Helper()
	var x, y any
	if err := json.Unmarshal(a, &x); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &y); err != nil {
		t.Fatal(err)
	}
	return reflect.DeepEqual(x, y)
}

// TestUnknownValueToCatchAll requires a value no listed variant takes to land
// in the schema's catch-all, which keeps the keys it does not declare and
// encodes the payload back unchanged.
func TestUnknownValueToCatchAll(t *testing.T) {
	t.Parallel()
	for _, tt := range catchAllUnions {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			in := `{"` + tt.key + `":"zz_future","extra":{"n":9007199254740993}` + tt.more + `}`
			u := tt.newPtr()
			if err := json.Unmarshal([]byte(in), u); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if got := setVariants(u); len(got) != 1 || got[0] != "Other" {
				t.Fatalf("variants set: %v, want [Other]", got)
			}
			other := reflect.ValueOf(u).Elem().FieldByName("Other").Elem()
			if got := other.FieldByName(tt.field).String(); got != "zz_future" {
				t.Errorf("Other.%s = %q", tt.field, got)
			}
			extra := other.FieldByName("Extra")
			if !extra.IsValid() {
				t.Fatalf("%s has no Extra field", other.Type())
			}
			kept := extra.Interface().(map[string]json.RawMessage)
			if len(kept) != 1 || string(kept["extra"]) != `{"n":9007199254740993}` {
				t.Errorf("Extra = %v, want only extra", kept)
			}
			b, err := json.Marshal(reflect.ValueOf(u).Elem().Interface())
			if err != nil || !sameJSON(t, b, []byte(in)) || !strings.Contains(string(b), "9007199254740993") {
				t.Errorf("encoded %s, %v; want %s", b, err, in)
			}
		})
	}
	var r CreateElicitationResponse
	err := json.Unmarshal([]byte(`{"action":"accept","content":5}`), &r)
	if err == nil || !strings.Contains(err.Error(), `CreateElicitationResponse: action "accept"`) {
		t.Errorf("known value with a bad payload: %v", err)
	}
}

// TestContentBlockKeepsMetaAndAnnotations requires a content block to encode
// its _meta and annotations, whether built in Go or decoded from the wire.
func TestContentBlockKeepsMetaAndAnnotations(t *testing.T) {
	t.Parallel()
	prio := 0.5
	built := ContentBlock{Text: &ContentBlockText{Text: "hi", Meta: map[string]any{"k": "v"}, Annotations: &Annotations{Priority: &prio}}}
	var decoded ContentBlock
	if err := json.Unmarshal([]byte(`{"type":"image","data":"AAAA","mimeType":"image/png","_meta":{"k":"v"},"annotations":{"priority":0.5}}`), &decoded); err != nil {
		t.Fatal(err)
	}
	for name, v := range map[string]ContentBlock{"built text block": built, "decoded image block": decoded} {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{`"_meta":{"k":"v"}`, `"annotations":{"priority":0.5}`} {
			if !strings.Contains(string(b), want) {
				t.Errorf("%s: %s lacks %s", name, b, want)
			}
		}
	}
}
