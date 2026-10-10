// Package model resolves the loaded schema into the facts the emitters
// render. Every Go name the generator declares is decided here, once, and a
// schema the generator cannot represent fails Build before any file is written.
package model

import (
	"fmt"
	"maps"
	"slices"
	"strings"

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
	// Version is the protocol version of the meta files.
	Version int

	// schema and meta are the merged inputs.
	schema *load.Schema
	meta   *load.Meta

	// Types holds one entry per definition, in sorted name order, the order
	// types_gen.go prints them.
	Types  []*Type
	byName map[string]*Type

	// AgentMethodConsts and ClientMethodConsts map a meta method key to the
	// name of its constant.
	AgentMethodConsts  map[string]string
	ClientMethodConsts map[string]string

	// AgentMethods and ClientMethods hold the methods of each side, sorted by
	// meta key. Protocol holds the protocol methods, which nothing emits yet.
	AgentMethods  []*Method
	ClientMethods []*Method
	Protocol      []*Method
}

// Input is what Build resolves: the merged meta and schema, and the merge's
// provenance, which may be nil when there is no unstable schema.
type Input struct {
	Meta       *load.Meta
	Schema     *load.Schema
	Provenance *load.Provenance
}

// Type returns the type of the named definition, or nil.
func (m *Model) Type(name string) *Type { return m.byName[name] }

// Type is one schema definition and the names derived from it.
type Type struct {
	Name string
	Kind Kind
	// Unstable is true for a definition the stable schema does not have.
	Unstable bool
	// def is the schema definition the type comes from.
	def *load.Definition
	// Doc is the definition's doc comment, one entry per line.
	Doc []string

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
	// DiscKey is a union's discriminator property, DiscField its Go field
	// name, and DiscSource whether the schema declared it.
	DiscKey    string
	DiscField  string
	DiscSource DiscSource
	// CatchAll is the index of a discriminated union's catch-all variant, the
	// one the schema meant for values it does not list, or -1.
	CatchAll int
	// HasUnknown is true for a discriminated union without a catch-all: it
	// keeps a payload it cannot place in an Unknown field.
	HasUnknown bool
	// Validate is the Validate method the type gets, and Checks the
	// required fields it checks.
	Validate ValidateKind
	Checks   []RequiredCheck
	// Enum holds the values and constant names of the enum kinds.
	Enum []EnumValue
	// Helpers holds the generated constructors of a oneOf union.
	Helpers []*Helper
}

// unionList returns the anyOf or oneOf entries of a KindUnion.
func (t *Type) unionList() []*load.Definition {
	if t.ExactlyOne {
		return t.def.OneOf
	}
	return t.def.AnyOf
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
	// Doc is the variant's description, for its wrapper field.
	Doc []string
	// Required lists the variant's required keys, for decoding by key match.
	Required []string
	// IsObject, IsArray, and IsNull describe the variant's JSON shape.
	IsObject, IsArray, IsNull bool
	// ArrayItemRequired lists, sorted, the required keys of an array
	// variant's object items.
	ArrayItemRequired []string
	// ConstPairs holds the variant's string const properties, sorted by key.
	ConstPairs [][2]string
	// HasSharedFields is true for an inline object variant, which carries
	// the union's own properties.
	HasSharedFields bool
	// KeyLess is true for a variant of a discriminated union that has no
	// discriminator value and is not the catch-all: the schema means it when
	// the discriminator is absent.
	KeyLess bool
}

// DiscSource is where a union's discriminator came from.
type DiscSource int

const (
	// DiscNone is no discriminator.
	DiscNone DiscSource = iota
	// DiscDeclared is the schema's discriminator.propertyName.
	DiscDeclared
	// DiscDerived is the first const property of a variant, in sorted order.
	DiscDerived
)

// ValidateKind is the Validate method a type gets.
type ValidateKind int

const (
	// ValidateNone is no Validate method.
	ValidateNone ValidateKind = iota
	// ValidateFields checks the required string and array fields of an RPC type.
	ValidateFields
	// ValidateToolCall checks ToolCallUpdate's toolCallId.
	ValidateToolCall
	// ValidateExactlyOne checks that a oneOf union has exactly one variant set.
	ValidateExactlyOne
	// ValidateShared checks the union's required properties on the variant set.
	ValidateShared
)

// RequiredCheck is one required string or array field Validate checks.
type RequiredCheck struct {
	Field string
	Prop  string
	// Kind is "string" or "array".
	Kind string
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
	Key   string
	Field string
	Type  TypeRef
}

// helperSkip lists unions whose constructors are hand-written in helpers.go.
var helperSkip = map[string]bool{"ContentBlock": true, "ToolCallContent": true, "SessionUpdate": true}

// fixedNames are package-level names the generator declares whatever the schema holds.
var fixedNames = []string{"ProtocolVersionNumber", "Agent", "AgentExperimental", "Client", "ClientExperimental"}

// Build resolves in.
func Build(in Input) (*Model, error) {
	m, err := resolve(in.Meta, in.Schema, in.Provenance)
	if err != nil {
		return nil, fmt.Errorf("model: %w", err)
	}
	return m, nil
}

// resolve builds the model; Build adds the error prefix.
func resolve(meta *load.Meta, schema *load.Schema, prov *load.Provenance) (*Model, error) {
	if err := checkRefs(schema); err != nil {
		return nil, err
	}
	m := &Model{
		Version:            meta.Version,
		schema:             schema,
		meta:               meta,
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
		t.Unstable = prov != nil && prov.UnstableDefs[name]
		if t.Unstable && t.Kind != KindNone {
			t.Doc = markUnstable(t.Doc)
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
	if err := buildMethods(m, n, prov); err != nil {
		return nil, err
	}
	return m, nil
}

// classify returns how def is emitted. The cases and their order are the
// types emitter's.
func classify(def *load.Definition) Kind {
	if def == nil {
		return KindNone
	}
	switch t := primaryType(def); {
	case len(def.Enum) > 0:
		return KindEnum
	case isStringConstUnion(def):
		return KindStringConstUnion
	case len(def.AnyOf) > 0 && isOpenStringEnum(def):
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
	t := &Type{Name: name, def: def, Kind: classify(def), CatchAll: -1}
	if def != nil {
		t.Doc = docLines(def.Description)
	}
	defer validateOf(t)
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
		u := primitiveRef(primaryType(def))
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
		for _, v := range t.def.Enum {
			values = append(values, fmt.Sprint(v))
		}
	case KindStringConstUnion:
		for _, v := range t.def.OneOf {
			if v != nil && v.Const != nil {
				values = append(values, fmt.Sprint(v.Const))
			}
		}
	case KindOpenEnum:
		for _, v := range t.def.AnyOf {
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
	if err := checkFields(t.Name, t.def.Properties); err != nil {
		return err
	}
	t.Nested = map[string]string{}
	for _, key := range slices.Sorted(maps.Keys(t.def.Properties)) {
		prop := t.def.Properties[key]
		owner := t.Name + ".properties." + key
		if prop == nil {
			return fmt.Errorf("%s: property schema is null", owner)
		}
		// Inline objects: no $ref, type object, with properties.
		if prop.Ref != "" || primaryType(prop) != schemaTypeObject || len(prop.Properties) == 0 {
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
	list := t.unionList()
	discKey, src := discriminatorKey(schema, t.def, list)
	t.DiscKey, t.DiscSource = discKey, src
	if discKey != "" {
		t.DiscField = util.ToExportedField(discKey)
	}
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
	if err := classifyUnion(schema, t, list); err != nil {
		return err
	}
	if !t.ExactlyOne || t.def.DocsIgnore || helperSkip[t.Name] {
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
		ev := expandAllOf(schema, v)
		for _, key := range ev.Required {
			if key == discKey || ev.Properties[key] == nil {
				continue
			}
			if err := checkParam(owner, key); err != nil {
				return err
			}
			h.Params = append(h.Params, Param{Key: key, Field: util.ToExportedField(key), Type: typeRef(ev.Properties[key])})
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
		ref = singleRefVariant(v)
	}
	ev := expandAllOf(schema, v)
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
	vi := &Variant{
		TypeName:        typeName,
		FieldName:       field,
		DiscValue:       dv,
		Doc:             docLines(ev.Description),
		Required:        ev.Required,
		IsObject:        isObj,
		IsArray:         primaryType(ev) == schemaTypeArray,
		IsNull:          isNull,
		ConstPairs:      constPairs(ev),
		HasSharedFields: ref == "" && isObj && !isNull,
	}
	if vi.IsArray {
		vi.ArrayItemRequired = arrayItemRequired(schema, ev)
	}
	// Phantom variants have no $ref, object shape, null type, or title.
	vi.Skip = !isObj && ref == "" && !isNull && ev.Title == ""
	if vi.Skip {
		return vi, nil
	}
	if err := checkExported(field); err != nil {
		return nil, fmt.Errorf("%s: wrapper field: %w", owner, err)
	}
	if ref == "" && isObj && !isNull {
		merged := maps.Clone(t.def.Properties)
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

// constPairs returns the string const properties of v, sorted by key.
func constPairs(v *load.Definition) [][2]string {
	var out [][2]string
	for _, k := range slices.Sorted(maps.Keys(v.Properties)) {
		if pd := v.Properties[k]; pd != nil && pd.Const != nil {
			if c, ok := pd.Const.(string); ok {
				out = append(out, [2]string{k, c})
			}
		}
	}
	return out
}

// arrayItemRequired returns the sorted required keys of an array variant's
// object items, resolving a $ref item.
func arrayItemRequired(schema *load.Schema, v *load.Definition) []string {
	if v.Items == nil {
		return nil
	}
	item := v.Items
	if name, ok := strings.CutPrefix(item.Ref, refPrefix); ok {
		if d := schema.Defs[name]; d != nil {
			item = d
		}
	}
	item = expandAllOf(schema, item)
	if primaryType(item) != schemaTypeObject || len(item.Required) == 0 {
		return nil
	}
	return slices.Sorted(slices.Values(item.Required))
}

// isRPCTypeName reports whether name is a request, response, or notification type.
func isRPCTypeName(name string) bool {
	return strings.HasSuffix(name, "Request") || strings.HasSuffix(name, "Response") || strings.HasSuffix(name, "Notification")
}

// validateOf decides t's Validate method. A union checks its variants; an RPC
// object checks its required string and array fields; ToolCallUpdate checks
// toolCallId.
func validateOf(t *Type) {
	switch {
	case t.Kind == KindNone:
	case t.Kind == KindUnion && t.ExactlyOne:
		t.Validate = ValidateExactlyOne
	case t.Kind == KindUnion:
		t.Validate = ValidateShared
		for _, prop := range slices.Sorted(slices.Values(t.def.Required)) {
			if c, ok := requiredCheck(t.def, prop); ok {
				t.Checks = append(t.Checks, c)
			}
		}
	case t.Name == "ToolCallUpdate":
		t.Validate = ValidateToolCall
		t.Checks = []RequiredCheck{{Field: "ToolCallId", Prop: "toolCallId", Kind: schemaTypeString}}
	case primaryType(t.def) == schemaTypeObject && isRPCTypeName(t.Name):
		t.Validate = ValidateFields
		for _, prop := range slices.Sorted(maps.Keys(t.def.Properties)) {
			if !slices.Contains(t.def.Required, prop) {
				continue
			}
			if c, ok := requiredCheck(t.def, prop); ok {
				t.Checks = append(t.Checks, c)
			}
		}
	}
}

// requiredCheck returns the check for a required string or array property.
func requiredCheck(def *load.Definition, prop string) (RequiredCheck, bool) {
	pd := def.Properties[prop]
	if pd == nil {
		return RequiredCheck{}, false
	}
	switch kind := primaryType(pd); kind {
	case schemaTypeString, schemaTypeArray:
		return RequiredCheck{Field: util.ToExportedField(prop), Prop: prop, Kind: kind}, true
	}
	return RequiredCheck{}, false
}

// primaryType returns the first non-null JSON Schema type of d, looking
// through allOf when d has no type of its own.
func primaryType(d *load.Definition) string {
	if d == nil {
		return ""
	}
	if d.Type == nil {
		// Properties are sometimes wrapped in allOf (commonly around a single
		// $ref); an inline type there still counts.
		for _, e := range d.AllOf {
			if e == nil {
				continue
			}
			if t := primaryType(e); t != "" {
				return t
			}
		}
		return ""
	}
	switch v := d.Type.(type) {
	case string:
		return v
	case []any:
		var first string
		for _, e := range v {
			if s, ok := e.(string); ok {
				if first == "" {
					first = s
				}
				if s != schemaTypeNull {
					return s
				}
			}
		}
		return first
	default:
		return ""
	}
}

// classifyUnion finds a discriminated union's catch-all and key-less variants
// and decides whether it keeps payloads it cannot place in Unknown. Every
// variant of a discriminated union must be an object.
func classifyUnion(schema *load.Schema, t *Type, list []*load.Definition) error {
	if t.DiscKey == "" {
		return nil
	}
	for idx, v := range list {
		vi := t.Variants[idx]
		if vi.Skip {
			continue
		}
		owner := fmt.Sprintf("%s variant %d", t.Name, idx+1)
		if !variantIsObject(schema, v, vi) {
			return fmt.Errorf("%s: a variant of a union discriminated by %q must be an object", owner, t.DiscKey)
		}
		if vi.DiscValue != "" {
			continue
		}
		if !isCatchAll(schema, v, t.DiscKey) {
			vi.KeyLess = true
			continue
		}
		if t.CatchAll >= 0 {
			return fmt.Errorf("%s: variant %d is already the catch-all", owner, t.CatchAll+1)
		}
		t.CatchAll = idx
		if vi.Inline != nil {
			for _, f := range vi.Inline.Fields {
				if f.Name == "Extra" {
					return fmt.Errorf("%s: property %q becomes field Extra, which the catch-all needs", owner, f.Key)
				}
			}
			vi.Inline.KeepExtra = true
		}
	}
	t.HasUnknown = t.CatchAll < 0
	if t.HasUnknown {
		for _, vi := range t.Variants {
			if !vi.Skip && vi.FieldName == "Unknown" {
				return fmt.Errorf("%s: variant %s takes the field Unknown, which the union needs", t.Name, vi.TypeName)
			}
		}
	}
	return nil
}

// isCatchAll reports whether v is the schema's catch-all for a union
// discriminated by key: an inline variant whose key is a required string
// without a constant, and which excludes the listed values with not.
func isCatchAll(schema *load.Schema, v *load.Definition, key string) bool {
	if v.Ref != "" || singleRefVariant(v) != "" {
		return false
	}
	ev := expandAllOf(schema, v)
	p := ev.Properties[key]
	return p != nil && p.Const == nil && primaryType(p) == schemaTypeString && slices.Contains(ev.Required, key) && ev.Not != nil
}

// variantIsObject reports whether a union variant decodes from a JSON object.
func variantIsObject(schema *load.Schema, v *load.Definition, vi *Variant) bool {
	if vi.IsArray || vi.IsNull {
		return false
	}
	ref := v.Ref
	if ref == "" {
		ref = singleRefVariant(v)
	}
	d := v
	if name, ok := strings.CutPrefix(ref, refPrefix); ok {
		d = schema.Defs[name]
	}
	d = expandAllOf(schema, d)
	return d != nil && (primaryType(d) == schemaTypeObject || len(d.Properties) > 0)
}
