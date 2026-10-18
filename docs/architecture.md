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
  assigns every field, the `Connection` from `buildConnection` included, and
  hands the agent or client its connection through `AgentConnAware` or
  `ClientConnAware`, declared in `connection.go`, when it implements them,
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

Every line of a generated comment, a schema description's included, is
written as a line comment, so no description can become code or a directive.

Every exported generated declaration has a doc comment
(0019-PLAN-examples-docs-tests-tooling.md, I12):

- a type, field or interface method has its schema description;
- `Agent` and `Client` say which side implements them and what serves them;
- an enum constant names its value, then gives the value's description;
- a codec or `Validate` names where its type's schema sits: a definition, by
  its own key; a union's variant; or a property's inline object;
- a typed outbound method says what it sends and to which side, then repeats
  the interface method's doc.

`TestExportedDeclarationsDocumented` parses the package's non-test files and
fails on each exported declaration without a doc comment.

The generator writes nothing until every file has rendered and the five files
type-check with `go/types` together with the hand-written files of the output
directory, at the language version of its `go.mod`: first the package as
built, the hand-written files the go command would build there with the
generated files that are not tests, then the package as tested, which adds
the hand-written in-package test files and `schema_gen_test.go`, so a name
both declare is refused as `go vet` would refuse it. Output that does not
type-check is refused with the first 10 errors, those in generated files
first. The output directory must exist; a missing one is refused, since the
generated code needs the hand-written runtime beside it. A file whose bytes
have not changed is left alone. Every other file is
staged in the output directory as `.<name>.*.tmp`, synced, and set to mode
`0644`; once all are staged, each is renamed into place.

The protocol behaviour the schema does not express lives in one table, `ACP`
in `cmd/generate/internal/model/acp.go`, which `main.go` passes to the model as
`Input.Protocol`. It holds:

- the optional `AgentLoader` interface, for `session/load`;
- the prompt turn: `AgentSideConnection` runs `session/prompt` under a
  per-turn context. `sessionCancels` holds each session's running turns, and
  `storeTurn` stores a new one as the connection's `WithPromptOverlapPolicy`
  has it: supersede, the default, cancels the stored turns with a cause that
  wraps `ErrPromptCancelled`; refuse answers the prompt `-32603` while a turn
  is stored, without calling the handler; concurrent adds it. An inbound
  `session/cancel` cancels the session's stored turns through `cancelTurns`,
  with `ErrPromptCancelled` as their cause. Every received request and
  notification has a receipt number from `connection.go`, and a cancel
  applies only to a turn received before it. Every cancel is also kept in
  `lateCancels` for its session, for a prompt received before it whose turn
  is not stored yet; `keepCancel` in `connection.go` first removes each kept
  receipt that no request in flight is older than, from the receipts it
  keeps of the requests in flight, and, under supersede, a prompt newer than
  the kept cancel removes it. `session/close` (the table's `ClosesSession`)
  runs the same cancel before its handler. When the handler returns, the case
  takes the lock, removes its own turn (`removeTurn`) and reads its cause
  once: a cancelled turn is answered `cancelled`, else the handler's answer
  stands.
  `ClientSideConnection.Prompt` sends `session/cancel` when its context ends
  with an error;
- the permission requests: the client's `session/request_permission` case
  registers each pending request by session in `pending`, and its
  `session/cancel`, and a `session/close` the agent answers with success,
  cancel them, so `answerUnlessCancelled` in
  `connection.go` answers each `cancelled` at once;
- `initialize`: its version and capabilities keys. Both sides keep the
  request and the answer as a `Negotiation` in an
  `atomic.Pointer[Negotiation]`, the JSON form of what was sent and received
  (`jsonCopy`), and `Negotiated` returns a copy of it; it is stored by the
  agent before its answer is
  written and by the client when `Initialize` returns, a zero version
  filled and another refused;
- the name `SessionUpdate` for the client's `session/update` handler;
- the unions whose constructors `helpers.go` writes;
- the capability that gates each of 15 methods, resolved to its Go fields
  (`CapabilitySteps`);
- `CapabilityRules`, the 19 properties and variants a capability gates,
  each with the specification sentence it follows: among them the rule that an
  `authenticate` must not name a terminal auth method, and the rule that an
  elicitation's catch-all variant, a mode the schema does not list, is set
  only under a capability no client can advertise, so it is always refused,
  naming `elicitation.` and the mode (`GateUndeclaredVariant`). `model/gates.go`
  resolves each on its definition and on each copy of it the merge made from
  the unstable schema under another name, and gives a `gates` method to every type
  that holds one, directly or through fields, members or variants; with no
  `initialize` rule, none applies;
- `AbsolutePaths`, the 17 properties the protocol requires to be absolute
  paths, each with the specification sentence it follows. A rule reaches every
  type that holds its definition's property, an inline variant built through
  `allOf` included, and `Validate` checks it;
- `PatchFields`, the 12 properties with patch semantics (omitted keeps the
  stored value, `null` clears it, a value replaces it), each with the
  specification sentence it follows. A rule reaches every type built from its
  definition, as `AbsolutePaths` does, and the model refuses one whose property
  is required, has a default, or lacks `x-deserialize-default-on-error`. The
  field's type becomes a `RefPatch` of its type without the pointer, emitted as
  `Patch[T]` and tagged `omitzero`;
- `NullRetained`, the optional raw properties whose explicit `null` the schema
  keeps apart from omission, each with the schema sentence it follows: the
  unstable `McpError.data`. A field built from one keeps `null` when decoded;
- `KeyOrders`, the typed maps whose key order the protocol gives meaning, each
  with the field that holds the order and the reason: `ElicitationSchema`'s
  `properties`, field `PropertyOrder`. The generated `UnmarshalJSON` reads the
  map's keys in order through `keyOrder`; `MarshalJSON` writes them in that
  order through `orderedObject` when an order is set, and as before when none
  is. The model refuses a rule on a property that is not a typed map of a
  struct, on a struct with flattened members or kept extra keys, or whose
  field the struct already has.

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
required key, a default, a lenient field, flattened members, or an `Extra`
gets a generated `UnmarshalJSON`. A struct gets a generated
`MarshalJSON` only when it does more than encode its fields: a nilable array
or object default it writes in, a nil required array or map, an `Extra`, or
flattened members. A catch-all has an `Extra`, and so does an object whose
`additionalProperties` is `true` (`Type.KeepExtra`): it keeps the keys it does
not declare, after folding, and writes them back, a declared field winning.
The model refuses an `additionalProperties` the struct would not keep: a typed
one on an object with properties, one beside flattened members, on a union
with keys of its own, on an inline object property, or on an inline variant
other than the catch-all, and a property that would take the field `Extra`.

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

A definition with properties and no `type` is an object, as a variant with
properties is: it gets its decoder, `Validate` and gates, and refuses a value
that is not an object, which plain JSON Schema would allow. The model refuses,
naming the definition, the shapes no Go type holds as the schema states them
([0019-PLAN-generator-hardening.md](decisions/0019-PLAN-generator-hardening.md)):
an `enum` with a value that is not a string; a type list of two types other
than `null`; a named array definition; a union variant with no title, `$ref`,
properties or null type; a `const` on a union variant that is not an object,
which no generated code would check (a `oneOf` of string consts, or of titled
integer consts, is an enum kind); a discriminator const that is not a string;
a property key a json struct tag cannot carry; a field, union wrapper field or
flattened member named `MarshalJSON`, `UnmarshalJSON` or `Validate`; a
`required` key no property declares; and a definition linked by `x-method` and
`x-side` to a method no meta file lists. `Roots`, written by hand on five
request types, is left to the type checker: a property named `roots` there
does not type-check.

Every keyword the schema loader accepts has one treatment, listed in
`cmd/generate/internal/model/keywords.go`; the loader refuses any other
keyword, and one written in another letter case, which `encoding/json` would
match, and the model refuses an integer or number `format` without a Go type.
A property's `default` is checked against its schema, through references,
union members, array items and object properties: its JSON kind, an integer's
range for its format, its `enum` and its `const`, so the generated decoding of
a default, which ignores the error, cannot fail.
A number takes the Go type of its `format`; a typed `additionalProperties`
gives a typed map; a value the model gives no Go type is `json.RawMessage`; and
an optional struct value carries no `omitempty`. A generated `UnmarshalJSON`
requires the required keys through `objectKeys`, decodes each
`x-deserialize-default-on-error` field through `orDefault` and each
`x-deserialize-skip-invalid-items` array through `skipInvalid`, empties
an optional raw value that is `null` through `absentIfNull`, so it is not
sent on, and reads a patch field through `patchOf`, or `patchItems` for an
array that skips invalid items: a missing key is absent, `null` is null, a
value that decodes is that value, and one that does not is absent;
`types_gen.go` emits each helper once, and `Patch[T]` with its constructors
and methods once, when a patch field exists. A null raw value is an absent
one, as ACP v1 defines it for `rawInput` and `rawOutput`; a required one, such
as `result`, keeps `null`, and so does an optional one the protocol table's
`NullRetained` names, a property whose schema says explicit null is retained
(the unstable `McpError.data`): the model refuses such a rule on a required or
non-raw property, or one whose description does not state its source, and a
model test requires every committed property that says so to be named.
`Validate` checks what the schema states: a
union has exactly one variant set, a string meets its `minLength`, a number
meets a bound its Go type does not enforce, and every nested value validates,
a patch's through the value it holds; an integer bound with a fraction is
rounded inward, a `uint64` one above the `int64` range is written as such, and
one the Go type cannot reach is refused;
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
first refuses a method received as the other kind, a request sent without an
id or a notification sent with one, by the mark `connection.go` puts in the
handler's context; it then decodes the params, absent ones as `{}`
(`decodeParams`), and calls `checkInbound`; `sendRequest` calls it on a
decoded result. `extensions.go` calls it on a received `_` message, with its
params as a `*json.RawMessage` and the side's `inboundGate`, the side's `gate`
when the schema has `initialize`, before the message reaches
`HandleExtensionMethod`.

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
`SendRequest[T]` and the outbound extension methods pass no gate.

`receive` reads one line at a time through a `bufio.Reader`, up to
`WithMaxMessageSize` (10 MiB by default), and classifies it: an overlong
line is discarded and answered `-32600`; a line `json.Valid` refuses,
`-32700`; JSON that is not an object, `-32600`; an object without `method`
that has `result` or `error`, or whose `id` names a call still pending, is a
response, matched to its pending call and never answered (one with neither
`result` nor `error` is logged as a malformed response); anything else is a
request (an `id` member, `null` included) or a
notification, and one that is not a valid Request object is answered
`-32600`, with its id when valid and `null` otherwise; `params` of `null` are
read as absent. A malformed line is logged by its fault, size and id, never its
content.

Each request runs on its own goroutine, its receipt kept in `inflightReceipts`
while it runs and its cancel in `inflight` by id, each request removing only
its own entry. With `WithMaxConcurrentRequests(n)`, `receive` takes a slot of
`requestSlots`, a channel of capacity `n`, for each request, and answers one it
finds no slot for `-32603` without starting it; a slot is given back when the
handler returns, before its answer is written. Notifications run one at a time from a queue, each handler's
context marked with its sequence number. A response is returned after the
notifications received before it are handled, unless the request was sent
under the context of a notification whose handler is still running: that wait
stops before it. A message's params are left out when they encode as `null`,
refused when they encode as neither an object nor an array, and a request
whose context has ended is not written.

One writer goroutine, started by `start`, writes every message, in the order
`sendMessage` hands them over through an unbuffered channel; a sender returns
when its context ends, even while a write is blocked, and a write that has
finished is reported in preference to the context. Once the connection's
context has ended, `sendMessage` writes nothing and returns `endError`:
`ErrConnectionClosed` when this side closed it, through `Close` or the
overflow policy (`closeWith`), and otherwise `ErrPeerDisconnected` wrapping how
the reader ended. `Close` runs once: it marks the connection closed, ends its
context with `ErrConnectionClosed`, and closes the reader and the writer that
implement `io.Closer`. A `*RequestError` the connection makes for a local
failure keeps that failure as its cause, which `Unwrap` returns; one a peer
answered has none.

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
- `connection_close_test.go` covers a send that returns on its context, `Close`,
  a send after either end, and the overflow close; `errors_test.go` the
  causes of local errors; `conn_aware_test.go` the hand-off before reading,
  on both sides; and `connection_cancel_test.go` the request limit.
- `cancel_test.go` covers prompt turns and permission requests on cancel
  and on `session/close`, cancels ordered across two prompts, and the bound
  on kept cancels,
  `jsonrpc_test.go` the answers to malformed input and the size limit,
  `negotiation_test.go` the record and the version, and `gates_test.go`
  every method and value gate both ways, the undeclared elicitation mode,
  extension messages through the hooks and the gate, the hook's order, the
  raw paths, and `initialize`'s own check; the engine cases
  `accept-prompt-cancellation`, `accept-permission-cancellation`,
  `accept-initialize-negotiation`, `accept-capability-gates`,
  `accept-capability-undeclared-variant` and `accept-session-close-cancels`
  cover the generated code, and
  `accept-description-comment-markers` the generator's comments.
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
