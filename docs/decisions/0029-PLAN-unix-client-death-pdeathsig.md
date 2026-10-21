---
status: in-progress
date: 2026-10-21
associated-madr: "0029-MADR-unix-client-death-pdeathsig.md"
---

# Implement 0029: end an agent with its client on Linux and FreeBSD, and start it from a thread that lives as long as it

Associated MADR: [0029-MADR-unix-client-death-pdeathsig.md](0029-MADR-unix-client-death-pdeathsig.md)

## Goal

Implement the MADR's O1, as the owner decided questions 1 to 3 on 2026-10-21:

* **The locked start** (question 2): on every platform, `Start` starts the agent from the watcher goroutine, locked to its OS thread
  until `Cmd.Wait` returns, so the thread that created the agent lives as long as it.
* **`Pdeathsig`** (questions 1, 3): on Linux and FreeBSD, `prepare` sets `Cmd.SysProcAttr.Pdeathsig` to SIGKILL when the caller set
  none; a caller's is kept.
* **The documentation** states the contract by platform.

`make api` must report no change to `agentproc`'s API or the root's. The release is a minor, with
[0030-PLAN-record-initialize-on-arrival.md](0030-PLAN-record-initialize-on-arrival.md)'s change: that PLAN writes its notes and tags it.

## Facts this plan rests on

Each was read on `f193910` on 2026-10-21; step 1 re-asserts them with a script. The MADR's D1 to D14 hold the probes.

| Id | Fact | Where |
| :--- | :--- | :--- |
| F1 | `Start` calls `Cmd.Start` on the caller's goroutine | `agentproc/agentproc.go:127` |
| F2 | the watcher and the monitor start after `attach` and the connection | `agentproc/agentproc.go:136`, `:143`, `:144` |
| F3 | Unix's `prepare` keeps the caller's `SysProcAttr` but for `Setpgid`, and refuses a non-zero `Pgid` | `agentproc/tree_unix.go:21` |
| F4 | `SysProcAttr.Pdeathsig` exists for `GOOS=linux` and `freebsd` only | `go doc syscall.SysProcAttr`, both toolchains |
| F5 | Go's FreeBSD start sets it with `procctl(PROC_PDEATHSIG_CTL)` | `syscall/exec_freebsd.go:209`–`:214` |
| F6 | a goroutine locked to its thread that returns ends that thread | `go doc runtime.LockOSThread`, both toolchains |

## Scope

* **Changed:** `agentproc/agentproc.go` (the locked start); `agentproc/tree_unix.go` (`prepare` calls `setDeathSignal`).
* **New:** `agentproc/deathsig_pdeathsig.go` (`//go:build linux || freebsd`: `setDeathSignal` sets SIGKILL when zero);
  `agentproc/deathsig_none.go` (`//go:build unix && !linux && !freebsd`: nothing); tests below.
* **Docs:** `Start`'s doc; the README's "Closing and limits"; `docs/architecture.md`'s "The agent process helper".
* **The records:** this PLAN and its row.
* **Not changed:** the root package; Windows' and the other platforms' trees.

## Design

### The locked start

`Start`, after its refusals, `prepare`, the pipes and `WaitDelay`, as today:

1. starts, through `p.goroutines.Go`, a goroutine that calls `runtime.LockOSThread`, then `Cmd.Start`, and sends the error on a channel;
2. on an error, the goroutine unlocks and returns, and `Start` returns the error as today;
3. on success, the goroutine waits for `Start`'s word: `Start` runs the parent's pipe closes, `attach` and the connection as today, then
   sends "watch", or, on a failure there, "abandon";
4. on "watch", the goroutine runs `watch` on the same thread; on "abandon", it kills the agent and calls `Cmd.Wait`; either way it calls
   `runtime.UnlockOSThread` once `Cmd.Wait` has returned, and returns.

`Cmd.Wait` is still called by one goroutine alone. `abandon` no longer calls `Cmd.Wait` itself; it hands that to the locked goroutine.

### `Pdeathsig`

`prepare` on Unix, after setting `Setpgid`, calls `setDeathSignal(cmd)`: on Linux and FreeBSD it sets `Cmd.SysProcAttr.Pdeathsig` to
`syscall.SIGKILL` when it is zero; elsewhere on Unix it does nothing. `prepare` always has a `SysProcAttr` by then (F3).

## Tests

The test binary runs itself as the agent, as `agentproc`'s tests do. New:

* **`TestStartFromExitingLockedGoroutine`** (`linux || freebsd`): a goroutine locks its thread, calls `Start` for a `stubborn` agent, and
  returns while locked; the process lives on. After a second, the agent still runs (`Done` open), and `Close` then returns `ErrKilled`.
* **`TestClientDeathEndsAgent`** (`linux || freebsd`): a child mode, `owner`, starts a `stubborn` agent through `Start`, writes its process
  ID to standard error, and waits; the test sends the owner SIGKILL; the agent's process is gone within two seconds
  (`syscall.Kill(pid, 0)` answers `ESRCH`).
* **`TestCallersPdeathsigKept`** (`linux || freebsd`): a `Cmd` with `Pdeathsig` SIGTERM keeps it after `Start`.
* **`TestDefaultPdeathsig`** (`linux || freebsd`): a `Cmd` with none has SIGKILL after `Start`.

## Implementation Steps

Each step ends with its gates and an execution record. Agents stage; a commit or a push on `main` needs the owner's explicit permission in
that turn (`AGENTS.md`, "Commits"). Every new test or check is seen to fail on a planted break in a scratch copy, never in the tree, before
it is relied on.

1. **Records:** this PLAN and its row `in-progress`; the facts re-asserted by a script.
2. **The locked start,** on every platform; `TestStartFromExitingLockedGoroutine`, with `Pdeathsig` set by the test on Linux.
3. **`Pdeathsig`:** `setDeathSignal` and its two files; `TestClientDeathEndsAgent`, `TestCallersPdeathsigKept`, `TestDefaultPdeathsig`.
4. **Docs:** `Start`'s doc, the README and `docs/architecture.md`; each claim mapped to a test. (Amended 2026-10-21, after step 3:
   and `rawWire`'s reader in `jsonrpc_test.go`; see the deviation after step 3.)
5. **Verification and close:** the gates; the Linux and Windows hosts reached over SSH; after CI's six jobs pass on the pushed commits,
   this PLAN and its row `complete`. The release follows with 0030's.

## Verification

Per step: phase 1's gates on go1.27.3 (regeneration, `make pre-add-check`, `GOEXPERIMENT=nojsonv2`, `GOOS=windows go vet`,
`make check-records`, `make markdownlint`, the identifier scan); `go vet ./agentproc` for `freebsd`, `darwin`, `linux`, `windows`, `js/wasm`,
`wasip1/wasm` and `plan9`; `make api`; the floor gate at go1.26.10; the tests on the Linux host.

Planted breaks, each in a scratch copy:

| For | Break | The check that must fail |
| :--- | :--- | :--- |
| the locked start | `Cmd.Start` back on the caller's goroutine | `TestStartFromExitingLockedGoroutine` (Linux) |
| the locked start | `runtime.UnlockOSThread` before `Cmd.Wait` | `TestStartFromExitingLockedGoroutine` (Linux) (cannot fail: see the deviation during step 2) |
| `Pdeathsig` | `setDeathSignal` doing nothing on Linux | `TestClientDeathEndsAgent`, `TestDefaultPdeathsig` |
| `Pdeathsig` | a caller's `Pdeathsig` overwritten | `TestCallersPdeathsigKept` |
| platforms | `deathsig_none.go` removed | `go vet ./agentproc` for `darwin` |

Acceptance:

* every step's checks pass and its record is written;
* every claim the new docs make is mapped to a test;
* the tests pass on macOS, on the Linux and Windows hosts, and in CI's six jobs;
* `make lint` reports 0 issues, `go fix -diff` prints nothing, `go mod tidy -diff` is clean;
* `make api` reports no change.

## Rollout and Rollback

Each step is one commit, reverted by reverting it.

For a consumer:

* on Linux and FreeBSD, an agent started through `agentproc` now ends when its client dies, by SIGKILL, unless the caller set its own
  `Pdeathsig`; an agent that cleans up at its input's end loses that cleanup in that case (the MADR's D10);
* everywhere, an agent is started from a thread reserved to it, so a `Pdeathsig` the caller sets no longer ends it early.

## Execution record

### Step 1, records, 2026-10-21

The owner committed the MADR, this PLAN and 0030's pair as `2f3dd47`, at the owner's word in that turn, and approved execution. What
changed: this PLAN and its `docs/README.md` row `in-progress`.

**The facts,** re-asserted by a script outside the tree, reading `2f3dd47`'s tree and the installed toolchains: 6 of 6 hold.

| Id | What the script read |
| :--- | :--- |
| F1 | `agentproc/agentproc.go:127` is `if err := cmd.Start(); err != nil {`; `Start` has no `go` statement and the file no `LockOSThread` |
| F2 | `:136` is `t, err := attach(cmd)`; `:143` and `:144` start the watcher, then the monitor |
| F3 | `tree_unix.go:21` is `func prepare(cmd *exec.Cmd) error {`; it refuses a non-zero `Pgid`, sets `Setpgid`, and names no `Pdeathsig` |
| F4 | `go doc syscall.SysProcAttr` names `Pdeathsig` for `linux` and `freebsd`, and not for `darwin`, `netbsd`, `openbsd`, `dragonfly` or `solaris`, under go1.27.3 and go1.26.10 |
| F5 | go1.27.3's `syscall/exec_freebsd.go:209` tests `sys.Pdeathsig != 0`, and `:212` and `:214` pass `_PROC_PDEATHSIG_CTL` |
| F6 | `go doc runtime.LockOSThread`, under go1.27.3 and go1.26.10: "exits without unlocking the thread, the thread will be terminated" |

**Planted breaks,** each in its own scratch copy of the tree: `Cmd.Start` called after a `runtime.LockOSThread`; the watcher and the
monitor swapped; `prepare` setting `Pdeathsig`. Each made the script report its own fact alone missing, exit 1: 3 of 3. F4 to F6 read the
installed toolchains, which a copy of the tree cannot plant.

A first run planted the three at once; the first break added a line, which moved F2's lines too, so F2 would have failed without its
own break. The breaks were run again, one to a copy, as above.

**Gates:** `make check-records` and `make markdownlint` exit 0; the identifier scan finds nothing in the changed records.

### Deviation, 2026-10-21, during step 2: an early unlock cannot be caught

**Found.** The plant "`runtime.UnlockOSThread` before `Cmd.Wait`", an unlock added right after `Cmd.Start` succeeds, left
`TestStartFromExitingLockedGoroutine` passing on the Linux host, 5 runs of 5. Once unlocked, `run`'s thread is still held through the
agent's life by the blocking wait for the agent; it is exposed only while `run` waits for `Start`'s word, before the wait, when another
goroutine could take the thread, lock it and end it. No deterministic test reaches that window.

**Decision (owner, 2026-10-21), the recommended resolution.** Keep `defer runtime.UnlockOSThread()`, so the thread is reserved until
`Cmd.Wait` has returned, with a comment in `run` saying why it stays; record the plant as one that cannot be caught. The verification
table's row is annotated. No test changes.

### Step 2, the locked start, 2026-10-21

The owner committed step 1 as `e068f97` and approved this step. What changed:

* **`agentproc/agentproc.go`:** `Start` builds the `Process` before the agent starts and hands the start to `run`, through
  `p.goroutines.Go`. `run` locks its goroutine to its OS thread, calls `Cmd.Start`, reports the error on a channel, then waits for
  `Start`'s word: "watch" runs `watch` on the same thread; "abandon" kills the agent and calls `Cmd.Wait`. It unlocks once `Cmd.Wait` has
  returned. `abandon` is now a method that sends "abandon" and waits for `run`, then closes the parent's pipe ends. `Cmd.Wait` is still
  called by one goroutine alone.
* **`agentproc/leak_test.go`:** `TestMain` locks the main goroutine to the main thread, which Go never ends, so a test's goroutine that
  locks its thread and returns ends a thread of its own.
* **`agentproc/pdeathsig_test.go`, new** (`linux || freebsd`): `TestStartFromExitingLockedGoroutine`: a goroutine locks its thread, calls
  `Start` for a `stubborn` agent with `Pdeathsig` SIGKILL, and returns, which ends the thread; the agent must say it is ready, still run a
  second later, and `Close` must return `ErrKilled`.

**The claims, each to what makes it:**

| Claim | Check |
| :--- | :--- |
| an agent with `Pdeathsig` lives while its client does, whatever thread called `Start` | `TestStartFromExitingLockedGoroutine`, the Linux host, 3 runs of 3 |
| every other behaviour is unchanged | `agentproc`'s tests: macOS, `-race`, 5 runs; the Linux host, 3 runs; the Windows host |

**Planted breaks,** each in its own scratch copy, the test binary cross-compiled and run on the Linux host. 1 of 2 failed as required;
the second is the deviation above:

| Break | The check that failed |
| :--- | :--- |
| `Cmd.Start` on the caller's goroutine again, `run` only waiting | `TestStartFromExitingLockedGoroutine`, 3 runs of 3: "the agent ended, signal: killed, when the thread that called Start did, before it said it was ready" |
| `runtime.UnlockOSThread` right after `Cmd.Start` | none, 5 runs of 5: the deviation above |

The first break's first run failed for its readiness wait, "the agent did not say it was ready within 30s": the agent died before it
could say so. The test now watches for the agent's end while it waits, and names it.

**Gates,** each exit 0, run alone:

* go1.27.3: regeneration unchanged; `make pre-add-check`, "166 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck)", and
  the new file's, "1 file(s) clean"; `GOEXPERIMENT=nojsonv2 go test ./...`; `GOOS=windows go vet ./...` in both modules;
  `make check-records`, `make markdownlint` and the identifier scan; `make lint`, "0 issues." four times, and `golangci-lint` for
  `GOOS=linux` and `GOOS=freebsd` on `agentproc`, "0 issues." each, since `make lint` builds neither; `go fix -diff ./...` prints nothing;
* `go vet ./agentproc` for `darwin`, `linux`, `freebsd`, `windows` (amd64, arm64), `openbsd`, `netbsd`, `illumos`, `js/wasm`,
  `wasip1/wasm`, `plan9`;
* `make api` against `v1.19.0`: "0 incompatible, 0 compatible";
* the Windows host: `go test -count=1 -shuffle=on ./...` in both modules;
* the Linux host: the `agentproc` test binary, 3 runs, "PASS".

**The floor gate at go1.26.10:** `go vet` and `go test -race -count=1` pass in both modules, the leak guards on.

### Step 3, `Pdeathsig`, 2026-10-21

The owner committed step 2 and approved this step. What changed:

* **`agentproc/deathsig_pdeathsig.go`, new** (`linux || freebsd`): `setDeathSignal` sets `Pdeathsig` to SIGKILL when it is zero.
* **`agentproc/deathsig_none.go`, new** (`unix && !linux && !freebsd`): `setDeathSignal` does nothing.
* **`agentproc/tree_unix.go`:** `prepare` calls `setDeathSignal` after `Setpgid`.
* **Tests:** `pdeathsig_test.go` gains `TestDefaultPdeathsig`, `TestCallersPdeathsigKept` and `TestClientDeathEndsAgent`;
  `child_unix_test.go` gains the mode `owner`, a client that starts a `stubborn` agent through `Start`, writes its process ID, and waits
  to be killed.

**The claims, each to what makes it:**

| Claim | Check |
| :--- | :--- |
| a stubborn agent ends when its client is killed without `Close` | `TestClientDeathEndsAgent`, the Linux host, 3 runs of 3 |
| `Start` sets SIGKILL when the caller set none | `TestDefaultPdeathsig` |
| a caller's `Pdeathsig` is kept | `TestCallersPdeathsigKept` |
| the other Unix systems build without it | `go vet ./agentproc` for `darwin`, `openbsd`, `netbsd`, `illumos` |

**Planted breaks,** each in its own scratch copy; the Linux ones run on the Linux host. 4 of 4 checks failed as required:

| Break | The check that failed |
| :--- | :--- |
| `setDeathSignal` doing nothing | `TestClientDeathEndsAgent`: "the agent, pid …, still runs 2s after its client was killed"; `TestDefaultPdeathsig`: "Pdeathsig = signal 0, want SIGKILL" |
| a caller's `Pdeathsig` overwritten | `TestCallersPdeathsigKept`: "Pdeathsig = killed, want the caller's SIGTERM" |
| `deathsig_none.go` removed | `GOOS=darwin go vet ./agentproc`: "undefined: setDeathSignal" |

The test's own cleanup killed the planted survivor; no test process was left on the host after any run.

**Found, outside this PLAN:** the first run of the floor gate on this step's tree failed in the root package, not `agentproc`: its
tests passed and its leak guard then reported one goroutine, `rawWire`'s line reader blocked on `lines <- sc.Text()`
(`jsonrpc_test.go:66`). `rawWire`'s channel holds 16 lines and its cleanup closes the pipes, which does not unblock a send to a full
channel, so a test that leaves more than 16 of the connection's lines unread leaks it. `jsonrpc_test.go` last changed in `711a1d5`, before
this work, and this PLAN changes nothing in the root package. Five runs of the root tests on go1.26.10 with the leak guard, and the floor
gate run again, found no leak. It waits for the owner's decision on where it is fixed.

**Gates,** each exit 0, run alone:

* go1.27.3: regeneration unchanged; `make pre-add-check`, "167 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck)", and
  the new files', "2 file(s) clean"; `GOEXPERIMENT=nojsonv2 go test ./...`; `GOOS=windows go vet ./...` in both modules;
  `make check-records`, `make markdownlint` and the identifier scan; `make lint`, "0 issues." four times; `golangci-lint` for
  `GOOS=linux` and `GOOS=freebsd` on `agentproc`, "0 issues." each; `go fix -diff ./...` prints nothing;
* `go vet ./agentproc` for `darwin`, `linux` (amd64, arm64), `freebsd`, `openbsd`, `netbsd`, `illumos`, `windows`, `js/wasm`,
  `wasip1/wasm`, `plan9`;
* `make api` against `v1.19.0`: "0 incompatible, 0 compatible";
* the Windows host: `go test -count=1 -shuffle=on ./...` in both modules;
* the Linux host: the `agentproc` test binary, 3 runs, "PASS"; macOS: `go test -count=5 -race ./agentproc`.

**The floor gate at go1.26.10,** run again after the leak above: `go vet` and `go test -race -count=1` pass in both modules, the leak
guards on.

### Deviation, 2026-10-21, after step 3: `rawWire`'s reader leaks

**Found.** As step 3 records, the floor gate's first run on that tree failed in the root package's leak guard: `rawWire`'s line reader,
`jsonrpc_test.go:66`, blocked on `lines <- sc.Text()`, its 16-line channel full; the pipes its cleanup closes do not unblock a send. One
run in seven failed so. It is test code of the root package, unchanged since `711a1d5`, and not this PLAN's subject.

**Decision (owner, 2026-10-21).** Fix it inside this PLAN, in step 4, as this deviation; the alternatives were an amendment to
[0023-PLAN-connection-lifetime-and-test-leaks.md](0023-PLAN-connection-lifetime-and-test-leaks.md), whose subject test leaks are, and
recording it only. Added to step 4's scope: `jsonrpc_test.go`, whose reader stops when the test ends (a send that also selects on a
channel the cleanup closes), and a planted break that shows the reader blocked without it. The MADR's decision is unchanged.
