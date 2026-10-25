---
status: in-progress
date: 2026-10-25
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

Under P1's answer, after phase 3 and after phase 7. Each through `RELEASING.md`, "Tag and Publish": the release-notes report and its
row committed before the tag; `tools/release_body.py`; the annotated tag and its push; the GitHub release; `tools/publication.py`;
`tools/consumer.py`, extended in that release's commit to use what it adds (`acptest.Connect` for the first; `PresentNotice`,
`CompactionTimeline` and `ExtensionMux` for the second), with `make check-tools`. When the last release is published, this PLAN and
its row are `complete`.

## Verification

* Every item of the MADR's "Confirmation" holds, each by its phase's tests.
* `make api` lists only compatible changes in `acp`, `agentproc` and `acptest`.
* `TestGeneratedFilesUpToDate`, `TestCheckAcceptsRepository`, `TestRuntimeFilesCoverHandWritten`, `TestUnstableManifestComplete`
  and the new engine cases pass.
* After 2c, the regenerated `*_gen.go` files are byte-identical to the committed ones; after 2d, they differ only by the
  deprecation paragraphs (`git diff` on them adds comment lines and nothing else).
* `make lint`, staticcheck's SA1019 included, passes on the whole module, `example/agent` included.
* The root's and each new package's `go test ./...` and `-race`; the `cmd/generate` tests; `make pre-add-check`; `tools/gates.py
  --all` at every code phase, on the Linux and Windows hosts and at the floor.
* `make disclosure-check` and `tools/idscan.py`: nothing found.
* Every planted break in phases 2 to 6 fails its check, each in its own copy of the tree, and the execution record keeps each one's
  failing line.

## Rollout and Rollback

* **Rollout.** Phases 2 to 6 are separate commits, and phase 2 several. Under P1, the first release carries the boundary and
  `acptest`; the second the three runtimes. Each consumer adopts a tag under its own issue.
* **What a consumer meets on the first release.** Every unstable-origin name is deprecated, so a consumer whose staticcheck runs
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
