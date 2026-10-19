package model

import "strings"

// resolveConstFields sets each object variant's ConstFields: the Go field of
// each of its ConstPairs, when every pair names a string field of the
// variant's struct that the constant can be assigned to and encodes as it is.
// A union then encodes the variant once, with the constants set on a copy
// (0019-PLAN-idioms-performance.md, D1). It leaves ConstFields nil when one
// does not, and the union encodes that variant through a map.
func (m *Model) resolveConstFields() {
	for _, t := range m.Types {
		for _, vi := range t.Variants {
			if len(vi.ConstPairs) == 0 {
				continue
			}
			var fields []*Field
			if in := vi.Inline; in != nil && in.Kind == InlineStruct {
				fields = in.Fields
			} else if rt := m.Type(vi.TypeName); rt != nil {
				fields = rt.Fields
			}
			vi.ConstFields = m.constFields(fields, vi.ConstPairs)
		}
	}
}

// constFields returns the field of each pair, or nil when a pair names no
// field, or a field that is not a string, or an empty constant its field
// would omit.
func (m *Model) constFields(fields []*Field, pairs [][2]string) []string {
	out := make([]string, 0, len(pairs))
	for _, kv := range pairs {
		var f *Field
		for _, fld := range fields {
			if fld.Key == kv[0] {
				f = fld
				break
			}
		}
		if f == nil || !m.isStringType(f.Type) || (kv[1] == "" && strings.HasSuffix(f.Tag, ",omitempty")) {
			return nil
		}
		out = append(out, f.Name)
	}
	return out
}

// isStringType reports whether r is string, or a named type over string that
// an untyped string constant is assigned to: an enum kind, or a primitive.
func (m *Model) isStringType(r TypeRef) bool {
	if r.Kind != RefNamed {
		return false
	}
	if r.Name == schemaTypeString {
		return true
	}
	t := m.Type(r.Name)
	if t == nil {
		return false
	}
	switch t.Kind {
	case KindEnum, KindStringConstUnion, KindOpenEnum:
		return true
	case KindPrimitive:
		return t.Underlying != nil && t.Underlying.Kind == RefNamed && t.Underlying.Name == schemaTypeString
	}
	return false
}
