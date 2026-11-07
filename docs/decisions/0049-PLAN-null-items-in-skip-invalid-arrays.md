---
status: proposed
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
  * the release `v1.34.1`.
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
   * `accept-skip-invalid-items.txtar`'s behaviour test gains `TestSkipInvalidItemsNull`: `{"items":[null,{"x":1}]}` decodes to
     one item, `x` 1.
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

Not started.
