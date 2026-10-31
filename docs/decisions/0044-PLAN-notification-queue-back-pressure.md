---
status: in-progress
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
   * while `len(c.notificationQueue) >= c.queueLimit`, `c.peerWaits == 0` and `c.ctx.Err() == nil`, it waits on `notifyCond`;
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
