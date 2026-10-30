---
status: accepted
date: 2026-10-28
decision-makers: the repository owner
consulted: the source of the ACP SDKs, MCP SDKs and Go projects named under "Findings"; both known consumers' code and records
informed: the consumers, through their own records
---

# Run an agent in the client's process with agentproc's lifecycle; serve agents gracefully, keep their stderr, and record their transcripts

## Context and Problem Statement

A client that is also the author of its agent can run that agent in its own process, joined by in-memory pipes, instead of as a
child process. The bytes stay ACP. What it loses is the lifecycle `agentproc` gives a child: a bounded `Close`, `Kill`, `Wait` with
the cause of the end, `WaitContext`, `Done`, `ExitedFirst`, and the taps. Today it writes that lifecycle itself.

Two earlier records declined this:

* [0035-REPORT-consumer-acp-code-for-the-sdk.md](../reports/0035-REPORT-consumer-acp-code-for-the-sdk.md), section F: "`ServeAgent`
  and an in-process `agentproc`", among the items "generic in shape, but each consumer's policy differs".
* [0041-MADR-close-the-gaps-the-consumers-wait-on.md](0041-MADR-close-the-gaps-the-consumers-wait-on.md), "Not chosen": "An in-process
  agent runner (`<cli-b>`'s `acpclient.Start`)", not recommended, citing 0035 F.

The owner asked for an assessment of one shape of it, here O2: a runner for a serve function, whose `Start` returns the same
`*agentproc.Process` a child process gets. This record collects the findings and the decision. The owner answered its seven questions on
2026-10-28, each as recommended ("Decided with the owner, 2026-10-28").

**The question.** Should `agentproc` run a Go serve function as an agent, with the lifecycle a child process has, and if so in what
shape?

**Status.** Item 1, the in-process runner, was decided with the owner on 2026-10-28. On the same day the owner asked for three more
items, 2 to 4, and answered their questions ("Decided with the owner, items 2 to 4, 2026-10-28"). The record is `accepted`.
On 2026-10-30, issue #12 brought an amendment, `proposed` until the owner approves it: a bounded close, a testable handlers error,
and `Wait`'s error said once ("Amended 2026-10-30, issue #12").

### Findings in this repository

* **F1. `agentproc` is the subprocess half,** and its lifecycle is the one to match:
  * `Command{Cmd, Grace, ConnectionOptions, OutputTap, InputTap}.Start(client) (*Process, error)`;
  * `Process`: `Close`, `Kill`, `Wait`, `WaitContext`, `Done`, `Conn`, `ExitedFirst`, `DroppedTapBytes`;
  * `ErrTerminated` and `ErrKilled` are joined to `Wait`'s error by the step that ended the agent (`go doc ./agentproc`).
* **F2. `Close` is a ladder of bounded steps** (`agentproc/agentproc.go:453`, `stop`):
  1. close the agent's input and wait up to the grace;
  2. on Unix, SIGTERM to the group, and wait up to the grace again;
  3. kill the group or job.

  `Kill` runs the steps without their waits. The watcher alone closes the connection, once it has read to the end of the agent's
  output, up to the grace from the exit (`watch`, `:276`; 0041's item 1).
* **F3. `Process`'s state is a child's.** Its fields are `cmd *exec.Cmd`, `stdin *sharedClose` over an `*os.File`, and
  `tree *proctree.Tree` (`agentproc.go:77`–`:119`). `run` locks its OS thread for the Pdeathsig (`:221`). The taps read an
  `*os.File` (`tapReader.f`, `agentproc/tap.go:131`–`:135`). A second kind of agent needs those parts behind an internal seam.
* **F4. The tests guard goroutines.** `agentproc/leak_test.go:32` fails the package's run when a test leaves a goroutine leaked, on
  the `goroutineleak` profile (0023-MADR-connection-lifetime-and-test-leaks.md, T2).
* **F5. The in-memory pair exists for tests only.** `acptest.Connect` joins an `acp.Agent` and an `acp.Client` over two `io.Pipe`s,
  takes a `testing.TB`, and fails the test on a bad end (`acptest/pair.go:36`–`:94`). It has no process-like handle.
* **F6. The root package's goroutine rule does not reach `agentproc`.** `TestGoStatementsCounted` (`connection_end_test.go:452`)
  counts the root package's `go` statements only.

### Findings in the consumers

* **C1. `<cli-b>` runs its own agent in-process, in production.** Read at `5fb6928`:
  * `acpclient.Start(ctx, serve ServeFunc, opts Options) (*Conn, error)` (`acpclient/conn.go:116`), with
    `type ServeFunc func(ctx context.Context, in io.Reader, out io.Writer) error` (`:33`);
  * the CLI's modes call it with `agentServe(e)`, which wraps `acpserver.Serve` (`internal/cli/agent.go:243`, `:263`). The same
    `acpserver.Serve(ctx, in, out, opts)` (`acpserver/serve.go:28`) serves `<cli-b> acp` on stdio;
  * five test files call `acpclient.Start`.
* **C2. What `<cli-b>`'s runner does:**
  * two `io.Pipe`s; `serve` runs on a goroutine with `context.WithoutCancel(ctx)`; when it returns, the pipe the client reads is
    closed with `CloseWithError(cmp.Or(err, io.EOF))`, so the client's reads end (`conn.go:117`–`:125`);
  * `Close` (`:179`–`:202`): close the agent's input, wait up to `closeTimeout` (5 s) for `serve`; then close the connection and wait
    up to `closeTimeout` again for its handlers (`handlersDone`, `:207`). It returns the agent's error joined with the connection's.
* **C3. Its serve function's context is never cancelled.** `Start` passes `context.WithoutCancel(ctx)` (`conn.go:122`), and neither
  `Start` nor `Close` cancels it. The only stop signals the agent gets are its input's end and the pipes closing.
* **C4. `<cli-b>`'s own design names the choice.** Its 0002 MADR: interactive commands "are `ClientSideConnection` calls, in-process
  over pipes or to a spawned stdio child … The PLAN picks in-process vs child transport; this record requires that the bytes are
  ACP" (`docs/decisions/0002-MADR-cli-acp-headless-mcp-v1.md:200`–`:204`). Its 0031 MADR's D5 bounds every wait on the connection's
  end, since its notification handlers do not read their context
  (`0031-MADR-sdk-current-releases-and-adoption.md:207`).
* **C5. magic-cli-remote does not run an agent in-process.** Read at `5d1e7e2c`: it starts every agent through `agentproc.Command`
  (`internal/provider/acpagent/acpagent.go:513`). Its only agent-side connection is a probe binary for its supervisor tests,
  `test/supervisors/probeagent/main.go:143`, run as a child.
* **C6. What `v1.25.0` already replaces in C2.** `handlersDone` is `WaitContext`, and `serve.go`'s `ended` is `EndedCleanly`
  ([0041-REPORT-consumer-adoption-of-v1-21-to-v1-25.md](../reports/0041-REPORT-consumer-adoption-of-v1-21-to-v1-25.md)). About 35
  lines remain the runner's own: the pipes, the goroutine, the close of the read end, and `Close`'s order.

### Findings in other ACP implementations

Read at the commits named, from shallow clones; none was modified.

* **E1. No ACP SDK ships an in-process runner with a lifecycle.** In-memory pairs are common, as transports or for tests:
  * **TypeScript**, `@agentclientprotocol/sdk` 2.0.2 (agentclientprotocol/typescript-sdk `bca8e49`). `AgentApp.connect(client:
    ClientApp)` joins two apps directly, "useful for tests and in-process examples that do not need a transport"
    (`src/v1/acp.ts:1791`–`:1797`). The pair is closed together: each side's `closed` closes the other (`:2020`–`:2021`). The
    stream pair `memoryStreamPair()` is not exported (`:118`).
  * **Rust**, `agent-client-protocol` 3.5.0 (agentclientprotocol/rust-sdk `644377a`). It has a public
    `Channel::duplex() -> (Self, Self)` over unbounded channels (`src/agent-client-protocol/src/jsonrpc.rs:7573`, `:7585`), "in-process
    message channels (for testing or direct connections)". An in-process agent is a future the caller polls, and dropping it cancels
    it. There is no handle.
  * **Python**, `agent-client-protocol` 0.12.1 (agentclientprotocol/python-sdk `2ad204b`). `memory_transport_pair()` is private
    (`src/acp/_transport.py:141`), used by tests.
  * **Kotlin** (agentclientprotocol/kotlin-sdk `cb69c39`): an in-memory pair only in a sample, `createInMemoryTransportPair()`.
  * **`coder/acp-go-sdk`** v0.13.5 (`0845a3b`): no helper; tests join two `io.Pipe` pairs by hand.
* **E2. The subprocess helpers stop a child in steps, as `agentproc` does:**
  * Python's `spawn_stdio_transport` closes stdin, waits `shutdown_timeout` (2 s), then `terminate()`, waits, then `kill()`
    (`src/acp/transports.py:95`–`:112`). Its docstring says it mirrors the MCP Python SDK.
  * Rust's `AcpAgent` gives the child `SHUTDOWN_GRACE_PERIOD`, 1 s (`src/agent-client-protocol/src/acp_agent.rs:24`), then SIGKILLs
    its process group, with no SIGTERM step.
  * TypeScript, Kotlin and `coder/acp-go-sdk` have no subprocess helper in the library.
* **E3. Zed runs its native agent in-process without ACP's wire.** `NativeAgentServer` returns a `NativeAgentConnection` as
  `Rc<dyn AgentConnection>` (zed-industries/zed `37faef8`, `crates/agent/src/native_agent_server.rs:23`–`:56`). In-memory ACP pairs
  appear only in its tests.
* **E4. The specification** defines stdio, whose end is "Close stdin, terminate subprocess", and allows custom transports that "MUST
  ensure they preserve the JSON-RPC message format and lifecycle requirements defined by ACP"
  (agentclientprotocol/agent-client-protocol `d6f00fd`, `docs/protocol/v1/transports.mdx:40`, `:50`–`:52`). It defines no
  in-process transport.

### Findings in other Go projects

* **M1. MCP's Go SDK** v1.8.0: `NewInMemoryTransports` is `net.Pipe()` under the newline-JSON connection stdio uses; "Servers must be
  connected before clients" (`mcp/transport.go:177`–`:186`). Its `CommandTransport.Close` closes stdin, waits `TerminateDuration`
  (5 s), sends SIGTERM, waits, then kills (`mcp/cmd.go:16`–`:106`). The in-memory transport has no lifecycle beyond `Close`.
* **M2. mark3labs/mcp-go** v1.4.0: `NewInProcessClient(server)` calls `server.HandleMessage` on the caller's goroutine, with no pipe
  or read loop; `Close` always returns nil and waits for nothing (`client/transport/inprocess.go:151`–`:212`).
* **M3. MCP TypeScript** v2.4.0: `InMemoryTransport.createLinkedPair()` is "Intended for testing and development"
  (`packages/core-internal/src/util/inMemory.ts:10`–`:73`). **MCP Python** v1.30.0 runs the server in an anyio task group and cancels
  it on exit (`src/mcp/shared/memory.py:24`–`:98`). v2.3.0 replaced that with a direct dispatcher.
* **M4. gopls runs the same server three ways in its integration tests:** in-process over pipes, through an in-process forwarder,
  and as a separate process (golang/tools `gopls/v0.23.0`, `gopls/internal/test/integration/runner.go:53`–`:74`). The pipes are
  `net.Pipe` (`internal/jsonrpc2/servertest/servertest.go:63`–`:91`). None of these gives the in-process side `Wait`, a cause, or
  `Kill`.
* **M5. `net/http/httptest.Server.Close`** "blocks until all outstanding requests on this server have completed", with no bound. It
  logs after 5 s (go1.27.3, `src/net/http/httptest/server.go:468`–`:525`).
* **M6. gRPC-Go's `bufconn`** is a buffered in-memory listener (v1.82.0, `test/bufconn/bufconn.go`). An in-process transport that
  does not serialize has been an open issue since 2016 (grpc/grpc-go#906).

No Go library was found that gives an in-process server a process-like lifecycle. O2 would be new ground, built from parts the
standard library and this repository already have.

### Go's idioms and guarantees

* **G1. `io.Pipe`** is "synchronous", with "no internal buffering". `CloseWithError(err)` makes the other end's reads return `err`,
  or `io.EOF` for nil, and its writes return `err`. The first error is kept. Each direction closes on its own (go1.27.3,
  `src/io/pipe.go`). **`net.Pipe`** is full duplex with deadlines, but its `Close` ends both directions at once, so it cannot close
  the agent's input alone (`src/net/pipe.go`).
* **G2. A goroutine cannot be killed.** A Go program cannot stop another goroutine; it can only ask, through a context, a closed
  channel or failed I/O. A step that "kills" an in-process agent can close its pipes and cancel its context, and nothing more.
* **G3. Goroutine lifetimes.** "Never start a goroutine without knowing how it will stop"
  (<https://dave.cheney.net/2016/12/22/never-start-a-goroutine-without-knowing-how-it-will-stop>). Go Code Review Comments: "When
  you spawn goroutines, make it clear when - or whether - they exit" (<https://go.dev/wiki/CodeReviewComments>).
* **G4. Panics.** An unrecovered panic ends the program, and `recover` works only in the panicking goroutine (the Go specification,
  "Handling panics"). net/http recovers handler panics, which golang/go#25245 calls "regrettable but we can't change it in Go 1.x".
  x/sync's `errgroup` v0.24.0 declines to propagate a panic: "it turns f's panic stack into a mere value, hiding it from
  crash-monitoring tools" (`errgroup/errgroup.go:81`–`:91`). `sync.WaitGroup.Go`: "The function f must not panic."
* **G5. Contexts.** `context.WithCancelCause` and `context.Cause` (Go 1.20) carry why a context ended. `context.WithoutCancel` (Go
  1.21) keeps a parent's values and drops its cancellation, as C2 uses it.
* **G6. `os/exec`'s `Cmd.Cancel` and `WaitDelay`** (Go 1.20) are the standard library's ladder: a polite stop, then force after a
  bound, which "closes the pipes … to unblock any goroutines currently blocked on Read or Write", reported as `ErrWaitDelay`
  (`src/os/exec/exec.go:262`–`:314`).
* **G7. `errors.Join`** keeps every error inspectable with `errors.Is` and `errors.As`, and drops nils.

Not verified: how a panic in a Rust in-process agent propagates; how the TypeScript SDK reports a handler that throws; JetBrains and
other clients that may embed an agent.

### Re-assessment, 2026-10-28

The owner asked for this record to be re-assessed against the codebase, its API and command surfaces, and both consumers as they
stand, and for three more items that would ease the consumers' adoption. What was found changes nothing in item 1. It grounds
items 2 to 4, set out under "Items 2 to 4, added 2026-10-28".

#### This repository now

* **R1. The surfaces at `26a7242` (`v1.25.0`).**
  * Top-level declarations, as `go doc -short` prints them, methods not counted:
    * the root package: 155 functions and 404 types;
    * `acptest`: 4 functions and 8 types;
    * `agentproc`: 1 function, 2 types and 2 variables;
    * `clienthost`: 1 function, 3 types and 1 constant.
  * Commands: 22 `make` targets, 15 scripts in `tools/`, `cmd/generate`, and four examples (`agent`, `client`, `claude-code`,
    `gemini`).
* **R2. No serve helper.** The root package has no function that serves an `acp.Agent` over a reader and a writer, so each agent
  writes that loop. The SDK's own example agent (`example/agent/main.go:339`–`:389`) selects on `asc.Done()` or the interrupt and
  returns. It neither waits for the connection's handlers nor reports how the connection ended.
* **R3. `Close` is not graceful.** `Connection.Close` ends every handler's context, and "every later send fails without writing"
  (`connection.go:891`–`:899`; `closeWith`, `:907`–`:913`). Only the end of the input lets running requests finish and their
  answers be written (`drainInput`, `:1287`–`:1309`, bounded by `notificationQueueDrainTimeout`). No exported method cancels the
  handlers and still writes their answers.
* **R4. The specification asks for those answers.** After `session/cancel`, "the Agent **MUST** respond to the original
  `session/prompt` request with the `cancelled` stop reason", and agents "**MUST** catch these errors and return the semantically
  meaningful `cancelled` stop reason, so that Clients can reliably confirm the cancellation" (agent-client-protocol `d6f00fd`,
  `docs/protocol/v1/prompt-turn.mdx:569`, `:576`). An agent that its own shutdown closes answers nothing at all.
* **R5. `agentproc` keeps none of the agent's stderr.** `Cmd.Stderr` is the caller's, and a nil one discards it (F1). Rust's
  `AcpAgent` reports "a bounded stderr tail for a nonzero status" (rust-sdk `644377a`,
  `src/agent-client-protocol/src/acp_agent.rs:518`–`:520`).
* **R6. The taps and the records do not meet.**
  * The taps copy raw bytes (F1).
  * `acptest.Record{Direction, Frame}` is JSON with the keys `direction` (`"sent"` or `"received"`) and `frame`, "for a golden
    file of records" (0041's amendment for phase 13). But `acptest` reads and writes no file.
  * `ValidateGolden` pairs a response with a request recorded before it, in one pass (`acptest/golden.go:32`–`:110`).
  * The two taps are written from two goroutines, and "the two streams have no order between them" (`Command.InputTap`'s doc). So a
    file built from them can hold a response before its request.
* **R7. `testing.TB.ArtifactDir`** exists at the floor, go1.26.10 (`go doc testing.TB`).

#### The consumers now

* **K1. magic-cli-remote** at `3cc402fb`, `master`.
  * It moved from `5d1e7e2c` during the reading; the new commits touch `updateclient` tests and docs.
  * It pins `v1.20.0` (`go.mod:11`). Its staged change moves `golang.org/x/net` only.
  * ACP serves grok alone: `acpagent.New` is called at `internal/provider/grok/grok.go:146`. Its 0214 MADR, `proposed`, plans
    OpenCode over ACP.
* **K2. `<cli-b>`** at `5fb6928`.
  * It is eight commits ahead of its remote's `6ee20140`, none of them touching ACP code.
  * It pins `v1.20.0`.
  * An untracked 0034 pair, `proposed`, plans `v1.21.0`'s constructors, `Kind()` and `ClientRefusals`. It keeps `session.golden`
    byte-identical as its proof (0034 D3).
* **K3. Their open issues.**
  * **magic-cli-remote #28**, "acpagent: adopt acp-go-sdk2 issue 11 Kind() and SessionUpdate constructors", says it is "Blocked on
    an acp-go-sdk2 tag that ships `Kind()`". `v1.21.0` shipped `Kind()` on 2026-10-25, so it now waits only on the consumer's own
    MADR. Part of its premise is out of date: it says elicitation "is still method-not-found until L3". Form elicitation is now
    implemented and advertised (`internal/provider/acpagent/elicitation.go:31`–`:107`, `acpagent.go:1087`–`:1090`); only URL mode's
    `CompleteElicitation` refuses.
  * **`<cli-b>` #5** is the agent-side twin. Its comments record `v1.21.0`'s release and that the adoption waits on its 0034 pair.
  * **`<cli-b>` #3 and #4** concern go-llmprovider-sdk, not this SDK.
  * Neither repository has an open pull request.

#### Findings for item 2: serving an agent

* **S1. `<cli-b>`'s `acpserver.Serve`** (`acpserver/serve.go:28`–`:53`):
  * builds the side connection;
  * selects on `Done` or its context, and closes the connection when the context ends;
  * waits for the handlers within a bound (`handlersDone`);
  * reports a clean end (`ended`, `:58`–`:66`).

  With `v1.25.0`, only the select and the close remain its own. Its options and `agent.Close()` are policy.
* **S2. A second copy** is its harness's `ScriptAgent.Serve` (`acpclient/acptest/script.go:75`–`:91`), used as a serve function by
  three tests.
* **S3. Its signal path loses the answers.**
  * `<cli-b> acp` cancels its context on SIGINT, SIGTERM or SIGHUP (`internal/cli/signal.go:60`–`:89`). `Serve` then closes the
    connection at once (`serve.go:46`–`:52`).
  * By R3, a running `session/prompt` is then never answered, and the client sees only the end of the output.
  * This is inferred from the code and R3; it was not run.
* **S4. Item 1 meets the same gap.** Its step 2 cancels `Serve`'s context with `ErrTerminated`. A serve function that closes its
  connection then, as S1's does, answers nothing either.
* **S5. Go's model is `http.Server.Shutdown`.** It "gracefully shuts down the server without interrupting any active connections
  … If the provided context expires before the shutdown is complete, Shutdown returns the context's error" (go1.27.3,
  `src/net/http/server.go`). Beside it, `Close` does not wait.
* **S6. magic-cli-remote's probe agent** serves through `acp.NewAgentSideConnection(&agent{}, os.Stdout, os.Stdin)`
  (`test/supervisors/probeagent/main.go:143`), as the SDK's example agent does.

#### Findings for item 3: the agent's stderr

* **T1. magic-cli-remote's ACP path keeps none of it.**
  * `cmd.Stderr` is a `slogWriter`, which logs each line at Debug, caps a line, and passes it to a limit detector
    (`internal/provider/acpagent/acpagent.go:493`–`:500`, `:950`–`:993`).
  * A failed `initialize` returns `acp initialize: %w` (`:544`), and an exit `"%s process exited: %v"` (`:608`–`:611`). Neither
    carries what the agent wrote before failing, so the phone shows only the RPC error or the exit status.
* **T2. Its two non-ACP providers each keep a tail of their own,** a `lineRing`:
  * codex's, at `internal/provider/codex/provider.go:1322`, is used at `:873`–`:882` as `initialize: %w; stderr:\n%s`;
  * httpagent's, at `internal/provider/httpagent/provider.go:1206`, "keeps the last max lines for health-failure diagnostics".
* **T3. `agentproc.LineTap` does not fit stderr.** It holds an unended line without a bound (`agentproc/linetap.go`), and it is made
  for the wire.
* **T4. Rust's `AcpAgent`** joins a bounded stderr tail to the error of a nonzero exit (R5).

#### Findings for item 4: transcripts

* **W1. magic-cli-remote's wire capture** (`internal/wirecap/wirecap.go:54`–`:69`) runs when `MCREMOTE_WIRE_CAPTURE_DIR` is set. It
  appends to one `frames.jsonl` per provider, fed by `OutputTap` alone (`acpagent.go:512`–`:517`). The client's requests and
  answers are not captured, and no frame records its direction.
* **W2. Five of its tests scan those files by hand** and decode the frames themselves:
  * `internal/provider/acpagent/initialize_test.go:20`–`:43`;
  * `sessiontitle_test.go:16`–`:50`;
  * `xaicost_test.go:63` and `xaiusage_test.go:71`;
  * `internal/provider/grok/configoptions_fixture_test.go:20`.
* **W3. Its 0214 MADR's gate G1** asks for "One real ACP turn on a free model, recorded": text and thinking, a tool call with a
  permission, a form elicitation, a subagent, and usage (`0214-MADR-run-opencode-v2-over-acp-with-a-lease-bound-http-control-plane.md:563`–`:569`).
  A one-way capture records half of that turn.
* **W4. `<cli-b>`'s goldens** are one line per frame: `client→agent` or `agent→client`, then RFC 8785 canonical JSON
  (`acpclient/acptest/recorder.go:121`–`:133`). Its `Golden` (`:135`–`:162`):
  * rewrites the file when `ACPTEST_UPDATE=1` is set;
  * otherwise writes the actual transcript to `t.ArtifactDir()` and reports the first differing line.
* **W5. `<cli-b>`'s 0016 MADR, D6:** "`acptest` gains `ValidateGolden(t, path)`"
  (`0016-MADR-acp-go-sdk2-v0.15.0-and-stable-surface.md:207`). The SDK's `ValidateGolden` takes records, not a path, and reads
  neither consumer's files.
* **W6. `<cli-b>`'s other test packages define `-update` flags** (`internal/cli/main_test.go:20` and two more). A package that
  defined `-update` when imported would collide with them in any test binary that imports both. `<cli-b>`'s harness uses an
  environment variable instead.
* **W7. RFC 8785 needs `encoding/json/v2`'s `jsontext.Canonicalize`.** This repository keeps off `encoding/json/v2` (its
  `nojsonv2` gate), and `acptest`'s canonical form is v1's
  ([0039-MADR-five-adopter-additions-on-schema-1-25-0.md](0039-MADR-five-adopter-additions-on-schema-1-25-0.md)).

## Decision Drivers

* **D1. Evidence of need.** One consumer of two runs an agent in-process (C1, C5). About 35 lines of its runner remain after `v1.25.0`
  (C6).
* **D2. One lifecycle for both kinds of agent.** C4's own record leaves "in-process vs child transport" to a later choice. One type
  lets a client make that choice without two code paths.
* **D3. Correct shutdown, which is easy to get wrong in-process:**
  * the agent's output must be closed when its serve returns, or the client never sees the end;
  * the connection must read to the end of that output before it closes, as `v1.25.0` fixed for a child (0041's item 1);
  * the agent needs a stop signal beyond its input's end (C3).
* **D4. Go's limits, stated, not hidden:** a goroutine cannot be killed (G2); a panic is not recovered by default (G4).
* **D5. The API is permanent under `v1`** (0021-MADR-resolve-issues-4-and-5-before-v1-14-0.md). It needs more evidence than the
  code it saves.
* **D6. Risk to `agentproc`.** Its stop sequence was hardened over 0027, 0029, 0030 and 0041. A shared type means changing its
  internals.
* **D7. No new dependency** (`AGENTS.md`, "Dependencies").
* **D8. Code the consumers write today (items 2 to 4).** Each item replaces code a consumer, or this repository's own example,
  writes now. Items 2 and 4 serve both consumers; item 3 serves magic-cli-remote, which would otherwise write a third copy (T2).
* **D9. An answer over silence (item 2).** A client that receives `cancelled` can tell a stopped turn from a lost one (R4). An agent
  stopped by its own shutdown should be able to give that answer too.
* **D10. One format from the field to the test (item 4).** What a client captures in use should be what its tests read, validate
  and compare.

## Considered Options

* **O1.** Nothing new; document the pattern. A README section and an example: two `io.Pipe`s, the serve goroutine, and a bounded
  close using `WaitContext` and `EndedCleanly`. 0035 F and 0041 stand.
* **O2.** `agentproc.InProcess`, a serve function run on a goroutine, whose `Start` returns the same `*agentproc.Process` as
  `Command.Start`, with the same ladder mapped onto a goroutine.
* **O3.** The same runner with a type of its own, `*agentproc.Running` or a new package, sharing nothing with `Process`.
* **O4.** A connected pair only: `acp.Pipe(agent, client, opts...)` returning both side connections, with no lifecycle.
* **O5.** No in-process runner: the client runs its own binary's stdio mode as a child, through `agentproc.Command`, with
  `os.Executable()`. A documented pattern.

These options are item 1's. Each of items 2 to 4 sets out its own alternatives in its questions, under "Items 2 to 4, added
2026-10-28".

## Decision Outcome

Chosen option: "O2", because it is the only option that gives D2, one lifecycle for both kinds of agent. That is the one benefit
a consumer cannot get by writing its own 35 lines (C6). O2 also covers D3's three shutdown rules, and states D4's limits as API. The
owner judged one consumer enough evidence for the type (question 1); had they not, the outcome would have been O1.

### The design

```go
// InProcess is an agent to run in this process, on a goroutine, connected
// by pipes: the agent a Go program serves itself, with the lifecycle a
// child process has.
type InProcess struct {
    // Serve runs the agent over in and out until in ends, as a
    // program's main serves os.Stdin and os.Stdout.
    Serve func(ctx context.Context, in io.Reader, out io.Writer) error

    Grace             time.Duration         // as Command's
    ConnectionOptions []acp.ConnectionOption // as Command's
    OutputTap         io.Writer              // as Command's
    InputTap          io.Writer              // as Command's
}

func (a *InProcess) Start(client acp.Client) (*Process, error)

// ErrAbandoned is joined to Wait's error when Serve did not return
// within the grace after its context was cancelled and its pipes closed.
var ErrAbandoned = errors.New("agentproc: in-process agent abandoned")
```

How each step of `Command`'s lifecycle maps onto a goroutine:

| Step | `Command`, a child process | `InProcess`, a goroutine |
| :--- | :--- | :--- |
| Start | `exec.Cmd.Start` on a locked thread, in a group or job | `go Serve(ctx, in, out)`, its context the runner's own, cancelled with a cause |
| The agent's own end | `Cmd.Wait` returns; the group or job is ended | `Serve` returns; the agent's output is closed, so the client reads `io.EOF` as it does at a child's exit, and `Serve`'s error is `Wait`'s; the agent's input is closed too, so the client's writes fail as they do to a child that has exited, where an `io.Pipe` with no reader would block them; a goroutine of the agent's still writing fails |
| Then | the connection reads to the end of the output, up to the grace, and closes | the same |
| Close, step 1 | close stdin; wait up to the grace | close the agent's input, so its reads return `io.EOF`; wait up to the grace for `Serve` |
| Close, step 2 | SIGTERM to the group (Unix); wait up to the grace | cancel `Serve`'s context with the cause `ErrTerminated`; wait up to the grace |
| Close, step 3 | kill the group or job; `WaitDelay` bounds the I/O | close both of the agent's pipe ends with `ErrKilled`, so its blocked reads and writes return; wait up to the grace; if `Serve` has still not returned, stop waiting and join `ErrAbandoned` (question 4) |
| Kill | the steps without their waits | the context cancelled and the pipes closed with `ErrKilled` at once, then step 3's bound |
| `Wait`'s error | the exit status, joined with `ErrTerminated` or `ErrKilled` | `Serve`'s error, joined the same way, or with `ErrAbandoned` |
| `ExitedFirst` | the exit was seen while the connection was open | `Serve` returned while the connection was open |
| Taps | over the `*os.File` pipes | the same taps, over the pipes question 3 picks |
| This process's death | the job, the Pdeathsig, or the input's end | nothing needed: the goroutine ends with the process |

What does not carry over, and is not offered: process groups and jobs, `Pdeathsig`, `Stderr` (the agent logs through its own
logger), and an exit status.

The handshake stays the client's, as with `Command`: `Start` sends no `initialize`. `<cli-b>`'s refusal wording and its connection
options (C2) stay its own.

The shape, as the owner's answers settled it:

* **A1.** The type is `agentproc.InProcess`, and its `Start` returns `*agentproc.Process` (question 2).
* **A2.** `Process`'s child-specific parts move behind an unexported seam inside `agentproc`: one implementation for `exec.Cmd`, one
  for a serve function. Its exported API is unchanged, and `make api` lists only `InProcess`, its method and `ErrAbandoned`.
* **A3.** The pipes are `io.Pipe`s (question 3), and the taps read an `io.Reader`, not an `*os.File`.
* **A4.** A `Serve` that ignores its context and its closed pipes is abandoned after step 3's bound (question 4). `Done` closes and
  `Wait` returns, with `ErrAbandoned` joined. The goroutine stays running, as Go allows nothing else (G2). The doc says so, and
  that the agent's code holds it.
* **A5.** A panic in `Serve` is not recovered (question 5). It ends the program, as a panic on any goroutine does, with its stack
  intact (G4). A client that needs isolation from its agent's faults runs it as a child (O5's pattern).
* **A6.** `Serve`'s context derives from `context.Background()`, cancelled by step 2 or by `Kill`, with the cause (question 6).
* **A7.** No platform code. The runner builds for every target `agentproc` does, and needs no `tools/platforms.py` change.

### Questions for the owner

1. **Is one consumer enough evidence?**
   * Recommended: yes, for O2. The type is small beside `Command`, and D2 is a property neither consumer can build alone.
   * Alternative: no; O1, the documented pattern, and revisit when a second consumer embeds an agent.
2. **A shared type, or its own?**
   * Recommended: `Start` returns `*agentproc.Process` (O2), so a client holds one type whichever it starts. The cost is A2's
     refactor of `Process`'s internals (D6), held by the existing tests unchanged.
   * Alternative: a type of its own (O3). It leaves `Process` untouched, but a client needs an interface of its own to treat both
     alike, and the type gives little over C6's 35 lines.
3. **Which pipes?**
   * (Corrected 2026-10-28, "Amended by the PLAN's phase 2": the cause reaches a blocked read or write only from the pipe's other
    end.) Recommended: `io.Pipe`. It uses no file descriptors, closes each direction on its own, and carries `ErrKilled` to a blocked read
     or write (G1). It is what `acptest` and `<cli-b>` use. The cost is the taps' reader generalized from `*os.File`.
   * Alternative: `os.Pipe`, byte for byte a child's pipes, with kernel buffering. It reuses the taps unchanged, but costs four
     descriptors per agent, and a close cannot carry a cause.
4. **A `Serve` that never returns.**
   * Recommended: abandon it after step 3's bound, with `ErrAbandoned` (A4). `Close` stays bounded, as F2 promises, and the error
     says what was left, as `WaitDelay`'s `ErrWaitDelay` does (G6) and `acptest.Connect`'s cleanup does (F5).
   * Alternative: `Wait` blocks until `Serve` returns, as `httptest.Server.Close` does (M5). Nothing leaks unreported, but `Close`
     can hang forever on the agent's bug.
5. **A panic in `Serve`.**
   * Recommended: not recovered (A5), as `errgroup` and `sync.WaitGroup.Go` decline to (G4).
   * Alternative: recovered into an error with its stack, the pipes closed, and `Wait` returning it, as net/http does with a
     handler. The client survives its agent's bug, but the state the panic left may be corrupt, and crash tooling never sees it.
6. **`Serve`'s context.**
   * Recommended: from `context.Background()`, as `Command.Start` takes no context (A6).
   * Alternative: `StartContext(ctx, client)`, with `context.WithoutCancel(ctx)`, so the agent inherits the context's values, such
     as a logger or a trace, as C2 does.
7. **The name.**
   * Recommended: `InProcess`, read at the call site as `(&agentproc.InProcess{Serve: serve}).Start(client)`.
   * Alternatives: `Func`, `Serve`, `Goroutine`.

#### Decided with the owner, 2026-10-28

Each question was put with three options; the owner chose the recommended one each time.

| Question | Answer | The other two options offered |
| :--- | :--- | :--- |
| 1. Evidence | Yes: build O2 | O1, the pattern documented; O5, the self-exec pattern documented |
| 2. The type | `Start` returns the shared `*agentproc.Process` | a type of its own in `agentproc`; a type of its own in a new package |
| 3. The pipes | `io.Pipe` | `os.Pipe`; `net.Pipe`, whose `Close` cannot end the agent's input alone |
| 4. A `Serve` that never returns | abandoned after step 3's bound, with `ErrAbandoned`; `Done` closes and `Wait` returns | `Wait` blocks until `Serve` returns; `Wait` returns but `Done` stays open until `Serve` returns |
| 5. Panics | not recovered | always recovered into an error; an opt-in field, off by default |
| 6. `Serve`'s context | from `context.Background()`, through `Start(client)` | `StartContext(ctx, client)` with `context.WithoutCancel`; `Start(ctx, client)` alone |
| 7. The name | `InProcess` | `Func`; `Goroutine` |

The design above is the decision. Its PLAN is
[0042-PLAN-in-process-agent-runner.md](0042-PLAN-in-process-agent-runner.md).

### Items 2 to 4, added 2026-10-28

Each item states its design, the code it replaces, and the questions put to the owner, each with its recommendation first. The
owner answered them on 2026-10-28 ("Decided with the owner, items 2 to 4, 2026-10-28"), and each design follows the answers.

#### Item 2. Serve an agent, and stop it gracefully

**Add, in the root package:**

```go
// Shutdown ends the connection gracefully. It ends every handler's
// context with ErrShutdown, and keeps writing until every request
// received before it has been answered; then it closes the connection.
// A request received after it is answered at once with -32603, and the
// connection keeps reading, so the answers to the agent's own requests
// still arrive. When ctx ends first, it closes at once and returns
// context.Cause(ctx).
func (c *Connection) Shutdown(ctx context.Context) error

// ErrShutdown is the cause a handler's context ends with under Shutdown.
var ErrShutdown = errors.New("acp: connection shut down")

// ServeAgent serves agent over in and out until in ends or ctx ends.
// At in's end it lets the running requests be answered, as the
// connection does now; at ctx's end it shuts the connection down within
// 5 s, the bound the connection gives its input's drain. It returns nil
// at a clean end, the connection's error at another, and
// context.Cause(ctx), joined with Shutdown's error, when ctx ended it.
func ServeAgent(ctx context.Context, agent Agent, in io.Reader, out io.Writer, opts ...ConnectionOption) error
```

* A prompt handler that honours its context answers `cancelled`, as it does on `session/cancel` (R4). The cause it reads is
  `ErrShutdown`, so it can tell a shutdown from a client's cancel.
* `ServeAgent` waits on the caller's goroutine and starts none, so F6's rule holds.
* A side connection shuts down through `Connection()`, as `WaitContext` does, since a side connection's methods that take a context
  are protocol methods (0041's amendment for phase 6).
* The agent's own cleanup, S1's `agent.Close()`, stays the caller's, after `ServeAgent` returns.

**What it replaces:**
* S1's select, close and end check, about 25 lines once `<cli-b>` is on `v1.25.0`;
* S2's copy in its harness;
* the SDK's own example loop (R2), and probe agents like S6's.

It also gives item 1's step 2 the answer it lacks (S4): a `Serve` built on `ServeAgent` answers its running prompts `cancelled`, and
returns.

**Questions:**
* **2a. `ServeAgent`'s bound at the context's end.**
  * Recommended: the bound the connection already gives its input's drain, 5 s, documented.
  * Alternative: a connection option, `WithShutdownGrace(d)`.
  * Alternative: a second context, `ServeAgent(ctx, stop, …)`, whose end bounds the shutdown that `ctx`'s end starts.
* **2b. A request received during `Shutdown`.**
  * Recommended: answered at once with an internal error, `-32603`, whose data says the agent is shutting down. The connection
    keeps reading, so the answers to the agent's own requests still arrive.
  * Alternative: answered `-32800`, request cancelled.
  * Alternative: not read. The connection stops reading, so the client's call fails only when the connection closes, and the
    answers to the agent's own requests are lost.
* **2c. The name.**
  * Recommended: `ServeAgent` and `Shutdown`, after `http.Server.Shutdown` (S5).
  * Alternative: `Serve` and `Shutdown`.
  * Alternative: `RunAgent` and `Drain`.

#### Item 3. Keep the end of the agent's stderr

**Add, in `agentproc`:**

```go
type Command struct {
    // … the existing fields …

    // StderrTail and StderrTailLines, when either is positive, keep the
    // end of what the agent writes to its standard error: at most
    // StderrTail bytes and at most StderrTailLines lines, a cap of zero
    // being no cap. Cmd.Stderr, when set, still receives every byte.
    StderrTail      int
    StderrTailLines int

    // JoinStderrTail joins a *StderrError holding the tail to Wait's
    // error, when that error is not nil and the tail is not empty.
    JoinStderrTail bool
}

// StderrTail returns the end of the agent's standard error that the
// Command's caps kept, final once Wait has returned.
func (p *Process) StderrTail() string

// StderrError is the end of the agent's standard error, joined to Wait's
// error under Command.JoinStderrTail.
type StderrError struct{ Tail string }

func (e *StderrError) Error() string
```

* With either cap set, `Start` wraps `Cmd.Stderr`. A nil `Stderr` is still discarded, but its tail is kept. Through the wrapper,
  `os/exec` copies the agent's stderr through a pipe and a goroutine, which `WaitDelay` bounds; `Start` already sets that to half
  the grace (0027, S1).
* The tail starts at a line's beginning. A line longer than the byte cap keeps its end, so the byte cap bounds the memory exactly,
  whatever the lines.
* Under `JoinStderrTail`, `Wait`'s error is the existing error with the `*StderrError` joined. `errors.Is(err, ErrKilled)` and
  `errors.As(err, &exitErr)` hold as before, and `errors.As(err, &stderrErr)` gives the tail. Like the rest of `Wait`'s error, it is
  the same on every call.
* An in-process agent (item 1) has no stderr. Its `StderrTail` is empty, and nothing is joined.

**What it replaces:** the third copy magic-cli-remote would write for its ACP path (T1, T2). Its `initialize` failure and its exit
message can then carry what the agent said, as its codex path's do.

**Questions:**
* **3a. Joined to `Wait`'s error?**
  * Recommended: no. A method only, and `Wait`'s error stays `exec`'s with the sentinels. The caller adds the tail where it
    reports, as codex does at `:881`.
  * Alternative: on a nonzero exit, `Wait`'s error has a `*StderrError` holding the tail joined, as Rust does (T4).
  * Alternative: both, the join opted into by a field.
* **3b. The unit.**
  * Recommended: bytes, cut to whole lines.
  * Alternative: lines, as `lineRing` counts them (T2).
  * Alternative: both, a byte cap and a line cap.

#### Item 4. One transcript format, from the field to the test

**Add, in `agentproc`,** since production code cannot import `acptest`, which imports `testing`:

```go
// Transcript writes the frames of both taps to w, one JSON object a line:
// {"direction":"sent"|"received","frame":<the frame>}, the form
// acptest.Record reads.
func NewTranscript(w io.Writer) *Transcript
func (t *Transcript) Input() io.Writer  // Command.InputTap: what the client sent, "sent"
func (t *Transcript) Output() io.Writer // Command.OutputTap: what the agent sent, "received"
func (t *Transcript) Close() error
```

* Lines are joined across writes, as `LineTap` joins them.
* A line that is not JSON is written with its frame as a JSON string, so the file stays one object a line.
* Its writes are serialized. The order between the two directions is the taps', which none is guaranteed (R6).
* Redaction stays the caller's: it wraps `w`.

**Add, in `acptest`:**

```go
func ReadRecords(r io.Reader) ([]Record, error)
func WriteRecords(w io.Writer, records []Record) error

// Golden compares records with the file at path, in ReadRecords' form.
// With ACPTEST_UPDATE=1 set it writes path instead; on a mismatch it writes
// the actual records to tb.ArtifactDir() and reports the first record that
// differs.
func Golden(tb testing.TB, path string, records []Record)

// Decode returns the params of each record that went way d with method,
// decoded into T.
func Decode[T any](records []Record, d Direction, method string) ([]T, error)
```

* `ValidateGolden` pairs a response with its request wherever the request is in the records. A transcript built from two taps then
  validates. This is a compatible relaxation: it accepts more than before, and refuses nothing it accepted.
* The canonical form stays `encoding/json` v1's (W7).

**Add, in `acptest`, a replay agent** (question 4c):

```go
// ReplayAgent plays the agent's side of a recorded session to a client
// under test. Serve is a serve function, for agentproc.InProcess or
// ConnectReplay.
type ReplayAgent struct {
    Records []Record
}

func (r *ReplayAgent) Serve(ctx context.Context, in io.Reader, out io.Writer) error

// ConnectReplay joins client to a ReplayAgent of records over two pipes,
// closes both when the test ends as Connect does, and returns the
// client's side.
func ConnectReplay(tb testing.TB, records []Record, client acp.Client, opts ...acp.ConnectionOption) *acp.ClientSideConnection
```

* **Its rule.** The received records, the agent's frames, are played in their recorded order:
  * a notification, or a request of the agent's own, is sent when its turn comes. The agent's request takes a fresh id, and the
    client's answer to it is checked as JSON-RPC only;
  * a recorded response waits for the client's matching request: the k-th request of a method is answered with the recorded
    answer to the k-th request of that method, under the live id.
* **Drift is reported, not hidden.**
  * A client request with no recorded answer left is answered `-32603`, naming the method.
  * At its input's end, `Serve` returns an error listing each such request and each recorded frame not played, so a test sees how
    the client departed from the recording.
* **It replays a transcript from two taps.** It matches by method and by order within one direction, so the missing order between
  the two directions (R6) does not matter.
* **Its limits are documented:** it does not reproduce timing, and a client that branches on what it receives meets the recorded
  content whatever it asked.

**What it replaces:**
* in magic-cli-remote:
  * the writer of its wire capture (W1), about 45 lines;
  * its five scanners (W2), with `Decode`;
  * and it gives 0214's G1 both directions (W3);
* in `<cli-b>`:
  * its `Golden`, `Transcript` and `firstDiff` (W4), about 90 lines;
  * and it meets its 0016 D6 (W5), as `ValidateGolden(t, records)` over `ReadRecords`.
* With item 1, `<cli-b>`'s in-process agent's taps can write the same transcript.

The cost: `<cli-b>`'s two goldens are rewritten once, into the record form and v1's canonical JSON. Its 0034 D3, which keeps
`session.golden` byte-identical, would record that.

**Questions:**
* **4a. Where the writer lives.**
  * Recommended: `agentproc`, beside the taps it is set as.
  * Alternative: the root package, over any pair of writers, so that a client without `agentproc`, `acptest.Connect`'s pipes
    included, can use it.
  * Alternative: a small new package, `transcript`.
* **4b. How `Golden` is told to update.**
  * Recommended: the environment variable `ACPTEST_UPDATE=1`, as `<cli-b>`'s harness does, since a flag can collide (W6).
  * Alternative: a namespaced flag, `-acptest.update`, which every test binary importing `acptest` then carries.
  * Alternative: no update; `Golden` compares only, and the caller writes the file with `WriteRecords`.
* **4c. A replay agent.**
  * Recommended: not now. `Decode` covers W2's need, and replay is policy: which recorded answer meets which request.
  * Alternative: `acptest.ReplayAgent`, which answers each request with the recorded answer to the same method, in order, and
    sends the recorded notifications between them.
  * Alternative: a replay client, the other side's equivalent, for an agent's tests.

#### Decided with the owner, items 2 to 4, 2026-10-28

Each question was put with three options. The owner chose the recommended one for five, and an alternative for three: 3a, 3b and
4c.

| Question | Answer | The other two options offered |
| :--- | :--- | :--- |
| 2a. `ServeAgent`'s bound | the drain's bound, 5 s, documented | `WithShutdownGrace(d)`; a second context |
| 2b. A request during `Shutdown` | answered `-32603` at once; the connection keeps reading | answered `-32800`; not read |
| 2c. The names | `ServeAgent` and `Shutdown` | `Serve` and `Shutdown`; `RunAgent` and `Drain` |
| 3a. The tail and `Wait`'s error | both: `StderrTail()`, and a `*StderrError` joined under the opt-in `JoinStderrTail` | the method only (recommended); joined on every nonzero exit |
| 3b. The unit | both caps: `StderrTail` bytes and `StderrTailLines` lines | bytes cut to lines (recommended); lines only |
| 4a. The writer's home | `agentproc` | the root package; a new package |
| 4b. `Golden`'s update | `ACPTEST_UPDATE=1` | a namespaced flag; no update |
| 4c. A replay agent | `acptest.ReplayAgent`, with `ConnectReplay` | not now (recommended); a replay client |

The designs above follow these answers. All four items are decided, so this record is `accepted`. The PLAN,
[0042-PLAN-in-process-agent-runner.md](0042-PLAN-in-process-agent-runner.md), carries all four.

#### Amended by the PLAN's phase 2, 2026-10-28

The PLAN's probes, run on macOS and on the Linux and Windows hosts, confirmed the mapping table and the reproduction (the PLAN's
phase 2 entry). They corrected one statement of fact, and changed no decision.

* **Which end carries the cause.** Question 3's recommendation says `io.Pipe` "carries `ErrKilled` to a blocked read or write".
  The probe P5 found that a read or write blocked on a pipe end returns:
  * the error given to `CloseWithError` when the **other** end is closed with it;
  * `io.ErrClosedPipe` when its **own** end is closed, whatever the error given.

  Step 3 closes the agent's own ends, so the agent's blocked reads and writes return `io.ErrClosedPipe`, and the step still
  unblocks them, as the table says. `ErrKilled` reaches the agent through its context's cause, which step 3 cancels with it first.
  On the client's side, the agent's output is closed with a plain `Close`, so the client reads `io.EOF` as at a child's exit.
* **Two consequences for item 1's design, which the PLAN's phase 4 follows:**
  * a connection whose writer fails does not end: P1's client stayed open 2 s after the pipe it wrote to lost its reader. So
    when `Serve` returns, the runner closes the agent's output, which ends the client's connection (P3), and not only its input;
  * a write to an `io.Pipe` whose reader is never closed blocks for ever. P4's first run overflowed the client's notification
    queue of 1024, the client closed its connection, and the agent's next write blocked, since the client's reader was wrapped in
    a type that does not close. So the runner closes its pipe ends itself whenever the connection ends, and does not rely on the
    connection's `Close` reaching them.

#### Amended 2026-10-30, issue #12: a bounded close, a testable handlers error, and `Wait`'s error said once

**Status of this amendment: proposed.** The owner set its direction on 2026-10-30: the issue's second option, a bounded close beside
`Close` and `Kill`; this record amended, rather than a new one; an error `errors.Is` can test for `ServeAgent`'s handlers; and the
repeated text in `Wait`'s error scoped and fixed. The choices below that the owner did not name are this amendment's, each with its
alternative, for the owner to accept with the PLAN's phases 12 to 17.

##### Findings, 2026-10-30

Read at `03b57e9` (`v1.27.0` and its records), and reproduced in scratch copies of the tree; nothing in the tree changed.

* **I1. The issue.** [#12](https://github.com/maccavelli/acp-go-sdk2/issues/12), filed 2026-10-30: "`Process.Close` waits without
  bound for a client handler that never returns (v1.26.0)". `<cli-b>` held its move to `InProcess` on it: its 0035 PLAN,
  deviation 9, read on GitHub at `6c5eb02`, waits for "a tag [that] carries one of those", the issue's three asks. It pins `v1.26.0`
  (`go.mod:16`).
* **I2. Why `Close` waits without bound.**
  * `Close` is `p.closeOnce.Do(p.stop)` then `p.Wait()`, and `Kill` the same after `hurry` (`agentproc/agentproc.go:516`–`:519`,
    `:531`–`:535`).
  * `Wait` waits for the agent (`<-p.done`), the watcher and the monitor, then the connection, `_ = p.conn.Wait()` (`:389`), then
    drains the taps (`:390`).
  * `Connection.Wait` waits for every goroutine the connection started, "its handlers' included", and says that "a handler that
    ignores its context" keeps it waiting (`connection.go:858`–`:872`), as
    [0026-MADR-connection-end-cause-and-wait.md](0026-MADR-connection-end-cause-and-wait.md) decided (questions 4 and 5).
  * A client's notification handlers, `SessionUpdate` among them, run one at a time on the connection's `processNotifications`
    goroutine (`connection.go:1748`–`:1768`). The watcher's close of the connection ends their context
    (`closeWith`, `:966`–`:988`); one that ignores it holds that goroutine, and so every `Wait` above it.
* **I3. Reproduced, grace 300 ms.** A `ScriptAgent` served in process sends one update, and the client's `SessionUpdate` blocks until
  released:
  * `Close` had not returned 10 s later; `Done` was closed, the agent ended. Released, `Close` returned `nil` within 1.5 ms. Its
    error never says that a handler held it.
  * The same with a child, the test binary's `burst` mode: not returned after 5 s; `nil` once released.
  * `go p.Close()` then `p.WaitContext(ctx)` returned `context.DeadlineExceeded` at its bound, but lost `Close`'s error and left two
    goroutines blocked until the handler returned. `<cli-b>`'s deviation 9 declined to bound the SDK's close itself so.
* **I4. The records claim otherwise.** "every wait bounded": `README.md:682`; the 0027 MADR, Consequences
  ([0027-MADR-agent-process-helper.md](0027-MADR-agent-process-helper.md)); this record's Consequences, "the shutdown rules a child
  needed, the output read to its end and every wait bounded, hold in-process too". Neither `Process.Close`'s doc nor `Process.Wait`'s
  names the handlers' unbounded wait; `Connection.Wait`'s does.
* **I5. This record's own finding was not carried into the design.** C2 records that `<cli-b>`'s runner bounds its handlers' wait
  with `closeTimeout`, and C6 that `WaitContext` replaces its helper. `InProcess` returns a `*Process`, whose `Close` never uses
  `WaitContext`, so that bound was lost.
* **I6. The agent's side is already bounded.** `ServeAgent` waits at most 5 s for its connection's handlers at its input's end, and
  then returns `fmt.Errorf("acp: the connection's handlers still run %v after its end", notificationQueueDrainTimeout)`
  (`serve.go:33`–`:38`): a text, which `errors.Is` cannot test. No test reaches that branch: `TestServeAgentInputEnd` and
  `TestServeAgentContextEnd` (`shutdown_test.go:275`, `:301`) do not.
* **I7. Bounding `Wait` itself costs a guarantee.** A scratch prototype bounded `Wait`'s connection wait by the grace and joined an
  error past it. It met the issue's case: `Close` returned after 0.30 s, the same error again, and a handler that returned within the
  grace was waited for. Under the whole suite's load, `TestNoLostOutput` then failed: "Wait = agentproc: the connection's handlers
  still run, want nil". Nothing hung: the client was still handling 1000 queued updates when 300 ms passed. Alone, the test passed
  10 times of 10, about 0.17 s each, in the tree and in the prototype. A bound cannot tell a busy client from a hung one.
* **I8. A bound beside `Close` costs nothing else.** A second prototype added `CloseContext` and `KillContext`, Close's and Kill's
  steps then `WaitContext`. Both returned `context.DeadlineExceeded` 0.30 s into a 300 ms bound, the agent ended; with a handler that
  returned within the bound, `CloseContext` waited and returned `nil`. The whole `agentproc` suite passed under `-race`,
  `TestNoLostOutput` among it, since `Wait` is unchanged.
* **I9. `Wait`'s error repeats its cause.** `Wait` joins the step's sentinel as `fmt.Errorf("%w: %w", p.ended, p.waitErr)`
  (`agentproc.go:401`), whether or not the agent's error already wraps it. An `InProcess` agent's context ends with that sentinel as
  its cause (`inprocess.go:121`–`:138`), and `InProcess.Serve`'s doc says `context.Cause` reports it; a `Serve` that returns it,
  `ServeAgent` among them, gives it twice. On the unmodified tree:

  | The agent's end | `Wait`'s error |
  | :--- | :--- |
  | `Kill`, `Serve` returning `context.Cause(ctx)` | `agentproc: agent killed: agentproc: agent killed` |
  | `Close`, step 2, the same `Serve` | `agentproc: agent terminated: agentproc: agent terminated` |
  | `Close`, step 2, `Serve` returning `fmt.Errorf("serve: %w", context.Cause(ctx))` | `agentproc: agent terminated: serve: agentproc: agent terminated` |
  | a child, the `stubborn` mode, killed (the control) | `agentproc: agent killed: signal: killed` |

  `errors.Is` holds in each; only the text repeats. No test, example or doc pins either text.

##### Decided

* **X1. `CloseContext` and `KillContext`.** `Close` and `Kill` bounded by a context, after `WaitContext` beside `Wait` (0041, item 6)
  and `Shutdown(ctx)` beside `Close` (item 2). `Wait`, `Close`, `Kill` and `WaitContext` are unchanged: a `Wait` that returns still
  means that every handler the connection ran has returned (I7).
* **X2. `acp.ErrHandlersRunning`,** in the root package beside `ErrShutdown`:
  * `ServeAgent` wraps it at its input's end, its text unchanged, so `errors.Is` can test what a text said;
  * `ServeAgent` wraps it too at its context's end, when `Shutdown`'s 5 s pass with requests unanswered, whose handlers still run;
  * `CloseContext` and `KillContext` join it when their context ends while `Wait` waits on the connection's goroutines, the agent
    ended and the connection closed.
* **X3. `Wait`'s error says its cause once.** When the agent's error already wraps the step's sentinel, `Wait` returns the agent's
  error as it is; otherwise it joins the sentinel as before. `errors.Is` and `errors.As` give what they gave.
* **X4. The docs say what is bounded.** The README, `Process.Close`'s, `Kill`'s and `Wait`'s docs, and `docs/architecture.md` name
  the handlers' unbounded wait and the methods that bound it. I4's claims in the 0027 MADR and here are annotated in place.

##### The specification

```go
// In the root package, errors.go.

// ErrHandlersRunning is why a wait for a connection's handlers ended before
// they had returned: a handler that ignores its context, or one still at
// work, holds them. ServeAgent wraps it when its bound passes with handlers
// still running, and agentproc's CloseContext and KillContext join it when
// their context ends with the connection's handlers still running; read
// with errors.Is.
var ErrHandlersRunning = errors.New("acp: the connection's handlers still run")
```

```go
// In agentproc.

// CloseContext is Close bounded by ctx. It runs Close's steps and returns
// Wait's error once Wait would return. Should ctx end during the steps, the
// steps left run as Kill's do: no more waiting, no SIGTERM, the kill. Should
// it end before Wait would return, CloseContext returns context.Cause(ctx),
// joined with acp.ErrHandlersRunning when the connection's handlers are what
// still runs, and with the agent's error, final once the agent has ended, as
// Wait gives it. The agent has ended by then, and Done is closed; what still
// runs is left to finish, and Wait, or WaitContext, returns the whole error
// once it has. CloseContext returns at most the kill's own bound, a grace,
// after ctx ends. When ctx ends, a Close in progress elsewhere is cut short
// too, as Kill cuts it.
func (p *Process) CloseContext(ctx context.Context) error

// KillContext is Kill bounded by ctx, as CloseContext is Close: Kill's steps,
// then Wait's error, or, should ctx end first, context.Cause(ctx) joined as
// CloseContext's is.
func (p *Process) KillContext(ctx context.Context) error
```

`Wait`'s doc gains: "When the agent's error already wraps that sentinel, as a `Serve` that returns its context's cause does, it is
returned as it is. `Wait` waits for the connection's goroutines without bound: a client handler that ignores its context holds it,
and `Close` and `Kill`, until the handler returns. `CloseContext` and `KillContext` bound that wait." `Close`'s and `Kill`'s docs
point to their bounded forms. `ServeAgent`'s doc names `ErrHandlersRunning` at both ends.

How the parts work, for the PLAN:

* **The agent's error** is `Wait`'s composition without the waits: the step's sentinel and the agent's own error (X3), and the
  stderr tail under `JoinStderrTail`. Factored out, `Wait` and the bounded methods give the same text. It is final once `Done` is
  closed: the watcher records it under the lock before it closes `Done`, and Close's steps return only once `Done` is closed.
* **The bound** reuses `WaitContext`'s one goroutine (0041, item 6), so a call starts none of its own.
* **The steps cut short** by a context run `Kill`'s `hurry`, through `context.AfterFunc` registered for the steps alone, so a
  context that ends once the steps are done changes nothing in the process.
* **"The handlers still run"** is read from where `Wait` is, not guessed: `Wait` marks the span in which it waits on the connection,
  after the watcher has closed it. A context that ends earlier, while the connection reads to the end of the agent's output, or later,
  while the taps drain, joins only its cause; each of those waits is bounded by the grace already.

##### Alternatives considered

* **For X1:**
  * `Wait` bounded by the grace, the issue's first option: rejected on I7, since it changes every caller's `Wait` and reports a
    busy client as a hung one.
  * The docs alone, the issue's third option: they state the problem without solving it; `<cli-b>` asked for a library bound (I1).
  * `ctx` bounding the wait alone, the steps left to the grace: a `CloseContext` whose context has ended would still wait up to two
    or three graces, where `Shutdown(ctx)` closes at once. Not chosen.
  * `context.Cause(ctx)` alone, as `WaitContext` returns: the agent's error would be lost exactly when a handler hangs, since `Wait`
    cannot be called then. `<cli-b>`'s close returns both today (C2).
* **For X2:**
  * The sentinel in `agentproc`: `ServeAgent`, in the root package, could not wrap it, and a caller would test two errors.
  * `WaitContext` joining it too: rejected, since `WaitContext` is released and returns `context.Cause(ctx)`, which a caller may
    compare with `==`; joining would break that comparison.
  * The sentinel joined whenever the context ends after the steps: rejected, since the output's read and the taps' drain would then
    be reported as handlers.
* **For X3:**
  * The duplicate stripped in `InProcess` alone, where it arises: a `Command` whose agent's error could wrap the sentinel would keep
    it, and the rule belongs to `Wait`, which composes the error.
  * Leaving it: the text, which logs show, misleads, and a wrapping `Serve` doubles a word of its own.

##### Consequences of the amendment

* Good, because a client bounds its close, child or in-process, with one call, and learns what a hung handler left behind, which
  `<cli-b>` needs to move to `InProcess` (I1).
* Good, because `ServeAgent`'s and the bounded close's handler errors are one sentinel that `errors.Is` reads.
* Good, because `Wait` keeps its meaning, every received message handled, and `TestNoLostOutput` its guarantee (I7).
* Good, because `Wait`'s error reads once per cause.
* Neutral, because `Close` and `Kill` still wait without bound for a hung handler. Their docs say so, and point to the bounded forms.
* Neutral, because a bounded method that returns leaves the handler's goroutine, and `WaitContext`'s, running until the handler
  returns, as `ErrAbandoned` leaves `Serve`'s. Go cannot stop a goroutine (G2).
* Bad, because `Process` gains two methods where one rule could have served, and a caller must choose the bounded form.
* Bad, because `ServeAgent`'s error at its context's end gains the sentinel's text when `Shutdown`'s bound passes; a caller that
  matched that text sees more of it.
* Bad, because `Wait`'s error text changes for an agent whose error wraps the sentinel; `errors.Is` and `errors.As` are unchanged.

##### Confirmation of the amendment

The PLAN's phases 12 to 17 show, each new check first seen to fail on a planted break in a copy of the tree:

* `CloseContext` and `KillContext`, for a child and an in-process agent with a client handler that ignores its context, return
  within their bound and the slack, with `errors.Is` true for the bound's cause and `acp.ErrHandlersRunning`, the agent's error
  joined, and `Done` closed; `Wait` then returns, once the handler is released, `Close`'s error;
* with a handler that returns within the bound, they return `Wait`'s error, and nothing is joined;
* a context that ends during the steps hurries them: a `stubborn` child under a 5 s grace is killed, without SIGTERM, near the
  context's end;
* a cause given with `context.WithTimeoutCause` is the one returned;
* no goroutine is left once the handler returns, under `agentproc`'s leak guard;
* `ServeAgent`'s error at its input's end wraps `ErrHandlersRunning`, its text unchanged, in a `synctest` bubble; and at its
  context's end, when `Shutdown`'s bound passes;
* `Wait`'s error for each row of I9's table says its cause once, and `errors.Is` holds; a child's is unchanged;
* `make api`: `ErrHandlersRunning`, `(*Process).CloseContext` and `(*Process).KillContext` added, nothing else;
* `tools/consumer.py` closing an in-process agent through `CloseContext`.

### Consequences

* Good, because a client that serves its own agent gets the lifecycle and the taps a child has, and can switch between the two kinds
  without changing its code (D2, C4).
* Good, because the agent's serve function gets a stop signal beyond its input's end, a cancelled context with a cause, which C3's
  agent never gets.
* Good, because the shutdown rules a child needed, the output read to its end and every wait bounded, hold in-process too (D3).
  (Corrected 2026-10-30: every wait of the agent's is bounded, but `Wait`, and so `Close` and `Kill`, waits without bound for a
  client handler that ignores its context, in-process and for a child alike; see "Amended 2026-10-30, issue #12", I2 to I4.)
* Good, because no dependency is added and no platform code is written (D7, A7).
* Neutral, because `<cli-b>`'s own runner shrinks by about 35 lines, and its handshake and options stay its own.
* Bad, because `Process`'s internals change behind a seam (D6), in code whose stop sequence four records hardened.
* Bad, because two documented behaviours have no counterpart in a child: an abandoned goroutine (A4), and a panic that ends the
  client (A5).
* Bad, because a permanent type rests on one consumer (D1, D5).

Items 2 to 4:

* Good, because an agent stopped by a signal or by item 1's step 2 answers its running prompts `cancelled`, as the specification
  asks at a cancel, instead of ending the output under them (item 2; D9).
* Good, because the SDK's own example, and every agent's main, stops writing the serve loop (item 2; R2, S1, S6).
* Good, because a failed start can show what the agent said on stderr, through the method or joined to `Wait`'s error, as the
  caller chooses (item 3; T1, T4).
* Good, because what a client records in use is what its tests read, compare and validate (item 4; D10). magic-cli-remote's
  capture gains the client's half of the wire (W1).
* Good, because a client can be tested against a real agent's recorded session, and sees where it departed from it (item 4,
  `ReplayAgent`; W1, W3).
* Neutral, because `ValidateGolden` accepts more than before: a response recorded before its request (item 4).
* Bad, because `Connection` gains a shutting-down state, in which it still reads but refuses new requests, in a hardened shutdown
  path (item 2; R3).
* Bad, because `<cli-b>`'s goldens are rewritten once (item 4; W4, W7).
* Bad, because `ReplayAgent` puts a matching policy in the SDK, the k-th request of a method meeting the k-th recorded answer,
  which a client that reorders its calls defeats (item 4, question 4c).
* Bad, because item 3 adds three fields and a type where one field would have served the recommended design (questions 3a, 3b).

### Confirmation

When accepted, the PLAN shows, each check first seen to fail on a planted break in a copy of the tree:

* each row of the mapping table, by a serve function of its own kind:
  * one that ends at its input's end;
  * one that ends only when its context is cancelled;
  * one that ends only when its pipes fail;
  * one that ignores all three and is abandoned, then released at the test's end;
  * one that returns while the connection is open (`ExitedFirst`);
* `TestDetachedTapIsPrefix` and `TestCloseReadsTheAgentsLastOutput` repeated for an in-process agent;
* every existing `agentproc` test passing unchanged after A2, 100 times on macOS under `-race` and on the Linux and Windows hosts, as
  0041's phase 3 ran them;
* the leak guard (F4) passing, and failing on a planted runner that leaves its watcher running;
* `make api`: only `InProcess`, `(*InProcess).Start` and `ErrAbandoned` added;
* `tools/consumer.py` running an in-process agent.

For items 2 to 4:

* **Item 2:**
  * a prompt running when `ServeAgent`'s context ends is answered `cancelled`, its handler having read `ErrShutdown`;
  * a request received during `Shutdown` is answered `-32603` at once, and an answer to the agent's own request still arrives;
  * `Shutdown` returns `context.Cause(ctx)` when a handler ignores its context past the bound;
  * `TestGoStatementsCounted` still allows only `WaitContext`'s `go` statement;
  * an `InProcess` agent built on `ServeAgent`, closed by step 2, answers its prompt `cancelled`;
  * the SDK's example agent serves through `ServeAgent`.
* **Item 3:**
  * the tail keeps the end of a stream longer than either cap, from a line's beginning, and a line longer than the byte cap keeps
    its end;
  * `Cmd.Stderr` still receives every byte;
  * an agent that writes and then fails `initialize` leaves its words in `StderrTail()`;
  * under `JoinStderrTail`, `Wait`'s error gives the tail through `errors.As`, and `errors.Is(err, ErrKilled)` still holds; without
    it, nothing is joined.
* **Item 4:**
  * a `Transcript` set as both taps of a real session reads back with `ReadRecords`, validates with `ValidateGolden`, and decodes
    its `session/update`s with `Decode`;
  * a response recorded before its request validates;
  * `Golden` fails on a changed record, writes the artifact, and updates under `ACPTEST_UPDATE=1`;
  * `ReplayAgent` plays a recorded session to a client that repeats it, and its `Serve` returns nil; a client that departs from
    it is answered `-32603`, and `Serve` returns the drift;
  * a transcript whose directions are interleaved out of order replays.
* Every new check is seen to fail on a planted break in a copy of the tree, and `make api` lists only the names this record adds.

## Pros and Cons of the Options

### O1. Nothing new; document the pattern

* Good, because it adds no permanent API on one consumer's evidence (D5).
* Good, because `WaitContext` and `EndedCleanly` already shorten the pattern to about 35 lines (C6).
* Bad, because each author writes the close order again, where D3's errors hide.
* Bad, because a client still holds two types for two kinds of agent (D2).

### O2. `agentproc.InProcess`, returning `*agentproc.Process`

* Good, because one type serves both kinds of agent, and the taps, `ExitedFirst` and `WaitContext` come with it (D2).
* Good, because the ladder maps step by step onto Go's own stop signals: input's end, context, failed I/O (G1, G5, G6).
* Neutral, because no other ACP or MCP SDK has it (E1, M1–M4), so there is no precedent to follow, and none to conflict with.
* Bad, because it changes `Process`'s internals (D6).
* Bad, because `Kill` cannot promise what it promises for a child (G2), so the shared type documents a weaker guarantee for one
  kind.

### O3. A runner with a type of its own

* Good, because `Process` is untouched (D6).
* Neutral, because it can state its own guarantees without qualifying `Process`'s.
* Bad, because a client needs its own interface to treat both kinds alike, so D2 is met only by the client.
* Bad, because it saves little over C6's 35 lines, which was 0035 F's ground for declining.

### O4. A connected pair only

* Good, because it is about fifteen lines and holds no policy.
* Bad, because it is two `io.Pipe` calls and two constructors, which saves nothing, and omits the close order (D3).
* Bad, because `acptest.Connect` already gives it to tests (F5).

### O5. The client's own binary, as a child

* Good, because it needs no new API: `agentproc.Command` with `os.Executable()` and the stdio mode the agent already has (C1's
  `<cli-b> acp`).
* Good, because the agent's panics and leaks stay in the child, with every guarantee of F2.
* Bad, because the agent no longer shares the client's memory: what `<cli-b>` passes as Go values (`agentOptions()`, its logger)
  must cross as flags or environment.
* Bad, because each start costs a process, and on Windows a job.

## More Information

### Relationship to earlier records

If O2 or O3 is accepted, this record supersedes the in-process runner in 0035 F and in
[0041-MADR-close-the-gaps-the-consumers-wait-on.md](0041-MADR-close-the-gaps-the-consumers-wait-on.md)'s "Not chosen", and that
MADR gains an amendment pointing here. The exported process-tree primitive 0041 also declines stays declined. If O1 is chosen,
both stand, and the pattern's documentation is this record's only change.

[0027-MADR-agent-process-helper.md](0027-MADR-agent-process-helper.md) decided `agentproc`'s lifecycle for a child. O2 extends it to
a second kind of agent, and does not reopen it.

Item 2 supersedes `ServeAgent` in 0035 F, which that report listed with the in-process `agentproc` as "not recommended". Items 3
and 4 reopen nothing. Item 4 builds on the record form 0041's item 8 added, and widens `ValidateGolden`'s pairing. The 0042 PLAN
carries all four items.

### Sources

* This repository at `26a7242`: `agentproc/agentproc.go`, `agentproc/tap.go`, `agentproc/leak_test.go`, `acptest/pair.go`,
  `connection_end_test.go`.
* `<cli-b>` at `5fb6928`: `acpclient/conn.go`, `acpclient/acptest/recorder.go`, `acpclient/acptest/script.go`,
  `acpserver/serve.go`, `internal/cli/agent.go`, `internal/cli/signal.go`; its 0002, 0016, 0031 and 0034 MADRs; its issues #3, #4
  and #5.
* magic-cli-remote at `5d1e7e2c` for item 1, and at `3cc402fb` for items 2 to 4:
  * `internal/provider/acpagent/acpagent.go`, `elicitation.go` and the tests W2 names;
  * `internal/provider/codex/provider.go`, `internal/provider/httpagent/provider.go`;
  * `internal/wirecap/wirecap.go`, `test/supervisors/probeagent/main.go`;
  * its 0214 MADR; its issue #28.
* This repository's `connection.go`, `acptest/golden.go`, `acptest/recorder.go` and `example/agent/main.go` at `26a7242`, for
  items 2 to 4.
* ACP: typescript-sdk `bca8e49`, rust-sdk `644377a`, python-sdk `2ad204b`, kotlin-sdk `cb69c39`, coder/acp-go-sdk `0845a3b`
  (v0.13.5), zed `37faef8`, agent-client-protocol `d6f00fd`.
* MCP and Go: modelcontextprotocol/go-sdk v1.8.0; mark3labs/mcp-go v1.4.0; modelcontextprotocol/typescript-sdk v2.4.0;
  modelcontextprotocol/python-sdk v1.30.0 and v2.3.0; golang/tools gopls/v0.23.0 and v0.52.0; grpc-go v1.82.0; golang.org/x/sync
  v0.24.0; Go go1.27.3's `io`, `net`, `os/exec`, `context`, `errors`, `net/http` and `net/http/httptest`.
* Prose: Dave Cheney, "Never start a goroutine without knowing how it will stop" (2016-12-22); Go Code Review Comments;
  golang/go#25245.

Line numbers were read at the commits and tags named. Where a source was read through a web page rather than its code, the
quotation is as the page renders it: Go Code Review Comments, Dave Cheney's article, and golang/go#25245.
