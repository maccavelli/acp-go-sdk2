---
status: accepted
date: 2026-11-14
decision-makers: the repository owner
consulted: "a read-only documentation survey of this tree at `77a81cb`, by a mainstream-tier agent at medium effort; this tree at `77a81cb`, read again for this record"
informed: the fleet coordinator, who relayed the owner's rulings
---

# Bring the docs current, add a changelog, and keep them current with no new checks

## Context and Problem Statement

The owner's mandate, 2026-11-14, relayed by the coordinator:

> "Docs across all related repos should be accurate, comprehensive and kept up to date. When they fall behind they are hard to fix"

This record applies the mandate to this repository. It decides what "comprehensive" means here, where the changelog
lives, how the docs stay current, and how docs work orders against a release. The owner made those four choices on
2026-11-14 (Decision Outcome). The PLAN,
[0056-PLAN-bring-the-docs-current-and-keep-them-current.md](0056-PLAN-bring-the-docs-current-and-keep-them-current.md),
brings the docs current. Several smaller choices are left to the owner (Open Questions).

### The state of the tree

Every fact below was read on 2026-11-14 at `77a81cb` on `main`, with a clean tree. The repository was only read. The
newest tag is `v1.36.0`. The two commits since it, `8c0170d` and `77a81cb`, change records alone. No `.go` file has
changed since the tag.

### The evidence and its limits

**The survey.** A mainstream-tier agent read the tree at `77a81cb` and changed nothing. It reported 1 verified wrong
statement and 2 it could not verify, 12 rows of missing documentation and 1 real stale item. The survey points to where
to look. It is not a verified list:

* It checked `README.md` (1,927 lines) and `docs/architecture.md` (1,469 lines) by extracting tokens, such as
  identifiers, paths and `make` targets, and checking that each one exists. It did not read the prose through. A token
  that exists but is the wrong one for its claim would pass.
* It measured documentation by name: whether an exported identifier appears as a word in some doc. That is not the
  owner's standard (Decision Outcome, C2), which asks for a doc comment on every exported name and does not ask for
  every name to appear in prose. The survey did not measure doc comments.
* Its row about a missing `acpext` module came from an error in the survey's brief: that module belongs to another
  repository. It is not a finding, and this record leaves it out.

**Checked again for this record**, at `77a81cb`, by reading the tree and by one-off scripts in a scratch directory
outside the tree:

| Id | Where | Finding | How it was checked | Result |
| :--- | :--- | :--- | :--- | :--- |
| W1 | `docs/architecture.md:1205` | "The output (`output.go`) is kept to the request's `outputByteLimit` and `MaxOutputBytes`" | `ls clienthost` has no `output.go`. The limit is in `clienthost/terminals.go` (`DefaultMaxOutputBytes`, `req.OutputByteLimit`). The coordinator checked this row too, with the same result | **wrong, verified** |
| W2 | `docs/architecture.md:246` | "the capability that gates each of 28 methods, the 13 unstable `providers/*`, `nes/*` and `document/*` among them" | In `cmd/generate/internal/model/acp.go`, 28 method entries carry a `Capability`, and 13 of them are `providers/`, `nes/` or `document/` methods. The 36 entries of `CapabilityRules` (`:248`) were counted again too. This counts the protocol table's text, not the generated code | **true; the survey's lead is refuted** |
| W3 | `README.md`, all of it; `docs/architecture.md`, all of it | prose that describes behaviour, the survey's A3 included ("Closing and limits", "Middleware and observing", "Remote agents", "Elicitation") | not read through, by the survey or for this record | **lead** |
| M1 | `Makefile:69`-`:163` | targets `all`, `vet`, `lint`, `tidy`, `vuln`, `bench` and `help` | none of them appears as `make <target>` or `` `<target>` `` in `README.md`, `docs/architecture.md`, `RELEASING.md`, `tools/README.md` or `AGENTS.md`. Only `make help` prints the Makefile's own `##` text | **missing, verified** |
| M2 | `Makefile:50`, `:145`-`:147` | the Make variables `GOLANGCI_LINT`, `MUTANTS`, `SEED` and `JOBS`. `GOLANGCI_LINT` is also an environment variable that `tools/platforms.py:31` reads | named in none of those five files. `FUZZTIME`, `BASE`, `VERSION`, `MDSH`, `GOCACHE` and `FILES` are named | **missing, verified; not in the survey** |
| M3 | `cmd/generate/main.go:41`; `cmd/generate/internal/mutate/main.go:65`-`:74`; `example/claude-code/main.go:209` | the generator's `-schema` flag; the mutation tool's ten flags; `example/claude-code`'s `-yolo` | `-schema` is named in no doc (`-out` and `-write-unstable-manifest` are). `-dir`, `-files`, `-pkgs`, `-seed`, `-n`, `-j` and `-o` appear in no doc. The hits for `-skip`, `-only` and `-timeout` were not read for context. `README.md:79` lists `example/gemini`'s flags, and `example/claude-code` has no flag list | **missing, verified; not in the survey** |
| M4 | `connection.go:906`-`:911`, `:1444` | the connection's diagnostic logging: `WithLogger` and `Connection.SetLogger` | `slog` appears in neither `README.md` nor `docs/architecture.md`. The one `Logger` in `README.md` (`:1438`) is about the remote handler | **missing, verified; not in the survey** |
| M5 | `tools/_common.py` | the tools' shared helper module | `tools/README.md` does not name it. It is a module, not a command, so whether the owner's standard covers it is a judgment for the PLAN's phase 1 | **lead** |
| M6 | the root | `CHANGELOG.md` | none exists. `RELEASING.md`, section "Release notes and the API", says "The changelog is the release notes report" | **missing, verified** |
| M7 | `RELEASING.md` | the release tools the recent release PLANs ran: `tools/release_body.py`, `tools/watch_ci.py`, `tools/publication.py`, `tools/consumer.py` and `tools/consumer_probe.py` | `RELEASING.md` names none of them. `tools/README.md:38`-`:43` describes each one. The `docs/README.md` row for 0054's PLAN records "publication 10 of 10 and the consumer at the floor" | **missing, verified; not in the survey** |
| M8 | the survey's name-level rows B2 to B6 and B8: 36 hand-written names of the root package, 13 of `acptest`, 4 of `clienthost`, 1 of `remote`, and 403 and 28 generated names | names that appear as a word in no doc | The owner's standard does not require a name to appear in prose, so none of these rows is a defect as it stands. Each one may point to a feature that no doc describes. For example, `README.md`'s "Errors on the wire" names `NewResourceNotFound` and none of the other seven constructors in `errors.go:310`-`:347` | **lead** |
| M9 | `opencode.json` | an agent harness's configuration file, with one key | described nowhere. Whether a harness file is a "settings key" under the owner's standard is a judgment for the PLAN's phase 1 | **lead** |
| S1 | `docs/README.md`, "I want to…" | rows that point to records | Of the 47 MADRs, 31 have no row: 0013 to 0015, 0017 to 0024, 0026 to 0031, 0033, 0034, 0036 to 0038, 0042, 0045 to 0050, 0052 and 0055. The survey's ranges left out 0042, and they span numbers that hold no MADR: 0016, 0025, 0032 and 0035 are reports alone. The Records table is complete, by the survey's set comparison, which this record did not run again | **stale, verified and corrected** |
| S2 | `docs/reports/` | frontmatter | 0 of the 48 reports have it. Their index rows say `observed` | **verified** |
| S3 | `docs/reports/*-REPORT-release-notes-*` | one report per tag | All 38 reports match a tag from `v0.15.0` to `v1.36.0`. `0019-REPORT-release-notes-v0-18-0.md` holds `v1.14.0`'s notes under the name of its draft, as its first lines say. The 15 tags `v0.4.3` to `v0.14.0`, two `-mcr` prereleases among them, have no report. `go.mod:7` retracts `v1.13.0`, which has no tag here | **verified; no defect, but a fact the changelog must handle** |

**Doc comments, measured for this record.** A lexical scan of every tracked non-test `.go` file found 0 exported
top-level declarations without a doc comment, in every package, hand-written and generated alike. The scan covered
functions, methods on exported receivers, types, and grouped or single constants and variables. That agrees with two
gates that already exist: revive's `exported` rule in `.golangci.yml`, which covers generated files
(`exclusions.generated: disable`), and `TestExportedDeclarationsDocumented` in `docs_test.go:22` for the root package.
The scan was tested on a copy outside the tree. With the comment removed above a method in `refusals.go` and above a type
in `types_gen.go`, it reported both, `refusals.go:21 method ClientRefusals.CompleteElicitation` and
`types_gen.go:18 type AgentAuthCapabilities`, with the copy's line numbers. Neither gate covers exported struct
fields or the methods of exported interfaces. Of those, the scan found these with no comment of their own:

| Package | Source | Exported fields and interface methods | With no comment of their own |
| :--- | :--- | ---: | ---: |
| root `acp` | hand-written | 43 | 37 |
| root `acp` | generated | 1,203 | 70 |
| `unstable` | generated | 56 | 2 |
| `acptest` | hand-written | 18 | 13 |
| `agentproc` | hand-written | 14 | 2 |
| `clienthost`, `remote`, `internal/tailbuf` | hand-written | 15 | 0 |
| `cmd/generate/internal/*` | hand-written | 301 | 150 |

The 72 generated ones are the discriminator fields of union variants, such as `AuthMethodTerminalInline.Type` and
`ContentBlockText.Type`. The schema gives them no description. Many of the hand-written ones are explained in their
type's doc comment. `RequestError`'s comment, for example, explains its cause and not its `Code`, `Message` or
`Data`. `CompactionEntity`'s comment describes each of its fields. The scan is lexical: it does not see the fields of
types declared in a `type ( … )` group, and it counts only the first name of a field line that declares several. The
PLAN measures again with `go/ast`.

## Decision Drivers

* **The mandate:** docs that are accurate, comprehensive and current, because they are hard to fix once they fall
  behind.
* **Users learn the SDK from the docs:** `README.md` (1,927 lines), `docs/architecture.md` (1,469 lines), and the doc
  comments that pkg.go.dev shows for each tagged version.
* **A standard someone can check:** "comprehensive" needs a definition a reader can test without opinion.
* **The cost of upkeep:** every check added is maintained in every repository. The owner chose to rely on the docs
  step in each brief instead (C9).
* **Releases stay unblocked:** the docs gaps already present must not hold up a tag. The gaps a release creates must
  not ship with it.
* **One release history:** the release-notes reports are cited by filename across the records and are each release's
  body. A changelog has to sit beside them, not replace them.

## Considered Options

The owner was given four questions, each with three options. The chosen option is marked.

**What "comprehensive" requires**

* C1. "Every name in a reference page".
* C2. **"Doc comment plus feature docs"** (chosen): every exported name has a doc comment, and every package, command,
  flag, environment variable, settings key and make target is described in a README, guide or reference. The prose
  does not have to list every name.
* C3. "Feature docs only".

**The changelog**

* C4. "Keep each repo's current practice".
* C5. **"CHANGELOG.md in every repo"** (chosen): one changelog file per Go repository, updated in the same phase as the
  change. acp-go-sdk2 keeps its release-notes reports, and the changelog links to them.
* C6. "Release-notes reports everywhere".

**How the docs stay current**

* C7. "Add checks to each gate".
* C8. "Checks in the three release repos only".
* C9. **"Fix the docs, no new checks"** (chosen): bring the docs current, and rely on the docs step in every brief to
  keep them so.

**How docs work orders against a release**

* C10. "All fixes before each tag".
* C11. "Tags first, docs after".
* C12. **"Release-caused fixes before each tag"** (chosen): before a tag, fix every statement that release makes false
  and document what it ships. Older gaps go into the repository's docs plan and land after the tag.

## Decision Outcome

Chosen options: "Doc comment plus feature docs" (C2), "CHANGELOG.md in every repo" (C5), "Fix the docs, no new checks"
(C9) and "Release-caused fixes before each tag" (C12). The owner chose them on 2026-11-14, and the coordinator relayed
them. The owner's reasons were not relayed with the choices. The reasons below are what the evidence shows about
each choice.

* C2 gives a standard someone can check. Exported top-level declarations already meet it, under revive's `exported`
  rule and `docs_test.go`. What is left is fields and interface methods (Open Question 4) and the feature-level gaps M1
  to M4 and M7.
* C5 gives this repository the changelog file the owner chose for every Go repository, and keeps the 38 reports that
  many records cite.
* C9 adds nothing to maintain. That makes the docs step, carried by the coordinator skill and each brief, the only
  thing that keeps the docs current (Consequences; Open Question 8).
* C12 keeps a release from shipping docs it makes false, and stops the backlog this record found from holding up a tag.

*Approved 2026-11-14 by the owner, relayed by the coordinator.* The owner said: "Approved to proceed with the docs
update plan for acp-go-sdk2". Earlier the same day, asked to approve the three docs plans, the owner had answered "Not
yet", and this pair stayed `proposed` while the owner read it. The approval follows that reading and covers this
repository's pair alone. From it, this record is `accepted` and its PLAN is `in-progress`.

### What each choice means here

* **D1, the standard.** A defect is one of:
  * a statement in a tracked doc or doc comment that the code at the same commit contradicts;
  * an exported name with no doc comment, with "name" scoped by Open Question 4;
  * a package, command (including `example/*`, `cmd/generate`, `cmd/generate/internal/mutate`, `tools/*.py` and
    `scripts/*`), flag, environment variable, settings key or Make target or variable that no README, guide or
    reference doc describes.

  Here "reference" means `README.md`, `docs/architecture.md`, `RELEASING.md`, `tools/README.md`, `AGENTS.md`, a
  package's `doc.go`, or a guide under `docs/guides/` if one is created.

  *Annotated 2026-11-15, by the owner's ruling relayed by the coordinator.* Asked what counts as describing a
  flag, the owner chose "Meaning and default for each": "Every flag gets at least a sentence saying what it does,
  with its default, where its reader would look. Adds the 15 rows to the missing-feature phase. This matches what
  you ruled for go-tui-lib ('at least one sentence, with its default'), so the two repositories hold one
  standard." Rejected: "Strict for user-facing commands only"; "Named is enough". So for a flag, "describes" in
  the third bullet above means that sentence and that default, in a reference doc or the command's package comment.
  A flag that is named in a usage line or a list, and has no sentence of its own, is undescribed. A flag with no
  default (one that is required, or a mode) needs the sentence alone. The PLAN's phase 3 lists the rows.
* **D2, the changelog.** `CHANGELOG.md` at the root. It has an `Unreleased` section that each phase adds to, in the
  same commit as its change. A release turns `Unreleased` into the version and its date, and links that version's
  release-notes report. The report stays where it is, keeps its number, and is still the source of the GitHub
  release's body ("Release text"). How far back the changelog starts is Open Question 1, answered 2026-11-14: from
  `v1.36.0`, with one pointer to the older reports.
* **D3, no new checks.** The PLAN adds no gate, CI job or check script. A one-off audit command that a phase runs to
  prove its own work is verification. It is run from a scratch directory, and its output is recorded in the PLAN's
  execution record. It is not committed. The gates that exist stay as they are: revive `exported`, `docs_test.go`,
  `TestGeneratedFilesUpToDate`, `check_records.py`, markdownlint and `check_disclosure.py`.
* **D4, ordering.** No code has changed since `v1.36.0`, so no statement has yet been made false by a release that has
  not shipped. Everything the PLAN fixes is an older gap. The PLAN cuts no tag. If a release is prepared while it is
  in progress, that release's own PLAN does its release-caused fixes and its changelog entry before its tag, and this
  PLAN's remaining phases land after that tag, checked again against the new HEAD.

  *Timing, answered 2026-11-14 by the owner, relayed by the coordinator.* The coordinator put this question once for
  the three docs plans. The owner chose "Yes, docs-only phases now": "Phases that change only Markdown run now, one at
  a time, and pause at a phase boundary if a release step starts. Doc-comment phases that edit Go files wait where code
  is about to be rewritten." No tag is waiting in this repository. The PLAN therefore runs its Markdown-only phases
  first and its Go phases last.
* **D5, the docs step.** Every later brief and PLAN in this repository that changes code carries a docs step. It
  names the docs the change makes false or newly needs, including doc comments and `CHANGELOG.md`'s `Unreleased`,
  changes them in the same phase, and records in the execution record which docs changed, or that none needed to.

### Open Questions

Each question lists its options, with the recommended one first. They are the owner's to answer. The PLAN's phases
that depend on an answer say so. Questions 1 to 7 and 9 were answered by the owner on 2026-11-14, and question 8 was
settled the same day by the coordinator's application of an owner's ruling. Each is annotated below. No question is
open.

1. **How far back `CHANGELOG.md` starts.**
   * Recommended: (b) one line per past tag from `v0.15.0` to `v1.36.0`, 38 tags, each with its date and a link to its
     report. One line covers the 15 tags before `v0.15.0`, which have no report. `v1.14.0` links
     `0019-REPORT-release-notes-v0-18-0.md`, and `v1.13.0` is noted as retracted. This is mechanical and gives a
     reader the whole history from one file without copying the reports.
   * (a) from `v1.36.0` forward, with one line that points to the release-notes reports in `docs/README.md` for
     everything before it. This is the least work, but the file says almost nothing on its first day.
   * (c) a full backfill: a summary of each report in the changelog. This duplicates 38 reports, and the two copies
     can then disagree.

   *Answered 2026-11-14 by the owner, relayed by the coordinator.* The owner chose "Latest tag forward": "An
   Unreleased section and one section for the latest tag, then a single pointer to where older notes live (tui's
   migration guide, acp's release-notes reports, magic's tags and records). Least work, nothing duplicated; older
   releases are one link away." Rejected: "Plus one line per older tag" (described to the owner as 38 lines for this
   repository) and "Full backfill". This is option (a), not the recommended (b). `CHANGELOG.md` has an `Unreleased`
   section, a `v1.36.0` section, and one pointer to the release-notes reports indexed in `docs/README.md`. The pointer
   states the three facts of S3 in one sentence, without a list per tag: `v1.14.0`'s notes are filed as
   `0019-REPORT-release-notes-v0-18-0.md`, `v1.13.0` is retracted (`go.mod`), and the tags before `v0.15.0` have no
   report.
2. **The "I want to…" matrix in `docs/README.md`** (S1).
   * Recommended: (b) redefine it as one row per task a reader comes with, each pointing to the current record or doc
     for that task, and state in `docs/README.md` that the Records table is the per-record index. Superseded
     decisions stay reachable through the Records table. A record that adds no new task adds no row. This is the
     `documentation-writing` skill's form, which asks for rows "phrased as the task in the reader's words" and does not
     ask for one per record.
   * (a) complete it: 31 rows, one per MADR that has none, and from now on a row for every new record.
   * (c) leave it, and say in `docs/README.md` that the matrix is a selection.

   *Answered 2026-11-14 by the owner, relayed by the coordinator.* The owner chose "Row per reader task": "One row per
   task a reader comes with, each pointing to the current record or doc for it. The Records table stays the complete
   per-record index. A record that adds no new task adds no row, so the table stops growing with every record."
   Rejected: "Row per record"; "Leave it, labelled". This is option (b).
3. **Frontmatter on the 48 reports** (S2).
   * Recommended: (c) leave them as they are. A report decides nothing, its status is always `observed`, the line
     that opens each one gives its date, and nothing reads report frontmatter.
   * (a) add `status: observed` and `date:` to all 48, the date taken from each report's opening line. This is
     mechanical, one commit, and changes 48 files that records cite.
   * (b) add frontmatter only to new reports from now on. This leaves the set inconsistent.

   *Answered 2026-11-14 by the owner, relayed by the coordinator.* The owner chose "Leave them as they are": "Change
   none of the 48. Nothing reads report headers, and the date is already on each report's first line." Rejected: "New
   reports only"; "Add to all 48". This is option (c).
4. **Which exported names need a comment of their own.** Top-level declarations already have one everywhere. What is
   open is the exported fields and interface methods.
   * Recommended: (b) every exported field and interface method of an exported type, in every package, counts as
     documented when it has its own comment or when its type's doc comment names it and says what it holds. This
     follows Go's doc-comment guidance, under which a type's comment may explain its fields. Measured above at
     roughly 274 members before that allowance. The PLAN's phase 1 counts how many the allowance clears.
   * (a) only what revive's `exported` rule and `docs_test.go` already cover: top-level declarations and methods.
     Phase 4 is then a confirmation, with no edits.
   * (c) every exported field and interface method has its own comment, whatever its type's comment says.

   *Answered 2026-11-14 by the owner, relayed by the coordinator.* The owner chose "Fields of public and contract
   types": "Top-level names, plus exported fields and interface methods of public packages and of the types users
   configure or receive (wire and config structs). A field counts as documented when its type's comment names it."
   Rejected: "Top-level names only"; "Every exported identifier". This is option (b). The scope is therefore the
   public packages (the root `acp`, `unstable`, `acptest`, `agentproc`, `clienthost` and `remote`) and the
   wire and config types they expose. `internal/*` and `cmd/generate/internal/*` are out of scope. Before the
   type-comment allowance, the scan above counts 124 members in scope: 37 hand-written in the root package, 13 in
   `acptest`, 2 in `agentproc`, and 72 generated. The owner was told that the 72 generated fields need a generator
   change, which question 5 asks about separately. Question 5 stays open.
5. **The 72 generated discriminator fields** (only if question 4 is answered (b) or (c)). A generated file is never
   edited by hand: it says "DO NOT EDIT", and `TestGeneratedFilesUpToDate` (`cmd/generate/main_test.go:46`) fails when
   it differs from the generator's output.
   * Recommended: (a) the generator writes a one-line comment on each discriminator field, such as `Type is always
     "terminal".`. This changes `cmd/generate/internal/emit`, the regenerated `*_gen.go` files and the engine goldens.
     No API changes. It is a code change, so full CI runs.
   * (b) the generator writes a sentence about the discriminator in the variant type's doc comment, which already says
     which variant the type is (`types_gen.go:739`). Same cost as (a).
   * (c) exempt generated code from question 4. The schema gives these fields no description.

   *Answered 2026-11-14 by the owner, relayed by the coordinator.* The owner chose "Name it in the type's comment": "The
   generator adds a sentence about the field to each variant type's existing comment. This meets your rule that a field
   counts when its type's comment names it, with the smallest change to generated code. No API change; full CI runs."
   Rejected: "Comment on each field" (the recommended option) and "Exempt generated code". This is option (b).

   *Annotated 2026-11-15, by the owner's ruling relayed by the coordinator.* Phase 1's defect list found two fields
   that option (b)'s discriminator sentence does not reach: `Negotiation.Request` and `Negotiation.Response`
   (`types_gen.go:25584`-`:25585`). They are not discriminators, and their type's comment describes them in lower
   case, so the audit's word match does not count them. The owner chose "Name them in the type's comment": "Amend
   the generator phase so the generator names both fields in Negotiation's comment, the same treatment you chose for
   the discriminator fields. Generator text of its own, in files already in that phase's list; no API change. The
   audit then reaches zero by your rule that a field counts when its type's comment names it." Rejected: "A comment
   on each field"; "Count the existing prose". So phase 4b's generator change is two things: the discriminator
   sentence on each variant type's comment, and a sentence in `Negotiation`'s comment that names `Request` and
   `Response`. It is generator text of its own, not schema text, and changes no API. The PLAN's phase 4b records
   what that means for its steps and its acceptance.
6. **How `CHANGELOG.md` is routed and link-checked.** Today a root `CHANGELOG.md` is not docs-safe: a commit that
   changes it alone runs CI's six jobs, and `make pre-add-check` runs every Go check for it. `check_records.py` checks
   the links of a fixed list of unnumbered docs (`scripts/check_records.py:239`), which would not include it.
   * Recommended: (b) add `CHANGELOG.md` to the docs-safe paths in `ci.yml` (both `paths-ignore` lists) and `docs.yml`
     (both `paths` lists), to `AGENTS.md`'s list of those paths, and to `UNNUMBERED_DOCS`. This adds a path to
     existing lists. It adds no gate, job or script. The owner should still confirm this is consistent with C9.
     `scripts/check_ci_filters.py` already requires the two workflows to agree.
   * (a) add it to the docs-safe paths only, and leave its links unchecked.
   * (c) change neither. Every changelog-only commit then runs full CI, and its links go unchecked.

   *Answered 2026-11-14 by the owner, relayed by the coordinator.* The owner chose "Docs path and link check": "Add
   CHANGELOG.md to both workflows' docs path lists, to AGENTS.md's list of those paths, and to the list of docs whose
   links the records tool checks. No new check: an existing link check reads one more file, and changelog-only commits
   run the light Docs workflow instead of the Go suite." Rejected: "Docs path only"; "Change nothing". This is option
   (b). It is the owner's own reading of "Fix the docs, no new checks" (C9) for this case, in the owner's words above. A
   changed list is unused until it has been seen to work, so the PLAN's phase 5b shows on a scratch copy that the link
   check fails on a deliberately broken link in `CHANGELOG.md` and passes on the real file, and that
   `check_ci_filters.py` fails when only one workflow's list is changed.
7. **Where new feature docs go.**
   * Recommended: (a) user-facing features (M4, and the M8 leads that phase 1 confirms) go in `README.md`, beside the
     features already described there. Maintainer items (M1 to M3) go in `docs/architecture.md` and
     `tools/README.md`. No `docs/guides/` tree is created by this PLAN.
   * (b) start `docs/guides/` with a reference guide for the build and tooling (`make` targets, variables, flags), and
     keep `README.md` for the SDK's users.

   *Answered 2026-11-14 by the owner, relayed by the coordinator.* The owner chose "Existing files": "Features an SDK
   user meets go in README.md beside those already described. Maintainer items go in docs/architecture.md and
   tools/README.md. No new docs folder is created." Rejected: "A new build guide". This is option (a).
8. **The docs step written down.** D5 relies on briefs. Agents that work without a brief from the coordinator read
   `AGENTS.md`.
   * Recommended: (a) the PLAN's phase 5 adds a short "Docs in the same change" section to `AGENTS.md`, stating D2's
     `Unreleased` rule and D5's docs step. This is rule text, not a check.
   * (b) leave `AGENTS.md` as it is, so briefs carry the step alone.

   *Settled 2026-11-14 by the coordinator, applying an owner's ruling; not put to the owner for this file.* On the same
   question for the global rules file, the owner ruled "Skill only": the docs rule lives in the coordinator skill and
   in briefs, not in a rules file. The coordinator applied that ruling to the per-repository files, including this
   repository's `AGENTS.md`, and told the owner so. The owner has not ruled on per-repository files specifically, and
   a contrary word from the owner replaces this. Until then, option (b) applies. `AGENTS.md` gains no docs-step
   section. Factual corrections to `AGENTS.md` stay in scope.
9. **Whether the fixes reach pkg.go.dev before the next code release.** pkg.go.dev shows the README and the doc
   comments of a tagged version, so phases 2 to 4 reach it only with a tag.
   * Recommended: (a) no tag of this PLAN's own. The next release carries the fixes, and its changelog entry lists
     them. Recent tags came days apart: `v1.34.0` 2026-11-05, `v1.34.2` 2026-11-07, `v1.35.0` 2026-11-10 and `v1.36.0`
     2026-11-11, by their tag dates.
   * (b) a patch tag once this PLAN ends, with its own release-notes report and changelog entry. `make api` would
     report no change.

   *Answered 2026-11-14 by the owner, relayed by the coordinator.* The owner chose "No release of its own": "The next
   code release carries the fixes and its changelog entry lists them." Rejected: "Docs-only patch tag". This is option
   (a).

### Consequences

* Good, because the defects found so far get fixed: one wrong statement (W1), seven `make` targets and four Make
  variables (M1, M2), the flags (M3), connection logging (M4), the release tools in `RELEASING.md` (M7) and the
  matrix (S1).
* Good, because a reader gets one file, `CHANGELOG.md`, that lists the releases and points to each report. The fleet's
  Go repositories then share that shape.
* Good, because no new check is added, so there is no new gate to maintain, debug or keep in step across
  repositories.
* Bad, because without a check, drift is caught only when an agent follows its docs step or someone reads the docs.
  M1, M2 and M7 had gone unnoticed until a survey looked for them.
  The existing gates cover links, Markdown style, disclosure, the root package's top-level doc comments, revive's
  `exported` and the README examples that `docs_test.go` compiles. They do not catch a false sentence.
* Bad, because the README is not docs-safe (`docs_test.go` reads it). Each phase that changes it runs
  `make pre-add-check`'s full Go checks, which are a heavy run.
* Neutral, because the release-notes reports keep their role: they are still numbered records, indexed in
  `docs/README.md`, and the source of the release body. `CHANGELOG.md` adds an index over them and the `Unreleased`
  section before a release.
* **`RELEASING.md` changes** in the PLAN's phase 5a:
  * "Release notes and the API": "The changelog is the release notes report" becomes false. It is replaced by:
    `CHANGELOG.md` is the changelog; each change adds its line under `Unreleased` in the same phase; the release's
    PLAN turns `Unreleased` into the version and date, with a link to the release-notes report; and the report's
    "Release text" is still the GitHub release's body.
  * "Tag and Publish": step 3 names `tools/release_body.py VERSION REV` as the source of the body, and the checks
    after the tag that the recent release PLANs ran (`tools/watch_ci.py`, `tools/publication.py`,
    `tools/consumer.py`), as they are practised today, by hand.
    [0048-MADR-create-releases-in-ci.md](0048-MADR-create-releases-in-ci.md) is still `proposed`, and `RELEASING.md`
    does not describe it.
  * "Additional Notes": "update examples and docs in the same commit" is widened to D5's docs step, and to the
    changelog line.
* Neutral, because `docs/architecture.md`'s "Tree" section (`:1414`) lists the root files, so it gains
  `CHANGELOG.md`.

### Confirmation

There is no new check, so the docs are known to be current from records and from commands anyone can re-run:

* The PLAN's execution record holds, for each phase, the one-off audit that phase ran, with its output at the phase's
  commit: zero remaining defects of that phase's class, out of the list phase 1 produced. Phase 1's list is a report,
  numbered 0056, that names every defect with its evidence and the phase that fixes it.
* `CHANGELOG.md` has a heading for every tag from its starting point (Open Question 1) to the newest, and an
  `Unreleased` section. `git tag --list 'v*'` compared with its headings shows no tag missing.
* Every PLAN in this repository after this one has a docs step and an execution-record line saying which docs changed.
  A PLAN without one is incomplete under D5.
* The gates that exist still pass: `make check-records`, `make markdownlint`, `make disclosure-check`, and the
  `docs_test.go` and lint steps of `make pre-add-check`.
* Re-running the survey's method, or phase 1's, on a later commit is how a reader measures drift. It is not decided
  here that anyone does that on a schedule.

## Pros and Cons of the Options

### C1. Every name in a reference page

* Good, because a name search finds every exported name.
* Bad, because about 1,650 exported top-level names in the public packages, 1,288 of them generated, would need prose,
  by this record's scan, and every schema bump would change them.

### C2. Doc comment plus feature docs

* Good, because doc comments are where Go users read about names, and top-level declarations already meet it.
* Good, because feature docs can be checked against an inventory of packages, commands, flags, variables and targets.
* Bad, because whether a feature is "described" takes judgment where a name search would not.

### C3. Feature docs only

* Good, because it is the least work.
* Bad, because it leaves doc comments out of the standard, although pkg.go.dev shows them for every release.

### C4. Keep each repo's current practice

* Good, because nothing changes.
* Bad, because this repository has no changelog file, and the fleet's repositories stay unlike each other.

### C5. CHANGELOG.md in every repo

* Good, because a reader has one file per repository, and this repository's reports are kept and linked.
* Bad, because there are now two places to update for a release, the changelog and the report.

### C6. Release-notes reports everywhere

* Good, because this repository would not change.
* Bad, because the other repositories would take on numbered reports for every release.

### C7. Add checks to each gate

* Good, because drift would fail a gate as it happens.
* Bad, because every repository maintains new checks, and a check catches missing names more easily than false
  sentences.

### C8. Checks in the three release repos only

* Good, because the cost is limited to where releases happen.
* Bad, because the repositories would differ, and the other repositories drift unchecked.

### C9. Fix the docs, no new checks

* Good, because there is nothing new to maintain.
* Bad, because keeping the docs current rests on discipline: the docs step in briefs and PLANs.

### C10. All fixes before each tag

* Good, because every release ships with docs that are fully current.
* Bad, because a backlog like the one found here holds up a release, a security release included.

### C11. Tags first, docs after

* Good, because releases are never held up.
* Bad, because a release can ship docs it has just made false.

### C12. Release-caused fixes before each tag

* Good, because a release never ships a statement it made false, and older gaps do not hold it up.
* Neutral, because older gaps need their own plan, which is this record's PLAN.

## More Information

* **Related records:**
  [0010-MADR-align-docs-with-current-tree.md](0010-MADR-align-docs-with-current-tree.md), the last whole-tree docs
  alignment; [0024-PLAN-documentation.md](0024-PLAN-documentation.md), which added the doc-comment tests in
  `docs_test.go`; [0037-MADR-align-docs-ci-and-disclosure-gates.md](0037-MADR-align-docs-ci-and-disclosure-gates.md),
  the docs-safe paths and the Docs workflow;
  [0048-MADR-create-releases-in-ci.md](0048-MADR-create-releases-in-ci.md), still `proposed`, which would change
  "Tag and Publish".
* **Other repositories:** the mandate covers every related repository. Each one gets its own record and plan. This
  record decides nothing for them.
* **Not verified here:**
  * the prose of `README.md` and `docs/architecture.md` (W3), which phase 1 reads through;
  * whether the features behind the M8 names are described without naming them;
  * the survey's index and status comparison (its C1 and C2), not run again;
  * the survey's check of the "Tree" section, which looked at paths and not at their descriptions;
  * the context of the `-skip`, `-only` and `-timeout` hits (M3);
  * the fields that the lexical scan cannot see (Context);
  * `.claude/rules/`, `.codex/rules/`, `.grok/rules/`, `.kilo/agent/` and `.opencode/rules.md`, beyond
    `.claude/rules/madr-and-plan-skill.md`.

  No build, test or Go lint was run for this record.
* **The PLAN** is [0056-PLAN-bring-the-docs-current-and-keep-them-current.md](0056-PLAN-bring-the-docs-current-and-keep-them-current.md).
