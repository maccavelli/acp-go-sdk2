---
status: in-progress
date: 2026-10-10
associated-madr: "0011-MADR-harden-code-generator-engine.md"
---

# Implement phase 2 of the generator hardening: the wire fixes

Associated MADR: [0011-MADR-harden-code-generator-engine.md](0011-MADR-harden-code-generator-engine.md)

## Goal

Make the generated unions read and write what peers send without changing it: a value the SDK does not know lands where the schema says and is sent
on unchanged, `ContentBlock` keeps `_meta` and `annotations`, integers keep every digit, errors name the union and keep their cause, a reused union holds
only what was decoded last, and JSON `null` does what `encoding/json` does. This is phase 2 of the MADR, as refined by its amendment "phase 2 facts and
decisions" (F1, F2, F3, F5, F18, F19).

## Scope

In scope:

* `cmd/generate/internal/model` (union classes, catch-all and key-less variants, the `Unknown` and `Extra` fields)
* `cmd/generate/internal/emit/types.go` (union `UnmarshalJSON`, `MarshalJSON`, `Validate`; catch-all variant methods), and the `ContentBlock` shape code
  in `types.go` and `constants.go`
* the regenerated `types_gen.go`
* new tests in the root module and in `cmd/generate`
* `README.md` and `docs/architecture.md`, which state the wire behaviour

Out of scope, each owned by another phase of the MADR:

* the protocol table, including the hand-written helper skip list (phase 3);
* the `x-deserialize-*` keywords, integer formats, typed `additionalProperties`, deeper `Validate`, and unknown keys on variants other than the six
  catch-alls (phase 4);
* rendering in memory, type-checking the output, atomic writes (phase 5);
* engine tests beyond the ones this plan adds (phase 6);
* the hand-written SDK files, the goldens, `encoding/json/v2`; a tag, a push, or an agent commit on `main`.

## Grounding

Measured on `e420cb6` in scratch copies; the tree was not changed. The MADR amendment of 2026-10-10 holds the same facts in prose.

* **W1. The unions.** 28 unions; 17 discriminated (7 declared, 10 derived). By the schema:
  * closed (7): `ContentBlock`, `PlanUpdateContent`, `RequestPermissionOutcome`, `SessionConfigOption`, `SessionUpdate`, `ToolCallContent`,
    `UnstableNesSuggestion`; every variant has a constant value and requires the key; all are oneOf;
  * with a catch-all (6): `CreateElicitationRequest` (`mode`), `CreateElicitationResponse` (`action`), `ElicitationPropertySchema` (`type`),
    `IdleStateUpdate` (`stopReason`), `MultiSelectItems` (`type`), `StateUpdate` (`state`); the catch-all is an inline variant titled `other` whose
    discriminator is a required string without a constant and which has `not`; these are the schema's 6 uses of `not`; all six unions are anyOf;
  * defaulted (4): `AuthMethod` (`agent` has no `type`), `McpServer` and `UnstableMcpServer` (`stdio`), `SetSessionConfigOptionRequest` (`value_id`);
    all anyOf;
  * key-less variants inside catch-all unions: `IdleStateUpdate` `none` (`stopReason` typed `null`, not required) and `MultiSelectItems` `titled`.
  The model already has the discriminator, its source, and each variant's value (`DiscValue`), object, array, and null flags, and `$ref` or inline origin.
* **W2. Unknown value today.** `{"<key>":"zz_future","extra":1}` decodes without error in all 17: 15 into a known variant (two of which then fail to
  encode), `CreateElicitationRequest` and `MultiSelectItems` into `Other`, which drops `extra`.
* **W3. Absent key today.** Correct for `AuthMethod` (`Agent`), `McpServer` (`Stdio`), `SetSessionConfigOptionRequest` (`ValueId`), and for
  `ContentBlock` `{"text":"hi"}` (`Text`). Wrong: `IdleStateUpdate` `{}` (`EndTurn`, should be `None`); `RequestPermissionOutcome`
  `{"optionId":"allow-once"}` (`Cancelled`); `CreateElicitationResponse` `{}` and `{"content":{"name":"x"}}` (`Accept`); `StateUpdate` `{}` (`Running`);
  `SessionUpdate` agent chunk and tool-call update (`UserMessageChunk`); `ContentBlock` audio `{"data":..,"mimeType":"audio/wav"}` (`Text`).
* **W12. The absent-key rule, applied by script to schema facts.** `RequestPermissionOutcome` `{"optionId":..}` → `Unknown` (2 fit), `{}` →
  `cancelled`; `CreateElicitationResponse` `{}` → error (3 fit); `CreateElicitationRequest` `{"requestedSchema":{},"message":"m"}` → `form`;
  `StateUpdate` `{}` → error (4 fit); `SessionUpdate` `{"content":{}}` → `Unknown` (4 fit), tool-call update → `Unknown` (2 fit), `{}` →
  `session_info_update`; `ContentBlock` `{"text":"hi"}` → `text`, audio → `Unknown` (2 fit), `{}` → `Unknown` (0 fit); `ToolCallContent`
  `{"path":"p","newText":"n"}` → `diff`.
* **W4. Names.** `StateUpdate`'s wrapper has `Unknown *StateUpdateUnknown`. The six catch-all structs: `IdleStateUpdateOther` (`_meta`,
  `stopReason`, `usage`), `CreateElicitationOther` (`_meta`, `message`, `mode`), `CreateElicitationResponseOther` (`_meta`, `action`),
  `StateUpdateOther`, `ElicitationPropertySchemaOther`, `MultiSelectItemsOther` (the discriminator only). None has a field or key named `extra`.
* **W5. Validate.** The 7 closed unions get the exactly-one `Validate`. The four defaulted unions' `Validate` returns nil whatever is set (their shared
  required properties are `$ref`s, F7). The six catch-all unions get the shared-field `Validate`.
* **W6. F2.** The shape table drops exactly `_meta` and `annotations` from each `ContentBlock` variant.
* **W7. F2 with F3.** Encoding object variants through `map[string]json.RawMessage` with constants injected as raw JSON strings, and no shape table:
  the root suite passed, every golden and pin included; `size` 9007199254740993 survived; `_meta` and `annotations` were kept; `"<a & b>"` encoded as
  `<a & b>`.
* **W8. Counts in `types_gen.go`.** 28 union `UnmarshalJSON`; `errors.New("invalid variant payload")` 160 times in them and 92 in union `MarshalJSON`;
  `no matching variant for union` 28 times; `var m map[string]any` 92 times; 67 injected constants (`m["key"] = "value"`); 5 `ContentBlock` shaping
  blocks (`var nm map[string]any`); 17 discriminator decode checks; 1 union with a null variant (`RequestId`); 33 goldens in `testdata/json_golden/`.
* **W9. F18.** A `ContentBlock` decoded from a text block, then an image block, has `Text` and `Image` set; `Validate` fails; encoding sends the text.
* **W10. F19.** `"update": null` in a `SessionNotification` sets `UserMessageChunk`; `null` into `ContentBlock` sets `Text`; `RequestId` sets `Null`.
  The `json.Unmarshal` documentation: an Unmarshaler is called "including when the input is a JSON null"; null "has no effect on the value and produces
  no error" for types other than interfaces, maps, pointers, and slices. The `Unmarshaler` documentation: "UnmarshalJSON must copy the JSON data if it
  wishes to retain the data after returning."
* **W11. Who reads the texts that change.** Outside the generator, only `generated_output_test.go` matches an error text (`no variant set`), which this
  plan keeps. Hand-written code reads union variants only after nil checks (`acp_test.go`, `example_gemini_test.go`); no composite literal of a union
  type is unkeyed.

## Design

### Decoding a discriminated union

`UnmarshalJSON` of each of the 17, in this order:

1. JSON `null`: no change, no error (F19).
2. Reset: `*u = <Union>{}` (F18).
3. Decode into `map[string]json.RawMessage`; a payload that is not an object fails with `<Union>: <cause>`. All variants of the 17 are objects; the
   model refuses a discriminated union with an array, primitive, or null-typed variant.
4. Key present and not `null`: decode it as a string; a non-string fails with `<Union>: discriminator "<key>": <cause>` (unchanged from 0009).
   * a known value decodes only its variant; a failure is `<Union>: <key> "<value>": <cause>`;
   * an unknown value: in a catch-all union, decodes the catch-all variant (failure: `<Union>: <key> "<value>": <cause>`); otherwise it is kept,
     copied, in `Unknown`.
5. Key absent or `null`:
   * a union with key-less variants tries only those, first by required keys, then by trial decode, in schema order; when none decodes, the error is
     `<Union>: missing discriminator "<key>"`;
   * otherwise, the variants other than the catch-all whose required keys, the discriminator aside, are all present in the payload are counted. Exactly
     one decodes strictly (failure: `<Union>: variant <Field>: <cause>`). None or several: a union with `Unknown` keeps the payload there; a catch-all
     union fails with `<Union>: missing discriminator "<key>"`.

### Decoding the other eleven unions

Steps 1 and 2 as above (`RequestId`'s null variant still takes `null`), then today's order, required keys, array items, trial decode, with a variant
failure reported as `<Union>: variant <Field>: <cause>` and no match as `<Union>: no variant matches`.

### Encoding

A set variant encodes as today, except that an object variant goes through `map[string]json.RawMessage` and its constants are injected as raw JSON
strings (F3), and `ContentBlock` takes the same path (F2). With no variant set, a union with `Unknown` writes `Unknown`; otherwise the error stays
`<Union>: no variant set`. A failure to reshape a variant reads `<Union>: <cause>`.

### Validate

The exactly-one `Validate` of the 7 closed unions counts `Unknown` as a variant. The defaulted unions' `Validate` is unchanged (W5).

### The catch-all variants

Each of the six catch-all variant structs gains `Extra map[string]json.RawMessage` (`json:"-"`). Its `UnmarshalJSON` decodes the declared fields and
keeps every other key in `Extra` (nil when there is none); its `MarshalJSON` writes the declared fields and adds each `Extra` key the declared fields
did not write. A struct that also needs default or fill methods gets one `MarshalJSON` doing both; none of the six does at `1.25.0`.

### The model

`Type` gains `HasUnknown` (discriminated, no catch-all) and `CatchAll` (the catch-all variant's index, or -1); `Variant` gains `KeyLess` (no
discriminator value, not the catch-all); `Inline` gains `KeepExtra`. A catch-all is an inline object variant of a discriminated union with no
discriminator value, whose discriminator property is a required string and which has `not`. The model refuses: two catch-alls in one union; a
discriminated union with a non-object variant; a union field named `Unknown` beside a variant field of that name; a catch-all property whose field is
`Extra`.

## Implementation Steps

1. Work on `main`. Each step ends with `make pre-add-check`, `make check-records`, and `make markdownlint`, their output recorded in this plan's execution
   record, and a stop for the owner's commit. Each step writes its test first, records its failure on the current tree, then changes the generator,
   regenerates, and classifies every changed line of `types_gen.go` with a script; the goldens must not change unless the step says so.
2. **F5, error texts.** Test: `ContentBlock` `{"type":"text","text":5}` fails with an error containing `ContentBlock`, `type "text"`, and
   `cannot unmarshal number`; `RequestId` given `true` fails with `RequestId: no variant matches`. Change `decodeStrict`, the final return of
   `unionUnmarshalBody`, and the reshape failure of `unionMarshalBody` to the texts of the design. Expected: the 160 decode sites, the 92 encode sites,
   and the 28 final returns change, and nothing else.
3. **F2, `ContentBlock`.** Test: a text block built with `_meta` and `annotations`, and one decoded with them, encode with both keys. Delete
   `contentBlockShape`, `contentBlockMarshal`, `keyMimeType`, and `unionContentBlock`; `ContentBlock` uses the generic path. Expected: the 5 shaping
   blocks become `return json.Marshal(m)`; goldens unchanged.
4. **F3, precision.** Test: a `resource_link` with `size` 9007199254740993 round-trips with every digit, through `ContentBlock` and through a
   `SessionUpdate` whose content holds it. `var m map[string]any` becomes `map[string]json.RawMessage`; each constant is injected as
   `json.RawMessage("\"value\"")`. Expected: the 92 declarations and 67 injections change; goldens unchanged; the HTML and sorted-key pins pass.
5. **Model: union classes.** Add `HasUnknown`, `CatchAll`, `KeyLess`, `KeepExtra`, and the four refusals of the design, each with a test that fails first.
   A test on the committed schema lists the classes: 7 closed, 6 catch-all, 4 defaulted, and the key-less variants of W1. Expected: no generated file
   changes.
6. **F1, `Unknown`.** Test: for each of the 11 unions with `HasUnknown`, `{"<key>":"zz_future","extra":1}` decodes without error, sets no variant and
   `Unknown`, and encodes to the same JSON; the 7 closed unions' `Validate` passes with only `Unknown` and fails with `Unknown` and a variant. Emit the
   field (with a doc comment), the unknown branch, the encoding branch, and the `Validate` count. Expected: changes in those 11 unions only.
7. **F1, catch-alls.** Test: for each of the 6 catch-all unions, the same payload sets `Other` with the value and `Extra` holding `extra`, and encodes to
   the same JSON; a known value with a bad payload names the union, key, and value. Emit the catch-all branch and the `Extra` field and methods. Expected:
   changes in those 6 unions and 6 variant types only.
8. **F18, F19, absent key.** Tests: a reused `ContentBlock` holds only the image; `null` leaves a union unchanged and sets `RequestId`'s `Null`;
   `IdleStateUpdate` `{}` sets `None`; `AuthMethod`, `McpServer`, and `SetSessionConfigOptionRequest` without the key set their default; each payload of
   W12 has the outcome W12 lists. The model gains, per variant, its required keys other than the discriminator. Emit the null check, the reset, and the
   absent-key rule. Expected: all 28 `UnmarshalJSON` change; goldens unchanged.
9. **Docs.** `README.md` "Wire encoding" and `docs/architecture.md` state the unknown-value, absent-key, and null rules, `Unknown`, and `Extra`. A script
   checks every name they mention against the tree.
10. **Review and close.** Run the verification below, record each result and the list of exported API changes for the release notes, and set this plan
    to `complete`.

## Verification

* A table test in the root module covers all 17 discriminated unions: an unknown value per class (W1) lands as the design says and encodes to the same
  JSON; an absent key sets the key-less variant in the 6 unions that have one; the payloads of W12 have the outcomes W12 lists.
* Every probe of W2, W3, W7, W9, and W10 is a test that failed on `e420cb6` and passes.
* Each step's change to `types_gen.go` matches its expectation, by script; no golden in `testdata/json_golden/` changed; the pins pass.
* `go test -race ./...` passes at the root and in `cmd/generate`; `go build ./example/...` passes; `TestGeneratedFilesUpToDate` passes.
* `make pre-add-check`, `make check-records`, and `make markdownlint` pass.
* `go test -cover ./internal/model` stays at or above 85%.

## Rollout and Rollback

Commits land on `main`, made by the owner, one per step. No tag; a push waits for the owner's ask. The module is at `v0`; the next minor version's
release notes list: `Unknown` on 11 unions, `Extra` on 6 variant types, the new error texts, `ContentBlock` sending `_meta` and `annotations`, exact
integers through unions, a payload without its discriminator kept in `Unknown` rather than guessed when several variants fit, and its rejection in four
catch-all unions when no single variant fits.

Rollback is `git revert` of the step commits, newest first; each step leaves the generator and the generated files in step.

## Execution record

### Step 2, error texts, 2026-10-10

The owner approved this plan and committed the records as `cbb773f`; the plan's status is `in-progress`. Step 2 ran on `main` from that clean tree. The
agent did not commit.

New `union_wire_test.go` in the root package, `TestUnionDecodeErrorsNameTheirPlace`, three cases. Run before the generator change:

```text
--- FAIL: TestUnionDecodeErrorsNameTheirPlace/no_variant_matches
    error "no matching variant for union" does not contain "RequestId: no variant matches"
--- FAIL: TestUnionDecodeErrorsNameTheirPlace/variant_found_by_its_keys,_with_a_bad_payload
    error "invalid variant payload" does not contain "EmbeddedResourceResource: variant TextResourceContents"
    error "invalid variant payload" does not contain "cannot unmarshal number"
--- FAIL: TestUnionDecodeErrorsNameTheirPlace/known_variant_with_a_bad_payload
    error "invalid variant payload" does not contain "ContentBlock"
    error "invalid variant payload" does not contain "type \"text\""
```

The second case was added after the generator change, to cover the key-match form, and was then run against a copy of the SDK at `e420cb6`, where it
failed as shown.

`cmd/generate/internal/emit/types.go`: `decodeStrict` takes the place it reports and returns `fmt.Errorf("<place>: %w", err)`; a discriminator case
reports `<Union>: <key> "<value>"`, a key-match or array-match case `<Union>: variant <Field>` (`variantPlace`); the final return of
`unionUnmarshalBody` is `errors.New("<Union>: no variant matches")`; the reshape failure of `unionMarshalBody` returns `nil, fmt.Errorf("<Union>: %w",
err)`. `invalidPayload` is gone.

Regenerated: `types_gen.go` changed by 532 lines in and 532 out, and no other file. A script classified every changed line, with none left over:

```text
- decode check                     160      + decode check                     160
- anonymous decode error           160      + decode error, discriminator      67
- reshape check                    92       + decode error, variant            93
- anonymous reshape error          92       + reshape check                    92
- anonymous no-match               28       + reshape error                    92
                                            + named no-match                   28
unclassified: 0
union error messages not starting with their union's name: 0
```

The 67 discriminator cases are the variants with a discriminator value: 38 in the closed unions, 21 in the catch-all unions, 8 in the defaulted ones.
No golden changed; `go build ./example/...` passes.

Gates: `make pre-add-check` with the three changed Go files named: `go-precheck: 3 file(s) clean (gofmt, golangci-lint, go vet, go test,
govulncheck).` Without names: `go-precheck: 61 file(s) clean (...)`. `make check-records` and `make markdownlint` exit 0.

Commit point: the agent stopped here. The owner commits step 2 on `main`.
