---
status: in-progress
date: 2026-10-26
associated-madr: "0039-MADR-five-adopter-additions-on-schema-1-25-0.md"
---

# Implement 0039: an unstable boundary, presentation and compaction runtimes, an extension mux, and acptest

Associated MADR: [0039-MADR-five-adopter-additions-on-schema-1-25-0.md](0039-MADR-five-adopter-additions-on-schema-1-25-0.md)

## Goal

The MADR's five additions ship, on the schema `1.25.0` pin, with no schema bump:

1. the unstable boundary: `schema/unstable-v1.json`, the frozen contract of every unstable-origin name `v1` exports, from which
   the generator builds them; a merge that no longer grows stable-named types; their `Deprecated:` paragraphs; package `unstable`,
   as question 7 decides; and an API gate that reports `unstable` without failing (*revised 2026-10-25, the MADR's second
   assessment*);
2. `PresentNotice`, `PresentCompaction` and `PresentCompactionSummary`;
3. `CompactionTimeline` and `CompactionEntity`;
4. `ExtensionMux`, `Handle`, `Notify`, `IsNotification`, `TraceMeta` and `AdvertiseMeta`;
5. package `acptest`.

`make api` lists only compatible changes in `acp` and `agentproc`, and `unstable` and `acptest` as new packages.

**Assumed answers.** This PLAN assumes the MADR's recommended answers to its questions 1 to 6, and one answer of its own (P1
below). An answer otherwise amends this PLAN before the phase it changes: question 1 changes phase 4, question 2 the phase order,
questions 3 and 5 phase 2, question 4 this PLAN's shape, question 6 phase 6, and P1 phase 8.

**P1. Releases.** The MADR leaves "each phase a minor or one minor at the end" to this PLAN.

* Recommended: two minors, one after phases 2 and 3 (the boundary and `acptest`), one after phases 4 to 6, so the generator change
  ships and is used before the runtimes that build on it.
* Alternatives: one minor at the end; a minor per phase.

**Answered 2026-10-25.** The owner took the recommended answer to each of the MADR's questions 1 to 6 and to P1, as the MADR's
"Decided with the owner, 2026-10-25" records, so this PLAN needs no amendment for them.

**P1 changed, 2026-10-25, after phase 3.** The owner: "we will do 1 release when done". One minor at the end, after phase 7, carries
all five additions; phase 8 and "Rollout" are amended for it.

**Question 7, answered 2026-10-25.** The MADR's second assessment added question 7, when to build package `unstable`'s
generation. The owner chose "build the emissions now", not the recommendation; 2f was amended for that answer before it starts,
as its first text required.

## Prerequisites

* [0038-PLAN-consumer-helpers-kind-and-agent-refusals.md](0038-PLAN-consumer-helpers-kind-and-agent-refusals.md) is complete
  before phase 2 starts. Phase 3's `ScriptAgent` embeds 0038's `AgentRefusals`, and both PLANs edit `helpers.go`, `README.md` and
  `docs/architecture.md`; running them in turn keeps each commit one step of one PLAN.
* The MADR is `accepted`, with its questions answered.

## Scope

* **Generator, phase 2** (*revised 2026-10-25*): `cmd/generate/internal/load` (provenance for properties and union entries; the
  merge reading the manifest), `cmd/generate/internal/model` (the `Unstable` flag on fields and variants; the manifest as a
  deprecation source), `cmd/generate/internal/emit` (the notice text; `File.Dir`; the `unstable` builders),
  `cmd/generate/main.go` (`-write-unstable-manifest`; the root import path), `cmd/generate/internal/output` (the
  serving importer; two directories; removing a stale generated file), `cmd/generate/internal/enginetest` (the second listing),
  engine cases under `cmd/generate/testdata/engine/`, and the regenerated `*_gen.go` files (*question 7 answered "now"*).
* **New committed data, phase 2:** `schema/unstable-v1.json`, the manifest, holding the frozen fragments; no second file.
* **Example, phase 2:** `example/agent/main.go`, which stops implementing the deprecated unstable methods.
* **New packages:** `unstable/` (phase 2: its hand-written `doc.go`, and its generated files whenever a schema has an unstable
  definition the manifest does not hold, none at `1.25.0`), `acptest/` (phase 3).
* **Root, hand-written:** `helpers.go` (phase 4); `compaction.go`, new (phase 5), with `cmd/generate/engine_test.go`'s
  `TestRuntimeFilesCoverHandWritten` naming it beside `helpers.go` and `refusals.go`; `extensions.go` and `connection.go` (phase 6).
* **Scripts:** `scripts/check_api.py` (phase 2), with tests in `scripts/`.
* **Tests:** each phase's, named below.
* **Docs:** `README.md`, `docs/architecture.md`, `RELEASING.md` (the schema-bump steps for the manifest and frozen stubs).
* **Records:** this pair and their rows; a release-notes report per release.
* **Not in scope:** schema `1.26.0`; protocol v2; `encoding/json/v2`; a transport; elicitation `Check`, `FileHost`, `TerminalHost`,
  `AllowsValue` and `LineTap` (the MADR's next wave); migrating this repository's tests to `acptest`; any change to `Agent` or
  `Client`.

## Implementation Steps

Each phase ends with its checks, an entry in the execution record, and its files staged for the owner, who commits on `main`
(`AGENTS.md`, "Commits"). A phase may be several commits, named in it. Every new check is seen to fail on a planted break, each in
its own copy of the tree, through `tools/plants.py`, or through a scratch script for a nested module, as
[0036-PLAN-session-update-constructors-kind-and-client-refusals.md](0036-PLAN-session-update-constructors-kind-and-client-refusals.md)
step 4 did for `cmd/generate`.

Every code phase's gates:

* `make pre-add-check` on the changed `.go` files;
* `(cd cmd/generate && go test -count=1 ./...)`, which `make pre-add-check` does not run;
* `make check-records`, `make markdownlint`, `make disclosure-check`, `make check-tools` and `make api`;
* `tools/gates.py --all`, with `ACP_LINUX_HOST` and `ACP_WINDOWS_HOST` set;
* `tools/idscan.py --paths` over the staged files.

### 1. Records

* This pair and their rows in `docs/README.md`, both `proposed`.
* The owner answers the MADR's questions 1 to 6 and P1. The MADR records them in a "Decided with the owner" table and becomes
  `accepted`; this PLAN becomes `in-progress` on approval.

Checks: `make check-records`, `make markdownlint`, `make disclosure-check`, `tools/idscan.py --paths` over the three files.

### 2. The unstable boundary (MADR item 1)

*Revision, 2026-10-25, the MADR's second assessment.* The first text of this phase assumed facts the generator does not have: an
origin for every field and variant, a way to deprecate an unmarked declaration, a second output package, and engine cases over
two packages (the MADR's G3 to G5). It also left out `example/agent`, which staticcheck would fail (G6), the two defaults only
the unstable schema changes (G10), and the order of a rebuilt union. It is rewritten from the MADR's "How it is built". Commits
2a to 2f, in order; 2f depends on question 7.

**2a. Inventory and re-measure, read-only.** Recorded in the execution record before 2b changes a file:

* the MADR's G2 and G10, recomputed from the committed schemas by a script in the session's scratch directory that reproduces
  `MergeStableAndUnstable`: the 102 unstable-only definitions, prefixed and not; the 8 replaced definitions and their 18
  differing entries; the 16 unstable methods; and the exported declarations each yields;
* in a scratch copy, the tree regenerated from schema `1.26.0` and `scripts/check_api.py` run against the newest `v1` tag, the
  counts beside 0025's (115 incompatible, 40 stable-named, against `v1.17.1`).

**2b. Provenance for properties and union entries** (`cmd/generate/internal/load`, `cmd/generate/internal/model`).

* `Provenance` (`load/merge.go:12`–`:24`) gains, for each definition `promoteChangedSharedDefs` replaces, the properties the
  unstable version adds, the properties it changes, and the union entries it adds or changes, computed where both versions are
  held (`:329`–`:331`).
* The model's `Field` and `Variant` gain an `Unstable` flag from it, as `Type` and `Method` have.
* Tests: a `load` test over a synthetic pair of schemas for each of the four kinds; a `model` test that, on the committed schemas,
  finds exactly the 10 fields, 2 changed properties and 6 variants of G2 and G10. No generated file changes.

**2c. The manifest, as the frozen contract** (`schema/unstable-v1.json`, the loader, the merge).

*Amended 2026-10-25, before the sub-step's code, by the deviation entry "2c, where a frozen type's schema is read" and the MADR's
question 8.* The bullets below stand, with these changes. Where they conflict, the amendment wins.

* **Scope gains** `instances_test.go` (root), whose `schemaDefs` reads the manifest's `$defs` over `schema.json`'s, and
  `cmd/generate/internal/model/gates.go`, whose `ruleTypes` also accepts a copy whose origin is the manifest.
* **The manifest's shape:**
  * `version`: 1;
  * `schema`: the schema version it was frozen from, `1.25.0`;
  * `methods`: by side, key to wire;
  * `definitions`: each name `acp` gives a kept definition, mapped to its key in `$defs`;
  * `changes`: for each replaced definition, the listed property keys, and, for a union, its entry keys in the frozen order,
    with the keys whose fragment is frozen;
  * `$defs`: the fragments, the JSON values `schema.unstable.json` holds (re-indented, not escaped), under its keys. These are each kept
    definition's source and the whole unstable version of each replaced definition.

  An entry key is `load.unionEntryKey`'s.
* **The merge with the manifest:**
  * A kept definition is built from its `$defs` fragment. One kept under another name has its references renamed, and its
    `x-method` cleared for a stable wire, as the merge does now.
  * A replaced definition is the stable one with each listed property taken from the frozen version. Its union is rebuilt in
    the frozen order: a frozen key's entry from the frozen version, any other key's from the stable definition, then any stable
    entry the order does not name.
  * The live unstable schema contributes nothing to `acp`.
  * Each definition built from the manifest has the origin `unstable-v1.json`, at its `$defs` key.
* **The writer.** `-write-unstable-manifest` refuses when the file exists. It also refuses unless merging through the manifest
  gives exactly the merge's present output: the schema, the meta, and the provenance but for the origins' file. That is this
  commit's guarantee of unchanged declarations.
* **Refusals:**
  * a manifest that does not parse, or has another `version`;
  * a `definitions` name the stable schema has;
  * a `definitions` key or `changes` name missing from `$defs`;
  * a `changes` name the stable schema lacks;
  * a listed property or frozen entry the frozen version lacks.
* **The acceptance:** `TestGeneratedFilesUpToDate` and `TestCheckAcceptsRepository` pass after regeneration. The `*_gen.go` files
  are byte-identical to today's, and `schema_gen_test.go` differs only in the `File` of its 112 entries (110 types, 2
  absolute-path rules) that named
  `schema.unstable.json`. The root tests pass. A scratch copy whose `schema.unstable.json` lacks `NesCapabilities` passes them
  too: they failed there before this sub-step.

* `schema/unstable-v1.json`: one entry for each of the 102 unstable-only definitions the merge keeps (its whole fragment; an
  unstable definition no walk reaches is dropped by the merge today and stays out), the 18 entries of the 8 replaced definitions
  (each a property or union-entry fragment, with the union's entry order), and each of the 16 unstable methods (its
  `meta.unstable.json` entry). It
  sits beside the schema files so that `make version`, which downloads four named files, never touches it, and so an engine case
  can supply its own as a section; `make clean` removes the four only.
* `go run ./cmd/generate -write-unstable-manifest` writes it from the current schemas; this is run once, in this commit, and
  never by `make version`.
* The merge reads it when present. A stable-reachable definition is the stable definition with the manifest's entries applied;
  a union keeps the frozen version's entry order, then any entries the stable schema adds. An unstable-only definition or method
  in the manifest is built from its frozen fragment, whatever the live unstable schema now says, so a change or a removal there
  changes nothing in `acp`. With no manifest, the merge is as today, so every existing engine case keeps its output.
* Tests:
  * `TestGeneratedFilesUpToDate` and `TestCheckAcceptsRepository` pass with the regenerated files byte-identical to today's: the
    acceptance of this commit.
  * `TestUnstableManifestComplete`: every unstable-origin declaration generated in `acp` has a manifest entry, and every entry is
    generated.
  * Engine cases, each with an `unstable-v1.json` section: `accept-unstable-manifest-kept`, `accept-unstable-manifest-changed` (the
    live schema changes a frozen definition's property; the API listing is the frozen one), `accept-unstable-manifest-removed`
    (the live schema drops a frozen definition and method; the API listing is unchanged, and a `behaviour_test.go` shows the
    method still dispatched to a handler that implements it, as before the removal), `accept-unstable-stable-addition` (the live
    unstable schema adds a property to a stable definition; the stable definition's fields are unchanged), and
    `accept-unstable-union-order`.
  * `refuse-unstable-manifest-*`: a manifest that does not parse, or names a definition the stable schema also has as
    unstable-only.

**2d. The deprecation source, and `example/agent`.**

* The model's `Deprecation` gains a second source, the manifest, with its own text, emitted by `emit/types.go`'s `withNotice`
  path: "Deprecated: this declaration comes from the unstable ACP schema; it stays under v1 for compatibility only
  (0039-MADR-five-adopter-additions-on-schema-1-25-0.md)." The schema-keyword path and its refusal of a note on an unmarked node
  (`model/protocol.go:909`–`:916`) are unchanged. A notice is its own paragraph, as go.dev/doc/comment requires; it names no
  replacement, which the convention allows when there is none.
* It is applied to every manifest declaration: each type, nested and inline type, enum constant and constructor; each of the 10
  fields and 6 variant fields; each of the 16 interface methods, method constants and outbound methods. `AgentExperimental` and
  `ClientExperimental` themselves carry it.
* `example/agent/main.go` no longer asserts `acp.AgentExperimental` or implements the 15 `Unstable*` methods; the agent answers
  them `-32601`, as an agent that does not implement them does. Its own tests, if any rely on them, change with it.
* Regenerate. `golangci-lint` for the host and Windows, with SA1019 enabled as now: the package's own and its external tests'
  uses are exempt by staticcheck's rule; any other report is resolved in this commit, never by excluding the check.
* Engine case `accept-unstable-deprecated`, with a `want.deprecated` listing a manifest type, field, variant field, constant,
  constructor, interface method and outbound method.

**2e. The API gate.** `scripts/check_api.py`:

* gains `REPORTED`, the packages whose incompatible changes are listed under their own heading and do not fail; `PACKAGES` keeps
  the gated ones;
* `scripts/test_check_api.py`, new, its first tests, over a scratch module with two tags: a compatible change, an incompatible
  change in a gated package (exit 1), one in a reported package (exit 0, listed), and a package new since the tag (compatible);
* `acptest` joins `PACKAGES` in phase 3, when it exists, since a package missing from the working tree is a tool error
  (`check_api.py:113`).

**2f. Package `unstable` and its emission, as question 7 was answered: "build the emissions now".**

*Amended 2026-10-25, before the sub-step starts, as the first text required for this answer.* The work is the generator's
second output package (the MADR's G4), and two facts the first text did not name: in the behaviour harness the root package's
import path is `enginetest/<case>` (`cmd/generate/behaviour_test.go:19`, `:130`–`:153`), not the module path, so the emitted
import of `acp` cannot be fixed; and `output.Check` and `enginetest.LoadPackage` resolve imports from source on disk
(`internal/output/check.go:109`–`:131`, `internal/enginetest/api.go:72`–`:97`), which cannot see the root package the same run
has just rendered.

* **What goes into `unstable`,** under question 5's answer: a new unstable-only definition not in the manifest, when standalone
  (a type, enum, request, response or notification), with its constructors; a new unstable method's request and response or
  notification types and a method-name constant; and for a new unstable variant of a stable union, a function
  `Decode<Union><Variant>(raw json.RawMessage) (<Variant type>, error)` that decodes the union's `Unknown` payload. Nothing in
  `unstable` adds to an `acp` type; a new field on a stable type is emitted nowhere.
* **`emit`:** `File` gains `Dir`, `""` for the root and `"unstable"`; `Render` takes the root's import path and returns both
  packages' files; the `unstable` builders use the existing `goFile.qual` to refer to `acp`, under that path.
* **`main.go`:** the root's import path is the module path of the `go.mod` that holds `-out`, joined with `-out`'s directory
  relative to it; with no `go.mod`, as in a non-behaviour engine case, it is the package name, `acp`. The `unstable` files go to
  `<out>/unstable`.
* **`output.Check`:** checks `acp` first, as now, under its import path, then `unstable` through an importer that returns the
  just-checked `acp` package for that path and defers every other path to the shared source importer.
* **`output.Write`:** writes each package's directory, staged as now. A package directory the run renders nothing into keeps no
  generated file: a `*_gen.go` left there from an earlier run is removed, in the same stage-then-rename order, so a schema that
  stops having new unstable definitions leaves `unstable/` with its `doc.go` alone.
* **`unstable/doc.go`,** hand-written: the package's purpose, its compatibility stance (`golang.org/x/exp`'s: "not subject to the
  Go 1 compatibility promise"), and how a name moves to `acp` when it stabilizes. At schema `1.25.0` it is the only file:
  every unstable definition is in the manifest.
* **Tests in `cmd/generate`:** `TestGeneratedFilesUpToDate` and `TestCheckAcceptsRepository` cover both directories; a `main.go`
  test for the import-path rule, with a `go.mod`, nested under one, and with none; an `output` test that a stale `*_gen.go` in an
  empty package directory is removed and a hand-written file is not.
* **Engine harness:** a case may have `want.api.unstable` and `want.deprecated.unstable` sections (a section name may not hold
  `/`, `enginetest/archive.go:60`–`:64`); `enginetest.LoadPackage` gains the same serving importer, to list `<out>/unstable`
  against the case's own root; a behaviour case's `unstable` is `enginetest/<case>/unstable`.
* **Engine cases:**
  * `accept-unstable-standalone`: a new standalone unstable definition is in `want.api.unstable` and not in `want.api`;
  * `accept-unstable-variant-decode`: a new unstable variant of a stable union is in no API listing as a field; its `Decode…` is
    in `unstable`; a `behaviour_test.go` decodes a payload into the union's `Unknown` and then into the variant;
  * `accept-unstable-method-types`: a new unstable method's types and constant are in `unstable`, and a `behaviour_test.go` shows
    a received one answered `-32601` by `acp`'s dispatch;
  * `accept-unstable-field-nowhere`: a new unstable property of a stable definition is in neither listing;
  * `accept-unstable-empty`: with every unstable definition in the manifest, no file is written to `unstable/`.
* **`scripts/check_api.py`:** `unstable` joins `REPORTED`, so `make api` lists it as a new package, and its later incompatible
  changes are listed, not gated.

*Amended 2026-10-25, before the sub-step's code, by the deviation entry "2f, what `unstable` may use of `acp`" and the MADR's
questions 9 and 10:*

* **What is new.** A definition is new when it meets three conditions:
  * the live merge (`MergeStableAndUnstable` over the live files) keeps it;
  * its key is neither a stable definition nor a manifest fragment;
  * its key is not an `acp` name, which is refused.

  A method is new when its wire is in neither the stable meta nor the manifest. A variant is new when the live version of a
  stable union has an entry whose key the `acp` union lacks.
* **The model of `unstable`.** It is `model.Build` over:
  * `acp`'s merged schema;
  * the new definitions, under their keys, with each reference to a frozen key that `acp` holds only under another name
    renamed to that name;
  * the live version of each union with a new variant;
  * `acp`'s meta with the new methods.

  `acp`'s names are declared in it, so a new name that collides with one is refused by the namer.
* **What is emitted.** One file, `unstable/unstable_gen.go`, holds:
  * the new definitions' types and constructors;
  * the new variants' inline types and `Decode<Union><Variant>`;
  * the new methods' constants;
  * the decoding helpers they use.

  Every other name is `acp`'s, written through `goFile.qual` with the root's import path. That covers the `acp` types,
  `ValidationError` and the `Rule*` constants.
* **`acp`'s exported API only:**
  * A failed check is `&acp.ValidationError{Type, Field, Value, Rule}`, without `msg`.
  * An `acp`-typed value is checked with its `Validate`, by the host's rule.
  * An `acp`-typed variant is decoded with `json.Unmarshal`, not `decodeKeys`.

  An engine case pins each of the three.

Docs: `README.md` gains "Stability": the stable `acp` API, the deprecated unstable names, package `unstable`, and the policy;
`docs/architecture.md` describes the provenance, the manifest, the merge restriction, the deprecation source and the second
package; `RELEASING.md`'s schema-bump steps say the manifest is never regenerated by a bump, and that `make api`'s report for
`unstable`, which does not gate, is read and recorded in the bump's record.

Planted breaks, each in its own copy of the tree, through a scratch script for the `cmd/generate` module:

| Break | The check that must fail |
| :--- | :--- |
| 2b: a changed property not recorded | the `model` test, naming `AgentCapabilities.mcpCapabilities` |
| 2c: one entry dropped from `unstable-v1.json` | `TestUnstableManifestComplete`, and `TestGeneratedFilesUpToDate` |
| 2c: the merge applies the live unstable version, not the frozen one | `accept-unstable-manifest-changed` |
| 2c: a removed frozen definition not rebuilt | `accept-unstable-manifest-removed` |
| 2c: a stable definition takes an unlisted unstable property | `accept-unstable-stable-addition` |
| 2c: a union's added entries appended in stable order | `accept-unstable-union-order` |
| 2d: one field's notice not emitted | `accept-unstable-deprecated`, and `TestGeneratedFilesUpToDate` |
| 2d: `example/agent` keeps `acp.AgentExperimental` | `make lint`, SA1019 |
| 2e: a reported package's incompatible change fails | `test_check_api.py` |
| 2e: a gated package's incompatible change passes | `test_check_api.py` |
| 2f: a new standalone unstable definition emitted into `acp` | `accept-unstable-standalone` |
| 2f: a new unstable variant emitted as a field of the stable union | `accept-unstable-variant-decode` |
| 2f: `unstable` imports `acp` under a fixed path | the behaviour cases, which build it as `enginetest/<case>/unstable` |
| 2f: `output.Check` type-checks `unstable` against the source on disk | `accept-unstable-standalone`, whose root exists only in memory |
| 2f: a stale `*_gen.go` left in an empty `unstable/` | the `output` test, and `accept-unstable-empty` |
| 2f: `unstable` added to `PACKAGES`, gated | `test_check_api.py` |

### 3. Package `acptest` (MADR item 5)

New files in `acptest/`, each with a doc comment citing the MADR:

* `doc.go`: what the package is for, that it uses only the public constructors, and that it is on `encoding/json` v1.
* `pair.go`: `Connect(t testing.TB, agent acp.Agent, client acp.Client, opts ...acp.ConnectionOption) Pair`, over two `io.Pipe`s;
  `Pair` with `Agent`, `Client`, `Close` and `Wait`; `t.Cleanup` closes and waits for both.
* `abs.go`: `Abs`, as the root's `testAbsPath` (`paths_test.go:24`) does.
* `recorder.go`: `Recorder`, wrapping a side's reader and writer, copying each newline-terminated frame; `Frames()` returns copies;
  canonical form by `encoding/json` v1 as `json_v1_pin_test.go` pins it.
* `script.go`: `ScriptAgent`, embedding `acp.AgentRefusals`; `Initialize` answers `ProtocolVersionNumber` and the given capabilities;
  each `Prompt` plays its list of `SessionUpdate`s, then its `StopReason`.
* `client.go`: `MemoryClient`, embedding `acp.ClientRefusals` and implementing `ReadTextFile` and `WriteTextFile` over a map, with
  1-based `line` and `limit`, and `NewResourceNotFound` for a missing path; `RequestPermission` from a map of scripted outcomes;
  `SessionUpdate` records what it receives.

Tests, in `acptest/*_test.go`, with a `TestMain` that runs the goroutine-leak guard as the root's `leak_test.go` does:

* `TestConnectRoundTrip`: `initialize` and one prompt with two updates, recorded by `MemoryClient`; cleanup leaves no goroutine.
* `TestAbs`: the result is accepted by `filepath.IsAbs` on the host, on Linux, macOS and Windows (the `windows` and `linux` gates).
* `TestRecorder`: frames split at newlines across short writes; copies, not aliases; canonical bytes equal to `json.Marshal`'s.
* `TestScriptAgent`, `TestMemoryClient`: each method, `line`/`limit` at the edges, the missing path's `-32002`, a gated method's
  `-32601`.
* Each example: `ExampleConnect`, `ExampleRecorder`.
* The package's tests pass with `GOEXPERIMENT=nojsonv2` and at the Go floor (the `nojsonv2` and `floor` gates).

`scripts/check_api.py`'s `PACKAGES` gains `acptest`, gated, in this phase's commit, now that the package exists; `make api`
then lists it as new.

Docs: `README.md` gains "Testing with acptest"; `docs/architecture.md` lists the package and its tree line.

Planted breaks:

| Break | The check that must fail |
| :--- | :--- |
| `Connect`'s cleanup does not `Wait` | the leak guard in `TestMain` |
| `Recorder` returns the frame slice it holds | `TestRecorder`, aliasing |
| `MemoryClient` reads `line` as 0-based | `TestMemoryClient`, first line |
| a missing path answered `-32603` | `TestMemoryClient`, missing path |
| `Abs` returns its argument unchanged on Windows | `TestAbs`, on the `windows` gate |

### 4. `PresentNotice` and `PresentCompaction` (MADR item 2)

In `helpers.go`: the three ~~methods on `*AgentSideConnection`~~ package-level functions, each taking the `*AgentSideConnection`
after the context (*amended 2026-10-25, by 0038's step 2 deviation; `TestSideConnectionMethodsAreProtocolMethods` refuses a
method*), taking `Notice`, `CompactionUpdate` and `CompactionSummaryChunk`, under question 1's answer, and an error for a nil
connection:

* with the capability in `Negotiated()`'s record: the dedicated variant, through `SessionUpdate`;
* without it: `PresentNotice` sends `UpdateAgentMessageText` of the title, and the description on a line after it when present;
  `PresentCompaction` sends `UpdateAgentMessageText` of "Compaction {status}", and the error when present;
  `PresentCompactionSummary` sends nothing and returns nil;
* before `initialize` has completed: `ErrNotInitialized`, as `SessionUpdate` returns.

Tests, in `present_test.go`, package `acp_test`, so they can use `acptest` (an in-package test cannot import a package that imports
`acp`):

* over `acptest.Connect` with a `MemoryClient` that records updates, with and without `session.notices` and `session.compaction`:
  the variant sent, or the agent message and its text; `UpdateNotice` sent directly still fails with `*CapabilityError` without the
  capability;
* the three terminal statuses each sent as the variant with the capability;
* before `initialize`, `ErrNotInitialized` and nothing written;
* ~~`ExampleAgentSideConnection_PresentNotice`~~ `ExamplePresentNotice` (*amended 2026-10-25*).

Docs: `README.md`'s agent section.

Planted breaks:

| Break | The check that must fail |
| :--- | :--- |
| `PresentNotice` reads `Session.Compaction` | the notice case without `session.notices` |
| the fallback text drops the description | the fallback's text |
| `PresentCompactionSummary` falls back to an agent message | the summary case without the capability |
| `PresentNotice` sends before `initialize` | the pre-initialize case |

### 5. `CompactionTimeline` (MADR item 3)

`compaction.go`, new, in the root: `CompactionEntity`, `CompactionTimeline`, `Apply`, `Get` and `Open`, as the MADR specifies.
`cmd/generate/engine_test.go`'s `TestRuntimeFilesCoverHandWritten` names `compaction.go` beside `helpers.go` and `refusals.go`, in
this commit, since the file names ACP types.

Tests, in `compaction_test.go`, table-driven over sequences of `SessionUpdate`s, and replay fixtures in
`testdata/compaction/*.jsonl`, each line one `session/update` params object:

* first-seen `compaction_update`: status `in_progress` then the notification's, optional fields unset then applied;
* each patch field omitted, `null` and a value; `summary: []` and `summary: null` clearing; a concrete `summary` replacing chunks;
* chunks appending in order; a chunk for an unknown ID first-seen-initializing, as the MADR infers;
* an unknown status kept as it arrived, and reported by `Open`; terminal statuses not reported;
* a non-compaction update returning `false`;
* each fixture applied twice, as a live stream and as a `session/load` replay into a fresh timeline, giving equal entities.

Docs: `README.md`'s client section; `docs/architecture.md`'s hand-written list.

Planted breaks:

| Break | The check that must fail |
| :--- | :--- |
| `summary: null` treated as absent | the clear case |
| a concrete `summary` appended, not replacing | the replace case |
| an unknown status replaced by `in_progress` | the unknown-status case |
| `Open` omits unknown statuses | the `Open` case |
| `compaction.go` missing from `TestRuntimeFilesCoverHandWritten` | that test, through the `cmd/generate` gate |

### 6. `ExtensionMux` and the `_meta` helpers (MADR item 4)

* `extensions.go`: `ExtensionMux`, `Handle`, `Notify`, `Methods`, `TraceMeta` and `AdvertiseMeta`. It uses no ACP type, since
  `extensions.go` is a runtime file: decode errors are `NewInvalidParams`, other errors go through `toReqErr`, both in `errors.go`.
* `connection.go`: `IsNotification(ctx) bool`, the exported form of `sentAsNotification` (`connection.go:85`), which reads the
  `messageMark` the receive loop puts in a notification handler's context (`:1602`) and which the generated dispatch already uses;
  false for a context `receive` did not make (question 6's answer).
* The mux: a registration whose name does not start with `_`, or is already registered, panics, as a `ServeMux` pattern conflict
  does; a request for a `Handle` name decodes `P` and answers `R`; a notification for a `Notify` name runs its handler; a request
  for a `Notify` name, or an unregistered request, answers `NewMethodNotFound`; an unregistered notification, or a notification
  for a `Handle` name, is ignored.

Tests, in `extensions_mux_test.go`, package `acp_test`, over `acptest.Connect`:

* a typed request and its answer; a params type mismatch answered `-32602`; a handler's `*RequestError` passed through;
* an unregistered `_` request answered `-32601`; an unregistered `_` notification ignored, and nothing answered;
* a request and a notification of the same `Notify` name: the handler runs only for the notification;
* registration panics; `Methods()` sorted;
* `TraceMeta` with empty values omitted; `AdvertiseMeta` copying its input and setting the namespace;
* `IsNotification` in a typed notification handler and a typed request handler.

Docs: `README.md`'s "Extension methods" section.

Planted breaks:

| Break | The check that must fail |
| :--- | :--- |
| `IsNotification` always false | the same-name request and notification case |
| an unregistered notification answered | the unregistered notification case |
| `AdvertiseMeta` writes into its input map | the copy case |
| `Handle` accepts a name without `_` | the registration panic case |

### 7. Close the implementation

* `make api` against the newest `v1` tag: 0 incompatible in `acp`, `agentproc` and `acptest`; `unstable` and `acptest` as new
  packages; record what it lists.
* `tools/gates.py --all`.
* `README.md`, `docs/architecture.md` and `RELEASING.md` read in full against the code.
* The MADR's "Confirmation" walked item by item in the execution record, each with the check that meets it.

The owner commits. A push needs the owner's ask in that turn: before it, `git fetch`, `make push-guard` and
`tools/idscan.py --range origin/main..main`; after it, `tools/watch_ci.py <commit>`.

### 8. Releases, only on the owner's ask in that turn

~~Under P1's answer, after phase 3 and after phase 7.~~ *Amended 2026-10-25, P1 changed:* one release, after phase 7. Through
`RELEASING.md`, "Tag and Publish":
* the release-notes report and its row, committed before the tag;
* `tools/release_body.py`;
* the annotated tag and its push;
* the GitHub release;
* `tools/publication.py`;
* `tools/consumer.py`, extended in the release's commit to use what it adds (`acptest.Connect`, `PresentNotice`,
  `CompactionTimeline` and `ExtensionMux`), with `make check-tools`.

When the release is published, this PLAN and its row are `complete`.

## Verification

* Every item of the MADR's "Confirmation" holds, each by its phase's tests.
* `make api` lists only compatible changes in `acp`, `agentproc` and `acptest`.
* `TestGeneratedFilesUpToDate`, `TestCheckAcceptsRepository`, `TestRuntimeFilesCoverHandWritten`, `TestUnstableManifestComplete`
  and the new engine cases pass.
* After 2c, the regenerated `*_gen.go` files are byte-identical to the committed ones; after 2d, they differ only by the
  deprecation paragraphs (`git diff` on them adds comment lines and nothing else).
  *Amended 2026-10-25 by the deviation entry "2d, the generated diff": once a method constant carries a notice, `gofmt`
  re-aligns its `const` block, so the check is that each file's token stream, comments skipped, is identical to the committed
  file's, and that `git diff -w` adds only comment lines and removes none.*
* `make lint`, staticcheck's SA1019 included, passes on the whole module, `example/agent` included.
* The root's and each new package's `go test ./...` and `-race`; the `cmd/generate` tests; `make pre-add-check`; `tools/gates.py
  --all` at every code phase, on the Linux and Windows hosts and at the floor.
* `make disclosure-check` and `tools/idscan.py`: nothing found.
* Every planted break in phases 2 to 6 fails its check, each in its own copy of the tree, and the execution record keeps each one's
  failing line.

## Rollout and Rollback

* **Rollout.** Phases 2 to 6 are separate commits, and phase 2 several. ~~Under P1, the first release carries the boundary and
  `acptest`; the second the three runtimes.~~ *Amended 2026-10-25, P1 changed:* one release carries all five additions. Each
  consumer adopts the tag under its own issue.
* **What a consumer meets on the release.** Every unstable-origin name is deprecated, so a consumer whose staticcheck runs
  SA1019 sees a report for each use; the API and the wire are unchanged. The release notes say so, and list the names by kind.
* **Rollback before a tag:** revert the phase's commits.
* **Rollback after a tag:** a name in `acp`, `agentproc` or `acptest` cannot be removed under `v1`; a defect is fixed forward in a
  patch. A name in `unstable` may be changed in a minor, by the MADR's policy. A broken tag is retracted, as
  [0020-MADR-retract-accidental-v1-tag.md](0020-MADR-retract-accidental-v1-tag.md) did.

## Execution record

### Phase 1, records, 2026-10-25

* **The pair** was written, assessed against its sources (the MADR's first assessment), and assessed again for item 1 against the
  generator's code (the second, G1 to G10, which rewrote phase 2). The owner committed the MADR and PLAN as `a91343a` and the
  second assessment as `14419b0`.
* **The owner's answers:** questions 1 to 6 and P1 as recommended; question 7, "build the emissions now", not the
  recommendation. The MADR's "Decided with the owner" table holds all seven, and 2f was amended for question 7's answer before
  it starts. The owner approved execution: "committed, proceed". This PLAN is `in-progress`.
* **The prerequisite** holds: [0038-PLAN-consumer-helpers-kind-and-agent-refusals.md](0038-PLAN-consumer-helpers-kind-and-agent-refusals.md)
  is `complete`, with `v1.22.0` published.
* **Checks** on the records, each exit 0: `make check-records`, `make markdownlint`, `make disclosure-check`, and
  `tools/idscan.py --paths` over the three files.

### 2a, inventory and re-measure, 2026-10-25

Read-only: nothing in the tree was written but this entry and the MADR's G2 correction. Every measurement ran in the session's
scratch directory.

* **How.** Rather than a reimplementation of `MergeStableAndUnstable`, a scratch test was dropped into a `git archive` copy of
  `4be1e0f`'s `cmd/generate`. It ran the generator's own `loadSchema` and `model.Build` over the committed schemas, then
  type-checked the root with `enginetest.LoadPackage`. Each exported package-level object of the `*_gen.go` files was attributed
  to the model type that declares it: the type itself, its inline variant types, its enum constants and its constructors.
* **G2, the merge:** 102 unstable-only definitions, 56 prefixed and 46 not; 15 unstable-only wire methods, giving 16 methods (15
  agent, 1 client: `mcp/message` is on both sides), the list G2 names. As G2 states.
* **G2, the declarations:** 174 exported package-level declarations, not 172: 102 types, 25 inline variant types (no nested
  struct), 21 enum constants and 26 constructors. Every attributed name is declared, and no `Unstable*` name is unattributed.
  The 23 came from a name-prefix match corrected by hand; the model's attribution, which leaves no name unplaced, finds 25. They also carry
  169 exported methods (`MarshalJSON`, `UnmarshalJSON`, `Validate`). The MADR's G2 is corrected, with the old figures struck
  through. No step of this PLAN depends on the count: `TestUnstableManifestComplete` (2c) compares the manifest with what is
  generated.
* **G10, the replaced definitions:** 13 definitions differ under the loader's `reflect.DeepEqual`, the same 13 as the raw-JSON
  diff. The merge replaces 8: `AgentCapabilities`, `ClientCapabilities`, `InitializeResponse`, `McpCapabilities`, `McpServer`,
  `PromptResponse`, `SessionCapabilities` and `SessionUpdate`. They differ in 18 entries and no other keyword: 10 added
  properties, 2 changed (`AgentCapabilities.mcpCapabilities`, `InitializeResponse.agentCapabilities`), and 6 added union entries
  (`SessionUpdate` at indexes 6, 7, 16, 17 and 18; `McpServer` at 2). As G10 states.
* **What the 18 entries yield in `acp`:**
  * 10 added fields, at the `types_gen.go` lines G2 cites, which were read again;
  * 2 changed fields, whose Go declaration the default does not change;
  * 6 variant fields, each with an inline variant type: `SessionPlanUpdate`, `SessionUpdatePlanRemoved`,
    `SessionUpdateSessionMessage`, `SessionUpdateSessionMessageChunk`, `SessionSubagentUpdate` and `McpServerAcpInline`;
  * one constructor, `NewMcpServerAcp` (`helpers_gen.go:224`).

  The six types and the constructor are manifest declarations under 2d's "every manifest declaration", so 2d deprecates them with
  the 10 fields and 6 variant fields. The 2 changed properties declare no new name and are not deprecated.
* **1.26.0 re-measure.**
  * Setup: a local `git clone` in the scratch directory. The four `schema-v1.26.0` files were downloaded into it: `meta.json` and
    `schema.json` are byte-identical to `1.25.0`, and the two unstable files differ.
  * Regeneration first failed in `output.Check`, with 30 type errors in root `*_test.go` files that name NES types 1.26.0
    removes (from `acp_test.go:145`). The 63 root test files were moved out of the clone, as `apidiff` reads only the package. The
    regeneration then succeeded.
  * Result: `scripts/check_api.py` against `v1.22.0`, the newest `v1` tag, exits 1 with **115 incompatible, 16 compatible**. 40 of
    the 115 name no `Unstable` identifier: 19 NES capability types (`NesCapabilities`, `ClientNesCapabilities` and 17 more),
    `PositionEncodingKind` and `TextDocumentSyncKind` with their 5 constants, the ten `AgentMethodDocument*` and
    `AgentMethodNes*` constants, and the four capability fields. These are the same counts as 0025's
    against `v1.17.1`.

### 2b, provenance for properties and union entries, 2026-10-25

* **`load`** (`merge.go`):
  * `Provenance.Changes` maps each stable definition the merge replaced to a `Changes`, holding the properties the unstable
    version adds and changes, by key, and the union entries it adds and changes, by index in its list. The list is the `anyOf`,
    or the `oneOf` when there is no `anyOf`, the rule by which the model sets `ExactlyOne`.
  * An entry equal to a stable one is unchanged. Otherwise `unionEntryKey` matches it to the stable entry it changes, by:
    1. the value of the union's declared discriminator;
    2. else its `const` properties;
    3. else its title, its reference, or its `allOf` references.

    An entry with no match is added. What the unstable version removes is not recorded: the merge removes nothing from a stable
    definition at `1.25.0`, and 2c builds stable definitions from the stable schema.
  * *Where it is computed:* `MergeStableAndUnstable` computes `Changes` from the set `promoteChangedSharedDefs` returns, over the
    stable and unstable schemas it holds. That is the same pair of versions the replacement at `:329`–`:331` compares, so
    `promoteChangedSharedDefs` and its edge tests are unchanged.
* **`model`:** `Field.Unstable` and `Variant.Unstable` are set by `markUnstableEntries` from `Changes`, for added properties and
  added entries only. A changed property's field and a changed entry's variant are a stable declaration.
* **Tests:**
  * `load/changes_test.go`, new:
    * `TestMergeChanges`, over a synthetic pair: an added and a changed property, union entries added (one with no key) and
      changed, in a `oneOf` with a discriminator and in an `anyOf` without one, and a definition the merge kept;
    * `TestUnionEntryKey`, the key's order.
  * `model/unstable_test.go`:
    * `TestUnstableEntries`, synthetic;
    * `TestUnstableEntriesOfCommittedSchema`, which finds the 8 replaced definitions, the 2 changed properties, no changed
      entry, the 10 fields and the 6 variants with their inline types, as 2a measured.

    `committedModel` in `classes_test.go` is split so that `committedInput` returns the provenance.
* **No generated file changed:** `TestGeneratedFilesUpToDate` passes, the `regen` gate leaves the files as they were, and
  `make api` reports 0 incompatible, 0 compatible against `v1.22.0`.
* **Planted breaks:** each was planted in its own copy of the tree through a scratch script, and each failed as required:

  | Break | Failing line |
  | :--- | :--- |
  | the changed-property case dropped (the PLAN's 2b row) | `unstable_test.go:172: changed properties:` … `want ["AgentCapabilities.mcpCapabilities" "InitializeResponse.agentCapabilities"]` |
  | the added-entry case dropped | `changes_test.go:66: Update: Changes {… AddedEntries:[] ChangedEntries:[2]}, want {… AddedEntries:[1 4] ChangedEntries:[2]}` |
  | the same, against the committed schemas | `unstable_test.go:172: unstable variants:` … `"SessionUpdate.PlanUpdate SessionPlanUpdate"` |
  | the declared discriminator ignored | `changes_test.go:95: case 0: unionEntryKey(entry, "kind") = "const kind=a,other=z", want "const kind=a"` |
  | a changed entry read as added | `changes_test.go:66: Update: Changes {… AddedEntries:[1 2 4] ChangedEntries:[]}, want …` |
  | variants not marked | `unstable_test.go:117: Update variant 1 (new): Unstable = false, want true` |
  | every field marked | `unstable_test.go:111: Caps.a: Unstable = true, want false` |

  The first attempt at "a changed entry read as added" did not compile: the plant left `key` unused. It was corrected to keep
  the variable. Two failure messages were then made to name values rather than pointers.
* **Checks**, each exit 0:
  * `make pre-add-check`: "184 file(s) clean";
  * `(cd cmd/generate && go test -count=1 ./...)`;
  * `tools/gates.py --all`, with the Linux and Windows hosts reached over SSH: all 18 gates `ok`, `newfiles` over
    `changes_test.go`;
  * `make disclosure-check`;
  * `tools/idscan.py --paths` over the six Go files: 0 matches.

### Deviation, 2026-10-25: 2c, where a frozen type's schema is read

* **Found** while reading 2c's inputs, before any 2c code. The emitted registry in `schema_gen_test.go`
  (`internal/emit/registry.go:79`, `:111`–`:112`) gives each type its origin's file and key. The root conformance tests build JSON
  instances from that file at that key: `instances_test.go:28`–`:45`, `newBuilder(st.File)` and `b.def(st.Def)` in
  `conformance_test.go`, `encodings_test.go` and `fuzz_test.go`. 112 entries name `schema.unstable.json`: 110 types and the 2
  absolute-path rules of `UnstableForkSessionRequest`.
* **Measured** in a scratch copy of `04c37eb`. `NesCapabilities` alone was removed from `schema/schema.unstable.json` and the
  generated files were kept, as a manifest-frozen `acp` keeps them. `go test -count=1 .` exits 1, and `TestEncodingsUnchanged`,
  `TestConformanceRoundTrip` and `TestConformanceLenient` fail, first at `TestConformanceRoundTrip/NesCapabilities`.
* **Pre-existing:** yes. Today a bump also regenerates the types, so nothing reads a missing key. The manifest is what makes it a
  failure.
* **Decided** by the owner, from two resolutions: "Manifest holds frozen $defs". The other, keeping the origin on
  `schema.unstable.json` in 2c and repointing it before the first unstable bump, was not chosen. The MADR records it as
  question 8 and amends "How it is built".
* **Scope added to 2c:** `instances_test.go`, `cmd/generate/internal/model/gates.go`. 2c's acceptance is restated in its
  amendment.

### 2c, the manifest, 2026-10-25

* **`load/manifest.go`, new.**
  * `Manifest`, `ParseManifest` (strict, with each `$defs` fragment decoded) and `ReadUnstableManifest`.
  * `MergeManifest`, the merge through the manifest that the amendment describes.
  * `WriteUnstableManifest` and `ManifestReproduces`.
  * `ManifestFile` is `unstable-v1.json`.
  * `unionEntryKey` is exported as `UnionEntryKey`, the manifest format's entry key, for the completeness test in package
    `main`.
* **`main.go`:** `loadSchema` reads the manifest after the live unstable files are read and checked. When the manifest is
  present, it merges through it; otherwise the merge is as before. `-write-unstable-manifest` freezes the manifest and generates
  nothing.
* **`schema/unstable-v1.json`**, 166,338 bytes, written by `go run ./cmd/generate -write-unstable-manifest`, once. A second run
  refuses: "… exists; the manifest is frozen once and never rewritten". It holds:
  * `schema` `1.25.0`;
  * 16 methods (15 agent, 1 client);
  * 102 definitions, 56 renamed, from 100 distinct `$defs` keys: two keys give both a prefixed copy and an unprefixed one;
  * 8 changes, of 12 properties (10 added, 2 changed) and 6 frozen entries in 2 unions;
  * 107 fragments.

  The writer's own check, that merging through it reproduces the merge, passed when it wrote.
* **Regenerated:** every `*_gen.go` is byte-identical. `schema_gen_test.go` differs in 112 lines, each only
  `"schema.unstable.json"` → `"unstable-v1.json"`: 110 type entries and the 2 absolute-path rules of
  `UnstableForkSessionRequest`. This was checked line by line against `HEAD` by a scratch script.
* **`instances_test.go`:** `schemaDefs` adds `unstable-v1.json`, the manifest's `$defs` over `schema.json`'s. **`model/gates.go`:**
  `ruleTypes` accepts a copy whose origin is the manifest.
* **Tests:**
  * `load/manifest_test.go`, new:
    * `TestWriteUnstableManifest`: the definitions, changes and order frozen; the round trip through `sameMerge`; an unescaped
      fragment; the refusals of a second write and of missing unstable files;
    * `TestApplyChange`: the union order, a dropped stable entry, an unkeyed stable entry, and the five refusals;
    * `TestManifestChangeRefusesUnkeyedEntries`.
  * `cmd/generate/manifest_test.go`, new: `TestUnstableManifestComplete`.
    * The model's unstable types, fields, variants and methods equal the manifest's.
    * Every listed property and frozen entry is generated, and every unstable type comes from the manifest.
    * While `schema/version` equals the manifest's `schema`, `load.ManifestReproduces` holds. Without that half, a manifest
      entry dropped from both the model and the manifest would go unseen. It was added when the first planted break showed this.
  * 12 engine cases, each written by a scratch script that freezes the manifest with `-write-unstable-manifest`:
    * `accept-unstable-manifest-kept`, `-changed`, `-removed` (with `accept-unstable-merge`'s `behaviour_test.go`) and
      `accept-unstable-stable-addition`. Each takes `accept-unstable-merge`'s `want.api`, unchanged, as its oracle; none was
      `-update`d.
    * `accept-unstable-union-order`, whose `want.api` was written by `-update` and read: `Pick` holds `A, N, B, C`.
    * `refuse-unstable-manifest-not-json`, `-version`, `-unknown-key`, `-stable-name`, `-missing-fragment`, `-change-not-stable`
      and `-change-property`.
* **The bump, simulated in scratch copies:**
  * With `NesCapabilities` removed from `schema.unstable.json`, regeneration leaves every generated file unchanged, and the
    root tests pass. They failed there before this sub-step (the deviation entry).
  * With the real `schema-v1.26.0` unstable files, regeneration again leaves every generated file unchanged, and the root tests
    pass. 2a measured 115 incompatible changes for the same files without the manifest.
* **Planted breaks:** each was planted in its own copy of the tree through a scratch script, and each failed as required:

  | Break | Failing line |
  | :--- | :--- |
  | 2c's row: `nes` dropped from the manifest's `AgentCapabilities` change | `manifest_test.go:77: the manifest is not what schema 1.25.0's merge gives: definition "AgentCapabilities" differs`; `main_test.go:64: types_gen.go is out of date` |
  | 2c's row: the merge applies the live unstable version (`loadSchema` merges through the manifest only without live files) | `--- FAIL: TestEngineCases/accept-unstable-manifest-changed` and `…/accept-unstable-stable-addition` |
  | 2c's row: a frozen method not rebuilt | `--- FAIL: TestEngineCases/accept-unstable-manifest-removed`, and its behaviour case: `definition UnstableNewRequest: linked … to agent method "x/new", which no meta file lists` |
  | 2c's row: added entries in stable order | `accept-unstable-union-order`: `type Pick struct{A …; B …; N …; C …}` against `A, N, B, C` |
  | one listed property not applied | `manifest_test.go:38: PromptResponse.usage is in the manifest and not generated` |
  | origins naming the unstable schema | `manifest_test.go:37: ClientNesCapabilities: … want a generated type from unstable-v1.json`; `schema_gen_test.go is out of date` |
  | the stable-name refusal removed | `--- FAIL: TestEngineCases/refuse-unstable-manifest-stable-name` |
  | the version check removed | `refuse-unstable-manifest-version`: `run accepted the schema` |
  | the frozen-once refusal removed | `manifest_test.go:98: second write: <nil>, want a refusal` |
  | fragments HTML-escaped | `manifest_test.go:65: the manifest escapes its fragments' text` |
  | an unkeyed entry frozen | `manifest_test.go:204: unkeyed: <nil>, want "union entry 1 has no key"` |
  | the root's frozen `$defs` without `schema.json` under them | `--- FAIL: TestConformanceRoundTrip/UnstableStartNesResponse` |

  Two first attempts did not fail on the intended line, and both were corrected:
  * `nes` dropped from the manifest did not fail `TestUnstableManifestComplete` as first written. Its comparison was then
    extended with `ManifestReproduces`.
  * Applying only the first property failed in `model.Build`, since the protocol table needs `ClientCapabilities.subagents`.
    It was replaced by skipping `usage`, which nothing else needs.
* **Checks**, each exit 0:
  * `make pre-add-check`: "185 file(s) clean". `gocognit` first refused `TestUnstableManifestComplete` at 56, and the test was
    split into `modelMarks` and `manifestMarks`.
  * `(cd cmd/generate && go test -count=1 ./...)`; `go test -count=1 .` at the root.
  * `tools/gates.py --all`, with the Linux and Windows hosts reached over SSH: all 18 `ok`. `newfiles` covered the three new Go
    files, and `api` reported "0 incompatible, 0 compatible" against `v1.22.0`.
  * `make disclosure-check`; `tools/idscan.py --paths` over the 24 changed and new files: 0 matches.

### Deviation, 2026-10-25: 2d, the generated diff

* **Found** when regenerating in 2d. The Verification's "`git diff` on them adds comment lines and nothing else" cannot hold as
  worded. 2d puts a notice on each frozen method constant, and those constants sit in the aligned `const` blocks of
  `constants_gen.go`. A comment line between two specs ends `gofmt`'s alignment group, so 24 constant lines change their
  padding: `git diff --numstat` gives 72 added and 24 removed for `constants_gen.go`. No other generated file removes a line.
  Leaving the constants without a notice was not an option, since 2d lists them, and a block of their own would move lines,
  not only pad them.
* **Not pre-existing:** this is the first notice on a method constant.
* **Resolved** by checking what the sentence was for, that no declaration changes, directly and more strictly. In a scratch
  program, each regenerated file's token stream, comments skipped, is compared with the committed file's, and it is identical
  for all six. `git diff -w` adds only comment lines and removes none. The comparison was seen to fail on a copy of
  `constants_gen.go` with one value changed (`"nes/accept"` → `"nes/acept"`: "same tokens without comments: false"). The
  Verification bullet is annotated. No decision and no MADR fact changes, since the MADR's Confirmation says "changes no
  generated declaration but the deprecation paragraphs".

### 2d, the deprecation source and `example/agent`, 2026-10-25

* **`model`:**
  * `Deprecation` gains `Frozen`, the manifest's mark.
  * `Interface` gains `Deprecation`.
  * `frozen.go`, new: `applyFrozenDeprecations` runs as the last build step, after `applyDeprecations`, so a protocol-table note
    never names a frozen mark, and a mark the schema gives is kept. It marks:
    * each type built from the manifest, with its nested structs, variants and enum constants;
    * each field and variant the manifest adds to a stable type;
    * each method whose request or notification type comes from the manifest;
    * each interface all of whose methods are so marked.
* **`emit`:**
  * `withNotice` writes `frozenNotice` for a frozen mark.
  * An interface's doc carries its mark.
  * `methodConsts` gives a frozen method's constant its notice.

  The schema-keyword text and the refusal of a note on an unmarked node are unchanged.
* **Regenerated:** 1,076 comment lines in all. `git diff --numstat` counts 1,100 lines added, 24 of them the re-aligned
  constants. The notices, counted by a scratch script over the five `*_gen.go` files, match
  2a's inventory:

  | Declaration | Notices |
  | :--- | :--- |
  | types | 133: the 102 kept definitions, 25 inline variant types, and the 6 inline types of the stable unions' new variants |
  | constructors | 27: 26, and `NewMcpServerAcp` |
  | enum constants | 21 |
  | method constants | 16 |
  | interface methods | 16 |
  | outbound methods | 16 |
  | interfaces | 2: `AgentExperimental` and `ClientExperimental` |
  | fields | 42: the 10 new fields, the 6 variant fields on stable unions, and the 26 variant fields of the unstable unions |

  Every manifest definition has its type's notice.
* **`example/agent/main.go`:** the `acp.AgentExperimental` assertion and the 15 `Unstable*` methods are removed. No test relies
  on them: `example_pair_test.go` builds and runs the example. Before the removal, `make lint` exited 2 with 24 SA1019 reports
  per OS, all in this file and none elsewhere in the module. After it, `make lint` reports "0 issues." on both. Nothing was
  excluded.
* **Tests:**
  * `model/frozen_test.go`, new: `TestFrozenDeprecations`. A type from the live unstable schema has no mark, a schema mark is
    kept, and an interface with a method that is not frozen has none, while its frozen method does.
  * The engine case `accept-unstable-deprecated`, new, its manifest frozen by the generator. Its `want.deprecated`, written by
    `-update` and read, lists 20 notices, and no stable declaration has one. They are:
    * the kept type `Level` and its two constants;
    * the copied union `UnstableShape`, its two variant types, fields and constructors;
    * the added field `Caps.N`;
    * the added variant `Pick.N`, with its type and constructor;
    * `UnstableNewRequest`, `UnstableNewResponse` and `AgentMethodNew`;
    * `AgentExperimental.UnstableNew`, `(*ClientSideConnection).UnstableNew` and `AgentExperimental` itself.
  * The five accepting 2c cases gain `want.deprecated` by `-update`. Each diff only adds that section, and no `want.api` changed.
* **Planted breaks:** each was planted in its own copy of the tree through a scratch script, and each failed as required:

  | Break | Failing line |
  | :--- | :--- |
  | 2d's row: one field's notice not emitted | `--- FAIL: TestEngineCases/accept-unstable-deprecated`, missing `field Caps.N: Deprecated: …`; `main_test.go:64: types_gen.go is out of date` |
  | 2d's row: `example/agent` as it was before 2d | `make lint`, exit 2: `example/agent/main.go:41:4: SA1019: github.com/maccavelli/acp-go-sdk2.AgentExperimental is deprecated: …`, 24 reports per OS |
  | a method constant's notice not emitted | `accept-unstable-deprecated`, missing `const AgentMethodNew: Deprecated: …` |
  | an interface marked when one of its methods is frozen | `frozen_test.go:61: AgentExperimental is marked, but one of its methods is not frozen` |
  | a schema mark replaced by the frozen one | `frozen_test.go:51: Marked: mark &{Note: Definition: Frozen:true}, want mark true frozen false` |
  | live unstable types marked | `frozen_test.go:51: Live: mark &{… Frozen:true}, want mark false frozen false` |

  The first attempt at the `example/agent` row restored only the assertion, without its 15 methods. It failed as a type error,
  not SA1019. The plant was corrected to the pre-2d file from `HEAD`.
* **Checks**, each exit 0:
  * `make pre-add-check`: "188 file(s) clean";
  * `(cd cmd/generate && go test -count=1 ./...)`; `go test -count=1 ./...` at the root;
  * `make lint`;
  * `tools/gates.py --all`, with the Linux and Windows hosts reached over SSH: all 18 `ok`. `newfiles` covered `frozen.go` and
    `frozen_test.go`, and `api` reported "0 incompatible, 0 compatible" against `v1.22.0`;
  * `make disclosure-check`; `tools/idscan.py --paths` over the 18 changed and new files: 0 matches.

### 2e, the API gate, 2026-10-25

* **`scripts/check_api.py`:**
  * `REPORTED`, empty until 2f adds `unstable`, sits beside `PACKAGES`, the gated packages.
  * The comparison is `compare(root, base, gated, reported)`, and `main` calls it with the repository's root and the two tuples,
    so a test can point it at a scratch module. `module_path` and `latest_release` take the root.
  * A reported package's incompatible changes are printed as `check_api: incompatible, reported, not gated: <pkg>: <change>`,
    with a summary line of their own. Its compatible changes are listed with the others, and only a gated package's
    incompatible change gives exit 1.
  * A package in both tuples is a tool error, exit 2.
  * The report of the tree is unchanged: "check_api: against v1.22.0: 0 incompatible, 0 compatible".
* **`scripts/test_check_api.py`**, new, its first tests. Each builds a scratch module, with this repository's `go` line, in a git
  repository tagged `v1.9.0` and `v1.10.0`, and runs the pinned `apidiff` through `compare`. They cover:
  * the newest tag by number (`v1.10.0`);
  * a compatible change, exit 0;
  * an incompatible change in a gated package, exit 1;
  * the same removal in a reported package, exit 0 and listed;
  * the same removal gated, exit 1;
  * a package new since the tag, compatible;
  * the two tool errors, a package in both tuples and one missing from the tree.

  They need Go, and the network only until `apidiff` is in the module cache. The CI job that runs `make check-tools`, the Linux
  non-floor one, already runs `go run` for actionlint.
* **`acptest`** joins `PACKAGES` in phase 3, as this sub-step says.
* **Planted breaks:** each was planted in its own copy of the tree through a scratch script, and each failed as required:

  | Break | Failing line |
  | :--- | :--- |
  | 2e's row: a reported package's incompatible change fails | `FAIL: test_incompatible_change_in_a_reported_package_is_listed_and_passes` |
  | 2e's row: a gated package's incompatible change passes | `FAIL: test_incompatible_change_in_a_gated_package_fails`, `FAIL: test_the_same_change_gated_fails` |
  | a package in both tuples accepted | `FAIL: test_tool_errors (… name='in both lists')` |
  | tags ordered as strings | `FAIL: test_newest_tag_by_number` |

  2f's row, `unstable` added to `PACKAGES` and gated, waits for 2f, which adds the package.
* **Checks**, each exit 0:
  * `make check-tools`: 53 tools tests, 30 scripts tests;
  * `make api`: "check_api: against v1.22.0: 0 incompatible, 0 compatible";
  * `tools/gates.py --all`, with the Linux and Windows hosts reached over SSH: all 18 `ok`;
  * `make disclosure-check`; `tools/idscan.py --paths` over the two scripts: 0 matches.

  No Go file changed.

### Deviation, 2026-10-25: 2f, what `unstable` may use of `acp`

* **Found** while reading 2f's inputs, before any 2f code. Generated type code uses three unexported names that a sibling
  package cannot reach:
  * the `msg` field of `ValidationError` (`internal/emit/types.go`, `validationError`), with `ValidationError` at
    `errors.go:99`–`:105`;
  * the `validate(abs)` method a `Validate` calls on a nested value (`validateCall`, `addValidate`);
  * the `decodeKeys` method a union calls on a variant (`decodeChosen`).

  The text of 2f did not name them, nor the names `unstable` declares.
* **Decided** by the owner, the MADR's questions 9 and 10: `acp`'s exported API only, and the schema key as the name. Not
  chosen: exported hooks in `acp`; a validation runtime of `unstable`'s own; the `Unstable` prefix. 2f is amended in place.
* **No scope added:** the files 2f names are the ones it changes.

### 2f, package `unstable` and its emission, 2026-10-25

* **`load/unstable.go`, new.** `NewUnstable` and `Unstable`, as the amendment defines them.
  * New definitions are the live merge's unstable-only keys, minus the stable and frozen keys.
  * New variants are the live entries of a stable union whose key the `acp` union lacks.
  * New methods are the live unstable wires the manifest lacks.
  * A new key that is an `acp` name is refused.
* **`model`:** `Check.Type`, the validated type of a nested check, so `emit` can tell an `acp` type.
* **`emit`:**
  * `File` gains `Dir` and `Path`, and `Dirs` lists the package directories.
  * `goFile` gains `root`, `local` and import aliases, with `typeName`, `runtimeName` and `rootName`. `rootName` writes `acp`
    and imports the root under that name when the path's last element differs, as it does for the module path and a behaviour
    case's `enginetest/<case>`.
  * `typeAST`, the union, member and constructor code, `validationError` (no `msg` from `unstable`) and `validateCall`
    (`Validate` for an `acp` type) go through them. A file of `acp` is unchanged: every generated file is byte-identical to
    `HEAD`.
  * `unstable.go`, new: `RenderUnstable`, which writes `unstable/unstable_gen.go`, and `emitDecodeVariant`.
  * *Against the text:* `Render` keeps its signature, and `RenderUnstable` renders the second package. 2f had `Render` return
    both.
* **`main.go`:**
  * `rootImportPath`, the module path of the `go.mod` above `-out` and the relative directory, or `acp`.
  * `renderUnstable`, which renders nothing without a manifest, without the live files, or when they add nothing.
* **`output`:**
  * `Check(dir, root, files)` checks the root as before, then `unstable` through `servingImporter`, which returns the
    just-checked root for its path.
  * `Write` puts each file in its package's directory, making `unstable/` when needed. Once every file is renamed, it removes a
    `*_gen.go` or `*_gen_test.go` a directory of `Dirs` holds and the run did not render.
* **`enginetest`:** `LoadPackageImporting`, with the same serving importer.
* **`engine_test.go`:**
  * A case may have `want.api.unstable` and `want.deprecated.unstable`, written by `-update`.
  * A case without the first must leave no `unstable/`.
* **`unstable/doc.go`**, hand-written: the package's purpose, its stance in `golang.org/x/exp`'s words, what is emitted, the move
  to `acp` under the same name, and question 9's two differences. At `1.25.0` it is the only file.
* **Tests:**
  * `TestGeneratedFilesUpToDate` and `TestCheckAcceptsRepository` render through `repositoryFiles`, as `run` does, `unstable`
    included. The first requires no generated file under `unstable/`.
  * `TestRootImportPath`: a `go.mod`, nested under one, none, and one with no `module` line.
  * `TestWritePackageDirectories`: `unstable/` made, a stale `*_gen.go` and `*_gen_test.go` removed, and hand-written files and
    another directory kept.
  * `scripts/test_check_api.py`: `test_unstable_is_reported_and_not_gated`.
* **Engine cases,** each written by a scratch script that freezes its manifest with `-write-unstable-manifest`:
  * `accept-unstable-standalone`, `-method-types` (a `behaviour_test.go`), `-field-nowhere` and `-empty`. Each takes
    `accept-unstable-merge`'s `want.api`, and a scratch script confirmed that it is unchanged after `-update`.
  * `accept-unstable-variant-decode` (a `behaviour_test.go`), with its own schemas: `acp`'s `Pick` keeps only `A` and `Unknown`.
  * `refuse-unstable-new-name-taken`. Its live files drop `x/new`. With them kept, the live merge itself refuses the copy first:
    "cannot merge unstable schema: "UnstableExtra" already exists".

  The two behaviour tests are external, `package acp_test`, since an in-package test cannot import `unstable`.
  `TestEngineBehaviour` requires each package to run a test. Each of question 9's rules is pinned:
  * `method-types` validates `PingRequest`: `&acp.ValidationError{…}` without `msg`, whose text is "PingRequest: label:
    minLength", and `v.Old.Validate()` on `acp`'s `OldRequest`;
  * `standalone`'s `Shape` decodes `acp`'s `OldRequest` with `json.Unmarshal`, and its own `Corner` with `decodeKeys`.

  The listings were written by `-update` and read:
  * `Spot`, `Level` with two constants, `Shape` with `NewShapeOldRequest` and `NewShapeCorner`, and `Corner`;
  * `AgentMethodPing`, `PingRequest` and `PingResponse`;
  * `DecodePickN` and `PickN`.
* **`scripts/check_api.py`:** `REPORTED = ("unstable",)`. `make api` prints "check_api: against v1.22.0: 0 incompatible, 0
  compatible" and "check_api: reported packages unstable: 0 incompatible, not gated". *Against the text:* 2f expected
  `unstable` listed as a new package. With `doc.go` alone it declares nothing, so `apidiff` lists nothing for it.
* **Docs:**
  * `README.md` gains "Stability".
  * `docs/architecture.md` gains "The unstable boundary": the merge and its provenance, the manifest, the deprecation source and
    package `unstable`. Its opening paragraph, the API gate's bullet and the tree name `unstable/`, `unstable-v1.json` and
    `check_api.py`.
  * `RELEASING.md`: a bump never rewrites the manifest, and `make api`'s report for `unstable` is read and recorded in the bump's
    record.
* **Planted breaks:** each was planted in its own copy of the tree through a scratch script, and each failed as required:

  | Break | Failing line |
  | :--- | :--- |
  | 2f's rows, a new standalone definition and a new variant emitted into `acp` (`acp` built from the live merge) | `--- FAIL: TestEngineCases/accept-unstable-standalone`, `…/accept-unstable-variant-decode`, `…/accept-unstable-method-types` |
  | 2f's row: `unstable` imports the root under a fixed path | `--- FAIL: TestEngineBehaviour/generate/accept-unstable-method-types`; `--- FAIL: TestEngineCases/accept-unstable-standalone` |
  | 2f's row: `output.Check` type-checks `unstable` against the source on disk | `accept-unstable-standalone`: "generated code does not type-check" |
  | 2f's row: a stale `*_gen.go` kept | `write_test.go:288: …/unstable/old_gen.go: Stat = <nil>, want it removed` |
  | 2f's row, `accept-unstable-empty`: `unstable/` always made | `engine_test.go:47: the run made unstable/ and generated no package unstable` |
  | 2f's row: `unstable` in `PACKAGES`, gated | `FAIL: test_unstable_is_reported_and_not_gated` |
  | a nested output directory's path ignored | `main_test.go:362: …/a/b: "example.invalid/m", <nil>; want "example.invalid/m/a/b"` |
  | a new key that is an `acp` name accepted | `refuse-unstable-new-name-taken`: `run accepted the schema` |
  | an `acp` value checked with `validate(abs)` | `unstable_gen.go:70:18: v.Old.validate undefined (… acp.OldRequest has no field or method validate …)` |
  | `msg` set from `unstable` | `unstable_gen.go:68:4: cannot refer to unexported field msg in struct literal of type acp.ValidationError` |
  | an `acp` variant decoded with `decodeKeys` | `unstable_gen.go:88:17: v.decodeKeys undefined (cannot refer to unexported method decodeKeys)` |
  | `Decode…` without its discriminator check | `behaviour_test.go:27: DecodePickN decoded a payload of the a variant` |

  The stale-file row fails only the `output` test, since an engine case writes into a new directory. `accept-unstable-empty` is
  shown by the row after it. The first attempt at the on-disk row did not compile (`pkg` left unused), and it was corrected to
  serve no path.
* **Fixes the checks asked for:**
  * `gocognit` refused `runEngineCase` at 51, so `requireUnstable` was split out;
  * `gofmt` on `check.go`;
  * staticcheck QF1001 on `write.go`'s condition.
* **Checks**, each exit 0:
  * `make pre-add-check`: "190 file(s) clean";
  * `(cd cmd/generate && go test -count=1 ./...)`; `go test -count=1 ./...` at the root, `unstable` included;
  * `make lint`: "0 issues." on both OSes;
  * `make check-tools`: 53 tools tests, 31 scripts tests;
  * `make markdownlint`; `make check-records`;
  * `tools/gates.py --all`, with the Linux and Windows hosts reached over SSH: all 18 `ok`. `newfiles` covered
    `emit/unstable.go`, `load/unstable.go` and `unstable/doc.go`;
  * `make disclosure-check`; `tools/idscan.py --paths` over the 28 changed and new files: 0 matches.

  This closes phase 2.

### Deviation, 2026-10-25: phase 3, the check a cleanup without `Wait` fails

* **Found** by the planted break. Phase 3's table gives "`Connect`'s cleanup does not `Wait`" to the leak guard in `TestMain`.
  Planted in a scratch copy, the cleanup that closes and does not wait passes every test, guard included: after `Close`, the
  pair's goroutines end on their own, and the goroutineleak profile counts only a goroutine blocked forever. The guard sees a
  cleanup that does not close, which was planted as well: "goroutines leaked by the tests".
* **Resolved** by a test of what `Wait` adds. `TestConnectCleanupWaits` runs a pair in a subtest whose agent's `session/new`
  handler runs until its context ends, then takes 100 ms to return. After the subtest, the test requires the handler to have
  returned. The row's check is now that test. No code, scope or decision changes; the table row is amended in the record below.

### Phase 3, package `acptest`, 2026-10-25

* **`acptest/`, new:**
  * `doc.go`;
  * `pair.go`: `Connect(tb testing.TB, agent, client, opts...)` and `Pair` with `Agent`, `Client`, `Close` and `Wait`. Its
    cleanup closes both, and fails the test when either side's `Wait` gives an end that is neither `ErrConnectionClosed` nor
    `ErrPeerDisconnected`. The parameter is `tb`, as `thelper` requires; the name is not API;
  * `abs.go`: `Abs`. It returns a host-absolute path cleaned, and any other path under `os.TempDir()`, as `testAbsPath` does;
  * `recorder.go`: `Recorder` with `Writer(io.Writer) io.WriteCloser`, `Reader(io.Reader) io.ReadCloser` and `Frames()`.
    * A newline-ended line that is one JSON value is re-encoded by `encoding/json` v1, from `UseNumber`; any other line is kept
      as it was.
    * Each wrapper forwards `Close` to what it wraps, so a connection's `Close` still ends the pipe.
  * `script.go`: `ScriptAgent`, embedding `AgentRefusals`, and `Turn`.
    * It receives its connection through `AgentConnAware`.
    * `NewSession` answers `session-1`, `session-2` and so on, and a prompt past the script is `-32603`.
    * `Authenticate`, `Cancel`, `SetSessionMode` and `SetSessionConfigOption` do nothing.
  * `client.go`: `MemoryClient`, embedding `ClientRefusals`, with `SetFile`, `File`, `SetPermission` and `Updates`.
    * `line` is 1-based, and `line` 0 is `-32602`. A line past the last answers no content.
    * A missing path is `NewResourceNotFound`, `-32002`, and a tool call with no scripted outcome is cancelled.
* **Tests** in `package acptest_test`:
  * `leak_test.go`: `TestMain` with a copy of the root's guard, bound 7 s, the root's drain timeout and two seconds;
  * `TestConnectRoundTrip`, `TestConnectCleanupWaits`, `TestAbs`, `TestRecorder`, `TestScriptAgent` and `TestMemoryClient`;
  * `ExampleRecorder`, run, whose output is the canonical `initialize` request and response;
  * `ExampleConnect`, compiled and not run, since an example has no `*testing.T`; `TestConnectRoundTrip` runs the same calls.

  Each passes 30 times under `-race`, without JSON v2 (the `nojsonv2` gate), at `go1.26.10` (the `floor` gate), and on the
  Linux and Windows hosts.
* **`scripts/check_api.py`:** `PACKAGES` gains `acptest`, and `test_unstable_is_reported_and_not_gated` asserts it. `make api`:
  "check_api: against v1.22.0: 0 incompatible, 7 compatible", the seven being `acptest`'s `Abs`, `Connect`, `MemoryClient`,
  `Pair`, `Recorder`, `ScriptAgent` and `Turn`, all added.
* **Docs:**
  * `README.md` gains "Testing with acptest";
  * `docs/architecture.md`'s opening paragraph describes the package, its API gate bullet names it, and the tree lists it.
* **Planted breaks:** each was planted in its own copy of the tree through a scratch script, and each failed as required:

  | Break | Failing line |
  | :--- | :--- |
  | the row's: `Connect`'s cleanup does not `Wait` (by the deviation above) | `acptest_test.go:104: Connect's cleanup returned while a handler of the pair was still running` |
  | `Connect`'s cleanup does nothing | "goroutines leaked by the tests"; "the pair was not closed" |
  | the row's: `Recorder` returns the frames it holds | `acptest_test.go:114: frames ["X\"a\":…" "changed" "not json"], want …` |
  | the row's: `line` read as 0-based | `acptest_test.go:203: first line: "two\nthree", <nil>; want "one\ntwo\nthree"` |
  | the row's: a missing path answered `-32603` | `acptest_test.go:210: a missing path: {"code":-32603,…}, want resource not found` |
  | the row's: `Abs` returns its argument unchanged on Windows, run on the Windows host | `acptest_test.go:132: Abs("/work/a.txt") = "\\work\\a.txt", not absolute on this host`; `:138: … want it under C:\Users\<user>\AppData\Local\Temp` |
  | frames kept as written, not canonical | `--- FAIL: TestRecorder`, `--- FAIL: ExampleRecorder` |
  | `Initialize` drops the capabilities | `acptest_test.go:152: Initialize: …` |
  | a recorder's `Close` not forwarded | `TestRecorder` fails: the read after `Close` does not end |

  The first attempt at `TestConnectCleanupWaits` hung. `authenticate` for a method the agent did not offer is refused by the
  client's gate before it is sent, and a cleanup registered after `Connect`'s ran first. It now uses `session/new`, selects on
  the handler's start or the call's end, and waits for the call after the subtest, for at most 10 s.
* **Checks**, each exit 0:
  * `make pre-add-check`: "193 file(s) clean";
  * `make lint`: "0 issues." on both OSes, after `thelper`'s two findings;
  * `make check-tools`: 53 tools tests, 31 scripts tests;
  * `make markdownlint`; `make check-records`;
  * `tools/gates.py --all`, with the Linux and Windows hosts reached over SSH: all 18 `ok`. `newfiles` covered the nine files of
    `acptest/`;
  * `make disclosure-check`; `tools/idscan.py --paths` over the 13 changed and new files: 0 matches.

### P1 changed, 2026-10-25

After phase 3, the owner: "we will do 1 release when done". The MADR records it under P1, and phase 8 and "Rollout" are amended in
place. No release was cut after phase 3.

### Phase 4, `PresentNotice` and `PresentCompaction`, 2026-10-25

* **`helpers.go`:** `PresentNotice`, `PresentCompaction` and `PresentCompactionSummary`, package-level, each taking the connection
  after the context, and `present`, which they share.
  * Before `initialize` (`Negotiated()` reports none), each sends its variant through `SessionUpdate`, which refuses with
    `ErrNotInitialized` and writes nothing. A nil connection is `errNilConnection`.
  * With the capability in the client's `initialize` (`Session.Notices`, `Session.Compaction`), the variant is sent, built from the
    definition type, `Meta` included.
  * Without it, the fallback is `UpdateAgentMessageText`:
    * for a notice, the title, then a newline and the description when it is set and not empty;
    * for a compaction, "Compaction <status>", then a newline and the error when it is set and not empty;
    * for a summary chunk, nothing, and nil is returned.

  The parameter is `sessionID`, as revive's `var-naming` requires; the name is not API.
* **`present_test.go`**, `package acp_test`, over `acptest.Connect` and `MemoryClient`. A final agent message marks the end of what
  was sent, so a test reads every update before it, in order.
  * `TestPresentNotice`: with `session.notices`, the variant; without it, the two fallback texts, with and without a description.
    `UpdateNotice` sent directly without it is a `*CapabilityError`.
  * `TestPresentCompaction`: with `session.compaction`, the three terminal statuses (`failed` with its error) and the summary
    chunk as variants; without it, the three as "Compaction completed", "Compaction failed\nout of memory" and "Compaction
    cancelled", and the summary as nothing.
  * `TestPresentBeforeInitialize`: each of the three is `ErrNotInitialized` with no frame written (an `acptest.Recorder` on the
    agent's writer), and a nil connection is an error.
  * `ExamplePresentNotice`, run: the fallback of a notice with a description.
* **Docs:** `README.md`'s agent section, and the helpers line of `docs/architecture.md`.
* **Planted breaks:** each was planted in its own copy of the tree through a scratch script, and each failed as required:

  | Break | Failing line |
  | :--- | :--- |
  | the row's: `PresentNotice` reads `Session.Compaction` | `present_test.go:85: with session.notices: …`; `:90: session/update: capability session.notices not advertised: variant notice` |
  | the row's: the fallback drops the description | `present_test.go:99: without session.notices: ["…Quota low…" …]`; `--- FAIL: ExamplePresentNotice` |
  | the row's: `PresentCompactionSummary` falls back to an agent message | `present_test.go:158: without session.compaction:`, its got ending in the agent message `"summary"` |
  | the row's: `PresentNotice` sends before `initialize` (through `Connection().SendNotification`) | `present_test.go:195: PresentNotice before initialize: <nil>, want ErrNotInitialized`; `:202: the agent wrote 3 frames before initialize` |
  | a nil connection accepted | `TestPresentBeforeInitialize` panics, at `present_test.go:185` |

  Two of the first runs failed on the right case, but missed their expected text: the message reads "capability … not
  advertised", and the log escapes the JSON. The expected text was corrected; the checks were not changed.
* **Checks**, each exit 0:
  * `make pre-add-check`: "202 file(s) clean";
  * `make lint`, after revive's `var-naming` and modernize's `newexpr`;
  * `go test -count=1 ./...`;
  * `make markdownlint`; `make check-records`;
  * `tools/gates.py --all`, with the Linux and Windows hosts reached over SSH: all 18 `ok`. `newfiles` covered `present_test.go`,
    and `api` reported "0 incompatible, 10 compatible": the three functions, and `acptest`'s seven;
  * `make disclosure-check`; `tools/idscan.py --paths` over the six changed and new files: 0 matches.

### Deviation, 2026-10-26: phase 3's `Recorder` reorders frames

* **Found** by phase 5's first `tools/gates.py --all`: the `linux` gate failed, `--- FAIL: ExampleRecorder`, in `acptest`,
  which phase 3 committed and phase 5 does not touch.
* **Cause.** A recording writer recorded a frame only after the wrapped `Write` returned. A pipe's `Write` returns once the peer
  has read the frame, and by then the peer may have answered and its own writer recorded the answer, so the answer can be
  recorded first.
* **Measured** before the fix, each in the session's scratch directory:
  * 3,000 runs of the example passed locally, and 3,000 on the Linux host: the window is narrow.
  * In a scratch copy with a 50 ms pause between `Write` returning and the frame being recorded, the example fails every time,
    with the `result` frame before the `initialize` frame.
  * The same copy with the write recorded first, the pause kept after the write, passes.
* **Pre-existing:** yes, since phase 3's commit.
* **Decided** by the owner, from three resolutions: "Record before writing". Not chosen: holding the recorder's lock across the
  wrapped `Write`; the same fix staged alone before phase 5. The fix is in phase 5's change set.
* **The fix.** `recordingWriter.Write` records the bytes it is given, then writes them. Its doc and `Writer`'s say a frame whose
  write fails is recorded too. Readers are unchanged: they record what a read returned, which the peer wrote first.
  `TestRecorderOrdersFramesAsSent`, new, has the wrapped writer answer through a second recording writer before it returns, and
  requires the ask before the answer.
* **Scope added to phase 5:** `acptest/recorder.go` and `acptest/acptest_test.go`.

### Phase 5, `CompactionTimeline`, 2026-10-26

* **`compaction.go`**, new, in the root: `CompactionEntity` and `CompactionTimeline`, with `Apply`, `Get` and `Open`.
  * `CompactionEntity` has the fields the MADR names. The MADR does not define `Seen`; it is defined as true once a
    `compaction_update` for the id has arrived, and false for an entity known only from summary chunks.
  * `Id` keeps the MADR's spelling, which the generated types use (`CompactionUpdate.CompactionId`), under a
    `//nolint:revive // var-naming` that says so.
  * `Apply`'s rules:
    * a first-seen id starts `in_progress`, with nothing else set;
    * a `compaction_update` sets the status, an unknown one kept as it arrived;
    * each patch, absent, `null` or a value, keeps, clears or replaces its field, and an empty `summary` clears it;
    * a `compaction_summary_chunk` appends, first-seen-initializing an unknown id;
    * any other update gives the zero entity and `false`.
  * `Open` lists the ids neither `completed`, `failed` nor `cancelled`, in first-seen order. Entities returned are copies.
  * The zero timeline is ready and safe for concurrent use.
* **`cmd/generate/engine_test.go`:** `TestRuntimeFilesCoverHandWritten` names `compaction.go`. Before the edit, the test failed
  on the tree as phase 5 left it: "hand-written files of the root are [compaction.go connection.go …], want [connection.go …]".
* **Tests**, `compaction_test.go`, `package acp_test`:
  * `TestCompactionTimeline`, nine sequences: first seen; each patch set, omitted and `null`; an empty summary; chunks in order;
    a concrete summary replacing chunks; a chunk first seen; an unknown status;
  * `TestCompactionTimelineOpen`: `Open`'s ids and order, a non-compaction update, an id never seen;
  * `TestCompactionTimelineCopies`;
  * `TestCompactionTimelineReplay`, over `testdata/compaction/lifecycle.jsonl` and `clear-and-unknown.jsonl`, one
    `session/update` params object a line, each applied live and again into a fresh timeline. The entities are equal, and
    equal to the expected JSON.
* **Docs:**
  * `README.md`'s client section;
  * `docs/architecture.md`'s hand-written list names `compaction.go`. Its "None of them holds protocol state" now excepts the
    compactions a timeline is fed, and says none holds a connection's state.
* **Planted breaks:** each was planted in its own copy of the tree through a scratch script, and each failed as required:

  | Break | Failing line |
  | :--- | :--- |
  | the row's: `summary: null` treated as absent | `--- FAIL: TestCompactionTimeline/each_patch_null_clears_it`: `summary ["s"], want []`; `…Replay/clear-and-unknown.jsonl` |
  | the row's: a concrete `summary` appended | `--- FAIL: …/a_concrete_summary_replaces_the_chunks`: `summary ["one" "two" "final"], want ["final"]` |
  | the row's: an unknown status replaced by `in_progress` | `--- FAIL: …/an_unknown_status_is_kept`: `entity {… Status:in_progress …}, want {… Status:summarizing …}` |
  | the row's: `Open` omits unknown statuses | `compaction_test.go:141: Open = ["z" "chunk"], want ["z" "odd" "chunk"]` |
  | the row's: `compaction.go` not named in `TestRuntimeFilesCoverHandWritten` | `engine_test.go:220: hand-written files of the root are [compaction.go …]` |
  | entities shared with the timeline | `compaction_test.go:164: the timeline shares what Apply returned` |
  | a chunk first seen dropped | `--- FAIL: …/a_chunk_first_seen_starts_the_compaction`; `Open = ["z" "odd"]` |
  | an empty summary kept | `--- FAIL: TestCompactionTimelineReplay/clear-and-unknown.jsonl` |
  | the deviation's: a frame recorded after its write | `acptest_test.go:211: frames ["{\"answer\":1}" "{\"ask\":1}"], want ["{\"ask\":1}" "{\"answer\":1}"]` |

  Phase 3's `frames-aliased`, `frames-not-canonical` and `recorder-close-dropped` were planted again against the changed
  `Recorder`, and each failed as before.
* **Checks**, each exit 0:
  * `make pre-add-check`: "203 file(s) clean";
  * `make lint`;
  * `go test ./...`; `(cd cmd/generate && go test ./...)`; the recorder's tests 500 times under `-race`;
  * `make markdownlint`; `make check-records`;
  * `tools/gates.py --all`, with the Linux and Windows hosts reached over SSH: all 18 `ok` on the second run. The first failed
    `linux`, as the deviation above records. `newfiles` covered `compaction.go` and `compaction_test.go`, and `api` reported
    "0 incompatible, 12 compatible";
  * `make disclosure-check`; `tools/idscan.py --paths` over the changed and new files: 0 matches.

### Phase 6, `ExtensionMux` and the `_meta` helpers, 2026-10-26

* **`extensions.go`**, a runtime file, uses no ACP type:
  * `ExtensionMux`, with `HandleExtensionMethod` and `Methods`, and the package functions `Handle[P, R]` and `Notify[P]`.
  * Registration panics, with an `acp: ` message, on a name `validateExtensionMethodName` refuses (empty, or without `_`), on
    a name already registered by either function, and on a nil function.
  * Params are decoded by `decodeParams`, absent ones as `{}`, as the generated dispatch decodes them. A decode failure is
    `NewInvalidParams` with the `error` field. A function's error is returned as it is, so `handleExtension`'s `toReqErr`
    passes a `*RequestError` through and makes anything else `-32603`.
  * Routing is by `IsNotification(ctx)`:
    * a request for an unregistered name, or a `Notify` name, is answered `NewMethodNotFound`;
    * a notification for an unregistered name, or a `Handle` name, returns nothing and runs nothing;
    * a call not made by a connection is taken for a request.
  * The zero mux is ready, and it is safe for concurrent use.
  * `TraceMeta` returns a non-nil map of the non-empty keys. `AdvertiseMeta` copies `meta` shallowly into a new map, `nil`
    included, and sets `namespace` only when it is non-empty.
* **`connection.go`:** `IsNotification(ctx)`, which returns `sentAsNotification(ctx)`.
* **Tests**, `extensions_mux_test.go`, `package acp_test`, over `acptest.Connect` with a `ScriptAgent` and a `MemoryClient`,
  logging at Warn and above to a buffer:
  * `TestExtensionMuxRequests`: a typed answer; a mismatch answered `-32602`; a handler's `-32050` passed through; an
    unregistered `_` request answered `-32601`; nothing logged.
  * `TestExtensionMuxNotifications`: notifications to a `Notify` name, an unregistered name and a `Handle` name, then a
    request.
    * Only the `Notify` handler runs, and the mux returns nothing for the other two.
    * A request to the `Notify` name is answered `-32601` and does not run it.
    * Nothing is logged.
  * `TestIsNotification`: true in the typed `Cancel`, false in the typed `NewSession` and in the test's own context.
  * `TestExtensionMuxDirect`, `TestExtensionMuxRegistration` (eight panics; `Methods` sorted before and after them),
    `TestTraceMeta` and `TestAdvertiseMeta`.
  * The notification cases check what the mux returned, which a wrapper around it records. The wire and the log cannot show
    it: `connection.go:1723` drops a `-32601` answer to a `_` notification without logging it. So the row's break, an
    unregistered notification answered, is caught only by that check.
  * The first `make pre-add-check` failed `TestExtensionMuxNotifications` on a defect in the test. It kept one answer per
    method, so the synchronizing request to `_test/echo` could overwrite the notification's answer: requests and notifications
    run concurrently, and only a response waits for earlier notifications. Answers are now kept per method and kind. The tests
    then passed 50 times under `-race`.
* **Docs:**
  * `README.md`'s "Extension methods": `ExtensionMux`, `Handle`, `Notify`, `IsNotification` and `Methods`; `AdvertiseMeta`
    and `TraceMeta` in the `_meta` subsection.
  * Not named by the step: `docs/architecture.md`'s hand-written list gives `extensions.go`'s new names. Its account of the
    generated dispatch says `IsNotification` exports the mark. Without them, phase 7's reading of it against the code would
    find it out of date.
* **Planted breaks:** each was planted in its own copy of the tree through a scratch script, and each failed as required:

  | Break | Failing line |
  | :--- | :--- |
  | the row's: `IsNotification` always false | `extensions_mux_test.go:223: the ping handler ran with [], want ["note _test/ping"]` |
  | the same, against the typed handlers | `IsNotification in the typed handlers was map[session/cancel:false session/new:false], want map[session/cancel:true …]` |
  | the row's: an unregistered notification answered | `:210: the mux answered the notification _test/unknown with (<nil>, {"code":-32601,…}), received true, want nothing` |
  | the row's: `AdvertiseMeta` writes into its input | `:345: AdvertiseMeta changed its input to map[other:1 zed.dev:map[…]], want map[other:1]` |
  | the row's: `Handle` accepts a name without `_` | `:311: Handle without _: panicked with "", want an acp: panic containing "must start with '_' …"` |
  | a `Handle` name run for a notification | `:214: the echo handler ran with ["note _test/echo" "sync"], want only ["sync"]` |
  | a name registered twice | `:311: Handle of a Handle name: panicked with "", …` |
  | a decode failure not `NewInvalidParams` | `:172: a params mismatch answered -32603, want -32602` |
  | a handler's `*RequestError` wrapped | `:176: a handler's *RequestError answered -32603, want -32050` |
  | `Methods` unsorted, run 20 times | `:293: Methods is ["_b" "_a" "_c"], want ["_a" "_b" "_c"]` |
  | `TraceMeta` keeps empty values | `:332: TraceMeta("00-trace-span-01", "", "user=a") = map[… tracestate:], want …` |

* **Checks**, each exit 0:
  * `make pre-add-check`: "205 file(s) clean";
  * `make markdownlint`; `make check-records`;
  * `tools/gates.py --all`: 16 gates `ok` on the first run. `windows` and `linux` failed there before running anything,
    because the host variables were not set in that shell ("ACP_WINDOWS_HOST is not set"). Rerun with them set, both were
    `ok`, so all 18 passed. `newfiles` covered `extensions_mux_test.go`. `api` reported "against v1.22.0: 0 incompatible, 18
    compatible", the six new names of this phase among them, and `unstable` "0 incompatible, not gated";
  * `make disclosure-check`; `tools/idscan.py --paths` over the changed and new files: 0 matches.
