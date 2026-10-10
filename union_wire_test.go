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
	size := int64(9007199254740993)
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

// TestUnionDecodeReplaces requires a union decoded twice to hold only what
// the second payload carries.
func TestUnionDecodeReplaces(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		first, second string
		into          any
		want          []string
	}{
		{"known then known", `{"type":"text","text":"hi"}`, `{"type":"image","data":"AAAA","mimeType":"image/png"}`, &ContentBlock{}, []string{"Image"}},
		{"unknown then known", `{"type":"zz_future"}`, `{"type":"text","text":"hi"}`, &ContentBlock{}, []string{"Text"}},
		{"without a discriminator", `5`, `"a"`, &RequestId{}, []string{"Str"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			for _, in := range []string{tt.first, tt.second} {
				if err := json.Unmarshal([]byte(in), tt.into); err != nil {
					t.Fatalf("decode %s: %v", in, err)
				}
			}
			if got := setVariants(tt.into); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("variants set: %v, want %v", got, tt.want)
			}
			if u := reflect.ValueOf(tt.into).Elem().FieldByName("Unknown"); u.IsValid() && u.Len() != 0 {
				t.Errorf("Unknown = %s, want empty", u.Bytes())
			}
		})
	}
}

// TestUnionNull requires JSON null to leave a union as it was, except for a
// union with a null variant, which it sets alone.
func TestUnionNull(t *testing.T) {
	t.Parallel()
	block := ContentBlock{Text: &ContentBlockText{Text: "hi"}}
	if err := json.Unmarshal([]byte(`null`), &block); err != nil || block.Text == nil || block.Text.Text != "hi" || len(setVariants(&block)) != 1 {
		t.Errorf("ContentBlock after null: %v, variants %v", err, setVariants(&block))
	}
	res := EmbeddedResourceResource{BlobResourceContents: &BlobResourceContents{Blob: "AAAA", Uri: "u"}}
	if err := json.Unmarshal([]byte(`null`), &res); err != nil || !reflect.DeepEqual(setVariants(&res), []string{"BlobResourceContents"}) {
		t.Errorf("EmbeddedResourceResource after null: %v, variants %v", err, setVariants(&res))
	}
	var n SessionNotification
	if err := json.Unmarshal([]byte(`{"sessionId":"s","update":null}`), &n); err != nil || len(setVariants(&n.Update)) != 0 || n.Update.Unknown != nil {
		t.Errorf("update null: %v, variants %v", err, setVariants(&n.Update))
	}
	num := RequestIdNumber(1)
	id := RequestId{Number: &num}
	if err := json.Unmarshal([]byte(`null`), &id); err != nil || !reflect.DeepEqual(setVariants(&id), []string{"Null"}) {
		t.Errorf("RequestId after null: %v, variants %v", err, setVariants(&id))
	}
}

// TestAbsentDiscriminator requires a payload without its discriminator to
// decode the key-less variant where the union has one, else the one variant
// whose required keys it carries, else to be kept in Unknown or, in a union
// with a catch-all, refused.
func TestAbsentDiscriminator(t *testing.T) {
	t.Parallel()
	const unknown = "<Unknown>"
	tests := []struct {
		name   string
		into   func() any
		in     string
		want   string // a variant field, unknown, or an error text
		errors bool
	}{
		{"IdleStateUpdate empty", func() any { return &IdleStateUpdate{} }, `{}`, "None", false},
		{"IdleStateUpdate null key", func() any { return &IdleStateUpdate{} }, `{"stopReason":null}`, "None", false},
		{"AuthMethod agent", func() any { return &AuthMethod{} }, `{"id":"a","name":"n"}`, "Agent", false},
		{"AuthMethod agent missing a required key", func() any { return &AuthMethod{} }, `{"id":"a"}`, `AuthMethod: missing discriminator "type"`, true},
		{"AuthMethod bad agent", func() any { return &AuthMethod{} }, `{"id":5}`, `AuthMethod: missing discriminator "type"`, true},
		{"McpServer stdio", func() any { return &McpServer{} }, `{"name":"n","command":"c","args":[],"env":[]}`, "Stdio", false},
		{"UnstableMcpServer stdio", func() any { return &UnstableMcpServer{} }, `{"name":"n","command":"c","args":[],"env":[]}`, "Stdio", false},
		{"SetSessionConfigOptionRequest value id", func() any { return &SetSessionConfigOptionRequest{} },
			`{"sessionId":"s","configId":"c","value":"v"}`, "ValueId", false},
		{"MultiSelectItems titled", func() any { return &MultiSelectItems{} }, `{"anyOf":[]}`, "Titled", false},
		{"RequestPermissionOutcome option only", func() any { return &RequestPermissionOutcome{} }, `{"optionId":"allow-once"}`, unknown, false},
		{"RequestPermissionOutcome empty", func() any { return &RequestPermissionOutcome{} }, `{}`, "Cancelled", false},
		{"CreateElicitationResponse empty", func() any { return &CreateElicitationResponse{} }, `{}`,
			`CreateElicitationResponse: missing discriminator "action"`, true},
		{"CreateElicitationRequest form", func() any { return &CreateElicitationRequest{} }, `{"requestedSchema":{},"message":"m"}`, "Form", false},
		{"StateUpdate empty", func() any { return &StateUpdate{} }, `{}`, `StateUpdate: missing discriminator "state"`, true},
		{"SessionUpdate content", func() any { return &SessionUpdate{} }, `{"content":{"type":"text","text":"hi"}}`, unknown, false},
		{"SessionUpdate tool call update", func() any { return &SessionUpdate{} }, `{"toolCallId":"t","status":"completed"}`, unknown, false},
		{"SessionUpdate empty", func() any { return &SessionUpdate{} }, `{}`, "SessionInfoUpdate", false},
		{"ContentBlock text", func() any { return &ContentBlock{} }, `{"text":"hi"}`, "Text", false},
		{"ContentBlock bad text", func() any { return &ContentBlock{} }, `{"text":5}`, "ContentBlock: variant Text", true},
		{"ContentBlock audio", func() any { return &ContentBlock{} }, `{"data":"AAAA","mimeType":"audio/wav"}`, unknown, false},
		{"ContentBlock empty", func() any { return &ContentBlock{} }, `{}`, unknown, false},
		{"ContentBlock not an object", func() any { return &ContentBlock{} }, `5`, "ContentBlock: json: cannot unmarshal", true},
		{"ToolCallContent diff", func() any { return &ToolCallContent{} }, `{"path":"p","newText":"n"}`, "Diff", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			u := tt.into()
			err := json.Unmarshal([]byte(tt.in), u)
			if tt.errors {
				if err == nil || !strings.Contains(err.Error(), tt.want) {
					t.Fatalf("error %v, want one containing %q; variants set: %v", err, tt.want, setVariants(u))
				}
				return
			}
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			got := setVariants(u)
			kept := reflect.ValueOf(u).Elem().FieldByName("Unknown")
			if tt.want == unknown {
				if len(got) != 0 || !kept.IsValid() || string(kept.Bytes()) != tt.in {
					t.Errorf("variants set: %v; want only Unknown = %s", got, tt.in)
				}
				return
			}
			if !reflect.DeepEqual(got, []string{tt.want}) || (kept.IsValid() && kept.Len() != 0) {
				t.Errorf("variants set: %v, want [%s]", got, tt.want)
			}
		})
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
