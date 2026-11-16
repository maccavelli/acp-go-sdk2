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

*Annotated 2026-11-15, in phase 5b:* from phase 5b's commit `CHANGELOG.md` is docs-safe, so it joins the list in the
sentence above and is no longer an exception to it. The sentences above are right for the phases before 5b, which is
how phases 5a and 6 ran. Phases 4a and 4b change `.go` files, so they run the Go checks whatever else they name.

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

     *Annotated 2026-11-15, by the coordinator's decision, reported to the owner (Execution record, "2026-11-15: the
     coordinator's decision on phase 3's audit"; it is not the owner's decision).* Phase 3's audit, run on `8f4cc9a`,
     found one flag short under this standard that the 15 rows above do not list. It joins by this amendment, as the
     sentence "a flag a later reading finds short is a new row by amendment" above provides. So there are 16 further
     rows, 31 flags for the standard (15 and 16), 46 flags with the report's 15, and 71 items for the audit to print
     on the tree.

     | Flag | Defined at | What the docs lack |
     | :--- | :--- | :--- |
     | `tools/remote.py --tree`, of `pkg` and of `test` | `tools/remote.py:270` (`pkg`, help "build from this tree instead of the repository"), `:277` (`test`, help "copy this tree instead of the repository"); neither has an argparse default, so the default is the repository | `tools/README.md:33` (the `pkg` row) does not show `--tree` at all; `:34` (the `test` row) shows `[--tree DIR]` in the synopsis cell only, and its sentence says "the tree copied to a test host" without naming the flag or its default; no other doc names `remote.py`'s `--tree` |

     The row is one item for the audit, since a reader sees one flag; phase 3 gives it its sentence in both rows.

     *Annotated 2026-11-15, by the owner's ruling "Name both in the next phase", relayed by the coordinator (Execution
     record, "2026-11-15: the owner's rulings after phase 3"). The two flags the phase-3 agent judged described on its
     own reading are short under this standard, and join by this amendment, as "a flag a later reading finds short is a
     new row by amendment" provides.*

     | Flag | Defined at | What the docs lack |
     | :--- | :--- | :--- |
     | `tools/consumer_probe.py --tree` | `tools/consumer_probe.py:64`, help "the tree the replace names, such as a planted break's copy (default: the repository)" | `tools/README.md:41` shows `[--tree DIR]` in the synopsis cell, and its sentence says the module requires the SDK "through a `replace` onto the repository, or onto `DIR`, such as a planted break's copy": what the flag is for and its default, without the flag's name |
     | `tools/remote.py --host`, of `tree`, `pkg` and `test` | `tools/remote.py:265`, `:268`, `:275`: `choices` the host names (`linux`, `windows`), `required=True`, no default | `tools/README.md:12`-`:13` write `remote.py --host linux` and `remote.py --host windows` in the Setting-up table's "For" cell, to say which variable names the host for each value, and the rows `:32`-`:34` show `--host H` in the synopsis cell; no sentence says that the flag is required, takes `linux` or `windows`, and has no default |

     So there are 18 further rows, 33 flags for the standard (15 and 18), 48 flags with the report's 15, and 73 items
     for the audit to print on the tree. Each `remote.py --host` is one item, as a flag two subcommands share counts
     once. Both are fixed in the next phase's commit, which is phase 6 by Timing's order (1, 2, 5a, 3, 6, 5b, 4a, 4b):
     `tools/README.md` joins phase 6's files by this amendment, beside `docs/README.md`. `ToolCallStream.WriteString`
     needs no row: `README.md:888`, in the "State" bullet (`:885`-`:889`), names it among the methods that return
     `acp.ErrToolCallStreamFinished` after `Finish`.

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

*Annotated 2026-11-15, in phase 5b, from the tree at `175f269`:* `docs/architecture.md`'s description of the lists is
at `:938`-`:946`, not `:913`-`:919`, and `AGENTS.md` describes them in two places, `:96`-`:97` (the link-checked docs) and
`:129`-`:130` (the docs-safe paths). Step 1 changed all of them.

*Amended 2026-11-15 by a deviation in phase 5b (Execution record, "2026-11-15: phase 5b", "Deviation"):*
`CHANGELOG.md` joins this phase's files, for one line in its `Unreleased` section, by `RELEASING.md`'s rule "Each
change adds its line under `## Unreleased` in the same phase as the change" and the owner's standing rule "Always in
the same commit", relayed by the coordinator. Nothing else of `CHANGELOG.md` changes.

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

*Amended 2026-11-15 by the owner's ruling "Name both in the next phase", relayed by the coordinator (Execution record,
"2026-11-15: the owner's rulings after phase 3"): `tools/README.md` joins this phase's files, for the two flag rows
that amendment adds under phase 3's 15 rows, `consumer_probe.py --tree` and `remote.py --host`. Nothing else of
`tools/README.md` changes in this phase.*

*Amended 2026-11-15 by a deviation in phase 6 (Execution record, "2026-11-15: phase 6", "Deviation"):
`CHANGELOG.md` joins this phase's files, for one line in its `Unreleased` section, by `RELEASING.md`'s rule "Each
change adds its line under `## Unreleased` in the same phase as the change" and the owner's standing rule "Always in
the same commit", relayed by the coordinator. Nothing else of `CHANGELOG.md` changes.*

*Amended 2026-11-15 by a second deviation in phase 5b (Execution record, "2026-11-15: phase 5b", "Deviation"):* step
2's "No other change to either script" gains one exception: the comment above `UNNUMBERED_DOCS` in
`scripts/check_records.py`, which lists the docs-safe files, names `CHANGELOG.md` with the others, by the owner's
standing rule "Always in the same commit", relayed by the coordinator. Nothing else of either script changes.

1. The Records table: every record file has a row, and every MADR and PLAN row's status equals its frontmatter
   `status`. This pair's rows and the 0056 report's row are brought up to date.
2. The matrix: one row per task a reader comes with, in the reader's words, each pointing to the current record or
   doc for it. The paragraph above it says that the Records table is the complete per-record index, and that a record
   which adds no new task adds no row. Every task the 31 MADRs of the MADR's S1 answer is covered by some row, which
   need not be one per MADR.

   *Annotated 2026-11-15, in phase 6, from the tree at `378779e`:* the 31 are the MADRs that had no row at `77a81cb`,
   out of the 47 the MADR's S1 counts there. The tree now holds 48 MADR files: those 47 and
   `0056-MADR-bring-the-docs-current-and-keep-them-current.md`, which the report's S1 adds as the 32nd without a row.
   Phase 6's record accounts for all 48.
3. `docs/README.md`'s first line ("Guides are not in the tree yet") stays, as no guide is created.

**Verification**

* A one-off script compares the record files under `docs/decisions/` and `docs/reports/` with the Records table's
  links, both ways, and compares each MADR and PLAN row's status with the file's frontmatter. It prints nothing and
  exits 0. It is first run on a scratch copy with one row removed and one status changed, and must report both.
* The execution record lists each task row and the record or doc it points to, and, for each of the 47 MADRs, the
  row that covers its task or the reason it adds none (superseded, or no task of its own). There is no count to
  compare.

  *Annotated 2026-11-15, in phase 6:* 47 is S1's count at `77a81cb`, before this pair. The tree at `378779e` holds 48
  MADR files, and the record accounts for each of the 48 ("2026-11-15: phase 6").
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

### 2026-11-15: the coordinator's decision on phase 3's audit

Provenance: written by a mainstream model at medium effort, on the coordinator's brief. This is the coordinator's
decision, made under the coordinator skill's rule that a conflict is resolved only when every assertion it rests on is
proven on evidence, and reported to the owner. It is not the owner's decision.

* **What was found.** Phase 3's inventory audit, run on `8f4cc9a` before any doc was written, printed 71 items. The
  phase's rows come to 70, and 70 of the 71 match exactly. The one extra is `tools/remote.py --tree`. The phase stopped
  at its step 3, as the Stop condition requires, with the tree clean and nothing staged.
* **The assertions and their evidence** (read in the tree at `8f4cc9a`):
  1. `tools/remote.py` defines `--tree` for `pkg` (`tools/remote.py:270`, help "build from this tree instead of the
     repository") and for `test` (`:277`, help "copy this tree instead of the repository"); neither has an argparse
     default, so the default is the repository.
  2. No doc describes it: `tools/README.md:33` (the `pkg` row) does not show `--tree` at all; `:34` (the `test` row)
     shows `[--tree DIR]` in the synopsis cell only, and its sentence says "the tree copied to a test host" without
     naming the flag or its default; a search of `README.md`, `docs/architecture.md`, `RELEASING.md`, `AGENTS.md` and
     `tools/README.md` finds no other mention of `remote.py`'s `--tree`.
  3. The owner's standard of 2026-11-15, "Meaning and default for each": "Every flag gets at least a sentence saying what
     it does, with its default, where its reader would look." Under it this flag is undescribed, by the same reading
     the PLAN already applied to `platforms.py --tree` and `plants.py --tree`.
  4. The PLAN provides for this (phase 3's amendment of 2026-11-15): "a flag a later reading finds short is a new row
     by amendment, not a silent addition."
* **Decision.** `tools/remote.py --tree` (`:270`, `:277`) joins phase 3's rows by the dated annotation under "The
  stricter standard adds 15 more rows": the further rows become 16, the flags 46, the phase's items 71, and the audit's
  list on the tree then equals the phase's rows. Nothing is judged "described by reading" to make the lists meet.
  `tools/README.md` gives the flag its sentence in both the `pkg` and the `test` rows. The MADR asserts no fact this
  changes: D1's standard stands as annotated, so the MADR is not amended.
* **Files.** This PLAN only. Phase 3 resumes at step 3 with the audit unchanged.

### 2026-11-15: phase 3

Provenance: run by a mainstream model at medium effort, as the phase's row prices it, on the coordinator's brief.

* **Approval.** The owner, 2026-11-15: "Resume the next phase of the acp-go-sdk2 docs plan, then pause". By Timing's
  order (1, 2, 5a, 3, 6, 5b, 4a, 4b) that is phase 3. This phase alone is approved to run; the plan pauses again when it
  is done.
* **Base.** `git rev-parse HEAD`: `8f4cc9a294d3292f0e4554761bd224224d534868`, on `main`; CI had passed on it.
  `git status --short` was empty. `git describe --tags --abbrev=0`: `v1.36.0`. `git diff --stat v1.36.0 HEAD -- '*.go'
  Makefile go.mod` printed nothing, so no `.go` file, the `Makefile` or `go.mod` has changed since `v1.36.0`, and the
  report's rows were re-checked against the same code. No release step had started: no PLAN but this one is
  `in-progress`, and no tag is being prepared.
* **A stop and a decision (steps 1 to 3).** The first run of the audit, on `8f4cc9a` before any doc was written,
  printed 71 items against the phase's 70 rows. The phase stopped at step 3, with the tree clean and nothing staged.
  The one extra was `tools/remote.py --tree`. The coordinator's decision, reported to the owner, is the entry above
  ("2026-11-15: the coordinator's decision on phase 3's audit"), and the annotation under the PLAN's "15 more rows". It
  joins the phase's rows, so the phase has 16 further rows and 71 items. The phase resumed at step 3 with the audit
  unchanged. This is a deviation, recorded there and not repeated here.
* **The audit (steps 1 and 2).** A scratch script, not committed, in the session's scratch folder (`acp-0056-p3/`),
  started from phase 1's. It rebuilds phase 1's inventory from the code: 21 packages, the commands, the flags, the
  environment and Make variables, `opencode.json`'s keys, the root's `With…` options and the option structs' fields, and
  the Make targets. It adds four classes the report found by reading (M12, M13, M4): the root's `New…` functions that
  return `*RequestError`, the root's exported `Err…` variables, `ToolCallStream`'s exported methods, and the
  hand-written `SetLogger` methods. A flag is one item per file and flag, so a flag two subcommands share
  (`remote.py`'s `--host`, `--tree` and `--out`, `ci_jobs.py`'s `--attempt`) counts once. The docs it searches are D1's:
  `README.md`, `docs/architecture.md`, `RELEASING.md`, `tools/README.md`, `AGENTS.md` and every package comment.
  * **The rule for a flag** (the standard of 2026-11-15: "a sentence and, where it has one, its default"). A flag is
    described when a block (a paragraph, a list item or a table row) of those docs that names its command has an
    occurrence of the flag that is: in prose, which is not in a fenced block, not in a table row's first cell (the
    synopsis), not inside a code span that is a command line, and not in a bare list (two or more flag tokens of the
    command in a run separated only by commas, "and", "or", `|`, brackets and one placeholder argument each, with no
    digit between them); in a sentence of at least 4 plain words (letters outside code spans), the next sentence counted
    too when its own has fewer; and, where the code gives the flag a default (a literal other than `""`, `false` or
    `None`, an expression, or a usage text that says "default"), followed, up to the next flag token of the command or
    the end of the block, by the word "default" or the default's literal as a whole token. `--out DIR` of the tools is
    taken as described by `tools/README.md:21`, which says it once for every tool. The other classes keep phase 1's rule:
    the name as written, in any of the docs.
  * **What the script cannot decide.** That a sentence says what the flag does, or says it truly. It prints the sentence
    it matched with `--all`, and I read all of them. Its matches can also be wrong in the other direction: for `-skip`
    of the mutation tool it matched the sentence about the `make` target's call, which names `-skip`, and not the
    sentence of its own, which is present (`docs/architecture.md:880`ff.).
  * **Settled by reading, not by the script** (`judged.json`, each with the doc line; the first six take the PLAN's own
    reading of the line, recorded in its list of flags that need no row):
    * `push_guard.py --remote` and `--branch`: `AGENTS.md:150`-`:151`, "`--remote` and `--branch` default to `origin`
      and `main`".
    * `check_ci_filters.py --docs-safe`: `docs/architecture.md:931`-`:932`, "as `scripts/check_ci_filters.py
      --docs-safe` reads CI's list".
    * `check_records.py --next`: `AGENTS.md:91`, "**Next number** comes from `python3 scripts/check_records.py --next`";
      `RELEASING.md:133`.
    * `ci_jobs.py --expect` and `--others`: `tools/README.md:44`, "a variable's value in each step's environment,
      checked with `--expect` and `--others`".
    * **Three on my own reading, which the PLAN's list does not name; each quoted so it can be checked.**
      * `tools/consumer_probe.py --tree`: `tools/README.md:41`, "requires the SDK through a `replace` onto the
        repository, or onto `DIR`, such as a planted break's copy". It says what the flag is for and its default, the
        repository, without the flag's name. It is the reading least firmly held: the row names the flag only in its
        synopsis cell.
        *Annotated 2026-11-15: the owner ruled "Name both in the next phase" (Execution record, "2026-11-15: the owner's
        rulings after phase 3"). It is a row by amendment, under the 15 rows, and is fixed in the next phase's commit.*
      * `tools/remote.py --host`: `tools/README.md:12`-`:13`, "`ACP_LINUX_HOST` | `remote.py --host linux`, the `linux`
        gate | an SSH host or alias with Go", and the same for `windows`. The flag is required and has no default; the
        lines say which host each value names.
        *Annotated 2026-11-15: the owner ruled "Name both in the next phase" (the same entry). It is a row by
        amendment, and is fixed in the next phase's commit.*
      * `ToolCallStream.Write` and `WriteString`: `README.md:857`, "The stream is an `io.Writer`: `cmd.Stdout = stream`,
        `io.Copy`, or `io.MultiWriter` with a buffer of the agent's own". `WriteString` is not named by any doc; I judged
        it covered as the same writer's string form (`toolstream.go:164`, "WriteString is Write for a string").
        *Annotated 2026-11-15: `README.md:888`, in the "State" bullet, names `WriteString` among the methods that return
        `acp.ErrToolCallStreamFinished` after `Finish`. The coordinator's brief and the owner's rulings add no row for it
        (Execution record, "2026-11-15: the owner's rulings after phase 3").*
  * **Planted breaks, seen to fail in this run before any result counted** (scratch clones; the first on the unedited
    tree, the rest on the final one):
    * the PLAN's plant, every `make api` and `` `api` `` removed from the docs and the Go package comments, before any
      edit: exit 1, `UNDESCRIBED target: api`, one more than the baseline's 71;
    * `idscan.py`'s sentence removed, leaving its flags in the synopsis only (before any edit, on the sentence that
      stood): exit 1, `UNDESCRIBED flag: tools/idscan.py --range`;
    * on the final tree, `plants_final.out`: control, no plant, exit 0, "254 items, 0 not described"; `make api`
      removed from the docs and package comments (4 files): exit 1, `UNDESCRIBED target: api`; `idscan.py`'s sentence
      about `--all`, `--paths` and `--range` removed: exit 1, those three flags named; `watch_ci.py`'s sentence about
      `--minutes` and `--more` replaced by "(flags: `--minutes`, `--more`)", a flag left named in a list with no
      sentence: exit 1, `UNDESCRIBED flag: tools/watch_ci.py --minutes` and `--more`; `example/gemini`'s five flags put
      back as "(flags: -model, -sandbox, -yolo, -debug, -gemini /path/to/gemini)", as `README.md` had them: exit 1, the
      five named.
  * **On the tree, before any edit:** exit 1, 71 undescribed, and the list equals the phase's 70 rows plus
    `tools/remote.py --tree`: the 7 targets, `GOLANGCI_LINT`, `MUTANTS`, `SEED`, `JOBS` and `GIT_BASH`, 46 flags (the
    report's 15, the PLAN's 15 and the 16th), and 13 name-level items. **On the tree after the edits:** exit 0,
    "254 items, 0 not described" (`final_tree.out`).
* **Step 4: each row, where it is now described, and the code behind it.** Line numbers are those of the staged files.
  * M1, the Make targets `all`, `vet`, `lint`, `tidy`, `vuln`, `bench` and `help`: `docs/architecture.md:966`-`:987`.
    Each says what it runs (`Makefile:69`, `:83`, `:88`, `:103`, `:107`, `:138`, `:163`) and whether CI runs it, read
    from `.github/workflows/ci.yml:112`-`:166`: CI runs `make api`, `make lint`, `make markdownlint`, `make
    check-records`, `make disclosure-check` and `make check-tools`, and runs `go vet`, `go mod tidy -diff` and
    govulncheck itself, not through `make vet`, `tidy` or `vuln`; `make all`, `help` and `bench` are not run.
    `make lint` needs the binary at `GOLANGCI_LINT` and prints its install command when it is missing
    (`Makefile:89`-`:94`); `make vuln` reads its pin from `ci.yml` and exits 1 without one (`Makefile:107`-`:112`), and
    fetches govulncheck with `go run`.
  * M2, the variables: `docs/architecture.md:989`-`:1001`. `GOLANGCI_LINT` (`Makefile:50`,
    `scripts/go-precheck.sh:102`, `tools/platforms.py:31`), each defaulting to `golangci-lint` in `$(go env GOPATH)/bin`;
    `MUTANTS`, `SEED` and `JOBS` (`Makefile:145`-`:148`, passed as `-n`, `-seed` and `-j`; defaults empty, 1, 1);
    `GIT_BASH` (`Makefile:4`-`:8`, Windows only, default `C:/PROGRA~1/Git/usr/bin/bash.exe`).
  * M3a, `cmd/generate -schema` (`main.go:41`) and the 16-row set's `-out` (`:42`): `docs/architecture.md:157`-`:164`.
    Both default from `findRepoRoot` (`main.go:197`-`:216`), the first directory upward that holds `schema/meta.json` and
    `schema/schema.json`; `-out` must exist (`main.go:25`-`:28`).
  * M3b, `example/claude-code -yolo` (`main.go:209`): `README.md:73`-`:77`; the selection is `RequestPermission`'s
    `autoApprove` branch (`main.go:49`-`:60`).
  * M3c, 13 flags: `tools/README.md` and `docs/architecture.md`. The scripts: `check_records.py --check` and
    `--check-all` (`:256`-`:257`, `docs/architecture.md:1005`ff.), `--root` of `check_ci_filters.py` (`:339`) and
    `check_disclosure.py` (`:100`), `push_guard.py --guard` (`:40`), all in `docs/architecture.md:1003`-`:1016`. The tools:
    `ci_jobs.py --runs`, `--created`, `--markdown`, `--attempt` (`tools/ci_jobs.py:278`-`:288`, row `tools/README.md:44`),
    `leftovers.py --name` (`:39`, row `:35`), `plants.py --keep` (`:136`, row `:37`), `remote.py --goarch` (`:269`,
    row `:33`), `watch_ci.py --minutes` and `--more` (`:86`-`:87`, row `:43`).
  * The mutation tool's 10 flags (`mutate/main.go:65`-`:74`): `docs/architecture.md:880`-`:892`, each with its default;
    `make mutation-engine` and `make mutation-generated` pass `-dir`, `-skip`, `-pkgs` and `-files` as `Makefile:151`,
    `:154` show.
  * `example/gemini`'s 5 flags (`main.go:209`-`:213`): `README.md:82`-`:89`; they become the arguments of the Gemini
    command (`main.go:216`-`:224`), and `-yolo` is the client's `autoApprove`.
  * The 16 further rows: `cmd/generate -out` as above; `example/remote serve -addr` (`main.go:63`, default
    `127.0.0.1:7331`), `README.md:91`-`:93`; `check_records.py --check-all` as above; `ci_jobs.py tally --branch`
    (`:277`, default `main`), `tools/README.md:44`; `consumer_probe.py --run` (`:65`), `:41`; `gates.py --all` (`:147`,
    `selected` at `tools/gates.py:134`-`:142`), `:29`; `idscan.py --all` and `--paths` (`:90`, `:92`; the default mode is
    the files changed since `HEAD` and the untracked ones, `:125`), `:36`; `interop.py --ts`, `--py` and `--tree`
    (`:265`-`:268`, defaults `TS_REV`, `PY_REV` and the repository), `:42`; `platforms.py --tree` (`:47`), `:31`;
    `plants.py --tree` (`:135`), `:37`; `remote.py test --module` (`:276`, default `.`) and `--timeout` (`:278`, default
    3600), `:34`; and the 16th, `remote.py --tree` (`:270`, `:277`), in the `pkg` row `:33` and the `test` row `:34`.
  * M4, `WithLogger` and `Connection.SetLogger` (`connection.go:906`-`:913`, `:1440`-`:1444`, `:1094`-`:1096`,
    `:1446`-`:1451`; the generated `SetLogger`s, `agent_gen.go:89`, `client_gen.go:83`): `README.md:1167`-`:1177`. Every
    diagnostic goes through `loggerOrDefault` (the only `slog.Default()` in `connection.go` is `:1450`).
  * M9, `opencode.json`: `docs/architecture.md:1535`, one line in the Tree. It states the key and its value
    (`opencode.json:3`, `"instructions": [".opencode/rules.md"]`) and nothing of how the harness reads it, which the
    repository does not show.
  * M10, `WithNotificationDropHandler` (`connection.go:892`-`:899`, `:1428`-`:1432`): `README.md:1125`-`:1130`, beside
    `OverflowDropNewest`.
  * M11, `Command.ConnectionOptions` and `InProcess.ConnectionOptions` (`agentproc/agentproc.go:52`, `:251`;
    `agentproc/inprocess.go:54`-`:55`, `:89`): `README.md:998`-`:1002`, beside `agentproc`.
  * M12, the five error constructors (`errors.go:310`-`:342`): `README.md:1592`-`:1603`, which lists all eight with their
    codes and messages, the three already named included.
  * M13, `ToolCallStream.Entity` (`toolstream.go:216`-`:221`) and `ErrToolCallStreamFinished` (`errors.go:227`-`:231`):
    `README.md:885`-`:889`. The methods that return it after `Finish` are `Write`, `WriteString`, `SetStatus`, `Update`,
    `Flush` and a second `Finish` (`toolstream.go:152`, `:274`, `:302`, `:379`).
  * The Tree's `Makefile` line (`docs/architecture.md:1493`) now lists the targets this phase describes.
* **Step 5.** `CHANGELOG.md`'s `Unreleased` gains one line for this phase.
* **Counts.** `docs/architecture.md`: 12 new list entries (6 targets, 3 variables and 3 script entries), two new
  passages for the generator's and the mutation tool's flags (the latter 10 flags, one sentence each), and one Tree
  line (`opencode.json`), with the Tree's `Makefile` line extended. `tools/README.md`: 11 rows changed (29, 31, 33, 34, 35, 36, 37, 41, 42, 43, 44).
* **Docs step.** Searched `README.md`, `docs/architecture.md`, `tools/README.md`, `RELEASING.md`, `AGENTS.md`,
  `docs/README.md` and the package comments for a list of Make targets, variables, flags or tools that claims to be
  complete, and for a count. Fixed: the Tree's `Makefile` line (`docs/architecture.md:1493`), which listed seven target
  groups and left out `vet`, `tidy`, `vuln`, `bench` and `help`. Nothing else was made false or incomplete, and no
  Go-file row is opened: `mutate/main.go`'s package comment lists the flags in its usage line and says nothing the new
  text contradicts. `AGENTS.md` is not changed.

  *Added 2026-11-15, after the coordinator's review of the staged phase.* The review found the Tree's `Makefile` line
  (`docs/architecture.md:1493`), as extended above, still short: it read as the list of targets, and `make help`
  prints two it did not name, `all` (`Makefile:69`) and `clean` (`Makefile:219`). Comparing `make help`'s 22 targets
  with the line, counting `docs gates` as `check-records`, `markdownlint` and `disclosure-check` and `mutation` as
  `mutation-engine` and `mutation-generated`, those two were the whole difference. The line now reads "Makefile
  all, version, release, clean, test, fmt, vet, lint, tidy, vuln, api, fuzz, bench, mutation, pre-add-check,
  push-guard, docs gates, check-tools, help". Only Markdown changed, so no heavy run was repeated; the earlier
  `pre-add-check` run covered `README.md`, which this addition does not touch. The `docs/README.md` row needs no change.
* **Verification.** Each output went to a scratch file and its exit status was taken before it was read.
  * The audit on the tree: exit 0, "254 items, 0 not described".
  * `python3 -B scripts/check_records.py --check-all`: exit 0, no output.
  * `python3 -B scripts/check_disclosure.py`: exit 0, "840 tracked file(s), 0 finding(s)".
  * Offline `markdownlint-cli2` v0.23.2, the version the Makefile pins: exit 0, 57 files, 0 issues.
  * `run-heavy -- make pre-add-check FILES="…"`: exit 0, `go-precheck`: 0 `.go` files to format; both modules linted, vetted and tested (`-race -shuffle=on`, uncached), tidy and govulncheck, clean. It waited 0 s in the queue and ran 115 s, 13:18:57 to 13:20:51 local, started once the four docs were final.
* **Not verified.** `opencode.json`'s semantics for the harness, which the repository does not hold. That the tools'
  and scripts' behaviour matches each new sentence was read from their source and not run: the tools need hosts, `gh`
  or a network. `make vuln` and `make lint` were not run. Whether CI's three direct `go` commands equal `make vet`,
  `tidy` and `vuln` was read, not run. The audit decides the presence of a sentence and a default, not their truth.
* **Files.** `README.md`; `docs/architecture.md`; `tools/README.md`; `CHANGELOG.md`; `docs/README.md` (this PLAN's row
  text); this PLAN, its annotation under the 15 rows, and this record. No `.go` file, workflow, script or `Makefile`
  changed.
* **Commit.** Staged, not committed. The coordinator commits and pushes after the disclosure guard. The plan pauses
  until the owner says to continue.

### 2026-11-15: the owner's rulings after phase 3

Provenance: written by a mainstream model at medium effort, on the coordinator's brief. These are the owner's rulings,
relayed by the coordinator. They are not the coordinator's decision and not the author's.

* **The two flags judged described on the phase-3 agent's own reading.** The owner chose "Name both in the next
  phase": `tools/consumer_probe.py --tree` (`tools/README.md:41`) and `tools/remote.py --host`
  (`tools/README.md:12`-`:13`).
  * Both are rows by amendment, under phase 3's 15 rows ("a flag a later reading finds short is a new row by
    amendment"). The further rows become 18, the flags for the standard 33 (15 and 18), the flags with the report's 15
    become 48, and the items the audit prints on the tree become 73 (71 and the two).
  * Both are fixed in the next phase's commit, phase 6 by Timing's order (1, 2, 5a, 3, 6, 5b, 4a, 4b). `tools/README.md`
    joins phase 6's files by the amendment under phase 3's rows and the note under phase 6's **Files**.
  * The "on my own reading" items of the phase-3 entry now point here: `consumer_probe.py --tree` and `remote.py
    --host` are annotated as rows; `ToolCallStream.Write` and `WriteString` is annotated with the line that names
    `WriteString`.
  * `ToolCallStream.WriteString` needs no row. Verified in the tree at `55be71e`: `README.md:888`, in the "State"
    bullet (`:885`-`:889`), reads "`Write`, `WriteString`, `SetStatus`, `Update`, `Flush` and a second `Finish` return
    `acp.ErrToolCallStreamFinished`".
* **The tools' sentences that were read and not run.** The owner chose "Run the local and read-only ones". The run, and
  what it found, are in the next entry.
* **`CHANGELOG.md`'s `Unreleased` line "Added this file. …".** The owner chose "Keep it as it is". No edit to
  `CHANGELOG.md`.
* **Files.** This PLAN only (and `docs/README.md`'s row text for it). No source, doc, workflow, script or `Makefile`
  file changed.

### 2026-11-15: phase 3's sentences, run

Provenance: run by a mainstream model at medium effort on the coordinator's brief.

* **Why.** Phase 3's "Not verified" said the tools' and scripts' behaviour matched each new sentence by reading, and that
  `make vuln`, `make lint` and CI's three direct `go` commands were not run. The owner ruled "Run the local and
  read-only ones" (the entry above). This is that run, on `55be71e` (`main`, CI green).
* **How.** Everything that writes ran in a scratch clone of `55be71e` or with its output in a scratch folder
  (`<scratch>`, outside the tree); read-only `gh` calls ran in the repository. The clone's `origin` was set to the
  repository's GitHub URL so `watch_ci.py` and `ci_jobs.py` read the same repository. Heavy runs went through
  `run-heavy`, one at a time, and each waited 0 s. Each command's output went to a scratch file and its exit status was
  taken before the file was read. After the run `git status --short` in the repository printed nothing.
* **Kinds of evidence.** "Run" is the command, here, against the tree or the clone. "Offline test" is an existing
  repository test run unchanged. "Read" is source read and not run, the owner's ruling for the items named.
* **Results.** `<clone>` is the scratch clone; `<repo>` the repository. `go run ./internal/mutate` is run in
  `<clone>/cmd/generate`.

| Item | Command | Exit | Observed | Verdict |
| :--- | :--- | :--- | :--- | :--- |
| `idscan.py`, no flag | `idscan.py` in `<clone>` with `README.md` edited and one untracked file | 0 | "2 file(s), 21 pattern(s), 0 match(es)": the changed and the untracked file | `tools/README.md:36` holds |
| `idscan.py --all` | `idscan.py --all` | 0 | "840 file(s)", equal to `git ls-files` | holds |
| `idscan.py --paths` | `idscan.py --paths README.md CHANGELOG.md` | 0 | "2 file(s)" | holds |
| `idscan.py --range` | `idscan.py --range HEAD~3..HEAD`; `--range HEAD` | 0; 2 | "3 commit(s)"; "--range takes A..B, two dots" | holds |
| `ci_jobs.py tally` defaults | `ci_jobs.py tally` in `<repo>` | 0 | "40 ci.yml run(s) of main", first attempts counted by job and conclusion | `--branch` default `main` and `--runs` default 40 hold |
| `tally --runs`, `--markdown` | `tally --runs 3 --markdown` | 0 | 3 runs; the groups printed again as a table | holds |
| `tally --branch` | `tally --branch <no such branch> --runs 3` | 1 | "no completed run of" that branch | holds |
| `tally --created` | `tally --runs 100 --created '<2026-11-01T00:00:00Z'` | 0 | 66 runs, the newest created 2026-10-30; without the flag the newest was today's | holds (passed to `gh run list --created` as given; unset by default) |
| `ci_jobs.py failure RUN`, `--attempt` | `failure <run of 2 attempts>`; `--attempt 1`; `--attempt 2` (each `--out <scratch>`) | 0; 0; 0 | no flag: "attempt 1 of 2: 1 failed job(s)" then "attempt 2 of 2: no failed job"; `--attempt 1` the first alone; `--attempt 2` the second alone | holds |
| `ci_jobs.py env RUN VAR`, `--attempt` | `env <run of 2 attempts> GOTRACEBACK`; with `--attempt 1` | 0; 0 | no flag: "attempt 2: 6 job(s)", the latest; `--attempt 1`: attempt 1 | holds |
| `env --expect`, `--others` | `env <run of 55be71e> GOTRACEBACK --expect '<windows floor job>=crash' --others ''`; the same with `--others x` | 0; 1 | "holds"; five "does not hold" lines and "does NOT hold" | holds |
| `watch_ci.py --minutes` | `watch_ci.py HEAD --minutes 0.1` in `<clone>`, `HEAD` a new local commit with no run | 2 | after 11 s: "no CI run for 06e9859 within 0.1 minutes" | holds (default 20 is read from `argparse`, not waited out) |
| `watch_ci.py --more` | `watch_ci.py 55be71e --more 1 --settle 20`; the same without `--more` | 0; 0 | the same 2 runs and 7 jobs each time; 6 s with `--more 1`, 34 s by default | holds (the 30 s wait is the difference) |
| `consumer_probe.py --run` | `consumer_probe.py --run '^TestPresentNotice$'` in `<clone>` | 0 | `go test` ran that test alone; "as required" | holds |
| `consumer_probe.py`, no `--run` | `consumer_probe.py --tree <tree copy>` | 0 | all 4 tests ran ("every test runs by default") | holds |
| `consumer_probe.py --tree` | the same; and `--tree <folder with no go.mod>` | 0; 2 | the probe `go.mod`'s `replace` names the clone with no flag and the tree copy with `--tree`; "no go.mod in" the empty folder | holds (the row to be added by the ruling above) |
| `plants.py`, no `--tree` | `plants.py <spec: an inverted nil check> --out <scratch>` in `<clone>` | 0 | "FAILED AS REQUIRED", "1/1 as required" | holds |
| `plants.py --tree` | a spec whose edit's `old` text exists only in a tree copy: no `--tree`; then `--tree <copy>` | 1; 0 | no flag: "refused: ... occurs 0 time(s)"; `--tree`: "FAILED AS REQUIRED" | holds (the default is the repository) |
| `plants.py --keep` | the same with `--keep`; the work folder listed after each run | 0 | without it the work folder was empty after the run; with it one run folder stayed | holds |
| `platforms.py` | `platforms.py --out <scratch>` in `<clone>` | 0 | "13 of 13 passed": `golangci-lint` for linux and freebsd, `go vet` for 11 platforms | holds |
| `platforms.py --tree` | `platforms.py --tree <copy with a syntax error in agentproc>` | 1 | "0 of 13 passed"; the vets name the broken file | holds (the default checks the clone) |
| `gates.py` and `--all` | offline test `GatesTest.test_selection` of `tools/test_tools.py`, and `selected([], False)` against `selected([], True)` | 0; 0 | the test passed; without `--all` 16 gates, with it 18; the two added are `windows` and `linux`, the hosts' gates | holds, by an offline test and by a call, with no host |
| `check_records.py`, no flag | `check_records.py` | 2 | usage printed | holds |
| `check_records.py --check`, `--check-all` | both; then with a broken relative link added to `tools/README.md` in `<clone>`; then to a record | 0, 0; 0 and 1; 1 | `--check` passed the broken link in an unnumbered doc and `--check-all` named it ("broken relative link: tools/README.md"); `--check` named the same break in a record | holds |
| `check_records.py --next` | `check_records.py --next` | 0 | "0057", one past the highest number in the tree (0056) | holds |
| `check_ci_filters.py` | no flag; `--root <clone>`; `--root <copy with one ignored path removed from the push list>`; no flag run from that copy | 0; 0; 1; 1 | "0 problem(s)"; the same; "2 problem(s)"; the same 2 (the default is the script's own repository) | holds |
| `check_ci_filters.py --docs-safe` | `--docs-safe docs/architecture.md tools/README.md AGENTS.md RELEASING.md`; with `main.go`; with `README.md`; with none | 0; 1; 1; 1 | silent; "not docs-safe: main.go"; "not docs-safe: README.md" (CI does not ignore it; `docs_test.go` reads it); "no path named" | holds |
| `check_disclosure.py --root` | no flag; `--root <scratch repo with a home path in a tracked file>` | 0; 1 | "840 tracked file(s), 0 finding(s)"; "a.md:1: macos-home", "1 tracked file(s), 1 finding(s)" | holds |
| `push_guard.py --guard` | `--guard <missing file>`; `--guard <stub that prints its arguments>` | 1; 0 | "BLOCKED: the guard ... is missing; nothing was checked" on stderr, exit 1; the stub was run as `pre-push <remote> <url>` with the branch's refs on stdin | holds (exit 1 when missing) |
| `push_guard.py --remote`, `--branch` | no flag; `--remote <other remote> --branch <other branch>`; `--branch <no such>` | 0; 0; 1 | no flag: "main to origin", the stub's stdin names `refs/heads/main`; the others: "other to up", `refs/heads/other`; the missing branch exits 1 with git's "Needed a single revision". The default guard in the home directory also ran over `main` and passed with 0 outgoing commits | holds (defaults `origin` and `main`) |
| `make vet` | `make vet` in `<clone>` | 0 | runs `go vet ./...` in the root and in `cmd/generate`; both clean | `docs/architecture.md` holds |
| `make lint` | `make lint` | 0 | golangci-lint 2.14.0, "0 issues." for the host and for windows, in both modules (4 runs) | holds |
| `make lint`, missing binary | `make lint GOLANGCI_LINT=<missing path>` | 2 | "golangci-lint not found at ...", then "Install: go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0"; the recipe's `exit 1`, which `make` reports as "Error 1" and its own exit 2 | holds (the sentence does not name an exit status) |
| `make vuln` | `make vuln` | 0 | `go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...` in both modules, "No vulnerabilities found." twice | holds |
| `make vuln` without a pin | `make vuln` in a clone whose `ci.yml` has no `govulncheck@` | 2 | "no govulncheck pin in .github/workflows/ci.yml"; recipe exit 1, `make` 2 | holds ("refuses to run without") |
| `make tidy` | `make tidy`; then an unused `require` added to `go.mod` and `go mod tidy -diff`, then `make tidy`, in `<clone>` | 0; 1; 0 | `go mod tidy` in both modules; on the dirty `go.mod`, `-diff` printed the diff and left the file as it was, and `make tidy` removed the line | holds |
| `make bench` | `make bench` | 0 | `go test -run '^$' -bench . -benchmem -count=6 .`: 66 result lines, each with `allocs/op`, each of 11 benchmarks six times; 88 s | holds |
| `make all` | `make all`, `make help` in `<repo>` | 0; 0 | identical output of 25 lines | holds |
| `MUTANTS`, `SEED`, `JOBS` | `make -n mutation-engine`; with `MUTANTS=1 SEED=7 JOBS=2`; then that run | 0; 0; 0 | defaults print `-seed 1 -j 1` and no `-n`; the variables print `-seed 7 -j 2 -n 1`; the run ran "1 of 2662 sites", and the report's `seed` is 7 | holds |
| `cmd/generate -schema`, no flag | `generate` built in `<clone>`: run in `<clone>/agentproc`, with `helpers_gen.go` removed; run in a folder with no schema above it | 0; 1 | the file was written back at the repository root, `git status` clean; "could not locate repository root from ...; ensure schema files exist" | holds |
| `cmd/generate -schema` | `-schema <folder with no meta.json>`; `-schema <clone>/schema -out <tree copy>` run from another folder | 1; 0 | "read meta.json: open ...: no such file"; the files were written | holds |
| `cmd/generate -out` | `-out <missing folder>`; `-out <empty folder>`; `-out <full tree copy with its generated files removed>` | 1; 1; 0 | "output directory ...: does not exist"; "generated code does not type-check with ...": `undefined: Connection`; the five `*_gen.go` files written, `types_gen.go` equal to the repository's | holds ("must exist and hold the hand-written package it type-checks them with") |
| the mutation tool's 10 flags | `go run ./internal/mutate` with each flag, `-h`, in `<clone>/cmd/generate`, and `make mutation-engine MUTANTS=1 SEED=7 JOBS=2` | 0 | `-dir`: report `dir` "." by default, `../..` given. `-files`: 34 sites of `internal/util/*.go`; default files with `-pkgs ./internal/util`: the same 34. `-pkgs`: report `pkgs` "./..." by default. `-skip`: with `-skip .` an `agentproc` mutant that was killed survived. (`internal/util` has no tests, so its mutants all survive.) `-only`: the one site ran, though `-n 3` was given. `-n`: 1 of 34 sites; without it all 34 (0 runs "every one"). `-j`: 6 sites with `-j 3`. `-seed`: 1 and 2 chose different sites, 1 twice the same. `-timeout`: `bogus` exit 1 "invalid duration"; `1ms` on an `agentproc` site made it "timeout"; `-h` prints the default `3m`. `-o`: written to the file given, and to a new file in the temporary directory without it. Exit 1 for `-only` with no such site | each sentence in `docs/architecture.md:880`ff. holds, 10 of 10 |
| `example/remote serve -addr` | the built `remote serve` with no flag, `-addr 127.0.0.1:47331`, `-addr 127.0.0.1:0`; a second default run while one was up | 0; 0; 0; 1 | "serving ACP at http://127.0.0.1:7331/acp", then the port given, then a free port; the second default: "listen tcp 127.0.0.1:7331: bind: address already in use" | holds (default `127.0.0.1:7331`) |
| `remote.py` flags, offline | `remote.py pkg --help`, `test --help`, `leftovers.py --help`, `interop.py --help`, `gates.py --help` (no host reached) | 0 | `--host` required with choices `linux` and `windows`; `--goarch` default `amd64`; `--module` default `.`; `--timeout` default 3600; `--tree` help "build from this tree instead of the repository" and "copy this tree ..."; `leftovers.py --name` "another image name to match exactly"; `interop.py --ts`, `--py`, `--tree` as described | read from source and `--help`, not run: the commands need hosts (owner's ruling); `remote.py --host` is the row added above |
| offline tests of the out-of-scope tools | `python3 -B -m unittest discover -s tools -p 'test_*.py'` and `-s scripts` | 0; 0 | 93 and 41 tests passed, among them `RemoteRunTest.test_module_dir` (`--module`, its refusals), `test_timeout` (`--timeout`, exit 124), `test_tree_files_without_git` and `test_tree_left_behind` (`--tree`), `test_race_without_cgo_refused`; `LeftoversTest.test_exact_image_names` (`--name`, compared whole) | offline tests, which cover `--module`, `--timeout`, `--tree` and `--name` of `remote.py` and `leftovers.py`; `--goarch`, `--host`, `interop.py --ts` and `--py` have none |

* **CI's three direct commands against the Makefile's recipes** (`.github/workflows/ci.yml`'s steps "govulncheck"
  (`:139`) and "vet, gofmt, tidy, lint" (`:145`); the Makefile at `55be71e`, with `make -n` for the expanded lines):
  * `go vet`: the same two commands, `go vet ./...` and `( cd cmd/generate && go vet ./... )`. CI runs them in the
    "vet, gofmt, tidy, lint" step, `if: runner.os == 'Linux' && !matrix.floor`; `docs/architecture.md` says "on Linux
    but not at the floor Go, and not through `make`", which holds.
  * `go mod tidy -diff` against `make tidy`: different. CI runs `go mod tidy -diff` in the root and in `cmd/generate`,
    which writes nothing and fails on a difference, in the same step. `make tidy` runs `go mod tidy`, which rewrites.
    The sentence says so.
  * govulncheck against `make vuln`: the same command lines, `go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...` in
    the root and in `cmd/generate`. `make` takes the version from `ci.yml` (`grep` of the `govulncheck@v` token) and
    stops without one; CI writes `@v1.9.0` itself. CI's step is `if: runner.os == 'Linux'`, floor jobs included;
    the sentence says "on Linux at that version itself", which holds.
  * `make lint`: CI calls `make lint` itself, in the same step, with `GOLANGCI_LINT` set to the binary it installed;
    the sentence says so.
* **Out of scope, read from source and not run, by the owner's ruling:** `remote.py` (all subcommands: they need an
  SSH host), `leftovers.py` (the Windows host), `interop.py` (the hosts' `node`, `npm`, `npx`, `uv` and `localhost:8000`),
  `gates.py`'s `windows` and `linux` gates (hosts), and `example/gemini`'s and `example/claude-code`'s flags (they need
  the Gemini CLI and the Claude Code adapter). `--help` of the first three and of `gates.py` ran, as above, and reached
  no host. No offline test covers the two examples' flags: their `main_test.go` files test the file write, the terminal
  methods and the tool-call line.
* **Sentences found false: none.** No doc, source, workflow, script or `Makefile` was edited in this run.
* **Observations that are not a doc defect.** `ci_jobs.py failure` and `env` read a job's log from GitHub, which
  expires: a run of 2026-10-29 answered "HTTP 410" and the tool exited 1 naming the job; runs of 2026-11-09 and later
  read. `make` reports a failed recipe as its own exit 2, whatever status the recipe's `exit 1` had.
* **Not verified.** That the tools' `--help` and sentences agree on a host: `remote.py`, `leftovers.py`, `interop.py`
  and the hosts' gates were not run. `watch_ci.py`'s 20-minute default and `--settle` and `--every` were not waited
  out; `ci_jobs.py`'s `--created` was run with one value.
* **Files.** This PLAN only. No `.go` file, workflow, script, `Makefile` or other doc changed.

### 2026-11-15: phase 6

Provenance: run by a strongest-tier model at high effort, as the phase's row prices it, on the coordinator's brief.
Line numbers in this entry are those of the staged files.

* **Approval.** The owner, 2026-11-15: "Docs phase 6 then pause". By Timing's order (1, 2, 5a, 3, 6, 5b, 4a, 4b) that is
  phase 6. This phase alone is approved to run. Phases 5b, 4a and 4b are not, and the plan pauses again when it is done.
* **Base.** `git rev-parse HEAD`: `378779ec5fc937ac87d6718289d268e7c883a5ce`, on `main`. `git status --short` was
  empty. `git describe --tags --abbrev=0`: `v1.36.0`. `git diff --stat v1.36.0 HEAD -- '*.go' Makefile go.mod` printed
  nothing. No release step had started: no PLAN but this one is `in-progress` in `docs/README.md`, and no tag is being
  prepared. `gh run list --commit 378779e` listed one run, the Docs workflow, `success`; the commit changed records
  alone, so CI's Go workflow does not run on it.
* **Step 1: the Records table.** A one-off script, not committed (`<scratch>/compare_index.py`), lists the record files
  under `docs/decisions/` and `docs/reports/`, reads the Records table's rows, and reports a file with no row, a row
  whose link names no file, a duplicate row, a row whose Number or Kind cell does not match its file name, and a MADR or
  PLAN row whose Status differs from the file's frontmatter `status`. Before any edit it printed nothing and exited 0:
  162 files, 162 rows. No row was added or removed and no status changed. The three 0056 rows were brought up to date:
  the MADR's (its two annotations of 2026-11-15, D1's standard for a flag and question 5's sentence for `Negotiation`),
  this PLAN's (phase 6 run, the plan paused again) and the report's (its stale rows S1 and S4 fixed by this phase).
* **Step 2: the matrix.** Its 40 rows are replaced by 44, and a paragraph above them says that each row is a task a
  reader comes with, pointing to the doc or record that answers it now, that the Records table is the complete
  per-record index, superseded and historical records included, and that a record which adds no new task adds no row.
  * **How a row was chosen.** A row is a task in the reader's words. It points to the doc that answers the task now
    (a `README.md` section, `architecture.md`, `AGENTS.md`, `RELEASING.md`, `tools/README.md` or `CHANGELOG.md`) when
    one does, and to the current record when the task is to know why and no doc answers it. Where a `README.md`
    section names the records behind its rules, as "Wire encoding" does (`README.md:532`-`:539`), the row points to the
    section, and the records are one link further. A row to a record names the record that decides the matter now,
    never one a later record superseded. Rows run from an SDK user's tasks, through the release history and the
    protocol audits and the reasons for the module's shape, to work on the repository.
  * **Rows a reader might dispute, and why they are so.**
    * Three rows point to `README.md`'s "Closing and limits": closing a connection, the agent process, and bounding what
      a connection holds. The section holds all three, with no heading of its own for any but "The byte budget". They
      are three tasks a reader comes with, and one place answers them.
    * "know what the SDK sends and accepts on the wire" pointed to `architecture.md`. It now points to `README.md`'s
      "Wire encoding", the user's description of the wire, which names the six records behind it. `architecture.md`
      stays the target of "understand how this SDK fits together" and of "change the code generator".
    * The old rows that pointed to a PLAN were tasks to carry out a change that has been made: rename the module, emit
      with go/ast, apply the separation, apply the Go 1.27.2 and schema 1.25.0 pin, fix the generated output, install
      the fleet workspace, and the five 0011 PLANs. No reader comes with them now. Each PLAN stays in the Records table.
      The subjects of the 0011 PLANs are described as they are now in `architecture.md`'s "What it is" (the generator
      `:152`, its output check `:183`, the protocol table `:206`), the target of "change the code generator".
    * The report's S4 rows, three tasks that are history phrased as current: "know why the next ACP feature is the
      remote transport" became "connect to or serve an agent over HTTP or WebSocket", to `README.md`'s "Remote agents",
      which links `0043-MADR-remote-http-and-websocket-transport.md` (`README.md:1383`). "see whether a Go 1.27.1 floor
      fits this SDK" and "apply the Go 1.27.2 and schema 1.25.0 pin" became "know why the SDK needs Go 1.26.11 and is
      built with go1.27.4", to `0055-MADR-pin-go-1-27-4-and-floor-1-26-11.md`, and "install the SDK, and know which Go
      version it needs", to `README.md`'s "Installation" (`README.md:20`-`:21`).
    * The audits: `0053-REPORT-acp-1-25-0-completeness-parity-and-adherence.md` is the target of "see how completely the
      SDK meets the ACP spec, as last audited". Its opening says it re-scores
      `0016-REPORT-acp-v1-conformance-audit.md` against the tree at `v1.35.0`, so 0016 and
      `0001-REPORT-go-127-acp-parity.md`'s "how the generated API lines up with the current ACP spec" are no longer
      the current answer. `0051-REPORT-acp-1-25-0-featureset-coverage-and-three-extensions.md` stays, as "see what the
      SDK could add beyond the spec, as last assessed": its first item shipped under
      `0052-MADR-agent-tool-call-snapshot-stream-and-message-size-default.md`, and 0053 does not reopen the other two.
      "as last audited" and "as last assessed" say that each is a dated observation.
    * "read a crash on CI's Windows floor job" points to `0033-MADR-traceback-on-windows-floor-ci.md`, a record and not
      a doc: no doc says what that job prints on a crash, or why.
    * "know why this repository has the fleet workspace and library CI" is kept, to
      `0003-MADR-install-fleet-agent-workspace.md`: no doc gives that reason.
    * "find a record by its number, or a release's notes" points to the Records table in the same file, so that a
      reader who comes with a number, or a version, is sent to the index.
  * **The old matrix's 40 rows, and where each one's information is now.**

    | Old row | Old target | Now |
    | :--- | :--- | :--- |
    | know how a connection can slow its peer … | `0044-MADR-…` | "bound what a connection holds when its peer sends faster than it is handled", `README.md`'s "Closing and limits", which links 0044 (`README.md:1115`) |
    | know why the next ACP feature is the remote transport … | `0043-MADR-…` | S4: "connect to or serve an agent over HTTP or WebSocket", `README.md`'s "Remote agents", which links 0043 (`:1383`) |
    | know what the two consumers still wait on … | `0041-MADR-…` | history: its PLAN is `complete`. "build an agent or a client …", `README.md`'s "Explore the API", which links 0041 (`:177`); the Records table |
    | find, for a consumer's code, the SDK API that replaces it … | `0041-REPORT-consumer-adoption-…` | kept, worded "a consumer's own ACP code" |
    | know how the SDK checks an elicitation form … | `0040-MADR-…` | "ask the user for input, and check a form, its answer or a URL", `README.md`'s "Elicitation", which links 0040 (`:1665`) |
    | know which five additions to make next for 1.25.0 adopters | `0039-MADR-…` | history: its PLAN is `complete`. "know which packages and names may change in a minor release", `README.md`'s "Stability", which links 0039 as the policy (`:1963`); the Records table |
    | see whether a Go 1.27.1 floor fits this SDK | `0001-REPORT-…` | S4: the Go rows above; the Records table |
    | see how the generated API lines up with the current ACP spec | `0001-REPORT-…` | "see how completely the SDK meets the ACP spec, as last audited", 0053; the Records table |
    | see what the current ACP v1 spec audit found | `0016-REPORT-…` | the same row, 0053, which re-scores 0016 and links it |
    | see whether the SDK covers ACP schema 1.25.0, and which three extensions … | `0051-REPORT-…` | kept, as "see what the SDK could add beyond the spec, as last assessed" |
    | see how close the SDK is to full ACP 1.25.0 parity … | `0053-REPORT-…` | kept, as "see how completely the SDK meets the ACP spec, as last audited, and which gaps remain" |
    | know why Go was accepted at 1.27.2 with schema 1.24.1 | `0002-MADR-…` | superseded: the Go row (0055) and the schema row (0054); the Records table |
    | know why the schema pin was 1.25.0 | `0004-MADR-…` | superseded: the schema row, 0054, which names 0004 as the stamp it supersedes |
    | know why the schema pin is 1.31.0, and how a client's system prompt is gated and checked | `0054-MADR-…` | kept |
    | know why this tree is not the Coder Go SDK | `0005-MADR-…` | kept, worded "know why this module is separate from github.com/coder/acp-go-sdk" |
    | rename the module off github.com/coder/acp-go-sdk | `0005-PLAN-…` | done; the Records table |
    | know why the generator uses go/ast instead of Jennifer | `0006-MADR-…` | kept, worded "writes Go with go/ast" |
    | emit the schema bindings with go/ast | `0006-PLAN-…` | done; the Records table |
    | keep this SDK self-contained, off the previous module and off Jennifer | `0007-MADR-…` | merged with the next-but-two row: "know why the SDK is self-contained and stays on encoding/json v1", 0007 |
    | apply the separation from the previous project | `0007-PLAN-…` | done; the Records table |
    | see whether 0005, 0006, and 0007 hold against the tree | `0008-REPORT-…` | history, a review of the tree of its day; the Records table |
    | read a go/ast generator that reproduces the Jennifer output | `0008-REPORT-…` | history: the generator is `cmd/generate`, described in `architecture.md` ("change the code generator"); the Records table |
    | know why the SDK stays on encoding/json v1 | `0007-MADR-…` | merged, as above |
    | see the v1 and v2 measurements behind that | `0008-REPORT-…` | kept, worded "see the encoding/json v1 and v2 measurements behind that" |
    | know why the generated output changed after the go/ast port | `0009-MADR-…` | history; "know what the SDK sends and accepts on the wire …", `README.md`'s "Wire encoding", which links 0009 (`:533`) |
    | see how unset unions, nil required arrays, and union docs were fixed | `0009-PLAN-…` | done; the same row; the Records table |
    | apply the Go 1.27.2 and schema 1.25.0 pin | `0004-PLAN-…` | S4: done; the Records table |
    | understand how this fits together | `architecture.md` | kept, worded "how this SDK fits together" |
    | know what the SDK sends on the wire | `architecture.md` | retargeted to `README.md`'s "Wire encoding" (above) |
    | know why the user-facing docs were brought up to date | `0010-MADR-…` | history: "know what the docs must cover, and how they are kept current", 0056, which names 0010 as the last whole-tree alignment |
    | see what is wrong with the code generator and how it would be hardened | `0011-MADR-…` | kept, worded "know why the generator is a validated, schema-driven engine" |
    | build the generator's resolved schema model | `0011-PLAN-resolved-schema-model.md` | done; "change the code generator", `architecture.md` (`:152`); the Records table |
    | fix how the generated unions read and write unknown, absent, and null values | `0011-PLAN-wire-fixes.md` | done; "know what the SDK sends and accepts on the wire …", `README.md`'s "Wire encoding" (`:534`); the Records table |
    | see where the generator keeps the protocol behaviour the schema does not express | `0011-PLAN-protocol-table.md` | "change the code generator", `architecture.md` (`:206`) |
    | see how the generator checks its output before writing it | `0011-PLAN-safe-output.md` | "change the code generator", `architecture.md` (`:183`) |
    | see how the generator and the generated SDK are tested | `0011-PLAN-engine-tests.md` | "change the code generator", `architecture.md`; the tests are listed in its "What it is" |
    | see how each schema keyword reaches the generated Go code | `0012-MADR-…` | kept |
    | know the rules agents follow in this repository | `AGENTS.md` | kept |
    | know why this repository has the fleet workspace and library CI | `0003-MADR-…` | kept |
    | install the fleet workspace and library CI | `0003-PLAN-…` | done; the Records table |

* **Step 3.** `docs/README.md`'s first line is unchanged: `git diff -U0 docs/README.md`'s first hunk starts at line 172.
* **The two flags in `tools/README.md`** (the owner's ruling "Name both in the next phase"). One sentence each, and nothing
  else in the file changed (`git diff --numstat`: 2 lines in, 2 out).
  * Row `:32`, `remote.py tree`: "`--host` names the test host, `linux` or `windows`, whose SSH host or alias
    `ACP_LINUX_HOST` or `ACP_WINDOWS_HOST` gives; `tree`, `pkg` and `test` each require it, and it has no default".
    From the code: `tools/remote.py:265`, `:268` and `:275`, `add_argument("--host", choices=sorted(_common.HOST_VARS),
    required=True)` in each of the three subparsers, with no `default`; `tools/_common.py:18`, `HOST_VARS = {"linux":
    "ACP_LINUX_HOST", "windows": "ACP_WINDOWS_HOST"}`; `_common.host` (`:28`) reads the variable.
  * Row `:41`, `consumer_probe.py`: "`--tree DIR` is the tree the `replace` names, by default the repository". From
    the code: `tools/consumer_probe.py:64`, help "the tree the replace names, such as a planted break's copy (default:
    the repository)"; `:67`, `tree = Path(args.tree).resolve() if args.tree else _common.repo()`.
* **The audit for the two flags.** Phase 3's audit (`inventory.py`, with its `judged.json`) was copied into this phase's
  scratch folder, and the copy's `judged.json` lost the two entries that marked these flags described by reading, as the
  owner's ruling requires. Phase 3's folder was not changed.
  * On the unedited tree, the check seen to fail: exit 1, `# 254 items, 2 not described`,
    `UNDESCRIBED flag: tools/consumer_probe.py --tree`, `UNDESCRIBED flag: tools/remote.py --host`. Nothing else.
  * After the edit: exit 0, `# 254 items, 0 not described`. With `--all`, the sentences it matched are the two above,
    at `tools/README.md:41` and `:32`.
* **The list the Verification asks for: each task row and what it points to** (in table order).
  1. understand how this SDK fits together: `architecture.md`
  2. install the SDK, and know which Go version it needs: `README.md#installation`
  3. write a first client or agent: `README.md#get-started`
  4. run the example agent and clients: `README.md#try-the-examples`
  5. build an agent or a client: the interfaces, the update constructors, `Kind()`, the refusal embeds and
     `clienthost`: `README.md#explore-the-api`
  6. know what the SDK sends and accepts on the wire: unions, absent and null values, patch fields, a form's property
     order, absolute paths: `README.md#wire-encoding`
  7. negotiate capabilities at `initialize`, authenticate, and give an agent a system prompt:
     `README.md#initialization-and-capabilities`
  8. cancel a turn, or choose what happens when a prompt arrives while one is running: `README.md#cancellation`
  9. stream a tool call's output from an agent: `README.md#streaming-a-tool-call`
  10. close a connection, learn why it ended, and wait for its goroutines: `README.md#closing-and-limits`
  11. start an agent as a child process or in this process, and stop it with everything it started:
      `README.md#closing-and-limits`
  12. bound what a connection holds when its peer sends faster than it is handled: `README.md#closing-and-limits`
  13. trace, log or authorize messages with middleware, or observe what it does not see:
      `README.md#middleware-and-observing`
  14. connect to or serve an agent over HTTP or WebSocket: `README.md#remote-agents`
  15. know which JSON-RPC error a peer gets, and when: `README.md#errors-on-the-wire`
  16. ask the user for input, and check a form, its answer or a URL: `README.md#elicitation`
  17. handle or call extension methods: `README.md#extension-methods`
  18. test a client or an agent against the other side, or replay a recorded session: `README.md#testing-with-acptest`
  19. know which packages and names may change in a minor release: `README.md#stability`
  20. see what changed in a release, what is not released yet, and which version is retracted: `CHANGELOG.md`
  21. find, for a consumer's own ACP code, the SDK API that replaces it and the release it came in:
      `0041-REPORT-consumer-adoption-of-v1-21-to-v1-25.md`
  22. see how completely the SDK meets the ACP spec, as last audited, and which gaps remain:
      `0053-REPORT-acp-1-25-0-completeness-parity-and-adherence.md`
  23. see what the SDK could add beyond the spec, as last assessed:
      `0051-REPORT-acp-1-25-0-featureset-coverage-and-three-extensions.md`
  24. know why the schema pin is 1.31.0, and how a client's system prompt is gated and checked:
      `0054-MADR-adopt-acp-schema-1-31-0-and-client-system-prompts.md`
  25. know why the SDK needs Go 1.26.11 and is built with go1.27.4: `0055-MADR-pin-go-1-27-4-and-floor-1-26-11.md`
  26. know why this module is separate from github.com/coder/acp-go-sdk: `0005-MADR-separate-module-from-coder.md`
  27. know why the SDK is self-contained and stays on encoding/json v1: `0007-MADR-self-contained-acp-sdk.md`
  28. see the encoding/json v1 and v2 measurements behind that: `0008-REPORT-self-contained-sdk-review.md`
  29. know the rules agents follow in this repository: `AGENTS.md`
  30. know why this repository has the fleet workspace and library CI: `0003-MADR-install-fleet-agent-workspace.md`
  31. write a decision record, a plan or a report, and number it: `AGENTS.md#records`
  32. find a record by its number, or a release's notes: the Records table (`#records`)
  33. check a change before staging it: `AGENTS.md#pre-add-checks`
  34. know whether a change runs CI or only the Docs workflow: `AGENTS.md#ci-routing`
  35. read a crash on CI's Windows floor job: `0033-MADR-traceback-on-windows-floor-ci.md`
  36. run the verification tools: the gates, the test hosts, planted breaks and the release checks: `tools/README.md`
  37. change the code generator: how it reads the schema, where the protocol rules live, how it checks its output:
      `architecture.md#what-it-is`
  38. know why the generator is a validated, schema-driven engine: `0011-MADR-harden-code-generator-engine.md`
  39. see how each schema keyword reaches the generated Go code: `0012-MADR-schema-fidelity-framework.md`
  40. know why the generator writes Go with go/ast instead of Jennifer: `0006-MADR-emit-go-with-go-ast.md`
  41. move the SDK to a new ACP schema version: `RELEASING.md#bump-the-schema-version`
  42. know what counts as an incompatible change, and how the API is checked:
      `RELEASING.md#release-notes-and-the-api`
  43. tag and publish a release, and check it after: `RELEASING.md#tag-and-publish`
  44. know what the docs must cover, and how they are kept current:
      `0056-MADR-bring-the-docs-current-and-keep-them-current.md`
* **Every MADR, and the row that covers its task or the reason it adds none.** The tree holds 48 MADR files (`ls
  docs/decisions/*-MADR-*.md`); the row numbers are the list's above. "Links" means the row's doc links the record at
  that line; "states" means the doc states the decision there without linking it, and the record is reached through the
  Records table.

  | MADR | Covered by | Evidence |
  | :--- | :--- | :--- |
  | `0002-MADR-pin-go-1-27-2-and-acp-schema-1-24-1.md` | none of its own: superseded | its Go pin by 0024 (its own annotation), then 0028 and 0055; its schema stamp by 0004, then 0054; its import-path sentence by 0005. The tasks are rows 25 and 24 |
  | `0003-MADR-install-fleet-agent-workspace.md` | row 30 | its own row |
  | `0004-MADR-pin-acp-schema-1-25-0.md` | none of its own: superseded | its schema stamp by 0054, whose D1 says it supersedes that stamp; its Go 1.27.2 floor by 0024. The task is row 24 |
  | `0005-MADR-separate-module-from-coder.md` | row 26 | its own row |
  | `0006-MADR-emit-go-with-go-ast.md` | row 40 | its own row |
  | `0007-MADR-self-contained-acp-sdk.md` | row 27 | its own row; row 6's section links it too (`README.md:396`) |
  | `0009-MADR-fix-generated-output-on-json-v1.md` | row 6 | links, `README.md:533` |
  | `0010-MADR-align-docs-with-current-tree.md` | none of its own: history | a past whole-tree docs alignment; the docs task now is row 44, and 0056's MADR names 0010 as "the last whole-tree docs alignment" |
  | `0011-MADR-harden-code-generator-engine.md` | row 38 | its own row; row 6 links it too (`README.md:534`) |
  | `0012-MADR-schema-fidelity-framework.md` | row 39 | its own row; row 6 links it too (`README.md:535`) |
  | `0013-MADR-close-phase-6-follow-ups.md` | none of its own: no task of its own | it closed 0011's phase 6: a test of the constructor registry, dead code removed, and the first release-notes report. The generator's tests are row 37's, a release's notes rows 32 and 43's |
  | `0014-MADR-union-constructors-take-every-required-key.md` | row 6 | links, `README.md:536` |
  | `0015-MADR-validate-absolute-protocol-paths.md` | row 6 | links, `README.md:537`; its rules are the "Paths" subsection (`README.md:443`) |
  | `0017-MADR-remediate-acp-v1-conformance-violations.md` | row 15 | links, `README.md:1652`; its cancelled turns are row 8's (`README.md:756`-`:758`) |
  | `0018-MADR-start-side-connections-after-assignment.md` | row 5 | states, `README.md:125`-`:130` (the connection is set before any handler reads); it changed no exported name, and `NewConnection`'s doc comment says the handler may run before it returns (`connection.go:975`-`:976`) |
  | `0019-MADR-remediate-debugging-pass-findings.md` | row 15 | links, `README.md:1654` |
  | `0020-MADR-retract-accidental-v1-tag.md` | row 20 | states, `CHANGELOG.md:36` ("v1.13.0 was retracted (`go.mod`)"); `go.mod:7` gives the reason |
  | `0021-MADR-resolve-issues-4-and-5-before-v1-14-0.md` | rows 6 and 8 | links, `README.md:539` (patch fields, `:399`); states, `README.md:767` (`WithPromptOverlapPolicy`) |
  | `0022-MADR-keep-elicitation-property-order-before-v1-15-0.md` | row 6 | states, `README.md:336`-`:340` (`PropertyOrder`) |
  | `0023-MADR-connection-lifetime-and-test-leaks.md` | row 10 | links, `README.md:939` |
  | `0024-MADR-go-sdk-best-practices-on-a-go-1-26-9-floor.md` | rows 25 and 42 | its floor policy stands, as 0055 says ("0024's policy and 0028's check stand"), its versions superseded by 0028 and 0055; its API gate is what row 42's section describes (`RELEASING.md:181`-`:184`) |
  | `0026-MADR-connection-end-cause-and-wait.md` | row 10 | links, `README.md:1085` |
  | `0027-MADR-agent-process-helper.md` | row 11 | links, `README.md:943` |
  | `0028-MADR-pin-go-1-27-3-and-floor-1-26-10.md` | none of its own: superseded | its versions by 0055, which says so; its check stands ("0024's policy and 0028's check stand"). The task is row 25 |
  | `0029-MADR-unix-client-death-pdeathsig.md` | row 11 | links, `README.md:962` |
  | `0030-MADR-record-initialize-on-arrival.md` | row 7 | states, `README.md:543`-`:547`; `architecture.md:552` names it |
  | `0031-MADR-verification-tools-in-repository.md` | row 36 | links, `tools/README.md:4` |
  | `0033-MADR-traceback-on-windows-floor-ci.md` | row 35 | its own row |
  | `0034-MADR-race-detector-on-windows-ci.md` | row 1 | states, `architecture.md:941`-`:942` ("both modules' tests with `-race` on the Linux and Windows jobs") |
  | `0036-MADR-session-update-constructors-kind-and-client-refusals.md` | row 5 | states, `README.md:148` (`ClientRefusals`), `:217`ff. (the session-update constructors), `:230` (`SessionUpdate.Kind()`) |
  | `0037-MADR-align-docs-ci-and-disclosure-gates.md` | row 34 | names it, `AGENTS.md:138` |
  | `0038-MADR-consumer-helpers-kind-and-agent-refusals.md` | row 5 | states, `README.md:112` (`AgentRefusals`), `:189` (`PickPermissionOption`); `architecture.md:84` links it |
  | `0039-MADR-five-adopter-additions-on-schema-1-25-0.md` | row 19 | links, `README.md:1963`; its additions are rows 5, 17 and 18's |
  | `0040-MADR-check-elicitation-forms-answers-and-urls.md` | row 16 | links, `README.md:1665` |
  | `0041-MADR-close-the-gaps-the-consumers-wait-on.md` | row 5 | links, `README.md:177`; its consumer report is row 21 |
  | `0042-MADR-in-process-agent-runner.md` | row 11 | links, `README.md:929`, `:1031` |
  | `0043-MADR-remote-http-and-websocket-transport.md` | row 14 | links, `README.md:1383`; row 19 too (`:1968`) |
  | `0044-MADR-notification-queue-back-pressure.md` | row 12 | links, `README.md:1115` |
  | `0045-MADR-notification-queue-byte-budget.md` | row 12 | links, `README.md:1182` |
  | `0046-MADR-sending-and-receiving-middleware.md` | row 13 | links, `README.md:1215` |
  | `0047-MADR-pre-add-check-full-coverage.md` | row 33 | names it, `AGENTS.md:116` |
  | `0048-MADR-create-releases-in-ci.md` | none: `proposed` | not decided and not built. Row 43's `RELEASING.md` describes releases made by hand and does not describe 0048, as this PLAN requires ("What this PLAN does not do") |
  | `0049-MADR-remediate-the-2026-11-01-debugging-pass.md` | rows 6, 10 and 13 | links, `README.md:295`, `:910`, `:1236` |
  | `0050-MADR-auth-method-ids-and-the-prompt-result-for-issue-14.md` | row 7 | links, `README.md:648` |
  | `0052-MADR-agent-tool-call-snapshot-stream-and-message-size-default.md` | row 9 | links, `README.md:826` |
  | `0054-MADR-adopt-acp-schema-1-31-0-and-client-system-prompts.md` | row 24 | its own row; row 7 links it too (`README.md:748`) |
  | `0055-MADR-pin-go-1-27-4-and-floor-1-26-11.md` | row 25 | its own row |
  | `0056-MADR-bring-the-docs-current-and-keep-them-current.md` | row 44 | its own row |

  Ten MADRs have a row of their own, 32 are covered by a row whose doc links or states them, and six add none: three
  superseded (0002, 0004, 0028), two history or housekeeping (0010, 0013), one `proposed` (0048).
* **The counts in this PLAN (the coordinator's question 1).** Step 2's "31" and the Verification's "47" are about
  different sets, both counted by the MADR's S1 at `77a81cb`: 31 is the MADRs with no row then, out of 47, the MADRs
  then. The tree at `378779e` holds 48 (`ls docs/decisions/*-MADR-*.md | wc -l`), the 48th being this pair's MADR, which
  the report's S1 counts ("32 of the 48 MADRs"). Neither number was wrong for its set, but the Verification's 47 is
  stale against the tree. Both places carry a dated annotation (above), and the list accounts for all 48. The MADR's S1
  is a reading of `77a81cb` and stays true of it, so the MADR is not amended.
* **Deviation, 2026-11-15: `CHANGELOG.md` joins the phase's files.**
  * Found: the phase's steps name no `CHANGELOG.md` line, but `RELEASING.md:171`-`:172` says "Each change adds its line
    under `## Unreleased` in the same phase as the change", and MADR D2 says the `Unreleased` section is one "that each
    phase adds to, in the same commit as its change". This phase changes what a reader sees: the matrix rewritten, and
    two flags described. Without a line, `CHANGELOG.md`'s `Unreleased` would be incomplete for this phase.
  * Decision: one line, under the owner's standing rule "Always in the same commit" (docs outside the phase's files that
    the phase makes false or incomplete are fixed in the same phase), as the coordinator's brief relays it. The line:
    "Rewrote `docs/README.md`'s "I want to…" table as one row per task a reader comes with, each pointing to the doc or
    record that answers it now; the Records table stays the index of every record. `tools/README.md` now describes
    `consumer_probe.py --tree` and `remote.py --host`."
  * Files added to the phase: `CHANGELOG.md`, that line alone (the note under phase 6's **Files**). `CHANGELOG.md` is
    not docs-safe until phase 5b, so the phase needs a heavy run (Verification, below).
  * The MADR asserts no fact this changes; it is not amended.
* **Docs step.** Searched `README.md`, `docs/architecture.md`, `AGENTS.md`, `RELEASING.md`, `tools/README.md`,
  `CHANGELOG.md` and the agent-rule files (`.claude/rules/`, `.codex/rules/`, `.grok/rules/`, `.kilo/agent/`,
  `.opencode/rules.md`) for a copy of the Records table or the matrix, or a count of records. None holds one. The
  mentions of `docs/README.md` stay true: `docs/architecture.md:1537` ("ToC and the "I want to…" matrix"), `AGENTS.md:10`
  and `:103`, `RELEASING.md:178`, `README.md:1977` and `CHANGELOG.md:34`. Nothing else was made false or incomplete,
  beyond the `CHANGELOG.md` line of the deviation. No Go-file row is opened.
* **Verification.** Each output went to a scratch file and its exit status was taken before the file was read.
  * The index script, planted break: on a scratch copy of `docs/README.md` with the
    `0033-PLAN-traceback-on-windows-floor-ci.md` row removed and the `0048-MADR-create-releases-in-ci.md` row's
    status changed to `accepted`: exit 1, `MISSING row for file decisions/0033-PLAN-traceback-on-windows-floor-ci.md`,
    `STATUS decisions/0048-MADR-create-releases-in-ci.md: row 'accepted', frontmatter 'proposed'`.
  * The index script on the tree, before any edit and after the last: exit 0, no output.
  * The new links' anchors: `check_records.py --check-all` checks them as GitHub computes anchors. On a scratch clone
    with this phase's `docs/README.md`, `CHANGELOG.md` and `tools/README.md`: exit 0, no output. With
    `#streaming-a-tool-call` changed to `#streaming-a-tool-calls` and `../AGENTS.md` to `../AGENT.md`: exit 1,
    `broken anchor: docs/README.md:192: ../README.md#streaming-a-tool-calls`,
    `broken relative link: docs/README.md:217: ../AGENT.md#ci-routing`, `2 error(s)`.
  * The flag audit: before, exit 1 with the two flags; after, exit 0 (above).
  * `python3 -B scripts/check_records.py --check-all`: exit 0, no output.
  * `python3 -B scripts/check_disclosure.py`: exit 0, "check-disclosure: 840 tracked file(s), 0 finding(s)".
  * Offline `markdownlint-cli2` v0.23.2, the version the Makefile pins: exit 0, "Linting: 57 files", "Summary: 0 issues
    in 0 files".
  * `python3 -B scripts/check_ci_filters.py --docs-safe docs/README.md tools/README.md CHANGELOG.md
    docs/decisions/0056-PLAN-bring-the-docs-current-and-keep-them-current.md`: exit 1, "check-ci-filters: not
    docs-safe: CHANGELOG.md". So the heavy run is needed.
  * `run-heavy -- make pre-add-check FILES="CHANGELOG.md docs/README.md tools/README.md
    docs/decisions/0056-PLAN-bring-the-docs-current-and-keep-them-current.md"`, on this Mac: exit 0. It waited 0 s in the
    queue and ran 120 s, 14:53:03 to 14:55:03 local, after the last edit of `CHANGELOG.md`, `docs/README.md` and
    `tools/README.md`. `go-precheck`: 0 `.go` files to format; both modules linted, vetted and tested (`-race
    -shuffle=on`, uncached), tidy and govulncheck, clean. Only this line of this record was written after it; the PLAN
    is docs-safe and no Go check reads it. The index script, the flag audit, `check_records.py --check-all`,
    `check_disclosure.py` and markdownlint were run again after this line, with the outputs above.
* **Not verified.**
  * That the 44 rows are every task a reader comes with. That is the judgment this phase is priced for; the list above
    is the evidence, not a measurement.
  * The anchors as GitHub renders them: `check_records.py` computes them as GitHub does, and no page was viewed.
  * The two sentences against a run: `remote.py` needs a test host and was not run; its `--host` was read from the
    source, and phase 3's run recorded the same from `--help` ("`--host` required with choices `linux` and `windows`").
    `consumer_probe.py --tree` was not run again; phase 3's run recorded it ("the probe `go.mod`'s `replace` names the
    clone with no flag and the tree copy with `--tree`").
  * The owner's standing rule "Always in the same commit" is quoted from the coordinator's brief. No record in this
    repository quotes it.
  * CI on this change: it is not pushed.
* **Files.** `docs/README.md` (the matrix, its paragraph, and the three 0056 rows); `tools/README.md` (rows `:32` and
  `:41`, one sentence each); `CHANGELOG.md` (one line, by the deviation); this PLAN (the notes under phase 6's
  **Files**, step 2 and Verification, and this record). No `.go` file, workflow, script or `Makefile` changed, and no
  report.
* **Commit.** Staged, not committed. The coordinator commits and pushes after the disclosure guard. The plan pauses
  until the owner says to continue.

### 2026-11-15: phase 5b

Provenance: run by a mainstream model at medium effort, as the phase's row prices it, on the coordinator's brief. Line
numbers in this entry are those of the staged files.

* **Approval.** The owner, 2026-11-15: "Run 5b, 4a and 4b through". The coordinator ran phase 5b first, one phase to an
  agent, and reviews, commits and watches CI before briefing phase 4a. A deviation still stops the phase and goes to the
  owner.
* **Base.** `git rev-parse HEAD`: `175f269e7135d0d29a0cf66b05c2733a711a25bc`, on `main`; `git status --short` was empty.
  Before any edit, `python3 -B scripts/check_ci_filters.py`: exit 0, "0 problem(s)"; and
  `python3 -B scripts/check_ci_filters.py --docs-safe CHANGELOG.md`: exit 1, "check-ci-filters: not docs-safe:
  CHANGELOG.md". No `.go` file names `CHANGELOG.md` (`grep -rn CHANGELOG --include=*.go .` printed nothing), so
  `check_ci_filters.py`'s "readers" part does not forbid ignoring it.
* **Step 1: the four lists and the three descriptions.** `'CHANGELOG.md'` is added after `'RELEASING.md'` in the same
  position in each list: `.github/workflows/ci.yml:15` (push `paths-ignore`) and `:27` (pull_request),
  `.github/workflows/docs.yml:13` (push `paths`) and `:26` (pull_request). The descriptions: `AGENTS.md:97` (the docs
  `make check-records` checks) and `:130` (CI's `paths-ignore`, "CI routing"), and `docs/architecture.md:938` (the docs
  `check-records` checks) and `:944` (CI's docs-safe paths). Those two architecture passages were at `:913`-`:919` when
  the plan was written and are at `:938`-`:946` at `175f269`; a dated annotation under phase 5b's steps records it.
  `AGENTS.md:97` and `docs/architecture.md:938`-`:944` were re-wrapped to stay under the lines' existing width.
* **Step 2: the script.** `"CHANGELOG.md"` is added after `"RELEASING.md"` in `UNNUMBERED_DOCS`
  (`scripts/check_records.py:239`) and in the docstring's list (`:11`-`:12`). The comment above it
  (`:235`-`:238`) is the one other line of either script that changed, by the second deviation below; no other line of
  either script or workflow changed.
* **The file `AGENTS.md` now lists.** CI routing, `AGENTS.md:129`-`:130`: "`paths-ignore` (`docs/**`, `AGENTS.md`,
  `RELEASING.md`, `CHANGELOG.md`, `tools/README.md` and the agent-rule files)".
* **Planted breaks, each on a scratch clone of `175f269` (`git clone --quiet`) with this phase's five changed files
  copied in; nothing in the tree was dirtied.** `<scratch>` is the phase's scratch folder.
  * Link check, planted: `CHANGELOG.md`'s `(docs/README.md)` changed to `(docs/READMEE.md)` in the clone.
    `python3 -B scripts/check_records.py --check-all`: exit 1, `broken relative link: CHANGELOG.md:34:
    docs/READMEE.md`, `1 error(s)`. On the same clone without the break: exit 0, no output.
  * Link check, control: the same break in a clone with `scripts/check_records.py` as at `175f269` (no
    `UNNUMBERED_DOCS` change): exit 0, no output. So the edit to `UNNUMBERED_DOCS` is what makes the check read
    `CHANGELOG.md`; before it, the link was not checked, as phase 5a's record says.
  * `check_ci_filters.py`, `'CHANGELOG.md'` removed from both of `docs.yml`'s lists alone: exit 1, "docs: Docs' paths
    are not CI's ignored paths and .github/workflows/docs.yml: missing ['CHANGELOG.md'], extra []" and "tracked: CI
    ignores CHANGELOG.md, and Docs does not run on it", "2 problem(s)".
  * The same, removed from both of `ci.yml`'s lists alone: exit 1, "docs: Docs' paths are not CI's ignored paths and
    .github/workflows/docs.yml: missing [], extra ['CHANGELOG.md']", "1 problem(s)". In that clone `--docs-safe
    CHANGELOG.md`: exit 1, "not docs-safe: CHANGELOG.md".
  * Removed from `ci.yml`'s pull_request list alone: exit 1, "filters: CI's push and pull_request paths-ignore differ,
    or are empty", "1 problem(s)".
  * `check_ci_filters.py` does not read `AGENTS.md`, `docs/architecture.md` or `check_records.py`, so no break in them
    can fail it. `AGENTS.md`, `docs/architecture.md` and `UNNUMBERED_DOCS` agree with the workflows by the read
    above and not by a check.
* **Does `docs.yml` run the link check on a changelog-only change?** Yes, by its trigger and its steps. A one-off
  Python call into `check_ci_filters.py`'s own glob matcher (`_matches`) on the real `docs.yml` and `ci.yml`: `docs.yml`
  push and pull_request `paths` match `CHANGELOG.md` (True, True), and `ci.yml`'s `paths-ignore` match it (True,
  True), so a push or pull request that touches only `CHANGELOG.md` skips `ci.yml`'s `push` and `pull_request` triggers
  and starts `docs.yml`. Its one job runs `make markdownlint`, `make check-records` and `make disclosure-check`
  (`docs.yml:55`-`:57`), and `check-records` runs `--check-all`, which reads `UNNUMBERED_DOCS`. That the GitHub
  runner reads the lists the same way was not run: it is not pushed.
* **Does anything else list the docs-safe paths?** Searched `AGENTS.md`, `RELEASING.md`, `README.md`,
  `docs/architecture.md`, `docs/README.md`, `tools/README.md`, `tools/*.py`, `scripts/*`, `Makefile`, the workflows'
  comments and the rule files (`.claude/rules/`, `.codex/rules/`, `.grok/rules/`, `.kilo/agent/`, `.opencode/rules.md`)
  for "docs-safe", "paths-ignore", "agent-rule" and `CHANGELOG`. Only the three descriptions above list the paths. The
  rest say "docs-safe" without a list (`AGENTS.md:113`, `docs/architecture.md:931`, `scripts/go-precheck.sh:17`,
  `tools/publication.py`) or read the list from the workflow. The rule files hold no list and no mention of
  `CHANGELOG.md`.
* **Phase 6's sentence.** Phase 6's entry says `CHANGELOG.md` "is not docs-safe until phase 5b, so the phase needs a
  heavy run", and phase 5a's Verification says the same. Each was true when its phase ran, in the order 1, 2, 5a, 3, 6,
  5b, 4a, 4b, and stays a true statement of that phase, so neither is rewritten. The live sentence for later phases is
  "Heavy runs" under Timing (`:75`-`:80`), which named `CHANGELOG.md` as the exception and listed the docs-safe paths
  without it. A dated annotation after it (`:82`-`:84`) says that `CHANGELOG.md` is docs-safe from this phase's commit.
* **Deviation, 2026-11-15: `CHANGELOG.md` joins the phase's files.**
  * Found: the phase's steps name no `CHANGELOG.md` line, but `RELEASING.md:171`-`:172` says "Each change adds its line
    under `## Unreleased` in the same phase as the change", and MADR D2 says `Unreleased` is one "that each phase adds to,
    in the same commit as its change". This phase changes what a reader sees, since a changelog-only change now runs the
    Docs workflow, and what a contributor reads in `AGENTS.md`. Without a line `Unreleased` is incomplete for it.
  * Decision: one line, under the owner's standing rule "Always in the same commit", as the coordinator's brief relays
    it, and as phase 6 did. `CHANGELOG.md:23`-`:24`: "Made this file docs-safe: a change to `CHANGELOG.md` alone now
    runs the Docs workflow and not CI's Go suite, and `make check-records` checks its links with the other unnumbered
    docs."
  * Files added to the phase: `CHANGELOG.md`, that line alone (the note under phase 5b's steps). The MADR asserts no fact
    this changes; it is not amended.
* **Deviation, 2026-11-15 (second): the comment above `UNNUMBERED_DOCS`.**
  * Found: the first draft of this phase left `scripts/check_records.py:235`-`:238` as it was. That comment says
    "RELEASING.md, tools/README.md and AGENTS.md are docs-safe, so the Docs workflow, which runs this check, is the one
    that sees their changes". It was not false, and this phase makes it incomplete, since `CHANGELOG.md` is now
    docs-safe. The coordinator's review found it. The phase's step 2 says "No other change to either script".
  * Decision: name `CHANGELOG.md` with the others, under the owner's standing rule "Always in the same commit", as the
    coordinator's review relays it. Before: `#: RELEASING.md, tools/README.md and AGENTS.md are docs-safe, so the Docs` /
    `#: workflow, which runs this check, is the one that sees their changes`. After (`:236`-`:237`): `#: RELEASING.md,
    CHANGELOG.md, tools/README.md and AGENTS.md are docs-safe, so` / `#: the Docs workflow, which runs this check, is
    the one that sees their changes`. The comment is re-wrapped and nothing else in the file changes.
  * Searched for the same omission in `scripts/check_ci_filters.py`, `scripts/test_check_ci_filters.py`,
    `scripts/test_check_records.py`, `scripts/go-precheck.sh`, and the comments of `ci.yml` (`:2`-`:6`) and `docs.yml`
    (`:2`-`:5`). None lists the docs-safe files or counts them: `check_ci_filters.py` names `README.md` and the
    `--docs-safe` rule alone, the workflows' comments say "the docs-safe paths" and "the same as docs.yml's paths",
    and `test_check_records.py:47`, a fixture loop over three files, is code and not a comment. No other comment needed
    the fix.
  * Files added to the phase: none new; `scripts/check_records.py` is on its list. The MADR is not amended.
* **Docs step.** Searched `README.md`, `docs/architecture.md`, `AGENTS.md`, `RELEASING.md`, `tools/README.md`,
  `docs/README.md`, `CHANGELOG.md`, the rule files and the Go files' comments (none names the docs-safe list). Fixed:
  `AGENTS.md:97` and `:130`, `docs/architecture.md:938` and `:944` (step 1); the plan's `:75`-`:80` (the annotation).
  `docs/architecture.md:1491`, the Tree line for `CHANGELOG.md`, stays true. Fixed, by the second
  deviation: the comment above `UNNUMBERED_DOCS` (`scripts/check_records.py:235`-`:238`), which listed the docs-safe
  files without `CHANGELOG.md`. Nothing is open.
* **Verification.** Each output went to a scratch file and its exit status was taken before the file was read. Times are
  local.
  * `python3 -B scripts/check_ci_filters.py`: exit 0, "0 problem(s)". `--docs-safe CHANGELOG.md`: exit 0, no output.
  * `make check-tools`: exit 0, "Ran 41 tests", "OK", and its own `check_ci_filters.py` run "0 problem(s)".
  * `actionlint` v1.7.12, the version `ci.yml:165` pins, on `ci.yml` and `docs.yml`: exit 0, no output.
  * `python3 -B scripts/check_records.py --check-all`: exit 0, no output.
  * `python3 -B scripts/check_disclosure.py`: exit 0, "840 tracked file(s), 0 finding(s)".
  * Offline `markdownlint-cli2` v0.23.2, the version the Makefile pins: exit 0, "Linting: 57 files", "Summary: 0 issues
    in 0 files".
  * `run-heavy -- make pre-add-check FILES=".github/workflows/ci.yml .github/workflows/docs.yml AGENTS.md CHANGELOG.md
    docs/architecture.md scripts/check_records.py"`, on this Mac, the first run: exit 0, waited 0 s, ran 117 s, 17:21:32
    to 17:23:29. Superseded: the second deviation edited `scripts/check_records.py` after it.
  * The same command, the second run, after the last edit of any of those six files: exit 0, waited 0 s, ran 115 s,
    17:31:45 to 17:33:40. `go-precheck`: 0 `.go` files to format; both modules linted, vetted and tested (`-race
    -shuffle=on`, uncached), tidy and govulncheck, clean. Only this PLAN and `docs/README.md`, which are docs-safe and
    which no Go check reads, were written after it. The other checks above were run again after the second deviation,
    with the same results.
* **Not verified.**
  * That GitHub starts `docs.yml` and skips `ci.yml` on a changelog-only push. It is read from the lists and the
    script's own matcher, and no push was made. CI on this change is not run.
  * The matcher is `check_ci_filters.py`'s `glob_re`, not GitHub's; for a literal file name at the root they agree.
  * `docs/architecture.md`, `AGENTS.md` and `UNNUMBERED_DOCS` agree with the workflows by reading, not by a check, since
    no check reads them.
  * The owner's standing rule "Always in the same commit" is quoted from the coordinator's brief, as in phase 6.
* **Files.** `.github/workflows/ci.yml`, `.github/workflows/docs.yml`, `AGENTS.md`, `docs/architecture.md`,
  `scripts/check_records.py`; `CHANGELOG.md` (one line, by the deviation); `docs/README.md` (this PLAN's row); this
  PLAN (the annotation at `:82`, the notes under phase 5b's steps, and this record); `scripts/check_records.py` also for
  its comment, by the second deviation. No `.go` file, test, report or
  `Makefile` changed.
* **Commit.** Staged, not committed. The coordinator commits and pushes after the disclosure guard. The plan pauses
  until the coordinator briefs phase 4a.

### 2026-11-15: phase 4a

Provenance: run by a mainstream model at medium effort, as the phase's row prices it, on the coordinator's brief. Line
numbers in this entry are those of the staged files.

* **Approval.** The owner, 2026-11-15: "Run 5b, 4a and 4b through". The coordinator ran phase 5b, committed it and
  watched CI, then briefed this phase alone. A deviation still stops the phase and goes to the owner.
* **Base.** `git rev-parse HEAD`: `7b0fbffe60e92e0728a061f3c8d3d363988248df`, on `main`; `git status --short` was
  empty. `git diff --stat v1.36.0 HEAD -- '*.go' Makefile go.mod` printed nothing, so no code has changed since
  `v1.36.0`. Timing: no other PLAN is `in-progress` in `docs/README.md` (the 0056 PLAN's row is the only one), so no
  approved PLAN was about to rewrite code in a file this phase touches.
* **Step 1: the measurement.** `<scratch>/undoc.go`, the standard library alone, run as `go run undoc.go <repo>`. It
  runs `git ls-files '*.go'`, parses each tracked non-test file of the root package, `unstable`, `acptest`,
  `agentproc`, `clienthost` and `remote` with `parser.ParseComments`, marks a file generated by the "Code generated
  ... DO NOT EDIT." line before its package clause, and counts as `docs_test.go:330`-`:371` does: a lone type takes
  its declaration's comment, a type in a group needs its own, a constant or variable takes its own or its group's.
  Methods count on exported receivers. Fields (each name; embedded ones too) and interface methods of every exported
  type, `type ( … )` groups included, count as documented by a comment of their own or by their type's comment
  naming them as a word (`\bName\b`); the program prints the rule per member under `-v`. It writes nothing to the
  tree. `-hand` limits the exit status to hand-written files (the plan's acceptance for this phase); without it the
  exit status covers generated files too, as phase 4b will use it.
* **Step 2: the planted break.** On a scratch clone of `7b0fbff`, three comments deleted: above
  `ClientRefusals.CompleteElicitation` (`refusals.go:21`-`:22`), above the generated type
  `AuthMethodTerminalInline` (`types_gen.go:739`-`:742`), and above the field `DialOptions.Header`
  (`remote/client.go:27`-`:28`; its type's comment, "DialOptions configure Dial", does not name it). The program,
  exit 1, printed:
  `UNDOCUMENTED hand method  ClientRefusals.CompleteElicitation  refusals.go:21`,
  `UNDOCUMENTED hand field   DialOptions.Header                    remote/client.go:27`,
  `UNDOCUMENTED gen  type    AuthMethodTerminalInline             types_gen.go:739`, and
  "undocumented: hand-written 36, generated 54; exit status counts 90", that is the tree's 87 and the three planted.
* **Step 3: the tree before any comment, against phase 1.** Exit 1, 87 undocumented, every one a field or interface
  method; top-level declarations and methods at 0. Per package, as phase 1's table gives them, and the same on every
  column (top-level, undocumented, methods, undocumented, members, own comment, named by type, undocumented):

  | Package | Source | Members | Own comment | Named by type | Undocumented before | after |
  | :--- | :--- | ---: | ---: | ---: | ---: | ---: |
  | `acp` | generated | 1,203 | 1,133 | 19 | 51 | 51 |
  | `acp` | hand-written | 43 | 6 | 15 | 22 | 0 |
  | `acptest` | hand-written | 20 | 5 | 5 | 10 | 0 |
  | `agentproc` | hand-written | 14 | 12 | 0 | 2 | 0 |
  | `clienthost` | hand-written | 4 | 3 | 1 | 0 | 0 |
  | `remote` | hand-written | 11 | 11 | 0 | 0 | 0 |
  | `unstable` | generated | 56 | 54 | 0 | 2 | 2 |

  Hand-written 34, generated 53, total 87, as phase 1 has them. With a directive counted as a comment (`-lenient`)
  the counts agree with phase 1's on every column.

  *Finding, and a reading.* Phase 1's scan counted a trailing `//nolint` directive as a member's own comment. Two
  hand-written fields carry one and nothing else: `CompactionEntity.Id` (`compaction.go:24`) and `ToolCallEntity.Id`
  (`tooltimeline.go:31`), neither named by its type's comment as the word `Id`. `go/ast`'s `CommentGroup.Text` drops
  directives, so they are not comments in `go/doc`'s sense, and by the owner's rule the two fields are undocumented.
  The program counts them (default; `-lenient` restores phase 1's reading), so the honest baseline is 89, 36 of them
  hand-written. Both are documented in this phase, by the same type-comment lists as their neighbours. This is a
  reading of "its own comment", not a change of scope, and no file outside the phase's list is touched; the owner may
  overrule it.
* **Step 4: the comments.** 36 members in 12 files. Where a field sits in a gofmt-aligned block, a comment above
  it would make gofmt re-pad the neighbouring code lines, and the acceptance script (every changed line blank or
  `//`) would fail on whitespace. The plan's alternative, naming the member in its type's comment, was therefore
  used for every struct whose fields are aligned; a field with no padding to disturb, and the interface methods, have
  comments of their own.

  | Where | Members | Form |
  | :--- | :--- | :--- |
  | `compaction.go:10`-`:22` | `CompactionEntity`'s `Id`, `Status`, `Summary`, `Error`, `Meta` | type comment, a list |
  | `tooltimeline.go:12`-`:29` | `ToolCallEntity`'s `Id`, `Title`, `Kind`, `Status`, `Name`, `Content`, `Locations`, `RawInput`, `RawOutput` | type comment, a list |
  | `errors.go:18`-`:24` | `RequestError.Code`, `Message`, `Data` | type comment, a list |
  | `errors.go:303`-`:306` | `ProtocolVersionError.Requested` | type comment |
  | `elicitation.go:43`-`:47` | `ElicitationField.Name`, `Required`, `Schema` | type comment |
  | `elicitation.go:391`-`:394` | `ElicitationURL.Host`, `Secure` | type comment |
  | `observer.go:71`-`:72` | `Observation.Kind` | the field's own comment |
  | `acptest/client.go:42`-`:55` | `TerminalHost`'s five methods | a comment on each |
  | `acptest/pair.go:17`-`:21` | `Pair.Agent`, `Client` | type comment |
  | `acptest/recorder.go:28`-`:31` | `Record.Direction`, `Frame` | type comment |
  | `acptest/script.go:16`-`:17` | `Turn.Updates` | the field's own comment |
  | `agentproc/agentproc.go:41`-`:44` | `Command.StderrTailLines` | type comment |
  | `agentproc/inprocess.go:42`-`:44` | `InProcess.InputTap` | type comment |

  Each was written from the code that uses the member: `Apply` and `entity` in `compaction.go` and `tooltimeline.go`
  (what a patch keeps, what an unseen id starts as); `json:"data,omitempty"` and the constructors for `RequestError`;
  `client_gen.go:535`-`:546` and `agent_gen.go:377`-`:380` for `ProtocolVersionError.Requested` (the application's
  version on the client side, the client's request's on the agent's); `Fields` and `Check` for `ElicitationField`;
  `ParseElicitationURL` for `ElicitationURL`; `ObservedFault = iota + 1` for `Observation.Kind` (zero is not a kind);
  `clienthost.Terminals`' own comments and the schema's request texts for `TerminalHost`; `connectWithin` for `Pair`;
  `Recorder.add` and `canonical` for `Record`; `ScriptAgent.Prompt` for `Turn.Updates`; `agentproc.go:221`-`:222` for
  `StderrTailLines`; `agentproc.go:253` and `inprocess.go:90` for `InputTap`. The ten of them a reader is most likely
  to dispute are given verbatim, with their lines, in the coordinator's report. No comment states behaviour the code
  does not have; none stopped the phase.
* **Step 4: the Go-file statement moved here from phase 2: W7.** `doc.go:36`-`:45`, the connection's sentinels. Before:
  "[ErrNotInitialized], [ErrTerminalAuthMethod], [ErrUnadvertisedAuthMethod], [ErrPeerDisconnected],
  [ErrConnectionClosed] and [ErrPromptCancelled]." (six). After: the same six, then "[ErrShutdown],
  [ErrHandlersRunning], [ErrSendingMiddleware] and [ErrNotificationQueueOverflow]" (ten), each read at its
  declaration (`errors.go:212`, `:221`, `:232`, `:258`) as one the connection returns. `ErrToolCallStreamFinished`,
  `ErrToolCallStreamField` and `ErrRepeatedCursor` are a stream's and a helper's, not the connection's, and stay out.
  Phase 3 moved no Go-file statement here: its record says no Go-file row was opened, and its inventory found a
  package comment in every package.
* **Step 5: the changelog.** `CHANGELOG.md:25`-`:30`, one line under `Unreleased`, covering the comments and W7.
* **After.** The program on the staged tree, `-hand`: exit 0, "undocumented: hand-written 0, generated 53", with every
  hand-written package at 0 (also with `-lenient`). Without `-hand` it exits 1 on the generated 53, which are phase
  4b's: 51 in `acp` (`types_gen.go`: 49 discriminator fields, and `Negotiation.Request` and `Response`) and 2 in
  `unstable` (`unstable_gen.go`). No `*_gen.go` file was edited.
* **Acceptance script.** `<scratch>/commentsonly.py`, over `git diff --cached -U0 -- '*.go'`: prints a line and exits
  1 when an added or removed line is neither blank nor a `//` comment. Seen to fail first: on a scratch clone with the
  phase's comments applied and one code line changed (`e.Seen = true` to `e.Seen = false` in `compaction.go`), exit 1,
  `CODE LINE b/compaction.go: -		e.Seen = true` and the `+` line; the same clone without that change, exit 0. On the
  staged diff: "95 added or removed lines, 0 not blank or comment", exit 0.
* **Docs step.** Searched `README.md`, `docs/architecture.md`, `RELEASING.md`, `AGENTS.md`, `tools/README.md` and
  `CHANGELOG.md` for each member and sentinel the comments describe (`ProtocolVersionError` and its `Requested`,
  `TerminalHost`, `CompactionEntity`, `ToolCallEntity`, `ElicitationURL`, the connection's sentinels). Nothing is
  contradicted: `README.md:559`-`:562` says the same of `ProtocolVersionError`'s `Requested` and `Answered`, and
  `README.md:914`-`:931`, `:1075`, `:1263` already describe the four sentinels W7 adds. One docs row is open, and is
  phase 4b's: `docs/architecture.md:172` says "a type, field or interface method has its schema description" of
  generated code, which the 49 discriminator fields do not have until 4b's sentence names them; 4b's docs step covers
  it.
* **Verification.** Each output went to a scratch file and its exit status was taken before it was read. Times are
  local.
  * The program, `-hand`, on the staged tree, after the last edit of a `.go` file: exit 0.
  * `gofmt -l` on the 12 changed `.go` files: prints nothing, exit 0.
  * `make api`, 19:00:32 to 19:00:43, after the last `.go` edit: exit 0, "against v1.36.0: 0 incompatible, 0
    compatible"; `unstable` and `remote` report 0 incompatible and are not gated.
  * `run-heavy -- make pre-add-check FILES="<the 12 .go files and CHANGELOG.md>"`, on this Mac, after the last `.go`
    edit: exit 0, waited 0 s, ran 139 s, 19:00:43 to 19:03:02. `go-precheck`: 12 files gofmt-clean; both
    modules linted, vetted and tested (`-race -shuffle=on`, uncached), tidy and govulncheck, clean. That covers
    revive's `exported` rule and `docs_test.go`, `TestExportedTypeDocsStartWithName` among them. Only this PLAN and
    `docs/README.md`, which no Go check reads, were written after it.
  * `python3 -B scripts/check_records.py --check-all`, `python3 -B scripts/check_disclosure.py` and offline
    `markdownlint-cli2` v0.23.2: run after the last record edit: `check_records.py` exit 0, no output; `check_disclosure.py` exit 0, "840 tracked file(s), 0 finding(s)"; markdownlint exit 0, "Linting: 57 files", "Summary: 0 issues in 0 files".
* **Not verified.**
  * That each comment's behavioural claim holds beyond a reading of the code that uses it: no test was written that
    asserts a comment (comments are not tested), and the claims about `TerminalHost`'s five methods rest on
    `clienthost.Terminals`' comments and the schema's request texts, not on a run.
  * That `ToolCallEntity.Kind`'s and `Status`'s "empty when unknown" holds for every decoding path: it was read at
    `ToolCall`'s and `ToolCallUpdate`'s `decodeKeys` (`types_gen.go`), where an unknown value becomes the empty or nil
    one, and not run.
  * The audit counts a field by the word match alone; whether a type's comment *says what the field holds* is judged
    by reading, not by the program.
* **Files.** `compaction.go`, `doc.go`, `elicitation.go`, `errors.go`, `observer.go`, `tooltimeline.go`,
  `acptest/client.go`, `acptest/pair.go`, `acptest/recorder.go`, `acptest/script.go`, `agentproc/agentproc.go`,
  `agentproc/inprocess.go`; `CHANGELOG.md` (one line); `docs/README.md` (this PLAN's row text); this record. Comments
  only: no code line, test or generated file changed, and no tag was cut.
* **Commit.** Staged, not committed. The coordinator commits and pushes after the disclosure guard. The plan pauses
  until the coordinator briefs phase 4b.
