---
status: proposed
date: 2026-10-14
decision-makers: Repository owner
---

# Resolve issues #4 and #5 before `v1.14.0`: a prompt-overlap policy option and a three-state `Patch[T]` for patch fields

## Context and Problem Statement

The owner asked on 2026-10-14, before tagging `v1.14.0`
([0020-MADR-retract-accidental-v1-tag.md](0020-MADR-retract-accidental-v1-tag.md)), for the two open GitHub issues to be evaluated
against the codebase and resolved in the most idiomatic way. Both issues were opened by the repository owner's account. Every
statement below was checked on `c1b1185`, the commit `v1.14.0` is to be tagged on, with `go1.27.2`, unless it names another commit.

### Why the timing matters

`v1.14.0` is the module's first `v1` release. "Keeping Your Modules Compatible" (<https://go.dev/blog/module-compatibility>) says
"breaking changes to a v1+ module must happen as part of a major version bump", that "you can almost always add a field or remove an
unexported field without breaking compatibility" provided its "zero value is meaningful and preserves the old behavior", and, for
a function, that "options are passed as variadic arguments, and each option is a function that changes the state". So, after the
tag, a change that alters the type of an exported field needs a new module path ending in `/v2`, while an added option, type or
field whose zero value keeps today's behaviour changes no existing identifier. The post does not discuss constants.

### Issue #4: "Make prompt supersession an agent-side choice"

Opened 2026-10-14T02:09:33Z, no comments. It asks for an agent-side choice among **supersede** (today's behaviour, staying the
default), **refuse** (answer the second `session/prompt` with an error and leave the running turn alone) and **pass through** (call
`Prompt` concurrently and let the agent decide). It names a downstream agent that carries its own methods for mid-turn input and
would rather queue or refuse.

What the code does on `c1b1185`:

* The generated `session/prompt` case, `agent_gen.go:645-698`, emitted by `promptCase` (`cmd/generate/internal/emit/dispatch.go:33-87`),
  stores one turn per session in `sessionCancels map[string]*promptTurn` (`agent_gen.go:20`). Under `a.mu` it cancels the session's
  stored turn with `fmt.Errorf("%w: superseded by a new prompt", ErrPromptCancelled)` and stores the new one; a prompt whose receipt
  is older than the stored turn's is itself cancelled at once (`agent_gen.go:662-676`). This happens before `a.agent.Prompt` is
  called (`:678`), so the agent cannot choose otherwise. The issue cites `agent_gen.go:553-560` at `v0.17.1`; the code has moved, not
  changed in this respect.
* `session/cancel` (`agent_gen.go:519-525`) and `session/close` (`:542-548`), emitted by `cancelActivePrompt`
  (`dispatch.go:89-109`), cancel the one stored turn when it was received before them, delete its entry, and keep their receipt in
  `lateCancels` through `keepCancel` (`connection.go:1211-1231`) for a prompt received before them and not yet stored.
* The behaviour is documented in `README.md:346` ("a second prompt for the same session cancels the first with an error that wraps
  it") and `errors.go:115`, and pinned by `TestPromptSupersededKeepsSuccessorCancel` (`cancel_test.go:192-213`, which requires the
  cause to wrap `ErrPromptCancelled` and say "superseded") and `TestCancelOrderedAcrossTwoPrompts` (`cancel_test.go:563`).
* The records: [0016-REPORT-acp-v1-conformance-audit.md](../reports/0016-REPORT-acp-v1-conformance-audit.md), lines 119-120, calls
  supersession "a safety decision, not a violation", "worth a sentence in whichever record next touches the prompt turn";
  [0017-MADR-remediate-acp-v1-conformance-violations.md](0017-MADR-remediate-acp-v1-conformance-violations.md), P1, fixed the
  successor's lost cancel and kept supersession.

What ACP says: nothing, at the versions this SDK follows. The v1 pages `prompt-turn.mdx`, `overview.mdx`, `session-setup.mdx` and
`cancellation.mdx` at upstream `e5bcca615c2560a7d707365c773ae768ae595e88` contain no rule for a `session/prompt` received while a turn
of the same session runs, and neither do the schema's `PromptRequest`, `PromptResponse` and `CancelNotification` descriptions. The
v2 draft `docs/rfds/v2/prompt.mdx`, at the same commit, line 88: "This RFD does not specify queueing, steering, or whether agents
insert new prompts while busy." `CancelNotification` is session-scoped: "Notification to cancel ongoing operations for a session."

ACP's `ErrorCode` (schema `1.25.0`, both files) defines `-32700`, `-32600`, `-32601`, `-32602`, `-32603` ("Reserved for
implementation-defined server errors"), `-32800`, `-32000` and `-32002`; none means "busy". `WithMaxConcurrentRequests` already
answers a refusal of this kind `-32603` with a `data` object
([0019-PLAN-api-decisions.md](0019-PLAN-api-decisions.md), B18).

The SDK's precedent for a runtime choice of protocol handling is `NotificationOverflowPolicy` (`connection.go:418-456`): an `int`
type whose zero value, `OverflowCloseConnection`, is the default, a second value `OverflowDropNewest`, and
`WithNotificationOverflowPolicy(p)`, a `ConnectionOption`, which treats "any value this package does not define" as the default.

### Issue #5: "Patch-semantic fields cannot represent null"

Opened 2026-10-14T12:37:55Z, no comments. It reports that fields with patch semantics (omitted: unchanged; `null`: cleared; a
value: replaced) decode an omitted key and an explicit `null` alike, and cannot be sent as `null`, and proposes a generic three-state
type, encoded with `omitzero`.

**Which fields have patch semantics.** A scan of every definition and property description in `schema/schema.json` and
`schema/schema.unstable.json` for `null` together with "clear", "unchanged", "patch", "retain" or "preserve", excluding the
descriptions that call the two "equivalent" or say "both mean", finds these, and no other:

| Definition | Properties | Schema file | Where the text says it |
| :--- | :--- | :--- | :--- |
| `CompactionUpdate` | `summary`, `error`, `_meta` | both | definition: "`summary`, `error`, and `_meta` have patch semantics: omission leaves the stored value unchanged, `null` clears it, and a concrete value replaces it. `summary: []` also clears the summary." |
| `SessionInfoUpdate` | `title`, `updatedAt` | both | each property: "Set to null to clear." |
| `SubagentUpdate` | `title`, `description`, `capabilities`, `state`, `_meta` | unstable | definition: "Omitted patch fields keep their previous values; `null` clears them." Each property repeats it. |
| `SessionMessage` | `content`, `_meta` | unstable | `content`: "Omitted leaves content unchanged; `null` or `[]` clears it."; `_meta`: "Omitted leaves metadata unchanged; `null` removes it." |

The scan's other matches state the opposite: `SessionMessage` and `SessionMessageChunk`'s `senderSessionId` and `recipientSessionId`
("omission or `null` retains a known value"), and `McpError.data` ("explicit null is retained"), which
[0019-MADR-remediate-debugging-pass-findings.md](0019-MADR-remediate-debugging-pass-findings.md) already decided (G3, decision 5).
`ToolCallUpdate` says "only changed fields need to be included" but gives `null` no meaning except for `name`, where "Omitting it or
sending `null` both mean that the existing name is left unchanged."

The issue names `CompactionUpdate` and `SubagentUpdate` only. `SessionInfoUpdate`, in the stable schema, and `SessionMessage` have the
same semantics.

**The Rust reference agrees field for field.** `agent-client-protocol-schema/src/v1/client.rs` at the `schema-v1.25.0` tag's commit
`4cf3dd858c819fc3ab99ae53f76883ede0345a10`, and unchanged at `e5bcca6`, types exactly these 12 fields as `MaybeUndefined<T>`
(`client.rs:291`, `:302`, `:776`, `:781`, `:790`, `:909`, `:918`, `:928`, `:938`, `:952`, `:1729`, `:1734`), each with
`#[serde(default, skip_serializing_if = "MaybeUndefined::is_undefined")]`. `serde_util.rs:578-586` defines it as a three-variant enum:
`Undefined` ("The field was not present", the `#[default]`), `Null` ("present with a JSON `null` value") and `Value(T)`; its
`update_to(&mut Option<T>)` (`:722`) applies a patch to a stored value. `v1/mcp.rs` types `McpError.data` the same way. GitHub code
search of the upstream repository finds `MaybeUndefined` under `src/v1/` only in `client.rs` and `mcp.rs`.

**The generated Go fields.** The model gives each of the 12 properties to two Go types, the definition's and the `SessionUpdate`
variant built from it, so 24 fields in 8 types: `CompactionUpdate` and `SessionCompactionUpdate` (`Meta map[string]any`, `Error
*string`, `Summary []ContentBlock`); `SessionInfoUpdate` and `SessionSessionInfoUpdate` (`Title`, `UpdatedAt *string`);
`SubagentUpdate` and `SessionSubagentUpdate` (`Meta`, `Capabilities *SubagentSessionCapabilities`, `Description *string`, `State
*StateUpdate`, `Title *string`); `SessionMessage` and `SessionUpdateSessionMessage` (`Meta`, `Content []ContentBlock`); every one
tagged `omitempty`. Outside the generated files, the tree names one of them only as a variant name in an expected error message
(`union_wire_test.go:314`); no hand-written code, test or example constructs these types or reads these fields.

**Why they lose the third state.** Each of the 8 generated `UnmarshalJSON` methods decodes the field from a `json.RawMessage`
shadow and passes it to `orDefault` (`types_gen.go:19673-19686`), or, for an array with `x-deserialize-skip-invalid-items`, to
`skipInvalid` (`:19688-19709`). `orDefault` returns the zero value when the raw value is empty (absent) or does not decode, and
`null` decodes without error into a nil pointer or map, the same zero value; `skipInvalid`'s comment says "An empty raw value or null
is no items". On encode, `omitempty` drops a nil pointer, a nil or empty slice and a nil or empty map.

**Reproduced** in a scratch copy of `c1b1185`, under the default build and under `GOEXPERIMENT=nojsonv2`, with identical output:

| Input or value | Result |
| :--- | :--- |
| `compaction_update` without `error`, `summary`, `_meta` | `Error` nil, `Summary` nil, `Meta` nil |
| the same with `"error":null,"summary":null,"_meta":null` | `Error` nil, `Summary` nil, `Meta` nil |
| the same with `"error":"boom","summary":[],"_meta":{}` | all three non-nil, `Summary` empty |
| `session_info_update` without, and with `"title":null,"updatedAt":null` | `Title` nil and `UpdatedAt` nil, both times |
| `subagent_update` without, and with `"title":null,"capabilities":null,"state":null` | all nil, both times |
| `json.Marshal(SubagentUpdate{SessionId: "child"})` | `{"sessionId":"child"}`: `null` cannot be sent |
| a `SessionCompactionUpdate` with `Summary: []ContentBlock{}` and `Meta: map[string]any{}` | neither key is written, so the `summary: []` clear the schema names cannot be sent either |

**Nothing in the schema's structure marks these fields.** In `schema/schema.unstable.json`, which names all 178 definitions of the
stable file among its 277, and with descriptions removed, the three patch `_meta` properties share their exact schema,
`{"additionalProperties": true, "type": ["object", "null"], "x-deserialize-default-on-error": true}`, with 199 other properties, and
the five patch strings share `{"type": ["string", "null"], "x-deserialize-default-on-error": true}` with 39 others. Only the prose
differs, so a generator cannot find them by shape.

**The records.** [0012-MADR-schema-fidelity-framework.md](0012-MADR-schema-fidelity-framework.md), "Amendment 2026-10-11, a null raw
value is an absent one", made a `null` in an optional *raw* field absent, because ACP v1 makes `null` and omission equivalent for
`rawInput` and `rawOutput`, and said of the types it rejected: "Neither gives ACP v2's three states, which will need a type of their
own." The patch fields are not raw fields (`NullIsAbsent` applies to raw fields only, `cmd/generate/internal/model/fields.go:416`,
`:435`), so that amendment does not cover them, and v1 itself needs the three states for these 12 fields.

**What Go provides, checked.** The `encoding/json` documentation: `omitzero` omits a field with a zero value, and "If the field type
has an "IsZero() bool" method, that will be used to determine whether the value is zero"; Unmarshal calls `UnmarshalJSON` "including
when the input is a JSON null". A probe module (`go1.27.2`, default build and `GOEXPERIMENT=nojsonv2`, identical output) with a
generic struct type, its state unexported, `IsZero`, `MarshalJSON` and `UnmarshalJSON`, and fields tagged `omitzero` showed:

* absent: the key is omitted, and decoding a missing key leaves the zero value without calling `UnmarshalJSON`;
* null: `"title":null`, `"summary":null` and `"_meta":null` are written, and decoding `null` calls `UnmarshalJSON` and records null;
* value: `""`, `[]` and `{}` are written as such and decode back as values, so empty values survive both ways;
* a value holding a nil slice encodes as `null`;
* the type is comparable with `==` when `T` is.

How should the SDK resolve both issues, and which of the two must land before `v1.14.0` is tagged?

## Decision Drivers

* **Compatibility at the tag.** After `v1.14.0` an incompatible API change needs `/v2`; an addition whose zero value keeps today's
  behaviour does not.
* **ACP.** Issue #5 is a rule of the v1 schema the SDK does not meet: senders cannot clear, receivers cannot see a clear. Issue #4 is
  outside the specification, which is silent on overlapping prompts, so the SDK's choice must stay a choice.
* **The Rust reference** is ACP's own typing of the same schema, and its `MaybeUndefined<T>` is a three-state type on exactly the 12
  fields.
* **Idiom in this codebase.** Protocol knowledge the schema does not express lives in the protocol table
  (`cmd/generate/internal/model/acp.go`); runtime choices are `ConnectionOption`s with typed policy constants whose zero value is the
  default (`NotificationOverflowPolicy`); a generated codec handles wire rules field by field.
* **Correct by construction.** A type should not admit contradictory states, and the zero value of a struct should keep meaning
  "send nothing", as `omitempty` fields do today.
* **Proof.** Each behaviour must be testable both ways, under both JSON builds and on Windows.

## Considered Options

Issue #4, the choice of what a second prompt does:

* A1. A `ConnectionOption`, `WithPromptOverlapPolicy`, with three policies: supersede (default), refuse, concurrent.
* A2. The same option with two policies: supersede (default) and concurrent; refusal left to the agent's `Prompt`.
* A3. An optional interface the `Agent` implements, returning the policy.
* A4. Keep supersession, and document it.

Issue #5, the representation of patch fields:

* B1. A generic `Patch[T]` with an unexported state, generated for the fields a new protocol-table rule lists, tagged `omitzero`.
* B2. Keep the fields and add a `bool` beside each, set when the key was `null` and sending `null` when set.
* B3. Make the patch fields `json.RawMessage`.
* B4. Keep the fields, and document that `null` reads as omitted and cannot be sent.

## Decision Outcome

Chosen options: "A1" for issue #4 and "B1" for issue #5, because A1 gives each behaviour the issue asks for through the mechanism the
SDK already uses for a runtime policy, without changing the default, and B1 is the Go form of the type ACP's reference uses on the
same 12 fields, cannot hold a contradictory state, and is the one representation that both receives and sends all three states. B1
changes the type of 24 exported fields, so it must ship in `v1.14.0` or wait for `/v2`; A1 only adds, so it could follow in a later
`v1` minor release, but ships with B1 at the owner's request.

### Issue #4: `WithPromptOverlapPolicy`

* A new type `PromptOverlapPolicy int` with `PromptOverlapSupersede` (the zero value, the default), `PromptOverlapRefuse` and
  `PromptOverlapConcurrent`, and `WithPromptOverlapPolicy(p PromptOverlapPolicy) ConnectionOption`, documented as applying to an
  `AgentSideConnection` and ignored elsewhere; a value the package does not define is the default, as for
  `WithNotificationOverflowPolicy`.
* **Supersede** is today's code, unchanged, and keeps every test that pins it.
* **Refuse:** a `session/prompt` that finds a turn of its session stored is answered at once, without calling `Prompt`, with
  `-32603` and `data` `{"error": "a prompt turn is already running for this session", "sessionId": <id>}`; the running turn is not
  touched. A turn that `session/cancel` or `session/close` has cancelled is no longer stored (`agent_gen.go:523`, `:546`), so a
  prompt sent after a cancel, the usual "stop, then ask again", is not refused. Which of two prompts sent together is refused is the
  one that reaches the dispatch second.
* **Concurrent:** every `session/prompt` calls `Prompt`. The session's turns are kept as a set, each removing only itself;
  `session/cancel` and `session/close` cancel every stored turn of the session received before them, and the kept-cancel rules
  (`lateCancels`, `keepCancel`) apply to each turn as they do to one. The agent queues, steers or refuses in its own `Prompt`; an
  error it returns is answered as any handler error.
* `$/cancel_request` keeps cancelling its own request in every policy.

### Issue #5: `Patch[T]`

* An exported generic type, emitted by the generator into `types_gen.go` when the schema has a patch field, as the decode helpers
  are emitted once when used, so engine cases need no new hand-written file: a struct with an unexported state (absent, null,
  value) and value; constructors for a null and for a value; `IsZero` (absent, so `omitzero` omits it), `IsNull`, and `Value() (T,
  bool)`; `MarshalJSON` and `UnmarshalJSON`. The zero value is absent, so a struct literal that sets nothing sends nothing, as today.
* A protocol-table rule list, `PatchFields`, of `{Definition, Property, Source}` entries, the 12 of the table above, each with its
  specification sentence; the model resolves each on its definition and on every type built from it, as it does `AbsolutePaths`,
  and refuses a rule whose property the definition lacks.
* Each listed field becomes `Patch[T]`, `T` the element type it has today (`string`, `[]ContentBlock`, `map[string]any`,
  `SubagentSessionCapabilities`, `StateUpdate`), tagged `omitzero` without `omitempty`. The generated `UnmarshalJSON` maps the raw
  value: empty to absent, `null` to null, a value that decodes to that value (an array through `skipInvalid`), and one that does not
  decode to absent, which is what `x-deserialize-default-on-error` gives the field today. `Validate` reaches a nested value through
  the patch.
* Documentation states the receiver's rule, that a value of `[]` also clears `summary` and `content`, and that a value holding a nil
  slice is sent as `null`.
* `McpError.data` stays `json.RawMessage`: a raw value already holds the three states (empty, the bytes `null`, a value), and G3 of
  0019 decides it.

### Consequences

* Good, because a client sees and an agent sends every clear the schema defines, under both JSON builds, and empty values (`[]`,
  `{}`, `""`) can be sent where `omitempty` dropped them.
* Good, because patch fields are listed in the protocol table with their sentences, where a schema update that adds one is a table
  change reviewed like any other.
* Good, because an agent can refuse or run overlapping prompts, and the default, the README and the tests stay as they are.
* Good, because nothing is changed after the tag that `v1` would forbid: B1's type change lands before it, and A1 is additive.
* Bad, because 24 exported fields in 8 types change type, a compile error for any code that sets or reads them; the tree has none, and
  the release notes must name them with the replacement code.
* Bad, because `Patch[T]` adds an SDK type where Go code might expect a pointer, and reading a value takes `Value()`.
* Bad, because the concurrent policy turns `sessionCancels` into a set and touches the bookkeeping 0017 and phase 1 of 0019 made
  correct; its tests must cover cancel ordering again with several turns.
* Neutral, because the refuse policy's choice between two prompts sent together follows dispatch order, not receipt order, which
  ACP does not define either way.

### Confirmation

* Issue #5: `TestConformanceRoundTrip` keeps passing over every schema instance, now with `null` kept; a test per state per field,
  decode and encode, under both builds, including `[]`, `{}` and `""` values and a malformed
  value read as absent; a guard test that scans the committed schemas' descriptions with the wording above and fails when a match is
  not in `PatchFields`; an engine case for a patch field with a behaviour test; the API listing shows the 24 field types changed and
  `Patch` added; each seen to fail on a planted break.
* Issue #4: tests per policy: refuse answers `-32603` and leaves the running turn uncancelled, and does not refuse after a cancel;
  concurrent calls `Prompt` twice, and one `session/cancel` cancels both turns received before it but not one received after; the
  supersede tests unchanged; each seen to fail on a planted break.
* The PLAN that implements this record names the files, the steps and the gates.

## Pros and Cons of the Options

### A1. `WithPromptOverlapPolicy`, three policies

* Good, because it is the SDK's established form for a runtime policy: a typed constant, the zero value the default, set by a
  `ConnectionOption` at construction, before reading starts.
* Good, because refusal needs to know whether a turn is running, which the SDK already tracks; built in, it needs no bookkeeping in
  the agent.
* Good, because a later policy is an added constant, which changes no existing identifier.
* Bad, because a `ConnectionOption` also reaches `NewConnection` and `NewClientSideConnection`, which ignore it.
* Bad, because it puts a prompt-turn setting in the hand-written `connectionConfig`, read by generated code, where protocol knowledge
  otherwise lives in the table and the generated sides.

### A2. Two policies, refusal in the agent

* Good, because it adds less: the concurrent policy alone lets an agent refuse by returning an error from `Prompt`.
* Bad, because each agent that only wants to refuse must then track its running turns, which the SDK already does.
* Bad, because the issue asks for refusal by name.

### A3. An optional interface on the `Agent`

* Good, because it needs no change to `ConnectionOption`.
* Bad, because the SDK's optional interfaces (`AgentLoader`, `ExtensionMethodHandler`, `AgentConnAware`) say what a value can do,
  not how a connection is configured; a method that returns a constant is configuration in disguise.
* Bad, because two connections of the same agent type could not choose differently without wrapping the agent.

### A4. Keep supersession

* Good, because nothing changes.
* Bad, because an agent that queues or refuses cannot, which is the issue, and ACP leaves the choice open.

### B1. `Patch[T]`, listed in the table

* Good, because it matches the Rust reference's `MaybeUndefined<T>` on the same 12 fields, and the three states the schema text
  names.
* Good, because the state is unexported: no value can be both null and set.
* Good, because the zero value is absent, so `omitzero` omits it and a struct literal keeps sending nothing; the probe showed this
  under both builds.
* Good, because it sends and receives `null`, values and empty values; today's fields can neither send nor receive `null`, and
  cannot send an empty array or object.
* Bad, because it changes 24 field types, which is breaking.

### B2. A `bool` beside each field

* Good, because it adds fields only, so it would be compatible after the tag.
* Bad, because a field could be set while its flag says `null`, a state the wire cannot express.
* Bad, because it adds 24 fields, and still cannot send `[]` or `{}` unless `omitempty` also goes.
* Bad, because the Rust reference, the one other implementation this record examined, types them as one three-state value.

### B3. `json.RawMessage`

* Good, because a raw value already has three states.
* Bad, because typed fields (`[]ContentBlock`, `StateUpdate`) would lose their Go type and their validation.

### B4. Document the limitation

* Good, because nothing changes.
* Bad, because the SDK would keep failing a v1 rule on four definitions, two of them stable.

## More Information

* **Evidence.** Issues #4 and #5, read with `gh issue view` on 2026-10-14; the schema scans and probes above, run in scratch copies of
  `c1b1185` and in a scratch module, none in the tree. Upstream: `agentclientprotocol/agent-client-protocol` at `e5bcca6`
  (documentation, `schema-v1.25.0`'s Rust files) and at `4cf3dd8` (the tag's commit), read through the GitHub API. Go:
  <https://pkg.go.dev/encoding/json> (`omitzero`, `UnmarshalJSON` on `null`) and <https://go.dev/blog/module-compatibility>.
* **What the issues say that does not hold, or is incomplete.** Issue #5 lists two definitions, `CompactionUpdate` and
  `SubagentUpdate`; four have patch semantics, and the two it omits are `SessionInfoUpdate`, which is stable, and `SessionMessage`. It says `summary: []` "survives as distinguishable"; it does on decode, but a sender cannot write
  it, because `omitempty` drops an empty slice. Issue #4's line references are to `v0.17.1`; the behaviour is unchanged on `c1b1185`.
* **Related records.** [0012-MADR-schema-fidelity-framework.md](0012-MADR-schema-fidelity-framework.md) (null-is-absent for raw
  fields), [0017-MADR-remediate-acp-v1-conformance-violations.md](0017-MADR-remediate-acp-v1-conformance-violations.md) (P1, the
  prompt turn), [0019-MADR-remediate-debugging-pass-findings.md](0019-MADR-remediate-debugging-pass-findings.md) (B5 and B7, the
  kept cancels; G3, `McpError.data`), [0020-MADR-retract-accidental-v1-tag.md](0020-MADR-retract-accidental-v1-tag.md) (the
  release), [0016-REPORT-acp-v1-conformance-audit.md](../reports/0016-REPORT-acp-v1-conformance-audit.md).
* **Implementation.** A PLAN, written once this record is accepted. Until it lands and is verified, `v1.14.0` is not tagged.
