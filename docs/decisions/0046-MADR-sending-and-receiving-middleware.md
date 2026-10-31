---
status: proposed
date: 2026-10-31
decision-makers: the repository owner
consulted: the MCP Go SDK, gRPC-Go, connect-go, golang.org/x/tools jsonrpc2 and jsonrpc2_v2, sourcegraph/jsonrpc2, the ACP TypeScript, Rust and Python SDKs, coder/acp-go-sdk, and the OpenTelemetry RPC semantic conventions, at the revisions under "Sources"
informed: gobble-cli, magic-cli-remote
---

# Add sending and receiving middleware around the connection's method handler, and an observer for the messages it does not see

## Context and Problem Statement

Two records deferred general middleware to "its own record".

* [0015-MADR-validate-absolute-protocol-paths.md](0015-MADR-validate-absolute-protocol-paths.md) found that "There is no hook
  around sending or receiving for tracing, metrics, authorization or rewriting". It named the MCP Go SDK's
  `type Middleware func(MethodHandler) MethodHandler` as the closest Go precedent. It rejected deciding it there (option X3),
  because "it is a decision about every message, with its own order and error semantics".
* [0017-PLAN-remediate-acp-v1-conformance-violations.md](0017-PLAN-remediate-acp-v1-conformance-violations.md) left "The
  middleware record 0015 names" out of its scope.

No record was written. The SDK has two inbound hooks instead, `WithInboundRewrite` and `WithInboundValidation`. They are typed,
they run inside the generated dispatch after decoding, and they cannot see results, outbound messages or timing. A consumer that
wants a span per request, a metric per method, or an authorization check before a handler runs must today write its own
`MethodHandler` around the SDK's, which only `NewConnection` accepts, or tap the transport's bytes.

The question: should the SDK offer middleware, and if so, of what shape, in which directions, where it sits relative to the
hooks, the queue and the barrier, and how it is ordered?

## Findings

At `9fcbb6f` (`v1.29.0`), and in the sources under "Sources".

* **F1. The SDK's seams.**
  * **Receiving.** Every request and notification that reaches a handler goes through one `MethodHandler`,
    `func(ctx, method string, params json.RawMessage) (any, *RequestError)` (`connection.go:327`).
    * `handleInbound` calls it (`connection.go:1988`) and encodes the result after it returns (`:2014`).
    * `NewConnection` takes it directly. `NewAgentSideConnection` and `NewClientSideConnection` pass the generated dispatch.
      All three build through `buildConnection(handler, …)` (`:753`).
  * **Sending.** Outbound calls pass through `SendRequest[T]`, `SendRequestNoResult` and `SendNotification`, and the generated
    typed methods and `CallExtension` call these. Params are encoded and validated in `encodeParams` (`:2341`).
* **F2. What never reaches the handler.** The reader handles these itself:
  * parse and invalid-request faults (`connection.go:1517-1538`);
  * `$/cancel_request` (`:1556`);
  * requests refused over the concurrency limit (`:1565-1574`) or during shutdown (`:1591-1598`);
  * responses (`:1529`).
* **F3. Where handlers run.** A request runs on its own goroutine (`:1619`). Notifications run one at a time from the queue
  (`processNotifications`, `:1823`), and under `OverflowBlock` a slow notification handler pauses the reader
  ([0044-MADR-notification-queue-back-pressure.md](0044-MADR-notification-queue-back-pressure.md), D7). A panic in a handler
  ends the process (`connection.go:320-322`).
* **F4. The hooks are typed and inside the dispatch.**
  * `judgeInbound` runs the rewrite hook, validation with the initialize gate, then the validation hook
    (`connection.go:2687-2712`).
  * It is called from each generated method (`agent_gen.go:244` and on; `client_gen.go:156` and on), and for responses in
    `sendRequestSent` (`:2128`).
  * `ChainRewrites` composes several rewrite hooks in order (`:680`).
* **F5. The MCP Go SDK.** `v1.8.0` has `type Middleware func(MethodHandler) MethodHandler`, where its `MethodHandler` is
  `func(ctx, method string, req Request) (Result, error)`, and `AddSendingMiddleware` and `AddReceivingMiddleware`
  (`mcp/shared.go:112-137`).
  * Its middleware is **typed**: it runs "after the request has been parsed" and is "an alternative to transport middleware
    that exposes MCP types instead of raw JSON-RPC" (`design/design.md:405-428`). Raw access is a separate `LoggingTransport`.
  * It can short-circuit and rewrite, and sees results and errors, for notifications too.
  * `addMiddleware` wraps in reverse (`shared.go:133-137`), so each later `Add*` call wraps **outside** the earlier ones. Issue
    #688 reports the order surprising its users.
  * Other issues found gaps: #1225 (a short-circuit loses work done inside), #1305 and #1479 (inputs that bypassed receiving
    middleware), #1265, open (the caller's context does not reach sending middleware), #64 and #818 (middleware cannot see a
    tool error the handler turned into a result).
* **F6. Other Go RPC interceptors.**
  * **gRPC-Go `v1.82.0`:** unary and stream interceptors, client and server, over decoded messages as `any`.
    `ChainUnaryInterceptor` makes the first-listed outermost (`interceptor.go:43`, `:81-108`; `server.go:1225-1260`).
  * **connect-go `v1.21.1`:** one `Interceptor` type for client and handler, the first outermost, repeated options appending
    (`interceptor.go:29-116`; `option.go:139-183`, `:337-385`). It warns that a request refused by its gate "never reach[es]
    the interceptor chain, logging or metrics interceptors will not observe them" (`option.go:170-173`).
  * **golang.org/x/tools jsonrpc2 and jsonrpc2_v2:** raw handler chains, receive only, with a `Preempter` before the queue
    that "must not block" (`jsonrpc2_v2/jsonrpc2.go:38-90`).
  * **sourcegraph/jsonrpc2:** observe-only `OnRecv` and `OnSend`. A handler "While it is running, the connection does not
    process any other requests or responses" (`jsonrpc2.go:61-68`).
* **F7. The ACP SDKs.**
  * **TypeScript (`f0981eb`) and Rust (`fe1e126`):** receive-side handler chains that claim a message or pass it on. They cannot
    see its result, and have no sending side (`src/core/jsonrpc.ts:345-400`; `jsonrpc.rs:340-400`). Both warn that handlers
    must not block.
  * **Python (`9853d8b`):** observers in both directions over raw dictionaries, deep-copied, with optional OpenTelemetry spans
    (`connection.py:28-39`, `:116-199`; `telemetry.py`).
  * **coder/acp-go-sdk (`b60322e`):** none.
* **F8. Tracing needs little.**
  * The OpenTelemetry RPC conventions at `semantic-conventions@026271d` (`docs/rpc/json-rpc.md:44-56`, status Development) want:
    * `rpc.system.name="jsonrpc"`, Required;
    * the status code, Conditionally Required;
    * `jsonrpc.request.id`, Recommended;
    * `rpc.method` as Opt-In, since method names are unbounded.
  * Older names such as `rpc.jsonrpc.request_id` are deprecated.
  * Trace context travels in `params._meta` as `traceparent`, `tracestate` and `baggage`. ACP reserves exactly those root keys
    (extensibility page; RFD `meta-propagation`), and the OpenTelemetry MCP convention does the same.
  * The SDK's `TraceMeta` builds such a `_meta` (`extensions.go:143-155`), but nothing propagates it.
  * MCP declined to build OpenTelemetry into its SDK (#59). This SDK has no dependencies, by rule.
* **F9. Costs and hazards recorded elsewhere.** Each layer allocates per call: gRPC allocates a closure per interceptor, MCP takes
  a mutex per message, and Python deep-copies. No benchmark was found for any of them. No Go library surveyed recovers panics in
  its chain by default; connect needs `WithRecover`.

## Decision Drivers

* Tracing, metrics, logging and authorization should be possible without forking the dispatch or reimplementing
  `MethodHandler`.
* One mental model for both directions, and the same type the SDK already exports.
* A clear, documented order, with no surprise when options are repeated (F5's issue #688).
* No cost when no middleware is registered.
* 0044's guarantees hold: the barrier, the notification order, and the rule for notification handlers.
* No dependency: OpenTelemetry stays the consumer's, and the SDK gives it what it needs (F8).

## Considered Options

* **Close the deferral.** Document the hooks, `NewConnection` and the taps as the answer.
* **Receiving middleware only**, wrapping the connection's `MethodHandler`.
* **Sending and receiving middleware, raw, one type**, `Middleware func(MethodHandler) MethodHandler`, registered by option.
* **Typed middleware**, MCP's shape, over decoded params and results.
* **An observer only**, stats-handler style: sees every frame, including F2's, and cannot change any.

## Decision Outcome

Chosen option (proposed): **"Sending and receiving middleware, raw, one type"**.

It answers 0015's need in the SDK's existing vocabulary. `MethodHandler` is already exported and raw, and a middleware is a
function from one to another. It covers tracing (F8 needs only the method, the id and `_meta`), metrics, logging, and authorization
by method or by a field the middleware decodes. It costs nothing when unused. Typed middleware would duplicate the hooks' place
inside the dispatch. An observer alone cannot authorize or rewrite. With the owner's answer to Q2, an observer is added beside
the chain (D9), reporting the messages the chain does not see.

The parts of the decision:

* **D1. The type.** `type Middleware func(MethodHandler) MethodHandler`. Middleware returns the handler it wraps, or a new one.
* **D2. Registration, at construction only.**
  * `WithReceivingMiddleware(mw ...Middleware)` and `WithSendingMiddleware(mw ...Middleware)` are `ConnectionOption`s, so all
    three constructors take them.
  * Repeated options append. The first registered is outermost, as in gRPC and connect, not MCP (F5, F6).
  * There is no registration on a live connection: reading starts inside the constructor, as `WithLogger`'s doc already says.
* **D3. Receiving.**
  * The chain wraps the connection's `MethodHandler` once, in `buildConnection`. A `nil` handler is wrapped as the method-not-found
    answer it gives today.
  * It runs where the handler runs: on the request's goroutine, or on the notification worker.
  * It is outside the generated dispatch. It sees the raw params before decoding, the rewrite and validation hooks and the
    initialize gate, and it sees the result as the handler returns it, before encoding.
  * A middleware that returns without calling the next handler answers in its place. The SDK's validation of params and results
    then does not run for that message. That is documented, since a refusal for authorization is the main use.
* **D4. Sending.**
  * The chain wraps the step from encoded params to the raw result, for `SendRequest[T]`, `SendRequestNoResult`,
    `SendNotification`, `CallExtension`, `NotifyExtension` and the generated methods.
  * It sees the method and the params as `json.RawMessage` after `encodeParams` has validated them, and returns the result as
    `json.RawMessage`, which `SendRequest[T]` then decodes. For a notification, both returns are nil.
  * The response barrier and the connection's waits stay inside the innermost handler, so a middleware's timing includes them.
  * The SDK's own `$/cancel_request`, sent when a caller's context ends, does not pass the chain (F2's inbound twin).
  * A middleware may replace the params, for example adding `traceparent` to `_meta`. The bytes it passes on are sent as they are.
* **D5. The request id.** `RequestID(ctx) (json.RawMessage, bool)` reports a request's id to middleware on both sides. On the
  sending side the id is allocated before the chain runs. The PLAN's first step checks that allocating it earlier changes nothing
  else. If it would, the sending side reports no id, and this record is amended.
* **D6. What the chain does not see.** F2's messages are outside it: faults, `$/cancel_request`, refusals by limit or shutdown, and
  responses as frames. A response is seen as its sending call's result. They are documented as outside the chain, as connect
  documents its gate, and reported to the observer (D9; Q2 b, answered 2026-10-31).
* **D7. The rules middleware inherits.**
  * A receiving middleware around a notification runs on the worker. Under `OverflowBlock` it must not wait for anything the
    peer sends later (0044 D7).
  * A panic ends the process, as in a handler; nothing recovers it.
  * Middleware must not keep the params after it returns.
* **D8. Helpers, none now.** No OpenTelemetry integration, by the no-dependency rule (F8). `TraceMeta` already builds the `_meta`
  value. An example shows a tracing middleware with the standard library only (`log/slog`, timing, `RequestID`).
* **D9. An observer for what the chain does not see** (Q2 b, answered 2026-10-31).
  * `WithObserver(fn func(ctx context.Context, o Observation))` is a `ConnectionOption`, set at construction. A later call
    replaces an earlier one, as `WithLogger` does.
  * `Observation` is a struct of plain values:
    * `Kind`: an `ObservationKind`;
    * `Method`: when the message names one;
    * `ID`: a `json.RawMessage`, a copy, when the message carries one;
    * `Size`: the line's length in bytes;
    * `Err`: the `*RequestError` the connection answered, or the one a response carried, or nil.
  * The kinds, each reported once, where it happens:
    * `ObservedFault`: a parse error, a non-object, an invalid request or a line over `WithMaxMessageSize`
      (`connection.go:1389-1393`, `:1517-1548`), with the answer sent;
    * `ObservedCancelReceived`: an inbound `$/cancel_request` (`:1556`);
    * `ObservedCancelSent`: the SDK's own `$/cancel_request`, sent when a caller's context ends;
    * `ObservedRefused`: a request answered without its handler, over the concurrency limit (`:1565-1574`), during shutdown
      (`:1591-1598`), or past
      [0045-MADR-notification-queue-byte-budget.md](0045-MADR-notification-queue-byte-budget.md)'s budget, with the answer;
    * `ObservedResponse`: a response read, matched or not, with its error if it carried one;
    * `ObservedDropped`: a notification dropped under `OverflowDropNewest`, or the one whose overflow closed the connection.
  * The observer runs synchronously on the goroutine that met the message, the reader for all but `ObservedCancelSent`. It must
    not block, as jsonrpc2_v2's `Preempter` "must not block" (F6). A slow observer stalls reading. A panic ends the process.
  * It sees no message the middleware chain sees, so the two together cover every message once. It cannot change or stop
    anything.
  * Unset, it costs one nil check at each site.

### Questions for the owner

* **Q1. Scope.**
  * **(a) Both directions.** Recommended. Tracing a call needs both ends, and the outbound side is where `traceparent` is set.
  * **(b) Receiving only.** Smaller. The sending side waits for a consumer to ask.
  * **(c) An observer only.** F6's sourcegraph and F7's Python model, and OpenTelemetry's preferred shape for gRPC: observing
    every frame, F2's included, but not authorizing or rewriting.
* **Q2. F2's messages.**
  * **(a) Outside the chain, documented.** Recommended. Faults and refusals have no handler to wrap.
  * **(b) An observer option as well**, reporting each with its reason, for metrics that must count refusals. A second
    mechanism.

#### Answered by the owner, 2026-10-31

* Q1: **(a) Both directions**, as recommended.
* Q2: **(b) An observer option as well**, not the recommended (a). D9 states its design, and D6 points to it.

The decision stays `proposed` until the owner accepts it.

### Consequences

* Good, because tracing, metrics, logging and authorization by method can be added in a few lines, around every message that
  reaches a handler and every call the application sends.
* Good, because the type is the one the SDK already exports, so a `MethodHandler` written for `NewConnection` composes with it.
* Good, because a connection with no middleware runs exactly as now: the chain is built once, and only if registered.
* Neutral, because each registered middleware adds a function call per message. A closure per call, as in gRPC, is avoided by
  building the chain once.
* Bad, because a receiving middleware that answers in the handler's place skips the SDK's validation of that message. It is
  documented, and the hooks remain for validation.
* Bad, because raw params mean that a middleware authorizing on a field decodes the params itself. The typed hooks remain for
  that, inside the dispatch.
* Good, because with the observer every message is seen once: through the chain, or by the observer with its reason. Metrics can
  count refusals, faults, drops and cancels.
* Bad, because the observer is a second mechanism to document and test. It runs on the reader, so a slow observer stalls reading.
  The godoc says it must not block.

### Confirmation

The PLAN, [0046-PLAN-sending-and-receiving-middleware.md](0046-PLAN-sending-and-receiving-middleware.md), confirms each of these.
Each new test is seen to fail on a planted break in a scratch copy.

1. Receiving middleware wraps requests and notifications, in registration order, first outermost, across repeated options.
2. A receiving middleware that returns an error answers the request with it, and the handler does not run.
3. A receiving middleware sees the result before encoding, and the raw params before the hooks; a rewrite it passes on is what
   the hooks and the handler see.
4. Sending middleware wraps requests and notifications from every sending path, sees the encoded params and the raw result, and
   can add `_meta`.
5. `RequestID` reports the id on both sides (D5).
6. The response barrier, notification order and 0044's guard hold with middleware registered; the existing tests pass unchanged.
7. The observer reports each kind of D9 once, with its method, id, size and error. It sees no message the chain sees, and is
   never called when unset.
8. With no middleware and no observer, the benchmark of a request round trip shows no change beyond noise.
9. `make api`: compatible additions only.
10. `tools/gates.py --all` and CI pass.

## Pros and Cons of the Options

### Close the deferral

* Good, because nothing is added.
* Bad, because `NewConnection` is the only way to wrap the handler, and the side connections, which most consumers use, cannot.
  Nothing covers outbound calls.

### Receiving middleware only

* Good, because it is half the work, and covers authorization and inbound metrics.
* Bad, because tracing a call, and setting `traceparent` on what is sent, need the sending side.

### Sending and receiving middleware, raw, one type

* Good, because it is one type, the SDK's existing one, and one order rule, for both directions.
* Good, because raw params cover extension methods the same way as the standard methods.
* Bad, because authorization on a field decodes the params a second time.

### Typed middleware

* Good, because it gives field access without decoding twice, as MCP's does.
* Bad, because it needs the typed values per method in the generated code: a large generator change.
* Bad, because it duplicates the hooks' place, and MCP's issues (F5) show what a short-circuit inside the typed layer loses.

### An observer only

* Good, because it sees every frame, refusals included, and cannot break the dispatch.
* Bad, because it cannot authorize, rewrite or add `_meta`, so 0015's needs stay unmet.

## More Information

* Related records: [0015-MADR-validate-absolute-protocol-paths.md](0015-MADR-validate-absolute-protocol-paths.md) (X3, and the
  finding "No middleware"); [0017-PLAN-remediate-acp-v1-conformance-violations.md](0017-PLAN-remediate-acp-v1-conformance-violations.md);
  [0019-MADR-remediate-debugging-pass-findings.md](0019-MADR-remediate-debugging-pass-findings.md), decision 7, the hooks on
  extension methods; [0044-MADR-notification-queue-back-pressure.md](0044-MADR-notification-queue-back-pressure.md), D7.
* Release: a minor. Added: the `Middleware` type, `WithReceivingMiddleware`, `WithSendingMiddleware`, `RequestID`, and the
  observer's `WithObserver`, `Observation` and `ObservationKind` with its constants.

### Sources

| Source | Revision | What was read |
| :--- | :--- | :--- |
| MCP Go SDK | `v1.8.0`; design doc; issues #59, #64, #688, #818, #1225, #1264, #1265, #1305, #1479; PR #1464 | `mcp/shared.go:112-137`, `:179-225`; `mcp/server.go:1836-1864`; `design/design.md:405-428` |
| gRPC-Go | `v1.82.0` | `interceptor.go:43`, `:81-108`; `server.go:485-503`, `:1225-1260`; `dialoptions.go:594-602` |
| connect-go | `v1.21.1` | `interceptor.go:29-116`; `option.go:139-183`, `:337-385` |
| golang.org/x/tools | `v0.52.0` | `internal/jsonrpc2/handler.go:17-100`; `internal/jsonrpc2_v2/jsonrpc2.go:38-90`, `conn.go:21-55` |
| sourcegraph/jsonrpc2 | `v0.2.3` | `conn_opt.go:16-32`; `jsonrpc2.go:61-68` |
| ACP TypeScript SDK | `f0981eb` | `src/core/jsonrpc.ts:345-400`, `:1105-1140` |
| ACP Rust SDK | `fe1e126` | `src/agent-client-protocol/src/jsonrpc.rs:340-400`, `:461` |
| ACP Python SDK | `9853d8b` | `src/acp/connection.py:28-39`, `:116-199`; `telemetry.py` |
| coder/acp-go-sdk | `b60322e` | `connection.go:91` |
| OpenTelemetry semantic conventions | `026271d` | `docs/rpc/json-rpc.md:44-56`, `:91-92`; `docs/registry/attributes/rpc.md:123-135` |
| ACP | `b4c72a8` | `docs/protocol/v1/extensibility.mdx:37-41`; `docs/rfds/meta-propagation.mdx` |
