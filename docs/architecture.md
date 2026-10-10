# Architecture

How this repository is put together, as it is now. This file carries no
history and no rationale — when you want to know *why* something is this
way, the decision records under [decisions/](decisions/) hold the argument,
and [README.md](README.md) points at the ones people ask for most.

## What it is

This repository is the Go module `github.com/maccavelli/acp-go-sdk2`, package
`acp`, at the repository root.

- Hand-written: `agent.go` and `client.go` (the two connection types),
  `connection.go` (JSON-RPC over a reader and a writer), `errors.go`,
  `extensions.go` (`_`-prefixed methods), `helpers.go` (union constructors and
  `Ptr`), and `doc.go`.
- Generated from `schema/`: `constants_gen.go`, `types_gen.go`, `agent_gen.go`,
  `client_gen.go`, and `helpers_gen.go`. They are never edited by hand.

`cmd/generate` is its own Go module with no requirements. `internal/load` reads
the schema files, `internal/ir` groups the protocol methods, `internal/emit`
builds the files as `go/ast` and prints them through `go/format`, and
`internal/util` holds naming helpers. `make version` downloads the schema for the
stamp in `schema/version` and runs the generator.

The SDK uses `encoding/json` v1 only. The generated `MarshalJSON` methods fail on
a union with no variant set, naming it, and send a nil required array or map as
`[]` or `{}`. A union's generated `UnmarshalJSON` fails on a discriminator that
is present and not a string, naming the union and the key. A struct with schema
defaults gets an `UnmarshalJSON` that fills them.

Checks that guard this:

- `TestGeneratedFilesUpToDate` in `cmd/generate` fails when the committed
  `*_gen.go` files differ from the generator's output.
- `json_parity_test.go` compares encodings with the goldens in
  `testdata/json_golden/`. `json_v1_pin_test.go` pins HTML escaping and sorted map
  keys. `generated_output_test.go` covers the encoding rules above.
- `make pre-add-check` runs `scripts/go-precheck.sh`: gofmt, golangci-lint for the
  host and for `GOOS=windows`, vet, tests with `-race`, `go mod tidy -diff`, and
  govulncheck, for both modules. `make check-records` and `make markdownlint`
  check the docs. GitHub Actions runs `.github/workflows/ci.yml`.

The schema stamp lives in `schema/version`. The `go` line lives in `go.mod` and
`cmd/generate/go.mod`.

## Tree

```text
README.md                 repository entry; links here
RELEASING.md              schema bump and release steps
AGENTS.md                 agent instructions
Makefile                  version, test, lint, pre-add-check, docs gates
go.mod                    the SDK module
*.go                      package acp: hand-written and *_gen.go files
cmd/generate/             the generator, its own module
  internal/load/          schema loading
  internal/ir/            method groups
  internal/emit/          go/ast emission
  internal/util/          naming helpers
schema/                   schema files and the version stamp
example/                  agent, client, claude-code, gemini programs
testdata/json_golden/     JSON goldens
scripts/go-precheck.sh    the pre-add gate
scripts/check_records.py  record numbering and link check
.github/workflows/ci.yml  CI
.markdownlint-cli2.jsonc  fleet Markdown style
.gitattributes            LF on every platform
docs/
  README.md               ToC and the "I want to…" matrix
  architecture.md         this file
  decisions/              MADR/PLAN pairs
  reports/                numbered observations
  guides/                 unnumbered how-to documents (not created yet)
```

## What is not here

`docs/guides/` does not exist. The first guide creates it. The Go floor and
the schema pin live in the decision records indexed from `docs/README.md`.
