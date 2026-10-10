package model

import (
	"encoding/json"
	"fmt"
	"maps"
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
)

// TypeRef is a Go type expression.
type TypeRef struct {
	Kind RefKind
	Name string
	Elem *TypeRef
}

func named(name string) TypeRef { return TypeRef{Kind: RefNamed, Name: name} }

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

// Facts are the schema's constraints on a property. The emitters do not use
// them yet.
type Facts struct {
	Format    string
	Minimum   *float64
	Maximum   *float64
	MinLength *int
	// Values is the type of an object's additional property values.
	Values *TypeRef
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
	Fill  bool
	Facts Facts
}

// Struct is a generated struct other than a definition's own type.
type Struct struct {
	Name   string
	Doc    []string
	Fields []*Field
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
}

// docLines formats a description as doc comment lines, or nil.
func docLines(desc string) []string {
	if desc == "" {
		return nil
	}
	return util.FormatDocComment(desc)
}

// primitiveRef maps a JSON Schema primitive type to its Go type, or any.
func primitiveRef(t string) TypeRef {
	switch t {
	case schemaTypeString:
		return named("string")
	case schemaTypeInteger:
		return named("int")
	case schemaTypeNumber:
		return named("float64")
	case schemaTypeBoolean:
		return named("bool")
	default:
		return named("any")
	}
}

// pointerPrimitive returns *T for a primitive schema type.
func pointerPrimitive(t string) (TypeRef, bool) {
	switch t {
	case schemaTypeString, schemaTypeInteger, schemaTypeNumber, schemaTypeBoolean:
		return wrap(RefPointer, primitiveRef(t)), true
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
		return primitiveRef(t)
	case schemaTypeArray:
		return wrap(RefSlice, typeRef(d.Items))
	case schemaTypeObject:
		return wrap(RefMap, named("any"))
	default:
		return named("any")
	}
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
		if p, ok := pointerPrimitive(other); ok {
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
			if p, ok := pointerPrimitive(primaryType(nonNull)); ok {
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
	f := Facts{
		Format:           prop.Format,
		Minimum:          prop.Minimum,
		Maximum:          prop.Maximum,
		MinLength:        prop.MinLength,
		DefaultOnError:   prop.XDeserializeDefaultOnError,
		SkipInvalidItems: prop.XDeserializeSkipInvalidItems,
	}
	if prop.AdditionalProperties != nil {
		v := typeRef(prop.AdditionalProperties)
		f.Values = &v
	}
	return f
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
		fld := &Field{Key: key, Name: name, Tag: key, Required: req[key], Doc: docLines(prop.Description), Facts: factsOf(prop)}
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
		fld := &Field{Key: key, Name: name, Tag: key, Required: req[key], Doc: docLines(prop.Description), Facts: factsOf(prop)}
		fld.Type = optionalTypeRef(prop)
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
