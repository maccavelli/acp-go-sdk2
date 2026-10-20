---
status: in-progress
date: 2026-10-19
associated-madr: "0024-MADR-go-sdk-best-practices-on-a-go-1-26-9-floor.md"
---

# Implement phase 3 of 0024: behaviour fixes

Associated MADR: [0024-MADR-go-sdk-best-practices-on-a-go-1-26-9-floor.md](0024-MADR-go-sdk-best-practices-on-a-go-1-26-9-floor.md)

## Goal

Implement phase 3 of the MADR's outcome, as the owner decided it on 2026-10-19. It covers findings A1, A2, A3, A4 and A14, with the
answers to questions 1, 3 and 5:

* **A1 (question 1):**
  * `NewConnection`, `NewAgentSideConnection` and `NewClientSideConnection` panic at once, naming the argument, on a nil agent, client,
    reader or writer.
  * `NewConnection` keeps answering `-32601` for a nil handler, documented.
  * A panicking handler keeps ending the process, and the docs of `Agent`, `Client` and `MethodHandler` say so.
* **A2 (question 3):** the docs of `Agent` and `Client`, and the README's "Errors on the wire", say that a handler's error text reaches
  the peer as it is. An application that must not leak a message returns a `*RequestError` of its own.
* **A3 (question 5):** the log lines that carry an id the connection could not canonicalise log its size, not its bytes.
* **A4 (question 5):** faults the peer causes log at Warn, and each connection's end at Debug.
* **A14 (question 5):** the validation messages that quote the value they refuse stay, documented.

The exported API does not change; the API listing against `v1.16.3` must show no difference. Three behaviours change, and the release
notes name each:

* a nil argument panics in the constructor instead of on a connection goroutine;
* eleven log lines move from Error to Warn, as does a refused notification's, and three of them lose their id's bytes;
* the end of a connection logs at Debug instead of Info.

The release is proposed as the minor `v1.17.0`, which the owner confirms at the tag.

## Facts this plan rests on

Each was read on `d6cab27` on 2026-10-19. Step 1 re-asserts the code facts with a script.

| For | Fact |
| :--- | :--- |
| A1 | `NewConnection` (`connection.go:578`) and the generated `NewAgentSideConnection` (`agent_gen.go:32`) and `NewClientSideConnection` (`client_gen.go:28`) check no argument. `cmd/generate/internal/emit/dispatch.go:111-213` (corrected in step 1 to `:111-209`) (`agentSide`, `clientSide`,
`emitSideConnection`) emits the two side constructors; `emit/dispatch_test.go:200` and `:212` pin their bodies. |
| A1 | A probe in a scratch copy of `d6cab27`, each case in its own test process: a nil reader panics at `connection.go:1049`, a nil writer at `connection.go:687`, a nil agent answering `initialize` at `agent_gen.go:344`, and a nil client given a `session/update` after initialize at `client_gen.go:230`. Each is a nil pointer dereference on a connection goroutine, and each ends the process, exit 1. |
| A1 | `handleInbound` (`connection.go:1570`) answers a request `-32601` when the handler is nil and drops a notification. The non-test code has no `recover()`. |
| A1 | `MethodHandler`'s doc is at `connection.go:279`. The docs of `Agent` and `Client` are in the protocol table (`cmd/generate/internal/model/protocol.go:189` and `:202`). |
| A2 | `toReqErr` (`errors.go:227-243`) answers an error that is not a `*RequestError` `-32603` with `{"error": err.Error()}`, and one that wraps `context.Canceled` `-32800` the same way. A notification handler's error is logged, not sent (`connection.go:1591`). |
| A2 | The README's "Errors on the wire" is at line 556. Its sentence "a call whose context ended wraps `context.Canceled` or `context.DeadlineExceeded`" predates phase 2's finding that the call wraps `context.Cause(ctx)` ([0024-PLAN-documentation.md](0024-PLAN-documentation.md), A6 annotation). |
| A3 | Three lines log an id that could not be canonicalised as `string(…)`: an inbound request's id (`connection.go:1275`), a response's id (`:1506`), and a `$/cancel_request`'s `requestId` (`:1546`). The first two have passed `validID`, which reads only the first byte, so they are JSON strings or numbers; a number whose canonical form fails, such as one too large, reaches the log in full. The `requestId` can be any JSON value up to the line limit. B15 ([0019-PLAN-behaviour-fixes.md](0019-PLAN-behaviour-fixes.md)) logs a malformed line's `"size", len(line)` instead of its bytes. |
| A4 | Sixteen calls log at Error. Eleven log a fault of the peer's message: `connection.go:1074` (line over the limit), `:1200` (not JSON), `:1207` (not an object), `:1217` (invalid request), `:1275` (inbound id), `:1408` (response without id), `:1432` (malformed response), `:1506` (response id), `:1536` (`$/cancel_request` params), `:1540` (no `requestId`), `:1546` (`requestId`). |
| A4 | Of the other five: `:1339` is a full notification queue closing the connection; `:1591` is a notification handler's error, which is the peer's fault when the dispatch refused the notification (`-32601`, `-32602`) and the application's otherwise; `:2061` is a handler's result that fails validation; `agent_gen.go:356` and `:373` are an initialize answer the agent's own handler made invalid. The connection's end logs at Info (`connection.go:1476`). |
| A4 | `TestWithLoggerReceivesEarlyDiagnostics` (`connection_logger_test.go:38`) and `end_of_input_test.go:232` wait for "connection closed" through a text handler at its default level, Info. |
| A14 | 55 generated sites build a `ValidationError` in `types_gen.go`. 24 put the value they refuse into the message: 18 absolute-path checks (`emit/types.go:1213`, `:1222`, `%q is not an absolute path`) and 6 excluded-value checks. The other 31 do not: 25 one-variant, 4 one-member and 2 minimum-length checks. `ValidationError` also holds the value in its `Value` field (`errors.go:94`). |
| — | `scripts/check_records.py --next` gives `0025`; this PLAN takes the MADR's number. |

**Corrections to the MADR's findings,** which step 1 annotates:

* **A3** names two lines. The response id's line, `:1506` now, is a third of the same kind.
* **A4** lists nine lines. The inbound and response id lines, `:1275` and `:1506`, are peer faults at Error too, and `:1591` is one
  when the dispatch refused the notification.
* **A14** says 55 sites quote the value. 24 do; 55 is every `ValidationError` site.

## Scope

In scope:

* **The runtime:** `connection.go` (the checks in `NewConnection`, log levels and fields, docs), `errors.go` (`ValidationError`'s
  doc).
* **The generator:** `cmd/generate/internal/emit/dispatch.go` (the side constructors' checks and docs) and
  `cmd/generate/internal/model/protocol.go` (the docs of `Agent` and `Client`), with their tests.
* **The regenerated files:** `agent_gen.go`, `client_gen.go`, `types_gen.go`.
* **Docs:** the README's "Errors on the wire".
* **Tests:** new root tests below; the two tests that wait for "connection closed".
* **The records:** the MADR's annotations; this PLAN and its row; the release notes `docs/reports/0024-REPORT-release-notes-v1-17-0.md`
  and their row.

Out of scope:

* recovering a handler's panic (question 1's alternative);
* an option that maps a handler's error before it is sent (question 3's alternatives);
* a rate limit on the log lines;
* phase 4 (V2, V3) and schema `1.26.0`.

## Design

### A1: nil arguments and handler panics

**The constructors.**

* **`NewConnection`** panics before building anything:
  * `"acp: NewConnection: nil peerInput"`;
  * `"acp: NewConnection: nil peerOutput"`.
* **The side constructors.** The emitter writes the same checks at the top of `NewAgentSideConnection` and
  `NewClientSideConnection`, before the connection is built or `SetAgentConnection` / `SetClientConnection` is called:
  * `"acp: NewAgentSideConnection: nil agent"`, `"… nil peerInput"`, `"… nil peerOutput"`;
  * the same for `NewClientSideConnection` and `client`.
* **The checks are inline.** The generator type-checks its output against stubs of the hand-written code (`cmd/generate/main_test.go`).
  An inline `if x == nil { panic(…) }` needs no new stub.
* **What counts as nil.** The check is `== nil` on the interface, as `http.Handle`'s is. A nil pointer in a non-nil interface is not
  caught.
* **The docs.** Each constructor's doc names the panic. `NewConnection`'s says a nil handler answers every request `-32601` and drops
  every notification.

**Handler panics.** The docs of `Agent`, `Client` and `MethodHandler` gain: "A method that panics ends the process, as an unrecovered
panic in any goroutine does. The connection does not recover it."

**The checks.**

* **`TestConstructorsPanicOnNil`** (root): each constructor with each nil argument, its `recover()` value equal to the message, and
  no goroutine started (the leak guard).
* **`TestHandlerPanicEndsProcess`** (root): runs the test binary again as a child, in which a handler panics. It requires the child to
  exit non-zero with the handler's panic message on its stderr.
* **`TestNilHandlerAnswersMethodNotFound`**, unless an existing test already makes that claim. Step 2 names the test it maps to.
* **The emitter's pins** in `emit/dispatch_test.go` are updated.

### A2: a handler's error text

**The docs of `Agent` and `Client`** gain: "An error a request's method returns reaches the peer. A `*RequestError` is sent as it is.
Any other error is answered `-32603`, or `-32800` when it wraps `context.Canceled`, with its text in `data.error`. To keep a message
private, return a `*RequestError` of your own."

**The README's "Errors on the wire"** gains the same as a paragraph. Its sentence on a call whose context ended is corrected to say it
wraps `context.Cause(ctx)`, which is `context.Canceled` or `context.DeadlineExceeded` when no cause was given.

**The check.** Step 3 maps each claim to a test that makes it, and adds one where none does.

### A3: ids that cannot be canonicalised

The three lines log `"size", len(…)` in place of the id's bytes, as B15 does for a malformed line.

**The check.** **`TestUncanonicalIDsLogSize`** (root) sends a request and a response whose numeric id cannot be canonicalised, and a
`$/cancel_request` whose `requestId` is an object holding a marker. It requires neither the marker nor the number in the log, and
`size` on each line.

### A4: log levels

* **The eleven peer faults** in "Facts" log at Warn.
* **`:1591`** logs at Warn when the notification's error is `-32601` or `-32602`, the codes the dispatch gives a notification it
  refused, and at Error otherwise.
* **`:1476`, the connection's end,** logs at Debug.
* **Error stays** for the full queue that closes the connection (`:1339`), an invalid handler result (`:2061`), and an invalid
  initialize answer (`agent_gen.go:356`, `:373`). Each is a fault of this side, or ends the connection.

**The checks.**

* **`TestPeerFaultLogLevels`** (root) drives each of the eleven faults, and a refused notification, through a JSON log handler at
  Debug. It requires each line at WARN.
* **Its local cases** require ERROR for a notification handler's own error and an invalid handler result.
* **Its end case** requires the connection's end at DEBUG.
* **The two tests** that wait for "connection closed" pass their handler `Level: slog.LevelDebug`.

### A14: validation messages

`ValidationError`'s doc gains: "The message of an absolute-path or excluded-value check quotes the value it refuses, so a value the
peer sent reaches the answer's `data` and the logs as it was." The README's paragraph on malformed messages says the same.

**The check.** **`TestValidationMessagesQuoteValue`** (root) requires one absolute-path and one excluded-value message to hold their
value, `%q`-quoted, and one one-variant message not to.

## Implementation Steps

Each step ends with its gates and an execution record. The owner commits it; agents do not commit on `main`. Every new test is seen to
fail on a planted break in a scratch copy, never in the tree, before it is relied on.

1. **Records:**
   * this PLAN and its row `in-progress`;
   * the MADR's three annotations, under A3, A4 and A14;
   * the facts above re-asserted by a script.
2. **A1:**
   * the checks in `NewConnection` and in the emitter;
   * the constructor and handler docs, in the emitter, the protocol table and `connection.go`;
   * regeneration and the emitter's pins;
   * `TestConstructorsPanicOnNil` and `TestHandlerPanicEndsProcess`;
   * the nil handler's test.
3. **A2 and A14:**
   * the docs of `Agent` and `Client` in the protocol table, and regeneration;
   * `ValidationError`'s doc;
   * the README;
   * `TestValidationMessagesQuoteValue`;
   * A2's claims mapped to tests.
4. **A3 and A4:**
   * the log lines in `connection.go`;
   * `TestUncanonicalIDsLogSize` and `TestPeerFaultLogLevels`;
   * the two tests' handler levels.
5. **Release notes.** `docs/reports/0024-REPORT-release-notes-v1-17-0.md` and its row.
6. **Verification and close.**
   * The gates over the tree.
   * `make push-guard`.
   * After CI's six jobs pass on the pushed commits, this PLAN and its row become `complete`.
   * The owner tags `v1.17.0` and publishes its release.

## Verification

Per code step:

* phase 1's gates on go1.27.2, with the API listing against `v1.16.3`;
* the go1.26.9 gate;
* Windows over SSH, before the push.

Planted breaks, at least. Each must be seen to fail on the check named.

| For | Break | The check that must fail |
| :--- | :--- | :--- |
| A1 | one nil check removed from the emitter, regenerated | `TestConstructorsPanicOnNil` |
| A1 | `NewConnection`'s reader check removed | `TestConstructorsPanicOnNil` |
| A1 | a `recover` around the handler call | `TestHandlerPanicEndsProcess` |
| A3 | one line's `string(…)` restored | `TestUncanonicalIDsLogSize` |
| A4 | one peer fault back at Error | `TestPeerFaultLogLevels` |
| A4 | `:1591` at Warn for every code | `TestPeerFaultLogLevels`, its local case |
| A4 | the connection's end back at Info | `TestPeerFaultLogLevels`, its end case |
| A14 | the absolute-path message without its `%q` | `TestValidationMessagesQuoteValue` |

Expected API listing difference against `v1.16.3`: none.

Acceptance:

* every step's checks pass and its record is written;
* every claim the new docs make is mapped to a test that makes it;
* `make lint` reports 0 issues and `go fix -diff` prints nothing;
* CI's six jobs pass on the pushed commits;
* the API listing against `v1.16.3` shows no difference.

## Rollout and Rollback

Each step is one commit, reverted by reverting it. Step 2 changes the generator and regenerates, so reverting it reverts the generated
checks with it.

The behaviour changes are for a consumer to read in the release notes:

* **A nil argument** panicked before too, but later, on a connection goroutine, with a nil pointer dereference. It now panics in the
  constructor, naming the argument.
* **A log handler filtering at Error** stops seeing the eleven peer faults, now at Warn.
* **A handler at Info** stops seeing the connection's end, now at Debug.

The release is proposed as the minor `v1.17.0`, which the owner confirms at the tag.

**Points for the owner's review**, each with the plan's choice:

* **`:1591` split by code** (above), rather than all at Warn or all at Error. Its line mixes the peer's faults with the application's.
* **The full queue (`:1339`) stays at Error.** The default policy closes the connection there, and the cause can be this side's slow
  handler as much as the peer's rate.
* **`v1.17.0`, a minor,** rather than a patch. The API is unchanged, but three behaviours change, and A4's levels change what a
  consumer's log handler sees.
* **The README's `context.Canceled` sentence** is corrected here, with A2's paragraph in the same section.

## Execution record

### Step 1, records, 2026-10-19

The owner committed this PLAN as `12dd249` and approved it. This step:

* marks this PLAN and its `docs/README.md` row `in-progress`;
* annotates the MADR, after its table of findings A1 to A14, with the corrections to A3, A4 and A14 in "Facts this plan rests on".

A script outside the tree re-asserted "Facts this plan rests on" on `12dd249`: 37 facts, each snippet at its line, plus the counts
quoted. The code is the same as at `d6cab27`. All hold, after one correction:

* **A1:** the side constructors' emission ends at `cmd/generate/internal/emit/dispatch.go:209`, not `:213`. The row is annotated.

The A1 probe ran again, in a scratch copy of `12dd249`, each case in its own test process. Each case still ends the process, exit 1,
with a nil pointer dereference:

* a nil reader at `connection.go:1049`;
* a nil writer at `connection.go:687`;
* a nil agent answering `initialize` at `agent_gen.go:344`;
* a nil client given a `session/update` after initialize at `client_gen.go:230`.

Gates: `make check-records`, `make markdownlint` and the identifier scan, each exit 0.
