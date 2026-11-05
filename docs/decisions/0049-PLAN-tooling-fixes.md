---
status: complete
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
  *Amended 2026-11-04, phase 6's deviation 2:* T7 and T10 are comments, checked by reading and `actionlint`, with no test;
  T12 gains one.
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

The owner committed phase 2 as `992f6a5`.

### Phase 3: T4, T11, T5 and T6, tools that fail loudly, 2026-11-04

Begun on the owner's "committed, proceed" after `992f6a5`. No deviation.

**What it did:**

* **T4, `tools/_common.py`:** `GO_ENV_CLEARED` (`GONOSUMDB`, `GONOSUMCHECK`, `GOPRIVATE`, `GONOPROXY`, `GOINSECURE`, `GOFLAGS`,
  `GOEXPERIMENT`); `go_env(**extra)`, which removes them and sets `GOENV=off`, `GOSUMDB=sum.golang.org` and
  `GOPROXY=https://proxy.golang.org,direct` before the caller's; `GO_ENV_WANT` and `go_env_problems(env)`, which reads `go env
  -json` of those under `env` and names each that differs. `publication.py` checks it before any check, and `consumer.py`
  before it writes its module, each exiting 1 with "the go command would not use the public proxy and checksum database:
  …"; both give `go` `go_env`'s environment.
  * Under `GOENV=off`, `go env` reports `GOENV` as `""`: the first run of the check, with `"off"` expected, reported
    "GOENV='', want 'off'" on a correct environment. `GO_ENV_WANT` holds `""`, with the reason.
* **T11:** `release_body.py` prints idscan's output only when there is some, and, when the scan fails, "release_body: the
  identifier scan exited N: <idscan's stderr>" on stderr. **T11b:** `scripts/check_disclosure.py` prints "git ls-files
  failed: it needs a git work tree: <git's stderr>", without the root, as its contract prints no path but a file's.
* **T5, `tools/idscan.py`:** `--range` refuses a range with `...` or without `..`, exit 2; otherwise `scan_log` reads `git log
  -p --no-color --no-ext-diff --format=commit %H A..B` and reports each added line that matches as `commit:path:line`, the
  summary counting commits. The usage and help say so.
* **T6, `tools/find.py`:** `lines` exits 2 for a path named outright that is a directory, "… is a directory; use P/** for
  its files", or no file, "no such file: P"; `files` for a `ROOT` that is no directory. A glob matching nothing is still
  "nothing found", exit 1. The usage says so.
* **The tests:**
  * `tools/test_tools.py`: `HelpersTest.test_go_env`, a parent with the variables set and a `GOENV` file holding
    `GOPRIVATE` and `GONOSUMDB`, then `go env` itself reporting nothing exempted, and a planted value and the file each
    caught; `ReleaseTest.test_scan_failure_says_why`; `IdscanTest.test_range_finds_what_a_later_commit_removed` (the
    finding's probe, with its commit) and `test_range_refuses_other_forms`; `FindTest.test_bad_paths`.
    `IdscanTest.test_range_reads_the_commit` now matches the place with its commit, the format it reported changing.
  * `scripts/test_check_disclosure.py`: `test_main_outside_a_work_tree_says_why`.
* **Docs:** `tools/README.md`'s `idscan.py` and `find.py` rows.

**Real runs:**

* `tools/publication.py v1.33.0 6e04da6 <the body>`: "10 of 10 checks hold"; and again with `GOSUMDB=off` and `GONOSUMDB`
  naming the module in the parent: "10 of 10 checks hold", `go_env` overriding both.
* `tools/consumer.py v1.33.0`: exit 0, "as required".

**Seen to fail,** by script, the tests run in a copy per plant, 7/7:

```text
FAILED AS REQUIRED: go_env clears only the four it cleared: exit 1: FAIL: test_go_env (test_tools.HelpersTest.test_go_env)
FAILED AS REQUIRED: go_env without GOENV=off: exit 1: FAIL: test_go_env (test_tools.HelpersTest.test_go_env)
FAILED AS REQUIRED: release_body's reason not printed: exit 1: "the reason, not a bare status (0049-PLAN-tooling-fixes.md, phase 3, T11)")
FAILED AS REQUIRED: check_disclosure's reason not printed: exit 1: FAIL: test_main_outside_a_work_tree_says_why (test_check_disclosure.ScanTest.test_main_outside_a_work_tree_says_why)
FAILED AS REQUIRED: idscan's range scanning the endpoints again: exit 1: FAIL: test_range_finds_what_a_later_commit_removed (test_tools.IdscanTest.test_range_finds_what_a_later_commit_removed)
FAILED AS REQUIRED: idscan's range form not checked: exit 1: FAIL: test_range_refuses_other_forms (test_tools.IdscanTest.test_range_refuses_other_forms)
FAILED AS REQUIRED: find.py's named-path check removed: exit 1: FAIL: test_bad_paths (test_tools.FindTest.test_bad_paths)
```

**Checks:**

* `make check-tools`: exit 0.
* `make pre-add-check` with the phase's files, at load 23 to 26: exit 0.
* `make check-records`, `make markdownlint`, `make disclosure-check`: exit 0 each. `tools/idscan.py --paths` over the phase's
  files: 0 matches.

Staged for the owner, who commits. Phase 4 begins on the owner's word.

The owner committed phase 3 as `01dfcec`.

### Phase 4: T9, T10, T12 and T13, the Makefile and the configuration, 2026-11-04

Begun on the owner's "committed, proceed" after `01dfcec`. No deviation.

**What it did:**

* **T9, the `Makefile`:** `schema-version-set`, phony, fails, exit 2, with "schema/version read as empty: it is empty, or this
  make, $(MAKE_VERSION), is older than GNU make 4.2, which reading it needs" when `ACP_VERSION` is empty. It is an order-only
  prerequisite of `version`, `release` and the four schema downloads. The message names both causes, since an empty file
  under a new make reads the same.
* **T10:** the `Makefile`'s `api` comment and `ci.yml`'s apidiff step comment name the gated packages, the root,
  `agentproc`, `acptest` and `clienthost`, and the reported ones, `unstable` and `remote`.
* **T12, `.golangci.yml`:** the `//sys` rule for `_windows.go`, the `godot` rule, and both `paths` lists, `third_party$`,
  `builtin$` and `examples$`, go. `example/` stays linted. The file is the fleet's shared configuration (`FLEET_LINT_CFG`), as
  `.markdownlint-cli2.jsonc` is; the change is to be offered upstream at the PLAN's close.
* **T13:** `all`, `check-records`, `check-tools`, `clean`, `disclosure-check`, `markdownlint`, `release` and `version` gain
  `##` help; `tools/platforms.py`'s check list is `checks(lint)`, which `main` runs.
* **The tests,** `tools/test_tools.py`:
  * `MakefileTest.test_schema_version_guard`: `make schema-version-set ACP_VERSION=` fails with the guard's text; for each of
    the six targets, `make -n -B` prints the guard's recipe before any that reads the stamp, a `schema-v` URL, the generator's
    `go run .` or `release`'s write of `schema/version`; `make -n check-records` prints none of it.
  * `MakefileTest.test_help_lists_targets`: `make help`, its colour codes stripped, names the eight.
  * `PlatformsTest.test_checks`: the thirteen checks' names, written out, not read from `VET`, and each one's command and
    environment.
  * Two first runs refined them. The guard's test first required the guard's recipe first of all; under `-B`, make runs
    `version`'s normal prerequisite, `README.md`'s `mdsh`, before the order-only guard, which reads no stamp. And `\b`
    found no word boundary after the help's colour code, whose final `m` is a letter.
* **Docs:** `RELEASING.md`'s prerequisites say what make 3.81 does now; `docs/architecture.md`'s stamp paragraph names the
  guard.

**Real runs:**

* `/usr/bin/make` is GNU Make 3.81 here and the `make` on PATH: `make version` stops, "schema/version read as empty: … 3.81
  …", exit 2; `make check-records` exits 0. `make -n -B schema/meta.json` prints the guard's recipe before the `schema-v/` URL.
* `gmake`, 4.4.1: `gmake schema-version-set` exits 0; `gmake -n -B schema/meta.json` names `schema-v1.25.0`.
* `MakefileTest` passes under both, `gmake` put first on PATH for the second.
* `tools/gates.py lint platforms`: "ok   lint", "ok   platforms", with the smaller `.golangci.yml`.

**Seen to fail,** by script, the tests run in a copy per plant, 3/3:

```text
FAILED AS REQUIRED: the guard dropped from schema/meta.json: exit 1: AssertionError: unexpectedly None : schema/meta.json runs no guard: ['curl -o schema/meta.json --fail -L https://github.com/agentclientprotocol/agent-client-protocol/releases/download/schema-v/meta.json']
FAILED AS REQUIRED: clean's help dropped: exit 1: FAIL: test_help_lists_targets (test_tools.MakefileTest.test_help_lists_targets) (target='clean')
FAILED AS REQUIRED: a platform dropped from VET: exit 1: "vet-openbsd-amd64", "vet-netbsd-amd64", "vet-illumos-amd64", "vet-windows-amd64", "vet-js-wasm", "vet-wasip1-wasm",
```

The first shows the finding's own URL, `schema-v/meta.json`. T10's and T12's changes are comments and configuration with
nothing to fail; the lint gates show T12 drops no issue.

**Checks:**

* `make check-tools`: exit 0. `actionlint` at CI's pin on `ci.yml`: exit 0.
* `make pre-add-check` with the phase's files, at load 20 to 41: exit 0.
* `make markdownlint` first failed, "RELEASING.md:10:201 error MD013/line-length … Actual: 328", the prerequisites line made
  longer; it is wrapped, and passes. `make check-records`, `make disclosure-check`: exit 0. `tools/idscan.py --paths` over the
  phase's files: 0 matches.

Staged for the owner, who commits. Phase 5 begins on the owner's word.

The owner committed phase 4 as `719198f`.

### Phase 5: F-C, dead code and code no test runs, 2026-11-04

Begun on the owner's "committed, proceed" after `719198f`. No deviation: step 5.1's reading found the model computes no
reachability, and what it needs is the walk the step allows, from the types the dispatch names.

*Corrected 2026-11-04, in phase 6:* "No deviation" missed one. Step 5.1's engine case was to hold a type reached only by
`allOf`; `accept-gates-unreachable.txtar` as committed in `32ca4c7` holds none, its `Archive` reached by no property. Phase
6's deviation 1 records it and its resolution.

**What it did:**

* **`ConfigOptionUpdate.gates`,** `cmd/generate/internal/model/gates.go`: `dispatchReach(holders)` walks from every method's
  `Req`, `Resp` and `Notif` through the holders' fields, at any depth of pointer, slice or map, their members and their
  variants; `resolveGates` gives `gates` to a type only when it is gated and reached. The dispatch calls `gates` on params and
  results alone, and a holder's `gates` on what it holds, so a `gates` outside the walk is never called. Regenerated,
  `types_gen.go` loses `ConfigOptionUpdate.gates`, 9 lines, and nothing else.
* **`abandon`,** `agentproc/agentproc.go`: `Start` calls `proctree.Attach` through the package variable `attachTree`, its doc
  saying why; on Unix `Attach` cannot fail, so nothing else reaches `abandon` there.
* **The tests:**
  * `cmd/generate/testdata/engine/accept-gates-unreachable.txtar`: `accept-capability-gates`' schema and an `Archive` that
    holds the gated `SessionUpdate` and that no method reaches; its `behaviour_test.go`,
    `TestGatesOnlyWhereTheDispatchReaches`, requires `gates` on `SessionNotification` and `SessionUpdate` and none on
    `Archive`; its `want.api` written by `-update`. The behaviour test runs in `TestEngineBehaviour`, which generates every
    case before it tests one, so it is run whole; `TestEngineCases` checks the listing.
  * `cmd/generate/internal/model/gates_test.go`: `TestCapabilityRulesApply`'s `UnstableVerdict` and `Choice` were types no
    method reached, which now get no `gates`. Each fixture gives `SessionNotification`, which `s/update` carries, a property
    holding its type; the test checks the rules' resolution as before.
  * `agentproc/abandon_test.go`, new: `TestStartAbandonsUnattachedAgent`, not parallel, replacing `attachTree` with one that
    fails: `Start` returns no `Process` and an error wrapping the attach's, and a "beat" child's file stops growing, so the
    agent was killed and reaped.
  * `agentproc/stderr_test.go`: `TestStderrErrorText`. `elicitation_schema_test.go`: `TestElicitationCheckSchemaDefaultText`,
    a refused default's text read through `CheckSchema`'s error, as `defaultError.Error` gives it.
* **Docs:** `docs/architecture.md`'s gate resolution and its list of engine cases.

**The coverage run,** step 5.4, `go test -coverprofile -coverpkg=./... . ./agentproc/`: `agentproc.go:292 abandon 100.0%`,
`stderr.go:21 Error 100.0%`, `elicitation.go:286 Error 100.0%`; `func (v *ConfigOptionUpdate) gates` is no longer in
`types_gen.go`.

**Seen to fail,** 4/4:

* by `tools/plants.py`:

  ```text
  FAILED AS REQUIRED: abandon not called on an attach failure (^TestStartAbandonsUnattachedAgent$): exit 1: abandon_test.go:46: the agent still runs after Start abandoned it: its file grew from -1 to 6 bytes in 300 ms
  FAILED AS REQUIRED: StderrError's text changed (^TestStderrErrorText$): exit 1: stderr_test.go:166: StderrError{Tail: "no newline"}.Error() = "agentproc: stderr: no newline", want "agentproc: the agent's standard error ended: no newline"
  FAILED AS REQUIRED: defaultError's text changed (^TestElicitationCheckSchemaDefaultText$): exit 1: elicitation_schema_test.go:125: the refused default reads "a default", want its RuleDefault error, …
  ```

* by script, the generator's test in a copy: `gates` for every gated type again,
  `TestEngineBehaviour`: "behaviour_test.go:15: Archive, which no method reaches, has gates".
* The generator's plant first failed to build, leaving `reach` unused, and was rewritten as `reach[h.name] || true`. It then
  passed under `TestEngineCases`, which runs no behaviour test, and failed to generate under
  `-run TestEngineBehaviour/accept-gates-unreachable`, which skips the subtest that generates; it fails as required under
  `TestEngineBehaviour` whole, which passes, in 35 s, on the fixed code.

**Checks:**

* `go test -count=1 ./...` in both modules: `ok`. `go test -race -count=3 ./agentproc/`: `ok`.
* `tools/gates.py regen`: `ok`. `make api` against `v1.33.0`: "0 incompatible, 0 compatible".
* `make pre-add-check` with the phase's files, at load 27 to 44: exit 0, "7 file(s) gofmt-clean; both modules linted, vetted
  and tested (-race -shuffle=on, uncached), tidy, govulncheck."
* `tools/remote.py tree --host linux`: both modules, exit 0.
* `tools/remote.py pkg --host windows ./agentproc -test.run 'TestStartAbandonsUnattachedAgent|TestStderrErrorText'
  -test.count=3`: "PASS", where `attachTree` stands for a real job attach.
* `make check-records`, `make markdownlint`, `make disclosure-check`: exit 0 each. `tools/idscan.py --paths` over the phase's
  files: 0 matches.

Staged for the owner, who commits. Phase 6 begins on the owner's word.

The owner committed phase 5 as `32ca4c7`.

### Phase 6: close, 2026-11-04

Begun on the owner's "committed, proceed" after `32ca4c7`.

**Deviation 1, 2026-11-04: step 5.1's `allOf` route had no engine case.**

* **Found:** step 5.1 (`:185`) asks for an engine case holding "a type reached only by `allOf`, which gets no `gates`".
  `cmd/generate/testdata/engine/accept-gates-unreachable.txtar:9-44`, at `32ca4c7`, has no `allOf`: its `Archive` is
  reached by no property. The rule was tested, the route the finding took, `ConfigOptionUpdate`'s, only by
  `tools/gates.py regen`, as a diff of `types_gen.go`. Phase 5's record said "No deviation"; it is annotated, not rewritten.
* **Decided,** by the owner, from three options (the others: strike the wording and rely on the regen gate; a model unit
  test instead): add the `allOf` route to `accept-gates-unreachable.txtar`, a gated definition used only through a
  `SessionUpdate` variant's `allOf`, its behaviour test requiring `gates` on the variant the merge flattens it into and none
  on the definition; `want.api` rewritten by `-update`; a plant; the pre-add check.
* **Files added to the phase:** `cmd/generate/testdata/engine/accept-gates-unreachable.txtar`; `docs/architecture.md` if its
  case list's wording changes. The MADR's decisions and facts stand: no amendment.

**Deviation 2, 2026-11-04: three findings had no test the Verification asks for.**

* **Found:** the Verification's first bullet asks a committed test of each of T1 to T7 and T9 to T14. T7 (`ci.yml`'s and
  `docs.yml`'s concurrency comments, phase 1) and T10 (the `Makefile`'s and `ci.yml`'s `make api` comments, phase 4) change
  comments; T12 (`.golangci.yml`'s dead exclusions, phase 4) changes configuration, which phase 4 showed drops no issue,
  but nothing keeps a dead exclusion from returning.
* **Decided,** by the owner, from three options (the others: record all three with no code; a test of T10's comment text
  too): a test in `tools/test_tools.py` requiring each path exclusion of `.golangci.yml` to match a tracked file, seen to
  fail on a planted one; T7 and T10 recorded as comments. The Verification bullet is annotated, not rewritten.
* **Files added to the phase:** `tools/test_tools.py`. The MADR's decisions and facts stand: no amendment.

**Deviation 3, 2026-11-04: a runtime test failed in the gate set.**

* **Found:** step 6.1's `tools/gates.py --all`, at load 24 to 26, failed one gate, `race3`, `go test -race -count=3
  -shuffle=on .`, seed `1793834057755576000`: `connection_overflow_block_test.go:329`, `TestOverflowBlockHeldInitialize`,
  "Initialize = {"code":-32603,…"connection ended before response"}, want its result". The other 17 gates passed. This phase
  changes no root-package code; the failure is the one
  [0049-PLAN-surface-fixes.md](0049-PLAN-surface-fixes.md)'s phase 2 saw once. Rerun: 200 runs alone under `-race`, `ok`;
  the package with the seed, `-race -count=3`, `ok`.
* **Decided,** by the owner, from three options (the others: stop the close and examine it first, as a phase of
  [0049-PLAN-runtime-fixes.md](0049-PLAN-runtime-fixes.md); note it here only, as the surface PLAN did): recorded as a
  finding, R11, in the MADR's F-R, open, to be examined by an amendment of the runtime PLAN on the owner's approval; the
  gate set run again for this record.
* **Files added to the phase:** the MADR, its R11 and the runtime PLAN's bullet in "Decision Outcome"; `docs/README.md`'s
  MADR row.

**What the deviations added:**

* **Deviation 1,** `cmd/generate/testdata/engine/accept-gates-unreachable.txtar`: `Batch`, holding `[]SessionUpdate`, reached
  only by a `batch_update` variant's `allOf`, which the merge flattens into `SessionBatchUpdate`, as it does
  `config_option_update` into `SessionConfigOptionUpdate`. Its behaviour test requires `gates` on `SessionBatchUpdate` and
  none on `Batch`; `want.api` rewritten by `-update`. A first try held `[]ContentBlock`, which this case gates nowhere: the
  test failed, "SessionBatchUpdate, which SessionUpdate holds, has no gates", and `Batch` having none proved nothing.
* **Deviation 2,** `tools/test_tools.py`: `GolangciTest.test_path_exclusions_match_a_file` reads `.golangci.yml`'s `path`,
  `path-except` and `paths` items, requires `_gen\.go$` among them, and each to match a file `git ls-files -co
  --exclude-standard` lists, or, outside a work tree, a file on disk.

**Seen to fail,** by script, the tests run in a copy per plant, 4/4:

```text
FAILED AS REQUIRED: gates emitted for every gated type again: exit 1: behaviour_test.go:22: Batch, reached only by allOf, has gates
FAILED AS REQUIRED: gates emitted for the allOf-reached type alone: exit 1: behaviour_test.go:22: Batch, reached only by allOf, has gates
FAILED AS REQUIRED: a dead paths item back, in a work tree: exit 1: AssertionError: False is not true : .golangci.yml excludes examples$, which matches no file
FAILED AS REQUIRED: a rule's path that matches nothing, outside a work tree: exit 1: AssertionError: False is not true : .golangci.yml excludes _tests\.go, which matches no file
```

The second plant gives `gates` to `Batch` alone, so the `allOf` assertion is seen to fail by itself; the generator's
plants run `TestEngineBehaviour` whole, as phase 5's did.

**Step 6.1,** `tools/gates.py --all`, `ACP_LINUX_HOST` and `ACP_WINDOWS_HOST` named per command:

* First run, at load 24 to 26: 17 of 18; `race3` failed, deviation 3.
* Second run, after deviations 1 to 3, at load 9 to 18: "all gates passed", the 18 each `ok`: `regen`, `precheck`,
  `newfiles`, `nojsonv2`, `winvet`, `records`, `mdlint`, `idscan`, `tools`, `lint`, `fix`, `tidy`, `api`, `race3`, `floor`,
  `platforms`, `windows`, `linux`.

The findings, each with what shows it:

| Finding | Phase | Shown by | Seen to fail |
| --- | --- | --- | --- |
| T1, the docs-safe docs' links | 2 | `scripts/test_check_records.py` `test_links_of_the_docs_safe_docs` | 1 |
| T2, CI's run, not Docs' | 1 | `PublicationTest.test_ci_of_head`, `test_ci_runs_command`, `test_passed`; the real run naming `41890673918` | 2 |
| T3, ignored directories | 2 | `test_ignored_directories_are_not_read`, `test_outside_a_work_tree`; `make markdownlint`, 47 files | 1 |
| T4, the Go environment | 3 | `HelpersTest.test_go_env` | 2 |
| T5, `idscan.py --range` | 3 | `IdscanTest.test_range_finds_what_a_later_commit_removed`, `test_range_refuses_other_forms` | 2 |
| T6, `find.py`'s bad paths | 3 | `FindTest.test_bad_paths` | 1 |
| T7, the concurrency comments | 1 | a comment, read; `actionlint` (deviation 2) | none |
| T8, `mdsh` in `RELEASING.md` | before 4e3d9cc | corrected in `f6bfdcf` | none |
| T9, the schema version guard | 4 | `MakefileTest.test_schema_version_guard`; `/usr/bin/make` 3.81 stopping | 1 |
| T10, `make api`'s packages | 4 | a comment, read (deviation 2) | none |
| T11, the scans' reasons | 3 | `ReleaseTest.test_scan_failure_says_why`; `test_main_outside_a_work_tree_says_why` | 2 |
| T12, dead lint exclusions | 4, 6 | `GolangciTest.test_path_exclusions_match_a_file`; `tools/gates.py lint platforms` | 2 |
| T13, help and `platforms.py` | 4 | `MakefileTest.test_help_lists_targets`, `PlatformsTest.test_checks` | 2 |
| T14, `ci_jobs.py`'s runs | 1 | `CiJobsTest.test_run_list_is_ci_s` | 1 |
| F-C, `ConfigOptionUpdate.gates` | 5, 6 | `accept-gates-unreachable`'s `TestGatesOnlyWhereTheDispatchReaches`; `types_gen.go` without it | 3 |
| F-C, `abandon` | 5 | `TestStartAbandonsUnattachedAgent`; coverage 100.0% | 1 |
| F-C, the two `Error` methods | 5 | `TestStderrErrorText`, `TestElicitationCheckSchemaDefaultText`; coverage 100.0% each | 2 |

**Step 6.2,** `make api`: "check_api: against v1.33.0: 0 incompatible, 0 compatible"; "reported packages unstable, remote: 0
incompatible, not gated"; exit 0.

**Step 6.4:** no release. Nothing exported or run-time changed; the next release's notes name this PLAN.

**Not done in this phase:**

* Step 6.3, CI on the head, waits for the owner's push; step 6.5, this PLAN and its row `complete` and the MADR's T and F-C
  findings done, follows CI, as the subpackage PLAN's did.
* R11 is open (deviation 3).
* `.markdownlint-cli2.jsonc`'s `"gitignore": true` (phase 2) and `.golangci.yml`'s removed exclusions (phase 4) are the
  fleet's shared files: they are to be offered upstream, by the owner, outside this repository.

**Checks:**

* `make check-tools`: exit 0, 41 tests, "check-ci-filters: 0 problem(s)".
* `cd cmd/generate && go test -count=1 -run '^(TestEngineBehaviour|TestEngineCases)$' .`: `ok`.
* `make check-records`, `make markdownlint`, `make disclosure-check`: exit 0 each. `tools/idscan.py --paths` over the phase's
  files: 0 matches.

Staged for the owner, who commits and pushes. Step 6.3 follows the push.

The owner committed phase 6 as `ea32a99` and pushed it; `make push-guard` passed first, 7 outgoing commits, `1fe3dd0..ea32a99`.

**Step 6.3,** CI on `origin/main`'s head, `ea32a99`: the CI workflow's run `41936179461`, "success", its six jobs each
"success": `validate` on `ubuntu-24.04`, `macos-15` and `windows-2025`, each with Go `1.27.3` and `1.26.10`. The Docs
workflow's run `41936179495`: "success".

**Step 6.5.** This PLAN and its row are `complete`. The MADR's F-T section records T1 to T14 as done, and its F-C section
the three gaps, with no release of their own. The MADR stays `accepted`, with the generator PLAN to come (Q1 a, Q7 a), and
R11 open.

**Verification,** against the section above:

* T1 to T6, T9, T11 to T14 and F-C's three gaps each have a committed test, seen to fail on its plant and passing now; T7
  and T10 are comments, read (deviation 2); T8 was fixed in `f6bfdcf`. See the table in step 6.1.
* `tools/publication.py v1.33.0 6e04da6` named the CI workflow's run `41890673918` for main's head (phase 1).
* `make markdownlint` and `make check-records` read only what git tracks or would (phase 2).
* `make check-tools`, `tools/gates.py --all`, 18 of 18 after deviation 3, and CI on `ea32a99` pass; `make api` reports no
  change.

The owner committed and pushed step 6.5 as `c69499d`: its Docs workflow run `41937797649`, "success".
