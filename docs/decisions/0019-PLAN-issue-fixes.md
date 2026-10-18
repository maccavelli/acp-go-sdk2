---
status: in-progress
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
