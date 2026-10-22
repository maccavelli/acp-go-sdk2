---
status: complete
date: 2026-10-22
associated-madr: "0031-MADR-verification-tools-in-repository.md"
---

# Implement 0031: keep the maintainer's verification tools in the repository, in `tools/`, with every identifier kept out

Associated MADR: [0031-MADR-verification-tools-in-repository.md](0031-MADR-verification-tools-in-repository.md)

## Goal

The reusable verification tools of the MADR's T3 live in `tools/`, standard-library Python, tested by `make check-tools`, each seen to
fail on a planted break, and none holding an identifier: the hosts and the deny list come from outside the repository. This PLAN assumes
the MADR's recommended answers to questions 1 to 4; an answer otherwise amends it before step 2. (The owner chose them on
2026-10-21.)

## Scope

* **New, `tools/`:** `_common.py`, `gates.py`, `idscan.py`, `floor.py`, `platforms.py`, `remote.py`, `leftovers.py`, `plants.py`,
  `release_body.py`, `publication.py`, `consumer.py`, `watch_ci.py`, `find.py`, `procs.py`, `test_tools.py`, `README.md`.
* **`Makefile`:** `check-tools`, `python3 -m py_compile` on every tool, then `python3 -m unittest` on `tools/test_tools.py`.
* **`.github/workflows/ci.yml`:** the records step also runs `make check-tools`.
* **Docs:** `docs/architecture.md`, a section "Verification tools" and the tree; `AGENTS.md`, a line under "Pre-add checks" naming
  `tools/README.md`.
* **The records:** this pair and their rows in `docs/README.md`.
* **Not in scope:** `scripts/`, unchanged; the one-off scripts of the MADR's T1; any Go file.

## Design

* **`_common.py`.** `repo()`, the root from `git rev-parse --show-toplevel`; `run(name, cmd, cwd, env, stdin, timeout)`, which returns the
  exit status and the output, the status taken before the output is read, and appends both to `<out>/<name>.log`; `host(kind)`, the value
  of `ACP_LINUX_HOST` or `ACP_WINDOWS_HOST`, or `SystemExit` naming the variable; `out_dir(arg)`, `--out` or `.cache/tools/<tool>/`,
  created; `files_of_tree()`, the tracked and untracked-but-not-ignored files, without `._` metadata.
* **`idscan.py`.** Reads the deny list from `DISCLOSURE_DENY` or `~/.config/git/disclosure-deny`: each non-comment line's first field, a
  regex; `identity` lines too, since a name in a file is still a name. Scans the changed files (`git diff --name-only HEAD` and the
  untracked), `--all` (every tracked file), or `--range A..B` (the files those commits touch, as committed). Prints `path:line` for each
  match, never the matched text; exits 1 on a match, 2 when the list is missing or holds no pattern.
* **`gates.py`.** The gates of the MADR's T3, by name: `regen`, `precheck`, `newfiles`, `nojsonv2`, `winvet`, `records`, `mdlint`, `idscan`,
  `lint`, `fix`, `tidy`, `api`, `race3`, `floor`, `platforms`, `windows`, `linux`. No argument: every gate but `windows` and `linux`;
  `--all`: every gate. Each gate's log under the output directory; a summary line per gate; exit 1 if any failed. `fix` fails when
  `go fix -diff` prints anything, as `tidy` when `go mod tidy -diff` does.
* **`floor.py`.** `GOTOOLCHAIN=go<the go line>`, its caches under the output directory; `go version`, `go vet ./...` and
  `go test -race -count=1 ./...` in both modules, the root's tests with `GOEXPERIMENT=goroutineleakprofile`. It fails when `go version`
  does not name that toolchain.
* **`platforms.py`.** `golangci-lint` for `GOOS=linux` and `GOOS=freebsd` on `./agentproc/...`, and `go vet ./agentproc` for `darwin/arm64`,
  `linux/amd64`, `linux/arm64`, `freebsd`, `openbsd`, `netbsd`, `illumos`, `windows`, `js/wasm`, `wasip1/wasm` and `plan9`; `--tree DIR`
  runs them on another tree.
* **`remote.py`.** `tree --host {linux,windows}`: the tree's files to a fresh directory in the remote home, `go test -count=1 -shuffle=on
  ./...` in both modules, the directory removed. `pkg --host H PKG [flags...]`: the package's test binary cross-compiled for the host's
  `GOOS` and `GOARCH` (`--goarch`, default `amd64`), copied, run with the flags as separate arguments, quoted for the remote shell, removed.
  The remote directory's name is `acp-tools-<random>`, relative to the remote home.
* **`leftovers.py`.** `tasklist` on the Windows host; exit 1 when an image name is exactly a test binary's (`--name` adds names).
* **`plants.py SPEC.json`.** Each plant: `name`, `edits` (`file`, `old`, `new`, each `old` asserted to occur once), `pkg`, `run` (a test
  pattern), `want` (a text the output must hold), and `host` (absent for local). A scratch copy per plant, without `.git`; local plants
  run `go test -count=1 -run`, host plants through `remote.py pkg`. Prints each plant's verdict and the first failing test line; exit 1
  unless every plant fails as required.
* **`release_body.py VERSION REV`.** The text under `### VERSION` in the report for that version at `REV`, up to "Sources not in the
  release text", each relative link made absolute at the tag, each target asserted to exist at `REV`; the result written to the output
  directory, and `idscan.py`'s patterns applied to it.
* **`publication.py VERSION COMMIT BODY`.** The ten checks of [0030-PLAN-record-initialize-on-arrival.md](0030-PLAN-record-initialize-on-arrival.md)'s
  "Publication of `v1.20.0`", in a scratch module cache. (Amended 2026-10-22, during step 6: two of them on origin's `main`; see the deviation during step 6.)
* **`consumer.py VERSION`.** A fresh module at the `go` line, with that toolchain forced, `go get` of the version, a program that starts
  `sh -c "cat >/dev/null"` through `agentproc` with an `OutputTap`, built and run, and built for Windows; its `go.mod` must keep the `go`
  line and gain no `toolchain` line.
* **`watch_ci.py SHA`.** Waits up to 20 minutes for the run of that full commit id, then watches it; prints each job's conclusion; exits
  with the run's.
* **`find.py`.** `lines PATH|GLOB... -- TEXT...`, each matching line with its file and number; `files ROOT PATTERN...`, the matching paths,
  `.git` left out. **`procs.py TEXT...`**, the processes whose command line holds any, this one excluded; exit 1 when any.

### `watch_ci.py` and the main check (added 2026-10-22, after step 8)

The MADR's "Amendment 2026-10-22, after step 8", W1 to W4, as the owner decided them.

* **`watch_ci.py`,** its pure parts functions the tests call:
  * `runs_for(runs, sha, branch)`: every run of the commit, newest first, those of `branch` alone when it is given;
  * `settled(jobs)`: every job's status "completed";
  * `verdict(run, jobs)`: success only when the run's conclusion is "success" and every job's is "success" or "skipped";
  * `wait_settled(fetch, bound, every, clock, sleep)`: calls `fetch` until `settled`, or the bound passes; returns the last read and
    whether it settled; `clock` and `sleep` injected, so a test needs no wait;
  * `watch(cmd, timeout, runner)`: runs `gh run watch`, returns its exit status, or a "did not finish within" verdict for
    `TimeoutExpired`.
  * The flow: wait for the first run (`--minutes`, 20); then 30 seconds more for others (`--more`, question 6); watch each run, then
    `wait_settled` it (`--settle`, 180 seconds, every 10, question 5); print each run's id, branch, event, status, conclusion, the
    watch's exit status, and each job; exit 0 only when every run's `verdict` is success, 1 when one is not or did not settle, 2 when no
    run appeared. `--branch` narrows.
* **`publication.py`:** "CI on origin main's head" asks `gh run list --commit <head> --branch main --limit 1`, and names the run's id and
  branch in its detail; "CI on the tag" names its run's id too. `main_run(runs)` picks the newest run whose `headBranch` is `main`.
* **`tools/README.md`:** `watch_ci.py`'s row and its flags.

## Tests

`tools/test_tools.py`, `unittest`, no host and no network:

* `idscan.py` against a deny list and files in a temporary directory: a match reported by place without its text; no match exits 0; a
  missing or empty list exits 2.
* `plants.py` against a temporary Go module with one test: a plant whose edit makes the test fail with the wanted text passes; one whose
  edit does not apply, or whose test passes, fails the run.
* `release_body.py` against a temporary repository with a report and a record: the body extracted, the link rewritten, a missing target
  refused.
* `find.py` and `procs.py`: a planted line found with its number; this process not listed.
* `_common.host`: unset, it stops naming the variable.
* `gates.py`'s `fix` and `tidy` rule: output on exit 0 counts as a failure.
* (Added 2026-10-22, after step 8.) `watch_ci.py`: `runs_for` returns both of two runs of a commit, and one with `--branch`;
  `verdict` fails a run whose conclusion is "success" with a job "failure", and passes "skipped"; `wait_settled`, on a fetch that reads
  "in_progress" twice then "completed", returns settled after three reads, and on one that never settles, unsettled at the bound, with a
  fake clock; `watch` turns `TimeoutExpired` into a verdict. `publication.py`: `main_run` picks the `main` run of a tag's and a branch's.
* (Added 2026-10-22, after step 9.) `find.py`: run with its output piped to a reader that takes one line and closes, it exits
  without a traceback, by Python's documented handling of `BrokenPipeError`.
* (Added 2026-10-22, after step 10.) Every tool: each `tools/*.py` but `_common.py` and `test_tools.py` ends
  `sys.exit(_common.main_guard(main, sys.argv[1:]))`, read from its source; ~~and `procs.py`, piped to a reader that closes early, exits
  without a traceback, as `find.py` does.~~ (Amended 2026-10-22, during step 11: and `main_guard` itself, in a subprocess, a `main`
  that writes to a reader closing after one line and a `main` that raises `BrokenPipeError`: exit 1, no traceback; see the deviation
  during step 11.)

## Implementation Steps

Each step ends with its checks, an execution record here, and the files staged for the owner. Every new check is seen to fail on a
planted break in a scratch copy, never in the tree.

1. **Records:** this pair and their rows, `proposed` until approved; then this PLAN `in-progress`.
2. **The base:** `_common.py`, `idscan.py`, `find.py`, `procs.py`; their tests; `make check-tools`. `idscan.py --all` over the tree finds
   nothing; with a planted identifier in a scratch copy, it finds it, by place.
3. **The gates:** `gates.py`, `floor.py`, `platforms.py`; their tests. On one tree, `gates.py --all` agrees gate by gate with the
   scratch directory's gate run of 0030's steps. (Amended 2026-10-21, during step 3: every gate but the two hosts'; see the deviation
   during step 3.)
4. **The hosts:** `remote.py`, `leftovers.py`. Both hosts: `remote.py tree` passes; a planted failing test fails it; `remote.py pkg` runs
   a single test by pattern, `|` in it; `leftovers.py` catches a planted name. (Amended 2026-10-21, during step 3: and `gates.py`'s
   `windows` and `linux` gates agree with the scratch directory's host runs, so that `--all` is matched.)
5. **The plants:** `plants.py`; its tests. 0030's step 7 plants, as a spec, fail as required, 3 of 3, as its record says.
6. **The release:** `release_body.py`, `publication.py`, `consumer.py`, `watch_ci.py`. `release_body.py v1.20.0 4fa7204` reproduces the
   published body byte for byte; `publication.py` holds 10 of 10 for `v1.20.0`; `consumer.py v1.20.0` as recorded; `watch_ci.py` on the
   last pushed commit.
7. **CI and docs:** `ci.yml`, `tools/README.md`, `docs/architecture.md`, `AGENTS.md`.
8. **Verification and close:** the gates, by `gates.py --all`; `make push-guard`; after CI passes, this PLAN and its row `complete`.
9. **(Added 2026-10-22, after step 8) The CI runs:** `watch_ci.py` and `publication.py` as the design's last section has it; their
   tests; `tools/README.md`. Live: `watch_ci.py 4fa7204` reports both its runs, the tag's and `main`'s, each success, and `--branch
   main` the one; `watch_ci.py b42620c` lists every job "completed"; `publication.py v1.20.0 4fa7204 <body>` holds 10 of 10, "CI on
   origin main's head" naming `39714586565`, `main` (amended 2026-10-22, during step 9: `main`'s run of origin main's head; see
   the deviation during step 9). Then the gates, `make push-guard`, and after CI passes, this PLAN and its row
   `complete` again.
10. **(Added 2026-10-22, after step 9) `find.py`'s broken pipe:** `_common.main_guard`, which runs a tool's `main` and, on
    `BrokenPipeError`, points standard output at the null device and exits 1, as Python's documentation of `signal` advises for
    SIGPIPE; `find.py` run through it; the test; the gates, `make push-guard`, and after CI passes, this PLAN and its row `complete`.
11. **(Added 2026-10-22, after step 10) Every tool through `main_guard`:** the twelve tools whose entry point still calls `main`
    directly, `consumer.py`, `floor.py`, `gates.py`, `idscan.py`, `leftovers.py`, `plants.py`, `platforms.py`, `procs.py`,
    `publication.py`, `release_body.py`, `remote.py` and `watch_ci.py`, run through it, `procs.py` importing `_common` to do so; the
    tests; the gates, `make push-guard`, and after CI passes, this PLAN and its row `complete`.

## Verification

Per step: `make check-tools`; `gates.py` from step 3, and before it the scratch directory's gate run; `idscan.py --all` from step 2.

Planted breaks, each in a scratch copy:

| For | Break | The check that must fail |
| :--- | :--- | :--- |
| `idscan.py` | a deny-list pattern planted in a tool | `idscan.py --all` |
| `idscan.py` | the matched text printed | `test_tools.py`, the text not printed |
| `gates.py` | `fix` passing on output | `test_tools.py`, the `fix` rule |
| `floor.py` | `GOTOOLCHAIN` not set | `floor.py`, the toolchain check |
| `remote.py` | a failing test in the copied tree | `remote.py tree` |
| `leftovers.py` | a substring match in place of the image name | `leftovers.py`, `tasklist.exe` not matched as `t.exe` |
| `plants.py` | an edit that does not apply accepted | `test_tools.py` |
| `release_body.py` | a link target missing | `test_tools.py` |
| `watch_ci.py` (added 2026-10-22, after step 8) | the verdict by the run's conclusion alone | `test_tools.py`, a job's failure under a run's success |
| `watch_ci.py` | no wait for the jobs to settle | `test_tools.py`, `wait_settled`'s three reads |
| `watch_ci.py` | the first run alone | `test_tools.py`, `runs_for`'s two runs |
| `watch_ci.py` | `--branch` ignored | `test_tools.py`, `runs_for` with a branch |
| `watch_ci.py` | `TimeoutExpired` not caught | `test_tools.py`, `watch` |
| `publication.py` | the main check without `--branch main` | `test_tools.py`, `main_run` |
| `find.py` (added 2026-10-22, after step 9) | the broken pipe not handled | `test_tools.py`, the closed reader |
| every tool (added 2026-10-22, after step 10) | one tool's entry point calling `main` directly | `test_tools.py`, the entry points read |
| ~~`procs.py`~~ `main_guard` (amended during step 11) | the broken pipe not handled | `test_tools.py`, ~~the closed reader~~ the guard's own test |

Acceptance:

* `make check-tools` passes locally and in CI's records step, which runs in the Linux job at the newest Go (`ci.yml:113`–`:119`), and
  CI's six jobs pass;
* each step's planted breaks fail as required;
* `idscan.py --all` finds nothing in the tree, and the push guard passes;
* `make pre-add-check`, `make lint`, `make api` and the tests are unchanged by this work.

## Rollout and Rollback

Each step is one commit, reverted by reverting it. Nothing in the module changes: no consumer is affected, and no release is needed. A
maintainer sets `ACP_LINUX_HOST` and `ACP_WINDOWS_HOST` for the host tools, and keeps the deny list where the push guard reads it.

## Execution record

### Step 1, records, 2026-10-21

The owner answered the MADR's four questions as recommended, "Follow recommendations", which approves this PLAN as written. What
changed: the MADR `accepted`, its Decision Outcome naming the answers, its specification "as decided", and a table of them; this PLAN
`in-progress`; their rows in `docs/README.md`.

**Before it,** while the pair was drafted: the identifier scan found the word the scan treats as a scratch path in eight lines of the
pair, and it was replaced by "scratch directory"; and the acceptance said CI's six jobs run the records step, where only the Linux job at
the newest Go does (`ci.yml:113`–`:119`), and was corrected.

**Gates,** each exit 0: `make check-records`, `make markdownlint` and the identifier scan. No file but these records changed.

### Deviation, 2026-10-21, during step 2: what the deny list covers

**Found.** Before relying on `idscan.py`, its patterns, the push guard's deny list, were checked against the kinds of identifier the
records' scans have caught (the MADR's "Amendment 2026-10-21, step 2: what the deny list covers", D1 to D3): `idscan.py --all` over the
tree found 0 matches in 506 files with 17 patterns, but the list lacks five kinds (D3), so a clean scan would not have shown them absent.

**Decision (owner, 2026-10-21).** The owner adds patterns for D3 to the deny list. Added to step 2: the coverage check, rerun after the
addition, every kind covered; then `idscan.py --all` over the tree again. No file is added to the step.

### Deviation, 2026-10-21, during step 2: the scratch-directory word dropped

**Found.** Once the owner had added six patterns, 23 in all, the coverage check covered every kind, and `idscan.py --all` found 11 lines,
each the scratch-directory word, in five published PLANs (the MADR's D4).

**Decision (owner, 2026-10-21).** The owner removes that pattern, by a Python script that removes exactly that line and prints nothing of
the list. Step 2's coverage check then requires every kind but the word, and `idscan.py --all` finds nothing.

### Step 2, the base, 2026-10-21

The owner approved this step. What changed:

* **`tools/_common.py`:** `repo`, `host` (stops naming `ACP_LINUX_HOST` or `ACP_WINDOWS_HOST` when unset), `out_dir` (`.cache/tools/<tool>/`
  or `--out`), `run` (the status taken before the output is read, both appended to a log), `files_of_tree`, `fail`.
* **`tools/idscan.py`:** the deny list from `DISCLOSURE_DENY` or `~/.config/git/disclosure-deny`; the changed files, `--all`, `--range A..B`
  (each file as at `B`, read as bytes) or `--paths`; each match printed as `path:line`, never its text; exit 1 on a match, 2 when the list
  is missing or empty.
* **`tools/find.py`** (`lines`, `files`) and **`tools/procs.py`.**
* **`tools/test_tools.py`:** 11 tests; the test repositories commit with `core.hooksPath=/dev/null`, so that no global hook runs.
* **`Makefile`:** `check-tools`, `py_compile` on every tool, then `unittest`.

The owner committed the step's files as `d6f3e78` before its checks were done; the two deviations above came after, and this record and
the deviations' records follow in the step's own commit.

**The claims, each to what makes it:**

| Claim | Check |
| :--- | :--- |
| a match is reported by place, without its text | `test_match_reported_by_place_only`; and the planted home path below |
| the changed files include the untracked; `--range` reads the commit, not the tree | `test_changed_files_include_untracked`, `test_range_reads_the_commit` |
| a missing or empty deny list fails, exit 2 | `test_missing_or_empty_list` |
| `find.py` and `procs.py` find what they should, and `procs.py` not itself | `FindTest`, `ProcsTest` |
| an unset host stops the tool, naming the variable | `test_unset_names_the_variable` |
| the deny list covers every kind of identifier the records' scans have caught, the scratch-directory word left out | the coverage check: 22 patterns, 8 of 8 kinds |
| the tree holds none | `idscan.py --all`: "511 file(s), 22 pattern(s), 0 match(es)" |

**Planted breaks,** each in its own scratch copy; 9 of 9 failed as required:

| Break | The check that failed |
| :--- | :--- |
| the matched text printed | `test_match_reported_by_place_only`: "the matched text must not be printed" |
| the untracked files not scanned | `test_changed_files_include_untracked`: "0 != 1" |
| `--range` read from the tree | `test_range_reads_the_commit`: "0 != 1" |
| a missing list taken as clean | `test_missing_or_empty_list`: "0 != 2" |
| `find.py`'s line numbers off by one | `test_lines` |
| `procs.py` listing itself | `test_finds_a_process_and_not_itself`: "1 != 0" |
| an unset host accepted | `test_unset_names_the_variable`: "SystemExit not raised" |
| a home path planted in a clone of the tree | `idscan.py --all`: "identifier at docs/planted.md:2", the path not printed |
| a syntax error in a tool | `make check-tools`: "SyntaxError: expected ':'" |

**Gates,** each exit 0: `make check-tools`, "Ran 11 tests", OK; the coverage check and `idscan.py --all` above; and the gate set this PLAN
replaces from step 3, run as in 0030's steps: regeneration unchanged, `make pre-add-check`, `GOEXPERIMENT=nojsonv2 go test ./...`,
`GOOS=windows go vet ./...` in both modules, `make check-records`, `make markdownlint` and its identifier scan, `make lint`, `go fix -diff`
and `go mod tidy -diff` in both modules, `make api` ("against v1.20.0: 0 incompatible, 0 compatible"), the root's tests under `-race`
three times, the floor gate, and both hosts' tests.

### Deviation, 2026-10-21, during step 3: the host gates compared in step 4

**Found.** Step 3's acceptance compares `gates.py --all` with the scratch directory's gate run, but `--all` includes the `windows` and
`linux` gates, which run through `remote.py tree`, and step 4 builds `remote.py`.

**Decision (owner, 2026-10-21).** Step 3 builds `gates.py` with its host gates calling `tools/remote.py`, and compares every other gate;
step 4 compares the two host gates once `remote.py` exists. The alternative was to move `remote.py tree` into step 3. Steps 3 and 4 are
annotated above; no file moves between them.

### Deviation, 2026-10-22, during step 3: the caches outside the repository

**Found.** The comparison of `tools/gates.py` with the gate set run after it disagreed on `mdlint` alone: 0 against 2. `floor.py` had put
the Go caches, a toolchain's Markdown files among them, under `.cache/tools/floor/`, which `make markdownlint` lints (the MADR's
"Amendment 2026-10-22, step 3: the caches outside the repository", C1 to C3).

**Decision (owner, 2026-10-22).** The caches go to the user's cache directory, `_common.cache_dir`, `ACP_TOOLS_CACHE` overriding it; the
logs stay in `.cache/tools/`. Added to step 3: `cache_dir` and its tests; the 882 MB `.cache/tools/floor/` this step's run made moved
out of the tree; and the comparison run again, with `make markdownlint` after the floor gate.

### Step 3, the gates, 2026-10-22

The owner committed step 2 as `dc15c42` and approved this step; the two deviations above were decided during it. What changed:

* **`tools/gates.py`:** the 18 gates of its docstring, by name; no name, every gate but `windows` and `linux`; `--all`, every gate.
  `fails_on_output` makes `fix` and `tidy` fail on any output. The host gates call `tools/remote.py tree`, which step 4 adds. A `tools`
  gate, `make check-tools`, which the scratch directory's gate set had no counterpart for, is added.
* **`tools/floor.py`:** the version from `go.mod`'s `go` line; `floor_env` forces that toolchain and puts the Go caches in
  `_common.cache_dir("floor")`; it fails when `go version` names another toolchain. Its log is `.cache/tools/floor/floor.log`.
* **`tools/platforms.py`:** the 13 per-platform checks, `--tree` for another tree.
* **`tools/_common.py`:** `cache_dir`, outside the repository (the second deviation).
* **`tools/test_tools.py`:** 18 tests: the gates' output rule, their selection and their definitions; `cache_dir`'s default and override;
  `floor_env`; the `go` line.

**The 882 MB of Go caches** the step's first floor run made under `.cache/tools/floor/` were moved to the floor's cache directory, not
downloaded again; `.cache` is 56 KB, `make markdownlint` lints 27 files, 0 issues.

**The comparison,** `tools/gates.py`, then the scratch directory's gate set, on the same tree, one after the other. Its first run
disagreed on `mdlint` alone (the deviation); run again after the change, every gate with a counterpart agrees:

| Gate | `tools/gates.py` | The scratch directory's |
| :--- | :--- | :--- |
| `regen`, `precheck`, `newfiles`, `nojsonv2`, `winvet`, `records`, `mdlint`, `idscan` | 0 each | 0 each |
| `lint`, `fix`, `tidy`, `api`, `race3`, `floor`, `platforms` | 0 each | 0 each |
| `tools` | 0 | none |

Then `tools/gates.py mdlint records`, after the floor gate had run: both 0.

**Planted breaks,** each in its own scratch copy; 6 of 6 failed as required:

| Break | The check that failed |
| :--- | :--- |
| a diff gate passing on output | `test_output_fails_a_diff_gate`: "output on exit 0 must fail" |
| the hosts in the default set | `test_selection`: "'windows' unexpectedly found" |
| the cache in the repository | `test_default_is_outside_the_repository`: "is inside the repository" |
| the `go` line misread as the `toolchain` line | `test_go_line`: "'1.27.3' != '1.26.10'" |
| `floor.py` without `GOTOOLCHAIN` | `floor.py`: "`go version` names 'go version go1.27.3 darwin/arm64', not go1.26.10" |
| a variable declared and not used in the Linux and FreeBSD file | `platforms.py`: "8 of 13 passed; failed: lint-linux, lint-freebsd, vet-linux-amd64, vet-linux-arm64, vet-freebsd-amd64" |

The cache plant's first run, in a copy of `tools/` alone, erred instead: the test calls `_common.repo()`, which needs a repository. Run
in a clone, it passed unplanted and failed as required planted.

**Gates,** by `tools/gates.py`, each exit 0: `make pre-add-check`, "176 file(s) clean (gofmt, golangci-lint, go vet, go test,
govulncheck)"; `make check-tools`, "Ran 18 tests"; `make lint`, "0 issues." four times; `platforms`, "13 of 13 passed"; `make api`,
"against v1.20.0: 0 incompatible, 0 compatible"; the floor gate at go1.26.10 passed; and the rest of the table above. The host gates
wait for step 4.

### Step 4, the hosts, 2026-10-22

The owner committed step 3 and approved this step, setting the two hosts' variables for its runs; they were set on each command only.
What changed:

* **`tools/remote.py`:** `tree --host H`, the tree's files to `acp-tools-<random>` in the remote home, `go test -count=1 -shuffle=on
  ./...` in both modules, the directory removed in a `finally`; `pkg --host H [--goarch A] PKG FLAG...`, the test binary cross-compiled
  with `CGO_ENABLED=0`, copied, run with each flag quoted by `shlex.quote`, then removed.
* **`tools/leftovers.py`:** `tasklist` on the Windows host; a row whose image name is a Go test binary's, ending in `.test.exe`, or
  equals a `--name`, compared whole and without case.
* **`tools/test_tools.py`:** 21 tests: the remote command's quoting, the fresh directory's name, the exact image names.

**On the hosts,** one script, 9 of 9 held:

| Check | Result |
| :--- | :--- |
| `gates.py windows` and the scratch directory's Windows run | both 0 |
| `gates.py linux` and the scratch directory's Linux run | both 0 |
| `remote.py pkg --host windows ./agentproc '-test.run=^(TestKill\|TestKillAfterExit)$'` | exit 0; ran and passed exactly those two |
| the same on the Linux host | exit 0; the same two |
| `leftovers.py` | "350 rows; test processes left: 0" |
| `leftovers.py --name tasklist.exe` | exit 1: `tasklist.exe` is listed while it runs |
| a failing test planted in a clone, `remote.py tree --host windows` | exit 1, "--- FAIL: TestPlantedFailure" |
| the same on the Linux host | exit 1, the same |
| `leftovers.py` after the runs | "353 rows; test processes left: 0" |

Afterwards neither host's home held an `acp-tools-` directory; that check, a scratch script, was first seen to catch one planted on the
Linux host, and the planted directory was removed.

**Planted breaks,** offline, each in a copy of `tools/`; 3 of 3 failed as required:

| Break | The check that failed |
| :--- | :--- |
| a substring image match | `test_exact_image_names`: "tasklist.exe must not match t.exe" |
| the flags not quoted | `test_flags_quoted_for_the_remote_shell` |
| a fixed remote directory | `test_fresh_relative_directory` |

With the planted failing test above, the PLAN's row for `remote.py` is met.

**Gates,** `tools/gates.py --all`, the hosts included, each exit 0: all 18 gates; `make pre-add-check`, "176 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck)"; `make check-tools`, "Ran 21 tests"; `platforms`, "13 of 13 passed"; `make api`, "against v1.20.0: 0 incompatible, 0 compatible"; the floor gate at go1.26.10 passed; `windows` and `linux` passed. `--all` is now matched against the scratch directory's
gate set, as the deviation during step 3 has it.

### Step 5, the plants, 2026-10-22

The owner committed step 4 and approved this step. What changed:

* **`tools/plants.py SPEC.json`:** each plant in its own copy of the tree, in `_common.cache_dir("plants")`, outside the repository,
  without `.git`, `.cache`, `.tools`, `.gocache` or macOS metadata; each edit's text asserted to occur once, else the plant is refused;
  local plants by `go test -count=1 -timeout -run`, host plants by `remote.py pkg --tree <copy>`; the first line the failing test wrote
  printed; the copies removed unless `--keep`; exit 0 only when every plant failed with its wanted text, 2 for a malformed spec.
* **`tools/remote.py`:** `pkg --tree DIR`, to build from a copy that has no `.git`; the PLAN's design had `plants.py` call `remote.py
  pkg`, and a copy cannot name its own tree to git.
* **`tools/test_tools.py`:** 27 tests; `PlantsTest` runs `plants.py` against a one-test Go module, `ACP_TOOLS_CACHE` in a temporary
  directory, so that no test writes the user's cache.

**The replay,** the PLAN's acceptance for this step: 0030's step 7 plants, as a spec, and its step 8 breakaway plant on the Windows host;
4 of 4 failed as required, the first three with the lines
[0030-PLAN-record-initialize-on-arrival.md](0030-PLAN-record-initialize-on-arrival.md) records:

| Plant | The line the failing test wrote |
| :--- | :--- |
| `Kill` running `Close` | "kill_test.go:23: Kill took longer than 3s" |
| `exitedWithin` not watching `hurry` | "kill_test.go:41: Kill took longer than 3s" |
| `ErrKilled` recorded though nothing was sent | "kill_test.go:65: Kill = agentproc: agent killed, want nil, the agent's exit of status 0" |
| `BREAKAWAY_OK` left out, on the Windows host | "the parent-breakaway agent exited before it was ready: exit status 1" |

Its first run printed each test's `--- FAIL` line rather than what the test wrote; `first_failure` now prefers the `_test.go:` line, and
the table is the second run's. After both, `leftovers.py` found no test process on the Windows host, and the plants' cache directory was
empty.

**Planted breaks** in `plants.py`, each in a copy of `tools/`; 4 of 4 failed as required:

| Break | The check that failed |
| :--- | :--- |
| an edit that does not apply accepted | `test_edit_that_does_not_apply` |
| the wanted text ignored | `test_want_text_absent`: "0 != 1" |
| the copy keeping `.git` | `test_copy_outside_the_tree_without_git`: "the copy must leave .git out" |
| the plant applied to the tree itself | `test_copy_outside_the_tree_without_git`: "1 != 0" |

**Gates,** `tools/gates.py --all`, each exit 0: all 18 gates; `make pre-add-check`, "176 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck)"; `make check-tools`, "Ran 27 tests"; `platforms`, "13 of 13 passed"; `make api`, "against v1.20.0: 0 incompatible, 0 compatible"; the floor gate at go1.26.10 passed; `windows` and `linux` passed.

### Deviation, 2026-10-22, during step 6: the publication's main checks on origin's main

**Found.** `publication.py v1.20.0 4fa7204` held 8 of 10. The two that missed, "origin main is the local main" and "CI on main's head",
compared origin's `main` with the local `main`, which was 7 commits ahead with this PLAN's unpushed work: they checked the local branch,
not what was published, and fail any replay made while work is unpushed.

**Decision (owner, 2026-10-22).** The two check what is published: the tag's commit is on origin's `main`, and CI on origin main's head
succeeded. That nothing is left unpushed at a release stays `make push-guard`'s. The alternative was to keep them and push first. Added:
a test of the ancestry check, and its planted break.

### Step 6, the release, 2026-10-22

The owner committed step 5 and approved this step; the deviation above was decided during it. What changed:

* **`tools/release_body.py VERSION REV`:** the report found at `REV` by its name; the text under `### VERSION` up to "Sources not in the
  release text"; every relative link, a fragment kept, made absolute at the tag on the origin remote's GitHub repository, its target
  asserted to exist at `REV`; the body scanned by `idscan.py --paths`, and written to `.cache/tools/release/`.
* **`tools/publication.py VERSION COMMIT BODY`:** the ten checks, two of them on origin's `main` (the deviation); the module cache in
  `_common.cache_dir("publication")`, removed after.
* **`tools/consumer.py VERSION`:** the floor consumer, its module and toolchain read from `go.mod`, its caches in
  `_common.cache_dir("consumer")`, its module removed after.
* **`tools/watch_ci.py REV`:** the revision resolved to its full id first, then its run found, watched, and its jobs printed.
* **`tools/_common.py`:** `module_path`, `github_repo` (https and SSH forms), `origin_repo`.
* **`tools/test_tools.py`:** 35 tests, the release tools' offline parts among them.

**Live, against `v1.20.0`:**

| Check | Result |
| :--- | :--- |
| `release_body.py v1.20.0 4fa7204` | 89 lines, 3 links, the scan clean; **byte for byte** the published body and the file it was published from, 7,498 bytes |
| `publication.py v1.20.0 4fa7204 <body>` | first 8 of 10, the local branch 7 commits ahead (the deviation); then 10 of 10 |
| `consumer.py v1.20.0` | go1.26.10; `go get` added `v1.20.0`; the program printed "Close: <nil>", "dropped 0 0", "protocol version 1"; built for Windows; `go.mod` kept `go 1.26.10`, no `toolchain` line |
| `watch_ci.py 4fa7204`, a short id | run `39716056732`, completed, success, its six jobs listed |

**Planted breaks,** each in a clone with the step's tools; 5 of 5 failed as required:

| Break | The check that failed |
| :--- | :--- |
| a missing link target accepted | `test_missing_target_refused`: "0 != 1" |
| a relative link left relative | `test_body_extracted_and_links_made_absolute` |
| the ancestry check reversed | `test_tag_on_main`: "an older tag is on main" |
| a short id not resolved | `test_watch_ci_resolves_a_short_id`: the short id returned as it was |
| a `toolchain` line accepted | `test_consumer_go_mod`: "True is not false" |

The PLAN's row for `release_body.py`, a link target missing, is the first.

**Gates,** `tools/gates.py --all`, each exit 0: all 18 gates; `make pre-add-check`, "176 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck)"; `make check-tools`, "Ran 35 tests"; `platforms`, "13 of 13 passed"; `make api`, "against v1.20.0: 0 incompatible, 0 compatible"; the floor gate at go1.26.10 passed; `windows` and `linux` passed.

### Step 7, CI and docs, 2026-10-22

The owner committed step 6 and approved this step. What changed:

* **`.github/workflows/ci.yml`:** the records step, renamed "markdownlint, actionlint, records, tools", also runs `make check-tools`.
* **`tools/README.md`,** new: the variables, the logs and caches, a table of the tools, a plant's JSON form, and `make check-tools`.
* **`docs/architecture.md`:** a section "Verification tools", and `tools/` in the tree and `check-tools` on the `Makefile`'s line.
* **`AGENTS.md`:** under "Pre-add checks", the tools' place and `make check-tools` before a commit that changes them.
* **`tools/test_tools.py`:** `ReadmeTest`, which resolves `tools/README.md`'s relative links; 36 tests.

**Before CI ran it,** `make check-tools` ran on the Linux host, from a copy of the tree: Python 3.14.7, procps-ng 4.0.4's `ps`, "Ran 35
tests", OK; `procs.py`'s `ps -axo` holds there. CI's image has an older Python; its run, in step 8, is what shows the tools hold there.

**Found:** `scripts/check_records.py` checks the links of the records, `README.md`, `docs/README.md`, `docs/architecture.md` and
`docs/guides/` (its docstring and `:223`–`:229`), not `tools/README.md`, so its one link was checked by nothing; a planted broken link
there passed `make check-records`. `ReadmeTest` now checks it, inside `make check-tools`. It is a test of the tools' own docs and changes
no file outside the step's scope.

**Planted breaks,** each in a clone with the step's files; 2 of 2 failed as required, each first seen to pass unplanted:

| Break | The check that failed |
| :--- | :--- |
| a failing assertion in a tool test | `make check-tools`, the command CI's records step runs under `set -euo pipefail`: "FAILED (failures=1)", exit 2 |
| a broken link in `tools/README.md` | `make check-tools`: "tools/README.md links ../docs/decisions/0031-MADR-gone.md, which does not exist" |

**Gates,** `tools/gates.py --all`, each exit 0: all 18 gates; `make pre-add-check`, "176 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck)"; `make check-tools`, "Ran 36 tests"; `make markdownlint`, 28 files, 0 issues; `platforms`, "13 of 13 passed"; `make api`, "against v1.20.0: 0 incompatible, 0 compatible"; the floor gate at go1.26.10 passed; `windows` and `linux` passed; and `actionlint` v1.7.12 over the workflows, exit 0, nothing printed.

### Step 8, verification and close, 2026-10-22

The owner committed step 7 as `b42620c` and pushed `main`.

**Gates on the committed tree,** `git status --porcelain` empty before them: `tools/gates.py --all`, all 18 gates, each exit 0;
`make pre-add-check`, "176 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck)"; `make check-tools`, "Ran 36 tests";
`make markdownlint`, 28 files, 0 issues; `platforms`, "13 of 13 passed"; `make api`, "against v1.20.0: 0 incompatible, 0 compatible";
the floor gate at go1.26.10; both hosts. Afterwards `leftovers.py` found no test process on the Windows host.

**Before the push:** a fetch, `main` 9 commits ahead of `origin/main`, 0 behind; `make push-guard`, "9 outgoing commit(s),
4fa7204..b42620c, main to origin", "the guard passed (exit 0)"; and `tools/idscan.py --range origin/main..main`, "24 file(s),
22 pattern(s), 0 match(es)".

**CI** on `b42620c`, run `39792270232`, watched by `tools/watch_ci.py b42620c`: all six jobs passed, ubuntu-24.04, macos-15 and
windows-2025 at 1.26.10 and at 1.27.3. In the Linux job at the newest Go, the step "markdownlint, actionlint, records, tools" ran
`make check-tools`: `py_compile` on every tool, then "Ran 36 tests", OK, on the image's Python.

**Found, not changed:** `watch_ci.py` printed the run as "completed success" with that one job still "in_progress": GitHub had marked
the run complete while the job's status lagged its steps, every one of which had completed with success; a later `gh run view` gave
the job "completed". The verdict was the run's own conclusion, and right; the job list was a snapshot taken a moment early. A later
change may wait for every job's status as well.

Every acceptance criterion is met, and this PLAN and its row in `docs/README.md` are now `complete`.

### Amendment, 2026-10-22, after step 8: step 9 proposed

**Found.** Step 8's record notes that `watch_ci.py` printed a job "in_progress" beside the run's "success". Assessed at the owner's
request, with each figure taken that day, the repository only read: the job list was read before the jobs had settled (the MADR's W1);
`watch_ci.py` reads only the newest of a commit's runs (W2), so in step 6 it watched `4fa7204`'s tag run, `39716056732`, and not
`main`'s, `39714586565`; it ignores the watch's own status and does not catch a timeout (W3); and `publication.py`'s "CI on origin main's
head" reads the newest run of the head commit, at a release the tag's (W4), so in step 6, as in
[0030-PLAN-record-initialize-on-arrival.md](0030-PLAN-record-initialize-on-arrival.md)'s publication, it read `39716056732` and not
`main`'s run. `main`'s run succeeded, so no recorded result changes; the records stand as written.

**Proposed.** The design's last section, the tests and planted breaks marked "added 2026-10-22, after step 8, proposed", and step 9; the
MADR's questions 5 and 6. Nothing is changed until the owner approves; this PLAN then returns to `in-progress` until step 9 closes it,
and step 8's close stands as recorded.

**Decided (owner, 2026-10-22).** W1 to W4 as proposed; question 5, the owner's variant, up to 180 seconds, reading every 10; question
6 as recommended. Step 9 is approved, and this PLAN is `in-progress` again until it closes.

### Deviation, 2026-10-22, during step 9: the main check's run

**Found.** Step 9's acceptance has `publication.py`'s "CI on origin main's head" name `39714586565`, `main`'s run of `4fa7204`. It named
`39792270232`, `main`: since the owner pushed step 8, origin's `main` is `b42620c`, and the check reads `main`'s run of origin main's
head, as the amendment decided. The acceptance assumed `main` still at the released commit.

**Decision (owner, 2026-10-22).** The acceptance is corrected: the check names `main`'s run of origin main's head; and, at the released
commit, the check's query, `gh run list --commit 4fa7204 --branch main`, gives `39714586565`, `main`, where the old one, `--limit 1`,
gives `39716056732`, `v1.20.0`, both run that day. `main_run`'s test and its planted break cover the choice. The alternative was a
`--head` option for the replay alone.

### Step 9, the CI runs, 2026-10-22

The owner committed step 8's close as `4083a6e`, decided the amendment and approved this step; the deviation above was decided during it.
What changed:

* **`tools/watch_ci.py`,** rewritten around `runs_for`, `settled`, `verdict`, `wait_settled` and `watch`: every run of the commit, or of
  `--branch`, waited for (`--minutes`, 20; `--more`, 30 s); each watched, the watch's exit status kept, a timeout a verdict; each read
  again until every job is "completed" (`--settle`, 180 s; `--every`, 10 s); each printed with its id, branch, event, status,
  conclusion, the watch's status and every job. A run passes only when the watch exited 0, its jobs settled, and its and every job's
  conclusion is success or, for a job, skipped. `find_run` is gone.
* **`tools/publication.py`:** "CI on origin main's head" asks `--commit <head> --branch main` and takes `main_run`'s choice, naming its
  run and branch; "CI on the tag" names its run and branch.
* **`tools/test_tools.py`:** `WatchCiTest`, five tests; `find_run`'s assertions removed with it; 41 tests.
* **`tools/README.md`:** `watch_ci.py`'s row.

**Live,** each exit 0:

| Check | Result |
| :--- | :--- |
| `watch_ci.py 4fa7204` | "2 run(s) for 4fa7204: 39716056732, 39714586565"; each "completed success", watch exit 0, six jobs "completed success" |
| `watch_ci.py 4fa7204 --branch main` | "1 run(s) for 4fa7204: 39714586565", `main` |
| `watch_ci.py b42620c` | run `39792270232`, `main`: six jobs "completed success", none "in_progress" |
| `publication.py v1.20.0 4fa7204 <body>` | 10 of 10; "CI on origin main's head", "run 39792270232, main"; "CI on the tag", "run 39716056732, v1.20.0" |
| at the released commit, `gh run list --commit 4fa7204 --branch main` | `39714586565`, `main`; the old query, `--limit 1`, `39716056732`, `v1.20.0` |

The settling cannot be made to lag on demand; `test_wait_settled` shows it, with a fake clock: three reads, 10 s apart, until the jobs
settle, and an unsettled run reported so at 180 s.

**Planted breaks,** each in a copy of `tools/`; 6 of 6 failed as required:

| Break | The check that failed |
| :--- | :--- |
| the verdict by the run's conclusion alone | `test_verdict_reads_every_job`: "a job's failure under a run's success must fail" |
| no wait for the jobs to settle | `test_wait_settled`: the lagging read returned |
| the first run alone | `test_every_run_of_the_commit`: "both runs of the commit" |
| `--branch` ignored | `test_every_run_of_the_commit`: "the branch's alone" |
| `TimeoutExpired` not caught | `test_watch_timeout_is_a_verdict`: the exception reached the test |
| the main check without its branch | `test_main_run_of_a_released_commit`: "main's run, not the newer tag's" |

**Gates,** `tools/gates.py --all`, each exit 0: all 18 gates; `make pre-add-check`, "176 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck)"; `make check-tools`, "Ran 41 tests"; `make markdownlint`, 28 files, 0 issues; `platforms`, "13 of 13 passed"; `make api`, "against v1.20.0: 0 incompatible, 0 compatible"; the floor gate at go1.26.10 passed; `windows` and `linux` passed.

**The push and CI.** The owner committed the step as `c39195a` and pushed `main`. Before it, `make push-guard` passed over what was then
outgoing. `tools/watch_ci.py HEAD`, the rewritten tool: "1 run(s) for c39195a: 39800319491"; "run 39800319491 (main, push): completed
success; watch exit 0"; its six jobs, ubuntu-24.04, macos-15 and windows-2025 at 1.26.10 and at 1.27.3, each "completed success" as read,
none in progress. In the Linux job at the newest Go, the step "markdownlint, actionlint, records, tools" ran `make check-tools`: "Ran 41
tests", OK.

Every acceptance criterion is met, step 9's as the deviation during it corrected; this PLAN and its row in `docs/README.md` are
`complete` again.

### Amendment, 2026-10-22, after step 9: step 10, `find.py`'s broken pipe

**Found.** During the investigation that [0032-REPORT-windows-floor-runtime-crashes.md](../reports/0032-REPORT-windows-floor-runtime-crashes.md)
records, `python3 -B tools/find.py lines <log> -- ... | head -n 12` printed its matches, then ended with a traceback,
"BrokenPipeError: [Errno 32] Broken pipe", at `tools/find.py:37`, its `print`: once `head` has read its lines and closed the pipe, the
next write fails, and nothing in the tool handles it.

**Decided (owner, 2026-10-22).** The owner asked for the tool to be fixed. Step 10 adds it, with its test and planted break; this PLAN is
`in-progress` until step 10 closes.

### Step 10, `find.py`'s broken pipe, 2026-10-22

The owner asked for the fix. What changed:

* **`tools/_common.py`:** `main_guard(main, argv)`, the handling Python's documentation of `signal` gives in its "Note on SIGPIPE":
  `main` run and standard output flushed inside the `try`; on `BrokenPipeError`, standard output pointed at the null device, "Python
  flushes standard streams on exit; redirect remaining output to devnull to avoid another BrokenPipeError at shutdown", and status 1,
  "Python exits with error code 1 on EPIPE" (both the documentation's comments, read that day).
* **`tools/find.py`:** run through `main_guard`; its docstring names status 1 for a reader that closes early.
* **`tools/test_tools.py`:** `test_reader_that_closes_early`, 200,000 matching lines read by one `readline` and the pipe closed: no
  traceback, no `BrokenPipeError`, status 1; 42 tests.

**Seen to fail first:** the test, run before the fix, failed with the tool's own traceback, "BrokenPipeError: [Errno 32] Broken pipe";
and `python3 -B tools/find.py lines <a log> -- runtime | head -n 2`, after it, printed two lines with no traceback, `find.py`'s status 1
and `head`'s 0.

**Planted breaks,** each in a copy of `tools/`:

| Break | Result |
| :--- | :--- |
| the broken pipe not handled, `main` called directly | failed as required: "'Traceback' unexpectedly found" |
| standard output not pointed at the null device | **passed**: the test does not reach it |

The second is not the PLAN's planted break; it was tried to see whether the test covers the redirect, and it does not: in these runs the
failed write left nothing in the buffer, so the flush at exit had nothing to fail on. The redirect is kept as the documentation gives it,
on the documentation's word, and no test claims it.

**Gates,** `tools/gates.py --all`, each exit 0: all 18 gates; `make pre-add-check`, "176 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck)"; `make check-tools`, "Ran 42 tests"; `make markdownlint`, 29 files, 0 issues; `platforms`, "13 of 13 passed"; `make api`, "against v1.20.0: 0 incompatible, 0 compatible"; the floor gate at go1.26.10 passed; `windows` and `linux` passed.

**The push and CI.** The owner committed the step as `bf0b215`, after the report as `e93457b`, and pushed `main`. Before it,
`make push-guard`, "2 outgoing commit(s), 1fc1d21..bf0b215", passed, and `tools/idscan.py --range origin/main..main` found 0 matches in
the 6 files. `tools/watch_ci.py bf0b215`: run `39825211145`, `main`, "completed success; watch exit 0", its six jobs each "completed
success", the Windows floor job among them. In the Linux job at the newest Go, `make check-tools`: "Ran 42 tests", OK.

Every acceptance criterion is met; this PLAN and its row in `docs/README.md` are `complete` again.

### Amendment, 2026-10-22, after step 10: step 11 proposed

**Found.** Step 10 routed `find.py` alone through `main_guard`; the twelve other tools' entry points call `main` directly
(`if __name__ == "__main__": sys.exit(main(sys.argv[1:]))`, read from each), so any of them piped into `head` can end with the
traceback `find.py` did. Besides `find.py`, `procs.py`, `idscan.py` and `leftovers.py` print a line for each thing they find; the
others print a line per gate, check or job.

**Proposed, at the owner's request.** Step 11 above, its tests and planted breaks. Nothing is changed until the owner approves; this PLAN
then returns to `in-progress` until step 11 closes.

**Approved (owner, 2026-10-22).** Step 11 as proposed; this PLAN is `in-progress` until it closes.

### Deviation, 2026-10-22, during step 11: the guard's test

**Found.** Step 11's proposed test pipes `procs.py` into a reader that closes after one line. `procs.py` prints a line per matching
process, and its whole output usually fits the pipe's buffer, 64 KB, before the reader closes, so no `BrokenPipeError` arises and the
test would pass with the guard or without it; on a CI runner, with fewer processes, the more so.

**Decision (owner, 2026-10-22).** The guard is tested itself, in a subprocess: a `main` that writes to a reader closing after one line,
and a `main` that raises `BrokenPipeError`, each run through `_common.main_guard`, must exit 1 without a traceback; with the test that
reads every tool's entry point, every tool is covered through the one guard. The alternative was a `procs.py` test made big with a few
hundred child processes. The test list and the planted-break row are annotated above.

### Step 11, every tool through `main_guard`, 2026-10-22

The owner approved this step; the deviation above was decided during it. What changed:

* **Twelve tools,** `consumer.py`, `floor.py`, `gates.py`, `idscan.py`, `leftovers.py`, `plants.py`, `platforms.py`, `procs.py`,
  `publication.py`, `release_body.py`, `remote.py` and `watch_ci.py`: each ends `sys.exit(_common.main_guard(main, sys.argv[1:]))`, by a
  script that asserted each change applied once; `procs.py` now imports `_common`.
* **`tools/test_tools.py`:** `GuardTest`: `test_every_entry_point` reads each tool's last two lines; `test_reader_that_closes_early` and
  `test_broken_pipe_raised` run `main_guard` in a subprocess; 45 tests.

**Seen to fail first:** `test_every_entry_point`, before the tools changed, failed naming the twelve; the guard's two tests passed, the
guard being step 10's. After the change, `make check-tools`: "Ran 45 tests", OK; `procs.py`, `idscan.py` and `gates.py` with `--help`,
and `procs.py` with a text no process holds ("none"), each exit 0.

**Planted breaks,** each in a copy of `tools/`; 2 of 2 failed as required:

| Break | The check that failed |
| :--- | :--- |
| one tool's entry point calling `main` directly, `idscan.py` | `test_every_entry_point`: "['idscan.py'] != []" |
| the guard catching nothing, `except ZeroDivisionError` | `GuardTest`, both guard tests: "'Traceback' unexpectedly found" |

**Gates,** `tools/gates.py --all`, each exit 0: all 18 gates; `make pre-add-check`, "176 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck)"; `make check-tools`, "Ran 45 tests"; `make markdownlint`, 29 files, 0 issues; `platforms`, "13 of 13 passed"; `make api`, "against v1.20.0: 0 incompatible, 0 compatible"; the floor gate at go1.26.10 passed; `windows` and `linux` passed.

**The push and CI.** With [0033-PLAN-traceback-on-windows-floor-ci.md](0033-PLAN-traceback-on-windows-floor-ci.md)'s commits,
at the owner's request, "Commit all 3, then push to main", the agent committed this work as `a716dfe` (records), `57ce3f2` (the
workflow) and `2f58588` (0031's step 11), and pushed `main`, `b533de4..2f58588`, after `make push-guard`, "3 outgoing commit(s)", passed and `tools/idscan.py
--range` found 0 matches in 19 files. CI run `39841121410`, attempt 1: five jobs passed, and `validate (windows-2025, 1.27.3)` crashed in
the root package's tests, "fatal error: found pointer to free object", a third occurrence of what
[0032-REPORT-windows-floor-runtime-crashes.md](../reports/0032-REPORT-windows-floor-runtime-crashes.md) records, on a commit that
changes no Go file. At the owner's request the failed job was rerun: attempt 2, `tools/watch_ci.py 2f58588`, "completed success", all
six jobs "completed success". In the Linux job at the newest Go, `make check-tools`: "Ran 45 tests", OK.

Every acceptance criterion is met; this PLAN and its row in `docs/README.md` are `complete` again.
