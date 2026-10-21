---
status: accepted
date: 2026-10-21
decision-makers: Repository owner
---

# Record a client's initialize when its answer arrives, and hold what comes before it

## Context and Problem Statement

While [0027-PLAN-agent-process-helper.md](0027-PLAN-agent-process-helper.md) built its tests, step 2 found that "the client refuses a
`session/update` before `initialize` has completed on its side ('initialize has not completed'): an agent that writes updates right after
its `initialize` answer can have the first few refused." That PLAN judged it no loss for session updates, which need a session the client
creates later, and worked around it in its test. On 2026-10-21 the owner asked for it to be scoped and specified, with the most robust,
idiomatic fix.

Every figure below comes from a command or a probe run that day. The repository was only read, at `f193910`. The probe is a Go program
outside the tree, built with go1.27.3 against the working tree through a `replace`, run on macOS.

### Where each side records initialize

The two side connections refuse every message but `initialize` until they hold a record of what `initialize` agreed (`gateMessage`,
`types_gen.go:23107`–`:23113`: `if n == nil { return fmt.Errorf("%s: %w", method, ErrNotInitialized) }`).

* **I1. The agent records before it answers.** `AgentSideConnection.handle`, for `initialize`, calls `a.negotiation.Store(&rec)`
  (`agent_gen.go:400`) inside the handler; the connection writes the answer after the handler returns. So no client message that follows
  the answer can reach the agent's gate before the record.
* **I2. The client records after its caller has the answer.** `ClientSideConnection.Initialize` calls `sendRequest`, which returns once
  the response has been handed over; it then checks the protocol version and calls `c.negotiation.Store(&rec)` (`client_gen.go:494`–`:523`)
  on the caller's goroutine. The generator emits it (`cmd/generate/internal/emit/dispatch.go:690`–`:693`).
* **I3. The reader moves on at once.** `handleResponse` (`connection.go:1577`–`:1604`), run by the reader as it reads a response
  (`:1508`), hands it to the waiting caller over a channel and returns to reading. `sendRequestSent` decodes and gates the result on the
  caller's goroutine (`:1807`, `:1820`, `:1823`). So a message the agent writes after its answer can be read, queued and handled before the
  caller has run `Store`.
* **I4. Extension messages pass the same gate.** `handleWithExtensions` hands a `_`-prefixed message to `handleExtension` with
  `c.inboundGate()` (`extensions.go:53`–`:58`), which returns `c.gate` (`client_gen.go:102`–`:104`), and `handleExtension` checks it
  (`extensions.go:67`).
* **I5. Only `initialize` records state this way.** `grep` for `.Store(`, `.Swap(` and `.CompareAndSwap(` in `client_gen.go` and
  `agent_gen.go` finds `c.negotiation.Store` and `a.negotiation.Store` alone.

### What it costs: the probe

A fake agent over two `io.Pipe`s reads the client's `initialize`, answers it, and sends an extension notification, `_x/hello`, and an
extension request, `_x/ask`; the client implements `ExtensionMethodHandler`. 1000 trials of each case, two runs:

| Id | Case | Handled, of 1000 |
| :--- | :--- | :--- |
| I6 | the two messages in the same write as the answer | notification 62 and 54; request 16 and 15 |
| I7 | the two messages 1 ms after the answer | notification 995 and 997; request 994 and 997 |
| I8 | agent side: a raw client sends `session/new` the moment it reads the `initialize` answer | answered 1000 and 1000 |

A refused request is answered `{"code":-32601,"message":"Method not found","data":{"error":"_x/ask: acp: initialize has not completed"}}`,
so the agent believes its client lacks the method; a refused notification is dropped, logged at Warn. I8 confirms I1.

### What the specification allows

* agentclientprotocol.com/protocol/v1/extensibility: "implementations **MAY** expose and call custom JSON-RPC requests as long as their
  name starts with an underscore"; "extensions **SHOULD** advertise their custom capabilities so that callers can check their
  availability first", in the `_meta` of the capability objects `initialize` exchanges.
* /protocol/initialization: "Before a Session can be created, Clients **MUST** initialize the connection". Nothing bars an agent from
  sending a message once it has answered `initialize`.

So an agent that advertises an extension in its `initialize` answer and uses it at once is within the specification, and this client
refuses it most of the time (I6). Session-scoped methods need a session the client creates after `Initialize` returns, so they cannot be
sent in the window in practice; extension messages can.

### Prior art

* **The MCP Go SDK** (`e844b98`) records its client's initialize result on the caller's goroutine too (`mcp/client.go:397`), but MCP's
  protocol closes the window: the client then sends `notifications/initialized` (`:401`), and a server is to wait for it before sending
  requests; the server's own comment calls its waiting flaky (`mcp/server.go:1665`).
* **ACP has no such third message,** so only the client can close the window.
* **The ACP TypeScript SDK** (`60619a4`, 2026-10-19), in its protocol-v2 code, `src/v2/stable/acp.ts`, does both O1 and O3 below:
  * its client sends `initialize` with a response parser that records the answer as it is parsed, `state.complete(parsed)`
    (`:1209`–`:1213`), on arrival, not after the caller resumes;
  * an inbound guard holds a message received while `initialize` is in flight, `await state.waitUntilInitialized(message.method)`
    (`:660`–`:663`), and refuses one received before `initialize` began, failing the initialization (`:575`–`:581`).

  Its v1 code has no such guard: `waitUntilInitialized` appears only under `src/v2`.
* `coder/acp-go-sdk` (`v0.13.5`) has no gate on what `initialize` agreed: a search of its sources for "not initialized", "before
  initialize" and "initialize has not" finds nothing. This SDK's gate is
  [0017-MADR-remediate-acp-v1-conformance-violations.md](0017-MADR-remediate-acp-v1-conformance-violations.md)'s G1.

## Decision Drivers

* **Conformance:** the gate of 0017 stays; no message the record forbids is let through.
* **Order:** a message the agent sends after its answer is judged against the record of that answer, as the agent's side already is (I1).
* **No new waits on the reader** that depend on another goroutine's progress: the reader is what every waiting call depends on.
* **Within `v1`, additive only**, and no exported API change.
* **The generator owns `client_gen.go`.**

## Considered Options

* **O1.** Commit on arrival: the reader records `initialize`, on the same goroutine and before it reads the next message, from the answer
  it has just read; the caller returns what was recorded.
* **O2.** A barrier: after handing the `initialize` answer over, the reader waits for the caller to record it before reading on.
* **O3.** Hold, not refuse: before a record exists, a received message waits until `initialize` completes or fails, then is gated.
* **O4.** Document only: agents should not send before the client is ready.

## Decision Outcome

Chosen option: "O1", with O3 for a message the agent sends before its answer, as the owner decided questions 1 to 4 on 2026-10-21
(see "Decided with the owner, 2026-10-21"): questions 1, 3 and 4 as recommended, question 2 the alternative. O1 orders the record before
every later message by construction, on one goroutine, with no wait on another; the hold judges what precedes the answer by the record
the answer makes, as the TypeScript SDK's v2 code does.

* O2 makes the reader wait on the caller; a caller that never resumes, its context ended between the hand-over and the record, holds every
  other call.
* O3 adds a queue and a bound for messages that cannot otherwise be judged, and delays requests the agent is waiting on; for what follows
  the answer, O1 alone suffices.
* O4 leaves I6, and the specification allows the agent's side of it.

### Specification, as decided

* **`connection.go`:** `pendingResponse` gains an unexported `commit func(result json.RawMessage) error`. `handleResponse`, when the
  pending request has a commit and the response is a result, calls it before it hands the response over, on the reader's goroutine; its
  error travels with the envelope. A response that is an error calls no commit.
* **`sendRequest`'s family:** an unexported variant takes the commit and returns its error before decoding, so a failed commit is the
  call's error. (Amended 2026-10-21, during the PLAN's step 2: it returns the value the commit checked, and decodes nothing.)
* **The generator** emits `ClientSideConnection.Initialize` with a commit that does what `Initialize` does today after `sendRequest`:
  decode the `InitializeResponse`, check the protocol version (`ProtocolVersionError`), apply the result gate, `jsonCopy` the record, and
  `c.negotiation.Store` it. `Initialize` then returns the decoded response or the commit's error. Its errors are unchanged.
* **The hold (question 2).** While the client's `initialize` is in flight, from the moment `Initialize` hands its request to the writer
  until the commit records the answer or `initialize` fails, a received message that the gate would refuse only because no record exists
  waits instead:
  * it waits in the client's gate, on the goroutine that judges it: the notification loop, so later notifications wait behind it in their
    order, or the request's own handler goroutine;
  * the commit wakes it, and it is then judged against the record, as any later message is;
  * `initialize` failing wakes it too, and it is refused as today: an error answer, a commit's error (`ProtocolVersionError`, a gate's),
    the caller giving up before the answer (question 3), or the connection's end;
  * the wait cannot block the answer it waits for: the reader reads and commits the answer itself (O1), and the caller's wait for the
    notifications received before the answer is satisfied once the commit wakes them;
  * a message received before `Initialize` is called is refused at once, as today; only an `initialize` in flight holds.
* **The agent's side** is unchanged (I1): a message a client sends before the agent has answered is refused, as today.
* **No exported API** changes; `make api` reports none.

### Questions

Each has a recommended answer.

1. **Where the record is made.**
   * Recommended: on the reader, at arrival (O1).
   * Alternative: a barrier, the reader waiting for the caller (O2).
2. **A message the agent sends before its answer,** while the client's `initialize` is in flight.
   * Recommended: refused, as today. No record exists to judge it by, and 0017's G1 is that nothing but `initialize` precedes the record;
     O1 changes only the order of the record and what follows the answer.
   * Alternative: held until `initialize` completes or fails, then judged, as the TypeScript SDK's v2 code does (O3 for this case alone).
3. **A caller that has given up.** Its context ended before the answer arrived, so the answer has no pending request.
   * Recommended: nothing is recorded, as today; `initialize` did not complete for that caller, and a later message is refused.
   * Alternative: record any `initialize` answer that arrives, pending or not.
4. **The probe as a test.**
   * Recommended: a root test that sends the answer and both extension messages in one write, 200 times, and requires every one handled;
     I6 shows it fails within the first few trials today.
   * Alternative: a single trial, which passes by chance about one time in twenty today.

#### Decided with the owner, 2026-10-21

| Question | Answer |
| :--- | :--- |
| 1. Where the record is made | As recommended: on the reader, at arrival (O1). |
| 2. A message sent before the answer | **The alternative:** held until `initialize` completes or fails, then judged. The specification above says how. |
| 3. A caller that has given up | As recommended: nothing is recorded; a held message is then refused. |
| 4. The probe as a test | As recommended: 200 trials in one write. |

### Consequences

* Good, because an agent can use an extension it advertised in its `initialize` answer at once.
* Good, because the record is made in the order the messages arrived, on the client as on the agent.
* Good, because a message the agent sends before its answer is judged by that answer, not refused for timing.
* Neutral, because, while a message is held, the notifications behind it wait too, until `initialize` completes or fails.
* Neutral, because the commit's work, a decode and a copy of one small answer, moves to the reader once per connection.
* Bad, because `sendRequest` gains an internal variant for one method.

### Confirmation

The PLAN will make each finding a test, seen to fail on a planted break in a scratch copy:

* I6 as a test (question 4), which fails with the commit removed and the record made by the caller;
* I8 as a test on the agent's side;
* `ProtocolVersionError` and the result gate's errors still returned by `Initialize`, and no record made after either;
* a message sent before the answer handled once `initialize` completes; refused when it fails, when the caller gives up, and when the
  connection ends; refused at once before `Initialize` is called;
* the engine cases and the generator's pins for the emitted commit; `make api` unchanged.

## Pros and Cons of the Options

### O1. Commit on arrival

* Good, because the order is by construction and needs no wait.
* Bad, because the commit runs on the reader, which must keep it short; it is one decode and a copy. (Amended 2026-10-21, during the
  PLAN's step 2: and the caller's inbound hooks, run once on the answer, as today; see "Amendment 2026-10-21, step 2".)

### O2. A barrier

* Good, because `Initialize`'s code stays where it is.
* Bad, because the reader waits on the caller.

### O3. Hold, not refuse

* Good, because no message is refused for timing.
* Bad, because it needs a queue, a bound and a timeout.

### O4. Document only

* Good, because it costs nothing.
* Bad, because a conforming agent's messages are refused.

## Amendment 2026-10-21, issue #10: the agent's streams, an immediate end, and job breakaway; and the reader's buffer

On 2026-10-21 the owner asked for issue #10 to be assessed and added to this record's scope, with the most idiomatic, flexible and stable
means of providing what it asks; and, the same day, decided that the connection's reader buffer grows from 64 KiB to 128 KiB. Questions 1
to 4 above stay as decided. This amendment adds questions 5 to 11, which the owner decided the same day (see "Decided with the owner,
2026-10-21, the amendment"); for question 5 the owner chose taps that each run on a goroutine of their own, and asked for that to be
evaluated, which "Taps on goroutines of their own" does.

Every figure below comes from a command or a probe run that day. The repository was only read, at `631b128`. The probes ran outside the
tree: a prototype in a scratch copy of `631b128`, on macOS; a buffer measurement on macOS, a Linux host and a Windows host reached over SSH;
a job probe on the Windows host; all with go1.27.3.

### The issue

Issue #10, opened by the owner on 2026-10-21, "agentproc: no hook on the agent's output stream, so a client that records the raw wire
cannot adopt it (v1.19.0)", asks for an opt-in hook on `Command` that lets a caller observe, or wrap, the agent's standard output before
the connection decodes it, and offers the same for its input. It names two related needs "so the design can account for them, not as part
of this request": ending an unused agent at once, and a process the agent starts on Windows that must outlive the agent. Its consumer is
magic-cli-remote, which records every line an agent writes as protocol fixtures, and would adopt `agentproc` in the release after one with
the hook.

### What the code does today

* **X1. Nothing runs between the pipe and the decoder.** `Start` refuses a `Cmd` whose `Stdout` is set (`agentproc/agentproc.go:110`) and
  hands the connection the read end it owns, unwrapped (`:157`).
* **X2. The hooks that exist see decoded values.** `WithInboundRewrite` sees a message "after it is decoded and before it is validated", and
  "A message that does not decode reaches no hook" (`connection.go:551`–`:559`); `WithInboundValidation` runs at the same point;
  `WithLogger` carries the connection's diagnostics.
* **X3. One goroutine reads, line by line.** `receive` reads through `bufio.NewReaderSize(c.r, 64*1024)` (`connection.go:1139`–`:1140`);
  `readLine` (`:1120`) gathers a line from `ReadSlice` chunks, and a line over `WithMaxMessageSize` is read through its newline and
  discarded. So the bytes as the agent wrote them exist whole only below the `bufio.Reader`.
* **X4. One goroutine writes,** one `c.w.Write` per message (`writeLoop`, `connection.go:705`–`:715`).
* **X5. `Close` ends a read only through `io.Closer`.** It closes the reader and the writer when they implement it (`connection.go:798`–
  `:820`); "A reader that Close cannot close, one that does not implement io.Closer, keeps Wait waiting until its Read returns" (`:768`–
  `:770`). `agentproc` depends on it: `watch` and `stop` close the connection (`agentproc.go:235`, `:298`) to end a read that a process the
  agent started still holds open (`TestEscapedDescendantDoesNotHoldWait`).
* **X6. The consumer's capture is a reader wrapper.** magic-cli-remote (`6db2792`), `internal/wirecap/wirecap.go:161`–`:193`: `TeeReader`
  returns a reader that appends what it reads to a buffer, records each complete line, and returns the source's `n` and error unchanged.
  Its type is not an `io.Closer`.
* **X7. `io.TeeReader` would end the connection on a capture's error:** "Any error encountered while writing is reported as a read error"
  (`io/io.go:617`).

### The prototype

`OutputTap` and `InputTap`, two `io.Writer` fields, in a scratch copy: a reader over the pipe that writes what it reads to the tap,
ignoring the tap's error, and a writer over the agent's input that does the same after each write.

| Id | Case | Result |
| :--- | :--- | :--- |
| T1 | an agent writes a 2 MiB line, a line that is not JSON and a notification; `WithMaxMessageSize(1 MiB)` | the output tap received 2,097,208 bytes, equal to what the agent wrote; the input tap received the connection's two answers, `-32600` "message exceeds 1048576 bytes" and `-32700` |
| T2 | a process the agent started holds the output; the tap's reader closes the pipe | `Wait` returned 440 ms after `Start` |
| T3 | the same; the tap's reader is not an `io.Closer` | **`Wait` still blocked 6 s after `Start`**, the agent exited and `Done` closed; it returned once that process was killed |
| T4 | a tap whose `Write` blocks until released 3 s later; `Close` | the agent, its output unread, did not say it was ready within 1 s; **`Close` returned after 3 s**, the agent terminated |

So a tap is byte-verbatim, the frames a capture most needs included (T1); the tap's reader must keep `Close`'s end of the read (T2, T3),
which a wrapper the caller supplies may not (X6); and a tap that blocks holds the connection, the agent's output and `Close` (T4).

### An immediate end

* **K1.** `Close`'s ladder waits up to the grace at each step (`agentproc.go:287`–`:299`), and `Grace` is per `Command` (`:39`–`:41`).
* **K2. The probe:** an agent that ignores its input's end, with a beat grandchild, at the default grace. `Close` returned after 5 s with
  `ErrTerminated`. The caller's own `Cmd.Process.Kill()` and then `Wait` returned after 10 ms with "signal: killed", the grandchild ended:
  the watcher ends the tree when the agent exits (`agentproc.go:222`–`:228`). That route is undocumented, reports no `ErrKilled`, and,
  when a process the agent started holds the output, still waits up to the grace for the connection to drain (`:229`–`:235`).
* **K3.** Killing while `Wait` runs is what os/exec does itself: "CommandContext sets the command's Cancel function to invoke the Kill
  method" ([0027-MADR-agent-process-helper.md](0027-MADR-agent-process-helper.md)). The ACP Python SDK's helper hands its caller the
  process, with `kill()`; the MCP Go SDK's `CommandTransport` has no kill of its own.

### Job breakaway on Windows

* **W1.** `attach` sets `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE` alone (`agentproc/tree_windows.go:43`); `end` terminates the job once the
  agent exits (`:74`).
* **W2. Microsoft's documentation:**
  * `CREATE_BREAKAWAY_FROM_JOB` (0x01000000): "The child processes of a process associated with a job are not associated with the job. …
    If the calling process is associated with a job, the job must set the JOB_OBJECT_LIMIT_BREAKAWAY_OK limit." (process-creation-flags)
  * `JOB_OBJECT_LIMIT_BREAKAWAY_OK` (0x00000800): "If any process associated with the job creates a child process using the
    CREATE_BREAKAWAY_FROM_JOB flag while this limit is in effect, the child process is not associated with the job."
  * `JOB_OBJECT_LIMIT_SILENT_BREAKAWAY_OK` (0x00001000): "Allows any process associated with the job to create child processes that are
    not associated with the job." (both JOBOBJECT_BASIC_LIMIT_INFORMATION)
  * Nested jobs: "If the immediate job object allows breakaway, the child process breaks away from the immediate job object and from each
    job in the parent job chain, moving up the hierarchy until it reaches a job that does not permit breakaway." (nested-jobs)
* **W3. The probe,** on the Windows host: a child assigned to a kill-on-close job starts a grandchild; the job is then terminated.

  | Id | The job | The grandchild | Result |
  | :--- | :--- | :--- | :--- |
  | W3a | kill-on-close | plain | in the job; ended with it |
  | W3b | kill-on-close | `CREATE_BREAKAWAY_FROM_JOB` | **not started: "Access is denied."** |
  | W3c | kill-on-close and `BREAKAWAY_OK` | plain | in the job; ended with it |
  | W3d | kill-on-close and `BREAKAWAY_OK` | `CREATE_BREAKAWAY_FROM_JOB` | **outside the job; alive 500 ms after the job was terminated** |

  The probe itself ran in the SSH session's job, whose flags were `0x1800` (both breakaway limits), so a parent chain that refuses breakaway
  was not probed; the nested-jobs text above says the process then leaves this job and stays in that one.
* **W4. The case the issue names.** go-selfupdate-lib (`18849cd`), `selfupdate/service/detach_windows.go:14`–`:25`, starts its detached run
  with `CREATE_BREAKAWAY_FROM_JOB` and, on `ERROR_ACCESS_DENIED`, again inside the job; on Unix in a new session
  (`internal/winproc/winproc_unix.go:18`). Under `agentproc` today, an update an agent starts so survives the agent's exit on Unix, having
  left the process group, and on Windows is refused breakaway (W3b), runs inside the job, and is ended by `end` when the agent exits.

### The reader's buffer (decided by the owner)

The owner decided on 2026-10-21 that `receive`'s `bufio.Reader` grows from 64 KiB to 128 KiB. Its one site is `connection.go:1140`, set when
[0017-PLAN-remediate-acp-v1-conformance-violations.md](0017-PLAN-remediate-acp-v1-conformance-violations.md) bounded the line size. A
measurement of `readLine`'s path, 64 MiB of lines of each size through an `os.Pipe` and an `io.Pipe`, one run each:

| Id | Measure | 64 KiB | 128 KiB |
| :--- | :--- | :--- | :--- |
| B1 | the largest `Read` from an `os.Pipe`, macOS and Linux | 65,536 B | 65,536 B: the pipe's capacity |
| B2 | the largest `Read` from an `os.Pipe` on Windows, and from an `io.Pipe` | 65,536 B | 131,072 B |
| B3 | `ReadSlice` chunks per 100 KiB line | 2 | 1 |
| B4 | allocated, 100 KiB lines | 107.5 MiB | 66.5 MiB |
| B5 | time, 100 KiB lines through an `os.Pipe`: macOS, Linux, Windows | 16.4, 103.1, 23.6 ms | 13.5, 80.3, 12.4 ms |
| B6 | chunks per line of 1 MiB and of 8 MiB | 16.02 and 128.12 | 8.02 and 64.12 |
| B7 | lines of 1 KiB and 16 KiB | no difference beyond run-to-run noise | |

So the larger buffer reads a line between 64 KiB and 128 KiB in one piece, with about 40% less allocation, and halves the pieces of a
longer one; it does not change what one read of a Unix pipe returns (B1). It costs 64 KiB more per connection, on either side.

### Prior art for the hook

None of the other SDKs offers the bytes as the peer wrote them:

* **ACP Rust SDK** (`b34addf`): `AcpAgent::with_debug`, a callback "invoked for each line sent/received", with its direction, stdin, stdout or
  stderr (`src/agent-client-protocol/src/acp_agent.rs:217`–`:241`); lines, their terminator removed, as UTF-8.
* **ACP Python SDK** (`11f8a4c`): stream observers, `add_observer`, "Register a callback that receives every raw JSON-RPC message"
  (`src/acp/connection.py:116`–`:117`), which receive each message decoded and deep-copied, in both directions (`:28`–`:39`, `:183`); an
  observer's exception is logged, "Stream observer failed" (`:189`), and never reaches the connection.
* **MCP Go SDK** (`b874fad`): `LoggingTransport`, "writing RPC logs to an io.Writer" (`mcp/transport.go:369`–`:370`), each message
  re-encoded with `jsonrpc2.EncodeMessage`, in both directions.
* **ACP TypeScript SDK** (`c2b1a86`) and **coder/acp-go-sdk** (`0845a3b`): the caller builds the streams, `ndJsonStream`
  (`src/core/stream.ts:45`) or the `io.Reader` and `io.Writer` (`connection.go:94`), and can wrap them; neither has a process helper.
* **ACP Kotlin SDK** (`a9a0f2e`): `Transport.onFrame`, incoming frames decoded (`acp/src/commonMain/kotlin/com/agentclientprotocol/transport/Transport.kt:45`).
* **The Go standard library** (`d934b909`) has no exported field of type `func(io.Reader) io.Reader`; the nearest are
  `xml.Decoder.CharsetReader` (`encoding/xml/xml.go:193`) and `mime.WordDecoder.CharsetReader`, wrappers that may fail the decode. Its
  observers of a stream are `io.Writer`s: `io.TeeReader`, `io.MultiWriter`.

The Rust, Python and MCP Go observers cover both directions, Kotlin's the incoming one; none lets an observer's failure reach the stream,
where `io.TeeReader` would (X7).

### Taps on goroutines of their own

The owner's answer to question 5: each tap runs on a goroutine of its own, so the connection never waits on it. The reader copies what it
reads into the tap's queue, which a goroutine writes to the tap in order. A queue that a tap empties more slowly than the reader fills it
must, at its bound, block the reader, drop bytes, or grow without one; this design detaches the tap, so that what it received stays an
exact prefix of the stream. Two measurements, macOS, go1.27.3:

* **A1. `readLine`'s path alone,** an `os.Pipe` flood of 64 MiB, which `readLine` reads at about 1 to 5 GiB/s:

  | Id | Tap | Lines | Reader | Delivered to the tap |
  | :--- | :--- | :--- | :--- | :--- |
  | A1a | synchronous, a file | 1 KiB | 47.4 ms | all |
  | A1b | own goroutine, a file, 16 MiB queue | 1 KiB | 56.9 ms | 33,968,128 of 67,108,864 B; detached |
  | A1c | own goroutine, a file, 16 MiB queue | 1 MiB | 34.0 ms | all; nothing dropped |
  | A1d | synchronous, 1 ms a write | 1 KiB | 1,355.2 ms | all |
  | A1e | own goroutine, 1 ms a write, 16 MiB queue | 1 KiB | 75.8 ms | 16,941,056 B; detached; its drain took 8.471 s after the end |
  | A1f | own goroutine, a `Write` that blocks until released after the end, 4 MiB queue | 1 KiB | 59.7 ms | 4,132,864 B, once released; detached |

* **A2. A real connection,** through `agentproc`: an agent writes 50,000 `session/update`s after `initialize`, with
  `WithMaxQueuedNotifications(1 << 20)`; the tap writes a file.

  | Id | Text per update | No tap | Synchronous | Own goroutine, queue of 4, 16 and 64 MiB |
  | :--- | :--- | :--- | :--- | :--- |
  | A2a | 1 KiB | 899 ms | 885 ms | 900, 900 and 906 ms; all 59,338,986 B delivered; largest queue 0.1 to 0.3 MiB |
  | A2b | 16 KiB | 8.479 s | 8.467 s | 8.562, 8.538 and 8.552 s; all 827,338,986 B delivered; largest queue 2.3 to 4.6 MiB |

  In every case the client received all 50,000 updates and `Wait` returned nil, and the tap's drain after `Wait` took under 1 ms.

So on a real connection a file tap keeps up, whether on the reader or on its own goroutine, and its queue stays under 5 MiB (A2); the
goroutine costs nothing measurable. What it buys is the case the synchronous tap loses: a tap that blocks or is slow no longer holds the
connection, the agent or `Close` (T4, A1d against A1e and A1f), at the price of a capture cut short (A1b, A1e, A1f) and memory up to the
queue's bound.

### Decision Drivers

* **The stream the connection judges is the agent's,** byte for byte: the gate of
  [0017-MADR-remediate-acp-v1-conformance-violations.md](0017-MADR-remediate-acp-v1-conformance-violations.md) judges what the agent sent.
* **The ties of [0027-MADR-agent-process-helper.md](0027-MADR-agent-process-helper.md) hold:** `Close` still ends the read (X5, T2), and
  `Wait` counts every goroutine, a tap's included, up to the grace. ~~No new goroutine runs.~~ (Amended the same day, by the owner's answer
  to question 5: each tap set runs one.)
* **Opt-in and free when unset:** with no hook, `Start` builds what it builds today.
* **Within `v1`, additive:** new fields and methods only; `make api` reports them compatible, and nothing incompatible.
* **Idiomatic Go:** an observer is an `io.Writer` (X7, prior art); an immediate end is `Kill`, as on `os.Process`.

### Considered Options

For the hook (question 5):

* **H1.** Taps: `OutputTap io.Writer` receives a copy of what the connection reads from the agent's output; `InputTap io.Writer`, of what
  it writes to the agent's input (question 6).
* **H1-async** (the owner's). H1, each tap on a goroutine of its own, fed by a queue bounded in bytes (questions 9 to 11).
* **H2.** Wrappers: `Output func(io.Reader) io.Reader`, and `Input func(io.Writer) io.Writer`; the connection reads what the wrapper returns.
* **H3.** A root `ConnectionOption` that observes each frame, in both directions, on every transport.
* **H4.** A `Cmd.Stdout` the caller sets, taken as a copy target.

For an immediate end (question 7):

* **K-a.** `Process.Kill`: ends the agent and the processes it started at once, and returns `Wait`'s error.
* **K-b.** Document the caller's `Cmd.Process.Kill()` and `Wait` (K2).

For breakaway (question 8):

* **W-a.** The job allows breakaway: `JOB_OBJECT_LIMIT_BREAKAWAY_OK`, never `SILENT_BREAKAWAY_OK`.
* **W-b.** A `Command` field, for Windows, that allows it.
* **W-c.** No change.

### Decision Outcome

Chosen option: "H1-async", with the input tap; "K-a"; "W-a"; and the 128 KiB buffer; as the owner decided questions 5 to 11 on 2026-10-21.
H1-async gives the consumer every byte (T1, A2) while the connection still reads only what the agent wrote, keeps `Close`'s end of the read
because `agentproc`, not the caller, builds the reader (T2), lets no capture's failure reach the connection (X7), and, on its own goroutine,
lets no slow or blocked capture hold the connection, the agent or `Close` (A1). K-a makes the end that already works (K2) a documented,
reported and immediate one. W-a lets a process that asks to leave the job leave it (W3d), as a Unix process that starts a session leaves
the process group, while every other process stays in it (W3c).

* H1, recommended before the owner's answer, writes on the reader: exact and free of copies, but a tap that blocks holds the connection,
  the agent's output and `Close` (T4), and a slow one slows the reader to its pace (A1d).

* H2 lets the caller change, delay or drop the stream the connection judges; a wrapper that is not an `io.Closer`, as the consumer's is
  (X6), stops `Close` ending the read (T3) unless `Start` closes the pipe beside it; a wrapper's error or goroutines are outside the
  process's accounting. It serves what H1 does not, a transformation, which no one has asked for.
* H3 adds to the root package an observer of what its `io.Reader` and `io.Writer` already expose to anyone who builds a connection, and
  sees lines after `readLine`, never an overlong line whole (X3).
* H4 gives `Cmd.Stdout` a meaning os/exec does not, and only for output.
* K-b leaves the end undocumented, without `ErrKilled`, and waiting up to the grace when the output is held (K2).
* W-b adds a field that means something on one platform, for what W-a gives every caller with the agent's consent; W-c leaves W4.

#### Specification, as decided

* **`Command.OutputTap io.Writer`.** When set, `Start` hands the connection, in place of the pipe's read end, a reader that reads the pipe
  and returns the pipe's `n` and error unchanged, and whose `Close` closes the pipe. Each read's `p[:n]`, up to 128 KiB (B1, B2), is copied
  into the tap's queue before the bytes are decoded: every byte the agent writes, in order, overlong and malformed lines included, through
  the end of its output. The read never waits on the tap.
* **`Command.InputTap io.Writer`** (question 6). When set, the connection's writer writes to the agent's input, then copies `p[:n]` of each
  write into the tap's queue; its `Close` closes the shared input, as today.
* **A tap's goroutine** writes its queue to the tap in order, one `Write` per copy, ignoring the tap's error. It starts with `Start` and
  ends once its stream has ended and its queue is empty. One writer set as both taps must be safe for concurrent use; no order between the
  two streams is defined.
* **The bound** (question 10): a tap's queue holds at most 32 MiB, a named constant. A copy that would pass it detaches the tap (question
  9): nothing more is queued for it, and every byte it does not receive is counted.
* **The drain** (question 11): `Wait`, once the agent has exited and the connection's goroutines have returned, waits up to the grace for
  each tap's goroutine to write its queue, then detaches the tap and counts what was left. A `Write` that never returns keeps its goroutine
  past `Wait`; the documentation says the writer must return.
* **`Process.DroppedTapBytes() (output, input uint64)`**: the bytes each tap did not receive, final once `Wait` has returned; zero for a
  complete capture. It mirrors `Connection.DroppedNotifications`.
* **Unset,** `Start` hands the connection the pipe and the input as today, and starts no goroutine.
* **`Process.Kill() error`** (question 7). It runs `Close`'s steps without their waits: it closes the agent's input, kills the process group
  or ends the job, or kills the agent where there is neither, records `ErrKilled` when it did, closes the connection, and returns `Wait`'s
  error. A `Close` in progress stops waiting and kills; a `Close` or `Kill` after it returns the same error. An agent that has exited is
  sent nothing.
* **Breakaway** (question 8). `attach` sets `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE | JOB_OBJECT_LIMIT_BREAKAWAY_OK`. A process the agent's tree
  starts with `CREATE_BREAKAWAY_FROM_JOB` is outside the job, and neither `Close`, `Kill`, the agent's exit nor this process's death ends it;
  every other process stays in the job.
* **The reader's buffer** (decided). `receive` reads through a 128 KiB `bufio.Reader`, named as a constant, on both sides.
* **`make api`** reports `Command.OutputTap`, `Command.InputTap`, `Process.Kill` and `Process.DroppedTapBytes` as compatible additions, and
  nothing incompatible; the release stays the minor `v1.20.0`.

### Questions

Each has a recommended answer.

5. **The hook's form.**
   * Recommended: taps, `io.Writer`s that receive a copy (H1).
   * Alternative: wrappers, `func(io.Reader) io.Reader`, with `Start` closing the pipe itself (H2).
6. **The agent's input.**
   * Recommended: `InputTap` too, as the Rust, Python and MCP Go observers cover both directions; a fixture of a whole exchange needs
     the client's side.
   * Alternative: the output only, as the issue requires.
7. **An immediate end.**
   * Recommended: `Process.Kill` (K-a).
   * Alternative: document `Cmd.Process.Kill()` (K-b).
8. **Breakaway on Windows.**
   * Recommended: the job allows it (W-a).
   * Alternative: a `Command` field that allows it (W-b); or no change (W-c).
9. **A tap whose queue is full,** asked once the owner had chosen H1-async.
   * Recommended: detach it; what it received stays an exact prefix, and the rest is counted.
   * Alternative: drop that read's copy and go on, leaving a hole; or block the reader until there is room, which waits on the tap.
10. **The queue's bound.**
    * Recommended: 16 MiB, fixed: about 3.5 times the largest queue measured (A2b), and more than one message of the 10 MiB default limit.
    * Alternative: a `Command` field.
    * Alternative, the owner's question: a cache that holds what arrives while the tap is detached and replays it first once the tap
      catches up. Weighed the same day: replayed in order, it delivers exactly what one queue of the two sizes together would, so it is a
      larger bound with a second path; a tap slower than the stream fills any finite cache (A1e) and is detached later rather than not at
      all, and an unbounded one lets the agent's output grow this process's memory without limit; on disk, it would write the wire,
      prompts and file contents included, to a file the caller did not choose, past any redaction the caller's tap does (X6). What it
      buys is room for a passing stall, which only the total size decides: at the fastest rate measured on a real connection, about
      93 MiB/s (A2b), 16 MiB holds about 0.17 s.
11. **The drain at the end.**
    * Recommended: `Wait` waits up to the grace, then detaches; a slow tap took 8.471 s to drain 16 MiB (A1e).
    * Alternative: `Wait` waits until every queued byte is written, without a bound.

#### Decided with the owner, 2026-10-21, the amendment

| Question | Answer |
| :--- | :--- |
| 5. The hook's form | **The owner's variant:** `io.Writer` taps, each on a goroutine of its own that does not block the connection (H1-async), evaluated in "Taps on goroutines of their own". |
| 6. The agent's input | `InputTap` too, on a goroutine of its own. |
| 7. An immediate end | As recommended: `Process.Kill`. |
| 8. Breakaway on Windows | As recommended: the job allows it. |
| 9. A tap whose queue is full | As recommended: detached. |
| 10. The queue's bound | **The owner's variant:** 32 MiB, fixed, once the cache above was weighed and found to be a larger bound: about 7 times the largest queue measured (A2b), and about 0.34 s of a stall at the fastest rate measured. |
| 11. The drain at the end | As recommended: up to the grace. |

### Consequences

* Good, because a client that records the wire can adopt `agentproc` (the issue), byte for byte, with no change to what the connection reads.
* Good, because a tap that fails, is slow or blocks cannot end, slow or hold the connection, and one that is unset costs nothing.
* Good, because an unused agent ends at once, with the cause reported.
* Good, because a process that asks to outlive the agent can, on Windows as on Unix.
* Good, because a line between 64 KiB and 128 KiB is read in one piece (B3, B4).
* Neutral, because a tap sees reads, not lines; a capture splits them, as the consumer's already does (X6).
* Bad, because a tap that falls 32 MiB behind is detached, and its capture is then a prefix; `DroppedTapBytes` says so.
* Bad, because each tap set holds a goroutine and up to 32 MiB of copies, and a tap's `Write` that never returns keeps its goroutine.
* Bad, because, on Windows, a process that asks to leave the job escapes `Close` and the client's death, as one that starts a session does
  on Unix.
* Bad, because each connection holds 64 KiB more.

### Confirmation

The PLAN makes each finding a test, seen to fail on a planted break in a scratch copy: T1 for both taps, complete at `Wait`; T2 with a
tap set, which fails when the tap's reader is not an `io.Closer` (T3); a tap whose `Write` always fails leaving the connection whole; a tap
that blocks leaving the connection and `Close` unheld and detached with an exact prefix (A1f); a slow tap's drain bounded by the grace
(A1e); the taps unset leaving the pipe and the input as they are, with no goroutine; K2 as `Kill` within the slack, with `ErrKilled`, and
cutting a `Close` short; W3c and W3d on Windows; the buffer's size at the connection's first read.

## Amendment 2026-10-21, step 2: the commit's checks, and a caller that gives up

Found while the PLAN's step 2 began, by reading `c2b33c1`, and decided by the owner the same day; the decision, O1, is unchanged.

* **The hooks decide the record today.** `Initialize` stores it only after `sendRequest` returns without error (`client_gen.go:507`–`:523`),
  by which time `checkInbound` has run the rewrite hook, the validation and its hook, and the gate on the answer (`connection.go:1823`,
  `:2156`–`:2181`). So the commit runs that whole path, once, on the reader, with the caller's context, and the caller returns the value
  it checked. The specification's "apply the result gate" means `checkInbound`, hooks included; its "one decode and a copy" also runs the
  caller's hooks, once.
* **A caller that gives up.** The reader claims the pending request (`:1584`–`:1588`) before it hands the answer over (`:1603`). A request
  with a commit whose caller gives up, or whose connection ends, after the claim waits for the commit and returns it; before the claim,
  nothing is recorded, as question 3 decided. A committed answer is returned when the wait for earlier notifications then ends. So
  `Initialize`'s return and the record always agree.
* **Consequence:** the hooks, for the `initialize` answer alone, run on the reader goroutine, and before the notifications received ahead of
  the answer are handled, where today they run on the caller's after them.

## More Information

* **Measurements outside the tree:** the probe and its output stay outside the repository, as its records do for scratch tools.
* **Related records:** [0017-MADR-remediate-acp-v1-conformance-violations.md](0017-MADR-remediate-acp-v1-conformance-violations.md), the
  gate; [0027-PLAN-agent-process-helper.md](0027-PLAN-agent-process-helper.md), step 2, where this was found.
* **Not decided here:** an `initialized` notification, which would be a protocol change, not this SDK's.
* **Issue #10** and its related needs: the amendment of 2026-10-21 above.
* **The PLAN** is [0030-PLAN-record-initialize-on-arrival.md](0030-PLAN-record-initialize-on-arrival.md).
