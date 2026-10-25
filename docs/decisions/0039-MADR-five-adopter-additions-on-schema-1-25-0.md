---
status: proposed
date: 2026-10-25
decision-makers: Repository owner
---

# Add five adopter capabilities on the schema 1.25.0 pin: an unstable boundary, presentation and compaction runtimes, an extension mux, and acptest

## Context and Problem Statement

This module pins stable ACP schema `1.25.0` (`schema/version`) on protocol version 1
([0004-MADR-pin-acp-schema-1-25-0.md](0004-MADR-pin-acp-schema-1-25-0.md)). The generated v1 surface already speaks every
stable method in `schema/meta.json`, and the hand-written runtime gates, cancels, validates, and ends a connection. Two later
records fill the small protocol helpers the two known consumers had copied
([0036-MADR-session-update-constructors-kind-and-client-refusals.md](0036-MADR-session-update-constructors-kind-and-client-refusals.md),
shipped; [0038-MADR-consumer-helpers-kind-and-agent-refusals.md](0038-MADR-consumer-helpers-kind-and-agent-refusals.md), accepted,
plan still proposed).

What remains is a different class of gap. Schema 1.25.0's release notes stabilize session notices and session compaction, reject
malformed values receivers act on, and add an unstable error stop reason after prompt insertion
(<https://github.com/agentclientprotocol/agent-client-protocol/releases/tag/schema-v1.25.0>, published 2026-10-09). The SDK
generates the types, constructors, and capability gates for those features. It does not implement the runtime contracts the v1
guides state around them: capability-aware fallback, ID-addressed patch and chunk semantics, live-only notice delivery, or a
typed way to use the protocol's own extension mechanisms. The unstable definitions still share the root package with the stable
API, which is the reason [0025-REPORT-schema-1-26-0-adoption-assessment.md](../reports/0025-REPORT-schema-1-26-0-adoption-assessment.md)
advised against adopting schema `1.26.0` under this `v1` module.

The owner asked for five additions that would give adopters significant benefit, with the work future-proofed, capability-enhancing,
and extensible, following established Go SDK practice. This record chooses those five. It does not execute them. A PLAN follows
only after the owner approves this decision.

Findings, from a read-only investigation at `ea09a07` on 2026-10-25, against the committed schema stamp `1.25.0`, the v1 guides and
RFDs at agentclientprotocol.com, the TypeScript and Rust SDK release notes, and Go's module compatibility rule.

### What the 1.25.0 spec is

Stable `schema.json` has 178 `$defs`. Stable `meta.json` lists protocol version 1, 13 agent methods, 11 client methods, and
the protocol method `$/cancel_request`. Unstable `schema.unstable.json` has 277 `$defs`. `go doc -short .` lists 79 exported `Unstable*` identifiers.
`AgentExperimental` and `ClientExperimental` already exist as optional interfaces (`types_gen.go:22521`, `:22691`).

The two features 1.25.0 moved from Preview to Completed, and that this SDK therefore now generates as stable, are:

* **Session notices** ([Session Notices RFD](https://agentclientprotocol.com/rfds/session-notices),
  [v1 prompt turn](https://agentclientprotocol.com/protocol/v1/prompt-turn.md#session-notices)). A fire-and-forget
  `sessionUpdate: "notice"` with required `severity` and non-empty `title`. No ID, no acknowledgement, no replay. v1 Agents
  **MUST NOT** send it unless the Client supplied `clientCapabilities.session.notices: {}`. When that capability is absent, the
  Agent may use an ordinary agent message if the information should still reach the user. Notice support advertises a presentation
  channel, not that any individual notice was seen. Zed, the Claude Agent adapter, and the Codex adapter already gate on the
  capability and keep notices out of history.
* **Session compaction** ([Session Compaction RFD](https://agentclientprotocol.com/rfds/session-compaction),
  [v1 prompt turn](https://agentclientprotocol.com/protocol/v1/prompt-turn.md#session-compaction)). An ID-addressed
  `compaction_update` / `compaction_summary_chunk` pair. The first notification for an ID fixes timeline position; later updates
  patch in place. `summary`, `error`, and `_meta` are omit / `null` / value patches; chunks append; a concrete `summary` replaces
  accumulated content; `summary: []` and `summary: null` clear it. The Agent **MUST** report a terminal status when a reported
  compaction ends, and **MUST NOT** send either variant unless the Client supplied `clientCapabilities.session.compaction: {}`,
  for live delivery and for `session/load` replay. Unknown statuses are preserved. The Claude Agent adapter is the published
  agent-side implementation evidence.

Elicitation is already stable. Clients **SHOULD** validate submitted form values before responding, and Agents **SHOULD** validate
them again ([v1 elicitation](https://agentclientprotocol.com/protocol/v1/elicitation.md)). The SDK's dispatch sends the handler's
answer unchecked.

Extensibility is a first-class v1 mechanism ([Extensibility](https://agentclientprotocol.com/protocol/extensibility.md)): `_`
methods, `_meta` on types that define it, custom capabilities advertised under `_meta` on capability objects, and three root
`_meta` keys reserved for W3C trace context (`traceparent`, `tracestate`, `baggage`). Implementations **MUST NOT** add custom
fields at the root of a specified type.

### What this SDK already does

| Layer | Present at `ea09a07` | Evidence |
| :--- | :--- | :--- |
| Generated v1 types, unions, methods | yes | `types_gen.go`, `agent_gen.go`, `client_gen.go`, `helpers_gen.go`, `constants_gen.go` from `schema/` |
| JSON-RPC connection, initialize record, prompt cancel, permission cancel, inbound/outbound gates, `Validate`, `Patch[T]` | yes | `docs/architecture.md`; `connection.go`; protocol table in `cmd/generate/internal/model/acp.go` |
| Notice and compaction constructors | yes | `UpdateNotice`, `UpdateCompaction`, `UpdateCompactionSummary`, `UpdateCompactionSummaryText` (`helpers.go:228`–`:266`) |
| Notice and compaction *send* gates | yes | `SessionUpdate.gates` refuses `notice` without `session.notices` and both compaction variants without `session.compaction` (`types_gen.go:23342`–`:23363`) |
| Method-level `Negotiation.Allows` | yes | `types_gen.go:23016`; `session/update` is always allowed, so this does not answer "may I send a notice" |
| `ClientRefusals` / (accepted) `AgentRefusals` | yes / in 0038 | "do not implement" embeds for unadvertised methods |
| Extension send and receive | yes, untyped | `ExtensionMethodHandler.HandleExtensionMethod(ctx, method, json.RawMessage)`; `CallExtension` / `NotifyExtension` return `json.RawMessage` (`extensions.go:27`, `:92`, `:127`) |
| `_meta` as `map[string]any` | yes, by design | [0012-MADR-schema-fidelity-framework.md](0012-MADR-schema-fidelity-framework.md); 0038 adds `MetaObject` / `DecodeMeta` |
| `agentproc` process tree, taps, death signals | yes | [0027-MADR-agent-process-helper.md](0027-MADR-agent-process-helper.md) |
| In-memory test pair | no package | 124 `io.Pipe()` calls across 27 test files in this repository; both consumers built their own harness ([0035-REPORT-consumer-acp-code-for-the-sdk.md](../reports/0035-REPORT-consumer-acp-code-for-the-sdk.md), D1) |
| Unstable API isolated from the default import | no | 79 `Unstable*` names in package `acp`; unstable fields such as `AgentCapabilities.Nes` merged onto stable-named types (0025) |

Sending `UpdateNotice` to a Client that omitted `session.notices` fails with a `*CapabilityError` and writes nothing
(`helpers.go:228`–`:231`). The v1 guide's documented fallback — an ordinary agent message — is left to every agent author. The
same is true of compaction. On the Client side there is no type that applies compaction's patch, chunk, first-seen, and replay
rules. `Kind()` names a `notice` or `compaction_update` after it arrives; it does not accumulate a timeline entity.

### What other SDKs did with the same schema

| SDK | 1.25.0 | Unstable boundary |
| :--- | :--- | :--- |
| TypeScript `@agentclientprotocol/sdk` | `v1.8.0` (2026-10-09): "update to v1.25.0" | `v2.0.0` three days after 1.26.0: default imports expose only stable-feature schemas; unstable v1 requires `@agentclientprotocol/sdk/unstable` |
| Rust `agent-client-protocol` | crate minors track schema tags; `unstable_session_compaction` Cargo feature removed when compaction stabilized | Cargo features (`unstable_nes`, `unstable_mcp_over_acp`, …); a user who never enabled a feature is not broken when it disappears |
| This Go module | types and gates for 1.25.0; no runtime for the new contracts | no boundary. 0025 measured 115 incompatible `apidiff` changes regenerating from 1.26.0, 40 of them on identifiers with no `Unstable` prefix |

Go's import compatibility rule is stricter than Cargo features or a TypeScript entrypoint. go.dev/ref/mod: "If an old package and
a new package have the same import path, the new package must be backwards compatible with the old package", and "a v1 version
acts as a commitment to compatibility". golang.org/x/exp is the standard-library pattern for experimental API: a separate import
path that "is not subject to the Go 1 compatibility promise". The Go module-compatibility article's rule for evolution inside a
v1 package is "add, don't change or remove"
(<https://go.dev/blog/module-compatibility>).

[0024-MADR-go-sdk-best-practices-on-a-go-1-26-9-floor.md](0024-MADR-go-sdk-best-practices-on-a-go-1-26-9-floor.md) finding V1 is
exactly this missing policy. [0019-MADR-remediate-debugging-pass-findings.md](0019-MADR-remediate-debugging-pass-findings.md)
question 16 left "the bump and the policy for removing unstable exported API under `v1`" to a new record. 0025 answered the bump:
not now. The policy is still unwritten.

### What 0035 already sorted, and what this record leaves

[0035-REPORT-consumer-acp-code-for-the-sdk.md](../reports/0035-REPORT-consumer-acp-code-for-the-sdk.md) sorted the consumers' ACP
code. Units 1–3 are 0036 and 0038. Units 4–6 (elicitation `Check`, FileHost / TerminalHost, `acptest`) and the unstable policy are
still open. This record takes the two that change an adopter's ability to use 1.25.0 and to extend the protocol, plus the test
harness 0035 called the largest remaining piece, plus the policy 0025 needs. Elicitation `Check`, `FileHost`, `TerminalHost`,
`Negotiation.AllowsValue`, and `agentproc.LineTap` stay as the next wave, recorded under More Information.

## Decision Drivers

* **The 1.25.0 contracts the types do not run.** Notices and compaction have MUSTs for capability checks, fallback, patch/chunk
  semantics, terminal status, and replay. Constructors and send-gates implement the "do not send" half. Adopters still write the
  "do send correctly" half.
* **Go v1 compatibility.** Additions are compatible under `apidiff`. Removals of unstable names wait for a published policy and,
  for names already in the root package, for stubs or a major. New unstable definitions stop landing on stable-named types.
* **Protocol-generic, deterministic, self-contained**, the test 0035 used: the specification or this SDK's own rule fixes the
  behaviour; the same input gives the same result; no new dependency.
* **Extensibility as the spec wrote it.** `_` methods and `_meta` are the supported way to add capabilities without waiting for
  the next schema tag.
* **Easy to test**, from 0024's Go SDK list and from 0035 D1: both consumers and this repository's tests each build in-memory
  peers by hand.
* **One decision, five additions.** Each is a unit a later PLAN can sequence. None reopens 0021, 0026, or 0027.

## Considered Options

* **O1.** The five additions specified below: an unstable import-path boundary with tombstones; capability-aware
  `PresentNotice` / `PresentCompaction`; a client `CompactionTimeline`; a typed `ExtensionMux` with reserved `_meta` keys; and
  package `acptest`.
* **O2.** O1 with item 5 replaced by the protocol-faithful Client hosts (elicitation `Check`, `FileHost`, `TerminalHost`).
* **O3.** Adopt schema `1.26.0` first, and treat the five as follow-up.
* **O4.** Only the unstable boundary (0019 question 16), and wait for more consumer copies of notices and compaction.
* **O5.** Nothing further on 1.25.0; keep generating types and shipping 0038's helpers.

## Decision Outcome

Chosen option: "O1", because these five close the 1.25.0 runtime and packaging gaps that types and 0038's small helpers leave,
they are additive under `v1`, and each one is something every adopter otherwise writes, differently. O2's Client hosts remain the
recommended next wave. O3 spends a `/v2` or 115 incompatible changes on an unstable Draft feature no consumer uses (0025). O4
leaves every agent author to invent notice fallback and compaction patches. O5 leaves the new stable features as types an
ordinary `SessionUpdate` call cannot send to a Client that has not advertised them, with no documented alternative in this
package.

This record answers 0019 question 16's *policy* half. It does not pin schema `1.26.0`. That bump uses this policy in a later
record, when a stable change or a consumer need makes it worth paying for.

### The five additions

Each new exported name has a doc comment that opens with its name and cites this record. None changes an existing signature.
Generated files change only where item 1 says so. 0038's helpers, once executed, stay; these names do not collide with them.

#### 1. Unstable surface isolation

**Problem.** Unstable definitions share package `acp`. The generator merges some of them into stable-named types, so
`AgentCapabilities.Nes` is a stable identifier for an abandoned Draft feature (0025). Regenerating from `1.26.0` is then 115
incompatible `apidiff` changes, 40 of them without an `Unstable` prefix. TypeScript isolated this with a `/unstable` entrypoint
and a major; Rust isolates it with Cargo features; Go's corresponding pattern is a separate import path.

**Add:**

* A new package `github.com/maccavelli/acp-go-sdk2/unstable`, documented as experimental: names may appear, change, or disappear
  in a minor, matching golang.org/x/exp's compatibility stance. It imports `acp`; `acp` does not import it.
* From the first release that includes this item, **new** unstable schema definitions emit only into `unstable`. They do not grow
  stable-named types in the root package.
* Existing `Unstable*` identifiers, `AgentExperimental`, `ClientExperimental`, and already-merged fields such as
  `AgentCapabilities.Nes` remain in `acp` so `apidiff` against current `v1` tags stays compatible. Each gets a `Deprecated:`
  paragraph naming this record and, where a counterpart exists, the `unstable` name.
* When a later schema *removes* an unstable definition, the generator emits a deprecated stub in `acp` (and in `unstable` until
  that package's next minor) so a `v1` minor never deletes a previously exported root name. Removing the stub is a `/v2` change.
* `scripts/check_api.py` continues to fail on an incompatible change in package `acp`. It compares `unstable` too, and treats
  incompatible changes there as a listed, non-failing report, so a Draft feature can leave `unstable` without forcing `/v2`.

**Adopter benefit.** Default importers stay on the stable 1.25.0 contract as the schema's unstable set moves. This module can
absorb 1.26.0's NES removal and system-prompt addition later without a major, by stubbing the removed root names and placing the
new Draft types in `unstable`.

#### 2. Capability-aware presentation: `PresentNotice` and `PresentCompaction`

**Problem.** The v1 guide requires Agents to choose a presentation channel. This SDK implements the refuse-if-unadvertised half.
An agent that calls `SessionUpdate(UpdateNotice(...))` against a Client that omitted `session.notices` gets a `*CapabilityError`
and the user sees nothing. The Claude Agent and Codex adapters already "gate live notices on the Client capability, retain
fallback behavior, and keep notices separate from history". Compaction has the same gate and the same documented fallback: an
ordinary agent message, which is conversation content, not an ID-addressed entity.

`Negotiation.Allows` cannot express this: it switches on method names, and both features travel as `session/update` variants
(`types_gen.go:23016`, `:23342`).

**Add**, hand-written in `helpers.go` (a runtime file that may name ACP types):

* `func (a *AgentSideConnection) PresentNotice(ctx context.Context, sessionId SessionId, n Notice) error`
* `func (a *AgentSideConnection) PresentCompaction(ctx context.Context, sessionId SessionId, u SessionCompactionUpdate) error`
* `func (a *AgentSideConnection) PresentCompactionSummary(ctx context.Context, sessionId SessionId, chunk SessionUpdateCompactionSummaryChunk) error`

Behaviour, fixed by the v1 guide:

* After `initialize`, read `Negotiated()`. If the matching Client capability is present (`Session.Notices` / `Session.Compaction`),
  send the dedicated `session/update` variant through `SessionUpdate`.
* If it is absent, send `UpdateAgentMessageText` (or `UpdateAgentMessage` for a notice that has a description) for the same
  `sessionId`, with a text block built from the notice's title and description, or from the compaction's status and optional
  error. That is the spec's fallback. It is conversation history, as the guide says.
* If `initialize` has not completed, return the same `ErrNotInitialized` `SessionUpdate` would.
* Options, if any, are functional and documented; a nil option panics, last value wins, as 0024 recorded for the connection
  options.

A caller that wants the dedicated variant or nothing keeps using `SessionUpdate` and `UpdateNotice` / `UpdateCompaction`; those
continue to fail with `*CapabilityError` when the capability is missing.

**Adopter benefit.** An agent written against this SDK can advertise-check and fall back in one call, matching Zed's peers, and
survives Clients that have not yet added `session.notices` or `session.compaction` to their `initialize`.

#### 3. Client compaction timeline

**Problem.** Compaction's receive contract is the most state-bearing stable addition in 1.25.0. First-seen initialization,
receive-order timeline placement, omit / `null` / value patches, chunk append versus authoritative `summary` replacement,
clearing with `[]` or `null`, unknown status preservation, the MUST to see a terminal status, and replay using the same rules
without implicitly resetting omitted fields: every Client that advertises `session.compaction` has to get this right. The SDK
exposes `Patch[T]` on the wire type (`README.md` documents `Summary Patch[[]ContentBlock]`, `Error Patch[string]`,
`Meta Patch[map[string]any]`). It does not accumulate those patches into an entity. 0035 recorded that consumers already
mis-implemented simpler unions (permission option by substring, grouped config options dropped). Compaction is easier to get
wrong.

**Add**, hand-written, root package:

* `type CompactionEntity struct` with `Id CompactionId`, `Status CompactionStatus`, `Summary []ContentBlock`, `Error string`,
  `Meta map[string]any`, and `Seen bool`
* `type CompactionTimeline struct` keyed by `CompactionId` per session
* `func (t *CompactionTimeline) Apply(u SessionUpdate) (CompactionEntity, bool)`
  * `compaction_update` for a new ID: initialize status `in_progress` and unset optional fields, then apply the notification, as
    the RFD states; return the entity and `true`
  * later `compaction_update` for a known ID: apply patches in place (`Patch.Present` / `Patch.Null` / value; `summary: []`
    clears); return the entity and `true`
  * `compaction_summary_chunk` for a known ID: append `content` to `Summary`; for an unknown ID, first-seen-initialize then
    append
  * any other variant: zero entity, `false`
  * unknown `Status` strings are stored as they arrive
* `func (t *CompactionTimeline) Get(id CompactionId) (CompactionEntity, bool)`
* `func (t *CompactionTimeline) Open() []CompactionId` — IDs whose status is `in_progress` or an unknown string, so a Client can
  fail or cancel them when the turn ends, which is the defect the Claude adapter's PR #1154 closed

Apply is the same function for live `session/update` and for `session/load` replay. The type holds no connection and sends
nothing.

**Adopter benefit.** A Client that advertises `session.compaction` embeds one timeline per session and feeds it every
`SessionUpdate`. Presentation (a row, a spinner, an expandable summary) stays with the Client; the protocol's entity algebra
lives here, once.

#### 4. Typed extension mux and reserved `_meta` keys

**Problem.** ACP's supported extension surface is `_` methods plus `_meta`. This SDK's API for it is one
`HandleExtensionMethod` that takes `json.RawMessage`, and `CallExtension` that returns `json.RawMessage`. Callers switch on
strings and decode by hand. 0035 recorded that both consumers built a mux, one of them on `encoding/json/v2`, which this
module's floor (Go 1.26.10) and CI's `GOEXPERIMENT=nojsonv2` job do not default to. The spec **SHOULD** advertise custom
capabilities under `_meta` on capability objects, and **SHOULD** reserve `traceparent`, `tracestate`, and `baggage` at the root
of `_meta`. 0038's `MetaObject` / `DecodeMeta` read a namespaced object; nothing writes the reserved keys or advertises a
namespace.

Go practice for this shape is a typed mux (net/http's `ServeMux`, the `Handler` interface) with generics available at this
module's floor.

**Add**, hand-written in `extensions.go` and a small new file that `TestRuntimeFilesCoverHandWritten` lists:

* `type ExtensionMux struct` that implements `ExtensionMethodHandler`
* `func Handle[P, R any](m *ExtensionMux, method string, fn func(context.Context, P) (R, error))` — refuses a method that does
  not start with `_` at registration; on a request, decodes params into `P` through `encoding/json` v1, runs `fn`, returns `R`;
  decode failure is `-32602`; `fn`'s `*RequestError` is sent as it is; any other error follows `toReqErr`
* `func Notify[P any](m *ExtensionMux, method string, fn func(context.Context, P) error)` — same decode; the handler's error is
  logged, not answered, as notifications are today
* Unregistered request → `NewMethodNotFound(method)`; unregistered notification → ignored, which is the spec's SHOULD
* `func (m *ExtensionMux) Methods() []string` — registered names, for tests and for capability advertisement
* `func TraceMeta(traceparent, tracestate, baggage string) map[string]any` — the three reserved keys, omitting empty strings
* `func AdvertiseMeta(meta map[string]any, namespace string, features map[string]any) map[string]any` — sets `meta[namespace]`
  to `features` when `namespace` is non-empty, copying `meta` first; this is the spec's capability-object `_meta` example
  (`zed.dev` / `workspace` / `fileNotifications`)

`CallExtension` and `NotifyExtension` stay. The mux is the inbound half; outbound stays untyped until a caller has a generated
or hand-written params type.

**Adopter benefit.** An agent or client registers `_vendor/foo` with a Go function. Capability advertisement and W3C trace
headers become one-liners that stay inside the spec's reserved shapes, so a later ACP version that promotes a `_meta` key does
not collide with a vendor field at the root.

#### 5. Package `acptest`

**Problem.** 0035 D1: `<cli-b>`'s `acptest` is about 1,300 lines (`Connect` over two `io.Pipe`s, a frame recorder with goldens, host
`Abs` because this SDK checks received paths with the host's `filepath.IsAbs`, a fake Client, a scripted Agent). magic-cli-remote
has a byte-level scripted peer and an initialize fixture. This repository's tests call `io.Pipe()` 124 times across 27 files.
`<cli-b>`'s recorder imports `encoding/json/v2` and `jsontext`, which fail at this module's Go 1.26.10 floor and under
`GOEXPERIMENT=nojsonv2`. 0024 listed "easy to test" as already true of the transport (`io.Reader` / `io.Writer`) and false of the
package surface: there is no supported way to stand up a pair, record frames, or script a peer.

**Add** a new exported package `github.com/maccavelli/acp-go-sdk2/acptest`:

* `func Connect(t testing.TB, agent acp.Agent, client acp.Client, opts ...acp.ConnectionOption) Pair` — two pipes, both side
  connections, cleanup that `Close`s and `Wait`s both, so the root leak guard stays green
* `type Pair struct` with `Agent *acp.AgentSideConnection`, `Client *acp.ClientSideConnection`, `Close() error`, `Wait() error`
* `func Abs(path string) string` — makes a POSIX-looking fixture path absolute on the host, the helper 0035 recorded both
  consumers need because inbound `Validate` uses `filepath.IsAbs`
* `type Recorder struct` — an `io.Writer` / `io.Reader` pair that copies each newline-terminated frame; `Frames() []json.RawMessage`
  returns copies; canonical form is `encoding/json` v1 with the same HTML-escaping and map-key sorting `json_v1_pin_test.go` pins
* `type ScriptAgent` — implements `acp.Agent` by playing a list of `SessionUpdate`s per `Prompt`, then a `StopReason`; ungated
  methods embed `AgentRefusals` once 0038 lands, and `Initialize` answers `ProtocolVersionNumber` plus caller-supplied capabilities
* `type MemoryClient` — implements `acp.Client` with in-memory files (1-based `line` / `limit`, missing path as
  `NewResourceNotFound`) and a map of scripted permission outcomes; terminal methods return `NewMethodNotFound` unless a later
  record adds `TerminalHost`

No `encoding/json/v2`. No new dependency. `Pair` uses the public constructors, so it sees the same initialize-hold, gates, and
close behaviour as production.

**Adopter benefit.** A consumer writes `acptest.Connect(t, agent, client)` and asserts on recorded frames or on handler side
effects. This repository's own tests can migrate off raw pipes in a later pass; that migration is not this item's scope. The
package is also how items 2–4 get their black-box tests without 124 more `io.Pipe` sites.

### Questions for the owner

1. **Compaction fallback text (item 2).**
   * Recommended: when `session.compaction` is absent, `PresentCompaction` sends an agent-message whose text is
     `Compaction {status}` plus the error string when present, and `PresentCompactionSummary` is a no-op (the spec's fallback is
     a description of compaction, not a streamed summary entity).
   * Alternative: `PresentCompaction*` return a typed `ErrCapabilityFallback` and send nothing, so the caller chooses the
     message. That is safer against putting a summary the Agent wanted hidden into history, and worse for the "one call" goal.
2. **Where `acptest` lives relative to 0035's next wave (item 5).**
   * Recommended: `acptest` in this five, as O1. Elicitation `Check`, `FileHost`, and `TerminalHost` are the next record.
   * Alternative: O2, ship the Client hosts here and leave `acptest` as its own later MADR, which is how 0035 numbered them.
3. **Unstable stubs versus a published apidiff exemption (item 1).**
   * Recommended: stubs in `acp` for any unstable name `v1` has already exported, so `make api` keeps its current meaning on the
     root package. `unstable` is allowed to break.
   * Alternative: document that every `Unstable*` name and every `AgentExperimental` / `ClientExperimental` method is outside
     the `v1` promise, and teach `check_api.py` to ignore them. Smaller generator work; 0025's 40 stable-named merged fields
     (`AgentCapabilities.Nes`, …) would still break unless the exemption lists those fields too, which is the dishonest shape
     0025 warned about.
4. **The PLAN.**
   * Recommended: a PLAN under this number after the owner answers 1–3, sequenced 1 (generator + package + deprecation), 5
     (`acptest`, so the rest test against it), 2, 3, 4. Each phase a minor or one minor at the end, owner's choice at the PLAN.
   * Alternative: five PLANs with slugs of their own, still numbered 0039.

#### Specification notes that are not questions

* Item 1 does not adopt schema `1.26.0`.
* Items 2–4 need no generator change beyond item 1's emission target for *new* unstable defs.
* `Present*` never send a notice or compaction variant to a Client that did not advertise the capability; the gate in
  `SessionUpdate.gates` remains the backstop.
* `CompactionTimeline` does not collapse earlier messages; the RFD leaves that to the Client.
* `ExtensionMux` does not inspect `_meta` on typed methods; `TraceMeta` is for callers who put it on a params struct themselves.
* `acptest.MemoryClient` does not confine paths to `Roots()`; 0035 left that as consumer policy.

### Consequences

* Good, because a v1 Client that has not advertised notices or compaction still sees the information, through the spec's
  fallback, from any agent that uses `Present*`.
* Good, because a v1 Client that *has* advertised compaction gets one tested implementation of the patch/chunk/replay algebra.
* Good, because `_` methods and reserved `_meta` keys become typed, stdlib-only, and match the extensibility page.
* Good, because `acptest` is a supported pair at the language floor, on `encoding/json` v1, which both consumers and this
  repository currently rebuild.
* Good, because the unstable boundary is the missing piece 0024 V1, 0019 question 16, and 0025 all named, and it is additive:
  existing names remain, new Draft types go to `unstable`.
* Neutral, because elicitation `Check`, `FileHost` (line/limit, `-32002`), and `TerminalHost` (character-boundary truncation,
  `types_gen.go:4094`) stay unbuilt; they are the next wave, and 0035 already specified them.
* Neutral, because `PresentCompaction`'s fallback is conversation history, which is what the guide says it is.
* Bad, because the root package grows `CompactionTimeline`, `Present*`, `ExtensionMux`, and the reserved `_meta` helpers, each a
  `v1` promise.
* Bad, because package `unstable` is a new import path whose contents will churn, and a user who imports it must read that
  documentation.
* Bad, because deprecated stubs for removed unstable names accumulate in `acp` until a `/v2`.
* Bad, because `acptest` is a new exported package that `make api` must compare from an empty baseline, and `<cli-b>` would still
  need to amend its import rule or keep a wrapper (0035 D1).

### Confirmation

A later PLAN shows each of the following, every new check first seen to fail on a planted break in a scratch copy:

* `PresentNotice` sends `notice` to a Client that advertised `session.notices`, and `agent_message_chunk` to one that did not;
  `UpdateNotice` still fails with `*CapabilityError` in the second case
* `PresentCompaction` matches the answer to question 1; a terminal `cancelled` / `failed` / `completed` is sent when the
  capability is present
* `CompactionTimeline.Apply` first-seen-initializes, patches, appends chunks, replaces on concrete `summary`, clears on `[]` and
  `null`, preserves an unknown status, and applies identically on a replay sequence from `testdata`
* `ExtensionMux` decodes a typed request, answers `-32601` for an unknown `_` request, ignores an unknown `_` notification, and
  refuses a registration whose name has no `_` prefix
* `TraceMeta` writes only the three reserved keys; `AdvertiseMeta` copies and sets a namespaced object
* `acptest.Connect` round-trips `initialize` and one prompt on `GOEXPERIMENT=nojsonv2`; `Abs` produces a path `filepath.IsAbs`
  accepts on the host; `Recorder` frames match the v1 pin
* `make api` against the newest `v1` tag lists only compatible changes in `acp`, including the deprecated stubs, and lists
  `acptest` and `unstable` as new packages
* `go doc` of package `acp` still shows existing `Unstable*` names; a new unstable definition added in a synthetic engine case
  appears only under `unstable`

## Pros and Cons of the Options

### O1. The five additions specified above

* Good, because each item maps to a 1.25.0 MUST/SHOULD or to a Go v1 packaging rule this repository has already recorded as
  missing.
* Good, because all five are additive: new names, a new package, deprecated existing names, no signature change.
* Good, because `acptest` is the test bed for items 2–4 and for every later protocol helper.
* Neutral, because item 1 is generator work and the other four are hand-written.
* Bad, because five units in one decision is a large PLAN, even if phases can ship separately.
* Bad, because O2's Client hosts, which close two protocol MUSTs (character-boundary truncation, `-32002` on a missing file),
  wait.

### O2. O1 with Client hosts as item 5

* Good, because elicitation `Check`, `FileHost`, and `TerminalHost` are protocol-generic and 0035 already specified them.
* Good, because `ClientRefusals` then has a matching "do implement" path for fs, terminal, and form answers.
* Bad, because this repository and both consumers still each maintain an in-memory pair, and items 2–4 have no supported harness.
* Bad, because `TerminalHost` is the largest of 0035's C items and pulls `agentproc` into the Client side, which is a design of
  its own.

### O3. Adopt schema 1.26.0 first

* Good, because the pin would match upstream's latest v1 tag, and NES would leave the generated tree.
* Bad, because 0025 measured 115 incompatible changes, the stable protocol is byte-identical, and the one addition is a Draft
  RFD no consumer uses. This option inverts the order 0025 recommended: policy first, bump when a stable change or a consumer
  need appears.

### O4. Only the unstable boundary

* Good, because it is the smallest answer to 0019 question 16 and unblocks a future 1.26.0 record.
* Bad, because notices and compaction stay as types an agent cannot present to an older Client, and as patches a Client must
  re-derive.
* Bad, because extensibility stays `json.RawMessage`.

### O5. Nothing further on 1.25.0

* Good, because 0038's helpers, once executed, already remove the consumers' known defects (paging, permission kind, error
  codes, `_meta` read, stdio rewrite).
* Bad, because 1.25.0's reason for existing — stabilized notices and compaction — remains a generated surface without a runtime.
* Bad, because the next schema tag is still blocked on an unwritten policy.

## More Information

### Sources

* Schema release: <https://github.com/agentclientprotocol/agent-client-protocol/releases/tag/schema-v1.25.0>
* v1 prompt turn, notices and compaction: <https://agentclientprotocol.com/protocol/v1/prompt-turn.md>
* Session Notices RFD (Completed 2026-10-09): <https://agentclientprotocol.com/rfds/session-notices>
* Session Compaction RFD (Completed 2026-10-06): <https://agentclientprotocol.com/rfds/session-compaction>
* Extensibility: <https://agentclientprotocol.com/protocol/extensibility.md>
* v1 elicitation: <https://agentclientprotocol.com/protocol/v1/elicitation.md>
* Go modules reference, import compatibility: <https://go.dev/ref/mod>
* Go blog, module compatibility ("add, don't change or remove"): <https://go.dev/blog/module-compatibility>
* golang.org/x/exp compatibility stance: <https://pkg.go.dev/golang.org/x/exp>
* TypeScript SDK 1.8.0 / 2.0.0: <https://github.com/agentclientprotocol/typescript-sdk/blob/main/CHANGELOG.md>
* 0025's regeneration and `apidiff` counts: [0025-REPORT-schema-1-26-0-adoption-assessment.md](../reports/0025-REPORT-schema-1-26-0-adoption-assessment.md)

### Tree citations

* `schema/version` = `1.25.0`; `go 1.26.10`; `toolchain go1.27.3`; module `github.com/maccavelli/acp-go-sdk2`
* `Notice` schema, including the MUST to send only when advertised and the agent-message fallback:
  `schema/schema.json` definition `Notice`
* `CompactionUpdate` schema, including patch semantics: `schema/schema.json` definition `CompactionUpdate`
* Terminal truncation MUST: `types_gen.go:4094`
* `NewResourceNotFound`: `errors.go:243`

### Next wave, not this decision

These stay as 0035 numbered them. They are the natural record after this one lands:

* C3. `ElicitationSchema.Fields` and `Check` (Clients **SHOULD** validate form answers; RE2 cannot compile every ECMA-262
  pattern, and that miss is reported, not treated as a match)
* C4 / C5. `TerminalHost` in `agentproc` (character-boundary truncation) and `ReadTextFileOS` / `WriteTextFileOS` (`-32002` for
  a missing file)
* C6. `Negotiation.AllowsValue`, exposing the generated value gates so a caller can drop an image block or an MCP server before
  a send
* C7. `agentproc.LineTap`
* E3 / E4 only if the owner reopens 0026 or 0027

### Out of scope

* Schema `1.26.0` and protocol v2
* `encoding/json/v2`
* A WebSocket or Unix-socket transport (0035 F: needs a dependency, needs its own MADR)
* Bounded `Wait` (0026 question 4)
* Changing `PromptOverlapRefuse`'s code (0021)
* Adding methods to `Agent` or `Client` (that is an incompatible `apidiff` change for every embedder)

Implementation waits on the owner's answers to questions 1–4 and an approved PLAN.
