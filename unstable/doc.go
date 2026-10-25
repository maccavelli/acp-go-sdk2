// Package unstable holds the ACP API the unstable schema adds after this
// module's v1 froze its unstable-origin names: definitions, methods and union
// variants that no acp declaration carries
// (docs/decisions/0039-MADR-five-adopter-additions-on-schema-1-25-0.md,
// item 1).
//
// Like golang.org/x/exp, this package is not subject to the Go 1
// compatibility promise: a name here may change or go away in a minor
// release of this module, when the unstable schema changes it. Package acp
// is: the unstable-origin names it declared before this package existed stay
// there, frozen and deprecated, whatever the unstable schema later says.
//
// The generator writes this package's code, from what the unstable schema
// adds beyond the frozen manifest, schema/unstable-v1.json:
//
//   - a new definition is a type here, under its schema key, with its
//     constructors;
//   - a new method is its request, response or notification type and its
//     method-name constant; acp's dispatch answers a received one as a
//     method it does not know;
//   - a new variant of a stable union is its type and a
//     Decode<Union><Variant> function, which decodes the payload acp's union
//     keeps for a variant it does not declare, its Unknown.
//
// A new field of an acp type is emitted nowhere: it reaches a program as the
// raw JSON it arrived as. When a definition stabilizes, it moves to acp under
// the same name.
//
// The code here uses acp's exported API only. A failed Validate returns
// *acp.ValidationError, whose text is its fallback, "Type: Field: Rule", and
// an acp value inside a type of this package is checked with acp's Validate,
// by the host's rule for absolute paths.
package unstable
