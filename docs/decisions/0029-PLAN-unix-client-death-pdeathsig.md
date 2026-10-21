---
status: proposed
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
4. **Docs:** `Start`'s doc, the README and `docs/architecture.md`; each claim mapped to a test.
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
| the locked start | `runtime.UnlockOSThread` before `Cmd.Wait` | `TestStartFromExitingLockedGoroutine` (Linux) |
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
