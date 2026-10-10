---
status: proposed
date: 2026-10-09
associated-madr: "0009-MADR-fix-generated-output-on-json-v1.md"
---

# Implement Fix the generated output on encoding/json v1

Associated MADR: [0009-MADR-fix-generated-output-on-json-v1.md](0009-MADR-fix-generated-output-on-json-v1.md)

## Goal

Change the go/ast generator so the SDK sends what schema `1.25.0` requires, reports an unset union by name, documents every union, and emits idiomatic Go, all on `encoding/json` v1.

## Scope

In scope:

* `cmd/generate/internal/emit/types.go` and `cmd/generate/internal/emit/dispatch.go`
* The five regenerated files: `constants_gen.go`, `types_gen.go`, `agent_gen.go`, `client_gen.go`, `helpers_gen.go`
* New tests in the root module and in `cmd/generate/internal/emit`
* New golden files under `testdata/json_golden/`

Out of scope:

* `encoding/json/v2` and `encoding/json/jsontext`. [0007-MADR-self-contained-acp-sdk.md](0007-MADR-self-contained-acp-sdk.md) keeps the SDK on v1
* Hand-written code in the root module: `connection.go`, `errors.go`, `extensions.go`, `helpers.go`
* Renaming or removing an exported identifier, or adding an exported type, function, or field. The `MarshalJSON` methods of step 5 are the only exported additions
* A schema stamp other than `1.25.0`, and protocol v2
* Hand-editing `*_gen.go`
* A tag, a push, or a merge to `main` without the owner's explicit ask

Preconditions:

* [0007-PLAN-self-contained-acp-sdk.md](0007-PLAN-self-contained-acp-sdk.md) is complete and merged. The generator is the go/ast one, and `TestGeneratedFilesUpToDate` exists.
* The owner has accepted [0009-MADR-fix-generated-output-on-json-v1.md](0009-MADR-fix-generated-output-on-json-v1.md).

## Implementation Steps

1. ~~Create `fix/generated-output-json-v1` from `main`.~~ Amended 2026-10-09: work on `main`. At the end of each step below, run `make pre-add-check`,
   `make check-records`, and `make markdownlint`, record their output, and stop. The owner commits with `git commit --no-edit`.
   An agent commits only on a branch the owner names in that turn, `feature/<slug>` by default, after an explicit ask.
2. Write the failing tests first, in the root module:
   * The zero value of `RequestPermissionResponse` fails to encode with an error containing `RequestPermissionOutcome` and "no variant set".
   * `NewSessionRequest{Cwd: "/home/<user>/project"}` encodes `"mcpServers":[]`, and the decoded value passes `Validate`.
   * A `ContentBlock` whose `type` is the number `1` fails to decode with an error naming `ContentBlock` and `type`.
   * The line above `type ContentBlock struct` in `types_gen.go` is a comment.
   Run them and record each failure message. They must fail now. A test that passes before the change is not testing it.
3. Write the pinning tests, which must pass before and after: text with `<`, `>`, and `&` encodes escaped as `\u003c`, `\u003e`, `\u0026`;
   a `map[string]any` in `_meta` encodes with sorted keys; every existing golden in `testdata/json_golden/` is unchanged.
   Run them and record that they pass.
4. Unset unions. In `unionMarshalBody`, end with `return nil, errors.New("<Union>: no variant set")` instead of `return []byte{}, nil`.
   Regenerate. The union test from step 2 passes.
5. Nil required arrays and maps. In `emitObjectSchema` and in the inline union-variant structs, collect required fields whose Go type is a slice or a map.
   For each such struct emit a value-receiver `MarshalJSON` through an alias type that replaces nil with an empty literal, or add those assignments to the
   `MarshalJSON` that `emitObjectDefaults` already emits. Expect 35 fields in 34 structs at stamp `1.25.0`. Regenerate. The `NewSessionRequest` test passes.
6. Union docs. In `emitSchemaDef`, hold a union's description and emit it directly above the wrapper type, after the variant types.
   Regenerate. The doc test passes, and 24 union types gain a doc comment.
7. Source cleanups, each regenerated:
   * a blank line after every function `emitOutbound` emits, and between a union's `UnmarshalJSON` and `MarshalJSON`;
   * `match := true` instead of `var match bool = true`;
   * check the discriminator decode error and return an error naming the union and the key;
   * use `_, _ok :=` in the defaulted, nullable branch of `emitObjectDefaults`, and add an emit test with a defaulted nullable property that compiles.
   The discriminator test from step 2 passes.
8. Review the regenerated diff. Every changed hunk must belong to one of steps 4 to 7. Record the line counts per file in the execution record.

## Verification

* The step 2 tests failed before steps 4 to 7, with recorded messages, and pass after.
* The step 3 tests pass before and after.
* `TestGeneratedFilesUpToDate` passes, so the committed files match the generator.
* `go test -race ./...` passes at the root and in `cmd/generate`. `go build ./example/...` passes.
* `make pre-add-check` passes, including `golangci-lint` for the host and for `GOOS=windows`.
* `git grep -nE '"encoding/json/(v2|jsontext)"'` finds nothing.
* `go doc -all` of the root package, compared with `main`, differs only by `MarshalJSON` methods on the structs step 5 names.

## Rollout and Rollback

~~All work is on `fix/generated-output-json-v1`.~~ Amended 2026-10-09: commits land on `main`, made by the owner. No tag. A push waits for the owner's explicit ask.

The change alters what the SDK sends for nil required arrays and the error text for an unset union. The release notes for the next module version say so.

Rollback is `git revert` of the step commits on `main`, newest first, made by the owner. Each step regenerates the files it changes, so a revert leaves the generator and the generated files in step.

## Execution record

Not started.
