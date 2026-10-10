// Package model resolves the loaded schema into the facts the emitters
// render. Every Go name the generator declares is decided here, once, and a
// schema the generator cannot represent fails Build before any file is written.
package model

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/ir"
	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/load"
	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/util"
)

// Kind is how a definition is emitted, in the order the cases are tested.
type Kind int

const (
	// KindNone is a null definition; nothing is emitted.
	KindNone Kind = iota
	// KindEnum is a string type with a constant per enum value.
	KindEnum
	// KindStringConstUnion is a oneOf of string consts, emitted like an enum.
	KindStringConstUnion
	// KindOpenEnum is an anyOf of strings with known consts, emitted like an enum.
	KindOpenEnum
	// KindUnion is a union wrapper with one pointer field per variant.
	KindUnion
	// KindObject is a struct with one field per property.
	KindObject
	// KindPrimitive is a named string, int, float64, or bool.
	KindPrimitive
	// KindEmptyObject is an empty struct.
	KindEmptyObject
	// KindAny is a named any.
	KindAny
)

// Model is the resolved schema.
type Model struct {
	// Schema and Meta are the merged inputs. The emitters still read
	// definitions from them for what the model does not carry yet.
	Schema *load.Schema
	Meta   *load.Meta

	// Types holds one entry per definition, in sorted name order, the order
	// types_gen.go prints them.
	Types  []*Type
	byName map[string]*Type

	// AgentMethodConsts and ClientMethodConsts map a meta method key to the
	// name of its constant.
	AgentMethodConsts  map[string]string
	ClientMethodConsts map[string]string
}

// Type returns the type of the named definition, or nil.
func (m *Model) Type(name string) *Type { return m.byName[name] }

// Type is one schema definition and the names derived from it.
type Type struct {
	Name string
	Def  *load.Definition
	Kind Kind

	// Nested maps a property key of a KindObject to the name of the struct
	// emitted for its inline object; NestedStructs holds those structs, in
	// property order.
	Nested        map[string]string
	NestedStructs []*Struct
	// Fields holds a KindObject's fields, in property order.
	Fields []*Field
	// Underlying is the Go type of a KindPrimitive.
	Underlying *TypeRef
	// Variants holds one entry per entry of a KindUnion's anyOf or oneOf.
	Variants []*Variant
	// ExactlyOne is true for a oneOf union.
	ExactlyOne bool
	// Enum holds the values and constant names of the enum kinds.
	Enum []EnumValue
	// Helpers holds the generated constructors of a oneOf union.
	Helpers []*Helper
}

// UnionList returns the anyOf or oneOf entries of a KindUnion.
func (t *Type) UnionList() []*load.Definition {
	if t.ExactlyOne {
		return t.Def.OneOf
	}
	return t.Def.AnyOf
}

// Variant is one entry of a union.
type Variant struct {
	// TypeName is the Go type of the variant.
	TypeName string
	// FieldName is the wrapper field that holds it.
	FieldName string
	// DiscValue is the variant's discriminator value, if it has one.
	DiscValue string
	// Skip is true for a null entry, or a variant with no $ref, object shape,
	// null type, or title; the wrapper has no field for it.
	Skip bool
	// Inline is the type emitted for a variant without a $ref, or nil.
	Inline *Inline
}

// EnumValue is one constant of an enum kind.
type EnumValue struct {
	Value string
	Const string
}

// Helper is a New<Union><Field> constructor for one oneOf variant.
type Helper struct {
	Name    string
	Variant int
	// Params are the variant's required properties, in schema order, without
	// the discriminator.
	Params []Param
}

// Param is one constructor parameter, named after its property key.
type Param struct {
	Key  string
	Type TypeRef
}

// helperSkip lists unions whose constructors are hand-written in helpers.go.
var helperSkip = map[string]bool{"ContentBlock": true, "ToolCallContent": true, "SessionUpdate": true}

// fixedNames are package-level names the generator declares whatever the schema holds.
var fixedNames = []string{"ProtocolVersionNumber", "Agent", "AgentExperimental", "Client", "ClientExperimental"}

// Build resolves schema and meta.
func Build(meta *load.Meta, schema *load.Schema) (*Model, error) {
	m, err := resolve(meta, schema)
	if err != nil {
		return nil, fmt.Errorf("model: %w", err)
	}
	return m, nil
}

// resolve builds the model; Build adds the error prefix.
func resolve(meta *load.Meta, schema *load.Schema) (*Model, error) {
	if err := checkRefs(schema); err != nil {
		return nil, err
	}
	m := &Model{
		Schema:             schema,
		Meta:               meta,
		byName:             map[string]*Type{},
		AgentMethodConsts:  map[string]string{},
		ClientMethodConsts: map[string]string{},
	}
	n := newNamer()
	names := slices.Sorted(maps.Keys(schema.Defs))
	for _, name := range names {
		if err := n.declareType(name, "definition "+name); err != nil {
			return nil, err
		}
	}
	for _, name := range fixedNames {
		if err := n.declare(name, "the generator"); err != nil {
			return nil, err
		}
	}
	for _, name := range names {
		t, err := buildType(schema, n, name, schema.Defs[name])
		if err != nil {
			return nil, err
		}
		m.Types = append(m.Types, t)
		m.byName[name] = t
	}
	for _, side := range []struct {
		prefix  string
		methods map[string]string
		consts  map[string]string
	}{{"AgentMethod", meta.AgentMethods, m.AgentMethodConsts}, {"ClientMethod", meta.ClientMethods, m.ClientMethodConsts}} {
		for _, key := range slices.Sorted(maps.Keys(side.methods)) {
			c := side.prefix + util.ToExportedField(key)
			if err := n.declare(c, fmt.Sprintf("method %q", side.methods[key])); err != nil {
				return nil, err
			}
			side.consts[key] = c
		}
	}
	return m, nil
}

// classify returns how def is emitted. The cases and their order are the
// types emitter's.
func classify(def *load.Definition) Kind {
	if def == nil {
		return KindNone
	}
	switch t := ir.PrimaryType(def); {
	case len(def.Enum) > 0:
		return KindEnum
	case IsStringConstUnion(def):
		return KindStringConstUnion
	case len(def.AnyOf) > 0 && IsOpenStringEnum(def):
		return KindOpenEnum
	case len(def.AnyOf) > 0, len(def.OneOf) > 0:
		return KindUnion
	case t == schemaTypeObject && len(def.Properties) > 0:
		return KindObject
	case t == schemaTypeString || t == "integer" || t == "number" || t == "boolean":
		return KindPrimitive
	case t == schemaTypeObject:
		return KindEmptyObject
	default:
		return KindAny
	}
}

func buildType(schema *load.Schema, n *namer, name string, def *load.Definition) (*Type, error) {
	t := &Type{Name: name, Def: def, Kind: classify(def)}
	switch t.Kind {
	case KindEnum, KindStringConstUnion, KindOpenEnum:
		for _, v := range enumValues(t) {
			c := util.ToEnumConst(name, v)
			if err := n.declare(c, fmt.Sprintf("%s enum value %q", name, v)); err != nil {
				return nil, err
			}
			t.Enum = append(t.Enum, EnumValue{Value: v, Const: c})
		}
	case KindUnion:
		t.ExactlyOne = len(def.AnyOf) == 0
		if err := buildUnion(schema, n, t); err != nil {
			return nil, err
		}
	case KindObject:
		if err := buildObject(n, t); err != nil {
			return nil, err
		}
	case KindPrimitive:
		u := primitiveRef(ir.PrimaryType(def))
		t.Underlying = &u
	}
	return t, nil
}

// enumValues returns the constant values of an enum kind, as the types
// emitter collects them.
func enumValues(t *Type) []string {
	var values []string
	switch t.Kind {
	case KindEnum:
		for _, v := range t.Def.Enum {
			values = append(values, fmt.Sprint(v))
		}
	case KindStringConstUnion:
		for _, v := range t.Def.OneOf {
			if v != nil && v.Const != nil {
				values = append(values, fmt.Sprint(v.Const))
			}
		}
	case KindOpenEnum:
		for _, v := range t.Def.AnyOf {
			if v == nil || v.Const == nil {
				continue
			}
			if s, ok := v.Const.(string); ok {
				values = append(values, s)
			}
		}
	}
	return values
}

// buildObject names the inline object properties of t and checks its fields
// and theirs.
func buildObject(n *namer, t *Type) error {
	if err := checkFields(t.Name, t.Def.Properties); err != nil {
		return err
	}
	t.Nested = map[string]string{}
	for _, key := range slices.Sorted(maps.Keys(t.Def.Properties)) {
		prop := t.Def.Properties[key]
		owner := t.Name + ".properties." + key
		if prop == nil {
			return fmt.Errorf("%s: property schema is null", owner)
		}
		// Inline objects: no $ref, type object, with properties.
		if prop.Ref != "" || ir.PrimaryType(prop) != schemaTypeObject || len(prop.Properties) == 0 {
			continue
		}
		nested, err := generateNestedTypeName(t.Name, key, n.typeNames)
		if err != nil {
			return fmt.Errorf("%s: %w", owner, err)
		}
		if err := n.declareType(nested, owner); err != nil {
			return err
		}
		if err := checkFields(nested, prop.Properties); err != nil {
			return err
		}
		fields, err := plainFields(nested, prop.Properties, requiredSet(prop.Required))
		if err != nil {
			return err
		}
		t.Nested[key] = nested
		t.NestedStructs = append(t.NestedStructs, &Struct{Name: nested, Doc: docLines(prop.Description), Fields: fields})
	}
	fields, err := objectFields(t)
	if err != nil {
		return err
	}
	t.Fields = fields
	return nil
}

// buildUnion names every variant of t, in schema order, and its helpers.
func buildUnion(schema *load.Schema, n *namer, t *Type) error {
	list := t.UnionList()
	discKey := DiscriminatorKey(schema, t.Def, list)
	fields := map[string]int{}
	for idx, v := range list {
		owner := fmt.Sprintf("%s variant %d", t.Name, idx+1)
		if v == nil {
			t.Variants = append(t.Variants, &Variant{Skip: true})
			continue
		}
		vi, err := buildVariant(schema, n, t, owner, idx, v, discKey)
		if err != nil {
			return err
		}
		t.Variants = append(t.Variants, vi)
		if vi.Skip {
			continue
		}
		if prev, ok := fields[vi.FieldName]; ok {
			return fmt.Errorf("%s: field %s is already used by variant %d", owner, vi.FieldName, prev)
		}
		fields[vi.FieldName] = idx + 1
	}
	if !t.ExactlyOne || t.Def.DocsIgnore || helperSkip[t.Name] {
		return nil
	}
	for idx, v := range list {
		vi := t.Variants[idx]
		if vi.Skip {
			continue
		}
		h := &Helper{Name: "New" + t.Name + vi.FieldName, Variant: idx}
		owner := fmt.Sprintf("%s variant %d", t.Name, idx+1)
		if err := n.declare(h.Name, owner+" constructor"); err != nil {
			return err
		}
		ev := ExpandAllOf(schema, v)
		for _, key := range ev.Required {
			if key == discKey || ev.Properties[key] == nil {
				continue
			}
			if err := checkParam(owner, key); err != nil {
				return err
			}
			h.Params = append(h.Params, Param{Key: key, Type: typeRef(ev.Properties[key])})
		}
		t.Helpers = append(t.Helpers, h)
	}
	return nil
}

// buildVariant names one union variant: its type, in priority $ref target,
// title, discriminator value, position; and its wrapper field, in priority
// discriminator value, title, type name. A generated type name is computed
// in full before it is declared.
func buildVariant(schema *load.Schema, n *namer, t *Type, owner string, idx int, v *load.Definition, discKey string) (*Variant, error) {
	ref := v.Ref
	if ref == "" {
		ref = SingleRefVariant(v)
	}
	ev := ExpandAllOf(schema, v)
	s, ok := ev.Type.(string)
	isNull := ok && s == schemaTypeNull

	dv := ""
	if discKey != "" {
		if pd := ev.Properties[discKey]; pd != nil && pd.Const != nil {
			dv = fmt.Sprint(pd.Const)
		}
	}

	var typeName string
	if target, ok := strings.CutPrefix(ref, refPrefix); ok {
		typeName = target
	} else {
		var candidate string
		var err error
		switch {
		case ev.Title != "":
			candidate, err = generateNestedTypeName(t.Name, ev.Title, n.typeNames)
		case dv != "":
			candidate, err = generateNestedTypeName(t.Name, dv, n.typeNames)
		default:
			candidate = fmt.Sprintf("%sVariant%d", t.Name, idx+1)
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %w", owner, err)
		}
		typeName = util.ToExportedField(candidate)
		if err := n.declareType(typeName, owner); err != nil {
			return nil, err
		}
	}

	field := ""
	switch {
	case dv != "":
		field = util.ToExportedField(dv)
	case ev.Title != "":
		field = util.ToExportedField(ev.Title)
	default:
		field = util.ToExportedField(typeName)
	}

	isObj := len(ev.Properties) > 0
	vi := &Variant{TypeName: typeName, FieldName: field, DiscValue: dv}
	// Phantom variants have no $ref, object shape, null type, or title.
	vi.Skip = !isObj && ref == "" && !isNull && ev.Title == ""
	if vi.Skip {
		return vi, nil
	}
	if err := checkExported(field); err != nil {
		return nil, fmt.Errorf("%s: wrapper field: %w", owner, err)
	}
	if ref == "" && isObj && !isNull {
		merged := maps.Clone(t.Def.Properties)
		if merged == nil {
			merged = map[string]*load.Definition{}
		}
		maps.Copy(merged, ev.Properties)
		if err := checkFields(typeName, merged); err != nil {
			return nil, err
		}
	}
	if ref == "" {
		in, err := inlineOf(t, typeName, ev, isObj, isNull)
		if err != nil {
			return nil, err
		}
		vi.Inline = in
	}
	return vi, nil
}
