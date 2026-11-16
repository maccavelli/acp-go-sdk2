---
status: in-progress
date: 2026-11-16
associated-madr: "0057-MADR-agentproc-test-graces-on-a-loaded-runner.md"
---

# Implement 0057: give agentproc's tests that need an agent's exit seen in time a grace a loaded runner fits

Associated MADR: [0057-MADR-agentproc-test-graces-on-a-loaded-runner.md](0057-MADR-agentproc-test-graces-on-a-loaded-runner.md)

## Goal

Remove the cause of CI run `43875424670`'s failure, and of the same kind in the rest of `agentproc`'s tests: an assertion that
needs an agent's exit seen within `testGrace`, 300 ms, or the standard-error copy done within `WaitDelay`, 150 ms, which a loaded
runner under `-race` exceeds (the MADR's F3, F6). At the end of it:

* every test the audit (phase 1) puts in the class runs under `exitGrace`, `slack`, with its assertion unchanged;
* every other test keeps `testGrace`;
* each moved test fails, at `HEAD`, the client-side plant and the in-process load, and passes them amended; the break "the first
  step waits 0 s" fails it amended;
* the docs the change makes false are corrected, and `CHANGELOG.md`'s `Unreleased` has its line;
* CI's six jobs pass on the commit that carries it.

This PLAN was drafted to the MADR's recommendations, and amended on 2026-11-16, before any phase ran, for the owner's answers: the
decision "Longer exit grace for the tests that need it", and the answers to questions 1, 3 and 4 (the questions, below, and the
execution record's entry of that date). No question is open. The owner approved it for execution the same day, phases 1 to 4 in
order and then phase 5 (the execution record's approval entry).

## Scope

**Preconditions.** The MADR is `accepted` with its questions answered (2026-11-16), and this PLAN amended to the answers (the same
day). This PLAN approved by the owner for execution. *Met 2026-11-16: the owner's ruling, relayed by the coordinator, "Run phases
1 to 4 through; then the report" (Execution record).* The MADR, this PLAN and their `docs/README.md` rows are committed alone
first, as the bootstrap exception allows.

**Order and review** (2026-11-16, by the approval). Phases 1, 2, 3 and 4 run in that order, each reviewed, committed and CI-checked
by the coordinator before the next starts; phase 5 runs after CI is green on the fix, phase 4's last step, and not earlier or in
parallel. A deviation stops the PLAN and goes to the owner.

**Commits.** Each phase ends in one commit, `git commit --no-edit`, after its checks pass. The executing agent stops with the phase
staged and does not commit or push. The coordinator commits and pushes each approved phase after the disclosure guard (`make
push-guard`) exits 0, by the owner's standing approval of 2026-11-15, "Yes, all approved plans", recorded in
[0056-PLAN-bring-the-docs-current-and-keep-them-current.md](0056-PLAN-bring-the-docs-current-and-keep-them-current.md),
"Commits". Tags and releases still need the owner's word each time; this PLAN cuts none.

**In scope:** `agentproc`'s test files, every test of the class (question 1's answer); `docs/architecture.md`; an in-place
annotation of [0027-PLAN-agent-process-helper.md](0027-PLAN-agent-process-helper.md)'s "The grace in tests is 300 ms";
`CHANGELOG.md`; this record pair and their rows. By question 4's answer, `ErrTerminated`'s doc comment in `agentproc/agentproc.go`
and one sentence of `README.md`. By question 3's answer, a measured REPORT, phase 5.

**Out of scope:** any behaviour of `agentproc` or `internal/proctree`; the default grace; `WaitDelay`'s default; CI; the root
package's and `remote`'s tests (question 1's option not chosen); a fix of the 56 MiB tests' 30 s bound, which waits for phase 5's
REPORT and the owner's decision on it (question 3's answer).

**Where the race gates run** (amended 2026-11-16, for question 3's answer). Until the owner decides on phase 5's REPORT, the
repository's race gates, `make pre-add-check` and the whole module's `go test -race -shuffle=on ./...`, run on this Mac through
`run-heavy`, where, as the text of the option the owner chose says, they pass (not run on this Mac for this record). The Linux
host's runs under `-race` stay in the PLAN as evidence for the tests this PLAN moves: they are
judged on those tests, and the MADR's F7 tests (`TestDetachedTapIsPrefix`, `TestInProcessDetachedTapIsPrefix`, and, under the
in-process load, `TestBlockedTapDetached` and `TestTapErrorIgnored`) are reported beside them with their own counts, run, not skipped,
and not counted as passes or as this PLAN's failures. A failure of any other test there is a finding, and stops the phase.

**The rule each test is classified by** (the constant's comment carries it):

| Class | The test's assertion | Its grace |
| :--- | :--- | :--- |
| A | needs the agent's exit seen within `Close`'s first step, or within `WaitDelay` of its exit: `Close` or `Wait` nil, "no signal", `ExitedFirst` after a clean exit | `exitGrace` |
| B | waits a grace out, then needs the exit seen within the next step: `TestCloseLadder/exits_on_SIGTERM` | `exitGrace`; its bounds in graces scale with it |
| C | waits graces out and asserts only a floor, a kill, or `ErrTerminated` or `ErrKilled` either way | `testGrace` |
| D | no grace in its outcome | unchanged |

## Implementation Steps

### Phase 1: the audit (mid tier, high effort)

Read-only in the tree; experiments in scratch copies (`git archive HEAD`).

1. List every test and subtest in `agentproc/*_test.go`, the `InProcess` ones included, with the grace it runs under, the agent or
   `Serve` it runs, and each assertion bounded by a grace, `WaitDelay` or `within`.
2. Classify each by the rule above, from the code, with `file:line` for the assertion that decides it.
3. Run the class experimentally: the whole package at `HEAD` beside `TestZZBurn` (the MADR's in-process load: 32 busy goroutines,
   `GOMAXPROCS=2`, a scratch-only test file), 5 rounds under `-race -shuffle=on` on the Linux host, through
   the fleet's `run-on-host --need linux --race -- <command>`, in chunks of a few minutes. Every
   failure named with its message; a failure the reading put in class C or D is a finding, investigated before phase 2, except the
   F7 tests, which are counted apart ("Where the race gates run").
4. Record the table, the chunks and their results in this PLAN's execution record.

**Verification:** every test of the package appears in the table once; every failure of step 3 is a class A or B test, or a named
finding. **Acceptance:** the table and the runs recorded; the owner told the count of class A and B tests before phase 2.
*Annotated 2026-11-16, by the approval:* the count goes to the coordinator, who reviews phase 1 before phase 2 starts; the owner is
asked only on a deviation ("Order and review").

### Phase 2: the tests (mid tier, medium effort)

1. `agentproc/agentproc_test.go`: `const exitGrace = slack`, 3 s, the figure the owner's chosen option named; its comment stating
   the rule and citing the MADR; `startCmd` gains a
   form that takes a grace (`startCmdGrace`), `start` and `startCmd` keeping `testGrace`.
2. `agentproc/ties_test.go`: `startBeating` gains the same (`startBeatingGrace`); `TestDescendantsEnded`'s "the agent exits at its
   input's end" runs under `exitGrace`, bounded by `within(t, exitGrace+slack, …)`.
3. Each other class A and B test of phase 1's table, the same way: its grace, and its `within` bound and any bound stated in graces.
   Class B's bounds keep their shape (`took >= exitGrace`, `took < 2*exitGrace`).
4. Comments that state a test's grace, as `tappedFrames`' does, say which grace and why.

**Verification, each in a scratch copy, through `tools/plants.py` where it fits:**

* the client-side plant (`childAgent.wait` sleeps 350 ms after `Cmd.Wait`; for an `InProcess` test, the same after `Serve`
  returns): each moved test fails at `HEAD` and passes amended;
* the plant 2.5 s late passes, 3.5 s late fails, for `TestDescendantsEnded`'s subtest (the margin claimed);
* the break "the first step waits 0 s": each class A test fails amended;
* the A/B under the in-process load, 10 rounds each, `-race`, the Linux host: fails at `HEAD` for at least the tests phase 1 saw fail,
  0 of 10 amended;
* `make pre-add-check FILES="<the changed test files>"` on this Mac, through the fleet's `run-heavy --`, since it runs
  `go test -race` on both modules ("Where the race gates run").

**Acceptance:** each check above passes, its output recorded; `git diff --stat` touches only `agentproc/*_test.go`.

### Phase 3: the docs (cheapest tier, medium effort)

1. `docs/architecture.md`, "Checks that guard it" for `agentproc`: the sentence that names the tests "under a grace of `slack`"
   names the class, or the tests, and cites the MADR.
2. [0027-PLAN-agent-process-helper.md](0027-PLAN-agent-process-helper.md), "The grace in tests is 300 ms": annotated in place,
   "(Amended 2026-…, by 0057: the tests that need an exit seen within a grace run under `slack`)", the original kept.
3. `CHANGELOG.md`, `Unreleased`: one line, for example "Made `agentproc`'s tests that need an agent's exit seen within `Close`'s
   first step run under a grace of 3 s, so a loaded runner under `-race` no longer fails them; tests only."
4. By question 4's answer: `ErrTerminated`'s comment (`agentproc/agentproc.go:29`-`:32`) and the README's `Close` paragraph say that an
   agent exiting at its input's end just as the grace ends can be reported terminated with a status of 0. A comment change: run
   `make api` against the latest tag, which must report 0 incompatible, 0 compatible.
5. Every other doc that names `testGrace`, `slack` or these tests: `grep -rn` the tree for them, each hit read, and each one the change
   makes false corrected.

**Verification:** `make check-records`, `make markdownlint`, `make disclosure-check`, each exit 0; `make pre-add-check` with the
changed files when `agentproc.go` changed. **Acceptance:** no doc found false by step 5.

### Phase 4: confirmation and CI (mid tier, medium effort)

1. The amended package, 20 rounds, `go test -race -shuffle=on -count=20 ./agentproc/`, on the Linux host through `run-on-host
   --need linux --race`, in chunks; then the whole module three times as CI's step runs it, through the same. Both are judged as
   "Where the race gates run" says: on the tests this PLAN moved, with the F7 tests' failures counted and reported apart.
   (Amended 2026-11-16: as first written, the module's three rounds on the Linux host had to pass, which F7 makes impossible today:
   one or both F7 tests failed in 5 of 5 module rounds there.)
2. The race gate of record: the whole module's `go test -race -shuffle=on ./...`, three times, on this Mac through `run-heavy`.
3. The gates: `tools/gates.py --all` through `run-heavy` here, its host gates through `run-on-host`, one heavy run per host at a time.
   Its `linux` gate runs without `-race`, where F7 was not measured; should an F7 test fail it there, that is a deviation, and the
   phase stops for the owner.
4. The commit, at the end of each phase after its pre-add checks, by `git commit --no-edit`; the push only on the owner's word.
   *Annotated 2026-11-16, by the approval:* the coordinator commits and pushes each phase, after the disclosure guard exits 0, under
   the owner's standing approval ("Commits", in Scope); a tag or a release still needs the owner's word.
5. After the push: CI's six jobs with `tools/watch_ci.py <commit> --branch main`, recorded.

**Acceptance:** on the Linux host, every test this PLAN moved passes in each of the 20 package rounds and the three module rounds,
and no test fails there but the F7 tests, whose failures are counted and recorded; the module's race gate passes three of three
on this Mac; the gates pass; CI's six jobs "completed success".

### Phase 5: the 56 MiB tests' 30 s, by question 3's answer (mid tier, high effort)

*Annotated 2026-11-16, by the approval:* this phase runs last, after CI is green on the fix (phase 4, step 5), not earlier and not
in parallel with phases 1 to 4. As first written, nothing here fixed its place; it needs no change of the tree the other phases make.

1. Measure `TestDetachedTapIsPrefix` and `TestInProcessDetachedTapIsPrefix` on the Linux host: the time to the 56 MiB under `-race`,
   alone and beside the module, and what it is spent on.
2. Write `docs/reports/0057-REPORT-<slug>.md`: what was measured, decided nothing; a row in `docs/README.md`.

**Acceptance:** the REPORT committed and put to the owner, who then decides what follows, as the answer "A measured report, then
decide" says; any defect it finds left for a record of its own. Until that decision, the race gates run as "Where the race gates
run" says.

## Verification

The phases' checks, together: the plants and breaks fail and pass as required; the A/B under load; 20 rounds on the Linux host,
judged on the moved tests with the F7 tests counted apart; the module's race gate on this Mac; `tools/gates.py --all`; CI's six
jobs. Each new check is seen to fail on a deliberately broken input in a scratch copy before it is
relied on, and the record says how it failed.

## Rollout and Rollback

A test and docs change, with no release of its own; the next release's notes may name it under maintenance. Rolling back is
reverting the phase commits: the tests return to `testGrace`, and the failure's rate with it. Nothing is migrated.

## Questions for the owner, answered 2026-11-16

Closed: each is answered under itself, by the owner's ruling, relayed by the coordinator, except question 2, answered by the text of
the chosen decision. The options are kept as they were put.

1. **Scope.** Answered (a): "Every `agentproc` test in the class". Not chosen: (b) and (c).
   * (a) Recommended: every `agentproc` test phase 1 puts in classes A and B. Cost: phase 1, and a few test files.
   * (b) The failing subtest alone, as 0027's step 9, 0030's step 12 and 0049's phase 3 did. Cost: least; the rest of the class
     (the MADR's F6) is left to fail later.
   * (c) (a), and an audit of the root package's and `remote`'s grace-bound tests, in a record of its own. Cost: another pair.
2. **The figure.** Answered (a), by the text of the decision the owner chose, "Longer exit grace for the tests that need it", which
   named the suite's existing 3 s busy-runner bound, `slack`; the coordinator did not put this question on its own.
   * (a) Recommended: `slack`, 3 s, the suite's bound for a step on a busy runner, about five times the worst measured.
   * (b) 1.2 s, 0049's: under twice the worst measured beside the module under load (the MADR's F3).
   * (c) `2 * slack`, as `descendantGrace`: more margin, and class B tests then take about 6 s a run.
3. **The 56 MiB tests' 30 s on the Linux host (the MADR's F7).** Answered (a): "A measured report, then decide". Not chosen: (b)
   and (c). The option's text added that until then the race gates keep running on this Mac, where they pass; "Where the race gates
   run" and phase 4 are amended to it.
   * (a) Recommended: phase 5, a REPORT, measured; a defect it finds gets a record of its own.
   * (b) Fold its fix into this PLAN. Cost: a second mechanism in one record.
   * (c) Note it and do nothing; the Linux host's race runs of `agentproc` then keep failing on it.
4. **`ErrTerminated`'s comment.** Answered (a): "Say what the code does". Not chosen: (b) and (c).
   * (a) Recommended: say what the code does, in the comment and the README, phase 3 step 4.
   * (b) Leave it.
   * (c) The MADR's O1, a product change, in a record of its own.

## Execution record

### Drafting (2026-11-16)

Drafted by a strongest-tier model at high effort on the coordinator's brief, which relayed the owner's ruling "Root-cause it in its
own record". Nothing in the tree changed but these two records and their `docs/README.md` rows. What was run, and where:

* **Read:** `agentproc/` and `internal/proctree/` at `32a74d7`; the 0027, 0030, 0042 and 0049 records named in the MADR;
  `docs/architecture.md`, `AGENTS.md`, `tools/README.md`; Go 1.27.4's `os/exec/exec.go`.
* **CI, read:** `gh run view 43875424670 --log-failed` and the failed job's log; `tools/ci_jobs.py tally` (40 runs) and `failure`
  for this run and three before it; the race steps of the 16 Linux jobs of this run and the seven before it.
* **This Mac, ran:** the overflow subtest alone; the SIGTERM probe; the plants and the break (8 cases, and the break 20 times); the
  measuring copy idle and under the in-process load; the A/B, 3 rounds, and one round of the whole package under the load; all
  without `-race`.
* **The Linux host, ran,** each through `run-on-host --need linux --race`, one heavy run at a time:

  | Chunk | Command | Rounds | Result |
  | :--- | :--- | ---: | :--- |
  | B1 | `go test -race -shuffle=1794835577691517934 -count=6 ./agentproc/` at `HEAD` | 6 | `TestDescendantsEnded` passed 6 of 6; failed: `TestDetachedTapIsPrefix` once, "the bulk agent did not say it was ready within 30s"; `TestCloseLadder/exits_on_SIGTERM` once, 632 ms against 600 |
  | M | the measuring copy: `go test -race -shuffle=on ./...` twice, then the measured subtest under the in-process load | 2 + 1 | 1 of 80 measured closes not seen within 300 ms, CI's message; 18 of 20 under load; `TestDetachedTapIsPrefix` failed in both module rounds, `TestInProcessDetachedTapIsPrefix` in one |
  | PA | the plants and the break under `-race`; the A/B under the in-process load, 10 rounds each | 8 + 20 | each case as required; `HEAD` 7 of 10 failed, amended 0 of 10 |
  | B3 | `go test -race -shuffle=on ./...` at `HEAD` | 3 | `TestDescendantsEnded` passed 3 of 3; `TestDetachedTapIsPrefix` failed in rounds 1 and 3, `TestInProcessDetachedTapIsPrefix` in 1 and 2 |
  | AU | the whole package at `HEAD` beside the in-process load, `-race -shuffle=on` | 2 | round 1 failed `TestCallersCommandKept` and `TestDefaultPdeathsig`, "Close = agentproc: agent terminated", `TestBlockedTapDetached` and `TestTapErrorIgnored`; round 2 passed |

  The scratch trees carried only what the MADR names: the measuring copy's stamps and `TestMeasureParentClose`; the plants'
  `PLANT_SEEN_LATE` and `PLANT_EXIT_LATE` and the break's `BREAK_NO_FIRST_WAIT`, each read from the environment and a no-op unset; the
  candidate fix's `exitGrace`, `startCmdGrace` and `startBeatingGrace`; and `TestZZBurn`, which skips unless `BURN` is set. B3 ran
  the A/B set's `HEAD` copy, which has `TestZZBurn` and is otherwise `HEAD`.

* **The CI re-run,** once, as evidence and not as a fix: `gh run rerun 43875424670 --failed` at 14:56 UTC, after the runs above.
  Attempt 2: the run "completed success"; `validate (ubuntu-24.04, 1.27.4)` (job `133382229133`) "completed success", its race step's
  root package in 22.970 s, `agentproc` in 30.381 s, `remote` in 28.052 s; `validate (macos-15, 1.26.11)`, cancelled in attempt 1,
  was re-run with it and "completed success". The failure did not repeat, which a margin this rare predicts (the MADR's F3); it is
  not evidence that the cause is gone.
* **Checks after the last edit,** with the two records and `docs/README.md` staged: `python3 -B scripts/check_records.py
  --check-all`, `python3 -B scripts/check_disclosure.py` and `markdownlint-cli2` v0.23.2 (`make markdownlint`'s command), each exit
  0; a scan of the two records for the hosts' names, the local account name and real-machine paths found none, the module's
  import path in the quoted log aside, which the records quote throughout.

### 2026-11-16: the owner's answers

The owner answered on 2026-11-16; the coordinator relayed each label, the option's text as the owner saw it, and the options not
chosen:

* **The decision:** "Longer exit grace for the tests that need it", the MADR's O2. Not chosen: O1, changing `Close` in the product;
  O3, raising the grace for the whole test package. Its text named the figure, `slack`, 3 s, which answers question 2 (a); question
  2 was not put on its own.
* **Question 1, scope:** "Every `agentproc` test in the class".
* **Question 3, the 56 MiB tests:** "A measured report, then decide". Its text: until then the race gates keep running on this Mac.
* **Question 4, `ErrTerminated`'s comment:** "Say what the code does".

**Touched, uncommitted and staged:**

* the MADR: `status: accepted`; Context's last sentence; F7's last sentences; "Decision Outcome", its first paragraph the choice with
  the label and an annotation of what the draft recommended; the Neutral consequence; More Information's `ErrTerminated` item; each
  of the four questions answered under itself;
* this PLAN: Goal's last paragraph; Scope's preconditions, in and out of scope, and a new "Where the race gates run"; phase 1 step
  3; phase 2 step 1 and its pre-add check; phase 3 step 4; phase 4 steps 1 to 5 and its acceptance, amended for F7 (below); phase 5's
  heading and acceptance; Verification; the questions, closed; this entry;
* `docs/README.md`: the MADR's row, `accepted`, and the PLAN's.

**The F7 amendment.** Phase 4 as first written required the whole module's three rounds under `-race` on the Linux host to pass, and
F7's two tests failed in 5 of 5 such rounds there: a gate that could not pass. Amended, without hiding F7: the race gate of record
runs on this Mac, as question 3's answer says; the Linux host's rounds are kept and judged on the tests this PLAN moves; F7's tests run
there, not skipped, and their failures are counted and recorded apart; a failure of any other test stops the phase.

**Status.** The owner approved the decision on 2026-11-16, not yet this PLAN's execution: it stays `proposed`, and no phase has run.
*Annotated 2026-11-16: superseded by the approval entry below.*

### 2026-11-16: approval

* **Approval.** Asked how this PLAN should run, the owner chose, by the owner's ruling, relayed by the coordinator: "Run phases 1
  to 4 through; then the report". Not chosen, as shown: one phase at a time; hold the plan. So phases 1, 2, 3 and 4 run in order,
  each reviewed, committed and CI-checked by the coordinator before the next; a deviation stops the PLAN and goes to the owner;
  phase 5 runs after CI is green on the fix ("Order and review", in Scope; phase 5's annotation).
* **Commits.** The coordinator commits and pushes each approved phase after the disclosure guard exits 0, by the owner's standing
  approval of 2026-11-15, "Yes, all approved plans" ("Commits", in Scope). Tags and releases still need the owner's word.
* **Status.** The MADR is `accepted` and this PLAN `in-progress`, their `docs/README.md` rows to match. That follows
  [0056-PLAN-bring-the-docs-current-and-keep-them-current.md](0056-PLAN-bring-the-docs-current-and-keep-them-current.md) as it stood
  between its approval and its phase 1 (`e67216c`): `status: in-progress`, its precondition annotated "Met", an approval entry, and
  its row `in-progress`. No phase has run; phase 1 is briefed separately.
* **Touched:** this PLAN's frontmatter, Goal, Scope (the precondition, "Order and review", "Commits"), phase 4 step 4 and phase 5,
  annotated; the "Status" line above, annotated; this entry; the PLAN's row in `docs/README.md`.
* **The F7 gate.** Settled before phase 1 by the amendment in the entry above: phase 4's Linux rounds are judged on the moved tests,
  F7's tests counted apart and not skipped, and the module's race gate of record runs on this Mac. No question is left open by it.
