---
status: accepted
date: 2026-10-20
decision-makers: Repository owner
---

# Start an agent process, and stop it with its descendants in bounded steps

## Context and Problem Statement

[0023-MADR-connection-lifetime-and-test-leaks.md](0023-MADR-connection-lifetime-and-test-leaks.md) left a third item undecided,
"a record of its own": "An SDK helper that starts an agent process and escalates its shutdown, as the Rust and Python SDKs have. The
example client's `stopAgent` shows the pattern with `os/exec`."

On 2026-10-20 the owner asked for it to be scoped and specified: idiomatic Go, current practice for go1.27.2, and compatible with the
module's go1.26.9 floor. This record holds what the SDK, its examples and its two known consumers do today, what `os/exec` gives on
both toolchains, how child processes behave on macOS, Linux and Windows, the prior art, and the questions to decide.

Every figure below comes from a command or a probe run that day. The repository was only read, at `d034801`. The probes are Go
programs outside the tree, run on three hosts:

* macOS 26.6.2, arm64, locally;
* a Linux host reached over SSH, kernel 7.0.0, amd64;
* a Windows host reached over SSH, Windows 10.0.26200, amd64.

Each probe binary was built with go1.27.2 and the `go 1.26.9` line. Each remote run used a fresh directory that was removed after it. The
probe outcomes are reproducible: a second full run on Linux and on Windows gave the same 16 and 15 outcomes, timings aside.

### What the ACP specification asks

The transports page (agentclientprotocol.com/protocol/transports), stdio:

* "The client launches the agent as a subprocess."
* "The agent **MAY** write UTF-8 strings to its standard error (`stderr`) for logging purposes. Clients **MAY** capture, forward, or
  ignore this logging."
* Its sequence diagram ends: "Close stdin, terminate subprocess".

It sets no time between the two, and no signal.

### What the SDK and its examples do today

* **A1. Three identical copies of `stopAgent`.** `example/client/main.go:316`, `example/claude-code/main.go:365` and
  `example/gemini/main.go:385` are byte for byte the same, 10 lines each (a script compared their SHA-256). Each closes the agent's
  input, then `time.AfterFunc(5*time.Second, cancel)` cancels the context of `exec.CommandContext`, whose `Cancel` is `Kill`
  ("CommandContext sets the command's Cancel function to invoke the Kill method"), then `cmd.Wait()`.
* **A2. Each example reads the agent through `StdoutPipe`** and sends its stderr to `os.Stderr`; none sets a process group, a job object,
  `WaitDelay` or a signal. `example/claude-code` starts `npx -y @agentclientprotocol/claude-agent-acp@latest` (`:215`), a launcher
  whose real agent is a descendant.
* **A3. The module has no requirements.** `go.mod` holds the module line, `go 1.26.9`, `toolchain go1.27.2` and a `retract`; no
  `require`.
* **A4. The root package builds everywhere tried.** `go vet .` exits 0 for 14 targets: `darwin`, `linux`, `windows`, `freebsd`,
  `openbsd`, `netbsd`, `dragonfly`, `solaris`, `illumos`, `aix`, `ios`, `android`, `js/wasm`, `wasip1/wasm` and `plan9`.
  Corrected 2026-10-20, during the PLAN's step 1: 15 targets. The list always named 15; the first count left out `darwin`, the
  host, which the PLAN's fact script vetted with the others.
* **A5. The API gate covers the root package only.** `scripts/check_api.py`: "The root package is the module's only API; the examples'
  main packages are not compared."
* **A6. A failed write does not end a connection.** `writeLoop` hands a write's error to its sender (`connection.go:705`). A probe
  against the tree closed a connection's writer, an `os.Pipe` standing for the agent's stdin, underneath it:
  * a later `SendNotification` failed, `-32603` with "write |1: file already closed";
  * 300 ms later the connection was still up;
  * a message the peer then wrote was received;
  * the peer's end then ended it, and `Wait` returned "acp: peer disconnected: peer connection closed".

  So a helper can close the agent's input and keep reading what the agent still writes.

### What the consumers do

* **C1. `magic-cli-remote`** (`go 1.27.2`; requires this module at `v1.15.0`) starts its agents in one place, `spawnAgent`
  (`internal/provider/acpagent/acpagent.go:456`):
  * it reads stdout through `StdoutPipe` (`:475`), while a separate watcher goroutine owns `cmd.Wait`;
  * on Unix it sets `Setpgid` (`internal/procutil/procutil_unix.go:29`) and kills with `syscall.Kill(-pid, SIGKILL)` (`:52`);
  * on Windows it uses `golang.org/x/sys/windows` (`procutil_windows.go:15`): `CREATE_NEW_PROCESS_GROUP` (`:104`) and a job object with
    `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE`;
  * its session close sends `session/close` under `closeTimeout = 2 * time.Second` (`session.go:1254`), closes the connection (`:1359`)
    and kills the process tree at once (`:1372`): it gives the agent no time to exit after its input closes;
  * a watcher on `conn.Done()` (`acpagent.go:625`) exists because the connection can end while the agent process lives.
* **C2. The second consumer,** a CLI named to the owner (this repository's identifier scan keeps its name out), `go 1.27.2`, requires
  this module at `v1.16.3`:
  * as a client, it runs its own agent in the same process, over two `io.Pipe`s (`acpclient/conn.go`), and starts no agent process;
  * its MCP stdio servers stop in the fullest ladder found: close stdin, wait `stdinGrace = 500 * time.Millisecond`, SIGTERM to the
    group, wait `closeTimeout = 2 * time.Second`, kill (`mcpclient/proc.go:20`–`:21`); its process trees use `Setpgid`
    (`internal/proctree/tree_unix.go:15`) and a kill-on-close job object through `golang.org/x/sys/windows`
    (`internal/proctree/tree_windows.go:39`);
  * as an agent that clients start, it ends on its input's end (`acpserver/serve.go:40`–`:41`: `case <-conn.Done(): return
    agent.Close()`); on SIGTERM or SIGHUP it exits with 128 plus the signal's number (`internal/cli/exit.go:19`–`:23`); an interrupt is
    first offered to the running turn (`internal/cli/signal.go:56`–`:60`).
* **C3. Both consumers already require `golang.org/x/sys v0.49.0`** (`go.mod:20` in each).

### What `os/exec` gives, at the floor and at 1.27.2

`go doc` was read under go1.26.9 and go1.27.2, for `darwin`, `linux` and `windows`.

* **S1. `Cmd.Cancel` and `Cmd.WaitDelay`.** `WaitDelay` "bounds the time spent waiting on two sources of unexpected delay in Wait: a
  child process that fails to exit after the associated Context is canceled, and a child process that exits but leaves its I/O pipes
  unclosed." When it elapses, a child that has not exited "will be terminated using os.Process.Kill", then still-open pipes are
  closed. "If WaitDelay is zero (the default), I/O pipes will be read until EOF, which might not occur until orphaned subprocesses of
  the command have also closed their descriptors for the pipes."
* **S2. `StdoutPipe`:** "Wait will close the pipe after seeing the command exit, so it is incorrect to call Wait before all reads from
  the pipe have completed." `StdinPipe`: "The pipe will be closed automatically after Cmd.Wait sees the command exit."
* **S3. `Process.Signal`:** "Sending Interrupt on Windows is not implemented." `os.Interrupt`: on Windows it "will return an error
  instead of sending a signal".
* **S4. The two toolchains behave alike.** The docs differ in three places, all go1.27.2 additions:
  * `Cmd.Wait` "must not be called concurrently from multiple goroutines. A custom Cmd.Cancel function should not call Wait.";
  * `Cmd.Stdin` and `Cmd.Stdout`: "Regardless of WaitDelay, Wait can block until a Read from Stdin completes" (and a Write to Stdout
    or Stderr);
  * a comment on `os.Process.Pid`.

  The code of `os/exec` differs only in `Cmd.String` (`c.Args[1:]` became `c.argv()[1:]`), by a `diff` of the two `GOROOT`s with
  comments removed. Neither release note names an `os/exec` change. go1.26's names two the helper can use: "The new
  Process.WithHandle method provides access to an internal process handle on supported platforms (pidfd on Linux 5.4 or later,
  Handle on Windows)", and `signal.NotifyContext` now cancels with a cause.
* **S5. `Process.WithHandle` exists at the floor:** `go doc os.Process.WithHandle` under go1.26.9 prints it. "The handle is guaranteed
  to refer to process p until f returns, even if p terminates."
* **S6. Process groups on every `unix` target.** A file with `//go:build unix` using `syscall.SysProcAttr{Setpgid: true, Pgid: 0}` and
  `syscall.Kill(-pid, syscall.SIGTERM)` passes `go vet` for `darwin`, `linux` (`amd64`, `arm64`), `freebsd`, `openbsd`, `netbsd`,
  `dragonfly`, `solaris`, `illumos`, `aix`, `ios` and `android`; the same file with an unknown field fails for `aix`, so the check
  compiles it.
* **S7. Windows has no process group to signal, and `os/exec` cannot start a child suspended.** Windows' `SysProcAttr` has
  `CreationFlags` and no job field, the same on both toolchains. `syscall.StartProcess` closes the new main thread's handle at once
  (`syscall/exec_windows.go:446`, `defer CloseHandle(Handle(pi.Thread))`), so a caller cannot resume a suspended child. A job object
  is assigned after the child has started. (Amended 2026-10-21: the thread can be opened again by its ID and resumed, and the agent is now started
  suspended and assigned before it runs; see [0030-MADR-record-initialize-on-arrival.md](0030-MADR-record-initialize-on-arrival.md),
  "Amendment 2026-10-21, Windows: a process the agent starts before its job assignment".)
* **S8. `golang.org/x/sys/windows` v0.49.0** declares `go 1.26.0`, under the floor. It has `CreateJobObject`, `SetInformationJobObject`,
  `AssignProcessToJobObject`, `TerminateJobObject`, `JOBOBJECT_EXTENDED_LIMIT_INFORMATION` and `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE`
  (`zsyscall_windows.go`, `types_windows.go`).

### How a child process behaves: the probes

A child is the probe binary itself in one mode: `burst` writes then exits; `eof` exits at its input's end; `term` ignores that end
and exits 3 on SIGTERM or an interrupt; `stubborn` ignores its input's end and SIGTERM; `parent` starts a `heartbeat` grandchild on
the same stdout and exits at its input's end; `parent-wait` starts one and waits for it; `heartbeat` writes a line every 50 ms. A
grandchild counts as alive when lines still arrive within 400 ms.

| Id | Case | macOS | Linux | Windows |
| :--- | :--- | :--- | :--- | :--- |
| P1 | 1 KiB written, child exits; `StdoutPipe`; `Wait`, then read | read 0 of 1024, "file already closed" | the same | the same |
| P2 | the same through an `os.Pipe` the caller owns | 1024 of 1024 | the same | the same |
| P3 | 32 KiB through an `os.Pipe`; `Wait` before reading | `Wait` exit 0, then 32768 read | the same | **`Wait` still running after 1 s**: the child blocks on a full pipe |
| P4 | `Close` of an `os.Pipe` read end during a `Read` | `Read` returns at once, "file already closed" | the same | the same |
| P5 | `eof`, stdin closed | exit 0 | the same | the same |
| P6 | `stubborn`, stdin closed, then `Kill` | running after 300 ms; then "signal: killed" | the same | running; then "exit status 1" |
| P7 | `term`, `Signal(os.Interrupt)` | exit status 3 | the same | **"not supported by windows"**, still running |
| P8 | `parent` exits at its input's end | the grandchild still writing | the same | the same |
| P9 | `parent-wait` killed | the grandchild still writing | the same | the same |
| P10 | P9 with a non-file `Stdout` writer | **`Wait` still running after 1 s** | the same | the same |
| P11 | P10 with `WaitDelay` 300 ms | `Wait` returns after about 500 ms | the same | the same |
| P12 | `term`, stdin closed, then SIGTERM | running after 300 ms; then exit status 3 | the same | not run: S3 |
| P13 | `stubborn`, SIGTERM, then `Kill` | running 500 ms after SIGTERM; killed | the same | not run |
| P14 | `parent-wait` with `Setpgid`; `Kill(-pgid, SIGTERM)` | parent "signal: terminated"; **the grandchild's output ended** | the same | not run |
| P15 | `CommandContext`, `Cancel` sending SIGTERM, `WaitDelay` 500 ms, `stubborn` | killed after about 700 ms | the same | with `Cancel` nil: "exit status 1" after about 700 ms |
| P16 | `taskkill /T /F` on `parent-wait` | not run | not run | **the grandchild's output ended** |
| P17 | `parent-wait` in a job object with kill-on-close; `CloseHandle(job)` | not run | not run | **the grandchild's output ended**; the parent reports **exit 0** |
| P18 | an owner starts `parent-wait` in a group (Unix) or a kill-on-close job (Windows), then exits without cleaning up | **the tree still writing** | the same | **the tree's output ended** |

Each run also showed that a `Setpgid` child's process group is its own process ID, not the parent's. By POSIX's definition of the
foreground process group, a terminal's interrupt does not reach it; that was not probed with a terminal.

What the probes establish:

* **P1, S2: reading through `StdoutPipe` while another goroutine waits loses the agent's last output.** P2: a pipe the helper owns
  does not, because `Wait` does not close it. C1's shape, a reader on `StdoutPipe` and a separate goroutine in `Wait`, matches P1.
* **P3: on Windows a pipe buffers less than 32 KiB,** so waiting for the agent before draining its output can deadlock. The output must
  be read while the agent runs, which the connection does.
* **P4: closing the read end unblocks a read in progress on all three,** so closing the connection always ends its reader.
* **P8, P9: a launcher's descendants survive it** and keep the output open, so neither the agent's exit nor `Kill` ends the
  connection. A2's `npx` is such a launcher.
* **P14 on Unix and P16, P17 on Windows end a whole tree.** Only the job object needs no external program.
* **P17: on Windows a job's kill reports exit 0,** so the exit status cannot say the agent was killed.
* **P10, P11: `Wait` blocks on a non-file writer a descendant holds, unless `WaitDelay` is set.** The caller's `Stderr` may be such a
  writer.
* **P18: if the client itself dies, a kill-on-close job ends the agent's tree on Windows. On Unix nothing does.**

### Prior art

The sources are the clones read for 0023, at the revisions named; each claim was read in the code.

| Library | Helper | Stop sequence | Descendants | Windows | Stderr | Close's result |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| MCP Go SDK (`e844b98`) | `mcp.CommandTransport{Command *exec.Cmd; TerminateDuration}`, `Connect` (`mcp/cmd.go:20`–`:51`) | stdin; wait; SIGTERM; wait; `Kill`; wait; each `TerminateDuration`, default 5 s (`:16`); then "unresponsive subprocess" (`:111`) | not handled | SIGTERM fails, skipped (`:99`) | the caller's `Cmd` | `Wait`'s error, once (`closeOnce`) |
| ACP Python SDK (`11f8a4c`) | `spawn_agent_process`, an async context manager (`src/acp/stdio.py:162`) | stdin; wait; `terminate()`; wait; `kill()`; each `shutdown_timeout = 2.0` (`src/acp/transports.py:55`) | not handled | `terminate()` is `kill()` | piped, never drained | `process.returncode` |
| ACP Rust SDK (`652d100`) | `AcpAgent`, `connect_to` (`acp_agent.rs`) | stdin's end; 1 s (`SHUTDOWN_GRACE_PERIOD`, `:24`); SIGKILL to the group | `process_group(0)` (`:272`); the group is killed also when the child exits | `CREATE_NO_WINDOW` (`:279`); the direct child only | drained; last 64 KiB in the error (`:21`) | `Err` with the status and stderr's tail |
| ACP TypeScript SDK (`60619a4`) | none; the example spawns the agent (`src/examples/client.ts:106`) | `agentProcess.kill()` (`:171`), with no wait | no | `npx.cmd` | inherited | none |
| ACP Kotlin SDK (`dc1a8f4`) | a sample, `createProcessStdioTransport` | none | no | no | piped, never read | none |
| `coder/acp-go-sdk` (`v0.13.5`) | none; examples `Kill` and never `Wait` | `Kill` | no | no | `os.Stderr` | none |

`golang.org/x/tools`' gopls starts a detached daemon with `Setsid` and never signals it; it is not a stdio child.

## Decision Drivers

* **The specification's sequence:** close stdin, then terminate.
* **No lost output.** The agent's last messages are read before anything is closed (P1, P2, A6).
* **The whole agent, not its launcher.** `npx`, `uvx` and `go run` start the real agent as a descendant (P8, P9, A2).
* **Bounded.** Every wait has a bound, and a stuck descendant cannot hold `Close` or `Wait` (P10, P11).
* **Every platform the root builds for** still builds (A4); Windows and Unix each get what they support (S3, S7).
* **Go idiom and the standard library:** an `*exec.Cmd` the caller configures, as `mcp.CommandTransport` takes; one goroutine owns
  `Wait` (S4); `Close` idempotent and safe for concurrent use.
* **The floor and the toolchain:** nothing beyond go1.26.9, and nothing that behaves differently under go1.27.2 (S4, S5).
* **Within `v1`, additive only**
  ([0021-MADR-resolve-issues-4-and-5-before-v1-14-0.md](0021-MADR-resolve-issues-4-and-5-before-v1-14-0.md)).
* **Dependencies only by a record that names them** (`AGENTS.md`).

## Considered Options

* **O1.** A helper in a new package, `agentproc`: it starts the agent with pipes it owns, in a process group (Unix) or a kill-on-close
  job object (Windows); stops it by closing stdin, then SIGTERM to the group (Unix), then a kill of the group or job, each step
  bounded; and ties the process and the connection together.
* **O2.** O1's helper in the root package, `acp`.
* **O3.** A minimal helper, as `mcp.CommandTransport`: the stop ladder on the direct child only, with no group or job.
* **O4.** No helper: fix the examples' `stopAgent`, and document the pattern.

## Decision Outcome

Chosen option: "O1", with the answers to questions 1 to 10 the owner decided on 2026-10-20 (see "Decided with the owner,
2026-10-20"): the recommended answer to each, but for question 2, where the owner chose a struct with fields. Only a helper that owns
the pipes and the agent's tree meets every driver, and a package of its own keeps platform code and a Windows dependency out of the
protocol package.

* O2 puts `syscall` and, by question 3's recommendation, `golang.org/x/sys/windows` into the package every agent and client imports, and
  build-tagged files into a package that today has none for process control (A4).
* O3 leaves P8 and P9: the launcher dies and the real agent keeps running, holding the output open; it also inherits P1 when it uses
  `StdoutPipe`, as `mcp.CommandTransport` does (`:33`–`:51`).
* O4 leaves each application to rediscover P1, P3, P8–P11 and P17; the three copies (A1) and C1 show the cost.

### Specification, as decided

**The package.** `github.com/maccavelli/acp-go-sdk2/agentproc`, three files beside the common one: `tree_unix.go` (`//go:build unix`),
`tree_windows.go`, and `tree_other.go` (`//go:build !unix && !windows`) for `js`, `wasip1` and `plan9`, where there is no tree to end.

```go
// Package agentproc starts an ACP agent as a child process, connects to it
// over the child's standard input and output, and stops it, with the
// processes it started, in bounded steps
// (0027-MADR-agent-process-helper.md).
package agentproc

// Command is an agent to start as a child process.
type Command struct {
	// Cmd is the agent's command. It must not have been started, and its
	// Stdin and Stdout must be nil: Start connects them to pipes it owns.
	// Its Path, Args, Env, Dir and Stderr are the caller's; a nil Stderr
	// discards the agent's log, as os/exec does.
	Cmd *exec.Cmd

	// Grace is how long each step of Close waits for the agent to exit.
	// Zero or less means 5 seconds.
	Grace time.Duration

	// ConnectionOptions configure the connection to the agent.
	ConnectionOptions []acp.ConnectionOption
}

// Start starts the agent and connects client to it over the agent's
// standard input and output. On Unix the agent runs in a process group of
// its own, and on Windows in a job object, so that stopping it stops the
// processes it started. When Cmd.WaitDelay is zero, Start sets it to the
// grace. If a step after the agent has started fails, Start kills the agent
// and its group or job, waits for it, and returns the error.
func (c *Command) Start(client acp.Client) (*Process, error)

// Process is a running agent and the connection to it. Its methods are safe
// for concurrent use.
type Process struct{ /* unexported */ }

// Conn returns the connection to the agent.
func (p *Process) Conn() *acp.ClientSideConnection

// Done returns a channel that is closed once the agent process has exited.
func (p *Process) Done() <-chan struct{}

// Wait blocks until the agent process has exited and the connection's
// goroutines have returned, and then returns the agent's error, the same
// each time: nil for an exit of status 0 of its own, otherwise the error
// exec.Cmd.Wait returned, joined with ErrTerminated or ErrKilled when Close
// ended the agent so. It does not stop the agent.
func (p *Process) Wait() error

// Close stops the agent and returns Wait's error. It closes the agent's
// standard input and waits up to the grace for the agent to exit. On Unix it
// then sends SIGTERM to the agent's process group and waits up to the grace
// again. It then kills the group, or on Windows ends the job, and closes the
// connection. A second call returns the same error.
func (p *Process) Close() error

// ErrTerminated is joined to Wait's error when the agent exited after Close
// sent SIGTERM to its process group.
var ErrTerminated = errors.New("agentproc: agent terminated")

// ErrKilled is joined to Wait's error when Close killed the agent's process
// group or ended its job.
var ErrKilled = errors.New("agentproc: agent killed")
```

(Amended 2026-10-21, during the PLAN's step 4: `Start` sets a zero `WaitDelay` to half the grace, not the grace, and the code's doc
comment says so; the PLAN's deviation during step 4 holds why.)

**Start, in order.**

1. Refuse, starting nothing, a nil `Cmd`, or one whose `Process` is set, whose `Stdin` or `Stdout` is set, or, on Unix, whose `SysProcAttr` sets
   a non-zero `Pgid`, which would put the agent in a group other processes share.
2. Create two `os.Pipe`s. The agent's stdout is one the helper owns, so `Wait` never closes the connection's reader (P1, P2).
3. On Unix, set `SysProcAttr.Setpgid` unless `Setsid` is set, which makes the agent its own group's leader too. The caller's
   `SysProcAttr` is kept otherwise.
4. Set `WaitDelay` to the grace when it is zero (P10, P11) (amended 2026-10-21, during the PLAN's step 4: half the grace; see the PLAN's deviation during step 4), then `cmd.Start()`. Close the agent's ends of both pipes in the parent.
5. On Windows, create a job object with `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE` and assign the agent to it through `Process.WithHandle`
   (S5), which needs no process ID and so meets no reused one. A descendant the agent starts before the assignment is outside the job
   (S7); in every probe run (P17, P18) the assignment came first.
6. Start one goroutine that owns `cmd.Wait` (S4) and, when it returns, runs "the agent's own end" below.
7. Connect: `acp.NewClientSideConnection(client, stdinWriter, stdoutReader, c.ConnectionOptions...)`.

**The agent's own end.** When `cmd.Wait` returns, the helper records its error and marks the agent exited, so it signals no process
from then on. It kills what is left of the group, or ends the job (P8: a launcher's descendants outlive it), then closes `Done`. It then
gives the connection up to the grace to read to the end of the agent's output (P2), and closes it.

**The connection's own end.** When the connection ends first, by an overflow or a read error, the helper runs Close's sequence: an
agent without its connection is unreachable.

**Close, in order.** Each step is skipped once the agent has exited.

1. Close the agent's standard input only, not the connection. A6: the connection keeps reading what the agent writes before it exits.
2. Wait up to the grace for the agent to exit.
3. On Unix, send SIGTERM to the group, `syscall.Kill(-pgid, SIGTERM)` (P12, P14), and wait up to the grace. Windows has no such step
   (S3, P7).
4. Kill the group with SIGKILL (P13, P14), or end the job with `TerminateJobObject` (P17). `Kill` on the agent is the fallback where
   neither applies. (Amended 2026-10-21, during the PLAN's step 3: on Unix the agent is killed directly too, after its group, as the
   Rust SDK does, so it ends even where the group cannot be signalled.) Wait for the agent's exit; `WaitDelay` bounds it (P11).
5. Close the connection, which closes the reader (P4) and the writer, and wait for its goroutines (`Wait`).
6. On Windows, close the job's handle.

Wait's error records which step ended the agent, so a Windows kill, which reports exit 0 (P17), still reads as a kill.

**The examples.** The three `stopAgent` copies (A1) give way to `(&agentproc.Command{Cmd: cmd}).Start(client)` and `Close`.

**The gate.** `scripts/check_api.py` compares `agentproc` as well as the root package. Before `agentproc` is released, its API is all
additions.

### Questions

Each has a recommended answer.

1. **Where the helper lives.**
   * Recommended: a new package, `agentproc`, so the protocol package keeps no process control and no new dependency (O1).
   * Alternative: the root package, as `acp.StartAgent` (O2).
2. **The API's shape.**
   * Recommended: `Start(cmd, client, opts ...Option)`, with functional options, as the root's `ConnectionOption` are.
   * Alternative: a struct with exported fields and a `Start` method, as `mcp.CommandTransport{Command, TerminateDuration}`.
3. **The agent's descendants on Windows.**
   * Recommended: a kill-on-close job object through `golang.org/x/sys/windows`, the module's first requirement, imported only by
     `tree_windows.go`. Both consumers already require it at `v0.49.0` (C3); it declares `go 1.26.0` (S8); it is the Go project's own.
   * Alternative: the same job object through `syscall.NewLazyDLL("kernel32.dll")`, with no requirement. The probes did this (P17,
     P18) on amd64, with the limit structure laid out by hand. Every other architecture's layout would then be this repository's to
     get right.
   * Alternative: the direct child only, as the Rust and MCP Go SDKs do on Windows (P9).
4. **The grace.**
   * Recommended: 5 seconds a step, as `mcp.CommandTransport` and the examples' `stopAgent` use, with `WithGrace` to change it.
   * Alternative: 2 seconds, as the Python SDK uses; or 1 second, as the Rust SDK uses.
5. **The terminating signal on Unix.**
   * Recommended: SIGTERM to the process group. C2's agent exits on it, with 128 plus its number, while it offers an interrupt to the
     running turn first.
   * Alternative: SIGINT; or no signal step, straight to SIGKILL, as the Rust SDK does.
6. **The process and the connection, tied.**
   * Recommended: the agent's exit ends what is left of its group or job, and then its connection; the connection's end stops the
     agent. This is what the Rust SDK does for the first, and what C1 builds by hand for both.
   * Alternative: neither; only `Close` stops anything, and the caller watches both.
7. **Close's error.**
   * Recommended: `exec.Cmd.Wait`'s error, joined with `ErrTerminated` or `ErrKilled` when Close escalated. P17 shows the exit status
     alone cannot tell a Windows kill from a clean exit.
   * Alternative: `exec.Cmd.Wait`'s error alone, as `mcp.CommandTransport` returns.
8. **The agent's stderr.**
   * Recommended: the caller's `cmd.Stderr`, as `mcp.CommandTransport` leaves it; the specification lets a client capture, forward or
     ignore it.
   * Alternative: the helper drains it and keeps a bounded tail for the error, as the Rust SDK does.
9. **The examples.**
   * Recommended: replace the three `stopAgent` copies with the helper, so the examples show it.
   * Alternative: leave them, and add one new example.
10. **A context.**
    * Recommended: `Start` takes none. A caller that wants the agent stopped with a context writes
      `context.AfterFunc(ctx, func() { p.Close() })`, which runs the whole sequence. A `cmd` from `exec.CommandContext` still works:
      its `Cancel` kills the direct child, and the helper's "own end" then ends the rest of the tree.
    * Alternative: `Start(ctx, …)`, whose context's end runs `Close`.

#### Decided with the owner, 2026-10-20

| Question | Answer |
| :--- | :--- |
| 1. Where the helper lives | As recommended: a new package, `agentproc`. |
| 2. The API's shape | **The alternative:** a struct with exported fields, `agentproc.Command{Cmd, Grace, ConnectionOptions}`, and its `Start(client)` method. The specification above is written so. |
| 3. The agent's descendants on Windows | As recommended: a kill-on-close job object through `golang.org/x/sys/windows`, the module's first requirement. |
| 4. The grace | As recommended: 5 seconds a step; `Command.Grace` changes it. |
| 5. The terminating signal on Unix | As recommended: SIGTERM to the process group. |
| 6. The process and the connection, tied | As recommended: the agent's exit ends what is left of its tree, then its connection; the connection's end stops the agent. |
| 7. Close's error | As recommended: `exec.Cmd.Wait`'s error, joined with `ErrTerminated` or `ErrKilled`. |
| 8. The agent's stderr | As recommended: the caller's `Cmd.Stderr`. |
| 9. The examples | As recommended: the three `stopAgent` copies replaced by the helper. |
| 10. A context | As recommended: `Start` takes none. |

This record names `golang.org/x/sys` as the dependency `AGENTS.md` requires a record for.

#### Amended by 0041, 2026-10-27: the exit order, and the output read to its end

Decided by the owner in [0041-MADR-close-the-gaps-the-consumers-wait-on.md](0041-MADR-close-the-gaps-the-consumers-wait-on.md):

* **`Process.ExitedFirst() bool`** is added (0041's question 3). It reports whether the agent's exit was seen before the connection's
  end, and is final once `Wait` has returned.
* **`Close` reads the agent's output to its end** (0041's item 1). Once the agent has exited, `Close` waits up to the grace for the
  connection's own end before closing it, as the watcher in
  [0027-PLAN-agent-process-helper.md](0027-PLAN-agent-process-helper.md) already does. `Kill` still closes at once. Question 6's
  answer stands: the agent's exit ends what is left of its tree, then its connection.
* **The tree code moves to `internal/proctree`,** unchanged, so that `clienthost.Terminals` can end a terminal's tree too (0041's
  question 1). `agentproc`'s API and behaviour are otherwise unchanged.

### Consequences

* Good, because a client starts an agent and stops all of it, launcher and descendants, in a few lines, with every wait bounded.
* Good, because no message the agent writes before it exits is lost (P1, P2, A6).
* Good, because on Windows the agent's tree ends even when the client process dies (P18).
* Good, because the protocol package and its supported platforms are unchanged (A4).
* Neutral, because on Unix the agent's tree outlives a client that dies without running `Close` (P18). Linux's `Pdeathsig` covers the
  direct child only, and is not proposed here.
* Neutral, because an agent in a group of its own no longer receives the terminal's interrupt; the client, which does, decides what an
  interrupt means, such as `session/cancel`.
* Neutral, because a descendant started before the job assignment escapes it on Windows (S7).
* Bad, because, by question 3's recommendation, the module gains its first requirement, `golang.org/x/sys`. A consumer that does not
  import `agentproc` does not build it, but its module graph lists it.
* Bad, because Start changes the caller's `cmd`: its `Stdin`, `Stdout`, `SysProcAttr` on Unix, and a zero `WaitDelay`.
* Bad, because a launcher that is meant to exit and leave its agent running in the same group or job has that agent ended by question
  6's recommendation.

### Confirmation

The PLAN will make each finding a test before the code relies on it, and see each test fail on a planted break in a scratch copy:

* the probes' cases as tests of the package, re-executing the test binary as a child, as `nil_panic_test.go` does in this repository
  and `mcp/cmd_test.go` in the MCP Go SDK: no lost output (P1, P2); a stubborn agent ended within its bounds (P6, P12, P13); a
  launcher's descendant ended (P14, P17); the Windows exit reported as a kill (P17); a `Stderr` writer held by a descendant (P10, P11);
* CI's three operating systems, at go1.26.9 and go1.27.2, and the Linux and Windows hosts reached over SSH before a push;
* `GOOS` builds for every target in A4;
* `scripts/check_api.py` covering `agentproc`; `go mod tidy -diff` and `govulncheck` covering `golang.org/x/sys`.

## Pros and Cons of the Options

### O1. A helper package, with the agent's tree

* Good, because it meets every driver.
* Good, because platform code and a Windows dependency stay out of the protocol package.
* Bad, because it is a second package to keep, document and gate.

### O2. The same helper in the root package

* Good, because callers import one package.
* Bad, because process control and `golang.org/x/sys` come with every import of the protocol, agents' included.

### O3. A minimal helper on the direct child

* Good, because it is small, and it matches `mcp.CommandTransport`.
* Bad, because a launcher's agent survives it (P8, P9), the common case for `npx` agents.

### O4. No helper

* Good, because it adds nothing to maintain.
* Bad, because every client rewrites the sequence, as three examples and C1 did, with C1 exposed to P1.

## More Information

* **Measurements outside the tree:** the probes, their source and their output stay outside the repository, as its records do for scratch
  tools. The consumers' trees and the other SDKs' clones were only read.
* **Related records:**
  * [0023-MADR-connection-lifetime-and-test-leaks.md](0023-MADR-connection-lifetime-and-test-leaks.md): the deferred item, and
    `Close`'s end of the reader;
  * [0026-MADR-connection-end-cause-and-wait.md](0026-MADR-connection-end-cause-and-wait.md): `Err` and `Wait`, which the helper's
    `Wait` uses;
  * [0021-MADR-resolve-issues-4-and-5-before-v1-14-0.md](0021-MADR-resolve-issues-4-and-5-before-v1-14-0.md): additive changes within
    `v1`.
* **Not decided here:**
  * the agent side: an agent's reaction to its input's end and to signals stays the agent's;
  * `Pdeathsig` on Linux;
  * capturing stderr, beyond question 8.
* **Schema `1.26.0`:** on 2026-10-20 the owner deferred its adoption, to be decided later
  ([0025-REPORT-schema-1-26-0-adoption-assessment.md](../reports/0025-REPORT-schema-1-26-0-adoption-assessment.md)). It does not touch
  this record.
* **The PLAN** is [0027-PLAN-agent-process-helper.md](0027-PLAN-agent-process-helper.md).
