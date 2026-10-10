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
the schema files, `internal/model` resolves them into types, unions, and
protocol methods, `internal/emit` builds the files as `go/ast` and prints them
through `go/format`, and `internal/util` holds naming helpers. `make version`
downloads the schema for the stamp in `schema/version` and runs the generator.

The protocol behaviour the schema does not express lives in one table, `ACP`
in `cmd/generate/internal/model/acp.go`, which `main.go` passes to the model as
`Input.Protocol`. It holds:

- the optional `AgentLoader` interface, for `session/load`;
- the prompt turn: `AgentSideConnection` runs `session/prompt` under a
  per-session context that an inbound `session/cancel` cancels, and
  `ClientSideConnection.Prompt` sends `session/cancel` when its context ends
  with an error;
- the name `SessionUpdate` for the client's `session/update` handler;
- the unions whose constructors `helpers.go` writes, and `ToolCallUpdate`'s
  `Validate`;
- the capability that gates each method, recorded and checked against the
  schema, not emitted.

The model applies a rule only where the schema carries its subject, and refuses
a subject whose shape the rule cannot use with an error that starts
`protocol table:`. The emitters take every protocol name from the model.
`constants_gen.go` has a constant for each method of the meta files, among them
`ProtocolMethodCancelRequest`, which `connection.go` uses.

The SDK uses `encoding/json` v1 only. The generated `MarshalJSON` methods send a
union's `Unknown` payload back when no variant is set, fail on a union with
neither, naming it, and send a nil required array or map as `[]` or `{}`. A
union's generated `UnmarshalJSON` leaves the union unchanged on `null`
(`RequestId` sets its `Null` variant) and otherwise clears it first. A union
with a discriminator fails on one that is present and not a string, naming the
union and the key; decodes the variant a known value names; sends an unknown
value to the schema's catch-all variant (`Other`, which keeps undeclared keys
in `Extra`) or keeps it in `Unknown`; and decodes a payload without the
discriminator into the schema's key-less variant, else into the one variant
whose required keys it carries, else into `Unknown`, or fails in a union with
a catch-all. A union without a discriminator tries required keys, array items,
and then each variant in turn. Decode errors name the union and, for a
variant, the discriminator value or the variant. A struct with schema defaults
gets an `UnmarshalJSON` that fills them.

Checks that guard this:

- `TestGeneratedFilesUpToDate` in `cmd/generate` fails when the committed
  `*_gen.go` files differ from the generator's output.
- `TestProtocolTableAppliesToCommittedSchema` in `internal/model` fails when a
  rule of the protocol table no longer applies to the committed schema.
- `json_parity_test.go` compares encodings with the goldens in
  `testdata/json_golden/`. `json_v1_pin_test.go` pins HTML escaping and sorted map
  keys. `generated_output_test.go` covers the encoding rules above, and
  `union_wire_test.go` the union decoding rules.
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
  internal/model/         the resolved schema and the protocol table
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
