---
status: accepted
date: 2026-10-21
decision-makers: Repository owner
---

# End an agent with its client on Linux and FreeBSD, and start it from a thread that lives as long as it

## Context and Problem Statement

[0027-MADR-agent-process-helper.md](0027-MADR-agent-process-helper.md) left one gap in `agentproc`, as a consequence: "Neutral, because on
Unix the agent's tree outlives a client that dies without running `Close` (P18). Linux's `Pdeathsig` covers the direct child only, and is
not proposed here." On Windows the job object's kill-on-close ends the tree when the client dies (its P18).

On 2026-10-21 the owner asked for the gap to be scoped and specified, and for the most robust, idiomatic means. This record holds what
happens today when a client dies, what `Pdeathsig` adds and costs, a latent bug the probes found in `v1.19.0`, and the questions to
decide.

Every figure below comes from a command or a probe run that day. The repository was only read, at `f193910`. The probes are Go programs
outside the tree, built with go1.27.3 against the working tree's `agentproc` through a `replace`, and run on:

* macOS 26.6.2, arm64, locally;
* a Linux host reached over SSH, Ubuntu 26.04, kernel 7.0.0, amd64.

FreeBSD was not probed; what this record says of it rests on its manual and Go's source.

### How the probes kill a client

A probe "owner" process starts an agent through `agentproc.Command.Start`, waits until the agent is working, and is then sent SIGKILL: the
client dies without `Close`, as a crash or an `os.Exit` leaves it. Each agent or grandchild appends to a file every 50 ms; it counts as
running when the file grows within 400 ms. The agents:

* `eof-beat`, which exits at its input's end;
* `stubborn-beat`, which ignores its input's end and SIGTERM;
* `launcher-eof` and `launcher-stubborn`, which start a grandchild of that kind on the same standard input and output and wait for it, as
  `npx` waits for the agent it runs;
* `eof-cleanup`, which at its input's end takes 300 ms to clean up, writes a file to say so, and exits.

### What happens today

| Id | Agent | macOS | Linux |
| :--- | :--- | :--- | :--- |
| D1 | `eof-beat` | the agent gone; its beat ended | the same |
| D2 | `stubborn-beat` | **the agent alive; its beat running** | the same |
| D3 | `launcher-eof` | the launcher gone; the grandchild ended | the same |
| D4 | `launcher-stubborn` | **the launcher alive; the grandchild running** | the same |
| D5 | `eof-cleanup` | the cleanup finished | the same |

The client's death already closes the agent's standard input: the client held the only write end, `Start` having closed the agent's copy
in the parent ([0027-PLAN-agent-process-helper.md](0027-PLAN-agent-process-helper.md), step 2). So an agent that exits at its input's
end exits, with any process it started that reads the same input, and it can clean up first (D1, D3, D5). Only an agent that ignores its
input's end, and what it started, survive (D2, D4). 0027's P18 used such a tree, so its "On Unix nothing does" holds for that case only.

### What `Pdeathsig` adds, on Linux

`syscall.SysProcAttr.Pdeathsig` asks the kernel to send the child a signal when its parent goes. With `Pdeathsig` SIGKILL set on the
agent's `Cmd`:

| Id | Agent | Linux, with `Pdeathsig` SIGKILL |
| :--- | :--- | :--- |
| D6 | `eof-beat` | the agent gone; its beat ended |
| D7 | `stubborn-beat` | **the agent gone; its beat ended** |
| D8 | `launcher-eof` | the launcher gone; the grandchild ended |
| D9 | `launcher-stubborn` | the launcher gone; **the grandchild running** |
| D10 | `eof-cleanup`, SIGKILL | **the cleanup cut short** |
| D11 | `eof-cleanup`, SIGTERM | **the cleanup cut short**: the agent has no SIGTERM handler, and SIGTERM's default ends it |

So `Pdeathsig` ends the direct child, stubborn or not (D7), but not a process the child started (D9), whose signal is not inherited
(FreeBSD's manual: "The value is cleared for child processes"). And it ends the child at once, before an agent that exits at its input's
end can clean up (D10, D11), unless the agent handles the signal it is sent.

### The thread `Pdeathsig` follows, on Linux

Go's documentation of `SysProcAttr.Pdeathsig` on Linux: "Pdeathsig, if non-zero, is a signal that the kernel will send to the child
process when the creating thread dies. Note that the signal is sent on thread termination, which may happen before process termination.
There are more details at https://go.dev/issue/27505." That issue, "syscall: misleading documentation for linux SysProcAttr.Pdeathsig",
closed 2026-08-12, quotes the prctl(2) manual: "the 'parent' in this case is considered to be the thread that created this process."

The Go runtime ends a thread when a goroutine locked to it returns without unlocking (`go doc runtime.LockOSThread`, both toolchains: "If
the calling goroutine exits without unlocking the thread, the thread will be terminated"), and the thread that ran `Cmd.Start` may later
carry such a goroutine, the caller's or a library's. The probes, the main goroutine locked to the main thread so that the test's goroutine runs on another, 3 runs of
3 on Linux:

| Id | Case | Result, the process alive throughout |
| :--- | :--- | :--- |
| D12 | a child with `Pdeathsig` started from a locked goroutine that then returns | **the child killed at once**; its beat never ran |
| D13 | the same, the goroutine kept until `Wait` returns | the child running |
| D14 | `agentproc.Command.Start`, `Pdeathsig` set on the `Cmd` by the caller, called from a locked goroutine that then returns | **the agent killed at once**; its beat never ran |

**D14 is a latent bug in `v1.19.0`.** `Start` calls `Cmd.Start` on the caller's goroutine. A caller may set `Pdeathsig` today, since
`prepare` keeps the caller's `SysProcAttr` but for `Setpgid`, and its agent then dies whenever the thread that ran `Start` ends, which the
caller cannot see. Without `Pdeathsig` the thread does not matter.

### Where `Pdeathsig` exists

`go doc syscall.SysProcAttr` names `Pdeathsig` for `GOOS=linux` and `GOOS=freebsd`, and not for `darwin`, `netbsd` or `openbsd`.

* **FreeBSD:** Go's `syscall/exec_freebsd.go` sets it with `procctl(PROC_PDEATHSIG_CTL)` (`:209`–`:214` in go1.27.3). The procctl(2)
  manual: "Request the delivery of a signal when the parent of the calling process exits": the parent process, not a thread.
* **macOS and the other Unix systems** have no equivalent that a parent can set for its child; an agent there ends with its client only by
  its input's end (D1, D3).

### Prior art

* **No ACP or MCP SDK sets it.** A search of the TypeScript, Rust, Python and Kotlin ACP SDKs, `coder/acp-go-sdk` and the MCP Go SDK, at
  the revisions 0027 read, for `pdeathsig`, `PR_SET_PDEATHSIG`, `prctl`, `parent_death` and `CLONE_NEWPID` finds nothing. The Rust SDK
  kills the agent's process group when its connection's future is dropped, which a dead client cannot do.
* **`magic-cli-remote`** sets it, `internal/procutil/owner_linux.go`: `SetDeathSignal` "asks the kernel to SIGKILL cmd's process as soon
  as the calling process dies. This is the first line of defence against orphaned children when the parent is killed without running its
  shutdown path." On Darwin and elsewhere it does nothing (`owner_darwin.go`, `owner_other.go`). Its second line is an owner token,
  `<pid>:<starttime>` from `/proc`, and a sweep at the next start for children whose owner has gone. It calls `SetDeathSignal` for its
  codex, HTTP-agent and authentication CLI processes (`internal/provider/codex/provider.go:566`, `internal/provider/httpagent/provider.go:518`,
  `internal/providerauth/cli.go:100`), not for its ACP agents. Its comment says that the runtime's retiring the forking thread "silently
  disarms" the signal; D12 shows the kernel sends it then instead, ending the child early.

### Other means considered

* **A PID namespace on Linux** (`CLONE_NEWPID`, with `CLONE_NEWUSER` unprivileged): when a namespace's first process dies, the kernel ends
  every process in it, so the whole tree would end. It changes what the agent sees: its own process ID 1, `/proc`, the user it runs as.
  It depends on the host's policy for unprivileged user namespaces (on the Linux host, `kernel.unprivileged_userns_clone` is 1 and
  `kernel.apparmor_restrict_unprivileged_userns` 0; other hosts differ). Too much for a helper to impose.
* **A supervising process,** a watchdog that outlives the client and kills the tree: a library has no binary of its own to run, and
  re-executing the caller's would require its `main` to cooperate.
* **`PR_SET_CHILD_SUBREAPER`** makes the client adopt orphaned descendants while it lives; it does nothing once the client is dead.
* **A cgroup's `cgroup.kill`,** through a systemd scope: needs systemd and a delegated cgroup, which a library cannot assume.

## Decision Drivers

* **No leak:** an agent that ignores its input's end must not outlive its client (D2).
* **No interference with a well-behaved agent:** one that exits at its input's end must still be able to clean up when its client dies
  (D5), or the record must say what it costs.
* **No premature end:** an agent must never die while its client lives (D12, D14).
* **One contract across platforms where it can be had:** Windows already ends the tree with the client (0027's P18).
* **Within `v1`, additive only**
  ([0021-MADR-resolve-issues-4-and-5-before-v1-14-0.md](0021-MADR-resolve-issues-4-and-5-before-v1-14-0.md)).
* **No new dependency.**

## Considered Options

* **O1.** On Linux and FreeBSD, `Start` sets `Pdeathsig` on the agent unless the caller set one; and on every platform `Start` starts the
  agent from the watcher goroutine, locked to its thread until `Cmd.Wait` returns.
* **O2.** The locked start only: `Pdeathsig` stays the caller's to set, and is then safe.
* **O3.** Nothing in code: document that the client's death closes the agent's input, and the thread hazard.
* **O4.** A PID namespace on Linux, ending the whole tree.

## Decision Outcome

Chosen option: "O1", with the recommended answer to each of questions 1 to 3, decided by the owner on 2026-10-21 (see "Decided with
the owner, 2026-10-21"), because it closes the leak for the agent itself (D7) where the kernel can, removes the latent bug (D14) for every
caller, and keeps the input's end for everything else.

* O2 fixes D14 but leaves D2 to each caller to know about.
* O3 leaves D14 a trap.
* O4 changes the agent's environment and depends on host policy, for the part (D9) the other options leave to the input's end.

### Specification, as decided

* **The start.** `Start` starts a goroutine that calls `runtime.LockOSThread`, then `Cmd.Start`, and reports the outcome to `Start` on a
  channel; on success the same goroutine becomes the watcher and calls `Cmd.Wait` on that thread, then `runtime.UnlockOSThread`. The thread
  that created the agent lives as long as the agent, and carries no other goroutine (D13). The cost is one OS thread reserved to each
  running agent.
* **`Pdeathsig`,** on `linux` and `freebsd` (`tree_unix.go` split, or a file per system): when `Cmd.SysProcAttr.Pdeathsig` is zero,
  `prepare` sets it to SIGKILL (question 1). A caller's non-zero `Pdeathsig` is kept. It is not inherited, so a process the agent starts
  ends with the client only by its input's end (D9).
* **Elsewhere,** `darwin`, the other Unix systems, `js`, `wasip1` and `plan9`: no change; the agent ends with its client by its input's
  end, as today.
* **The documentation** of `Start` and the README's "Closing and limits" state the contract by platform: Windows ends the tree; Linux and
  FreeBSD end the agent, and the rest of the tree by its input's end; elsewhere, the input's end alone.

### Questions

Each has a recommended answer.

1. **The signal `Start` sets.**
   * Recommended: SIGKILL. It ends a stubborn agent (D7), which no one can escalate once the client is dead, and matches Windows, whose job
     ends its processes at once. It costs an agent that cleans up at its input's end that cleanup, when its client dies (D10).
   * Alternative: SIGTERM. An agent that handles it can clean up; one that does not dies as with SIGKILL (D11); one that ignores it
     survives, as today.
   * Alternative: none by default (O2); a caller sets `Pdeathsig` itself, safely.
2. **The locked start.**
   * Recommended: always, on every platform, so the rule has no condition; on systems without `Pdeathsig` it costs one reserved thread
     per running agent for nothing it needs.
   * Alternative: only when `Pdeathsig` is set, on Linux.
3. **FreeBSD.**
   * Recommended: included. Its `Pdeathsig` follows the parent process, not a thread (its manual), so it has no D12; it was not probed.
   * Alternative: Linux only, until a FreeBSD host can probe it.

#### Decided with the owner, 2026-10-21

| Question | Answer |
| :--- | :--- |
| 1. The signal `Start` sets | As recommended: SIGKILL, unless the caller set a `Pdeathsig`. |
| 2. The locked start | As recommended: always, on every platform. |
| 3. FreeBSD | As recommended: included. |

### Consequences

* Good, because on Linux and FreeBSD a stubborn agent no longer outlives its client.
* Good, because `Pdeathsig`, the caller's or the helper's, can no longer end an agent while its client lives.
* Neutral, because a process the agent starts still ends with the client only by its input's end, as on macOS (D9).
* Bad, because, by question 1's recommendation, an agent that cleans up at its input's end loses that cleanup when its client dies on
  Linux or FreeBSD (D10). A clean `Close` is unaffected.
* Bad, because behaviour changes on Linux and FreeBSD for a client that dies, which the release notes name.

### Confirmation

The PLAN will make each finding a test, seen to fail on a planted break in a scratch copy:

* on Linux: a client, the test binary in a child mode, starts a stubborn agent through `Start` and is killed; the agent ends (D7);
* on every platform: `Start` called from a locked goroutine that then returns; the agent lives until `Close` (D14), which fails with the
  locked start removed and `Pdeathsig` set;
* a caller's `Pdeathsig` kept;
* the tests on the Linux host and in CI's Linux, macOS and Windows jobs; `go vet` for `freebsd`.

## Pros and Cons of the Options

### O1. `Pdeathsig` on Linux and FreeBSD, and the locked start

* Good, because it meets every driver but the cleanup one, whose cost question 1 names.
* Bad, because the cleanup of an input's-end agent is cut short when the client dies.

### O2. The locked start only

* Good, because it changes no behaviour and removes D14.
* Bad, because a stubborn agent still outlives its client unless the caller knows to set `Pdeathsig`.

### O3. Documentation only

* Good, because it costs nothing.
* Bad, because D14 stays a trap for any caller that sets `Pdeathsig`.

### O4. A PID namespace

* Good, because it would end the whole tree.
* Bad, because it changes the agent's environment and depends on host policy.

## More Information

* **Measurements outside the tree:** the probes and their output stay outside the repository, as its records do for scratch tools.
* **Related records:** [0027-MADR-agent-process-helper.md](0027-MADR-agent-process-helper.md), whose consequence this resolves and whose
  P18 D1 to D4 refine; [0027-PLAN-agent-process-helper.md](0027-PLAN-agent-process-helper.md), where `Start` closes the agent's ends of
  its pipes.
* **Not decided here:** ending a Unix agent's grandchildren with the client (D9), beyond their input's end.
* **The PLAN** is [0029-PLAN-unix-client-death-pdeathsig.md](0029-PLAN-unix-client-death-pdeathsig.md).
