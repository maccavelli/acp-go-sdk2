# Release notes for v1.33.0, against v1.32.0

Observed 2026-11-04 on `41150d9`, for [0049-PLAN-subpackage-fixes.md](../decisions/0049-PLAN-subpackage-fixes.md), phase 6,
step 4. This report drafts the text of the GitHub release for `v1.33.0`, as `RELEASING.md` describes it. It decides nothing.
It is committed before the tag, so the tagged tree holds it. The commit that adds it changes records and nothing else, so the
API it tags is `41150d9`'s.

Each statement names its source:

- a record;
- a commit;
- or `make api`, run on `41150d9` with nothing uncommitted but this report.

Summary:

- 2 compatible additions and nothing incompatible. `make api` against `v1.32.0`: "0 incompatible, 2 compatible", both in
  package `remote`, `DialOptions.MaxBuffered` and `HandlerOptions.AllowedHosts`, so the release is a minor, as `RELEASING.md`
  says and the subpackage PLAN planned. `remote` is not under the Go 1 promise; `make api` reports its changes without
  gating them and counts its compatible ones.
- Nine fixes to the subpackages, P1 to P9 of
  [0049-MADR-remediate-the-2026-11-01-debugging-pass.md](../decisions/0049-MADR-remediate-the-2026-11-01-debugging-pass.md),
  and a flaky test's margin.
- What a consumer may meet, in the PLAN's order:
  - a request to a `remote` handler on a loopback listener whose `Host` is a name other than `localhost`'s is refused `403`;
  - a WebSocket whose reader falls behind stays open, up to the client's `MaxBuffered`;
  - an over-long WebSocket message, and an agent's over-long answer, are answered `-32600`, and the connection reads on;
  - `Transcript.Close` writes nothing after it returns;
  - `Files` and `MemoryClient` take any `line` and `limit` on a 32-bit target, and `Files` reads one line of a large file
    without reading the file, and refuses a FIFO by the file it opened;
  - `remote.Conn.Read` ends with `io.EOF` itself at a Streamable HTTP agent's clean end.
- The root package, the `go` lines, the requirements and the schema are unchanged.

The section "Release text" is what goes into the GitHub release.

## Release text

### v1.33.0

The ACP schema is still `1.25.0` (`git diff v1.32.0 -- schema/` is empty), and the root package is unchanged. `make api`
reports two compatible additions against `v1.32.0`, both in package `remote`, and nothing incompatible. This release fixes
the nine defects of the subpackages that the 2026-11-01 debugging pass found
([0049-MADR-remediate-the-2026-11-01-debugging-pass.md](../decisions/0049-MADR-remediate-the-2026-11-01-debugging-pass.md),
P1 to P9). Each fix has a test that fails without it.

One change refuses requests that `v1.32.0` served. Read "Changed: a loopback handler checks `Host`" before upgrading a
`remote` server.

#### Changed: a loopback handler checks `Host` (P2)

A `remote.Handler` accepted a browser's request whenever its `Origin` matched its `Host`. After a DNS rebinding, a page at
`attacker.example` reaches a handler on `127.0.0.1` with both headers `attacker.example`, so it passed, and the page could
open a connection and run `initialize`. (`994ba94`.)

- **`HandlerOptions.AllowedHosts`,** new: the host names a request's `Host` may carry, as patterns matched as
  `OriginPatterns` are, the port ignored.
- **Left nil, on a loopback listener,** the handler allows an IP literal, `localhost` and a name under `.localhost`, and
  answers any other name `403`, the WebSocket upgrade included. The listener is read from `http.LocalAddrContextKey`.
- **On any other listener,** or one that does not say, nil allows every name, as before.
- **A server reached by a name on loopback,** through a reverse proxy for one, lists that name in `AllowedHosts`, or its
  requests are refused.

#### Fixed: a WebSocket that falls behind stays open (P1)

The library reads a ping's pong only while a read runs, so a side that stopped reading for back-pressure stopped answering
pings, and the server ended the connection with `remote.ErrDisconnected`: a client whose application was slow, and a live
client whose agent was busy. (`f133869`.)

- **The client** reads every frame the server sends up to a byte bound, the new `DialOptions.MaxBuffered`, 64 MiB by default
  and at least `MaxMessageSize`, so it goes on answering pings. Past the bound it stops reading, and the server ends it as
  before. Streamable HTTP's client queue takes the same bound, where both held 256 lines.
- **The server** does not count a ping that fails while its reader waits for the agent to read.

#### Fixed: a message over `MaxMessageSize` is answered (P6, P9)

On stdio, a message over the limit is discarded, answered `-32600` with `"id": null`, and reading goes on. Over WebSocket
it ended the connection, with status 1009, and an agent's over-long answer was dropped with nothing sent, so the request
it answered hung. (`c299242`.)

- **A WebSocket message over the limit,** from either side, is drained without being held whole, dropped, answered
  `-32600` with `"id": null` and "message exceeds N bytes", and the connection reads on.
- **An agent's line over the limit,** on either profile, is dropped and answered `-32600` on the agent's own input, so the
  agent learns of it.
- **A POST over the limit** keeps its `413`.

#### Fixed: `clienthost.Files` and `acptest.MemoryClient` (P3, P4, P5)

- **P3.** `ReadTextFile` read the whole file, whatever `line` and `limit` asked: one line of a 64 MiB file allocated 640 MiB.
  It now reads no further than the last line it returns, through a 64 KiB buffer, and holds no line it skips. A read with
  neither reads the whole file into a string of its size. (`63485ff`.)
- **P4.** Both methods checked the path, then opened it, so a path swapped for a FIFO in between blocked the handler in the
  open, with no context to end it. They now open first, on Unix without waiting for a FIFO's other end, and refuse what the
  opened file shows is not regular. A write truncates only once the file is known to be regular. (`63485ff`.)
- **P5.** On a 32-bit target, a `line` or `limit` of 2^31 or more panicked both `Files` and `MemoryClient`. They now count in
  `uint64`. (`63485ff`.)

#### Fixed: `agentproc.Transcript.Close` (P7)

`Close` cleared a line its tap was still writing, without a lock, while a tap that `Wait` had abandoned was still writing:
a data race. `LineTap` now guards what it holds, and once `Close` has returned nothing more reaches the writer, so the
caller may close it. A record already being written finishes first. (`dbccdb5`.)

#### Fixed: `remote.Conn.Read`'s clean end (P8)

Over Streamable HTTP, `Read` ended an agent's clean end with an error wrapping `io.EOF`, which `io.ReadAll` and any caller
testing `err == io.EOF` took as a failure. It now returns `io.EOF` itself, as over WebSocket. (`c299242`.)

#### For maintainers

- **Tests.** Each finding has a test that fails with its fix reverted in a copy of the tree, P5's on linux/386. The
  subpackage PLAN's phase 6 lists them by finding, and phases 1 to 5 record each planted break and its failure.
- **The flaky `TestInProcessServeAgentStep2`** runs under a grace of 3 seconds: under load, 300 ms killed 9 of 100 runs, and
  3 s killed none. (`dbccdb5`.)
- **No 32-bit run is added** to CI or the gates, by the owner's choice; the new tests check the arithmetic on 64-bit hosts.
- **The documentation** was checked against the code before the work and corrected
  (`f6bfdcf`): the README, `docs/architecture.md`, `RELEASING.md`, and the records.

## Sources not in the release text

- **Commits since `v1.32.0`** (`git log v1.32.0..41150d9`), eight:
  - `277aca0` marks the surface PLAN complete;
  - `f6bfdcf` corrects the documentation and the records, adds P8 and P9 and the subpackage PLAN, and records Q12 to Q15;
  - `f133869`, phase 1 (P1): `remote/frame.go`, `client.go`, `ws.go`, `handler.go`, `httpclient.go`, their tests;
  - `63485ff`, phase 2 (P3, P4, P5): `clienthost/files.go`, `open_unix.go` and `open_other.go` (new), `acptest/client.go`,
    their tests;
  - `dbccdb5`, phase 3 (P7, the margin): `agentproc/linetap.go`, `transcript.go`, their tests;
  - `994ba94`, phase 4 (P2): `remote/host.go` (new), `handler.go`, `handler_test.go`;
  - `c299242`, phase 5 (P6, P9, P8): `remote/frame.go`, `ws.go`, `handler.go`, `httpclient.go`, `client.go`, three test files;
  - `41150d9`, phase 6's deviation 1, `clienthost/files_unix_test.go`, and steps 6.1 and 6.2's record.
- **Go files changed.** `git diff --name-status v1.32.0 41150d9 -- '*.go'` lists 22, all in `acptest/`, `agentproc/`,
  `clienthost/` and `remote/`; four new: `clienthost/files_unix_test.go`, `open_other.go`, `open_unix.go`, `remote/host.go`.
- **Unchanged** (`git diff --stat v1.32.0 41150d9`, empty for these): the root package's Go files, the `*_gen.go` files,
  `go.mod`, `go.sum`, `cmd/`, `internal/`, `unstable/`, the schema, `.github/` and `tools/consumer.py`. `RELEASING.md` changed
  in `f6bfdcf`'s documentation corrections only.
- **`make api` on `41150d9`,** this report its only uncommitted file: exit 0, "remote: DialOptions.MaxBuffered: added",
  "remote: HandlerOptions.AllowedHosts: added", "against v1.32.0: 0 incompatible, 2 compatible"; "reported packages
  unstable, remote: 0 incompatible, not gated". Step 6.1's `api` gate had the same.
- **The gate set,** the PLAN's phase 6 step 1: 18 of 18, with the Linux and Windows hosts, after its deviation 1.
- **The consumer.** `tools/consumer.py` is unchanged: its program serves `remote` through `httptest.NewServer`, whose
  `127.0.0.1` the Host rule allows. `consumer.py v1.33.0` runs after the tag.
