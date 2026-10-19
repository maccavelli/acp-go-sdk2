# Release notes for v1.16.3, against v1.16.2

Observed 2026-10-19 on `54cb0a2`, for [0024-PLAN-documentation.md](../decisions/0024-PLAN-documentation.md), step 4. This report
drafts the text of the GitHub release for `v1.16.3`, as `RELEASING.md` describes it. It decides nothing.

Each statement names its source:

- a record;
- a commit;
- or the exported API listing of the 0013 report's listing program, run on `git archive v1.16.2` (2429 lines) and on the tree at
  `54cb0a2` (2429 lines).

Bring it up to date if more changes land before the tag.

- No exported API changes, and every change to the SDK's non-test Go files is to comments, so the release is proposed as the patch
  `v1.16.3` ([0024-PLAN-documentation.md](../decisions/0024-PLAN-documentation.md), "Rollout and Rollback"). The owner confirms it
  at the tag.
- What a consumer meets is documentation: on pkg.go.dev, in the README and in `RELEASING.md`.

The section "Release text" is what goes into the GitHub release.

## Release text

### v1.16.3

The ACP schema is still `1.25.0` (`git diff v1.16.2 -- schema/` is empty), and the exported API and the SDK's behaviour are unchanged:
every change to a non-test Go file of the SDK is to its comments. This release is phase 2 of
[0024-MADR-go-sdk-best-practices-on-a-go-1-26-9-floor.md](../decisions/0024-MADR-go-sdk-best-practices-on-a-go-1-26-9-floor.md):
documentation.

#### API documentation

- **Every typed method names the errors it returns.** (`3db905a`.)
  - Before anything is sent, a typed method can return an error wrapping `ErrNotInitialized`, a `*CapabilityError` and a
    `*ValidationError`. `Initialize` names `*ProtocolVersionError` instead of `ErrNotInitialized`, and the client's `Authenticate`
    names `ErrTerminalAuthMethod`.
  - Once sent, it names a `*RequestError` wrapping `context.Cause(ctx)`, `ErrPeerDisconnected` or `ErrConnectionClosed`, or the
    write's error, and the peer's error answer.
  - A request also names the errors of a response that breaks the schema, breaks what initialize agreed, or does not decode.
  - `CallExtension` and `NotifyExtension` name theirs. They send before initialize too, so theirs do not include `ErrNotInitialized`.
- **Every generated type's doc starts with its name** and what it is built from, such as "AgentAuthCapabilities is the
  AgentAuthCapabilities schema definition.", before the schema's description. (`3db905a`.)
- **The concurrency contract is stated.** (`3db905a`, `54cb0a2`.)
  - `Connection`, `AgentSideConnection` and `ClientSideConnection` are safe for concurrent use. Messages sent from several
    goroutines are written whole, one at a time.
  - The connection calls the `Agent` or `Client` method for each request on a goroutine of its own, and the methods for
    notifications one at a time, in order.
- **The wire-schema types the connection does not use say so.** (`3db905a`.)
  - These are the ten JSON-RPC envelope types, `AgentRequest` to `ClientError`, plus `Error` and `CancelRequestNotification`.
  - `ErrorCode`'s doc says how to compare it with `RequestError.Code`, which is an `int`.
- **The options' semantics are stated** on `ConnectionOption`. (`54cb0a2`.)
  - Options apply in order.
  - `n <= 0` keeps the value already set for `WithMaxQueuedNotifications` and `WithMaxMessageSize`, but removes the limit of
    `WithMaxConcurrentRequests`.
  - A nil option panics.
  - The tool-call options apply in order.
- **The package overview** in `doc.go` describes the two sides, the constructors, how a connection ends, and the error model.
  (`54cb0a2`.)
- **Four examples with checked output:** `ExampleNewClientSideConnection`, `ExampleStartToolCall`, `ExampleRequestError` and
  `ExampleConnection_Close`. (`54cb0a2`.)

#### README and releasing

- **The README has a quickstart,** "A first connection", which a test keeps equal to `ExampleNewClientSideConnection`. (`54cb0a2`.)
- **`RELEASING.md` names the release notes and the API listing.** It also says an incompatible change needs `/v2`. (`54cb0a2`.)

#### For contributors

- **The generator** writes the type and method docs from the protocol table and the model. The protocol table gains `DocNotes`.
  (`3db905a`.)
- **New tests:**
  - `TestExportedTypeDocsStartWithName` and `TestTypedMethodDocsNameErrors` check the docs;
  - `TestMethodErrorsAsDocumented` checks the documented errors through the side connections;
  - `TestConcurrentUse`, under `-race`, calls every exported method of the three connection types from several goroutines;
  - `TestOptionSemantics` checks the options' doc;
  - `TestReadmeQuickstartIsTheExample` checks the README's quickstart.

  (`3db905a`, `54cb0a2`.)

## Sources not in the release text

- **Commits since `v1.16.2`** (`git log v1.16.2..HEAD`):
  - `3db905a` changes the generator, the three regenerated files, `extensions.go`'s comments and tests;
  - `54cb0a2` changes `connection.go`, `doc.go` and `helpers.go`'s comments, the README, `RELEASING.md` and tests;
  - `8c9ad41`, `fbd9afa` and `cd7c254` change records only (`git show --name-only`).
- **Comments only.** `git diff --name-only v1.16.2 HEAD -- '*.go' ':!*_test.go'` lists seven SDK files and five of the generator's.
  A script outside the tree compared each of the seven, `agent_gen.go`, `client_gen.go`, `connection.go`, `doc.go`, `extensions.go`,
  `helpers.go` and `types_gen.go`, with `v1.16.2`. With comments and positions dropped, each has the same syntax tree.
- **The API listing:** 0 lines removed, 0 added, `v1.16.2` against `54cb0a2`.
- **The evidence:** [0024-PLAN-documentation.md](../decisions/0024-PLAN-documentation.md), steps 2 and 3, with their three deviations,
  the mapping of each documented error to a test, the planted breaks (19 in step 2, 14 in step 3), and the gates on go1.27.2 and
  go1.26.9.
- **Not in this release:** the MADR's phases 3 and 4 (behaviour fixes, the `gorelease` gate and `Deprecated:`), and schema `1.26.0`.
