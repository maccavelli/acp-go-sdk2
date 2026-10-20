---
status: in-progress
date: 2026-10-20
associated-madr: "0024-MADR-go-sdk-best-practices-on-a-go-1-26-9-floor.md"
---

# Implement phase 4 of 0024: the API gate and deprecation notices

Associated MADR: [0024-MADR-go-sdk-best-practices-on-a-go-1-26-9-floor.md](0024-MADR-go-sdk-best-practices-on-a-go-1-26-9-floor.md)

## Goal

Implement phase 4 of the MADR's outcome: findings V2 and V3, as questions 6 and 7 decided them on 2026-10-19, and as the MADR's
"Phase 4 scoped, 2026-10-20" specifies them, with the owner's answers to questions 9 to 13 the same day.

* **V2 (question 6, question 12):** `gorelease`, pinned, gates the root module's exported API against the latest release, in CI and
  as `make api`. A change it calls incompatible fails, with no override.
* **V3 (questions 7, 9, 10, 11):** the generator models the schema's `deprecated` keyword on a definition, a property or a union
  variant. Each Go declaration made from a marked node gets a `Deprecated:` paragraph. The paragraph is a fixed sentence naming the
  node, then an optional note from the protocol table naming the replacement. A request or notification definition also marks its
  methods.
* **Question 13:** a root test fails on a hand-written notice in a form tools ignore.
* **`RELEASING.md`:** `make api` before tagging, `make api`'s output in each release's notes, and an annotated tag command (R12).

The committed schema marks nothing deprecated (D1). Every generated file therefore stays byte-identical, and the exported API does
not change: `gorelease` against `v1.17.0` must report no change. No behaviour changes. The release is proposed as the patch `v1.17.1`,
which the owner confirms at the tag.

## Facts this plan rests on

Each was read on `f6f9d73` on 2026-10-20, which differs from `b463216`, where the MADR's findings were read, only in the MADR. Step 1
re-asserts the code facts with a script. The MADR's R1 to R12 and D1 to D13 hold the probes; this table holds what the steps change.

| For | Fact |
| :--- | :--- |
| gate | `ci.yml`: one `validate` job over three systems and two Go versions. `GOTOOLCHAIN: local` (`:38`); the checkout is `fetch-depth: 1` (`:43`); `- run: go mod download` is the first step after `setup-go` (`:55`). Three steps run only on Linux at `1.27.2`, `if: runner.os == 'Linux' && !matrix.floor` (`:71`, `:82`, `:93`). `govulncheck` is pinned there, `@v1.9.0` (`:79`, `:80`). |
| gate | `Makefile:27`: `GOVULNCHECK_PIN` is read from `ci.yml` with `grep -oE`. `vuln` (`:80`-`:86`) fails when the pin is missing. `.PHONY` at `:43`. `make help` lists every target with a `## ` comment. `make release VERSION=` takes a schema version (`RELEASING.md:23`). |
| gate | `RELEASING.md`: "Tag and Publish" (`:80`) shows `git tag vX.Y.Z` (`:85`); "Release notes and the API" (`:97`) says each release's notes record the exported API listing against the last tag (`:103`). |
| gate | `docs/architecture.md`: the checks (`:384`-`:390`) name `make pre-add-check`, `make check-records`, `make markdownlint` and CI's `nojsonv2` run, and no API check. |
| V3 | `load.Definition` (`cmd/generate/internal/load/load.go:38`) has no `deprecated` field. `Definition.UnmarshalJSON` (`:97`) decodes strictly, so the keyword is refused today (D5). |
| V3 | `model/keywords.go:30`: the registry. `TestKeywordRegistryMatchesLoader` (`keywords_test.go:18`) requires it to equal `load.Definition`'s fields. [0012-MADR-schema-fidelity-framework.md](0012-MADR-schema-fidelity-framework.md) holds the keyword table (`:111`-`:121`), annotated once already by this MADR (`:123`). |
| V3 | `model/schema.go`: `placement` (`:213`) is `placeDefinition`, `placeProperty` (a property of an object definition that is not a union), `placeVariant` (a direct `anyOf` or `oneOf` member of a definition) or `placeOther`. `checkTypable` (`:234`) assigns them, and skips `not` and `unevaluatedProperties`. `subschemas` (`:355`) lists every child. |
| V3 | The model's declarations: `Type` (`model.go:109`), `Variant` (`:176`), `EnumValue` (`:238`), `Helper` (`:245`), `Field` (`fields.go:119`), `Struct` (`:148`), `Inline` (`:172`), `Member` (`:192`) and `Method` (`methods.go:33`). Descriptions become docs at 13 calls of `docLines` (D7). A method's doc is the description of its notification, request or response definition, the first that has one (`methods.go:210`-`:216`). |
| V3 | The protocol table's doc notes: `DocNote` (`model/protocol.go:65`), checked by `checkDocNotes` (`:371`), applied by `applyDocNotes` (`:832`). A note on a definition the schema lacks applies nothing; `checkCommittedDocNotes` (`patch_test.go:273`) catches it against the committed schema. `CapabilityRule` names a variant by its discriminator value (`protocol.go:111`). |
| V3 | The emitter's docs: `typeDoc` (`emit/types.go:160`), `defPlace` (`:155`), `schemaVariantPlace` (`:173`), `enumSpec` (`:141`), `addRPCMethod` (`:89`), `emitStruct` (`:246`), `emitUnion` (`:1267`), `emitHelperConstructors` and `helperDoc` (`emit/helpers.go:15`, `:61`), `outboundDoc` and `wrapDoc` (`emit/dispatch.go:689`, `:755`). A nested struct's place is "the %q property of" its definition (`emit/types.go:235`). |
| V3 | Engine cases: `runEngineCase` (`cmd/generate/engine_test.go:47`) requires exactly one of `want.err` and `want.api` (`:72`-`:77`), and `-update` rewrites `want.api` (`:98`-`:103`). `enginetest.API` (`internal/enginetest/api.go:104`) lists signatures, not docs. |
| Q13 | The root's hand-written non-test files are `connection.go`, `doc.go`, `errors.go`, `extensions.go` and `helpers.go`; the other five non-test files start "Code generated". No `.go` or `.txtar` file mentions "deprecated", in any case (D13). |
| — | `scripts/check_records.py --next` gives `0025`; this PLAN takes the MADR's number. |

## Scope

In scope:

* **CI:** one step in `.github/workflows/ci.yml`.
* **Make:** `GORELEASE_PIN` and the `api` target in the `Makefile`.
* **The generator:** `cmd/generate/internal/load` (the keyword), `internal/model` (the registry, the placement check, the flags, the
  protocol table's deprecation notes), `internal/emit` (the notices), `internal/enginetest` (listing notices), `engine_test.go` (the
  `want.deprecated` section), with their tests, and new engine cases.
* **The root:** `deprecation_test.go`, new. No non-test root file changes; the regenerated files stay byte-identical.
* **Docs:** `RELEASING.md`, `docs/architecture.md`.
* **The records:** this PLAN and its row; the annotation of 0012's keyword table; the release notes
  `docs/reports/0024-REPORT-release-notes-v1-17-1.md` and their row.

Out of scope:

* any deprecation of an existing declaration: nothing is deprecated today (D1, D13);
* an override for an intended incompatible change (question 12);
* the unstable API's removal policy and schema `1.26.0` (V1; the record question 16 of
  [0019-MADR-remediate-debugging-pass-findings.md](0019-MADR-remediate-debugging-pass-findings.md) calls for). From step 2 on, that
  update fails the gate until its record decides (R10);
* `gorelease` in `cmd/generate`, which has no released version (R8);
* a README note on compatibility or deprecation for consumers (see "Points for the owner's review");
* `//go:fix inline` directives, which the Go wiki suggests for a newer, preferred API (D9).

## Design

### The gate

**CI.** A step directly after `go mod download` (`ci.yml:55`), so no earlier step has written the tree (R6):

```yaml
      # The exported API against the latest release, by gorelease: a change it
      # calls incompatible fails the job (0024-PLAN-api-gate-and-deprecation.md).
      # It needs a clean tree, so it comes before every step that builds.
      - name: API compatibility (gorelease)
        if: runner.os == 'Linux' && !matrix.floor
        run: go run golang.org/x/exp/cmd/gorelease@v0.0.0-20261014143641-f0251eb4aa3e
```

* **No flags.** No `-base`: the proxy gives the latest release, and a named older base cannot suggest a version (R3). No `-version`:
  `make api TAG=` checks a proposed tag before tagging.
* **When it runs:** each push to `main`, each pull request, the weekly schedule, and a tag push, where it compares the tag with itself
  (R4).
* **One job.** It runs once per workflow run, on the job that runs lint. It works on go1.26.9 too (R3); one run is the gate.

**`make api`.**

```make
GORELEASE_PIN := $(shell grep -oE 'golang.org/x/exp/cmd/gorelease@v0\.0\.0-[0-9]{14}-[0-9a-f]{12}' .github/workflows/ci.yml | head -1)

api: ## Runs gorelease, pinned in CI, on the last commit; TAG=vX.Y.Z checks a proposed tag
	@if [ -z "$(GORELEASE_PIN)" ]; then \
		echo "no gorelease pin in .github/workflows/ci.yml"; \
		exit 1; \
	fi
	go run $(GORELEASE_PIN)$(if $(TAG), -version=$(TAG))
```

* The pin has one home, `ci.yml`, as `govulncheck`'s does.
* `TAG`, not `VERSION`, which `make release` uses for a schema version.
* It checks the last commit and refuses a dirty tree with `gorelease`'s own message (R6). It is not part of `make pre-add-check`. Offline
  it fails with R7's message.
* `api` joins the `.PHONY` line.

**`RELEASING.md`.**

* **"Tag and Publish"** gains a first step: `make api TAG=vX.Y.Z` on the commit to be tagged must exit 0. The tag command becomes
  `git tag -a vX.Y.Z -m vX.Y.Z` (R12).
* **"Release notes and the API"**: each release's notes record `make api`'s output against the last tag, its compatible and
  incompatible changes and its suggested version, in place of the listing program. An incompatible change still needs `/v2`.

### The keyword

* **`load.Definition`** gains `Deprecated bool` with the tag `json:"deprecated"`, documented as the JSON Schema 2020-12 annotation
  (D4).
* **`null` is refused.** `Definition.UnmarshalJSON` fails on a `deprecated` key whose value is `null`, which a `bool` would take as
  `false` (D12): "deprecated is null; it must be true or false". A string or a number is already refused by the decoder (D12).
* **The registry** gives `"deprecated"` `TreatAnnotation`.
* **0012's keyword table** is annotated with the row: `deprecated` | annotation | a `Deprecated:` paragraph on the declarations made
  from the node; refused where no declaration is made.
* **Placement.** A new `checkDeprecated`, called from `checkRefs` (`schema.go:200`), walks every node with `checkTypable`'s
  placements. Unlike `checkTypable`, it also walks `not` and `unevaluatedProperties`. `deprecated: true` is refused at
  `placeOther`, and on a variant the model skips (a `null` entry), naming the path: "<path>: deprecated here marks no Go
  declaration; only a definition, a property of an object definition, or a direct anyOf or oneOf member of a definition can be
  deprecated". `false` is accepted anywhere, as absent.

### The notices

**What the model carries.** A `Deprecation` value, nil when the node is not marked:

```go
// Deprecation marks a declaration built from a node the schema marks
// deprecated. Note is the protocol table's replacement for it, or "".
type Deprecation struct {
	Note string
}
```

It is a field of each declaration the specification's table names:

| Schema node | Model | Go declarations |
| :--- | :--- | :--- |
| a definition | `Type.Deprecation`; for a request or notification definition, also `Method.Deprecation` | the type; the method of its interface and of its side's connection |
| a property of an object definition | `Field.Deprecation`; for an inline object property, also `Struct.Deprecation` | the struct field; the nested type |
| a union variant | `Variant.Deprecation`, which the variant's inline type and helper read; for a constant variant, `EnumValue.Deprecation` | the union's field, the inline type, the helper constructor; the constant |

A response definition marks its type only, as question 11 decided, even when the method's doc text is the response's description (`methods.go:210`-`:216`). A method's mark comes from its request or notification definition alone.

**The notice's text,** composed in the emitter, which names places already (`defPlace`, `schemaVariantPlace`, the nested struct's):

* the sentence: `Deprecated: the ACP schema marks <place> deprecated.`;
* the place:
  * a definition: `the McpServerSse schema definition`;
  * a property: `the "sse" property of the McpCapabilities schema definition`;
  * a variant: `the "sse" variant of the McpServer schema definition`, or its field name when it has no discriminator value;
* a method's notice names its request or notification definition;
* the note, when there is one, follows after a space;
* the whole is wrapped by `wrapDoc`, and added as the doc's last paragraph after a blank comment line (D10).

**The protocol table's notes.** `ACP` gains `DeprecationNotes []DeprecationNote`, empty today:

```go
// DeprecationNote names the replacement of a node the schema marks
// deprecated: Definition, as the merged schema names it, and Property or
// Variant (a discriminator value, or the field name of a variant without one)
// when the node is one of those.
type DeprecationNote struct {
	Definition, Property, Variant, Note string
}
```

* **`checkDeprecationNotes`,** beside `checkDocNotes`, refuses a note without a definition or a note text, one with both a
  property and a variant, and two notes on one node.
* **Applying a note** (`applyDeprecationNotes`, in the build steps after `applyDocNotes`) refuses one whose node the schema lacks or
  does not mark: "deprecation note <node>: the schema does not mark it deprecated". It is stricter than a doc note, as question 10
  decided; the MADR's correction of 2026-10-20 records the difference.

**The engine harness.**

* **`enginetest.Deprecations(dir, keep)`** lists every declaration of the generated files whose doc comment has a paragraph starting
  `Deprecated: `, one line each, sorted: `<kind> <name>: <the paragraph, its lines joined by spaces>`. The kinds are `type`, `const`,
  `func`, `field T.F`, `method T.M` for an interface's, and `method (*T).M` for a concrete type's. Its own test, `TestDeprecations`,
  reads a small package with one notice of each kind, and one of each of D10's ignored forms, which it must not list.
* **`runEngineCase`** compares it with an optional `want.deprecated`. An accept case without one must list none, so a notice where
  none is due fails each of the existing cases. `-update` writes the section when the list is not empty.

**Cases,** in `cmd/generate/testdata/engine/`:

* **`accept-deprecated.txtar`:**
  * a deprecated object definition;
  * a deprecated request definition bound to a method, and its response not deprecated;
  * a deprecated property, and a deprecated inline object property;
  * a oneOf union with a deprecated discriminated variant and a deprecated titled variant, with their helpers;
  * a string-constant union with a deprecated constant;
  * `deprecated: false` on a definition and a property, which marks nothing;
  * a property whose `$ref` names a deprecated definition: the type has its notice, and the field none.
* **`refuse-deprecated-string.txtar`** and **`refuse-deprecated-null.txtar`:** each names the definition and `deprecated`.
* **`refuse-deprecated-placement.txtar`:** `deprecated: true` on an array's `items`, refused with its path.

**Model tests:** `TestDeprecationNotes`, as `TestDocNotes` (`patch_test.go:211`) sets a table's notes. It covers a note's text after
the sentence, and the refusals of `checkDeprecationNotes` and of a note on an unmarked or absent node.

### Hand-written notices (question 13)

**`TestDeprecationNotices`,** in a new root `deprecation_test.go`, parses every non-test `.go` file of the root that `ast.IsGenerated`
does not report. In each comment group:

* **A line whose text starts with "deprecated"**, in any case, must start with `Deprecated: ` and begin a paragraph: the group's first
  line, or one after a blank `//` line.
* **Any other line holding "deprecated:"**, in any case, fails.

Together the two rules refuse each of D10's four ignored forms. The generated files are left to the engine cases: their descriptions
are upstream's prose.

## Implementation Steps

Each step ends with its gates and an execution record. Agents stage; a commit or a push on `main` needs the owner's explicit
permission in that turn (`AGENTS.md`, "Commits"). Every new test or check is seen to fail on a planted break in a scratch copy or a
scratch clone, never in the tree, before it is relied on.

1. **Records:**
   * this PLAN and its row `in-progress`;
   * the facts above re-asserted by a script.
2. **The gate:**
   * the CI step;
   * `GORELEASE_PIN` and `make api`;
   * `RELEASING.md`'s two sections;
   * `docs/architecture.md`'s checks;
   * the gate's planted breaks.
3. **The keyword:**
   * `load.Definition.Deprecated`, the `null` refusal, the registry;
   * `checkDeprecated`;
   * the three refuse cases;
   * 0012's annotation;
   * `docs/architecture.md`'s keyword paragraph.

   At this step `deprecated: true` at a supported place is accepted and marks nothing yet. Step 4 follows before the push, so no
   release carries the gap.
4. **The notices:**
   * the model's `Deprecation` fields and their setting;
   * `DeprecationNotes` with `checkDeprecationNotes` and `applyDeprecationNotes`;
   * the emitter's notices;
   * `enginetest.Deprecations` and its test, `TestDeprecations`, and `want.deprecated` in `runEngineCase`;
   * `accept-deprecated.txtar`;
   * `TestDeprecationNotes`;
   * regeneration, byte-identical;
   * `docs/architecture.md`'s engine-case paragraph.
5. **Hand-written notices:** `TestDeprecationNotices`.
6. **Release notes:** `docs/reports/0024-REPORT-release-notes-v1-17-1.md` and its row, its API facts from `make api` on step 5's
   commit, as the new `RELEASING.md` says.
7. **Verification and close:**
   * the gates over the tree;
   * `make api` and `make push-guard` before the push;
   * after CI's six jobs pass on the pushed commits, with the gate step's log naming the inferred base and "Suggested version", this
     PLAN and its row become `complete`;
   * the owner tags `v1.17.1` and publishes its release.

## Verification

Per code step:

* phase 1's gates on go1.27.2: regeneration, `make pre-add-check`, `GOEXPERIMENT=nojsonv2`, `GOOS=windows go vet`,
  `make check-records`, `make markdownlint`, the identifier scan;
* the API: from step 2 on, `make api` exits 0 with no change against `v1.17.0`, in a scratch clone of the committed history with the
  step's changes committed there, since it refuses an uncommitted tree (R6);
* `go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12` clean, for step 2;
* the go1.26.9 gate;
* Windows over SSH, before the push.

Planted breaks, at least. Each must be seen to fail on the check named.

| For | Break, in a scratch clone or copy | The check that must fail |
| :--- | :--- | :--- |
| gate | an exported method removed, committed | `make api` exits 1, "incompatible changes" |
| gate | a method added to `Agent`, committed | `make api` exits 1, "Agent.<name>: added" |
| gate | an exported function added, committed, with `TAG=v1.17.1` | `make api` exits 1, "minor version is not incremented" |
| gate | the pin removed from `ci.yml` | `make api` exits 1, "no gorelease pin" |
| gate | the CI step's `if:` or `run:` mistyped | `actionlint` |
| keyword | the registry's `deprecated` entry removed | `TestKeywordRegistryMatchesLoader` |
| keyword | the `null` refusal removed | `refuse-deprecated-null` |
| keyword | `checkDeprecated` not called | `refuse-deprecated-placement` |
| notices | the type's notice not emitted | `accept-deprecated`, its `want.deprecated` |
| notices | the helper's notice not emitted | `accept-deprecated` |
| notices | the method's mark not taken from its request definition | `accept-deprecated` |
| notices | the notice joined to the paragraph before it | `accept-deprecated` |
| notices | every type marked deprecated | every accept case without `want.deprecated` |
| notices | a note on an unmarked node accepted | `TestDeprecationNotes` |
| notices | `enginetest.Deprecations` returning nothing | `TestDeprecations` in `internal/enginetest`, and `accept-deprecated` |
| Q13 | each of D10's four ignored forms, one at a time, in `errors.go` | `TestDeprecationNotices` |
| Q13 | the paragraph form in `errors.go`, the control | `TestDeprecationNotices` must pass |

The CI step itself cannot be planted without pushing a break. Its command is `make api`'s, from the same pin, and its first run on
the pushed commits is read in step 7.

Annotated 2026-10-20, step 2: where the table says `make api` exits 1, it exits 2, Make's status when a recipe fails; `gorelease`
itself exits 1.

Expected: `gorelease` against `v1.17.0` reports no change, and the generated files are byte-identical to `f6f9d73`'s.

Acceptance:

* every step's checks pass and its record is written;
* every claim the new docs make (`RELEASING.md`, `docs/architecture.md`, the code's comments) is mapped to a test or a check that
  makes it;
* `make lint` reports 0 issues and `go fix -diff` prints nothing;
* CI's six jobs pass on the pushed commits, and the gate step ran;
* `make api` reports no change against `v1.17.0`.

## Rollout and Rollback

Each step is one commit, reverted by reverting it.

* **Step 2** adds the CI step. Reverting it removes the gate.
* **Steps 3 and 4** change only the generator. With nothing deprecated in the committed schema, no generated byte changes, so neither
  reverting them nor landing them changes the SDK.
* **Step 5** adds a test.

There is no behaviour change for a consumer. What changes is for maintainers:

* a push that makes an incompatible change fails CI;
* a schema update that deprecates something regenerates with notices, where it stopped before (D5).

The release is proposed as the patch `v1.17.1`, which the owner confirms at the tag.

**Points for the owner's review**, each with the plan's choice:

* **Placements beyond the three are refused,** naming the path: a union definition's own properties, an inline variant's
  properties, `items`, `additionalProperties`, `allOf` members, `not`, `unevaluatedProperties`, and `null` variants. None makes a
  declaration the table names. Question 9 decided the three; a later schema that marks one of the others stops `make version` until a
  record extends the table.
* **A property whose type is a deprecated definition** gets no notice of its own. Its type's notice warns its users, through SA1019 on
  the type (D11).
* **The table's deprecation notes are tested in the model,** not by an engine case. The engine cases run with the `ACP` table, whose
  notes are empty.
* **The root test covers the root's hand-written files,** not `example/` or `cmd/generate`, where nothing is exported to consumers.
* **The CI step joins the existing job,** rather than a job of its own. That saves a runner, and the step's position keeps the tree
  clean.
* **The README is unchanged.** A consumer-facing note on compatibility and deprecations is left to a later record.
* **`v1.17.1`, a patch:** nothing a consumer can see changes.

## Execution record

### Step 1, records, 2026-10-20

The owner committed this PLAN as `d078835` and approved it ("committed, proceed"). This PLAN and its row are `in-progress`.

**The facts,** re-asserted on `d078835` by a script outside the tree: 76 of 76 hold. It checks each line this PLAN cites by its
content, and these counts:

* three steps in `ci.yml` run only on Linux at `1.27.2`, and `ci.yml` names no `gorelease`;
* the `Makefile` has no `api` target, and `load.go`, `keywords.go` and the model name no `deprecated` or `Deprecation`;
* 13 calls of `docLines` on a description, and a method's doc taken from its notification, request or response definition in that
  order;
* the five hand-written and five generated non-test root files;
* `git grep -i deprecated -- '*.go' '*.txtar'` exits 1;
* `scripts/check_records.py --next` gives `0025`.

**The script, planted.** In a scratch copy of `d078835`, with `"deprecated": TreatAnnotation` added to the registry, it exits 1:
"MISS keywords.go: no deprecated", 75 of 76.

Gates: `make check-records`, `make markdownlint` and the identifier scan, each exit 0.

### Step 2, the gate, 2026-10-20

The owner committed step 1 as `1545949` and approved this step. What changed:

* **`ci.yml`:** the step "API compatibility (gorelease)", directly after `go mod download`, on Linux at `1.27.2` only, running
  `go run golang.org/x/exp/cmd/gorelease@v0.0.0-20261014143641-f0251eb4aa3e` with no flags, with its comment.
* **`Makefile`:** `GORELEASE_PIN`, read from `ci.yml` beside `GOVULNCHECK_PIN`; `api` on the `.PHONY` line; the `api` target, after
  `vuln`, with its comment. `make help` lists it under GNU Make 3.81 and 4.4.1.
* **`RELEASING.md`:** "Tag and Publish" gains `make api TAG=vX.Y.Z` as its first step, and its tag command becomes
  `git tag -a vX.Y.Z -m vX.Y.Z`. "Release notes and the API" says the notes record `make api`'s report, and that CI runs it on each
  push and pull request.
* **`docs/architecture.md`:** the checks gain `make api` and CI's step.

**The docs' claims, each to what makes it:**

| Claim | Check |
| :--- | :--- |
| a change `gorelease` calls incompatible fails: a removed declaration, a method added to an interface | the plants "an exported method removed" and "a method added to `Agent`" |
| a tag that does not follow from the changes fails | the plant "an addition with `TAG=v1.17.1`" |
| the version CI pins | the plant "the pin removed from `ci.yml`" |
| it needs nothing uncommitted | the plant "an uncommitted file" |
| it needs the network | the MADR's R7 |
| CI runs it on each push and pull request | `ci.yml`'s `on:` (`push` to `main`, `pull_request`), and the step's job runs for each |

**Planted breaks.** Each ran in a scratch clone of the committed history with this step's changes committed on top, and its break
committed there, under GNU Make 3.81 (the system's `make`) and 4.4.1 (`gmake`). 11 of 11 under each were as required:

| Break or control | Result |
| :--- | :--- |
| control: no API change | exit 0, "Suggested version: v1.17.1" |
| `(*ClientSideConnection).UnstableDidCloseDocument` removed | exit 2, "## incompatible changes", "(*ClientSideConnection).UnstableDidCloseDocument: removed" |
| `ProbeMethod` added to `Agent` | exit 2, "Agent.ProbeMethod: added" |
| an exported function added, `TAG=v1.17.1` | exit 2, "minor version is not incremented" |
| control: the same, `TAG=v1.18.0` | exit 0, "v1.18.0 is a valid semantic version" |
| control: the same, no `TAG` | exit 0, "ProbeAdded: added" |
| the pin in `ci.yml` replaced by `@latest` | exit 2, "no gorelease pin in .github/workflows/ci.yml" |
| an untracked file | exit 2, "has uncommitted changes" |
| control: `actionlint` on the new `ci.yml` | exit 0 |
| the step's `if:` naming `matrix.flor` | `actionlint` exit 1: "property "flor" is not defined in object type" |
| the step's `run:` spelled `runs:` | `actionlint` exit 1: "step must run script with "run" section or run action with "uses" section" |

**Gates** on the working tree, go1.27.2, each exit 0:

* regeneration: the generated files unchanged;
* `make pre-add-check`: "146 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck)";
* `GOEXPERIMENT=nojsonv2 go test ./...`, and `GOOS=windows go vet ./...` in both modules;
* `make check-records`, `make markdownlint` and the identifier scan;
* the API listing against `v1.17.0`: 2429 lines at the tag and in the tree, 0 removed, 0 added; and `make api` in the plants' control
  clone, no change;
* `actionlint` v1.7.12 on the tree.

**The go1.26.9 gate:** `go version` reports go1.26.9; `go vet` and `go test -race -count=1` pass in both modules, the root's with the
leak guard.
