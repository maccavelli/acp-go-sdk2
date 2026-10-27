package acp

import (
	"maps"
	"slices"
)

// The type values of the elicitation property schemas the schema declares.
const (
	propertyTypeString  = "string"
	propertyTypeNumber  = "number"
	propertyTypeInteger = "integer"
	propertyTypeBoolean = "boolean"
	propertyTypeArray   = "array"
)

// ElicitationField is one property of an elicitation form, as a client shows
// it: its name, whether the form requires it, and its schema
// (0040-MADR-check-elicitation-forms-answers-and-urls.md, item 1).
type ElicitationField struct {
	Name     string
	Required bool
	Schema   ElicitationPropertySchema
}

// Fields returns the properties of s in the order MarshalJSON writes them:
// each name of PropertyOrder that has a property, once, in that order, then
// the other names sorted. A form decoded from the wire so shows its fields in
// the order the agent wrote them. Required is true for a name s.Required
// lists; a required name with no property gives no field. A property of a type
// this package does not know is a field too, whose Schema.Kind() is its type:
// a client must not render it as a known control
// (0040-MADR-check-elicitation-forms-answers-and-urls.md, item 1).
func (s ElicitationSchema) Fields() []ElicitationField {
	names := make([]string, 0, len(s.Properties))
	for _, name := range s.PropertyOrder {
		if _, ok := s.Properties[name]; ok && !slices.Contains(names, name) {
			names = append(names, name)
		}
	}
	for _, name := range slices.Sorted(maps.Keys(s.Properties)) {
		if !slices.Contains(names, name) {
			names = append(names, name)
		}
	}
	fields := make([]ElicitationField, len(names))
	for i, name := range names {
		fields[i] = ElicitationField{Name: name, Required: slices.Contains(s.Required, name), Schema: s.Properties[name]}
	}
	return fields
}

// Kind returns the type of the property schema p holds: "string", "number",
// "integer", "boolean" or "array", the type of one this package does not
// know, or "" when no variant is set
// (0040-MADR-check-elicitation-forms-answers-and-urls.md, item 1).
func (p ElicitationPropertySchema) Kind() string {
	switch {
	case p.String != nil:
		return propertyTypeString
	case p.Number != nil:
		return propertyTypeNumber
	case p.Integer != nil:
		return propertyTypeInteger
	case p.Boolean != nil:
		return propertyTypeBoolean
	case p.Array != nil:
		return propertyTypeArray
	case p.Other != nil:
		return p.Other.Type
	}
	return ""
}

// Defaults returns the default each property of s declares, typed as an
// answer of that property: a String, Number, Integer, Boolean or StringArray
// value, keyed by the property's name. It is nil when no property declares
// one. A client pre-fills its form with them, as the specification asks; they
// are not an answer until the user sends them. The values are copies
// (0040-MADR-check-elicitation-forms-answers-and-urls.md, item 1).
func (s ElicitationSchema) Defaults() map[string]ElicitationContentValue {
	var out map[string]ElicitationContentValue
	set := func(name string, v ElicitationContentValue) {
		if out == nil {
			out = make(map[string]ElicitationContentValue)
		}
		out[name] = v
	}
	for name, p := range s.Properties {
		switch {
		case p.String != nil && p.String.Default != nil:
			set(name, NewElicitationContentValueString(*p.String.Default))
		case p.Number != nil && p.Number.Default != nil:
			set(name, NewElicitationContentValueNumber(*p.Number.Default))
		case p.Integer != nil && p.Integer.Default != nil:
			set(name, NewElicitationContentValueInteger(*p.Integer.Default))
		case p.Boolean != nil && p.Boolean.Default != nil:
			set(name, NewElicitationContentValueBoolean(*p.Boolean.Default))
		case p.Array != nil && p.Array.Default != nil:
			set(name, NewElicitationContentValueStringArray(slices.Clone(p.Array.Default)))
		}
	}
	return out
}
