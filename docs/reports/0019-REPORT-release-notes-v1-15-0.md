# Release notes for v1.15.0, against v1.14.0

Observed 2026-10-15 on `3704df9`, for [0019-PLAN-generator-hardening.md](../decisions/0019-PLAN-generator-hardening.md), step 7. This
report drafts the text of the GitHub release for `v1.15.0`, as `RELEASING.md` describes it. It decides nothing. Each statement names
its source: a record, a commit, or the exported API listing of the 0013 report's listing program, run on `git archive v1.14.0` (2426
lines) and on the tree at `3704df9` (2428 lines). Bring it up to date if more changes land before the tag. Phase 3 of
[0019-MADR-remediate-debugging-pass-findings.md](../decisions/0019-MADR-remediate-debugging-pass-findings.md) rides the release after
it lands (decision 12); it adds exported API, so the release is a minor version, which the owner confirms at the tag.

The section "Release text" is what goes into the GitHub release.

## Release text

### v1.15.0

The ACP schema is still `1.25.0` (`git diff v1.14.0 -- schema/` is empty). This release hardens the code generator: shapes of a
schema that it typed wrongly or without a word are now refused with an error naming the definition, or typed correctly. For the SDK,
one type changes: the unstable `UnstableMcpError` keeps the keys it does not declare, and an explicit `null` in its `data`, as its
schema says. The exported API grows by two entries and loses none. (0019-MADR, phase 3.)

#### Behaviour change

- **`UnstableMcpError.Data` keeps an explicit `null`.** The schema says "Optional error data; explicit null is retained." A
  received `"data": null` now decodes to the bytes `null`, where it decoded empty, and is sent on as `null`; an absent `data` is
  still `nil`, and a value is kept byte for byte. Code that took `len(e.Data) == 0` to mean "no data" should test `e.Data == nil`
  for absent and `string(e.Data) == "null"` for an explicit null. Every other optional raw field, `rawInput` and `rawOutput` among
  them, still reads `null` as absent, as ACP v1 defines. (`131fa07`; 0019-MADR, decision 5;
  [0012-MADR-schema-fidelity-framework.md](../decisions/0012-MADR-schema-fidelity-framework.md), amendment of 2026-10-14.)

#### Additions

- `UnstableMcpError.Extra map[string]json.RawMessage` and `(UnstableMcpError) MarshalJSON`: the keys of a received error that the
  type does not declare are kept in `Extra`, as received, and sent back; a declared field wins over an `Extra` key of the same name.
  The schema gives `McpError` `"additionalProperties": true`. (`131fa07`; 0019-MADR, decision 5.)
- The README documents both, in "Wire encoding". (`131fa07`.)

#### For the generator

These change nothing in the generated SDK beyond what is listed above. A schema that used to generate may now be refused, with an
error that names the definition and the property or variant:

- Refused, where the generated type would have lost or not checked what the schema says: an `enum` with a value that is not a
  string; a type list of two types other than `null`; a named array definition; a union variant with no title, `$ref`, properties or
  null type, which was dropped; a `const` on a union variant that is not an object, which nothing checked; a discriminator const that
  is not a string; a property key a json struct tag cannot carry; a field, wrapper field or member named `MarshalJSON`,
  `UnmarshalJSON` or `Validate`; a `required` key no property declares; a definition linked by `x-method` and `x-side` to a method no
  meta file lists; a schema keyword written in another letter case; a `default` its property's schema does not admit; an integer
  bound its Go type cannot reach; and an `additionalProperties` a struct would not keep. (`1ee1eb7`, `8d2286a`, `852054f`, `131fa07`,
  `3704df9`.)
- Handled: a definition with properties and no `"type": "object"` is an object, with its decoder, `Validate` and gates; an integer
  `minimum` or `maximum` with a fraction is rounded inward; a `uint64` bound above the `int64` range is written as one; an object
  whose `additionalProperties` is `true` keeps its other keys in `Extra`; and the protocol table's `NullRetained` names a raw property
  that keeps `null`. (`1ee1eb7`, `8d2286a`, `131fa07`.)
- The output check type-checks the package as tested too, with the hand-written test files, so a name a test file and
  `schema_gen_test.go` both declare is refused, as `go vet` refuses it; and a missing output directory is refused, where it gave
  hundreds of type errors. (`852054f`.)
- Every error site of the generator is run by a test or named with its reason, as
  [0011-MADR-harden-code-generator-engine.md](../decisions/0011-MADR-harden-code-generator-engine.md) requires. (`3704df9`.)

## Sources not in the release text

- The commits since `v1.14.0` that change the SDK's Go code: `131fa07` (`git log v1.14.0..HEAD -- '*.go' ':!cmd/generate'`), whose
  only change to a generated file is `UnstableMcpError` in `types_gen.go` (35 lines added, 2 removed); and the generator's:
  `131fa07`, `1ee1eb7`, `8d2286a`, `852054f`, `3704df9` (`git log v1.14.0..HEAD -- cmd/generate`). The others change records and
  documentation.
- The API listing difference at `3704df9`: 0 lines removed, 2 added, `field UnstableMcpError.Extra map[string]json.RawMessage` and
  `method (UnstableMcpError) MarshalJSON () ([]byte, error)`.
- Each change, its tests, the planted breaks each test was seen to fail on, and a run on Windows for each step are in the execution
  record of [0019-PLAN-generator-hardening.md](../decisions/0019-PLAN-generator-hardening.md), with every deviation from it.
- Not in this release: phases 4 and 5 of 0019 (tooling, examples and documentation; idioms and performance), which ride the releases
  after each lands (0019-MADR, decision 12).
