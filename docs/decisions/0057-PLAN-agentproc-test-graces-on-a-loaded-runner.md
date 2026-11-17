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
*Added 2026-11-16, after phase 2's A/B, by the owner's ruling, relayed by the coordinator, "Count it apart with the other readiness failure". As put to the owner: `TestTapDrainBoundedByGrace`'s grace move stays (its plant failed today's code and passes amended); its 30 s readiness failure ("the bulk agent did not say it was ready within 30s", `tap_test.go:441`) joins `TestBlockedTapDetached`'s for phase 5's measured report, by a dated amendment of the plan's gates; consistent with the earlier ruling "A measured report, then decide". Options not chosen: measure the readiness wait and rescale it now; revert its move.* `TestTapDrainBoundedByGrace` is counted apart for its failure to be ready within 30 s (`tap_test.go:441`), as `TestBlockedTapDetached` is; any other failure of either test counts as a moved test's.

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
8. *Added 2026-11-16, for question 5's answer (a), "Measure it; rescale only if it still fails".* The fifth bound,
   `closecontext_test.go:156`: stamped with the other four (the release, the exit, `Wait`'s return) and measured with them, 20
   runs of each of its four subtests under each load; then, in a scratch copy in which its subtest's grace and grace-stated bound
   are moved as phase 2 will move them (`exitGrace`, 3 s, for the grace and for `const bound`, and the `within` bound that states
   them in graces), measured again under both loads, 20 runs of each subtest. The record says whether the fixed 3 s `Wait` still
   fails there, with the counts. A figure is proposed, by steps 3 to 5, only if it does; if it does not, it stays at 3 s and the
   record says why it stopped failing.

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
* **Counted apart** (added 2026-11-16, after phase 2's A/B, by the owner's ruling, relayed by the coordinator, "Count it apart with the other readiness failure". As put to the owner: `TestTapDrainBoundedByGrace`'s grace move stays (its plant failed today's code and passes amended); its 30 s readiness failure ("the bulk agent did not say it was ready within 30s", `tap_test.go:441`) joins `TestBlockedTapDetached`'s for phase 5's measured report, by a dated amendment of the plan's gates; consistent with the earlier ruling "A measured report, then decide". Options not chosen: measure the readiness wait and rescale it now; revert its move.): the "0 of 10 amended" of the load bullet above and of this phase's acceptance is judged on every moved test's own outcome; a failure of the 30 s readiness wait in `TestTapDrainBoundedByGrace` or `TestBlockedTapDetached` is counted apart, as F7's are, and any other failure of either counts.
* **The five figures** (added 2026-11-16, before phase 2 ran): the owner's ruling, relayed by the coordinator, "The five as measured". As put to the owner: 35 s, 39 s, 28 s, 42 s and 35 s, each from its own measurement (phase 1b's table and addendum): `TestNoLostOutput`'s `Wait` bound (`agentproc_test.go:174`) 35 s; `TestTapErrorIgnored`'s `Wait` bound (`tap_test.go:256`) 39 s; `tappedFrames`' grace (`tap_test.go:195`, `TestOutputTapVerbatim`'s drain) 28 s; `TestCloseReadsTheAgentsLastOutput`'s grace (`tap_test.go:377`) 42 s, its `within` following; the fixed `Wait` at `closecontext_test.go:156` 35 s. Phase 2's proof, 0 failures in 10 rounds under the starved load, can then hold for these tests; a passing run is no slower, and a hung wait takes up to its figure to fail. Options not chosen: one shared bound for all five; set them from the module load only. The "What moves" bullet's four bounds, and `closecontext_test.go:156`, which phase 1b's step 8 measured and found to fail still, take these figures; the margin's two results (0.8 passes, 1.2 fails) and the break are shown again on phase 2's amended files, the measured copies being phase 2's shape and not its edits.

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
*Added 2026-11-16, after phase 2's A/B:* the tests counted apart also include `TestTapDrainBoundedByGrace`'s failure to be ready within 30 s (`tap_test.go:441`), by the owner's ruling "Count it apart with the other readiness failure", relayed by the coordinator; any other failure of it, or of `TestBlockedTapDetached`, is a moved test's.

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
*Added 2026-11-16, after phase 2's A/B:* the report measures `TestTapDrainBoundedByGrace`'s time to be ready too (its 24 MiB bulk agent, `tap_test.go:441`, the same message as `TestBlockedTapDetached`'s), by the owner's ruling "Count it apart with the other readiness failure", relayed by the coordinator.

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

*Annotated 2026-11-16: answered (a), below the options.*

5. **`closecontext_test.go:156`, a fixed 3 s `Wait` (the MADR's G7).** It failed once, in the audit's round 3, beside its subtest's
   grace failure, which the bound scaling moves; whether it fails by itself is not known.
   * (a) Recommended: measure it in phase 1b with W2's four, and rescale it only if it fails once its subtest's grace and bound have
     moved. Cost: one more measurement in phase 1b.
   * (b) Rescale it now as a fifth bound of W2. Cost: a figure for a bound that may not need one.
   * (c) Leave it to phase 5's report. Cost: a bound the load exposed left outside the plan the owner's ruling made for them.

   Phase 1b's step for it runs only on answer (a) or (b); the rest of phase 1b does not depend on it.

   * **Answered (a), 2026-11-16,** the owner's ruling, relayed by the coordinator: "Measure it; rescale only if it still fails".
     As put to the owner: phase 1b stamps it with the other four; it is rescaled only if it still fails once its subtest's grace
     and grace-stated bound have moved; if it stops failing, it stays at 3 s and the record says why. Not chosen: rescale it now
     with the other four; leave it to the report. Phase 1b's step for it (step 8, added below) therefore runs; "out of scope
     still", in "Scope, widened", holds only until that step's result. Phase 5's "question 5's answer (c)" is not the answer given.

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

### 2026-11-16: phase 1b (the bounds that are not a grace, measured)

Run by a mainstream-tier model at high effort, as the phase prices it, on the coordinator's brief. `date +%F` printed 2026-11-16,
the date of the earlier entries. Nothing in the tree changed but this PLAN (this entry, the question 5 answer and annotations),
the MADR (question 5's answer) and `docs/README.md` (the two rows). No test file, source file, workflow or script changed; every
instrument below lived in scratch copies made by `git archive HEAD`.

* **Approval and rulings.**
  * The owner's rulings, relayed by the coordinator, "Run phases 1 to 4 through; then the report" (the approval entry above) and
    "Widen 0057 to these bounds now" (the entry "2026-11-16: the owner's rulings on the audit"). That entry placed phase 1b inside
    the approved run and called the placement its own reading, "for the coordinator to confirm". *Confirmed 2026-11-16 by the
    coordinator, in the brief for this phase, as the coordinator's reading of those two rulings; it is not a ruling of the
    owner's.*
  * Question 5, answered (a) by the owner's ruling, relayed by the coordinator: "Measure it; rescale only if it still fails".
    Recorded under question 5 in the MADR and in this PLAN, and carried into phase 1b as step 8 (added above). Not chosen: rescale
    it now with the other four; leave it to the report.
* **Base commit.** `73e7e89`, `main`, clean. Line numbers below are that tree's, except where a message is quoted from a scratch
  copy, which says so ("the copy's"): the stamped copies add lines to the test files.
* **Read:** this PLAN and its MADR in full; `agentproc/agentproc_test.go`, `tap_test.go`, `closecontext_test.go`, `child_test.go`,
  `leak_test.go`, `agentproc/agentproc.go`, `tap.go`, `inprocess.go`; `AGENTS.md`, `tools/README.md`; the skills
  `madr-and-plan-writing` and `available-dev-hosts`; the audit's scratch load (`TestZZBurn`) and the root-cause work's stamping
  copy, which shaped the stamps (nothing in them was reused unread).
* **Ran, this Mac,** without `-race`, in the scratch copies: `gofmt`, `go vet ./agentproc/` on each copy (exit 0); the seven
  measured tests (the five bounds' tests, `TestInputTapSeesWrites` and the moved copy) once per build of the stamping copy, each
  run writing its stamps (13 records a run; `ok`); the plant knobs once each on an idle machine (the 1 s plant passed a 2 s bound,
  the 3 s plant failed it with "Wait took longer than 2s"; the output-tap plant passed at 2 s of a 3 s grace and failed at 4 s;
  the exit plant the same), so that no knob was first tried on the Linux host.
* **Ran, the Linux host,** every chunk through `run-on-host --need linux --race --`, one heavy run at a time on the host, never a WSL
  distro. The chunks are in "The chunks", below; none waited in the host's queue (each printed `waited=0s`), so no other agent's
  run interleaved with them.

#### The measuring copies

Three copies of the tree at `73e7e89`, each built by a script that asserts every edit's old text occurs once before it replaces
it: the **stamping copy** (stamps, the in-process load and the moved copy of the fifth bound's test); the **planting copy**
(that, and the knobs of the margin plants and breaks); a third (the planting copy, and a stamp at every 100th update the fifth
bound's hung client handles after its release). Each knob reads the environment and is a no-op unset. What they carry:

* **The stamps.** One record per test run, appended as a line of JSON when the run ends: every stamp's name and wall-clock time
  in nanoseconds (one clock for the test process and the agent it started, on one host), so the parts of a run can be set against
  each other.
  * The bound's start, the go notification before and after `SendNotification`; the agent's own stamps in a file it appends to
    (it received the go, built its output, finished its write, returned); the client's first, and 1000th, update handled
    (`TestNoLostOutput`, `TestTapErrorIgnored`); `Cmd.Wait`'s return and `done` (`watch`); which of the watcher's three events
    ended its wait, and the connection closed; the goroutines' wait and the connection's wait in `Wait`; `Wait`'s return.
  * `TestOutputTapVerbatim` (through `tappedFrames`, so `TestInputTapSeesWrites` too): `Close`'s call and return; `stop`'s close of
    the agent's input; `drainTaps`' start and end; each tap `Write`'s start and end with its size.
  * `TestCloseReadsTheAgentsLastOutput`: the go notification; each held read, at the reader's hook (the hook runs before the read,
    so a read's size is the matching tap `Write`'s, the tap taking one `Write` a read); the agent's stamps; `done`; `Close`'s call
    and return.
  * `closecontext_test.go:156`, in all four subtests of both copies: the call of `CloseContext` or `KillContext` and its return,
    the release, `Wait`'s start and return, the exit (`Cmd.Wait`'s or `Serve`'s return), and, in the third copy, the 100th,
    200th, ... update the hung client handled after the release.
* **The shape measured.** The tests as phase 2 will run them, with each bound lifted so that it measures the time and does not
  cut it (the first run, below, showed a bound that cuts leaves the time unmeasured): `TestNoLostOutput` and
  `TestTapErrorIgnored` with their agent's grace at `exitGrace`, 3 s, and `within` 120 s; `tappedFrames`' grace (the drain, the
  watcher's read and `Close`'s first step) 120 s; `TestCloseReadsTheAgentsLastOutput`'s grace 120 s, `within` following it; the
  fifth bound's `within(t, slack, "Wait")` 120 s, in the copy as it is at `HEAD` and in the **moved copy** (its subtests' grace and
  `const bound` at 3 s, and the `within` that states them in graces, `bound+exitGrace+slack`). Which tests would exceed
  today's bound is read from the measured time ("Counts", below), not from a pass or a fail.
* **The in-process load,** the MADR's: 32 busy goroutines in the test process and `GOMAXPROCS=2`, started by an `init` of a
  scratch-only test file when `BURN` is set, so that the load is on from the binary's start to its end and no test runs outside
  it (not as a test, as the audit's was). The `init` does nothing in the child processes, which are the same binary.
* **The breaks, planted** (scratch only): for `TestNoLostOutput`, the watcher closes the connection at the agent's exit, not
  after reading to its end (MADR 0057, W2); for `TestTapErrorIgnored`, the first failed tap `Write` makes the tap's next read
  return an error, "a tap's `Write` error returned to the connection's reader"; for `TestOutputTapVerbatim`, `tap.run` skips its
  4th queued write; for `TestCloseReadsTheAgentsLastOutput`, `stop` closes the connection at once, which the comment at
  `agentproc/agentproc.go:664`-`:668` forbids; for the fifth bound, which W2 gives none, **a break this phase chose**, `Wait`
  sleeping a minute once its wait on the connection has ended, "a `Wait` that does not return once its handlers have", for the
  coordinator to review.
* **The margin plants** (scratch only): the thing waited for is delayed so that it ends at a set time after the bound began: the
  1000th update's handler held until that time after `Wait`'s start (`TestNoLostOutput`, `TestTapErrorIgnored`); the output
  tap's last `Write` held until that time after the drain's start (`TestOutputTapVerbatim`); the agent's exit held until that
  time after it received its go (`TestCloseReadsTheAgentsLastOutput`); the 1000th update's handler held until that time after the
  release (the fifth bound, in the moved copy).

#### The chunks

Every chunk's output, status and seeds are recorded; the module rounds and the in-process iterations each had their own seed, set
explicitly (`-shuffle=<seed>`; `go test` hides the seed of a package that passes), and the plant cases ran `-test.shuffle=off`.

| Chunk | Command | What | Seeds | Seconds | Rounds with a failing test |
| :--- | :--- | :--- | :--- | ---: | ---: |
| P1 | `sh zzburn.sh 1 1` | the whole package, first stamping copy, bounds not lifted | 1794854633547748 | 394 | 1 of 1 |
| mod-01 | `sh zzmod.sh 1 4` | module, 4 rounds | 1794855137549260, 1794855252549606, 1794855341549874, 1794855428550136 | 386 | 1 |
| mod-02 | `sh zzmod.sh 5 8` | module, 4 rounds | 1794855524550425, 1794855643550783, 1794855730551045, 1794855823551325 | 395 | 4 |
| mod-03 | `sh zzmod.sh 9 12` | module, 4 rounds | 1794855919551614, 1794856036551966, 1794856126552237, 1794856215552505 | 390 | 3 |
| mod-04 | `sh zzmod.sh 13 16` | module, 4 rounds | 1794856309552788, 1794856423553131, 1794856516553411, 1794856607553685 | 402 | 3 |
| mod-05 | `sh zzmod.sh 17 20` | module, 4 rounds | 1794856712554001, 1794856843554395, 1794856940554687, 1794857033554967 | 422 | 4 |
| burnsel-01 | `sh zzburnsel.sh 1 1 5` | in-process, the measured tests, `-test.count=5` | 1794857148555293 | 250 | 1 |
| burnsel-02 | `sh zzburnsel.sh 2 2 5` | in-process, the measured tests, `-test.count=5` | 1794857401556053 | 290 | 1 |
| burnsel-03 | `sh zzburnsel.sh 3 3 5` | in-process, the measured tests, `-test.count=5` | 1794857688556915 | 280 | 1 |
| burnsel-04 | `sh zzburnsel.sh 4 4 5` | in-process, the measured tests, `-test.count=5` | 1794857970557762 | 246 | 1 |
| burnfull-01 | `sh zzburn.sh 1 1` | in-process, the whole package, 1 iteration | 1794858215558494 | 348 | 1 |
| burnfull-02 | `sh zzburn.sh 2 2` | in-process, the whole package, 1 iteration (connection lost; recovered) | 1794858563559539 | - | 1 of 1 |
| pl-nlo | `sh zzcases.sh cases-nlo.txt` | plant cases, `TestNoLostOutput` | none (`-test.shuffle=off`) | 255 | - |
| pl-tei | `sh zzcases.sh cases-tei.txt` | plant cases, `TestTapErrorIgnored` | none (`-test.shuffle=off`) | 532 | - |
| pl-otv | `sh zzcases.sh cases-otv.txt` | plant cases, `TestOutputTapVerbatim` | none (`-test.shuffle=off`) | 194 | - |
| pl-crlo | `sh zzcases.sh cases-crlo.txt` | plant cases, `TestCloseReadsTheAgentsLastOutput` | none (`-test.shuffle=off`) | 253 | - |
| pl-cc | `sh zzcases.sh cases-cc.txt` | plant cases, the fifth bound, moved copy | none (`-test.shuffle=off`) | 445 | - |
| burncc-01 | `sh zzburncc.sh 1 1 5` | in-process, both copies of the fifth bound, the third copy, `-test.count=5` | 1794860948566693 | 117 | 1 |
| burncc-02 | `sh zzburncc.sh 2 2 5` | in-process, both copies of the fifth bound, the third copy, `-test.count=5` | 1794861066567048 | 114 | 1 |
| nochunk-burnsel-01 | `sh zzburnsel.sh 1 1 5` | the script missing from the copy shipped: nothing ran | - | exit 2, 5 | - |
| nochunk-burnsel-02 | `sh zzburnsel.sh 2 2 5` | the script missing from the copy shipped: nothing ran | - | exit 2, 4 | - |
| nochunk-burnsel-03 | `sh zzburnsel.sh 3 3 5` | the script missing from the copy shipped: nothing ran | - | exit 2, 4 | - |
| nochunk-burnsel-04 | `sh zzburnsel.sh 4 4 5` | the script missing from the copy shipped: nothing ran | - | exit 2, 4 | - |

* **P1, the first run, kept as a finding.** The whole package, once, under the in-process load, with the first stamping copy,
  whose graces and bounds were the tests' own (not lifted): 14 top-level tests failed, among them four of the five bounds' tests
  (all but `TestOutputTapVerbatim`, which passed with its drain running 3005 ms, to the end of its 3 s grace, its last `Write`
  ending with it): `TestNoLostOutput` and
  `TestTapErrorIgnored` "Wait took longer than 3s"; `TestCloseReadsTheAgentsLastOutput` "Close = agentproc: agent terminated:
  signal: terminated, want nil" at its 5 s grace; and the fifth bound's `Wait` in both copies ("closecontext_test.go:165: Wait
  took longer than 3s" in the copy of `HEAD`'s test, "zz_cc_test.go:125: Wait took longer than 3s" in the moved copy, twice); and
  class A and B tests (`TestCloseLadder`, `TestExitedFirst`, `TestProcessWaitContext`, `TestDescendantsEnded`,
  `TestDefaultPdeathsig`, `TestTapDrainBoundedByGrace`) besides F7's. Its stamps measured only what a bound had not cut, so its
  figures are in no table; it is why the bounds were lifted. Its failing tests are all class A, class B or F7's; none is class
  C or D.
* **Four attempts that ran nothing.** `zzburnsel.sh`, the in-process chunk of the measured tests, was missing from the copy the
  four first calls shipped, `sh: cannot open zzburnsel.sh`, exit 2 within 5 s; the script was added to the copy and the four
  chunks run again (`burnsel-01` to `-04`). The module chunks that were running had been shipped before, and are unaffected.
* **`burnfull-02`, a lost connection.** `run-on-host` reported "ssh exited 255" 119 s into the chunk; the run itself continued on
  the host and ended (its heavy-run lease with it). Its output file and stamps were read from the work root the dispatcher keeps
  after a failed run, over ssh, and are the chunk's record here; the chunk's own printed summary was lost, so its seed is the
  first line of that output and its `exit` line is not available (the output ends `FAIL`).
* *Added 2026-11-16:* the chunks `burnfull-03` to `-11`, `pl-crlo42` and `pl-cc35` are in the addendum, below.

#### The measurements

Milliseconds. "Module": 20 rounds of `go test -race -shuffle=<seed> ./...`, one run of each test a round. "In-process": the
measured tests alone, 4 iterations of `-test.count=5`, 20 runs of each test, under the in-process load. The whole-package
iterations of the in-process load (`burnfull-01` and `-02`, 2 runs of each test) are in "The figures". The bold rows are the
quantities the bounds wait for.

##### TestNoLostOutput

| Part | Module runs | Module median | Module p90 | Module max | In-process runs | In-process median | In-process p90 | In-process max |
| :--- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| **the bounded time: `within` start to `Wait`'s return** | 20 | 500 | 742 | 899 | 20 | 5328 | 6526 | 6920 |
| the agent builds its 1000 updates | 20 | 7.5 | 13.2 | 16 | 20 | 7.1 | 11 | 21.6 |
| the agent's one write, until the client has read all but a pipe's worth | 20 | 83 | 177 | 222 | 20 | 785 | 1159 | 1397 |
| the agent's return to `Cmd.Wait`'s return | 20 | 14.9 | 36 | 41.7 | 20 | 418 | 640 | 993 |
| `within` start to the 1000th update handled | 20 | 500 | 741 | 899 | 20 | 5315 | 6525 | 6861 |
| `done` to the watcher closing the connection | 20 | 99.1 | 162 | 187 | 20 | 626 | 1023 | 1221 |
| the 1000th update handled to `Wait`'s return | 20 | 0.1 | 0.2 | 0.3 | 20 | 12.8 | 213 | 320 |

##### TestTapErrorIgnored

| Part | Module runs | Module median | Module p90 | Module max | In-process runs | In-process median | In-process p90 | In-process max |
| :--- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| **the bounded time: `within` start to `Wait`'s return** | 20 | 465 | 758 | 1049 | 20 | 5056 | 5469 | 5735 |
| the agent builds its 1000 updates | 20 | 8.2 | 14.3 | 17.1 | 20 | 6.4 | 10.4 | 21.5 |
| the agent's one write, until the client has read all but a pipe's worth | 20 | 79.8 | 130 | 158 | 20 | 691 | 1106 | 1251 |
| the agent's return to `Cmd.Wait`'s return | 20 | 9.7 | 26 | 32.5 | 20 | 344 | 469 | 500 |
| `within` start to the 1000th update handled | 20 | 465 | 758 | 1048 | 20 | 4960 | 5391 | 5703 |
| `done` to the watcher closing the connection | 20 | 92.5 | 162 | 188 | 20 | 777 | 1107 | 1226 |
| the 1000th update handled to `Wait`'s return | 20 | 0.1 | 0.3 | 0.7 | 20 | 98.4 | 195 | 298 |

##### TestOutputTapVerbatim

| Part | Module runs | Module median | Module p90 | Module max | In-process runs | In-process median | In-process p90 | In-process max |
| :--- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| **the drain: `drainTaps`' start to its end** | 20 | 176 | 271 | 302 | 20 | 3469 | 4871 | 5596 |
| **`Close`'s first step: the input closed to `done`** | 20 | 11.1 | 18.8 | 33.7 | 20 | 401 | 584 | 843 |
| **the watcher: `done` to the connection closed** | 20 | 0.1 | 0.6 | 3.8 | 20 | 31.1 | 244 | 277 |
| `Close`, called to returned | 20 | 192 | 284 | 318 | 20 | 4012 | 5282 | 6185 |
| one tap `Write`, median (1 ms asked) | 20 | 1.4 | 2.1 | 2.4 | 20 | 169 | 206 | 223 |
| one tap `Write`, slowest | 20 | 107 | 146 | 169 | 20 | 613 | 788 | 817 |
| tap `Write`s ended after the drain began, of 33 | 20 | 24.5 | 28 | 29 | 20 | 20.5 | 25 | 27 |
| the tap's first `Write` begun to its last ended | 20 | 295 | 397 | 428 | 20 | 5343 | 6445 | 6891 |

##### TestCloseReadsTheAgentsLastOutput

| Part | Module runs | Module median | Module p90 | Module max | In-process runs | In-process median | In-process p90 | In-process max |
| :--- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| **`Close`'s first step: the input closed to `done`** | 20 | 706 | 948 | 1154 | 20 | 6066 | 7325 | 7603 |
| **the watcher: `done` to the connection closed** | 20 | 362 | 533 | 580 | 20 | 1232 | 1718 | 2114 |
| **the drain** | 20 | 0 | 0 | 0 | 20 | 0 | 0 | 0 |
| `Close`, called to returned | 20 | 1084 | 1428 | 1734 | 20 | 7361 | 8174 | 8817 |
| the agent builds its 2 MiB | 20 | 41.5 | 73.5 | 77.3 | 20 | 35.9 | 51.5 | 62.2 |
| the agent's write, until the connection has read through the held reads | 20 | 648 | 903 | 1058 | 20 | 5700 | 6697 | 6990 |
| the agent's return to `Cmd.Wait`'s return | 20 | 10 | 36.6 | 65.3 | 20 | 462 | 619 | 874 |
| held reads | 20 | 32 | 32 | 32 | 20 | 32 | 32 | 32 |
| between two held reads, median (5 ms asked) | 20 | 7.3 | 8.7 | 11.5 | 20 | 25.8 | 194 | 228 |
| between two held reads, longest | 20 | 410 | 583 | 641 | 20 | 1873 | 2300 | 2408 |


##### The fifth bound, `Wait` after the release, all subtests, both copies

| Subtest | Copy | Module runs | Module median | Module p90 | Module max | In-process runs | In-process median | In-process p90 | In-process max | Above 3 s: module, in-process, whole package |
| :--- | :--- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | :--- |
| `a child, CloseContext` | as `HEAD` has it | 20 | 367 | 513 | 938 | 20 | 2955 | 3624 | 3756 | 0 of 20, 8 of 20, 0 of 2 |
| `a child, CloseContext` | moved | 20 | 375 | 571 | 1384 | 20 | 4711 | 5306 | 6449 | 0 of 20, 20 of 20, 2 of 2 |
| `a child, KillContext` | as `HEAD` has it | 20 | 18.8 | 81.9 | 192 | 20 | 2217 | 3726 | 4242 | 0 of 20, 5 of 20, 1 of 2 |
| `a child, KillContext` | moved | 20 | 14.3 | 22.2 | 83.3 | 20 | 2248 | 3124 | 4119 | 0 of 20, 3 of 20, 0 of 2 |
| `in process, CloseContext` | as `HEAD` has it | 20 | 0.1 | 0.1 | 0.3 | 20 | 0.1 | 0.1 | 41.3 | 0 of 20, 0 of 20, 0 of 2 |
| `in process, CloseContext` | moved | 20 | 0.1 | 0.1 | 0.2 | 20 | 0.1 | 0.1 | 0.4 | 0 of 20, 0 of 20, 0 of 2 |
| `in process, KillContext` | as `HEAD` has it | 20 | 0.1 | 0.2 | 0.2 | 20 | 0.1 | 0.3 | 482 | 0 of 20, 0 of 20, 0 of 2 |
| `in process, KillContext` | moved | 20 | 0.1 | 0.2 | 2.6 | 20 | 0.1 | 0.1 | 284 | 0 of 20, 0 of 20, 0 of 2 |

#### What the time is spent on

* **`TestNoLostOutput` and `TestTapErrorIgnored`** (`agentproc/agentproc_test.go:174`, `agentproc/tap_test.go:256`). `Wait` returns
  when the client has handled the agent's 1000th update, and the time is the client handling 1000 updates: beside the module,
  500 ms and 465 ms (median) to the 1000th, 0.1 ms to `Wait`'s return after it; under the in-process load, 5315 ms and 4960 ms
  (median), about 5 ms an update, and `Wait` returns a median 13 ms and 98 ms after it. The agent's part is smaller: its
  write, until the client has read all but a pipe's worth, 785 ms and 691 ms (in-process medians), its return to `Cmd.Wait`'s
  return 418 ms and 344 ms, the watcher's close 626 ms and 777 ms after `done`; all of it overlaps the handling. The worst
  `TestTapErrorIgnored` run was a whole-package iteration, 7669 ms, and the worst `TestNoLostOutput` one in the tests-alone
  load, 6920 ms.
* **`TestOutputTapVerbatim`** (`agentproc/tap_test.go:195`, the drain, `Close`'s first step and the watcher's read share the
  grace). The time is the output tap delivering 2 MiB in 33 `Write`s that each ask 1 ms. Beside the module a `Write` takes a
  median 1.4 ms and the slowest 107 ms (median over runs), and the delivery 295 ms from the first `Write` to the last; under the
  in-process load a `Write` takes a median 169 ms, the slowest 613 ms, and the delivery 5343 ms (max 6891 ms). Most of that is
  still to do when the drain begins (a median 24.5 of the 33 `Write`s ended after it beside the module, 20.5 under the load), so
  the drain is most of the delivery: 176 ms and 3469 ms (medians), at worst 302 ms and 5596 ms. `Close`'s first step (the agent
  exiting at its input's end) and the watcher's read are far smaller, 844 ms and 277 ms at worst. This measures the audit's
  finding 3, which read "about 100 ms a 64 KiB write" from the figures: under the load a `Write` that asks 1 ms takes a median
  169 ms.
* **`TestCloseReadsTheAgentsLastOutput`** (`agentproc/tap_test.go:377`, `:397`-`:398`). The agent's one 2 MiB write returns only
  when the connection has read all but a pipe's worth through 32 reads, each held 5 ms by the hook; the agent exits after that.
  Beside the module a held read cycles in a median 7.3 ms and the longest in a run takes 410 ms (median), the write 648 ms, and
  `Close`'s first step 706 ms (max 1154 ms). Under the in-process load a read cycles in a median 25.8 ms, but a few take 1 to 2.4
  s (the longest in a run 1873 ms median, 2409 ms at worst), so the write takes 5700 ms (max 6990 ms) and the first step 6066 ms
  (max 7603 ms): more than the 5 s grace in 18 of the 20 runs and in both whole-package iterations. After the exit, the watcher's
  read to the end takes a median 362 ms and 1232 ms (max 580 ms and 2114 ms); the drain takes no time (the tap is a plain buffer
  with no delay).
* **`closecontext_test.go:156`.** After the release, the hung client's handler returns and the connection has the rest of the
  agent's 1000 updates to hand to the client's handler, which `Wait` waits for. In the moved copy's child `CloseContext` subtest
  that is the whole wait: in the third copy, a median 4860 ms of its 4918 ms from the release to the 1000th update handled, and 17.5
  ms from it to `Wait`'s return (10 runs, in-process). That the wait is the handling of the queue was counted: after the release
  the moved copy's child `CloseContext` subtest handled all 1000 updates in 10 of 10 runs, and the copy of `HEAD`'s, whose
  kill comes at its 300 ms bound and not at 3 s, handled 400 to 800 (the largest 100th stamped, 10 runs); so the copy of `HEAD`'s
  test waits less (in-process median 2955 ms against 4711 ms), which is why moving the grace and the bound moves this bound too.
  The `KillContext` subtest of the child waits less, 2248 ms in-process (the kill leaves fewer updates queued: 100 to 500 stamped
  in the moved copy); the in-process agents' subtests have one update and wait a median 0.1 ms, 283.8 ms at worst.

#### Counts

Runs whose waited time exceeds the bound the test has at `HEAD` (read from the time, the bounds having been lifted). In P1, with their own bounds in force, the first, second and fourth failed and the third passed.

| Bound at `HEAD` | Beside the module | In-process, tests alone | In-process, whole package |
| :--- | ---: | ---: | ---: |
| `TestNoLostOutput`, `Wait`, 3 s | 0 of 20 | 20 of 20 | 2 of 2 |
| `TestTapErrorIgnored`, `Wait`, 3 s | 0 of 20 | 20 of 20 | 2 of 2 |
| `TestOutputTapVerbatim`, the drain, `tappedFrames`' 3 s | 0 of 20 | 18 of 20 | 1 of 2 |
| `TestCloseReadsTheAgentsLastOutput`, the first step, its 5 s | 0 of 20 | 18 of 20 | 2 of 2 |

#### The figures

By the PLAN's step 3: five times the worst measured over both loads, rounded up to a whole second (not to a multiple of `slack`,
which the step allows instead). "Worst" is the largest value of the quantity the bound waits for in any run of any load: the 20
module runs, the 20 in-process runs of the measured tests alone, and the 2 whole-package in-process runs. The quantity of a bound
with several waits (`Close`'s first step, the watcher's read, the drain, each bounded by the one grace) is the largest of them.

| Bound | Worst, module | Worst, in-process, tests alone | Worst, in-process, whole package | Worst, all | Five times | Figure |
| :--- | ---: | ---: | ---: | ---: | ---: | ---: |
| `TestNoLostOutput`, `Wait`, `agentproc_test.go:174` | 898.8 | 6920.2 | 5492.2 | 6920.2 | 34.60 s | **35 s** |
| `TestTapErrorIgnored`, `Wait`, `tap_test.go:256` | 1048.8 | 5734.9 | 7668.7 | 7668.7 | 38.34 s | **39 s** |
| `TestOutputTapVerbatim`, `tappedFrames`' grace, `tap_test.go:195` (the drain; the first step 843.5 and the watcher 277.4 are less) | 302.2 | 5595.9 | 4702.6 | 5595.9 | 27.98 s | **28 s** |
| `TestCloseReadsTheAgentsLastOutput`, grace, `tap_test.go:377` (the first step; the watcher 2114.3 and the drain 0 are less) | 1153.8 | 7602.7 | 6560.6 | 7602.7 | 38.01 s | **39 s** |
| `closecontext_test.go:156`, as moved (all four subtests; the child `CloseContext` one) | 1384.0 | 6448.5 | 5226.9 | 6448.5 | 32.24 s | **33 s** |

The arithmetic, each: 6920.2 ms x 5 = 34.601 s, up to 35 s; 7668.7 ms x 5 = 38.343 s, up to 39 s; 5595.9 ms x 5 = 27.979 s, up to
28 s; 7602.7 ms x 5 = 38.013 s, up to 39 s; 6448.5 ms x 5 = 32.243 s, up to 33 s. Each figure follows from what the time is spent
on, above, and none was raised until a run passed: every figure was fixed before any plant ran. `TestCloseReadsTheAgentsLastOutput`'s
`within(c.Grace+slack)` at `tap_test.go:397` follows its grace, 39 s + 3 s = 42 s.

For review, not stops:

* The figures are set by the in-process load, which the MADR chose as its proof; beside the module alone, five times the worst is
  5 s, 6 s, 2 s, 6 s and 7 s, in the order of the table. The MADR's first decision, `slack`, was 3 s: the new figures are 28 to
  39 s, above `readyWait`'s 30 s for four of the five, and a defect that hangs one of these waits now takes up to its figure to
  fail. A passing run takes no longer (a bound is a maximum). The rule is the MADR's W3; this phase applied it as written.
* `TestOutputTapVerbatim`'s figure rests on the drain, as the PLAN's step 1 names it. The tap's whole delivery, from its first
  `Write` to its last, an upper bound on any drain, has a worst of 6890.8 ms (five times, 34.45 s, 35 s); the figure of 28 s is
  4.1 times that worst. Should the coordinator prefer the bound the drain could never exceed, it is 35 s.
  *Annotated 2026-11-16: the line bounds the drain, not the delivery (point 5b, in the addendum); the 35 s of this bullet is not a
  bound the line imposes, and the figure stays 28 s.*
* The worst of 20 is a sample statistic; the whole-package iterations (2 runs of each test, all that the time allowed of the 20
  the step asks) set one figure, `TestTapErrorIgnored`'s.
  *Annotated 2026-11-16: superseded. The whole-package load was run to 20 iterations (the addendum); the full sample raised two
  figures.*

#### The margins and the breaks

Each case on the Linux host under `-race`, with no load other than the plant (idle), the figure given to the bound by its
environment knob, `-test.shuffle=off`. "Waited" is the time the stamps give for the thing the bound waits for. The messages are the
tests' own, from the copy (its line numbers).

| Case | Environment | Runs | Passed | Failed | Waited | Failure message (count) |
| :--- | :--- | ---: | ---: | ---: | :--- | :--- |
| nlo-base | `NLO_BOUND=35s` | 3 | 3 | 0 | 0.3 to 0.4 s over 3 stamped runs | - |
| nlo-in-28s | `NLO_BOUND=35s PLANT_LAST_UPDATE=28s` | 3 | 3 | 0 | 28.0 to 28.0 s over 3 stamped runs | - |
| nlo-out-42s | `NLO_BOUND=35s PLANT_LAST_UPDATE=42s` | 3 | 0 | 3 | 42.0 to 42.0 s over 1 stamped runs | Wait took longer than 35s (3x) |
| nlo-break-35s | `NLO_BOUND=35s BREAK=closeAtExit` | 10 | 0 | 10 | 0.1 to 0.2 s over 10 stamped runs | the client received N of the 1000 session updates the agent wrote (10x) |
| nlo-break-head | `BREAK=closeAtExit` | 10 | 0 | 10 | 0.1 to 0.2 s over 10 stamped runs | the client received N of the 1000 session updates the agent wrote (10x) |
| tei-base | `TEI_BOUND=39s` | 3 | 3 | 0 | 0.3 to 0.4 s over 3 stamped runs | - |
| tei-in-31.2s | `TEI_BOUND=39s PLANT_LAST_UPDATE=31.2s` | 3 | 3 | 0 | 31.2 to 31.2 s over 3 stamped runs | - |
| tei-out-46.8s | `TEI_BOUND=39s PLANT_LAST_UPDATE=46.8s` | 3 | 0 | 3 | 46.8 to 46.8 s over 2 stamped runs | Wait took longer than 39s (3x) |
| tei-break-39s | `TEI_BOUND=39s BREAK=taperr` | 10 | 0 | 10 | 3.0 to 3.0 s over 10 stamped runs | Wait = agentproc: agent terminated: signal: terminated, want nil (10x) |
| tei-break-head | `BREAK=taperr` | 10 | 0 | 10 | 3.0 to 3.0 s over 6 stamped runs | Wait took longer than 3s (10x) |
| otv-base | `TAPPED_GRACE=28s` | 3 | 3 | 0 | 0.1 to 0.1 s over 3 stamped runs | - |
| otv-in-22.4s | `TAPPED_GRACE=28s PLANT_TAP_DRAIN=22.4s` | 3 | 3 | 0 | 22.5 to 22.5 s over 3 stamped runs | - |
| otv-out-33.6s | `TAPPED_GRACE=28s PLANT_TAP_DRAIN=33.6s` | 3 | 0 | 3 | 28.0 to 28.0 s over 3 stamped runs | the output tap received 2097152 bytes, want the agent's 2097208, equal (3x) |
| otv-break-28s | `TAPPED_GRACE=28s BREAK=tapskip` | 10 | 0 | 10 | 0.0 to 0.1 s over 10 stamped runs | the output tap received 2031672 bytes, want the agent's 2097208, equal (10x) |
| otv-break-head | `BREAK=tapskip` | 10 | 0 | 10 | 0.0 to 0.1 s over 10 stamped runs | the output tap received 2031672 bytes, want the agent's 2097208, equal (10x) |
| crlo-base | `LAST_GRACE=39s` | 3 | 3 | 0 | 0.4 to 0.5 s over 3 stamped runs | - |
| crlo-in-31.2s | `LAST_GRACE=39s PLANT_EXIT_AT=31.2s` | 3 | 3 | 0 | 31.2 to 31.2 s over 3 stamped runs | - |
| crlo-out-46.8s | `LAST_GRACE=39s PLANT_EXIT_AT=46.8s` | 3 | 0 | 3 | 39.0 to 39.0 s over 3 stamped runs | Close = agentproc: agent terminated: signal: terminated, want nil: the agent exits of itself (3x) |
| crlo-break-39s | `LAST_GRACE=39s BREAK=lastclose` | 10 | 0 | 10 | 0.0 to 0.1 s over 10 stamped runs | Close = signal: broken pipe, want nil: the agent exits of itself (10x) |
| crlo-break-head | `BREAK=lastclose` | 10 | 0 | 10 | 0.0 to 0.1 s over 10 stamped runs | Close = signal: broken pipe, want nil: the agent exits of itself (10x) |
| cc-base | `CC_WAIT_BOUND=33s` | 3 | 3 | 0 | 0.3 to 0.3 s over 3 stamped runs | - |
| cc-in-26.4s | `CC_WAIT_BOUND=33s PLANT_HUNG_LAST=26.4s` | 3 | 3 | 0 | 26.4 to 26.4 s over 3 stamped runs | - |
| cc-out-39.6s | `CC_WAIT_BOUND=33s PLANT_HUNG_LAST=39.6s` | 3 | 0 | 3 | 39.6 to 39.6 s over 1 stamped runs | Wait took longer than 33s (3x) |
| cc-break-33s | `CC_WAIT_BOUND=33s BREAK=waitHang` | 2 | 0 | 2 | 60.3 to 60.3 s over 2 stamped runs | Wait took longer than 33s (8x) |

Each variable in "Environment" is `ZZ_`-prefixed. A `-head` case has the bound as `HEAD` has it (3 s, 3 s, 3 s, 5 s) and the other knobs unset: for `TestNoLostOutput` and `TestTapErrorIgnored` the agent's grace is phase 2's 3 s in both, not `HEAD`'s 300 ms. "Waited" has fewer stamped runs where a run failed at its bound before the waited thing ended; the dump ran first.

Results, by bound:

* **`TestNoLostOutput`, 35 s.** Ending at 0.8 of it (28 s), 3 of 3 passed (waited 28.0 s). Ending at 1.2 of it (42 s), 3 of 3
  failed, "Wait took longer than 35s" (waited 42.0 s). No plant, 3 of 3 passed in 0.3 to 0.5 s. The break (the connection closed
  at the agent's exit): 10 of 10 failed at 35 s and 10 of 10 at `HEAD`'s 3 s, each "the client received N of the 1000 session
  updates the agent wrote", N from 432 to 475 and from 428 to 490. The figure does not touch what the break fails.
* **`TestTapErrorIgnored`, 39 s.** 0.8 (31.2 s): 3 of 3 passed (31.2 s). 1.2 (46.8 s): 3 of 3 failed, "Wait took longer than 39s".
  The break (a tap's `Write` error returned to the connection's reader): 10 of 10 failed at 39 s, "Wait = agentproc: agent
  terminated: signal: terminated, want nil" after 3.0 s, by the agent's 3 s grace and not by the bound (the connection ends, the
  agent is stopped as `Close` stops it), and 10 of 10 failed at 3 s, "Wait took longer than 3s", the bound and the grace being equal.
  The test fails on the defect with the bound at 39 s because the defect ends the connection, which a bound does not touch.
* **`TestOutputTapVerbatim`, 28 s.** 0.8 (22.4 s after the drain began): 3 of 3 passed (waited 22.5 s). 1.2 (33.6 s): 3 of 3
  failed, "the output tap received 2097152 bytes, want the agent's 2097208, equal", the drain having ended at 28.0 s with the last
  56 bytes' `Write` not done. The break (a tap that skips one queued write): 10 of 10 failed at 28 s and 10 of 10 at 3 s,
  "the output tap received 2031672 bytes, want the agent's 2097208, equal".
* **`TestCloseReadsTheAgentsLastOutput`, 39 s.** 0.8 (31.2 s after the agent's go): 3 of 3 passed (the agent's exit seen at 31.2
  s). 1.2 (46.8 s): 3 of 3 failed, "Close = agentproc: agent terminated: signal: terminated, want nil: the agent exits of itself",
  after 39.0 s, the first step. The break (`stop` closes the connection at once): 10 of 10 failed at 39 s and 10 of 10 at `HEAD`'s
  5 s, "Close = signal: broken pipe, want nil: the agent exits of itself".
  *Replaced 2026-11-16: the figure is 42 s; its margin plants and break re-run (the addendum).*
* **`closecontext_test.go:156`, 33 s, in the moved copy.** 0.8 (26.4 s after the release): 3 of 3 passed (waited 26.4 s). 1.2 (39.6
  s): 3 of 3 failed, "Wait took longer than 33s", in the child `CloseContext` subtest (the only one whose handler reaches the
  1000th update; the `KillContext` subtest kills the agent before it has written them, and the in-process ones have one update).
  The break (this phase's choice, above): 2 of 2 runs failed in all four subtests, "Wait took longer than 33s", 8 of 8 subtests.
  *Replaced 2026-11-16: the figure is 35 s; its margin plants and break re-run, and the break also at `HEAD`'s 3 s (the addendum).*

#### The fifth bound

By question 5's answer: it is rescaled only if it still fails once its subtest's grace and grace-stated bound have moved.
**It does.** Above 3 s, the bound as it stands, in the moved copy:

* `a child, CloseContext`: 0 of 20 beside the module, 20 of 20 in-process (tests alone), 2 of 2 in the whole-package iterations
* `a child, KillContext`: 0 of 20 beside the module, 3 of 20 in-process (tests alone), 0 of 2 in the whole-package iterations
* `in process, CloseContext`: 0 of 20 beside the module, 0 of 20 in-process (tests alone), 0 of 2 in the whole-package iterations
* `in process, KillContext`: 0 of 20 beside the module, 0 of 20 in-process (tests alone), 0 of 2 in the whole-package iterations

and in the copy as `HEAD` has it, the child `CloseContext` subtest exceeded 3 s in 8 of 20 in-process runs and the child
`KillContext` one in 5 of 20, none beside the module. In P1, where the 3 s bound was in force, both child subtests of the moved
copy failed it, and one of `HEAD`'s. So the move of the grace and the bound does not make `closecontext_test.go:156` pass under the
in-process load; it makes it fail more often than before, since the child then writes all 1000 updates before the context's end.
By steps 3 to 5 the figure is **33 s** (6448.5 ms x 5 = 32.243 s, up to 33 s), with its margin and its break as above. Phase 2
rescales that one line, for all four subtests, to 33 s.

#### W4

No bound stops: for each of the five, a figure follows from the measurement, its margin passes at 0.8 and fails at 1.2, and the
break that fails the test at `HEAD`'s bound fails it at the figure. The fifth bound's break is this phase's choice, W2 having none
for it, and is put to the coordinator for review with the figures' size, above.

#### What distorted a measurement, and the odd runs

* **Queue and other agents' work:** none; every chunk started at once (`waited=0s`) and the host ran only this phase's heavy runs
  (each chunk printed `uptime` at its start, a 1-minute load of 0.08 to 3.04 on four CPUs; the larger ones are the previous chunk's
  own tail, chunks following each other at once).
* **F7's tests, as load:** the module rounds ran the two 56 MiB tests and `TestBlockedTapDetached` as CI does, and they moved
  megabytes and waited 30 s in most rounds (below). They are part of the module load as measured, and not separated from it. This
  changes no figure: each is set by an in-process sample, the module's worst being 1.3 to 7 times smaller than it. A run with them
  skipped was not made.
* **A stamp's order:** `go-sent` is sometimes later than the agent's receipt of the go (down to -200 ms in P1's
  `TestTapErrorIgnored`): `SendNotification` returns after the agent has read, and the caller's goroutine was starved. The
  bound starts when `SendNotification` returns, which is what `within-start` stamps; the time before it is not in the quantity,
  and is not a clock error (one clock for both processes).
* **Failed assertions with the bounds lifted:** the copy of `HEAD`'s `TestCloseContextBoundsHandlers` failed in all 20 runs of
  the in-process load (its child subtests: "CloseContext = context deadline exceeded ... want the bound's cause and
  acp.ErrHandlersRunning", at its 300 ms grace and bound), and in none beside the module. That is W1's, not this phase's: the
  quantity measured, `Wait` after the release, was stamped all the same. No other measured test failed in the tests-alone load.
  The whole-package iterations failed class A and B tests only, besides F7's (below).

#### F7's tests, counted apart

Run, not skipped, in the module rounds and in the whole-package iterations. Beside the module, 20 rounds: `TestDetachedTapIsPrefix`
failed in 14 ("the bulk agent did not say it was ready within 30s", `tap_test.go:365` in the copy, and once "Close = agentproc:
agent terminated: signal: terminated"), `TestInProcessDetachedTapIsPrefix` in 7 ("the connection did not read Serve's first 56
MiB within 30s"), `TestBlockedTapDetached` in 2 ("the bulk agent did not say it was ready within 30s"); 15 of the 20 rounds had
at least one of them fail, 5 none; no other test failed in a module round. In the two whole-package in-process iterations
(`burnfull-01`, `-02`) all three failed in both, besides class A and B tests: `TestCloseLadder`, `TestCallersCommandKept`,
`TestCallersPdeathsigKept`, `TestDefaultPdeathsig`, `TestExitedFirst`, `TestProcessWaitContext`, `TestLineTapAsOutputTap`,
`TestTapDrainBoundedByGrace`, `TestCloseContextBoundsHandlers` (the copy of `HEAD`'s, its child subtests) in both, and
`TestTapKeepsCloseEnding`, `TestKillAfterExit`, `TestDescendantsEnded` in one and `TestEscapedDescendantDoesNotHoldWait` in the other. None is class C or D;
the five bounds' tests, with their bounds lifted, did not fail there. Not counted as passes, nor as this phase's failures.

#### Not verified

* The figures are from one 4-CPU Linux host under `-race`; CI's runners, macOS and Windows were not measured, and the module
  load here includes F7's tests moving megabytes where CI's step passes them.
* Twenty module rounds and twenty runs of the tests alone are a sample: a worst of 20 is not the worst there is. The whole-package
  load, the realest in-process shape, ran 2 iterations of the 20 the step asks (about 6 minutes each); one of its two runs set
  a figure (`TestTapErrorIgnored`'s). Whether a third would have raised others was not measured.
  *Annotated 2026-11-16: superseded; the whole-package load was run to 20 iterations (the addendum), and the further iterations
  raised two figures.*
* The measured copies are phase 2's shape, not phase 2's edits: no test was moved in the tree, and `startCmdGrace`, `exitGrace`
  and the rest do not exist yet. The moved copy of the fifth bound is a copy of the test with the three constants changed.
* A margin plant delays one wait by a sleep; it shows where the bound's edge is, not that the delayed run would otherwise last
  so long, and it ran idle, not under load. The plants delay the waited thing and no other part.
* The breaks were chosen to fit W2's descriptions (and the fifth bound's by this phase); that each is the defect its bound is
  for was read, not proved. `TestTapErrorIgnored`'s break fails by the agent's 3 s grace, not by its bound, which is the way
  the defect shows.
* The stamps' own cost (a lock and an append each) was not measured.
* `GOMAXPROCS=2` was inherited by the agent children the tests start, as in the audit's rounds; the agents are not starved by
  the in-process load, only slowed by two Ps.

#### Files, checks and staging

Touched: this PLAN (this entry; phase 1b's step 8; the question 5 annotation and answer), the MADR (question 5's answer) and
`docs/README.md` (the MADR's and the PLAN's rows). Not touched: any test or source file, any workflow, any script. Scratch only,
not staged: the three copies' builder scripts and Go files, the chunk scripts and cases, the analysis and table scripts, every
chunk's output and the tables; the tools are saved outside the scratch folder for phase 2, whose stamping and planting tools
they are.

After the last edit, with the PLAN, the MADR and `docs/README.md` staged: `python3 -B scripts/check_records.py --check-all`,
`python3 -B scripts/check_disclosure.py` and `markdownlint-cli2` v0.23.2, each exit 0. Staged, not committed, not pushed.

#### Addendum, 2026-11-16, after the coordinator's review: the whole-package load completed to 20, and two figures replaced

Added the same day, on the coordinator's message that a figure resting on 2 whole-package runs is a gap in a step that is in scope.
Nothing above is rewritten: where it is superseded, it is annotated and points here.

* **The PLAN's sentence, and what met it.** Step 2 of phase 1b: "**Two loads**, each test 20 runs under each, `-race`: beside the
  whole module's `go test -race -shuffle=on ./...`, as CI's step runs; and the in-process load (32 busy goroutines,
  `GOMAXPROCS=2`) started from the scratch test binary's own start, by an `init` in a scratch-only file when `BURN` is set, so that it
  lasts the whole run and no test runs outside it." The step asks for 20 runs of each test under each load, not for 20 iterations
  of the whole package; the `burnsel` chunks (the measured tests alone, 20 runs each) met it, and the whole-package iterations were
  this phase's own addition, a cross-check on a heavier shape. The question was only whether a figure may rest on the two-run
  sample of that addition. It may not: it set `TestTapErrorIgnored`'s figure. So the whole-package load was run to 20 iterations,
  and all the figures are recomputed from the full data.
* **The chunks.** Iterations 3 to 20, two to a chunk, each through `run-on-host --need linux --race --`, one at a time on the host,
  never a WSL distro, each with `waited=0s`. The host-side script (`zzburn2.sh`) wrote each iteration's output, its exit status and
  its stamps to files kept on the host outside the dispatcher's work root, so that a lost connection could not lose an `exit`
  line; none was lost in these nine chunks, and the host-side record was not needed. Iteration 2 stays as recovered over ssh (the
  first report); its `exit` line was not recorded.

| Chunk | Command | Iterations | Seeds | Seconds | Iterations with a failing test |
| :--- | :--- | :--- | :--- | ---: | ---: |
| burnfull-03 | `sh zzburn2.sh bf03 3 4` | 3 to 4 | 1794861880569491, 1794862299570749 | 419, 341 (dispatch exit 0) | 2 of 2 |
| burnfull-04 | `sh zzburn2.sh bf04 5 6` | 5 to 6 | 1794862656571821, 1794862996572842 | 339, 336 (dispatch exit 0) | 2 of 2 |
| burnfull-05 | `sh zzburn2.sh bf05 7 8` | 7 to 8 | 1794863347573896, 1794863695574941 | 348, 319 (dispatch exit 0) | 2 of 2 |
| burnfull-06 | `sh zzburn2.sh bf06 9 10` | 9 to 10 | 1794864030575947, 1794864344576890 | 314, 331 (dispatch exit 0) | 2 of 2 |
| burnfull-07 | `sh zzburn2.sh bf07 11 12` | 11 to 12 | 1794864690577929, 1794865028578944 | 338, 328 (dispatch exit 0) | 2 of 2 |
| burnfull-08 | `sh zzburn2.sh bf08 13 14` | 13 to 14 | 1794865371579974, 1794865681580905 | 310, 352 (dispatch exit 0) | 2 of 2 |
| burnfull-09 | `sh zzburn2.sh bf09 15 16` | 15 to 16 | 1794866048582007, 1794866392583040 | 344, 359 (dispatch exit 0) | 2 of 2 |
| burnfull-10 | `sh zzburn2.sh bf10 17 18` | 17 to 18 | 1794866770584175, 1794867156585334 | 386, 316 (dispatch exit 0) | 2 of 2 |
| burnfull-11 | `sh zzburn2.sh bf11 19 20` | 19 to 20 | 1794867699586964, 1794868036587976 | 337, 386 (dispatch exit 0) | 2 of 2 |
| pl-crlo42 | `sh zzcases.sh cases-crlo42.txt` | plant cases, `TestCloseReadsTheAgentsLastOutput` at 42 s | none (`-test.shuffle=off`) | 262 | - |
| pl-cc35 | `sh zzcases.sh cases-cc35.txt` | plant cases, the fifth bound at 35 s, and at 3 s | none (`-test.shuffle=off`) | 723 | - |

* **The whole-package iterations' failures,** 20 iterations (iteration 2 from its recovered output), a count of the iterations each
  test failed in:

  | Iterations failed (of 20) | Test |
  | ---: | :--- |
  | 20 | `TestBlockedTapDetached` |
  | 20 | `TestCloseContextBoundsHandlers` |
  | 20 | `TestCloseContextBoundsHandlers/a_child,_CloseContext` |
  | 20 | `TestCloseContextBoundsHandlers/a_child,_KillContext` |
  | 20 | `TestCloseLadder` |
  | 20 | `TestCloseLadder/exits_at_its_input's_end` |
  | 20 | `TestCloseLadder/exits_on_SIGTERM` |
  | 20 | `TestDetachedTapIsPrefix` |
  | 20 | `TestExitedFirst` |
  | 20 | `TestExitedFirst/the_agent_exits_first` |
  | 20 | `TestExitedFirst/the_connection_ends_first` |
  | 20 | `TestInProcessDetachedTapIsPrefix` |
  | 20 | `TestTapDrainBoundedByGrace` |
  | 18 | `TestProcessWaitContext` |
  | 15 | `TestCallersPdeathsigKept` |
  | 15 | `TestProcessWaitContext/one_goroutine` |
  | 14 | `TestCallersCommandKept` |
  | 14 | `TestDefaultPdeathsig` |
  | 14 | `TestLineTapAsOutputTap` |
  | 14 | `TestProcessWaitContext/ctx_ends_first` |
  | 13 | `TestDescendantsEnded` |
  | 12 | `TestDescendantsEnded/the_agent_exits_at_its_input's_end` |
  | 7 | `TestTapKeepsCloseEnding` |
  | 3 | `TestInProcessKill` |
  | 3 | `TestKillAfterExit` |
  | 2 | `TestDescendantsEnded/the_agent_exits_of_itself` |
  | 2 | `TestEscapedDescendantDoesNotHoldWait` |
  | 1 | `TestCloseContextBoundsHandlers/in_process,_KillContext` |
  | 1 | `TestConnectionEndStopsAgent` |
  | 1 | `TestConnectionEndStopsAgent/a_notification_overflow` |
  | 1 | `TestProcessWaitContext/the_agent_ends_first` |

  Each is a class A or B test of the audit's table, `TestCloseContextBoundsHandlers/in_process,_KillContext` (one of W1's four), or
  F7's (`TestBlockedTapDetached`, `TestDetachedTapIsPrefix` and `TestInProcessDetachedTapIsPrefix`, each in 20 of 20); none is a
  class C or D test, and the five bounds' tests, with their bounds lifted, did not fail. F7's three are counted apart, not as passes
  or as this phase's failures. Several class A tests the audit's five rounds had not seen fail failed here (`TestTapKeepsCloseEnding`,
  `TestKillAfterExit`, `TestEscapedDescendantDoesNotHoldWait`, `TestInProcessKill`), which is what the load's covering the whole run
  was for.

#### The figures, from the full data

Milliseconds. For each bound, median, p90 and max over the 20 runs of each load; "whole package" is now 20 iterations. The
quantity is the one the figure's bound waits for, as above. Five times the worst over all three samples, rounded up to a whole
second.

| Bound | Module, 20: median / p90 / max | In-process, tests alone, 20: median / p90 / max | In-process, whole package, 20: median / p90 / max | Worst | Five times | Figure | First report's |
| :--- | :--- | :--- | :--- | ---: | ---: | ---: | ---: |
| `TestNoLostOutput`, `Wait` | 20: 500 / 742 / 899 | 20: 5328 / 6526 / 6920 | 20: 5453 / 6146 / 6421 | 6920.2 | 34.60 s | **35 s** | 35 s |
| `TestTapErrorIgnored`, `Wait` | 20: 465 / 758 / 1049 | 20: 5056 / 5469 / 5735 | 20: 5295 / 6691 / 7669 | 7668.7 | 38.34 s | **39 s** | 39 s |
| `TestOutputTapVerbatim`, the drain (`tap_test.go:195`) | 20: 176 / 271 / 302 | 20: 3469 / 4871 / 5596 | 20: 3895 / 4655 / 5086 | 5595.9 | 27.98 s | **28 s** | 28 s |
| `TestCloseReadsTheAgentsLastOutput`, the first step | 20: 706 / 948 / 1154 | 20: 6066 / 7325 / 7603 | 20: 6277 / 7244 / 8297 | 8297.0 | 41.49 s | **42 s** | 39 s |
| `closecontext_test.go:156`, as moved, all four subtests | 80: 4.4 / 379 / 1384 | 80: 374 / 4855 / 6449 | 80: 776 / 4882 / 6821 | 6820.8 | 34.10 s | **35 s** | 33 s |

What changed, and what did not:

* **`TestNoLostOutput`: unchanged, 35 s.** The worst is still 6920.2 ms (the tests-alone load, chunk `burnsel-04`); the 20 whole-package runs' worst is 6421.0 ms (iteration 4), beside it. 6920.2 ms x 5 = 34.601 s, up to 35 s.
* **`TestTapErrorIgnored`: unchanged, 39 s.** The worst is still 7668.7 ms, the whole-package iteration 1, now the largest of 20 and no longer of 2; the tests-alone worst is 5734.9 ms. 7668.7 ms x 5 = 38.343 s, up to 39 s.
* **`TestOutputTapVerbatim`: unchanged, 28 s.** The worst drain is still 5595.9 ms (tests alone); the whole-package worst is 5086.0 ms (iteration 12), beside it. 5595.9 ms x 5 = 27.979 s, up to 28 s.
* **`TestCloseReadsTheAgentsLastOutput`: replaced, 39 s by 42 s.** The whole-package load's iteration 5 had the first step take 8297.0 ms, above the 7602.7 ms the figure of 39 s rested on. 8297.0 ms x 5 = 41.485 s, up to 42 s. The first report's 39 s stands in the record above as what it was, and is replaced by 42 s: the full data's worst, by the same rule. `within(c.Grace+slack)` at `tap_test.go:397` follows, 42 s + 3 s = 45 s.
* **`closecontext_test.go:156`: replaced, 33 s by 35 s.** The whole-package load's iteration 13 had the child `CloseContext` subtest's `Wait` take 6820.8 ms, above the 6448.5 ms the figure of 33 s rested on. 6820.8 ms x 5 = 34.104 s, up to 35 s. The first report's 33 s stands above as what it was, replaced by 35 s.

The old figures stay where they were written (the first report's table and results, above), annotated with what replaced them; they
were fixed before any plant ran, as were these, and neither set was raised until a run passed.

#### The worst against the rest

For each bound, the five largest values of each load, and how far the largest stands above the p90 and above the next, so that a
reader can tell an outlier from the edge of a steady spread.

| Bound | Load | Median | p90 | Max | Five largest | Max / p90 | Max above the next |
| :--- | :--- | ---: | ---: | ---: | :--- | ---: | ---: |
| TestNoLostOutput | module | 500 | 742 | 899 | 899, 788, 742, 733, 637 | 1.21 | 111 ms |
| TestNoLostOutput | in-process, tests alone | 5328 | 6526 | 6920 | 6920, 6861, 6526, 6343, 6149 | 1.06 | 59 ms |
| TestNoLostOutput | in-process, whole package | 5453 | 6146 | 6421 | 6421, 6303, 6146, 6034, 6005 | 1.04 | 118 ms |
| TestTapErrorIgnored | module | 465 | 758 | 1049 | 1049, 793, 758, 756, 700 | 1.38 | 256 ms |
| TestTapErrorIgnored | in-process, tests alone | 5056 | 5469 | 5735 | 5735, 5703, 5469, 5429, 5411 | 1.05 | 32 ms |
| TestTapErrorIgnored | in-process, whole package | 5295 | 6691 | 7669 | 7669, 6700, 6691, 6688, 6605 | 1.15 | 969 ms |
| TestOutputTapVerbatim (drain) | module | 176 | 271 | 302 | 302, 281, 271, 254, 249 | 1.12 | 21 ms |
| TestOutputTapVerbatim (drain) | in-process, tests alone | 3469 | 4871 | 5596 | 5596, 5430, 4871, 4601, 4370 | 1.15 | 166 ms |
| TestOutputTapVerbatim (drain) | in-process, whole package | 3895 | 4655 | 5086 | 5086, 4703, 4655, 4558, 4536 | 1.09 | 383 ms |
| TestCloseReadsTheAgentsLastOutput (first step) | module | 706 | 948 | 1154 | 1154, 1008, 948, 923, 894 | 1.22 | 146 ms |
| TestCloseReadsTheAgentsLastOutput (first step) | in-process, tests alone | 6066 | 7325 | 7603 | 7603, 7362, 7325, 6799, 6701 | 1.04 | 241 ms |
| TestCloseReadsTheAgentsLastOutput (first step) | in-process, whole package | 6277 | 7244 | 8297 | 8297, 7512, 7244, 6878, 6771 | 1.15 | 785 ms |
| fifth, moved, child CloseContext | module | 375 | 571 | 1384 | 1384, 774, 571, 474, 423 | 2.43 | 610 ms |
| fifth, moved, child CloseContext | in-process, tests alone | 4711 | 5306 | 6449 | 6449, 5963, 5306, 5279, 5210 | 1.22 | 485 ms |
| fifth, moved, child CloseContext | in-process, whole package | 4734 | 5865 | 6821 | 6821, 5944, 5865, 5484, 5277 | 1.16 | 877 ms |

Read: in the tests-alone load the maximum is 1.04 to 1.22 times the p90 and 32 to 485 ms above the next value, the edge of a
spread (the fifth bound's, 6448.5 ms, is the widest, 1.22 and 485 ms). The whole-package samples are wider at the top:
`TestTapErrorIgnored`'s maximum, 7668.7 ms, stands 969 ms above the next (6700.0 ms) and 1.15 times the p90, one run (iteration 1)
above a steady 6600 to 6700 ms; `TestCloseReadsTheAgentsLastOutput`'s, 8297.0 ms, 785 ms above the next (7512.1 ms, iteration 5);
the fifth bound's, 6820.8 ms, 877 ms above the next (5944 ms, iteration 13). Under the rule they set their figures, being the
worst measured; without them the three figures would be 34 s, 39 s and 33 s (6700.0, 7602.7 and 6448.5 ms, times five, up). The
module's maxima stand 1.12 to 2.43 times their p90, the spread being a tenth of the in-process one's, and set no figure.

#### The margins and the breaks, again, for the two figures that changed

The same cases as above, at the new figures (idle, `-race`, `-test.shuffle=off`; the knobs and plants are those above).

| Case | Environment | Runs | Passed | Failed | Waited | Failure message (count) |
| :--- | :--- | ---: | ---: | ---: | :--- | :--- |
| crlo-base-42s | `LAST_GRACE=42s` | 3 | 3 | 0 | 0.4 to 0.5 s over 3 stamped runs | - |
| crlo-in-33.6s | `LAST_GRACE=42s PLANT_EXIT_AT=33.6s` | 3 | 3 | 0 | 33.6 to 33.6 s over 3 stamped runs | - |
| crlo-out-50.4s | `LAST_GRACE=42s PLANT_EXIT_AT=50.4s` | 3 | 0 | 3 | 42.0 to 42.0 s over 3 stamped runs | Close = agentproc: agent terminated: signal: terminated, want nil: the agent exits of itself (3x) |
| crlo-break-42s | `LAST_GRACE=42s BREAK=lastclose` | 10 | 0 | 10 | 0.0 to 0.0 s over 10 stamped runs | Close = signal: broken pipe, want nil: the agent exits of itself (10x) |
| cc-base-35s | `CC_WAIT_BOUND=35s` | 3 | 3 | 0 | 0.3 to 0.4 s over 3 stamped runs | - |
| cc-in-28s | `CC_WAIT_BOUND=35s PLANT_HUNG_LAST=28s` | 3 | 3 | 0 | 28.0 to 28.0 s over 3 stamped runs | - |
| cc-out-42s | `CC_WAIT_BOUND=35s PLANT_HUNG_LAST=42s` | 3 | 0 | 3 | 42.0 to 42.0 s over 2 stamped runs | Wait took longer than 35s (3x) |
| cc-break-35s | `CC_WAIT_BOUND=35s BREAK=waitHang` | 2 | 0 | 2 | 60.3 to 60.3 s over 2 stamped runs | Wait took longer than 35s (8x) |
| cc-break-3s-moved | `BREAK=waitHang` | 2 | 0 | 2 | 60.3 to 60.3 s over 2 stamped runs | Wait took longer than 3s (8x) |
| cc-break-3s-head | `BREAK=waitHang` | 2 | 0 | 2 | - | Wait took longer than 3s (8x) |

A `-3s` case has the bound as `HEAD` has it, 3 s, and no other knob but the break; `-3s-head` runs the copy of `HEAD`'s `TestCloseContextBoundsHandlers`, whose four subtests run the same `Wait` bound.

* **`TestCloseReadsTheAgentsLastOutput`, 42 s.** 0.8 (33.6 s after the agent's go): 3 of 3 passed. 1.2 (50.4 s): 3 of 3 failed,
  "Close = agentproc: agent terminated: signal: terminated, want nil: the agent exits of itself", after 42.0 s. The break (`stop`
  closes the connection at once): 10 of 10 failed, "Close = signal: broken pipe, want nil: the agent exits of itself".
* **`closecontext_test.go:156`, 35 s, in the moved copy.** 0.8 (28 s after the release): 3 of 3 passed. 1.2 (42 s): 3 of 3 failed,
  "Wait took longer than 35s" in the child `CloseContext` subtest. The break (this phase's choice, "5a" below): 2 of 2 runs failed in all four subtests at 35 s (Wait took longer than 35s (8x)); at `HEAD`'s 3 s, 2 of 2 runs of the moved copy (Wait took longer than 3s (8x)), and 2 of 2 runs of the copy of `HEAD`'s test (Wait took longer than 3s (8x)).

#### Point 5a: the defect `closecontext_test.go:156` is for, and the fifth bound's break

*The break below is the executing agent's choice, W2 giving none for this bound; it is for the coordinator to confirm.*

*Confirmed 2026-11-16 by the coordinator, as the coordinator's confirmation and not the owner's, on this evidence: the test's comment
says that once the handler is released `Wait` returns the agent's error; the break delays `Wait`'s return after its connection wait
has ended; and it fails `HEAD`'s test at 3 s in all four subtests, in 2 of 2 runs.*

* **What the line is for.** The test's own comment (`closecontext_test.go:118`-`:124`) says that `CloseContext` and `KillContext`
  return within their bound, with `acp.ErrHandlersRunning` joined, "when a client handler ignores its context", and that "Once the
  handler is released, Wait returns that agent's error, which Close or Kill would have returned" (the amendment of 2026-10-30,
  X1 and X2, of [0042-MADR-in-process-agent-runner.md](0042-MADR-in-process-agent-runner.md)). The code that follows the release is
  `release()` and `_, werr := within(t, slack, "Wait", p.Wait)` (`closecontext_test.go:155`-`:156`), then the assertion on `werr`.
  X1's text says what a `Wait` that returns means ("a `Wait` that returns still means that every handler the connection ran has
  returned", I7), and `Process.Wait`'s doc (`agentproc/agentproc.go:408`-`:411`) says it waits for the connection's goroutines
  without bound, until the handler returns. So the bound at `:156` is the test's check that, once the handler is released, `Wait`
  does return: the defect it catches is a `Wait` (or the one goroutine `CloseContext` and `KillContext` share with
  `WaitContext`) left blocked after its handlers have returned, so that a caller who bounded the close and released the handler
  still waits for ever.
* **Why the break is that defect.** The break lets `Wait`'s wait on the connection end, which is the handlers' return, and then
  makes `Wait` sleep a minute before it returns: `Wait` does not return once its handlers have. It is placed after the point at
  which `connWaitEnded` is set, so that `CloseContext`'s own check (the error joins `acp.ErrHandlersRunning` only while the wait on
  the connection is pending) is unaffected and only the post-release `Wait` fails, which is the line's assertion and nothing else.
* **It fails at `HEAD`'s 3 s as well as at the figure,** in the moved copy and in the copy of `HEAD`'s test (table above): the same
  break with no bound knob, 3 s.

#### Point 5b: what the line at `agentproc/tap_test.go:195` bounds

The line is `p = startTapped(t, "frames", nil, slack, out, in, acp.WithMaxMessageSize(1<<20))`: `slack` is the agent's `Grace`. The
comment above `tappedFrames` (`tap_test.go:186`-`:191`) says why: "Wait gives the taps up to the grace to drain". The grace is read by
three timers in the product, each started where its own wait starts:

* `Close`'s first step, `case p.exitedWithin(p.grace):` (`agentproc/agentproc.go:658`), from the agent's input's close;
* the watcher's, `t := time.NewTimer(p.grace)` (`agentproc/agentproc.go:368`), from the agent's exit, for the connection to read
  to the end of the agent's output;
* the drain's, `t := time.NewTimer(p.grace)` in `drainTaps` (`agentproc/agentproc.go:529`), from the drain's start, over
  `select { case <-tp.done: continue; case <-t.C: expired = true }` (`:538`-`:543`).

No timer starts at the tap's first `Write`, so the line does **not** bound the tap's whole delivery: the `Write`s that are done
before the drain starts, and the time they took, are outside every timer. What the test then needs, at `:218`-`:222`, is that the
tap has received every byte when `Close` returns, which the drain's timer ending at the grace puts at risk (the tap is abandoned and
what it has not written is dropped, `agentproc/tap.go:76`). The line therefore waits on three things, each of its own start, and
the greatest of their worsts is what the figure must cover: the drain, 5595.9 ms (the first step's worst is 843.5 ms and the
watcher's 281.9 ms, beside it). The whole delivery's worst, 6890.8 ms, is not a time the line waits for; it is only the longest the
drain could ever take, were it to start with the first `Write`. The figure, 28 s, is for the drain, and is unchanged by the full
data.

#### Not verified, as it now stands

The bullet "The worst of 20 is a sample statistic; the whole-package iterations (2 runs ...)" in "For review", and the one under "Not
verified" that says the whole-package load ran 2 of the 20 iterations, are superseded: all three samples are 20 runs of each test.
What stays: the figures are from one 4-CPU Linux host under `-race`; the worst of 20 is still a sample statistic, and the
maximum of three whole-package figures stands one run above its spread (above); a margin plant delays one wait, idle; each break
was read, not proved, to be the defect its bound is for.

#### The addendum's files, checks and staging

Touched: this PLAN (the addendum and the annotations of the entry above) and `docs/README.md` (the PLAN's row). Not touched: any test or
source file, any workflow, any script; the MADR is as the entry above left it. The new scripts (`zzburn2.sh`, the plant cases, the
addendum's table scripts) are scratch only, not staged, saved with the other tools for phase 2.

After the last edit, with the PLAN, the MADR and `docs/README.md` staged: `python3 -B scripts/check_records.py --check-all`,
`python3 -B scripts/check_disclosure.py` and `markdownlint-cli2` v0.23.2, each exit 0. Staged, not committed, not pushed.

### 2026-11-16: the owner's ruling on the five figures

Before phase 2 ran, the coordinator relayed one more ruling on phase 1b's figures, recorded here and in phase 2's "What moves" bullet and the MADR's W2 and W3 (the amendment of that date): the owner's ruling, relayed by the coordinator, "The five as measured". As put to the owner: 35 s, 39 s, 28 s, 42 s and 35 s, each from its own measurement (phase 1b's table and addendum): `TestNoLostOutput`'s `Wait` bound (`agentproc_test.go:174`) 35 s; `TestTapErrorIgnored`'s `Wait` bound (`tap_test.go:256`) 39 s; `tappedFrames`' grace (`tap_test.go:195`, `TestOutputTapVerbatim`'s drain) 28 s; `TestCloseReadsTheAgentsLastOutput`'s grace (`tap_test.go:377`) 42 s, its `within` following; the fixed `Wait` at `closecontext_test.go:156` 35 s. Phase 2's proof, 0 failures in 10 rounds under the starved load, can then hold for these tests; a passing run is no slower, and a hung wait takes up to its figure to fail. Options not chosen: one shared bound for all five; set them from the module load only. Nothing in the tree changed by it.

### 2026-11-16: the owner's ruling on the readiness failure

After phase 2's A/B, the coordinator relayed one more ruling, recorded here, in the PLAN's "Where the race gates run", phase 2's "Counted apart" bullet, phase 4's acceptance and phase 5, and in the MADR at W5: the owner's ruling, relayed by the coordinator, "Count it apart with the other readiness failure". As put to the owner: `TestTapDrainBoundedByGrace`'s grace move stays (its plant failed today's code and passes amended); its 30 s readiness failure ("the bulk agent did not say it was ready within 30s", `tap_test.go:441`) joins `TestBlockedTapDetached`'s for phase 5's measured report, by a dated amendment of the plan's gates; consistent with the earlier ruling "A measured report, then decide". Options not chosen: measure the readiness wait and rescale it now; revert its move. Nothing in the tree changed by it.

### 2026-11-16: phase 2 (the tests)

Run by a mainstream-tier model at medium effort, as the phase prices it, on the coordinator's brief. `date +%F` printed 2026-11-16,
the date of the earlier entries. Changed: fourteen files of `agentproc/*_test.go`, this entry and the PLAN's row in `docs/README.md`
(and the ruling's three lines, recorded first, above). No source file, workflow, script or tool changed. Every instrument below lived
in scratch copies made by `git archive HEAD`.

* **Approval and rulings.** "Run phases 1 to 4 through; then the report" (the approval entry), "Move all four, each proved first" and
  "Widen 0057 to these bounds now" (the entry on the audit's rulings), "Measure it; rescale only if it still fails" (question 5), and,
  recorded before this phase ran (the entry "2026-11-16: the owner's ruling on the five figures"), the owner's ruling, relayed by the
  coordinator, "The five as measured": the five figures 35 s, 39 s, 28 s, 42 s and 35 s; and, after the A/B (the entry above), "Count it apart with the other readiness failure". All as the earlier entries record them.
* **Base commit.** `032320a`, `main`, level with `origin/main`, clean. Line numbers below are that tree's.
* **Read:** this PLAN and its MADR; every `agentproc/*_test.go` and `agentproc.go`, `tap.go`, `inprocess.go`, `stderr.go`; `AGENTS.md`,
  `tools/README.md`; phase 1b's tools (the in-process load's `init`, the margin plants and breaks, the chunk scripts), which were
  read and then rewritten smaller for the amended files, not reused as they were.

#### What changed

* `agentproc_test.go`: `const exitGrace = slack`, its comment stating the rule and citing the MADR; `startCmdGrace`, which `startCmd`
  calls with `testGrace` (`start` and `startCmd` keep it); `const burstWait = 35 * time.Second`, `TestNoLostOutput`'s `Wait` bound,
  with its measurement in the comment; the two ladder subtests under `exitGrace`, their `within` and class B's bounds
  (`took < exitGrace`, `took >= 2*exitGrace`) in graces.
* `ties_test.go`: `startBeatingGrace` (`startBeating` keeps `testGrace`, so the Windows-only caller is unchanged); the moved tests
  of `TestDescendantsEnded`, `TestExitedFirst`, `TestCallersCommandKept` and `TestConnectionEndStopsAgent` under `exitGrace`, with
  `within(t, exitGrace+slack, ...)`.
* `closecontext_test.go`: `hungInProcess` takes its grace (`TestCloseContextOneGoroutine`, class D, passes `testGrace`);
  `hungChild` runs under `exitGrace`; `const bound = exitGrace` and the `within` that states it in graces, `:180` and `:246`'s
  contexts, `TestCloseContextNotHandlers` from 2 s to `exitGrace`; `const releasedWait = 35 * time.Second` for the fixed `Wait`
  of `:156`; `:214` stays.
* `tap_test.go`: `const tappedFramesGrace = 28 * time.Second` (`tappedFrames`' grace, which `TestInputTapSeesWrites` shares),
  `tapErrorWait` 39 s, `lastOutputGrace` 42 s (its `within` follows, 45 s); `TestTapErrorIgnored`, `TestBlockedTapDetached` and
  `TestTapDrainBoundedByGrace` under `exitGrace`.
* `inprocess_test.go`, `agentproc_unix_test.go`, `kill_test.go`, `linetap_test.go`, `pdeathsig_test.go`, `stderr_test.go`,
  `transcript_test.go`, `waitcontext_test.go`: each moved test's grace and `within`; `TestWaitErrorSaysTheCauseOnce`'s table gains a
  grace a case (the abandonment keeps `testGrace`).
* `breakaway_windows_test.go`, `suspended_windows_test.go`: the three Windows-only tests (below); `breakaway` takes a grace, `Close` passing `exitGrace` and `Kill` `testGrace`.
* A test already under `slack` or more kept its grace; tests of class C and D kept `testGrace`.

#### The moved tests

45 tests and subtests moved: class A 32 (of them `TestNoLostOutput` and `TestTapErrorIgnored` also carry W2's bounds), class B 6,
W1's four, and the three more of W2 whose graces rose (`TestOutputTapVerbatim` and `TestInputTapSeesWrites` through
`tappedFrames`, and `TestCloseReadsTheAgentsLastOutput`). W2's fifth bound, `closecontext_test.go:156`, moved in the four subtests of
`TestCloseContextBoundsHandlers`. The five bounds: 35 s, 39 s, 28 s, 42 s (its `within`, 45 s) and 35 s. Each row: the plant that fits
the test's wait, seen to fail at `HEAD` (the failure quoted) and to pass amended; "Amended" is the same plant on the amended tree.
Every moved test, the three Windows-only ones included, found a plant that fails it at `HEAD`; none was reverted.

| Test | Class | Grace and bounds before | After | Plant that fits | Failure seen at `HEAD` | Amended |
| :--- | :---: | :--- | :--- | :--- | :--- | :--- |
| `TestNoLostOutput` | A, W2 | grace 300 ms, `Wait` 3 s | grace 3 s, `Wait` 35 s | a grandchild holds stderr 1.2 s past the exit | `agentproc_test.go:176: Wait = exec: WaitDelay expired before I/O complete, want nil` | pass |
| `TestCloseLadder/exits_at_its_input's_end` | A | 300 ms; `took` under 300 ms, `within` 3 s | 3 s; under 3 s, `within` 6 s | exit seen 350 ms late | `agentproc_test.go:195: Close = agentproc: agent terminated after 353.348958ms, want nil within the grace, 300ms` | pass |
| `TestCloseLadder/exits_on_SIGTERM` | B | 300 ms; 300 to 600 ms | 3 s; 3 to 6 s, `within` 9 s | exit seen 350 ms late | `agentproc_test.go:210: Close = agentproc: agent killed: exit status 3 after 656.974084ms, want ErrTerminated with exit status 3 between one and two gr` | pass |
| `TestEscapedDescendantDoesNotHoldWait` | A | 300 ms, `within` 3.3 s | 3 s, `within` 6 s | a grandchild holds stderr 1.2 s past the exit | `agentproc_unix_test.go:103: Wait = exec: WaitDelay expired before I/O complete, want nil: the agent exits of itself` | pass |
| `TestTapKeepsCloseEnding` | A | 300 ms, `within` 3.3 s | 3 s, `within` 6 s | a grandchild holds stderr 1.2 s past the exit | `agentproc_unix_test.go:135: Wait = exec: WaitDelay expired before I/O complete, want nil: the agent exits of itself` | pass |
| `TestCloseContextBoundsHandlers/in_process,_CloseContext` | A | 300 ms, bound 300 ms | 3 s, bound 3 s, `within` 9 s | exit seen 350 ms late | `closecontext_test.go:150: CloseContext = context deadline exceeded` | pass |
| `TestCloseContextBoundsHandlers/in_process,_KillContext` | W1 | 300 ms, bound 300 ms | 3 s, bound 3 s, `within` 9 s | exit seen 350 ms late | `closecontext_test.go:150: KillContext = context deadline exceeded` | pass |
| `TestCloseContextBoundsHandlers/a_child,_CloseContext` | A | 300 ms, bound 300 ms, `Wait` 3 s | 3 s, bound 3 s, `Wait` 35 s | exit seen 350 ms late | `closecontext_test.go:150: CloseContext = context deadline exceeded` | pass |
| `TestCloseContextBoundsHandlers/a_child,_KillContext` | A | 300 ms, bound 300 ms, `Wait` 3 s | 3 s, bound 3 s, `Wait` 35 s | exit seen 350 ms late | `closecontext_test.go:150: KillContext = context deadline exceeded` | pass |
| `TestCloseContextJoinsTheAgentsError` | A | 300 ms, ctx 300 ms | 3 s, ctx 3 s, `within` 9 s | exit seen 350 ms late | `closecontext_test.go:185: CloseContext = context deadline exceeded` | pass |
| `TestCloseContextWaitsForHandlers` | A | 300 ms, release at 150 ms | 3 s, release at 1.5 s | exit seen 350 ms late | `closecontext_test.go:201: CloseContext = agentproc: agent terminated after 352.288042ms, want Wait's, nil` | pass |
| `TestCloseContextCause` | W1 | 300 ms, ctx 300 ms | 3 s, ctx 3 s, `within` 9 s | exit seen 350 ms late | `closecontext_test.go:250: CloseContext = the test's bound` | pass |
| `TestCloseContextNotHandlers` | W1 | 2 s, `within` 5 s | 3 s, `within` 6 s | a grandchild holds stderr 1.2 s past the exit | `closecontext_test.go:287: CloseContext = context deadline exceeded` | pass |
| `TestInProcessTerminate` | B | 300 ms; floor 300 ms | 3 s; floor 3 s, `within` 9 s | exit seen 350 ms late | `inprocess_test.go:116: Close = agentproc: agent killed: serve's own error, want ErrTerminated joined to Serve's error` | pass |
| `TestInProcessKill` | B | 300 ms, `within` 3.9 s | 3 s, `within` 12 s | exit seen 350 ms late | `inprocess_test.go:134: Close = agentproc: agent killed: agentproc: in-process agent abandoned, want ErrKilled joined to Serve's io.ErrClosedPipe, and ` | pass |
| `TestWaitErrorSaysTheCauseOnce/Kill,_Serve_returning_its_cause` | W1 | 300 ms, `within` 3.9 s | 3 s, `within` 12 s | exit seen 350 ms late | `inprocess_test.go:236: Wait's error = agentproc: agent killed: agentproc: in-process agent abandoned, want "agentproc: agent killed", and errors.Is ag` | pass |
| `TestWaitErrorSaysTheCauseOnce/step_2,_Serve_returning_its_cause` | B | 300 ms, `within` 3.9 s | 3 s, `within` 12 s | exit seen 350 ms late | `inprocess_test.go:236: Wait's error = agentproc: agent killed: agentproc: agent terminated, want "agentproc: agent terminated", and errors.Is agentpro` | pass |
| `TestWaitErrorSaysTheCauseOnce/step_2,_Serve_wrapping_its_cause` | B | 300 ms, `within` 3.9 s | 3 s, `within` 12 s | exit seen 350 ms late | `inprocess_test.go:236: Wait's error = agentproc: agent killed: serve: agentproc: agent terminated, want "serve: agentproc: agent terminated", and erro` | pass |
| `TestWaitErrorSaysTheCauseOnce/step_2,_Serve_returning_its_own_error` | B | 300 ms, `within` 3.9 s | 3 s, `within` 12 s | exit seen 350 ms late | `inprocess_test.go:236: Wait's error = agentproc: agent killed: serve's own error, want "agentproc: agent terminated: serve's own error", and errors.Is` | pass |
| `TestInProcessReturnsFirst/connection_first` | A | 300 ms, `within` 3.3 s | 3 s, `within` 6 s | exit seen 350 ms late | `inprocess_test.go:275: Wait = agentproc: agent terminated, want nil` | pass |
| `TestKillAfterExit` | A | 300 ms, `within` 3.3 s | 3 s, `within` 6 s | a grandchild holds stderr 1.2 s past the exit | `kill_test.go:65: Kill = exec: WaitDelay expired before I/O complete, want nil, the agent's exit of status 0` | pass |
| `TestLineTapAsOutputTap` | A | 300 ms | 3 s | exit seen 350 ms late | `linetap_test.go:153: Close = agentproc: agent terminated` | pass |
| `TestStderrTailOfFailedAgent` | A | 300 ms | 3 s | the standard-error copy slowed 800 ms a `Write`, lines 2 ms apart | `stderr_test.go:93: StderrTail = "line 1\n", want "line 98\nline 99\nline 100\n"` | pass |
| `TestStderrTailNotJoined` | A | 300 ms | 3 s | the standard-error copy slowed 800 ms a `Write`, lines 2 ms apart | `stderr_test.go:117: StderrTail = "line 1\n", want the stream's last 64 bytes or fewer, from a line` | pass |
| `TestTapErrorIgnored` | A, W2 | grace 300 ms, `Wait` 3 s | grace 3 s, `Wait` 39 s | exit seen 350 ms late | `tap_test.go:257: Wait = agentproc: agent terminated, want nil` | pass |
| `TestBlockedTapDetached` | A | 300 ms, `within` 3.3 s | 3 s, `within` 6 s | exit seen 350 ms late | `tap_test.go:312: Close = agentproc: agent terminated` | pass |
| `TestTapDrainBoundedByGrace` | A | 300 ms, `within` 3.3 s | 3 s, `within` 6 s | exit seen 350 ms late | `tap_test.go:425: Close = agentproc: agent terminated` | pass |
| `TestDescendantsEnded/the_agent_exits_at_its_input's_end` | A | 300 ms, `within` 3.3 s | 3 s, `within` 6 s | exit seen 350 ms late | `ties_test.go:56: Close = agentproc: agent terminated, want nil` | pass |
| `TestDescendantsEnded/the_agent_exits_of_itself` | A | 300 ms, `within` 3.3 s | 3 s, `within` 6 s | a grandchild holds stderr 1.2 s past the exit | `ties_test.go:84: Wait = exec: WaitDelay expired before I/O complete, want nil` | pass |
| `TestExitedFirst/the_agent_exits_first` | A | 300 ms, `within` 3.3 s | 3 s, `within` 6 s | a grandchild holds stderr 1.2 s past the exit | `ties_test.go:130: run 0: Wait = exec: WaitDelay expired before I/O complete, want nil` | pass |
| `TestExitedFirst/the_connection_ends_first` | A | 300 ms, `within` 3.3 s | 3 s, `within` 6 s | exit seen 350 ms late | `ties_test.go:134: run 0: Wait = agentproc: agent terminated, want nil` | pass |
| `TestCallersCommandKept` | A | 300 ms | 3 s | exit seen 350 ms late | `ties_test.go:149: Close = agentproc: agent terminated` | pass |
| `TestConnectionEndStopsAgent/a_notification_overflow` | A | 300 ms, 3.3 s | 3 s, 6 s | exit seen 350 ms late | `ties_test.go:223: Wait = agentproc: agent terminated, want nil: the flood agent exits at its input's end` | pass |
| `TestTranscriptOfSession` | A | 300 ms | 3 s | exit seen 350 ms late | `transcript_test.go:156: Close = agentproc: agent terminated` | pass |
| `TestProcessWaitContext/ctx_ends_first` | A | 300 ms | 3 s | exit seen 350 ms late | `waitcontext_test.go:38: Close = agentproc: agent terminated` | pass |
| `TestProcessWaitContext/the_agent_ends_first` | A | 300 ms | 3 s | a grandchild holds stderr 1.2 s past the exit | `waitcontext_test.go:50: WaitContext = exec: WaitDelay expired before I/O complete, want Wait's, nil: the agent exits 0 of itself` | pass |
| `TestProcessWaitContext/one_goroutine` | A | 300 ms | 3 s | exit seen 350 ms late | `waitcontext_test.go:71: Close = agentproc: agent terminated` | pass |
| `TestDefaultPdeathsig` | A | 300 ms | 3 s | exit seen 350 ms late | `pdeathsig_test.go:34: Close = agentproc: agent terminated` | pass |
| `TestCallersPdeathsigKept` | A | 300 ms | 3 s | exit seen 350 ms late | `pdeathsig_test.go:48: Close = agentproc: agent terminated` | pass |
| `TestOutputTapVerbatim` | W2 | `tappedFrames`' grace 3 s | 28 s | the output tap's last `Write` held to 4 s after the drain's start | `tap_test.go:229: the output tap received 2097152 bytes, want the agent's 2097208, equal` | pass |
| `TestInputTapSeesWrites` | W2 | `tappedFrames`' grace 3 s | 28 s | exit seen 3.5 s late | `tap_test.go:231: Close = agentproc: agent terminated` | pass at 350 ms and 3.5 s late |
| `TestCloseReadsTheAgentsLastOutput` | W2 | grace 5 s, `within` 8 s | grace 42 s, `within` 45 s | the agent's exit held to 6 s after its go | `tap_test.go:409: Close = agentproc: agent terminated: signal: terminated, want nil: the agent exits of itself` | pass |
| `TestBreakawayLeavesJob/Close` | A (Windows) | 300 ms, `within` 3.3 s | 3 s, `within` 6 s | exit seen 350 ms late, on the Windows laptop | `breakaway_windows_test.go:49: stop = agentproc: agent killed, want nil` | pass |
| `TestDescendantInJobDespiteSlowAttach` | A (Windows) | 300 ms, `within` 3.3 s | 3 s, `within` 6 s | the same | `suspended_windows_test.go:31: Close = agentproc: agent killed, want nil` | pass |
| `TestCallersCreationFlagsKept` | A (Windows) | 300 ms | 3 s | the same | `suspended_windows_test.go:81: Close = agentproc: agent killed, want nil: the agent runs, and exits at its input's end` | pass |

The plants (scratch only, each a no-op unset; the HEAD and the amended trees carry the same ones):

* **The client-side plant** (`PLANT_SEEN_LATE`): `childAgent.wait` sleeps after `Cmd.Wait`, and, for an `InProcess` test, the agent's
  `returned` is closed after the same sleep following `Serve`'s return, so the exit is seen that much late. It is a sleep, a
  stand-in for a starved watcher, not a starved one.
* **A standard-error copy that outlives `WaitDelay`** (`PLANT_LINGER`), for the twelve whose outcome has `WaitDelay` and no first
  step: every child, at its exit, leaves a grandchild in a process group of its own that holds the child's standard error 1.2 s
  (at `HEAD` `WaitDelay` is 150 ms, 1 s for `TestCloseContextNotHandlers`; amended 1.5 s), so `Cmd.Wait` returns `exec.ErrWaitDelay`
  for real. This one is real, not a sleep in the product.
* **A slow standard-error copy** for the two stderr-tail tests, which assert lines and a status that a `WaitDelay` that expires
  after the last read cannot lose: the tail's `Write` takes 800 ms and the child writes its 100 lines 2 ms apart, so the copy is
  behind when the child exits and the lines are lost at `HEAD`'s `WaitDelay` and kept at the amended 1.5 s. Real.
* W2's own plants, in "The margins and the breaks", below.

`TestDefaultPdeathsig` and `TestCallersPdeathsigKept` build for Linux and FreeBSD only: their plants ran on the Linux host
(chunks `pd-head` and `pd-amend`), the three Windows-only ones on the Windows laptop (below), the rest on this Mac, unraced, one test a run (`plants.py`, 672 runs).

**The three Windows-only tests, moved** (this replaces the first report's "did not move", which rested on no Windows host being in
the brief; the coordinator's correction). Read: each test and the audit's row. Each needs `Close` nil, the agent's exit at its input's end
seen within the first step (M1): `TestBreakawayLeavesJob/Close` (`breakaway_windows_test.go:79`, whose `ok` is `err == nil`, the
shared `breakaway` helper's `Kill` subtest keeping `testGrace`), `TestDescendantInJobDespiteSlowAttach` (`suspended_windows_test.go:31`,
after a 200 ms attach delay) and `TestCallersCreationFlagsKept` (`:81`). The client-side plant fits all three and uses nothing
Unix-specific, so no Windows form of the lingering-grandchild plant was needed; their outcomes do not wait on `WaitDelay` (the
shared `ready` write is long before the exit). On the Windows laptop, a scratch copy of each tree (the planted `HEAD`, and the
planted amended tree, without the Unix-only linger plant), `go test -c -race`, each test by name, one run each, `-test.shuffle=off`,
the host keeping each run's output and exit status in its own files (chunks `win-head`, `win-amend`, below):

| Test | no plant | 350 ms late | 2.5 s late | 3.5 s late | first step waits 0 s |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `TestBreakawayLeavesJob/Close`, `HEAD` | pass | fail, `stop = agentproc: agent killed, want nil` | fail, the same | fail, `stop took longer than 3.3s` | fail, `agent killed: exit status 1` |
| `TestBreakawayLeavesJob/Close`, amended | pass | pass | pass | fail, `stop = agentproc: agent killed, want nil` | fail, `agent killed: exit status 1` |
| `TestDescendantInJobDespiteSlowAttach`, `HEAD` | pass | fail, `Close = agentproc: agent killed, want nil` | fail, the same | fail, `Close took longer than 3.3s` | fail, `agent killed: exit status 1` |
| `TestDescendantInJobDespiteSlowAttach`, amended | pass | pass | pass | fail, `Close = agentproc: agent killed, want nil` | fail, `agent killed: exit status 1` |
| `TestCallersCreationFlagsKept`, `HEAD` | pass | fail, `Close = agentproc: agent killed, want nil` | fail, the same | fail, the same | fail, `agent killed: exit status 1` |
| `TestCallersCreationFlagsKept`, amended | pass | pass | pass | fail, `Close = agentproc: agent killed, want nil` | fail, `agent killed: exit status 1` |

Each fails at `HEAD` under the 350 ms plant and passes amended (and at 2.5 s); a past-the-grace plant (3.5 s) and the break fail the
amended test. All three moved to `exitGrace`. `go vet ./agentproc/` on the Windows laptop, exit 0; `GOOS=windows go vet` here, exit 0.
Not verified: that a loaded Windows runner meets the new grace; the Windows laptop ran each test once, idle.

#### The margins and the breaks

* **`TestDescendantsEnded`'s subtest** (the exit at the input's end, margin of `exitGrace`, 3 s): the exit seen 2.5 s late passes
  (3.9 s), 3.5 s late fails (`ties_test.go:63: Close = agentproc: agent terminated, want nil`); 350 ms late passes; no plant passes
  (all four, this Mac, `margin.json`).
* **The break "the first step waits 0 s"** (`BREAK_NO_FIRST_WAIT`), amended, fails every class A test whose outcome waits the first
  step: `TestCloseLadder/exits_at_its_input's_end`, `TestDescendantsEnded/the_agent_exits_at_its_input's_end`,
  `TestCloseContextBoundsHandlers/in_process,_CloseContext` and `a_child,_CloseContext`, `TestCloseContextJoinsTheAgentsError`,
  `TestCloseContextWaitsForHandlers`, `TestInProcessReturnsFirst/connection_first`, `TestLineTapAsOutputTap`,
  `TestDefaultPdeathsig`, `TestCallersPdeathsigKept` (Linux), `TestCallersCommandKept`,
  `TestConnectionEndStopsAgent/a_notification_overflow`, `TestTranscriptOfSession`, `TestExitedFirst/the_connection_ends_first`,
  `TestProcessWaitContext/ctx_ends_first` and `/one_goroutine`, `TestBlockedTapDetached`, `TestTapDrainBoundedByGrace` and the three Windows-only tests (on the Windows laptop, above); the
  class B tests with a floor fail it too (`TestCloseLadder/exits_on_SIGTERM`, `TestInProcessTerminate`). The tests whose outcome
  does not wait the first step (the twelve, the kills, `TestInProcessKill`, the step-2 subtests' text, `TestCloseContextCause`, W1's
  others) pass it, as they should: the break is not their defect. Their defect is caught by the past-the-grace plants: each
  fitting plant at 3.5 s late (`PLANT_SEEN_LATE`), or 2 s of lingering, or 2 s of slow copy, fails the amended test in every row of
  the table but W2's three, whose past-the-bound plants are the margins below (`matrix1.json`, `drip.json`).
* **The five bounds**, amended files, on the Linux host under `-race`, idle, `-test.shuffle=off` (chunk `w2-amend`; each case three
  runs, each break ten, `TestCloseContextBoundsHandlers`' break two):

  | Bound | 0.8 passes | 1.2 fails | The break fails it |
  | :--- | :--- | :--- | :--- |
  | `TestNoLostOutput`, 35 s | 28 s: 3 of 3 | 42 s: 3 of 3, `Wait took longer than 35s` | the connection closed at the agent's exit: 10 of 10, "the client received N of the 1000 session updates the agent wrote" (N 435 to 475) |
  | `TestTapErrorIgnored`, 39 s | 31.2 s: 3 of 3 | 46.8 s: 3 of 3, `Wait took longer than 39s` | a tap's `Write` error returned to the reader: 10 of 10, `Wait = agentproc: agent terminated: signal: terminated, want nil` |
  | `TestOutputTapVerbatim`, 28 s | 22.4 s after the drain's start: 3 of 3 | 33.6 s: 3 of 3, "the output tap received 2097152 bytes, want the agent's 2097208, equal" | a tap that skips a queued write: 10 of 10, "received 2031672 bytes" |
  | `TestCloseReadsTheAgentsLastOutput`, 42 s | 33.6 s after the go: 3 of 3 | 50.4 s: 3 of 3, "agent terminated: signal: terminated, want nil: the agent exits of itself" | `stop` closing the connection at once: 10 of 10, "Close = signal: broken pipe, want nil" |
  | `closecontext_test.go:156`, 35 s | 28 s after the release: 3 of 3 | 42 s: 3 of 3, `Wait took longer than 35s` | `Wait` hanging after its handlers returned: 2 of 2, "Wait took longer than 35s" (every subtest, both runs) |

  The same plants at `HEAD`'s own bounds (chunk `w2-head`, two runs each, a plant ending at 5 s, 5 s, 4 s, 6 s and 4 s) fail all five
  at `HEAD` (each with its own test's message, `w2-head`'s output) and pass amended (chunk `w2-amend`, cases `*-same`, two runs each). The no-plant runs pass
  (3 of 3 each). The break's message at `TestTapErrorIgnored` is the agent's 3 s grace's, as in phase 1b (the defect ends the
  connection; a bound does not touch it). The plant that breaks the taps fires only once `Wait`'s bound has begun, so the break's
  failure is at the bound's assertion and not at `Initialize`.

#### The A/B under the in-process load

`HEAD` (plus `zz_load_test.go` only) and the amended tree (plus the same file), the whole package, `go test -c -race`, run on the
Linux host as `GOMAXPROCS=2 BURN=32 agentproc.test -test.shuffle=<seed> -test.v`, the load started by the binary's `init`, never in a
child, so it covers every test; 10 rounds each, seeds 1795000001 to 1795000010, the same seed for both. Each chunk two rounds, through
`run-on-host --need linux --race`, one at a time on the host, never a WSL distro; the host kept each round's output, seed and exit
status in its own files. Every round of both exited 1 (F7's tests and the one below, as in phase 1's rounds). Failing tests by
round count, `HEAD` and amended:

| Test | `HEAD`, of 10 | Amended, of 10 |
| :--- | ---: | ---: |
| `TestNoLostOutput` | 10 | 0 |
| `TestTapErrorIgnored` | 10 | 0 |
| `TestCloseReadsTheAgentsLastOutput` | 10 | 0 |
| `TestOutputTapVerbatim` | 4 | 0 |
| `TestCloseLadder/exits_at_its_input's_end` | 10 | 0 |
| `TestCloseLadder/exits_on_SIGTERM` | 10 | 0 |
| `TestExitedFirst/the_agent_exits_first` | 10 | 0 |
| `TestExitedFirst/the_connection_ends_first` | 10 | 0 |
| `TestCloseContextBoundsHandlers/a_child,_CloseContext` | 10 | 0 |
| `TestCloseContextBoundsHandlers/a_child,_KillContext` | 9 | 0 |
| `TestProcessWaitContext/one_goroutine` | 8 | 0 |
| `TestCallersCommandKept` | 7 | 0 |
| `TestCallersPdeathsigKept` | 7 | 0 |
| `TestLineTapAsOutputTap` | 7 | 0 |
| `TestDefaultPdeathsig` | 6 | 0 |
| `TestProcessWaitContext/ctx_ends_first` | 6 | 0 |
| `TestDescendantsEnded/the_agent_exits_at_its_input's_end` | 4 | 0 |
| `TestDescendantsEnded/the_agent_exits_of_itself` | 3 | 0 |
| `TestInProcessKill` | 2 | 0 |
| `TestProcessWaitContext/the_agent_ends_first` | 2 | 0 |
| `TestConnectionEndStopsAgent/a_notification_overflow`, `TestEscapedDescendantDoesNotHoldWait`, `TestTapKeepsCloseEnding`, `TestWaitErrorSaysTheCauseOnce/step_2,_Serve_returning_its_cause` | 1 each | 0 |
| `TestBlockedTapDetached` (readiness only, counted apart) | 10 | 10 |
| `TestDetachedTapIsPrefix`, `TestInProcessDetachedTapIsPrefix` (F7, counted apart) | 10 each | 10 each |
| `TestTapDrainBoundedByGrace` (readiness only, counted apart) | 10 | 10 |

`HEAD` failed every test phase 1 and phase 1b saw fail. **Judged by the owner's ruling "Count it apart with the other readiness
failure" (above): amended, 0 of 10 for every moved test's own outcome.** `TestTapDrainBoundedByGrace` failed in 10 of 10 amended
rounds, and each of the ten is the readiness message and nothing else: in each of rounds 1 to 10 (seeds 1795000001 to 1795000010) the
host's output holds exactly one `--- FAIL: TestTapDrainBoundedByGrace` and, for it, exactly one line, `tap_test.go:441: the bulk
agent did not say it was ready within 30s` (the test taking 31 to 38 s), and over all ten amended rounds the only failure messages in
the output are four: that one, `tap_test.go:317` and `tap_test.go:358` (the same message, `TestBlockedTapDetached` and
`TestDetachedTapIsPrefix`) and `inprocess_test.go:468` (`TestInProcessDetachedTapIsPrefix`), ten each. `TestBlockedTapDetached`'s ten
are the readiness message too. Counted apart: `TestTapDrainBoundedByGrace`'s and `TestBlockedTapDetached`'s readiness, F7's two. No
other test failed in any amended round, and no round shows another message for either test; the same ten at `HEAD`.

#### The gate

`make pre-add-check FILES="<the changed test files>"` on this Mac through `run-heavy --`, after the last change to any test file.
The first run, over the twelve files, started 2026-11-16T23:02:48Z and ended 23:04:43Z (116 s), exit 0; the Windows-only files
changed afterwards, so it was re-run over all fourteen: started 2026-11-17T00:00:36Z, ended 00:02:24Z (108 s), exit status 0
(`gate2.status`, read), output "14 file(s) gofmt-clean; both modules linted, vetted and tested (-race -shuffle=on, uncached),
tidy, govulncheck". The fourteen files' modification times are at latest 2026-11-16T23:56:49Z (the Windows files; the other twelve
21:32:47Z), before the second gate's start, by 3 minutes 47 seconds. `GOOS=windows go vet ./agentproc/` and `GOOS=freebsd go vet
./agentproc/`, exit 0. Not run: phase 4's confirmation, `tools/gates.py`.

#### The chunks

| Chunk | What | Seconds (the queue's wait included) | Result |
| :--- | :--- | ---: | :--- |
| `matrix1`, `drip`, `margin` | the plants, this Mac, 672 runs | about 1500 | as the table |
| `w2-amend`, `w2-head` | the margins and breaks, the Linux host, `-race`, idle | 1058, 621 | as above |
| `pd-head`, `pd-amend` | the two Linux-only tests under the plants | 1472, 1374 | each plant as the table |
| `ab-head-01` to `-05` | `HEAD` under the load, rounds 1 to 10 | 618, 640, 1075, 1461, 1365 | as above |
| `ab-amend-01` to `-05` | amended under the load, rounds 1 to 10 | 1254, 1350, 1485, 1027, 726 | as above |
| `win-head`, `win-amend`, `win-vet` | the three Windows-only tests under the plant, the Windows laptop, `-race`; `go vet` | 52, 59, 8 | each as the table above, exit 0 |
| the gate, twice | `make pre-add-check`, this Mac, twelve files then fourteen | 116, 108 | exit 0, exit 0 |

One chunk was cancelled before it ran (`w2-01`, replaced by `w2-amend` and `w2-head` once the HEAD-side plants were built).

#### Deviations

1. **`TestTapDrainBoundedByGrace`'s readiness wait** failed in 10 of 10 amended rounds, as at `HEAD`. Resolved by the owner's ruling,
   relayed by the coordinator, "Count it apart with the other readiness failure" (the entry above): its grace move stays, its 30 s
   readiness failure is counted apart with `TestBlockedTapDetached`'s and is phase 5's report's. Evidence in "The A/B", restated.
2. **The three Windows-only tests** were first left unmoved for want of a Windows host. Resolved: they were planted on the Windows
   laptop and moved ("The three Windows-only tests, moved").

#### Docs cross-check (phase 3 owns the fixes)

`docs/architecture.md:1253` names tests "under a grace of `slack`" and, at `:1240`-`:1256`, lists the `agentproc` tests; it does not
say which run under `exitGrace` or name the five bounds: open, phase 3. `docs/decisions/0027-PLAN-agent-process-helper.md:206`,
"The grace in tests is 300 ms": open, phase 3, its annotation. `CHANGELOG.md`'s `Unreleased` has no line: open, phase 3. The test
files' own comments that stated a grace (`tappedFrames`', `startInProcess`'s "under testGrace unless its Grace is set", which is still
true) were fixed in this change. Searched `docs/architecture.md`, `README.md`, `AGENTS.md`, `tools/README.md`, `CHANGELOG.md`,
`docs/guides`, `agentproc/*.go` and every `*.md` and `*.py` of the tree for `testGrace`, `slack`, `300 ms` and `3 s`;
`tools/consumer.py:33`'s "300 ms bound" is the consumer program's own and not these tests'.

#### Not verified

* The plants are partly sleeps: the client-side plant delays what `Close` sees by a sleep and is not a starved process; the W2
  plants delay one wait each. Whether a loaded runner's real exits reach the new graces' edges was shown only by the load runs.
* The plants ran on this Mac unraced and on the Linux host raced, one test a run, `-test.shuffle=off`; idle, not under load.
* The 10-round A/B is a sample; the figures rest on phase 1b's measurement from one 4-CPU Linux host.
* FreeBSD: vet only. Windows: the three moved tests ran once each on the laptop, idle, under `-race`; the rest of the package was
  not run there in this phase.
* `TestInputTapSeesWrites`' only plant is the late exit (350 ms and 3.5 s); a plant on its input tap's drain was not found,
  since a late drain does not fail it.
* The breaks were chosen by phase 1b to fit W2's descriptions and were not proved to be the defects.

#### Files, checks and staging

Changed: `agentproc/agentproc_test.go`, `agentproc_unix_test.go`, `breakaway_windows_test.go`, `suspended_windows_test.go`, `closecontext_test.go`, `inprocess_test.go`, `kill_test.go`,
`linetap_test.go`, `pdeathsig_test.go`, `stderr_test.go`, `tap_test.go`, `ties_test.go`, `transcript_test.go`,
`waitcontext_test.go`; this PLAN, the MADR (the ruling) and `docs/README.md`. Not changed: any source file, workflow, script or
tool. Scratch only, not staged: the plant and tree builders, `plants.py`, the chunk scripts, the tables and every chunk's output.
Staged, not committed, not pushed.

### 2026-11-16: phase 3 (the docs)

Run by a lightweight-tier model at medium effort, as the phase prices it, on the coordinator's brief. `date +%F` printed
2026-11-16. Base commit `f102c5a`, `main`, clean at the start.

* **Approval.** The owner's ruling, relayed by the coordinator: "Run phases 1 to 4 through; then the report" (the approval entry
  above, 2026-11-16). Phase 2's commit is `f102c5a`.
* **Read:** this PLAN (Scope, the rule and its additions, phase 1b's addendum with the five figures, phase 2's record); the MADR's
  F1 and W1 to W5; `git show f102c5a` for `agentproc/agentproc_test.go`, `tap_test.go` and `closecontext_test.go`, from which the
  names and figures below are copied; `docs/architecture.md` at `:1240`-`:1264`; `0027-PLAN-agent-process-helper.md` at
  `:200`-`:212`; `CHANGELOG.md`'s `Unreleased`; `README.md`'s `Close` paragraph; `agentproc/agentproc.go`'s `Wait`, `Close`'s `stop`,
  `signal` and `agentError`. `AGENTS.md` and `tools/README.md` were searched in step 5, not read through.
* **Figures, as the test files state them** (`git show f102c5a`): `exitGrace = slack` (3 s, `agentproc_test.go:38`);
  `burstWait = 35 * time.Second` (`agentproc_test.go:45`); `releasedWait = 35 * time.Second` (`closecontext_test.go:175`);
  `tappedFramesGrace = 28 * time.Second` (`tap_test.go:304`); `tapErrorWait = 39 * time.Second` (`tap_test.go:333`);
  `lastOutputGrace = 42 * time.Second` (`tap_test.go:339`). Each matches phase 1b's addendum, "The five as measured".

#### Steps

1. **`docs/architecture.md:1253`-`:1257`.** Before: "`TestInProcessDetachedTapIsPrefix`, `TestInProcessServeAgentStep2`,
   under a grace of `slack`, and `TestWaitErrorSaysTheCauseOnce`, which". After: "`TestInProcessDetachedTapIsPrefix`, under 5 s,
   and `TestInProcessServeAgentStep2`, under `slack`. The tests that need an agent's exit, or a `Serve`'s return, seen within the
   first wait a grace bounds run under `exitGrace`, `slack` (3 s), and the five bounds that are not a grace are `burstWait`
   (35 s), `tapErrorWait` (39 s), `tappedFramesGrace` (28 s), `lastOutputGrace` (42 s) and `releasedWait` (35 s), as
   0057-MADR-agentproc-test-graces-on-a-loaded-runner.md records; the rest keep `testGrace` (300 ms) or the grace their comment
   states, and `TestWaitErrorSaysTheCauseOnce`, which". Also a correction of an earlier error: the old clause put
   `TestInProcessDetachedTapIsPrefix` under `slack`, but it runs under 5 s (`inprocess_test.go:450`, `Grace: 5 * time.Second`).
2. **`0027-PLAN-agent-process-helper.md:206`-`:209`.** The bullet "The grace in tests is 300 ms" is kept with its text and its
   2026-10-30 amendment; appended: "(Amended 2026-11-16, by 0057: the tests that need an exit seen within a grace run under
   `slack`, `exitGrace`, and five bounds that are not a grace are rescaled to 28 s to 42 s; see
   [0057-PLAN-agentproc-test-graces-on-a-loaded-runner.md](0057-PLAN-agentproc-test-graces-on-a-loaded-runner.md).)"
3. **`CHANGELOG.md:5`-`:7`,** the first bullet of `Unreleased`: "Made `agentproc`'s tests that need an agent's exit seen within a
   grace run under a grace of 3 s (`exitGrace`), and rescaled five bounds that are not a grace (`burstWait` 35 s, `tapErrorWait`
   39 s, `tappedFramesGrace` 28 s, `lastOutputGrace` 42 s, `releasedWait` 35 s), so a loaded runner under `-race` no longer fails
   them; tests only." The phrase "tests only" is the PLAN's example's; the same commit also changes a doc comment (step 4) and a
   README sentence, neither of them behaviour.
4. **Question 4: `agentproc/agentproc.go:29`-`:35`,** `ErrTerminated`'s comment. Before: "ErrTerminated is joined to Wait's error when
   the agent exited after Close sent SIGTERM to its process group, or, for an InProcess agent, after Close cancelled Serve's
   context with it." After: "ErrTerminated is joined to Wait's error when Close sent SIGTERM to the agent's process group, or,
   for an InProcess agent, cancelled Serve's context with it, before the agent's exit was seen. So an agent that exits at its
   input's end just as the grace ends can be reported with ErrTerminated alone, as terminated with a status of 0: its exit error
   is nil, and Wait returns ErrTerminated by itself."
   The code the sentence rests on: `agentproc.go:659`, `case !p.hurried() && p.signal(p.agent.terminate, ErrTerminated) &&
   p.exitedWithin(p.grace):`, which runs when step 1's `exitedWithin(p.grace)` has returned false (`:657`-`:658`); `:698`-`:709`,
   `signal` sends unless `p.exited` is set (set at `:365`, when the wait returns), and then records `p.ended = why`; `:427`-`:442`,
   `agentError`, with `ended` set and `waitErr` nil, returns `ended` alone. So the error is `ErrTerminated` with no exit error, and
   the agent's status of 0 is not reported, whenever the send happens before the watcher records the exit. The MADR's F1 probe
   measured status 0 with `ErrTerminated` in 15 of 20 runs, under a break that makes the first step wait 0 s.
   **`README.md:970`-`:972`,** the `Close` paragraph, after "when `Close` had to escalate.": "An agent that exits at its input's end
   just as the grace ends can be reported with `agentproc.ErrTerminated` alone, terminated with a status of 0, because the SIGTERM
   was sent before its exit was seen." No code, identifier or other comment changed.
   `make api` against `v1.36.0`, the latest tag (`git describe --tags --abbrev=0`): "check_api: against v1.36.0: 0 incompatible,
   0 compatible"; and "check_api: reported packages unstable, remote: 0 incompatible, not gated"; exit 0.
5. **Hit list.** `grep -rn` over the tree for `testGrace`, `slack`, `300 ms`, `300ms`, `grace of`, `exitGrace`, the five bounds'
   names and the moved tests' names, in `*.md`, `*.go`, `*.py`, `*.yml`, `*.yaml`, `*.sh` and `Makefile`: 1,278 lines. The
   non-record hits were read in full; the record hits (about 1,100 lines, under `docs/decisions/` and `docs/reports/`) were read
   by file and by their grace lines, not line by line. Verdicts:
   * `docs/architecture.md:1253`, "under a grace of `slack`": false, fixed (step 1).
   * `docs/architecture.md:1245`-`:1263`, the test names: true, left. `docs/architecture.md:1424`, "the 300 ms a stream must last
     under a 100 ms `WriteTimeout`": true, left; it is `remote`'s `TestHTTPServerWriteTimeout`, not these tests.
   * `docs/decisions/0027-PLAN-agent-process-helper.md:206`, "The grace in tests is 300 ms": true on 2026-10-30, false now;
     annotated by dated note (step 2). The other 0027 hits are the PLAN's record of its own past states: left.
   * `tools/consumer.py:33`, "closes another under a 300 ms bound": true, left. It is the consumer program's own bound, not these
     tests', as the brief says.
   * `tools/test_tools.py:291`-`:292`, `TestKill$|TestKillAfterExit$`: a run filter, not a grace: true, left.
   * `README.md`, `CHANGELOG.md`, `AGENTS.md`, `tools/README.md`, `docs/guides`: no hit other than the two edited above.
   * `docs/README.md:124`-`:125` (0042's rows) and `:175`-`:176` (0057's rows): records; the 0057 rows' text is unchanged and still
     true (five phases; in-progress).
   * `docs/decisions/*` and `docs/reports/*`, the other records (0026, 0029, 0030, 0034, 0040 to 0044, 0049, 0052, 0055, and the
     release notes 0030, 0032, 0041 to 0043, 0049): each a record of the state of its own time, for example
     `0030-PLAN-record-initialize-on-arrival.md:1014`, "under `testGrace`, 300 ms", for `TestOutputTapVerbatim` as it then was.
     Left: committed record text changes only by a dated addition, and only a current statement the change makes false needs one,
     which is `0027`'s line 206.
   * `docs/decisions/0057-*`: this record pair's own text: left.
   * `agentproc/*_test.go` (not docs, and not in this phase's file list): the comment lines that name a grace were read in the
     grep output, about sixty of them. No test file was edited. The lines I checked are still true: `start` at
     `agentproc_test.go:95` keeps `testGrace`; `inprocess_test.go:385`, "Its grace is slack, not testGrace", for
     `TestInProcessServeAgentStep2`, which keeps `slack`. I did not read each test file in full.
   * `remote/*_test.go` (four lines): package `remote`'s own test names and graces, outside question 1's scope: left.

#### Verification

Each after the last edit, in UTC.

* `make api`, through `run-heavy`: 2026-11-17T00:15:15Z to 00:15:25Z, exit 0; its result
  lines are in step 4.
* `make pre-add-check FILES="agentproc/agentproc.go"`, through `run-heavy`: 2026-11-17T00:15:32Z
  to 00:17:25Z, exit 0; "go-precheck: 1 file(s) gofmt-clean; both modules linted, vetted and tested (-race -shuffle=on, uncached),
  tidy, govulncheck."
* `make check-records`, exit 0, 2026-11-17T00:18:43Z to 00:18:44Z ("python3 scripts/check_records.py --check-all", no finding).
  `make markdownlint`, exit 0, 00:18:44Z to 00:18:46Z ("57 files", "0 issues in 0 files"; its glob leaves out the MADR and PLAN
  files). `make disclosure-check`, exit 0, 00:18:46Z to 00:18:47Z ("842 tracked file(s), 0 finding(s)"). Run with the files staged.

#### Not verified

* Step 4's sentence rests on the code as read and the MADR's F1 probe; the case was not re-measured in this phase.
* The CHANGELOG line says "tests only"; the same commit changes a comment and a README sentence, which are not behaviour.
* The test files' comments were read in grep output only, and not edited.

#### Files and staging

Changed: `docs/architecture.md`, `docs/decisions/0027-PLAN-agent-process-helper.md` (one annotation), `CHANGELOG.md`, `README.md`,
`agentproc/agentproc.go` (one comment), and this PLAN (this entry). Not changed: `docs/README.md` (the rows' text is unchanged);
any test file, code, identifier, workflow or script. Staged, not committed, not pushed.

### 2026-11-16: phase 4 (confirmation and CI)

Run by a mainstream-tier model at medium effort, as the phase prices it, on the coordinator's brief. `date +%F` printed 2026-11-16;
the times below are UTC, so the runs fall on 2026-11-17. Base commit `8b0b051`, `main`, level with `origin/main`, tree clean at the
start. The phase changed no source, test, workflow or tool; its only tree change is this entry.

* **Approval.** The owner's ruling, relayed by the coordinator: "Run phases 1 to 4 through; then the report" (the approval entry
  above). The rulings the acceptance rests on, as labels: "A measured report, then decide" and "Count it apart with the other
  readiness failure", each the owner's ruling, relayed by the coordinator. Phase 2 is commit `f102c5a`, phase 3 `8b0b051`.
* **Read:** this PLAN ("Where the race gates run" with its additions, phase 4 with its acceptance and additions, phase 5, phase 2's
  list of the 45 moved tests), `AGENTS.md`, `tools/README.md`, `tools/gates.py`, `tools/remote.py`'s `tree` command, the first lines
  of `tools/watch_ci.py`, the `Makefile`'s race targets, `.github/workflows/ci.yml` and the skill `available-dev-hosts`.
* **What load each run had.** Every Linux run was on the Linux host, alone in its queue, with no in-process starved load (that was
  phase 2's A/B): the package rounds ran idle, the host's own load average before and after each chunk 1.4 to 3.4 on its four CPUs
  (other sessions' work, read from `uptime` in each chunk's output); the module rounds ran as CI runs it, the packages in
  parallel beside each other. The Mac's rounds ran one at a time through `run-heavy`.

#### The commands

* **Package chunks.** `go test -race -shuffle=on -count=5 -v -timeout=30m ./agentproc/`, four of them, 20 rounds, each through
  `run-on-host --need linux --race -- sh pkg.sh <name> 5` (a scratch copy of the tree at `8b0b051`; never a WSL distro). `-v` is
  added so that passes are in the output. The timeout: a round measured about 105 to 109 s beside the starved load in phase 1, and a
  hung wait now takes up to 45 s, so five rounds are about 9 minutes at that rate; 30 minutes is more than three times that. The
  chunks' own time was 235 to 253 s (about 49 s a round, idle). With `-count=5` and `-shuffle=on` the binary prints one seed, which
  seeds the order of every round of the chunk.
* **Module rounds.** CI's step, quoted from `.github/workflows/ci.yml`: `go test -race -shuffle=on ./...` and
  `( cd cmd/generate && go test -race -shuffle=on ./... )`, each with `-v` added; three rounds through `run-on-host --need linux
  --race -- sh mod.sh <name>`. Each package of a `./...` run prints its own seed; they are in the host's files.
* **This Mac.** `go test -race -shuffle=on ./...` exactly, three rounds, each through `run-heavy -- sh mac.sh`.
* **Gates.** `tools/gates.py --all` was run in two parts, so that no run of mine shared a host with another: the sixteen gates that
  are not the hosts' (`tools/gates.py --out <scratch>`) through `run-heavy` here, then `tools/gates.py linux windows` through
  `run-heavy` here with `ACP_LINUX_HOST` and `ACP_WINDOWS_HOST` set to the two hosts' SSH aliases. `tools/remote.py tree`, which
  those two gates run, copies the tree and runs `go test -count=1 -shuffle=on ./...` over `ssh` and does not queue on the target
  host: no other run of mine was on either host then, and another session's was not checked beyond `run-heavy --status` here.
  *Added 2026-11-16, at the coordinator's request:* the second run started 2026-11-17T01:02:04Z and ended 01:05:24Z (200 s); the
  `linux` gate, run first, wrote its log at 01:03:49Z (about 105 s after the start) and the `windows` gate its log at 01:05:23Z
  (about 94 s after that), so a reader can set them beside another session's runs on those hosts. The first run, of the sixteen
  gates, started 00:30:45Z and ended 00:36:50Z.

#### The chunks

| Chunk | Host | Rounds | Seed | Start | Seconds | Exit | Failed tests |
| :--- | :--- | ---: | :--- | :--- | ---: | ---: | :--- |
| `pkg-1` | the Linux host, `-race` | 5 | 1794875375736694669 | 00:29:20Z | 263 | 0 | none |
| `pkg-2` | the Linux host, `-race` | 5 | 1794875636073867985 | 00:33:43Z | 249 | 0 | none |
| `pkg-3` | the Linux host, `-race` | 5 | 1794875886196072637 | 00:37:52Z | 267 | 0 | none |
| `pkg-4` | the Linux host, `-race` | 5 | 1794876152677963260 | 00:42:19Z | 254 | 0 | none |
| `mod-1` | the Linux host, `-race` | 1 | per package, in the host's file | 00:46:33Z | 304 (root 111, generate 188) | 0 and 0 | none |
| `mod-2` | the Linux host, `-race` | 1 | per package | 00:51:37Z | 303 (root 110, generate 188) | 0 and 0 | none |
| `mod-3` | the Linux host, `-race` | 1 | per package | 00:56:39Z | 317 (root 114, generate 198) | 0 and 0 | none |
| `mac-mod-1` | this Mac, `-race` | 1 | `-shuffle=on` | 00:29:34Z | 24 | 0 | none |
| `mac-mod-2` | this Mac, `-race` | 1 | `-shuffle=on` | 00:29:58Z | 23 | 0 | none |
| `mac-mod-3` | this Mac, `-race` | 1 | `-shuffle=on` | 00:30:22Z | 23 | 0 | none |
| `gates-local` | this Mac | 16 gates | - | 00:30:45Z | 365 | 0 | - |
| `gates-hosts` | the two hosts, from here | 2 gates | - | 01:02:04Z | 200 | 0 | - |
| `win-probe` | the Windows laptop, `-race` | 1, the three tests | 1794877697389566800 | 01:08:08Z | 15 (the run 3.6 s) | 0 | none |
| `win-3tests` | the Windows laptop, `-race` | 20, the three tests | 1794877716860926500 | 01:08:27Z | 56 (the run 45.0 s) | 0 | none |
| `win-pkg` | the Windows laptop, `-race` | 3, the whole package | 1794877772160841800 | 01:09:23Z | 137 (the run 127.0 s) | 1 | `TestBlockedTapDetached`, `TestDetachedTapIsPrefix`, `TestInProcessDetachedTapIsPrefix`, each in 3 of 3 rounds |

No chunk was re-run, cancelled or lost; every chunk run is in the table. The module chunks' agentproc package took 54.6, 51.6 and
54.9 s there, and the package chunks' rounds about 49 s each, so the whole package under the module's own load was no slower than
alone, within a few seconds.

#### The judgment, as "Where the race gates run" says

* **The tests this PLAN moved.** 45 in the table of phase 2; 42 build on Linux. Each of the 42, by name from the host's `-v`
  output, passed in each of the 20 package rounds and each of the 3 module rounds: 23 passes, 0 failures, 0 skips, each. The three
  Windows-only (`TestBreakawayLeavesJob/Close`, `TestDescendantInJobDespiteSlowAttach`, `TestCallersCreationFlagsKept`) do not
  build on Linux, so they are not in these rounds; they ran on the Windows laptop in phase 2, once each, and in the `windows` gate
  here (`go test -count=1 -shuffle=on ./...`, without `-race`), which passed once, and in CI's Windows jobs for `f102c5a` and
  `8b0b051`.
* **Counted apart.** `TestDetachedTapIsPrefix` and `TestInProcessDetachedTapIsPrefix`: 23 passes, 0 failures each.
  `TestBlockedTapDetached` and `TestTapDrainBoundedByGrace`: 23 passes, 0 readiness failures ("the bulk agent did not say it was
  ready within 30s") and 0 other failures each. None of the four failed in any of the 23 rounds, so there is nothing of theirs to
  count or to check against the readiness message. This is not evidence for or against phase 5's question: the host was idle, not
  under phase 2's starved load, where the readiness failures appeared.
* **Any other test.** None failed in any round; no `DATA RACE` and no `panic` in any chunk's output. Every package of every module
  round was `ok` (the judge's table: `judge-all.txt`).

#### The Windows-only moved tests, run on the Windows laptop

*Added 2026-11-16, at the coordinator's request, after the entry above was written, run by the same model.* The coordinator's
application of the acceptance's intent, recorded as the coordinator's and not as the owner's: a moved test is confirmed where it
builds. The three Windows-only moved tests became moved tests after the acceptance was written (phase 2), so it could not name a
Windows confirmation. Run at `8b0b051` (the same scratch copy of the tree), through `run-on-host --need windows --race` (never a
WSL distro), one heavy run at a time on that host, the host keeping its own output and exit status in files, `go1.27.4
windows/amd64`, `CGO_ENABLED=1`:

1. The three tests: `go test -race -shuffle=on -count=20 -v -timeout=10m -run 'TestBreakawayLeavesJob|TestDescendantInJobDespiteSlowAttach|TestCallersCreationFlagsKept' ./agentproc/`
   (`win-3tests`). The timeout was set from one measured round, `win-probe`, the same command with `-count=1`: the run took 3.6 s
   (15 s with the build and the dispatch), so 20 rounds are about 72 s; 10 minutes is more than eight times that. Exit 0 in 56 s;
   `ok` in 45.0 s.
2. The whole package: `go test -race -shuffle=on -count=3 -v -timeout=30m ./agentproc/` (`win-pkg`), so that the three are seen
   beside the rest of the package, as CI's Windows jobs run it (no `-v` there). The timeout, 30 minutes, is over thirteen times the
   run's 127 s.

Pass counts, from the host's `-v` output:

| Test | `win-3tests`, 20 rounds | `win-pkg`, 3 rounds |
| :--- | :--- | :--- |
| `TestBreakawayLeavesJob` | 20 passes, 0 failures | 3 passes, 0 failures |
| `TestBreakawayLeavesJob/Close` | 20 passes, 0 failures | 3 passes, 0 failures |
| `TestBreakawayLeavesJob/Kill` | 20 passes, 0 failures | 3 passes, 0 failures |
| `TestDescendantInJobDespiteSlowAttach` | 20 passes, 0 failures | 3 passes, 0 failures |
| `TestCallersCreationFlagsKept` | 20 passes, 0 failures | 3 passes, 0 failures |

The package rounds: exit status 1, the package `FAIL` in 127.0 s, and exactly nine failed top-level tests, three in each round,
each of them a test counted apart; the failure lines are quoted from `win-pkg.txt`, the same three messages in each of the three
rounds:

* `TestBlockedTapDetached`: `tap_test.go:317: the bulk agent did not say it was ready within 30s`, the readiness message and
  nothing else, 3 of 3 rounds, so counted apart as the plan says (34.8 s, 34.7 s and 33.9 s).
* `TestDetachedTapIsPrefix` (a 56 MiB test): `tap_test.go:358: the bulk agent did not say it was ready within 30s`, 3 of 3.
* `TestInProcessDetachedTapIsPrefix` (the other 56 MiB test): `inprocess_test.go:468: the connection did not read Serve's first 56
  MiB within 30s`, 3 of 3.
* `TestTapDrainBoundedByGrace` passed in 3 of 3 rounds, so there is no count of its readiness failure here. No other test failed,
  and the output has no `DATA RACE` and no `panic`.

So no moved test failed and no test failed outside those counted apart; the phase does not stop. The three counted-apart tests
failing here and not on the Linux host is a measurement for phase 5's report, not a finding of this phase; not investigated here.

#### The Mac's module rounds

Three of three passed: exit 0 each, 24, 23 and 23 s; every package `ok`, none cached (the output has no `(cached)`).

#### `tools/gates.py`

All 18 gates exit 0: `regen`, `precheck`, `newfiles` ("no new .go files"), `nojsonv2`, `winvet`, `records`, `mdlint`, `idscan`,
`tools`, `lint`, `fix`, `tidy`, `api`, `race3`, `floor`, `platforms` (the first run, 00:30:45Z to 00:36:50Z, "all gates passed"),
then `linux` and `windows` (the second run, "all gates passed"; the `linux` gate's log shows every package `ok` at go1.27.4,
`agentproc` included, so no F7 test failed there and there is no F7 deviation). These ran on the clean tree at `8b0b051`, before this entry was written.

#### CI

`tools/watch_ci.py <commit> --branch main`, each exit 0. Two runs a commit: the `CI` workflow's six `validate` jobs, and `Docs`.

| Commit | Run | Job | Job id | Conclusion |
| :--- | :--- | :--- | :--- | :--- |
| `f102c5a` | `CI`, 43985679958, <https://github.com/maccavelli/acp-go-sdk2/actions/runs/43985679958> | `validate (windows-2025, 1.26.11)` | 133713094026 | success |
| | | `validate (ubuntu-24.04, 1.26.11)` | 133713094179 | success |
| | | `validate (macos-15, 1.27.4)` | 133713094219 | success |
| | | `validate (windows-2025, 1.27.4)` | 133713094221 | success |
| | | `validate (macos-15, 1.26.11)` | 133713094346 | success |
| | | `validate (ubuntu-24.04, 1.27.4)` | 133713094404 | success |
| `f102c5a` | `Docs`, 43985679996, <https://github.com/maccavelli/acp-go-sdk2/actions/runs/43985679996> | `docs` | 133713093741 | success |
| `8b0b051` | `CI`, 43987715086, <https://github.com/maccavelli/acp-go-sdk2/actions/runs/43987715086> | `validate (windows-2025, 1.26.11)` | 133720062420 | success |
| | | `validate (macos-15, 1.27.4)` | 133720062810 | success |
| | | `validate (windows-2025, 1.27.4)` | 133720062836 | success |
| | | `validate (ubuntu-24.04, 1.26.11)` | 133720062855 | success |
| | | `validate (ubuntu-24.04, 1.27.4)` | 133720062953 | success |
| | | `validate (macos-15, 1.26.11)` | 133720062979 | success |
| `8b0b051` | `Docs`, 43987715139, <https://github.com/maccavelli/acp-go-sdk2/actions/runs/43987715139> | `docs` | 133720062116 | success |

`f102c5a`'s `CI` run (created 2026-11-17T00:12:04Z) is the first CI run with the amended tests: its parent `032320a` and the
commits before it change no test file, and `f102c5a` is the commit that changed them. `TestDescendantsEnded`, the failure this PLAN
was opened for, appears in none of the six jobs' logs, passed or failed: CI runs `go test` without `-v`, so a pass prints no test
name; what the logs show is the package's line, `ok ... agentproc` in the race step of each of the four jobs that have one (Linux
and Windows; the macOS jobs have no race step): 40.7 s, 45.9 s, 34.6 s and 43.9 s on `f102c5a`, no `--- FAIL` in any of the six.
So CI shows no failure of it, and does not show it passed by name; the Linux host's `-v` rounds do (23 of 23, each subtest).
`8b0b051`'s jobs were not read beyond their conclusions.

#### Acceptance, line by line

* **On the Linux host every moved test passes in each of the 20 package rounds and the 3 module rounds.** Met for the 42 that build
  there, 23 of 23 each. Not met as written for the 3 Windows-only tests, which cannot run there (above); their evidence is the
  laptop run of phase 2, the `windows` gate, and CI's Windows jobs, none of them 23 rounds.
  *Added 2026-11-16, at the coordinator's request, as the coordinator's application of the acceptance's intent (a moved test is
  confirmed where it builds), not the owner's ruling:* the Windows-only moved tests are confirmed on the Windows laptop: each
  passed in 20 of 20 rounds under `-race -shuffle=on`, and in 3 of 3 whole-package rounds ("The Windows-only moved tests", above).
* **No test fails there but the F7 tests, whose failures are counted and recorded.** Met: no test failed; the F7 count is 0 in 23.
  On the Windows laptop, three counted-apart tests failed in 3 of 3 package rounds, as recorded above, and nothing else.
* **The module's race gate passes three of three on this Mac.** Met: 3 of 3.
* **The gates pass.** Met: 18 of 18.
* **CI's six jobs "completed success".** Met for `f102c5a` and for `8b0b051`.

#### Deviations

None.

#### Not verified

* That a loaded runner meets the new graces: these Linux rounds were idle, or the module beside itself; phase 2's A/B is the loaded
  evidence, and the F7 readiness failures did not appear here, so nothing was learned of them.
* 23 rounds on one four-CPU host is a sample; a failure rate under 1 in 23 is not excluded.
* FreeBSD (vet only, phase 2). The 3 Windows-only moved tests were repeated on the Windows laptop (above), on one host whose load was not measured, 20 rounds
  and 3 whole-package rounds; not under a starved load, and not on a loaded Windows runner.
* The module rounds were run with `-v` added to CI's line; the Mac's were not.
* `tools/remote.py tree` is not queued on its host (above).
* The CI logs' per-test passes: CI does not print them.
* The seeds of the module rounds are per package and are in the host's files, not in this entry.

#### Files, checks and staging

Changed: this PLAN (this entry). Not changed: `docs/README.md` (the rows' text is unchanged), any source, test, workflow or tool.
Scratch only: the chunk scripts, the judge and every chunk's output (`pkg.sh`, `mod.sh`, `mac.sh`, `judge.py`, `out/`).
Checks, each after the last edit, in UTC, run with the PLAN staged: `make check-records`, exit 0; `make markdownlint`, exit 0 ("57 files", "0 issues in 0 files"); `make disclosure-check`, exit 0 ("842 tracked file(s), 0 finding(s)").
Staged, not committed, not pushed.

### 2026-11-16: phase 5 (the report)

Run by a mainstream-tier model at high effort, as the phase prices it, on the coordinator's brief. `date +%F` printed 2026-11-16, the
date of the earlier entries; the times below are UTC and fall on 2026-11-17. Base commit `b050c9f`, `main`, level with
`origin/main`, tree clean at the start. The phase changed no source, test, workflow or tool, and no bound, grace or data size; its
tree changes are the report, its row in `docs/README.md` and this entry.

* **Approval and rulings.** The owner's ruling, relayed by the coordinator, "Run phases 1 to 4 through; then the report" (the
  approval entry above); phases 1 to 4 are committed (`f0b7c2e`, `032320a`, `f102c5a`, `8b0b051`, `b050c9f`) and CI is green on
  the fix. The report answers "A measured report, then decide" and "Count it apart with the other readiness failure", each the
  owner's ruling, relayed by the coordinator. Two conditions are the coordinator's, as the coordinator's application of "A measured
  report, then decide" and not the owner's ruling: the Windows laptop (the test alone and the whole package, where phase 4 saw
  three tests fail in 3 of 3 rounds) and this Mac (the reference for the race gate of record).
* **Read:** this PLAN (phase 5 with its three additions, "Where the race gates run", the audit's record for these tests, phase 1b's
  method, phase 2's A/B and the counted-apart rulings, phase 4's record with its section on the Windows laptop); the MADR's F7 and
  W5; `agentproc/tap_test.go`, `inprocess_test.go`, `child_test.go`, `tap.go`, `agentproc.go`, `inprocess.go` and the connection's
  receive path in `connection.go`; `.github/workflows/ci.yml`; `AGENTS.md`, `tools/README.md`; the skills
  `available-dev-hosts`, `madr-and-plan-writing` and `documentation-writing`; two existing reports.
* **Ran,** every `-race` run through `run-on-host --need linux --race`, `--need windows --race` or `run-heavy` on this Mac, never a
  WSL distro, one heavy run at a time on each host, from scratch copies of `git archive HEAD` carrying stamps and bounds the
  environment lifts; nothing in the tree. 15 chunks on the Linux host (254 minutes, queue waits included), 22 chunks on the
  Windows laptop (76 minutes, queue waits included), 7 chunks on this Mac (29 minutes, queue waits included); 716 runs of the four
  tests in all (584 under `-race`). One chunk dispatched to the laptop (`W12-pkg-gmp16`) ran on this Mac when `run-on-host`'s copy
  check failed; its records are this Mac's. The CI logs of the 14 latest CI runs on `main` were read with `gh`.
* **The chunks** (also in the report, which says how each was measured). "Seconds" is the dispatcher's, a queue wait included.

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

* **Findings, in five lines.** (1) Alone and idle, each wait is 1.4 to 12.3 s on the three hosts (`-race`), none near 30 s; it is
  the connection's per-line `json.Valid` and `Unmarshal` at race speed, one goroutine's work, not the tap, the pipe, the write size
  or `GOMAXPROCS`. (2) What moves it is how many of the four read at once: the Linux host's `TestDetachedTapIsPrefix` waits 10.6 s
  alone, 25.3 s with the four bulk tests together and 25.8 s beside the module (2 of 10 rounds over 30 s); the Windows laptop's
  4.5 s alone, 13.1 s with two, 28.4 s with four and 41.3 s in the whole package (10 of 10 rounds over 30 s, 3 of 3 with the
  bounds as written); its sustained race-instrumented work also runs at a quarter of its first three seconds' speed. (3) Under the
  starved load the wait is 90 to 116 s,
  0.5 MiB/s, in 5 of 5 rounds; `Close`'s first step of the two 56 MiB tests is 12 to 18 s against its 5 s. (4) CI's race step passed
  52 of 52 jobs, `agentproc` taking 22.9 to 50.7 s there; its CPU count is not in the logs. (5) The 5 s grace's failure of the
  audit's round 1 is reproduced beside the starved load (c2: `Close` failed in 2 of 2 runs of each 56 MiB test), the first step
  waiting for the agent's last 8 MiB at the connection's rate; four tests taking one scratch lock brought the Windows package's
  waits to 6.8 to 14.0 s and the Linux module's longest to 24.0 s, which the report offers, with four other options, for the owner
  to decide.
* **Where the report is:**
  [0057-REPORT-agentproc-56-mib-tests-30s-bound-measured.md](../reports/0057-REPORT-agentproc-56-mib-tests-30s-bound-measured.md),
  with "What could follow": five options, each with what it changes, its cost and what comes first, one marked as the
  recommendation. It decides nothing. This plan's status stays `in-progress`: the plan completes when the owner has decided what
  follows.
* **Deviations.** None stops the phase. Recorded in the report's "What distorted a measurement": two package chunks that held my
  scratch experiments, one chunk that `run-on-host` ran on this Mac when its copy check failed, an exit status 2 that is my own
  edit of a running script, a serialised-variant chunk whose root package lacked fixtures, and this Mac not being idle.
* **Not verified:** CI's own per-test times and CPU count; why the Windows laptop slows with the number of tests and after
  3 s of sustained load; the bounds' margins and breaks at any figure; the serialised variant under the starved load; FreeBSD;
  the stamps' own cost. Ten runs or rounds is a sample, and the starved-load rows have 5 and 2 rounds.
* **Files:** touched, this PLAN (this entry), the report and `docs/README.md` (its row). Not touched: any source, test, workflow or
  tool. Scratch only: the stamped trees, scripts, tables and every chunk's output.
* **Checks,** each after the last edit, in UTC, with the three files staged: `make check-records`, exit 0; `make markdownlint`,
  exit 0 ("Linting: 58 files", "0 issues in 0 files"); `make disclosure-check`, exit 0 ("843 tracked file(s), 0 finding(s)"); a
  scan of the three staged files for the hosts' names, the local account name, real-machine paths, queue tickets and leases found
  none.
  Staged, not committed, not pushed.
