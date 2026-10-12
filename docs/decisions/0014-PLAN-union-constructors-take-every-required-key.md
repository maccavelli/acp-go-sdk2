---
status: in-progress
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
   * each new test or check is seen failing on a planted break in a scratch copy ~~(`git archive HEAD` into a temporary directory)~~ (a
     copy of the working tree's tracked and untracked files in a temporary directory, since `git archive HEAD` lacks the step's uncommitted
     changes; amended in step 3), never in the tree; the record of each step quotes the failure;
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
     name`); `schema_gen_test.go` the two matching `New` calls; `types_gen.go` nothing; engine goldens: ~~`accept-discriminated-shapes`
     only~~ `accept-discriminated-shapes`, and `accept-inline-variants` as step 3's deviation of 2026-10-12 changes it.
   * (Added 2026-10-12, step 3's deviation.) The fixtures that held the refused shape move their `$ref` variant into a second union
     without properties of its own: `accept-inline-variants.txtar` and `TestInlineVariants` in `internal/model/fields_test.go`.
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
   * Plants: lowering only the first rune, which must fail `TestParamName` on `uRLPath` ~~and `xAPIKey`~~ (not `xAPIKey`, whose first word
     is one letter, so the plant leaves it unchanged; corrected in step 4) and `accept-constructor-names`;
     dropping the `types.Universe` lookup, which must fail on `errorValue`; dropping the collision check, which must fail the two collision
     cases.

5. **Catch-alls, every union and every variant, nil arrays, doc comments (C1, A3, V3, X1, items 6 to 8).**
   * Model:
     * `buildUnion` builds constructors for every union, `oneOf` or `anyOf`, unless `t.def.DocsIgnore` or it is in `HandWrittenHelpers`;
     * a catch-all's constructor (`idx == t.CatchAll`) has its discriminator among its parameters, under the step 4 name rule; (amended
       in step 5) so does a key-less variant's whose type requires it: the discriminator is skipped only where the constructor writes the
       variant's constant, as the MADR's P1 and item 1 say;
     * a non-object variant (by its JSON type: an object without properties, such as a `map[string]any` variant, keeps the object shape
       and its constructor, amended in step 5) gets a constructor of one of two further shapes, recorded on the `Helper`: a value constructor with one
       parameter named `value`, typed by `Inline.Type` for an inline named variant, by the target's `Underlying` for a `$ref` to a
       `KindPrimitive` definition, and by the target type itself for a `$ref` to any other non-object definition; or an empty constructor,
       with no parameter, for a `null` variant and a variant with no type;
     * a catch-all `Variant` gains `Excluded []string`, the string `const` values of the discriminator in its `not` and in each member of
       the `not`'s `anyOf` or `oneOf`, sorted. A catch-all whose `not` yields none is refused (`<Union> variant <n>: the catch-all's not
       excludes no value of <key>`), following the MADR's "refuse rather than lose": its `Validate` check could not be emitted.
     * (Added 2026-10-12, step 5's deviation.) The fixtures whose catch-all `not` names no value list the other variants' values instead,
       and `TestUnionClassRefusals` gains the refusal as a case: `testdata/engine/accept-catch-all-variant.txtar`,
       `internal/emit/registry_test.go`, `internal/model/classes_test.go`.
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
     * a value constructor setting another field: `TestConformanceConstructors` reports `the constructor set`. (Amended in step 5: no two
       variants of a committed union share a Go type, so setting another field does not compile; the plant sets a second field as well.)

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

   (Amended 2026-10-12, by step 2 of [0015-PLAN-validate-absolute-protocol-paths.md](0015-PLAN-validate-absolute-protocol-paths.md), under
   the owner's choice that 0015 runs before steps 6 to 8.) Step 6 documents both records: besides the items above, the README's "Paths"
   part and the `docs/architecture.md` lines that 0015's step 7 writes are checked for agreement with this step's text, and neither repeats
   the other. Step 7's report covers both records: under changes to what the SDK accepts and sends, the absolute-path checks of the 17
   properties, sending enforced and receiving validated with the consumer's hooks, with a security note; it names the 28 empty-string
   checks `v0.15.0` removed, as the 0013 report's erratum does; under additions, `ValidationError`, `ValidationRule` and its constants,
   `WithInboundRewrite`, `WithInboundValidation` and the five `Roots` methods; under breaking changes, that a non-conforming path is no
   longer sent; and it drafts the GitHub security advisory for `v0.15.0` (affected `v0.15.0`, fixed in `v0.16.0`, referring to issue #2),
   which the owner publishes at the tag.

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

### Step 2, records, 2026-10-12

Ran on `main` from `1067d4a`, after the owner committed the pair and said "proceed". The agent did not commit.

* [0012-MADR-schema-fidelity-framework.md](0012-MADR-schema-fidelity-framework.md) gains "Amendment 2026-10-12, from 0014", placed before
  its "More Information", which is unchanged: outcome 7's list gains the catch-all check, and a nil array variant is sent as `[]`, each
  with the probe that found it.
* This plan's status is `in-progress`, and its row in `docs/README.md` says so.
* No source changes, so no test and no plant.

### Step 3 deviation, 2026-10-12: two fixtures held the shape R1 refuses

With `checkOwnKeys` in place, `go test ./...` in `cmd/generate` failed in `TestEngineCases/accept-inline-variants`,
`TestEngineBehaviour/generate/accept-inline-variants` and `TestHelperParams` with `model: V variant 5: Ref cannot hold the union's own
properties shared`, and in `internal/model`'s `TestInlineVariants` with the same error for variant 6. Both fixtures, written before this
plan, give the union `V` a required key of its own, `shared`, and a variant `{"$ref": "#/$defs/Ref"}`, whose generated type `Ref` has no
`Shared` field: the loss R1 refuses. Neither tests `shared` for that variant; the `$ref` is there for the assertion that a `$ref` variant
has no inline type. The MADR's "no committed union has this shape" was checked against the schema files only.

Asked, the owner chose to move the `$ref` variant, in both fixtures, out of `V` into a second union `W` with no properties of its own,
beside an inline variant, keeping the assertion. Not chosen: dropping the `$ref` variant, which loses the assertion; revisiting R1, which
the MADR decided. The MADR's fact is amended. Files added to step 3: `internal/model/fields_test.go`, and the `want.api` and behaviour
test of `accept-inline-variants.txtar`.

### Step 3, parameters and the refusal, 2026-10-12

Ran on `main` from `98b2372`. The agent did not commit.

* `internal/model/model.go`: `paramKeys` returns the required keys of the variant's Go type and the properties that type them (the union's
  own keys, then the variant's, for an inline variant; the named definition's for a `$ref` variant), and `buildUnion`'s constructor loop
  uses it. `checkOwnKeys`, called after `classifyUnion`, refuses a union with properties of its own and a `$ref` or single-`allOf` `$ref`
  variant. The `Helper.Params` comment, and the emitter's at `internal/emit/helpers.go`, say what the parameters are.
* `cmd/generate/helpers_test.go`, `TestHelperParams`: on the committed schema and every accepting engine case, each constructor's parameter
  fields, with the discriminator field when the constructor writes the variant's value, equal the required fields of the variant's Go
  type. A catch-all's discriminator field is left out of the comparison until step 5 makes it a parameter.
* Engine cases: new `refuse-union-keys-ref-variant.txtar` (`want.err`: `Event variant 1:` and `Start cannot hold the union's own
  properties id`) and `accept-union-own-keys.txtar` (`func NewOptA(id string, name string, value int64) Opt`, `func NewOptB(id string,
  name string) Opt`; its behaviour test requires `NewOptB("i", "n")` to encode `{"id":"i","name":"n","type":"b"}` and decode back to `B`).
  `-update` changed two goldens, each line reviewed: `accept-discriminated-shapes` (`-func NewEventStart() Event`, `+func
  NewEventStart(at int64) Event`) and, under this step's deviation, `accept-inline-variants` (`V` loses `Ref`; `W`, `WVariant1` and their
  methods are added).
* Regenerated. The asserted diff script passed: `helpers_gen.go: 8 changed lines`, `schema_gen_test.go: 4 changed lines`, `types_gen.go`,
  `agent_gen.go`, `client_gen.go`, `constants_gen.go`: 0, `asserted diff: ok`; the 8 lines are the two signatures, old and new, and `Id:
  id` and `Name: name` in each body.
* Tests: `go test -count=1 ./...` passes in `cmd/generate` (all seven packages `ok`) and at the root; `TestConformanceConstructors` runs
  its 11 subtests, 10 passing and `SessionConfigOption_Select` skipped as before (its zero value does not encode).
* Plants, each in its own scratch copy:
  * `paramKeys` replaced by the variant's own keys: `exit=1`, `committed schema: NewSessionConfigOptionBoolean: missing the required fields
    [Id Name], has the fields [] the type does not require`, and the same for `NewOptA` and `NewOptB` of `accept-union-own-keys`, which
    also failed `TestEngineCases`;
  * the refusal disabled: `exit=1`, `TestEngineCases/refuse-union-keys-ref-variant`: `run accepted the schema`.
* `make pre-add-check`: `go-precheck: 101 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck).` The untracked
  `cmd/generate/helpers_test.go`: `go-precheck: 1 file(s) clean`, with `govulncheck` skipped. `make check-records` and `make markdownlint`
  exit 0, and the identifier scan of the changed files found nothing.
* Breaking change for the release notes: `NewSessionConfigOptionSelect` and `NewSessionConfigOptionBoolean` take `id SessionConfigId,
  name string` first.

### Step 4, names, 2026-10-12

Ran on `main` from `75bf968`. The agent did not commit.

* `internal/model/names.go`: `paramName` replaces `checkParam`, its one caller being `buildUnion`. An identifier or keyword key keeps its
  text; any other key goes through `lowerCamel`, which splits it at runs of characters that are not ASCII letters or digits, puts each word
  in Go case with `util.ToExportedField`, and lowers the first word with `lowerLead` (L1). A name for which `token.IsKeyword` holds or
  `types.Universe.Lookup` is not nil gets `Value` (N1). A result that is not an identifier is refused with the message `checkParam` had.
* `internal/model/model.go`: `Param` gains `Name`; `buildUnion` refuses two parameters of one constructor with one name (`<owner>:
  parameters <key> and <key> are both named <name>`). `internal/emit/helpers.go` uses `p.Name`.
* Tests: new `internal/model/names_test.go`, `TestParamName`, with 24 names (identifier keys kept, the table of the MADR's finding 12,
  the three keywords the schema files require, `$type`, and the predeclared `error`, `string`, `any`, `min`, `nil`) and 4 refusals (`$`,
  `--`, `2fa`, `9-lives`). The keyword cases of `TestBuildRefuses` and `TestRunRefusesBeforeWriting` became collision cases (`type` and
  `typeValue` both required), and `TestBuildNames` compares `Param` with its `Name`. New engine case `accept-constructor-names.txtar`:
  `func NewUA(rangeValue string, errorValue string, xAPIKey string, urlPath string) U`; its behaviour test requires `NewUA("r", "e",
  "k", "p")` to encode `{"URL-path":"p","error":"e","kind":"a","range":"r","x-API-key":"k"}`.
* Regenerated: no generated file changed (`git diff` of `*_gen.go` and `schema_gen_test.go`: 0 lines), as no committed constructor's key
  is a keyword, predeclared, or not an identifier.
* Tests: `go test -count=1 ./...` passes in `cmd/generate` (seven packages `ok`) and at the root.
* Plants, each in its own scratch copy:
  * `lowerLead` lowering the first rune only: `exit=1`; `TestParamName` failed on `URL-path` (`"uRLPath"`), `HTTPServer-url`,
    `XMLHttp-request`, `HTTP2-server` and `ACME-id`, and `TestEngineCases/accept-constructor-names` got `func NewUA(rangeValue string,
    errorValue string, xAPIKey string, uRLPath string) U`. `xAPIKey` passed, its first word being one letter; the plan's text is corrected;
  * the `types.Universe` lookup disabled: `exit=1`, `paramName("error") = "error", <nil>; want "errorValue"`, and the same for `string`,
    `any`, `min`, `nil`;
  * the collision check disabled: `exit=1`, `TestBuildRefuses`: `Build accepted the schema`.
* `make pre-add-check`: `go-precheck: 102 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck).` The untracked
  `internal/model/names_test.go`: `go-precheck: 1 file(s) clean`, with `govulncheck` skipped. `make check-records` and `make
  markdownlint` exit 0, and the identifier scan of the changed files found nothing.

### Step 5 deviation, 2026-10-12: fixtures with a catch-all whose not names no value

With step 5's model in place, `go test ./...` in `cmd/generate` failed with `model: U variant 2: the catch-all's not excludes no value of
"type"` in `TestEngineCases/accept-catch-all-variant`, `TestEngineBehaviour/generate/accept-catch-all-variant` and `TestHelperParams`; with
`Open variant 2` in `internal/emit`'s `TestRegistry`; and in two cases of `internal/model`'s `TestUnionClassRefusals` (`two catch-alls`,
`catch-all property named extra`), which test other refusals. The four fixtures, written before this plan, give the catch-all `"not":
{"required": ["type"]}`, which names no excluded value and, read as JSON Schema, refuses every payload that carries `type`, which the
variant also requires. Both schema files list `const` values in each catch-all's `not`; `isCatchAll` accepted any `not`.

Asked, the owner chose to fix the fixtures: each `not` becomes an `anyOf` of the other variant's value, `"a"`, the schema files' shape,
and the refusal is kept as planned. `TestUnionClassRefusals` gains `catch-all excluding no value`, the old placeholder, which pins the
refusal. Not chosen: dropping the refusal, which leaves such a catch-all unchecked; ceasing to treat such a variant as a catch-all, a
change of classification the MADR does not decide. The MADR is unchanged: the refusal is this plan's, under its "refuse rather than lose".

### Step 5, catch-alls, every union and every variant, nil arrays, doc comments, 2026-10-12

Ran on `main` from `4b21cbe`. The agent did not commit. The deviation above, decided by the owner, ran within this step.

* `internal/model/model.go`:
  * constructors for every union, `oneOf` or `anyOf` (the `!t.ExactlyOne` test is gone), unless documentation-ignored or hand-written;
  * the discriminator is a parameter wherever the constructor does not write the variant's constant: the catch-all's, and, found while
    running `TestHelperParams`, the key-less variants of `accept-key-less-variant` and `accept-key-less-variant-without-keys`, whose types
    require it (`NewULoose(typeValue string, x string)`). This follows the MADR's P1 and item 1; the plan's bullet is amended;
  * `valueParam` decides the value shape: a variant that is not an object, `null`, or untyped takes `value`, typed by its inline named
    type, by the underlying type of a named primitive it refers to, or by the definition it refers to. The shape is decided by the JSON
    type, not `Variant.IsObject`, which is false for an object without properties: `NewEventNone()` keeps its signature;
  * `Helper.Value` marks the value shape; `Variant.Excluded` holds the catch-all's excluded values, read by `excludedValues` from its
    `not` and the members of the `not`'s `anyOf` or `oneOf`; a catch-all without any is refused.
* `internal/emit`: `helpers.go` emits the value shape (`v := RequestIdNumber(value); return RequestId{Number: &v}`, or `&value` when the
  parameter is the variant's own type) and the doc comments (`using the '<value>' variant`, else `using its <Field> variant`; a
  catch-all's two lines on `Extra`); `types.go`'s `unionValidateBody` takes the type and emits the catch-all `switch`, and
  `unionMarshalBody` returns `[]` for a set array variant holding nil.
* Tests: `TestHelperParams` checks the value shape and counts every discriminator; `TestRegistry` expects `Closed.X`, `Closed.Y`, `Open.A`,
  `Open.Other` and the call `NewOpenOther(*new(string))`; `TestUnionClassRefusals` gains `catch-all excluding no value`;
  `accept-catch-all-variant` gains `TestCatchAllConstructor`; new case `accept-value-variants.txtar` (`NewValColor(value Color)`,
  `NewValCount(value int64)`, `NewValMarker()`, `NewValName(value string)`, `NewValText(value string)`, `NewValValVariant1()`,
  `NewValWords(value []string)`; its behaviour test sends `NewValWords(nil)` as `[]`); `TestConformanceConstructors` applies the MADR's
  item 8 and checks each catch-all with `checkCatchAll` and `setDisc`.
* Engine goldens, asserted against a snapshot taken before the step: `func New lines: 38 added, 1 removed; unexpected: 0` across eleven
  existing cases (the one removal is `-func NewEventOther() Event`, `+func NewEventOther(kind string) Event`); the new
  `accept-value-variants` was reviewed line by line.
* Regenerated. `grep -c '^func New' helpers_gen.go`: `67`. The exported API listing of `git archive HEAD` (2308 lines) against the tree
  (2364 lines): `added=56 removed=0 added-non-New=0`. `types_gen.go`, asserted by function: 6 lines in the `Validate` of each of
  `CreateElicitationRequest`, `CreateElicitationResponse`, `ElicitationPropertySchema`, `IdleStateUpdate`, `MultiSelectItems` and
  `StateUpdate`; 3 in `ElicitationContentValue`'s and 6 in `SessionConfigSelectOptions`' `MarshalJSON`; no line removed; `types_gen.go
  asserted diff: ok`.
* `TestConformanceConstructors`: 67 subtests, 63 passing and 4 skipped because the zero-argument value does not encode, each for a nested
  required union with no variant set: `SessionConfigOption_Select` (`SessionConfigSelectOptions`), `ElicitationPropertySchema_Array`
  (`MultiSelectItems`), `CreateElicitationRequest_Form` and `CreateElicitationRequest_Url` (the elicitation scope). Root coverage: all 67
  functions of `helpers_gen.go` at `100.0%`.
* Tests: `go test -count=1 ./...` passes in `cmd/generate` and at the root; `GOEXPERIMENT=nojsonv2 go test -count=1 ./...` passes at the
  root.
* Plants, each in its own scratch copy, all `exit=1`:
  * the generator's catch-all check disabled: `TestEngineBehaviour`, `NewUOther("a").Validate() = <nil>, want an error naming U, type and
    "a"`;
  * the `ElicitationPropertySchema` check removed from `types_gen.go`: `the catch-all holding type "string": Validate() = <nil>, want an
    error naming ElicitationPropertySchema and the value`;
  * the generator's `[]` branch disabled: `TestEngineBehaviour`, `NewValWords(nil) encoded as null, <nil>`;
  * `SessionConfigSelectOptions`' `[]` branch removed from `types_gen.go`: `null decoded into [], want one variant`;
  * constructors for `oneOf` only: `schemaHelpers [Closed.X Closed.Y], want [Closed.X Closed.Y Open.A Open.Other]`, and `TestEngineCases`
    failed;
  * the discriminator never a parameter: `committed schema: NewElicitationPropertySchemaOther: missing the required fields [Type]`;
  * `NewRequestIdNumber` also setting `Null` (the plan's plant adapted, see above): `the constructor set [Null Number], want Number`;
  * the catch-all refusal disabled: `TestUnionClassRefusals`, `Build accepted the schema`.
* `make pre-add-check`: `go-precheck: 103 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck).` No untracked Go file.
  `make check-records` and `make markdownlint` exit 0, and the identifier scan of the changed files found nothing.
* For the release notes: 56 constructors added; `Validate` refuses a catch-all holding another variant's value; a nil array variant is
  sent as `[]`.

### Step 6, documentation, 2026-10-12

Ran on `main` from `b59ced2`, after 0015 closed, as the amendment to this step requires. The agent did not commit. No Go file changed.

* `README.md`, "Helper constructors": a bullet on the generated `acp.New<Union><Variant>` constructors: every union but the three
  hand-written ones and `AgentResponse` and `ClientResponse` (marked `x-docs-ignore`, which `buildUnion` skips, `model.go:602`; neither
  has a constructor in `helpers_gen.go`); parameters are the variant's required keys in the order of the `required` list (`paramKeys`),
  named after the key, a keyword or predeclared name gaining `Value` and a non-Go key joined in camel case (`x-API-key` to `xAPIKey`, from
  `accept-constructor-names`); a discriminator the constructor cannot write is a parameter, a catch-all keeping other keys in `Extra`; a
  non-object variant takes `value`. Each example names a function in `helpers_gen.go`: `NewSessionConfigOptionBoolean(id, name,
  currentValue)`, `NewElicitationPropertySchemaOther(typeValue)`, `NewRequestIdNumber(value int64)`.
* `README.md`, "Wire encoding": the nil-array bullet adds a union's set array variant holding nil, with
  `acp.NewSessionConfigSelectOptionsUngrouped(nil)`; the `Validate` bullet adds the catch-all check, quoting the generated message form
  (`ElicitationPropertySchema: variant Other: type %q names another variant`, `types_gen.go`); the list of decisions gains this record.
* `docs/architecture.md`, "What it is": `helpers.go` holds the constructors of the three unions the protocol table names, `Ptr` and
  `Roots`; `helpers_gen.go` a constructor for every variant of every other union the schema documents.
* Agreement with 0015 (the amendment): the README's `Validate` bullet now names both records' checks once and points to "Paths" for the
  path rule; "Paths" mentions no constructor; `docs/architecture.md`'s 0015 lines (the table's `AbsolutePaths`, the `validate` pair, the
  sending and receiving checks) are unchanged, and its `helpers.go` line now also names 0015's `Roots`. Neither file repeats the other
  record's text.
* `make markdownlint` and `make check-records` exit 0; the identifier scan of the changed files found nothing. No generated file and no
  Go file changed, so no Go gate ran.
