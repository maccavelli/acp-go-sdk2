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
* **`WaitDelay`** is set to the grace only when zero.
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
| Close | `sync.Once` removed | `TestCloseIdempotent` |
| ties | the tree not ended on the agent's own exit | `TestDescendantsEnded`, the own exit |
| ties | the connection not closed after the drain | `Wait` past its bound in `TestDescendantsEnded` |
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
