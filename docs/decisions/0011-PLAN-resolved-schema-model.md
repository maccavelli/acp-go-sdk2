---
status: in-progress
date: 2026-10-10
associated-madr: "0011-MADR-harden-code-generator-engine.md"
---

# Implement phase 1 of the generator hardening: the resolved schema model

Associated MADR: [0011-MADR-harden-code-generator-engine.md](0011-MADR-harden-code-generator-engine.md)

## Goal

Put one resolved, validated model between `internal/load` and `internal/emit`, so that every Go name, type, union shape, and method binding is decided
once, a schema the generator cannot represent is refused with a named error before any file is written, and the unstable types and methods are marked
from the merge's own data. Steps 2 to 7 leave all five generated files byte-identical; step 8 is the only step that changes them.

This is phase 1 of the six in the MADR. Phases 2 to 6 each get their own PLAN.

## Scope

In scope:

* `cmd/generate/main.go`, `cmd/generate/main_test.go`
* `cmd/generate/internal/load` (new keywords, strict decoding, merge provenance)
* a new package `cmd/generate/internal/model`, which absorbs `cmd/generate/internal/ir`; `ir` is deleted
* `cmd/generate/internal/emit` (reads the model; loses its naming, typing, and classification code)
* `cmd/generate/internal/util` (one camel-case splitter)
* in step 8 only, `types_gen.go`

Out of scope, each owned by a later phase of the MADR:

* phase 2: F1 unknown variants, F2 `ContentBlock` shapes, F3 precision, F5 error text;
* phase 3: the protocol table, the `ClientTerminal` branches, the helper skip list, `session/*` literals in `emit`;
* phase 4: using the integer formats, bounds, typed `additionalProperties`, and `x-deserialize-*` keywords this phase records; deeper `Validate`;
* phase 5: rendering in memory, `go/types` checking of the output, atomic writes, and the three layout panics in `emit/file.go` and `emit/types.go`
  (`printedSize`, `apply`, `fieldList`);
* phase 6: engine tests beyond the ones this plan adds;
* F13 dead code other than what this phase's moves delete, and F14 and F15.

Also out of scope: any change to the hand-written SDK files, the goldens, or the exported API; a tag; a push; an agent commit on `main`.

## Grounding

Every fact this plan depends on was measured on `ccc6e1c` in scratch copies of the repository. The tree was not changed. The commands are the ones
described; the results are as printed.

* **G1. The loader drops nine keywords.** A walk of every definition counted these keywords that `load.Definition` does not model, in
  `schema/schema.unstable.json`: `x-deserialize-default-on-error` 385, `additionalProperties` 216, `format` 55, `minimum` 28,
  `x-deserialize-skip-invalid-items` 24, `not` 6, `unevaluatedProperties` 1, `maximum` 1, `minLength` 1. No other keyword is unmodeled.
* **G2. Modeling them changes no output.** A scratch copy added the nine fields to `load.Definition`, walked `additionalProperties`, `not`, and
  `unevaluatedProperties` in `visitDefinition`, and deep-copied them in `deepCopyDefinition`. The merge still produced 280 definitions, 8 stable
  definitions replaced, and 56 `Unstable`-prefixed copies, and the generator's five files were byte-identical to the committed ones (`cmp`).
* **G3. Strict decoding passes, and catches a new keyword.** With `json.Decoder.DisallowUnknownFields` in `Definition.UnmarshalJSON` on top of G2, both
  schema files decoded, the output stayed byte-identical, and a copy of `schema.json` with `"x-new-keyword": true` on `CancelNotification` failed with
  `parse schema.json: json: unknown field "x-new-keyword"`, which does not name the definition. Both meta files have exactly the keys `agentMethods`,
  `clientMethods`, `protocolMethods`, `version`.
* **G4. Every reference resolves.** The merged schema holds 387 `$ref`s: 0 non-local, 0 dangling. A planted dangling `$ref` was detected.
* **G5. Names are consistent today, so one naming pass can reproduce them.** Instrumenting `emit`:
  * 111 union variants, and for none does the name registered in `usedTypeNames` differ from the name emitted after `util.ToExportedField`;
  * 111 results of `generateNestedTypeName`, all unchanged by `util.ToExportedField`;
  * 11 helper variants, each with the same union, type name, and field name as the types emitter computed;
  * `splitCamelCase` and `util.SplitCamel` agree on all 100 distinct inputs `generateNestedTypeName` receives;
  * `toExportedConst` and `util.ToExportedField` agree on every meta method key.
* **G6. One splitter can serve both.** `util.ToExportedField` received 390 distinct inputs; replacing `util.SplitCamel` with the rune-based
  `splitCamelCase` changes none of their outputs. It does change inputs with consecutive capitals: `UX` becomes `UX` instead of `Ux`, `URLValue` becomes
  `URLValue` instead of `Urlvalue`, `mcpServerHTTP` becomes `McpServerHTTP` instead of `McpServerHttp`.
* **G7. The F8 panic is the re-casing.** On a schema with a oneOf `U` whose variants carry `type` consts `x` and `y`, `generateNestedTypeName` returns
  `UX`, which is registered, and `util.ToExportedField` then emits `Ux`, which is not: `panic: BUG: attempting to emit unregistered type: "Ux"`.
* **G8. Method facts.** On the merged schema, `ir.BuildMethodGroups` returns 41 groups; 40 are meta methods and the 41st is `protocol|$/cancel_request`,
  which nothing emits. Every meta method in both meta files has request, response, or notification definitions linked by `x-method` and `x-side`, so the
  meta fallback and `ir.inferTypeBaseFromMethodKey` never run (0% coverage). For every group with a request, `Resp` equals
  `TrimSuffix(Req, "Request") + "Response"`; no response definition is missing or typed `null`, and the generated files contain no
  `SendRequestNoResult` call. On the merged schema the 70 (side, wire, role) slots each hold one definition, and ~~16 wires of the unstable meta are
  not in the stable meta's agent, client, or protocol methods~~ 15 wires of the unstable meta are in none of them; `mcp/message` serves both an agent and a client method, so they carry
  16 methods (corrected 2026-10-10, see the deviation below).
* **G9. The merge's data gives the same bindings as the description text.** `ir.isUnstableMethod` marks 16 of the 40 meta methods unstable; "the wire is
  not in the stable meta's agent, client, or protocol methods" marks the same 16.
* **G10. What the merge changes.** It replaces 8 stable definitions with their unstable versions: `AgentCapabilities`, `ClientCapabilities`,
  `InitializeResponse`, `McpCapabilities`, `McpServer`, `PromptResponse`, `SessionCapabilities`, `SessionUpdate`. Their additions are 10 properties and
  6 union variants, and every one of them already carries `**UNSTABLE**` in its description.
* **G11. What needs marking.** 102 merged definitions are not in the stable schema; 41 carry `**UNSTABLE**` in their description and 61 do not. Of the 16
  methods on `AgentExperimental` and `ClientExperimental`, the printed doc of 6 carries the marker and 10 do not. The schema's own marker, printed 69
  times in `types_gen.go`, is the line `**UNSTABLE**`, a blank line, and `This capability is not part of the spec yet, and may be removed or changed at
  any point.`
* **G12. The hostile schemas of F8.** With a build of `cmd/generate` and `-schema`/`-out` in a scratch directory: enum values `a-b` and `a_b` exit 0 with
  `KindAB redeclared`; properties `foo_bar` and `fooBar` exit 0 with `FooBar redeclared`; a dangling `$ref` exits 0 with `undefined: Missing`; property
  `2fa` exits 1 at `format` after `constants_gen.go` was written; a required variant property `range` exits 1 at `format` of `helpers_gen.go` after the
  other four files were written; the oneOf `U` panics (G7).
* **G13. Where decisions live today.** `emit` imports `internal/load`, `internal/ir`, and `internal/util`. Its uses of them: `util.ToExportedField` 18,
  `ir.PrimaryType` 15, `util.FormatDocComment` 10, `ir.SortedKeys` 6, `ir.IsNullResponse` 4, `ir.DispatchMethodNameForNotification` 3, `ir.Groups` 3,
  the `ir.Bind*` constants 8, `ir.BuildMethodGroups` 2, `ir.MethodInfo` 2, `util.ToEnumConst` 1, `util.TitleWord` 1. The generator has 8 `panic`
  calls; 5 guard naming and move into the model as errors (`generateNestedTypeName`, `emitInlineNestedTypes`, `emitInlineUnionStruct`,
  `registerName`, `emitSchemaDef`), and 3 guard layout and stay until phase 5.
* **G14. Existing tests this plan touches.** `TestGeneratedFilesUpToDate` (`main_test.go`) calls the four `emit.Write*` functions with the loaded schema;
  `TestDefaultedNullablePropertyCompiles` (`emit/types_test.go`) calls `emitObjectSchema` with a hand-built definition; `TestMergeStableAndUnstable`
  (`load/merge_test.go`) covers the merge; `TestRenderLayout`, `TestStructTagQuoting`, and `TestLayoutStaysBelowPrinterSentinel` (`emit/file_test.go`)
  cover layout and are unaffected. Counted across packages, the generator's tests cover 82.5% of its statements.

## Design

The model is plain data. `emit` renders it and makes no decision of its own; after step 6 `emit` imports neither `internal/load` nor `internal/ir`.

```go
// Package model resolves the loaded schema into the facts the emitters render.
package model

func Build(in Input) (*Model, error) // Input: the merged meta and schema, and the merge's provenance

type Model struct {
	Version  int
	Types    []*Type   // top-level definitions, in the order types_gen.go prints them (sorted schema names)
	Methods  []*Method // agent then client, each sorted by meta key
	Protocol []*Method // $/cancel_request; recorded, not emitted in this phase
}

type Type struct {
	SchemaName, GoName string
	Kind               Kind // Enum, StringConstUnion, OpenEnum, Union, Object, Primitive, EmptyObject, Any: emitSchemaDef's switch, in its order
	Doc                []string
	Unstable           bool
	Nested             []*Type   // inline object properties, printed before the type
	Fields             []*Field  // Object
	Union              *Union    // Union
	Enum               []EnumValue
	Validate           ValidateKind // none, required-fields, exactly-one-variant, shared-fields, tool-call-update
	Defaults           []*Field  // fields with schema defaults, for MarshalJSON and UnmarshalJSON
	Fills              []*Field  // required slices and maps filled on encode
}

type Field struct {
	JSONName, GoName string
	Type             TypeRef
	Required, OmitEmpty, Nullable bool
	Default          *Default // canonical JSON, kind, nilable
	Doc              []string
	Facts            Facts    // format, minimum, maximum, minLength, additionalProperties type, x-deserialize-*: recorded, unused until phase 4
}

type TypeRef struct { Kind RefKind; Name string; Elem, Key *TypeRef } // Named, Pointer, Slice, Map, Primitive, Any

type Union struct {
	ExactlyOne       bool
	Discriminator    string
	DiscriminatorSrc DiscSource // Declared or Derived
	Variants         []*Variant // name, field name, disc value, const pairs, required, object/array/null flags, inline struct, doc
	SharedChecks     []SharedCheck
}

type Method struct {
	Side, Wire, Key   string
	Req, Resp, Notif  string // Go type names, from x-method/x-side only
	GoName            string // interface and wrapper method name
	Binding           Binding
	Unstable          bool
	Doc               []string
}
```

Rules the model enforces, each an error of the form `model: <definition path>: <problem>`, returned before any file is written:

* every `$ref` is local and names a definition;
* every Go identifier it produces is valid (`go/token.IsIdentifier`), exported where exported, and not a Go keyword (`go/token.IsKeyword`); a raw
  schema key used as a parameter name is checked the same way;
* names are unique in their scope: package scope (types, constants, generated functions), each struct's fields, each union's variants, each
  interface's methods;
* a method group has at most one request, response, and notification; a meta method has linked types; a request has its response definition.

The model refuses rather than renames: a rename would change the exported API without a decision.

## Implementation Steps

1. Work on `main`. Each step below ends with `make pre-add-check`, `make check-records`, and `make markdownlint`, their output recorded in the execution
   record, and a stop. The owner commits with `git commit --no-edit`. Every new test is first run against a deliberately broken input in a scratch copy
   and its failure recorded.
2. **Loader.** In `internal/load`:
   * add `Format`, `Minimum`, `Maximum`, `MinLength`, `AdditionalProperties`, `Not`, `UnevaluatedProperties`, `XDeserializeDefaultOnError`, and
     `XDeserializeSkipInvalidItems` to `Definition`; walk the three schema-valued ones in `visitDefinition` and copy all of them in
     `deepCopyDefinition` (G1, G2);
   * decode each definition strictly with `DisallowUnknownFields`, decoding `$defs` entry by entry so an error names its definition, and decode the meta
     files strictly (G3);
   * have `MergeStableAndUnstable` also return its provenance: the definitions not in the stable schema and the wires not in the stable meta; correct its
     doc comment, which says stable definitions stay unchanged (G10).
   Tests: a planted unknown keyword fails naming the definition; a reflection test fails when a pointer, slice, or map field of `Definition` is not
   deep-copied (seen to fail by deleting one copy line in a scratch copy); provenance on the committed schema is 102 definitions and ~~16 wires, the 16 methods
   G9 lists~~ 15 wires carrying the 16 methods G9 lists (corrected 2026-10-10), with `$/cancel_request` counted as stable because it is in the stable meta's protocol methods. The five generated files are byte-identical.
3. **Model names.** Create `internal/model` with the reference resolver and the namer. The namer allocates every name in the order the types emitter
   registers them today: sorted definitions; within an object, its inline nested types in sorted property order; within a union, its variants in schema
   order. It computes each name completely, including `util.ToExportedField`, before registering it (G5, G7). Move `generateNestedTypeName`,
   `splitCamelCase`, `singleRefVariant`, and the variant-naming part of `oneUnionVariant` into it; delete `oneHelperVariant`'s naming and read the
   model's names in `emit/helpers.go` (G5). Replace `util.SplitCamel` with the rune-based splitter (G6). Convert the 5 naming panics to errors (G13).
   Tests: table tests for `UX`, `URLValue`, `2fa`, a keyword, and each collision scope, each seen to fail against the current code; a fuzz test that every
   exported name is a valid exported identifier and stable under a second pass. The five generated files are byte-identical.
4. **Model types and fields.** Move `typeExpr`, `optionalTypeExpr`, `primitiveType`, `pointerPrimitive`, `expandAllOf`, `copyMissingAllOfFields`,
   `includesNull`, `defaultKindOf`, the `omitempty` rule of `emitObjectSchema`, the defaults and fills of `emitObjectSchema`, `emitInlineNestedTypes`, and
   `emitInlineUnionStruct`, and the per-property facts of G1 into the model as `Field` and `TypeRef`. `emit` renders a `TypeRef` to `ast.Expr` and renders
   fields from `Field`. `TestDefaultedNullablePropertyCompiles` builds its input through `model.Build` on a one-definition schema. The five generated
   files are byte-identical.
5. **Model classification and unions.** Move the `emitSchemaDef` switch, `isStringConstUnion`, `isOpenStringEnum`, `isRPCTypeName`, `discriminatorKey`
   (recording whether the key was declared or derived), `sharedFieldChecks`, `requiredSet`, the rest of `oneUnionVariant`, and the `Validate`
   eligibility into the model. `emitSchemaDef`, `emitUnion`, `emitValidate`, and `collectHelperVariants` read `Type.Kind`, `Type.Union`, and
   `Type.Validate`. After this step no file in `internal/emit` imports `internal/load` (`go list -f '{{.Imports}}' ./internal/emit`); `ir` still
   does until step 6. The five generated files are byte-identical.
6. **Model methods.** Move `BuildMethodGroups`, `classifyBinding`, `isDocsIgnoredMethod`, `DispatchMethodNameForNotification`, and `methodDescription`
   into the model, with these changes, each proven not to alter the output (G8, G9): roles come from `x-method`/`x-side` only, and the meta fallback and
   `inferTypeBaseFromMethodKey` are deleted; a meta method without linked types, a group with two definitions in one role, and a request without its
   response definition are errors; `writeAgentGen`, `writeClientGen`, `emitOutbound`, and `addRPCMethod` take the response name from `Method.Resp`
   instead of `TrimSuffix`; unstable binding comes from the provenance of step 2, not from description text. `$/cancel_request` is kept in
   `Model.Protocol`. Delete `internal/ir`. The five generated files are byte-identical.
7. **Refuse before writing.** `main.run` takes its arguments (`run(args []string) error` with its own `flag.FlagSet`), loads, builds the model, and only
   then calls the writers, which take the model. A table test in `main_test.go` runs `run` on the six hostile schemas of G12, each in its own directory
   with an output directory holding a sentinel file:
   * enum values `a-b`, `a_b`: error naming `Kind` and `KindAB`;
   * properties `foo_bar`, `fooBar`: error naming `F` and `FooBar`;
   * dangling `$ref`: error naming `R`, `m`, and `Missing`;
   * property `2fa`: error naming `Thing` and `2fa`;
   * required variant property `range`: error naming the union and `range`;
   * for each of these five, the output directory afterwards holds only the sentinel, unchanged;
   * the oneOf `U`: no error, and the five files type-check with `go/types` together with a stub of the hand-written names they use.
   Each case is first run against `ccc6e1c`'s generator and its current failure recorded (G12). The five generated files are byte-identical.
8. **Mark unstable types and methods.** From `Type.Unstable` and `Method.Unstable`, prepend the schema's three-line marker (G11) to the doc of every
   unstable top-level type and every experimental interface method whose doc does not already contain `**UNSTABLE**`. Expected: 61 types and 10 methods.
   A script classifies the regenerated diff and accepts only added comment lines: the marker, and the blank line separating it from an existing doc.
9. **Review and close.** Run the verification below, record each result, and set this plan to `complete`.

## Verification

* After each of steps 2 to 7, `TestGeneratedFilesUpToDate` passes and `git diff --stat` lists no generated file.
* After step 8, the diff of `types_gen.go` contains only added comment lines: 61 type docs and 10 interface-method docs gain the marker, and no other
  line changes. `constants_gen.go`, `agent_gen.go`, `client_gen.go`, and `helpers_gen.go` are unchanged.
* `internal/ir` does not exist. `go list -deps ./internal/emit` lists neither `internal/load` nor `internal/ir`.
* `go test -cover ./internal/model` reports at least 85% of statements, from the model's own tests, without the full-schema test.
* The six hostile schemas behave as step 7 lists, and each new test was seen to fail first, as recorded.
* `go test -race ./...` passes at the root and in `cmd/generate`; `go build ./example/...` passes; `make pre-add-check`, `make check-records`, and
  `make markdownlint` pass.
* The generator contains no `panic` outside `emit/file.go` and `emit/types.go`'s `fieldList`, the three layout guards left for phase 5.

## Rollout and Rollback

Commits land on `main`, made by the owner, one per step. No tag. A push waits for the owner's explicit ask.

Steps 2 to 7 change no generated file, so they carry no release note. Step 8 adds doc comments only; the next release notes mention that unstable types
and methods are now marked.

Rollback is `git revert` of the step commits, newest first. Each step leaves the generator and the generated files in step, so any prefix of the steps is
a consistent tree.

## Deviation 2026-10-10, unstable wire count

Found in step 2: G8 and step 2 said 16 wires of the unstable meta are new. The provenance, and an independent computation in its test, give 15. The
unstable method `mcp/message` is listed under both `agentMethods` and `clientMethods`, so 16 unstable methods use 15 wires. The earlier scratch measurement
counted meta entries rather than distinct wires. G9's 16 of 40 methods is right.

Decision: the owner chose to correct the plan now. G8 and step 2 are struck through and corrected above. No code, test, or scope changes.

## Execution record

### Step 2, loader, 2026-10-10

The owner accepted [0011-MADR-harden-code-generator-engine.md](0011-MADR-harden-code-generator-engine.md) and approved this plan on 2026-10-10; the
plan's status is `in-progress`. Step 2 ran on `main` from a clean tree at `e109c7b`. The agent did not commit.

Changes in `cmd/generate/internal/load`:

* `Definition` gained `Format`, `Minimum`, `Maximum`, `MinLength`, `AdditionalProperties`, `Not`, `UnevaluatedProperties`,
  `XDeserializeDefaultOnError`, and `XDeserializeSkipInvalidItems`. `visitDefinition` walks the three schema-valued ones; `deepCopyDefinition` copies all
  of them, through a new `clonePtr` for the pointers.
* `Definition.UnmarshalJSON` decodes through `decodeStrict`, which sets `DisallowUnknownFields`. `parseSchema` decodes `$defs` entry by entry, in sorted
  order, wrapping an error as `definition "<name>": ...`, and keeps a `null` definition as nil. `parseMeta` decodes the meta files strictly. The four
  `Read*` functions use them.
* `MergeStableAndUnstable` returns a fourth result, `*Provenance`: `UnstableDefs`, the merged definitions the stable schema lacks, and `UnstableWires`, the
  unstable-only wires it already computed. Its doc comment now lists what the merge adds, including the stable definitions it replaces.

Callers: `merge_test.go`'s `mustMerge` takes the extra result. `main.loadSchema` keeps its signature and drops the provenance: a first version that
returned it failed `golangci-lint` for host and Windows with `loadSchema - result 2 (*...load.Provenance) is never used (unparam)`, because nothing reads
it before step 6. Step 6 threads it through when the model reads it. `main_test.go` is unchanged.

New tests in `cmd/generate/internal/load/load_test.go`:

* `TestParseSchemaRejectsUnknownKeywords`: an unknown keyword at the top of a definition, and one nested in a property, fail naming the definition and the
  keyword.
* `TestParseSchemaKeepsModeledKeywords`: the nine keywords decode into their fields, and a `null` definition stays nil.
* `TestParseMetaRejectsUnknownKeys`.
* `TestDeepCopyDefinitionCoversEveryField`: fails when the fixture leaves a field of `Definition` zero, or when the copy shares a pointer, map, or slice
  field with the original.
* `TestMergeProvenanceMatchesInputs`: on the committed schema, the provenance equals an independent computation from the inputs, and `$/cancel_request` is
  not unstable. It asserts the sets rather than fixed counts, so a schema update does not break it; it logs `unstable definitions: 102; unstable wires: 15`.

Each was seen to fail, with one planted breakage per run in a fresh scratch copy:

```text
lenient decoding: exit=1 failing=['TestParseMetaRejectsUnknownKeys', 'TestParseSchemaRejectsUnknownKeywords']
    load_test.go:73: got <nil>, want an unknown field error naming extra
deep copy shares Not: exit=1 failing=['TestDeepCopyDefinitionCoversEveryField']
    load_test.go:128: Not is shared between the original and the copy
new field missing from the fixture: exit=1 failing=['TestDeepCopyDefinitionCoversEveryField']
    load_test.go:116: the fixture leaves Future zero; set it so its copy is checked
provenance skips prefixed copies: exit=1 failing=['TestMergeProvenanceMatchesInputs']
    load_test.go:179: UnstableDefs: got 46, want 102; extra []
format keyword renamed: exit=1 failing=['TestMergeProvenanceMatchesInputs', 'TestParseSchemaKeepsModeledKeywords']
    load_test.go:51: definition "N": json: unknown field "format"
```

The last plant also failed the provenance test, because strict decoding rejected `format` in the committed schema.

Output: `cd cmd/generate && go run .` regenerated the five files and none changed; `TestGeneratedFilesUpToDate` passes.

Gates: `make pre-add-check` with the five changed Go files named: `go-precheck: 5 file(s) clean (gofmt, golangci-lint, go vet, go test, govulncheck).`
Without names: `go-precheck: 50 file(s) clean (...)`. `make check-records` and `make markdownlint` exit 0.

Commit point: the agent stopped here. The owner commits step 2 on `main`.
