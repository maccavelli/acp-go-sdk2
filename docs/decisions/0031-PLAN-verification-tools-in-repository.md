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
   scratch directory's gate run of 0030's steps.
4. **The hosts:** `remote.py`, `leftovers.py`. Both hosts: `remote.py tree` passes; a planted failing test fails it; `remote.py pkg` runs
   a single test by pattern, `|` in it; `leftovers.py` catches a planted name.
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
