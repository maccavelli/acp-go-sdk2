---
status: proposed
date: 2026-10-09
decision-makers: Repository owner
---

# Emit generated Go with go/ast instead of Jennifer

## Context and Problem Statement

`cmd/generate` reads the ACP schema and writes `constants_gen.go`, `types_gen.go`, `agent_gen.go`, `client_gen.go`, and `helpers_gen.go`. The emit package builds those files with `github.com/dave/jennifer` v1.7.1. `jenwrap.go` aliases that library. `WriteConstantsJen`, `WriteTypesJen`, `WriteDispatchJen`, and `WriteHelpersJen` render a Jennifer file and write the bytes. The load and IR packages do not import Jennifer.

The emit sources are about 2,100 lines: `types.go` 1,505, `dispatch.go` 277, `helpers.go` 192, `constants.go` 63, and `jenwrap.go` 88. The owner asked to stop using Jennifer and to emit Go from the schema with the AST.

`go/types` is the type checker. It does not write source. The standard-library writer is `go/ast` for the tree, `go/token` for positions, and `go/printer` or `go/format` for the bytes.

## Decision Drivers

* The owner asked to stop depending on Jennifer for schema emission.
* The generator should keep its only third-party requirement from growing, and this change removes one.
* The published SDK surface is the generated files. An emitter swap that changes those files changes the SDK.
* Jennifer's fluent calls do not map one-to-one onto `go/ast` nodes. The port is a rewrite of the emit package, not a rename of an import.

## Considered Options

* Build `*ast.File` values and print them with `go/format`.
* Keep Jennifer.
* Emit Go with text templates.

## Decision Outcome

Chosen option: "Build `*ast.File` values and print them with `go/format`", because that is the standard-library AST path and it removes Jennifer. This decision is proposed until the owner approves it.

Small constructors in `cmd/generate` may return `ast.Expr`, `ast.Stmt`, and `ast.Decl`. They are local helpers. They are not a new code-generation library. `load` and `ir` stay. The generated files are regenerated from the current schema. A diff against the current generated files, after `gofmt` on both sides, must be empty before Jennifer is removed. A nonempty diff stops the work.

### Consequences

* Good, because `cmd/generate` no longer requires `github.com/dave/jennifer`.
* Good, because the emitted tree is ordinary `go/ast`.
* Neutral, because the emit sources become more verbose. `go/ast` has no fluent builder.
* Bad, because comment placement and blank lines can differ under `go/printer` even when the Go means the same thing. Those differences still fail the empty-diff gate.

### Confirmation

`cmd/generate/go.mod` has no Jennifer requirement. `jenwrap.go` is gone. No emit file imports `github.com/dave/jennifer/jen`. Regenerating and running `gofmt` leaves `constants_gen.go`, `types_gen.go`, `agent_gen.go`, `client_gen.go`, and `helpers_gen.go` unchanged. `go test ./...` passes at the root and in `cmd/generate`.

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

[0005-MADR-separate-module-from-coder.md](0005-MADR-separate-module-from-coder.md) renames the module path and leaves Jennifer in place. This decision does not perform that rename. The two plans can land in either order. Whichever runs second updates the import path or the requirement it finds.

Implementation is [0006-PLAN-emit-go-with-go-ast.md](0006-PLAN-emit-go-with-go-ast.md).
