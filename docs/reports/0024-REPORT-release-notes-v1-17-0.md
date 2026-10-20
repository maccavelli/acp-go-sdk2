# Release notes for v1.17.0, against v1.16.3

Observed 2026-10-19 on `945115b`, for [0024-PLAN-behaviour-fixes.md](../decisions/0024-PLAN-behaviour-fixes.md), step 5. This
report drafts the text of the GitHub release for `v1.17.0`, as `RELEASING.md` describes it. It decides nothing.

Each statement names its source:

- a record;
- a commit;
- or the exported API listing of the 0013 report's listing program, run on `git archive v1.16.3` (2429 lines) and on the tree at
  `945115b` (2429 lines).

Bring it up to date if more changes land before the tag.

- No exported API changes. Three behaviours change, so the release is proposed as the minor `v1.17.0`
  ([0024-PLAN-behaviour-fixes.md](../decisions/0024-PLAN-behaviour-fixes.md), "Rollout and Rollback"). The owner confirms it at the
  tag.
- What a consumer may meet: a constructor that panics on a nil argument, and log lines at other levels.

The section "Release text" is what goes into the GitHub release.

## Release text

### v1.17.0

The ACP schema is still `1.25.0` (`git diff v1.16.3 -- schema/` is empty), and the exported API is unchanged. This release is phase 3
of [0024-MADR-go-sdk-best-practices-on-a-go-1-26-9-floor.md](../decisions/0024-MADR-go-sdk-best-practices-on-a-go-1-26-9-floor.md):
behaviour fixes. Three behaviours change.

#### Behaviour changes

- **A nil argument panics in the constructor.** (`6ceac3f`.)
  - `NewConnection` panics on a nil `peerInput` or `peerOutput`. `NewAgentSideConnection` and `NewClientSideConnection` panic on a
    nil agent or client, `peerInput` or `peerOutput`.
  - The message names the argument, such as "acp: NewAgentSideConnection: nil agent".
  - Each panics before anything is built, and before an `AgentConnAware` or `ClientConnAware` is handed the connection.
  - Before, each of these ended the process later, with a nil pointer dereference on a connection goroutine.
  - A nil handler for `NewConnection` is unchanged: it answers every request `-32601` and drops every notification.
- **Faults of the peer's message log at Warn, not Error.** (`945115b`.)
  - Eleven lines: a line over the size limit; one that is not JSON, not an object, or not a valid request; a response without an id,
    or malformed; an id that cannot be canonicalised; and a `$/cancel_request` with bad params, no `requestId`, or one that cannot be
    canonicalised.
  - A notification the dispatch refused, its error `-32601` or `-32602`, logs at Warn. A notification handler's own error stays at
    Error.
  - A log handler that filters at Error no longer sees these lines.
- **Ids that cannot be canonicalised log their size.** The three lines log `size` in place of the id's or the `requestId`'s bytes,
  which could be any JSON up to the line limit. (`945115b`.)
- **A connection's end logs at Debug, not Info.** A log handler at Info no longer sees "connection closed". (`945115b`.)

#### Documentation

- **A panicking handler ends the process.** The docs of `MethodHandler`, `Agent` and `Client` say so: the connection does not
  recover a panic. (`6ceac3f`.)
- **A handler's error text reaches the peer.** (`e7dd716`.)
  - A `*RequestError` is sent as it is.
  - Any other error is answered `-32603`, or `-32800` when it wraps `context.Canceled`, with its text in the answer's `data` as
    `"error"`. To keep a message private, return a `*RequestError` of your own.
  - An agent's `Prompt` and a client's `RequestPermission` are the exceptions: once their prompt turn is cancelled, they are
    answered cancelled, whatever they return.
  - The docs of `Agent` and `Client`, and the README's "Errors on the wire", say so.
- **Validation messages quote the value they refuse,** for an absolute path or an excluded value. `ValidationError`'s doc and the
  README say so. (`e7dd716`.)
- **The README's "Errors on the wire"** now says a call whose context ended wraps `context.Cause(ctx)`. That is `context.Canceled` or
  `context.DeadlineExceeded` only when no cause was given. (`e7dd716`.)

#### For contributors

- **The generator** writes the side constructors' nil checks. The model, not the shared interface table, names the methods that are
  exceptions to the error paragraph. (`6ceac3f`, `e7dd716`.)
- **New tests:**
  - `TestConstructorsPanicOnNil`;
  - `TestHandlerPanicEndsProcess`, which runs the test binary as a child process;
  - `TestHandlerErrorReachesPeer` and `TestValidationMessagesQuoteValue`;
  - `TestPeerFaultLogLevels` and `TestUncanonicalIDsLogSize`;
  - in the generator, `TestAnsweredCancelledNotes`.

  (`6ceac3f`, `e7dd716`, `945115b`.)

## Sources not in the release text

- **Commits since `v1.16.3`** (`git log v1.16.3..HEAD`):
  - `6ceac3f` changes `connection.go`, the generator, the three regenerated files, three engine cases and tests;
  - `e7dd716` changes `errors.go`'s and the README's docs, the generator's model, `types_gen.go` and tests;
  - `945115b` changes `connection.go`'s log calls and tests;
  - `d6cab27`, `12dd249` and `d5405d1` change records only (`git show --name-only`).
- **SDK files changed.** `git diff --name-only v1.16.3 HEAD -- '*.go' ':!*_test.go'` lists five SDK files and two of the generator's.
  The SDK files are `agent_gen.go`, `client_gen.go`, `connection.go`, `errors.go` and `types_gen.go`.
- **The API listing:** 0 lines removed, 0 added, `v1.16.3` against `945115b`.
- **The evidence:** [0024-PLAN-behaviour-fixes.md](../decisions/0024-PLAN-behaviour-fixes.md). Its steps 2 to 4 hold the planted
  breaks (5, 8 and 6) and the gates on go1.27.2 and go1.26.9. Its two deviations during steps 2 and 3 are in the same record.
- **The MADR's findings** A1 to A4 and A14, with step 1's corrections to A3, A4 and A14.
- **Not in this release:** the MADR's phase 4 (the `gorelease` gate and `Deprecated:`), and schema `1.26.0`.
