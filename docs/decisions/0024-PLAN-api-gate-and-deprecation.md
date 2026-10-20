---
status: complete
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

  Annotated 2026-10-20, step 3: the refusal on "a variant the model skips" is not written. The model skips only a JSON `null`
  entry of a union's list (`model.go:748`), which holds no keyword. A `{"type": "null"}` variant is not skipped; it is a union
  field like any other (`emit/types.go:1009`).

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

Annotated 2026-10-20 by the deviation during step 4: a direct `anyOf` or `oneOf` member that becomes a flattened member
marks its field (`Member.Deprecation`), and an enum kind's member with no `const`, from which no declaration is built, is
refused.

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
* **`refuse-deprecated-catch-all.txtar`,** added by the deviation during step 4: `deprecated: true` on an open enum's member with
  no `const`, refused, naming the definition and the member.
* **`refuse-deprecated-under-not.txtar`,** added in step 3: `deprecated: true` inside a definition's `not`, which `checkTypable`
  does not walk and `checkDeprecated` must.

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

### Step 3, the keyword, 2026-10-20

The owner committed step 2 as `803c77b` and approved this step. What changed:

* **`internal/load/load.go`:** `Definition.Deprecated` (`json:"deprecated"`), documented; `refuseNull`, called by
  `Definition.UnmarshalJSON` after the strict decode, refuses `"deprecated": null`.
* **`internal/model/keywords.go`:** `"deprecated": TreatAnnotation`.
* **`internal/model/schema.go`:** `childPlace`, the placement rule `checkTypable` held inline, now shared; `checkDeprecated`, called
  by `checkRefs` for each definition, refuses `deprecated: true` at `placeOther`, and walks `not` and `unevaluatedProperties`, which
  `checkTypable` skips. `checkRefs`'s doc names it.
* **Engine cases:** `refuse-deprecated-string`, `refuse-deprecated-null`, `refuse-deprecated-placement`, and
  `refuse-deprecated-under-not`, added beyond the three this PLAN lists, for `checkDeprecated`'s walk of `not`.
* **`internal/load/load_test.go`:** `TestDeepCopyDefinitionCoversEveryField`'s fixture sets `Deprecated`. The test requires every
  field of `Definition` set, and failed without it in the first `make pre-add-check` run: "the fixture leaves Deprecated zero; set it
  so its copy is checked". `deepCopyDefinition` already copies the field (`copyDef := *d`, `merge.go:455`).
* **Docs:** 0012's keyword table is annotated with the row; `docs/architecture.md`'s keyword paragraph names the refusals.

**The refusals, as the generator prints them,** the same on go1.27.2's default JSON backend and with `GOEXPERIMENT=nojsonv2`, exit 1
each:

| Case | Error |
| :--- | :--- |
| a string | "parse schema.json: definition "T": json: cannot unmarshal string into Go struct field alias.deprecated of type bool" |
| `null` on a property | "parse schema.json: definition "T": deprecated is null; it must be true or false" |
| on `items` | "model: T.properties.a.items: deprecated here marks no Go declaration; only a definition, a property of an object definition, or a direct anyOf or oneOf member of a definition can be deprecated" |
| inside `not` | "model: T.not: deprecated here marks no Go declaration; …" |

**Accepted, and marking nothing yet.** In a scratch copy, `deprecated: true` on `McpServerSse`, `McpCapabilities.sse` and
`McpServer`'s `sse` variant, in both `schema.json` and `schema.unstable.json`: the generator exits 0, and the five generated files are
byte-identical. Step 4 adds the notices before the push.

**Found: a deprecation in one schema file alone renames unstable types.** The same marks in `schema.json` only changed
`types_gen.go` and `helpers_gen.go`: `UnstableMcpServer`'s `sse` variant type became `UnstableMcpServerSseInline`, where it was
`UnstableMcpServerSse`. The merge copies an unstable definition that differs from the stable one, by `reflect.DeepEqual`
(`internal/load/merge.go:211`, `:329`), so `Deprecated` takes part as `Description` does; the unstable `McpServerSse` then needs its
own name, and the variant's moves aside. Upstream writes both files from one Rust source (D3), so both carry a mark. Were they ever to
differ, the rename is an incompatible change, and step 2's gate fails on it.

**The claims, each to what makes it:**

| Claim | Check |
| :--- | :--- |
| a value that is not a boolean is refused | `refuse-deprecated-string` |
| `null` is refused | `refuse-deprecated-null` |
| refused on a node no Go declaration is built from | `refuse-deprecated-placement` |
| `checkDeprecated` walks `not` and `unevaluatedProperties` | `refuse-deprecated-under-not` |
| accepted on a definition, a property or a variant | the scratch probe above; step 4's `accept-deprecated` |
| `false` is the same as absent | step 4's `accept-deprecated` |
| the registry lists the keyword | `TestKeywordRegistryMatchesLoader` |

**Planted breaks.** Each ran in its own scratch copy of the working tree. All 4 failed the check named:

| Break | The check that failed |
| :--- | :--- |
| the registry's `deprecated` entry removed | `TestKeywordRegistryMatchesLoader`: the registry's list lacks `deprecated`, `load.Definition`'s has it |
| `refuseNull`'s call removed | `refuse-deprecated-null`: "run accepted the schema" |
| `checkDeprecated`'s call removed | `refuse-deprecated-placement`: "run accepted the schema" |
| `checkDeprecated` skipping `not` and `unevaluatedProperties` | `refuse-deprecated-under-not`: "run accepted the schema" |

The `null` plant was first expected to fail with the refusal's own text. It fails with "run accepted the schema": without the
refusal, `null` decodes as `false`. The runner's expectation was corrected, and the plant run again.

**Gates** on the working tree, go1.27.2, each exit 0:

* regeneration: the generated files unchanged;
* `make pre-add-check`: "146 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck)", on its second run, after the fixture;
* `GOEXPERIMENT=nojsonv2 go test ./...`, and `GOOS=windows go vet ./...` in both modules;
* `make check-records`, `make markdownlint` and the identifier scan;
* the API listing against `v1.17.0`: 2429 lines each, 0 removed, 0 added;
* `make api` in a scratch clone with this step committed (10 files): "Inferred base version: v1.17.0", "Suggested version: v1.17.1".

**The go1.26.9 gate:** `go version` reports go1.26.9; `go vet` and `go test -race -count=1` pass in both modules, the root's with the
leak guard.

### Deviation, 2026-10-20, during step 4: flattened members and enum members with no `const`

**Found,** before step 4's first change. Step 3's `checkDeprecated` accepts `deprecated: true` on every direct `anyOf` or `oneOf`
member of a definition. Two kinds of such members are not in this PLAN's table:

* **Flattened members.** A definition with properties of its own and an `anyOf` or `oneOf` whose members are all references is an
  object; each member becomes a pointer field of the struct (`Member`, `model.go:1052`), not a union's field. In the committed schema:
  `ElicitationFormMode`, `ElicitationUrlMode` and `SessionConfigOption`, two members each, in both schema files.
* **Enum members with no `const`.** An enum kind's member without a `const`, such as an open enum's string catch-all, makes no Go
  declaration: `enumValues` and the integer enum's loop skip it (`model.go:509`, `:568`). In the committed schema: `CompactionStatus`,
  `ErrorCode`, `NoticeSeverity` and `SessionConfigOptionCategory`, one each, and `LlmProtocol` in the unstable file.

Left as it was, a mark on either would be accepted and give no notice, against question 9's answer and this PLAN's "refused where no
declaration is made".

**Decision (owner, 2026-10-20), the recommended resolution.**

* A flattened member's field carries the notice, as a union's field does: `Member.Deprecation`, and the field's place, "the <field>
  member of the <definition> schema definition".
* An enum kind's member with no `const` is refused in the model, naming the definition and the member: no declaration is built from
  it.

**Files added to step 4's scope:** none beyond those named. `accept-deprecated.txtar` gains a flattened member, and
`refuse-deprecated-catch-all.txtar` is new. The MADR's specification table is amended the same day.

### Step 4, the notices, 2026-10-20

The owner committed step 3 as `45fa905` and approved this step. Its deviation, above, was decided before the first change. What
changed:

* **The model** (`internal/model`):
  * `Deprecation {Note, Definition}`, and `deprecationOf`, which marks a node whose `deprecated` is true;
  * a `Deprecation` field on `Type`, `Field`, `Struct`, `Variant`, `EnumValue`, `Member` and `Method`, set where each is built:
    `buildType`, `objectFields`, `buildObject`'s nested structs, `buildVariant`, the enum and integer enum values, and
    `flatMembers`;
  * `refuseConstless`, called for the four enum kinds, refuses a marked member without a `const` (the deviation);
  * the protocol table's `DeprecationNotes []DeprecationNote`, empty in `ACP`; `checkDeprecationNotes`, beside `checkDocNotes`;
    and `applyDeprecations`, a build step after `applyDocNotes`, which refuses a note on a node the schema lacks or does not mark,
    gives a note to each declaration of its node, and then marks each method whose notification or request definition is marked,
    naming the definition.
* **The emitter** (`internal/emit`): `withNotice`, which adds "Deprecated: the ACP schema marks <place> deprecated.", then the note,
  wrapped by `wrapDoc`, as the doc's last paragraph after a blank line, without changing the doc it is given; and `methodNotice`. They
  are applied to a definition's type and a union's wrapper, a nested struct, a struct field, a flattened member's field, a union's
  field, an inline variant type, a helper, a constant, an interface method and a side connection's method. `enumConsts` now takes the
  type, and `enumSpec` and `emitInlineVariant` the place and the mark.
* **The harness:** `enginetest.Deprecations` and `TestDeprecations` (`internal/enginetest/deprecations.go`, new);
  `runEngineCase` compares `want.deprecated`, none when absent, and `-update` writes it.
* **Cases and tests:** `accept-deprecated.txtar`; `refuse-deprecated-catch-all.txtar` (the deviation); `TestDeprecationNotes`
  (`internal/model/deprecation_test.go`) and `TestWithNotice` (`internal/emit/deprecation_test.go`), both new.
* **Docs:** `docs/architecture.md`'s keyword paragraph names the notices, its engine-case bullet `want.deprecated`, and its tree
  line the listing.

**`accept-deprecated`'s notices,** as `enginetest.Deprecations` lists them:

```text
  const LevelLow: Deprecated: the ACP schema marks the "low" variant of the Level schema definition deprecated.
  field Holder.A: Deprecated: the ACP schema marks the "a" property of the Holder schema definition deprecated.
  field Holder.Inner: Deprecated: the ACP schema marks the "inner" property of the Holder schema definition deprecated.
  field Mode.Session: Deprecated: the ACP schema marks the Session member of the Mode schema definition deprecated.
  field U.A: Deprecated: the ACP schema marks the "a" variant of the U schema definition deprecated.
  field V.Text: Deprecated: the ACP schema marks the Text variant of the V schema definition deprecated.
  func NewUA: Deprecated: the ACP schema marks the "a" variant of the U schema definition deprecated.
  func NewVText: Deprecated: the ACP schema marks the Text variant of the V schema definition deprecated.
  method (*ClientSideConnection).Init: Deprecated: the ACP schema marks the InitRequest schema definition deprecated.
  method Agent.Init: Deprecated: the ACP schema marks the InitRequest schema definition deprecated.
  type HolderInner: Deprecated: the ACP schema marks the "inner" property of the Holder schema definition deprecated.
  type InitRequest: Deprecated: the ACP schema marks the InitRequest schema definition deprecated.
  type Old: Deprecated: the ACP schema marks the Old schema definition deprecated.
  type UA: Deprecated: the ACP schema marks the "a" variant of the U schema definition deprecated.
  type VText: Deprecated: the ACP schema marks the Text variant of the V schema definition deprecated.
```

`Holder.Old`, whose `$ref` names the deprecated `Old`, `Holder.Keep` and `Kept`, marked `false`, the response `InitResponse`, and
`U`'s `b` variant have none. The regenerated files of the committed schema are byte-identical.

**End to end.** In a scratch copy, `deprecated: true` on `McpServerSse`, `McpCapabilities.sse` and `McpServer`'s `sse` variant, in
both schema files, regenerated:

* the notices land on `McpServerSse`, `McpCapabilities.Sse`, `McpServerSseInline`, `McpServer.Sse` and `NewMcpServerSse`, and on
  the unstable `UnstableMcpServerSse`, `UnstableMcpServer.Sse` and `NewUnstableMcpServerSse`;
* `golangci-lint` with the repository's config reports 0 issues at the root for both `GOOS` values: the package's own uses;
* a package of the module that uses the three gets SA1019 for each, quoting the sentence: "github.com/maccavelli/acp-go-sdk2.McpServerSse
  is deprecated: the ACP schema marks the McpServerSse schema definition deprecated.", and the same for `(McpCapabilities).Sse` and
  `NewMcpServerSse`.

**Found while linting, fixed before the gates passed:**

* gocritic's `deprecatedComment` check reported `DeprecationNote`'s own doc, which wrapped so a line began "deprecated:": "use
  `Deprecated: ` (note the casing) instead of `deprecated: `". The comment was reworded. The repository's lint already refuses that
  form, so step 5's test is measured against what gocritic already catches.
* gocognit reported `TestDeprecationNotes` at 52, over 50; its reading of the marks moved to `readMarks`.

**A first expectation corrected.** `TestWithNotice` first expected a method's sentence on one line; at 81 characters it wraps, as
`wrapDoc` is meant to. The test's expectation was corrected.

**The claims, each to what makes it:**

| Claim | Check |
| :--- | :--- |
| each declaration of the table gets the notice, with its place | `accept-deprecated`'s `want.deprecated` |
| a flattened member's field gets it; a member without a `const` is refused | `accept-deprecated`; `refuse-deprecated-catch-all` |
| a method is marked by its request definition, naming it, and its response by its own mark only | `accept-deprecated`; `TestDeprecationNotes` |
| a note follows the sentence, on each declaration of its node | `TestDeprecationNotes`; `TestWithNotice` |
| a note on a node the schema lacks or does not mark is refused | `TestDeprecationNotes` |
| the notice is the last paragraph, after a blank line, wrapped at 76 | `TestWithNotice`; `accept-deprecated` |
| `withNotice` does not change the doc it is given | `TestWithNotice` |
| a case without `want.deprecated` has no notice | the plant "every type marked" |
| tools honour the notice | the end-to-end check above |

**Planted breaks.** Each ran in its own scratch copy of the working tree. All 13 failed the check named:

| Break | The check that failed |
| :--- | :--- |
| the type's notice not emitted | `accept-deprecated`: "deprecation notices differ from want.deprecated" |
| the helper's notice not emitted | the same |
| the field's notice not emitted | the same |
| the constant's notice not emitted | the same |
| the flattened member's notice not emitted | the same |
| the method's mark not taken from its request definition | the same |
| the notice joined to the paragraph before it | the same |
| every type marked deprecated | `accept-enum`, which has no `want.deprecated`: the same |
| a note on an unmarked or absent node accepted | `TestDeprecationNotes`: "an unmarked definition: Build = <nil>, want an error containing …" |
| `enginetest.Deprecations` listing nothing | `TestDeprecations`, and, separately, `accept-deprecated` |
| `refuseConstless` disabled | `refuse-deprecated-catch-all`: "run accepted the schema" |
| `withNotice` appending to the doc it is given | `TestWithNotice`: "the doc given was changed: the first notice is now … the B schema definition …" |

The type's plant was first expected to print a diff line; the test prints the two lists instead. It failed with "deprecation notices
differ from want.deprecated", `type Old` and `type InitRequest` only in the want list. The runner's expectation was corrected and the
plant run again.

**Gates** on the working tree, go1.27.2, each exit 0:

* regeneration: the generated files unchanged;
* `make pre-add-check`: "146 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck)"; and the four new Go files alone:
  "4 file(s) clean";
* `GOEXPERIMENT=nojsonv2 go test ./...`, and `GOOS=windows go vet ./...` in both modules;
* `make check-records`, `make markdownlint` and the identifier scan;
* the API listing against `v1.17.0`: 2429 lines each, 0 removed, 0 added;
* `make api` in a scratch clone with this step committed (17 files): "Inferred base version: v1.17.0", "Suggested version: v1.17.1".

**The go1.26.9 gate**, on the final tree: `go version` reports go1.26.9; `go vet` and `go test -race -count=1` pass in both modules,
the root's with the leak guard.

### Deviation, 2026-10-20, during step 5: the lint already reports three of the four forms

**Found,** before step 5's test was written. In a scratch copy of `61ac81e`, each of D10's forms alone on a new function in
`errors.go`, `golangci-lint` with the repository's config over the root:

| Form | Result |
| :--- | :--- |
| the paragraph, the control | exit 0, no report |
| no blank line before it | exit 1, gocritic `deprecatedComment`: "`Deprecated: ` notices should be in a dedicated paragraph, separated from the rest" |
| mid-line | exit 0, no report |
| lower case | exit 1, gocritic `deprecatedComment`: "use `Deprecated: ` (note the casing) instead of `deprecated: `" |
| a comma | exit 1, gocritic `deprecatedComment`: "use `:` instead of `,` in `Deprecated, `" |

Question 13's alternative was put as "review alone"; `make lint` and CI's lint step already refuse three of the four forms.

**Decision (owner, 2026-10-20), the recommended resolution.** Keep `TestDeprecationNotices` as planned. It refuses all four in
`go test`, whatever the lint configuration, and it is the only check on the mid-line form. Its doc names the overlap.

### Step 5, hand-written notices, 2026-10-20

The owner committed step 4 as `61ac81e`, on the owner's permission in that turn, and approved this step. Its deviation, above, was
decided before the test was written. What changed:

* **`deprecation_test.go`, new:** `TestDeprecationNotices` parses the root's non-test files that `ast.IsGenerated` does not report:
  `connection.go`, `doc.go`, `errors.go`, `extensions.go` and `helpers.go`. In each comment, a line whose text starts with
  "deprecated", in any case, must start with "Deprecated: " and begin a paragraph, and no other line may hold "deprecated:", in any
  case. A block comment is read line by line. Its doc names gocritic's overlap.
* **`docs/architecture.md`:** the checks list the test, and the one form only it reports.

It passes on the tree: no hand-written file holds a notice.

**The claims, each to what makes it:**

| Claim | Check |
| :--- | :--- |
| a notice in each of D10's four ignored forms fails | the four plants below |
| the paragraph form passes | the control below |
| a block comment is read too | the block comment plant |
| the generated files are left to the engine cases | the generated-file control |
| gocritic reports three of the four; the mid-line form only this test | the deviation's probe, above |

**Planted breaks and controls.** Each ran in its own scratch copy of the working tree, its text appended to the file named. 7 of 7
as required:

| Plant | Result |
| :--- | :--- |
| `errors.go`: "Deprecated: use X." with no blank line before it | exit 1: "a notice must begin a paragraph, after a blank comment line: Deprecated: use X." |
| `errors.go`: the notice mid-line | exit 1: "a notice must begin its own line and paragraph: Probe is a probe. Deprecated: use X." |
| `errors.go`: "deprecated: use X." | exit 1: "a notice must start with "Deprecated: ": deprecated: use X." |
| `errors.go`: "Deprecated, use X." | exit 1: "a notice must start with "Deprecated: ": Deprecated, use X." |
| `errors.go`: a block comment with the notice mid-line | exit 1: "a notice must begin its own line and paragraph" |
| control: `errors.go`, the paragraph form | exit 0 |
| control: `types_gen.go`, the notice mid-line | exit 0: the generated file is not read |

**Gates** on the working tree, go1.27.2, each exit 0:

* regeneration: the generated files unchanged;
* `make pre-add-check`: "150 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck)"; and the new file alone: "1 file(s)
  clean";
* `GOEXPERIMENT=nojsonv2 go test ./...`, and `GOOS=windows go vet ./...` in both modules;
* `make check-records`, `make markdownlint` and the identifier scan;
* the API listing against `v1.17.0`: 2429 lines each, 0 removed, 0 added;
* `make api` in a scratch clone with this step committed: "Inferred base version: v1.17.0", "Suggested version: v1.17.1".

**The go1.26.9 gate:** `go version` reports go1.26.9; `go vet` and `go test -race -count=1` pass in both modules, the root's with the
leak guard.

### Step 6, release notes, 2026-10-20

The owner committed step 5 as `0a9b5c1`, on the owner's permission in that turn, and approved this step. It adds
[0024-REPORT-release-notes-v1-17-1.md](../reports/0024-REPORT-release-notes-v1-17-1.md) and its row in `docs/README.md`.

Its facts, each run on `0a9b5c1` with nothing uncommitted:

* `make api`, in the repository: exit 0, "Inferred base version: v1.17.0", "Suggested version: v1.17.1"; with `TAG=v1.17.1`, exit 0,
  "v1.17.1 is a valid semantic version for this release"; with `TAG=v1.17.0`, exit 2, "version v1.17.0 already exists". `git status`
  was empty before and after;
* `git diff v1.17.0 0a9b5c1 -- schema/` is empty;
* `git diff --name-only v1.17.0 0a9b5c1 -- '*.go' ':!*_test.go'` lists eleven files, all in `cmd/generate`;
* the root's non-test `.go` files, `go.mod` and `README.md`: `git diff --quiet v1.17.0 0a9b5c1` exits 0;
* the eight commits since `v1.17.0` and their files, from `git show --name-only`.

**A point for the owner.** The package's code is the same as at `v1.17.0`. [0023-PLAN-connection-lifetime-and-test-leaks.md](0023-PLAN-connection-lifetime-and-test-leaks.md),
step 6, made no release of its own for a change with no SDK code ("No SDK code changes, so no release of its own"). This PLAN
proposes `v1.17.1`, which the owner confirms at the tag, or leaves the notes for the next release.

Gates: `make check-records`, `make markdownlint` and the identifier scan, each exit 0.

### Step 7, verification and close, 2026-10-20

The owner committed step 6 as `1d8aa96`.

**The gates over the tree**, on `1d8aa96`, go1.27.2, each exit 0:

* regeneration: the generated files unchanged;
* `make pre-add-check`: "151 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck)";
* `GOEXPERIMENT=nojsonv2 go test ./...`, and `GOOS=windows go vet ./...` in both modules;
* `make check-records`, `make markdownlint` and the identifier scan;
* the API listing against `v1.17.0`: 2429 lines each, 0 removed, 0 added;
* `actionlint` v1.7.12 on the tree; `go fix -diff ./...` in both modules: no output;
* the go1.26.9 gate: `go version` reports go1.26.9; `go vet` and `go test -race -count=1` pass in both modules, the root's with the
  leak guard;
* Windows over SSH, before the push, `go test -count=1 -shuffle=on ./...` in both modules: root exit 0, `cmd/generate` exit 0.

**The push.**

* `make api` on `1d8aa96`, in the repository with nothing uncommitted: exit 0, "Inferred base version: v1.17.0", "Suggested
  version: v1.17.1"; with `TAG=v1.17.1`, exit 0, "v1.17.1 is a valid semantic version for this release".
* `git fetch`, then `make push-guard` over the eight commits not yet pushed, `b463216..1d8aa96`: "the guard passed (exit 0)". The
  owner pushed `1d8aa96`.
* **CI run `39454425664` on `1d8aa96`** concluded `success` on its first attempt, in all six jobs: `ubuntu-24.04`, `macos-15` and
  `windows-2025`, each with `1.26.9` and `1.27.2`. Its only annotation, on the two `macos-15` jobs, is GitHub's notice on macOS arm64
  queue times.
* **The gate step's first run in CI,** on `validate (ubuntu-24.04, 1.27.2)`, step 6, after `go mod download` and before the tests:
  `success`, in about 3.4 s. Its log: "go: downloading golang.org/x/exp v0.0.0-20261014143641-f0251eb4aa3e", "Inferred base version:
  v1.17.0", "Suggested version: v1.17.1". The step is `skipped` on the other five jobs.

**Acceptance.** Every criterion in "Verification" is met:

* **Each step's checks** passed and its record is written, with the deviations during steps 4 and 5 and the annotations of steps 2
  and 3.
* **The planted breaks** were each seen to fail on the check named: 11 of 11 under each of GNU Make 3.81 and 4.4.1 in step 2, 4 in
  step 3, 13 in step 4 and 7 in step 5, each in a scratch clone or copy.
* **Every claim the new docs make** maps to a test or a check, in the claim tables of steps 2 to 5.
* **`make lint` reports 0 issues,** through `make pre-add-check`, and `go fix -diff` prints nothing.
* **CI's six jobs passed** on the pushed commits, and the gate step ran.
* **`make api` reports no change** against `v1.17.0`.

This PLAN and its `docs/README.md` row are `complete`. The MADR stays `accepted`; its four phases are complete.

**Left to the owner.**

* Push this close, after `make push-guard`.
* Decide the release, as step 6 put it: tag `v1.17.1` on this close commit once its CI passes, with `make api TAG=v1.17.1` first, as
  `RELEASING.md` now says, and `git tag -a v1.17.1 -m v1.17.1`; or make no release, and keep the notes for the next one.
* If tagged, publish its release from the release text of
  [0024-REPORT-release-notes-v1-17-1.md](../reports/0024-REPORT-release-notes-v1-17-1.md), with its relative links rewritten to the
  tag.

### Deviation, 2026-10-20, after the close: the pre-tag check made the mirror refuse `v1.17.1`

**Found.** After the owner ran `make api TAG=v1.17.1` and pushed the tag, the mirror and the checksum database refused `v1.17.1` for
about 30 minutes: "invalid version: unknown revision v1.17.1". `gorelease -version` asks the mirror for the proposed version before
it exists, and the mirror caches the "not found" (the MADR's R13, R14). The fix was probed: routing this module around the mirror
keeps the check and asks nothing about the proposed version (R15). CI is not affected (R16).

**Decision:** the MADR's question 14, decided by the owner the same day: `apidiff`, as the sibling repositories gate, in
place of `gorelease`. (Written first as: the MADR's question 14, open.) This PLAN reopens, `in-progress`, for step 8 below, and the `v1.17.1` publication record
waits for the release to be published.

### Step 8, the pre-tag check without the mirror (proposed, for question 14)

Superseded the same day by question 14's answer: the step below, "Step 8, respecified", replaces this one.

* **`Makefile`:** `api` sets `GONOPROXY=github.com/maccavelli/acp-go-sdk2` for its run when `TAG` is set, with the reason in its
  comment.
* **`RELEASING.md`:** "Tag and Publish" says why, and that nothing should fetch the new version through the mirror, such as a
  `go get` or a consumer check, before its tag is pushed.
* **Planted breaks,** each with `make api TAG=v1.99.95` in a scratch clone, its `GOPROXY` pointed at the logging server:
  * the `GONOPROXY` removed: the server logs `/@v/v1.99.95.info` at the mirror;
  * as committed: no mirror request for the module, and the checksum database asked only for the base;
  * an addition committed, `TAG=` a patch: exit 2, "minor version is not incremented".
* **Gates:** phase 1's, the go1.26.9 gate, and `make api` in a scratch clone.
* **After CI passes,** the publication record of `v1.17.1`, then this PLAN `complete` again.

### Step 8, respecified: the gate on `apidiff` (proposed, for approval)

The MADR's R17 to R20 hold the probes.

* **`scripts/check_api.py`, new, Python and the standard library,** as `go-llmprovider-sdk`'s:
  * the base is the newest `v1.X.Y` tag, by number, from `git tag --list 'v1.*'`; an argument names another ref;
  * the base's tree comes from `git archive`, in a temporary directory;
  * `apidiff`, pinned in the script at `golang.org/x/exp/cmd/apidiff@v0.0.0-20261014143641-f0251eb4aa3e` and run with `go run`,
    writes the root package's API from each tree (package mode, R18), then compares them;
  * it prints the report, the compatible and incompatible changes, and exits 1 on an incompatible one, 0 otherwise, 2 on a tool error
    or when there is no tag;
  * it reads the working tree, so it needs no commit and no network once the tool is cached.
* **`ci.yml`:** the step "API compatibility (apidiff)" runs `make api`, on Linux at `1.27.2` only. That job's checkout fetches every
  commit and tag, `fetch-depth: ${{ (runner.os == 'Linux' && !matrix.floor) && '0' || '1' }}`, as `go-selfupdate-lib` does; the others
  stay at depth 1. The `gorelease` step goes.
* **`Makefile`:** `api` runs `python3 -B scripts/check_api.py`; `GORELEASE_PIN` and `TAG` go.
* **`RELEASING.md`:** before tagging, `make api` exits 0 on the release commit, and its compatible changes, when there are any, mean
  a minor release. The notes record its report. The tag command stays annotated.
* **`docs/architecture.md`:** the check described as it is.
* **Planted breaks,** each in a scratch clone, with `make api`: a removed method, exit 1; a method added to `Agent`, exit 1; an
  exported function added, exit 0 with it listed; an exported function in `example/agent`, exit 0 and not listed; no `v1` tag (a
  clone without tags), exit 2; the report's incompatible lines not read (the parse removed), the removal passing. And `actionlint` on a
  mistyped `fetch-depth` expression.
* **Gates:** phase 1's, the go1.26.9 gate, and Windows over SSH before the push.
* **After CI passes,** the publication record of `v1.17.1`, then this PLAN `complete` again.

### Step 8, the gate on `apidiff`, 2026-10-20

The owner committed the respecified step as `1dfce5e` and approved it. What changed:

* **`scripts/check_api.py`, new:** as specified. It also refuses, exit 2, a line of `apidiff`'s report it does not recognise, so a
  change in the report's form cannot pass as "no change".
* **`ci.yml`:** the Linux job at `1.27.2` checks out every commit and tag (`fetch-depth: ${{ (runner.os == 'Linux' &&
  !matrix.floor) && '0' || '1' }}`), with the reason in a comment; the step "API compatibility (apidiff)" runs `make api` in place of
  `gorelease`.
* **`Makefile`:** `api` runs `python3 -B scripts/check_api.py $(BASE)`, with its comment; `GORELEASE_PIN` and `TAG` are gone.
* **`RELEASING.md`:** "Tag and Publish" runs `make api` first, and says compatible changes mean a minor; "Release notes and the API"
  records its report. `docs/architecture.md` describes the check.
* `git grep -i gorelease`, outside `docs/decisions` and `docs/reports`, finds nothing (exit 1).

On the tree: `make api` prints "check_api: against v1.17.1: 0 incompatible, 0 compatible", exit 0; `make help` lists "api  Fails on
an incompatible exported API change (apidiff against the newest v1 tag, or BASE)".

**The claims, each to what makes it:**

| Claim | Check |
| :--- | :--- |
| an incompatible change fails: a removed declaration, a method added to an interface | the plants "a method removed" and "a method added to `Agent`" |
| the compatible changes are listed | "an exported function added"; the control `BASE=v1.14.0` |
| the examples' `main` packages are not compared | "an exported function in `example/agent`" |
| it needs the tags | "a clone without tags" |
| `apidiff` exits 0 either way, so the report is read, strictly | the two parse plants |
| `BASE` names another ref | the control `BASE=v1.14.0` |
| it asks the mirror nothing about this module | the MADR's R19 |
| CI's job fetches the tags | `actionlint`, and the step's first run in CI, read at the close |

**Planted breaks and controls.** Each ran in a scratch clone of the committed history with this step's changes applied. 12 of 12 as
required:

| Plant | Result |
| :--- | :--- |
| control: no change | exit 0, "against v1.17.1: 0 incompatible, 0 compatible" |
| control: `make api BASE=v1.14.0` | exit 0, the three compatible changes, "against v1.14.0: 0 incompatible, 3 compatible" |
| `(*ClientSideConnection).UnstableDidCloseDocument` removed | exit 1, "check_api: incompatible: (*ClientSideConnection).UnstableDidCloseDocument: removed"; through `make api`, exit 2 |
| `ProbeMethod` added to `Agent` | exit 1, "check_api: incompatible: Agent.ProbeMethod: added" |
| an exported function added | exit 0, "check_api: compatible: ProbeAdded: added" |
| an exported function in `example/agent` | exit 0, "0 incompatible, 0 compatible", not named |
| a clone at depth 1 without tags | exit 2, "no v1.X.Y tag to compare against; fetch the tags, or pass BASE" |
| the incompatible section's header not recognised, a method removed | exit 2, "apidiff's report has a line this script does not recognise: 'Incompatible changes:'" |
| the exit decision broken, a method removed | exit 0: the removal passes, which the "a method removed" check catches when the script is whole |
| control: `actionlint` on the new `ci.yml` | exit 0 |
| `fetch-depth`'s expression naming `matrix.flor` | `actionlint` exit 1: "property "flor" is not defined in object type" |

**Gates** on the working tree, go1.27.2, each exit 0:

* regeneration: the generated files unchanged;
* `make pre-add-check`: "151 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck)";
* `GOEXPERIMENT=nojsonv2 go test ./...`, and `GOOS=windows go vet ./...` in both modules;
* `make check-records`, `make markdownlint` and the identifier scan;
* the API listing against `v1.17.1`: 2429 lines each, 0 removed, 0 added;
* `actionlint` v1.7.12 on the tree.

**The go1.26.9 gate:** `go version` reports go1.26.9; `go vet` and `go test -race -count=1` pass in both modules, the root's with the
leak guard.

### Step 8, close, 2026-10-20

The owner committed step 8 as `609cb41` and pushed it. Git's pre-push hook ran the disclosure guard on the push.

* **Before the push, not run:** Windows over SSH and `make push-guard`, which this PLAN puts there; the owner pushed first. Windows
  ran after the push, on `609cb41`: `go test -count=1 -shuffle=on ./...` in both modules, root exit 0, `cmd/generate` exit 0.
* **CI run `39464566129` on `609cb41`.**
  * Attempt 1 concluded `failure`. Its two `macos-15` jobs were `cancelled` with no step run: "The job was not acquired by Runner of
    type hosted even after multiple attempts", with GitHub's notice on macOS arm64 queue times. Its four other jobs passed.
  * The owner had the failed jobs rerun (`gh run rerun 39464566129 --failed`). Attempt 2 concluded `success` in all six jobs:
    `ubuntu-24.04`, `macos-15` and `windows-2025`, each with `1.26.9` and `1.27.2`; the two `macos-15` jobs ran at 17:38Z and 17:43Z.
* **The gate's first run in CI,** on `validate (ubuntu-24.04, 1.27.2)`: its checkout with `fetch-depth: 0` fetched 15 tags
  (`* [new tag]` lines), and "API compatibility (apidiff)" ran `python3 -B scripts/check_api.py`: "check_api: against v1.17.1: 0
  incompatible, 0 compatible", `success`. The step is `skipped` on the other five jobs.

**Acceptance.** The criteria of "Verification" hold again with the gate on `apidiff`: step 8's checks passed and its record is written;
its 12 plants and controls were as required; its claims map to checks; `make lint` reports 0 issues; CI's six jobs passed on the pushed
commits, and the gate step ran; `make api` reports no change against `v1.17.1`.

This PLAN and its `docs/README.md` row are `complete` again.

**Left open.** The `v1.17.1` GitHub release is not yet published. Its tag, its CI (run `39456238211`, `success` in all six jobs), the
mirror, the checksum database and a go1.26.9 consumer all hold (the MADR's R14, and the follow-up above). Its publication record, the
nine checks, follows the release; its text describes the `gorelease` gate that `v1.17.1` contains.

### Publication of `v1.17.1`, 2026-10-20

**What was published.**

* **The tag.** The owner ran `make api TAG=v1.17.1`, then tagged `f19d332` `v1.17.1`, an annotated tag whose message is its name, and
  pushed it. CI run `39456238211`, started by the tag, concluded `success` on its first attempt, in all six jobs.
* **The mirror's refusal, and its end.** For about 30 minutes the mirror and the checksum database refused the version, "invalid
  version: unknown revision v1.17.1": the pre-tag check had asked the mirror for it before the tag existed (the MADR's R13, R14). At
  16:56Z both served it. That led to question 14 and step 8.
* **The release.** The owner published the GitHub release `v1.17.1` with `gh release create v1.17.1 --title v1.17.1 --notes-file
  <file>`. Its body is the release text of [0024-REPORT-release-notes-v1-17-1.md](../reports/0024-REPORT-release-notes-v1-17-1.md),
  as a script outside the tree extracted it: the text under "### v1.17.1" up to "Sources not in the release text", with its one
  relative link rewritten to the file at the tag. It describes the `gorelease` gate that `v1.17.1` contains; step 8 replaced it after
  the tag.

**Checks.** The `v1.17.0` publication check, with its tag and commit changed, ran from outside the tree after the release was published,
with a scratch module cache, the default `GOPROXY` and the checksum database on. 9 of 9 hold:

* the release is not a draft or a prerelease, its body is the extracted text, and it is GitHub's latest;
* the tag is annotated and names `f19d332`;
* `main` on GitHub is the local `main`, `9e3f23b`, and CI on it succeeded (run `39472390273`, attempt 2: its first attempt's
  `macos-15, 1.26.9` job was cancelled unrun, "The job was not acquired by Runner of type hosted even after multiple attempts", and
  rerun by the owner);
* the mirror's `@v/v1.17.1.info` gives the origin `refs/tags/v1.17.1` at `f19d332`;
* `go list -m github.com/maccavelli/acp-go-sdk2@latest` gives `v1.17.1`;
* `go mod download` of `v1.17.1` exits 0.

**A consumer on the floor,** run once the mirror served the version: a fresh module at `go 1.26.9`, with `GOTOOLCHAIN=go1.26.9`
forced, the default `GOPROXY` and the checksum database on.

* `go get github.com/maccavelli/acp-go-sdk2@v1.17.1` exited 0: "go: added github.com/maccavelli/acp-go-sdk2 v1.17.1".
* The program, which calls `NewConnection` with a nil handler, built with go1.26.9 and ran, printing "protocol version 1".
* The consumer's `go.mod` kept `go 1.26.9` and gained no `toolchain` line.

Nothing of this PLAN is left open.
