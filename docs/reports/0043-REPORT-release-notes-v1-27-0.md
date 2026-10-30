# Release notes for v1.27.0, against v1.26.0

Observed 2026-10-30 on `00c37a3`, for
[0043-PLAN-remote-http-and-websocket-transport.md](../decisions/0043-PLAN-remote-http-and-websocket-transport.md), phase 10.
This report drafts the text of the GitHub release for `v1.27.0`, as `RELEASING.md` describes it. It decides nothing. It is
committed before the tag, so the tagged tree holds it. The commit that adds it changes `tools/consumer.py`, `tools/README.md`,
records and nothing else, so the API it tags is `00c37a3`'s.

Each statement names its source:

- a record;
- a commit;
- or `make api`, run on `00c37a3` with only `tools/consumer.py`, `tools/README.md`, `docs/README.md` and the records
  uncommitted, which hold no Go package.

Summary:

- One new package, `remote`: ACP over the network, the RFD "Streamable HTTP & WebSocket Transport" at its revision of
  2026-07-02. `make api` against `v1.26.0`: "0 incompatible, 11 compatible", each in `remote`, which it reports without
  gating, as it does `unstable`. The release is the minor `v1.27.0`, as `RELEASING.md` says for compatible changes.
- No defect fixed.
- What a consumer may meet:
  - a new requirement in its module graph, `github.com/coder/websocket v1.8.15`, compiled only into a program that imports
    `remote`;
  - nothing the root package, `agentproc`, `acptest` or `clienthost` sends or accepts changes, and none of them imports
    `remote`;
  - `remote` is outside the compatibility promise: a name in it may change in a minor release when the RFD changes.
- The `go` lines are unchanged, and no generated file changed.

The section "Release text" is what goes into the GitHub release.

## Release text

### v1.27.0

The ACP schema is still `1.25.0` (`git diff v1.26.0 -- schema/` is empty). This release adds package `remote`, which runs ACP
over the network: the RFD
["Streamable HTTP & WebSocket Transport"](https://agentclientprotocol.com/rfds/streamable-http-websocket-transport) at its
revision of 2026-07-02, on both sides and in both of its profiles. A Go program can now serve an agent over the network,
and reach an agent served over it, with the agent and client code it has today. It interoperates with the official TypeScript
and Python SDKs, run against each other, and follows the Rust SDK's rules as its source reads
([0043-MADR-remote-http-and-websocket-transport.md](../decisions/0043-MADR-remote-http-and-websocket-transport.md)).

`remote` is not subject to the Go 1 compatibility promise: the RFD is Active, not Completed, and a name in the package may
change or go away in a minor release when the RFD changes. `make api` reports its eleven additions without gating them. Nothing
else changes.

#### New: package `remote`

Both sides present the remote connection as the byte stream the rest of the module reads and writes, one JSON-RPC message a
line. So connections, `acp.ServeAgent`, the taps and transcripts work over the network as they do over a child's pipes.

- **`Dial(ctx, url, opts)`** returns a **`*Conn`**, an `io.ReadWriteCloser` for both of a connection's streams:
  `acp.NewClientSideConnection(client, c, c)`.
  - `ws://` and `wss://` dial WebSocket; `http://` and `https://` dial Streamable HTTP.
  - **`DialOptions`**: `HTTPClient`, whose nil `Jar` becomes a cookie jar of the connection's own; `Header`, sent on every
    request, the handshake included; `MaxMessageSize`, 10 MiB by default.
  - Over Streamable HTTP the client works with each official server. It POSTs in order, sends `Acp-Session-Id` as TypeScript
    requires, retries a `404` without it as Python needs, and opens a session's stream before `session/load` or
    `session/resume`. It reads every stream into one connection, in no order between streams, as the RFD gives none.
  - (`9e8def2`, `7e57c81`.)
- **`NewHandler(serve, opts)`** returns a **`*Handler`**, an `http.Handler` for the RFD's single endpoint. It runs `serve` once
  per connection, usually `acp.ServeAgent`.
  - A GET with `Upgrade: websocket` opens a WebSocket. POST, GET and DELETE serve Streamable HTTP with the RFD's statuses, and
    any other method is `405`.
  - A message's session comes from `Acp-Session-Id` or `params.sessionId`, Rust's rule, so the requests each official client
    sends are accepted: tested as each shapes them, and run against the TypeScript and Python clients.
  - A POST is answered `202` once its message is queued, before the agent can answer.
  - Each session's messages go on its own event stream. Messages for a stream nobody is receiving are held, up to
    `MaxBuffered`.
  - **`HandlerOptions`**: `Grace` (5 s), `KeepAlive` (15 s), `MaxMessageSize` (10 MiB), `MaxBuffered` (64 MiB),
    `OriginPatterns` and `Logger`, which never logs a message, an id or a session id.
  - (`9e8def2`, `8451cab`, `d302ec7`.)
- **Endings.** Every connection ends through one ladder, as `agentproc.InProcess` does: close the serve's input, cancel its
  context, close its pipes, each step waiting up to `Grace`.
  - The context's cause says why: **`ErrDeleted`**, **`ErrDisconnected`**, **`ErrHandlerClosed`** or **`ErrOverflow`**.
  - **`Handler.Close(ctx)`** ends every connection, answers later requests `503`, and returns **`ErrAbandoned`** for a serve
    that outlived the ladder. Call it before `http.Server.Shutdown`.
  - (`9e8def2`, `8451cab`.)
- **The host's part.** Authentication, TLS, CORS and limits on connections are the host's, as the RFD layers them on the
  transport. `Acp-Connection-Id` is not a credential, so authenticate every request, not only `initialize`.

```go
h := remote.NewHandler(func(ctx context.Context, in io.Reader, out io.Writer) error {
    return acp.ServeAgent(ctx, agent, in, out)
}, nil)
mux.Handle("/acp", h) // behind your authentication
```

Not supported: resuming an event stream with `Last-Event-ID`, automatic reconnection, batches (`501`), WebSocket subprotocols,
compression and WebSocket over HTTP/2, and Unix sockets.

#### Examples and interoperability

- **`example/remote`**: `serve -addr ADDR` serves an echo agent that can load its sessions; `dial URL` prompts it and loads the
  session again. (`d302ec7`.)
- **Interoperability.** `tools/interop.py` runs eight pairs against the official TypeScript SDK at `08a0d30` and Python SDK at
  `9853d8b`: the Go client against each one's example server, and their example clients against `example/remote`, over both
  profiles. All eight pass, `session/load` against the TypeScript server included. (`d302ec7`.)

#### For maintainers

- **The dependency.** `go.mod` requires `github.com/coder/websocket v1.8.15`, ISC-licensed, with no requirements of its own.
  `remote` alone imports it. (`9e8def2`.)
- **`scripts/check_api.py`** reports `remote` beside `unstable`, without gating it. (`924c12a`.)
- **Tests.** `remote` has 55 tests and seven examples, on loopback, under the package's goroutine-leak guard.
  - The server is tested against requests shaped as each official client sends them.
  - The client is tested against a fake server in each official server's mode.
  - A round trip runs over HTTP/1.1, HTTP/2 over TLS and unencrypted HTTP/2.
  - `TestReadmeRemoteIsTheExample` keeps the README's example compiling.
- **Tools.** `tools/interop.py`, above. `tools/consumer.py` builds a module at the Go floor that also serves a `ScriptAgent`
  through `remote.NewHandler` and dials it over `ws://` and `http://`, so the proxy's copy of `github.com/coder/websocket` is
  fetched with the checksum database on.

## Sources not in the release text

- **Commits since `v1.26.0`** (`git log v1.26.0..00c37a3`), eleven:
  - `0170641` marks the 0042 PLAN complete;
  - `6708b47`, `a3d3b99` and `36e2c45` add the 0043 MADR and PLAN, and the probes' record;
  - `924c12a` (the seam), `9e8def2` (WebSocket and the dependency), `7e57c81` (the Streamable HTTP client), `8451cab` (its
    server) and `d302ec7` (`example/remote`, `tools/interop.py`, and the server's inbound queue) change the packages;
  - `b9e93c3` changes `README.md`, `docs/architecture.md`, `docs_test.go` and `remote/example_test.go`;
  - `00c37a3` adds `session/fork` and `session/resume` routing tests and five examples, and corrects docs;
  - each changes the PLAN (`git show --name-only`).
- **Go files changed.** `git diff --name-status -M v1.26.0 00c37a3 -- '*.go' ':!*_test.go'` lists 11 files, each added:
  `example/remote/main.go`, and `remote/`'s `client.go`, `doc.go`, `errors.go`, `frame.go`, `handler.go`, `httpclient.go`,
  `httpserver.go`, `route.go`, `sse.go` and `ws.go`.
- **Changed outside `remote/` and `example/remote/`:** `go.mod` and `go.sum`, which gain the requirement and its two sums;
  `scripts/check_api.py` and `scripts/test_check_api.py`; `docs_test.go`; `README.md`; `docs/architecture.md`; `tools/`.
- **Unchanged** (`git diff`): `cmd/generate/go.mod` and `cmd/generate/go.sum`; the schema, the workflows, the `Makefile` and
  `RELEASING.md`; `agentproc/`, `acptest/`, `clienthost/` and `internal/`; every `*_gen.go` file.
- **`make api` on `00c37a3`:** exit 0, "against v1.26.0: 0 incompatible, 11 compatible", each "added" in `remote`: `Conn`,
  `Dial`, `DialOptions`, `ErrAbandoned`, `ErrDeleted`, `ErrDisconnected`, `ErrHandlerClosed`, `ErrOverflow`, `Handler`,
  `HandlerOptions` and `NewHandler`; and "reported packages unstable, remote: 0 incompatible, not gated". `make api` counts a
  new type's methods in the type, so `Conn`'s and `Handler`'s are not listed.
- **CI on `4ae65b8`**, the commit that adds this report, pushed with the 0043 commits before it:
  - Run `41069406511`, attempt 1: five of six jobs succeeded. `validate (windows-2025, 1.27.3)` failed in its "race detector"
    step, in `agentproc`'s `TestStderrHeldByDescendant` (`agentproc_test.go:234`): `Close = agentproc: agent killed: exec:
    WaitDelay expired before I/O complete, want exec.ErrWaitDelay, and no signal`. `remote` and `example/remote` passed in the
    same step.
    - `agentproc` is unchanged since `v1.26.0`; the test is 0027's.
    - Its grace is 300 ms, so the agent's `WaitDelay` is 150 ms, and the agent must be seen to exit within `Close`'s first
      step.
    - It was the test's only failure in the last 15 runs of `main`, and it passed 50 times of 50 on the Windows test host
      without `-race`.
  - Attempt 2, the failed job rerun on the owner's choice: "completed success", all six jobs.
  - The Docs workflow's run `41069406598`: "completed success".
  - The "race detector" step is the `-race` run on Linux and Windows that the MADR's "Confirmation" names (the PLAN's
    phase 9). Every 0043 package passed it in both attempts.
  - The test's margin under the race step's load is left to a later amendment of
    [0027-PLAN-agent-process-helper.md](../decisions/0027-PLAN-agent-process-helper.md).
- **The consumer.** `tools/consumer.py`'s program and test, built against `00c37a3`'s tree through a `replace` at `go1.26.10`,
  printed `WANT_RUN` exactly, the two `remote` lines included, and built for Windows. With the server's frames made binary in a
  scratch copy, its output differed, as required. `consumer.py v1.27.0` runs after the tag.
- **The evidence:**
  - the 0043 PLAN's phases 2 to 9: the probes, and each phase's planted breaks;
  - the timing-sensitive tests passing 20 times on macOS under `-race` and on the Linux and Windows hosts, in phases 4, 6, 7 and 9;
  - `tools/interop.py`'s eight pairs, in phase 7;
  - the gates on macOS and on the Linux and Windows hosts;
  - the deviation entries, each recorded in the PLAN and, where a decision or a fact changed, in the MADR.
