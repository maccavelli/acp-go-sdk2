package acp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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
	if val, ok := v.(interface{ Validate() error }); ok {
		if err := val.Validate(); err != nil {
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
