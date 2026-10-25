---
status: in-progress
date: 2026-10-25
associated-madr: "0038-MADR-consumer-helpers-kind-and-agent-refusals.md"
---

# Implement 0038: the consumers' protocol helpers, Kind() on four more unions, and AgentRefusals

Associated MADR: [0038-MADR-consumer-helpers-kind-and-agent-refusals.md](0038-MADR-consumer-helpers-kind-and-agent-refusals.md)

## Goal

The MADR's "Specification, as proposed" ships, under its recommended answers to questions 1 to 4:

1. the client-side reading helpers: `AllSessions`, `SessionInfo.Updated`, `PickPermissionOption`, `RequestPermissionResponse.Chosen`,
   `MetaObject`, `DecodeMeta`, `ConfigOptionByCategory` and `SessionConfigSelectOptions.All`;
2. the error helpers: `IsMethodNotFound`, `IsAuthRequired`, `IsCancelled` and `RequestError.Reason`;
3. the agent-side helpers: `ResolveStdioCommands`, `ChainRewrites`, `OfferedAuthMethods` and `AgentRefusals`;
4. `Kind()` on `ContentBlock`, `ToolCallContent`, `RequestPermissionOutcome` and `SessionConfigOption`;
5. `Connection.RefusedMessages`.

`make api` against `v1.21.0` lists only compatible changes. On the owner's ask, the work is released as `v1.22.0`.

An answer other than the recommended one amends this PLAN before the step it changes: question 1 changes step 5, question 2 step 2,
question 3 step 6, and question 4 step 8.

## Scope

* **Source, hand-written files only, no new root file** (the MADR's R11):
  * `helpers.go`: step 2's helpers, `ResolveStdioCommands`, `OfferedAuthMethods` and step 5's `Kind()` methods;
  * `errors.go`: step 3's helpers;
  * `connection.go`: `ChainRewrites` and `RefusedMessages`;
  * `refusals.go`: `AgentRefusals`.
* **Tests:**
  * new: `consumer_helpers_test.go` (steps 2 and 4), `errors_classify_test.go` (step 3), `refused_messages_test.go` (step 6);
  * extended: `refusals_test.go` (step 4), `kind_test.go` (step 5), `example_api_test.go` (four examples).
* **Docs:** `README.md`'s helper list, client and agent sections; `docs/architecture.md`'s hand-written list.
* **Records:** this pair and their rows in `docs/README.md`; at step 8, `docs/reports/0038-REPORT-release-notes-v1-22-0.md` and its
  row.
* **Not in scope:**
  * any generated file, the generator, `HandWrittenHelpers` or the schema;
  * a new hand-written root file, which `TestRuntimeFilesCoverHandWritten` would need to name; needing one is a deviation;
  * `Kind()` on any other union, the unstable ones included;
  * changing `WithInboundRewrite` to keep more than one hook;
  * the report's units 4 to 7;
  * `example/` and both consumers, which adopt the tag under their own records.

## Implementation Steps

Each step ends with its checks, an entry in the execution record below, and its files staged for the owner, who commits on `main`
(`AGENTS.md`, "Commits"). Every new check is seen to fail on a planted break before its step closes. The breaks run through
`tools/plants.py`, each in its own copy of the tree, from a list kept in the session's scratch directory.

Every step's gates:

* `make pre-add-check` on the changed `.go` files;
* `(cd cmd/generate && go test -count=1 ./...)`, which `make pre-add-check` does not run, so `TestRuntimeFilesCoverHandWritten`
  and `TestGeneratedFilesUpToDate` are seen;
* `make check-records`, `make markdownlint`, `make disclosure-check` and `make api`;
* `tools/idscan.py --paths` over the staged files.

Steps 4, 6 and 7 also run `tools/gates.py --all`, with `ACP_LINUX_HOST` and `ACP_WINDOWS_HOST` set, since they change
`connection.go` or `refusals.go`.

### 1. Records

* This pair and their rows in `docs/README.md`, both `proposed`.
* The owner answers the MADR's questions; the MADR records them in a "Decided with the owner" table and becomes `accepted`. On
  approval, this PLAN becomes `in-progress`.

Checks: `make check-records`, `make markdownlint`, `make disclosure-check`, and `tools/idscan.py --paths` over the three files.

### 2. The client-side reading helpers

In `helpers.go`, after the `SessionUpdate` helpers, with `ErrRepeatedCursor` in its own `var` and `iter` and `time` imported:
`AllSessions` (*amended 2026-10-25: a package-level function, `AllSessions(ctx, c, req)`, see this step's deviation*),
`SessionInfo.Updated`, `PickPermissionOption`, `RequestPermissionResponse.Chosen`, `MetaObject`, `DecodeMeta`,
`ConfigOptionByCategory` and `SessionConfigSelectOptions.All`, as the MADR specifies.

Tests, in `consumer_helpers_test.go`:

* **`TestAllSessions`,** over a paired agent and client whose agent advertises `sessionCapabilities.list` and answers from a table
  of pages:
  * two pages, the second without `nextCursor`: both pages' sessions, in order, and two calls;
  * a second page whose `nextCursor` is the first's: the first page's sessions, then `ErrRepeatedCursor`, and no third call;
  * an error answer to the second call: the first page, then that error, once;
  * a `break` after the first session: one call;
  * `nextCursor` of `""`: treated as absent;
  * an agent that did not advertise the capability: one yield of a `*CapabilityError`, and nothing written.
* **`TestSessionInfoUpdated`:** an RFC 3339 time, one with fractional seconds and an offset, absent, and `"yesterday"`.
* **`TestPickPermissionOption`:**
  * an allow asked of `[reject_once "Don't allow", allow_always "Always"]` gives the second;
  * kinds asked in order `allow_once, allow_always` prefer `allow_once` when both are present;
  * no match, and no kinds, give `false`;
  * the name is never read: an option named "Allow" of kind `reject_once` is not an allow.
* **`TestChosen`:** `Selected`, `Cancelled`, `Unknown` and the zero value.
* **`TestMetaObject` and `TestDecodeMeta`:** a nested object, a key holding a string, an absent key, a nil map; `DecodeMeta` into
  a struct, a mismatched type giving an error, and absent giving `false` with no error.
* **`TestConfigOptionByCategory` and `TestSelectOptionsAll`:** the first `Select` of the category, past a `Boolean` and a `Select`
  without one; ungrouped, grouped in order, and neither.
* **`ExampleClientSideConnection_AllSessions`** (*amended 2026-10-25: `ExampleAllSessions`*) and **`ExamplePickPermissionOption`,**
  in `example_api_test.go`, with `// Output:`.

Docs: `README.md`'s helper list and client section.

Planted breaks:

| Break | The check that must fail |
| :--- | :--- |
| `AllSessions` ignores `NextCursor` | `TestAllSessions`, two pages |
| `AllSessions` does not remember cursors | `TestAllSessions`, repeating cursor (the test bounds itself with a context timeout) |
| `AllSessions` keeps calling after `yield` returns false | `TestAllSessions`, break |
| `PickPermissionOption` matches `"allow"` in the name | `TestPickPermissionOption`, "Don't allow" |
| `All` returns only `Ungrouped` | `TestSelectOptionsAll`, grouped |
| `DecodeMeta` returns `true` for an absent key | `TestDecodeMeta`, absent |

### 3. The error helpers

In `errors.go`, after `NewResourceNotFound`: `IsMethodNotFound`, `IsAuthRequired`, `IsCancelled` and `RequestError.Reason`. The
codes are written as numbers, each with the `ErrorCode` constant it equals named in a comment (the MADR's R11).

Tests, in `errors_classify_test.go`:

* **`TestErrorCodesPinned`:** each number equals its constant, `int(ErrorCodeAuthenticationRequired)`,
  `int(ErrorCodeRequestCancelled)` and `int(ErrorCodeMethodNotFound)`.
* **`TestIsMethodNotFound`:** a peer's `-32601` over a paired connection; a local method-gate `*CapabilityError` from sending a
  method the peer did not advertise; each wrapped with `fmt.Errorf("%w")`; false for a value-gate `*CapabilityError` with a
  `Variant`, for `-32602`, and for nil.
* **`TestIsAuthRequired`** and **`TestIsCancelled`:** each code and cause, wrapped and not; `IsCancelled` true for
  `context.Canceled`, `ErrPromptCancelled` and `-32800`, false for `context.DeadlineExceeded`.
* **`TestReason`:** `data.error` a string, an empty string, a number, absent, `Data` nil; the message as the fallback.

Planted breaks:

| Break | The check that must fail |
| :--- | :--- |
| `IsMethodNotFound` accepts any `*CapabilityError` | `TestIsMethodNotFound`, value gate |
| `IsMethodNotFound` uses a type assertion, not `errors.As` | `TestIsMethodNotFound`, wrapped |
| the authentication code written as `-32001` | `TestErrorCodesPinned` |
| `IsCancelled` accepts `context.DeadlineExceeded` | `TestIsCancelled` |
| `Reason` returns an empty `data.error` | `TestReason`, empty string |

### 4. The agent-side helpers

* `helpers.go`: `ResolveStdioCommands` and `OfferedAuthMethods`.
* `connection.go`: `ChainRewrites`, beside `WithInboundRewrite`. It uses no ACP type, since `connection.go` is a runtime file.
* `refusals.go`: `AgentRefusals` and its five methods, after `ClientRefusals`, in the `Agent` interface's order.

Tests:

* **`TestResolveStdioCommands`,** in `consumer_helpers_test.go`, with a fake `lookPath` over a table:
  * for each of `*NewSessionRequest`, `*LoadSessionRequest`, `*ResumeSessionRequest` and `*UnstableForkSessionRequest`, a bare
    command is replaced by the fake's absolute answer, and an absolute command is not looked up;
  * an answer that is relative, and an error, leave the command as it was;
  * http and sse servers, and another method's params, are untouched; the hook returns nil throughout.
* **`TestResolveStdioCommandsOverTheWire`:** an agent with `WithInboundRewrite(ResolveStdioCommands(fake))` receives
  `session/new` with `"npx"` and sees the absolute path in its handler; the same agent, for a command the fake cannot resolve,
  answers `-32602`.
* **`TestChainRewrites`:** order, the first error stopping the rest, and no hooks.
* **`TestOfferedAuthMethods`:** `Terminal` removed without `auth.terminal` and kept with it; `Agent` and `Unknown` kept; order
  kept; the input not changed. Over the wire, an agent whose `Initialize` answers `OfferedAuthMethods(req.ClientCapabilities, all)`
  is accepted by a client without `auth.terminal`, where the unfiltered list is refused.
* **`TestAgentRefusalsAnswers`, `TestAgentRefusalsMethodSet` and `TestAgentRefusalsOverTheWire`,** in `refusals_test.go`, as
  `ClientRefusals`' three: each of the five answers `-32601` naming its method; the method set is exactly the five, and
  `AgentRefusals` alone is not an `Agent`, while `var _ Agent = refusingAgent{}` compiles for a type that embeds it and implements the
  seven ungated methods; and an agent that advertises `sessionCapabilities.list` and embeds it answers the client's `ListSessions`
  `-32601` with data naming `session/list`.
* **`ExampleResolveStdioCommands`** and **`ExampleAgentRefusals`.**

Docs: `README.md`'s agent section; `docs/architecture.md`'s hand-written list.

Planted breaks:

| Break | The check that must fail |
| :--- | :--- |
| `ResolveStdioCommands` skips `*UnstableForkSessionRequest` | `TestResolveStdioCommands`, fork |
| `ResolveStdioCommands` takes a relative `lookPath` answer | `TestResolveStdioCommands`, relative answer |
| `ChainRewrites` runs every hook after an error | `TestChainRewrites` |
| `OfferedAuthMethods` drops `Unknown` | `TestOfferedAuthMethods` |
| `Logout` answers with `AgentMethodSessionList` | `TestAgentRefusalsAnswers`, `Logout` |
| a `Prompt` method added to `AgentRefusals` | `TestAgentRefusalsMethodSet` |

### 5. `Kind()` on four unions

In `helpers.go`, after `SessionUpdate.Kind()`: `ContentBlock.Kind`, `ToolCallContent.Kind`, `RequestPermissionOutcome.Kind` and
`SessionConfigOption.Kind`. Each switches on its pointer fields in its generated `MarshalJSON`'s order, returning the value as a
named constant, as `SessionUpdate.Kind()` does; the constants beside `blockTypeText` are reused where they exist.
`unknownSessionUpdateKind` becomes `unknownKind(b json.RawMessage, key string) string`, and `SessionUpdate.Kind()` calls it with
`"sessionUpdate"`.

Tests, in `kind_test.go`:

* **`TestUnionKinds`,** for each of the four, from `schemaUnions()`: every variant, set by reflection, gives its `Value`; every
  pointer field is among the variants; an `Unknown` with a named key, a folded key, the last of two keys, and `""`, `1`, `{}`, `[]`
  and `not json` giving `"unknown"`; decoding agrees for the string keys; the zero value gives `""`.
* **`TestKindCoversKeyedStableUnions`:** for every union in `schemaUnions()` with a key, no key-less variant, no catch-all, and a
  definition in `schema/schema.json`, the type has a `Kind() string` method. It names the five today, `SessionUpdate` included, and
  fails when a schema bump adds a sixth.
* `TestSessionUpdateKind` still passes, with the helper renamed.

Planted breaks:

| Break | The check that must fail |
| :--- | :--- |
| `ContentBlock.Kind`'s `ResourceLink` case removed | `TestUnionKinds`, `ContentBlock`, naming `resource_link` |
| `RequestPermissionOutcome.Kind` reads `"type"` from `Unknown` | `TestUnionKinds`, `RequestPermissionOutcome`, named key |
| `SessionConfigOption`'s `Kind` method removed | `TestKindCoversKeyedStableUnions`, naming `SessionConfigOption` |
| `unknownKind` returns `""` for an empty key | `TestUnionKinds` and `TestSessionUpdateKind`, `""` |

### 6. `RefusedMessages`

In `connection.go`: an `atomic.Uint64` beside `droppedNotifications`, added to where `checkInbound` returns a refusal, and
`RefusedMessages`, beside `DroppedNotifications`. The exact place is read before the edit, so that a message is counted once
whichever of the rewrite hook, validation or the gate refuses it, and a held message judged again is counted when it is refused.

Tests, in `refused_messages_test.go`:

* a rewrite hook's error, a validation error the hook did not accept, and a method gate's refusal each add one;
* a validation error the hook accepted adds none;
* a malformed line and an unknown method's `-32601` add none, if `checkInbound` does not see them;
* a refused response, from `sendRequest`, adds one;
* the count is safe under `-race` with concurrent refusals.

Planted breaks:

| Break | The check that must fail |
| :--- | :--- |
| the gate's refusal not counted | the gate case |
| a refusal counted twice, at the hook and at the return | the rewrite case |
| an accepted validation error counted | the accepted case |

### 7. Close the implementation

* `make api` against `v1.21.0`: 0 incompatible, and the compatible changes listed; record what it lists.
* `tools/gates.py --all`.
* The README and architecture text read once more against the code, in full.
* This PLAN's execution record complete, with the MADR's "Confirmation" walked item by item, each with the check that meets it.

The owner commits. A push needs the owner's ask in that turn: before it, `git fetch`, `make push-guard` and
`tools/idscan.py --range origin/main..main`; after it, `tools/watch_ci.py <commit>`, which records the CI and Docs runs.

### 8. Release `v1.22.0`, only on the owner's ask in that turn

Through `RELEASING.md`, "Tag and Publish", in this order:

1. `docs/reports/0038-REPORT-release-notes-v1-22-0.md` and its row, with its "Release text" and `make api`'s output, committed
   before the tag, so the tagged tree holds it;
2. `tools/release_body.py v1.22.0 <commit>`;
3. the annotated tag and its push;
4. the GitHub release;
5. `tools/publication.py`;
6. `tools/consumer.py v1.22.0`, extended to call `PickPermissionOption`, `IsMethodNotFound` and `ContentBlock.Kind`, and to embed
   `AgentRefusals` in a type the program builds; the extension is part of this step, with `make check-tools`.

Commenting on the consumer issues needs the owner's ask too. When the release is published, this PLAN and its row are `complete`.

## Verification

* Every item of the MADR's "Confirmation" holds, each by its step's tests.
* `make api` against `v1.21.0` lists only additions.
* No generated file, no file under `cmd/generate`, and no new non-test root file in `git diff --stat` since this pair's commit;
  `TestRuntimeFilesCoverHandWritten` and `TestGeneratedFilesUpToDate` pass.
* The root's `go test ./...` and `-race`, the `cmd/generate` tests, `make pre-add-check`, and `tools/gates.py --all` at steps 4, 6
  and 7.
* `make disclosure-check` and `tools/idscan.py`: nothing found.
* Every planted break in steps 2 to 6 fails its check, each in its own copy of the tree, and the execution record keeps each one's
  failing line.

## Rollout and Rollback

* **Rollout.** Steps 2 to 6 are one commit each, and each stands without the later ones. Nothing is released before step 8, so no
  consumer sees the change before the tag. Each consumer adopts the tag under its own issue and records.
* **Rollback before the tag:** revert the step's commit.
* **Rollback after the tag:** an exported name cannot be removed under `v1`, so a defect is fixed forward in a patch release; a tag
  found broken is retracted, as [0020-MADR-retract-accidental-v1-tag.md](0020-MADR-retract-accidental-v1-tag.md) did.

## Execution record

### Step 1, records, 2026-10-25

* **The pair** was written from a read-only investigation at `ea09a07`, every finding in the MADR's R1 to R12.
* **The owner's answers:** questions 1 to 4 were put as three options each, and the owner chose the recommended one each time; the
  MADR records them in "Decided with the owner, 2026-10-25" and is `accepted`. The owner committed the pair, decided, as
  `8b6d65f`, then approved execution: "committed, proceed". This PLAN is `in-progress`.
* **Checks** on the records, each exit 0: `make check-records`, `make markdownlint`, `make disclosure-check`, and `tools/idscan.py
  --paths` over the three files, "0 match(es)".

### Step 2, the client-side reading helpers, 2026-10-25

What changed:

* **`helpers.go`:** `context`, `errors`, `iter` and `time` imported; after `unknownSessionUpdateKind`, `ErrRepeatedCursor`,
  `errNilConnection`, `AllSessions`, `SessionInfo.Updated`, `PickPermissionOption`, `RequestPermissionResponse.Chosen`,
  `MetaObject`, `DecodeMeta`, `ConfigOptionByCategory` and `SessionConfigSelectOptions.All`.
* **`consumer_helpers_test.go`, new:** `TestAllSessions` with seven subtests (two pages, repeating cursor, error on the second page,
  break, empty next cursor, not advertised, nil connection), over a paired agent that answers from a table of pages and counts the
  calls; `TestSessionInfoUpdated`, `TestPickPermissionOption`, `TestChosen`, `TestMetaObject`, `TestDecodeMeta`,
  `TestConfigOptionByCategory` and `TestSelectOptionsAll`.
* **`side_methods_test.go`, new:** `TestSideConnectionMethodsAreProtocolMethods`, the deviation's rule.
* **`example_api_test.go`:** `ExampleAllSessions`, over a `pagingAgent` that wraps `demoAgent` and answers two pages, and
  `ExamplePickPermissionOption`; `os` imported, for a `cwd` that is absolute on every host.
* **`README.md`:** the client section names `acp.AllSessions` and `acp.PickPermissionOption`, and says why a helper that takes a
  side connection is a function; the helper list gains "Reading values".

**Deviation, 2026-10-25: `AllSessions` is a function.** The first `make pre-add-check` failed, exit 2. Three findings were lint, and
were fixed: `errcheck` on a two-value type assertion in `MetaObject`, `revive` on `Updated`'s receiver name (`SessionInfo`'s other
method names it `r`), and `thelper` on `listCtx`. The others were four tests that treat every exported method of a side connection
taking a context as a protocol method:

* `docs_test.go:133`: "AllSessions's doc does not name *RequestError";
* `docs_test.go:145`: "AllSessions's doc does not name ErrNotInitialized";
* `docs_test.go:151`: "41 typed and 4 extension methods, want the 40 of agent_gen.go and client_gen.go and the 4 of extensions.go";
* `gates_test.go:303`: "*acp.ClientSideConnection.AllSessions = <nil>, want an error wrapping ErrNotInitialized";
* `concurrent_use_test.go:127`: "8200 typed calls, want the 40 typed methods 200 times".

None failed before this step. Two resolutions were put to the owner: a package-level function, keeping the four tests; or keeping
the method and narrowing the tests to methods whose last result is an `error`, which would stop them seeing such a helper. The
owner chose "option 1, make it robust". `AllSessions` is `AllSessions(ctx, c, req)`, as `SendRequest[T]` is; a nil `c` yields
`errNilConnection`; and `TestSideConnectionMethodsAreProtocolMethods` states the rule, so a later helper fails with that message
rather than a miscount. The MADR has "Amendment, 2026-10-25"; this step's text is annotated; the rule is carried into
[0039-MADR-five-adopter-additions-on-schema-1-25-0.md](0039-MADR-five-adopter-additions-on-schema-1-25-0.md) and its PLAN for
`Present*`.

**Tests,** `go test -count=1 -race`, exit 0: the step's tests, the two examples, `TestSideConnectionMethodsAreProtocolMethods`,
`TestTypedMethodDocsNameErrors`, `TestNotInitialized`, `TestConcurrentUse`, `TestExportedDeclarationsDocumented` and
`TestExportedTypeDocsStartWithName`.

**Planted breaks,** `tools/plants.py`, each in its own copy of the tree, the list in the session's scratch directory; eight of eight
failed as required:

| Break | The check that failed |
| :--- | :--- |
| `AllSessions` ignores `NextCursor` | "consumer_helpers_test.go:99: ids [a b], errors [], 1 calls; want [a b c], none, 2" |
| `AllSessions` does not remember cursors | `TestAllSessions/repeating_cursor`, after its 5 s context ended, with the second page's session repeated |
| `AllSessions` keeps going after `yield` returns false | `TestAllSessions/break`: "panic: runtime error: range function continued iteration after function for loop body returned false" |
| `PickPermissionOption` also matches an option named "Allow" | "consumer_helpers_test.go:212: name never read: \"trap\", true; want \"\", false" |
| `All` returns only `Ungrouped` | "consumer_helpers_test.go:314: grouped: []" |
| `DecodeMeta` returns `true` for an absent key | "consumer_helpers_test.go:270: absent: true, <nil>; want false, nil" |
| a context-taking method `Pages` added to `ClientSideConnection` in `helpers.go` | "side_methods_test.go:42: helpers.go:346:1: ClientSideConnection.Pages takes a context but is declared in helpers.go, not in a generated file or extensions.go; …" |
| `AllSessions` without its nil check | `TestAllSessions/nil_connection`: "panic: runtime error: invalid memory address or nil pointer dereference" |

**Gates,** each exit 0:

* `make pre-add-check`: "180 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck)";
* `(cd cmd/generate && go test -count=1 ./...)`: every package `ok`;
* `make api`: "against v1.21.0: 0 incompatible, 9 compatible", the nine names above, `errNilConnection` being unexported;
* `make check-records`, `make markdownlint`, `make disclosure-check`, and `tools/idscan.py --paths` over the staged files.

Step 1 was committed by the owner as `15e89f6`. Step 2's files are staged for the owner, with the 0039 MADR's and PLAN's
amendments for the same rule.

### Step 3, the error helpers, 2026-10-25

What changed:

* **`errors.go`:** after `NewResourceNotFound`, three unexported constants, `codeMethodNotFound`, `codeAuthenticationRequired` and
  `codeRequestCancelled`, each with the `ErrorCode` constant it equals named in a comment, since `errors.go` is compiled into the
  generator's synthetic cases; then `IsMethodNotFound`, `IsAuthRequired`, `IsCancelled` and `(*RequestError).Reason`, which reads
  `data[jsonErrorField]`, the key the SDK's own refusals use. Each looks through wrapping with `errors.AsType` and `errors.Is`,
  and `Reason` and the predicates accept a nil `*RequestError`.
* **`errors_classify_test.go`, new:** `TestErrorCodesPinned`; `TestIsMethodNotFound`, with a peer's `-32601` over a raw `Connection`
  pair, a local method gate from `ListSessions` on an unadvertised record, a value gate from sending `UpdateNotice` (checked to
  carry a `Variant`), each wrapped too; `TestIsAuthRequired`, a peer's `-32000` included; `TestIsCancelled`; `TestReason`.
* **`README.md`:** "Errors on the wire" names the four helpers.

The step lists no docs item; the README line follows the MADR's "Documentation", which names every new helper.

**Tests,** `go test -count=1 -race`, exit 0: the five tests, `TestExportedDeclarationsDocumented`,
`TestExportedTypeDocsStartWithName` and `TestSideConnectionMethodsAreProtocolMethods`.

**Planted breaks,** `tools/plants.py`, each in its own copy of the tree; five of five failed as required:

| Break | The check that failed |
| :--- | :--- |
| `IsMethodNotFound` accepts any `*CapabilityError` | "errors_classify_test.go:73: value gate: IsMethodNotFound(session/update: capability session.notices not advertised: variant notice) = true, want false" |
| `IsMethodNotFound` uses a type assertion, not `errors.AsType` | "errors_classify_test.go:73: peer's -32601, wrapped: IsMethodNotFound(listing: {\"code\":-32601,…}) = false, want true" |
| the authentication code written as `-32001` | "errors_classify_test.go:27: authentication required: -32001, want -32000" |
| `IsCancelled` accepts `context.DeadlineExceeded` | "errors_classify_test.go:117: context.DeadlineExceeded: IsCancelled(context deadline exceeded) = true, want false" |
| `Reason` returns an empty `data.error` | "errors_classify_test.go:139: empty data.error: Reason() = \"\", want \"Invalid params\"" |

**Gates,** each exit 0:

* `make pre-add-check`: "182 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck)";
* `(cd cmd/generate && go test -count=1 ./...)`: every package `ok`, so the synthetic cases compile with the new `errors.go`;
* `make api`: "against v1.21.0: 0 incompatible, 13 compatible", step 2's nine and `IsAuthRequired`, `IsCancelled`,
  `IsMethodNotFound` and `(*RequestError).Reason`, each "added";
* `make check-records`, `make markdownlint`, `make disclosure-check`, and `tools/idscan.py --paths` over the step's files.

Step 2 was committed by the owner as `0ac866b`. Step 3's files are staged for the owner.

### Step 4, the agent-side helpers, 2026-10-25

What changed:

* **`helpers.go`:** `path/filepath` imported; after `SessionConfigSelectOptions.All`, `ResolveStdioCommands` and
  `OfferedAuthMethods`. `ResolveStdioCommands` switches on `*NewSessionRequest`, `*LoadSessionRequest`,
  `*ResumeSessionRequest` and `*UnstableForkSessionRequest`; it looks up a command only when `filepath.IsAbs` refuses it, and
  keeps an answer only when `filepath.IsAbs` accepts it.
* **`connection.go`:** `ChainRewrites`, after `WithInboundRewrite`, with no ACP type.
* **`refusals.go`:** `AgentRefusals` and its five methods, `CloseSession`, `DeleteSession`, `ListSessions`, `ResumeSession` and
  `Logout`, with the doc comment the MADR requires.
* **`consumer_helpers_test.go`:** `TestResolveStdioCommands`, over all four requests with a counting fake `lookPath`;
  `TestResolveStdioCommandsOverTheWire`, which sends `session/new` with `"npx"` through the untyped `Connection`, as a client in
  another language would, since this SDK's own client refuses to send a relative command; `TestChainRewrites`;
  `TestOfferedAuthMethods`, whose wire half first checks that the unfiltered list is refused, so that the filtered one passing
  proves something.
* **`refusals_test.go`:** `refusingAgent`, with `var _ Agent = refusingAgent{}`; `TestAgentRefusalsAnswers`,
  `TestAgentRefusalsMethodSet` and `TestAgentRefusalsOverTheWire`.
* **`example_api_test.go`:** `ExampleResolveStdioCommands`, whose output, `true npx`, is the same on every host, and
  `ExampleAgentRefusals` with `promptOnlyAgent`.
* **`README.md`:** the agent section names `AgentRefusals`, `ResolveStdioCommands`, `ChainRewrites` and `OfferedAuthMethods`.
* **`docs/architecture.md`:** the hand-written list says what `connection.go`, `errors.go`, `helpers.go` and `refusals.go` hold
  after steps 2 to 4, and states the rule of step 2's deviation.

The first `go vet` failed: `initializeWith`, the step's helper, was already declared in `initialize_test.go`. It is
`initializeOffering`.

**Tests,** `go test -count=1 -race`, exit 0: the seven tests, the two examples, `TestExportedDeclarationsDocumented`,
`TestExportedTypeDocsStartWithName` and `TestSideConnectionMethodsAreProtocolMethods`.

**Planted breaks,** `tools/plants.py`, each in its own copy of the tree; six of six failed as required:

| Break | The check that failed |
| :--- | :--- |
| `ResolveStdioCommands` skips `*UnstableForkSessionRequest` | "consumer_helpers_test.go:374: fork: command \"npx\", want \"`<temp>`/bin/npx\"" |
| `ResolveStdioCommands` takes a relative `lookPath` answer | "consumer_helpers_test.go:366: load server 2: command \"bin/rel\", want \"rel\"" |
| `ChainRewrites` runs every hook after an error | "consumer_helpers_test.go:440: first error: <nil>, [1 2]; want stop, [1]" |
| `OfferedAuthMethods` drops `Unknown` | "consumer_helpers_test.go:470: without auth.terminal: [agent]" |
| `Logout` answers with `AgentMethodSessionList` | "refusals_test.go:166: Logout: {\"code\":-32601,…,\"data\":{\"method\":\"session/list\"}}, want -32601 naming logout" |
| a `Prompt` method added to `AgentRefusals` | "refusals_test.go:181: methods = [CloseSession DeleteSession ListSessions Logout Prompt ResumeSession], want [CloseSession DeleteSession ListSessions Logout ResumeSession]" |

`<temp>` stands for the host's temporary directory, which the scratch copy's failure line printed in full.

**Gates,** each exit 0:

* `make pre-add-check`: "183 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck)";
* `(cd cmd/generate && go test -count=1 ./...)`: every package `ok`, so the synthetic cases compile with the new `connection.go`;
* `make api`: "against v1.21.0: 0 incompatible, 17 compatible", steps 2's and 3's thirteen and `AgentRefusals`, `ChainRewrites`,
  `OfferedAuthMethods` and `ResolveStdioCommands`; `apidiff` lists the type once, without its methods;
* `tools/gates.py --all`, with `ACP_LINUX_HOST` and `ACP_WINDOWS_HOST` set: "all gates passed", 18 of 18, `floor`, `nojsonv2`,
  `windows` and `linux` among them, on the working tree of steps 2 to 4;
* `make check-records`, `make markdownlint`, `make disclosure-check`, and `tools/idscan.py --paths` over the step's files.

Step 3 was committed by the owner as `79038ac`. Step 4's files are staged for the owner.

### Step 5, `Kind()` on four unions, 2026-10-25

What changed:

* **`helpers.go`:** four constants beside `rawInputContentKey`, `outcomeCancelled`, `outcomeSelected`, `configOptionSelect` and
  `configOptionBoolean`; `unknownSessionUpdateKind` became `unknownKind(b, key)`, and `SessionUpdate.Kind()` calls it with
  `"sessionUpdate"`; after it, `ContentBlock.Kind`, `ToolCallContent.Kind`, `RequestPermissionOutcome.Kind` and
  `SessionConfigOption.Kind`. Each switches on its pointer fields in its generated `MarshalJSON`'s order, read from
  `types_gen.go` before the edit, which is the struct's field order for all four; the block and tool-content values reuse
  `blockTypeText` to `blockTypeTerminal`.
* **`kind_test.go`:** `TestUnionKinds`, a subtest per union driven by `schemaUnions()` through reflection: every variant, every
  pointer field among them, an `Unknown` with a named, folded, last-of-two, empty, numeric, empty-object, array and non-JSON key,
  decoding agreeing for the four string keys, the empty one included, and the zero value; and `TestKindCoversKeyedStableUnions`,
  which finds every union in `schema/schema.json` with a key, no key-less variant and no catch-all, requires a `Kind() string` on
  each, and pins the set as the five that have one today.
* **`README.md`:** the "Session updates" item names the four.

The first run left the empty key's decoding unasserted; asserting it, it agreed for all four unions, and the case stays in.

**Tests,** `go test -count=1 -race`, exit 0: `TestUnionKinds` and its four subtests, `TestKindCoversKeyedStableUnions`, and
`TestSessionUpdateKind` with the renamed helper.

**Planted breaks,** `tools/plants.py`, each in its own copy of the tree; four of four failed as required:

| Break | The check that failed |
| :--- | :--- |
| `ContentBlock.Kind`'s `ResourceLink` case removed | "kind_test.go:177: ContentBlock.ResourceLink: Kind() = \"\", want \"resource_link\"" |
| `RequestPermissionOutcome.Kind` reads `"type"` from `Unknown` | "kind_test.go:178: RequestPermissionOutcome {\"outcome\":\"future\"}: Kind() = \"unknown\", want \"future\"" |
| `SessionConfigOption`'s `Kind` method made a function | "kind_test.go:276: SessionConfigOption: every variant carries \"type\" and it is stable, but it has no Kind() string" |
| `unknownKind` returns an empty key | "kind_test.go:123: {\"sessionUpdate\":\"\"}: decoded Kind() = \"\", want \"unknown\"" |

**Gates,** each exit 0:

* `make pre-add-check`: "183 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck)", before and after the empty-key
  case was asserted;
* `(cd cmd/generate && go test -count=1 ./...)`: every package `ok`;
* `make api`: "against v1.21.0: 0 incompatible, 21 compatible", the seventeen before and the four `Kind` methods;
* `make check-records`, `make markdownlint`, `make disclosure-check`, and `tools/idscan.py --paths` over the step's files.

Step 4 was committed by the owner as `b4ca784`. Step 5's files are staged for the owner.

### Step 6, `RefusedMessages`, 2026-10-25

Before the edit, every caller of `checkInbound` was read: 28 in `agent_gen.go` and 12 in `client_gen.go`, one per dispatch case;
`sendRequestSent` and `sendRequestCommit` in `connection.go`, one each, on the two paths a result takes; and `handleExtension` in
`extensions.go`. Each calls it once for a message, and a held message is judged inside the same call, by `heldGate`.

What changed:

* **`connection.go`:** `refusedMessages atomic.Uint64` beside `droppedNotifications`; `checkInbound` now calls
  `judgeInbound`, its former body unchanged, and adds one when it returns an error; `RefusedMessages()` beside
  `DroppedNotifications()`. Counting at the one return keeps the count to one a message, whichever of the rewrite hook,
  validation and the gate refused it.
* **`refused_messages_test.go`, new:** `TestRefusedMessages`, over `rawWire`: a rewrite hook's error, a validation error, a gate's
  refusal, a refused `_` notification and a refused response each count one; a validation error the hook accepted, a line that
  is not JSON, and an unknown method count none; fifty refusals sent at once count fifty, under `-race`.
* **`README.md`:** "the order" of the inbound checks says what `RefusedMessages()` counts, and the rewrite-hook passage names
  `ResolveStdioCommands` before its hand-written example.

The first `make pre-add-check` failed, exit 2, on `perfsprint`: "fmt.Sprint can be replaced with faster strconv.Itoa". It uses
`strconv.Itoa`.

**Tests,** `go test -count=1 -race -run '^TestRefusedMessages$'`, exit 0: the eight subtests.

**Planted breaks,** `tools/plants.py`, each in its own copy of the tree; three of three failed as required:

| Break | The check that failed |
| :--- | :--- |
| a gate's refusal not counted | "refused_messages_test.go:113: gate: RefusedMessages() = 0, want 1" |
| a rewrite refusal counted at the hook as well as at the return | "refused_messages_test.go:80: rewrite: RefusedMessages() = 2, want 1" |
| a validation error counted before the hook accepts it | "refused_messages_test.go:102: accepted: RefusedMessages() = 1, want 0" |

**Gates,** each exit 0:

* `make pre-add-check`: "183 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck)";
* `(cd cmd/generate && go test -count=1 ./...)`: every package `ok`, so the synthetic cases compile with the new `connection.go`;
* `make api`: "against v1.21.0: 0 incompatible, 22 compatible", the twenty-one before and `(*Connection).RefusedMessages`;
* `tools/gates.py --all`, with `ACP_LINUX_HOST` and `ACP_WINDOWS_HOST` set: "all gates passed", 18 of 18;
* `make check-records`, `make markdownlint`, `make disclosure-check`, and `tools/idscan.py --paths` over the step's files.

Step 5 was committed by the owner as `cb1a763`. Step 6's files are staged for the owner.

### Step 7, close the implementation, 2026-10-25

On the tree of steps 2 to 6, with step 6 staged:

* **`make api`:** exit 0, "against v1.21.0: 0 incompatible, 22 compatible": `(*Connection).RefusedMessages`,
  `(*RequestError).Reason`, `AgentRefusals`, `AllSessions`, `ChainRewrites`, `ConfigOptionByCategory`, `ContentBlock.Kind`,
  `DecodeMeta`, `ErrRepeatedCursor`, `IsAuthRequired`, `IsCancelled`, `IsMethodNotFound`, `MetaObject`, `OfferedAuthMethods`,
  `PickPermissionOption`, `RequestPermissionOutcome.Kind`, `RequestPermissionResponse.Chosen`, `ResolveStdioCommands`,
  `SessionConfigOption.Kind`, `SessionConfigSelectOptions.All`, `SessionInfo.Updated` and `ToolCallContent.Kind`, each
  "added". `apidiff` lists `AgentRefusals` once, without its five methods.
* **`tools/gates.py --all`,** with `ACP_LINUX_HOST` and `ACP_WINDOWS_HOST` set: "all gates passed", 18 of 18.
* **`README.md`,** read in full, all 867 lines, against the code. One fault was found and fixed: the client section's
  `AllSessions` item ran on, after a semicolon, into `PickPermissionOption`; each now has its own item. `docs/architecture.md`'s
  hand-written list, the only part of it this PLAN changed, was read against the four files it describes.
* **The README's statements checked against the code:** `WithInboundRewrite(ResolveStdioCommands(exec.LookPath))` type-checks,
  since `exec.LookPath` is a `func(string) (string, error)`; "the SDK would refuse to send" a terminal auth method is the
  agent's `-32603` that "Initialization and capabilities" describes; `RefusedMessages` counts at steps 2 to 4 of the inbound
  order and at the gate, and not a message that never decodes, as `TestRefusedMessages` shows.

The MADR's "Confirmation", item by item:

| Item | Met by |
| :--- | :--- |
| `AllSessions` over two pages, a repeating cursor, an error on the second page, a break, and an unadvertised agent | `TestAllSessions`: `two_pages`, `repeating_cursor`, `error_on_second_page`, `break`, `not_advertised`, and `empty_next_cursor` and `nil_connection` besides |
| `PickPermissionOption` refusing an option named "Don't allow" of kind `reject_once` when asked for an allow | `TestPickPermissionOption`, "don't allow is not an allow" and "name never read" |
| each `Is*` through a wrapped error, and `IsMethodNotFound` false for a value gate | `TestIsMethodNotFound`, `TestIsAuthRequired`, `TestIsCancelled`, each with wrapped cases; the value gate from a real send of `UpdateNotice` |
| `ResolveStdioCommands` over the four requests with a fake `lookPath`, and the SDK still refusing what it leaves | `TestResolveStdioCommands`; `TestResolveStdioCommandsOverTheWire`, `-32602` for an unresolvable command |
| `OfferedAuthMethods` making an answer the SDK would refuse pass | `TestOfferedAuthMethods`, whose wire half first sees the unfiltered list refused |
| `Kind()` equal to every `schemaUnions()` value for the four unions, with the `Unknown` and zero cases | `TestUnionKinds`; `TestKindCoversKeyedStableUnions` for the set |
| `AgentRefusals` with exactly five methods, not an `Agent` alone, and `-32601` over the wire | `TestAgentRefusalsAnswers`, `TestAgentRefusalsMethodSet`, `TestAgentRefusalsOverTheWire` |
| `RefusedMessages` counting a rewrite, a validation and a gate refusal, and not a malformed line | `TestRefusedMessages` |
| `make api` against `v1.21.0` listing only compatible changes | above |

Every planted break of steps 2 to 6 failed its check: 8, 5, 6, 4 and 3, 26 in all, each line in its step's table.

Not done: the push, which waits for the owner's ask, with `git fetch`, `make push-guard` and `tools/idscan.py --range
origin/main..main` before it and `tools/watch_ci.py` after; and step 8, the release, on the owner's ask. This PLAN stays
`in-progress` until the release is published.

Step 6 was committed by the owner as `b039e6b`. Before any push: `git fetch`; `make push-guard`, "9 outgoing commit(s),
ea09a07..b039e6b, main to origin", "the guard passed (exit 0)"; `tools/idscan.py --range origin/main..main`, "18 file(s), 17
pattern(s), 0 match(es)". Step 7's files are staged for the owner.
