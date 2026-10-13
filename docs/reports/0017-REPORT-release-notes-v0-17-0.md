# Release notes for v0.17.0, against v0.16.0

Observed 2026-10-12 on `d7babc2`, for
[0017-PLAN-remediate-acp-v1-conformance-violations.md](../decisions/0017-PLAN-remediate-acp-v1-conformance-violations.md), step 12.
This report drafts the text of the GitHub release for `v0.17.0`, as `RELEASING.md` describes it. It decides nothing. Each statement names
its source: a record, a commit, or the exported API listing of the 0013 report's listing program, run on `git archive v0.16.0` (2390
lines) and on `d7babc2` (2411 lines). Bring it up to date if more changes land before the tag. The version is the next minor, `v0.17.0`;
the owner confirms it at the tag.

The section "Release text" is what goes into the GitHub release.

## Release text

### v0.17.0

The ACP schema is still `1.25.0` (`schema/version`; `git diff v0.16.0 -- schema/` is empty). This release makes the SDK follow ACP v1
and JSON-RPC 2.0 where it did not: a cancelled prompt turn is answered `cancelled`, pending permission requests are answered when their
turn is cancelled, malformed input is answered, the protocol version is negotiated, and what each side sends and receives is checked
against the capabilities `initialize` negotiated. Most of these are breaking changes for an application that relied on the old
behaviour; each says what to do. The exported API only grows. (0017-MADR; the 0016 audit report.)

#### Breaking changes

- **`initialize` comes first.** Until it completes on a connection, every typed method but `Initialize` returns an error wrapping
  `ErrNotInitialized` and writes nothing, and a typed request received is answered `-32601`. Call `Initialize` before anything else; a
  test that builds connections without it now needs it. (`fb3464e`; 0017-MADR, G1.)
- **Methods the peer did not advertise are refused.** A typed method returns a `*CapabilityError` naming the method and the capability,
  and writes nothing, when the record of `initialize` does not allow it; received, such a request is answered `-32601` and a notification
  is dropped. The gated methods: `session/load` (`loadSession`); `session/list`, `session/delete`, `session/resume`, `session/close` and
  the unstable `session/fork` (`sessionCapabilities.*`); `logout` (`auth.logout`); `fs/read_text_file` and `fs/write_text_file`
  (`fs.readTextFile`, `fs.writeTextFile`); the `terminal/*` methods (`terminal`); `elicitation/complete` (`elicitation.url`). Advertise
  what your side supports in `initialize`, and check `Negotiated()` or `Negotiation.Allows(method)` before offering a feature. (`fb3464e`;
  0017-MADR, G1.)
- **Values the peer did not advertise are refused.** The same holds for what a message carries: a non-empty `additionalDirectories`
  (`sessionCapabilities.additionalDirectories`); an `http`, `sse` or unstable `acp` MCP server (`mcpCapabilities.*`); an `image`, `audio`
  or `resource` prompt block (`promptCapabilities.image`, `.audio`, `.embeddedContext`); an elicitation's `form` or `url` mode
  (`elicitation.*`); a `notice` or compaction session update (`session.notices`, `session.compaction`); a `boolean` config option
  (`session.configOptions.boolean`); a `terminal` auth method in the `initialize` answer (`auth.terminal`). The `*CapabilityError` names
  the property, such as `prompt[1]`, or the variant, such as `image`. Received, a request is answered `-32602`, a notification dropped, and
  a response returned to the caller as an error. A handler's result that carries one, such as a `session/new` answer with a boolean config
  option for a client that did not advertise them, is not sent: the peer gets `-32603`. An agent sends these only when the client
  advertised them; read the client's capabilities from `Negotiated()`. (`8fae8b3`; 0017-MADR, G1.)
- **`Authenticate` with a terminal auth method's id is refused.** It returns an error wrapping `ErrTerminalAuthMethod` and sends nothing,
  as ACP forbids it: a terminal method is run, not authenticated. Received, it is answered `-32602`. (`8fae8b3`; 0017-MADR, G1.)
- **The protocol version is checked.** A version left zero in `InitializeRequest` or in an agent's `InitializeResponse` is
  `ProtocolVersionNumber`. A client asked to send another gets a `*ProtocolVersionError`; an agent answering another makes `Initialize`
  return a `*ProtocolVersionError` with `Requested` and `Answered` and record nothing, so the connection stays unusable: close it and tell
  the user. An agent whose handler answers another version is answered `-32603`. (`e0db539`; 0017-MADR, N1.)
- **A cancelled prompt turn is answered `cancelled`.** When `session/cancel` (or `$/cancel_request`, or a later prompt for the same
  session) cancels a turn before its handler returns, the answer is `{"stopReason":"cancelled"}`, whatever the handler returned; it was
  `-32800` when the handler returned the context's error. `context.Cause` of the handler's context is `ErrPromptCancelled`. A cancel that
  arrives after the handler returned changes nothing. A client that looked for `-32800` on `Prompt` reads `StopReason` instead.
  (`f945232`; 0017-MADR, P1.)
- **Pending permission requests are answered on cancel.** When a client sends `session/cancel`, through `Cancel` or `Prompt`, each of that
  session's `session/request_permission` requests still running is answered `cancelled` at once; its handler's context is cancelled with
  `ErrPromptCancelled`, and what it returns later is discarded. A handler that shows a dialog closes it when its context ends. Marking the
  turn's unfinished tool calls `cancelled` stays the application's. (`4e04f37`; 0017-MADR, Q1.)
- **Malformed input is answered.** A line that is not JSON is answered `-32700`; JSON that is not an object, a batch included, and an
  object that is not a valid request or notification, `-32600`. Before, all of these were dropped, and a request with a `jsonrpc` other
  than `"2.0"` was handled. A line longer than the limit, 10 MiB by default, is skipped and answered `-32600`; it used to close the
  connection. (`f025059`; 0017-MADR, J1.)

#### Fixes

- A second prompt for a session no longer loses its own cancel: when the superseded turn returned, it removed the session's entry, which
  was by then the second turn's, so a later `session/cancel` did not reach it. (`f945232`; 0017-MADR, "Beyond the report: superseding a
  prompt loses the successor's cancel".)
- A response with neither `result` nor `error`, with both, or with another `jsonrpc` fails the call it answers; it completed as a success
  with a zero result. A response that matches no call is logged at Debug and never answered. (`f025059`; 0017-MADR, "Beyond the report: a
  malformed response completes the call as a success".)
- A request whose `id` is `null` is answered, with `"id": null`; it was taken for a notification, so its handler ran unanswered.
  (`f025059`; 0017-MADR, V2.)

#### Additions

- `Negotiation`, with `Request` and `Response` and `Allows(method string) error`; `Negotiated() (Negotiation, bool)` on
  `AgentSideConnection` and `ClientSideConnection`. (API listing; `e0db539`, `fb3464e`.)
- `CapabilityError` (`Method`, `Capability`, `Field`, `Variant`) and `ProtocolVersionError` (`Requested`, `Answered`). (API listing;
  `80195ad`.)
- `ErrPromptCancelled`, `ErrNotInitialized` and `ErrTerminalAuthMethod`. (API listing; `80195ad`, `8fae8b3`.)
- `WithMaxMessageSize(n int)`, the largest line the connection reads. (API listing; `f025059`.)
- `NewResourceNotFound(uri string)`, the `-32002` error ACP defines. (API listing; `80195ad`; 0017-MADR, K1.)
- A validation hook set with `WithInboundValidation` sees the capability errors and may accept them: a message that fails validation
  reaches it with that error first and, only if it accepts that, with the capability error. (`fb3464e`; 0017-MADR, G1.)
- The README gains "Initialization and capabilities", "Cancellation" and "Errors on the wire". `Connection()`, `SendRequest`,
  `SendRequestNoResult`, `SendNotification` and the extension methods are not checked against the record, and their documentation says
  so. (`d7babc2`.)

#### For the generator

These change nothing in the generated SDK beyond what is listed above:

- `AgentSideConnection` and `ClientSideConnection` are generated, with their state, in `agent_gen.go` and `client_gen.go`; the
  hand-written `agent.go` and `client.go` are gone, and their exported API is unchanged. (`19e062d`; 0017-MADR, S1.)
- The protocol table gains the `initialize` rule, the answer-on-cancel rule for permission requests, a stop key for the prompt turn, the
  15th method gate (`elicitation/complete`), and 18 capability rules, each with its specification sentence; the method gates are now
  enforced. The model refuses a rule whose subject has the wrong shape, and applies no capability rule to a schema without `initialize`.
  (`f945232`, `4e04f37`, `e0db539`, `fb3464e`, `8fae8b3`.)

## Sources not in the release text

- The commits since `v0.16.0` that change the SDK's Go code: `80195ad`, `19e062d`, `f945232`, `4e04f37`, `f025059`, `e0db539`,
  `fb3464e`, `8fae8b3`, `d7babc2` (`git log v0.16.0..HEAD -- '*.go' ':!cmd/generate'`); and the generator's: `19e062d`, `f945232`,
  `4e04f37`, `e0db539`, `fb3464e`, `8fae8b3`, `d7babc2` (`git log v0.16.0..HEAD -- cmd/generate`). `271aa1c` and `6ec3916` change records
  only.
- The API listing difference, classified: changed, none; removed, none; added, 21 lines: the types `CapabilityError`, `Negotiation` and
  `ProtocolVersionError` with their 8 fields, the 2 `Error` methods, `Negotiation.Allows`, the 2 `Negotiated` methods,
  `NewResourceNotFound`, `WithMaxMessageSize`, and the 3 `Err…` variables.
- Each enforcement and its tests are in the PLAN's execution records, steps 3 to 11, with the planted breaks each test was seen to fail
  on, and a run of the tests on Windows for each step.
- Not in this release: unstable features whose capabilities the specification does not tie to a rule (`nes`, `plan`, `providers`,
  `subagents`, `positionEncoding`) are not checked (0017-MADR, G1).
