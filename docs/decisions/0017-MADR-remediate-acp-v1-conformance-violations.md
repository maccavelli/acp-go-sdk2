---
status: accepted
date: 2026-10-12
decision-makers: Repository owner
---

# The SDK answers cancelled turns and permission requests as ACP requires, answers malformed input as JSON-RPC requires, and enforces the negotiated version and capabilities

## Context and Problem Statement

[0016-REPORT-acp-v1-conformance-audit.md](../reports/0016-REPORT-acp-v1-conformance-audit.md), observed 2026-10-12, audits the SDK against
the published ACP v1 specification and the `schema-v1.25.0` release, byte-identical to `schema/`. It finds seven violations (V1 to V7) and
records six notes. The owner asked that the findings be assessed against the code, that the SDK adhere to the current standard with
compliance enforced, and, as asked for 0015, that consumers keep room to adjust, extend and observe. This record checks each finding on
`a438f99` (0015 step 6) on 2026-10-12, adds what the check found beyond the report, and decides the remedy. The ACP sources are the pages
at <https://agentclientprotocol.com> named under "More Information", read on 2026-10-12, and the committed schema.

Four behaviours were also run, not only read: a probe test file added to a scratch copy of the working tree, run with `go test -v` and
removed with the copy. Their results are quoted below as "probe".

### The findings, checked

* **V1 (High) holds: a cancelled prompt turn is answered `-32800`.** `toReqErr` (`errors.go:127`) turns any error that `errors.Is`
  `context.Canceled` into `NewRequestCancelled`. The generated `session/prompt` case (`agent_gen.go:415`, from `promptCase` in
  `cmd/generate/internal/emit/dispatch.go`) runs `Prompt` on a context that `session/cancel` cancels (`agent_gen.go:309`), and returns
  `toReqErr(err)`. Probe: a handler that returns `ctx.Err()` after `session/cancel` is answered
  `{"code":-32800,"message":"Request cancelled","data":{"error":"context canceled"}}` with no result. The standard:
  * prompt turn, "Cancellation": "After all ongoing operations have been successfully aborted and pending updates have been sent, the Agent
    MUST respond to the original `session/prompt` request with the `cancelled` stop reason", and "Agents MUST catch these errors and
    return the semantically meaningful `cancelled` stop reason, so that Clients can reliably confirm the cancellation";
  * schema, `StopReason` `cancelled`: "This stop reason MUST be returned when the client sends a `session/cancel` notification, even if the
    cancellation causes exceptions in underlying operations";
  * cancellation (published under the v1 draft path): a receiver of `$/cancel_request` "MUST send one of these responses for the original
    request: A valid response with appropriate data (such as partial results or cancellation marker); An error response with code
    `-32800`", and on internal cancellation "SHOULD ... Send the same `-32800` (Cancelled) error response". So `-32800` is right for every
    request except a prompt turn, where `{"stopReason":"cancelled"}` satisfies both mechanisms.
* **Beyond the report: superseding a prompt loses the successor's cancel.** A second `session/prompt` for a session cancels the first
  (`agent_gen.go:427`) and stores its own cancel function; when the first handler returns, the case deletes the session's entry
  unconditionally (`agent_gen.go:434`), which is now the second's. Probe: after the first turn returned, `session/cancel` did not cancel the
  second turn's context (`false` after one second); the superseded first turn was answered `-32800`. The report recorded superseding as a
  safety decision; this defect is in its bookkeeping.
* **V2 (Medium) holds, and is wider than the report.** `Connection.receive` (`connection.go:591`) drops and logs a line that fails to
  decode ("failed to parse incoming message") and one with neither `id` nor `method`; `NewParseError` and `NewInvalidRequest` have no
  caller; `jsonrpc` is never read. Probe, each line written to an agent connection:
  * `not json`: no reply. JSON-RPC 2.0 requires `-32700` with `"id": null`.
  * `{"jsonrpc":"2.0","method":5,"id":7}`: no reply (the value fails to decode into `anyMessage`). JSON-RPC requires `-32600`.
  * `{"jsonrpc":"1.0","id":8,"method":"session/new",...}`: handled and answered with a result. JSON-RPC: `jsonrpc` "MUST be exactly
    "2.0"".
  * `{"jsonrpc":"2.0","id":null,"method":"session/new",...}`: no reply, though the handler ran and created a session. The schema's
    `RequestId` admits `null` ("MUST contain a String, Number, or NULL value if included. If it is not included it is assumed to be a
    notification"); `anyMessage.ID` is a `*json.RawMessage`, which `encoding/json` sets to `nil` for `null`, so the request is treated as a
    notification.
  * `{"jsonrpc":"2.0"}` and `[]`: no reply. JSON-RPC requires a single `-32600` with `"id": null` for an invalid request and for an
    empty array.

  A line longer than the scanner's 10 MiB limit (`maxBufSize`, `connection.go:594`) ends `bufio.Scanner`, and `shutdownReceive` closes
  the connection with no reply. JSON-RPC also says "The Server MUST NOT reply to a Notification" and defines no reply to a response, which
  any remedy must keep: a peer's own error response to a garbled line carries `"id": null` and must not be answered, or two peers can
  answer each other's errors without end.
* **Beyond the report: a malformed response completes the call as a success.** JSON-RPC: "Either the result member or error member MUST
  be included, but both members MUST NOT be included." Probe: a peer that answers `{"jsonrpc":"2.0","id":1}` to `session/new` makes
  `ClientSideConnection.NewSession` return a zero `NewSessionResponse` (empty `sessionId`) and a `nil` error; `SendRequest`
  (`connection.go:862`) skips decoding and `checkInbound` when `result` is empty, so 0015's response validation does not run.
* **V3 (Medium) holds.** Neither side connection keeps what `initialize` negotiated. The protocol table (`cmd/generate/internal/model/acp.go`)
  records 14 method gates, `session/load` on `loadSession` and 13 under the comment "Capability gates the schema describes, recorded, not
  acted on"; `checkCapabilities` (`protocol.go`) resolves each path against the schema and nothing else reads them.
  `ClientSideConnection.LoadSession` (`client_gen.go:278`) and `AgentSideConnection.ReadTextFile` (`agent_gen.go:493`) send whatever the
  peer advertised. The standard's gates, each a MUST or MUST NOT:
  * initialization: "Clients and Agents MUST treat all capabilities omitted in the initialize request as UNSUPPORTED", and "Before a
    Session can be created, Clients MUST initialize the connection";
  * methods: `session/load` ("Clients MUST NOT attempt to call `session/load`"), `session/resume` and `session/close` (the same sentence
    form), `session/list` ("If `sessionCapabilities.list` is not present, the Agent does not support listing sessions and Clients MUST NOT
    attempt to call `session/list`"), `session/delete` ("Clients MUST NOT attempt to call `session/delete`"), `logout` ("If
    `agentCapabilities.auth.logout` is omitted or `null`, the Agent does not support `logout` and Clients MUST NOT call it"), `fs/*` (an
    Agent "MUST NOT attempt to call the corresponding filesystem method") and `terminal/*` ("MUST NOT attempt to call any terminal
    methods");
  * properties and variants: "Clients MUST only send `additionalDirectories` when the Agent advertises
    `sessionCapabilities.additionalDirectories`"; "Before using HTTP or SSE transports, Clients MUST verify the Agent's capabilities";
    "Clients MUST restrict types of content according to the Prompt Capabilities established during initialization"; elicitation,
    "Agents MUST NOT request a mode the Client has not advertised"; config options, "Agents MUST NOT include `type: "boolean"` options in
    `configOptions` payloads unless the Client advertised support"; schema, `Notice` and the compaction updates, "Agents MUST only send
    this update when the Client advertised" `ClientSessionCapabilities::notices` / `::compaction`; schema, `AuthMethodTerminal`, "Agents
    MUST advertise this method only when the client enabled its terminal authentication capability"; authentication, "the Client MUST NOT
    send an `authenticate` request for a terminal method".
* **V4 is remediated.** The report predates 0015 step 5 (`b83b84d`), which wires `validateOutbound` into `prepareRequest` and
  `prepareNotification`, and step 6 (`a438f99`), which validates responses. Nothing is decided here for it.
* **V5 (Medium) holds and is in no record.** Initialization: "The initialize request MUST include the latest protocol version the Client
  supports"; "If the Agent supports the requested version, it MUST respond with the same version. Otherwise, the Agent MUST respond with
  the latest version it supports"; "If the Client does not support the version specified by the Agent in the initialize response, the
  Client SHOULD close the connection and inform the user about it." `ProtocolVersion` is a `uint16` (`types_gen.go`);
  `ProtocolVersionNumber = 1` is read only by tests and examples; the `initialize` case (`agent_gen.go:115`) and
  `ClientSideConnection.Initialize` (`client_gen.go:202`) compare nothing. A handler that leaves the field zero answers
  `"protocolVersion":0`. This SDK speaks version 1 only, so a conforming agent built on it answers 1 to every request: the same version
  when 1 is asked, its latest otherwise.
* **V6 (Medium) holds.** Prompt turn: "The Client MUST respond to all pending `session/request_permission` requests with the `cancelled`
  outcome", and "SHOULD preemptively mark all non-finished tool calls pertaining to the current turn as `cancelled`"; the schema repeats
  the first on `RequestPermissionOutcome`. `ClientSideConnection.Cancel` (`client_gen.go:254`) and the automatic cancel in `Prompt`
  (`client_gen.go:288`) send `session/cancel` and nothing else; the `session/request_permission` case (`client_gen.go:82`) runs the
  handler on its request context, which only `$/cancel_request` or the connection's end cancels. `NewRequestPermissionOutcomeCancelled()`
  exists. Marking tool calls is UI state the SDK does not hold.
* **V7 (Minor) holds.** `ErrorCodeResourceNotFound ErrorCode = -32002` is generated (`types_gen.go`; schema: "A given resource, such as a
  file, was not found"); `errors.go` has no constructor for it, as it has for the other codes.

### The report's notes, checked

* **The 10 MiB line cap** is the overlong line above; it is fixed and not configurable.
* **The outbound `$/cancel_request` queue** (`maxPendingCancelRequests = 1024`, `connection.go:362`) logs a dropped cancel at Debug. The
  standard lets a receiver ignore cancellation; no requirement is unmet.
* **JSON-RPC batches** are v2 scope; the v1 book states no batching. Today an array is dropped silently.
* **Duplicate keys** resolve last-wins; the standard is silent.
* **Superseding a second prompt** is the safety behaviour the report describes, with the defect found above.
* **Initialization order** is not in the report: the SDK sends and handles any method before `initialize`.

### The flexibility the codebase offers today

* Since 0015: `WithInboundRewrite` and `WithInboundValidation`, run by `Connection.checkInbound` on every received request, notification
  and response, and the typed `*ValidationError`. A hook receives the SDK's error and may accept it, refuse a valid message, or return a
  `*RequestError` whose code is answered as it is (`inboundError`).
* The escape hatches: `Connection()` on both side connections, `SendRequest[T]`, `SendRequestNoResult`, `SendNotification`,
  `NewConnection` with any `MethodHandler`, and the extension methods.
* Missing, each found in the code: no accessor for what the peer advertised or what was agreed; no way for a prompt handler to tell
  `session/cancel` from `$/cancel_request` from superseding (`context.Cause` returns `context.Canceled` for all three); no limit option on
  message size.

### Where the side connections live

Added 2026-10-12, when the owner asked, reading the first PLAN, why protocol code was going into hand-written files.

* `docs/architecture.md` splits the package: generated from `schema/` are `constants_gen.go`, `types_gen.go`, `agent_gen.go`,
  `client_gen.go`, `helpers_gen.go` and `schema_gen_test.go`; hand-written are `agent.go` and `client.go` ("the two connection
  types"), `connection.go`, `errors.go`, `extensions.go`, `helpers.go` and `doc.go`.
* `agent.go` (50 lines) and `client.go` (44 lines) declare only the `AgentSideConnection` and `ClientSideConnection` structs, their
  constructors, and `DroppedNotifications`, `Done`, `SetLogger` and `Connection`, each forwarding to the `*Connection`. Every method
  that speaks the protocol on those types is generated into `agent_gen.go` and `client_gen.go`. `AgentSideConnection` already holds
  protocol state by hand: `mu` and `sessionCancels`, the prompt turns the protocol table's `PromptTurn` rule drives.
* The generated files are type-checked, and each engine case compiled, with the hand-written runtime files
  (`enginetest.RuntimeFiles`: `agent.go`, `client.go`, `connection.go`, `doc.go`, `errors.go`, `extensions.go`), which therefore name
  no ACP type and only types every schema generates.
* So a hand-written side connection cannot hold a generated type. Recording `initialize` there needs the record typed `any` and
  asserted back by generated code, and enforcing gates needs a generated function installed into the connection at construction;
  the permission registry of Q1 would be hand-written protocol state as `sessionCancels` is. The first PLAN did exactly that.
* `extensions.go` declares `handleWithExtensions`, `CallExtension` and `NotifyExtension` on both side types and calls the generated
  `handle`; it is transport for `_`-prefixed methods and stays valid on generated types of the same package.
* The engine's API listing (`enginetest.API`) lists the declarations made in the generated files, and `TestEngineCases` compares it
  with each case's `want.api`; `-update` rewrites them. 55 of the 88 engine cases carry a `want.api`.

**Question:** how should the SDK meet each requirement V1, V2, V3, V5, V6 and V7 leave unmet, and the two defects found with them, so that
compliance is enforced where the standard binds the SDK, while consumers keep the room the standard leaves them; where the protocol
state this needs lives; and in what order with the open steps of 0014 and 0015?

## Decision Drivers

* **Conform to the current standard, enforced.** The owner's direction: the published v1 standard is the baseline and compliance is not
  optional. What the SDK sends, and how it answers, must conform whatever the application's code does.
* **The easy path is the conforming path.** The idiomatic Go handler returns `ctx.Err()` when cancelled; that must produce the conforming
  answer, not the violation (V1 is High for this reason).
* **The SDK owns what only it can see.** Pending inbound requests, the bytes of a line, the negotiated version and capabilities are
  state the connection holds; an application can only reconstruct them by duplicating the connection's bookkeeping.
* **Never loop, never answer what must not be answered.** No reply to a notification or a response, including an error response with
  `"id": null`.
* **Room for consumers where the standard leaves it.** The standard binds senders; a receiver's response to a violation is open. As in
  0015, receiving refuses by default and a hook may decide otherwise; observing and inspecting need accessors and typed errors.
* **Consistent with 0015.** Sending is enforced with no opt-out; receiving goes through `checkInbound` and its two hooks; protocol rules sit
  in the protocol table with their citation and are checked against the schema
  ([0011-MADR-harden-code-generator-engine.md](0011-MADR-harden-code-generator-engine.md), F11 and F12).
* **Idiomatic Go.** `context.WithCancelCause` and `context.Cause`, sentinel errors with `errors.Is`, typed errors with `errors.As`,
  functional options, `bufio.Reader`.
* **Say what changed.** Each enforcement refuses something earlier versions sent or accepted; release notes list each as breaking.
* **Protocol code is generated.** The generator exists so that what the protocol defines is derived from the schema and the protocol
  table, not written by hand; hand-written code is the transport, which no schema change touches. Protocol state belongs on the
  generated side, fully typed, with no `any` and no installed hooks to bridge the two.

## Considered Options

Answering a cancelled prompt turn (V1, the supersede defect):

* P1. A prompt turn whose context was cancelled, by `session/cancel`, by `$/cancel_request` on that request, or by a later prompt for the
  session, is answered `{"stopReason":"cancelled"}` whatever the handler returned; the cancel carries an exported cause; bookkeeping is per
  turn.
* P2. As P1, but only a handler error is converted; a result the handler returns after the cancel is sent as it is.
* P3. Narrow `toReqErr` alone, so `context.Canceled` becomes `-32603`.
* P4. Document the pattern and test it, no code change (the report's alternative).

Answering pending permission requests (V6):

* Q1. When the client sends `session/cancel`, through `Cancel` or `Prompt`'s automatic cancel, the SDK answers each pending
  `session/request_permission` for that session `{"outcome":{"outcome":"cancelled"}}` at once, and cancels the handler's context with the
  exported cause; the handler's later return is discarded.
* Q2. Cancel the handler's context with the cause and convert whatever it returns to `cancelled`, answering when it returns.
* Q3. A helper and documentation; the application keeps the bookkeeping.

Answering malformed input (V2, the response defect, the line cap):

* J1. Classify every line as JSON-RPC 2.0 requires: `-32700` with `"id": null` for invalid JSON; `-32600` for an invalid request, with
  its id when the id is valid and `null` otherwise, a single one for any array; a request with `"id": null` handled and answered with
  `null`; never a reply to a notification or a response; a response with neither or both of `result` and `error` fails the pending call;
  an overlong line skipped to its newline and answered `-32600` with `"id": null`, under a limit `WithMaxMessageSize` sets.
* J2. J1 without the line-cap change: an overlong line still closes the connection.
* J3. Keep logging; document.

Capabilities and initialization order (V3):

* G1. Each side connection records the `initialize` exchange; the protocol table states every gate, at method, property and variant level;
  the typed methods refuse to send what the record does not allow, with no opt-out; received messages are refused by default through
  `checkInbound`, which the hooks can override; accessors expose the record.
* G2. G1 when sending only; received messages are not checked.
* G3. Record and expose only; applications enforce.
* G4. G1 with hand-written checks in place of protocol-table rules.

Protocol version (V5):

* N1. The agent side fills a zero `protocolVersion` with `ProtocolVersionNumber` and refuses any other value than 1; the client side fills
  a zero request version the same way and refuses sending another, and `Initialize` returns a typed error, recording nothing, when the
  answer is not 1.
* N2. The SDK writes 1 into every initialize request and response, whatever the application set.
* N3. Document.

Resource-not-found helper (V7):

* K1. `NewResourceNotFound(uri string)`, data `{"uri": uri}`.
* K2. Nothing; the `Code` field suffices.

Order with the open steps of 0014 and 0015:

* O1. After `v0.16.0`: 0015 steps 7 and 8 and 0014 steps 6 to 8 finish and `v0.16.0` is tagged with its advisory; this record's PLAN
  then runs and is released as `v0.17.0`.
* O2. Before 0014 steps 6 to 8, so `v0.16.0` carries all three records.

Where the side connections live (added 2026-10-12, at the owner's question):

* S1. Generated: the generator emits `AgentSideConnection` and `ClientSideConnection`, their constructors and four forwarding methods
  into `agent_gen.go` and `client_gen.go`, with the protocol state the table's rules need as typed fields; `agent.go` and `client.go`
  are removed; the hand-written runtime is `connection.go`, `errors.go`, `extensions.go` and `doc.go`.
* S2. Hand-written, as today: the record held as `any` on `Connection`, a generated gate installed at construction, and the
  permission registry hand-written beside `sessionCancels`.

## Decision Outcome

Chosen options: "P1", "Q1", "J1", "G1", "N1", "K1", "O1" and "S1", accepted by the owner on 2026-10-12 as recommended, with P1 as
amended the same day and S1 added the same day (see "Decided with the acceptance" below).

* **P1**, because it makes the idiomatic handler conforming and covers every way a turn ends cancelled. The `StopReason` text binds the
  answer to the client's cancel, not to the handler's error, so P2 leaves a handler that returns `end_turn` after the cancel
  non-conforming; P3 turns a cancel into an internal error, still not `cancelled`; P4 leaves the default steering into the violation.
  `{"stopReason":"cancelled"}` is also a "valid response ... (such as ... cancellation marker)" for `$/cancel_request`, so one rule
  satisfies both mechanisms. Keying the bookkeeping by turn fixes the supersede defect where the defect lives. The override ends
  where the turn does: a cancel that arrives after the handler has returned finds no turn, so the handler's answer stands, since
  the specification lets a client cancel only "an ongoing prompt turn"; overriding there would answer `cancelled` for a turn that
  finished.
* **Q1**, because the MUST is on the client and only the connection knows which permission requests are pending. Answering at once does
  not depend on the handler noticing its context: a handler blocked on a dialog would hold Q2's answer, and the agent's turn with it. Q3
  is the status quo with a helper.
* **J1**, because each case is a JSON-RPC MUST, and a peer that sent garbage learns why instead of waiting. J2 keeps a silent close for a
  message the SDK chose to cap; making the cap an option costs one field. J3 is the violation.
* **G1**, because sending is where the standard binds the SDK, and 0015 set the receiving pattern. A gate the protocol table states is
  checked against the schema and cited, as the 14 method gates already are; G4 would put protocol knowledge in hand-written code, which
  0011 removed. G2 leaves a peer that ignores the agent's advertisement reaching a handler the agent never offered, which the interface
  assertion catches only for unimplemented methods; G3 leaves every application to rebuild the record.
* **N1**, because a zero value should mean "the SDK's version", as Go zero values do elsewhere in the SDK, while a deliberate wrong value
  is a bug to report, not to overwrite silently (N2). Closing the connection and informing the user are the application's; the typed error
  carries what it needs.
* **K1**, because every other code in the schema's `ErrorCode` has a constructor, and the schema's description names the resource.
* **O1**, because `v0.16.0` carries the fix and advisory for issue #2, a security regression; G1 adds work that would delay it, and its
  breaking changes are better announced by themselves. O2 saves one round of README and notes and delays the fix.
* **S1**, because the side connections hold protocol state (prompt turns today; the permission registry and the negotiation record
  with this record), and that state follows the protocol table's rules, so the generator should emit it beside the methods that use
  it. Each piece is then a typed field, emitted only when its rule applies, and generated code passes its own gate where it is needed.
  S2 keeps a boundary that makes protocol state hand-written and bridges it with `any` and an installed function, which is the
  workaround the owner questioned.

Concretely:

1. **Prompt turns (P1).**
   * `errors.go` exports `ErrPromptCancelled`, the cause given to a prompt turn's context when it is cancelled by `session/cancel`; a turn
     superseded by a later prompt is cancelled with an error that wraps it (`errors.Is(cause, ErrPromptCancelled)`). `$/cancel_request`
     keeps `context.Canceled` as its cause, as today.
   * The generated `session/prompt` case derives the turn's context with `context.WithCancelCause` and stores the turn in
     `sessionCancels` as a pointer to a per-turn value holding its cancel function (function values are not comparable in Go, a
     pointer is), and removes the entry only if it is still that turn's.
   * The boundary is the handler's return. As soon as it returns, the case takes `a.mu`, the lock `session/cancel` and a superseding
     prompt take, removes the turn's entry and reads `context.Cause` of the turn's context once, then releases the lock; the answer
     follows from that one reading. When the turn's context was cancelled by then, for any reason but the connection's end, the answer
     is `PromptResponse{StopReason: StopReasonCancelled}`, keeping the rest of the handler's result when it returned one. Otherwise
     the handler's result or error is answered as today, so a `session/cancel` that arrives after the handler returned finds no turn
     and changes nothing. `$/cancel_request` cancels the request's context without that lock, so one that arrives in the instant
     between the return and the reading may still turn the answer into `cancelled`; the cancellation page accepts either a valid
     response or `-32800` for it, so both outcomes conform.
   * `toReqErr` keeps mapping `context.Canceled` to `-32800` for every other method, as the cancellation page asks.
   * The protocol table carries this behaviour already (`PromptTurn`, `CancelsPromptTurn`); `promptCase` in
     `cmd/generate/internal/emit/dispatch.go` emits it.
   * (Amended 2026-10-12, "More Information".) A prompt turn and a `session/cancel` are ordered by receipt, not by which goroutine
     runs first: a cancel received after the prompt applies to its turn even when the turn is stored later, and one received before
     it does not.
2. **Permission requests on cancel (Q1).** The generated `session/request_permission` case registers each pending request by session.
   When the client connection sends `session/cancel` for a session, through `Cancel` or `Prompt`, it cancels each registered handler
   context with `ErrPromptCancelled` and each pending request is answered `RequestPermissionResponse{Outcome:
   NewRequestPermissionOutcomeCancelled()}` without waiting for the handler; a handler's later return is discarded and logged at Debug.
   A request that arrives after the cancel is handled normally. The README says that marking unfinished tool calls `cancelled`, a SHOULD,
   stays the application's.
3. **Malformed input (J1).**
   * `receive` reads with a `bufio.Reader` up to the limit, default 10 MiB, set by `WithMaxMessageSize(n int)`; an overlong line is read
     to its newline, discarded, logged, and answered `NewInvalidRequest` with `"id": null` and `data` naming the limit.
   * A line that `json.Valid` rejects is answered `NewParseError` with `"id": null`.
   * Valid JSON is decoded into an envelope of raw members. Not an object (an array included, batches being outside v1): `-32600`,
     `"id": null`. An object with `result` or `error` is a response: matched to a pending call, which fails with an internal error when it
     has neither or both or its `jsonrpc` is not `"2.0"`; never answered; unmatched ones are logged. Otherwise a request (an `id` member,
     `null` included) or a notification (no `id`): `jsonrpc` other than `"2.0"`, `method` absent or not a string, `id` not a string,
     number or `null`, or `params` present and not an object or array, makes it an invalid Request object, answered `-32600` with its
     id when the id is valid and `null` otherwise, an absent id included; only a valid notification goes unanswered. (Amended
     2026-10-12, writing the PLAN: the first text dropped an invalid object without `id` as a notification; JSON-RPC's section 7
     answers `{"jsonrpc": "2.0", "method": 1, "params": "bar"}` with `-32600` and `"id": null`, since a notification is a
     Request object without `id`, and an invalid object is not one.)
   * `anyMessage` distinguishes an absent `id` from `"id": null`, so a request with a `null` id is answered with `"id": null`.
4. **Negotiation and gates (G1).**
   * A `Negotiation` type holds the `InitializeRequest` and `InitializeResponse` of a completed `initialize`. `ClientSideConnection`
     records it when `Initialize` returns without error; `AgentSideConnection` records it when its `initialize` answer is sent.
     `Negotiated() (Negotiation, bool)` on both returns it; a later successful `initialize` replaces it. `Negotiation.Allows(method
     string) error` reports a method gate for an application that shows or hides features.
   * The protocol table gains capability rules beside the 14 method gates, each with its source sentence: the property and variant gates
     listed under "The findings, checked", namely `additionalDirectories` when not empty on `session/new`, `session/load`,
     `session/resume` and `session/fork`; `McpServer` `http`, `sse` and the unstable `acp` variants; `ContentBlock` `image`, `audio` and
     `resource` in `session/prompt`; `elicitation/create` by mode, and `elicitation/complete`, which the schema describes as sent "when a
     URL-based elicitation is complete", on `elicitation.url`; the `notice`, `compaction_update` and `compaction_summary_chunk` updates;
     `SessionConfigOption` `boolean` anywhere it is sent; `AuthMethod` `terminal` in `InitializeResponse.authMethods`; and an
     `authenticate` whose `methodId` names a `terminal` method. `Build` checks each against the schema, as it does the others. The
     comment "recorded, not acted on" goes.
   * Sending: until a negotiation is recorded, the typed methods other than `initialize` return `ErrNotInitialized`; after it, a method or
     value the record does not allow returns a `*CapabilityError` (`Method`, `Capability`, and the `Field` or `Variant` when the gate is
     below the method) without writing; an `authenticate` naming a terminal method returns an error wrapping `ErrTerminalAuthMethod`
     instead (amended 2026-10-12, "More Information"). A handler's result that breaks a gate is not sent (amended 2026-10-12, "More Information": a
     result is checked against the property and variant gates only, the method having been gated when the request was): the peer gets `-32603` naming it, as 0015 does
     for a non-conforming result. There is no option to skip this. `Connection`, `SendRequest[T]` and the extension methods hold no
     negotiation and stay ungated; their documentation says so.
   * Receiving: `checkInbound` runs the same checks against the record, after the SDK's validation and before the validation hook, which
     receives the error and may accept it. (Amended 2026-10-12, "More Information".) When the message fails validation, the hook
     receives that error first, and only if it accepts it the record's error, if any; each is accepted on its own. A refused request is answered `-32601` for a method gate or before initialization, and `-32602`
     for a property or variant gate; a refused notification is dropped and logged; a refused response is returned to the caller as an
     error.
   * Unstable features whose gates the table does not state today (`nes`, `plan`, `providers`, `subagents`, `positionEncoding`) are not
     gated by this record.
5. **Version (N1).** The agent side's `initialize` answer is `ProtocolVersionNumber` when the handler leaves it zero, and is refused
   (`-32603`) when the handler sets another value. `ClientSideConnection.Initialize` sends `ProtocolVersionNumber` when the request leaves
   it zero and refuses to send another value; an answer other than `ProtocolVersionNumber` returns a `*ProtocolVersionError` (`Requested`,
   `Answered`) and records no negotiation, so every gated method keeps refusing. The README says the application closes the connection
   and informs the user.
6. **Helper (K1).** `NewResourceNotFound(uri string) *RequestError`, code `-32002`, message "Resource not found", data `{"uri": uri}`.
7. **Documentation.** README: cancellation of a prompt turn and of permission requests, with `ErrPromptCancelled`; the JSON-RPC error
   answers and `WithMaxMessageSize`; negotiation, the gates, `Negotiated`, `ErrNotInitialized`, `*CapabilityError` and how a hook accepts
   one; the version rule. `docs/architecture.md`: the receive classification and the negotiation record. The examples drop any check
   the SDK now makes.
8. **Release.** The `v0.17.0` notes list each enforcement as a breaking change, in its own release-notes report.
9. **Side connections (S1).** First of the changes, before any protocol state is added:
   * the generator emits `AgentSideConnection` and `ClientSideConnection` into `agent_gen.go` and `client_gen.go`: the struct with
     `conn` and the `agent` or `client` it dispatches to, `NewAgentSideConnection` and `NewClientSideConnection`, and
     `DroppedNotifications`, `Done`, `SetLogger` and `Connection`, with today's signatures and documentation, so the exported API does
     not change;
   * each rule adds its state as typed fields of the side it serves, emitted only when the rule applies to the schema: the prompt
     turns (`mu`, `sessionCancels`, P1), the pending permission requests (Q1), the negotiation record as an
     `atomic.Pointer[Negotiation]` (G1, N1);
   * the generated dispatch and wrappers pass the side's generated gate to the runtime calls that need it (`checkInbound`,
     `checkResult`, the typed send), as an argument; `Connection` stores no gate and no record;
   * `agent.go` and `client.go` are removed; `enginetest.RuntimeFiles` becomes `connection.go`, `doc.go`, `errors.go` and
     `extensions.go`; `docs/architecture.md` says so. Every engine case's `want.api` gains the two side types' declarations.

### Decided with the acceptance, 2026-10-12

Asked, the owner chose:

* **P1's boundary.** A cancel that arrives after the prompt handler has returned changes nothing; the override applies only to a
  turn whose context was cancelled before the handler returned. Not chosen: overriding every answer the cancel races.
* **The options.** P1, Q1, J1, G1, N1, K1 and O1, as recommended.
* **S1**, added 2026-10-12: asked, reading the first PLAN, why generated code was bridged to hand-written side connections with `any`
  and an installed gate, the owner chose to generate the side connections. Not chosen: S2; a separate record run first.

### Consequences

* Good, because a cancelled prompt turn and every pending permission request are answered as the standard requires whatever the handler
  does, and the idiomatic handler that returns `ctx.Err()` is conforming.
* Good, because every malformed line is answered as JSON-RPC requires, or deliberately not answered when it is a notification or a
  response, so a peer learns of a fault instead of waiting.
* Good, because the SDK sends no method, property or variant the peer has not advertised, nothing before `initialize`, and no version but
  1; and a received message that breaks a gate is refused unless the application chose otherwise.
* Good, because an application can read what was negotiated, ask whether a method is allowed, tell a client's cancel from a protocol
  cancel through `context.Cause`, branch on typed errors, and size the message limit.
* Bad, because applications and tests that skip `initialize` now get `ErrNotInitialized`; in this repository's tests, 33 calls to
  typed session, prompt, file and permission methods sit in six files, of which three call `Initialize`, so most must initialize first.
* Bad, because a client that sends a gated method or content its agent did not advertise now gets an error, and an agent whose result
  offers a boolean config option or a terminal auth method to a client that did not advertise it now answers `-32603`; the release notes
  and README say how to read `Negotiated` first.
* Bad, because a peer that ignores capabilities is refused by default; an application that must interoperate with one installs a
  validation hook, as with 0015.
* Bad, because a handler that returns a result after its turn's context was cancelled is answered `cancelled`, overriding what it
  returned; the README says so.
* Neutral, because a cancel that arrives after the handler returned changes nothing, so a client that cancels as a turn ends can
  receive `end_turn`, as any client of an asynchronous protocol can; the specification's "Clients MAY cancel an ongoing prompt turn"
  covers only a turn still running.
* Bad, because the receive path, the prompt case and the permission case grow state and branches, each a source of races the tests must
  cover.
* Neutral, because `Connection`, `SendRequest[T]` and the extension methods stay the ungated transport, the same escape hatch they are
  today.
* Good, because every piece of protocol state is generated and typed, emitted only where its rule applies, and the hand-written
  runtime is transport alone.
* Bad, because the generator gains the side-connection emission, and every engine case's `want.api` changes once, by the same
  declarations; the list of runtime files 0011 recorded no longer describes the tree, and `docs/architecture.md` is updated.

### Confirmation

* Prompt turns: a handler returning `ctx.Err()`, a wrapped error, or `end_turn` after `session/cancel` is answered
  `{"stopReason":"cancelled"}`; after `$/cancel_request` on the prompt, likewise; `context.Cause` is `ErrPromptCancelled` for
  `session/cancel`; a superseded turn is answered `cancelled`, and `session/cancel` after it returns cancels its successor (the probe's
  `false` becomes `true`); `-32800` still answers a cancelled non-prompt request.
* After the handler returns: with the handler's return held until a `session/cancel` for its session has been handled after the
  turn's removal, the answer is the handler's `end_turn`, and the cancel finds no turn.
* Permission requests: with a handler blocked on its context and one ignoring it, `Cancel` and `Prompt`'s automatic cancel each produce
  the `cancelled` outcome for every pending request of that session and none of another session.
* Malformed input: each probe line above gets the answer J1 names; a `null`-id request is answered with `"id": null`; an error response
  with `"id": null` gets no reply; a response with neither member fails the call; a line one byte over the limit is answered and the next
  line is handled.
* Gates: for each rule, a typed method refuses before writing, a received message is refused with the stated code, and the validation
  hook can accept it; before `initialize` every typed method but `initialize` returns `ErrNotInitialized`; `Negotiated` reports the
  record; a protocol-table rule renamed to a property the schema lacks fails `TestProtocolTableAppliesToCommittedSchema` (`Build` skips a
  rule whose subject the schema lacks, as `TestProtocolTableSkipsAbsentSubjects` requires, and the table test requires every rule to
  apply to the committed schema).
* Version: a zero version is filled on both sides; a handler's version 2 is answered `-32603`; an agent answering 2 makes `Initialize`
  return a `*ProtocolVersionError` and leaves the gates closed.
* Each new test is seen failing on a planted break in a scratch copy: the conversion to `cancelled` removed, the per-turn delete made
  unconditional again, the boundary removed (the cause read again after the turn's removal, so a late cancel overrides the answer),
  the permission answer removed, each J1 answer removed, a gate rule removed, the zero-version fill removed.
* Generating the side connections leaves the root's exported API listing byte-identical, and every engine case's `want.api` gains
  exactly the two side types, their constructors and their four forwarding methods; `TestRuntimeFilesCoverHandWritten` passes with
  `connection.go`, `doc.go`, `errors.go`, `extensions.go` and `helpers.go` as the hand-written files.
* The exported API listing grows only by `ErrPromptCancelled`, `ErrNotInitialized`, `CapabilityError`, `ProtocolVersionError`,
  `Negotiation`, `Negotiation.Allows`, the two `Negotiated` methods, `WithMaxMessageSize` and `NewResourceNotFound`.
* Every test passes on the three CI hosts and on the owner's Windows host.

## Pros and Cons of the Options

### P1. Every cancelled turn answered `cancelled`

* Good, because it holds whatever the handler returns, which is what the `StopReason` text requires.
* Good, because one answer satisfies `session/cancel` and `$/cancel_request` alike.
* Good, because the override stops at the handler's return, so a turn that finished is never reported cancelled.
* Bad, because the SDK overrides a result the handler returned while its turn was cancelled, which a reader of the handler does not
  see; the README says so.

### P2. Only errors converted

* Good, because the SDK never changes a result the handler chose.
* Bad, because a handler that finishes with `end_turn` after the cancel still answers non-conformingly.

### P3. Narrow `toReqErr`

* Good, because it is one line.
* Bad, because `-32603` is no more `cancelled` than `-32800`, and the cancellation page asks for `-32800` for every other request.

### P4. Documentation and a test

* Good, because nothing changes for existing agents.
* Bad, because the SDK's default keeps steering handlers into the violation the report rated High.

### Q1. Answer pending permission requests at once

* Good, because the answer does not wait on a handler that may be blocked on a user.
* Bad, because a handler's later return is discarded, and its work after the cancel is wasted unless it watches its context.

### Q2. Convert the handler's return

* Good, because the handler always decides when the answer goes.
* Bad, because a handler that does not watch its context holds the answer, and with it the agent's turn.

### Q3. Helper and documentation

* Good, because it adds no state to the connection.
* Bad, because every client rebuilds the connection's bookkeeping, as the report found none does.

### J1. JSON-RPC answers, with a size option

* Good, because every case JSON-RPC defines is answered, and none it forbids.
* Good, because an overlong line no longer ends the connection, and the limit is the consumer's.
* Bad, because the receive loop gains a classifier and a reader of its own.

### J2. Without the size change

* Good, because the reader is unchanged.
* Bad, because one overlong line still closes the connection without a word.

### J3. Logging only

* Good, because nothing changes.
* Bad, because it is the violation.

### G1. Record, enforce both ways, expose

* Good, because what the SDK sends conforms, and what it receives is checked by the 0015 path with its hooks.
* Good, because the rules sit in the protocol table, checked against the schema.
* Bad, because the table grows a second kind of capability rule, and the generator a check that takes the record.

### G2. Sending only

* Good, because nothing received is refused that is refused today.
* Bad, because an agent cannot rely on a client having respected its advertisement.

### G3. Record and expose

* Good, because it is small.
* Bad, because every application must enforce the gates, and most will not.

### G4. Hand-written checks

* Good, because no generator change.
* Bad, because protocol knowledge leaves the table, and a schema change no longer fails the table's test against the committed schema.

### N1. Fill zero, refuse another

* Good, because the zero value works and a wrong value is reported where it was made.
* Bad, because an agent that set `ProtocolVersion: 2` by mistake now fails `initialize`.

### N2. Always overwrite

* Good, because nothing can go wrong on the wire.
* Bad, because the application's value is replaced silently.

### N3. Document

* Good, because nothing changes.
* Bad, because the zero value answers version 0 today.

### K1. A constructor

* Good, because it matches the other codes.
* Neutral, because the `data` shape is the SDK's choice; the schema defines none.

### K2. Nothing

* Good, because no API grows.
* Bad, because the one code without a constructor is the one a client's `fs/read_text_file` handler needs most.

### O1. After `v0.16.0`

* Good, because the security fix ships without waiting for this work, and each release's breaking changes are explained by themselves.
* Bad, because the README and release notes are written in two rounds.

### O2. Before 0014 steps 6 to 8

* Good, because one release and one round of documentation.
* Bad, because the fix for issue #2 and its advisory wait for this record's PLAN.

### S1. Generated side connections

* Good, because protocol state is typed and generated with the rules that need it; no `any`, no installed hook.
* Good, because a schema with no prompt turn, no permission rule or no `initialize` gets side connections without that state.
* Bad, because the generator emits about 94 lines it did not, and every engine case's `want.api` changes.

### S2. Hand-written side connections

* Good, because the generator and the engine cases do not change for it.
* Bad, because the negotiation record is `any` asserted back, the gate is a function installed at construction, and the prompt and
  permission state is hand-written protocol code.

## More Information

* **Relation to 0015.** [0015-MADR-validate-absolute-protocol-paths.md](0015-MADR-validate-absolute-protocol-paths.md), "Decided with
  the acceptance", sent capability gates to their own record, to be written after 0015 is implemented. 0015 steps 7 and 8 are open; the
  owner asked for this record now, and what it builds on, `checkInbound`, the two hooks and `*ValidationError`, landed in `967dd26` and
  `a438f99`. This record is that capability record, widened by the audit; the middleware record 0015 named stays its own.
* **Open work this record does not change.** [0015-PLAN-validate-absolute-protocol-paths.md](0015-PLAN-validate-absolute-protocol-paths.md)
  steps 7 and 8, then [0014-PLAN-union-constructors-take-every-required-key.md](0014-PLAN-union-constructors-take-every-required-key.md)
  steps 6 to 8 and `v0.16.0`.
* **Not changed.** The outbound cancel queue, duplicate keys and batching (beyond the single `-32600`), as the report's notes found no
  requirement unmet. A client whose `Prompt` context ends returns at once rather than waiting for the agent's `cancelled` answer; the
  cancellation page leaves that waiting a MAY.
* **~~Noted, not decided~~ Decided 2026-10-12, in PLAN step 5.** A `session/cancel` handled before the prompt request's goroutine has
  stored the turn's cancel function finds no turn; this is read in `receive` (requests run on their own goroutine, notifications on the
  queue)~~, not probed, and the PLAN for this record tests it before deciding whether it needs a remedy~~. The PLAN's probe, prompt and
  cancel written back to back 1000 times, left 875 turns uncancelled, so a client that cancels at once got no `cancelled` answer,
  against the prompt-turn MUST. Asked, the owner chose to order the two by receipt: the connection gives every received request and
  notification its receipt number; a cancel applies to the session's running turn only if that turn's prompt was received before it,
  and otherwise is kept for the session; a prompt that stores its turn and finds a kept cancel received after it is cancelled at once;
  of two prompts for one session, the one received first is the one superseded. Not chosen: storing the turn in the receive loop,
  which puts protocol knowledge in the hand-written runtime S1 moved out; leaving the race and documenting it. After the change the
  probe leaves none uncancelled. This adds to item 1 below; nothing else in it changes.
* **Decided 2026-10-12, in PLAN step 9: the hook rules on each error.** As the PLAN first wrote it, `checkInbound` called the
  gate only after the SDK's validation passed, and the hook received the first error. A hook that accepts a validation error, as
  0015 allows for a bare MCP command, would then let through a method the record does not allow, the gate never having run. Asked,
  the owner chose to run both: the hook receives the validation error, and if it accepts it, the gate's error, and its answer to
  each decides; a message that passes both reaches the hook once, with `nil`, as before. Not chosen: keeping the first error only,
  as a documented limit. This adds to item 4's receiving clause; nothing else in it changes.
* **Decided 2026-10-12, in PLAN step 10: the terminal-auth rule has its own error.** The authentication page's "the Client MUST NOT
  send an `authenticate` request for a terminal method" depends on no capability, so a `*CapabilityError`, whose message names a
  capability not advertised, would state something false. Asked, the owner chose an exported sentinel, `ErrTerminalAuthMethod`,
  which the refusal wraps with the method and the id; received, it is answered `-32602`, like the other value gates. The rule
  carries no capability. Not chosen: a `*CapabilityError` with an empty `Capability` and a message for that case. The exported API
  gains the sentinel. This adds to item 4's sending clause; nothing else in it changes.
* **Decided 2026-10-12, in PLAN step 9: results are gated by value only.** As the PLAN first wrote it, a handler's result and a
  received response went through the side's whole gate, method gate included. A validation hook that accepted a method gate let
  the handler run, and the result was then refused `-32603` by the same gate, after the handler's effects. Asked, the owner chose
  that a result be checked against property and variant gates only: the method is gated when the request is received or sent.
  Not chosen: the whole gate on results, which makes a hook's acceptance of a method gate end in `-32603`. This adds to item 4's
  sending clause; nothing else in it changes.
* **Amended by 0019, 2026-10-13: `"params": null` is accepted.** J1 answers an object that is not a valid Request object `-32600`,
  and the implementation counted a `params` of `null` among those. ACP schema 1.25.0 declares `params` in its request and notification
  envelopes as the params union or `null`. The owner decided that a received `"params": null` is treated as omitted params; the SDK
  still never sends it. Recorded in
  [0019-MADR-remediate-debugging-pass-findings.md](0019-MADR-remediate-debugging-pass-findings.md), decision 13; nothing else in J1
  changes.
* **Revision.** Amended 2026-10-12 at the owner's request, before any decision: P1's override ends at the handler's return, so a
  cancel that arrives after the turn finished leaves the handler's answer as it is. Corrected the same day, writing the PLAN: a rule
  whose subject the schema lacks is skipped by `Build` and caught by the table's test, so "Confirmation" and G4 name that test; and
  an invalid JSON-RPC object without `id` is answered, not dropped (item 3). Amended the same day at the owner's question: option
  group S and outcome S1 (item 9), and P1's bookkeeping described as a per-turn pointer.
* **Relation to 0011.** [0011-MADR-harden-code-generator-engine.md](0011-MADR-harden-code-generator-engine.md) records, as facts of
  its time, the six runtime files and that generated code refers to `AgentSideConnection` and `ClientSideConnection` as hand-written
  names. S1 changes both; that record is not rewritten.
* **Sources.** <https://agentclientprotocol.com/protocol/initialization>, `/protocol/prompt-turn`, `/protocol/session-setup`,
  `/protocol/session-list`, `/protocol/draft/session-delete`, `/protocol/authentication`, `/protocol/file-system`,
  `/protocol/terminals`, `/protocol/session-config-options`, `/protocol/draft/cancellation` (published under the v1 draft path, the target
  of the schema's link for `$/cancel_request`), and the elicitation page; the committed `schema/schema.json` and `schema/meta.json`
  (`StopReason`, `RequestPermissionOutcome`, `RequestId`, `ErrorCode`, `Notice`, `CompactionUpdate`, `AuthMethodTerminal`); JSON-RPC 2.0
  (<https://www.jsonrpc.org/specification>), "Request object", "Notification", "Response object", "Error object" and "Batch"; Go:
  `context.WithCancelCause`, `context.Cause`, `encoding/json.Valid`, `bufio.Reader`.
* **Related records.** [0016-REPORT-acp-v1-conformance-audit.md](../reports/0016-REPORT-acp-v1-conformance-audit.md) (the audit),
  [0015-MADR-validate-absolute-protocol-paths.md](0015-MADR-validate-absolute-protocol-paths.md) (hooks, typed error, sending without
  opt-out), [0011-MADR-harden-code-generator-engine.md](0011-MADR-harden-code-generator-engine.md) (the protocol table).
* **Implementation.** [0017-PLAN-remediate-acp-v1-conformance-violations.md](0017-PLAN-remediate-acp-v1-conformance-violations.md).
