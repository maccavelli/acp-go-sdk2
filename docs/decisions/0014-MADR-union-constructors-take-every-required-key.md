---
status: accepted
date: 2026-10-12
decision-makers: Repository owner
---

# Union constructors take every required key of their variant's type, cover the catch-all and every union, and the engine refuses a union key a variant's type cannot hold

## Context and Problem Statement

`helpers_gen.go` holds one `New<Union><Field>` constructor for each variant of each `oneOf` union, except the three whose constructors
`helpers.go` writes by hand (`HandWrittenHelpers` in `cmd/generate/internal/model/acp.go`). The model documents a constructor's parameters
as "the variant's required properties, in schema order, without the discriminator" (`Helper.Params`, `internal/model/model.go:216`), and
the emitter comments them as "every required property except the const discriminator" (`internal/emit/helpers.go:20`).
[0013-MADR-close-phase-6-follow-ups.md](0013-MADR-close-phase-6-follow-ups.md) recorded that keys a union requires of every variant stay at
their zero values, and left the question open ("whether a constructor should take those keys is not decided here").

This record assesses that gap, then, at the owner's requests of 2026-10-12, the catch-all variant's constructor, which a first draft of
this record had left for later, the three items the analysis of it left open, and the rule for a leading acronym that one of them needs.
Read on `bc0cb78` (`v0.15.0` plus the 0013 close), 2026-10-12. Every probe and prototype below ran on a scratch
copy made with `git archive bc0cb78`, never on the tree.

### The required keys

1. **A union's own required keys are not parameters.** `buildUnion` (`internal/model/model.go:592`) builds the parameter list from
   `expandAllOf(schema, v)`, the variant definition alone. An inline object variant's Go type also holds the union's own properties:
   `inlineOf` (`internal/model/fields.go:442`) merges `t.def.Properties` and `t.def.Required` into it, and `Variant.HasSharedFields` marks
   it. `SessionConfigOption` requires `id` and `name` of every variant, at the union level, beside its `oneOf`. Its constructors are
   `NewSessionConfigOptionSelect(currentValue, options)` and `NewSessionConfigOptionBoolean(currentValue)`. A probe of
   `NewSessionConfigOptionBoolean(true)`:

   ```text
   marshal: {"currentValue":true,"id":"","name":"","type":"boolean"} err=<nil>
   unmarshal err=<nil>
   validate: <nil>
   response validate: <nil>   (the value inside SetSessionConfigOptionResponse.ConfigOptions)
   ```

   Nothing catches it. The decoder requires the keys to be present, and `""` is present. `Validate` checks what the schema states
   ([0012-MADR-schema-fidelity-framework.md](0012-MADR-schema-fidelity-framework.md), outcome 7), and the schema gives `id` and `name` no
   `minLength`. A caller that trusts the constructor's signature sends an option with an empty identifier and label.

2. **A direct `$ref` variant's required keys are not parameters.** In step 9,
   [0011-MADR-harden-code-generator-engine.md](0011-MADR-harden-code-generator-engine.md) ("Found in step 9 and decided") decided that a
   variant that is a direct `$ref` takes its required keys, discriminator value and shape from the definition it names. `buildVariant` does so
   through `facts` (`internal/model/model.go:621`). The constructor loop in `buildUnion` was not changed and still reads the variant itself,
   which for `{"$ref": "#/$defs/Start"}` has no `required`. The committed golden pins the result: in
   `cmd/generate/testdata/engine/accept-discriminated-shapes.txtar`, `Start` requires `at` (`int64`), and the API listing has
   `func NewEventStart() Event` (line 41). No committed union has a direct `$ref` variant.

   Defects 1 and 2 share one cause: the constructor reads its variant's definition alone.

3. **A union's own properties are lost for a `$ref` variant, and the engine accepts the schema.** A variant that is a `$ref`, or an `allOf`
   of one `$ref` (`singleRefVariant`, `internal/model/schema.go:105`), is emitted as the definition it names, which does not hold the
   union's properties (`HasSharedFields` is false, `internal/model/model.go:679`). When every variant is such a reference, `classify`
   turns the definition into an object with flattened members instead (`internal/model/model.go:372`); `ElicitationFormMode` and
   `ElicitationUrlMode` take that path. When only some are, the type stays a union and the union's keys are dropped for those variants. An
   engine-case probe, with `Event` requiring its own `id` beside a `$ref` variant `Start` and an inline variant `stop`, was accepted and gave:

   ```text
   type EventStop struct{Id string "json:\"id\""; Kind string "json:\"kind\""}
   type Start struct{At int64 "json:\"at\""; Kind string "json:\"kind\""}
   func NewEventStart() Event
   func NewEventStop() Event
   ```

   `id` reaches neither `Start` nor any constructor. Decoding a `start` payload drops its `id`, and encoding the value cannot send one. The
   generator is to refuse a schema it cannot represent
   ([0011-MADR-harden-code-generator-engine.md](0011-MADR-harden-code-generator-engine.md), decision drivers, and "Found in step 3 and
   decided" for two other lossy shapes). No committed union has this shape: the only definitions with both properties of their own and a
   reference variant are `ElicitationFormMode` and `ElicitationUrlMode`, two reference variants each, and both become flattened members.

### The catch-all

4. **A catch-all's constructor leaves its discriminator empty.** A catch-all is the variant a union keeps for discriminator values it does
   not list: an inline object whose discriminator is a required string with no `const`, excluding the listed values with `not`
   (`isCatchAll`, `internal/model/model.go:920`). Its value is its own, so it is not a constant the constructor can write, and the
   discriminator key is skipped as a parameter like every other variant's. `accept-discriminated-shapes` has `func NewEventOther() Event`.

5. **A catch-all can hold a value its schema excludes, and nothing says so.** Probed on the committed `ElicitationPropertySchema`, whose
   catch-all `Other` is discriminated by `type`:

   ```text
   type=""        validate=<nil> marshal={"type":"","x":1}        decoded=Other
   type="_custom" validate=<nil> marshal={"type":"_custom","x":1} decoded=Other
   type="string"  validate=<nil> marshal={"type":"string","x":1}  decoded=String
   ```

   A catch-all holding `"string"`, a value its `not` excludes, validates, is sent, and is read by the peer as the `String` variant. The
   union's `Validate` (`types_gen.go`, `(*ElicitationPropertySchema).Validate`) counts the variants set and validates `Array`; it does not
   look at the catch-all's discriminator. In the two schema files, each of the ten catch-alls (six unions) has a `not` that lists exactly
   the discriminator values of the union's other variants, so what the schema excludes and what the decoder routes elsewhere are the same set.

6. **Every committed catch-all is in an `anyOf` union, which gets no constructors.** Catch-alls: `CreateElicitationRequest` (`mode`),
   `CreateElicitationResponse` (`action`), `ElicitationPropertySchema` (`type`), `MultiSelectItems` (`type`), and in the unstable schema
   `IdleStateUpdate` (`stopReason`) and `StateUpdate` (`state`). Constructors are generated for `oneOf` unions only
   (`if !t.ExactlyOne || …`, `internal/model/model.go:579`), a rule that came with the upstream initial commit `fe457c7`
   (`len(def.OneOf) == 0` in its `internal/emit/helpers.go`) and that no record decides. So a catch-all constructor builds nothing in the SDK
   unless constructors reach `anyOf` unions.

7. **A parameter named after a Go keyword is refused; one named after a predeclared identifier shadows it.** Parameters take the raw key as
   their name, and `checkParam` (`internal/model/names.go:85`) refuses a key that is not an identifier, so a keyword: `model_test.go:72`
   pins `required property "range" cannot be a Go parameter name`. Required keys in the schema files that are Go keywords: `type` (60
   places, the discriminator of `ElicitationPropertySchema` and `MultiSelectItems` among them), `const` (`EnumOption`), `range`
   (`NesDiagnostic`, `NesTextEdit`). A predeclared identifier passes: the prototype below generated
   `func NewUnstableMessageMcpResponseError(error UnstableMcpError) UnstableMessageMcpResponse`, whose parameter shadows the type `error`.

8. **The doc comment quotes an empty value.** A constructor's comment is `"%s constructs a %s using the '%s' variant."` with the variant's
   discriminator value. A variant without one (a catch-all, a key-less variant, every variant of an undiscriminated union) gets
   `using the '' variant`, which the doc-comment formatter prints as `using the ” variant`.

### What building it all would give

A prototype on the scratch copy combined the parameters of defects 1 and 2, constructors for `anyOf` unions, the discriminator as a
parameter of a catch-all, and stand-in names for keyword keys. Its first run failed the generator's own type check on six variants of
`ElicitationContentValue` and `RequestId`: they are primitive types (`invalid composite literal type ElicitationContentValueString`), and
the emitter builds every constructor as a composite literal. With constructors limited to object variants, it generated 57 constructors
(11 today) and type-checked. `TestConformanceConstructors` passed 53 and skipped 4 whose zero-argument value does not encode
(`SessionConfigOption_Select`, as today, and `ElicitationPropertySchema_Array`, `CreateElicitationRequest_Url`,
`CreateElicitationRequest_Form`). The six catch-all constructors were `NewCreateElicitationRequestOther(message string, mode string)`,
`NewCreateElicitationResponseOther(action string)`, `NewElicitationPropertySchemaOther(typ string)`, `NewIdleStateUpdateOther(stopReason
string)`, `NewMultiSelectItemsOther(typ string)` and `NewStateUpdateOther(state string)`. The generator's tests failed only where their
expectations name the old behaviour: nine engine-case goldens changed, gaining constructors or parameters; `TestRegistry` found the new
helpers; `TestBuildNames` compared a `Param` without the prototype's name field; and the `range` refusal case was accepted. The root
module's other tests were not rerun on this second prototype; the first, with only defects 1 and 2 fixed, passed them all.

### The items the first analysis left open

9. **Primitive and array variants have no constructor shape.** Ten variants of three unions are not objects: `RequestId` (`Null`, a
   `struct{}`; `Number`, `int64`; `Str`, `string`), `ElicitationContentValue` (`String`, `Integer` `int64`, `Number` `float64`, `Boolean`,
   `StringArray` `[]string`) and `SessionConfigSelectOptions` (`Ungrouped` and `Grouped`, both slices). A caller writes
   `RequestId{Number: acp.Ptr(RequestIdNumber(7))}` (`union_wire_test.go:275` uses the two-line form). `SessionConfigSelectOptions` is a
   parameter of `NewSessionConfigOptionSelect`, and `ElicitationContentValue` fills the `content` map a client sends when it accepts an
   elicitation, from form values a caller holds as plain `string`, `int64`, `float64` and `bool`.

10. **A key that is not a Go identifier stops the whole generator.** No key in either schema file, required or optional, fails
    `token.IsIdentifier`, and none begins with an upper-case letter. A required one would fail `checkParam` inside `buildUnion`, which fails
    `Build`: the generator would refuse the schema for a problem only the constructor has, though the key's struct field is generated
    through `ToExportedField`.

11. **A catch-all's extra keys are a field, not a parameter.** Each of the six catch-all types holds `Extra map[string]json.RawMessage`,
    which is what an `_`-prefixed extension value carries its payload in. Its `MarshalJSON` writes an `Extra` key only where no declared
    field wrote that key (`if _, ok := m[k]; !ok`), so a colliding key is dropped silently.

12. **Lower camel case needs a rule for a leading acronym.** A name derived from a key (10) lowers its first letter, and lowering only the
    first rune gives `uRLPath` for a key `URL-path`. Go's guidance is that an initialism keeps one case, `url` or `URL` and never `Url`,
    lower case when it leads (`urlPony`, `xmlHTTPRequest`, `appID`), and that generated code is held to a lower standard than hand-written
    code ([Go Code Review Comments, "Initialisms"](https://go.dev/wiki/CodeReviewComments)). This repository's generated fields do not
    follow it: 193 fields are spelled `Url`, `Id` and the like, none `URL` or `ID`. Generators differ:
    [oapi-codegen](https://github.com/oapi-codegen/oapi-codegen/blob/main/pkg/codegen/utils.go)'s `LowercaseFirstCharacters` lowers the
    leading run of capitals and stops before one that begins a lower-case word, with no list;
    [go-openapi/swag](https://github.com/go-openapi/swag/blob/v0.22.3/util.go)'s `ToVarName` lowers a whole name only when it is in
    golint's initialism list and otherwise the first rune; [staticcheck](https://staticcheck.dev/docs/configuration/options/)'s ST1003
    checks hand-written code against a configurable list of 42; protobuf-go has no lower-camel Go identifier rule. Three rules, run on
    sample keys through a copy of `ToExportedField`:

    ```text
    key              field           first rune      L2 field run    L1 first word   L3 list
    URL-path         URLPath         uRLPath         urlPath         urlPath         urlPath
    x-API-key        XAPIKey         xAPIKey         xapiKey         xAPIKey         xAPIKey
    HTTPServer-url   HTTPServerUrl   hTTPServerUrl   httpServerUrl   httpServerUrl   httpServerUrl
    XMLHttp-request  XMLHttpRequest  xMLHttpRequest  xmlHttpRequest  xmlHttpRequest  xmlHttpRequest
    HTTP2-server     HTTP2Server     hTTP2Server     http2Server     http2Server     http2Server
    ACME-id          ACMEId          aCMEId          acmeId          acmeId          aCMEId
    ID               ID              iD              id              id              id
    ```

Inside the repository, the only caller of the two `SessionConfigOption` constructors is the generated registry `schema_gen_test.go`.

**Questions:** what should a constructor take; what should the engine do with a union key a variant's type cannot hold; how should a
catch-all be constructed and checked, and should it take its extra keys; what should a parameter be named when its key cannot be, and how
is a leading acronym lowered; and which unions and variants get constructors, with what parameter for a variant that has no key?

## Decision Drivers

* **A constructor's signature is its contract.** A value built from every parameter should carry every key the schema requires of it, set
  by the caller and not defaulted, and the result should be read by a peer as the variant it was built as.
* **The schema, not the SDK, says what is valid.** `Validate` checks what the schema states and nothing else
  ([0012-MADR-schema-fidelity-framework.md](0012-MADR-schema-fidelity-framework.md), outcome 7); `v0.15.0` removed 28 empty-string checks
  for that reason. A catch-all's `not` is stated by the schema; a non-empty `id` is not.
* **Follow the repository's own idiom.** The hand-written helpers in `helpers.go` take every required key as a positional parameter and the
  optional ones as functional options: `StartToolCall(id ToolCallId, title string, opts ...ToolCallStartOpt)`.
* **One reading of a variant.** Since step 9 the model reads a direct `$ref` variant through the definition it names. The constructor
  should read the same facts the decoder and the type do.
* **Refuse rather than lose.** The generator refuses a schema it cannot represent
  ([0011-MADR-harden-code-generator-engine.md](0011-MADR-harden-code-generator-engine.md)).
* **Rules over tables.** Names derived from the toolchain's own sets (`go/token`, `go/types`) follow Go releases without upkeep; a
  hand-kept list does not.
* **Generated from the model, tested from the registry.** Constructors and their test
  ([0013-MADR-close-phase-6-follow-ups.md](0013-MADR-close-phase-6-follow-ups.md), C1) follow the schema with no hand-kept list.
* **Compatibility is pre-1.0.** The module is at `v0.15.0`. A signature change is allowed in a minor release and goes under "Breaking
  changes" in its release notes, as `ErrorCode` did in `v0.15.0`; added functions are not breaking, but each is API kept from then on.

## Considered Options

For a constructor's parameters:

* P1. One parameter for every required key the variant's Go type holds.
* P2. Keep the signatures and document that the caller sets the shared keys.
* P3. Shared keys as functional options.
* P4. No generated constructor for a union with required keys of its own.
* P5. `Validate` refuses a required key at its zero value.

For a union key a variant's type cannot hold:

* R1. Refuse the schema.
* R2. Accept it, as now.

For the catch-all:

* C1. Its discriminator is a parameter, and the union's `Validate` refuses a catch-all holding a value its `not` excludes.
* C2. Its discriminator is a parameter, with no new check.
* C3. Its constructor returns `(U, error)` and refuses an excluded value.
* C4. No constructor for a catch-all.

For a parameter whose key cannot name it:

* N1. The key, or the key followed by `Value` when the key is a Go keyword or predeclared identifier.
* N2. A fixed table of short names in the standard library's style (`typ`, `rng`, `err`, …).
* N3. The key followed by `_`.
* N4. Refuse keywords, as now, and allow predeclared identifiers.

For which unions and variants get constructors:

* A1. Every object variant of every union, `oneOf` and `anyOf`, except the hand-written and documentation-ignored ones.
* A2. `oneOf` unions only, as now.
* A3. Every variant of every union, `oneOf` and `anyOf`, primitive and array variants included, with the same exceptions.

For a primitive or array variant:

* V1. No constructor (A1).
* V2. A constructor taking the variant's own type: `NewRequestIdNumber(value RequestIdNumber)`.
* V3. A constructor taking the type the variant's type is declared with: `NewRequestIdNumber(value int64)`.
* V4. Hand-written helpers for the few that matter.

For a key that is not a Go identifier:

* K1. Refuse, as now.
* K2. Name the parameter in lower camel case from the key's words.
* K3. Generate no constructor for that variant.
* K4. Positional names (`p1`, `p2`).

For the lower camel case of K2 (presented to the owner as B, A and C):

* L1. Split the key at its separators; lower the first word's leading capitals, keeping a capital that begins a lower-case word; keep
  the other words as the Go field has them.
* L2. Lower the leading capitals of the whole Go field name the same way (oapi-codegen's `LowercaseFirstCharacters`).
* L3. Lower a leading initialism from golint's list whole, otherwise the first rune.

For a catch-all's extra keys:

* X1. Not a parameter; the caller sets `Extra` on the variant.
* X2. A trailing `extra map[string]json.RawMessage` parameter.
* X3. Generated functional options, one `With…Extra` per catch-all.
* X4. An `extra map[string]any` parameter, encoded inside the constructor.

## Decision Outcome

Chosen options: "P1", "R1", "C1", "N1", "A3" with "V3", "K2" with "L1", and "X1". The owner chose V3, K2, L1 and X1 on 2026-10-12
("follow the recommended solutions"; L1 from the three rules above), and accepted the record as a whole the same day, with items 7 and 8
below, which planning found and the owner decided.

* **P1**, because it is the only option that makes the signature match the schema without a rule the schema does not state, it is the
  idiom `helpers.go` uses, and it reads a variant through the same facts as its type and its decoder.
* **R1**, because it closes the one shape in which the engine still drops a key silently, and changes no generated file.
* **C1**, because it is the only option under which a catch-all built in Go is either read by the peer as a catch-all or refused by
  `Validate`, by a rule the schema states, and the constructor keeps the one shape all constructors have.
* **N1**, because it derives both sets from the toolchain, makes `type`, `range` and `const` usable, ends the shadowing of predeclared
  identifiers, and names each parameter after the key it holds.
* **A3**, because without `anyOf` unions the catch-all constructor builds nothing in the SDK, every committed catch-all being in one; nothing
  recorded limits constructors to `oneOf`; and with V3 a primitive variant's constructor is as useful as an object variant's.
* **V3**, because the callers that build these values (form answers, select options, request IDs) hold plain Go values, which V2 would make
  them convert first; a slice argument is assignable as it is.
* **K2** with **L1**, because a key that only a constructor cannot name should not stop the generator, and L1 is the only one of the three
  rules that gives every sample the case Go's guidance asks for (`xAPIKey`, `urlPath`, `acmeId`) with no list to keep; the name is the
  field it fills with its first word lowered.
* **X1**, because `Extra` is optional, setting an optional field on the returned value is ordinary Go, and every constructor keeps one
  shape: its parameters are the required keys.

Concretely:

1. **Parameters (P1).** A constructor takes one parameter for each required key of the Go type that holds its variant:
   * an inline object variant: the union's own required keys in schema order, then the variant's (through its `allOf`), each key once; a
     property the variant redefines takes the variant's type, as `inlineOf` already does for the field;
   * a direct `$ref` variant, or an `allOf` of one `$ref`: the required keys of the definition it names, through `allOf`;
   * the union's discriminator is not a parameter where the constructor writes the variant's constant, and is one for the catch-all (3).

   One model function computes the list, reading the variant through the same facts as `inlineOf` and `buildVariant`, so the
   constructor's reading cannot fall behind theirs again, as it did after step 9. The `Helper.Params` and emitter comments say what the
   parameters are.
2. **Refusal (R1).** The model refuses a union that stays a union (not turned into flattened members) and has properties of its own when any
   of its variants is a reference: the variant's type cannot hold those properties. The error names the union, the variant and the keys,
   and an engine case `refuse-union-keys-ref-variant` pins it. Any property of its own counts, required or not; an optional key is lost the
   same way.
3. **Catch-all (C1).** A catch-all's constructor takes its discriminator as a parameter, with its other required keys; keys it keeps in
   `Extra` are not parameters, and the caller sets `Extra` on the variant (X1); the constructor's doc comment says so, and that a key a
   declared field also writes is not sent from `Extra`. The union's `Validate` refuses a catch-all whose discriminator equals a value its
   `not` excludes, with an error naming the union, the key and the value. This adds one check to the list in
   [0012-MADR-schema-fidelity-framework.md](0012-MADR-schema-fidelity-framework.md), outcome 7, under its own rule (the schema states it),
   so 0012 gets an amendment saying so. The decoder never produces such a value, since it routes each listed value to its variant; the
   check fires only on a value built in Go.
4. **Names (N1).** A parameter is named after its key. When the key is a Go keyword (`token.IsKeyword`) or a predeclared identifier
   (`types.Universe.Lookup`), the name is the key followed by `Value`: `typeValue`, `rangeValue`, `constValue`, `errorValue`. A key that is
   not an identifier at all is named under K2 and L1 instead of being refused: the key is split at its non-alphanumeric runes, each word
   is turned into Go case as `ToExportedField` does, and the first word's leading run of capitals is lowered, except a capital that begins
   a lower-case word (`URL-path` gives `urlPath`, `x-API-key` gives `xAPIKey`, `HTTPServer-url` gives `httpServerUrl`). N1 then applies
   to the result (`$type` gives `type`, then `typeValue`). A key with no letter or digit, or whose name would begin with a digit, is
   still refused. Two parameters of one constructor with the same name are refused.
5. **Unions and variants (A3, V3).** Every union the model builds gets a constructor for each variant, whether its list is `oneOf` or
   `anyOf`, unless it is in `HandWrittenHelpers` or marked `x-docs-ignore`. An object variant's constructor is as in 1. A primitive or
   array variant's constructor takes one parameter, `value`, of the type the variant's type is declared with (`int64` for
   `RequestIdNumber`, `[]string` for `ElicitationContentValueStringArray`), converts it, and sets the wrapper field to its address; a
   `null` variant's takes none: `NewRequestIdNumber(value int64) RequestId`, `NewRequestIdNull() RequestId`. Clarified while planning,
   for shapes neither schema file has: a variant with no type (a title only, an empty struct) takes no parameter, like `null`; a `$ref`
   variant to a named primitive definition takes that definition's underlying type, and a `$ref` variant to any other non-object
   definition (an enum, an array, a union) takes the definition's own type, which needs no conversion.
6. **Doc comment.** A constructor's comment names its variant by the discriminator value where it has one, and otherwise by its field
   name, so no comment quotes an empty value.
7. **A nil array variant is sent as `[]` (decided 2026-10-12, while planning).** A probe of the ten non-object variants at their zero
   values on the scratch copy found that a set array variant holding `nil` encodes as `null`, where the schema requires an array, and
   decodes back with no variant set and no error:

   ```text
   ECV StringArray    marshal=null  decoded=(none)
   SCSO Ungrouped     marshal=null  decoded=(none)
   SCSO Grouped       marshal=null  decoded=(none)
   ```

   Required array fields already send `[]` for `nil` (README, "Wire encoding"); union array variants did not. Asked, the owner chose that
   the union's `MarshalJSON` writes `[]` for a set array variant holding `nil`, for literals and constructors alike. Not chosen: the
   constructor alone replacing `nil`, which leaves literals sending `null`; `Validate` refusing it, which the schema's type does not state.
   This is a change to what the SDK sends, for the release notes.
8. **The round trip of a union without a discriminator is wire-stable (decided 2026-10-12, while planning).** The same probe found that
   JSON cannot carry every variant apart: `ElicitationContentValue{Number: 5}` encodes `5`, which decodes as `Integer` (the decoder takes the
   first variant that fits; `5.5` stays `Number`), and `[]` fits both `SessionConfigSelectOptions` variants. Asked, the owner chose that
   `TestConformanceConstructors` requires, for a union without a discriminator, that decoding sets exactly one variant and that it encodes
   to the same bytes again; a discriminated union keeps the same-variant requirement, and the check that the constructor set its own variant
   runs before encoding for every union. Not chosen: re-implementing the decoder's fit order in the test, or per-constructor sample
   arguments from the registry.

### Consequences

* Good, because a value built by a generated constructor carries every key its schema requires, each from the caller, and a catch-all
  that the peer would misread fails `Validate`.
* Good, because the gap fails at compile time for any caller of the two changed constructors, which is where a missing `id` and `name`
  should show up.
* Good, because the constructor, the decoder and the variant's type read a variant through the same facts, and the last silent loss of a
  union key becomes a refusal with a message.
* Good, because every union gains constructors without a hand-kept list: 46 object-variant constructors for `anyOf` unions in the
  prototype, `McpServer`, `AuthMethod`, `CreateElicitationRequest` and `IdleStateUpdate` among them, and 10 for primitive and array
  variants, so `NewSessionConfigOptionSelect`'s `options` and an elicitation answer's values no longer need a literal.
* Good, because a key that is not a Go identifier no longer stops the generator.
* Good, because a nil array variant is sent as `[]`, which the schema allows, rather than `null`, which it does not.
* Bad, because `NewSessionConfigOptionSelect` and `NewSessionConfigOptionBoolean` change signature: a breaking change, to be released in a
  minor version and listed under "Breaking changes".
* Bad, because the exported API grows by every new constructor, each kept from then on; a schema change that renames a variant renames
  or removes its constructor.
* Bad, because `Validate` refuses a catch-all value it accepted in `v0.15.0`; such a value was already misread by every peer.
* Bad, because a positional list grows with the schema: `NewSessionConfigOptionSelect` and `NewMcpServerStdio` would take four.
* Bad, because a primitive variant's parameter is named `value` by the generator, not by the schema, and its type in the signature is the
  declared type (`int64`), not the schema's name for it (`RequestIdNumber`); a format change in the schema changes the signature.
* Bad, because a catch-all's extension payload takes two statements: the constructor, then `Extra`.
* Neutral, because `typeValue` is longer than the standard library's `typ`.

### Confirmation

* A model unit test asserts, for every constructor built from the committed schema and from every accepting engine case, that its
  parameters plus any discriminator it writes are exactly the required fields of its variant's Go type. It is seen failing on a planted
  break: the constructor reading the variant alone again.
* `accept-discriminated-shapes` carries `func NewEventStart(at int64) Event` and a catch-all constructor that takes the discriminator; an
  accepting case has an inline variant of a union with required keys of its own, a key that is a keyword, and a key that is predeclared;
  `accept-catch-all-variant` carries `NewUOther(typeValue string)`.
* `refuse-union-keys-ref-variant` refuses the probe's shape of defect 3, and is seen failing when the refusal is removed in a scratch
  copy. The `range` case of `TestBuildRefuses` becomes an accepting case with `rangeValue`.
* `TestConformanceConstructors` covers every constructor, and for each catch-all calls it with each value the union's other variants
  list, requiring `Validate` to refuse it, and with a value they do not list, requiring the round trip to keep the catch-all. The check is
  seen failing on a planted break: the `Validate` check removed.
* `TestConformanceConstructors` covers the primitive and array variants' constructors, whose registry entries pass the zero value of the
  declared type.
* A unit test of the name function covers the sample table of finding 12 under L1, a keyword after it (`$type` gives `typeValue`), and
  the refusals (a key with no letter or digit, one beginning with a digit), and is seen failing on a planted break: lowering the first rune
  only. An accepting engine case has a required key `x-API-key` and carries `xAPIKey` in its API listing.
* The exported API listing, run before and after, differs by the two changed signatures and the added constructors, and by nothing else.
* `TestGeneratedFilesUpToDate`, both modules' tests with `-race`, and `make pre-add-check` pass on the regenerated SDK.
* The union array variant probe of item 7 sends `[]` and decodes back into an array variant, in an engine behaviour test and through
  `TestConformanceConstructors`; it is seen failing on a planted break: the `[]` branch removed.
* `TestConformanceConstructors` applies item 8: same variant for a discriminated union, one variant and the same bytes for the others.
* The release notes of the next version list the two signatures under "Breaking changes", the `Validate` check and the `[]` encoding under
  changes to what the SDK accepts and sends, and the constructors under additions.

## Pros and Cons of the Options

### P1. One parameter for every required key the variant's Go type holds

* Good, because the signature says what the schema requires, and the caller cannot forget a key.
* Good, because it is the idiom `helpers.go` uses, so generated and hand-written constructors read the same way.
* Neutral, because the order (the union's keys, then the variant's) is a convention; it matches identity first in `StartToolCall(id,
  title, …)`.
* Bad, because two exported functions change signature.

### P2. Keep the signatures and document that the caller sets the shared keys

* Good, because nothing breaks.
* Bad, because the signature keeps promising the required keys and keeps not taking them, and nothing catches an empty one.
* Bad, because a `$ref` variant's keys are not shared keys, so documentation does not cover defect 2.

### P3. Shared keys as functional options

* Good, because existing calls compile.
* Bad, because a required key becomes optional to the compiler, the opposite of the hand-written helpers.
* Bad, because the generator gains an option type and setters per union, more output for less safety.

### P4. No generated constructor for a union with required keys of its own

* Good, because no constructor can be half-filled.
* Bad, because callers write the nested composite literal the README says the helpers spare them, and removing functions breaks callers as
  much as P1 does.

### P5. `Validate` refuses a required key at its zero value

* Good, because a half-filled value fails before it is sent.
* Bad, because the schema does not state it: `""`, `false` and `0` are valid values. It reverses
  [0012-MADR-schema-fidelity-framework.md](0012-MADR-schema-fidelity-framework.md), outcome 7, and the 28 checks `v0.15.0` removed.
* Bad, because it fixes no signature.

### R1. Refuse the schema

* Good, because no key is dropped silently, and the refusal changes no generated file.
* Bad, because a schema in that shape cannot be generated until the engine can represent it.

### R2. Accept it, as now

* Good, because no schema is refused.
* Bad, because the union's keys vanish from the type, the decoder and the constructor alike.

### C1. Its discriminator is a parameter, and the union's `Validate` refuses a catch-all holding a value its `not` excludes

* Good, because a catch-all can be built for any value the schema allows it, and a value it excludes is caught where every other schema
  rule is caught.
* Good, because the constructor keeps the shape every constructor has.
* Neutral, because `Validate` gains a check; the schema states it, so outcome 7's rule holds.
* Bad, because the constructor itself accepts an excluded value; only `Validate` refuses it.

### C2. Its discriminator is a parameter, with no new check

* Good, because it is the smallest change.
* Bad, because the defect-5 probe stands: a catch-all holding `"string"` validates and is read as `String`.

### C3. Its constructor returns `(U, error)` and refuses an excluded value

* Good, because the error comes at construction.
* Bad, because one kind of constructor gets a second shape, and a value built without the constructor is still not checked.

### C4. No constructor for a catch-all

* Good, because no catch-all is built half-filled.
* Bad, because the variant meant for extensions (`_`-prefixed values, in the schema's own descriptions) is the one left without a helper,
  and the defect-5 check is still missing for literals.

### N1. The key, or the key followed by `Value`

* Good, because `token.IsKeyword` and `types.Universe` come with the toolchain and follow Go releases (`any` and `comparable` came in
  1.18; `min`, `max` and `clear` in 1.21) with no list to keep.
* Good, because the name still says which key it holds, and every key the schema uses today keeps its name.
* Neutral, because `.golangci.yml` excludes generated files (`generated: lax`), so the rule is for readers of the API, not a linter.
* Bad, because `typeValue` is longer than the standard library's `typ`.

### N2. A fixed table of short names

* Good, because `typ` matches `go/types` and `reflect`.
* Bad, because most keywords and predeclared identifiers have no settled short form (`range`, `const`, `default`, `len`), so the table
  invents names, and it must follow Go releases.

### N3. The key followed by `_`

* Good, because it is mechanical.
* Bad, because underscores in Go names are against Go's naming convention, which `revive`'s `var-naming` rule enforces on hand-written
  code.

### N4. Refuse keywords, as now, and allow predeclared identifiers

* Good, because nothing changes.
* Bad, because a catch-all discriminated by `type`, as two committed ones are, cannot have a constructor, and `error error` stays.

### A1. Every object variant of every union

* Good, because the catch-all constructors reach the SDK, and `anyOf` unions get what `oneOf` unions have.
* Good, because the prototype type-checked and every constructor's zero value that encodes round-tripped.
* Bad, because ten variants are left without a constructor, with nothing in the API saying why.

### A2. `oneOf` unions only, as now

* Good, because the API stays as it is apart from P1.
* Bad, because C1 and N1 then build nothing in the SDK, and the rule rests on no record.

### A3. Every variant of every union, primitive and array variants included

* Good, because every variant has a constructor, `RequestId`, `ElicitationContentValue` and `SessionConfigSelectOptions` included.
* Bad, because a primitive variant has no key to name its parameter, and the emitter needs a second shape (V decides it).
* Bad, because the API grows by 56 functions at once.

### V1. No constructor

* Good, because there is no new API, and `acp.Ptr` makes the literal one line.
* Bad, because it is A1's gap: the caller must know the variant's Go type and the pointer wrapping, and a `string` from a form needs a
  conversion before `Ptr`.

### V2. A constructor taking the variant's own type

* Good, because the body is `&value`, and an untyped constant needs no conversion: `NewRequestIdNumber(7)`.
* Good, because the signature names the schema's type.
* Bad, because a typed variable still needs a conversion, `NewElicitationContentValueString(ElicitationContentValueString(s))`, which is
  the common case for form input.

### V3. A constructor taking the type the variant's type is declared with

* Good, because plain Go values, constants and variables alike, pass as they are, and a slice is assignable without a conversion.
* Good, because it reads like the hand-written `TextBlock(text string)`.
* Bad, because the body converts before taking the address, and the signature shows `int64` rather than `RequestIdNumber`.

### V4. Hand-written helpers for the few that matter

* Good, because a person chooses each name.
* Bad, because it is a hand-kept list against "generated from the model", and `HandWrittenHelpers` works per union, so mixing hand-written
  and generated constructors in one union needs new machinery.

### K1. Refuse, as now

* Good, because it needs no code and invents no name for a case the schema has never had.
* Bad, because one such key stops every regeneration, not just one constructor.

### K2. Name the parameter in lower camel case from the key's words

* Good, because it never refuses what field generation accepts, and today's names do not change, since every current key is an identifier.
* Good, because lower camel case is Go's convention for a parameter.
* Bad, because the parameter's name no longer matches the key's text, and it needs a rule for a leading acronym (L).

### K3. Generate no constructor for that variant

* Good, because it never stops the generator and invents no name.
* Bad, because it leaves a silent hole in the API that the registry and its test must allow for, against A3.

### K4. Positional names

* Good, because it is trivial and never collides.
* Bad, because the documentation no longer says what each argument is.

### L1. The key's first word, lowered at its leading capitals

* Good, because it is the only rule of the three that gives every sample the case Go's guidance asks for, `xAPIKey` among them, the shape
  of the guidance's own `xmlHTTPRequest`.
* Good, because it needs no list, and the name is the field it fills with its first word lowered.
* Neutral, because a capital run in a later word stays as the field has it: right by the guidance, but beside fields spelled `Url`.
* Bad, because it is a little more code than L2: a split, then L2's loop on one word.

### L2. The field name, lowered at its leading capitals

* Good, because it is one loop with no list, and a widely used Go generator does it.
* Bad, because the field has already lost the key's word breaks, so `x-API-key` gives `xapiKey`.

### L3. A leading initialism from a list

* Good, because it uses the vocabulary of Go's linters.
* Bad, because an acronym not in the list comes out mixed (`aCMEId`), and the list needs keeping, against "rules over tables".

### X1. Not a parameter

* Good, because a constructor's parameters stay exactly the required keys, and setting an optional field on the returned value is ordinary
  Go.
* Bad, because an extension payload takes two statements, and the caller must learn about `Extra`.

### X2. A trailing `extra` parameter

* Good, because one call builds a complete extension payload.
* Bad, because it breaks the rule that parameters are the required keys, for one kind of variant, and most calls pass `nil`.

### X3. Generated functional options

* Good, because it is the hand-written idiom for optional data (`StartToolCall`'s options), and a call without extras stays short.
* Bad, because six option types and setters are generated for one field, and constructors get a second shape.

### X4. An `extra map[string]any` parameter, encoded inside

* Good, because the caller passes Go values directly.
* Bad, because encoding can fail, so the constructor must return `(U, error)`, the second shape C3 was rejected for, and `any` loses the
  raw form `Extra` keeps everywhere else.

## More Information

* **Prototype.** Model and emitter changes made on the scratch copy only. Generator: `go run .` in `cmd/generate`. Tests: the generator
  module's, and `TestConformanceConstructors` in the root module; the first prototype (defects 1 and 2 only) also ran the root module's
  full tests, which passed. The prototype's stand-in names (`typ`) were for measurement and are not the chosen rule. The prototype did not
  build V3, K2 or X1; the counts for V3 come from the generated wrapper types, and the L rules ran as a script over a copy of
  `ToExportedField`.
* **Not decided here.** Nothing from the first analysis remains open. A rule for acronyms inside generated field names (`Url` against
  `URL`) is a separate question: it would rename exported fields across the SDK.
* **Related records.** [0011-MADR-harden-code-generator-engine.md](0011-MADR-harden-code-generator-engine.md) ("Found in step 9 and
  decided"; "Found in step 3 and decided"), [0012-MADR-schema-fidelity-framework.md](0012-MADR-schema-fidelity-framework.md) (outcome 7,
  to be amended under C1), [0013-MADR-close-phase-6-follow-ups.md](0013-MADR-close-phase-6-follow-ups.md) (the gap recorded, C1 registry).
* **Implementation.** [0014-PLAN-union-constructors-take-every-required-key.md](0014-PLAN-union-constructors-take-every-required-key.md).
