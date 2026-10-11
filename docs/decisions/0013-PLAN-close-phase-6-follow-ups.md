---
status: in-progress
date: 2026-10-11
associated-madr: "0013-MADR-close-phase-6-follow-ups.md"
---

# Implement the phase 6 follow-ups

Associated MADR: [0013-MADR-close-phase-6-follow-ups.md](0013-MADR-close-phase-6-follow-ups.md)

## Goal

Every union constructor of `helpers_gen.go` runs from the root tests and is checked against the schema; `util.SanitizeComment` is gone; and
the notes of the next release, `v0.15.0`, are drafted from the records and the API listing, ready to paste into the GitHub release. The
exported API and the wire do not change.

This plan carries out the MADR's options C1, S1, and R1, which the owner chose and approved on 2026-10-11.

## Scope

In scope:

* `cmd/generate/internal/util/util.go`: `SanitizeComment` removed.
* `cmd/generate/internal/emit/registry.go` and `registry_test.go`: a list of constructors in `schema_gen_test.go`.
* `schema_gen_test.go`, regenerated.
* `conformance_test.go` at the root: the constructor test.
* `docs/reports/0013-REPORT-release-notes-v0-15-0.md` (new) and `docs/README.md` (its index row).
* This plan's records.

Out of scope:

* Whether a constructor should take the keys a union requires of every variant (`SessionConfigOption`'s `id` and `name`): the MADR records
  the fact and decides nothing; a change would be its own decision.
* The tag, the GitHub release, and the push: the owner's.

## Implementation Steps

1. **Rules.** As in [0011-PLAN-engine-tests.md](0011-PLAN-engine-tests.md), step 1: each new test fails first or on a planted fault in a
   scratch copy, never in the tree; a behaviour no record decides stops the step for the owner; after each step, regenerate, run
   `make pre-add-check` (and the untracked Go files by name), `make check-records`, `make markdownlint`, and the identifier scan, record
   the output, and stop for the owner's commit on `main`.
2. **Remove `SanitizeComment`.** Delete the function. Confirm with `grep` that nothing names it, and that the module builds and its tests pass.
3. **The constructor list and its test.**
   * `internal/emit`: the registry emits `schemaHelpers()`, a `schemaHelper{Union, Field string; New func() any}` per constructor, `New`
     calling it with each argument's zero value. `TestRegistry` (on its synthetic model) requires one entry per helper of the model, in its
     order, with the union and variant field.
   * At the root, `TestConformanceConstructors`: for each entry, the value it builds has exactly that variant set; ~~encoded, it carries the
     discriminator value `schemaUnions()` gives the variant, when the union has one; and decoded again, it sets the same variant~~ its
     discriminator field holds the value `schemaUnions()` gives the variant, when the union has one; and a value that encodes decodes back to
     the same variant (amended 2026-10-11, see step 3's deviation).
   * Plants in scratch copies: a constructor in `helpers_gen.go` setting another variant; one writing a wrong discriminator; the registry
     leaving out a helper.
   * Root coverage of `helpers_gen.go`: 100%.
4. **The release notes.** `docs/reports/0013-REPORT-release-notes-v0-15-0.md` holds the text of the GitHub release for `v0.15.0`, against
   `v0.14.0`:
   * the schema version, from `schema/version`;
   * the exported API changes, from the listing program run on `git archive v0.14.0` and on the tree, each line grouped as added, changed,
     or removed;
   * every change to what the SDK sends or accepts, from the 0012-MADR's amendments and the 0011-MADR's "Found in step" sections, each with
     the record it comes from;
   * what consumers must change in their code, with the replacement for each removed name.

   Each statement names its source; a statement without one is cut. `docs/README.md` indexes the report.
5. **Close.** Run the verification below, record each result, set this plan to `complete`, and the MADR to `accepted` if the owner
   approved it unchanged.

## Verification

* `grep -rn SanitizeComment` finds nothing; `cmd/generate` builds and its tests pass.
* `TestConformanceConstructors` passes, has an entry for each of the 11 constructors, and fails on each plant of step 3; `helpers_gen.go`
  is at 100% from the root tests.
* The five SDK files are unchanged; `schema_gen_test.go` gains only the constructor list; the exported API listing is unchanged from
  `85ef7a8`.
* Each statement of the report traces to a record, a commit, or the API listing.
* `go test -race -count=1 ./...` passes at the root and in `cmd/generate`; `GOEXPERIMENT=nojsonv2 go test ./...` passes at the root.
* `make pre-add-check`, `make check-records`, and `make markdownlint` pass; the identifier scan finds nothing.

## Rollout and Rollback

Commits land on `main`, made by the owner, one per step. No tag; the owner tags `v0.15.0` and pastes the report's text into the GitHub
release when ready. Rollback is `git revert` of a step's commit; the steps are independent.

## Execution record

### Step 2, remove SanitizeComment, 2026-10-11

Ran on `main` from `85ef7a8`, after the owner approved the pair. The agent did not commit.

* `SanitizeComment` and its doc comment are deleted from `cmd/generate/internal/util/util.go`; the package's imports are unchanged, since the
  other helpers use them.
* `grep -rn SanitizeComment --include=*.go .` finds nothing (exit 1). `cmd/generate` builds; `go vet ./internal/util` passes;
  `internal/util` has no tests, and `internal/emit` and the `cmd/generate` package pass. Regenerating changes no generated file.
* No test is added: the step removes code that nothing reaches, so there is nothing to fail first.
* `make pre-add-check`: `go-precheck: 101 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck).` `make check-records` and
  `make markdownlint` exit 0, and the identifier scan of the changed files found nothing.

Commit point: the agent stopped here. The owner commits the pair (the two records and `docs/README.md`) first, as the bootstrap exception
requires, then step 2 (`util.go` and this record) on `main`.

### Step 3 deviation, 2026-10-11: a constructor whose zero-argument value does not encode

The owner committed the pair as `f12c010` and step 2 as `735c524`. Called with zero arguments, `NewSessionConfigOptionSelect` builds a value
holding an empty `SessionConfigSelectOptions` union, which refuses to encode (`SessionConfigSelectOptions: no variant set`), so the step's
"encoded, it carries the discriminator" cannot hold for it. Resolutions put to the owner: check the discriminator in the Go value for every
constructor and round-trip those whose value encodes (recommended), or build schema instances as arguments by reflection. The owner chose the
first. Step 3 is annotated; the MADR's decision does not change.

### Step 3, the constructor list and its test, 2026-10-11

Ran on `main` from `735c524`. The agent did not commit.

* `internal/emit/registry.go`: `schema_gen_test.go` gains the type `schemaHelper{Union, Field string; New func() any}` and
  `schemaHelpers()`, one entry per constructor of the model in the order of `helpers_gen.go`; `New` calls it with `*new(T)` for each argument
  and returns a pointer to the union.
* `TestRegistry`: its synthetic `Closed` union gained a required key on one variant, and the test requires `schemaHelpers` to list
  `Closed.X` and `Closed.Y` and to call `NewClosedX()` and `NewClosedY(*new(string))`. Before the emitter change it failed:
  `schema_gen_test.go has no func schemaHelpers`.
* `TestConformanceConstructors` (`conformance_test.go`): before, `undefined: schemaHelpers`. Now 11 subtests: 10 pass, and
  `SessionConfigOption_Select` passes its variant and discriminator checks and skips the round trip, as the deviation records.
* Root coverage of `helpers_gen.go`: 11 of 11 statements, 100% (18.2% before); each of its 11 functions at 100%.
* Plants in scratch copies (`s13_plants.json`), each failing as planted:
  * `NewRequestPermissionOutcomeCancelled` setting `Selected`: `the constructor set [Selected], want Cancelled`;
  * `NewPlanUpdateContentFile` writing `fil`: `the discriminator type is "fil", want "file"`;
  * `NewSessionConfigOptionSelect` writing `selec`: `the discriminator type is "selec", want "select"`, so the skipped round trip does not hide
    its checks;
  * the registry skipping each union's first constructor: `schemaHelpers [Closed.Y], want Closed.X and Closed.Y`. Its first version,
    `t.Helpers[1:]`, panicked on a type without constructors; rewritten, it failed as planted.
* The five SDK files are unchanged; `schema_gen_test.go` gains 102 lines; the exported API listing is unchanged from `85ef7a8`.
* The root tests pass by default, with `-race`, and with `GOEXPERIMENT=nojsonv2`.
* `make pre-add-check`: `go-precheck: 101 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck).` `make check-records` and
  `make markdownlint` exit 0, and the identifier scan of the changed files found nothing.

Commit point: the agent stopped here. The owner commits step 3 on `main`.
