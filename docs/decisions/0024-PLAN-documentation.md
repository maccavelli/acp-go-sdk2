---
status: in-progress
date: 2026-10-19
associated-madr: "0024-MADR-go-sdk-best-practices-on-a-go-1-26-9-floor.md"
---

# Implement phase 2 of 0024: documentation

Associated MADR: [0024-MADR-go-sdk-best-practices-on-a-go-1-26-9-floor.md](0024-MADR-go-sdk-best-practices-on-a-go-1-26-9-floor.md)

## Goal

Implement phase 2 of the MADR's outcome, as the owner decided it on 2026-10-19. It covers findings A5, A6, A7, A8, A10, A11 and V4,
with question 4's answer for A7:

* **A5:** a concurrency contract on `Connection`, `AgentSideConnection`, `ClientSideConnection`, `Agent` and `Client`.
* **A6:** each typed method's doc names the errors it returns.
* **A7:** each generated type's doc comment starts with its name, `// <Name> is …`, before the schema's description. This amends
  [0012-MADR-schema-fidelity-framework.md](0012-MADR-schema-fidelity-framework.md)'s mapping.
* **A8:** a package overview with a happy path in `doc.go`, a Go quickstart in the README, and per-symbol `Example` functions with
  `// Output:`.
* **A10:** the options' semantics.
* **A11:** doc notes on the wire-schema types the runtime does not use, and on `ErrorCode` against `RequestError`.
* **V4:** `RELEASING.md` names the release notes and the API listing.

Every change is to comments, documentation, examples or tests. The compiled code, the exported API and the wire bytes do not change; a
check compares the syntax trees without comments to prove it. The release is proposed as the patch `v1.16.3`, which the owner confirms
at the tag.

Phase 3's documentation stays in phase 3, as the MADR places it: A2's note on a handler's error text, A14's on validation messages,
and A1's on handler panics.

## Facts this plan rests on

Each was read on `8c9ad41` on 2026-10-19. Step 1 re-asserts the code facts with a script.

| For | The tree as it is |
| :--- | :--- |
| A5 | `connection.go:290`: "Connection is a simple JSON-RPC 2.0 connection over line-delimited JSON." `cmd/generate/internal/emit/dispatch.go:114` and `:132` emit the side connections' one-line docs. No doc of the five types states concurrent use. Requests are handled each on its own goroutine (`connection.go:1279`); notifications one at a time in order (`processNotifications`, `connection.go:1467`; corrected in step 1 to `:1471`). |
| A6 | `outboundDoc` (`emit/dispatch.go:665`) writes "sends the <wire> request to <peer> and returns its response", then the interface method's doc. The paths a typed method's errors come from: `gateMessage` (`types_gen.go:22293`); `encodeParams` and `validateOutbound` (`connection.go:1762`, `:165`); `sendRequestSent`; `toReqErr` (`errors.go:228-242`). |
| A6 | `gateMessage`: initialize passes; before initialize every other method returns `fmt.Errorf("%s: %w", method, ErrNotInitialized)`; after it, `n.Allows(method)` and the params' gates. `validateOutbound` returns a `*ValidationError` unwrapped. A response is checked by `checkInbound` and its error wrapped `"%s response: %w"`. |
| A7 | 370 of the 378 exported types in `types_gen.go` have a doc that does not start with their name. `emitSchemaDef` (`emit/types.go:167`) writes `t.Doc`, the schema's description, except for unions, which `emitUnion` documents. `defPlace` (`:155`) and `schemaVariantPlace` (`:159`) name a definition and a variant, as the methods' docs use them; nested structs use "the %q property of …" (`:222`). The 8 that comply: `Cost`, `Agent`, `AgentLoader`, `AgentExperimental`, `Client`, `ClientExperimental`, `Patch`, `Negotiation`. |
| A8 | `doc.go` is four lines with no happy path. The README's first Go block is at line 199 (corrected in step 1 to 204). There are three `Example` functions, all package-level. `mdsh`, which the `Makefile`'s `README.md` target runs, is not installed on this host. |
| A10 | `connection.go:393-456` and `:602`: `n <= 0` leaves `WithMaxQueuedNotifications` and `WithMaxMessageSize` unchanged but resets `WithMaxConcurrentRequests`; options apply in order; a nil option panics. None of this is documented together. |
| A11 | The ten JSON-RPC envelope types (`AgentRequest` … `ClientError`), `Error`, `ErrorCode` (`int32`, constants at `types_gen.go:5945-5975`) and `CancelRequestNotification` are generated from the schema, and the runtime uses none of them. `RequestError.Code` is an `int`. |
| A11 | `cmd/generate/internal/model/acp.go:11` declares `ACP`, the protocol table, whose rules `Build` checks against the schema. `TestProtocolTableRefuses` and `TestProtocolTableAppliesToCommittedSchema` (`internal/model/protocol_test.go:186`, `:440`) test its rules. |
| V4 | `RELEASING.md` names neither `docs/reports/NNNN-REPORT-release-notes-*` nor the API listing. |
| — | `docs_test.go:19` `TestExportedDeclarationsDocumented` requires a doc on every exported declaration. It does not require the doc to start with the name. |

## Scope

In scope:

* **The generator:** `cmd/generate/internal/emit` (`types.go`, `dispatch.go`) and `internal/model` (`acp.go` and the `Protocol` type),
  with their tests and engine cases.
* **The regenerated files:** `types_gen.go`, `agent_gen.go`, `client_gen.go`, with comment changes only.
* **Hand-written docs:** `connection.go` (comments only), `doc.go`, `README.md`, `RELEASING.md`.
* **Tests:** new `Example` functions, a README test, and doc tests.
* **The records:**
  * 0012's annotation;
  * this PLAN and its row;
  * the release notes `docs/reports/0024-REPORT-release-notes-v1-16-3.md` and their row.

Out of scope:

* phase 3's behaviour and its documentation (A1, A2, A3, A4, A14);
* phase 4 (V2, V3);
* the 41 constants in `constants_gen.go` that rely on a group comment, which Go's conventions and `revive` accept;
* any code, API or wire change.

## Design

### A7: generated type comments

* **The opening line.** `emitSchemaDef`, `emitUnion` and the nested-struct path write `// <Name> is <place>.` as the comment's first
  line. `<place>` is the phrase that type's `Validate`, `MarshalJSON` and `UnmarshalJSON` docs already use:
  * `defPlace(t)`, for a definition;
  * `schemaVariantPlace(t, vi)`, for a union's variant type;
  * `the "<key>" property of <defPlace>`, for a nested struct.

  The emitter computes the place from the model, so the 54 types with no such method get one too: `SessionId`, `StopReason` and the
  like.
* **The description follows** after a blank comment line, unchanged.
* **A doc that already starts with "<Name> "** is left as it is: the 8 listed in "Facts".
* **Example:**

  ```go
  // AgentAuthCapabilities is the AgentAuthCapabilities schema definition.
  //
  // Authentication-related capabilities supported by the agent.
  type AgentAuthCapabilities struct {
  ```

* **The check.** A new root test, `TestExportedTypeDocsStartWithName` (`docs_test.go`), requires every exported type's doc to start
  with its name followed by a space.
* **The record.** 0012 gains a dated annotation, under its mapping of `description` to doc comments, naming this amendment.

### A6: the errors of a typed method

`outboundDoc` appends one paragraph to each typed outbound method's doc, after the interface method's doc.

For a request:

```text
Before anything is sent, it returns an error wrapping ErrNotInitialized
until initialize completes, a *CapabilityError when what initialize agreed
does not allow the method or a property or variant of params, and a
*ValidationError when params break the schema. Once sent, it returns the
peer's error answer as a *RequestError; a *RequestError wrapping
context.Canceled or context.DeadlineExceeded when ctx ends, or
ErrPeerDisconnected or ErrConnectionClosed when the connection does; and an
error wrapping *ValidationError or *CapabilityError when the response breaks
the schema or what initialize agreed.
```

**Variations:**

* **A notification** has no response. Its paragraph keeps the first sentence, and says the second only of the context and the
  connection.

Annotated during step 2, 2026-10-19: the paragraph as built differs from the sample above in three places, each a correction to what
the code does. A context's end wraps `context.Cause(ctx)`, which is `context.Canceled` or `context.DeadlineExceeded` only when no
cause was given. Two claims are added: the write's error when writing fails, and a `*RequestError` for a response that does not
decode. The clauses of each list are joined with semicolons, as some hold an "or" of their own.
* **`Initialize`** does not wrap `ErrNotInitialized`. It also returns a `*ProtocolVersionError` for a version this SDK does not speak,
  asked or answered.
* **The client's `Authenticate`** names `ErrTerminalAuthMethod`.
* **The model drives the variations.** It knows which method initializes and which carries a terminal-auth gate. The emitter does not
  name methods.

**The checks:**

* **The emitter's pins** (`emit/dispatch_test.go`) are updated.
* **A root test, `TestTypedMethodDocsNameErrors`,** requires every exported method of the two side connections that takes a
  `context.Context` to name `ErrNotInitialized`, unless it initializes, and `*RequestError`.
  Annotated during step 2, 2026-10-19 (see "Deviation, 2026-10-19, during step 2: the extension methods"): the four
  extension methods also take one and never return `ErrNotInitialized`. Their docs gain a paragraph of their own; the test
  requires `*RequestError` in it and refuses `ErrNotInitialized`.
* **Each sentence's claim is mapped** in step 2's record to the existing test that exercises it. A claim with no test gets one in that
  step.

### A5: the concurrency contract

* **`Connection`, `AgentSideConnection` and `ClientSideConnection`** each gain: "Its methods are safe for concurrent use. Messages sent
  from several goroutines are written whole, one at a time, in the order they reach the connection's writer." The side connections'
  sentences come from the emitter.
* **`Agent` and `Client`,** the interfaces, each gain: "The connection calls a request's method on a goroutine of its own, so the
  methods for requests run concurrently. It calls the methods for notifications one at a time, in the order they arrive."
* **The check.** A new root test, `TestConcurrentUse`, runs under `-race` in the gates. It calls every exported method of the three
  types from several goroutines at once, over a pair connected by pipes: the sends, `Done`, `SetLogger`, `Negotiated`,
  `DroppedNotifications` and `Close`.

### A8: overview, quickstart and examples

* **`doc.go`** gains the package's model:
  * the two sides;
  * the constructors;
  * that reading starts in the constructor;
  * how a connection ends (`Done`, `Close`);
  * the error model (sentinels and the four error types);
  * a pointer to `Example_agent`, `Example_client` and the README.
* **Four per-symbol examples,** each with `// Output:`, in `package acp_test`:

  | Example | What it shows |
  | :--- | :--- |
  | `ExampleNewClientSideConnection` | a client and an agent over two `io.Pipe`s; `Initialize`; prints the agreed protocol version; closes both |
  | `ExampleStartToolCall` | a tool-call update built with `WithStartKind` and `WithStartStatus`, printed as JSON |
  | `ExampleRequestError` | `errors.AsType[*acp.RequestError]` on an error a handler returned, printing its code and message |
  | `ExampleConnection_Close` | a send after `Close` fails, and `errors.Is(err, acp.ErrConnectionClosed)` is true |

* **The README's quickstart.** "Get Started" gains a Go block between `<!-- quickstart: ExampleNewClientSideConnection -->` and
  `<!-- /quickstart -->`, with the body of `ExampleNewClientSideConnection`.
  * **The check.** A new root test, `TestReadmeQuickstartIsTheExample`, prints that function's body from its file with `go/printer`
    and requires the README block to equal it. So the snippet is compiled and run as the example, and cannot drift from it.
  * **Why not `mdsh`.** The tool is not installed, and adding it needs a dependency record.

### A10: option semantics

`ConnectionOption`'s doc gains:

* options apply in order, so a repeated option's last value wins;
* `n <= 0` leaves `WithMaxQueuedNotifications` and `WithMaxMessageSize` at their defaults, but sets `WithMaxConcurrentRequests` back
  to no limit;
* a nil option panics.

`ToolCallStartOpt` and `ToolCallUpdateOpt` gain "options apply in order".

### A11: wire-schema types

**The table.** The protocol table gains `DocNotes []DocNote`, each a definition name and doc lines. `Build` appends them to the
definition's doc, ~~and refuses a note on a definition the schema lacks, naming it~~. The notes:

Annotated during step 2, 2026-10-19 (see "Deviation, 2026-10-19, during step 2: a note on an absent definition"): a note on a
definition the schema lacks applies nothing, as every rule of the table does; the committed schema's check refuses one.

Annotated during step 2, 2026-10-19 (see "Deviation, 2026-10-19, during step 2: the envelope's variant types"): a note on a
union also reaches its inline variants' types, so the six envelope definitions' notes reach the ten types.

| Definitions | Note |
| :--- | :--- |
| the ten envelope types | "A message of the JSON-RPC envelope. Connection reads and writes the envelope itself and does not use this type." |
| `Error` | "Connection reports an error as a *RequestError, not as this type." |
| `ErrorCode` | "RequestError.Code is an int: compare it with int(ErrorCodeInternalError), for example." |
| `CancelRequestNotification` | "Connection sends $/cancel_request itself when a call's context ends." |

**The checks.** ~~`TestProtocolTableRefuses` gains a row for a note on an unknown definition.~~
`TestProtocolTableAppliesToCommittedSchema` covers the notes on the committed schema.

Annotated during step 2, 2026-10-19 (see "Deviation, 2026-10-19, during step 2: a note on an absent definition"):
`TestDocNotes` holds the refusals, and `checkCommittedDocNotes`, called from `TestProtocolTableAppliesToCommittedSchema`, refuses a
note whose definition the committed schema lacks.

### V4: `RELEASING.md`

A section, "Release notes and the API", says:

* `docs/reports/NNNN-REPORT-release-notes-vX-Y-Z.md` is the changelog, with its "Release text" the GitHub release's body;
* the API listing against the last tag is recorded in each release's notes;
* an incompatible change needs `/v2` ([0021-MADR-resolve-issues-4-and-5-before-v1-14-0.md](0021-MADR-resolve-issues-4-and-5-before-v1-14-0.md)).

## Implementation Steps

Each step ends with its gates and an execution record. The owner commits it; agents do not commit on `main`. Every new test is seen to
fail on a planted break in a scratch copy, never in the tree, before it is relied on.

1. **Records.** This PLAN and its row; 0012's annotation; the facts above re-asserted by a script.
2. **Generated docs:**
   * A7 in the emitter;
   * A6's paragraphs and A5's side-connection and interface sentences;
   * A11's `DocNotes` in the protocol table;
   * regeneration;
   * the emitter's and model's tests;
   * `TestExportedTypeDocsStartWithName` and `TestTypedMethodDocsNameErrors`;
   * the mapping of A6's claims to tests.
3. **Hand-written docs:**
   * A5 on `Connection`, and `TestConcurrentUse`;
   * A10;
   * `doc.go`;
   * the four examples;
   * the README quickstart and `TestReadmeQuickstartIsTheExample`;
   * `RELEASING.md`.
4. **Release notes.** `docs/reports/0024-REPORT-release-notes-v1-16-3.md` and its row.
5. **Verification and close.**
   * The gates over the tree.
   * `make push-guard`.
   * After CI's six jobs pass on the pushed commits, this PLAN and its row become `complete`.
   * The owner tags `v1.16.3` and publishes its release.

## Verification

Per code step:

* phase 1's gates on go1.27.2, and the go1.26.9 gate;
* **the comments-only check.** A script outside the tree parses each changed non-test `.go` file before and after the step. It
  compares the syntax trees with comments and positions dropped, and requires them equal. Phase 4 of 0019 used the same check for its
  documentation (I12).
  Annotated during step 2, 2026-10-19: it covers the SDK package's files. Step 2's changes to the generator are its code change.

Planted breaks, at least. Each must be seen to fail on the check named.

| For | Break | The check that must fail |
| :--- | :--- | :--- |
| A7 | the emitter writing the description first, without `<Name> is …` | `TestExportedTypeDocsStartWithName` |
| A6 | one method's error paragraph dropped | `TestTypedMethodDocsNameErrors` |
| A5 | `SetLogger` storing without its atomic | `TestConcurrentUse` under `-race` |
| A8 | one line of the README's quickstart changed | `TestReadmeQuickstartIsTheExample` |
| A8 | one example's printed line changed | that example under `go test` |
| A11 | ~~a note naming a definition the schema lacks~~ a note naming a definition the committed schema lacks (step 2) | ~~`TestProtocolTableRefuses`'s new row; `Build` refuses it~~ `checkCommittedDocNotes` |
| — | a code token changed in a regenerated file | the comments-only check |

Expected API listing difference against `v1.16.2`: none.

Acceptance:

* every step's checks pass and its record is written;
* the comments-only check holds for every non-test file each step changed;
* `TestExportedDeclarationsDocumented`, `TestExportedTypeDocsStartWithName` and `TestTypedMethodDocsNameErrors` pass;
* `make lint` reports 0 issues and `go fix -diff` prints nothing;
* CI's six jobs pass on the pushed commits;
* the API listing against `v1.16.2` shows no difference.

## Rollout and Rollback

Each step is one commit, reverted by reverting it. Step 2 changes the generator and regenerates, so reverting it reverts its comments
with it. Nothing compiled changes, so no consumer's build or behaviour changes. The release is the patch `v1.16.3`, which the owner
confirms at the tag.

## Execution record

### Step 1, records, 2026-10-19

The owner committed this PLAN as `fbd9afa`, pushed it, and approved it. This step:

* marks this PLAN and its `docs/README.md` row `in-progress`;
* annotates [0012-MADR-schema-fidelity-framework.md](0012-MADR-schema-fidelity-framework.md), after its table of schema keywords:
  a generated type's doc comment now opens with `// <Name> is <place>.` before the `description`.

A script outside the tree re-asserted "Facts this plan rests on" on `fbd9afa`: 24 facts, each snippet at its line, plus the counts
quoted. All hold, after two corrections:

* **A5:** `processNotifications` is at `connection.go:1471`, not `:1467`. 0023's and phase 1's changes to `connection.go` moved it.
  `connection.go:1279`, the request goroutine, holds.
* **A8:** the README's first Go block is at line 204, not 199. Phase 1 added the minimum Go version and the raw-value line above it.

Both rows are annotated. The counts hold: 378 exported types in `types_gen.go`, 8 starting with their name, and 54 non-compliant ones
with no method that names their place.

Gates: `make check-records`, `make markdownlint` and the identifier scan, each exit 0.

### Deviation, 2026-10-19, during step 2: a note on an absent definition

**Found.** A11's design has `Build` refuse a doc note on a definition the schema lacks. Every other rule of the protocol table applies
nothing to an absent subject, and `TestProtocolTableSkipsAbsentSubjects` requires it: a table outlives a schema that drops a
definition.

**Decision (owner, 2026-10-19).** Follow the table's convention. A note on an absent definition applies nothing. The committed schema's
check, `checkCommittedDocNotes` in `TestProtocolTableAppliesToCommittedSchema`, refuses a note whose definition it lacks, so a typo in
`ACP` still fails. `TestDocNotes` holds `Build`'s refusals: a note with no definition or no lines, and two notes on one definition.

"A11: wire-schema types" and the verification table are annotated.

**Files added to step 2's scope:** none.

### Deviation, 2026-10-19, during step 2: the extension methods

**Found.** `TestTypedMethodDocsNameErrors`, as A6 words it, covers every exported method of the side connections that takes a
`context.Context`. Four hand-written methods match: `CallExtension` and `NotifyExtension` of both sides (`extensions.go`). They send
before initialize, through `SendRequest` and `SendNotification`, which do not check what initialize agreed
([0017-MADR-remediate-acp-v1-conformance-violations.md](0017-MADR-remediate-acp-v1-conformance-violations.md), G1). They never
return `ErrNotInitialized`, so the test as written passes only if their docs say something false.

**Decision (owner, 2026-10-19).** Keep the test over all 44 methods.

* **The 40 typed methods** must name `ErrNotInitialized`, unless the method is `Initialize`.
* **The four extension methods,** whose params are `any`, must not name it.
* **Every one** must name `*RequestError`.
* **The extension methods' docs** gain a hand-written paragraph naming what they return:
  * an error for a method that does not start with `_`;
  * a `*ValidationError` for params of this package's types that break the schema;
  * a `*RequestError` for params that do not encode, or that encode as neither null nor an object or an array;
  * then the context, connection and write errors, and the peer's error answer, as the typed methods' docs name them.

The MADR is amended under its phase 2. The A6 design is annotated.

**Files added to step 2's scope:** `extensions.go`, comments only.

### Deviation, 2026-10-19, during step 2: the envelope's variant types

**Found.** A11's table puts the envelope note on "the ten envelope types". Six are schema definitions: `AgentRequest`,
`AgentNotification`, `AgentResponse`, `ClientRequest`, `ClientNotification` and `ClientResponse`. The other four, `AgentResult`,
`AgentError`, `ClientResult` and `ClientError`, are types built from inline variants of the two response unions. A note names a
definition, so it reached only the six.

**Decision (owner, 2026-10-19).** A note on a union also reaches the types of its inline variants. It does not reach their wrapper
fields' docs. All ten types carry the note. `TestDocNotes` covers it on a test union with an inline variant and a `$ref` one.
`checkCommittedDocNotes` requires exactly those four variant types to carry it.

"A11: wire-schema types" is annotated.

**Files added to step 2's scope:** none.

### Step 2, generated docs, 2026-10-19

What changed, on `cd7c254`:

* **A7, the emitter** (`emit/types.go`):
  * `typeDoc` opens each generated type's doc with `<Name> is <place>.`, then a blank line and the description;
  * a description that already starts with the name and a space is kept as it is;
  * it covers schema definitions, nested structs, union wrappers and inline variants;
  * a union's interface keeps its comment.
* **A6, the emitter** (`emit/dispatch.go`):
  * `outboundErrors` appends the errors paragraph to each typed method;
  * the model drives its variations: `Initializes`, and the terminal-auth gates of `m.Gated`;
  * the emitter names no method;
  * `listOf` joins clauses with semicolons, and `wrapDoc` wraps at 76 characters.
* **A5:** the side connections' docs and the `Agent` and `Client` interfaces' docs carry the concurrency sentences. The side
  connections' come from the emitter, the interfaces' from the protocol table.
* **A11:** `DocNotes` in `Protocol`, checked by `checkDocNotes` and applied by `applyDocNotes`, after `checkTypeRules`. The nine notes
  are in `ACP`; the six envelope definitions share `envelopeNote`.
  * A note opens a doc that has no description, with no blank line before it.
  * A note on a union reaches its inline variants (deviation above).
* **Regenerated:** `types_gen.go`, `agent_gen.go`, `client_gen.go`.
* **Hand-written:** `extensions.go`'s four paragraphs (deviation above).
* **Tests:**
  * `docs_test.go`: `TestExportedTypeDocsStartWithName` (over every exported type of the package, at least 378) and
    `TestTypedMethodDocsNameErrors` (40 typed methods and 4 extension methods);
  * `method_errors_test.go`: `TestMethodErrorsAsDocumented`, for the claims below that had no test through the side connections;
  * `emit/docs_test.go`: `TestTypeDoc`, `TestOutboundErrors`, `TestListOf`, `TestWrapDoc`;
  * `model/patch_test.go`: `TestDocNotes` and `checkCommittedDocNotes`;
  * two pins updated: `emit/dispatch_test.go` (the side connections' concurrency sentences) and `generated_output_test.go`
    (`TestUnionDocAboveWrapper`).
* **Lint:** the first gate run reported four issues in the new generator code. They are fixed:
  * `goconst`: the repeated envelope note became `envelopeNote`;
  * `gocyclo`: `newRules` reached 41, so the notes' checks moved to `checkDocNotes`;
  * `modernize`: `wrapDoc` ranges over `strings.FieldsSeq`.

**A6's claims, each mapped to a test that makes it.** "New" is `TestMethodErrorsAsDocumented`, through the side connections.

| Claim | Test |
| :--- | :--- |
| `ErrNotInitialized` until initialize completes | `TestNotInitialized` (`gates_test.go`): every typed method but `Initialize`, by reflection |
| a `*CapabilityError` for the method | `TestMethodGates`: each of the 15 method gates |
| a `*CapabilityError` for a property or variant of params | `TestValueGates`: each capability rule |
| `ErrTerminalAuthMethod` | `TestTerminalAuthMethod` |
| a `*ProtocolVersionError`, asked and answered | `TestProtocolVersion` (`negotiation_test.go`): `Requested: 2`; `Requested: 1, Answered: 2` |
| a `*ValidationError` for params | new, "params that break the schema"; `TestSendRefusesRelativePaths` through `Connection` |
| a `*RequestError` wrapping `context.Cause(ctx)` | new, before the send (a request and a notification) and while waiting (in a synctest bubble) |
| `ErrConnectionClosed` or `ErrPeerDisconnected` | new, "the connection's end"; `TestCloseEndsConnection`, `TestSendAfterPeerEndFails` through `Connection` |
| the write's error | new, "a write's error" |
| the peer's error answer as a `*RequestError` | new, "the peer's error answer" |
| a response that breaks the schema: a `*ValidationError` | `TestReceiveRefusesRelativePaths`, its response route through `ListSessions` |
| a response that breaks what initialize agreed: a `*CapabilityError` | `TestResultGates`, through `NewSession` |
| a response that does not decode: a `*RequestError` | new, "a response that does not decode" |
| extension: a method that does not start with `_` | `TestNotifyExtensionNames`; `TestClientSideConnection_ConnectionReachesThePeer` and the agent's, for `CallExtension` |
| extension: params that do not encode, or encode as neither null nor an object nor an array | new; `TestParamsNeverSentAsNull` for null, through `Connection` |
| extension: params of this package's types that break the schema | new |
| extension: the context, connection, write and answer errors | new, in each of those subtests |

"While waiting" runs in a synctest bubble. Run plainly, a cancel can land before the writer reports the write done. `sendMessageSent`
then returns the cause (`connection.go:1644`), and the plant on `waitForResponse` passed once. In the bubble it fails every time.

**The comments-only check.** A script outside the tree compares each changed non-test `.go` file of the SDK package with `HEAD`,
with comments and positions dropped. `agent_gen.go`, `client_gen.go`, `extensions.go` and `types_gen.go` are each the same tree;
exit 0. Against the tree before the variant deviation, regeneration changed only the four variant types' docs.

**Planted breaks.** Each ran in its own scratch copy of the working tree, never in the tree. All 19 failed the check named, with the
text named, after their expected texts were corrected:

| Break | The check that failed |
| :--- | :--- |
| A7: `typeDoc` keeps the description first, regenerated | `TestExportedTypeDocsStartWithName`: "'s doc starts" |
| A7: `typeDoc` keeps a doc that starts with a longer name | `TestTypeDoc`, "a longer name" |
| A6: `NewSession`'s errors paragraph dropped from `client_gen.go` | `TestTypedMethodDocsNameErrors`: "does not name *RequestError" |
| A6: an extension method's doc naming `ErrNotInitialized` | `TestTypedMethodDocsNameErrors`: "extension method NotifyExtension's doc names ErrNotInitialized" |
| A6: the emitter's terminal-auth clause dropped | `TestOutboundErrors`, "a terminal-auth request" |
| A6: a context's cause dropped before the send | `TestMethodErrorsAsDocumented`, all four calls |
| A6: a context's cause dropped in `waitForResponse` | `TestMethodErrorsAsDocumented`, "while waiting" |
| A6: params not validated | `TestMethodErrorsAsDocumented`: ", want a *ValidationError" |
| A6: a connection closed here reported as the peer's end | `TestMethodErrorsAsDocumented`: "wrapping acp: connection closed" |
| A6: the write's error replaced | `TestMethodErrorsAsDocumented`: "wrapping the disk is full" |
| A6: the peer's error answer replaced | `TestMethodErrorsAsDocumented`: "want the peer's *RequestError, -32042" |
| A6: a response that does not decode returned as the decoder's error | `TestMethodErrorsAsDocumented`, "does not decode" |
| A6: params of another shape sent | `TestMethodErrorsAsDocumented`: "CallExtension(string)" |
| A6: params that do not encode returned as the encoder's error | `TestMethodErrorsAsDocumented`: "json: unsupported type" |
| A11: a note on `ErrorCodes`, which the schema lacks | `TestProtocolTableAppliesToCommittedSchema`: "doc note ErrorCodes: no such type" |
| A11: two notes on one definition accepted | `TestDocNotes`: "two notes: Build = <nil>" |
| A11: a blank line before a note on a type with no description | `TestDocNotes`: `Update.Doc = ["" "Only."]` |
| A11: a union's note kept from its variants | `TestDocNotes` and `TestProtocolTableAppliesToCommittedSchema`: "AgentResult.Doc" |
| a code token changed in `types_gen.go` | the comments-only check: "DIFFERS:", exit 1 |

Two plants hung to the test timeout on their first run. A send that should have been refused was written to a peer that never answers.
The refusal subtests now bound such calls with a five-second context, and every plant fails in seconds.

**Gates** on the working tree, go1.27.2, each exit 0:

* `go run .` in `cmd/generate`: generated files unchanged by regeneration;
* `make pre-add-check`: "136 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck)";
* `scripts/go-precheck.sh` on the two new `_test.go` files, `method_errors_test.go` and `emit/docs_test.go`;
* `GOEXPERIMENT=nojsonv2 go test ./...`;
* `GOOS=windows go vet ./...` in both modules;
* the identifier scan of the changed files;
* the API listing against `v1.16.2`: 2429 lines at the tag and in the tree, 0 removed, 0 added;
* `go fix -diff ./...` in both modules: no output.

**The go1.26.9 gate**, with `GOTOOLCHAIN=go1.26.9` and scratch caches: `go version` reports go1.26.9; `go vet ./...` and
`go test -race -count=1 ./...` pass in both modules. The root's tests ran with `GOEXPERIMENT=goroutineleakprofile`, so the leak guard
ran.

`TestMethodErrorsAsDocumented` passed 20 runs of 20 under `-race`.
