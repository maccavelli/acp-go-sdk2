package model

import (
	"fmt"

	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/load"
)

// Gates is what one type's gates method checks against the record of
// initialize: its own rules, then each field, member, or variant whose type
// has gates (0017-MADR-remediate-acp-v1-conformance-violations.md, G1).
type Gates struct {
	Type   string
	Union  bool
	Checks []GateCheck
}

// GateCheckKind is what one GateCheck verifies.
type GateCheckKind int

const (
	// GateCheckNested calls the gates of the field's type.
	GateCheckNested GateCheckKind = iota + 1
	// GateCheckNonEmpty refuses a non-empty array without the capability.
	GateCheckNonEmpty
	// GateCheckVariant refuses a set variant without the capability.
	GateCheckVariant
	// GateCheckTerminalAuth refuses an id that names a method of the
	// initialize response's list in the variant.
	GateCheckTerminalAuth
	// GateCheckUndeclaredVariant refuses the union's catch-all variant, set.
	GateCheckUndeclaredVariant
)

// GateCheck is one check of a gates method.
type GateCheck struct {
	Kind GateCheckKind
	// Field and Key are the Go field and JSON key checked; both are empty for
	// a variant of the union itself.
	Field, Key string
	// Shape is how the field holds what is checked: RefNamed for a value,
	// RefPointer, RefSlice, or RefMap.
	Shape RefKind
	// Variant is the union's Go field of the gated variant, and Value its
	// discriminator value.
	Variant, Value string
	// Capability is the capability path, and Steps its Go fields from the
	// capabilities of the side that advertises it: the client's, in the
	// record's request, when FromRequest; the agent's, in its response,
	// otherwise.
	Capability  string
	Steps       []CapabilityStep
	FromRequest bool
	// List is the initialize response's field holding the methods, and ID
	// the variant's field the checked property names, for GateCheckTerminalAuth.
	List, ID string
	// Disc is the catch-all variant's Go field holding the discriminator
	// value, for GateCheckUndeclaredVariant, whose Variant is the catch-all's
	// field and whose Capability is the prefix of the capability it names.
	Disc string
}

// gateHolder is one generated type a gates method may sit on: a struct with
// its fields and members, or a union with its variants.
type gateHolder struct {
	name     string
	union    bool
	fields   []*Field
	members  []*Member
	variants []*Variant
}

// resolveGates resolves each capability rule of the table to a check on its
// definition, then gives gates to every type that holds a checked one,
// directly or through fields, members, or variants, as buildValidations does
// for Validate. Without the initialize rule there is no record to check
// against, and no rule applies. A rule whose definition or property the
// schema lacks is skipped; one whose subject has the wrong shape, or whose
// capability the schema lacks, is refused.
func (m *Model) resolveGates(r *rules) error {
	m.Gated = nil
	if r.table == nil || len(r.table.CapabilityRules) == 0 {
		return nil
	}
	own := map[string][]GateCheck{}
	for _, rule := range r.table.CapabilityRules {
		if m.Initialize == nil {
			continue
		}
		for _, t := range m.ruleTypes(rule.Definition) {
			c, ok, err := m.gateCheck(t, rule)
			if err != nil {
				return fmt.Errorf("%s: %w", rule.name(), err)
			}
			if ok {
				own[t.Name] = append(own[t.Name], c)
			}
		}
	}
	holders := m.gateHolders()
	gated := map[string]bool{}
	for name := range own {
		gated[name] = true
	}
	for changed := true; changed; {
		changed = false
		for _, h := range holders {
			if gated[h.name] {
				continue
			}
			if len(h.nested(gated)) > 0 {
				gated[h.name], changed = true, true
			}
		}
	}
	for _, h := range holders {
		for _, f := range h.fields {
			if f.Type.Kind != RefNamed && f.Type.Elem != nil && f.Type.Elem.Kind != RefNamed && holdsGated(*f.Type.Elem, gated) {
				return fmt.Errorf("%s.%s: %s holds a gated type in a shape gates does not descend into", h.name, f.Key, f.Type)
			}
		}
		if gated[h.name] {
			m.Gated = append(m.Gated, &Gates{Type: h.name, Union: h.union, Checks: append(own[h.name], h.nested(gated)...)})
		}
	}
	return nil
}

// ruleTypes returns the types a capability rule names: its definition's, and
// each copy the merge made of that definition from the unstable schema under
// another name, as it does for a changed definition an unstable one reaches
// (0019-PLAN-behaviour-fixes.md, B6). It is empty when the schema has none.
func (m *Model) ruleTypes(def string) []*Type {
	var out []*Type
	if t := m.Type(def); t != nil {
		out = append(out, t)
	}
	for _, t := range m.Types {
		if t.Name != def && t.Origin.File == load.UnstableSchemaFile && t.Origin.Key == def {
			out = append(out, t)
		}
	}
	return out
}

// gateCheck resolves one rule on its definition t; false when the schema
// lacks the rule's property.
func (m *Model) gateCheck(t *Type, rule CapabilityRule) (GateCheck, bool, error) {
	c := GateCheck{Capability: rule.Capability, Value: rule.Variant, FromRequest: rule.Side == sideClient}
	var f *Field
	if rule.Property != "" {
		for _, x := range t.Fields {
			if x.Key == rule.Property {
				f = x
			}
		}
		if f == nil {
			return c, false, nil
		}
		c.Field, c.Key, c.Shape = f.Name, f.Key, RefNamed
	}
	switch rule.Kind {
	case GateNonEmpty:
		if f.Type.Kind != RefSlice {
			return c, false, fmt.Errorf("%s is %s, not an array", rule.Property, f.Type)
		}
		c.Kind, c.Shape = GateCheckNonEmpty, RefSlice
	case GateVariant:
		union := t
		if f != nil {
			ref := f.Type
			if ref.Kind == RefSlice || ref.Kind == RefPointer {
				c.Shape, ref = ref.Kind, *ref.Elem
			}
			union = m.Type(ref.Name)
			if ref.Kind != RefNamed || union == nil {
				return c, false, fmt.Errorf("%s is %s, not a union or an array of one", rule.Property, f.Type)
			}
		}
		vi, err := variantOf(union, rule.Variant)
		if err != nil {
			return c, false, err
		}
		c.Kind, c.Variant = GateCheckVariant, vi.FieldName
	case GateNotTerminalAuthMethod:
		if err := m.terminalAuth(&c, f, rule.Variant); err != nil {
			return c, false, err
		}
	case GateUndeclaredVariant:
		if err := m.catchAll(&c, t); err != nil {
			return c, false, err
		}
	}
	if rule.Capability != "" {
		steps, err := m.capabilitySteps(rule.Side, rule.Capability)
		if err != nil {
			return c, false, err
		}
		c.Steps = steps
	}
	return c, true, nil
}

// catchAll finds union t's catch-all variant and its discriminator's field.
func (m *Model) catchAll(c *GateCheck, t *Type) error {
	if t.Kind != KindUnion || t.CatchAll < 0 {
		return fmt.Errorf("%s is not a union with a catch-all variant", t.Name)
	}
	vi := t.Variants[t.CatchAll]
	for _, f := range m.variantFields(vi) {
		if f.Key == t.DiscKey && f.Type.Kind == RefNamed && f.Type.Name == "string" {
			c.Kind, c.Variant, c.Disc = GateCheckUndeclaredVariant, vi.FieldName, f.Name
			return nil
		}
	}
	return fmt.Errorf("%s: the catch-all variant has no string %s", t.Name, t.DiscKey)
}

// variantOf returns the variant of union t whose discriminator value is value.
func variantOf(t *Type, value string) (*Variant, error) {
	if t.Kind != KindUnion {
		return nil, fmt.Errorf("%s is not a union", t.Name)
	}
	for _, vi := range t.Variants {
		if !vi.Skip && vi.DiscValue == value {
			return vi, nil
		}
	}
	return nil, fmt.Errorf("%s has no variant %q", t.Name, value)
}

// terminalAuth finds, for the id property f, the initialize response's array
// of a union with the variant value, and that variant's field of f's type.
func (m *Model) terminalAuth(c *GateCheck, f *Field, value string) error {
	if f.Type.Kind != RefNamed {
		return fmt.Errorf("%s is %s, not an id", f.Key, f.Type)
	}
	for _, lf := range m.Type(m.Initialize.Resp).Fields {
		if lf.Type.Kind != RefSlice || lf.Type.Elem.Kind != RefNamed {
			continue
		}
		u := m.Type(lf.Type.Elem.Name)
		if u == nil || u.Kind != KindUnion {
			continue
		}
		vi, err := variantOf(u, value)
		if err != nil {
			continue
		}
		fields := m.variantFields(vi)
		for _, idf := range fields {
			if idf.Type.Kind == RefNamed && idf.Type.Name == f.Type.Name {
				c.Kind, c.List, c.Variant, c.ID = GateCheckTerminalAuth, lf.Name, vi.FieldName, idf.Name
				return nil
			}
		}
	}
	return fmt.Errorf("%s has no array of a union whose variant %q holds a %s", m.Initialize.Resp, value, f.Type.Name)
}

// variantFields returns the fields of a variant's struct.
func (m *Model) variantFields(vi *Variant) []*Field {
	if vi.Inline != nil {
		return vi.Inline.Fields
	}
	if t := m.Type(vi.TypeName); t != nil {
		return t.Fields
	}
	return nil
}

// gateHolders lists the generated structs and unions, as validationShapes
// lists the structs Validate may sit on.
func (m *Model) gateHolders() []gateHolder {
	var out []gateHolder
	for _, t := range m.Types {
		switch {
		case t.Kind == KindUnion:
			var vs []*Variant
			for _, vi := range t.Variants {
				if !vi.Skip {
					vs = append(vs, vi)
				}
			}
			out = append(out, gateHolder{name: t.Name, union: true, variants: vs})
		case t.Kind != KindNone && primaryType(t.def) == schemaTypeObject:
			out = append(out, gateHolder{name: t.Name, fields: t.Fields, members: t.Members})
		}
		for _, ns := range t.NestedStructs {
			out = append(out, gateHolder{name: ns.Name, fields: ns.Fields})
		}
		for _, vi := range t.Variants {
			if in := vi.Inline; in != nil && in.Kind == InlineStruct {
				out = append(out, gateHolder{name: vi.TypeName, fields: in.Fields, members: in.Members})
			}
		}
	}
	return out
}

// nested returns the holder's calls into the gates of what it holds: each
// field whose type has gates, by value or one pointer, slice, or map deep;
// each member; each variant.
func (h gateHolder) nested(gated map[string]bool) []GateCheck {
	var out []GateCheck
	for _, f := range h.fields {
		r := f.Type
		switch {
		case r.Kind == RefNamed && gated[r.Name]:
			out = append(out, GateCheck{Kind: GateCheckNested, Field: f.Name, Key: f.Key, Shape: RefNamed})
		case r.Kind != RefNamed && r.Elem != nil && r.Elem.Kind == RefNamed && gated[r.Elem.Name]:
			out = append(out, GateCheck{Kind: GateCheckNested, Field: f.Name, Key: f.Key, Shape: r.Kind})
		}
	}
	for _, mb := range h.members {
		if gated[mb.Type] {
			out = append(out, GateCheck{Kind: GateCheckNested, Field: mb.Field, Shape: RefPointer})
		}
	}
	for _, vi := range h.variants {
		if gated[vi.TypeName] {
			out = append(out, GateCheck{Kind: GateCheckNested, Field: vi.FieldName, Shape: RefPointer})
		}
	}
	return out
}

// holdsGated reports whether r holds, at any depth, a type with gates.
func holdsGated(r TypeRef, gated map[string]bool) bool {
	if r.Kind == RefNamed {
		return gated[r.Name]
	}
	return r.Elem != nil && holdsGated(*r.Elem, gated)
}
