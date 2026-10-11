package emit

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"slices"
	"strconv"
	"testing"

	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/load"
	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/model"
)

// registryModel has a type of each kind, a union with a discriminator and a
// catch-all, one with an Unknown field, an agent request, an agent
// notification, a client request without a result, and a protocol method;
// one type comes from the unstable schema under another name.
func registryModel(t *testing.T) *model.Model {
	t.Helper()
	var schema load.Schema
	if err := json.Unmarshal([]byte(`{"$defs":{
		"Color":{"type":"string","enum":["red"]},
		"Mode":{"oneOf":[{"type":"string","const":"ask"}]},
		"Level":{"anyOf":[{"type":"string","const":"low"},{"type":"string"}]},
		"Count":{"type":"integer","format":"uint32"},
		"Empty":{"type":"object"},
		"Anything":{},
		"Nothing":{"type":"null"},
		"Thing":{"type":"object","properties":{"n":{"type":"string"}}},
		"Open":{"anyOf":[{"type":"object","required":["type"],"properties":{"type":{"type":"string","const":"a"}}},
			{"title":"other","type":"object","required":["type"],"properties":{"type":{"type":"string"}},"not":{"required":["type"]}}]},
		"Closed":{"oneOf":[{"type":"object","required":["kind"],"properties":{"kind":{"type":"string","const":"x"}}},
			{"type":"object","required":["kind"],"properties":{"kind":{"type":"string","const":"y"}}}]},
		"InitRequest":{"type":"object","x-side":"agent","x-method":"init"},
		"InitResponse":{"type":"object","x-side":"agent","x-method":"init"},
		"PingNotification":{"type":"object","x-side":"agent","x-method":"ping"},
		"WriteRequest":{"type":"object","x-side":"client","x-method":"write"},
		"WriteResponse":{"type":"null","x-side":"client","x-method":"write"},
		"CancelRequestNotification":{"type":"object","x-side":"protocol","x-method":"$/cancel_request"},
		"UnstableFresh":{"type":"string"}}}`), &schema); err != nil {
		t.Fatal(err)
	}
	m, err := model.Build(model.Input{
		Meta: &load.Meta{AgentMethods: map[string]string{"init": "init", "ping": "ping"}, ClientMethods: map[string]string{"write": "write"},
			ProtocolMethods: map[string]string{"cancel_request": "$/cancel_request"}},
		Schema:     &schema,
		Provenance: &load.Provenance{Origins: map[string]load.Origin{"UnstableFresh": {File: load.UnstableSchemaFile, Key: "Fresh"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// registryEntries returns, for each element of the slice the named function
// of src returns, the string and bool values of its keyed fields.
func registryEntries(t *testing.T, src, fn string) []map[string]string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "schema_gen_test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	var out []map[string]string
	for _, d := range file.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd.Name.Name != fn {
			continue
		}
		lit := fd.Body.List[0].(*ast.ReturnStmt).Results[0].(*ast.CompositeLit)
		for _, e := range lit.Elts {
			entry := map[string]string{}
			for _, kvx := range e.(*ast.CompositeLit).Elts {
				kv := kvx.(*ast.KeyValueExpr)
				key := kv.Key.(*ast.Ident).Name
				switch v := kv.Value.(type) {
				case *ast.BasicLit:
					entry[key], _ = strconv.Unquote(v.Value)
				case *ast.Ident:
					entry[key] = v.Name
				case *ast.IndexExpr:
					entry[key] = v.Index.(*ast.Ident).Name
				case *ast.CompositeLit:
					entry[key] = strconv.Itoa(len(v.Elts))
				}
			}
			out = append(out, entry)
		}
		return out
	}
	t.Fatalf("schema_gen_test.go has no func %s", fn)
	return nil
}

// methodsOf returns the names of the methods declared on recv in src.
func methodsOf(t *testing.T, src, recv string) []string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "schema_gen_test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, d := range file.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Recv != nil && fd.Recv.List[0].Type.(*ast.Ident).Name == recv {
			out = append(out, fd.Name.Name)
		}
	}
	return out
}

func TestRegistry(t *testing.T) {
	t.Parallel()
	m := registryModel(t)
	src := rendered(t, m)["schema_gen_test.go"]
	if src == "" {
		t.Fatal("Render wrote no schema_gen_test.go")
	}

	var types []string
	for _, e := range registryEntries(t, src, "schemaTypes") {
		types = append(types, e["Type"]+"="+e["File"]+"#"+e["Def"]+":"+e["New"])
	}
	var want []string
	for _, ty := range m.Types {
		if ty.Kind == model.KindNone {
			continue
		}
		want = append(want, ty.Name+"="+ty.Origin.File+"#"+ty.Origin.Key+":"+ty.Name)
	}
	if !slices.Equal(types, want) {
		t.Errorf("schemaTypes:\n got %v\nwant %v", types, want)
	}
	if !slices.Contains(types, "UnstableFresh=schema.unstable.json#Fresh:UnstableFresh") {
		t.Errorf("schemaTypes lacks the unstable origin: %v", types)
	}

	unions := map[string]map[string]string{}
	for _, e := range registryEntries(t, src, "schemaUnions") {
		unions[e["Type"]] = e
	}
	if u := unions["Open"]; u == nil || u["Key"] != "type" || u["CatchAll"] != "Other" || u["Unknown"] != "false" || u["Variants"] != "2" {
		t.Errorf("Open: %v", u)
	}
	if u := unions["Closed"]; u == nil || u["Key"] != "kind" || u["CatchAll"] != "" || u["Unknown"] != "true" || u["Variants"] != "2" {
		t.Errorf("Closed: %v", u)
	}
	if len(unions) != 2 {
		t.Errorf("schemaUnions holds %d unions, want Open and Closed", len(unions))
	}

	var methods []string
	for _, e := range registryEntries(t, src, "schemaMethods") {
		methods = append(methods, e["Side"]+" "+e["Wire"]+" "+e["Params"]+" "+e["Result"]+" "+e["Notification"])
	}
	wantMethods := []string{
		"agent init InitRequest InitResponse false",
		"agent ping PingNotification  true",
		"client write WriteRequest  false",
		"protocol $/cancel_request CancelRequestNotification  true",
	}
	if !slices.Equal(methods, wantMethods) {
		t.Errorf("schemaMethods:\n got %q\nwant %q", methods, wantMethods)
	}
	if got := methodsOf(t, src, "recordingAgent"); !slices.Equal(got, []string{"Init", "Ping"}) {
		t.Errorf("recordingAgent methods %v, want Init and Ping", got)
	}
	if got := methodsOf(t, src, "recordingClient"); !slices.Equal(got, []string{"Write"}) {
		t.Errorf("recordingClient methods %v, want Write", got)
	}
	for _, in := range []string{"Agent", "AgentExperimental", "Client", "ClientExperimental"} {
		if !regexp.MustCompile(`_\s+` + in + `\s+= recording`).MatchString(src) {
			t.Errorf("no assertion that a recorder implements %s", in)
		}
	}
}

// TestRegistryEdges requires a null definition to leave the types after it in
// schemaTypes, and a catch-all that is a union's first variant to be recorded
// as its catch-all. Found by the mutation run of 0011-PLAN-engine-tests.md,
// step 9.
func TestRegistryEdges(t *testing.T) {
	t.Parallel()
	var schema load.Schema
	if err := json.Unmarshal([]byte(`{"$defs":{
		"A":{"type":"string","enum":["a"]},
		"N":null,
		"Mark":{"anyOf":[
			{"title":"other","type":"object","properties":{"kind":{"type":"string"}},"required":["kind"],"not":{"anyOf":[{"type":"object","properties":{"kind":{"type":"string","const":"a"}},"required":["kind"]}]}},
			{"type":"object","properties":{"kind":{"type":"string","const":"a"}},"required":["kind"]}]},
		"Z":{"type":"string","enum":["z"]}}}`), &schema); err != nil {
		t.Fatal(err)
	}
	m, err := model.Build(model.Input{Meta: &load.Meta{}, Schema: &schema})
	if err != nil {
		t.Fatal(err)
	}
	src := rendered(t, m)["schema_gen_test.go"]
	var types []string
	for _, e := range registryEntries(t, src, "schemaTypes") {
		types = append(types, e["Type"])
	}
	if !slices.Contains(types, "Z") || !slices.Contains(types, "Mark") || slices.Contains(types, "N") {
		t.Errorf("schemaTypes %v, want every type but N", types)
	}
	unions := registryEntries(t, src, "schemaUnions")
	if len(unions) != 1 || unions[0]["Type"] != "Mark" || unions[0]["CatchAll"] != "Other" {
		t.Errorf("schemaUnions %v, want Mark with the catch-all Other", unions)
	}
}
