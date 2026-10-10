---
status: accepted
date: 2026-10-09
decision-makers: Repository owner
---

# Separate this module from github.com/coder/acp-go-sdk

## Context and Problem Statement

The owner is taking this repository over and will work on it alone. It is a separate project. It must not keep `github.com/coder/acp-go-sdk` as its identity, as an upstream, or as a dependency.

On 2026-10-09 the only git remote is `origin`, and that remote is already this repository. There is no `upstream` remote. The Go module line is still `github.com/coder/acp-go-sdk`. `cmd/generate/go.mod` is `github.com/coder/acp-go-sdk/cmd/generate`. Imports in `example/` and `cmd/generate` use that path. `README.md`, `AGENTS.md`, and `docs/architecture.md` name it. The root `go.mod` has no `require` block and no `replace` directive. `cmd/generate` requires `github.com/dave/jennifer v1.7.1` and nothing from `github.com/coder`.

Sibling libraries owned the same way use a module path of `github.com/maccavelli/` plus the repository name. This repository's origin name is `acp-go-sdk2`.

## Decision Drivers

* The owner said this tree is entirely separate and will be worked on by them.
* The module path is the identity other Go code imports. A remote rename does not change it.
* The root module already has no library dependency on the Coder module. The remaining tie is the path itself.
* The generator still needs Jennifer. That requirement is not a Coder dependency.

## Considered Options

* Rename both module paths to `github.com/maccavelli/acp-go-sdk2` and do not add a Coder remote or a Coder requirement.
* Leave the module path as `github.com/coder/acp-go-sdk` because `origin` is already a different remote.
* Remove every third-party requirement, including Jennifer.

## Decision Outcome

Chosen option: "Rename both module paths to `github.com/maccavelli/acp-go-sdk2` and do not add a Coder remote or a Coder requirement", because the owner is keeping this tree separate and the import path is the remaining Coder identity. `origin` stays the current remote. No `upstream` remote is added.

The root module path becomes `github.com/maccavelli/acp-go-sdk2`. The generator module path becomes `github.com/maccavelli/acp-go-sdk2/cmd/generate`. Imports follow those paths. Live docs name the new path. Historical records keep the path they recorded when they were written.

### Consequences

* Good, because importers of this tree no longer use the Coder module path.
* Good, because the root `go.mod` stays free of a `require` or `replace` for `github.com/coder/acp-go-sdk`.
* Neutral, because `cmd/generate` still requires Jennifer.
* Bad, because any external caller still importing `github.com/coder/acp-go-sdk` does not follow this tree. This decision does not publish a module tag.

### Confirmation

`go.mod` and `cmd/generate/go.mod` show the new paths. A search of tracked source, `README.md`, `AGENTS.md`, and `docs/architecture.md` finds no `github.com/coder/acp-go-sdk`. `git remote` has no `upstream`. `go test ./...` passes in the root module and in `cmd/generate`.

## Pros and Cons of the Options

### Rename both module paths to `github.com/maccavelli/acp-go-sdk2` and do not add a Coder remote or a Coder requirement

* Good, because the path matches the existing origin and the sibling-library pattern.
* Good, because it removes the Coder name from the import path without adding a dependency.
* Neutral, because historical decision text still mentions the old path as the path at that time.
* Bad, because the README install line can no longer name a Coder version tag. No replacement tag exists yet.

### Leave the module path as `github.com/coder/acp-go-sdk` because `origin` is already a different remote

* Good, because existing importers of the Coder path would still match this checkout.
* Bad, because the module line would keep presenting this tree as the Coder SDK. The owner said it is separate.

### Remove every third-party requirement, including Jennifer

* Good, because the generator would then have no third-party requirement.
* Bad, because Jennifer emits the generated Go. Removing it is a generator rewrite, not this separation.

## More Information

[0002-MADR-pin-go-1-27-2-and-acp-schema-1-24-1.md](0002-MADR-pin-go-1-27-2-and-acp-schema-1-24-1.md) said the import path stays `github.com/coder/acp-go-sdk` for v1. This decision supersedes that sentence. It does not change the Go 1.27.2 floor.

[0004-MADR-pin-acp-schema-1-25-0.md](0004-MADR-pin-acp-schema-1-25-0.md) left the README `go get` line on the Coder path at `@v0.13.5`. This decision replaces that path. It does not invent a module tag. The schema stamp stays `1.25.0`.

Implementation is [0005-PLAN-separate-module-from-coder.md](0005-PLAN-separate-module-from-coder.md). [0007-MADR-self-contained-acp-sdk.md](0007-MADR-self-contained-acp-sdk.md) runs that plan together with the Jennifer removal.

## Amendment 2026-10-09, Jennifer

The owner said Jennifer must no longer be a dependency at all. [0006-MADR-emit-go-with-go-ast.md](0006-MADR-emit-go-with-go-ast.md) accepts that removal. The sentences in this decision that leave Jennifer required no longer stand. This decision still does not perform the generator rewrite.

## Amendment 2026-10-09, review findings

[0008-REPORT-self-contained-sdk-review.md](../reports/0008-REPORT-self-contained-sdk-review.md) checked this decision against the tree on 2026-10-09. Every fact in the Context still holds. The review adds three facts. None of them changes the chosen option.

The old module path also appears in an `mdsh` directive on `README.md` line 14: `printf 'go get github.com/coder/acp-go-sdk@v%s\n' "$(cat schema/version)"`. The `Makefile` rebuilds `README.md` from `schema/version` whenever `make version` runs. Changing only the install line therefore does not last. The next `make version` writes `go get github.com/maccavelli/acp-go-sdk2@v1.25.0`, which is the tag this decision says it does not invent. The rename includes that directive, and the directive stops appending the schema stamp.

`LICENSE` line 189 reads "Copyright 2025 Coder Technologies, Inc.". Section 4(c) of the Apache License 2.0 requires a derivative work to retain the copyright notices of the work it derives from. "Not the Coder module" means the import path, the remotes, and the requirements. It does not mean the license notice. `LICENSE` and that line stay as they are.

`docs/README.md` names the old path in three record titles. Those titles are historical, like the records they name. The rename does not touch `docs/README.md` or any file under `docs/decisions/` or `docs/reports/`.

[0005-PLAN-separate-module-from-coder.md](0005-PLAN-separate-module-from-coder.md) carries the matching amendment.
