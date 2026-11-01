---
status: in-progress
date: 2026-11-01
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
   message with sending middleware, none for receiving middleware, and none for the observer. *Amended by deviation 2:* three
   per request sent, two per notification.
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

### Phase 8: when the observer runs, proposed 2026-11-01

The MADR's "Amendment, 2026-11-01: when the observer runs", option (a), as the owner answered on 2026-11-01, with Q3 (a), a
patch `v1.31.1`. Proposed: it runs only on the owner's approval, after
[0047-PLAN-pre-add-check-full-coverage.md](0047-PLAN-pre-add-check-full-coverage.md) is complete, and this PLAN returns to
`in-progress` then. Base: `cc2a72d`, whose Go code is `v1.31.0`'s; the step that begins it re-pins every line it names.

1. **The test first,** `TestObserverRunsAfterActing` in `connection_observer_test.go`. Each subtest runs in a synctest bubble,
   as `TestObserverReportsEachKind` does, and its observer records, as it is called, what the connection has done by then:
   * **a fault** (invalid JSON), **a refusal over the concurrency limit**, and **a refusal past the byte budget**: the
     connection's writer is wrapped in a recorder that appends each line once its `Write` returns; the observer requires the
     recorder to hold the answer to that id already;
   * **a cancel sent:** the recorder holds the `$/cancel_request` naming the id;
   * **a cancel received:** a held request with id 9 runs; the observer requires that request's handler context to be done;
   * **a drop under `OverflowDropNewest`:** a `WithNotificationDropHandler` that counts; the observer requires the count to be
     1 already;
   * **the overflow that closes the connection:** the observer requires `Done()` to be still open.
2. **Seen to fail,** `tools/plants.py`, one plant per order, each in the tool's copies:
   * `answerObserved` observing before `answer`;
   * `ObservedCancelSent` observed before `notifyDirect`;
   * `ObservedCancelReceived` observed before `handleCancelRequest` runs: the id taken from the params first;
   * `dropNotification` observing before the drop handler;
   * the overflow observed after `closeWith`.
   Each `want` is its subtest's own message.
3. **The godoc,** `WithObserver` in `observer.go`, after "fn runs synchronously on the goroutine that met the message …": fn runs
   after the connection has acted on the message: after the answer to a fault or a refusal is written, after the request a
   `$/cancel_request` names is cancelled, after the connection's own `$/cancel_request` is written, after the drop handler, and
   after a response is found to match no call; for the notification whose overflow closes the connection, before it closes. A peer
   can therefore read an answer before fn has run for it, and a program that counts refusals waits for fn, not for the answer.
4. **The README,** "What the chains do not see": one sentence with the same rule, beside the paragraph on where the observer
   runs, and the refusal-counting example's comment unchanged.
5. **Checks:** `go test -race -count=3 -run 'Observer' .`; `make pre-add-check FILES="observer.go connection_observer_test.go
   README.md"`, 0047's check, which runs both modules uncached; `tools/gates.py --all`; `make api`: 0 incompatible, 0 compatible; the record, markdown and disclosure checks; and the
   execution record, then staging.
6. **The release, `v1.31.1`, a patch** (the MADR's Q3, answered (a)), each step on the owner's ask:
   `docs/reports/0046-REPORT-release-notes-v1-31-1.md` and its index row; the push and its CI; `tools/release_body.py`; the
   owner's tag and release; `tools/publication.py` and `tools/consumer.py v1.31.1`. `tools/consumer.py` itself is unchanged:
   it already waits for the observer. Then this PLAN and its row `complete` again.

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

The owner committed phase 4 as `3f7f4b1`.

**Deviation 2, 2026-10-31, step 5.4: a request through sending middleware costs three allocations, not two.**

* *Found.* `BenchmarkPromptRoundTripMiddleware`, one pass-through middleware each way on both sides, gave 102 allocations per
  round trip against `BenchmarkPromptRoundTrip`'s 99, and 363 bytes more. The MADR's D11, and step 5.4, said two per sent message.
* *The cause.* `go build -gcflags=-m` reports `resp.msg.Result escapes to heap` at both of `sendInnermost`'s result returns
  (`middleware.go:243`, `:252`): the raw result, a slice, is boxed as the `any` a `MethodHandler` returns.
  * The three are the call's state, the context that carries it, and that box.
  * A notification, whose innermost step returns `nil`, costs two. A scratch copy with a notification benchmark through
    sending middleware gave 67 allocations against 65, and about 340 bytes more.
  * Receiving middleware and an observer add none: `BenchmarkSessionUpdateNotifyObserved` has 65.
* *Options put to the owner:*
  * amend D11 to the measured cost, keeping D4's contract (recommended);
  * fold the call into its context, one type that is both, for two per request;
  * return a `*json.RawMessage` into the call, for two per request, changing what middleware sees.
* *Decided.* The owner chose the first. The MADR's D11 and its "Neutral" consequence carry annotated amendments. Step 5.4 is
  annotated, and no code changed.

### Phase 5, 2026-10-31

The owner said "committed, proceed".

**What changed.**

* **`README.md`,** 140 lines added: "Middleware and observing", after "Closing and limits":
  * the type and the two options, where each chain sits, the order and its exception, `RequestIDFromContext`, and the rules;
  * the tracing middleware, `ExampleWithSendingMiddleware`'s body between `<!-- example: ExampleWithSendingMiddleware -->`
    markers;
  * "What the chains do not see": the observer, a table of its six kinds, where it runs, and a metrics sketch that counts
    refusals;
  * a sentence that tells this from `remote`'s HTTP middleware.
* **`docs_test.go`:** `TestReadmeMiddlewareIsTheExample`, as `TestReadmeRemoteIsTheExample` binds its example.
* **`docs/architecture.md`:**
  * a paragraph and list on the chains, the call, the innermost step and the observer's sites;
  * `middleware.go` and `observer.go` in the hand-written files;
  * the three test files in the checks;
  * "`exchange` and `SendRequestNoResult` count" became "`exchangePrepared`, which every request takes, counts", since phase 1
    made it so.
* **`example_api_test.go`:** `ExampleWithReceivingMiddleware`, `ExampleWithSendingMiddleware` and `ExampleWithObserver`.
* **`bench_test.go`:** `benchPair` takes options. `BenchmarkPromptRoundTripMiddleware` and
  `BenchmarkSessionUpdateNotifyObserved` use `benchPass`, a middleware that wraps `next` in a call of its own.
* **The records:** deviation 2, in this PLAN and the MADR, and the index row.

**Refinements, not deviations.**

* *Trailing comments in the README's example.* `exampleBody` prints with `go/printer`, which pads a trailing comment with tabs,
  and then turns each tab into four spaces, so a trailing comment never lines up with gofmt's. The first runs of
  `TestReadmeMiddlewareIsTheExample` failed on the three trailing comments, which moved onto lines of their own. A scratch
  script writes the README's block from the example's source.
* *The benchmark's pattern.* With the new benchmarks, `PromptRoundTrip|SessionUpdateNotify` matched four in the tree and two in
  the base. That comparison gave `SessionUpdateNotify` +9.8% pooled, on a phase that changed no code outside tests and docs. The
  pattern was anchored to the two benchmarks, and the comparison run again, below.

**Step 5.3, seen to fail.** `tools/plants.py`, four plants, "4/4 as required":

| Break | Test | Failure seen |
| :--- | :--- | :--- |
| the receiving middleware neutralized | `ExampleWithReceivingMiddleware` | got `{"jsonrpc":"2.0","id":1,"result":{"ok":true}}`: the method was not refused |
| the sending middleware neutralized | `ExampleWithSendingMiddleware` | got `peer read: …"params":{"n":1}}`, without `_meta`, and no line from the middleware |
| the observer neutralized | `ExampleWithObserver` | got nothing |
| the README's block changed by one space | `TestReadmeMiddlewareIsTheExample` | "README.md's middleware example is not ExampleWithSendingMiddleware's body." |

Each option was replaced by one that does nothing but keeps its variable used, so each copy compiles.

**Step 5.4, the benchmarks,** `-count=10` on the tree:

| Benchmark | ns/op | B/op | allocs/op |
| :--- | ---: | ---: | ---: |
| `BenchmarkPromptRoundTrip` | 30002 | 5437 | 99 |
| `BenchmarkPromptRoundTripMiddleware` | 29061 | 5800 | 102 |
| `BenchmarkSessionUpdateNotify` | 15288 | 4878 | 65 |
| `BenchmarkSessionUpdateNotifyObserved` | 16991 | 4877 | 65 |

The middleware's time is within the run's noise; its cost shows in allocations and bytes, as deviation 2 records.

**Step 5.5, the checks.**

* `make pre-add-check`: "go-precheck: 284 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck)."
* `make api`: "against v1.30.0: 0 incompatible, 14 compatible", unchanged from phase 4.
* At the floor, with `GOEXPERIMENT=goroutineleakprofile`, the three examples and `TestReadmeMiddlewareIsTheExample`: `ok`.
* `tools/remote.py pkg` on the Linux and Windows hosts, the three examples with `-test.count=3`: "PASS", exit 0, on each. The
  README's test reads the source, which `pkg` does not send, so it runs on macOS and in CI.
* **The benchmark against the baseline,** anchored, three alternating rounds of `-count=5`. Allocations are equal in every round.
  Pooled:

  | Benchmark | ns/op | B/op | allocs/op |
  | :--- | :--- | :--- | :--- |
  | `BenchmarkPromptRoundTrip` | 26509 → 26487 (−0.1%) | +4 | 99 → 99 |
  | `BenchmarkSessionUpdateNotify` | 14778 → 15016 (+1.6%) | +0 | 65 → 65 |

  Round 3 alone ran −16.6% and +13.6% in opposite directions, which is the machine's noise.
* `make check-records`, `make markdownlint` and `make disclosure-check`: exit 0.

The owner committed phase 5 as `f8e7ecb`.

### Phase 6, 2026-10-31

The owner said "committed, proceed".

**Deviation 3, 2026-10-31, step 6.1: the engine's runtime files lacked `middleware.go` and `observer.go`.**

* *Found.* `tools/gates.py --all` on `f8e7ecb`: fifteen gates `ok`; `floor`, `windows` and `linux` failed, each in `cmd/generate`.
  * `TestEngineBehaviour` failed every engine case: "generated code does not type-check … connection.go:510:16: undefined:
    Observation … undefined: Middleware … undefined: notFound … undefined: chainMiddleware".
  * `TestRuntimeFilesCoverHandWritten` failed: "hand-written files of the root are […, middleware.go, observer.go, …], want
    …".
  * The engine compiles a synthetic schema's generated code against `enginetest.RuntimeFiles`
    (`cmd/generate/internal/enginetest/api.go:26`), the root's files that use no ACP type. It copied `connection.go`, which since
    phase 2 calls code in `middleware.go` and, since phase 4, in `observer.go`, and copied neither.
  * The guard test names this case. It failed from phase 2's commit on.
* *Why the phases' checks missed it.* Given changed files, `scripts/go-precheck.sh` tests only their own module's packages
  (`:189-200`), so a change to the root's files never runs `cmd/generate`'s tests, which compile them. Only the `floor` and
  host gates run both modules, and step 6.1 is where this PLAN runs them. The gap predates 0046.
* *Nothing was pushed:* `main` was six commits ahead of `origin/main`.
* *Options put to the owner:*
  * add the two files to `RuntimeFiles`, and record the precheck's gap for a follow-up of its own (recommended);
  * also make `go-precheck.sh` test `cmd/generate` when a root runtime file changes, in this PLAN;
  * move the two files' code into `connection.go`.
* *Decided.* The owner chose the first. A scratch copy with the two names added passed every `cmd/generate` package under
  `-race`, and no engine case's `want.api` changed. Both files use no ACP type, the list's rule.
* *Changed.* `cmd/generate/internal/enginetest/api.go` is added to this phase's scope: `RuntimeFiles` names `middleware.go` and
  `observer.go`, and its comment cites this entry. No decision of the MADR moves.
* *Left for a follow-up record:* `go-precheck.sh`'s module selection. A root file that `RuntimeFiles` names should also run
  `cmd/generate`'s tests, so that a change like phase 2's fails before it is staged.
  * *2026-11-01:* that record is [0047-MADR-pre-add-check-full-coverage.md](0047-MADR-pre-add-check-full-coverage.md). Its
    research found the gap wider, and the `precheck` gate's pass on `f8e7ecb` above explained by go test's cache; the check now
    tests both modules whole and uncached.

**Step 6.1, the gates,** after the fix, `tools/gates.py --all`, exit 0:

```text
ok   regen (exit 0)
ok   precheck (exit 0)
  newfiles: no new .go files
ok   newfiles (exit 0)
ok   nojsonv2 (exit 0)
ok   winvet (exit 0)
ok   records (exit 0)
ok   mdlint (exit 0)
ok   idscan (exit 0)
ok   tools (exit 0)
ok   lint (exit 0)
ok   fix (exit 0)
ok   tidy (exit 0)
ok   api (exit 0)
ok   race3 (exit 0)
ok   floor (exit 0)
ok   platforms (exit 0)
ok   windows (exit 0)
ok   linux (exit 0)
all gates passed
```

* `precheck`: "go-precheck: 284 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck)."
* `api`: "against v1.30.0: 0 incompatible, 14 compatible".
* `floor`: "floor gate at go1.26.10: passed", both modules.
* `windows` and `linux`: the whole tree from source, `go1.27.3`, the root module and `cmd/generate` each "exit 0".

**Step 6.3, the MADR's Confirmation.**

| # | Item | Shown by |
| :--- | :--- | :--- |
| 1 | Receiving middleware wraps requests and notifications, first outermost across options; a `nil` middleware or handler panics at construction; a `nil` handler under a chain answers `-32601` and drops notifications silently | `TestReceivingMiddlewareOrder`, `TestReceivingMiddlewareConstruction`, `TestReceivingMiddlewareNilHandler`, `TestOptionSemantics` |
| 2 | A receiving middleware's error answers the request, and the dispatch and handler do not run | `TestReceivingMiddlewareShortCircuit`; `ExampleWithReceivingMiddleware` |
| 3 | Raw params before the hooks, the result after `checkResult`; params passed on are what the hooks and handler see | `TestReceivingMiddlewareSeesRawAndResult` |
| 4 | Every sending path, `Initialize` included, passes the chain; encoded params, raw result, `_meta`; a replaced result decoded and checked; the SDK's `$/cancel_request` outside it | `TestSendingMiddlewareAllPaths` (eleven paths), `TestSendingMiddlewareOrder`, `TestSendingMiddlewareResult`, `TestSendingMiddlewareSkipsCancelRequest`, `TestSendingMiddlewareSentFlag`; `ExampleWithSendingMiddleware` |
| 5 | Each misuse of D10 fails with `ErrSendingMiddleware` and writes nothing | `TestSendingMiddlewareMisuse`, five subtests |
| 6 | `RequestIDFromContext` on both sides, `false` for notifications, the sent call's id inside a received request's handler | `TestRequestIDFromContextReceiving`, `TestRequestIDFromContextSending` |
| 7 | The barrier, the order, 0044's guard and 0030's record hold; the existing tests pass unchanged | `TestReceivingMiddlewareKeepsOrdering`, `TestReceivingMiddlewareCountsInBudget`, `TestSendingMiddlewareKeepsBarrier`, `TestSendingMiddlewareShortCircuitInitialize`, `TestSendingMiddlewareInitializeRecord`; `git diff --numstat 523f319` removes no line from any existing test but `benchPair`'s signature, and the suite passes in `race3`, `floor` and on both hosts |
| 8 | The observer reports each kind once, with method, id, size and error, never under a lock; nothing either chain sees; never called when unset | `TestObserverReportsEachKind` (fifteen subtests), `TestObserverNoLockHeld`, `TestObserverAndChainsDisjoint`, `TestObserverUnset`, `TestObserverCopiesID`, `TestWithObserverLastWins`; `ExampleWithObserver` |
| 9 | With nothing registered, the same allocations and no time change beyond noise | phases 1 to 5's alternating benchmarks against `523f319`: 99 and 65 allocations in every round, pooled medians within 5%; `TestObserverUnset` |
| 10 | `make api`: compatible additions only | the `api` gate: 14 compatible, 0 incompatible |
| 11 | `tools/gates.py --all` and CI pass | step 6.1; CI in step 6.2 |

**The tests no plant had reached, planted now,** `tools/plants.py`, eleven plants on the tree with deviation 3's fix, "11/11 as
required":

| Break | Test | Failure seen |
| :--- | :--- | :--- |
| `sendRequestSent` off the chain | `TestSendingMiddlewareAllPaths`, all eleven subtests run | "the middleware did not happen" in `Prompt`, `SendRequest`, a generated request, and each side's `CallExtension` |
| `sendRequestCommit` off the chain | the same | in `Initialize` |
| `SendRequestNoResult` off the chain | the same | in `SendRequestNoResult` |
| `SendNotification` off the chain | the same | in `SendNotification`, a generated notification, and each side's `NotifyExtension` |
| the sending chain built last-outermost | `TestSendingMiddlewareOrder` | `"c b a a b c", want "a b c c b a"` |
| a middleware's error after a recorded `initialize` dropped | `TestSendingMiddlewareInitializeRecord/an_error_after_next` | "Initialize = <nil>, want the middleware's -32603" |
| `initialize`'s committed result not returned | `TestSendingMiddlewareInitializeRecord/observed` | "Initialize = {…}, <nil>; want the answer" |
| `sent` taken as false | `TestSendingMiddlewareSentFlag/written` | `the peer read ["$/cancel_request"] after Prompt returned; session/cancel sent false, want true` |
| a cancel received unreported | `TestObserverReportsEachKind/a_cancel_received_whose_params_do_not_decode` | "cancel_received did not happen" |
| an unmatched response unreported | `TestObserverReportsEachKind/an_error_response_to_an_unknown_id` | "unmatched_response did not happen" |
| a repeated `WithSendingMiddleware` replacing the earlier | `TestOptionSemantics` | `repeated WithSendingMiddleware ran "c c", want all three, the first outermost` |

With phases 1 to 5's plants, every new test and subtest has now been seen to fail, save one site with no input that reaches it
(phase 4's record). `TestRuntimeFilesCoverHandWritten` was seen to fail by the first gate run above.

**Step 6.2, CI.** The owner pushed `fbbbd5c`, `1de8d98`, `df22f9e`, `f23e107`, `3f7f4b1`, `f8e7ecb` and `767ee32`;
`origin/main` is `767ee32`, moved from `523f319`. The push came before an agent run of `make push-guard` over those commits; it
passed the global pre-push guard the push runs. `tools/watch_ci.py --branch main 767ee32`, exit 0:

```text
watch_ci: 2 run(s) for 767ee32: 41317956982, 41317956970
watch_ci: run 41317956982 (main, push): completed success; watch exit 0
  completed success: docs
watch_ci: run 41317956970 (main, push): completed success; watch exit 0
  completed success: validate (windows-2025, 1.27.3)
  completed success: validate (macos-15, 1.26.10)
  completed success: validate (ubuntu-24.04, 1.26.10)
  completed success: validate (macos-15, 1.27.3)
  completed success: validate (windows-2025, 1.26.10)
  completed success: validate (ubuntu-24.04, 1.27.3)
```

Confirmation item 11 is met. Phase 6 is done.

**Step 6.4, the status lines.** The MADR stays `accepted`. This PLAN stays `in-progress` until phase 7, the release, ends, and
the index rows are unchanged.

### Phase 7, 2026-11-01: before the tag

The owner committed phase 6's CI record as `27afb8f` and said "committed, proceed".

**Step 7.1, `tools/consumer.py`.**

* A second pair of side connections over pipes: the agent side with a receiving middleware, `WithMaxConcurrentRequests(1)` and
  an observer; the client side with a sending middleware. Each middleware counts the methods it sees, keyed by side, and keeps
  the id `RequestIDFromContext` gives; the observer counts kinds by `String`.
  * The client initializes. It then calls `_x/hold`, which the receiving middleware holds until released, and while it is held
    calls `_x/fast`, refused over the limit. It calls `_x/misuse`, which the sending middleware passes on as `_x/other`.
  * `WANT_RUN` gains four lines:
    * `middleware <nil> <nil> map[received _x/hold:1 received initialize:1 sent _x/fast:1 sent _x/hold:1 sent _x/misuse:1
      sent initialize:1]`: the receiving chain saw neither the refused request nor the misuse, which was never written;
    * `context id 1 1 3 false`: `initialize`'s id on both sides, `_x/fast`'s in the sending chain, and `false` for a context
      the connection did not make;
    * `observed map[refused:1]` and the refusal's `-32603`, `{"error":"too many concurrent requests","limit":1}`;
    * `misuse true <nil> <nil>`: `errors.Is` of the misuse's error for `acp.ErrSendingMiddleware`, and both closes.
  * The docstring names this PLAN and says `VERSION` is `v1.31.0` or later. `tools/README.md`'s row names the middleware
    options, the observer, `RequestIDFromContext` and `ErrSendingMiddleware`, and `v1.31.0`.
* *The observer's timing, found and fixed in the program.* The first three rounds through a `replace` had one whose run printed
  `observed map[]`. `answerObserved` writes the refusal, then calls the observer, and `WithObserver`'s godoc promises no order
  between the peer reading the answer and the observer running, so the client could print the counts first. That is the program's
  race, not the SDK's: the observer now signals `acp.ObservedRefused` on a channel of one, without blocking, and the program waits
  for it under its 10-second context before it releases `_x/hold`. Neither godoc nor README says the observer runs after the
  answer is written; this record notes it for a later doc change and changes nothing in the SDK.
* *Before the tag.* A scratch script built the program and test against an export of `27afb8f` through a `replace`, at
  `go1.26.10`, three times, and ran the built program 50 times more in each round: `go mod tidy`, the build, the run, `go test`
  and the Windows build, each exit 0; "output equals WANT_RUN: True" in each round, no repeat differing; "all rounds as
  required".
* *Seen to fail, three times.*
  * `tools/consumer.py v1.30.0`, the published release without middleware: exit 1, `go build` "undefined:
    acp.RequestIDFromContext", "undefined: acp.Observation", "undefined: acp.ObservedRefused", "undefined:
    acp.WithReceivingMiddleware", "undefined: acp.WithObserver", "undefined: acp.WithSendingMiddleware", "undefined:
    acp.ErrSendingMiddleware"; "NOT AS REQUIRED".
  * Scratch copies of the export, through the same script: with `misuse`'s `withCause(ErrSendingMiddleware)` dropped, the run
    differs at `misuse false <nil> <nil>`; with `observe` returning early for `ObservedRefused`, the run ends at "observer: the
    refusal was not reported". "2/2 failed as required".
* `make check-tools`, exit 0: "Ran 58 tests" and "Ran 32 tests", both "OK"; "check-ci-filters: 0 problem(s)".

**Step 7.2, the release notes.**
[0046-REPORT-release-notes-v1-31-0.md](../reports/0046-REPORT-release-notes-v1-31-0.md), and its row in `docs/README.md`. It leads
with the two additions, the middleware and the observer, then their cost, then what maintainers meet. Its sources: the eleven
commits since `v1.30.0`, the five Go files changed outside the tests, the unchanged areas, `make api` on `27afb8f` ("0
incompatible, 14 compatible"), CI on `767ee32`, and the consumer before the tag.

**Left, each on the owner's ask:** the commit and push of this phase and its CI; the tag `v1.31.0`, the GitHub release from
`tools/release_body.py`, `tools/publication.py`, and `tools/consumer.py v1.31.0`; then this PLAN and its row `complete`.

### Phase 7, 2026-11-01: the release

The owner committed step 7.1 and 7.2 as `093649b` and pushed it.

* **CI on `093649b`,** `tools/watch_ci.py --branch main 093649b`, exit 0: run `41319327566`, "completed success", the six
  `validate` jobs on macOS, Ubuntu and Windows at Go `1.27.3` and `1.26.10`; run `41319327653`, the Docs workflow, "completed
  success".
* **The body,** `tools/release_body.py v1.31.0 093649b`, exit 0: "107 lines, 2 links", its identifier scan "0 match(es)".
* **The tag.** The owner pushed `v1.31.0`, annotated, naming `093649b`, as `git ls-remote` shows. Its CI,
  `tools/watch_ci.py --branch v1.31.0 093649b`, exit 0: run `41319985865`, "completed success", the same six jobs.
* **`tools/consumer.py v1.31.0`,** exit 0, "as required". At `go1.26.10`, from the proxy: `go get` of `v1.31.0`, `go mod tidy`,
  the build, the run, whose output equals `WANT_RUN`, `go test` and the Windows build, each exit 0. The consumer's `go.mod`
  keeps `go 1.26.10` and has no toolchain line.
* **The release.** The owner ran `gh release create v1.31.0 --verify-tag --title v1.31.0 --notes-file` with the extracted body:
  <https://github.com/maccavelli/acp-go-sdk2/releases/tag/v1.31.0>.
* **`tools/publication.py v1.31.0 093649b`** with that body, exit 0, "10 of 10 checks hold": the release exists, not a draft or
  prerelease; its body is the extracted release text; it is marked latest; the tag is annotated and names `093649b`, which is on
  origin's `main`; CI passed on `main`'s head, run `41319327566`, and on the tag, run `41319985865`; the proxy serves the
  `.info`, its origin the tag at that commit; `go list -m @latest` gives `v1.31.0`; `go mod download` of the tag passes with the
  checksum database on.

Every item of the MADR's Confirmation was shown in phase 6, and CI and the release pass. The release is published, so this PLAN
and its row are `complete`. Left for later records, each on the owner's ask: `scripts/go-precheck.sh`'s module selection
(deviation 3), and a line in `WithObserver`'s godoc and the README saying the observer runs after a refusal's answer is written
(step 7.1).

*2026-11-01:* the first is [0047-PLAN-pre-add-check-full-coverage.md](0047-PLAN-pre-add-check-full-coverage.md); the second is
phase 8 above, proposed, to run after it.

### Phase 8, 2026-11-01: when the observer runs

The owner committed 0047's close as `a863ba1` and said "committed, proceed". The MADR's amendment is accepted, and this PLAN and
its row are `in-progress` again. No Go file changed between `v1.31.0` (`093649b`) and `a863ba1`
(`git diff --stat 093649b a863ba1 -- '*.go'` is empty), so every line the amendment cites holds as written.

**Step 8.1, `TestObserverRunsAfterActing`,** in `connection_observer_test.go`, 196 lines added:

* `written` wraps the connection's writer and keeps each write once it returns; `acting` holds what the observer inspects: the
  connection, `written`, the context of the request `_x/hold` runs (`atomic.Pointer`), and the drop handler's count; its handler
  holds `_x/hold` until released or cancelled, and `_x/n` until released.
* Seven subtests, each in a synctest bubble, its observer recording what it finds when first called for the case's kind:
  * a fault (`{oops`): the `-32700` answer already written;
  * a refusal over the concurrency limit, and one past the byte budget: `"id":2,"error"` and `"id":3,"error"` already written;
  * a cancel received for a held request with id 9: that request's context already done;
  * a cancel sent after `SendRequestNoResult`'s context ends: `"$/cancel_request"` and `"requestId":1` already written;
  * a notification dropped under `OverflowDropNewest`: the drop handler already called;
  * the overflow that closes the connection: `Done()` still open.
* `go test -race -count=3 -run '^TestObserverRunsAfterActing$' -v .`: each of the seven subtests "PASS" three times; `ok`.

**Step 8.2, seen to fail,** `tools/plants.py`, five plants, "5/5 as required", each built from the source's own lines by a
scratch script (`plants_p8.py`):

| Break | Failed | Message |
| :--- | :--- | :--- |
| `answerObserved` observes before it answers | `a_fault`, both refusal subtests | "fault: the observer ran before the fault's answer was written"; "refused: the observer ran before the refusal was written" |
| the cancel sent observed before `notifyDirect` | `a_cancel_sent` | "cancel_sent: the observer ran before the $/cancel_request was written" |
| the cancel received observed before `handleCancelRequest` | `a_cancel_received` | "cancel_received: the observer ran before the request it names was cancelled" |
| `dropNotification` observes before the drop handler | `a_notification_dropped` | "dropped: the observer ran before the drop handler" |
| the overflow observed after `shutdownReceive` | `the_overflow_that_closed_the_connection` | "dropped: the observer ran after the connection closed" |

**Step 8.3, the godoc,** `WithObserver` in `observer.go`, 9 lines: a paragraph after the one on where fn runs, stating the order
at each site, the overflow's exception, and that a program counting refusals waits for fn, not for the answer, citing the
amendment.

**Step 8.4, the README,** "What the chains do not see": the paragraph on where the observer runs ends at the `nil` check; a new
paragraph states the order and the exception, and leads into the refusal-counting example, unchanged.
`go test -count=1 -run 'Readme|Doc|Example|Exported' .`: `ok`.

**Step 8.5, checks:**

* `go test -race -count=3 -run 'Observer' .`: `ok`.
* `make pre-add-check FILES="observer.go connection_observer_test.go README.md"`, 0047's check, exit 0: "go-precheck: 2 file(s)
  gofmt-clean; both modules linted, vetted and tested (-race -shuffle=on, uncached), tidy, govulncheck."
* `make api`, exit 0: "against v1.31.0: 0 incompatible, 0 compatible".
* `tools/gates.py --all`, the hosts set per command, exit 0: all eighteen gates `ok`, `regen` to `linux`, "all gates passed".
* `make check-records`, `make markdownlint`, `make disclosure-check`, each exit 0.

No deviation. **Left, each on the owner's ask (step 8.6):** the commit and push of this phase and its CI; then
`docs/reports/0046-REPORT-release-notes-v1-31-1.md`, `tools/release_body.py`, the owner's tag `v1.31.1` and release,
`tools/publication.py` and `tools/consumer.py v1.31.1`; then this PLAN and its row `complete`.
