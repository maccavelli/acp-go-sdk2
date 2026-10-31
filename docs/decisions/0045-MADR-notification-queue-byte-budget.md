---
status: proposed
date: 2026-10-31
decision-makers: the repository owner
consulted: the sources of Codex, Kilo Code, opencode and Grok Build; the ACP TypeScript, Rust and Python SDKs; golang.org/x/net/http2, gRPC-Go, net/http, coder/websocket, golang.org/x/tools jsonrpc2, the MCP Go SDK and vscode-jsonrpc, at the revisions under "Sources"
informed: gobble-cli, magic-cli-remote
---

# Bound what a connection keeps of its peer's messages in bytes as well as in count

## Context and Problem Statement

A connection queues the notifications it receives for one worker that handles them in order. The queue is bounded by count:
[0044-MADR-notification-queue-back-pressure.md](0044-MADR-notification-queue-back-pressure.md) kept `WithMaxQueuedNotifications`,
default 1024, and since `v1.29.0` pauses the reader at that count under `OverflowBlock`, the default.

Its F8 says the count is not a memory bound. With messages of up to `defaultMaxMessageSize`, 10 MiB, 1024 queued notifications can
hold about 10 GiB. 0044 left "a byte budget for what a connection keeps from its peer" to a separate decision (its "Not in this
decision", and the "A larger queue bounded by bytes" option: "worth having beside any policy"). This is that decision.

Two things make it more pressing than when 0044 was written:

* **`OverflowBlock` keeps the queue full while paused.** Under the old close-on-overflow default, a peer that filled the queue
  ended the connection. Under the new default the connection stays open with n notifications held. While a handler waits on its
  own request, or `Initialize` waits behind held notifications, it reads on to 2n (0044 D3 and D4), up to about 20 GiB at the
  default sizes.
* **Requests are not queued at all** (F3). Each received request runs on its own goroutine holding its params until its handler
  returns. The only bound is `WithMaxConcurrentRequests`, which has no default.

The question: should a connection bound in bytes what it keeps of its peer's messages, and if so, what is counted, what the
default is, and what each overflow policy does when the budget is reached?

## Findings

Measured at `9fcbb6f` (`v1.29.0`) unless stated. The external sources are under "Sources".

* **F1. What a queued notification keeps.**
  * `readLine` allocates each line afresh (`connection.go:1365-1381`).
  * The queue holds a `*anyMessage` (`connection.go:40-53`, `:1658`). Its `Params` is a `json.RawMessage` that
    `encoding/json` copies from the line, so the line itself becomes garbage.
  * A queued notification therefore keeps less than its line's length. The line length is an upper bound that needs no
    decoding to know.
* **F2. When it is let go.** A notification's memory is held until its handler returns. That is also when 0044's count
  advances: `completedNotificationSeq` moves in `processNotifications` after `handleInbound` (`connection.go:1823-1842`), and the
  pause counts "received and not completed" (0044 D2, as amended). A handler may keep its params longer, but that is the
  handler's memory.
* **F3. Requests.**
  * A request is not queued. `receiveLine` starts a goroutine for it at once (`connection.go:1617-1637`), holding the
    message until the handler returns.
  * `WithMaxConcurrentRequests(n)` bounds how many run, and refuses one over the limit at once with `-32603` and
    `{"error":"too many concurrent requests","limit":n}`. The reader never waits for a slot, since a running handler may be
    waiting for a response only the reader delivers (`connection.go:1561-1574`; 0019-PLAN-api-decisions.md, B18).
  * There is no default limit, so the memory requests hold is bounded only by the peer.
* **F4. This SDK already has one byte budget.** `remote`'s Streamable HTTP server holds a client's messages up to
  `MaxBuffered`, 64 MiB by default, counted by raw line length, and ends the connection with `ErrOverflow` past it
  (`remote/handler.go:45-48`, `:123-124`; `remote/httpserver.go:405-411`;
  [0043-MADR-remote-http-and-websocket-transport.md](0043-MADR-remote-http-and-websocket-transport.md)).
* **F5. One ACP SDK bounds inbound bytes: TypeScript's WebSocket server.**
  * It counts each frame waiting behind the one being handled, as its text length plus 1 KiB, "Otherwise small or empty frames
    would barely count", against `maxBufferedBytes`, 64 MiB (`ws-server.ts:44-47`, `:147-161`; `connection.ts:12`).
  * Past it, the connection closes (`ConnectionLimitError`), except that a frame is admitted alone when nothing waits.
  * Its per-message cap is 32 MiB (`stream-limits.ts:1`), so the budget is twice the largest message.
  * The Rust SDK's core, the Python SDK, Codex's stdio app-server, Grok Build's reader and vscode-jsonrpc bound no bytes.
* **F6. The other byte budgets surveyed fail the consumer; none pauses its own reader.**
  * Codex's MCP event stream: 2 MiB per subscription against 1 MiB per message, ending the subscription
    (`event_notification_transport.rs:22-23`, `:217-244`).
  * Kilo Code's cloud stream: 8 MiB, aborting "consumer is too slow" (`websocket-stream.ts:7-9`, `:117-133`).
  * The ACP Rust SDK's MCP-over-ACP polyfill: 16 notifications **and** 256 KiB per request, cancelling it (`mod.rs:33-38`,
    `:47-82`).
  * HTTP/2 and gRPC get back-pressure from flow-control credit the peer honours (x/net `http2/config.go:104-114`; gRPC
    `flowcontrol.go`).
  * Under `OverflowBlock`, the paused reader plays the part of that credit, so a byte budget fits as a second pause trigger.
* **F7. Count and bytes together, whichever is reached first, has precedent:** the Rust polyfill (16 messages and 256 KiB) and
  HTTP/2 (`MaxConcurrentStreams` with its windows). Counting both keeps small messages from being free without a per-entry
  overhead such as TypeScript's.
* **F8. A message larger than the budget is handled three ways.**
  * Admitted when nothing waits: TypeScript; Kilo, which checks before adding; and the MCP Go SDK's event store, "could result
    in nBytes > maxBytes, but we'll live with that" (`event.go:327-328`).
  * Refused outright: the Rust polyfill.
  * Given window for that one message: gRPC.

  Codex and TypeScript avoid the case by keeping the budget at least the largest message, twice it.

## Decision Drivers

* A connection's memory should be bounded by what its options say, whatever its peer sends, under every overflow policy.
* Nothing a conforming peer sends at the configured limits should be refused for size alone. A single message up to
  `WithMaxMessageSize` always fits.
* The byte bound must keep 0044's guarantees: the response barrier, the guard that reads on while the loop waits on the reader
  (D3), and the order of notifications.
* The existing options keep their meaning. `WithMaxQueuedNotifications` stays a count, and the overflow policies keep their
  names and effects.
* The accounting must be cheap and exact: one addition and one subtraction under the lock the count already takes.

## Considered Options

* **Count only.** Keep the status quo: F8's 10 GiB, 20 GiB while reading on.
* **A byte budget beside the count, whichever is reached first.** Each policy acts on it as it acts on the count.
* **Replace the count by bytes**, with a per-entry overhead, the TypeScript route.
* **Lower `defaultMaxMessageSize`** from 10 MiB, which shrinks the worst case without a new bound.

## Decision Outcome

Chosen option (proposed): **"A byte budget beside the count, whichever is reached first"**.

It bounds memory under every policy, keeps both existing options' meaning, and follows the strongest precedents: the TypeScript
SDK's budget, the Rust polyfill's pairing of count and bytes, and this SDK's own `MaxBuffered`. Under `OverflowBlock` it is one
more reason to pause, so nothing is lost.

The parts of the decision:

* **D1. The option.** `WithMaxQueuedBytes(n int)` sets the budget. A value of 0 or less is ignored, as
  `WithMaxQueuedNotifications` ignores it.
* **D2. What is counted, and when it is let go.**
  * A notification counts its line's length (F1) from the moment it is queued until its handler returns (F2), under `notifyMu`.
  * The sequence count changes at the same moment, so both bounds release at the same time.
  * There is no per-entry overhead. The count bound still applies, so small messages are not free (F7).
* **D3. Each policy at the budget.** A notification whose line would take the counted bytes past the budget, while anything is
  counted, is treated as the count's overflow is:
  * `OverflowBlock`: the reader pauses until enough handlers return. Under 0044 D3's guard it reads on, to twice the budget,
    then closes with `ErrNotificationQueueOverflow`, as 0044 D4 does at twice the count.
  * `OverflowCloseConnection`: the connection closes with `ErrNotificationQueueOverflow`.
  * `OverflowDropNewest`: the notification is dropped, counted by `DroppedNotifications` and passed to the drop handler.
* **D4. The budget is never smaller than one message.** A budget below `WithMaxMessageSize` is raised to it, so a single message
  the reader accepts always fits when nothing else is counted (F8; the invariant Codex's and TypeScript's two-to-one sizing
  gives).
* **D5. The default: 64 MiB** (Q1 a, answered 2026-10-31).
* **D6. One budget for everything received and unfinished** (Q2 a, answered 2026-10-31). A request counts its line from
  receipt until its handler returns. A request that would pass the budget is refused at once: `-32603` with
  `{"error":"too many queued bytes","limit":n}`, as `WithMaxConcurrentRequests` refuses one, never waiting.
* **D7. Observability.**
  * The pause's `Warn` after 5 s, and the overflow's `Error`, gain the counted bytes and the budget.
  * `ErrNotificationQueueOverflow` keeps its identity, so `errors.Is` works as before.
  * A new accessor, `QueuedBytes()`, reports the counted bytes, for a consumer's metrics.

### Questions for the owner

* **Q1. The default budget.**
  * **(a) 64 MiB.** Recommended. It is `remote`'s `MaxBuffered` (F4) and the TypeScript SDK's `maxBufferedBytes` (F5), and
    6.4 times the 10 MiB message. At the default count of 1024 it binds only when the average notification passes 64 KiB.
    Under `OverflowBlock` reaching it only slows the peer.
  * **(b) Twice `WithMaxMessageSize`**, 20 MiB by default. It is Codex's and TypeScript's ratio, and follows the message size
    when a consumer changes it. It is tighter: a few large tool-call payloads would pause the reader.
  * **(c) No default**, opt-in only. It changes nothing for existing consumers, but leaves F8's 10 GiB in place by default.
* **Q2. Requests.**
  * **(a) One budget for everything received and unfinished.** Recommended. Requests count their line too, from receipt to their
    handler's return. A request that would pass the budget is refused at once, `-32603` with
    `{"error":"too many queued bytes","limit":n}`, as `WithMaxConcurrentRequests` refuses one (F3), never waiting, for B18's
    reason. One option then bounds what the peer can make the connection hold.
  * **(b) Notifications only**, as 0044 F8 framed it. Requests stay bounded by `WithMaxConcurrentRequests` alone, with no
    default.
  * **(c) A separate request budget**, its own option and default. That is two knobs for one concern.

#### Answered by the owner, 2026-10-31

* Q1: **(a) 64 MiB.**
* Q2: **(a) One budget for requests and notifications.**

Both are the recommended answers; D5 and D6 above state them. The decision stays `proposed` until the owner accepts it.

### Consequences

* Good, because a connection's memory for its peer's unfinished messages is bounded by default (with Q1 a or b), under every
  policy: about 64 MiB, or 128 MiB while reading on, where F8 had 10 GiB.
* Good, because under `OverflowBlock` the budget only slows a fast peer; nothing is dropped and the connection stays open, as at
  the count.
* Good, because the count and the bytes release together, so 0044's barrier and guard need no change: a pause on bytes is the same
  pause.
* Neutral, because each queued message adds one integer, and receiving one costs one addition and one subtraction under `notifyMu`.
* Bad, because a consumer on `OverflowCloseConnection` or `OverflowDropNewest` that legitimately keeps more than the budget
  queued now closes or drops where it did not. With Q1 (a), that takes more than 64 MiB of unfinished notifications. The
  release notes say so and name the option.
* Bad, with Q2 (a), because a request can be refused for bytes the connection holds for notifications, while the reader reads on
  under 0044's guard. The refusal is immediate and names the limit, as the concurrency limit's does.
* Neutral, because `remote`'s `MaxBuffered` is a different bound, on what its server holds for a client's streams, and is
  unchanged.

### Confirmation

The PLAN, [0045-PLAN-notification-queue-byte-budget.md](0045-PLAN-notification-queue-byte-budget.md), confirms each of these. Each
new test is seen to fail on a planted break in a scratch copy.

1. Under `OverflowBlock`, notifications whose lines pass the budget while the count has room pause the reader. Every one is
   delivered in order once handlers return, none is dropped, and the connection stays open.
2. Under 0044's guard the reader reads on to twice the budget, then closes with `ErrNotificationQueueOverflow`.
3. Under `OverflowCloseConnection` and `OverflowDropNewest`, the budget closes or drops as the count does.
4. A budget below `WithMaxMessageSize` is raised to it, and a single maximum-size line is accepted when nothing is counted.
5. The bytes counted return to 0 when every handler has returned, and `QueuedBytes()` reports them.
6. With Q2 (a), a request past the budget is answered `-32603` with the limit, and its handler does not run.
7. The existing queue, barrier, overflow and hold tests pass unchanged.
8. `make api` reports only compatible additions.
9. `tools/gates.py --all` and CI pass.

## Pros and Cons of the Options

### Count only

* Good, because nothing changes.
* Bad, because memory stays bounded only by the peer: about 10 GiB of queued notifications, 20 GiB while reading on, and nothing
  for requests.

### A byte budget beside the count, whichever is reached first

* Good, because it bounds memory under every policy, with one option, at the cost of one counter.
* Good, because it has the strongest precedents: TypeScript's per-connection budget, the Rust polyfill's pairing, and this SDK's
  `MaxBuffered`.
* Neutral, because it adds a second reason for each policy to act, which the logs and the release notes must name.
* Bad, because a non-default policy can now close or drop at the budget, where it acted only at the count.

### Replace the count by bytes

* Good, because it is one knob.
* Bad, because it changes what `WithMaxQueuedNotifications` means, or retires it, and every consumer that set it.
* Bad, because without the count, small messages need an invented per-entry overhead to count at all.

### Lower `defaultMaxMessageSize`

* Good, because it shrinks the worst case with no new mechanism: 1 MiB gives about 1 GiB.
* Bad, because ACP carries file contents, diffs and embedded resources in single messages. A smaller default refuses messages a
  conforming peer sends today.
* Bad, because the bound is still a count times a size, not a budget.

## More Information

* Related records: [0044-MADR-notification-queue-back-pressure.md](0044-MADR-notification-queue-back-pressure.md), whose F8 and
  "Not in this decision" this answers; [0043-MADR-remote-http-and-websocket-transport.md](0043-MADR-remote-http-and-websocket-transport.md)
  (`MaxBuffered`); 0019-PLAN-api-decisions.md, B17 and B18.
* Release: a minor, `v1.30.0`. One option and one accessor are added, and with Q1 (a) or (b) a default bound changes behaviour.
  The release notes lead with that, as `v1.29.0`'s led with its default.

### Sources

| Source | Revision | What was read |
| :--- | :--- | :--- |
| ACP TypeScript SDK | `c959b06` | `src/transports/ws-server.ts:44-47`, `:84-89`, `:147-161`; `src/transports/connection.ts:12`, `:44-45`; `src/core/stream-limits.ts:1`, `:37-41` |
| ACP Rust SDK | `fe1e126` | `src/agent-client-protocol-polyfill/src/mcp_over_acp/mod.rs:33-38`, `:47-82`; `src/agent-client-protocol/src/stdio.rs:59` |
| ACP Python SDK | `9853d8b` | `src/acp/_transport.py`, the read loop and its unbounded `asyncio.Queue` (read through the web, not re-verified line by line) |
| Codex | `b91cedc080` | `codex-rs/rmcp-client/src/event_notification_transport.rs:22-23`, `:58-69`, `:217-244`; `app-server-transport/src/transport/mod.rs:24`, `:241-268` |
| Kilo Code | `0a9faeff69` | `packages/opencode/src/kilocode/cloud/websocket-stream.ts:7-9`, `:117-133` |
| opencode | `237a847056` | `packages/server/src/event-feed.ts:8`, `:66-68` |
| Grok Build | `2bdd1d6a` | `crates/codegen/xai-acp-lib/src/stdin_reader.rs:64`, `:94-96`, `:169-178` |
| golang.org/x/net | `v0.62.0` | `http2/config.go:104-114`; `http2/server.go:1773-1776`, `:2408-2425` |
| gRPC-Go | `v1.82.0` | `internal/transport/defaults.go:28-30`; `flowcontrol.go`; `http2_server.go:771-808` |
| net/http | Go 1.27.3 | `request.go:1199-1223` (`MaxBytesReader`) |
| coder/websocket | `v1.8.15` | `read.go:88-107` |
| golang.org/x/tools | `v0.52.0` | `internal/jsonrpc2/stream.go:127`, `:140`; `internal/jsonrpc2_v2/frame.go:155`, `:168` |
| MCP Go SDK | `v1.8.0` | `mcp/transport.go:124`; `mcp/streamable.go:224`, `:408-410`; `mcp/event.go:284`, `:323-331` |
| vscode-jsonrpc | `a0981e3` | `jsonrpc/src/common/messageReader.ts:229-234`; `connection.ts:625` |
