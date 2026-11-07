---
status: in-progress
date: 2026-11-07
associated-madr: "0049-MADR-remediate-the-2026-11-01-debugging-pass.md"
---

# Implement 0049, the generator: G1 to G13 and G15 to G37

Associated MADR: [0049-MADR-remediate-the-2026-11-01-debugging-pass.md](0049-MADR-remediate-the-2026-11-01-debugging-pass.md),
F-G, as its "Re-assessment of F-G at `ff31967`" and "Re-assessment of F-G at `73b0537`" correct it, and its "The generator
PLAN's designs, proposed 2026-11-07", Q22 to Q29, answered by the owner 2026-11-07: (a) for Q22 to Q28, and (b) for
Q29, a patch release at the close. Each step names its question.

## Goal

Fix the generator findings of the MADR's F-G section, apart from G14, which
[0049-PLAN-null-items-in-skip-invalid-arrays.md](0049-PLAN-null-items-in-skip-invalid-arrays.md) fixed in `v1.34.1`.

* The generator refuses, naming the schema path or the rule, each shape it now crashes on, mistypes, or renders with part of
  it dropped. It exits 0 only on what it renders faithfully.
* It checks the tree that `Write` leaves, and deletes only files it generated.
* The tests that guard the protocol table check the model the SDK ships.

Each fix lands with its finding's probe as a committed test, seen to fail on the code it replaces and to pass after.

This is the last of the MADR's five PLANs (Q1 a, Q7 a). The generated files stay byte-identical for the committed schemas, so
it changes no exported API and no consumer-visible behaviour. It closes with a patch release, the next after `v1.34.1`,
`v1.34.2` unless another comes first (Q29 b).

Base: `73b0537`, whose Go code is `v1.34.1`'s. Every file and line below is at that commit. The commit that adds this PLAN
changes records only. The MADR's re-assessment at `73b0537` re-ran the probes of G1, G2, G8, G11, G15 to G17, G21 to G23, G32
and G33 there. Paths are relative to `cmd/generate/internal/` unless they start at the repository (`cmd/`, `docs/`, the
root's files).

## Scope

In scope:

* **The generator module, `cmd/generate`:**
  * `output/check.go`, `output/write.go`;
  * `load/load.go`, `load/manifest.go`, `load/unstable.go`, `load/merge.go`;
  * `model/schema.go`, `model/fields.go`, `model/defaults.go`, `model/validate.go`, `model/gates.go`, `model/model.go`,
    `model/methods.go`, `model/protocol.go`, `model/frozen.go`;
  * `emit/unstable.go`, `emit/types.go`, `emit/gates.go`, `emit/decode.go`;
  * `main.go`.
* **Its tests:**
  * new and changed engine cases in `cmd/generate/testdata/engine/`;
  * unit tests in `output`, `load` and `model`;
  * `cmd/generate/main_test.go`, `cmd/generate/fuzz_test.go`;
  * `model/protocol_test.go`, `model/classes_test.go`, `model/patch_test.go`, `model/gates_test.go`.
* **Docs:** `docs/architecture.md`, where it states what the generator writes, checks, refuses and holds in its protocol table
  (G34).
* **The records:** this PLAN, the MADR's F-G findings marked done, `docs/README.md`'s rows, and the patch's notes,
  `docs/reports/0049-REPORT-release-notes-v1-34-2.md`.

Out of scope:

* G14, done in `v1.34.1`;
* any change to a generated file, `types_gen.go`, `agent_gen.go`, `client_gen.go`, `schema_gen_test.go` or `unstable/`. A step
  that would change one stops as a deviation (design rule 2);
* the root package's hand-written code, and the other modules' packages;
* the 1.26.0 schema, which stays deferred by the owner;
* `encoding/json/v2`, which no file may import
  ([0009-MADR-fix-generated-output-on-json-v1.md](0009-MADR-fix-generated-output-on-json-v1.md)).

**Facts it rests on,** from the MADR's fifth re-assessment:

* Since `ff31967` only G14's fix changed the generator. The other findings' code is as their probes found it.
* `ElicitationSchema.type` is the one non-zero scalar default, and its closed enumeration excludes the zero value (Q22).
* No keyword stands beside a `$ref`, and no default sits below a nested object or an inline variant.
* The one `anyOf` with two non-null members is a member of `ClientNotification.params`'s list, not a property's own.
* `Model.Validates` has three callers, all tests of package `model`.
* No engine case's `meta.json` lacks `version` (a scan of `cmd/generate/testdata/engine/*.txtar` at `73b0537`).
* `accept-default-scalar.txtar` accepts G6's shape: optional `mode` `"auto"`, `n` `3` and `on` `true`, with no enumeration.
  Q22 (a) keeps it accepted, with pointer fields.

## Design rules

1. **Refuse with a place.** Each new refusal returns an error that names the schema path (`Def.properties.key…`), the
   manifest key or the protocol-table rule. Each comes with a `refuse-*.txtar` engine case whose `want.err` holds that text
   ([0012-MADR-schema-fidelity-framework.md](0012-MADR-schema-fidelity-framework.md)).
2. **The generated files do not change.** After each phase, `cd cmd/generate && go run .` leaves `git diff --exit-code --
   '*_gen.go' '*_gen_test.go' unstable/` clean, and `tools/gates.py regen` passes. A new refusal that refuses the committed
   schemas, or a change that alters a generated byte, stops the phase as a deviation.
3. **A behaviour change has a behaviour test.** An `accept-*.txtar` case whose package's behaviour changes gains a test in
   the case, as `accept-skip-invalid-items.txtar`'s `TestSkipInvalidItemsNull` is. `TestEngineBehaviour` runs whole, never
   filtered by subtest.
4. **Seen to fail.** Every new test and case is seen to fail on a planted break in a scratch copy of the tree, by a scratch
   script in the manner of `plants_g14.py`, never in the tree. The record quotes each failure.
5. **No new dependency,** and the standard library's `encoding/json` alone.
6. **The committed schemas are scanned before a refusal.** A step that adds a refusal first counts the shape in
   `schema/schema.json` and `schema/schema.unstable.json` with a scratch script, and the record quotes the counts.

## Implementation Steps

Every phase ends staged for the owner, who commits. A phase begins only on the owner's word. Every phase runs:

* `cd cmd/generate && go test -race -count=1 -shuffle=on ./...`, and the root module's tests;
* design rule 2's regeneration check;
* `make pre-add-check FILES="<the phase's files>"`;
* `make check-records`, `make markdownlint`, `make disclosure-check`.

The order is the MADR's: the mediums first (G16, G1, G2, G15, G17, G3, G18), then the lows by area, then the info findings.

### Phase 1: G16 and G1, what `Write` deletes and what `Check` checks

1. **G16.** `staleFiles` (`output/write.go:99-124`) lists a `*_gen.go` or `*_gen_test.go` that the run did not render only
   when its first line is `// Code generated by acp-go-generator; DO NOT EDIT.`, the header `emit` writes (`emit/file.go:20`,
   `generatedHeader`). `output.Write` returns the paths it removed, and `main.go` prints "generate: removed <path>" to stderr
   for each.
2. **G1.** `Check` (`output/check.go:44-84`) takes the same stale set:
   * `handWritten` (`:183-215`) leaves out the files that `Write` will remove, so `Check` type-checks the tree that `Write`
     leaves;
   * `Check` checks `unstable/` whenever that directory holds hand-written Go files, even when nothing unstable is rendered
     (`:72-74` returns early today).
3. **Tests,** in `output`:
   * `TestWriteKeepsHandWrittenGenTest`: a hand-written `helpers_gen_test.go`, with no header, survives `Write`;
   * `TestWriteNamesRemovedFiles`: a stale `old_gen.go` with the header is removed and named;
   * `TestCheckSeesTheTreeWriteLeaves`, three cases:
     * a stale `old_gen.go` with the header, used by a hand-written file, is refused "undefined: staleHelper";
     * a stale file with the header that declares a name the render also declares is accepted, since `Write` removes it;
     * a hand-written file in `unstable/` that uses a stale `unstable_gen.go` is refused when nothing unstable renders.
4. **Seen to fail:**
   * `staleFiles` without the header test: `TestWriteKeepsHandWrittenGenTest` fails;
   * `handWritten` keeping stale files: `TestCheckSeesTheTreeWriteLeaves` fails;
   * the early return restored: its `unstable/` case fails.
5. **Docs:** `docs/architecture.md`'s paragraph on `output` says which files `Write` removes, and that `Check` checks the
   tree `Write` leaves.

### Phase 2: G2, G15, G17, G3 and G18, the model's mediums

1. **G2.** `renderer.structValue` (`model/defaults.go:236-267`) carries the names of the types it is rendering. A default
   whose rendering re-enters one of them is refused "Node.next: the default recurses through Node".
   * Case: `refuse-default-recursive.txtar`, `Node.next` a `$ref` to `Node` with `default: {}`.
   * Seen to fail: the set removed, and the test binary dies with "fatal error: stack overflow".
2. **G15.** Three changes:
   * `UnionEntryKey` (`load/merge.go:410`) keys an entry that has a `const` and no key property by its `const`;
   * `newEntries` (`load/unstable.go:150-176`) compares a keyless entry with the annotations (`description`, `title`,
     `x-docs-ignore`) left out;
   * ~~`model.Build` refuses an unstable added-entry index that the model's type does not have: "ToolKind: the live unstable
     schema adds an entry, index 10, that a ToolKind enum cannot hold". `emit/unstable.go:74` then never indexes past
     `Variants`.~~ *Amended 2026-11-07, deviation 1:* `emit.RenderUnstable` refuses it, before rendering, with that
     message. `Build` never receives the indexes.

   Tests:
   * `load`: `TestNewEntriesConstEntries`. A changed description gives no entry, and a new value gives its index.
   * Cases: `accept-unstable-enum-reworded.txtar`, a stable enum whose live unstable copy changes one value's description:
     accepted, with nothing in package `unstable`. `refuse-unstable-enum-value.txtar`, a value added in the unstable copy
     alone: refused with the message above.
   * Seen to fail: each change reverted in its own copy. The panic, "index out of range", returns for the reverted refusal.
3. **G17 (Q24 a).**
   * `walkRefs` (`model/schema.go:345-375`) refuses a property whose own `anyOf`, or `oneOf`, has exactly two members, both
     non-null: "Alias.properties.p: anyOf of two non-null members, which one field cannot hold".
   * `optionalTypeRef` (`model/fields.go:315-362`) types a `const` member from its own `type`, and an untyped one from its
     JSON kind: a string as `string`, a boolean as `boolean`, a whole number as `integer`, another number as `number`.

   Cases:
   * `refuse-two-member-union.txtar`: `p`, `{anyOf: [string, int32]}`;
   * `refuse-two-member-union-refs.txtar`: `q`, two `$ref`s;
   * `accept-typed-const-member.txtar`: `n`, `{anyOf: [{type: integer, format: int32, const: 5}, null]}`, generated as
     `N *int32`. Its behaviour test decodes `{"n":5}`.

   Committed schemas: the scan counts properties whose own list has two non-null members, 0 expected. The regeneration check
   shows `ClientNotification.Params` still `json.RawMessage`. Seen to fail: the refusal removed; the const retyped as a string.
4. **G3.** `applyChange` (`load/manifest.go:184-244`) refuses a changed property, or entry, that the stable schema now holds
   with a different definition: "manifest: Shared.b: the stable schema now defines it, differently; drop it from the
   manifest's changes". ~~Annotations are left out of the comparison, as in G15.~~ *Amended 2026-11-07, deviation 2:*
   annotations and `default` are left out of the comparison, at every depth.
   * Case: `refuse-unstable-manifest-stabilized.txtar`, from the MADR's `stabilized.txtar` probe: the stable `Shared.b` is
     `{"type":"integer"}`, and the manifest's frozen one a string.
   * `accept-unstable-manifest-changed.txtar` stays accepted.
   * Seen to fail: the comparison removed, and the case generates `B string`.
5. **G18.** `loadSchema` (`cmd/generate/main.go:172-206`) moves into package `load` as `load.Committed(schemaDir)`.
   `main.go` calls it, and so does `committedInput` (`model/classes_test.go:25-49`), in place of `MergeStableAndUnstable`.
   * Test: `load`'s `TestCommittedFollowsTheManifest`. In a temporary schema directory whose live unstable meta adds a method
     that the manifest lacks, `Committed` leaves the method out.
   * Seen to fail: `Committed` merging the live files.
   * `TestProtocolTableAppliesToCommittedSchema` and the union-class tests pass unchanged, since the models are the same
     today (the MADR's "types 280/280").

### Phase 3: G9, G27, G28 and G10, the output edge's lows

1. **G9.** `Check` type-checks the package for the host, and for `linux/amd64` and `windows/amd64`. Each platform's file list
   comes from `go/build` with `GOOS` and `GOARCH` set and cgo on, as `handWritten` sets it (`output/check.go:189-191`). A
   platform whose file list equals one already checked is skipped. An error names the platform.
   *Amended 2026-11-07, deviation 1:* a platform whose lists differ from the host's is not type-checked in process. It is
   compiled by the go command for that `GOOS` and `GOARCH`, `go test -c` with `-overlay`, so the rendered files stay in
   memory and the stale ones read as deleted.
   * Test: `output`'s `TestCheckOtherPlatforms`. A `x_windows.go` that declares a generated name is refused on a `darwin` or
     `linux` host, naming `windows/amd64`.
   * Seen to fail: the platform loop removed.
2. **G27.** `handWritten` also returns `XTestGoFiles`, and `Check` type-checks the root's external test package. It imports
   the in-memory root through a `servingImporter` (`output/check.go:84-101`), as `unstable/` does. A package of the module
   that an external test imports, such as `acptest`, is type-checked from source by the same importer, against the in-memory
   root. If that needs more than a source importer that serves the module's own packages, the phase stops as a deviation.
   * Test: `output`'s `TestCheckExternalTests`. An `x_test.go` in `package acp_test` that names `acp.ToolKindSearch`, after a
     render that lacks it, is refused "undefined: acp.ToolKindSearch".
   * Seen to fail: `XTestGoFiles` left out.
3. **G28.** `goVersion` (`output/check.go:219-234`) reads the `go` line of the `go.mod` that `rootImportPath`
   (`cmd/generate/main.go:89-119`) finds, walking up from `-out`. One function, in `output`, serves both.
   * Test: `TestCheckVersionFromEnclosingModule`. `go 1.21` in a parent `go.mod`, `-out` a subdirectory, and `for range 3 {}`
     in a hand-written file: refused "requires go1.22 or later".
   * Seen to fail: `goVersion` back to `<out>/go.mod`.
4. **G10.** `TestGeneratedFilesUpToDate` (`cmd/generate/main_test.go:38-68`) runs its stale-file loop over the root too. A
   root `*_gen.go` or `*_gen_test.go` with the generated header that the run does not render fails it.
   * Seen to fail: in a copy with an `extra_gen.go` at the root, the loop removed, and the test passes.

### Phase 4: G4, G5, G7, G8, G32 and G37, the model's refusals

1. **G4.** `load.Definition`'s `Minimum` and `Maximum` (`load/load.go:64-65`) become `*json.Number`.
   * `bounds` (`model/validate.go:311-361`) parses an integer type's bound exactly, with `strconv.ParseInt` or `ParseUint`.
     It refuses a fractional bound, a bound out of the type's range, and a magnitude at or above 2^53, as `integerConst` does.
   * A `number` type's bound stays a float, so `accept-validate-fractional-bound.txtar` is unchanged.
   * `boundLit` (`emit/types.go:1466-1474`) writes the exact integer.
   * Cases: `refuse-bound-above-2-53.txtar`, an `int64` `maximum` of 2^53+1; `refuse-bound-uint64-range.txtar`, a `uint64`
     `minimum` of 2^64, refused the same on `arm64` and `amd64`.
   * Seen to fail: the 2^53 refusal removed.
   * *Amended 2026-11-07, deviation 1:* the refusals of a fractional bound and of a magnitude of 2^53 or more, and
     `refuse-bound-above-2-53.txtar`, are replaced. An integer type's bound is computed exactly with `math/big`: a fraction
     still rounds inward (0019's G12), the range check is exact, and the literal is written from the exact integer, so a
     `uint64` bound above `math.MaxInt64` stays one (0019's G21). `accept-validate-fractional-bound.txtar` stays accepted,
     unchanged. Cases: `accept-validate-exact-bound.txtar`, an `int64` `maximum` of 2^53+1 written as `9007199254740993`;
     `refuse-bound-uint64-range.txtar`, as above. Seen to fail: the bounds back to `float64`.
2. **G5,** one refusal each, in the model's schema walk (`model/schema.go`, `walkRefs` and `subschemas`), or in `load` for
   (g):
   * (a) `not` or `unevaluatedProperties` outside a catch-all variant;
   * (b) `const` or `enum` on a property of type `object`;
   * (c) `minimum`, `maximum` or `minLength` on array items or map values;
   * (d) any keyword beside `$ref` but the annotations `description`, `title` and `deprecated`;
   * (e) a definition that is only a `$ref`;
   * (f) a `type` name outside JSON Schema's seven;
   * (g) a key repeated in one object of `schema.json`, `schema.unstable.json`, `meta.json` or the manifest. A token walk
     with `json.Decoder.Token` finds it, in `load`.

   Each has a case: `refuse-not-keyword.txtar`, `refuse-object-property-const.txtar`, `refuse-item-bound.txtar`,
   `refuse-ref-sibling.txtar`, `refuse-ref-only-definition.txtar`, `refuse-unknown-type.txtar` and
   `refuse-duplicate-key.txtar`. The scan of design rule 6 counts each shape in the committed schemas, 0 expected. For (d),
   it counts every keyword beside `$ref`, 0 at `73b0537`. Seen to fail: each refusal removed in its own copy.
3. **G7 (Q23 a).** `checkDefaults` (`model/fields.go:380`) refuses a default on a property that `plainFields`
   (`:588-610`) builds, that is, below a nested inline object or in an inline variant: "Outer.properties.inner.properties.flag:
   a default below a nested object, which is not applied".
   * Cases: `refuse-default-nested-inline.txtar`; `refuse-default-inline-variant.txtar`.
   * `refuse-default-nested.txtar`, a wrong kind through an `allOf` of one reference, is a different shape and stays.
   * Seen to fail: the refusal removed.
4. **G8.** `load.ReadMeta` and `ReadMetaUnstable` refuse a `version` below 1: "meta.json: version 0; want 1 or more". Then
   `MergeManifest` (`load/manifest.go:116`) and `MergeStableAndUnstable` (`load/merge.go:84`) never see 0.
   * Case: `refuse-meta-no-version.txtar`.
   * Seen to fail: the check removed.
5. **G32.** `parseSchema` (`load/load.go:213-216`) refuses a definition whose value is JSON `null`: "$defs.X: null, not a
   schema".
   * Case: `refuse-definition-null-value.txtar`.
   * `accept-null-definition.txtar`, a definition `{"type": "null"}`, and `refuse-unstable-null-definition.txtar` stay as
     they are. The phase reads the latter first. If it pins today's nil-keeping behaviour for the unstable file, the step
     records how the two now agree.
   * Seen to fail: the refusal removed.
6. **G37.** `admitsNull` (`model/schema.go:393-408`) also returns true in three cases:
   * an `allOf` whose members all admit `null`;
   * an `enum` that holds `null`, or a `const` of `null`;
   * an item with no `type`, `$ref`, `const`, `enum`, `anyOf`, `oneOf` or `allOf`.

   Tests:
   * `model`: `TestAdmitsNull`, a table of each case and its controls (a `string`, a closed enum, an `allOf` with one
     non-null member).
   * Cases: `refuse-skip-invalid-allof-nullable-items.txtar` and `refuse-skip-invalid-untyped-items.txtar`, from the MADR's
     probe.
   * The committed schemas' 19 stable and 24 unstable skip-invalid arrays stay accepted: the regeneration check.
   * Seen to fail: each of the three cases removed.

### Phase 5: G6, G21, G22, G23 and G24, types and decoding

1. **G6 (Q22 a).** An optional, non-nullable scalar field with a non-zero default whose zero value is a valid value gets a
   pointer type, as a nullable one has, so an explicit zero is sent. The plain type stays when the property's closed
   enumeration or `const`, followed through `allOf` and ``, excludes the zero value. The change is in `objectFields` (`model/fields.go:540-586`), beside the
   default (`:551-567`).
   * `ElicitationSchema.Type` keeps its type, and the regeneration check shows nothing generated changes.
   * `accept-default-scalar.txtar`'s `mode`, `n` and `on` become `*string`, `*int32` and `*bool`, so its `want.api` changes
     by `-update`. Its behaviour test gains `{"mode":"","n":0,"on":false}`, re-encoded with all three keys.
   * Case: `accept-default-scalar-excluded-zero.txtar`, a closed enum without `""`: a plain field.
   * Seen to fail: the pointer rule removed, and the behaviour test's re-encoding loses `on`.
2. **G21 (Q26 a).** `discriminatorKey` (`model/schema.go:125-141`) and `discriminatorValue` (`model/model.go:744-757`) read a
   one-value string `enum` as its `const`.
   * Case: `accept-enum-discriminator.txtar`, the MADR's `Shape`. Its behaviour test decodes `{"kind":"circle","r":1}` into
     `Circle`, with `Unknown` empty.
   * Seen to fail: the `enum` read removed, and the payload lands in `Unknown`.
3. **G22.** `isOpenStringEnum` (`model/schema.go:167-194`) requires at least one string member that is not a `const`. An
   `anyOf` of string consts alone is a closed enumeration, which `Validate` checks (`model/validate.go:212-215`).
   * Case: `accept-anyof-const-only.txtar`. Its behaviour test: `Validate` refuses `"c"`, and accepts `"a"`.
   * Committed: each `anyOf` string enum, such as `CompactionStatus`, has a plain string member, so the regeneration check
     holds.
   * Seen to fail: the member rule removed.
4. **G23 (Q27 a).** `Fill` (`model/fields.go:580`, `:603`) is false for a required collection that admits `null`
   (`includesNull`).
   * Case: `accept-required-nullable-collection.txtar`. Its behaviour test: `{"xs":null}` decodes with no error and
     re-encodes as `{"xs":null}`; `{"xs":[]}` stays `[]`.
   * Seen to fail: `Fill` back, and `null` re-encodes as `[]`.
5. **G24.** `unknownAbsent` (`emit/types.go:779-802`) assigns the default `def`, when there is one, in the `RefSlice` case
   without skip-invalid items (`:787-793`) and in the `RefPointer` case (`:784-786`). The comment at `:791-792` says so.
   * Case: `accept-default-on-error.txtar` gains `kinds`, an array of `K {a, b}` with default `["a"]`. Its behaviour test:
     `{"kinds":["zzz"]}` and `{"kinds":["b","zzz"]}` give `["a"]`.
   * Committed: `audience` and `positionEncodings` skip invalid items, so the regeneration check holds.
   * Seen to fail: `nil` back.

### Phase 6: G19, G20, G25 and G26, validation, gates and rules

1. **G19 (Q25 a).**
   * `validationShapes` (`model/validate.go:143-173`) adds the item type of an `InlineNamed` slice or map variant.
     `buildValidations` (`:110-112`) sets `vi.Validates` when the item type validates.
   * `unionValidateBody` (`emit/types.go:1212-1245`) emits a loop over the variant's items.
   * The gates do the same: `dispatchReach` (`model/gates.go:479-481`) and `gateHolder.nested` (`:505-509`). The refusal of
     a shape that the gates do not descend into (`:145-151`) covers variants too.
   * Case: `accept-union-array-variant-validates.txtar`, the MADR's `Options`. Its behaviour test: an `Option` whose `name`
     is `""`, inside the array variant, fails `Validate` with "Option: name: shorter than minLength 1".
   * Committed: `SessionConfigSelectOptions`' item types have no check, so the regeneration check holds.
   * Seen to fail: the variant shapes removed.
2. **G20 (Q28 a).** `gateCheck` (`model/gates.go:197-259`, the property lookup at `:200-208`) refuses a capability rule whose definition, `allOf` expanded,
   holds the property, while `t.Fields` lacks it: "capability rule LoadSessionRequest.dirs: the property is in copies
   (WrapD), not a field of LoadSessionRequest". `resolveKeyOrders` (`model/model.go:1655-1679`) refuses a key-order rule the
   same way. A rule whose definition is absent, as in the stable-only build, still skips (`:1661-1663`).
   * Cases: `refuse-capability-property-in-copies.txtar`; `refuse-key-order-property-in-copies.txtar`.
   * Seen to fail: the `false, nil` return restored.
   * *Amended 2026-11-07, deviation 1:* `resolveKeyOrders` already refused such a rule, "a key-order rule needs a property
     of an object", so the key-order half has no change; `refuse-key-order-property-in-copies.txtar` guards that refusal,
     and passes at the base. The capability half's test is `model`'s `TestCapabilityRuleOnCopiedProperty`, not an engine
     case: engine cases run with ACP's own table, which holds no such rule.
3. **G25.** `plainFields` (`model/fields.go:599`) sets `Deprecation: deprecationOf(prop)`. `deprecationsOf`
   (`model/protocol.go:1013-1023`) and `applyFrozenDeprecations` (`model/frozen.go:49-59`) mark the copies by `f.prop`.
   * Case: `accept-deprecated.txtar` gains `U`, whose `oneOf[0]` is an `allOf` of a `Base` with a deprecated `old`. Its
     `want.deprecated` lists `field UBase.Old`.
   * Committed: no deprecated property is copied into an inline variant (the MADR). The regeneration check proves it.
   * Seen to fail: the `Deprecation` field dropped.
4. **G26.** Three checks:
   * `buildMethods` (`model/methods.go:132-186`) keeps one set of outbound method names per side, across the stable and
     unstable bindings. It reserves the connections' own methods, `Close`, `Connection`, `Done`, `DroppedNotifications`,
     `Err`, `SetLogger` and `Wait`, the list the engine cases' `want.api` shows.
   * A key order's `Field` is checked against the generated method names (`checkNotMethod`) and against every other
     key-order rule of its definition (`model/protocol.go:436`).
   * `Interface.Var` is checked with `token.IsIdentifier`, and against the dispatch's locals, which the phase lists from
     `emit` first (`model/protocol.go:373`).

   Cases: `refuse-method-name-across-bindings.txtar` (a stable `FooRequest` and an unstable `FooNotification`);
   `refuse-method-name-reserved.txtar` (a method that becomes `Close`); `refuse-key-order-field-method.txtar` (`Field:
   "MarshalJSON"`). The `Var` check is a `model` unit test, since engine cases run with `&model.ACP`. Seen to fail: each
   check removed.
   *Amended 2026-11-07, deviation 4:* the engine harness reads an optional `protocol.json` in a case, which replaces the
   named fields of a copy of `model.ACP` for that case, so `refuse-key-order-field-method.txtar` is written as planned,
   with `refuse-key-order-field-twice.txtar` for two rules of one field.

### Phase 7: G11, G33 and G31, the input edge and the CLI

1. **G11.** `run` (`cmd/generate/main.go:32-82`) refuses `fs.NArg() > 0`: "generate: unexpected argument \"stray\"; use
   -schema and -out".
2. **G33.** `run` returns nil on `flag.ErrHelp` (`:37-39`). `flag` prints the usage, and `main` exits 0.
3. **G31.** `ManifestMethods` and `ManifestChange` (`load/manifest.go:45-60`) gain an `UnmarshalJSON` through `decodeStrict`
   (`load/load.go:141-148`), on an alias type, as `Discriminator` has.
4. **Tests:**
   * `main_test.go`'s `TestRunRefusesArguments` and `TestRunHelp` (nil error);
   * `load`'s `TestManifestNestedKeysStrict`: `"changes": {"Obj": {"PROPERTIES": ["b"]}}` refused, and
     `"methods": {"AGENT": {}}` refused.
5. **Seen to fail:** each check removed.

### Phase 8: G29, G35, G36 and G12, the table's tests and the fuzzer

1. **G29.** Three wording scans, beside `patchWording` and `retainedWording` (`model/patch_test.go:307`, `:349`):
   * `capabilityWording`, `(?i)only available if the (agent|client) supports the`: each definition that carries it has a
     capability rule or a method gate;
   * `methodGateWording`, `(?i)MUST only send this (update|notification|request)`: each carrier has a method gate;
   * `absPathWording`, `(?i)must be an absolute path`: each carrier has an absolute-path rule.

   At `73b0537` the regexes match 7, 5 and 6 times in the stable schema, and 8, 5 and 7 in the unstable one. The phase first
   lists each match with its rule. A match with no rule is a real gap: it stops as a deviation, not a test to loosen.
   *Amended 2026-11-07, deviation 1:* the capability and the send wordings each require the carrier to be gated by any
   rule that applies: a method gate on its `x-method`, or a capability rule on the definition, its property or its union
   variant, a variant's payload type included. The absolute-path wording requires a path rule.
   * Seen to fail: the MADR's three probe sentences added in a copy, each failing its scan; one rule removed.
2. **G35.**
   * `model`'s `TestDeprecationNoteVariantWithoutDiscriminator` covers `model/protocol.go:1026` for a variant with no
     discriminator value.
   * Case: `accept-empty-clears.txtar`. Its behaviour test: an empty collection sent where the rule says empty clears.
   * Seen to fail: the mutant `&&` to `||` at `:1026`; the rule's emission removed.
3. **G36.** `methodByWire` (`model/protocol_test.go:72-79`) takes the side. Its callers pass the rule's side.
   * Test: a table case for a client-side rule on `mcp/message`, which is on both sides.
   * Seen to fail: the side ignored.
4. **G12.** `FuzzBuild` (`cmd/generate/fuzz_test.go:17-44`) seeds each case's `meta.json` with its `schema.json`, takes both,
   and builds with the case's meta. It does not run `Check`: `TestEngineCases` type-checks every accepted case, and a
   type-check per fuzz input would cut the inputs a minute covers.
   * Test: `TestFuzzSeedsBuild`. Every `accept-*` case's pair builds and renders.
   * The record gives the seed census, accepted and refused, before and after: "accepted 1, refused as unlisted-method 116,
     refused otherwise 70" at `ff31967`.
   * Seen to fail: the empty `Meta` back.

### Phase 9: G13, G30 and G34, the info findings

1. **G13.** `Model.Validates` (`model/validate.go:80`) moves to a test file of package `model`, its three callers'. `deadcode
   ./...` in `cmd/generate` no longer lists it, and the record quotes the run.
2. **G30.** `markGateHelpers` (`emit/gates.go:59-73`), or the union emission, marks `helperUnknownKind` for every union that
   has an `Unknown`, not only for a gate check with a `RawKey`.
   * Test: `emit`'s `TestUnknownKindWithoutRawKeyGate`. The ACP table with the prompt's undeclared-variant rule
     (`model/acp.go:232`) removed still renders `unknownKind`.
   * The regeneration check: `types_gen.go` already has it, and it does not change.
   * Seen to fail: the marking back.
3. **G34.** `docs/architecture.md:182-282` gains the `DocNotes` bullet: notes the table adds to a definition's doc comment,
   10 in `model/acp.go`.
4. **Docs:** `docs/architecture.md`'s generator sections, for the behaviour of phases 1 to 8: what `Write` removes; the
   platforms and external tests `Check` reads; where the language version comes from; the new refusals, listed by shape.

### Phase 10: close

1. `tools/gates.py --all`, with the Linux and Windows hosts named per command.
2. A table of G1 to G13 and G15 to G37, each with the test or case that shows it and its plant's failure.
3. `make api` against `v1.34.1`: "0 incompatible, 0 compatible".
4. After the owner's push, CI on the head.
5. **The notes** (Q29 b), `docs/reports/0049-REPORT-release-notes-v1-34-2.md`, against `v1.34.1`:
   * consumers meet no change: the generated files and the exported API are as `v1.34.1`'s;
   * for maintainers, what the generator now refuses, names, keeps and checks, by finding.
6. **The release,** in one short hand-off:
   * the body is built from the notes, with each relative link made absolute at the tag and checked to exist;
   * `make push-guard`, after a fetch;
   * the owner commits, pushes, tags `v1.34.2` and creates the release;
   * then CI on the tag, `tools/publication.py v1.34.2 <rev> <the body>`, and `tools/consumer.py v1.34.2`.
7. This PLAN and its row are marked `complete`, and the MADR's F-G findings done.

## Verification

* Each of G1 to G13 and G15 to G37 has a committed test or engine case that fails at `73b0537`'s code, by its plant, and
  passes after. G13 and G34 are verified by `deadcode` and by reading.
* After every phase, the generated files are byte-identical to `73b0537`'s, and `tools/gates.py regen` passes.
* The MADR's probes, re-run at the close by `reprobe.py` and `verify.py`, give refusals or the faithful types:
  * G1: refused before writing;
  * G2: refused;
  * G8: refused;
  * G11: refused;
  * G15: accepted, for the reworded copy;
  * G16: the file kept;
  * G17: refused;
  * G21: routed;
  * G22: closed;
  * G23: `null` kept;
  * G32: refused;
  * G33: exit 0;
  * G37: refused.
* `make api` reports no change; `tools/gates.py --all` and CI pass.
* `v1.34.2`: CI on the tag passes, `publication.py` reports 10 of 10, and `consumer.py` reports "as required".

## Rollout and Rollback

* **Rollout.** The generator only. What a maintainer meets:
  * `go run ./cmd/generate` refuses the shapes listed above with their paths, where it exited 0;
  * it names each file it removes, and keeps a hand-written `*_gen_test.go`;
  * it type-checks for three platforms and the external tests, and is slower by about that.

  The SDK's consumers meet nothing: the generated files do not change. The patch `v1.34.2` carries the work, with notes
  that say so.
* **Rollback.** Revert the phase's commit. Each phase stands alone. No generated file, exported API or wire format changes,
  so a consumer pinning `v1.34.1` instead of `v1.34.2` loses nothing.

## Execution record

The owner committed this PLAN and the answers to Q22 to Q29 as `189a307`, and said "committed proceed". The PLAN and its row
are marked `in-progress`.

### Phase 1: G16 and G1, what `Write` deletes and what `Check` checks, 2026-11-07

**What changed:**

* **G16.**
  * `staleFiles` (`cmd/generate/internal/output/write.go`) lists a `*_gen.go` or `*_gen_test.go` that the run did not render
    only when its first line is `// ` and `emit.GeneratedHeader`, read by a new `generatedFile`.
  * `Write` and `writeWith` return the paths they removed. `run` (`cmd/generate/main.go`) prints "generate: removed <path>"
    to stderr for each.
  * The header's text was `emit`'s unexported `generatedHeader` (`emit/file.go:20`). It is exported as
    `emit.GeneratedHeader`, so `output` reads the one constant rather than a copy. The file's one other use,
    `layout`, names it.
* **G1.**
  * `Check` computes the stale set by `staleFiles`. `handWritten` takes it, by name per package directory (`staleIn`), and
    leaves those files out.
  * `Check` checks `unstable/` when it holds hand-written files, even when nothing renders into it. `checkPackage` then takes
    the package's name from a hand-written file.
* **Docs:** `docs/architecture.md`'s paragraph on what the generator writes, and its `output.Check` and `output.Write`
  sentences.

**Tests:**

* `TestWriteKeepsHandWrittenGenTest`: three files named like generated ones survive `Write`, and a stale one with the header
  goes:
  * a hand-written `helpers_gen_test.go`;
  * a file with another tool's header;
  * a file with the header on a later line.
* `TestWriteNamesRemovedFiles`: nothing removed is named on a clean run. Then a stale file at the root and one in `unstable/`
  are each removed and named.
* `TestCheckSeesTheTreeWriteLeaves`, three subtests: a hand-written file using a stale generated one is refused "undefined:
  staleHelper"; a stale file colliding with the render is accepted; `unstable/`'s `doc.go` using a stale `unstable_gen.go`, with
  nothing rendered there, is refused "undefined: staleUnstable".
* `TestWritePackageDirectories`' stale fixtures, `old_gen_test.go`, `unstable/old_gen.go` and the `unstable_gen.go` that the
  first `Write` puts there, now start with the header. Under G16 that is what makes a file stale. The test also requires
  `Write` to name those three. The other `Write` and `writeWith` callers take the new result.

**Seen to fail,** by a scratch script, `plants_gen.py 1`, each break in its own copy of the working tree:

* `staleFiles` without the header test: "write_test.go:338: notes_gen.go: "", open …/notes_gen.go: no such file or
  directory; want it kept".
* `handWritten` keeping stale files:
  * "check_test.go:228: Check = <nil>, want it to refuse undefined: staleHelper";
  * "check_test.go:235: Check refused a stale file that Write removes: generated code does not type-check with …".
* The early return before `unstable/` restored: "check_test.go:254: Check = <nil>, want it to refuse undefined:
  staleUnstable in package unstable".

All four as required.

**Checks:**

* `cd cmd/generate && go run .`: exit 0, no output. `git diff --exit-code -- '*_gen.go' '*_gen_test.go' unstable/`: exit 0,
  so the generated files are unchanged.
* `python3 tools/gates.py regen`: "ok   regen (exit 0)".
* `cd cmd/generate && go test -race -count=1 -shuffle=on ./...`: every package `ok`.
* `make pre-add-check FILES="<the six Go files>"`: exit 0, "6 file(s) gofmt-clean; both modules linted, vetted and tested
  (-race -shuffle=on, uncached), tidy, govulncheck."
* `make check-records`, `make markdownlint` ("0 issues"), `make disclosure-check` ("0 finding(s)") and `tools/idscan.py` over
  the nine changed files: exit 0 each.

**Status:** no deviation. Staged for the owner, who commits. Phase 2, G2, G15, G17, G3 and G18, begins on the owner's word.

### Phase 2: G2, G15, G17, G3 and G18, the model's mediums, 2026-11-07

The owner committed phase 1 as `5e0761b`, and said "committed, proceed".

**Deviation 1, 2026-11-07: where G15's refusal goes.**

* **Found:** step 2.2's third bullet puts the refusal in `model.Build`. `Build` never receives the added-entry indexes: they
  go from `load.NewUnstable` to `emit.RenderUnstable` (`cmd/generate/main.go:162-169`), as `emit.Unstable.Variants`. And
  `emit/unstable.go:74` indexes `t.Variants[i]` with no check, which is the panic.
* **Decision:** the owner chose the recommended option. `emit.RenderUnstable` refuses, before rendering, an index that the
  model's type does not have, naming the type and the index. It already returns an error and holds both. Not chosen: an
  `Input` field for `Build`; the check in `main.go`'s `renderUnstable`, which would leave `emit` able to panic for another
  caller.
* **Scope:** `emit/unstable.go` was already in scope. No file is added.
* **The MADR:** not amended. G15's fix, "refuse an index the model's type does not have, with an error instead of a panic",
  names no place.

**Deviation 2, 2026-11-07: G3's comparison and the manifest's own overrides.**

* **Found:** with step 2.4's refusal in place, `cd cmd/generate && go run .` exited 1: "generate: unstable-v1.json: change of
  "AgentCapabilities": property "mcpCapabilities": the stable schema now defines it, differently; drop it from the manifest's
  changes". That refuses the committed schemas, which design rule 2 stops on.
* **The cause:** a scratch script, `g3_diff.py`, compared each property in the manifest's `changes` with the stable schema's,
  annotations aside. Two are in the stable schema with another shape: `AgentCapabilities.mcpCapabilities` and
  `InitializeResponse.agentCapabilities`. Each differs only in `default`, where the frozen one adds the unstable
  `"acp": false`, as `McpCapabilities.acp` is unstable. The manifest overrides them on purpose. The other eleven are not in the
  stable schema.
* **Decision:** the owner chose the recommended option. G3 compares without annotations and without `default`, at every
  depth. A change of type or shape, G3's hazard, is still refused. Not chosen: recording freeze-time stable copies in the
  manifest, or marking deliberate overrides in it, each a change of the manifest's format.
* **The trial:** in a scratch copy with the comparison so, `go run .` exited 0, and `git status --porcelain` was empty: every
  generated file the same. So step 2.1's, 2.2's, 2.3's and 2.5's changes refuse nothing committed either.
* **Scope:** no file added.
* **The MADR:** G3's fix is amended, 2026-11-07.

**What changed:**

* **G2.** `renderer` (`cmd/generate/internal/model/defaults.go`) carries `fallbacks`, the fields, as `Type.key`, whose schema
  default it is rendering in place of an absent key. `structValue` refuses a field met again among them: "the default recurses
  through Node.next". Only a schema default can lead back to itself, as an explicit value is finite.
* **G15:**
  * `UnionEntryKey` (`load/merge.go`) keys an entry that has none of the other keys by its `const`, as `value "a"`. It is
    checked last, so no entry that had a key before changes it, a titled const included.
  * `newEntries` (`load/unstable.go`) matches a keyless entry by `sameShape`, a new helper in `load/merge.go`, which leaves out
    `description`, `title` and `x-docs-ignore` at every depth.
  * `emit.RenderUnstable` refuses an index the model's type lacks, per deviation 1: "K: the live unstable schema adds an entry,
    index 2, that the generated K, with 0 variants, cannot hold".
* **G17:**
  * `walkRefs` (`model/schema.go`) calls a new `twoNonNullMembers` on each property: "Alias.properties.p: anyOf of two non-null
    members, which one field cannot hold".
  * `optionalTypeRef` (`model/fields.go`) keeps a `const` member's own `type`, and gives an untyped one its value's JSON kind,
    by a new `constType`.
* **G3.** `applyChange` (`load/manifest.go`) refuses a changed property, or frozen entry, that the stable schema holds with
  another shape. Per deviation 2, it compares by `sameShape` with defaults left out too: "change of "Shared": property "b": the
  stable schema now defines it, differently; drop it from the manifest's changes".
* **G18.** `load.Committed` (`load/load.go`) holds what `loadSchema` held. `loadSchema` (`cmd/generate/main.go`) calls it, and
  so does `committedInput` (`model/classes_test.go`), in place of `MergeStableAndUnstable`.

**Tests:**

* Engine cases, the seven the step names:
  * `refuse-default-recursive.txtar`, with `next` as an `allOf` of one `$ref` with the default, as the committed
    `ElicitationSchema.type` is, since phase 4's G5 (d) refuses a keyword beside a `$ref`;
  * `accept-unstable-enum-reworded.txtar` and `refuse-unstable-enum-value.txtar`, built from `accept-unstable-empty.txtar`,
    with a stable enum `K` used by `OldRequest`;
  * `refuse-unstable-manifest-stabilized.txtar`, built from `accept-unstable-manifest-changed.txtar`, with the stable `Shared.b`
    an integer;
  * `refuse-two-member-union.txtar`; `refuse-two-member-union-refs.txtar`, a `oneOf` of two references;
  * `accept-typed-const-member.txtar`, generated as `N *int32`, `F *bool` and `S *string`. Its behaviour test decodes
    `{"n":5,"f":true,"s":"x"}`.
* `load`'s `TestNewEntriesConstEntries`:
  * a reworded value, a new value at index 2, and reordered values;
  * the key itself, and a titled const's;
  * a reworded keyless entry, matched, and a keyless entry of another shape, new.
* `load`'s `TestCommittedFollowsTheManifest`, from `accept-unstable-empty.txtar` with `x/late` added to the live unstable
  files: `Committed` lacks it, and the live merge, the control, has it.
* `TestApplyChange` gains three cases: a changed property the stable schema now types otherwise, refused; a frozen entry it now
  shapes otherwise, refused; a default-only override, accepted with the frozen default.

**Seen to fail,** by `plants_gen.py 2`, each break in its own copy of the working tree:

* G2, the check removed and the call back to `r.value`, as at `73b0537`: "fatal error: stack overflow". The first run of
  this plant left `via.value` in place. It exited 1 too, "signal: killed" after 90 s, as the growing `fallbacks` ran out of
  memory before the stack did. The plant was corrected to restore the original call.
* G15:
  * the const key removed: "unstable_test.go:40: UnionEntryKey of the const "a" = "", want value "a"";
  * the keyless comparison with annotations: "unstable_test.go:49: a keyless entry reworded: newEntries = [0], want none";
  * both removed: "engine_test.go:47: K: the live unstable schema adds an entry, index 0, …";
  * the index check removed: "panic: runtime error: index out of range [2] with length 0".

  The two `load` changes each mask the other in `newEntries`, so each is shown by its own assertion, and the engine case by
  both.
* G17: the refusal removed, "engine_test.go:47: run accepted the schema", for each of the two cases; the const typed as a
  string again, `want.api` differing at "type Thing struct{F *string …; N *string …".
* G3:
  * the refusal removed: "run accepted the schema", and "manifest_test.go:194: a changed property the stable schema now types
    otherwise: <nil>, …";
  * defaults compared too: "manifest_test.go:206: a default-only override: property "extra": the stable schema now defines it,
    differently; …".
* G18, `Committed` merging the live files: "unstable_test.go:96: Committed has the live unstable x/late, which the manifest
  lacks: …".

All as required.

**Checks:**

* `cd cmd/generate && go run .`: exit 0. `git diff --exit-code -- '*_gen.go' '*_gen_test.go' unstable/`: exit 0.
* `python3 tools/gates.py regen`: "ok   regen (exit 0)".
* `cd cmd/generate && go test -race -count=1 -shuffle=on ./...`: every package `ok`.
* `make pre-add-check FILES="<the phase's Go files and engine cases>"`: exit 0, "12 file(s) gofmt-clean; both modules linted,
  vetted and tested (-race -shuffle=on, uncached), tidy, govulncheck."
* `make check-records`, `make markdownlint`, `make disclosure-check` and `tools/idscan.py` over the changed files: exit 0 each.

**Status:** two deviations, recorded above. Staged for the owner, who commits. Phase 3, G9, G27, G28 and G10, begins on the
owner's word.

### Phase 3: G9, G27, G28 and G10, the output edge's lows, 2026-11-07

The owner committed phase 2 as `1a8d09c`, and said "committed, proceed".

**Deviation 1, 2026-11-07: G9's other platforms and the importer.**

* **Found:** step 3.1 type-checks the other platforms' file lists in process. `Check`'s one importer,
  `importer.ForCompiler(fset, "source", nil)` (`output/check.go`, `sharedImporter`), resolves every import, the standard
  library's included, for the host, and `go/importer` takes no build context. A correct `windows`-only file that uses
  `windows`-only API of the standard library, such as `syscall.Handle`, would be refused.
* **Today:** `go list` with `GOOS` `darwin`, `linux` and `windows` lists the same 18 files for the root, so only the host's
  check would run.
* **Decision:** the owner chose the recommended option. When a platform's file lists differ from the host's, `Check` compiles
  the package and its tests for that platform with the go command, `go test -c` under `-overlay`, and its error names the
  platform. Not chosen: refusing platform-specific hand-written files; the in-process check with the host's importer.
* **Scope:** no file added.
* **The MADR:** not amended. G9's fix, "also check for `windows` and `linux`", holds.

**What changed:**

* **G9 (deviation 1).** `Check` (`cmd/generate/internal/output/check.go`) lists the hand-written files for the host, then for
  `linux/amd64` and `windows/amd64`, by `listFor` and `handWritten`, which now takes the platform. A platform whose lists equal
  one already checked is skipped. Today all three are equal, so only the host is checked. Otherwise `compileFor` runs
  `go test -c -o <temporary directory>/ -overlay <file> .` with `GOOS`, `GOARCH` and `CGO_ENABLED=0`, adding `./unstable` when
  package unstable is checked.
  * The overlay maps each rendered file's path to its bytes in a temporary file, and each stale file to "", deleted.
  * Its paths are resolved through symbolic links, as the go command resolves its working directory. Without that the overlay
    matched nothing on macOS, whose temporary directories sit behind `/var`'s link. `TestCheckOtherPlatforms` passed
    silently on the first run because of it.
  * An error reads "generated code does not compile for windows/amd64 with <dir>: …", with the go command's output.
* **G27.** `handWritten` returns `XTestGoFiles` too, and `checkHost` type-checks the external test package against the package
  as tested. A new `moduleImporter` type-checks from source a package of the module under the output directory that an
  external test imports, such as `acptest`, against that same root. The root's 17 external test files and `acptest` are now
  checked on every run of the generator.
* **G28.** A new exported `output.FindGoMod` walks up from a directory to the `go.mod` that holds it. `goVersion` reads the
  `go` line from it, and `rootImportPath` (`cmd/generate/main.go`) takes the module path from it.
* **G10.** `TestGeneratedFilesUpToDate` (`cmd/generate/main_test.go`) runs its stale-file loop over every directory of
  `emit.Dirs`, the root included, for a `*_gen.go` or `*_gen_test.go` the run does not render whose first line is the
  generated header.

**Tests,** in `output`:

* `TestCheckOtherPlatforms`. A `x_windows.go` declaring the generated `U` is refused "redeclared", naming `windows/amd64` on a
  host that is not Windows. A `x_windows.go` using `syscall.Handle` is accepted.
* `TestCheckExternalTests`. An external test using the render through `acptest`, with `*acp.RequestError` shared, is accepted;
  one naming `acp.ToolKindSearch` is refused "undefined: acp.ToolKindSearch".
* `TestCheckVersionFromEnclosingModule`. `go 1.21` in a parent `go.mod` and `for range 3` in `sub/` are refused "requires go1.22
  or later".
* `TestCheckSelectsLikeTheGoCommand` (0019's G15) required an external test file with an undefined name to stay out of the
  check, the rule G27 reverses. Its `hand_x_test.go` is now a correct external test, `acp.F` through `import "acp"`, which
  must pass. `TestCheckExternalTests` holds the refusal.

**Deviation 2, 2026-11-07: two of phase 1's tests on Windows.**

* **Found:** `tools/remote.py tree --host windows`, run in this phase for G9, failed `TestWriteNamesRemovedFiles` and
  `TestWritePackageDirectories` in `cmd/generate/internal/output`. Every other package passed there, and the Linux host passed
  every package. `Write` returns each removed file's `emit.File.Path`, which joins with "/" on every OS
  (`cmd/generate/internal/emit/file.go:147`). The two tests, committed in phase 1's `5e0761b`, expected `filepath.Join`'s
  `unstable\gone_gen.go`.
* **The code is right:** `staleIn` splits with `filepath.Split`, and `os.Remove` takes the slash form, both on Windows.
* **Decision:** the owner chose the recommended option. The tests expect `emit.UnstableDir + "/" + name`, the form `Write`
  returns on every OS. Not chosen: `Write` returning native paths; comparing through `filepath.ToSlash`.
* **Scope:** `cmd/generate/internal/output/write_test.go` joins this phase.
* **After:** `tools/remote.py tree --host windows`: exit 0, every package of both modules `ok`.

**Seen to fail,** by `plants_gen.py 3`, each break in its own copy of the working tree:

* G9, no other platform compiled: "check_test.go:291: Check = <nil>, want it to refuse U redeclared".
* G9, the overlay's paths left unresolved: the same line.
* G27, the external tests left out: "check_test.go:330: Check = <nil>, want it to refuse undefined: acp.ToolKindSearch".
* G28, `FindGoMod` not walking up: "check_test.go:348: Check = <nil>, want it to refuse the range over an int under go
  1.21".
* G10, a stale `extra_gen.go` with the header written at the root of the copy: "main_test.go:70: extra_gen.go is a generated
  file run would remove".

All as required.

**Checks:**

* `cd cmd/generate && go run .`: exit 0. `git diff --exit-code -- '*_gen.go' '*_gen_test.go' unstable/`: exit 0.
* `python3 tools/gates.py regen`: "ok   regen (exit 0)".
* `cd cmd/generate && go test -race -count=1 -shuffle=on ./...`: every package `ok`.
* `tools/remote.py tree`, both modules: Linux, exit 0; Windows, exit 0 after deviation 2.
* `make pre-add-check FILES="<the five Go files>"`: exit 0, "5 file(s) gofmt-clean; both modules linted, vetted and tested
  (-race -shuffle=on, uncached), tidy, govulncheck."
* `make check-records`, `make markdownlint`, `make disclosure-check` and `tools/idscan.py` over the changed files: exit 0 each.

**Status:** two deviations, recorded above. Staged for the owner, who commits. Phase 4, G4, G5, G7, G8, G32 and G37, begins on
the owner's word.

### Phase 4: G4, G5, G7, G8, G32 and G37, the model's refusals, 2026-11-07

The owner committed phase 3 as `5124800`, and said "committed, proceed".

**Design rule 6's scan,** by a scratch script, `facts4.py`, over `schema/schema.json`, `schema/schema.unstable.json` and
`schema/unstable-v1.json`, by the model's placements:

* `not`, 4 stable and 6 unstable, and `unevaluatedProperties`, 1 and 1, all on a direct `anyOf` member of a definition; none
  elsewhere;
* a `const` or `enum` on an object property, a bound under `items` or `additionalProperties`, a keyword beside `$ref`, a
  definition that is only a `$ref`, an unknown `type`, an integer bound of 2^53 or more, a default below a nested object or a
  variant, a definition that is `null`: 0 each;
* duplicate keys: 0 in each file; `meta.json` and `meta.unstable.json`: `version` 1.

**The engine cases,** by `facts4e.py` over the 205 cases: only two refusal cases hold a shape the phase refuses.

* `refuse-deprecated-under-not.txtar` has `not` on a definition. It keeps its error, since the new keyword checks run after
  `checkDeprecated`.
* `refuse-unstable-null-definition.txtar` has a `null` definition in the unstable schema, step 4.5's case.

**Deviation 1, 2026-11-07: G4 against 0019's bounds.**

* **Found:** step 4.1 refuses a fractional bound, and the MADR's G4 fix refuses a magnitude of 2^53 or more. Yet the committed
  `accept-validate-fractional-bound.txtar`, which the step says stays unchanged, accepts both: an `int64` `minimum` of 1.5,
  rounded inward (0019-PLAN-generator-hardening.md, G12), and a `uint64` `maximum` of `1e19`, written as a `uint64` (G21).
* **Decision:** the owner chose the recommended option: read the bounds exactly. Not chosen: refusing 2^53 or more, which
  reverses 0019's G21 and splits that case; refusing a bound a `float64` does not hold exactly.
* **Scope:** `cmd/generate/internal/model/fields.go` (`Facts`), `model/validate.go` and its `Check`, `emit/types.go`
  (`boundLit`), and the tests that read the bounds, `load/load_test.go` and `model/fields_test.go`, join the step.
* **The MADR:** G4's fix is amended, 2026-11-07.

**What changed:**

* **G4 (deviation 1).**
  * `load.Definition`'s `Minimum` and `Maximum`, and `model.Facts`', are `*json.Number`.
  * `bounds` (`cmd/generate/internal/model/validate.go`) computes an integer type's bound with `math/big`: `ceilRat` and
    `floorRat` round a fraction inward, and `exactRange` gives each sized type's range exactly. A `number` type's bound is
    parsed to a `float64`, as before.
  * `Check` gains `BoundText`, the exact integer, and `boundLit` (`emit/types.go`) writes it. Its two branches that wrote an
    integer from the `float64`, the `math.MaxInt64` one included, are gone, since every integer check now carries the text.
* **G5,** a new `checkKeywords` in `model/schema.go`, run by `checkRefs` after `checkDeprecated`:
  * (a) `not` or `unevaluatedProperties` anywhere but a direct `anyOf` or `oneOf` member of a definition. And in
    `buildUnion` (`model/model.go`), either one on a variant that is not the union's catch-all, which alone reads `not`: "U
    variant 1: not or unevaluatedProperties on a variant that is not the union's catch-all, which nothing reads";
  * (b) `const` or `enum` on a property of type `object`;
  * (c) `minimum`, `maximum` or `minLength` at any depth under `items` or `additionalProperties`;
  * (d) any keyword beside `$ref` but `description`, `title` and `deprecated`, listed by a new `refSiblings`;
  * (e) a definition that is only a `$ref`;
  * (f) a `type` that is not one of JSON Schema's seven names;
  * (g) in `load`, a new `duplicateKey`, a walk of `json.Decoder.Token`, run by `parseSchema`, `parseMeta` and `ParseManifest`
    on the whole file. It leaves JSON that does not parse to the decoder, so those files' errors are as they were.
* **G7.** `checkDefaults` (`model/fields.go`) refuses a default on a field of a nested struct or an inline variant, the
  fields `plainFields` builds: "OuterInner.flag: a default below a nested object or in an inline variant, which is not
  applied".
* **G8.** `parseMeta` refuses a `version` below 1: "parse meta.json: version 0; want 1 or more".
* **G32.** `parseSchema` refuses a definition whose value is `null`: "parse schema.json: definition "X" is null, not a schema".
  `refuse-unstable-null-definition.txtar`, step 4.5's case, pinned merge's later "unstable schema missing definition
  "Ghost"". It now meets this refusal first, and its `want.err` and comment say so. Merge's check still guards a schema built
  in memory.
* **G37.** `admitsNull` (`model/schema.go`) also answers true for an `allOf` whose members all admit `null`, an `enum` that
  holds `null`, the boolean schema `true`, and a schema with no `type`, `$ref`, `const`, `enum`, `anyOf`, `oneOf` or `allOf`.
  `"const": null` decodes as no `const`, so it lands in the last case, which answers true.

**Tests:**

* Engine cases, sixteen new:
  * G4: `accept-validate-exact-bound.txtar`, whose behaviour test requires 2^53+1 to pass `Validate` and 2^53+2 to fail;
    `refuse-bound-uint64-range.txtar`;
  * G5: `refuse-not-keyword.txtar`, `refuse-not-on-variant.txtar`, `refuse-object-property-const.txtar`,
    `refuse-item-bound.txtar`, `refuse-ref-sibling.txtar`, `refuse-ref-only-definition.txtar`, `refuse-unknown-type.txtar`
    and `refuse-duplicate-key.txtar`;
  * G7: `refuse-default-nested-inline.txtar` and `refuse-default-inline-variant.txtar`;
  * G8: `refuse-meta-no-version.txtar`;
  * G32: `refuse-definition-null-value.txtar`;
  * G37: `refuse-skip-invalid-allof-nullable-items.txtar` and `refuse-skip-invalid-untyped-items.txtar`.

  `accept-validate-fractional-bound.txtar` and every other committed case pass unchanged, but step 4.5's.
* Unit tests:
  * `load`: `TestParseSchemaRefusesNullDefinition`; `TestParseMetaRequiresVersion`; `TestDuplicateKeys`, at the top level,
    nested, in an array, and through each of the three parsers, with the controls of two objects sharing a key, JSON that does
    not parse, an array and a string;
  * `model`: `TestAdmitsNull`, sixteen schemas, with a reference cycle among the controls.
* Three committed tests pinned behaviour this phase replaces. Each now holds the new rule, its purpose kept:
  * `TestParseSchemaKeepsModeledKeywords` (`load/load_test.go`) required `"Z": null` to decode as a nil definition. `Z` is
    gone from it, and `TestParseSchemaRefusesNullDefinition` holds G32. Its bounds compare as `json.Number` text.
  * `TestCheckTypable` (`model/schema_test.go`) had the `not` on its union definition. The `not` is on a variant now, where a
    catch-all reads it, and the test still requires the inline objects inside a `not` to go untyped.
  * `TestReferenceWithPropertiesIsNotNested` (`model/edges_test.go`) required a `$ref` with `type` and `properties` beside it
    to be the reference, the siblings dropped, the very shape G5 (d) names. It is `TestReferenceWithPropertiesIsRefused`, and
    requires "T.properties.p: type, properties beside $ref is not read".
  * `model/fields_test.go` compares its bounds as `json.Number` text.

**Seen to fail,** by `plants_gen.py 4`, each break in its own copy of the working tree:

* G4:
  * the literal written from the `float64`: "behaviour_test.go:8: 2^53+1: T: x: greater than maximum 9007199254740992, want
    nil";
  * the exact range check removed: the case fails, as the type checker refuses the generated comparison with 2^64,
    "generated code does not type-check with …". The first run expected "run accepted the schema", and the plant's expected
    text was corrected.
* G5:
  * each of (a), on a property and on a variant that is not the catch-all, and (b), (c), (e) and (f): "engine_test.go:47: run
    accepted the schema";
  * (d): that line, and "edges_test.go:99: Build = <nil>, want the keywords beside the $ref refused";
  * (g): that line, and "load_test.go:272: top level: duplicateKey = <nil>, …".
* G7: "run accepted the schema", for each of the two cases.
* G8: that line, and "load_test.go:250: parseMeta({"agentMethods":{},"clientMethods":{}}) = <nil>, …".
* G32: "error "model: O.properties.x: $ref "#/$defs/X" names no definition" does not contain …", the old cause, and
  "load_test.go:239: parseSchema = <nil>, …".
* G37:
  * `allOf` not followed: "run accepted the schema", and "schema_test.go:223: admitsNull({"allOf":[{"$ref":"#/$defs/Maybe"}]})
    = false, want true".

    The first run found a gap. With the `allOf` branch removed, the case still passed, because the last case, a schema that
    says nothing, did not exclude `allOf` and so answered true for an item that is only an `allOf`. The step lists `allOf`
    among the keywords that case requires to be absent. The code now checks for `allOf` there, and the re-run failed as
    above.
  * an `enum` holding `null` not read: "admitsNull({"enum":["a",null]}) = false, want true";
  * the last case removed: "run accepted the schema", and "admitsNull({}) = false, want true".

All as required.

**Checks:**

* `cd cmd/generate && go run .`: exit 0. `git diff --exit-code -- '*_gen.go' '*_gen_test.go' unstable/`: exit 0.
* `python3 tools/gates.py regen`: "ok   regen (exit 0)".
* `cd cmd/generate && go test -race -count=1 -shuffle=on ./...`: every package `ok`.
* `make pre-add-check FILES="<the phase's Go files and engine cases>"`: first exit 2. `golangci-lint` found `duplicateIn`'s
  unchecked type assertion (errcheck), and a third `"properties"` beside the constant `kwProperties` (goconst), in
  `refSiblings`, which now uses the `kw` constants. Then exit 0, "11 file(s) gofmt-clean; both modules linted, vetted and
  tested (-race -shuffle=on, uncached), tidy, govulncheck." The G5 (g) plant was re-run after the change, as required.
* `make check-records`, `make markdownlint`, `make disclosure-check` and `tools/idscan.py` over the changed files: exit 0 each.

**Status:** one deviation, recorded above. Staged for the owner, who commits. Phase 5, G6, G21, G22, G23 and G24, begins on
the owner's word.

### Phase 5: G6, G21, G22, G23 and G24, types and decoding, 2026-11-07

The owner said "Commit to main, then proceed": phase 4 was committed on `main`, with `git commit --no-edit`, as `ac41197`.

**What changed:**

* **G6 (Q22 a).** A new pass, `pointerScalarDefaults` (`cmd/generate/internal/model/fields.go`), runs in `Build` between
  `checkDefaults` and `renderDefaults` (`model/model.go`).
  * It gives an optional, non-nullable field whose schema default is a scalar other than its zero value a pointer type, when
    `admitsValue` finds the zero value admitted. `admitsValue` follows `$ref` and an `allOf` of one through `resolveSchema`,
    and reads a `const`, an `enum`, or a union of consts.
  * Step 5.1 placed the change in `objectFields`, which has no schema to follow a `$ref` with. The pass sits beside it, in
    the same file, and changes the same fields.
  * The rendered default is then the pointer's, `new(...)`, as a nullable field's already is.
* **G21 (Q26 a).** A new `discConst` (`model/schema.go`) is a property's `const`, or the one value of a one-value `enum`.
  `discriminatorKey`, `discriminatorValue` (`model/model.go`) and `isCatchAll` read the discriminator through it.
* **G22.**
  * `isStringConstUnion` reads the members of a `oneOf`, or of an `anyOf` when there is no `oneOf`, by a new
    `constMembers`, so an `anyOf` of string consts alone is `KindStringConstUnion`, a closed enumeration that `Validate`
    checks. `enumValues` reads the same members.
  * `isOpenStringEnum` also requires a member without a `const`. Since `classify` tests `isStringConstUnion` first, that
    guard is never reached by an `anyOf` of consts alone; it states the rule where the open enum is defined.
* **G23 (Q27 a).** Both `Fill` assignments (`model/fields.go`, `objectFields` and `plainFields`) also require
  `!includesNull(prop)`.
* **G24.** `unknownAbsent` (`emit/types.go`) assigns the default, by a new `orNil`, in the `RefPointer` case and in the
  `RefSlice` case without skip-invalid items; `nil` when there is none.

**Tests:**

* Engine cases:
  * `accept-default-scalar.txtar`: `Mode *string`, `N *int32` and `On *bool`, by `-update`. Its behaviour test reads them
    through the pointers, and `TestDefaultScalarSendsZero` requires `{"mode":"","n":0,"on":false}` to re-encode with all
    three keys.
  * `accept-default-scalar-excluded-zero.txtar`, new: `mode`, an enumeration without `""`, and `kind`, a `oneOf` of the one
    const `"object"` as the committed `ElicitationSchemaType` is, keep `Mode Mode` and `Kind Kind`.
  * `accept-enum-discriminator.txtar`, new: the circle and the square route, with `Unknown` empty.
  * `accept-anyof-const-only.txtar`, new: `type E string` with `EA` and `EB`; `Validate` passes `a` and fails `c`.
  * `accept-required-nullable-collection.txtar`, new: `{"xs":null}`, `{"xs":[]}` and `{"xs":["a"]}` each re-encode as they
    came.
  * `accept-default-on-error.txtar` gains `kinds`, an array of `K {a, b}` with default `["a"]`, and `kp`, a nullable `K`
    with default `"a"`. `TestDefaultOnErrorUnknownValue` requires `["zzz"]`, `["b","zzz"]` and `5` each to give `[a]` and
    `a`, and the known `b` to stay.
* G6 reaches four committed tests, each now holding the pointer, its inputs and purpose kept. Step 5.1 named the first only.
  * `accept-folded-keys.txtar`'s `Item.count`, an optional `int64` with default 3, is `*int64`. `TestFoldedObjectKeys`
    compares `Id` and `*Count` for the same four inputs and values.
  * `TestObjectFields` (`model/fields_test.go`): `mode` is `*string`.
  * `TestRenderDefaults` (`model/defaults_test.go`): `on`, `name`, `n` and `f` render as pointers. `mode`, whose enumeration
    excludes `""`, stays a constant, and the zero defaults stay zero.
  * `TestLenientDecoder` (`emit/types_test.go`): `a.Mode = orDefault[*string](s.Mode, new("auto"))`.

**Seen to fail,** by `plants_gen.py 5`, each break in its own copy of the working tree:

* G6:
  * the pass not run: `want.api` differs at "type Opts struct{Mode string …; N int32 …; On bool …}", and
    "fields_test.go:117: field 4: got {mode Mode string …}";
  * the zero value's exclusion not read: "type Opts struct{Kind *Kind …; Mode *Mode …}".
* G21, the one-value enum not read: "behaviour_test.go:12: circle decoded as {Circle:<nil> Square:<nil>
  Unknown:{"kind":"circle","r":1}}, <nil>".
* G22, both changes reverted: "accept-anyof-const-only/behaviour_test.go:7:24: (&O{…}).Validate undefined (type *O has no
  field or method Validate)". As an open enum, `E` gave `O` nothing to validate. The first run expected the test's own
  message, and the plant's expected text was corrected.
* G23, `Fill` back: "behaviour_test.go:17: {"xs":null} re-encoded as {"xs":[]}, <nil>".
* G24, `nil` back: "behaviour_test.go:43: {"kinds":["zzz"],"kp":"zzz"} decoded as kinds [], kp <nil>; want [a] and a".

All as required.

**Checks:**

* `cd cmd/generate && go run .`: exit 0. `git diff --exit-code -- '*_gen.go' '*_gen_test.go' unstable/`: exit 0.
  `ElicitationSchema.Type` keeps its type.
* `python3 tools/gates.py regen`: exit 0.
* `cd cmd/generate && go test -race -count=1 -shuffle=on ./...`: every package `ok`.
* `make pre-add-check FILES="<the phase's Go files and engine cases>"`: exit 0, "7 file(s) gofmt-clean; both modules linted,
  vetted and tested (-race -shuffle=on, uncached), tidy, govulncheck."
* `make check-records`, `make markdownlint`, `make disclosure-check` and `tools/idscan.py` over the changed files: exit 0 each.

**Status:** no deviation. Staged for the owner, who commits. Phase 6, G19, G20, G25 and G26, begins on the owner's word.

### Phase 6: G19, G20, G25 and G26, validation, gates and rules, 2026-11-07

The owner committed phase 5 as `624edc1`, and said "committed, proceed".

**Deviation 1, 2026-11-07: G20's two halves.**

* **Found:**
  * Step 6.2 has `resolveKeyOrders` refuse a key-order rule whose definition holds the property while its fields lack it.
    It already did: `cmd/generate/internal/model/model.go`'s `resolveKeyOrders` reads the property through `expandAllOf`, and
    with no such field returns "a key-order rule needs a property of an object". A scratch script, `g20_probe.py`, ran a case
    with ACP's own rule's `ElicitationSchema` written as a union holding `properties`, on a `git archive` copy of `624edc1`.
    The case passed, refused. So `refuse-key-order-property-in-copies.txtar` cannot be seen to fail.
  * The capability half's case cannot be an engine case. Engine cases run with ACP's own table (`cmd/generate/main.go:61`),
    which has no capability rule on a property that only copies hold.
* **Decision:** the owner chose the recommended option.
  * The capability refusal is tested by a `model` unit test with a synthetic rule, seen to fail.
  * The key-order case is kept, as a guard of the existing refusal, recorded as passing at the base.
  * The MADR's G20 is amended.

  Not chosen: rewording the key-order refusal to name the copies; dropping the key-order case.
* **Scope:** `cmd/generate/internal/model/gates_test.go` was in scope; no file added.

**Deviation 2, 2026-11-07: G20's second shape.**

* **Found:** the MADR's G20 has two probes. A rule on a union whose property only its variants hold is Q28 (a)'s, and
  `gateCheck` now refuses it. The other is a definition copied into a union's inline variant by an `allOf`: the rule gates
  the definition's own field and not the copy, so a payload of that variant is not gated. Q28 (a), "a rule whose property the
  definition's own fields lack", is worded for the first alone.
* **Today:** a scratch script, `g20_copies.py`, listed in a copy of the working tree the structs holding each property of
  ACP's capability rules. Each of the 11 rules with a property finds it in its own definition alone.
* **Decision:** the owner chose the recommended option. `gateCheck` also refuses a rule whose property is a field of another
  struct too, naming the copies the rule does not reach. Not chosen: leaving the shape open, recorded; gating the copies, as
  Q28 (b) would have.
* **Scope:** no file added. `TestCapabilityRuleOnCopiedProperty` holds both shapes.
* **The MADR:** G20 is amended, 2026-11-07.

**Deviation 3, 2026-11-07: G26's reserved names and the model tests' fixture.**

* **Found:** with the reserved-method check, 22 `model` tests failed "model: agent method "s/close": it becomes Close, a
  method the connections already have". Their shared fixture, `protocolDefs` (`model/protocol_test.go:21-23`), declares
  `CloseRequest` and `CloseResponse` for the wire `s/close`. A method's Go name is its request type's less `Request`
  (`model/methods.go:222`), so the fixture's method is `Close`, which rendered would collide with the connections' own
  `Close()`. Committed ACP passes: its `session/close` is `CloseSession`, and the regeneration was byte-identical.
* **Decision:** the owner chose the recommended option. The fixture's types are `CloseSessionRequest` and
  `CloseSessionResponse`; the wire `s/close`, and every rule keyed by it, stay. Not chosen: exempting fixtures from the check;
  dropping the reserved names.
* **Scope:** `model/protocol_test.go` and `model/deprecation_test.go`, which names `CloseRequest` four times.

**Deviation 4, 2026-11-07: G26's key-order case and the engine harness.**

* **Found:** step 6.4's `refuse-key-order-field-method.txtar` cannot be written. Engine cases run with ACP's own table
  (`cmd/generate/main.go:61`), whose one key-order rule's `Field` is `PropertyOrder`.
* **Decision:** the owner chose the engine-harness option, not the recommended unit test. A case may carry `protocol.json`, an
  object whose keys name fields of `model.Protocol`. `runEngineCase` and the behaviour harness give the case a copy of
  `model.ACP` with each named field replaced by a fresh value decoded from it, so `model.ACP` is never changed under the
  parallel cases. `run` becomes `runWith(args, protocol)`, and `run` passes `&model.ACP`. Not chosen: a `newRules` unit
  test; a `Build` test with a custom `Protocol`.
* **Scope:** `cmd/generate/main.go`, `cmd/generate/engine_test.go`, `cmd/generate/behaviour_test.go`, and
  `docs/architecture.md`'s paragraph on the engine cases, all in the generator module or the docs the PLAN names.
* **The MADR:** not amended for either.

**What changed:**

* **G19 (Q25 a):**
  * A new `collectionItems` (`cmd/generate/internal/model/gates.go`) names the item type of an `InlineNamed` slice or map
    variant, or reports one whose items are not a named type.
  * `buildValidations` (`model/validate.go`) sets a new `Variant.ItemsValidate` (`model/model.go`) when the item type
    validates, and refuses a deeper shape holding a validated type, as `nested` does for a field.
  * `unionValidateBody` (`emit/types.go`) then calls `nestedStmt` on `(*u.Field)` for that variant.
  * The gates: `dispatchReach` adds the item type; `gateHolder.nested` gives the variant a `GateCheckNested` with the new
    `Items` and the collection's shape; a deeper gated shape is refused; and `nestedGate` (`emit/gates.go`) calls each item's
    gates when the variant is set.
* **G20 (Q28 a; deviations 1 and 2):** `gateCheck` refuses a rule whose definition holds the property while its fields lack it,
  "the property is in copies (…), not a field of …", and a rule whose property is also a field of another struct, "the
  property is also in copies (…), which the rule does not reach". A new `copiesOf` names them, by a new `fieldsHolding`.
  `resolveKeyOrders` is unchanged.
* **G25:**
  * `plainFields` sets `deprecationOf(prop)` on a copy.
  * `deprecationsOf` (`model/protocol.go`) gives a property's note to every field built from it, by `fieldsHolding`.
  * `applyFrozenDeprecations` marks a frozen field's copies. Its per-type marking moved into a new `markFrozenType`, as
    `gocognit` counted 53 against its limit of 50 after the change; the behaviour is as it was.
* **G26 (deviations 3 and 4):**
  * `buildMethods` (`model/methods.go`) refuses a method that becomes one of the connections' own methods, a new
    `connectionMethods`, and two methods of a side, of any binding, that become one outbound method.
  * `newRules` (`model/protocol.go`) refuses a key order's `Field` named as a generated method (`checkNotMethod`) or as
    another rule's on the same definition, and an `Interface.Var` that is not an identifier or is one of a new
    `dispatchLocals`.
* **The harness (deviation 4):** `run` calls a new `runWith(args, protocol)` with `&model.ACP`; `renderUnstable` takes the
  protocol. `runEngineCase` and the behaviour harness run a case with `caseProtocol` (`cmd/generate/engine_test.go`), and do
  not write `protocol.json` into the schema directory. `docs/architecture.md`'s paragraph on the engine cases says so.

**Tests:**

* Engine cases, new:
  * `accept-union-array-variant-validates.txtar`: the same short `Option` name fails `Validate` in a field, in the
    `Ungrouped` variant and in the `Grouped` variant's `Group`, and a good one passes;
  * `refuse-key-order-property-in-copies.txtar`, which passes at the base (deviation 1);
  * `refuse-method-name-reserved.txtar`, a method `x/close` that becomes `Close`;
  * `refuse-method-name-across-bindings.txtar`, written with `protocol.json`;
  * `refuse-key-order-field-method.txtar` and `refuse-key-order-field-twice.txtar`, with `protocol.json`.
* `accept-deprecated.txtar` gains `Base`, with a deprecated `old`, and `W`, whose `Wrap` variant is an `allOf` of `Base` with
  its own `kind`. Its `want.deprecated` gains "field WWrap.Old: … the "old" property of the "w" variant of the W schema
  definition deprecated."
* Unit tests:
  * `model`: `TestGatesDescendIntoArrayVariant`; `TestCapabilityRuleOnCopiedProperty`, both shapes;
    `TestDeprecationNoteReachesCopies`; three `TestProtocolTableRefuses` cases for `Var`, `"1x"`, `"ctx"` and `"_"`;
  * `emit`: `TestNestedGateItems`, a slice and a map.
* The `model` tests' fixture, per deviation 3: `CloseSessionRequest` and `CloseSessionResponse` in `protocol_test.go`, and
  their twelve mentions in `deprecation_test.go`.

**A probe shape, found:** the MADR's G26 probe, a stable `FooRequest` beside an unstable `FooNotification`, does not collide
at `624edc1`. A scratch script, `g26_probe.py`, generated that schema: the live merge prefixes unstable definitions, so the
methods are `Foo` and `UnstableFoo`. The collision is reached through the optional interface, which `protocol.json` binds
`x/foo` to in the case. The MADR's G26 records it.

**Not tested:** `markFrozenType`'s marking of a frozen field's copies. It needs a manifest that freezes a property an `allOf`
copies into an inline variant, and no case holds one. The regeneration check shows nothing committed reaches it.

**Seen to fail,** by `plants_gen.py 6`, each break in its own copy of the working tree:

* G19:
  * items not validated: "behaviour_test.go:19: the Ungrouped variant: Validate = <nil>, want the short name refused";
  * the gates not descending: "gates_test.go:288: Verdicts: [], want one nested check of the Many variant's items";
  * `nestedGate`'s branch removed: "ast_test.go:66: … for i := range v.Many { … } lacks "if v.Many != nil {"".
* G20:
  * a union's property: "gates_test.go:325: a union's property: Build = <nil>, want "capability rule
    LoadSessionRequest.dirs: the property is in copies (""";
  * the copies: "a property an allOf copies: Build = <nil>, …".
* G25:
  * the copy's mark: `want.deprecated` differs at "field WWrap.Old: …";
  * the note to the definition's field alone: "deprecation_test.go:193: the copy in W's Wrap: … want the note".
* G26:
  * the reserved names, and one outbound name per side: each case fails as at the base, `Check` refusing "generated code
    does not type-check with … method … already declared";
  * the key order's field named as a method, and two rules of one field: each case fails, the type check refusing what the
    rule made;
  * `Var`: "protocol_test.go:385: Build accepted the table", for the three cases.
* The harness, `protocol.json` ignored: "engine_test.go:50: run accepted the schema".

The first run's expected texts for the two method-name plants and the harness plant were wrong: each run failed, but with the
type check's message, or with the case accepted. They were corrected, and the three re-run as above. All as required.

**Checks:**

* `cd cmd/generate && go run .`: exit 0. `git diff --exit-code -- '*_gen.go' '*_gen_test.go' unstable/`: exit 0.
* `python3 tools/gates.py regen`: exit 0.
* `cd cmd/generate && go test -race -count=1 -shuffle=on ./...`: every package `ok`.
* `make pre-add-check FILES="<the phase's Go files and engine cases>"`: first exit 2, `gocognit` on
  `applyFrozenDeprecations`, 53 > 50; after the split, exit 0, "17 file(s) gofmt-clean; both modules linted, vetted and
  tested (-race -shuffle=on, uncached), tidy, govulncheck."
* `make check-records`, `make markdownlint`, `make disclosure-check` and `tools/idscan.py` over the changed files: exit 0 each.

**Status:** four deviations, recorded above. Staged for the owner, who commits. Phase 7, G11, G33 and G31, begins on the
owner's word.

### Phase 7: G11, G33 and G31, the input edge and the CLI, 2026-11-07

The owner committed phase 6 as `08ad4c1`, and said "committed, proceed".

**What changed:**

* **G33.** `runWith` (`cmd/generate/main.go`) returns nil when `fs.Parse` returns `flag.ErrHelp`. `flag` has printed the
  usage, and `main` exits 0.
* **G11.** `runWith` refuses `fs.NArg() > 0` before anything is read or written: "unexpected argument "stray"; use -schema and
  -out", which `main` prints after "generate:".
* **G31.** `ManifestMethods` and `ManifestChange` (`cmd/generate/internal/load/manifest.go`) gain an `UnmarshalJSON` through
  `decodeStrict`, on an alias type, as `Discriminator` has.

**Tests:**

* `main_test.go`: `TestRunRefusesArguments`, `stray` alone and after `-out x`; `TestRunHelp`, `-h`, `-help` and `--help`.
  `flag` prints its usage to the test's stderr.
* `load`'s `TestManifestNestedKeysStrict`: a change's `PROPERTIES`, a side's `AGENT`, and a change's unknown `props` are each
  refused, naming the key; a well-cased manifest decodes.
* The built command, by hand: `generate -h` printed "Usage of generate:" and exited 0; `generate stray` printed "generate:
  unexpected argument "stray"; use -schema and -out" and exited 1.

**Seen to fail,** by `plants_gen.py 7`, each break in its own copy of the working tree:

* G11, the check removed: "main_test.go:450: run(["stray"]) = <nil>, want the argument refused". The run went on to generate
  into the copy.
* G33, `flag.ErrHelp` a failure again: "main_test.go:461: run(-h) = flag: help requested, want nil".
* G31, each `UnmarshalJSON` renamed away: "manifest_test.go:245: a change's key: ParseManifest = <nil>, want an error naming
  "PROPERTIES"", and "a side of the methods: ParseManifest = <nil>, want an error naming "AGENT"".

All as required.

**Checks:**

* `cd cmd/generate && go run .`: exit 0, so the committed manifest decodes strictly.
  `git diff --exit-code -- '*_gen.go' '*_gen_test.go' unstable/`: exit 0.
* `python3 tools/gates.py regen`: exit 0.
* `cd cmd/generate && go test -race -count=1 -shuffle=on ./...`: every package `ok`.
* `make pre-add-check FILES="<the four Go files>"`: exit 0, "4 file(s) gofmt-clean; both modules linted, vetted and tested
  (-race -shuffle=on, uncached), tidy, govulncheck."
* `make check-records`, `make markdownlint`, `make disclosure-check` and `tools/idscan.py` over the changed files: exit 0 each.

**Status:** no deviation. Staged for the owner, who commits. Phase 8, G29, G35, G36 and G12, begins on the owner's word.

### Phase 8: G29, G35, G36 and G12, the table's tests and the fuzzer, 2026-11-07

The owner committed phase 7 as `78f72e9`, and said "committed proceed".

**Step 8.1's listing,** by a scratch script, `g29_list.py`, a test in a copy of the working tree over the committed model's
schema:

* the capability wording: 8 request definitions, `CloseSessionRequest`, `DeleteSessionRequest`, `ListSessionsRequest`,
  `LoadSessionRequest`, `ReadTextFileRequest`, `ResumeSessionRequest`, `UnstableForkSessionRequest` and `WriteTextFileRequest`,
  each gated by the method rule of its `x-method`;
* the send wording: `SessionUpdate`'s variants 13, 14 and 15, and the payload types `CompactionUpdate` and
  `CompactionSummaryChunk`, each gated by a variant capability rule of `SessionUpdate`;
* the absolute-path wording: the `cwd` of 7 definitions, each with a path rule.

No carrier lacks a rule.

**Deviation 1, 2026-11-07: which rule the send wording asks for.**

* **Found:** step 8.1 has the send wording, "MUST only send this update", require a method gate. Its committed carriers are
  `SessionUpdate`'s variants and their payload types, which variant capability rules gate; no method rule could.
* **Decision:** the owner chose the recommended option. The capability and the send wordings each require any applicable
  gate, and the absolute-path wording a path rule. Not chosen: a variant capability rule for the send wording alone; dropping
  that scan.
* **Scope:** no file added.
* **The MADR:** not amended. G29's fix, "the same wording scans for these three rule kinds", holds.

**Deviation 2, 2026-11-07: the fuzz corpus.**

* **Found:** with `FuzzBuild` taking a meta and a schema, `go test -run ^FuzzBuild$ .` failed: "fuzz_test.go:27:
  "testdata/fuzz/FuzzBuild/2e4f0f4e77985ba7": wrong number of values in corpus entry: 1, want 2". The entry, a schema alone,
  was committed in `77d053b`, a key-folding regression.
* **Decision:** the owner chose the recommended option. The entry is rewritten in the two-value form, its meta `{}`, an empty
  `Meta` as it ran before, its schema unchanged. Not chosen: keeping the one-value target beside a new one; removing the entry.
* **Scope:** `cmd/generate/testdata/fuzz/FuzzBuild/2e4f0f4e77985ba7` joins the phase.
* **The MADR:** not amended.

**What changed:**

* **G29 (deviation 1).** `cmd/generate/internal/model/gates_test.go` gains `capabilityWording`, `sendWording` and
  `absPathWording`, and `wordingGaps`, which an index of the rules, `wordingIndex`, and a per-definition scan, `gapsOf`,
  carry out; a variant rule's union is found by `unionOf`. `checkCommittedWording` runs it on the committed model from
  `TestProtocolTableAppliesToCommittedSchema` (`model/protocol_test.go`), beside the other committed checks.
* **G36.** `methodByWire` (`model/protocol_test.go`) takes the side. Its eighteen callers pass theirs, and the committed table
  loop passes the rule's.
* **G35.** No code changes: a unit test and an engine case reach the two rule kinds.
* **G12 (deviation 2).** `FuzzBuild` (`cmd/generate/fuzz_test.go`) seeds each engine case's `meta.json` with its
  `schema.json`, by a new `seedPairs`, and builds with the meta, by a new `buildSeed`, which returns a decode error rather than
  nil. The corpus entry is in the two-value form.

**Tests:**

* G29: the committed scan passes. Step 8.1's listing is above.
* G35: `TestDeprecationNoteVariantWithoutDiscriminator` (`model/deprecation_test.go`), a note on `V`'s `Text`, with `Count`
  deprecated too, so a note that reached every variant without a discriminator value shows; `accept-empty-clears.txtar`, its
  rule in `protocol.json`, `Tags []string "json:\"tags,omitzero\""` beside `Notes` with `omitempty`. Its behaviour test sends
  an empty `Tags` as `[]` and omits a nil one.
* G36: `TestMethodByWireSide`, `mcp/message` from each side, and `session/prompt` not the client's.
* G12: `TestFuzzSeedsBuild`, every plain accepted case's seed built and rendered by `buildSeed`. The seed census, by a
  scratch script, `g12_census.py`, over the 230 seeds: with an empty meta, "accepted 1, refused as unlisted-method 137, refused
  otherwise 92"; with each case's meta, "accepted 123, refused as unlisted-method 1, refused otherwise 106". The refused are
  mostly the refusal cases. `go test -fuzz ^FuzzBuild$ -fuzztime 20s`: "execs: 736371", PASS.

**Seen to fail,** by `plants_gen.py 8`, each break in its own copy of the working tree:

* G29, the MADR's three sentences, each added to the copy's `schema/schema.json`:
  * "protocol_test.go:526: SetSessionModeRequest: its description asks for a gate, and no rule gates it";
  * "SessionUpdate variant 13: its description asks for a gate, and no rule gates it", for `usage_update`, which the message
    counts from 1;
  * "CreateTerminalRequest.command: its description asks for an absolute path, and no path rule names it".
* G29, the path rule on `CreateTerminalRequest.cwd` removed from `acp.go`: "CreateTerminalRequest.cwd: its description asks
  for an absolute path, and no path rule names it".
* G35:
  * the mutant `&&` to `||`: "deprecation_test.go:219: variant Count: &{Note:Use Count. …}, want the note """;
  * the empty-clears tag not set: `want.api` differs at "Tags []string "json:\"tags,omitempty\""".
* G36, the side ignored: "protocol_test.go:544: methodByWire(client, mcp/message) = &{Side:agent …}".
* G12, the empty meta back: "fuzz_test.go:102: accept-absolute-paths: Build: model: definition CancelRequestNotification:
  linked by x-method and x-side to protocol method "$/cancel_request", which no meta file lists".

The G29 and G12 plants were run again after the lint changes below, on the final code, with the same lines. All as required.

**Checks:**

* `cd cmd/generate && go run .`: exit 0. `git diff --exit-code -- '*_gen.go' '*_gen_test.go' unstable/`: exit 0.
* `python3 tools/gates.py regen`: exit 0.
* `cd cmd/generate && go test -race -count=1 -shuffle=on ./...`: every package `ok`.
* `make pre-add-check FILES="<the phase's Go files, the corpus entry and the engine case>"`: first exit 2, `gocognit` on
  `wordingGaps`, 55 > 50, and `nilerr` on `buildSeed`'s nil after a decode error. After the split into `wordingIndex` and
  `gapsOf`, and the decode error returned: exit 0, "5 file(s) gofmt-clean; both modules linted, vetted and tested (-race
  -shuffle=on, uncached), tidy, govulncheck."
* `make check-records`, `make markdownlint`, `make disclosure-check` and `tools/idscan.py` over the changed files: exit 0 each.

**Status:** two deviations, recorded above. Staged for the owner, who commits. Phase 9, G13, G30 and G34, begins on the
owner's word.
