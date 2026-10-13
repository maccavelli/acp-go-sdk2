# Architecture

How this repository is put together, as it is now. This file carries no
history and no rationale — when you want to know *why* something is this
way, the decision records under [decisions/](decisions/) hold the argument,
and [README.md](README.md) points at the ones people ask for most.

## What it is

This repository is the Go module `github.com/maccavelli/acp-go-sdk2`, package
`acp`, at the repository root.

- Hand-written: `connection.go` (JSON-RPC over a reader and a writer),
  `errors.go`, `extensions.go` (`_`-prefixed methods), `helpers.go` (the
  constructors of the three unions the protocol table names, `Ptr`, and
  `Roots`), and `doc.go`. None of them holds protocol state.
- Generated from `schema/`: `constants_gen.go`, `types_gen.go`, `agent_gen.go`
  and `client_gen.go` (the two connection types, `AgentSideConnection` and
  `ClientSideConnection`, with the state the protocol table's rules need:
  the agent's prompt turns, the client's pending permission requests, and
  on both the record of `initialize`; their dispatch, their outbound
  methods, and the gates each passes to `connection.go`. Each constructor
  assigns every field, the `Connection` from `buildConnection` included,
  before it calls `start`, so a message already waiting never reaches the
  dispatch with a field unset), and `helpers_gen.go`, which
  holds a constructor for every variant of every other union the schema
  documents, and the test file `schema_gen_test.go`, which lists every
  generated type, union, and method for the conformance test. They are never
  edited by hand.

`cmd/generate` is its own Go module with no requirements. `internal/load` reads
the schema files, `internal/model` resolves them into types, unions, and
protocol methods, `internal/emit` builds the files as `go/ast` and prints them
through `go/format` into memory, `internal/output` type-checks and writes them,
and `internal/util` holds naming helpers. `make version` downloads the schema
for the stamp in `schema/version` and runs the generator.

The generator writes nothing until every file has rendered and the five files
type-check with `go/types` together with the hand-written files of the output
directory: those the go command would build there, at the language version of
its `go.mod`. Output that does not type-check is refused with the first 10
errors. A file whose bytes have not changed is left alone. Every other file is
staged in the output directory as `.<name>.*.tmp`, synced, and set to mode
`0644`; once all are staged, each is renamed into place.

The protocol behaviour the schema does not express lives in one table, `ACP`
in `cmd/generate/internal/model/acp.go`, which `main.go` passes to the model as
`Input.Protocol`. It holds:

- the optional `AgentLoader` interface, for `session/load`;
- the prompt turn: `AgentSideConnection` runs `session/prompt` under a
  per-turn context, kept in `sessionCancels` by session, that an inbound
  `session/cancel` or a later prompt for the session cancels with
  `ErrPromptCancelled` as its cause. Every received request and
  notification has a receipt number from `connection.go`, and a cancel
  applies only to a turn received before it, kept in `lateCancels` when the
  turn is not stored yet. When the handler returns, the case takes the
  lock, removes the turn and reads its cause once: a cancelled turn is
  answered `cancelled`, else the handler's answer stands.
  `ClientSideConnection.Prompt` sends `session/cancel` when its context ends
  with an error;
- the permission requests: the client's `session/request_permission` case
  registers each pending request by session in `pending`, and its
  `session/cancel` cancels them, so `answerUnlessCancelled` in
  `connection.go` answers each `cancelled` at once;
- `initialize`: its version and capabilities keys. Both sides keep the
  request and the answer as a `Negotiation` in an
  `atomic.Pointer[Negotiation]`, stored by the agent before its answer is
  written and by the client when `Initialize` returns, a zero version
  filled and another refused;
- the name `SessionUpdate` for the client's `session/update` handler;
- the unions whose constructors `helpers.go` writes;
- the capability that gates each of 15 methods, resolved to its Go fields
  (`CapabilitySteps`);
- `CapabilityRules`, the 18 properties and variants a capability gates,
  each with the specification sentence it follows, and the rule that an
  `authenticate` must not name a terminal auth method. `model/gates.go`
  resolves each on its definition and gives a `gates` method to every type
  that holds one, directly or through fields, members or variants; with no
  `initialize` rule, none applies;
- `AbsolutePaths`, the 17 properties the protocol requires to be absolute
  paths, each with the specification sentence it follows. A rule reaches every
  type that holds its definition's property, an inline variant built through
  `allOf` included, and `Validate` checks it.

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
whose own required keys it carries (not those the union requires of every
variant), else into `Unknown`, or fails in a union with a catch-all. A union
without a discriminator tries required keys, array items, and then each
variant in turn. A variant that is a `$ref` takes its keys, discriminator
value, and shape from the definition it names. Decode errors name the union
and, for a variant, the discriminator value or the variant. A struct with a
required key, a default, a lenient field, flattened members, or a catch-all's
`Extra` gets a generated `UnmarshalJSON`.

Generated decoders read a key as `encoding/json` reads a struct field's name:
under Unicode case folding, the last matching key winning (`foldKeys` in
`types_gen.go`), and the model refuses a schema in which two keys one decoder
reads differ only in case. An object with properties of its own and an `anyOf`
or `oneOf` of references is a struct with one pointer field per member, whose
keys share its JSON object: decoding takes the first member whose required
keys are present, and `Validate` requires exactly one. An `anyOf` or `oneOf`
whose members are all integers, at least one a constant, is a named integer
type with a constant per value, named by the member's title; any integer
decodes. A definition the model gives no Go type is an alias of
`json.RawMessage`.

Every keyword the schema loader accepts has one treatment, listed in
`cmd/generate/internal/model/keywords.go`; the loader refuses any other
keyword, and the model refuses an integer or number `format` without a Go type.
A number takes the Go type of its `format`; a typed `additionalProperties`
gives a typed map; a value the model gives no Go type is `json.RawMessage`; and
an optional struct value carries no `omitempty`. A generated `UnmarshalJSON`
requires the required keys through `objectKeys`, decodes each
`x-deserialize-default-on-error` field through `orDefault` and each
`x-deserialize-skip-invalid-items` array through `skipInvalid`, and empties
an optional raw value that is `null` through `absentIfNull`, so it is not
sent on; `types_gen.go` emits each helper once. A null raw value is an absent
one, as ACP v1 defines it for `rawInput` and `rawOutput`; a required one, such
as `result`, keeps `null`. `Validate` checks what the schema states: a
union has exactly one variant set, a string meets its `minLength`, a number
meets a bound its Go type does not enforce, and every nested value validates;
and it checks the table's absolute paths. Each exported `Validate` calls an
unexported `validate(abs func(string) bool)` with the host's `filepath.IsAbs`,
and passes `abs` to the nested values; every check returns a
`*ValidationError` naming the type, the property and the rule.

`connection.go` applies that check in both directions. Before sending, the
params of a generated type, and a handler's result, are validated with
`isPortableAbs`, which accepts a path absolute in POSIX or Windows form;
nothing is written on failure, and a failing result is answered `-32603`.
On receipt, `checkInbound` runs on every decoded request, notification and
response: the `WithInboundRewrite` hook, then `validate(filepath.IsAbs)`
and the gate, then the `WithInboundValidation` hook with the validation
error and, only if it accepts that, the gate's. The generated dispatch
calls it after decoding, and `sendRequest` on a decoded result.

Gates are generated and passed in, never stored: `connection.go` holds no
record. `gateMessage` in `types_gen.go` lets `initialize` through, refuses
anything else before a record with `ErrNotInitialized`, then applies
`Negotiation.Allows`, the method gates, and the message's `gates`;
`gateResult` applies only `gates`, the method having been gated when the
request was. Each side's `gate` and `resultGate` load its record and call
them: every typed wrapper but `Initialize` calls `gate` before writing and
passes `resultGate` for the response; the dispatch passes `gate` to
`checkInbound` and `resultGate` to `checkResult`; and the `initialize`
paths check the answer against the record it is about to make.
`inboundError` answers `ErrNotInitialized` and a method gate's
`*CapabilityError` `-32601`, other refusals `-32602`. `Connection`,
`SendRequest[T]` and the extension methods pass no gate.

`receive` reads one line at a time through a `bufio.Reader`, up to
`WithMaxMessageSize` (10 MiB by default), and classifies it: an overlong
line is discarded and answered `-32600`; a line `json.Valid` refuses,
`-32700`; JSON that is not an object, `-32600`; an object with `result` or
`error` is a response, matched to its pending call and never answered;
anything else is a request (an `id` member, `null` included) or a
notification, and one that is not a valid Request object is answered
`-32600`, with its id when valid and `null` otherwise.

Checks that guard this:

- `TestGeneratedFilesUpToDate` in `cmd/generate` fails when the committed
  `*_gen.go` files differ from the generator's output, and
  `TestCheckAcceptsRepository` when that output does not type-check with the
  hand-written files. The tests of `internal/output` cover the check and the
  staged write.
- `TestProtocolTableAppliesToCommittedSchema` in `internal/model` fails when a
  rule of the protocol table no longer applies to the committed schema.
- `TestKeywordRegistryMatchesLoader` and `TestCommittedSchemaKeywordsRegistered`
  in `internal/model` fail when a keyword has no treatment.
  `fidelity_test.go` covers the types, decoding, and validation it describes.
- `raw_value_test.go` guards raw values: an absent or `null` optional one
  decodes to empty and is not sent, under both builds of `encoding/json`.
- `paths_test.go` covers the absolute-path rule: sending, receiving, a
  handler's result, the two hooks and their order, and `Roots`; the engine
  case `accept-absolute-paths` covers the generated checks.
- `cancel_test.go` covers prompt turns and permission requests on cancel,
  `jsonrpc_test.go` the answers to malformed input and the size limit,
  `negotiation_test.go` the record and the version, and `gates_test.go`
  every method and value gate both ways, the hook's order, the raw paths,
  and `initialize`'s own check; the engine cases
  `accept-prompt-cancellation`, `accept-permission-cancellation`,
  `accept-initialize-negotiation` and `accept-capability-gates` cover the
  generated code.
- `TestEngineCases` in `cmd/generate` runs the generator on each synthetic case
  in `testdata/engine/`: a case with `want.err` must be refused with those
  lines, one with `want.api` must give that exported API. `TestEngineBehaviour`
  compiles each case's output with its `behaviour_test.go` and runs it.
  `internal/enginetest` reads the cases and lists an API.
- The conformance test at the root, `conformance_test.go` over
  `schema_gen_test.go`, builds instances of every definition from the schema
  files (`instances_test.go`) and requires each generated type to decode,
  validate, and send them back, to refuse what the schema refuses, and each
  method to reach its peer and bring back its result or error.
- `FuzzRoundTrip` at the root and `FuzzBuild` in `cmd/generate` run their seeds
  in every `go test`; `make fuzz` fuzzes them and `FuzzExportedName` for
  `FUZZTIME`. A crasher is kept under the package's `testdata/fuzz/` as a seed.
- `make mutation-engine` and `make mutation-generated` run
  `cmd/generate/internal/mutate`: one operator changed at a time, compiled
  through `go test -overlay`, reported and never gating. The engine run skips
  `TestGeneratedFilesUpToDate`.
- `json_parity_test.go` compares encodings with the goldens in
  `testdata/json_golden/`. `json_v1_pin_test.go` pins HTML escaping and sorted map
  keys. `generated_output_test.go` covers the encoding rules above, and
  `union_wire_test.go` the union decoding rules.
- `make pre-add-check` runs `scripts/go-precheck.sh`: gofmt, golangci-lint for the
  host and for `GOOS=windows`, vet, tests with `-race`, `go mod tidy -diff`, and
  govulncheck, for both modules. `make check-records` and `make markdownlint`
  check the docs. GitHub Actions runs `.github/workflows/ci.yml`, which also runs
  the root tests with `GOEXPERIMENT=nojsonv2` on Linux.

The schema stamp lives in `schema/version`. The `go` line lives in `go.mod` and
`cmd/generate/go.mod`.

## Tree

```text
README.md                 repository entry; links here
RELEASING.md              schema bump and release steps
AGENTS.md                 agent instructions
Makefile                  version, test, lint, pre-add-check, fuzz, mutation, docs gates
go.mod                    the SDK module
*.go                      package acp: hand-written and *_gen.go files
schema_gen_test.go        generated list of types, unions, and methods for tests
cmd/generate/             the generator, its own module
  internal/load/          schema loading
  internal/model/         the resolved schema and the protocol table
  internal/emit/          go/ast emission, rendered in memory
  internal/output/        type-check and staged write of the output
  internal/util/          naming helpers
  internal/enginetest/    engine case reader and API listing, for tests
  internal/mutate/        the mutation tool
  testdata/engine/        synthetic engine cases, one txtar each
  testdata/fuzz/          FuzzBuild seeds kept from crashers
schema/                   schema files and the version stamp
example/                  agent, client, claude-code, gemini programs
testdata/json_golden/     JSON goldens
testdata/fuzz/            FuzzRoundTrip seeds kept from crashers
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
