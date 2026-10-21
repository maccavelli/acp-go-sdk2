---
status: in-progress
date: 2026-10-21
associated-madr: "0030-MADR-record-initialize-on-arrival.md"
---

# Implement 0030: record a client's initialize when its answer arrives, and hold what comes before it

Associated MADR: [0030-MADR-record-initialize-on-arrival.md](0030-MADR-record-initialize-on-arrival.md)

Amended 2026-10-21, before any step ran, for the MADR's amendment of that date (issue #10, and the reader's buffer): the goal, facts F9 to
F17, the scope, the design, the tests, steps 4 to 7 (new; the former steps 4 to 6 are now 8 to 10), the planted breaks and the acceptance
line on `make api`. The design follows the owner's answers to the MADR's questions 5 to 11, the same day: taps on goroutines of their
own, detached at a 32 MiB queue, drained up to the grace; `Process.Kill`; breakaway allowed.

## Goal

Implement the MADR's O1 with the hold, as the owner decided questions 1 to 4 on 2026-10-21:

* **The commit** (question 1): the reader records the client's `initialize`, from the answer it has just read, before it reads the next
  message; `Initialize` returns what was recorded, or the commit's error.
* **The hold** (question 2): while the client's `initialize` is in flight, a received message that the gate would refuse only for want
  of a record waits until `initialize` completes or fails, then is judged.
* **A caller that gave up** (question 3): nothing recorded; a held message is then refused.
* **The test** (question 4): 200 trials in one write.

And the MADR's amendment of 2026-10-21:

* **The reader's buffer,** as the owner decided: 128 KiB.
* **Issue #10,** as the owner decided questions 5 to 11: `Command.OutputTap` and `Command.InputTap`, each on a goroutine of its own,
  with `Process.DroppedTapBytes`; `Process.Kill`; a job that allows breakaway.

~~`make api` must report no exported change.~~ (Amended 2026-10-21: `make api` must report `Command.OutputTap`, `Command.InputTap`,
`Process.Kill` and `Process.DroppedTapBytes` as compatible additions, and nothing else.) The release is the minor `v1.20.0`, with
[0029-PLAN-unix-client-death-pdeathsig.md](0029-PLAN-unix-client-death-pdeathsig.md)'s change; this PLAN writes its notes and the owner
tags it.

## Facts this plan rests on

F1 to F8 were read on `f193910`, and F9 to F17 on `631b128`, on 2026-10-21; step 1 re-asserts them with a script. The MADR's I1 to I8 hold
the initialize probe, and its amendment's T, K, W and B findings the others.

| Id | Fact | Where |
| :--- | :--- | :--- |
| F1 | the agent stores its record in the handler, before its answer is written | `agent_gen.go:400` |
| F2 | the client stores its record after `sendRequest` returns, on the caller's goroutine | `client_gen.go:494`–`:523` |
| F3 | the generator emits the client's `Store` | `cmd/generate/internal/emit/dispatch.go:690`–`:693` |
| F4 | `pendingResponse` holds only its channel | `connection.go:130`–`:132` |
| F5 | `handleResponse` hands the response over and returns to reading | `connection.go:1577`–`:1604`, called at `:1508` |
| F6 | `sendRequestSent` decodes and gates the result on the caller's goroutine | `connection.go:1807`, `:1820`, `:1823` |
| F7 | `gateMessage` refuses all but `initialize` with no record | `types_gen.go:23107`–`:23113` |
| F8 | the client's gate and extension gate are one, generated | `client_gen.go:102`–`:110`; `extensions.go:53`–`:58` |
| F9 | `Start` refuses a set `Stdout` and hands the connection the pipe's read end unwrapped | `agentproc/agentproc.go:110`, `:157` |
| F10 | `watch` and `stop` close the connection, which ends a read a descendant holds | `agentproc/agentproc.go:235`, `:298` |
| F11 | the connection's `Close` closes its reader and writer only when they are `io.Closer`s | `connection.go:798`–`:820` |
| F12 | `receive` reads through a 64 KiB `bufio.Reader` | `connection.go:1140` |
| F13 | `writeLoop` writes each message with one `Write` | `connection.go:705`–`:715` |
| F14 | `Close`'s ladder waits up to the grace at each step | `agentproc/agentproc.go:287`–`:299` |
| F15 | the watcher ends the tree when the agent exits, then gives the connection up to the grace | `agentproc/agentproc.go:222`–`:235` |
| F16 | the job is kill-on-close, with no breakaway limit | `agentproc/tree_windows.go:43` |
| F17 | `end` terminates the job once the agent has exited | `agentproc/tree_windows.go:74` |

## Scope

* **`connection.go`:** `pendingResponse` gains `commit`; `handleResponse` runs it; an unexported variant of `sendRequestSent` takes it.
  (Amended 2026-10-21: and `receive`'s buffer, 128 KiB, as a constant.)
* **The generator,** `cmd/generate/internal/emit/dispatch.go`: the client's `Initialize` with a commit; the client's state for an
  `initialize` in flight; the client's gate that holds. Regenerated: `client_gen.go`. The engine cases' `want.api` should not change; the
  pins in `emit/dispatch_test.go` follow the emitted code.
* **(Added 2026-10-21) `agentproc`:** `agentproc.go`, the two taps' fields, `Kill` and `DroppedTapBytes`; `tap.go`, new, the tap's
  queue, goroutine, reader and writer; `tree_windows.go`, the breakaway limit; the tests and child modes below; an example of a tap in
  `example_test.go`.
* **Tests:** at the root and in `agentproc`, below; in the generator, the pins.
* **Docs:** the README's paragraph on the record (`README.md:382`–`:384`); `docs/architecture.md`'s gate paragraph (`:275`–`:286`).
  (Amended 2026-10-21: and the README's "Closing and limits" paragraph on `agentproc`; `docs/architecture.md`'s paragraph on `receive`
  (`:289`) and its section "The agent process helper"; the docs of `Command`, `Process.Kill` and `Start`.)
* **The records:** this PLAN and its row; the release notes `docs/reports/0030-REPORT-release-notes-v1-20-0.md` and their row, covering
  0029 too.

## Design

### The commit

* ~~`pendingResponse{ch, commit func(result json.RawMessage) error}`.~~ (Amended 2026-10-21, during step 2: see the deviation during step 2.)
* `handleResponse`, for a pending request with a commit, whose response is a result: runs the commit before it hands the response over;
  the envelope carries the commit's error. An error response runs no commit.
* ~~`sendRequestCommit[T]`, beside `sendRequestSent`, registers the commit with the pending request and returns the commit's error, when
  there is one, before decoding; otherwise it decodes as `sendRequestSent` does. The commit decodes the answer once more; it is one small
  message per connection.~~ (Amended 2026-10-21, during step 2: see the deviation during step 2.)
* The emitted `Initialize`: its commit decodes the `InitializeResponse`, checks the version (`ProtocolVersionError`), applies the result
  gate, `jsonCopy`s the record and `Store`s it, exactly as today's code after `sendRequest`, then marks `initialize` complete. (Amended 2026-10-21, during step 2: see the deviation during step 2.)

### The hold

* The client gains, generated, an `initialize` in-flight state: a pointer to the current attempt, whose `done` channel closes once, when
  the attempt completes (the commit stored the record) or fails.
* `Initialize` starts an attempt before it sends; a deferred function fails it unless it completed: an error answer, the commit's error,
  the caller's context, the connection's end.
* The client's gate, when `gateMessage` would refuse only for want of a record (`errors.Is(err, ErrNotInitialized)`) and an attempt is
  in flight: waits for the attempt's `done` or the connection's `Done`, then judges again against the record. No attempt in flight:
  refuses at once, as today.
* The agent's side is unchanged.

### The reader's buffer (added 2026-10-21)

* `connection.go`: `const readBufferSize = 128 << 10`, with a comment citing the MADR's amendment; `receive` reads through
  `bufio.NewReaderSize(c.r, readBufferSize)`. Both sides share it. Nothing else reads its size.

### The taps (added 2026-10-21)

* `Command` gains `OutputTap io.Writer` and `InputTap io.Writer`, after `ConnectionOptions`, each documented as the MADR's specification
  says: what reaches it and in what order, that a goroutine of its own calls it, that its error is ignored, that it is detached once 32 MiB
  behind, that `Wait` waits up to the grace for it, that its `Write` must return and must not retain `p`, and that one writer set as both
  must be safe for concurrent use.
* `tap.go`, new: `tap{w io.Writer; mu sync.Mutex; queue [][]byte; queued int; ended, detached bool; dropped uint64; wake chan struct{};
  done chan struct{}}`, with:
  * `offer(p)`: under the lock, when detached or when `queued+len(p)` would pass `tapQueueBytes` (`32 << 20`), marks it detached and adds
    `len(p)` to `dropped`; otherwise queues `bytes.Clone(p)` and wakes the goroutine. It never waits.
  * `end()`: marks the stream ended and wakes the goroutine.
  * `run()`: the goroutine; takes the whole queue under the lock, writes each copy outside it, ignoring the error, and returns once the
    stream has ended and the queue is empty, closing `done`.
  * `abandon()`: detaches, and adds what is still queued to `dropped`; the goroutine writes nothing more after the `Write` in progress.
* `tapReader{f *os.File; t *tap}`: `Read` reads `f`, offers `p[:n]` when `n > 0`, calls `end` on an error, and returns `f`'s `n` and error.
  `Close` calls `end` and closes `f`, so the connection's `Close` still ends a read (F10, F11).
* `tapWriter{w *sharedClose; t *tap}`: `Write` writes `w`, then offers `p[:n]` when `n > 0`, and returns `w`'s `n` and error. `Close` calls
  `end` and closes `w`, which `stop` also closes, once (`sharedClose`); `watch` and `stop` call `end` on it too, so that an input the
  connection never closes still ends its tap.
* `Start` passes the connection `outR` and `p.stdin` when the taps are unset, and the tap types when set; an unexported helper builds the
  pair, so the test can see it. Each tap's goroutine is started there and is not in `p.goroutines`.
* `Wait`, after `p.goroutines.Wait()` and the connection's `Wait`, waits for each tap's `done` or the grace, whichever comes first, from
  one timer, then calls `abandon` on any tap not done. It does so once; later calls return the same result.
* `Process.DroppedTapBytes() (output, input uint64)` reads each tap's `dropped` under its lock; zero for a tap unset.

### Kill (added 2026-10-21)

* `Process` gains `hurry chan struct{}`, closed once by `Kill` (`sync.Once`), made in `Start`.
* `exitedWithin` also returns false when `hurry` is closed; `stop` skips the terminating step once `hurry` is closed.
* `Kill` closes `hurry`, runs `closeOnce.Do(p.stop)`, and returns `p.Wait()`. A `Close` already in its steps wakes from its wait and kills;
  `Kill` then waits for those steps to end, through `closeOnce`. `signal` already sends nothing once the agent has exited. `Wait`'s drain
  of the taps is unchanged: a tap that keeps up adds nothing measurable (the MADR's A2).
* Documented beside `Close`: what it does, that it ends the processes the agent started, that it returns `ErrKilled` joined when it
  killed, and that a `Close` in progress is cut short. `ErrKilled`'s doc, "when Close killed …" (`agentproc/agentproc.go:27`), names
  `Kill` too.

### Breakaway (added 2026-10-21)

* `attach` sets `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE | JOB_OBJECT_LIMIT_BREAKAWAY_OK`; its comment cites W3 and W4. `Start`'s doc and the
  README say that a process started with `CREATE_BREAKAWAY_FROM_JOB` leaves the job, and is then the agent's to end.

## Tests

At the root, a fake agent over `io.Pipe`s, as the MADR's probe:

* **`TestInitializeRecordedOnArrival`:** the answer, an extension notification and an extension request in one write, 200 times; each
  notification handled and each request answered with a result.
* **`TestHeldUntilInitialized`:** the extension notification and request in one write *before* the answer; both handled once
  `Initialize` returns, 200 times.
* **`TestHeldRefusedWhenInitializeFails`:** the same two before an error answer; before an answer with another protocol version; with the
  caller's context ended first; with the connection closed first: each request answered `ErrNotInitialized`'s `-32601`, each notification
  not handled, and none held past `Initialize`'s return.
* **`TestNotHeldBeforeInitialize`:** a message before `Initialize` is called is refused at once, as today.
* **`TestAgentRecordsBeforeAnswering`:** a raw client sends `session/new` the moment it reads the answer; answered, 200 times (the MADR's
  I8).
* **The pins:** the emitted `Initialize` registers a commit; the emitted gate holds.
* **(Added 2026-10-21) `TestReceiveReadBufferSize`:** a reader that records the length of the buffer each `Read` is given, handed to
  `NewConnection`; the first is 131,072 bytes.

(Added 2026-10-21.) In `agentproc`, with the test binary as the agent:

* **`TestOutputTapVerbatim`** (T1): a child mode `frames` writes a 2 MiB line, a line that is not JSON and a notification, then exits at
  its input's end; with `WithMaxMessageSize(1 << 20)`, the output tap's bytes after `Close` equal what the agent wrote.
* **`TestInputTapSeesWrites`** (T1): in the same run, the input tap holds the `-32600` and `-32700` answers, each a whole line.
* **`TestTapKeepsCloseEnding`** (T2, Unix): `parent-escape` with an output tap; `Wait` returns within `testGrace` and the slack.
* **`TestTapErrorIgnored`:** both taps return an error on every `Write`; the `burst` agent's 1000 updates all arrive, as in
  `TestNoLostOutput`, and `Wait` returns nil.
* **`TestTapsUnset`:** with neither tap set, the helper hands the connection the pipe's read end and the shared input themselves, and no
  tap goroutine runs.
* **`TestBlockedTapDetached`** (A1f): a child mode `bulk` writes 48 MiB of lines, then a notification, then exits at its input's end; an
  output tap whose `Write` blocks until the test releases it. The client receives the notification, `Close` returns within the grace and
  the slack, `DroppedTapBytes` reports output bytes dropped, and, released, the tap receives an exact prefix of what the agent wrote.
* **`TestDetachedTapIsPrefix`:** the same agent writing 64 MiB; the tap blocks until `DroppedTapBytes` reports a drop, then is released
  while the agent still writes; what it received is an exact prefix, and nothing after the first dropped byte.
* **`TestTapDrainBoundedByGrace`** (A1e): a tap that sleeps 1 ms a write, the `bulk` agent, a grace of 300 ms; `Wait` returns within the
  grace and the slack after the agent's exit, and `DroppedTapBytes` reports the rest.
* **`TestKill`** (K2): `parent-wait`, whose beat grandchild runs, at the default grace; `Kill` returns within the slack, `errors.Is(err,
  ErrKilled)`, and, where there is a tree, the grandchild has stopped.
* **`TestKillCutsCloseShort`:** a `stubborn` agent at a grace of 5 s; `Close` in a goroutine, then `Kill` 100 ms later; both return within
  the slack, with the same error, `ErrKilled` joined.
* **`TestKillAfterExit`:** an agent that exits of itself; `Kill` returns `Wait`'s error, without `ErrKilled`.
* **`TestBreakawayLeavesJob`** (W3c, W3d; Windows): a child mode `parent-breakaway` starts one beat with `CREATE_BREAKAWAY_FROM_JOB` and
  one without, and records both process IDs; after `Close`, the first still beats and the second has stopped; the cleanup kills the first.
* **`ExampleCommand_outputTap`:** compiles; a capture file as the output tap, with no `// Output:`.

## Implementation Steps

Each step ends with its gates and an execution record. Agents stage; a commit or a push on `main` needs the owner's explicit permission in
that turn (`AGENTS.md`, "Commits"). Every new test or check is seen to fail on a planted break in a scratch copy, never in the tree, before
it is relied on.

1. **Records:** this PLAN and its row `in-progress`; the facts re-asserted by a script. (Amended 2026-10-21: F9 to F17 too.)
2. **The commit:** `connection.go`; the emitter's `Initialize`; regeneration; `TestInitializeRecordedOnArrival`,
   `TestAgentRecordsBeforeAnswering`; the pins. (Amended 2026-10-21, during step 2: and `TestInitializeHooksRunOnce`,
   `TestInitializeRefusedByHookRecordsNothing`, `TestInitializeGiveUpAgrees`.)
3. **The hold:** the emitter's in-flight state and gate; regeneration; `TestHeldUntilInitialized`, `TestHeldRefusedWhenInitializeFails`,
   `TestNotHeldBeforeInitialize`; the pins.
4. **(Added 2026-10-21) The reader's buffer:** `readBufferSize`; `TestReceiveReadBufferSize`.
5. **(Added 2026-10-21) The taps:** `Command.OutputTap`, `Command.InputTap`, `tap.go`, `Process.DroppedTapBytes`; the child modes
   `frames` and `bulk`; the eight tap tests; `ExampleCommand_outputTap`; `make api`, which reports the two fields and the method.
6. **(Added 2026-10-21) Kill:** `Process.Kill` and `hurry`; `TestKill`, `TestKillCutsCloseShort`, `TestKillAfterExit`; `make api`, which
   reports the method.
7. **(Added 2026-10-21) Breakaway:** `attach`'s limit; the child mode `parent-breakaway`; `TestBreakawayLeavesJob`, on the Windows host
   and in CI's Windows jobs.
8. **Docs:** the README and `docs/architecture.md`; each claim mapped to a test. (Was step 4; amended 2026-10-21: the paragraphs the
   scope adds.)
9. **Release notes:** `docs/reports/0030-REPORT-release-notes-v1-20-0.md` and its row, for 0029 and 0030, with `make api`'s report. (Was
   step 5; amended 2026-10-21: naming issue #10, `Command.OutputTap`, `Command.InputTap`, `Process.Kill`, `Process.DroppedTapBytes`, the
   breakaway and the buffer.)
10. **Verification and close:** the gates; the Linux and Windows hosts and `make push-guard` before the push; after CI's six jobs pass,
    this PLAN and its row `complete`, and 0029's if it waits on the release; the owner tags `v1.20.0` and publishes its release; the
    publication checks and a consumer on the floor. (Was step 6; amended 2026-10-21: the owner closes issue #10 with the release.)

## Verification

Per step: phase 1's gates on go1.27.3, regeneration included; `make api`; the floor gate at go1.26.10; the root tests on the Linux and
Windows hosts. (Amended 2026-10-21: and, from step 4, `make lint`, `golangci-lint` for `GOOS=linux` and `GOOS=freebsd` on `agentproc`, and
`go vet ./agentproc` for the platforms of 0029; the `agentproc` tests on both hosts.)

Planted breaks, each in a scratch copy:

| For | Break | The check that must fail |
| :--- | :--- | :--- |
| the commit | the record made by the caller again, the commit removed | `TestInitializeRecordedOnArrival` |
| the commit | the commit run after the hand-over | `TestInitializeRecordedOnArrival` |
| the commit | the version check left out of the commit | `TestHeldRefusedWhenInitializeFails`, another version |
| the commit (added 2026-10-21, during step 2) | the caller decoding and checking the answer again after the commit | `TestInitializeHooksRunOnce` |
| the commit | the commit applying the gate alone, the hooks left to the caller | `TestInitializeRefusedByHookRecordsNothing` |
| the commit | `waitForResponse` returning the caller's error though the reader has claimed the answer | `TestInitializeGiveUpAgrees` |
| the hold | the gate refusing at once, as today | `TestHeldUntilInitialized` |
| the hold | a failure not closing `done` | `TestHeldRefusedWhenInitializeFails`, by its bound |
| the hold | an attempt not failed when the caller gives up | `TestHeldRefusedWhenInitializeFails`, the caller's context |
| the hold | messages held with no attempt in flight | `TestNotHeldBeforeInitialize`, by its bound |
| the emitter | the pins' expected code changed | the pins |
| the buffer (added 2026-10-21) | 64 KiB again | `TestReceiveReadBufferSize` |
| the taps (added 2026-10-21) | the output tap given `p[:n-1]` | `TestOutputTapVerbatim` |
| the taps | the input tap never called | `TestInputTapSeesWrites` |
| the taps | `tapReader` without `Close` | `TestTapKeepsCloseEnding`, by its bound (the MADR's T3) |
| the taps | the tap's error returned as the read's, as `io.TeeReader` does | `TestTapErrorIgnored` |
| the taps | a `tapReader` even when no tap is set | `TestTapsUnset` |
| the taps | the copy written on the reader's goroutine, as a synchronous tap | `TestBlockedTapDetached`, by its bound |
| the taps | no bound on the queue | `TestBlockedTapDetached`, no bytes dropped |
| the taps | a full queue's copy dropped and later ones queued | `TestDetachedTapIsPrefix` |
| the taps | `Wait` not waiting for the taps' drain | `TestOutputTapVerbatim`, bytes missing at `Wait` |
| the taps | `Wait` waiting for the drain without a bound | `TestTapDrainBoundedByGrace`, by its bound |
| Kill (added 2026-10-21) | `Kill` running `Close` | `TestKill`, by its bound |
| Kill | `exitedWithin` not watching `hurry` | `TestKillCutsCloseShort`, by its bound |
| Kill | `ErrKilled` recorded though nothing was sent | `TestKillAfterExit` |
| breakaway (added 2026-10-21) | `BREAKAWAY_OK` left out | `TestBreakawayLeavesJob` (Windows host) |

Acceptance:

* every step's checks pass and its record is written; every claim the new docs make is mapped to a test;
* the tests pass on macOS, on the Linux and Windows hosts, and in CI's six jobs;
* `make lint` reports 0 issues, `go fix -diff` prints nothing, `go mod tidy -diff` is clean;
* ~~`make api` reports no exported change.~~ (Amended 2026-10-21: `make api` reports `Command.OutputTap`, `Command.InputTap`,
  `Process.Kill` and `Process.DroppedTapBytes` as compatible, and nothing incompatible.)

## Rollout and Rollback

Each step is one commit, reverted by reverting it; steps 2 and 3 revert the generated code with the emitter.

For a consumer: a client no longer refuses a message its agent sends right after, or before, its `initialize` answer; what that answer
does not allow is still refused once the record exists. The release is the minor `v1.20.0`, which the owner confirms at the tag.

(Added 2026-10-21.) And:

* a connection reads through a 128 KiB buffer, 64 KiB more than before, on either side;
* `agentproc` can copy the agent's output and input to writers the caller gives it, each on a goroutine of its own, detached and counted
  should it fall 32 MiB behind, and end an agent at once with `Process.Kill`;
* on Windows, a process the agent starts with `CREATE_BREAKAWAY_FROM_JOB`, refused before, now starts outside the job and outlives the
  agent; every other process ends with the job as before.

## Execution record

### Step 1, records, 2026-10-21

The owner committed the MADR's amendment and this PLAN's as `c2b33c1`, and approved execution. What changed: this PLAN and its
`docs/README.md` row `in-progress`.

**The facts,** re-asserted by a script outside the tree, reading `c2b33c1`'s tree, each fact from lines no other fact reads: 17 of 17
hold.

| Id | What the script read |
| :--- | :--- |
| F1 | `agent_gen.go:400` is `a.negotiation.Store(&rec)`, and `:401` `return result, nil` |
| F2 | `client_gen.go:494` begins `Initialize`; `:501` calls `sendRequest[InitializeResponse]`; `:523` is `c.negotiation.Store(&rec)` |
| F3 | `emit/dispatch.go:690` emits the `jsonCopy`, and `:693` the `Store` |
| F4 | `connection.go:130`–`:132`: `pendingResponse` holds `ch chan responseEnvelope` alone |
| F5 | `:1577` begins `handleResponse`; `:1603` sends the envelope on `pr.ch`; `:1508` calls it |
| F6 | `:1807` waits for the response; `:1820` decodes the result; `:1823` gates it |
| F7 | `types_gen.go:23107`–`:23112`: `gateMessage` passes `initialize` and returns `ErrNotInitialized` with no record |
| F8 | `client_gen.go:102`–`:103`: `inboundGate` returns `c.gate`; `:109` calls `gateMessage`; `extensions.go:55` passes `c.inboundGate()` |
| F9 | `agentproc/agentproc.go:110` is `case cmd.Stdout != nil:`; `:157` hands the connection `outR` |
| F10 | `:235` and `:298` are `_ = p.conn.Close()` |
| F11 | `connection.go:798` begins `closeWith`; `:807` is `cl, ok := x.(io.Closer)`; it ends at `:820` |
| F12 | `:1140` is `r := bufio.NewReaderSize(c.r, 64*1024)` |
| F13 | `:705` begins `writeLoop`; `:711` is `_, err := c.w.Write(req.b)`; it ends at `:715` |
| F14 | `agentproc/agentproc.go:287` begins `stop`; `:292` and `:293` wait a grace each; it ends at `:299` |
| F15 | `:222` begins `watch`; `:225` is `p.tree.end()`; `:229` starts the grace's timer |
| F16 | `agentproc/tree_windows.go:43` sets `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE`, and the file names no breakaway limit |
| F17 | `:74` begins `end`; `:75` terminates the job |

**Planted breaks,** each a change to one line in place, in its own scratch copy of the tree; each made the script report its own fact
alone missing, exit 1: 17 of 17.

A first run caught 16 of 17: F6 then tested that its lines contained a call, and its break, a comment added after the call, left the call
in place. F6 now tests its three lines whole, as the other facts do, and the run above is the second.

**Gates:** `make check-records` and `make markdownlint` exit 0; the identifier scan finds nothing in the changed records.

### Deviation, 2026-10-21, during step 2: the commit's checks, and a caller that gives up

**Found,** reading the code before any change, at `c2b33c1`:

1. **The hooks decide the record.** `Initialize` stores the record only once `sendRequest` has returned without error
   (`client_gen.go:507`–`:523`), and `sendRequestSent` has by then run `checkInbound` on the decoded answer (`connection.go:1823`): the
   rewrite hook, which may change it, the validation, the validation hook, and the gate (`:2156`–`:2181`). So the record is the answer as
   the hooks left it, and none is made when a hook refuses it. The design above, a commit that applies the gate alone and a caller that
   then decodes and checks as `sendRequestSent` does, would record the answer before the rewrite hook, and record it when a hook refuses;
   checking in both places would run the hooks twice.
2. **A caller can give up after the reader has claimed the answer.** `handleResponse` removes the pending request (`:1584`–`:1588`) before
   it hands the answer over (`:1603`); the commit runs between. A context that ends then takes `waitForResponse`'s `ctx.Done` branch, which
   returns the context's error (`:1960`–`:1975`), and the connection's end its `Done` branch, which finds no answer yet (`:1976`–`:1981`):
   `Initialize` would fail while the record exists.
3. **The wait for earlier notifications** (`waitNotificationsUpTo`, `:1811`, `:1985`–`:2023`) also ends with the caller's context or the
   connection, after the commit.

**Decision (owner, 2026-10-21).**

1. The commit runs today's whole path after the answer, once, on the reader: decode, `checkInbound` with the caller's context, the version,
   `jsonCopy`, `Store`; the caller returns the value the commit checked, and decodes nothing. The alternative was this PLAN's design as
   written.
2. A caller that gives up after the reader has claimed the answer waits for the commit and returns its result, so that `Initialize`'s
   return and the record always agree; a request without a commit is unchanged. The alternative was to leave the race. By the same rule,
   the owner's, a committed answer is returned when the wait for earlier notifications then ends with the context or the connection: the
   record exists.

**The design, as amended:**

* `pendingResponse{ch, commit func(result json.RawMessage) (any, error)}`; `responseEnvelope` gains `committed any` and `commitErr error`.
* `handleResponse`, for a pending request with a commit and a response with no error: runs the commit before it hands the envelope over,
  and puts its value and error in it.
* `sendRequestCommit[T]`, beside `sendRequestSent`: as `sendRequestSent` up to the answer; an error response is returned as it is; a commit's
  error is returned; otherwise it waits for the earlier notifications, ignoring that wait's error, and returns the committed value.
* `waitForResponse`, for a request with a commit, when the context, the input or the connection ends: under the lock, a request still pending
  is removed and the error returned, as today; one the reader has claimed is waited for, and its envelope returned.
* Found by `TestInitializeGiveUpAgrees`'s first run, the same race on a second path: a send whose context ends while the writer still
  writes returns the context's error (`writeLoop`: "A write that has begun runs to its end"), after which the agent's answer can be read
  and committed. `exchange`'s send-failure path makes the same check, through `claimedAnswer`, which both paths call.
* The emitted `Initialize`: its commit does what `sendRequestSent` and today's `Initialize` do after the answer, in that order: an empty
  result decodes to the zero value; a result is decoded (`NewInternalError` on failure) and checked with `checkInbound`, wrapped as
  "%s response: %w"; then the version (`ProtocolVersionError`), `jsonCopy` and `Store`. Its errors are today's.

**Added to step 2:**

* `TestInitializeHooksRunOnce`: a rewrite hook that counts its calls for `initialize` and changes the answer's `agentInfo`; it runs once,
  and the record and `Initialize`'s return both carry its change.
* `TestInitializeRefusedByHookRecordsNothing`: a validation hook that refuses the `initialize` answer; `Initialize` returns its error, and a
  `session/update` sent afterwards is refused, `ErrNotInitialized`.
* `TestInitializeGiveUpAgrees`: a rewrite hook that, on the reader, ends the caller's context, and in a second case closes the connection;
  in each, `Initialize` succeeds and the record exists, 200 times.

The MADR gets an amendment for the same; its decision, O1, is unchanged.

### Step 2, the commit, 2026-10-21

The owner committed step 1 and approved this step; the deviation above was decided before any code changed. What changed:

* **`connection.go`:** `pendingResponse.commit` and `responseEnvelope`'s `committed` and `commitErr`; `handleResponse` runs the commit
  before it hands the envelope over; `sendRequestCommit[T]`; `exchange`, the sending and waiting `sendRequestSent` did, now shared by both;
  `claimedAnswer`, called by `waitForResponse` when the context, the input or the connection ends and by `exchange` when the send fails.
* **The emitter** (`cmd/generate/internal/emit/dispatch.go`): `initializeWrapper` returns `sendRequestCommit` with a commit that checks
  the version, `jsonCopy`s the record and stores it; `outboundDoc` gives `Initialize` a paragraph saying when the answer is recorded.
  Regenerated: `client_gen.go`, `Initialize` alone.
* **The pins** (`emit/dispatch_test.go`): the `Hello` wrapper's `sendRequestCommit` call, its commit and its doc line.
* **Tests,** `initialize_test.go`, new: `TestInitializeRecordedOnArrival`, `TestAgentRecordsBeforeAnswering`, `TestInitializeHooksRunOnce`,
  `TestInitializeRefusedByHookRecordsNothing`, `TestInitializeGiveUpAgrees`.

`TestInitializeRefusedByHookRecordsNothing` shows the refusal after a refused answer with an extension request, answered `-32601` for
`ErrNotInitialized`, where the deviation names a `session/update`: a refused notification has no answer to show it.

**The claims, each to what makes it:**

| Claim | Check |
| :--- | :--- |
| what the agent sends in the write that carries its answer is judged by the answer (the MADR's I6) | `TestInitializeRecordedOnArrival`, 200 trials |
| the agent records before it answers (I1, I8) | `TestAgentRecordsBeforeAnswering`, 200 trials |
| the hooks see the answer once, and the record is the answer as they left it | `TestInitializeHooksRunOnce` |
| an answer a hook refuses records nothing | `TestInitializeRefusedByHookRecordsNothing` |
| `Initialize`'s return and the record agree when the caller gives up or the connection ends after the claim | `TestInitializeGiveUpAgrees`, 200 trials each |
| an answer with another version fails and records nothing | `TestProtocolVersion`, existing |
| the emitter writes the commit | `TestDispatchFollowsTheModel` |
| no exported change | `make api` |

**Found during the step, within the deviation's decision:** `TestInitializeGiveUpAgrees`'s first run, 5 runs of 200 trials, failed in each,
at trials 98 to 195: `Initialize` returned "Request cancelled" while the record was still being made. The caller had given up inside
`sendMessageSent`, whose write runs to its end, so `exchange`'s send-failure path returned before the claim check `waitForResponse` makes.
That path now calls `claimedAnswer` too, as the deviation's design records; 10 runs then passed.

**Planted breaks,** each in its own scratch copy; 8 of 8 failed as required:

| Break | The check that failed |
| :--- | :--- |
| the record made by the caller again, the commit removed | `TestInitializeRecordedOnArrival`, trial 0: `_x/ask` answered `-32601`, "initialize has not completed" |
| the commit run by the caller after the hand-over | `TestInitializeRecordedOnArrival`, trial 0, the same |
| the caller decoding and checking the answer again | `TestInitializeHooksRunOnce`: "saw the initialize answer 2 times, want once" |
| the commit applying the gate alone, the hooks left to the caller | `TestInitializeRefusedByHookRecordsNothing`: "a record was made of an answer the hook refused" |
| no claim check | `TestInitializeGiveUpAgrees`, trial 0: "connection ended before response" with no record |
| no claim check on the send path alone | `TestInitializeGiveUpAgrees`: "Request cancelled" with no record; caught in 6 of 6 runs, at trials 5 to 130 |
| the agent recording after its answer, in a goroutine | `TestAgentRecordsBeforeAnswering`, trial 0: `session/new` answered `-32601` |
| the emitted call renamed | `TestDispatchFollowsTheModel`: "lacks `return sendRequestCommit[HelloResponse]`" |

The PLAN's third break for the commit, the version check left out of it, is listed against step 3's `TestHeldRefusedWhenInitializeFails`;
run now against the root package, the existing `TestProtocolVersion` failed: "Initialize = <nil>, want *ProtocolVersionError{Requested: 1,
Answered: 2}" and "the client recorded an answer it refused".

**Gates,** each exit 0, run one at a time. Their first run failed `make lint`, 4 issues: an unchecked type assertion and an evaluation
order in `sendRequestCommit`, and two unused `nolint` directives in the test; fixed, the planted breaks were run again on the fixed code,
8 of 8, and the gates again:

* go1.27.3: regeneration unchanged; `make pre-add-check`, "170 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck)", and
  the new file's, "1 file(s) clean"; `GOEXPERIMENT=nojsonv2 go test ./...`; `GOOS=windows go vet ./...` in both modules;
  `make check-records`, `make markdownlint` and the identifier scan; `make lint`, "0 issues." four times; `go fix -diff ./...` and
  `go mod tidy -diff` print nothing, in both modules;
* `make api` against `v1.19.0`: "0 incompatible, 0 compatible";
* `go test -race -count=3 -shuffle=on .`;
* the Windows host: `go test -count=1 -shuffle=on ./...` in both modules; the Linux host: the same, from a copy of the tree;
* the floor gate at go1.26.10: `go vet` and `go test -race -count=1` in both modules, the leak guards on.
