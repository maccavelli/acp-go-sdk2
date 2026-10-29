# Release notes for v1.26.0, against v1.25.0

Observed 2026-10-28 on `dd0d498`, for
[0042-PLAN-in-process-agent-runner.md](../decisions/0042-PLAN-in-process-agent-runner.md), phase 11. This report drafts the text of
the GitHub release for `v1.26.0`, as `RELEASING.md` describes it. It decides nothing. It is committed before the tag, so the tagged
tree holds it. The commit that holds it changes only records and `tools/`, so the API it tags is `dd0d498`'s.

Each statement names its source:

- a record;
- a commit;
- or `make api`, run on `dd0d498` with only `tools/consumer.py`, `tools/README.md`, `docs/README.md` and the records uncommitted,
  which hold no Go package.

Summary:

- Eighteen additions to the exported API: three in the root package, nine in `agentproc` and six in `acptest`. `make api` against
  `v1.25.0`: "0 incompatible, 18 compatible". The release is the minor `v1.26.0`, as `RELEASING.md` says for compatible changes.
- No defect fixed.
- What a consumer may meet:
  - nothing the SDK sends or accepts on the wire changes;
  - `acptest.ValidateGolden` accepts a response recorded before its request, which it used to fail;
  - a connection that never calls `Shutdown`, a `Command` with no stderr cap, and today's goldens behave as before.
- The `go` lines and the requirements are unchanged, and no generated file changed.

The section "Release text" is what goes into the GitHub release.

## Release text

### v1.26.0

The ACP schema is still `1.25.0` (`git diff v1.25.0 -- schema/` is empty). This release adds what the SDK's two consumers had each
written for themselves around an agent's life and its wire
([0042-MADR-in-process-agent-runner.md](../decisions/0042-MADR-in-process-agent-runner.md)):

- an agent written in Go run in the client's process, with the lifecycle a child has;
- an agent's graceful shutdown, and its main in one call;
- the end of a child agent's standard error, kept and joined to its error;
- transcripts of the wire, and tests that read, compare and replay them.

`make api` reports eighteen compatible additions and no incompatible change. Nothing sent or accepted on the wire changes.

#### New for clients: `agentproc`

- **`InProcess{Serve, Grace, ConnectionOptions, OutputTap, InputTap}.Start(client)`** runs an agent written in Go in this
  process, and returns the same `*Process` a `Command`'s child gets.
  - `Serve(ctx, in, out)` is what the agent's own main runs, such as `acp.ServeAgent`, here on a goroutine joined to the client
    by two `io.Pipe`s.
  - When `Serve` returns, its output and input are closed, as at a child's exit, and its error is `Wait`'s.
  - `Close`'s steps are mapped onto the goroutine: the first closes `Serve`'s input; the second cancels its context with
    `ErrTerminated`; the third cancels it with `ErrKilled` and closes its pipes. `Kill` runs them without their waits, but for
    the abandonment's grace.
  - A `Serve` that still runs a grace after the third step is abandoned, since Go cannot stop a goroutine, and `Wait`'s error
    joins **`ErrAbandoned`** to `ErrKilled`.
  - A panic in `Serve` is not recovered. An agent that must not take its client down with it runs as a child.
  - (`5293572`, `a5ff41f`.)
- **`Command.StderrTail`** and **`Command.StderrTailLines`** keep the end of the agent's standard error, at most so many bytes and
  lines, from a line's beginning. A last line longer than the byte cap keeps its end, so the byte cap bounds the memory.
  - `Cmd.Stderr` still receives every byte.
  - **`Process.StderrTail()`** returns the tail once `Wait` has returned.
  - **`Command.JoinStderrTail`** joins a **`*StderrError`** holding the tail to a `Wait` error that is not nil, for
    `errors.As`; `errors.Is` reads the rest of the error as before.
  - (`ad56dc2`, `dd0d498`.)
- **`NewTranscript(w)`** returns a **`Transcript`** whose `Input()` and `Output()`, set as the two taps, write both ways of the
  wire to `w`, one `{"direction","frame"}` object a line, the form `acptest.ReadRecords` reads.
  - A frame that is not JSON is written as a JSON string, so the file stays one object a line.
  - The two ways have no order between them, so a response can be written before its request.
  - `Close` drops a line either stream left unended, which is not a frame, and returns the first error a write to `w`
    returned. It does not close `w`.
  - (`1bb1eeb`.)

```go
tr := agentproc.NewTranscript(file)
p, err := (&agentproc.InProcess{
    Serve:     func(ctx context.Context, in io.Reader, out io.Writer) error { return acp.ServeAgent(ctx, agent, in, out) },
    InputTap:  tr.Input(),
    OutputTap: tr.Output(),
}).Start(client)
```

#### New for agents and both sides

- **`Connection.Shutdown(ctx)`** is the graceful close, as `http.Server.Shutdown` is a server's.
  - Every handler's context ends with **`ErrShutdown`**, and the connection keeps writing until each request it received before
    is answered: an agent's prompt whose handler honours its context is answered `cancelled`, as after `session/cancel`.
  - A request received meanwhile is answered `-32603`, and the connection keeps reading, so the answers to this side's own
    requests still arrive.
  - Then it closes, and `Err()` wraps `ErrShutdown` beside `ErrConnectionClosed`. When ctx ends first, it closes at once and
    returns ctx's cause.
  - (`5da0d0f`.)
- **`ServeAgent(ctx, agent, in, out, opts...)`** is an agent's main.
  - It serves until the input ends, waits up to 5 seconds for the handlers, and returns nil at a clean end.
  - When ctx ends, as at an interrupt, it shuts the connection down within the same 5 seconds and returns ctx's cause, joined
    with `Shutdown`'s error.
  - The example agent, `example/agent`, now serves through it.
  - (`5da0d0f`.)

#### New in `acptest`

- **`ReadRecords(r)`** and **`WriteRecords(w, records)`** read and write records one JSON object a line, the form
  `agentproc.Transcript` writes. A line that is not a record is an error naming its line. (`1bb1eeb`.)
- **`Decode[T](records, direction, method)`** decodes the params of each request or notification of a method, one way, into `T`.
  (`1bb1eeb`.)
- **`Golden(tb, path, records)`** compares records with a golden file, each frame in canonical form.
  - With `ACPTEST_UPDATE=1` set it writes the file.
  - Otherwise, on a difference, it writes the records to the test's artifact directory and names the first record that differs.
  - A golden file's CRLF line ends read as LF.
  - (`1bb1eeb`.)
- **`ReplayAgent{Records}`** plays a recorded session's agent side to a client under test. Its `Serve` is a serve function, for
  `InProcess` or **`ConnectReplay(tb, records, client, opts...)`**, which joins one to a client and fails the test when the client
  departs from the recording.
  - Each recorded answer is sent under the live request's id; the agent's own requests take fresh ids.
  - A client request beyond the recorded ones of its method is answered `-32603`, and reported.
  - The records need both ways, but no order between them.
  - (`5828c46`, `dd0d498`.)
- **`ValidateGolden`** matches a response to its request wherever the request is among the records, before it or after it, as a
  transcript's two ways require. Requests that share a way and an id are matched to their responses in order. It used to fail a
  response recorded before its request. (`1bb1eeb`.)

#### For maintainers

- **`agentproc`'s seam.** `Process` drives its agent through an unexported interface of five operations, which a child and an
  in-process agent each implement. The taps read and write an `io.ReadCloser` and an `io.WriteCloser`. No exported change, and
  every existing `agentproc` test passes unchanged. (`5293572`.)
- **`TestRuntimeFilesCoverHandWritten`** names `serve.go` (`5da0d0f`). **`TestGoStatementsCounted`** is unchanged: `Shutdown` and
  `ServeAgent` add no `go` statement to the root package.
- **Tools.** `tools/consumer.py` builds a module at the Go floor that also runs an `InProcess` agent through `ServeAgent` with a
  `Transcript` as its taps, prints a failing child's stderr tail, and, in its test, reads a transcript back with `ReadRecords`,
  checks it with `ValidateGolden` and `Decode`, and replays it with `ConnectReplay`.
- **Tests.**
  - At the root: `shutdown_test.go`, and `ExampleServeAgent` in `example_serve_test.go`.
  - In `agentproc`: `inprocess_test.go`, `stderr_test.go` and `transcript_test.go`, with `ExampleInProcess` and
    `ExampleTranscript`.
  - In `acptest`: `records_test.go` and `replay_test.go`, with `ExampleReplayAgent`.

## Sources not in the release text

- **Commits since `v1.25.0`** (`git log v1.25.0..dd0d498`), twelve:
  - `5293572`, `a5ff41f`, `5da0d0f`, `ad56dc2`, `1bb1eeb`, `5828c46` and `dd0d498` change the packages, as above;
  - `5da0d0f` also `cmd/generate/engine_test.go` and `example/agent/main.go`;
  - `2a053e9` changes `README.md`, `docs/architecture.md`, the examples in `acptest/example_test.go`,
    `agentproc/example_test.go` and `example_serve_test.go`, and the PLAN;
  - `dd0d498` also `docs/architecture.md`;
  - `26a7242`, `2a3ea34`, `5b09afc` and `4d84311` change records only (`git show --name-only`).
- **Go files changed.** `git diff --name-status -M v1.25.0 dd0d498 -- '*.go' ':!*_test.go'` lists 12 files.
  - Added: `serve.go`, `acptest/records.go`, `acptest/replay.go`, `agentproc/inprocess.go`, `agentproc/stderr.go` and
    `agentproc/transcript.go`.
  - Changed: `connection.go`, `errors.go`, `acptest/golden.go`, `agentproc/agentproc.go`, `agentproc/tap.go` and
    `example/agent/main.go`.
- **Unchanged** (`git diff`):
  - `go.mod`, `go.sum`, `cmd/generate/go.mod` and `cmd/generate/go.sum`;
  - the schema, the workflows, the `Makefile`, `RELEASING.md`, `scripts/`, `clienthost/` and `internal/`;
  - every `*_gen.go` file.
- **`make api` on `dd0d498`:** exit 0, "against v1.25.0: 0 incompatible, 18 compatible", each "added":
  - the root package's three: `(*Connection).Shutdown`, `ErrShutdown` and `ServeAgent`;
  - `agentproc`'s nine: `(*Process).StderrTail`, `Command.JoinStderrTail`, `Command.StderrTail`, `Command.StderrTailLines`,
    `ErrAbandoned`, `InProcess`, `NewTranscript`, `StderrError` and `Transcript`;
  - `acptest`'s six: `ConnectReplay`, `Decode`, `Golden`, `ReadRecords`, `ReplayAgent` and `WriteRecords`;
  - and "reported packages unstable: 0 incompatible, not gated".
- **New types' methods.** `(*InProcess).Start`, `StderrError.Error`, `Transcript`'s `Input`, `Output` and `Close`, and
  `(*ReplayAgent).Serve` are not in `make api`'s list: it counts a new type's methods in the type.
- **The evidence:**
  - the 0042 PLAN's phases 2 to 10: the probes of `io.Pipe` and the connection in phase 2, and the 31 planted breaks of phases 4
    to 10;
  - after the seam, `TestDetachedTapIsPrefix`, `TestCloseReadsTheAgentsLastOutput` and `TestKillClosesAtOnce` passing 100 times
    each on macOS under `-race` and on the Linux host, and the first two on the Windows host;
  - `TestInProcessDetachedTapIsPrefix` passing 10 times on each of the three;
  - the gates on macOS and on the Linux and Windows hosts;
  - the deviation entries, each recorded in the PLAN and, where a decision or a fact changed, in the MADR.
