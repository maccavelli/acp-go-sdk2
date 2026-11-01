# Release notes for v1.31.0, against v1.30.0

Observed 2026-11-01 on `27afb8f`, for
[0046-PLAN-sending-and-receiving-middleware.md](../decisions/0046-PLAN-sending-and-receiving-middleware.md), phase 7. This report
drafts the text of the GitHub release for `v1.31.0`, as `RELEASING.md` describes it. It decides nothing. It is committed before
the tag, so the tagged tree holds it. The commit that adds it changes `tools/consumer.py`, `tools/README.md`, records and nothing
else, so the API it tags is `27afb8f`'s.

Each statement names its source:

- a record;
- a commit;
- or `make api`, run on `27afb8f` with only `tools/consumer.py`, `tools/README.md`, `docs/README.md` and the records uncommitted,
  which hold no Go package.

Summary:

- Fourteen additions to the exported API: the `Middleware` type, `WithReceivingMiddleware`, `WithSendingMiddleware`,
  `RequestIDFromContext`, `ErrSendingMiddleware`, `WithObserver`, `Observation`, `ObservationKind` and its six constants.
  `make api` against `v1.30.0`: "0 incompatible, 14 compatible". The release is the minor `v1.31.0`, as `RELEASING.md` says for
  compatible changes and the 0046 MADR's "More Information" names it.
- No change of behaviour for a connection that registers no middleware and no observer: nothing sent or accepted on the wire
  changes, and its allocations per message are unchanged.
- The `go` lines and the requirements are unchanged. No generated file changed, and the schema did not.

The section "Release text" is what goes into the GitHub release.

## Release text

### v1.31.0

The ACP schema is still `1.25.0` (`git diff v1.30.0 -- schema/` is empty). This release adds middleware around the messages a
connection hands to its handler and around those the application sends, and an observer for the messages the connection handles
itself ([0046-MADR-sending-and-receiving-middleware.md](../decisions/0046-MADR-sending-and-receiving-middleware.md)).

`make api` reports fourteen compatible additions and no incompatible change.

#### Added: sending and receiving middleware

`acp.Middleware` is `func(next acp.MethodHandler) acp.MethodHandler`, the type a `NewConnection` handler already has. A
middleware calls `next` at most once, or answers in its place. Both options are `ConnectionOption`s, so `NewConnection`,
`NewAgentSideConnection` and `NewClientSideConnection` all take them. (`df22f9e`, `f23e107`.)

- **`acp.WithReceivingMiddleware(mw...)`** wraps every request and notification that reaches the handler.
  - It runs where the handler runs: on the request's goroutine, inside its `WithMaxConcurrentRequests` slot and its
    `WithMaxQueuedBytes` count, or on the notification worker.
  - It sees the method and the raw params before the side connections decode them, before `WithInboundRewrite`,
    `WithInboundValidation` and the check of what `initialize` agreed, and the result as the dispatch returns it, checked and
    not yet encoded.
  - One that answers without calling `next`, to refuse a method for example, skips the validation of that message.
- **`acp.WithSendingMiddleware(mw...)`** wraps every request and notification the application sends: `SendRequest`,
  `SendRequestNoResult`, `SendNotification`, `CallExtension`, `NotifyExtension`, and every typed method, `Initialize` included.
  - Before the chain, the connection checks and encodes the message as it does without one, so a message it would refuse never
    reaches the chain.
  - Inside it, the connection writes the message and waits for the answer and for the notifications received before it, so a
    middleware's timing covers the round trip.
  - After it, the result is decoded and checked as a peer's result is, so a result a middleware returns instead is checked too.
  - The connection's own `$/cancel_request`, sent when a caller's context ends, does not pass the chain.
- **Order.** The first registered is outermost: `WithReceivingMiddleware(a, b)` then `WithReceivingMiddleware(c)` runs `a`, `b`,
  `c`, then the handler. Repeated middleware options **append**, the one exception to "the last option wins", so a library that
  adds its own middleware keeps the application's. Each `Middleware` is called once, when the connection is built; a `nil` one,
  or one that returns a `nil` handler, panics there.
- **`acp.RequestIDFromContext(ctx)`** gives a copy of the id of the request being handled or sent, and `false` for a
  notification. Handlers can read it too.
- **Misuse fails at once.** On the sending side, calling `next` twice, with a context that does not carry the call, with
  another method, or with params that are not `nil`, an object or an array, and replacing `Initialize`'s result, fail the call
  with a `-32603` wrapping **`acp.ErrSendingMiddleware`**, and nothing is written.

A tracing middleware, with the standard library alone, adds a W3C trace context where ACP reserves it, in `params._meta`:

```go
trace := func(next acp.MethodHandler) acp.MethodHandler {
    return func(ctx context.Context, method string, params json.RawMessage) (any, *acp.RequestError) {
        p := map[string]any{}
        if len(params) > 0 {
            if err := json.Unmarshal(params, &p); err != nil {
                return nil, acp.NewInternalError(map[string]any{"error": err.Error()})
            }
        }
        p["_meta"] = acp.TraceMeta(traceparent, "", "")
        traced, err := json.Marshal(p)
        if err != nil {
            return nil, acp.NewInternalError(map[string]any{"error": err.Error()})
        }
        return next(ctx, method, traced)
    }
}
conn := acp.NewClientSideConnection(client, stdin, stdout, acp.WithSendingMiddleware(trace))
```

#### Added: an observer for what the chains do not see

**`acp.WithObserver(fn)`** reports to `fn` each message the connection handles itself, which no middleware sees, as an
`acp.Observation` with its `Kind`, `Method`, a copy of its `ID`, its `Size` in bytes and its `Err`. With both chains and an
observer set, every message is seen once. The last `WithObserver` given wins, and `nil` removes it. (`3f7f4b1`.)

| Kind | What |
| :--- | :--- |
| `ObservedFault` | a line answered as invalid: over the size limit, not JSON, not an object, or not a valid request |
| `ObservedCancelReceived` | a `$/cancel_request` the peer sent, `ID` the request it names |
| `ObservedCancelSent` | a `$/cancel_request` the connection sent as a caller's context ended |
| `ObservedRefused` | a request answered without its handler: over the concurrency limit, past the byte budget, or during `Shutdown` |
| `ObservedUnmatchedResponse` | a response no pending call took, such as a late answer |
| `ObservedDropped` | a notification dropped under `OverflowDropNewest`, or the one whose overflow closed the connection |

`ObservationKind`'s `String` gives `fault`, `cancel_received`, `cancel_sent`, `refused`, `unmatched_response` and `dropped`, for
a log or a metric's label. The observer runs on the goroutine that met the message, the reader for every kind but
`ObservedCancelSent`; it must be safe for concurrent use and must not block, since the connection reads nothing while it runs. No
lock of the connection's is held while it runs, so it may call `QueuedBytes` or `RefusedMessages`.

#### Cost

- **Nothing registered:** no allocation is added. `BenchmarkPromptRoundTrip` stays at 99 allocations per round trip and
  `BenchmarkSessionUpdateNotify` at 65, with median times within 5% of `v1.30.0`'s code. With no observer, each place it would
  be called costs one `nil` check. (`1de8d98` to `f8e7ecb`.)
- **Sending middleware:** three allocations per request sent and two per notification, plus what the middleware allocates.
  `BenchmarkPromptRoundTripMiddleware`, one middleware each way on both sides, has 102 allocations per round trip.
- **Receiving middleware and an observer:** none added by the SDK.

#### For maintainers

- **Internals.** `SendRequestNoResult` now shares `exchange` with `SendRequest`, keeping its checks and errors in their order,
  and the connection's own `$/cancel_request` is written on a path of its own rather than through `SendNotification`.
  (`1de8d98`, `f23e107`.)
- **Docs.** The README gains "Middleware and observing" and "What the chains do not see"; `docs/architecture.md` describes the
  chains and the observer; `ExampleWithReceivingMiddleware`, `ExampleWithSendingMiddleware` and `ExampleWithObserver` show an
  authorization refusal, a tracing middleware and a refusal observed. (`f8e7ecb`.)
- **Tests.** `connection_middleware_test.go`, `connection_sending_middleware_test.go` and `connection_observer_test.go`, every
  sending path, every misuse and every observation kind among them; `TestOptionSemantics` requires the middleware options to
  append; `TestReadmeMiddlewareIsTheExample` keeps the README's example equal to `ExampleWithSendingMiddleware`; two benchmarks.
  (`df22f9e`, `f23e107`, `3f7f4b1`, `f8e7ecb`.)
- **The generator's engine tests** compile `middleware.go` and `observer.go` with the runtime they copy. (`767ee32`.)
- **The tree.** The leftover `communique.toml` is removed
  ([0010-MADR-align-docs-with-current-tree.md](../decisions/0010-MADR-align-docs-with-current-tree.md), the amendment of
  2026-10-31; `dd63f56`).
- **Tools.** `tools/consumer.py` builds a module at the Go floor that also registers receiving and sending middleware and an
  observer, and requires their counts, `RequestIDFromContext`'s ids, a refusal over `WithMaxConcurrentRequests(1)`, and a
  misuse that `errors.Is` reads as `acp.ErrSendingMiddleware`.

## Sources not in the release text

- **Commits since `v1.30.0`** (`git log v1.30.0..27afb8f`), eleven:
  - `476d765` records the `v1.30.0` release and marks the 0045 PLAN complete;
  - `dd63f56` removes `communique.toml` and records the 0010 amendment;
  - `523f319` records CI for that amendment in the 0010 PLAN;
  - `fbbbd5c` revises the 0046 MADR and PLAN;
  - `1de8d98` changes `connection.go`;
  - `df22f9e` changes `connection.go` and `options_test.go`, and adds `middleware.go` and `connection_middleware_test.go`;
  - `f23e107` changes `connection.go`, `errors.go`, `middleware.go` and `options_test.go`, and adds
    `connection_sending_middleware_test.go`;
  - `3f7f4b1` changes `connection.go`, and adds `observer.go` and `connection_observer_test.go`;
  - `f8e7ecb` changes `README.md`, `bench_test.go`, `docs/architecture.md`, `docs_test.go` and `example_api_test.go`;
  - `767ee32` changes `cmd/generate/internal/enginetest/api.go`;
  - `27afb8f` records phase 6's CI in the 0046 PLAN;
  - each changes `docs/README.md`, a MADR, a PLAN or a report as well (`git show --name-only`).
- **Go files changed.** `git diff --name-status -M v1.30.0 27afb8f -- '*.go' ':!*_test.go'` lists 5 files: `connection.go` and
  `errors.go` changed, `middleware.go` and `observer.go` added, and `cmd/generate/internal/enginetest/api.go`, a test helper of
  the generator's module, changed.
- **Unchanged** (`git diff --stat v1.30.0 27afb8f`, empty):
  - `go.mod`, `go.sum`, `cmd/generate/go.mod` and `cmd/generate/go.sum`;
  - the schema, the workflows, the `Makefile`, `RELEASING.md` and `scripts/`;
  - `acptest/`, `agentproc/`, `clienthost/`, `remote/` and `internal/`;
  - every `*_gen.go` file.
- **`make api` on `27afb8f`:** exit 0; "compatible: … added" for `ErrSendingMiddleware`, `Middleware`, `Observation`,
  `ObservationKind`, the six `Observed…` constants, `RequestIDFromContext`, `WithObserver`, `WithReceivingMiddleware` and
  `WithSendingMiddleware`; "against v1.30.0: 0 incompatible, 14 compatible"; and "reported packages unstable, remote: 0
  incompatible, not gated".
- **CI on `767ee32`**, pushed with `fbbbd5c` to `f8e7ecb`:
  - run `41317956970`, "completed success", six jobs on Ubuntu, Windows and macOS at Go `1.27.3` and `1.26.10`;
  - the Docs workflow's run `41317956982`, "completed success".
- **The consumer before the tag.** A scratch script built `tools/consumer.py`'s program and test against an export of `27afb8f`
  through a `replace`, at `go1.26.10`, three times, and ran the program 50 times more in each. Every output equals `WANT_RUN`,
  whose new lines give the middleware's counts, the ids, `observed map[refused:1]` and `misuse true`; the test and the Windows
  build pass. `tools/consumer.py v1.30.0`, a release without middleware, fails its build with "undefined:
  acp.RequestIDFromContext", as required. In scratch copies, with the misuse's cause dropped the run differs at `misuse false`,
  and with refusals not reported it prints "observer: the refusal was not reported". `consumer.py v1.31.0` runs after the tag.
- **The evidence:**
  - the 0046 MADR's findings F1 to F13, and the survey of other implementations under "Sources";
  - the 0046 PLAN's planted breaks in phases 1 to 6, each failing as required, and its benchmarks against `523f319`, whose Go
    files are `v1.30.0`'s (`git diff --stat 523f319 v1.30.0 -- '*.go'` is empty);
  - the gates on macOS and on the Linux and Windows hosts, 18 of 18, in the 0046 PLAN's phase 6;
  - the deviation entries, 1 to 3, each recorded in the PLAN, and the MADR's amendments of D5 and D11.
