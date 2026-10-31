---
status: complete
date: 2026-10-31
associated-madr: "0044-MADR-notification-queue-back-pressure.md"
---

# Implement 0044: `OverflowBlock`, back-pressure for the notification queue, as the default

Associated MADR: [0044-MADR-notification-queue-back-pressure.md](0044-MADR-notification-queue-back-pressure.md)

## Goal

Under `OverflowBlock`, a connection stops reading its peer while `n` notifications wait in its queue. The peer's writes then wait,
and nothing is dropped or closed. It reads on, to `2n`, while its notification loop waits on the reader, and logs a pause that lasts
5 s. `OverflowBlock` is the policy of every connection that names none.

That is the MADR's "The design", D1 to D9, as "Decided with the owner, 2026-10-31" settled it.

* The one exported addition is `OverflowBlock`; `make api` reports it as compatible and nothing else.
* `OverflowCloseConnection` and `OverflowDropNewest` behave as before when chosen.
* Each existing test that exercised the close by default names `OverflowCloseConnection`, and passes. The capacity checks read the
  configured `n`. Nothing else in the existing tests changes.
* Issue #13 is answered once `v1.29.0` is published, with notes that lead with the change of default.

## Prerequisites

* The MADR is `accepted`, with the owner's answers of 2026-10-31.
* The tree is at `24e88d1` or later, with no change to `connection.go`'s receive path since; the line numbers below are `24e88d1`'s.
* `tools/README.md`'s tools as they stand: `tools/gates.py`, `tools/remote.py`, `tools/watch_ci.py`, `tools/release_body.py`,
  `tools/publication.py`, `tools/consumer.py`.

## Scope

| File | Change | Phase |
| :--- | :--- | :--- |
| `connection.go` | `OverflowBlock` and the default; the pause, the guard, the read-on limit, the log; the godoc of D8 | 1 |
| `errors.go` | `ErrNotificationQueueOverflow`'s documentation: the close policy, and `OverflowBlock`'s read-on limit | 1 |
| `doc.go` | "How a connection ends" (`:27`-`:30`) | 1 |
| `connection_overflow_block_test.go` (new) | the tests of step 1.7 | 1 |
| the existing tests of step 1.8 | name `OverflowCloseConnection` where they exercise the close; capacity read as `n` | 1 |
| `example_api_test.go` | `ExampleWithNotificationOverflowPolicy` | 2 |
| `README.md` | "Closing and limits": the three policies and the default; the hold paragraph; the `Err` section and its example | 2 |
| `docs/architecture.md` | the notification queue's paragraph, and the checks list | 2 |
| `docs/decisions/0044-*.md`, `docs/README.md` | the execution record, the status, the index rows | each |
| `tools/consumer.py`, `tools/README.md` | a check that the published module offers `OverflowBlock` | 4 |
| `docs/reports/0044-REPORT-release-notes-v1-29-0.md` (new) | the release text and its sources | 4 |

Out of scope, as the MADR's "Not in this decision": a byte budget; queues per session; `session/cancel` on the reader; coalescing;
`remote`'s server bounds. The generated code (`client_gen.go`, `agent_gen.go`) does not change.

## Implementation Steps

### Phase 1: the policy, the pause and the guard

1. **The policy.** In `connection.go`, after `OverflowDropNewest` (`:568`), add `OverflowBlock`. Its documentation says:
   * When the queue holds `n` notifications, the reader stops reading until the notification loop takes one. Nothing is read
     meanwhile, responses, requests and `$/cancel_request` included, so the peer's writes wait; a response still returns once the
     notifications before it are handled, as under any policy.
   * While the loop waits on this connection, for the answer to a request a notification handler sent under its context or for
     `initialize`'s answer, the reader reads on, to `2n`. Past `2n` it closes as `OverflowCloseConnection` does.
   * A pause of 5 s is logged at `Warn`.
   * The rule of the MADR's D7: a notification handler must not wait for a request or a `$/cancel_request` the peer sends later.
   * Over `remote`, the server's `Grace` and `MaxBuffered` still apply.
2. **The default and the documentation.**
   * `OverflowCloseConnection`'s documentation (`:548`-`:551`) loses "This is the default."; `OverflowBlock`'s says it is the
     default.
   * `WithNotificationOverflowPolicy` (`:571`-`:574`): the default is `OverflowBlock`, and an undefined value is treated as the
     default. Choosing `OverflowCloseConnection` keeps the close.
   * `WithMaxQueuedNotifications` (`:466`-`:470`): `n` is how many notifications wait while a handler runs. Past it the policy acts,
     and `OverflowBlock` reads on to `2n` while the loop waits on the connection.
   * `MethodHandler` (`:300`-`:313`): one sentence on `OverflowBlock`'s rule, which now binds by default.
   * `Err` (`:821`-`:830`) and `EndedCleanly` (`:841`-`:846`): "a notification overflow" names the close policy and the read-on
     limit.
   * `NotificationOverflowPolicy` (`:542`-`:545`): unchanged.
   * `errors.go:206`-`:209`: `ErrNotificationQueueOverflow` is why a connection closed itself under `OverflowCloseConnection`, or
     under `OverflowBlock` past its read-on limit.
   * `doc.go:27`-`:30`: a full queue closes a connection under `OverflowCloseConnection`, or under `OverflowBlock` past its read-on
     limit. The default pauses reading instead.
3. **State and the default.**
   * In `buildConnection` (`:701`-`:704`), the `connectionConfig` literal starts with `notificationOverflow: OverflowBlock`. After the
     options, a value that is none of the three constants becomes `OverflowBlock`.
   * On `Connection`, beside `notificationQueue` (`:415`-`:417`):
     * `queueLimit int`: `n`, where the reader pauses;
     * `peerWaits int`, guarded by `notifyMu`: how many waits of the notification loop on this connection are running (the MADR's D3).
   * The channel's capacity (`:729`) is `2n` under `OverflowBlock` and `n` otherwise. `queueLimit` is `n`.
   * The overflow log line (`:1605`) reports `queueLimit` as `capacity`.
4. **The pause.** In `receiveLine`, after `c.notifyMu.Lock()` and before `lastEnqueuedNotificationSeq++` (`:1579`-`:1580`), under
   `OverflowBlock`, call `waitForRoom`. It is entered and left with `notifyMu` held:
   * while `len(c.notificationQueue) >= c.queueLimit`, `c.peerWaits == 0` and `c.ctx.Err() == nil`, it waits on `notifyCond`.
     *Amended 2026-10-31* (see "Deviation, 2026-10-31, step 1.4"): the count is
     `lastEnqueuedNotificationSeq - completedNotificationSeq > queueLimit`, not the channel's length;
   * the first wait starts a `time.AfterFunc` of `pauseWarnAfter`, a new constant of 5 s beside `notificationQueueDrainTimeout`
     (`:22`). It logs `Warn` "notification queue full; reading paused", with `capacity` and `paused`. The timer is stopped when the
     wait ends;
   * it reports whether the connection is still live.

   When it is not, `receiveLine` unlocks, calls `c.shutdownReceive(context.Cause(c.ctx))`, and returns false. The sequence is not
   advanced before the wait, so no watermark counts a notification the reader has not queued. Past the wait, the existing `select`
   either queues the notification or, at `2n`, reaches `default`, which closes as `OverflowCloseConnection` does. `OverflowBlock` is
   not `OverflowDropNewest`, so the existing branch at `:1597` already does that.

   What wakes the wait:
   * `processNotifications` broadcasts after each handler (`:1765`);
   * `wakeNotificationWaits` runs at the connection's end (`:755`);
   * `beginPeerWait` broadcasts (step 5).
5. **The guard.**
   * `beginPeerWait() (end func())` increments `peerWaits` under `notifyMu`, broadcasts, and returns a `sync.OnceFunc` that decrements
     it.
   * `inRunningNotification(ctx) bool` is true when `markOf(ctx)` is a notification's mark whose `seq` exceeds
     `completedNotificationSeq`: the test `waitNotificationsUpTo` uses (`:2351`).
   * `exchange` (`:2133`-`:2137`) and `SendRequestNoResult` (`:2434`-`:2439`) call `beginPeerWait` after registering the pending
     request, when `inRunningNotification(ctx)`, and defer its end.
   * `sendRequestCommit` (`:2023`) calls `beginPeerWait` first, ends it in the `settled` it hands on, before calling the caller's, and
     also defers it, so that a path that never settles still ends it.

   The guard runs under every policy. It only matters to `OverflowBlock`, and costs one lock and a broadcast for each such request.
6. **No other path changes.** `handleResponse`, the barrier, `processNotifications`' sequence checks, the writer and `shutdownReceive`
   are as they were.
7. **Tests**, `connection_overflow_block_test.go`. Each is a `synctest` bubble where its transport is a pipe made in the bubble, so a
   blocked write is a durable block and `synctest.Wait` can assert it; otherwise bounded waits, as `connection_overflow_policy_test.go`
   uses. Each runs under `-race`.
   * `TestOverflowBlockPausesReading`, a queue of 2 with notification 0's handler held.
     * Notifications 1 and 2 are queued, and 3 is read and waits. The write of 4 has not returned.
     * `DroppedNotifications` is 0, and `Done` is open.
     * After the release, 0 to 6 are delivered in order, and the connection is open.
   * `TestOverflowBlockResponseAsWithRoom`: a request whose response follows four notifications, the first held, returns only after the
     release, with 0 to 3 delivered first. It is checked under a queue of 100 with `OverflowCloseConnection`, and under `OverflowBlock`
     with a queue of 2.
   * `TestOverflowBlockRequestFromNotificationHandler`: notification 0's handler sends a request under its context, and the peer answers
     it after three more notifications, with a queue of 2.
     * The request is answered, within the test's bound.
     * After it returns and the handler is released, a new burst pauses the reader again at 2: the guard is ended.
   * `TestOverflowBlockHeldInitialize`: three extension notifications before `initialize`'s answer, with a queue of 1, as
     `TestHeldNotificationsFillQueue` sends them.
     * `Initialize` succeeds, and the three are handled.
     * Under `OverflowCloseConnection` the same input still closes, as that test requires.
   * `TestOverflowBlockReadOnLimit`: notification 0's handler waits on the peer, with a queue of 2.
     * Four more notifications are queued and the connection stays open.
     * A fifth closes it, `Err` wrapping `ErrNotificationQueueOverflow` and `ErrConnectionClosed`, and the handler's request fails with
       the connection's end.
   * `TestOverflowBlockEndsOnClose`: with the reader paused, `Close`, and in a second case `Shutdown` under a context of 1 s.
     * `Done` closes, and `Wait` returns, within the bound.
     * `Err` wraps `ErrConnectionClosed`, and no goroutine is left (the package's leak guard).
   * `TestOverflowBlockSlowsAgent`: a client under `OverflowBlock` with a queue of 2, its `SessionUpdate` held, over `promptPair`'s
     pipes.
     * The agent's prompt handler sends updates in a loop. While the client is held, at most four of its `SessionUpdate` calls return
       (one held, two queued, one read and waiting; `io.Pipe` holds nothing), and the next one waits.
     * The client's `Cancel` ends the agent's turn, and the blocked `SessionUpdate` returns its context's error.
     * After the release, `Prompt` returns `cancelled`.
   * `TestOverflowBlockLogsLongPause`: in a bubble, a pause of 5 s logs exactly one `Warn` with `capacity`. A pause of 4 s logs none.
   * `TestOverflowBlockIsTheDefault`: with no policy option, and with `NotificationOverflowPolicy(99)`, the reader pauses as in
     `TestOverflowBlockPausesReading`, and `Done` stays open. With `OverflowCloseConnection` named, the same input closes the connection
     with `ErrNotificationQueueOverflow`.

   The tests above name `OverflowBlock` except where they say otherwise, so each states its policy; `TestOverflowBlockIsTheDefault`
   alone relies on the default.
8. **The existing tests that relied on the old default.** Each exercises the close, so each names `OverflowCloseConnection` and keeps
   its assertions:

   | Test | Change |
   | :--- | :--- |
   | `acp_test.go:891`, `TestConnectionFailsFastOnNotificationQueueOverflow` | `WithNotificationOverflowPolicy(OverflowCloseConnection)` |
   | `connection_close_test.go:186`, `TestOverflowCloseClosesTransport` | the option, through `newOverflowFixture` |
   | `connection_end_test.go:107`, `TestErr`'s subtest "notification overflow" | the option |
   | `connection_end_test.go:213`, `TestEndedCleanly`'s overflow case | the option |
   | `initialize_test.go:381`, `TestHeldNotificationsFillQueue` | the option; `TestOverflowBlockHeldInitialize` covers the default |
   | `agentproc/ties_test.go:194`, `TestConnectionEndStopsAgent`'s "a notification overflow" | the option, in `ConnectionOptions` |
   | `connection_overflow_policy_test.go:135`, `TestConnection_NotificationOverflowPolicyClosesByDefault` | renamed `TestConnection_NotificationOverflowPolicyClose`. It keeps its case "explicit close"; the cases "no option" and "undefined policy" move to `TestOverflowBlockIsTheDefault`, which requires the new default |

   The capacity checks read `n` as the configured limit, `queueLimit`, where they read the channel's capacity:
   * `connection_queue_size_test.go:11`, `:30`, `:49`, `:67` and `:85`;
   * `options_test.go:22`'s `check`.

   `TestWithMaxQueuedNotifications_DefaultSize` also requires the channel's capacity, `2n` under the default.

   Before this step, a run of the whole suite on the phase's code, with these tests unchanged, records which ones fail. The list above
   must match it exactly. A test that fails and is not listed is a deviation, and is put to the owner.

   The list was drawn up from that run on the MADR's scratch prototype, with the guarded pause as the default (MADR F9). It failed
   exactly these seven tests and six capacity checks, and nothing else in any package.
9. **Planted breaks**, each in a scratch copy of the tree, never in the tree; each must make the named test fail, and the failure
   is recorded:

   | Break | Test that must fail |
   | :--- | :--- |
   | the default left at `OverflowCloseConnection` | `TestOverflowBlockIsTheDefault` |
   | an undefined value not mapped to `OverflowBlock` | `TestOverflowBlockIsTheDefault` |
   | `OverflowBlock` treated as `OverflowCloseConnection` (no pause) | `TestOverflowBlockPausesReading` |
   | the pause at `len >= queueLimit-1` | `TestOverflowBlockPausesReading` |
   | no guard in `exchange` | `TestOverflowBlockRequestFromNotificationHandler` |
   | the guard's end not called | `TestOverflowBlockRequestFromNotificationHandler` |
   | no guard in `sendRequestCommit` | `TestOverflowBlockHeldInitialize` |
   | the channel's capacity `n` under `OverflowBlock` | `TestOverflowBlockReadOnLimit` |
   | the wait not ended by the connection's end | `TestOverflowBlockEndsOnClose` |
   | no `Warn` timer | `TestOverflowBlockLogsLongPause` |
10. **Checks.**
    * `make pre-add-check`, which runs `go test -race` over every package, `agentproc` included.
    * `go test -race -count=3 -run 'OverflowBlock|Overflow|Barrier|Held|QueueSize|EndedCleanly|ConnectionEnd' ./...`.
    * `tools/remote.py pkg --host linux` and `--host windows`, for `.` with `-run OverflowBlock -count=3`, and for `./agentproc`.
    * `make api`, `make check-records`, `make markdownlint` and `make disclosure-check`.

    Then the execution record, and staging for the owner.

### Phase 2: the documentation and the example

1. `ExampleWithNotificationOverflowPolicy` in `example_api_test.go`: a connection under `OverflowBlock` with a queue of 1 and a handler
   that is slow for its first notification. Three notifications are written; the example prints them in order and
   `DroppedNotifications`, with a fixed `// Output:`.
2. `README.md`:
   * "Closing and limits" (`:646`-`:655`): `:654`-`:655` no longer name the close as the default. A paragraph covers the three
     policies, what each does with a full queue, that `OverflowBlock` is the default, and its consequences: what waits behind a pause,
     the rule, the log, `remote`'s bounds. A short code block chooses `OverflowCloseConnection` for a consumer that wants the close.
   * The hold paragraph (`:497`-`:502`): "the overflow policy, which closes the connection by default" changes. Under the default a
     held notification does not stop the reader; under `OverflowCloseConnection` the queue's limit still applies.
   * The `Err` section (`:803`-`:833`): the overflow's close is `OverflowCloseConnection`'s, or `OverflowBlock`'s past its read-on
     limit. The example's message at `:829`-`:830` says so.
3. `docs/architecture.md`:
   * the notification paragraph (`:435`-`:443`): the pause, `queueLimit`, `peerWaits`, the read-on limit, the log;
   * the checks list (`:509`-`:518`): the new test file and what each test pins.
4. Checks: `go test -race ./...`; `go vet ./...`; `make pre-add-check`; `make check-records`; `make markdownlint`;
   `make disclosure-check`. Then the execution record, and staging.

### Phase 3: the close

1. `tools/gates.py --all`, all 18 gates, with its output in the record.
2. After the owner pushes, `tools/watch_ci.py` on the commit: every job of `ci.yml`.
3. A table of the MADR's twelve "Confirmation" items, each with the test or command that shows it. Item 12 is shown in phase 4.
4. The MADR's status line and the index rows; the PLAN `in-progress` until phase 4 ends.

### Phase 4: the release, on the owner's ask

As [0042-PLAN-in-process-agent-runner.md](0042-PLAN-in-process-agent-runner.md)'s phase 17 ran it:

1. `tools/consumer.py`:
   * Its run, under the new default, is unchanged.
   * It also opens a connection with `WithNotificationOverflowPolicy(acp.OverflowCloseConnection)`, the opt-out the notes name, and
     prints that it compiles and runs.
   * `WANT_RUN` and the docstring name `v1.29.0`, and `tools/README.md`'s row is updated.
   * `make check-tools` runs, and the check is seen to fail on a planted break: a consumer that names an undefined identifier.
2. `docs/reports/0044-REPORT-release-notes-v1-29-0.md` from `tools/release_body.py v1.29.0 REV`, with its sources, and its index row.
   * Its first section is the change of default: what pauses now where it closed, what waits behind a pause, the rule, the 5 s log,
     `remote`'s bounds, and the one-line opt-out.
3. The owner tags `v1.29.0` and creates the release; then `tools/publication.py` and `tools/consumer.py v1.29.0` against the
   published tag.
4. On the owner's ask, a comment on issue #13 with the release link, and the issue closed.
5. The PLAN `complete`, and the MADR's status line.

## Verification

* Every test of step 1.7 passes under `-race` on macOS, and with `-count=3` on the Linux and Windows hosts, and was seen to fail on its
  planted break.
* The existing tests of the queue pass unchanged:
  * `connection_overflow_policy_test.go`;
  * `connection_queue_size_test.go`, whose capacity checks use the default policy;
  * `connection_notification_barrier_test.go`;
  * `initialize_test.go`'s hold tests.
* `make api`: one compatible addition, `OverflowBlock`; 0 incompatible changes in the four gated packages.
* `tools/gates.py --all` passes, and so does CI on the pushed commits.

## Rollout and Rollback

* **Rollout.**
  * The API change is additive, but the default changes for every consumer that names no policy (MADR D9).
  * The release notes lead with it, and with the opt-out: `WithNotificationOverflowPolicy(acp.OverflowCloseConnection)`.
  * They also say what a pause holds, the rule, the 5 s log, and `remote`'s bounds.
* **Rollback before the release.** Revert the phase commits; nothing outside this PLAN depends on them.
* **Rollback after the release.**
  * `OverflowBlock` stays exported, under the compatibility promise.
  * Should the default prove wrong, a patch release can restore `OverflowCloseConnection` as the default, a one-line change in
    `buildConnection`. `OverflowBlock` would stay available to those that choose it.
  * Should `OverflowBlock` itself prove wrong, a patch release can make it act as `OverflowCloseConnection` while a fix is found. That
    loses nothing silently.

## Execution record

*2026-10-31:* the owner accepted the MADR and approved this PLAN ("acccepted, approved, proceed"). The records are committed
alone, as `AGENTS.md`'s bootstrap exception requires, before phase 1's source.

### Phase 1, 2026-10-31

**What changed.**

* `connection.go`:
  * `OverflowBlock`, the default: the `connectionConfig` literal, and an undefined value mapped to it.
  * `queueLimit` and `peerWaits`; the channel at `2n` under `OverflowBlock`.
  * `waitForRoom`, called in `receiveLine` before the sequence advances.
  * `beginPeerWait` and `inRunningNotification`; the guard in `exchange`, `SendRequestNoResult` and `sendRequestCommit`.
  * `pauseWarnAfter`, and the overflow log's `capacity` as `queueLimit`.
  * The godoc of D8: `MethodHandler`, `WithMaxQueuedNotifications`, the three policies, `WithNotificationOverflowPolicy`, `Err` and
    `EndedCleanly`.
* `errors.go` (`ErrNotificationQueueOverflow`) and `doc.go` ("How a connection ends").
* `connection_overflow_block_test.go`, new: the nine tests of step 1.7.
* The thirteen tests of step 1.8.

**Step 1.8, the run before the tests changed.** `go test -race -count=1 ./...` on the phase's code with the existing tests
unchanged failed exactly the list:
* `TestConnectionFailsFastOnNotificationQueueOverflow`;
* `TestConnection_NotificationOverflowPolicyClosesByDefault/no_option` and `/undefined_policy`;
* the five `TestWithMaxQueuedNotifications_*`, and `TestOptionSemantics`;
* `TestEndedCleanly`, `TestOverflowCloseClosesTransport`, `TestHeldNotificationsFillQueue` and `TestErr/notification_overflow`;
* `agentproc`'s `TestConnectionEndStopsAgent/a_notification_overflow`.

Every other package passed: `acptest`, `clienthost`, `remote` and the four examples. After the changes of step 1.8, the whole suite
passed.

**Deviation, 2026-10-31, step 1.4: the pause counted the channel's length.**
* *Found.* `tools/remote.py pkg` on the Linux and Windows hosts. On Linux, `TestOverflowBlockPausesReading` and
  `TestOverflowBlockIsTheDefault` failed with "2 notifications queued while the handler is held, want 3" and "3 writes returned, want
  4". On Windows, `TestOverflowBlockEndsOnClose` failed with "1 notifications queued, want 2: the reader paused". macOS never showed it.
* *Cause.* The worker frees a slot when it takes a notification from the channel, but broadcasts only when a handler returns. When
  the reader filled the queue before the worker had taken notification 0, it paused one early, and stayed paused, with room, until the
  held handler returned.
* *Resolution.* Count the notifications received and not completed, `lastEnqueuedNotificationSeq - completedNotificationSeq`, which
  change only under `notifyMu` and with a broadcast at each completion. Pause while that count exceeds `queueLimit`: `n` waiting
  besides the one running, which is the MADR's D2 unchanged. No file was added to the phase.
* *Evidence.* On Linux, `TestOverflowBlockPausesReading` with `-test.count=30` failed 18 of 30 times on the old condition, from a
  scratch copy through `--tree`, and 0 of 30 times on the new.
* *Gosec.* The comparison's `uint64(c.queueLimit)` drew gosec's G115. It carries a `//nolint:gosec` with its reason, that
  `queueLimit` is at least 1, as `clienthost/terminals.go:117` does.

**Refinements within step 1.7.**
* `TestOverflowBlockEndsOnClose` gained a third case, "Close, the handler ignoring its context". In the first two, `Close` and
  `Shutdown` end the held handler, so the queue drains and the reader resumes, whether or not the connection's end woke it. The
  break "the wait not ended by the connection's end" would then pass. The new case holds a handler that ignores its context and
  requires that no goroutine is left in `waitForRoom` after `Close`. It reads every goroutine's stack, so the test is not parallel.
* `blockPeer` releases its held handler in a cleanup, through `releaseOnce`. A test that stops early still lets the handler return
  before its `synctest` bubble ends. The Windows failure above had ended in "deadlock: main bubble goroutine has exited but blocked
  goroutines remain".

**Step 1.9, the planted breaks**, each in a scratch copy, by a script that requires each break's text to match exactly once, rerun on
the final code:

| Break | Test | Failure seen |
| :--- | :--- | :--- |
| the default left at `OverflowCloseConnection` | `TestOverflowBlockIsTheDefault/no_option` | "the connection ended while paused: acp: connection closed: notification queue overflow" |
| an undefined value not mapped to `OverflowBlock` | `TestOverflowBlockIsTheDefault/undefined_policy` | the same |
| no pause | `TestOverflowBlockPausesReading` | "5 notifications queued while the handler is held, want 3"; "6 writes returned, want 4"; the overflow's close |
| the pause one notification early | `TestOverflowBlockPausesReading` | "2 notifications queued … want 3"; "3 writes returned, want 4" |
| no guard in `exchange` | `TestOverflowBlockRequestFromNotificationHandler` | "the handler's request is not answered: the reader paused while the handler waited on it" |
| the guard's end not called | `TestOverflowBlockRequestFromNotificationHandler` | "5 notifications queued after the request returned, want 4: the reader pauses again" |
| no guard in `sendRequestCommit` | `TestOverflowBlockHeldInitialize` | "Initialize did not return: the reader paused on the held notifications" |
| the channel's capacity `n` under `OverflowBlock` | `TestOverflowBlockReadOnLimit` | "3 notifications queued, ended true (acp: connection closed: notification queue overflow); want 5 and open" |
| the wait not ended by the connection's end | `TestOverflowBlockEndsOnClose/Close,_the_handler_ignoring_its_context` | "the reader still waits for room after Close" |
| no `Warn` timer | `TestOverflowBlockLogsLongPause` | "0 warnings after a pause of 5 s, want 1" |

**Step 1.10, the checks.**
* `make pre-add-check`: "go-precheck: 277 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck)."
* `go test -race -count=1 ./...`: every package `ok`.
* `go test -race -count=3 -run 'OverflowBlock|Overflow|Barrier|Held|QueueSize|EndedCleanly|ConnectionEnd|TestErr|OptionSemantics|FailsFast' ./...`:
  every package `ok`.
* `make api`: "compatible: OverflowBlock: added"; "against v1.28.0: 0 incompatible, 1 compatible".
* `tools/remote.py pkg`, after the deviation's fix, both exit 0 on each host:
  * `.` with `-test.run 'OverflowBlock|Overflow|Held|QueueSize|EndedCleanly|TestErr|OptionSemantics|FailsFast' -test.count=3`;
  * `./agentproc` with `-test.run ConnectionEndStopsAgent -test.count=3`.
* *Beyond the step.* The whole root package was also run once on each host. It fails only tests that read the source tree, the
  schema, `testdata/`, a golden file or `types_gen.go`, or build an example; the binary runs there without the source. The same
  class fails on a clean export of `24e88d1` through `--tree`, so it is the harness, not this change. Which of them a run reports
  varies, because one of them panics and ends the binary. No connection test is among them. CI runs the whole suite from source on
  all three systems (phase 3).

The owner committed the records as `d9ff0c4` and phase 1 as `c4dcb77`.

### Phase 2, 2026-10-31

**What changed.**
* `example_api_test.go`: `ExampleWithNotificationOverflowPolicy`. A connection under `OverflowBlock` with a queue of 1 and a first
  handler that sleeps 10 ms. Four notifications are written on the example's own goroutine, whose writes wait while the reader is
  paused. It prints `handled 1` to `handled 4` and `dropped: 0`.
* `README.md`:
  * The hold paragraph: under the default the connection reads on while `Initialize` waits; under `OverflowCloseConnection`, the
    held notifications close it.
  * "Closing and limits":
    * Its first paragraph no longer names the close as the default.
    * New paragraphs on the queue's limit and the three policies, with the default and its version: what a pause holds, the rule,
      the read-on limit, the log, `remote`'s bounds, and the opt-out in a code block.
  * The `Err` section: an overflow's close is `OverflowCloseConnection`'s, or `OverflowBlock`'s past its read-on limit. The
    example's message is "the client's notifications overflowed the queue's limit".
* `docs/architecture.md`:
  * A paragraph and list on the policies' mechanism: `queueLimit`, the channel at `2n`, `waitForRoom` on the sequence count,
    `peerWaits` and the three places that count, and the log.
  * `endError`'s "an overflow's close".
  * The checks list names `connection_overflow_block_test.go`.

No step was skipped and nothing deviated.

**Seen to fail.** In a scratch copy, the example given `OverflowCloseConnection` in place of `OverflowBlock` fails. It got
`write: io: read/write on closed pipe`, `handled 1`, `handled 2`, `dropped: 0`, against the four `handled` lines it wants.

**Checks.**
* `go test -race -count=1 ./...`: every package `ok`.
* `go vet ./...`: clean.
* `make pre-add-check`: "go-precheck: 278 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck)."
* `make check-records`, `make markdownlint` and `make disclosure-check`: exit 0.
* A search of the Markdown outside the records for the old default's wording ("closes the connection by", "default overflow")
  finds none.

The owner committed phase 2 as `3845b28`.

### Phase 3, 2026-10-31

**Step 3.1, the gates.** `tools/gates.py --all` on `3845b28`, the tree clean, exit 0:

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

* `precheck`: "go-precheck: 278 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck)."
* `api`: "compatible: OverflowBlock: added"; "against v1.28.0: 0 incompatible, 1 compatible".
* `windows` and `linux` run `tools/remote.py tree`, the whole tree from source, `go1.27.3` on each host: every package `ok`, the
  root package included. Phase 1's runs of the root package there failed only the tests that read the source, since `pkg` sends
  the test binary alone; from source, nothing fails.

`make push-guard`, after a fetch: "3 outgoing commit(s), 24e88d1..3845b28, main to origin"; "the guard passed (exit 0)".

**Step 3.2, CI.** The owner pushed `d9ff0c4`, `c4dcb77` and `3845b28`; `origin/main` is `3845b28`. `tools/watch_ci.py 3845b28`,
exit 0:

```text
watch_ci: 2 run(s) for 3845b28: 41207967212, 41207967160
watch_ci: run 41207967212 (main, push): completed success; watch exit 0
  completed success: docs
watch_ci: run 41207967160 (main, push): completed success; watch exit 0
  completed success: validate (windows-2025, 1.27.3)
  completed success: validate (ubuntu-24.04, 1.26.10)
  completed success: validate (macos-15, 1.26.10)
  completed success: validate (windows-2025, 1.26.10)
  completed success: validate (ubuntu-24.04, 1.27.3)
  completed success: validate (macos-15, 1.27.3)
```

**Step 3.3, the MADR's Confirmation.**

| # | Item | Shown by |
| :--- | :--- | :--- |
| 1 | The reader stops at a full queue; every notification delivered in order, none dropped, the connection open | `TestOverflowBlockPausesReading` |
| 2 | A response behind a full queue returns as under a queue with room | `TestOverflowBlockResponseAsWithRoom` |
| 3 | A notification handler's request under its own context is answered while the queue is full | `TestOverflowBlockRequestFromNotificationHandler` |
| 4 | Notifications held for `initialize` past the capacity do not stop `Initialize` | `TestOverflowBlockHeldInitialize` |
| 5 | While the loop waits on the reader, the reader reads on to `2n`, then closes with `ErrNotificationQueueOverflow` | `TestOverflowBlockReadOnLimit` |
| 6 | `Close` and `Shutdown` end a paused reader; `Done` closes, `Wait` returns | `TestOverflowBlockEndsOnClose`: `Close`, `Shutdown`, and `Close` with a handler ignoring its context |
| 7 | An agent whose client pauses is slowed at `SessionUpdate`, and `session/cancel` still ends its turn | `TestOverflowBlockSlowsAgent` |
| 8 | A pause of 5 s logs one `Warn` | `TestOverflowBlockLogsLongPause` |
| 9 | No policy or an undefined one pauses; `OverflowCloseConnection` closes as before; the barrier and hold tests unchanged | `TestOverflowBlockIsTheDefault`; the thirteen tests of step 1.8 with `OverflowCloseConnection` named; `connection_notification_barrier_test.go` and `initialize_test.go`'s hold tests, unchanged, in `race3` |
| 10 | `make api`: one compatible addition, `OverflowBlock`, no incompatible change | the `api` gate above |
| 11 | `tools/gates.py --all`, the new tests on the Linux and Windows hosts, and CI pass | step 3.1; phase 1's `tools/remote.py pkg` runs; CI on `3845b28` (step 3.2) |
| 12 | The `v1.29.0` release notes lead with the change of default and its opt-out | phase 4 |

**Deviation, 2026-10-31, step 1.9: two tests had no break of their own.** Building the table showed that step 1.9's breaks cover
items 1, 3, 4, 5, 6, 8 and 9, but not `TestOverflowBlockResponseAsWithRoom` (item 2) or `TestOverflowBlockSlowsAgent` (item 7),
although the MADR's Confirmation requires each new test to be seen to fail. Both were planted now, in a scratch copy, on `3845b28`:

| Break | Test | Failure seen |
| :--- | :--- | :--- |
| the response barrier skipped (`waitNotificationsUpTo` returns at once) | `TestOverflowBlockResponseAsWithRoom/room_for_all` | "the request returned `<nil>` while notification 0 was held" |
| the pause never lifts | `TestOverflowBlockResponseAsWithRoom/OverflowBlock,_a_queue_of_two` | "the request did not return once the notifications before its response were handled" |
| no pause | `TestOverflowBlockSlowsAgent` | "6 updates returned while the client was held, want at most 4"; the overflow's close |

The `OverflowBlock` case passes with the barrier skipped: at a queue of two the pause holds the response unread until the held
handler returns, so the order holds without the barrier. The control case catches that break, and the `OverflowBlock` case catches
a pause that does not lift. A paused reader that `Close` does not wake passes `TestOverflowBlockSlowsAgent`, which ends without a
`Close`; `TestOverflowBlockEndsOnClose` catches it (step 1.9). No test or code changed.

Every item from 1 to 9 is now shown by a test seen to fail on a planted break, and the example was seen to fail in phase 2.

**Step 3.4, the status lines.** The MADR stays `accepted` and the index rows unchanged: the MADR `accepted`, this PLAN
`in-progress` until phase 4 ends.

The owner committed phase 3's record as `934e33b`.

### Phase 4, 2026-10-31: before the tag

The owner asked for phase 4 ("committed, proceed with phase 4").

**Step 4.1, `tools/consumer.py`.**
* The program opens a client connection to an agent over a pipe with
  `WithNotificationOverflowPolicy(acp.OverflowCloseConnection)`, initializes it, closes both ends, and prints
  `acp.OverflowBlock != acp.OverflowCloseConnection`, the protocol version and the three errors.
* `WANT_RUN` gains `overflow true 1 <nil> <nil> <nil>`; the docstring names this PLAN and says `VERSION` is `v1.29.0` or later.
* `tools/README.md`: `consumer.py`'s row names the opt-out and `v1.29.0`.
* *Before the tag.* A scratch script built the program and test against `934e33b`'s tree through a `replace`, at `go1.26.10`, three
  times: `go mod tidy`, the build, the run, `go test` and the Windows build, each exit 0; "output equals WANT_RUN: True". The first
  run, before `WANT_RUN` changed, differed first at line 27, the new line, so the 26 lines before it are unchanged under the new
  default.
* *Seen to fail.* `tools/consumer.py v1.28.0`, the published release without `OverflowBlock`, a consumer naming an undefined
  identifier: exit 1, `go build` "./main.go:242:30: undefined: acp.OverflowBlock", "NOT AS REQUIRED".
* `make check-tools`, exit 0: "Ran 58 tests" and "Ran 32 tests", both "OK"; "check-ci-filters: 0 problem(s)".

**Step 4.2, the release notes.**
[0044-REPORT-release-notes-v1-29-0.md](../reports/0044-REPORT-release-notes-v1-29-0.md), and its row in `docs/README.md`. Its first
section is the change of default: what pauses now where it closed, what waits behind a pause, the rule, the reading on, the 5 s
log, `remote`'s bounds, and the opt-out in a code block. Its sources: the five commits since `v1.28.0`, the three Go files changed
outside the tests, the unchanged areas, `make api` on `934e33b` ("0 incompatible, 1 compatible"), CI on `3845b28`, and the
consumer before the tag.

**Left, each on the owner's ask:** the push of this commit and its CI; the tag `v1.29.0`, the GitHub release from
`tools/release_body.py`, `tools/publication.py`, and `tools/consumer.py v1.29.0`, which with the notes shows the Confirmation's
item 12; then issue #13 answered and closed with the release's link, and this PLAN and its row `complete`.

### Phase 4, 2026-10-31: the release

The owner committed the release notes and the consumer as `f2c53b9` and pushed it.

* **CI on `f2c53b9`,** `tools/watch_ci.py --branch main f2c53b9`, exit 0: run `41208877236`, "completed success", the six
  `validate` jobs on macOS, Ubuntu and Windows at Go `1.27.3` and `1.26.10`; run `41208877323`, the Docs workflow, "completed
  success".
* **The body,** `tools/release_body.py v1.29.0 f2c53b9`: "53 lines, 2 links", its identifier scan "0 match(es)".
* **The tag.** The owner pushed `v1.29.0`, annotated, naming `f2c53b9`, as `git ls-remote` shows. Its CI,
  `tools/watch_ci.py --branch v1.29.0 f2c53b9`, exit 0: run `41209567306`, "completed success", the same six jobs.
* **`tools/consumer.py v1.29.0`,** exit 0, "as required". At `go1.26.10`, from the proxy: `go get` of `v1.29.0`, `go mod tidy`,
  the build, the run, whose output equals `WANT_RUN`, `overflow true 1 <nil> <nil> <nil>` among its lines, `go test` and the
  Windows build, each exit 0. The consumer's `go.mod` keeps `go 1.26.10` and has no toolchain line.
* **The release.** The owner ran `gh release create v1.29.0 --verify-tag --title v1.29.0 --notes-file` with the extracted body:
  <https://github.com/maccavelli/acp-go-sdk2/releases/tag/v1.29.0>.
* **`tools/publication.py v1.29.0 f2c53b9`,** exit 0, "10 of 10 checks hold": the release exists, not a draft or prerelease; its
  body is the extracted release text; it is marked latest; the tag is annotated and names `f2c53b9`, which is on origin's `main`;
  CI passed on `main`'s head, run `41208877323`, and on the tag, run `41209567306`; the proxy serves the `.info`, its origin the
  tag at that commit; `go list -m @latest` gives `v1.29.0`; `go mod download` of the tag passes with the checksum database on.

**The Confirmation's item 12,** "The `v1.29.0` release notes lead with the change of default and its opt-out", is shown by the
published release, whose first section is "Changed: a full notification queue pauses reading, by default", ending in the opt-out.
Items 1 to 11 were shown in phase 3.

* **Issue #13,** on the owner's "post and close": answered with the release's link, the new default and what it pauses, the rule,
  the reading on, the 5 s log, the opt-out, and the MADR's link at the tag
  (<https://github.com/maccavelli/acp-go-sdk2/issues/13#issuecomment-6544689241>); closed as completed, 2026-10-31T14:27:51Z.

Every item of the MADR's Confirmation is shown, the release is published, and the issue is closed, so this PLAN and its row are
`complete`.
