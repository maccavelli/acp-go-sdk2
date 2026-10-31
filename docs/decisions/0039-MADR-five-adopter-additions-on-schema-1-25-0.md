---
status: accepted
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
the protocol method `$/cancel_request`. Unstable `schema.unstable.json` has 277 `$defs`. `go doc -short .` declares 65 exported
names that start with `Unstable`, on 79 lines that mention one; the "79" of
[0024-MADR-go-sdk-best-practices-on-a-go-1-26-9-floor.md](0024-MADR-go-sdk-best-practices-on-a-go-1-26-9-floor.md) V1 is that
line count.
`AgentExperimental` and `ClientExperimental` already exist as optional interfaces (`types_gen.go:22521`, `:22691`).

The two features 1.25.0 moved from Preview to Completed, and that this SDK therefore now generates as stable, are:

* **Session notices** ([Session Notices RFD](https://agentclientprotocol.com/rfds/session-notices),
  [v1 prompt turn](https://agentclientprotocol.com/protocol/v1/prompt-turn.md#session-notices)). A fire-and-forget
  `sessionUpdate: "notice"` with required `severity` (an open enum, initially `info`, `warning`, `error`) and a non-empty plain-text
  `title`; `description` is optional. No ID, response or acknowledgement; notices "SHOULD NOT be included in session replay". v1
  Agents **MUST NOT** send it unless the Client supplied `clientCapabilities.session.notices: {}`. When that capability is absent,
  "the Agent may use an ordinary agent message instead". Notice support "advertises a presentation channel, not acknowledgement of
  any individual notice". The RFD lists Zed, the Claude Agent adapter and the Codex adapter: both adapters gate live notices on the
  capability and keep them out of history, and the Codex adapter "preserves legacy fallback behavior".
* **Session compaction** ([Session Compaction RFD](https://agentclientprotocol.com/rfds/session-compaction),
  [v1 prompt turn](https://agentclientprotocol.com/protocol/v1/prompt-turn.md#session-compaction)). An ID-addressed
  `compaction_update` / `compaction_summary_chunk` pair. The first notification for an ID fixes timeline position; later updates
  patch in place. `summary`, `error`, and `_meta` are omit / `null` / value patches; chunks append; a concrete `summary` replaces
  accumulated content; `summary: []` and `summary: null` clear it. Clients initialize a first-seen ID with status `in_progress`
  and unset optional fields, then apply the notification. The statuses are `in_progress`, `completed`, `failed` and `cancelled`;
  Clients "preserve an unknown string". The Agent **MUST** report a terminal status when a reported compaction ends. Agents "must
  only send" either variant to a v1 Client that supplied `clientCapabilities.session.compaction: {}` (lowercase in the RFD), and
  "the v1 capability gate applies to replay as well as live delivery". When the capability is absent, "the Agent may describe
  compaction through an ordinary agent message instead" (v1 prompt turn). "Replay does not implicitly reset omitted patch fields."
  The RFD cites the Claude Agent adapter as "public v1 Agent-side implementation evidence". It was stabilized on 2026-10-06; its
  latest revision, 2026-10-13, changes `_meta` patching for v2 only.

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
| Extension send and receive | yes, untyped | `ExtensionMethodHandler.HandleExtensionMethod(ctx, method, json.RawMessage) (any, error)` (`extensions.go:27`) receives requests and notifications alike, and nothing exported tells them apart; `CallExtension` returns `json.RawMessage` and `NotifyExtension` an `error` (agent side `:92`, `:109`; client side `:127`, `:144`) |
| `_meta` as `map[string]any` | yes, by design | [0012-MADR-schema-fidelity-framework.md](0012-MADR-schema-fidelity-framework.md); 0038 adds `MetaObject` / `DecodeMeta` |
| `agentproc` process tree, taps, death signals | yes | [0027-MADR-agent-process-helper.md](0027-MADR-agent-process-helper.md) |
| In-memory test pair | no package | 124 `io.Pipe()` calls across 27 test files in this repository; both consumers built their own harness ([0035-REPORT-consumer-acp-code-for-the-sdk.md](../reports/0035-REPORT-consumer-acp-code-for-the-sdk.md), D1) |
| Unstable API isolated from the default import | no | 65 `Unstable*` names in package `acp`; unstable fields such as `AgentCapabilities.Nes` (`types_gen.go:98`) merged onto stable-named types (0025) |

Sending `UpdateNotice` to a Client that omitted `session.notices` fails with a `*CapabilityError` and writes nothing
(`helpers.go:228`–`:231`). The v1 guide's documented fallback — an ordinary agent message — is left to every agent author. The
same is true of compaction. On the Client side there is no type that applies compaction's patch, chunk, first-seen, and replay
rules. `Kind()` names a `notice` or `compaction_update` after it arrives; it does not accumulate a timeline entity.

### What other SDKs did with the same schema

| SDK | 1.25.0 | Unstable boundary |
| :--- | :--- | :--- |
| TypeScript `@agentclientprotocol/sdk` | `v1.8.0` (2026-10-09): "update to v1.25.0" | `v2.0.0` three days after 1.26.0: default imports expose only stable-feature schemas; unstable v1 requires `@agentclientprotocol/sdk/unstable` |
| Rust schema crate `agent-client-protocol-schema` | one crate release per schema tag, under its own numbers: `1.11.0` with `schema-v1.25.0` (same commit), `1.12.0` with `schema-v1.26.0`; `1.11.0` dropped the `unstable_session_compaction` and `unstable_session_notices` features | Cargo features: `unstable_nes` until `1.11.0`, removed in `1.12.0` with NES; `unstable_mcp_over_acp` still present. A user who never enabled a feature is not broken when it disappears. (The SDK crate is `agent-client-protocol`, `3.5.0`.) |
| This Go module | types and gates for 1.25.0; no runtime for the new contracts | no boundary. 0025 measured, against `v1.17.1`, 115 incompatible `apidiff` changes regenerating from 1.26.0, 40 of them on identifiers with no `Unstable` prefix |

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
code. Units 1–3 are 0036 and 0038. Units 4 (C3 elicitation `Check`, C6 `AllowsValue`), 5 (C4 `TerminalHost`, C5 file reads and
writes, C7 `LineTap`) and 6 (D1 `acptest`), and the unstable policy, are still open. This record takes the two that change an adopter's ability to use 1.25.0 and to extend the protocol, plus the test
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
  *Refined by the second assessment: the "stub" is the frozen declaration itself, built from the manifest's fragment, and a
  schema that changes a manifest definition is handled the same way; see "How it is built". The `unstable` half applies only if
  question 7 is answered "now".*
* `scripts/check_api.py` continues to fail on an incompatible change in package `acp`. It compares `unstable` too, and treats
  incompatible changes there as a listed, non-failing report, so a Draft feature can leave `unstable` without forcing `/v2`.

**What a separate package cannot hold** (assessment, 2026-10-25). Go allows no type to gain a field, a union variant or a method
from another package, and `acp` cannot import `unstable`, which imports it. So "new unstable definitions emit only into
`unstable`" fits a new standalone definition: a type, an enum, a request or response struct. It does not fit three shapes the
unstable schema has used:

* a new unstable **field on a stable type**, as NES added `AgentCapabilities.Nes`: a stable struct drops a key it does not declare
  on decode, unless the generator keeps extra keys for it;
* a new unstable **variant of a stable union**, as `subagent_update` is of `SessionUpdate`: it would decode into `Unknown`;
* a new unstable **method**, as the `AgentExperimental` methods are: the generated dispatch in `acp` cannot call a handler declared
  in `unstable`, so a received one is answered `-32601`.

Question 5 decides how those reach an adopter.

Two further constraints:

* The generator refuses a deprecation note on a node the schema does not mark (`docs/architecture.md`, the paragraph on
  `deprecated`). The `Deprecated:` paragraphs on existing unstable names therefore need a new deprecation source in the generator,
  not a protocol-table note.
* Where no `unstable` counterpart exists, as for `AgentExperimental` until question 5's mechanism does, the notice names this
  record and says the name stays under `v1`; it names no replacement.

**How it is built** (second assessment, 2026-10-25; its findings G1 to G9 are under More Information). The draft named the
outcome; this is the mechanism, read from the generator, within the owner's answers to questions 3 and 5:

* **Where unstable API comes from.** The merge (`cmd/generate/internal/load/merge.go`) does three things (G1): a stable
  definition that a stable method reaches is replaced by its unstable version when the two differ (`:329`–`:331`), which is how
  `AgentCapabilities.Nes` and `SessionUpdate`'s unstable variants arrive; a definition only the unstable schema has is added under
  its own name; and the subgraph of an unstable-only method is copied under the `Unstable` prefix (`:102`–`:105`). At `1.25.0`
  that is 102 definitions, 10 fields on 8 stable-named types, 6 union variants and 16 methods (G2).
* **The manifest is the frozen `v1` contract.** `schema/unstable-v1.json` records every unstable-origin name `acp` exports, each
  with its schema fragment as of the first release with this item. The generator builds `acp`'s unstable-origin API from those
  fragments, not from the live unstable schema, so a later schema that changes or removes one changes nothing in `acp`. This is
  question 3's frozen stubs, and it makes "kept" and "removed" one rule; no second file of frozen fragments is needed.
* **The manifest is also where a frozen type's schema is read** (*amended 2026-10-25, by the PLAN's 2c deviation entry, before
  its code*).
  * *What was found:* the root conformance tests (`instances_test.go`, `conformance_test.go`, `encodings_test.go`) build each
    generated type's JSON instances from `schema/<File>` at `<Key>`. They read both from the type's origin, which the generated
    registry in `schema_gen_test.go` holds. 112 of those entries name `schema.unstable.json`: 110 types and 2 absolute-path
    rules. A schema that drops a frozen
    definition therefore leaves the tests reading a key that is gone. Deleting `NesCapabilities` alone from a scratch copy's
    `schema.unstable.json` fails `TestConformanceRoundTrip`, `TestConformanceLenient` and `TestEncodingsUnchanged`.
  * *What changes:* the manifest holds its fragments as a frozen `$defs`, under the keys they have in `schema.unstable.json`:
    each kept definition's source, and the whole unstable version of each replaced one. With them it holds the names `acp` gives
    the definitions, the entries applied to each replaced definition, and the methods. Each type built from the manifest takes
    the manifest as its origin, and the root tests read its `$defs` over `schema.json`'s, as the merge resolves a frozen
    fragment's references.
  * *What it costs:* the generated `*_gen.go` files are still byte-identical. `schema_gen_test.go` differs only in the `File` of
    those 112 entries.
* **The merge stops growing stable-named types.** For a stable-reachable definition, the merge takes the stable definition with
  the manifest's entries for it applied, not the whole unstable version. An entry is a property the unstable version adds, a
  property it changes (its whole fragment, so a changed default is carried), or a union entry it adds or changes. At `1.25.0`
  the eight replaced definitions differ from their stable versions in exactly 18 such entries, and in nothing else (no
  description or other keyword): G2's ten added fields and six added variants, and the defaults of
  `AgentCapabilities.mcpCapabilities` and `InitializeResponse.agentCapabilities`, which the unstable schema changes to include
  `acp: false`. Applying the 18 to the stable definitions gives the merge's present output, so the generated code does not
  change. An unstable change the manifest does not list reaches no stable-named type. A new unstable-only definition, not in the
  manifest, goes where question 5's answer sends it.
* **Provenance must reach fields and variants.** The model records origin for types (`Type.Unstable`, `Type.Origin`,
  `model/model.go:112`–`:116`) and methods (`Method.Unstable`), not for fields or variants; only the merge holds both versions
  (G3). The merge records which properties and variants of a replaced definition are unstable, and the model carries it.
* **Deprecation needs a source of its own.** Nothing today can mark a declaration the schema does not, and the emitted text, "the
  ACP schema marks … deprecated", would be false here (G5). The manifest is the new source, with its own text.
* **`example/agent` changes with it.** It asserts `acp.AgentExperimental` and implements all 15 `Unstable*` methods
  (`example/agent/main.go:41`). staticcheck's SA1019, which CI runs, flags a deprecated name's use from any other package (G6), so
  the example stops implementing them in the same commit.
* **A second generated package is real work.** The generator writes one package, `acp`, to one directory, type-checks one package
  per call against the source on disk, and an engine case has one output and one API listing (G4). Package `unstable` needs all
  four extended. At `1.25.0` every unstable definition is in the manifest, so `unstable` would start with no generated
  declaration, and that machinery would be exercised only by synthetic engine cases (G9). Question 7 asks whether to build it now.
* **The API gate gains a non-gating package.** `scripts/check_api.py` pools every package's incompatible changes into one list and
  fails on any (G7); it gains a list of packages whose changes are reported and not gated, and its first tests.

**Adopter benefit.** Default importers stay on the stable 1.25.0 contract as the schema's unstable set moves. This module can
absorb 1.26.0's NES removal and system-prompt addition later without a major, by stubbing the removed root names and placing the
new Draft types in `unstable`.

#### 2. Capability-aware presentation: `PresentNotice` and `PresentCompaction`

**Problem.** The v1 guide requires Agents to choose a presentation channel. This SDK implements the refuse-if-unadvertised half.
An agent that calls `SessionUpdate(UpdateNotice(...))` against a Client that omitted `session.notices` gets a `*CapabilityError`
and the user sees nothing. The Notices RFD records that the Claude Agent adapter "gates live notices on the Client capability" and
keeps them separate from history, and that the Codex adapter gates them and "preserves legacy fallback behavior". Compaction has
the same gate and a documented fallback: "the Agent may describe compaction through an ordinary agent message instead", which is
conversation content, not an ID-addressed entity.

`Negotiation.Allows` cannot express this: it switches on method names, and both features travel as `session/update` variants
(`types_gen.go:23016`, `:23342`).

**Add**, hand-written in `helpers.go`, which may name ACP types because it is not one of the generator's runtime files
(`connection.go`, `doc.go`, `errors.go`, `extensions.go`, which may not):

* `func PresentNotice(ctx context.Context, a *AgentSideConnection, sessionId SessionId, n Notice) error`
* `func PresentCompaction(ctx context.Context, a *AgentSideConnection, sessionId SessionId, u CompactionUpdate) error`
* `func PresentCompactionSummary(ctx context.Context, a *AgentSideConnection, sessionId SessionId, chunk CompactionSummaryChunk) error`

*Amended 2026-10-25:* these were methods of `AgentSideConnection`. They are package-level functions, by the rule
[0038-MADR-consumer-helpers-kind-and-agent-refusals.md](0038-MADR-consumer-helpers-kind-and-agent-refusals.md)'s "Amendment,
2026-10-25" records: every exported method of a side connection that takes a context is a protocol method, which
`TestSideConnectionMethodsAreProtocolMethods` enforces. A nil `a` returns an error.

Each takes the schema's definition type (`types_gen.go:9864`, `:2250`, `:2190`), not the `SessionUpdate` variant, which also carries
a `SessionUpdate` discriminator field. The draft took the variant types for compaction and the definition type for the notice;
the assessment made the three uniform.

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
  * later `compaction_update` for a known ID: apply patches in place (`Patch.IsZero` leaves the field, `IsNull` clears it,
    `Value` replaces it; `summary: []` clears); return the entity and `true`
  * `compaction_summary_chunk` for a known ID: append `content` to `Summary`; for an unknown ID, first-seen-initialize then
    append. The RFD states the first-seen rule for "a first-seen compaction ID" without naming chunks, so this case is inferred
    from it, not stated
  * any other variant: zero entity, `false`
  * unknown `Status` strings are stored as they arrive
* `func (t *CompactionTimeline) Get(id CompactionId) (CompactionEntity, bool)`
* `func (t *CompactionTimeline) Open() []CompactionId` — IDs whose status is `in_progress` or an unknown string, so a Client can
  show them as unfinished when a turn ends without a terminal status. The agent-side counterpart is the defect
  agentclientprotocol/claude-agent-acp PR #1154 fixed, "close interrupted compactions lifecycles" (merged 2026-09-22, in `0.80.0`):
  compactions left open by cancelled or failed prompts

The RFD leaves each Client to keep this state; 0035 F lists "a session-state reducer" as not recommended because no consumer had
one as a unit. `CompactionTimeline` is narrower: one entity type whose algebra the RFD fixes, not a reducer over the session.

Apply is the same function for live `session/update` and for `session/load` replay. The type holds no connection and sends
nothing.

**Adopter benefit.** A Client that advertises `session.compaction` embeds one timeline per session and feeds it every
`SessionUpdate`. Presentation (a row, a spinner, an expandable summary) stays with the Client; the protocol's entity algebra
lives here, once.

#### 4. Typed extension mux and reserved `_meta` keys

**Problem.** ACP's supported extension surface is `_` methods plus `_meta`. This SDK's API for it is one
`HandleExtensionMethod` that takes `json.RawMessage`, and `CallExtension` that returns `json.RawMessage`. Callers switch on
strings and decode by hand. 0035 recorded gobble-cli's mux, on `encoding/json/v2`, which this module's floor (Go 1.26.10) and
CI's `GOEXPERIMENT=nojsonv2` job do not build. 0035 F listed "an extension-method mux" among shapes "not recommended now", because
each consumer's policy differs. This item reverses that, for its dispatch core only: the extensibility page fixes the `_` prefix,
the method-not-found answer to an unknown request, and that implementations "SHOULD ignore unrecognized notifications", so the
mux is protocol-generic; each consumer's handlers stay its own. The spec **SHOULD** advertise custom
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
* `func IsNotification(ctx context.Context) bool`, in `connection.go` — reads the mark the connection already puts in a handler's
  context, which nothing exported reads today. `HandleExtensionMethod` receives requests and notifications alike, so without it
  the mux cannot tell one from the other, and so cannot ignore an unregistered notification or refuse a request sent to a `Notify`
  name (question 6)
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

**Problem.** 0035 D1: gobble-cli's `acptest` is about 1,300 lines (`Connect` over two `io.Pipe`s, a frame recorder with goldens, host
`Abs` because this SDK checks received paths with the host's `filepath.IsAbs`, a fake Client, a scripted Agent). magic-cli-remote
has a byte-level scripted peer and an initialize fixture. This repository's tests call `io.Pipe()` 124 times across 27 files.
gobble-cli's recorder imports `encoding/json/v2` and `jsontext`, which fail at this module's Go 1.26.10 floor and under
`GOEXPERIMENT=nojsonv2`. 0024 listed "easy to test" as already true of the transport (`io.Reader` / `io.Writer`) and false of the
package surface: there is no supported way to stand up a pair, record frames, or script a peer.

**Add** a new exported package `github.com/maccavelli/acp-go-sdk2/acptest`:

* `func Connect(t testing.TB, agent acp.Agent, client acp.Client, opts ...acp.ConnectionOption) Pair` — two pipes, both side
  connections, cleanup that `Close`s and `Wait`s both, so the root leak guard stays green
* `type Pair struct` with `Agent *acp.AgentSideConnection`, `Client *acp.ClientSideConnection`, `Close() error`, `Wait() error`
* `func Abs(path string) string` — makes a POSIX-looking fixture path absolute on the host, as gobble-cli's `acptest` does (0035
  D1), because inbound `Validate` uses `filepath.IsAbs`; this repository's tests have the same helper, `testAbsPath`
  (`paths_test.go:24`)
* `type Recorder struct` — an `io.Writer` / `io.Reader` pair that copies each newline-terminated frame; `Frames() []json.RawMessage`
  returns copies; canonical form is `encoding/json` v1 with the same HTML-escaping and map-key sorting `json_v1_pin_test.go` pins
* `type ScriptAgent` — implements `acp.Agent` by playing a list of `SessionUpdate`s per `Prompt`, then a `StopReason`; the five
  capability-gated methods come from embedding `AgentRefusals`, so 0038's step 4 lands first, and `Initialize` answers
  `ProtocolVersionNumber` plus caller-supplied capabilities
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
   * Recommended: a PLAN under this number after the owner answers 1–3, and 5 and 6 since the assessment, sequenced 1 (generator + package + deprecation), 5
     (`acptest`, so the rest test against it), 2, 3, 4. Each phase a minor or one minor at the end, owner's choice at the PLAN.
   * Alternative: five PLANs with slugs of their own, still numbered 0039.
   * Written 2026-10-25, at the owner's request, as
     [0039-PLAN-five-adopter-additions-on-schema-1-25-0.md](0039-PLAN-five-adopter-additions-on-schema-1-25-0.md), under the
     recommended answers to questions 1 to 6; an answer otherwise amends it before the phase it changes.
5. **How new unstable fields, union variants and methods reach an adopter (item 1).** Added by the assessment: a separate package
   cannot hold them (item 1, "What a separate package cannot hold").
   * Recommended: `unstable` holds new standalone definitions, and for the three shapes it cannot hold, the generator emits nothing
     into `acp` and the shape is not typed until it is stable. A new unstable variant decodes into the union's `Unknown`, which
     `unstable` can decode with a generated `Decode…` function over the raw payload; a new unstable field or method is reachable
     only through `_meta` or the untyped `Connection` until it stabilizes. Smallest surface; the cost is that a Draft feature of
     those shapes is not typed.
   * Alternative: a registry in `acp`, through which `unstable` registers method handlers, union variant decoders and extra struct
     keys at `init`. Typed reach for every shape, at the cost of a new exported registration API in `acp` and a dispatch path that
     no stable method uses.
   * Alternative: keep generating unstable definitions into `acp`, under `Unstable`-prefixed names only and never merged into a
     stable-named type, and publish that `Unstable*` names are outside the `v1` promise. Closest to today; it is question 3's
     alternative, and leaves the boundary a naming rule rather than an import path.
6. **How `ExtensionMux` tells a notification from a request (item 4).** Added by the assessment.
   * Recommended: `IsNotification(ctx) bool` in `connection.go`, reading the mark the connection already sets.
   * Alternative: a new optional interface, `ExtensionNotificationHandler`, that the connection calls for `_` notifications when
     the handler implements it, so the mux implements both.
   * Alternative: no distinction. An unregistered method answers `NewMethodNotFound` for both; the connection already drops a
     notification's answer. A request to a `Notify` registration is then run and answered with a `null` result.

7. **When to build package `unstable`'s generation (item 1).** Added by the second assessment (G4, G9).
   * Recommended: now, the policy and the frozen contract; later, the emission. Phase 2 ships the manifest, the merge restriction,
     the deprecations, `example/agent`'s change and the non-gating API gate, and creates `unstable/` with its `doc.go` only. The
     generator does not emit a new unstable-only definition, and lists each one it leaves out. Emitting into `unstable` becomes a
     phase of the first schema-bump record that brings one, which then has a real definition to build and test against.
   * Alternative: build the emission now, as the PLAN first had it, proved by synthetic engine cases only.
   * Alternative: no package `unstable` at all; every new unstable definition stays untyped until it is stable.

#### Decided with the owner, 2026-10-25

The owner chose, out of three options for each:

| Question | Answer | Not chosen |
| :--- | :--- | :--- |
| 1. Compaction fallback | As recommended: `PresentCompaction` sends "Compaction {status}" and the error as an agent message; `PresentCompactionSummary` sends nothing | a typed fallback error; the summary's chunks sent as messages too |
| 2. `acptest` or the Client hosts | As recommended: `acptest` here (O1); the Client hosts are the next record | the Client hosts here (O2); both in this record |
| 3. Unstable names `v1` has exported | As recommended: frozen stubs in `acp`; only `unstable` may break | a published exemption; empty stubs without the frozen schema |
| 4. The PLAN | As recommended: one PLAN, as written | five PLANs; one PLAN with `acptest` first and the boundary last |
| 5. New unstable fields, variants and methods | As recommended: untyped until stable, with `Decode…` in `unstable` for a new variant | a registry in `acp`; the `Unstable` prefix rule only |
| 6. Notification or request in the mux | As recommended: `IsNotification(ctx)` in `connection.go` | an `ExtensionNotificationHandler` interface; no distinction |
| 7. When to build package `unstable`'s generation (added by the second assessment) | Not the recommendation: the owner chose "build the emissions now". The generator emits into `unstable` in this record, proved by synthetic engine cases until a schema bump brings a real definition | the policy now and the emission later; no package `unstable` |
| 8. Where a frozen type's schema is read (found in the PLAN's 2c; two options) | As recommended: the manifest holds a frozen `$defs`; each type built from it names the manifest as its origin; the root conformance tests read it | keep the origin on `schema.unstable.json` in 2c and repoint it before the first unstable bump |
| 9. What `unstable`'s generated code may use of `acp` (found in the PLAN's 2f) | As recommended: `acp`'s exported API only. A failed check returns `*acp.ValidationError` without the unexported message, so its text is the type's fallback, "Type: Field: Rule". An `acp`-typed value inside an `unstable` type is checked with its exported `Validate`, by the host's absolute-path rule. Types within `unstable` keep the full behaviour between themselves | exported hooks in `acp` for generated siblings; a validation runtime of `unstable`'s own |
| 10. The names in `unstable` (found in the PLAN's 2f; two options) | As recommended: the schema key, as in `unstable.FooRequest` and `unstable.AgentMethodFoo`, so a definition keeps its name when it moves to `acp`; a key that is also an `acp` name is refused | the `Unstable` prefix the merge gives in `acp` |

The PLAN's own question, P1, the releases: as recommended, two minors, one after the boundary and `acptest`, one after the three
runtimes. Not chosen: one minor at the end; a minor per phase.

*Amended 2026-10-25, after the PLAN's phase 3:* the owner changed P1's answer to one minor at the end, "we will do 1 release when
done". The release carries all five additions.

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
* Bad, because `acptest` is a new exported package that `make api` must compare from an empty baseline, and gobble-cli would still
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
* a new unstable field, union variant and method, each in a synthetic engine case, follow question 5's answer
* at `1.25.0`, adding the manifest and the merge restriction changes no generated declaration but the deprecation paragraphs;
  a synthetic case that changes or removes a manifest definition leaves its `acp` API listing identical; one whose unstable
  schema adds a property to a stable definition leaves that definition's fields as they were
* `example/agent` builds and lints with every unstable name deprecated
* `ExtensionMux` runs a `Notify` handler for a notification and not for a request of the same name, by question 6's answer

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
* TypeScript SDK 1.8.0 (2026-10-09) / 2.0.0 (2026-10-19): <https://github.com/agentclientprotocol/typescript-sdk/blob/main/CHANGELOG.md>
* Rust schema crate features, by version: <https://docs.rs/crate/agent-client-protocol-schema/latest/features>
* RFD updates, stabilization dates: <https://agentclientprotocol.com/rfds/updates>
* Claude Agent adapter PR #1154: <https://github.com/agentclientprotocol/claude-agent-acp/pull/1154>
* 0025's regeneration and `apidiff` counts: [0025-REPORT-schema-1-26-0-adoption-assessment.md](../reports/0025-REPORT-schema-1-26-0-adoption-assessment.md)

### Tree citations

* `schema/version` = `1.25.0`; `go 1.26.10`; `toolchain go1.27.3`; module `github.com/maccavelli/acp-go-sdk2`
* `Notice` schema, including the MUST to send only when advertised and the agent-message fallback:
  `schema/schema.json` definition `Notice`
* `CompactionUpdate` schema, including patch semantics: `schema/schema.json` definition `CompactionUpdate`
* Terminal truncation MUST: `types_gen.go:4094`
* `NewResourceNotFound`: `errors.go:246`, its doc comment from `:243`
* `Patch[T]`'s methods `IsZero`, `IsNull`, `Value`: `types_gen.go:22729`–`:22740`
* `Negotiated()`: `agent_gen.go:130`, `client_gen.go:134`; `ErrNotInitialized`: `errors.go:127`
* `scripts/check_api.py` compares `PACKAGES = ("", "agentproc")` (`:43`); items 1 and 5 add `unstable` and `acptest` to it

### Assessment, 2026-10-25

At the owner's request ("assess madr 0039. ensure it is grounded in facts and research, and that all assertions are proven"),
every assertion was checked: the tree's at `0ff0849` by script, and the external ones by reading each source. The draft's
decision and its five items are unchanged. What changed:

| Assertion in the draft | Finding | Change |
| :--- | :--- | :--- |
| `go doc -short .` lists 79 exported `Unstable*` identifiers | 65 names start with `Unstable`; 79 is the count of lines that mention one | corrected, with 0024's figure explained |
| `CallExtension` / `NotifyExtension` return `json.RawMessage` (`extensions.go:27`, `:92`, `:127`) | `NotifyExtension` returns `error` (`:109`, `:144`); `:127` is the client's `CallExtension` | corrected |
| `helpers.go` is "a runtime file that may name ACP types" | the runtime files are `connection.go`, `doc.go`, `errors.go`, `extensions.go`, which may not; `helpers.go` is excluded because it does | corrected |
| `PresentCompaction` takes `SessionCompactionUpdate`; `PresentCompactionSummary` takes `SessionUpdateCompactionSummaryChunk` | those are the variant types; `Notice` was the definition type; `CompactionUpdate` and `CompactionSummaryChunk` exist | the three take definition types |
| `Patch.Present` / `Patch.Null` | the methods are `IsZero`, `IsNull`, `Value` | corrected |
| compaction: Agents "MUST NOT send either variant unless …, for live delivery and for `session/load` replay" | the RFD says "must only send …" in lowercase, and "applies to replay as well as live delivery" | quoted as written |
| compaction RFD "Completed 2026-10-06" | stabilized 2026-10-06 (updates page); latest revision 2026-10-13, v2 `_meta` only | dated as stabilized |
| a chunk for an unknown ID first-seen-initializes | the RFD's first-seen rule does not name chunks | marked as inferred |
| the Claude and Codex adapters "retain fallback behavior" | the RFD says that of the Codex adapter only | quoted per adapter |
| PR #1154 closed compactions left open at turn end | it is in `claude-agent-acp`, merged 2026-09-22, and closes compactions on interrupted prompts | corrected |
| Rust `agent-client-protocol` minors track schema tags; `unstable_nes` a current feature | the schema crate is `agent-client-protocol-schema`, one release per tag under its own numbers; `unstable_nes` was removed in `1.12.0` | corrected |
| 0025's 115 / 40 | confirmed, measured against `v1.17.1` | baseline named; a PLAN re-measures |
| 0035 recorded both consumers built a mux | 0035 names gobble-cli's only, and lists a mux as "not recommended now" (0035 F) | corrected; item 4 states the reversal and its reason |
| `Abs` is a helper 0035 recorded both consumers need | 0035 names gobble-cli's; this repository has `testAbsPath` | corrected |
| `ScriptAgent`'s "ungated methods embed `AgentRefusals`" | `AgentRefusals` holds the five gated methods | corrected; the dependency on 0038's step 4 named |
| new unstable definitions emit only into `unstable` | a separate package cannot add a field, a variant or a method to an `acp` type | item 1 states the limit; question 5 added |
| existing unstable names get `Deprecated:` paragraphs | the generator refuses a note on a node the schema does not mark | item 1 names the generator change |
| `ExtensionMux` answers an unregistered request and ignores an unregistered notification | `HandleExtensionMethod` cannot tell them apart; nothing exported reads the connection's mark | `IsNotification` added; question 6 added |
| `CompactionTimeline` | 0035 F lists "a session-state reducer" as not recommended | item 3 says why one entity type differs |

### Assessment, second pass, 2026-10-25

At the owner's request ("re-evaluate the 0039 plan related to the boundry and generator changes … with precision"), item 1 and
the PLAN's phase 2 were checked against the generator's code and against outside sources. Four read-only investigations were
run; every claim below that the PLAN rests on was then read again directly, and G10 was computed directly.

| | Finding | Source |
| :--- | :--- | :--- |
| G1 | The merge replaces a stable definition a stable method reaches with its unstable version when they differ; adds an unstable-only definition unprefixed; copies an unstable-only method's subgraph under `Unstable` | `cmd/generate/internal/load/merge.go:102`–`:105`, `:302`–`:346` (`promoteChangedSharedDefs`, the replacement at `:329`–`:331`) |
| G2 | At `1.25.0`: 102 unstable-only definitions (56 prefixed, 46 not), giving ~~172~~ 174 exported package-level declarations (102 types, ~~23 nested or inline types~~ 25 inline variant types and no nested struct, 21 enum constants, 26 constructors; *corrected 2026-10-25 by the PLAN's 2a, which attributed each declaration through the model rather than by name prefix*); 2 stable properties whose default only the unstable schema changes (`AgentCapabilities.mcpCapabilities`, `InitializeResponse.agentCapabilities`); 10 fields on 8 stable-named types (`AgentCapabilities.Nes`, `.PositionEncoding`, `.Providers`; `ClientCapabilities.Nes`, `.Plan`, `.PositionEncodings`, `.Subagents`; `McpCapabilities.Acp`; `PromptResponse.Usage`; `SessionCapabilities.Fork`); 6 union-variant fields (5 on `SessionUpdate`, `McpServer.Acp`), each with its inline variant type, and one constructor, `NewMcpServerAcp` (*the types and the constructor named by the PLAN's 2a*); 16 methods (15 agent, 1 client), each an interface method, a dispatch case, a method constant and an outbound method | the merge re-run over the schema files; the ten fields read at `types_gen.go:98`, `:104`, `:117`, `:1533`, `:1542`, `:1548`, `:1564`, `:7868`, `:11119`, `:12359` |
| G3 | Origin is recorded for types (`Type.Unstable`, `Type.Origin`) and methods (`Method.Unstable`), not for fields, variants or members; `Type.Unstable` is false for a replaced stable-named definition | `model/model.go:112`–`:116`, `:355`–`:361`; `load/merge.go:12`–`:24` (`Provenance`) |
| G4 | The generator emits one package: `sdkPackage = "acp"`; `output.Check` type-checks one directory per call, resolving imports from disk; `output.Write` writes one directory; `TestGeneratedFilesUpToDate` and `TestCheckAcceptsRepository` cover the root only; an engine case has one output directory and one `want.api`, and a section name may not contain `/` | `emit/file.go:20`–`:21`, `:54`; `output/check.go:38`–`:101`; `output/write.go`; `main_test.go`; `enginetest/archive.go:60`–`:64` |
| G5 | A deprecation mark comes only from the schema's `deprecated` keyword; a protocol-table note on an unmarked node is refused; the notice text is "Deprecated: the ACP schema marks <place> deprecated." | `model/model.go:256`–`:268`; `model/protocol.go:909`–`:916`; `emit/types.go:177`–`:190` |
| G6 | SA1019 exempts the declaring package and its external tests, and flags any other package; `.golangci.yml` enables staticcheck with no SA1019 exclusion; `example/agent/main.go` asserts `acp.AgentExperimental` and implements the 15 methods | the staticcheck source, `staticcheck/sa1019/sa1019.go` (`checkIdentObj`); `.golangci.yml`; `example/agent/main.go:41` |
| G7 | `check_api.py` pools every package's incompatible changes and fails on any; a package missing from the base tag is stubbed, one missing from the working tree is a tool error; it has no tests | `scripts/check_api.py:43`, `:113`–`:127` |
| G8 | Precedent: grpc-go marks experimental APIs in place and exempts them from minor-release compatibility; the MCP Go SDK gated client OAuth behind a temporary build tag; build-tag API is not shown on pkg.go.dev, and `apidiff` sees only the default build | grpc-go `Documentation/versioning.md`; modelcontextprotocol/go-sdk issue #833; pkg.go.dev/about; `golang.org/x/exp/cmd/apidiff` |
| G9 | Every unstable definition at `1.25.0` would be in the manifest, so `unstable` would start with no generated declaration | G2 and item 1's manifest |
| G10 | 13 definitions both schemas have differ. The 8 the merge replaces differ only in added properties (10), properties with a changed `default` (2: `AgentCapabilities.mcpCapabilities`, `InitializeResponse.agentCapabilities`) and added union entries (5 on `SessionUpdate`, 1 on `McpServer`). The other 5, `AgentRequest`, `ClientRequest` and `ClientNotification` (a property's `anyOf`) and `AgentResponse` and `ClientResponse` (one `anyOf` entry each), no stable method reaches, so they are generated from the stable schema. No description or other top-level keyword differs | a key-by-key diff of the 13 over `schema/schema.json` and `schema/schema.unstable.json`, in the session's scratch directory; the merge re-run for which are replaced |

Consequences for the draft: item 1 gains "How it is built", which states the mechanism; the PLAN's phase 2 is rewritten from it
(the PLAN's "Revision, 2026-10-25"); question 7 is added; and "Confirmation" gains three items. The decisions recorded in
"Decided with the owner" are unchanged.

### Assessment, first pass

Confirmed as written, with the source read: the counts of `$defs` (178, 277) and `meta.json` methods (13, 11, one protocol method);
`schema/version`, `go` and `toolchain` lines; every other line citation; 124 `io.Pipe()` calls in 27 test files; the schema
release's four items and its date; the Notices RFD's shape, gate, fallback and replay rule; the compaction patch, chunk,
first-seen, terminal-status and unknown-status rules; the extensibility page's `_` prefix, `_meta` capability example,
trace keys (SHOULD), root-field MUST NOT and notification SHOULD; the elicitation SHOULDs; the TypeScript changelog's two entries;
and the Go modules reference, `golang.org/x/exp` and module-compatibility quotations.

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
* A WebSocket or Unix-socket transport (0035 F: needs a dependency, needs its own MADR). *(WebSocket decided by 0043; see
  "Amended by 0043-MADR-remote-http-and-websocket-transport.md, 2026-10-29".)*
* Bounded `Wait` (0026 question 4)
* Changing `PromptOverlapRefuse`'s code (0021)
* Adding methods to `Agent` or `Client` (that is an incompatible `apidiff` change for every embedder)

The owner answered questions 1–6 on 2026-10-25 ("Decided with the owner"), and question 7, from the second assessment, the same
day: "build the emissions now". The PLAN is approved: "committed, proceed".

#### Amended by 0043-MADR-remote-http-and-websocket-transport.md, 2026-10-29

[0043-MADR-remote-http-and-websocket-transport.md](0043-MADR-remote-http-and-websocket-transport.md), `accepted` on 2026-10-29,
is the MADR of its own that "Out of scope" asked for. It decides the WebSocket half of "A WebSocket or Unix-socket transport": ACP's
remote transport, Streamable HTTP and WebSocket, in package `remote`. Its WebSocket implementation, `github.com/coder/websocket`
v1.8.15, is the dependency named there. Unix sockets stay out of scope; 0043 lists them under "Not in this record". Like
`unstable`, item 1 here, `remote` is outside the compatibility promise, and `make api` reports its changes without gating them.

*Note, 2026-10-31: gobble-cli was written `<cli-b>` in this record until the owner lifted that redaction ([0044-PLAN-notification-queue-back-pressure.md](0044-PLAN-notification-queue-back-pressure.md), phase 5).*
