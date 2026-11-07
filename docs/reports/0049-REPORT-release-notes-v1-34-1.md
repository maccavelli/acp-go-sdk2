# Release notes for v1.34.1, against v1.34.0

Observed 2026-11-07 on `7b0cb46`, for
[0049-PLAN-null-items-in-skip-invalid-arrays.md](../decisions/0049-PLAN-null-items-in-skip-invalid-arrays.md), phase 2,
step 2. This report drafts the text of the GitHub release for `v1.34.1`, as `RELEASING.md` describes it. It decides nothing.
The commit that adds it also makes `skip_invalid_null_test.go`'s session paths absolute on every host, the PLAN's deviation
2. It changes no other code, so the API it tags is `7b0cb46`'s.

Each statement names its source:

- a record;
- a commit;
- or `make api`, run on `7b0cb46` with only records uncommitted.

Summary:

- No API change: `make api` against `v1.34.0` reports "0 incompatible, 0 compatible". This is a patch, as
  [0049-MADR-remediate-the-2026-11-01-debugging-pass.md](../decisions/0049-MADR-remediate-the-2026-11-01-debugging-pass.md)'s
  Q21 (a) decided.
- One fix, G14 of that MADR, found by its fourth re-assessment, of the generator at `ff31967`. A received message is accepted
  where `v1.34.0` refused it.
- The schema, the `go` lines and the requirements are unchanged.

The section "Release text" is what goes into the GitHub release.

## Release text

### v1.34.1

The ACP schema is still `1.25.0` (`git diff v1.34.0 -- schema/` is empty), and `make api` reports no change to the exported
API against `v1.34.0`. This release fixes one defect, G14 of
[0049-MADR-remediate-the-2026-11-01-debugging-pass.md](../decisions/0049-MADR-remediate-the-2026-11-01-debugging-pass.md):
a `null` item in certain arrays made the SDK refuse a valid message.

#### Fixed: a `null` item in a skip-invalid array is dropped (G14)

The schema marks some arrays `x-deserialize-skip-invalid-items`: an item that fails to decode is dropped, and the rest are
kept. A `null` item was not dropped. `encoding/json` decodes `null` into a string or a union as no change, so the item was
kept as its zero value. That value then failed validation, which runs on every received message, and the whole message was
refused. (`7b0cb46`.)

- **What a peer saw:**
  - a `session/update` whose tool call `content` held a `null` was answered `-32602`, and the client's handler never ran;
  - an `initialize` answer whose `authMethods` held a `null`, or a `session/list` answer whose `additionalDirectories` held
    one, made `Initialize` or `ListSessions` return an error.
- **Now** the `null` item is dropped, as an item of the wrong type already was, and the message is accepted.
- **Where:** every skip-invalid array, such as a tool call's `content` and `locations`, `authMethods`, `configOptions`,
  `availableModes`, a plan's `entries`, `availableCommands` and `additionalDirectories`. The patch fields that skip invalid
  items, a compaction's `summary` and a session message's `content`, are covered too.
- **Sending is unchanged.**

#### For maintainers

- **The generator** now refuses a skip-invalid array whose items admit `null`, naming its path, since the helper drops
  `null`. No committed schema has one. (`7b0cb46`.)
- **Tests.** `TestSkipInvalidItemsDropNull` covers five decodes. `TestSkipInvalidItemsNullReceived` covers a received
  `session/update` and a `session/list` answer. The engine cases are `accept-skip-invalid-items` and the new
  `refuse-skip-invalid-nullable-items`. Each failed with the fix removed in a copy of the tree, as the PLAN's phase 1 records.
- **The generator's other findings,** G1 to G13 and G15 to G36 of the same MADR, change no generated code for the committed
  schemas. They are the generator PLAN's.

## Sources not in the release text

- **Commits since `v1.34.0`** (`git log v1.34.0..7b0cb46`), four, and the commit that adds this report:
  - `899dec8` adds the `v1.34.0` notes and records CI;
  - `ff31967` marks the 0050 PLAN complete and records issue #14's reply;
  - `f31f487` records the fourth re-assessment, G14 to G36, Q21 and this PLAN;
  - `7b0cb46`, phase 1: `cmd/generate/internal/emit/decode.go`, `cmd/generate/internal/model/schema.go`, two engine cases,
    `types_gen.go`, `skip_invalid_null_test.go` (new), `README.md` and `docs/architecture.md`;
  - the commit that adds this report: these notes, the MADR's G14 marked done, the PLAN's phase 2 record, and
    `skip_invalid_null_test.go`'s paths built with `testAbsPath`, since `"/a"` is not absolute on Windows.
- **Go files changed** (`git diff --name-status v1.34.0 7b0cb46 -- '*.go'`): four. They are `types_gen.go`,
  `skip_invalid_null_test.go`, new, and the generator's `emit/decode.go` and `model/schema.go`.
- **Unchanged** (`git diff --stat v1.34.0 7b0cb46`, empty for these): `go.mod`, `go.sum`, `cmd/generate/go.mod`,
  `cmd/generate/go.sum`, the schema, `.github/`, `tools/`, `internal/`, `unstable/`, `remote/`, `agentproc/`, `clienthost/`,
  `acptest/` and `RELEASING.md`.
- **`make api` on `7b0cb46`,** with only records uncommitted: "check_api: against v1.34.0: 0 incompatible, 0 compatible";
  "reported packages unstable, remote: 0 incompatible, not gated". The PLAN's step 1.7 had the same.
- **The diff of `types_gen.go`:** `skipInvalid` alone, its comment and the three lines that skip a `null` item.
