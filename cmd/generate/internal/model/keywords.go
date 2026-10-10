package model

// Treatment is what the generator does with a schema keyword. See
// 0012-MADR-schema-fidelity-framework.md for the table this file holds.
type Treatment int

const (
	// TreatStructure shapes types, unions, and variants.
	TreatStructure Treatment = iota + 1
	// TreatType chooses a Go type or declares constants.
	TreatType
	// TreatDecode changes how a value is decoded.
	TreatDecode
	// TreatDecodeEncode changes how a value is decoded and encoded.
	TreatDecodeEncode
	// TreatTypeOrValidate is enforced by the Go type when it matches the
	// type's range, and checked by Validate otherwise.
	TreatTypeOrValidate
	// TreatValidate is checked by Validate.
	TreatValidate
	// TreatAnnotation documents and is not checked.
	TreatAnnotation
	// TreatProtocol binds methods and docs.
	TreatProtocol
)

// keywords gives every keyword load.Definition declares one treatment.
// TestKeywordRegistryMatchesLoader keeps the two in step; the loader refuses
// any other keyword.
var keywords = map[string]Treatment{
	"$ref":                  TreatStructure,
	"allOf":                 TreatStructure,
	"anyOf":                 TreatStructure,
	"oneOf":                 TreatStructure,
	"discriminator":         TreatStructure,
	"not":                   TreatStructure,
	"unevaluatedProperties": TreatStructure,
	"properties":            TreatStructure,
	"items":                 TreatStructure,
	"title":                 TreatStructure,

	"type":                 TreatType,
	"format":               TreatType, // integer and number; on a string, an annotation
	"additionalProperties": TreatType,
	"enum":                 TreatType,
	"const":                TreatType,

	"required":                         TreatDecode,
	"x-deserialize-default-on-error":   TreatDecode,
	"x-deserialize-skip-invalid-items": TreatDecode,
	"default":                          TreatDecodeEncode,

	"minimum":   TreatTypeOrValidate,
	"maximum":   TreatTypeOrValidate,
	"minLength": TreatValidate,

	"description": TreatAnnotation,

	"x-method":      TreatProtocol,
	"x-side":        TreatProtocol,
	"x-docs-ignore": TreatProtocol,
}

// goInt64 and goUint64 are the widest Go integer types.
const (
	goInt64  = "int64"
	goUint64 = "uint64"
)

// integerFormats and numberFormats map a format to its Go type; "" is a node
// without one. schemars writes these formats from the Rust types (OpenAPI's
// format registry lists them); JSON Schema defines none of them.
var (
	integerFormats = map[string]string{
		"": goInt64, "int": goInt64, "int8": "int8", "int16": "int16", "int32": "int32", goInt64: goInt64,
		"uint": goUint64, "uint8": "uint8", "uint16": "uint16", "uint32": "uint32", goUint64: goUint64,
	}
	numberFormats = map[string]string{"": "float64", "float": "float32", "double": "float64"}
)

// numericFormat returns the Go type of an integer or number with the given
// format, or "" when the type is not numeric or the format is unknown.
func numericFormat(typ, format string) string {
	switch typ {
	case schemaTypeInteger:
		return integerFormats[format]
	case schemaTypeNumber:
		return numberFormats[format]
	default:
		return ""
	}
}
