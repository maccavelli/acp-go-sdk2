package model

import (
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"reflect"
	"slices"
	"strings"

	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/load"
	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/util"
)

const (
	schemaTypeArray   = "array"
	schemaTypeInteger = "integer"
	schemaTypeNumber  = "number"
	schemaTypeBoolean = "boolean"
)

// RefKind is the shape of a TypeRef.
type RefKind int

const (
	// RefNamed is a named type: a definition, a predeclared type, or any.
	RefNamed RefKind = iota
	// RefPointer is *Elem.
	RefPointer
	// RefSlice is []Elem.
	RefSlice
	// RefMap is map[string]Elem.
	RefMap
	// RefRaw is json.RawMessage: a value the schema gives no Go type, carried
	// as it was received (0012-MADR-schema-fidelity-framework.md).
	RefRaw
	// RefPatch is Patch[Elem]: absent, null, or a value, for a property with
	// patch semantics (0021-MADR-resolve-issues-4-and-5-before-v1-14-0.md).
	RefPatch
)

// TypeRef is a Go type expression.
type TypeRef struct {
	Kind RefKind
	Name string
	Elem *TypeRef
}

func named(name string) TypeRef { return TypeRef{Kind: RefNamed, Name: name} }

// rawJSON is the TypeRef of json.RawMessage.
func rawJSON() TypeRef { return TypeRef{Kind: RefRaw} }

func wrap(kind RefKind, elem TypeRef) TypeRef { return TypeRef{Kind: kind, Elem: &elem} }

// String is the type as Go source.
func (r TypeRef) String() string {
	switch r.Kind {
	case RefPointer:
		return "*" + r.Elem.String()
	case RefSlice:
		return "[]" + r.Elem.String()
	case RefMap:
		return "map[string]" + r.Elem.String()
	case RefRaw:
		return "json.RawMessage"
	case RefPatch:
		return "Patch[" + r.Elem.String() + "]"
	default:
		return r.Name
	}
}

// IsCollection reports whether the zero value of r is a nil slice or map.
func (r TypeRef) IsCollection() bool { return r.Kind == RefSlice || r.Kind == RefMap }

// DefaultKind classifies a schema default value.
type DefaultKind int

const (
	// DefaultNone is no default.
	DefaultNone DefaultKind = iota
	// DefaultScalar is a string, number, or boolean default.
	DefaultScalar
	// DefaultArray is an array default.
	DefaultArray
	// DefaultObject is an object default.
	DefaultObject
)

// Default is a property's schema default.
type Default struct {
	// JSON is the default's canonical encoding.
	JSON string
	Kind DefaultKind
	// AllowNull is true when the property's type includes null.
	AllowNull bool
	// Nilable is true when the field's zero value is a nil slice or map.
	Nilable bool
}

// Facts are the schema's constraints on a property that the model reads
// after the field is built: the bounds, which validation checks, and the
// x-deserialize keywords, which the decoders follow.
type Facts struct {
	Minimum   *float64
	Maximum   *float64
	MinLength *int
	// DefaultOnError and SkipInvalidItems are the x-deserialize keywords.
	DefaultOnError   bool
	SkipInvalidItems bool
}

// Field is one struct field.
type Field struct {
	// Key is the JSON property name; Name is the Go field name.
	Key  string
	Name string
	Type TypeRef
	// Tag is the json struct tag value, such as "name,omitempty".
	Tag      string
	Required bool
	// Doc is the field's doc comment, one entry per line.
	Doc []string
	// Default is set on a top-level object's field with a schema default.
	Default *Default
	// Fill is true for a required slice or map that encodes nil as empty.
	Fill bool
	// NullIsAbsent is true for a raw field that is not required: a JSON null
	// in it decodes to an empty value, as ACP v1 defines a null raw value to
	// be an absent one (0012-MADR-schema-fidelity-framework.md, amendment of
	// 2026-10-11), unless a null-retained rule names its property.
	NullIsAbsent bool
	// Patch is true for a property a patch rule names: its type is RefPatch,
	// omitted when absent (0021-MADR-resolve-issues-4-and-5-before-v1-14-0.md).
	Patch bool
	Facts Facts
	// prop is the schema property the field is built from; every type that
	// holds the property, an inline allOf variant included, shares it.
	prop *load.Definition
}

// Struct is a generated struct other than a definition's own type.
type Struct struct {
	Name   string
	Doc    []string
	Fields []*Field
	// Validation is what the struct's Validate checks; nil is no Validate.
	Validation *Validation
}

// InlineKind is how an inline union variant's type is emitted.
type InlineKind int

const (
	// InlineStruct is a struct of the variant's properties and the union's.
	InlineStruct InlineKind = iota
	// InlineNamed is a defined type of Type.
	InlineNamed
	// InlineEmpty is an empty struct.
	InlineEmpty
)

// Inline is the type emitted for a union variant without a $ref.
type Inline struct {
	Kind   InlineKind
	Doc    []string
	Fields []*Field
	Type   TypeRef
	// KeepExtra is true for a union's catch-all: the struct keeps the keys it
	// does not declare, so the payload encodes back unchanged.
	KeepExtra bool
	// Exempt is the discriminator of a discriminated union's inline variant:
	// the union placed the payload, so decoding does not require the key.
	Exempt string
	// Validation is what the struct's Validate checks; nil is no Validate.
	Validation *Validation
	// Members are the flattened members of the definition the variant is an
	// allOf of, when it has them (see Type.Members).
	Members []*Member
}

// Member is one flattened member of an object: a pointer field of the
// referenced type, whose keys share the object's JSON object.
type Member struct {
	// Field is the Go field, named by the member's title; Type is the
	// referenced definition.
	Field, Type string
	Doc         []string
	// Required are the member type's required keys: a payload that carries
	// them all fits the member.
	Required []string
	// Validates is true when the member's type has a Validate method.
	Validates bool
}

// docLines formats a description as doc comment lines, or nil.
func docLines(desc string) []string {
	if desc == "" {
		return nil
	}
	return util.FormatDocComment(desc)
}

// primitiveRef maps a JSON Schema primitive type and its format to a Go type,
// or any. An integer or number takes the Go type of its format (keywords.go);
// checkRefs has refused a format without one.
func primitiveRef(t, format string) TypeRef {
	switch t {
	case schemaTypeString:
		return named("string")
	case schemaTypeInteger, schemaTypeNumber:
		return named(numericFormat(t, format))
	case schemaTypeBoolean:
		return named("bool")
	default:
		return named("any")
	}
}

// pointerPrimitive returns *T for a primitive schema type and its format.
func pointerPrimitive(t, format string) (TypeRef, bool) {
	switch t {
	case schemaTypeString, schemaTypeInteger, schemaTypeNumber, schemaTypeBoolean:
		return wrap(RefPointer, primitiveRef(t, format)), true
	default:
		return TypeRef{}, false
	}
}

// typeRef maps a schema node to a Go type.
func typeRef(d *load.Definition) TypeRef {
	if d == nil {
		return named("any")
	}

	// JSON Schema 2020-12 commonly uses allOf wrappers around a single $ref so a property can
	// attach local metadata like descriptions and defaults.
	if len(d.AllOf) > 0 && d.Ref == "" {
		// Prefer a $ref element when present, otherwise the first non-nil schema.
		for _, e := range d.AllOf {
			if e != nil && e.Ref != "" {
				return typeRef(e)
			}
		}
		for _, e := range d.AllOf {
			if e != nil {
				return typeRef(e)
			}
		}
	}

	if d.Ref != "" {
		if name, ok := strings.CutPrefix(d.Ref, refPrefix); ok {
			return named(name)
		}
		return named("any")
	}
	if len(d.Enum) > 0 {
		return named("string")
	}
	switch t := primaryType(d); t {
	case schemaTypeString, schemaTypeInteger, schemaTypeNumber, schemaTypeBoolean:
		return primitiveRef(t, d.Format)
	case schemaTypeArray:
		return wrap(RefSlice, typeRef(d.Items))
	case schemaTypeObject:
		return wrap(RefMap, mapValues(d.AdditionalProperties))
	default:
		// No Go type: carried as received.
		return rawJSON()
	}
}

// mapValues is the value type of a map from its additionalProperties schema:
// that schema's type, or any when it has none (true, or no schema).
func mapValues(ap *load.Definition) TypeRef {
	if ap == nil {
		return named("any")
	}
	if v := typeRef(ap); v.Kind != RefRaw {
		return v
	}
	return named("any")
}

// optionalTypeRef maps unions that include null to pointer types where applicable.
func optionalTypeRef(d *load.Definition) TypeRef {
	if d == nil {
		return named("any")
	}
	// Case: property type is a union like ["string","null"]
	if arr, ok := d.Type.([]any); ok && len(arr) == 2 {
		var other string
		for _, v := range arr {
			if s, ok := v.(string); ok && s != schemaTypeNull {
				other = s
			}
		}
		if p, ok := pointerPrimitive(other, d.Format); ok {
			return p
		}
	}
	list := d.AnyOf
	if len(list) == 0 {
		list = d.OneOf
	}
	if len(list) == 2 {
		var nonNull *load.Definition
		for _, e := range list {
			if e == nil {
				continue
			}
			if s, ok := e.Type.(string); ok && s == schemaTypeNull {
				continue
			}
			if e.Const != nil {
				nn := *e
				nn.Type = schemaTypeString
				nonNull = &nn
			} else {
				nonNull = e
			}
		}
		if nonNull != nil {
			if name, ok := strings.CutPrefix(nonNull.Ref, refPrefix); ok {
				return wrap(RefPointer, named(name))
			}
			if p, ok := pointerPrimitive(primaryType(nonNull), nonNull.Format); ok {
				return p
			}
		}
	}
	return typeRef(d)
}

// includesNull reports whether the property's type union contains null.
func includesNull(d *load.Definition) bool {
	if d == nil {
		return false
	}
	arr, ok := d.Type.([]any)
	return ok && slices.Contains(arr, any(schemaTypeNull))
}

// checkDefaults refuses a property default its schema does not admit: a value
// of another JSON kind, an integer that is fractional or outside the range of
// its format's Go type, a value outside its enum or other than its const,
// through a reference, an allOf of one, array items, object properties and
// union members. The generated code decodes a default without checking the
// error, which a default checked here cannot return
// (0019-PLAN-generator-hardening.md, G10).
func (m *Model) checkDefaults() error {
	for _, t := range m.Types {
		lists := [][]*Field{t.Fields}
		for _, ns := range t.NestedStructs {
			lists = append(lists, ns.Fields)
		}
		for _, vi := range t.Variants {
			if vi.Inline != nil {
				lists = append(lists, vi.Inline.Fields)
			}
		}
		for _, fields := range lists {
			for _, f := range fields {
				if f.prop == nil || f.prop.Default == nil {
					continue
				}
				if err := admits(m.schema, t.Name+"."+f.Key, f.prop, f.prop.Default); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// admits fails when the schema d does not admit the JSON value v, naming
// where in the default it is.
func admits(schema *load.Schema, where string, d *load.Definition, v any) error {
	d = resolveSchema(schema, d)
	if d == nil {
		return nil
	}
	if len(d.Enum) > 0 && !slices.ContainsFunc(d.Enum, func(e any) bool { return reflect.DeepEqual(e, v) }) {
		return fmt.Errorf("%s: default %s is not one of its enum values", where, valueText(v))
	}
	if d.Const != nil && !reflect.DeepEqual(d.Const, v) {
		return fmt.Errorf("%s: default %s is not its const %s", where, valueText(v), valueText(d.Const))
	}
	if list := slices.Concat(d.AnyOf, d.OneOf); len(list) > 0 && len(d.Properties) == 0 {
		for _, member := range list {
			if member != nil && admits(schema, where, member, v) == nil {
				return nil
			}
		}
		return fmt.Errorf("%s: default %s fits no member of its union", where, valueText(v))
	}
	pt := primaryType(d)
	if pt == "" && len(d.Properties) > 0 {
		pt = schemaTypeObject
	}
	if v == nil {
		if pt == "" || includesNull(d) {
			return nil
		}
		return fmt.Errorf("%s: default null is not a %s", where, pt)
	}
	switch pt {
	case schemaTypeString:
		if _, ok := v.(string); !ok {
			return fmt.Errorf("%s: default %s is not a string", where, valueText(v))
		}
	case schemaTypeBoolean:
		if _, ok := v.(bool); !ok {
			return fmt.Errorf("%s: default %s is not a bool", where, valueText(v))
		}
	case schemaTypeNumber:
		if _, ok := v.(float64); !ok {
			return fmt.Errorf("%s: default %s is not a number", where, valueText(v))
		}
	case schemaTypeInteger:
		goType := numericFormat(pt, d.Format)
		f, ok := v.(float64)
		rng, sized := goRange[goType]
		if !ok || f != math.Trunc(f) || (sized && (f < rng[0] || f > rng[1])) {
			return fmt.Errorf("%s: default %s is not a %s", where, valueText(v), goType)
		}
	case schemaTypeArray:
		items, ok := v.([]any)
		if !ok {
			return fmt.Errorf("%s: default %s is not an array", where, valueText(v))
		}
		for i, item := range items {
			if err := admits(schema, fmt.Sprintf("%s[%d]", where, i), d.Items, item); err != nil {
				return err
			}
		}
	case schemaTypeObject:
		obj, ok := v.(map[string]any)
		if !ok {
			return fmt.Errorf("%s: default %s is not an object", where, valueText(v))
		}
		props := expandAllOf(schema, d).Properties
		for _, key := range slices.Sorted(maps.Keys(obj)) {
			if err := admits(schema, where+"."+key, props[key], obj[key]); err != nil {
				return err
			}
		}
	}
	return nil
}

// resolveSchema follows d's $ref, and an allOf of one schema, to the schema
// that types it, or nil for a reference the schema lacks.
func resolveSchema(schema *load.Schema, d *load.Definition) *load.Definition {
	for range len(schema.Defs) + 1 {
		switch {
		case d == nil:
			return nil
		case d.Ref != "":
			d = schema.Defs[strings.TrimPrefix(d.Ref, refPrefix)]
		case len(d.AllOf) == 1 && len(d.Properties) == 0:
			d = d.AllOf[0]
		default:
			return d
		}
	}
	return d
}

// valueText writes a JSON value of the schema for an error: a scalar as the
// schema writes it, an array or an object by its kind.
func valueText(v any) string {
	switch v.(type) {
	case []any:
		return "array"
	case map[string]any:
		return "object"
	}
	return schemaValue(v)
}

// defaultKindOf classifies a default value decoded from JSON.
func defaultKindOf(val any) DefaultKind {
	switch val.(type) {
	case nil:
		return DefaultNone
	case []any:
		return DefaultArray
	case map[string]any:
		return DefaultObject
	default:
		return DefaultScalar
	}
}

// factsOf records the constraints of prop.
func factsOf(prop *load.Definition) Facts {
	return Facts{
		Minimum:          prop.Minimum,
		Maximum:          prop.Maximum,
		MinLength:        prop.MinLength,
		DefaultOnError:   prop.XDeserializeDefaultOnError,
		SkipInvalidItems: prop.XDeserializeSkipInvalidItems,
	}
}

// objectFields builds the fields of a definition's own struct. A property
// with an inline object takes the nested type the namer gave it.
func objectFields(t *Type) ([]*Field, error) {
	req := requiredSet(t.def.Required)
	var out []*Field
	for _, key := range slices.Sorted(maps.Keys(t.def.Properties)) {
		prop := t.def.Properties[key]
		name, err := fieldName(t.Name, key)
		if err != nil {
			return nil, err
		}
		fld := &Field{Key: key, Name: name, Tag: key, Required: req[key], Doc: docLines(prop.Description), Facts: factsOf(prop), prop: prop}
		if prop.Default != nil {
			// Whether the field's zero value is nil (slice or map), for the encode fill-in.
			pt := primaryType(prop)
			nilable := pt == schemaTypeArray || (pt == schemaTypeObject && len(prop.Properties) == 0 && prop.Ref == "")
			defJSON := schemaTypeNull
			if b, err := json.Marshal(prop.Default); err == nil {
				defJSON = string(b)
			}
			fld.Default = &Default{JSON: defJSON, Kind: defaultKindOf(prop.Default), AllowNull: includesNull(prop), Nilable: nilable}
		}
		if !fld.Required {
			// Optional fields are omitted when empty, except a field whose default
			// is an array or object and whose zero value is nil: it is always sent.
			dp := fld.Default
			if dp == nil || (dp.Kind != DefaultArray && dp.Kind != DefaultObject) || !dp.Nilable {
				fld.Tag = key + ",omitempty"
			}
		}
		if dp := fld.Default; dp != nil && dp.JSON != schemaTypeNull {
			// An empty comment line separates the default from the description.
			if prop.Description != "" {
				fld.Doc = append(fld.Doc, "")
			}
			fld.Doc = append(fld.Doc, fmt.Sprintf("Defaults to %s if unset.", dp.JSON))
		}
		if nested, ok := t.Nested[key]; ok {
			fld.Type = named(nested)
		} else {
			fld.Type = optionalTypeRef(prop)
			fld.Fill = fld.Required && fld.Default == nil && fld.Type.IsCollection()
		}
		fld.NullIsAbsent = !fld.Required && fld.Type.Kind == RefRaw
		out = append(out, fld)
	}
	return out, nil
}

// plainFields builds the fields of a nested object or an inline variant: an
// optional field is omitted when empty, and a required slice or map encodes
// nil as empty. Defaults on these properties are not applied.
func plainFields(owner string, props map[string]*load.Definition, req map[string]bool) ([]*Field, error) {
	var out []*Field
	for _, key := range slices.Sorted(maps.Keys(props)) {
		prop := props[key]
		name, err := fieldName(owner, key)
		if err != nil {
			return nil, err
		}
		fld := &Field{Key: key, Name: name, Tag: key, Required: req[key], Doc: docLines(prop.Description), Facts: factsOf(prop), prop: prop}
		fld.Type = optionalTypeRef(prop)
		fld.NullIsAbsent = !fld.Required && fld.Type.Kind == RefRaw
		if fld.Required {
			fld.Fill = fld.Type.IsCollection()
		} else {
			fld.Tag = key + ",omitempty"
		}
		out = append(out, fld)
	}
	return out, nil
}

// inlineOf returns the type emitted for a union variant without a $ref, or
// nil when there is none.
func inlineOf(t *Type, tname string, ev *load.Definition, isObj, isNull bool) (*Inline, error) {
	switch {
	case isNull:
		return &Inline{Kind: InlineEmpty}, nil
	case isObj:
		req := requiredSet(ev.Required)
		maps.Copy(req, requiredSet(t.def.Required))
		props := maps.Clone(t.def.Properties)
		if props == nil {
			props = map[string]*load.Definition{}
		}
		maps.Copy(props, ev.Properties)
		for _, key := range slices.Sorted(maps.Keys(req)) {
			if props[key] == nil {
				// (0019-PLAN-generator-hardening.md, G11.)
				return nil, fmt.Errorf("%s: required key %q is not a property", tname, key)
			}
		}
		fields, err := plainFields(tname, props, req)
		if err != nil {
			return nil, err
		}
		return &Inline{Kind: InlineStruct, Doc: docLines(ev.Description), Fields: fields}, nil
	case ev.Title != "":
		in := &Inline{Kind: InlineEmpty, Doc: docLines(ev.Description)}
		if primaryType(ev) != "" {
			in.Kind, in.Type = InlineNamed, typeRef(ev)
		}
		return in, nil
	default:
		return nil, nil
	}
}

// checkFoldedKeys refuses a schema in which two keys that one generated decoder
// reads differ only in case: the decoder reads a key as encoding/json reads a
// field's name, under Unicode case folding, so it would read the two as one
// (0011-MADR-harden-code-generator-engine.md, "Found in step 8 and decided").
// The keys are an object's properties with its members' required keys, and a
// union's discriminator with the keys that pick its variants.
func (m *Model) checkFoldedKeys() error {
	object := func(fields []*Field, members []*Member) []string {
		var keys []string
		for _, f := range fields {
			keys = append(keys, f.Key)
		}
		for _, mb := range members {
			keys = append(keys, mb.Required...)
		}
		return keys
	}
	for _, t := range m.Types {
		sets := map[string][]string{t.Name: object(t.Fields, t.Members)}
		for _, ns := range t.NestedStructs {
			sets[ns.Name] = object(ns.Fields, nil)
		}
		union := []string{}
		if t.DiscKey != "" {
			union = append(union, t.DiscKey)
		}
		for _, v := range t.Variants {
			if v.Inline != nil {
				sets[v.TypeName] = object(v.Inline.Fields, v.Inline.Members)
			}
			union = append(union, v.Required...)
			union = append(union, v.FitKeys...)
			sets[v.TypeName+" items"] = v.ArrayItemRequired
		}
		sets[t.Name+" variants"] = union
		for _, owner := range slices.Sorted(maps.Keys(sets)) {
			if a, b, ok := foldedPair(sets[owner]); ok {
				return fmt.Errorf("%s: the keys %q and %q differ only in case, and would be read as one", owner, a, b)
			}
		}
	}
	return nil
}

// foldedPair returns the first two distinct keys that differ only in case.
func foldedPair(keys []string) (string, string, bool) {
	for i, a := range keys {
		for _, b := range keys[i+1:] {
			if a != b && strings.EqualFold(a, b) {
				return a, b, true
			}
		}
	}
	return "", "", false
}

// dropStructOmitempty removes omitempty from every field whose Go type is a
// struct value: a named object, a union wrapper, or a nested struct.
// encoding/json never omits a struct, so the tag then says what the encoder
// does (0012-MADR-schema-fidelity-framework.md).
func (m *Model) dropStructOmitempty() {
	nested := map[string]bool{}
	for _, t := range m.Types {
		for _, ns := range t.NestedStructs {
			nested[ns.Name] = true
		}
	}
	isStruct := func(r TypeRef) bool {
		if r.Kind != RefNamed {
			return false
		}
		if t := m.Type(r.Name); t != nil {
			return t.Kind == KindObject || t.Kind == KindUnion
		}
		return nested[r.Name]
	}
	fix := func(fields []*Field) {
		for _, f := range fields {
			if isStruct(f.Type) {
				f.Tag = strings.TrimSuffix(f.Tag, ",omitempty")
			}
		}
	}
	for _, t := range m.Types {
		fix(t.Fields)
		for _, ns := range t.NestedStructs {
			fix(ns.Fields)
		}
		for _, v := range t.Variants {
			if v.Inline != nil {
				fix(v.Inline.Fields)
			}
		}
	}
}
