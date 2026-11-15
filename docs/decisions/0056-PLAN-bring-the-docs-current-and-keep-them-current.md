---
status: in-progress
date: 2026-11-15
associated-madr: "0056-MADR-bring-the-docs-current-and-keep-them-current.md"
---

# Implement 0056: bring the docs current, add a changelog, and keep them current with no new checks

Associated MADR: [0056-MADR-bring-the-docs-current-and-keep-them-current.md](0056-MADR-bring-the-docs-current-and-keep-them-current.md)

Provenance: drafted by the strongest model at high effort; the survey it starts from was made by a mainstream model at
medium effort; the rulings are the owner's, of 2026-11-14, relayed by the coordinator.

## Goal

Bring this repository's docs current under the MADR's standard (D1), and leave them in a state the docs step (D5)
can keep current with no new check (D3). At the end:

* no statement in a tracked doc or doc comment is contradicted by the code, out of the full defect list that phase 1
  produces;
* every package, command, flag, environment variable, settings key, Make target and Make variable is described in a
  README, guide or reference;
* every exported name in the scope of the MADR's Open Question 4, as the owner answered it, has a doc comment;
* `CHANGELOG.md` exists, with an `Unreleased` section, a `v1.36.0` section and one pointer to the older release-notes
  reports (Open Question 1, as the owner answered it), and `RELEASING.md` describes the changelog beside the reports;
* `docs/README.md`'s index is current, and its "I want to…" matrix has one row per reader task (Open Question 2,
  as the owner answered it).

## Scope

**Preconditions.**

* The owner approves this PLAN. The owner answered Open Questions 1 to 7 and 9 on 2026-11-14, and question 8 was
  settled the same day by the coordinator's application of an owner's ruling (the MADR's annotations). No question
  is open, and no phase waits for an answer.

  *Met 2026-11-14: the owner, relayed by the coordinator, said "Approved to proceed with the docs update plan for
  acp-go-sdk2", after reading the pair. Earlier that day the owner had answered "Not yet" to approving the three docs
  plans. The approval covers this repository's PLAN alone (Execution record).*
* This pair and their two `docs/README.md` rows are committed alone, as `AGENTS.md`'s bootstrap exception allows.
* The base is the commit holding the pair. Phase 1 records it.

**Files each phase may change** are listed in the phase. Every phase also changes this PLAN's execution record and its
`docs/README.md` row. No phase changes a file outside its list. A defect that phase 1 finds in a file no phase lists
waits: the pair is amended and approved again, as `AGENTS.md` requires.

**Ordering against a release (MADR D4).** `v1.36.0` is the newest tag. The two commits since it change records alone,
and no `.go` file has changed. Every defect this PLAN fixes is therefore an older gap, and none of its phases has to
come before the next tag. This PLAN cuts no tag: the next code release carries its fixes (Open Question 9, as the
owner answered it). When a release is prepared while this PLAN is in progress:

1. That release's PLAN fixes every statement its own change makes false and documents what it ships, before its tag.
   If phase 5a has landed by then, it turns `Unreleased` into its version. If not, it writes the release-notes report
   as `RELEASING.md` says now, and phase 5a's section is then for that newer tag, with the pointer covering the rest.
2. This PLAN's next phase starts after the tag. It first re-runs phase 1's verification for the rows in its own
   class, on the new HEAD, and records any row the release fixed or changed.
3. No phase of this PLAN is held open across a tag. A phase that is half done when a release starts is finished, or
   its working-tree edits are kept out of the release's commits.

**Timing, by the owner's ruling of 2026-11-14 (the MADR's D4 annotation).** "Phases that change only Markdown run now,
one at a time, and pause at a phase boundary if a release step starts. Doc-comment phases that edit Go files wait
where code is about to be rewritten." No tag is waiting in this repository, so nothing holds the Markdown phases back.
The order is:

1. Markdown only, one at a time: phase 1, phase 2, phase 5a, phase 3, phase 6. Before each phase starts, the agent
   checks whether a release step has started (a release PLAN `in-progress`, or a tag being prepared). If one has, the
   agent pauses at that boundary.
2. Phase 5b, the workflows and a script, after phase 5a.
3. Go files last: phase 4a, then phase 4b. Each waits while an approved PLAN is about to rewrite code in a file it
   would touch, and runs after that PLAN's commit, against the new code.

To keep phases 2 and 3 Markdown-only, a wrong or missing statement in a Go file, whether a doc comment or a package's
`doc.go`, is moved to phase 4a. A generated one goes to phase 4b.

**Heavy runs.** `README.md` is not docs-safe: `docs_test.go` reads it. Neither is any `.go` file, nor `CHANGELOG.md`
until phase 5b makes it docs-safe. For those files `make pre-add-check` runs every Go check, `go test -race`
for both modules among them. That is a heavy run. It runs on this Mac through the fleet's queue,
`run-heavy -- make pre-add-check FILES="…"` (the `available-dev-hosts` skill), one at a time. A phase that changes only
docs-safe paths (`docs/**`, `AGENTS.md`, `RELEASING.md`, `tools/README.md`, the agent-rule files) runs no Go check:
`make pre-add-check` given docs-safe files alone runs no Go check, so it is not needed.

**Commands.** Each command's output goes to a scratch file outside the tree. Its exit status is captured before any
filter, and the file is read before anything is concluded from it. The one-off audits are scripts in the session's
scratch directory. They are never committed (MADR D3). Before a phase relies on an audit, the phase runs it on a
scratch clone (`git clone --quiet <repo> <scratch>/clone`) in which one item of the audit's class has been
deliberately broken, and records the failure the audit printed. An audit that has not been seen to fail is not used.

**Commits.** Each phase ends in one commit, `git commit --no-edit`, after its checks pass. On `main` the agent commits
only with the owner's explicit permission in that turn. Otherwise it stops with the phase staged, and the owner
commits (`AGENTS.md`, "Commits"). This PLAN never pushes or tags.

*Amended 2026-11-15 by the owner's ruling, relayed by the coordinator.* Asked whether the standing approval for
commits and pushes, given 2026-11-12 for the auth-contract effort, extends to the docs plans and to the other
approved plans the coordinator runs in these repositories, the owner chose "Yes, all approved plans": "I commit and
push each owner-approved phase of any plan on the coordination ledger, in the six related repos, after the disclosure
guard exits 0. Tags, releases, force pushes and anything outside an approved plan still need your word each time. The
acp plan's 'the owner pushes' text gets a dated note recording this. This matches what I have been doing." ("I" is
the coordinator.) Rejected: "Commit yes, push no"; "Stage only". So the coordinator commits and pushes each approved
phase of this PLAN, after the disclosure guard (`make push-guard`) exits 0. The executing agent stops with the phase
staged and does not commit or push. This PLAN still cuts no tag, and "never pushes" above now reads as "pushes only
through the coordinator, phase by phase". `AGENTS.md`'s "Commits" rule, that permission given in one turn does not
carry to the next, is that repository's rule for agents in general and is not changed. The owner's ruling is an
explicit standing grant to the coordinator, recorded here, and it is what stands in for the per-turn permission that
rule asks for, for this PLAN's approved phases. A tag, a release, a force push or anything outside an approved phase
still needs the owner's word each time.

**Stop condition, every phase.** An unexpected state, or the same step failing twice, ends the task. The agent records
the state and its evidence in the execution record and returns. It does not improvise, and it does not try a third
time.

## Phases at a glance

| Phase | What | Files | Model and effort | Acceptance |
| :--- | :--- | :--- | :--- | :--- |
| 1 | verify every lead; read `README.md` and `docs/architecture.md` through; the full defect list | a new report, `docs/reports/0056-REPORT-documentation-defect-list.md`; `docs/README.md` | strongest, high | every MADR row and survey lead verified or refuted with evidence; both files read in full; each defect assigned to a phase |
| 2 | wrong statements in Markdown | the Markdown files the report's "wrong" rows name | mainstream, medium | every "wrong" Markdown row fixed and its evidence command run again |
| 3 | missing feature docs | `README.md`, `docs/architecture.md`, `tools/README.md` | mainstream, medium | the inventory audit reports 0 undescribed items |
| 4a | doc comments and wrong or missing Go-file statements, hand-written | hand-written non-test `.go` files of the public packages | mainstream, medium | the `go/ast` audit reports 0 undocumented names in scope |
| 4b | doc comments, generated: each variant type's comment names its discriminator field | `cmd/generate/internal/emit/*`, the regenerated `*_gen.go`, the engine goldens | strongest, high | the same audit reports 0; `make api` reports no change; `TestGeneratedFilesUpToDate` passes |
| 5a | `CHANGELOG.md`, `RELEASING.md` | `CHANGELOG.md` (new), `RELEASING.md`, `docs/architecture.md` | mainstream, medium | `Unreleased` and `v1.36.0` sections and the pointer, its links resolving; `RELEASING.md` describes the changelog and the release tools |
| 5b | `CHANGELOG.md`'s routing and link check | `ci.yml`, `docs.yml`, `AGENTS.md`'s docs-safe list, `docs/architecture.md`, `scripts/check_records.py` | mainstream, medium | a changelog-only change is docs-safe; the link check and `check_ci_filters.py` seen to fail on scratch breaks; `make check-tools` passes |
| 6 | index and matrix | `docs/README.md` | strongest, high | the index matches the files; the matrix has one row per reader task |

The model class is the cheapest tier that can pass the phase's gate. Mechanical work goes to the lightweight tier,
docs written from a clear outline to the mainstream tier at medium effort, and judgment to the strongest tier at high
effort.

The table lists the phases by number. They run in the order that "Timing" gives: 1, 2, 5a, 3, 6, 5b, 4a, 4b.

## Implementation Steps

### Phase 1: verify, read through, and list every defect

**Model:** strongest, high effort. Which features no doc describes, and whether a sentence is false, are judgments.

**Files:** `docs/reports/0056-REPORT-documentation-defect-list.md` (new); `docs/README.md` (its row, and this PLAN's
status). No doc is corrected in this phase.

1. Record the base: `git rev-parse HEAD`, `git status --short` (it must be empty but for this phase's files),
   `git describe --tags --abbrev=0`, and `git diff --stat v1.36.0..HEAD -- '*.go' Makefile go.mod`. If code has changed
   since `77a81cb`, the rows in the MADR are re-checked against the new code, not taken over.
2. Check each MADR row again (W1 to W3, M1 to M9, S1 to S3) with the command or reading that row names, and give each
   one a status: verified, refuted, or changed since the MADR.
3. Read `README.md` from line 1 to the end. For each statement about behaviour, a default, a name, a path, a command
   or an output, find the code that decides it and record either "holds" or a defect with `file:line` on both sides.
   Statements about the wire are checked against the generated code and the protocol table
   (`cmd/generate/internal/model/acp.go`). The sections the survey could not verify (its A3) are included. Record the
   line ranges read, so that "read in full" can be checked.
4. Do the same for `docs/architecture.md`, from line 1 to the end, with every path in its "Tree" section checked
   against its description, not only against the disk.
5. Read `RELEASING.md`, `tools/README.md`, `AGENTS.md`, the prose of `docs/README.md`, every package's `doc.go` or
   package comment, the usage text of every command (the `flag` definitions of `example/*`, `cmd/generate` and
   `cmd/generate/internal/mutate`, and the `argparse` definitions of `tools/*.py` and `scripts/*.py`, read from the
   source), and the `##` text of each Make target, in the same way.
6. Inventory, against MADR D1:
   * the packages, from `go list ./...` in each of the two modules;
   * the commands: every `package main`, every `tools/*.py` and `scripts/*` with a `main`;
   * the flags: every `flag.*` call in non-test Go, and every `add_argument` in `tools/` and `scripts/`;
   * the environment variables: every `os.Getenv` and `os.LookupEnv` in Go, every `os.environ` read in Python, and
     every `?=` variable and `$(…)` environment read in the `Makefile` and `scripts/go-precheck.sh`;
   * the settings keys: the connection's `With…` options, the option structs' exported fields that a user sets, and
     `opencode.json` (MADR M9);
   * the Make targets, from `^[a-zA-Z0-9_-]+:` in the `Makefile`.

   Mark each one as described, with the doc and line, or not described. This inventory is the outline phase 3
   follows.
7. Turn the survey's name-level leads (MADR M8) into feature rows. For each name, name the feature it belongs to and
   whether a doc describes that feature. Only an undescribed feature is a defect.
8. Run phase 4's `go/ast` audit (phase 4a, step 1) once, in the scope the owner set for Open Question 4, after its
   planted-break check, to size phase 4. Record its counts by package, and how many members the type-comment rule
   clears. Running it writes nothing to the tree.
9. Write the report. It is an observation that decides nothing. Give it one table per class: wrong, missing feature
   doc, missing doc comment, stale. Each row carries an id, the location, the evidence, and the phase that fixes it
   (2, 3, 4a, 4b, 5 or 6). Refuted leads go in a table of their own, with their evidence, so that nobody checks them
   again. Add its row to `docs/README.md`.
10. A defect in a file no phase lists, or one that needs a decision rather than a fix, is listed as such and put to the
    owner. The pair is amended before any phase acts on it.

**Verification**

* `python3 -B scripts/check_records.py --check-all`: exit 0, no output.
* `python3 -B scripts/check_disclosure.py`: exit 0, "0 finding(s)".
* `make markdownlint`, or offline `markdownlint-cli2` at the pinned version when it is installed: exit 0. The report
  is linted, since only MADR and PLAN files are excluded.

**Acceptance**

* Every MADR row and every survey row has a status backed by a command or a line reference.
* The report states that `README.md` and `docs/architecture.md` were read in full, with the line ranges.
* Every defect has a phase. The counts per class are stated.

### Phase 2: wrong statements

**Model:** mainstream, medium effort. The report gives each correct fact and its source. A row whose correct statement
is not clear from the code is a judgment, and the phase stops on it.

**Files:** the Markdown files named by the report's "wrong" rows. At `77a81cb` that is `docs/architecture.md` (W1),
plus whatever phase 1 finds in `README.md`, `docs/architecture.md`, `RELEASING.md`, `tools/README.md` and `AGENTS.md`.
Factual corrections to `AGENTS.md` are in scope. A wrong statement in a Go file, whether a doc comment or a `doc.go`,
belongs to phase 4a, or to phase 4b when it is generated (Timing).

1. Before any edit, re-run each row's evidence command on the current HEAD (Ordering, step 2).
2. Change each wrong statement to what the code says, citing nothing new. For W1: the sentence at
   `docs/architecture.md:1205` names `terminals.go` in place of `output.go`, and the rest of the sentence is checked
   against `clienthost/terminals.go` again.
3. Phase 2 runs before phase 5a (Timing), so phase 5a's `Unreleased` section records these corrections as one line.
   If phase 5a has landed first, add that line here.

**Verification**

* For each row, its evidence command now shows the statement holds. For W1: `grep -n 'output\.go' docs/architecture.md`
  prints nothing (exit 1), and the limit names the sentence cites are present in `clienthost/terminals.go`.
* `git diff --cached` shows only hunks that map to the report's rows. Each hunk's row id is recorded.
* `python3 -B scripts/check_records.py --check-all`, `python3 -B scripts/check_disclosure.py` and markdownlint: exit 0.
* When `README.md` changed: `run-heavy -- make pre-add-check FILES="<changed files>"`, exit 0. No `.go` file is in
  this phase.

**Acceptance:** every "wrong" row assigned to phase 2 is marked fixed in the execution record with its re-run
evidence. No other content changed.

### Phase 3: missing feature docs

**Model:** mainstream, medium effort, writing from phase 1's inventory.

**Where the docs go:** Open Question 7, answered "Existing files" by the owner. No new docs folder is created.

**Files:** `README.md` for the SDK's users (at `77a81cb`: connection logging, M4, and the M8 features phase 1
confirms, such as the error constructors and the `acptest` types), `docs/architecture.md` for maintainers (M1, the
Make targets; M2, the Make variables; M3, the generator's and the mutation tool's flags), `README.md`'s "Try the
Examples" (M3, `example/claude-code -yolo`), and `tools/README.md` (M5, if phase 1 found it in scope). A package whose
own description is missing from its `doc.go` goes to phase 4a. The survey found a package comment in every package.

1. Write one inventory audit as a scratch script. It takes the repository root, rebuilds phase 1's inventory (step 6)
   from the code, and for each item searches the docs that D1 names for a description: the item's name as written
   (`make vet`, `` `-schema` ``, `ACP_TOOLS_CACHE`) in a sentence about it. It prints each item not found, and exits 1
   when there is any.
2. Planted break: on a scratch clone, remove one Make target's description (for example, `make api` from every doc)
   and run the audit. It must print that target and exit 1. Record the output.
3. Run the audit on the tree. Its list must equal the report's "missing feature doc" rows for phase 3. A difference
   is an unexpected state.
4. Describe each item in the file named above. A Make target gets what it does, what it needs, and whether CI runs
   it. A flag gets its meaning and default. An environment or Make variable gets who reads it and its default.
5. Add the line to `CHANGELOG.md`'s `Unreleased` section if phase 5a has landed.

   *Amended 2026-11-15 by the owner's rulings, relayed by the coordinator ("2026-11-15: the owner's rulings on phase
   1's three items", Execution record).*

   * **M9 is ruled.** The owner chose "One line in the architecture Tree": "Add the file and what its key does to the
     Tree section of docs/architecture.md, which lists the repository's files. One line in the missing-feature phase;
     a tracked file no doc names is then accounted for." Rejected: "In AGENTS.md instead"; "Not a settings key; drop
     it". Phase 3 adds that one line to `docs/architecture.md`'s "Tree", saying what `opencode.json`'s `instructions`
     key does (`opencode.json:3`, the agent harness's pointer to `.opencode/rules.md`). The report's row M9 stands as
     written.
   * **The standard for a flag** (step 4's "a flag gets its meaning and default", confirmed). The owner chose "Meaning
     and default for each": "Every flag gets at least a sentence saying what it does, with its default, where its
     reader would look. Adds the 15 rows to the missing-feature phase. This matches what you ruled for go-tui-lib
     ('at least one sentence, with its default'), so the two repositories hold one standard." Rejected: "Strict for
     user-facing commands only"; "Named is enough". A flag named in a usage line or a list, and nothing more, is
     undescribed. A flag with no default needs the sentence alone. Where the reader looks: the mutation tool's flags
     in `docs/architecture.md`, beside `cmd/generate`, or its package comment, which lists them already
     (`cmd/generate/internal/mutate/main.go:10`-`:12`); `example/gemini`'s in `README.md`'s "Try the Examples"; the
     tools' in `tools/README.md`; the scripts' in `docs/architecture.md`. The inventory audit of step 1 changes to
     match: a flag counts as described only with a sentence and, where it has one, its default, so its list on the
     tree is the report's 32 items plus the rows below, and step 3's equality holds against that list.
   * **The 15 rows the report names** ("For the owner": the mutation tool's 10 flags and `example/gemini`'s 5). Each is
     named, in the mutation tool's usage line (`mutate/main.go:12`) or `README.md:79`'s list, and has no sentence or
     default in a reference doc. Where each is defined, read from the code on `632e097`:

     | Flag | Defined at |
     | :--- | :--- |
     | `-dir` | `cmd/generate/internal/mutate/main.go:65` |
     | `-files` | `cmd/generate/internal/mutate/main.go:66` |
     | `-pkgs` | `cmd/generate/internal/mutate/main.go:67` |
     | `-skip` | `cmd/generate/internal/mutate/main.go:68` |
     | `-only` | `cmd/generate/internal/mutate/main.go:69` |
     | `-n` | `cmd/generate/internal/mutate/main.go:70` |
     | `-j` | `cmd/generate/internal/mutate/main.go:71` |
     | `-seed` | `cmd/generate/internal/mutate/main.go:72` |
     | `-timeout` | `cmd/generate/internal/mutate/main.go:73` |
     | `-o` | `cmd/generate/internal/mutate/main.go:74` |
     | `example/gemini`'s `-gemini` | `example/gemini/main.go:209` |
     | `example/gemini`'s `-model` | `example/gemini/main.go:210` |
     | `example/gemini`'s `-sandbox` | `example/gemini/main.go:211` |
     | `example/gemini`'s `-yolo` | `example/gemini/main.go:212` |
     | `example/gemini`'s `-debug` | `example/gemini/main.go:213` |

   * **The stricter standard adds 15 more rows**, found by checking each flag of `cmd/generate`, `example/*`, the
     mutation tool, `tools/*.py` and `scripts/*.py` that phase 1 counted as described against the standard: its
     doc line, read, for a sentence and a default. A flag that has both is not listed: `--out DIR`, whose default
     `tools/README.md:21` states for every tool; `scripts/push_guard.py`'s `--remote` and `--branch`
     (`AGENTS.md:150`-`:151`); `-write-unstable-manifest` (`docs/architecture.md:966`, `RELEASING.md:81`);
     `scripts/check_ci_filters.py --docs-safe` (`docs/architecture.md:907`); `scripts/check_records.py --next`
     (`AGENTS.md:91`, `RELEASING.md:133`); `watch_ci.py`'s `--branch`, `--settle` and `--every` (`tools/README.md:43`);
     `idscan.py --range` (`tools/README.md:36`); and `ci_jobs.py`'s `--expect` and `--others` (`tools/README.md:44`).
     The rows, with what each doc line lacks. This is the reading of the standard recorded here; phase 3's audit
     takes it as its rule, and a flag a later reading finds short is a new row by amendment, not a silent addition.

     | Flag | Defined at | What the docs lack |
     | :--- | :--- | :--- |
     | `cmd/generate`'s `-out` | `cmd/generate/main.go:42` | `docs/architecture.md:1005` names it only to say where `main.go` finds the root's import path; no sentence says it is the output directory, nor that it defaults to the repository root |
     | `example/remote serve`'s `-addr` | `example/remote/main.go:63` | `README.md:80` gives it a value in an example and `:1512` writes `ADDR`; neither says it is the listen address or that `127.0.0.1:7331` is its default |
     | `scripts/check_records.py --check-all` | `scripts/check_records.py:257` | `AGENTS.md:99` names it in a command; no doc says it is `--check` plus the unnumbered docs |
     | `tools/ci_jobs.py tally --branch` | `tools/ci_jobs.py:277` | the `--branch` in `tools/README.md:43` is `watch_ci.py`'s; `tally`'s, default `main`, is described nowhere |
     | `tools/consumer_probe.py --run` | `tools/consumer_probe.py:65` | `tools/README.md:41` shows `[--run PATTERN]` in the synopsis only; no sentence says it is `go test`'s `-run` pattern, nor that every test runs by default |
     | `tools/gates.py --all` | `tools/gates.py:147` | `tools/README.md:29` shows `[--all]` and says what no gate named does; no sentence says `--all` adds the hosts' gates |
     | `tools/idscan.py --all` | `tools/idscan.py:90` | `tools/README.md:36` shows it in the synopsis; only `--range` has a sentence |
     | `tools/idscan.py --paths` | `tools/idscan.py:92` | the same line; only `--range` has a sentence |
     | `tools/interop.py --ts` | `tools/interop.py:265` | `tools/README.md:42` shows `[--ts REV]`; its sentence says the SDKs are cloned at a pinned revision, not that `--ts` overrides the TypeScript SDK's |
     | `tools/interop.py --py` | `tools/interop.py:266` | the same line, for the Python SDK's revision |
     | `tools/interop.py --tree` | `tools/interop.py:268` | `docs/architecture.md:1403` says what it builds, and no doc gives its default, the repository |
     | `tools/platforms.py --tree` | `tools/platforms.py:47` | `tools/README.md:31` describes `platforms.py` with no flag; the `--tree` hits are other tools' |
     | `tools/plants.py --tree` | `tools/plants.py:135` | `tools/README.md:37` describes `plants.py SPEC.json` with no flag; the `--tree` hits are other tools' |
     | `tools/remote.py test --module` | `tools/remote.py:276` | `tools/README.md:34` shows `[--module DIR]` and says "the module `DIR` names"; its default `.` is not stated |
     | `tools/remote.py test --timeout` | `tools/remote.py:278` | `tools/README.md:34` shows `[--timeout S]` and says a timeout exits 124; the default, 3600 seconds, is not stated |

     That is 15 rows, and 30 flags in all for the standard (15 and 15), beside the report's 32 items, whose 15
     flags are `-schema`, `-yolo` and the 13 of the tools and scripts (M3a, M3b, M3c). The standard changes none of
     the report's other rows. It does not apply to positional arguments.

**Verification**

* The audit on the tree: exit 0, no item printed.
* The audit's planted-break output, recorded.
* `python3 -B scripts/check_records.py --check-all`, `python3 -B scripts/check_disclosure.py` and markdownlint: exit 0.
* When `README.md` changed: `run-heavy -- make pre-add-check FILES="README.md <other changed files>"`, exit 0. Its
  `docs_test.go` step checks that the README examples still match `example_*_test.go`.

**Acceptance:** the audit reports 0. Each phase 3 row of the report is marked fixed with the doc and line that now
describes it.

### Phase 4a: doc comments on hand-written code

**Model:** mainstream, medium effort.

**Depends on:** Open Question 4, answered (b) by the owner on 2026-11-14: "Top-level names, plus exported fields and
interface methods of public packages and of the types users configure or receive (wire and config structs). A field
counts as documented when its type's comment names it." The scope is the public packages: the root `acp`,
`unstable`, `acptest`, `agentproc`, `clienthost` and `remote`. `internal/*` and `cmd/generate/internal/*` are out of
scope. Per Timing, this phase waits while an approved PLAN is about to rewrite code in a file it would touch.

**Files:** the hand-written non-test `.go` files that the audit names, and the Go-file statements moved here from
phases 2 and 3. At `77a81cb`, before the type-comment rule: `compaction.go`, `connection.go`, `elicitation.go`,
`errors.go`, `extensions.go`, `observer.go`, `tooltimeline.go`, `acptest/client.go`, `acptest/pair.go`,
`acptest/recorder.go`, `acptest/replay.go`, `acptest/script.go`, `agentproc/agentproc.go` and
`agentproc/inprocess.go`: 52 members.

1. **The measurement.** Write a `go/ast` program in the session's scratch directory, using the standard library alone,
   and run it with `go run <scratch>/undoc.go <repo>`. It reads the tree and writes nothing to it. It parses every
   tracked non-test `.go` file of the public packages with `parser.ParseComments`, and marks each file as generated
   when it carries the standard "Code generated … DO NOT EDIT." line. It reports, per package and split into
   hand-written and generated:
   * exported top-level functions, types, constants and variables with no doc comment, a grouped constant or variable
     counting as documented by its group's comment, as `docs_test.go:22` counts it;
   * exported methods on exported receivers with no doc comment;
   * the exported fields (each name of a multi-name field, and embedded fields) and interface methods of exported
     types, those in `type ( … )` groups included. By the owner's rule, a member counts as documented when it has its
     own comment or when its type's doc comment names it, as a word. The program prints which rule documented each
     member.

   It exits 1 when anything in scope is undocumented.
2. **Planted break.** On a scratch clone, delete the doc comment above one hand-written method (for example
   `ClientRefusals.CompleteElicitation`, `refusals.go:21`-`:22`), above one generated type, and above one field in
   scope. The program must report all three and exit 1. Record the output. This record's lexical scan was checked the
   same way on 2026-11-14 for the first two.
3. Run it on the tree. Record the counts. They must agree with phase 1's.
4. Write each missing comment: it starts with the name and says what the member holds or does, its unit and its zero
   value where they matter. Naming the member in its type's comment is an alternative, when that reads better. Fix
   the Go-file statements moved here from phases 2 and 3 in the same way. Change no code. A comment that would state
   behaviour the code does not have is a wrong statement not in the report, and the phase stops on it.
5. Add the line to `CHANGELOG.md`'s `Unreleased` section if phase 5a has landed.

**Verification**

* The program on the tree: exit 0, with every hand-written package at 0 in scope.
* `gofmt -l` on the changed files prints nothing.
* `run-heavy -- make pre-add-check FILES="<changed .go files>"`: exit 0. That runs revive's `exported` rule and the
  `docs_test.go` tests, `TestExportedTypeDocsStartWithName` among them.
* `make api`: exit 0, with 0 incompatible and 0 compatible changes. Comments are not API.

**Acceptance:** 0 undocumented hand-written names in scope. The diff changes comments alone. A one-off script over
`git diff --cached -U0 -- '*.go'` confirms that every added or removed line is blank or a `//` comment, after it has
been seen to fail on a scratch copy with one changed code line.

### Phase 4b: doc comments on generated code

**Model:** strongest, high effort. It changes the generator, its goldens and every generated file.

**Files:** `cmd/generate/internal/emit/*.go` (the field and type docs are set in `emit/types.go`, where `fld.Doc` is set
from the schema's description at `:25`), its tests, the engine goldens under `cmd/generate/testdata/engine/`, and the
regenerated `agent_gen.go`, `client_gen.go`, `constants_gen.go`, `helpers_gen.go`, `types_gen.go` and
`unstable/unstable_gen.go`.

**How generated code is handled.** A `*_gen.go` file is never edited by hand. It says "DO NOT EDIT", and
`TestGeneratedFilesUpToDate` (`cmd/generate/main_test.go:46`) fails when it differs from the generator's output. The
emitter is changed, and the files are regenerated from the committed schema files with the two commands of
`make version` that do not download anything:

```bash
cd cmd/generate && env -u GOPATH -u GOMODCACHE go run .
cd ../.. && gofmt -w . && gofmt -w ./cmd/generate
```

`make version` itself is not run: it re-downloads the four schema files and runs `mdsh` over `README.md`.
`schema/version` does not change.

1. Change the emitter so that each variant type's existing doc comment gains one sentence that names its
   discriminator field and the field's constant value, for example `Its Type field is always "terminal".` This is
   Open Question 5, answered "Name it in the type's comment" by the owner. The sentence goes after the comment's first
   line, so that `TestExportedTypeDocsStartWithName` still holds. Use the variant's constant value from the model. Add
   no text the schema does not support. The 72 fields (70 in the root package, 2 in `unstable`) then count as
   documented under the owner's rule for Open Question 4.

   *Amended 2026-11-15 by the owner's ruling, relayed by the coordinator ("2026-11-15: the owner's rulings on phase 1's
   three items", Execution record).* The owner chose "Name them in the type's comment" for `Negotiation.Request` and
   `Negotiation.Response` (`types_gen.go:25584`-`:25585`): "Amend the generator phase so the generator names both fields
   in Negotiation's comment, the same treatment you chose for the discriminator fields. Generator text of its own, in
   files already in that phase's list; no API change. The audit then reaches zero by your rule that a field counts when
   its type's comment names it." Rejected: "A comment on each field"; "Count the existing prose". So step 1 gains a
   second change: the emitter also adds to `Negotiation`'s doc comment a sentence that names `Request` and `Response`,
   each as a word, and it keeps the comment's first line, so `TestExportedTypeDocsStartWithName` still holds. It adds no
   text the schema does not support: the sentence names the fields the comment already describes. For the rest of the
   phase:
   * step 2: the engine case or golden covers `Negotiation`'s comment as well, and the golden diff still shows comment
     lines only;
   * step 3: the comments-only check runs over the `*_gen.go` diff, which now includes `Negotiation`'s comment;
   * step 4 and the acceptance: the audit's 53 generated members are the 49 discriminator fields, the 2 in `unstable`
     and these 2. After both changes the audit reports 0 in the generated files, and the acceptance, "0 undocumented
     generated names in scope", can be met. Before this amendment it could not, as phase 1's report said;
   * the files are the ones already listed, so no file joins the phase, and `make api` still reports no change.
2. Add an engine case or change one so that the new comment is covered, and run the engine cases with their `-update`
   flag (`cmd/generate/engine_test.go:22`), in the one package `cmd/generate`: `go test . -run TestEngineCases -update`.
   Then read the golden diff: only comment lines change.
3. Regenerate as above. Phase 4a's comments-only script, run over the `*_gen.go` diff, confirms that only comment
   lines changed.
4. Run phase 4a's program: the generated packages are at 0 in scope.
5. Add the line to `CHANGELOG.md`'s `Unreleased` section if phase 5a has landed.

**Verification**

* `run-heavy -- make pre-add-check FILES="<changed files>"`: exit 0. It runs both modules' tests, so
  `TestGeneratedFilesUpToDate` and the engine cases are included. No separate `go test ./...` is run.
* `make api`: exit 0, with 0 incompatible and 0 compatible changes.
* Full CI runs on the owner's push, because `.go` files changed. That is the owner's to start. This PLAN does not
  push.

  *Amended 2026-11-15 by the owner's ruling, relayed by the coordinator ("Commits", above):* the coordinator pushes
  this phase after the disclosure guard exits 0, and full CI runs on that push.

**Acceptance:** 0 undocumented generated names in scope. The generated diff is comments only. Generator and output are
in the same commit.

### Phase 5a: `CHANGELOG.md` and `RELEASING.md`

**Model:** mainstream, medium effort.

**Depends on:** Open Question 1, answered by the owner on 2026-11-14 ("Latest tag forward"), and Open Question 8, as
the coordinator settled it: `AGENTS.md` gains no docs-step section.

**Files:** `CHANGELOG.md` (new); `RELEASING.md`; `docs/architecture.md` (the "Tree" section, `:1414`). Markdown only.

1. Write `CHANGELOG.md` with three parts and no more:
   * `## Unreleased`: what phases 1 and 2 changed, if they have landed, one line each.
   * A section for the newest tag when the phase runs, today `## v1.36.0 — 2026-11-11`. The date comes from
     `git for-each-ref --format='%(taggerdate:short)' refs/tags/v1.36.0`. Add one short summary from the "Summary" of
     `docs/reports/0054-REPORT-release-notes-v1-36-0.md`, and a relative link to that report.
   * A closing pointer, one paragraph of one or two sentences, to the older notes. For example: "Releases before
     v1.36.0 are described by the release-notes reports listed in `docs/README.md`
     (`docs/reports/*-REPORT-release-notes-v*.md`). v1.14.0's notes are filed as
     `0019-REPORT-release-notes-v0-18-0.md`, v1.13.0 was retracted (`go.mod`), and tags before v0.15.0 have no
     notes." In `CHANGELOG.md`, `docs/README.md` is a relative link. This covers the three facts of the MADR's S3
     without a line per tag.
2. `RELEASING.md`, as the MADR's Consequences say:
   * "Release notes and the API": `CHANGELOG.md` is the changelog. Each change adds its line under `Unreleased` in the
     same phase. The release's PLAN turns `Unreleased` into the version and its date, linking the release-notes
     report. The report keeps its number, its index row, and its "Release text", which is still the release's body.
   * "Tag and Publish", step 3: the body comes from `python3 tools/release_body.py VERSION REV`. After the tag, the
     checks the recent release PLANs ran are `tools/watch_ci.py`, `tools/publication.py` and `tools/consumer.py`,
     each described in `tools/README.md`, all run by hand. 0048 is not described.
   * "Additional Notes": the same-commit rule is widened to the docs step and the changelog line.
3. `docs/architecture.md`'s "Tree": add `CHANGELOG.md` beside `RELEASING.md`.
4. `AGENTS.md` is not changed in this phase (Open Question 8, as settled).

**Verification**

* A one-off script checks `CHANGELOG.md`. Its version headings are exactly `Unreleased` and the newest tag from
  `git describe --tags --abbrev=0`. That tag's date equals its tag date. Every relative link resolves to a tracked
  file. It is first run on a scratch copy with the `v1.36.0` heading removed and a link broken, and must report both.
* `grep -n 'The changelog is the release notes report' RELEASING.md` prints nothing (exit 1).
* `python3 -B scripts/check_records.py --check-all`, `python3 -B scripts/check_disclosure.py` and markdownlint: exit 0.
  `CHANGELOG.md` is linted, as `**/*.md` is. Its links are not checked by `check_records.py` unless phase 5b adds it,
  so the one-off script checks them.
* `CHANGELOG.md` is not docs-safe until phase 5b, so
  `run-heavy -- make pre-add-check FILES="CHANGELOG.md RELEASING.md docs/architecture.md"` runs every Go check: exit 0.

**Acceptance:** `CHANGELOG.md` has the two sections and the pointer, with dates and links correct. `RELEASING.md`
describes the changelog, the reports and the release tools as they are used.

### Phase 5b: `CHANGELOG.md`'s routing and link check

**Model:** mainstream, medium effort. It is not a Markdown-only phase. It runs after phase 5a (Timing).

**Basis:** Open Question 6, answered "Docs path and link check" by the owner, who read it this way against "Fix the
docs, no new checks": "No new check: an existing link check reads one more file, and changelog-only commits run the
light Docs workflow instead of the Go suite."

**Files:** `.github/workflows/ci.yml` (both `paths-ignore` lists), `.github/workflows/docs.yml` (both `paths` lists),
`AGENTS.md` ("CI routing", the docs-safe list, as a factual correction), `docs/architecture.md` (`:913`-`:919`, which
describe those lists), and `scripts/check_records.py` (`UNNUMBERED_DOCS` at `:239` and the docstring at `:9`-`:12`).

1. Add `'CHANGELOG.md'` to the four lists in the two workflows, in the same position, and to `AGENTS.md`'s and
   `docs/architecture.md`'s descriptions of them.
2. Add `"CHANGELOG.md"` to `UNNUMBERED_DOCS` and to the docstring's list. No other change to either script or
   workflow.

**Verification**

* `make check-tools`: exit 0. It includes `scripts/check_ci_filters.py`, which requires the workflows to agree.
  `python3 -B scripts/check_ci_filters.py --docs-safe CHANGELOG.md`: exit 0. `actionlint` on both workflows: exit 0.
* A changed list is unused until it has been seen to work. On scratch clones of the tree with this phase's changes:
  * `python3 -B scripts/check_records.py --check-all` with one deliberately broken link in `CHANGELOG.md` exits 1 and
    names that link. On the real file it exits 0.
  * `python3 -B scripts/check_ci_filters.py` with `'CHANGELOG.md'` removed from `docs.yml`'s lists alone exits 1.
    The same with it removed from `ci.yml`'s lists alone exits 1. On the tree it exits 0.

  The outputs are recorded.
* `run-heavy -- make pre-add-check FILES="<changed files>"`: exit 0. The workflows and the script are not docs-safe.

**Acceptance:** a changelog-only change is docs-safe, and `CHANGELOG.md`'s links are checked with the other
unnumbered docs. Both are shown working by the scratch failures above.

### Phase 6: index and matrix

**Model:** strongest, high effort. Regrouping the matrix by reader task (Open Question 2, answered "Row per reader
task" by the owner) is a judgment.

**Files:** `docs/README.md`. The 48 reports are not changed (Open Question 3, answered "Leave them as they are").

1. The Records table: every record file has a row, and every MADR and PLAN row's status equals its frontmatter
   `status`. This pair's rows and the 0056 report's row are brought up to date.
2. The matrix: one row per task a reader comes with, in the reader's words, each pointing to the current record or
   doc for it. The paragraph above it says that the Records table is the complete per-record index, and that a record
   which adds no new task adds no row. Every task the 31 MADRs of the MADR's S1 answer is covered by some row, which
   need not be one per MADR.
3. `docs/README.md`'s first line ("Guides are not in the tree yet") stays, as no guide is created.

**Verification**

* A one-off script compares the record files under `docs/decisions/` and `docs/reports/` with the Records table's
  links, both ways, and compares each MADR and PLAN row's status with the file's frontmatter. It prints nothing and
  exits 0. It is first run on a scratch copy with one row removed and one status changed, and must report both.
* The execution record lists each task row and the record or doc it points to, and, for each of the 47 MADRs, the
  row that covers its task or the reason it adds none (superseded, or no task of its own). There is no count to
  compare.
* `python3 -B scripts/check_records.py --check-all`, `python3 -B scripts/check_disclosure.py` and markdownlint: exit 0.

**Acceptance:** the index matches the files and their statuses. The matrix has one row per reader task, and every
MADR is accounted for in the execution record. This PLAN's status becomes `complete` after the last phase in
Timing's order, and its `docs/README.md` row records it.

## Verification

* Phase 1's report lists every defect, and each one is fixed by its phase or carried, with a reason, to a later
  amendment.
* The one-off audits of phases 3, 4a, 5a and 6 print nothing on the final tree. Their outputs, and their planted-break
  failures, are in the execution record. None of them is committed.
* `make check-records`, `make markdownlint` and `make disclosure-check` pass. `make pre-add-check` passed for each phase
  that changed a file that is not docs-safe.
* `make api` reports no change against `v1.36.0`. This PLAN changes no exported API.

## After this PLAN: the docs step in every brief

Per the mandate and MADR D5, every later brief and PLAN in this repository that changes code carries a docs step:

* name the docs the change makes false or newly needs: `README.md`, `docs/architecture.md`, `RELEASING.md`,
  `tools/README.md`, a package's `doc.go`, the changed names' doc comments, and `CHANGELOG.md`'s `Unreleased`;
* change them in the same phase as the code;
* record in the execution record which docs changed, or that none needed to.

A release PLAN also does the release-caused fixes before its tag (MADR D4).

## Rollout and Rollback

* **Rollout:** one commit per phase on `main`, in the order "Timing" gives (1, 2, 5a, 3, 6, 5b, 4a, 4b), each after
  its checks pass. The Markdown phases run one at a time and pause at a phase boundary if a release step starts. Phase
  1 always comes first, and 4b comes after 4a. Nothing is pushed or tagged by this PLAN. The owner pushes, and full
  CI runs for any phase that changed a file that is not docs-safe.

  *Amended 2026-11-15 by the owner's ruling, relayed by the coordinator:* "I commit and push each owner-approved
  phase of any plan on the coordination ledger, in the six related repos, after the disclosure guard exits 0." ("I"
  is the coordinator; the full ruling is under "Commits".) The coordinator, not the owner, commits and pushes each
  approved phase, after the disclosure guard exits 0, and full CI runs on that push for any phase that changed a file
  that is not docs-safe. This PLAN still cuts no tag.
* **Rollback:** `git revert <commit>` for a phase, as a new commit. A phase 4b revert takes the generator, its goldens
  and the generated files together, since they are one commit and `TestGeneratedFilesUpToDate` needs them to agree. A
  phase 5a revert removes `CHANGELOG.md` and restores `RELEASING.md` together, after any 5b commit is reverted.
  Reverting phase 5b makes a changelog-only change run full CI again, which is safe. No release is involved, so there
  is nothing to retract.

## What this PLAN does not do

* It adds no gate, CI job or check script, and commits no audit script (MADR D3). Phase 5b adds one path to
  existing lists and nothing else, which the owner ruled is no new check (Open Question 6).
* It adds no docs-step section to `AGENTS.md` (Open Question 8, as the coordinator settled it). The docs step lives in
  the coordinator skill and in briefs.
* It changes no behaviour and no exported API, and adds no dependency.
* It pushes nothing and cuts no tag (Open Question 9, as the owner answered it). *Amended 2026-11-15 by the owner's
  ruling, relayed by the coordinator ("Commits"):* the coordinator pushes each approved phase after the disclosure
  guard exits 0. It still cuts no tag and makes no release.
* It does not implement [0048-MADR-create-releases-in-ci.md](0048-MADR-create-releases-in-ci.md), or describe it as
  done.
* It does not rewrite the content of existing records: their history stands. The only edits to records are the index
  rows and this pair's own records. The 48 reports keep their form (Open Question 3).
* It does not touch other repositories. Each has its own pair under the same mandate.
* It does not create `docs/guides/` or any other docs folder (Open Question 7).

## Execution record

Not started. The pair is `proposed`, and no phase is approved.

*Annotated 2026-11-14: superseded by the approval entry below.*

### 2026-11-14: approval

* **Approval.** The owner, relayed by the coordinator: "Approved to proceed with the docs update plan for
  acp-go-sdk2". Earlier the same day, asked to approve the three docs plans, the owner had answered "Not yet", and
  the pair stayed `proposed` until the owner had read it. This approval follows that reading and covers this
  repository's PLAN alone, not the other repositories' docs plans.
* **Status.** The MADR is `accepted` and this PLAN is `in-progress`, with their `docs/README.md` rows to match.
* **Executed.** Nothing. No phase has started, and no file outside this pair and its two index rows has changed.
* **Order.** The phases run in the order "Timing" gives: 1, 2, 5a, 3, 6, 5b, 4a, 4b. Phase 1 comes first.
* **Commits and pushes.** The coordinator makes them, under the owner's standing approval of 2026-11-12 for the
  phases of an approved PLAN, after the disclosure guard (`make push-guard`). The executing agent stages each phase
  and does not commit, push or tag. This PLAN still cuts no tag. The first commit holds this pair and its two
  `docs/README.md` rows alone (Preconditions).

  *Annotated 2026-11-15:* the scope of the 2026-11-12 approval was stated in the coordinator's brief as fact
  before the owner had ruled on it. On 2026-11-15 the owner confirmed, relayed by the coordinator, that the approval
  extends to this PLAN ("Yes, all approved plans"; the ruling is quoted under "Commits").

### 2026-11-15: phase 1

Provenance: run by the strongest model at high effort, as the phase's row prices it, on the coordinator's brief.

* **Base (step 1).** `git rev-parse HEAD`: `e67216c7ae19f04ee9a7c8c0259ae88407ac502f`, the commit holding this pair.
  `git status --short` was empty. `git describe --tags --abbrev=0`: `v1.36.0`. Before any edit,
  `git diff --stat v1.36.0..HEAD -- '*.go' Makefile go.mod` printed nothing, so no code had changed since `77a81cb`.
  Before this phase started, no release step was in progress (Timing): no PLAN but this one is `in-progress` in
  `docs/README.md`, and no tag was being prepared.
* **What was read (steps 3 to 5).**
  * `README.md`, lines 1 to 1927, in full, in the ranges 1-260, 260-510, 510-760, 760-989, 989-1218, 1218-1467,
    1467-1706 and 1706-1927.
  * `docs/architecture.md`, lines 1 to 1469, in full, in the ranges 1-260, 260-500, 500-760, 758-958, 957-1217 and
    1216-1469, with every "Tree" path checked against the disk.
  * In full: `RELEASING.md`, `tools/README.md`, `AGENTS.md`, `docs/README.md`'s prose and matrix, every package
    comment, the `flag` definitions of `example/*`, `cmd/generate` and `cmd/generate/internal/mutate`, and every
    Make target's `##` text.
  * Read in part: the `argparse` definitions of `tools/*.py` and `scripts/*.py`. The flags were inventoried; their
    help text was not checked against what each tool does.
* **The report (step 9).** It is
  [0056-REPORT-documentation-defect-list.md](../reports/0056-REPORT-documentation-defect-list.md).
  * Wrong: 5 rows. W1 and W4 to W6 go to phase 2; W7, the root `doc.go`'s list of sentinels, goes to phase 4a.
  * Missing feature doc: 13 rows, 46 items. Eleven rows go to phase 3; M6 and M7 go to phase 5a.
  * Missing doc comment: 87 members. Phase 4a has 34 hand-written; phase 4b has 53 generated.
  * Stale: 2 rows, S1 and S4, both for phase 6.
  * Refuted: W2 and M5.
* **MADR rows (step 2).**
  * Verified: W1, M1, M4, M6, M7, S2 and S3.
  * Verified and changed: M2, which gains `GIT_BASH`, and S1, which has 32 rows with this pair's MADR.
  * M3 partly changed: the mutation tool's flags are named in its package comment, and 13 flags of the tools and
    scripts are new.
  * M8 was turned into features M4, M10, M12 and M13.
  * M9 is judged in scope.
  * W3 was done by the two readings.
* **The inventory audit (steps 6 and 7; phase 3's audit).**
  * On the tree: exit 1, 223 items, 32 undescribed: 15 flags, 5 variables, 5 settings keys and 7 Make targets. No
    package or command is undescribed. 8 environment variables that belong to Make or to the operating system are
    listed and not counted, and 8 items were judged described by reading.
  * Planted breaks on a scratch clone: `ACPTEST_UPDATE` removed from the docs printed
    `UNDESCRIBED env: ACPTEST_UPDATE (acptest/golden.go)`, exit 1. `make api` removed from the docs and the package
    comments printed `UNDESCRIBED target: api`, exit 1.
* **The doc-comment audit (step 8; phase 4a's measurement).**
  * On the tree: exit 1, 87 undocumented in scope, all of them fields or interface methods; top-level declarations
    and methods are at 0.
  * By package: `acp` generated 51 of 1,203 members, `acp` hand-written 22 of 43, `acptest` 10 of 20, `agentproc`
    2 of 14, `clienthost` 0 of 4, `remote` 0 of 11, and `unstable` 2 of 56.
  * The type-comment rule clears 40: 21 hand-written and 19 generated.
  * Planted breaks, four on a scratch clone:
    * three were reported, exit 1, 90 undocumented: `ClientRefusals.CompleteElicitation`, `AgentAuthCapabilities`
      and `DialOptions.Header`;
    * `Files.MkdirAll`, a field its type's comment names, was not reported.
  * It wrote nothing to the tree.
* **Step 10, for the owner.** `Negotiation.Request` and `Response` are generated and are not discriminators. Phase
  4b's step 1 does not cover them, so phase 4b cannot reach 0 as written. The report gives the resolutions. The pair
  is not amended in this phase.
* **Not verified.** The report's "Not verified" section lists the line ranges whose behavioural claims were read but
  not traced through the code.
* **Verification.** Each output went to a scratch file, and its exit status was taken before it was read.
  * `python3 -B scripts/check_records.py --check-all`: exit 0, no output.
  * `python3 -B scripts/check_disclosure.py`: exit 0, "0 finding(s)", run again once the report was staged.
  * Offline `markdownlint-cli2` v0.23.2, the version the Makefile pins: exit 0, 56 files linted, the report among
    them.
* **Files.** `docs/reports/0056-REPORT-documentation-defect-list.md` (new); `docs/README.md` (the report's row, and
  this PLAN's row text); this record. No doc was corrected.
* **Docs.** No doc outside this phase's files was made false or left incomplete by the phase.
* **Commit.** Staged, not committed. The coordinator commits and pushes.

### 2026-11-15: phase 2

Provenance: run by a mainstream model at medium effort, as the phase's row prices it, on the coordinator's brief.

* **Approval.** The owner, 2026-11-15: "Pause the docs plan when you get the defect report." and, later the same day,
  "Proceed with the acp-go-sdk2 docs plan through the next phase". This phase alone is approved to run; the plan
  pauses again when it is done.
* **Base.** `git rev-parse HEAD`: `632e097d7d28fa8be96ebabf74f295abf71a0159`, on `main`. `git status --short` was
  empty. `git diff --stat 632e097 HEAD -- '*.go' Makefile go.mod` printed nothing, and the report's own base
  `e67216c` is two commits back, so no `.go` file, the `Makefile` or `go.mod` changed. No release step had started:
  the newest tag is `v1.36.0`, and no PLAN but this one is `in-progress`.
* **Step 1: each row's evidence, re-run on `632e097` before any edit.** Every row held as the report states it.
  * W1: `grep -n 'output\.go' docs/architecture.md` printed line 1205 (exit 0). `ls clienthost` holds no `output.go`.
    The limit is set at `clienthost/terminals.go:113`-`:117`; the cut from the front at a character boundary, 64 KiB
    past the limit, is `internal/tailbuf/tailbuf.go:19` (`compactAfter = 64 << 10`) and `:65`-`:72`, and the hold-back
    of an unfinished character while the command runs is `Snapshot(final)` (`tailbuf.go`), called as
    `term.out.Snapshot(exited)` at `terminals.go:152`.
  * W4: `example/remote/main.go:235` and `:271` are `InitializeRequest{ProtocolVersion: …}` with no
    `ClientCapabilities`, and `dialClient` embeds `acp.ClientRefusals` (`:303`-`:304`). `example/client/main.go:254`,
    `example/gemini/main.go:246` and `example/claude-code/main.go:230` advertise
    `Fs: {ReadTextFile: true, WriteTextFile: true}` and no `Terminal`, and their `WriteTextFile` refuses a path
    outside the working directory; their terminal methods return `NewMethodNotFound`. `example/agent` is an agent, not
    a client.
  * W5: `grep -c '^func Test' agentproc/stderr_test.go` printed 7 (`:16`, `:49`, `:87`, `:110`, `:123`, `:143`,
    `:158`).
  * W6: counts of `^func Test…(t *testing.T)` per file in `remote/`: `frame_test.go` 3, `route_test.go` 4,
    `handler_test.go` 8, `httpclient_test.go` 16, `httpserver_test.go` 16, `ws_test.go` 17. The sum is 64.
    `grep -h '^func Test' remote/*_test.go | wc -l` printed 65, one of which is `TestMain` in `leak_test.go`.
* **Step 2: the changes.** Each hunk maps to one row.

  | Hunk | Row | Old | New |
  | :--- | :--- | :--- | :--- |
  | `docs/architecture.md:1205` | W1 | "The output (`output.go`) is kept" | "The output (`terminals.go`) is kept" |
  | `README.md:85`-`:88` | W4 | "The example clients advertise the file-system capability and not the terminal one, and write only inside the session's working directory." | "The example clients, `example/remote`'s apart, advertise the file-system capability and not the terminal one, and write only inside the session's working directory. `example/remote`'s `dial` advertises no capability and serves no file method." |
  | `docs/architecture.md:1170` | W5 | "the stderr tail's six in `stderr_test.go`" | "the stderr tail's seven in `stderr_test.go`" |
  | `docs/architecture.md:1336` | W6 | "`remote`'s 63 tests" | "`remote`'s 64 tests" |
  | `docs/architecture.md:1345` | W6 | "`ws_test.go`'s sixteen" | "`ws_test.go`'s seventeen" |

  The rest of W1's sentence was checked against `clienthost/terminals.go` and `internal/tailbuf` again and holds
  (the smaller of `outputByteLimit` and `MaxOutputBytes` at `terminals.go:110`-`:118`; the 64 KiB compaction; the
  held-back character), so only the file name changed. The other counts in W5's and W6's sentences and neighbours
  were counted on `632e097` and hold: `InProcess`'s thirteen (`inprocess_test.go`), the transcript's four, the bounded
  close's seven, `frame_test.go`'s three, `route_test.go`'s four, `handler_test.go`'s eight and its Host rule's two,
  `httpclient_test.go`'s sixteen, `httpserver_test.go`'s sixteen, `ws_test.go`'s three of back-pressure
  (`TestWebSocketSlowClientStaysConnected`, `TestWebSocketBusyAgentStaysConnected`,
  `TestWebSocketClientPastMaxBuffered`). No count was found wrong beyond the report's rows.
* **After, per row.** W1: `grep -n 'output\.go' docs/architecture.md` prints nothing (exit 1), and the limit names
  `outputByteLimit` and `MaxOutputBytes` are present in `clienthost/terminals.go`. W4: the sentence now holds for
  each example client the list names. W5: 7 matches the count. W6: 3 + 4 + 17 + 8 + 16 + 16 = 64 matches.
  W1, W4, W5 and W6 are fixed. W7 is a Go file and stays with phase 4a, untouched.
* **Step 3: the changelog line.** Phase 5a has not landed, so there is no `CHANGELOG.md` and no `Unreleased` section
  to add a line to. Phase 5a carries the line: one line for these corrections (the wrong statements W1 and W4 to W6
  in `README.md` and `docs/architecture.md`).
* **Docs step.** Searched `README.md`, `docs/architecture.md`, `RELEASING.md`, `tools/README.md`, `AGENTS.md`,
  `clienthost`'s package comment and `example/*`'s Go files for each corrected fact: the file name `output.go`
  (no other place), the example clients' capabilities (`README.md:85` alone), and the counts 6, 63 and sixteen
  for `stderr_test.go` and `remote` (no other place). Nothing else was made false or incomplete, and no Go-file row is
  opened by this phase. There is no changelog yet (phase 5a creates it).
* **Verification.** Each output went to a scratch file and its exit status was taken before it was read.
  * `git diff --cached --numstat`: `README.md` 4/2, `docs/architecture.md` 4/4; every hunk is in the table above.
  * `python3 -B scripts/check_records.py --check-all`: exit 0, no output.
  * `python3 -B scripts/check_disclosure.py`: exit 0, "0 finding(s)".
  * Offline `markdownlint-cli2` v0.23.2, the version the Makefile pins: exit 0, 56 files, 0 issues.
  * `run-heavy -- make pre-add-check FILES="README.md docs/architecture.md"`: exit 0 (waited in the queue about 33 minutes, ran 200 s, 12:03 to 12:07 local). `go-precheck`: 0 `.go` files to format; both modules linted, vetted and tested (`-race -shuffle=on`, uncached), tidy and `govulncheck`, all clean.
* **Not verified.** The report's "Not verified" ranges still stand: behavioural claims there were not traced. Test
  counts count `func Test…(t *testing.T)` declarations, not subtests.
* **Files.** `README.md` (W4); `docs/architecture.md` (W1, W5, W6); `docs/README.md` (this PLAN's row text); this
  record. The report is a dated observation and is not edited.
* **Commit.** Staged, not committed. The coordinator commits and pushes after the disclosure guard. The plan pauses
  until the owner says to continue.

### 2026-11-15: the owner's rulings on phase 1's three items

Provenance: written by a mainstream model at medium effort, on the coordinator's brief. This entry runs no phase. No
`.go` file, no doc and no report changed, and the report, a dated observation, is not edited. The coordinator committed
phase 2 as `f629f8f`, and the plan stays paused after this entry.

The owner ruled on 2026-11-15, relayed by the coordinator, on the three items the report's "For the owner" section put.
Each is recorded as a dated note where it applies: the MADR's Open Question 5 and D1, and this PLAN's phase 3 and
phase 4b.

1. **`Negotiation.Request` and `Negotiation.Response`.** The owner chose "Name them in the type's comment": "Amend the
   generator phase so the generator names both fields in Negotiation's comment, the same treatment you chose for the
   discriminator fields. Generator text of its own, in files already in that phase's list; no API change. The audit
   then reaches zero by your rule that a field counts when its type's comment names it." Rejected: "A comment on each
   field"; "Count the existing prose". Phase 4b's step 1 is amended (above), and so are its steps 2 to 4 and its
   acceptance, which can now be met. The MADR's Open Question 5 carries a dated note of what the generator change is.
2. **`opencode.json`.** The owner chose "One line in the architecture Tree": "Add the file and what its key does to the
   Tree section of docs/architecture.md, which lists the repository's files. One line in the missing-feature phase; a
   tracked file no doc names is then accounted for." Rejected: "In AGENTS.md instead"; "Not a settings key; drop it".
   Row M9 stands as written, and phase 3 notes that it is ruled.
3. **What counts as describing a flag.** The owner chose "Meaning and default for each": "Every flag gets at least a
   sentence saying what it does, with its default, where its reader would look. Adds the 15 rows to the missing-feature
   phase. This matches what you ruled for go-tui-lib ('at least one sentence, with its default'), so the two
   repositories hold one standard." Rejected: "Strict for user-facing commands only"; "Named is enough". The MADR's D1
   and phase 3 carry the standard. Phase 3's list gains the 15 rows the report names, with each flag's `file:line`
   read from the code, and 15 more that checking the flags phase 1 counted as described found (phase 3, above).
* **Not verified.** The second 15 rows are one reading of the standard, made by reading each doc line, not by a
  script. The phase 3 audit, which phase 3 writes, is what measures them.
* **Index.** The 0056 PLAN row's text said "one item for the owner"; it now says the owner ruled on it. The MADR
  row and the report row are not false and are not changed.
* **Commit.** Staged, not committed. The coordinator commits and pushes.

### 2026-11-15: phase 5a

Provenance: run by a mainstream model at medium effort, as the phase's row prices it, on the coordinator's brief.

* **Approval.** The owner, 2026-11-15, after phase 2 and the rulings landed: "Proceed to the next phase of the
  acp-go-sdk2 docs plan". By Timing's order (1, 2, 5a, 3, 6, 5b, 4a, 4b) that is phase 5a. This phase alone is approved
  to run; the plan pauses again when it is done.
* **Base.** `git rev-parse HEAD`: `2be7f24cfcd6080f72ef2e9f13de67e12812c4b7`, on `main`. `git status --short` was
  empty. `git describe --tags --abbrev=0`: `v1.36.0`, the newest tag, so this phase's section is for it.
  `git diff --stat v1.36.0..HEAD -- '*.go' Makefile go.mod` printed nothing: no `.go` file, the `Makefile` or `go.mod`
  changed since `v1.36.0`. The commits since the tag are `77a81cb`, `e67216c`, `632e097`, `f629f8f` and `2be7f24`
  (records and docs). No release step had started: no PLAN but this one is `in-progress`, and no tag is being prepared.
* **`CHANGELOG.md` (step 1; new).** Three parts and no more: `## Unreleased`, `## v1.36.0 — 2026-11-11`, and the
  pointer, set apart by a rule (`---`) and not a heading.
  * The date: `git for-each-ref --format='%(taggerdate:short)' refs/tags/v1.36.0` printed `2026-11-11`.
  * The summary is taken from the "Summary" of `docs/reports/0054-REPORT-release-notes-v1-36-0.md` and matches it:
    57 compatible additions and nothing incompatible against `v1.35.0`; the schema pin `1.25.0` to `1.31.0`; the client
    system prompt with its gates and its check; one change on the wire, limited to session requests that carry
    `systemPrompt`; the `go` line to `1.26.11`. The link is relative, to that report.
  * The pointer's three facts, re-checked on `2be7f24`:
    * `head -12 docs/reports/0019-REPORT-release-notes-v0-18-0.md` reads "Release notes for v1.14.0, against v0.17.1" and
      says it was "drafted for `v0.18.0`" and "keeps the file name it is cited by".
    * `go.mod:7` is `retract v1.13.0 // Published accidentally from an unfinished commit.`
    * `ls docs/reports | grep release-notes` lists 38 reports, the first being
      `0013-REPORT-release-notes-v0-15-0.md`; `git tag -l` shows `v0.14.0` and older with no report, `v0.15.0` the
      earliest with one.
  * `Unreleased`, three lines, one for each of phases 1, 2 and this one: the defect report (changes no doc and no code);
    the four corrected statements (W1, W4, W5, W6: `docs/architecture.md` for the terminal output's file name and the two
    test counts, `README.md` for `example/remote`'s capabilities); and this file, with `RELEASING.md`'s description of it
    and of the release tools. The third line follows MADR D2 ("an `Unreleased` section that each phase adds to, in the
    same commit as its change") and the owner's rule that a phase's docs step includes its line. The PLAN's step 1 names
    phases 1 and 2 only, so this line is a reading of D2, and it is the one line to drop if the owner reads the PLAN
    narrowly. The rulings entry (records alone) has no line.
* **`RELEASING.md` (step 2; the three places).**
  * "Release notes and the API": the sentence "The changelog is the release notes report" is replaced. `CHANGELOG.md` is
    the changelog; each change adds its line under `## Unreleased` in the same phase; the release's PLAN turns it into
    `## vX.Y.Z — YYYY-MM-DD` and links the report; releases before `v1.36.0` are not listed and the closing paragraph
    points to their reports. The report keeps its number, its index row and its "Release text", the release's body.
  * "Tag and Publish", step 3, and a new step 4. Step 3 gives the body's source, `python3 tools/release_body.py
    vX.Y.Z REV`. Each statement was read from the tool: it reads the report from git as at `REV` (`report_path`, `body`
    in `tools/release_body.py`), takes the text under `### vX.Y.Z` up to `## Sources not in the release text`, makes each
    relative link absolute at the tag, runs `idscan.py` on the body, and writes `release-body-vX.Y.Z.md` to the
    `--out DIR` directory, default `.cache/tools/release` (`--help`: "default .cache/tools/release"); it exits 1 on a
    missing link target or a scan match. Step 3's "named for the tag, neither a draft nor a prerelease, and the latest
    release" is what `publication.py`'s first checks require (`tools/publication.py`, the first three `check` calls).
    Step 4 lists, after the tag, `python3 tools/watch_ci.py REV`, `python3 tools/publication.py vX.Y.Z REV BODY` and
    `python3 tools/consumer.py vX.Y.Z`, run by hand. Checked against each `--help`: `watch_ci.py`'s `--branch` (all runs
    of the commit by default), 20 minutes for the first run, 30 seconds for others, exit 0, 1 and 2; `publication.py`'s
    three positional arguments, ten checks (counted: release, body, latest, tag, tag on main, CI on main, CI on tag,
    proxy `.info`, `go list -m @latest`, `go mod download`), exit 0 only when every check holds; `consumer.py`'s
    `VERSION`, `v1.36.0` or later, its fresh module at the `go` line's version, the proxy and the checksum database. The
    flags the text mentions are `--out` and `--branch`, each with its meaning and default. `consumer_probe.py` and
    0048 are not mentioned: the PLAN names three tools, and 0048 stays undescribed.
  * "Additional Notes": the same-commit rule gains a second bullet, MADR D5's docs step: the docs a code change makes
    false or newly needs, including `CHANGELOG.md`'s `Unreleased`, changed in the same commit and recorded.
* **`docs/architecture.md` (step 3).** The "Tree" gains the line `CHANGELOG.md` after `RELEASING.md`. `AGENTS.md` is
  not changed (step 4).
* **The one-off script (not committed).** A scratch script reads `git describe --tags --abbrev=0` and the tag's date,
  compares the `## ` headings with `Unreleased` and `<tag> — <date>`, and checks every relative link against
  `git ls-files`.
  * Planted break, on a scratch copy with the `v1.36.0` heading removed and the report link renamed: exit 1,
    `FINDING: version headings ['Unreleased'] != ['Unreleased', 'v1.36.0 — 2026-11-11']`,
    `FINDING: link target not a tracked file: docs/reports/0054-REPORT-nope.md`, `2 finding(s)`.
  * On the real file: exit 0, `0 finding(s); tag v1.36.0, tag date 2026-11-11`.
* **Docs step.** Searched `README.md`, `docs/architecture.md`, `tools/README.md`, `AGENTS.md`, `docs/README.md`,
  `RELEASING.md` and every `.go` file for "changelog" and "release notes". No statement says the repository has no
  changelog or says where the release notes live other than `RELEASING.md` (fixed) and `tools/README.md:38`
  ("a release's body, from its release-notes report", which stays true). `docs/README.md` indexes no file outside the
  Records table and its three links, and does not list `RELEASING.md`, so `CHANGELOG.md` gets no index row. Open for
  phase 5b: `docs/architecture.md:913`-`:919` and `AGENTS.md:96` and `:129` list the docs-safe and link-checked paths
  without `CHANGELOG.md`, which is right until 5b makes it docs-safe. No Go file is made false.
* **Verification.** Each output went to a scratch file and its exit status was taken before it was read.
  * `git diff --cached --numstat`: `CHANGELOG.md` 28/0, `RELEASING.md` 58/7, `docs/README.md` 1/1, `docs/architecture.md` 1/0, before this record; every hunk is described above.
  * `grep -n 'The changelog is the release notes report' RELEASING.md`: exit 1, no output.
  * `python3 -B scripts/check_records.py --check-all`: exit 0, no output.
  * `python3 -B scripts/check_disclosure.py`: exit 0, "840 tracked file(s), 0 finding(s)".
  * Offline `markdownlint-cli2` v0.23.2, the version the Makefile pins: exit 0, 57 files, 0 issues, `CHANGELOG.md`
    among them.
  * `run-heavy -- make pre-add-check FILES="CHANGELOG.md RELEASING.md docs/architecture.md"`: exit 0 (waited in the queue 846 s, ran 136 s, 12:41:12 to 12:43:28 local). `go-precheck`: 0 `.go` files to format; both modules linted, vetted and tested (`-race -shuffle=on`, uncached), tidy and `govulncheck`, all clean.
* **Not verified.** The release steps themselves were not run; the commands were checked against each tool's source and
  `--help` and not executed, since each needs a tag or a published release. `CHANGELOG.md`'s links are checked by the
  one-off script and not by `check_records.py` until phase 5b.
* **Files.** `CHANGELOG.md` (new); `RELEASING.md`; `docs/architecture.md` (the Tree line); `docs/README.md` (this
  PLAN's row text); this record. No file outside the phase's list changed, and no deviation was needed.
* **Commit.** Staged, not committed. The coordinator commits and pushes after the disclosure guard. The plan pauses
  until the owner says to continue.
