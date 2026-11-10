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
    after; (Amended 2026-11-10, proposed: and `test`, `go test` with its arguments on a copy of the tree, the removal checked; see
    "Amendment 2026-11-10: `go test` on a host with its own flags, and plants under `-race` and in another module".)
  * `leftovers.py`: the Windows host's process list, test binaries matched by their exact image name;
  * `plants.py`: planted breaks from a JSON file the step writes, outside the repository or under `.cache/`, local or on a host;
    (Amended 2026-11-10, proposed: a plant may ask for `-race` and name the module it runs in; see the same amendment.)
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

## Amendment 2026-10-22, after step 11: CI's job history, read by attempt

Found while the third crash of [0032-REPORT-windows-floor-runtime-crashes.md](../reports/0032-REPORT-windows-floor-runtime-crashes.md) was recorded, and assessed at the owner's request the same
day: the owner asked for the scratch scripts that filled its tallies to be made a tool. The decisions above are unchanged; this
adds a tool, and its outcome is proposed. Every figure below was taken that day with gh 2.102.0, the repository only read, at
`ef050db`.

### What was found, for this amendment

* **H1. The report's procedure rests on two scratch scripts.** Its "If it happens again" asks for a tally of every job's first
  attempt, by system and Go version, and the failed step's log. The tally behind its addendum's table was a scratch script of
  36 lines, the repository's path and its GitHub name written in; the environment check of
  [0033-PLAN-traceback-on-windows-floor-ci.md](0033-PLAN-traceback-on-windows-floor-ci.md)'s step 3 was another, 30 lines, its one
  job and value written in. Neither is in the repository, and a session's scratch directory goes with the session (T1).
* **H2. `gh run view --job` reads the latest attempt.** Run `39841121410` crashed in attempt 1, in job `120042086316`, and passed
  when that job was rerun, attempt 2. `gh run view 39841121410 --job 120042086316 --log`, with attempt 1's job id, returned 49,279
  bytes stamped from 19:46:01Z, attempt 2's, with the root package "ok" and no crash; with `--attempt 1` added, 5,150,437 bytes
  from 19:33:25Z, holding "fatal error: found pointer to free object". `--log-failed` did the same: 1,850 bytes without the crash,
  and 5,107,398 with `--attempt 1`. So the report's step, `gh run view <run> --job <job> --log-failed`, shows the passing rerun
  once the failed job has been rerun.
* **H3. The REST endpoint reads by job id, so by attempt.** `gh api repos/<owner>/<name>/actions/jobs/<id>/logs` gave, for
  attempt 1's id, 3,268,240 bytes with the crash, and for attempt 2's, `120048503562`, 25,051 bytes without it. gh 2.102.0
  refuses it unless `--allow-escape-sequences` is passed ("the response contains terminal escape sequences; pass
  --allow-escape-sequences to output it anyway"). In that run the Windows floor job, not rerun, is listed in attempt 2 under a
  new id, `120048506640`, with attempt 1's start, 19:33:24Z, and attempt 1's log, which holds `GOTRACEBACK: crash`.
* **H4. The report misquotes the runtime.** The third crash's quotation, `0032-REPORT-windows-floor-runtime-crashes.md:222`, ends
  "try -d=checkptr)"; the log's line, read both ways, ends "try -d=checkptr or -race)". The commit that holds it, `a76e929`, is
  on `origin/main`.
* **H5. 0033's step 3 named the run, not the attempt.** Its environment check read each job's log through `gh run view --job
  --log` (H2), on run `39841121410`, which has two attempts; the record does not say which was read. Both run the same workflow
  at the same commit, so their values should agree: step 12 checks it rather than assume it.
* **H6. A tally's window can be read again.** `gh run list --branch main --limit 40 --created '<2026-10-22T20:00:00Z'` gives 40
  runs, the newest `39841121410`, the oldest `39175773059`: the window of the report's second tally, so a tool's tally can be
  compared with the recorded one.

### Considered Options, for this amendment

* **E1.** One tool, `tools/ci_jobs.py`: `tally`, `failure` and `env`, every log read by its job id.
* **E2.** `tools/ci_tally.py`, the tally alone, as first proposed; the failed step and the environment read by hand.
* **E3.** Two tools, a tally and an environment check, as the scratch scripts were.
* **E4.** No tool: the report's step corrected to `--attempt N`, the scripts left out.

### Decision Outcome, for this amendment

Proposed: "E1", because the report's procedure becomes three commands of one tool that reads each job's log by its id, so the
attempt is never implicit (H2, H3), and each command can be checked against what the records already hold (H6). (Decided
2026-10-22: E1, by the owner's answers to questions 7 and 8, both of which build on it; see "Decided with the owner,
2026-10-22, the job-history amendment".)

* E2 leaves the step that H2 breaks a hand-run command.
* E3 splits two readers of the same API by the same id, and leaves the failed step by hand.
* E4 keeps the procedure on scripts a session deletes (H1).

#### Questions, for this amendment

7. **The tool's name.**
   * Recommended: `ci_jobs.py`, since it tallies, reads failures and checks the environment, all of CI's jobs.
   * Alternative: `ci_tally.py`, the name first proposed, with the same three subcommands.
8. **The report's misquotation (H4) and its procedure (H2).**
   * Recommended: the quotation corrected in place to the log's line, with a dated note under it naming the correction; the
     procedure's `gh run view` item struck through and annotated, `ci_jobs.py`'s commands beside it.
   * Alternative: the report left as written, and a third addendum carrying both corrections.

#### Decided with the owner, 2026-10-22, the job-history amendment

| Question | Answer |
| :--- | :--- |
| 7. The tool's name | As recommended: `ci_jobs.py`. |
| 8. The report's misquotation and its procedure | As recommended: the quotation corrected in place with a dated note; the procedure's `gh run view` item struck through and annotated, `ci_jobs.py`'s commands beside it. |

### Consequences, for this amendment

* Good, because the report's tables and rows each come from one command, and a rerun cannot hide the attempt that failed.
* Good, because the records' figures for the three crashes, their seeds and traces and the second tally, can be read again.
* Neutral, because a crash's log is megabytes, 3.3 MB for the third, kept under `.cache/tools/`.
* Bad, because one more tool and its tests are code to maintain.

## Amendment 2026-11-10: `go test` on a host with its own flags, and plants under `-race` and in another module

Raised by GitHub issue #15, "tools/remote.py: run go test -race (and -run, -count) on a test host", which the owner had filed
after [0052-PLAN-agent-tool-call-snapshot-stream-and-message-size-default.md](0052-PLAN-agent-tool-call-snapshot-stream-and-message-size-default.md)'s
phase 1, and scoped by the owner on 2026-11-10 (see "Decided with the owner, 2026-11-10, the host-test amendment"). The decisions
above are unchanged; this adds a subcommand to `remote.py` and two keys to `plants.py`'s plants. Its outcome was proposed, and
accepted by the owner on 2026-11-10.
Every figure below was taken that day, the repository only read, at `8d8642f`.

### What was found, for this amendment

* **R1. No tool runs `go test -race` on a host.** `remote.py pkg` cross-compiles the test binary here with `CGO_ENABLED=0`
  (`tools/remote.py:87`–`:88`), and the race detector needs cgo, so `-race` cannot be one of its flags. `remote.py tree` runs
  `go test -count=1 -shuffle=on ./...` in both modules (`:71`–`:72`), its command fixed: no `-race`, no `-run`, no `-count`, no
  package. 0052's phase 1, step 5, asked for `go test -race -count=3 -run 'MaxMessageSize' . ./remote` on the Linux host; a
  scratch script did it in `remote.py`'s way, the tree's files copied to a fresh `acp-tools-<random>` and removed after, and
  passed, the root package in 41.2 s and `remote` in 118.0 s, 151 s in all (that PLAN's "Deviation, 2026-11-10, the tool").
* **R2. No plant runs under `-race`.** A local plant runs `go test -count=1 -timeout=<t> -run <run> <pkg>` (`tools/plants.py:89`),
  and a host plant goes through `remote.py pkg` (`:85`–`:86`), R1's cross-compiled binary. 0052's phase 3 planted a data race,
  `TestToolCallStreamConcurrent`'s, the change path without the stream's lock, which only `-race` shows; a scratch script ran it,
  "the same gap as issue #15's, in another tool" (that PLAN's "Deviation, 2026-11-10, the race plant"). It failed as required:
  four `WARNING: DATA RACE` and "race detected during execution of test".
* **R3. No plant runs in another module.** A local plant's `go test` runs in the copy's root (`tools/plants.py:89`–`:90`, `cwd=copy`),
  so a package of the generator's module, `cmd/generate`, cannot be named: from the root, `go list ./cmd/generate/internal/output`
  exits 1, "main module (github.com/maccavelli/acp-go-sdk2) does not contain package
  github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/output". 0052's phase 3 planted the generator check back on its old importer by a scratch script in the copy's `cmd/generate`,
  "since `tools/plants.py` runs from the root module"; it failed as required at `check_test.go:354`.
* **R4. The remote directory's removal is not checked, and a timeout is a traceback.** `tree` and `pkg` run `rm -rf` on the
  remote directory in a `finally` (`tools/remote.py:76`, `:109`) and ignore its status, so a directory a Windows process still holds
  is left behind unreported. `_common.run` passes `timeout` to `subprocess.run` (`tools/_common.py:70`), which raises
  `TimeoutExpired` past it, and `remote.py` does not catch it: the run ends with a traceback, after the `finally`'s removal.
* **R5. Both hosts can run `-race`.** A read-only probe: each host reports go1.27.3 for its own system on amd64, `go env CGO_ENABLED`
  `1`, `CC` `gcc`, and a `gcc` on its path, on Windows a MinGW-w64 build in the account's home. A throwaway module of two
  tests, copied to a fresh `acp-tools-<random>` on each host and removed after, its removal checked: `go test -race -count=1 -run
  '^TestClean$'` passed (Windows 2.2 s, Linux 1.7 s, wall time), and `-run '^TestRacy$'`, an unsynchronised counter, exited 1 with
  `WARNING: DATA RACE` and "race detected during execution of test" on both. So the issue's Windows question is answered: no
  refusal is needed today; a host without cgo is still possible, and should be refused with a message, not left to `go test`.
* **R6. Two other scratch scripts of 0052 are not in this scope.** Its phase 4's regenerating plants edited the emitter, ran `go
  run .` in the copy's `cmd/generate`, then a root test (that PLAN's phase 4 plants, the "regenerated in the copy" rows); and its
  phase 6 planted a break in a copy of `tools/` for `test_tools.py`. Neither is a `-race` gap or a module gap; the owner's scope
  is R1 to R4.

### Considered Options, for this amendment

* **For R1.** (a) A subcommand, `remote.py test`, that copies the tree as `tree` does and runs `go test` with the arguments given,
  each quoted for the remote shell. (b) Flags on `tree`: `--race`, `--run`, `--count`, packages. (c) `pkg --race`: copy the tree,
  build the test binary with `-race` on the host, run it with `-test.*` flags.
* **For R2 and R3.** (a) Two optional keys on a plant, `race` and `module`; a host plant that sets either runs through `remote.py
  test`. (b) `race` alone (R2), the module left to scratch scripts. (c) Neither, as the issue's own scope.
* **For R4.** (a) One helper for all three subcommands: remove the directory, check it is gone, and turn a timeout into a verdict.
  (b) The same, for `test` alone. (c) Leave it.

### Decision Outcome, for this amendment

Chosen: R1 (a), R2 and R3 (a), R4 (a), each the owner's answer of 2026-11-10; the design below was proposed, and the owner
accepted it and approved the PLAN's steps 13 and 14 the same day. Together they turn the three scratch scripts of R1 to R3 into commands of the tools a PLAN already names, with no new dependency, and make every host run report what it
left behind.

* R1 (b) needs an option for each `go test` flag a PLAN may ask for, and its packages mix with `tree`'s default of both modules;
  (c) builds in two stages and still copies the whole tree, so it costs what (a) does for a narrower result.
* R2 (b) leaves R3's script for the next generator plant; (c) leaves both.
* R4 (b) gives one tool two clean-up behaviours; (c) leaves the issue's clean-up requirement unmet.

#### The design, as proposed

* **`remote.py test --host H [--module DIR] [--tree DIR] [--timeout SECONDS] -- ARG...`.** The files of the tree, or of
  `--tree DIR`, copied to a fresh `acp-tools-<random>` as `tree` copies them; then, in `<dir>/<module>`, `go version`, `go env
  GOOS GOARCH CGO_ENABLED` and `go test ARG...`, each argument quoted for the remote shell as `pkg` quotes its flags. The arguments
  are `go test`'s own, packages among them, and at least one is required: the tool cannot tell a package from a flag's value, so it
  adds none. `--module` is a directory of the tree holding a `go.mod`, `.` by default, refused when it is absolute, leaves the
  tree, or holds none. With `--tree DIR`, the files are the git-listed ones when `DIR` has `.git`, as now; otherwise every file
  under it but macOS's `._*` and `.DS_Store`, since a plant's copy has no `.git` (`plants.py`'s `IGNORE` has already left the rest
  out). `--timeout` bounds the remote `go test`, 3600 seconds by default as now.
* **The race detector's precondition.** When an argument asks for it, `-race`, `--race`, `-race=true` or `-race=1`, the tool
  reads `go env CGO_ENABLED` on the host before copying; anything but `1` is refused, exit 2, with a message that names the host's
  kind and the value. R5's hosts pass.
* **Clean-up, for `tree`, `pkg` and `test`.** One helper, `rm -rf` and then `test ! -e` on the directory, in one SSH command,
  after a pass, a failure or a timeout. A directory left behind is named in the output, as `acp-tools-<random>` relative to the
  remote home, and fails the run.
* **A timeout** in any SSH step is caught: the step reports "did not finish within N s" and status 124, what `timeout(1)` gives;
  the clean-up still runs. A remote `go test` that outlives its SSH session may hold the directory on Windows; the clean-up's check
  then reports it, and `leftovers.py` lists the process.
* **Exit status** of each subcommand: the first failing step's status (a copy, a test run, 124 for a timeout; for `tree`, 1 when
  either module failed, as now); 2 for a refusal; otherwise 3 when the directory was left behind; 0 only when everything passed
  and the directory is gone.
* **`plants.py`.** Two optional keys. `"race": true` runs the plant's test under `-race`: locally, `go test -race`; on a host,
  through `remote.py test`. `"module": "cmd/generate"` runs it in that directory of the copy, which must hold a `go.mod`: locally,
  `go test`'s working directory; on a host, `remote.py test --module`. A host plant that sets neither still goes through `remote.py
  pkg`, which needs no cgo on either side and builds once here. A key a plant does not know is refused, as a missing one is now, so a
  misspelt `race` is not run without it; `race` must be a boolean and `module` a relative path that stays in the tree. The verdict
  line names the module and `-race` when set.
* **Docs.** `remote.py`'s and `plants.py`'s usage; `tools/README.md`'s rows and the plant example; `docs/architecture.md`'s line
  for `remote.py`.

#### Decided with the owner, 2026-11-10, the host-test amendment

| Question | Answer |
| :--- | :--- |
| 9. Where the spec lives | As recommended: this amendment, and a step 13 of the PLAN, not a new pair: the same topic, as `AGENTS.md` has it, and the precedent of steps 9 to 12. |
| 10. Scope | **The owner's choice, the widest of three:** R1, R2 and R3, `remote.py test` and plants under `-race` and in another module. The recommendation was R1 and R2; the issue alone was R1. |
| 11. The shape of R1 | As recommended: R1 (a), `remote.py test` with `go test`'s own arguments, the issue's first proposal; the Windows refusal replaced by the cgo check, after R5. |
| 12. Clean-up | As recommended: R4 (a), for `tree`, `pkg` and `test`. |

### Consequences, for this amendment

* Good, because a PLAN's `-race`, `-run` and `-count` on a host, a race plant, and a generator plant are each one command of a
  tool in the repository, and the next PLAN does not rebuild a scratch script.
* Good, because every host run says whether its directory is gone, and a timeout ends in a verdict.
* Neutral, because `tree`'s and `pkg`'s output gains a line for the removal, and the two host gates' logs with it.
* Neutral, because a race plant compiles with `-race`, a few seconds more locally and the tree's copy on a host.
* Bad, because `remote.py` and `plants.py` grow, and their tests with them; a host's `go test` still prints nothing until it ends.
* Bad, because R6's two scripts remain outside the tools.

## More Information

* **The scope** was the owner's answer of 2026-10-21: the reusable tools, generalized; the one-off record, probe and plant scripts not kept.
* **Related records:** [0030-PLAN-record-initialize-on-arrival.md](0030-PLAN-record-initialize-on-arrival.md), whose gates, planted breaks
  and publication the tools ran; [0029-PLAN-unix-client-death-pdeathsig.md](0029-PLAN-unix-client-death-pdeathsig.md), the per-platform
  checks.
