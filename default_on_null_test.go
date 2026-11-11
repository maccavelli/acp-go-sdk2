package acp

import (
	"encoding/json"
	"maps"
	"slices"
	"testing"
)

// defaultOnNullSites is how many properties of the stable schema carry
// x-deserialize-default-on-null at schema 1.31.0: args and env of
// AuthMethodTerminal, CreateTerminalRequest and McpServerStdio; headers of
// McpServerHttp and McpServerSse; and additionalDirectories and mcpServers of
// the new-, load- and resume-session requests
// (0054-MADR-adopt-acp-schema-1-31-0-and-client-system-prompts.md, F1).
const defaultOnNullSites = 14

// TestDefaultOnNullLists requires each property of schema.json marked
// x-deserialize-default-on-null to decode null as its empty value: no error,
// Validate passes, and the encoding holds the empty array or object when the
// property is required and leaves it out when it is optional. The generator
// reads the keyword and emits nothing for it, since the decoders already
// behave so; this test is what holds them to it
// (0054-MADR-adopt-acp-schema-1-31-0-and-client-system-prompts.md, D2).
func TestDefaultOnNullLists(t *testing.T) {
	t.Parallel()
	types := map[string]schemaType{}
	for _, st := range schemaTypes() {
		if st.Type == st.Def {
			types[st.Def] = st
		}
	}
	stable := schemaDefs()["schema.json"]
	sites := 0
	for _, def := range slices.Sorted(maps.Keys(stable)) {
		node, _ := stable[def].(schemaNode)
		props, _ := node["properties"].(schemaNode)
		for _, key := range sortedKeys(props) {
			if p, _ := props[key].(schemaNode); p["x-deserialize-default-on-null"] != true {
				continue
			}
			sites++
			t.Run(def+"."+key, func(t *testing.T) {
				st, ok := types[def]
				if !ok {
					t.Fatalf("no generated type is built from %s", def)
				}
				b := newBuilder(st.File)
				n := b.resolve(b.def(def))
				inst, ok := b.value(n, 0, instanceShape{full: true}).(schemaNode)
				if !ok {
					t.Fatalf("the instance of %s is not an object", def)
				}
				in := maps.Clone(inst)
				in[key] = nil
				raw := mustJSON(t, in)
				v := st.New()
				if err := json.Unmarshal(raw, v); err != nil {
					t.Fatalf("%s did not decode: %v", raw, err)
				}
				if val, ok := v.(interface{ Validate() error }); ok {
					if err := val.Validate(); err != nil {
						t.Errorf("%s: Validate = %v, want nil", raw, err)
					}
				}
				sent := mustJSON(t, v)
				out, _ := decodeTree(t, sent).(map[string]any)
				got, has := out[key]
				if slices.Contains(requiredKeys(n), key) {
					list, isList := got.([]any)
					object, isObject := got.(map[string]any)
					if !has || (!isList && !isObject) || len(list) != 0 || len(object) != 0 {
						t.Errorf("%s sent as %s: required %s is %v, want an empty array or object", raw, sent, key, got)
					}
				} else if has {
					t.Errorf("%s sent as %s: optional %s is %v, want it left out", raw, sent, key, got)
				}
			})
		}
	}
	if sites != defaultOnNullSites {
		t.Errorf("%d properties of schema.json carry x-deserialize-default-on-null, want %d", sites, defaultOnNullSites)
	}
}
