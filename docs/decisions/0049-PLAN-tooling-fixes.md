---
status: in-progress
date: 2026-11-04
associated-madr: "0049-MADR-remediate-the-2026-11-01-debugging-pass.md"
---

# Implement 0049, the tooling: T1 to T14 and F-C

Associated MADR: [0049-MADR-remediate-the-2026-11-01-debugging-pass.md](0049-MADR-remediate-the-2026-11-01-debugging-pass.md)

## Goal

Fix the tooling findings of the MADR's F-T section, T1 to T14, the parts of T10 and T13 still open, and F-C's coverage gaps
and dead code, as the MADR states them and as its "Re-assessment of F-T and F-C at `1fe3dd0`" corrects them. The MADR's
"The tooling PLAN's designs, proposed 2026-11-04" puts the four details this PLAN needs, Q16 to Q19, answered by the owner on
2026-11-04: (a) each, the recommended ones. Each fix lands with its finding's probe as
a committed test, seen to fail on the code it replaces and to pass after. This is the fourth of the MADR's five PLANs (Q1 a,
Q7 a). It changes no exported API and no consumer-visible behaviour, so it has no release of its own; the next release's
notes name it.

Base: `1fe3dd0`, whose Go code is `v1.33.0`'s. Every file and line below is at that commit; the commit that adds this PLAN
changes records only. The MADR's re-assessment at `1fe3dd0` shows every finding below reproducing there.

## Scope

In scope:

* **`tools/`:**
  * `publication.py`: `main_run` (`:32-37`), "CI on origin main's head" and "CI on the tag" (`:82-90`), the Go environment
    (`:103-105`);
  * `ci_jobs.py`: `run_list` (`:183-191`);
  * `consumer.py`: the Go environment (`:492-494`);
  * `release_body.py`: the scan's report (`:77-81`);
  * `idscan.py`: `--range` (`:82-87`);
  * `find.py`: a missing or directory path (`:34-36`);
  * `_common.py`: a new helper for the Go environment;
  * `test_tools.py`, and `tools/README.md`'s rows for the tools that change.
* **`scripts/`:** `check_records.py` (`markdown_files`, `:59-66`; `unnumbered_docs`, `:221-229`); `check_disclosure.py`
  (`:88-90`); a new `test_check_records.py`.
* **Configuration:** `.markdownlint-cli2.jsonc` (Q17 a); the `Makefile` (`:12`, `:37`, `:106`, the targets' `##` help);
  `.github/workflows/ci.yml`'s comments (`:36`, `:102`); `.golangci.yml`'s dead exclusions (`:113-116`, `:125-127`, `:131`,
  `:144`, and `third_party$` and `builtin$`).
* **F-C:**
  * the generator, `cmd/generate/internal/model` and `internal/emit`, where `gates` is emitted, and the regenerated
    `types_gen.go`;
  * `agentproc/agentproc.go`'s `proctree.Attach` call (`:242`), for a seam a test can fail;
  * tests in `agentproc` and the root package.
* **Docs:** `docs/architecture.md`'s tools and checks; `RELEASING.md` where a tool's behaviour it states changes.
* **A record:** [0048-PLAN-create-releases-in-ci.md](0048-PLAN-create-releases-in-ci.md), amended after this PLAN's phase 1
  to build on it (Q19 a).

Out of scope:

* the generator's findings, G1 to G13, and its two `main` bodies, which are the MADR's generator PLAN;
* executing 0048, which stays `proposed` until the owner accepts it;
* any change to an exported Go API or to what the module does at run time;
* the 1.26.0 schema.

## Design rules

1. **A check checks what its name says.** Each fix makes a check's output name what it read: a run's workflow and commit, a
   file's path, the scan's reason.
2. **Fail loudly.** A tool that cannot do its work exits non-zero with the reason on stderr, never with an empty output or a
   status a caller reads as a result (T5, T6, T11).
3. **No new dependency** ([0031-MADR-verification-tools-in-repository.md](0031-MADR-verification-tools-in-repository.md)):
   stdlib Python, `git`, `gh` and `go` only.
4. **The tests need no host or network,** as `make check-tools` runs them in CI's records step: `gh` and `git` are faked or
   run against a temporary repository, as `WatchCiTest`'s `FakeApi` does (`tools/test_tools.py:483`).
5. **Every new check is seen to fail** on a planted break in a scratch copy, by `tools/plants.py` where it runs the test, and
   by script where the test is Python (`plants.py` runs `go test` only, `tools/plants.py:83-90`).

## Implementation Steps

Every phase ends staged for the owner, who commits. A phase begins only on the owner's word. Every phase runs `make
check-tools`, `make check-records`, `make markdownlint` and `make disclosure-check`, and `make pre-add-check` with its files
when one is Go or read by Go. The phases follow the MADR's order, T2, T1, T3, then T4 to T14, grouped by tool.

### Phase 1: T2, T14 and T7, which run is CI's (Q16 a, Q19 a)

1. **`publication.py`'s CI checks read the CI workflow's runs.**
   * A new `ci_run(commit)` lists `gh run list --repo R --commit C --workflow ci.yml --json
     conclusion,status,headBranch,headSha,databaseId --limit 20`, from any ref, and takes the newest.
   * "CI on origin main's head" asks `ci_run` of main's head. When the head has none, it walks main's first-parent history
     back, at most 50 commits, to the newest commit `X` with a CI run such that every path changed in `X..head` is docs-safe,
     as `scripts/check_ci_filters.py --docs-safe` answers it; and checks that run. Its detail names the run, its commit, and
     "docs-safe since" when it is an ancestor's.
   * "CI on the tag" lists the tag's runs with `--workflow ci.yml` too.
   * `main_run` goes, and its doc's claim that "the tag's run is listed first" with it.
   * The selection is a pure function of injected `runs_of(commit)`, `ancestors()` and `docs_safe(paths)`, so its tests need
     no network.
2. **`ci_jobs.py`'s `run_list`** passes `--workflow ci.yml` (T14), so `tally` and `failure` read CI's jobs alone, and its
   help says so.
3. **`ci.yml:36`'s comment** says what the concurrency group does (T7): "a newer push to a pull request cancels its older
   run; on main a running push finishes, and of the pushes queued behind it only the newest runs". `docs.yml`'s block gains
   the same comment.
4. **Tests,** `tools/test_tools.py`:
   * `PublicationTest.test_ci_run_of_main`: a head with the tag's CI run; a docs-only head whose parent has one ("docs-safe
     since"); a head whose ancestor has one but with a Go file changed since (no run, so the check fails); a Docs run alone
     (not counted); the command carrying `--workflow ci.yml`.
   * `PublicationTest.test_ci_run_of_tag`: the tag's CI run, and a tag with none.
   * `test_main_run_of_a_released_commit` (`:476-480`) goes with `main_run`; the record names it.
   * `CiJobsTest`: `run_list`'s command carries `--workflow ci.yml`.
5. **A real run:** `tools/publication.py v1.33.0 6e04da6 <the body>` on the live repository, where main's head has moved past
   the tag by docs-only commits: "CI on origin main's head" names the tag's CI run `41890673918`, docs-safe since, not a Docs
   run; the record quotes it.
6. **Docs:** `tools/README.md`'s `publication.py` and `ci_jobs.py` rows; `docs/architecture.md`'s tools paragraph.
7. **Seen to fail,** by script, the tests run in a copy per plant: the workflow filter removed; the ancestor walk ignoring
   the docs-safe rule; `ci_jobs.py` without `--workflow`.
8. **0048 (Q19 a):** after this phase is committed, [0048-PLAN-create-releases-in-ci.md](0048-PLAN-create-releases-in-ci.md)
   gains a dated amendment: its `--run-id` (D5) stands for `ci_run`'s answer on the tag, and its `release_body.py` change
   (D3) builds on phase 3's. A record change only; 0048 stays `proposed`.

### Phase 2: T1 and T3, what the record checks read (Q17 a)

1. **T1.** `unnumbered_docs` (`scripts/check_records.py:221-229`) adds `RELEASING.md`, `tools/README.md` and `AGENTS.md`, so
   their relative links are checked by `make check-records`, which the Docs workflow runs. At `1fe3dd0` their links all
   resolve (the re-assessment's clean-tree run, exit 0).
2. **T3.**
   * `markdown_files` lists `git ls-files -co --exclude-standard -- '*.md'`, the tracked files and the untracked ones git does
     not ignore, `.git/info/exclude` included; outside a work tree, as in a `git archive` copy, it falls back to `rglob` and
     `SKIP_PARTS`, as now.
   * `.markdownlint-cli2.jsonc` gains `"gitignore": true`. It is the fleet's shared configuration: the record says so, and the
     line is offered upstream with the PLAN's close.
3. **Tests,** a new `scripts/test_check_records.py`, which `make check-tools` discovers (`Makefile:25`):
   * a broken link in `RELEASING.md`, in `tools/README.md` and in `AGENTS.md` each fails `--check-all`;
   * a record copied under an ignored `.worktrees/` neither moves `--next` nor claims a number twice, in a temporary
     repository;
   * outside a work tree, the files are still found.
   * `ReadmeTest` (`tools/test_tools.py:592`) stays: it checks `tools/README.md`'s tool list, not only its links.
4. **A real run:** `make markdownlint` on the tree lints 47 files, not 48; `make check-records` passes.
5. **Docs:** `docs/architecture.md`'s records paragraph; `AGENTS.md`'s "Records" bullet names the files `make check-records`
   checks.
6. **Seen to fail,** by script: `unnumbered_docs` without the three; `markdown_files` back to `rglob`.

### Phase 3: T4, T11, T5 and T6, tools that fail loudly

1. **T4.** `_common.go_env(extra)` returns the environment both tools give `go`: `GOENV=off`, `GOSUMDB=sum.golang.org`,
   `GOPROXY=https://proxy.golang.org,direct`, and `GONOSUMDB`, `GONOSUMCHECK`, `GOPRIVATE`, `GONOPROXY`, `GOINSECURE`,
   `GOFLAGS` and `GOEXPERIMENT` removed, then the caller's own. `publication.py` and `consumer.py` use it, and each runs
   `go env -json GOENV GOSUMDB GONOSUMDB GOPRIVATE GONOPROXY GOPROXY` under it first, and stops, exit 1, naming the variable,
   when one is not as set.
2. **T11.** `release_body.py` prints idscan's stderr and its exit status when the scan fails. **T11b.**
   `check_disclosure.py` prints git's stderr when `git ls-files` fails, and says it needs a work tree.
3. **T5.** `idscan.py --range A..B` scans the lines each commit of `git log -p --format=commit %H A..B` adds, and reports
   `commit:path:line` for a match; a range without `..`, or with `...`, exits 2 naming it. `--range` keeps its one caller,
   `tools/test_tools.py:80`.
4. **T6.** `find.py lines` and `files` exit 2, on stderr, for a path that does not exist, "find.py: no such file: P", and for
   a directory, "find.py: P is a directory; use P/** for its files".
5. **Tests,** `tools/test_tools.py` and `scripts/test_check_disclosure.py`:
   * `HelpersTest.test_go_env`: a parent with `GOSUMDB=off`, `GONOSUMDB`, `GOPRIVATE` and a `GOENV` file gives none of them
     to `go`; the assertion fails on a planted value.
   * `ReleaseTest`: a failing scan's reason is printed.
   * `IdscanTest`: an identifier added and removed inside a range is found, with its commit; `HEAD` and `A...B` exit 2.
   * `FindTest`: a missing file and a directory exit 2 with the message.
   * `test_check_disclosure`: outside a work tree, the reason is printed.
6. **Docs:** `tools/README.md`'s rows for `idscan.py` and `find.py`.
7. **Seen to fail,** by script: each fix reverted in its own copy, its test failing.

### Phase 4: T9, T10, T12 and T13, the Makefile and the configuration (Q18 a)

1. **T9.** A target `schema-version-set`, run as an order-only prerequisite of `version`, `release` and the four schema
   downloads, fails with "schema/version read as empty: GNU make 4.2 or newer is needed, this is $(MAKE_VERSION)" when
   `ACP_VERSION` is empty. Under make 3.81 the docs gates keep working.
2. **T10.** `Makefile:106`'s and `ci.yml:102`'s comments name the gated packages, the root, `agentproc`, `acptest` and
   `clienthost`, and the reported ones, `unstable` and `remote`.
3. **T12.** `.golangci.yml` loses the exclusions that match nothing: the `//sys` rule, `godot`'s, `examples$` twice,
   `third_party$` and `builtin$`. `example/` stays linted: no rename to `example$`.
4. **T13.** `all`, `check-records`, `check-tools`, `clean`, `disclosure-check`, `markdownlint`, `release` and `version` gain
   `##` help, so `make help` lists them; `tools/test_tools.py` gains `PlatformsTest`, which requires `platforms.py`'s checks
   to name each `VET` target and the two lint targets, without running Go.
5. **Tests:**
   * `MakefileTest.test_schema_version_guard`: `make -n version ACP_VERSION=` exits non-zero with the guard's text, under the
     make on PATH, and `make -n check-records ACP_VERSION=` does not;
   * `MakefileTest.test_help_lists_targets`: `make help` names the eight;
   * `PlatformsTest`.
6. **Checks:** `make lint` and `tools/gates.py lint platforms`, so the smaller `.golangci.yml` reports nothing new on every
   platform; under `/usr/bin/make` 3.81, `make version` fails with the guard and `make check-records` passes.
7. **Seen to fail,** by script: the guard removed; a target's `##` dropped; a `VET` target dropped from `platforms.py`.

### Phase 5: F-C, dead code and code no test runs

1. **`ConfigOptionUpdate.gates`.** The generator emits `gates` only for a type that a method's params or result, or a field of
   such a type, reaches. The phase starts by reading where `gates` is emitted and whether the model already computes
   reachability; if it does not, and computing it is more than a walk of the types the dispatch names, the phase stops as a
   deviation. An engine case, `accept-gates-unreachable.txtar`, holds a type reached only by `allOf`, which gets no
   `gates`. Regenerated, `types_gen.go` loses `ConfigOptionUpdate.gates` and nothing else; `tools/gates.py regen` is clean.
2. **`agentproc`'s `abandon`.** `Start` calls `proctree.Attach` through an unexported package variable, `attachTree`, which a
   test replaces. `TestStartAbandonsUnattachedAgent` makes it fail: `Start` returns an error wrapping it, and the child, which
   wrote its pid to a file, has been reaped, on Unix and Windows.
3. **The two `Error` methods.** `TestStderrErrorText` (`agentproc`) and a case in the elicitation tests (the root) require
   `StderrError.Error`'s text and `defaultError.Error`'s, as `errors.As` reaches them.
4. **A coverage run,** as the re-assessment's: `abandon`, the two `Error` methods run, and `ConfigOptionUpdate.gates` is gone.
5. **Seen to fail,** by `tools/plants.py` for the Go tests and by script for the generator: `abandon` not called on an attach
   failure; each `Error` method's text changed; `gates` emitted for every type again.

### Phase 6: close

1. `tools/gates.py --all`, with the hosts named per command, and T1 to T14 and F-C listed, each with the test that shows it,
   in a table.
2. `make api` against `v1.33.0`: 0 incompatible, 0 compatible.
3. After the owner's push, CI on the head.
4. No release of its own: no exported API or run-time behaviour changes. The next release's notes name this PLAN.
5. This PLAN and its row are marked `complete`, and the MADR's T and F-C findings done.

## Verification

* Each of T1 to T7, T9 to T14 and F-C's three gaps has a committed test that fails at `1fe3dd0`'s code, by its plant, and
  passes after; T8 is fixed.
* `tools/publication.py v1.33.0 6e04da6` names the CI workflow's run for main's head.
* `make markdownlint` and `make check-records` read only what git tracks or would.
* `make check-tools`, `tools/gates.py --all` and CI pass; `make api` reports no change.

## Rollout and Rollback

* **Rollout.** Tools and checks only. What a maintainer meets: `publication.py` names a CI run where it named a Docs run;
  `make check-records` checks three more files' links; `make markdownlint` lints one file fewer locally; `idscan.py
  --range` reports commits; `find.py` exits 2 on a bad path; `make version` under make 3.81 stops with the reason.
* **Rollback.** Revert the phase's commit; each phase stands alone.

## Execution record

The owner answered Q16 to Q19 on 2026-11-04 (the MADR's "Answered by the owner, 2026-11-04", under the tooling PLAN's
designs), committed this PLAN as `4e3d9cc`, and said "committed, proceed".

### Phase 1: T2, T14 and T7, which run is CI's, 2026-11-04

Begun on the owner's "committed, proceed" after `4e3d9cc`. No deviation.

**What it did:**

* **`tools/publication.py`:**
  * `CI_WORKFLOW = "ci.yml"`; `ci_runs_command(repo, commit=, branch=)`, the CI workflow's runs of a commit from any ref, or
    of a branch or tag, newest first; `passed(run)`.
  * `ci_of_head(head, runs_of, ancestors, docs_safe)`, as step 1.1 has it: head's own CI run, or the nearest first-parent
    ancestor's whose every later change `check_ci_filters.py --docs-safe` passes, stopping at the first that does not.
    `main` gives it `gh` for `runs_of`, `git rev-list --first-parent --max-count=51` and `git diff --name-only` for
    `ancestors`, and the script for `docs_safe`.
  * "CI on origin main's head" names the run, its commit and "docs-safe since" when it is an ancestor's; "CI on the tag"
    lists the CI workflow's runs of the tag. `main_run` is gone. The module doc says what the two checks read.
* **`tools/ci_jobs.py`:** `CI_WORKFLOW` and `run_list_command`, with `--workflow ci.yml`; `run_list` uses it; the usage and
  `tally`'s summary line name the workflow.
* **`.github/workflows/ci.yml:36`** and **`docs.yml`'s concurrency block:** the comment step 1.3 gives.
* **The tests,** `tools/test_tools.py`: `PublicationTest`, with `test_ci_of_head` (the tag's run for its commit; a docs-only
  head after it, "docs-safe since"; a Go file changed since, no run; no run at all), `test_ci_runs_command` and
  `test_passed`; `CiJobsTest.test_run_list_is_ci_s`. `WatchCiTest.test_main_run_of_a_released_commit` went with `main_run`.
* **Docs:** `tools/README.md`'s two rows; `docs/architecture.md`'s release tools.

**A real run,** step 1.5, `tools/publication.py v1.33.0 6e04da6 <the body>`, `origin/main` at `1fe3dd0`, one docs-only commit
past the tag: exit 0, "10 of 10 checks hold", "ok   CI on origin main's head  run 41890673918 on 6e04da6, docs-safe since,
v1.33.0", the tag's CI run, where the old check named the Docs run `41889919614`; "ok   CI on the tag  run 41890673918,
v1.33.0".

**Seen to fail,** by script, the tools' tests run in a copy of the tree per plant, `tools/plants.py` running `go test` alone,
3/3:

```text
FAILED AS REQUIRED: publication.py's runs without the workflow filter: exit 1: ERROR: test_ci_runs_command (test_tools.PublicationTest.test_ci_runs_command)
FAILED AS REQUIRED: the ancestor walk ignoring the docs-safe rule: exit 1: (None, None), "a change that runs CI is not covered by an older run")
FAILED AS REQUIRED: ci_jobs.py's runs without the workflow filter: exit 1: ERROR: test_run_list_is_ci_s (test_tools.CiJobsTest.test_run_list_is_ci_s)
```

The first and third fail as errors, a `ValueError` where the test reads the value after `--workflow`, which is not there; a
failure either way.

**Checks:**

* `python3 -B -m unittest discover -s tools -p 'test_*.py'`: the 16 tests of the three classes `ok`; `make check-tools`: exit 0,
  37 tests, "check-ci-filters: 0 problem(s)".
* `actionlint` at CI's pin, `go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12`, on both workflows: exit 0.
* `make pre-add-check` with the phase's files, at load 24 to 33: exit 0.
* `make check-records`, `make markdownlint`, `make disclosure-check`: exit 0 each. `tools/idscan.py --paths` over the phase's
  files: 0 matches.
* `ci.yml` is not docs-safe, so the push runs CI.

Step 1.8, 0048's amendment, follows the owner's commit of this phase, with phase 2.

Staged for the owner, who commits. Phase 2 begins on the owner's word.

The owner committed phase 1 as `72a4667`.

**Step 1.8,** with phase 2's change: [0048-PLAN-create-releases-in-ci.md](0048-PLAN-create-releases-in-ci.md) gains two dated
amendments, under its phase 1's step 1, `--no-idscan` building on this PLAN's phase 3, and under its step 4, `--run-id`
building on `ci_runs_command` and `ci_of_head`. 0048 stays `proposed`.

### Phase 2: T1 and T3, what the record checks read, 2026-11-04

Begun on the owner's "committed, proceed" after `72a4667`. No deviation.

**What it did:**

* **`scripts/check_records.py`:**
  * `markdown_files` lists `git ls-files -z -co --exclude-standard -- '*.md'`, the files on disk among them; when git fails, as
    outside a work tree, it walks with `rglob` and `SKIP_PARTS`, as before. Every mode reads through it, `--next` and the name
    checks included.
  * `UNNUMBERED_DOCS`, the root `README.md`, `docs/README.md`, `docs/architecture.md`, `RELEASING.md`, `tools/README.md` and
    `AGENTS.md`, with `docs/guides/`, are what `--check-all` checks the links of. The module doc says both.
* **`.markdownlint-cli2.jsonc`:** `"gitignore": true`, beside the `.tools/**` ignore 0003 named, with its reason. The file is
  the fleet's shared configuration; the line is to be offered upstream at the PLAN's close.
* **The tests,** a new `scripts/test_check_records.py`, each copying the script into a temporary repository with
  `GIT_CEILING_DIRECTORIES` set to it: `test_links_of_the_docs_safe_docs`, a broken link in each of the three, "broken
  relative link: …:1: missing.md"; `test_ignored_directories_are_not_read`, copies under `.worktrees/` and `.cache/`, `--next`
  `0002` and no number claimed twice; `test_outside_a_work_tree`, `rglob`'s fallback finding the records and the link.
  `ReadmeTest` stays.
* **Docs:** `AGENTS.md`'s `make check-records` bullet names the docs it checks and what it reads; `docs/architecture.md`'s
  checks and tools paragraphs.

**A real run,** step 2.4: `make markdownlint` on the tree, "Linting: 47 files … 0 issues", where it linted 48 with
`.cache/tools/release/release-body-v1.28.0.md`; `python3 scripts/check_records.py --check-all`, exit 0, the three docs' links
resolving; `--next`, `0050`.

**Seen to fail,** by script, `scripts/test_check_records.py` run in a copy per plant, 2/2:

```text
FAILED AS REQUIRED: unnumbered_docs without RELEASING.md, tools/README.md and AGENTS.md: exit 1: FAIL: test_links_of_the_docs_safe_docs (test_check_records.CheckRecordsTest.test_links_of_the_docs_safe_docs) (doc='RELEASING.md')
FAILED AS REQUIRED: markdown_files back to rglob in a work tree: exit 1: FAIL: test_ignored_directories_are_not_read (test_check_records.CheckRecordsTest.test_ignored_directories_are_not_read)
```

The second's log: "number 0001 is claimed by 2 MADRs: .worktrees/copy/docs/decisions/0001-MADR-first.md, …" and `--next`
`0100`, the finding's own symptoms.

**Checks:**

* `make check-tools`: exit 0, the new tests among `scripts/`'s.
* `make pre-add-check` with the phase's files, once its wait for the load had passed: exit 0.
* `make check-records`, `make markdownlint`, `make disclosure-check`: exit 0 each. `tools/idscan.py --paths` over the phase's
  files: 0 matches.

Staged for the owner, who commits. Phase 3 begins on the owner's word.
