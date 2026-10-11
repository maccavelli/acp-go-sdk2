---
status: in-progress
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
* ~~A node that constrains no type (no `type`, `$ref`, `enum`, `const`, or composition), or whose only type is `null`, is `RefRaw`, rendered
  `json.RawMessage`; as the value type of an `additionalProperties: true` map it stays `any`.~~ (Amended 2026-10-10, see the deviation below.) A
  property or variant the model would give `any` is `RefRaw`, rendered `json.RawMessage`; a map value it would give `any` stays `any`.
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

## Deviation 2026-10-10, the rule for raw JSON

Found in step 5, before any change: the design's rule covered 12 of S3's 18 fields; the six envelope `params` and `result` are inline `anyOf`
compositions the model does not name. Decision: the owner chose "whatever has no Go type" (the MADR's amendment of the same date). The step's
expectation is unchanged: exactly the 18 fields of S3 become `json.RawMessage`. No file is added to the step.

## Deviation 2026-10-10, StartEditToolCall's content

Found in step 5, before changing it: `StartReadToolCall` and `StartEditToolCall` in `helpers.go` build their raw input from a map, and the step named
only the four options and `WithStartLocations`. Decision: the owner chose `StartEditToolCall(id, title, path string, content json.RawMessage,
opts...)` (the MADR's amendment of the same date). `StartReadToolCall` keeps its signature and builds `{"path":…}` the way `WithStartLocations` does.
Both stay in `helpers.go`, already in scope; the release notes list the fifth signature.

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

### Step 2, the registry, 2026-10-10

The owner committed the MADR and this plan as `bc33916`, accepted the MADR, and approved the plan. The MADR's status is `accepted` and this plan's
`in-progress`, here and in `docs/README.md`. Step 2 ran on `main` from that clean tree. The agent did not commit.

Tests, in a new `cmd/generate/internal/model/keywords_test.go`:

* `TestKeywordRegistryMatchesLoader`: the registry's keys equal the `json` tag names of `load.Definition`, read by reflection, and each has a treatment.
* `TestCommittedSchemaKeywordsRegistered`: every key of every schema node of `schema.json` and `schema.unstable.json` is registered, and every
  `format` on an integer or number node has a Go type.
* `TestUnknownNumericFormat`: `int128` on an integer, `decimal` on a nullable number, and `u64` on an array's integer items are refused, naming the
  node; `email` on a string is accepted.

Run before the change, the package did not compile:

```text
internal/model/keywords_test.go:29:36: undefined: keywords
internal/model/keywords_test.go:32:21: undefined: keywords
internal/model/keywords_test.go:88:17: undefined: keywords
internal/model/keywords_test.go:92:47: undefined: numericFormat
```

The change: `model/keywords.go` (new) holds `Treatment`, the 26-keyword registry `keywords` with the treatments of the MADR's table, `integerFormats`,
`numberFormats`, and `numericFormat`. `checkRefs`' walk, which visits every node with its path, now also refuses an integer or number whose format
`numericFormat` does not know: `<path>: format "<f>" on <type> has no Go type`. The maps are not yet used for types; step 3 uses them.

Regenerated: each of the five generated files compares equal, by `cmp`, to its copy from `bc33916`.

Each part was seen to fail, one breakage per fresh scratch copy, then `go test ./...` in `cmd/generate`:

```text
keyword missing from the registry: failing=['TestCommittedSchemaKeywordsRegistered', 'TestKeywordRegistryMatchesLoader']
keyword the loader does not declare: failing=['TestKeywordRegistryMatchesLoader']
keyword without a treatment: failing=['TestKeywordRegistryMatchesLoader']
format check off: failing=['TestUnknownNumericFormat']
uint16 unknown: failing=['TestCommittedSchemaKeywordsRegistered', 'TestFitKeysOfCommittedSchema', 'TestGeneratedFilesUpToDate', 'TestProtocolTableAppliesToCommittedSchema', 'TestUnionClassesOfCommittedSchema']
```

The first gate run found `goconst` on the repeated `"int64"` and `"uint64"`; they are the constants `goInt64` and `goUint64`. The plants ran before
that change, which does not change behaviour; the tests and the comparison were rerun after it.

After the change every test in the root and in `cmd/generate` passes. Coverage: `internal/model` 94.1%, `internal/emit` 54.2%.

Gates: `make pre-add-check` with the three changed Go files named: `go-precheck: 3 file(s) clean (gofmt, golangci-lint, go vet, go test,
govulncheck).` Without names: `go-precheck: 67 file(s) clean (...)`. `make check-records` and `make markdownlint` exit 0.

Commit point: the agent stopped here. The owner commits step 2 on `main`.

### Step 3, numeric formats, 2026-10-10

The owner committed step 2 as `13809b6`. Step 3 ran on `main` from that clean tree. The agent did not commit.

Tests:

* `TestNumericFormatTypes` (`model/keywords_test.go`): for each of the 14 format rows of the design, a synthetic schema gives the format to a named
  primitive, a union variant, and five properties (required, optional, `["T","null"]`, `anyOf [T, null]`, array items), and requires the Go type
  (`T`, `T`, `*T`, `*T`, `[]T`).
* `TestIntegerFormatsBoundDecoding` (new root `fidelity_test.go`): `-1` into `CreateTerminalRequest.outputByteLimit`, `4294967296` into
  `ReadTextFileRequest.line`, `65536` into `InitializeRequest.protocolVersion`, and `9223372036854775808` into `ResourceLink.size` fail with
  `cannot unmarshal number`; `9007199254740993` in `size` decodes exactly.

Run before the change, the model test reported 74 mismatches (`integer "": property r is int, want int64` and on), and the root test failed three of
its four cases (`decoded {"sessionId":"s","command":"c","outputByteLimit":-1}: <nil>` and the `line` and `protocolVersion` cases); the `int64` case
already failed to decode on this 64-bit host, where `int` is 64 bits, and stays as a guard.

The change: `primitiveRef` and `pointerPrimitive` (`model/fields.go`) take the node's format and give an integer or number the Go type of
`numericFormat`; their four callers pass it (`typeRef`, both branches of `optionalTypeRef`, and `buildType`'s named primitive in `model.go`).

Corrections to tests, recorded: `TestTypeRefs` expected `int` and `*int` for an integer without a format, which the MADR now maps to `int64`;
`TestObjectFields`' `count` fixture declares `uint64` and is now `*uint64`; and the first draft of `TestNumericFormatTypes` expected an optional,
non-nullable integer to be a pointer, where the generator writes a value with `omitempty`, as for every optional non-nullable property.

Regenerated: only `types_gen.go` changed. A script compared it line by line with `13809b6`'s: 46 fields (`int64` 12, `uint64` 17, `uint32` 16,
`int32` 1) and 12 named types (`int32` 9, `int64` 2, `uint16` 1) changed from `int` to a sized type, and no other line changed. `helpers_gen.go` is
unchanged: no constructor takes one of these fields. No golden changed.

Hand-written code, to the new types:

* `example/client`, `example/claude-code`, `example/gemini` (`main.go`) and `example_client_test.go`: the `ReadTextFile` line and limit arithmetic
  compares in `uint64` and converts to `int` only once the value is known to fit the slice, so a large `uint32` cannot overflow `int` on a 32-bit
  platform.
* `json_parity_test.go` (two `size` fixtures `int64(1024000)`, and `line, limit` as `uint32`) and `union_wire_test.go` (`size` as `int64`).

Each call site was broken in a fresh scratch copy, then the generator run and both modules tested:

```text
format ignored everywhere: cmd/generate ['TestNumericFormatTypes', 'TestObjectFields']; root: build failed
nullable property ignores format: cmd/generate ['TestNumericFormatTypes', 'TestObjectFields']; root: build failed
anyOf-null property ignores format: cmd/generate ['TestNumericFormatTypes']
named primitive ignores format: cmd/generate ['TestNumericFormatTypes']; root ['TestIntegerFormatsBoundDecoding']
```

The `anyOf`-null plant first passed every test: no committed property has that shape, and the test covered only `["T","null"]`. The test gained the
`anyOf [T, null]` property, and the plant then failed.

After the change every test in the root (`go test -race`) and in `cmd/generate` passes; `go build ./example/...` passes. Coverage: `internal/model`
94.0%, `internal/emit` 54.2%.

Gates: `make pre-add-check` with the twelve changed Go files named: `go-precheck: 12 file(s) clean (gofmt, golangci-lint, go vet, go test,
govulncheck).` Without names: `go-precheck: 69 file(s) clean (...)`. `make check-records` and `make markdownlint` exit 0.

Commit point: the agent stopped here. The owner commits step 3 on `main`.

### Step 4, typed maps, 2026-10-10

The owner committed step 3 as `b1ee848`. Step 4 ran on `main` from that clean tree. The agent did not commit.

`TestTypedAdditionalProperties` (root `fidelity_test.go`) requires the six fields of S2 to have their map types, by reflection
(`AuthMethodTerminalInline.Env` and `AuthMethodTerminal.Env` `map[string]string`, `CreateElicitationAccept.Content` and
`ElicitationAcceptAction.Content` `map[string]ElicitationContentValue`, `ElicitationSchema.Properties` `map[string]ElicitationPropertySchema`,
`UnstableSetProviderRequest.Headers` `map[string]string`), an `AuthMethodTerminal` with `"env":{"A":"1"}` to decode, and one with `"env":{"A":1}` to
fail. Run before the change it failed seven times: each field was `map[string]interface {}`, and the number decoded.

The change: `typeRef`'s object case (`model/fields.go`) types the map's values with `typeRef(additionalProperties)`; `additionalProperties: true`, or
none, stays `any`.

`TestObjectFields`' fixture `env` declares `"additionalProperties":{"type":"string"}`; its expected type is now `map[string]string`, as decided.

Regenerated: `types_gen.go` changed in exactly six lines, the six field declarations above, and nothing else; the default fill of
`ElicitationSchema.properties` (`{}`) did not change. No golden changed; no hand-written file needed a change.

The rule was broken in a fresh scratch copy (the values left `any`): `cmd/generate` failed `TestObjectFields` and the root failed
`TestTypedAdditionalProperties`.

After the change every test in the root (`go test -race`) and in `cmd/generate` passes; `go build ./example/...` passes. Coverage: `internal/model`
94.1%, `internal/emit` 54.2%.

Gates: `make pre-add-check` with the four changed Go files named: `go-precheck: 4 file(s) clean (gofmt, golangci-lint, go vet, go test,
govulncheck).` Without names: `go-precheck: 70 file(s) clean (...)`. `make check-records` and `make markdownlint` exit 0.

Commit point: the agent stopped here. The owner commits step 4 on `main`.

### Step 5, arbitrary JSON, 2026-10-10

The owner committed step 4 as `42fd612`. Step 5 ran on `main` from that clean tree. The agent did not commit.

Two deviations were found and decided before the code changed; both are recorded above and in the MADR's amendments: the rule for raw JSON is "whatever
has no Go type" (the six envelope fields included), and `StartEditToolCall`'s `content` is `json.RawMessage`.

Tests (root `fidelity_test.go`), written first:

* `TestRawJSONKeepsDigits`: `{"n":9007199254740993}` as a `tool_call`'s `rawInput`, an `Error`'s `data`, and an `AgentRequest`'s `params` encodes back
  with every digit.
* `TestStartLocationsMergesRawInput`: `WithStartLocations` gives no input `{"path":"/p"}`, adds `path` to an object input
  (`{"n":9007199254740993,"path":"/p"}`), keeps an object's own `path`, and leaves an array input alone.

Run before the change, four failures:

```text
object input: rawInput {"n":9007199254740993}, <nil>; want {"n":9007199254740993,"path":"/p"}
SessionUpdate rawInput: encoded {"rawInput":{"n":9007199254740992},"sessionUpdate":"tool_call","title":"x","toolCallId":"t"}, <nil>
Error data: encoded {"code":-32000,"data":{"n":9007199254740992},"message":"m"}, <nil>
AgentRequest params: encoded {"id":1,"method":"x","params":{"n":9007199254740992}}, <nil>
```

The change:

* `model/fields.go`: `RefRaw` and `rawJSON()`; `typeRef`'s fallback for a node with no Go type is `rawJSON()`; `mapValues` gives a map's values the
  type of its `additionalProperties`, or `any` where that type would be raw, and `factsOf` uses it.
* `emit`: `typeAST` takes the `goFile`, so `RefRaw` renders as `json.RawMessage` with its import; its five callers pass the file.
* `helpers.go`: `WithStartRawInput`, `WithStartRawOutput`, `WithUpdateRawInput`, `WithUpdateRawOutput` take `json.RawMessage`; `WithStartLocations`
  merges `path` through `withRawPath`, which decodes an object into `map[string]json.RawMessage` and leaves any other input unchanged;
  `StartReadToolCall` uses `withRawPath(nil, path)`; `StartEditToolCall(..., content json.RawMessage, ...)` builds `{"content":…,"path":…}` in
  `editRawInput`, writing `content` as given.
* `example/agent/main.go`, `example_agent_test.go`, `json_parity_test.go`: the maps they passed are `json.RawMessage` literals with their keys in
  the order `encoding/json` writes a map, so the goldens keep their bytes.

Test corrections, recorded: `TestTypeRefs` expected `any` for `{}` and for the plain `typeRef` of two `anyOf`-with-`null` nodes; under the amended
rule each is `json.RawMessage`.

Regenerated: only `types_gen.go` changed, in 21 lines. A script classified them: 18 fields changed from `any` to `json.RawMessage` (`Data` 2,
`Params` 4, `RawInput` 4, `RawOutput` 4, `Result` 3, `StopReason` 1), and 3 lines changed only in whitespace, realigned by `gofmt` in the same
structs (`Meta` 1, `SessionUpdate` 2). No golden changed.

Each rule was broken in a fresh scratch copy, then the generator run and both modules tested:

```text
no Go type stays any: cmd/generate ['TestTypeRefs']; root: build failed
map values become raw: cmd/generate ['TestTypeRefs']; root: build failed
path not merged into an object: root ['TestStartLocationsMergesRawInput']
edit input keys reordered: root ['TestStartEditToolCallRawInput']
```

Two of these first passed and showed gaps: "map values become raw" failed only the root build, since no generator test held that an
`additionalProperties: true` map keeps `any` values (`TestTypeRefs` gained that row); and "edit input keys reordered" passed everything, since the
golden comparison is semantic (`TestStartEditToolCallRawInput` now requires the raw input byte for byte, for a string, an object with a large integer,
and nil content).

The first gate run found `errcheck` on `editRawInput`'s ignored error from encoding the path; it is checked, and returns nil on that branch, which a
string cannot reach.

After the change every test in the root (`go test -race`) and in `cmd/generate` passes; `go build ./example/...` passes. Coverage: `internal/model`
94.1%, `internal/emit` 54.1%.

Gates: `make pre-add-check` with the ten changed Go files named: `go-precheck: 10 file(s) clean (gofmt, golangci-lint, go vet, go test,
govulncheck).` Without names: `go-precheck: 70 file(s) clean (...)`. `make check-records` and `make markdownlint` exit 0.

Commit point: the agent stopped here. The owner commits step 5 on `main`.

### Step 6, struct-valued `omitempty`, 2026-10-10

The owner committed step 5 as `633281b`. Step 6 ran on `main` from that clean tree. The agent did not commit.

The plan's 8 fields came from a probe of named object types only. A union wrapper and a nested struct are struct values too, so a scratch probe listed
every field tagged `omitempty` whose Go type is any of the three: the same 8 fields, all named objects with a schema `default`; no union or nested
struct. The rule covers all three kinds.

Tests:

* `TestNoOmitemptyOnStructValues` (`model/keywords_test.go`): no struct-valued field of the committed model carries `omitempty`; in a synthetic
  schema, optional properties of an object, a union, and an inline nested object carry none, and an optional string keeps it.
* `TestStructValuesHaveNoOmitempty` (root `fidelity_test.go`): the 8 fields' `json` tags, read by reflection, carry no `omitempty`, and
  `InitializeResponse{}` still encodes `"agentCapabilities":{`.

Run before the change:

```text
keywords_test.go:234: struct-valued fields with omitempty: [AgentCapabilities.auth AgentCapabilities.mcpCapabilities AgentCapabilities.promptCapabilities AgentCapabilities.sessionCapabilities ClientCapabilities.auth ClientCapabilities.fs InitializeRequest.clientCapabilities InitializeResponse.agentCapabilities]
keywords_test.go:248: synthetic struct-valued fields with omitempty: [T.inner T.obj T.uni]
fidelity_test.go:154: acp.AgentCapabilities.Auth tag "auth,omitempty" (and the other 7)
```

The change: `Model.dropStructOmitempty` (`model/fields.go`), run in `resolve` once every type is built, trims `,omitempty` from each field whose
type is a named object, a union, or a nested struct.

Regenerated: `types_gen.go` changed in exactly 8 lines, each one of the 8 fields losing `,omitempty` from its tag. No golden changed; no hand-written
file needed a change.

Each part was broken in a fresh scratch copy:

```text
pass not run: cmd/generate ['TestNoOmitemptyOnStructValues']; root ['TestStructValuesHaveNoOmitempty']
unions left out: cmd/generate ['TestNoOmitemptyOnStructValues']
nested structs left out: cmd/generate ['TestNoOmitemptyOnStructValues']
```

The last two pass the root tests, as expected: the committed schema has no optional union or nested struct value; the synthetic model case catches
them.

After the change every test in the root (`go test -race`) and in `cmd/generate` passes; `go build ./example/...` passes. Coverage: `internal/model`
94.2%, `internal/emit` 54.1%.

Gates: `make pre-add-check` with the five changed Go files named: `go-precheck: 5 file(s) clean (gofmt, golangci-lint, go vet, go test,
govulncheck).` Without names: `go-precheck: 70 file(s) clean (...)`. `make check-records` and `make markdownlint` exit 0.

Commit point: the agent stopped here. The owner commits step 6 on `main`.

### Step 7, required keys, 2026-10-10

The owner committed step 6 as `d9ffc24`. Step 7 ran on `main` from that clean tree. The agent did not commit. The owner committed the step's code and
first tests as `3588f8a` while its plants were running; the emit test `TestRequiredKeysDecoder` and this record came after and follow that commit.

Tests, written first:

* `TestRequiredKeysOnDecode` (root `fidelity_test.go`): `PromptRequest` without `sessionId`, `InitializeRequest` without `protocolVersion`, an image
  `ContentBlock` without `data`, and `null` into a `PromptRequest` fail, naming the type and the property; `ContentBlock` `{"text":"hi"}` still decodes
  as `Text`; dispatch of `session/prompt` without `sessionId` answers `-32602` with the missing property, without reaching the agent.
* `TestDiscriminatorExemptOnDecode` (`model/classes_test.go`): every inline variant of a union records its discriminator as `Inline.Exempt`, and no
  `$ref` variant's own type requires its union's discriminator, which a payload placed without the key decodes through.

Run before the change, the root test failed five times: each decode returned `<nil>`, and dispatch panicked on the nil agent
(`invalid memory address or nil pointer dereference`). That panic is the current gap: `PromptRequest`'s `Validate` did not see the missing `sessionId`,
because its `""` check skips a `$ref`'d string, so dispatch went on to call the agent. The model test did not compile (`v.Inline.Exempt undefined`).

The change:

* `model`: `Inline.Exempt`, set in `classifyUnion` to the union's discriminator for each inline variant.
* `emit/decode.go` (new): `helperObjectKeys` and `emitDecodeHelpers`, which emits `objectKeys` once when the file used it; `goFile.uses` records it.
* `emit/types.go`: `emitObjectDefaults` became `emitObjectCodec`, which emits `MarshalJSON` (`emitObjectMarshal`, as before) and, for a struct with a
  required key, a default, or a catch-all, `UnmarshalJSON` (`emitObjectUnmarshal`): `objectKeys(b, "<Type>", <required, sorted>)`, then the alias
  decode wrapped as `<Type>: %w`, then the defaults and `Extra` as before. `emitStruct` takes the exempt key.

Corrections, recorded:

* The two emit tests that type-check one struct (`TestDefaultedNullablePropertyCompiles`, `TestRequiredMapFilledOnEncode`) now call
  `emitDecodeHelpers`, since the struct calls `objectKeys`.
* `TestAbsentDiscriminator`'s row `AuthMethod agent without its keys` (`{"id":"a"}` set `Agent` by trial decode) is schema-invalid: the `agent` variant
  requires `id` and `name`. It is now `AuthMethod agent missing a required key` and expects `AuthMethod: missing discriminator "type"`.

Found and not changed: since a key-less variant's decoder now requires its keys, the trial decode that phase 2 emits for key-less variants after the
key match can no longer succeed; it is dead code in five unions (`AuthMethod`, `McpServer`, `UnstableMcpServer`,
`SetSessionConfigOptionRequest`, `MultiSelectItems`; `IdleStateUpdate`'s `None` has no required key and no trial). Removing it changes the union decoders of
[0011-PLAN-wire-fixes.md](0011-PLAN-wire-fixes.md) and is not in this step.

Regenerated: only `types_gen.go` changed, from 12,190 to 14,823 lines. Compared declaration by declaration, each with its own doc comment: 621 to 809
declarations, none removed; 188 added, 187 `UnmarshalJSON` and the function `objectKeys`; 15 changed, all `UnmarshalJSON` of structs that had one
for defaults or `Extra`; the 28 union decoders unchanged. `UnmarshalJSON` methods: 43 before, 230 after; `objectKeys(b, ` is called 202 times. No golden
changed: every golden payload carries its required keys.

Each part was broken in a fresh scratch copy, then the generator run and both modules tested:

```text
presence never fails: cmd/generate []; root ['TestAbsentDiscriminator', 'TestRequiredKeysOnDecode']
required keys not listed: cmd/generate ['TestRequiredKeysDecoder']; root ['TestAbsentDiscriminator', 'TestRequiredKeysOnDecode']
discriminator not exempt in the emitter: cmd/generate ['TestRequiredKeysDecoder']; root ['TestAbsentDiscriminator', 'TestRequiredKeysOnDecode']
discriminator not exempt in the model: cmd/generate ['TestDiscriminatorExemptOnDecode', 'TestRequiredKeysDecoder']; root ['TestAbsentDiscriminator', 'TestRequiredKeysOnDecode']
```

Two first passed the generator's tests and failed only at the root: no generator test held the decoder's text. `TestRequiredKeysDecoder`
(`emit/types_test.go`) now requires a struct with required keys `zeta` and `alpha` to call `objectKeys(b, "Options", "alpha", "zeta")`, an inline
union variant to leave out its discriminator, the helper to be present, and the output to compile. The first form of the presence plant made the
generated code fail to compile (an unused `ok`) rather than skip the check; it was rerun with code that compiles. The generator's tests check the
decoder's text and that it compiles; its behaviour is checked at the root.

After the change every test in the root (`go test -race`) and in `cmd/generate` passes; `go build ./example/...` passes. Coverage: `internal/model`
94.3%, `internal/emit` 70.5% (54.1% before).

Gates: `make pre-add-check` on `3588f8a`'s files passed before the owner's commit; with `emit/types_test.go`, the one file changed since, named:
`go-precheck: 1 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck).` Without names: `go-precheck: 71 file(s) clean (...)`.
`make check-records` and `make markdownlint` exit 0.

Commit point: the agent stopped here. The owner commits the rest of step 7 (`emit/types_test.go` and this record) on `main`.

### Step 8, `x-deserialize-*`, 2026-10-10

The owner committed the rest of step 7 as `1ae3f08`. Step 8 ran on `main` from that clean tree. The agent did not commit.

Tests, written first:

* `TestLenientFields` (root `fidelity_test.go`): `InitializeResponse` with `"agentCapabilities":"garbage"` and `"authMethods":"x"` decodes equal to one
  without either key (their schema defaults); `ElicitationSchema` `{"type":5,...}` falls back to its default `"object"`; `Plan` keeps the one valid entry
  of `[valid, 5, {"content":"y"}]`; `{"entries":"x"}` gives no entries; `{}` fails as missing `entries`; a `ToolCall`'s `content` holding an unknown
  variant keeps it in `Unknown`.
* `TestSkipInvalidItemsNeedsAnArray` (`model/keywords_test.go`): the keyword on a string is refused, naming the node; on `["array","null"]` it is
  accepted.

Run before the change:

```text
malformed lenient fields: InitializeResponse: AgentCapabilities: json: cannot unmarshal string into Go value of type map[string]jsontext.Value
malformed fields decode as {...}, want the defaults {...}
ElicitationSchema type 5: ElicitationSchema: json: cannot unmarshal number into Go struct field Alias.type of type acp.ElicitationSchemaType, Type "", want the default object
Plan with invalid entries: Plan: PlanEntry: json: cannot unmarshal number into Go value of type map[string]jsontext.Value, 0 entries, want 1
Plan entries not an array: Plan: json: cannot unmarshal string into Go struct field Alias.entries of type []acp.PlanEntry, []
keywords_test.go:264: error <nil>, want one containing "T.properties.n: x-deserialize-skip-invalid-items on string, not an array"
```

The `Plan {}` and unknown-variant cases passed before, as guards. The first draft stopped at the first failure (`Fatalf`); it reports each case.

The change:

* `model/schema.go`: `checkRefs`' walk refuses `x-deserialize-skip-invalid-items` on a node that is not an array.
* `emit/types.go`: a struct with a lenient field decodes into `var s struct { Alias; <Field> json.RawMessage ... }`, each lenient field shadowed by its
  raw value under the same JSON name, then `a := s.Alias` and, per field: default-on-error alone, `a.F = orDefault[T](s.F, "<default JSON or empty>")`;
  both keywords, `if items, err := skipInvalid[E](s.F); err == nil { a.F = items }`, with an `else` that applies the schema default when there is
  one; skip alone, an error `<Type>: "<key>": <cause>` when the value is not an array.
* `emit/decode.go`: the generic helpers `orDefault[T any](raw json.RawMessage, def string) T` and `skipInvalid[T any](raw json.RawMessage) ([]T,
  error)`, emitted once each.

Correction, recorded: step 3's `TestIntegerFormatsBoundDecoding` expected `ResourceLink.size` `9223372036854775808` to fail; `size` carries
`x-deserialize-default-on-error`, so it now decodes with no size, as in Rust, and the case says so.

Regenerated: only `types_gen.go` changed, from 14,823 to 18,029 lines. By declaration: 809 to 898, none removed; 89 added, 87 `UnmarshalJSON` and the
two helpers; 187 changed, all `UnmarshalJSON`. `UnmarshalJSON` methods: 230 before, 317 after; 274 decode through a shadow struct (the count of structs
with a lenient field); `objectKeys` is called 289 times, `orDefault` 494 (493 lenient fields and one fallback, `InitializeResponse.authMethods`'s `[]`),
`skipInvalid` 35. No golden changed.

`TestLenientDecoder` (`emit/types_test.go`), added for the shapes the committed schema lacks or only the root saw: a string with a default, an array
with both keywords and a default, and an array with skip alone; it requires the shadow field, each call, the fallback, the skip-only error, no shadow for
a plain field, and compiling output.

Each part was broken in a fresh scratch copy:

```text
orDefault ignores the schema default: root ['TestLenientFields']
skipInvalid keeps invalid items: root ['TestLenientFields']
no lenient decoding: cmd/generate ['TestLenientDecoder']; root ['TestIntegerFormatsBoundDecoding', 'TestLenientFields']
no fallback after a non-array: cmd/generate ['TestLenientDecoder']; root ['TestLenientFields']
skip-only ignores a non-array: cmd/generate ['TestLenientDecoder']
skip on a non-array allowed: cmd/generate ['TestSkipInvalidItemsNeedsAnArray']
```

The first form of "keeps invalid items" compared two separately created errors, which never match, so it changed nothing and passed; the plant now
appends every item. The helpers' behaviour is checked at the root; the generator's tests check the text and that it compiles.

After the change every test in the root (`go test -race`) and in `cmd/generate` passes; `go build ./example/...` passes. Coverage: `internal/model`
94.3%, `internal/emit` 73.8%.

Gates: `make pre-add-check` with the seven changed Go files named: `go-precheck: 7 file(s) clean (gofmt, golangci-lint, go vet, go test,
govulncheck).` Without names: `go-precheck: 71 file(s) clean (...)`. `make check-records` and `make markdownlint` exit 0.

Commit point: the agent stopped here. The owner commits step 8 on `main`.

### Step 9, Validate, 2026-10-10

The owner committed step 8 as `7eedeba`. Step 9 ran on `main` from that clean tree. The agent did not commit.

Tests, written first:

* `TestValidateSchemaConstraints` (root `fidelity_test.go`), through a `Validate() error` interface so that it compiled before the methods existed: a
  `PromptRequest` with an empty `ContentBlock` fails naming `prompt[0]`; a `ReadTextFileRequest` with an empty `path` passes; a `Notice` with an empty
  title fails its `minLength`; an empty union as a map value of `ElicitationSchema.properties` and as the pointer target `AvailableCommand.input` fails;
  `CreateElicitationRequest{}` fails and a form with an empty message passes; a valid prompt passes.
* `elicitation_validate_test.go`, as the plan says: its message test is now `TestCreateElicitationRequest_MessageIsRequired`, which requires decoding
  a form without `message` to fail, the empty request to fail `Validate`, and a valid form to pass.

Run before the change, seven failures:

```text
empty block in a prompt: <nil>, want "PromptRequest: prompt[0]: ContentBlock must have exactly one variant set"
empty required string: path is required, want it to pass
title below minLength: *acp.Notice has no Validate
map value: *acp.ElicitationSchema has no Validate
pointer target: *acp.AvailableCommand has no Validate
no variant: message is required, want "CreateElicitationRequest must have exactly one variant set"
form with an empty message: message is required, want it to pass
```

The elicitation test passed before: decoding has required `message` since step 7. A case added after the change, "set variant validates"
(`SessionUpdate{UserMessageChunk: &SessionUpdateUserMessageChunk{}}` fails through the variant's `content`), covers a union calling its variant.

The change:

* `model/validate.go` (new): `Validation`, `Check`, `CheckKind`; `buildValidations`, run in `resolve` after the types are built: every union and RPC
  object validates; a struct's own checks are `minLength` and any bound outside its Go type's range (`goRange`); a struct with a field whose type
  validates, as a value, pointer, slice element, or map value, validates, to a fixed point; `Variant.Validates` records which variants the union
  calls; a validated type deeper than one level, a bound on a type it cannot apply to, and a bound a named primitive type does not enforce are refused.
* `model`: `Type.Validation`, `Struct.Validation`, `Inline.Validation`, and `Model.Validates` replace `ValidateKind`, `RequiredCheck`, `validateOf`,
  and `requiredCheck`; the protocol table's `Validate` field, `ValidateRule`, its check, and `ACP`'s `ToolCallUpdate` rule are removed.
* `emit/types.go`: `emitValidate` and `checkStmt` render a struct's checks; `unionValidateBody` requires exactly one variant or `Unknown` and calls
  the set variant's `Validate`; `requiredCheck`, `sharedUnionValidateBody`, and `emitFieldValidate` are gone; a union's methods take `u`, every other
  type's `v`.
* `model/keywords.go`: the Go number types are constants (`goInt8` … `goFloat64`), shared with `validate.go`.

Tests rewritten with the removed API, as decided: `TestValidateKinds` is now `TestValidations` (a union and an RPC object with no checks, a plain
object without `Validate`, `minLength`, an `int32` minimum and a `double` maximum checked and a `uint32` minimum of 0 not, a two-level fixed point, the
four nested shapes, and the refusal of `[][]U`); `protocol_test.go` loses the validate rule's test case, refusal case, and committed-schema loop.
`TestValidateMethods` (`emit/types_test.go`) requires the rendered checks, a union calling only its validating variant, and compiling output.

Regenerated: only `types_gen.go` changed, from 18,029 to 18,807 lines. By declaration: 898 to 944, none removed; 46 added and 62 changed, all
`Validate`; the imports gained `maps`, `slices`, and `unicode/utf8`. `Validate` methods: 99 before, 145 after, and each of the 99 is still there.
No `... is required` check remains; the schema's one `minLength` gives two checks (`Notice`, `SessionUpdateNotice`). Types whose methods use two
receiver names: 21 before, 0 after. No golden changed.

Each part was broken in a fresh scratch copy:

```text
union does not validate its variant: cmd/generate ['TestValidateMethods']; root ['TestValidateSchemaConstraints']
slices not descended: cmd/generate ['TestValidateMethods']; root ['TestValidateSchemaConstraints']
maps not descended: cmd/generate ['TestValidateMethods']; root ['TestValidateSchemaConstraints']
pointers not descended: cmd/generate ['TestValidateMethods']; root ['TestValidateSchemaConstraints']
minLength not checked: cmd/generate ['TestValidateMethods', 'TestValidations']; root ['TestValidateSchemaConstraints']
no fixed point: cmd/generate ['TestValidations']
bound at the type's edge checked: the generator refuses (ProtocolVersion: a bound on a named primitive type is not checked); cmd/generate fails 7 tests
```

"No fixed point" passes the root: one pass in sorted order happens to reach every committed case; the model test's two-level chain catches it.

The first gate run found `gocognit` on `buildValidations` (66, above 50), `goconst` on the Go type names, `revive` on an uncommented constant, an
unnecessary conversion, and a trailing blank line; `buildValidations` is split into `validationShapes`, `ownChecks`, and `spreadValidates`, the type
names are constants, and the plants above were rerun on the final code with the same results.

After the change every test in the root (`go test -race`) and in `cmd/generate` passes; `go build ./example/...` passes. Coverage: `internal/model`
93.9%, `internal/emit` 81.2%.

Gates: `make pre-add-check` with the thirteen changed Go files named: `go-precheck: 13 file(s) clean (gofmt, golangci-lint, go vet, go test,
govulncheck).` Without names: `go-precheck: 71 file(s) clean (...)`. `make check-records` and `make markdownlint` exit 0.

Commit point: the agent stopped here. The owner commits step 9 on `main`.
