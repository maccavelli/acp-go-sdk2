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

The owner committed step 2's record as `131fa07`.

### Step 3, shapes, 2026-10-14

What was built, on `131fa07`:

* G4, G5, G7. `checkGoType`, called by `checkTypable` for every typed node (`model/schema.go`), refuses an `enum` value that is not
  a string ("enum value 1 is not a string; only a string enum has a Go type"), a type list with two types other than `null` ("type list
  of integer and string has no Go type; only one type, or one type and null, has one"), and a named array definition ("a named array
  definition has no Go type; declare the array where it is used"). `schemaValue` renders the offending value as the schema writes it.
* G6. `classify` makes a definition with properties and no `type` a `KindObject`, and `gateHolders` (`model/gates.go`) and
  `validationShapes` (`model/validate.go`) select by kind, `KindObject` or `KindEmptyObject`, where they re-read `type`. Such a definition
  now refuses a value that is not an object, as a typed one does.
* G8. `buildVariant` refuses a variant with no title, `$ref`, properties or null type, which it skipped: "a variant without a title,
  $ref, properties, or null type has no Go name". A `nil` entry of a union's list is still skipped.
* G20. `buildVariant` refuses a variant with no `$ref` and no properties that has a `const`: one reaches the union path only beside a
  non-string const, since a oneOf of string consts and one of integer consts are enum kinds: "const 1.5 on a union variant is not
  enforced; only a oneOf of string consts or of titled integer consts has a Go type".
* Tests. `TestEnumValueKinds` no longer builds the number enum `Nums` as a string enum; it requires `Build` to refuse it. The untitled
  filler variants of `TestUnionDiscriminator`, `TestValidations` (two schemas) and `TestNoOmitemptyOnStructValues` gain titles. Engine
  cases: `refuse-enum-integer`, `refuse-enum-null`, `refuse-enum-on-property`; `refuse-type-list-two-types`, `refuse-type-list-nullable`;
  `refuse-named-array`; `refuse-untitled-primitive-variant`, `refuse-untitled-array-variant`; `refuse-const-variant-number`, `-boolean`,
  `-mixed`, `-untyped`; `accept-untyped-object` (with a behaviour test: decodes, requires, validates, refuses `42`, and its flattened
  form validates and encodes its member) and `accept-untyped-object-gates` (`accept-capability-gates` with `type` left out of the gated
  request, same API, its behaviour test).
* Regenerated: no generated file changes.

The first run of `make pre-add-check` failed on lint, and the code was corrected before this record: `json.Marshal`'s error discarded
twice (errcheck), replaced by `schemaValue`, which cannot fail; and `gateHolders`' switch on `t.Kind` (staticcheck QF1002), now a tagged
switch. The planted breaks were then run again on the corrected tree; the results below are that run's.

Planted breaks, each in a fresh scratch copy (14 of 14 seen to fail):

| Break | What failed |
| :--- | :--- |
| G4's check never matches | `refuse-enum-integer`, `-null`, `-on-property`: "run accepted the schema" |
| G4 on definitions only | `refuse-enum-on-property` |
| G4 skipping `null` | `refuse-enum-null` |
| G5's check removed; G5 refusing only more than two types | `refuse-type-list-two-types` and `-nullable` |
| G7's check removed | `refuse-named-array` |
| `classify` reverted | `accept-untyped-object-gates`: "undefined: AdditionalDirectories" |
| `validationShapes` re-reading `type` | `accept-untyped-object`: "(&Plain{}).Validate undefined" |
| `gateHolders` re-reading `type` | `accept-untyped-object-gates`: "NewSession = \<nil\>", the ungated send passing |
| G8's untitled variant skipped again | `refuse-untitled-primitive-variant` and `-array-variant` |
| G20's check removed | all four `refuse-const-variant-*` cases |
| G20 on non-string consts only; on number consts only | `refuse-const-variant-mixed`: the integer variant refused, not the string one |
| G20 on typed variants only | `refuse-const-variant-untyped` |

Gates, through the Python runner, on the tree: regenerating changes no generated file; `make pre-add-check` exit 0; no new `.go` file;
`GOEXPERIMENT=nojsonv2 go test ./...` exit 0; `GOOS=windows go vet` exit 0 in both modules; `make check-records` and `make
markdownlint` exit 0; the identifier scan finds nothing; Windows `go test -count=1 -shuffle=on ./...` exit 0 at the root and in
`cmd/generate`. The API listing against `v1.14.0` is unchanged from step 2's: 0 removed, the same 2 added.

The documentation of the refused shapes and of G6's handling is step 7's.

The owner committed step 3's record as `1ee1eb7`.

### Step 4, keywords and values, 2026-10-14

What was built, on `1ee1eb7`:

* G9 and G14, `model/names.go`. `fieldName` refuses a key a json struct tag cannot carry (`tagKey`, Q2's rule: "a json struct tag
  cannot carry this key") and a field named for a generated method (`checkNotMethod`: `MarshalJSON`, `UnmarshalJSON`, `Validate`; "field
  Validate collides with the generated method Validate"). `buildVariant` applies `checkNotMethod` to a wrapper field and `checkMembers`
  to a flattened member's field. `discriminatorValue`, out of `buildVariant`, refuses a discriminator const that is not a string
  ("discriminator 1 is not a string"), which `fmt.Sprint` turned into a value before. `Roots`, hand-written on five types, is left to
  the type checker, as "Design" says; step 7 documents it.
* G10, `model/fields.go`. `checkDefaults`, a model pass after `resolveNullRetained`, checks every property default through `admits`:
  `$ref` and an allOf of one followed (`resolveSchema`), then `enum` and `const`, a union admitting what any member admits, and the
  JSON kind, an integer also integral and in the range of its format's Go type ("default 300 is not a uint8"), array items and object
  properties by recursion. The 43 committed defaults pass; the 24 generated `json.Unmarshal` statements are unchanged.
* G11. `checkRequired` (`model/model.go`), for an object and an object with flattened members, refuses a required key that neither
  the object nor a member declares, and does the same for each inline object property; `inlineOf` (`model/fields.go`) for an inline
  variant against the union's properties and its own ("required key \"missing\" is not a property").
* G12 and G21. `bounds` (`model/validate.go`) rounds an integer `minimum` up and `maximum` down, refuses one the Go type cannot reach
  after rounding ("minimum 300 is out of range for uint8"), and omits one the type always meets, as before; `boundLit`
  (`emit/types.go`) writes an integer bound at or above `math.MaxInt64` through `strconv.FormatUint`.
* G13, `load/load.go`. `decodeStrict` first runs `exactKeys`, which refuses a key that is a field's JSON name in another letter case;
  a key that names no field is still refused by `DisallowUnknownFields`, so its text, `unknown field "x-new"`, is unchanged. The
  error for a mis-cased key reads `unknown field "REQUIRED": keywords are matched exactly, and this is "required" in another case`.
  `Discriminator` gains an `UnmarshalJSON` through `decodeStrict`, and `parseSchema` applies `exactKeys` to the top level, so a
  mis-cased `$defs` is refused.
* Tests. `TestKeywordsMatchExactly` (`load/load_test.go`): a definition, a nested property, a discriminator, the top level and a meta
  file each refuse a mis-cased key, and exact keywords pass. Engine cases: `refuse-property-key-comma`, `-quote`; `refuse-default-range`,
  `-type`, `-nested`, `-enum`; `refuse-required-undeclared`, `-variant`; `refuse-unreachable-bound`; `refuse-keyword-case`;
  `refuse-discriminator-const`; `refuse-method-field`, `-variant`, `-member`; `accept-validate-fractional-bound`, its `want.api` written
  by `-update` and read, with a behaviour test: `minimum 1.5` refuses 1 and accepts 2, `maximum -1.5` refuses -1 and accepts -2, and a
  `uint64` `maximum 1e19` accepts `10000000000000000000` and refuses one more.
* Regenerated: no generated file changes.

Where this differs from "Design" in detail, not in decision: the design named one or two cases per finding, and the step pins each
refusal it lists, so `refuse-default-nested`, `refuse-default-enum`, `refuse-required-undeclared-variant` and `refuse-method-member`
were added, and the design's `refuse-default-type` is split into `-range` and `-type`, since one schema stops at its first error; the
error texts read as above, where "Design" sketches them.

The first run of `make pre-add-check` failed on lint, and the code was corrected before this record: `buildVariant`'s cyclomatic
complexity 41 over the limit of 40 (gocyclo), lowered by moving the discriminator's value into `discriminatorValue`; and `exactKeys`
returning nil on a decode error (nilerr), now returning the error after checking that the input is an object. The first planted break
of the kind check, `g10-no-kind`, failed only because it left a variable unused; it was rewritten. Every planted break was then run
again on the corrected tree; the results below are that run's.

Planted breaks, each in a fresh scratch copy (19 of 19 seen to fail):

| Break | What failed |
| :--- | :--- |
| `tagKey` admitting every key | `refuse-property-key-comma` and `-quote`: "run accepted the schema" |
| `checkNotMethod` admitting every name | `refuse-method-field`, `-variant`, `-member`: the output does not type-check |
| the wrapper's check, or the member's check, ignored | `refuse-method-variant`, or `refuse-method-member`: the output does not type-check |
| a discriminator const turned into a string | `refuse-discriminator-const`: "wrapper field: \"1\" is not an exported Go identifier", not the expected error |
| `checkDefaults` ignored | the four `refuse-default-*` cases |
| the range, kind, recursion or enum check ignored | `refuse-default-range`, `-type`, `-nested`, `-enum` in turn |
| the object or the inline-variant required check ignored | `refuse-required-undeclared`, or `-variant` |
| `minimum` or `maximum` truncated, not rounded | `accept-validate-fractional-bound`: "{Hi:-2 Lo:1 U:0}: error \<nil\>", "{Hi:-1 Lo:2 U:0}: error \<nil\>" |
| the bound written through `int64` | the same case: "u: greater than maximum 9223372036854775807" |
| an unreachable minimum accepted | `refuse-unreachable-bound`: the output does not type-check |
| `exactKeys` matching no name | `refuse-keyword-case`: "run accepted the schema" |
| the discriminator decoded loosely; the top level unchecked | `TestKeywordsMatchExactly`: "discriminator: got \<nil\>"; "top level: got \<nil\>" |

Gates, through the Python runner, on the tree: regenerating changes no generated file; `make pre-add-check` exit 0; no new `.go` file;
`GOEXPERIMENT=nojsonv2 go test ./...` exit 0; `GOOS=windows go vet` exit 0 in both modules; `make check-records` and `make
markdownlint` exit 0; the identifier scan finds nothing; Windows `go test -count=1 -shuffle=on ./...` exit 0 at the root and in
`cmd/generate`. The API listing against `v1.14.0` is unchanged from step 2's.

The owner committed step 4's record as `8d2286a`.

### Step 5, output, `main` and hygiene, 2026-10-14

What was built, on `8d2286a`:

* G15, `output/check.go`. `Check` type-checks the package as built, the hand-written `GoFiles` with the generated files that are not
  tests, and then, when there are test files, the package as tested: those with the hand-written `TestGoFiles` and every generated
  file, so a name a hand-written test file and `schema_gen_test.go` both declare is refused, as `go vet` refuses it, and a built file
  that needs a name only a test file declares is refused too. An external test package (`XTestGoFiles`) is another package and stays
  out. `report` lists the errors in generated files first, then the rest, each part in position order (`compareBool`).
* G16. `handWritten` refuses a missing output directory, "output directory \<dir\>: does not exist"; `main.go` loses its
  `os.MkdirAll`, which a missing directory never reached, and `run`'s documentation says the directory must exist.
* G17. The six unused constants of `emit/constants.go` are deleted, `schemaTypeNull` and `jsonStructTag` kept; `Facts.Format`,
  `Facts.Values` and `Facts.AbsolutePath` are deleted with their writes (`factsOf`, `validationShapes`), and the `Facts` documentation
  says what the model reads from it; `util.go`'s comment no longer names Jennifer; `buildMethod`'s `resp == nil`, which `buildMethod`'s
  own refusal of a request without a response and `linkRoles`' skipping of a null definition make impossible, is removed. The tests
  that read the deleted fields check the field's type instead (`env`, `map[string]string`), or drop the read (`paths_test.go`, whose
  `CheckAbsolutePath` assertions remain).
* G18, `model/methods.go`. `buildMethods` refuses a definition linked by `x-method` and `x-side` to a method no meta file lists:
  "definition OrphanThingNotification: linked by x-method and x-side to agent method \"orphan/thing\", which no meta file lists".
* Tests. `TestCheckIncludesHandWrittenTestFiles` and `TestReportListsGeneratedFilesFirst` (`output`);
  `TestCheckSelectsLikeTheGoCommand` now has a valid in-package `hand_test.go`, which is checked, and an external test package with
  an undefined name, which is not; `TestCheckRefuses`' missing directory wants "does not exist"; `report`'s other caller passes no
  generated files; `TestRunRefusesMissingOutputDirectory` (`main_test.go`) requires the refusal and that nothing is created. The fixtures
  that used an unlisted linked definition: `TestProtocolMethodConstants` leaves the definition out when it lists no protocol method,
  and the "prompt turn without its cancel" case of `TestProtocolTableRefuses` unlinks the cancel definition it removes from the meta.
  Engine case `refuse-method-unlisted`.
* Regenerated: no generated file changes.

Planted breaks, each in a fresh scratch copy (6 of 6 seen to fail):

| Break | What failed |
| :--- | :--- |
| hand-written test files left out | `TestCheckIncludesHandWrittenTestFiles`: "Check = \<nil\>, want it to refuse recordingAgent redeclared" |
| the built package's pass ignored | the same test: "want it to refuse a built file that needs a test file's name" |
| position order only | `TestReportListsGeneratedFilesFirst`: the hand-written file's error listed first |
| a missing directory accepted | `TestRunRefusesMissingOutputDirectory`: "generated code does not type-check with …/missing" |
| `HasResult` inverted, so the remaining expression is pinned | `TestGeneratedFilesUpToDate`: "a prompt turn must be a request with a result" |
| an unlisted definition accepted | `refuse-method-unlisted`: "run accepted the schema" |

The deletions of G17 are checked by the compiler, which builds no reader of a deleted name, and by `grep`.

Gates, through the Python runner, on the tree: regenerating changes no generated file; `make pre-add-check` exit 0; no new `.go` file;
`GOEXPERIMENT=nojsonv2 go test ./...` exit 0; `GOOS=windows go vet` exit 0 in both modules; `make check-records` and `make
markdownlint` exit 0; the identifier scan finds nothing; Windows `go test -count=1 -shuffle=on ./...` exit 0 at the root and in
`cmd/generate`. The API listing against `v1.14.0` is unchanged from step 2's.

The documentation of the missing directory and of the second type-check is step 7's.

The owner committed step 5's record as `852054f`.

### Step 6, G19, every engine error site run or named, 2026-10-15

**The measure**, D6's (Q3): `go test -count=1 -coverpkg=./... -coverprofile … -skip '^(<the nine>)$' ./...` in `cmd/generate`, the nine
being the committed-schema tests of
[0011-MADR-harden-code-generator-engine.md](0011-MADR-harden-code-generator-engine.md): `TestGeneratedFilesUpToDate`,
`TestCheckAcceptsRepository`, `TestUnionClassesOfCommittedSchema`, `TestFitKeysOfCommittedSchema`, `TestDiscriminatorExemptOnDecode`,
`TestNoOmitemptyOnStructValues`, `TestProtocolTableAppliesToCommittedSchema`, `TestCommittedSchemaKeywordsRegistered` and
`TestMergeProvenanceMatchesInputs`. A block counts at its highest count across the test binaries; an error site is a line of
non-test code that returns or builds an error (`return … err`, `fmt.Errorf`, `errors.New`, `f.fail(`) in a block that never ran.

On `852054f`, after steps 2 to 5: the engine (`main.go` and `internal/`, less `internal/mutate` and `internal/enginetest`) at 96.4% (3694
of 3832 statements), 48 engine sites unrun, and 23 in the tooling. "Design"'s table had 41 engine sites, measured with every test on
`c97e6d3`; the sites steps 2 to 5 added and the nine skipped make the difference, among them eight of step 4's `admits` and the
`maximum` of G21.

**Run by a test, 35 sites:**

* 18 engine cases, `refuse-*.txtar`, one site each: `merge-client-key-two-wires` and `merge-protocol-key-two-wires` (`load/merge.go:89`,
  `:93`); `required-undeclared-members` (`model/model.go:427`), `integer-enum-same-title` (`:498`), `nested-name-taken` (`:584`),
  `nested-bad-key` (`:587`), `required-undeclared-nested` (`:672`), `constructor-name-taken` (`:766`), `constructor-param-name`
  (`:785`), `variant-name-taken` (`:923`), `variant-bad-key` (`:979`); `gated-in-patch` (`model/gates.go:120`),
  `gated-property-not-union` (`:178`), `catch-all-discriminator-nullable` (`:217`), `auth-method-id-not-named` (`:237`);
  `permission-no-session` (`model/protocol.go:577`), `permission-outcome-not-union` (`:603`); `rpc-holds-deep-union`
  (`model/validate.go:111`). Each schema came from the probes of the planning; each was run through the generator before it was
  written, and its `want.err` is the error it gave.
* `TestDefaultsChecked` (`model/fields_test.go`): `admits`' const, union, null, string, number, array, array item and object refusals,
  and `bounds`' unreachable `maximum` (`model/validate.go:295`); and the defaults it must accept.
* `TestCheckRefuses` (`output/check_test.go`), three cases: a `go.mod` that is a directory (`output/check.go:45`, `:180`), a hand-written
  file that does not parse (`:66`), a generated one that does not parse (`:74`).
* `TestWriteStagedFileFailureReplacesNothing` (`output/write_test.go`), through a seam: `writeWith` takes the function that creates the
  temporary file, as it already took the rename, so a test hands `stage` a file closed once created (`output/write.go:93`, "`:89`" on
  `852054f`).
* `TestRunRefusesUnknownFlag` (`main.go:34`); `TestFindRepoRootWithoutWorkingDirectory`, through a seam: `findRepoRoot` is
  `findRepoRootFrom(os.Getwd)`, so a test hands it a working directory that fails (`main.go:105`, "`:101`" on `852054f`). A first
  version removed the working directory under `t.Chdir`; it passed, but for another reason: a probe on this host found `os.Getwd`
  returning the removed directory's path with no error, so the test reached "could not locate repository root", not the site. It was
  replaced before this record.
* `TestFoldNamesRecordsCaseCollision` (`emit/types_test.go`): the emitter's backstop `foldNames` (`emit/decode.go:393`), called
  directly, since the model refuses such keys first.

**Found while writing `TestDefaultsChecked`:** step 4's `admits` accepted any value for a `{"type":"null"}` member, so a union of a
string and null admitted the default `5`; and it would have refused a `null` for that member, since `includesNull` reads only a type
list. `admits` gains a `null` case and accepts `null` for the `null` type, and the test pins both.

**Named, 13 engine sites**, each with its reason:

| Site | Why no test runs it |
| :--- | :--- |
| `model/fields.go:521` (`objectFields`), `model/model.go:598` | `buildObject` runs `checkFields` on the same properties first; `fieldName` cannot fail on a key it accepted. |
| `model/fields.go:570` (`plainFields`), `model/model.go:591`, `model/fields.go:607` (`inlineOf`) | the same, after `checkFields` on the nested object's properties, or on the variant's merged properties, which are the properties `inlineOf` receives. The planted breaks show these are second checks, not dead code: with `model.go:587` or `:979` broken alone, the engine case still saw the same error from here. |
| `model/validate.go:169` | `isPathShape` admits a string without enum or const, or an array whose items are strings; such a field is `string`, `*string` or `[]string` (a nullable item is a string, probed), and the path rules name no patch property. Before G5 a type list of two types reached it; G5 refuses that list. |
| `emit/file.go:131`, `:195`, `:301` | `layout` fails only through `decl`, which records a `printedSize` error of a function body; a body is an `*ast.BlockStmt`, which `go/printer` prints or panics on, never returning an error (0011, step 4). `:131` passes `layout`'s error on. |
| `emit/file.go:198` | `apply` fails only on a negative reservation, which only a test builds. |
| `load/merge.go:118`, `:285` | a reachable definition that is null is refused first, by the visit of every reachable name before the fixpoint and the copy (0011, step 4). |
| `main.go:60` | `emit.Render` fails only when an emitter refuses what the model accepted, which the model's refusals prevent; `foldNames`' backstop above is such a refusal. |

**Tooling, outside D6's engine, 23 sites, named:** `internal/mutate` (20: `main.go:81` to `:261`, the mutation tool's flags, `go list`
and file errors; `sites.go:69` to `:96`) and `internal/enginetest` (3: `api.go:36`, `:75`, `:83`, the test helper's read errors). They
are the test tooling's, not the engine's.

**After**, the same measure: the engine at 97.4% (3736 of 3834), 13 engine sites unrun, each named above; the 23 of the tooling.

Planted breaks, each in a fresh scratch copy: each covered site's line, checked against its text, replaced by a statement that does
nothing, and every test that targets it required to fail (16 of 16 groups):

| Group | What failed |
| :--- | :--- |
| `merge.go:89`, `:93` | both `refuse-merge-*` cases |
| the seven single `model.go` sites | their seven cases |
| `model.go:587` with its second check `:591`; `:979` with `fields.go:607` | `refuse-nested-bad-key`: "run accepted the schema"; `refuse-variant-bad-key`: the output does not type-check |
| each `gates.go` site, alone | `:120`: "run accepted the schema"; `:178` and `:237`: a nil dereference in the generator; `:217`: a print error, not the expected one |
| each `protocol.go` site, alone | `:577`: a nil dereference; `:603`: "RequestPermissionOutcome has no constructor for a variant \"cancelled\"", not the expected error |
| `validate.go:111` | `refuse-rpc-holds-deep-union` |
| the nine `admits` sites and `validate.go:295` | `TestDefaultsChecked` |
| `check.go:45`, `:66`, `:74` | the three `TestCheckRefuses` cases |
| `write.go:93` | `TestWriteStagedFileFailureReplacesNothing` |
| `main.go:34`, `:105` | `TestRunRefusesUnknownFlag`, `TestFindRepoRootWithoutWorkingDirectory` |
| `decode.go:393` | `TestFoldNamesRecordsCaseCollision`: "recorded \<nil\>" |

The first run of these breaks grouped every site of a file in one copy. Four groups did not show each test failing: the two bad-key
cases, because of the second checks above; `gates.go` and `protocol.go`, because one site's nil dereference ended the test binary
before the others ran; and `decode.go`, because the replacement left `fmt` unused and the package did not build. They were split or
corrected as the table shows, and every group was run again.

Gates, through the Python runner, on the tree: regenerating changes no generated file; `make pre-add-check` exit 0; no new `.go` file;
`GOEXPERIMENT=nojsonv2 go test ./...` exit 0; `GOOS=windows go vet` exit 0 in both modules; `make check-records` and `make
markdownlint` exit 0; the identifier scan finds nothing; Windows `go test -count=1 -shuffle=on ./...` exit 0 at the root and in
`cmd/generate`. The API listing against `v1.14.0` is unchanged from step 2's. Its first run in this step failed before listing
anything: the scratch directory outside the tree that holds the listing tool had been emptied by the host's cleanup of temporary
files at midnight; the tool was restored from this session's record of it, and the listing gave the counts of steps 2 to 5 again.
