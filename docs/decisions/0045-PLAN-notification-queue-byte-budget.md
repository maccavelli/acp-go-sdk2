---
status: proposed
date: 2026-10-31
associated-madr: "0045-MADR-notification-queue-byte-budget.md"
---

# Implement 0045: a byte budget beside the notification count

Associated MADR: [0045-MADR-notification-queue-byte-budget.md](0045-MADR-notification-queue-byte-budget.md)

## Goal

A connection bounds in bytes what it keeps of its peer's unfinished messages, beside the count it already keeps. Each overflow policy
acts at the budget as it acts at the count, under the MADR's D1 to D7. The owner answered on 2026-10-31: a 64 MiB default (Q1 a)
and one budget for requests and notifications (Q2 a). The notes below on Q1 (c) and Q2 (b) record what those answers would have
changed.

## Scope

In scope:

* `connection.go`:
  * the option `WithMaxQueuedBytes`, its default and its clamp to `WithMaxMessageSize` (D1, D4);
  * the counter under `notifyMu`;
  * the size carried by `queuedNotification`;
  * `receiveLine`'s checks per policy (D3);
  * `waitForRoom`'s second condition;
  * `processNotifications`' release;
  * with Q2 (a), the request path's check and release;
  * the accessor `QueuedBytes` and the log fields (D7);
  * the godoc of `WithMaxQueuedNotifications`, the three policies, `WithMaxConcurrentRequests` and
    `ErrNotificationQueueOverflow`.
* A new test file, `connection_queue_bytes_test.go`. Existing tests change only if one is found to depend on an unbounded queue;
  each such change is recorded.
* `README.md`, "Closing and limits"; `docs/architecture.md`; `example_api_test.go`, an example of the option.
* `tools/consumer.py`, the release notes and the release, on the owner's ask, as 0044's phase 4 ran them.

Out of scope:

* `remote`'s `MaxBuffered`, a different bound (MADR, Consequences);
* `WithMaxMessageSize`'s default;
* a per-entry overhead;
* bytes held by a handler after it returns;
* queues per session;
* the outbound side.

## Implementation Steps

### Phase 1: the budget

1. **Grounding.** In a scratch copy, add a temporary test that queues notifications under each policy and reads the counters, to
   confirm D2's accounting point against `processNotifications`, and that `readLine`'s line length is what reaches
   `receiveLine`. Recorded, then deleted.
2. **The option and the counter.**
   * `connectionConfig.maxQueuedBytes`, default 64 MiB (Q1 a). With Q1 (c) it stays unset, and every check below is skipped
     when it is 0.
   * `WithMaxQueuedBytes(n)` ignores `n <= 0`.
   * `buildConnection` raises a budget below `maxMessageSize` to it (D4).
   * `Connection.queuedBytes int64` and `Connection.byteLimit int64`, under `notifyMu`.
3. **Notifications.**
   * `queuedNotification` gains `size int`. `receiveLine` passes `len(line)`; the line is in scope there.
   * Under `OverflowBlock`, `waitForRoom(size)` also waits while `queuedBytes > 0 && queuedBytes+size > byteLimit`, with
     0044 D3's guard unchanged. Past the wait, a check closes with `ErrNotificationQueueOverflow` when
     `queuedBytes+size > 2*byteLimit`, as the channel's capacity does at 2n.
   * Under `OverflowCloseConnection` and `OverflowDropNewest`, the same test before the `select` takes the existing overflow
     path: close, or drop with the drop handler.
   * On queueing, `queuedBytes += size`. In `processNotifications`, after the handler returns, `queuedBytes -= queued.size`,
     beside `completedNotificationSeq`, before the `Broadcast`.
4. **Requests (Q2 a; skipped with Q2 b).**
   * Before a request's goroutine starts, under `notifyMu`: if `queuedBytes > 0 && queuedBytes+size > byteLimit`, answer
     `-32603` with `{"error":"too many queued bytes","limit":byteLimit}`, as the concurrency limit answers, and never wait.
   * Otherwise add `size`, and subtract it in the goroutine's deferred cleanup, with a `Broadcast` so a paused reader re-checks.
   * The check sits after the concurrency limit's, so a request refused for count is not counted.
5. **Observability (D7).** `QueuedBytes() int64`. The pause's `Warn` and the overflow's `Error` gain `bytes` and `byte_limit`.
6. **Godoc.** The option, and the sentences that state each policy's trigger: "n notifications, or the byte budget".
7. **Tests**, in `connection_queue_bytes_test.go`, each in a `synctest` bubble where it waits:
   * `TestQueuedBytesPausesReading`: count room, bytes full; the writer waits, all delivered in order, none dropped, the
     connection open.
   * `TestQueuedBytesReadOnLimit`: under 0044's guard the reader reads to twice the budget, then closes with
     `ErrNotificationQueueOverflow`.
   * `TestQueuedBytesClose` and `TestQueuedBytesDrop`: the other two policies at the budget.
   * `TestQueuedBytesClampAndSingleLine`: a budget below the message size is raised to it, and one maximum-size line is
     accepted with nothing counted.
   * `TestQueuedBytesReleased`: `QueuedBytes()` returns to 0 after every handler.
   * `TestQueuedBytesRefusesRequest` (Q2 a): the answer, the limit, and the handler never runs.
   * `TestQueuedBytesDefault`: a connection given no option has a 64 MiB budget (Q1 a).
8. **Planted breaks**, each in a scratch copy, each failing its test:
   * no byte check in `waitForRoom`;
   * release at dequeue instead of completion;
   * no clamp;
   * no read-on limit on bytes;
   * no check under `OverflowCloseConnection`;
   * no drop under `OverflowDropNewest`;
   * no release in the request cleanup;
   * the default left unset.
9. **Checks.**
   * `go test -race ./...`, and `-count=3` on the new tests.
   * `tools/remote.py pkg` on the Linux and Windows hosts, with the new tests' `-test.run`.
   * `make pre-add-check`, `make api` (compatible additions only), `make check-records`, `make markdownlint`,
     `make disclosure-check`.
   * Then the execution record, and staging.

### Phase 2: documentation and an example

1. `README.md`, "Closing and limits": the budget, its default, what counts and when it is released, each policy at it, the
   request refusal (Q2 a), and `QueuedBytes`.
2. `docs/architecture.md`: the counter beside the sequence count, and the checks list.
3. `ExampleWithMaxQueuedBytes` in `example_api_test.go`, with fixed output, seen to fail with the option removed.
4. The phase 1 checks again, then the execution record and staging.

### Phase 3: close

1. `tools/gates.py --all`, with its output in the record.
2. After the owner pushes, `tools/watch_ci.py` on the commit.
3. A table of the MADR's nine Confirmation items, each with the test or command that shows it.
4. The MADR's status and the index rows.

### Phase 4: release, on the owner's ask

As [0044-PLAN-notification-queue-back-pressure.md](0044-PLAN-notification-queue-back-pressure.md)'s phase 4 ran it:

* `tools/consumer.py`, also setting `WithMaxQueuedBytes`, with `v1.30.0` in `WANT_RUN`;
* `docs/reports/0045-REPORT-release-notes-v1-30-0.md`, leading with the new default bound;
* the owner's tag, then `tools/publication.py` and `tools/consumer.py v1.30.0`.

## Verification

* Every test of step 1.7 passes under `-race` on macOS, and with `-count=3` on the Linux and Windows hosts, and was seen to fail on
  its planted break.
* The existing queue, barrier, overflow, hold and concurrency-limit tests pass unchanged, or each change is recorded with its
  reason.
* `make api`: compatible additions only, `WithMaxQueuedBytes` and `Connection.QueuedBytes`.
* `tools/gates.py --all` and CI pass.

## Rollout and Rollback

* **Rollout.** A minor, `v1.30.0`. With a default budget, a consumer on `OverflowCloseConnection` or `OverflowDropNewest` that
  keeps more than the budget queued now closes or drops. The release notes say so and name `WithMaxQueuedBytes` to raise it.
* **Rollback before the release.** Revert the phase commits.
* **Rollback after the release.** The option and accessor stay, under the compatibility promise. A patch can leave the budget
  unset by default (Q1 c's behaviour) while a problem is found.
