---
status: accepted
date: 2026-10-10
decision-makers: Repository owner
---

# Fix the generated output on encoding/json v1

## Context and Problem Statement

[0006-MADR-emit-go-with-go-ast.md](0006-MADR-emit-go-with-go-ast.md) replaces Jennifer with `go/ast` and requires the generated files to stay byte-identical.
That rule keeps every defect in the current output. [0007-MADR-self-contained-acp-sdk.md](0007-MADR-self-contained-acp-sdk.md), as amended on 2026-10-09, keeps the SDK on `encoding/json` v1 for good.
Any fix therefore has to work through v1 `MarshalJSON` and `UnmarshalJSON` methods.

[0008-REPORT-self-contained-sdk-review.md](../reports/0008-REPORT-self-contained-sdk-review.md) found the defects below on 2026-10-09, with the schema stamp `1.25.0` and Go 1.27.2.

Two change what goes over the wire:

* A union with no variant set encodes to zero bytes. All 28 generated union types end `MarshalJSON` with `return []byte{}, nil`, and `encoding/json` then fails with
  "json: error calling MarshalJSON for type *acp.ContentBlock: unexpected end of JSON input". 57 of the 322 generated struct types fail to encode their zero value this way,
  including `RequestPermissionResponse`, `SessionNotification`, and `ContentBlock`. The error does not name the field that was left unset.
* A required array or map left nil is sent as `null`. ~~35 such fields in 34 structs carry no `omitempty`.~~ 34 such fields in 33 structs (corrected 2026-10-10, see the amendment below). A `NewSessionRequest` with `Cwd` set and `McpServers` nil sends `"mcpServers":null`, although the
  schema makes `mcpServers` a required array. The receiving side of this SDK rejects it, because the generated `Validate` reports "mcpServers is required" for a nil slice.

Union-typed fields decide what an unset union means. 27 struct fields hold a union by value, and all 27 are required. The 3 optional union fields are pointers and are omitted when nil.
22 fields are slices of unions. An unset union value therefore only ever means a required field was not filled in.

Four affect the generated source but not the wire:

* 24 of the 28 union types have no doc comment: 18 with a description in the stable schema and 6 in the unstable schema. The emitter writes the union's description
  before the first variant type it emits, so it is glued onto that variant's doc. `type ContentBlock struct` has no doc, and `ContentBlockText` carries both texts.
* `agent_gen.go` and `client_gen.go` have no blank line between their functions. Union `UnmarshalJSON` and `MarshalJSON` are adjacent in `types_gen.go`.
* Union decoding declares `var match bool = true` and calls `json.Unmarshal(v, &disc)` without checking the error. A discriminator that is present but not a string is silently
  treated as missing, and decoding falls through to key matching.
* A property with a schema default whose type allows `null` would emit `_rm, _ok := m[...]` and use only `_ok`. That does not compile. No property in the current schema reaches
  that branch.

The golden suite in `json_parity_test.go` covers populated values only. None of the cases above is under test.

## Decision Drivers

* The SDK stays on `encoding/json` v1. No fix may import `encoding/json/v2` or `encoding/json/jsontext`.
* What the SDK sends should satisfy the schema for the stamp it carries.
* A message this SDK sends should pass this SDK's own `Validate` on the receiving side.
* A nil slice and an empty slice mean the same thing to a Go caller.
* No outbound call that succeeds today may start failing, and no exported identifier may be renamed or removed. A stricter inbound check is a trade-off stated in the Consequences.
* Every change to generated code arrives through the generator and is reviewed as a diff of the regenerated files.

## Considered Options

* Fix in the generator: an unset union fails with an error naming it, nil required arrays and maps encode as empty, and the source defects are corrected
* Validate outbound messages in the connection and leave the generated encoding as it is
* Encode an unset union as `null`, and leave nil required arrays as `null`
* Leave the generated output as it is

## Decision Outcome

Chosen option: "Fix in the generator: an unset union fails with an error naming it, nil required arrays and maps encode as empty, and the source defects are corrected",
because it makes the SDK send what the schema asks for without adding a new failure to any outbound call that succeeds today, and it keeps every change in the generator.

The owner accepted this option on 2026-10-09.

The proposed changes, all on `encoding/json` v1:

* A union's `MarshalJSON` with no variant set returns `nil` and an error that names the union, such as "ContentBlock: no variant set".
* Each struct with a required slice or map field gets a generated `MarshalJSON`, or extends the one it already has for schema defaults, that encodes a nil value as `[]` or `{}`.
* The union's description is written directly above `type <Union> struct`.
* A blank line separates generated functions.
* Union decoding uses `match := true`. A discriminator that is present and not a string returns an error naming the union and the key.
* The defaulted, nullable branch uses `_, _ok := m[...]`.

### Consequences

* Good, because a `NewSessionRequest` with `Cwd` set and `McpServers` nil sends `"mcpServers":[]`, which the schema accepts and this SDK's `Validate` accepts.
* Good, because a caller who forgets a required union gets an error naming the type instead of "unexpected end of JSON input".
* Good, because pkg.go.dev shows the schema description on 24 union types that have none today.
* Neutral, because a populated message encodes to the same bytes as today. The existing goldens stay unchanged.
* Neutral, because encoding an unset union still fails. Only the error text changes.
* Bad, because a peer that sends a non-string discriminator, which decodes today by falling back to key matching, gets an error. (Narrowed 2026-10-10,
  see the amendment below: this can hold for at most 6 of the 17 discriminated unions; the other 11 already fail.)
* Bad, because ~~34 structs gain or extend~~ 33 structs gain a `MarshalJSON` (corrected 2026-10-10, see the amendment below), and each of them pays one
  extra encode through an alias type.
* Bad, because every generated file changes, and `types_gen.go` changes in many places. Review relies on the generator diff and the new tests.

### Confirmation

* New golden and unit tests, written before the generator changes and seen to fail on the current output, cover: the zero value of a struct with a required union, the
  zero value of `NewSessionRequest`, a non-string discriminator, and a union description in `types_gen.go`.
* Pinning tests cover current v1 behavior that must not move: HTML-sensitive text, sorted map keys, and the existing populated goldens.
* After regeneration, `TestGeneratedFilesUpToDate` from [0006-PLAN-emit-go-with-go-ast.md](0006-PLAN-emit-go-with-go-ast.md) passes, and so do the root tests and the examples.
* No file imports `encoding/json/v2` or `encoding/json/jsontext`.

## Pros and Cons of the Options

### Fix in the generator: an unset union fails with an error naming it, nil required arrays and maps encode as empty, and the source defects are corrected

* Good, because every fix lands where the code comes from, and one regeneration applies it to every type.
* Good, because treating nil as empty matches what a Go caller means and what the schema allows.
* Neutral, because it depends on the go/ast generator from 0006. The fixes are simpler to write in it than in Jennifer.
* Bad, because the generated diff is large.

### Validate outbound messages in the connection and leave the generated encoding as it is

`SendRequest` and `SendNotification` would call `Validate` on their params before encoding.

* Good, because it is one hand-written change, and it mirrors the validation the receiving side already does.
* Bad, because it rejects a nil slice that a caller meant as empty, which the schema allows. Calls that succeed today would start failing.
* Bad, because the generated `Validate` on a struct checks only required string and array fields. It does not look inside union fields, so the unset-union failure remains.
* Bad, because the doc and source defects remain.

### Encode an unset union as `null`, and leave nil required arrays as `null`

* Good, because encoding never fails.
* Bad, because every field that can hold an unset union is required, so `null` breaks the schema and moves the failure to the peer.
* Bad, because it hides a caller's mistake instead of reporting it.

### Leave the generated output as it is

* Good, because nothing changes for callers.
* Bad, because the SDK keeps sending `null` where the schema requires an array, and keeps rejecting its own messages.
* Bad, because 24 union types stay undocumented.

## Amendment 2026-10-10, count of required arrays and maps

Step 5 of [0009-PLAN-fix-generated-output-on-json-v1.md](0009-PLAN-fix-generated-output-on-json-v1.md) found the count above wrong by one. At stamp `1.25.0`,
34 required slice or map fields in 33 structs encode a nil value as `null`. Each of the 33 structs gains a new `MarshalJSON`; none extends one emitted for
schema defaults.

Counting struct fields by tag, a slice or map type with no `omitempty`, gives 36 fields in 35 structs. Two of those are not required:
`ElicitationSchema.Properties` and `InitializeResponse.AuthMethods` are optional, and their schema defaults already encode a nil value as `{}` and `[]`.
The original count excluded only one of the two. The decision is unchanged.

## Amendment 2026-10-10, unions that accepted a non-string discriminator

Step 7 of [0009-PLAN-fix-generated-output-on-json-v1.md](0009-PLAN-fix-generated-output-on-json-v1.md) found the consequence about non-string discriminators
too broad. 17 unions decode by a discriminator. In 11 of them every variant struct carries the key as a typed string field, so a non-string value already
failed: `ContentBlock` with `"type":1` returned "invalid variant payload", an error that names neither the union nor the key. In the other 6, `AuthMethod`,
`IdleStateUpdate`, `McpServer`, `MultiSelectItems`, `SetSessionConfigOptionRequest`, and `UnstableMcpServer`, at least one variant has no such field or,
in `IdleStateUpdate`, holds it as `any`, so such a payload could decode. One was probed: a stdio-shaped `McpServer` with `"type":1` decoded with no
error before the change.

After the change all 17 return an error that names the union and the key. A new failure is possible only in those 6 unions; for the other 11 only the error
text changes. The decision is unchanged.

## More Information

Implementation is [0009-PLAN-fix-generated-output-on-json-v1.md](0009-PLAN-fix-generated-output-on-json-v1.md). It runs after [0007-PLAN-self-contained-acp-sdk.md](0007-PLAN-self-contained-acp-sdk.md) is complete.

Counts in this record come from `types_gen.go` and the schema files at stamp `1.25.0`. The 57 failing zero values and the unchanged populated encodings were measured in a scratch copy, as described in
Appendix B of [0008-REPORT-self-contained-sdk-review.md](../reports/0008-REPORT-self-contained-sdk-review.md).
