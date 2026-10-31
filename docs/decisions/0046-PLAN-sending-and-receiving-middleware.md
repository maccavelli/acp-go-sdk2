---
status: proposed
date: 2026-10-31
associated-madr: "0046-MADR-sending-and-receiving-middleware.md"
---

# Implement 0046: sending and receiving middleware, and an observer

Associated MADR: [0046-MADR-sending-and-receiving-middleware.md](0046-MADR-sending-and-receiving-middleware.md)

## Goal

A connection runs registered middleware around every message that reaches its handler, and around every call the application
sends, and reports every other message to an observer, as the MADR's D1 to D9 state. The owner answered on 2026-10-31: both
directions (Q1 a), and an observer for the messages outside the chain (Q2 b).

## Scope

In scope:

* `connection.go`:
  * the `Middleware` type;
  * `WithReceivingMiddleware` and `WithSendingMiddleware`;
  * the receiving chain in `buildConnection`;
  * the sending chain between `encodeParams` and the exchange, for each sending path;
  * `RequestID` (D5);
  * the observer: `WithObserver`, `Observation` and `ObservationKind` with its six constants, and the report at each site D9
    lists (D9);
  * the godoc of `MethodHandler`, the two hooks and the sending functions.
* `extensions.go`: `CallExtension` and `NotifyExtension` reach the chain through the same sending functions.
* A new test file, `connection_middleware_test.go`, and a benchmark.
* `README.md`, a "Middleware" section; `docs/architecture.md`; `example_api_test.go`, `ExampleWithReceivingMiddleware` and
  `ExampleWithSendingMiddleware`.
* The release, on the owner's ask.

Out of scope:

* typed middleware;
* an OpenTelemetry integration or any dependency;
* recovering panics;
* middleware on F2's messages: they go to the observer instead (D9);
* an observer able to change or stop a message;
* registering middleware on a live connection;
* any change to the hooks.

## Implementation Steps

### Phase 1: the chains

1. **Grounding.** In a scratch copy, trace each sending path to one function from the encoded params to the result:
   `SendRequest[T]`, `SendRequestNoResult`, `SendNotification`, `sendRequestCommit` for `initialize`, `CallExtension` and
   `NotifyExtension`. Record the function, and where the request id is allocated relative to `pending` registration and the
   write.
   * If allocating the id before the chain changes the order of registration and write, or the barrier, record it. D5 then
     reports no id on the sending side, and the MADR is amended before step 4.
   * Record how `sendRequestCommit`'s held-notification path (0044's `beginPeerWait`) composes with a chain around it.
2. **The type and options.**
   * `type Middleware func(MethodHandler) MethodHandler`.
   * `connectionConfig.receiving` and `connectionConfig.sending`, `[]Middleware`, appended by each option call.
   * The chain is built once, first registered outermost: a loop from the last to the first, so `mw[0]` wraps everything
     after it.
3. **Receiving.** In `buildConnection`, when `receiving` is not empty, `handler = chain(receiving, handlerOrNotFound)`, where a
   `nil` handler becomes the function that answers `NewMethodNotFound` as `handleInbound` does today. `handleInbound` is
   unchanged; it calls `c.handler`.
4. **Sending.**
   * The innermost sending handler is the step from step 1: given the method and the encoded params, it performs the exchange
     or the notification write, and returns the raw result, or the `*RequestError` the peer answered.
   * Each sending path calls the built chain in its place when `sending` is not empty, and calls the step directly otherwise.
   * `SendRequest[T]` decodes what the chain returns.
   * The SDK's own `$/cancel_request` keeps its direct path (D4).
5. **`RequestID`.** A context key set before the chain on both sides: on receipt from the message's id, on sending from the id
   allocated per step 1. A notification has none.
6. **The observer (D9).**
   * `connectionConfig.observer`; `WithObserver` sets it, a later call replacing an earlier one.
   * `Observation` and `ObservationKind` with `ObservedFault`, `ObservedCancelReceived`, `ObservedCancelSent`,
     `ObservedRefused`, `ObservedResponse` and `ObservedDropped`.
   * One unexported `c.observe(ctx, Observation)` call, guarded by a nil check, at each site D9 lists:
     * the line too long and the four faults in `receive` and `receiveLine`;
     * the `$/cancel_request` branch;
     * the concurrency-limit and shutdown refusals, and 0045's byte refusal if 0045 has landed;
     * `receiveResponse`;
     * `dropNotification` and the overflow close;
     * the SDK's own `$/cancel_request` send.
   * `ID` is copied, so the observer may keep it.
   * If step 1 or this step finds a site where the method, id or answer is not known, record which fields stay empty there.
7. **Godoc.** `MethodHandler`, the hooks (they run inside the receiving chain's innermost handler), the sending functions, D6's
   list of what the chain does not see, and the observer: where it runs, that it must not block, and that it sees no message
   the chain sees.
8. **Tests**, in `connection_middleware_test.go`:
   * `TestReceivingMiddlewareOrder`: three middlewares over two option calls, outermost first, for a request and a notification.
   * `TestReceivingMiddlewareShortCircuit`: an error answers the request, and the handler and hooks do not run.
   * `TestReceivingMiddlewareSeesRawAndResult`: raw params before the rewrite hook; a rewrite it passes on is what the hook sees;
     the result before encoding.
   * `TestSendingMiddlewareAllPaths`: one test per sending path, with `_meta.traceparent` added by the middleware and read by
     the peer.
   * `TestSendingMiddlewareResult`: the raw result and a peer's error, as returned to the caller.
   * `TestRequestIDInMiddleware`: on both sides.
   * `TestMiddlewareKeepsOrdering`: the response barrier and notification order with middleware on both sides, in a `synctest`
     bubble.
   * `TestObserverReportsEachKind`, one case per kind, with method, id, size and error as D9 states, in a `synctest` bubble
     where a case waits.
   * `TestObserverAndChainDisjoint`: a run with middleware and an observer, every message counted once across the two.
   * `BenchmarkRoundTripNoMiddleware`, with neither middleware nor observer, against the same at `9fcbb6f`, recorded.
9. **Planted breaks**, each in a scratch copy:
   * the chain built in MCP's order (last outermost);
   * the chain skipped for notifications;
   * one sending path not routed through the chain;
   * the id unset on the sending side;
   * the short-circuit calling the handler anyway;
   * one observer site removed, such as the concurrency refusal;
   * the observer also called for a message that reaches the chain.
10. **Checks.** `go test -race ./...`; the new tests with `-count=3` on the Linux and Windows hosts through `tools/remote.py pkg`;
   `make pre-add-check`; `make api`, compatible additions only; `make check-records`, `make markdownlint` and
   `make disclosure-check`. Then the execution record, and staging.

If [0045-PLAN-notification-queue-byte-budget.md](0045-PLAN-notification-queue-byte-budget.md) lands after this phase, its byte
refusal gains the observer's `ObservedRefused` report there, and its execution record says so.

### Phase 2: documentation and examples

1. `README.md`, "Middleware":
   * the type and the two options;
   * the order rule;
   * where each chain sits relative to the hooks, the queue and the barrier;
   * what it does not see (D6);
   * the rules it inherits (D7);
   * a tracing sketch with `TraceMeta` and `RequestID`;
   * the observer: its kinds, where it runs, that it must not block;
   * a metrics sketch counting refusals.
2. `docs/architecture.md`: the two chains and the observer's sites in the message path.
3. Three examples with fixed output, each seen to fail with its option removed: `ExampleWithReceivingMiddleware`,
   `ExampleWithSendingMiddleware` and `ExampleWithObserver`.
4. The phase 1 checks again, then the record and staging.

### Phase 3: close

1. `tools/gates.py --all`, recorded.
2. After the owner pushes, `tools/watch_ci.py`.
3. A table of the MADR's ten Confirmation items.
4. The MADR's status and the index rows.

### Phase 4: release, on the owner's ask

A minor. Release notes in `docs/reports/0046-REPORT-release-notes-vX-Y-0.md`, then `tools/consumer.py` extended to register one
middleware each way, then the owner's tag and `tools/publication.py`. If 0045 is released first, this is the next minor.

## Verification

* Every test of step 1.8 passes under `-race`, and with `-count=3` on the Linux and Windows hosts, and was seen to fail on its
  planted break.
* The existing tests pass unchanged, the barrier, overflow and hook tests among them.
* The no-middleware benchmark shows no change beyond noise.
* `make api`: the additions only.
* `tools/gates.py --all` and CI pass.

## Rollout and Rollback

* **Rollout.** Additive: no behaviour changes for a connection without middleware.
* **Rollback before the release.** Revert the phase commits.
* **Rollback after the release.** The type, options and function stay, under the compatibility promise. A defect in a chain is
  fixed in a patch.
