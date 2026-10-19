package acp

import (
	"encoding/json"
	"reflect"
	"testing"
)

// TestSchemaDefaultsMatchTheirJSON requires each schema default the generated
// code writes as a Go value to equal what decoding the default's JSON into a
// new value of the field's type gives, so the generator cannot render a
// default other than the one the schema states
// (0019-PLAN-idioms-performance.md, D3).
func TestSchemaDefaultsMatchTheirJSON(t *testing.T) {
	t.Parallel()
	defaults := schemaDefaults()
	if len(defaults) == 0 {
		t.Fatal("schemaDefaults lists no default")
	}
	for _, d := range defaults {
		p := d.New()
		if err := json.Unmarshal([]byte(d.JSON), p); err != nil {
			t.Errorf("%s.%s: default %s does not decode: %v", d.Type, d.Field, d.JSON, err)
			continue
		}
		if got, want := reflect.ValueOf(p).Elem().Interface(), d.Value(); !reflect.DeepEqual(got, want) {
			t.Errorf("%s.%s: default %s decodes to %#v; the generated code writes %#v", d.Type, d.Field, d.JSON, got, want)
		}
	}
}
