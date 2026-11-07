---
status: in-progress
date: 2026-11-07
associated-madr: "0049-MADR-remediate-the-2026-11-01-debugging-pass.md"
---

# Implement 0049, G14: drop a `null` item from a skip-invalid array, in a patch `v1.34.1`

Associated MADR: [0049-MADR-remediate-the-2026-11-01-debugging-pass.md](0049-MADR-remediate-the-2026-11-01-debugging-pass.md),
G14, and Q21 (a), answered by the owner 2026-11-07.

## Goal

A `null` item in an array the schema marks `x-deserialize-skip-invalid-items` is dropped on decode, as an item of the wrong
type already is, and the received message is no longer refused. The fix ships alone, as the patch `v1.34.1`, ahead of the
generator PLAN.

## Scope

* **In:**
  * the generated helper `skipInvalid`, which `patchItems` also calls;
  * a refusal, in the generator, of a skip-invalid array whose items admit `null`;
  * an engine case for each;
  * a root test of the committed paths;
  * the regeneration;
  * the release `v1.34.1`;
  * *added 2026-11-07, deviation 1:* the README's and `docs/architecture.md`'s sentences on skip-invalid arrays.
* **Out:**
  * G15 to G36 and G1 to G13, which stay the generator PLAN's;
  * any change to `default-on-error`, which G24 covers.
* **Facts it rests on,** read at `ff31967`:
  * `skipInvalid` keeps an item when `json.Unmarshal(it, &v)` returns nil (`cmd/generate/internal/emit/decode.go:423-443`).
    For `null`, that is the zero value.
  * `judgeInbound` runs `validate` on every received message (`connection.go:3176-3180`), so the zero item refuses it.
  * The committed schemas have 19 stable and 24 unstable skip-invalid arrays. The items of none of them admit `null`: their
    items are `$ref`s to objects, unions and closed enums, or plain `string`. That was read by a script over
    `schema/schema.json` and `schema/schema.unstable.json`, following each `$ref`. So dropping `null` is right for every one,
    and the refusal of step 1.2 refuses none of them.
* **Exported API:** none changes. `make api` against `v1.34.0` is to report "0 incompatible, 0 compatible".

## Implementation Steps

### Phase 1: the fix, the tests and the regeneration

1. **The helper,** `emitSkipInvalid` in `cmd/generate/internal/emit/decode.go`. Before decoding an item, it skips one whose
   raw value is `null`. It compares `string(it) == "null"`, as `patchItems` compares its whole value (`decode.go:358`). The
   emitted comment says that a `null` item is dropped.
2. **The refusal,** in the model's schema walk, beside the check that skip-invalid-items is on an array
   (`cmd/generate/internal/model/schema.go:361-363`). It refuses `x-deserialize-skip-invalid-items` whose `items` admit
   `null`, naming the path. "Admit `null`" means a `type` of `null` or one that includes it, a `null` member of an
   `anyOf`/`oneOf`, or a `$ref` to a definition that admits it, followed with a visited set. `includesNull`
   (`model/fields.go:365`) reads a type array only, so it is not enough alone.
3. **The engine cases:**
   * `accept-skip-invalid-items.txtar`'s behaviour test gains `TestSkipInvalidItemsNull`: ~~`{"items":[null,{"x":1}]}`
     decodes to one item, `x` 1~~. *Amended 2026-11-07, deviation 1:* that input passed with the fix removed, since
     `Item`'s required `x` refuses `null` whatever the helper does. The case's `List` gains `names`, a skip-invalid array of
     strings, and `{"names":[null,"a"]}` decodes to `["a"]`.
   * `refuse-skip-invalid-nullable-items.txtar`, new: items of `{"type":["object","null"]}` and, in a second definition, a
     `$ref` to a union with a `null` member. Its `want.err` names the first path.
   * `cd cmd/generate && go test . -run '^TestEngineCases$/^accept-skip-invalid-items$' -update` refreshes `want.api` if the
     helper's text changes it.
4. **Regenerate:** `cd cmd/generate && go run .`. The only change to `types_gen.go` is `skipInvalid` and its comment.
5. **The root test,** `skip_invalid_null_test.go`, `TestSkipInvalidItemsDropNull`. Each case decodes the message, then
   requires the items to be as listed and `Validate` to be nil:
   * a `SessionNotification` whose `tool_call` `content` is `[null, <a content block>]`: one item;
   * a `tool_call_update` whose `content` is `[null, <a content block>]`, through `patchItems`: a `Patch` holding one item;
   * an `InitializeResponse` whose `authMethods` is `[null, <an agent method>]`: one method;
   * a `ListSessionsResponse` whose session has `additionalDirectories` `[null, "/a"]`: `["/a"]`;
   * a `SessionNotification` whose `tool_call` `content` is `[null]`: no items.

   And `TestSkipInvalidItemsNullReceived`: a client over a connection pair receives that `session/update`, and its handler
   runs with one item, where it was refused before.
6. **Seen to fail,** in scratch copies, never in the tree:
   * the helper's skip removed and the copy regenerated: `TestSkipInvalidItemsDropNull`, `TestSkipInvalidItemsNullReceived`
     and the engine case's `TestSkipInvalidItemsNull` fail;
   * the refusal removed: `refuse-skip-invalid-nullable-items` fails.
7. **Checks:**
   * `go test -race -count=1 -shuffle=on ./...`, in both modules;
   * `tools/gates.py regen`;
   * `make api`, "0 incompatible, 0 compatible";
   * `make pre-add-check FILES="<the phase's files>"`;
   * `make check-records`, `make markdownlint`, `make disclosure-check`.

   Then stage the files.

### Phase 2: the release `v1.34.1`

1. **The record:** the MADR's G14 is marked done, with its commit. This PLAN's execution record names the tests and the
   plants.
2. **The notes,** `docs/reports/0049-REPORT-release-notes-v1-34-1.md`. One fix, stated as a behaviour change a peer can see: a
   message that was refused is now accepted, with the `null` item dropped. They also note the generator's new refusal.
3. **The gates:** `tools/gates.py --all`, with the Linux and Windows hosts named per command.
4. **The release,** in one short hand-off:
   * the release body is built from the notes, with links made absolute at the tag and checked to exist there;
   * `make push-guard`, after a fetch;
   * the owner commits, pushes, tags `v1.34.1` and creates the release;
   * then CI on the tag, `tools/publication.py v1.34.1 <rev> <the body>`, and `tools/consumer.py v1.34.1`.
5. **Status:** this PLAN and its row are marked `complete`.

## Verification

* `TestSkipInvalidItemsDropNull`, `TestSkipInvalidItemsNullReceived` and the engine case's `TestSkipInvalidItemsNull` pass,
  and each failed with the skip removed.
* `refuse-skip-invalid-nullable-items` passes, and failed with the refusal removed.
* The diff of `types_gen.go` is `skipInvalid` alone.
* `make api` against `v1.34.0`: "0 incompatible, 0 compatible".
* `tools/gates.py --all`, 18 of 18, and CI on the head and on the tag pass. `publication.py` reports 10 of 10, and
  `consumer.py` reports "as required".

## Rollout and Rollback

* **Rollout:** a patch, `v1.34.1`. What a consumer meets: a received message with a `null` item in a skip-invalid array,
  such as a tool call's `content` or `initialize`'s `authMethods`, is accepted with that item dropped, where `v1.34.0` refused it with `-32602`, or returned an error
  from the call whose result it was. Nothing else changes.
* **Rollback:** pin `v1.34.0`. No exported API, wire format or schema changes, so reverting is a dependency change alone.

## Execution record

The owner committed this PLAN with the MADR's Q21 answer as `f31f487`, and said "committed, proceed".

### Phase 1: the fix, the tests and the regeneration, 2026-11-07

**What it did:**

* **The helper,** step 1.1: `emitSkipInvalid` (`cmd/generate/internal/emit/decode.go`) emits
  `if string(it) == "null" { continue }` before decoding each item. Its doc comment and the emitted comment say a `null`
  item is dropped, and why.
* **The refusal,** step 1.2: `walkRefs` (`cmd/generate/internal/model/schema.go`) refuses `x-deserialize-skip-invalid-items`
  whose items admit `null`. The error is "`<path>`: x-deserialize-skip-invalid-items on items that admit null, which it would
  drop". The new `admitsNull` reads a `null` type, a type array holding it (`includesNull`), the members of an
  `anyOf`/`oneOf`, and a `$ref`, each definition followed once.
* **The engine cases,** step 1.3, as amended by deviation 1:
  * `accept-skip-invalid-items.txtar`'s `List` gains `names`, a skip-invalid array of strings.
  * Its `TestSkipInvalidItemsNull` decodes `{"names":[null,"a"]}` to `["a"]`. `want.api` gains `Names []string`, refreshed
    with `-update`.
  * `refuse-skip-invalid-nullable-items.txtar` is new. Its `A` reaches `null` through a `$ref` to a union, and its `B` through
    a type array. `want.err` is `A.properties.xs: x-deserialize-skip-invalid-items on items that admit null`, the first error.
* **Regenerated,** step 1.4: the diff of `types_gen.go` is `skipInvalid` alone: its comment, and the three lines that skip a
  `null` item. No other generated file changed.
* **The root tests,** step 1.5, in `skip_invalid_null_test.go`:
  * `TestSkipInvalidItemsDropNull` has five cases: a tool call's content with a text block, a tool call's content of `null`
    alone, a compaction summary (a `Patch`, through `patchItems`), `initialize`'s `authMethods`, and a session's
    `additionalDirectories`. Each decodes, holds the items listed, and passes the exported `Validate`.
  * `TestSkipInvalidItemsNullReceived` checks two received messages. A `session/update` reaches the client's handler, through
    `handle` after `negotiate`, as `paths_test.go` does. A `session/list` result reaches the caller, through `rawPeer`.
  * The patch case is a compaction summary because `ToolCallUpdate.Content` is a plain slice. `patchItems` serves
    `SessionCompactionUpdate.Summary` and `SessionUpdateSessionMessage.Content`.
* **The documentation,** added by deviation 1:
  * The README's sentence on skip-invalid arrays adds "and its `null` items", citing G14.
  * `docs/architecture.md`'s passage on `skipInvalid` says it drops a `null` item by its raw value, and that the model refuses
    items that admit `null`.

**Deviation 1, 2026-11-07: the engine test could not fail, and the documentation.**

* **Found:** the first plant run gave "NOT AS REQUIRED: skipInvalid keeping a null item (cmd/generate -run
  ^TestEngineBehaviour$ .): exit 0". The test as step 1.3 wrote it, `{"items":[null,{"x":1}]}`, passed with the fix removed.
  `Item`'s `x` is required, so decoding `null` into an `Item` already fails, and the item was dropped before the fix.
* **Found too:** `README.md:333` and `docs/architecture.md:376` describe skip-invalid arrays as dropping the items that fail to
  decode. The PLAN did not list them.
* **Decision:** the owner chose the recommended option for each. The case gains `names`, a skip-invalid array of strings,
  since `null` decodes into a string as no change. Both documents are updated in this phase.
* **Scope:** `README.md` and `docs/architecture.md` join phase 1's files.
* **The MADR:** not amended. No decision and no asserted fact changes.

**Seen to fail,** step 1.6. A scratch script copied the tree, planted, regenerated where a plant needs it, and ran the
tests there, 3 of 3:

```text
FAILED AS REQUIRED: skipInvalid keeping a null item (. -run ^TestSkipInvalidItems(DropNull|NullReceived)$ .): exit 1: skip_invalid_null_test.go:96: a tool call's content: Validate = SessionNotification: update: SessionUpdate: variant ToolCall: SessionUpdateToolCall: content[0]: ToolCallContent must have exactly one variant set, want nil
FAILED AS REQUIRED: skipInvalid keeping a null item (cmd/generate -run ^TestEngineBehaviour$ .): exit 1: === RUN   TestSkipInvalidItemsNull
FAILED AS REQUIRED: the refusal of nullable skip-invalid items removed (cmd/generate -run ^TestEngineCases$/^refuse-skip-invalid-nullable-items$ .): exit 1: --- FAIL: TestEngineCases/refuse-skip-invalid-nullable-items (0.92s)
```

In the first plant's log:

* the engine test fails with `names = ["" "a"], want a alone`;
* all five root cases fail, the compaction summary's with "summary[0]: ContentBlock must have exactly one variant set";
* `TestSkipInvalidItemsNullReceived` fails three ways:
  * the `session/update` "answered {"code":-32602,…}";
  * "the handler's content = [], want the text block alone";
  * "ListSessions with a null directory: session/list response: ListSessionsResponse: sessions[0]: SessionInfo:
    additionalDirectories[0]: "" is not an absolute path, want accepted".

**Checks,** step 1.7:

* `tools/gates.py regen`: "all gates passed".
* `make api`: "check_api: against v1.34.0: 0 incompatible, 0 compatible".
* `make pre-add-check` with the phase's seven files, the docs-safe `docs/architecture.md` aside: exit 0. Its output: "4
  file(s) gofmt-clean; both modules linted, vetted and tested (-race -shuffle=on, uncached), tidy, govulncheck." That run
  covers the race step of both modules.
* `make check-records`, `make markdownlint` and `make disclosure-check`: exit 0 each.

One deviation, recorded above. Staged for the owner, who commits. Phase 2, the release, begins on the owner's word.
