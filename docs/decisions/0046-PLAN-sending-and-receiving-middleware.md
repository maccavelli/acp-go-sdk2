---
status: in-progress
date: 2026-10-31
associated-madr: "0046-MADR-sending-and-receiving-middleware.md"
---

# Implement 0046: sending and receiving middleware, and an observer

Associated MADR: [0046-MADR-sending-and-receiving-middleware.md](0046-MADR-sending-and-receiving-middleware.md)

## Goal

A connection runs registered middleware around every message that reaches its handler, and around every message the
application sends, and reports every other message to an observer, as the MADR's D1 to D11 state. The owner answered on
2026-10-31: both directions (Q1 a), and an observer for the messages outside the chains (Q2 b). With nothing registered, a
connection runs as it does at `v1.30.0`, with no allocation added.

Base: `523f319`, whose Go code is `v1.30.0`'s. Every file and line named below is at that commit. The MADR's "Revision" says
what the assessment of 2026-10-31 changed in the first draft of this pair.

## Scope

In scope:

* **New files:**
  * `middleware.go`: the `Middleware` type, `WithReceivingMiddleware`, `WithSendingMiddleware`, the chain builder,
    `RequestIDFromContext`, the sending call's state and the sending chain's innermost step;
  * `observer.go`: `WithObserver`, `Observation`, `ObservationKind`, its six constants and `String`, and the connection's
    `observe` helper;
  * `connection_middleware_test.go`, `connection_sending_middleware_test.go` and `connection_observer_test.go`.
* **Changed files:**
  * `connection.go`, at the seams only:
    * `connectionConfig` and `Connection` gain the chains and the observer;
    * `buildConnection` builds the chains;
    * `messageMark` gains the received id;
    * `exchange` is split at the write;
    * `SendRequestNoResult` uses `exchange`;
    * `SendNotification` and the cancel sender share an unexported direct path;
    * `sendRequestSent`, `sendRequestCommit`, `SendRequestNoResult` and `SendNotification` call the sending chain when one is
      set;
    * `readLine` counts a discarded line;
    * the observer's sites;
    * godoc.
  * `errors.go`: `ErrSendingMiddleware`.
  * `options_test.go`: the append rule in `TestOptionSemantics`.
  * `bench_test.go`: the benchmarks with middleware and an observer.
  * `example_api_test.go`: `ExampleWithReceivingMiddleware`, `ExampleWithSendingMiddleware`, `ExampleWithObserver`.
  * `docs_test.go`: `TestReadmeMiddlewareIsTheExample`.
  * `README.md`, `docs/architecture.md`, `docs/README.md`.
* The release, `v1.31.0`, on the owner's ask: `tools/consumer.py`, `tools/README.md`, the release notes.

Out of scope:

* typed middleware;
* an OpenTelemetry integration, or any dependency;
* recovering panics;
* middleware on F2's messages: they go to the observer instead (D9);
* an observer able to change or stop a message;
* registering middleware or an observer on a live connection;
* any change to the hooks' behaviour;
* any generated file. `Initialize`'s godoc is generated, so D4's rule for it is stated in `WithSendingMiddleware`'s godoc,
  and `tools/gates.py regen` stays clean;
* `remote`'s HTTP middleware, which the README's remote section already describes and this work does not touch.

## Implementation Steps

Every phase ends with the owner's commit of what it staged. A phase does not begin until the owner says to proceed. Every new
test is seen to fail on a planted break: each break is an entry in a scratch spec for `tools/plants.py`, whose `want` is the
failing test's own message, run in the tool's copies of the tree, never in the tree.

### Phase 1: refactors with no change of behaviour

The seams D4 and D5 need, made first, so that the existing suite proves them alone.

1. **The baseline benchmark.**
   * A `git archive 523f319` into the scratch directory runs `go test -run '^$' -bench
     'PromptRoundTrip|SessionUpdateNotify' -benchmem -count=10 .`. Its output is kept for phases 2 to 4.
   * A scratch script takes each benchmark's median `ns/op`, and its `B/op` and `allocs/op`.
2. **`SendRequestNoResult` through `exchange`.**
   * Its body (`connection.go:2721-2764`) repeats `exchange`'s. It becomes `exchange` with a fresh `pendingResponse`, then the
     barrier, then the error answer, as `sendRequestSent` does without the decode.
   * The order of its checks and their errors is unchanged: context, input, prepare, register, peer wait, write, wait.
     `claimedAnswer` returns false for a `pendingResponse` without a commit, so the write-failure path is the same.
   * Before the change, a scratch search lists the tests that call `SendRequestNoResult`; the record names them.
3. **`exchange` split at the write.**
   * `exchange` keeps its checks and `prepareRequest`, then calls a new `exchangePrepared(ctx, msg, idKey, pr)`: the
     registration (`:2413-2415`), the peer wait (`:2420-2423`), the write and its failure path (`:2425-2437`), and
     `waitForResponse`.
   * Phase 3's innermost step calls `exchangePrepared`, so both paths share one copy of those steps.
4. **A direct notification path.**
   * `SendNotification`'s body moves to an unexported `sendNotification(ctx, method, params)`. `SendNotification` calls it.
   * `sendCancelRequests` (`:2525`) calls `sendNotification` too, so phase 3 can put the chain in `SendNotification` alone
     (D4).
5. **The received id in the context.**
   * ~~`messageMark` gains `id *json.RawMessage`. `receiveLine` sets it to `msg.ID` for a request (`:1656`), and notifications
     leave it nil.~~ *Amended by deviation 1:* a request's `receiptKey` value becomes `requestReceipt{receipt, id}`, with `id`
     the message's `msg.ID`. A notification's stays a `uint64`, and `receiptOf` reads both.
   * Nothing reads it until phase 2.
6. **Checks.**
   * `go test -race -count=1 ./...`: every package `ok`, no test changed.
   * The benchmark of step 1 on the tree: `allocs/op` equal to the baseline; `B/op` up by at most 8 per received request (the
     pointer, D5); median `ns/op` within 5%. If the time differs more, rerun both alternately, three times each, before
     concluding. A difference that stays is recorded and stops the phase for the owner.
   * **Seen to fail.** Each refactored path is still covered, each break a `tools/plants.py` entry:
     * `exchangePrepared` without its peer wait (`:2420-2423`) fails `TestOverflowBlockRequestFromNotificationHandler`
       (`connection_overflow_block_test.go:271`);
     * a `sendNotification` that writes nothing fails a `SendNotification` test;
     * a `SendRequestNoResult` that drops the peer's error fails its test.

     The record names each test and its message.
   * `make pre-add-check`; `make api`: no change; `make check-records`, `make markdownlint`, `make disclosure-check`;
     `tools/idscan.py --paths` over the changed files.
   * The execution record, then staging.

### Phase 2: the receiving chain

1. **The type and options** (`middleware.go`).
   * `type Middleware func(next MethodHandler) MethodHandler`, documented with D1 and D7's rules.
   * `connectionConfig.receiving`, `[]Middleware`. `WithReceivingMiddleware(mw ...Middleware)` **appends** (D2).
     `WithSendingMiddleware` comes in phase 3, so no commit carries an option that does nothing.
   * `ConnectionOption`'s doc (`connection.go:484-491`) states the exception.
2. **The chain builder.**
   * `chainMiddleware(option string, mw []Middleware, inner MethodHandler) MethodHandler` wraps from the last to the first, so
     `mw[0]` is outermost.
   * It panics with `acp: <option>: middleware <i> is nil` or `acp: <option>: middleware <i> returned a nil handler`, where
     `i` is the position across repeated options.
   * It is called once per chain, in `buildConnection`, and returns `inner` unchanged for an empty list.
3. **Receiving.**
   * In `buildConnection`, when `receiving` is not empty, the handler is replaced by the chain over the handler.
   * A `nil` handler is first replaced by `notFound`, which answers a request `NewMethodNotFound(method)` and returns
     `nil, nil` for a notification (`IsNotification(ctx)`), as `handleInbound`'s `nil` branch does (`:2126-2134`).
   * `handleInbound` is unchanged.
4. **`RequestIDFromContext`, receiving half.** It returns a copy of `*messageMark.id` and `true` for a request's context,
   `json.RawMessage("null")` for a null id, and `nil, false` otherwise. Phase 3 adds the sending half before this check.
5. **Godoc:**
   * `MethodHandler`: middleware wraps it;
   * `WithInboundRewrite` and `WithInboundValidation`: they run inside the receiving chain's innermost handler;
   * `IsNotification`: inside a sending middleware it still describes the received message, and `RequestIDFromContext` is the
     sending side's test;
   * `NewConnection`: a `nil` handler under a chain.
6. **Tests**, in `connection_middleware_test.go`, each in a `synctest` bubble where it waits:
   * `TestReceivingMiddlewareOrder`: `a`, `b` in one option and `c` in a second each record their name before and after `next`.
     For a request and a notification the log is `a b c handler c b a`. (Confirmation 1.)
   * `TestReceivingMiddlewareConstruction`: a `nil` middleware, and one returning `nil`, panic with the messages of step 2.
     Each middleware is called once across ten messages. (Confirmation 1.)
   * `TestReceivingMiddlewareNilHandler`, through `NewConnection(nil, …)` with a pass-through middleware:
     * a request is answered `-32601`;
     * a notification is dropped, and a logger at `Debug` records nothing for it. (Confirmation 1.)
   * `TestReceivingMiddlewareShortCircuit`: a middleware returning `NewAuthRequired(nil)` for one method:
     * the request is answered `-32000`;
     * the agent's method does not run;
     * `RefusedMessages` stays 0, since the dispatch never ran;
     * a rewrite hook set beside it is not called. (Confirmation 2.)
   * `TestReceivingMiddlewareSeesRawAndResult`:
     * The middleware records the raw params, which equal the bytes sent, before a rewrite hook runs.
     * It passes params with one field changed; the rewrite hook and the handler see the change.
     * It records the result, a `*NewSessionResponse` after `checkResult`, before it is encoded.
     * A result that fails `checkResult` reaches the middleware as the `-32603` error. (Confirmation 3.)
   * `TestReceivingMiddlewareCountsInBudget`: with `WithMaxConcurrentRequests(1)`, a middleware that holds a request keeps
     the slot, and a second request is refused until it returns; with 0045's budget, `QueuedBytes` counts the request until the
     middleware returns. (Confirmation 7.)
   * `TestRequestIDFromContextReceiving`:
     * a request's numeric, string and null ids are seen by a middleware and by the handler;
     * a notification gives `false`;
     * changing the returned slice does not change the id the answer carries. (Confirmation 6.)
   * `TestReceivingMiddlewareKeepsOrdering`: with a pass-through middleware on both sides, the response barrier and the
     notification order hold. It is 0019 B1's and 0044's existing barrier cases, run with the option, in a bubble.
     (Confirmation 7.)
   * `TestOptionSemantics` gains a case: two `WithReceivingMiddleware` calls keep all three middleware, and the existing
     last-wins cases are unchanged.
7. **Planted breaks** (`tools/plants.py` entries):
   * the chain built last-outermost, MCP's order (`TestReceivingMiddlewareOrder`);
   * the second option replacing the first (`TestOptionSemantics`, `TestReceivingMiddlewareOrder`);
   * the chain built per message (`TestReceivingMiddlewareConstruction`'s count);
   * no `nil` check (`TestReceivingMiddlewareConstruction`);
   * `notFound` answering notifications `-32601` (`TestReceivingMiddlewareNilHandler`);
   * the short-circuit calling the handler anyway (`TestReceivingMiddlewareShortCircuit`);
   * the chain inside the dispatch, after decoding (`TestReceivingMiddlewareSeesRawAndResult`);
   * `release` before the chain returns (`TestReceivingMiddlewareCountsInBudget`);
   * `RequestIDFromContext` returning the id without copying it (`TestRequestIDFromContextReceiving`).
8. **Checks:**
   * phase 1's checks, the benchmark included, `allocs/op` equal to the baseline;
   * the new tests with `-race -count=3`;
   * `tools/remote.py pkg --host linux .` and `--host windows .` with
     `-test.run 'ReceivingMiddleware|RequestIDFromContext|OptionSemantics' -test.count=3`;
   * `make api`: `Middleware`, `WithReceivingMiddleware` and `RequestIDFromContext`, compatible additions only;
   * `TestExportedDeclarationsDocumented` and `TestExportedTypeDocsStartWithName` pass.

   Then the execution record, and staging.

### Phase 3: the sending chain

1. **The option.** `connectionConfig.sending`, `[]Middleware`, and `WithSendingMiddleware(mw ...Middleware)`, appending as
   `WithReceivingMiddleware` does; `buildConnection` builds `c.sending` over `c.sendInnermost` with `chainMiddleware`, or
   leaves it `nil`. `TestOptionSemantics` gains its case.
2. **The call's state** (`middleware.go`).
   * `type outboundCall struct`, carried under `outboundKey{}`:
     * `method`;
     * `id` (`nil` for a notification), `idKey`, and the `pendingResponse`;
     * the encoded `params`, to tell a replacement;
     * `entered atomic.Bool`;
     * `sent`;
     * the innermost step's `responseEnvelope`, raw result and barrier error, for the commit path.
3. **The innermost step**, `func (c *Connection) sendInnermost(ctx, method, params) (any, *RequestError)`, in this order:
   * the call from `ctx`, or `ErrSendingMiddleware`, "next called with a context that does not carry the call";
   * `entered.CompareAndSwap(false, true)`, or "next called twice";
   * `method == call.method`, or "next called with another method";
   * if `params` is not the encoded slice (a length or first-element address that differs): `nil`, or a first non-space byte of
     `{` or `[`, or "params must be a JSON object or array";
   * `ctx.Err()`, as `exchange` checks it;
   * a notification: `sendMessage`, and `nil` with `toReqErr` of the error;
   * a request: `exchangePrepared`; for a request without a commit, the barrier (`waitNotificationsUpTo`); then the raw
     result, or the peer's or the local `*RequestError`. `sent` is recorded on the call;
   * a request with a commit: the envelope and the barrier's error are recorded on the call, and the answer is returned as
     above.

   Each refusal is `NewInternalError({"error": <rule>}).withCause(ErrSendingMiddleware)`, so `errors.Is` finds it, and the
   rule is the error's text.
4. **The paths.** Each keeps today's code when `c.sending == nil`, a single nil check. With a chain:
   * **`sendRequestSent`:**
     * the checks and `prepareRequest`, as `exchange` orders them;
     * the call, the context `context.WithValue(ctx, outboundKey{}, call)`, and the chain;
     * the result to raw bytes: `json.RawMessage` as is, `nil` as none, anything else through `json.Marshal`, whose failure is
       `-32603`;
     * the decode, `checkInbound` and `checkPair` of today's `:2272-2284`;
     * `sent` from the call.

     A `*RequestError` the chain returns is returned only when it is not nil, so a typed nil never becomes a non-nil `error`.
   * **`sendRequestCommit`:**
     * the same, with its `pendingResponse` carrying the commit;
     * after the chain:
       * if no answer was judged, because the innermost step was never entered or its exchange failed, `settled()` runs, as
         today's error path runs it (`:2335-2338`), so a message held for `initialize` is released (F10; 0030's question 2).
         `settled` is safe to run twice: `end` is a `sync.OnceFunc` and `attempt.end` a `sync.Once`;
       * if the chain returned an error, the call returns it;
       * if its result is not the innermost step's raw result (`bytes.Equal`), the call fails with "a sending middleware
         replaced the result of initialize";
       * otherwise today's `switch` (`:2346-2355`) runs on the recorded envelope and barrier error.
   * **`SendRequestNoResult`:** as `sendRequestSent`, the result discarded.
   * **`SendNotification`:** its context check, `prepareNotification`, the call without an id, and the chain. The cancel
     sender keeps `sendNotification`.
5. **`RequestIDFromContext`, sending half.** An `outboundCall` in the context comes first: its id, copied, or `false` for a
   notification, before phase 2's received mark is read (D5).
6. **`ErrSendingMiddleware`** in `errors.go`, beside the other sentinels, documented with D10's list.
7. **Godoc:**
   * `WithSendingMiddleware`, with D4's before, inside and after, the result and params rules, and `Initialize`'s rule;
   * `SendRequest`, `SendRequestNoResult` and `SendNotification`: they pass the chain;
   * `ErrSendingMiddleware`.
8. **Tests**, in `connection_sending_middleware_test.go`, each in a `synctest` bubble where it waits, over `rawWire` or a side
   pair:
   * `TestSendingMiddlewareAllPaths`, one subtest per path:
     * `SendRequest[T]`, `SendRequestNoResult` and `SendNotification`;
     * a generated request (`NewSession`), a generated notification (`SessionUpdate`), `Prompt`, and `Initialize`;
     * `CallExtension` and `NotifyExtension`, on each side;
     * in each, the middleware adds `_meta.traceparent` with `TraceMeta`, and the peer reads it. (Confirmation 4.)
   * `TestSendingMiddlewareOrder`: three middleware over two options run first-outermost around one call. (Confirmation 4.)
   * `TestSendingMiddlewareResult`:
     * a raw result and a peer's error reach the middleware and the caller;
     * a middleware returning another `json.RawMessage` makes `SendRequest[T]` decode it;
     * a struct is encoded, then decoded;
     * a result breaking the schema is refused by `checkInbound`, as the peer's would be. (Confirmation 4.)
   * `TestSendingMiddlewareSkipsCancelRequest`: a caller's context ends while waiting. The peer receives `$/cancel_request`,
     and the middleware never sees that method. (Confirmation 4.)
   * `TestSendingMiddlewareMisuse`, one subtest per rule of D10. Each returns an error for which
     `errors.Is(err, ErrSendingMiddleware)`, the peer reads nothing, and no pending entry is left. (Confirmation 5.)
   * `TestSendingMiddlewareShortCircuitInitialize`: a middleware refuses `initialize` without calling `next`.
     * `Initialize` returns its error, and no record is made.
     * A request the agent sent before, held for the attempt, is judged and refused `-32601`, not held.
     * A second `Initialize` without the refusal succeeds. (Confirmation 7, 0030.)
   * `TestSendingMiddlewareInitializeRecord`:
     * A middleware that observes `initialize` leaves the record as the peer answered.
     * One that returns an error after `next` returns that error, and the record stays, as documented. (Confirmation 7.)
   * `TestSendingMiddlewareKeepsBarrier`: a notification handler sends a request under its context through a middleware that
     derives a context with a value. The request is answered while the handler runs, which is B1's case, and 0044's guard reads
     on under `OverflowBlock`. (Confirmation 7.)
   * `TestRequestIDFromContextSending`:
     * the middleware sees the id the peer receives;
     * a notification gives `false`;
     * inside a received request's handler, a sending middleware sees the sent request's id, and for a notification `false`,
       not the received id. (Confirmation 6.)
   * `TestSendingMiddlewareSentFlag`: `Prompt`'s `session/cancel` after a cancelled turn is sent when `next` wrote the request,
     and not when the middleware refused it. (Confirmation 4.)
9. **Planted breaks:**
   * a path left off the chain, one entry per path (`TestSendingMiddlewareAllPaths`, its subtest);
   * the cancel sender through `SendNotification` (`TestSendingMiddlewareSkipsCancelRequest`);
   * each D10 check removed, one entry each (`TestSendingMiddlewareMisuse`, its subtest);
   * a replaced result not decoded (`TestSendingMiddlewareResult`);
   * the decode before the chain instead of after (`TestSendingMiddlewareResult`, the schema case);
   * no `settled()` when the innermost step is not entered (`TestSendingMiddlewareShortCircuitInitialize`, which then hangs
     until its bubble's deadline);
   * the barrier outside the innermost step's context, for the caller's (`TestSendingMiddlewareKeepsBarrier`);
   * the received mark read before the call (`TestRequestIDFromContextSending`);
   * `sent` taken as true (`TestSendingMiddlewareSentFlag`);
   * a typed nil `*RequestError` returned as an `error` (`TestSendingMiddlewareAllPaths`).
10. **Checks:** phase 2's, with `-test.run 'SendingMiddleware|RequestIDFromContext'` on the hosts. `make api` adds
   `ErrSendingMiddleware`. Then the execution record, and staging.

### Phase 4: the observer

1. **The types** (`observer.go`):
   * `type ObservationKind int`, with `ObservedFault`, `ObservedCancelReceived`, `ObservedCancelSent`, `ObservedRefused`,
     `ObservedUnmatchedResponse` and `ObservedDropped`, starting at 1 so the zero value is no kind;
   * `String`, which names each and prints an unknown one as `ObservationKind(n)`;
   * `type Observation struct { Kind ObservationKind; Method string; ID json.RawMessage; Size int; Err error }`;
   * `WithObserver(fn func(Observation))`, the last given winning, `nil` removing it.
2. **The helper.** `func (c *Connection) observe(kind ObservationKind, method string, id json.RawMessage, size int, err
   error)` returns at once when no observer is set. It copies `id`, and calls the observer. Its call sites add no branch to
   their functions, so `receiveLine` stays within `gocognit`'s limit; the phase records its measure.
3. **`readLine`** returns the length of a line it discards for the limit, counted per chunk, for `Size`.
4. **The sites**, each after any lock is released:
   * `ObservedFault`: `receive` (`:1456-1458`) and `receiveLine`'s four (`:1585-1617`), with the `*RequestError` each answers;
   * `ObservedCancelReceived`: `handleCancelRequest`, at each return, with the id when its params decode;
   * `ObservedCancelSent`: `sendCancelRequests`, after `sendNotification`, with its error;
   * `ObservedRefused`: the three refusals (`:1633-1652`, `:1669-1678`), with the answer;
   * `ObservedUnmatchedResponse`: `receiveResponse`'s no-id branch (`:1849-1852`) and `handleResponse`'s `pr == nil`
     (`:2063-2066`), with the response's error;
   * `ObservedDropped`: `dropNotification`, and the overflow close before `closeWith`, with `ErrNotificationQueueOverflow`.
5. **Godoc:** `WithObserver` with D9's rules; each kind with its sites; `RefusedMessages` and `DroppedNotifications` naming
   the observer.
6. **Tests**, in `connection_observer_test.go`, each in a `synctest` bubble where it waits:
   * `TestObserverReportsEachKind`, a subtest per kind and per site, each checking `Kind`, `Method`, `ID`, `Size` and `Err`
     exactly, and that it is reported once:
     * the five faults;
     * a cancel received, with and without params that decode;
     * a cancel sent;
     * the three refusals;
     * a late response, an unknown id, and a response without an id;
     * a drop, and the overflow close. (Confirmation 8.)
   * `TestObserverNoLockHeld`: an observer that calls `QueuedBytes`, `RefusedMessages` and `DroppedNotifications` at every
     kind. The run ends, and the race detector is quiet. (Confirmation 8.)
   * `TestObserverAndChainsDisjoint`: a run with both chains and the observer, over a mixed sequence of every kind and
     ordinary traffic. Each message the peer sent or the SDK wrote is counted once, across the three. (Confirmation 8.)
   * `TestObserverUnset`: the same sequence with no observer runs, and `testing.AllocsPerRun` of `c.observe` with an id and
     no observer is 0. (Confirmation 8, 9.)
   * `TestObservationKindString`: every kind, and an unknown one.
7. **Planted breaks:**
   * each site removed, one entry per site (`TestObserverReportsEachKind`, its subtest);
   * the observer called under `notifyMu` at the drop (`TestObserverNoLockHeld`, which then hangs until its bubble's deadline);
   * a matched response reported (`TestObserverAndChainsDisjoint`);
   * `ID` not copied (`TestObserverReportsEachKind`, a case that changes the slice after the call);
   * the copy made before the nil check (`TestObserverUnset`).
8. **Checks:** phase 3's, with `-test.run 'Observer|Observation'` on the hosts, and `make api`. Then the execution record,
   and staging.

### Phase 5: documentation, examples and benchmarks

1. **`README.md`,** a "Middleware and observing" section after "Closing and limits":
   * the type, the two options, and the order rule, with its exception to "last wins";
   * where each chain sits: receiving outside the dispatch, inside the slot and the budget; sending between the SDK's checks
     and the exchange, its result checked after;
   * what the chains do not see (D6), and the observer's six kinds;
   * the rules (D7, D10), and `Initialize`'s;
   * a tracing sketch with `TraceMeta` and `RequestIDFromContext`, which is `ExampleWithSendingMiddleware`'s body, bound by
     `TestReadmeMiddlewareIsTheExample` as the other README examples are bound (`docs_test.go:187-232`);
   * a metrics sketch counting refusals with the observer;
   * one sentence telling this from `remote`'s HTTP middleware.
2. **`docs/architecture.md`:** the two chains and the observer's sites in the message path, and the new files and tests in
   its lists.
3. **Examples**, with fixed output, each seen to fail with its option removed:
   * `ExampleWithReceivingMiddleware`: an authorization middleware refusing one method, over `NewConnection` and a pipe;
   * `ExampleWithSendingMiddleware`: `_meta.traceparent` added, and the method, id and outcome printed;
   * `ExampleWithObserver`: a request refused over the concurrency limit, printed as its kind, id and error.
4. **Benchmarks** in `bench_test.go`:
   * `BenchmarkPromptRoundTripMiddleware`: one pass-through middleware each way on both sides;
   * `BenchmarkSessionUpdateNotifyObserved`: an observer set, which these messages never call.

   They are run with step 1.1's command, and their `allocs/op` against the baseline is recorded: D11 expects two more per sent
   message with sending middleware, none for receiving middleware, and none for the observer.
5. Phase 4's checks, the benchmark comparison of step 1.6 included; then the execution record, and staging.

### Phase 6: close

1. `tools/gates.py --all`, with its output in the record; `make push-guard` after a fetch.
2. After the owner pushes, `tools/watch_ci.py --branch main` on the head.
3. A table of the MADR's eleven Confirmation items, each with the test or command that shows it, and any test that no plant
   reached, planted then.
4. The MADR's status, this PLAN's, and the index rows.

### Phase 7: the release, on the owner's ask

As [0045-PLAN-notification-queue-byte-budget.md](0045-PLAN-notification-queue-byte-budget.md)'s phase 4 ran it:

1. `tools/consumer.py`:
   * one receiving and one sending middleware that count the methods they see;
   * an observer counting kinds, with a request refused over a concurrency limit of 1;
   * the counts, a `RequestIDFromContext` line, and `errors.Is` of a misuse against `acp.ErrSendingMiddleware` printed in
     `WANT_RUN`;
   * `VERSION` `v1.31.0` or later in the docstring, and `tools/README.md`'s row.

   It is built against the tree through a `replace` three times. `tools/consumer.py v1.30.0` must fail its build, and
   `make check-tools` must pass.
2. `docs/reports/0046-REPORT-release-notes-v1-31-0.md`, from `make api` and the commits since `v1.30.0`, and its index row.
3. The owner's tag; then `tools/release_body.py`, the owner's release, `tools/publication.py`, and `tools/consumer.py v1.31.0`.
4. The PLAN and its row `complete`.

## Verification

* Every test of phases 2 to 4 passes under `-race`, and with `-count=3` on the Linux and Windows hosts, and was seen to fail on
  its planted break.
* The existing tests pass unchanged, the barrier, overflow, hold, budget and hook tests among them.
* `BenchmarkPromptRoundTrip` and `BenchmarkSessionUpdateNotify`, with nothing registered: `allocs/op` equal to the baseline at
  `523f319`, `B/op` up by at most one word per received request, median `ns/op` within 5%.
* `make api` against `v1.30.0`: compatible additions only, those of the MADR's "More Information".
* `tools/gates.py regen` clean: no generated file changes.
* `tools/gates.py --all` and CI pass.

## Rollout and Rollback

* **Rollout.** Additive: no behaviour changes for a connection without middleware or an observer. A minor, `v1.31.0`.
* **Rollback before the release.** Revert the phase commits, the latest first. Phase 1's refactor stands alone and may stay.
* **Rollback after the release.** The type, options, function, sentinel and observer stay, under the compatibility promise. A
  defect in a chain is fixed in a patch. A defect only in the sending chain can be contained in a patch by documenting
  `WithSendingMiddleware` as inert and skipping the chain, while it is fixed.

## Execution record

*2026-10-31:* the owner committed the revised pair as `fbbbd5c` and, asked to accept the MADR and approve this PLAN, said
"committed, proceed". The MADR is `accepted` and this PLAN `in-progress`.

**Deviation 1, 2026-10-31, step 1.5: the received id is kept beside the receipt, not in `messageMark`.**

* *Found.* With step 1.5 as written, step 1.6's benchmark gave `BenchmarkPromptRoundTrip` 100 allocations per operation
  against the baseline's 99, on each of three alternating rounds, with time within −1.3% to +2.3%.
  `BenchmarkSessionUpdateNotify` gained about 8 bytes per operation, since the mark grew from 16 to 24 bytes.
  * The cause, from a scratch probe of `context.WithValue` with `testing.AllocsPerRun` on `go1.27.3` and `go1.26.10`: a
    request's mark is the zero literal `messageMark{}`, which Go boxes from static data without allocating. Any mark holding an
    id allocates.
  * The probe measured 3 allocations for today's request context, 4 with the id in the mark, and 3 with the id beside the
    receipt, whose value allocates already.
  * This contradicted the MADR's D5, "no allocation is added", and D11, and step 1.6's "`allocs/op` equal".
* *Options put to the owner:*
  * keep the id beside the receipt (recommended), for one word more per received request;
  * keep it in the mark, and accept an allocation per request and a word per notification on every connection;
  * set it only when a receiving chain is registered, which leaves handlers without it otherwise.
* *Decided.* The owner chose the first. The MADR's D5 carries an annotated amendment. Step 1.5 above is annotated. No file is
  added to the phase's scope.

### Phase 1, 2026-10-31

**What changed,** all in `connection.go`, 34 lines added and 34 removed:

* **Step 1.2.** `SendRequestNoResult` is `exchange`, then the barrier, then the error answer.
  * Before the change, eight tests called it: `TestSendRequestNoResultAnswers`, `TestSendRequestNoResultHeldBehindNotification`,
    `TestConcurrentUse`, `TestEndedCleanly`, `TestErr`, `TestParamsNeverSentAsNull`, `TestRequestAfterEndOfInputNotWritten`,
    and `testRequestWithEndedContextWritesNothing`.
  * Its checks and errors keep their order. On a failed write, `claimedAnswer` returns false for a response without a commit,
    so the path is the one it had.
* **Step 1.3.** `exchange` keeps its checks and `prepareRequest`, and calls `exchangePrepared(ctx, msg, idKey, pr)`, which
  registers, counts the peer wait, writes and waits.
* **Step 1.4.** `SendNotification` calls `notifyDirect`, its former body, and so does `sendCancelRequests`.
  * *Refinement, not a deviation:* the PLAN's name, `sendNotification`, failed `make pre-add-check`: "confusing-naming: Method
    'sendNotification' differs only by capitalization to method 'SendNotification' in the same source file (revive)".
  * It is `notifyDirect`, and the later phases' `sendNotification` means it.
* **Step 1.5, as amended by deviation 1.**
  * `requestReceipt{receipt uint64; id *json.RawMessage}` is a received request's `receiptKey` value, with `msg.ID`.
  * `receiptOf` reads it or a bare `uint64`, which notifications and `cancel_test.go`'s direct calls keep.
  * `messageMark` is unchanged.

**Step 1.1, the baseline:** a `git archive 523f319` in the scratch directory, `go test -run '^$' -bench
'PromptRoundTrip|SessionUpdateNotify' -benchmem -count=10 .`. Medians:

| Benchmark | ns/op | B/op | allocs/op |
| :--- | ---: | ---: | ---: |
| `BenchmarkPromptRoundTrip` | 26696 | 5426 | 99 |
| `BenchmarkSessionUpdateNotify` | 14483 | 4878 | 65 |

**Step 1.6, the checks.**

* `go test -race -count=1 ./...`: every package `ok`. No test changed.
* **The benchmark,** the base and the tree alternately, three rounds of `-count=5`, on the final code:

  | Round | `PromptRoundTrip` ns/op | B/op | allocs/op | `SessionUpdateNotify` ns/op | B/op | allocs/op |
  | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
  | 1 | 28212 → 28084 (−0.5%) | +7 | 99 → 99 | 19980 → 15456 (−22.6%) | +2 | 65 → 65 |
  | 2 | 34128 → 27350 (−19.9%) | +4 | 99 → 99 | 16832 → 14640 (−13.0%) | −1 | 65 → 65 |
  | 3 | 26100 → 26628 (+2.0%) | +9 | 99 → 99 | 13459 → 13493 (+0.3%) | +0 | 65 → 65 |

  * Allocations are equal.
  * Bytes rise by about one word per round trip, which receives one request, and are unchanged for notifications.
  * The base's slower rounds 1 and 2 are the machine's noise: the base is the slower side. Round 3 is within 2%.
* **Seen to fail,** `tools/plants.py`, four plants in its copies, on the final code, "4/4 as required":

  | Break | Test | Failure seen |
  | :--- | :--- | :--- |
  | `exchangePrepared` without its peer wait | `TestOverflowBlockRequestFromNotificationHandler` | "the handler's request is not answered: the reader paused while the handler waited on it" |
  | `notifyDirect` writes nothing | `TestNotifyExtensionNames` | "the notification did not happen" |
  | `SendRequestNoResult` drops the peer's error | `TestSendRequestNoResultAnswers` | "SendRequestNoResult = <nil>, want code -32602" |
  | `receiptOf` misses a request's receipt | `TestPromptCancelRightAfterPrompt` | "answer {…"result":{"stopReason":""}}, want stopReason cancelled" |

* `make pre-add-check`: "go-precheck: 279 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck)."
* `make api`: "against v1.30.0: 0 incompatible, 0 compatible".

The owner committed phase 1 as `1de8d98`.

### Phase 2, 2026-10-31

The owner said "committed, proceed".

**What changed.**

* **`middleware.go`, new, 97 lines:**
  * `Middleware`, with D1 and D7's rules;
  * `WithReceivingMiddleware`, appending, with D2, D3 and D6 in its godoc;
  * `chainMiddleware`, which wraps over `slices.Backward`, so `mw[0]` is outermost, and panics as step 2.2 says;
  * `notFound`;
  * `RequestIDFromContext`, its receiving half, which returns `slices.Clone` of the `requestReceipt`'s id.
* **`connection.go`:**
  * `connectionConfig.receiving`;
  * `buildConnection` replaces the handler with the chain when one is registered, a `nil` handler first becoming `notFound`;
  * godoc of `ConnectionOption` (the exception), `MethodHandler`, `WithInboundRewrite`, `WithInboundValidation` and
    `NewConnection`.
* **`options_test.go`:** `TestOptionSemantics` checks that two `WithReceivingMiddleware` calls keep all three middleware, the
  first outermost, over a `nil` handler answering `-32601`.
* **`connection_middleware_test.go`, new:** the eight tests of step 2.6, `TestReceivingMiddlewareKeepsOrdering` as two
  subtests, each in a `synctest` bubble.

**Refinements, not deviations.**

* *`IsNotification`'s godoc.* The sentence about sending middleware (step 2.5) waits for phase 3, so no doc names an option that
  does not exist yet.
* *Two plants changed, and two added.*
  * "The short-circuit calling the handler anyway" is the middleware's behaviour, in the test, not the SDK's. The SDK's break
    that `TestReceivingMiddlewareShortCircuit` guards is a chain that is never built, which replaced it.
  * "The chain inside the dispatch, after decoding" cannot be planted without generated code. The break planted in its place
    is what that test sees of it: the result encoded before the chain sees it.
  * `TestReceivingMiddlewareKeepsOrdering` gained plants of its own: completion before the chain returns, and the barrier
    without B1's cap.
* *A test's formatting.* `TestRequestIDFromContextReceiving` prints the id with `string(id)`. Under Go 1.27's default build
  `json.RawMessage` is `jsontext.Value`, whose `String` prints a nil value as `null`. With `%s` the test passed at the floor and
  failed at `go1.27.3`.

**Step 2.7, seen to fail.** `tools/plants.py`, eleven plants, "11/11 as required". The first run had ten as required: the
eleventh failed its test, but with "the handler's request's return did not happen", not the "deadlock" the spec guessed. The spec
was corrected and all eleven rerun.

| Break | Test | Failure seen |
| :--- | :--- | :--- |
| the chain built last-outermost, MCP's order | `TestReceivingMiddlewareOrder` | `request: "c b a handler a b c", want "a b c handler c b a"` |
| a repeated option replacing the earlier | `TestOptionSemantics` | `repeated WithReceivingMiddleware ran "c c", want all three, the first outermost` |
| the chain built per message | `TestReceivingMiddlewareConstruction` | no panic at construction, and "over ten messages, want once" |
| no `nil` check on a middleware | `TestReceivingMiddlewareConstruction` | `panic runtime error: invalid memory address or nil pointer dereference, want "acp: WithReceivingMiddleware: middleware 1 is nil"` |
| `notFound` answering notifications `-32601` | `TestReceivingMiddlewareNilHandler` | "the dropped notification was logged" |
| no receiving chain at all | `TestReceivingMiddlewareShortCircuit` | `answer {ID:1 Result:{"sessionId":"s"} Error:<nil>}, want -32000` |
| the result encoded before the chain sees it | `TestReceivingMiddlewareSeesRawAndResult` | `result jsontext.Value {"sessionId":"s"}, … want *NewSessionResponse s` |
| `release` before the chain returns | `TestReceivingMiddlewareCountsInBudget` | "QueuedBytes = 0 while the middleware holds the request" |
| `RequestIDFromContext` not copying | `TestRequestIDFromContextReceiving` | `"handler x true", want "handler 7 true"` |
| notifications completed before their chain returns | `TestReceivingMiddlewareKeepsOrdering/the_response_barrier` | "SendRequestNoResult returned <nil> while the notification before its answer ran" |
| the barrier without B1's cap | `TestReceivingMiddlewareKeepsOrdering/a_notification_handler's_own_request` | "the handler's request's return did not happen" |

**Step 2.8, the checks.**

* `go test -race -count=3 -run 'ReceivingMiddleware|RequestIDFromContext|OptionSemantics' .`: `ok`. At the floor,
  `GOTOOLCHAIN=go1.26.10 GOEXPERIMENT=goroutineleakprofile`, once: `ok`.
* `tools/remote.py pkg` on the Linux and Windows hosts, with that `-test.run` and `-test.count=3`: "PASS", exit 0, on each.
* `make pre-add-check`: "279 file(s) clean"; with the new files named, "2 file(s) clean". The whole suite runs in it, and
  `TestExportedDeclarationsDocumented` and `TestExportedTypeDocsStartWithName` pass.
* `make api`: "compatible: Middleware: added", "compatible: RequestIDFromContext: added", "compatible: WithReceivingMiddleware:
  added"; "against v1.30.0: 0 incompatible, 3 compatible".
* **The benchmark,** three alternating rounds of `-count=5` against the baseline. Allocations are equal in every round (99 and
  65). Pooled over the fifteen runs of each side:

  | Benchmark | ns/op | B/op | allocs/op |
  | :--- | :--- | :--- | :--- |
  | `BenchmarkPromptRoundTrip` | 27166 → 27347 (+0.7%) | +7 | 99 → 99 |
  | `BenchmarkSessionUpdateNotify` | 15608 → 15048 (−3.6%) | +0 | 65 → 65 |

  The rounds alone ranged from −8.3% to +6.9%, in both directions, which is the machine's noise.

The owner committed phase 2 as `df22f9e`.

### Phase 3, 2026-10-31

The owner said "committed, proceed".

**What changed.**

* **`middleware.go`, 252 lines added:**
  * `WithSendingMiddleware`, appending, with D4 and D10 in its godoc;
  * `outboundCall` and `outboundKey`; `misuse`, `sameBytes` and `structuredParams`;
  * `sendInnermost`, which checks D10's four rules in the order of step 3.3, then writes or exchanges, and records `sent`,
    `answered`, the envelope, the raw result, the barrier's error and the error it returned;
  * `startCall` (`exchange`'s checks and `prepareRequest`), `runChain`, `resultBytes`;
  * `sendThroughChain` and `commitThroughChain`;
  * `RequestIDFromContext`'s sending half, ahead of the received mark.
* **`connection.go`:**
  * `connectionConfig.sending`, and `Connection.sending`, the chain over `sendInnermost`, built once in `buildConnection`, or
    `nil`;
  * `sendRequestSent` and `sendRequestCommit` call the chain's path when `c.sending` is set, and keep their code otherwise;
  * `decodeResult` and `committedResult` are their decode and their final `switch`, moved out unchanged, so both paths share
    them;
  * `SendRequestNoResult` and `SendNotification` gained their chain branches; the cancel sender keeps `notifyDirect`;
  * godoc of `ConnectionOption`, `IsNotification` (deferred from phase 2), `SendRequest`, `SendRequestNoResult` and
    `SendNotification`.
* **`errors.go`:** `ErrSendingMiddleware`.
* **`options_test.go`:** two `WithSendingMiddleware` calls keep all three middleware, the first outermost; the chain called
  without a call in its context refuses with `ErrSendingMiddleware`.
* **`connection_sending_middleware_test.go`, new:** the ten tests of step 3.8. `TestSendingMiddlewareAllPaths` covers eleven
  paths, among them `Initialize`, `Prompt`, and each side's `CallExtension` and `NotifyExtension`.

**Refinements, not deviations.**

* *What the innermost step returns for `initialize`.* For a committed answer whose commit failed, it returns that error to the
  middleware as a `*RequestError`, so a tracing middleware sees the failure. The caller still gets the commit's own error, a
  `*ProtocolVersionError` for example: when the chain returns the step's own error, `commitThroughChain` applies
  `committedResult` to the recorded answer.
* *Two test corrections before the first green run:*
  * the test package already had a type `sent`; the new one is `sentLine`;
  * `TestSendingMiddlewareKeepsBarrier` first sent three notifications after the handler's request, with a queue of one. Under
    `OverflowBlock` the reader reads on only to twice the queue, so the third closed the connection: "failed to queue
    notification; closing connection … capacity=1 queued=2". Two notifications need the guard and stay within the read-on
    limit.
* *`thelper`* asked for `t.Helper()` in the two builder closures of `sendPaths`.

**Step 3.9, seen to fail.** `tools/plants.py`, seventeen plants. The first run refused three as written, since removing a check
left a variable unused (`declared and not used: ok`, `raw`, `raw`). They were rewritten to keep the variable used. The second run
had sixteen as required. The seventeenth failed its test with a pause and the bubble's deadlock, not the "did not happen" the
spec guessed; with its `want` corrected, it ran alone, "1/1 as required".

| Break | Test | Failure seen |
| :--- | :--- | :--- |
| `sendRequestSent` off the chain | `TestSendingMiddlewareAllPaths/SendRequest` | "the middleware did not happen" |
| `sendRequestCommit` off the chain | `TestSendingMiddlewareAllPaths/Initialize` | "the middleware did not happen" |
| `SendRequestNoResult` off the chain | `TestSendingMiddlewareAllPaths/SendRequestNoResult` | "the middleware did not happen" |
| `SendNotification` off the chain | `TestSendingMiddlewareAllPaths/SendNotification` | "the middleware did not happen" |
| the cancel sender through `SendNotification` | `TestSendingMiddlewareSkipsCancelRequest` | `the middleware saw "$/cancel_request", the connection's own cancel` |
| no check of a second `next` | `TestSendingMiddlewareMisuse/next_called_twice` | "the call did not happen": the second request was written and waits |
| no check of the context | `TestSendingMiddlewareMisuse/a_context_that_does_not_carry_the_call` | a nil pointer panic in the innermost step |
| no check of the method | `TestSendingMiddlewareMisuse/another_method` | "the call did not happen" |
| no check of replaced params | `TestSendingMiddlewareMisuse/params_that_are_not_an_object_or_an_array` | "the call did not happen" |
| no check of a replaced `initialize` result | `TestSendingMiddlewareMisuse/a_replaced_result_of_initialize` | "Initialize = <nil>, want ErrSendingMiddleware" |
| a replaced result not decoded | `TestSendingMiddlewareResult` | "want n 2", and the replaced `session/list` result not refused |
| a replaced result not checked | `TestSendingMiddlewareResult/a_replaced_result_is_checked` | "ListSessions = <nil>, want the replaced result refused with a *ValidationError" |
| no `settled()` when no answer was judged | `TestSendingMiddlewareShortCircuitInitialize` | "the held request's answer did not happen" |
| the exchange off the middleware's context | `TestSendingMiddlewareKeepsBarrier` | "notification queue full; reading paused", then "deadlock: all goroutines in bubble are blocked" |
| the received mark read before the call | `TestRequestIDFromContextSending` | `"_x/note \"recv\" true", want "_x/note  false"` |
| `sent` taken as true | `TestSendingMiddlewareSentFlag/refused_unwritten` | `the peer read ["session/cancel"] after Prompt returned; session/cancel sent true, want false` |
| a typed nil returned as an `error` | `TestSendingMiddlewareAllPaths/SendRequestNoResult` | "the call = <nil>, want nil" |

**Step 3.10, the checks.**

* `go test -race -count=3 -run 'SendingMiddleware|RequestIDFromContext|ReceivingMiddleware|OptionSemantics' .`: `ok`. At the
  floor, with `GOEXPERIMENT=goroutineleakprofile`, once: `ok`.
* `tools/remote.py pkg` on the Linux and Windows hosts, `-test.run 'SendingMiddleware|RequestIDFromContext|OptionSemantics'
  -test.count=3`: "PASS", exit 0, on each.
* `make pre-add-check`: "go-precheck: 281 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck)"; with the new
  file named, "1 file(s) clean".
* `make api`: "compatible: ErrSendingMiddleware: added" and "compatible: WithSendingMiddleware: added", beside phase 2's three;
  "against v1.30.0: 0 incompatible, 5 compatible".
* **The benchmark,** nothing registered, three alternating rounds of `-count=5`. Allocations are equal in every round (99 and
  65). Pooled:

  | Benchmark | ns/op | B/op | allocs/op |
  | :--- | :--- | :--- | :--- |
  | `BenchmarkPromptRoundTrip` | 33689 → 34152 (+1.4%) | +10 | 99 → 99 |
  | `BenchmarkSessionUpdateNotify` | 19395 → 18488 (−4.7%) | +0 | 65 → 65 |

  The machine was busier than in phase 2, with both sides slower, and the rounds ranged from −11.6% to +4.5%.

The owner committed phase 3 as `f23e107`.

### Phase 4, 2026-10-31

The owner said "committed, proceed".

**What changed.**

* **`observer.go`, new, 129 lines:**
  * `ObservationKind`, from 1, with its six constants, each documented with its sites and fields, and `String`, which gives
    `fault`, `cancel_received`, `cancel_sent`, `refused`, `unmatched_response` and `dropped`, or `ObservationKind(n)`;
  * `Observation`;
  * `WithObserver`, with D9's rules;
  * `observe`, which returns before copying anything when no observer is set;
  * `answerObserved`, which answers and then reports, so each answered site stays one call;
  * `requestErr`, a `*RequestError` as an `error` that is never a typed nil.
* **`connection.go`,** 85 lines added and 53 removed:
  * `connectionConfig.observer` and `Connection.observer`;
  * `readLine` returns the length of a line it discards, newline excluded;
  * the sites of step 4.4:
    * the five faults;
    * `$/cancel_request` received, through `handleCancelRequest`, which now returns the id it named;
    * the cancel sender;
    * the three refusals;
    * `receiveResponse`'s no-id branch, with the error the response carried if it decodes, and a response `handleResponse`
      reports it did not hand over (it now returns whether it did);
    * `dropNotification`, which takes the size;
    * the overflow close, before `closeWith`.
  * godoc of `DroppedNotifications` and `RefusedMessages`, which name the observer.
* **`connection_observer_test.go`, new:**
  * `TestObserverReportsEachKind`, fifteen subtests, one per site, with the size taken from each line;
  * `TestObserverNoLockHeld`, `TestObserverAndChainsDisjoint`, `TestObserverUnset` and `TestObservationKindString`;
  * two not in step 4.6: `TestObserverCopiesID` and `TestWithObserverLastWins` (below).

**Refinements, not deviations.**

* *`gocognit`* (step 4.2), measured with a scratch config at a minimum of 1, here and at `523f319`:
  * `receiveLine` is 43 before and after;
  * `receiveResponse` went from 10 to 14, and `handleResponse` from 11 to 7;
  * `sendCancelRequests` is 14 before and after.

  The limit is 50.
* *`TestObserverCopiesID`, added.* No test through the connection can see whether `ID` is copied. Each id comes from a line
  `readLine` allocates afresh, or from the cancel sender's own copy, and the connection does not use it after the report. The
  plant "`ID` not copied" therefore needed a test of `observe` itself: it reports an id, changes the bytes it passed, and
  requires the observer's copy unchanged.
* *`TestWithObserverLastWins`, added,* for the option's rule, the last given winning and `nil` removing it, which step 4.1 states
  and no test covered.
* *One site has no test.* `receiveLine`'s "method must be a string" fault (`json.Unmarshal` of the method) cannot be reached by any
  input. The line has passed `json.Valid`, and `requestFault` has required the method to begin with a quote, so it is a valid
  JSON string, which decodes. The site reports like the other four faults, through `answerObserved`, and no plant can make it
  fail a test.
* *A late answer* is not a case of its own. It takes the same branch, `handleResponse`'s `pr == nil`, as the unknown id, which
  the subtest covers.
* *Before the first green run,* the hand-counted sizes of nine subtests were wrong, by one or two bytes. The subtests now take
  `len` of named lines. A failure message printed a nil id as `null` (`jsontext.Value`'s `String`); it prints `string(o.ID)`.
  `errOf` clashed with a test helper and is `requestErr`. `thelper` asked for `t.Helper()` in six table closures.

**Step 4.7, seen to fail.** `tools/plants.py`, twenty plants:

* The first run had nineteen as required. "ID not copied" did not compile: dropping `slices.Clone` left `slices` unused. It
  became `slices.Clip(id)`, which keeps the import and shares the bytes, and ran alone, "1/1 as required".
* After `t.Helper()` was added, all twenty were run again on the final code: "20/20 as required".

| Break | Test | Failure seen |
| :--- | :--- | :--- |
| each of the thirteen reporting sites removed, one plant each | `TestObserverReportsEachKind`, the site's subtest | "fault did not happen", "cancel_received did not happen", "cancel_sent did not happen", "refused did not happen", "unmatched_response did not happen", "dropped did not happen" |
| the observer called under `notifyMu` at a drop | `TestObserverNoLockHeld` | the reader deadlocked on the observer's `QueuedBytes`; "test timed out" |
| a matched response reported | `TestObserverAndChainsDisjoint` | `"observe unmatched_response"` twice in what was seen, "want each message once" |
| `ID` not copied | `TestObserverCopiesID` | `the observer's ID became "xyz" with the connection's bytes, want its own copy "abc"` |
| the copy made before the nil check | `TestObserverUnset` | "observe without an observer allocated 1 times, want 0" |
| a typed nil reported as `Err` | `TestObserverReportsEachKind/a_response_to_an_unknown_id` | "observed Err <nil>, want nil" |
| a kind misnamed | `TestObservationKindString` | `2: "cancel-received", want "cancel_received"` |
| the first `WithObserver` winning | `TestWithObserverLastWins` | `observers called ["first" "first"], want only the last given, and none after nil` |

The thirteen sites: the line over the limit, invalid JSON, not an object, an invalid request, a cancel received, a cancel sent,
the three refusals, an unmatched response, a response without an id, a drop, and the overflow close.

**Step 4.8, the checks.**

* `go test -race -count=3 -run 'Observer|Observation' .`: `ok`. At the floor, with `GOEXPERIMENT=goroutineleakprofile`, once:
  `ok`. `TestObserverUnset`'s `testing.AllocsPerRun` gives 0 under `-race` too.
* `tools/remote.py pkg` on the Linux and Windows hosts, `-test.run 'Observer|Observation' -test.count=3`: "PASS", exit 0, on
  each.
* `make pre-add-check`: "go-precheck: 282 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck)"; with the new
  files named, "2 file(s) clean".
* `make api`: nine more compatible additions, `Observation`, `ObservationKind`, its six constants and `WithObserver`; "against
  v1.30.0: 0 incompatible, 14 compatible".
* **The benchmark,** nothing registered, three alternating rounds of `-count=5`. Allocations are equal in every round. Pooled:

  | Benchmark | ns/op | B/op | allocs/op |
  | :--- | :--- | :--- | :--- |
  | `BenchmarkPromptRoundTrip` | 26454 → 26422 (−0.1%) | +7 | 99 → 99 |
  | `BenchmarkSessionUpdateNotify` | 14113 → 14167 (+0.4%) | −1 | 65 → 65 |
