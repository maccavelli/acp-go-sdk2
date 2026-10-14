---
status: proposed
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

Phase acceptance: every phase 2 finding has a test or a recorded check; the phase 2 probes pass, B21's being text only; the phase 1
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

Nothing executed yet.
