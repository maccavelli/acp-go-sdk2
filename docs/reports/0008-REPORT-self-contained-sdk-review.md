# Review of the self-contained SDK records, a go/ast generator, and encoding/json/v2

Observed 2026-10-09. This report checks [0005-MADR-separate-module-from-coder.md](../decisions/0005-MADR-separate-module-from-coder.md),
[0006-MADR-emit-go-with-go-ast.md](../decisions/0006-MADR-emit-go-with-go-ast.md),
[0007-MADR-self-contained-acp-sdk.md](../decisions/0007-MADR-self-contained-acp-sdk.md), and their three PLANs against the tree as it stands.
It carries a complete go/ast rewrite of `cmd/generate`, built and verified outside the tree, and an assessment of moving from `encoding/json` to `encoding/json/v2`.
It decides nothing. Where it recommends a change to a record, the owner decides.

## Answer

| Record | Accurate | Comprehensive | Feasible |
| :--- | :--- | :--- | :--- |
| 0005 MADR | Yes. Every fact checked holds. | Misses the README `mdsh` directive and does not say the Apache-2.0 notice stays. | Yes. Executed in a scratch copy: root tests and examples pass. |
| 0005 PLAN | Yes. | Step 3 is undone by the next `make version`. Rollout leaves work that cannot be committed under the branch rule. | Yes, with the amendments below. |
| 0006 MADR | Yes, except the emit file list omits `dispatch_helpers.go` (92 lines). | Does not name the main technical risk: go/printer places comments and line breaks by token position, and a built tree has none. | Yes. A prototype reproduces all five generated files byte for byte. |
| 0006 PLAN | Yes. `go.sum` handling is right. | Port order fights shared code. The empty-diff gate runs once and is then gone. | Yes, with a reordered port and a permanent test. |
| 0007 MADR | Yes. Pins, requires, and "neither plan executed" all hold. | Lists two of the nine JSON v2 behavior differences measured here. | Yes. It adds no work beyond 0005 and 0006. |
| 0007 PLAN | Yes. | Inherits the 0005 and 0006 gaps. Rollback is not defined for a dirty tree. | Yes, after the amendments. |

The go/ast rewrite in [Appendix A](#appendix-a-the-goast-generator) emits the same bytes as the Jennifer generator for the current schema and for twelve earlier schema stamps.
It has no third-party requirement. It passes `go vet`, `gofmt`, `golangci-lint` with `.golangci.yml` for the host and for `GOOS=windows`, and `go test -race`.

On JSON: keep `encoding/json` for this separation, as 0007 decides. A move to `encoding/json/v2` is worth its own record, after the go/ast port and after two existing wire defects are fixed.
The golden tests cannot guard that move today. They pass under v2 defaults while 33 generated types already serialize differently.

## How this was checked

Toolchain: `go1.27.2 darwin/arm64`. Linter: `golangci-lint` 2.14.0 built with go1.27.2.

Read in the tree: the six records, [0001-REPORT-go-127-acp-parity.md](0001-REPORT-go-127-acp-parity.md), the 0002 and 0004 MADRs, `AGENTS.md`, `README.md`, `LICENSE`,
`Makefile`, `.github/workflows/ci.yml`, both `go.mod` files, `cmd/generate/go.sum`, every file under `cmd/generate`, `connection.go`, `errors.go`, `extensions.go`,
`json_parity_test.go`, `types_gen.go`, `agent_gen.go`, `client_gen.go`, the schema files, and the Jennifer v1.7.1 and Go 1.27.2 `go/printer` sources.

Every experiment ran in a scratch copy outside the repository. The tree was not built into, regenerated, or formatted. This report and its rows in `docs/README.md` are the only files written.

Experiments, each repeatable from the description in [Appendix B](#appendix-b-reproducing-the-checks):

- Ran the current generator into a scratch directory and compared its output with the committed `*_gen.go` files.
- Applied the 0005 rename to a scratch copy and ran `go test ./...` and `go build ./example/...`.
- Wrote the go/ast generator, then compared its output with the Jennifer generator's on schema files taken from 13 commits.
- Broke five things on purpose in throwaway copies and confirmed that the new tests fail.
- Compared `encoding/json` with `encoding/json/v2` on a probe program, on the golden suite, and on the zero value of all 322 generated struct types.

## State of the tree on 2026-10-09

- Both `go` lines are `1.27.2`. `schema/version` and `version` are `1.25.0`. The root `go.mod` has no `require` and no `replace`.
- The only git remote is `origin`, and it is this repository. There is no `upstream`.
- `cmd/generate/go.mod` requires `github.com/dave/jennifer v1.7.1`. `go.sum` has its two lines. `jenwrap.go` is the only Jennifer import.
- The old module path appears in both `go.mod` files, seven Go files in `cmd/generate`, four example programs, `AGENTS.md`, `README.md` (six places), and `docs/architecture.md`.
- The current generator reproduces all five committed generated files byte for byte. The generated files are in sync with `schema/`.
- `cmd/generate/internal/emit` has no tests. The only generator tests are in `internal/load`.
- `golangci-lint` with `.golangci.yml` reports 0 issues on `cmd/generate` today, on the host.
- `HEAD` is `main`. The working tree holds uncommitted edits to `docs/README.md` and the 0005 and 0006 MADRs, and the untracked 0007 records.

## Findings on 0005

The MADR's facts hold: the remote, the missing `upstream`, the module lines, the import sites, and the Jennifer requirement.
A scratch run of the rename edited `go.mod`, the four examples, `README.md`, `AGENTS.md`, and `docs/architecture.md`. The generator module in that copy was the rewrite, which already uses the new path.
`go test ./...` and `go build ./example/...` passed on the renamed copy.

### The README install line comes back on the next `make version`

`README.md` line 14 is an `mdsh` directive:

```text
<!-- `$ printf 'go get github.com/coder/acp-go-sdk@v%s\n' "$(cat schema/version)"` as bash -->
```

The `README.md` target in `Makefile` depends on `schema/version`, and `make version` depends on `README.md`. PLAN step 3 drops `@v0.13.5` and says not to write `@v1.25.0`.
If only the path in the directive changes, the next `make version` rewrites line 17 as `go get github.com/maccavelli/acp-go-sdk2@v1.25.0`, which is exactly what step 3 forbids.
The step has to change the directive itself, for example to `printf 'go get github.com/maccavelli/acp-go-sdk2\n'`, or remove the `mdsh` block.

### The Apache-2.0 notice stays

`LICENSE` line 189 reads "Copyright 2025 Coder Technologies, Inc.". Section 4(c) of the Apache License 2.0 requires a derivative work to keep the copyright notices of the work it came from.
Neither plan touches `LICENSE`, which is right. The MADR says the tree must not keep the previous project "as its identity".
One sentence saying the license notice is retained would stop a later reader from applying that to `LICENSE`.

### Records keep the old path

`docs/README.md` names the old path in three record titles. The PLAN's verification search correctly leaves `docs/README.md` and the records out.
Step 2 does not say so, and a global replace over `*.md` rewrites those titles. The scratch run's plain replace did exactly that.

### Rollout cannot end in a commit

All three PLANs say: stay on the current branch, no branch, no commit. `HEAD` is `main`, and the repository's git rules allow no commit on `main`.
Executed as written, the change can only stay uncommitted. Committing it later requires a branch the PLAN forbids.

Rollback is also undefined. It says "restoring those files", but `docs/README.md` already carries uncommitted record edits, and nothing in the plan separates them from the plan's own edits.

A rollout that works: commit the current records on a branch first, then run each phase on that branch and commit it after `make pre-add-check`, `make check-records`, and `make markdownlint`.

## Findings on 0006

The MADR's sizes are right: `types.go` 1,505, `dispatch.go` 277, `helpers.go` 192, `constants.go` 63, `jenwrap.go` 88.
The list omits `dispatch_helpers.go` (92 lines), which also builds Jennifer code through the aliases in `jenwrap.go`.
The claim that `go/types` does not write source is right. So is the PLAN's handling of `go.sum`.
`go mod tidy` leaves a 0-byte `go.sum` when the last requirement goes, and `go mod tidy -diff` is clean once it is deleted.

### The risk the MADR does not name

The MADR lists "comment placement and blank lines can differ" as a consequence. That understates it.
`go/printer` derives comment placement and most line breaks from token positions. A tree built in memory has none, and the printer then does the following:

| What the emitters need | What go/printer does with no positions |
| :--- | :--- |
| Doc comments above declarations and struct fields | It interleaves `*ast.File` comments by position. A position-free doc comment on a function printed as `package// F does things.` followed by `p`. |
| Multi-line function bodies | Once the `func` keyword has a position, which a doc comment needs, it collapses any body that fits in 100 columns onto the signature line, as `func F() { return x }`, unless the closing brace sits on a later line. |
| `const ( X T = "x" )` with one spec | It drops the parentheses unless `Lparen` is a valid position. |
| `type X = json.RawMessage` | It prints a defined type unless `TypeSpec.Assign` is a valid position. |
| `struct{}` and `interface{}` | It prints `struct {` and `}` on two lines unless both braces share a line. |
| One element per line in the `New…` helpers | It puts every composite literal on one line. |

Positions solve all six, but they bring a constraint the prototype hit. The printer treats any source offset at or above `1<<30` as "no more comments" and dereferences a nil comment.
A layout that gives each line a fixed large stride crashes on `types_gen.go`. The prototype keeps lines short and reserves the printed size of each function body on its lines instead.

The empty-diff gate is achievable. Without a layout technique it fails on every file, so the MADR should name one.

### Port order fights shared code

PLAN step 4 ports helpers, then dispatch, then types. `helpers.go` calls `jenTypeFor`, `expandAllOf`, `generateNestedTypeName`, `singleRefVariant`, and `isStringConstUnion`, all in `types.go`.
Porting helpers first needs go/ast copies of those functions next to the Jennifer ones until types is done. An order with no duplication: constants, then dispatch, then types and helpers together.

### The gate runs once

The PLAN compares regenerated files with the old ones during the port. Nothing keeps that comparison afterwards, and `emit` has no tests.
The prototype adds `TestGeneratedFilesUpToDate` to `cmd/generate`. It regenerates into a temporary directory and requires the committed files to match byte for byte.
CI already runs `go test` in `cmd/generate`, so that one test turns the PLAN's gate into a standing check that also catches a schema bump without a regeneration.

### `main.go` scope

The PLAN limits `main.go` to the call renames. The prototype also returns errors instead of panicking, and it only searches for the repository root when a flag is missing.
The current `main.go` calls `findRepoRoot()` even when `-schema` and `-out` are both set, so it panics when run outside the repository. That was observed in the scratch runs.
Those two changes are outside the PLAN as written. Adopting them needs one line in its scope.

## Findings on 0007

The pins it records are in the tree: both `go` lines at `1.27.2`, `schema/version` at `1.25.0`, and no `require` in the root module. Neither 0005 nor 0006 has been executed.
Its statement that the v1 API already runs on the v2 implementation holds. On 1.27.2 `json.RawMessage` is an alias of `jsontext.Value`, and `GOEXPERIMENT=nojsonv2` removes the v2 packages.

Its JSON reasoning names invalid UTF-8 and duplicate names. Those hold, but they are two of nine measured differences, listed below.
Five of the others change what this SDK sends, not only what it accepts.

The PLAN only sequences 0005 then 0006, so it carries both sets of gaps. Its rollback has the same dirty-tree problem as 0005.

"Self-contained, Go-native, idiomatic" holds for the generator after the port: standard library only, `slices` and `maps` iteration, errors instead of panics.
It does not yet hold for the generated output, because the empty-diff rule preserves the generator's existing output, including these quirks:

- In 18 stable unions the schema description lands on the first variant type, not the union. `type ContentBlock struct` has no doc comment, and `ContentBlockText` carries both.
- `agent_gen.go` and `client_gen.go` have no blank line between functions.
- Union decoding writes `var match bool = true` and ignores the error from decoding the discriminator.
- A defaulted property whose type allows `null` would generate an unused `_rm` and fail to compile. No property in the current schema hits that branch.

Fixing those changes the generated files, so each fix is a later, separate step with its own reviewed diff.

## The go/ast generator

[Appendix A](#appendix-a-the-goast-generator) is the complete rewrite. It replaces `internal/emit` and `main.go`.
`internal/load`, `internal/ir`, and `internal/util` stay as they are, apart from the module path in one import of `ir.go`. The module path is the 0005 path, because 0007 runs 0005 first.

### Design

- `ast.go` holds small constructors that return `go/ast` nodes. They are local helpers, as the 0006 MADR allows, and they build a fresh node on every call so no node is shared.
- `file.go` holds a `goFile`: an ordered list of comment lines, blank lines, and declarations, plus the imports seen through `qual`. Emitters add items in the order the Jennifer emitters wrote them.
- Rendering lays the items out on the lines they would occupy in written source. It sets the positions go/printer needs, prints the `*ast.File` with `go/format.Node`, and then runs `go/format.Source`.
  The second pass parses real positions back, so doc-comment reformatting applies exactly as gofmt applies it to written source. Jennifer also finishes with `format.Source`.
- `constants.go`, `helpers.go`, `dispatch.go`, `dispatch_helpers.go`, and `types.go` are the Jennifer emitters ported one function at a time. The public writers lose the `Jen` suffix.
- Discriminator and const-property order is now sorted instead of following map iteration, which Go randomizes.
  No schema variant has more than one const property, so the output does not change.

### Verification

| Check | Result |
| :--- | :--- |
| Output against the committed `*_gen.go` files | All five byte-identical. |
| Output against the Jennifer generator, schema files from 13 commits (stamps 0.4.5 to 1.25.0, with and without unstable files) | 13 of 13 identical. |
| Six runs on the same input | One hash. |
| `go vet`, `gofmt -l` | Clean. |
| `golangci-lint` with `.golangci.yml`, host and `GOOS=windows` | 0 issues after five fixes (two unchecked type assertions, one repeated string, one De Morgan, one constant parameter). |
| `go test -race ./...` in `cmd/generate` | Pass. |
| `go test ./...` and `go build ./example/...` at the root, on the renamed copy | Pass. |
| `go mod tidy -diff` in both modules; `go list -m all` in `cmd/generate` | Clean; the module itself and nothing else. |

Each new test was seen to fail on a deliberately broken copy:

| Breakage | Test | Failure |
| :--- | :--- | :--- |
| One line added to `types_gen.go` | `TestGeneratedFilesUpToDate` | `types_gen.go is out of date with schema/; run make version (first difference at line 2)` |
| One emitted error string changed | `TestGeneratedFilesUpToDate` | `types_gen.go is out of date … (first difference at line 1656)` |
| Empty-struct braces split onto two lines | `TestRenderLayout` | `render mismatch` in the empty-struct case |
| Function braces put on one line | `TestRenderLayout` | `render mismatch` in the short-body case |
| Line length raised to `1<<20` | `TestLayoutStaysBelowPrinterSentinel` | `layout spans 12588178880 bytes` |

### Size

The Jennifer `emit` package and `main.go` are 2,338 lines. The rewrite is 2,641 lines of non-test code plus 245 lines of tests.
583 of those lines are the replacement for Jennifer itself (`ast.go` 229, `file.go` 354).
The emitters shrink: `types.go` 1,505 to 1,389, `dispatch.go` and `dispatch_helpers.go` 369 to 334, `helpers.go` 192 to 154.

## encoding/json to encoding/json/v2

### What the SDK uses

| Where | Use |
| :--- | :--- |
| `connection.go` | 10 calls: `Marshal`, `Unmarshal`, and a `Decoder` with `UseNumber` to canonicalize JSON-RPC ids. `anyMessage` holds `json.RawMessage` fields. |
| `errors.go` | One `Marshal` in `RequestError.Error`. |
| `extensions.go` | `json.RawMessage` in the exported `ExtensionMethodHandler` and `CallExtension` signatures. |
| `types_gen.go` | 28 union types and 9 defaulted structs with `MarshalJSON` and `UnmarshalJSON`, all calling `encoding/json`. 28 assertions on `*json.UnmarshalTypeError`. |
| `agent_gen.go`, `client_gen.go` | `json.Unmarshal` of params in every dispatch case. |

### Measured differences

Probe program, Go 1.27.2. "v2" means `encoding/json/v2` with default options.

| Behavior | v1 (today) | v2 default | Direction |
| :--- | :--- | :--- | :--- |
| nil slice or map | `null` | `[]` / `{}` | Outbound |
| `omitempty` on `bool` | omits `false` | keeps `false` | Outbound |
| `omitempty` on a struct | never omits; `{"in":{}}` | omits when it encodes as `{}`; `{}` | Outbound |
| `<`, `>`, `&` in strings | escaped as `\u003c` | written as is | Outbound |
| `map` key order | sorted | random unless `Deterministic(true)` | Outbound |
| Object names | case-insensitive match | exact match | Inbound |
| Duplicate names | last wins | error | Inbound |
| Invalid UTF-8 | replaced with U+FFFD | error | Inbound |
| Type mismatch error | `*json.UnmarshalTypeError` | `*json.SemanticError` | Generated code |

The last row matters most. Every generated union's `UnmarshalJSON` tells "not an object" apart from "broken JSON" with `err.(*json.UnmarshalTypeError)`.
If those methods called v2, that assertion would always fail, so a union whose variant is a string or a number (`RequestId`, `ErrorCode`) would return an error instead of trying its other variants.

On the zero value of all 322 generated struct types, v2 defaults change the bytes of 33. Every one of the 33 is a nil slice becoming `[]`.
`encoding/json/v2` with `json.DefaultOptionsV1()` changes 0 of 322. So a mechanical call-site swap that passes the v1 options is safe and gains nothing.
The 28 unions and 9 defaulted structs keep v1 behavior under a v2 call site, because their own `MarshalJSON` and `UnmarshalJSON` call `encoding/json` inside.

The golden suite in `json_parity_test.go` passes unchanged when its marshal and unmarshal calls are switched to v2 defaults. It covers populated values only, so it cannot see any of the differences above.

### Two existing defects the comparison surfaced

- A union with no variant set marshals to zero bytes, because `MarshalJSON` ends with `return []byte{}, nil`. Encoding then fails with "unexpected end of JSON input".
  57 of the 322 zero values fail this way, including `RequestPermissionResponse`, `SessionNotification`, and `ContentBlock`. This happens under v1 and v2 alike.
- A required array left nil goes out as `null`. `NewSessionRequest{}` sends `"mcpServers":null`, but the schema requires an array.
  On the receiving side this SDK's own `Validate` rejects that request with "mcpServers is required". v2's default would send `[]`, which is what the schema asks for.

### Where v2 is worth having

- Union decoding is the hot path. Today it decodes each payload into a map and then decodes again into the variant, and the fallback paths decode more times.
  `UnmarshalJSONFrom` on a `jsontext.Decoder` can read the discriminator and decode once. Not measured here.
- Explicit options at the transport boundary would state the SDK's strictness in one place. The `schema-v1.25.0` notes say receivers reject malformed values they act on, which fits v2's inbound defaults.
- `json.RawMessage` in the exported API is already `jsontext.Value`, so the public surface does not have to change.

### Costs

- Every difference in the table above is a wire change for somebody. Five of them change what this SDK sends.
- Importing `encoding/json/v2` or `encoding/json/jsontext` stops the module from building with `GOEXPERIMENT=nojsonv2`, which was confirmed with the probe.
  The Go 1.27 notes expect that opt-out to go away, so this cost is temporary.
- Generated `UnmarshalJSON` methods would need regenerating; editing the 28 assertions by hand would fight `make version`.

### Assessment

0007 is right to leave the codec alone in this change. A separate record, after the go/ast port, can take it in order:
(Note added 2026-10-09: the owner has since decided to stay on v1. See [After this report](#after-this-report).)

1. Fix the two defects above and decide the nil-slice behavior on purpose.
2. Extend the golden suite to zero values, optional booleans, HTML-sensitive text, case-variant and duplicate names, and invalid UTF-8, pinned to current behavior.
3. Have the generator emit `MarshalJSONTo` and `UnmarshalJSONFrom` beside the v1 methods. go/ast emission makes that easier than Jennifer did.
4. Choose the transport options in `connection.go` explicitly, at least `Deterministic(true)`, and take the inbound strictness as a protocol decision.

## Recommended amendments

These are proposals. None of them had been applied when this report was written. (Note added 2026-10-09: all are now in the records. See [After this report](#after-this-report).)

0005 PLAN:

1. Step 3 changes the `mdsh` directive on `README.md` line 14 as well as the install line, or removes the `mdsh` block.
2. Step 2 says not to edit `docs/README.md` or any record.
3. Rollout names a branch and a commit per phase. The uncommitted record edits are committed first.

0005 MADR:

1. One sentence: the Apache-2.0 `LICENSE` and its copyright notice stay.

0006 MADR:

1. Context lists `dispatch_helpers.go`.
2. The outcome names the layout technique: comment and line positions from a synthetic `token.File`, line offsets below `1<<30`, and a final `go/format.Source` pass.

0006 PLAN:

1. Port order: constants, dispatch, then types and helpers together.
2. Add `TestGeneratedFilesUpToDate` and the layout tests as standing checks, and record that each was seen to fail.
3. Scope `main.go` to include returning errors and the `findRepoRoot` fix, or keep them out explicitly.

0007 MADR and PLAN:

1. The JSON option lists the measured differences, or links this report.
2. Rollback is defined against a committed starting point.
3. A follow-up record owns the output-changing cleanups: union doc placement, blank lines between functions, `var match bool = true`, the ignored decode error, and the `_rm` branch.

## After this report

Added 2026-10-09. The observations above are unchanged.

- The owner decided that the SDK stays on `encoding/json` v1, not only for this separation. The amendment in
  [0007-MADR-self-contained-acp-sdk.md](../decisions/0007-MADR-self-contained-acp-sdk.md) records it. Assessment steps 3 and 4 above, the v2 methods and the v2
  transport options, are not taken.
- Assessment steps 1 and 2, the two wire defects and the wider golden tests, carry over on v1, together with the source quirks, into
  [0009-MADR-fix-generated-output-on-json-v1.md](../decisions/0009-MADR-fix-generated-output-on-json-v1.md) and
  [0009-PLAN-fix-generated-output-on-json-v1.md](../decisions/0009-PLAN-fix-generated-output-on-json-v1.md), both proposed.
- The recommended amendments are written into the 0005, 0006, and 0007 MADRs and PLANs as dated amendments. The PLANs remain `proposed`.
- Counting descriptions from the unstable schema as well, 24 of the 28 union types lack a doc comment, not only the 18 with a stable-schema description.
- Correction: `Validate` checks `cwd` before `mcpServers`, so the zero `NewSessionRequest{}` is rejected for `cwd`. The "mcpServers is required" rejection
  applies to a request with `Cwd` set and `McpServers` nil, which still sends `"mcpServers":null`.

## Appendix A: the go/ast generator

Module `github.com/maccavelli/acp-go-sdk2/cmd/generate`, `go 1.27.2`, no `require`, no `go.sum`. Files not listed here are unchanged.
`internal/emit/jenwrap.go` is deleted. `internal/emit/doc.go` is unchanged. `internal/ir/ir.go` changes only its import path.

Indentation below is four spaces so this page passes the repository's Markdown lint. `gofmt` turns it back into tabs.
The text of each block, after `gofmt`, is byte-identical to the file that passed the checks above.

### `cmd/generate/go.mod`

```text
module github.com/maccavelli/acp-go-sdk2/cmd/generate

go 1.27.2
```

### `cmd/generate/main.go`

```go
// Command generate reads the ACP JSON schemas and writes the Go bindings.
package main

import (
    "flag"
    "fmt"
    "os"
    "path/filepath"

    "github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/emit"
    "github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/load"
)

func main() {
    if err := run(); err != nil {
        fmt.Fprintln(os.Stderr, "generate:", err)
        os.Exit(1)
    }
}

func run() error {
    schemaDir := flag.String("schema", "", "path to schema directory (defaults to <repo>/schema)")
    outDir := flag.String("out", "", "output directory for generated go files (defaults to <repo>)")
    flag.Parse()

    if *schemaDir == "" || *outDir == "" {
        repoRoot, err := findRepoRoot()
        if err != nil {
            return err
        }
        if *schemaDir == "" {
            *schemaDir = filepath.Join(repoRoot, "schema")
        }
        if *outDir == "" {
            *outDir = repoRoot
        }
    }
    if err := os.MkdirAll(*outDir, 0o750); err != nil {
        return err
    }

    meta, schema, err := loadSchema(*schemaDir)
    if err != nil {
        return err
    }

    if err := emit.WriteConstants(*outDir, meta); err != nil {
        return err
    }
    if err := emit.WriteTypes(*outDir, schema, meta); err != nil {
        return err
    }
    if err := emit.WriteDispatch(*outDir, schema, meta); err != nil {
        return err
    }
    // Emit helpers after types so they can reference generated structs.
    return emit.WriteHelpers(*outDir, schema, meta)
}

// loadSchema reads the stable schema and, when present, merges the unstable one.
func loadSchema(schemaDir string) (*load.Meta, *load.Schema, error) {
    meta, err := load.ReadMeta(schemaDir)
    if err != nil {
        return nil, nil, err
    }
    schema, err := load.ReadSchema(schemaDir)
    if err != nil {
        return nil, nil, err
    }
    unstableMeta, unstableMetaFound, err := load.ReadMetaUnstable(schemaDir)
    if err != nil {
        return nil, nil, err
    }
    unstableSchema, unstableSchemaFound, err := load.ReadSchemaUnstable(schemaDir)
    if err != nil {
        return nil, nil, err
    }
    if unstableMetaFound != unstableSchemaFound {
        return nil, nil, fmt.Errorf("unstable schema/meta mismatch: meta found=%v schema found=%v", unstableMetaFound, unstableSchemaFound)
    }
    if !unstableMetaFound {
        return meta, schema, nil
    }
    return load.MergeStableAndUnstable(meta, schema, unstableMeta, unstableSchema)
}

// findRepoRoot walks up from the working directory to the first directory
// with schema/meta.json and schema/schema.json.
func findRepoRoot() (string, error) {
    cwd, err := os.Getwd()
    if err != nil {
        return "", err
    }
    for dir := cwd; ; {
        if hasSchema(dir) {
            return dir, nil
        }
        parent := filepath.Dir(dir)
        if parent == dir {
            return "", fmt.Errorf("could not locate repository root from %q; ensure schema files exist", cwd)
        }
        dir = parent
    }
}

func hasSchema(dir string) bool {
    for _, name := range []string{"meta.json", "schema.json"} {
        if _, err := os.Stat(filepath.Join(dir, "schema", name)); err != nil {
            return false
        }
    }
    return true
}
```

### `cmd/generate/main_test.go`

```go
package main

import (
    "bytes"
    "os"
    "path/filepath"
    "testing"

    "github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/emit"
)

// repoRoot is the SDK module root, two directories above cmd/generate.
const repoRoot = "../.."

var generatedFiles = []string{
    "constants_gen.go",
    "types_gen.go",
    "agent_gen.go",
    "client_gen.go",
    "helpers_gen.go",
}

// TestGeneratedFilesUpToDate regenerates the bindings from schema/ and
// requires the committed *_gen.go files to match byte for byte.
func TestGeneratedFilesUpToDate(t *testing.T) {
    meta, schema, err := loadSchema(filepath.Join(repoRoot, "schema"))
    if err != nil {
        t.Fatal(err)
    }
    out := t.TempDir()
    if err := emit.WriteConstants(out, meta); err != nil {
        t.Fatal(err)
    }
    if err := emit.WriteTypes(out, schema, meta); err != nil {
        t.Fatal(err)
    }
    if err := emit.WriteDispatch(out, schema, meta); err != nil {
        t.Fatal(err)
    }
    if err := emit.WriteHelpers(out, schema, meta); err != nil {
        t.Fatal(err)
    }
    for _, name := range generatedFiles {
        got, err := os.ReadFile(filepath.Join(out, name))
        if err != nil {
            t.Fatal(err)
        }
        want, err := os.ReadFile(filepath.Join(repoRoot, name))
        if err != nil {
            t.Fatal(err)
        }
        if !bytes.Equal(got, want) {
            t.Errorf("%s is out of date with schema/; run make version (first difference at line %d)", name, firstDiffLine(got, want))
        }
    }
}

// firstDiffLine returns the 1-based line of the first byte where a and b differ.
func firstDiffLine(a, b []byte) int {
    n := min(len(a), len(b))
    i := 0
    for i < n && a[i] == b[i] {
        i++
    }
    return bytes.Count(a[:i], []byte("\n")) + 1
}
```

### `cmd/generate/internal/emit/ast.go`

```go
package emit

import (
    "go/ast"
    "go/token"
    "strconv"
    "strings"
)

// Small constructors for go/ast nodes. They keep the emitters readable and
// return fresh nodes on every call, so no node is shared between two places
// in a tree.

// placeholderPos marks a position whose only job is to be valid, such as the
// '=' of a type alias or the '(' of a grouped declaration. The file layout
// replaces it with a real position.
const placeholderPos token.Pos = 1

func id(name string) *ast.Ident { return ast.NewIdent(name) }

// sel builds x.name1.name2...
func sel(x ast.Expr, names ...string) ast.Expr {
    for _, n := range names {
        x = &ast.SelectorExpr{X: x, Sel: id(n)}
    }
    return x
}

// dot builds name.names[0].names[1]...
func dot(name string, names ...string) ast.Expr { return sel(id(name), names...) }

func call(fun ast.Expr, args ...ast.Expr) *ast.CallExpr { return &ast.CallExpr{Fun: fun, Args: args} }

func str(s string) *ast.BasicLit { return &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(s)} }

func intLit(n int) *ast.BasicLit { return &ast.BasicLit{Kind: token.INT, Value: strconv.Itoa(n)} }

func boolLit(b bool) *ast.Ident { return id(strconv.FormatBool(b)) }

func nilExpr() *ast.Ident { return id("nil") }

func star(x ast.Expr) *ast.StarExpr { return &ast.StarExpr{X: x} }

func addr(x ast.Expr) *ast.UnaryExpr { return &ast.UnaryExpr{Op: token.AND, X: x} }

func not(x ast.Expr) *ast.UnaryExpr { return &ast.UnaryExpr{Op: token.NOT, X: x} }

func binary(x ast.Expr, op token.Token, y ast.Expr) *ast.BinaryExpr {
    return &ast.BinaryExpr{X: x, Op: op, Y: y}
}

func paren(x ast.Expr) *ast.ParenExpr { return &ast.ParenExpr{X: x} }

func index(x, i ast.Expr) *ast.IndexExpr { return &ast.IndexExpr{X: x, Index: i} }

func typeAssert(x, typ ast.Expr) *ast.TypeAssertExpr { return &ast.TypeAssertExpr{X: x, Type: typ} }

func sliceOf(elt ast.Expr) *ast.ArrayType { return &ast.ArrayType{Elt: elt} }

func mapOf(key, value ast.Expr) *ast.MapType { return &ast.MapType{Key: key, Value: value} }

// byteSlice is []byte.
func byteSlice() *ast.ArrayType { return sliceOf(id("byte")) }

// bytesOf is []byte("s").
func bytesOf(s string) *ast.CallExpr { return call(byteSlice(), str(s)) }

// emptyBytes is []byte{}.
func emptyBytes() *ast.CompositeLit { return composite(byteSlice()) }

// rawMap is map[string]<value>.
func rawMap(value ast.Expr) *ast.MapType { return mapOf(id("string"), value) }

func composite(typ ast.Expr, elts ...ast.Expr) *ast.CompositeLit {
    return &ast.CompositeLit{Type: typ, Elts: elts}
}

func kv(key, value ast.Expr) *ast.KeyValueExpr { return &ast.KeyValueExpr{Key: key, Value: value} }

func exprs(xs ...ast.Expr) []ast.Expr { return xs }

func define(lhs []ast.Expr, rhs ...ast.Expr) *ast.AssignStmt {
    return &ast.AssignStmt{Lhs: lhs, Tok: token.DEFINE, Rhs: rhs}
}

func assign(lhs []ast.Expr, rhs ...ast.Expr) *ast.AssignStmt {
    return &ast.AssignStmt{Lhs: lhs, Tok: token.ASSIGN, Rhs: rhs}
}

func exprStmt(x ast.Expr) *ast.ExprStmt { return &ast.ExprStmt{X: x} }

func ret(results ...ast.Expr) *ast.ReturnStmt { return &ast.ReturnStmt{Results: results} }

func block(stmts ...ast.Stmt) *ast.BlockStmt { return &ast.BlockStmt{List: stmts} }

func ifStmt(init ast.Stmt, cond ast.Expr, body ...ast.Stmt) *ast.IfStmt {
    return &ast.IfStmt{Init: init, Cond: cond, Body: block(body...)}
}

func ifElse(init ast.Stmt, cond ast.Expr, then, els []ast.Stmt) *ast.IfStmt {
    s := ifStmt(init, cond, then...)
    s.Else = block(els...)
    return s
}

func incr(x ast.Expr) *ast.IncDecStmt { return &ast.IncDecStmt{X: x, Tok: token.INC} }

func rangeStmt(key, value, x ast.Expr, body ...ast.Stmt) *ast.RangeStmt {
    return &ast.RangeStmt{Key: key, Value: value, Tok: token.DEFINE, X: x, Body: block(body...)}
}

func switchStmt(tag ast.Expr, clauses ...ast.Stmt) *ast.SwitchStmt {
    return &ast.SwitchStmt{Tag: tag, Body: block(clauses...)}
}

// caseOf is a case clause. A nil list is the default clause.
func caseOf(list []ast.Expr, body ...ast.Stmt) *ast.CaseClause {
    return &ast.CaseClause{List: list, Body: body}
}

// varStmt is var name typ [= value].
func varStmt(name string, typ ast.Expr, value ast.Expr) *ast.DeclStmt {
    spec := &ast.ValueSpec{Names: []*ast.Ident{id(name)}, Type: typ}
    if value != nil {
        spec.Values = []ast.Expr{value}
    }
    return &ast.DeclStmt{Decl: &ast.GenDecl{Tok: token.VAR, Specs: []ast.Spec{spec}}}
}

// typeStmt is a local type declaration, type name typ.
func typeStmt(name string, typ ast.Expr) *ast.DeclStmt {
    return &ast.DeclStmt{Decl: &ast.GenDecl{Tok: token.TYPE, Specs: []ast.Spec{&ast.TypeSpec{Name: id(name), Type: typ}}}}
}

// param is one parameter or result. An empty name leaves it unnamed.
func param(name string, typ ast.Expr) *ast.Field {
    f := &ast.Field{Type: typ}
    if name != "" {
        f.Names = []*ast.Ident{id(name)}
    }
    return f
}

func fields(list ...*ast.Field) *ast.FieldList { return &ast.FieldList{List: list} }

// results is nil for no results, so the printer omits the list.
func results(list ...*ast.Field) *ast.FieldList {
    if len(list) == 0 {
        return nil
    }
    return fields(list...)
}

func funcType(params []*ast.Field, res ...*ast.Field) *ast.FuncType {
    return &ast.FuncType{Params: fields(params...), Results: results(res...)}
}

// method is func (recv) name(params) results { body }.
func method(recv *ast.Field, name string, typ *ast.FuncType, body ...ast.Stmt) *ast.FuncDecl {
    return &ast.FuncDecl{Recv: fields(recv), Name: id(name), Type: typ, Body: block(body...)}
}

// function is func name(params) results { body }.
func function(name string, typ *ast.FuncType, body ...ast.Stmt) *ast.FuncDecl {
    return &ast.FuncDecl{Name: id(name), Type: typ, Body: block(body...)}
}

// typeDecl is type name typ.
func typeDecl(name string, typ ast.Expr) *ast.GenDecl {
    return &ast.GenDecl{Tok: token.TYPE, Specs: []ast.Spec{&ast.TypeSpec{Name: id(name), Type: typ}}}
}

// aliasDecl is type name = typ.
func aliasDecl(name string, typ ast.Expr) *ast.GenDecl {
    return &ast.GenDecl{Tok: token.TYPE, Specs: []ast.Spec{&ast.TypeSpec{Name: id(name), Assign: placeholderPos, Type: typ}}}
}

// constSpec is name [typ] = value.
func constSpec(name string, typ ast.Expr, value ast.Expr) *ast.ValueSpec {
    return &ast.ValueSpec{Names: []*ast.Ident{id(name)}, Type: typ, Values: []ast.Expr{value}}
}

// constDecl is a const declaration. A grouped declaration prints in
// parentheses even when it holds one spec.
func constDecl(grouped bool, specs ...*ast.ValueSpec) *ast.GenDecl {
    d := &ast.GenDecl{Tok: token.CONST}
    for _, s := range specs {
        d.Specs = append(d.Specs, s)
    }
    if grouped {
        d.Lparen = placeholderPos
    }
    return d
}

// structTag is a json struct tag literal, `json:"name"`.
func structTag(name string) *ast.BasicLit {
    s := jsonStructTag + ":" + strconv.Quote(name)
    if strconv.CanBackquote(s) {
        s = "`" + s + "`"
    } else {
        s = strconv.Quote(s)
    }
    return &ast.BasicLit{Kind: token.STRING, Value: s}
}

// docLines turns comment text into // lines. Text that already starts with
// a comment marker is kept as written.
func docLines(lines ...string) *ast.CommentGroup {
    if len(lines) == 0 {
        return nil
    }
    g := &ast.CommentGroup{}
    for _, line := range lines {
        g.List = append(g.List, &ast.Comment{Text: commentText(line)})
    }
    return g
}

func commentText(line string) string {
    switch {
    case strings.HasPrefix(line, "//") || strings.HasPrefix(line, "/*"):
        return line
    case line == "":
        return "//"
    default:
        return "// " + line
    }
}
```

### `cmd/generate/internal/emit/file.go`

```go
package emit

import (
    "bytes"
    "fmt"
    "go/ast"
    "go/format"
    "go/token"
    "maps"
    "os"
    "path"
    "path/filepath"
    "slices"
    "strconv"
)

const (
    generatedHeader = "Code generated by acp-go-generator; DO NOT EDIT."
    // sdkPackage is the package name of every generated file.
    sdkPackage = "acp"
)

// goFile collects the top-level items of one generated file in output order:
// comment lines, blank lines, and declarations.
//
// go/printer places comments and line breaks by token position. A tree built
// in memory has none, so render lays the items out on the lines they would
// occupy in written source before printing, then formats the result with
// go/format.
type goFile struct {
    pkg     string
    imports map[string]bool
    items   []fileItem
}

// fileItem is one comment line, one blank line, or one declaration.
type fileItem struct {
    comment string
    blank   bool
    decl    ast.Decl
}

func newFile() *goFile {
    return &goFile{pkg: sdkPackage, imports: map[string]bool{}}
}

// comment adds one comment line per argument. Comment lines directly above a
// declaration become its doc comment.
func (f *goFile) comment(lines ...string) {
    for _, line := range lines {
        f.items = append(f.items, fileItem{comment: line})
    }
}

// line adds a blank line.
func (f *goFile) line() { f.items = append(f.items, fileItem{blank: true}) }

// add appends a declaration.
func (f *goFile) add(d ast.Decl) { f.items = append(f.items, fileItem{decl: d}) }

// qual returns the qualified identifier name from the package at importPath
// and records the import.
func (f *goFile) qual(importPath, name string) ast.Expr {
    f.imports[importPath] = true
    return dot(path.Base(importPath), name)
}

// write renders the file and writes it to outDir/name.
func (f *goFile) write(outDir, name string) error {
    src, err := f.render()
    if err != nil {
        return fmt.Errorf("%s: %w", name, err)
    }
    return os.WriteFile(filepath.Join(outDir, name), src, generatedFileMode)
}

// render prints the file and formats it.
func (f *goFile) render() ([]byte, error) {
    fset := token.NewFileSet()
    file := f.layout(fset)
    var buf bytes.Buffer
    if err := format.Node(&buf, fset, file); err != nil {
        return nil, fmt.Errorf("print: %w", err)
    }
    // A second pass parses real positions back, so gofmt rules that depend
    // on them (doc comment reformatting) apply as they do to written source.
    out, err := format.Source(buf.Bytes())
    if err != nil {
        return nil, fmt.Errorf("format: %w", err)
    }
    return out, nil
}

func (f *goFile) importDecl() *ast.GenDecl {
    if len(f.imports) == 0 {
        return nil
    }
    d := &ast.GenDecl{Tok: token.IMPORT}
    for _, p := range slices.Sorted(maps.Keys(f.imports)) {
        d.Specs = append(d.Specs, &ast.ImportSpec{Path: &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(p)}})
    }
    return d
}

// layout builds the *ast.File and gives every line-starting token a position.
func (f *goFile) layout(fset *token.FileSet) *ast.File {
    l := &layout{}
    file := &ast.File{Name: id(f.pkg)}

    l.doc(docLines(generatedHeader))
    l.next() // blank line, so the header is not the package doc
    l.at(&file.Package, l.next())
    if imp := f.importDecl(); imp != nil {
        l.next()
        l.genDecl(imp)
        file.Decls = append(file.Decls, imp)
    }
    l.next()

    var pending *ast.CommentGroup
    for _, it := range f.items {
        switch {
        case it.decl != nil:
            setDoc(it.decl, pending)
            pending = nil
            l.decl(it.decl)
            file.Decls = append(file.Decls, it.decl)
        case it.blank:
            pending = nil
            l.next()
        default:
            c := &ast.Comment{Text: commentText(it.comment)}
            l.at(&c.Slash, l.next())
            if pending == nil {
                pending = &ast.CommentGroup{}
                l.comments = append(l.comments, pending)
            }
            pending.List = append(pending.List, c)
        }
    }

    l.apply(fset)
    file.Comments = l.comments
    return file
}

func setDoc(d ast.Decl, g *ast.CommentGroup) {
    switch d := d.(type) {
    case *ast.FuncDecl:
        d.Doc = g
    case *ast.GenDecl:
        d.Doc = g
    }
}

// lineBytes is the byte length of a laid-out line, longer than any line the
// generator prints.
//
// Between two positioned tokens go/printer advances its idea of the source
// offset by the bytes it prints, and it flushes a pending comment once that
// offset passes the comment. Lines inside a function body therefore also
// reserve the printed size of the body, so the estimate never reaches the
// doc comment of the next declaration. Offsets stay far below the printer's
// own 1<<30 "no more comments" sentinel.
const lineBytes = 1 << 12

// layout hands out line numbers in output order and records which positions
// get which line. Positions are resolved once the line count is known.
type layout struct {
    line     int
    marks    []mark
    comments []*ast.CommentGroup
    extra    map[int]int // line -> bytes reserved beyond lineBytes
}

// reserve adds n bytes to the length of line.
func (l *layout) reserve(line, n int) {
    if l.extra == nil {
        l.extra = map[int]int{}
    }
    l.extra[line] += n
}

// printedSize is the length of n printed without positions.
func printedSize(n ast.Node) int {
    var buf bytes.Buffer
    if err := format.Node(&buf, token.NewFileSet(), n); err != nil {
        panic(fmt.Sprintf("emit: print %T: %v", n, err))
    }
    return buf.Len()
}

type mark struct {
    pos  *token.Pos
    line int
}

func (l *layout) next() int {
    l.line++
    return l.line
}

func (l *layout) at(p *token.Pos, line int) {
    if p != nil {
        l.marks = append(l.marks, mark{pos: p, line: line})
    }
}

// doc gives each comment of g its own line.
func (l *layout) doc(g *ast.CommentGroup) {
    if g == nil {
        return
    }
    for _, c := range g.List {
        l.at(&c.Slash, l.next())
    }
    l.comments = append(l.comments, g)
}

func (l *layout) apply(fset *token.FileSet) {
    lines := make([]int, l.line+1)
    for i := 1; i < len(lines); i++ {
        lines[i] = lines[i-1] + lineBytes + l.extra[i]
    }
    tf := fset.AddFile("", -1, lines[l.line]+lineBytes)
    if !tf.SetLines(lines) {
        panic("emit: invalid line table")
    }
    for _, m := range l.marks {
        *m.pos = tf.LineStart(m.line)
    }
}

func (l *layout) decl(d ast.Decl) {
    switch d := d.(type) {
    case *ast.FuncDecl:
        // Braces on different lines keep go/printer from collapsing a short
        // body onto the signature line.
        size := printedSize(d.Body)
        line := l.next()
        l.reserve(line, size)
        l.at(&d.Type.Func, line)
        l.at(&d.Body.Lbrace, line)
        l.literals(d.Body, size)
        l.at(&d.Body.Rbrace, l.next())
    case *ast.GenDecl:
        l.genDecl(d)
    }
}

func (l *layout) genDecl(d *ast.GenDecl) {
    line := l.next()
    l.at(&d.TokPos, line)
    if d.Lparen.IsValid() || len(d.Specs) != 1 {
        l.at(&d.Lparen, line)
        for _, s := range d.Specs {
            l.spec(s, l.next())
        }
        l.at(&d.Rparen, l.next())
        return
    }
    l.spec(d.Specs[0], line)
}

func (l *layout) spec(s ast.Spec, line int) {
    switch s := s.(type) {
    case *ast.ImportSpec:
        l.at(&s.Path.ValuePos, line)
    case *ast.ValueSpec:
        l.at(&s.Names[0].NamePos, line)
    case *ast.TypeSpec:
        l.at(&s.Name.NamePos, line)
        if s.Assign.IsValid() {
            l.at(&s.Assign, line)
        }
        switch t := s.Type.(type) {
        case *ast.StructType:
            l.members(t.Fields, line)
        case *ast.InterfaceType:
            l.members(t.Methods, line)
        }
    }
}

// members puts each struct field or interface method on its own line after
// its doc comment. An empty list stays on the opening line, as struct{}.
func (l *layout) members(list *ast.FieldList, line int) {
    l.at(&list.Opening, line)
    if len(list.List) == 0 {
        l.at(&list.Closing, line)
        return
    }
    for _, fld := range list.List {
        l.doc(fld.Doc)
        fline := l.next()
        if len(fld.Names) > 0 {
            l.at(&fld.Names[0].NamePos, fline)
        } else {
            l.at(firstPos(fld.Type), fline)
        }
    }
    l.at(&list.Closing, l.next())
}

// literals puts the elements of every composite literal with more than one
// element on their own lines. Each line reserves size bytes, the printed
// size of the enclosing body.
func (l *layout) literals(n ast.Node, size int) {
    line := func() int {
        ln := l.next()
        l.reserve(ln, size)
        return ln
    }
    ast.Inspect(n, func(n ast.Node) bool {
        lit, ok := n.(*ast.CompositeLit)
        if !ok || len(lit.Elts) < 2 {
            return true
        }
        l.at(&lit.Lbrace, line())
        for _, e := range lit.Elts {
            l.at(firstPos(e), line())
            l.literals(e, size)
        }
        l.at(&lit.Rbrace, line())
        return false
    })
}

// firstPos returns the position field of the first token of x, or nil.
func firstPos(x ast.Expr) *token.Pos {
    switch x := x.(type) {
    case *ast.Ident:
        return &x.NamePos
    case *ast.BasicLit:
        return &x.ValuePos
    case *ast.KeyValueExpr:
        return firstPos(x.Key)
    case *ast.UnaryExpr:
        return &x.OpPos
    case *ast.StarExpr:
        return &x.Star
    case *ast.SelectorExpr:
        return firstPos(x.X)
    case *ast.CallExpr:
        return firstPos(x.Fun)
    case *ast.CompositeLit:
        if x.Type != nil {
            return firstPos(x.Type)
        }
        return &x.Lbrace
    default:
        return nil
    }
}
```

### `cmd/generate/internal/emit/constants.go`

```go
package emit

import (
    "go/ast"
    "strings"

    "github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/ir"
    "github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/load"
    "github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/util"
)

const (
    schemaTypeObject  = "object"
    schemaTypeArray   = "array"
    schemaTypeString  = "string"
    schemaTypeInteger = "integer"
    schemaTypeNumber  = "number"
    schemaTypeBoolean = "boolean"
    schemaTypeNull    = "null"
    jsonStructTag     = "json"
    unionContentBlock = "ContentBlock"
    wireSessionCancel = "session/cancel"
    nameSessionUpdate = "SessionUpdate"
    generatedFileMode = 0o600

    pkgContext = "context"
    pkgErrors  = "errors"
    pkgFmt     = "fmt"
    pkgJSON    = "encoding/json"
)

// WriteConstants writes the version and method constants to constants_gen.go.
func WriteConstants(outDir string, meta *load.Meta) error {
    f := newFile()
    f.comment("ProtocolVersionNumber is the ACP protocol version supported by this SDK.")
    f.add(constDecl(false, constSpec("ProtocolVersionNumber", nil, intLit(meta.Version))))

    f.comment("Agent method names")
    f.add(methodConsts("AgentMethod", meta.AgentMethods))

    f.comment("Client method names")
    f.add(methodConsts("ClientMethod", meta.ClientMethods))

    return f.write(outDir, "constants_gen.go")
}

// methodConsts declares prefix+Name = "wire" for each method, sorted by key.
func methodConsts(prefix string, methods map[string]string) *ast.GenDecl {
    var specs []*ast.ValueSpec
    for _, k := range ir.SortedKeys(methods) {
        specs = append(specs, constSpec(prefix+toExportedConst(k), nil, str(methods[k])))
    }
    return constDecl(true, specs...)
}

// toExportedConst turns a meta method key such as session_set_mode into
// SessionSetMode.
func toExportedConst(s string) string {
    parts := strings.Split(s, "_")
    for i := range parts {
        parts[i] = util.TitleWord(parts[i])
    }
    return strings.Join(parts, "")
}
```

### `cmd/generate/internal/emit/helpers.go`

```go
package emit

import (
    "fmt"
    "go/ast"
    "maps"
    "slices"
    "strings"

    "github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/load"
    "github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/util"
)

type helperVariant struct {
    fieldName string
    typeName  string
    discKey   string
    discValue string
    required  []string
    props     map[string]*load.Definition
}

func collectHelperVariants(name string, def *load.Definition, schema *load.Schema, usedTypeNames map[string]bool) []helperVariant {
    if def == nil || def.DocsIgnore || len(def.OneOf) == 0 {
        return nil
    }
    // Skip string-const unions
    if isStringConstUnion(def) {
        return nil
    }
    // Skip generating New... helpers for unions that have stable, static helpers
    // implemented in helpers.go.
    switch name {
    case unionContentBlock, "ToolCallContent", nameSessionUpdate:
        return nil
    }
    // Build variant info using the same logic as the types emitter (emitUnion)
    // to ensure type names match exactly.
    discKey := discriminatorKey(schema, def, def.OneOf)
    variants := []helperVariant{}
    for idx, v := range def.OneOf {
        if hv, ok := oneHelperVariant(name, schema, idx, v, discKey, usedTypeNames); ok {
            variants = append(variants, hv)
        }
    }
    return variants
}

func oneHelperVariant(name string, schema *load.Schema, idx int, v *load.Definition, discKey string, usedTypeNames map[string]bool) (helperVariant, bool) {
    if v == nil {
        return helperVariant{}, false
    }
    // Match the types emitter's allOf-to-ref shortcut.
    ref := v.Ref
    if ref == "" {
        ref = singleRefVariant(v)
    }
    v = expandAllOf(schema, v)

    // Compute type name matching the types emitter (emitUnion).
    tname := ""
    if after, ok := strings.CutPrefix(ref, "#/$defs/"); ok {
        tname = after
    } else if v.Title != "" {
        tname = generateNestedTypeName(name, v.Title, usedTypeNames)
        usedTypeNames[tname] = true
    } else {
        if discKey != "" {
            if pd := v.Properties[discKey]; pd != nil && pd.Const != nil {
                tname = generateNestedTypeName(name, fmt.Sprint(pd.Const), usedTypeNames)
                usedTypeNames[tname] = true
            }
        }
        if tname == "" {
            tname = fmt.Sprintf("%sVariant%d", name, idx+1)
            usedTypeNames[tname] = true
        }
    }
    tname = util.ToExportedField(tname)

    // Derive field name with same priority as types emitter.
    fieldName := ""
    dv := ""
    if discKey != "" {
        if pd := v.Properties[discKey]; pd != nil && pd.Const != nil {
            dv = fmt.Sprint(pd.Const)
            fieldName = util.ToExportedField(dv)
        }
    }
    if fieldName == "" && v.Title != "" {
        fieldName = util.ToExportedField(v.Title)
    }
    if fieldName == "" {
        fieldName = util.ToExportedField(tname)
    }
    return helperVariant{
        fieldName: fieldName,
        typeName:  tname,
        discKey:   discKey,
        discValue: dv,
        required:  slices.Clone(v.Required),
        props:     v.Properties,
    }, true
}

// emitHelperConstructors emits func New<Union><Field>(required...) <Union> per variant.
func emitHelperConstructors(f *goFile, name string, variants []helperVariant) {
    for _, vi := range variants {
        // Parameters: every required property except the const discriminator.
        var params []*ast.Field
        assigns := map[string]ast.Expr{}
        for _, rk := range vi.required {
            if rk == vi.discKey {
                continue
            }
            pd := vi.props[rk]
            if pd == nil {
                continue
            }
            params = append(params, param(rk, typeExpr(pd)))
            assigns[util.ToExportedField(rk)] = id(rk)
        }
        if vi.discKey != "" && vi.discValue != "" {
            assigns[util.ToExportedField(vi.discKey)] = str(vi.discValue)
        }
        var elts []ast.Expr
        for _, field := range slices.Sorted(maps.Keys(assigns)) {
            elts = append(elts, kv(id(field), assigns[field]))
        }
        variant := addr(composite(id(vi.typeName), elts...))

        f.comment(fmt.Sprintf("New%s%s constructs a %s using the '%s' variant.", name, vi.fieldName, name, vi.discValue))
        f.add(function("New"+name+vi.fieldName, funcType(params, param("", id(name))),
            ret(composite(id(name), kv(id(vi.fieldName), variant))),
        ))
        f.line()
    }
}

// WriteHelpers emits helpers_gen.go with constructor helpers for union variants.
func WriteHelpers(outDir string, schema *load.Schema, _ *load.Meta) error {
    f := newFile()

    // Schema-driven helpers: New<Union><Variant>(required fields only).
    // Track used type names to match the types emitter's naming (generateNestedTypeName).
    usedTypeNames := make(map[string]bool, len(schema.Defs))
    for k := range schema.Defs {
        usedTypeNames[k] = true
    }
    for _, name := range slices.Sorted(maps.Keys(schema.Defs)) {
        emitHelperConstructors(f, name, collectHelperVariants(name, schema.Defs[name], schema, usedTypeNames))
    }
    return f.write(outDir, "helpers_gen.go")
}
```

### `cmd/generate/internal/emit/dispatch_helpers.go`

```go
package emit

import (
    "go/ast"
    "go/token"

    "github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/ir"
)

// invalidParams returns nil, NewInvalidParams(map[string]any{"error": err.Error()}).
func invalidParams() ast.Stmt {
    return ret(nilExpr(), call(id("NewInvalidParams"),
        composite(rawMap(id("any")), kv(str("error"), call(dot("err", "Error")))),
    ))
}

// toRequestError returns nil, toReqErr(err).
func toRequestError() ast.Stmt { return ret(nilExpr(), call(id("toReqErr"), id("err"))) }

// methodNotFound returns nil, NewMethodNotFound(method).
func methodNotFound() ast.Stmt { return ret(nilExpr(), call(id("NewMethodNotFound"), id("method"))) }

// errNotNil is err != nil.
func errNotNil() ast.Expr { return binary(id("err"), token.NEQ, nilExpr()) }

// unmarshalValidate decodes params into p and validates it.
func unmarshalValidate(f *goFile, typeName string) []ast.Stmt {
    return []ast.Stmt{
        varStmt("p", id(typeName), nil),
        ifStmt(define(exprs(id("err")), call(f.qual(pkgJSON, "Unmarshal"), id("params"), addr(id("p")))), errNotNil(),
            invalidParams()),
        ifStmt(define(exprs(id("err")), call(dot("p", "Validate"))), errNotNil(),
            invalidParams()),
    }
}

// agentAssert returns the interface assertion prelude for a binding and the
// receiver the method is called on.
func agentAssert(f *goFile, binding ir.MethodBinding, methodName, paramType, respType string, hasResponse bool) ([]ast.Stmt, ast.Expr) {
    switch binding {
    case ir.BindAgentLoader:
        return namedAssert(dot("a", "agent"), "loader", "AgentLoader"), id("loader")
    case ir.BindAgentExperimental:
        return singleMethodAssert(f, dot("a", "agent"), "exp", methodName, paramType, respType, hasResponse), id("exp")
    default:
        return nil, dot("a", "agent")
    }
}

// clientAssert returns the interface assertion prelude for a binding and the
// receiver the method is called on.
func clientAssert(f *goFile, binding ir.MethodBinding, methodName, paramType, respType string, hasResponse bool) ([]ast.Stmt, ast.Expr) {
    switch binding {
    case ir.BindClientExperimental:
        return singleMethodAssert(f, dot("c", "client"), "exp", methodName, paramType, respType, hasResponse), id("exp")
    case ir.BindClientTerminal:
        return namedAssert(dot("c", "client"), "t", "ClientTerminal"), id("t")
    default:
        return nil, dot("c", "client")
    }
}

// namedAssert is name, ok := receiver.(iface); if !ok { return method not found }.
func namedAssert(receiver ast.Expr, name, iface string) []ast.Stmt {
    return []ast.Stmt{
        define(exprs(id(name), id("ok")), typeAssert(receiver, id(iface))),
        ifStmt(nil, not(id("ok")), methodNotFound()),
    }
}

// singleMethodAssert asserts receiver to an anonymous one-method interface.
func singleMethodAssert(f *goFile, receiver ast.Expr, name, methodName, paramType, respType string, hasResponse bool) []ast.Stmt {
    res := []*ast.Field{param("", id("error"))}
    if hasResponse {
        res = []*ast.Field{param("", id(respType)), param("", id("error"))}
    }
    iface := &ast.InterfaceType{Methods: fields(&ast.Field{
        Names: []*ast.Ident{id(methodName)},
        Type:  funcType([]*ast.Field{param("", f.qual(pkgContext, "Context")), param("", id(paramType))}, res...),
    })}
    return []ast.Stmt{
        define(exprs(id(name), id("ok")), typeAssert(receiver, iface)),
        ifStmt(nil, not(id("ok")), methodNotFound()),
    }
}

// recvCall is recv.method(ctx, p).
func recvCall(recv ast.Expr, methodName string) *ast.CallExpr {
    return call(sel(recv, methodName), id("ctx"), id("p"))
}

// callNoResult calls a method that returns only an error.
func callNoResult(recv ast.Expr, methodName string) []ast.Stmt {
    return []ast.Stmt{
        ifStmt(define(exprs(id("err")), recvCall(recv, methodName)), errNotNil(), toRequestError()),
        ret(nilExpr(), nilExpr()),
    }
}

// callWithResult calls a method that returns a response and an error.
func callWithResult(recv ast.Expr, methodName string) []ast.Stmt {
    return []ast.Stmt{
        define(exprs(id("resp"), id("err")), recvCall(recv, methodName)),
        ifStmt(nil, errNotNil(), toRequestError()),
        ret(id("resp"), nilExpr()),
    }
}
```

### `cmd/generate/internal/emit/dispatch.go`

```go
package emit

import (
    "go/ast"
    "go/token"
    "strings"

    "github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/ir"
    "github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/load"
)

// WriteDispatch emits agent_gen.go and client_gen.go with handlers and wrappers.
func WriteDispatch(outDir string, schema *load.Schema, meta *load.Meta) error {
    groups := ir.BuildMethodGroups(schema, meta)

    if err := writeAgentGen(outDir, groups, schema, meta); err != nil {
        return err
    }
    return writeClientGen(outDir, groups, schema, meta)
}

// sessionKey is string(p.SessionId).
func sessionKey() ast.Expr { return call(id("string"), dot("p", "SessionId")) }

// sessionCancel is a.sessionCancels[string(p.SessionId)].
func sessionCancel() ast.Expr { return index(dot("a", "sessionCancels"), sessionKey()) }

// lockCall is a.mu.Lock() or a.mu.Unlock().
func lockCall(name string) ast.Stmt { return exprStmt(call(dot("a", "mu", name))) }

// handleFunc is func (recv *recvType) handle(ctx, method, params) (any, *RequestError) { switch method { ... } }.
func handleFunc(f *goFile, recv, recvType string, cases []ast.Stmt) *ast.FuncDecl {
    cases = append(cases, caseOf(nil, methodNotFound()))
    return method(param(recv, star(id(recvType))), "handle",
        funcType(
            []*ast.Field{param("ctx", f.qual(pkgContext, "Context")), param("method", id("string")), param("params", f.qual(pkgJSON, "RawMessage"))},
            param("", id("any")), param("", star(id("RequestError"))),
        ),
        switchStmt(id("method"), cases...),
    )
}

// promptCase derives a cancellable context per session prompt, calls Prompt,
// then drops the cancel entry.
func promptCase(f *goFile, recv ast.Expr, methodName string) []ast.Stmt {
    return []ast.Stmt{
        varStmt("reqCtx", f.qual(pkgContext, "Context"), nil),
        varStmt("cancel", f.qual(pkgContext, "CancelFunc"), nil),
        assign(exprs(id("reqCtx"), id("cancel")), call(f.qual(pkgContext, "WithCancel"), id("ctx"))),
        lockCall("Lock"),
        ifStmt(define(exprs(id("prev"), id("ok")), sessionCancel()), id("ok"), exprStmt(call(id("prev")))),
        assign(exprs(sessionCancel()), id("cancel")),
        lockCall("Unlock"),
        define(exprs(id("resp"), id("err")), call(sel(recv, methodName), id("reqCtx"), id("p"))),
        lockCall("Lock"),
        exprStmt(call(id("delete"), dot("a", "sessionCancels"), sessionKey())),
        lockCall("Unlock"),
        ifStmt(nil, errNotNil(), toRequestError()),
        ret(id("resp"), nilExpr()),
    }
}

// cancelActivePrompt cancels the in-flight prompt context of the session, if any.
func cancelActivePrompt() []ast.Stmt {
    return []ast.Stmt{
        lockCall("Lock"),
        ifStmt(define(exprs(id("cn"), id("ok")), sessionCancel()), id("ok"),
            exprStmt(call(id("cn"))),
            exprStmt(call(id("delete"), dot("a", "sessionCancels"), sessionKey())),
        ),
        lockCall("Unlock"),
    }
}

func writeAgentGen(outDir string, groups ir.Groups, schema *load.Schema, meta *load.Meta) error {
    f := newFile()

    var cases []ast.Stmt
    for _, k := range ir.SortedKeys(meta.AgentMethods) {
        mi := groups["agent|"+meta.AgentMethods[k]]
        if mi == nil {
            continue
        }
        var body []ast.Stmt
        if mi.Notif != "" {
            body = append(body, unmarshalValidate(f, mi.Notif)...)
            // session/cancel also cancels any in-flight prompt ctx for the session.
            if mi.Method == wireSessionCancel {
                body = append(body, cancelActivePrompt()...)
            }
            callName := ir.DispatchMethodNameForNotification(k, mi.Notif)
            pre, recv := agentAssert(f, mi.Binding, callName, mi.Notif, "", false)
            body = append(body, pre...)
            body = append(body, callNoResult(recv, callName)...)
        } else if mi.Req != "" {
            respName := strings.TrimSuffix(mi.Req, "Request") + "Response"
            nullResp := ir.IsNullResponse(schema.Defs[respName])
            body = append(body, unmarshalValidate(f, mi.Req)...)
            methodName := strings.TrimSuffix(mi.Req, "Request")
            pre, recv := agentAssert(f, mi.Binding, methodName, mi.Req, respName, !nullResp)
            body = append(body, pre...)
            switch {
            case mi.Method == "session/prompt":
                body = append(body, promptCase(f, recv, methodName)...)
            case nullResp:
                body = append(body, callNoResult(recv, methodName)...)
            default:
                body = append(body, callWithResult(recv, methodName)...)
            }
        }
        if len(body) > 0 {
            cases = append(cases, caseOf(exprs(id("AgentMethod"+toExportedConst(k))), body...))
        }
    }
    f.add(handleFunc(f, "a", "AgentSideConnection", cases))

    // Agent outbound wrappers (agent -> client).
    emitOutbound(f, "AgentSideConnection", "client", "ClientMethod", groups, schema, meta.ClientMethods)

    return f.write(outDir, "agent_gen.go")
}

func writeClientGen(outDir string, groups ir.Groups, schema *load.Schema, meta *load.Meta) error {
    f := newFile()

    var cases []ast.Stmt
    for _, k := range ir.SortedKeys(meta.ClientMethods) {
        mi := groups["client|"+meta.ClientMethods[k]]
        if mi == nil {
            continue
        }
        var body []ast.Stmt
        if mi.Notif != "" {
            body = append(body, unmarshalValidate(f, mi.Notif)...)
            callName := ir.DispatchMethodNameForNotification(k, mi.Notif)
            pre, recv := clientAssert(f, mi.Binding, callName, mi.Notif, "", false)
            body = append(body, pre...)
            body = append(body, callNoResult(recv, callName)...)
        } else if mi.Req != "" {
            respName := strings.TrimSuffix(mi.Req, "Request") + "Response"
            nullResp := ir.IsNullResponse(schema.Defs[respName])
            body = append(body, unmarshalValidate(f, mi.Req)...)
            methodName := strings.TrimSuffix(mi.Req, "Request")
            pre, recv := clientAssert(f, mi.Binding, methodName, mi.Req, respName, !nullResp)
            body = append(body, pre...)
            if nullResp {
                body = append(body, callNoResult(recv, methodName)...)
            } else {
                body = append(body, callWithResult(recv, methodName)...)
            }
        }
        if len(body) > 0 {
            cases = append(cases, caseOf(exprs(id("ClientMethod"+toExportedConst(k))), body...))
        }
    }
    f.add(handleFunc(f, "c", "ClientSideConnection", cases))

    // Client outbound wrappers (client -> agent).
    emitOutbound(f, "ClientSideConnection", "agent", "AgentMethod", groups, schema, meta.AgentMethods)

    return f.write(outDir, "client_gen.go")
}

// emitOutbound emits one method per peer method on recvType that sends the
// request or notification over c.conn.
func emitOutbound(f *goFile, recvType, side, constPrefix string, groups ir.Groups, schema *load.Schema, methods map[string]string) {
    constByWire := map[string]string{}
    for k, wire := range methods {
        constByWire[wire] = constPrefix + toExportedConst(k)
    }
    recv := func() *ast.Field { return param("c", star(id(recvType))) }
    params := func(typeName string) []*ast.Field {
        return []*ast.Field{param("ctx", f.qual(pkgContext, "Context")), param("params", id(typeName))}
    }

    for _, k := range ir.SortedKeys(methods) {
        mi := groups[side+"|"+methods[k]]
        if mi == nil {
            continue
        }
        constName := constByWire[mi.Method]
        if constName == "" {
            continue
        }
        if mi.Notif != "" {
            name := strings.TrimSuffix(mi.Notif, "Notification")
            switch mi.Method {
            case "session/update":
                name = nameSessionUpdate
            case wireSessionCancel:
                name = "Cancel"
            }
            f.add(method(recv(), name, funcType(params(mi.Notif), param("", id("error"))),
                ret(call(dot("c", "conn", "SendNotification"), id("ctx"), id(constName), id("params"))),
            ))
            continue
        }
        if mi.Req == "" {
            continue
        }
        name := strings.TrimSuffix(mi.Req, "Request")
        respName := name + "Response"
        if ir.IsNullResponse(schema.Defs[respName]) {
            f.add(method(recv(), name, funcType(params(mi.Req), param("", id("error"))),
                ret(call(dot("c", "conn", "SendRequestNoResult"), id("ctx"), id(constName), id("params"))),
            ))
            continue
        }
        body := []ast.Stmt{
            define(exprs(id("resp"), id("err")),
                call(index(id("SendRequest"), id(respName)), dot("c", "conn"), id("ctx"), id(constName), id("params"))),
        }
        // session/prompt from the client: if ctx was canceled, send session/cancel best-effort.
        if side == "agent" && mi.Method == "session/prompt" {
            body = append(body, ifStmt(nil, errNotNil(),
                ifStmt(nil, binary(call(dot("ctx", "Err")), token.NEQ, nilExpr()),
                    assign(exprs(id("_")), call(dot("c", "Cancel"),
                        call(f.qual(pkgContext, "Background")),
                        composite(id("CancelNotification"), kv(id("SessionId"), dot("params", "SessionId"))),
                    )),
                ),
            ))
        }
        body = append(body, ret(id("resp"), id("err")))
        f.add(method(recv(), name, funcType(params(mi.Req), param("", id(respName)), param("", id("error"))), body...))
    }
}
```

### `cmd/generate/internal/emit/types.go`

```go
package emit

import (
    "encoding/json"
    "fmt"
    "go/ast"
    "go/token"
    "maps"
    "slices"
    "strings"
    "unicode"

    "github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/ir"
    "github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/load"
    "github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/util"
)

// splitCamelCase splits a CamelCase string into words.
// Example: "RequestPermissionRequest" -> ["Request", "Permission", "Request"]
func splitCamelCase(s string) []string {
    if s == "" {
        return nil
    }
    var words []string
    var current strings.Builder

    for i, r := range s {
        if i > 0 && unicode.IsUpper(r) {
            // Start new word on uppercase letter
            if current.Len() > 0 {
                words = append(words, current.String())
                current.Reset()
            }
        }
        current.WriteRune(r)
    }
    if current.Len() > 0 {
        words = append(words, current.String())
    }
    return words
}

// generateNestedTypeName applies multiple heuristics to create idiomatic nested type names
// while guaranteeing stability (name depends only on parent, field, and collision check).
//
// If a collision is detected (e.g. the schema already defines a type with the same name we
// would otherwise generate), fall back to a deterministic suffixed name. Panics only if it
// cannot find a unique name (which would indicate a codegen bug).
func generateNestedTypeName(parentName, rawFieldName string, usedNames map[string]bool) string {
    // Heuristic 1: Ensure field name is properly capitalized first
    fieldName := util.ToExportedField(rawFieldName)

    // Heuristic 2: Try RPC suffix stripping for Request/Response/Notification parents
    // This handles cases like RequestPermissionRequest + ToolCall → RequestPermissionToolCall
    for _, suffix := range []string{"Request", "Response", "Notification"} {
        if baseName, ok := strings.CutSuffix(parentName, suffix); ok && baseName != "" {
            candidate := baseName + fieldName
            if !usedNames[candidate] {
                return candidate
            }
        }
    }

    // Heuristic 3: Try word-boundary deduplication
    // This handles cases like AgentNotification + ExtNotification → AgentExtNotification
    parentWords := splitCamelCase(parentName)
    fieldWords := splitCamelCase(fieldName)

    // Need at least 2 words in parent to consider deduplication
    if len(parentWords) >= 2 && len(fieldWords) > 0 {
        lastParentWord := parentWords[len(parentWords)-1]
        firstFieldWord := fieldWords[0]
        lastFieldWord := fieldWords[len(fieldWords)-1]

        if strings.EqualFold(lastParentWord, firstFieldWord) || strings.EqualFold(lastParentWord, lastFieldWord) {
            // Remove last word from parent, then concatenate
            parentWithoutLast := strings.Join(parentWords[:len(parentWords)-1], "")
            if parentWithoutLast != "" {
                candidate := parentWithoutLast + fieldName
                if !usedNames[candidate] {
                    return candidate
                }
            }
        }
    }

    // Heuristic 4: Fall back to full concatenation
    fullName := parentName + fieldName
    if !usedNames[fullName] {
        return fullName
    }

    // Collision: this can happen when the schema already defines a top-level type with the
    // same name as an inline property we'd otherwise generate.
    base := fullName + "Inline"
    if !usedNames[base] {
        return base
    }
    for i := 2; i < 1000; i++ {
        candidate := fmt.Sprintf("%s%d", base, i)
        if !usedNames[candidate] {
            return candidate
        }
    }

    // DEFENSIVE PROGRAMMING: If we get here, something is deeply wrong with name tracking.
    panic(fmt.Sprintf("unable to generate unique nested type name (parent: %q, field: %q)", parentName, rawFieldName))
}

// fieldSet collects struct fields or interface methods, each with the doc
// comment lines added before it.
type fieldSet struct {
    list []*ast.Field
    doc  []string
}

// comment adds the lines of desc as doc for the next field.
func (s *fieldSet) comment(lines ...string) { s.doc = append(s.doc, lines...) }

func (s *fieldSet) add(fld *ast.Field) {
    fld.Doc = docLines(s.doc...)
    s.doc = nil
    s.list = append(s.list, fld)
}

func (s *fieldSet) fieldList() *ast.FieldList {
    if len(s.doc) > 0 {
        panic("emit: doc comment with no field after it")
    }
    return fields(s.list...)
}

func (s *fieldSet) structType() *ast.StructType { return &ast.StructType{Fields: s.fieldList()} }

func (s *fieldSet) interfaceType() *ast.InterfaceType {
    return &ast.InterfaceType{Methods: s.fieldList()}
}

// structField is name typ `json:"tag"`.
func structField(name string, typ ast.Expr, tag string) *ast.Field {
    return &ast.Field{Names: []*ast.Ident{id(name)}, Type: typ, Tag: structTag(tag)}
}

// WriteTypes emits types_gen.go with all types and the Agent/Client interfaces.
func WriteTypes(outDir string, schema *load.Schema, meta *load.Meta) error {
    f := newFile()

    // Track all type names to avoid collisions when generating nested types
    usedTypeNames := make(map[string]bool, len(schema.Defs))
    for k := range schema.Defs {
        usedTypeNames[k] = true
    }
    for _, name := range slices.Sorted(maps.Keys(schema.Defs)) {
        emitSchemaDef(f, name, schema.Defs[name], schema, usedTypeNames)
    }
    emitRPCInterfaces(f, schema, meta)

    return f.write(outDir, "types_gen.go")
}

// interfaceMethod is name(ctx context.Context, params paramType) error, or
// (respType, error) when respType is set.
func interfaceMethod(f *goFile, name, paramType, respType string) *ast.Field {
    res := []*ast.Field{param("", id("error"))}
    if respType != "" {
        res = []*ast.Field{param("", id(respType)), param("", id("error"))}
    }
    return &ast.Field{
        Names: []*ast.Ident{id(name)},
        Type:  funcType([]*ast.Field{param("ctx", f.qual(pkgContext, "Context")), param("params", id(paramType))}, res...),
    }
}

// addRPCMethod adds the interface method for mi to target.
func addRPCMethod(f *goFile, target *fieldSet, schema *load.Schema, methodKey string, mi *ir.MethodInfo) {
    if desc := methodDescription(schema, mi); desc != "" {
        target.comment(util.FormatDocComment(desc)...)
    }
    switch {
    case mi.Notif != "":
        target.add(interfaceMethod(f, ir.DispatchMethodNameForNotification(methodKey, mi.Notif), mi.Notif, ""))
    case mi.Req != "":
        methodName := strings.TrimSuffix(mi.Req, "Request")
        respName := methodName + "Response"
        if ir.IsNullResponse(schema.Defs[respName]) {
            respName = ""
        }
        target.add(interfaceMethod(f, methodName, mi.Req, respName))
    }
}

func emitRPCInterfaces(f *goFile, schema *load.Schema, meta *load.Meta) {
    // Append Agent & Client interfaces from method groups
    groups := ir.BuildMethodGroups(schema, meta)

    var agent, agentLoader, agentExperimental fieldSet
    for _, k := range ir.SortedKeys(meta.AgentMethods) {
        mi := groups["agent|"+meta.AgentMethods[k]]
        if mi == nil {
            continue
        }
        target := &agent
        switch mi.Binding {
        case ir.BindAgentLoader:
            target = &agentLoader
        case ir.BindAgentExperimental:
            target = &agentExperimental
        }
        addRPCMethod(f, target, schema, k, mi)
    }
    f.add(typeDecl("Agent", agent.interfaceType()))
    if len(agentLoader.list) > 0 {
        f.comment("AgentLoader defines optional support for loading sessions. Implement and advertise the capability to enable 'session/load'.")
        f.add(typeDecl("AgentLoader", agentLoader.interfaceType()))
    }
    // Always emit AgentExperimental, even if empty.
    f.comment("AgentExperimental defines unstable methods that are not part of the official spec. These may change or be removed without notice.")
    f.add(typeDecl("AgentExperimental", agentExperimental.interfaceType()))

    var client, clientExperimental, clientTerminal fieldSet
    for _, k := range ir.SortedKeys(meta.ClientMethods) {
        mi := groups["client|"+meta.ClientMethods[k]]
        if mi == nil {
            continue
        }
        target := &client
        switch mi.Binding {
        case ir.BindClientExperimental:
            target = &clientExperimental
        case ir.BindClientTerminal:
            target = &clientTerminal
        }
        addRPCMethod(f, target, schema, k, mi)
    }
    f.add(typeDecl("Client", client.interfaceType()))
    if len(clientTerminal.list) > 0 {
        f.comment("ClientTerminal defines terminal-related experimental methods (x-docs-ignore). Implement and advertise 'terminal: true' to enable 'terminal/*'.")
        f.add(typeDecl("ClientTerminal", clientTerminal.interfaceType()))
    }
    // Always emit ClientExperimental, even if empty.
    f.comment("ClientExperimental defines unstable methods that are not part of the official spec. These may change or be removed without notice.")
    f.add(typeDecl("ClientExperimental", clientExperimental.interfaceType()))
}

// enumConsts declares <name><Value> name = "value" for each value.
func enumConsts(f *goFile, name string, values []string) {
    if len(values) == 0 {
        return
    }
    var specs []*ast.ValueSpec
    for _, s := range values {
        specs = append(specs, constSpec(util.ToEnumConst(name, s), id(name), str(s)))
    }
    f.add(constDecl(true, specs...))
}

func emitSchemaDef(f *goFile, name string, def *load.Definition, schema *load.Schema, usedTypeNames map[string]bool) {
    if def == nil {
        return
    }

    // DEFENSIVE PROGRAMMING: Ensure we're not about to emit a type that conflicts
    // with a previously generated nested type. This should never happen since we
    // pre-populate usedTypeNames with all schema definitions.
    if usedTypeNames[name] && name != "" {
        if _, inSchema := schema.Defs[name]; !inSchema {
            panic(fmt.Sprintf("BUG: attempting to emit type %q that was already generated as a nested type", name))
        }
    }

    if def.Description != "" {
        f.comment(util.FormatDocComment(def.Description)...)
    }

    switch {
    case len(def.Enum) > 0:
        f.add(typeDecl(name, id("string")))
        var values []string
        for _, v := range def.Enum {
            values = append(values, fmt.Sprint(v))
        }
        enumConsts(f, name, values)
        f.line()
    case isStringConstUnion(def):
        f.add(typeDecl(name, id("string")))
        var values []string
        for _, v := range def.OneOf {
            if v != nil && v.Const != nil {
                values = append(values, fmt.Sprint(v.Const))
            }
        }
        enumConsts(f, name, values)
        f.line()
    case len(def.AnyOf) > 0 && isOpenStringEnum(def):
        // "Open" string enum: `anyOf` of string consts plus (typically) a
        // free-form string catch-all. Emit as a named string type with
        // constants for the known values. Unknown values are still
        // representable as strings, matching the schema's extensibility.
        f.add(typeDecl(name, id("string")))
        var values []string
        for _, v := range def.AnyOf {
            if v == nil || v.Const == nil {
                continue
            }
            if s, ok := v.Const.(string); ok {
                values = append(values, s)
            }
        }
        enumConsts(f, name, values)
        f.line()
    case len(def.AnyOf) > 0:
        emitUnion(f, name, schema, def, def.AnyOf, false, usedTypeNames)
    case len(def.OneOf) > 0 && !isStringConstUnion(def):
        // Generic union generation for non-enum oneOf
        // Use the same implementation, but require exactly one variant
        emitUnion(f, name, schema, def, def.OneOf, true, usedTypeNames)
    case ir.PrimaryType(def) == schemaTypeObject && len(def.Properties) > 0:
        emitObjectSchema(f, name, def, usedTypeNames)
    case slices.Contains([]string{schemaTypeString, schemaTypeInteger, schemaTypeNumber, schemaTypeBoolean}, ir.PrimaryType(def)):
        f.add(typeDecl(name, primitiveType(ir.PrimaryType(def))))
        f.line()
    case ir.PrimaryType(def) == schemaTypeObject && len(def.Properties) == 0:
        // Empty object shape: emit a concrete empty struct so methods can be defined
        // and the wire encoding is consistently {} rather than null.
        f.add(typeDecl(name, &ast.StructType{Fields: fields()}))
        f.line()
    default:
        f.comment(fmt.Sprintf("%s is a union or complex schema; represented generically.", name))
        f.add(typeDecl(name, id("any")))
        f.line()
    }

    // Validators for selected types. Union wrappers get Validate from
    // emitUnion; emitValidate reads properties on the wrapper, which is the
    // wrong shape for those types.
    emittedUnion := (len(def.OneOf) > 0 && !isStringConstUnion(def)) || (len(def.AnyOf) > 0 && !isOpenStringEnum(def))
    if !emittedUnion && (isRPCTypeName(name) || name == "ToolCallUpdate") {
        emitValidate(f, name, def)
    }
}

func isRPCTypeName(name string) bool {
    return strings.HasSuffix(name, "Request") || strings.HasSuffix(name, "Response") || strings.HasSuffix(name, "Notification")
}

// requiredSet returns the names in required as a set.
func requiredSet(required []string) map[string]bool {
    set := make(map[string]bool, len(required))
    for _, r := range required {
        set[r] = true
    }
    return set
}

func emitObjectSchema(f *goFile, name string, def *load.Definition, usedTypeNames map[string]bool) {
    req := requiredSet(def.Required)
    pkeys := slices.Sorted(maps.Keys(def.Properties))

    // Pre-generate nested struct types for inline object properties
    nestedTypes := emitInlineNestedTypes(f, name, pkeys, def, usedTypeNames)

    // Track fields with schema defaults for generic (de)serialization
    var defaults []defaultProp
    var st fieldSet
    for _, pk := range pkeys {
        prop := def.Properties[pk]
        field := util.ToExportedField(pk)
        if prop.Description != "" {
            st.comment(util.FormatDocComment(prop.Description)...)
        }
        tag := pk
        var dp *defaultProp
        if prop.Default != nil {
            // Whether field zero is nil (slice/map) for Marshal fill-in
            nilable := ir.PrimaryType(prop) == schemaTypeArray || (ir.PrimaryType(prop) == schemaTypeObject && len(prop.Properties) == 0 && prop.Ref == "")
            // Capture canonical JSON of default
            defJSON := schemaTypeNull
            if b, err := json.Marshal(prop.Default); err == nil {
                defJSON = string(b)
            }
            dp = &defaultProp{
                fieldName:   field,
                propName:    pk,
                defaultJSON: defJSON,
                kind:        defaultKindOf(prop.Default),
                allowNull:   includesNull(prop),
                nilable:     nilable,
            }
            defaults = append(defaults, *dp)
        }
        if !req[pk] {
            // Default: omit if empty for optional fields.
            // Keep always-present behavior only for defaults where the zero value is nil (slice/map).
            // For typed object defaults (non-nilable), still allow omission on the wire.
            if dp == nil || (dp.kind != kindArray && dp.kind != kindObject) || !dp.nilable {
                tag = pk + ",omitempty"
            }
        }
        // Emit an additional comment line indicating the default, if any.
        if dp != nil && dp.defaultJSON != schemaTypeNull {
            // An empty comment line separates the default from the description.
            if prop.Description != "" {
                st.comment("")
            }
            st.comment(fmt.Sprintf("Defaults to %s if unset.", dp.defaultJSON))
        }
        var fieldType ast.Expr
        if nestedTypeName, ok := nestedTypes[pk]; ok {
            fieldType = id(nestedTypeName)
        } else {
            fieldType = optionalTypeExpr(prop)
        }
        st.add(structField(field, fieldType, tag))
    }
    f.add(typeDecl(name, st.structType()))
    f.line()

    emitObjectDefaults(f, name, defaults)
}

func emitInlineNestedTypes(f *goFile, name string, pkeys []string, def *load.Definition, usedTypeNames map[string]bool) map[string]string {
    nestedTypes := map[string]string{}
    for _, pk := range pkeys {
        prop := def.Properties[pk]
        // Detect inline objects: no $ref, type is object, has properties
        if prop.Ref != "" || ir.PrimaryType(prop) != schemaTypeObject || len(prop.Properties) == 0 {
            continue
        }
        nestedTypeName := generateNestedTypeName(name, pk, usedTypeNames)
        nestedTypes[pk] = nestedTypeName

        // DEFENSIVE PROGRAMMING: Assert no collision before registering
        if usedTypeNames[nestedTypeName] {
            panic(fmt.Sprintf("BUG: nested type name collision: %q already registered", nestedTypeName))
        }
        usedTypeNames[nestedTypeName] = true

        if prop.Description != "" {
            f.comment(util.FormatDocComment(prop.Description)...)
        }
        nestedReq := requiredSet(prop.Required)
        var st fieldSet
        for _, npk := range slices.Sorted(maps.Keys(prop.Properties)) {
            nprop := prop.Properties[npk]
            if nprop.Description != "" {
                st.comment(util.FormatDocComment(nprop.Description)...)
            }
            ntag := npk
            if !nestedReq[npk] {
                ntag = npk + ",omitempty"
            }
            st.add(structField(util.ToExportedField(npk), optionalTypeExpr(nprop), ntag))
        }
        f.add(typeDecl(nestedTypeName, st.structType()))
        f.line()
    }
    return nestedTypes
}

type defaultKind int

const (
    kindNone defaultKind = iota
    kindScalar
    kindArray
    kindObject
)

type defaultProp struct {
    fieldName   string
    propName    string
    defaultJSON string
    kind        defaultKind
    allowNull   bool
    nilable     bool // whether zero-value is nil (slice/map)
}

// emitObjectDefaults synthesizes MarshalJSON and UnmarshalJSON for structs
// with fields that carry schema defaults.
func emitObjectDefaults(f *goFile, name string, defaults []defaultProp) {
    if len(defaults) == 0 {
        return
    }
    unmarshal := func(args ...ast.Expr) *ast.CallExpr { return call(f.qual(pkgJSON, "Unmarshal"), args...) }

    // MarshalJSON: coerce nil slices and maps to their defaults before encoding.
    body := []ast.Stmt{
        typeStmt("Alias", id(name)),
        varStmt("a", id("Alias"), nil),
        assign(exprs(id("a")), call(id("Alias"), id("v"))),
    }
    for _, dp := range defaults {
        // Typed object defaults (non-nilable) are not injected on encode.
        if (dp.kind == kindArray || dp.kind == kindObject) && dp.nilable {
            body = append(body, ifStmt(nil, binary(dot("a", dp.fieldName), token.EQL, nilExpr()),
                exprStmt(unmarshal(bytesOf(dp.defaultJSON), addr(dot("a", dp.fieldName)))),
            ))
        }
    }
    body = append(body, ret(call(f.qual(pkgJSON, "Marshal"), id("a"))))
    f.add(method(param("v", id(name)), "MarshalJSON",
        funcType(nil, param("", byteSlice()), param("", id("error"))), body...))
    f.line()

    // UnmarshalJSON: apply defaults when a field is missing, or null when
    // the schema does not allow null.
    body = []ast.Stmt{
        varStmt("m", rawMap(f.qual(pkgJSON, "RawMessage")), nil),
        ifStmt(define(exprs(id("err")), unmarshal(id("b"), addr(id("m")))), errNotNil(), ret(id("err"))),
        typeStmt("Alias", id(name)),
        varStmt("a", id("Alias"), nil),
        ifStmt(define(exprs(id("err")), unmarshal(id("b"), addr(id("a")))), errNotNil(), ret(id("err"))),
    }
    for _, dp := range defaults {
        var cond ast.Expr = not(id("_ok"))
        if !dp.allowNull {
            cond = binary(not(id("_ok")), token.LOR, paren(binary(call(id("string"), id("_rm")), token.EQL, str(schemaTypeNull))))
        }
        body = append(body, block(
            define(exprs(id("_rm"), id("_ok")), index(id("m"), str(dp.propName))),
            ifStmt(nil, cond, exprStmt(unmarshal(bytesOf(dp.defaultJSON), addr(dot("a", dp.fieldName))))),
        ))
    }
    body = append(body,
        assign(exprs(star(id("v"))), call(id(name), id("a"))),
        ret(nilExpr()),
    )
    f.add(method(param("v", star(id(name))), "UnmarshalJSON",
        funcType([]*ast.Field{param("b", byteSlice())}, param("", id("error"))), body...))
    f.line()
}

// emitInlineUnionStruct emits the type for an inline (non-$ref) union variant.
func emitInlineUnionStruct(f *goFile, name, tname, ref string, v *load.Definition, isObj, isNull bool, sharedProps map[string]*load.Definition, sharedRequired map[string]bool, usedTypeNames map[string]bool) {
    if ref != "" || (!isObj && !isNull && v.Title == "") {
        return
    }
    // DEFENSIVE PROGRAMMING: Verify tname is registered before emitting
    if !usedTypeNames[tname] {
        panic(fmt.Sprintf("BUG: attempting to emit unregistered type: %q (parent union: %q)", tname, name))
    }

    var st fieldSet
    switch {
    case !isNull && isObj:
        req := requiredSet(v.Required)
        maps.Copy(req, sharedRequired)
        mergedProps := make(map[string]*load.Definition, len(sharedProps)+len(v.Properties))
        maps.Copy(mergedProps, sharedProps)
        maps.Copy(mergedProps, v.Properties)
        if v.Description != "" {
            f.comment(util.FormatDocComment(v.Description)...)
        }
        for _, pk := range slices.Sorted(maps.Keys(mergedProps)) {
            pDef := mergedProps[pk]
            if pDef.Description != "" {
                st.comment(util.FormatDocComment(pDef.Description)...)
            }
            tag := pk
            if !req[pk] {
                tag = pk + ",omitempty"
            }
            st.add(structField(util.ToExportedField(pk), optionalTypeExpr(pDef), tag))
        }
    case !isNull && !isObj && v.Title != "":
        // Title-only variants. Extension types keep the raw payload instead
        // of dropping it.
        isExtension := strings.Contains(strings.ToLower(v.Description), "extension") || strings.Contains(v.Title, "Ext")
        if v.Description != "" {
            f.comment(util.FormatDocComment(v.Description)...)
        }
        switch {
        case isExtension:
            // An alias keeps json.RawMessage's methods, unlike a defined type.
            f.add(aliasDecl(tname, f.qual(pkgJSON, "RawMessage")))
        case ir.PrimaryType(v) != "":
            // Non-object title-only variants can still carry payloads (e.g. arrays).
            // Emit a concrete type so sequential decode can succeed for valid payloads.
            f.add(typeDecl(tname, typeExpr(v)))
        default:
            // Emit as empty struct only for truly empty variants.
            f.add(typeDecl(tname, st.structType()))
        }
        f.line()
        return
    }
    f.add(typeDecl(tname, st.structType()))
    f.line()
}

// invalidPayload returns errors.New("invalid variant payload") with the given leading results.
func invalidPayload(f *goFile, lead ...ast.Expr) ast.Stmt {
    return ret(append(lead, call(f.qual(pkgErrors, "New"), str("invalid variant payload")))...)
}

// setVariant is u.<field> = &v; return nil.
func setVariant(vi variantInfo) []ast.Stmt {
    return []ast.Stmt{
        assign(exprs(dot("u", vi.fieldName)), addr(id("v"))),
        ret(nilExpr()),
    }
}

// decodeStrict decodes b into v, fails on error, and sets the variant.
func decodeStrict(f *goFile, vi variantInfo) []ast.Stmt {
    return append([]ast.Stmt{
        ifStmt(nil, binary(call(f.qual(pkgJSON, "Unmarshal"), id("b"), addr(id("v"))), token.NEQ, nilExpr()),
            invalidPayload(f)),
    }, setVariant(vi)...)
}

// keyMatch sets match = false when key is missing from m.
func keyMatch(m, key string) ast.Stmt {
    return ifStmt(define(exprs(id("_"), id("ok")), index(id(m), str(key))), not(id("ok")),
        assign(exprs(id("match")), boolLit(false)))
}

func unionUnmarshalBody(f *goFile, variants []variantInfo, discKey string) []ast.Stmt {
    unmarshal := func(args ...ast.Expr) *ast.CallExpr { return call(f.qual(pkgJSON, "Unmarshal"), args...) }
    var body []ast.Stmt

    // Handle literal null with the first null-only variant.
    for _, vi := range variants {
        if vi.isNull {
            body = append(body, ifStmt(nil, binary(call(id("string"), id("b")), token.EQL, str(schemaTypeNull)),
                append([]ast.Stmt{varStmt("v", id(vi.typeName), nil)}, setVariant(vi)...)...))
            break
        }
    }

    var object []ast.Stmt
    // Prefer discriminator-based dispatch when available (e.g. "type", "outcome").
    if discKey != "" {
        var cases []ast.Stmt
        for _, vi := range variants {
            if vi.discValue != "" {
                cases = append(cases, caseOf(exprs(str(vi.discValue)),
                    append([]ast.Stmt{varStmt("v", id(vi.typeName), nil)}, decodeStrict(f, vi)...)...))
            }
        }
        object = append(object, block(
            varStmt("disc", id("string"), nil),
            ifStmt(define(exprs(id("v"), id("ok")), index(id("m"), str(discKey))), id("ok"),
                exprStmt(unmarshal(id("v"), addr(id("disc"))))),
            switchStmt(id("disc"), cases...),
        ))
    }
    // Required-key match.
    for _, vi := range variants {
        if !vi.isObject || len(vi.required) == 0 {
            continue
        }
        stmts := []ast.Stmt{
            varStmt("v", id(vi.typeName), nil),
            varStmt("match", id("bool"), boolLit(true)),
        }
        for _, rk := range vi.required {
            stmts = append(stmts, keyMatch("m", rk))
        }
        stmts = append(stmts, ifStmt(nil, id("match"), decodeStrict(f, vi)...))
        object = append(object, block(stmts...))
    }
    body = append(body,
        varStmt("m", rawMap(f.qual(pkgJSON, "RawMessage")), nil),
        ifElse(define(exprs(id("err")), unmarshal(id("b"), addr(id("m")))), binary(id("err"), token.EQL, nilExpr()),
            object,
            // Not an object (e.g., primitive union variant) or invalid JSON.
            []ast.Stmt{ifStmt(define(exprs(id("_"), id("ok")), typeAssert(id("err"), star(f.qual(pkgJSON, "UnmarshalTypeError")))), not(id("ok")),
                ret(id("err")))},
        ),
    )

    // For array variants with required keys on object items, try key-based matching first.
    var array []ast.Stmt
    for _, vi := range variants {
        if !vi.isArray || len(vi.arrayItemRequired) == 0 {
            continue
        }
        var loop []ast.Stmt
        for _, rk := range vi.arrayItemRequired {
            loop = append(loop, keyMatch("elem", rk))
        }
        array = append(array, block(
            varStmt("v", id(vi.typeName), nil),
            varStmt("match", id("bool"), boolLit(true)),
            rangeStmt(id("_"), id("elem"), id("arr"), loop...),
            ifStmt(nil, id("match"), decodeStrict(f, vi)...),
        ))
    }
    body = append(body,
        varStmt("arr", sliceOf(rawMap(f.qual(pkgJSON, "RawMessage"))), nil),
        ifStmt(nil, binary(unmarshal(id("b"), addr(id("arr"))), token.EQL, nilExpr()), array...),
    )

    // Fallback: try each variant in turn.
    for _, vi := range variants {
        body = append(body, block(
            varStmt("v", id(vi.typeName), nil),
            ifStmt(nil, binary(unmarshal(id("b"), addr(id("v"))), token.EQL, nilExpr()), setVariant(vi)...),
        ))
    }
    return append(body, ret(call(f.qual(pkgErrors, "New"), str("no matching variant for union"))))
}

const keyMimeType = "mimeType"

// contentBlockShape lists the keys a ContentBlock variant keeps on the wire:
// required keys always, optional keys only when present.
var contentBlockShape = map[string]struct{ required, optional []string }{
    "text":          {required: []string{"text"}},
    "image":         {required: []string{"data", keyMimeType}, optional: []string{"uri"}},
    "audio":         {required: []string{"data", keyMimeType}},
    "resource_link": {required: []string{"name", "uri"}, optional: []string{"description", keyMimeType, "size", "title"}},
    "resource":      {required: []string{"resource"}},
}

// contentBlockMarshal rebuilds the variant map with only the keys of its
// wire shape, to preserve exact ContentBlock JSON.
func contentBlockMarshal(f *goFile, disc string) ast.Stmt {
    shape, ok := contentBlockShape[disc]
    if !ok {
        return nil
    }
    nm := func(key string) ast.Expr { return index(id("nm"), str(key)) }
    stmts := []ast.Stmt{
        varStmt("nm", rawMap(id("any")), nil),
        assign(exprs(id("nm")), call(id("make"), rawMap(id("any")))),
        assign(exprs(nm("type")), str(disc)),
    }
    for _, k := range shape.required {
        stmts = append(stmts, assign(exprs(nm(k)), index(id("m"), str(k))))
    }
    for i, k := range shape.optional {
        // The single optional key of image uses _v/_ok; resource_link numbers them.
        v, okName := "_v", "_ok"
        if disc == "resource_link" {
            v, okName = fmt.Sprintf("v%d", i+1), fmt.Sprintf("ok%d", i+1)
        }
        stmts = append(stmts, ifStmt(define(exprs(id(v), id(okName)), index(id("m"), str(k))), id(okName),
            assign(exprs(nm(k)), id(v))))
    }
    stmts = append(stmts, ret(call(f.qual(pkgJSON, "Marshal"), id("nm"))))
    return block(stmts...)
}

func unionMarshalBody(f *goFile, name string, variants []variantInfo) []ast.Stmt {
    marshal := func(x ast.Expr) *ast.CallExpr { return call(f.qual(pkgJSON, "Marshal"), x) }
    var body []ast.Stmt
    for _, vi := range variants {
        var then []ast.Stmt
        switch {
        case vi.isNull:
            // Null-only variant encodes to JSON null.
            then = []ast.Stmt{ret(marshal(nilExpr()))}
        case !vi.isObject:
            // Non-object variants (e.g., arrays/primitives) are already in final wire shape.
            then = []ast.Stmt{
                define(exprs(id("_b"), id("_e")), marshal(star(dot("u", vi.fieldName)))),
                ifStmt(nil, binary(id("_e"), token.NEQ, nilExpr()), ret(emptyBytes(), id("_e"))),
                ret(id("_b"), nilExpr()),
            }
        default:
            then = []ast.Stmt{
                define(exprs(id("_b"), id("_e")), marshal(star(dot("u", vi.fieldName)))),
                ifStmt(nil, binary(id("_e"), token.NEQ, nilExpr()), ret(emptyBytes(), id("_e"))),
                // Marshal object variant to map for discriminant injection and shaping.
                varStmt("m", rawMap(id("any")), nil),
                ifStmt(nil, binary(call(f.qual(pkgJSON, "Unmarshal"), id("_b"), addr(id("m"))), token.NEQ, nilExpr()),
                    invalidPayload(f, emptyBytes())),
            }
            // Inject const discriminants.
            for _, kv := range vi.constPairs {
                then = append(then, assign(exprs(index(id("m"), str(kv[0]))), str(kv[1])))
            }
            if name == unionContentBlock {
                // Special shaping for ContentBlock variants to preserve exact wire JSON.
                if s := contentBlockMarshal(f, vi.discValue); s != nil {
                    then = append(then, s)
                }
            } else {
                // Default: remarshal, possibly with an injected discriminant.
                then = append(then, ret(marshal(id("m"))))
            }
        }
        body = append(body, ifStmt(nil, binary(dot("u", vi.fieldName), token.NEQ, nilExpr()), then...))
    }
    return append(body, ret(emptyBytes(), nilExpr()))
}

type sharedFieldCheck struct {
    field string
    prop  string
    kind  string
}

func sharedFieldChecks(def *load.Definition) []sharedFieldCheck {
    if def == nil {
        return nil
    }
    var out []sharedFieldCheck
    for _, propName := range slices.Sorted(slices.Values(def.Required)) {
        pDef := def.Properties[propName]
        if pDef == nil {
            continue
        }
        switch kind := ir.PrimaryType(pDef); kind {
        case schemaTypeString, schemaTypeArray:
            out = append(out, sharedFieldCheck{field: util.ToExportedField(propName), prop: propName, kind: kind})
        }
    }
    return out
}

// requiredCheck returns fmt.Errorf("<prop> is required") when x is the zero
// value for a required string or array property.
func requiredCheck(f *goFile, x ast.Expr, kind, prop string) ast.Stmt {
    var zero ast.Expr = nilExpr()
    if kind == schemaTypeString {
        zero = str("")
    }
    return ifStmt(nil, binary(x, token.EQL, zero), ret(call(f.qual(pkgFmt, "Errorf"), str(prop+" is required"))))
}

func sharedUnionValidateBody(f *goFile, variants []variantInfo, checks []sharedFieldCheck) []ast.Stmt {
    carriers := 0
    for _, vi := range variants {
        if vi.hasSharedFields {
            carriers++
        }
    }
    if len(checks) == 0 || carriers == 0 {
        return []ast.Stmt{ret(nilExpr())}
    }
    body := []ast.Stmt{varStmt("set", id("bool"), nil)}
    for _, vi := range variants {
        if !vi.hasSharedFields {
            continue
        }
        var then []ast.Stmt
        for _, c := range checks {
            then = append(then, requiredCheck(f, dot("v", vi.fieldName, c.field), c.kind, c.prop))
        }
        then = append(then, assign(exprs(id("set")), boolLit(true)))
        body = append(body, ifStmt(nil, binary(dot("v", vi.fieldName), token.NEQ, nilExpr()), then...))
    }
    return append(body,
        ifStmt(nil, not(id("set")), ret(call(f.qual(pkgFmt, "Errorf"), str(checks[0].prop+" is required")))),
        ret(nilExpr()),
    )
}

func unionValidateBody(f *goFile, name string, variants []variantInfo) []ast.Stmt {
    body := []ast.Stmt{varStmt("count", id("int"), nil)}
    for _, vi := range variants {
        body = append(body, ifStmt(nil, binary(dot("u", vi.fieldName), token.NEQ, nilExpr()), incr(id("count"))))
    }
    return append(body,
        ifStmt(nil, binary(id("count"), token.NEQ, intLit(1)),
            ret(call(f.qual(pkgErrors, "New"), str(name+" must have exactly one variant set")))),
        ret(nilExpr()),
    )
}

func isStringConstUnion(def *load.Definition) bool {
    if def == nil || len(def.OneOf) == 0 {
        return false
    }
    for _, v := range def.OneOf {
        if v == nil || v.Const == nil {
            return false
        }
        if _, ok := v.Const.(string); !ok {
            return false
        }
    }
    return true
}

// isOpenStringEnum reports whether def is an "open" string enum: an `anyOf`
// where every variant is a plain string schema (type: "string") and at least
// one variant has a string `const`. Such schemas describe a string value with
// a recommended set of known values while still permitting arbitrary strings
// for extensibility (see e.g. SessionConfigOptionCategory).
//
// These are emitted as a Go string-derived type with named constants for each
// known value, rather than as a discriminated union wrapper.
func isOpenStringEnum(def *load.Definition) bool {
    if def == nil || len(def.AnyOf) == 0 {
        return false
    }
    sawConst := false
    for _, v := range def.AnyOf {
        if v == nil {
            return false
        }
        // Each variant must be a plain string schema: type "string", no nested
        // structure (properties, items, refs, nested unions, allOf, etc.).
        if ir.PrimaryType(v) != schemaTypeString {
            return false
        }
        if v.Ref != "" || len(v.Properties) > 0 || v.Items != nil ||
            len(v.AnyOf) > 0 || len(v.OneOf) > 0 || len(v.AllOf) > 0 ||
            len(v.Enum) > 0 {
            return false
        }
        if v.Const != nil {
            if _, ok := v.Const.(string); !ok {
                return false
            }
            sawConst = true
        }
    }
    return sawConst
}

// validateMethod is func (recv *name) Validate() error { body }.
func validateMethod(recv, name string, body ...ast.Stmt) *ast.FuncDecl {
    return method(param(recv, star(id(name))), "Validate", funcType(nil, param("", id("error"))), body...)
}

// emitValidate generates Validate for RPC object types and ToolCallUpdate.
func emitValidate(f *goFile, name string, def *load.Definition) {
    if name == "ToolCallUpdate" {
        f.add(validateMethod("t", name,
            requiredCheck(f, dot("t", "ToolCallId"), schemaTypeString, "toolCallId"),
            ret(nilExpr()),
        ))
        return
    }
    if def == nil || ir.PrimaryType(def) != schemaTypeObject || !isRPCTypeName(name) {
        return
    }
    var body []ast.Stmt
    for _, propName := range slices.Sorted(maps.Keys(def.Properties)) {
        if !slices.Contains(def.Required, propName) {
            continue
        }
        switch kind := ir.PrimaryType(def.Properties[propName]); kind {
        case schemaTypeString, schemaTypeArray:
            body = append(body, requiredCheck(f, dot("v", util.ToExportedField(propName)), kind, propName))
        }
    }
    f.add(validateMethod("v", name, append(body, ret(nilExpr()))...))
}

func methodDescription(schema *load.Schema, mi *ir.MethodInfo) string {
    if mi == nil || mi.DocsIgnored {
        return ""
    }
    // Prefer notification descriptions when present, otherwise fall back to request/response types.
    for _, name := range []string{mi.Notif, mi.Req, mi.Resp} {
        if name == "" {
            continue
        }
        if def := schema.Defs[name]; def != nil && def.Description != "" {
            return def.Description
        }
    }
    return ""
}

// primitiveType maps a JSON Schema primitive type to its Go type.
func primitiveType(t string) ast.Expr {
    switch t {
    case schemaTypeString:
        return id("string")
    case schemaTypeInteger:
        return id("int")
    case schemaTypeNumber:
        return id("float64")
    case schemaTypeBoolean:
        return id("bool")
    default:
        return id("any")
    }
}

// defaultKindOf classifies the JSON Schema default value into a coarse kind.
func defaultKindOf(val any) defaultKind {
    switch val.(type) {
    case nil:
        return kindNone
    case []any:
        return kindArray
    case map[string]any:
        return kindObject
    case string, float64, bool:
        return kindScalar
    default:
        // Fallback: classify by fmt string
        s := fmt.Sprint(val)
        if strings.HasPrefix(s, "[") {
            return kindArray
        }
        if strings.HasPrefix(s, "map[") || strings.HasPrefix(s, "{") {
            return kindObject
        }
        return kindScalar
    }
}

// includesNull reports whether the property's type union contains null.
func includesNull(d *load.Definition) bool {
    if d == nil {
        return false
    }
    arr, ok := d.Type.([]any)
    return ok && slices.Contains(arr, any(schemaTypeNull))
}

// expandAllOf merges JSON Schema allOf nodes into a shallow composite definition.
//
// ACP's schema frequently uses `allOf: [{"$ref": "#/$defs/Type"}]` so a property or union
// variant can attach local metadata (description/default/const constraints) while reusing
// a referenced shape.
//
// For codegen purposes we approximate allOf by merging object properties + required lists.
// We intentionally do not attempt to resolve semantic constraints beyond that.
func expandAllOf(schema *load.Schema, d *load.Definition) *load.Definition {
    if d == nil || len(d.AllOf) == 0 {
        return d
    }

    merged := *d
    // Avoid re-processing in recursive calls.
    merged.AllOf = nil
    merged.Properties = maps.Clone(d.Properties)
    merged.Required = slices.Clone(d.Required)
    reqSet := requiredSet(merged.Required)

    for _, part := range d.AllOf {
        if part == nil {
            continue
        }
        resolved := part
        if name, ok := strings.CutPrefix(part.Ref, "#/$defs/"); ok && schema != nil {
            if def := schema.Defs[name]; def != nil {
                resolved = def
            }
        }
        resolved = expandAllOf(schema, resolved)

        copyMissingAllOfFields(&merged, resolved)

        if len(resolved.Properties) > 0 {
            if merged.Properties == nil {
                merged.Properties = make(map[string]*load.Definition)
            }
            for k, v := range resolved.Properties {
                if _, ok := merged.Properties[k]; !ok {
                    merged.Properties[k] = v
                }
            }
        }
        for _, r := range resolved.Required {
            if !reqSet[r] {
                merged.Required = append(merged.Required, r)
                reqSet[r] = true
            }
        }
    }

    return &merged
}

func copyMissingAllOfFields(merged *load.Definition, resolved *load.Definition) {
    if merged.Type == nil && resolved.Type != nil {
        merged.Type = resolved.Type
    }
    if merged.Items == nil && resolved.Items != nil {
        merged.Items = resolved.Items
    }
    if merged.Ref == "" && resolved.Ref != "" {
        merged.Ref = resolved.Ref
    }
    if len(merged.Enum) == 0 && len(resolved.Enum) > 0 {
        merged.Enum = resolved.Enum
    }
    if len(merged.AnyOf) == 0 && len(resolved.AnyOf) > 0 {
        merged.AnyOf = resolved.AnyOf
    }
    if len(merged.OneOf) == 0 && len(resolved.OneOf) > 0 {
        merged.OneOf = resolved.OneOf
    }
}

// typeExpr maps a schema node to a Go type expression.
func typeExpr(d *load.Definition) ast.Expr {
    if d == nil {
        return id("any")
    }

    // JSON Schema 2020-12 commonly uses allOf wrappers around a single $ref so a property can
    // attach local metadata like descriptions and defaults.
    if len(d.AllOf) > 0 && d.Ref == "" {
        // Prefer a $ref element when present, otherwise the first non-nil schema.
        for _, e := range d.AllOf {
            if e != nil && e.Ref != "" {
                return typeExpr(e)
            }
        }
        for _, e := range d.AllOf {
            if e != nil {
                return typeExpr(e)
            }
        }
    }

    if d.Ref != "" {
        if name, ok := strings.CutPrefix(d.Ref, "#/$defs/"); ok {
            return id(name)
        }
        return id("any")
    }
    if len(d.Enum) > 0 {
        return id("string")
    }
    switch t := ir.PrimaryType(d); t {
    case schemaTypeString, schemaTypeInteger, schemaTypeNumber, schemaTypeBoolean:
        return primitiveType(t)
    case schemaTypeArray:
        return sliceOf(typeExpr(d.Items))
    case schemaTypeObject:
        return rawMap(id("any"))
    default:
        return id("any")
    }
}

// pointerPrimitive returns *T for a primitive schema type, or nil.
func pointerPrimitive(t string) ast.Expr {
    switch t {
    case schemaTypeString, schemaTypeInteger, schemaTypeNumber, schemaTypeBoolean:
        return star(primitiveType(t))
    default:
        return nil
    }
}

// optionalTypeExpr maps unions that include null to pointer types where applicable.
func optionalTypeExpr(d *load.Definition) ast.Expr {
    if d == nil {
        return id("any")
    }
    // Case: property type is a union like ["string","null"]
    if arr, ok := d.Type.([]any); ok && len(arr) == 2 {
        var other string
        for _, v := range arr {
            if s, ok := v.(string); ok && s != schemaTypeNull {
                other = s
            }
        }
        if p := pointerPrimitive(other); p != nil {
            return p
        }
    }
    list := d.AnyOf
    if len(list) == 0 {
        list = d.OneOf
    }
    if len(list) == 2 {
        var nonNull *load.Definition
        for _, e := range list {
            if e == nil {
                continue
            }
            if s, ok := e.Type.(string); ok && s == schemaTypeNull {
                continue
            }
            if e.Const != nil {
                nn := *e
                nn.Type = schemaTypeString
                nonNull = &nn
            } else {
                nonNull = e
            }
        }
        if nonNull != nil {
            if name, ok := strings.CutPrefix(nonNull.Ref, "#/$defs/"); ok {
                return star(id(name))
            }
            if p := pointerPrimitive(ir.PrimaryType(nonNull)); p != nil {
                return p
            }
        }
    }
    return typeExpr(d)
}

type variantInfo struct {
    fieldName         string
    typeName          string
    required          []string
    isObject          bool
    isArray           bool
    arrayItemRequired []string
    discValue         string
    constPairs        [][2]string
    isNull            bool
    description       string
    hasSharedFields   bool
}

// singleRefVariant returns the $ref when v is only an allOf wrapper around one $ref.
func singleRefVariant(v *load.Definition) string {
    if v.Type == nil &&
        len(v.Properties) == 0 &&
        len(v.Required) == 0 &&
        len(v.Enum) == 0 &&
        v.Items == nil &&
        len(v.AnyOf) == 0 &&
        len(v.OneOf) == 0 &&
        len(v.AllOf) == 1 &&
        v.AllOf[0] != nil &&
        v.AllOf[0].Ref != "" {
        return v.AllOf[0].Ref
    }
    return ""
}

// discriminatorKey returns the schema's discriminator property, or the first
// const property found on a variant, in sorted property order.
func discriminatorKey(schema *load.Schema, parent *load.Definition, variants []*load.Definition) string {
    if parent != nil && parent.Discriminator != nil && parent.Discriminator.PropertyName != "" {
        return parent.Discriminator.PropertyName
    }
    for _, v := range variants {
        if v == nil {
            continue
        }
        v = expandAllOf(schema, v)
        for _, k := range slices.Sorted(maps.Keys(v.Properties)) {
            if pd := v.Properties[k]; pd != nil && pd.Const != nil {
                return k
            }
        }
    }
    return ""
}

// registerName records tname as used and panics on a collision.
func registerName(usedTypeNames map[string]bool, tname, context string) {
    if usedTypeNames[tname] {
        panic(fmt.Sprintf("BUG: variant type name collision: %q already registered (%s)", tname, context))
    }
    usedTypeNames[tname] = true
}

func oneUnionVariant(f *goFile, name string, schema *load.Schema, idx int, v *load.Definition, discKey string, sharedProps map[string]*load.Definition, sharedRequired map[string]bool, usedTypeNames map[string]bool) (variantInfo, bool) {
    if v == nil {
        return variantInfo{}, false
    }
    ref := v.Ref
    // If this is an allOf wrapper around a single $ref with no additional structure, treat it
    // as a $ref variant (keeps types stable and avoids duplicate structs).
    if ref == "" {
        ref = singleRefVariant(v)
    }
    v = expandAllOf(schema, v)
    // Detect null-only variant
    s, ok := v.Type.(string)
    isNull := ok && s == schemaTypeNull

    // Determine type name: prefer $ref target name when present; do not treat Title as a rename for $ref.
    tname := ""
    if after, ok := strings.CutPrefix(ref, "#/$defs/"); ok {
        tname = after
    } else if v.Title != "" {
        // Scope inline variant titles to parent union name with smart naming
        tname = generateNestedTypeName(name, v.Title, usedTypeNames)
        registerName(usedTypeNames, tname, fmt.Sprintf("parent: %q, title: %q", name, v.Title))
    } else {
        if discKey != "" {
            if pd := v.Properties[discKey]; pd != nil && pd.Const != nil {
                c := fmt.Sprint(pd.Const)
                tname = generateNestedTypeName(name, c, usedTypeNames)
                registerName(usedTypeNames, tname, fmt.Sprintf("parent: %q, discriminator: %q", name, c))
            }
        }
        if tname == "" {
            tname = fmt.Sprintf("%sVariant%d", name, idx+1)
            registerName(usedTypeNames, tname, fmt.Sprintf("parent: %q", name))
        }
    }
    // Ensure Title-derived names are exported (e.g., "stdio" -> "Stdio").
    tname = util.ToExportedField(tname)

    // Derive field name with priority: discriminator value > title > type name
    fieldName := ""
    dv := ""
    if discKey != "" {
        if pd := v.Properties[discKey]; pd != nil && pd.Const != nil {
            dv = fmt.Sprint(pd.Const)
            fieldName = util.ToExportedField(dv)
        }
    }
    if fieldName == "" && v.Title != "" {
        fieldName = util.ToExportedField(v.Title)
    }
    if fieldName == "" {
        fieldName = util.ToExportedField(tname)
    }
    isObj := len(v.Properties) > 0
    isArray := ir.PrimaryType(v) == schemaTypeArray
    arrayItemRequired := []string{}
    if isArray && v.Items != nil {
        item := v.Items
        if itemName, ok := strings.CutPrefix(item.Ref, "#/$defs/"); ok {
            if d := schema.Defs[itemName]; d != nil {
                item = d
            }
        }
        item = expandAllOf(schema, item)
        if ir.PrimaryType(item) == schemaTypeObject && len(item.Required) > 0 {
            arrayItemRequired = slices.Sorted(slices.Values(item.Required))
        }
    }
    // Skip phantom variants that have neither $ref nor object shape nor null nor title
    // (but allow title-only variants like ExtMethodRequest to generate as empty structs)
    if !isObj && ref == "" && !isNull && v.Title == "" {
        return variantInfo{}, false
    }
    // Collect const properties (e.g., type, outcome).
    var consts [][2]string
    for _, pk := range slices.Sorted(maps.Keys(v.Properties)) {
        if pd := v.Properties[pk]; pd != nil && pd.Const != nil {
            if c, ok := pd.Const.(string); ok {
                consts = append(consts, [2]string{pk, c})
            }
        }
    }
    emitInlineUnionStruct(f, name, tname, ref, v, isObj, isNull, sharedProps, sharedRequired, usedTypeNames)
    return variantInfo{
        fieldName:         fieldName,
        typeName:          tname,
        required:          v.Required,
        isObject:          isObj,
        isArray:           isArray,
        arrayItemRequired: arrayItemRequired,
        discValue:         dv,
        constPairs:        consts,
        isNull:            isNull,
        description:       v.Description,
        hasSharedFields:   ref == "" && isObj && !isNull,
    }, true
}

func emitUnion(f *goFile, name string, schema *load.Schema, parentDef *load.Definition, defs []*load.Definition, exactlyOne bool, usedTypeNames map[string]bool) {
    discKey := discriminatorKey(schema, parentDef, defs)
    sharedProps := map[string]*load.Definition{}
    sharedRequired := map[string]bool{}
    if parentDef != nil {
        maps.Copy(sharedProps, parentDef.Properties)
        sharedRequired = requiredSet(parentDef.Required)
    }

    var variants []variantInfo
    for idx, v := range defs {
        if vi, ok := oneUnionVariant(f, name, schema, idx, v, discKey, sharedProps, sharedRequired, usedTypeNames); ok {
            variants = append(variants, vi)
        }
    }

    // Wrapper: one pointer field per variant, set by UnmarshalJSON.
    var st fieldSet
    for _, vi := range variants {
        if vi.description != "" {
            st.comment(util.FormatDocComment(vi.description)...)
        }
        st.add(structField(vi.fieldName, star(id(vi.typeName)), "-"))
    }
    f.add(typeDecl(name, st.structType()))
    f.line()
    f.add(method(param("u", star(id(name))), "UnmarshalJSON",
        funcType([]*ast.Field{param("b", byteSlice())}, param("", id("error"))),
        unionUnmarshalBody(f, variants, discKey)...))
    f.add(method(param("u", id(name)), "MarshalJSON",
        funcType(nil, param("", byteSlice()), param("", id("error"))),
        unionMarshalBody(f, name, variants)...))
    f.line()

    // oneOf: exactly one variant. anyOf: shared required fields live on the variants.
    if exactlyOne {
        f.add(validateMethod("u", name, unionValidateBody(f, name, variants)...))
    } else {
        f.add(validateMethod("v", name, sharedUnionValidateBody(f, variants, sharedFieldChecks(parentDef))...))
    }
    f.line()
}
```

### `cmd/generate/internal/emit/file_test.go`

```go
package emit

import (
    "go/ast"
    "go/token"
    "strings"
    "testing"
)

func render(t *testing.T, f *goFile) string {
    t.Helper()
    out, err := f.render()
    if err != nil {
        t.Fatal(err)
    }
    return string(out)
}

// src joins lines into Go source text with a trailing newline.
func src(lines ...string) string { return strings.Join(lines, "\n") + "\n" }

const header = "// Code generated by acp-go-generator; DO NOT EDIT.\n\npackage acp\n"

func TestRenderLayout(t *testing.T) {
    t.Parallel()
    tests := []struct {
        name  string
        build func(f *goFile)
        want  string
    }{
        {
            name: "doc comment and grouped const with one spec",
            build: func(f *goFile) {
                f.comment("Kinds of things.", "", "Second paragraph.")
                f.add(constDecl(true, constSpec("KindA", id("Kind"), str("a"))))
            },
            want: header + src(
                "",
                "// Kinds of things.",
                "//",
                "// Second paragraph.",
                "const (",
                "\tKindA Kind = \"a\"",
                ")",
            ),
        },
        {
            name: "single const without parentheses",
            build: func(f *goFile) {
                f.add(constDecl(false, constSpec("N", nil, intLit(1))))
            },
            want: header + src("", "const N = 1"),
        },
        {
            name: "empty struct, alias, and documented fields",
            build: func(f *goFile) {
                f.add(typeDecl("Empty", &ast.StructType{Fields: fields()}))
                f.line()
                f.add(aliasDecl("Raw", f.qual("encoding/json", "RawMessage")))
                f.line()
                var st fieldSet
                st.comment("The name.")
                st.add(structField("Name", id("string"), "name"))
                st.add(structField("Count", star(id("int")), "count,omitempty"))
                f.add(typeDecl("Thing", st.structType()))
            },
            want: header + src(
                "",
                "import \"encoding/json\"",
                "",
                "type Empty struct{}",
                "",
                "type Raw = json.RawMessage",
                "",
                "type Thing struct {",
                "\t// The name.",
                "\tName  string `json:\"name\"`",
                "\tCount *int   `json:\"count,omitempty\"`",
                "}",
            ),
        },
        {
            name: "short body stays multi-line and adjacent funcs keep no blank line",
            build: func(f *goFile) {
                f.add(function("A", funcType(nil, param("", id("int"))), ret(intLit(1))))
                f.add(function("B", funcType(nil, param("", id("int"))), ret(intLit(2))))
                f.line()
                f.add(function("C", funcType(nil)))
            },
            want: header + src(
                "",
                "func A() int {",
                "\treturn 1",
                "}",
                "func B() int {",
                "\treturn 2",
                "}",
                "",
                "func C() {",
                "}",
            ),
        },
        {
            name: "composite literal with several elements is one element per line",
            build: func(f *goFile) {
                f.add(function("New", funcType(nil, param("", id("T"))),
                    ret(composite(id("T"), kv(id("Outer"), addr(composite(id("U"),
                        kv(id("Alpha"), id("a")),
                        kv(id("B"), str("b")),
                    ))))),
                ))
            },
            want: header + src(
                "",
                "func New() T {",
                "\treturn T{Outer: &U{",
                "\t\tAlpha: a,",
                "\t\tB:     \"b\",",
                "\t}}",
                "}",
            ),
        },
        {
            name: "imports are sorted and grouped",
            build: func(f *goFile) {
                f.add(function("F", funcType([]*ast.Field{param("ctx", f.qual("context", "Context"))}, param("", id("error"))),
                    ret(call(f.qual("errors", "New"), str("x")))))
            },
            want: header + src(
                "",
                "import (",
                "\t\"context\"",
                "\t\"errors\"",
                ")",
                "",
                "func F(ctx context.Context) error {",
                "\treturn errors.New(\"x\")",
                "}",
            ),
        },
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()
            f := newFile()
            tt.build(f)
            if got := render(t, f); got != tt.want {
                t.Errorf("render mismatch\n--- got ---\n%s\n--- want ---\n%s", got, tt.want)
            }
        })
    }
}

func TestStructTagQuoting(t *testing.T) {
    t.Parallel()
    if got := structTag("a,omitempty").Value; got != "`json:\"a,omitempty\"`" {
        t.Errorf("structTag = %s", got)
    }
    // A backquote cannot sit in a raw string, so the tag is double-quoted.
    if got := structTag("a`b").Value; got[0] != '"' {
        t.Errorf("structTag with backquote = %s", got)
    }
}

func TestLayoutStaysBelowPrinterSentinel(t *testing.T) {
    t.Parallel()
    // go/printer treats offsets at or above 1<<30 as "no more comments".
    f := newFile()
    for range 3000 {
        f.comment("doc")
        f.add(function("F", funcType(nil), exprStmt(call(id("g")))))
        f.line()
    }
    fset := token.NewFileSet()
    file := f.layout(fset)
    if end := fset.File(file.Package).Size(); end >= 1<<30 {
        t.Fatalf("layout spans %d bytes", end)
    }
}
```

### `cmd/generate/internal/ir/ir.go`

Only the import block changes:

```go
import (
    "sort"
    "strings"

    "github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/load"
    "github.com/maccavelli/acp-go-sdk2/cmd/generate/internal/util"
)
```

## Appendix B: reproducing the checks

All of these run on copies, never in the tree. `<copy>` is a scratch copy of the repository without `.git`.

- Old output: `go run .` in `cmd/generate` with `-schema schema -out <dir>`, from the repository root; compare `<dir>/*.go` with the committed files using `cmp`.
- Rename: replace the old module path in `*.go`, `*.mod`, `README.md`, `AGENTS.md`, and `docs/architecture.md` of `<copy>`; then `go test ./...` and `go build ./example/...`.
- Cross-version: for each commit, `git show <commit>:schema/<file>` for the four schema files and `version`; run both generator binaries from the repository root; compare outputs byte for byte.
  Commits used: `HEAD`, `9fb9ae5`, `0845a3b`, `d03a894`, `192e108`, `6595492`, `72c8f1d`, `c180b9d`, `584abe6`, `c225368`, `cc23bb6`, `2c1aef2`, `da204ff`.
- Breakage: copy `<copy>`, apply one change from the table above, run `go test -run '^<Test>$' ./...` in `cmd/generate`, and expect a failure.
- JSON zero values: a test in `<copy>` that, for every `type X struct` in `types_gen.go`, compares `json.Marshal(X{})` with `jsonv2.Marshal(X{}, jsonv2.Deterministic(true))`
  and with `jsonv2.Marshal(X{}, json.DefaultOptionsV1())`.
- Golden suite under v2: replace the three calls in `runGolden` in `<copy>/json_parity_test.go` with `jsonv2.Marshal` and `jsonv2.Unmarshal`, with and without `json.DefaultOptionsV1()`.
