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
  `initialize` in flight; the client's gate that holds. Regenerated: `client_gen.go`. ~~The engine cases' `want.api` should not change;~~ (Amended 2026-10-21, during step 3:
  four of them change by one line; see the deviation during step 3.) The pins in `emit/dispatch_test.go` follow the emitted code.
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

### The suspended start on Windows (added 2026-10-21, during step 5)

* `prepare`, on Windows, refuses a `Cmd` whose `SysProcAttr.CreationFlags` has `CREATE_SUSPENDED`, starting nothing, as Unix's refuses a
  shared `Pgid`; otherwise it adds `CREATE_SUSPENDED` to them, keeping the caller's other flags, creating `SysProcAttr` when it is nil.
* `attach` assigns the agent to its job, then resumes it: a thread snapshot (`CreateToolhelp32Snapshot`, `TH32CS_SNAPTHREAD`) lists the
  agent's threads, one for a new suspended process, and each is opened (`OpenThread`, `THREAD_SUSPEND_RESUME`) and resumed
  (`ResumeThread`). An error in either returns `attach`'s error, and `Start` abandons the agent, as for any `attach` error.
* `attachDelay`, an unexported `atomic.Int64` in `tree_windows.go`, zero in production: `attach` sleeps that long before the assignment,
  so that a test can make it late (the MADR's amendment, question 13).
* `attach`'s comment no longer says that a process the agent starts before the assignment is outside the job; it cites the amendment.

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
* **(Added 2026-10-21, during step 5) `TestDescendantInJobDespiteSlowAttach`** (Windows, not parallel): `attachDelay` at 200 ms, the
  `parent-exit` agent, whose beat grandchild starts at once; once the agent has exited and `Wait` returned, the beat has stopped.
* **`TestStartRefusesSuspended`** (Windows): a `Cmd` with `CREATE_SUSPENDED` is refused, and nothing started.
* **`TestCallersCreationFlagsKept`** (Windows): a caller's `CREATE_NEW_PROCESS_GROUP` is kept beside `CREATE_SUSPENDED`, and the agent runs.

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
6. **(Added 2026-10-21, during step 5) The suspended start on Windows:** `prepare`'s flag and refusal, `attach`'s resume, `attachDelay`;
   `TestDescendantInJobDespiteSlowAttach`, `TestStartRefusesSuspended`, `TestCallersCreationFlagsKept`; on the Windows host, and the whole
   package there 30 times. Its docs are step 9's.
7. **(Added 2026-10-21) Kill:** `Process.Kill` and `hurry`; `TestKill`, `TestKillCutsCloseShort`, `TestKillAfterExit`; `make api`, which
   reports the method. (Was step 6; renumbered 2026-10-21, during step 5.)
8. **(Added 2026-10-21) Breakaway:** `attach`'s limit; the child mode `parent-breakaway`; `TestBreakawayLeavesJob`, on the Windows host
   and in CI's Windows jobs. (Was step 7; renumbered 2026-10-21, during step 5.)
9. **Docs:** the README and `docs/architecture.md`; each claim mapped to a test. (Was step 4; amended 2026-10-21: the paragraphs the
   scope adds; renumbered from 8, during step 5, and the suspended start added.)
10. **Release notes:** `docs/reports/0030-REPORT-release-notes-v1-20-0.md` and its row, for 0029 and 0030, with `make api`'s report. (Was
    step 5; amended 2026-10-21: naming issue #10, `Command.OutputTap`, `Command.InputTap`, `Process.Kill`, `Process.DroppedTapBytes`, the
    breakaway and the buffer; renumbered from 9, during step 5, and the suspended start added.)
11. **Verification and close:** the gates; the Linux and Windows hosts and `make push-guard` before the push; after CI's six jobs pass,
    this PLAN and its row `complete`, and 0029's if it waits on the release; the owner tags `v1.20.0` and publishes its release; the
    publication checks and a consumer on the floor. (Was step 6; amended 2026-10-21: the owner closes issue #10 with the release;
    renumbered from 10, during step 5.)

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
| the suspended start (added 2026-10-21, during step 5) | `CREATE_SUSPENDED` not added, the resume kept | `TestDescendantInJobDespiteSlowAttach` |
| the suspended start | the resume before the assignment | `TestDescendantInJobDespiteSlowAttach` |
| the suspended start | a caller's `CREATE_SUSPENDED` accepted | `TestStartRefusesSuspended` |
| the suspended start | the caller's flags replaced by `CREATE_SUSPENDED` | `TestCallersCreationFlagsKept` |

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

### Deviation, 2026-10-21, during step 3: the engine cases' `want.api`

**Found.** The hold's state, `initializing atomic.Pointer[initializeAttempt]`, a field of the generated `ClientSideConnection` beside
`negotiation`, changes the `want.api` of the engine cases that have an `initialize`: `TestEngineCases` lists a side connection's
unexported fields too. Measured in a scratch copy with `-update`, 4 of the 172 cases change, by that one line each:
`accept-capability-gates`, `accept-capability-undeclared-variant`, `accept-initialize-negotiation` and `accept-untyped-object-gates`. The
scope above expected none to change.

**Decision (owner, 2026-10-21).** Accept the four, rewritten by `-update`; the alternative was to keep the attempt on the hand-written
`Connection`, apart from the record it gates on, only so that the snapshots stay. `make api` still reports no exported change. The MADR is
unchanged.

### Step 3, the hold, 2026-10-21

The owner committed step 2 and approved this step; the deviation above was decided during it. What changed:

* **`connection.go`:** `pendingResponse.settle`, which `handleResponse` runs once it has judged an answer, after the commit for a result
  and at once for an error answer; `sendRequestCommit` takes `settled`, set as `settle`, and runs it itself when no answer will come;
  `initializeAttempt`, whose `done` closes once, and `holdUntilInitialized`, which holds a message the gate refuses only for want of a
  record while an attempt is in flight, until the attempt ends or the connection does.
* **The emitter:** the side descriptor's `holds`, true for the client when the schema has an `initialize`; the client's struct gains
  `initializing atomic.Pointer[initializeAttempt]`; a generated `heldGate` that calls `holdUntilInitialized`; `inboundGate` and the client's
  typed dispatch use it, while its outbound methods keep `gate`; `Initialize` stores a new attempt before it sends and passes `attempt.end`
  as `settled`. Regenerated: `client_gen.go` alone. The engine cases: four `want.api`, by one line each (the deviation).
* **The pins:** the attempt, `heldGate`, `inboundGate` and the client's inbound checks; the agent's file names none of them, and the
  client's outbound methods do not call `heldGate`.
* **Tests,** in `initialize_test.go`: `TestHeldUntilInitialized`, `TestHeldRefusedWhenInitializeFails` (four cases),
  `TestNotHeldBeforeInitialize`; a validation hook reports each extension message's verdict, which shows a refused notification too.

**Settled on the reader, by design:** an answer that ends the attempt only when `Initialize` returns would deadlock on an error answer:
`sendRequestCommit` waits for the notifications received before the answer, and a held one waits for the attempt. So the reader ends the
attempt once it has judged any answer, and the caller only when no answer will come.

**Checked, as the MADR states:** the hold cannot block the answer it waits for. The reader never waits for a request slot, answering a
request over the limit at once (`connection.go`, the comment citing B18), nor for room in the notification queue, which drops or closes
(the comment "The reader must not block here"). A consequence the MADR's "Neutral" names in part: held notifications stay in the queue, so
an agent that sends more notifications before its answer than the queue holds meets the overflow policy, where before they were refused one
by one.

**A first run** of the new tests failed in `TestHeldRefusedWhenInitializeFails`, the caller's context: the test read the client's
`$/cancel_request` for its `initialize` as the extension request's answer. The test now passes over that line; 10 runs with `-race` then
passed.

**The claims, each to what makes it:**

| Claim | Check |
| :--- | :--- |
| a message the agent sends before its answer is held, then judged by the answer | `TestHeldUntilInitialized`, 200 trials |
| held messages are refused when the attempt fails: an error answer, another version, the caller's context, the connection's end | `TestHeldRefusedWhenInitializeFails` |
| nothing is held before `Initialize` is called | `TestNotHeldBeforeInitialize` |
| only the client holds, and only what it receives | the pins, `TestDispatchFollowsTheModel` |
| no exported change | `make api` |

**Planted breaks,** each in its own scratch copy; 7 of 7 failed as required:

| Break | The check that failed |
| :--- | :--- |
| the gate refusing at once, as before | `TestHeldUntilInitialized`, trial 0: `_x/ask` answered `-32601` |
| a failure not ending the attempt | `TestHeldRefusedWhenInitializeFails`, an error answer: "Initialize did not happen", by its bound |
| an attempt not ended when the caller gives up | the same, the caller's context: "the gate's verdict ... did not happen" |
| messages held with no attempt in flight | `TestNotHeldBeforeInitialize`: "the gate's verdict ... did not happen" |
| the version check left out of the commit | `TestHeldRefusedWhenInitializeFails`, another version: "Initialize = nil, want its failure" |
| the agent's side holding too | `TestDispatchFollowsTheModel`: "agent_gen.go contains \"heldGate\"" |
| the client's outbound methods calling `heldGate` | `TestDispatchFollowsTheModel`: "holds an outbound message" |

**Gates,** each exit 0, run one at a time:

* go1.27.3: regeneration unchanged; `make pre-add-check`, "171 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck)";
  `GOEXPERIMENT=nojsonv2 go test ./...`; `GOOS=windows go vet ./...` in both modules; `make check-records`, `make markdownlint` and the
  identifier scan; `make lint`, "0 issues." four times; `go fix -diff ./...` and `go mod tidy -diff` print nothing, in both modules;
* `make api` against `v1.19.0`: "0 incompatible, 0 compatible";
* `go test -race -count=3 -shuffle=on .`;
* the Windows host: `go test -count=1 -shuffle=on ./...` in both modules; the Linux host: the same, from a copy of the tree;
* the floor gate at go1.26.10: `go vet` and `go test -race -count=1` in both modules, the leak guards on.

### Step 4, the reader's buffer, 2026-10-21

The owner committed step 3 and approved this step. What changed:

* **`connection.go`:** `readBufferSize = 128 << 10`, in the constants beside `defaultMaxMessageSize`, citing the MADR's amendment; `receive`
  reads through `bufio.NewReaderSize(c.r, readBufferSize)`, its one use.
* **Test,** `jsonrpc_test.go`: `TestReceiveReadBufferSize`, with `sizeRecorder`, a reader that reports the length of the buffer each `Read`
  is given; the connection's first `Read` is given 131,072 bytes. It compares with the literal, so that a change of the constant fails it.

**Planted break,** in a scratch copy: the constant at `64 << 10`. `TestReceiveReadBufferSize` failed: "the reader's first Read was given
65536 bytes, want 131072". 1 of 1.

**Gates,** each exit 0, run one at a time:

* go1.27.3: regeneration unchanged; `make pre-add-check`, "171 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck)";
  `GOEXPERIMENT=nojsonv2 go test ./...`; `GOOS=windows go vet ./...` in both modules; `make check-records`, `make markdownlint` and the
  identifier scan; `make lint`, "0 issues." four times; `go fix -diff ./...` and `go mod tidy -diff` print nothing, in both modules;
* `make api` against `v1.19.0`: "0 incompatible, 0 compatible";
* `go test -race -count=3 -shuffle=on .`;
* the Windows host: `go test -count=1 -shuffle=on ./...` in both modules; the Linux host: the same, from a copy of the tree;
* the floor gate at go1.26.10: `go vet` and `go test -race -count=1` in both modules, the leak guards on.

### Step 5, the taps, 2026-10-21

The owner committed step 4 and approved this step. What changed:

* **`agentproc/tap.go`, new:** `tapQueueBytes` (32 MiB); `tap`, its queue, its goroutine `run`, and `offer`, `end`, `abandon` and
  `droppedBytes`; `tapReader` and `tapWriter`, whose `Close` ends the tap and closes the pipe or the shared input; `streams`, which builds
  what the connection reads and writes, and the taps, only when set.
* **`agentproc/agentproc.go`:** `Command.OutputTap` and `Command.InputTap`, documented as the MADR specifies; `Process` holds the two taps
  and `drainOnce`; `Start` builds the connection on `streams`; `Wait` drains the taps once, through `drainTaps`, which ends each tap's
  stream and waits for it up to the grace, from one timer, then abandons it; `DroppedTapBytes`. As built, a tap's stream ends when the
  connection closes the tap's reader or writer, which `watch` and `stop` always do, and in `drainTaps`, rather than by calls in `watch` and
  `stop` as the design above says; the result is the same, every tap's stream ended before `Wait` drains it.
* **Tests:** `tap_test.go`, new, with the child modes `frames` and `bulk` (in `child_test.go`) and the eight tap tests;
  `TestTapKeepsCloseEnding` in `agentproc_unix_test.go`; `ExampleCommand_outputTap` in `example_test.go`.

**As built, the tests differ from the list above in three places:** `TestBlockedTapDetached` takes the tap's drop before `Close`, requiring
about the last 16 MiB of the 48 dropped, between 16 MiB less 256 KiB and 16 MiB more, which shows the tap was detached at the 32 MiB bound
and not only abandoned by `Wait`; the `bulk` agent says it is ready once it has written its first part, and writes its second after a
line of input, so that `TestDetachedTapIsPrefix` releases the tap before the agent's last 8 MiB; and `TestTapDrainBoundedByGrace`'s tap
takes 30 ms a write, through 24 MiB, so that its drain outlasts the grace and the slack.

**Found while building the tests:** two of the first ten planted breaks were not caught as required. The queue without a bound passed
`TestBlockedTapDetached`, which then checked only that something was dropped, as `Wait`'s abandon drops the queue anyway; the test now
checks the drop before `Close`, as above. The synchronous tap failed it only by `go test`'s timeout, since the test's cleanup closed the
agent before it released the tap that held the connection; the blocked tests now release first (`launchTapped`, `blockedTap`). The first
version of the `bulk` agent's stream took 21 to 27 s a test under `-race`; built byte by byte, and once for the comparison, 7 to 8 s.

**The claims, each to what makes it:**

| Claim | Check |
| :--- | :--- |
| the output tap receives every byte, the line too long and the line not JSON included, by `Close`'s return (T1) | `TestOutputTapVerbatim` |
| the input tap receives what the connection writes, a line each | `TestInputTapSeesWrites` |
| a tap set leaves `Close` ending a read a descendant holds (T2, T3) | `TestTapKeepsCloseEnding` |
| a tap's error does not reach the connection (X7) | `TestTapErrorIgnored` |
| unset, nothing is wrapped and no goroutine started | `TestTapsUnset` |
| a blocked tap holds neither the connection nor `Close`, and is detached at 32 MiB (A1f) | `TestBlockedTapDetached` |
| a detached tap's copy is the stream's beginning, nothing after a gap | `TestDetachedTapIsPrefix` |
| `Wait` waits for a slow tap no longer than the grace (A1e) | `TestTapDrainBoundedByGrace` |
| the additions, and nothing else | `make api`: "compatible: agentproc: (*Process).DroppedTapBytes: added", "Command.InputTap: added", "Command.OutputTap: added"; "0 incompatible, 3 compatible" |

**Planted breaks,** each in its own scratch copy; 11 of 11 failed as required:

| Break | The check that failed |
| :--- | :--- |
| the output tap given `p[:n-1]` | `TestOutputTapVerbatim`: "received 2097175 bytes, want the agent's 2097208" |
| the input tap never offered | `TestInputTapSeesWrites`: "the input tap holds \"\", not the connection's two answers, after 30s" |
| `tapReader` without `Close` | `TestTapKeepsCloseEnding`: "Wait took longer than 3.3s" |
| the tap's error returned as the read's | `TestTapErrorIgnored`: `SendNotification`, "peer disconnected: the tap fails" |
| a reader wrapped with no tap set | `TestTapsUnset`: "want the input and the output themselves, and no tap" |
| the copy written on the reader's goroutine | `TestBlockedTapDetached`: "the bulk agent did not say it was ready within 30s" |
| no bound on the queue | `TestBlockedTapDetached`: "dropped 0 bytes in 30s, want about the last 16 MiB" |
| a bound of 24 MiB | `TestBlockedTapDetached`: "dropped 25165824 bytes, want about the last 16 MiB: detached before 32 MiB" |
| a full queue's copy dropped and later ones queued | `TestDetachedTapIsPrefix`: "received 41943040 bytes that are not the stream's beginning" |
| `Wait` not draining | `TestOutputTapVerbatim`: "received 655360 bytes, want the agent's 2097208" |
| the drain without a bound | `TestTapDrainBoundedByGrace`: "Close took longer than 3.3s" |

**Gates,** run one at a time. The first run of `make lint` found gosec's G115 on a conversion of the queue's count and a field gofmt
had not aligned; the counts are `uint64` now, and the second and third runs report "0 issues." four times. Then, each exit 0:

* go1.27.3: regeneration unchanged; `make pre-add-check`, "171 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck)", and
  the new files', "2 file(s) clean"; `GOEXPERIMENT=nojsonv2 go test ./...`; `GOOS=windows go vet ./...` in both modules;
  `make check-records`, `make markdownlint` and the identifier scan; `make lint`; `go fix -diff ./...` and `go mod tidy -diff` print
  nothing, in both modules;
* `make api` against `v1.19.0`: "0 incompatible, 3 compatible", the three additions;
* `go test -race -count=3 -shuffle=on .`; the tap tests, `-race -count=3`;
* the Linux host: `go test -count=1 -shuffle=on ./...` in both modules, from a copy of the tree;
* the floor gate at go1.26.10: `go vet` and `go test -race -count=1` in both modules, the leak guards on;
* the Windows host: `go test -count=1 -shuffle=on ./...` in both modules: the first run failed, below; run again, both modules passed.

**Found, outside this step:** the first Windows run failed in 0027's `TestDescendantsEnded`, the agent exiting of itself: "the agent's
grandchild still runs after the agent exited". Its tap tests passed. The deviation during step 5, below, holds the investigation and the
owner's decision.

### Deviation, 2026-10-21, during step 5: a process the agent starts before its job assignment

**Found.** Step 5's first Windows gate failed in `TestDescendantsEnded` (0027), which this PLAN does not change. The same test failed at
`HEAD` (`b5177dd`, before step 5) in 1 of 15 runs of the whole package on the Windows host, its other two cases, "the agent's grandchild
still runs after Close"; from step 5's tree, 0 of 15; run alone, 0 of 20 from either. The MADR's amendment of 2026-10-21 for this
deviation holds the measurements: the two ways the test could see a dead grandchild as running were not seen in 1,200 trials; under load
the grandchild was created 6.6 to 26.1 ms after the agent's assignment; and with the assignment 100 ms late, a plain start left the
grandchild outside the job in 20 of 20 trials, and a suspended start, resumed after the assignment, in 0 of 20.

**Decision (owner, 2026-10-21).** Fix it inside this PLAN, now: the alternatives were to record it and fix it in a later pair, or to stop
and investigate first. Then, as recommended: a caller's own `CREATE_SUSPENDED` is refused (the MADR's question 12); and the test makes the
assignment late through an unexported delay, zero in production (question 13). Added: step 6, its design ("The suspended start on
Windows"), its tests and its planted breaks above; files `agentproc/tree_windows.go`, a Windows test file, and the docs in step 9. Steps
6 to 10 are renumbered 7 to 11, each annotated.

### Step 6, the suspended start on Windows, 2026-10-21

The owner committed step 5 with the deviation above and approved this step. What changed:

* **`agentproc/tree_windows.go`:** `prepare` refuses a `Cmd` whose `CreationFlags` has `CREATE_SUSPENDED`, and otherwise adds it,
  creating `SysProcAttr` when nil and keeping the caller's other flags; `attach` waits `attachDelay` (zero but in a test), assigns the
  agent to its job, then calls `resume`, which lists the agent's threads in a snapshot, opens each by its ID and resumes it, and fails when
  it finds none. The comments cite the amendment; the one that said a process started before the assignment is outside the job is gone.
* **Tests,** `agentproc/suspended_windows_test.go`, new: `TestDescendantInJobDespiteSlowAttach`, not parallel, with `endBeat`, which ends
  a beat a failing run leaves; `TestStartRefusesSuspended`; `TestCallersCreationFlagsKept`. `startBeat` (`child_test.go`) records its beat's
  process ID beside the beat's file, for `endBeat`.

**As built, the first test differs from the list above:** it uses the `parent` agent and `Close`, not `parent-exit` and `Wait`. Its first
version, with `parent-exit`, failed under the break that leaves out `CREATE_SUSPENDED`, but with "assigning the agent to its job object:
Access is denied": unsuspended, `parent-exit` exits within the 200 ms before the late assignment, which then finds no process. `parent`
stays alive, so that without the fix it is assigned late and its grandchild is not, which is the failure the test means. That run left
the escaped grandchild running on the host; it was found (`tasklist`), ended by its process ID, and its directory removed, and `endBeat`
now ends such a grandchild.

**The claims, each to what makes it:**

| Claim | Check |
| :--- | :--- |
| a process the agent starts at once is in its job however late the assignment (J4) | `TestDescendantInJobDespiteSlowAttach`, the Windows host, 3 runs of 3 |
| a caller's own `CREATE_SUSPENDED` is refused, nothing started (question 12) | `TestStartRefusesSuspended` |
| the caller's other flags are kept, and the agent runs | `TestCallersCreationFlagsKept` |
| `TestDescendantsEnded` passes on Windows | the whole package on the Windows host, three binaries at once, shuffled, 30 rounds: 0 of 90 runs failed |

The 90 runs alone prove little: before the fix the failure was rare, 2 seen in about 250 runs (the deviation). The deterministic test, with
its late assignment, and its two planted breaks are what show the window closed.

**Planted breaks,** each in its own scratch copy, run on the Windows host; 4 of 4 failed as required:

| Break | The check that failed |
| :--- | :--- |
| `CREATE_SUSPENDED` not added, the resume kept | `TestDescendantInJobDespiteSlowAttach`: "the grandchild the agent started at once, before a late assignment, still runs after Close" |
| the resume before the assignment | the same |
| a caller's `CREATE_SUSPENDED` accepted | `TestStartRefusesSuspended`: "want a refusal naming CREATE_SUSPENDED" |
| the caller's flags replaced by `CREATE_SUSPENDED` | `TestCallersCreationFlagsKept`: "CreationFlags = 0x4, want the caller's CREATE_NEW_PROCESS_GROUP and CREATE_SUSPENDED" |

After them, `tasklist` on the host listed no test process, and no scratch directory was left.

**Gates,** run one at a time. The first run of `make lint` found a `nolint` directive gosec does not need in the test file; removed, the
gates ran again, each exit 0:

* go1.27.3: regeneration unchanged; `make pre-add-check`, "173 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck)", and
  the new file's, "1 file(s) clean"; `GOEXPERIMENT=nojsonv2 go test ./...`; `GOOS=windows go vet ./...` in both modules;
  `make check-records`, `make markdownlint` and the identifier scan; `make lint`, "0 issues." four times; `go fix -diff ./...` and
  `go mod tidy -diff` print nothing, in both modules;
* `make api` against `v1.19.0`: "0 incompatible, 3 compatible", step 5's three additions, nothing more;
* `go test -race -count=3 -shuffle=on .`;
* the Windows host: `go test -count=1 -shuffle=on ./...` in both modules; the Linux host: the same, from a copy of the tree;
* the floor gate at go1.26.10: `go vet` and `go test -race -count=1` in both modules, the leak guards on.

### Step 7, Kill, 2026-10-21

The owner committed step 6 and approved this step. What changed, as the PLAN's "Kill" section has it:

* **`agentproc/agentproc.go`:** `Process` gains `hurry`, made in `Start`, and `hurryOnce`. `Kill` closes `hurry` once, runs
  `closeOnce.Do(p.stop)` and returns `Wait`'s error. `exitedWithin` returns false at once when `hurry` is closed; `stop` sends no SIGTERM
  once it is (`hurried`). `Kill`'s doc sits beside `Close`'s, which now says that `Kill` cuts a `Close` in progress short. `ErrKilled`'s
  doc names `Kill`; so does `Wait`'s, which said "when Close ended the agent so", a line the PLAN did not list.
* **Tests,** `agentproc/kill_test.go`, new: `TestKill`, `TestKillCutsCloseShort`, `TestKillAfterExit`. They start the agent through
  step 5's `startTapped`, with no tap, for its grace parameter; no child mode was added.

**The claims, each to what makes it:**

| Claim | Check |
| :--- | :--- |
| `Kill` ends an agent that ignores its input's end at once, at the default grace, with `ErrKilled` and not `ErrTerminated`, and, where there is a tree, the grandchild holding its output (K2) | `TestKill`, within the slack of 3 s, shorter than the grace |
| a `Close` waiting out a grace of 5 s stops waiting and kills; both return the same error, `ErrKilled` joined | `TestKillCutsCloseShort` |
| an agent that has exited is sent nothing; `Kill` returns `Wait`'s error | `TestKillAfterExit` |

Not checked by a test: that a hurried `stop` sends no SIGTERM before the kill. Where there is a process group, the SIGTERM would be
followed at once by the kill, and which of the two ends the agent is a race; no test tells the two apart without one.

The three tests, 20 times each: under `-race` on macOS, and on the Windows and Linux hosts, 60 of 60 passed on each.

**Planted breaks,** each in its own scratch copy, run on macOS; 3 of 3 failed as required:

| Break | The check that failed |
| :--- | :--- |
| `Kill` running `Close`: `hurry` never closed | `TestKill`: "Kill took longer than 3s" |
| `exitedWithin` not watching `hurry` | `TestKillCutsCloseShort`: "Kill took longer than 3s" |
| `Kill` recording `ErrKilled` though nothing was sent | `TestKillAfterExit`: "Kill = agentproc: agent killed, want nil, the agent's exit of status 0" |

**Gates,** each exit 0:

* go1.27.3: regeneration unchanged; `make pre-add-check`, "174 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck)", and
  the new file's, "1 file(s) clean"; `GOEXPERIMENT=nojsonv2 go test ./...`; `GOOS=windows go vet ./...` in both modules;
  `make check-records`, `make markdownlint` and the identifier scan; `make lint`, "0 issues." four times; `go fix -diff ./...` and
  `go mod tidy -diff` print nothing, in both modules;
* `make api` against `v1.19.0`: "0 incompatible, 4 compatible", step 5's three additions and `(*Process).Kill`;
* `go test -race -count=3 -shuffle=on .`;
* the Windows host: `go test -count=1 -shuffle=on ./...` in both modules; the Linux host: the same, from a copy of the tree;
* the floor gate at go1.26.10: `go vet` and `go test -race -count=1` in both modules, the leak guards on.

### Step 8, breakaway, 2026-10-21

The owner committed step 7 and approved this step. What changed, as the PLAN's "Breakaway" section has it:

* **`agentproc/tree_windows.go`:** `attach` sets `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE | JOB_OBJECT_LIMIT_BREAKAWAY_OK`; its comment says
  that a process started with `CREATE_BREAKAWAY_FROM_JOB` is outside the job, citing W3 and W4.
* **`Start`'s doc** (`agentproc/agentproc.go`): on Windows, such a process is outside the job, as one that starts a session of its own is
  outside the process group on Unix; what ends the job or the group does not end it, and it is the agent's to end. The MADR names the
  four ends, `Close`, `Kill`, the agent's exit and this process's death; the doc names what they share, ending the job, since the tests
  below stop the agent by `Close` and by `Kill` alone. The README's paragraph is step 9's.
* **The child mode** `parent-breakaway` (`agentproc/child_windows_test.go`, whose `platformChild` is now a `switch`): it starts a beat
  with `CREATE_BREAKAWAY_FROM_JOB` on the beat file's name with `.away` added, its standard output unset, as a detached updater runs (W4),
  then a beat without it, as `parent` does, and exits at its input's end; a beat it cannot start is written to its standard error.
  `startBeatAt` (`child_test.go`) is `startBeat` for a given file, which `startBeat` now calls with the file the environment names, so
  that its callers are unchanged.
* **The test,** `agentproc/breakaway_windows_test.go`, new: `TestBreakawayLeavesJob`, with two subtests, as the PLAN lists it and more:
  the agent stopped by `Close`, which ends the job once the agent exits at its input's end (`end`), and by `Kill`, which terminates it
  (`kill`). The PLAN named `Close` alone; `Kill` ends the job by the other call. `stderrText` keeps the agent's standard error, so that an
  agent that exits before it is ready says why; the cleanup ends both beats by their recorded process IDs (`endBeat`).

**The claims, each to what makes it:**

| Claim | Check |
| :--- | :--- |
| a process the agent starts with `CREATE_BREAKAWAY_FROM_JOB` starts, and still runs after `Close` and after `Kill` (W3d) | `TestBreakawayLeavesJob`, both subtests |
| a process the agent starts without it ends with the job, the limit set (W3c) | the same |

The test, 5 times on the Windows host: 5 of 5 passed, each subtest. Afterwards the host's process list held no test binary, by its exact
image name; that check was first seen to catch a planted name, `tasklist.exe`, which it lists while it runs.

**Planted breaks,** each in its own scratch copy, run on the Windows host; 2 of 2 failed as required, the second beyond the PLAN's table:

| Break | The check that failed |
| :--- | :--- |
| `BREAKAWAY_OK` left out | `TestBreakawayLeavesJob`, both subtests: "the parent-breakaway agent exited before it was ready: exit status 1", the agent having written "starting the beat that breaks away: fork/exec …: Access is denied.", W3b |
| `SILENT_BREAKAWAY_OK` in its place, which lets every process leave | `TestBreakawayLeavesJob`, both subtests: "the grandchild started without CREATE_BREAKAWAY_FROM_JOB still runs after the agent was stopped" |

After them, the host's process list held no test binary.

**Gates,** each exit 0:

* go1.27.3: regeneration unchanged; `make pre-add-check`, "175 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck)", and the new file's, "1 file(s) clean";
  `GOEXPERIMENT=nojsonv2 go test ./...`; `GOOS=windows go vet ./...` in both modules; `make check-records`, `make markdownlint` and the
  identifier scan; `make lint`, "0 issues." four times; `go fix -diff ./...` and `go mod tidy -diff` print nothing, in both modules;
* `make api` against `v1.19.0`: "0 incompatible, 4 compatible", as after step 7;
* `go test -race -count=3 -shuffle=on .`;
* the Windows host: `go test -count=1 -shuffle=on ./...` in both modules; the Linux host: the same, from a copy of the tree;
* the floor gate at go1.26.10: `go vet` and `go test -race -count=1` in both modules, the leak guards on.

CI's Windows jobs run the test with step 11's push.
