---
status: accepted
date: 2026-10-22
decision-makers: Repository owner
---

# Run the race detector on CI's Windows jobs, once the tap drain test no longer depends on the machine's speed

## Context and Problem Statement

[0032-REPORT-windows-floor-runtime-crashes.md](../reports/0032-REPORT-windows-floor-runtime-crashes.md) records three crashes of the
root package's tests on CI, all on Windows, at both Go versions, each a corruption the runtime found. Its third addendum leaves a data
race in the package "weakened, not excluded": CI runs the race detector on Linux alone (`.github/workflows/ci.yml:90`–`:95`, the step
"race detector", `if: runner.os == 'Linux'`), and ten runs of the root package under `-race` on the Windows host found none. After the
third crash the owner chose, on 2026-10-22, to keep recording occurrences and to leave a Windows race step for later; later that day the
owner asked for it: "Add a windows race detector". Neither choice is in a record until this one. This record decides how.

Every figure below was taken that day; the repository was only read, at `b1be5bf`. The Windows host is the one reached over SSH, with
go1.27.3 and MinGW-W64 gcc 16.2.0.

### What was found

* **R1. On Windows, `-race` needs cgo and a C compiler on `PATH`.** `go help build` (go1.27.3): `-race` is "Supported only on
  darwin/amd64, darwin/arm64, freebsd/amd64, linux/amd64, linux/arm64 (only for 48-bit VMA), linux/ppc64le, linux/riscv64 and
  windows/amd64". A one-test module on the Windows host: with `CGO_ENABLED=1`, "ok"; with `CGO_ENABLED=0`, exit 2, "go: -race requires
  cgo; enable cgo by setting CGO_ENABLED=1"; with gcc taken off `PATH`, `go env CGO_ENABLED` still printed 1 and the test failed to
  build, "cgo: C compiler "gcc" not found: exec: "gcc": executable file not found in %PATH%". On macOS the same module passed under
  `-race` with `CGO_ENABLED=0`, so this is Windows's requirement, not Go's everywhere.
* **R2. The runner's image documents a gcc.** actions/runner-images' `Windows2025-Readme.md`, "Image Version: 20261021.299.1", lists
  "gcc 15.2.0" under "Tools", with "gdb 17.1" and "GNU Binutils 2.46". It marks MSYS2 ("pre-installed on image but not added to PATH")
  and Miniconda so; gcc carries no such note. Its source says how it gets there: `build.windows-2025.pkr.hcl:176` runs
  `Install-Mingw64.ps1`, which downloads niXman's MinGW-w64 build, the asset `x86_64-*-release-posix-seh-<runtime>-*.7z`, the version
  and runtime from `toolset-2025.json`, `{"version": "15.*", "runtime": "ucrt"}`, unpacks it to `C:\`, and runs
  `Add-MachinePathItem "C:\mingw64\bin"`; the image's own test, `Tools.Tests.ps1:83`–`:91`, requires `gcc --version` to exit 0. The
  Windows host's gcc, "gcc.exe (MinGW-W64 x86_64-ucrt-posix-seh, built by Brecht Sanders, r1) 16.2.0", has the same target, runtime, threads
  and exceptions, from another builder and at another version. This is the image's source at its latest commits that
  day, not a run on a runner: the first CI run of the step confirms it.
* **R3. CI's Windows steps run in PowerShell.** The Windows jobs' logs: "shell: C:\Program Files\PowerShell\7\pwsh.EXE -command ".
  '{0}'"". The race step is bash: `set -euo pipefail`, and `( cd cmd/generate && ... )`. GitHub's workflow syntax: `shell: bash` runs
  `bash --noprofile --norc -eo pipefail {0}`, and "When specifying a bash shell on Windows, the bash shell included with Git for Windows
  is used."
* **R4. The cost.** The 10 most recent runs on `main`, first attempts: the Linux race step's median 96 s at 1.26.10 and 99 s at 1.27.3;
  the Windows jobs' median 117 s and 115 s, against `timeout-minutes: 30`. On the Windows host under `-race`: the root module 33 s at
  each version, the generator module 98 s at 1.27.3 and 104 s at 1.26.10.
* **R5. No data race on the Windows host.** CI's race step, both modules, `go test -race -shuffle=on -count=1 ./...`, at go1.27.3 and at
  go1.26.10 with `GOEXPERIMENT=goroutineleakprofile`: no "WARNING: DATA RACE" in any package.
* **R6. One test fails there, and not for a race.** In the same run at 1.27.3, `agentproc`'s `TestTapDrainBoundedByGrace`
  (`agentproc/tap_test.go:344`–`:357`) failed: "DroppedTapBytes reports nothing dropped by a tap still writing when the grace ended",
  11.93 s. Again, `agentproc` whole under `-race`, `-shuffle=on -count=5`: 5 of 5 failed, each 9.36 s to 12.04 s. The test alone,
  `-count=20`: under `-race`, 20 of 20 passed at 1.27.3, the first five read taking 1.88 s to 4.52 s, and 20 of 20 at the floor; without `-race`, 20 of 20.
  In the both-module run at the floor, `agentproc` passed, once. CI has never seen it fail: Windows runs it without `-race`, and Linux's
  race step passes.
* **R7. Why.** The test starts the "bulk" agent, which writes 24 MiB before it says it is ready, with a tap that takes 30 ms a write; it
  closes the agent and requires something counted as dropped. `drainTaps` waits up to the grace, 300 ms here, for each tap, then
  abandons it (`agentproc/agentproc.go:320`–`:338`), and `abandon` counts what is queued, less the copy being written, as dropped
  (`agentproc/tap.go:70`–`:80`). So the test holds only while copies are still queued when the grace ends. When the agent's output is
  read slowly, as under `-race` beside the package's other parallel tests, the tap keeps pace, nothing is left, and the test fails: its
  failures took 9 s to 12 s where its passes alone took 1.88 s to 4.52 s, in the five read. The package does what its documentation says; the test's premise
  depends on the machine's speed.
* **R8. A test that does not.** `blockedTap` (`agentproc/tap_test.go:156`–`:165`), which `TestDetachedTapIsPrefix` uses, is a tap whose
  first Write blocks until the test releases it. Close does not wait for a tap it has abandoned (R7), so it cannot hang on one, and
  everything after the first copy stays queued and is counted. A prototype, the test's tap made `blockedTap`, released by a cleanup, in a
  copy of the tree on the Windows host, never in the tree: `agentproc` whole, `-shuffle=on -count=5`, under `-race` at 1.27.3, 5 of 5
  passed (the test 12.19 s to 15.07 s); under `-race` at the floor, 5 of 5; without `-race`, 5 of 5. No data race.

## Decision Drivers

* **Look for a race where the crashes are,** on Windows, at the versions they came at.
* **No step that fails for a reason other than a defect:** a race step that fails on a test's assumption about speed would teach CI's
  readers to rerun it.
* **Tests check behavior, not the machine's speed.**
* **No new dependency** without a record that names it (`AGENTS.md`, "Dependencies").

## Considered Options

* **O1.** The race step on both Windows jobs, after `TestTapDrainBoundedByGrace` is made independent of speed (R8).
* **O2.** The same, on the Windows job at the newest Go alone.
* **O3.** The root package alone under `-race` on Windows, where the crashes are, with no test change.
* **O4.** Leave CI as it is.

## Decision Outcome

Chosen option: "O1", decided by the owner on 2026-10-22 with the recommended answers to questions 1 and 2 (see "Decided with
the owner, 2026-10-22"), because it runs on Windows, at both versions the crashes came at, the
check that Linux already runs, after removing the one failure it would otherwise bring (R6) by a test that no longer depends on speed
(R8), with no new dependency (R2).

* O2 leaves out the floor job, where two of the three crashes came, with its experiment.
* O3 leaves `agentproc`, the package that drives processes on Windows, out of the detector, and the test's premise as it is.
* O4 leaves a race on Windows unlooked for (the report's "weakened, not excluded").

### Specification, as decided

* **`agentproc/tap_test.go`:** `TestTapDrainBoundedByGrace`'s tap is `blockedTap`, released by a cleanup registered after
  `startTapped`'s, so that it runs first; its comment says the tap is still writing because it is blocked in its first Write, not because
  it is slow. What it requires is unchanged: Close within the grace and the slack, and something counted as dropped.
* **`.github/workflows/ci.yml`:** the step "race detector" runs where `runner.os` is `Linux` or `Windows`, as question 1 decides, with
  `shell: bash` (R3), and a comment citing this record and the report. Its commands are unchanged.
* **`docs/architecture.md`:** the paragraph on CI says where the race detector runs.
* **The report:** an addendum, the owner's answer and the first CI runs' result.
* **Unchanged:** the macOS jobs, which have not crashed; every other step.

#### Questions

1. **Which Windows jobs.**
   * Recommended: both, the floor's and the newest's, as on Linux: the crashes came at both versions, two at the floor with its
     experiment. Each job takes about 2 to 3 minutes more (R4).
   * Alternative: the newest's alone (O2).
2. **Where the C compiler comes from.**
   * Recommended: the image's gcc (R2), no action added; the first CI run shows it is on `PATH`, and a later image that drops it fails
     the step, naming the compiler (R1).
   * Alternative: a pinned action that installs a compiler, a dependency this record would then name.

#### Decided with the owner, 2026-10-22

| Question | Answer |
| :--- | :--- |
| 1. Which Windows jobs | As recommended: both. |
| 2. Where the C compiler comes from | As recommended: the image's, "already installed and in path", the owner said, asking that it be verified: MinGW-w64, on the machine's `PATH` (R2). |

### Consequences

* Good, because a race in either module, on Windows, at both versions, fails CI where today it would pass.
* Good, because `TestTapDrainBoundedByGrace` checks the grace's bound on any machine, at any speed.
* Neutral, because each Windows job takes about 2 to 3 minutes more, in jobs of about 2.
* Bad, because the step depends on the image's gcc, which a later image could move or drop; the step then fails, naming it.

### Confirmation

The PLAN shows: the changed test passing under `-race` on the Windows host where the old one failed, and failing on planted breaks of
the bound it checks; and, from a CI run after the push, the race step run and passed on both Windows jobs, by a check first seen to fail
on a run from before the change, where the step is skipped there.

## More Information

* [0032-REPORT-windows-floor-runtime-crashes.md](../reports/0032-REPORT-windows-floor-runtime-crashes.md): the crashes, and the open
  item this record answers.
* [0033-MADR-traceback-on-windows-floor-ci.md](0033-MADR-traceback-on-windows-floor-ci.md): the owner's earlier answers to that
  report, and the Windows floor job's `GOTRACEBACK`, which the race step runs under too.
* [0030-MADR-record-initialize-on-arrival.md](0030-MADR-record-initialize-on-arrival.md): the taps, their grace and their dropped
  count, which the test checks.
