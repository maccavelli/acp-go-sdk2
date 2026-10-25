---
status: proposed
date: 2026-10-25
associated-madr: "0039-MADR-five-adopter-additions-on-schema-1-25-0.md"
---

# Implement 0039: an unstable boundary, presentation and compaction runtimes, an extension mux, and acptest

Associated MADR: [0039-MADR-five-adopter-additions-on-schema-1-25-0.md](0039-MADR-five-adopter-additions-on-schema-1-25-0.md)

## Goal

The MADR's five additions ship, on the schema `1.25.0` pin, with no schema bump:

1. the unstable boundary: package `unstable`, a manifest of the unstable names `v1` has exported, their `Deprecated:` paragraphs,
   frozen stubs when a schema removes one, and an API gate that reports `unstable` without failing;
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

## Prerequisites

* [0038-PLAN-consumer-helpers-kind-and-agent-refusals.md](0038-PLAN-consumer-helpers-kind-and-agent-refusals.md) is complete
  before phase 2 starts. Phase 3's `ScriptAgent` embeds 0038's `AgentRefusals`, and both PLANs edit `helpers.go`, `README.md` and
  `docs/architecture.md`; running them in turn keeps each commit one step of one PLAN.
* The MADR is `accepted`, with its questions answered.

## Scope

* **Generator, phase 2:** `cmd/generate/internal/model` (origin of each definition, the manifest, the deprecation source, the
  frozen stubs), `cmd/generate/internal/emit` and `cmd/generate/internal/output` (a second output package), `cmd/generate/main.go`,
  engine cases under `cmd/generate/testdata/engine/`, and the regenerated `*_gen.go` files.
* **New committed data, phase 2:** `cmd/generate/internal/model/unstable_v1.json`, the manifest; `schema/unstable-frozen.json`, the
  frozen schema fragments, empty until a schema removes a manifest name.
* **New packages:** `unstable/` (phase 2), `acptest/` (phase 3).
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

Commits 2a to 2e, in order.

**2a. Inventory and re-measure, read-only.**

* List every exported declaration, field and interface method of package `acp` that the generator builds from a definition,
  property, variant or method found only in `schema.unstable.json` / `meta.unstable.json`: its Go name, kind (type, field, method,
  constant, union variant) and schema origin. The model's merge is where origin is known; the listing is a generator flag that
  prints it, added in 2b, or a script over the two schema files until then.
* In a scratch copy, regenerate from schema `1.26.0` and run `scripts/check_api.py` against the newest `v1` tag. Record the counts
  beside 0025's (115 incompatible, 40 stable-named, against `v1.17.1`).

Recorded in the execution record before 2b changes any file.

**2b. The manifest.**

* `cmd/generate/internal/model/unstable_v1.json`: every name from 2a, with kind and origin, sorted. It is what `v1` has exported
  from the unstable schema and must keep.
* `go run ./cmd/generate -unstable-manifest` prints it from the current schema; `TestUnstableManifestCurrent` in `cmd/generate`
  fails when the committed file differs from the output for a schema that removes nothing.

**2c. Deprecation source.**

* The model gains a deprecation source for manifest names, separate from the schema's `deprecated` keyword, so the rule that
  refuses a protocol-table note on an unmarked node stays as it is.
* Each manifest declaration gets a last paragraph: "Deprecated: this name comes from the unstable ACP schema, and stays under v1
  only for compatibility (0039-MADR-five-adopter-additions-on-schema-1-25-0.md)." and, where `unstable` has a counterpart, "Use
  unstable.<Name>." Under question 5's answer, none has one at `1.25.0`.
* `deprecation_test.go` still passes: each notice is its own paragraph starting "Deprecated: ".
* Regenerate, and run `golangci-lint` for the host and Windows. Whether staticcheck's SA1019 reports this package's own uses of the
  newly deprecated names is checked here, not assumed; any report, and any use in `example_*_test.go` (package `acp_test`), is read
  and resolved in this commit without disabling the rule, or recorded as a deviation.

**2d. Emission into `unstable`, under question 5's answer.**

* The output gains a second package, `unstable/`, generated with its own `types_gen.go` and `helpers_gen.go`, and a hand-written
  `unstable/doc.go` stating `golang.org/x/exp`'s stance: names may appear, change or disappear in a minor.
* A definition found only in the unstable schema and absent from the manifest is emitted into `unstable` when it is standalone: a
  type, enum, request or response. A new unstable field on a stable type, method, or variant of a stable union is emitted into
  neither package; for a variant, `unstable` gains `Decode<Union><Variant>(raw json.RawMessage) (<Type>, error)` over the union's
  `Unknown` payload.
* At `1.25.0` the manifest holds every unstable name, so `unstable` holds only `doc.go`, and the root's generated files change only by
  2c's paragraphs. Engine cases prove the rest:
  * `accept-unstable-standalone`: a new standalone unstable definition appears in `unstable`'s API listing and not in `acp`'s;
  * `accept-unstable-field`, `accept-unstable-variant`, `accept-unstable-method`: none reaches `acp`'s API; the variant has its
    `Decode…` in `unstable`, and a payload with it decodes into the union's `Unknown`;
  * `accept-unstable-manifest-kept`: a manifest name the schema still has is generated in `acp` as before.
* `TestGeneratedFilesUpToDate` and `TestCheckAcceptsRepository` cover both packages; the type check of `unstable` includes `acp`.

**2e. Frozen stubs and the API gate.**

* When a manifest name is absent from the schema, the generator builds it from `schema/unstable-frozen.json`, which holds the frozen
  schema fragment of each removed name, with its manifest entry and its 2c paragraph, so `acp`'s API is unchanged. A removed method
  stays in its interface; its dispatch answers `-32601`. `RELEASING.md` gains the bump step: copy each removed definition's fragment
  into `schema/unstable-frozen.json` before regenerating.
* Engine case `accept-unstable-removed-frozen`: a synthetic schema that removes a manifest definition leaves the case's `acp` API
  listing identical.
* `scripts/check_api.py`: `PACKAGES` gains `unstable` and `acptest`. A package missing from the base tag compares against an empty
  one, as now. An incompatible change in `unstable` is listed under "unstable (not gated)" and does not fail; one in `acp`,
  `agentproc` or `acptest` fails, as now. Tests in `scripts/test_check_api.py` over a scratch module.

Docs: `README.md` gains "Stability": the stable `acp` API, the deprecated unstable names, package `unstable`, and the policy;
`docs/architecture.md` describes the manifest, the two packages and the frozen stubs.

Planted breaks:

| Break | The check that must fail |
| :--- | :--- |
| a name dropped from `unstable_v1.json` | `TestUnstableManifestCurrent` |
| one manifest declaration's `Deprecated:` paragraph not emitted | `TestGeneratedFilesUpToDate`, and a model test that every manifest name has one |
| the emitter places a new standalone unstable definition in `acp` | `accept-unstable-standalone` |
| a new unstable field emitted on its stable type | `accept-unstable-field` |
| the frozen fragment ignored for a removed name | `accept-unstable-removed-frozen` |
| `check_api.py` fails on an incompatible change in `unstable` | `test_check_api.py`, unstable not gated |
| `check_api.py` drops `acptest` from `PACKAGES` | `test_check_api.py`, packages |

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

In `helpers.go`: the three methods on `*AgentSideConnection`, taking `Notice`, `CompactionUpdate` and `CompactionSummaryChunk`,
under question 1's answer:

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
* `ExampleAgentSideConnection_PresentNotice`.

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
* `connection.go`: `IsNotification(ctx) bool`, reading the mark the dispatch already sets (question 6's answer).
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
* `TestGeneratedFilesUpToDate`, `TestCheckAcceptsRepository`, `TestRuntimeFilesCoverHandWritten`, `TestUnstableManifestCurrent`
  and the new engine cases pass.
* The root's and each new package's `go test ./...` and `-race`; the `cmd/generate` tests; `make pre-add-check`; `tools/gates.py
  --all` at every code phase, on the Linux and Windows hosts and at the floor.
* `make disclosure-check` and `tools/idscan.py`: nothing found.
* Every planted break in phases 2 to 6 fails its check, each in its own copy of the tree, and the execution record keeps each one's
  failing line.

## Rollout and Rollback

* **Rollout.** Phases 2 to 6 are separate commits, and phase 2 several. Under P1, the first release carries the boundary and
  `acptest`; the second the three runtimes. Each consumer adopts a tag under its own issue.
* **Rollback before a tag:** revert the phase's commits.
* **Rollback after a tag:** a name in `acp`, `agentproc` or `acptest` cannot be removed under `v1`; a defect is fixed forward in a
  patch. A name in `unstable` may be changed in a minor, by the MADR's policy. A broken tag is retracted, as
  [0020-MADR-retract-accidental-v1-tag.md](0020-MADR-retract-accidental-v1-tag.md) did.

## Execution record

Not started: this PLAN is `proposed`.
