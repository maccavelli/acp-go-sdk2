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
  call's error.
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
* Bad, because the commit runs on the reader, which must keep it short; it is one decode and a copy.

### O2. A barrier

* Good, because `Initialize`'s code stays where it is.
* Bad, because the reader waits on the caller.

### O3. Hold, not refuse

* Good, because no message is refused for timing.
* Bad, because it needs a queue, a bound and a timeout.

### O4. Document only

* Good, because it costs nothing.
* Bad, because a conforming agent's messages are refused.

## More Information

* **Measurements outside the tree:** the probe and its output stay outside the repository, as its records do for scratch tools.
* **Related records:** [0017-MADR-remediate-acp-v1-conformance-violations.md](0017-MADR-remediate-acp-v1-conformance-violations.md), the
  gate; [0027-PLAN-agent-process-helper.md](0027-PLAN-agent-process-helper.md), step 2, where this was found.
* **Not decided here:** an `initialized` notification, which would be a protocol change, not this SDK's.
* **The PLAN** is [0030-PLAN-record-initialize-on-arrival.md](0030-PLAN-record-initialize-on-arrival.md).
