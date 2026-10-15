package model

import (
	"fmt"
	"math"
	"strconv"
)

// Validation is what a type's Validate method checks: nil means the type has
// no Validate. Validate checks what the schema states and nothing else
// (0012-MADR-schema-fidelity-framework.md): a union has exactly one variant
// set, and the set one validates; a field meets its minLength and any bound
// outside its Go type's range; and every field whose type validates does.
type Validation struct {
	Checks []Check
	// Members are the struct's flattened members: exactly one must be set,
	// and the set one validates when its type does.
	Members []*Member
}

// CheckKind is what one Check verifies.
type CheckKind int

const (
	// CheckNested calls the field's own Validate.
	CheckNested CheckKind = iota + 1
	// CheckMinLength compares the field's length in characters with Bound.
	CheckMinLength
	// CheckMinimum requires the field to be at least Bound.
	CheckMinimum
	// CheckMaximum requires the field to be at most Bound.
	CheckMaximum
	// CheckAbsolutePath requires the field to be an absolute path, by the
	// rule the caller of validate passes (0015-MADR-validate-absolute-protocol-paths.md).
	CheckAbsolutePath
)

// Check is one check of a Validate method, in field order.
type Check struct {
	Kind       CheckKind
	Field, Key string
	// Shape is how the field holds the checked value: RefNamed for a value,
	// RefPointer, RefSlice, or RefMap.
	Shape RefKind
	// Bound is the minLength, minimum, or maximum; Integer reports whether the
	// field is an integer, so the bound is written without a fraction.
	Bound   float64
	Integer bool
	// InPatch is true for a nested check of a Patch field: Shape is that of
	// the patch's value, checked when the patch holds one.
	InPatch bool
}

// goRange is the range of each sized Go number type.
var goRange = map[string][2]float64{
	goInt8: {math.MinInt8, math.MaxInt8}, goInt16: {math.MinInt16, math.MaxInt16},
	goInt32: {math.MinInt32, math.MaxInt32}, goInt64: {math.MinInt64, math.MaxInt64},
	goUint8: {0, math.MaxUint8}, goUint16: {0, math.MaxUint16}, goUint32: {0, math.MaxUint32}, goUint64: {0, math.MaxUint64},
}

// Validates reports whether the named Go type has a Validate method.
func (m *Model) Validates(name string) bool { return m.validates[name] }

// shape is one generated struct the validation pass visits.
type shape struct {
	name    string
	fields  []*Field
	rpc     bool
	set     func(*Validation)
	members []*Member
}

// buildValidations decides which types have Validate and what each checks:
// every union and RPC object, every struct with a bound or minLength check,
// and, to a fixed point, every struct with a field whose type validates. It
// refuses a constraint on a named primitive type and a validated type held in
// a shape Validate does not descend into.
func (m *Model) buildValidations() error {
	m.validates = map[string]bool{}
	structs, err := m.validationShapes()
	if err != nil {
		return err
	}
	own, err := m.ownChecks(structs)
	if err != nil {
		return err
	}
	if err := m.spreadValidates(structs); err != nil {
		return err
	}
	for _, t := range m.Types {
		for _, vi := range t.Variants {
			vi.Validates = !vi.Skip && m.validates[vi.TypeName]
		}
	}
	for _, s := range structs {
		if !m.validates[s.name] {
			continue
		}
		v := &Validation{Members: s.members}
		for _, mb := range s.members {
			mb.Validates = m.validates[mb.Type]
		}
		for _, f := range s.fields {
			for _, c := range own[s.name] {
				if c.Key == f.Key {
					v.Checks = append(v.Checks, c)
				}
			}
			if sh, ok, err := m.nested(s.name, f); err != nil {
				return err
			} else if ok {
				v.Checks = append(v.Checks, Check{Kind: CheckNested, Field: f.Name, Key: f.Key, Shape: sh, InPatch: f.Type.Kind == RefPatch})
			}
		}
		s.set(v)
	}
	return nil
}

// validationShapes marks every union as validating and lists the generated
// structs: object types, nested structs, and inline variant structs. A named
// primitive type with a bound its Go type does not enforce is refused.
func (m *Model) validationShapes() ([]shape, error) {
	var structs []shape
	for _, t := range m.Types {
		switch {
		case t.Kind == KindUnion:
			m.validates[t.Name] = true
			t.Validation = &Validation{}
		case t.Kind == KindPrimitive && (t.def.Minimum != nil || t.def.Maximum != nil || t.def.MinLength != nil):
			if own, err := m.bounds(t.Name, "", t.def.Minimum, t.def.Maximum, t.def.MinLength, *t.Underlying); err != nil || len(own) > 0 {
				return nil, fmt.Errorf("%s: a bound on a named primitive type is not checked", t.Name)
			}
		case t.Kind == KindObject || t.Kind == KindEmptyObject:
			structs = append(structs, shape{t.Name, t.Fields, isRPCTypeName(t.Name), func(v *Validation) { t.Validation = v }, t.Members})
		}
		for _, ns := range t.NestedStructs {
			structs = append(structs, shape{ns.Name, ns.Fields, false, func(v *Validation) { ns.Validation = v }, nil})
		}
		for _, vi := range t.Variants {
			if in := vi.Inline; in != nil && in.Kind == InlineStruct {
				structs = append(structs, shape{vi.TypeName, in.Fields, false, func(v *Validation) { in.Validation = v }, in.Members})
			}
		}
	}
	// A struct with flattened members validates: exactly one must be set.
	for _, s := range structs {
		if len(s.members) > 0 {
			m.validates[s.name] = true
		}
	}
	return structs, nil
}

// ownChecks returns each struct's bound and minLength checks, and marks a
// struct that has one, or is an RPC object, as validating.
func (m *Model) ownChecks(structs []shape) (map[string][]Check, error) {
	own := map[string][]Check{}
	for _, s := range structs {
		for _, f := range s.fields {
			c, err := m.bounds(s.name, f.Key, f.Facts.Minimum, f.Facts.Maximum, f.Facts.MinLength, f.Type)
			if err != nil {
				return nil, err
			}
			if m.absProps[f.prop] {
				shape, ok := pathFieldShape(f.Type)
				if !ok {
					return nil, fmt.Errorf("%s: %s: an absolute path must be a string, a pointer to one, or a slice of them, not %s", s.name, f.Key, f.Type)
				}
				c = append(c, Check{Kind: CheckAbsolutePath, Key: f.Key, Shape: shape})
			}
			for i := range c {
				c[i].Field = f.Name
			}
			own[s.name] = append(own[s.name], c...)
		}
		if s.rpc || len(own[s.name]) > 0 {
			m.validates[s.name] = true
		}
	}
	return own, nil
}

// spreadValidates marks, until nothing changes, every struct with a field
// whose type validates.
func (m *Model) spreadValidates(structs []shape) error {
	for changed := true; changed; {
		changed = false
		for _, s := range structs {
			if m.validates[s.name] {
				continue
			}
			for _, f := range s.fields {
				_, ok, err := m.nested(s.name, f)
				if err != nil {
					return err
				}
				if ok {
					m.validates[s.name], changed = true, true
					break
				}
			}
		}
	}
	return nil
}

// nested reports whether field f holds a type that validates, and in which
// shape. A validated type deeper than one pointer, slice, or map is refused.
// A Patch is looked through: the shape is that of its value.
func (m *Model) nested(owner string, f *Field) (RefKind, bool, error) {
	r := f.Type
	if r.Kind == RefPatch {
		r = *r.Elem
	}
	if r.Kind == RefNamed {
		return RefNamed, m.validates[r.Name], nil
	}
	if r.Elem == nil {
		return 0, false, nil
	}
	if r.Elem.Kind == RefNamed {
		return r.Kind, m.validates[r.Elem.Name], nil
	}
	if m.holdsValidated(*r.Elem) {
		return 0, false, fmt.Errorf("%s.%s: %s holds a validated type in a shape Validate does not descend into", owner, f.Key, r)
	}
	return 0, false, nil
}

// holdsValidated reports whether r holds, at any depth, a type that validates.
func (m *Model) holdsValidated(r TypeRef) bool {
	if r.Kind == RefNamed {
		return m.validates[r.Name]
	}
	return r.Elem != nil && m.holdsValidated(*r.Elem)
}

// bounds returns the checks a property's minimum, maximum, and minLength need:
// none for a bound its Go type already enforces. A bound on a type it cannot
// apply to is refused.
func (m *Model) bounds(owner, key string, minimum, maximum *float64, minLength *int, r TypeRef) ([]Check, error) {
	sh := RefNamed
	if r.Kind == RefPointer {
		sh, r = RefPointer, *r.Elem
	}
	where := owner
	if key != "" {
		where += "." + key
	}
	base := r.Name
	if t := m.Type(base); t != nil && t.Underlying != nil {
		base = t.Underlying.Name
	}
	var out []Check
	if minLength != nil {
		if r.Kind != RefNamed || base != schemaTypeString {
			return nil, fmt.Errorf("%s: minLength on %s, not a string", where, r)
		}
		out = append(out, Check{Kind: CheckMinLength, Key: key, Shape: sh, Bound: float64(*minLength), Integer: true})
	}
	if minimum == nil && maximum == nil {
		return out, nil
	}
	rng, integer := goRange[base]
	if r.Kind != RefNamed || (!integer && base != goFloat32 && base != goFloat64) {
		return nil, fmt.Errorf("%s: minimum or maximum on %s, not a number", where, r)
	}
	if !integer {
		if minimum != nil {
			out = append(out, Check{Kind: CheckMinimum, Key: key, Shape: sh, Bound: *minimum})
		}
		if maximum != nil {
			out = append(out, Check{Kind: CheckMaximum, Key: key, Shape: sh, Bound: *maximum})
		}
		return out, nil
	}
	// An integer bound rounds inward, so a fractional one admits exactly the
	// integers it allows; one the Go type cannot reach would leave no value,
	// and the type checker would refuse the generated comparison
	// (0019-PLAN-generator-hardening.md, G12 and G21).
	if minimum != nil {
		lo := math.Ceil(*minimum)
		if lo > rng[1] {
			return nil, fmt.Errorf("%s: minimum %s is out of range for %s", where, formatBound(*minimum), base)
		}
		if lo > rng[0] {
			out = append(out, Check{Kind: CheckMinimum, Key: key, Shape: sh, Bound: lo, Integer: true})
		}
	}
	if maximum != nil {
		hi := math.Floor(*maximum)
		if hi < rng[0] {
			return nil, fmt.Errorf("%s: maximum %s is out of range for %s", where, formatBound(*maximum), base)
		}
		if hi < rng[1] {
			out = append(out, Check{Kind: CheckMaximum, Key: key, Shape: sh, Bound: hi, Integer: true})
		}
	}
	return out, nil
}

// formatBound writes a schema bound as the schema does, without a needless
// fraction or exponent.
func formatBound(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// pathFieldShape is how a field holds an absolute path: a string, a pointer
// to one, or a slice of them.
func pathFieldShape(r TypeRef) (RefKind, bool) {
	switch {
	case r.Kind == RefNamed && r.Name == schemaTypeString:
		return RefNamed, true
	case (r.Kind == RefPointer || r.Kind == RefSlice) && r.Elem != nil && r.Elem.Kind == RefNamed && r.Elem.Name == schemaTypeString:
		return r.Kind, true
	}
	return 0, false
}
