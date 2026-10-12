---
status: proposed
date: 2026-10-12
associated-madr: "0014-MADR-union-constructors-take-every-required-key.md"
---

# Implement the union constructor decisions

Associated MADR: [0014-MADR-union-constructors-take-every-required-key.md](0014-MADR-union-constructors-take-every-required-key.md)

## Goal

Generated union constructors take every required key of their variant's Go type (P1), exist for every variant of every union (A3, V3),
build a catch-all with its discriminator (C1, X1), and name every parameter (N1, K2 with L1). The engine refuses a union key a variant's
type cannot hold (R1). A catch-all holding a value its `not` excludes fails `Validate` (C1), and a nil array variant is sent as `[]`
(item 7). Each change is proven by a test that has been seen failing on a planted break.

Facts this plan rests on, all from the MADR and read on `bc0cb78`:

* 11 constructors today, for four `oneOf` unions; a prototype of P1, A1 and the catch-all parameter generated 57 and type-checked; the
  ten non-object variants of `RequestId`, `ElicitationContentValue` and `SessionConfigSelectOptions` bring the expected total to 67.
* In the SDK, P1 changes only `NewSessionConfigOptionSelect` and `NewSessionConfigOptionBoolean`, which gain `id SessionConfigId, name
  string` before their other parameters. Their only caller in the repository is `schema_gen_test.go`.
* Six unions have a catch-all: `CreateElicitationRequest`, `CreateElicitationResponse`, `ElicitationPropertySchema`, `MultiSelectItems`,
  `IdleStateUpdate` and `StateUpdate`; each one's `not` lists exactly the other variants' discriminator values.
* No committed union has the shape R1 refuses, and no committed key is a non-identifier or begins with a capital; the required keys that
  are Go keywords are `type`, `const` and `range`.

## Scope

In scope:

* `cmd/generate/internal/model`: `model.go` (`buildUnion`, `Helper`, `Param`, `Variant`), `names.go` (`checkParam` and the new name
  function), `validate.go` if the catch-all check needs a model field there, and their tests (`model_test.go`, a new `names_test.go`).
* `cmd/generate/internal/emit`: `helpers.go` (constructor shapes, doc comments), `types.go` (`unionValidateBody`, `unionMarshalBody`),
  `registry.go` only if a constructor shape needs it, and `registry_test.go`.
* `cmd/generate`: `main_test.go` (`TestRunRefusesBeforeWriting`'s keyword case), a new `helpers_test.go` (`TestHelperParams`), and engine
  cases in `testdata/engine/`, new and regenerated.
* Generated files at the root: `helpers_gen.go`, `types_gen.go`, `schema_gen_test.go`, regenerated only.
* `conformance_test.go`: `TestConformanceConstructors`.
* Records and documentation: an amendment to
  [0012-MADR-schema-fidelity-framework.md](0012-MADR-schema-fidelity-framework.md); `README.md` ("Helper constructors", "Wire encoding");
  `docs/architecture.md`; a report `docs/reports/0014-REPORT-release-notes-v0-16-0.md`; `docs/README.md`.

Out of scope:

* Acronyms inside generated field names (`Url` against `URL`), which the MADR leaves to a separate decision.
* Changing what the decoder picks for a payload several variants fit (MADR item 8 records it; the test follows it).
* Hand-written helpers in `helpers.go`, and the three unions `HandWrittenHelpers` lists.
* Tagging and publishing the release, which the owner does.

## Implementation Steps

1. **Rules.** As in [0013-PLAN-close-phase-6-follow-ups.md](0013-PLAN-close-phase-6-follow-ups.md), step 1:
   * each new test or check is seen failing on a planted break in a scratch copy (`git archive HEAD` into a temporary directory), never in the
     tree; the record of each step quotes the failure;
   * a behaviour no record decides stops the step for the owner, with evidence and resolutions that leave the tree correct;
   * after each step: regenerate (`cd cmd/generate && env -u GOPATH -u GOMODCACHE go run .`), run `make pre-add-check` and, for each
     untracked Go file, `GO_PRECHECK_SKIP_VULN=1 scripts/go-precheck.sh <file>`; `make check-records`; `make markdownlint`; and the
     identifier scan of the changed files (the local account name, the hostname domain, real home and temporary directory prefixes);
     record the outputs; stop for the owner's commit on `main`;
   * a step's diff of generated files is asserted, not eyeballed: a script lists every changed line of `helpers_gen.go`, `types_gen.go`
     and `schema_gen_test.go` and of each engine case's `want.api`, and fails on any line outside what the step names.

2. **Records.** Amend [0012-MADR-schema-fidelity-framework.md](0012-MADR-schema-fidelity-framework.md) with a dated section, "Amendment
   2026-10-12, from 0014", that records the two decisions changing what it asserts:
   * outcome 7's list gains one check: a union's `Validate` refuses a catch-all whose discriminator is a value its `not` excludes, a rule
     the schema states (0014-MADR, C1);
   * a set array variant holding `nil` is sent as `[]`, as required array fields already are (0014-MADR, item 7).

   No source changes. `make check-records` and `make markdownlint` pass.

3. **Parameters and the refusal (P1, R1).**
   * `model.go`: a function `paramKeys(schema, t, v, vi)` returns the required keys of the Go type that holds variant `v`, in schema order,
     each once, and the properties that type them: an inline object variant (`vi.HasSharedFields`) has the union's `t.def.Required` then
     the variant's, the variant's property winning, as in `inlineOf`; a direct `$ref` or a single-`allOf` `$ref` variant has its target
     definition's, through `expandAllOf`. `buildUnion`'s constructor loop uses it in place of `expandAllOf(schema, v)`. The `Helper.Params`
     comment says what the parameters are; so does the comment at `internal/emit/helpers.go:20`.
   * `model.go`: after `classifyUnion`, `buildUnion` refuses a union with properties of its own (`len(t.def.Properties) > 0`) when any
     variant is a `$ref` or a single-`allOf` `$ref` (`v.Ref != "" || singleRefVariant(v) != ""`). The error reads `<Union> variant <n>:
     <Variant> cannot hold the union's own properties <keys>`, keys sorted. A definition that `classify` turned into flattened members is
     not a union and is not reached.
   * `cmd/generate/helpers_test.go`, `TestHelperParams`: builds the model of the committed schema and of every engine case with `want.api`,
     and for each constructor requires that the set of its parameters' fields, plus the discriminator field when the constructor writes
     the variant's constant, equals the set of required fields of the variant's Go type: `vi.Inline.Fields` for an inline variant, the
     target type's `Fields` for a `$ref` variant. The failure names the union, the variant, and the missing and extra fields.
   * Engine cases:
     * new `refuse-union-keys-ref-variant.txtar`: the MADR's defect-3 probe (`Event` requiring `id`, a `$ref` variant `Start`, an inline
       variant `stop`); `want.err` holds `Event variant 1`, `Start`, `id`;
     * new `accept-union-own-keys.txtar`: a `oneOf` union requiring `id` and `name` of its own, an inline variant requiring `value`, and a
       variant with only the discriminator; `want.api` has `func NewOptA(id string, name string, value int64) Opt` and `func NewOptB(id
       string, name string) Opt`; its `behaviour_test.go` requires `NewOptB("i", "n")` to encode `{"id":"i","name":"n","type":"b"}` and
       decode back to `B`;
     * `accept-discriminated-shapes.txtar` regenerated with `-update`: `func NewEventStart(at int64) Event`.
   * Regenerate. Asserted diff: `helpers_gen.go` changes the two `SessionConfigOption` constructors only (signature, `Id: id`, `Name:
     name`); `schema_gen_test.go` the two matching `New` calls; `types_gen.go` nothing; engine goldens: `accept-discriminated-shapes` only.
   * Plants: `paramKeys` returning `expandAllOf(schema, v)`'s keys, which must fail `TestHelperParams` naming `SessionConfigOption` and the
     fields `Id` and `Name`, and `accept-union-own-keys`; the refusal removed, which must fail `refuse-union-keys-ref-variant`.

4. **Names (N1, K2 with L1).**
   * `names.go`: `paramName(owner, key string) (string, error)` replaces `checkParam` at its one caller:
     * a key that is an identifier keeps its text (`planId`, `id`, `currentValue`: no generated name changes);
     * otherwise, L1: split the key at runs of characters that are not ASCII letters or digits, turn each word into Go case as
       `util.ToExportedField` does, and lower the first word's leading run of capitals, stopping before a capital that a lower-case letter
       follows (`URL-path` → `urlPath`, `x-API-key` → `xAPIKey`, `HTTPServer-url` → `httpServerUrl`, `XMLHttp-request` →
       `xmlHttpRequest`, `HTTP2-server` → `http2Server`, `ACME-id` → `acmeId`, `ID` stays an identifier);
     * then N1: a result for which `token.IsKeyword` holds or `types.Universe.Lookup` is not nil gets `Value` appended (`typeValue`,
       `rangeValue`, `constValue`, `errorValue`, `stringValue`, `anyValue`, `minValue`; `$type` → `type` → `typeValue`);
     * a result that is empty or not an identifier (no letter or digit, or a leading digit) is refused with the current message.
   * `model.go`: `Param` gains `Name`; `buildUnion` refuses two parameters of one constructor with the same name: `<Union> variant <n>:
     parameters <key1> and <key2> are both named <name>`. `internal/emit/helpers.go` uses `p.Name` for the parameter and the value.
   * Tests:
     * new `names_test.go`, `TestParamName`: the table above, each keyword the schema files require (`type`, `const`, `range`), the
       predeclared `error`, `string`, `any`, `min`, and the two refusals;
     * `model_test.go`: the `keyword as a constructor parameter` case of `TestBuildRefuses` becomes a collision case (a variant requiring
       `kind` as discriminator, `type`, and `typeValue`), and `TestBuildNames` compares `Param` with its `Name`;
     * `main_test.go`: the same replacement in `TestRunRefusesBeforeWriting`, still requiring the directory unchanged;
     * new engine case `accept-constructor-names.txtar`: a `oneOf` variant requiring `range`, `error`, `x-API-key` and `URL-path`;
       `want.api` has `func NewUA(rangeValue string, errorValue string, xAPIKey string, urlPath string) U`, in the variant's required
       order.
   * Regenerate. Asserted diff: no generated file changes.
   * Plants: lowering only the first rune, which must fail `TestParamName` on `uRLPath` and `xAPIKey` and `accept-constructor-names`;
     dropping the `types.Universe` lookup, which must fail on `errorValue`; dropping the collision check, which must fail the two collision
     cases.

5. **Catch-alls, every union and every variant, nil arrays, doc comments (C1, A3, V3, X1, items 6 to 8).**
   * Model:
     * `buildUnion` builds constructors for every union, `oneOf` or `anyOf`, unless `t.def.DocsIgnore` or it is in `HandWrittenHelpers`;
     * a catch-all's constructor (`idx == t.CatchAll`) has its discriminator among its parameters, under the step 4 name rule;
     * a non-object variant gets a constructor of one of two further shapes, recorded on the `Helper`: a value constructor with one
       parameter named `value`, typed by `Inline.Type` for an inline named variant, by the target's `Underlying` for a `$ref` to a
       `KindPrimitive` definition, and by the target type itself for a `$ref` to any other non-object definition; or an empty constructor,
       with no parameter, for a `null` variant and a variant with no type;
     * a catch-all `Variant` gains `Excluded []string`, the string `const` values of the discriminator in its `not` and in each member of
       the `not`'s `anyOf` or `oneOf`, sorted. A catch-all whose `not` yields none is refused (`<Union> variant <n>: the catch-all's not
       excludes no value of <key>`), following the MADR's "refuse rather than lose": its `Validate` check could not be emitted.
   * Emitter:
     * `helpers.go`: the value shape converts and takes the address (`v := RequestIdNumber(value); return RequestId{Number: &v}`), with no
       conversion when the parameter type is the variant type; the empty shape writes `&RequestIdNull{}`;
     * every constructor's comment reads `<Name> constructs a <Union> using the '<value>' variant.` when the variant has a discriminator
       value, otherwise `<Name> constructs a <Union> using its <Field> variant.`; a catch-all's comment adds that keys it does not declare
       go in `Extra`, and that `Extra` does not override a key a declared field writes;
     * `types.go`, `unionValidateBody`: after the count, for a catch-all with `Excluded`, a `switch` on its discriminator field that returns
       `fmt.Errorf("<Union>: variant <Field>: <key> %q names another variant", value)` for each excluded value;
     * `types.go`, `unionMarshalBody`: in the non-object branch, for an array variant (`vi.IsArray`), `if *u.<Field> == nil { return
       []byte("[]"), nil }` before marshalling.
   * Tests:
     * `TestHelperParams` covers the new shapes: a value constructor has exactly one parameter, `value`, of the type the rule above gives;
       an empty constructor has none; a catch-all's parameters include its discriminator field;
     * `registry_test.go`, `TestRegistry`: `schemaHelpers` lists the helpers of `Open` too, in model order, and calls `NewOpenOther` with
       one `string` argument;
     * engine goldens regenerated with `-update`; the asserted diff allows only `func New…` lines to be added or changed;
     * `accept-catch-all-variant.txtar` gains a `behaviour_test.go`: its catch-all constructor with each excluded value fails `Validate`
       with an error naming the union, the key and the value; with `_x` and an `Extra` key it validates and encodes the key and decodes back
       to the catch-all with the same `Extra`;
     * new `accept-value-variants.txtar`: an `anyOf` union with a `null` variant, an `integer`, a `string`, an array of strings, a title-only
       variant, a `$ref` to a named string definition and a `$ref` to an enum; `want.api` has each constructor in its shape; its
       `behaviour_test.go` requires the array constructor given `nil` to encode `[]` and decode back to the array variant, and the
       integer constructor given `5` to encode `5`;
     * `conformance_test.go`, `TestConformanceConstructors`:
       * the constructor sets exactly its variant (unchanged, for every union);
       * a discriminated union: the discriminator check and the same-variant round trip (unchanged);
       * a union without a discriminator: decoding sets exactly one variant and re-encoding gives the same bytes (MADR item 8);
       * a catch-all (`u.CatchAll == h.Field`): with its discriminator set by reflection to each other variant's `Value`, `Validate`
         returns an error containing the union's name and the value; set to `_conformance`, `Validate` returns nil and the round trip keeps
         the catch-all.
   * Regenerate. Asserted results:
     * `grep -c '^func New' helpers_gen.go` prints `67`; any other count stops the step for an account of the difference;
     * the exported API listing (the listing program of the 0013 report), run on `git archive HEAD` before the step and on the tree, differs
       only by added `func New…` lines;
     * `types_gen.go` changes only the `Validate` bodies of the six catch-all unions and the `MarshalJSON` bodies of
       `ElicitationContentValue` and `SessionConfigSelectOptions`;
     * root coverage of `helpers_gen.go` is 100%.
   * Plants:
     * the catch-all `switch` removed: the catch-all subtests of `TestConformanceConstructors` and the `accept-catch-all-variant` behaviour
       test fail;
     * the `[]` branch removed: `TestConformanceConstructors` fails for `SessionConfigSelectOptions Ungrouped` and `Grouped` and
       `ElicitationContentValue StringArray`, and `accept-value-variants` fails;
     * constructors limited to `oneOf` again: `TestRegistry` and the goldens fail;
     * the catch-all's discriminator left out of its parameters: `TestHelperParams` fails;
     * a value constructor setting another field: `TestConformanceConstructors` reports `the constructor set`.

6. **Documentation.**
   * `README.md`, "Helper constructors": the generated `New<Union><Variant>` constructors cover every variant of every union except the
     hand-written ones; their parameters are the variant's required keys, named after the keys (`typeValue` for `type`, and the rule for a
     key that is not a Go name); a catch-all takes its discriminator and keeps other keys in `Extra`; a value variant takes `value`.
   * `README.md`, "Wire encoding": a set array variant of a union holding `nil` is sent as `[]`; `Validate` refuses a catch-all holding a
     value another variant has.
   * `docs/architecture.md`: where it describes `helpers.go` and the generated files, `helpers_gen.go` holds a constructor for every
     variant of every union the hand-written file does not cover.
   * `make markdownlint` passes.

7. **Release notes.** `docs/reports/0014-REPORT-release-notes-v0-16-0.md` drafts the GitHub release text against `v0.15.0`, in the shape of
   [0013-REPORT-release-notes-v0-15-0.md](../reports/0013-REPORT-release-notes-v0-15-0.md): breaking changes (the two signatures, with the
   replacement call), changes to what the SDK accepts and sends (the catch-all `Validate` check, `[]` for a nil array variant), additions
   (the constructors, counted by union, from the API listing), and generator changes (the R1 refusal, parameter names). Each statement
   names its source: a record, a commit, or the API listing. The version, `v0.16.0`, is the next minor; the owner confirms it at tag time.
   `docs/README.md` indexes the report.

8. **Close.** Run the verification below, record each result in the execution record, set this plan to `complete`, and update its row in
   `docs/README.md`.

## Verification

* `TestHelperParams` passes on the committed schema and every accepting engine case, and failed on step 3's plant.
* `refuse-union-keys-ref-variant` refuses with its three substrings and leaves the directory unchanged; it failed with the refusal removed.
* `TestParamName` passes on the L1 table, the keywords, the predeclared identifiers and the refusals; it failed on step 4's three plants.
* `TestConformanceConstructors` passes with 67 subtests (none skipped except values that do not encode, each named in the record), and
  failed on each of step 5's plants that names it.
* Engine cases: every `want.api` change is a `func New…` line; `TestEngineCases` and `TestEngineBehaviour` pass.
* `grep -c '^func New' helpers_gen.go` prints `67`; root coverage of `helpers_gen.go` is 100%.
* The exported API listing against `bc0cb78` differs by the two changed `SessionConfigOption` signatures and 56 added constructors, and
  nothing else.
* `go test -race -count=1 ./...` passes at the root and in `cmd/generate`; `GOEXPERIMENT=nojsonv2 go test ./...` passes at the root;
  `go mod tidy -diff` is clean in both modules.
* `make pre-add-check`, `make check-records` and `make markdownlint` pass; the identifier scan finds nothing.
* CI passes on the owner's pushes.

## Rollout and Rollback

Commits land on `main`, made by the owner, one per step; steps 3 to 5 each carry their regenerated files in the same commit, since
`TestGeneratedFilesUpToDate` requires it. The two signature changes break callers at compile time, so the release is a minor version,
`v0.16.0`, with the release notes of step 7; the owner tags and publishes it. Rollback is `git revert` of a step's commit, latest first:
step 5 depends on step 4's `Param.Name`, and step 4 on step 3's `paramKeys`.

## Execution record

Not started.
