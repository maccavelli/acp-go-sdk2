package model

import (
	"fmt"
	"slices"
	"strings"

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
	// GateCheckAdvertisedAuth refuses an id that names no method of the
	// initialize response's list.
	GateCheckAdvertisedAuth
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
	// RawKey is the JSON key read from a raw Unknown: the discriminator's, for
	// a GateCheckUndeclaredVariant whose catch-all is Unknown, which Variant
	// then names and Disc does not; an entry's id, for
	// GateCheckAdvertisedAuth, or empty when its union has no Unknown.
	RawKey string
	// Advertised is, for GateCheckAdvertisedAuth, each variant of List's
	// union, Elem, with its field of IDType, the checked id's type.
	Advertised   []VariantField
	Elem, IDType string
	// Terminal is, for GateCheckAdvertisedAuth, the variant of Advertised
	// that the GateCheckTerminalAuth of the same Field and List names: its
	// ids are refused first, and the list of accepted ids leaves them out,
	// even one another variant also holds. It is empty when the type has no
	// terminal check (0050-PLAN-auth-method-ids-and-the-prompt-result-for-issue-14.md,
	// phase 1).
	Terminal string
	// Items is true for a GateCheckNested of a union's variant that is a
	// named slice or map, Shape's kind, behind the variant's pointer: the
	// gates of each item are called
	// (0049-MADR-remediate-the-2026-11-01-debugging-pass.md, G19, Q25 a).
	Items bool
}

// collectionItems is the named type of the items of an inline variant that is
// a named slice or map, and the collection's kind, or "" for any other
// variant; deep is true when the items are not a named type.
func collectionItems(vi *Variant) (name string, kind RefKind, deep bool) {
	in := vi.Inline
	if vi.Skip || in == nil || in.Kind != InlineNamed || (in.Type.Kind != RefSlice && in.Type.Kind != RefMap) || in.Type.Elem == nil {
		return "", 0, false
	}
	if in.Type.Elem.Kind != RefNamed {
		return "", in.Type.Kind, true
	}
	return in.Type.Elem.Name, in.Type.Kind, false
}

// VariantField is a union's variant, by its Go field, and a Go field of the
// variant's struct.
type VariantField struct {
	Variant, Field string
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
	for _, checks := range own {
		linkTerminal(checks)
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
	reach := m.dispatchReach(holders)
	for _, h := range holders {
		for _, f := range h.fields {
			deep := f.Type.Kind == RefPatch || (f.Type.Kind != RefNamed && f.Type.Elem != nil && f.Type.Elem.Kind != RefNamed)
			if deep && holdsGated(*f.Type.Elem, gated) {
				return fmt.Errorf("%s.%s: %s holds a gated type in a shape gates does not descend into", h.name, f.Key, f.Type)
			}
		}
		for _, vi := range h.variants {
			if _, _, deep := collectionItems(vi); deep && holdsGated(*vi.Inline.Type.Elem, gated) {
				return fmt.Errorf("%s variant %s: %s holds a gated type in a shape gates does not descend into", h.name, vi.FieldName, vi.Inline.Type)
			}
		}
		if gated[h.name] && reach[h.name] {
			m.Gated = append(m.Gated, &Gates{Type: h.name, Union: h.union, Checks: append(own[h.name], h.nested(gated)...)})
		}
	}
	return nil
}

// linkTerminal gives each advertised check of checks, one type's own, the
// variant its property's terminal check names, so that the list of ids the
// advertised check accepts leaves the terminal ones out. Both checks read the
// same union, so the variant is one of Advertised
// (0050-PLAN-auth-method-ids-and-the-prompt-result-for-issue-14.md, phase 1).
func linkTerminal(checks []GateCheck) {
	for i := range checks {
		if checks[i].Kind != GateCheckAdvertisedAuth {
			continue
		}
		for _, t := range checks {
			if t.Kind == GateCheckTerminalAuth && t.Field == checks[i].Field && t.List == checks[i].List {
				checks[i].Terminal = t.Variant
			}
		}
	}
}

// ruleTypes returns the types a capability rule names: its definition's, and
// each copy the merge made of that definition from the unstable schema, or
// from the manifest's fragment of it, under another name, as it does for a
// changed definition an unstable one reaches (0019-PLAN-behaviour-fixes.md,
// B6). It is empty when the schema has none.
func (m *Model) ruleTypes(def string) []*Type {
	var out []*Type
	if t := m.Type(def); t != nil {
		out = append(out, t)
	}
	for _, t := range m.Types {
		if t.Name != def && (t.Origin.File == load.UnstableSchemaFile || t.Origin.File == load.ManifestFile) && t.Origin.Key == def {
			out = append(out, t)
		}
	}
	return out
}

// fieldsHolding returns every generated field built from the property prop:
// a definition's own, and its copies in nested structs and inline variants.
func (m *Model) fieldsHolding(prop *load.Definition) []*Field {
	var out []*Field
	add := func(fields []*Field) {
		for _, f := range fields {
			if f.prop == prop {
				out = append(out, f)
			}
		}
	}
	for _, t := range m.Types {
		add(t.Fields)
		for _, ns := range t.NestedStructs {
			add(ns.Fields)
		}
		for _, vi := range t.Variants {
			if vi.Inline != nil {
				add(vi.Inline.Fields)
			}
		}
	}
	return out
}

// copiesOf names the generated structs, nested and inline variants included,
// with a field of the property prop, in sorted order.
func (m *Model) copiesOf(prop *load.Definition) []string {
	var out []string
	add := func(owner string, fields []*Field) {
		if slices.ContainsFunc(fields, func(f *Field) bool { return f.prop == prop }) {
			out = append(out, owner)
		}
	}
	for _, t := range m.Types {
		add(t.Name, t.Fields)
		for _, ns := range t.NestedStructs {
			add(ns.Name, ns.Fields)
		}
		for _, vi := range t.Variants {
			if vi.Inline != nil {
				add(vi.TypeName, vi.Inline.Fields)
			}
		}
	}
	slices.Sort(out)
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
			// A property the definition holds that is not one of its own
			// fields, as a union's is, lives in copies the rule does not
			// reach: refused, so a rule never gates nothing silently. One the
			// schema lacks applies nothing
			// (0049-MADR-remediate-the-2026-11-01-debugging-pass.md, G20, Q28 a).
			if def := m.schema.Defs[rule.Definition]; def != nil {
				if prop := expandAllOf(m.schema, def).Properties[rule.Property]; prop != nil {
					return c, false, fmt.Errorf("capability rule %s.%s: the property is in copies (%s), not a field of %s",
						rule.Definition, rule.Property, strings.Join(m.copiesOf(prop), ", "), t.Name)
				}
			}
			return c, false, nil
		}
		// The property copied into another struct, as an allOf copies it
		// into an inline variant, is out of the rule's reach there: refused,
		// so a rule never gates part of where its property is
		// (0049-PLAN-generator-fixes.md, phase 6, deviation 2).
		if copies := slices.DeleteFunc(m.copiesOf(f.prop), func(n string) bool { return n == t.Name }); len(copies) > 0 {
			return c, false, fmt.Errorf("capability rule %s.%s: the property is also in copies (%s), which the rule does not reach",
				rule.Definition, rule.Property, strings.Join(copies, ", "))
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
		union, err := m.ruleUnion(&c, t, f)
		if err != nil {
			return c, false, err
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
		union, err := m.ruleUnion(&c, t, f)
		if err != nil {
			return c, false, err
		}
		if err := m.catchAll(&c, union); err != nil {
			return c, false, err
		}
	case GateAdvertisedAuthMethod:
		if err := m.advertisedAuth(&c, f); err != nil {
			return c, false, err
		}
	}
	switch {
	case rule.Kind == GateUndeclaredVariant:
		// The capability is the prefix a refusal names: no capability lifts
		// the gate, so it need only exist.
		if _, _, err := m.capabilityPath(rule.Side, rule.Capability); err != nil {
			return c, false, err
		}
	case rule.Capability != "":
		steps, err := m.capabilitySteps(rule.Side, rule.Capability)
		if err != nil {
			return c, false, err
		}
		c.Steps = steps
	}
	return c, true, nil
}

// ruleUnion returns the union a rule gates: t, when the rule names no
// property, or the union property f holds, by value, pointer or array, whose
// shape it records in c.
func (m *Model) ruleUnion(c *GateCheck, t *Type, f *Field) (*Type, error) {
	if f == nil {
		return t, nil
	}
	ref := f.Type
	if ref.Kind == RefSlice || ref.Kind == RefPointer {
		c.Shape, ref = ref.Kind, *ref.Elem
	}
	union := m.Type(ref.Name)
	if ref.Kind != RefNamed || union == nil {
		return nil, fmt.Errorf("%s is %s, not a union or an array of one", f.Key, f.Type)
	}
	return union, nil
}

// catchAll finds union t's catch-all: a variant and its discriminator's
// field, or the raw Unknown of a discriminated union without one, whose
// discriminator is read by its key.
func (m *Model) catchAll(c *GateCheck, t *Type) error {
	if t.Kind == KindUnion && t.HasUnknown && t.DiscKey != "" {
		c.Kind, c.Variant, c.RawKey = GateCheckUndeclaredVariant, "Unknown", t.DiscKey
		return nil
	}
	if t.Kind != KindUnion || t.CatchAll < 0 {
		return fmt.Errorf("%s is not a union with a catch-all variant or Unknown", t.Name)
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
		if idf := idField(m.variantFields(vi), f.Type.Name); idf != nil {
			c.Kind, c.List, c.Variant, c.ID = GateCheckTerminalAuth, lf.Name, vi.FieldName, idf.Name
			return nil
		}
	}
	return fmt.Errorf("%s has no array of a union whose variant %q holds a %s", m.Initialize.Resp, value, f.Type.Name)
}

// advertisedAuth finds, for the id property f, the initialize response's
// array of a union whose variants hold an id of f's type, and each variant's
// field of it. Every variant must hold one, and a union with Unknown must
// give them one JSON key, which is read from Unknown.
func (m *Model) advertisedAuth(c *GateCheck, f *Field) error {
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
		var ids []VariantField
		keys := map[string]bool{}
		for _, vi := range u.Variants {
			if vi.Skip {
				continue
			}
			idf := idField(m.variantFields(vi), f.Type.Name)
			if idf == nil {
				ids = nil
				break
			}
			ids = append(ids, VariantField{Variant: vi.FieldName, Field: idf.Name})
			keys[idf.Key] = true
		}
		if len(ids) == 0 {
			continue
		}
		c.Kind, c.List, c.Elem, c.IDType, c.Advertised = GateCheckAdvertisedAuth, lf.Name, u.Name, f.Type.Name, ids
		if u.HasUnknown {
			if len(keys) != 1 {
				return fmt.Errorf("%s: its variants' %s ids have %d JSON keys, not one to read from Unknown", u.Name, f.Type.Name, len(keys))
			}
			for k := range keys {
				c.RawKey = k
			}
		}
		return nil
	}
	return fmt.Errorf("%s has no array of a union every variant of which holds a %s", m.Initialize.Resp, f.Type.Name)
}

// idField returns the field of fields whose type is the named type id.
func idField(fields []*Field, id string) *Field {
	for _, f := range fields {
		if f.Type.Kind == RefNamed && f.Type.Name == id {
			return f
		}
	}
	return nil
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
		switch t.Kind {
		case KindUnion:
			var vs []*Variant
			for _, vi := range t.Variants {
				if !vi.Skip {
					vs = append(vs, vi)
				}
			}
			out = append(out, gateHolder{name: t.Name, union: true, variants: vs})
		case KindObject, KindEmptyObject:
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

// dispatchReach returns the types a gate can be called on: each method's
// request, response and notification type, which the dispatch hands to
// gateMessage and gateResult, and every type their fields, members and
// variants hold, at any depth, whose gates a holder's gates call. A gates
// method on any other type is never called, as ConfigOptionUpdate's was not:
// it is reached only by allOf, which the merge flattened into another type
// (0049-PLAN-tooling-fixes.md, phase 5, F-C).
func (m *Model) dispatchReach(holders []gateHolder) map[string]bool {
	by := make(map[string]gateHolder, len(holders))
	for _, h := range holders {
		by[h.name] = h
	}
	reach := map[string]bool{}
	var queue []string
	add := func(name string) {
		if name != "" && !reach[name] {
			reach[name] = true
			queue = append(queue, name)
		}
	}
	for _, methods := range [][]*Method{m.AgentMethods, m.ClientMethods, m.Protocol} {
		for _, me := range methods {
			add(me.Req)
			add(me.Resp)
			add(me.Notif)
		}
	}
	for len(queue) > 0 {
		h, ok := by[queue[0]]
		queue = queue[1:]
		if !ok {
			continue
		}
		for _, f := range h.fields {
			for r := &f.Type; r != nil; r = r.Elem {
				if r.Kind == RefNamed {
					add(r.Name)
				}
			}
		}
		for _, mb := range h.members {
			add(mb.Type)
		}
		for _, vi := range h.variants {
			add(vi.TypeName)
			if items, _, _ := collectionItems(vi); items != "" {
				add(items)
			}
		}
	}
	return reach
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
		case r.Kind != RefNamed && r.Kind != RefPatch && r.Elem != nil && r.Elem.Kind == RefNamed && gated[r.Elem.Name]:
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
		} else if items, kind, _ := collectionItems(vi); items != "" && gated[items] {
			out = append(out, GateCheck{Kind: GateCheckNested, Field: vi.FieldName, Shape: kind, Items: true})
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
