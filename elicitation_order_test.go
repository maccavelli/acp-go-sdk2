package acp

import (
	"encoding/json"
	"slices"
	"testing"
)

// field is an elicitation property schema as the tests send it.
func field(title string) string { return `{"type":"string","title":"` + title + `"}` }

// sent is the same property as the SDK encodes it: keys sorted.
func sent(title string) string { return `{"title":"` + title + `","type":"string"}` }

// TestElicitationSchemaKeepsPropertyOrder requires a form's properties to keep
// the order the agent wrote them in, read into PropertyOrder and written back,
// under either JSON build; an order set by hand to be written as named, missing
// names skipped and unnamed keys sorted after; and a schema without an order to
// encode exactly as before (0022-PLAN-keep-elicitation-property-order-before-v1-15-0.md).
func TestElicitationSchemaKeepsPropertyOrder(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name, in  string
		wantOrder []string
		wantOut   string
	}{
		{"the agent's order", `{"type":"object","properties":{"name":` + field("N") + `,"email":` + field("E") + `,"age":` + field("A") + `}}`,
			[]string{"name", "email", "age"},
			`{"properties":{"name":` + sent("N") + `,"email":` + sent("E") + `,"age":` + sent("A") + `},"type":"object"}`},
		{"reversed", `{"properties":{"c":` + field("C") + `,"b":` + field("B") + `,"a":` + field("A") + `}}`,
			[]string{"c", "b", "a"},
			`{"properties":{"c":` + sent("C") + `,"b":` + sent("B") + `,"a":` + sent("A") + `},"type":"object"}`},
		{"a duplicate key keeps its first place and its last value",
			`{"properties":{"a":` + field("first") + `,"b":` + field("B") + `,"a":` + field("last") + `}}`,
			[]string{"a", "b"},
			`{"properties":{"a":` + sent("last") + `,"b":` + sent("B") + `},"type":"object"}`},
		{"absent properties", `{"type":"object"}`, nil, `{"properties":{},"type":"object"}`},
		{"null properties", `{"properties":null}`, nil, `{"properties":{},"type":"object"}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var s ElicitationSchema
			if err := json.Unmarshal([]byte(tt.in), &s); err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(s.PropertyOrder, tt.wantOrder) {
				t.Errorf("PropertyOrder = %q, want %q", s.PropertyOrder, tt.wantOrder)
			}
			out, err := json.Marshal(s)
			if err != nil {
				t.Fatal(err)
			}
			if string(out) != tt.wantOut {
				t.Errorf("sent as\n %s\nwant\n %s", out, tt.wantOut)
			}
		})
	}

	props := func() map[string]ElicitationPropertySchema {
		var m map[string]ElicitationPropertySchema
		if err := json.Unmarshal([]byte(`{"a":`+field("A")+`,"b":`+field("B")+`,"c":`+field("C")+`}`), &m); err != nil {
			t.Fatal(err)
		}
		return m
	}
	for _, tt := range []struct {
		name  string
		order []string
		want  string
	}{
		{"no order: sorted, as before", nil, `{"properties":{"a":` + sent("A") + `,"b":` + sent("B") + `,"c":` + sent("C") + `}}`},
		{"an order that names a missing key and leaves one out, with a repeat", []string{"c", "missing", "a", "c"},
			`{"properties":{"c":` + sent("C") + `,"a":` + sent("A") + `,"b":` + sent("B") + `}}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			out, err := json.Marshal(ElicitationSchema{Properties: props(), PropertyOrder: tt.order})
			if err != nil {
				t.Fatal(err)
			}
			if string(out) != tt.want {
				t.Errorf("sent as\n %s\nwant\n %s", out, tt.want)
			}
		})
	}

	t.Run("inside a request", func(t *testing.T) {
		t.Parallel()
		wire := `{"sessionId":"s","mode":"form","message":"m","requestedSchema":{"properties":{"name":` + field("N") +
			`,"email":` + field("E") + `}}}`
		var r CreateElicitationRequest
		if err := json.Unmarshal([]byte(wire), &r); err != nil {
			t.Fatal(err)
		}
		out, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		want := `{"message":"m","mode":"form","requestedSchema":{"properties":{"name":` + sent("N") + `,"email":` + sent("E") +
			`},"type":"object"},"sessionId":"s"}`
		if string(out) != want {
			t.Errorf("sent as\n %s\nwant\n %s", out, want)
		}
	})
}
