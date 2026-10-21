---
status: in-progress
date: 2026-10-20
associated-madr: "0027-MADR-agent-process-helper.md"
---

# Implement 0027: start an agent process, and stop it with its descendants in bounded steps

Associated MADR: [0027-MADR-agent-process-helper.md](0027-MADR-agent-process-helper.md)

## Goal

Implement the MADR's O1, as the owner decided questions 1 to 10 on 2026-10-20:

* **A new package, `agentproc`** (question 1), whose `Command{Cmd, Grace, ConnectionOptions}` has a `Start(client)` method (question
  2) returning a `*Process` with `Conn`, `Done`, `Wait` and `Close`, and the sentinels `ErrTerminated` and `ErrKilled` (question 7).
* **The agent's tree:** a process group on Unix; a kill-on-close job object through `golang.org/x/sys/windows` on Windows, the module's
  first requirement (question 3).
* **Close's sequence:** stdin closed; up to the grace, 5 seconds by default (question 4); on Unix SIGTERM to the group (question 5) and
  the grace again; then a kill of the group or the job; then the connection closed.
* **The ties** (question 6): the agent's exit ends what is left of its tree and then its connection; the connection's end stops the
  agent.
* **The caller's `Cmd.Stderr`** (question 8), **no context** on `Start` (question 10), and **the examples** using the helper (question 9).

`make api` must report only additions: `agentproc`'s whole API, and no change to the root package. The release is proposed as the minor
`v1.19.0`, which the owner confirms at the tag.

## Facts this plan rests on

Each was read on `d034801` on 2026-10-20; step 1 re-asserts them with a script. The MADR's A1 to A6, C1 to C3, S1 to S8 and P1 to P18
hold the probes.

| Id | Fact | Where |
| :--- | :--- | :--- |
| F1 | `go.mod` has no `require` | `go.mod` |
| F2 | the API gate compares the root package only, and fails when a package cannot be loaded | `scripts/check_api.py`: its docstring; `run` exits 2 on a failed `apidiff -w` |
| F3 | the three `stopAgent` copies are identical | `example/client/main.go:316`, `example/claude-code/main.go:365`, `example/gemini/main.go:385` |
| F4 | a failed write is handed to its sender, and the connection stays up | `writeLoop`, `connection.go:705`; the MADR's A6 |
| F5 | `golang.org/x/sys` v0.49.0 declares `go 1.26.0` and has the job-object calls | the module cache; the MADR's S8 |
| F6 | `os.Process.WithHandle` exists under go1.26.9 | `go doc`; the MADR's S5 |
| F7 | `os/exec` cannot start a child suspended on Windows | `syscall/exec_windows.go:446`; the MADR's S7 |
| F8 | the root package vets for 14 targets, `js/wasm`, `wasip1/wasm` and `plan9` among them (corrected 2026-10-20, step 1: 15) | the MADR's A4 |
| F9 | a test may re-execute its own binary as a child | `nil_panic_test.go:78` |
| F10 | the leak guard is the root package's `TestMain` only | `leak_test.go` |
| F11 | `.github` holds `workflows/` only: no `dependabot` configuration | `ls .github` |

## Scope

* **New:** `agentproc/agentproc.go` (the API, `Start`, `Close`, `Wait`, the watcher), `agentproc/tree_unix.go`,
  `agentproc/tree_windows.go`, `agentproc/tree_other.go`, `agentproc/agentproc_test.go`, `agentproc/child_test.go` (the test binary's
  child modes), `agentproc/leak_test.go`, `agentproc/example_test.go`.
* **Changed:** `go.mod` and `go.sum` (`golang.org/x/sys`, step 4); `scripts/check_api.py` (step 2); `example/client/main.go`,
  `example/claude-code/main.go`, `example/gemini/main.go` (step 5); `README.md`, `docs/architecture.md` (step 6).
* **The records:** this PLAN and its row; the MADR's row; the release notes `docs/reports/0027-REPORT-release-notes-v1-19-0.md` and
  their row.
* **Not changed:** the root package's API and behaviour; the generator.

## Design

### `Start`

As the MADR's "Start, in order", with these details:

* **Refusals** return an error naming the field and start nothing: a nil `Cmd`; `Cmd.Process` set; `Cmd.Stdin` or `Cmd.Stdout` set; on
  Unix, `Cmd.SysProcAttr.Pgid` non-zero.
* **The pipes** are two `os.Pipe` pairs. After `Cmd.Start`, the parent closes the child's ends, so the agent's exit, with its tree's,
  ends the output (P2, P8).
* **`WaitDelay`** is set to the grace only when zero (corrected 2026-10-21: to half the grace; see the deviation during step 4).
* **A failure after `Cmd.Start`** (the job's creation or assignment, on Windows) kills the agent with `Process.Kill`, waits for it, closes
  the pipes, and returns the error wrapped with its step.

### The watcher

One goroutine, started by `Start`, owns `Cmd.Wait`. When it returns:

1. under the lock, record the error and mark the agent exited: no signal is sent from then on;
2. end the rest of the tree: SIGKILL to the group, which `ESRCH` answers once the group is empty, or `TerminateJobObject`;
3. close `Done`;
4. wait up to the grace for the connection's own end, the end of the agent's output; then close the connection and wait for it.

### Close

`sync.Once` around the MADR's "Close, in order". It records which step ended the agent (`ErrTerminated` after the SIGTERM step,
`ErrKilled` after the kill), waits for the watcher, and returns `Wait`'s error. The connection's own end calls it on a goroutine counted
by the `Process`, so `Wait` waits for it.

### Platforms

| File | Build | Group or job | Terminate | Kill |
| :--- | :--- | :--- | :--- | :--- |
| `tree_unix.go` | `unix` | `Setpgid`, unless `Setsid` | `syscall.Kill(-pgid, SIGTERM)` | `syscall.Kill(-pgid, SIGKILL)` |
| `tree_windows.go` | `windows` | a job with `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE`, through `Process.WithHandle` | none | `TerminateJobObject`, then `CloseHandle` |
| `tree_other.go` | `!unix && !windows` | none | none | `Process.Kill` |

### The API gate

`scripts/check_api.py` compares a list of packages: the root and `agentproc`. A package missing from the base tree is compared
against an empty package of the same path written into the base's scratch copy, so its API reads as additions. A package missing from
the working tree remains a tool error, exit 2.

## Tests

The test binary is its own child: `TestMain` runs a child mode when an environment variable names one, as `mcp/cmd_test.go` does and
as `nil_panic_test.go` re-executes its binary (F9). The modes are the MADR's: `eof`, `term`, `stubborn`, `parent`, `parent-wait`,
`heartbeat`, `burst`, and `agent`, a minimal ACP agent over `acp.NewConnection` that answers `initialize` and writes a notification
just before it exits.

* **`TestStartRefuses`:** each refusal, with nothing started.
* **`TestNoLostOutput`:** the `agent` child's last notification, written just before it exits, reaches the client (P1, P2).
* **`TestCloseLadder`:** with a 300 ms grace:
  * `eof` exits on its input's end, and `Close` returns nil within the first step;
  * `term`, on Unix, exits on SIGTERM, and `Close`'s error wraps `ErrTerminated` and reports exit status 3;
  * `stubborn` is killed, and `Close`'s error wraps `ErrKilled`, on Windows too, where the job's kill reports exit 0 (P17);
  * each within its bound: about one, two, or two grace periods and the kill.
* **`TestDescendantsEnded`:** for `parent` and `parent-wait`, the `heartbeat` grandchild's output ends after `Close`, and after the
  agent's own exit (P8, P9, P14, P17).
* **`TestStderrHeldByDescendant`:** a non-file `Cmd.Stderr` a grandchild holds does not keep `Wait` past `WaitDelay` (P10, P11).
* **`TestConnectionEndStopsAgent`:** a connection ended by an overflow stops the agent.
* **`TestCloseIdempotent`:** concurrent and repeated `Close` calls return the same error.
* **`TestOtherPlatformsBuild`,** not a test but a gate: `GOOS=js GOARCH=wasm`, `GOOS=wasip1 GOARCH=wasm` and `GOOS=plan9` vet the
  package.
* **The leak guard:** `agentproc`'s `TestMain` runs the root's goroutine-leak check after its tests (F10), so no watcher outlives a
  test.

## Implementation Steps

Each step ends with its gates and an execution record. Agents stage; a commit or a push on `main` needs the owner's explicit permission
in that turn (`AGENTS.md`, "Commits"). Every new test or check is seen to fail on a planted break in a scratch copy, never in the tree,
before it is relied on.

1. **Records:** this PLAN and its row `in-progress`; the facts above re-asserted by a script.
2. **The core, on Unix and the other platforms:** `agentproc.go`, `tree_unix.go`, `tree_other.go`; `Start`, `Close`, `Wait`, `Done`,
   the sentinels; the child modes, `TestStartRefuses`, `TestNoLostOutput`, `TestCloseLadder`, `TestStderrHeldByDescendant`,
   `TestCloseIdempotent` and the leak guard; `scripts/check_api.py` comparing `agentproc`.
3. **The ties:** the watcher's tree end and connection drain; the connection's end stopping the agent; `TestDescendantsEnded` and
   `TestConnectionEndStopsAgent`.
4. **Windows:** `tree_windows.go`; `golang.org/x/sys` required in `go.mod` and `go.sum` at the newest release that declares a `go` line at
   or under `1.26.9`, `v0.49.0` or later; the tests on the Windows host reached over SSH and in CI's Windows jobs.
5. **The examples:** the three `stopAgent` copies replaced by `agentproc` (F3); `example_pair_test.go` still passes.
6. **Docs:** the README's "Closing and limits" gains a paragraph and an example; `docs/architecture.md` describes the package; each claim
   mapped to a test.
7. **Release notes:** `docs/reports/0027-REPORT-release-notes-v1-19-0.md` and its row, with `make api`'s report.
8. **Verification and close:** the gates over the tree; the Windows and Linux hosts reached over SSH and `make push-guard` before the push;
   after CI's six jobs pass, this PLAN and its row `complete`; the owner tags `v1.19.0` and publishes its release.

## Verification

Per code step:

* phase 1's gates on go1.27.2: regeneration, `make pre-add-check`, `GOEXPERIMENT=nojsonv2`, `GOOS=windows go vet`,
  `make check-records`, `make markdownlint`, the identifier scan;
* `go vet ./agentproc` for `js/wasm`, `wasip1/wasm` and `plan9`;
* `make api` against `v1.18.0`: the root package unchanged; `agentproc` all additions;
* the go1.26.9 gate;
* from step 2, the tests on the Linux host reached over SSH; from step 4, on the Windows host too.

Planted breaks, at least. Each must be seen to fail on the check named.

| For | Break, in a scratch copy | The check that must fail |
| :--- | :--- | :--- |
| Start | the agent's stdout through `StdoutPipe` | `TestNoLostOutput` |
| Start | a refusal removed | `TestStartRefuses` |
| Start | `Setpgid` not set | `TestDescendantsEnded` (Unix) |
| Start | `WaitDelay` not set | `TestStderrHeldByDescendant` |
| Close | the kill first, without closing stdin | `TestCloseLadder`, `eof` |
| Close | the SIGTERM step removed | `TestCloseLadder`, `term` |
| Close | `ErrKilled` not joined | `TestCloseLadder`, `stubborn` (on Windows, the exit 0 case) |
| Close | `sync.Once` removed | `TestCloseIdempotent` (cannot fail: see the deviation during step 2) |
| ties | the tree not ended on the agent's own exit | `TestDescendantsEnded`, the own exit |
| ties | the connection not closed after the drain | `Wait` past its bound in `TestDescendantsEnded` (step 3: in `TestEscapedDescendantDoesNotHoldWait`, Unix; in `TestDescendantsEnded` the tree's end ends the output first) |
| ties | the connection's end not stopping the agent | `TestConnectionEndStopsAgent` |
| Windows | no job object | `TestDescendantsEnded` on Windows |
| platforms | `tree_other.go` removed | `go vet ./agentproc` for `js/wasm` |
| gate | the empty package for a package missing from the base removed | `make api`, exit 2 |
| leaks | the watcher not waited for | `agentproc`'s leak guard |

Acceptance:

* every step's checks pass and its record is written;
* every claim the new docs make is mapped to a test that makes it;
* the tests pass on macOS, on the Linux and Windows hosts, and in CI's six jobs;
* `make lint` reports 0 issues, `go fix -diff` prints nothing, and `go mod tidy -diff` is clean;
* `govulncheck` reports nothing for `golang.org/x/sys`;
* `make api` reports the root package unchanged and `agentproc`'s API as additions.

## Rollout and Rollback

Each step is one commit, reverted by reverting it; step 4 reverts `go.mod` and `go.sum` with `tree_windows.go`.

For a consumer:

* a new package, and no change to the root package;
* `golang.org/x/sys` appears in its module graph; it builds only for a consumer that imports `agentproc` on Windows;
* both known consumers already require `golang.org/x/sys v0.49.0` (the MADR's C3).

The release is proposed as the minor `v1.19.0`, which the owner confirms at the tag.

**Points for the owner's review**, each with the plan's choice:

* **`golang.org/x/sys` is updated by hand** (F11): nothing in the repository proposes its updates.
* **The child modes live in a `_test.go` file,** so they ship in no binary.
* **The grace in tests is 300 ms,** to keep the suite fast; the default stays 5 seconds.
* **The Linux host joins the Windows host** in the pre-push checks from step 2.

## Execution record

### Step 1, records, 2026-10-20

The owner approved the MADR's answers and this PLAN, and committed both as `1ea8475`. What changed:

* this PLAN and its `docs/README.md` row `in-progress`;
* a correction to the MADR's A4 and to F8 above: 15 targets, not 14.

**The facts,** re-asserted by a script outside the tree, reading `1ea8475`'s tree: 11 of 11 hold.

| Id | What the script read |
| :--- | :--- |
| F1 | no line of `go.mod` starts with `require` |
| F2 | the docstring's "The root package is the module's only API"; `run` calls `fail`, which exits 2; both `apidiff -w` calls load `module`, the root package, in the base and in the tree |
| F3 | each `stopAgent` found at `:316`, `:365` and `:385`, the three bodies one SHA-256 |
| F4 | `connection.go:705` is `func (c *Connection) writeLoop() {`; its body holds `req.done <- err` and no `closeWith`, `cancel(` or `shutdownReceive` |
| F5 | `golang.org/x/sys@v0.49.0/go.mod` has `go 1.26.0`; its `windows` package declares the four calls, the limit structure and the flag |
| F6 | under go1.26.9, `go doc os.Process.WithHandle` exits 0 and prints `func (p *Process) WithHandle(f func(handle uintptr)) error` |
| F7 | under go1.26.9, `syscall/exec_windows.go:446` is `defer CloseHandle(Handle(pi.Thread))` |
| F8 | `go vet .` exits 0 for `darwin`, `linux`, `windows`, `freebsd`, `openbsd`, `netbsd`, `dragonfly`, `solaris`, `illumos`, `aix`, `ios`, `android`, `js/wasm`, `wasip1/wasm` and `plan9`: 15 |
| F9 | `nil_panic_test.go:78` runs `exec.CommandContext` on `os.Args[0]` |
| F10 | the only test file with both `func TestMain` and `goroutineleak` is `leak_test.go` |
| F11 | `.github` holds `workflows` alone |

**Planted breaks,** six at once in a scratch copy of the tree: a `require` in `go.mod`; one `stopAgent` changed; `writeLoop` calling
`closeWith` on a failed write; a root file using `syscall.Kill`; a second test file with `TestMain` and `goroutineleak`; a
`.github/dependabot.yml`. The script reported exactly F1, F3, F4, F8, F10 and F11 missing, "5 of 11 facts hold", exit 1. F8's break
failed for `windows` and `plan9` alone: `js` and `wasip1` declare `syscall.Kill`.

**Found, and corrected:** the MADR's A4 counted 14 targets for a list of 15; the first probe had vetted 14, without `darwin`, the host.
The script vetted all 15. A4 and F8 carry the correction.

**Gates:** `make check-records` and `make markdownlint` exit 0; the identifier scan finds nothing in the changed records.

### Deviation, 2026-10-20, during step 2: `Close`'s `sync.Once` cannot be caught

**Found.** The plant "`sync.Once` removed", `p.closeOnce.Do(p.stop)` made `p.stop()`, left `TestCloseIdempotent` passing, exit 0, over
3 runs. Every step of `stop` is idempotent by itself: a second close of the agent's input is harmless; each signal is skipped once the
agent has exited, under the lock that records the exit; the connection's `Close` is idempotent. A second sequence, concurrent or
later, ends in the same state and the same error, so no test can tell the `Once`'s absence.

**Decision (owner, 2026-10-20), the recommended resolution.** Keep `closeOnce`, so concurrent callers share one sequence of graces and
send no signal twice, with a comment saying why it stays; record the plant as one that cannot be caught. The verification table's row is
annotated. No test changes.

### Step 2, the core, 2026-10-20

The owner committed step 1 as `68130ca` and approved this step. What changed:

* **`agentproc/agentproc.go`, new:** `Command{Cmd, Grace, ConnectionOptions}` and its `Start`; `Process` with `Conn`, `Done`, `Wait` and
  `Close`; `ErrTerminated`, `ErrKilled`. `Start` refuses a nil `Cmd` or client, a started `Cmd`, and a set `Stdin` or `Stdout`, starting
  nothing; it owns both pipes (`os.Pipe`), sets a zero `WaitDelay` to the grace, and starts the watcher that alone calls `Cmd.Wait`.
  `Close` runs the MADR's sequence once.
* **`agentproc/tree_unix.go`, new:** `Setpgid` unless `Setsid`; a non-zero `Pgid` refused; SIGTERM and SIGKILL to the group, an emptied
  group (`ESRCH`) not an error, `Process.Kill` when the group cannot be signalled.
* **`agentproc/tree_other.go`, new,** `//go:build !unix` for this step: no group, no terminating signal (`errors.ErrUnsupported`),
  `Process.Kill`. Windows builds and runs with it until step 4 adds `tree_windows.go` and narrows this file to `!unix && !windows`, as
  Scope says.
* **Tests, new:** `leak_test.go` (`TestMain`: a child mode when `AGENTPROC_TEST_CHILD` names one; otherwise the tests, then the
  goroutine-leak guard); `child_test.go` (the modes `eof`, `term`, `stubborn`, `burst`, `parent-stderr`, `heartbeat-stderr`; each writes
  "ready" to stderr once set up); `agentproc_test.go` (`TestStartRefuses`, `TestNoLostOutput`, `TestCloseLadder`,
  `TestStderrHeldByDescendant`, `TestCloseIdempotent`); `agentproc_unix_test.go` (`TestStartRefusesSharedGroup`); `example_test.go`
  (`ExampleCommand`, compiled, not run).
* **`scripts/check_api.py`:** `PACKAGES`, the root and `agentproc`; a package missing from the base is compared against an empty package
  written into the base's scratch copy; one missing from the working tree exits 2. Root changes print as before; `agentproc`'s carry its
  name.

**Found while writing the tests,** each settled in the tests, not the package:

* the client refuses a `session/update` before `initialize` has completed on its side ("initialize has not completed"): an agent that
  writes updates right after its `initialize` answer can have the first few refused. A real agent sends updates only for a session the
  client creates after `initialize`, so this is not a loss; the `burst` agent waits for one more message from the client first;
* the client's notification queue holds 1024, the default; the burst is 1000 updates;
* a child built with `-race` waits the race runtime's `atexit_sleep_ms`, 1000 ms by default, before it exits: measured, 1005 to 1013 ms
  against 1 ms without `-race`, and 1 ms with `GORACE=atexit_sleep_ms=0`, which the test children now get;
* a child's start, under the load of the gates running alongside, took over 3 s; the wait for "ready" is a setup bound of its own, 30 s.

**The claims, each to what makes it:**

| Claim | Check |
| :--- | :--- |
| `Start` refuses each bad `Command`, starting nothing | `TestStartRefuses`; `TestStartRefusesSharedGroup` |
| no message written before the agent exits is lost | `TestNoLostOutput`: 1000 of 1000 updates |
| an agent that exits at its input's end is not signalled, and `Close` returns nil | `TestCloseLadder`, "exits at its input's end" |
| one that exits on SIGTERM is terminated between one and two graces, `ErrTerminated` with exit status 3 | `TestCloseLadder`, "exits on SIGTERM" |
| one that ignores both is killed, `ErrKilled`, after two graces where there is a group, one elsewhere | `TestCloseLadder`, "ignores both" |
| a `Stderr` writer a descendant holds does not hold `Close` past `WaitDelay` | `TestStderrHeldByDescendant` |
| concurrent and repeated calls return the same error | `TestCloseIdempotent` |
| the package builds where the root does | `go vet ./agentproc` for the targets below |
| the API gate covers `agentproc` | `make api` |

**Planted breaks.** Each ran in its own scratch copy of the working tree. 10 of 11 failed as required; the eleventh is the deviation
above:

| Break | The check that failed |
| :--- | :--- |
| the agent's stdout through `StdoutPipe` | `TestNoLostOutput`: "the client received … of the 1000 session updates the agent wrote" |
| the `Stdout` refusal removed | `TestStartRefuses`, "Stdout set" |
| the `Pgid` refusal removed | `TestStartRefusesSharedGroup`: "want a refusal naming Pgid" |
| `WaitDelay` not set | `TestStderrHeldByDescendant`: "want exec.ErrWaitDelay, and no kill" |
| the kill first, without waiting after closing the input | `TestCloseLadder`: "want nil within the grace" |
| the SIGTERM step removed | `TestCloseLadder`: "want ErrTerminated with exit status 3" |
| `ErrKilled` not joined | `TestCloseLadder`: "want ErrKilled after" |
| `sync.Once` removed | none: the deviation above |
| the watcher never returning | the leak guard: "goroutines leaked by the tests" |
| `tree_other.go` removed | `GOOS=js GOARCH=wasm go vet ./agentproc`: "undefined: prepare" |
| the API gate's empty package removed | `scripts/check_api.py`: exit 2, "check_api: go run … failed" |

A first run of the set, alongside the gates, failed the `ErrKilled` plant for another reason: its children did not say "ready" within
3 s. With the setup bound of its own, the whole set was run again alone, on the final tree, with the results above.

**Gates** on the working tree, go1.27.2 (corrected 2026-10-21: go1.27.3; see the correction after step 5), each exit 0, run alone:

* regeneration: the generated files unchanged by a second run;
* `make pre-add-check`: "152 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck)", and the new files' check, "8 file(s)
  clean";
* `GOEXPERIMENT=nojsonv2 go test ./...`, and `GOOS=windows go vet ./...` in both modules;
* `make check-records`, `make markdownlint` and the identifier scan;
* `make lint`: "0 issues." four times; `go fix -diff ./...` prints nothing;
* `go vet ./agentproc` for `darwin`, `linux`, `windows` (amd64 and arm64), `freebsd`, `openbsd`, `netbsd`, `illumos`, `ios`, `android`,
  `js/wasm`, `wasip1/wasm` and `plan9`; `aix` and `solaris` too, before a test used `syscall.Getpgid`, which they lack, and the owner then
  set `aix` aside;
* `make api` against `v1.18.0`: "0 incompatible, 4 compatible": `agentproc: Command`, `ErrKilled`, `ErrTerminated` and `Process`, each
  "added"; the root package unchanged;
* the Windows host reached over SSH: `go test -count=1 -shuffle=on ./...` in both modules, `agentproc` among them;
* the Linux host reached over SSH: the cross-compiled `agentproc` test binary, `-test.count=3`, "PASS", the leak guard included.

**The go1.26.9 gate:** `go version` reports go1.26.9; `go vet` and `go test -race -count=1` pass in both modules, the root's tests and
`agentproc`'s with `GOEXPERIMENT=goroutineleakprofile`, the leak guards on.

### Step 3, the ties, 2026-10-21

The owner committed step 2 as `e33eec2` and approved this step. What changed:

* **`agentproc/agentproc.go`:**
  * the watcher, when `Cmd.Wait` returns, kills what is left of the agent's group (`endTree`) under the lock that marks the agent exited,
    closes `Done`, gives the connection up to the grace to reach the end of the agent's output, then closes it;
  * a monitor stops the agent, through `closeOnce`, when the connection ends while the agent runs;
  * `Wait` waits for both goroutines (`goroutines`, a `sync.WaitGroup` started with `Go`);
  * `sharedClose`, the agent's input as both the connection and `Close` hold it: closed once, the same result to every caller;
  * the docs of `Start` and `Done` describe the ties.
* **`agentproc/tree_unix.go`:** `hasTree`; `endTree`, SIGKILL to the group; `kill` now also kills the agent directly, after the group.
* **`agentproc/tree_other.go`:** `hasTree` false; `endTree` does nothing.
* **Tests:** `ties_test.go` (`TestDescendantsEnded`, `TestConnectionEndStopsAgent`); `agentproc_unix_test.go` gains
  `TestEscapedDescendantDoesNotHoldWait`; `child_test.go` gains the modes `beat` (appends to a file every 50 ms), `parent`,
  `parent-wait`, `parent-exit` and `flood`, and `answerInitialize`, which `burst` now uses too; `child_unix_test.go` has
  `parent-escape` (a beat in a session of its own, outside the group); `child_other_test.go` has none.

**Found, and fixed in this step:**

* **Two closes of the agent's input raced.** When the caller closed the connection, the connection's `Close` and the monitor's stop closed
  the same file at once, and the second failed: `Conn().Close()` returned "close |1: file already closed". `sharedClose` closes it once.
  `TestConnectionEndStopsAgent`, "the caller closes the connection", fails on the race, as the plant below shows.
* **A group that cannot be signalled left the agent running.** With no group, `kill` had treated `ESRCH` as done and killed nothing, and
  `Close` waited for an agent nobody had killed. `prepare` always makes the group, so it was not reachable, but `kill` now kills the agent
  directly too. The MADR's kill step carries the amendment.

**On Windows,** `TestDescendantsEnded` is skipped for this step: `tree_other.go` has no tree to end until step 4.

**The claims, each to what makes it:**

| Claim | Check |
| :--- | :--- |
| a descendant ends after `Close`, when the agent exits at its input's end | `TestDescendantsEnded`, "the agent exits at its input's end" |
| a descendant ends after `Close`, when the agent is terminated | `TestDescendantsEnded`, "the agent is terminated" |
| a descendant ends when the agent exits of itself, and the connection is closed | `TestDescendantsEnded`, "the agent exits of itself" |
| a descendant outside the group that holds the output does not hold `Wait` past the grace | `TestEscapedDescendantDoesNotHoldWait` |
| the connection's end stops the agent: closed by the caller, or by an overflow | `TestConnectionEndStopsAgent` |
| closing the connection and stopping the agent at once raises no error | `TestConnectionEndStopsAgent`, "the caller closes the connection" |

**Planted breaks.** Each ran in its own scratch copy of the working tree. 5 of 5 failed as required:

| Break | The check that failed |
| :--- | :--- |
| `Setpgid` not set | `TestDescendantsEnded`: "still runs after" |
| the tree not ended on the agent's own exit | `TestDescendantsEnded`, "the agent exits of itself": "still runs after the agent exited" |
| the connection not closed after the drain | `TestEscapedDescendantDoesNotHoldWait`: "Wait took longer than" |
| the monitor removed | `TestConnectionEndStopsAgent` |
| `sharedClose` bypassed, the connection given the bare file | `TestConnectionEndStopsAgent`, "the caller closes the connection", 20 runs: "file already closed" |

The plants that leave a descendant alive left four `beat` grandchildren running, as they should; each was ended after the run.

**Gates** on the working tree, go1.27.2 (corrected 2026-10-21: go1.27.3; see the correction after step 5), each exit 0, run alone:

* regeneration: the generated files unchanged by a second run;
* `make pre-add-check`: "160 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck)", and the new files' check, "3 file(s)
  clean";
* `GOEXPERIMENT=nojsonv2 go test ./...`, and `GOOS=windows go vet ./...` in both modules;
* `make check-records`, `make markdownlint` and the identifier scan;
* `make lint`: "0 issues." four times; `go fix -diff ./...` prints nothing;
* `go vet ./agentproc` for the 13 targets of step 2;
* `make api` against `v1.18.0`: "0 incompatible, 4 compatible", unchanged by this step;
* the Windows host reached over SSH, both modules, `agentproc` among them;
* the Linux host reached over SSH: the `agentproc` test binary, `-test.count=3`, "PASS";
* `go test -count=5` of `agentproc`, with and without `-race`, on macOS.

**The go1.26.9 gate:** `go version` reports go1.26.9; `go vet` and `go test -race -count=1` pass in both modules, `agentproc` with the
leak guard.

### Deviation, 2026-10-21, during step 4: `WaitDelay` is half the grace

**Found.** With the job object in place, `TestStderrHeldByDescendant` failed on the Windows host, 3 runs of 3: "Close = agentproc: agent
killed, want exec.ErrWaitDelay, and no kill". The agent exits at its input's end, but a grandchild still holds the caller's `Cmd.Stderr`,
a writer that is not a file, so `Cmd.Wait` returns, and the agent is seen to exit, only when `WaitDelay` closes that pipe. With
`WaitDelay` equal to the grace, `Close`'s first wait ran out at the same time, the kill ended the job, and a clean exit was reported as a
kill. Unix had passed only because its SIGTERM step, which the grandchild ignores, came first; Windows had passed in step 2 because its
kill could not reach the grandchild.

**Decision (owner, 2026-10-21), the recommended resolution.** `Start` sets a zero `WaitDelay` to half the grace, so an agent that exits
within half the grace is seen to exit within `Close`'s first step, with no signal on any platform; `Close` then reports
`exec.ErrWaitDelay`. The MADR's "Start, in order", step 4, and its specification carry the amendment; Design above is annotated.
`TestStderrHeldByDescendant` now also refuses `ErrTerminated`. Added to this step: `agentproc/agentproc.go`'s one line and doc, the
test's comment and assertion.

### Step 4, Windows, 2026-10-21

The owner committed step 3 as `529bdac` and approved this step, and committed this step's code, the MADR's amendment and this PLAN's
annotation as `449b2b6` while its gates ran; this record follows it. What changed:

* **`agentproc/tree_windows.go`, new:** `attach` creates a job object with `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE` and assigns the agent
  through `os.Process.WithHandle`; `kill` is `TerminateJobObject(job, 1)`, with `Process.Kill` as the fallback; `end` terminates what is
  left of the job and closes its handle; `terminate` is `errors.ErrUnsupported`; `hasGroups` false, `hasTree` true.
* **The platforms' state:** each `tree_*.go` now has a `tree` type made by `attach` after `Cmd.Start`, with `terminate`, `kill` and `end`;
  `agentproc.go` holds it, and `abandon` kills, waits for and closes an agent `Start` started but cannot connect.
* **`agentproc/tree_other.go`** narrowed to `!unix && !windows`, as Scope says.
* **`go.mod` and `go.sum`:** `require golang.org/x/sys v0.49.0`, the newest release, published 2026-10-06, whose `go` line is `1.26.0`
  (`go list -m -json golang.org/x/sys@latest`); `go mod tidy -diff` prints nothing.
* **Tests:** `agentproc_windows_test.go`, `TestClientDeathEndsTree`: an `owner` child starts an agent through the helper and exits
  without `Close`; the agent's grandchild stops. `child_windows_test.go` has the `owner` mode; `child_other_test.go` narrowed to
  `!unix && !windows`. On Windows, `TestDescendantsEnded` now runs.

**Where `golang.org/x/sys` is linked:** `go list -deps` names `golang.org/x/sys/windows` for `agentproc` under `GOOS=windows` alone; for
`agentproc` under `linux` and `js/wasm`, and for the root package, it names no `golang.org/x/sys` package.

**The claims, each to what makes it:**

| Claim | Check |
| :--- | :--- |
| on Windows, the agent's descendants end with it, after `Close` and on its own exit | `TestDescendantsEnded`, on the Windows host |
| on Windows, the agent's descendants end when the process that started them dies | `TestClientDeathEndsTree` |
| a Windows kill reads as a kill | `TestCloseLadder`, "ignores both", on the Windows host |
| a held `Stderr` writer makes no signal and no kill | `TestStderrHeldByDescendant`, on the three platforms |
| `golang.org/x/sys` is built for Windows only | `go list -deps`, above |

**Planted breaks.** Each ran in its own scratch copy of the working tree; a Windows one's test binary was cross-compiled from that copy
and run on the Windows host. 7 of 7 checks failed as required:

| Break | Where | The check that failed |
| :--- | :--- | :--- |
| no job object | Windows | `TestDescendantsEnded`: "still runs after"; `TestClientDeathEndsTree`: "still runs after the process that started it died" |
| the job without `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE` | Windows | `TestClientDeathEndsTree`: "still runs after the process that started it died" |
| `WaitDelay` not set | Windows | `TestStderrHeldByDescendant`: "want exec.ErrWaitDelay, and no signal" |
| `WaitDelay` not set | macOS | `TestStderrHeldByDescendant`: "want exec.ErrWaitDelay, and no signal" |
| `WaitDelay` the whole grace, as before the deviation | Windows | `TestStderrHeldByDescendant`: "want exec.ErrWaitDelay, and no signal" |
| `ErrKilled` not joined | Windows | `TestCloseLadder`: "want ErrKilled" |

The plants that leave descendants alive left up to three test processes on the Windows host each; the runner ended them after each run.

**Gates,** each exit 0, run alone:

* on `449b2b6`'s tree, go1.27.2 (corrected 2026-10-21: go1.27.3; see the correction after step 5): regeneration unchanged; `make pre-add-check`, "166 file(s) clean (gofmt, golangci-lint, go vet, go test,
  govulncheck)"; `GOEXPERIMENT=nojsonv2 go test ./...`; `GOOS=windows go vet ./...` in both modules; `make check-records`,
  `make markdownlint` and the identifier scan; `make lint`, "0 issues." four times; `go fix -diff` and `go mod tidy -diff` print nothing;
* `go vet ./agentproc` for 14 targets: `darwin`, `linux`, `windows` (amd64, arm64 and 386), `freebsd`, `openbsd`, `netbsd`, `illumos`,
  `ios`, `android`, `js/wasm`, `wasip1/wasm`, `plan9`;
* `make api` against `v1.18.0`: "0 incompatible, 4 compatible", unchanged by this step;
* the Windows host: `go test -count=1 -shuffle=on ./...` in both modules; the cross-compiled `agentproc` test binary, 5 runs, every test
  "PASS"; and, built with go1.26.9 and `GOEXPERIMENT=goroutineleakprofile`, 3 runs, "PASS";
* the Linux host: the `agentproc` test binary, 3 runs, "PASS";
* macOS: `go test -count=5 -race ./agentproc`.

**The go1.26.9 gate:** `go vet` and `go test -race -count=1` pass in both modules, `agentproc` with the leak guard; `GOOS=windows go vet
./agentproc` under go1.26.9 exits 0.

### Step 5, the examples, 2026-10-21

The owner committed step 4's record as `5930996` and approved this step. What changed, in `example/client/main.go`,
`example/claude-code/main.go` and `example/gemini/main.go`, by one asserted script:

* `exec.CommandContext(ctx, …)` became `exec.Command(…)`, and `context.WithCancel` with its `defer cancel()` became
  `context.Background()`: the helper, not a context, ends the agent;
* the `StdinPipe`, `StdoutPipe` and `Cmd.Start` block, and `NewClientSideConnection` with `SetLogger`, became
  `(&agentproc.Command{Cmd: cmd, ConnectionOptions: []acp.ConnectionOption{acp.WithLogger(slog.Default())}}).Start(client)` and
  `agent.Conn()`; the logger is an option because the connection reads from the moment `Start` returns; each example keeps its own start
  error message;
* each `stopAgent(cmd, stdin, cancel)` became `stopAgent(agent)`, four calls in each file;
* `stopAgent` itself, the 10 identical lines of F3, became 5 that call `agent.Close()` and print its error;
* the unused `time` import went, and `example/client`'s stale "Set up pipes for stdio" comment.

`git diff --stat`: 3 files, 55 lines added, 100 removed.

**The claims, each to what makes it:**

| Claim | Check |
| :--- | :--- |
| the example client and agent still work together | `TestExamplePair`, "allowed" and "input ended", under `-race` |
| the client, run as a user runs it, starts its agent through `go run`, a launcher, and stops it cleanly | `go run ./example/client`: exit 0 in 5.6 s, "Turn ended", no "agent exited" line, no example agent process left |
| the three examples build and their tests pass | `go vet ./example/...`; `go test -race ./example/...` |

**Planted breaks:** none; this step adds no test or check. It moves the examples onto checks steps 2 to 4 made.

**Gates** on the working tree, go1.27.2 (corrected 2026-10-21: go1.27.3; see the correction after step 5), each exit 0, run alone:

* regeneration unchanged; `make pre-add-check`, "166 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck)";
* `GOEXPERIMENT=nojsonv2 go test ./...`; `GOOS=windows go vet ./...` in both modules;
* `make check-records`, `make markdownlint` and the identifier scan;
* `make lint`, "0 issues." four times; `go fix -diff ./...` prints nothing;
* `make api` against `v1.18.0`: "0 incompatible, 4 compatible", unchanged;
* the Windows host: `go test -count=1 -shuffle=on ./...` in both modules, the root package and the three examples among them.

**The go1.26.9 gate:** `go vet` and `go test -race -count=1` pass in both modules, the root's tests and `agentproc`'s with the leak
guard.

### Correction, 2026-10-21: the toolchain steps 2 to 5 ran on

Steps 2 to 5 record their gates as run on go1.27.2. They ran on go1.27.3. This machine's default toolchain changed on 2026-10-20 between
19:44 and 22:31: `go env GOTOOLCHAIN` is `go1.27.3`, set in Go's environment file, and every binary built in the session's scratch
directories from 22:31 on names go1.27.3 (`go version <file>`), as those built up to 19:44 name go1.27.2. Step 2's first build, at 22:31,
is the earliest go1.27.3 one. The Windows and Linux hosts reached over SSH run go1.27.3 too. The `toolchain` line still names go1.27.2,
which a `GOTOOLCHAIN` set to one version overrides.

Nothing else in those records changes: their gates and their results stand, on go1.27.3. Step 4's Windows run built with go1.26.9 and
`GOEXPERIMENT=goroutineleakprofile`, and the go1.26.9 gates, are as recorded. The steps' lines are annotated, not rewritten.
[0028-MADR-pin-go-1-27-3-and-floor-1-26-10.md](0028-MADR-pin-go-1-27-3-and-floor-1-26-10.md) pins the repository to go1.27.3.
