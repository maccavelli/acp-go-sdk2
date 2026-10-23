# Windows floor CI: two runtime crashes in the root package's tests, not reproduced

Observed 2026-10-22, at the owner's request, after CI run `39803032026` failed on `1fc1d21`, a commit of two Markdown records. This
report sets down what is known of the two crashes CI has met on its Windows floor job, what was checked, and what is still open. It
decides nothing. The repository was only read; the figures come from GitHub's API and job logs, with gh 2.102.0, and from runs on the
Windows host reached over SSH.

## Summary

- Twice, the Windows job at the Go floor (1.26.x, with `GOEXPERIMENT=goroutineleakprofile`) failed in the root package's tests with a
  crash inside the Go runtime or the standard library, not a test's failure: once "fatal error: invalid g status" in the garbage
  collector, once "panic: reflect: Field index out of bounds" inside `encoding/json`. Neither commit changed code.
- Of the 25 most recent runs on `main` with the six-job matrix, the Windows floor job failed 2 times on its first attempt; the other five
  jobs, the Linux and macOS floor jobs with the same experiment among them, never failed (table below). Today's failed job passed when
  rerun.
- 100 runs of the same test binary, 50 with the experiment and 50 without, on the Windows host at go1.26.10, did not crash.
- No Go issue matches either crash. The cause is not known; what the evidence does and does not support is in "What the evidence
  supports".

## The two crashes

| | 2026-10-20 | 2026-10-22 |
| :--- | :--- | :--- |
| Run, attempt | `39477545696`, 1 | `39803032026`, 1 |
| Commit | `2965021`, "docs(decisions): record v1.17.1 publication verification" | `1fc1d21`, "docs(plan): mark 0031 verification tools PLAN complete" |
| What the commit changed | records only | `docs/README.md` and a PLAN only |
| Job | `validate (windows-2025, 1.26.9)` | `validate (windows-2025, 1.26.10)` |
| Environment | `GOTOOLCHAIN=local`, `GOEXPERIMENT=goroutineleakprofile` | the same |
| Step | `go test -shuffle=on ./...` | the same |
| Shuffle seed | `1792521974776092100` | `1792682148899561700` |
| Package | the root, `FAIL github.com/maccavelli/acp-go-sdk2 8.774s` | the root, `FAIL github.com/maccavelli/acp-go-sdk2 9.803s` |
| Other packages | passed | passed, `agentproc` among them |
| The rerun | not rerun | attempt 2, started by the repository's owner at 16:30:33Z: the Windows floor job passed, all six jobs passed |

**2026-10-20: "fatal error: invalid g status".** Two of the collector's mark workers threw at once:

```text
runtime:   gp: gp=0x1af84b1b2960, goid=0, gp->atomicstatus=0
runtime: getg:  g=0x1af84b116f00, goid=0,  g->atomicstatus=0
fatal error: invalid g status
runtime:   gp: gp=0x1af84b1b2b40, goid=0, gp->atomicstatus=0
runtime: getg:  g=0x1af84b5d92c0, goid=0,  g->atomicstatus=0
fatal error: invalid g status

runtime stack:
runtime.throw({0x7ff6c084d358?, 0x1af84b3ee000?})
        C:/hostedtoolcache/windows/go/1.26.9/x64/src/runtime/panic.go:1229
runtime.suspendG(0x1af84b1b2960)
        C:/hostedtoolcache/windows/go/1.26.9/x64/src/runtime/preempt.go:136
runtime.markroot.func1()
        C:/hostedtoolcache/windows/go/1.26.9/x64/src/runtime/mgcmark.go:298
runtime.markroot(0x1af84b10f260, 0x14c, 0x1)
        C:/hostedtoolcache/windows/go/1.26.9/x64/src/runtime/mgcmark.go:280
runtime.gcDrain(0x1af84b10f260, 0x7)
        C:/hostedtoolcache/windows/go/1.26.9/x64/src/runtime/mgcmark.go:1284
runtime.gcDrainMarkWorkerIdle(...)
        C:/hostedtoolcache/windows/go/1.26.9/x64/src/runtime/mgcmark.go:1177
runtime.gcBgMarkWorker.func2()
        C:/hostedtoolcache/windows/go/1.26.9/x64/src/runtime/mgc.go:1881
```

The collector, scanning the goroutines as roots, met two with goid 0 and status 0, `_Gidle`, which `suspendG` does not accept: the
runtime's own record of a goroutine was in a state the collector should not find.

**2026-10-22: "panic: reflect: Field index out of bounds".** On the connection's reader goroutine:

```text
panic: reflect: Field index out of bounds [recovered, repanicked]

goroutine 8983 [running]:
sync.(*WaitGroup).Go.func1.1()
        C:/hostedtoolcache/windows/go/1.26.10/x64/src/sync/waitgroup.go:251
reflect.(*structType).Field(0x21c568b09e0?, 0x30?)
        C:/hostedtoolcache/windows/go/1.26.10/x64/src/reflect/type.go:1185
reflect.(*rtype).Field(0x299cf99cf688?, 0x299cf99cf7f0?)
        C:/hostedtoolcache/windows/go/1.26.10/x64/src/reflect/type.go:777
encoding/json.(*decodeState).object(...)
        C:/hostedtoolcache/windows/go/1.26.10/x64/src/encoding/json/decode.go:732
encoding/json.(*decodeState).value(...)
        C:/hostedtoolcache/windows/go/1.26.10/x64/src/encoding/json/decode.go:380
encoding/json.(*decodeState).unmarshal(...)
        C:/hostedtoolcache/windows/go/1.26.10/x64/src/encoding/json/decode.go:183
encoding/json.Unmarshal({0x299cf8ec1a80, 0x72, 0x80}, {0x7ff6e38d2220, 0x299cf99f5cb0})
        C:/hostedtoolcache/windows/go/1.26.10/x64/src/encoding/json/decode.go:113
github.com/maccavelli/acp-go-sdk2.(*Connection).receiveLine(0x299cf8d1d340, {0x299cf8ec1a80, 0x72, 0x80}, 0x299cf99cfe40)
        D:/a/acp-go-sdk2/acp-go-sdk2/connection.go:1295
github.com/maccavelli/acp-go-sdk2.(*Connection).receive(0x299cf8d1d340)
        D:/a/acp-go-sdk2/acp-go-sdk2/connection.go:1166
```

`connection.go:1288`–`:1295` (at `1fc1d21`) has, by then, accepted the line with `json.Valid` and trimmed it, and decodes it into
`env`, a local `envelope`. `encoding/json` asked `reflect` for a field of the struct by an index the struct type does not have: the
field index it held, or the type it was looking at, was wrong. A 114-byte valid JSON line decoded into a local struct does not do this
when the program's memory is intact.

**Common to both.** Each log shows the same tests just before the crash: the malformed-input answers of `jsonrpc_test.go` ("method must
be a string", "jsonrpc must be \"2.0\"", "method is missing", "params must be an object, an array or null", ...), the notification
to `nope/x`, and the `initialize` result refused for `auth.terminal`. The shuffle seeds differ, so the order differs; that both crashes
came within the same stretch of the run, about 8 to 10 seconds in, is noted and not explained.

## Where it happens

Every job's first attempt, of the 40 most recent runs on `main` (2026-10-18 to 2026-10-22). Fifteen of the 40, the oldest, predate the
floor jobs and ran one job per system; the 25 with the six-job matrix:

| Job | `GOEXPERIMENT` | Jobs | Success | Failure | Cancelled |
| :--- | :--- | :--- | :--- | :--- | :--- |
| ubuntu-24.04, floor 1.26.x | `goroutineleakprofile` | 25 | 25 | 0 | 0 |
| macos-15, floor 1.26.x | `goroutineleakprofile` | 25 | 22 | 0 | 3 |
| windows-2025, floor 1.26.x | `goroutineleakprofile` | 25 | 23 | **2** | 0 |
| ubuntu-24.04, newest 1.27.x | none | 25 | 25 | 0 | 0 |
| macos-15, newest 1.27.x | none | 25 | 23 | 0 | 2 |
| windows-2025, newest 1.27.x | none | 25 | 25 | 0 | 0 |

The five cancelled macOS jobs have no steps: they were cancelled while queued (`39635999722`'s two, queued at 16:34:35Z, ended at
16:49:36Z), and ran no test. `.github/workflows/ci.yml:27`–`:39` sets `GOEXPERIMENT` for the whole job on every floor job, so the Linux
and macOS floor jobs run the same experiment as the Windows one; the race detector step runs on both Linux jobs
(`ci.yml:82`–`:87`), at the floor too, and passed in all 50.

## What was checked

**The Go issue tracker.** `gh search issues --repo golang/go` for "goroutineleak", "goroutineleakprofile", "invalid g status
goroutineleak" and "Field index out of bounds reflect", and two web searches: no issue matches either crash. The nearest:

- [google/gvisor#15412](https://github.com/google/gvisor/issues/15412), open: on Go 1.27.1, "fatal error: bad g->status in ready" when
  the goroutineleak profile is collected while a goroutine is parked, and the goroutine is then woken through gVisor's `sync.Goready`, a
  link into the runtime. A corrupted goroutine status, like the 2026-10-20 crash, but its trigger is not this package's: the root
  package reads the profile only in `TestMain`, after `m.Run()` has returned (`leak_test.go:15`–`:31`), and the 2026-10-22 crash came
  during the tests.
- [golang/go#76540](https://github.com/golang/go/issues/76540), [#76749](https://github.com/golang/go/issues/76749) and
  [#81136](https://github.com/golang/go/issues/81136), closed: failures of the runtime's own concurrency tests of the profile; none is
  either crash, none Windows.
- [golang/go#74609](https://github.com/golang/go/issues/74609), the feature's proposal: a goroutine leak profile request runs a
  special garbage collection cycle that marks the leaked goroutines.
- [golang/go#80311](https://github.com/golang/go/issues/80311), open: "runtime/pprof: Profile.Count is permanently stale for
  goroutineleak". Not a crash; but the leak guard reads `p.Count()` after `WriteTo` (`leak_test.go:44`–`:55`), and whether it counts
  correctly is not examined here.

**A reproduction on the Windows host.** The root package's test binary, built at go1.26.10 for windows/amd64 with and without
`GOEXPERIMENT=goroutineleakprofile`, run from a copy of the tree, 25 rounds of two copies at once, `-test.shuffle=on`. A run counted as
a crash when its output held "fatal error:", "invalid g status", "reflect: Field index out of bounds" or another top-level panic. Before
the rounds, one plain run of each passed.

| Binary | Runs | Passed | Crashed | Exit status |
| :--- | :--- | :--- | :--- | :--- |
| with the experiment | 50 | 50 | 0 | 0 each |
| without it | 50 | 50 | 0 | 1 each: `TestMain` exits 1 after the tests when the profile is missing, by design |

Two attempts before it were not valid, and their results are not counted: the first ran no test (its shell command started the second
copy outside the test directory, exit 127); the second ran the binaries alone, without the tree, and every run panicked opening
`schema\schema.json`.

## What the evidence supports

- **Supported:** the crashes are in the runtime or the standard library, in a state that a program's own logic cannot reach while its
  memory is intact: a goroutine `_Gidle` during root marking, and a struct field index out of range for its own type.
- **Supported:** they are not caused by the commits, which changed no code; and they are rare, 2 of 25 Windows floor jobs, gone on a
  rerun.
- **Supported:** they are confined, so far, to Windows at 1.26.x with the experiment: 0 of 25 on Linux and 0 of 22 run on macOS at the
  floor with the same experiment, 0 of the 73 run at 1.27.x without it.
- **Not decided:** whether the experiment causes them. On Windows every 1.26.x job runs with it, so Windows at 1.26.x and the
  experiment on Windows cannot be told apart from CI's history. The 50 clean runs with it on the Windows host do not decide it either:
  if each run there crashed as often as CI's 2 in 25, 8%, 50 clean runs would happen 1.6% of the time, which suggests the host's
  conditions differ from the runners' in a way that matters; but 2 in 25 is a rough rate, its exact 95% interval 1.0% to 26.0%, and at
  1% 50 clean runs happen 60.5% of the time (`(1 - p)^50`, and the Clopper-Pearson interval, computed that day). A data race in this
  package is not excluded either, though the race detector, on Linux, passed in every run of both Linux jobs.

## Open

None of these was done; each is the owner's to decide, and any change to CI or the code takes a record of its own.

- **More from the next crash:** `GOTRACEBACK=crash` on the Windows floor job prints every goroutine's stack, not only the panicking
  one's.
- **Separating the experiment from Windows at 1.26:** a Windows floor job without the experiment, the leak guard there given a way to
  stand aside, would show whether the crashes stop; it weakens one check on one job.
- **Upstream:** the two traces filed with golang/go, the owner's to send.
- **The leak guard's count:** [golang/go#80311](https://github.com/golang/go/issues/80311), above.

## If it happens again

Add a row to "The two crashes" and to "Where it happens", from:

- `python3 tools/watch_ci.py <commit>`: the run and each job, once settled;
- ~~`gh run view <run> --job <job> --log-failed`: the step, the seed, the trace;~~ (Amended 2026-10-22: that command reads the
  run's latest attempt whatever the job's id, so once the failed job has been rerun it shows the rerun.
  `python3 tools/ci_jobs.py failure <run>` reads every attempt's failed jobs by their ids: the step, the seed, the crash. See
  [0031-MADR-verification-tools-in-repository.md](../decisions/0031-MADR-verification-tools-in-repository.md), "Amendment 2026-10-22, after step 11".)
- the tally above: every job's first attempt, by system and Go version, from the runs' `attempts/1/jobs`. (Amended 2026-10-22:
  `python3 tools/ci_jobs.py tally --markdown`.)

A crash on another job, at 1.27.x, or without the experiment, would change "What the evidence supports" above.

## Addendum, 2026-10-22: the owner's answers, and the leak guard's count

The owner answered "Open" the same day:

- **More from the next crash:** done by [0033-MADR-traceback-on-windows-floor-ci.md](../decisions/0033-MADR-traceback-on-windows-floor-ci.md),
  `GOTRACEBACK=crash` on the Windows floor job alone.
- **Separating the experiment from Windows at 1.26:** not now; the leak guard stays on every floor job.
- **Upstream:** not now.
- **The leak guard's count:** not affected by [golang/go#80311](https://github.com/golang/go/issues/80311). A probe that leaks goroutines,
  at go1.27.3 and at go1.26.10 with the experiment, showed `Count` stale before a `WriteTo`, and equal to the profile just written after
  one (3, then 5); the guard calls `WriteTo` (`leak_test.go:49`) before every `Count` (`:54`).
  [0033-PLAN-traceback-on-windows-floor-ci.md](../decisions/0033-PLAN-traceback-on-windows-floor-ci.md) holds the table.

## Addendum, 2026-10-22: a third crash, at 1.27.3 without the experiment

A third crash came the same day, on the job "What the evidence supports" said would change it: Windows at the newest Go, without the
experiment. Added as "If it happens again" says.

| | 2026-10-22, later |
| :--- | :--- |
| Run, attempt | `39841121410`, 1 |
| Commit | `2f58588`, "chore(tools): route every verification tool through main_guard"; the run also carried `57ce3f2` and `a716dfe`, which change `ci.yml` and records; none changes a Go file of the module |
| Job | `validate (windows-2025, 1.27.3)` |
| Environment | `GOTOOLCHAIN=local`; `GOEXPERIMENT` and `GOTRACEBACK` empty |
| Step | `go test -shuffle=on ./...` |
| Shuffle seed | `1792697648940304500` |
| Package | the root |
| The other five jobs | passed, the Windows floor job, at 1.26.10 with the experiment, among them |
| The rerun | attempt 2, at the owner's request, started 19:46:00Z: the job passed, all six jobs passed |

**"fatal error: found pointer to free object".** The runtime's own report, before the stack:

```text
runtime: marked free object in span 0x23ade95dc90, elemsize=80 freeindex=13 (bad use of unsafe.Pointer or having race conditions? try -d=checkptr or -race)
...
fatal error: found pointer to free object

runtime stack:
runtime.throw({0x7ff625acd81b?, 0x68ad5ff150?})
        C:/hostedtoolcache/windows/go/1.27.3/x64/src/runtime/panic.go:1243
runtime.(*mspan).reportZombies(0x23ade95dc90)
        C:/hostedtoolcache/windows/go/1.27.3/x64/src/runtime/mgcsweep.go:893
runtime.(*sweepLocked).sweep(0x7ff62568420e?, 0x0)
        C:/hostedtoolcache/windows/go/1.27.3/x64/src/runtime/mgcsweep.go:673
runtime.(*mcentral).uncacheSpan(0x7ff62563f3fa?, 0x68ad5ff2d8?)
        C:/hostedtoolcache/windows/go/1.27.3/x64/src/runtime/mcentral.go:237
```

(Corrected 2026-10-22: the first line above read "try -d=checkptr)" when this addendum was written, dropping the log's "or
-race"; it now reads as the log does, by `python3 tools/ci_jobs.py failure 39841121410`. See
[0031-MADR-verification-tools-in-repository.md](../decisions/0031-MADR-verification-tools-in-repository.md), "Amendment 2026-10-22, after step 11".)

The collector, sweeping, found an 80-byte object it had freed still marked reachable: a pointer to freed memory. As a failure inside the
runtime, it printed every goroutine (the log's "goroutine 1 ... [chan receive]" and the rest), with `GOTRACEBACK` unset.

**What the root package does that the runtime's hint names.** No `.go` file at the root, the tests included, uses `unsafe`,
`go:linkname`, `syscall`, `reflect.SliceHeader` or `reflect.StringHeader` (`tools/find.py`, no line found). The module requires no
package but `golang.org/x/sys`, which only `agentproc` imports, on Windows.

**The race detector on Windows.** CI runs it on Linux alone (`ci.yml:82`–`:87`). On the Windows host, go1.27.3 with MinGW-W64 gcc 16.2.0
and cgo on: the root package from a copy of the tree, `go test -race -shuffle=on -count=1 .`, 10 runs: all passed, no "WARNING: DATA
RACE", no fatal error. The detector was first seen to work there: a test written into a copy, two goroutines incrementing one variable,
`go test -race -run TestPlantedRace .`, printed "WARNING: DATA RACE" and failed.

**Where it happens, again.** The 40 most recent runs on `main`, 28 of them with the six-job matrix, first attempts:

| Job | `GOEXPERIMENT` | Jobs | Success | Failure | Cancelled |
| :--- | :--- | :--- | :--- | :--- | :--- |
| ubuntu-24.04, floor 1.26.x | `goroutineleakprofile` | 28 | 28 | 0 | 0 |
| macos-15, floor 1.26.x | `goroutineleakprofile` | 28 | 25 | 0 | 3 |
| windows-2025, floor 1.26.x | `goroutineleakprofile` | 28 | 26 | **2** | 0 |
| ubuntu-24.04, newest 1.27.x | none | 28 | 28 | 0 | 0 |
| macos-15, newest 1.27.x | none | 28 | 26 | 0 | 2 |
| windows-2025, newest 1.27.x | none | 28 | 27 | **1** | 0 |

**What changes in "What the evidence supports".**

- **No longer supported:** that the crashes are confined to 1.26.x with the experiment. They are on Windows, 3 of 56 Windows jobs, at both
  Go versions, with and without the experiment; 0 of 56 on Linux, 0 of the 51 run on macOS.
- **Still supported:** each is a corruption the runtime finds, in a state ordinary Go code cannot reach while its memory is intact, in the
  root package's tests, on commits that change no Go file of the module.
- **Weakened, not excluded:** a data race in this package or its tests. The race detector found none in 10 Windows runs or on Linux in
  every run of both Linux jobs; a race the tests reach rarely could still be missed by both.
- **Not decided:** whether the cause is this package, the Go runtime on Windows, or the runners. The experiment is no longer the leading
  suspect.

## Addendum, 2026-10-22: the race detector on Windows

The owner asked for a race step on CI's Windows jobs; [0034-MADR-race-detector-on-windows-ci.md](../decisions/0034-MADR-race-detector-on-windows-ci.md) decided it, on both Windows jobs, and
[0034-PLAN-race-detector-on-windows-ci.md](../decisions/0034-PLAN-race-detector-on-windows-ci.md) carried it out.

- **On the Windows host,** CI's race step, both modules, at go1.27.3 and at go1.26.10 with the experiment: no "WARNING: DATA RACE".
  One test failed under `-race` beside the rest of `agentproc`, `TestTapDrainBoundedByGrace`, 6 of 6 at 1.27.3: not a race, but a
  premise that depended on the machine's speed. It was changed first, and passed 15 of 15 there after.
- **On CI,** the first run with the step, `39877624745` on `616864e`: the step passed on both Windows jobs, 131 s at 1.26.10 and 159 s
  at 1.27.3, in Git for Windows' bash, with `-race` built by the image's MinGW-w64 gcc; every package of both modules "ok", no data race
  warning. Each Windows job took about 275 s, against about 115 s before.

**What changes in "What the evidence supports".** A data race in either module stays "weakened, not excluded"; but from now on every
Windows job, at both versions, looks for one, and a race the tests reach fails the step, where until now only a crash could show it.
Should a crash recur, the race step's result on the same job is part of its row: `python3 tools/ci_jobs.py failure <run>` names the
failing step.
