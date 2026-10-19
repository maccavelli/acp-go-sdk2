package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
)

// ValueKind is how a Value is written in Go.
type ValueKind int

const (
	// ValueZero is the zero value of the field's type: nothing is written.
	ValueZero ValueKind = iota
	// ValueNil is nil, written out as an item of a slice or a map.
	ValueNil
	// ValueBool is the literal Text, true or false.
	ValueBool
	// ValueString is the string literal Text, quoted.
	ValueString
	// ValueNumber is the number literal Text.
	ValueNumber
	// ValueConst is the enum constant Text.
	ValueConst
	// ValueStruct is a zero struct, written out as {}: an item of a slice
	// or a map.
	ValueStruct
	// ValueSlice is a non-nil slice of Type holding Items.
	ValueSlice
	// ValueMap is a non-nil map of Type holding Items by Keys.
	ValueMap
	// ValuePointer is a pointer to Items[0], a value of Type, written with
	// new.
	ValuePointer
	// ValueRaw is a raw value of Type holding the JSON Text, as decoding
	// gives it, null included.
	ValueRaw
)

// Value is a schema default as a Go value of its field's type: what decoding
// the default's JSON into a new value of that type gives
// (0019-PLAN-idioms-performance.md, D3).
type Value struct {
	Kind ValueKind
	Text string
	// Type is the type of a ValueSlice, ValueMap or ValueRaw, and the type
	// a ValuePointer points to.
	Type  TypeRef
	Keys  []string
	Items []Value
}

// fieldsOf is the fields of a generated struct, and whether decoding fills
// more than its fields: flattened members, a key order, or undeclared keys.
type fieldsOf struct {
	fields []*Field
	more   bool
}

// renderDefaults sets the Go value of every field default, or refuses a
// default no rule renders, naming its definition and property.
func (m *Model) renderDefaults() error {
	structs := map[string]fieldsOf{}
	for _, t := range m.Types {
		if t.Kind == KindObject || t.Kind == KindEmptyObject {
			structs[t.Name] = fieldsOf{t.Fields, len(t.Members) > 0 || len(t.KeyOrders) > 0 || t.KeepExtra}
		}
		for _, ns := range t.NestedStructs {
			structs[ns.Name] = fieldsOf{ns.Fields, false}
		}
		for _, vi := range t.Variants {
			if in := vi.Inline; in != nil && in.Kind == InlineStruct {
				structs[vi.TypeName] = fieldsOf{in.Fields, len(in.Members) > 0}
			}
		}
	}
	r := renderer{m: m, structs: structs}
	for _, owner := range slices.Sorted(maps.Keys(structs)) {
		for _, f := range structs[owner].fields {
			if f.Default == nil {
				continue
			}
			v, err := r.value(f.Type, f.prop.Default, false)
			if err != nil {
				return fmt.Errorf("%s.%s: default %s: %w", owner, f.Key, f.Default.JSON, err)
			}
			f.Default.Value = v
		}
	}
	return nil
}

type renderer struct {
	m       *Model
	structs map[string]fieldsOf
}

// value renders v, a JSON value, as a Go value of r. An item of a slice or a
// map, which a literal must write out, is never ValueZero.
func (r renderer) value(t TypeRef, v any, item bool) (Value, error) {
	zero := func(written Value) (Value, error) {
		if item {
			return written, nil
		}
		return Value{}, nil
	}
	switch t.Kind {
	case RefPointer:
		if v == nil {
			return zero(Value{Kind: ValueNil})
		}
		to, err := r.value(*t.Elem, v, true)
		if err != nil {
			return Value{}, err
		}
		return Value{Kind: ValuePointer, Type: *t.Elem, Items: []Value{to}}, nil
	case RefSlice, RefMap:
		return r.collection(t, v, item)
	case RefRaw:
		return rawValue(t, v)
	case RefNamed:
	default:
		return Value{}, fmt.Errorf("no Go value of %s is rendered", t)
	}
	switch name := t.Name; {
	case name == goBool:
		b, ok := v.(bool)
		switch {
		case !ok:
			return Value{}, fmt.Errorf("%s is not a bool", valueText(v))
		case !b:
			return zero(Value{Kind: ValueBool, Text: "false"})
		}
		return Value{Kind: ValueBool, Text: "true"}, nil
	case name == schemaTypeString:
		s, ok := v.(string)
		switch {
		case !ok:
			return Value{}, fmt.Errorf("%s is not a string", valueText(v))
		case s == "":
			return zero(Value{Kind: ValueString, Text: strconv.Quote("")})
		}
		return Value{Kind: ValueString, Text: strconv.Quote(s)}, nil
	case isGoNumber(name):
		n, ok := v.(float64)
		switch {
		case !ok:
			return Value{}, fmt.Errorf("%s is not a number", valueText(v))
		case n == 0:
			return zero(Value{Kind: ValueNumber, Text: "0"})
		}
		return Value{Kind: ValueNumber, Text: strconv.FormatFloat(n, 'f', -1, 64)}, nil
	}
	if s, ok := r.structs[t.Name]; ok {
		return r.structValue(t.Name, s, v, item)
	}
	nt := r.m.Type(t.Name)
	if nt == nil {
		return Value{}, fmt.Errorf("%s is not a generated type", t.Name)
	}
	switch nt.Kind {
	case KindPrimitive:
		return r.value(*nt.Underlying, v, item)
	case KindEnum, KindStringConstUnion, KindOpenEnum, KindIntegerEnum:
		text := fmt.Sprint(v)
		if n, ok := v.(float64); ok {
			text = strconv.FormatFloat(n, 'f', -1, 64)
		}
		for _, ev := range nt.Enum {
			if ev.Const != "" && ev.Value == text {
				return Value{Kind: ValueConst, Text: ev.Const}, nil
			}
		}
		// A value with no constant is a literal of the type.
		if nt.Kind == KindIntegerEnum {
			return r.value(named(goInt64), v, item)
		}
		return r.value(named(schemaTypeString), v, item)
	case KindAny:
		return rawValue(t, v)
	}
	return Value{}, fmt.Errorf("no Go value of %s, a %s, is rendered", t.Name, kindName(nt.Kind))
}

// collection renders v as a slice or a map of t: null is nil, and an array
// or an object a literal of its items, each rendered by the same rules.
func (r renderer) collection(t TypeRef, v any, item bool) (Value, error) {
	if v == nil {
		if item {
			return Value{Kind: ValueNil}, nil
		}
		return Value{}, nil
	}
	out := Value{Kind: ValueSlice, Type: t}
	if t.Kind == RefMap {
		obj, ok := v.(map[string]any)
		if !ok {
			return Value{}, fmt.Errorf("%s is not an object", valueText(v))
		}
		out.Kind = ValueMap
		for _, k := range slices.Sorted(maps.Keys(obj)) {
			iv, err := r.value(*t.Elem, obj[k], true)
			if err != nil {
				return Value{}, fmt.Errorf("key %q: %w", k, err)
			}
			out.Keys, out.Items = append(out.Keys, k), append(out.Items, iv)
		}
		return out, nil
	}
	list, ok := v.([]any)
	if !ok {
		return Value{}, fmt.Errorf("%s is not an array", valueText(v))
	}
	for i, x := range list {
		iv, err := r.value(*t.Elem, x, true)
		if err != nil {
			return Value{}, fmt.Errorf("item %d: %w", i, err)
		}
		out.Items = append(out.Items, iv)
	}
	return out, nil
}

// errNotZero is the refusal of a struct default decoding gives a value other
// than the zero struct.
var errNotZero = errors.New("is not the zero struct, which is the only struct default rendered")

// structValue renders v as the struct name: only the zero struct is
// rendered. It is zero when every key v has decodes to its field's zero
// value, and every field v leaves out, or holds null where the field refuses
// null, has no default or a zero one. A struct decoding fills more than its
// fields, or a default without a required key, is refused.
func (r renderer) structValue(name string, s fieldsOf, v any, item bool) (Value, error) {
	obj, ok := v.(map[string]any)
	if !ok {
		return Value{}, fmt.Errorf("%s is not an object", valueText(v))
	}
	if s.more {
		return Value{}, fmt.Errorf("%s fills more than its fields when decoded, so no default of it is rendered", name)
	}
	for _, f := range s.fields {
		x, present := obj[f.Key]
		if !present && f.Required {
			return Value{}, fmt.Errorf("%s lacks the required key %q", name, f.Key)
		}
		if !present || (x == nil && f.Default != nil && !f.Default.AllowNull) {
			if f.prop == nil || f.prop.Default == nil {
				continue
			}
			x = f.prop.Default
		}
		fv, err := r.value(f.Type, x, false)
		if err != nil {
			return Value{}, fmt.Errorf("%s.%s: %w", name, f.Key, err)
		}
		if fv.Kind != ValueZero {
			return Value{}, fmt.Errorf("%s.%s %w", name, f.Key, errNotZero)
		}
	}
	if item {
		return Value{Kind: ValueStruct}, nil
	}
	return Value{}, nil
}

// rawValue renders v as a raw value of t: its JSON, which decoding copies,
// null included.
func rawValue(t TypeRef, v any) (Value, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return Value{}, fmt.Errorf("encode: %w", err)
	}
	return Value{Kind: ValueRaw, Type: t, Text: string(b)}, nil
}

// isGoNumber reports whether name is a Go number type the model uses.
func isGoNumber(name string) bool {
	_, sized := goRange[name]
	return sized || name == goFloat32 || name == goFloat64
}

// kindName names a type kind for an error.
func kindName(k Kind) string {
	switch k {
	case KindUnion:
		return "union"
	case KindNone:
		return "null definition"
	}
	return fmt.Sprintf("kind %d", k)
}
