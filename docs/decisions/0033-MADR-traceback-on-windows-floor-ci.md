---
status: accepted
date: 2026-10-22
decision-makers: Repository owner
---

# Print every goroutine's stack when the Windows floor CI job crashes

## Context and Problem Statement

[0032-REPORT-windows-floor-runtime-crashes.md](../reports/0032-REPORT-windows-floor-runtime-crashes.md) records two crashes on CI's
Windows floor job, `validate (windows-2025, 1.26.x)`, and leaves their cause open. It lists what would help decide it; on 2026-10-22 the
owner chose one of them: that the job's next crash print more. This record decides how.

Every figure below was taken that day; the repository was only read, at `b533de4`.

### What a crash prints today

* **G1. Go's documentation of `GOTRACEBACK`** (`go doc runtime`, go1.27.3): "By default, a failure prints a stack trace for the current
  goroutine, eliding functions internal to the run-time system, and then exits with exit code 2. The failure prints stack traces for all
  goroutines if there is no current goroutine or the failure is internal to the run-time." "GOTRACEBACK=all adds stack traces for all
  user-created goroutines. GOTRACEBACK=system is like 'all' but adds stack frames for run-time functions and shows goroutines created
  internally by the run-time. GOTRACEBACK=crash is like 'system' but crashes in an operating system-specific manner instead of exiting.
  For example, on Unix systems, the crash raises SIGABRT to trigger a core dump. GOTRACEBACK=wer is like 'crash' but doesn't disable
  Windows Error Reporting (WER)."
* **G2. The two crashes, as printed.** The 2026-10-20 crash, "fatal error: invalid g status", was internal to the runtime, and its log
  already holds every goroutine's stack (goroutine 225, the mark worker; goroutine 1, the test runner; and the rest). The 2026-10-22
  crash, "panic: reflect: Field index out of bounds", was a panic on one goroutine, and its log holds that goroutine's stack alone,
  goroutine 8983's. So of the two, the second is the one more output would have told more about.
* **G3. What each setting prints on Windows.** A probe program, built at go1.26.10 for windows/amd64: three goroutines waiting on a
  channel, and a fourth that panics; run on the Windows host reached over SSH, each setting once:

  | `GOTRACEBACK` | Exit | Time | Goroutine headers | The waiting goroutines | Run-time frames |
  | :--- | :--- | :--- | :--- | :--- | :--- |
  | unset | 2 | 0.4 s | 1 | no | no |
  | `all` | 2 | 0.3 s | 5 | yes | no |
  | `system` | 2 | 0.4 s | 9 | yes | yes |
  | `crash` | 2 | 0.4 s | 9 | yes | yes |

  `crash` neither hung nor raised a dialog: as G1 says, it disables Windows Error Reporting; its output and exit status matched `system`'s.
* **G4. An empty value is the default.** The same probe, built for macOS at go1.26.10: `GOTRACEBACK` unset and `GOTRACEBACK=` (empty)
  each printed 1 goroutine header and exited 2. The runtime reads the variable alike on every system.
* **G5. How CI sets a job's environment.** `.github/workflows/ci.yml:27`–`:39`: a matrix of three systems by two Go versions, an
  `include` that adds `floor: true` and `experiment: goroutineleakprofile` to every 1.26.10 job, and a job `env` that sets
  `GOEXPERIMENT: ${{ matrix.experiment }}`, empty on the 1.27.3 jobs. GitHub's documentation of `include` ("Running variations of
  jobs in a workflow"): an entry whose keys match a combination's values adds its other keys to that combination alone, as
  "`{color: pink, animal: cat}` adds `color:pink` only to the original matrix combinations that include `animal: cat`".

## Decision Drivers

* **More of the next crash,** on the job that has crashed, the Windows floor job.
* **No change to what any job tests,** or to how a job passes or fails.
* **Nothing that can hang or wait** on a CI runner.

## Considered Options

* **O1.** `GOTRACEBACK=crash` on the Windows floor job alone.
* **O2.** `GOTRACEBACK=system` on that job alone.
* **O3.** `GOTRACEBACK=all` on that job alone.
* **O4.** `GOTRACEBACK=crash` on every job.
* **O5.** Leave CI as it is.

## Decision Outcome

Chosen option: "O1", decided by the owner on 2026-10-22, because it prints every goroutine's stack, run-time frames included, on the one
job that has crashed, and on Windows ends as `system` does (G3), with Windows Error Reporting off (G1).

* O2 prints the same on Windows (G3); O1 was the owner's choice, and on a Unix job would also leave a core dump, though it runs on none.
* O3 leaves out the run-time's own goroutines and frames, which the 2026-10-20 crash was among.
* O4 adds output to five jobs that have not crashed.
* O5 leaves the next panic with one goroutine's stack (G2).

### Specification, as decided

* **`.github/workflows/ci.yml`:** an `include` entry `{os: windows-2025, go: '1.26.10', traceback: crash}`, and in the job's `env`,
  `GOTRACEBACK: ${{ matrix.traceback }}`: `crash` on the Windows floor job, empty, the default (G4), on the other five. A comment beside
  it cites this record and 0032's report.
* **Nothing else changes:** no test, no `GOEXPERIMENT`, no step.

#### Decided with the owner, 2026-10-22

The owner answered the options 0032's report left open:

| Option | Answer |
| :--- | :--- |
| Print more from the next crash | **This record:** `GOTRACEBACK=crash`, the Windows floor job alone. |
| Run the Windows floor job without the experiment | **Not now:** the leak guard stays on every floor job. |
| Report it to golang/go | **Not now.** |

### Consequences

* Good, because the next crash of the Windows floor job prints every goroutine's stack.
* Neutral, because a crash's log grows by every goroutine's stack; a passing job prints nothing more.
* Neutral, because the cause stays open until a crash prints it, or never recurs.

### Confirmation

The PLAN shows, from a CI run's job logs after the push, that the Windows floor job's steps run with `GOTRACEBACK: crash` and the five
others' with none, by a script first seen to fail on the logs of a run from before the change.

## More Information

* [0032-REPORT-windows-floor-runtime-crashes.md](../reports/0032-REPORT-windows-floor-runtime-crashes.md): the crashes, and the
  options this record answers.
* [0024-MADR-go-sdk-best-practices-on-a-go-1-26-9-floor.md](0024-MADR-go-sdk-best-practices-on-a-go-1-26-9-floor.md): the floor job and
  its experiment.
