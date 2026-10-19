package acp

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// TestCatchAllExtraKeepsSpelling requires each union's catch-all that keeps
// undeclared keys in Extra to keep them as received when decoded through its
// union. The union folds its own keys on its map, so a catch-all decoded from
// that map would see a key the union folds under the union's spelling; the
// catch-all decodes from the payload instead (0019-PLAN-idioms-performance.md,
// D2). Each row also pins the keys its union folds that the catch-all does not
// declare: with one, the payload holds it in another case; without one, no key
// can differ, and the payload holds another undeclared key.
func TestCatchAllExtraKeepsSpelling(t *testing.T) {
	t.Parallel()
	folds := unionFolds(t)
	types := map[string]reflect.Type{}
	for _, st := range schemaTypes() {
		types[st.Type] = reflect.TypeOf(st.New()).Elem()
	}
	for _, tt := range []struct {
		union, field string
		// undeclared is the keys the union folds that the catch-all does
		// not declare, sorted.
		undeclared []string
		payload    string
		key, value string
	}{
		{"CreateElicitationRequest", "Other", []string{"elicitationId", "requestedSchema", "url"},
			`{"mode":"x-other","message":"m","URL":"u"}`, "URL", `"u"`},
		{"ElicitationPropertySchema", "Other", []string{"items"}, `{"type":"x-other","ITEMS":1}`, "ITEMS", "1"},
		{"MultiSelectItems", "Other", []string{"anyOf", "enum"}, `{"type":"x-other","ENUM":[1]}`, "ENUM", "[1]"},
		{"CreateElicitationResponse", "Other", nil, `{"action":"x-other","kept":true}`, "kept", "true"},
		{"IdleStateUpdate", "Other", nil, `{"stopReason":"x-other","kept":true}`, "kept", "true"},
		{"StateUpdate", "Other", nil, `{"state":"x-other","kept":true}`, "kept", "true"},
	} {
		union, ok := types[tt.union]
		if !ok {
			t.Errorf("%s: not a schema type", tt.union)
			continue
		}
		f, ok := union.FieldByName(tt.field)
		if !ok {
			t.Errorf("%s: no field %s", tt.union, tt.field)
			continue
		}
		declared := map[string]bool{}
		for i := range f.Type.Elem().NumField() {
			declared[strings.Split(f.Type.Elem().Field(i).Tag.Get("json"), ",")[0]] = true
		}
		var undeclared []string
		for _, k := range folds[tt.union] {
			if !declared[k] {
				undeclared = append(undeclared, k)
			}
		}
		slices.Sort(undeclared)
		if !slices.Equal(undeclared, tt.undeclared) {
			t.Errorf("%s folds %v; of them %s does not declare %v, want %v", tt.union, folds[tt.union], f.Type.Elem().Name(), undeclared, tt.undeclared)
		}

		u := reflect.New(union)
		if err := json.Unmarshal([]byte(tt.payload), u.Interface()); err != nil {
			t.Errorf("%s: %s: %v", tt.union, tt.payload, err)
			continue
		}
		other := u.Elem().FieldByName(tt.field)
		if other.IsNil() {
			t.Errorf("%s: %s did not decode into %s", tt.union, tt.payload, tt.field)
			continue
		}
		extra, _ := other.Elem().FieldByName("Extra").Interface().(map[string]json.RawMessage)
		if got, ok := extra[tt.key]; !ok || string(got) != tt.value {
			t.Errorf("%s: %s kept Extra %v, want %q holding %s as received", tt.union, tt.payload, extra, tt.key, tt.value)
		}
	}
}

// unionFolds returns, by union, the keys its UnmarshalJSON folds on its map,
// read from the foldKeys call in types_gen.go.
func unionFolds(t *testing.T) map[string][]string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "types_gen.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string][]string{}
	for _, d := range file.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "UnmarshalJSON" || fn.Recv == nil || fn.Recv.List[0].Names[0].Name != "u" {
			continue
		}
		recv, ok := fn.Recv.List[0].Type.(*ast.StarExpr)
		if !ok {
			continue
		}
		name, ok := recv.X.(*ast.Ident)
		if !ok {
			continue
		}
		union := name.Name
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			c, ok := n.(*ast.CallExpr)
			if !ok || len(c.Args) < 3 {
				return true
			}
			if name, ok := c.Fun.(*ast.Ident); !ok || name.Name != "foldKeys" {
				return true
			}
			for _, a := range c.Args[2:] {
				if lit, ok := a.(*ast.BasicLit); ok {
					s, err := strconv.Unquote(lit.Value)
					if err != nil {
						t.Fatal(err)
					}
					out[union] = append(out[union], s)
				}
			}
			return true
		})
	}
	if len(out) == 0 {
		t.Fatal("no union in types_gen.go folds a key")
	}
	return out
}
