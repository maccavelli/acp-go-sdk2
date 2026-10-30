---
status: in-progress
date: 2026-10-29
associated-madr: "0043-MADR-remote-http-and-websocket-transport.md"
---

# Implement 0043: ACP's remote transport, Streamable HTTP and WebSocket, in package `remote`

Associated MADR: [0043-MADR-remote-http-and-websocket-transport.md](0043-MADR-remote-http-and-websocket-transport.md)

## Goal

Package `github.com/maccavelli/acp-go-sdk2/remote` implements the RFD "Streamable HTTP & WebSocket Transport" at its revision of
2026-07-02, on both sides and in both profiles, as the MADR's "The design", "Decided with the owner, 2026-10-29" and "Amended by
the PLAN, 2026-10-29" describe. Its exported API is exactly this:

```go
func Dial(ctx context.Context, url string, opts *DialOptions) (*Conn, error)
type DialOptions struct {
    HTTPClient     *http.Client
    Header         http.Header
    MaxMessageSize int
}
type Conn struct{ /* unexported */ }
func (c *Conn) Read(p []byte) (int, error)
func (c *Conn) Write(p []byte) (int, error)
func (c *Conn) Close() error

func NewHandler(serve func(ctx context.Context, in io.Reader, out io.Writer) error, opts *HandlerOptions) *Handler
type HandlerOptions struct {
    Grace          time.Duration
    KeepAlive      time.Duration
    MaxMessageSize int
    MaxBuffered    int
    OriginPatterns []string
    Logger         *slog.Logger
}
type Handler struct{ /* unexported */ }
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request)
func (h *Handler) Close(ctx context.Context) error

var ErrDeleted, ErrDisconnected, ErrHandlerClosed, ErrOverflow, ErrAbandoned error
```

* `acp.NewClientSideConnection(client, c, c)` over a `*Conn`, and `acp.ServeAgent` inside `serve`, work unchanged. So do the
  taps, `agentproc.Transcript` and `acptest`'s records.
* `make api` lists `remote`'s additions as reported and gates nothing in it. The four gated packages show "0 incompatible, 0
  compatible" changes.
* `github.com/coder/websocket v1.8.15` is the module's second requirement, and `remote` alone imports it.
* `tools/interop.py` passes against the TypeScript and Python SDKs at the revisions the MADR names.

## Prerequisites

* The MADR is `accepted`, with the owner's answers of 2026-10-29 and the amendment written with this PLAN.
* [0042-PLAN-in-process-agent-runner.md](0042-PLAN-in-process-agent-runner.md) is `complete` (`v1.26.0`). The serve-function shape
  (`agentproc/inprocess.go:42`), `acp.ServeAgent` (`serve.go:29`), `agentproc.NewTranscript` and `acptest.ReadRecords` are what
  this PLAN builds on.
* Node, npm and `uv` on the machine that runs phase 2's peer probes and phase 7. They are present on the owner's macOS host
  (Node 26.10.0, npm 11.19.1, uv 0.12.21). The Linux and Windows hosts need only Go.

## Scope

* **New package `remote/`:**

  | File | What it holds |
  | :--- | :--- |
  | `doc.go` | the package doc: the RFD and its revision, the two profiles, the seam, outside the compatibility promise |
  | `errors.go` | `ErrDeleted`, `ErrDisconnected`, `ErrHandlerClosed`, `ErrOverflow`, `ErrAbandoned` |
  | `frame.go` | the seam: bytes written split into lines; a message received made one compact line |
  | `route.go` | a message's kind, id key, method, `params.sessionId` and `result.sessionId`; the session-scoped methods |
  | `sse.go` | an SSE event writer and a bounded SSE reader |
  | `client.go` | `Dial`, `DialOptions`, `Conn`, and the unexported interface its two profiles implement |
  | `ws.go` | WebSocket, both sides |
  | `httpclient.go` | the Streamable HTTP client |
  | `handler.go` | `NewHandler`, `HandlerOptions`, `Handler`: routing by method and upgrade, the registry, the ending ladder, `Close` |
  | `httpserver.go` | a Streamable HTTP connection: POST, the SSE streams, the outbound router, buffers, the idle bound |
  | `*_test.go`, `leak_test.go`, `example_test.go` | the tests, the goroutine-leak guard, `ExampleDial` and `ExampleNewHandler` |

* **New example `example/remote/`:** `main.go` with two modes.
  * `serve -addr ADDR` serves an echo agent that can load its sessions, through `NewHandler` and `acp.ServeAgent`.
  * `dial URL` runs `initialize`, `session/new` and one prompt, and prints the agent's text and the stop reason. When the agent
    advertises `loadSession`, it loads the session on a second connection.

  It also has a `main_test.go`. Phase 7 drives both modes.
* **Module:** `go.mod` and `go.sum`, with `github.com/coder/websocket v1.8.15`, in the commit of the first import (`AGENTS.md`,
  "Dependencies").
* **Scripts and tools:**
  * `scripts/check_api.py`: `REPORTED` gains `"remote"`, and `scripts/test_check_api.py` asserts it;
  * a new `tools/interop.py` with offline tests in `tools/test_tools.py`;
  * `tools/README.md`;
  * `tools/consumer.py` (phase 10).
* **Docs:**
  * `README.md`: a "Remote agents" section, and `remote` in "Stability";
  * `docs_test.go`: `TestReadmeRemoteIsTheExample`, as `TestReadmeAgentprocIsTheExample`;
  * `docs/architecture.md`: the package, its seam and its tree entry.
* **Records:** this PLAN and its row; the MADR's row; the release-notes report in phase 10.
* **Not in scope:**
  * the root package's non-test files, `agentproc`, `acptest`, `clienthost`, `internal/proctree` and every generated file, none
    of which changes;
  * `tools/platforms.py`, since `remote` has no per-platform file;
  * everything the MADR's "Not in this record" lists: SSE resumability and `Last-Event-ID`, automatic reconnect, batches,
    WebSocket subprotocols and compression, WebSocket over HTTP/2, Unix sockets, proxy chains, the schema pin;
  * the host's part: authentication, CORS preflight, TLS, and limits on connections per client. Rust's guide leaves the same to
    its host, "This SDK does not provide a general authentication configuration";
  * any change in either consumer's repository.

## Implementation Steps

Each phase ends with its checks, an entry in the execution record, and its files staged for the owner, who commits on `main`
(`AGENTS.md`, "Commits"). Every new check is seen to fail on a planted break, each break in its own copy of the tree through
`tools/plants.py`, and the record keeps each break's failing line.

Every code phase runs these gates:

* `make pre-add-check`, and `scripts/go-precheck.sh` on each new file;
* `make check-records`, `make markdownlint`, `make disclosure-check` and `make api`;
* `tools/gates.py --all`, with `ACP_LINUX_HOST` and `ACP_WINDOWS_HOST` set;
* `tools/idscan.py --paths` over the staged files.

Every test in `remote` follows four rules:

* It runs on loopback, through `httptest.NewServer` or `httptest.NewUnstartedServer`. `httptest.NewTestServer` is not used: the
  1.26.10 floor does not have it (`go doc` at `go1.26.10`: "no symbol NewTestServer").
* It waits on an observable event (a frame read, a status received, a channel closed) under a bound named in its failure, never
  on a sleep. 0042's phase 11 deviation is the lesson: a test that released its tap before the connection had read failed in CI
  only.
* A test that times the ending ladder runs `-count=20` on macOS under `-race` and on both hosts through `tools/remote.py pkg`
  before the phase closes.
* The package's `TestMain` fails the run on a leaked goroutine through the `goroutineleak` profile, as `agentproc/leak_test.go`
  does. The profile needs `GOEXPERIMENT=goroutineleakprofile` on Go 1.26, which the Makefile, `scripts/go-precheck.sh` and CI's
  floor jobs already set.

### 1. Records

This PLAN becomes `in-progress`, and its row follows it. The MADR's amendment, "Amended by the PLAN, 2026-10-29", was written with
this PLAN and is reviewed with it.

[0039-MADR-five-adopter-additions-on-schema-1-25-0.md](0039-MADR-five-adopter-additions-on-schema-1-25-0.md) gains "Amended by
0043-MADR-remote-http-and-websocket-transport.md, 2026-10-29", in the same way 0042's PLAN amended 0041's "Not chosen". It says
that its "Out of scope" item "A WebSocket or Unix-socket transport (0035 F: needs a dependency, needs its own MADR)" is decided for
WebSocket by 0043, and that Unix sockets stay out. Its row in `docs/README.md` says so.

### 2. Probes

These run in a scratch module outside the tree, which requires `github.com/coder/websocket v1.8.15`, and in clones of the two
SDKs in the tools' cache directory (`ACP_TOOLS_CACHE`). Nothing in the tree changes. Each result is recorded with its command and output.

* **Go and the WebSocket library:**

  | Probe | Question | What the design assumes |
  | :--- | :--- | :--- |
  | P1 | Does `websocket.Dial` to a `wss://` `httptest` TLS server with `EnableHTTP2` succeed through the default `http.Transport`, or does ALPN pick HTTP/2 and the upgrade fail? | It fails, or may: `Dial` clones the caller's transport with `Protocols` set to HTTP/1 alone (the MADR's amendment, 4) |
  | P2 | Does a header set on `w.Header()` before `websocket.Accept` reach the `101` response? | Yes: `accept.go:135`–`:151` sets its own headers on `w.Header()` and then calls `WriteHeader(101)` |
  | P3 | Does a `Read` whose context ends close the WebSocket connection? | Yes, as `Conn`'s doc says, so each side reads under one context that lives as long as the connection |
  | P4 | Does `Ping` return while no goroutine is reading? | No, since control frames are handled by reads, so the keepalive runs beside the reader |
  | P5 | Does an SSE response survive `http.Server.WriteTimeout` once `ResponseController.SetWriteDeadline(time.Time{})` clears it, and does `Flush` deliver each event at once? | Yes to both |
  | P6 | With `Transport.Protocols` set to `UnencryptedHTTP2` alone, do one long-lived GET and concurrent POSTs share one TCP connection with an `http.Server` whose `Protocols` allows it? | Yes |
  | P7 | Does `websocket.Accept` pass a handshake with no `Origin`, refuse one whose `Origin` host is foreign with `403`, and pass it once `OriginPatterns` matches that host? | Yes to all three, as `authenticateOrigin` reads (`accept.go:228`–`:259`); the HTTP methods copy that rule (the MADR's amendment, 7) |
  | P8 | Does a `cookiejar` on the `HTTPClient` given to `websocket.Dial` send a cookie the server set on an earlier POST? | Yes, since `Dial` sends its handshake through `HTTPClient.Do` (`dial.go:223`) |

* **The peer servers**, the TypeScript example (`npx tsx src/examples/http-server.ts`, `PORT` set) and the Python one
  (`uv run examples/http_server.py`, `localhost:8000`). Each probe is a raw HTTP exchange from a scratch Python script:

  | Probe | Question | What the MADR's K3 reads in the code |
  | :--- | :--- | :--- |
  | P9 | Is a session-scoped GET for a session the connection has not named answered `200` or `404`? | TypeScript `200`; Python `404` |
  | P10 | On which stream does each server send the answer to `session/new`, and the TypeScript server the answer to `session/load` and its replay? | the connection stream |
  | P11 | Does each server's WebSocket `101` carry `Acp-Connection-Id`, and does it accept `initialize` as the first frame? | yes |
  | P12 | Does each answer a plain HTTP/1.1 POST, a POST that is not `application/json` (`415`), and a batch (`501`)? | yes, `415`, `501` |
  | P13 | Is a `session/load` POST on a fresh connection answered `400` by TypeScript without `Acp-Session-Id`, and `404` by Python with it? Is Python's answer `202` once the header is dropped? | `400`; `404`; `202` |

  Rust's server is read, not run: no Rust toolchain is required by this PLAN, and K3 cites its source.

A result that contradicts the MADR or its amendment stops the phase. The MADR is amended with the owner before phase 3.

### 3. The seam and the skeleton (no dependency yet)

* `doc.go`: the package doc. It names the RFD, its revision of 2026-07-02 and the specification commit `2bb9fac`. It says the
  package is not subject to the Go 1 compatibility promise: a name in it may change in a minor release when the RFD changes. It
  describes the seam and the two profiles.
* `errors.go`: the five errors, each `errors.New` with a `remote:` prefix. Each doc says when the error appears: a serve's
  `context.Cause` for the first four, `Handler.Close` for `ErrAbandoned`.
* `frame.go`:
  * `lineSplitter`: bytes written are split at `\n` and a line split across writes is joined, as `agentproc/linetap.go` does;
    each complete line goes to a send function; what is held at `Close` is dropped. A line over `MaxMessageSize` is refused with
    an error, not sent.
  * `toLine(msg []byte) []byte`: `json.Valid`, then `json.Compact`, then a `\n` appended. A message that is not JSON has its CR and
    LF replaced by spaces (the MADR's amendment, 6).
  * `lineQueue`: received lines, read by `Conn.Read` and the handler's agent input through `io.Reader`, with an end error.
* `route.go`:
  * `classify(line) message`: kind (request, notification, response), the id key (the compacted raw `id`), method,
    `params.sessionId` and `result.sessionId`, decoded through a struct of `json.RawMessage`s, as `connection.go`'s `envelope` is.
  * `sessionScoped(method)`: true for `acp.AgentMethodSessionPrompt`, `…Cancel`, `…Close`, `…Delete`, `…Load`, `…Resume`,
    `…SetConfigOption` and `…SetMode` (`constants_gen.go:69`–`:82`), and for the literal `"session/fork"`. This is the list of
    Rust's `protocol.rs` and TypeScript's `protocol.ts` (the MADR's K3), less `session/set_model`, which the pinned schema does not
    have. `acp.AgentMethodSessionFork` is not used: it carries a `Deprecated:` notice (`constants_gen.go:72`–`:75`), and
    staticcheck's SA1019, which `.golangci.yml` enables, flags its use outside `acp`.
  * A `$/` method is connection-scoped, as Rust's is.
  * The id key is exact. The connection writes `anyMessage.ID` as a `*json.RawMessage` (`connection.go:36`–`:38`), so an answer
    carries the request's id as it was received. `toLine` compacts the request and `json.Marshal` compacts a `RawMessage` again,
    so the bytes match. *(Amended in phase 3's execution: a `json.RawMessage` decoded from a member holds a scalar's bytes with no
    space around or inside them, so the id's bytes are the key, uncompacted. See "Deviation, 2026-10-29, phase 3's id-key
    break".)*
* `scripts/check_api.py`: `REPORTED = ("unstable", "remote")`. `scripts/test_check_api.py` asserts `"remote"` in `REPORTED` and
  not in `PACKAGES`.
* `leak_test.go`: `TestMain` and `waitForLeaks`, as `agentproc/leak_test.go`.

Tests:

| Test | Required |
| :--- | :--- |
| `TestLineSplitter` | lines joined across writes, several in one write, an unended line dropped at `Close`, an over-long line refused |
| `TestToLine` | pretty-printed JSON compacted to one line; a JSON string holding `\n` escapes kept; not-JSON with CR and LF made one line |
| `TestClassify` | each kind; a numeric, a string and a fractional id; `params.sessionId` and `result.sessionId`; a `$/cancel_request` connection-scoped; params that are an array |
| `TestSessionScoped` | the eight constants and `"session/fork"` true; `initialize`, `session/new`, `session/list`, `authenticate` false |
| `TestIDKeyRoundTrip` | an id received, through `toLine`, `acp.ServeAgent` with an `acptest.ScriptAgent` and its answer, keys the same |

Planted breaks:
* the line join removed, so a split line becomes two;
* `toLine` not compacting;
* `$/` treated as session-scoped;
* ~~the id key taken before compaction~~ *(replaced in execution by "answers not keyed", with `TestIDKeyBytesExact` and its break
  "id keys that normalize a number" added: see "Deviation, 2026-10-29, phase 3's id-key break")*.

`make api`: `remote`'s additions reported, the rest "0 incompatible, 0 compatible".

### 4. WebSocket, client and server

This phase adds `github.com/coder/websocket v1.8.15` (`go get github.com/coder/websocket@v1.8.15`, then `go mod tidy`). `go.mod`,
`go.sum` and `ws.go`, its first import, are in one commit.

* **`client.go`:** `Dial` takes the URL apart. For `ws://` and `wss://` it runs the WebSocket handshake under `ctx`. For
  `http://` and `https://` it returns the Streamable HTTP `Conn` of phase 5. Between this phase's commit and phase 5's, those
  schemes return the error "remote: Streamable HTTP is not implemented yet"; no tag falls between them. Another scheme is an
  error. `ctx` bounds the handshake only, never the connection.
  * `DialOptions`: a nil `HTTPClient` is a fresh `&http.Client{}`. A client whose `Jar` is nil is copied with a fresh
    `cookiejar.New(nil)`, so each `Conn` keeps its own cookies (the RFD's "Clients MUST accept, store, and return cookies").
    `Header` goes on every request, the handshake included. `MaxMessageSize` of zero or less means 10 MiB, the connection's default
    (`connection.go:26`).
  * For WebSocket, the client's transport is cloned with `Protocols` set to HTTP/1 alone, if P1 shows that the default
    negotiates HTTP/2. If P1 shows that it does not, the clone is still made, so that a caller's HTTP/2-only transport cannot
    break the handshake.
* **`ws.go`, both sides:**
  * one reader goroutine per connection reads under the connection's own context (P3). A text frame becomes a line through
    `toLine`. A binary frame is ignored, as the RFD says, and the handler logs the first one. A normal closure is `io.EOF` to the
    reader; any other close or error ends the reader with that error;
  * `SetReadLimit(MaxMessageSize)`, since the library's default is 32768 bytes (`read.go:91`, `:107`);
  * a write sends each line as one text frame, without its newline, under a bound of `Grace` *(on the server; a client's writes
    run for the connection's life, by "Deviation, 2026-10-29, phase 4's client write bound")*;
  * on the server, a keepalive goroutine pings every `KeepAlive` while the reader runs (P4), and a ping that fails ends the
    connection with `ErrDisconnected`. The client sends no ping, since `DialOptions` has no `KeepAlive`: its reader answers the
    server's pings, as the library answers every ping it reads.
* **`handler.go`:**
  * `NewHandler` panics on a nil `serve`, naming it, as `acp.NewClientSideConnection` panics on a nil argument (`client_gen.go:32`).
    Defaults: `Grace` 5 s, `KeepAlive` 15 s (the MADR's amendment, 8), `MaxMessageSize` 10 MiB, `MaxBuffered` 64 MiB, and
    `slog.Default()` when `Logger` is nil.
  * `ServeHTTP` sends a `GET` with `Upgrade: websocket` to `websocket.Accept`, with `OriginPatterns`, after setting
    `Acp-Connection-Id`, `crypto/rand.Text()`, on `w.Header()` (P2). The other methods are phase 6's, and until then they are
    answered `501`.
  * Each connection runs `serve` on its own goroutine with two `io.Pipe`s, under `context.WithCancelCause(context.Background())`,
    as `InProcess.Start` does (`agentproc/inprocess.go:79`–`:91`). The agent's output goes through a `lineSplitter` to the socket,
    and the socket's lines go to its input.
  * **The ending ladder**, the same for a WebSocket close, a `DELETE` (phase 6), the idle bound (phase 6), an overflow and
    `Close`:
    1. close the agent's input and wait up to `Grace` for `serve` to return;
    2. cancel its context with the cause (`ErrDisconnected`, `ErrDeleted`, `ErrOverflow` or `ErrHandlerClosed`) and wait up to
       `Grace`;
    3. close its pipes with that cause and wait up to `Grace`; a `serve` still running is abandoned: logged, counted, and its
       goroutine left, as `InProcess` leaves it (0042-MADR-in-process-agent-runner.md, item 1).
  * A `serve` that returns first closes the socket with a normal closure after its last line is sent. Its error, if any, is
    logged under the privacy rule.
  * `Close(ctx)` refuses new connections with `503`, runs the ladder on every connection at once, and waits for them, or for
    `ctx`. It returns `ErrAbandoned`, joined with the count, when any serve was abandoned, and `context.Cause(ctx)` when `ctx`
    ended first. A second `Close` waits for the first.
* **Logging:** an `slog` record per refused request, per serve error and per abandonment, with fixed messages and byte counts,
  never a body, an id, a session id or a close reason (the MADR's amendment, 1).

Tests:

| Test | Required |
| :--- | :--- |
| `TestWebSocketRoundTrip` | `Dial` to `NewHandler(ServeAgent(ScriptAgent))`, an `acptest.MemoryClient`: `initialize`, `session/new`, a prompt with two updates, `end_turn` |
| `TestWebSocketPermission` | a test agent's `session/request_permission`, which `ScriptAgent` does not send, answered by a `MemoryClient`, and the prompt completed |
| `TestWebSocketCancel` | a `WaitCancel` turn ended by `session/cancel`, answered `cancelled`; a prompt ended by `$/cancel_request`, answered `stopReason` `cancelled`; and a test agent's `session/set_mode` that waits for its context, ended by `$/cancel_request` and answered `-32800` (`README.md:575`–`:584`) |
| `TestWebSocketLargeMessage` | a 64 KiB prompt and an update at `MaxMessageSize` arrive; one byte past it ends the connection, on the side that received it |
| `TestWebSocketConnectionID` | the `101` carries `Acp-Connection-Id`: at least 26 characters of the RFC 4648 base32 alphabet, as `crypto/rand.Text` gives, and distinct per connection |
| `TestWebSocketBinaryIgnored` | a binary frame from a raw `websocket.Dial` is ignored and logged once; a text frame after it is served |
| `TestWebSocketOrigin` | a handshake with a foreign `Origin` refused; allowed with `OriginPatterns` |
| `TestWebSocketHTTP2Client` | `Dial` with a caller's transport that would negotiate HTTP/2 completes the handshake |
| `TestWebSocketCookies` | a cookie the server sets on the handshake is sent on a second `Dial` that shares the `HTTPClient` |
| `TestHandlerLadderInputEnd` | a serve that returns at its input's end: a client's `Close` ends it in step 1, its cause unset |
| `TestHandlerLadderCause` | a serve that ignores its input and returns at its context's end sees `ErrDisconnected` |
| `TestHandlerLadderAbandon` | a serve that ignores everything until released: `Close` returns `ErrAbandoned` within three graces and a slack; the test then releases it, so the leak guard passes |
| `TestHandlerClose` | `Close` with two live connections ends both with `ErrHandlerClosed`; a handshake after it is `503` |
| `TestHandlerServeReturnsFirst` | a serve that returns after its last update: the client reads the update, then `io.EOF`, and its connection's end is `acp.EndedCleanly` |
| `TestWebSocketKeepAlive` | with `KeepAlive` 50 ms, a peer that stops answering pings is ended with `ErrDisconnected` |
| `TestWebSocketTranscript` | an `agentproc.NewTranscript` as both taps of a client over `Dial`, read back with `acptest.ReadRecords` and passed through `acptest.ValidateGolden` |

Planted breaks:
* `SetReadLimit` removed, so `TestWebSocketLargeMessage` fails at 32 KiB;
* the reader's context made per read, so the connection closes after the first read;
* `Acp-Connection-Id` set after `Accept`;
* step 2's cause dropped;
* the abandonment bound removed;
* the HTTP/1 clone removed, if P1 showed it needed;
* the ping loop never started.

### 5. The Streamable HTTP client

`httpclient.go`, behind `Conn` for `http://` and `https://`. `Dial` makes no request; the first `Write` does. The steps follow
the RFD's flow diagram and K3's rules:

1. The first line must be an `initialize` request, or `Write` fails and the `Conn` ends. It is POSTed with `Content-Type:
   application/json` and no connection header. The answer is read as follows:
   * A `200` with `Content-Type: application/json` and `Acp-Connection-Id` gives the first line read and the connection's id.
   * An answer whose body is a JSON-RPC response to the `initialize` id, but which carries no `Acp-Connection-Id`, gives that line
     to the reader and then ends the `Conn`. This covers Rust's `200` with the agent's error, and TypeScript's and this
     package's `500` with `-32603` (the MADR's K3 and amendment, 9). So the connection's `Initialize` returns the agent's error
     rather than a transport error.
   * Any other answer ends the `Conn` with an error naming the status and at most 512 bytes of the body.
2. The connection-scoped stream is opened: `GET` with `Accept: text/event-stream` and `Acp-Connection-Id`.
3. Each later line is POSTed, one at a time in the order written, each waiting for its `202`, so the server receives what the
   connection wrote in order. On every request after `initialize` the client sends `Acp-Connection-Id`, `Acp-Protocol-Version: 1`
   and `DialOptions.Header`. It sends `Acp-Session-Id` when:
   * the line is a request or notification whose method is session-scoped or whose params carry `sessionId`, and it is not a
     `$/` message;
   * the line is a response to the server's request, and that request carried `params.sessionId`. The client records each
     received request's session by its id key for this.

   A POST answered `404` while it carried `Acp-Session-Id` is sent once more without it, and that answer stands. This is Python's
   answer for a session it has not seen (the MADR's amendment, 5, and P13).
4. A received response whose `result.sessionId` names a session with no stream opens that session's stream before the response
   is delivered. A `session/load` or `session/resume` request opens its session's stream before it is POSTed. If that GET is
   answered `404`, the client opens it again once the request's response has arrived (the MADR's amendment, 5).
5. Every stream's events feed one `lineQueue`. The reader merges them, with no order between streams, as the RFD gives none.
6. **Ends.** The connection-scoped stream's end, or a non-`202` to a POST, ends the `Conn`: the reader returns the error after
   the lines already queued. A session stream's end ends that stream only. `Close` sends `DELETE` with `Acp-Connection-Id`, under
   a bound of 5 s, closes every stream, and ends the reader with `io.EOF`.
7. HTTP versions: the caller's `HTTPClient`, as given (Q4 as decided). On `https://`, Go's transport negotiates HTTP/2 by ALPN; on
   `http://`, HTTP/1.1 unless the caller's transport sets `Protocols` to unencrypted HTTP/2.

`sse.go`'s reader follows the SSE format: lines end in LF, CR or CRLF; `data:` lines are joined with `\n` into one event; comment
lines (`:`) and the `event`, `id` and `retry` fields are ignored; an event over `MaxMessageSize` ends the stream with an error.
Each event's data goes through `toLine`.

Tests run against an `httptest` fake server in the test files. It plays the RFD's side and records each request it receives. It
has three modes, one per column of the MADR's K3 second table:
* `tsMode`: `400` without the header when `params.sessionId` is present; `session/load`'s answer on the connection stream;
* `rustMode`: the header or params; `session/load`'s answer on the session stream;
* `pyMode`: `404` for a header naming an unknown session, on POST and on GET; `session/load`'s answer and replay on the
  connection stream.

| Test | Required |
| :--- | :--- |
| `TestHTTPClientInitialize` | the first POST has no connection header; the `200` body is the first line read; the connection stream's GET follows with the id |
| `TestHTTPClientNotInitializeFirst` | a first line that is not `initialize` fails, and nothing is sent |
| `TestHTTPClientInitializeRefused` | a `200` with a JSON-RPC error and no id header, and a `500` with a `-32603` body: `Initialize` returns that `*acp.RequestError`, and the `Conn` ends; a `504` with a text body: a transport error naming `504` |
| `TestHTTPClientHeaders` | each kind of line gets the headers of step 3, `$/cancel_request` without `Acp-Session-Id`, and a permission answer with its request's session |
| `TestHTTPClientSessionStream` | the `session/new` response on the connection stream opens the session stream before it is read; updates on both streams all arrive |
| `TestHTTPClientLoadStreamFirst` | `session/load` opens its stream before its POST; a `404` to it is retried after the response |
| `TestHTTPClientLoadEachMode` | on a fresh connection, `session/load` and a prompt after it complete in each of the three modes: the replay and the answer read once, in order within each stream |
| `TestHTTPClientHeaderRetry` | in `pyMode`, the `404` POST is sent once more without `Acp-Session-Id`; a second `404` ends the `Conn`; in `tsMode`, no retry is made |
| `TestHTTPClientOrder` | 200 notifications written at once arrive at the fake server in order |
| `TestHTTPClientPostRefused` | a `400` to a POST ends the `Conn`, its error naming the status; the lines already received are still read |
| `TestHTTPClientStreamEnds` | the connection stream's end ends the `Conn`; a session stream's end does not |
| `TestHTTPClientClose` | `Close` sends `DELETE` with the id; `Read` returns `io.EOF` |
| `TestHTTPClientCookies` | a cookie set on the `initialize` answer is sent on every later request |
| `TestSSEReader` | LF, CR and CRLF endings; multi-line `data`; comments, `event`, `id` and `retry` ignored; an over-long event refused |
| `TestHTTPClientProtocols` | HTTP/1.1 on `http://`; HTTP/2 on an `httptest` TLS server with `EnableHTTP2`; unencrypted HTTP/2 when the caller's transport asks |

Planted breaks:
* `Acp-Session-Id` sent on `$/cancel_request`;
* the session stream opened after the `session/new` response is delivered;
* POSTs sent concurrently;
* the CR ending not handled;
* `Close` without `DELETE`;
* the header retry removed, so `pyMode`'s `session/load` ends the `Conn`.

### 6. The Streamable HTTP server

`httpserver.go`, reached from `ServeHTTP` for `POST`, `DELETE` and `GET` without `Upgrade`. Another method, `OPTIONS` included,
is `405`; a browser's CORS preflight is the host's middleware, as Rust's guide leaves credentialed CORS to the host. The checks
and their order are the RFD's routing tree. Where the RFD is silent, or where the official servers depart from it, the server
accepts what any official client sends (the MADR's amendment, 2):

* **POST:**
  * not `application/json`: `415`;
  * a body over `MaxMessageSize`, through `http.MaxBytesReader`: `413`;
  * not JSON: `400`; a JSON array: `501`; not an object: `400`;
  * an id or session id over 1024 bytes: `400`.
  * `initialize` with no connection header creates the connection, starts `serve`, and forwards the line. It then waits for the
    answer to its id, up to 30 s (the MADR's amendment, 9):
    * an answer with a `result`: `200`, the answer's line as the body, `Content-Type: application/json` and `Acp-Connection-Id`;
    * an answer with an `error`: `200` with that line, and the connection is ended through the ladder, with no
      `Acp-Connection-Id`;
    * `serve` returning or failing first: `500`, with a JSON-RPC `-32603` body for the request's id, and the ladder;
    * 30 s with no answer: `504`, and the ladder;
    * the request's context ending: the ladder, and nothing is written.
    `initialize` with a connection header is `400`.
  * Without a connection header: `400`. An unknown connection: `404`.
  * A request or notification takes its session from `Acp-Session-Id` or from `params.sessionId`, Rust's rule. A session-scoped
    method with neither: `400`. Both, differing: `400`. A session the connection has not seen is not refused: `session/load` and
    `session/resume` name sessions this connection has never had.
  * A response to the agent's request may omit `Acp-Session-Id`. When it is present and is not the session of that request: `400`.
  * Otherwise the line goes to `serve`'s input, and the POST is answered `202` once the pipe has taken it.
* **GET, SSE:**
  * no `text/event-stream` in `Accept`: `406`;
  * without a connection header: `400`; an unknown connection: `404`;
  * a stream that already has a receiver: `409`.
  * With `Acp-Session-Id`, the session's stream is opened, or created for a session not yet named, as TypeScript and Rust do. A
    provisional stream is closed when no `session/new` or `session/fork` result and no `session/load` or `session/resume` POST
    names it within `Grace`, or when that `session/load` or `session/resume` is answered with an error, as Python closes it.
  * A session stream stays open after its session's `session/close` or `session/delete`, until its client ends it or the
    connection ends. The RFD gives a session's end no stream rule.
  * A line written to a stream whose receiver has gone is lost, not held again: v1 does not replay in-flight messages (the RFD's
    "In-flight messages are not replayed"). A line routed after the receiver has gone is held for the next one.
  * The response sets `Content-Type: text/event-stream` and `Cache-Control: no-cache`, clears the write deadline (P5), and
    flushes. It sends what was held for the stream, then each new message as one `data:` event, flushed. It sends a `:` comment
    every `KeepAlive`.
* **DELETE:** without a connection header: `400`; an unknown connection: `404`; otherwise the ladder with `ErrDeleted`, and
  `202`.
* **Outbound router:** one goroutine per connection reads the agent's output through a `lineSplitter` and routes each line:
  * the `initialize` answer to its POST;
  * a request or notification with `params.sessionId`, not `$/`, to that session's stream;
  * an answer to a request that named a session, by header or params, goes to that session's stream. The exceptions are answers
    to `session/new`, `session/fork`, `session/load` and `session/resume`, which go to the connection stream, as TypeScript and
    Python send them (the MADR's design). A route is recorded by the request's id key when the POST is taken, and removed with
    its answer;
  * an answer the router has no route for, and everything else, to the connection stream.
* **Holding:** a line for a stream with no receiver is held for it. Over `MaxBuffered` bytes held on the connection, the
  connection ends with `ErrOverflow` and is not answered further.
* **The idle bound:** once no connection-scoped stream has been attached for 60 s, the connection ends with `ErrDisconnected`.
  The 60 s counts from `initialize` or from the last stream's end (the MADR's amendment, 3). A test sets it through an unexported
  field, as `agentproc` tests set `readHook`.
* **Origins:** POST, GET and DELETE apply the WebSocket upgrade's rule (P7, the MADR's amendment, 7). A request with no `Origin`
  passes; so does one whose `Origin` host equals the request's `Host`, or matches an `OriginPatterns` entry with `path.Match`,
  against the scheme and host when the entry holds `://`. Any other is `403`. The check runs before the routing tree.

Tests, through phase 5's client and through raw `http.Client` requests for what the client never sends:

| Test | Required |
| :--- | :--- |
| `TestHTTPServerRoundTrip` | phase 4's round trip, permission and cancel tests, rerun over `http://` with HTTP/1.1, over TLS with HTTP/2, and over unencrypted HTTP/2 |
| `TestHTTPServerStatuses` | each row of the routing tree: `415`, `413`, `400` (each cause), `501`, `404`, `405` (`PUT` and `OPTIONS`), `406`, `409` |
| `TestHTTPServerClientShapes` | a fresh connection's `session/load` and the prompt after it, POSTed as each official client sends them: with the header (TypeScript); without it, `params.sessionId` alone (Python, Rust); a response to a permission request with and without the header; a mismatched header `400` |
| `TestHTTPServerInitialize` | an answer with a result: `200` with the id; with an error: `200` without the id, and the connection gone; a serve that returns first: `500` with a `-32603` body; no answer within the bound, set through an unexported field: `504`; each ending the serve through the ladder |
| `TestHTTPServerRouting` | two sessions on one connection: each session's updates, requests and answers on its own stream; `session/new`'s answer on the connection stream; `$/cancel_request`'s answer on the connection stream |
| `TestHTTPServerHeld` | updates for a session whose stream opens late are held, and sent in order once it opens |
| `TestHTTPServerOverflow` | `MaxBuffered` 64 KiB and a stream never opened: the connection ends with `ErrOverflow`, and its serve sees that cause |
| `TestHTTPServerProvisional` | a session stream opened before `session/load` receives its replay; one never named is closed after `Grace`; one whose `session/load` is answered with an error is closed |
| `TestHTTPServerDelete` | `DELETE` ends the serve with `ErrDeleted`, closes the streams, and a later POST is `404` |
| `TestHTTPServerIdle` | with the bound at 100 ms, a connection whose stream is closed and not reopened ends with `ErrDisconnected` |
| `TestHTTPServerKeepAlive` | with `KeepAlive` 50 ms, an idle stream receives comments |
| `TestHTTPServerWriteTimeout` | under an `http.Server` with `WriteTimeout` 100 ms, a stream lives past it (P5) |
| `TestHTTPServerOrigin` | a POST, a GET and a DELETE with a foreign `Origin` are `403`; with `OriginPatterns` matching its host, served; with no `Origin`, served; and the same answers as `TestWebSocketOrigin`'s for the same headers |
| `TestHTTPServerTranscript` | phase 4's transcript test over Streamable HTTP |

Planted breaks:
* the `409` check removed, so a second receiver steals the stream;
* `session/new`'s answer routed to a session stream;
* held lines sent out of order;
* the overflow bound not applied;
* the idle bound not started;
* the write deadline left in place;
* the origin check skipped for `DELETE`;
* the header required whenever `params.sessionId` is present, TypeScript's rule, so the Python-shaped `session/load` is `400`;
* the `initialize` bound removed, so a silent agent holds the POST.

### 7. The interoperability check and `example/remote`

* **`example/remote/main.go`:**
  * `serve -addr ADDR` listens with an `http.Server` that sets `ReadHeaderTimeout`, as gosec's G112 requires, and serves an echo
    agent at `/acp` through `NewHandler` and `acp.ServeAgent`.
    * The agent advertises `loadSession`, and keeps each session's updates in memory, shared by every connection, as the
      TypeScript example server keeps `durableSessions`.
    * On an interrupt it calls `Handler.Close` and then `http.Server.Shutdown`, in that order. `Shutdown` waits for active
      connections, and an open SSE stream is one until the handler ends it.
  * `dial URL` runs `initialize`, `session/new` and a prompt of `hello`. It prints each `agent_message_chunk`'s text and then
    `stop reason: <reason>`.
    * When `initialize` advertised `loadSession`, it closes the `Conn` and dials again. On the new connection it runs
      `initialize` and `session/load` for the saved session, and prints `loaded: <n> updates`.
    * It exits 0 on `end_turn`, and on a load that replayed at least one update.
  * `main_test.go` runs both modes in process over loopback, for `ws://` and `http://`, the load included.
* **`tools/interop.py [--ts REV] [--py REV]`:** the TypeScript revision defaults to `08a0d30`, the Python one to `9853d8b`.
  * It clones each SDK into the tool's cache directory and checks out the revision.
  * It runs `npm ci` in the TypeScript clone, and lets `uv run` resolve the Python examples' inline dependencies. *(Amended by
    phase 2: the inline dependencies name `agent-client-protocol[http]` from PyPI, not the pinned commit. So the tool makes a venv
    with `uv venv`, installs the clone with `uv pip install '.[http]' 'hypercorn>=0.17'`, and runs each example with that venv's
    `python`, as phase 2's probes did.)*
  * It stops, naming the command, when `node`, `npm` or `uv` is missing.
  * Each pair runs under a 60 s bound. Its output goes to `.cache/tools/interop/<pair>.log`, and the tool checks for the line
    each program prints on success:

  | Pair | Server | Client | Required line |
  | :--- | :--- | :--- | :--- |
  | ts-http | `npx tsx src/examples/http-server.ts`, `PORT` a free port | `example/remote dial http://127.0.0.1:PORT/acp` | `stop reason: end_turn`, then `loaded: 1 updates` (the example agent advertises `loadSession` and replays its history) |
  | ts-ws | the same | `example/remote dial ws://127.0.0.1:PORT/acp` | as ts-http |
  | py-http | `uv run examples/http_server.py` (`localhost:8000`) | `example/remote dial http://localhost:8000/acp` | `stop reason: end_turn` |
  | py-ws | the same | `example/remote dial ws://localhost:8000/acp` | `stop reason: end_turn` |
  | go-ts-http | `example/remote serve -addr 127.0.0.1:PORT` | `npx tsx src/examples/http-client.ts`, `ACP_HTTP_URL` set | `Done: end_turn` |
  | go-ts-ws | the same | `npx tsx src/examples/ws-client.ts`, `ACP_WS_URL` set | `Done: end_turn` (`ws-client.ts:98`) |
  | go-py-http | `example/remote serve -addr localhost:8000` | `uv run examples/http_client.py` | `stop reason: end_turn` (`http_client.py:48`) |
  | go-py-ws | the same | `uv run examples/ws_client.py` | `stop reason: end_turn` (`ws_client.py:49`) |

  The two Python lines print `result.stop_reason` as formatted text. If the first run shows the enum's name rather than
  `end_turn`, the required line is corrected in the tool and the record says so.

  * The Python examples hard-code `localhost:8000`, so the tool refuses to start when that port is taken. It serves the Go side
    on `localhost:8000`, and records whether `localhost` resolved to IPv4 or IPv6.
  * `tools/test_tools.py` gains offline tests: the pair table's commands, the success-line match, the free-port helper, and the
    missing-tool stop.
  * It ends through `_common.main_guard`, as `GuardTest` requires, and `tools/README.md` gains its row.
* **The run** on the owner's macOS host: all eight pairs pass. Each log's last lines are recorded. A pair that fails is a
  deviation, recorded with the RFD's text, both SDKs' behaviour and the resolution chosen with the owner.

Planted breaks, in copies of the tree, each run as the one pair that shows it:
* phase 6's `session/new` routing changed to the session stream, for go-ts-http;
* phase 5's `Acp-Session-Id` dropped from the prompt, for ts-http;
* phase 4's `Acp-Connection-Id` dropped from the `101`, for go-ts-ws.

`make check-tools` passes.

### 8. Docs

* `README.md`:
  * a "Remote agents" section after "Closing and limits". It covers:
    * `Dial` for both schemes, and `NewHandler` with `acp.ServeAgent`;
    * the options and their defaults, and the end causes;
    * the host's part: authentication as middleware; `Acp-Connection-Id` is not a credential (the RFD), so every request is
      authenticated, not only `initialize`; limits on how many connections a client may open; CORS for browsers; and
      `Handler.Close` before `http.Server.Shutdown`;
    * what is not supported, and the RFD revision.

    Its Go block is between `<!-- example: remote ExampleNewHandler -->` and `<!-- /example -->`;
  * "Explore the API": the agent's bullets gain `remote.NewHandler` for serving over the network, and the client's gain
    `remote.Dial`;
  * "Stability": `remote`, like `unstable`, is not subject to the compatibility promise, and `make api` lists its changes without
    gating them.
* `docs_test.go`: `TestReadmeRemoteIsTheExample`, the agentproc test's shape, reading `remote/example_test.go`.
* `docs/architecture.md`: package `remote` in "What it is"; a section on the seam, the two profiles, the outbound router, the
  ladder and the limits; `interop.py` in "Verification tools"; and the tree entries for `remote/` and `example/remote/`.
* Every exported name of `remote` has a doc comment (`revive`'s `exported`). Every doc is read against the code.

Planted break: the README's block changed by one line, failing `TestReadmeRemoteIsTheExample`.

### 9. Close the implementation

* `make api` against the newest `v1` tag: `remote`'s additions reported, nothing gated, the four gated packages unchanged.
* `tools/gates.py --all`.
* The ladder tests, `-count=20`, on macOS under `-race` and on both hosts.
* `README.md`, `docs/architecture.md` and `remote`'s package doc read in full against the code.
* The MADR's "Confirmation" walked item by item, each to the test or record that holds it.

### 10. The release, only on the owner's ask in that turn

As 0042's phase 11 ran:

* the release-notes report `0043-REPORT-release-notes-v1-27-0.md`, for `v1.27.0`, a minor release that adds a package;
* `tools/consumer.py` extended: its program serves a `ScriptAgent` through `remote.NewHandler` on loopback and dials it over
  `ws://` and `http://`. So the proxy's copy of `github.com/coder/websocket` is fetched with the checksum database on, at the floor.
  Its required output and its version floor move to `v1.27.0`, with `make check-tools`;
* the push and its CI, before the report cites it;
* the tag on the owner's ask, the GitHub release, `tools/publication.py`, and `tools/consumer.py v1.27.0`.

## Verification

* Every item of the MADR's "Confirmation" holds, each by its phase's tests:

  | Confirmation item | Where |
  | :--- | :--- |
  | doc comments and examples; the RFD revision in `go doc ./remote` | phases 3, 8 |
  | a round trip over each profile and each HTTP version | phases 4, 6 |
  | each status code of the RFD's table | phase 6 |
  | messages past 32 KiB, at and past the limit | phase 4, and phase 5 for SSE |
  | outbound routing, two sessions, `session/new`'s answer before its stream | phase 6 |
  | the endings, each bounded by `Grace` | phases 4, 6 |
  | a transcript validated with `ReadRecords` and `ValidateGolden` | phases 4, 6 |
  | `-race` on macOS, Linux and Windows, and the leak guard | every code phase's gates |
  | each new check failing on a planted break | every phase |
  | `make api`, `go mod tidy -diff`, `govulncheck` | every code phase's gates |
  | the client against each official server's mode; the handler against each official client's requests | phases 5, 6 |
  | interoperability with the TypeScript and Python SDKs | phase 7 |

* `make api` reports only additions, in `remote`. The root package, `agentproc`, `acptest` and `clienthost` show "0
  incompatible, 0 compatible".
* No generated file changes; `tools/gates.py regen` passes.
* `go.mod` requires `golang.org/x/sys v0.49.0` and `github.com/coder/websocket v1.8.15`, and nothing else.
* `go list -f '{{.ImportPath}} {{join .Imports " "}}' ./...`, its output read by a Python script: `github.com/coder/websocket` is
  imported by `remote` alone. `example/remote` imports `remote`, not the library.
* `make disclosure-check` and `tools/idscan.py`: nothing found.

## Rollout and Rollback

* **Rollout.** Phases 3 to 8 are separate commits, so the seam lands without the dependency, and the dependency arrives with its
  first import. One minor release, `v1.27.0`, carries them. Each consumer adopts under its own records:
  * `<cli-b>`: its `exp/acpws` placeholder becomes a use of `remote`, or is removed;
  * magic-cli-remote: its next ACP-over-HTTP agent dials through `remote` instead of rebuilding `acphttp`. It already requires
    `github.com/coder/websocket v1.8.15`, so its module graph does not grow.
* **What a consumer meets on the release:** a new package, and one new requirement in its module graph, compiled only when it
  imports `remote`. Nothing else behaves differently.
* **Rollback before a tag:** revert the phase's commits, later phases first. Phase 6 reverts alone; phase 5 reverts with phase 6,
  whose tests use its client; phase 7 reverts alone. Reverting phase 4 removes the dependency with its last import, in the same
  commit.
* **Rollback after a tag:** `remote` is outside the compatibility promise, so a defect may be fixed by changing its API in a
  minor release, and the release notes say so. A fix that keeps the API ships in a patch.

## Execution record

### Phase 1, the records (2026-10-29)

The owner approved the PLAN with "committed, proceed" after `a3d3b99`. This PLAN is `in-progress`, and its row says so.
[0039-MADR-five-adopter-additions-on-schema-1-25-0.md](0039-MADR-five-adopter-additions-on-schema-1-25-0.md) gained "Amended by
0043-MADR-remote-http-and-websocket-transport.md, 2026-10-29". Its "Out of scope" entry for a WebSocket or Unix-socket transport
is annotated in place, and its row says "amended by 0043 2026-10-29".

### Phase 2, the probes (2026-10-29)

Nothing in the tree changed. The Go probes ran in a scratch module outside the tree, requiring `github.com/coder/websocket
v1.8.15`, under `GOTOOLCHAIN=go1.26.10`, the floor: `go test -v -count=1 .`, exit 0. The peer probes ran against clones in the
tools' cache directory:
* `agentclientprotocol/typescript-sdk` at `08a0d30f023bf86574d49d59c4154e44d4386f55`, after `npm ci`;
* `agentclientprotocol/python-sdk` at `9853d8b514d6340000bcbd01ec07808436db80cf`, installed into a venv with
  `uv pip install '.[http]' 'hypercorn>=0.17'`.

Each probe is a raw HTTP or WebSocket exchange from a scratch Python script, run with that venv's `python`. Afterwards
`tools/procs.py http-server.ts http_server.py hypercorn` printed "none".

**Go and the WebSocket library**, each line as the probe logged it:

| Probe | Output | Against the PLAN |
| :--- | :--- | :--- |
| P1 | default (HTTP/2-capable) transport: `err=<nil> resp.Proto="HTTP/1.1"`; HTTP/1-only clone: `err=<nil> resp.Proto="HTTP/1.1"` | Go's transport sends the upgrade over HTTP/1.1 even with HTTP/2 enabled, so the default does not fail. See P1b. |
| P1b | `ws://` with an unencrypted-HTTP/2-only transport: `failed to send handshake request: … http2: invalid Upgrade request header: ["websocket"]`; `wss://` with an HTTP/2-only transport: `err=<nil> proto="HTTP/1.1"` | A caller's transport set for prior-knowledge HTTP/2, as Streamable HTTP over `http://` may use, breaks the handshake. Phase 4's HTTP/1 clone is needed, and its planted break is kept. |
| P2 | `status=101 Acp-Connection-Id="probe-conn"` | as assumed |
| P3 | read: `failed to get reader: context deadline exceeded`; write after it: `failed to write msg: use of closed network connection` | as assumed: one reader context per connection |
| P4 | no reader: `failed to ping: failed to wait for pong: context deadline exceeded`; with a reader: `err=<nil> in 0s` | as assumed |
| P5 | deadline not cleared: events `[0 1]`, `unexpected EOF`; cleared: `[0 1 2 3 4 5]`, `err=<nil>` | as assumed: `WriteTimeout` 100 ms ends an uncleared stream |
| P6 | `GET proto HTTP/2.0; POST codes [202 ×10]; distinct client addresses 1; protos map[HTTP/2.0:true]` | as assumed |
| P7 | no `Origin`: `101`; foreign: `403`; host pattern: `101`; `https://` pattern: `101` | as assumed |
| P8 | `cookie on the handshake: "node-7"` | as assumed |

**The peer servers:**

| Probe | TypeScript | Python | Against the MADR |
| :--- | :--- | :--- | :--- |
| P9 | session `GET` for a session never named: `200` | `404` | as K3 reads |
| P10 | `session/new`'s answer on the connection stream; a prompt's update and answer on the session stream; `session/load` on a fresh connection: its replay on the session stream, its answer on the connection stream | `session/new`'s answer on the connection stream; a prompt's update and answer on the session stream; `session/load`'s answers on the connection stream | as K3 reads; TypeScript's load matches the RFD's diagram |
| P11 | `101` carries `Acp-Connection-Id`; `initialize` as the first frame answered | the same | as assumed |
| P12 | `initialize` over HTTP/1.1: `200`, `Acp-Connection-Id` set; `text/plain`: `415`; batch: `501` | the same `200`; batch: `501`; `text/plain` with a JSON body: `415`; `text/plain` with a body that is not JSON: `400` | Python parses before it checks the media type. The handler checks the media type first, `415`, as the RFD's order puts it. No change. |
| P13 | fresh connection, `session/load` with the header: `202`; without: `400 Missing Acp-Session-Id` | with the header: `404 {"error":"Unknown session id"}`; without: `202`; session `GET` before any load: `404` | as the MADR's amendment, 2 and 5, read: a server must accept both shapes, and a client must retry without the header |

The first P13 run sent the headerless `session/load` before the one with the header. The headerless load named the session on
Python's connection, so the header was then accepted (`202`). The second run, with the header first on a fresh connection,
gives the table's answers. The first run is kept in the scratch log and is not evidence.

**Findings for later phases:**
* No result contradicts the MADR or its amendment, so the phase stands.
* P1's assumption was wrong in the safe direction. The clone stays, for P1b's reason.
* Phase 7's Python setup was amended: an inline-dependency `uv run` would test PyPI's release, not the pinned commit.

Checks: `make check-records`, `make markdownlint`, `make disclosure-check`; the identifier scan over the changed records.

### Phase 3, the seam and the skeleton (2026-10-29)

Approved with "committed, proceed" after `36e2c45`. No dependency was added: `go.mod` is unchanged.

**Files:**
* `remote/doc.go`: the package doc, naming the RFD, its revision of 2026-07-02, the specification commit `2bb9fac`, the two
  profiles and the seam, and that the package is outside the Go 1 compatibility promise. It names no function yet; phases 4 to 6
  add what they build, and phase 8 reads it whole.
* `remote/errors.go`: `ErrDeleted`, `ErrDisconnected`, `ErrHandlerClosed`, `ErrOverflow`, `ErrAbandoned`.
* `remote/frame.go`: `lineSplitter` (`newLineSplitter`, `Write`, `Close`), `toLine` and `lineQueue` (`push`, `end`, `Read`).
  * `lineQueue.push` waits while 256 lines are queued (`queueLimit`), so a reader that falls behind slows its transport.
  * `lineSplitter.Write` drops an over-long line, returns `errLineTooLong`, and sends the lines after it.
* `remote/route.go`: `kind`, `message`, `classify`, `sessionIDOf`, `connectionScoped`, `sessionScopedMethods` and
  `sessionScoped`, and `message.session`. `"session/fork"` is spelled out, for SA1019.
* `remote/leak_test.go`, `remote/frame_test.go`, `remote/route_test.go`.
* `scripts/check_api.py`: `REPORTED = ("unstable", "remote")`. `scripts/test_check_api.py`:
  `test_remote_is_reported_and_not_gated`.

The 10 MiB default message size (`defaultMaxMessageSize`) moved to phase 4. In this phase nothing used it, and the linter's
`unused` reported it.

**Tests**, the PLAN's five and `TestLineQueue`, added for `lineQueue`, which the PLAN names and gives no test:

| Test | Result |
| :--- | :--- |
| `TestLineSplitter` | pass |
| `TestToLine` | pass |
| `TestLineQueue` | pass: order across one-byte reads; `end`'s error after the queued lines; a refused late `push`; a `push` past 256 waiting for a read |
| `TestClassify` | pass, with the ids `1`, `"a b"` and `2.50`, a `$/cancel_request` carrying `sessionId`, array params, a non-string `sessionId`, a batch, `"method": null` and text that is not JSON |
| `TestSessionScoped` | pass |
| `TestIDKeyRoundTrip` | pass: `acp.ServeAgent` with an `acptest.ScriptAgent` answered the ids `7` (pretty-printed request), `"a b"` and `123456789012345678901234567890` with the same bytes |

**Planted breaks**, `python3 tools/plants.py` over seven plants, "7/7 as required". Each failing line:

| Plant | Failing line |
| :--- | :--- |
| line join removed | `frame_test.go:33: lines sent = [":1}" "x" "y"], want ["{\"a\":1}" "x" "y"]` |
| `toLine` not compacting | `frame_test.go:79: toLine("{\n  \"id\": 1, …}") = "{   \"id\": 1,   \"method\": \"x\" }\n", want "{\"id\":1,\"method\":\"x\"}\n"` |
| `$/` treated as session-scoped | `route_test.go:48: a $/ notification's session = "s1", want none: it is connection-scoped` |
| answers not keyed (the deviation below) | `route_test.go:105: request id key "7", answer {"jsonrpc":"2.0","id":7,…}: …want a response keyed "7"` |
| `lineQueue` ending before its queued lines | `frame_test.go:98: read = "", <nil>, want the two lines and io.EOF` |
| `lineQueue.push` not waiting at its limit | `frame_test.go:109: a push past queueLimit returned before any read` |
| a goroutine leaked by a test | the guard's profile, `remote.TestToLine.func1+0x23 … frame_test.go:73` |

**Gates:**
* `ACP_LINUX_HOST` and `ACP_WINDOWS_HOST` set, `tools/gates.py --all`: all 18 gates `ok`, "all gates passed".
* Precheck: "253 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck)".
* `remote`'s tests passed at the floor (`go1.26.10`, `ok github.com/maccavelli/acp-go-sdk2/remote 4.184s`), on the Windows host
  (`0.576s`) and on the Linux host (`0.587s`).
* `golangci-lint` for `darwin` and `windows` over `./remote/...`: 0 issues.
* `make check-tools`: OK.
* `make api`: `remote`'s five errors listed as "compatible: remote: … added", then "against v1.26.0: 0 incompatible, 5
  compatible" and "reported packages unstable, remote: 0 incompatible, not gated". The script's summary counts the reported
  package's additions. The four gated packages have none.

#### Deviation, 2026-10-29, phase 3's id-key break

* **Found.** The planted break "the id key taken before compaction" cannot fail, and the compaction it would remove does
  nothing. `classify` decodes `id` into a `json.RawMessage`, which holds the member's value bytes without the whitespace around
  them. A JSON-RPC id is a scalar, and a scalar has no whitespace inside.
* **Evidence.** A scratch probe, `TestRawID`, decoded `7`, `"a b"`, `123456789012345678901234567890` and `2.50e3`, each set off
  by spaces, tabs and newlines in a pretty-printed object. It logged `equal=true` for the raw and compacted bytes of each. Not
  pre-existing: phase 3's own code.
* **Resolution.** The owner chose "Substitute, plus a numeric-form check" from three options, the others being the substitute
  alone and a revert to compaction. Applied:
  * `classify` takes the id's bytes as the key, without `json.Compact`, with a comment saying why. The route step above is
    annotated.
  * The planted break is replaced by "answers not keyed": `classify` clears the key of a response. It fails `TestIDKeyRoundTrip`
    as shown. It breaks what the test exists to catch, a request and its answer keying differently.
  * The original break is struck through, not removed.
  * The numeric-form check, `TestIDKeyBytesExact`: the answer to id `1.0` keys as the request with id `1.0` does, and the answer
    to id `1` keys differently. `TestIDKeyRoundTrip` gained a fourth request, `session/new` with id `1.0`. Its pass shows that the
    agent connection answers `1.0` with `1.0`, so byte-exact keys are what routing needs. Its planted break, "id keys that
    normalize a number" (`strings.TrimSuffix(string(env.ID), ".0")`), failed as required: `route_test.go:90: the answer to id 1
    keys "1", as the request with id 1.0 does: want byte-exact keys, which differ`.
  * After the change: both linters 0 issues; `go test -race -count=3 -shuffle=on ./remote/` ok; `tools/gates.py precheck newfiles
    idscan windows linux` all passed, `remote` ok on the Windows host (`0.578s`) and the Linux host (`0.571s`).
* **The alternative not taken:** a revert to compaction would leave a check that cannot fail standing in the record as seen to
  fail, against `AGENTS.md`'s rule.
* **No MADR change:** the MADR asserts no compaction of the id.

### Phase 4, WebSocket, client and server (2026-10-29)

Approved with "committed, proceed" after `924c12a`.

**The dependency.** `go get github.com/coder/websocket@v1.8.15`, then `go mod tidy`. `go.mod` gains `require
github.com/coder/websocket v1.8.15`, and `go.sum` its two lines. `ws.go` and `handler.go`, its first imports, are in the same
change, as `AGENTS.md`'s "Dependencies" requires. `go mod tidy -diff`: clean.

**Files:**
* `remote/client.go`:
  * `Dial`, `DialOptions` (`HTTPClient`, `Header`, `MaxMessageSize`) and `Conn` (`Read`, `Write`, `Close`) over the unexported
    `transport`;
  * `resolveDialOptions`: a copy of the caller's client, a fresh `cookiejar` when its `Jar` is nil, and 10 MiB;
  * `defaultMaxMessageSize`, moved here from phase 3;
  * `errHTTPNotYet` for `http://` and `https://` until phase 5.
* `remote/ws.go`:
  * the client side: `wsClient`, `dialWebSocket`, `http1Only` and `readLoop`;
  * the server's per-connection parts: `runWebSocket`, `readWebSocket`, `pumpWebSocket`, `keepAlive` and `pump`;
  * `socketEnd`: a normal closure is `io.EOF`.
* `remote/handler.go`: `HandlerOptions`, `Handler`, `NewHandler`, `ServeHTTP`, `Close`, `serverConn`, and the ladder in
  `serverConn.ladder`. Other methods are answered `501` until phase 6.
* `remote/doc.go`: a paragraph on `Dial`, `NewHandler`, `Handler.Close` before `Shutdown`, and the host's part.
* `remote/helpers_test.go`, `remote/ws_test.go`, `remote/handler_test.go`.

**How the server ends a connection.** Every end runs one ladder. `runWebSocket` waits for it, then for the pump, then closes
the socket and stops the keepalive:
* the socket's end, from the reader, starts it with `ErrDisconnected`;
* a ping not answered within `KeepAlive` starts it with `ErrDisconnected`;
* `Handler.Close` starts it with `ErrHandlerClosed`;
* a serve that returns first ends its output. The pump then sends the last line, closes the socket normally, and the reader's
  end starts the ladder, which finds the serve returned in step 1.

The serve's goroutine is outside `Handler.active`, as `InProcess` keeps it outside `Process`'s group, so that `Close` does not
wait for an abandoned serve.

**Tests**, the PLAN's sixteen, with `TestDialSchemes` and `TestHandlerRefusals` added:

| Test | Result |
| :--- | :--- |
| `TestWebSocketRoundTrip`, `TestWebSocketPermission` | pass |
| `TestWebSocketCancel` | pass: `session/cancel` and `$/cancel_request` (sent by `Connection().SendNotification` with the second prompt's recorded id) each stop a `WaitCancel` turn `cancelled`; a cancelled `SetSessionMode` is answered `-32800` on the wire and `$/cancel_request` was sent |
| `TestWebSocketLargeMessage` | pass: 64 KiB both ways under a 128 KiB limit; on the server, a frame at the limit read and one byte past it ending the connection; on the client, a line at its limit read and one byte past it ending `Read` with an error |
| `TestWebSocketConnectionID`, `TestWebSocketBinaryIgnored`, `TestWebSocketOrigin` (`403`, then `101` with `OriginPatterns`) | pass |
| `TestWebSocketHTTP2Client` | pass, through a caller's transport limited to unencrypted HTTP/2 |
| `TestWebSocketCookies` | pass: the handshakes' cookies `["" "node-7" "" ""]`, a shared `Jar` carrying it and two default clients each keeping their own |
| `TestHandlerLadderInputEnd`, `TestHandlerLadderCause`, `TestHandlerLadderAbandon`, `TestHandlerClose`, `TestHandlerServeReturnsFirst`, `TestWebSocketKeepAlive`, `TestWebSocketTranscript` | pass |
| `TestDialSchemes`, `TestHandlerRefusals` | pass |

`go test -race -count=3 -shuffle=on ./remote/`: ok. The ladder tests (`TestHandlerLadder…`, `TestHandlerClose`,
`TestWebSocketKeepAlive`, `TestHandlerServeReturnsFirst`), `-count=20`: macOS under `-race` ok in `104.190s`; `tools/remote.py
pkg` on the Linux host and on the Windows host, each exit 0.

**Planted breaks**, `python3 tools/plants.py` over eight plants, "8/8 as required":

| Plant | Failing line |
| :--- | :--- |
| `SetReadLimit` removed from `Dial` | `ws_test.go:181: a 64 KiB prompt and a 64 KiB update: {"code":-32603,…"connection ended before response"}` |
| `SetReadLimit` removed from the handler | the same line |
| the client's read context made per read | `ws_test.go:38: Initialize = {"code":-32603,…"connection ended before response"}` |
| `Acp-Connection-Id` set after `Accept` | `ws_test.go:245: Acp-Connection-Id = "", want 26 or more base32 characters, new each time` |
| step 2's cause dropped | `handler_test.go:77: the serve's cause = context canceled, want ErrDisconnected` |
| the abandonment bound removed | `handler_test.go:107: Close = context deadline exceeded, want ErrAbandoned` |
| the HTTP/1 clone removed | `ws_test.go:321: Dial(ws://…) = … http2: invalid Upgrade request header: ["websocket"]` |
| the ping loop never started | `ws_test.go:395: a client answering no ping was not ended` |

The two read-limit plants were run again after the client-side at-limit case was added to `TestWebSocketLargeMessage`: "2/2 as
required".

**Gates:**
* `tools/gates.py --all`: all 18 `ok`.
* Precheck: "260 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck)".
* `remote`'s tests: ok at the floor (`7.514s`), on the Windows host (`5.659s`) and on the Linux host (`5.683s`).
* After the last test and doc edits, `tools/gates.py precheck newfiles idscan lint windows linux`: all `ok`. `remote` was ok on
  Windows (`5.661s`) and Linux (`5.634s`), and the precheck was again "260 file(s) clean".
* `make api`: `remote`'s eleven names "added", then "0 incompatible, 11 compatible" and "reported packages unstable, remote: 0
  incompatible, not gated". The four gated packages have none.

**Observations:**
* A WebSocket's graceful close after its serve has returned waits for the client's answer. `websocket.Conn.Close` bounds that
  at 5 seconds of its own. `TestHandlerClose`, whose raw clients never read, takes 5.1 s for that reason. `Handler.Close`'s doc
  says so.
* The server's writes are bounded by `Grace`, as the PLAN says. A message that takes longer than `Grace` to write, such as
  10 MiB at under 2 MiB/s with the default 5 s, ends its connection. A host that sends large messages over slow links raises
  `Grace`.

#### Deviation, 2026-10-29, phase 4's client write bound

* **Found.** Phase 4 says "a write sends each line as one text frame, without its newline, under a bound of `Grace`", of
  `ws.go`'s both sides. `Grace` is a `HandlerOptions` field. `DialOptions` has none, by the MADR's design and its amendment 1
  ("`Conn` has no method beyond `Read`, `Write` and `Close`").
* **Built.** The client's writes run under the connection's own context, which `Conn.Close` ends. The server's writes are
  bounded by `Grace`, as written.
* **Resolution.** The owner chose "Connection's life" from three options, the others being a fixed 5 s bound per message and a
  new `DialOptions.WriteTimeout`. So:
  * a client's write runs until it completes or the connection ends, as a write to a child's stdin does;
  * a stuck server holds it until the caller closes the `Conn`;
  * a large message on a slow link is not ended by the transport;
  * no exported name changes.

  The PLAN's step is annotated in place.
* **Why not the others.** A fixed bound would end a connection whose message takes more than 5 s to send, with no way for the
  caller to raise it. A new option is API that the MADR's amendment 1 did not list.
* **No MADR change:** the MADR names no bound on a client's writes.

### Phase 5, the Streamable HTTP client (2026-10-29)

Approved with "committed proceed" after `9e8def2`.

**Files:**
* `remote/sse.go`: `sseReader` with `next` and `readLine`.
  * Lines end in LF, CR or CRLF. `data:` lines are joined with `\n`.
  * Comments and the `event`, `id` and `retry` fields are ignored.
  * An event over `MaxMessageSize`, or a line over it and the field's name, is `errLineTooLong`. A last line with no ending is
    no event.
  * The writer is phase 6's.
* `remote/httpclient.go`:
  * `httpClient` and the steps of the PLAN's phase 5 in `post`, `sessionOf`, `initialize`, `openStream`, `readStream` and `do`;
  * `refused` and `refusedText`, quoting at most 512 bytes;
  * `drain`, `isType`, and the `Acp-Session-Id`, `Acp-Protocol-Version`, media-type and `deleteBound` constants.
* `remote/client.go`:
  * `Dial` returns the Streamable HTTP `Conn` for `http://` and `https://`, and sends nothing until the first `Write`;
  * phase 4's `errHTTPNotYet` is gone;
  * `Conn.Close`'s doc names the `DELETE` and its 5 s bound.
* `remote/fakeserver_test.go`: the PLAN's fake, in `tsMode`, `rustMode` and `pyMode`, with K3's second table's rules.
  * It routes on POST and GET, holds lines for streams with no receiver, and records every request: method, protocol, the
    three ACP headers, the cookie, `X-Test`, the body and the status.
  * Hooks: `initFail`, `failNext`, `setCookie`, `getDelay`, `forget`, `closeStream`.
  * Its `loadAgent` advertises `loadSession` and replays one update.
* `remote/httpclient_test.go`. `remote/ws_test.go`'s `TestDialSchemes` now requires `http://` to make no request until a
  `Write`.

**One choice made in the code.** The connection stream's clean end, the SSE body's `io.EOF`, ends the `Conn` with `io.EOF`.
So the client's connection ends cleanly (`acp.EndedCleanly`), as at a WebSocket's normal closure or the end of a child's
output: a server ends its streams so when its agent's serve has returned. Any other end of the stream ends the `Conn` with that
error. The PLAN's step 6, "The connection-scoped stream's end … ends the `Conn`", holds. `TestHTTPClientStreamEnds` first
required an error naming the stream's end; it now requires the clean end its doc gives the reason for.

**Tests**, the PLAN's fifteen:

| Test | Result |
| :--- | :--- |
| `TestHTTPClientInitialize`, `TestHTTPClientNotInitializeFirst` | pass |
| `TestHTTPClientInitializeRefused` | pass: a `200` with the agent's error and no id, and a `500` with `-32603`, reach `Initialize` as `*acp.RequestError` `-32603`; a `504` as an error naming it; each ends the connection |
| `TestHTTPClientHeaders` | pass, in `tsMode`, which refuses a response to the agent's session request without its session: the id, `Acp-Protocol-Version: 1` and `X-Test` after `initialize`; the session on `session/prompt`, the permission answer and `session/cancel`; none on `session/new` or on a `$/cancel_request` whose params name a session |
| `TestHTTPClientSessionStream` | pass: with the session `GET` delayed 200 ms, the stream is attached when `NewSession` returns |
| `TestHTTPClientLoadStreamFirst` | pass: the first session `GET` before the first `session/load` POST in `tsMode` and `pyMode`; in `pyMode`, `404`, then `200` after the answer |
| `TestHTTPClientLoadEachMode` | pass: in each mode, the updates `replayed,after load` and `end_turn` |
| `TestHTTPClientHeaderRetry` | pass: in `pyMode`, `session/load` POSTed `404` with the header then `202` without; after the fake forgets the connection, a prompt's two `404`s end it; in `tsMode`, one POST |
| `TestHTTPClientOrder` | pass: 200 notifications in order, the fake answering each after 0–2 ms |
| `TestHTTPClientPostRefused` | pass: `Write` answered `400` returns an error naming it; `session/new`'s answer, queued before, is read; then the error |
| `TestHTTPClientStreamEnds`, `TestHTTPClientClose`, `TestHTTPClientCookies`, `TestSSEReader` | pass |
| `TestHTTPClientProtocols` | pass: every request `HTTP/1.1` on `http://`, `HTTP/2.0` over TLS with `EnableHTTP2`, and `HTTP/2.0` unencrypted when the caller's transport asks |

`go test -race -count=3 -shuffle=on ./remote/`: ok. The client's tests, `-race -count=10 -shuffle=on`: ok.

**Planted breaks**, `python3 tools/plants.py` over six plants, "6/6 as required":

| Plant | Failing line |
| :--- | :--- |
| `Acp-Session-Id` sent on `$/cancel_request` | `httpclient_test.go:164: the POST holding "$/cancel_request" = [{… session:session-1 …}]` |
| the session stream opened after the `session/new` answer is delivered | `httpclient_test.go:181: NewSession returned before the session's stream was open` |
| POSTs sent concurrently | `httpclient_test.go:312: notification 0 to arrive = {…"n":13}, want n 0` |
| the CR ending not handled | `httpclient_test.go:455: events = ["one" "twodata:threedata: four"], want "one\|two\nthree\|four"` |
| `Close` without `DELETE` | `httpclient_test.go:410: requests = […], want a DELETE with conn-1` |
| the header retry removed | `httpclient_test.go:274: py: LoadSession = {"code":-32603,…"remote: POST answered 404: "}` |

**Gates:**
* `tools/gates.py --all`: all 18 `ok`.
* Precheck: "266 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck)".
* `remote`'s tests: ok at the floor (`8.518s`), on the Windows host (`5.666s`) and on the Linux host (`5.845s`).
* `golangci-lint` for `darwin` and `windows`: 0 issues, after one `//nolint:bodyclose` on the retried POST, which `refused` or
  `drain` closes.
* `make api`: "0 incompatible, 11 compatible", the same eleven names as phase 4. Phase 5 adds no exported name.

### Phase 6, the Streamable HTTP server (2026-10-29)

Approved with "committed, proceed" after `7e57c81`.

**Files:**
* `remote/httpserver.go`:
  * `httpConn` and `sseStream`;
  * `servePost` and `admit`, Rust's session rule;
  * `serveInitialize` and `fail`;
  * `run`, `check` and `closeStreamLocked`;
  * `route`, `dispatch`, `isErrorAnswer` and `settleAttach`;
  * `serveStream`, `attach`, `detach` and `stream`;
  * `serveDelete`, and `maxIDLength`, 1024.
* `remote/handler.go`:
  * `ServeHTTP` routes `POST`, `GET` and `DELETE`, after `originAllowed`, which copies `websocket.Accept`'s rule (`accept.go:228`–`:264`, with its lower-casing `path.Match`); any other method is `405` with `Allow`;
  * the handler gains `httpConns`, and the unexported `initBound` (30 s) and `idleBound` (60 s) that the tests shorten.
* `remote/sse.go`: `writeEvent` and `keepAliveComment` (`":\n\n"`, as Rust's empty comment).
* `remote/httpserver_helpers_test.go`:
  * `scripted`, a serve that answers each message as a test says, so routing is checked line by line;
  * `rawConn` and `rawStream`, for the requests the client never sends;
  * `countComments`.
* `remote/httpserver_test.go`. `remote/handler_test.go`'s `TestHandlerRefusals` now requires `405` and `Allow` for a `PUT`, in
  place of phase 4's interim `501`.

**How a connection runs.** `serveInitialize` starts the serve through phase 4's `Handler.start`, and `httpConn.run` owns the
connection from then on:
* the router reads the agent's output through phase 4's `pump`;
* a ticker, a quarter of the shorter of `Grace` and the idle bound and at least 10 ms, checks the idle bound and closes the
  provisional streams;
* when the ladder is done and the router has read to the end, `done` closes, and each stream sends what it holds, then
  ends.

Every end goes through phase 4's ladder: `DELETE`, the idle bound, an overflow, `Handler.Close`, the serve's own end and a failed
`initialize`. A connection is in `httpConns` only once `initialize` has a result, so every request before that is `404`.

**Tests**, the PLAN's fourteen:

| Test | Result |
| :--- | :--- |
| `TestHTTPServerRoundTrip` | pass, in three subtests: `http1`, `tls-h2` and `h2c`. The round trip, the permission, `session/cancel`, and `SetSessionMode` ended by `$/cancel_request`, its `-32800` read on the wire |
| `TestHTTPServerStatuses` | pass: `415`; `413` at a 1024-byte limit; `400` for not JSON, not JSON-RPC, an over-long id or header, no connection header, `initialize` on a connection, a session method with no session, a header differing from params, a GET with no connection header and a DELETE with none; `501` for a batch; `404` for an unknown connection on POST, GET and DELETE; `405` for `PUT` and `OPTIONS`; `406`; `409` for a second connection stream |
| `TestHTTPServerClientShapes` | pass: on fresh connections, `session/load` with the header and with `params.sessionId` alone, each `202`, its replay on the session stream opened before it and its answer on the connection stream; the prompt after it; the permission answer `202` with the header and without it, `400` with another session's |
| `TestHTTPServerInitialize` | pass: a result `200` with the id; an error `200` without it; a serve that returns first `500` with `"id":7,"error":{"code":-32603`; no answer in 100 ms `504`; the last three ending the serve |
| `TestHTTPServerRouting` | pass: two sessions, each prompt's update, permission request and answer on its own stream; both `session/new` answers and both `$/cancel_request` notifications naming a session on the connection stream |
| `TestHTTPServerHeld`, `TestHTTPServerOverflow`, `TestHTTPServerProvisional`, `TestHTTPServerDelete`, `TestHTTPServerIdle`, `TestHTTPServerKeepAlive` | pass |
| `TestHTTPServerWriteTimeout` | pass: an event 300 ms into a stream under a 100 ms `WriteTimeout` |
| `TestHTTPServerOrigin` | pass: a foreign `Origin` `403` on POST, GET and DELETE, and on the WebSocket upgrade; served with a host pattern and with a `https://` pattern; served with no `Origin` |
| `TestHTTPServerTranscript` | pass, over `http://` |

`go test -race -count=3 -shuffle=on ./remote/`: ok. The timing-sensitive tests (`…Idle`, `…KeepAlive`, `…WriteTimeout`,
`…Overflow`, `…Provisional`, `…Initialize`, `…Delete`, `…Held`, `…Routing`), `-count=20`: macOS under `-race` ok in `11.798s`;
`tools/remote.py pkg` on the Linux host and on the Windows host, each exit 0.

**One test fix, from a measurement.** `TestHTTPServerRoundTrip` first took 5.05 s in each subtest. A temporary
`zz_timing_test.go`, deleted after, timed each part at under 110 ms. The 5 s was the teardown: the test closed the connection
before the agent had read the `$/cancel_request`, so `acp.ServeAgent`'s 5 s handler drain waited for `SetSessionMode`'s handler,
as 0042's item 2 designed. The test now waits for the `-32800` answer on the wire, as `TestWebSocketCancel` does and as the
PLAN's row requires, and each subtest takes 0.04–0.06 s.

**Planted breaks**, `python3 tools/plants.py` over nine plants, "9/9 as required":

| Plant | Failing line |
| :--- | :--- |
| the `409` check removed | `httpserver_test.go:187: a second receiver for the connection stream = 200, want 409` |
| `session/new`'s answer routed to a session stream | `httpserver_test.go:333: no event within 10s` |
| held lines sent out of order | `httpserver_test.go:384: held event 0 = {…"id":101,"result":{"stopReason":"end_turn"}}, want n 0` |
| the overflow bound not applied | `httpserver_test.go:420: holding over MaxBuffered did not end the connection` |
| the idle bound not started | `httpserver_test.go:521: a connection with no stream was not ended` |
| the write deadline left in place | `httpserver_test.go:555: the stream ended before its next event` |
| the origin check skipped for `DELETE` | `httpserver_test.go:590: patterns [], origin "https://app.example": DELETE = 202, want refused true` |
| the header required whenever `params.sessionId` is present | `httpserver_test.go:223: params alone: session/load = 400, want 202` |
| the `initialize` bound removed | `httpserver_test.go:297: Post "http://127.0.0.1:…": context deadline exceeded` |

**Gates:**
* `tools/gates.py --all`: all 18 `ok`.
* Precheck: "270 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck)".
* `remote`'s tests: ok at the floor (`8.682s`), on the Windows host (`5.671s`) and on the Linux host (`5.798s`).
* `golangci-lint` for `darwin` and `windows`: 0 issues, after `errors.AsType` for the body limit's `*http.MaxBytesError`,
  `strconv.Itoa` and `t.Helper()` in the tests, and one `//nolint:bodyclose` on the raw stream, whose reading goroutine closes
  it.
* `make api`: "0 incompatible, 11 compatible". Phase 6 adds no exported name.
