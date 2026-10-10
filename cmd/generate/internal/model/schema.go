package model

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/load"
)

const (
	refPrefix        = "#/$defs/"
	schemaTypeObject = "object"
	schemaTypeString = "string"
	schemaTypeNull   = "null"
)

// expandAllOf merges JSON Schema allOf nodes into a shallow composite definition.
//
// ACP's schema frequently uses `allOf: [{"$ref": "#/$defs/Type"}]` so a property or union
// variant can attach local metadata (description/default/const constraints) while reusing
// a referenced shape.
//
// For codegen purposes we approximate allOf by merging object properties + required lists.
// We intentionally do not attempt to resolve semantic constraints beyond that.
func expandAllOf(schema *load.Schema, d *load.Definition) *load.Definition {
	if d == nil || len(d.AllOf) == 0 {
		return d
	}

	merged := *d
	// Avoid re-processing in recursive calls.
	merged.AllOf = nil
	merged.Properties = maps.Clone(d.Properties)
	merged.Required = slices.Clone(d.Required)
	reqSet := requiredSet(merged.Required)

	for _, part := range d.AllOf {
		if part == nil {
			continue
		}
		resolved := part
		if name, ok := strings.CutPrefix(part.Ref, refPrefix); ok && schema != nil {
			if def := schema.Defs[name]; def != nil {
				resolved = def
			}
		}
		resolved = expandAllOf(schema, resolved)

		copyMissingAllOfFields(&merged, resolved)

		if len(resolved.Properties) > 0 {
			if merged.Properties == nil {
				merged.Properties = make(map[string]*load.Definition)
			}
			for k, v := range resolved.Properties {
				if _, ok := merged.Properties[k]; !ok {
					merged.Properties[k] = v
				}
			}
		}
		for _, r := range resolved.Required {
			if !reqSet[r] {
				merged.Required = append(merged.Required, r)
				reqSet[r] = true
			}
		}
	}

	return &merged
}

func copyMissingAllOfFields(merged *load.Definition, resolved *load.Definition) {
	if merged.Type == nil && resolved.Type != nil {
		merged.Type = resolved.Type
	}
	if merged.Items == nil && resolved.Items != nil {
		merged.Items = resolved.Items
	}
	if merged.Ref == "" && resolved.Ref != "" {
		merged.Ref = resolved.Ref
	}
	if len(merged.Enum) == 0 && len(resolved.Enum) > 0 {
		merged.Enum = resolved.Enum
	}
	if len(merged.AnyOf) == 0 && len(resolved.AnyOf) > 0 {
		merged.AnyOf = resolved.AnyOf
	}
	if len(merged.OneOf) == 0 && len(resolved.OneOf) > 0 {
		merged.OneOf = resolved.OneOf
	}
}

// requiredSet returns the names in required as a set.
func requiredSet(required []string) map[string]bool {
	set := make(map[string]bool, len(required))
	for _, r := range required {
		set[r] = true
	}
	return set
}

// singleRefVariant returns the $ref when v is only an allOf wrapper around one $ref.
func singleRefVariant(v *load.Definition) string {
	if v.Type == nil &&
		len(v.Properties) == 0 &&
		len(v.Required) == 0 &&
		len(v.Enum) == 0 &&
		v.Items == nil &&
		len(v.AnyOf) == 0 &&
		len(v.OneOf) == 0 &&
		len(v.AllOf) == 1 &&
		v.AllOf[0] != nil &&
		v.AllOf[0].Ref != "" {
		return v.AllOf[0].Ref
	}
	return ""
}

// discriminatorKey returns the schema's discriminator property, or the first
// const property found on a variant, in sorted property order, and which of
// the two it is.
func discriminatorKey(schema *load.Schema, parent *load.Definition, variants []*load.Definition) (string, DiscSource) {
	if parent != nil && parent.Discriminator != nil && parent.Discriminator.PropertyName != "" {
		return parent.Discriminator.PropertyName, DiscDeclared
	}
	for _, v := range variants {
		if v == nil {
			continue
		}
		v = expandAllOf(schema, v)
		for _, k := range slices.Sorted(maps.Keys(v.Properties)) {
			if pd := v.Properties[k]; pd != nil && pd.Const != nil {
				return k, DiscDerived
			}
		}
	}
	return "", DiscNone
}

// isStringConstUnion reports whether def is a oneOf whose variants are all string consts.
func isStringConstUnion(def *load.Definition) bool {
	if def == nil || len(def.OneOf) == 0 {
		return false
	}
	for _, v := range def.OneOf {
		if v == nil || v.Const == nil {
			return false
		}
		if _, ok := v.Const.(string); !ok {
			return false
		}
	}
	return true
}

// isOpenStringEnum reports whether def is an "open" string enum: an `anyOf`
// where every variant is a plain string schema (type: "string") and at least
// one variant has a string `const`. Such schemas describe a string value with
// a recommended set of known values while still permitting arbitrary strings
// for extensibility (see e.g. SessionConfigOptionCategory).
//
// These are emitted as a Go string-derived type with named constants for each
// known value, rather than as a discriminated union wrapper.
func isOpenStringEnum(def *load.Definition) bool {
	if def == nil || len(def.AnyOf) == 0 {
		return false
	}
	sawConst := false
	for _, v := range def.AnyOf {
		if v == nil {
			return false
		}
		// Each variant must be a plain string schema: type "string", no nested
		// structure (properties, items, refs, nested unions, allOf, etc.).
		if primaryType(v) != schemaTypeString {
			return false
		}
		if v.Ref != "" || len(v.Properties) > 0 || v.Items != nil ||
			len(v.AnyOf) > 0 || len(v.OneOf) > 0 || len(v.AllOf) > 0 ||
			len(v.Enum) > 0 {
			return false
		}
		if v.Const != nil {
			if _, ok := v.Const.(string); !ok {
				return false
			}
			sawConst = true
		}
	}
	return sawConst
}

// checkRefs fails on a $ref that is not local or names no definition, and on
// an integer or number whose format has no Go type (keywords.go).
func checkRefs(schema *load.Schema) error {
	for _, name := range slices.Sorted(maps.Keys(schema.Defs)) {
		if err := walkRefs(schema, name, schema.Defs[name]); err != nil {
			return err
		}
	}
	return nil
}

func walkRefs(schema *load.Schema, path string, d *load.Definition) error {
	if d == nil {
		return nil
	}
	if d.Ref != "" {
		target, ok := strings.CutPrefix(d.Ref, refPrefix)
		if !ok {
			return fmt.Errorf("%s: $ref %q is not a reference into $defs", path, d.Ref)
		}
		if schema.Defs[target] == nil {
			return fmt.Errorf("%s: $ref %q names no definition", path, d.Ref)
		}
	}
	if typ := primaryType(d); (typ == schemaTypeInteger || typ == schemaTypeNumber) && numericFormat(typ, d.Format) == "" {
		return fmt.Errorf("%s: format %q on %s has no Go type", path, d.Format, typ)
	}
	children := []schemaNode{
		{path + ".items", d.Items},
		{path + ".additionalProperties", d.AdditionalProperties},
		{path + ".not", d.Not},
		{path + ".unevaluatedProperties", d.UnevaluatedProperties},
	}
	for _, k := range slices.Sorted(maps.Keys(d.Properties)) {
		children = append(children, schemaNode{path + ".properties." + k, d.Properties[k]})
	}
	for _, kw := range []struct {
		name string
		list []*load.Definition
	}{{"anyOf", d.AnyOf}, {"oneOf", d.OneOf}, {"allOf", d.AllOf}} {
		for i, v := range kw.list {
			children = append(children, schemaNode{fmt.Sprintf("%s.%s[%d]", path, kw.name, i), v})
		}
	}
	for _, c := range children {
		if err := walkRefs(schema, c.path, c.def); err != nil {
			return err
		}
	}
	return nil
}

// schemaNode is a definition and its path from the top of the schema.
type schemaNode struct {
	path string
	def  *load.Definition
}
