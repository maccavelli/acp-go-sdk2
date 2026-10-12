package acp

import (
	"bytes"
	"encoding/json"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
)

// This file builds JSON instances of a schema definition from the schema files
// themselves, apart from the generator, for the conformance test
// (0011-PLAN-engine-tests.md, D3). It reads only the definition's file and key,
// which schema_gen_test.go gives.

// schemaNode is a JSON Schema node as encoding/json decodes it, numbers as
// json.Number.
type schemaNode = map[string]any

// schemaDefs returns the $defs of a file of schema/, read once.
var schemaDefs = sync.OnceValue(func() map[string]map[string]any {
	out := map[string]map[string]any{}
	for _, file := range []string{"schema.json", "schema.unstable.json"} {
		b, err := os.ReadFile(filepath.Join("schema", file))
		if err != nil {
			panic(err)
		}
		d := json.NewDecoder(bytes.NewReader(b))
		d.UseNumber()
		var doc struct {
			Defs map[string]any `json:"$defs"`
		}
		if err := d.Decode(&doc); err != nil {
			panic(err)
		}
		out[file] = doc.Defs
	}
	return out
})

// instanceShape is which instance of a node to build: with the required
// properties only or with every property, and with the first or the last
// choice of each enum, union, and range.
type instanceShape struct{ full, last bool }

// shapes are the instances built for each definition.
var shapes = []instanceShape{{false, false}, {true, false}, {true, true}}

// maxDepth is how deep instances nest optional properties; below it, only
// required ones are built, so recursive definitions end.
const maxDepth = 3

// builder builds instances of the nodes of one schema file. abs holds the
// property nodes the protocol requires to be absolute paths, by identity,
// which merge keeps, so a property reached through allOf is found too.
type builder struct {
	defs map[string]any
	abs  map[uintptr]bool
}

func newBuilder(file string) builder {
	b := builder{defs: schemaDefs()[file], abs: map[uintptr]bool{}}
	// A definition with the rule's key in this file is the same definition the
	// other file carries, as the merge treats it, so the rule applies here too.
	for _, ap := range schemaAbsolutePaths() {
		props, _ := b.def(ap.Def)["properties"].(schemaNode)
		if n, ok := props[ap.Property].(schemaNode); ok {
			b.abs[reflect.ValueOf(n).Pointer()] = true
		}
	}
	return b
}

// instancePath is the absolute path an instance gives a property the protocol
// requires to be one: absolute on every host
// (0015-PLAN-validate-absolute-protocol-paths.md, step 4).
var instancePath = filepath.Join(os.TempDir(), "acp")

func (b builder) def(name string) schemaNode {
	n, _ := b.defs[name].(schemaNode)
	return n
}

// resolve follows $ref and merges an allOf of one schema into the node, as
// the ACP schema uses them: a node's own keys win, its properties and required
// keys are added.
func (b builder) resolve(n schemaNode) schemaNode {
	for {
		switch {
		case n["$ref"] != nil:
			target := b.def(strings.TrimPrefix(n["$ref"].(string), "#/$defs/"))
			n = merge(target, without(n, "$ref"))
		case n["allOf"] != nil:
			parts := n["allOf"].([]any)
			n = merge(b.resolve(parts[0].(schemaNode)), without(n, "allOf"))
		default:
			return n
		}
	}
}

// merge returns base with over's keys, properties and required keys added.
func merge(base, over schemaNode) schemaNode {
	out := schemaNode{}
	for k, v := range base {
		out[k] = v
	}
	for k, v := range over {
		switch k {
		case "properties":
			props := schemaNode{}
			if bp, ok := base["properties"].(schemaNode); ok {
				for pk, pv := range bp {
					props[pk] = pv
				}
			}
			for pk, pv := range v.(schemaNode) {
				props[pk] = pv
			}
			out[k] = props
		case "required":
			req, _ := base["required"].([]any)
			for _, r := range v.([]any) {
				if !slices.Contains(req, r) {
					req = append(slices.Clone(req), r)
				}
			}
			out[k] = req
		default:
			out[k] = v
		}
	}
	return out
}

func without(n schemaNode, key string) schemaNode {
	out := schemaNode{}
	for k, v := range n {
		if k != key {
			out[k] = v
		}
	}
	return out
}

// members returns a union node's members other than {"type":"null"}, each
// with the union's own properties and required keys merged in; a member's own
// property wins over the union's.
func (b builder) members(n schemaNode) []schemaNode {
	list, _ := n["oneOf"].([]any)
	if list == nil {
		list, _ = n["anyOf"].([]any)
	}
	shared := schemaNode{}
	if p, ok := n["properties"]; ok {
		shared["properties"] = p
	}
	if r, ok := n["required"]; ok {
		shared["required"] = r
	}
	var out []schemaNode
	for _, m := range list {
		m := b.resolve(m.(schemaNode))
		if primaryType(m) == "null" && len(m) <= 3 {
			continue
		}
		out = append(out, merge(shared, m))
	}
	return out
}

// isUnion reports whether n is an anyOf or oneOf the generator makes a union
// or an enum of.
func isUnion(n schemaNode) bool { return n["anyOf"] != nil || n["oneOf"] != nil }

// primaryType is the first type of n other than null, or "".
func primaryType(n schemaNode) string {
	switch t := n["type"].(type) {
	case string:
		return t
	case []any:
		for _, x := range t {
			if x != "null" {
				return x.(string)
			}
		}
		return "null"
	}
	return ""
}

// value builds an instance of n.
func (b builder) value(n schemaNode, depth int, s instanceShape) any {
	n = b.resolve(n)
	if c, ok := n["const"]; ok {
		return c
	}
	if e, ok := n["enum"].([]any); ok {
		return pick(e, s.last)
	}
	if isUnion(n) {
		ms := b.members(n)
		if len(ms) == 0 {
			return nil
		}
		return b.value(pick(ms, s.last), depth, s)
	}
	switch primaryType(n) {
	case "object":
		return b.object(n, depth, s)
	case "array":
		items, _ := n["items"].(schemaNode)
		if items == nil {
			return []any{}
		}
		return []any{b.value(items, depth+1, s)}
	case "string":
		return strings.Repeat("s", max(1, intKey(n, "minLength")))
	case "integer":
		lo, hi := integerRange(n)
		if s.last {
			return json.Number(hi.String())
		}
		return json.Number(lo.String())
	case "number":
		if s.last {
			if m, ok := n["maximum"].(json.Number); ok {
				return m
			}
			return json.Number("1.5")
		}
		if m, ok := n["minimum"].(json.Number); ok {
			return m
		}
		return json.Number("0.5")
	case "boolean":
		return !s.last
	case "null":
		return nil
	default:
		// No Go type: raw JSON, with an integer beyond 2^53.
		return schemaNode{"raw": json.Number("9007199254740993")}
	}
}

func pick[T any](list []T, last bool) T {
	if last {
		return list[len(list)-1]
	}
	return list[0]
}

// object builds an object instance: the required properties, and every
// property when s.full and the depth allows; a typed additionalProperties
// gives one entry.
func (b builder) object(n schemaNode, depth int, s instanceShape) schemaNode {
	out := schemaNode{}
	required, _ := n["required"].([]any)
	props, _ := n["properties"].(schemaNode)
	full := s.full && depth < maxDepth
	for _, k := range sortedKeys(props) {
		if !full && !slices.Contains(required, any(k)) {
			continue
		}
		pn := props[k].(schemaNode)
		switch {
		case b.abs[reflect.ValueOf(pn).Pointer()] && primaryType(pn) == "array":
			out[k] = []any{instancePath}
		case b.abs[reflect.ValueOf(pn).Pointer()]:
			out[k] = instancePath
		default:
			out[k] = b.value(pn, depth+1, s)
		}
	}
	if full {
		switch ap := n["additionalProperties"].(type) {
		case schemaNode:
			out["k"] = b.value(ap, depth+1, s)
		case bool:
			if ap && props == nil {
				out["k"] = "v"
			}
		}
	}
	return out
}

func sortedKeys(m schemaNode) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

func intKey(n schemaNode, key string) int {
	if v, ok := n[key].(json.Number); ok {
		i, err := v.Int64()
		if err == nil {
			return int(i)
		}
	}
	return 0
}

// integerRange is the range of an integer node: its format's, narrowed by
// minimum and maximum. An integer without a format is an int64, as the
// generator types it.
func integerRange(n schemaNode) (lo, hi *big.Int) {
	bits := map[string]struct {
		signed bool
		size   uint
	}{"int32": {true, 32}, "int64": {true, 64}, "": {true, 64}, "int": {true, 64}, "uint16": {false, 16}, "uint32": {false, 32}, "uint64": {false, 64}, "uint": {false, 64}}
	f, _ := n["format"].(string)
	r := bits[f]
	one := big.NewInt(1)
	if r.signed {
		hi = new(big.Int).Sub(new(big.Int).Lsh(one, r.size-1), one)
		lo = new(big.Int).Neg(new(big.Int).Lsh(one, r.size-1))
	} else {
		hi = new(big.Int).Sub(new(big.Int).Lsh(one, r.size), one)
		lo = big.NewInt(0)
	}
	if m, ok := n["minimum"].(json.Number); ok {
		if v, ok := new(big.Int).SetString(m.String(), 10); ok && v.Cmp(lo) > 0 {
			lo = v
		}
	}
	if m, ok := n["maximum"].(json.Number); ok {
		if v, ok := new(big.Int).SetString(m.String(), 10); ok && v.Cmp(hi) < 0 {
			hi = v
		}
	}
	return lo, hi
}

// instances returns the instances of a definition: three shapes of it, or of
// each member of a union.
func (b builder) instances(def string) []any {
	n := b.resolve(b.def(def))
	nodes := []schemaNode{n}
	if isUnion(n) && n["enum"] == nil {
		if ms := b.members(n); len(ms) > 0 {
			nodes = ms
		}
	}
	var out []any
	for _, m := range nodes {
		for _, s := range shapes {
			out = append(out, b.value(m, 0, s))
		}
	}
	return out
}

// TestInstanceBuilder checks the builder on a small schema of its own.
func TestInstanceBuilder(t *testing.T) {
	t.Parallel()
	var doc map[string]any
	d := json.NewDecoder(strings.NewReader(`{
		"Id":{"type":"string","minLength":2},
		"N":{"type":"integer","format":"uint16","minimum":1},
		"Kind":{"type":"string","enum":["a","b"]},
		"Obj":{"type":"object","required":["id"],"properties":{
			"id":{"allOf":[{"$ref":"#/$defs/Id"}],"description":"d"},
			"n":{"$ref":"#/$defs/N"},
			"kind":{"$ref":"#/$defs/Kind"},
			"maybe":{"type":["boolean","null"]},
			"raw":{"description":"anything"},
			"list":{"type":"array","items":{"type":"number","minimum":0.25}},
			"env":{"type":"object","additionalProperties":{"type":"string"}}}},
		"U":{"oneOf":[{"type":"object","required":["t"],"properties":{"t":{"const":"x","type":"string"}}},
			{"allOf":[{"$ref":"#/$defs/Obj"}],"properties":{"t":{"const":"y","type":"string"}},"required":["t"]}]},
		"Shared":{"type":"object","properties":{"t":{"type":"string"},"at":{"type":"string"}},"required":["at"],
			"anyOf":[{"type":"object","required":["t"],"properties":{"t":{"const":"x","type":"string"}}}]}}`))
	d.UseNumber()
	if err := d.Decode(&doc); err != nil {
		t.Fatal(err)
	}
	b := builder{defs: doc}
	got := func(v any) string {
		out, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		return string(out)
	}
	want := []string{
		`{"id":"ss"}`,
		`{"env":{"k":"s"},"id":"ss","kind":"a","list":[0.25],"maybe":true,"n":1,"raw":{"raw":9007199254740993}}`,
		`{"env":{"k":"s"},"id":"ss","kind":"b","list":[1.5],"maybe":false,"n":65535,"raw":{"raw":9007199254740993}}`,
	}
	var gotObj []string
	for _, v := range b.instances("Obj") {
		gotObj = append(gotObj, got(v))
	}
	if !slices.Equal(gotObj, want) {
		t.Errorf("Obj instances:\n got %q\nwant %q", gotObj, want)
	}
	var gotU []string
	for _, v := range b.instances("U") {
		gotU = append(gotU, got(v))
	}
	wantU := []string{`{"t":"x"}`, `{"t":"x"}`, `{"t":"x"}`, `{"id":"ss","t":"y"}`,
		`{"env":{"k":"s"},"id":"ss","kind":"a","list":[0.25],"maybe":true,"n":1,"raw":{"raw":9007199254740993},"t":"y"}`,
		`{"env":{"k":"s"},"id":"ss","kind":"b","list":[1.5],"maybe":false,"n":65535,"raw":{"raw":9007199254740993},"t":"y"}`}
	if !slices.Equal(gotU, wantU) {
		t.Errorf("U instances:\n got %q\nwant %q", gotU, wantU)
	}
	// A union's own property merges into each member, and the member's wins.
	var gotShared []string
	for _, v := range b.instances("Shared") {
		gotShared = append(gotShared, got(v))
	}
	if want := []string{`{"at":"s","t":"x"}`, `{"at":"s","t":"x"}`, `{"at":"s","t":"x"}`}; !slices.Equal(gotShared, want) {
		t.Errorf("Shared instances:\n got %q\nwant %q", gotShared, want)
	}
}
