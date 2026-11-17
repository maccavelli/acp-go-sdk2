# The 56 MiB tests' 30 s, measured

Observed 2026-11-16 and 2026-11-17 (UTC) on `b050c9f`, for phase 5 of
[0057-PLAN-agentproc-test-graces-on-a-loaded-runner.md](../decisions/0057-PLAN-agentproc-test-graces-on-a-loaded-runner.md).
It answers the owner's ruling on that plan's question 3, "A measured report, then decide" (the owner's ruling, relayed by the
coordinator), for the two 56 MiB tests, `TestDetachedTapIsPrefix` and `TestInProcessDetachedTapIsPrefix`, and for the two
readiness waits the owner's ruling "Count it apart with the other readiness failure" added (the owner's ruling, relayed by the
coordinator): the time to be ready of `TestBlockedTapDetached` and of `TestTapDrainBoundedByGrace`. It also measures the failure
of `TestDetachedTapIsPrefix` at its 5 s grace that the plan's phase 1 audit saw in round 1. Two conditions are the
coordinator's addition, as the coordinator's application of "A measured report, then decide" and not as the owner's ruling: the
Windows laptop, the test alone and the whole package, where phase 4 saw the three tests fail in 3 of 3 rounds, and this Mac, the
reference for the race gate of record.

This report records what was measured. It decides nothing, changes no test, bound, grace, data size, workflow or tool, and does
not complete the plan: the plan completes when the owner has decided what follows. "What could follow" lists the options with
their costs, and marks one as a recommendation.

## Summary

- **What the wait is.** Under `-race` each of the four waits is the connection's own work on the child's output, and nothing
  else: the reader checks each 1 MiB line with `json.Valid` and decodes its envelope with `json.Unmarshal`, which together run at
  about 6 MiB/s on the Linux host, 11 MiB/s on this Mac and 17 MiB/s on the Windows laptop when the test runs alone. 56 MiB then
  takes 10.6, 5.7 and 4.5 s (median, `TestDetachedTapIsPrefix`), against 0.6, 0.2 and 0.2 s without `-race`. The tap, the
  child's building of its stream, the pipe, the size of the child's writes and `GOMAXPROCS` are not in it: removing the tap, writing in
  64 KiB or 1 MiB pieces, and `GOMAXPROCS=1` each left the Linux host's median within 0.2 s of 10.6 s.
- **What moves it is how many run at once.** The same 56 MiB takes (median, `TestDetachedTapIsPrefix`) 10.6 s alone,
  25.3 s with the four bulk tests selected together and 25.8 s beside the module as CI's step runs it on the Linux host
  (2 of 10 rounds over 30 s); 4.5 s alone, 13.1 s with two together, 28.4 s with four and 41.3 s in the whole
  package on the Windows laptop (10 of 10 rounds over 30 s, `TestInProcessDetachedTapIsPrefix` and `TestBlockedTapDetached` the
  same); 5.7, 6.5, 8.7 and 11.7 s on this Mac. The Windows laptop, with 32 hardware threads, gets slower with each test added
  by more than the Linux host's 4 CPUs do; the cause is not isolated here.
- **Under the in-process starved load** (32 busy goroutines, `GOMAXPROCS=2`) the wait is 90 to 116 s, about 0.5 MiB/s, in
  5 of 5 rounds over 30 s, and `Close`'s first step of the two 56 MiB tests takes 12 to 18 s, over its 5 s.
- **CI:** the race step passed on all 52 Linux and Windows jobs of the last 13 CI runs on `main`, so no test waited past 30 s
  there; `agentproc`'s whole time in that step was 22.9 to 50.7 s (median 31.9 on Linux and 34.3 on Windows). CI's logs print
  neither a CPU count nor a test's own time.
- **The 5 s grace:** reproduced. Beside the starved load, with the readiness bound lifted and the 5 s graces as written (c2, 2
  rounds), `Close` failed in 2 of 2 runs of `TestDetachedTapIsPrefix`, each with the audit's own message, "agentproc: agent
  terminated: signal: terminated", and in 2 of 2 runs of `TestInProcessDetachedTapIsPrefix`, with "agentproc: agent killed: io:
  read/write on closed pipe": the first step ended at its 5 s with no exit, and the agent ended on SIGTERM (`Close` returned after
  5.8 s) or after the kill (11 s). With the graces lifted (c) the same step waited 12 to 18 s for the exit, over 5 s in every run.
- **No defect in the tap path was found.** In `TestBlockedTapDetached`, where every byte is read before "ready", `json.Valid` and
  the envelope's `Unmarshal` took 78 to 99 % of the wait in every condition measured and the tap's `offer` 0.2 to 1.6 %; the time
  is the race detector's cost on `encoding/json`'s scans of each line, which the connection runs three times (`json.Valid`,
  `Unmarshal`'s own validity pass, its decode), and it is the same for the in-process and the child variant.
- **One option was measured:** the four bulk tests taking one lock from launch to ready took 6.8 to 14.0 s (medians, 18.5 s at
  most) in the Windows package, against 19.2 to 41.3 s as written, and 6.3 to 17.9 s medians beside the module on the Linux host
  against 8.8 to 25.8 s.

## What is measured

Four tests, at `b050c9f`:

| Test | The wait | Data | Bounds | Built for |
| :--- | :--- | ---: | :--- | :--- |
| `TestDetachedTapIsPrefix` | the bulk child's "ready" on its standard error, said once its first 56 MiB are written to the pipe and read (a pipe holds 64 KiB, so a write returns when the reader has read all but that); then, after `Close`, 8 MiB more | 56 + 8 MiB | `awaitReady`, `readyWait`, 30 s (`tap_test.go:358`); `Close`'s first step, `Grace`, 5 s (`tap_test.go:356`, `:371`) | Unix and Windows |
| `TestInProcessDetachedTapIsPrefix` | `Serve`'s `Write` of 56 MiB to an `io.Pipe` returning, which it does when the connection has read it; then 8 MiB more | 56 + 8 MiB | `time.After(readyWait)`, 30 s (`inprocess_test.go:468`); `InProcess.Grace`, 5 s | all |
| `TestBlockedTapDetached` | the bulk child's "ready" after 48 MiB | 48 MiB | `awaitReady`, 30 s (`tap_test.go:317`); its `Close` is under `exitGrace` | Unix and Windows |
| `TestTapDrainBoundedByGrace` | the bulk child's "ready" after 24 MiB | 24 MiB | `awaitReady`, 30 s (`tap_test.go:441`); its `Close` is under `exitGrace` | Unix and Windows |

In each, the tap's first `Write` blocks until the test releases it, so what the wait depends on is the connection's reader:
`tapReader.Read` takes at most 64 KiB a read from the pipe, offers a copy of it to the tap's queue (`tap.offer`, a
`bytes.Clone`, the tap detached once 32 MiB are queued), and `Connection.receive` assembles lines of 1 MiB, runs `json.Valid` on
each, decodes it into an envelope with `json.Unmarshal` (which scans it again) and queues it as a notification of an extension
method that no handler takes. The tap's own `Write` is not in the wait.

The line numbers in the plan's records are the audit's (`51c5fd4`) and `f102c5a`'s; those above are `b050c9f`'s. The audit's
"`tap_test.go:349`" is `Close`'s failure in `TestDetachedTapIsPrefix`, now `tap_test.go:372`.

## How it was measured

Everything ran on scratch copies of `git archive HEAD`, never the tree. A script that asserts each of its edits lands exactly once
patched a copy; it added, and changed nothing else:

- **Bounds lifted by the environment,** so that the time is measured and not cut: `readyWait` (30 s) to 600 s and the two 5 s
  graces of the 56 MiB tests to 120 s, each a no-op when unset. One more setting lifts the readiness bound alone, so that the
  5 s graces run as written (conditions c2 and d2).
- **Stamps,** one JSON line a run, one clock a host: the test's start, the launch, the start, "ready", the release, the input
  closed, the exit seen, `Close`'s return; the child's own (process start, `bulk` entered, its stream built, its first write
  returned, "ready", the line received, its second write returned); the connection's reader, at every 4 MiB (the time blocked in
  the pipe's `Read` and in the tap's `offer`, cumulative); the connection's own counters (time in `readLine`, in `json.Valid`,
  in the envelope `Unmarshal`, in queueing and in handling a notification: a scratch exported method of `Connection`); the
  tap's `Write`s and its queue's high-water mark; the collector's count and pause time at the start and at "ready".
- **The load of the MADR's F3,** where named: `GOMAXPROCS=2` and 32 busy goroutines started from the test binary's `init`
  (never in a child), on from the binary's start to its end.
- **A run** is one process of the test binary, built once a chunk by `go test -c` (`-race` unless a row says otherwise). A chunk
  is one `run-on-host --need <os> [--race]` call, or `run-heavy` on this Mac, one at a time on each host; the host keeps its own
  output and the stamps' file, and the chunk's output is the record. The shuffle seed of every package or module round is in the
  chunk's output.

| Letter | Condition |
| :--- | :--- |
| a | Linux host, each test alone, 10 runs of its own process; the host idle (load average 0.2 to 1.1) |
| b | Linux host, beside the whole root module as CI's step runs it, `go test -race -shuffle=SEED ./...`, 10 rounds |
| c | Linux host, the whole `agentproc` package beside the starved load, 5 (a chunk of 3 and a chunk of 2) rounds |
| c2 | as c, the two 5 s graces as written, 2 rounds |
| d | Windows laptop, each test alone, 10 runs; and the whole package, idle, 10 rounds |
| d2 | Windows laptop, the whole package with the 5 s graces as written, 5 rounds |
| e | this Mac, each test alone, 10 runs (its load average was not low: "What distorted a measurement") |

The rounds of c are fewer than 10: a round of the whole package under the starved load took 8 to 10 minutes with the bounds
lifted, the Linux host's queue is shared with other sessions' work, and after the first round took 9.6 minutes five rounds were
run (about 50 minutes), and the rounds with the 5 s graces as written were cut to 2. The module of b is the root module, which
CI's
step runs before `cmd/generate`, not beside it.

Cheap experiments, run beside these where one could settle a cause: the same 56 MiB with the output tap removed; the child
writing in 64 KiB and in 1 MiB pieces; `GOMAXPROCS` 1 and 2 (Linux) and 4 and 8 (Windows), alone, and 4, 8, 16 and 32 for the
Windows package; the connection fed the same 56 MiB from memory (no child, no pipe, no tap); `json.Valid` and the envelope
`Unmarshal` on the stream's lines alone; an `os.Pipe` moving 56 MiB, written whole and in 4 KiB, 64 KiB and 1 MiB pieces; the four
bulk tests selected together, and two of them together, in one process as the package runs them; and the four bulk tests taking
one scratch lock from launch to ready, so that one runs at a time.

## The machines

What is known without identifying them, from the stamps and the hosts' own reports:

| | OS and architecture | CPUs as Go sees them | Go | Other |
| :--- | :--- | ---: | :--- | :--- |
| Linux host | linux/amd64 | 4 | go1.27.4 | idle during these runs, load average 0.2 to 1.1 before and after each chunk; others' heavy runs queued before mine |
| Windows laptop | windows/amd64 | 32 | go1.27.4 | a laptop-class processor with 24 cores and 32 hardware threads, performance and efficiency cores both; the "Turbo" power scheme; load 1 to 4 % at each dispatch |
| this Mac | darwin/arm64 | 10 | go1.27.4 | other sessions' work ran on it: load average 19.5 falling to 3.6 over the first chunk, 7 to 14 over the second, 4 to 6 later |
| CI, Linux | `ubuntu-24.04`, image 20261101.360.1, 20261111.127.1 or 20261111.370.1 | not printed | 1.26.11 and 1.27.4 | Hosted Compute Agent, Azure |
| CI, Windows | `windows-2025-vs2026`, image 20261103.298.1 | not printed | 1.26.11 and 1.27.4 | the same |

CI's figures are `agentproc`'s time in two steps of each Linux and Windows job (`go test -shuffle=on ./...` and the race step
`go test -race -shuffle=on ./...`), read from the logs of the 13 most recent CI workflow runs on `main` that have them (26 jobs
of each OS, both Go versions; the oldest, a scheduled run, has no per-step times):

| CI job OS | Step | Jobs | Min, s | Median | p90 | Max |
| :--- | :--- | ---: | ---: | ---: | ---: | ---: |
| Linux | `go test -shuffle=on ./...` | 26 | 7.4 | 8.2 | 20.4 | 21.1 |
| Linux | race step | 26 | 23.6 | 31.9 | 42.4 | 50.7 |
| Windows | `go test -shuffle=on ./...` | 26 | 9.0 | 11.3 | 20.1 | 21.9 |
| Windows | race step | 26 | 22.9 | 34.3 | 39.9 | 42.4 |

All 52 jobs' race steps passed. `agentproc`'s plain-step time rose from about 8 s to about 20 s at `f102c5a`, the commit that
raised the graces (`8b0b051` and `f102c5a`: Linux 20.4 to 21.1 s, Windows 20.1 to 21.9 s, against 7.4 to 9.6 s and 9.0 to 12.9 s
before); the plan's phase 4 recorded the race times only, so this was not seen there. It is not investigated here.

## Results

### The wait the 30 s bound waits for

Seconds from the start of `awaitReady` (or the `select`) to "ready" (to `Serve`'s write returning), with the bound lifted;
"Over 30 s" counts the runs that would have failed at today's bound. No run timed out at the lifted bounds, and none failed for
another reason, but the four runs of c2, whose 5 s graces are as written and whose `Close` failed ("Other failure").

| Condition | Test | Runs | Min | Median | p90 | Max | Over 30 s | Timed out | Other failure |
| :--- | :--- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| Linux host, alone, idle, `-race` (a) | `TestDetachedTapIsPrefix` | 10 | 10.1 | 10.6 | 11.3 | 12.3 | 0 | 0 | 0 |
| Linux host, alone, idle, `-race` (a) | `TestInProcessDetachedTapIsPrefix` | 10 | 9.3 | 9.5 | 9.6 | 9.7 | 0 | 0 | 0 |
| Linux host, alone, idle, `-race` (a) | `TestBlockedTapDetached` | 10 | 8.5 | 8.9 | 9.3 | 10.2 | 0 | 0 | 0 |
| Linux host, alone, idle, `-race` (a) | `TestTapDrainBoundedByGrace` | 10 | 4.2 | 4.4 | 4.6 | 4.7 | 0 | 0 | 0 |
| Linux host, beside the module as CI's step runs it, `-race` (b) | `TestDetachedTapIsPrefix` | 10 | 21.2 | 25.8 | 32.2 | 32.6 | 2 | 0 | 0 |
| Linux host, beside the module as CI's step runs it, `-race` (b) | `TestInProcessDetachedTapIsPrefix` | 10 | 13.8 | 24.0 | 26.9 | 27.1 | 0 | 0 | 0 |
| Linux host, beside the module as CI's step runs it, `-race` (b) | `TestBlockedTapDetached` | 10 | 16.9 | 22.3 | 30.0 | 32.8 | 1 | 0 | 0 |
| Linux host, beside the module as CI's step runs it, `-race` (b) | `TestTapDrainBoundedByGrace` | 10 | 6.3 | 8.8 | 12.6 | 19.2 | 0 | 0 | 0 |
| Linux host, whole package beside the in-process starved load, `-race` (c) | `TestDetachedTapIsPrefix` | 5 | 103.4 | 106.4 | 114.3 | 115.7 | 5 | 0 | 0 |
| Linux host, whole package beside the in-process starved load, `-race` (c) | `TestInProcessDetachedTapIsPrefix` | 5 | 90.6 | 93.4 | 100.8 | 104.3 | 5 | 0 | 0 |
| Linux host, whole package beside the in-process starved load, `-race` (c) | `TestBlockedTapDetached` | 5 | 91.4 | 91.9 | 96.0 | 98.3 | 5 | 0 | 0 |
| Linux host, whole package beside the in-process starved load, `-race` (c) | `TestTapDrainBoundedByGrace` | 5 | 43.1 | 45.8 | 47.3 | 47.5 | 5 | 0 | 0 |
| Linux host, as (c), the 5 s graces as the tests have them (c2) | `TestDetachedTapIsPrefix` | 2 | 106.7 | 106.8 | 106.9 | 106.9 | 2 | 0 | 2 |
| Linux host, as (c), the 5 s graces as the tests have them (c2) | `TestInProcessDetachedTapIsPrefix` | 2 | 97.5 | 99.0 | 100.2 | 100.5 | 2 | 0 | 2 |
| Linux host, as (c), the 5 s graces as the tests have them (c2) | `TestBlockedTapDetached` | 2 | 92.0 | 94.8 | 97.0 | 97.5 | 2 | 0 | 0 |
| Linux host, as (c), the 5 s graces as the tests have them (c2) | `TestTapDrainBoundedByGrace` | 2 | 46.0 | 47.3 | 48.4 | 48.7 | 2 | 0 | 0 |
| Windows laptop, alone, idle, `-race` (d) | `TestDetachedTapIsPrefix` | 10 | 3.8 | 4.5 | 5.1 | 5.7 | 0 | 0 | 0 |
| Windows laptop, alone, idle, `-race` (d) | `TestInProcessDetachedTapIsPrefix` | 10 | 3.6 | 3.8 | 3.9 | 4.1 | 0 | 0 | 0 |
| Windows laptop, alone, idle, `-race` (d) | `TestBlockedTapDetached` | 10 | 2.8 | 2.9 | 3.0 | 3.3 | 0 | 0 | 0 |
| Windows laptop, alone, idle, `-race` (d) | `TestTapDrainBoundedByGrace` | 10 | 1.4 | 1.4 | 1.4 | 1.4 | 0 | 0 | 0 |
| Windows laptop, whole package, idle, `-race` (d) | `TestDetachedTapIsPrefix` | 10 | 36.8 | 41.3 | 43.8 | 46.0 | 10 | 0 | 0 |
| Windows laptop, whole package, idle, `-race` (d) | `TestInProcessDetachedTapIsPrefix` | 10 | 33.7 | 37.9 | 40.8 | 41.3 | 10 | 0 | 0 |
| Windows laptop, whole package, idle, `-race` (d) | `TestBlockedTapDetached` | 10 | 32.5 | 36.4 | 39.5 | 41.0 | 10 | 0 | 0 |
| Windows laptop, whole package, idle, `-race` (d) | `TestTapDrainBoundedByGrace` | 10 | 16.8 | 19.2 | 21.2 | 22.0 | 0 | 0 | 0 |
| Windows laptop, whole package, idle, `-race`, scratch experiments in the package (d, contaminated) | `TestDetachedTapIsPrefix` | 7 | 38.2 | 39.7 | 42.6 | 43.1 | 7 | 0 | 0 |
| Windows laptop, whole package, idle, `-race`, scratch experiments in the package (d, contaminated) | `TestInProcessDetachedTapIsPrefix` | 7 | 37.3 | 38.8 | 41.3 | 42.0 | 7 | 0 | 0 |
| Windows laptop, whole package, idle, `-race`, scratch experiments in the package (d, contaminated) | `TestBlockedTapDetached` | 7 | 33.7 | 36.0 | 38.2 | 38.9 | 7 | 0 | 0 |
| Windows laptop, whole package, idle, `-race`, scratch experiments in the package (d, contaminated) | `TestTapDrainBoundedByGrace` | 7 | 18.2 | 20.1 | 21.4 | 21.5 | 0 | 0 | 0 |
| Windows laptop, whole package, `-race`, the 5 s graces as the tests have them (d2) | `TestDetachedTapIsPrefix` | 5 | 36.2 | 37.2 | 39.6 | 40.1 | 5 | 0 | 0 |
| Windows laptop, whole package, `-race`, the 5 s graces as the tests have them (d2) | `TestInProcessDetachedTapIsPrefix` | 5 | 32.6 | 34.7 | 36.4 | 36.5 | 5 | 0 | 0 |
| Windows laptop, whole package, `-race`, the 5 s graces as the tests have them (d2) | `TestBlockedTapDetached` | 5 | 31.0 | 33.2 | 35.4 | 35.4 | 5 | 0 | 0 |
| Windows laptop, whole package, `-race`, the 5 s graces as the tests have them (d2) | `TestTapDrainBoundedByGrace` | 5 | 15.9 | 18.3 | 18.9 | 19.0 | 0 | 0 | 0 |
| Windows laptop, whole package, every bound as written (`W60`): a timed-out wait counts 30 s | `TestDetachedTapIsPrefix` | 3 | 30.0 | 30.0 | 30.0 | 30.0 | 3 | 3 | 0 |
| Windows laptop, whole package, every bound as written (`W60`): a timed-out wait counts 30 s | `TestInProcessDetachedTapIsPrefix` | 3 | 30.0 | 30.0 | 30.0 | 30.0 | 3 | 3 | 0 |
| Windows laptop, whole package, every bound as written (`W60`): a timed-out wait counts 30 s | `TestBlockedTapDetached` | 3 | 30.0 | 30.0 | 30.0 | 30.0 | 3 | 3 | 0 |
| Windows laptop, whole package, every bound as written (`W60`): a timed-out wait counts 30 s | `TestTapDrainBoundedByGrace` | 3 | 17.5 | 18.2 | 20.9 | 21.6 | 0 | 0 | 0 |
| This Mac, alone, `-race` (e) | `TestDetachedTapIsPrefix` | 10 | 5.7 | 5.7 | 5.8 | 5.8 | 0 | 0 | 0 |
| This Mac, alone, `-race` (e) | `TestInProcessDetachedTapIsPrefix` | 10 | 5.1 | 5.1 | 5.2 | 5.2 | 0 | 0 | 0 |
| This Mac, alone, `-race` (e) | `TestBlockedTapDetached` | 10 | 4.7 | 4.8 | 4.9 | 5.3 | 0 | 0 | 0 |
| This Mac, alone, `-race` (e) | `TestTapDrainBoundedByGrace` | 10 | 2.3 | 2.4 | 2.4 | 2.4 | 0 | 0 | 0 |
| This Mac, whole package, `-race` | `TestDetachedTapIsPrefix` | 3 | 10.8 | 11.7 | 12.1 | 12.2 | 0 | 0 | 0 |
| This Mac, whole package, `-race` | `TestInProcessDetachedTapIsPrefix` | 3 | 11.1 | 11.1 | 11.5 | 11.6 | 0 | 0 | 0 |
| This Mac, whole package, `-race` | `TestBlockedTapDetached` | 3 | 10.3 | 10.8 | 10.9 | 10.9 | 0 | 0 | 0 |
| This Mac, whole package, `-race` | `TestTapDrainBoundedByGrace` | 3 | 6.0 | 6.1 | 6.1 | 6.1 | 0 | 0 | 0 |
| Linux host, alone, no `-race` | `TestDetachedTapIsPrefix` | 10 | 0.5 | 0.6 | 0.6 | 0.6 | 0 | 0 | 0 |
| Linux host, alone, no `-race` | `TestInProcessDetachedTapIsPrefix` | 10 | 0.3 | 0.3 | 0.4 | 0.4 | 0 | 0 | 0 |
| Linux host, alone, no `-race` | `TestBlockedTapDetached` | 10 | 0.4 | 0.4 | 0.5 | 0.5 | 0 | 0 | 0 |
| Linux host, alone, no `-race` | `TestTapDrainBoundedByGrace` | 10 | 0.2 | 0.2 | 0.3 | 0.4 | 0 | 0 | 0 |
| Windows laptop, alone, no `-race` | `TestDetachedTapIsPrefix` | 10 | 0.2 | 0.2 | 0.2 | 0.2 | 0 | 0 | 0 |
| Windows laptop, alone, no `-race` | `TestInProcessDetachedTapIsPrefix` | 10 | 0.1 | 0.1 | 0.1 | 0.1 | 0 | 0 | 0 |
| Windows laptop, alone, no `-race` | `TestBlockedTapDetached` | 10 | 0.1 | 0.1 | 0.1 | 0.2 | 0 | 0 | 0 |
| Windows laptop, alone, no `-race` | `TestTapDrainBoundedByGrace` | 10 | 0.1 | 0.1 | 0.1 | 0.1 | 0 | 0 | 0 |
| Windows laptop, whole package, no `-race` | `TestDetachedTapIsPrefix` | 3 | 0.2 | 0.2 | 0.3 | 0.4 | 0 | 0 | 0 |
| Windows laptop, whole package, no `-race` | `TestInProcessDetachedTapIsPrefix` | 3 | 0.2 | 0.2 | 0.2 | 0.2 | 0 | 0 | 0 |
| Windows laptop, whole package, no `-race` | `TestBlockedTapDetached` | 3 | 0.2 | 0.2 | 0.2 | 0.3 | 0 | 0 | 0 |
| Windows laptop, whole package, no `-race` | `TestTapDrainBoundedByGrace` | 3 | 0.1 | 0.1 | 0.1 | 0.1 | 0 | 0 | 0 |
| This Mac, alone, no `-race` | `TestDetachedTapIsPrefix` | 10 | 0.2 | 0.2 | 0.3 | 0.3 | 0 | 0 | 0 |
| This Mac, alone, no `-race` | `TestInProcessDetachedTapIsPrefix` | 10 | 0.1 | 0.1 | 0.1 | 0.1 | 0 | 0 | 0 |
| This Mac, alone, no `-race` | `TestBlockedTapDetached` | 10 | 0.2 | 0.2 | 0.2 | 0.2 | 0 | 0 | 0 |
| This Mac, alone, no `-race` | `TestTapDrainBoundedByGrace` | 10 | 0.1 | 0.1 | 0.1 | 0.1 | 0 | 0 | 0 |

The rows marked "contaminated" are whole-package rounds on the Windows laptop that ran with the scratch experiments in the
package (the experiments skip unless asked, from the third Windows package chunk on); they are kept and marked, and the clean
rounds are the rows above them.

### The close of the 56 MiB tests, and the 5 s grace

`Close`'s first step waits `Grace` (5 s in both tests) for the agent's exit from the moment `Close` closes the agent's input;
after it, SIGTERM and `ErrTerminated`, the failure of the audit's round 1 ("Close = agentproc: agent terminated: signal:
terminated"). After `go`, the agent writes the last 8 MiB, which the connection must read before the agent's write returns and
it exits, so what the grace waits on is the connection's read of those 8 MiB, at the rate of the row above, from a standing start
of an empty pipe.

| Condition | Test | Runs | Close median | Close max | First step median | First step max | Over 5 s | Close failed |
| :--- | :--- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| Linux host, alone, idle, `-race` (a) | `TestDetachedTapIsPrefix` | 10 | 1.74 | 2.46 | 1.62 | 2.34 | 0 | 0 |
| Linux host, alone, idle, `-race` (a) | `TestInProcessDetachedTapIsPrefix` | 10 | 1.86 | 2.24 | 1.69 | 2.05 | 0 | 0 |
| Linux host, beside the module as CI's step runs it, `-race` (b) | `TestDetachedTapIsPrefix` | 10 | 2.95 | 5.16 | 2.76 | 4.78 | 0 | 0 |
| Linux host, beside the module as CI's step runs it, `-race` (b) | `TestInProcessDetachedTapIsPrefix` | 10 | 3.79 | 6.45 | 3.46 | 5.93 | 1 | 0 |
| Linux host, whole package beside the in-process starved load, `-race` (c) | `TestDetachedTapIsPrefix` | 5 | 18.01 | 19.16 | 16.44 | 17.94 | 5 | 0 |
| Linux host, whole package beside the in-process starved load, `-race` (c) | `TestInProcessDetachedTapIsPrefix` | 5 | 14.33 | 17.39 | 13.14 | 15.83 | 5 | 0 |
| Linux host, as (c), the 5 s graces as the tests have them (c2) | `TestDetachedTapIsPrefix` | 2 | 5.77 | 5.83 | 5.69 | 5.79 | 2 | 2 |
| Linux host, as (c), the 5 s graces as the tests have them (c2) | `TestInProcessDetachedTapIsPrefix` | 2 | 11.04 | 11.05 | 10.30 | 10.39 | 2 | 2 |
| Windows laptop, alone, idle, `-race` (d) | `TestDetachedTapIsPrefix` | 10 | 1.34 | 1.56 | 1.24 | 1.44 | 0 | 0 |
| Windows laptop, alone, idle, `-race` (d) | `TestInProcessDetachedTapIsPrefix` | 10 | 1.51 | 1.56 | 1.34 | 1.40 | 0 | 0 |
| Windows laptop, whole package, idle, `-race` (d) | `TestDetachedTapIsPrefix` | 10 | 2.38 | 3.14 | 2.25 | 2.99 | 0 | 0 |
| Windows laptop, whole package, idle, `-race` (d) | `TestInProcessDetachedTapIsPrefix` | 10 | 4.73 | 5.24 | 4.22 | 4.70 | 0 | 0 |
| Windows laptop, whole package, idle, `-race`, scratch experiments in the package (d, contaminated) | `TestDetachedTapIsPrefix` | 7 | 3.32 | 3.68 | 3.19 | 3.55 | 0 | 0 |
| Windows laptop, whole package, idle, `-race`, scratch experiments in the package (d, contaminated) | `TestInProcessDetachedTapIsPrefix` | 7 | 4.46 | 4.89 | 3.96 | 4.39 | 0 | 0 |
| Windows laptop, whole package, `-race`, the 5 s graces as the tests have them (d2) | `TestDetachedTapIsPrefix` | 5 | 2.78 | 3.35 | 2.64 | 3.22 | 0 | 0 |
| Windows laptop, whole package, `-race`, the 5 s graces as the tests have them (d2) | `TestInProcessDetachedTapIsPrefix` | 5 | 4.52 | 4.89 | 4.03 | 4.34 | 0 | 0 |
| This Mac, alone, `-race` (e) | `TestDetachedTapIsPrefix` | 10 | 0.82 | 0.86 | 0.74 | 0.78 | 0 | 0 |
| This Mac, alone, `-race` (e) | `TestInProcessDetachedTapIsPrefix` | 10 | 0.84 | 0.85 | 0.75 | 0.76 | 0 | 0 |
| This Mac, whole package, `-race` | `TestDetachedTapIsPrefix` | 3 | 1.16 | 1.91 | 1.04 | 1.67 | 0 | 0 |
| This Mac, whole package, `-race` | `TestInProcessDetachedTapIsPrefix` | 3 | 1.18 | 1.20 | 1.05 | 1.08 | 0 | 0 |
| Linux host, alone, no `-race` | `TestDetachedTapIsPrefix` | 10 | 0.09 | 0.13 | 0.09 | 0.13 | 0 | 0 |
| Linux host, alone, no `-race` | `TestInProcessDetachedTapIsPrefix` | 10 | 0.07 | 0.08 | 0.07 | 0.08 | 0 | 0 |
| Windows laptop, alone, no `-race` | `TestDetachedTapIsPrefix` | 10 | 0.02 | 0.03 | 0.02 | 0.03 | 0 | 0 |
| Windows laptop, alone, no `-race` | `TestInProcessDetachedTapIsPrefix` | 10 | 0.02 | 0.02 | 0.01 | 0.02 | 0 | 0 |
| Windows laptop, whole package, no `-race` | `TestDetachedTapIsPrefix` | 3 | 0.03 | 0.07 | 0.02 | 0.07 | 0 | 0 |
| Windows laptop, whole package, no `-race` | `TestInProcessDetachedTapIsPrefix` | 3 | 0.02 | 0.03 | 0.02 | 0.02 | 0 | 0 |
| This Mac, alone, no `-race` | `TestDetachedTapIsPrefix` | 10 | 0.02 | 0.03 | 0.02 | 0.03 | 0 | 0 |
| This Mac, alone, no `-race` | `TestInProcessDetachedTapIsPrefix` | 10 | 0.02 | 0.03 | 0.02 | 0.02 | 0 | 0 |

With the 5 s graces as written (c2, readiness lifted), the failures of `Close` seen were: 2 times at inprocess_test.go:480, `Close
= agentproc: agent killed: io: read/write on closed pipe`; 2 times at tap_test.go:372, `Close = agentproc: agent terminated:
signal: terminated`. In d2 (the Windows package, five rounds, the graces as written) no `Close` failed: the first step waited 2.6
to 4.3 s.

### Where the time goes

Medians over the runs of each row, `-race`. "Reads to ready" is the connection's read rate between 4 MiB marks, minimum, median
and maximum; a steady reader has the three close together.

| Condition | Test | Child start to code | Child builds the stream, s | Write of the first part, s | Reads to ready, MiB/s (4 MiB marks: min / median / max) | Longest gap between marks, s | `json.Valid`, MiB/s | Envelope `Unmarshal`, MiB/s | Tap offer, MiB/s | Collector pause to ready, s |
| :--- | :--- | ---: | ---: | ---: | :--- | ---: | ---: | ---: | ---: | ---: |
| Linux host, alone, idle, `-race` (a) | `TestDetachedTapIsPrefix` | 0.02 | 0.93 | 9.7 | 4.9 / 5.7 / 6.3 | 0.81 | 18.4 | 9.7 | 574 | 0.01 |
| Linux host, alone, idle, `-race` (a) | `TestInProcessDetachedTapIsPrefix` | - | - | 9.5 | 5.3 / 6.0 / 6.4 | 0.75 | 19.2 | 10.2 | 331 | 0.00 |
| Linux host, alone, idle, `-race` (a) | `TestBlockedTapDetached` | 0.02 | 0.65 | 8.2 | 5.3 / 5.8 / 6.2 | 0.75 | 19.0 | 10.1 | 465 | 0.00 |
| Linux host, alone, idle, `-race` (a) | `TestTapDrainBoundedByGrace` | 0.02 | 0.35 | 4.0 | 5.4 / 5.9 / 6.3 | 0.74 | 19.3 | 10.2 | 306 | 0.00 |
| Linux host, beside the module as CI's step runs it, `-race` (b) | `TestDetachedTapIsPrefix` | 0.03 | 1.54 | 24.4 | 1.6 / 2.4 / 3.5 | 2.43 | 7.7 | 4.0 | 378 | 0.09 |
| Linux host, beside the module as CI's step runs it, `-race` (b) | `TestInProcessDetachedTapIsPrefix` | - | - | 24.0 | 1.6 / 2.5 / 3.7 | 2.49 | 7.7 | 3.9 | 152 | 0.04 |
| Linux host, beside the module as CI's step runs it, `-race` (b) | `TestBlockedTapDetached` | 0.02 | 0.80 | 21.5 | 1.6 / 2.2 / 2.8 | 2.57 | 7.0 | 3.9 | 280 | 0.06 |
| Linux host, beside the module as CI's step runs it, `-race` (b) | `TestTapDrainBoundedByGrace` | 0.02 | 0.43 | 8.4 | 2.2 / 3.0 / 3.5 | 1.86 | 8.7 | 4.7 | 202 | 0.03 |
| Linux host, whole package beside the in-process starved load, `-race` (c) | `TestDetachedTapIsPrefix` | 0.02 | 0.92 | 105.2 | 0.4 / 0.5 / 0.7 | 9.77 | 1.9 | 1.0 | 61 | 0.00 |
| Linux host, whole package beside the in-process starved load, `-race` (c) | `TestInProcessDetachedTapIsPrefix` | - | - | 93.4 | 0.5 / 0.6 / 0.7 | 8.35 | 1.8 | 1.1 | 112 | 0.00 |
| Linux host, whole package beside the in-process starved load, `-race` (c) | `TestBlockedTapDetached` | 0.02 | 0.68 | 90.9 | 0.4 / 0.5 / 0.6 | 9.50 | 1.9 | 1.0 | 32 | 0.00 |
| Linux host, whole package beside the in-process starved load, `-race` (c) | `TestTapDrainBoundedByGrace` | 0.02 | 0.35 | 45.1 | 0.5 / 0.5 / 0.6 | 8.64 | 1.9 | 1.1 | 19 | 0.00 |
| Linux host, as (c), the 5 s graces as the tests have them (c2) | `TestDetachedTapIsPrefix` | 0.02 | 0.96 | 105.2 | 0.4 / 0.5 / 0.6 | 9.42 | 2.0 | 1.0 | 26 | 0.00 |
| Linux host, as (c), the 5 s graces as the tests have them (c2) | `TestInProcessDetachedTapIsPrefix` | - | - | 99.0 | 0.4 / 0.6 / 0.7 | 9.32 | 1.7 | 1.0 | 232 | 0.00 |
| Linux host, as (c), the 5 s graces as the tests have them (c2) | `TestBlockedTapDetached` | 0.02 | 0.79 | 93.5 | 0.4 / 0.5 / 0.6 | 10.30 | 1.9 | 1.0 | 49 | 0.00 |
| Linux host, as (c), the 5 s graces as the tests have them (c2) | `TestTapDrainBoundedByGrace` | 0.02 | 0.38 | 46.5 | 0.5 / 0.5 / 0.5 | 8.58 | 2.0 | 1.0 | 24 | 0.00 |
| Windows laptop, alone, idle, `-race` (d) | `TestDetachedTapIsPrefix` | 0.06 | 0.25 | 4.2 | 6.4 / 16.5 / 18.0 | 0.62 | 36.3 | 18.2 | 3093 | 0.02 |
| Windows laptop, alone, idle, `-race` (d) | `TestInProcessDetachedTapIsPrefix` | - | - | 3.8 | 6.0 / 17.6 / 18.5 | 0.67 | 38.2 | 19.4 | 2453 | 0.00 |
| Windows laptop, alone, idle, `-race` (d) | `TestBlockedTapDetached` | 0.05 | 0.18 | 2.7 | 17.1 / 17.6 / 18.4 | 0.23 | 55.7 | 28.1 | 1812 | 0.02 |
| Windows laptop, alone, idle, `-race` (d) | `TestTapDrainBoundedByGrace` | 0.05 | 0.09 | 1.3 | 17.6 / 18.0 / 18.2 | 0.23 | 56.8 | 29.0 | 1463 | 0.02 |
| Windows laptop, whole package, idle, `-race` (d) | `TestDetachedTapIsPrefix` | 0.97 | 0.27 | 41.0 | 1.1 / 1.4 / 1.9 | 3.51 | 4.5 | 2.3 | 894 | 0.94 |
| Windows laptop, whole package, idle, `-race` (d) | `TestInProcessDetachedTapIsPrefix` | - | - | 37.9 | 1.1 / 1.4 / 6.6 | 3.54 | 4.5 | 2.3 | 877 | 0.86 |
| Windows laptop, whole package, idle, `-race` (d) | `TestBlockedTapDetached` | 1.04 | 0.20 | 36.1 | 1.1 / 1.4 / 1.6 | 3.55 | 4.0 | 2.0 | 491 | 0.86 |
| Windows laptop, whole package, idle, `-race` (d) | `TestTapDrainBoundedByGrace` | 1.02 | 0.11 | 19.1 | 1.1 / 1.2 / 1.2 | 3.56 | 3.7 | 1.8 | 441 | 0.55 |
| Windows laptop, whole package, idle, `-race`, scratch experiments in the package (d, contaminated) | `TestDetachedTapIsPrefix` | 2.20 | 0.25 | 39.3 | 1.1 / 1.4 / 2.2 | 3.65 | 4.6 | 2.3 | 833 | 1.06 |
| Windows laptop, whole package, idle, `-race`, scratch experiments in the package (d, contaminated) | `TestInProcessDetachedTapIsPrefix` | - | - | 38.8 | 1.1 / 1.3 / 2.0 | 3.62 | 4.5 | 2.3 | 762 | 0.99 |
| Windows laptop, whole package, idle, `-race`, scratch experiments in the package (d, contaminated) | `TestBlockedTapDetached` | 2.27 | 0.19 | 35.7 | 1.1 / 1.3 / 1.6 | 3.70 | 4.1 | 2.0 | 399 | 0.99 |
| Windows laptop, whole package, idle, `-race`, scratch experiments in the package (d, contaminated) | `TestTapDrainBoundedByGrace` | 2.21 | 0.10 | 19.9 | 1.1 / 1.2 / 1.3 | 3.58 | 3.6 | 1.8 | 308 | 0.69 |
| Windows laptop, whole package, `-race`, the 5 s graces as the tests have them (d2) | `TestDetachedTapIsPrefix` | 1.00 | 0.28 | 36.9 | 1.2 / 1.5 / 2.0 | 3.43 | 5.0 | 2.4 | 915 | 0.90 |
| Windows laptop, whole package, `-race`, the 5 s graces as the tests have them (d2) | `TestInProcessDetachedTapIsPrefix` | - | - | 34.7 | 1.2 / 1.5 / 4.5 | 3.33 | 4.9 | 2.5 | 796 | 0.83 |
| Windows laptop, whole package, `-race`, the 5 s graces as the tests have them (d2) | `TestBlockedTapDetached` | 0.99 | 0.21 | 32.9 | 1.2 / 1.4 / 1.6 | 3.29 | 4.3 | 2.2 | 564 | 0.83 |
| Windows laptop, whole package, `-race`, the 5 s graces as the tests have them (d2) | `TestTapDrainBoundedByGrace` | 0.99 | 0.11 | 18.1 | 1.2 / 1.3 / 1.4 | 3.30 | 4.0 | 1.9 | 420 | 0.52 |
| This Mac, alone, `-race` (e) | `TestDetachedTapIsPrefix` | 0.02 | 0.51 | 5.2 | 10.5 / 10.7 / 10.8 | 0.38 | 35.7 | 16.0 | 1915 | 0.01 |
| This Mac, alone, `-race` (e) | `TestInProcessDetachedTapIsPrefix` | - | - | 5.1 | 10.4 / 10.7 / 10.9 | 0.38 | 36.1 | 16.1 | 1979 | 0.00 |
| This Mac, alone, `-race` (e) | `TestBlockedTapDetached` | 0.02 | 0.38 | 4.4 | 10.6 / 10.8 / 10.9 | 0.38 | 36.3 | 16.3 | 1485 | 0.00 |
| This Mac, alone, `-race` (e) | `TestTapDrainBoundedByGrace` | 0.03 | 0.20 | 2.1 | 10.7 / 10.8 / 10.9 | 0.37 | 36.4 | 16.4 | 1020 | 0.00 |
| This Mac, whole package, `-race` | `TestDetachedTapIsPrefix` | 0.03 | 0.53 | 11.1 | 3.8 / 5.2 / 8.0 | 1.05 | 16.6 | 7.9 | 1640 | 0.08 |
| This Mac, whole package, `-race` | `TestInProcessDetachedTapIsPrefix` | - | - | 11.1 | 3.7 / 5.1 / 7.9 | 1.08 | 16.8 | 7.9 | 1539 | 0.07 |
| This Mac, whole package, `-race` | `TestBlockedTapDetached` | 0.03 | 0.40 | 10.4 | 3.7 / 4.4 / 6.4 | 1.07 | 14.6 | 7.1 | 1195 | 0.08 |
| This Mac, whole package, `-race` | `TestTapDrainBoundedByGrace` | 0.03 | 0.21 | 5.8 | 3.8 / 4.0 / 4.1 | 1.06 | 12.5 | 6.0 | 688 | 0.06 |

What each experiment settled, and what it did not:

| Host | What | Seconds | MiB/s | Detail |
| :--- | :--- | ---: | ---: | :--- |
| Linux host | the child's work before it writes: build 56 MiB of `bulkStream` (5 runs) | 0.81 | 69 | |
| Linux host | the connection alone, 56 MiB from memory, no child, no pipe, no tap (5 runs) | 9.18 | 6.1 | its `json.Valid` 2.80 s, envelope `Unmarshal` 5.42 s, reading lines 0.90 s |
| Linux host | `json.Valid`, then the envelope `Unmarshal`, on the 56 lines, no connection (5 runs) | 3.00 + 5.43 | 19 and 10 | |
| Linux host | `os.Pipe`, 56 MiB, written whole, read 128 KiB at a time (5 runs) | 0.280 | 200 | 897 reads, 64 KiB each on average |
| Linux host | `os.Pipe`, 56 MiB, written in pieces of 1024 KiB, read 128 KiB at a time (5 runs) | 0.289 | 194 | 897 reads, 64 KiB each on average |
| Linux host | `os.Pipe`, 56 MiB, written in pieces of 4 KiB, read 128 KiB at a time (5 runs) | 0.428 | 131 | 12816 reads, 4 KiB each on average |
| Linux host | `os.Pipe`, 56 MiB, written in pieces of 64 KiB, read 128 KiB at a time (5 runs) | 0.292 | 192 | 897 reads, 64 KiB each on average |
| Windows laptop | the child's work before it writes: build 56 MiB of `bulkStream` (5 runs) | 0.19 | 289 | |
| Windows laptop | the connection alone, 56 MiB from memory, no child, no pipe, no tap (5 runs) | 9.38 | 6.0 | its `json.Valid` 3.06 s, envelope `Unmarshal` 5.95 s, reading lines 0.31 s |
| Windows laptop | `json.Valid`, then the envelope `Unmarshal`, on the 56 lines, no connection (5 runs) | 1.04 + 2.71 | 54 and 21 | |
| Windows laptop | `os.Pipe`, 56 MiB, written whole, read 128 KiB at a time (5 runs) | 0.033 | 1682 | 449 reads, 128 KiB each on average |
| Windows laptop | `os.Pipe`, 56 MiB, written in pieces of 1024 KiB, read 128 KiB at a time (5 runs) | 0.033 | 1715 | 449 reads, 128 KiB each on average |
| Windows laptop | `os.Pipe`, 56 MiB, written in pieces of 4 KiB, read 128 KiB at a time (5 runs) | 0.067 | 837 | 14184 reads, 4 KiB each on average |
| Windows laptop | `os.Pipe`, 56 MiB, written in pieces of 64 KiB, read 128 KiB at a time (5 runs) | 0.027 | 2083 | 897 reads, 64 KiB each on average |
| this Mac | the child's work before it writes: build 56 MiB of `bulkStream` (5 runs) | 0.45 | 124 | |
| this Mac | the connection alone, 56 MiB from memory, no child, no pipe, no tap (5 runs) | 5.09 | 11.0 | its `json.Valid` 1.54 s, envelope `Unmarshal` 3.42 s, reading lines 0.12 s |
| this Mac | `json.Valid`, then the envelope `Unmarshal`, on the 56 lines, no connection (5 runs) | 1.60 + 3.42 | 35 and 16 | |
| this Mac | `os.Pipe`, 56 MiB, written whole, read 128 KiB at a time (5 runs) | 0.045 | 1231 | 897 reads, 64 KiB each on average |
| this Mac | `os.Pipe`, 56 MiB, written in pieces of 1024 KiB, read 128 KiB at a time (5 runs) | 0.039 | 1436 | 897 reads, 64 KiB each on average |
| this Mac | `os.Pipe`, 56 MiB, written in pieces of 4 KiB, read 128 KiB at a time (5 runs) | 0.056 | 1007 | 9415 reads, 6 KiB each on average |
| this Mac | `os.Pipe`, 56 MiB, written in pieces of 64 KiB, read 128 KiB at a time (5 runs) | 0.040 | 1416 | 897 reads, 64 KiB each on average |

| Host | Experiment, `TestDetachedTapIsPrefix`'s agent, 56 MiB | Runs | Median s to ready | Max |
| :--- | :--- | ---: | ---: | ---: |
| Linux host | as written (a) | 10 | 10.59 | 12.28 |
| Linux host | no output tap | 5 | 10.61 | 10.94 |
| Linux host | child writes 64 KiB at a time | 5 | 10.42 | 11.69 |
| Linux host | child writes 1 MiB at a time | 5 | 10.49 | 11.19 |
| Windows laptop | as written (d) | 10 | 4.47 | 5.72 |
| Windows laptop | no output tap | 5 | 3.85 | 4.23 |
| Windows laptop | child writes 64 KiB at a time | 5 | 4.02 | 4.37 |
| Windows laptop | child writes 1 MiB at a time | 5 | 3.98 | 4.20 |
| this Mac | as written (e) | 10 | 5.73 | 5.81 |
| this Mac | no output tap | 5 | 5.60 | 5.78 |
| this Mac | child writes 64 KiB at a time | 5 | 5.64 | 5.73 |
| this Mac | child writes 1 MiB at a time | 5 | 5.62 | 5.73 |

- **The tap is not in it:** the agent without an output tap came to "ready" in 10.61, 3.85 and 5.60 s (Linux host, Windows
  laptop, this Mac), as written in 10.59, 4.47 and 5.73 s.
- **Write size is not in it:** 64 KiB and 1 MiB pieces, against one write, changed the median by 0.2 s or less on the Linux
  host and this Mac and by 0.5 s down on the Windows laptop; the child's write is a lockstep through a 64 KiB pipe whatever its size.
- **The pipe is not in it:** an `os.Pipe` moved 56 MiB in 0.03 to 0.43 s.
- **The child's start and build are small:** 0.02 to 0.06 s to its first line of code (0.9 to 1.3 s inside the Windows package,
  where process starts are slower) and 0.08 to 2.1 s to build its stream, 5 to 9 % of the wait alone. The tap's `offer` is 0.2 to
  1.6 % of it, and the time blocked in the pipe's `Read` 1 to 10 %.
- **The connection's scans are:** the connection fed 56 MiB from memory, with no child, pipe or tap, took 9.2, 5.1 and 9.3 s
  (Linux host, this Mac, Windows laptop) against the pipeline's 10.6, 5.7 and 4.5 s. On the Linux host and this Mac the
  connection is the whole wait (the child, the pipe and the tap add 0.5 to 1.4 s), and its `json.Valid` and `Unmarshal` took
  8.2 of its 9.2 s on the Linux host. On the Windows laptop the in-memory run was twice the pipeline's, with the same
  `json.Valid` taking 3.1 s inside it against 1.0 s outside: that difference is not explained here.
- **One goroutine's work:** `GOMAXPROCS=1` on the Linux host changed nothing (10.4 s against 10.6 s): the reader is the critical
  path, and a second CPU does not shorten it.
- **Steady or stalled:** alone, the read rate between 4 MiB marks is steady on the Linux host and this Mac (4.9 to 6.3 and 10.5 to
  10.9 MiB/s). On the Windows laptop it is steady at 17 to 18 MiB/s through about 48 MiB and then 5.4 to 6.9 MiB/s for the last
  8 MiB of `TestDetachedTapIsPrefix` and `TestInProcessDetachedTapIsPrefix`, in 20 of 20 runs ("A sustained-load probe"). In the
  Windows package it is steady and low, 1.0 to 2.2 MiB/s, the longest gap between marks 3.0 to 4.2 s, the collector pausing
  0.4 to 1.1 s by "ready" and the heap 270 to 510 MiB at "ready". Under the starved load it is steady at 0.4 to 0.7 MiB/s, the
  longest gap between marks 8 to 10 s.

### How many run at once, and `GOMAXPROCS`

| Host | Test | Alone | Two bulk tests together | Four bulk tests together | Whole package | Beside the module |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| Linux host | `TestDetachedTapIsPrefix` | 10.6 / 12.3 (10) | 17.1 / 18.1 (5) | 25.3 / 27.5 (5) | - | 25.8 / 32.6 (10) |
| Linux host | `TestInProcessDetachedTapIsPrefix` | 9.5 / 9.7 (10) | 16.5 / 17.1 (5) | 24.2 / 26.4 (5) | - | 24.0 / 27.1 (10) |
| Linux host | `TestBlockedTapDetached` | 8.9 / 10.2 (10) | - | 23.0 / 24.2 (5) | - | 22.3 / 32.8 (10) |
| Linux host | `TestTapDrainBoundedByGrace` | 4.4 / 4.7 (10) | - | 12.0 / 13.1 (5) | - | 8.8 / 19.2 (10) |
| Windows laptop | `TestDetachedTapIsPrefix` | 4.5 / 5.7 (10) | 13.1 / 14.5 (5) | 28.4 / 28.8 (5) | 41.3 / 46.0 (10) | - |
| Windows laptop | `TestInProcessDetachedTapIsPrefix` | 3.8 / 4.1 (10) | 12.7 / 13.9 (5) | 27.9 / 28.4 (5) | 37.9 / 41.3 (10) | - |
| Windows laptop | `TestBlockedTapDetached` | 2.9 / 3.3 (10) | - | 25.7 / 26.0 (5) | 36.4 / 41.0 (10) | - |
| Windows laptop | `TestTapDrainBoundedByGrace` | 1.4 / 1.4 (10) | - | 11.4 / 11.8 (5) | 19.2 / 22.0 (10) | - |
| this Mac | `TestDetachedTapIsPrefix` | 5.7 / 5.8 (10) | 6.5 / 6.8 (5) | 8.7 / 9.8 (5) | 11.7 / 12.2 (3) | - |
| this Mac | `TestInProcessDetachedTapIsPrefix` | 5.1 / 5.2 (10) | 6.0 / 6.3 (5) | 8.1 / 8.9 (5) | 11.1 / 11.6 (3) | - |
| this Mac | `TestBlockedTapDetached` | 4.8 / 5.3 (10) | - | 7.7 / 8.5 (5) | 10.8 / 10.9 (3) | - |
| this Mac | `TestTapDrainBoundedByGrace` | 2.4 / 2.4 (10) | - | 4.1 / 4.7 (5) | 6.1 / 6.1 (3) | - |

| Host | Condition | `GOMAXPROCS` | `TestDetachedTapIsPrefix` | `TestInProcessDetachedTapIsPrefix` |
| :--- | :--- | ---: | :--- | :--- |
| Linux host | alone | 1 | 10.4 / 11.7 (5) | 9.9 / 10.7 (5) |
| Linux host | alone | 2 | 10.6 / 10.7 (5) | 9.9 / 11.0 (5) |
| Linux host | alone | 4 (default) | 10.6 / 12.3 (10) | 9.5 / 9.7 (10) |
| Windows laptop | alone | 4 | 4.1 / 4.2 (5) | 4.0 / 4.0 (5) |
| Windows laptop | alone | 8 | 4.3 / 4.3 (5) | 3.9 / 3.9 (5) |
| Windows laptop | alone | 32 (default) | 4.5 / 5.7 (10) | 3.8 / 4.1 (10) |
| Windows laptop | whole package | 4 | 32.7 / 35.0 (3) | 17.9 / 35.1 (3) |
| Windows laptop | whole package | 8 | 36.3 / 38.1 (3) | 36.5 / 39.2 (3) |
| Windows laptop | whole package | 16 | 40.2 / 42.2 (6) | 37.7 / 40.6 (6) |
| Windows laptop | whole package | 32 (default) | 41.3 / 46.0 (10) | 37.9 / 41.3 (10) |
| this Mac | whole package | 16 (10 CPUs; run on this Mac by run-on-host's fallback) | 11.8 / 12.0 (3) | 11.1 / 11.4 (3) |
| this Mac | whole package | 10 (default) | 11.7 / 12.2 (3) | 11.1 / 11.6 (3) |

### A sustained-load probe

Not a test of this repository: `N` goroutines each sum a 1 MiB buffer over and over, race-instrumented as the connection's
scans are, for 8 s each, for `N` = 1, 2, 4, 8, 16 in that order in one process. The table gives the units of work (MiB
summed) done per goroutine in each of the first eight seconds, in medians over the runs, and all goroutines' total in the first
and the last second.

| Host | Goroutines | Runs | Per goroutine per second: s0 | s1 | s2 | s3 | s4 | s5 | s6 | s7 | All together, s0 | s7 |
| :--- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| Linux host | 1 | 3 | 59 | 63 | 60 | 53 | 65 | 58 | 62 | 69 | 59 | 69 |
| Linux host | 2 | 3 | 64 | 56 | 66 | 64 | 66 | 56 | 60 | 63 | 128 | 126 |
| Linux host | 4 | 3 | 61 | 55 | 56 | 61 | 60 | 58 | 60 | 60 | 244 | 242 |
| Linux host | 8 | 3 | 28 | 30 | 31 | 29 | 30 | 29 | 30 | 27 | 226 | 218 |
| Linux host | 16 | 3 | 11 | 15 | 15 | 14 | 14 | 14 | 14 | 14 | 183 | 228 |
| Windows laptop | 1 | 3 | 244 | 243 | 243 | 63 | 63 | 63 | 62 | 63 | 244 | 63 |
| Windows laptop | 2 | 3 | 61 | 64 | 64 | 64 | 64 | 64 | 64 | 64 | 122 | 128 |
| Windows laptop | 4 | 3 | 62 | 65 | 65 | 65 | 65 | 65 | 65 | 65 | 247 | 259 |
| Windows laptop | 8 | 3 | 61 | 65 | 65 | 65 | 65 | 65 | 65 | 65 | 490 | 521 |
| Windows laptop | 16 | 3 | 58 | 64 | 64 | 64 | 64 | 64 | 64 | 64 | 927 | 1028 |
| this Mac | 1 | 3 | 123 | 127 | 127 | 127 | 127 | 127 | 127 | 128 | 123 | 128 |
| this Mac | 2 | 3 | 124 | 124 | 127 | 126 | 127 | 126 | 126 | 124 | 249 | 249 |
| this Mac | 4 | 3 | 122 | 123 | 124 | 122 | 123 | 124 | 124 | 124 | 489 | 497 |
| this Mac | 8 | 3 | 113 | 120 | 121 | 121 | 119 | 117 | 123 | 121 | 902 | 971 |
| this Mac | 16 | 3 | 63 | 66 | 67 | 67 | 66 | 66 | 66 | 65 | 1015 | 1037 |

On the Windows laptop one goroutine did 244 units a second for its first three seconds and 63 after, and two to sixteen
goroutines, started after it, did 61 to 65 each from their first second: sixteen did 1,028 a second between them, so the slow
speed is each goroutine's own, not a share of a fixed total. On this Mac the rate is flat (122 to 128 a goroutine up to 4
goroutines, 113 to 121 at 8) and halves when 16 share its 10 CPUs. On the Linux host the rate is flat in time (53 to 69 a
goroutine up to 4 goroutines), halves at 8 and falls to about a quarter at 16, as its 4 CPUs are shared. A sustained
race-instrumented loop therefore runs on the Windows laptop at about a quarter of the speed of the same loop's first three
seconds; the 56 MiB tests alone show that shape (17 to 18 MiB/s through about 48 MiB, then 6 MiB/s). Whether it is the efficiency
cores, a power limit or something in the process was not tested.

### The four tests serialised

One scratch lock taken by the four bulk tests (and released at "ready"), so that one at a time reads its megabytes; nothing else
changed, and the time spent waiting for the lock is not in the wait.

| Host | Condition | Test | As written | Serialised |
| :--- | :--- | :--- | :--- | :--- |
| Windows laptop | whole package | `TestDetachedTapIsPrefix` | 41.3 / 46.0 (10) | 14.0 / 18.5 (4) |
| Windows laptop | whole package | `TestInProcessDetachedTapIsPrefix` | 37.9 / 41.3 (10) | 10.1 / 10.8 (4) |
| Windows laptop | whole package | `TestBlockedTapDetached` | 36.4 / 41.0 (10) | 11.2 / 14.3 (4) |
| Windows laptop | whole package | `TestTapDrainBoundedByGrace` | 19.2 / 22.0 (10) | 6.8 / 7.5 (4) |
| Linux host | beside the module | `TestDetachedTapIsPrefix` | 25.8 / 32.6 (10) | 17.9 / 24.0 (7) |
| Linux host | beside the module | `TestInProcessDetachedTapIsPrefix` | 24.0 / 27.1 (10) | 14.0 / 20.3 (7) |
| Linux host | beside the module | `TestBlockedTapDetached` | 22.3 / 32.8 (10) | 11.9 / 15.5 (7) |
| Linux host | beside the module | `TestTapDrainBoundedByGrace` | 8.8 / 19.2 (10) | 6.3 / 8.1 (7) |

## What the measurements show

Each statement is tied to the table that holds its rows.

1. **Alone and idle, none of the four waits comes near 30 s on any of the three hosts.** The longest is `TestDetachedTapIsPrefix`
   on the Linux host, 12.3 s of 10 runs; 5.8 s on this Mac and 5.7 s on the Windows laptop (headline, a, d, e). Without `-race`
   the longest is 0.6 s (Linux host).
2. **`-race` costs a factor of 18 to 40 here** (the four tests, three hosts; 19, 27 and 24 for `TestDetachedTapIsPrefix`: 10.6
   against 0.56 s on the Linux host, 4.5 against 0.17 s on the Windows laptop, 5.7 against 0.24 s on this Mac) (headline).
3. **The wait is the connection's per-line JSON work at race speed,** `json.Valid` at 18 to 57 MiB/s and the envelope's `Unmarshal`
   at 10 to 29 MiB/s, 78 to 99 % of `TestBlockedTapDetached`'s wait in every condition; a second CPU, the tap, the write size and the
   pipe do not change it (stages; experiments).
4. **The number of tests running together changes the wait more than the host does.** Linux host, `TestDetachedTapIsPrefix`: 10.6 s
   alone, 25.3 s with four together, 25.8 s beside the module. Windows laptop: 4.5, 13.1 (two), 28.4 (four), 41.3 (package):
   each of two tests together waits longer than the two take one after the other (8.3 s), and the package's 41.3 s is
   three times the four tests' 12.6 s in sequence. This Mac: 5.7, 6.5, 8.7, 11.7 (how many).
5. **On the Windows laptop the package fails three of the four tests every time at today's bound,** 10 of 10 rounds over 30 s for
   the two 56 MiB tests and `TestBlockedTapDetached` (medians 41.3, 37.9 and 36.4 s), none for `TestTapDrainBoundedByGrace`
   (19.2 s), and the same three failed in 3 of 3 rounds with the bounds as written (`W60-pkg-asis`), as phase 4's 3 of 3 rounds
   saw; alone, the same tests wait 2.9 to 4.5 s. `GOMAXPROCS` 4, 8, 16 and 32 changed the package's waits little (medians 32.7 to
   41.3 s for `TestDetachedTapIsPrefix`, two of its three rounds over 30 s at 4) (headline d; `GOMAXPROCS`).
6. **On the Linux host, beside the module, the wait is near the bound:** `TestDetachedTapIsPrefix` 25.8 s median and
   32.6 s at most, over 30 s in 2 of 10 rounds; `TestBlockedTapDetached` 1 of 10. This is what phases 1
   and 2 saw as F7 (one or both over 30 s in 5 of 5 module rounds there), at a lower rate today (2 of 10 rounds with at least
   one of the four over 30 s).
7. **CI does not reach it.** 52 of 52 race steps passed on both OSs, with `agentproc` taking 22.9 to 50.7 s in the whole step
   (ci). The runner's CPU count is not in the logs; the Linux host (4 CPUs) beside the module is the closest condition measured,
   and it is over 30 s in 2 of 10 rounds, so a failure in CI is possible and not seen in 52 jobs.
8. **Under the starved load the wait is about ten times the idle one,** 90 to 116 s at 0.5 MiB/s, in every round (headline, c);
   no test that reads these bytes through the connection passes 30 s there, with the connection's work as it is.
9. **The 5 s grace fails when the connection is slow enough:** what the first step waits for is the connection's read of the
   agent's last 8 MiB, at the rate of the rest: 8 MiB at the starved load's 0.5 MiB/s is 16 s (measured 12 to 18 s, over 5 s in 10
   of 10 runs, and `Close` failed in all 4 runs of c2); at the module's 2.4 MiB/s on the Linux host about 3.3 s (measured 2.8 and
   3.5 s medians, over 5 s in 1 of 20 runs, 5.9 s the longest, in `TestInProcessDetachedTapIsPrefix`); alone, 1.6 s median and 2.3
   s at most on the Linux host. The audit's round 1 failure, at `tap_test.go:349` then, `:372` now, is this wait when the
   readiness passed and the exit did not follow within 5 s.
10. **No defect of the tap was found:** with the bounds lifted, no run failed an assertion of its test (0 of 676 runs of the four
    tests in the rows with the bounds lifted); the tap's
    queue reached its 32 MiB bound and detached as asserted, and its `offer` was 0.2 to 1.6 % of `TestBlockedTapDetached`'s wait
    (stages). The runs that failed had a bound as written: c2's `Close` and `W60-pkg-asis`'s readiness waits.
11. **Serialising the four, one at a time from launch to ready, brought the Windows package's waits to 6.8 to 14.0 s (medians),
    all of 16 runs under 19 s,** against 19.2 to 41.3 s as written (serialised). On the Linux host beside the module (7 runs of
    each test) the medians were 6.3 to 17.9 s, the longest 24.0 s, against 8.8 to 25.8 s and 32.8 s as written.

## What they do not show

- **Samples:** 10 runs alone and 10 rounds of the package or module is a sample; a rate under 1 in 10 is not excluded. The
  starved-load conditions have 5 and 2 rounds, and the Windows `GOMAXPROCS` rows 3 to 6; the group rows have 5.
- **The Windows laptop's cause.** That two or four tests together are slower each than one alone, by more than the CPU count
  would explain on 32 hardware threads, is measured; why is not. Candidates this campaign did not test: the efficiency cores;
  page-fault and commit costs of the race runtime's fresh allocations (the heap grows to 500 MiB and the collector pauses 0.5 to
  1.1 s); locks inside the race runtime; process start costs (1.0 s a child in the package against 0.06 s alone). The in-memory
  connection also ran slower there than the pipeline (9.3 against 4.5 s) and is not explained.
- **CI's own times.** CI prints the package's time, not a test's, and no CPU count: that a CI runner is slower or faster than
  the Linux host was not measured. No run was made on CI.
- **This Mac was not idle** (below), and its numbers are the race gate of record's: three rounds of the package, not ten.
- **The bound's figure.** The phase 1b method (five times the worst measured, with a margin plant and a break) was not run for
  these four bounds: a figure follows only from the owner's decision on the options.
- **Another load.** One starved load (the MADR's F3) and the module as CI's step runs it were measured; no other. The serialised
  variant was run on the Windows package and the Linux module, and not under the starved load (planned, then cut for the
  working budget, as were the starved-load rounds from 10 to 5 and the rounds with the 5 s graces as written from 5 to 2).
- **The failure at today's 30 s in the Linux module rows is the measured time over 30 s,** not a run with the bound as written;
  with the bounds as written, only the Windows package (`W60-pkg-asis`, 3 rounds) and the 5 s graces (c2, d2) were run.
- **FreeBSD, and the Windows laptop under the starved load,** were not run.

## What could follow

The owner decides. Each option says what it would change, what it costs, and what would have to be measured or decided first.

**A. Scale the bounds from measurement, as phase 1b did for the five.** Each of the four 30 s waits, and the two 5 s graces,
gets a figure that follows from the worst time measured over the loads (about five times, rounded up), with its margin plant
(at 0.8 of the figure the test passes, at 1.2 it fails) and its break, in a phase of its own.

- *Changes:* the bounds only; the assertions, the data and the tests' structure stay.
- *Cost:* the figures are large. The worst waits measured are 46.0 s (the Windows package, idle), 32.8 s (the Linux host beside
  the module) and 116 s (the starved load); five times is 230 s, 165 s and about 560 s, so a hung agent takes minutes to
  fail in four tests, and the starved load decides the figure by itself: left out, the figure is about 230 s. The 5 s
  graces would go to about 30 s (five times the first) or 90 s (five times the second), from 5.9 s, the worst first step outside
  the starved load, and 17.9 s in it.
- *First:* the owner's word on whether the starved load of the MADR's F3 is a condition these tests must pass; the margin plants
  and breaks, which were not run here; a figure for each of the hosts.

**B. Take the four bulk tests one at a time.** A test-only lock, in `agentproc`'s test files, taken from launch to "ready" by
`TestDetachedTapIsPrefix`, `TestInProcessDetachedTapIsPrefix`, `TestBlockedTapDetached` and `TestTapDrainBoundedByGrace`.

- *Changes:* when the four read their megabytes; their assertions, bounds and data are as they are, and the 30 s bound keeps its
  meaning (a wait for one test's own 56 MiB).
- *Measured here, as a scratch lock:* the Windows package's waits fell to 6.8 to 14.0 s (medians, 18.5 s at most in 16 runs,
  against 19.2 to 41.3 s as written); on the Linux host beside the module the medians were 6.3 to 17.9 s and the longest 24.0 s (7
  rounds), against 8.8 to 25.8 s and 32.8 s as written. The package's own time grew by about 3 s a round on the Windows laptop.
- *Cost:* a small test-only change and its own proof (a plant that fails it without the lock, and the 20-round confirmation of
  phase 4 on the Linux host and the laptop); it is not shown to help under the starved load, where the four readers are 4 of 36
  runnable goroutines on 2 CPUs and each of them reads at about a tenth of its idle rate (the starved load was not run serialised).
- *First:* the owner's word on the starved load, as for A; a phase that measures the serialised tests under it.

**C. Less data under `-race`.** The 56 MiB exist to pass `tapQueueBytes`, 32 MiB (`agentproc/tap.go`). A package variable in its
place, which the tests lower under `-race`, would run the same assertions on an eighth of the bytes: the wait is linear in the
bytes (the 24 MiB drain waits 0.42 of the 56 MiB test's time on the Linux host, 24/56 being 0.43), so about 1.3 s idle on the
Linux host and about 13 s under the starved load.

- *Changes:* product code (a constant becomes a variable) and the tests, so it is outside 0057's scope as the plan states it; the
  real 32 MiB would be exercised without `-race` only.
- *Cost:* an amendment of the decision that fixed 32 MiB (0030-MADR-record-initialize-on-arrival.md, the amendment of
  2026-10-21, questions 9 and 10), a seam in the product, and the loss of the real size under `-race`; not measured here beyond the linearity.
- *First:* the owner's decision that a test-only seam in `agentproc` is acceptable.

**D. Keep the tests as they are,** and the race gate of record on this Mac, where they pass (the plan's "Where the race gates run").

- *Changes:* nothing.
- *Cost:* the Linux host's and the Windows laptop's race runs of `agentproc` keep failing these tests when loaded: the Windows
  package in 10 of 10 rounds, the Linux host beside the module in 2 of 10 rounds, each recorded and counted apart as
  the plan does. CI is not affected in 52 of 52 jobs; its runners' CPU count is not known, so a failure there is possible, not seen.
- *First:* nothing.

**E. A record of its own about the connection's per-line scanning.** The measurements show no defect of the tap; they show that
`Connection.receive` scans each notification line three times under `-race` (`json.Valid`; `json.Unmarshal`'s validity pass; its
decode) at 6 to 17 MiB/s together, the whole of the wait, and at about 270 to 1,100 MiB/s without it.

- *Changes:* product code, if the owner wants fewer scans; no test.
- *Cost:* a product change for a test-time cost that only the race detector makes, and a decision of its own; not
  recommended for this purpose. It is listed because the plan says a defect the measurements find is left for a record of its
  own, and this is the nearest thing to one.
- *First:* a measurement of what removing a scan buys under `-race`, which was not made.

**Recommendation: B**, then A only if the owner wants the starved load covered. B is the only option that is test-only, keeps
the 30 s bound meaningful and the data size as the MADR chose it, and attacks the factor the measurements move most: how many
of the four read at once. Its measured effect is large on the Windows laptop (the waits fell to a third) and smaller on the Linux host beside the
module (the longest wait from 32.8 s to 24.0 s: under 30 s in 7 of 7 rounds, against 2 of 10 rounds with a test over). It does not
help under the starved load, which is a synthetic load of the MADR's F3 and not a CI condition (the CI race step passed 52 of 52
jobs), and A is the answer to that load if the owner wants one. D is the cheapest and leaves two hosts failing a known set of tests.

## What distorted a measurement, and the odd runs

Recorded, not replaced; none stopped the phase.

1. **Two Windows package chunks held the scratch experiments.** `W03-pkg` and `W04-pkg` (7 rounds) ran with the scratch
   experiment tests (`TestZZ*`, 56 MiB each, CPU-heavy) in the package, running beside the package's own tests: I had not made
   them skip unless asked. Their rows are marked "contaminated" and kept; the clean rounds are `W05-pkg`, `W09-pkg` and `W31-pkg`
   (3, 4 and 3 rounds). The contaminated rounds' medians (39.7, 38.8, 36.0 and 20.1 s) are within 2 s of the clean ones'
   (41.3, 37.9, 36.4, 19.2 s), so the experiments did not move the figure much.
2. **One chunk dispatched to the Windows laptop ran on this Mac.** `W12-pkg-gmp16`: `run-on-host`'s copy check failed ("target has
   854 files, cwd has 855": a Mac chunk of mine was writing its output file into the shipped tree), and it ran the command
   locally under the local `run-heavy`, as its own output says; the go version line in the output is `darwin/arm64` and so are
   the records. Its three rounds are this Mac's whole package at `GOMAXPROCS=16` (11.1 to 11.8 s medians) and are relabelled so;
   the Windows `GOMAXPROCS=16` rows are `W30` and `W32`. No other chunk's records disagree with the host it was sent to
   (checked for every chunk).
3. **A chunk's exit status 2 is mine.** `M02-alone-norace` ended with the shell's syntax error, after all 40 runs had finished and
   passed: I replaced the script it was running, in place. Its records were read from the file the chunk wrote in the scratch
   tree.
4. **The serialised variant's first Linux module chunk failed three root tests.** `L40-serial-mod` ran from a copy that had
   dropped the `testdata` `*.jsonl` fixtures (my ignore pattern), so the root package's `TestToolCallTimelineReplay` and
   `TestCompactionTimelineReplay` failed in each round at once; `agentproc` was not affected, and the root package's own time (29
   to 33 s against 31 to 41 s) shows no difference in load. Kept; `L42-serial-mod` repeated it with the fixtures.
5. **This Mac was not idle.** The load average at the first chunk's start was 19.5, falling to 3.6; at the second's, 11.7 to 13.8
   falling to 7.5 (other sessions' work). The tests' waits did not follow it: the ten runs of `TestDetachedTapIsPrefix` waited
   5.70 to 5.81 s while it fell, and a run at load 31 in the first trial took 5.62 s.
6. **The Windows laptop slows a sustained race-instrumented loop to a quarter of its first seconds' speed,** after about 3 s:
   the probe ("A sustained-load probe") ran at 244 units a second for three seconds and 63 after; two to sixteen goroutines
   from the start ran at 61 to 65 each. The 56 MiB tests alone show the same shape (17 to 18 MiB/s for the first 48 MiB, about 3 s,
   then 6 MiB/s; `TestDetachedTapIsPrefix` and `TestInProcessDetachedTapIsPrefix` in 10 of 10 runs each), so their "alone" figures
   on that host mix two speeds. The Linux host and this Mac do not show it. Why is not known here.
7. **Queue waits.** The Linux host's queue was shared with another session's heavy runs; one chunk of mine waited 402 s. The queue
   serialises heavy runs, so none of mine overlapped another's on a host; a chunk's "seconds" in the table include the wait.
8. **The stamps' own cost was not isolated.** They are a few `time.Now` calls per 64 KiB read; the module rounds with them took 82
   to 127 s on the Linux host against 110 to 114 s for the plan's phase 4's unstamped root module, which does not show a difference.
9. **Lifted bounds.** With the bounds lifted no run timed out and none failed (0 of 676 runs of the four tests in the rows with
   the bounds lifted); the failures "at today's bound"
   are the runs whose time is over 30 s (or over 5 s for a first step), by the measured time, as phase 1b read them. Three sets
   ran with a bound as written, so that a failure is seen and not inferred: `W60-pkg-asis` (Windows laptop,
   readiness at 30 s, 5 s graces as written, 3 rounds: each round failed `TestBlockedTapDetached` (`tap_test.go:317` at
   `b050c9f`), `TestDetachedTapIsPrefix` (`:358`) and `TestInProcessDetachedTapIsPrefix` (`inprocess_test.go:468`) with the
   readiness messages, after 34.0 to 39.1 s, and no other test; `TestTapDrainBoundedByGrace` passed in each: 3 of 3 rounds, as
   phase 4 saw) and c2 and d2 (the readiness bound lifted, the 5 s graces as written).

## The chunks

Every chunk is one dispatch. "Seconds" is the dispatcher's, from its start to the end, a queue wait included. The seed of each
package or module round is in the chunk's output. A chunk whose Records column is larger than its runs holds the stamps of every
test in the package.

| Chunk | Host (records say) | What | `-race` | Bounds | Start (UTC) | Seconds | Exit | Records | Shuffle seeds (a round each) |
| :--- | :--- | :--- | :---: | :---: | :--- | ---: | ---: | ---: | :--- |
| `L01-alone-race` | Linux host | each test alone, one process a run, 10 runs of 4 tests | yes | lifted | 01:28:00 | 551 | 0 | 40 | - |
| `M01-alone-race` | this Mac | each test alone, one process a run, 10 runs of 4 tests | yes | lifted | 01:28:00 | 341 | 0 | 40 | - |
| `W01-alone-race` | Windows laptop | each test alone, one process a run, 10 runs of 4 tests | yes | lifted | 01:28:00 | 321 | 0 | 40 | - |
| `W02-alone-norace` | Windows laptop | each test alone, one process a run, 10 runs of 4 tests | no | lifted | 01:33:21 | 105 | 0 | 40 | - |
| `M02-alone-norace` | this Mac | each test alone, one process a run, 10 runs of 4 tests | no | lifted | 01:33:42 | 544 | 2 | 40 | - |
| `W03-pkg` | Windows laptop | whole package, 4 rounds | yes | lifted | 01:35:07 | 313 | 0 | 84 | 1794884702637955,1794884778638184,1794884853638410,1794884931638645 |
| `L02-alone-norace` | Linux host | each test alone, one process a run, 10 runs of 4 tests | no | lifted | 01:37:11 | 118 | 0 | 40 | - |
| `L03-x` | Linux host | experiments, 5 rounds | yes | lifted | 01:39:09 | 377 | 0 | 50 | - |
| `W04-pkg` | Windows laptop | whole package, 3 rounds | yes | lifted | 01:40:20 | 233 | 0 | 63 | 1794885014638891,1794885088639114,1794885161639334 |
| `M03-x` | this Mac | experiments, 5 rounds | yes | lifted | 01:42:45 | 512 | 0 | 50 | - |
| `W05-pkg` | Windows laptop | whole package, 3 rounds | yes | lifted | 01:44:14 | 147 | 0 | 48 | 1794885249639596,1794885292639726,1794885337639862 |
| `L04-gmp` | Linux host | alone, GOMAXPROCS sweep, 5 runs, GOMAXPROCS 1,2 | yes | lifted | 01:45:27 | 318 | 0 | 20 | - |
| `W06-x` | Windows laptop | experiments, 5 rounds | yes | lifted | 01:46:41 | 234 | 0 | 50 | - |
| `W07-gmp` | Windows laptop | alone, GOMAXPROCS sweep, 5 runs, GOMAXPROCS 4,8 | yes | lifted | 01:50:34 | 169 | 0 | 20 | - |
| `L05-mod` | Linux host | the module, `go test -race -shuffle=SEED ./...`, rounds 1 to 4 | yes | lifted | 01:50:45 | 382 | 0 | 72 | 1794885634640751,1794885743641079,1794885826641329,1794885914641594 |
| `M04-pkg` | this Mac | whole package, 3 rounds | yes | lifted | 01:51:17 | 52 | 0 | 54 | 1794885662640835,1794885678640884,1794885694640933 |
| `W08-pkg2` | Windows laptop | whole package, 5 rounds | yes | readiness lifted, 5 s graces as written | 01:53:24 | 237 | 0 | 80 | 1794885798641243,1794885841641373,1794885885641506,1794885930641642,1794885976641781 |
| `L06-mod` | Linux host | the module, `go test -race -shuffle=SEED ./...`, rounds 5 to 8 | yes | lifted | 01:57:07 | 408 | 0 | 72 | 1794886016641901,1794886143642283,1794886242642581,1794886336642864 |
| `W09-pkg` | Windows laptop | whole package, 4 rounds | yes | lifted | 01:57:24 | 212 | 0 | 64 | 1794886038641963,1794886089642117,1794886139642268,1794886192642428 |
| `W10-pkg-gmp4` | Windows laptop | whole package, 3 rounds, GOMAXPROCS=4 | yes | lifted | 02:00:56 | 200 | 0 | 48 | 1794886251642602,1794886306642768,1794886370642961 |
| `L07-mod` | Linux host | the module, `go test -race -shuffle=SEED ./...`, rounds 9 to 10 | yes | lifted | 02:03:55 | 206 | 0 | 36 | 1794886424643129,1794886536643466 |
| `W11-pkg-gmp8` | Windows laptop | whole package, 3 rounds, GOMAXPROCS=8 | yes | lifted | 02:04:17 | 158 | 0 | 48 | 1794886451643202,1794886500643350,1794886548643495 |
| `L20-grp` | Linux host | tests selected together, 5 runs, four | yes | lifted | 02:05:18 | 288 | 0 | 20 | - |
| `M20-grp` | this Mac | tests selected together, 5 runs, four | yes | lifted | 02:05:18 | 94 | 0 | 20 | - |
| `W20-grp` | Windows laptop | tests selected together, 5 runs, four | yes | lifted | 02:05:18 | 266 | 0 | 20 | - |
| `M21-grp` | this Mac | tests selected together, 5 runs, two | yes | lifted | 02:06:52 | 47 | 0 | 10 | - |
| `W12-pkg-gmp16` | Windows laptop, but records say darwin | whole package, 3 rounds, GOMAXPROCS=16 | yes | lifted | 02:06:54 | 94 | 0 | 54 | 1794886645643784,1794886661643833,1794886677643882 |
| `L08-burn` | Linux host | whole package, 3 rounds, starved load | yes | lifted | 02:07:20 | 1924 | 0 | 54 | 1794887041644972,1794887616646698,1794888085648106 |
| `W13-pkg-norace` | Windows laptop | whole package, 3 rounds | no | lifted | 02:08:28 | 124 | 0 | 48 | 1794886768644153,1794886785644205,1794886801644254 |
| `W21-grp` | Windows laptop | tests selected together, 5 runs, two | yes | lifted | 02:09:44 | 146 | 0 | 10 | - |
| `L21-grp` | Linux host | tests selected together, 5 runs, two | yes | lifted | 02:10:06 | 1880 | 0 | 10 | - |
| `W30-pkg-gmp16` | Windows laptop | whole package, 3 rounds, GOMAXPROCS=16 | yes | lifted | 02:12:49 | 158 | 0 | 48 | 1794886964644741,1794887014644892,1794887062645037 |
| `W31-pkg` | Windows laptop | whole package, 3 rounds | yes | lifted | 02:15:28 | 284 | 0 | 48 | 1794887246645587,1794887297645741,1794887348645895 |
| `L40-serial-mod` | Linux host | the module, `go test -race -shuffle=SEED ./...`, rounds 1 to 3 | yes | lifted | 02:17:16 | 1722 | 0 | 54 | 1794888670649859,1794888773650169,1794888856650419 |
| `W40-serial-pkg` | Windows laptop | whole package, 4 rounds | yes | lifted | 02:17:16 | 374 | 0 | 64 | 1794887399646046,1794887448646194,1794887496646339,1794887544646484 |
| `W32-pkg-gmp16` | Windows laptop | whole package, 3 rounds, GOMAXPROCS=16 | yes | lifted | 02:20:12 | 348 | 0 | 48 | 1794887597646640,1794887645646785,1794887696646939 |
| `L09-burn` | Linux host | whole package, 2 rounds, starved load | yes | lifted | 02:22:42 | 2431 | 0 | 36 | 1794888954650711,1794889437652161 |
| `W33-grp-gmp16` | Windows laptop | tests selected together, 5 runs, four-gmp16 | yes | lifted | 02:26:00 | 171 | 0 | 20 | - |
| `L50-spin` | Linux host | sustained-load probe | yes | lifted | 02:43:24 | 1325 | 0 | 15 | - |
| `M50-spin` | this Mac | sustained-load probe | yes | lifted | 02:43:24 | 129 | 0 | 15 | - |
| `W50-spin` | Windows laptop | sustained-load probe | yes | lifted | 02:43:24 | 138 | 0 | 15 | - |
| `W60-pkg-asis` | Windows laptop | whole package, 3 rounds | yes | as written | 02:46:28 | 138 | 0 | 48 | 1794888983650798,1794889025650925,1794889065651046 |
| `L42-serial-mod` | Linux host | the module, `go test -race -shuffle=SEED ./...`, rounds 4 to 7 | yes | lifted | 02:46:55 | 1511 | 0 | 72 | 1794890113654191,1794890236654561,1794890327654835,1794890420655115 |
| `L11-burn2` | Linux host | whole package, 2 rounds, starved load | yes | readiness lifted, 5 s graces as written | 03:03:24 | 1780 | 0 | 36 | 1794890759656126,1794891293657729 |

## Files

Touched: this report, `docs/README.md` (its row) and `docs/decisions/0057-PLAN-agentproc-test-graces-on-a-loaded-runner.md` (the
execution record's entry "phase 5 (the report)"). Not touched: any source, test, workflow or tool. Scratch only, not staged: the
stamped copy of the tree and its patch script, the host-side runner, the chunk and chain scripts, the table generators and every
chunk's output.
