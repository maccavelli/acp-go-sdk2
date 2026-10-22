---
status: accepted
date: 2026-10-21
decision-makers: Repository owner
---

# Keep the maintainer's verification tools in the repository, in `tools/`, with every identifier kept out

## Context and Problem Statement

The PLANs of this repository verify each step with more than `make pre-add-check`: a gate set run one gate at a time with each exit
status recorded, the tests on a Linux host and a Windows host reached over SSH, the Go floor, planted breaks in scratch copies, and, at a
release, the release body, the publication on GitHub and the Go module proxy, and a consumer built at the floor. Their records cite these
runs, for example [0030-PLAN-record-initialize-on-arrival.md](0030-PLAN-record-initialize-on-arrival.md)'s gates and its "Publication of
`v1.20.0`".

The tools that run them have never been in the repository. They are Python scripts in an agent session's scratch directory, which the session
deletes, and they were copied forward from one record to the next. On 2026-10-21 the owner asked for them to be kept in the repository,
in directories named `tools` and `scripts`, and chose the scope: the reusable tools, not the one-off scripts.

Every figure below was taken that day; the repository was only read, at `4fa7204`.

### What exists

* **T1. The scratch directory holds 416 Python scripts of its own,** 25,778 lines, outside the copied trees and module caches under it. By name:
  69 write records, 55 run planted breaks, 11 run gates, 11 reach a host, 43 check a release or its publication, 5 search or list
  processes, and 222 are probes or other one-offs.
* **T2. Most cannot be committed as they are.** 322 of the 416 hold an identifier the repository's rule keeps out of every commit: the
  local account name in an absolute path, a host's name, or a scratch path (`AGENTS.md`, "Identifiers").
* **T3. The reusable ones,** each used across several records:
  * the gate set: the generator's regeneration unchanged, `make pre-add-check`, the pre-add check of new Go files,
    `GOEXPERIMENT=nojsonv2 go test`, `GOOS=windows go vet` in both modules, `make check-records`, `make markdownlint`, an identifier scan
    of the changed files, `make lint`, `go fix -diff` and `go mod tidy -diff` in both modules, `make api`, the root's tests under `-race`
    three times, the floor gate, and the tests on both hosts;
  * the floor gate: `go vet` and `go test -race` in both modules with `GOTOOLCHAIN` at the `go` line, the root's leak guard on;
  * the per-platform checks: `golangci-lint` for `linux` and `freebsd` and `go vet ./agentproc` for 0029's platforms;
  * the host runners: the working tree copied to a fresh directory on a host, its tests run, the directory removed; or one package's test
    binary cross-compiled, copied and run with flags; and, on Windows, a check that no test binary is left running;
  * the planted-break runner: a scratch copy per break, an exact edit asserted to apply once, a named test that must fail with a given
    text, locally or on a host;
  * the release tools: the release body extracted from a report with its links made absolute and checked; the publication check, ten
    points; a consumer at the floor; and a watcher that waits for a commit's CI run;
  * the search and process helpers, which the owner's rule requires in place of `grep` and `pgrep`.
* **T4. Two need a source outside the repository.** The identifier scan needs the identifiers themselves; the host runners need the hosts'
  names. Neither may be committed (T2).
* **T5. A deny list already exists for the first.** The global pre-push disclosure guard reads its identifiers from `DISCLOSURE_DENY`, or
  `~/.config/git/disclosure-deny` when that is unset, one Python regex per line, a tab, a placeholder, and an optional `identity` field,
  and keeps none in its own file (its docstring). `make push-guard` runs that guard (`scripts/push_guard.py:25`).
* **T6. The repository already carries Python.** `scripts/` holds `check_records.py`, `check_api.py` and `push_guard.py`, run by
  `make check-records`, `make api` and `make push-guard`, and `go-precheck.sh`; `.gitignore` ignores `__pycache__/`, `*.py[cod]` and
  `.cache/`. CI runs `make check-records` in its records step. No Python file has a test, and nothing lints Python.
* **T7. Nothing named `tools` exists.** `.tools/`, untracked, holds installed binaries; there is no `tools/`.

## Decision Drivers

* **Nothing identifying is committed** (`AGENTS.md`, "Identifiers"), and the guard refuses a push that carries it (T5).
* **The tools must outlive a session,** so that a record's commands can be run again.
* **Every check is seen to fail on a planted break** before it is relied on, the tools themselves included.
* **Python and the standard library only,** as the owner's rule and `scripts/` already have it (T6); no dependency.
* **The repository's own gates stay where they are:** `make` and CI call `scripts/`, and moving them would change both for no gain.

## Considered Options

* **O1.** A new `tools/` for the reusable verification tools, generalized; `scripts/` keeps the repository's gates.
* **O2.** Everything in `scripts/`, beside the gates.
* **O3.** A Go program under `cmd/`.
* **O4.** Leave them in the scratch directory, as now.

## Decision Outcome

Chosen option: "O1", with the recommended answer to each of questions 1 to 4, decided by the owner on 2026-10-21 (see "Decided
with the owner, 2026-10-21"), because it keeps what `make` and CI call apart from what a maintainer or an agent runs by hand, needs no
new dependency, and puts the tools where a record can name them, with the identifiers they need read from outside the repository.

* O2 mixes about fifteen hand-run tools with the four that `make` and CI call, in one flat directory.
* O3 rewrites working tools in another language and adds a module or a package to the build for code no consumer uses.
* O4 is what the owner asked to end.

### Specification, as decided

* **`tools/`,** standard-library Python, each tool a script run as `python3 tools/<name>.py`, its usage in its docstring and `--help`:
  * `_common.py`: the repository's root from `git rev-parse --show-toplevel`; a runner that records each command's exit status before its
    output is read, and writes the output to a log; the hosts (question 1); the output directory, `.cache/tools/` (ignored, T6) unless
    `--out` names another; (Amended 2026-10-22, during the PLAN's step 3: the logs only; the caches outside the repository; see
    "Amendment 2026-10-22, step 3: the caches outside the repository".)
  * `gates.py`: the gate set of T3, each gate by name, the default every gate but the hosts';
  * `idscan.py`: the changed files, or `--all`, or a commit range, against the deny list (question 2); it prints the file and line of each
    match, never the match;
  * `floor.py`: the floor gate, `GOTOOLCHAIN` taken from the `go` line of `go.mod`, not written in the tool;
  * `platforms.py`: the per-platform checks;
  * `remote.py`: `tree` and `pkg`, on the host `--host linux` or `--host windows` names (question 1), always in a fresh directory removed
    after;
  * `leftovers.py`: the Windows host's process list, test binaries matched by their exact image name;
  * `plants.py`: planted breaks from a JSON file the step writes, outside the repository or under `.cache/`, local or on a host;
  * `release_body.py`, `publication.py`, `consumer.py`, `watch_ci.py`: the release tools of T3, the version, commit and run given as
    arguments;
  * `find.py` (`lines`, `files`) and `procs.py`: the search and process helpers.
* **`scripts/`** keeps `check_records.py`, `check_api.py`, `push_guard.py` and `go-precheck.sh` as they are.
* **The one-off scripts** of T1 stay out (the owner's answer to the scope): the records already hold what they found.
* **Tests,** `tools/test_tools.py`, `unittest`: what runs without a host or the network, against fixtures in temporary directories; `make
  check-tools` runs them, with `python3 -m py_compile` on every tool (question 3).
* **Docs** (question 4): `tools/README.md`; `docs/architecture.md`, a section and the tree; a pointer in `AGENTS.md`.

### Questions

Each has a recommended answer.

1. **Where the hosts' names come from.**
   * Recommended: the environment, `ACP_LINUX_HOST` and `ACP_WINDOWS_HOST`, each an SSH host or alias; a tool that needs one that is unset
     stops with a message naming the variable, and a gate set that includes it fails rather than skips.
   * Alternative: an untracked file, `.cache/tools/hosts.json`.
2. **Where the identifier scan's patterns come from.**
   * Recommended: the guard's deny list, `DISCLOSURE_DENY` or `~/.config/git/disclosure-deny` (T5): one list for the push and the scan.
     Missing or empty, the scan fails.
   * Alternative: an environment variable of its own, `ACP_ID_PATTERN`.
3. **CI.**
   * Recommended: CI's records step also runs `make check-tools`, so a change to a tool is tested where a change to a record is.
   * Alternative: `make check-tools` locally only.
4. **Docs.**
   * Recommended: `tools/README.md` for the tools and their variables, a section of `docs/architecture.md`, and a line in `AGENTS.md`.
   * Alternative: `tools/README.md` alone.

#### Decided with the owner, 2026-10-21

| Question | Answer |
| :--- | :--- |
| 1. Where the hosts' names come from | As recommended: `ACP_LINUX_HOST` and `ACP_WINDOWS_HOST`. |
| 2. Where the identifier scan's patterns come from | As recommended: the guard's deny list. (Amended 2026-10-21, during the PLAN's step 2: the owner adds what it lacked; see "Amendment 2026-10-21, step 2: what the deny list covers".) |
| 3. CI | As recommended: the records step runs `make check-tools`. |
| 4. Docs | As recommended: `tools/README.md`, a section of `docs/architecture.md`, a line in `AGENTS.md`. |

### Consequences

* Good, because a record's commands can be run again, by anyone with the repository and the two hosts.
* Good, because the tools are tested, and seen to fail on planted breaks, as the code they check is.
* Good, because one deny list serves the push guard and the scan, and the repository holds no identifier.
* Neutral, because the one-off scripts are not kept: a record's figures stay its evidence, as now.
* Bad, because about fifteen tools, rewritten from the scratch directory's versions, are code to maintain, outside `go test`.
* Bad, because the host tools need two environment variables and SSH access that CI does not have, so CI tests only their offline parts.

### Confirmation

The PLAN makes each claim a check, seen to fail on a planted break: each tool's offline parts by `make check-tools`; `gates.py` agreeing,
gate by gate, with the scratch directory's gate run on the same tree; `idscan.py` finding a planted identifier from the deny list and printing
only its place; `remote.py` and `leftovers.py` on both hosts; `plants.py` reproducing a recorded set of planted breaks;
`release_body.py` reproducing the published `v1.20.0` body byte for byte, and `publication.py` holding 10 of 10 for `v1.20.0`; and
`idscan.py --all` over `tools/` finding nothing.

## Pros and Cons of the Options

### O1. `tools/`, generalized

* Good, because what `make` and CI call stays in `scripts/`, unchanged.
* Good, because the tools take their hosts and identifiers from outside the repository.
* Bad, because it is a second directory of Python.

### O2. `scripts/`

* Good, because one directory holds all the Python.
* Bad, because the gates `make` and CI call sit among tools run by hand, and a reader cannot tell which a change affects.

### O3. A Go program

* Good, because `go test` and `golangci-lint` would cover it.
* Bad, because it rewrites working tools, and adds code to the module, or a module, that no consumer uses.

### O4. The scratch directory

* Good, because it costs nothing now.
* Bad, because the tools are lost with each session, and a record's commands cannot be run again.

## Amendment 2026-10-21, step 2: what the deny list covers

Found during the PLAN's step 2, and decided by the owner the same day; question 2's answer stands.

* **D1. The check.** `tools/idscan.py --paths` over a scratch file of one line per kind of identifier the records' scans have looked for,
  each written as the scripts write it; the lines it reports show which kinds the deny list covers, and nothing of the list is printed.
  The list held 17 patterns.
* **D2. What it covers:** the local account name, a home path, the Windows host's remote working path, and the Linux host's alias where
  the scripts use it, after `ssh`.
* **D3. What it lacks:** the Windows host's full name, the second consumer's name, the organization's short name, paths in
  macOS's system and per-user temporary directories, and the word an agent session's scratch directory is named by. The records' scans caught each so
  far; the push guard lets each through today, in every repository.

**Decision (owner, 2026-10-21):** the owner adds patterns for D3 to the deny list, so that the scan and the push guard both catch them;
the alternatives were for the agent to append them with the owner's leave, or a second list for the scan alone (question 2's
alternative). The PLAN's step 2 reruns D1 once they are added, and requires every kind covered. The Linux host's alias alone, a common
word, is left to the list as it stands (D2).

* **D4. The word, added, matched published prose.** With the six patterns the owner added, 23 in all, every kind of D1 is covered, but
  `idscan.py --all` found 11 lines, all the scratch-directory word, in five PLANs of
  [0011-PLAN-engine-tests.md](0011-PLAN-engine-tests.md)'s and [0012-PLAN-schema-fidelity-framework.md](0012-PLAN-schema-fidelity-framework.md)'s
  records, on `origin/main` since 2026-10-11, where it describes past runs and names no one. The guard would also refuse any outgoing
  commit, in any repository, that holds the word.

**Decision (owner, 2026-10-21):** the owner drops the word's pattern from the deny list. It identifies no one, and the paths it guarded
against are caught by the two temporary-directory patterns. The alternative was to keep it and reword the 11 lines in a new commit. The
coverage check of D1 then leaves the word out.

## Amendment 2026-10-22, step 3: the caches outside the repository

Found during the PLAN's step 3, and decided by the owner the same day.

* **C1.** `floor.py` kept its Go caches under `.cache/tools/floor/`, as the specification said: 882 MB once it had run, the downloaded
  go1.26.10 toolchain with its Markdown files among them.
* **C2.** `make markdownlint` globs every `*.md` but under `node_modules`, `vendor`, `dist`, `.git` and `.tools`
  (`.markdownlint-cli2.jsonc`), git's ignores not consulted: after a floor run it linted 48 files, not 27, and failed with 513 issues in
  the toolchain's docs. `scripts/check_records.py` walks every `*.md` too (`:59`–`:66`), `.cache` not among its skipped parts.
* **C3.** The step's comparison found it: `tools/gates.py` runs `mdlint` before `floor`, so it passed; the gate set run after it failed.

**Decision (owner, 2026-10-22):** a tool's caches, the floor's Go caches now, and later the consumer's module cache and the plants'
copies of the tree, go to the user's cache directory, `ACP_TOOLS_CACHE` when set, else `~/Library/Caches/acp-go-sdk2-tools` on macOS,
`%LOCALAPPDATA%\acp-go-sdk2-tools` on Windows, and `$XDG_CACHE_HOME/acp-go-sdk2-tools` or `~/.cache/acp-go-sdk2-tools` elsewhere, a
directory per tool; `.cache/tools/` holds the logs alone. The alternative was to exclude `.cache` in `.markdownlint-cli2.jsonc` and
`scripts/check_records.py`, two copied templates, leaving about a gigabyte in the tree for editors, indexers and backups to walk.

## Amendment 2026-10-22, after step 8: the CI runs a tool reads, and when it reads them

Found when the PLAN's step 8 watched its push, assessed at the owner's request the same day, and decided by the owner then (see "Decided
with the owner, 2026-10-22, the CI amendment"). The decisions above are
unchanged; what changes is how two of the tools read GitHub Actions. Every figure below was taken that day with gh 2.102.0, the
repository only read, at `b42620c`.

### What was found

* **W1. A job list taken before the jobs settled.** `tools/watch_ci.py b42620c` printed "watch_ci: run completed success" with five jobs
  "success" and the sixth, "validate (ubuntu-24.04, 1.27.3)", "in_progress" (its output, kept outside the tree). A later
  `gh run view 39792270232` gave that job "completed success", every one of its steps "completed success", its `completedAt`
  14:05:52Z, and the run's `updatedAt` 14:09:44Z. The tool reads the run once, as soon as `gh run watch` returns
  (`tools/watch_ci.py:57`–`:58`), prints each job as that read has it, and decides by the run's `conclusion` alone (`:64`). So the run's
  verdict was GitHub's final one, and right, but the job list beside it was a moment early, and a record quoting it would contradict
  itself.
* **W2. One run of several.** `gh run list --commit` filters by commit (its help: "Filter runs by the SHA of the commit") and lists the
  newest first: for `4fa7204`, the released commit, it gives the tag's push, `39716056732`, `v1.20.0`, created 03:17:51Z, then `main`'s
  push, `39714586565`, created 03:01:41Z. `find_run` takes the first run whose commit matches (`:30`–`:34`), so `watch_ci.py 4fa7204`, in
  the PLAN's step 6, watched the tag's run and never `main`'s. A commit with a branch's run and a tag's, or one day more than one
  workflow (there is one now, `ci.yml`), is reported by its newest run alone.
* **W3. The watch's own status discarded.** `gh run watch --exit-status` exits non-zero "if run fails" (its help), but the tool ignores
  its exit status (`:57`); and a watch longer than its `timeout` raises `subprocess.TimeoutExpired` (a probe: `subprocess.run(["sleep",
  "2"], timeout=0.2)` raised it), which the tool does not catch, so it would end with a traceback rather than a verdict.
* **W4. The same choice in `publication.py`.** Its check "CI on origin main's head" asks `gh run list --commit <origin main> --limit 1`
  (`tools/publication.py:74`), which, at a release, when `main`'s head is the tagged commit, returns the tag's run: for `4fa7204` it
  returns `39716056732`, `headBranch` `v1.20.0`. So the check read the tag's run twice, as "CI on the tag" (`:78`) does, and never
  `main`'s. The scratch directory's check it replaced asked the same, so
  [0030-PLAN-record-initialize-on-arrival.md](0030-PLAN-record-initialize-on-arrival.md)'s "Publication of `v1.20.0`" records "CI on
  it, and on the tag, run `39716056732`": both checks were that one run. `main`'s own run, `39714586565`, succeeded (W2's listing), so
  the publication's result stands; what the check proved was less than its name.

### Considered Options, for this amendment

* **For W1.** (a) After the watch, read the run again until every job is "completed", up to a bound, then decide by the run and every
  job. (b) Decide by the run's `conclusion`, as now, and print no job list. (c) Leave it, and record the list's timing where it matters.
* **For W2.** (a) Watch every run of the commit, each reported, all required to succeed; `--branch` narrows them to one branch's.
  (b) Require `--branch`, and watch the newest run of that branch. (c) Leave it.
* **For W3.** (a) Report the watch's exit status, and turn a timeout into a verdict, "did not finish within". (b) Leave it.
* **For W4.** (a) Ask for `main`'s run by branch, `--commit <head> --branch main`, and name its run in the check's detail. (b) Leave
  it, and rename the check to what it reads.

### Decision Outcome, for this amendment

Chosen: W1 (a), W2 (a), W3 (a), W4 (a), as proposed, because each makes the tool report what its name says, from data that has settled, with no
new dependency.

* W1 (b) drops what a record needs, which job passed or failed; (c) leaves a record able to quote a list that contradicts its verdict.
* W2 (b) makes the common case, a branch's push, need a flag, and still misses a tag's run of the same commit; (c) leaves a failing
  run unread whenever a newer run of the same commit passed.
* W3 (b) leaves a hung or failed watch reported as a traceback, or not at all.
* W4 (b) leaves `main`'s own run unchecked at every release.

#### Questions, for this amendment

5. **How long to wait for the jobs to settle** (W1 a).
   * Recommended: up to 120 seconds, reading every 5, the bound a flag. The lag seen (W1) was not measured to the second, so the bound
     is a choice; a run still unsettled at the bound fails, saying so.
   * Alternative: no bound, waiting as long as the jobs take.
6. **Which runs `watch_ci.py` waits to appear** (W2 a).
   * Recommended: the runs of the commit that exist once the first has appeared, then a further 30 seconds for others, such as a tag's
     pushed after the branch; `--branch` narrows to one.
   * Alternative: only those present when the first appears.

#### Decided with the owner, 2026-10-22, the CI amendment

| Question | Answer |
| :--- | :--- |
| W1 to W4 | As proposed: (a) for each. |
| 5. How long to wait for the jobs to settle | **The owner's variant:** up to 180 seconds, reading every 10. |
| 6. Which runs `watch_ci.py` waits to appear | As recommended: those present once the first appears, and 30 seconds more for others. |

### Consequences, for this amendment

* Good, because a watched verdict and its job list agree, each job read once it has settled.
* Good, because every run of a commit is read, and `main`'s at a release, by its branch.
* Neutral, because a watch takes up to the settle bound longer, only when GitHub lags.
* Bad, because `watch_ci.py` grows from one run to several, and its tests with it.

## More Information

* **The scope** was the owner's answer of 2026-10-21: the reusable tools, generalized; the one-off record, probe and plant scripts not kept.
* **Related records:** [0030-PLAN-record-initialize-on-arrival.md](0030-PLAN-record-initialize-on-arrival.md), whose gates, planted breaks
  and publication the tools ran; [0029-PLAN-unix-client-death-pdeathsig.md](0029-PLAN-unix-client-death-pdeathsig.md), the per-platform
  checks.
