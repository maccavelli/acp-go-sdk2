---
status: accepted
date: 2026-11-16
decision-makers: the repository owner
consulted: "this tree at `32a74d7`; CI run `43875424670` and the race steps of the Linux jobs of the seven runs before it on `main`; `tools/ci_jobs.py tally` and `failure`; Go 1.27.4's `os/exec`; scratch copies of the tree run on this Mac and on the Linux test host"
informed: the coordinator
---

# Give agentproc's tests that need an agent's exit seen in time a grace a loaded runner fits

## Context and Problem Statement

CI run `43875424670`, attempt 1, on `main` at `32a74d7`, failed one job of six: `validate (ubuntu-24.04, 1.27.4)` (job
`133327092180`), its step "race detector", `go test -race -shuffle=on ./...`. The owner ruled on 2026-11-16, choosing "Root-cause it
in its own record". This record holds the cause, what was reproduced and what was not, and the fix, proposed for the owner and chosen by the owner
on 2026-11-16 (Decision Outcome).

### The failure, as the log gives it

Read from `gh run view 43875424670 --log-failed` and the job's full log:

```text
2026-11-16T13:26:53.8591768Z -test.shuffle 1794835577691517934
2026-11-16T13:26:53.8592404Z --- FAIL: TestDescendantsEnded (0.00s)
2026-11-16T13:26:53.8593528Z     --- FAIL: TestDescendantsEnded/the_agent_exits_at_its_input's_end (1.25s)
2026-11-16T13:26:53.8594931Z         ties_test.go:56: Close = agentproc: agent terminated, want nil
2026-11-16T13:26:53.8596478Z 2026/11/16 13:26:47 ERROR failed to queue notification; closing connection err="notification queue overflow" capacity=1 queued=1 bytes=314 size=157 byte_limit=67108864
2026-11-16T13:26:53.8597703Z FAIL
2026-11-16T13:26:53.8598187Z FAIL	github.com/maccavelli/acp-go-sdk2/agentproc	36.180s
```

* **The run.** Five jobs besides it: both Windows jobs and `validate (macos-15, 1.27.4)` "completed success",
  `validate (ubuntu-24.04, 1.26.11)` "completed success", `validate (macos-15, 1.26.11)` "cancelled" with no step run. In the
  failed step every other package passed: the root package in 34.049 s, `remote` in 41.483 s, `clienthost` in 5.451 s. Runner image
  `ubuntu-24.04`, version `20261111.370.1`.
* **The commit.** `git diff v1.36.0 32a74d7 -- agentproc internal/proctree` is seven added lines, all comments, in
  `agentproc/agentproc.go` and `agentproc/inprocess.go`. Nothing the test runs changed.
* **The history.** `python3 -B tools/ci_jobs.py tally` (40 runs of `main`, 2026-10-14 to 2026-11-16, first attempts): 74 Linux jobs,
  3 failed: this one, and `d8badcb`'s two, which
  [0042-PLAN-in-process-agent-runner.md](0042-PLAN-in-process-agent-runner.md) records as `TestInProcessDetachedTapIsPrefix`. Their logs
  are gone (`HTTP 410`), so that attribution is the record's, not read here. This is the first Linux failure of
  `TestDescendantsEnded` in the window.
* **The load.** The race steps of the Linux jobs of this run and the seven before it on `main` (16 jobs, read from their logs): the
  root package took 22.7 to 38.2 s, `remote` 26.8 to 45.3 s, `agentproc` 23.6 to 40.1 s, all passing but this one. The failed
  job's 34.0 s and 41.5 s are at the slow end of that spread and inside it. `agentproc` took 29.8 to 37.1 s at 1.27.4 and 23.6 to
  40.1 s at 1.26.11. CI's runner reports no load figure.

### The overflow line is another test's expected output

`ERROR failed to queue notification; closing connection` is logged at `connection.go:2083`. Its fields, `capacity=1 queued=1
bytes=314 size=157`, are those of `TestConnectionEndStopsAgent/a_notification_overflow` (`agentproc/ties_test.go:194`-`:225`),
which sets `acp.WithMaxQueuedNotifications(1)` and `acp.OverflowCloseConnection` and leaves the connection's default logger on. Run
alone on this Mac (`go test -count=1 -v -run '^TestConnectionEndStopsAgent$/overflow' ./agentproc/`), that subtest passed and logged
the same line, field for field. `go test` prints a package's output only when the package fails, so the line appears beside any
`agentproc` failure. It was logged at 13:26:47, about 30 s into the package's 36 s; it is not part of this failure. On the Linux host it was
logged once per round of the package in chunk B1 below, six in six rounds, all of which passed that subtest.

### The subtest, and what it needs

`agentproc/ties_test.go:52`-`:61`:

```go
t.Run("the agent exits at its input's end", func(t *testing.T) {
	t.Parallel()
	p, beat := startBeating(t, "parent")
	if _, err := within(t, testGrace+slack, "Close", p.Close); err != nil {
		t.Errorf("Close = %v, want nil", err)
	}
```

* `testGrace` is 300 ms (`agentproc/agentproc_test.go:20`), "to keep the suite fast"
  ([0027-PLAN-agent-process-helper.md](0027-PLAN-agent-process-helper.md), "The grace in tests is 300 ms"). `slack` is 3 s
  (`:24`), "how long past its expected time a step may take on a busy runner". `within` (`:104`-`:116`) fails the test only when
  `Close` takes longer than `testGrace+slack`; it did not.
* `startBeating` (`ties_test.go:31`-`:41`) starts the agent through `startCmd` (`agentproc_test.go:83`-`:100`): `Grace: testGrace`,
  and a `Cmd.Stderr` that is a `readyWriter`, not a file, so `os/exec` copies the agent's standard error on a goroutine of its own.
  `Start` sets `Cmd.WaitDelay` to half the grace, 150 ms (`agentproc/agentproc.go:228`-`:230`).
* The `parent` child (`agentproc/child_test.go:144`-`:150`) starts the `beat` grandchild on its own standard output, which it holds
  (`startBeatAt`, `:105`-`:126`: `g.Stdout = os.Stdout`; its standard input and error are `/dev/null`), says it is ready, then
  `io.Copy(io.Discard, os.Stdin)` and exits 0 at its input's end.

### How `Close` came to say "agent terminated"

`Close` runs `stop` once (`agentproc.go:574`-`:577`). `stop` (`:653`-`:669`):

```go
_ = p.agent.closeInput() //nolint:errcheck // a failed close leaves the agent to the next steps.
switch {
case p.exitedWithin(p.grace):
case !p.hurried() && p.signal(p.agent.terminate, ErrTerminated) && p.exitedWithin(p.grace):
default:
```

`exitedWithin` (`:673`-`:684`) waits for `done`, which only the watcher closes (`watch`, `:354`-`:376`), after `p.agent.wait()`,
`Cmd.Wait`, has returned (`:355`) and `p.exited` is set under the mutex (`:365`). `signal` (`:698`-`:709`) sends SIGTERM to the
group unless `p.exited`, and records `ErrTerminated`. Wait's error is then `p.ended` alone when the agent's own error is nil
(`agentError`, `:427`-`:451`). So the message means: **the first step's 300 ms passed before the watcher saw the agent's exit;
SIGTERM was sent to the group; the agent's exit status was 0; its exit was seen within the second step.** It came back through the
second rung of the ladder.

`Cmd.Wait` returns only when both the process has been reaped and the standard-error copy has ended. In Go 1.27.4's `os/exec`
(`exec.go:944`, `:1007`-`:1020`), after `Process.Wait` returns, a timer of `WaitDelay` starts, and if the copying goroutine has not
finished when it fires, `Cmd.Wait` closes the pipe and returns `exec.ErrWaitDelay`. So two windows bound this test: the agent's exit
seen within 300 ms of its input's end, and, inside it, the copy goroutine done within 150 ms of the reap.

### Facts measured

Every measurement ran in a scratch copy of `32a74d7` (`git archive`), never in the tree. "This Mac" is the session machine,
go1.27.4 darwin/arm64; "the Linux host" is the fleet's Linux test host, 4 CPUs, go1.27.4 linux/amd64, reached through
`run-on-host --need linux --race`, which runs one heavy run at a time.

* **F1. What a status of 0 tells.** A probe started the `parent` child in a group of its own and sent SIGTERM to the group: with its
  input open, exit status -15 (killed by the signal), and the same with its input closed just before the signal (this Mac). A
  planted break that makes `stop`'s first step wait 0 s, so SIGTERM follows the input's close at once, reported "agent terminated"
  with status 0 in 15 of 20 runs and "agent terminated: signal: terminated" in 5 (this Mac, no `-race`). So a status of 0 with
  `ErrTerminated` does not prove the agent had exited before the signal; it proves the agent had seen its input's end by the time
  the signal was handled. An agent that never saw its input end would have died of the signal.
* **F2. The two plants tell the sides apart** (each case on this Mac without `-race` and on the Linux host under `-race`, identical
  messages; the times are the Linux host's):

  | Plant, in a scratch copy | The test at `HEAD` |
  | :--- | :--- |
  | the client sees the exit 350 ms late: `childAgent.wait` sleeps after `Cmd.Wait` | **failed**, `Close = agentproc: agent terminated, want nil`, CI's message word for word, 1.21 s |
  | the agent exits 350 ms late: the `parent` child sleeps after its input's end | failed, `Close = agentproc: agent terminated: signal: terminated, want nil`, 1.16 s |
  | no plant | passed, 0.86 s |

* **F3. Where the time goes, measured.** A scratch copy stamped, for the `parent` agent closed as the subtest closes it: the input's
  close (in `stop`), the child's own reading of its input's end (a file it writes after `io.Copy` returns), `Cmd.Wait`'s return,
  and `done`'s close. Per run, in milliseconds:

  | Where, how loaded | Runs | Input close to the child's EOF, max | EOF to `Cmd.Wait`'s return, median / p90 / p99 / max | Input close to `done`, max | Not seen within 300 ms |
  | :--- | ---: | ---: | :--- | ---: | ---: |
  | this Mac, idle, no `-race` | 12 | 0.6 | 0.9 / 1.0 / 3.3 / 3.3 | 3.4 | 0 |
  | the Linux host, beside the whole module's `go test -race -shuffle=on ./...`, two rounds, as CI's step runs | 80 | 8.2 | 3.5 / 10.0 / 29.0 / **574.4** | 574.7 | **1**, with CI's message |
  | this Mac, 32 busy goroutines in the test process, `GOMAXPROCS=2`, no `-race` | 20 | 1.3 | 422.1 / 472.8 / 488.8 / 488.8 | 488.8 | 16 |
  | the Linux host, the same load, `-race` | 20 | 3.1 | 428.2 / 587.5 / 639.1 / 639.1 | 639.3 | 18 |

  In every row the child saw its input's end within 9 ms. The time is spent after that: the agent's own exit and, in the client,
  the goroutines `Cmd.Wait` needs, the one blocked in the wait on `run`'s locked thread and the one copying standard error, getting
  a CPU. `done`'s close after `Cmd.Wait` took at most 4 ms. Under the in-process load only the client is starved, and the failure
  follows from that alone; beside the module both sides share four CPUs, and how the 574 ms divide between the agent's exit and
  the client's wait was **not measured**. Under the in-process load the runs that failed said "agent terminated" (CI's message),
  "agent terminated: exec: WaitDelay expired before I/O complete" (the 150 ms window), and once "agent killed" (not seen within
  two graces).
* **F4. The candidate fix, A/B, under the same in-process load** (32 busy goroutines beside the tests, `GOMAXPROCS=2`, `-race`, 10
  rounds each, the Linux host): the subtest as it is failed 7 of 10, 5 with CI's message, 1 "agent terminated: exec: WaitDelay
  expired before I/O complete", 1 "exec: WaitDelay expired before I/O complete"; under a grace of `slack`, 0 of 10.
* **F5. The planted cases against the candidate fix** (this Mac and the Linux host under `-race`, identical): the client 350 ms
  late, passed; 2.5 s late, within `slack`, passed in 3.36 s; 3.5 s late, past it, failed, "agent terminated"; the product break that
  makes the first step wait 0 s, failed, on the Linux host "Close = agentproc: agent terminated: signal: terminated, want nil", and
  in 20 runs of 20 on this Mac, 15 of them "agent terminated" alone (F1). With no plant the subtest took 0.85 s, as at `HEAD`:
  the grace bounds the step, and a passing run does not wait it.
* **F6. The class.** The same cause is not this subtest's alone. Under the same in-process load, on this Mac without `-race`, one
  round of the whole package at `HEAD` also failed `TestCallersCommandKept` (`ties_test.go:149`, "Close = agentproc: agent
  terminated") and `TestTapDrainBoundedByGrace` (`tap_test.go:425`, the same), and three rounds of `TestDescendantsEnded` failed
  "the agent exits of itself" twice, "Wait = exec: WaitDelay expired before I/O complete, want nil" (`ties_test.go:84`). Without
  added load, on the Linux host, `TestCloseLadder/exits_on_SIGTERM` failed once, "agent terminated: exit status 3 after
  632.288271ms, want ErrTerminated with exit status 3 between one and two graces" (`agentproc_test.go:210`): the `term` agent's exit
  on SIGTERM seen 32 ms past the second grace. The repository has fixed this kind one test at a time three times:
  `TestStderrHeldByDescendant` ([0027-PLAN-agent-process-helper.md](0027-PLAN-agent-process-helper.md), step 9, `descendantGrace`),
  `tappedFrames` ([0030-PLAN-record-initialize-on-arrival.md](0030-PLAN-record-initialize-on-arrival.md), step 12) and
  `TestInProcessServeAgentStep2`
  ([0049-MADR-remediate-the-2026-11-01-debugging-pass.md](0049-MADR-remediate-the-2026-11-01-debugging-pass.md), "The flaky
  `TestInProcessServeAgentStep2`"), each by a grace of `slack` or more for that test.
  The whole package at `HEAD` beside the same load under `-race` on the Linux host (chunk AU, two rounds) failed, in its first round,
  `TestCallersCommandKept` (`ties_test.go:149`) and `TestDefaultPdeathsig` (`pdeathsig_test.go:34`), each "Close = agentproc: agent
  terminated", besides F7's kind; its second round passed.
* **F7. A second, different failure on the Linux host.** `TestDetachedTapIsPrefix` failed "the bulk agent did not say it was ready
  within 30s" (`tap_test.go:335`) and `TestInProcessDetachedTapIsPrefix` "the connection did not read Serve's first 56 MiB within
  30s" (`inprocess_test.go:461`): beside the whole module under `-race`, one or both failed in **5 of 5** module rounds (chunks M
  and B3), and `TestDetachedTapIsPrefix` in 1 of 6 rounds of the package alone (B1). Under the in-process load,
  `TestBlockedTapDetached` failed the same way (`tap_test.go:294`) and `TestTapErrorIgnored` "Wait took longer than 3s"
  (`tap_test.go:256`). Each moves megabytes through the connection under `-race` before its bound, a throughput, not a grace;
  none has failed in CI's window, where the race step passed on the 14 Linux jobs of the seven runs before this one. Its cause was not
  investigated here; it is question 3, answered: a measured report, the PLAN's phase 5. It also means the Linux host cannot, today, pass the module's race run of `agentproc`.
  *Amended 2026-11-16, after phase 1's audit (the amendment of that date, below):* `TestTapErrorIgnored`'s fixed 3 s wait is now
  within this record's decision, by the owner's ruling on the bounds that are not a grace; `TestBlockedTapDetached`'s failure to be
  ready within 30 s stays with phase 5's report, beside the two 56 MiB tests.

### Reproduced, and not

| Chunk | Where | Rounds | `TestDescendantsEnded` failed | Other failures |
| :--- | :--- | ---: | ---: | :--- |
| B1: `go test -race -shuffle=1794835577691517934 -count=6 ./agentproc/` at `HEAD` | the Linux host | 6 | 0 | `TestDetachedTapIsPrefix` (F7), `TestCloseLadder/exits_on_SIGTERM` (F6) |
| M: the measuring copy, `go test -race -shuffle=on ./...` twice, then the in-process load | the Linux host | 2 + 1 | the measured close: 1 of 80, then 18 of 20 | `TestDetachedTapIsPrefix` twice, `TestInProcessDetachedTapIsPrefix` once (F7) |
| PA: the plants (F2, F5) and the A/B (F4) under `-race` | the Linux host | 8 cases + 2 x 10 | as F2, F4, F5 | none |
| B3: `go test -race -shuffle=on ./...` at `HEAD`, three times | the Linux host | 3 | 0 | F7's two tests, in each round |
| AU: the whole package at `HEAD` beside the in-process load, `-race -shuffle=on`, twice | the Linux host | 2 | 0 | round 1: `TestCallersCommandKept`, `TestDefaultPdeathsig` (F6), `TestBlockedTapDetached`, `TestTapErrorIgnored` (F7); round 2: none |

The failed job, re-run once as evidence (`gh run rerun 43875424670 --failed`), passed in attempt 2, with `agentproc` in 30.381 s
and the root package in 22.970 s, against attempt 1's 36.180 s and 34.049 s.

CI's failure itself, `TestDescendantsEnded` failing in an unaltered test, was **not** reproduced on the Linux host: it passed in
all 13 package runs there, each under `-race`: B1 6 and B3 3 of `HEAD`, M 2 in the measuring copy, AU 2 under the in-process
load. Its mechanism was reproduced: deterministically by the client-side plant (F2), with CI's message; by stress in 1 of 80
measured closes beside the whole module under `-race` (F3); and in 7 of 10 rounds under the in-process load (F4).

### The answers

1. **The overflow line** is `TestConnectionEndStopsAgent/a_notification_overflow`'s expected log, printed because the package
   failed; not this failure.
2. **What fails, and through which step:** the agent exits 0 at its input's end, but its exit is seen after the first step's
   300 ms; `Close` sends SIGTERM, sees the exit within the second step, and returns `ErrTerminated` with no exit error.
3. **A budget, not an ordering bug.** The agent saw its input's end within 9 ms in every measured run (F3), and an agent that had not
   seen it would have died of the signal (F1); no handle held the pipe and no wake-up was lost. The time is spent between the
   agent's EOF and `Cmd.Wait`'s return, in goroutines and processes waiting for a CPU, and 300 ms (150 ms for the copy) is less than
   a loaded runner under `-race` has taken (F3, F4).
4. **Not 0030's cause.** [0030-MADR-record-initialize-on-arrival.md](0030-MADR-record-initialize-on-arrival.md)'s J1 to J5 are a
   Windows grandchild started before the job assignment, outside the job, failing "the agent's grandchild still runs after …";
   its fix is `CREATE_SUSPENDED` in `tree_windows.go`, which Linux does not build. This failure is Linux, a different assertion
   (`Close`'s error, not the grandchild), and a different mechanism. It is the cause of
   [0027-PLAN-agent-process-helper.md](0027-PLAN-agent-process-helper.md)'s step 9, in a different test.
5. **What it depends on.** Not the seed: six rounds with CI's seed passed (B1), and the plant fails under any order. Not the Go
   version as a mechanism: the plants and the load fail at 1.27.4, and nothing in the path is version-specific; that `agentproc`'s
   race run takes longer at 1.27.4 than at 1.26.11 in CI (passing runs above: median 32.3 s against 24.5 s, the ranges overlapping)
   was read, not explained, and 1.26.11 was not run here. It depends on load and parallelism: the CPU the client's goroutines and the agent get within 300 ms.

## Decision Drivers

* The cause is a test's assumption, measured (F3): a clean exit is seen within 300 ms of the input's end on a loaded runner. Nothing
  in `agentproc` is wrong (F1 to F3); the product's grace is the caller's, 5 s by default.
* A fix must keep each test able to catch the defect it is for: a product that signals an agent which exits at its input's end must
  still fail the test (F5's break).
* A figure for a bound must follow from what the time is spent on, not be raised until a run passes.
* The suite should not grow slower in passing runs.
* The kind has recurred four times; a fix one test at a time leaves the rest of the class (F6) to fail later.
* *Added 2026-11-16, after phase 1's audit:* the driver on a bound's figure, that it must follow from what the time is spent on and
  not be raised until a run passes, now binds the bounds that are not a grace too (the amendment of that date, below). A bound that
  cannot be given such a figure, keeping its test able to fail on the defect it is for, goes to the owner; it does not get a larger
  number.

## Considered Options

* O1. Change `Close` so that it sees an exit that happened in time.
* O2. Give each test whose assertion needs an agent's exit seen within a grace a grace of `slack`; the tests whose outcome needs no
  exit seen within a grace keep `testGrace`.
* O3. Raise `testGrace` for the whole package.
* O4. Give the tests' agents a `Cmd.Stderr` that is a file, so `os/exec` has no copy goroutine.
* O5. Leave it, and record it as a known intermittent.

## Decision Outcome

Chosen option: "O2", by the owner's ruling, relayed by the coordinator, on 2026-11-16: "Longer exit grace for the tests that need
it". The option's text, as the owner saw it, named the figure: the suite's existing 3 s busy-runner bound, `slack`, about five
times the worst measured, the assertions unchanged. The options shown and not chosen were O1, changing `Close` in the product, and
O3, raising the grace for the whole test package. (Annotated 2026-11-16: the draft of this record recommended "O2", for the reasons
that follow, and said "Recommended to the owner, not decided"; the owner chose it.)

It was chosen because it removes the assumption that failed, a clean exit seen within 300 ms on a
loaded runner, with the bound the suite already defines for that, `slack`, "how long past its expected time a step may take on a
busy runner"; it keeps every assertion and the product as they are, the break of F5 still failing the amended test; and it costs
nothing in a passing run, since `Close` returns when the exit is seen (F5: 0.85 s with or without it).

The figure follows from the measurement, not from passing: the worst exit seen beside the whole module was 575 ms, and under the
deliberately starved process 639 ms (F3); `slack`, 3 s, is about five times the worst, and the margin is the one claimed: a plant
2.5 s late passes, 3.5 s late fails (F5). 0049's probe found 1.2 s enough in 100 runs under its load, which is less margin than
the suite's own bound and was not chosen there either.

*Added 2026-11-16, after phase 1's audit.* The decision above, decided first, stands. The owner then widened it, by two rulings,
each relayed by the coordinator (the amendment of that date, below): the rule reads "the exit, the drain or the return seen within
the first grace-bound wait", which moves four tests the audit left undecided, each proved first; and the decision also covers the
fixed waits and drain bounds that are not a grace and that the starved load exposed in four tests, each rescaled from its own
measurement. The two 56 MiB tests stay with phase 5's report.

### Consequences

* Good, because the cause is removed for the class, not for one test (F6), which a fifth one-test fix would leave behind.
* Good, because each amended test keeps its assertion, and the planted product break still fails it (F5).
* Good, because a passing run takes no longer; a test whose exit is seen late now passes up to `slack` later instead of failing.
* Neutral, because the product's behaviour and its default grace do not change; its docs say what the code does, by question 4's answer.
* Bad, because a test that waits a grace out and then needs an exit seen within the next, as `TestCloseLadder/exits_on_SIGTERM`
  does, then takes about `slack`, 3 s, a run instead of 300 ms; it runs in parallel with the package's others.
* Bad, because the tests carry two graces, and each new test must choose: the PLAN's rule, written into the constant's comment, is
  what keeps that choice reviewable.
* Bad, because a product defect that makes `Close` slow by less than 3 s, but more than 300 ms, would no longer fail these tests;
  the ladder tests that keep `testGrace`, and `TestCloseLadder`'s own bounds on the steps, still measure the steps.
* *Added 2026-11-16, after phase 1's audit:* Bad, because the record grows past the CI failure it was opened for, and its fix waits
  for the measurement of four more bounds (the owner's ruling, as put to the owner, said so).
* *Added 2026-11-16:* Good, because one PLAN covers every bound the starved load exposed in `agentproc`'s tests, the two 56 MiB tests
  and `TestBlockedTapDetached`'s readiness aside, which phase 5's report measures.
* *Added 2026-11-16:* Bad, because a bound that is not a grace may not admit a figure that both follows from measurement and keeps
  its test able to fail on its defect; such a bound stops the PLAN for the owner.

### Confirmation

* **The plant:** in a scratch copy, the client sees the exit 350 ms late; the subtest as it is fails with CI's message, and as
  amended passes; 2.5 s late passes, 3.5 s late fails; the break "the first step waits 0 s" fails the amended test. Done once for
  this record (F2, F5); the PLAN runs it again on the amended tree, and the same plant for each test it moves.
* **The load:** the A/B of F4 for each test the PLAN moves: failing at `HEAD` under the in-process load, passing amended.
* **Repeats:** the amended package, 20 rounds under `-race -shuffle=on` on the Linux host, and the whole module beside it as CI's
  step runs it, through `run-on-host`.
* **CI:** the six jobs green on the commit that carries the fix.
* *Added 2026-11-16, after phase 1's audit:*
  * **Each moved test seen to fail first:** at `HEAD`, under a plant that fits the wait it depends on (a late exit for the first
    step; a standard-error copy that outlives `WaitDelay` for a test whose agent exits of itself; a late `Serve` return or drain for
    the borderline four), and passing amended. A test that no plant can fail at `HEAD` does not move and is reported.
  * **Each rescaled bound:** its figure a multiple of the worst measured under both loads, with the planted margin shown, just inside
    passing and just outside failing; and the planted product break for its defect still failing it after the move.
  * **The load:** the A/B under a load that covers every moved test for its whole run, 0 of 10 amended, for every test the PLAN
    moves, the borderline four and the four rescaled bounds included.

## Pros and Cons of the Options

### O1. Change `Close` so that it sees an exit that happened in time

For example, before SIGTERM, ask the kernel whether the agent has already exited and not been reaped (`waitid` with `WNOWAIT`, per
platform), and treat it as exited.

* Good, because `ErrTerminated` would not be reported for an agent that was already a zombie when the step ended.
* Bad, because it narrows the window and does not close it: an agent that has seen its input's end but not finished exiting, or
  whose standard-error copy is still running, is still signalled (F1, F3).
* Bad, because the measured time includes the client's own goroutines waiting for a CPU (F3, in-process load), which no change to
  the ladder removes; under a caller's grace, 5 s by default, the window does not arise in practice.
* Bad, because it adds platform code to `internal/proctree` for a test's margin, which
  [0027-PLAN-agent-process-helper.md](0027-PLAN-agent-process-helper.md)'s step 9 refused for the same reason (its option 4).

### O2. A grace of `slack` for each test that needs an exit seen within a grace

* Good, because the bound becomes the one the suite defines for a step on a busy runner, about five times the worst measured.
* Good, because the assertions stay, and the planted break still fails (F5).
* Good, because a passing run is not slower (F5).
* Neutral, because it is what 0027's step 9, 0030's step 12 and 0049's phase 3 did for one test each; this does it for the class.
* Bad, because the tests that wait a grace out and then need an exit seen take about 3 s a run (Consequences).

### O3. Raise `testGrace` for the whole package

* Good, because every grace-bound test gains margin at once, with one change.
* Bad, because every test that waits a grace out, the `term`, `stubborn` and `parent-wait` agents', slows by as much: with 3 s, by
  about 3 s or 6 s each, for a margin only some need. 0027's step 9 weighed the same and refused it (its option 2).
* Bad, because a smaller figure, such as 1 s, is not one the measurements give: 1 s is under twice the worst seen (F3).

### O4. A file for the tests' `Cmd.Stderr`

* Good, because `os/exec` then has no copy goroutine, and the 150 ms window of `WaitDelay` is gone from these tests.
* Bad, because the 300 ms window stays: the reap and the waiting goroutine still need a CPU (F3), and CI's message is the first
  step's, not `WaitDelay`'s.
* Bad, because the tests would stop running a non-file writer, which a caller's `Stderr` often is, and which `readyWriter` stands
  for (`agentproc_test.go:59`-`:61`).

### O5. Leave it, and record it as a known intermittent

* Good, because nothing changes.
* Bad, because it fails CI again on a loaded runner, at a rate measured here only on one host (F3: 1 in 80 closes beside the module).
* Bad, because a red CI that is "known" trains a reader to rerun, which hides a failure that is not this one.
* Bad, because it is a workaround by the repository's own rule: a test known to fail for its own margin is not a check.

## More Information

* **Relation to the earlier records.** This record is **independent** of
  [0030-MADR-record-initialize-on-arrival.md](0030-MADR-record-initialize-on-arrival.md): J1's cause is Windows' job assignment, fixed
  and not come back (answer 4). It **extends**
  [0027-MADR-agent-process-helper.md](0027-MADR-agent-process-helper.md)'s tests without amending the MADR: 0027 decides the ladder,
  the grace and `WaitDelay` (P8, P9, P14, "Close, in order"), which stay; it names no test grace, and its PLAN's "The grace in tests
  is 300 ms" is what this changes. The PLAN annotates that line in place.
* **What a status of 0 with `ErrTerminated` means.** `ErrTerminated`'s comment says it is joined "when the agent exited after Close
  sent SIGTERM" (`agentproc.go:29`-`:32`). In CI, and under the client-side plant, the agent had seen its input's end and exited, or
  was exiting, of itself by the time the signal was handled, and its exit was seen only after the signal was sent (F1, F2). The README's wording, "when `Close` had to escalate", is exact; the
  comment's is not. Question 4 is answered: the comment and the README say what the code does (the PLAN's phase 3).
* **Not measured:** how the 574 ms divide between the agent's exit and the client's wait beside the module; CI's runner's load at
  13:26; Windows and macOS under the same load; Go 1.26.11 here.
* **Implementation:** [0057-PLAN-agentproc-test-graces-on-a-loaded-runner.md](0057-PLAN-agentproc-test-graces-on-a-loaded-runner.md).

### Questions for the owner

Each had three options; the first was recommended. They are repeated, with their costs, in the PLAN. All four are answered, on
2026-11-16, each answer recorded under its question.

1. **Scope.** (a) every `agentproc` test the PLAN's audit finds bound by an exit seen within `testGrace` or `WaitDelay`
   (recommended: the class, F6); (b) the failing subtest alone, as before; (c) (a), and the root package's and `remote`'s
   grace-bound tests too, in a record of their own.
   * **Answered (a),** the owner's ruling, relayed by the coordinator: "Every `agentproc` test in the class". Not chosen: only the
     failing subtest; the class plus an audit of the other packages.
2. **The figure.** (a) `slack`, 3 s (recommended: the suite's bound, five times the worst seen); (b) 1.2 s, 0049's; (c) `2 * slack`,
   as `descendantGrace`.
   * **Answered (a), by the text of the chosen option,** not by a ruling on this question: the coordinator did not put question 2 to
     the owner on its own. The decision the owner chose, "Longer exit grace for the tests that need it", named the figure in its
     text: the suite's existing 3 s busy-runner bound, `slack`.
3. **F7, the 56 MiB tests' 30 s on the Linux host.** (a) a REPORT of its own under this number, measured, and a record if it is a
   defect (recommended: a different mechanism, not seen in CI); (b) fold it into this PLAN; (c) note it here and do nothing.
   * **Answered (a),** the owner's ruling, relayed by the coordinator: "A measured report, then decide". Not chosen: fold the fix
     into 0057; note it and do nothing. The option's text, as shown, added that until then this repository's race gates keep running
     on this Mac, where they pass. The PLAN's gates are amended to match (its amendment of 2026-11-16).
4. **`ErrTerminated`'s comment.** (a) say that an agent exiting at its input's end as the grace ends can be reported terminated with
   a status of 0, in the comment and the README (recommended: the docs say what the code does); (b) leave it; (c) O1, in a record of
   its own.
   * **Answered (a),** the owner's ruling, relayed by the coordinator: "Say what the code does". Not chosen: leave the comment;
     change the behaviour in a separate record.

## Amendment 2026-11-16, after phase 1's audit: four borderline tests, and the bounds that are not a grace

Found by the PLAN's phase 1, its entry "2026-11-16: phase 1 (the audit)" in
[0057-PLAN-agentproc-test-graces-on-a-loaded-runner.md](0057-PLAN-agentproc-test-graces-on-a-loaded-runner.md), committed as
`f0b7c2e`, and ruled on by the owner the same day. The decision above, as first decided, stands; this amendment adds to it. Line
numbers are those of `f0b7c2e`, whose tests are those of `32a74d7`.

### What the audit found

Facts from that entry, its five rounds of the whole package under the in-process load on the Linux host:

* **G1. Four tests the rule left undecided.** Three run under `testGrace` and the audit classed them "unclear":
  `TestWaitErrorSaysTheCauseOnce/Kill,_Serve_returning_its_cause` (`inprocess_test.go:227`, Kill's abandonment wait),
  `TestCloseContextBoundsHandlers/in_process,_KillContext` (`closecontext_test.go:142`, its assertion `:149`) and
  `TestCloseContextCause` (`closecontext_test.go:242`, its assertion `:249`), the last two through a context bound of `testGrace`
  that decides whether `acp.ErrHandlersRunning` is joined (`agentproc.go:644`). The fourth, `TestCloseContextNotHandlers`
  (`closecontext_test.go:259`), is class A at a grace of 2 s, below `slack`.
* **G2. Four tests failed on bounds that a longer exit grace does not move.** `TestNoLostOutput`, "Wait took longer than 3s"
  (`agentproc_test.go:174`, round 5), and `TestTapErrorIgnored`, the same (`tap_test.go:256`, round 5): each a fixed
  `within(t, slack, "Wait")`. `TestOutputTapVerbatim` (`tap_test.go:219`, `:222`, round 1): the tap held 2031616 of the agent's
  2097208 bytes when the drain's 3 s grace ended. `TestCloseReadsTheAgentsLastOutput` (`tap_test.go:398`, round 2): "agent
  terminated: signal: terminated" at a 5 s grace.
* **G3.** Twelve class A tests have no first-step wait in their outcome, only `WaitDelay` (the audit's finding 4), so a late exit
  cannot fail them.
* **G4.** Bounds stated in graces: `closecontext_test.go:145` (`const bound = testGrace`), `:180` and `:246` (context timeouts of
  `testGrace`), and `:214` (`const grace, bound = 5 * time.Second, testGrace`) (finding 5).
* **G5.** The load, a parallel test of 60 s, covered part of each round, so "not seen to fail" is weak (finding 6).
* **G6.** Nine class A and B tests, and four unclear ones, already run under `slack` or more (finding 7).
* **G7.** `TestCloseContextBoundsHandlers/a_child,_CloseContext` failed in round 3 on its grace and also "Wait took longer than 3s"
  (`closecontext_test.go:156`), a fixed `within(t, slack, "Wait")` not among G2's four.

### The owner's rulings

* **The four borderline tests (G1):** "Move all four, each proved first", the owner's ruling, relayed by the coordinator. As put to
  the owner: the rule is read as "the exit, the drain or the return seen within the first grace-bound wait", and the four move to
  3 s; each must first be seen to fail at today's code under a planted delay; one that cannot be made to fail does not move and is
  reported. Options not chosen: leave all four; move only the three at 300 ms.
* **The bounds that are not a grace (G2):** "Widen 0057 to these bounds now", the owner's ruling, relayed by the coordinator; not the
  option the coordinator recommended. As put to the owner: 0057 also rescales the fixed 3 s waits and the drain bounds, each from its
  own measurement; one plan covers every bound the load exposed; it grows past the CI failure it was opened for and delays that fix.
  Options not chosen: keep 0057 to graces and let phase 5's report measure the rest; stop using the starved load as proof.
* **Unchanged:** "A measured report, then decide", on F7. Of the tests F7 named "under the in-process load", `TestTapErrorIgnored` is
  one of G2's four and now within this decision. `TestBlockedTapDetached` stays with phase 5's report: each of its failures, in the
  root-cause chunk AU and in the audit's rounds 1 and 2, was "the bulk agent did not say it was ready within 30s" (`tap_test.go:294`),
  the bulk agent's megabytes before it is ready, the mechanism of `TestDetachedTapIsPrefix`'s F7 failure; its `Close` under
  `testGrace` (class A) moves with the rest of the class. The two 56 MiB tests stay with the report, `TestDetachedTapIsPrefix`'s
  audit failure at its 5 s grace (`tap_test.go:349`, round 1) included.

### The coordinator's applications of standing rules

Recorded as the coordinator's applications, not the owner's rulings:

* **A check is unused until seen to fail (G3).** Each moved test must be seen to fail at `HEAD` under a plant that fits the wait it
  depends on; for the twelve, one that outlives `WaitDelay`. The audit's claim is from reading, so phase 2 verifies it by running. A
  test that no plant can fail at `HEAD` does not move and is reported.
* **The rule never lowers a grace (G6).** A test already under `slack` or more keeps its grace.
* **Bounds stated in graces scale with the test's grace (G4).** `closecontext_test.go:145`, `:180` and `:246` move with their tests'
  grace; `:214` belongs to `TestCloseContextHurriesTheSteps`, class C, which does not move, so it stays.
* **The load must cover every moved test (G5).** Phase 2's load runs for the whole of each test binary's run, not as a test that
  takes a parallel slot for part of it.

### The decision, as widened

* **W1.** Class A reads "the exit, the drain or the return seen within the first grace-bound wait". The four tests of G1 move to
  `exitGrace`, each first seen to fail at `HEAD` under a planted delay; one that cannot be made to fail stays and is reported.
* **W2.** The four bounds of G2 are rescaled, each from its own measurement, and each keeps the defect it is for:

  | Test | The bound | What it waits for | The defect it is for | The planted break that must still fail it |
  | :--- | :--- | :--- | :--- | :--- |
  | `TestNoLostOutput` | `within(t, slack, "Wait")`, `agentproc_test.go:174` | the burst agent's 1000 updates read, decoded and handled; its exit; `Wait`'s return | lost output, the count at `:178` (0027's P1, P2) | the connection closed at the agent's exit, before it has read to the end |
  | `TestTapErrorIgnored` | `within(t, slack, "Wait")`, `tap_test.go:256` | the same, with both taps failing every `Write` | a tap error that stops the connection or holds `Wait`, or loses updates (`:259`) | a tap's `Write` error returned to the connection's reader |
  | `TestOutputTapVerbatim` | the drain within `tappedFrames`' grace, `slack` (`tap_test.go:195`; the tap's 1 ms, `:194`; the checks `:219`, `:222`) | the output tap's 2 MiB in 64 KiB writes, 1 ms a write plus the wake-ups | a tap that does not receive every byte, in order | a tap that skips one queued write |
  | `TestCloseReadsTheAgentsLastOutput` | `Close`'s first step at its 5 s grace, `tap_test.go:377`, `:397`-`:398` | the agent's 2 MiB read through a reader held back 5 ms a read, then its exit | a `Close` that loses the agent's last output (0041-PLAN, phases 2 and 3) | `stop` closing the connection at once, which the comment at `agentproc.go:664`-`:668` forbids |

* **W3. A measurement before any figure.** For each bound of W2: what it waits for, split into its parts where they can be stamped;
  the time each takes beside the whole module under `-race`, and under the in-process load covering the whole run, on the Linux
  host; the figure that follows, by this record's standard, a multiple of the worst measured, about five times as the first
  decision's figure is; and the planted margin, a delay putting the waited thing just inside the new bound passing, just outside
  failing. The audit's finding 3 shows that the multiple the first decision gave a grace does not hold for a tap drain under the
  starved load (about 100 ms a 64 KiB write, read from the figures, not measured), so no figure is assumed before it is measured.
* **W4. The stop.** A bound whose figure cannot both follow from its measurement and keep its test able to fail on its defect, under
  W2's break, goes to the owner with the measurement. It does not get a larger number.
* **W5. Not in this decision:** the two 56 MiB tests and `TestBlockedTapDetached`'s readiness, which phase 5's report measures; and
  G7's fixed wait, which question 5 asks about.

### Question, after the audit

5. **G7, `closecontext_test.go:156`.** Its fixed 3 s `Wait` failed once, in the same run as its subtest's grace failure, which W1's
   bound scaling moves; whether it fails on its own is not known.
   * (a) Recommended: measure it with W2's four in the PLAN's phase 1b, and rescale it only if it fails once its subtest's grace and
     bound have moved. Cost: one more measurement. The ruling's text, one plan for every bound the load exposed, reaches it; the
     evidence does not yet show it exposed by itself.
   * (b) Rescale it now as a fifth bound of W2. Cost: a figure for a bound that may not need one.
   * (c) Leave it to phase 5's report. Cost: a bound the load exposed left outside the plan the ruling made for them.
   * **Answered (a), 2026-11-16,** the owner's ruling, relayed by the coordinator: "Measure it; rescale only if it still fails".
     As put to the owner: phase 1b stamps it with the other four; it is rescaled only if it still fails once its subtest's grace
     and grace-stated bound have moved; if it stops failing, it stays at 3 s and the record says why. Not chosen: rescale it now
     with the other four; leave it to the report. W5's exclusion of G7's fixed wait is lifted to that extent: it is measured in
     phase 1b, and rescaled only on that condition.
