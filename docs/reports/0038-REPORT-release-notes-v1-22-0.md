# Release notes for v1.22.0, against v1.21.0

Observed 2026-10-25 on `df0462e`, for
[0038-PLAN-consumer-helpers-kind-and-agent-refusals.md](../decisions/0038-PLAN-consumer-helpers-kind-and-agent-refusals.md),
step 8. This report drafts the text of the GitHub release for `v1.22.0`, as `RELEASING.md` describes it. It decides nothing. It is
committed before the tag, so the tagged tree holds it; the commit that holds it changes only records and `tools/`, so the API it
tags is `df0462e`'s.

Each statement names its source:

- a record;
- a commit;
- or `make api`, run on `df0462e` with nothing uncommitted.

Summary:

- Twenty-two additions to the root package's exported API; `agentproc`'s is unchanged. `make api` against `v1.21.0`: "0 incompatible,
  22 compatible". The release is the minor `v1.22.0`, as `RELEASING.md` says for compatible changes.
- What a consumer may meet: nothing the SDK sends or accepts on the wire changes. The `go` lines and the requirements are unchanged.

The section "Release text" is what goes into the GitHub release.

## Release text

### v1.22.0

The ACP schema is still `1.25.0` (`git diff v1.21.0 -- schema/` is empty). This release adds the protocol helpers the SDK's consumers
had each written for themselves, in several cases differently and once wrongly
([0038-MADR-consumer-helpers-kind-and-agent-refusals.md](../decisions/0038-MADR-consumer-helpers-kind-and-agent-refusals.md)).
`make api` reports twenty-two compatible additions and no incompatible change. Nothing sent or accepted on the wire changes.

#### New for clients

- **`AllSessions(ctx, client, req)`** ranges over every session `session/list` pages through, following `nextCursor`. A cursor the
  agent gives twice yields `ErrRepeatedCursor` and stops, so the list is known to be incomplete; an error yields once and stops; a
  `break` sends nothing more; a nil client yields an error. (`0ac866b`.)
- **`PickPermissionOption(opts, kinds...)`** picks an option by its `kind`, trying each kind in turn, and never by its name, so an
  option named "Don't allow" of kind `reject_once` is never taken as an allow. `RequestPermissionResponse.Chosen()` gives the
  selected option. (`0ac866b`.)
- **`SessionInfo.Updated()`** parses `updatedAt` as RFC 3339. **`ConfigOptionByCategory`** finds a select option by category, such
  as the model, and **`SessionConfigSelectOptions.All()`** lists its values, grouped or not. **`MetaObject`** and
  **`DecodeMeta[T]`** read a namespaced `_meta` entry, untyped or into a type. (`0ac866b`.)

```go
for s, err := range acp.AllSessions(ctx, client, acp.ListSessionsRequest{}) {
    if err != nil {
        return err // errors.Is(err, acp.ErrRepeatedCursor): the agent repeated a cursor
    }
    fmt.Println(s.SessionId)
}
```

#### New for agents

- **`AgentRefusals`** answers method-not-found (`-32601`), with data naming the method, for the five `Agent` methods an agent may
  leave unadvertised: `ListSessions`, `DeleteSession`, `ResumeSession`, `CloseSession` and `Logout`. Embed it and implement the
  seven ungated methods, which stay required at compile time, as `ClientRefusals` does for a client. (`b4ca784`.)
- **`ResolveStdioCommands(exec.LookPath)`** is a `WithInboundRewrite` hook that resolves a client's bare stdio MCP command, such
  as `npx`, to an absolute path before the SDK refuses it, for `session/new`, `session/load`, `session/resume` and the unstable
  `session/fork`; what it cannot resolve is refused `-32602`, as before. **`ChainRewrites`** combines it with rewrites of your
  own, since a connection keeps one rewrite hook. (`b4ca784`.)
- **`OfferedAuthMethods(req.ClientCapabilities, methods)`** drops terminal auth methods for a client that did not advertise
  `auth.terminal`, so an `initialize` answer built from it is one the SDK sends. (`b4ca784`.)

#### New for both sides

- **`IsMethodNotFound`**, **`IsAuthRequired`** and **`IsCancelled`** classify an error, wrapped or not: a peer's `-32601`, or a
  method the peer did not advertise; `-32000`; and `context.Canceled`, `ErrPromptCancelled` or `-32800`.
  **`(*RequestError).Reason()`** gives the `"error"` of its data, where the SDK writes why it refused, or else its message.
  (`79038ac`.)
- **`Kind()`** on `ContentBlock`, `ToolCallContent`, `RequestPermissionOutcome` and `SessionConfigOption`, as on
  `SessionUpdate`: the discriminator of the variant a value sends, or of an unknown one received, and `""` for the zero value.
  (`cb1a763`.)
- **`Connection.RefusedMessages()`** counts the received messages the SDK refused after decoding them: by a rewrite hook, by
  validation the validation hook did not accept, or by what `initialize` negotiated. (`b039e6b`.)

#### A rule for helpers

A helper that takes a side connection is a package-level function, as `AllSessions` and `SendRequest` are: every method of
`AgentSideConnection` and `ClientSideConnection` that takes a context is a protocol method. A new test,
`TestSideConnectionMethodsAreProtocolMethods`, enforces it. (`0ac866b`.)

#### For maintainers

- **CI routing.** A change to the docs-safe paths alone runs a new Docs workflow, `docs.yml`, in place of full CI; any other
  change, a `v*` tag and the schedule run CI. Both run `make markdownlint`, `make check-records` and the new portable
  `make disclosure-check`, and `scripts/check_ci_filters.py` fails when their paths, gates or pins disagree
  ([0037-MADR-align-docs-ci-and-disclosure-gates.md](../decisions/0037-MADR-align-docs-ci-and-disclosure-gates.md)).
  (`c1b86d1`, `abb59b2`, `5f0cb2b`.)
- **Tools.** `tools/consumer.py` builds a module at the Go floor that uses `ClientRefusals` and `Kind` (`9fc22d9`), and now
  `AgentRefusals`, `IsMethodNotFound`, `PickPermissionOption` and `ContentBlock.Kind` too.
- **Tests.** New at the root: `consumer_helpers_test.go`, `errors_classify_test.go`, `refused_messages_test.go` and
  `side_methods_test.go`; `kind_test.go` and `refusals_test.go` extended.

## Sources not in the release text

- **Commits since `v1.21.0`** (`git log v1.21.0..df0462e`), twenty:
  - `0ac866b`, `79038ac`, `b4ca784`, `cb1a763` and `b039e6b` change the package, as above; `df0462e` changes `README.md`;
  - `c1b86d1` and `abb59b2` change `.github/workflows/`, the `Makefile` and `scripts/`, and `5f0cb2b` `AGENTS.md`;
  - `9fc22d9` changes `tools/consumer.py` and `tools/README.md`;
  - the other ten change records only (`git show --name-only`).
- **Go files changed.** `git diff --name-only v1.21.0 df0462e -- '*.go' ':!*_test.go'` lists four: `connection.go`, `errors.go`,
  `helpers.go` and `refusals.go`. No `*_gen.go` changes. `go.mod`, `go.sum`, `cmd/generate/go.mod`, the schema, `RELEASING.md`
  and the examples are unchanged (`git diff`).
- **`make api` on `df0462e`:** exit 0, "against v1.21.0: 0 incompatible, 22 compatible": `(*Connection).RefusedMessages`,
  `(*RequestError).Reason`, `AgentRefusals`, `AllSessions`, `ChainRewrites`, `ConfigOptionByCategory`, `ContentBlock.Kind`,
  `DecodeMeta`, `ErrRepeatedCursor`, `IsAuthRequired`, `IsCancelled`, `IsMethodNotFound`, `MetaObject`, `OfferedAuthMethods`,
  `PickPermissionOption`, `RequestPermissionOutcome.Kind`, `RequestPermissionResponse.Chosen`, `ResolveStdioCommands`,
  `SessionConfigOption.Kind`, `SessionConfigSelectOptions.All`, `SessionInfo.Updated` and `ToolCallContent.Kind`, each "added".
  `apidiff` lists `AgentRefusals` once, without its methods.
- **CI on `df0462e`:** run `40210644204`, "completed success", six jobs on Ubuntu, Windows and macOS at Go `1.27.3` and
  `1.26.10`; and the Docs workflow's run `40210644229`, "completed success".
- **The evidence:** the 0038 PLAN's steps 1 to 7, with their 26 planted breaks, the gates on macOS and on the Linux and Windows
  hosts, and the deviation of step 2, `AllSessions` as a function; and
  [0037-PLAN-align-docs-ci-and-disclosure-gates.md](../decisions/0037-PLAN-align-docs-ci-and-disclosure-gates.md) for CI.
- **Not in this release:** [0039-MADR-five-adopter-additions-on-schema-1-25-0.md](../decisions/0039-MADR-five-adopter-additions-on-schema-1-25-0.md),
  accepted, whose PLAN waits for this one.
