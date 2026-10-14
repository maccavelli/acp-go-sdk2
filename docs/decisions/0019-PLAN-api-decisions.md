---
status: in-progress
date: 2026-10-13
associated-madr: "0019-MADR-remediate-debugging-pass-findings.md"
---

# Implement phase 2 of 0019: the API and behaviour decisions, released with phase 1 as v0.18.0

Associated MADR: [0019-MADR-remediate-debugging-pass-findings.md](0019-MADR-remediate-debugging-pass-findings.md)

## Goal

Implement phase 2 of the MADR's outcome, with the owner's answers to decisions 1, 3, 4, 6 (as amended while planning this phase), 7,
9 and 10: B2's connection hand-off, B14, B16 to B18 with the writer goroutine, B19, B20, I8 and B21. Each change has a test seen to
fail first. Then release phases 1 and 2 together as `v0.18.0` (decision 12), with release notes in a 0019 report.

Phase 1 is [0019-PLAN-behaviour-fixes.md](0019-PLAN-behaviour-fixes.md), complete.

## Standards this plan follows

The ACP schema is still 1.25.0, the latest stable release, byte-identical to tag `schema-v1.25.0`. ACP documents are quoted from
`agentclientprotocol/agent-client-protocol` at `e5bcca615c2560a7d707365c773ae768ae595e88`, as `docs/protocol/v1/<page>.mdx:<line>`;
JSON-RPC 2.0 from <https://www.jsonrpc.org/specification>. Each quotation was checked verbatim at its line against a downloaded copy.

| Id | Source | Text | Bears on |
| :--- | :--- | :--- | :--- |
| Q1 | `elicitation.mdx:54` | "Agents **MUST NOT** request a mode the Client has not advertised." | B20 sending |
| Q2 | `elicitation.mdx:166` | "Requests using a mode the Client has not advertised produce JSON-RPC `-32602` (Invalid params)." | B20 receiving (the amended decision 6) |
| Q3 | `extensibility.mdx:56` | "implementations **MAY** expose and call custom JSON-RPC requests as long as their name starts with an underscore (`_`)." | B14 |
| Q4 | `extensibility.mdx:69` | "Upon receiving a custom request, implementations **MUST** respond accordingly with the provided `id`" | B14 |
| Q5 | `extensibility.mdx:113` | "Unlike with custom requests, implementations **SHOULD** ignore unrecognized notifications." | B14 |
| Q6 | `extensibility.mdx:117` | "Implementations **SHOULD** use the `_meta` field in capability objects to advertise support for extensions and their methods" | B14 (capabilities come from `initialize`) |
| Q7 | `initialization.mdx:24` | "Before a Session can be created, Clients **MUST** initialize the connection" | B14 (gating before `initialize` is SDK policy, consistent with Q6) |
| Q8 | `transports.mdx:40` | the stdio sequence ends "Client->>Agent Process: Close stdin, terminate subprocess" | B17 |
| Q9 | JSON-RPC, Error object | codes "-32000 to -32099" are "Server error", implementation-defined | B18 |
| Q10 | `schema/schema.json:3493` | ACP's `ErrorCode` uses "the reserved range (-32000 to -32099) for protocol-specific errors" (`-32000` auth required, `-32002` resource not found) | B18: no new code in that range |

Where the sources are silent, the choice is SDK policy and documented as such: what a connection over its request limit answers
(B18), and whether a send whose message is already being written can be withdrawn (B16; it cannot, as a `Write` is not interruptible).

## Facts this plan rests on

Each code fact was asserted, snippet at line, on `e5e716e` (36 of 36 facts and quotations hold). Each finding was reproduced on
`e5e716e` in a scratch copy by a probe that fails while the defect is present, run with `go test -race -count=1`.

| Finding | The code as it is | Reproduction on `e5e716e` |
| :--- | :--- | :--- |
| B2 | The generated constructor calls `start` with nothing between it and the side's value (`cmd/generate/internal/emit/dispatch.go:197`); the example agent assigns its connection afterwards (`example/agent/main.go:450`, `example_agent_test.go:129`), and its comment names an `acp.AgentConnAware` that does not exist (`example/agent/main.go:154`). | `TestP2_B2_SetterAfterConstructorRaces`: one `DATA RACE`, the handler's read in `Initialize` against the assignment after the constructor. |
| B14 | `handleWithExtensions` hands a `_` method straight to `HandleExtensionMethod` (`extensions.go:37`, `:43`); `WithInboundRewrite` says it "sees every request, notification" (`connection.go:456`); the README says the extension calls "hold no record" (`README.md:327`). | `TestP2_B14_ExtensionSkipsHooksAndGate`: before `initialize`, `_x/ping` ran its handler while `session/list` was refused; neither hook saw `_x/ping`. |
| B16 | `sendMessage` holds `writeMu` across `c.w.Write(b)` (`connection.go:1313`, `:1315`). | `TestP2_B16_WriteIgnoresCtx`: a 200 ms deadline still blocked after 1.5 s on a peer that does not read. |
| B17 | `Done` is the reader's end (`connection.go:1757`); `OverflowCloseConnection` "closes the connection" (`:398`) by `shutdownReceive` alone (`:1045`); no `Close` exists. | `TestP2_B17_SendAfterDone`: a notification written after `Done`; `*Connection` is not an `io.Closer`. |
| B18 | Each request gets its own goroutine (`connection.go:1000`) with no limit. | `TestP2_B18_RequestsUnbounded`: 200 blocking handlers running at once. |
| B19 | `RequestError` has no cause (`errors.go:11`); a local cancel maps to `-32800` (`errors.go:207`), a disconnect to a plain `-32603` (`connection.go:1480`). | `TestP2_B19_ErrorsLoseIdentity`: a request past its deadline is not `errors.Is(context.DeadlineExceeded)`; a disconnect wraps nothing. |
| B20 | `CreateElicitationRequest` has a catch-all `Other *CreateElicitationOther` (`types_gen.go:3094`) that its `gates` (`:19938`) never checks; the table has `form` and `url` rules only (`cmd/generate/internal/model/acp.go:121-122`); the gate kinds start at `model/protocol.go:41`. | `TestP2_B20_UndeclaredModeBothWays`: sending mode `telepathy` passes the gate; receiving it reaches the handler, not `-32602`. |
| I8 | Seven `MarshalJSON` methods only copy into an alias: `AgentCapabilities` (`types_gen.go:113`), `AuthCapabilities` (`:554`), `ClientCapabilities` (`:1396`), `FileSystemCapabilities` (`:5585`), `InitializeRequest` (`:6345`), `McpCapabilities` (`:7012`), `PromptCapabilities` (`:9651`). `emitObjectMarshal` is called whenever a type has a default (`cmd/generate/internal/emit/types.go:270`), but adds a statement only for a nilable array or object default (`:295`), a fill, an `Extra` or members. | A script over `types_gen.go`: 7 methods of exactly that shape. |
| B21 | The 0017 MADR's outcome excludes "the connection's end" (`0017-MADR-remediate-acp-v1-conformance-violations.md:282`). | `TestP2_B21_ConnectionEndAnsweredCancelled`: the answer after the connection's end is `{"stopReason":"cancelled"}`. |

**A downstream consumer.** The module in the owner's workspace that requires `v0.17.1` names its setter `SetConnection`, so it gains
the hand-off by renaming it `SetAgentConnection`; it calls `CallExtension` only after `initialize`; it reads `*acp.RequestError`
codes, which this phase keeps; and it uses `OverflowDropNewest`, which this phase does not change.

## Scope

In scope: `connection.go`, `errors.go`, `extensions.go`; the generator (`cmd/generate/internal/model`, `internal/emit`, the stub in
`cmd/generate/main_test.go`, engine cases); the regenerated `*_gen.go` files; tests; `example/agent` and `example_agent_test.go`;
`README.md`, `docs/architecture.md`; the 0017 MADR's amendment (B21); the release notes report; the records.

Out of scope: phases 3 to 5 (generator hardening, tooling, idioms and performance); outbound extension calls stay ungated; the
initialisms; `encoding/json` v2.

## Design

Every new test is seen to fail on a planted break, in a scratch copy, before it is relied on.

### B19. Local failures keep their Go identity (decision 3)

* `errors.go`: `RequestError` gains an unexported `cause error` and `Unwrap() error`, returning it. JSON and `Error()` are unchanged,
  so the wire and every code stay as they are. New exported sentinels: `ErrPeerDisconnected` (the peer ended the connection) and,
  with B17, `ErrConnectionClosed` (`Close` ended it).
* Each local failure sets its cause: a context's end (`toReqErr(context.Cause(ctx))` keeps the cause), in `waitForResponse`,
  `waitNotificationsUpTo` and the early check of `sendRequest`, `SendRequestNoResult` and `SendNotification`; a disconnect in the
  same places, with `ErrPeerDisconnected`. A peer's error answer has no cause, so a peer's `-32800` does not unwrap to
  `context.Canceled`.
* Tests, `errors_test.go`: `TestLocalErrorsUnwrap` (cancel, deadline, disconnect: `errors.Is` holds for `context.Canceled`,
  `context.DeadlineExceeded` and `ErrPeerDisconnected`, and each code is as before); `TestPeerErrorsWrapNothing`. The tests that pin
  the codes (`TestConnectionOutboundCancelRequest_SendsNotification`, `TestConnectionOutboundRequestTimeout_ReturnsInternalError`)
  still pass. Planted break: drop the cause at one site.

### B16 and B17. A writer goroutine, `Close`, and nothing sent after the end (decision 4)

* `connection.go`: a writer goroutine, started by `start`, takes each message from an unbuffered channel and writes it. `sendMessage`
  takes a context: it hands the message over, or returns when the context ends or the connection is done; then it waits for the
  write, or returns when the context ends. A message whose write has begun cannot be withdrawn; a sender whose context ends then
  returns its context's error, and the message may still reach the peer. An answer to a received request is sent under a context
  that ends only with the connection. Order on the wire is the order the writer receives messages.
* `Close() error` on `Connection`, idempotent: ends the connection's context with `ErrConnectionClosed`, stops the writer, closes the
  reader and the writer when they implement `io.Closer`, and returns their errors joined. The side connections gain `Close`,
  generated, forwarding to it. `OverflowCloseConnection` closes the connection through it.
* After `Done`, every send fails at once with a `*RequestError` (`-32603`) whose cause is `ErrPeerDisconnected` or
  `ErrConnectionClosed`, and writes nothing; calls waiting for a response end the same way.
* Tests: `TestSendHonoursContextWhileWriteBlocks` (a peer that does not read: a send with a 200 ms deadline returns within 1 s with
  `context.DeadlineExceeded`; a second send also returns by its context; the connection still works for a reading peer afterwards);
  `TestCloseEndsConnection` (`Done` closes, a pending call returns `ErrConnectionClosed`, a send after it fails without writing, the
  pipes are closed, a second `Close` returns nil); `TestSendAfterPeerEndFails`; `TestOverflowCloseClosesTransport`. Existing tests
  that send after `Done` are found by the full run, and each is a deviation to bring to the owner. Planted breaks: write under the
  lock again; skip the after-`Done` check; leave the transport open on `Close`.

### B18. A bound on concurrent inbound requests (decision 4)

* `WithMaxConcurrentRequests(n int)`: unlimited when `n` is zero or less, the default. When `n` requests are running, the receive
  loop answers a further one at once, without starting its handler, with `-32603` and `data` `{"error": "too many concurrent
  requests", "limit": n}`, and logs it at Debug. Notifications and `$/cancel_request` are not counted. The code is `-32603`, not one
  of `-32000` to `-32099`, which ACP's `ErrorCode` uses for its own errors (Q10).
* Not chosen: blocking the reader until a request ends. A running handler may be waiting for a response the reader must deliver,
  such as a prompt turn waiting for `session/request_permission`, so blocking can deadlock the connection.
* Tests, `connection_cancel_test.go`: `TestMaxConcurrentRequests` (limit 2: the third of three blocking requests is answered
  `-32603` at once and its handler never runs; once one returns, a new request runs); unlimited by default (the 200-request case).
  Planted break: ignore the limit.

### B2. The connection hand-off (decision 1)

* Hand-written, beside the runtime: `AgentConnAware` (`SetAgentConnection(*AgentSideConnection)`) and `ClientConnAware`
  (`SetClientConnection(*ClientSideConnection)`), exported and documented: the constructor calls the method, when the agent or the
  client value has it, after assigning every field and before reading starts, so a handler always finds it set. The method must only
  store the connection: the connection is not running yet, and a send from it would wait.
* Generator: `emitSideConnection` emits the call between the assignment of `conn` and `start`. The stub in
  `cmd/generate/main_test.go` declares the two interfaces.
* `example/agent/main.go` and `example_agent_test.go` drop the call after the constructor, which the constructor now makes; the comment
  at `example/agent/main.go:154` names the interface that exists.
* Tests: `TestConnAwareCalledBeforeReading`, for both sides, with input already waiting, run under `-race` (no race, the setter sees a
  non-nil connection, a handler reading the field finds it); `TestDispatchFollowsTheModel` gains the call. Planted break: call the
  setter after `start`.

### B14. Extension messages are hooked and gated (decision 7)

* Generator: each side gains `inboundGate() gateFunc`, its `gate` when the schema has `initialize`, nil otherwise, so the
  hand-written `extensions.go`, compiled with every engine case, can call it.
* `extensions.go`: `handleWithExtensions` runs `checkInbound(ctx, method, &raw, inboundGate())` on a `_` method before its handler,
  with `raw` the message's params as a `json.RawMessage`. The rewrite hook may replace it; the handler gets what the hooks leave. A
  refusal is answered as any typed one: before `initialize`, `-32601` for a request and a dropped notification; a hook's error as the
  hooks document. An unrecognized `_` notification is still ignored (Q5). Outbound `CallExtension` and `NotifyExtension` stay
  ungated, and their documentation says so.
* Tests, `gates_test.go`: `TestExtensionMessagesHookedAndGated` (before `initialize` a `_` request is answered `-32601` and its
  handler does not run, and a `_` notification is dropped; after it, both hooks see the method with `*json.RawMessage` params, a
  rewrite reaches the handler, and a refusal by the validation hook is answered `-32602`). Planted breaks: skip `checkInbound`; pass a
  nil gate.

### B20. An undeclared elicitation mode is refused both ways (amended decision 6)

* Model: a new gate kind, `GateUndeclaredVariant`: the union's catch-all variant must be unset, a value the schema reserves for
  custom or future variants being one no capability advertises. The rule names the union and a capability prefix, and the model
  resolves the catch-all's field and its discriminator's Go field, refusing a union without a catch-all.
* Table: `{Side: client, Definition: "CreateElicitationRequest", Capability: "elicitation", Kind: GateUndeclaredVariant}`, its
  source Q2's sentence.
* Emitted: `if u.Other != nil` returns a `*CapabilityError` with `Capability` `"elicitation." + u.Other.Mode` and `Variant`
  `u.Other.Mode`, so the message reads "capability elicitation.telepathy not advertised: variant telepathy". Sending it is refused;
  received, `inboundError` answers `-32602`, as for every value gate, and the validation hook can accept it.
* Tests: `TestUndeclaredElicitationMode` (refused on send; `-32602` on receipt with the handler not run; accepted by a validation
  hook that returns nil); `TestValueGates` counts; the model's `TestCapabilityRulesApply` and refusal cases for the kind; engine case
  `accept-capability-undeclared-variant.txtar` with a behaviour test; `checkCommittedGates` expects 19 rules and 22 checks. Planted
  breaks: drop the emitted check; resolve the kind on a union without a catch-all.

### I8. Identity `MarshalJSON` methods are not emitted (decision 9)

* `emitObjectMarshal` is called only when it adds a statement: a nilable array or object default, a fill, an `Extra`, or members.
  The seven methods go; their types keep `UnmarshalJSON`.
* Proof the bytes do not change: a scratch comparison of `json.Marshal` of each of the seven types, on every schema instance the
  conformance builder makes, before and after; `TestConformanceRoundTrip` and the JSON goldens unchanged. The API listing shows the
  seven methods removed. Planted break: none needed for a removal; the byte comparison is the check.

### B21. The 0017 text (decision 10)

* An amendment in the 0017 MADR's "More Information": a turn whose connection ended is answered `cancelled`, which no peer receives;
  the outcome's "for any reason but the connection's end" is superseded. No code change.

## Implementation Steps

Each step ends with its gates and an execution record; the owner commits it. Agents do not commit on `main`.

1. **Records.** This PLAN and its `docs/README.md` row; the MADR's amendment of decision 6 is already written.
2. **B19**, `errors.go` and `connection.go`, with its tests.
3. **B16 and B17**, `connection.go`, the generated side `Close`, with their tests.
4. **B18**, `connection.go`, with its tests.
5. **B2**, the interfaces, the generator, the examples, with their tests.
6. **B14**, the generator's `inboundGate`, `extensions.go`, with its tests.
7. **B20**, the model, the table, the emitter, with its tests and engine case.
8. **I8**, the emitter, with the byte comparison.
9. **B21 and documentation**: the 0017 amendment; `README.md` (the hand-off in "Get Started", `Close`, `WithMaxConcurrentRequests`,
   error identity in "Errors on the wire", extension messages hooked and gated, the undeclared mode); `docs/architecture.md`.
10. **Release notes**: `docs/reports/0019-REPORT-release-notes-v0-18-0.md`, phases 1 and 2 against `v0.17.1`, shaped like the 0017
    and 0018 reports, with every behaviour change and API change named; indexed in `docs/README.md`.
11. **Verification and close**: the gates over the tree, the phase 1 and phase 2 probes, the API listing against `v0.17.1` with
    exactly the expected changes, the example checks, Windows; after CI passes on the pushed commits, this PLAN and its row become
    `complete`; the owner tags `v0.18.0` and publishes the release from the report.

## Verification

Per code step, through a Python runner outside the tree: regenerate, and the diff of the generated files is exactly the step's; `make
pre-add-check`; `scripts/go-precheck.sh` on each new `.go` file; `GOEXPERIMENT=nojsonv2 go test ./...`; `GOOS=windows go vet ./...` in
both modules; each new test seen to fail on its planted break in a scratch copy; `make check-records`, `make markdownlint`, the
identifier scan; Windows `go test -count=1 -shuffle=on ./...` in both modules; and the API listing against `v0.17.1`, whose difference
grows only by the step's own lines.

Expected API listing difference at the end, against `v0.17.1`: added `func (*Connection).Close() error`, `func
(*AgentSideConnection).Close() error`, `func (*ClientSideConnection).Close() error`, `func (*RequestError).Unwrap() error`, `var
ErrPeerDisconnected error`, `var ErrConnectionClosed error`, `func WithMaxConcurrentRequests(n int) ConnectionOption`, `type
AgentConnAware interface{…}`, `type ClientConnAware interface{…}`; removed the seven `MarshalJSON` methods of I8. Nothing else.

Phase acceptance: every phase 2 finding has a test or a recorded check; the phase 2 probes pass, B21's being text only and B18's exempt (step 4's deviation of 2026-10-13); the phase 1
probes still pass, with phase 1's three exemptions; the API listing matches the expected difference; CI passes; `v0.18.0` is tagged
and its release published.

## Rollout and Rollback

`v0.18.0` is a minor release. The behaviour changes its notes must name, beyond phase 1's: sends after the connection's end fail
instead of being written; a send returns when its context ends even while the peer does not read; `OverflowCloseConnection` now
closes the transport; extension requests before `initialize` are refused and extension messages go through the hooks; an
undeclared elicitation mode is refused both ways; the seven types lose an explicit `MarshalJSON` (their encoding is unchanged). Each
step is one commit; rolling back a step is reverting it, after the steps that build on it (3 uses 2's sentinels; 6 uses 5's
generated accessors only in the same files).

## Execution record

### Step 1, records, 2026-10-13

This PLAN, its `docs/README.md` row, and the MADR's amendment of decision 6, made while planning; `make check-records` and `make
markdownlint` exit 0, and the identifier scan of the three files finds nothing. The owner committed them as `484f65d` and approved
execution on 2026-10-13.

### Step 2, error identity, 2026-10-13

What was built:

* `errors.go`: `RequestError` gains the unexported `cause` and the exported `Unwrap() error`, nil-safe, and the unexported
  `withCause`. Its documentation says which errors carry a cause. `ErrPeerDisconnected` is added. `toReqErr` keeps the error it
  converts as the cause, for a cancelled context (`-32800`) and for any other error (`-32603`).
* `connection.go`: the two disconnect errors carry `ErrPeerDisconnected`; `SendNotification`'s ended context carries
  `context.Cause(ctx)`; the ended contexts of `sendRequest`, `SendRequestNoResult`, `waitForResponse` and `waitNotificationsUpTo`
  carry theirs through `toReqErr`. Beyond the sites "Design", B19, lists, the three failed writes (in `sendRequest`,
  `SendRequestNoResult` and `SendNotification`) carry the write's error, which the same rule, a local failure keeps its cause, covers
  and the type's documentation names. A result that fails to decode and a result that fails validation are not failures of the
  connection, and carry none. Every code is unchanged.

Tests added, `errors_test.go`: `TestLocalErrorsUnwrap` (a cancelled request `-32800` and a cancelled notification `-32603` wrap
`context.Canceled`, a request past its deadline `-32603` wraps `context.DeadlineExceeded`, a request whose peer disconnected `-32603`
wraps `ErrPeerDisconnected`) and `TestPeerErrorsWrapNothing` (a peer's `-32800` answer unwraps to nothing). `go test -race -count=10`
of the two: ok; the whole root suite under `-race`: ok, the tests that pin the codes included.

Seen to fail, each in its own scratch copy (5 of 5):

* `b19-no-disconnect-cause`: "a request whose peer disconnected: … does not wrap acp: peer disconnected".
* `b19-no-cancel-cause` (`toReqErr` without the cause): "a cancelled request: … does not wrap context canceled".
* `b19-no-notification-cause`: "a cancelled notification: … does not wrap context canceled".
* `b19-unwrap-nil` (`Unwrap` returns nil): the cancelled request, the cancelled notification and the deadline each "does not wrap".
* `b19-peer-error-caused` (a decoded answer given a cause): `TestPeerErrorsWrapNothing`, "the peer's -32800 unwraps to context
  canceled".

The phase 2 probes, re-run on the step 2 tree: `TestP2_B19_ErrorsLoseIdentity` now passes; the others fail, their steps being later,
but B21's, which is text only.

Gates, through the Python runner: regenerating changes no generated file; `make pre-add-check` exit 0; no new `.go` file;
`GOEXPERIMENT=nojsonv2 go test ./...` exit 0; `GOOS=windows go vet` exit 0 in both modules; `make check-records` and `make
markdownlint` exit 0; the identifier scan finds nothing; the API listing against `v0.17.1`, 2411 and 2413 lines, adds exactly
`method (*RequestError) Unwrap () error` and `var ErrPeerDisconnected`, as expected for this step. Windows: `go test -count=1
-shuffle=on ./...` exit 0 at the root and in `cmd/generate`.

### Step 3, writer, `Close` and the end of a connection, 2026-10-13

What was built:

* `connection.go`: `writeMu` is replaced by a writer goroutine, started by `start`, that takes each message from an unbuffered
  channel and writes it, and stops when the connection's context ends. `sendMessage` takes a context: it returns `endError` at once
  when the connection has ended; otherwise it hands the message to the writer, or returns when its context or the connection ends;
  then it waits for the write, or returns when its context ends. Answers to received messages are sent under
  `context.Background()`, so they wait only for the writer or the connection's end. `sendRequest` and `SendRequestNoResult` whose
  send ended on their context send `$/cancel_request` for the id, since the message may have been written, and return `toReqErr`
  of the error, which keeps its cause and its code; `SendNotification` returns the same.
* `Close() error` on `Connection`: once, it marks the connection closed locally, ends its context with `ErrConnectionClosed`, and
  closes the reader and the writer when they implement `io.Closer`, one pointer passed as both closed once, returning their errors
  joined; a later call returns that result. Its body is `closeWith(cause)`, which `OverflowCloseConnection` calls with the overflow
  as the cause, before `shutdownReceive`, so the context's cause stays the overflow, which the existing overflow tests pin, and the
  connection is marked closed before its context ends, so every waiter reads the same cause.
* `endError`: `ErrConnectionClosed` when this side closed the connection, wrapping the overflow when that closed it; otherwise
  `ErrPeerDisconnected`, wrapping how the peer ended it. The errors of a call waiting for a response, or for the notifications
  before it, are made when they are returned, from `endError`, and their message is "connection ended …" in place of "peer
  disconnected …"; their code is unchanged.
* `errors.go`: `ErrConnectionClosed`. The documentation of `Done` and of `OverflowCloseConnection` says what ends the connection;
  the latter no longer claims the behaviour of every earlier release, which closing the transport changes.
* Generator, `cmd/generate/internal/emit/dispatch.go`: each side connection gains `Close() error`, forwarding to its connection, and
  the documentation of its `Done` follows `Connection.Done`. `cmd/generate/main_test.go`'s stub gains `Close`. Regenerated:
  `agent_gen.go` and `client_gen.go`, and the 60 engine goldens of the deviation below.

Tests added, `connection_close_test.go`: `TestSendHonoursContextWhileWriteBlocks` (a peer that does not read: a send whose write
has begun and one behind it each return within 1 s of a 200 ms deadline, `-32603` wrapping `context.DeadlineExceeded`; once the peer
reads, a third send succeeds and the peer reads the first message and the third, not the second); `TestCloseEndsConnection`
(`Done` closes, a pending call returns `-32603` wrapping `ErrConnectionClosed`, a send after it fails the same way and writes
nothing, the writer is closed and its reader sees the end, a peer's write meets a closed reader, a second `Close` returns nil);
`TestSendAfterPeerEndFails` (a notification and a request after the peer ended each fail `-32603` wrapping `ErrPeerDisconnected`,
not `ErrConnectionClosed`, and nothing is written); `TestOverflowCloseClosesTransport` (after an overflow the reader is closed, and
a send fails `-32603` wrapping `ErrConnectionClosed` and the overflow). `go test -race -count=20` of the four: ok. The full root and
generator suites under `-race`: ok, the generator's after the deviation below. No existing test sends after `Done`; the deviation
"Design", B16 and B17, provides for did not arise.

Seen to fail, each in its own scratch copy (3 of 3):

* `b16-write-under-lock` (the write runs under a mutex on the sender's goroutine, as before): `TestSendHonoursContextWhileWriteBlocks`,
  "a send whose write blocks did not return within 1s of its 200ms deadline".
* `b17-no-after-done-check` (no check before the hand-off, no end case at it, and a writer that outlives the connection):
  `TestSendAfterPeerEndFails`, "a notification after the peer ended: <nil>, want a *RequestError with code -32603" and "2 writes
  after the peer ended, want 0".
* `b17-transport-left-open` (`Close` closes nothing): `TestCloseEndsConnection`, "Close left the writer open";
  `TestOverflowCloseClosesTransport`, "the reader's close did not happen".

The phase 2 probes, re-run on the step 3 tree: `TestP2_B16_WriteIgnoresCtx` and `TestP2_B17_SendAfterDone` now pass, the latter with
0 bytes written, as does B19's; the others fail, their steps being later, but B21's, which is text only.

Gates, through the Python runner: regenerating changes no generated file; `make pre-add-check` exit 0, after `gofmt` aligned the
connection's struct fields, which the first run reported; the new test file passes the same check; `GOEXPERIMENT=nojsonv2 go test
./...` exit 0; `GOOS=windows go vet` exit 0 in both modules; `make check-records` and `make markdownlint` exit 0; the identifier scan
finds nothing; the API listing against `v0.17.1`, 2411 and 2417 lines, adds exactly the step 2 pair and `method (*Connection) Close
() error`, `method (*AgentSideConnection) Close () error`, `method (*ClientSideConnection) Close () error` and `var
ErrConnectionClosed`, as expected for this step. Windows: `go test -count=1 -shuffle=on ./...` exit 0 at the root and in
`cmd/generate`.

Deviation, 2026-10-13: the generated `Close` of `AgentSideConnection` and `ClientSideConnection` changes the `want.api` golden of
every engine case, 60 files under `cmd/generate/testdata/`, which "Design", B16 and B17, does not name. Found by the generator
suite: `TestEngineCases` failed in each case with "API differs from want.api". Measured in a scratch copy before choosing:
`go test -run TestEngineCases . -update` adds `func (*AgentSideConnection).Close() error` and `func
(*ClientSideConnection).Close() error` to each golden and changes no other line and no file outside `testdata`, and the generator
suite then passes. Decision (owner): accept the goldens; the 60 files join step 3's scope. The MADR is unchanged: no decision and no
asserted fact moves.

Amended 2026-10-13, during step 7: a send whose write had finished could return its context's error; see step 7's deviation.

### Step 4, a bound on concurrent inbound requests, 2026-10-13

What was built:

* `connection.go`: `WithMaxConcurrentRequests(n int)`, unlimited when `n` is zero or less, the default. A connection with a limit
  holds a slot channel of capacity `n`. The receive loop takes a slot for each request, a null id's included, before counting it as
  received; with none free it answers at once, `-32603` with `data` `{"error": "too many concurrent requests", "limit": n}`, logs
  the refusal at Debug, and does not start the handler. It never waits for a slot. Notifications and `$/cancel_request` take none.
* A slot is released when the handler returns, before its answer is written, through a `handled` callback `handleInbound` now takes
  (nil for notifications), and once more, idempotently, when the request's goroutine ends, which covers a connection with no handler.
  "Design", B18, says a request counts while it is running; releasing before the answer means a peer that sends a request after
  reading an answer is not refused for a slot the answered request still held.

Tests added, `connection_cancel_test.go`: `TestMaxConcurrentRequests`. With a limit of 2, of three blocking requests the third is
answered first, `-32603` with the error and `"limit": 2`; once one of the two running requests is released and answered, a fourth
runs; the handlers ran for requests 1, 2 and 4 only. Without the option, 200 blocking requests all run. `go test -race -count=20` of
it: ok.

Seen to fail, in a scratch copy (1 of 1): `b18-limit-ignored` (the check is never taken): "the third request's answer did not
happen".

The phase 2 probes, re-run on the step 4 tree: B16's, B17's and B19's pass; `TestP2_B18_RequestsUnbounded` still reports 200
running, which the deviation below records; the others fail, their steps being later, but B21's, which is text only.

Gates, through the Python runner: regenerating changes no generated file; `make pre-add-check` exit 0; no new `.go` file;
`GOEXPERIMENT=nojsonv2 go test ./...` exit 0; `GOOS=windows go vet` exit 0 in both modules; `make check-records` and `make
markdownlint` exit 0; the identifier scan finds nothing; the API listing against `v0.17.1`, 2411 and 2418 lines, adds exactly the
six entries of steps 2 and 3 and `func WithMaxConcurrentRequests (n int) ConnectionOption`, as expected for this step. Windows: `go
test -count=1 -shuffle=on ./...` exit 0 at the root and in `cmd/generate`.

Deviation, 2026-10-13: "Verification" requires the phase 2 probes to pass, and `TestP2_B18_RequestsUnbounded` cannot. It measures
the default, 200 blocking requests with no option, and "Design", B18, after decision 4, keeps the default unlimited; on the step 4
tree it still reports "nothing bounds concurrent inbound requests (200 running)". With the option set the bound holds:
`TestMaxConcurrentRequests` passes, and the planted break that ignores the limit fails it. Decision (owner): the B18 probe is exempt,
as phase 1 exempted three of its probes, and `TestMaxConcurrentRequests` is the evidence for B18; the acceptance line is annotated.
The MADR is unchanged: no decision and no asserted fact moves.

### Step 5, the connection hand-off, 2026-10-13

What was built:

* `connection.go`: `AgentConnAware` (`SetAgentConnection(conn *AgentSideConnection)`) and `ClientConnAware`
  (`SetClientConnection(conn *ClientSideConnection)`), exported and documented: the constructor calls the method once, after building
  the connection and before reading starts, and the method must only store the connection, since a send from it would block the
  constructor.
* Generator, `cmd/generate/internal/emit/dispatch.go`: `side` names each side's interface and setter, and `emitSideConnection` emits
  `if aware, ok := agent.(AgentConnAware); ok { aware.SetAgentConnection(asc) }`, and the client's equivalent, between the
  assignment of `conn` and `start`. `cmd/generate/main_test.go`'s stub declares the two interfaces. Regenerated: `agent_gen.go` and
  `client_gen.go`; the engine goldens list no function bodies and are unchanged.
* `example/agent/main.go` and `example_agent_test.go` drop the call after the constructor, and the comment on each
  `SetAgentConnection` says it implements `AgentConnAware`, which the constructor calls before reading starts. `Example_agent` keeps
  its `asc` for the comment that blocks on `Done`, as `_ = asc`.

Tests added, `conn_aware_test.go`: `TestConnAwareCalledBeforeReading`, for both sides, with a line already waiting on the input. The
input's first read reports whether the setter had returned; the setter pauses 10 ms before it marks itself done, so a read begun
during it finds the hand-off unfinished. Each side requires the setter to have returned before the first read and to have been given
the connection the constructor returns, built; the agent's `initialize` handler, run for the waiting line, finds the connection set.
`go test -race -count=20` of it, with `Example_agent`: ok, no race report. `TestSideConnectionsFollowTheModel` pins the call, per
the first deviation below.

Seen to fail, in a scratch copy (1 of 1): `b2-setter-after-start` (the generator emits the call after `start`, regenerated):
"reading started before SetAgentConnection returned" and "the initialize handler found the connection unset" for the agent,
"reading started before SetClientConnection returned" for the client. It failed by these assertions; the race detector reported
nothing.

The phase 2 probes, re-run on the step 5 tree with B2's restated, per the second deviation below: `TestP2_B2_HandOffBeforeReading`
passes, "ran 50 hand-offs; the handler found the connection unset 0 times", with no race report; on `e5e716e`, extracted with `git
archive` into a scratch directory, it fails, "CONFIRMED: the initialize handler found the connection unset in 50 of 50 hand-offs".
B16's, B17's and B19's pass; B18's is exempt (step 4); the others fail, their steps being later, but B21's, which is text only. The
original B2 probe's text is kept in the scratch directory beside the restated one.

Gates, through the Python runner: regenerating changes no generated file; `make pre-add-check` exit 0, after `gofmt` aligned two
`side` literals, which the first run reported; the new test file passes the same check; `GOEXPERIMENT=nojsonv2 go test ./...` exit
0; `GOOS=windows go vet` exit 0 in both modules; `make check-records` and `make markdownlint` exit 0; the identifier scan finds
nothing; the API listing against `v0.17.1`, 2411 and 2420 lines, adds exactly the seven entries of steps 2 to 4 and `type
AgentConnAware interface { SetAgentConnection(conn *AgentSideConnection) }` and `type ClientConnAware interface {
SetClientConnection(conn *ClientSideConnection) }`, as expected for this step. Windows: `go test -count=1 -shuffle=on ./...` exit 0
at the root and in `cmd/generate`.

Deviation, 2026-10-13: "Design", B2, says `TestDispatchFollowsTheModel` gains the call. The test that pins the constructor is 0018's
`TestSideConnectionsFollowTheModel` (`cmd/generate/internal/emit/dispatch_test.go`), which requires the exact text of the
assignment, `start` and `return`; with the call between them it failed on both sides, with prompt turns and without.
`TestDispatchFollowsTheModel` checks the names the dispatch takes from the model, and the interface and setter names are
hand-written. Decision (owner): `TestSideConnectionsFollowTheModel`'s pinned sequence gains the call, assignment, then the call,
then `start`, then `return`, on both sides; `TestDispatchFollowsTheModel` is unchanged. The MADR is unchanged.

Deviation, 2026-10-13: the phase 2 probe of B2, `TestP2_B2_SetterAfterConstructorRaces`, assigns the connection to the
agent after the constructor returns, as the examples did, so it races by construction whatever the SDK does: on the step 5 tree it
still reports one `DATA RACE`. This step removes that assignment from the examples, and "Verification" requires the probe to pass.
Decision (owner): restate the probe as the examples' new pattern, an agent that implements `SetAgentConnection` and assigns nothing
after the constructor, whose `initialize` handler, run for input already waiting, must find its connection set with no race report;
show that the restated probe fails on `e5e716e`, where the setter is never called, and passes on the step 5 tree, and record both
runs here. The MADR is unchanged.

### Step 6, extension messages hooked and gated, 2026-10-13

What was built:

* Generator, `cmd/generate/internal/emit/dispatch.go`: each side connection gains `inboundGate() gateFunc`, returning its `gate`
  when the schema has `initialize` and nil otherwise, so `extensions.go`, compiled with every engine case, can call it.
  `cmd/generate/main_test.go`'s stub declares `gateFunc`. Regenerated: `agent_gen.go` and `client_gen.go`; the engine goldens list
  no unexported methods and are unchanged.
* `extensions.go`: both `handleWithExtensions` hand a `_` message to `handleExtension`, which runs `checkInbound(ctx, method, &raw,
  gate)`, with `raw` the message's params as a `json.RawMessage`, before it looks for `ExtensionMethodHandler`; the handler gets
  what the hooks leave. A refusal is answered as a typed one's, through `inboundError`: before `initialize`, `-32601` for a request,
  and a notification dropped without a log line, since `handleInbound` already ignores a `-32601` for a `_` notification (Q5).
  `ExtensionMethodHandler`'s documentation says so. `CallExtension` and `NotifyExtension` stay ungated; their documentation already
  says they do not check what `initialize` negotiated.
* `connection.go`: `WithInboundRewrite`'s documentation says an extension message's params is a `*json.RawMessage`.

Tests added, `gates_test.go`: `TestExtensionMessagesHookedAndGated`. Before `initialize`, a `_` request is answered `-32601`, and a
`_` notification is refused, its handler not run. After it, both hooks see `_x/echo`, `_x/refuse` and `_x/late` with
`*json.RawMessage` params; the rewrite of `_x/echo` reaches the handler and its answer; `_x/refuse`, which the validation hook
refuses, is answered `-32602` and its handler does not run; the handler runs for `_x/echo` and `_x/late` only, in that order. The
params' type is named by type assertion: with the default `encoding/json`, `%T` prints `*jsontext.Value`, the alias target, which
the first run showed. `go test -race -count=20` of it and the `TestExtensionMethods` tests: ok.

Seen to fail, each in its own scratch copy (2 of 2):

* `b14-no-check-inbound` (no `checkInbound` on a `_` message): "a request before initialize: id 1, <nil>; want -32601" and "the hooks
  never saw the notification sent before initialize".
* `b14-nil-gate` (the hooks run with no gate): "a request before initialize: id 1, <nil>; want -32601" and the handler running for `_x/echo` and
  `_x/early`, the early notification having reached the handler, where `_x/echo` and `_x/late` were wanted.

The phase 2 probes, re-run on the step 6 tree: `TestP2_B14_ExtensionSkipsHooksAndGate` now passes, `_x/ping` answered `-32601`
before `initialize` as `session/list` is; B2's, B16's, B17's and B19's pass; B18's is exempt (step 4); B20's fails, its step being
later, and B21's is text only.

Gates, through the Python runner: regenerating changes no generated file; `make pre-add-check` exit 0; no new `.go` file;
`GOEXPERIMENT=nojsonv2 go test ./...` exit 0; `GOOS=windows go vet` exit 0 in both modules; `make check-records` and `make
markdownlint` exit 0; the identifier scan finds nothing; the API listing against `v0.17.1` is unchanged from step 5, 2420 lines,
the nine expected entries, since `inboundGate` is unexported. The generator module's suite under `-race`: ok. Windows: `go test
-count=1 -shuffle=on ./...` exit 0 at the root and in `cmd/generate`.

Deviation, 2026-10-13: three existing tests in `acp_test.go` send an extension message before `initialize`, which decision 7 now
refuses. `TestExtensionMethods_ClientToAgentRequest` and `TestExtensionMethods_AgentToClientRequest` failed with `-32601`
"_vendor.test/echo: acp: initialize has not completed", and `TestExtensionMethods_UnknownNotification_DoesNotLog` timed out, its
notification dropped before the handler. Decision (owner): each test completes a real `Initialize` from the client over its pipe
pair before the extension call, as the other pair tests in that file do, with every assertion kept; the agent-to-client test keeps
the client connection it discarded. A fourth, `TestExtensionMethods_UnknownRequest_ReturnsMethodNotFound`, still passed, but its `-32601` came from the gate rather than from the agent that lacks `HandleExtensionMethod`, so it no longer tested what it names; the owner chose the same change for it. `acp_test.go` joins step 6's scope. The MADR is unchanged: decision 7 names this change.

### Step 7, an undeclared elicitation mode refused both ways, 2026-10-13

What was built:

* Model, `cmd/generate/internal/model`: the gate kind `GateUndeclaredVariant` (`protocol.go`), whose rule names a definition and a
  capability prefix and takes no property or variant; `checkCapabilityRules` requires exactly that. `gateCheck` resolves it
  (`gates.go`, `catchAll`) to a `GateCheckUndeclaredVariant` with the catch-all's Go field and, in the new `GateCheck.Disc`, the Go
  field of the catch-all's discriminator, a string; a union without a catch-all, or a catch-all without a string discriminator, is
  refused.
* Table, `acp.go`: `{Side: client, Definition: CreateElicitationRequest, Capability: "elicitation", Kind: GateUndeclaredVariant}`,
  its source `srcElicitation`, the sentence the form and url rules already cite; the definition's name is now the constant
  `defElicitation`, which `golangci-lint`'s `goconst` asked for once it occurred three times.
* Emitter, `emit/gates.go`: `if u.Other != nil` returns `&CapabilityError{Method: method, Capability: "elicitation." +
  u.Other.Mode, Variant: u.Other.Mode}`, so the message reads "capability elicitation.telepathy not advertised: variant telepathy".
  Regenerated: `types_gen.go` gains those seven lines in `(*CreateElicitationRequest).gates` and nothing else. Sending is refused;
  received, `inboundError` answers `-32602`, as for every value gate, and the validation hook can accept it.

Tests added or changed:

* `gates_test.go`: `TestUndeclaredElicitationMode`. With every elicitation capability advertised, `CreateElicitation` in mode
  `telepathy` returns that `*CapabilityError` and writes nothing; received, it is answered `-32602` without reaching the handler;
  with a validation hook that returns nil, it is handled. Its send runs under a 2 s deadline, so a missing gate fails it instead of
  waiting for an answer the idle peer never sends, which the first run of the first plant below showed: it hung until `go test`'s
  timeout. `valueGates`' comment names this test beside the terminal-auth rule's; `TestValueGates` keeps its 19 gates, since nothing
  advertises an undeclared mode and so it has no with-capability half.
* `cmd/generate/internal/model/gates_test.go`: `TestCapabilityRulesApply` resolves the kind on a union with a catch-all, to its
  `Other` field and `Kind` discriminator; `TestCapabilityRulesRefuse` gains the kind with a variant, without a capability, and on a
  union without a catch-all; `checkCommittedGates` expects 22 checks from 19 rules and the elicitation catch-all's on `Mode`.
* Engine case `accept-capability-undeclared-variant.txtar`, its `want.api` written by `-update` and read: a client-side
  `CreateElicitationRequest` with form, url and a catch-all, and a behaviour test, `TestUndeclaredModeRefused`, over a real pair
  with both modes advertised: `telepathy` refused on send, a raw send answered `-32602`, and `form` handled. The behaviour tests run
  under `TestEngineBehaviour`, not `TestEngineCases`; the case's ran and passed.

`go test -race -count=20` of `TestUndeclaredElicitationMode` and `TestValueGates`: ok.

Seen to fail, each in its own scratch copy (3 of 3):

* `b20-drop-emitted-check` (the emitter writes no check, regenerated): `TestUndeclaredElicitationMode`, "send = …context deadline
  exceeded…, want *CapabilityError{elicitation/create, elicitation.telepathy, Variant telepathy}", "99 bytes written, want none",
  "received: <nil>, handler ran true, want -32602 and no handler".
* `b20-drop-emitted-check-engine` (the same edit, under `TestEngineBehaviour`): "CreateElicitation(telepathy) = <nil>". Its first
  run filtered `TestEngineCases`, which runs no behaviour test, and passed; the plant was corrected and re-run.
* `b20-no-catch-all-accepted` (the kind resolves to nothing on a union without a catch-all): `TestCapabilityRulesRefuse`, "Build =
  <nil>, want an error containing \"Verdict is not a union with a catch-all variant\"".

The amendment of step 3, per the deviation below: `go test -race -count=400 -run '^TestValueGates$'` on the step 7 tree, 0
failures, against 8 on `a49d0bb`.

The phase 2 probes, re-run on the step 7 tree: `TestP2_B20_UndeclaredModeBothWays` now passes, "sent: elicitation/create: capability
elicitation.telepathy not advertised: variant telepathy; received: -32602"; B2's, B14's, B16's, B17's and B19's pass; B18's is
exempt (step 4); B21's is text only.

Gates, through the Python runner: regenerating changes no generated file; `make pre-add-check` exit 0, after the `goconst` finding
above; no new `.go` file; `GOEXPERIMENT=nojsonv2 go test ./...` exit 0; `GOOS=windows go vet` exit 0 in both modules; `make
check-records` and `make markdownlint` exit 0; the identifier scan finds nothing; the API listing against `v0.17.1` is unchanged
from step 5, the nine expected entries. The generator module's suite under `-race`: ok. Windows: `go test -count=1 -shuffle=on
./...` exit 0 at the root and in `cmd/generate`.

Deviation, 2026-10-13, an amendment of step 3 (B16): the step 7 run of `TestValueGates` failed once,
`session.notices_notice`, "send with the capability: {"code":-32800,…"context canceled"}". Measured with `go test -race
-count=400 -run '^TestValueGates$'` in scratch copies: 8 failures on `a49d0bb`, every one a notification's send, and none on
`29778f3`, before step 3. Two causes: in `sendMessage`, once the write has finished and the context has ended, both cases of the
second `select` are ready and Go picks one at random, so a message that was written can be reported cancelled; and the test
cancels as soon as it sees bytes, which can fall between the write and its report, where step 3's documented behaviour lets the
context's error win. Decision (owner): `sendMessage` takes a write that has finished before it returns the context's cause; and
`TestValueGates` waits for a notification's send, which returns once written, before it cancels, while a request is cancelled
after its bytes are seen, as before. `connection.go` and `gates_test.go`'s `TestValueGates` join step 7's scope. The MADR is
unchanged: decision 4's behaviour is as it states.

### Step 8, identity `MarshalJSON` methods not emitted, 2026-10-14

What was built:

* Generator, `cmd/generate/internal/emit/types.go`: `emitObjectCodec` calls `emitObjectMarshal` only when the method adds a
  statement: a default it injects, a fill, an `Extra`, or members. `injectsDefault`, a nilable array or object default, is the test
  `emitObjectMarshal`'s loop already applied, now named and shared. Regenerated: `types_gen.go` loses the seven methods, 49 lines,
  at the lines "Facts" names (`AgentCapabilities`, `AuthCapabilities`, `ClientCapabilities`, `FileSystemCapabilities`,
  `InitializeRequest`, `McpCapabilities`, `PromptCapabilities`), and changes nothing else; none of the seven had an
  `UnmarshalJSON` to lose. No identity `MarshalJSON` remains.

The byte proof, a scratch test run in two scratch copies, `c28159c` and the step 8 tree: `json.Marshal` of every generated schema
type's zero value and of every instance the conformance builder makes, decoded, each through a pointer and through a value, 1528
encodings, 28 of them of the seven types, 7 of those zero values. Before and after are identical byte for byte; no instance failed
to decode. The comparison was seen to fail: with the guard also dropping a type with fills, regenerated in the after copy, 23
encodings differed, `{"availableCommands":[]}` becoming `{"availableCommands":null}` and the like. `TestConformanceRoundTrip`
passes, and no JSON golden changed.

The phase 2 probes are unchanged from step 7: none concerns I8.

Gates, through the Python runner: regenerating changes no generated file; `make pre-add-check` exit 0; no new `.go` file;
`GOEXPERIMENT=nojsonv2 go test ./...` exit 0; `GOOS=windows go vet` exit 0 in both modules; `make check-records` and `make
markdownlint` exit 0; the identifier scan finds nothing; the API listing against `v0.17.1`, 2411 and 2413 lines, removes exactly
the seven `MarshalJSON` methods and adds exactly the nine entries of steps 2 to 5, the difference "Verification" expects for the
phase. The generator module's suite under `-race`: ok. Windows: `go test -count=1 -shuffle=on ./...` exit 0 at the root and in
`cmd/generate`.

Deviation, 2026-10-14: the generator change also removes identity `MarshalJSON` methods from six engine cases' `want.api`
goldens, which "Design", I8, does not name: `TestEngineCases` failed with "API differs from want.api" in
`accept-agent-optional`, `accept-allof-property`, `accept-default-object`, `accept-default-on-error`, `accept-default-scalar` and
`accept-folded-keys`. Measured in a scratch copy before choosing: `go test -run TestEngineCases . -update` removes 8 lines, each a
`func (T).MarshalJSON() ([]byte, error)`, and changes nothing else, and the generator suite, behaviour tests included, then
passes. Decision (owner): accept the goldens; the six files join step 8's scope. The MADR is unchanged.
