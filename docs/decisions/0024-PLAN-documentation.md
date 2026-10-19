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
* **`Initialize`** does not wrap `ErrNotInitialized`. It also returns a `*ProtocolVersionError` for a version this SDK does not speak,
  asked or answered.
* **The client's `Authenticate`** names `ErrTerminalAuthMethod`.
* **The model drives the variations.** It knows which method initializes and which carries a terminal-auth gate. The emitter does not
  name methods.

**The checks:**

* **The emitter's pins** (`emit/dispatch_test.go`) are updated.
* **A root test, `TestTypedMethodDocsNameErrors`,** requires every exported method of the two side connections that takes a
  `context.Context` to name `ErrNotInitialized`, unless it initializes, and `*RequestError`.
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
definition's doc, and refuses a note on a definition the schema lacks, naming it. The notes:

| Definitions | Note |
| :--- | :--- |
| the ten envelope types | "A message of the JSON-RPC envelope. Connection reads and writes the envelope itself and does not use this type." |
| `Error` | "Connection reports an error as a *RequestError, not as this type." |
| `ErrorCode` | "RequestError.Code is an int: compare it with int(ErrorCodeInternalError), for example." |
| `CancelRequestNotification` | "Connection sends $/cancel_request itself when a call's context ends." |

**The checks.** `TestProtocolTableRefuses` gains a row for a note on an unknown definition. `TestProtocolTableAppliesToCommittedSchema`
covers the notes on the committed schema.

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

Planted breaks, at least. Each must be seen to fail on the check named.

| For | Break | The check that must fail |
| :--- | :--- | :--- |
| A7 | the emitter writing the description first, without `<Name> is …` | `TestExportedTypeDocsStartWithName` |
| A6 | one method's error paragraph dropped | `TestTypedMethodDocsNameErrors` |
| A5 | `SetLogger` storing without its atomic | `TestConcurrentUse` under `-race` |
| A8 | one line of the README's quickstart changed | `TestReadmeQuickstartIsTheExample` |
| A8 | one example's printed line changed | that example under `go test` |
| A11 | a note naming a definition the schema lacks | `TestProtocolTableRefuses`'s new row; `Build` refuses it |
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
