---
status: accepted
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

F1 to F4 and F10 to F13 are measured at `523f319`, whose Go code is `v1.30.0`'s (`d65819a`). The later commits change records
and delete `communique.toml`. F5 to F9 are from the sources under "Sources". This revision re-grounded the first draft's
findings, whose line numbers were taken at `9fcbb6f`, before 0045 moved them (see "Revision").

* **F1. The SDK's seams.**
  * **Receiving.** Every request and notification that reaches a handler goes through one `MethodHandler`,
    `func(ctx, method string, params json.RawMessage) (any, *RequestError)` (`connection.go:333`).
    * `handleInbound` calls it (`connection.go:2136`) and encodes a request's result after it returns (`:2162`).
    * A `nil` handler is handled before the call: a request is answered `-32601`, and a notification is dropped without a log
      (`:2126-2134`).
    * `NewConnection` takes the handler directly (`:768`). `NewAgentSideConnection` and `NewClientSideConnection` pass their
      generated dispatch, `handleWithExtensions` (`agent_gen.go:49`; `client_gen.go:44`). All three build through
      `buildConnection` (`connection.go:802`).
  * **Sending.** Three internal paths carry every request and notification the application sends:
    * `exchange` (`connection.go:2397`) checks the context and the input, allocates the id and encodes the params
      (`prepareRequest`, `:2467`; `encodeParams`, `:2489`), registers the pending response (`:2413-2415`) before the write
      (`:2425`), and waits for the answer. Two functions call it:
      * `sendRequestSent` (`:2259`), behind `SendRequest[T]`, the generated typed requests (18 on the client side, `Prompt`
        among them, which reads whether the request was written; 10 on the agent side) and `CallExtension` (`extensions.go:232`, `:267`). After `exchange` it waits for the notifications before
        the answer (the barrier, `:2264`), then decodes and checks the result (`:2273-2283`);
      * `sendRequestCommit` (`:2303`), which only `Initialize` uses (`client_gen.go:537`).
    * `SendRequestNoResult` (`:2721`) repeats `exchange`'s steps itself. No generated method calls it.
    * `SendNotification` (`:2768`) checks the context, encodes the params (`prepareNotification`, `:2878`) and writes. The
      generated notifications (2 on the agent side, 9 on the client side) and `NotifyExtension` call it.
    * The generated typed methods check what `initialize` agreed before any of these (for example `Prompt`,
      `client_gen.go:1044-1047`).
  * **Errors.** Every error `exchange`, the barrier and `SendNotification` return is a `*RequestError` (`toReqErr`,
    `errors.go:386-402`). A local failure is kept as its cause, so `errors.Is(err, ErrConnectionClosed)` and the
    context's errors work through it (`errors.go:11-39`). A `*RequestError` can therefore carry every sending error.
* **F2. What never reaches the handler.** The reader handles these itself:
  * **faults**, each answered: a line over `WithMaxMessageSize` (`connection.go:1456-1458`), invalid JSON (`:1585-1589`), not a
    JSON-RPC object (`:1592-1596`), an invalid request (`:1601-1606`), a method that is not a string (`:1614-1617`);
  * **`$/cancel_request`** received (`:1624-1627`, `handleCancelRequest`, `:2089`);
  * **refusals**, each answered `-32603`: over the concurrency limit (`:1633-1642`), past 0045's byte budget (`:1645-1652`), or
    during shutdown (`:1669-1678`);
  * **responses** (`:1597-1600`, `receiveResponse`, `:1848`). A response with no usable id is logged and dropped
    (`:1849-1852`), and one that matches no pending request, such as a late answer, is logged at `Debug` (`:2063-2066`);
  * **notifications dropped** under `OverflowDropNewest` (`:1775-1781`, `dropNotification`, `:1149`), and the one whose
    overflow closes the connection (`:1783-1790`);
  * **the SDK's own `$/cancel_request`**, queued when a caller's context ends (`sendCancelRequest`, `:2533`) and written by
    `sendCancelRequests` (`:2508`) **through the public `SendNotification`** (`:2525`). It is the only message the SDK sends
    on its own: every other `SendNotification` and `sendRequest` call in the package is an application's typed or extension
    call.
* **F3. Where handlers run.**
  * A request runs on its own goroutine (`connection.go:1698`). Its concurrency slot and its bytes are released by `release`
    as the handler returns, before the answer is written (`:1703-1708`, `:2137-2139`).
  * Notifications run one at a time from the queue (`processNotifications`, `:1924`). Under `OverflowBlock` a slow notification
    handler pauses the reader ([0044-MADR-notification-queue-back-pressure.md](0044-MADR-notification-queue-back-pressure.md),
    D7).
  * A panic in a handler ends the process (`:326-328`).
* **F4. The hooks are typed and inside the dispatch.**
  * `judgeInbound` runs the rewrite hook, validation with the initialize gate, then the validation hook (`connection.go:2835-2860`).
    `checkInbound` counts its refusals in `RefusedMessages` (`:2825-2831`).
  * It is called from each generated method (`agent_gen.go:244` and on; `client_gen.go` likewise), and for a response in
    `sendRequestSent` (`:2276`) and in `sendRequestCommit`'s commit (`:2321`).
  * A handler's result is checked inside the dispatch before it is returned (`checkResult`, `:2791`).
  * `ChainRewrites` composes several rewrite hooks in order (`:729`), because a connection keeps only the last one.
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
  * The SDK's `TraceMeta` builds such a `_meta` (`extensions.go:147`), but nothing propagates it.
  * MCP declined to build OpenTelemetry into its SDK (#59). This SDK adds no dependency without a MADR (`AGENTS.md`).
* **F9. Costs and hazards recorded elsewhere.** Each layer allocates per call: gRPC allocates a closure per interceptor, MCP takes
  a mutex per message, and Python deep-copies. No benchmark was found for any of them. No Go library surveyed recovers panics in
  its chain by default; connect needs `WithRecover`.
* **F10. `initialize`'s result is recorded on the reader.** `sendRequestCommit`'s commit decodes, checks and records the result
  as the reader reads it, before the reader reads on and before the caller resumes (`handleResponse`, `connection.go:2079-2081`;
  [0030-MADR-record-initialize-on-arrival.md](0030-MADR-record-initialize-on-arrival.md), O1). A sending chain around
  `Initialize` sees a result that is already recorded.
* **F11. Contexts carry what the dispatch and the barrier need.**
  * `receiveLine` gives a request's context its receipt and a `messageMark{notification, seq}` (`connection.go:1656`), and
    `processNotifications` a notification's (`:1926`). The received id is not kept in the context. `IsNotification(ctx)` reports
    the mark (`:109`).
  * The barrier's guard against a notification handler waiting on itself (`waitNotificationsUpTo`, `:2650`;
    [0019-PLAN-behaviour-fixes.md](0019-PLAN-behaviour-fixes.md), B1) and 0044's peer wait (`exchange`, `:2420`) read the
    mark from the caller's context. A request sent under a context **not** derived from the running handler's waits for that
    handler, and deadlocks if the handler waits for it.
* **F12. Options: the last wins, and that is tested.** `ConnectionOption`'s doc says "Options apply in the order given, so the
  last of a repeated option wins" (`connection.go:484-491`). `TestOptionSemantics` requires it (`options_test.go:15`).
  `ChainRewrites` exists because of it.
* **F13. Probes and benchmarks in the tree.**
  * `json.Marshal` of an `anyMessage` whose `Params` is a `json.RawMessage` compacts it and refuses invalid JSON, on
    `go1.26.10`, on `go1.27.3`, and on `go1.27.3` with `GOEXPERIMENT=nojsonv2` (a scratch probe, 2026-10-31). A multi-line
    object was written on one line, and `{"a":` failed with "unexpected end of JSON input". The error names `json.RawMessage`,
    or `*jsontext.Value` under the default `go1.27.3` build. A scalar such as `"str"` **is** accepted, which JSON-RPC 2.0
    forbids for params and `encodeParams` refuses (`connection.go:2502-2503`).
  * `bench_test.go` has `BenchmarkPromptRoundTrip` (`:92`) and `BenchmarkSessionUpdateNotify` (`:105`), run by `make bench`,
    six times with allocations (`Makefile:122-124`).

## Decision Drivers

* Tracing, metrics, logging and authorization should be possible without forking the dispatch or reimplementing
  `MethodHandler`.
* One mental model for both directions, and the same type the SDK already exports.
* A clear, documented order, with no surprise when options are repeated (F5's issue #688).
* No cost when nothing is registered: no allocation and no lock on any message's path.
* 0044's guarantees hold: the barrier, the notification order, the rule for notification handlers, and 0030's record of
  `initialize`.
* Misuse fails loudly and at once, never as a hang or a corrupted exchange.
* No dependency: OpenTelemetry stays the consumer's, and the SDK gives it what it needs (F8).

## Considered Options

* **Close the deferral.** Document the hooks, `NewConnection` and the taps as the answer.
* **Receiving middleware only**, wrapping the connection's `MethodHandler`.
* **Sending and receiving middleware, raw, one type**, `Middleware func(MethodHandler) MethodHandler`, registered by option.
* **Typed middleware**, MCP's shape, over decoded params and results.
* **An observer only**, stats-handler style: sees every frame, including F2's, and cannot change any.

## Decision Outcome

Chosen option: **"Sending and receiving middleware, raw, one type"**.

It answers 0015's need in the SDK's existing vocabulary. `MethodHandler` is already exported and raw, and a middleware is a
function from one to another. On the sending side its `*RequestError` carries every error the exchange can return (F1, Errors).
It covers tracing (F8 needs only the method, the id and `_meta`), metrics, logging, and authorization by method or by a field the
middleware decodes. It costs nothing when unused. Typed middleware would duplicate the hooks' place inside the dispatch. An
observer alone cannot authorize or rewrite. With the owner's answer to Q2, an observer is added beside the chains (D9), reporting
the messages they do not see.

The parts of the decision:

* **D1. The type.** `type Middleware func(next MethodHandler) MethodHandler`. A middleware returns `next`, or a handler that
  calls it at most once, or answers in its place.
* **D2. Registration, at construction only.**
  * `WithReceivingMiddleware(mw ...Middleware)` and `WithSendingMiddleware(mw ...Middleware)` are `ConnectionOption`s, so all
    three constructors take them.
  * **Repeated options append**, the first registered outermost, as in gRPC and connect, not MCP (F5, F6):
    `WithReceivingMiddleware(a, b)` then `WithReceivingMiddleware(c)` runs `a`, then `b`, then `c`, then the handler. This is
    the one exception to F12's "last wins", stated in `ConnectionOption`'s doc and required by `TestOptionSemantics`, because a
    library that wraps a connection's options, such as `acptest` or a consumer's own helper, must not erase the application's
    middleware.
  * Each chain is built **once**, in `buildConnection`. Each `Middleware` is called once there, not per message. A `nil`
    middleware, or one that returns a `nil` handler, panics there, naming the option and the position, as a `nil` option
    already panics (F12).
  * There is no registration on a live connection: reading starts inside the constructor, as `WithLogger`'s doc already says.
* **D3. Receiving.**
  * The chain wraps the connection's `MethodHandler` once, and replaces it, so `handleInbound` is unchanged and a connection
    without receiving middleware runs exactly as now.
  * A `nil` handler under a chain is replaced by one that behaves as `handleInbound`'s `nil` branch (F1): `-32601` for a request,
    and `nil, nil` for a notification, so nothing new is logged.
  * It runs where the handler runs: on the request's goroutine, inside its concurrency slot and byte count (F3), or on the
    notification worker.
  * It is outside the generated dispatch. It sees the method and the raw params before decoding, the rewrite and validation
    hooks, the initialize gate and the hold for `initialize`. It sees the result as the dispatch returns it, after `checkResult`
    and before encoding. `params` is `nil` when the peer left them out or sent `null`.
  * It may pass `next` other params, or another method, to route the message. The dispatch then decodes and checks what it is
    given.
  * A middleware that returns without calling `next` answers in its place. The dispatch's validation, gate and result check
    then do not run for that message. That is documented, since a refusal for authorization is the main use. A result it
    returns is encoded as a handler's is.
  * `IsNotification(ctx)` tells a notification from a request, as it does for handlers.
* **D4. Sending.**
  * The chain wraps one innermost step, the same for every path: from the encoded params to the raw result. Every request and
    notification the application sends passes it: `SendRequest[T]`, `SendRequestNoResult`, `SendNotification`,
    `CallExtension`, `NotifyExtension` and every generated typed method, `Initialize` included.
  * **Before the chain**, as today and in today's order: the typed methods' gate, the context and input checks, the id's
    allocation, and `encodeParams`' validation and encoding. A message the SDK would refuse never reaches the chain.
  * **Inside the innermost step:** the context check again, for the context the chain passes; the pending registration, the
    write, the wait for the answer, and the barrier. A middleware's timing therefore includes the round trip and the barrier.
  * **After the chain:** `SendRequest[T]` decodes the result and runs `checkInbound` and the result check on it, as today. A
    sending middleware cannot make the SDK accept a response it would refuse.
  * The result is a `json.RawMessage` from the innermost step. A middleware may return another: a `json.RawMessage` is used as
    is, `nil` is no result, and any other value is encoded with `encoding/json`, as `handleInbound` encodes a handler's result.
  * The params a middleware passes on are written as given. A replacement must be `nil`, a JSON object or a JSON array (F13:
    `encoding/json` compacts it and refuses invalid JSON, but accepts a scalar).
  * `Initialize`'s result is recorded on the reader before the chain sees it (F10). A middleware may observe it and may fail
    the call, but a result different from the innermost step's fails the call with `ErrSendingMiddleware`, since the record
    cannot follow it. An error a middleware returns after a recorded `initialize` leaves the record in place; the godoc says so.
  * The SDK's own `$/cancel_request` keeps a direct path, outside the chain, and is reported to the observer (D9). That needs
    `sendCancelRequests` to stop calling the public `SendNotification` (F2).
  * `Prompt`'s `session/cancel` after a cancelled turn is the application's typed `Cancel`, and passes the chain.
* **D5. The request id: `RequestIDFromContext(ctx) (json.RawMessage, bool)`.**
  * The name follows Go's `FromContext` idiom, and avoids `RequestID` beside the generated `RequestId` type, which differ only
    in case.
  * It returns a copy of the id, and `false` for a notification or a context the connection did not make.
  * **Receiving:** the id from the request, `null` included. ~~It is kept in `messageMark` as a pointer to the message's existing
    `*json.RawMessage`, so no allocation is added and the value grows by one word (F11).~~ Handlers can read it too.

    *Amended 2026-10-31, in phase 1 (the PLAN's deviation 1):* it is kept **beside the request's receipt**, as a pointer to the
    message's existing `*json.RawMessage`. In `messageMark` it added one allocation per received request: a request's mark is
    the zero literal `messageMark{}`, which Go boxes from static data without allocating, and any mark holding an id
    allocates. The receipt's value allocates already. `BenchmarkPromptRoundTrip` went from 99 to 100 allocations with the id in
    the mark, and stays at 99 beside the receipt, with one word more per received request. A notification's receipt is
    unchanged.
  * **Sending:** the id the SDK allocated before the chain (D4). Allocating it there changes nothing else: the registration
    still precedes the write, and no answer can arrive before the write. A short-circuited request leaves a gap in the ids,
    which JSON-RPC allows.
  * The sending call's state shadows the received message's: inside a sending middleware for a notification, it returns
    `false`, even when the context derives from a received request's.
* **D6. What the chains do not see.** F2's messages are outside both chains: faults, `$/cancel_request` in both directions,
  refusals by limit, budget or shutdown, drops, and responses as frames. A matched response is seen as its sending call's result.
  Each is documented as outside the chain, as connect documents its gate, and reported to the observer (D9; Q2 b, answered
  2026-10-31).
* **D7. The rules middleware inherits.**
  * Middleware must be safe for concurrent use: requests run concurrently, and so do sending calls.
  * A receiving middleware around a notification runs on the worker. Under `OverflowBlock` it must not wait for anything the
    peer sends later (0044 D7).
  * `next` is called at most once, and before the middleware returns, with a context derived from the one it was given. On the
    sending side the context matters for the barrier's guard (F11), and the SDK enforces all three (D10).
  * `params` must not be changed in place or kept after the middleware returns; a change passes a new slice.
  * A panic ends the process, as in a handler; nothing recovers it.
* **D8. Helpers, none now.** No OpenTelemetry integration, by the no-dependency rule (F8). `TraceMeta` already builds the `_meta`
  value. An example shows a tracing middleware with the standard library only (`log/slog`, timing, `RequestIDFromContext`).
* **D9. An observer for what the chains do not see** (Q2 b, answered 2026-10-31).
  * `WithObserver(fn func(Observation))` is a `ConnectionOption`, set at construction. A later call replaces an earlier one, as
    `WithLogger` does. `nil` removes it.
  * `Observation` is a struct of plain values:
    * `Kind`: an `ObservationKind`, with a `String` method for logs;
    * `Method`: when the message names one and it was decoded;
    * `ID`: a copy, when the message carries one: the request's, the response's, or for a cancel the `requestId` it names;
    * `Size`: the received line's length in bytes, counted while a line over the limit is discarded; 0 for a sent message;
    * `Err`: the `*RequestError` the connection answered, the one a response carried, the send's error, or
      `ErrNotificationQueueOverflow`; `nil` otherwise.
  * The kinds, each reported once, where it happens (F2):
    * `ObservedFault`: each of the five faults, with the answer sent and its id, if valid. `Method` is empty, since the line
      is not trusted;
    * `ObservedCancelReceived`: an inbound `$/cancel_request`, with the id it names, or none when its params do not decode;
    * `ObservedCancelSent`: the SDK's own `$/cancel_request` once written, or with the write's error;
    * `ObservedRefused`: a request answered without its handler, over the concurrency limit, past the byte budget, or during
      shutdown, with the answer. These are not counted by `RefusedMessages`, which counts the hooks' and validation's refusals
      inside the chain (F4);
    * `ObservedUnmatchedResponse`: a response no pending request took, a late answer or an unknown id, or one without a usable
      id, with its error if it carried one. A matched response goes to its call (D6);
    * `ObservedDropped`: a notification dropped under `OverflowDropNewest`, beside the drop handler, or, with
      `ErrNotificationQueueOverflow`, the one whose overflow closed the connection.
  * The observer has no context argument. None of these messages has a handler's context, and a connection-wide one would
    mislead.
  * It runs synchronously on the goroutine that met the message: the reader for all kinds but `ObservedCancelSent`, which runs
    on the cancel sender. It must be safe for concurrent use and must not block, as jsonrpc2_v2's `Preempter` "must not block"
    (F6): a slow observer stalls reading. It is never called with a lock of the connection's held, so it may call the
    connection's accessors. A panic ends the process.
    *Amended 2026-11-02* (see "Amendment, 2026-11-02: where faults and refusals are observed" below): `ObservedFault` and
    `ObservedRefused` run on the goroutine that writes their answers, not on the reader.
  * It sees no message either chain sees: with both chains and the observer set, every message is seen once. It cannot change
    or stop anything.
  * *Amendment accepted 2026-11-01* (see "Amendment, 2026-11-01: when the observer runs" below): the godoc and the README
    state when the observer runs relative to what the connection does with the message, and a test pins it.
  * Unset, it costs one nil check at each site, and the id is not copied.
* **D10. Misuse fails at once.** On the sending side the innermost step refuses, sends nothing, and returns a `*RequestError`
  (`-32603`) wrapping the exported sentinel `ErrSendingMiddleware`, naming the rule broken:
  * `next` called a second time for one call;
  * `next` called with a context that does not carry the call, such as `context.Background()` (D7, F11);
  * another method passed to `next`: the gate, the result's type and the pending registration belong to the caller's;
  * replacement params that are not `nil`, an object or an array (D4);
  * a replaced result for `initialize` (D4, F10).
  The receiving side has no such state per message and does not enforce D7's rules; its godoc states them.
* **D11. Cost.**
  * Nothing registered: no allocation is added to any path, the receiving path is unchanged, and the sending paths add one nil
    check. `BenchmarkPromptRoundTrip` and `BenchmarkSessionUpdateNotify` must show the same allocations per operation, and
    bytes per operation up by at most one word per received request (D5).
  * ~~Sending middleware registered: the call's state and the context that carries it, two allocations per sent message, plus
    whatever the middleware allocates.~~ Receiving middleware registered: none added by the SDK. Both are measured by new
    benchmarks and recorded.

    *Amended 2026-10-31, in phase 5 (the PLAN's deviation 2):* sending middleware registered costs **three allocations per
    request sent** and two per notification, plus whatever the middleware allocates. They are the call's state, the context
    that carries it, and, for a request, the raw result boxed as the `any` a `MethodHandler` returns, which D4's contract, a
    `json.RawMessage` to the chain, requires. Measured:
    * `BenchmarkPromptRoundTripMiddleware`, one middleware each way on both sides, has 102 allocations per round trip against
      99, and 363 bytes more;
    * a notification sent through one, in a scratch copy, has 67 against 65.

    Escape analysis names the box (`resp.msg.Result escapes to heap`). The owner chose to keep D4's contract and state the
    cost, over folding the call into its context, or returning a `*json.RawMessage`.

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

*2026-10-31:* asked to accept the revised MADR and approve the PLAN before phase 1, the owner answered "committed, proceed".
The decision is `accepted`; D5 was amended the same day, in phase 1.

### Revision, 2026-10-31

On the owner's ask to assess the pair against the tree, before acceptance. Nothing in Q1 or Q2 changes. What changed, and why:

* **The findings were re-pinned** at `523f319`. 0045 had moved every `connection.go` line, and its byte-budget refusal, which
  the first draft named conditionally, is now a site. F10 to F13 are new.
* **D2:** the append rule is stated as the exception to F12, with its test. The chain's construction-time panics are new.
* **D3:** the `nil` handler's replacement now drops notifications silently. As first written, it would have answered `-32601`
  for every method, and `handleInbound` logs that at `Warn` for each notification.
* **D4:**
  * The innermost step's place in each of the three paths is fixed (F1).
  * The SDK's `$/cancel_request` needs its own path, since it goes through the public `SendNotification` today.
  * The result's encoding rule, the params rule and the `Initialize` rule are new (F10, F13).
* **D5:** `RequestID` became `RequestIDFromContext`. It returns a copy, its storage costs no allocation, the sending call
  shadows a received message, and the step that was to check the id's allocation is answered here.
* **D7:** the concurrency and `next` rules are new.
* **D9:**
  * `ObservedResponse`, "a response read, matched or not", contradicted "it sees no message the chain sees" for every matched
    response. It became `ObservedUnmatchedResponse`.
  * `Err` became an `error`, to carry `ErrNotificationQueueOverflow`.
  * The observer lost its `context.Context` argument.
  * The observer gained the concurrency rule and the guarantee that no lock is held while it runs.
* **D10 and D11** are new. D10 adds an exported sentinel, `ErrSendingMiddleware`.

### Amendment, 2026-11-01: when the observer runs

Accepted 2026-11-01 (below). The decision stays `accepted`.

**Found** in [0046-PLAN-sending-and-receiving-middleware.md](0046-PLAN-sending-and-receiving-middleware.md), step 7.1. The release's
consumer program counted refusals with an observer and printed the count once its refused call returned. One run in three
printed `observed map[]`: the client had read the refusal before the observer counted it.

**What the code does,** at `v1.31.0` (`093649b`), at each site:

* `ObservedFault` and `ObservedRefused`: `answerObserved` (`observer.go`) calls `answer`, which writes the error response
  synchronously through `sendMessage` (`connection.go:1957-1964`), then `observe`. The answer is written first.
* `ObservedCancelReceived`: `observe` takes the id `handleCancelRequest` returns (`connection.go:1691`), so the named request's
  context is cancelled first.
* `ObservedCancelSent`: `observe` follows `notifyDirect`'s write and carries its error (`connection.go:2638-2643`).
* `ObservedDropped` under `OverflowDropNewest`: `dropNotification` calls the drop handler, then `observe`
  (`connection.go:1210-1217`).
* `ObservedDropped` for the overflow that closes the connection: `observe`, then `closeWith` and `shutdownReceive`
  (`connection.go:1850-1856`). The observer runs before the connection closes.
* `ObservedUnmatchedResponse`: `observe` follows `handleResponse`'s refusal to take the response (`connection.go:1951-1953`), or
  the warning for a response without an id (`connection.go:1918-1925`).

Neither `WithObserver`'s godoc nor the README's "What the chains do not see" says any of this. D9 says only that the observer
runs "synchronously on the goroutine that met the message". So a reader cannot tell whether a count taken when the peer sees the
answer is complete, and a change of order would break no test: `TestObserverReportsEachKind` reads each observation after its
answer, whichever came first.

**Options:**

* **(a) State the order the code has, and pin it with a test** (recommended). The observer runs after the connection has acted on
  the message, with one exception, the overflow that closes the connection, where it runs before the close. A program that counts
  refusals waits for the observer, not for the answer. Cost: one paragraph in each of the godoc and the README, and one test.
* **(b) State that no order is promised.** It keeps the freedom to reorder, but it leaves the consumer's question unanswered,
  and every program that counts would have to synchronize as if the order could change.
* **(c) Observe before acting.** A count would then be complete when the answer is read. But `Err` is the answer, and would be
  reported before the write that can fail; a slow observer would delay every answer; and it changes the behaviour `v1.31.0`
  shipped.

**Proposed decision:** (a). It changes no behaviour, so it is not a change to the exported API, and `make api` stays clean. A
change of order later would have to amend this record and its test.

**Question for the owner, Q3. The release.**

* **(a) A patch, `v1.31.1`** (recommended): only godoc, the README and a test change. Consumers read godoc from the newest
  version, and a consumer counting refusals today has nothing to go on.
* **(b) The next minor release,** with whatever comes next.

#### Answered by the owner, 2026-11-01

Each question was put with three options; the owner chose the recommended one each time:

* The amendment: **(a), state the order the code has, and pin it with a test.** Not chosen: (b) no order promised; (c) observe
  before acting.
* Q3: **a patch, `v1.31.1`.** Not chosen: the next minor release; the same patch held until
  [0047-PLAN-pre-add-check-full-coverage.md](0047-PLAN-pre-add-check-full-coverage.md) lands.
* The order of work: 0047's PLAN first, then this amendment's phase, so it is checked by 0047's check.

The amendment stays proposed until the owner accepts it.

*2026-11-01:* with [0047-PLAN-pre-add-check-full-coverage.md](0047-PLAN-pre-add-check-full-coverage.md) complete, the owner
committed `a863ba1` and said "committed, proceed". The amendment is accepted; the PLAN's phase 8 runs it.

### Amendment, 2026-11-02: where faults and refusals are observed

Accepted with [0049-MADR-remediate-the-2026-11-01-debugging-pass.md](0049-MADR-remediate-the-2026-11-01-debugging-pass.md),
whose R1 and its design, "R1's and R2's designs, decided 2026-11-01", decide it; executed by
[0049-PLAN-runtime-fixes.md](0049-PLAN-runtime-fixes.md), phase 1. The decision stays `accepted`.

**Why.** The reader wrote its answers to faults and refusals itself. Two SDK peers that refused each other's requests each
blocked in a write the other's reader had to read, and neither read again (0049's R1). The reader now queues those answers for a
goroutine of their own, `answerLoop`, which writes them in order.

**What changes in D9.** `ObservedFault` and `ObservedRefused` are reported on that goroutine, after the answer is written, as
the order of the 2026-11-01 amendment requires. A fault or refusal met after the connection has ended, which cannot be answered,
is still reported on the reader. The other kinds run where they did. A slow observer of faults and refusals no longer stalls
reading; it delays the next such answer instead. The godoc of `WithObserver` and of the two kinds, and the README, say so.

**What holds.** Every message is still observed once, the answers left unwritten at the end included. The order of the
2026-11-01 amendment holds for every kind. `TestObserverNoLockHeld` waits for the refusal's report before sending the next
message, since the refusal and the next kind are now reported on different goroutines; no other observer test changed.

### Consequences

* Good, because tracing, metrics, logging and authorization by method can be added in a few lines, around every message that
  reaches a handler and every call the application sends.
* Good, because the type is the one the SDK already exports, so a `MethodHandler` written for `NewConnection` composes with it.
* Good, because a connection with no middleware and no observer runs as now, with no allocation added (D11).
* Good, because on the sending side every SDK check runs before the chain or after it, so middleware cannot send what the SDK
  would refuse, nor make it accept a response it would refuse.
* Good, because misuse on the sending side fails at once with a named rule (D10), where it would otherwise hang under the
  barrier or desynchronize a pending call.
* Neutral, because each registered middleware adds a function call per message, and sending middleware ~~two allocations per sent
  message~~ three allocations per request sent and two per notification (D11 as amended). The chain is built once, so there is
  no closure per call, as in gRPC.
* Bad, because a receiving middleware that answers in the handler's place skips the SDK's validation and result check of that
  message. It is documented, and the hooks remain for validation.
* Bad, because raw params mean that a middleware authorizing on a field decodes the params itself. The typed hooks remain for
  that, inside the dispatch.
* Bad, because `Initialize`'s result cannot be replaced by sending middleware (D4), an exception to the sending side's rule,
  forced by 0030's record.
* Good, because with the observer every message is seen once: through a chain, or by the observer with its reason. Metrics can
  count refusals, faults, drops, cancels and unmatched responses.
* Bad, because the observer is a second mechanism to document and test. It runs on the reader, so a slow observer stalls
  reading. The godoc says it must not block.

### Confirmation

The PLAN, [0046-PLAN-sending-and-receiving-middleware.md](0046-PLAN-sending-and-receiving-middleware.md), confirms each of these.
Each new test is seen to fail on a planted break in a scratch copy.

1. Receiving middleware wraps requests and notifications, in registration order, first outermost, across repeated options. A
   `nil` middleware or a `nil` handler it returns panics at construction. A `nil` handler under a chain answers requests `-32601`
   and drops notifications without a log.
2. A receiving middleware that returns an error answers the request with it, and the dispatch and handler do not run.
3. A receiving middleware sees the raw params before the hooks and the result after `checkResult`. Params it passes on are what
   the hooks and the handler see.
4. Sending middleware wraps every sending path, `Initialize` included, sees the encoded params and the raw result, and can add
   `_meta`. A replaced result is decoded and checked as the peer's would be. The SDK's own `$/cancel_request` does not pass it.
5. Each misuse of D10 fails with `ErrSendingMiddleware` and writes nothing.
6. `RequestIDFromContext` reports the id on both sides, `false` for notifications, and the sending call's id inside a received
   request's handler.
7. The response barrier, notification order, 0044's guard and 0030's record hold with middleware registered. The existing tests
   pass unchanged.
8. The observer reports each kind of D9 once, with its method, id, size and error, never under a lock. It sees no message the
   chains see, and is never called when unset.
9. With nothing registered, `BenchmarkPromptRoundTrip` and `BenchmarkSessionUpdateNotify` show the same allocations per
   operation, and no time change beyond noise (D11).
10. `make api`: compatible additions only.
11. `tools/gates.py --all` and CI pass.

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
* Bad, because the sending side needs rules a typed design would not (D4, D10), since raw bytes and one innermost step serve
  every path.

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
  extension methods; [0030-MADR-record-initialize-on-arrival.md](0030-MADR-record-initialize-on-arrival.md), O1;
  [0044-MADR-notification-queue-back-pressure.md](0044-MADR-notification-queue-back-pressure.md), D3 and D7;
  [0045-MADR-notification-queue-byte-budget.md](0045-MADR-notification-queue-byte-budget.md), D6.
* Release: a minor, `v1.31.0`. Added: the `Middleware` type, `WithReceivingMiddleware`, `WithSendingMiddleware`,
  `RequestIDFromContext`, `ErrSendingMiddleware`, and the observer's `WithObserver`, `Observation`, `ObservationKind` with its
  six constants and `String`.

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
| This repository | `523f319` (Go code as `d65819a`, `v1.30.0`) | the files and lines under F1 to F4 and F10 to F13 |
