---
status: accepted
date: 2026-10-09
decision-makers: Repository owner
---

# Emit generated Go with go/ast instead of Jennifer

## Context and Problem Statement

`cmd/generate` reads the ACP schema and writes `constants_gen.go`, `types_gen.go`, `agent_gen.go`, `client_gen.go`, and `helpers_gen.go`. The emit package builds those files with `github.com/dave/jennifer` v1.7.1. `jenwrap.go` aliases that library. `WriteConstantsJen`, `WriteTypesJen`, `WriteDispatchJen`, and `WriteHelpersJen` render a Jennifer file and write the bytes. The load and IR packages do not import Jennifer.

The emit sources are about 2,100 lines: `types.go` 1,505, `dispatch.go` 277, `helpers.go` 192, `constants.go` 63, and `jenwrap.go` 88. The owner asked to stop using Jennifer and to emit Go from the schema with the AST.

`go/types` is the type checker. It does not write source. The standard-library writer is `go/ast` for the tree, `go/token` for positions, and `go/printer` or `go/format` for the bytes.

## Decision Drivers

* The owner said Jennifer must no longer be a dependency at all. That means no `require`, no `replace`, no `exclude`, no `go.sum` line, and no Go import.
* The generator should keep its only third-party requirement from growing, and this change removes one.
* The published SDK surface is the generated files. An emitter swap that changes those files changes the SDK.
* Jennifer's fluent calls do not map one-to-one onto `go/ast` nodes. The port is a rewrite of the emit package, not a rename of an import.

## Considered Options

* Build `*ast.File` values and print them with `go/format`.
* Keep Jennifer.
* Emit Go with text templates.

## Decision Outcome

Chosen option: "Build `*ast.File` values and print them with `go/format`", because that is the standard-library AST path and it removes Jennifer completely. The owner accepted this on 2026-10-09.

Small constructors in `cmd/generate` may return `ast.Expr`, `ast.Stmt`, and `ast.Decl`. They are local helpers. They are not a new code-generation library. `load` and `ir` stay. The generated files are regenerated from the current schema. A diff against the current generated files, after `gofmt` on both sides, must be empty before Jennifer is removed. A nonempty diff stops the work.

### Consequences

* Good, because `github.com/dave/jennifer` is not a requirement of either module, directly or indirectly.
* Good, because the emitted tree is ordinary `go/ast`.
* Neutral, because the emit sources become more verbose. `go/ast` has no fluent builder.
* Bad, because comment placement and blank lines can differ under `go/printer` even when the Go means the same thing. Those differences still fail the empty-diff gate.

### Confirmation

Neither `go.mod` requires `github.com/dave/jennifer`. Neither file contains a `replace` or an `exclude` for it. `cmd/generate/go.sum` does not name it. `jenwrap.go` is gone. No `.go` file imports `github.com/dave/jennifer/jen`. `go list -m all` in `cmd/generate` does not list it. Regenerating and running `gofmt` leaves `constants_gen.go`, `types_gen.go`, `agent_gen.go`, `client_gen.go`, and `helpers_gen.go` unchanged. `go test ./...` passes at the root and in `cmd/generate`.

## Pros and Cons of the Options

### Build `*ast.File` values and print them with `go/format`

* Good, because the printer and the formatter are in the standard library.
* Good, because the schema loader and the IR do not need a rewrite.
* Bad, because every Jennifer call site in the emit package has to be rebuilt, and the first nonempty generated diff blocks the deletion of Jennifer.

### Keep Jennifer

* Good, because the generator already emits the current SDK.
* Bad, because the owner asked to stop using it. `cmd/generate` would still require a third-party code generator.

### Emit Go with text templates

* Good, because templates avoid building syntax nodes.
* Bad, because the owner asked for the AST. Templates can emit Go that does not parse, and this generator's unions and dispatch are full declarations, not short fragments.

## More Information

[0005-MADR-separate-module-from-coder.md](0005-MADR-separate-module-from-coder.md) renames the module path. This decision owns the removal of Jennifer. 0005 must not add Jennifer back. This decision does not perform the rename. The two plans can land in either order. Whichever runs second updates the import path or the requirement it finds.

Implementation is [0006-PLAN-emit-go-with-go-ast.md](0006-PLAN-emit-go-with-go-ast.md). [0007-PLAN-self-contained-acp-sdk.md](0007-PLAN-self-contained-acp-sdk.md) runs this plan after the module rename.

## Amendment 2026-10-09, review findings and layout design

[0008-REPORT-self-contained-sdk-review.md](../reports/0008-REPORT-self-contained-sdk-review.md) checked this decision against the tree and built the chosen option outside the tree.
The chosen option stands. This amendment corrects one fact, records the evidence the review found, and names the design the review proved.

### Corrected fact

The emit package has six source files that build Jennifer code, not five. `dispatch_helpers.go` (92 lines) builds statements through the aliases in `jenwrap.go`. The port covers it.

### What go/printer needs

`go/printer` places comments and most line breaks by token position. A tree built in memory has no positions. A probe on Go 1.27.2 showed what the printer does with position-free nodes, and what the emitters need instead:

| The generated files need | go/printer with no positions |
| :--- | :--- |
| Doc comments above declarations and struct fields | A position-free doc comment on a function printed as `package// F does things.` followed by `p`. |
| Multi-line function bodies | Once `func` has a position, which a doc comment requires, any body that fits in 100 columns collapses onto the signature line unless the closing brace sits on a later line. |
| `const ( X T = "x" )` with one spec | The parentheses are dropped unless `GenDecl.Lparen` is valid. |
| `type X = json.RawMessage` | A defined type is printed unless `TypeSpec.Assign` is valid. |
| `struct{}` and `interface{}` | `struct {` and `}` print on two lines unless both braces share a line. |
| One element per line in the `New…` helpers | Every composite literal prints on one line. |

The printer also treats any source offset at or above `1<<30` as "no more comments" and then dereferences a nil comment. A layout that gave each line a fixed 1 MiB stride crashed on `types_gen.go`.

The 0006 consequence "comment placement and blank lines can differ" understated this. Without a layout technique the empty-diff gate fails on every file.

### Layout design

The emitters build declarations with small `go/ast` constructors, as already decided. A file is an ordered list of comment lines, blank lines, and declarations, in the order the Jennifer emitters wrote them.
Rendering gives every line-starting token a position on one synthetic `token.File`, so each item lands on the line it would occupy in written source:

* Comment groups, declaration keywords, struct fields, interface methods, and grouped `const` specs each get their own line. Empty field lists keep both braces on one line.
* A function's opening brace shares the `func` line and its closing brace takes a later line. Composite literals with more than one element put each element on its own line.
* Lines are 4 KiB long. Each line inside a function body also reserves the printed size of that body, so the printer's running offset never passes the next doc comment, and the whole file stays far below `1<<30`.
* The file is printed with `go/format.Node`, then passed through `go/format.Source`. The second pass parses real positions back, so doc-comment reformatting applies as gofmt applies it to written source. Jennifer also finishes with `format.Source`.
* Discriminator and const-property order is sorted instead of following map iteration. No variant in the current stable or unstable schema has more than one const property, so the output does not change. The old order followed map iteration, which Go randomizes.

`main.go` returns errors instead of panicking. It searches for the repository root only when `-schema` or `-out` is missing. The current `main.go` panics outside the repository even when both flags are set.

### Evidence

The design was built as a complete rewrite in Appendix A of [0008-REPORT-self-contained-sdk-review.md](../reports/0008-REPORT-self-contained-sdk-review.md). It has no `require` and no `go.sum`.
Its output is byte-identical to the committed generated files, and to the Jennifer generator's output on schema files from 13 commits, stamps `0.4.5` to `1.25.0`.
It passes `go vet`, `gofmt`, `golangci-lint` with `.golangci.yml` for the host and for `GOOS=windows`, and `go test -race`.

The rewrite is 2,641 lines of non-test code against 2,338 for the Jennifer `emit` package and `main.go`. 583 lines replace Jennifer itself. The emitters shrink.

### Confirmation, added

The confirmation above stays. These checks are added, and they stay after the port:

* `TestGeneratedFilesUpToDate` in `cmd/generate` regenerates into a temporary directory and requires the committed `*_gen.go` files to match byte for byte. CI already runs `go test` in `cmd/generate`.
* `TestRenderLayout` pins each printer behavior in the table above. `TestLayoutStaysBelowPrinterSentinel` pins the `1<<30` limit.
* Each of those tests was seen to fail on a deliberately broken copy, as recorded in the report.
* Before Jennifer is deleted, both generators produce identical output on the schema files at the 13 commits listed in Appendix B of the report.

### Still out of this decision

The empty-diff rule keeps every quirk of the current output: union descriptions attached to the first variant type, no blank lines between functions in `agent_gen.go` and `client_gen.go`, `var match bool = true`, an ignored discriminator decode error, and a latent unused `_rm`.
It also keeps two wire defects: a union with no variant set encodes to zero bytes, and a required array left nil is sent as `null`.
Changing any of them changes the generated files. [0009-MADR-fix-generated-output-on-json-v1.md](0009-MADR-fix-generated-output-on-json-v1.md) proposes those changes, after this decision is executed.
