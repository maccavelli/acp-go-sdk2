---
status: in-progress
date: 2026-10-22
associated-madr: "0034-MADR-race-detector-on-windows-ci.md"
---

# Implement 0034: run the race detector on CI's Windows jobs, once the tap drain test no longer depends on the machine's speed

Associated MADR: [0034-MADR-race-detector-on-windows-ci.md](0034-MADR-race-detector-on-windows-ci.md)

## Goal

CI's step "race detector" runs, and passes, on both Windows jobs as on the Linux jobs; before it does, `TestTapDrainBoundedByGrace`
checks the grace's bound without depending on the machine's speed. This PLAN assumes the MADR's recommended answers to questions 1 and
2; an answer otherwise amends it before step 2.

## Scope

* **`agentproc/tap_test.go`:** `TestTapDrainBoundedByGrace`, its tap and its comment (the MADR's R8).
* **`.github/workflows/ci.yml`:** the step "race detector": its `if`, `shell: bash`, and a comment.
* **`docs/architecture.md`:** where CI runs the race detector.
* **`docs/reports/0032-REPORT-windows-floor-runtime-crashes.md`:** an addendum.
* **The records:** this pair and their rows in `docs/README.md`.
* **Not in scope:** any non-test Go file; the macOS jobs; every other step of CI; `scripts/go-precheck.sh`, which already runs
  `go test -race` on the host it runs on.

## Implementation Steps

Each step ends with its checks, an execution record here, and the files staged for the owner. Every new check is seen to fail on a
planted break in a scratch copy, never in the tree.

1. **Records:** this pair and their rows, `proposed` until approved; then this PLAN `in-progress`.
2. **The test.** `TestTapDrainBoundedByGrace`: `tap, release := blockedTap(t)` in place of `&capture{delay: 30 * time.Millisecond}`;
   `t.Cleanup(release)` after `startTapped`, so that the release runs before the agent's Close in cleanup; its comment: Wait waits for a
   tap no longer than the grace, a tap blocked in its first Write being abandoned when the grace ends and what it has not written counted
   as dropped. Checks:
   * on the Windows host, a copy of the tree: `agentproc` whole, `go test -race -shuffle=on -count=5`, at go1.27.3 and at go1.26.10
     with the experiment, and without `-race`: each 5 of 5 passed; and the test as it was, in the same copy at 1.27.3 under `-race`,
     fails as the MADR's R6 recorded;
   * planted breaks, each in a scratch copy: `drainTaps` waiting for the tap without the grace's timer, and `abandon` not counting what
     is queued; each must fail the test;
   * the gates, `tools/gates.py --all`.
3. **CI.** The step "race detector": `if: runner.os == 'Linux' || runner.os == 'Windows'`, `shell: bash`, a comment citing the MADR
   and the report; `docs/architecture.md`: the paragraph on CI says the race detector runs on the Linux and Windows jobs. Checks:
   `actionlint` at the version CI pins; the gates. After the owner's push:
   * `tools/watch_ci.py <commit>`: every job passed;
   * a check that reads the run's first-attempt jobs through `tools/ci_jobs.py`'s readers and requires the step "race detector" to have
     run and passed on both Windows jobs and both Linux jobs, and skipped on the macOS jobs; first seen to fail on run `39869786459`, from
     before the change, where the Windows jobs skip it;
   * that step's log on each Windows job, read by its job id: an `ok` line for the root package, `agentproc` and the generator's
     packages, and no "WARNING: DATA RACE".
4. **Close:** the report's addendum: the owner's answer, the Windows host's runs (the MADR's R5), and the first CI runs' result; this
   PLAN and its row `complete`.

## Verification

Planted breaks, each in a scratch copy:

| For | Break | The check that must fail |
| :--- | :--- | :--- |
| the test | `drainTaps` waits for the tap's end with no timer | `TestTapDrainBoundedByGrace`, Close not within the grace and the slack |
| the test | `abandon` counts nothing as dropped | `TestTapDrainBoundedByGrace`, nothing dropped |
| the test | the 30 ms tap restored | `agentproc` whole under `-race` at 1.27.3 on the Windows host, as R6 |
| the CI check | run `39869786459`, from before the change | the check: the step skipped on the Windows jobs |

Acceptance:

* `TestTapDrainBoundedByGrace` passes on the Windows host under `-race` at both versions, and without it, and on every CI job;
* CI's race step runs and passes on both Windows jobs and both Linux jobs, and the macOS jobs are unchanged;
* `make pre-add-check`, `make lint`, `make api` and the other tests are unchanged by this work.

## Rollout and Rollback

Steps 2 and 3 are one commit each, reverted by reverting it. The test change stands without the CI change. Nothing in the module's
non-test code changes: no consumer is affected, and no release is needed. Should a later runner image drop gcc, the Windows race step
fails naming the compiler (the MADR's R1); reverting step 3 restores CI as it was while the source of a compiler is decided.

## Execution record

### Step 1, records, 2026-10-22

The owner asked for a Windows race detector; this pair was written from a read-only investigation, every figure in the MADR's R1 to R8.
The owner answered the MADR's questions, 1 "both", and 2 "it is already installed and in path i think it is ming, verify": the image's
source was read and R2 records it, MinGW-w64 on the machine's `PATH`; the MADR is `accepted`. The owner approved this PLAN
("proceed"); it is `in-progress`. `make check-records` and `make markdownlint` passed, and `tools/idscan.py` found no match in the
three files, against the deny list's 17 patterns.

### Step 2, the test, 2026-10-22

The owner committed step 1 as `b8a978e`. What changed:

* **`agentproc/tap_test.go`:** `TestTapDrainBoundedByGrace`'s tap is `blockedTap(t)`, released by `t.Cleanup(release)` registered after
  `startTapped`, so that it runs before the agent's Close in cleanup; its comment says why a slow tap was not enough, citing the MADR.
  What it requires is unchanged: Close within the grace and the slack, and something counted as dropped.

**Locally,** `gofmt -l agentproc/` printed nothing; `go vet ./agentproc`, exit 0; the test under `-race`, `-count=3`, 3 of 3 passed,
2.79 s to 2.82 s.

**On the Windows host,** each a fresh copy of the tree, removed after; `agentproc` whole, `-shuffle=on -count=5 -v`:

| Run | Exit | The test | Data races |
| :--- | :--- | :--- | :--- |
| the changed test, `-race`, go1.27.3 | 0 | 5 of 5 passed, 11.35 s to 14.84 s | 0 |
| the changed test, `-race`, go1.26.10 with the experiment | 0 | 5 of 5 passed, 4.50 s to 8.02 s | 0 |
| the changed test, without `-race`, go1.27.3 | 0 | 5 of 5 passed, 1.78 s to 2.92 s | 0 |
| the test as it was, `-race`, go1.27.3 | 1, as required | 5 of 5 failed, 11.56 s to 14.87 s, "nothing dropped" | 0 |

**Planted breaks,** each in its own copy of the tree, the test run under `-race`; 2 of 2 failed as required, and the third row above is
the third:

| Break | The check that failed |
| :--- | :--- |
| `drainTaps` waits for the tap's end with no timer | the test: "tap_test.go:356: Close took longer than 3.3s" |
| `abandon` counts nothing as dropped | the test: "tap_test.go:360: DroppedTapBytes reports nothing dropped by a tap still writing when the grace ended" |
| the 30 ms tap restored | on the Windows host, as the table shows |

**Gates,** `ACP_LINUX_HOST` and `ACP_WINDOWS_HOST` set, `tools/gates.py --all`, each exit 0: all 18 gates, "all gates passed";
`make pre-add-check`, "176 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck)"; `make check-tools`, "Ran 53 tests";
`make markdownlint`, 29 files; `platforms`, "13 of 13 passed"; `make api`, "against v1.20.0: 0 incompatible, 0 compatible"; the floor
gate at go1.26.10 passed; `windows` and `linux` passed; `idscan`, "1 file(s), 17 pattern(s), 0 match(es)".

### Step 3, CI, 2026-10-22

The owner committed step 2 as `347949e`. What changed:

* **`.github/workflows/ci.yml`:** the step "race detector": `if: runner.os == 'Linux' || runner.os == 'Windows'`, `shell: bash`, and a
  comment citing the MADR and the report; its commands unchanged.
* **`docs/architecture.md`:** CI runs both modules' tests with `-race` on the Linux and Windows jobs.

**`actionlint`,** v1.7.12, the version CI pins: exit 0, nothing reported.

**The CI check,** a script in the scratch directory that reads a run's first-attempt jobs, and each Windows job's log by its id, through
`tools/ci_jobs.py`'s readers: the step "race detector" "success" on both Linux and both Windows jobs, "skipped" on both macOS jobs,
and in each Windows job's race step an `ok` line for the root package, `agentproc` and a package of the generator module, and no
"WARNING: DATA RACE". **Seen to fail first,** on run `39869786459`, from before the change, exit 1: "MISS validate (windows-2025,
1.27.3): step 'race detector' skipped, want success", the same for 1.26.10, the four others "ok", "does NOT hold". Its log reading,
which that run's Windows jobs could not reach, was run on that run's Linux job at the newest Go, where the step ran: the step's 24
lines, from "##[group]Run set -euo pipefail", each of the three `ok` lines found, and, with the `ok` lines taken out, all three
reported missing.

**Gates,** `tools/gates.py --all`, each exit 0: all 18 gates, "all gates passed"; `make pre-add-check`, "176 file(s) clean (gofmt,
golangci-lint, go vet, go test, govulncheck)"; `make check-tools`, "Ran 53 tests"; `make markdownlint`, 29 files; `platforms`, "13 of
13 passed"; `make api`, "against v1.20.0: 0 incompatible, 0 compatible"; the floor gate passed; `windows` and `linux` passed;
`idscan`, "2 file(s), 17 pattern(s), 0 match(es)".

What is left: the owner's push; then `tools/watch_ci.py` and the CI check on its run, and step 4.
