# Release notes for v1.23.0, against v1.22.0

Observed 2026-10-26 on `ac1e526`, for
[0039-PLAN-five-adopter-additions-on-schema-1-25-0.md](../decisions/0039-PLAN-five-adopter-additions-on-schema-1-25-0.md),
phase 8. This report drafts the text of the GitHub release for `v1.23.0`, as `RELEASING.md` describes it. It decides nothing. It is
committed before the tag, so the tagged tree holds it; the commit that holds it changes only records and `tools/`, so the API it
tags is `ac1e526`'s.

Each statement names its source:

- a record;
- a commit;
- or `make api`, run on `ac1e526` with only `tools/consumer.py` and `tools/README.md` uncommitted, which hold no Go package.

Summary:

- Eighteen additions to the exported API: eleven in the root package and seven in the new package `acptest`; `agentproc`'s is
  unchanged. `make api` against `v1.22.0`: "0 incompatible, 18 compatible". The release is the minor `v1.23.0`, as `RELEASING.md`
  says for compatible changes. The new package `unstable` is reported and not gated, and declares nothing yet.
- What a consumer may meet: nothing the SDK sends or accepts on the wire changes, and no name is removed or changed. Every name
  `acp` took from the ACP unstable schema now carries a `Deprecated:` notice, so staticcheck's SA1019 reports each use. The `go`
  lines and the requirements are unchanged.

The section "Release text" is what goes into the GitHub release.

## Release text

### v1.23.0

The ACP schema is still `1.25.0` (`git diff v1.22.0 -- schema/` adds only `schema/unstable-v1.json`, below). This release adds
five capabilities the SDK's adopters had built for themselves or asked for: a boundary for the unstable schema, capability-aware
presentation, a compaction timeline, a typed extension mux, and a test harness
([0039-MADR-five-adopter-additions-on-schema-1-25-0.md](../decisions/0039-MADR-five-adopter-additions-on-schema-1-25-0.md)).
`make api` reports eighteen compatible additions and no incompatible change. Nothing sent or accepted on the wire changes.

#### Upgrading: the unstable-origin names are deprecated

Package `acp` keeps every name it took from the ACP unstable schema, frozen as `v1.22.0` had them, whatever a later unstable
schema changes or drops. Each now carries the notice "Deprecated: this declaration comes from the unstable ACP schema; it stays
under v1 for compatibility only". A program that uses one still builds and behaves as before. A linter that runs staticcheck's
SA1019 reports each use, implementing `AgentExperimental` or `ClientExperimental` included. The notices are on:

- 133 types: the unstable definitions, such as `UnstableForkSessionRequest`, their inline variant types, and the inline types of
  the variants they add to stable unions;
- 42 fields: those the unstable schema adds to stable types, such as `AgentCapabilities.Nes`, and the variant fields;
- 27 constructors, 21 enum constants, and the 16 method-name constants;
- the 16 frozen methods, on their interfaces and side connections: `UnstableForkSession`, `UnstableMessageMcp` (both sides), the
  five `UnstableDid…Document` notifications, the five `…Nes` methods, and `UnstableListProviders`, `UnstableSetProvider` and
  `UnstableDisableProvider`;
- the interfaces `AgentExperimental` and `ClientExperimental`.

To keep using them without the report, suppress SA1019 for those uses. (`0c8088e`, `e8067a8`.)

#### The unstable boundary

- **`schema/unstable-v1.json`** freezes the unstable-origin API `acp` exports, from schema `1.25.0`. The generator builds `acp`
  from it, so a later unstable schema cannot change or remove an `acp` name. A schema bump never rewrites it. (`04c37eb`,
  `0c8088e`.)
- **Package `unstable`** receives what a later unstable schema adds beyond the manifest: a new definition as a type under its
  schema key, a new method's types and constant, and a new variant of a stable union as a `Decode<Union><Variant>` function over
  the payload `acp` keeps in `Unknown`. A new field of an `acp` type is not emitted. Like `golang.org/x/exp`, the package is
  outside the `v1` compatibility promise. At schema `1.25.0` it holds only its documentation. (`a3a5d5a`.)
- **`make api`** compares `acptest` with the root and `agentproc`, and reports `unstable`'s incompatible changes without failing.
  (`605f485`.)

#### New for agents

- **`PresentNotice(ctx, conn, sessionId, notice)`** sends a notice as the `notice` update to a client that advertised
  `session.notices`, and as an agent message of its title and description to one that did not. **`PresentCompaction`** does the
  same for a compaction's state, with "Compaction <status>" and its error as the fallback; **`PresentCompactionSummary`** sends a
  summary chunk only to a client that advertised `session.compaction`. `UpdateNotice` and the compaction updates sent directly
  are still refused without the capability. (`41f3c38`.)

#### New for clients

- **`CompactionTimeline`** keeps a session's compactions as the protocol's rules apply them, live or replayed by `session/load`:
  a compaction first seen starts `in_progress`; an absent patch keeps a field, `null` clears it, a value replaces it; summary
  chunks append, a concrete `summary` replaces them, `[]` clears it; an unknown status is kept as it arrived. `Get` returns a
  copy of one, and `Open` lists those not known to have ended. (`377107c`.)

```go
// c.timeline returns the session's *acp.CompactionTimeline, one per session.
func (c *myClient) SessionUpdate(ctx context.Context, n acp.SessionNotification) error {
    if e, ok := c.timeline(n.SessionId).Apply(n.Update); ok {
        c.show(e.Id, e.Status, e.Summary)
    }
    return nil
}
```

#### New for both sides

- **`ExtensionMux`** serves `_` extension methods with typed params and results. `Handle[P, R]` registers a request method and
  `Notify[P]` a notification method. Params that do not decode are answered `-32602`. A request for a name not registered with
  `Handle` is answered `-32601`, and a notification for a name not registered with `Notify` is ignored, as the specification
  says. A name without `_`, or one registered twice, panics. **`IsNotification(ctx)`** tells a handler whether the connection
  received the message as a notification. (`78ae7e5`.)
- **`AdvertiseMeta(meta, namespace, features)`** returns a copy of a capability object's `_meta` with your features under your
  namespace, and **`TraceMeta(traceparent, tracestate, baggage)`** the root `_meta` keys the specification reserves for trace
  context, leaving out empty ones. (`78ae7e5`.)

```go
var mux acp.ExtensionMux
acp.Handle(&mux, "_example.com/hello", func(ctx context.Context, p HelloParams) (HelloResult, error) {
    return HelloResult{Greeting: "hello " + p.Name}, nil
})
```

#### New package `acptest`

**`acptest.Connect(t, agent, client)`** joins the two sides over pipes through the public constructors, and closes and waits
for both when the test ends, so no goroutine outlives it. **`ScriptAgent`** plays scripted prompt turns; **`MemoryClient`** serves
files from memory, with 1-based `line` and `limit`, and answers permission requests from a script; **`Recorder`** keeps the frames
a side writes or reads in `encoding/json` v1's canonical form, for goldens; **`Abs`** makes a fixture path absolute on the host. It
needs no dependency and builds without JSON v2. (`d47f386`; `Recorder`'s frame order fixed in `377107c`.)

#### For maintainers

- **`example/agent`** no longer implements `AgentExperimental` or the fifteen unstable methods, so it lints clean under
  SA1019. (`e8067a8`.)
- **Tools.** `tools/consumer.py` builds a module at the Go floor that also uses `CompactionTimeline`, `ExtensionMux` and
  `TraceMeta`, and, in its `go test`, `acptest.Connect` and `PresentNotice`.
- **Tests.** New at the root: `present_test.go`, `compaction_test.go` with `testdata/compaction/`, and
  `extensions_mux_test.go`; `instances_test.go` reads the manifest's frozen schema fragments. `acptest` has its own tests, under
  a goroutine-leak guard. In `cmd/generate`: `manifest_test.go`, `load/manifest_test.go`, `load/changes_test.go`,
  `model/frozen_test.go`, and 19 new engine cases on the unstable boundary.

## Sources not in the release text

- **Commits since `v1.22.0`** (`git log v1.22.0..ac1e526`), fourteen:
  - `04c37eb`, `0c8088e`, `e8067a8` and `a3a5d5a` change the generator, the generated files, `schema/` and `example/agent`;
  - `605f485` changes `scripts/check_api.py` and its tests;
  - `d47f386`, `41f3c38`, `377107c` and `78ae7e5` change the packages, as above; `ac1e526` changes a test and the docs;
  - the other four change records only (`git show --name-only`).
- **Go files changed.** `git diff --name-only v1.22.0 ac1e526 -- '*.go' ':!*_test.go' ':!cmd/**'` lists seventeen: the six of
  `acptest/`, `unstable/doc.go`, `compaction.go`, `connection.go`, `extensions.go`, `helpers.go`, `example/agent/main.go` and the
  five `*_gen.go`, which differ only in comments and the alignment `gofmt` gives them, by the 0039 PLAN's 2d. `go.mod`, `go.sum` and `cmd/generate/go.mod` are
  unchanged (`git diff`).
- **`make api` on `ac1e526`:** exit 0, "against v1.22.0: 0 incompatible, 18 compatible": `AdvertiseMeta`, `CompactionEntity`,
  `CompactionTimeline`, `ExtensionMux`, `Handle`, `IsNotification`, `Notify`, `PresentCompaction`, `PresentCompactionSummary`,
  `PresentNotice` and `TraceMeta`, and `acptest`'s `Abs`, `Connect`, `MemoryClient`, `Pair`, `Recorder`, `ScriptAgent` and `Turn`,
  each "added"; and "reported packages unstable: 0 incompatible, not gated".
- **The notices:** 273 in the five `*_gen.go` files, by the kinds the 0039 PLAN's 2d counts.
- **CI on `ac1e526`:** run `40420748662`, "completed success", six jobs on Ubuntu, Windows and macOS at Go `1.27.3` and `1.26.10`;
  and the Docs workflow's run `40420748538`, "completed success".
- **The evidence:** the 0039 PLAN's phases 2 to 7, their planted breaks, the gates on macOS and on the Linux and Windows hosts, and
  its deviation entries, among them the `Recorder`'s frame order and the extension mux test's sync point.
