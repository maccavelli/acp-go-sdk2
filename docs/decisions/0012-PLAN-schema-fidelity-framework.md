---
status: proposed
date: 2026-10-10
associated-madr: "0012-MADR-schema-fidelity-framework.md"
---

# Implement the schema fidelity framework

Associated MADR: [0012-MADR-schema-fidelity-framework.md](0012-MADR-schema-fidelity-framework.md)

## Goal

Every keyword the generator's loader accepts has one treatment in a registry in the model, and the generated SDK applies it: integers take their
declared width and sign, typed maps are typed, arbitrary JSON is carried byte for byte, required keys are checked on decode, the two `x-deserialize-*`
keywords behave as in ACP's Rust implementation, `Validate` checks what the schema states at every depth, and the generated code is consistent Go. A
valid message encodes and decodes as before. This is the MADR's decision, which refines phase 4 of
[0011-MADR-harden-code-generator-engine.md](0011-MADR-harden-code-generator-engine.md).

## Scope

In scope:

* `cmd/generate/internal/model`: a new `keywords.go` (the registry); `fields.go`, `model.go`, `protocol.go`, `acp.go`; their tests
* `cmd/generate/internal/emit`: `types.go`, `ast.go`, `constants.go`; their tests
* the regenerated `types_gen.go`, `agent_gen.go`, `client_gen.go`, `helpers_gen.go`
* hand-written code and tests that use a changed type: `helpers.go` (the four raw options and `WithStartLocations`), `example/*/main.go`, and the root
  `*_test.go` files that build or read a changed field (`acp_test.go`, `json_parity_test.go`, `defaults_test.go`, `union_wire_test.go`,
  `elicitation_validate_test.go`, `generated_output_test.go`, `example_*_test.go`), changed only as far as the new types and the decided
  `Validate` require
* `README.md` and `docs/architecture.md`

Out of scope: `_meta` and the `mcp/message` `params` maps (still `map[string]any`); `RequestId`'s `Null` decoding of objects; `cancelRequestParams`;
ACP v2 schemas; rendering in memory and type-checking the output (0011 phase 5); engine tests beyond this plan's (0011 phase 6); the goldens; a tag, a
push, or an agent commit on `main`.

## Grounding

Measured on `08821e9` with schema `1.25.0`; the MADR's S1 to S9 hold the counts. Code facts:

* **G1. Types.** `primitiveRef` (`model/fields.go:159`) maps `integer` to `int`, `number` to `float64`, and anything else to `any`, without reading
  `format`; `typeRef` and `optionalTypeRef` call it for properties, `buildType` for named primitives, and `inlineOf` for title-only variants. `Facts`
  (`fields.go:90`) records `Format`, `Minimum`, `Maximum`, `MinLength`, `Values`, `DefaultOnError`, `SkipInvalidItems`; no emitter reads it.
* **G2. Loader.** `load.Definition` (`load/load.go:36`) declares 26 keywords and decodes strictly (`DisallowUnknownFields`), so an undeclared keyword
  fails the load; `format` values are not checked.
* **G3. Struct codecs.** `emitObjectDefaults` (`emit/types.go`) gives a struct `MarshalJSON` and `UnmarshalJSON` only when it has a default, a fill, or
  a catch-all's `Extra` (43 `UnmarshalJSON` today). Its `UnmarshalJSON` decodes into a fresh alias and then a raw map, fills absent keys with their
  defaults, and keeps undeclared keys in `Extra`.
* **G4. Validate.** `validateOf` (`model/model.go`) gives unions `ValidateExactlyOne` (oneOf) or `ValidateShared` (anyOf), RPC types `ValidateFields`,
  and `ToolCallUpdate` `ValidateListed` from the table rule; `requiredCheck` emits `== ""` and `== nil` checks; receivers are `u`, `v`, or `t`.
* **G5. Tags.** `objectFields` gives an optional field `,omitempty`, including the 8 struct-valued fields of S7.
* **G6. Encoding/json v1 in Go 1.27.2**, by probe: a sized integer rejects an out-of-range number with `UnmarshalTypeError`; an outer struct field
  shadows an embedded one with the same JSON name; `omitempty` writes a zero struct as `{}`; a decode continues after a type error and reports the first.
* **G7. Hand-written users of changed types.** `helpers.go:172` to `:257` set `RawInput`/`RawOutput` from `any` and type-assert `map[string]any`;
  `example/client`, `example/claude-code`, `example/gemini` do `int` arithmetic on `ReadTextFileRequest.Line` and `.Limit`; seven root test files
  build values with changed fields.

## Design

### The registry

`model/keywords.go` holds `keywords`, a map from each JSON key `load.Definition` declares to a `Treatment` (`Structure`, `Type`, `Decode`, `Encode`,
`Validate`, `Annotation`, `Protocol`), and `numericFormats`, a map from each integer and number `format` to its Go type:

```text
int8 int16 int32 int64 → int8 int16 int32 int64      uint8 uint16 uint32 uint64 → uint8 uint16 uint32 uint64
int → int64    uint → uint64    (none) on integer → int64    float → float32    double, (none) on number → float64
```

`Build` refuses an `integer` or `number` node whose `format` is not in `numericFormats`: `model: <Definition>.<property>: format "<f>" has no Go type`.
A `format` on a `string` is an annotation.

### Types

* `primitiveRef` takes the node and reads `format`; named primitives, properties, pointers, slice items, and variants all go through it.
* A node that constrains no type (no `type`, `$ref`, `enum`, `const`, or composition), or whose only type is `null`, is `RefRaw`, rendered
  `json.RawMessage`; as the value type of an `additionalProperties: true` map it stays `any`.
* `additionalProperties` with a schema gives `map[string]T` where `T` is `typeRef` of that schema.
* An optional struct-valued field (a named object type, not a pointer) gets no `omitempty`.

### Decoding

A struct with a required key, a lenient field, a default, a fill, or a catch-all gets one generated `UnmarshalJSON`, and the file gets three
unexported helpers once:

```go
// objectKeys decodes b as a JSON object and fails, naming typ and the key,
// when a required key is absent.
func objectKeys(b []byte, typ string, required ...string) (map[string]json.RawMessage, error)
// orDefault decodes raw into a T, or returns fallback when raw is empty or does not decode.
func orDefault[T any](raw json.RawMessage, fallback T) T
// skipInvalid decodes raw as an array, dropping items that do not decode into a T.
func skipInvalid[T any](raw json.RawMessage) ([]T, error)
```

```go
func (v *ToolCall) UnmarshalJSON(b []byte) error {
	m, err := objectKeys(b, "ToolCall", "title", "toolCallId")
	if err != nil {
		return err
	}
	type alias ToolCall
	var s struct {
		alias
		Content json.RawMessage `json:"content,omitempty"`
		Kind    json.RawMessage `json:"kind,omitempty"`
		// one json.RawMessage per lenient field, shadowing the alias's
	}
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("ToolCall: %w", err)
	}
	*v = ToolCall(s.alias)
	if items, err := skipInvalid[ToolCallContent](s.Content); err == nil {
		v.Content = items // default-on-error: a non-array keeps the fallback
	}
	v.Kind = orDefault(s.Kind, v.Kind)
	// absent keys with a schema default, and a catch-all's Extra, from m, as today
	_ = m
	return nil
}
```

The required list is the struct's required keys in sorted order, without the discriminator for an inline variant of a discriminated union. A lenient
field's fallback is its schema default when it declares one, else the zero value. A skip-only field (none at `1.25.0`) returns
`fmt.Errorf("<Type>: %q: %w", key, err)` when the value is not an array. `MarshalJSON` is unchanged.

### Validate

Each type gets `Validate` when it is a union, an RPC type, has a `minLength` or an out-of-range bound, or has a field whose type, slice element, map
value, or pointer target has `Validate` (a fixed point over the model). The generated checks, in field order:

* a union: exactly one variant or `Unknown` set (`<Union> must have exactly one variant set`), then the set variant's `Validate`;
* `minLength n`: `utf8.RuneCountInString(v.F) < n` → `<Type>: <key>: shorter than n characters`;
* a bound outside the Go type's range: `<Type>: <key>: less than` / `greater than <bound>`;
* a nested value: `if err := v.F.Validate(); err != nil { return fmt.Errorf("<Type>: <key>: %w", err) }`, through a nil check for a pointer, an index
  loop for a slice (`<key>[%d]`), and sorted keys for a map (`<key>[%q]`).

`ValidateKind`'s four kinds, `requiredCheck`, `RequiredCheck`, the table's `Validate` rule and `ValidateRule`, and `ValidateListed` are removed. A
union's methods take the receiver `u`; every other type's take `v`.

## Implementation Steps

1. Work on `main`. Each step writes its tests first and records their failure; changes the generator and any hand-written file it names; regenerates
   (`cd cmd/generate && env -u GOPATH -u GOMODCACHE go run .`); classifies every changed line of the generated files by script against the step's
   expectation; shows each new check failing on a planted breakage in a scratch copy; runs `make pre-add-check`, `make check-records`, and
   `make markdownlint`; records the output in this plan's execution record; and stops for the owner's commit. No golden in `testdata/json_golden/`
   changes in any step; a root test fixture changes only to the new type of a field it sets, and each change is listed.
2. **The registry.** Tests: `TestKeywordRegistryMatchesLoader` (the registry's keys equal the `json` tag names of `load.Definition`, by reflection);
   `TestCommittedSchemaKeywordsRegistered` (every keyword and every numeric `format` in both schema files is registered); `TestUnknownNumericFormat`
   (`{"type":"integer","format":"int128"}` is refused, naming the property). Add `keywords.go` and the refusal. Expected: no generated file changes.
3. **Numeric formats.** Tests: the model maps each `format` of the table (a synthetic definition per format); root: `-1` into
   `CreateTerminalRequest.outputByteLimit`, `4294967296` into `ReadTextFileRequest.line`, and `65536` into `ProtocolVersion` each fail;
   `ResourceLink.size` 9007199254740993 round-trips. Change `primitiveRef` and its callers; fix the compile errors in the three examples (convert
   `Line` and `Limit` to `int` at the slice bounds) and in root tests. Expected: in `types_gen.go`, exactly the 46 fields and 12 named types of S1
   change type, and no other line; `helpers_gen.go` changes only where a constructor parameter is one of those fields.
4. **Typed maps.** Test: `AuthMethodTerminal.env` decodes to `map[string]string`, and `{"env":{"A":1}}` fails; `ElicitationSchema.properties` holds
   `ElicitationPropertySchema` values. Expected: exactly the 6 fields of S2 change type.
5. **Arbitrary JSON.** Tests: `rawInput` `{"n":9007199254740993}` round-trips with every digit through `SessionUpdate`; `Error.data` and the envelope
   `params` keep their bytes; `WithStartLocations` adds `path` to a raw object input without touching its other keys. Add `RefRaw`; change
   `helpers.go`'s four options to `json.RawMessage` and `WithStartLocations` to merge through `map[string]json.RawMessage`; fix root tests.
   Expected: exactly the 18 fields of S3 change to `json.RawMessage`.
6. **Struct-valued `omitempty`.** Test: the 8 fields' tags carry no `omitempty`, and `InitializeResponse{}` still encodes `agentCapabilities` as an
   object. Expected: exactly 8 tags change; no golden changes.
7. **Required keys.** Tests: `PromptRequest` without `sessionId` fails naming `PromptRequest` and `sessionId`; dispatch answers it `-32602`; an
   `ImageContent` block without `data` fails; `ContentBlock` `{"text":"hi"}` still decodes as `Text` (phase 2); `null` into a struct with a required
   key fails. Add `objectKeys` and the generated decoders for required keys, merged with the existing default, fill, and `Extra` code. Expected: every
   struct with a required key gains or extends `UnmarshalJSON`; the helpers appear once. A root fixture that omits a required key is schema-invalid:
   it is corrected and listed; a failure on a schema-valid payload stops the step for the owner.
8. **`x-deserialize-*`.** Tests: `InitializeResponse` `{"protocolVersion":1,"agentCapabilities":"garbage"}` decodes with zero capabilities; a field
   with a schema default falls back to it; `Plan` `{"entries":[{"content":"x","priority":"high","status":"pending"},5]}` keeps one entry; `Plan`
   `{"entries":"x"}` gives no entries, and `Plan` `{}` fails; a `ToolCall` whose `content` holds an unknown variant keeps it in `Unknown`. Add
   `orDefault` and `skipInvalid` and the shadow fields. Expected: every struct with a lenient field gains or extends `UnmarshalJSON`.
9. **Validate.** Tests: a `PromptRequest` with an empty `ContentBlock` fails `Validate`, naming `prompt[0]`; `{"sessionId":""}` passes; a `Notice` with
   an empty title fails; a map value and a pointer target are checked; `CreateElicitationRequest{}` fails and a form with an empty message passes
   (`elicitation_validate_test.go`'s message test becomes a decode test for the missing key). Replace `validateOf` and the emitter as designed; remove
   the table's validate rule; unify receivers. Expected: every type with `Validate` at `08821e9` still has it; new ones are listed.
10. **Docs.** `README.md` ("Wire encoding", helper options) and `docs/architecture.md` state the registry, the treatments, and the new types; the name
    check of 0011's plans runs over both.
11. **Review and close.** Run the verification below, record the exported API changes for the release notes, and set this plan to `complete`.

## Verification

* `TestKeywordRegistryMatchesLoader` and `TestCommittedSchemaKeywordsRegistered` pass; each test this plan adds failed first; each planted breakage
  fails a test.
* Each step's generated diff matches its expectation by script; no golden in `testdata/json_golden/` changes in the phase.
* The probes of the MADR's S1, S3, S4, S5, S6 pass as tests.
* `go test -race ./...` passes at the root and in `cmd/generate`; `go build ./example/...` passes; `TestGeneratedFilesUpToDate` passes.
* `go test -cover ./internal/model` stays at or above 85%; `./internal/emit` is reported.
* The exported API difference from `08821e9`, by the API listing program of 0011's plans, is: the 58 integer declarations, 6 map fields, and 18 raw
  fields with their new types; the four raw options' parameter types; new `UnmarshalJSON` and `Validate` methods; nothing removed.
* `make pre-add-check`, `make check-records`, and `make markdownlint` pass.

## Rollout and Rollback

Commits land on `main`, made by the owner, one per step. No tag; a push waits for the owner's ask. The next minor version's release notes list every
changed field type and signature from the API difference, and the behaviour changes: a missing required key fails with `-32602`; malformed lenient
fields fall back and invalid lenient items are dropped; `Validate` no longer rejects empty strings and now checks nested values.

Rollback is `git revert` of the step commits, newest first; each step leaves the generator and the generated files in step.

## Execution record

Not started.
