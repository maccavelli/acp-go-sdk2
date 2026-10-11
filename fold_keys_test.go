package acp

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestFoldKeysMatchesEncodingJSON requires foldKeys to give each name the value
// encoding/json gives the struct field of that name, and to keep only the keys
// no name matches (0011-MADR-harden-code-generator-engine.md, "Found in step 8
// and decided"): keys in other cases, Unicode folds, escaped keys, and several
// matching keys in either order.
func TestFoldKeysMatchesEncodingJSON(t *testing.T) {
	t.Parallel()
	type fields struct {
		Kind json.RawMessage `json:"kind"`
		Id   json.RawMessage `json:"toolCallId"`
		S    json.RawMessage `json:"s"`
	}
	names := []string{"kind", "toolCallId", "s"}
	for _, in := range []string{
		`{}`,
		`{"kind":1,"toolCallId":2,"s":3}`,
		`{"KIND":1,"TOOLCALLID":2,"S":3}`,
		`{"kind":1,"KIND":2}`,
		`{"KIND":2,"kind":1}`,
		`{"Kind":1,"kInD":2,"kind":3}`,
		`{"kind":1,"kind":2}`,
		"{\"Kind\":1}",
		"{\"ſ\":1}",
		`{"Kind":1}`,
		`{"tool_call_id":1,"toolcallid":2}`,
		`{"other":1,"Other":2,"KIND":3}`,
		`{"kind":null,"Kind":1}`,
		`{"Kind":1,"kind":null}`,
	} {
		var want fields
		if err := json.Unmarshal([]byte(in), &want); err != nil {
			t.Fatalf("%s: %v", in, err)
		}
		var m map[string]json.RawMessage
		if err := json.Unmarshal([]byte(in), &m); err != nil {
			t.Fatalf("%s: %v", in, err)
		}
		foldKeys([]byte(in), m, names...)
		for i, got := range []json.RawMessage{m["kind"], m["toolCallId"], m["s"]} {
			w := []json.RawMessage{want.Kind, want.Id, want.S}[i]
			if string(got) != string(w) {
				t.Errorf("%s: %s is %s, encoding/json gives %s", in, names[i], got, w)
			}
		}
		for k := range m {
			for _, n := range names {
				if k != n && strings.EqualFold(k, n) {
					t.Errorf("%s: key %q is kept beside %q", in, k, n)
				}
			}
		}
	}
}
