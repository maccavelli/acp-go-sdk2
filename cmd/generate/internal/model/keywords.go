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
	"deprecated":  TreatAnnotation,

	"x-method":      TreatProtocol,
	"x-side":        TreatProtocol,
	"x-docs-ignore": TreatProtocol,
}

// The Go number types a format maps to.
const (
	goInt8    = "int8"
	goInt16   = "int16"
	goInt32   = "int32"
	goInt64   = "int64"
	goUint8   = "uint8"
	goUint16  = "uint16"
	goUint32  = "uint32"
	goUint64  = "uint64"
	goFloat32 = "float32"
	goFloat64 = "float64"
	goBool    = "bool"
)

// integerFormats and numberFormats map a format to its Go type; "" is a node
// without one. schemars writes these formats from the Rust types (OpenAPI's
// format registry lists them); JSON Schema defines none of them.
var (
	integerFormats = map[string]string{
		"": goInt64, "int": goInt64, goInt8: goInt8, goInt16: goInt16, goInt32: goInt32, goInt64: goInt64,
		"uint": goUint64, goUint8: goUint8, goUint16: goUint16, goUint32: goUint32, goUint64: goUint64,
	}
	numberFormats = map[string]string{"": goFloat64, "float": goFloat32, "double": goFloat64}
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
