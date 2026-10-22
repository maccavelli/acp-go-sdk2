---
status: in-progress
date: 2026-10-21
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
  "Publication of `v1.20.0`", in a scratch module cache.
* **`consumer.py VERSION`.** A fresh module at the `go` line, with that toolchain forced, `go get` of the version, a program that starts
  `sh -c "cat >/dev/null"` through `agentproc` with an `OutputTap`, built and run, and built for Windows; its `go.mod` must keep the `go`
  line and gain no `toolchain` line.
* **`watch_ci.py SHA`.** Waits up to 20 minutes for the run of that full commit id, then watches it; prints each job's conclusion; exits
  with the run's.
* **`find.py`.** `lines PATH|GLOB... -- TEXT...`, each matching line with its file and number; `files ROOT PATTERN...`, the matching paths,
  `.git` left out. **`procs.py TEXT...`**, the processes whose command line holds any, this one excluded; exit 1 when any.

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
