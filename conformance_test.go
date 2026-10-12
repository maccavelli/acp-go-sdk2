package acp

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

// The conformance test runs every generated type, union, and method of
// schema_gen_test.go against instances built from the schema files
// (instances_test.go), apart from the generator (0011-PLAN-engine-tests.md,
// D3). A difference is a finding: the test reports it and does not adjust its
// expectation to the SDK.

// typeByName maps a Go type name to its registry entry.
var typeByName = func() map[string]schemaType {
	out := map[string]schemaType{}
	for _, st := range schemaTypes() {
		out[st.Type] = st
	}
	return out
}()

// decodeTree decodes JSON into an any tree with json.Number numbers.
func decodeTree(t *testing.T, b []byte) any {
	t.Helper()
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	var v any
	if err := d.Decode(&v); err != nil {
		t.Fatalf("%s: %v", b, err)
	}
	return v
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("encode instance: %v", err)
	}
	return b
}

// comparer compares a decoded SDK encoding with the instance it came from,
// against the schema: the output must hold every key of the input with an
// equal value, and may add a key the input left out only when it carries the
// schema's default, or is a struct value that holds only such keys. It may
// leave out a key whose input value is null or the schema's default, which
// the schema defines as the same as an absent key ("Omitted and null are
// equivalent"; a default is the value of an absent key).
type comparer struct{ b builder }

// props returns the property schemas of n, of every member when n is a union.
func (c comparer) props(n schemaNode) schemaNode {
	n = c.b.resolve(n)
	out := schemaNode{}
	if p, ok := n["properties"].(schemaNode); ok {
		maps.Copy(out, p)
	}
	if isUnion(n) {
		for _, m := range c.b.members(n) {
			if p, ok := m["properties"].(schemaNode); ok {
				for k, v := range p {
					if _, seen := out[k]; !seen {
						out[k] = v
					}
				}
			}
		}
	}
	return out
}

// child returns the schema of key k of an object node.
func (c comparer) child(n schemaNode, k string) schemaNode {
	if n == nil {
		return nil
	}
	if p, ok := c.props(n)[k].(schemaNode); ok {
		return p
	}
	if ap, ok := c.b.resolve(n)["additionalProperties"].(schemaNode); ok {
		return ap
	}
	return nil
}

// member returns the member of a union node whose const properties the
// object in carries, or n when none does or n is not a union.
func (c comparer) member(n schemaNode, in map[string]any) schemaNode {
	if n == nil || !isUnion(c.b.resolve(n)) {
		return n
	}
	for _, m := range c.b.members(c.b.resolve(n)) {
		props, _ := m["properties"].(schemaNode)
		matched := false
		for k, p := range props {
			cv, ok := c.b.resolve(p.(schemaNode))["const"]
			if !ok {
				continue
			}
			if !reflect.DeepEqual(normalize(cv), in[k]) {
				matched = false
				break
			}
			matched = true
		}
		if matched {
			return m
		}
	}
	return n
}

func (c comparer) diff(path string, in, out any, n schemaNode) []string {
	switch iv := in.(type) {
	case map[string]any:
		return c.diffObject(path, iv, out, c.member(n, iv))
	case []any:
		return c.diffArray(path, iv, out, n)
	default:
		if !reflect.DeepEqual(in, out) {
			return []string{fmt.Sprintf("%s: sent %s, want %s", path, show(out), show(in))}
		}
		return nil
	}
}

// diffObject compares an object key by key: a key of the input must be sent
// equal or may be left out as omitOK allows; a key the SDK adds must be one
// absentOK allows.
func (c comparer) diffObject(path string, iv map[string]any, out any, n schemaNode) []string {
	ov, ok := out.(map[string]any)
	if !ok {
		return []string{fmt.Sprintf("%s: sent %s, want an object", path, show(out))}
	}
	var d []string
	for _, k := range slices.Sorted(maps.Keys(iv)) {
		o, ok := ov[k]
		switch {
		case ok:
			d = append(d, c.diff(path+"."+k, iv[k], o, c.child(n, k))...)
		case !c.omitOK(iv[k], c.child(n, k)):
			d = append(d, fmt.Sprintf("%s.%s: not sent, want %s", path, k, show(iv[k])))
		}
	}
	for _, k := range slices.Sorted(maps.Keys(ov)) {
		if _, ok := iv[k]; !ok && !c.absentOK(path+"."+k, ov[k], c.child(n, k)) {
			d = append(d, fmt.Sprintf("%s.%s: sent %s, which the input did not have", path, k, show(ov[k])))
		}
	}
	return d
}

// diffArray compares an array element by element. The item schema is the
// array's, or, for a union of arrays, that of the first member under which the
// elements compare equal.
func (c comparer) diffArray(path string, iv []any, out any, n schemaNode) []string {
	ov, ok := out.([]any)
	if !ok || len(ov) != len(iv) {
		return []string{fmt.Sprintf("%s: sent %s, want %s", path, show(out), show(iv))}
	}
	var first []string
	for ci, items := range c.itemSchemas(n) {
		var d []string
		for i := range iv {
			d = append(d, c.diff(fmt.Sprintf("%s[%d]", path, i), iv[i], ov[i], items)...)
		}
		if len(d) == 0 {
			return nil
		}
		if ci == 0 {
			first = d
		}
	}
	return first
}

// itemSchemas returns the item schemas an array node may have: its items, or
// each array member's, or nil when there is none.
func (c comparer) itemSchemas(n schemaNode) []schemaNode {
	if n == nil {
		return []schemaNode{nil}
	}
	r := c.b.resolve(n)
	if items, ok := r["items"].(schemaNode); ok {
		return []schemaNode{items}
	}
	var out []schemaNode
	if isUnion(r) {
		for _, m := range c.b.members(r) {
			if items, ok := m["items"].(schemaNode); ok {
				out = append(out, items)
			}
		}
	}
	if len(out) == 0 {
		return []schemaNode{nil}
	}
	return out
}

// omitOK reports whether the SDK may leave out a key whose input value is in:
// null, the schema's default for the key, or an empty map, which holds no key
// as an absent one does (0011-MADR-harden-code-generator-engine.md, "Found in
// step 7 and decided").
func (c comparer) omitOK(in any, n schemaNode) bool {
	if in == nil {
		return true
	}
	if n == nil {
		return false
	}
	if m, ok := in.(map[string]any); ok && len(m) == 0 {
		r := c.b.resolve(n)
		if _, isMap := r["additionalProperties"]; isMap && r["properties"] == nil {
			return true
		}
	}
	for _, m := range []schemaNode{n, c.b.resolve(n)} {
		if def, ok := m["default"]; ok && reflect.DeepEqual(normalize(def), in) {
			return true
		}
	}
	return false
}

// absentOK reports whether out is what the SDK may send for a key the input
// did not have: the schema's default, or a struct value of such keys.
func (c comparer) absentOK(path string, out any, n schemaNode) bool {
	if n == nil {
		return false
	}
	if def, ok := n["default"]; ok && reflect.DeepEqual(normalize(def), out) {
		return true
	}
	r := c.b.resolve(n)
	if def, ok := r["default"]; ok && reflect.DeepEqual(normalize(def), out) {
		return true
	}
	if _, ok := out.(map[string]any); ok && primaryType(r) == "object" && !isUnion(r) {
		return len(c.diff(path, map[string]any{}, out, n)) == 0
	}
	return false
}

// normalize re-decodes a schema value so numbers compare as the output's do.
func normalize(v any) any {
	b, err := json.Marshal(v)
	if err != nil {
		return v
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	var out any
	if d.Decode(&out) != nil {
		return v
	}
	return out
}

func show(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(b)
}

// roundTrip decodes in into a new value of st, validates it, encodes it, and
// returns the differences between in and what was sent.
func roundTrip(t *testing.T, st schemaType, in []byte) (any, []string) {
	t.Helper()
	v := st.New()
	if err := json.Unmarshal(in, v); err != nil {
		return nil, []string{fmt.Sprintf("decode %s: %v", in, err)}
	}
	// A golden or an instance is a peer's message from a host this test does
	// not know, so paths are checked with the portable rule
	// (0015-PLAN-validate-absolute-protocol-paths.md, step 4).
	if val, ok := v.(interface{ validate(func(string) bool) error }); ok {
		if err := val.validate(isPortableAbs); err != nil {
			return v, []string{fmt.Sprintf("validate %s: %v", in, err)}
		}
	}
	out, err := json.Marshal(v)
	if err != nil {
		return v, []string{fmt.Sprintf("encode %s: %v", in, err)}
	}
	b := newBuilder(st.File)
	return v, comparer{b}.diff(st.Def, decodeTree(t, in), decodeTree(t, out), b.def(st.Def))
}

// TestConformanceRoundTrip requires every instance of every generated type to
// decode, validate, and encode to an equal value.
func TestConformanceRoundTrip(t *testing.T) {
	t.Parallel()
	for _, st := range schemaTypes() {
		t.Run(st.Type, func(t *testing.T) {
			t.Parallel()
			for _, inst := range newBuilder(st.File).instances(st.Def) {
				in := mustJSON(t, inst)
				if _, d := roundTrip(t, st, in); len(d) > 0 {
					t.Errorf("%s:\n\t%s", in, strings.Join(d, "\n\t"))
				}
			}
		})
	}
}

// objectNodes returns the object nodes a definition's instances come from:
// the definition, or each member of a union, with the members' discriminator
// keys.
func objectNodes(b builder, def string) (nodes []schemaNode, union bool) {
	n := b.resolve(b.def(def))
	if isUnion(n) {
		return b.members(n), true
	}
	if primaryType(n) == "object" {
		return []schemaNode{n}, false
	}
	return nil, false
}

// variantKeys maps a Go type name to the discriminators it is decoded without,
// as the variant of a discriminated union (TestDiscriminatorExemptOnDecode).
var variantKeys = func() map[string][]string {
	out := map[string][]string{}
	for _, u := range schemaUnions() {
		if u.Key == "" {
			continue
		}
		rt := reflect.TypeOf(typeByName[u.Type].New()).Elem()
		for _, vr := range u.Variants {
			if f, ok := rt.FieldByName(vr.Field); ok && f.Type.Kind() == reflect.Pointer {
				out[f.Type.Elem().Name()] = append(out[f.Type.Elem().Name()], u.Key)
			}
		}
	}
	return out
}()

// discKey returns the discriminator of a union type, or "".
func discKey(typ string) string {
	for _, u := range schemaUnions() {
		if u.Type == typ {
			return u.Key
		}
	}
	return ""
}

// TestConformanceRequired requires every required key, when removed from a
// minimal instance, to fail decoding; a discriminator is exempt.
func TestConformanceRequired(t *testing.T) {
	t.Parallel()
	for _, st := range schemaTypes() {
		t.Run(st.Type, func(t *testing.T) {
			t.Parallel()
			b := newBuilder(st.File)
			nodes, _ := objectNodes(b, st.Def)
			exempt := append(slices.Clone(variantKeys[st.Type]), discKey(st.Type))
			for _, n := range nodes {
				inst, _ := b.value(n, 0, instanceShape{}).(schemaNode)
				required, _ := n["required"].([]any)
				for _, k := range required {
					key := k.(string)
					if slices.Contains(exempt, key) {
						continue
					}
					cut := maps.Clone(inst)
					delete(cut, key)
					if err := json.Unmarshal(mustJSON(t, cut), st.New()); err == nil {
						t.Errorf("%s without required %q decoded", show(inst), key)
					}
				}
			}
		})
	}
}

// kinds are JSON values of each kind, for a value a node does not accept.
var kinds = []struct {
	name  string
	value any
}{{"string", "zz-not-this"}, {"number", json.Number("5.5")}, {"boolean", true}, {"array", []any{true}}, {"object", map[string]any{"zz": true}}}

// accepts returns the JSON kinds n accepts, or nil for any value.
func (b builder) accepts(n schemaNode) map[string]bool {
	n = b.resolve(n)
	out := map[string]bool{}
	if c, ok := n["const"]; ok {
		_ = c
		out["string"] = true
		return out
	}
	if n["enum"] != nil {
		out["string"] = true
		return out
	}
	if isUnion(n) {
		for _, m := range b.members(n) {
			for k := range b.accepts(m) {
				out[k] = true
			}
		}
		if len(out) == 0 {
			return nil
		}
		return out
	}
	switch t := primaryType(n); t {
	case "":
		return nil
	case "integer":
		out["number"] = true
	default:
		out[t] = true
	}
	return out
}

// wrongValue returns a value of a kind n does not accept, or false.
func (b builder) wrongValue(n schemaNode) (any, bool) {
	acc := b.accepts(n)
	if acc == nil {
		return nil, false
	}
	for _, k := range kinds {
		if !acc[k.name] {
			return k.value, true
		}
	}
	return nil, false
}

// TestConformanceLenient requires a property marked
// x-deserialize-default-on-error that holds a value of the wrong kind to
// decode as if absent, and an array marked x-deserialize-skip-invalid-items
// to drop an item of the wrong kind.
func TestConformanceLenient(t *testing.T) {
	t.Parallel()
	for _, st := range schemaTypes() {
		t.Run(st.Type, func(t *testing.T) {
			t.Parallel()
			b := newBuilder(st.File)
			nodes, _ := objectNodes(b, st.Def)
			// A discriminator is not lenient: one that is not a string is an
			// error (TestNonStringDiscriminatorFailsByName).
			discs := append(slices.Clone(variantKeys[st.Type]), discKey(st.Type))
			for _, n := range nodes {
				inst, _ := b.value(n, 0, instanceShape{full: true}).(schemaNode)
				props, _ := n["properties"].(schemaNode)
				for _, k := range sortedKeys(props) {
					p := props[k].(schemaNode)
					if _, has := inst[k]; !has || slices.Contains(discs, k) {
						continue
					}
					if p["x-deserialize-skip-invalid-items"] == true {
						items, _ := b.resolve(p)["items"].(schemaNode)
						bad, ok := b.wrongValue(items)
						if !ok {
							continue
						}
						withBad := maps.Clone(inst)
						withBad[k] = append(slices.Clone(inst[k].([]any)), bad)
						checkAs(t, st, b, withBad, inst, "an invalid item in "+k)
					} else if p["x-deserialize-default-on-error"] == true {
						bad, ok := b.wrongValue(p)
						if !ok {
							continue
						}
						withBad := maps.Clone(inst)
						withBad[k] = bad
						cut := maps.Clone(inst)
						delete(cut, k)
						checkAs(t, st, b, withBad, cut, fmt.Sprintf("%s holding %s", k, show(bad)))
					}
				}
			}
		})
	}
}

// checkAs decodes in and requires what is sent to equal want.
func checkAs(t *testing.T, st schemaType, b builder, in, want schemaNode, what string) {
	t.Helper()
	v := st.New()
	raw := mustJSON(t, in)
	if err := json.Unmarshal(raw, v); err != nil {
		t.Errorf("%s: %s did not decode: %v", what, raw, err)
		return
	}
	out, err := json.Marshal(v)
	if err != nil {
		t.Errorf("%s: encode: %v", what, err)
		return
	}
	if d := (comparer{b}).diff(st.Def, decodeTree(t, mustJSON(t, want)), decodeTree(t, out), b.def(st.Def)); len(d) > 0 {
		t.Errorf("%s: %s sent as %s:\n\t%s", what, raw, out, strings.Join(d, "\n\t"))
	}
}

// TestConformanceBounds requires a value below minimum, above maximum, or
// shorter than minLength, that the Go type holds, to fail Validate naming its
// key.
func TestConformanceBounds(t *testing.T) {
	t.Parallel()
	for _, st := range schemaTypes() {
		t.Run(st.Type, func(t *testing.T) {
			t.Parallel()
			b := newBuilder(st.File)
			nodes, _ := objectNodes(b, st.Def)
			for _, n := range nodes {
				inst, _ := b.value(n, 0, instanceShape{full: true}).(schemaNode)
				props, _ := n["properties"].(schemaNode)
				for _, k := range sortedKeys(props) {
					for _, bad := range b.outOfBounds(props[k].(schemaNode)) {
						withBad := maps.Clone(inst)
						withBad[k] = bad
						raw := mustJSON(t, withBad)
						v := st.New()
						if err := json.Unmarshal(raw, v); err != nil {
							continue // the Go type holds no such value
						}
						if held, _ := decodeTree(t, mustJSON(t, v)).(map[string]any); !reflect.DeepEqual(held[k], normalize(bad)) {
							continue // a lenient field fell back: the Go type holds no such value
						}
						val, ok := v.(interface{ Validate() error })
						if !ok {
							t.Errorf("%s: %s breaks a bound, and %s has no Validate", k, show(bad), st.Type)
							continue
						}
						if err := val.Validate(); err == nil || !strings.Contains(err.Error(), k) {
							t.Errorf("%s = %s: Validate = %v, want an error naming %s", k, show(bad), err, k)
						}
					}
				}
			}
		})
	}
}

// outOfBounds returns values just outside the bounds n declares.
func (b builder) outOfBounds(n schemaNode) []any {
	r := b.resolve(n)
	var out []any
	if ml := intKey(r, "minLength"); ml > 0 {
		out = append(out, strings.Repeat("s", ml-1))
	}
	for key, step := range map[string]string{"minimum": "-1", "maximum": "1"} {
		m, ok := r[key].(json.Number)
		if !ok {
			continue
		}
		f, err := m.Float64()
		if err != nil {
			continue
		}
		delta := 1.0
		if primaryType(r) == "number" {
			delta = 0.5
		}
		if step == "-1" {
			delta = -delta
		}
		out = append(out, json.Number(strings.TrimSuffix(strings.TrimRight(fmt.Sprintf("%f", f+delta), "0"), ".")))
	}
	return out
}

// TestConformanceUnions requires each member of a discriminated union to
// decode into the variant its value names, and a value no variant names to be
// kept in Unknown or the catch-all and sent back unchanged.
func TestConformanceUnions(t *testing.T) {
	t.Parallel()
	for _, u := range schemaUnions() {
		if u.Key == "" {
			continue
		}
		t.Run(u.Type, func(t *testing.T) {
			t.Parallel()
			st := typeByName[u.Type]
			b := newBuilder(st.File)
			members, _ := objectNodes(b, st.Def)
			byValue := map[string]string{}
			for _, vr := range u.Variants {
				if vr.Value != "" {
					byValue[vr.Value] = vr.Field
				}
			}
			for _, m := range members {
				inst, _ := b.value(m, 0, instanceShape{}).(schemaNode)
				value, _ := inst[u.Key].(string)
				field, known := byValue[value]
				if !known {
					continue
				}
				v := st.New()
				if err := json.Unmarshal(mustJSON(t, inst), v); err != nil {
					t.Errorf("%s: %v", show(inst), err)
					continue
				}
				if set := setFields(v); !slices.Equal(set, []string{field}) {
					t.Errorf("%s decoded into %v, want %s", show(inst), set, field)
				}
			}
			if len(members) == 0 {
				return
			}
			inst, _ := b.value(members[0], 0, instanceShape{}).(schemaNode)
			unknown := maps.Clone(inst)
			unknown[u.Key] = "zz-no-such-variant"
			raw := mustJSON(t, unknown)
			v := st.New()
			if err := json.Unmarshal(raw, v); err != nil {
				t.Errorf("an unknown %s %s did not decode: %v", u.Key, raw, err)
				return
			}
			want := "Unknown"
			if u.CatchAll != "" {
				want = u.CatchAll
			}
			if set := setFields(v); !slices.Equal(set, []string{want}) {
				t.Errorf("an unknown %s decoded into %v, want %s", u.Key, set, want)
			}
			out, err := json.Marshal(v)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(decodeTree(t, raw), decodeTree(t, out)) {
				t.Errorf("an unknown %s %s was sent back as %s", u.Key, raw, out)
			}
		})
	}
}

// TestConformanceVariants requires the instance of each member of every
// union to decode into that member's variant, discriminated or not: a const
// that is not a string, a primitive, or an array is placed as the schema says.
// A member of a discriminated union without a value of its own, other than the
// catch-all, is decoded by the absent-discriminator rule and is not checked.
func TestConformanceVariants(t *testing.T) {
	t.Parallel()
	for _, u := range schemaUnions() {
		t.Run(u.Type, func(t *testing.T) {
			t.Parallel()
			st := typeByName[u.Type]
			b := newBuilder(st.File)
			n := b.resolve(b.def(st.Def))
			list, _ := n["oneOf"].([]any)
			if list == nil {
				list, _ = n["anyOf"].([]any)
			}
			if len(list) != len(u.Variants) {
				t.Skipf("%d schema members, %d variants: a member has no field", len(list), len(u.Variants))
			}
			shared := schemaNode{}
			for _, k := range []string{"properties", "required"} {
				if v, ok := n[k]; ok {
					shared[k] = v
				}
			}
			for i, raw := range list {
				vr := u.Variants[i]
				if u.Key != "" && vr.Value == "" && vr.Field != u.CatchAll {
					continue
				}
				member := merge(shared, b.resolve(raw.(schemaNode)))
				for _, shape := range shapes {
					inst := b.value(member, 0, shape)
					in := mustJSON(t, inst)
					v := st.New()
					if err := json.Unmarshal(in, v); err != nil {
						t.Errorf("member %s: %s: %v", vr.Field, in, err)
						continue
					}
					if set := setFields(v); !slices.Equal(set, []string{vr.Field}) {
						t.Errorf("member %s: %s decoded into %v", vr.Field, in, set)
					}
				}
			}
		})
	}
}

// TestConformanceWrongKinds requires a value of a JSON kind the schema does
// not accept to be refused: as the whole value of a type, and as a property
// that is not lenient (x-deserialize-default-on-error and
// x-deserialize-skip-invalid-items take wrong values in TestConformanceLenient).
// The error names the type.
func TestConformanceWrongKinds(t *testing.T) {
	t.Parallel()
	for _, st := range schemaTypes() {
		t.Run(st.Type, func(t *testing.T) {
			t.Parallel()
			b := newBuilder(st.File)
			if bad, ok := b.wrongValue(b.def(st.Def)); ok {
				raw := mustJSON(t, bad)
				if err := json.Unmarshal(raw, st.New()); err == nil {
					t.Errorf("%s decoded as %s", raw, st.Type)
				}
			}
			nodes, _ := objectNodes(b, st.Def)
			for _, n := range nodes {
				inst, _ := b.value(n, 0, instanceShape{full: true}).(schemaNode)
				raws := rawFields(t, st, inst)
				props, _ := n["properties"].(schemaNode)
				for _, k := range sortedKeys(props) {
					p := props[k].(schemaNode)
					if _, has := inst[k]; !has || raws[k] || p["x-deserialize-default-on-error"] == true || p["x-deserialize-skip-invalid-items"] == true {
						continue
					}
					bad, ok := b.wrongValue(p)
					if !ok {
						continue
					}
					withBad := maps.Clone(inst)
					withBad[k] = bad
					raw := mustJSON(t, withBad)
					err := json.Unmarshal(raw, st.New())
					if err == nil {
						t.Errorf("%s holding %s decoded", k, show(bad))
						continue
					}
					if !strings.Contains(err.Error(), st.Type) {
						t.Errorf("%s holding %s: error %q does not name %s", k, show(bad), err, st.Type)
					}
				}
			}
		})
	}
}

// rawFields returns the JSON keys that the struct inst decodes into, or the
// union variant it sets, carries as json.RawMessage: raw JSON takes any value
// (0012-MADR-schema-fidelity-framework.md, treatment 3).
func rawFields(t *testing.T, st schemaType, inst schemaNode) map[string]bool {
	t.Helper()
	v := st.New()
	if err := json.Unmarshal(mustJSON(t, inst), v); err != nil {
		return nil
	}
	rv := reflect.ValueOf(v).Elem()
	if rv.Kind() != reflect.Struct {
		return nil
	}
	// A union: the struct of the variant that is set.
	for i := range rv.NumField() {
		f := rv.Field(i)
		if rv.Type().Field(i).Tag.Get("json") == "-" && f.Kind() == reflect.Pointer && !f.IsNil() && f.Elem().Kind() == reflect.Struct {
			if _, isMember := rv.Type().FieldByName("Unknown"); isMember || len(setFields(v)) == 1 {
				rv = f.Elem()
				break
			}
		}
	}
	raw := reflect.TypeFor[json.RawMessage]()
	out := map[string]bool{}
	for i := range rv.NumField() {
		f := rv.Type().Field(i)
		if f.Type == raw {
			out[strings.Split(f.Tag.Get("json"), ",")[0]] = true
		}
	}
	return out
}

// TestConformanceAbsentDiscriminator decodes each variant's instances without
// the discriminator, as is and without each other required key, and requires
// the README's rule. A variant the schema gives no discriminator value, other
// than the catch-all, takes the payload when it carries that variant's own
// required keys, or refuses it. Else the one variant whose own required keys
// the payload carries takes it, or refuses it when the payload lacks a key the
// union requires of every variant. When none or several fit, the payload is
// kept in Unknown, or, in a union with a catch-all, refused as missing its
// discriminator.
func TestConformanceAbsentDiscriminator(t *testing.T) {
	t.Parallel()
	for _, u := range schemaUnions() {
		if u.Key == "" {
			continue
		}
		t.Run(u.Type, func(t *testing.T) {
			t.Parallel()
			st := typeByName[u.Type]
			b := newBuilder(st.File)
			members, own := unionMembers(b, st.Def)
			if len(members) != len(u.Variants) {
				t.Skipf("%d schema members, %d variants: a member has no field", len(members), len(u.Variants))
			}
			for i, member := range members {
				if u.Variants[i].Value == "" {
					continue
				}
				for _, shape := range shapes {
					inst, _ := b.value(member, 0, shape).(schemaNode)
					delete(inst, u.Key)
					cases := []schemaNode{inst}
					for _, k := range sortedKeys(inst) {
						if slices.Contains(requiredKeys(member), k) {
							cases = append(cases, without(inst, k))
						}
					}
					for _, in := range cases {
						checkAbsent(t, st, u, own, in)
					}
				}
			}
		})
	}
}

// unionMembers returns the members of the union definition def in schema
// order, each with the union's properties and required keys merged in, and
// each member's own required keys, without the union's.
func unionMembers(b builder, def string) ([]schemaNode, [][]string) {
	n := b.resolve(b.def(def))
	list, _ := n["oneOf"].([]any)
	if list == nil {
		list, _ = n["anyOf"].([]any)
	}
	shared := schemaNode{}
	for _, k := range []string{"properties", "required"} {
		if v, ok := n[k]; ok {
			shared[k] = v
		}
	}
	var out []schemaNode
	var own [][]string
	for _, raw := range list {
		m := b.resolve(raw.(schemaNode))
		out = append(out, merge(shared, m))
		own = append(own, requiredKeys(m))
	}
	return out, own
}

// requiredKeys returns a node's required keys.
func requiredKeys(n schemaNode) []string {
	var out []string
	req, _ := n["required"].([]any)
	for _, r := range req {
		out = append(out, r.(string))
	}
	return out
}

// checkAbsent decodes in, which lacks the discriminator, and requires the
// variant, Unknown, or error the rule of TestConformanceAbsentDiscriminator
// gives.
func checkAbsent(t *testing.T, st schemaType, u schemaUnion, own [][]string, in schemaNode) {
	t.Helper()
	carries := func(i int) bool {
		for _, k := range own[i] {
			if _, ok := in[k]; !ok && k != u.Key {
				return false
			}
		}
		return true
	}
	var keyLess, fit []string
	for i, vr := range u.Variants {
		switch {
		case vr.Value == "" && vr.Field != u.CatchAll:
			if carries(i) {
				keyLess = append(keyLess, vr.Field)
			}
		case vr.Field != u.CatchAll && carries(i):
			fit = append(fit, vr.Field)
		}
	}
	raw := mustJSON(t, in)
	v := st.New()
	err := json.Unmarshal(raw, v)
	set := setFields(v)
	switch {
	case hasKeyLess(u):
		if len(keyLess) > 0 && err == nil && !slices.Equal(set, keyLess[:1]) {
			t.Errorf("%s: decoded into %v, %v; want %s", raw, set, err, keyLess[0])
		}
	case len(fit) == 1:
		if err == nil && !slices.Equal(set, fit) {
			t.Errorf("%s: decoded into %v; want %s", raw, set, fit[0])
		}
	case u.Unknown:
		if err != nil || !slices.Equal(set, []string{"Unknown"}) {
			t.Errorf("%s: %d variants fit (%v), decoded into %v, %v; want Unknown", raw, len(fit), fit, set, err)
		}
	default:
		if err == nil || !strings.Contains(err.Error(), "missing discriminator") {
			t.Errorf("%s: %d variants fit (%v), decoded into %v, %v; want missing discriminator", raw, len(fit), fit, set, err)
		}
	}
}

// hasKeyLess reports whether u has a variant without a discriminator value
// that is not its catch-all.
func hasKeyLess(u schemaUnion) bool {
	for _, vr := range u.Variants {
		if vr.Value == "" && vr.Field != u.CatchAll {
			return true
		}
	}
	return false
}

// TestConformanceNullUnion requires JSON null to leave a union that holds a
// variant unchanged, without error, as the README states; RequestId takes it
// as its Null variant.
func TestConformanceNullUnion(t *testing.T) {
	t.Parallel()
	for _, u := range schemaUnions() {
		t.Run(u.Type, func(t *testing.T) {
			t.Parallel()
			st := typeByName[u.Type]
			v := st.New()
			in := mustJSON(t, newBuilder(st.File).instances(st.Def)[0])
			if err := json.Unmarshal(in, v); err != nil {
				t.Fatalf("%s: %v", in, err)
			}
			before := mustJSON(t, v)
			if err := json.Unmarshal([]byte("null"), v); err != nil {
				t.Fatalf("null: %v", err)
			}
			if u.Type == "RequestId" {
				if set := setFields(v); !slices.Equal(set, []string{"Null"}) {
					t.Errorf("null decoded into %v; want Null", set)
				}
				return
			}
			if after := mustJSON(t, v); string(after) != string(before) {
				t.Errorf("null changed %s to %s", before, after)
			}
		})
	}
}

// TestCatchAllWithoutExtraKeys requires a catch-all variant decoded from a
// payload with no undeclared key to leave Extra nil.
func TestCatchAllWithoutExtraKeys(t *testing.T) {
	t.Parallel()
	var v StateUpdate
	if err := json.Unmarshal([]byte(`{"state":"paused"}`), &v); err != nil || v.Other == nil {
		t.Fatalf("decoded into %+v, %v; want Other", v, err)
	}
	if v.Other.Extra != nil {
		t.Errorf("Extra = %#v, want nil", v.Other.Extra)
	}
}

// TestConformanceReplies requires each request with a result to bring back
// what the peer returns: an instance of the result type, equal to the one the
// peer gave, and an error from the peer as an error.
func TestConformanceReplies(t *testing.T) {
	t.Parallel()
	for _, m := range schemaMethods() {
		if m.Side == "protocol" || m.Notification || m.Result == nil {
			continue
		}
		t.Run(m.Side+" "+m.Wire, func(t *testing.T) {
			t.Parallel()
			params := newInstance(t, m.Params())
			result := m.Result()
			rt := typeByName[reflect.TypeOf(result).Elem().Name()]
			want := newBuilder(rt.File).instances(rt.Def)[0]
			got, err := exchange(t, m, params, func(r any) error { return json.Unmarshal(mustJSON(t, want), r) })
			if err != nil {
				t.Fatalf("%s: %v", m.Wire, err)
			}
			c := comparer{newBuilder(rt.File)}
			if d := c.diff(rt.Type, decodeTree(t, mustJSON(t, want)), decodeTree(t, got), c.b.def(rt.Def)); len(d) > 0 {
				t.Errorf("%s: the result arrived as %s, want %s: %v", m.Wire, got, mustJSON(t, want), d)
			}
			if _, err := exchange(t, m, params, func(any) error { return errors.New("refused") }); err == nil || !strings.Contains(err.Error(), "refused") {
				t.Errorf("%s: the peer's error arrived as %v", m.Wire, err)
			}
		})
	}
}

// TestConformanceNotificationErrors requires the error a notification's
// handler returns to be logged by the connection that received it, since a
// notification has no reply to carry it.
func TestConformanceNotificationErrors(t *testing.T) {
	t.Parallel()
	for _, m := range schemaMethods() {
		if m.Side == "protocol" || !m.Notification {
			continue
		}
		t.Run(m.Side+" "+m.Wire, func(t *testing.T) {
			t.Parallel()
			params := newInstance(t, m.Params())
			logged := make(chan string, 4)
			ar, cw := io.Pipe()
			cr, aw := io.Pipe()
			t.Cleanup(func() { _ = cw.Close(); _ = aw.Close() })
			fail := func(any) error { return errors.New("refused") }
			asc := NewAgentSideConnection(recordingAgent{calls: make(chan any, 1), reply: fail}, aw, ar)
			csc := NewClientSideConnection(recordingClient{calls: make(chan any, 1), reply: fail}, cw, cr)
			sender, receiver := csc.conn, asc.conn
			if m.Side == "client" {
				sender, receiver = asc.conn, csc.conn
			}
			receiver.SetLogger(slog.New(slog.NewTextHandler(chanWriter(logged), nil)))
			if err := sender.SendNotification(t.Context(), m.Wire, params); err != nil {
				t.Fatalf("%s: %v", m.Wire, err)
			}
			select {
			case line := <-logged:
				if !strings.Contains(line, "failed to handle notification") || !strings.Contains(line, m.Wire) || !strings.Contains(line, "refused") {
					t.Errorf("%s: logged %q", m.Wire, line)
				}
			case <-time.After(10 * time.Second):
				t.Errorf("%s: the handler's error was not logged", m.Wire)
			}
		})
	}
}

// chanWriter sends each write, a log line, on its channel.
type chanWriter chan string

func (w chanWriter) Write(b []byte) (int, error) {
	w <- string(b)
	return len(b), nil
}

// newInstance decodes the first schema instance of v's type into v.
func newInstance(t *testing.T, v any) any {
	t.Helper()
	st, ok := typeByName[reflect.TypeOf(v).Elem().Name()]
	if !ok {
		t.Fatalf("%T is not in schemaTypes", v)
	}
	raw := mustJSON(t, newBuilder(st.File).instances(st.Def)[0])
	if err := json.Unmarshal(raw, v); err != nil {
		t.Fatalf("%s: %v", raw, err)
	}
	return reflect.ValueOf(v).Elem().Interface()
}

// exchange sends the request m with params over a pipe to a recording peer
// whose reply is reply, and returns the raw result and the error.
func exchange(t *testing.T, m schemaMethod, params any, reply func(any) error) (json.RawMessage, error) {
	t.Helper()
	ar, cw := io.Pipe()
	cr, aw := io.Pipe()
	t.Cleanup(func() { _ = cw.Close(); _ = aw.Close() })
	agent := recordingAgent{calls: make(chan any, 1), reply: reply}
	client := recordingClient{calls: make(chan any, 1), reply: reply}
	asc := NewAgentSideConnection(agent, aw, ar)
	csc := NewClientSideConnection(client, cw, cr)
	sender := csc.conn
	if m.Side == "client" {
		sender = asc.conn
	}
	return SendRequest[json.RawMessage](sender, t.Context(), m.Wire, params)
}

// TestConformanceConstructors calls each union constructor of helpers_gen.go
// with zero arguments and requires the value to set exactly its variant, with
// the discriminator value the schema gives that variant; a value that encodes
// must decode back to the same variant (0013-PLAN-close-phase-6-follow-ups.md).
// A union without a discriminator need only decode to one variant that
// encodes to the same bytes, since JSON cannot carry all its variants apart;
// a catch-all must fail Validate holding another variant's value
// (0014-MADR-union-constructors-take-every-required-key.md, C1 and item 8).
func TestConformanceConstructors(t *testing.T) {
	t.Parallel()
	unions := map[string]schemaUnion{}
	for _, u := range schemaUnions() {
		unions[u.Type] = u
	}
	helpers := schemaHelpers()
	if len(helpers) == 0 {
		t.Fatal("schemaHelpers lists no constructor")
	}
	for _, h := range helpers {
		t.Run(h.Union+" "+h.Field, func(t *testing.T) {
			t.Parallel()
			u, ok := unions[h.Union]
			if !ok {
				t.Fatalf("%s is not in schemaUnions", h.Union)
			}
			v := h.New()
			if set := setFields(v); !slices.Equal(set, []string{h.Field}) {
				t.Fatalf("the constructor set %v, want %s", set, h.Field)
			}
			if u.Key != "" {
				want := ""
				for _, vr := range u.Variants {
					if vr.Field == h.Field {
						want = vr.Value
					}
				}
				if got := discValue(v, h.Field, u.Key); got != want {
					t.Errorf("the discriminator %s is %q, want %q", u.Key, got, want)
				}
			}
			if u.CatchAll == h.Field {
				checkCatchAll(t, h, u)
			}
			b, err := json.Marshal(v)
			if err != nil {
				t.Skipf("the zero-argument value does not encode: %v", err)
			}
			back := typeByName[h.Union].New()
			if err := json.Unmarshal(b, back); err != nil {
				t.Fatalf("%s does not decode: %v", b, err)
			}
			if u.Key != "" {
				if set := setFields(back); !slices.Equal(set, []string{h.Field}) {
					t.Errorf("%s decoded into %v, want %s", b, set, h.Field)
				}
				return
			}
			if set := setFields(back); len(set) != 1 {
				t.Errorf("%s decoded into %v, want one variant", b, set)
			}
			if again, err := json.Marshal(back); err != nil || !bytes.Equal(again, b) {
				t.Errorf("%s decoded and encoded again as %s, %v", b, again, err)
			}
		})
	}
}

// checkCatchAll requires the catch-all value h builds to fail Validate with
// its discriminator set to each value another variant of u has, naming the
// union and the value, and to validate and decode back to the catch-all with
// a value no variant has.
func checkCatchAll(t *testing.T, h schemaHelper, u schemaUnion) {
	t.Helper()
	validate := func(v any) error { return v.(interface{ Validate() error }).Validate() }
	for _, vr := range u.Variants {
		if vr.Value == "" {
			continue
		}
		v := h.New()
		setDisc(v, h.Field, u.Key, vr.Value)
		if err := validate(v); err == nil || !strings.Contains(err.Error(), h.Union) || !strings.Contains(err.Error(), fmt.Sprintf("%q", vr.Value)) {
			t.Errorf("the catch-all holding %s %q: Validate() = %v, want an error naming %s and the value", u.Key, vr.Value, err, h.Union)
		}
	}
	v := h.New()
	setDisc(v, h.Field, u.Key, "_conformance")
	if err := validate(v); err != nil {
		t.Fatalf("the catch-all holding %s %q: Validate() = %v", u.Key, "_conformance", err)
	}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	back := typeByName[h.Union].New()
	if err := json.Unmarshal(b, back); err != nil || !slices.Equal(setFields(back), []string{h.Field}) {
		t.Errorf("%s decoded into %v, %v, want %s", b, setFields(back), err, h.Field)
	}
}

// setDisc sets the string field of the variant field of the union *v whose
// JSON key is key to value.
func setDisc(v any, field, key, value string) {
	s := reflect.ValueOf(v).Elem().FieldByName(field).Elem()
	for i := range s.NumField() {
		if strings.Split(s.Type().Field(i).Tag.Get("json"), ",")[0] == key {
			s.Field(i).SetString(value)
		}
	}
}

// discValue returns the string field of the variant field of the union *v
// whose JSON key is key, or "".
func discValue(v any, field, key string) string {
	variant := reflect.ValueOf(v).Elem().FieldByName(field)
	if variant.Kind() != reflect.Pointer || variant.IsNil() {
		return ""
	}
	s := variant.Elem()
	for i := range s.NumField() {
		if strings.Split(s.Type().Field(i).Tag.Get("json"), ",")[0] == key && s.Field(i).Kind() == reflect.String {
			return s.Field(i).String()
		}
	}
	return ""
}

// setFields returns the names of the union fields of v that are set.
func setFields(v any) []string {
	rv := reflect.ValueOf(v).Elem()
	var out []string
	for i := range rv.NumField() {
		f := rv.Field(i)
		switch {
		case f.Kind() == reflect.Pointer && !f.IsNil(), f.Kind() == reflect.Slice && f.Len() > 0:
			out = append(out, rv.Type().Field(i).Name)
		}
	}
	return out
}

// TestConformanceMethods sends every agent and client method over a pipe and
// requires the recording peer to receive parameters equal to those sent, and
// a request to return the peer's zero result, or the error of encoding it when
// it does not encode.
func TestConformanceMethods(t *testing.T) {
	t.Parallel()
	for _, m := range schemaMethods() {
		if m.Side == "protocol" {
			continue // the connection handles $/cancel_request itself
		}
		t.Run(m.Side+" "+m.Wire, func(t *testing.T) {
			t.Parallel()
			params := m.Params()
			name := reflect.TypeOf(params).Elem().Name()
			st, ok := typeByName[name]
			if !ok {
				t.Fatalf("%s is not in schemaTypes", name)
			}
			inst := newBuilder(st.File).instances(st.Def)[0]
			raw := mustJSON(t, inst)
			if err := json.Unmarshal(raw, params); err != nil {
				t.Fatalf("parameters %s: %v", raw, err)
			}
			ar, cw := io.Pipe()
			cr, aw := io.Pipe()
			t.Cleanup(func() { _ = cw.Close(); _ = aw.Close() })
			agent := recordingAgent{calls: make(chan any, 1)}
			client := recordingClient{calls: make(chan any, 1)}
			asc := NewAgentSideConnection(agent, aw, ar)
			csc := NewClientSideConnection(client, cw, cr)
			sender, calls := csc.conn, agent.calls
			if m.Side == "client" {
				sender, calls = asc.conn, client.calls
			}
			value := reflect.ValueOf(params).Elem().Interface()
			done := make(chan error, 1)
			go func() {
				if m.Notification {
					done <- sender.SendNotification(t.Context(), m.Wire, value)
					return
				}
				_, err := SendRequest[json.RawMessage](sender, t.Context(), m.Wire, value)
				done <- err
			}()
			select {
			case got := <-calls:
				if g, w := decodeTree(t, mustJSON(t, got)), decodeTree(t, mustJSON(t, value)); !reflect.DeepEqual(g, w) {
					t.Errorf("the peer received %s, want %s", show(g), show(w))
				}
			case <-time.After(10 * time.Second):
				t.Fatalf("%s did not reach the peer", m.Wire)
			}
			err := <-done
			// The recording peer returns the zero result. When that value does
			// not encode by itself, such as a union with no variant set, the
			// request must fail with that error and no other.
			var zeroErr error
			if m.Result != nil {
				_, zeroErr = json.Marshal(m.Result())
			}
			switch {
			case zeroErr == nil && err != nil:
				t.Errorf("%s: %v", m.Wire, err)
			case zeroErr != nil && (err == nil || !strings.Contains(err.Error(), "no variant set")):
				t.Errorf("%s: the zero result does not encode (%v), but the request returned %v", m.Wire, zeroErr, err)
			}
		})
	}
}
