---
status: accepted
date: 2026-10-31
decision-makers: the repository owner
consulted: the sources of Codex, Grok Build, Kilo Code and opencode; the ACP TypeScript, Rust and Python SDKs; coder/acp-go-sdk; golang.org/x/tools' jsonrpc2 and jsonrpc2_v2, sourcegraph/jsonrpc2 and vscode-jsonrpc, at the revisions under "Sources"
informed: issue #13
---

# Add back-pressure as a third notification overflow policy, and make it the default: pause the reader at the queue's capacity, and read on while the notification loop waits on the reader

## Context and Problem Statement

[Issue #13](https://github.com/maccavelli/acp-go-sdk2/issues/13), opened by the owner on 2026-10-30: "back-pressure, instead of dropping
updates when the queue is full", with the body "we need to find the best way to implement this."

A connection handles its inbound notifications one at a time, in order, from a bounded queue. When a notification handler falls behind
and the queue fills, the connection today either closes (the default) or drops the notification. Neither tells the peer to slow down.
The owner asked for the most idiomatic way to do that, considering "an async or a separate goroutine dedicated for that flow", and for
the practice of the Grok, Codex, Kilo and opencode sources to be weighed.

**The question.** How should a connection whose notification handler falls behind make its peer slow down, losing nothing and staying
open, without the deadlock that a reader waiting on a full queue invites?

**Status.** Accepted. The owner answered the six questions on 2026-10-31 (see "Decided with the owner, 2026-10-31"): five as
recommended. The sixth, the default, went against the recommendation: `OverflowBlock` becomes the default. The design below
includes that answer (D1, D6, D9). Nothing is implemented.

### Findings in this repository

* **F1. One reader, one notification loop.** `receive` (`connection.go:1319`) reads every line of the peer's output.
  * A response is delivered on the reader itself (`receiveResponse`, `:1672`, then `handleResponse`, `:1770`).
  * A request runs on its own goroutine (`:1511`-`:1574`).
  * `$/cancel_request` is handled on the reader, "so cancellations take effect immediately" (`:1489`-`:1494`).
  * A notification goes into `notificationQueue`, a channel of capacity `n` (`WithMaxQueuedNotifications`, default 1024: `:23`,
    `:471`, `:729`), which one goroutine, `processNotifications` (`:1748`), drains one at a time.

  The SDK already has the "separate goroutine dedicated for that flow" the owner names: everything but notifications is already off it.
* **F2. A full queue today.** The reader's send is a `select` with a `default` (`:1582`-`:1613`).
  * `OverflowCloseConnection`, the default, closes the connection with `ErrNotificationQueueOverflow` (`:1605`-`:1612`,
    `errors.go:206`).
  * `OverflowDropNewest` discards the notification and counts it (`:1597`-`:1604`). It came in `eb6e808` (2026-09-22), and its
    comment states the constraint this record answers: "The reader must not block here: responses are dispatched on this goroutine
    too."
  * The policy type's documentation already says "More policies may be added in the future." (`:544`). An undefined value is the
    default (`:571`-`:574`; `TestConnection_NotificationOverflowPolicyClosesByDefault`, `connection_overflow_policy_test.go:135`).
* **F3. Every response already waits for the notifications before it.** `handleResponse` records a watermark, the number of
  notifications received before the response (`:1788`-`:1796`). Each typed or untyped request returns only once those are handled:
  `sendRequestSent` (`:1984`), `sendRequestCommit` (`:2049`) and `SendRequestNoResult` (`:2451`) call `waitNotificationsUpTo`
  (`:2333`). This is the response barrier of `3a19dcb` (#30, 2026-04-20). So a caller whose response is read late, because the reader
  paused behind the same notifications, returns no later than it does today: at the moment the last notification before its response
  is handled. F9 shows it.
* **F4. The barrier has two exemptions, and each one waits on the reader.**
  * **A request a notification handler sends under its own context.** It waits only for the notifications before that handler
    (`:2347`-`:2353`; [0019-PLAN-behaviour-fixes.md](0019-PLAN-behaviour-fixes.md), B1). The handler waits for its response; only
    the reader can read it.
  * **`initialize` in flight.** The reader commits and settles the answer before the barrier (`:2008`-`:2022`), because messages the
    agent sends before its answer are held until it is judged: `heldGate` (`client_gen.go:112`) waits in `holdUntilInitialized`
    (`connection.go:2090`), on the notification loop for a notification. "A held notification waits in the notification queue, with the
    notifications behind it" (`README.md:497`-`:502`; `TestHeldNotificationsFillQueue`, `initialize_test.go:381`). Only
    `Initialize` uses `sendRequestCommit` (`client_gen.go:537`).

  A reader that waited for room while either wait is running waits for itself. F9 shows both.
* **F5. The outbound side already back-pressures.** `sendMessage` waits for its write, on the one writer goroutine
  (`connection.go:1894`-`:1898`; `docs/architecture.md:445`-`:448`), and "Every send returns when its context ends, even while the peer
  does not read" (`README.md:836`). An agent whose client stops reading is slowed at its `SessionUpdate` calls, and a
  `session/cancel` still ends a turn blocked in one, since the turn's context ends.
* **F6. What carries the bytes decides how far the pause reaches.**
  * A child's stdio: the OS pipe fills, then the agent's writes wait.
  * `agentproc.InProcess`: `io.Pipe`, which holds nothing, so the agent's write waits at once.
  * `remote`, client side: `lineQueue` holds 256 lines, then its push waits (`remote/frame.go:104`-`:139`), and TCP carries the pause to
    the server.
  * `remote`, server side, WebSocket: each frame is written under `Grace` (`remote/ws.go:159`-`:169`), 5 s by default
    (`remote/handler.go:114`-`:115`). A client paused longer ends the connection there.
  * `remote`, server side, Streamable HTTP: lines are held for a stream up to `MaxBuffered`, 64 MiB by default, then the connection
    ends with `ErrOverflow` (`remote/httpserver.go:405`-`:412`, `remote/handler.go:45`-`:48`).
* **F7. On an agent, the queue also carries `session/cancel`.** The drop policy's documentation warns that dropping one "leaves the
  agent working on a turn its client has abandoned" (`connection.go:565`-`:567`).
* **F8. The bound is a count, not bytes.** A queue of 1024 notifications of up to 10 MiB each (`defaultMaxMessageSize`, `:26`) can
  hold about 10 GiB.
* **F9. Probes, on a scratch copy of this tree, not committed.** A naive blocking send (policy value 2: wait for room, or the
  connection's end) and the design below (policy value 3) were added to a copy, with three probes. Each probe ran three ways, under
  `go test -race`; the root package's full suite also passed on the copy with the prototype in it.

  | Probe | Room for all (today, queue of 100) | Naive blocking send (queue of 2, 1) | This design (queue of 2, 1) |
  | :--- | :--- | :--- | :--- |
  | A request whose response follows four notifications, the first held | returns only after the release; order 0 1 2 3 | the same | the same |
  | Notification 0's handler sends a request under its context; the answer follows three notifications | answered | **deadlock**: the request's 2 s deadline ends it | answered |
  | Three extension notifications before `initialize`'s answer, each held | `Initialize` succeeds | **deadlock**: `Initialize`'s 2 s deadline ends it | `Initialize` succeeds |
  | Seven notifications, the first held (this design only) | | | three read while held (one running, two queued), then all seven in order, none dropped, connection open |
  | Notification 0's handler waits on the peer, which sends six more (this design only) | | | reads on to four queued, then closes with `ErrNotificationQueueOverflow` |

  After the owner made the design the default (D9), it was made the copy's default too, and every package's tests were run under
  `-race`.
  * Exactly seven tests failed, each one that exercises the close without naming a policy, and six checks that read the queue's
    channel capacity.
  * `acptest`, `clienthost`, `remote` and the four examples passed, as did every other test of the root package and `agentproc`.

  The PLAN lists the thirteen.

### Findings in other implementations

The surveys read each source at the revision under "Sources". Citations are theirs; the five marked *re-checked* were read again for
this record.

* **E1. coder/acp-go-sdk, this SDK's origin, rejected a blocking queue in review.** PR #8's review (2025-12-09,
  [r2602080571](https://github.com/coder/acp-go-sdk/pull/8#discussion_r2602080571), *re-checked*): "a burst of >100 notifications or a
  slow handler fills the buffer and blocks this `receive` loop before it can read subsequent frames (including responses) … active
  requests can hang indefinitely". The same review rejected an unbounded queue ("unbounded memory growth, which is risky for a library
  default") and a fall-back to concurrent handling ("the ordering guarantee is silently broken"), and adopted the close. That review
  predates the response barrier (F3) by four months. coder later capped a handler's own request (#68, as F4's first exemption here) and
  released its inbound barrier while the running handler waits on the peer (#71, a `peerWaits` count). Its open issue #72, item 5,
  asks for a byte budget per connection: "A hostile peer can pin about 10 GiB today."
* **E2. golang.org/x/tools' `jsonrpc2_v2` explains why its queue is unbounded** (`internal/jsonrpc2_v2/conn.go:602`-`:608`, v0.52.0,
  *re-checked*): "we cannot in general limit the size of the handler queue: we have to read every response that comes in on the wire
  (because it may be responding to a request issued by, say, an asynchronous handler), and in order to get to that response we have to
  read all of the requests that came in ahead of it." Its v1 `AsyncHandler`, which gopls uses, chains one goroutine per message:
  "unbounded goroutines all stalled on the previous one" (`internal/jsonrpc2/handler.go:81`-`:107`).
* **E3. sourcegraph/jsonrpc2 blocks by default and says what that costs** (`jsonrpc2.go:60`-`:68`, v0.2.3): "While it is running, the
  connection does not process any other requests or responses. A handler that waits for a call's response must therefore run
  asynchronously."
* **E4. vscode-jsonrpc** queues without bound, and with limited parallelism resolves responses on the reader "to avoid dead locks"
  (`connection.ts:703`-`:712`, 10.0.0).
* **E5. The ACP SDKs never pause their readers.**
  * TypeScript 2.2.0: each message is a promise not awaited; the only bound is 32 MiB a message (`src/core/jsonrpc.ts:1022`-`:1096`,
    `src/core/stream-limits.ts:1`).
  * Rust 3.6.0: unbounded channels; a dispatch loop that awaits each callback, with the deadlock documented
    (`src/agent-client-protocol/src/concepts/ordering.rs:48`-`:65`). Its cookbook: "A bounded queue must not wait on a consumer that is
    itself awaiting later inbound traffic" (`ordered_application_dispatch.rs:124`-`:126`).
  * Python: a task per message (`src/acp/connection.py:168`-`:170`); its HTTP transport's outbound queue awaits when full, because
    "dropping a JSON-RPC response would permanently hang the peer's pending request" (`src/acp/http/server.py:73`-`:80`).
* **E6. Codex tried waiting on a bounded queue and deadlocked.** `e9996ec62a` (2026-03-25) preserved transcript events by awaiting a
  bounded queue; `6fc6b9d6d2` (2026-08-13, *re-checked*) undid it: "Awaiting a bounded consumer event queue can stall the in-process
  app-server worker when notifications are not being drained, preventing it from delivering a request response queued behind them."
  The fix is an unbounded, ordered last hop (`app-server-client/src/lib.rs:379`-`:383`). Its stdio app-server does wait on its inbound
  queue for responses and notifications (`app-server-transport/src/transport/mod.rs:241`-`:268`), safely, because its handlers never
  run inline. Its in-process runtime parks one must-deliver event and keeps a separate lane for control replies
  (`app-server/src/in_process.rs:701`-`:721`).
* **E7. opencode removed a reader that waited on its slowest subscriber.** `ac77cc46b8` (2026-08-31, *re-checked*, "isolate shared
  event consumers") replaced it with a bounded buffer per subscriber that fails that subscriber alone; `b073b052d3` (2026-09-19,
  *re-checked*) made a websocket's inbound queue unbounded because "The socket pushes frames synchronously and cannot be paused". Its
  ACP agent waits on each `sessionUpdate` write and batches model deltas every 100 ms before they reach it.
* **E8. Kilo Code records what an unbounded queue cost it.** `packages/opencode/src/kilocode/server/sse.ts:30`-`:35`: "Since
  session.diff events can contain complete, multi-megabyte patches, extension reconnects can retain tens of gigabytes in those queues."
  Its cloud stream bounds a queue by bytes, 8 MiB, and aborts when "consumer is too slow" (`websocket-stream.ts:7`-`:9`).
* **E9. Grok Build** uses the Rust ACP SDK 0.10.4, which spawns a task per notification over unbounded channels. Upstream of the parser
  it reads stdin on its own thread into a channel of 64 lines whose `blocking_send` "parks this thread … applying natural backpressure to
  a flooding peer" (`crates/codegen/xai-acp-lib/src/stdin_reader.rs:61`-`:90`); that wait is safe only because nothing behind it waits on
  a handler. It coalesces consecutive message chunks before sending them, and re-sends `session/cancel` because it treats it as
  loss-prone.

**What the evidence says.** No implementation surveyed pauses its inbound reader on a full notification queue, and where one gives a
reason, it is E1's and E2's: the response a caller waits for is behind the queue. Here that reason holds only at F4's two exemptions:
everywhere else the barrier already makes the caller wait for exactly those notifications (F3, F9). The other implementations read on
without bound (E2, E4, E5, E6), at the cost E8 records, or fail the slow consumer (E1, E7), which is this SDK's default.

## Decision Drivers

* Nothing lost, and no connection closed, because a handler is slow: the issue.
* No deadlock the SDK can see. Responses, a handler's own requests and the `initialize` hold keep working.
* Memory bounded by configuration, as `WithMaxQueuedNotifications` bounds it today.
* The pause reaches the peer's producer, an agent's `SessionUpdate` calls, rather than moving the backlog to another buffer.
* Compatibility. The default and both existing policies behave as before; `make api` reports additions only.
* Idiomatic Go. A bounded hand-off that waits, with the connection's end as the way out, as `remote`'s `lineQueue` does
  (`remote/frame.go:107`-`:111`); no goroutine per message.
* A pause that lasts is diagnosable from the log.

## Considered Options

* A third policy, `OverflowBlock`: the reader pauses at the queue's capacity, reads on while the notification loop waits on the reader,
  and closes at twice the capacity. Whether it is the default is a separate choice, decided with the owner (see "Decided with the
  owner, 2026-10-31").
* A plain blocking send: the reader waits for room or the connection's end, with no guard.
* An unbounded queue: the reader reads on and never pauses.
* A larger queue bounded by bytes, closing past the budget.
* A queue and a goroutine per session.
* Coalescing `session/update` chunks in the SDK when the queue is under pressure.
* Flow control in the protocol: credits or acknowledgements.

## Decision Outcome

Chosen option: "A third policy, `OverflowBlock`", made the default, because it is the only option that slows the peer while losing
nothing and keeping memory bounded. The probes show it adds no wait to a caller the response barrier does not already impose, and
removes the two deadlocks a plain blocking send has (F3, F4, F9). The owner made it the default, so a connection that sets no policy
no longer closes because a handler is slow. That is what the ACP SDKs and the other implementations surveyed do too, but here
memory stays bounded (E2, E4 to E6). A consumer that wants the close passes `OverflowCloseConnection`.

### The design

* **D1. API.** One exported identifier: `OverflowBlock`, a `NotificationOverflowPolicy` after `OverflowDropNewest`, set by the existing
  `WithNotificationOverflowPolicy`. It is the default (D9). The constants keep their values: `OverflowCloseConnection` stays `0`,
  `OverflowDropNewest` `1`, and `OverflowBlock` is `2`.
* **D2. The pause.**
  * When `n` notifications wait in the queue, the reader waits before reading on. The one whose handler is running is not counted, as
    today: of 1 to 5 sent while a handler is held, 1 to 4 fill a queue of 4.
  * It waits until one of these happens:
    * the loop takes a notification (it is woken as each handler returns);
    * the loop waits on the reader (D3);
    * the connection ends, by `Close` or a read-on overflow (D4). The reader then ends as at a failed read, through `shutdownReceive`
      with the connection's cause, and the notification in hand is not handled, as with `Close` today.
  * While it waits, nothing more is read: no response, no request, no `$/cancel_request`, no end of input. The transport fills, and the
    peer's writes wait (F5, F6).
* **D3. The guard.** The notification loop waits on the reader in exactly F4's two cases. Each is counted, under `notifyMu`, for as long
  as it lasts, and counting it wakes a paused reader. While the count is above zero, the reader does not pause; it reads on and queues
  past `n`.
  * A request sent under the context of the notification whose handler is running, or a context derived from it, counts from its
    registration until its answer or its failure. The test is the one `waitNotificationsUpTo` uses: the context's mark is a
    notification's, and that notification has not completed.
  * An `initialize` in flight counts from its registration until `settled` runs: on the reader once its answer is judged, or on the
    caller when no answer will come.
* **D4. The read-on limit.** At `2n` queued, a further notification closes the connection as `OverflowCloseConnection` does, with
  `ErrNotificationQueueOverflow`. The queue's channel has capacity `2n` under `OverflowBlock` and `n` under the other policies, so
  memory stays bounded by configuration, and the existing capacity tests are unchanged.
* **D5. Diagnostics.** A pause that lasts 5 s logs one `Warn`, with the queue's capacity and how long the reader has waited, so a handler
  that never returns is found from the log. A pause logs nothing before then.
* **D6. What is unchanged.**
  * `OverflowCloseConnection` and `OverflowDropNewest` behave as before when they are chosen.
  * An undefined value is still treated as the default, which is now `OverflowBlock`.
  * `DroppedNotifications` stays zero unless `OverflowDropNewest` is chosen.
  * `Err`, `EndedCleanly` and `ErrNotificationQueueOverflow` keep their meanings. Under the default, the overflow is reached only past
    the read-on limit (D4).
  * The writer, the barrier and `$/cancel_request` on the reader are untouched.
* **D7. The rule the SDK cannot check**, stated on `OverflowBlock`. Under it, a notification handler must not wait for a request or a
  `$/cancel_request` the peer sends later, nor for code that waits for one: while the queue is full, the reader does not read it. The
  rule the barrier already sets, that a handler must not wait for a goroutine that waits for a response on this connection, is
  unchanged (`MethodHandler`, `connection.go:300`-`:308`).
* **D8. Documentation.**
  * The godoc of `OverflowBlock`, `OverflowCloseConnection`, `WithNotificationOverflowPolicy`, `WithMaxQueuedNotifications`,
    `MethodHandler`, `ErrNotificationQueueOverflow`, `Err` and `EndedCleanly`, and `doc.go`'s "How a connection ends".
  * The README: "Closing and limits", the hold paragraph, and the `Err` section with its example.
  * `docs/architecture.md`.
  * An `ExampleWithNotificationOverflowPolicy`.
* **D9. The default.**
  * A connection given no policy, or one this package does not define, uses `OverflowBlock`, on both sides and through `ServeAgent`,
    `agentproc` and `remote`.
  * A consumer that relied on the close keeps it with one option: `WithNotificationOverflowPolicy(acp.OverflowCloseConnection)`.
  * Each existing test that exercises the close without naming a policy names `OverflowCloseConnection`, so it goes on testing what
    it tested. The test that pins the default pins `OverflowBlock`. The tests of the queue's capacity read the configured `n`, since
    under `OverflowBlock` the channel holds `2n`. The PLAN lists each.
  * The release is a minor one, `v1.29.0`. Its notes lead with this change and the one-line opt-out.

### Consequences

* Good, because nothing is lost and the connection stays open while a handler is merely slow, which is the issue's request.
* Good, because the pause reaches the agent: over stdio and in process, its `SessionUpdate` calls wait (F5, F6), so its own turn slows
  rather than a buffer growing.
* Good, because memory is bounded by configuration: at most `2n` notifications.
* Good, because code that keeps the barrier's existing rule meets no new deadlock (F9), and the SDK's own two waits on the reader are
  guarded.
* Good, because, as the default, it protects every consumer: one that never thought about a flood no longer loses its connection to a
  slow handler. `make api` still reports one compatible addition.
* Bad, because the default changes behaviour for every consumer that sets no policy. A handler that never returns used to close the
  connection after `n` more notifications. Now it pauses the connection, and with it the agent, until something ends the wait; the
  5 s `Warn` is the sign. A consumer that detected floods by `ErrNotificationQueueOverflow` must choose `OverflowCloseConnection`.
* Bad, because D7's rule now binds every consumer by default, not only those that chose the policy.
* Bad, because seven existing tests that exercised the close by default, and six capacity checks, change. The PLAN changes each to keep
  testing what it tested (D9), not to loosen it.
* Bad, because a pause holds every inbound message behind the queue. A peer's request, such as `session/request_permission`, and a
  `$/cancel_request` wait for room, where today they are read at once.
* Bad, because D7 is a rule the SDK documents and cannot enforce. A handler that breaks it waits until something ends the wait; the
  5 s log (D5) is the only sign.
* Bad, because over `remote` the pause reaches the server's own bounds, not the agent. A WebSocket client paused past the server's
  `Grace` (5 s) is disconnected, and Streamable HTTP holds up to `MaxBuffered` (64 MiB) before `ErrOverflow` (F6).
* Bad, because the bound is still a count (F8). A byte budget is a separate decision.
* Neutral, because on an agent `session/cancel` stays in the queue, in order, as today. Under this policy it is delayed by a full queue,
  never dropped.
* Neutral, because the reader gains one wait and one counter, under the lock it already takes for each notification.

### Confirmation

The PLAN, [0044-PLAN-notification-queue-back-pressure.md](0044-PLAN-notification-queue-back-pressure.md), confirms each of these, and
each new test is seen to fail on a planted break in a scratch copy:

1. With the queue full and a handler held, the reader stops reading: the peer's next write waits, and every notification is then
   delivered in order, none dropped, the connection open.
2. A request whose response is behind a full queue returns once the notifications before it are handled, as under a queue with room.
3. A notification handler's request under its own context is answered while the queue is full.
4. Notifications held for `initialize` past the queue's capacity do not stop `Initialize`.
5. While the loop waits on the reader, the reader reads on to `2n` and then closes with `ErrNotificationQueueOverflow`.
6. `Close` and `Shutdown` end a paused reader, `Done` closes, and `Wait` returns.
7. An agent whose client pauses is slowed at its `SessionUpdate` calls, and `session/cancel` still ends its turn.
8. A pause of 5 s logs one `Warn`.
9. A connection given no policy, or an undefined one, pauses as under `OverflowBlock`. One given `OverflowCloseConnection` closes as
   before: the existing close tests pass with that option named, and the barrier and hold tests pass unchanged.
10. `make api` reports one compatible addition, `OverflowBlock`, and no incompatible change.
11. `tools/gates.py --all`, the new tests on the Linux and Windows hosts, and CI pass.
12. The `v1.29.0` release notes lead with the change of default and its opt-out.

## Pros and Cons of the Options

### A third policy, `OverflowBlock`

* Good, because it slows the peer, loses nothing and bounds memory.
* Good, because the probes show it adds no wait to a caller beyond the barrier's (F9).
* Good, because the two waits the SDK itself has on the reader are guarded (D3).
* Good, because it fits the existing option, as its documentation anticipated (F2).
* Bad, because every inbound message waits behind a full queue, and D7 is documented, not enforced.

### A plain blocking send

* Good, because it is the simplest change: one `select` with the connection's end.
* Bad, because it deadlocks a handler's own request and the `initialize` hold (F9), both of which the SDK supports today.

### An unbounded queue

* Good, because the reader never waits, so nothing can deadlock on it, and nothing is lost. This is what x/tools, vscode-jsonrpc, the
  ACP TypeScript and Rust SDKs, and Codex's client do (E2, E4, E5, E6).
* Bad, because it is not back-pressure: the backlog moves into this process, and nothing slows the agent.
* Bad, because memory has no bound. E1's review rejected it for a library default, and E8 records tens of gigabytes held this way.

### A larger queue bounded by bytes

* Good, because it bounds what F8 leaves open, as E1's issue #72 and E8 recommend.
* Bad, because it is a bigger cliff, not back-pressure: past the budget it still closes or drops.
* Neutral, because it is worth having beside any policy, and is left to a separate decision.

### A queue and a goroutine per session

* Good, because one slow session would not hold another's notifications, which matters when many sessions share a connection.
* Bad, because it does not create back-pressure. Each session's queue still needs a policy when full, the same question per session.
* Bad, because it breaks the connection-wide order the barrier is built on (F3): a response would have to wait for its own session's
  notifications, and the generated dispatch would have to name a session for every notification.

### Coalescing `session/update` chunks in the SDK

* Good, because it cuts the number of updates, as opencode, Kilo and Grok do at their producers and their UIs (E7 to E9).
* Bad, because merging is ACP-specific and changes how many times a handler is called. It belongs to the agent that produces chunks, or
  to the client's own handler, not to the connection.

### Flow control in the protocol

* Good, because credits per session, as in HTTP/2, would slow one stream without stopping the others.
* Bad, because ACP defines no such messages. It needs a change to the specification, which this SDK does not own.

## Decided with the owner, 2026-10-31

The six questions were put to the owner with three options each and a recommendation; the answers are below.

1. **The default: `OverflowBlock`, against the recommendation.**
   * Options:
     * keep `OverflowCloseConnection` (recommended);
     * make `OverflowBlock` the default (chosen);
     * make it the default on client connections only.
   * The recommendation was to keep the close because changing the default changes behaviour for every consumer that set no policy.
   * The owner chose a default that loses nothing and keeps connections open. Its cost is recorded under "Consequences", and D9 says
     how it is carried out.
2. **The name: `OverflowBlock`, as recommended.** The other options were `OverflowPauseReading` and `OverflowWait`.
3. **The read-on limit: twice the capacity, as recommended (D4).** The other options were no limit while the loop waits on the reader,
   as Codex's last hop reads on, and no read-on at all, closing at `n` whenever the loop waits on the reader.
4. **A stall: the 5 s `Warn` only, as recommended (D5).** The other options were:
   * an opt-in `WithNotificationStallTimeout(d)` that closes after a pause of `d`, as the ACP TypeScript SDK's HTTP transport closes
     after a 60 s output stall;
   * a fixed timeout with no option.
   The guard (D3) covers every wait the SDK can see, and a caller with a deadline ends any other.
5. **`session/cancel`: left in the queue, as recommended (D6).** The other options were:
   * handle it on the reader now, as `$/cancel_request` is;
   * decide that in a separate record.
   Moving it changes how a cancel is ordered against a prompt, which
   [0017-PLAN-remediate-acp-v1-conformance-violations.md](0017-PLAN-remediate-acp-v1-conformance-violations.md) builds on receipts.
   Under the default it is delayed by a full queue, never dropped.
6. **The release: `v1.29.0`, flagged, as recommended (D9).** One minor release; the API is compatible, and the notes lead with the
   change of default and the opt-out. The other options were:
   * two releases: opt-in first, the default later;
   * a `/v2` module.

## More Information

### Sources

| Source | Revision |
| :--- | :--- |
| openai/codex (a clone) | `b91cedc080`, 2026-10-30 |
| Grok Build (a clone) | `2bdd1d6a`, 2026-09-29 |
| Kilo Code (a clone) | `0a9faeff69`, 2026-10-29 |
| opencode (a clone) | `237a847056`, 2026-10-29 |
| ACP TypeScript SDK, `@agentclientprotocol/sdk` | 2.2.0, `c959b06` |
| ACP Rust SDK, `agent-client-protocol` | 3.6.0, `fe1e126`; the older design at `v0.10.4` |
| ACP Python SDK | `9853d8b` |
| coder/acp-go-sdk | `b60322e`; PR #8 and issues #68, #71 and #72 |
| golang.org/x/tools, `internal/jsonrpc2` and `internal/jsonrpc2_v2` | v0.52.0 |
| sourcegraph/jsonrpc2 | v0.2.3 |
| vscode-jsonrpc | 10.0.0, `a0981e3` |

The probes of F9 ran on a scratch copy of this tree at `24e88d1`, outside the repository; PLAN phase 1 writes their committed form.

### Related records

* [0019-PLAN-behaviour-fixes.md](0019-PLAN-behaviour-fixes.md), B1: a request from a notification handler does not wait for that
  handler. D3's first case is its counterpart.
* [0030-MADR-record-initialize-on-arrival.md](0030-MADR-record-initialize-on-arrival.md), question 2 and its amendment of 2026-10-21:
  the hold. D3's second case is its counterpart.
* [0026-MADR-connection-end-cause-and-wait.md](0026-MADR-connection-end-cause-and-wait.md): `Err`, and the overflow's cause.
* [0043-MADR-remote-http-and-websocket-transport.md](0043-MADR-remote-http-and-websocket-transport.md): the `remote` bounds F6 names.

### Not in this decision

* A byte budget for what a connection keeps from its peer (F8; E1's issue #72, item 5).
* Queues per session, and `session/cancel` on the reader (question 5).
* Coalescing chunks, an agent's or a client's own concern.
* `remote`'s server bounds, which apply whatever the client's policy.
