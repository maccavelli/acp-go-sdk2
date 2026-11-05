---
status: accepted
date: 2026-11-05
decision-makers: the repository owner
consulted: "probe tests in `git archive` copies of `b4ee2d4`; `schema/schema.json` and `schema/schema.unstable.json` at ACP 1.25.0; the official TypeScript (`08a0d30`) and Python (`9853d8b`) SDKs' clones in the interop cache; shallow clones of gobble-cli (`627035c`) and magic-cli-remote (`89d18c4`), read as data"
informed: gobble-cli, magic-cli-remote
---

# Give clients the ids authenticate accepts, name them in its refusal, count an unknown entry only by a string id, and document the prompt result's stop reason

## Context and Problem Statement

Issue #14, "v1.32.0 follow-up: PromptResponse zero value and unadvertised authenticate", opened 2026-11-04, asks for the SDK
work that two refusals of `v1.32.0` left to consumers. Both refusals come from
[0049-MADR-remediate-the-2026-11-01-debugging-pass.md](0049-MADR-remediate-the-2026-11-01-debugging-pass.md):

* **S6,** in `cedb98d`: a closed enumeration is checked when sent, so an agent whose `Prompt` returns `PromptResponse{}`
  answers `-32603` instead of sending `"stopReason": ""`.
* **S8,** in `6ad8759`, with Q10's sentinel: an `authenticate` whose `methodId` names no method of the agent's `initialize`
  answer is refused before it is sent, with `ErrUnadvertisedAuthMethod`, and answered `-32602` when received.

The issue keeps both refusals. It asks for:

* constructors and godoc for the prompt result;
* fixed stub agents and examples;
* a helper listing the advertised ids, and one for the ids that are not terminal;
* an `ErrUnadvertisedAuthMethod` error that names the advertised ids;
* a migration note.

This record decides which of those the SDK provides, and in what form. It also decides how to fix a defect in S8's gate
that the investigation found, F5 below. The issue's claims were checked against the tree at `b4ee2d4`, which is `v1.33.1`;
some hold and some do not.

### Findings

Each finding says how it was established:

* **Read:** the code or text was read at the cited lines.
* **Probe:** a test ran in a `git archive` copy of `b4ee2d4`, never in the tree, and its output is quoted.

The probe test is `i14/probe_issue14_test.go.txt` in the session's scratch directory. The stub plants are run by `probe.py`
and `stubwho.py` beside it. All three are scratch files, not committed.

#### The prompt result (S6)

**F1. The refusal is as the issue says.**

* **Read:**
  * `StopReason` is a closed set of five constants (`types_gen.go:18058-18091`).
  * The schema's `PromptResponse` requires `stopReason` and gives it no default.
  * `checkResult` (`connection.go:3122-3141`) validates a handler's result before it is sent. A failure is logged at
    `Error` as "handler result fails validation; not sent", and answered `-32603` with the error in `data.error`.
  * On a cancelled turn, the dispatch replaces the result's stop reason with `StopReasonCancelled` and clears the
    handler's error (`agent_gen.go:777-785`).
  * `TestClosedEnumsRefusedOnSend` (`enum_closed_test.go:43-77`) plants `PromptResponse{}` on purpose.
  * `acptest.ScriptAgent` gives an empty turn stop `StopReasonEndTurn` (`acptest/script.go:17`, `:140`).
* **Probe P1:** an agent returning `PromptResponse{}, nil` gives the client `code=-32603 message="Internal error"
  data=map[error:PromptResponse: stopReason: "" is not a StopReason]`. The data names the type, the property and the
  value.

**F2. Most of what the issue lists for the prompt is already done, or is not as it says.**

* **Read, the README and the notes:** `README.md:343-348` already says that `PromptResponse{}`'s empty `stopReason` fails
  and that such an agent answers `-32603`. The `v1.32.0` release notes, lines 56-57, say "An agent whose `Prompt` returns
  `PromptResponse{}` now answers `-32603`. Set `StopReason`, for example `acp.StopReasonEndTurn`". The issue's item 5, the
  migration note, therefore exists.
* **Read, the examples:** every `PromptResponse{}` in `example/` and the root's example tests returns an error beside it
  (`example/agent/main.go:126`, `:138`; `example/remote/main.go:190`, `:201`; `example_agent_test.go:53-103`). Every
  successful return in them sets `StopReason`.
* **Read, the two test files the issue names:**
  * `refusals_test.go:132` returns `PromptResponse{StopReason: StopReasonEndTurn}`.
  * `connection_overflow_block_test.go:453` returns `PromptResponse{}` with `ctx.Err()`, which is an error path.
* **Read, the stubs:** the root's only success-path zero results outside `TestClosedEnumsRefusedOnSend` are three stubs in
  `acp_test.go`:
  * `agentFuncs.Prompt`'s default when `PromptFunc` is nil (`:200-205`);
  * `forkOnlyUnstableAgent.Prompt` (`:409-411`);
  * `agentNoExtensions.Prompt` (`:1340-1342`).
* **Probe, which tests reach the stubs:** each stub printed a marker, with the suite run serially (`-parallel 1 -p 1`).
  Only `TestConcurrentUse` reached one: "agentFuncs reached under TestConcurrentUse: 200", and no other marker. That test
  calls every typed method with zero params, and its comment says "what each returns is not this test's to check"
  (`concurrent_use_test.go:88-91`).
* **Probe, the end-turn plant:** with all three stubs returning `StopReasonEndTurn`, `go test -count=1 .` passes, "ok …
  10.831s".
* **Probe, the control:** a plant that exits the process with status 97 from the stubs killed the suite. A control test
  that calls the default stub alone died with "exit status 97", so the plant does show a stub being reached.

**F3. The godoc of `Prompt` and `PromptResponse` does not say it, and only `PromptResponse`'s can say it without a new
generator mechanism.**

* **Read, the godoc today:** `PromptResponse`'s godoc (`types_gen.go:12087-12101`) and `Agent.Prompt`'s
  (`types_gen.go:24634-24639`) are the schema's descriptions alone.
* **Read, the route that exists:** the protocol table's `DocNotes` add lines to the godoc of a generated type
  (`cmd/generate/internal/model/protocol.go:79-86`, `:944-970`). Its notes today cover the envelope types, `Error`,
  `ErrorCode` and `CancelRequestNotification` (`cmd/generate/internal/model/acp.go:155-169`). `checkDocNotes` refuses a note
  without lines, and a second note on one definition (`protocol.go:455-469`).
* **Read, the route that does not:** an interface method's godoc is its request definition's description
  (`cmd/generate/internal/model/methods.go:214-218`), so no doc note reaches it.

#### The authenticate method id (S8)

**F4. The gate, its order and its text are as the issue says, and the error lists nothing.**

* **Read, the generated gate:** `(*AuthenticateRequest).gates` (`types_gen.go:25413-25426`):
  * It first refuses an id of a `Terminal` entry with `ErrTerminalAuthMethod`.
  * It then accepts the id of any `Terminal` or `Agent` entry, or the `"id"` an `Unknown` entry holds, read by
    `unknownKind`.
  * It refuses any other id with `ErrUnadvertisedAuthMethod`.
* **Read, `Authenticate`'s godoc:** it names `ErrNotInitialized`, then `*CapabilityError`, `ErrTerminalAuthMethod`,
  `ErrUnadvertisedAuthMethod` and `*ValidationError`, in that order, all returned before anything is sent
  (`client_gen.go:355-372`).
* **Read, the test of the text:** `TestAuthenticateUnadvertisedMethod` requires the text exactly (`gates_surface_test.go:146`).
* **Probe P2,** against an agent that advertised `openai` and `grok` as agent methods, and `grok-login` as terminal:
  * Sent: `Authenticate(nope)` gives `err="authenticate: methodId \"nope\": acp: methodId names no auth method the agent
    advertised" is=true requestError=false wrote=0`. The error is not a `*RequestError`, and nothing is written.
  * Received: `code=-32602 data=map[error:authenticate: methodId "nope": acp: methodId names no auth method the agent
    advertised] handler ran=false`.
  * `Authenticate(grok-login)` gives `ErrTerminalAuthMethod`, and nothing is written.
* **Probe P3:** an id advertised both as a terminal and as an agent method is refused as terminal. That holds on send and
  in `AllowsValue`: "acp: a terminal auth method is not authenticated".

**F5. A defect: an `Unknown` entry without a string `"id"` advertises the id `"unknown"`.**

* **Read:** `unknownKind` returns `"unknown"` when the key is absent, empty or not a string (`types_gen.go:25003-25017`).
  That is right for a discriminator, which is what `Kind()` reads. But the gate uses the same function for an id
  (`types_gen.go:25419-25421`, emitted by `cmd/generate/internal/emit/gates.go:154-181`).
* **Read, how such an entry arises:** a received entry whose `"type"` this SDK does not know decodes into `Unknown`
  whatever else it holds (`types_gen.go:846-849`).
* **Probe P4,** for each of `{"type":"future","name":"F"}`, `{"type":"future","id":"","name":"F"}` and
  `{"type":"future","id":7,"name":"F"}`:
  * `AllowsValue(unknown)=<nil>`;
  * `Authenticate(unknown)` is written, `wrote=81`;
  * received, `methodId: "unknown"` is dispatched, `handler ran=true`.
* **Probe P4's control:** without the entry, `AllowsValue(unknown)` is refused with `ErrUnadvertisedAuthMethod`.
* **Consequence:** an `authenticate` that the schema forbids, since it is "Must be one of the methods advertised", is sent
  and dispatched. Reaching it takes an agent that lists an entry of a type unknown to this SDK with no string id, and a
  client that names `"unknown"`. **Low.** It was introduced with the gate in `6ad8759`.

**F6. The client has the agent's methods but no id accessor and no list of the ids it may send.**

* **Read:**
  * `ClientSideConnection.Negotiated()` returns a copy of the record, the agent's `AuthMethods` included
    (`client_gen.go:128-145`).
  * `AuthMethod` has `Kind()` (`helpers.go:418-433`) and no id accessor.
  * `OfferedAuthMethods` (`helpers.go:740-756`) is for the agent: it drops terminal methods from an answer for a client
    without `auth.terminal`.
* **Probe P5:**
  * "AuthMethod.ID exists: false", and the same for `Id` and `MethodID`;
  * "Negotiation.AuthMethodIDs exists: false", and the same for `AuthenticateMethodIDs`;
  * `csc.Negotiated()` gives `methods=[agent:openai agent:grok terminal:grok-login]`.
* **Consequence:** a client that wants the ids repeats the three-way variant switch and the `Unknown` read, and has to
  match the gate's rules itself, the terminal-first order included.

**F7. The documentation of `AllowsValue` and of the package does not name S8's sentinel.**

* **Read:**
  * `AllowsValue`'s godoc names `ErrTerminalAuthMethod` and not `ErrUnadvertisedAuthMethod` (`helpers.go:1020-1031`), yet
    returns it (P2, and `gates_surface_test.go:155`).
  * The package doc's list of sentinels names `ErrTerminalAuthMethod` and not its sibling (`doc.go:36-38`).
  * `README.md:614-617` says an `Unknown` entry "counts by its `"id"`", which F5 shows is not exact.

**F8. Generated code cannot call a hand-written helper.**

* **Read:** `TestRuntimeFilesCoverHandWritten` (`cmd/generate/engine_test.go:202-223`) keeps `helpers.go` out of the runtime
  files that each synthetic engine case compiles with, because it uses ACP types.
* **Consequence:** a gate that is to share one list with an exported helper must read it from generated code. The helper
  in `helpers.go` then calls that code, as `Kind()` already calls the generated `unknownKind`.
* **Read, what tests the emitter's auth gates today:** they are tested only through the root's tests and the `regen` gate.
  The one engine case that names `AuthenticateRequest`, `refuse-auth-method-id-not-named.txtar`, holds no auth method list.

#### Outside this SDK

**F9. The official SDKs neither refuse nor list.**

* **Read,** at the revisions above, outside their generated schema code:
  * the TypeScript SDK's code names no `authMethods`. Its `authenticate` request spec parses the params' shape only
    (`src/v1/acp.ts:1260-1267`);
  * the Python SDK's code names no `auth_method`. Its client `authenticate` sends `method_id` as given
    (`src/acp/client/connection.py:281-287`);
  * the TypeScript example answers `{ stopReason: "end_turn" }` (`src/examples/http-server.ts:94`).
* **Consequence:** this SDK is alone in refusing an unadvertised id, which makes a list of what it accepts more useful
  here than there.

**F10. What the consumers would meet.** Read as data.

* **magic-cli-remote, at `89d18c4`,** requires `v1.33.0` (`go.mod:11`).
  * It builds its own list of advertised ids from `Agent` entries alone (`internal/provider/acpagent/acpagent.go:1480-1484`).
  * Without a safe-list, it authenticates the configured pin without checking that list (`acpagent.go:736-742`). From
    `v1.32.0` on, an unadvertised pin fails with `ErrUnadvertisedAuthMethod` (P2), and a terminal one with
    `ErrTerminalAuthMethod`.
  * `selectACPAuthMethod` returns the spec's key method "advertised or not" when a key exists
    (`internal/provider/acpagent/authselect.go:33-35`), which the SDK refuses when the agent did not list it.
  * Its error for a missing method prints the advertised list itself (`authselect.go:43-47`).
* **gobble-cli, at `627035c`,** requires `v1.31.2` (`go.mod:16`), before S6 and S8.
  * **Its stop reason cannot be empty.** Its agent's turn ends only through `turn.end` with one of five constants
    (`agent/agent.go:293-294`, called at `:307`-`:577`), and its `Prompt` takes the stop reason from that `End` event
    (`acpserver/agent.go:754-770`). The issue's "an empty internal stop becomes `""`" cannot happen there today.
  * **Two of its own tests will fail on an upgrade.** `acpserver/agent_test.go:197` and `acpserver/auth_test.go:91` expect
    an unknown id to come back as `-32602` in a `*RequestError`. From `v1.32.0` on, the client returns
    `ErrUnadvertisedAuthMethod` before sending, with no `*RequestError` (P2). Those tests change when gobble-cli upgrades.
* **Out of scope here:** each consumer's own change. This record gives them the list the SDK accepts, and an error that
  names it.

## Decision Drivers

* **Keep S6 and S8.** The schema requires `stopReason` from a closed set, and says `methodId` "Must be one of the methods
  advertised". Neither refusal is loosened.
* **One rule, one implementation.** What a helper says `authenticate` accepts must be exactly what the gate accepts, read by
  the same code. F6 shows the cost of a second copy, and F10's consumer copy already differs from the gate.
* **Fix the defect the investigation found.** F5 lets through an id the schema forbids.
* **Tell the caller what to send.** A refusal that names the accepted ids turns a pin's typo or a stale configuration into
  a one-line fix.
* **Spend surface only where it removes code.** A constructor that is no shorter than the literal, and keeps the zero value
  just as easy to write, does not earn a `v1` promise.
* **Additive under `v1`.** No exported signature changes, and nothing that is sent today changes, except what F5 wrongly
  sends.
* **Generated code stays self-contained** (F8). The generator changes only where the gate and its text are emitted, and
  where a doc note is added.

## Considered Options

* **O1.** The authenticate work, and a doc note for the prompt result:
  * fix F5;
  * one generated list of the ids `authenticate` accepts, which the gate, its refusal's text and two new exported helpers
    all use: `AuthMethod.ID()` and `AuthenticateMethodIDs`;
  * a doc note on `PromptResponse`;
  * F7's documentation;
  * the three stubs set to `StopReasonEndTurn`;
  * a minor release, `v1.34.0`.
* **O2.** O1, and the issue's prompt constructors: `NewPromptResponse(stop)`, `PromptEndTurn()` and `PromptCancelled()`.
* **O3.** O1, and a typed error, `*UnadvertisedAuthMethodError{MethodID, Accepted}`, wrapping the sentinel, in place of the
  list in the text.
* **O4.** Documentation and the F5 fix only, with no new helper: a patch release.
* **O5.** Nothing. Answer the issue with the README and release-note passages that already exist (F2).

## Decision Outcome

Chosen option: **"O1"**, accepted 2026-11-05 with Q1 to Q4 answered (a), because it closes the gap F6 names with one list that cannot drift from the gate, fixes
F5 in the same emitted code, and makes the refusal name the way out. It spends two exported names, against O2's three for
no shorter code and O3's new error type. The prompt side gets the one doc change that the existing generator route allows
(F3). The rest of the issue's prompt list is already done or not as stated (F2).

* **Against O2:** `acp.NewPromptResponse(acp.StopReasonEndTurn)` is longer than `acp.PromptResponse{StopReason:
  acp.StopReasonEndTurn}`. The zero value stays one keystroke away, so the constructors prevent nothing. They are three
  `v1` names.
* **Against O3:** a caller that wants the ids as data has `AuthenticateMethodIDs` (D3). A typed error would duplicate it,
  in a type every later gate error would be asked to match. Q10 of the 0049 MADR already chose a sentinel for this
  refusal.
* **Against O4:** the client keeps the three-way switch, the `Unknown` read and the terminal-first rule, which magic-cli-remote
  copies today with a different rule (F10).
* **Against O5:** F5 stays, and so do F6 and F7.

### Decisions

**D1. Count an `Unknown` entry only by a string `"id"`** (F5).

* The gate reads an `Unknown` entry's `"id"` with the key folding its decoder uses, which today's test pins with `"ID"`
  (`gates_surface_test.go:132`).
* A JSON string counts, the empty string included, as an `Agent` entry's empty `Id` would. An absent `"id"`, or one that
  is not a string, advertises nothing.
* A new generated helper reads the value with that folding and reports whether it is a string. `unknownKind` keeps its
  `"unknown"` fallback for discriminators, and reads through the new helper.

**D2. One generated list of what `authenticate` accepts.** The generator emits, beside the gate, one unexported function
from the initialize answer's list to the ids the gate accepts:

* in the list's order, each id once;
* the `Agent` ids;
* the `Unknown` entries' string ids, per D1;
* not any id that a `Terminal` entry names, even one an `Agent` entry also names (P3).

The gate keeps its terminal check first, so a terminal id still meets `ErrTerminalAuthMethod`. It then refuses an id the
list does not hold. Which variants the list leaves out comes from the protocol table's terminal rule on the same property
(`cmd/generate/internal/model/acp.go:207`). It is not written into the emitter, so the emitter stays as generic as S8 left
it.

**D3. Two exported helpers in `helpers.go`.**

* **`func (a AuthMethod) ID() AuthMethodId`:** the `Terminal` or `Agent` entry's `Id`, or the `Unknown` entry's string
  `"id"` read as D1 reads it. It returns `""` for an `Unknown` entry without one, and for the zero value. It sits beside
  `Kind()`, for display and matching (Q3).
* **`func AuthenticateMethodIDs(methods []AuthMethod) []AuthMethodId`:** D2's list, from an `initialize` answer's
  `AuthMethods`, as `Initialize` returns it or as `Negotiated()` records it. It returns nil for none.
  * Its godoc says that an id it returns is one `Authenticate` sends, and that every other id is refused before sending.
  * It says too that a client wanting `agent` methods alone filters by `Kind()` (Q1, Q2).

**D4. The refusal names the accepted ids** (Q4). The text keeps its prefix and the sentinel, so `errors.Is` holds. It
adds the list from D2:

* `authenticate: methodId "nope": acp: methodId names no auth method the agent advertised; authenticate accepts "openai",
  "grok"`;
* `…; authenticate accepts none`, when the list is empty.

The same text reaches the peer in `-32602`'s `data.error` when received. It lists the agent's own advertised ids, which
the client already received in `initialize`.

**D5. A doc note on `PromptResponse`** (F3). The protocol table's `DocNotes` gain one entry on `PromptResponse`:
`StopReason` is required, its zero value `""` is not a `StopReason`, and an agent whose `Prompt` returns it answers `-32603`
instead of sending it, so a successful turn sets one of the constants, such as `StopReasonEndTurn`. `Agent.Prompt`'s godoc
stays the schema's (F3). The README already says it (F2).

**D6. Documentation** (F7):

* `AllowsValue`'s godoc names `ErrUnadvertisedAuthMethod`.
* The package doc's sentinel list names it.
* `README.md`'s authenticate passage gains D1's rule, D4's text and the two helpers.
* The README's client section points to `AuthenticateMethodIDs`.
* `docs/architecture.md`'s list of what `helpers.go` holds names the helpers.
* A runnable `ExampleAuthenticateMethodIDs` picks a pin when it is accepted, and falls back to the first accepted id
  otherwise.

**D7. The stubs** (F2): the three `acp_test.go` stubs return `StopReasonEndTurn`, so the root has no success-path
`PromptResponse{}` but the one `TestClosedEnumsRefusedOnSend` plants. No production code changes for S6.

**D8. Not done:**

* the prompt constructors (O2);
* `AuthenticateAdvertised` on `ClientSideConnection`, which the issue itself calls "probably unnecessary";
* a hint in the `-32603` text, which is `Validate`'s generic message and already names the type, the property and the
  value (P1);
* a doc note on `Agent.Prompt`, which would need a new generator mechanism for one sentence (F3);
* any change to S6 or S8;
* the consumers' own changes (F10).

**D9. Release and the issue.**

* **The release** is a minor, `v1.34.0`, because D3 adds two exported names. `make api` against `v1.33.1` is to report
  "0 incompatible, 2 compatible".
* **The notes** lead with D4 and D3. They state F5's fix as a behaviour change: `methodId: "unknown"` against an entry
  without a string id is now refused.
* **The issue reply** goes on issue #14 only on the owner's ask. It corrects the issue's table (F2), states what shipped,
  and names each consumer's own change (F10). The reply's text is drafted in the PLAN.

### Questions for the owner

1. **Where the list lives.**
   * (a) A function over `[]AuthMethod`, `AuthenticateMethodIDs(methods)` (recommended). It takes `Initialize`'s result
     directly, and composes with `OfferedAuthMethods` on the agent's side.
   * (b) A method, `Negotiation.AuthenticateMethodIDs()`, read from `Negotiated()`. One call fewer for a client that
     already holds the record, but not usable on an answer before it is recorded.
   * (c) Both: two names for one list.
2. **`Unknown` entries in the list.**
   * (a) Included, by string id, as the gate accepts them (recommended). The list is then exactly what `Authenticate` will
     send, and `Kind()` filters them.
   * (b) Excluded, so the list is `Agent` ids alone, as magic-cli-remote builds it. The list then no longer matches the
     gate, and an id it omits can still be sent.
   * (c) Excluded, and the gate refuses them too. List and gate agree, but an entry of a type a later schema adds could not
     be authenticated until this SDK knew its type: a behaviour change.
3. **`AuthMethod.ID()`'s shape.**
   * (a) `ID() AuthMethodId`, `""` for none (recommended), as `Kind()` returns one value.
   * (b) `ID() (AuthMethodId, bool)`. Exact for an `Unknown` entry whose id is the empty string, at the cost of the
     accessor's ease.
   * (c) No accessor. `AuthenticateMethodIDs` alone; a display still needs the switch.
4. **The refusal's form.**
   * (a) The accepted ids in the text, sentinel kept (recommended, D4).
   * (b) O3's typed error wrapping the sentinel. Programs could read the list from the error, at the cost of a new type.
   * (c) The text unchanged. `AuthenticateMethodIDs` alone tells the caller.

#### Decided with the owner, 2026-11-05

The owner chose, out of three options for each:

| Question | Answer |
| :--- | :--- |
| 1. Where the list lives | (a), as recommended: `AuthenticateMethodIDs(methods []AuthMethod)`. Not chosen: a `Negotiation` method; both. |
| 2. `Unknown` entries in the list | (a), as recommended: included by string id, as the gate accepts them. Not chosen: excluded with the gate as is; excluded with the gate refusing them too. |
| 3. `AuthMethod.ID()`'s shape | (a), as recommended: `ID() AuthMethodId`, `""` for none. Not chosen: `(AuthMethodId, bool)`; no accessor. |
| 4. The refusal's form | (a), as recommended: the accepted ids in the text, the sentinel kept. Not chosen: a typed error; the text unchanged. |

O1 is accepted as specified in D1 to D9.

### Consequences

* Good, because a client can name an id that the SDK will send without repeating the gate's rules, and the list cannot
  drift from them: both are one generated function.
* Good, because a stale pin or a typo is answered with what to send instead, on both sides of the connection.
* Good, because F5's forbidden `authenticate` is no longer sent or dispatched.
* Good, because an agent author reading `PromptResponse`'s godoc learns the rule that `README.md:343-348` and the `v1.32.0`
  notes already state.
* Neutral, because the prompt's constructors, which the issue asked for, are not added, and the issue reply says why.
* Neutral, because the refusal's text grows by the agent's accepted ids. The text is not a contract, but
  `TestAuthenticateUnadvertisedMethod` and gobble-cli's tests compare error strings, so the notes say it.
* Bad, because the root package gains two `v1` names, and the generator gains a helper and a list function to keep.
* Bad, because a refused `authenticate` allocates the list once more. That is one `initialize` answer's worth of ids, on a
  call that happens about once a connection.
* Bad, because a later schema field or variant named `ID` on `AuthMethod` would collide with D3's method. No field or
  variant has that name at ACP 1.25.0.

### Confirmation

The PLAN shows each of the following, every new check first seen to fail on a planted break in a scratch copy:

* **D1:** P4's three entries refused on send, on receive and in `AllowsValue`; a string `"id"`, `"ID"` included, accepted.
* **D2:** for each id `AuthenticateMethodIDs` returns, `AllowsValue` is nil. For every other advertised id and a stranger,
  it is refused. The terminal-and-agent id is refused as terminal and is not listed.
* **D2 in the generator:** an engine case whose behaviour test checks the emitted list and gate on a synthetic schema, the
  terminal variant left out.
* **D3:** `ID()` for each variant, an `Unknown` entry with and without a string id, and the zero value.
* **D4:** the exact texts, with ids and with none, on send and in the `-32602` received; `errors.Is` still true.
* **D5:** a docs test that `PromptResponse`'s godoc names `-32603` and `StopReasonEndTurn`.
* **D6:** `ExampleAuthenticateMethodIDs` with its `// Output:`; a docs test that the package doc names both auth
  sentinels.
* **D7:** the suite passes with the stubs set.
* **D9:** `make api` against `v1.33.1`, "0 incompatible, 2 compatible"; `tools/gates.py --all`; CI on the head and on the
  tag.

## Pros and Cons of the Options

### O1

* Good, because the gate, its text and the helpers read one list.
* Good, because F5 is fixed where it is emitted.
* Good, because the doc note uses an existing generator route.
* Neutral, because the issue's prompt constructors are declined, with reasons.
* Bad, because the emitter and the model gain a field, a helper and a function, with an engine case to keep them.

### O2

* Good, because it gives the issue every item it asked for.
* Bad, because the constructors are no shorter than the literal and leave the zero value as easy (F2).
* Bad, because they are three more `v1` names.

### O3

* Good, because a program could read the accepted ids from the error itself.
* Bad, because it duplicates `AuthenticateMethodIDs`, and adds a type that Q10 of the 0049 MADR decided against for this
  refusal.

### O4

* Good, because it is a patch, with no new surface.
* Bad, because every client keeps its own copy of the gate's rules, and F10's copy already differs.

### O5

* Good, because it costs nothing.
* Bad, because F5, F6 and F7 stay.

## More Information

* Issue #14, opened 2026-11-04.
* [0049-MADR-remediate-the-2026-11-01-debugging-pass.md](0049-MADR-remediate-the-2026-11-01-debugging-pass.md): S6, S8, and
  Q10's sentinel.
* [0049-PLAN-surface-fixes.md](0049-PLAN-surface-fixes.md), phase 4: the S8 gate this record amends in its emitter.
* [0049-REPORT-release-notes-v1-32-0.md](../reports/0049-REPORT-release-notes-v1-32-0.md): the notes that already say
  how to set `StopReason`.
* [0038-MADR-consumer-helpers-kind-and-agent-refusals.md](0038-MADR-consumer-helpers-kind-and-agent-refusals.md):
  `OfferedAuthMethods`, and the rules for hand-written helpers that this record follows.
* [0041-MADR-close-the-gaps-the-consumers-wait-on.md](0041-MADR-close-the-gaps-the-consumers-wait-on.md), item 10:
  `AuthMethod.Kind()`, beside which `ID()` goes.
* The implementation is [0050-PLAN-auth-method-ids-and-the-prompt-result-for-issue-14.md](0050-PLAN-auth-method-ids-and-the-prompt-result-for-issue-14.md).
