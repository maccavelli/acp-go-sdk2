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

*Added 2026-11-16, after phase 1's audit, for the owner's two rulings on it (the MADR's amendment of that date, W1 to W5).* The
goal also covers: the four tests the audit left undecided, `TestWaitErrorSaysTheCauseOnce/Kill,_Serve_returning_its_cause`,
`TestCloseContextBoundsHandlers/in_process,_KillContext`, `TestCloseContextCause` and `TestCloseContextNotHandlers`, moved to
`exitGrace`, each first seen to fail at `HEAD`; and four bounds that are not a grace, in `TestNoLostOutput`, `TestTapErrorIgnored`,
`TestOutputTapVerbatim` and `TestCloseReadsTheAgentsLastOutput`, rescaled, each from its own measurement, by a new phase 1b. A
test that cannot be made to fail at `HEAD` does not move and is reported; a bound that cannot be given a figure that follows from
its measurement and keeps its test able to fail on its defect stops the PLAN for the owner.

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
*Added 2026-11-16, after phase 1's audit:* the tests the Linux rounds are judged on include, once moved, the four of the MADR's W1
and the four of W2. `TestTapErrorIgnored` leaves the list counted apart: its fixed 3 s wait is W2's. `TestBlockedTapDetached` stays
on it for its failure to be ready within 30 s (`tap_test.go:294`), with the two 56 MiB tests; any other failure of it counts as a
moved test's, since its `Close` moves with class A.

**Scope, widened** (added 2026-11-16, after phase 1's audit, by the owner's rulings "Move all four, each proved first" and "Widen
0057 to these bounds now", each relayed by the coordinator; the MADR's amendment of that date). In scope as well: the four tests of
the MADR's W1, and the four bounds of its W2, in `agentproc_test.go` and `tap_test.go`, measured in phase 1b before phase 2 changes
them. Out of scope still: the two 56 MiB tests and `TestBlockedTapDetached`'s readiness (phase 5's report); `closecontext_test.go:156`
until question 5 is answered.

**Order and review, amended** (added 2026-11-16): phases 1, 1b, 2, 3 and 4 run in that order, each reviewed, committed and
CI-checked by the coordinator before the next; phase 5 after CI is green on the fix, as before.

**The rule each test is classified by** (the constant's comment carries it):

| Class | The test's assertion | Its grace |
| :--- | :--- | :--- |
| A | needs the agent's exit seen within `Close`'s first step, or within `WaitDelay` of its exit: `Close` or `Wait` nil, "no signal", `ExitedFirst` after a clean exit | `exitGrace` |
| B | waits a grace out, then needs the exit seen within the next step: `TestCloseLadder/exits_on_SIGTERM` | `exitGrace`; its bounds in graces scale with it |
| C | waits graces out and asserts only a floor, a kill, or `ErrTerminated` or `ErrKilled` either way | `testGrace` |
| D | no grace in its outcome | unchanged |

*Added 2026-11-16, after phase 1's audit.* By the owner's ruling "Move all four, each proved first", relayed by the coordinator,
class A reads: the exit, the drain or the return seen within the first grace-bound wait. By the coordinator's applications of
standing rules, recorded as such and not as the owner's rulings:

* a test already under `slack` or more keeps its grace: the rule never lowers one (the audit's finding 7);
* a bound stated in graces scales with its test's grace: `closecontext_test.go:145`, `:180` and `:246`; `:214` belongs to
  `TestCloseContextHurriesTheSteps`, class C, which does not move, and stays (finding 5);
* each moved test is seen to fail at `HEAD` under a plant that fits the wait it depends on, and a test no plant can fail does not
  move and is reported (finding 4: the twelve class A tests whose outcome has `WaitDelay` and no first step need a plant that
  outlives `WaitDelay`);
* the in-process load runs for the whole of each test binary's run, so every moved test runs under it (finding 6).

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
*Annotated 2026-11-16, after the audit:* the counts went to the coordinator, who put the audit's open points to the owner; the
owner's two rulings are in the execution record's entry "2026-11-16: the owner's rulings on the audit".

### Phase 1b: the bounds that are not a grace, measured (mid tier, high effort)

*Added 2026-11-16, after phase 1's audit, by the owner's ruling "Widen 0057 to these bounds now", relayed by the coordinator.* A
phase of its own, not steps of phase 2: its output is measurements and figures, recorded in this PLAN and committed alone, so the
coordinator reviews each figure, and the owner hears of any bound that stops (the MADR's W4), before a test changes; phase 2's commit
then holds test changes whose figures are already reviewed. Read-only in the tree; experiments in scratch copies (`git archive
HEAD`); every run on the Linux host through the fleet's `run-on-host --need linux --race`, in chunks of a few minutes.

1. **The parts, stamped.** For each bound of the MADR's W2, a scratch copy stamps the bound's start and each part it waits for:
   * `TestNoLostOutput` and `TestTapErrorIgnored`: the go notification sent; the burst agent's write done and its exit (the child's
     stamps); the client's 1000th update handled; `Cmd.Wait`'s return; `Wait`'s return.
   * `TestOutputTapVerbatim`: the drain's start in `drainTaps`; each tap `Write`'s start and end, with its size; the drain's end.
   * `TestCloseReadsTheAgentsLastOutput`: the go notification sent; each held read, with its size; the agent's exit (the child's
     stamp); `done`; `Close`'s return.
   * Under question 5's answer (a) or (b), `closecontext_test.go:156` the same way: the release, the exit, `Wait`'s return.
2. **Two loads**, each test 20 runs under each, `-race`: beside the whole module's `go test -race -shuffle=on ./...`, as CI's step
   runs; and the in-process load (32 busy goroutines, `GOMAXPROCS=2`) started from the scratch test binary's own start, by an
   `init` in a scratch-only file when `BURN` is set, so that it lasts the whole run and no test runs outside it.
3. **The figure:** per bound, a multiple of the worst measured over both loads, about five times as the first decision's figure is,
   rounded up to a whole second or to `slack`'s multiple, and written with its arithmetic.
4. **The margin:** a scratch plant delaying the waited thing so that it ends just inside the new bound (at 0.8 of it) must pass, and
   just outside (at 1.2 of it) must fail.
5. **The defect kept:** the MADR's W2 break for each test, planted in a scratch copy with the new figure, must fail the test.
6. **The stop (W4):** a bound for which steps 3 to 5 cannot all hold goes to the coordinator for the owner, with its measurement, and
   phase 2 waits for the owner's word on it.
7. Record, per bound: the parts, their median, p90 and worst per load, the figure, the margin's two results and the break's.

**Verification:** each stamp seen in the runs; for each bound, the margin's two results and the break's failure recorded, with the
message. **Acceptance:** a figure for each of W2's four bounds, or its stop sent to the owner; the coordinator's review before phase 2.

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

*Added 2026-11-16, after phase 1's audit (the owner's rulings and the coordinator's applications, in the rule's addition above):*

* **What moves.** The class A and B tests phase 1's table marks "yes"; the four tests of the MADR's W1, to `exitGrace`; the four
  bounds of W2, to phase 1b's figures: `TestNoLostOutput`'s and `TestTapErrorIgnored`'s `within` bounds, `tappedFrames`' grace (which
  `TestInputTapSeesWrites` shares, a raise the rule allows) and `TestCloseReadsTheAgentsLastOutput`'s grace and its `within` bound.
  Tests already under `slack` or more keep their grace, but where phase 1b's figure raises it.
* **Bounds in graces:** `closecontext_test.go:145`, `:180` and `:246` scale with their tests' grace; `:214` stays.
* **A plant that fits each moved test**, seen to fail at `HEAD` and pass amended, the client-side plant above serving only the tests
  whose outcome waits the first step: for the twelve whose outcome has `WaitDelay` and no first step, a standard-error copy that
  outlives `WaitDelay`; for W1's four, a late `Serve` return or a late handler, as fits each one's wait; for W2's four, phase 1b's
  margin plants. A test no plant can fail at `HEAD` does not move, and is reported.
* **The load covers every moved test:** the A/B runs under the in-process load started from the test binary's start (phase 1b,
  step 2), not as a parallel test of a fixed length. Its "0 of 10 amended" applies to every test this phase moves, W1's four and
  W2's four included.
* **The breaks:** "the first step waits 0 s" fails each class A test whose outcome waits the first step; W2's break fails each of its
  four tests; each amended.

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

*Added 2026-11-16, after phase 1's audit:* the docs and the `CHANGELOG.md` line also name the rescaled bounds of the MADR's W2, with
the figures phase 1b gave them, beside the grace.

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
*Added 2026-11-16, after phase 1's audit:* the tests this PLAN moved include W1's four and W2's four; the tests counted apart are the
two 56 MiB tests and `TestBlockedTapDetached`'s readiness failure ("Where the race gates run", its addition).

### Phase 5: the 56 MiB tests' 30 s, by question 3's answer (mid tier, high effort)

*Annotated 2026-11-16, by the approval:* this phase runs last, after CI is green on the fix (phase 4, step 5), not earlier and not
in parallel with phases 1 to 4. As first written, nothing here fixed its place; it needs no change of the tree the other phases make.

1. Measure `TestDetachedTapIsPrefix` and `TestInProcessDetachedTapIsPrefix` on the Linux host: the time to the 56 MiB under `-race`,
   alone and beside the module, and what it is spent on.
2. Write `docs/reports/0057-REPORT-<slug>.md`: what was measured, decided nothing; a row in `docs/README.md`.

*Added 2026-11-16, after phase 1's audit:* the report also measures `TestBlockedTapDetached`'s time to be ready (its failures were
all "the bulk agent did not say it was ready within 30s", `tap_test.go:294`) and `TestDetachedTapIsPrefix`'s failure at its 5 s grace
in the audit's round 1 (`tap_test.go:349`). `TestTapErrorIgnored` is no longer the report's: its fixed wait is phase 1b's. Under
question 5's answer (c), `closecontext_test.go:156` is the report's too.

**Acceptance:** the REPORT committed and put to the owner, who then decides what follows, as the answer "A measured report, then
decide" says; any defect it finds left for a record of its own. Until that decision, the race gates run as "Where the race gates
run" says.

## Verification

The phases' checks, together: the plants and breaks fail and pass as required; the A/B under load; 20 rounds on the Linux host,
judged on the moved tests with the F7 tests counted apart; the module's race gate on this Mac; `tools/gates.py --all`; CI's six
jobs. Each new check is seen to fail on a deliberately broken input in a scratch copy before it is
relied on, and the record says how it failed.

*Added 2026-11-16, after phase 1's audit:*

| Tests | Seen to fail at `HEAD` by | Passes amended | Its defect still caught by | Load proof | Phase |
| :--- | :--- | :--- | :--- | :--- | :--- |
| class A, outcome waits the first step | the client-side plant, 350 ms | yes | "the first step waits 0 s" | A/B, 0 of 10 amended | 2 |
| class A, `WaitDelay` only (the twelve) | a standard-error copy that outlives `WaitDelay` | yes | the same plant at the new `WaitDelay`, past it | A/B, 0 of 10 amended | 2 |
| class B | the client-side plant in the second step | yes | its floor and its bound in graces | A/B, 0 of 10 amended | 2 |
| W1's four | a late `Serve` return or handler, as fits | yes | its own assertion's plant, past `exitGrace` | A/B, 0 of 10 amended | 2 |
| W2's four | phase 1b's margin plant, past the bound | yes, at 0.8 of the bound | W2's break | measured in 1b; A/B, 0 of 10 amended | 1b, 2 |
| all moved tests | - | - | - | 20 rounds on the Linux host, F7's counted apart; the module race gate on this Mac | 4 |

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

### Question after phase 1's audit, open (added 2026-11-16)

5. **`closecontext_test.go:156`, a fixed 3 s `Wait` (the MADR's G7).** It failed once, in the audit's round 3, beside its subtest's
   grace failure, which the bound scaling moves; whether it fails by itself is not known.
   * (a) Recommended: measure it in phase 1b with W2's four, and rescale it only if it fails once its subtest's grace and bound have
     moved. Cost: one more measurement in phase 1b.
   * (b) Rescale it now as a fifth bound of W2. Cost: a figure for a bound that may not need one.
   * (c) Leave it to phase 5's report. Cost: a bound the load exposed left outside the plan the owner's ruling made for them.

   Phase 1b's step for it runs only on answer (a) or (b); the rest of phase 1b does not depend on it.

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

### 2026-11-16: phase 1 (the audit)

Run by a mainstream-tier model at high effort, as the phase prices it, on the coordinator's brief. `date +%F` printed 2026-11-16, the
date of the drafting and the approval. Nothing in the tree changed but this entry and the PLAN's row in `docs/README.md`.

* **Approval.** The owner's ruling, relayed by the coordinator and recorded in the approval entry above: "Run phases 1 to 4 through;
  then the report". The coordinator reviews and commits this phase before phase 2 is briefed ("Order and review").
* **Base commit.** `51c5fd4`, `main`, clean. Every line number below is that tree's, and each citation in the table was checked
  against it by the script described under "How the list was proved complete".
* **Read:** this PLAN and its MADR in full; every `agentproc/*_test.go`; `agentproc/agentproc.go`, `inprocess.go` and `tap.go`;
  `AGENTS.md`, `tools/README.md`; the skills `madr-and-plan-writing` and `available-dev-hosts`.
* **Ran, this Mac,** in a scratch copy made by `git archive HEAD`, without `-race`: `go test -list '.*' ./agentproc/` (67 names);
  `go test -v -count=1 ./agentproc/` (102 passes, 0 failures); `GOOS=windows`, `GOOS=freebsd` and `GOOS=linux go vet ./agentproc/`,
  each exit 0, so the platform-specific files compile.
* **Ran, the Linux host,** the five rounds below, each through `run-on-host --need linux --race`, one heavy run at a time, in
  chunks of one round (about 2 to 4 minutes with the queue's turn), never a WSL distro.

#### The mechanisms a grace bounds

The rule's letters name the exit seen within `Close`'s first step and within `WaitDelay`. Reading `agentproc.go` and `inprocess.go`
at `51c5fd4`, six waits are bounded by a grace, and the table's "why" cells name them:

| Code | The wait | Where |
| :--- | :--- | :--- |
| M1 | `Close`'s first step: the exit seen within the grace, else SIGTERM and `ErrTerminated` | `agentproc.go:658` |
| M2 | its second step: the exit seen within the grace after SIGTERM, else the kill and `ErrKilled` | `agentproc.go:659` |
| M3 | `WaitDelay`, half the grace: `Cmd.Wait` returns `exec.ErrWaitDelay` when the standard-error copy is not done within it of the exit | `agentproc.go:228`-`:230` |
| M4 | the watcher: the connection reads to the end of the agent's output within the grace of the exit, else it is closed | `agentproc.go:368`-`:375` |
| M5 | `Wait`'s tap drain: each tap written within the grace, else abandoned | `agentproc.go:528`-`:547` |
| M6 | an `InProcess` agent after the kill: `Serve` returned within the grace, else `ErrAbandoned` | `inprocess.go:152`-`:158` |

#### How the list was built

Mechanically first, then each row read. A `go/ast` program, with `go/build` for each of five GOOS (linux, darwin, freebsd, windows,
plan9), lists every `Test*`, `Example*` and `t.Run` call of `agentproc`'s test files: 73 tests, 6 examples and 25 `t.Run` calls, of
which 21 have a literal name and 4 a computed one (`TestStartRefuses`, `TestCloseContextBoundsHandlers`,
`TestCloseContextHurriesTheSteps`, `TestWaitErrorSaysTheCauseOnce`; their 5, 4, 2 and 5 subtests are the names the runs print). That
is 100 rows from the source and 16 from the computed names: **116 rows, 104 leaves and 12 containers** (a test whose body is
`t.Run` calls). For each test the same program lists the bounds it states: every `within(t, d, …)`, `time.After`, deadline and
context timeout in its body, and the helpers it starts through.

#### How the list was proved complete

`check_table.py` (scratch) reads the table and fails unless: every test, example and subtest the source declares is one row, once;
every row is one of them, at the line the source gives and for the platforms its build constraints give; the names `go test -list`
prints on this Mac (67) and on the Linux host (71) are rows, and the rows built for that GOOS are the names listed; the names every
`-v` run printed, this Mac's and the five rounds', are rows, and the rows built for that GOOS were run; and each cited line holds the
cited text. Three examples with no `Output` comment are compiled and never run, so no list or run prints them; the 7 Windows-only
rows come from the source alone and compile. It printed "TABLE CHECK OK: 116 rows (104 leaves, 12 containers)".

Seen to fail, each on a scratch copy of the table: a row removed (`TestKillAfterExit`): "in the source, not in the table:
TestKillAfterExit (kill_test.go:54)", and the list and run checks named it too; a row duplicated: "duplicate row:
TestKillCutsCloseShort x2"; one of the five computed-name subtests removed: "ran but not in the table
['TestStartRefuses/Stdout_set']" in each run; a wrong cited line: "agentproc_test.go:177 does not hold 'Wait = %v, want nil'"; a row
for no test: "in the table, not in the source: TestNoSuchTest". The enumerating program was seen to list a test and a subtest added to
a scratch copy of the sources (106 entries against 104: `TestPlantedUnlisted` and `TestPlantedUnlisted/planted_sub`).

#### How the rule was read

* A test is classed by the assertion that decides it, by the rule's table, from the code. Where the deciding wait is none of M1 to M4,
  the rule has no letter, and the row is "unclear", not forced.
* The rule's last column says A and B tests run under `exitGrace`. Nine of the A and B tests already run under 3 s or more (`slack`,
  `descendantGrace`, 5 s, the default): `exitGrace` would shorten their grace. The table's "Moves" column says so: "yes" where the
  grace is `testGrace`, "no: grace already >= exitGrace" where it is not; C and D rows do not move.
* A test whose assertion is a bound of its own, `within(t, slack, …)` or a count of milliseconds, with no wait of the product's
  grace in its outcome, is D: a grace does not move it.
* Where a bound is stated in graces (`const bound = testGrace`, `ctx` timeouts of `testGrace`), it moves with the grace
  (phase 2, step 3); the row says so.

#### The table

"Built for": all is every GOOS; unix is linux, darwin and freebsd. "Grace" is the grace the test's agent runs under, `testGrace`
being 300 ms and `slack` 3 s. "Seen to fail" is of the five rounds below, on the Linux host; the Windows-only rows were not run.

| Test | Defined | Built for | Grace | Agent or `Serve` | Bounds the test states | Deciding assertion, and why | Class | Moves | Seen to fail, 5 rounds |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :---: | :---: | :--- |
| `TestStartAbandonsUnattachedAgent` | `abandon_test.go:23` | all | testGrace, never reached (Start fails) | beat | a 300 ms sleep | `abandon_test.go:45` no Close, Wait or step: Start's abandon is synchronous, and the 300 ms is a sleep over a file's size, not a grace | D | no | not seen to fail |
| `TestStartRefuses` | `agentproc_test.go:121` | all | - | - | - | a container: its subtests are the rows after it | - | - | - |
| `TestStartRefuses/nil_Cmd` | `agentproc_test.go:143` | all | none: Start refuses | none | none | `agentproc_test.go:150` Start refuses before any agent runs | D | no | not seen to fail |
| `TestStartRefuses/nil_client` | `agentproc_test.go:143` | all | none: Start refuses | none | none | `agentproc_test.go:150` Start refuses before any agent runs | D | no | not seen to fail |
| `TestStartRefuses/already_started` | `agentproc_test.go:143` | all | none: Start refuses | none (an eof child, started by os/exec itself) | none | `agentproc_test.go:150` Start refuses before any agent runs | D | no | not seen to fail |
| `TestStartRefuses/Stdin_set` | `agentproc_test.go:143` | all | none: Start refuses | none | none | `agentproc_test.go:150` Start refuses before any agent runs | D | no | not seen to fail |
| `TestStartRefuses/Stdout_set` | `agentproc_test.go:143` | all | none: Start refuses | none | none | `agentproc_test.go:150` Start refuses before any agent runs | D | no | not seen to fail |
| `TestNoLostOutput` | `agentproc_test.go:162` | all | testGrace (300 ms) | burst child | within(slack, "Wait"); `start`: ready wait 30 s | `agentproc_test.go:176` Wait nil needs the clean exit's standard-error copy done within WaitDelay (M3), and :178 the 1000 updates read within the grace of the exit (M4); in round 5 it failed 'Wait took longer than 3s' (:174), the absolute within(slack), F7's kind, which a grace does not move | A | yes | failed in round 5 |
| `TestCloseLadder` | `agentproc_test.go:188` | all | - | - | - | a container: its subtests are the rows after it | - | - | - |
| `TestCloseLadder/exits_at_its_input's_end` | `agentproc_test.go:190` | all | testGrace (300 ms) | eof child | within(slack, "Close"); `start`: ready wait 30 s | `agentproc_test.go:194` Close nil and under one grace: the exit seen within the first step (M1) | A | yes | failed in round 3 |
| `TestCloseLadder/exits_on_SIGTERM` | `agentproc_test.go:198` | all (the unix branch; windows takes :203, class C) | testGrace (300 ms) | term child | within(testGrace*2+slack, "Close"); `start`: ready wait 30 s | `agentproc_test.go:209` waits the first step out, then needs the exit on SIGTERM seen within the second (M2): ErrTerminated and not ErrKilled, under two graces | B | yes | failed in round 3 |
| `TestCloseLadder/ignores_both` | `agentproc_test.go:213` | all | testGrace (300 ms) | stubborn child | within(testGrace*2+slack, "Close"); `start`: ready wait 30 s | `agentproc_test.go:221` waits both graces out; asserts ErrKilled and a floor | C | no | not seen to fail |
| `TestStderrHeldByDescendant` | `agentproc_test.go:234` | all | descendantGrace (6 s) | parent-stderr child | within(descendantGrace*3+slack, "Close"); `startTapped`: ready wait 30 s | `agentproc_test.go:242` Close = exec.ErrWaitDelay and no signal: the exit seen within the first step after WaitDelay (M1, M3) | A | no: already 2*slack (0027-PLAN step 9) | not seen to fail |
| `TestDefaultGrace` | `agentproc_test.go:250` | all | default 5 s (Grace 0 and -1 s) | eof child | none | `agentproc_test.go:258` asserts the grace's value and WaitDelay's; Close nil at :262 is incidental, at 5 s | D | no | not seen to fail |
| `TestCloseIdempotent` | `agentproc_test.go:281` | all | testGrace (300 ms) | stubborn child | `start`: ready wait 30 s | `agentproc_test.go:291` waits both graces out; asserts the same ErrKilled from concurrent calls | C | no | not seen to fail |
| `TestClientDeathEndsAgentAtInputEnd` | `agentproc_unix_test.go:22` | unix | default 5 s (the owner child's Start) | owner child, eof agent | `killOwner`: a 30 s ready wait; `requireGone`: 2 s | `agentproc_unix_test.go:82` the agent ends at its input's end after its client is killed; the bound is 2 s, absolute, with no Close | D | no | not seen to fail |
| `TestEscapedDescendantDoesNotHoldWait` | `agentproc_unix_test.go:92` | unix | testGrace (300 ms) | parent-escape child | within(testGrace+slack, "Wait"); `startBeating`: ready wait 30 s, a 400 ms beat window | `agentproc_unix_test.go:103` Wait nil of an agent that exits of itself: the copy done within WaitDelay (M3); it also waits the grace out for the held output (M4) | A | yes | not seen to fail |
| `TestTapKeepsCloseEnding` | `agentproc_unix_test.go:120` | unix | testGrace (300 ms) | parent-escape child | within(testGrace+slack, "Wait"); `startTapped`: ready wait 30 s | `agentproc_unix_test.go:135` as TestEscapedDescendantDoesNotHoldWait, with an output tap | A | yes | not seen to fail |
| `TestKillClosesAtOnce` | `agentproc_unix_test.go:144` | unix | default 5 s (Grace 0) | parent-escape child | within(slack, "Kill"); `startTapped`: ready wait 30 s | `agentproc_unix_test.go:163` Kill, with no step waited; the bound is slack, absolute, shorter than the grace so that waiting it out would fail | D | no | not seen to fail |
| `TestStartRefusesSharedGroup` | `agentproc_unix_test.go:170` | unix | none: Start refuses | none (eof child, not started) | none | `agentproc_unix_test.go:175` Start refuses before any agent runs | D | no | not seen to fail |
| `TestCloseContextBoundsHandlers` | `closecontext_test.go:125` | all | - | - | - | a container: its subtests are the rows after it | - | - | - |
| `TestCloseContextBoundsHandlers/in_process,_CloseContext` | `closecontext_test.go:142` | all | testGrace (300 ms); ctx bound = testGrace (:145) | InProcess, acp.ServeAgent over a ScriptAgent | ctx timeout bound; within(bound+testGrace+slack, m.name); within(slack, "Wait"); `hang`: ready wait 30 s | `closecontext_test.go:159` Wait nil after the release, as a CloseContext's: the exit seen within the first step (M1), which the bound, testGrace, also cuts short (agentproc.go:594); the bound stands in graces and moves with the grace | A | yes | not seen to fail |
| `TestCloseContextBoundsHandlers/in_process,_KillContext` | `closecontext_test.go:142` | all | testGrace (300 ms); ctx bound = testGrace (:145) | InProcess, acp.ServeAgent over a ScriptAgent | ctx timeout bound; within(bound+testGrace+slack, m.name); within(slack, "Wait"); `hang`: ready wait 30 s | `closecontext_test.go:149` kill is the expected path, so no exit need be seen in a step (not A) and no grace is waited out (not C); yet ErrHandlersRunning (:149) is joined only if Wait's goroutine reached the connection's wait before the bound, testGrace (agentproc.go:644), after Kill's abandon wait (M6): a grace-linked outcome the rule has no letter for | unclear | undecided | not seen to fail |
| `TestCloseContextBoundsHandlers/a_child,_CloseContext` | `closecontext_test.go:142` | all | testGrace (300 ms); ctx bound = testGrace (:145) | burst child | ctx timeout bound; within(bound+testGrace+slack, m.name); within(slack, "Wait"); `hang`: ready wait 30 s | `closecontext_test.go:159` Wait nil after the release: neither ErrKilled nor exec.ErrWaitDelay is accepted for a CloseContext (M1, M3) | A | yes | failed in round 3 |
| `TestCloseContextBoundsHandlers/a_child,_KillContext` | `closecontext_test.go:142` | all | testGrace (300 ms); ctx bound = testGrace (:145) | burst child | ctx timeout bound; within(bound+testGrace+slack, m.name); within(slack, "Wait"); `hang`: ready wait 30 s | `closecontext_test.go:159` Wait after the release may be nil or ErrKilled, but not exec.ErrWaitDelay: the standard-error copy done within WaitDelay of the exit (M3) | A | yes | not seen to fail |
| `TestCloseContextJoinsTheAgentsError` | `closecontext_test.go:174` | all | testGrace (300 ms); ctx bound = testGrace (:180) | InProcess, ServeAgent then errServe | ctx timeout testGrace; within(2*testGrace+slack, "CloseContext"); `hang`: ready wait 30 s | `closecontext_test.go:184` the error equals the join with errServe alone, so no SIGTERM: the exit seen within the first step (M1) | A | yes | not seen to fail |
| `TestCloseContextWaitsForHandlers` | `closecontext_test.go:192` | all | testGrace (300 ms); ctx 5 s | InProcess, ServeAgent over a ScriptAgent | ctx timeout 5*time.Second; within(5*time.Second+slack, "CloseContext"); `hang`: ready wait 30 s | `closecontext_test.go:200` CloseContext nil: the exit seen within the first step (M1), the handler released at testGrace/2 | A | yes | not seen to fail |
| `TestCloseContextHurriesTheSteps` | `closecontext_test.go:212` | all | - | - | - | a container: its subtests are the rows after it | - | - | - |
| `TestCloseContextHurriesTheSteps/CloseContext` | `closecontext_test.go:223` | all | 5 s; ctx bound = testGrace (:214) | stubborn child | ctx timeout bound; within(3*grace+slack, m.name); `startTapped`: ready wait 30 s | `closecontext_test.go:229` the context's end cuts the steps short; asserts ErrKilled without ErrTerminated and a floor, the bound | C | no | not seen to fail |
| `TestCloseContextHurriesTheSteps/KillContext` | `closecontext_test.go:223` | all | 5 s; ctx bound = testGrace (:214) | stubborn child | ctx timeout bound; within(3*grace+slack, m.name); `startTapped`: ready wait 30 s | `closecontext_test.go:229` the context's end cuts the steps short; asserts ErrKilled without ErrTerminated and a floor, the bound | C | no | not seen to fail |
| `TestCloseContextCause` | `closecontext_test.go:242` | all | testGrace (300 ms); ctx bound = testGrace (:246) | InProcess, ServeAgent over a ScriptAgent | ctx timeout testGrace; within(2*testGrace+slack, "CloseContext"); `hang`: ready wait 30 s | `closecontext_test.go:249` asserts the cause and ErrHandlersRunning only, so no exit need be seen in a step; ErrHandlersRunning needs Wait's goroutine inside the connection's wait before the bound, testGrace (agentproc.go:644): grace-linked, no letter | unclear | undecided | not seen to fail |
| `TestCloseContextNotHandlers` | `closecontext_test.go:259` | all | 2 s; ctx bound 100 ms (:282) | burst child, blocked tap | ctx timeout readyWait; ctx timeout bound; within(2*time.Second+slack, "CloseContext") | `closecontext_test.go:290` Wait nil after the release: the exit seen already (:278), so no step is waited; the copy done within WaitDelay, 1 s (M3) | A | decide: grace 2 s, below exitGrace, is the drain wait the 100 ms bound races | not seen to fail |
| `TestCloseContextOneGoroutine` | `closecontext_test.go:298` | all | testGrace (300 ms) | InProcess, ServeAgent over a ScriptAgent | deadline +slack; `hang`: ready wait 30 s | `closecontext_test.go:310` asserts a count of goroutines, and that the waiter ends within slack of the release; no step's outcome | D | no | not seen to fail |
| `TestInProcessEndsAtInputEnd` | `inprocess_test.go:86` | all | 5 s | InProcess, a Serve that ends at its input's end | within(slack, "Close") | `inprocess_test.go:96` Serve's own error alone, no signal: the exit seen within the first step (M1); :99 adds an absolute 1 s | A | no: grace already >= exitGrace | not seen to fail |
| `TestInProcessTerminate` | `inprocess_test.go:106` | all | testGrace (300 ms) | InProcess, Serve waits for its context | within(2*testGrace+slack, "Close") | `inprocess_test.go:115` waits the first step out, then needs Serve's return after the cancel seen within the second (M2): ErrTerminated and not ErrKilled; and a floor at :121 | B | yes | not seen to fail |
| `TestInProcessKill` | `inprocess_test.go:129` | all | testGrace (300 ms) | InProcess, readUntilFailure | within(3*testGrace+slack, "Close") | `inprocess_test.go:133` waits two steps out, then needs Serve's return after the kill seen within the next, the abandonment wait (M6): ErrKilled and not ErrAbandoned | B | yes | not seen to fail |
| `TestInProcessAbandon` | `inprocess_test.go:142` | all | testGrace (300 ms) | InProcess, Serve blocked until released | within(3*testGrace+slack, "Close"); time.After(slack) | `inprocess_test.go:155` waits three graces out; asserts ErrKilled and ErrAbandoned and a floor of three graces | C | no | not seen to fail |
| `TestInProcessKillAtOnce` | `inprocess_test.go:177` | all | 5 s | InProcess, readUntilFailure | within(slack, "Kill") | `inprocess_test.go:181` Kill waits no step, and needs Serve's return after the kill seen within the abandonment wait (M6), here 5 s, and within 1 s absolute: B's shape without the waits, which the rule does not name | unclear | no: grace already >= exitGrace | not seen to fail |
| `TestWaitErrorSaysTheCauseOnce` | `inprocess_test.go:200` | all | - | - | - | a container: its subtests are the rows after it | - | - | - |
| `TestWaitErrorSaysTheCauseOnce/Kill,_Serve_returning_its_cause` | `inprocess_test.go:227` | all | testGrace (300 ms) | InProcess, causeServe | within(3*testGrace+slack, "the stop") | `inprocess_test.go:235` Kill waits no step, and the exact text 'agentproc: agent killed' needs Serve's return within the abandonment wait (M6); with ErrAbandoned joined the text differs: B's shape without the waits | unclear | undecided | not seen to fail |
| `TestWaitErrorSaysTheCauseOnce/step_2,_Serve_returning_its_cause` | `inprocess_test.go:227` | all | testGrace (300 ms) | InProcess, causeServe | within(3*testGrace+slack, "the stop") | `inprocess_test.go:235` waits the first step out, then needs Serve's return seen within the second: the text 'agent terminated' and ErrTerminated, not a kill's | B | yes | not seen to fail |
| `TestWaitErrorSaysTheCauseOnce/step_2,_Serve_wrapping_its_cause` | `inprocess_test.go:227` | all | testGrace (300 ms) | InProcess, a Serve wrapping its cause | within(3*testGrace+slack, "the stop") | `inprocess_test.go:235` as above | B | yes | not seen to fail |
| `TestWaitErrorSaysTheCauseOnce/step_2,_Serve_returning_its_own_error` | `inprocess_test.go:227` | all | testGrace (300 ms) | InProcess, a Serve returning errServe | within(3*testGrace+slack, "the stop") | `inprocess_test.go:235` as above | B | yes | not seen to fail |
| `TestWaitErrorSaysTheCauseOnce/abandoned` | `inprocess_test.go:227` | all | testGrace (300 ms) | InProcess, Serve blocked until released | within(3*testGrace+slack, "the stop") | `inprocess_test.go:235` waits three graces out; the expected text is the abandonment's | C | no | not seen to fail |
| `TestWaitErrorSaysTheCauseOnce/a_child,_killed` | `inprocess_test.go:240` | all | testGrace (300 ms) | stubborn child | within(2*testGrace+slack, "Close"); `start`: ready wait 30 s | `inprocess_test.go:244` waits both graces out; asserts ErrKilled said once, first | C | no | not seen to fail |
| `TestInProcessReturnsFirst` | `inprocess_test.go:253` | all | - | - | - | a container: its subtests are the rows after it | - | - | - |
| `TestInProcessReturnsFirst/Serve_first` | `inprocess_test.go:255` | all | testGrace (300 ms) | InProcess, Serve returns nil | within(testGrace+slack, "Wait") | `inprocess_test.go:261` no Close before Wait: Serve returned of itself, and no step or WaitDelay applies to an InProcess agent; ExitedFirst is set at its return (agentproc.go:359-363) | D | no | not seen to fail |
| `TestInProcessReturnsFirst/connection_first` | `inprocess_test.go:265` | all | testGrace (300 ms) | InProcess, Serve copies its input to Discard | within(testGrace+slack, "Wait") | `inprocess_test.go:275` Wait nil after the connection's close: monitor runs Close's steps, and the exit must be seen within the first (M1), else ErrTerminated | A | yes | not seen to fail |
| `TestInProcessClientWriteAfterReturn` | `inprocess_test.go:287` | all | testGrace (300 ms) | InProcess, Serve returns nil | time.After(slack); within(slack, "Initialize") | `inprocess_test.go:299` a write after Serve's return fails: bounds are slack, absolute; no step | D | no | not seen to fail |
| `TestInProcessEndClosesInput` | `inprocess_test.go:310` | all | testGrace (300 ms) | InProcess, Serve returns nil | within(testGrace+slack, "Wait"); within(slack, "a read of the agent's input") | `inprocess_test.go:324` no Close before Wait; asserts which end of a pipe was closed | D | no | not seen to fail |
| `TestInProcessTaps` | `inprocess_test.go:333` | all | 5 s | InProcess, Serve writes 64 MiB | none | `inprocess_test.go:353` no Close before Wait, and no exit to be seen; the outcome is the tap drained within the grace (M5, agentproc.go:528-547), which the rule has no letter for | unclear | no: grace already >= exitGrace | not seen to fail |
| `TestInProcessServeAgentStep2` | `inprocess_test.go:383` | all | slack (3 s) | InProcess, acp.ServeAgent, a prompt in flight | time.After(slack); within(3*slack+slack, "Close") | `inprocess_test.go:410` waits the first step out (the prompt holds Serve), then needs ServeAgent's return seen within the second: ErrTerminated and not ErrKilled | B | no: already slack (0049 phase 3) | not seen to fail |
| `TestInProcessDetachedTapIsPrefix` | `inprocess_test.go:437` | all | 5 s | InProcess, Serve writes 56 + 8 MiB | time.After(readyWait); deadline +readyWait | `inprocess_test.go:473` Close nil: the last 8 MiB written, and so Serve's return, within the first step, 5 s (M1); a throughput bound under -race, F7 | A | no: grace already >= exitGrace; F7 | failed in round 3, 4, 5 |
| `TestInProcessStartRefuses` | `inprocess_test.go:487` | all | none: Start refuses | none | none | `inprocess_test.go:500` Start refuses before any Serve runs | D | no | not seen to fail |
| `TestKill` | `kill_test.go:16` | all | default 5 s (Grace 0) | parent-wait child | within(slack, "Kill"); `startTapped`: ready wait 30 s | `kill_test.go:24` Kill waits no step: ErrKilled alone, within slack, absolute, shorter than the grace so that waiting it out would fail | D | no | not seen to fail |
| `TestKillCutsCloseShort` | `kill_test.go:35` | all | 5 s | stubborn child | within(slack, "Kill"); time.After(slack); `startTapped`: ready wait 30 s | `kill_test.go:44` Kill cuts a Close short: ErrKilled from both, within slack, absolute | D | no | not seen to fail |
| `TestKillAfterExit` | `kill_test.go:54` | all | testGrace (300 ms) | parent-exit child | time.After(readyWait); within(testGrace+slack, "Kill"); `startTapped`: ready wait 30 s | `kill_test.go:64` Kill and Wait nil after an exit of itself already seen: nothing is sent, so only WaitDelay (M3) can make Wait's error not nil | A | yes | not seen to fail |
| `TestLineTapFrames` | `linetap_test.go:48` | all | - | - | - | a container: its subtests are the rows after it | - | - | - |
| `TestLineTapFrames/whole` | `linetap_test.go:63` | all | none: no agent | none | none | `linetap_test.go:59` a LineTap alone, no agent or step | D | no | not seen to fail |
| `TestLineTapFrames/a_byte_at_a_time` | `linetap_test.go:64` | all | none: no agent | none | none | `linetap_test.go:59` a LineTap alone, no agent or step | D | no | not seen to fail |
| `TestLineTapFrames/every_split` | `linetap_test.go:65` | all | none: no agent | none | none | `linetap_test.go:59` a LineTap alone, no agent or step | D | no | not seen to fail |
| `TestLineTapCopies` | `linetap_test.go:77` | all | none: no agent | none | none | `linetap_test.go:99` a LineTap alone | D | no | not seen to fail |
| `TestLineTapHoldsUnended` | `linetap_test.go:109` | all | none: no agent | none | none | `linetap_test.go:128` a LineTap alone | D | no | not seen to fail |
| `TestLineTapNilFn` | `linetap_test.go:135` | all | none: no agent | none | none | `linetap_test.go:139` LineTap(nil) panics | D | no | not seen to fail |
| `TestLineTapAsOutputTap` | `linetap_test.go:148` | all | testGrace (300 ms) | frames child | `startTapped`: ready wait 30 s | `linetap_test.go:153` Close nil: the exit seen within the first step (M1); the frames equal at :157 also need the tap drained within the grace (M5) | A | yes | failed in round 5 |
| `TestDefaultPdeathsig` | `pdeathsig_test.go:26` | linux/freebsd | testGrace (300 ms) | eof child | `startCmd`: ready wait 30 s | `pdeathsig_test.go:34` Close nil: the exit seen within the first step (M1) | A | yes | not seen to fail |
| `TestCallersPdeathsigKept` | `pdeathsig_test.go:39` | linux/freebsd | testGrace (300 ms) | eof child | `startCmd`: ready wait 30 s | `pdeathsig_test.go:48` Close nil: the exit seen within the first step (M1) | A | yes | failed in round 2 |
| `TestClientDeathEndsAgent` | `pdeathsig_test.go:56` | linux/freebsd | default 5 s (the owner child's Start) | owner child, stubborn agent | `killOwner`: a 30 s ready wait; `requireGone`: 2 s | `agentproc_unix_test.go:82` the agent ends at its client's death; the bound is 2 s, absolute | D | no | not seen to fail |
| `TestStartFromExitingLockedGoroutine` | `pdeathsig_test.go:66` | linux/freebsd | testGrace (300 ms) | stubborn child | time.After(readyWait); time.After(time.Second) | `pdeathsig_test.go:101` waits both graces out; asserts ErrKilled | C | no | not seen to fail |
| `TestTailWriterCaps` | `stderr_test.go:16` | all | none: no agent | none | none | `stderr_test.go:41` a tail writer alone | D | no | not seen to fail |
| `TestTailWriterForwards` | `stderr_test.go:49` | all | none: no agent | none | none | `stderr_test.go:58` a tail writer alone | D | no | not seen to fail |
| `TestStderrTailOfFailedAgent` | `stderr_test.go:87` | all | testGrace (300 ms) | stderr-fail child | `startStderrFail`: WaitContext within 30 s | `stderr_test.go:92` the tail and the caller's copy hold all 100 lines: the standard-error copy done within WaitDelay of the exit (M3), else os/exec closes the pipe and lines are lost | A | yes | not seen to fail |
| `TestStderrTailNotJoined` | `stderr_test.go:110` | all | testGrace (300 ms) | stderr-fail child | `startStderrFail`: WaitContext within 30 s | `stderr_test.go:116` the tail ends at line 100: the copy done within WaitDelay of the exit (M3) | A | yes | not seen to fail |
| `TestStderrTailOfKilledAgent` | `stderr_test.go:123` | all | testGrace (300 ms) | stubborn child | within(slack, "Kill"); `awaitReady`: ready wait 30 s | `stderr_test.go:136` Kill waits no step; the tail is the line written long before the kill | D | no | not seen to fail |
| `TestInProcessHasNoStderrTail` | `stderr_test.go:143` | all | testGrace (300 ms) | InProcess, Serve returns errServe | within(testGrace+slack, "Wait") | `stderr_test.go:146` no Close before Wait; an InProcess agent has no step or WaitDelay in this path | D | no | not seen to fail |
| `TestStderrErrorText` | `stderr_test.go:158` | all | none: no agent | none | none | `stderr_test.go:165` an error's text alone | D | no | not seen to fail |
| `TestOutputTapVerbatim` | `tap_test.go:215` | all | slack (3 s) (tappedFrames) | frames child, 1 ms a tap write | `tappedFrames`: grace slack, a 30 s wait for the input tap | `tap_test.go:218` Close nil (:205) is met at slack (M1); the outcome is the 2 MiB tap, 1 ms a write, drained within the grace (M5), which the rule has no letter for; in round 1 it failed at slack: 31 of 32 64 KiB writes done when the 3 s drain ended (tap.go:76 does not count the write in flight as dropped) | unclear | no: grace already >= exitGrace | failed in round 1 |
| `TestInputTapSeesWrites` | `tap_test.go:229` | all | slack (3 s) (tappedFrames) | frames child | `tappedFrames`: grace slack, a 30 s wait for the input tap | `tap_test.go:233` as TestOutputTapVerbatim: Close nil (:205) at slack, and the input tap drained within the grace (M5) | unclear | no: grace already >= exitGrace | not seen to fail |
| `TestTapErrorIgnored` | `tap_test.go:242` | all | testGrace (300 ms) | burst child, failing taps | within(slack, "Wait") | `tap_test.go:257` Wait nil of an exit of itself: the copy done within WaitDelay (M3), the updates read within the grace of the exit (M4); the failing taps' drain is M5; under the in-process load it failed 'Wait took longer than 3s' (:256), F7 (MADR) | A | yes | failed in round 5 |
| `TestTapsUnset` | `tap_test.go:266` | all | none: no agent | none | none | `tap_test.go:275` streams alone | D | no | not seen to fail |
| `TestBlockedTapDetached` | `tap_test.go:288` | all | testGrace (300 ms) | bulk child, 48 MiB, tap blocked | deadline +readyWait; within(testGrace+slack, "Close"); deadline +slack; `awaitReady`: ready wait 30 s | `tap_test.go:312` Close nil within TG+slack: the exit seen within the first step (M1), the blocked tap abandoned after the grace (M5); F7 when it fails 'did not say it was ready within 30s' (:294) | A | yes (its Close also waits the grace out, M5: about exitGrace a run) | failed in round 1, 2 |
| `TestDetachedTapIsPrefix` | `tap_test.go:330` | all | 5 s | bulk child, 56 + 8 MiB, tap blocked | deadline +readyWait; `awaitReady`: ready wait 30 s | `tap_test.go:349` Close nil: the last 8 MiB written, and the exit, within the first step, 5 s (M1); a throughput bound under -race, F7 | A | no: grace already >= exitGrace; F7 | failed in round 1 |
| `TestCloseReadsTheAgentsLastOutput` | `tap_test.go:368` | all | 5 s | last child | within(c.Grace+slack, "Close"); `awaitReady`: ready wait 30 s | `tap_test.go:398` Close nil within grace+slack: the exit seen within the first step (M1); the tap complete needs M4 and M5 within 5 s; in round 2 it failed at 5 s, 'agent terminated: signal: terminated': the agent's exit follows the connection's read of 2 MiB through a 5 ms read hook, a throughput as F7's (read, not measured) | A | no: grace already >= exitGrace | failed in round 2 |
| `TestTapDrainBoundedByGrace` | `tap_test.go:415` | all | testGrace (300 ms) | bulk child, 24 MiB, tap blocked | within(testGrace+slack, "Close"); `startTapped`: ready wait 30 s | `tap_test.go:425` Close nil: the exit seen within the first step (M1); then the drain waits the grace out and abandons the blocked tap (M5, the assertion at :427) | A | yes (its Close also waits the grace out, M5: about exitGrace a run) | not seen to fail |
| `TestDescendantsEnded` | `ties_test.go:47` | all (skips where hasTree is false) | - | - | - | a container: its subtests are the rows after it | - | - | - |
| `TestDescendantsEnded/the_agent_exits_at_its_input's_end` | `ties_test.go:52` | all (skips where hasTree is false) | testGrace (300 ms) | parent child, beat grandchild | within(testGrace+slack, "Close"); `startBeating`: ready wait 30 s, a 400 ms beat window | `ties_test.go:56` Close nil: the exit seen within the first step (M1); CI run 43875424670's failure | A | yes | failed in round 3 |
| `TestDescendantsEnded/the_agent_is_terminated` | `ties_test.go:62` | all (skips where hasTree is false) | testGrace (300 ms) | parent-wait child | within(2*testGrace+slack, "Close"); `startBeating`: ready wait 30 s, a 400 ms beat window | `ties_test.go:66` ErrTerminated or ErrKilled, either way | C | no | not seen to fail |
| `TestDescendantsEnded/the_agent_exits_of_itself` | `ties_test.go:72` | all (skips where hasTree is false) | testGrace (300 ms) | parent-exit child | within(testGrace+slack, "Wait"); `startCmd`: ready wait 30 s | `ties_test.go:84` Wait nil of an exit of itself: the copy done within WaitDelay (M3), and the connection closed after Wait (M4) | A | yes | not seen to fail |
| `TestExitedFirst` | `ties_test.go:103` | all | - | - | - | a container: its subtests are the rows after it | - | - | - |
| `TestExitedFirst/the_agent_exits_first` | `ties_test.go:128` | all | testGrace (300 ms) (50 runs) | parent-exit child | the shared `run` (`ties_test.go:105`): time.After(readyWait) for the exit; within(testGrace+slack, "Wait"); `startCmd`: ready wait 30 s | `ties_test.go:121` Wait nil (M3) and ExitedFirst true after a clean exit of itself | A | yes | not seen to fail |
| `TestExitedFirst/the_connection_ends_first` | `ties_test.go:132` | all | testGrace (300 ms) (50 runs) | closeout child | the shared `run` (`ties_test.go:105`): time.After(readyWait) for the exit; within(testGrace+slack, "Wait"); `startCmd`: ready wait 30 s | `ties_test.go:121` Wait nil after the connection ended first: monitor's Close needs the exit at the input's end seen within the first step (M1), and ExitedFirst false | A | yes | not seen to fail |
| `TestCallersCommandKept` | `ties_test.go:141` | all | testGrace (300 ms) | where child | `startCmd`: ready wait 30 s | `ties_test.go:149` Close nil: the exit seen within the first step (M1) | A | yes | not seen to fail |
| `TestConnectionEndStopsAgent` | `ties_test.go:182` | all | - | - | - | a container: its subtests are the rows after it | - | - | - |
| `TestConnectionEndStopsAgent/the_caller_closes_the_connection` | `ties_test.go:184` | all | testGrace (300 ms) | stubborn child | within(2*testGrace+slack, "Wait"); `start`: ready wait 30 s | `ties_test.go:190` waits both graces out; asserts ErrKilled | C | no | not seen to fail |
| `TestConnectionEndStopsAgent/a_notification_overflow` | `ties_test.go:194` | all | testGrace (300 ms) | flood child | time.After(testGrace + slack) | `ties_test.go:223` Wait nil: the exit at the input's end seen within the first step (M1) after the overflow closed the connection; the Done wait at :216 is TG+slack | A | yes | not seen to fail |
| `TestTranscriptLines` | `transcript_test.go:40` | all | none: no agent | none | none | `transcript_test.go:68` a Transcript alone | D | no | not seen to fail |
| `TestTranscriptWriteError` | `transcript_test.go:80` | all | none: no agent | none | none | `transcript_test.go:86` a Transcript alone | D | no | not seen to fail |
| `TestTranscriptOfSession` | `transcript_test.go:129` | all | testGrace (300 ms) | InProcess, acp.ServeAgent | none | `transcript_test.go:156` Close nil: ServeAgent's return seen within the first step (M1) | A | yes | not seen to fail |
| `TestTranscriptCloseAfterAbandon` | `transcript_test.go:219` | all | 100 ms (its own) | InProcess, Serve writes 5000 frames | none | `transcript_test.go:241` needs the tap still writing when the 100 ms drain ends (M5): it waits the grace out, and asserts the abandonment | C | no | not seen to fail |
| `TestProcessWaitContext` | `waitcontext_test.go:21` | all | - | - | - | a container: its subtests are the rows after it | - | - | - |
| `TestProcessWaitContext/ctx_ends_first` | `waitcontext_test.go:23` | all | testGrace (300 ms) | eof child | `start`: ready wait 30 s | `waitcontext_test.go:38` Close nil: the exit seen within the first step (M1) | A | yes | not seen to fail |
| `TestProcessWaitContext/the_agent_ends_first` | `waitcontext_test.go:44` | all | testGrace (300 ms) | parent-exit child | `startCmd`: ready wait 30 s | `waitcontext_test.go:50` WaitContext nil of an exit of itself (M3), ExitedFirst true | A | yes | not seen to fail |
| `TestProcessWaitContext/one_goroutine` | `waitcontext_test.go:56` | all | testGrace (300 ms) | eof child | deadline +slack; `start`: ready wait 30 s | `waitcontext_test.go:71` Close nil (a Fatalf guard): the exit seen within the first step (M1); the goroutine counts are no grace's | A | yes | not seen to fail |
| `ExampleCommand` | `example_test.go:27` | all | default 5 s | my-agent (no Output: compiled, not run) | none | `example_test.go:27` has no Output comment: compiled, never run | D | no | not run (no Output comment) |
| `ExampleLineTap` | `example_test.go:48` | all | default 5 s | my-agent (no Output: compiled, not run) | none | `example_test.go:48` has no Output comment: compiled, never run | D | no | not run (no Output comment) |
| `ExampleCommand_outputTap` | `example_test.go:64` | all | default 5 s | my-agent (no Output: compiled, not run) | none | `example_test.go:64` has no Output comment: compiled, never run | D | no | not run (no Output comment) |
| `ExampleInProcess` | `example_test.go:99` | all | default 5 s | InProcess, acp.ServeAgent over a ScriptAgent | none | `example_test.go:112` the output 'close: <nil>': the exit seen within the first step (M1), at the default 5 s | A | no: grace already >= exitGrace (a public example of the default) | not seen to fail |
| `ExampleProcess_CloseContext` | `example_test.go:122` | all | default 5 s; ctx 5 s | InProcess, acp.ServeAgent over a ScriptAgent | ctx timeout 5*time.Second | `example_test.go:137` the output 'close: <nil>': the exit seen within the first step (M1), at the default 5 s | A | no: grace already >= exitGrace (a public example of the default) | not seen to fail |
| `ExampleTranscript` | `example_test.go:146` | all | default 5 s | InProcess, acp.ServeAgent over a ScriptAgent | none | `example_test.go:163` Close nil: the exit seen within the first step (M1), at the default 5 s | A | no: grace already >= exitGrace (a public example of the default) | not seen to fail |
| `TestClientDeathEndsTree` | `agentproc_windows_test.go:14` | windows | default 5 s (the owner child's Start) | owner child, parent-wait agent | time.After(readyWait) | `agentproc_windows_test.go:33` the job ends the tree at its client's death: the bound is the absolute 400 x 25 ms of the child | D | no | not run (Windows only) |
| `TestBreakawayLeavesJob` | `breakaway_windows_test.go:45` | windows | - | - | - | a container: its subtests are the rows after it | - | - | - |
| `TestBreakawayLeavesJob/Close` | `breakaway_windows_test.go:47` | windows | testGrace (300 ms) | parent-breakaway child | the shared `breakaway` (`breakaway_windows_test.go:59`): within(testGrace+slack, "stop"); a 30 s ready wait | `breakaway_windows_test.go:79` Close nil (its ok func is err == nil): the exit at the input's end seen within the first step (M1) | A | yes | not run (Windows only) |
| `TestBreakawayLeavesJob/Kill` | `breakaway_windows_test.go:51` | windows | testGrace (300 ms) | parent-breakaway child | the shared `breakaway` (`breakaway_windows_test.go:59`): within(testGrace+slack, "stop"); a 30 s ready wait | `breakaway_windows_test.go:79` Kill waits no step: ErrKilled is the expected outcome | D | no | not run (Windows only) |
| `TestDescendantInJobDespiteSlowAttach` | `suspended_windows_test.go:25` | windows | testGrace (300 ms) | parent child, beat grandchild | within(testGrace+slack, "Close"); `startBeating`: ready wait 30 s, a 400 ms beat window | `suspended_windows_test.go:31` Close nil: the exit seen within the first step (M1), after a 200 ms attach delay | A | yes | not run (Windows only) |
| `TestStartRefusesSuspended` | `suspended_windows_test.go:60` | windows | none: Start refuses | none | none | `suspended_windows_test.go:65` Start refuses before any agent runs | D | no | not run (Windows only) |
| `TestCallersCreationFlagsKept` | `suspended_windows_test.go:72` | windows | testGrace (300 ms) | eof child | `startCmd`: ready wait 30 s | `suspended_windows_test.go:81` Close nil: the exit seen within the first step (M1) | A | yes | not run (Windows only) |

#### Counts

| Class | Leaves | Of which |
| :--- | ---: | :--- |
| A | 41 | 32 move to `exitGrace`; 8 no, grace already at least `slack`; 1 to decide (`TestCloseContextNotHandlers`, 2 s) |
| B | 7 | 6 move; 1 no (`TestInProcessServeAgentStep2`, already `slack`) |
| C | 11 | none moves |
| D | 38 | none moves |
| unclear | 7 | 3 at `testGrace`, 4 at a grace of `slack` or more |
| | 104 | and 12 containers: 116 rows |

**A and B together: 48. Of them, 38 run under `testGrace` and move, 9 do not move, and 1 is for the coordinator to decide.** Of the 48,
12 were seen to fail in the five rounds (4 of them F7's tests), 33 were not, and 3, Windows-only, were not run.

#### The chunks

Each round ran the whole package at `51c5fd4` in a scratch copy beside the MADR's in-process load, through the fleet's
`run-on-host --need linux --race`, one heavy run at a time on the host. `chunk.sh` ran, on the host,
`GOMAXPROCS=2 BURN=32 BURN_FOR=60s go test -race -shuffle=on -count=1 -v ./agentproc/`, and printed the seed. Round 1 also ran
`go test -list '.*' ./agentproc/` there (71 names). Each chunk's own exit status was 0; the test run's, printed by the script, was 1
in every round.

| Chunk | Command | Rounds | Seed | Result |
| :--- | :--- | ---: | :--- | :--- |
| R1 | `run-on-host --need linux --race -- sh chunk.sh 1` | 1 | 1794842628057943104 | FAIL in 109.312 s; failing tests 3, passes 104, skips 0 |
| R2 | `run-on-host --need linux --race -- sh chunk.sh 2` | 1 | 1794842855249088448 | FAIL in 104.753 s; failing tests 3, passes 104, skips 0 |
| R3 | `run-on-host --need linux --race -- sh chunk.sh 3` | 1 | 1794842976527828494 | FAIL in 108.824 s; failing tests 5, passes 99, skips 0 |
| R4 | `run-on-host --need linux --race -- sh chunk.sh 4` | 1 | 1794843102361850269 | FAIL in 107.601 s; failing tests 1, passes 106, skips 0 |
| R5 | `run-on-host --need linux --race -- sh chunk.sh 5` | 1 | 1794843224957491326 | FAIL in 105.634 s; failing tests 4, passes 103, skips 0 |

**The load.** The MADR's, rebuilt from its description (F3, F4: 32 busy goroutines in the test process, `GOMAXPROCS=2`, a scratch-only
test file): `zz_burn_test.go`, `TestZZBurn`, written afresh in the scratch copy and absent from the tree; it skips unless `BURN` is set,
calls `t.Parallel()`, and runs `BURN` goroutines in a busy loop for `BURN_FOR`. Its shape is that of the earlier scratch test of the
root-cause work, which was read before this one was written; `BURN_FOR` is this audit's, 60 s (the earlier run used 45 s). It passed in
60.1 s in each round and is excluded from every list above.

#### The failures of the five rounds

16 failing tests in all, none a container. Messages are the test's own, verbatim.

| Round | Seed | Test | Took | Message | Class |
| ---: | :--- | :--- | ---: | :--- | :---: |
| 1 | 1794842628057943104 | `TestDetachedTapIsPrefix` | 23.21 s | `tap_test.go:349: Close = agentproc: agent terminated: signal: terminated` | A, F7 |
| 1 | 1794842628057943104 | `TestBlockedTapDetached` | 31.96 s | `tap_test.go:294: the bulk agent did not say it was ready within 30s` | A, F7 |
| 1 | 1794842628057943104 | `TestOutputTapVerbatim` | 5.55 s | `tap_test.go:219: the output tap received 2031616 bytes, want the agent's 2097208, equal; tap_test.go:222: DroppedTapBytes = 56, 0; want 0, 0` | U |
| 2 | 1794842855249088448 | `TestBlockedTapDetached` | 31.99 s | `tap_test.go:294: the bulk agent did not say it was ready within 30s` | A, F7 |
| 2 | 1794842855249088448 | `TestCallersPdeathsigKept` | 0.91 s | `pdeathsig_test.go:48: Close = agentproc: agent terminated` | A |
| 2 | 1794842855249088448 | `TestCloseReadsTheAgentsLastOutput` | 6.58 s | `tap_test.go:398: Close = agentproc: agent terminated: signal: terminated, want nil: the agent exits of itself` | A |
| 3 | 1794842976527828494 | `TestInProcessDetachedTapIsPrefix` | 40.73 s | `inprocess_test.go:461: the connection did not read Serve's first 56 MiB within 30s` | A, F7 |
| 3 | 1794842976527828494 | `TestCloseLadder/exits_on_SIGTERM` | 1.35 s | `agentproc_test.go:210: Close = agentproc: agent terminated: exit status 3 after 1.013400313s, want ErrTerminated with exit status 3 between one and two graces` | B |
| 3 | 1794842976527828494 | `TestCloseLadder/exits_at_its_input's_end` | 0.87 s | `agentproc_test.go:195: Close = agentproc: agent terminated after 447.150099ms, want nil within the grace, 300ms` | A |
| 3 | 1794842976527828494 | `TestDescendantsEnded/the_agent_exits_at_its_input's_end` | 2.20 s | `ties_test.go:56: Close = agentproc: agent terminated, want nil` | A |
| 3 | 1794842976527828494 | `TestCloseContextBoundsHandlers/a_child,_CloseContext` | 5.24 s | `closecontext_test.go:150: CloseContext = context deadline exceeded; agentproc: agent killed after 855.162553ms, want the bound's cause and acp.ErrHandlersRunning; closecontext_test.go:156: Wait took longer than 3s` | A |
| 4 | 1794843102361850269 | `TestInProcessDetachedTapIsPrefix` | 41.49 s | `inprocess_test.go:461: the connection did not read Serve's first 56 MiB within 30s` | A, F7 |
| 5 | 1794843224957491326 | `TestInProcessDetachedTapIsPrefix` | 42.05 s | `inprocess_test.go:461: the connection did not read Serve's first 56 MiB within 30s` | A, F7 |
| 5 | 1794843224957491326 | `TestTapErrorIgnored` | 4.28 s | `tap_test.go:256: Wait took longer than 3s` | A, F7 |
| 5 | 1794843224957491326 | `TestLineTapAsOutputTap` | 1.94 s | `linetap_test.go:153: Close = agentproc: agent terminated` | A |
| 5 | 1794843224957491326 | `TestNoLostOutput` | 4.56 s | `agentproc_test.go:174: Wait took longer than 3s` | A |

By class: 8 failures of class A or B tests that are not F7's; 7 of F7's four tests; 1 of an unclear test. None of a class C or D test.

**F7's tests, counted apart** (run in all five rounds, none skipped): `TestDetachedTapIsPrefix` failed in 1 of 5 (round 1,
"Close = agentproc: agent terminated: signal: terminated", `tap_test.go:349`, not F7's "did not say it was ready within 30s");
`TestInProcessDetachedTapIsPrefix` in 3 of 5 (rounds 3, 4, 5, "the connection did not read Serve's first 56 MiB within 30s",
`inprocess_test.go:461`); `TestBlockedTapDetached` in 2 of 5 (rounds 1, 2, "the bulk agent did not say it was ready within 30s",
`tap_test.go:294`); `TestTapErrorIgnored` in 1 of 5 (round 5, "Wait took longer than 3s", `tap_test.go:256`).

#### Findings

1. **Seven tests the rule does not decide** (class "unclear"), of three kinds. What makes each so is in the table's "why" cell.
   * Kill's abandonment wait, M6, which the rule's A and B do not name, with no step waited out: `TestInProcessKillAtOnce`
     (`inprocess_test.go:177`, 5 s), and `TestWaitErrorSaysTheCauseOnce/Kill,_Serve_returning_its_cause` (`inprocess_test.go:227`,
     `testGrace`). B's shape without the waits.
   * A context bound equal to `testGrace` that decides whether `acp.ErrHandlersRunning` is joined (`agentproc.go:644`), with no exit
     needing to be seen in a step: `TestCloseContextBoundsHandlers/in_process,_KillContext` (`closecontext_test.go:142`, assertion
     `:149`) and `TestCloseContextCause` (`:242`, assertion `:249`), both `testGrace`.
   * The tap drain, M5, within a grace, which is no exit: `TestInProcessTaps` (`inprocess_test.go:333`, 5 s), `TestOutputTapVerbatim`
     (`tap_test.go:215`) and `TestInputTapSeesWrites` (`:229`), both at `slack`. `TestOutputTapVerbatim` failed in round 1 (finding 3).
   * The three at `testGrace`, `TestCloseContextBoundsHandlers/in_process,_KillContext`, `TestCloseContextCause` and
     `TestWaitErrorSaysTheCauseOnce/Kill,_Serve_returning_its_cause`, need a ruling before phase 2 moves or leaves them.
     Recommended: read A as "the exit, the drain or the
     return seen within the first grace-bound wait the call makes" and move them, since a longer grace and a bound stated in graces
     move each of their failures; the four at `slack` or more stay as they are either way.
2. **A tests that fail for what a grace does not move.** `TestNoLostOutput` (A, moves) failed in round 5 "Wait took longer than 3s"
   (`agentproc_test.go:174`), the absolute `within(t, slack, …)` that F7's `TestTapErrorIgnored` (A, moves) failed in the same
   round with the same message (`tap_test.go:256`); both burst 1000 updates through the connection beside the load. Moving them to
   `exitGrace` leaves `within(t, slack, …)`, 3 s, unchanged ("assertion unchanged"), so phase 2's A/B under this load ("0 of 10
   amended") cannot be expected of these two: they fail amended as they do at `HEAD`. "Where the race gates run" counts
   `TestTapErrorIgnored` apart under this load, and does not name `TestNoLostOutput`, which has the same failure. Resolutions
   that leave the tree correct: judge `TestNoLostOutput` with F7's tests in phase 2 and phase 4, by a record amendment the owner
   approves (cost: an amendment); or have phase 5's REPORT measure the burst tests as it measures the 56 MiB ones (cost: scope);
   doing nothing leaves phase 2's A/B unable to pass for these two.
3. **Tests that failed at a grace already at or above `slack`, which phase 2 does not change:** `TestCloseReadsTheAgentsLastOutput`
   (A, 5 s), round 2, "Close = agentproc: agent terminated: signal: terminated, want nil" (`tap_test.go:398`): the agent's exit follows
   the connection's read of 2 MiB through a 5 ms read hook, a throughput as F7's, which a longer grace helps only until the
   throughput falls further (read, not measured); `TestDetachedTapIsPrefix` and `TestInProcessDetachedTapIsPrefix`, F7's, at 5 s.
   `TestOutputTapVerbatim` (unclear, `slack`) failed in round 1: "the output tap received 2031616 bytes, want the agent's 2097208"
   and "DroppedTapBytes = 56, 0": 2031616 is 31 times 65536, the 56 counted dropped are, by size, the stream's last bytes, and the
   missing 65536 are consistent with the write in flight when the 3 s drain ended, which `tap.go:76` does not count as dropped
   (read from the figures; the in-flight write was not observed). The tap writes with
   1 ms a call (`capture{delay: time.Millisecond}`, `tap_test.go:194`), so under this load a write took on the order of 100 ms
   (3 s over 31 writes; not measured). It is the kind of failure a grace moves, but it is at `slack`: the MADR's "about five times
   the worst measured" does not hold for a tap drain under this load.
4. **Twelve A tests have no M1 in their outcome**, only M3 (and M4, M5): the exit is of itself, so no step is waited, and the grace
   enters through `WaitDelay`, half of it. The client-side plant of phase 2 (a sleep after `Cmd.Wait`) delays the exit's being seen
   and cannot fail them, since nothing is signalled and `Wait` returns nil; those tests need a plant of their own for "fails at
   `HEAD`, passes amended", such as a standard-error copy that outlives `WaitDelay`. They are `TestNoLostOutput`,
   `TestEscapedDescendantDoesNotHoldWait`, `TestTapKeepsCloseEnding`, `TestDescendantsEnded/the_agent_exits_of_itself`,
   `TestExitedFirst/the_agent_exits_first`, `TestKillAfterExit`, `TestTapErrorIgnored`, `TestProcessWaitContext/the_agent_ends_first`,
   `TestStderrTailOfFailedAgent`, `TestStderrTailNotJoined`, `TestCloseContextBoundsHandlers/a_child,_KillContext` and
   `TestCloseContextNotHandlers`. By reading; not run. `TestDescendantsEnded/the_agent_exits_of_itself`' failure through `WaitDelay`
   is the MADR's F6 (`ties_test.go:84`); none of the twelve failed through M3 in these five rounds.
5. **Bounds that move with the grace.** `TestCloseContextBoundsHandlers`, `TestCloseContextJoinsTheAgentsError` and the rows that
   follow state a bound of `testGrace`, which hurries Close's steps at its end (`agentproc.go:594`): `const bound = testGrace`
   (`closecontext_test.go:145`, `:214`) and the two `ctx` timeouts (`:180`, `:246`). A grace alone does not move them. In round
   3, `TestCloseContextBoundsHandlers/a_child,_CloseContext` failed so: its error held the bound's cause and "agentproc: agent killed", after
   855.162553 ms, and no `acp.ErrHandlersRunning` (`closecontext_test.go:150`); the agent was not seen to exit before the bound.
6. **The load covers part of each round, at random.** `TestZZBurn` takes one of the two parallel slots (`GOMAXPROCS=2` makes
   `-test.parallel` 2) for 60 s, in a shuffled position; a round took about 105 to 109 s, so tests that ran after it ended, or before it
   started, ran unloaded. "Not seen to fail" for a test is therefore weaker than 5 loaded runs. The rounds were not repeated with
   a longer `BURN_FOR`, which the phase did not ask for.
7. **The rule and the PLAN say nothing of tests already at `slack` or more** (9 of the A and B tests, and 4 unclear ones): phase 2's step 3
   names "each other class A and B test" and says to give it its grace; for those it would lower it. The table says "no", and the
   coordinator's brief for phase 2 should say they stay.

No finding is a deviation: every failure of step 3 is a class A or B test, an F7 test counted apart, or a named finding above, and
no class C or D test failed.

#### Not verified

* Class by reading, not by instrumentation: which wait (M1 to M6) decided each failure was read from its message and the code, not
  measured. The failures above agree with the reading in each case
  (a failing `Close`, "agent terminated", for A; the SIGTERM test's `Close` taking 1.013 s against its window of 0.3 to 0.6 s for B),
  none contradicts it, and the 33 A and B tests that did not fail are neither confirmed nor shown wrong by it.
* The Windows-only rows (7, one a container, 3 of the leaves A) and the platform branch of `TestCloseLadder/exits_on_SIGTERM` that takes `:203` were not
  run; their build was checked by `GOOS=windows go vet` only. FreeBSD likewise.
* The plants and breaks of phase 2, the A/B, and the 20 rounds of phase 4 were not run; the claim in finding 4 about the client-side
  plant is from reading `watch` and `Wait`.
* How the 5 rounds' load divided in time (finding 6), and the cost of a goroutine's wake-up under it (finding 3), were not measured.
* No test of this repository outside `agentproc` was read or run.

#### Files

Touched: this PLAN (this entry) and `docs/README.md` (the PLAN's row). Not touched: any test or source file, any workflow. Scratch
only, not staged: the enumerating program, the table and its check, the failure parser, `TestZZBurn`, `chunk.sh` and the rounds'
output.

#### Checks, and staging

After the last edit, with the PLAN and `docs/README.md` staged: `python3 -B scripts/check_records.py --check-all`,
`python3 -B scripts/check_disclosure.py` and `markdownlint-cli2` v0.23.2, each exit 0, as the coordinator's brief for this phase asks. Staged, not
committed, not pushed.

### 2026-11-16: the owner's rulings on the audit

Phase 1 was committed as `f0b7c2e`. The coordinator put the audit's open points to the owner, who ruled the same day; the
coordinator relayed each label, the option's text as put, and the options not chosen. This entry amends the pair; no phase ran, and
no source or test changed. Drafted by a strongest-tier model at high effort on the coordinator's brief.

* **"Move all four, each proved first"**, the owner's ruling, relayed by the coordinator, on the four tests the rule left undecided:
  the three "unclear" at `testGrace` and `TestCloseContextNotHandlers`, class A at 2 s. Not chosen: leave all four; move only the
  three at 300 ms.
* **"Widen 0057 to these bounds now"**, the owner's ruling, relayed by the coordinator, on the bounds that are not a grace in
  `TestNoLostOutput`, `TestTapErrorIgnored`, `TestOutputTapVerbatim` and `TestCloseReadsTheAgentsLastOutput`; not the option the
  coordinator recommended. Not chosen: keep 0057 to graces and let phase 5's report measure the rest; stop using the starved load
  as proof.
* **Unchanged:** "A measured report, then decide". `TestTapErrorIgnored` is now W2's; `TestBlockedTapDetached`'s readiness stays
  with the report, its `Close` moving with class A.
* **The coordinator's applications of standing rules,** recorded as such, not as the owner's rulings: each moved test seen to fail
  at `HEAD` under a plant that fits it (finding 4); the rule never lowers a grace (finding 7); bounds in graces scale, at
  `closecontext_test.go:145`, `:180` and `:246`, and `:214` stays (finding 5); the load covers every moved test for the whole run
  (finding 6).
* **The phase structure, as it now stands:** 1, the audit (mid tier, high effort), done; 1b, the bounds that are not a grace,
  measured (mid tier, high effort), new; 2, the tests (mid tier, medium effort); 3, the docs (cheapest tier, medium effort); 4,
  confirmation and CI (mid tier, medium effort); then 5, the report (mid tier, high effort). Phase 1b is its own phase so that each
  figure, and any stop under the MADR's W4, is reviewed and committed before a test changes.
* **New question:** 5, `closecontext_test.go:156`, open; it holds only phase 1b's step for that bound.
* **Status.** The MADR stays `accepted`, amended; this PLAN stays `in-progress`, its execution approved, phase 1 done. Phase 1b
  carries out "Widen 0057 to these bounds now", whose text, as put, named a measurement for each bound, and this PLAN places it
  inside the approved run, before phase 2. That placement is this amendment's reading, for the coordinator to confirm before
  phase 1b is briefed; it is not a ruling of the owner's.
* **Touched:**
  * the MADR: F7's paragraph (an addition after it); Decision Drivers (an addition); Decision Outcome (an addition); Consequences
    (three additions); Confirmation (an addition); a new section, "Amendment 2026-11-16, after phase 1's audit: four borderline
    tests, and the bounds that are not a grace", with G1 to G7, the rulings, the coordinator's applications, W1 to W5 and question 5;
  * this PLAN: Goal (an addition); Scope, "Where the race gates run" (an addition), "Scope, widened" and "Order and review,
    amended" (new); the rule's table (an addition); phase 1's acceptance (an annotation); phase 1b (new); phase 2 (an addition);
    phase 3 (an addition); phase 4's acceptance (an addition); phase 5 (an addition); Verification (a table added); "Question after
    phase 1's audit, open" (new); this entry;
  * `docs/README.md`: the MADR's and this PLAN's rows.
