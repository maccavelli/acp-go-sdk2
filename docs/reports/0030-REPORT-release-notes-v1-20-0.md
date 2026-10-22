# Release notes for v1.20.0, against v1.19.0

Observed 2026-10-21 on `02f20a8`, for [0030-PLAN-record-initialize-on-arrival.md](../decisions/0030-PLAN-record-initialize-on-arrival.md),
step 10. This report drafts the text of the GitHub release for `v1.20.0`, as `RELEASING.md` describes it, covering
[0029-PLAN-unix-client-death-pdeathsig.md](../decisions/0029-PLAN-unix-client-death-pdeathsig.md) and the 0030 PLAN. It decides nothing.

Each statement names its source:

- a record;
- a commit;
- or `make api`, run on `02f20a8` with nothing uncommitted.

Bring it up to date if more changes land before the tag.

- Four additions to the exported API, all in `agentproc`; the root package's exported API is unchanged. `make api` against `v1.19.0`:
  "0 incompatible, 4 compatible". The release is proposed as the minor `v1.20.0`, as `RELEASING.md` says for compatible changes. The
  owner confirms it at the tag.
- What a consumer may meet: a client now handles messages its agent sends with, or before, its `initialize` answer, where it refused some;
  on Linux and FreeBSD, an agent now ends when its client dies; on Windows, a process the agent starts with `CREATE_BREAKAWAY_FROM_JOB`
  now starts, and outlives the agent. The `go` lines and the requirements are unchanged.

The section "Release text" is what goes into the GitHub release.

## Release text

### v1.20.0

The ACP schema is still `1.25.0` (`git diff v1.19.0 -- schema/` is empty). This release records a client's `initialize` when its answer
arrives, and holds what the agent sends before it
([0030-MADR-record-initialize-on-arrival.md](../decisions/0030-MADR-record-initialize-on-arrival.md)); adds to `agentproc` taps on the
agent's streams, `Process.Kill` and job breakaway on Windows, for issue
[#10](https://github.com/maccavelli/acp-go-sdk2/issues/10) (the same record's amendment); starts the agent suspended on Windows until it is
in its job (its Windows amendment); and ends an agent with its client on Linux and FreeBSD
([0029-MADR-unix-client-death-pdeathsig.md](../decisions/0029-MADR-unix-client-death-pdeathsig.md)). `make api` reports four compatible
additions and no incompatible change.

#### Changed on the wire: `initialize` on the client

- **Recorded as the answer arrives.** `ClientSideConnection.Initialize` records the negotiation on the connection's reader, before it
  reads what the agent wrote after the answer, and returns the answer it recorded. Before, it recorded once its caller had the answer, so
  a message the agent sent with the answer could be refused as sent before `initialize` completed, an extension request answered
  `-32601` and a notification dropped. With both in the answer's write, of 1000 trials in each of two runs, the notification was handled
  in 62 and 54, the request in 16 and 15 (the MADR's I6). A caller whose context ends, or whose connection closes, once the answer has
  arrived gets the answer, and the record holds it. (`8923725`.)
- **Held until the answer, not refused.** While `Initialize` waits for its answer, a message the agent sends that only a record could
  allow is held, then judged against the record; it is refused, as before, when `initialize` fails by an error answer, another protocol
  version, the caller's context ending or the connection closing. Before `Initialize` is called nothing is held. (`c0c7e4d`.)
- **What a held message keeps.** A held notification waits in the notification queue, with those behind it, and a held request keeps its
  request slot, until the answer. An agent that sends more of them before its answer than `WithMaxQueuedNotifications` or
  `WithMaxConcurrentRequests` allows meets those limits: the overflow policy, which closes the connection by default, or `-32603`. Before,
  each was refused as it was judged; a burst written at once overflowed then too. (`02f20a8`.)
- **The hooks see the answer on the reader.** `WithInboundRewrite` and `WithInboundValidation` see the `initialize` answer once, on the
  connection's reading goroutine, before anything after it is read. What a rewrite hook makes of it is returned and recorded; an answer
  the validation hook refuses records nothing; a hook that blocks on it holds every message behind it. (`8923725`, `02f20a8`.)
- **The agent's side is unchanged:** it records before its answer is written, as before.

#### Changed: the reader's buffer

- A connection reads through a 128 KiB buffer, on either side, 64 KiB more per connection than before. A line between 64 KiB and
  128 KiB is read in one piece, with about 40% less allocation (the MADR's B3, B4). The message size limit is unchanged. (`b5177dd`.)

#### New in `agentproc`, for issue #10

- **`Command.OutputTap` and `Command.InputTap`,** writers that receive a copy of every byte the agent writes to its output, and of what
  the connection writes to its input, in order, before anything is decoded: a line too long or not JSON included. Each tap is written on
  a goroutine of its own, so the connection never waits for it, and its errors are ignored. A tap that falls 32 MiB behind is detached:
  what it received is the stream's beginning, and the new **`Process.DroppedTapBytes`** counts the rest. `Wait` gives the taps up to the
  grace to write what they hold. With no tap set, nothing changes. (`e2755e9`.)
- **`Process.Kill`** ends the agent, and the processes it started, at once, without `Close`'s waits, with `ErrKilled`; an agent that has
  exited is sent nothing; a `Close` in progress stops waiting and kills. (`445fc21`.)
- **Breakaway on Windows.** The agent's job allows breakaway: a process the agent's tree starts with `CREATE_BREAKAWAY_FROM_JOB`, which
  Windows refused before ("Access is denied"), now starts outside the job, and neither `Close`, `Kill` nor the agent's exit ends it, as a
  process that starts a session of its own already leaves the process group on Unix. It is the agent's to end. Every other process still
  ends with the job. (`d1055d6`.)

```go
f, err := os.Create("agent-output.jsonl")
if err != nil {
    log.Fatal(err)
}
p, err := (&agentproc.Command{Cmd: cmd, OutputTap: f}).Start(client{})
// ...
if out, _ := p.DroppedTapBytes(); out > 0 {
    log.Printf("the record lacks the last %d bytes", out)
}
```

#### Fixed in `agentproc`

- **On Windows, a process the agent starts at once is in its job.** The agent was assigned to its job after it started, so a process it
  started before the assignment was outside the job and outlived `Close`: with the assignment 100 ms late, in 20 of 20 trials (the MADR's
  J4a); `TestDescendantsEnded` failed in 1 of 15 runs of its package (J1). `Start` now starts the agent suspended, assigns it, then
  resumes it. A command whose `CreationFlags` already has `CREATE_SUSPENDED` is refused, since `Start` resumes the agent; the caller's
  other flags are kept. (`1133e55`.)
- **On Linux and FreeBSD, an agent ends with its client.** When the client dies without `Close`, the kernel sends the agent SIGKILL: `Start`
  sets `Cmd.SysProcAttr.Pdeathsig` to SIGKILL unless the caller set one. Before, an agent that ignores its input's end outlived its
  client. A process the agent starts still ends with the client only at its input's end; an agent that cleans up at its input's end
  loses that cleanup when its client dies, though not on `Close`. macOS and the other systems are unchanged. (`666af54`.)
- **A caller's `Pdeathsig` no longer ends the agent early.** On Linux the kernel sends it when the thread that started the agent ends, which
  could be long before the client did; in `v1.19.0` an agent started from a goroutine locked to its thread was killed as that goroutine
  returned. `Start` now starts the agent from a goroutine locked to its OS thread until the agent is reaped, on every platform: one OS
  thread per running agent. (`bbe7430`.)

#### Documentation

- The README's "Initialization and capabilities" and "Closing and limits", the docs of `Start`, `Kill`, `Command`'s taps and
  `ErrKilled`, `ExampleCommand_outputTap`, and `docs/architecture.md` describe the above. (`631b128`, `e2755e9`, `445fc21`, `d1055d6`,
  `02f20a8`.)

#### For maintainers

- **The generator** emits the client's `Initialize` with a commit run on the reader, an `initializing` attempt and a `heldGate`; four
  engine cases' `want.api` gain the `initializing` field. (`8923725`, `c0c7e4d`.)
- **New tests:** at the root, `initialize_test.go`, eleven tests, four of them repeated 200 times, and `TestReceiveReadBufferSize`;
  in `agentproc`, the taps', `Kill`'s, the suspended start's and breakaway's, and the client-death tests on Linux, FreeBSD and Unix.
  (`8923725`, `c0c7e4d`, `b5177dd`, `e2755e9`, `1133e55`, `445fc21`, `d1055d6`, `02f20a8`, `bbe7430`, `666af54`, `631b128`.)

## Sources not in the release text

- **Commits since `v1.19.0`** (`git log v1.19.0..02f20a8`), sixteen:
  - `bbe7430` starts the agent from a locked goroutine, in `agentproc.go`, with its tests;
  - `666af54` adds `deathsig_pdeathsig.go` and `deathsig_none.go`, and changes `tree_unix.go`, with their tests;
  - `631b128` changes `agentproc.go`'s docs, the README, `docs/architecture.md` and tests;
  - `8923725` changes `connection.go`, `client_gen.go` and the generator, and adds `initialize_test.go`;
  - `c0c7e4d` changes the same three, the generator's pins and four engine cases;
  - `b5177dd` changes `connection.go` and `jsonrpc_test.go`;
  - `e2755e9` adds `agentproc/tap.go`, changes `agentproc.go`, and adds tests and an example;
  - `1133e55` changes `tree_windows.go`, with a Windows test file;
  - `445fc21` changes `agentproc.go`, with `kill_test.go`;
  - `d1055d6` changes `tree_windows.go` and `agentproc.go`'s docs, with a Windows test file;
  - `02f20a8` changes the README, `docs/architecture.md`, `initialize_test.go` and the 0030 MADR;
  - `f193910`, `2f3dd47`, `e068f97`, `c2b33c1` and `ced609e` change records only (`git show --name-only`).
- **Go files changed.** `git diff --name-only v1.19.0 02f20a8 -- '*.go' ':!*_test.go'` lists nine: `agentproc`'s `agentproc.go`,
  `deathsig_none.go`, `deathsig_pdeathsig.go`, `tap.go`, `tree_unix.go` and `tree_windows.go`; the root's `client_gen.go` and
  `connection.go`; and the generator's `internal/emit/dispatch.go`. `go.mod`, `go.sum`, CI, the scripts, `RELEASING.md` and the examples
  are unchanged (`git diff`).
- **`make api` on `02f20a8`:** exit 0, "against v1.19.0: 0 incompatible, 4 compatible": `agentproc: (*Process).DroppedTapBytes`,
  `agentproc: (*Process).Kill`, `agentproc: Command.InputTap` and `agentproc: Command.OutputTap`, each "added".
- **The evidence:** [0029-PLAN-unix-client-death-pdeathsig.md](../decisions/0029-PLAN-unix-client-death-pdeathsig.md), its steps 1 to 5,
  with CI run `39660261939` on `631b128`; and [0030-PLAN-record-initialize-on-arrival.md](../decisions/0030-PLAN-record-initialize-on-arrival.md),
  its steps 1 to 9, with their planted breaks, gates on macOS and on the Linux and Windows hosts, and deviations (the commit's checks, the
  engine cases, the Windows window, what a held message holds). The figures above are the MADRs': 0030's I6, B3, B4, J1 and J4a; 0029's
  D2, D7, D9, D10 and D14.
- **Issue #10,** "agentproc: no hook on the agent's output stream, so a client that records the raw wire cannot adopt it (v1.19.0)", is
  open (`gh issue view 10`); the owner closes it with the release.
- **Not in this release:** schema `1.26.0`, deferred by the owner
  ([0025-REPORT-schema-1-26-0-adoption-assessment.md](0025-REPORT-schema-1-26-0-adoption-assessment.md)); a hold on the agent's side;
  a tap that blocks the connection instead of detaching; ending a process the agent starts when the client dies, on Unix.
