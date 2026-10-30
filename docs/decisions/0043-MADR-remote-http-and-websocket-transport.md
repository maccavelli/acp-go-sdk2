---
status: accepted
date: 2026-10-29
decision-makers: the repository owner
consulted: the ACP specification and its RFDs; the official TypeScript, Rust, Python and Kotlin SDKs and the ACP registry, at the revisions named under "Sources"; both known consumers' code and records
informed: the consumers, through their own records
---

# Add ACP's remote transport, Streamable HTTP and WebSocket, as an experimental package, chosen over an MCP-over-ACP binding and a registry client

## Context and Problem Statement

The owner asked for one ACP feature or function this SDK does not implement, chosen from three assessed for their value, with a
record detailed enough to implement and to review.

**What "does not implement" means here.** The SDK generates every method of the pinned schema `1.25.0`, stable and unstable: the
13 agent methods, the 11 client methods and `$/cancel_request` of `schema/meta.json`, and the 28, 12 and 1 of
`schema/meta.unstable.json`. A gap is therefore not a missing method. It is behaviour the specification or its RFDs define that a
program cannot get from this module without writing it itself. The Completed RFDs the ACP site lists were checked against the
tree: their methods and types are generated, and those with behaviour beyond the types are covered (R5), but for the registry
(S8). Of the Active RFDs, the two that official SDKs ship are the other candidates (K1, K6).

**The question.** Of the three gaps that would bring most benefit (a remote transport, the MCP-over-ACP binding and a registry
client), which one should this SDK implement next, and in what shape?

**Status.** Accepted. The owner answered the five questions under "Questions for the owner" on 2026-10-29, each as recommended
("Decided with the owner, 2026-10-29"). Nothing is implemented. Writing the PLAN, and checking it against the official servers'
code, fixed seven details and changed two ("Amended by the PLAN, 2026-10-29").

### Findings in this repository

* **R1. The connection is a newline-framed byte stream.** `NewConnection(handler, peerInput io.Writer, peerOutput io.Reader, ...)`
  (`connection.go:666`) reads one JSON-RPC message a line (`readLine`, `:1300`), at most 10 MiB unless `WithMaxMessageSize` says
  otherwise (`:26`, `:534`). `NewClientSideConnection` (`client_gen.go:32`) and `NewAgentSideConnection` (`agent_gen.go:35`) take
  the same pair. A JSON array, a batch, is answered `-32600` with `"id": null` (`README.md:825`).
* **R2. Most of the SDK's runtime sits on that seam.** `acp.ServeAgent(ctx, agent, in, out, ...)` (`serve.go:29`);
  `agentproc.InProcess.Serve func(ctx, in io.Reader, out io.Writer) error` (`agentproc/inprocess.go:42`); the taps;
  `agentproc.NewTranscript` (`agentproc/transcript.go:34`); `acptest.Recorder`, `ReadRecords` (`acptest/records.go:17`) and
  `ValidateGolden`. An adapter that presents a remote profile as that byte stream gets all of them unchanged.
* **R3. No remote transport exists here.** The only transports are the caller's own reader and writer, a child's stdio
  (`agentproc.Command`) and two pipes (`agentproc.InProcess`, `acptest.Connect`).
  [0001-REPORT-go-127-acp-parity.md](../reports/0001-REPORT-go-127-acp-parity.md) records "Streamable HTTP is not implemented",
  and [0016-REPORT-acp-v1-conformance-audit.md](../reports/0016-REPORT-acp-v1-conformance-audit.md) did not audit it because it
  was a draft.
* **R4. Two records set it aside, for a reason this record answers.** [0035-REPORT-consumer-acp-code-for-the-sdk.md](../reports/0035-REPORT-consumer-acp-code-for-the-sdk.md),
  section F: "A WebSocket implementation needs a dependency, so it would need a MADR." [0039-MADR-five-adopter-additions-on-schema-1-25-0.md](0039-MADR-five-adopter-additions-on-schema-1-25-0.md),
  "Out of scope": "A WebSocket or Unix-socket transport (0035 F: needs a dependency, needs its own MADR)".
  [0023-MADR-connection-lifetime-and-test-leaks.md](0023-MADR-connection-lifetime-and-test-leaks.md), S4, declined an SDK
  keepalive because v1 has no ping and the transport RFD leaves liveness to the implementer.
* **R5. What the tree already covers, and so is not a candidate.** `$/cancel_request` (stable, `connection.go`); the reserved
  trace keys of the completed `_meta` propagation RFD (`acp.TraceMeta`, `extensions.go:139`); session compaction and notices
  (`compaction.go`, `helpers.go`); elicitation (`elicitation.go`); the terminal and file-system host (`clienthost`); every
  method of the completed session RFDs, generated.
* **R6. The MCP-over-ACP wire types are generated, frozen and deprecated.** `McpServerAcpInline` (`types_gen.go:8153`),
  `McpCapabilities.Acp` (`types_gen.go:7958`), `UnstableMessageMcpRequest` with `serverId` and `requestId` (`:20310`, `:20324`),
  the dispatch cases (`client_gen.go:211`, `agent_gen.go:418`) and the senders (`client_gen.go:606`, `agent_gen.go:954`). They
  are marked "Deprecated: … stays under v1 for compatibility only" by 0039. Nothing routes a `serverId`, tracks a `requestId` or
  joins an MCP library.
* **R7. The module has one dependency.** `require golang.org/x/sys v0.49.0` (`go.mod:9`), named by
  [0027-MADR-agent-process-helper.md](0027-MADR-agent-process-helper.md) on two grounds: both consumers already required it (C3
  there), and it declared `go 1.26.0`, under the floor (S8 there). `AGENTS.md`, "Dependencies": none may be added without a MADR
  that names it.
* **R8. An experimental package has a precedent.** Package `unstable` is "not subject to the Go 1 compatibility promise"
  (`unstable/doc.go`), and `scripts/check_api.py:49`–`:52` gates `"", "agentproc", "acptest", "clienthost"` and only reports
  `"unstable"`.

### Findings in the specification

Read at `agentclientprotocol/agent-client-protocol` `2bb9fac` and on agentclientprotocol.com, 2026-10-29.

* **S1. The stable v1 transport is stdio alone.** `docs/protocol/v1/transports.mdx`: "The protocol currently defines one transport
  mechanism … stdio", "A remote transport based on Streamable HTTP is being specified in the transport RFD", and custom
  transports "MUST ensure they preserve the JSON-RPC message format and lifecycle requirements".
* **S2. The remote transport is the RFD "Streamable HTTP & WebSocket Transport"** (`docs/rfds/streamable-http-websocket-transport.mdx`),
  authors alexhancock and jh-block, champion anna239. Its revision history ends "2026-07-02: Moved to Active to reflect current
  Transports Working Group focus", and the site's navigation (`docs/docs.json`) lists it under "Active"; the RFD updates page's
  last entry for it is the move to Draft. "Active" is "a visibility signal, not a stability commitment or final approval"
  (`docs/rfds/about.mdx`). It targets "v1 as an additive feature".
* **S3. It has changed shape four times in 2026**: a two-header identity model (04-01), a single long-lived GET stream (04-23), a
  split into connection-scoped and session-scoped streams (05-04), and a durability section (06-05).
* **S4. What it requires, in its own words:**
  * one `/acp` endpoint: `POST` sends a message, `initialize` answered `200` with a JSON body and `Acp-Connection-Id`, all else
    `202 Accepted`; `GET` with `Accept: text/event-stream` opens a connection-scoped stream, or a session-scoped one with
    `Acp-Session-Id`; `GET` with `Upgrade: websocket` upgrades; `DELETE` terminates;
  * "Clients that support remote ACP over HTTP MUST support both Streamable HTTP and WebSocket. This allows servers to support
    only WebSocket if they choose";
  * "Streamable HTTP transport MUST use HTTP/2";
  * "Clients MUST accept, store, and return cookies set by the server on all HTTP-based transports";
  * `415` for a POST that is not `application/json`, `406` for a GET without `text/event-stream`, `400` for a missing header,
    `404` for an unknown connection or session, `501` for a batch;
  * WebSocket: "All messages are WebSocket text frames containing JSON-RPC. Binary frames are ignored", and the client "must
    still send `initialize` as the first JSON-RPC message";
  * auth is "orthogonal and layered on top via HTTP headers, query parameters, or WebSocket subprotocols";
  * v1 leaves reconnect, retry and liveness to the implementer, and does not replay in-flight messages; `Last-Event-ID`
    resumption and a standard keepalive are deferred to v2; `Acp-Protocol-Version` is a SHOULD, "Not yet implemented".
* **S5. The RFD leaves points open.** Its `session/load` diagram sends the replay on the session stream and the response on the
  connection stream. The Python SDK sends both on the connection stream, and the Rust SDK sends the response on the session
  stream (K3). Its WebSocket section says the server "cleans up
  the connection and any associated sessions" on disconnect, while its v1 section says "Sessions survive disconnects". The name of
  the `initialize` body's connection field, `connectionId`, appears in a diagram only.
* **S6. The working group.** "Transports Working Group", published 2026-04-22: "Remote Agent support is a key focus of ACP, and in
  order to make this more of a reality, we need to standardize all of the approaches to transports people have been trying."
* **S7. MCP-over-ACP** (`docs/rfds/mcp-over-acp.mdx`, author nikomatsakis) declares a server `{"type": "acp", "name", "serverId"}`,
  advertises `mcpCapabilities.acp`, and carries each MCP operation as one `mcp/message` request with a caller-generated
  `requestId`, results under `result.result`, inner errors under `result.error`, notifications as `mcp/message` notifications,
  and cancellation as `$/cancel_request`. It "targets MCP 2026-07-28 only", replaced earlier connect/disconnect drafts that "are
  not compatibility commitments", and its stabilization gates say "The draft schema must be released and dependent SDK major
  versions coordinated before publication". The site's navigation lists it under "Active", as it does the transport.
* **S8. The ACP Registry** (`docs/rfds/acp-agent-registry.mdx`, author ignatov) is Completed, the one candidate here that is.
  `registry.json` at `https://cdn.agentclientprotocol.com/registry/v1/latest/registry.json` lists manifests whose `distribution` is
  `binary` (per-target `archive`, `cmd`, `args`, `env`, and in `agent.schema.json` an optional `sha256`), `npx` or `uvx`.
  Fetched 2026-10-29: `version` `1.0.0`, 43 agents, 23 with `npx`, 20 with `binary`, 2 with `uvx`; 65 of 107 binary targets carry
  a `sha256`.

### Findings in the official SDKs

Read at the revisions under "Sources".

* **K1. Every official SDK ships the remote transport, and marks it experimental.**
  * TypeScript `@agentclientprotocol/sdk` 2.1.0: `./experimental/http-client`, `./experimental/ws-client`, `./experimental/server`
    and `./experimental/node` (`package.json`); first released in 0.27.0, 2026-06-18, "Experimental Streamable HTTP & WebSocket
    Transport". `ws` is a development dependency only; the client takes a WebSocket constructor, `globalThis.WebSocket` by default.
  * Rust: the crate `agent-client-protocol-http` 3.6.0, `client` and `server` features off by default; first released 0.1.0,
    2026-06-18, "add HTTP/WebSocket transport support"; 3.0.0, 2026-10-06, was breaking (configuration, WebSocket handshake policy,
    connection drain).
  * Python `agent-client-protocol` 0.12.1: an optional `http` extra (`httpx2[http2]`, `websockets`, `starlette`); its
    `docs/web-transport.md` opens "**Experimental.** The remote web transports are experimental and may change."
  * Kotlin: `acp-ktor`'s `WebSocketTransport.kt` with client and server extensions on Ktor; the tree has no Streamable HTTP files.
* **K2. They share one shape: the transport adapts to the SDK's own stream of messages,** and the server takes an agent factory,
  one agent per connection: Rust `AcpHttpServer::new(|| my_agent())`, Python `create_asgi_app(lambda conn: MyAgent())`, TypeScript
  `createAgent`. Python: "Both use the existing `AgentSideConnection` and its message-level `Transport` interface."
* **K3. They route by the same rules.** Rust `protocol.rs` and TypeScript `protocol.ts` list the same session-scoped methods
  (`session/prompt`, `cancel`, `close`, `delete`, `fork`, `load`, `resume`, `set_config_option`, `set_mode`, `set_model`) and take
  the session from `params.sessionId`. Rust keeps `$/` messages connection-scoped, and rejects an `Acp-Session-Id` that differs from
  `params.sessionId`. Python's server table: the `initialize` response in the POST body; a response carrying a new `sessionId`, and
  `session/load` and `session/resume`, on the connection stream; others on their session's stream when known.

  They differ where a client meets a session the connection has not named. Read in `src/transports/server.ts` and
  `connection.ts`, `src/agent-client-protocol-http/src/http_server.rs` and `protocol.rs`, and `src/acp/http/server.py` and
  `protocol.py`:

  | | TypeScript | Rust | Python |
  | :--- | :--- | :--- | :--- |
  | session stream `GET` for a session not yet named | creates it (`acquireSessionStream`) | creates it (`subscribe_session_stream`) | `404` until `session/new`'s result or a `session/load` or `session/resume` POST names it |
  | a second receiver for a stream | `409` | `409` | no `409` in `server.py` |
  | the session of a session-scoped POST | `Acp-Session-Id` required for the ten methods and whenever `params.sessionId` is present | the header or `params.sessionId`, either; `400` when neither or when they differ | the header required for five methods (`session/prompt`, `cancel`, `close`, `set_mode`, `set_config_option`); `404` when the header names an unknown session |
  | a client's response to the agent's request | `Acp-Session-Id` must match the request's session | not checked | not checked |
  | `session/load`'s answer | the connection stream | the session stream | the connection stream |
  | `initialize` that fails | `500` with a `-32603` body, `499` when the request is aborted | the agent's error answer, without `Acp-Connection-Id` | `504` after 30 s (`INITIALIZE_TIMEOUT_SECONDS`), `500` on failure |
  | a POST body over the limit | — | `413` over 16 MiB | — |
  | the SSE keepalive | — | an empty comment every 15 s | `": keepalive"` every 15 s, "in sync with the TypeScript reference (15s)" |

  So no single client header rule satisfies every server. A `session/load` on a fresh connection needs `Acp-Session-Id` for
  TypeScript, and that header is a `404` for Python.
* **K4. Not all of them serve HTTP/2.** Rust's workspace builds `axum` with `features = ["http1", "tokio", "tracing",
  "tower-log"]` (`Cargo.toml:69`), and the 3.0.0 notes "Limit Axum to HTTP1"; its client applies "HTTP/1.1 and disables redirects"
  for WebSocket URLs. Python follows the RFD and says so: "For a spec-compliant Streamable HTTP server, run an HTTP/2-capable ASGI
  server … or terminate HTTP/2 at a proxy", since "Uvicorn does not serve HTTP/2". magic-cli-remote's goose spike found "goose
  v1.44.0 accepts **HTTP/1.1** on loopback" (C2). A Go client that forced HTTP/2 on `http://` would fail against the Rust server.
* **K5. They defer the same things:** batches (`501`), `Last-Event-ID`, automatic reconnect, `Acp-Protocol-Version` enforcement
  (Python's "Not yet supported" list); Rust caps a POST body at 16 MiB and does not negotiate WebSocket subprotocols or
  extensions.
* **K6. MCP-over-ACP is in two of the four.** TypeScript (`./experimental/mcp-server`, `./experimental/mcp-http`, with
  `@modelcontextprotocol/server` as an optional peer dependency) and Rust (`agent-client-protocol-rmcp`, the conductor's
  polyfill). Python and Kotlin have only the serializers.
* **K7. No official SDK has a registry client.** None of the four trees has a registry file but Rust's unrelated
  `mcp_server/registry.rs`.

### Findings in the consumers

Read at `<cli-b>` `ccd8aa8` and magic-cli-remote `0e84b1f0`. Nothing in either tree was written.

* **C1. `<cli-b>` reserved the package and deferred it.** `exp/acpws/doc.go`: "Package acpws is the experimental ACP WebSocket
  transport. … Phase 1 declares the package only." Its `0004-MADR-go-module-architecture.md` places it under `exp/` as "ACP over
  WebSocket / Unix socket (remote-agent transport)", guarded by `http.CrossOriginProtection`. Its `0005-PLAN-v1-feature-scope.md`
  lists it under `exp/`, where "Each item is off by default, enabled by a setting, and needs a later MADR to promote it".
* **C2. magic-cli-remote built one and removed it.** Its `0025-MADR-goose-provider.md` (implemented 2026-07-26) added
  `internal/provider/acphttp` for goose's "ACP over HTTP" (`goose serve`), on WebSocket, with `POST /acp` for `initialize` and
  `Acp-Connection-Id`. Its `0160-MADR-remove-goose-cli-support.md` (2026-09-18) removed Goose and the transport with it: 6,301
  lines with "no other importer", and "a future ACP-over-HTTP agent must rebuild `acphttp` from history". Its
  `0029-MADR-provider-platform-canonicalization.md` keeps `coder/websocket` direct for "daemon WS, relay splice, ACP-over-HTTP
  engine".
* **C3. magic-cli-remote already requires `github.com/coder/websocket v1.8.15`** and this module at `v1.26.0` (`go.mod`).
  `<cli-b>` requires this module at `v1.20.0` and no WebSocket module.
* **C4. Neither consumer wants MCP-over-ACP now.** magic-cli-remote's `0167-REPORT-acp-go-sdk2-integration-assessment.md`:
  `mcp/message` "decline (method-not-found) unless MCP-over-ACP is adopted". `<cli-b>` refuses a `type: "acp"` server
  (`acpserver/mcp_test.go:200`) and lists `mcpCapabilities.acp` as out of its v1 plan.
* **C5. Neither consumer reads the registry.** A scan of both trees for `registry.json`, `cdn.agentclientprotocol` and
  `agentclientprotocol.com/registry` found nothing.

### Findings in Go and its libraries

* **G1. The standard library covers all but WebSocket.** HTTP/2 over TLS is negotiated by ALPN; since Go 1.24, `Server.Protocols`
  and `Transport.Protocols` accept unencrypted HTTP/2 "with Prior Knowledge" (go.dev/doc/go1.24); SSE is a `text/event-stream`
  response and a line reader; `net/http/cookiejar` keeps cookies; `http.CrossOriginProtection` (Go 1.25), `http.MaxBytesReader` and
  `crypto/rand.Text` exist at the 1.26.10 floor. The MCP Go SDK v1.8.0 serves MCP's Streamable HTTP with none of
  its eight requirements a transport library (its `go.mod`).
* **G2. Go has no WebSocket in the standard library.** `golang.org/x/net/websocket` "currently lacks some features found in an
  alternative and more actively maintained WebSocket packages" and names `github.com/gorilla/websocket` and
  `github.com/coder/websocket`.
* **G3. `github.com/coder/websocket` v1.8.15** (2026-06-15): ISC licence; its `go.mod` declares `go 1.23` and requires nothing;
  "Zero dependencies", "First class context.Context support", "Concurrent writes", a ping API and a close handshake;
  `Accept(w, r, *AcceptOptions)` checks the origin by default, `OriginPatterns` widens it, compression defaults to disabled;
  `Dial` takes an `*http.Client` and handshake headers; "By default, the connection has a message read limit of 32768 bytes".
  On 2026-10-29, neither OSV (`api.osv.dev`, any version) nor the Go vulnerability database's module index lists a vulnerability for
  `github.com/coder/websocket` or its former path, `nhooyr.io/websocket`.
* **G4. `github.com/gorilla/websocket` v1.5.3** (2024-06-14): BSD-2-Clause; no release since.

## Decision Drivers

* **Benefit to the programs that use this SDK,** measured by what they had to write themselves (C1, C2) and what they cannot do
  without it.
* **Parity with the official SDKs,** whose clients and servers a Go program must interoperate with (K1–K4).
* **Standing of the feature in the specification,** completed, active or draft, and the cost of following it while it changes
  (S2, S3, S7, S8).
* **No change to the stable API.** The `acp` package, `agentproc`, `acptest` and `clienthost` keep their compatibility promise
  (R8).
* **Dependencies:** none unless a MADR names one, and then the smallest that does the job (R7).
* **Testability on every CI platform without a network,** under the race detector and the leak check.

## Considered Options

* **O1. The remote transport:** Streamable HTTP and WebSocket, client and server, in a new experimental package.
* **O2. The MCP-over-ACP binding:** route `mcp/message` by `serverId`, track `requestId`, and serve MCP operations a client
  provides to its agent.
* **O3. A registry client:** fetch and parse `registry.json`, pick a distribution for the platform, and build an
  `agentproc.Command`.
* **O4. None now.**

## Decision Outcome

Chosen option: "O1. The remote transport", because it is the only remote path ACP has, every official SDK ships it (K1), one
consumer has already built and discarded 6,301 lines of it (C2) and the other has reserved a package for it (C1), and the
standard library plus one small dependency already used by a consumer covers it (G1, G3, C3). O2 and O3 each bring less: O2 is as
unsettled as O1, is in two official SDKs of four, and neither consumer wants it (S7, K6, C4); O3 is the one completed standard but
sits in client applications, not SDKs (S8, K7, C5).

The cost of O1 is that its RFD is Active, not Completed, and changed four times this year (S2, S3). The design answers it by
putting the transport in a package outside the compatibility promise (R8), as all four official SDKs mark theirs experimental
(K1), and by naming the RFD revision it implements.

### The design

**Package.** `github.com/maccavelli/acp-go-sdk2/remote` (Q1), in this module. Its doc comment says that it implements the RFD
at the revision of 2026-07-02, that it is not subject to the Go 1 compatibility promise, and that a name in it may change in a
minor release when the RFD changes. `scripts/check_api.py` reports it beside `unstable` and does not gate it. Package `acp` does
not import it, and no exported name changes in `acp`, `agentproc`, `acptest` or `clienthost`.

**The seam.** Both sides present a remote connection as the byte stream of R1: each message received, a WebSocket text frame,
an SSE event's data or a POST body, is checked to be one JSON value, compacted so that it holds no newline, and delivered as one
line; each line written becomes one text frame or one POST body. So `acp.NewClientSideConnection`, `acp.ServeAgent`, the taps,
`agentproc.Transcript` and `acptest`'s records work unchanged (R2), and the RFD's "same JSON-RPC message format and ACP lifecycle
as … stdio" holds by construction.

**Client.**

```go
c, err := remote.Dial(ctx, "wss://agent.example/acp", &remote.DialOptions{Header: auth})
conn := acp.NewClientSideConnection(client, c, c)
```

* `Dial(ctx, url, *DialOptions) (*Conn, error)`. A `ws://` or `wss://` URL dials WebSocket; an `http://` or `https://` URL uses
  Streamable HTTP. The RFD requires a client to support both (S4).
* `*Conn` is an `io.ReadWriteCloser`. `Close` sends `DELETE` with `Acp-Connection-Id` for HTTP, or the close handshake for
  WebSocket, and ends the reader.
* `DialOptions`: `HTTPClient` (its `Jar` defaulting to a fresh `cookiejar` per connection, as the RFD requires cookies), `Header`
  (sent on every request and the handshake: authentication is the caller's, S4), and `MaxMessageSize` (10 MiB by default, as R1;
  the WebSocket read limit is set to it, since G3's default is 32 KiB).
* Streamable HTTP, as K3 does it: the first line must be the `initialize` request, POSTed with no connection header; its `200`
  body is the first line read, and its `Acp-Connection-Id` header binds the rest. The client then opens the connection-scoped
  stream. A line carrying `params.sessionId`, unless it is a `$/` message, is POSTed with `Acp-Session-Id`. A response carrying a
  new `sessionId` opens that session's stream before it is delivered; a `session/load` or `session/resume` request opens its
  session's stream before it is POSTed, as the RFD's diagram does for `session/load` and Rust's guide says for both
  (`md/http-transport.md`). POSTs go one at a time in the order written, each waiting for its `202`, so the order the connection
  wrote is the order the server receives. The SSE streams are merged into the one reader. The RFD gives no order between streams,
  and neither does this client.
* HTTP versions: `https://` negotiates HTTP/2 by ALPN; `http://` uses HTTP/1.1 unless the caller's `HTTPClient` sets
  `Protocols` to unencrypted HTTP/2 only (Q4, K4). `Acp-Protocol-Version: 1` is sent after `initialize` (S4's SHOULD).

**Server.**

```go
h := remote.NewHandler(func(ctx context.Context, in io.Reader, out io.Writer) error {
    return acp.ServeAgent(ctx, newAgent(), in, out)
}, nil)
defer h.Close(ctx)
http.Handle("/acp", h)
```

* `NewHandler(serve, *HandlerOptions) *Handler`; `*Handler` is an `http.Handler` with a `Close(ctx)`. `serve` has
  `agentproc.InProcess.Serve`'s signature (R2) and runs once per ACP connection, on its own goroutine, as K2's factories do.
* Routing is the RFD's table (S4): `POST`, `GET` with and without `Upgrade: websocket`, `DELETE`, and its status codes. Bodies are
  limited by `http.MaxBytesReader` to the message size. `Acp-Connection-Id` is `crypto/rand.Text()`.
* Outbound routing for Streamable HTTP follows K3: the `initialize` response in the POST body; a request or notification with
  `params.sessionId` to that session's stream; a response to the stream of the POST that carried its request, except responses to
  `session/new`, `session/load`, `session/resume` and `session/fork`, which go to the connection stream; everything else to the
  connection stream. A message for a stream not yet open is held, up to `HandlerOptions.MaxBuffered` bytes per connection; past it
  the connection is closed and its serve told why, rather than a message dropped silently.
* Ending: `DELETE` or a WebSocket close ends one connection, and `Handler.Close` ends all of them, the same way: close `serve`'s
  input, wait up to `Grace` (5 s by default, as `InProcess`), then cancel its context with a cause, and after a further `Grace`
  abandon it, as `InProcess` does (0042-MADR-in-process-agent-runner.md, item 1).
* Liveness, which the RFD leaves to the implementer (S4): an SSE comment and a WebSocket ping every `KeepAlive`, 30 s by default.
  *(Changed by "Amended by the PLAN, 2026-10-29", 8: 15 s by default.)*
* Security: no authentication is built in; the host wraps the handler (S4). The WebSocket upgrade keeps G3's same-origin check
  unless `HandlerOptions.OriginPatterns` widens it, and the other methods run behind `http.CrossOriginProtection` (G1, C1).
  *(Changed by "Amended by the PLAN, 2026-10-29", 7: the other methods apply the WebSocket upgrade's origin rule instead.)*
* The handler accepts HTTP/1.1, HTTP/2 and, where the server's `Protocols` allows it, unencrypted HTTP/2 (Q4).

**WebSocket** is `github.com/coder/websocket` v1.8.15, the module's second dependency (Q2). It is the only third-party import, in
package `remote` only.

**Not in this record:** SSE resumability and `Last-Event-ID`; automatic reconnect; JSON-RPC batches (`501`, as the RFD and K5);
WebSocket subprotocols and compression; WebSocket over HTTP/2; Unix sockets and other custom transports; the proxy-chain RFD;
any change to the schema pin. Each waits for the RFD, or for a consumer that needs it.

### Questions for the owner

1. **Where does the transport live?**
   * **Recommended: package `remote` in this module**, reported and not gated, as `unstable` is. Cost: the root `go.mod` gains a
     requirement that every consumer's module graph reads, though only an importer of `remote` compiles it.
   * A separate module, `remote/go.mod`, as `cmd/generate` is one. The root `go.mod` stays as it is, at the cost of its own tags,
     release notes, `make api` and consumer check.
   * Under `unstable/`. Its doc ties it to the unstable schema, and this is a transport, not a schema type.
2. **Which WebSocket implementation?**
   * **Recommended: `github.com/coder/websocket` v1.8.15** (G3): no requirements of its own, `go 1.23` under the floor, context-first,
     already required by magic-cli-remote (C3), so that consumer's graph does not grow.
   * `github.com/gorilla/websocket` v1.5.3 (G4): no release since 2024-06-14.
   * RFC 6455 written here: no dependency, but a framing, masking, fragmentation and close-handshake implementation this repository
     would own and fuzz.
   * Streamable HTTP alone, with no WebSocket: a server may do that, but a client MUST support both (S4), so the client side would
     not conform.
3. **What ships first?**
   * **Recommended: both profiles on both sides**, in phases of one PLAN: WebSocket client and server, then the Streamable HTTP
     client, then its server.
   * Both profiles in the client and WebSocket alone in the server, which the RFD allows (S4). Cheaper: the Streamable HTTP server's
     stream routing is the largest part. A Go agent behind a proxy that does not pass WebSocket could not be served.
4. **Which HTTP versions?**
   * **Recommended: what reaches every server in K4:** HTTP/2 by ALPN over TLS, HTTP/1.1 on `http://` unless the caller configures
     prior-knowledge HTTP/2; the server accepts all three. This departs from the RFD's "MUST use HTTP/2" only where the peer does.
   * Refuse Streamable HTTP over HTTP/1.1. This conforms to the letter of S4 but cannot reach the Rust server or goose over plain
     HTTP (K4).
5. **What interoperability evidence is required?**
   * **Recommended: on demand, a `tools/` check** that runs this package's client against the TypeScript and Python SDKs' example
     servers, and their example clients against this package's server, recorded in the PLAN. It needs Node and Python, so it does
     not run in CI.
   * Self-tests only: the two sides of this package could agree with each other and still disagree with K3.

#### Decided with the owner, 2026-10-29

The owner was asked each question with three options and chose the recommended one each time:

1. **Package:** `remote` in this module, reported by `make api` and not gated, as `unstable` is. Not a separate module, and not
   under `unstable/`.
2. **WebSocket:** `github.com/coder/websocket` v1.8.15, the module's second dependency, imported by package `remote` alone. Not
   `github.com/gorilla/websocket`, and not RFC 6455 written here.
3. **Scope:** both profiles on both sides, in phases of one PLAN: WebSocket client and server, the Streamable HTTP client, then its
   server. Not a WebSocket-only server, and not WebSocket alone.
4. **HTTP versions:** reach every peer in K4. The client negotiates HTTP/2 by ALPN on `https://` and uses HTTP/1.1 on `http://`
   unless the caller opts into prior-knowledge HTTP/2; the server accepts HTTP/1.1, HTTP/2 and unencrypted HTTP/2. Not a
   prior-knowledge attempt with a fallback, and not HTTP/2 alone.
5. **Interoperability:** an on-demand `tools/` check against the TypeScript and Python SDKs' example servers and clients at
   pinned versions, its results recorded in the PLAN; not a CI job, and not self-tests alone.

"The design" above already describes these answers, so it is unchanged.

#### Amended by the PLAN, 2026-10-29

Writing [0043-PLAN-remote-http-and-websocket-transport.md](0043-PLAN-remote-http-and-websocket-transport.md), and checking it
against the three official servers' code (K3's second table), fixed details that "The design" left open (1 to 6, and 9). It also
changed two of its details (7, 8). No owner's decision above changes.

1. **Names the design needs and did not list.** `HandlerOptions.Logger`, an `*slog.Logger` for what the handler cannot return:
   a serve's error, an abandonment, a refused request. Following Rust's diagnostic privacy rule (`md/http-transport.md`), it logs
   byte counts, message kinds and fixed labels, never a body, a method's parameters, an id or a session id. There are four end
   causes, which `context.Cause` reports to a serve: `ErrDeleted`, `ErrDisconnected`, `ErrHandlerClosed` and `ErrOverflow`.
   `Handler.Close` returns `ErrAbandoned` for a serve that outlived the ladder. `Conn` has no method beyond `Read`, `Write` and
   `Close`.
2. **Server rules, where the RFD is silent or the official servers depart from it.** The rule is to accept what any of the three
   official clients sends (K3's second table):
   * A session stream may be opened for a session the connection has not named yet, as TypeScript and Rust allow. The RFD's `404`
     therefore applies to an unknown connection, not to an unknown session. This lets a client open the stream before
     `session/load`, as the RFD's diagram does.
   * A second receiver for a stream that has one is answered `409`, as TypeScript and Rust answer it.
   * A session-scoped method takes its session from `Acp-Session-Id` or from `params.sessionId`, Rust's rule. Neither is `400`, and
     both, differing, are `400`. TypeScript's stricter rule, the header whenever `params.sessionId` is present, would refuse the
     Python client's `session/load`, which carries no header.
   * A client's response to the agent's request may omit `Acp-Session-Id`, as Rust and Python accept it. When present, it must be
     the session of that request (`400`), as TypeScript requires.
   * `initialize` on an existing connection is `400`, and so is an id or session id longer than 1024 bytes (TypeScript's
     `DEFAULT_MAX_ID_LENGTH`).
   * `MaxBuffered` defaults to 64 MiB, TypeScript's `DEFAULT_MAX_BUFFERED_BYTES`.
3. **A Streamable HTTP connection ends** with `ErrDisconnected` once no connection-scoped stream has been attached for 60 s. v1
   leaves liveness to the implementer (S4). Without this bound, a client that leaves without `DELETE` would hold its serve for
   ever. TypeScript's `DEFAULT_MAX_OUTPUT_STALL_MS` is the same 60 s.
4. **The WebSocket handshake is HTTP/1.1.** `websocket.Accept` refuses a request below HTTP/1.1 and needs an `http.Hijacker`
   (`accept.go:185`, `:130` at v1.8.15), which an HTTP/2 `ResponseWriter` is not. `Dial` therefore uses HTTP/1.1 for `ws://` and
   `wss://`, as Rust's client does (K4), whatever the caller's `HTTPClient` would negotiate.
5. **The client works with each official server's routing** (K3's second table):
   * Its streams feed one reader, so an answer may arrive on whichever stream a server sends it. `session/load`'s answer arrives on
     the connection stream from TypeScript and Python, and on the session stream from Rust.
   * It sends `Acp-Session-Id` on every session-scoped method and whenever `params.sessionId` is present, which TypeScript
     requires.
   * A POST answered `404` while it carried `Acp-Session-Id` is sent once more without the header. That is the Python server's
     answer to a `session/load`, `session/resume` or `session/delete` naming a session the connection has not seen. Without the
     header, Python accepts the POST; for `session/load` and `session/resume` it opens the session's stream from
     `params.sessionId` (`SESSION_ATTACH_METHODS`).
   * A `404` to a session stream opened before `session/load` or `session/resume` is retried once that request's answer has
     arrived, which is when the Python client opens it.
6. **A received message that is not JSON** is delivered as one line, its CR and LF bytes replaced by spaces. The connection then
   answers it as it answers an unparsable line on stdio.
7. **One origin rule for every method, not `http.CrossOriginProtection`.** "The design" put the HTTP methods behind
   `http.CrossOriginProtection`. Its `AddTrustedOrigin` takes exact origins, while `OriginPatterns` are host patterns, which
   `websocket.Accept` matches against the `Origin`'s host, or its scheme and host when a pattern holds `://` (`accept.go:228`–`:259`
   at v1.8.15). One option cannot configure both. So `POST`, `GET` and `DELETE` apply the WebSocket rule. A request without
   `Origin` passes, as non-browser clients send none. So does one whose `Origin` host is the request's `Host`, or matches a pattern.
   Any other is `403`.
8. **`KeepAlive` defaults to 15 s, not 30 s.** Rust's server sends an SSE keepalive every 15 s, and Python's every 15 s, "in sync
   with the TypeScript reference (15s)" (K3's second table). An intermediary that times out an idle stream at 30 s would end a
   30 s stream before its first keepalive.
9. **An `initialize` that fails.** The handler waits up to 30 s for the agent's answer, Python's `INITIALIZE_TIMEOUT_SECONDS`.
   * No answer by then: `504`.
   * A serve that returns or fails before answering: `500`, with a JSON-RPC `-32603` body, as TypeScript does.
   * Either way the connection is ended through the ladder, and no `Acp-Connection-Id` is sent.
   * An answer that is a JSON-RPC error is returned as it is, with `200`. No connection is kept, and no `Acp-Connection-Id` is
     sent, as Rust does.

### Consequences

* Good, because a Go program can serve an agent over the network and reach any agent served by an official SDK, with the client
  and agent code it has today (R2).
* Good, because the consumers can drop their own: `<cli-b>`'s placeholder becomes an import, and magic-cli-remote's next
  ACP-over-HTTP agent does not rebuild 6,301 lines (C1, C2).
* Good, because nothing stable changes: the new names are outside the compatibility promise, and `make api` keeps gating the
  four stable packages (R8).
* Bad, because the module gains its second dependency, and every consumer's module graph reads it (Q1, Q2).
* Bad, because the RFD can change again (S3), and the package will have to follow it, in a minor release, possibly breaking its
  importers. The doc comment says so.
* Bad, because the Streamable HTTP server holds per-connection state, buffers and goroutines that the SDK did not have before, and
  with them a new place for leaks and lost messages.
* Neutral, because the RFD's open points (S5) are settled here by following the reference SDKs, which may differ from the RFD's
  final text.

### Confirmation

* Every exported name of `remote` has a doc comment and an example; `go doc ./remote` names the RFD revision.
* Tests, in the package, without a network beyond loopback:
  * a round trip over each profile with `httptest.Server`, HTTP/1.1 and HTTP/2 over TLS and unencrypted HTTP/2: `initialize`,
    `session/new`, a prompt with updates, a permission request answered, `session/cancel`, `$/cancel_request`, and `DELETE`;
  * each status code of the RFD's table (S4), from the handler;
  * a message larger than 32 KiB and one at the size limit over WebSocket (G3); one past it refused;
  * outbound routing: each row of K3's table, two sessions on one connection, and a response to `session/new` reaching the client
    before that session's stream is open;
  * ending: `DELETE`, `Handler.Close`, a client gone without a close, and a serve that ignores its context, each bounded by
    `Grace`;
  * a transcript of a remote session read back with `acptest.ReadRecords` and passed through `ValidateGolden`;
  * the client against a fake server in each official server's mode of K3's second table, `session/load` on a fresh connection
    included, and the handler against requests shaped as each official client sends them.
* The package passes under `-race` on macOS, Linux and Windows, and the goroutine-leak check that `agentproc` runs.
* Each new check is seen to fail on a planted break in a scratch copy, as `AGENTS.md` requires.
* `make api` reports `remote`'s additions and gates nothing in it; `go mod tidy -diff` is clean; `govulncheck` covers the new
  dependency.
* If Q5 is answered as recommended: the interoperability check passes against the TypeScript and Python SDKs at named versions.

## Pros and Cons of the Options

### O1. The remote transport

* Good, because it is ACP's only remote path, and the working group's stated focus (S1, S6).
* Good, because all four official SDKs ship it (K1), so a Go program is the only one that cannot speak it.
* Good, because both consumers wanted it enough to reserve or build it (C1, C2).
* Good, because it needs no schema change and works at the `1.25.0` pin.
* Neutral, because most of it is the standard library (G1).
* Bad, because its RFD is Active, not Completed, and has changed four times this year (S2, S3).
* Bad, because it needs a dependency, or an RFC 6455 implementation owned here (G2).
* Bad, because the official implementations differ from the RFD, Rust on HTTP/2 and Python on where `session/load`'s replay goes
  (K3, K4, S5), so conformance is to observed behaviour as much as to text.

### O2. The MCP-over-ACP binding

* Good, because a client could give its agent tools over the ACP connection it already has, with no MCP process or listener (S7).
* Good, because the wire types are already generated (R6), and two official SDKs have it (K6).
* Bad, because the RFD, Active like the transport's, is tied to one MCP revision, has replaced its own earlier wire shape, and says
  its schema "must be released … before publication" (S7).
* Bad, because neither consumer wants it: one declines `mcp/message`, the other refuses `type: "acp"` (C4).
* Bad, because a useful binding needs an MCP library behind it, as TypeScript's peer dependency and Rust's `rmcp` crate show (K6),
  which for Go means `github.com/modelcontextprotocol/go-sdk` and its eight requirements, or a raw-JSON surface few would use.

### O3. A registry client

* Good, because the registry is a Completed RFD, the only one of the three (S8).
* Good, because a manifest maps onto `agentproc.Command`: `cmd`, `args`, `env`.
* Bad, because no official SDK has one (K7): discovery and installation belong to the client application, as editors do it.
* Bad, because 25 of the 45 distributions are `npx` or `uvx`, which need a Node or Python toolchain the SDK cannot supply, and 42
  of 107 binary targets carry no `sha256`, so downloading and running them is a trust policy the application must own (S8).
* Bad, because neither consumer reads the registry (C5).

### O4. None now

* Good, because it costs nothing and waits for the RFD to complete.
* Bad, because the consumers keep writing their own (C2), and a Go agent stays unreachable over the network from every official
  SDK's client.

## More Information

### Relationship to earlier records

* [0035-REPORT-consumer-acp-code-for-the-sdk.md](../reports/0035-REPORT-consumer-acp-code-for-the-sdk.md), F, and
  [0039-MADR-five-adopter-additions-on-schema-1-25-0.md](0039-MADR-five-adopter-additions-on-schema-1-25-0.md), "Out of scope",
  deferred a WebSocket transport to a MADR of its own. This is that MADR. Unix sockets stay out.
* [0023-MADR-connection-lifetime-and-test-leaks.md](0023-MADR-connection-lifetime-and-test-leaks.md), S4: the connection still has
  no keepalive. The ping here is the transport's, which the RFD leaves to the implementer.
* [0027-MADR-agent-process-helper.md](0027-MADR-agent-process-helper.md) is the precedent for naming a dependency: consumers
  already require it, and it declares a `go` line under the floor.
* [0039-MADR-five-adopter-additions-on-schema-1-25-0.md](0039-MADR-five-adopter-additions-on-schema-1-25-0.md), item 1, is the
  precedent for a package outside the compatibility promise.
* [0042-MADR-in-process-agent-runner.md](0042-MADR-in-process-agent-runner.md) gave the serve-function shape and the ending
  ladder the handler reuses.

### What would follow

[0043-PLAN-remote-http-and-websocket-transport.md](0043-PLAN-remote-http-and-websocket-transport.md), in phases: the package skeleton and
the dependency; the WebSocket client and server; the Streamable HTTP client; its server; the interoperability check; the docs and a
release. Each consumer then adopts it under its own records.

### Sources

* ACP specification repository `agentclientprotocol/agent-client-protocol` at `2bb9fac`: `docs/protocol/v1/transports.mdx`,
  `docs/protocol/v2/draft/transports.mdx`, `docs/rfds/streamable-http-websocket-transport.mdx`, `docs/rfds/mcp-over-acp.mdx`,
  `docs/rfds/acp-agent-registry.mdx`, `docs/rfds/about.mdx`, `docs/rfds/updates.mdx`, `docs/docs.json` (the RFD lifecycle groups).
* <https://agentclientprotocol.com/rfds/updates.md>, <https://agentclientprotocol.com/announcements/transports-working-group.md>,
  <https://agentclientprotocol.com/protocol/v1/extensibility.md>, <https://agentclientprotocol.com/get-started/registry.md>, read
  2026-10-29.
* `agentclientprotocol/typescript-sdk` at `08a0d30`: `package.json`, `CHANGELOG.md`, `src/transports/protocol.ts`, `server.ts`,
  `ws-stream.ts`.
* `agentclientprotocol/rust-sdk` at `1d2b358`: `Cargo.toml`, `md/http-transport.md`, `src/agent-client-protocol-http/Cargo.toml`,
  `CHANGELOG.md`, `src/protocol.rs`.
* `agentclientprotocol/python-sdk` at `9853d8b`: `pyproject.toml`, `docs/web-transport.md`.
* `agentclientprotocol/kotlin-sdk` at `cb69c39`: the file tree.
* `agentclientprotocol/registry` at `97b2e38`: `agent.schema.json`; and the published `registry.json`, fetched 2026-10-29.
* <https://go.dev/doc/go1.24>; <https://pkg.go.dev/golang.org/x/net/websocket>;
  <https://pkg.go.dev/github.com/coder/websocket@v1.8.15>; the Go module proxy for `github.com/coder/websocket`,
  `github.com/gorilla/websocket` and `github.com/modelcontextprotocol/go-sdk@v1.8.0`.
* The consumers, read-only: `<cli-b>` at `ccd8aa8` (`exp/acpws/doc.go`, `go.mod`, its module-architecture and v1-scope records,
  `acpserver/mcp_test.go`); magic-cli-remote at `0e84b1f0` (`go.mod`, `0025-MADR-goose-provider.md`,
  `0029-MADR-provider-platform-canonicalization.md`, `0160-MADR-remove-goose-cli-support.md`,
  `0167-REPORT-acp-go-sdk2-integration-assessment.md`).
