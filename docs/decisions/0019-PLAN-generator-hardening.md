---
status: in-progress
date: 2026-10-14
associated-madr: "0019-MADR-remediate-debugging-pass-findings.md"
---

# Implement phase 3 of 0019: generator hardening

Associated MADR: [0019-MADR-remediate-debugging-pass-findings.md](0019-MADR-remediate-debugging-pass-findings.md)

## Goal

Implement phase 3 of the MADR's outcome: G2 and G3 with decision 5, G4 to G19, and G20 and G21 as "Amended while planning phase 3,
2026-10-14" adds them. Each latent shape is refused with an error that names the definition, or handled, with an engine case each;
each case is seen to fail on a planted break first. The committed generated files change only for G2 and G3, in `UnstableMcpError`.
Phase 3 rides the next release (decision 12), `v1.15.0`, since G2 adds exported API.

Phases 1 and 2 are [0019-PLAN-behaviour-fixes.md](0019-PLAN-behaviour-fixes.md) and
[0019-PLAN-api-decisions.md](0019-PLAN-api-decisions.md), complete and released as `v1.14.0`.

## Standards this plan follows

| Id | Source | Text | Bears on |
| :--- | :--- | :--- | :--- |
| Q1 | `schema/schema.unstable.json:8991` | `McpError.data`: "Optional error data; explicit null is retained." | G3 |
| Q2 | `go doc encoding/json.Marshal`, `go1.27.2` | "The key name will be used if it's a non-empty string consisting of only Unicode letters, digits, and ASCII punctuation except quotation marks, backslash, and comma." | G9 |
| Q3 | [0011-MADR-harden-code-generator-engine.md](0011-MADR-harden-code-generator-engine.md), D6 | "every error site listed above run by a test or named in the plan with its reason", measured "without the nine committed-schema tests" | G19 |
| Q4 | the MADR, decision 5 | "keep extra keys where `additionalProperties` is `true`, and exempt from null-is-absent a field whose schema text says explicit null is retained, amending 0012" | G2, G3 |

## Facts this plan rests on

Each finding was reproduced on `c97e6d3` in a scratch copy, by a probe engine case or a root probe test, under the default build and
for G2, G3 and G9 also under `GOEXPERIMENT=nojsonv2`. Each fix below was prototyped in a scratch copy: the generator suite passes, the
committed generated files are byte-identical except where a step says otherwise, and each proposed case failed on its planted break.
Step 1 re-asserts each code fact, snippet at line, before any code changes.

| Finding | The code as it is | Reproduction on `c97e6d3` |
| :--- | :--- | :--- |
| G2 | `classify` makes an object with properties `KindObject` without reading `additionalProperties` (`cmd/generate/internal/model/model.go:379`); `objectFields` and `plainFields` ignore it (`model/fields.go:383`, `:433`); the struct is emitted with `keepExtra` false (`emit/types.go:194`). A catch-all variant already keeps an `Extra` map (`model/model.go:1041`, `emit/types.go:214-217`, `:317-327`, `:375-379`, `:418-427`). | `{"code":1,"message":"m","vendor":"x"}` decoded and re-encoded as `UnstableMcpError` gives `{"code":1,"message":"m"}`. |
| G3 | `UnstableMcpError.UnmarshalJSON` runs `a.Data = absentIfNull(a.Data)` (`types_gen.go:16729`), from `NullIsAbsent` (`model/fields.go:133-137`, `:424`, `:443`). Encoding already sends `json.RawMessage("null")` as `null`. `raw_value_test.go:130` pins the current decode. | `{"code":1,"message":"m","data":null}` decodes with `Data` empty. |
| G4 | An `enum` of any kind becomes a string type (`model/model.go:369-370`, `:517-520`; `model/fields.go:263-264`). | `{"type":"integer","enum":[1,2]}` gives `type Level string`; `{"level":1}` fails to decode, `{"level":"1"}` decodes; `["a",null]` gives the constant `"<nil>"`. |
| G5 | `optionalTypeRef` takes the last non-null type and `primaryType` the first (`model/model.go:949-985`, `model/fields.go:297-306`, `:266`). | A required `["string","integer"]` is `*int64`; `["string","integer","null"]` is `string`. |
| G6 | `classify` requires `type == "object"` (`model/model.go:379`); `gates.go:287` and `validate.go:134` re-read `type`. | A definition with properties and no `type` is `json.RawMessage` and accepts `42`; with flattened members it is a struct without `Validate`. |
| G7 | A named array definition is `KindAny` (`model/model.go:385-386`, `emit/types.go:181-186`). | `type Tags = json.RawMessage`; `{"tags":{"not":"an array"}}` decodes. |
| G8 | A variant with no `$ref`, object shape, null type or title is skipped (`model/model.go:816-819`). | `anyOf [string, array, boolean, {title Num integer}]` emits `Num` only; `"s"` gives "no variant matches". |
| G9 | The tag is the key as it is (`model/fields.go:392`, `:404`, `:441`, `:447`; `emit/ast.go:202`). | Key `"a,omitempty"` is emitted as `json:"a,omitempty"`; key `a"b` encodes as `"a"` by default and `"AB"` under `nojsonv2`. |
| G10 | A `default` is re-marshalled and never checked (`model/fields.go:394-401`); 24 generated `json.Unmarshal` statements drop their error (`emit/types.go:308`, `:415`), and `orDefault`, emitted by `emitOrDefault` (`emit/decode.go:241`), returns the zero value. | A `uint8` with `default: 300` and an `int32` with `default: "x"` are accepted; `{}` decodes to zero. |
| G11 | `requiredSet` is not checked against the properties (`model/fields.go:384`, `:461-462`; `model/model.go:723-725`). | `required: ["a","missing"]` is accepted; `{"a":"x"}` decodes. |
| G12 | `bounds` and `boundLit` truncate (`model/validate.go:243`; `emit/types.go:1044-1048`, `int64(c.Bound)` at `:1046`). | `minimum: 1.5` on an integer is emitted `< 1`, so `1` passes. |
| G13 | `Definition.UnmarshalJSON` decodes through `decodeStrict`, whose `DisallowUnknownFields` matches case-insensitively (`load/load.go:86-111`; also `:114`, `:124-128`). | `"Properties"`, `"Type"` and `"REQUIRED"` are honoured. |
| G14 | A discriminator const is `fmt.Sprint`ed (`model/model.go:760-764`); method names are not reserved (`model/names.go:60-83`). | An integer const fails as "not an exported Go identifier"; a boolean one, and a property `validate`, fail in the type checker without naming the property. |
| G15 | `handWritten` lists `bp.GoFiles` only (`output/check.go:113-139`). | A root test file declaring `recordingAgent` passes the generator; `go vet` then fails "recordingAgent redeclared". |
| G16 | `handWritten` returns nothing for a missing directory (`output/check.go:114`), so `Check` reports type errors, and the `os.MkdirAll` after it (`main.go:65`) is never reached. | `-out <missing>`: "does not type-check … undefined: Connection … and 458 more". |
| G17 | `schemaTypeObject` to `schemaTypeBoolean` unused (`emit/constants.go:10-15`); `Facts.Format`, `Values` and `AbsolutePath` read only by tests, and the doc "The emitters do not use them yet" stale (`model/fields.go:101-116`); "jen will render them" (`util/util.go:32`); `resp == nil` impossible (`model/methods.go:186`). | Read and `grep`. |
| G18 | Linked definitions no meta key consumes are dropped (`model/methods.go:132-148`). | A definition with `x-side: agent`, `x-method: orphan/thing` and no meta entry becomes a plain type. |
| G19 | 41 untested error sites in the engine and `main.go`, listed in step 6 (measured with all tests). | Coverage profile. |
| G20 | A variant with no `$ref` and no properties keeps its `type` and drops its `const` (`model/model.go`, `buildVariant`, before `:816`). | See the MADR's amendment. |
| G21 | `boundLit` converts through `int64` (`emit/types.go:1046`). | `uint64` `maximum: 1e19` is emitted `> 9223372036854775807`; `uint8` `minimum: 300` fails in the type checker. |

The committed schemas (`schema/schema.json`, 178 definitions; `schema/schema.unstable.json`, 277) contain none of the latent shapes:
no `enum` keyword under `$defs`; every type list is one type and `"null"`; no node with properties lacks `"type":"object"`; no array
definition; no untitled primitive variant; no property key outside Q2; 43 defaults, all valid; no undeclared required key; no
fractional integer bound; no mis-cased keyword; no non-string discriminator const; no property or variant named for a generated method;
no unlisted linked definition; no non-string const outside `ErrorCode`'s integer enum. Objects with both `properties` and
`additionalProperties` (all `true`): 3 stable and 6 unstable, all catch-all variants that already keep `Extra`, except `McpError`.

## Scope

In scope: the generator (`cmd/generate/internal/load`, `model`, `emit`, `output`, `util`, `main.go`, their tests, new engine cases);
the regenerated `types_gen.go` (G2, G3); root tests (`raw_value_test.go`, a G2 and G3 test); `README.md`, `docs/architecture.md`;
[0012-MADR-schema-fidelity-framework.md](0012-MADR-schema-fidelity-framework.md)'s amendment (decision 5); release notes for
`v1.15.0` as `docs/reports/0019-REPORT-release-notes-v1-15-0.md`; these records and their `docs/README.md` rows.

Out of scope: phases 4 and 5; handling a refused shape (each can be handled later by its own record); the open string and integer
enums (by design, 0011 and 0012); `encoding/json` v2; the mutation tool's and `enginetest`'s own error sites (tooling, not the engine
D6 measures; they are named in step 6).

## Design

Every refusal names the definition and the property or variant, in the form the existing refusals use (`T.properties.key: …`, `U
variant N: …`), and is pinned by a `refuse-*.txtar` engine case whose `want.err` holds its lines. Every handled shape is pinned by an
`accept-*.txtar` case with `want.api` and a `behaviour_test.go`. A planted break is run in a scratch copy for each case before it is
relied on.

### G2. Extra keys where `additionalProperties` is `true` (decision 5)

* Model: `Type.KeepExtra`, set by `buildObject` when the object has properties and `additionalProperties` is present and not the
  boolean schema `false`; the loader gains an accessor for its boolean schema. The emitter passes `t.KeepExtra` where it passes `false`
  today, reusing the catch-all path: an `Extra map[string]json.RawMessage` field tagged `json:"-"`, filled on decode with every key
  no declared property reads (after folding, so `"Code"` decodes into `Code`), and written back on encode, a declared field winning.
* Refused, latent: a property whose field would be `Extra`; `additionalProperties` with flattened members or on a union with its own
  keys; a typed `additionalProperties` schema on an object with properties; `additionalProperties` on a nested inline object or a
  non-catch-all inline variant.
* Generated change: `UnstableMcpError` gains `Extra` and a `MarshalJSON`; nothing else changes. API listing: the field `Extra`
  and `func (UnstableMcpError) MarshalJSON` are added.
* Cases: `accept-additional-properties` (`Open` `true`, `Closed` `false`, `Plain` absent; extra keys round-trip byte-exact, a folded
  key does not land in `Extra`, a declared field wins, `Closed` and `Plain` drop extras); `refuse-additional-properties-extra-name`;
  `refuse-additional-properties-members`. Planted break: `emitStruct` given `false` (prototype: `want.api` differs, the behaviour test
  does not compile, "o.Extra undefined").

### G3. Explicit `null` kept where the schema says so (decision 5)

* Protocol table: `NullRetained []PatchRule`, beside `PatchFields`, with one rule, `McpError` `data`, its `Source` Q1. A rule whose
  definition or property the schema lacks is skipped; one on a required or non-raw property is refused; a rule whose `Source` is not
  in the property's description is refused, so an upstream rewording fails loudly.
* Model: a property a rule names is not `NullIsAbsent`, so the field stays `json.RawMessage`: `nil` absent, the bytes `null` an
  explicit null, anything else a value. No exported type changes.
* Guard: `TestProtocolTableAppliesToCommittedSchema` requires every property of the committed schemas whose description says
  explicit null is retained to be named in `NullRetained`.
* Generated change: the `absentIfNull` line leaves `UnstableMcpError.UnmarshalJSON`.
* Root: `raw_value_test.go`'s `UnstableMcpError.data` row moves to a test that the field keeps absent, `null` and a value both ways,
  run under both builds. With G2's: `TestUnstableMcpErrorKeepsExtraAndNull`.
* Cases: `accept-raw-null-retained` (`McpError.data` keeps the three states, another raw field still reads `null` as absent);
  `refuse-null-retained-required`; `refuse-null-retained-source`. Planted break: the exemption removed (prototype: the behaviour test
  fails, `data decoded as ""`).
* Records: [0012-MADR-schema-fidelity-framework.md](0012-MADR-schema-fidelity-framework.md) gains "Amendment 2026-10-14, explicit
  null retained where the schema says so", after "a null raw value is an absent one".

### G4, G5, G7, G8, G20. Shapes without a Go type, refused

* G4, in `checkTypable` (`model/schema.go:233`): an `enum` value that is not a string. Error: `<path>: enum value <v> is not a
  string; only a string enum has a Go type`. `TestEnumValueKinds` (`model/schema_test.go:123`), whose `Nums` definition and row
  (`:128`, `:139`) assert the old behaviour, changes to expect the refusal. Cases `refuse-enum-integer`, `refuse-enum-null`, `refuse-enum-on-property`.
* G5: a type list with two non-null types. Error: `<path>: type list of <a> and <b> has no Go type`. Cases
  `refuse-type-list-two-types` (property), `refuse-type-list-nullable` (definition).
* G7: a named array definition. Error: `<Def>: a named array definition has no Go type; declare the array where it is used`. Case
  `refuse-named-array`.
* G8: a variant with no title, `$ref`, properties or null type, which is skipped today. Error: `<U> variant <n>: a variant without a
  title, $ref, properties, or null type has no Go name`. The fixtures that use such variants as filler gain titles
  (`model/unions_test.go:17`, `:89`, `:147`; `model/keywords_test.go:238`). Cases `refuse-untitled-primitive-variant`,
  `refuse-untitled-array-variant`.
* G20, in `buildVariant`: a variant with no `$ref` and no properties whose `const` is not a string, or is a string in a union that
  also holds a non-string const. Error: `<U> variant <n>: const <json> on a union variant is not enforced; only a oneOf of string
  consts or of titled integer consts has a Go type`. Cases `refuse-const-variant-number`, `-boolean`, `-mixed`, `-untyped`.
* Planted breaks (run in the prototype): each check removed; G4 checking definitions only, and skipping `null`; G5 testing `len > 2`;
  G20 checking non-string consts only, typed variants only, number consts only. Each fails at least one case.

### G6. An object without `"type":"object"`, handled

`classify` treats a node with properties and no `type` as an object, and `gates.go:287` and `validate.go:134` test the kind, not
`type`, as variants already do (`variantIsObject`, `model/model.go:1100`). Refusing it instead broke `accept-inline-variants`,
`accept-declared-discriminator` and three model tests in the prototype. Consequence, stated in the architecture document: such a
definition refuses a non-object value (`42`), which plain JSON Schema allows. Cases `accept-untyped-object` (gains `UnmarshalJSON`
and `Validate`) and `accept-untyped-object-gates` (`accept-capability-gates` with the gated type's `type` removed; same API; its
behaviour test). Planted breaks: each of the three sites reverted; `gates.go:287` alone passes the API check and fails the behaviour
test, so that case keeps its `behaviour_test.go`.

### G9 to G14 and G21. Keywords and values

* G9, in `checkFields`: a property key outside Q2's set. Error: `T.properties.<key>: a json struct tag cannot carry this key`.
  Cases `refuse-property-key-comma`, `refuse-property-key-quote`.
* G10, at generation time: each `default` checked against its field: JSON kind, integer value and range for the `format`, `enum` and
  `const`, through `$ref` and `properties`. Error: `T.<key>: default <v> is not a <type>`. The 24 generated statements stay as they
  are, since a checked default cannot fail; output is byte-identical. Case `refuse-default-type` (`uint8` 300, `int32` `"x"`).
* G11: a `required` key not in the effective property set (after `allOf`, parent and variant properties). Error: `T: required key
  "<k>" is not a property`. Case `refuse-required-undeclared`.
* G12 and G21: an integer `minimum` rounds up and `maximum` down; a `uint64` bound is formatted without `int64`; a bound outside the Go
  type's range after rounding is refused, `T.<key>: minimum <v> is out of range for <type>`. Cases
  `accept-validate-fractional-bound` (behaviour: `minimum 1.5` refuses 1 and accepts 2, `maximum -1.5` refuses -1 and accepts -2,
  `uint64` `maximum 1e19` accepts `1e19`), `refuse-unreachable-bound` (`uint8` `minimum 300`). Planted break: the rounding reverted.
* G13: `Definition.UnmarshalJSON`, and the meta, discriminator and top-level decoders, first read the object's keys exactly
  (`map[string]json.RawMessage`) and refuse a key not exactly one of the struct's tags, derived by reflection. Works under both
  builds. Error: `definition "T": unknown keyword "REQUIRED"`. Case `refuse-keyword-case`; `keywords_test.go:70` stays.
* G14: a non-string discriminator const, and a field, wrapper field or member field named `Validate`, `MarshalJSON` or
  `UnmarshalJSON`. Errors: `<U> variant <n>: discriminator <v> is not a string`; `T.properties.<key>: collides with the generated
  method <M>`. `Roots`, hand-written on five types, is left to the type checker and named in `docs/architecture.md`. Cases
  `refuse-discriminator-const`, `refuse-method-field`, `refuse-method-variant`.

### G15 to G18. Output, `main` and model hygiene

* G15: `output.Check` type-checks twice: the hand-written `GoFiles` with the generated non-test files, then those with the
  hand-written `TestGoFiles` and every generated file. Errors in generated files are reported first. `TestCheckSelectsLikeTheGoCommand`
  (`output/check_test.go:84`), which pins the old selection, is amended. Test: a `check_test` case whose hand-written test file
  redeclares a name of `schema_gen_test.go`, wanting `redeclared`. Planted break: `GoFiles` only.
* G16: a missing output directory is refused, `output directory <dir>: does not exist`; `os.MkdirAll` and its error site go, and
  `main.go`'s usage drops "creates". The "a missing directory" case of `check_test.go:48` wants the new text; a `main_test` runs with a
  missing `-out` and asserts nothing was created. Planted break: `return nil, nil` restored.
* G17: the six constants deleted; `Facts.Format`, `Values` and `AbsolutePath` deleted with the three tests' reads, and the `Facts`
  doc rewritten; the `util` comment corrected; the impossible `resp == nil` removed. Check: the compiler, `grep`, and the suite.
* G18: a linked definition no meta file lists is refused, `definition <D>: linked by x-method and x-side to <side> method "<m>",
  which no meta file lists`. `TestProtocolMethodConstants` (`emit/dispatch_test.go:339`) and the "prompt turn without its cancel"
  case (`model/protocol_test.go:240`), which use unlisted definitions, gain their meta entries or leave the definition out. Case `refuse-method-unlisted`. Planted break: the check removed.

### G19. Every engine error site run by a test or named (Q3)

Measured on `c97e6d3` with all tests: 41 engine sites, and 23 in `internal/mutate` and `internal/enginetest`.

| Class | Sites | Remedy |
| :--- | :--- | :--- |
| Reachable through a schema | `load/merge.go:89`, `:93`; `model/gates.go:120`, `:178`, `:217`, `:237`; `model/model.go:408`, `:411`, `:479`, `:562`, `:565`, `:627`, `:646`, `:786`, `:831`; `model/protocol.go:561`, `:587`; `model/validate.go:110`, `:168` | a `refuse-*` engine case each, probed on `c97e6d3` with the expected message |
| Reachable from a unit test | `output/check.go:40`, `:48`, `:56`, `:149`; `main.go:34`; `emit/decode.go:393` (through `foldNames`); `output/write.go:89` (through a `createTemp` seam); `main.go:104` (a Unix-only `t.Chdir` into a removed directory, or named) | a unit test each |
| Defensive, unreachable | `emit/file.go:131`, `:195`, `:198`, `:301`; `load/merge.go:118`, `:285`; `model/fields.go:390`, `:439`, `:470`; `model/model.go:569`, `:576`, `:837`; `main.go:60` | named in the execution record with its reason, or removed where `checkFields` can return the names it already checked (the six `fields.go` and `model.go` sites) |
| Removed by G16 | `main.go:66` | none |
| Tooling, outside D6 | 20 in `internal/mutate`, 3 in `internal/enginetest` | named |

Step 6 first re-measures under D6's measure, without the nine committed-schema tests, after steps 2 to 5 have added their own sites;
the list it closes is that one, recorded with each site's test or reason.

## Implementation Steps

Each step ends with its gates and an execution record; the owner commits it. Agents do not commit on `main`. Every new test is seen to
fail on a planted break in a scratch copy, never in the tree, before it is relied on.

1. **Records.** This PLAN and its `docs/README.md` row, and the MADR's amendment. Before step 2, a script asserts every code fact
   above, snippet at line, on the commit step 2 starts from. Gates: `make check-records`, `make markdownlint`, the identifier scan.
2. **G2 and G3.** Model, emitter, protocol table, the guard, regeneration, the root test, the engine cases; the 0012 amendment; the
   `README.md` and `docs/architecture.md` passages on extra keys and raw `null`. The diff of the generated files is exactly
   `UnstableMcpError`'s; the API listing against `v1.14.0` adds exactly G2's lines.
3. **Shapes: G4, G5, G6, G7, G8, G20.** Committed output byte-identical.
4. **Keywords and values: G9, G10, G11, G12 with G21, G13, G14.** Committed output byte-identical.
5. **Output, `main` and hygiene: G15, G16, G17, G18.** Committed output byte-identical.
6. **G19.** Re-measure, then close each site by a test or a named reason.
7. **Documentation and release notes.** `docs/architecture.md` lists the refused shapes and G6's handling; the release notes for
   `v1.15.0` in `docs/reports/0019-REPORT-release-notes-v1-15-0.md`, with its `docs/README.md` row: additions (G2's `Extra`,
   `MarshalJSON`), the behaviour change (G3's `null`), and the generator's refusals for schema authors.
8. **Verification and close.** The gates over the tree; the probes of every finding re-run (each refused or handled); the API
   listing against `v1.14.0`; Windows. After CI passes on the pushed commits, this PLAN and its row become `complete`, and the owner
   tags `v1.15.0` and publishes its release from the report.

## Verification

Per code step, through the Python runner outside the tree: regenerate, and the diff of the generated files is exactly the step's;
`make pre-add-check`; `scripts/go-precheck.sh` on each new `.go` file; `GOEXPERIMENT=nojsonv2 go test ./...`; `GOOS=windows go vet
./...` in both modules; the generator suite under `-race`, engine behaviour cases included; each new test seen to fail on its planted
break in a scratch copy; `make check-records`, `make markdownlint`, the identifier scan; Windows `go test -count=1 -shuffle=on ./...`
in both modules; the API listing against `v1.14.0`.

Expected API listing difference against `v1.14.0`: the field `UnstableMcpError.Extra` and `func (UnstableMcpError) MarshalJSON
() ([]byte, error)` are added; nothing else. Step 2 records the exact lines the gate's listing prints.

Acceptance: every step's checks pass and its record is written; every finding of phase 3 has an engine case, a unit test, or a
recorded check that failed on a planted break; G19's list is closed under D6's measure; the full suites pass under both builds and on
Windows; CI passes on the pushed commits.

## Rollout and Rollback

Each step is one commit, reverted by reverting it; steps 3 to 6 change no committed generated file, so reverting one only lets the
generator accept a shape again. Step 2 changes `UnstableMcpError`, an unstable type: once `v1.15.0` is tagged, `Extra` and
`MarshalJSON` stay within `v1`, and G3's `null` can be reverted only as a behaviour change named in a release's notes. A schema author
whose schema a new refusal stops sees the error naming the definition; the shape had no correct Go type before.

## Execution record

### Step 1, records, 2026-10-14

The owner approved step 1 on 2026-10-14. Written: this PLAN, `in-progress`, and its `docs/README.md` row; the MADR's "Amended while
planning phase 3, 2026-10-14", which adds G20 and G21 as the owner decided that day, and the note on outcome item 3.

Before this PLAN was written, each finding of the phase was reproduced on `c97e6d3` in scratch copies, by probe engine cases and
root probe tests, and each fix in "Design" prototyped there; the repository was not written. The owner chose, for G20, refusal over
handling, and, for G21, to join G12.

The code facts, asserted by a Python script outside the tree, snippet at line, on `c97e6d3` (this step changes no code, so step 2
starts from the same code): 78 facts and the 41 G19 sites, each an error return or report, hold. The first run of the check found
four references of the draft off and they were corrected before this record: `bounds` starts at `model/validate.go:243`; `boundLit`
at `emit/types.go:1044`, its `int64` at `:1046`; `orDefault` is emitted by `emitOrDefault`, `emit/decode.go:241`; the untitled
variant fixtures are `model/unions_test.go:17`, `:89`, `:147` and `model/keywords_test.go:238`. The check was seen to fail on a
planted break in a scratch copy: with `types_gen.go:16729` and `main.go:66` changed, it named both and exited 1.

Gates, through the Python runner: `make check-records` and `make markdownlint` exit 0; the identifier scan finds nothing. No Go file
changed in this step.

The owner committed step 1's record as `100701e`.

### Step 2, G2 and G3, 2026-10-14

What was built, on `100701e`:

* G2. `load.Definition.BoolSchema` reports a boolean schema. `model.Type.KeepExtra` is set by `keepExtra`, called at the end of
  `buildObject`, when `additionalProperties` is the boolean schema `true`; `emitObjectSchema` passes it to `emitStruct`, the catch-all's
  path. The model refuses, naming the definition: a typed `additionalProperties` on an object with properties; one beside flattened
  members; a property that would take the field `Extra`; one on an inline object property (in `buildObject`); and, in
  `checkUnionExtra`, one on a union with keys of its own or on an inline variant other than the catch-all. `admitsExtra` reads an
  `additionalProperties` that is present and not `false`.
* G3. The protocol table's `NullRetained []PatchRule`, checked by `checkRules` like `PatchFields` (a definition, a property and a
  source; no two rules for one property), with one rule in `ACP`: `UnstableMcpError` `data`, its source the schema's sentence
  "Optional error data; explicit null is retained." `resolveNullRetained`, after `resolvePatchFields`, skips a rule whose definition or
  property the schema lacks, refuses one on a required property or whose source the description does not contain, and, for every field
  built from the property, refuses a non-raw one and clears `NullIsAbsent` on a raw one.
* Regenerated: `types_gen.go` changes in `UnstableMcpError` alone: the field `Extra`, a `MarshalJSON`, and an `UnmarshalJSON` that fills
  `Extra` and no longer calls `absentIfNull` on `Data`; 35 lines added, 2 removed. No other generated file changes.
* Tests. Root: the `UnstableMcpError.data` row leaves `TestOptionalRawFieldsNullIsAbsent`; `TestUnstableMcpErrorKeepsExtraAndNull`
  (`raw_value_test.go`) requires absent, `null`, a value and extra keys to be read and sent back as received, and a declared key to win
  over an `Extra` entry of its name, under both builds. Model: `TestNullRetainedRules` (`patch_test.go`); the guard
  `checkCommittedNullRetained`, called by `TestProtocolTableAppliesToCommittedSchema`, requires every committed definition and property
  whose description says an explicit null is retained to be named by a rule, and `UnstableMcpError.data` to be the only optional raw
  field that keeps `null`. Engine cases: `accept-additional-properties` and `accept-raw-null-retained`, each with a behaviour test;
  `refuse-additional-properties-extra-name`, `-members`, `-typed`, `-nested`, `-union`, `-variant`; `refuse-null-retained-required`,
  `-source`, `-raw`.
* Documentation: `README.md`, the raw-value passage (`UnstableMcpError.data` keeps `null`) and a new passage on `Extra`;
  `docs/architecture.md`, the protocol table's `NullRetained`, the open object's `Extra` and its refusals, and the null-retained
  exception to the raw `null` rule; [0012-MADR-schema-fidelity-framework.md](0012-MADR-schema-fidelity-framework.md), "Amendment
  2026-10-14, explicit null retained where the schema says so", placed last, after the amendments in date order.

Where this differs from "Design" in detail, not in decision: the rule names the definition `UnstableMcpError`, as the merged schema
names it and as every rule of the table does, where "Design" says `McpError`; the design named two of G2's refusal cases and two of G3's,
and the step pins each refusal it lists, so four more G2 cases and `refuse-null-retained-raw` were added; the `UnstableMcpError.data`
entry leaves `patchExceptions`, which now skips a property a null-retained rule names.

Planted breaks, each in a fresh scratch copy (15 of 15 seen to fail):

| Break | What failed |
| :--- | :--- |
| `emitStruct` given `false` for `t.KeepExtra` | `TestEngineBehaviour/accept-additional-properties`: "o.Extra undefined" |
| the same, regenerated, at the root | `raw_value_test.go:192`: "v.Extra undefined" (build failed) |
| the `Extra` name check never matches | `refuse-additional-properties-extra-name`: the output does not type-check, not the expected error |
| each of the members, typed, nested, union and variant refusals removed | its case: "run accepted the schema" |
| `NullIsAbsent` not cleared | `TestEngineBehaviour/accept-raw-null-retained`: `{"code":1,"data":null}: data decoded as ""` |
| the same, regenerated, at the root | `TestUnstableMcpErrorKeepsExtraAndNull/null`: `data decoded as "", want "null"` |
| each of the required, source and raw refusals removed | its case: "run accepted the schema" |
| the `ACP` rule removed | `TestProtocolTableAppliesToCommittedSchema`: "UnstableMcpError.data: its description says an explicit null is retained, and no null-retained rule names it" |
| the two-rules check never matches | `TestNullRetainedRules`: "two rules: Build = \<nil\>" |

Gates, through the Python runner, on the tree: regenerating changes no generated file; `make pre-add-check` exit 0; no new `.go` file;
`GOEXPERIMENT=nojsonv2 go test ./...` exit 0; `GOOS=windows go vet` exit 0 in both modules; `make check-records` and `make
markdownlint` exit 0; the identifier scan finds nothing; Windows `go test -count=1 -shuffle=on ./...` exit 0 at the root and in
`cmd/generate`. The API listing: 2426 lines at `v1.14.0`, 2428 in the tree; 0 removed and 2 added, exactly as expected:

```text
+ field UnstableMcpError.Extra map[string]json.RawMessage
+ method (UnstableMcpError) MarshalJSON () ([]byte, error)
```
