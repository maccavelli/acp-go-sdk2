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
