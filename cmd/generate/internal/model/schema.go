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

// checkRefs fails on a $ref that is not local or names no definition, on an
// integer or number whose format has no Go type (keywords.go), on
// x-deserialize-skip-invalid-items on a node that is not an array, and on a
// shape the model would type with loss (checkTypable).
func checkRefs(schema *load.Schema) error {
	for _, name := range slices.Sorted(maps.Keys(schema.Defs)) {
		if err := walkRefs(schema, name, schema.Defs[name]); err != nil {
			return err
		}
		if err := checkTypable(name, schema.Defs[name], placeDefinition); err != nil {
			return err
		}
	}
	return nil
}

// placement is where a schema node sits, for checkTypable.
type placement int

const (
	// placeDefinition is a definition of $defs.
	placeDefinition placement = iota
	// placeProperty is a property of an object definition, which becomes a
	// nested struct when it is an inline object.
	placeProperty
	// placeVariant is a direct anyOf or oneOf member of a definition, which
	// becomes an inline variant.
	placeVariant
	// placeOther is any other node.
	placeOther
)

// checkTypable fails on the shapes the model would type with loss, which
// 0011-MADR-harden-code-generator-engine.md (phase 6 amendment) refuses: an
// allOf of more than one schema, an allOf on a definition, and an inline
// object with properties anywhere but a definition, a property of an object
// definition, or a direct anyOf or oneOf member of a definition. The
// subschemas of not and unevaluatedProperties are not typed and not checked.
func checkTypable(path string, d *load.Definition, place placement) error {
	if d == nil {
		return nil
	}
	if place == placeDefinition && len(d.AllOf) > 0 {
		return fmt.Errorf("%s: allOf on a definition is not supported", path)
	}
	if n := len(slices.DeleteFunc(slices.Clone(d.AllOf), func(e *load.Definition) bool { return e == nil })); n > 1 {
		return fmt.Errorf("%s: allOf of %d schemas has no Go type; only an allOf of one schema is supported", path, n)
	}
	if d.Ref == "" && len(d.Properties) > 0 {
		typed := place == placeDefinition || place == placeVariant || (place == placeProperty && primaryType(d) == schemaTypeObject)
		if !typed {
			return fmt.Errorf("%s: an inline object with properties here has no Go type; declare it in $defs and refer to it", path)
		}
	}
	// not and unevaluatedProperties constrain a value and are never typed, so
	// they are not walked.
	isUnion := len(d.AnyOf) > 0 || len(d.OneOf) > 0
	for _, c := range subschemas(path, d) {
		child := placeOther
		switch {
		case c.keyword == kwNot || c.keyword == kwUnevaluated:
			continue
		case c.keyword == kwProperties && place == placeDefinition && !isUnion:
			child = placeProperty
		case (c.keyword == kwAnyOf || c.keyword == kwOneOf) && place == placeDefinition:
			child = placeVariant
		}
		if err := checkTypable(c.path, c.def, child); err != nil {
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
	if typ := primaryType(d); d.XDeserializeSkipInvalidItems && typ != schemaTypeArray {
		return fmt.Errorf("%s: x-deserialize-skip-invalid-items on %s, not an array", path, typ)
	}
	for _, c := range subschemas(path, d) {
		if err := walkRefs(schema, c.path, c.def); err != nil {
			return err
		}
	}
	return nil
}

// The keywords that hold subschemas, as subschemas names them.
const (
	kwItems       = "items"
	kwAdditional  = "additionalProperties"
	kwNot         = "not"
	kwUnevaluated = "unevaluatedProperties"
	kwProperties  = "properties"
	kwAnyOf       = "anyOf"
	kwOneOf       = "oneOf"
	kwAllOf       = "allOf"
)

// subschemas returns the subschemas of d with their paths and the keyword that
// holds each: items, additionalProperties, not, unevaluatedProperties, the
// properties in name order, then the members of anyOf, oneOf, and allOf.
func subschemas(path string, d *load.Definition) []schemaNode {
	out := []schemaNode{
		{path + "." + kwItems, d.Items, kwItems},
		{path + "." + kwAdditional, d.AdditionalProperties, kwAdditional},
		{path + "." + kwNot, d.Not, kwNot},
		{path + "." + kwUnevaluated, d.UnevaluatedProperties, kwUnevaluated},
	}
	for _, k := range slices.Sorted(maps.Keys(d.Properties)) {
		out = append(out, schemaNode{path + "." + kwProperties + "." + k, d.Properties[k], kwProperties})
	}
	for _, kw := range []struct {
		name string
		list []*load.Definition
	}{{kwAnyOf, d.AnyOf}, {kwOneOf, d.OneOf}, {kwAllOf, d.AllOf}} {
		for i, v := range kw.list {
			out = append(out, schemaNode{fmt.Sprintf("%s.%s[%d]", path, kw.name, i), v, kw.name})
		}
	}
	return out
}

// schemaNode is a definition, its path from the top of the schema, and the
// keyword of its parent that holds it.
type schemaNode struct {
	path    string
	def     *load.Definition
	keyword string
}
