# Release notes for v0.18.0, against v0.17.1

Observed 2026-10-14 on `d4e1ec4`, for
[0019-PLAN-api-decisions.md](../decisions/0019-PLAN-api-decisions.md), step 10. This report drafts the text of the GitHub release for
`v0.18.0`, as `RELEASING.md` describes it. It decides nothing. Each statement names its source: a record, a commit, or the exported
API listing of the 0013 report's listing program, run on `git archive v0.17.1` (2411 lines) and on the tree at `d4e1ec4` (2413
lines). Bring it up to date if more changes land before the tag. The version, `v0.18.0`, is decision 12 of
[0019-MADR-remediate-debugging-pass-findings.md](../decisions/0019-MADR-remediate-debugging-pass-findings.md): phases 1 and 2 of that
record ship together; the owner confirms it at the tag.

The section "Release text" is what goes into the GitHub release.

## Release text

### v0.18.0

The ACP schema is still `1.25.0` (`git diff v0.17.1 -- schema/` is empty). This release fixes what a debugging pass of `v0.17.1`
found: a deadlock, races, gaps against ACP and JSON-RPC 2.0, and a connection that could not be closed. It adds `Close`, a bound on
concurrent requests, a race-free way for an agent or a client to keep its connection, and errors that keep their Go cause. Some
changes are breaking for an application that relied on the old behaviour; each says what to do. The exported API grows by nine
entries and loses seven methods no caller needs. (0019-MADR.)

#### Breaking changes

- **Extension messages received before `initialize` are refused.** A received `_`-prefixed message now goes through what a typed
  one does: both inbound hooks, then the record of `initialize`. Before `initialize`, an extension request is answered `-32601` and
  an extension notification is dropped. A peer that sends extension requests first must send `initialize` first. `WithInboundRewrite`
  and `WithInboundValidation` now see extension messages, with `params` as a `*json.RawMessage` the rewrite hook may replace; a hook
  that assumed it saw only typed messages must let these through. Outbound `CallExtension` and `NotifyExtension` are unchanged and
  unchecked. (`943f073`; 0019-MADR, decision 7.)
- **An elicitation in a mode the schema does not list is refused.** No client can advertise such a mode, so `CreateElicitation` with
  the `Other` variant returns a `*CapabilityError` naming `elicitation.<mode>` and writes nothing, and a received one is answered
  `-32602`. An application that uses a custom mode by agreement accepts it in a `WithInboundValidation` hook. (`c28159c`; 0019-MADR,
  decision 6 as amended.)
- **`session/close` cancels the session's work.** Before the agent's `CloseSession` handler runs, the session's running turn is
  cancelled with `ErrPromptCancelled` and answered `cancelled`, as ACP requires. When an agent answers a client's `CloseSession` with
  success, the session's pending `session/request_permission` requests are answered `cancelled`, as on `Cancel`. (`13dc276`;
  0019-MADR, B4.)
- **`session/fork`'s MCP servers are checked.** An `http`, `sse` or `acp` server on `session/fork` now needs `mcpCapabilities.*`, as on
  `session/new`; sent without it, `UnstableForkSession` returns a `*CapabilityError`, and received, it is answered `-32602`.
  (`03fcfa6`; 0019-MADR, B6.)
- **A method sent as the wrong kind is refused.** A request-only method sent without an id runs no handler and is logged; a
  notification-only method sent with an id is answered `-32601`. Both used to run their handler. (`23396f4`; 0019-MADR, B12.)
- **Nothing is written after the connection's end.** A send after `Done()` fails at once, `-32603`, wrapping
  `ErrPeerDisconnected`, or `ErrConnectionClosed` after `Close`, and writes nothing; it used to succeed. A call waiting for an answer
  when the connection ends returns the same way; the error's message now reads "connection ended before response", where it read
  "peer disconnected before response". Match the error with `errors.Is`, not its text. A turn whose connection ended is not
  answered. (`7e7acfd`; 0019-MADR, B17 and B21.)
- **The default overflow policy closes the transport.** `OverflowCloseConnection` now closes the connection as `Close` does: the
  reader and the writer are closed when they implement `io.Closer`. (`7e7acfd`; 0019-MADR, B17.)
- **The constructors call `SetAgentConnection` and `SetClientConnection`.** An agent with a method `SetAgentConnection(*AgentSideConnection)`,
  or a client with `SetClientConnection(*ClientSideConnection)`, now has it called by the constructor, before reading starts. It must
  only store the connection: a send from it blocks the constructor. Drop the call your code makes after the constructor; it is no
  longer needed, and it raced a handler already running. (`3f0a822`; 0019-MADR, decision 1.)
- **Params are never sent as `null`, nor as a scalar.** Params that encode as `null`, such as a nil map, are left out; params that
  encode as neither an object nor an array are refused `-32602` before anything is written. Reachable through `SendRequest`,
  `SendNotification` and the extension methods. (`c342496`; 0019-MADR, B10.)
- **Seven `MarshalJSON` methods are removed.** `AgentCapabilities`, `AuthCapabilities`, `ClientCapabilities`,
  `FileSystemCapabilities`, `InitializeRequest`, `McpCapabilities` and `PromptCapabilities` lose a `MarshalJSON` that only encoded
  their fields. `json.Marshal` of each gives the same bytes; only code that called the method by name stops compiling, and
  `json.Marshal` replaces it. (`323f539`; 0019-MADR, I8.)

#### Fixes

- A request sent from inside a notification handler, under its context or one derived from it, no longer waits for that handler:
  a client whose `SessionUpdate` called `CallExtension` or `SendRequest`, or an agent whose `Cancel` handler sent a request,
  deadlocked. Sent under an unrelated context, it still waits. (`c342496`; 0019-MADR, B1.)
- A send returns when its context ends, even while the peer does not read; it used to block past its deadline and hold up every
  other send. One goroutine now writes every message. (`7e7acfd`, `c28159c`; 0019-MADR, B16.)
- A request whose context has already ended is not written; it used to be written and then cancelled. (`c342496`; 0019-MADR, B9.)
- `params` left out, or `null` as ACP's schema allows, read as empty params: `logout` and `session/list` without params are
  handled, where they were refused `-32602`. (`c342496`, `23396f4`; 0019-MADR, B3 and decision 13.)
- A cancel reaches every prompt of its session received before it, two prompts in flight included. A cancel is kept for a prompt
  received before it and not yet stored, and each keep removes the kept cancels no request in flight could still match, so they no
  longer grow without bound.
  (`13dc276`; 0019-MADR, B5 and B7.)
- Two requests in flight with the same id: `$/cancel_request` reaches the later one after the first returns. (`c342496`; 0019-MADR,
  B11.)
- A handler that returns a nil `*RequestError` as a non-nil `error` is answered `-32603`, not success. (`c342496`; 0019-MADR, B8.)
- `Negotiated()` returns a copy that shares nothing with the record the gates read, or with the values passed to `Initialize` or a
  handler. (`199f99e`; 0019-MADR, B13.)
- Malformed input is logged by its fault, its size and its id, never its content, which may hold prompt text or file contents.
  (`c342496`; 0019-MADR, B15.)
- The examples: a permission prompt ends at EOF or when its context ends; the REPL examples cancel a turn with `:cancel`; the agent
  exits on interrupt and builds its paths under the session's root; the example agent keeps its connection through
  `AgentConnAware`. (`77704b7`, `3f0a822`; 0019-MADR, E1 to E4 and B2.)

#### Additions

- `Close() error` on `Connection`, `AgentSideConnection` and `ClientSideConnection`: it ends the connection, closes the reader and
  the writer that implement `io.Closer`, and returns their errors; a later `Close` returns nil. (API listing; `7e7acfd`, `d4e1ec4`.)
- `ErrPeerDisconnected` and `ErrConnectionClosed`, and `(*RequestError).Unwrap`: an error the connection makes for a local failure
  keeps it, so `errors.Is` finds `context.Canceled`, `context.DeadlineExceeded`, either sentinel, or a write's error. Codes are
  unchanged; an error the peer answered unwraps to nothing. (API listing; `29778f3`, `7e7acfd`; 0019-MADR, decision 3.)
- `WithMaxConcurrentRequests(n int)`: over `n` running handlers, a request is answered at once, `-32603` with `{"error": "too many
  concurrent requests", "limit": n}`, and its handler does not run. The default is no limit. (API listing; `52fe4ed`; 0019-MADR,
  decision 4.)
- `AgentConnAware` and `ClientConnAware`, the interfaces of the hand-off above. (API listing; `3f0a822`; 0019-MADR, decision 1.)
- The README gains "Closing and limits", and documents the hand-off, error causes, extension messages and the undeclared mode.
  (`d4e1ec4`.)

#### For the generator

These change nothing in the generated SDK beyond what is listed above:

- The protocol table gains the `ClosesSession` role, set on `session/close`, and the gate kind `GateUndeclaredVariant`, set on
  `CreateElicitationRequest`; it now has 19 capability rules, each also applied to every copy of its definition the merge made from
  the unstable schema. (`13dc276`, `03fcfa6`, `c28159c`.)
- The side connections' constructors call the hand-off setter between assigning the connection and starting it, and gain `Close`
  and an unexported `inboundGate`. (`3f0a822`, `7e7acfd`, `943f073`.)
- The dispatch decodes params through `decodeParams` and refuses a method received as the other kind. (`23396f4`.)
- A struct gets a `MarshalJSON` only when it does more than encode its fields, and every line of a generated comment is a line
  comment, so no description can become code. (`323f539`, `a356149`.)

## Sources not in the release text

- The commits since `v0.17.1` that change the SDK's Go code: `c342496`, `23396f4`, `13dc276`, `03fcfa6`, `199f99e`, `77704b7`,
  `29778f3`, `7e7acfd`, `52fe4ed`, `3f0a822`, `943f073`, `a49d0bb`, `c28159c`, `323f539`, `d4e1ec4` (`git log v0.17.1..HEAD -- '*.go'
  ':!cmd/generate'`); and the generator's: `23396f4`, `13dc276`, `03fcfa6`, `199f99e`, `a356149`, `7e7acfd`, `3f0a822`, `943f073`,
  `c28159c`, `323f539` (`git log v0.17.1..HEAD -- cmd/generate`). `a49d0bb` changes tests only. The others change records and
  documentation.
- The API listing difference, classified: changed, none; removed, 7 lines, the `MarshalJSON` methods of `AgentCapabilities`,
  `AuthCapabilities`, `ClientCapabilities`, `FileSystemCapabilities`, `InitializeRequest`, `McpCapabilities` and
  `PromptCapabilities`; added, 9 lines, `func WithMaxConcurrentRequests (n int) ConnectionOption`, the three `Close () error`
  methods, `method (*RequestError) Unwrap () error`, `type AgentConnAware`, `type ClientConnAware`, `var ErrConnectionClosed` and
  `var ErrPeerDisconnected`.
- `json.Marshal` of every generated type's zero value and of every instance the conformance builder makes, 1528 encodings, is
  identical byte for byte before and after the `MarshalJSON` removal ([0019-PLAN-api-decisions.md](../decisions/0019-PLAN-api-decisions.md),
  step 8).
- Each change, its tests, the planted breaks each test was seen to fail on, and a run on Windows for each step are in the execution
  records of [0019-PLAN-behaviour-fixes.md](../decisions/0019-PLAN-behaviour-fixes.md) and
  [0019-PLAN-api-decisions.md](../decisions/0019-PLAN-api-decisions.md), with every deviation from them.
- Not in this release: phases 3 to 5 of 0019 (generator hardening, tooling, idioms and performance), which ride the releases after
  each lands (0019-MADR, decision 12).
