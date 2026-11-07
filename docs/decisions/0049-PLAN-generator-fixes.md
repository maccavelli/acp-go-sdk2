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
