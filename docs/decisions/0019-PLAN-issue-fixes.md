---
status: complete
date: 2026-10-18
associated-madr: "0019-MADR-remediate-debugging-pass-findings.md"
---

# Implement the fixes for issues #7 to #9 under 0019, before phase 5

Associated MADR: [0019-MADR-remediate-debugging-pass-findings.md](0019-MADR-remediate-debugging-pass-findings.md)

## Goal

Implement what the MADR's "Evaluated and decided with the owner, 2026-10-18" sets before phase 5:

| Finding | Issue | What it fixes |
| :--- | :--- | :--- |
| B22 | [#7](https://github.com/maccavelli/acp-go-sdk2/issues/7) | `Prompt` cancels only a prompt it wrote. |
| B23 | [#8](https://github.com/maccavelli/acp-go-sdk2/issues/8) | Requests received before the end of input are answered. |
| B24 | [#9](https://github.com/maccavelli/acp-go-sdk2/issues/9) | The subagent updates are gated on `subagents`. |
| E18 | | The example clients print a tool call's status, not its pointer. |
| I10 | | `Prompt`'s best-effort cancel keeps the caller's values. |

- **Tests:** each change has a test seen to fail on a planted break first.
- **API:** no exported API changes.
- **Behaviour:** B23 changes behaviour at the end of input, and B24 for the three variants. The release notes name both.
- **Release:** it is released after it lands, before phase 5, as a patch, `v1.15.2`, which the owner confirms at the tag.

Phases 1 to 4 are complete and released as `v1.14.0`, `v1.15.0` and `v1.15.1`. The schema stays at `1.25.0` (the MADR, question 16).

## Standards this plan follows

| Id | Source | Text | Bears on |
| :--- | :--- | :--- | :--- |
| Q1 | `schema/schema.unstable.json`, `SubagentCapabilities` | "The client must advertise this capability before the agent sends subagent updates or session-directed messages." | B24 |
| Q2 | [0017-MADR-remediate-acp-v1-conformance-violations.md](0017-MADR-remediate-acp-v1-conformance-violations.md), step 4 of its PLAN | Unstable features whose gates the table does not state are not gated by that record; a gate the table can cite belongs in it. | B24 |
| Q3 | `agentclientprotocol/rust-sdk` at `d5e8bf4d`, `src/agent-client-protocol/src/jsonrpc.rs:1775-1777` | "On clean EOF, messages already accepted by the outgoing queue—including handler responses …—are drained". Pending outbound requests fail with "Incoming transport closed" (`:2363-2372`). | B23 |
| Q4 | the MADR, decision 12 | Phases ride the release after each lands; the version is confirmed at the tag. | release |

## Facts this plan rests on

Each was checked on `7c48c95` on 2026-10-18, read-only, with probes in scratch copies of the tree. Step 1 re-asserts each code fact
before any code changes.

| Finding | The code as it is | Re-check on `7c48c95` |
| :--- | :--- | :--- |
| B22, I10 | `ClientSideConnection.Prompt` (`client_gen.go:706-717`) calls `sendRequest`, and on an error with an ended context sends `c.Cancel(context.Background(), …)`. `sendRequest` (`connection.go:1566`) writes nothing when the context has already ended (`:1569-1572`, B9). `sendMessage` (`:1516`) returns the context's cause either before the writer goroutine takes the message, when nothing is written, or after, when the write runs to its end. The generator emits the cancel at `cmd/generate/internal/emit/dispatch.go:744-754`, and `TestDispatchFollowsTheModel` pins its text. | Probe: with an ended context, `session/cancel` is the first line written. |
| B23 | `receive` (`connection.go:1014`) calls `shutdownReceive` at the end of input. It cancels the connection's context first (`:1354`), and a request handler's context derives from it (`:1162`), so the handler sees it ended and its reply is refused by `sendMessage` (`:1518-1519`). A notification handler runs under `inboundCtx`, cancelled after the queue drains or `notificationQueueDrainTimeout` (5 s, `:22`) passes (`:1369-1373`). `Done`'s doc (`:1996-1999`) says every send fails once it is closed. | Probe: 0 of 50 requests answered. A prototype in a scratch copy answered 50 of 50: a channel closed at clean end of input fails pending requests in `waitForResponse`, and the inbound requests in flight are waited for, bounded by `notificationQueueDrainTimeout`, before `shutdownReceive`. Under `-race`, one root test failed, `TestConnectionCancelsRequestHandlersOnDisconnectEvenWithNotificationBacklog`, which pins today's end at a clean close; no other test failed or slowed by more than a second (the package took 13.2 s, where it took 12.3 s). |
| B24 | `SessionUpdate`'s variant gates are rules at `cmd/generate/internal/model/acp.go:153-155`, with their schema sentence in `srcSessionUpdate` (`:233-234`). `gates_test.go:499-501` tests `notice` and the compaction variants. `README.md:404-406` lists `subagents` among the unchecked features. | Probe: all three variants sent to a client without `subagents`; a received `session_message_chunk` reached the client's handler. |
| E18 | `example/client/main.go:84`, `example/claude-code/main.go:108` and `example/gemini/main.go:108` print `u.ToolCallUpdate.Status`, a `*acp.ToolCallStatus`, with `%v`. | Read; phase 4's step 4 record shows `updated: 0x…`. |

## Scope

In scope:

- `connection.go` and `client_gen.go`, the latter through the generator;
- the generator's `emit/dispatch.go` and `model/acp.go`, and the regenerated files;
- root tests, and `cmd/generate` tests that pin the changed output;
- the three example clients and their tests;
- `README.md` and `docs/architecture.md`;
- the release notes for `v1.15.2`;
- these records.

Out of scope:

- **Phase 5:** I1 to I7, I9, I11, I13 to I16 and E16's linters.
- **The schema:** `1.26.0` is not pinned (question 16).
- **Ending on a read error:** a read that ends with an error other than `io.EOF` still ends the connection at once. The output is then
  likely broken, and the decision names the end of input.
- **Closing issues #7 to #9:** left to the owner.

## Design

### B22 and I10: cancel only a prompt that was written

`sendMessage` reports whether it handed the message to the writer goroutine, and `sendRequest` carries that out through an
unexported variant, so the exported `SendRequest` and the typed methods keep their signatures:

- **Not written:** the request was not handed over when the context ended before the hand-over, when the request did not prepare, or
  when the connection had ended. Nothing was written.
- **Possibly written:** once it was handed over, the write runs to its end and may reach the peer.

The generated `Prompt` sends `session/cancel` only when the request may have been written and the call failed with an ended context.
It sends it under `context.WithoutCancel(ctx)`, which keeps the caller's values without its end, where `context.Background()` kept
neither (I10). The cancel's own write is still best effort.

### B23: answer the requests received before the end of input

At a clean end of input (`io.EOF`), the connection:

1. **Ends input.** It closes an unexported channel. A request waiting for its response then returns at once, with an error that keeps
   its code (`-32603`) and wraps `ErrPeerDisconnected` with "end of input". A request sent after that point is refused the same way
   and not written: no answer could arrive.
2. **Drains.** It lets the inbound requests already received run, under contexts that stay live, and writes their answers.
   Notifications sent meanwhile are written, and the notification queue drains as it does today.
3. **Bounds the drain.** It waits for those requests, bounded by `notificationQueueDrainTimeout`, the bound the notification drain
   already uses.
4. **Shuts down.** It runs `shutdownReceive` as today. That ends the connection's context, closes `Done`, and cancels the handlers still
   running, whose answers are then not written, as today.

`Close` during the drain ends the connection at once, as it does now. A read that ends with another error ends the connection at once,
as today. `Done`'s doc, the README's paragraph on how a connection's errors wrap their cause, and `docs/architecture.md`'s account of
the writer and the end of a connection state the new order.

The existing test that pins the old end at a clean close keeps its intent for a broken input: it closes the pipe with an error, which
still cancels the handlers at once. The clean close gets tests of its own, run in a `testing/synctest` bubble so the bound costs no
wall time:

- a request answered;
- a pending request failed at once;
- a new request refused unwritten;
- a stuck handler cancelled at the bound, its answer not written;
- `Close` during the drain.

### B24: gate the subagent updates

The protocol table gains three variant rules on `subagents`, beside `notice` and the compaction variants: `subagent_update`,
`session_message` and `session_message_chunk`. A new source constant holds Q1's sentence, and the generator's existing guard requires
it to be found in the schema. Amended 2026-10-18: there was no such guard; step 2 adds one ("Deviation, 2026-10-18, during step 2").

Each follows 0017's rules:

- **Sending:** a `*CapabilityError` naming `subagents` and the variant.
- **Receiving:** the notification is dropped and logged, which `WithInboundValidation` can override.

`gates_test.go` gains the three as value gates, both ways. The README's list of unchecked unstable features drops `subagents`.

### E18: print the status

Each example client prints the status's value, and nothing when it is absent, through a small function its test calls with a set
status and with none.

## Implementation Steps

Each step ends with its gates and an execution record, and the owner commits it; agents do not commit on `main`. Every new test is
seen to fail on a planted break in a scratch copy, never in the tree, before it is relied on.

1. **Records.** This PLAN and its `docs/README.md` row, with the MADR's amendment of 2026-10-18. The facts above are re-asserted on the
   tree.
2. **B24.** The rules and their source; regeneration; the gate tests; the README.
3. **B22 and I10.** `sendMessage` and `sendRequest`, then the generator's cancel and regeneration. `TestPromptWithEndedContextWritesNothing`
   uses the probe's marker. The existing `TestPromptCancellationSendsCancelAndAllowsNewSession` still requires the cancel after a
   written prompt. `TestDispatchFollowsTheModel` pins the new emitted line.
4. **B23.** The end of input as "Design" sets it; its tests; the changed test; `Done`'s doc, the README and `docs/architecture.md`.
   The design was prototyped (see "Facts"). If the implementation finds it does not hold, the step stops as a deviation.
5. **E18.** The three example clients and their tests.
6. **Release notes.** `docs/reports/0019-REPORT-release-notes-v1-15-2.md` and its `docs/README.md` row: B23's behaviour change, B24's
   gates, B22, and the examples; no exported API changes.
7. **Verification and close.** The gates over the tree; each issue's probe re-run, each passing; the API listing against `v1.15.1`,
   unchanged; Windows. The pre-push guard runs (`make push-guard`), and the owner pushes. After CI passes on the pushed commits, this
   PLAN and its row become `complete`. The owner tags `v1.15.2`, publishes its release from the report, and closes issues #7 to #9.

## Verification

Per code step, through the Python runner outside the tree:

- regeneration: the generated files' diff is exactly the step's;
- `make pre-add-check`, and `scripts/go-precheck.sh` on each new `.go` file;
- `GOEXPERIMENT=nojsonv2 go test ./...`, and `GOOS=windows go vet ./...` in both modules;
- each new test seen to fail on its planted break in a scratch copy;
- `make check-records`, `make markdownlint` and the identifier scan;
- Windows `go test -count=1 -shuffle=on ./...` in both modules;
- the API listing against `v1.15.1`.

The tests that use a context or a bubble also run `-count=50 -race`.

Planted breaks, at least:

| Finding | Break |
| :--- | :--- |
| B22 | the cancel sent whatever was written; the cancel sent under `context.Background()` (seen by `TestDispatchFollowsTheModel`) |
| B23 | the end of input cancelling the handlers at once, as now; pending requests left waiting; a new request written after the end of input; the drain unbounded |
| B24 | each variant's rule removed; the source sentence altered (refused by the guard step 2 adds, as the deviation of 2026-10-18 records) |
| E18 | the status printed as its pointer |

Expected API listing difference against `v1.15.1`: none.

Acceptance:

- every step's checks pass and its record is written;
- the three issue probes pass on the tree, unchanged from the MADR's evaluation;
- the root suite passes with the changed test and no other change in what it asserts;
- CI passes on the pushed commits.

## Rollout and Rollback

Each step is one commit, reverted by reverting it.

The release is a patch: no exported API changes. Two behaviours change, and the release notes name them:

- **End of input:** a request received before it is answered, and a pending request fails at once.
- **Subagent updates:** they are refused or dropped without `subagents`.

An application that relied on its handlers being cancelled at once by a clean end of input sees them run up to the bound. Reverting
B23 restores that.

## Execution record

### Step 1, records, 2026-10-18

The owner committed this PLAN, its `docs/README.md` row and the MADR's amendment of 2026-10-18 as `3765e19`, and approved the PLAN.
This PLAN and its row are now `in-progress`.

Each code fact the table "Facts this plan rests on" cites was re-asserted on `3765e19`. A script outside the tree checks that each
cited line holds its snippet: 31 facts. On the first run, 27 held; four citations were one line early:

| Citation | Correct line |
| :--- | :--- |
| `sendMessage` at `connection.go:1515` | `:1516` |
| its refusal at `:1517-1518` | `:1518-1519` |
| a request's context at `:1161` | `:1162` |

The last of these also appears in the MADR's "Evaluated and decided with the owner, 2026-10-18". Each was corrected in place, in both
records, and the second run found 31 of 31 holding.

The script was seen to fail on a planted break: run on the `v1.15.0` tree, whose lines differ, it held 21 of 31 and exited 1.

The issue probes of the MADR's evaluation, rerun when step 7 verifies, are kept outside the tree with the probe of step 4's prototype.

Gates on the tree: `make check-records` and `make markdownlint` exit 0; the identifier scan finds nothing. No Go file changed.

The owner committed step 1's record as `b092edb`.

### Deviation, 2026-10-18, during step 2

**Found.** B24's design and the planted-break table say that the generator's existing guard refuses a capability rule's source
sentence that is not found in the schema. There is no such guard. The model requires only that a source be non-empty
(`cmd/generate/internal/model/gates_test.go:122`). The capability rules quote two kinds of source:

- **Web pages:** `srcRootsGate`, `srcMcp`, `srcPrompt`, `srcAuthenticate`, `srcElicitation` and `srcBooleanOption`.
- **The schema:** `srcSessionUpdate`, `srcTerminalAuth`, and the new source of B24.

So the planted break "the source sentence altered" could not fail.

**Decision (owner, 2026-10-18).** Step 2 adds the guard: a model test requires each capability rule's source to appear, with whitespace
normalised, in the committed schema's descriptions. The web-page sources are exempt, each named with the page it quotes, and an
exemption no rule uses fails the test. The planted break then holds. "Design", B24, and the planted-break row are annotated.

The test covers the capability rules only, as decided. The schema-quoting sources of the patch and null-retained rules are not
covered by it. No decision or fact of the MADR changes.

### Step 2, B24, the subagent updates gated, 2026-10-18

What changed, on `b092edb`:

- **The protocol table** (`cmd/generate/internal/model/acp.go`) gains three variant rules on `SessionUpdate`: `subagent_update`,
  `session_message` and `session_message_chunk`, each on `capSubagents` (`"subagents"`), a new constant like `capRoots`. Their
  source, `srcSubagents`, is Q1's sentence.
- **`types_gen.go`**, regenerated: `(*SessionUpdate).gates` gains the three checks, a `*CapabilityError` naming `subagents` and the
  variant. The diff is those 21 lines.
- **The guard the deviation of 2026-10-18 adds:** `TestCapabilitySourcesQuoteTheSchema` (`cmd/generate/internal/model/sources_test.go`,
  new) requires each capability rule's source to appear, with whitespace collapsed, in a description of the committed schema
  files. The exceptions are the six sources quoted from the specification's pages, each listed with its page, and the test also
  fails on a listed page source that no rule uses.
- **Root tests:**
  - `TestValueGates` gains the three variants as value gates, sent and received with the capability and without it, so it holds 22
    gates, where it held 19.
  - `allAdvertised` advertises `subagents`, as its doc requires it to advertise every capability the table names.
- **Generator tests:**
  - `checkCommittedGates` counts 25 checks from 22 rules, where it counted 22 from 19.
  - The four engine fixtures that mirror `SessionUpdate`'s gated variants gain the three variants and `ClientCapabilities.subagents`
    (`accept-capability-gates`, `accept-untyped-object-gates`, `refuse-gated-in-patch` and `refuse-gated-property-not-union`).
  - Their `want.api` was rewritten with `-update`. The difference is the three variant types and the `Subagents` field. The two refuse
    cases still refuse with their own errors.
- **`README.md`**: the gate list names the three variants, and the list of unchecked unstable features no longer names `subagents`.

The first pre-add check failed on `goconst`: `"subagents"` appeared three times in the table. That is why the constant
`capSubagents` was added. The planted breaks below were re-run against the rules as committed.

Issue #9's probe, from the MADR's evaluation, was run on scratch copies:

- **The fix:** all four of its cases pass.
- **`b092edb`:** all four fail.

The probe now builds its record without `subagents` explicitly, since `allAdvertised` now advertises it. What it checks is unchanged.

Planted breaks, each in a fresh scratch copy; those in the table regenerate in the copy (6 of 6 seen to fail):

| Break | What failed |
| :--- | :--- |
| the `subagent_update` rule removed | `TestValueGates/subagents_subagent_update`: `send = <nil>`, 136 bytes written |
| the `session_message` rule removed | `TestValueGates/subagents_session_message`: `send = <nil>`, 133 bytes written |
| the `session_message_chunk` rule removed | `TestValueGates/subagents_session_message_chunk`: `send = <nil>`, 176 bytes written |
| `srcSubagents` reworded | `TestCapabilitySourcesQuoteTheSchema`: no description contains it, for each of the three rules |
| `srcSubagents`'s "must" written "MUST" | `TestCapabilitySourcesQuoteTheSchema`, the same three |
| an unused page source listed | `TestCapabilitySourcesQuoteTheSchema`: used by no capability rule |

`go test -race -count=50` over the three new value gates: 50 PASS each.

The plant runner was rewritten to its former behaviour for this step, after the cleanup of the host's temporary directory that
step 9 of the phase 4 PLAN records.

Gates, through the Python runner, on the tree:

- regenerating changes no generated file;
- `make pre-add-check` exit 0, and `scripts/go-precheck.sh` on the new file exit 0;
- `GOEXPERIMENT=nojsonv2 go test ./...` exit 0;
- `GOOS=windows go vet` exit 0 in both modules;
- `make check-records` and `make markdownlint` exit 0;
- the identifier scan finds nothing;
- Windows `go test -count=1 -shuffle=on ./...` exit 0 at the root and in `cmd/generate`;
- the API listing: 2429 lines at `v1.15.1` and in the tree; 0 removed, 0 added.

The owner committed step 2's record as `ef08400`.

### Step 3, B22 and I10, a cancel only for a prompt that was written, 2026-10-18

What changed, on `ef08400`:

- **`connection.go`:** `sendMessage` keeps its signature and calls a new `sendMessageSent`. It reports whether the message was handed
  to the writer goroutine:
  - false on every return before the hand-over: the connection had ended, the message did not encode, or the context ended first;
  - true after it, whatever the error, since the write then runs to its end.

  `sendRequest` keeps its signature and calls a new generic `sendRequestSent`, which reports the same: false for a context already
  ended (B9) or a request that did not prepare; `sendMessageSent`'s answer for the write; true once the request was handed over. The
  exported `SendRequest` and the typed methods are unchanged.
- **The generator** (`emit/dispatch.go`): a prompt-turn method with a canceller calls `sendRequestSent` and sends its cancel only
  when `sent && ctx.Err() != nil`, under `context.WithoutCancel(ctx)` (I10), where it sent it under `context.Background()` whenever
  `ctx.Err() != nil`. Other methods still call `sendRequest`.
- **`client_gen.go`**, regenerated: `Prompt`'s call and its cancel, three changed lines.
- **Tests:**
  - `TestPromptWithEndedContextWritesNothing` (`connection_cancel_test.go`) is issue #7's probe as a test: with an ended context,
    `Prompt` returns `-32800` and writes nothing, and a marker sent after it is the first line written.
  - `TestPromptEndedBeforeHandOverSendsNoCancel` is added for the race B22 is about: in a `synctest` bubble, a peer that stopped
    reading holds the writer, and `Prompt`'s context ends while its request waits for the hand-over. `Prompt` returns `-32800`.
    Before the fix it sent the cancel and blocked on the same writer.
  - The existing `TestPromptCancellationSendsCancelAndAllowsNewSession` still requires the cancel after a written prompt.
  - `TestDispatchFollowsTheModel` pins the emitted `sendRequestSent` call and the cancel under `context.WithoutCancel(ctx)`.

The second test is in addition to the one "Implementation Steps" names. It covers the hand-over path the first does not reach.

Issue #7's probe, from the MADR's evaluation, run on scratch copies:

- **The fix:** it passes.
- **`ef08400`:** it fails: "the first line written is \"session/cancel\"".

Planted breaks, each in a fresh scratch copy; the emitter's regenerates in the copy (4 of 4 seen to fail):

| Break | What failed |
| :--- | :--- |
| `sendRequestSent` reports `sent` for a context already ended | `TestPromptWithEndedContextWritesNothing`: wrote "session/cancel" |
| `sendMessageSent` reports `sent` when the context ends before the hand-over | `TestPromptEndedBeforeHandOverSendsNoCancel`: "deadlock: all goroutines in bubble are blocked" |
| the emitted cancel never sent (`sent && false`) | `TestPromptCancellationSendsCancelAndAllowsNewSession`: timeout waiting for session/cancel |
| the cancel under `context.Background()` | `TestDispatchFollowsTheModel`: `client_gen.go` lacks the `context.WithoutCancel(ctx)` line |

`TestPromptEndedBeforeHandOverSendsNoCancel` was also run on a scratch copy with `connection.go` and `client_gen.go` as at
`ef08400`. It failed with the same deadlock.

`go test -race -count=50` over the three tests of the cancel: 50 PASS each.

Gates, through the Python runner, on the tree:

- regenerating changes no generated file;
- `make pre-add-check` exit 0;
- `GOEXPERIMENT=nojsonv2 go test ./...` exit 0;
- `GOOS=windows go vet` exit 0 in both modules;
- `make check-records` and `make markdownlint` exit 0;
- the identifier scan finds nothing;
- Windows `go test -count=1 -shuffle=on ./...` exit 0 at the root and in `cmd/generate`;
- the API listing: 2429 lines at `v1.15.1` and in the tree; 0 removed, 0 added.

The owner committed step 3's record as `2cdc39a`.

### Step 4, B23, the requests received before the end of input answered, 2026-10-18

What changed, on `2cdc39a`, as "Design", B23, sets it. The design was implemented as prototyped, with three additions the prototype
lacked: an answer read before the end of input is still delivered; a request sent after it is refused unwritten; and the bound's timer
is stopped.

**`connection.go`.**

- `Connection` gains `inputDone`, closed at a clean end of input, and `inboundRequests`, which counts each received request from its
  dispatch until its answer is written.
- At `io.EOF`, `receive` calls a new `drainInput` before `shutdownReceive`:
  1. It closes `inputDone`.
  2. It waits for `inboundRequests`, bounded by `notificationQueueDrainTimeout` (5 s) with a stopped timer, or until `Close` ends the
     connection.
  3. If it reaches the bound, it logs at Debug that requests were still running.
- A read error other than `io.EOF` shuts down at once, as before.
- **Pending requests:** `waitForResponse` returns at once when `inputDone` closes, with `inputEndedError()`: `-32603`, data
  `{"error": "input ended before a response"}`, whose cause `errInputEnded` wraps `ErrPeerDisconnected` with "end of input". An answer
  already read is preferred over it.
- **New requests:** `sendRequestSent` and `SendRequestNoResult` return the same error after the end of input, before writing anything.
- **Handlers:** request handlers keep the connection's context through the drain. `shutdownReceive` then cancels the ones still
  running, as before. Their contexts end with the connection's own cause, "peer connection closed", unchanged.
- **`Done`'s doc** states the new order.

**Tests** (`end_of_input_test.go`, new):

| Test | What it requires |
| :--- | :--- |
| `TestRequestAnsweredAtEndOfInput` | 20 runs of a request followed by end of input: each answered before `Done`, its handler under a live context. |
| `TestPendingRequestAtEndOfInput` | A pending request fails with the end-of-input error; one answered before the end returns its answer. |
| `TestRequestAfterEndOfInputNotWritten` | In a `synctest` bubble, a handler still running sends `SendRequest` and `SendRequestNoResult` after the end: both fail at once, unwritten, and its own answer is the only line. |
| `TestEndOfInputDrainIsBounded` | In a bubble, a handler that never returns holds the connection for the bound only, then its context is cancelled and its answer not written. |
| `TestCloseEndsEndOfInputDrain` | In a bubble, `Close` during the drain ends it at once, the reader's shutdown included, even with a handler that ignores its context. |

**The changed test.** `TestConnectionCancelsRequestHandlersOnDisconnectEvenWithNotificationBacklog` closed its input cleanly and
required the handlers cancelled within two seconds, today's end, which B23 changes, as "Facts" found. It now closes its input with an
error, so it still pins the prompt cancel on a broken input. Its doc says why.

**Docs.** `README.md` ("Closing and limits") and `docs/architecture.md` state the drain.

**Issue #8's probe**, from the MADR's evaluation, run on scratch copies:

- **The fix:** it passes, answering 50 of 50.
- **`2cdc39a`:** it fails: "0 of 50 requests answered when input ended after them; 50 handlers saw their context done".

Planted breaks, each in a fresh scratch copy (7 of 7 seen to fail):

| Break | What failed |
| :--- | :--- |
| the end of input ending the connection at once, as before | `TestRequestAnsweredAtEndOfInput`: the handler ran under an ended context |
| the count of received requests not kept | `TestRequestAnsweredAtEndOfInput`, the same |
| a pending request left waiting at the end of input | `TestPendingRequestAtEndOfInput`: it failed later, as the connection's end ("connection ended before response") |
| `SendRequest` after the end of input written | `TestRequestAfterEndOfInputNotWritten`: the first line written was that request |
| `SendRequestNoResult` after the end of input written | `TestRequestAfterEndOfInputNotWritten`, the same |
| the drain unbounded | `TestEndOfInputDrainIsBounded`: the bubble deadlocked |
| `Close` not ending the drain | `TestCloseEndsEndOfInputDrain`: "the reader had not shut down after Close" |

**The first form of `TestCloseEndsEndOfInputDrain`** passed the last break. Its handler waited on its context, which `Close` cancels,
so the drain ended either way. It was reworked so the handler ignores its context, and the test reads the shutdown's "connection
closed" log line. It then failed on the break.

**What no test pins.** Preferring an answer already read over the end of input cannot be told apart by a test. A caller parked in
`waitForResponse` is woken by whichever arrives first, and the answer is read before the end of file, so the preference only matters
for a caller not yet parked. Amended 2026-10-18: the caller is not always parked, as CI showed. `TestPendingRequestAtEndOfInput`
then fails, rarely: about 1 run in 2000 without the preference on `Done` ("Deviation, 2026-10-18, after the close").

**Repeat runs.** `go test -race -count=50` over the five new tests and the changed one: 50 PASS each. The root package under `-race`
took 11.5 s; the prototype's comparison is in "Facts".

**Gates**, through the Python runner, on the tree:

- regenerating changes no generated file;
- `make pre-add-check` exit 0, and `scripts/go-precheck.sh` on the new file exit 0;
- `GOEXPERIMENT=nojsonv2 go test ./...` exit 0;
- `GOOS=windows go vet` exit 0 in both modules;
- `make check-records` and `make markdownlint` exit 0;
- the identifier scan finds nothing;
- Windows `go test -count=1 -shuffle=on ./...` exit 0 at the root and in `cmd/generate`;
- the API listing: 2429 lines at `v1.15.1` and in the tree; 0 removed, 0 added.

The owner committed step 4's record as `5763a36`.

### Step 5, E18, the tool call status printed, 2026-10-18

What changed, on `5763a36`:

- **The three example clients** (`example/client`, `example/claude-code`, `example/gemini`) each gain `toolCallUpdateLine`. It returns
  the line printed for a tool call update: its id and its status's value, or the id alone when the update carries no status. It
  replaces `Printf("… updated: %v", …, u.ToolCallUpdate.Status)`, which printed the `*acp.ToolCallStatus` pointer.
- **Tests:** each client's `main_test.go` gains `TestToolCallUpdateLine`, covering a set status (`completed`) and none. Like the tests
  beside it, it does not call `t.Parallel`.

`gocritic`'s `sprintfQuotedString` refused the first form, which wrote the id in a backquoted `%s`. The id is now formatted with
`%#q`, which gives the same backquoted text for these ids, and lint over the examples is clean for the host and for Windows.

Planted breaks, each in a fresh scratch copy (6 of 6 seen to fail):

| Break | What failed |
| :--- | :--- |
| each client printing the status with `%v` of its pointer, as before | `TestToolCallUpdateLine`: "updated: 0xc0001c2320", want "updated: completed" (each client, its own address) |
| each client without the absent status's case | `TestToolCallUpdateLine`: "invalid memory address or nil pointer dereference" |

Gates, through the Python runner, on the tree:

- regenerating changes no generated file;
- `make pre-add-check` exit 0, which lints the examples;
- `GOEXPERIMENT=nojsonv2 go test ./...` exit 0;
- `GOOS=windows go vet` exit 0 in both modules;
- `make check-records` and `make markdownlint` exit 0;
- the identifier scan finds nothing;
- Windows `go test -count=1 -shuffle=on ./...` exit 0 at the root, the example packages included, and in `cmd/generate`;
- the API listing: 2429 lines at `v1.15.1` and in the tree; 0 removed, 0 added.

The owner committed step 5's record as `f260ac8`.

### Step 6, release notes, 2026-10-18

What changed, on `f260ac8`:

- [0019-REPORT-release-notes-v1-15-2.md](../reports/0019-REPORT-release-notes-v1-15-2.md), new: the text of the GitHub release for
  `v1.15.2` and the sources behind it, in the form of the `v1.15.1` notes.
- Its row in `docs/README.md`.

The release text has three sections:

- **Behaviour changes:** B23's drain at the end of input, and B24's gates. "Rollout and Rollback" says the notes must name both.
- **Fixes:** B22 with I10, and E18.
- **For contributors:** the source guard of step 2.

Each statement cites its commit and finding.

Facts re-read for this step:

- `git log v1.15.1..HEAD`: seven commits.
- `git log v1.15.1..HEAD -- '*.go' ':!cmd/generate'`: `ef08400`, `2cdc39a`, `5763a36`, `f260ac8`.
- `git diff --stat v1.15.1 HEAD` over the root's non-test, non-example Go files: `client_gen.go`, `connection.go` and `types_gen.go`,
  139 lines added and 22 removed.
- `git diff v1.15.1 -- schema/`: empty.
- The API listing: 0 removed, 0 added (step 5's gates).

Gates on the tree: `make check-records` and `make markdownlint` exit 0; the identifier scan finds nothing. No Go file changed in this
step.

The owner committed step 6's record as `771a67f`.

### Step 7, verification, 2026-10-18

Run over the finished tree at `771a67f`, with a clean working tree, through the Python runner.

- **Gates:**
  - regenerating changes no generated file;
  - `make pre-add-check` exit 0;
  - `GOEXPERIMENT=nojsonv2 go test ./...` exit 0;
  - `GOOS=windows go vet` exit 0 in both modules;
  - `make check-records` and `make markdownlint` exit 0;
  - the identifier scan finds nothing;
  - Windows `go test -count=1 -shuffle=on ./...` exit 0 at the root and in `cmd/generate`.
- **API listing:** 2429 lines at `v1.15.1` and at `771a67f`; 0 removed and 0 added, exactly "Verification"'s expectation.
- **The issues' probes:** the three probes of the MADR's evaluation, run on `git archive` of each commit with the probe file added:

  | Probe | `771a67f` | `v1.15.1` |
  | :--- | :--- | :--- |
  | #7, `Prompt` with an ended context writes nothing | PASS | FAIL |
  | #8, requests before the end of input answered | PASS | FAIL |
  | #9, the subagent updates gated both ways | PASS | FAIL |

  Since step 2, the #9 probe names its record without `subagents` itself, as that step records.

Every acceptance criterion in "Verification" is met on the tree, apart from CI on the pushed commits. After the push and CI, this
PLAN and its `docs/README.md` row become `complete`. The owner then tags `v1.15.2`, publishes its release from
[0019-REPORT-release-notes-v1-15-2.md](../reports/0019-REPORT-release-notes-v1-15-2.md), and closes issues #7 to #9.

The pre-push disclosure guard runs through `make push-guard` over the commits not yet pushed, `3765e19` to this record's commit, when
the owner is asked to push.

The owner committed step 7's record as `e084599`.

### Close, 2026-10-18

**The push.** `make push-guard` ran over the eight commits not yet pushed, `7c48c95..e084599`, after a fetch, and passed, exit 0. The
owner pushed `e084599`. CI run `39175000805` on it concluded `success` on its first attempt, on `validate (ubuntu-24.04)`, `validate
(macos-15)` and `validate (windows-2025)`.

**Acceptance.** Every criterion in "Verification" is met:

- each step's checks passed and its record is written;
- the three issue probes pass on the tree and fail on `v1.15.1`, as step 7 records;
- the root suite passes; the one changed test keeps its intent for a broken input;
- CI passed on the pushed commits.

This PLAN and its `docs/README.md` row are `complete`. The MADR stays `accepted`, with phase 5 to come. Superseded: CI on this
close failed, and the PLAN reopened ("Deviation, 2026-10-18, after the close").

**Left to the owner.**

- Tag `v1.15.2`, an annotated tag as the earlier ones are, on this close commit once its CI passes. That is where `v1.15.1`'s tag went,
  and the code is the same as at `e084599`.
- Publish its release from the release text of
  [0019-REPORT-release-notes-v1-15-2.md](../reports/0019-REPORT-release-notes-v1-15-2.md), with its relative link rewritten to the
  tag.
- Close issues #7, #8 and #9, naming `2cdc39a`, `5763a36` and `ef08400`.

The owner committed the close as `63eb624` and pushed it.

### Deviation, 2026-10-18, after the close

**Found.** CI run `39175773059` on `63eb624` failed on its first attempt:

- **The failure:** `validate (ubuntu-24.04)`, step "race detector". `TestPendingRequestAtEndOfInput` (`end_of_input_test.go:91`) reported
  "error data map[error:connection ended before response], want the end of input". `validate (macos-15)` and `validate (windows-2025)`
  passed.
- **Not transient:** `go test -race -count=500 -run '^TestPendingRequestAtEndOfInput$'` fails 28 times in 500 on the tree, about one
  run in 18. Step 4's 50 runs all passed, which at that rate happens about one time in twenty.

**The cause** is in step 4's code:

- At a clean end of input with no request received in flight, `drainInput` returns at once, and `shutdownReceive` then ends the
  connection's context.
- A caller still in `waitForResponse` can find both `inputDone` and `Done` closed. Go's `select` picks between ready cases at random, so
  it sometimes returns the connection's end ("connection ended before response") rather than the end of input.
- Both are `-32603` wrapping `ErrPeerDisconnected`, so the documented contract held. The error data the release notes describe did not.

**Decision (owner, 2026-10-18).** `waitForResponse` reports the end of input whenever the input has ended, whichever signal it sees.
No test is loosened. The close above is superseded: this PLAN and its row go back to `in-progress`, and a new close follows once CI
passes on the fix. The owner had not tagged `v1.15.2`. No decision or fact of the MADR changes.

### The fix of the deviation after the close, 2026-10-18

What changed, on `63eb624`: `waitForResponse` (`connection.go`) has two changes.

- **The end of input reported.** Its `ended` error, which the connection-end branches return, is `inputEndedError()` whenever the input
  has ended. A request pending at the end of input then gets the end-of-input error, whichever of `inputDone` and `Done` the select
  takes.
- **An answer already read preferred.** The `inputDone` branch and the `Done` branch first take an answer already read, through a
  shared `answered` check. Step 4 did that in the `inputDone` branch only.

**Found while fixing.** With the first change alone, `go test -race -count=2000` still failed 6 times in 2000. The test's other case was
losing an answer read before the end of input, when the select took `Done`. That is the same race, against the design step 4 already
states ("an answer read before the end of input is delivered"), so it was fixed under the same decision.

**Seen to fail.** Run on scratch copies of the tree with one half of the fix undone, `go test -race -count=2000 -run
'^TestPendingRequestAtEndOfInput$'`:

| Undone | Failures |
| :--- | :--- |
| the end of input reported on `Done` | 46 of 2000: "error data map[error:connection ended before response], want the end of input" |
| an answer preferred on `Done` | 1 of 2000: "answered before the end of input: … input ended before a response …, want the answer" |

The tree before the fix failed 28 of 500.

**With the fix:**

- `TestPendingRequestAtEndOfInput`, three runs of `-count=2000`: 6000 PASS.
- Every test this PLAN added or changed, `-count=1000 -race`: all pass. These are the five of step 4 and the changed one, the three
  of step 3, and `TestValueGates`, whose new gates are step 2's.
- The root package, `-count=10 -race`: all pass.

Gates, through the Python runner, on the tree:

- regenerating changes no generated file;
- `make pre-add-check` exit 0;
- `GOEXPERIMENT=nojsonv2 go test ./...` exit 0;
- `GOOS=windows go vet` exit 0 in both modules;
- `make check-records` and `make markdownlint` exit 0;
- the identifier scan finds nothing;
- Windows `go test -count=1 -shuffle=on ./...` exit 0 at the root and in `cmd/generate`;
- the API listing: 2429 lines at `v1.15.1` and in the tree; 0 removed, 0 added.

The release notes are unchanged: they describe the behaviour this fix makes deterministic. The new close follows CI on the pushed
fix.

The owner committed the fix as `3fb248d`.

### Close, after the fix, 2026-10-18

**The CI runs.**

- **On `63eb624`:** the owner re-ran the failed job. Attempt 2 of run `39175773059` passed on all three runners, as a race that fails
  about one run in 18 lets it. It is not taken as evidence of a fix.
- **The push of the fix:** `make push-guard` ran over the one commit not yet pushed, `63eb624..3fb248d`, and passed, exit 0. The owner
  pushed `3fb248d`.
- **On `3fb248d`:** CI run `39176657417` concluded `success` on its first attempt, on `validate (ubuntu-24.04)`, `validate (macos-15)`
  and `validate (windows-2025)`.

**Acceptance.** Every criterion in "Verification" is met:

- each step's checks passed and its record is written, the fix of the deviation after the close included;
- the three issue probes pass on the tree and fail on `v1.15.1` (step 7); the fix changes only `waitForResponse`, under the end-of-input
  tests' 6000 runs;
- the root suite passes, 10 runs under `-race`;
- CI passed on the pushed commits.

This PLAN and its `docs/README.md` row are `complete`. The MADR stays `accepted`, with phase 5 to come.

**Left to the owner.**

- Tag `v1.15.2`, an annotated tag, on this close commit once its CI passes.
- Publish its release from the release text of
  [0019-REPORT-release-notes-v1-15-2.md](../reports/0019-REPORT-release-notes-v1-15-2.md), extracted with its relative link rewritten to
  the tag.
- Close issues #7, #8 and #9, naming `2cdc39a`, `5763a36` with `3fb248d`, and `ef08400`.

### Publication, 2026-10-18

**The close.** The owner committed the close as `f7defb4` and pushed it. Git's pre-push hook ran the disclosure guard on the push. CI
run `39177055258` on it concluded `success` on its first attempt, on all three runners.

**What was published.** The owner tagged `f7defb4` `v1.15.2`, an annotated tag, and pushed the tag. CI run `39177390304`, started by the
tag, concluded `success` on its first attempt, on `validate (ubuntu-24.04)`, `validate (macos-15)` and `validate (windows-2025)`. The
owner published the GitHub release `v1.15.2`, marked latest, from the release text of
[0019-REPORT-release-notes-v1-15-2.md](../reports/0019-REPORT-release-notes-v1-15-2.md), extracted with its one relative link
rewritten to the tag.

**Checks.** A Python script outside the tree, the `v1.15.1` check with its tag and commit changed, checked the publication. It used a
scratch consumer, `GOPROXY` the default `https://proxy.golang.org,direct`, a scratch `GOMODCACHE` and `GOPATH`, and the checksum
database on. 9 of 9 checks hold:

- the release is not a draft or a prerelease, its body is the extracted text, and it is GitHub's latest;
- the tag is annotated and names `f7defb4`;
- `main` on GitHub is the local `main`, and CI on it succeeded;
- the proxy's `@v/v1.15.2.info` gives the origin `refs/tags/v1.15.2` at `f7defb4`;
- `go list -m github.com/maccavelli/acp-go-sdk2@latest` gives `v1.15.2`;
- `go mod download` of `v1.15.2` exits 0.

**Seen to fail.** Its first run held 8 of 9 and exited 1. The latest CI run on `f7defb4`, the one the tag started, was still in
progress, and it passed once that run concluded.

**The issues.** Issues #7, #8 and #9 were still open when this was checked. The owner then closed them, each naming its fix: #7
`2cdc39a`; #8 `5763a36` and `3fb248d`; #9 `ef08400`. Nothing of this PLAN is left open.
