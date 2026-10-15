// Package model resolves the loaded schema into the facts the emitters
// render. Every Go name the generator declares is decided here, once, and a
// schema the generator cannot represent fails Build before any file is written.
package model

import (
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"slices"
	"strconv"
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
	// KindAny is a definition the model gives no Go type, emitted as an alias
	// of json.RawMessage.
	KindAny
	// KindIntegerEnum is an anyOf or oneOf of titled integer consts and plain
	// integers: a named integer type with a constant per const.
	KindIntegerEnum
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

	// AbsolutePaths are the protocol table's absolute-path rules that apply
	// to this schema, in the table's order; absProps holds their properties.
	AbsolutePaths []PathProperty
	absProps      map[*load.Definition]bool
	// patchProps holds the properties the protocol table's patch rules name.
	patchProps map[*load.Definition]bool
	// Gated lists, in type order, each type with a gates method and what it
	// checks (0017-MADR-remediate-acp-v1-conformance-violations.md, G1).
	Gated []*Gates

	// AgentMethodConsts, ClientMethodConsts, and ProtocolMethodConsts map a
	// meta method key to the name of its constant.
	AgentMethodConsts    map[string]string
	ClientMethodConsts   map[string]string
	ProtocolMethodConsts map[string]string

	// AgentMethods and ClientMethods hold the methods of each side, sorted by
	// meta key. Protocol holds the protocol methods, which have constants
	// and no interface.
	AgentMethods  []*Method
	ClientMethods []*Method
	Protocol      []*Method
	// Initialize is the agent method that negotiates the connection, from
	// the protocol table's initialize rule, or nil.
	Initialize *Method
	// Interfaces are the Go interfaces the methods are called through, in
	// binding order: the SDK's four and each protocol-table interface a
	// method binds to.
	Interfaces []Interface

	// validates holds the Go types that have a Validate method.
	validates map[string]bool
}

// Input is what Build resolves: the merged meta and schema, the merge's
// provenance, which may be nil when there is no unstable schema, and the
// protocol table, which may be nil for none.
type Input struct {
	Meta       *load.Meta
	Schema     *load.Schema
	Provenance *load.Provenance
	Protocol   *Protocol
}

// Type returns the type of the named definition, or nil.
func (m *Model) Type(name string) *Type { return m.byName[name] }

// Type is one schema definition and the names derived from it.
type Type struct {
	Name string
	Kind Kind
	// Unstable is true for a definition the stable schema does not have.
	Unstable bool
	// Origin is the file and key of the definition the type was built from,
	// in the schema directory.
	Origin load.Origin
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
	// Members are a KindObject's flattened members: a definition with
	// properties of its own and an anyOf or oneOf whose members are all
	// references (0011-MADR-harden-code-generator-engine.md, "Found in step 7
	// and decided").
	Members []*Member
	// KeepExtra is true for a KindObject whose additionalProperties is the
	// boolean schema true: the struct keeps the keys it does not declare, so
	// the payload encodes back with them (0019-PLAN-generator-hardening.md,
	// G2).
	KeepExtra bool
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
	// Validation is what the type's Validate checks; nil is no Validate.
	Validation *Validation
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
	// Validates is true when the variant's type has a Validate method, which
	// the union's Validate calls.
	Validates bool
	// KeyLess is true for a variant of a discriminated union that has no
	// discriminator value and is not the catch-all: the schema means it when
	// the discriminator is absent.
	KeyLess bool
	// FitKeys lists, in schema order, a variant's required keys other than
	// its union's discriminator: a payload without the discriminator fits
	// the variant when it carries them all. Nil outside a discriminated union.
	FitKeys []string
	// Excluded lists, sorted, the discriminator values a catch-all's not
	// excludes, which its union's Validate refuses in it. Nil for any other
	// variant (0014-MADR-union-constructors-take-every-required-key.md, C1).
	Excluded []string
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

// EnumValue is one constant of an enum kind.
type EnumValue struct {
	Value string
	Const string
}

// Helper is a New<Union><Field> constructor for one union variant.
type Helper struct {
	Name    string
	Variant int
	// Params are the required keys of the Go type that holds the variant,
	// in schema order, without the discriminator: for an inline variant the
	// union's own keys, then the variant's; for a $ref variant those of the
	// definition it names (paramKeys); a catch-all's discriminator among them.
	// For a Value constructor, the one parameter value (valueParam).
	Params []Param
	// Value is true for a constructor that takes the variant's value: a
	// variant that is not an object, null, or untyped.
	Value bool
}

// Param is one constructor parameter: Name is the Go name paramName gives
// its property key.
type Param struct {
	Key   string
	Name  string
	Field string
	Type  TypeRef
}

// fixedNames are package-level names the generator declares whatever the schema holds.
var fixedNames = []string{"ProtocolVersionNumber", "Agent", "AgentExperimental", "Client", "ClientExperimental"}

// Build resolves in.
func Build(in Input) (*Model, error) {
	m, err := resolve(in.Meta, in.Schema, in.Provenance, in.Protocol)
	if err != nil {
		return nil, fmt.Errorf("model: %w", err)
	}
	return m, nil
}

// resolve builds the model; Build adds the error prefix.
func resolve(meta *load.Meta, schema *load.Schema, prov *load.Provenance, proto *Protocol) (*Model, error) {
	if err := checkRefs(schema); err != nil {
		return nil, err
	}
	r, err := newRules(proto)
	if err != nil {
		return nil, fmt.Errorf(tablePrefix+"%w", err)
	}
	m := &Model{
		Version:              meta.Version,
		schema:               schema,
		meta:                 meta,
		byName:               map[string]*Type{},
		AgentMethodConsts:    map[string]string{},
		ClientMethodConsts:   map[string]string{},
		ProtocolMethodConsts: map[string]string{},
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
		t, err := buildType(schema, n, name, schema.Defs[name], r)
		if err != nil {
			return nil, err
		}
		t.Unstable = prov != nil && prov.UnstableDefs[name]
		t.Origin = load.Origin{File: load.StableSchemaFile, Key: name}
		if prov != nil {
			if o, ok := prov.Origins[name]; ok {
				t.Origin = o
			}
		}
		if t.Unstable && t.Kind != KindNone {
			t.Doc = markUnstable(t.Doc)
		}
		m.Types = append(m.Types, t)
		m.byName[name] = t
	}
	m.dropStructOmitempty()
	if err := m.resolveAbsolutePaths(r); err != nil {
		return nil, err
	}
	if err := m.resolvePatchFields(r); err != nil {
		return nil, err
	}
	if err := m.resolveNullRetained(r); err != nil {
		return nil, err
	}
	if err := m.checkDefaults(); err != nil {
		return nil, err
	}
	if err := m.buildValidations(); err != nil {
		return nil, err
	}
	if err := m.checkFoldedKeys(); err != nil {
		return nil, err
	}
	for _, side := range []struct {
		prefix  string
		methods map[string]string
		consts  map[string]string
	}{
		{"AgentMethod", meta.AgentMethods, m.AgentMethodConsts},
		{"ClientMethod", meta.ClientMethods, m.ClientMethodConsts},
		{"ProtocolMethod", meta.ProtocolMethods, m.ProtocolMethodConsts},
	} {
		for _, key := range slices.Sorted(maps.Keys(side.methods)) {
			c := side.prefix + util.ToExportedField(key)
			if err := n.declare(c, fmt.Sprintf("method %q", side.methods[key])); err != nil {
				return nil, err
			}
			side.consts[key] = c
		}
	}
	if err := buildMethods(m, prov, r); err != nil {
		return nil, err
	}
	for _, step := range []func() error{
		func() error { return m.linkPromptTurns(r) },
		func() error { return m.linkCancelledRequests(r) },
		func() error { return m.linkInitialize(r) },
		func() error { return m.checkCapabilities(r) },
		func() error { return m.resolveGates(r) },
		func() error { return m.checkTypeRules(r) },
		func() error { return m.buildInterfaces(n, r) },
	} {
		if err := step(); err != nil {
			return nil, fmt.Errorf(tablePrefix+"%w", err)
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
	switch t := primaryType(def); {
	case len(def.Enum) > 0:
		return KindEnum
	case isStringConstUnion(def):
		return KindStringConstUnion
	case len(def.AnyOf) > 0 && isOpenStringEnum(def):
		return KindOpenEnum
	case isIntegerEnum(def):
		return KindIntegerEnum
	case len(def.AnyOf) > 0, len(def.OneOf) > 0:
		return KindUnion
	case (t == schemaTypeObject || t == "") && len(def.Properties) > 0:
		// Properties without a type mean an object, as they do for a
		// variant (0019-PLAN-generator-hardening.md, G6).
		return KindObject
	case t == schemaTypeString || t == "integer" || t == "number" || t == "boolean":
		return KindPrimitive
	case t == schemaTypeObject:
		return KindEmptyObject
	default:
		return KindAny
	}
}

func buildType(schema *load.Schema, n *namer, name string, def *load.Definition, r *rules) (*Type, error) {
	t := &Type{Name: name, def: def, Kind: classify(def), CatchAll: -1}
	if def != nil {
		t.Doc = docLines(def.Description)
	}
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
		if members, ok := flatMembers(schema, def); ok {
			t.Kind, t.Members = KindObject, members
			if err := buildObject(n, t); err != nil {
				return nil, err
			}
			if err := checkMembers(name, t.Fields, members); err != nil {
				return nil, err
			}
			if err := checkRequired(schema, t); err != nil {
				return nil, err
			}
			break
		}
		t.ExactlyOne = len(def.AnyOf) == 0
		if err := buildUnion(schema, n, t, r); err != nil {
			return nil, err
		}
	case KindObject:
		if err := buildObject(n, t); err != nil {
			return nil, err
		}
		if err := checkRequired(schema, t); err != nil {
			return nil, err
		}
	case KindPrimitive:
		u := primitiveRef(primaryType(def), def.Format)
		t.Underlying = &u
	case KindIntegerEnum:
		if err := buildIntegerEnum(n, t); err != nil {
			return nil, err
		}
	}
	return t, nil
}

// isIntegerEnum reports whether def is an anyOf or oneOf whose members are
// all integers, at least one of them a const.
func isIntegerEnum(def *load.Definition) bool {
	list := def.AnyOf
	if len(list) == 0 {
		list = def.OneOf
	}
	consts := 0
	for _, v := range list {
		if v == nil || primaryType(v) != schemaTypeInteger || v.Ref != "" || len(v.Properties) > 0 || len(v.AllOf) > 0 {
			return false
		}
		if v.Const != nil {
			consts++
		}
	}
	return consts > 0
}

// buildIntegerEnum gives an integer enum the Go type of its members' format
// and a constant per const, named by the member's title. Members of different
// formats, a const without a title, and a const that is not an integer of the
// format (integerConst) are refused.
func buildIntegerEnum(n *namer, t *Type) error {
	list := t.def.AnyOf
	if len(list) == 0 {
		list = t.def.OneOf
	}
	format := list[0].Format
	for i, v := range list {
		if v.Format != format {
			return fmt.Errorf("%s: members of formats %s and %s have no one Go type", t.Name, numericFormat(schemaTypeInteger, format), numericFormat(schemaTypeInteger, v.Format))
		}
		if v.Const == nil {
			continue
		}
		value, err := integerConst(v.Const, format)
		if err != nil {
			return fmt.Errorf("%s variant %d: %w", t.Name, i+1, err)
		}
		if v.Title == "" {
			return fmt.Errorf("%s variant %d: integer const %s has no title to name its constant", t.Name, i+1, value)
		}
		c := util.ToEnumConst(t.Name, v.Title)
		if err := n.declare(c, fmt.Sprintf("%s value %s", t.Name, value)); err != nil {
			return err
		}
		t.Enum = append(t.Enum, EnumValue{Value: value, Const: c})
	}
	u := primitiveRef(schemaTypeInteger, format)
	t.Underlying = &u
	return nil
}

// integerConst returns an integer enum member's const as a Go integer literal.
// The loader reads a JSON number as float64, so the const must be integral,
// below 2^53 in magnitude, where a float64 cannot have rounded it, and
// within the range of the format's Go type
// (0011-MADR-harden-code-generator-engine.md, "Found in step 8 and decided").
func integerConst(c any, format string) (string, error) {
	shown, err := json.Marshal(c)
	if err != nil {
		shown = []byte(fmt.Sprint(c))
	}
	f, ok := c.(float64)
	if !ok || f != math.Trunc(f) {
		return "", fmt.Errorf("const %s is not an integer", shown)
	}
	if math.Abs(f) >= 1<<53 {
		return "", fmt.Errorf("const %s is 2^53 or more in magnitude, which the schema loader does not read exactly", shown)
	}
	goType := numericFormat(schemaTypeInteger, format)
	if r, ok := goRange[goType]; ok && (f < r[0] || f > r[1]) {
		return "", fmt.Errorf("const %s is outside %s", shown, goType)
	}
	return strconv.FormatFloat(f+0, 'f', -1, 64), nil
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
		if admitsExtra(prop.AdditionalProperties) {
			return fmt.Errorf("%s: additionalProperties on an inline object is not kept; declare it as a definition", owner)
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
	return keepExtra(t)
}

// keepExtra sets t.KeepExtra when t's additionalProperties is the boolean
// schema true, and refuses what the struct could not keep: a typed
// additionalProperties, flattened members, whose keys Extra would hold a
// second time, and a property that would take the field Extra
// (0019-PLAN-generator-hardening.md, G2).
func keepExtra(t *Type) error {
	ap := t.def.AdditionalProperties
	if !admitsExtra(ap) {
		return nil
	}
	if v, ok := ap.BoolSchema(); !ok || !v {
		return fmt.Errorf("%s: a typed additionalProperties on an object with properties is not kept; only true is", t.Name)
	}
	if len(t.Members) > 0 {
		return fmt.Errorf("%s: additionalProperties cannot keep the keys of an object with flattened members", t.Name)
	}
	for _, f := range t.Fields {
		if f.Name == "Extra" {
			return fmt.Errorf("%s: property %q becomes field Extra, which additionalProperties needs", t.Name, f.Key)
		}
	}
	t.KeepExtra = true
	return nil
}

// discriminatorValue returns the const a variant's facts give its union's
// discriminator, or "" for none. A const that is not a string is refused: the
// union matches and sends the discriminator as a string
// (0019-PLAN-generator-hardening.md, G14).
func discriminatorValue(owner string, facts *load.Definition, discKey string) (string, error) {
	if discKey == "" {
		return "", nil
	}
	pd := facts.Properties[discKey]
	if pd == nil || pd.Const == nil {
		return "", nil
	}
	s, ok := pd.Const.(string)
	if !ok {
		return "", fmt.Errorf("%s: discriminator %s is not a string", owner, schemaValue(pd.Const))
	}
	return s, nil
}

// checkRequired refuses a required key that no property of the object, of its
// flattened members, or of an inline object property declares: nothing
// would read it, so its absence would go unchecked
// (0019-PLAN-generator-hardening.md, G11).
func checkRequired(schema *load.Schema, t *Type) error {
	have := map[string]bool{}
	for key := range t.def.Properties {
		have[key] = true
	}
	for _, mb := range t.Members {
		for key := range expandAllOf(schema, schema.Defs[mb.Type]).Properties {
			have[key] = true
		}
	}
	if err := requiredDeclared(t.Name, t.def.Required, have); err != nil {
		return err
	}
	for _, key := range slices.Sorted(maps.Keys(t.Nested)) {
		prop := t.def.Properties[key]
		nested := map[string]bool{}
		for k := range prop.Properties {
			nested[k] = true
		}
		if err := requiredDeclared(t.Name+".properties."+key, prop.Required, nested); err != nil {
			return err
		}
	}
	return nil
}

// requiredDeclared fails on the first key of required not in have.
func requiredDeclared(owner string, required []string, have map[string]bool) error {
	for _, key := range required {
		if !have[key] {
			return fmt.Errorf("%s: required key %q is not a property", owner, key)
		}
	}
	return nil
}

// checkUnionExtra refuses an additionalProperties a union would not keep: on
// a union with keys of its own, and on an inline variant other than the
// catch-all, which alone keeps the keys it does not declare
// (0019-PLAN-generator-hardening.md, G2).
func checkUnionExtra(t *Type, list []*load.Definition) error {
	if len(t.def.Properties) > 0 && admitsExtra(t.def.AdditionalProperties) {
		return fmt.Errorf("%s: additionalProperties on a union with keys of its own is not kept", t.Name)
	}
	for idx, v := range list {
		vi := t.Variants[idx]
		if vi.Skip || vi.Inline == nil || vi.Inline.Kind != InlineStruct || idx == t.CatchAll {
			continue
		}
		if admitsExtra(v.AdditionalProperties) {
			return fmt.Errorf("%s variant %d: additionalProperties on a variant other than the catch-all is not kept", t.Name, idx+1)
		}
	}
	return nil
}

// admitsExtra reports whether an additionalProperties admits keys an object
// does not declare: it is present and not the boolean schema false.
func admitsExtra(ap *load.Definition) bool {
	if ap == nil {
		return false
	}
	v, ok := ap.BoolSchema()
	return !ok || v
}

// buildUnion names every variant of t, in schema order, and its helpers.
func buildUnion(schema *load.Schema, n *namer, t *Type, r *rules) error {
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
	if err := checkOwnKeys(t, list); err != nil {
		return err
	}
	if err := checkUnionExtra(t, list); err != nil {
		return err
	}
	if t.def.DocsIgnore || r.helpers[t.Name] {
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
		if typ, ok := valueParam(schema, v, vi); ok {
			h.Value = true
			h.Params = []Param{{Name: "value", Type: typ}}
			t.Helpers = append(t.Helpers, h)
			continue
		}
		keys, props := paramKeys(schema, t, v, vi)
		named := map[string]string{}
		for _, key := range keys {
			// The discriminator is a parameter unless the constructor writes the
			// variant's constant: a catch-all's, or a key-less variant's that
			// requires it (P1).
			if (key == discKey && vi.DiscValue != "") || props[key] == nil {
				continue
			}
			name, err := paramName(owner, key)
			if err != nil {
				return err
			}
			if prev, ok := named[name]; ok {
				return fmt.Errorf("%s: parameters %s and %s are both named %s", owner, prev, key, name)
			}
			named[name] = key
			h.Params = append(h.Params, Param{Key: key, Name: name, Field: util.ToExportedField(key), Type: typeRef(props[key])})
		}
		t.Helpers = append(t.Helpers, h)
	}
	return nil
}

// checkOwnKeys refuses a union with properties of its own and a variant that
// is a reference: the variant is emitted as the definition it names, which
// does not hold them, so they would be dropped
// (0014-MADR-union-constructors-take-every-required-key.md, R1).
func checkOwnKeys(t *Type, list []*load.Definition) error {
	if len(t.def.Properties) == 0 {
		return nil
	}
	for idx, v := range list {
		if v == nil || (v.Ref == "" && singleRefVariant(v) == "") {
			continue
		}
		return fmt.Errorf("%s variant %d: %s cannot hold the union's own properties %s",
			t.Name, idx+1, t.Variants[idx].TypeName, strings.Join(slices.Sorted(maps.Keys(t.def.Properties)), ", "))
	}
	return nil
}

// valueParam reports whether variant v's constructor takes the variant's
// value, and the type of that parameter: for a variant that is not an
// object, null, or untyped, the type the variant's type is declared with;
// for a $ref to a named primitive, its underlying type; for a $ref to any
// other definition that is not an object, that definition's own type
// (0014-MADR-union-constructors-take-every-required-key.md, V3, and item 5).
func valueParam(schema *load.Schema, v *load.Definition, vi *Variant) (TypeRef, bool) {
	if vi.IsObject || vi.IsNull {
		return TypeRef{}, false
	}
	if vi.Inline != nil {
		if vi.Inline.Kind != InlineNamed || primaryType(expandAllOf(schema, v)) == schemaTypeObject {
			return TypeRef{}, false
		}
		return vi.Inline.Type, true
	}
	ref := v.Ref
	if ref == "" {
		ref = singleRefVariant(v)
	}
	name := strings.TrimPrefix(ref, refPrefix)
	target := schema.Defs[name]
	if target == nil {
		return TypeRef{}, false
	}
	switch classify(target) {
	case KindObject, KindEmptyObject, KindNone:
		return TypeRef{}, false
	case KindPrimitive:
		return primitiveRef(primaryType(target), target.Format), true
	default:
		return named(name), true
	}
}

// paramKeys returns the required keys of the Go type that holds variant v,
// in schema order and each once, and the properties that type them: for an
// inline object variant, the union's own keys, then the variant's, the
// variant's property winning as in inlineOf; for a $ref variant, or an allOf
// of one $ref, the keys of the definition it names
// (0014-MADR-union-constructors-take-every-required-key.md, P1).
func paramKeys(schema *load.Schema, t *Type, v *load.Definition, vi *Variant) ([]string, map[string]*load.Definition) {
	ev := expandAllOf(schema, v)
	if target, ok := strings.CutPrefix(v.Ref, refPrefix); ok && schema.Defs[target] != nil {
		ev = expandAllOf(schema, schema.Defs[target])
	}
	keys, props := ev.Required, ev.Properties
	if vi.HasSharedFields {
		keys = append(slices.Clone(t.def.Required), ev.Required...)
		props = map[string]*load.Definition{}
		maps.Copy(props, t.def.Properties)
		maps.Copy(props, ev.Properties)
	}
	var out []string
	for _, key := range keys {
		if !slices.Contains(out, key) {
			out = append(out, key)
		}
	}
	return out, props
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
	// A variant that is a $ref takes its keys, discriminator value, and shape
	// from the definition it names; its own title and description still name
	// and document it (0011-MADR-harden-code-generator-engine.md, "Found in step
	// 9 and decided").
	facts := ev
	if target, ok := strings.CutPrefix(v.Ref, refPrefix); ok && schema.Defs[target] != nil {
		facts = expandAllOf(schema, schema.Defs[target])
	}
	s, ok := facts.Type.(string)
	isNull := ok && s == schemaTypeNull

	dv, err := discriminatorValue(owner, facts, discKey)
	if err != nil {
		return nil, err
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

	isObj := len(facts.Properties) > 0
	vi := &Variant{
		TypeName:        typeName,
		FieldName:       field,
		DiscValue:       dv,
		Doc:             docLines(ev.Description),
		Required:        facts.Required,
		IsObject:        isObj,
		IsArray:         primaryType(facts) == schemaTypeArray,
		IsNull:          isNull,
		ConstPairs:      constPairs(facts),
		HasSharedFields: ref == "" && isObj && !isNull,
	}
	if vi.IsArray {
		vi.ArrayItemRequired = arrayItemRequired(schema, facts)
	}
	// A const on a variant that is not an object reaches here only beside a
	// non-string const (a oneOf of string consts, or of integer consts, is an
	// enum kind), and no generated code would check it
	// (0019-PLAN-generator-hardening.md, G20).
	if ref == "" && !isObj && ev.Const != nil {
		return nil, fmt.Errorf("%s: const %s on a union variant is not enforced; only a oneOf of string consts or of titled integer consts has a Go type", owner, schemaValue(ev.Const))
	}
	// A variant with no $ref, object shape, null type, or title has no name
	// for its field and would match nothing
	// (0019-PLAN-generator-hardening.md, G8).
	if !isObj && ref == "" && !isNull && ev.Title == "" {
		return nil, fmt.Errorf("%s: a variant without a title, $ref, properties, or null type has no Go name", owner)
	}
	if err := checkExported(field); err != nil {
		return nil, fmt.Errorf("%s: wrapper field: %w", owner, err)
	}
	if err := checkNotMethod(field); err != nil {
		return nil, fmt.Errorf("%s: wrapper %w", owner, err)
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
		if len(v.AllOf) == 1 && v.AllOf[0] != nil && in.Kind == InlineStruct {
			if base, ok := strings.CutPrefix(v.AllOf[0].Ref, refPrefix); ok {
				if members, ok := flatMembers(schema, schema.Defs[base]); ok {
					if err := checkMembers(typeName, in.Fields, members); err != nil {
						return nil, err
					}
					in.Members = members
				}
			}
		}
	}
	return vi, nil
}

// flatMembers returns the members of def when it has properties of its own
// and an anyOf or oneOf whose members are all references, a $ref or an allOf
// of one $ref: one per member, named by its title, or by the referenced
// definition without one.
func flatMembers(schema *load.Schema, def *load.Definition) ([]*Member, bool) {
	if def == nil || len(def.Properties) == 0 {
		return nil, false
	}
	list := def.AnyOf
	if len(list) == 0 {
		list = def.OneOf
	}
	if len(list) == 0 {
		return nil, false
	}
	var out []*Member
	for _, v := range list {
		if v == nil {
			return nil, false
		}
		ref := v.Ref
		if ref == "" {
			ref = singleRefVariant(v)
		}
		name, ok := strings.CutPrefix(ref, refPrefix)
		if !ok || schema.Defs[name] == nil {
			return nil, false
		}
		field := util.ToExportedField(name)
		if v.Title != "" {
			field = util.ToExportedField(v.Title)
		}
		out = append(out, &Member{Field: field, Type: name, Doc: docLines(v.Description), Required: expandAllOf(schema, schema.Defs[name]).Required})
	}
	return out, true
}

// checkMembers fails when a member's field is not an exported identifier, or
// is the name of a field or of another member of the struct.
func checkMembers(owner string, fields []*Field, members []*Member) error {
	seen := map[string]string{}
	for _, f := range fields {
		seen[f.Name] = "property " + f.Key
	}
	for _, mb := range members {
		if err := checkExported(mb.Field); err != nil {
			return fmt.Errorf("%s: member %s: %w", owner, mb.Type, err)
		}
		if err := checkNotMethod(mb.Field); err != nil {
			return fmt.Errorf("%s: member %s: %w", owner, mb.Type, err)
		}
		if prev, ok := seen[mb.Field]; ok {
			return fmt.Errorf("%s: member %s and %s both become field %s", owner, mb.Type, prev, mb.Field)
		}
		seen[mb.Field] = "member " + mb.Type
	}
	return nil
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

// classifyUnion finds a discriminated union's catch-all and key-less variants,
// lists each variant's keys for decoding without the discriminator, and
// decides whether the union keeps payloads it cannot place in Unknown. Every
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
		if vi.Inline != nil {
			vi.Inline.Exempt = t.DiscKey
		}
		if !variantIsObject(schema, v, vi) {
			return fmt.Errorf("%s: a variant of a union discriminated by %q must be an object", owner, t.DiscKey)
		}
		for _, k := range vi.Required {
			if k != t.DiscKey {
				vi.FitKeys = append(vi.FitKeys, k)
			}
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
		vi.Excluded = excludedValues(expandAllOf(schema, v).Not, t.DiscKey)
		if len(vi.Excluded) == 0 {
			// Validate could not refuse a value the catch-all's not excludes
			// (0014-PLAN-union-constructors-take-every-required-key.md, step 5).
			return fmt.Errorf("%s: the catch-all's not excludes no value of %q", owner, t.DiscKey)
		}
		if vi.Inline != nil {
			// The catch-all keeps every key no field declares, which would hold
			// the members' keys a second time.
			if len(vi.Inline.Members) > 0 {
				base := strings.TrimPrefix(v.AllOf[0].Ref, refPrefix)
				return fmt.Errorf("%s: a catch-all cannot hold the members of %s", owner, base)
			}
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

// excludedValues returns, sorted and each once, the string const values of
// key in not and in each member of its anyOf or oneOf.
func excludedValues(not *load.Definition, key string) []string {
	if not == nil {
		return nil
	}
	var out []string
	for _, d := range slices.Concat([]*load.Definition{not}, not.AnyOf, not.OneOf) {
		if d == nil || d.Properties[key] == nil {
			continue
		}
		if s, ok := d.Properties[key].Const.(string); ok && !slices.Contains(out, s) {
			out = append(out, s)
		}
	}
	slices.Sort(out)
	return out
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

// PathProperty is an absolute-path rule that applies to the schema: the Go
// type of its definition, where the definition comes from, and the property.
type PathProperty struct {
	Type     string
	Origin   load.Origin
	Property string
}

// resolveAbsolutePaths resolves each absolute-path rule of the table to its
// property's definition. A rule whose definition or property the schema
// lacks is skipped, as the method rules are; one whose property is not a
// string, an optional string, or an array of strings is refused
// (0015-PLAN-validate-absolute-protocol-paths.md, step 4).
func (m *Model) resolveAbsolutePaths(r *rules) error {
	m.absProps = map[*load.Definition]bool{}
	if r.table == nil {
		return nil
	}
	for _, rule := range r.table.AbsolutePaths {
		t := m.Type(rule.Definition)
		def := m.schema.Defs[rule.Definition]
		if t == nil || def == nil {
			continue
		}
		prop := expandAllOf(m.schema, def).Properties[rule.Property]
		if prop == nil {
			continue
		}
		if !isPathShape(prop) {
			return fmt.Errorf("%s.%s: an absolute-path rule needs a string, an optional string, or an array of strings", rule.Definition, rule.Property)
		}
		m.absProps[prop] = true
		m.AbsolutePaths = append(m.AbsolutePaths, PathProperty{Type: t.Name, Origin: t.Origin, Property: rule.Property})
	}
	return nil
}

// resolvePatchFields resolves each patch rule of the table to its property's
// definition, and makes every field built from that property, in every type
// that holds it, an inline allOf variant included, a Patch of the type it
// had, its pointer removed, omitted when absent. A rule whose definition or
// property the schema lacks is skipped; one on a required property, or one
// with a default, is refused
// (0021-MADR-resolve-issues-4-and-5-before-v1-14-0.md, issue #5).
func (m *Model) resolvePatchFields(r *rules) error {
	m.patchProps = map[*load.Definition]bool{}
	if r.table == nil {
		return nil
	}
	for _, rule := range r.table.PatchFields {
		def := m.schema.Defs[rule.Definition]
		if m.Type(rule.Definition) == nil || def == nil {
			continue
		}
		ed := expandAllOf(m.schema, def)
		prop := ed.Properties[rule.Property]
		if prop == nil {
			continue
		}
		switch {
		case slices.Contains(ed.Required, rule.Property):
			return fmt.Errorf("%s.%s: a patch rule needs an optional property", rule.Definition, rule.Property)
		case prop.Default != nil:
			return fmt.Errorf("%s.%s: a patch rule needs a property without a default", rule.Definition, rule.Property)
		case !prop.XDeserializeDefaultOnError:
			// The patch decoders read a malformed value as absent, which only
			// x-deserialize-default-on-error allows.
			return fmt.Errorf("%s.%s: a patch rule needs x-deserialize-default-on-error", rule.Definition, rule.Property)
		}
		m.patchProps[prop] = true
	}
	patch := func(fields []*Field) {
		for _, f := range fields {
			if !m.patchProps[f.prop] {
				continue
			}
			elem := f.Type
			if elem.Kind == RefPointer {
				elem = *elem.Elem
			}
			f.Type, f.Tag, f.Patch = wrap(RefPatch, elem), f.Key+",omitzero", true
		}
	}
	for _, t := range m.Types {
		patch(t.Fields)
		for _, ns := range t.NestedStructs {
			patch(ns.Fields)
		}
		for _, vi := range t.Variants {
			if vi.Inline != nil {
				patch(vi.Inline.Fields)
			}
		}
	}
	return nil
}

// resolveNullRetained resolves each null-retained rule of the table to its
// property's definition, and makes every field built from that property keep
// a JSON null, where another optional raw field reads it as absent. A rule
// whose definition or property the schema lacks is skipped; one on a required
// or non-raw property, or one whose source the property's description does
// not state, is refused (0019-PLAN-generator-hardening.md, G3).
func (m *Model) resolveNullRetained(r *rules) error {
	if r.table == nil {
		return nil
	}
	props := map[*load.Definition]string{}
	for _, rule := range r.table.NullRetained {
		def := m.schema.Defs[rule.Definition]
		if m.Type(rule.Definition) == nil || def == nil {
			continue
		}
		ed := expandAllOf(m.schema, def)
		prop := ed.Properties[rule.Property]
		if prop == nil {
			continue
		}
		owner := rule.Definition + "." + rule.Property
		switch {
		case slices.Contains(ed.Required, rule.Property):
			return fmt.Errorf("%s: a null-retained rule needs an optional property", owner)
		case !strings.Contains(prop.Description, rule.Source):
			return fmt.Errorf("%s: its description does not state %q", owner, rule.Source)
		}
		props[prop] = owner
	}
	retain := func(fields []*Field) error {
		for _, f := range fields {
			owner, ok := props[f.prop]
			if !ok {
				continue
			}
			if f.Type.Kind != RefRaw {
				return fmt.Errorf("%s: a null-retained rule needs a raw property, not %s", owner, f.Type)
			}
			f.NullIsAbsent = false
		}
		return nil
	}
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
			if err := retain(fields); err != nil {
				return err
			}
		}
	}
	return nil
}

// isPathShape reports whether a property is a string, a string or null, or
// an array of strings.
func isPathShape(prop *load.Definition) bool {
	switch primaryType(prop) {
	case schemaTypeString:
		return len(prop.Enum) == 0 && prop.Const == nil
	case schemaTypeArray:
		return prop.Items != nil && primaryType(prop.Items) == schemaTypeString
	}
	return false
}
