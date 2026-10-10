---
status: accepted
date: 2026-10-09
decision-makers: Repository owner
---

# Keep this SDK self-contained on the current ACP schema

## Context and Problem Statement

The owner wants a clean separation from the previous project. This tree must not depend on `github.com/coder/acp-go-sdk` or on Jennifer. It is to be its own self-contained, Go-native, idiomatic SDK, written around the current ACP and schema standards.

On 2026-10-09 the accepted pins are already in the tree. Both `go` lines are `1.27.2`. `schema/version` is `1.25.0`, the latest stable schema from [0004-MADR-pin-acp-schema-1-25-0.md](0004-MADR-pin-acp-schema-1-25-0.md). The root `go.mod` has no `require` block. What still presents this tree as the previous project is the module path `github.com/coder/acp-go-sdk`, the imports and README links that use it, and `github.com/dave/jennifer` v1.7.1 in `cmd/generate`.

[0005-MADR-separate-module-from-coder.md](0005-MADR-separate-module-from-coder.md) accepts the module rename. [0006-MADR-emit-go-with-go-ast.md](0006-MADR-emit-go-with-go-ast.md) accepts removing Jennifer by emitting with `go/ast`. Neither plan has been executed.

[0001-REPORT-go-127-acp-parity.md](../reports/0001-REPORT-go-127-acp-parity.md) separates two further changes from a floor bump. Moving call sites to `encoding/json/v2` changes which payloads the codec accepts. Replacing the generator's `os.ReadFile` calls with `os.OpenRoot` is a filesystem change, not a dependency removal. The wire codec today is `encoding/json`. The v1 API on Go 1.27.2 already runs on the v2 implementation.

## Decision Drivers

* The owner wants this repository independent of the previous project.
* The owner wants no dependency on `github.com/coder/acp-go-sdk` and no dependency on Jennifer.
* The current stable ACP schema is already pinned. A prerelease v2 schema is not that standard.
* A self-contained SDK uses the standard library for code generation and does not take a third-party generator.

## Considered Options

* Carry out the accepted module rename and the accepted Jennifer removal, and leave the JSON codec and the schema stamp as they are.
* Also move JSON call sites to `encoding/json/v2` and the generator reads to `os.OpenRoot`.
* Treat the current module path as already separate because `origin` is a different remote.

## Decision Outcome

Chosen option: "Carry out the accepted module rename and the accepted Jennifer removal, and leave the JSON codec and the schema stamp as they are", because that is the separation the owner described, and the schema and Go floor are already the accepted current pins.

The module path becomes `github.com/maccavelli/acp-go-sdk2`, as 0005 decided. Jennifer is removed completely, as 0006 decided, by emitting Go with `go/ast` and `go/format`. The root module stays free of a `require` block. `cmd/generate` has no third-party requirement after 0006. Protocol v2 stays out. `encoding/json` stays the call-site package. The G304 `os.ReadFile` directives stay until their own record.

### Consequences

* Good, because importers of this tree no longer use the previous module path, and neither module requires Jennifer.
* Good, because the schema stamp and the Go floor do not move again in this decision.
* Neutral, because generated JSON methods keep calling `encoding/json`. On Go 1.27.2 that package is implemented with `encoding/json/v2`.
* Bad, because the public files still say `github.com/coder/acp-go-sdk`, and `cmd/generate` still requires Jennifer, until the plans below are executed.

### Confirmation

After both plans have been executed: the module paths are the 0005 paths, no `.go` file imports `github.com/coder/acp-go-sdk` or `github.com/dave/jennifer/jen`, neither `go.mod` requires either module, and `go test ./...` passes at the root and in `cmd/generate`. `schema/version` still reads `1.25.0`. Both `go` lines still read `1.27.2`.

## Pros and Cons of the Options

### Carry out the accepted module rename and the accepted Jennifer removal, and leave the JSON codec and the schema stamp as they are

* Good, because 0005 and 0006 already record the two removals, and 0004 already records the current schema.
* Good, because the generated SDK behavior stays on the codec the golden tests cover.
* Neutral, because `os.ReadFile` in the generator keeps its `//nolint:gosec` directives.

### Also move JSON call sites to `encoding/json/v2` and the generator reads to `os.OpenRoot`

* Good, because those are newer standard-library APIs.
* Bad, because v2 defaults reject invalid UTF-8 and duplicate object names unless the caller sets options. That is a wire-behavior change, and this decision does not ask for one.
* Bad, because `os.OpenRoot` does not remove a dependency. It belongs in a later record.

### Treat the current module path as already separate because `origin` is a different remote

* Good, because git `origin` is already this repository and there is no `upstream` remote.
* Bad, because Go imports the module path, which is still `github.com/coder/acp-go-sdk`. Jennifer is still required by `cmd/generate`.

## More Information

Implementation is [0007-PLAN-self-contained-acp-sdk.md](0007-PLAN-self-contained-acp-sdk.md). It runs [0005-PLAN-separate-module-from-coder.md](0005-PLAN-separate-module-from-coder.md) and then [0006-PLAN-emit-go-with-go-ast.md](0006-PLAN-emit-go-with-go-ast.md). Those two plans remain the step lists.

## Amendment 2026-10-09, the SDK stays on encoding/json v1

[0008-REPORT-self-contained-sdk-review.md](../reports/0008-REPORT-self-contained-sdk-review.md) measured `encoding/json` against `encoding/json/v2` on Go 1.27.2 for this SDK.
After reading it, the owner decided on 2026-10-09 that the SDK stays on `encoding/json` v1. That decision is not limited to this separation.
The option "Also move JSON call sites to `encoding/json/v2`" is rejected, not deferred. No later record is planned to adopt v2.

What this means:

* Neither module imports `encoding/json/v2` or `encoding/json/jsontext`. Hand-written code calls `encoding/json`.
* The generator emits `MarshalJSON` and `UnmarshalJSON` that call `encoding/json`. It does not emit `MarshalJSONTo` or `UnmarshalJSONFrom`.
* `json.RawMessage` stays in the exported API. On Go 1.27.2 it is an alias of `jsontext.Value`, so the type is the same either way.
* The wire behavior is the v1 behavior, implemented by Go 1.27 on top of v2 internals. Go's compatibility promise covers the v1 API. Error strings may change between toolchains.

The measurements behind the decision, all on Go 1.27.2:

| Behavior | v1 | v2 default |
| :--- | :--- | :--- |
| nil slice or map | `null` | `[]` / `{}` |
| `omitempty` on `bool` | omits `false` | keeps `false` |
| `omitempty` on a struct | never omits | omits when it encodes as `{}` |
| `<`, `>`, `&` in strings | escaped | written as is |
| `map` key order | sorted | random unless `Deterministic(true)` |
| Object names | case-insensitive | exact |
| Duplicate names | last wins | error |
| Invalid UTF-8 | replaced with U+FFFD | error |
| Type mismatch error | `*json.UnmarshalTypeError` | `*json.SemanticError` |

* Five of the nine change what the SDK sends. The 2026-10-09 text of this decision named only invalid UTF-8 and duplicate names.
* The generated union decoders depend on `*json.UnmarshalTypeError`. Under v2 they would reject valid string and number variants such as `RequestId` and `ErrorCode`.
* With v2 defaults, 33 of the 322 generated struct types encode their zero value differently. With `json.DefaultOptionsV1()`, none do. A swap that keeps v1 behavior gains nothing.
* The golden suite passes under v2 defaults, so it could not have guarded a migration.
* Importing the v2 packages stops the module from building with `GOEXPERIMENT=nojsonv2`.

Two wire defects the measurement surfaced exist under v1 as well. A union with no variant set encodes to zero bytes, and a required array left nil is sent as `null`.
They are not caused by the codec choice. [0009-MADR-fix-generated-output-on-json-v1.md](0009-MADR-fix-generated-output-on-json-v1.md) proposes fixing them, and the generated-output quirks [0006-MADR-emit-go-with-go-ast.md](0006-MADR-emit-go-with-go-ast.md) lists, within `encoding/json` v1.

## Amendment 2026-10-09, review findings

The same report checked this decision against the tree. Its facts hold: both `go` lines are `1.27.2`, `schema/version` is `1.25.0`, the root module has no `require`, and neither plan had been executed.

Two remnants of the previous project were missing from the Context. Both now belong to 0005:

* The `mdsh` directive on `README.md` line 14 rebuilds the install line with the old path and the schema stamp. [0005-MADR-separate-module-from-coder.md](0005-MADR-separate-module-from-coder.md) adds it to the rename.
* `LICENSE` keeps "Copyright 2025 Coder Technologies, Inc.". Apache-2.0 requires a derivative work to retain that notice. A clean separation keeps it.

"Self-contained, Go-native, idiomatic" holds for the generator once 0006 is executed: standard library only, no third-party requirement.
It does not yet hold for the generated output, because 0006 keeps that output byte-identical. 0009 proposes the output changes.

[0007-PLAN-self-contained-acp-sdk.md](0007-PLAN-self-contained-acp-sdk.md) carries the matching amendment, including a branch and commits in place of "no branch, no commit".
