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
